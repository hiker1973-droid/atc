package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
)

// Range areas: the lettered target areas A-O on the training kneeboard's
// TARGET RANGES cards. A pilot tells Command where he is going ("Command,
// Raider 302, proceeding to Alpha") and Command answers with what is there,
// the threat, a bearing and range when Tacview has him, and anyone else
// already working it. The 9-line areas go to the JTAC that runs them.
// Operator 2026-10-16: "report for the non 9 line targets that pilot reports
// heading to Alpha (A on kneeboard) and Command responds with something".

// rangeArea is one lettered area.
type rangeArea struct {
	Letter string
	Word   string    // phonetic, as Command says it
	Pos    orb.Point // lon, lat: the centre of the area's objects
	Where  string    // spoken
	What   string    // spoken
	Threat string    // spoken radar SAM threat, "" = none
	// NineLine: the 9-line target numbers in the area. Command sends the
	// flight to the JTAC that runs them instead of describing it.
	NineLine []int
}

// caucasusRangeAreas mirrors TARGET RANGES 1/4 and 2/4 of the vSFG-7 Caucasus
// Training kneeboard, V1.25 (Day and Night are the same). Letters and
// positions are clustered from the .miz by _BUILD/targetcard.py; dumped from
// it 2026-10-16 - regenerate when the mission's ranges move. What and Threat
// are from the unit types, not the card: the card calls E (S-300PS) and G
// (S-75) "soft x5" and leaves M (Moskva, Neustrashimy) blank.
var caucasusRangeAreas = []rangeArea{
	{"A", "Alpha", orb.Point{40.0356, 44.6777}, "Maykop airfield", "SA-6 site and parked aircraft", "SA-6", nil},
	{"B", "Bravo", orb.Point{39.1824, 45.0420}, "Krasnodar Pashkovsky airfield", "SA-6 site and parked aircraft", "SA-6", nil},
	{"C", "Charlie", orb.Point{40.3713, 44.9498}, "22 miles north-east of Maykop", "SA-6 site, armour and infantry", "SA-6", nil},
	{"D", "Delta", orb.Point{41.8903, 43.9408}, "55 miles south-west of Mineralnye Vody", "", "", []int{1, 2}},
	{"E", "Echo", orb.Point{33.3000, 46.4656}, "191 miles north-west of Anapa", "SA-10 site", "SA-10", nil},
	{"F", "Foxtrot", orb.Point{38.7857, 44.8848}, "14 miles south-west of Krasnodar Center", "armour and infantry", "", nil},
	{"G", "Golf", orb.Point{37.9982, 45.1146}, "9 miles north of Krymsk", "SA-2 site", "SA-2", nil},
	{"H", "Hotel", orb.Point{42.1149, 44.1906}, "42 miles west of Mineralnye Vody", "", "", []int{4}},
	{"I", "India", orb.Point{41.9809, 44.3363}, "48 miles west of Mineralnye Vody", "", "", []int{5}},
	{"J", "Juliet", orb.Point{37.9035, 44.8381}, "9 miles south-west of Krymsk", "armour and trucks", "", nil},
	{"K", "Kilo", orb.Point{37.4317, 46.7378}, "104 miles north of Anapa", "carrier group, the Kuznetsov with two corvettes", "SA-N-9 and SA-15", nil},
	{"L", "Lima", orb.Point{39.9362, 45.3344}, "36 miles north-east of Krasnodar Pashkovsky", "a train", "", nil},
	{"M", "Mike", orb.Point{33.1125, 44.2232}, "187 miles west of Anapa", "surface group, the cruiser Moskva, a frigate and a tanker", "SA-N-6 and SA-N-9", nil},
	{"N", "November", orb.Point{37.5308, 45.1403}, "11 miles north-east of Anapa", "trucks", "", nil},
	{"O", "Oscar", orb.Point{42.8257, 41.6675}, "35 miles south-east of Kutaisi", "trucks and speedboats", "", nil},
}

func rangeAreasForMap(mapName string) []rangeArea {
	switch strings.ToLower(strings.TrimSpace(mapName)) {
	case "ca", "caucasus", "blacksea", "black-sea", "black sea":
		return caucasusRangeAreas
	}
	return nil
}

// rangeAreaHold: a flight that called an area counts as working it this long,
// or until it calls off.
const rangeAreaHold = 45 * time.Minute

var (
	// rangeAreaMoveRe: the pilot is on his way somewhere.
	rangeAreaMoveRe = regexp.MustCompile(`\b(proceeding|heading|en ?route|enroute|inbound|going|moving|pushing|tracking|vectoring|on the way|working|setting up)\b`)
	// rangeAreaWordRe: a phonetic letter after "to", "for", "area" etc.
	rangeAreaWordRe = regexp.MustCompile(`\b(?:to|for|towards?|area|target|range|inbound|working)\s+(?:the\s+)?(?:target\s+area\s+|area\s+|target\s+|range\s+)?(alpha|alfa|bravo|charlie|delta|echo|foxtrot|golf|hotel|india|juliet|juliett|kilo|lima|mike|november|oscar)\b`)
	// rangeAreaLetterRe: a bare letter only after "area", "target" or "range".
	rangeAreaLetterRe = regexp.MustCompile(`\b(?:area|target|range)\s+([a-o])\b`)
	// rangeAreaOffRe: done with an area ("off Alpha", "off target", "egressing area Bravo").
	rangeAreaOffRe = regexp.MustCompile(`\b(?:off|leaving|departing|egressing|clear of)\s+(?:the\s+)?(?:target\s+area\s+|area\s+|range\s+)?(target|alpha|alfa|bravo|charlie|delta|echo|foxtrot|golf|hotel|india|juliet|juliett|kilo|lima|mike|november|oscar)\b`)
)

// rangeAreaIn finds the area a call names, -1 for none.
func rangeAreaIn(areas []rangeArea, lower string) int {
	var key string
	if m := rangeAreaWordRe.FindStringSubmatch(lower); m != nil {
		key = m[1]
	} else if m := rangeAreaLetterRe.FindStringSubmatch(lower); m != nil {
		key = m[1]
	} else {
		return -1
	}
	key = strings.NewReplacer("alfa", "alpha", "juliett", "juliet").Replace(key)
	for i, a := range areas {
		if strings.EqualFold(a.Word, key) || strings.EqualFold(a.Letter, key) {
			return i
		}
	}
	return -1
}

// rangeAreaBoard remembers which flight called which area, for "off" and to
// tell the next flight who is already there.
type rangeAreaBoard struct {
	mu      sync.Mutex
	areas   []rangeArea
	working map[string]rangeWork // callsign key -> area
	now     func() time.Time
	// toJTAC hands a 9-line area to the JTAC that runs it; nil = no JTAC.
	toJTAC func(key string, numbers []int) *jtacNet
}

type rangeWork struct {
	area int
	at   time.Time
}

var (
	commandRangeAreas     *rangeAreaBoard
	commandRangeAreasOnce sync.Once
)

func commandRangeAreaBoard() *rangeAreaBoard {
	commandRangeAreasOnce.Do(func() {
		commandRangeAreas = &rangeAreaBoard{areas: rangeAreasForMap(flagMap), working: make(map[string]rangeWork), now: time.Now,
			toJTAC: func(key string, numbers []int) *jtacNet { return commandTaskingBoard().handOffTarget(key, numbers) }}
	})
	return commandRangeAreas
}

// Response answers a pilot calling an area or calling off one. pic gives the
// bearing and range when Tacview has the caller; nil is no picture.
func (r *rangeAreaBoard) Response(text, callsign, channelName string, pic jtacPicture, magVar magVarFn) string {
	if len(r.areas) == 0 {
		return ""
	}
	lower := strings.ToLower(text)
	key := strings.ToLower(strings.TrimSpace(callsign))
	if key == "" {
		return ""
	}
	say := func(format string, a ...interface{}) string {
		return fmt.Sprintf("%s, %s, ", callsign, channelName) + fmt.Sprintf(format, a...)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()

	if m := rangeAreaOffRe.FindStringSubmatch(lower); m != nil {
		w, ok := r.working[key]
		if !ok || now.Sub(w.at) > rangeAreaHold {
			return ""
		}
		a := r.areas[w.area]
		if m[1] != "target" && rangeAreaIn(r.areas, "to "+m[1]) != w.area {
			return ""
		}
		delete(r.working, key)
		return say("copy off %s.", a.Word)
	}

	if !rangeAreaMoveRe.MatchString(lower) {
		return ""
	}
	i := rangeAreaIn(r.areas, lower)
	if i < 0 {
		return ""
	}
	a := r.areas[i]

	if len(a.NineLine) > 0 {
		nums := make([]string, len(a.NineLine))
		for j, n := range a.NineLine {
			nums[j] = fmt.Sprint(n)
		}
		what := "nine-line target " + nums[0]
		if len(nums) > 1 {
			what = "nine-line targets " + strings.Join(nums[:len(nums)-1], ", ") + " and " + nums[len(nums)-1]
		}
		var n *jtacNet
		if r.toJTAC != nil {
			n = r.toJTAC(key, a.NineLine)
		}
		if n == nil {
			return say("%s is %s. Request tasking.", a.Word, what)
		}
		return say("%s is %s. Contact %s on %s for tasking.", a.Word, what, n.Spoken, formatFreqMHz(n.FreqMHz))
	}

	parts := []string{say("copy %s, %s, %s.", a.Word, a.Where, a.What)}
	if a.Threat != "" {
		parts = append(parts, fmt.Sprintf("Threat %s.", a.Threat))
	} else {
		parts = append(parts, "No radar threat.")
	}
	if pic != nil {
		if pos, _, ok := pic.Contact(callsign); ok {
			brg, nm := bearingRangeFromRef(pos, a.Pos[0], a.Pos[1])
			parts = append(parts, fmt.Sprintf("%s bears %s for %d.", a.Word, spokenBearing(toMagnetic(brg, magVar(pos))), int(nm+0.5)))
		}
	}
	// Who else is there, so flights deconflict before they run in.
	var others []string
	for k, w := range r.working {
		if k != key && w.area == i && now.Sub(w.at) <= rangeAreaHold {
			others = append(others, displayCallsign(k))
		}
	}
	if len(others) > 0 {
		parts = append(parts, fmt.Sprintf("Be advised, %s working %s.", strings.Join(others, " and "), a.Word))
	}
	parts = append(parts, "Report off target.")
	r.working[key] = rangeWork{area: i, at: now}
	return strings.Join(parts, " ")
}

// rangeAreaWhisperWords: the area names for Command's Whisper prompt.
const rangeAreaWhisperWords = "proceeding to Alpha, Bravo, Charlie, Delta, Echo, Foxtrot, Golf, Hotel, India, Juliet, Kilo, Lima, Mike, November, Oscar, off target"
