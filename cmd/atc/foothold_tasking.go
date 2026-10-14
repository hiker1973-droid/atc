package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
)

// Foothold tasking: on the Syria Foothold campaign, Command tasks a flight
// against an enemy zone from the mission's own save file (foothold_save.go)
// instead of a fixed 9-line set. It picks a red zone on the front line - the
// red zones closest to blue ground - preferring the one nearest the caller,
// and gives its bearing and range and the SAMs still alive in it:
//
//	"Raider 32, Command, tasking Bassel Al-Assad, bearing zero-four-five,
//	 38 miles. Threats SA-6, SA-8, triple-A."
//	"copy"  -> "Raider 32, Command, Bassel Al-Assad, cleared hot."
//
// There is no JTAC on Foothold, so Command clears the flight itself. A flight
// asking again gets a different zone where there is one. Operator 2026-10-14.

const (
	// footholdFrontCount is how many front-line zones the pick chooses from.
	footholdFrontCount = 3
	footholdSideRed    = 1
	footholdSideBlue   = 2
)

type footholdAssignment struct {
	zone     string
	issued   time.Time
	awaiting bool
}

type footholdTasking struct {
	save *footholdSave
	now  func() time.Time

	mu      sync.Mutex
	flights map[string]*footholdAssignment
}

var (
	footholdBoard     *footholdTasking
	footholdBoardOnce sync.Once
)

// footholdTaskingBoard is the board when Command runs on Syria, nil elsewhere.
func footholdTaskingBoard() *footholdTasking {
	footholdBoardOnce.Do(func() {
		if !strings.EqualFold(strings.TrimSpace(flagMap), "syria") || !flagFootholdTasking {
			return
		}
		footholdBoard = newFootholdTasking(&footholdSave{paths: footholdSavePaths})
		if zones, err := footholdBoard.save.Zones(); err != nil {
			log.Warn().Err(err).Msg("Foothold tasking: save file not readable yet - Command answers negative tasking until it is")
		} else {
			log.Info().Str("file", footholdBoard.save.path).Int("zones", len(zones)).Msg("Foothold tasking: reading the campaign save")
		}
	})
	return footholdBoard
}

func newFootholdTasking(save *footholdSave) *footholdTasking {
	return &footholdTasking{save: save, now: time.Now, flights: make(map[string]*footholdAssignment)}
}

// Response answers a tasking request or its readback. "" for any other call.
// store may be nil (no Tacview): the zone is then named without a bearing.
func (f *footholdTasking) Response(text, callsign, channelName string, store *tacviewPositions) string {
	lower := strings.ToLower(text)
	key := strings.ToLower(strings.TrimSpace(callsign))
	if key == "" {
		return ""
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	cur := f.flights[key]

	if isTaskingRequest(lower) {
		zones, err := f.save.Zones()
		if err != nil {
			log.Warn().Err(err).Msg("Foothold tasking: save file")
		}
		var pos orb.Point
		havePos := false
		if store != nil {
			pos, havePos = store.Get(callsign)
		}
		last := ""
		if cur != nil {
			last = cur.zone
		}
		z, ok := pickFootholdZone(zones, f.held(key, now), last, pos, havePos)
		if !ok {
			return fmt.Sprintf("%s, %s, negative tasking at this time.", callsign, channelName)
		}
		f.flights[key] = &footholdAssignment{zone: z.Name, issued: now, awaiting: true}
		log.Info().Str("callsign", callsign).Str("zone", z.Name).Int("units", len(z.Units)).Msg("Foothold tasking: zone assigned")
		resp := fmt.Sprintf("%s, %s, tasking %s", callsign, channelName, spokenZone(z.Name))
		if havePos {
			brg, nm := bearingRangeFromRef(pos, z.Pos[0], z.Pos[1])
			resp += fmt.Sprintf(", bearing %s, %d miles", spokenBearing(toMagnetic(brg, nearestFieldMagVar(pos))), int(nm+0.5))
		}
		if th := footholdThreats(z.Units); len(th) > 0 {
			resp += ". Threats " + strings.Join(th, ", ") + "."
		} else {
			resp += ". No SAMs reported."
		}
		return resp
	}

	if cur == nil || !cur.awaiting || now.Sub(cur.issued) > taskingAckWindow {
		return ""
	}
	if isTaskingAck(lower) || strings.Contains(lower, strings.ToLower(cur.zone)) {
		cur.awaiting = false
		return fmt.Sprintf("%s, %s, %s, cleared hot.", callsign, channelName, spokenZone(cur.zone))
	}
	return ""
}

// held is the zones other flights were given inside taskingHoldTime.
func (f *footholdTasking) held(key string, now time.Time) map[string]bool {
	h := map[string]bool{}
	for k, a := range f.flights {
		if k != key && now.Sub(a.issued) < taskingHoldTime {
			h[a.zone] = true
		}
	}
	return h
}

// pickFootholdZone chooses the zone to task: an active red zone with units
// left and a position, among the footholdFrontCount closest to blue ground,
// skipping zones other flights hold and the caller's last one where it can.
// With the caller's position the nearest of those wins, else the most
// forward. Falls back to any red zone when there is no blue zone at all.
func pickFootholdZone(zones []footholdZone, held map[string]bool, last string, pos orb.Point, havePos bool) (footholdZone, bool) {
	var red, blue []footholdZone
	for _, z := range zones {
		if !z.Active || z.Pos == (orb.Point{}) {
			continue
		}
		switch z.Side {
		case footholdSideRed:
			if len(z.Units) > 0 {
				red = append(red, z)
			}
		case footholdSideBlue:
			blue = append(blue, z)
		}
	}
	if len(red) == 0 {
		return footholdZone{}, false
	}
	front := func(z footholdZone) float64 {
		best := math.MaxFloat64
		for _, b := range blue {
			if d := haversineNm(z.Pos, b.Pos); d < best {
				best = d
			}
		}
		return best
	}
	sort.SliceStable(red, func(i, j int) bool { return front(red[i]) < front(red[j]) })

	choose := func(skip func(footholdZone) bool) (footholdZone, bool) {
		var cands []footholdZone
		for _, z := range red {
			if !skip(z) {
				cands = append(cands, z)
				if len(cands) == footholdFrontCount {
					break
				}
			}
		}
		if len(cands) == 0 {
			return footholdZone{}, false
		}
		if havePos {
			sort.SliceStable(cands, func(i, j int) bool {
				return haversineNm(pos, cands[i].Pos) < haversineNm(pos, cands[j].Pos)
			})
		}
		return cands[0], true
	}
	if z, ok := choose(func(z footholdZone) bool { return held[z.Name] || z.Name == last }); ok {
		return z, true
	}
	if z, ok := choose(func(z footholdZone) bool { return held[z.Name] }); ok {
		return z, true
	}
	return choose(func(footholdZone) bool { return false })
}

// spokenZone reads a zone name aloud: underscores and dashes as spaces.
func spokenZone(name string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(name)), " ")
}

// footholdThreatTypes names the air defences a pilot cares about, by DCS type
// name, longest range first. Types not listed (trucks, armour, radars of no
// interest on their own) are not called out.
var footholdThreatTypes = []struct {
	name  string
	types []string
}{
	{"SA-10", []string{"S-300PS 5P85C ln", "S-300PS 5P85D ln", "S-300PS 40B6M tr"}},
	{"SA-5", []string{"S-200_Launcher", "RPC_5N62V"}},
	{"Patriot", []string{"Patriot ln", "Patriot str"}},
	{"SA-2", []string{"S_75M_Volhov", "SNR_75V"}},
	{"SA-11", []string{"SA-11 Buk LN 9A310M1"}},
	{"Hawk", []string{"Hawk ln", "Hawk tr"}},
	{"NASAMS", []string{"NASAMS_LN_C", "NASAMS_LN_B"}},
	{"IRIS-T", []string{"CHAP_IRISTSLM_LN"}},
	{"SA-3", []string{"5p73 s-125 ln", "snr s-125 tr"}},
	{"SA-6", []string{"Kub 2P25 ln", "Kub 1S91 str"}},
	{"SA-22", []string{"CHAP_PantsirS1"}},
	{"SA-15", []string{"Tor 9A331", "CHAP_TorM2"}},
	{"SA-8", []string{"Osa 9A33 ln"}},
	{"HQ-7", []string{"HQ-7_LN_SP", "HQ-7_STR_SP"}},
	{"Roland", []string{"Roland ADS"}},
	{"SA-19", []string{"2S6 Tunguska"}},
	{"SA-13", []string{"Strela-10M3"}},
	{"SA-9", []string{"Strela-1 9P31"}},
	{"Avenger", []string{"M1097 Avenger"}},
	{"Linebacker", []string{"M6 Linebacker"}},
	{"Gepard", []string{"Gepard"}},
	{"Shilka", []string{"ZSU-23-4 Shilka"}},
	{"triple-A", []string{"ZSU_57_2", "ZU-23 Emplacement Closed", "ZU-23 Emplacement", "Ural-375 ZU-23",
		"Ural-375 ZU-23 Insurgent", "HL_ZU-23", "tt_ZU-23", "KS-19", "S-60_Type59_Artillery", "Vulcan"}},
	{"MANPADS", []string{"SA-18 Igla manpad", "SA-18 Igla-S manpad", "Igla manpad INS", "Soldier stinger"}},
}

// footholdThreats lists the air defences among units, most dangerous first.
func footholdThreats(units []string) []string {
	have := map[string]bool{}
	for _, u := range units {
		have[u] = true
	}
	var out []string
	for _, t := range footholdThreatTypes {
		for _, ty := range t.types {
			if have[ty] {
				out = append(out, t.name)
				break
			}
		}
	}
	return out
}
