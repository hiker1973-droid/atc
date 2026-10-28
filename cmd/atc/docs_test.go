package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/parser"
	"github.com/vsfg7/atc/pkg/lso"
)

// The pilot documentation (docs/phraseology, the ATC Phraseology Reference)
// is generated, and these tests keep it honest:
//
//   - TestDocFacts: the frequencies the document prints are read out of the
//     start_*.bat files and SkyEye's own tables into docs/phraseology/
//     facts.json. The test fails when that file no longer matches the code.
//     To refresh it:  UPDATE_DOC_FACTS=1 go test ./cmd/atc -run TestDocFacts
//     then rebuild the PDF (docs/phraseology/README.md).
//   - TestDocPhrases: every pilot call the document prints (docs/phraseology/
//     phrases.json, written by the builder) must still trigger what the
//     document says it triggers.

const docsDir = "../../docs/phraseology"

type docStation struct {
	Name    string  `json:"name"`
	ICAO    string  `json:"icao"`
	FreqMHz float64 `json:"freq"`
}

type docTheatre struct {
	Key      string       `json:"key"`
	Title    string       `json:"title"`
	Command  float64      `json:"command"`
	JTAC     float64      `json:"jtac,omitempty"`
	JTAC2    float64      `json:"jtac2,omitempty"`
	Deckboss float64      `json:"deckboss,omitempty"`
	Marshal  float64      `json:"marshal,omitempty"`
	LSO      float64      `json:"lso,omitempty"`
	Tanker   float64      `json:"tanker,omitempty"`
	Tanker2  float64      `json:"tanker2,omitempty"`
	Tankers  []docTanker  `json:"tankers,omitempty"` // --tanker-net: Texaco 2-1, the Shells
	GCI      float64      `json:"gci,omitempty"`
	GCIName  string       `json:"gci_name,omitempty"`
	Towers   []docStation `json:"towers"`
	ATIS     []docStation `json:"atis"`
}

// docTanker is one --tanker-net net.
type docTanker struct {
	Name    string  `json:"name"` // "Shell 1-1"
	FreqMHz float64 `json:"freq"`
	Boom    bool    `json:"boom,omitempty"`
}

// docTheatres lists the theatres the document covers and their start scripts.
var docTheatres = []struct{ key, title, suffix, atisMap string }{
	{"pg", "Persian Gulf", "", "pg"},
	{"caucasus", "Black Sea (Caucasus)", "_caucasus", "caucasus"},
	{"syria", "Syria (Eastern Med)", "_syria", "syria"},
	{"germany", "Cold War Germany", "_germany", "germany"},
	{"iraq", "Iraq", "_iraq", "iraq"},
	{"kola", "Kola", "_kola", "kola"},
}

// batLaunches returns the atc.exe command lines a start script actually runs
// (commented-out lines skipped), with the window title.
func batLaunches(t *testing.T, name string) (lines []string, titles []string) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	title := regexp.MustCompile(`start "([^"]+)"`)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		low := strings.ToLower(l)
		if strings.HasPrefix(l, "::") || strings.HasPrefix(low, "rem ") || !strings.Contains(l, "atc.exe") {
			continue
		}
		lines = append(lines, l)
		m := title.FindStringSubmatch(l)
		if len(m) > 1 {
			titles = append(titles, m[1])
		} else {
			titles = append(titles, "")
		}
	}
	return lines, titles
}

func batFlag(line, flag string) string {
	m := regexp.MustCompile(`--` + regexp.QuoteMeta(flag) + `[ =](\S+)`).FindStringSubmatch(line)
	if len(m) < 2 {
		return ""
	}
	return strings.Trim(m[1], `"`)
}

func batFreq(t *testing.T, name, flag string) float64 {
	lines, _ := batLaunches(t, name)
	for _, l := range lines {
		if v := batFlag(l, flag); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err == nil && f > 0 {
				return f
			}
		}
	}
	return 0
}

func currentDocFacts(t *testing.T) []docTheatre {
	var out []docTheatre
	for _, th := range docTheatres {
		// Paddles and GCI came to PG after the other theatres, as
		// start_lso_pg.bat / start_gci_pg.bat beside its unsuffixed scripts.
		newer := th.suffix
		if newer == "" {
			newer = "_" + th.key
		}
		d := docTheatre{Key: th.key, Title: th.title,
			Command:  batFreq(t, "start_command"+th.suffix+".bat", "command-freq"),
			JTAC:     batFreq(t, "start_command"+th.suffix+".bat", "jtac-freq"),
			JTAC2:    batFreq(t, "start_command"+th.suffix+".bat", "jtac2-freq"),
			Deckboss: batFreq(t, "start_deckboss"+th.suffix+".bat", "deckboss-freq"),
			Marshal:  batFreq(t, "start_marshal"+th.suffix+".bat", "marshal-freq"),
			LSO:      batFreq(t, "start_lso"+newer+".bat", "lso-freq"),
			Tanker:   batFreq(t, "start_command"+th.suffix+".bat", "tanker-freq"),
			Tanker2:  batFreq(t, "start_command"+th.suffix+".bat", "tanker2-freq"),
			GCI:      batFreq(t, "start_gci"+newer+".bat", "gci-freq"),
		}
		clines, _ := batLaunches(t, "start_command"+th.suffix+".bat")
		for _, l := range clines {
			for _, m := range regexp.MustCompile(`--tanker-net[ =](\S+)`).FindAllStringSubmatch(l, -1) {
				cfg, err := parseTankerNet(strings.Trim(m[1], `"`))
				if err != nil {
					t.Errorf("%s: %v", th.key, err)
					continue
				}
				d.Tankers = append(d.Tankers, docTanker{Name: cfg.Tacview, FreqMHz: cfg.FreqMHz, Boom: cfg.Boom})
			}
		}
		if d.GCI > 0 {
			lines, _ := batLaunches(t, "start_gci"+newer+".bat")
			for _, l := range lines {
				if v := batFlag(l, "gci-callsign"); v != "" {
					d.GCIName = v
				}
			}
		}
		lines, titles := batLaunches(t, "start_towers"+th.suffix+".bat")
		for i, l := range lines {
			icao := batFlag(l, "airfield")
			af := airfield.ByICAO(icao)
			if af == nil {
				t.Errorf("%s: unknown airfield %q", th.key, icao)
				continue
			}
			freq := af.TowerFreqMHz
			if v := batFlag(l, "freq"); v != "" {
				freq, _ = strconv.ParseFloat(v, 64)
			}
			d.Towers = append(d.Towers, docStation{Name: titles[i], ICAO: af.ICAO, FreqMHz: freq})
		}
		alines, _ := batLaunches(t, "start_atis"+th.suffix+".bat")
		var only map[string]bool
		for _, l := range alines {
			if v := batFlag(l, "atis-stations"); v != "" {
				only = map[string]bool{}
				for _, s := range strings.Split(v, ",") {
					only[strings.ToUpper(strings.TrimSpace(s))] = true
				}
			}
		}
		if len(alines) > 0 {
			for _, st := range atisStationsForMap(th.atisMap) {
				if only == nil || only[strings.ToUpper(st.ICAO)] {
					d.ATIS = append(d.ATIS, docStation{Name: st.Name, ICAO: st.ICAO, FreqMHz: st.FreqMHz})
				}
			}
		}
		sort.SliceStable(d.Towers, func(i, j int) bool { return d.Towers[i].Name < d.Towers[j].Name })
		sort.SliceStable(d.ATIS, func(i, j int) bool { return d.ATIS[i].Name < d.ATIS[j].Name })
		out = append(out, d)
	}
	return out
}

func TestDocFacts(t *testing.T) {
	got, err := json.MarshalIndent(currentDocFacts(t), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join(docsDir, "facts.json")
	if os.Getenv("UPDATE_DOC_FACTS") != "" {
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s - rebuild the PDF", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (UPDATE_DOC_FACTS=1 go test ./cmd/atc -run TestDocFacts)", err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != string(got) {
		t.Errorf("docs/phraseology/facts.json is stale: the frequencies in the code changed.\n" +
			"Run  UPDATE_DOC_FACTS=1 go test ./cmd/atc -run TestDocFacts  and rebuild the PDF.")
	}
}

// docPhrase is one printed pilot call and what the document says it does.
type docPhrase struct {
	Role  string `json:"role"`  // tower | command | jtac | marshal | deckboss | tanker | lso | gci
	Say   string `json:"say"`   // as printed, with the address
	Tower string `json:"tower"` // tower calls: the tower addressed; gci: the GCI callsign
	Want  string `json:"want"`  // the intent the document promises
}

var towerIntents = map[string]controller.RequestType{
	"taxi": controller.RequestTaxiClear, "holding_short": controller.RequestHoldingShort,
	"takeoff": controller.RequestTakeoffClear, "airborne": controller.RequestClearTraffic,
	"rolling": controller.RequestRolling, "dme": controller.RequestDistanceCheck,
	"pushing_command": controller.RequestPushingCommand, "startup": controller.RequestStartup,
	"inbound": controller.RequestDistanceInitial, "overhead": controller.RequestOverhead,
	"break": controller.RequestBreak, "downwind": controller.RequestDownwind, "base": controller.RequestBase,
	"final": controller.RequestLandingClear, "straight_in": controller.RequestStraightIn,
	"go_around": controller.RequestGoAround, "vacated": controller.RequestRunwayVacated,
	"say_again": controller.RequestSayAgain, "wind_check": controller.RequestWindCheck,
	"radar_check": controller.RequestRadarCheck, "radio_check": controller.RequestRadioCheck,
	"hung": controller.RequestHungOrdnance, "emergency": controller.RequestEmergency,
	"souls_fuel": controller.RequestSoulsFuel,
}

// commandAttackReplies: the attack calls Command copies when nothing else
// answered (commandShotResponse), and a word each reply must carry.
var commandAttackReplies = map[string]*regexp.Regexp{
	"in_hot":      regexp.MustCompile(`cleared hot`),
	"release":     regexp.MustCompile(`(?i)pickle|rifle`),
	"shot":        regexp.MustCompile(`(?i)fox|splash`),
	"attack_move": regexp.MustCompile(`(?i)copy|roger`),
}

// lsoDocKind is what Paddles did with a printed call: the ball call and red
// deck are handled with no text reply (the ball call goes to the TX queue).
func lsoDocKind(f *lsoFeed, said *[]lso.Call, say string) string {
	*said = nil
	_, resp := f.heard(say)
	switch {
	case resp == netHandled && len(*said) == 1 && (*said)[0] == lso.CallRogerBall:
		return "ball"
	case resp == netHandled && len(*said) == 1 && (*said)[0] == lso.CallClara:
		return "clara"
	case resp == netHandled && len(*said) == 0:
		return "red_deck"
	case strings.Contains(resp, "roger green deck"):
		return "green_deck"
	case strings.Contains(resp, "call the ball at three-quarters"):
		return "check_in"
	case strings.Contains(resp, "call the ball"):
		return "abeam"
	case strings.Contains(resp, "call abeam"):
		return "break"
	case strings.Contains(resp, "loud and clear"):
		return "radio_check"
	}
	return resp
}

var marshalIntents = map[string]marshalIntent{
	"radio_check": marshalRadioCheck, "radar_check": marshalRadarCheck, "brc": marshalBRCRequest,
	"marking_moms": marshalMarkingMom, "see_you_at_ten": marshalSeeYouAtTen, "clear": marshalDepartureClear,
	"dme": marshalDME, "state": marshalState, "established": marshalEstablished, "commencing": marshalCommencing,
	"platform": marshalPlatform, "pushing_command": marshalPushCommand, "initial": marshalInitial,
	"check_in": marshalCheckIn, "needles": marshalNeedles, "coupled": marshalACLSReport,
	"push_time": marshalPushTime,
}

func jtacKind(lower string) string {
	switch {
	case containsAny(lower, "radio check"):
		return "radio_check"
	case isJTACCheckIn(lower):
		return "check_in"
	case isBDARequest(lower):
		return "bda"
	case jtacAbortRe.MatchString(lower):
		return "abort"
	case jtacOffRe.MatchString(lower):
		return "off"
	case isJTACInbound(lower):
		return "ip_inbound"
	case jtacInRe.MatchString(lower):
		return "in"
	case isTaskingAck(lower) || isTargetReadback(lower, docReadbackTarget):
		return "copy"
	}
	return ""
}

// gciDocKind is what the GCI (gci.go runGCI) does with a printed call:
// checking out, fence in / out and splash are ours (gciCheckOut, gciAck);
// everything else goes through SkyEye's parser after the fast / slow movers
// rewrite (gciAliases).
func gciDocKind(say, gci string) string {
	if gciCheckOut(say, gci) != "" {
		return "check_out"
	}
	if kind, _ := gciAck(say, gci); kind != "" {
		return kind
	}
	switch req := parser.New(gci, nil, false).Parse(gciAliases(say)).(type) {
	case *brevity.RadioCheckRequest:
		return "radio_check"
	case *brevity.CheckInRequest:
		return "check_in"
	case *brevity.AlphaCheckRequest:
		return "alpha"
	case *brevity.PictureRequest:
		return "picture"
	case *brevity.BogeyDopeRequest:
		switch req.Filter {
		case brevity.FixedWing:
			return "bogey_fast"
		case brevity.RotaryWing:
			return "bogey_slow"
		}
		return "bogey"
	case *brevity.DeclareRequest:
		return "declare"
	case *brevity.SnaplockRequest:
		return "snaplock"
	case *brevity.SpikedRequest, *brevity.StrobeRequest:
		return "spiked" // printed on one row: same answer
	case *brevity.VectorRequest:
		return "vector"
	case nil:
		return "not understood"
	default:
		return fmt.Sprintf("%T", req)
	}
}

// docReadbackTarget is the card target the Phraseology Reference's Hitman
// readback example names ("target 4, Indigo"; training V1.25 north range).
var docReadbackTarget = taskingTarget{IP: "Indigo", Number: 4, Name: "Fuel Depot"}

func TestDocPhrases(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(docsDir, "phrases.json"))
	if err != nil {
		t.Fatalf("%v - run the builder (docs/phraseology/README.md)", err)
	}
	var phrases []docPhrase
	if err := json.Unmarshal(raw, &phrases); err != nil {
		t.Fatal(err)
	}
	if len(phrases) < 40 {
		t.Fatalf("only %d phrases - the builder wrote a short list", len(phrases))
	}
	// Texaco: the printed flow runs in order through one tanker net, and each
	// reply must carry the words the document gives (Want).
	tanker := newTankerNet("Texaco one one", func() int { return 13 }, func() int { return 260 })
	tanker.pick = func(o [3]string) string { return o[0] }
	var lsoSaid []lso.Call
	paddles := newLSOFeed(lso.NimitzParams(), func(c lso.Call) { lsoSaid = append(lsoSaid, c) })
	for _, p := range phrases {
		lower := strings.ToLower(p.Say)
		fail := func(got interface{}) { t.Errorf("%s %q: got %v, the document says %s", p.Role, p.Say, got, p.Want) }
		switch p.Role {
		case "tower":
			want, ok := towerIntents[p.Want]
			if !ok {
				t.Errorf("tower intent %q not mapped in docs_test.go", p.Want)
				continue
			}
			req := controller.ParseIntent(p.Say, p.Tower)
			if req == nil {
				fail("not addressed")
			} else if req.Type != want {
				fail(fmt.Sprint(req.Type))
			}
		case "marshal":
			want, ok := marshalIntents[p.Want]
			if !ok {
				t.Errorf("marshal intent %q not mapped in docs_test.go", p.Want)
				continue
			}
			if got := classifyMarshalCall(lower, extractFuelStateMarshal(lower)); got != want {
				fail(got)
			}
		case "jtac":
			if got := jtacKind(lower); got != p.Want {
				fail(got)
			}
		case "command":
			cs := extractCallsignSkippingAddress(p.Say, "command")
			var got string
			switch p.Want {
			case "tasking":
				if isTaskingRequest(lower) {
					got = "tasking"
				}
			case "bullseye":
				if isBullseyeRequest(lower) {
					got = "bullseye"
				}
			case "tanker":
				if isTankerRequest(lower) {
					got = "tanker"
				}
			// Range areas and the run close-out (command_areas.go,
			// commandBrevityResponse): parsed apart from commandResponse.
			case "area", "overlord":
				body := strings.Replace(lower, strings.ToLower(cs), "", 1)
				if rangeAreaIn(caucasusRangeAreas, body) >= 0 {
					switch {
					case rangeAreaPermissionRe.MatchString(body):
						got = "overlord"
					case rangeAreaMoveRe.MatchString(body):
						got = "area"
					}
				}
			case "off_target":
				if rangeAreaOffRe.MatchString(lower) || rangeAreaOffAnyRe.MatchString(lower) {
					got = "off_target"
				}
			case "in_hot", "release", "shot", "attack_move":
				if r := commandReply(p.Say, cs, "Command", newTacviewPositions()); commandAttackReplies[p.Want].MatchString(r) {
					got = p.Want
				} else {
					got = r
				}
			case "winchester", "bingo", "joker":
				switch {
				case winchesterRe.MatchString(lower):
					got = "winchester"
				case bingoRe.MatchString(lower):
					got = "bingo"
				case jokerRe.MatchString(lower):
					got = "joker"
				}
			default:
				if commandResponse(p.Say, cs, "Command") != "" {
					got = p.Want
				}
			}
			if got != p.Want {
				fail(got)
			}
		case "tanker":
			if r := tanker.Response(p.Say, tankerCallsign(p.Say)); !strings.Contains(r, p.Want) {
				fail(fmt.Sprintf("%q", r))
			}
		case "lso":
			if got := lsoDocKind(paddles, &lsoSaid, p.Say); got != p.Want {
				fail(fmt.Sprintf("%q", got))
			}
		case "gci":
			gci := p.Tower
			if gci == "" {
				gci = "Darkstar"
			}
			if got := gciDocKind(p.Say, gci); got != p.Want {
				fail(got)
			}
		case "deckboss":
			// Deckboss parses inline in deckboss.go; its phrases are checked by
			// hand (docs/phraseology/README.md).
		default:
			t.Errorf("unknown role %q", p.Role)
		}
	}
}
