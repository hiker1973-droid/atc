package controller

import (
	"regexp"
	"strings"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// Multi-ship flights at the tower (operator 2026-10-21: "can we have a way
// for a multi ship to take off? Example: Senaki traffic, flight of X FA-18s,
// Raider XX, XX and XX, holding short runway 27"). Rulings: ONE takeoff
// clearance and ONE landing clearance cover the whole flight, as USAF / Navy
// towers do for a formation.
//
// The flight is the lead's tower entry, as Marshal's section is
// (cmd/atc/marshal_section.go): replies go to "<lead> flight", a wingman who
// calls is answered as the flight, the next departure waits the normal
// spacing plus flightExtraSpacing per extra jet, every jet gets a parking
// spot, and wingmen are not challenged as unknown traffic.

const (
	// flightMemory: a flight is forgotten this long after its last call.
	flightMemory = 90 * time.Minute
	// flightExtraSpacing is added to the departure spacing per jet after the lead.
	flightExtraSpacing = 20 * time.Second
	// flightJoinNm: an unnamed wingman is a player contact this close to the lead.
	flightJoinNm = 2.0
	maxFlightSize = 6
)

type towerFlight struct {
	lead     string   // "Raider 301"
	size     int      // jets including the lead
	wingmen  []string // "Raider 302", "Raider 303" (may be fewer than size-1)
	at       time.Time
	announce bool // the next reply says "flight of three"
}

var flightCountWords = map[string]int{"two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"2": 2, "3": 3, "4": 4, "5": 5, "6": 6}

// flightSizeRes: "flight of three" / "flight of 2" / "two-ship" / "three F-18s".
var flightSizeRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i),?\s*\b(?:a\s+|one\s+)?flight of (two|three|four|five|six|[2-6])\b(?:\s+(?:f/?a?[\s-]?1[68]s?|hornets?|vipers?|jets|ships|aircraft))?`),
	regexp.MustCompile(`(?i),?\s*\b(two|three|four|five|six|[2-6])[\s-]?ship\b`),
	regexp.MustCompile(`(?i),?\s*\b(two|three|four|five|six)\s+(?:f/?a?[\s-]?1[68]s|hornets|vipers|jets)\b`),
}

// flightMemberRe is one callsign in a list: "Raider 302", "302", "3-0-3".
var flightMemberRe = regexp.MustCompile(`(?i)(?:\b(raider|venom|viper)s?[\s,-]*)?\b(\d(?:[\s-]?\d){2})\b`)

// flightListGapRe is what may sit between two members of a list.
var flightListGapRe = regexp.MustCompile(`(?i)^[\s,.]*(?:(?:and|&|plus|with)[\s,]*)?$`)

// parseFlight reads the flight size and the wingmen named after the lead out
// of a call, and returns lower with those words taken out (so "flight of two"
// is not read as "two miles"). lead is the parsed callsign ("Raider 301").
func parseFlight(lower, lead string) (size int, wingmen []string, cleaned string) {
	cleaned = lower
	for _, re := range flightSizeRes {
		if m := re.FindStringSubmatchIndex(cleaned); m != nil {
			if n := flightCountWords[strings.ToLower(cleaned[m[2]:m[3]])]; n > size {
				size = n
			}
			cleaned = cleaned[:m[0]] + cleaned[m[1]:]
		}
	}
	leadNum := strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimLeft(lead, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz '"))
	word := strings.TrimSpace(strings.TrimRight(lead, "0123456789 -"))
	if leadNum != "" {
		ms := flightMemberRe.FindAllStringSubmatchIndex(cleaned, -1)
		for i, m := range ms {
			if digits(cleaned[m[4]:m[5]]) != leadNum {
				continue
			}
			end := m[1]
			seen := map[string]bool{leadNum: true}
			for _, n := range ms[i+1:] {
				if !flightListGapRe.MatchString(cleaned[end:n[0]]) {
					break
				}
				num := digits(cleaned[n[4]:n[5]])
				if !seen[num] {
					seen[num] = true
					w := word
					if n[2] >= 0 {
						w = strings.ToUpper(cleaned[n[2]:n[2]+1]) + strings.ToLower(cleaned[n[2]+1:n[3]])
					}
					wingmen = append(wingmen, strings.TrimSpace(w+" "+num))
				}
				end = n[1]
			}
			if len(wingmen) > 0 {
				cleaned = cleaned[:m[1]] + cleaned[end:]
			}
			break
		}
	}
	if len(wingmen) >= size {
		size = len(wingmen) + 1
	}
	if size > maxFlightSize {
		size = maxFlightSize
	}
	if size < 2 {
		return 0, nil, cleaned
	}
	return size, wingmen, cleaned
}

func digits(s string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(s)
}

// noteFlight records the flight a call names, and answers a wingman's call as
// his flight: req.Callsign becomes the lead's. It reports whether the call was
// a wingman's. Caller holds mu.
func (c *ATCController) noteFlight(req *ATCRequest) (wingman bool) {
	// Wingmen's own tower entries are dropped after flightsMu is released:
	// AirfieldState.ClearForTakeoff takes s.mu and then flightsMu
	// (DepartureExtra), so the order here must not be the reverse.
	var drop []string
	defer func() {
		for _, w := range drop {
			c.airfieldState.Remove(w)
		}
	}()
	c.flightsMu.Lock()
	defer c.flightsMu.Unlock()
	if c.flights == nil {
		c.flights = make(map[string]*towerFlight)
	}
	now := time.Now()
	for k, f := range c.flights {
		if now.Sub(f.at) > flightMemory {
			delete(c.flights, k)
		}
	}
	key := compactCallsign(req.Callsign)
	if req.FlightSize > 1 {
		f := c.flights[key]
		if f == nil || f.size != req.FlightSize || len(req.Wingmen) > len(f.wingmen) {
			nf := &towerFlight{lead: req.Callsign, size: req.FlightSize, wingmen: req.Wingmen, announce: true}
			if f != nil && len(req.Wingmen) == 0 {
				nf.wingmen = f.wingmen
				nf.announce = f.size != req.FlightSize
			}
			f = nf
			c.flights[key] = f
			// The wingmen fly as the lead now: drop any entry of their own.
			for _, w := range f.wingmen {
				delete(c.flights, compactCallsign(w))
				drop = append(drop, w)
			}
			log.Info().Str("lead", f.lead).Int("size", f.size).Strs("wingmen", f.wingmen).Msg("tower: multi-ship flight")
		}
		f.at = now
		return false
	}
	if f := c.flights[key]; f != nil {
		f.at = now
		return false
	}
	for _, f := range c.flights {
		for _, w := range f.wingmen {
			if compactCallsign(w) == key {
				log.Info().Str("wingman", req.Callsign).Str("lead", f.lead).Msg("tower: wingman answered as his flight")
				req.Callsign = f.lead
				f.at = now
				return true
			}
		}
	}
	return false
}

// flightOf returns the live flight led by callsign, or nil (single ship).
func (c *ATCController) flightOf(callsign string) *towerFlight {
	c.flightsMu.Lock()
	defer c.flightsMu.Unlock()
	f := c.flights[compactCallsign(callsign)]
	if f == nil || time.Since(f.at) > flightMemory {
		return nil
	}
	return f
}

// flightAddress turns "Raider 301, Senaki Tower, ..." into "Raider 301
// flight, Senaki Tower, ..." for a flight, adding "flight of three" after the
// speaker the first time the tower answers it.
func (c *ATCController) flightAddress(callsign, text string) string {
	if callsign == "" || len(text) <= len(callsign) || !strings.EqualFold(text[:len(callsign)], callsign) ||
		!strings.HasPrefix(text[len(callsign):], ",") {
		return text
	}
	c.flightsMu.Lock()
	defer c.flightsMu.Unlock()
	f := c.flights[compactCallsign(callsign)]
	if f == nil || time.Since(f.at) > flightMemory {
		return text
	}
	rest := text[len(callsign)+1:]
	if f.announce {
		f.announce = false
		of := ", flight of " + numberName(f.size) + ","
		if speaker, tail, ok := strings.Cut(rest, ","); ok &&
			containsAny(strings.ToLower(speaker), "tower", "ground", "traffic", "departure") {
			rest = speaker + of + tail
		} else {
			rest = of[1:] + rest
		}
	}
	return text[:len(callsign)] + " flight," + rest
}

func numberName(n int) string {
	for w, v := range flightCountWords {
		if v == n && len(w) > 1 {
			return w
		}
	}
	return "two"
}

// flightDepartureExtra is the extra departure spacing after callsign's takeoff
// clearance (AirfieldState.DepartureExtra).
func (c *ATCController) flightDepartureExtra(callsign string) time.Duration {
	if f := c.flightOf(callsign); f != nil {
		return time.Duration(f.size-1) * flightExtraSpacing
	}
	return 0
}

// flightParking gives every jet of callsign's flight a spot; nil for a single
// ship. names are the jets the spots go to ("" where a wingman was not named).
// Caller holds mu.
func (c *ATCController) flightParking(callsign string, now time.Time) (spots []int, names []string) {
	f := c.flightOf(callsign)
	if f == nil {
		return nil, nil
	}
	af := c.airfieldState.Airfield
	members := append([]string{f.lead}, f.wingmen...)
	for i := len(members); i < f.size; i++ {
		members = append(members, "") // unnamed wingman: a spot under the lead's name
	}
	for i, m := range members {
		key := m
		if key == "" {
			key = f.lead + " #" + numberName(i+1)
		}
		spot := c.assignParking(key, af, now)
		if spot == 0 {
			return nil, nil
		}
		spots = append(spots, spot)
		names = append(names, m)
	}
	return spots, names
}

// isFlightWingman: a player contact that is a wingman of a live flight -
// named in it, or flying within flightJoinNm of a lead whose flight has
// unnamed wingmen. Not challenged as unknown traffic. pt is its position.
// Caller holds allPositionsMu (read).
func (c *ATCController) isFlightWingman(callsign string, pt orb.Point) bool {
	c.flightsMu.Lock()
	defer c.flightsMu.Unlock()
	key := compactCallsign(callsign)
	for _, f := range c.flights {
		if time.Since(f.at) > flightMemory {
			continue
		}
		if compactCallsign(f.lead) == key {
			continue
		}
		for _, w := range f.wingmen {
			if compactCallsign(w) == key {
				return true
			}
		}
		if f.size > len(f.wingmen)+1 {
			if lc := c.lookupContact(f.lead); lc != nil && haversineNm(orb.Point{lc.Lon, lc.Lat}, pt) <= flightJoinNm {
				return true
			}
		}
	}
	return false
}
