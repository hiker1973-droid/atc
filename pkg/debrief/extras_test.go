package debrief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCallsignIn(t *testing.T) {
	for in, want := range map[string]string{
		"Mozdok Tower, Raider 302, request taxi.":                       "Raider 302",
		"Union Marshal, Raider 302, mark your mom's 078 for 65":         "Raider 302",
		"Texaco 3-1, one flight of F-18, Raider 3-0-2, 25 miles":        "Raider 302",
		"Senior Marshal, reader 302, 5.7.":                              "Reader 302",
		"Hitman 1-1, Venom 1-1, checking in":                            "Venom 1-1",
		"Raider 302, Union Marshal, radar contact, angels 25, range 68": "Raider 302",
		"Union Marshal, angels 25, state 6.5":                           "",
	} {
		if got := CallsignIn(in); got != want {
			t.Errorf("CallsignIn(%q) = %q, want %q", in, got, want)
		}
	}
	if CallsignKey("reader 302") != CallsignKey("Raider 3-0-2") || CallsignKey("Venom 1-1") != "venom11" {
		t.Fatal("keys")
	}
}

// A carrier session: Hiker (Raider 302) taxis at Mozdok, tanks twice on one
// visit, bolters then traps on CVN-71; Dabs crashes on his own.
const extrasLog = `2026-10-24 18:00:00.000 INFO    Scripting (Main): event:type=mission start,event_id=1,t=0,linked_event_id=0,
2026-10-24 18:01:00.000 INFO    Scripting (Main): event:type=takeoff,initiatorPilotName=RAIDER 302 | HIKER,place=Mozdok,t=60,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=2,linked_event_id=0,
2026-10-24 18:01:40.000 INFO    Scripting (Main): event:type=takeoff,initiatorPilotName=VENOM 1 | DABS,place=Senaki-Kolkhi,t=100,initiator_unit_type=F-16C_50,initiator_coalition=2,initiator_ws_type1=1,event_id=3,linked_event_id=0,
2026-10-24 18:15:00.000 INFO    Scripting (Main): event:type=refuel,initiatorPilotName=RAIDER 302 | HIKER,t=900,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=4,linked_event_id=0,
2026-10-24 18:16:00.000 INFO    Scripting (Main): event:type=refuel stop,initiatorPilotName=RAIDER 302 | HIKER,t=960,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=5,linked_event_id=0,
2026-10-24 18:16:20.000 INFO    Scripting (Main): event:type=refuel,initiatorPilotName=RAIDER 302 | HIKER,t=980,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=6,linked_event_id=0,
2026-10-24 18:18:20.000 INFO    Scripting (Main): event:type=refuel stop,initiatorPilotName=RAIDER 302 | HIKER,t=1100,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=7,linked_event_id=0,
2026-10-24 18:25:00.000 INFO    Scripting (Main): event:type=crash,initiatorPilotName=VENOM 1 | DABS,t=1500,initiator_unit_type=F-16C_50,initiator_coalition=2,initiator_ws_type1=1,event_id=8,linked_event_id=0,
2026-10-24 18:30:00.000 INFO    Scripting (Main): event:type=landing quality mark,initiatorPilotName=RAIDER 302 | HIKER,place=CVN-71,t=1800,initiator_unit_type=FA-18C_hornet,comment=LSO: GRADE:B,initiator_coalition=2,event_id=9,linked_event_id=0,
2026-10-24 18:31:40.000 INFO    Scripting (Main): event:type=landing quality mark,initiatorPilotName=RAIDER 302 | HIKER,place=CVN-71,t=1900,initiator_unit_type=FA-18C_hornet,comment=LSO: GRADE:OK : WIRE# 3,initiator_coalition=2,event_id=10,linked_event_id=0,
2026-10-24 18:31:40.000 INFO    Scripting (Main): event:type=land,initiatorPilotName=RAIDER 302 | HIKER,place=CVN-71,t=1900,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=11,linked_event_id=0,
`

var extrasRadio = map[string]string{
	"atc-ug5x.log": `{"level":"info","text":"Mozdok Tower, Raider 302, request taxi.","time":"2026-10-24T18:00:30Z","message":"recognized"}
{"level":"info","callsign":"Raider 302","type":3,"time":"2026-10-24T18:00:30Z","message":"ATC request"}
{"level":"info","text":"Raider 302, Mozdok Tower, taxi runway two six, hold short of the runway.","time":"2026-10-24T18:00:31Z","message":"Tower TX"}
{"level":"info","text":"Taxi runway 26, Raider 302.","time":"2026-10-24T18:00:40Z","message":"recognized"}
{"level":"info","text":"Mozdok Tower, Raider 302, ready for takeoff.","time":"2026-10-24T18:00:50Z","message":"recognized"}
{"level":"info","text":"Raider 302, Mozdok Tower, say again your request.","time":"2026-10-24T18:00:55Z","message":"Tower TX"}
{"level":"info","text":"Mozdok Tower, Raider 302, request takeoff.","time":"2026-10-24T18:00:58Z","message":"recognized"}
{"level":"info","text":"Raider 302, Mozdok Departure, have you seven miles, you are leaving my airspace. Push command, two eight two point zero, channel one. Good day.","time":"2026-10-24T18:06:00Z","message":"Tower TX"}
`,
	"atc-command.log": `{"level":"info","text":"Texaco 31, Raider 302, request 4,000 pounds.","channel":"Texaco-3-1","time":"2026-10-24T18:14:00Z","message":"Command heard"}
{"level":"info","text":"Texaco 31, Raider 302, request 4,000 pounds.","callsign":"Raider 302","channel":"Texaco-3-1","time":"2026-10-24T18:14:00Z","message":"Command transcribed"}
{"level":"info","callsign":"Raider 302","text":"Raider 302, Texaco copies four thousand pounds.","freq":252.3,"time":"2026-10-24T18:14:01Z","message":"Command TX"}
`,
	"atc-marshal.log": `{"level":"info","text":"Union Marshal, Raider 302, radar check.","time":"2026-10-24T18:20:00Z","message":"Marshal heard"}
{"level":"info","text":"Raider 302, Union Marshal, radar contact, angels 25.","time":"2026-10-24T18:20:00Z","message":"Marshal TX"}
{"level":"info","text":"Raider 302, Union Marshal, signal Charlie.","time":"2026-10-24T18:25:00Z","message":"Marshal TX"}
{"level":"info","text":"Union Marshal, reader 302, commencing.","time":"2026-10-24T18:26:00Z","message":"Marshal heard"}
{"level":"info","text":"Raider 302, Union Marshal, copy commencing, check in three mile initial.","time":"2026-10-24T18:26:01Z","message":"Marshal TX"}
{"level":"info","text":"Union Marshal, Raider 302, three mile initial.","time":"2026-10-24T18:28:00Z","message":"Marshal heard"}
`,
}

func TestExtras(t *testing.T) {
	dir := t.TempDir()
	for n, body := range extrasRadio {
		os.WriteFile(filepath.Join(dir, n), []byte(body), 0o644)
	}
	from := time.Date(2026, 10, 24, 17, 50, 0, 0, time.UTC)
	rad, err := ReadRadio(dir, from, from.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := ParseDCSLog(strings.NewReader(extrasLog))
	if err != nil {
		t.Fatal(err)
	}
	s := Analyze(nil, srv, from, from.Add(2*time.Hour), time.UTC)
	x := ComputeExtras(s, rad, srv.Sessions, AllOptions())

	if x.CallsignOf["RAIDER 302 | HIKER"] != "Raider 302" {
		t.Fatalf("callsigns %v", x.CallsignOf)
	}
	if len(x.Comms) != 1 {
		t.Fatalf("comms %+v", x.Comms)
	}
	c := x.Comms[0]
	// 4 tower + 1 tanker + 3 Marshal calls: the readback and the call after
	// "say again" reply to the tower, so need no answer; only the initial
	// went unanswered. The handoff to Command: the next call (the tanker)
	// was within 10 minutes.
	if c.Pilot != "Hiker" || c.Calls != 8 || c.Answered != 7 || c.Asked != 1 || c.Done != 1 || c.SayAgain != 1 {
		t.Fatalf("comms %+v", c)
	}
	if want := 88; c.Score != want { // 100 x (0.6 x 7/8 + 0.4 x 1/1) - 5 = 87.5
		t.Fatalf("score %d want %d", c.Score, want)
	}
	if !strings.Contains(strings.Join(c.Stations, ","), "TEXACO 3-1") {
		t.Fatalf("stations %v", c.Stations)
	}

	if len(x.Tanker) != 1 || x.Tanker[0].Plugs != 2 || x.Tanker[0].Visits != 1 || x.Tanker[0].Connected != 3*time.Minute ||
		x.Tanker[0].Requested != "4,000 lb" || x.Tanker[0].Tanker != "Texaco 3-1" || x.Tanker[0].Pilot != "Hiker" {
		t.Fatalf("tanker %+v", x.Tanker)
	}

	if len(x.Recoveries) != 1 {
		t.Fatalf("recoveries %+v", x.Recoveries)
	}
	rc := x.Recoveries[0]
	ap := rc.Approaches[0]
	if rc.Passes != 2 || rc.Traps != 1 || rc.Bolters != 1 || ap.Passes != 2 || ap.CheckIn.Minute() != 20 || ap.Charlie.Minute() != 25 ||
		ap.Commencing.Minute() != 26 || ap.Initial.Minute() != 28 || ap.Trap.Minute() != 31 || rc.Start.Minute() != 20 {
		t.Fatalf("recovery %+v", rc)
	}

	if len(x.Losses) != 1 || x.Losses[0].Pilot != "Dabs" || x.Losses[0].Cause != "Crashed" {
		t.Fatalf("losses %+v", x.Losses)
	}

	if x.HistoryRuns != 1 || len(x.Currency) != 2 {
		t.Fatalf("currency %d %+v", x.HistoryRuns, x.Currency)
	}
	for _, cu := range x.Currency {
		switch cu.Pilot {
		case "Hiker":
			if !cu.Carrier || cu.TrapStatus != "current" || cu.Traps30 != 1 || cu.Plugs30 != 2 || cu.Sorties30 != 1 {
				t.Fatalf("Hiker %+v", cu)
			}
		case "Dabs":
			if cu.Carrier || cu.FlyStatus != "current" {
				t.Fatalf("Dabs %+v", cu)
			}
		}
	}

	// The pages: everything ticked, Claude's write-up still to come.
	o := AllOptions()
	r := &Report{Session: s, ReportNo: "AAR-1", Mission: "Test", Radio: rad, Opt: &o, X: x}
	page := r.HTML()
	for _, want := range []string{"Comms score", "Carrier recovery", "Tankers", "Currency", "Losses", "Pilot · Hiker", "Pilot · Dabs",
		"Not in this report", "Weapon effectiveness: no player shots recorded.", "written by Claude"} {
		if !strings.Contains(page, want) {
			t.Errorf("page has no %q", want)
		}
	}
	// No write-up, only two pages: no Claude placeholders, the rest left out.
	o = Options{Comms: true, Tanker: true}
	r = &Report{Session: s, ReportNo: "AAR-1", Mission: "Test", Radio: rad, Opt: &o, X: x}
	page = r.HTML()
	if strings.Contains(page, "written by Claude") || strings.Contains(page, "Carrier recovery") || !strings.Contains(page, "Tankers") ||
		strings.Contains(page, "Sustain") {
		t.Fatal("options not followed")
	}
	records := Build(Inputs{Session: s, Radio: rad, Extras: x})
	for _, want := range []string{"## Comms score", "## Tanking", "## Carrier recovery at CVN-71", "## Player losses", "## Currency"} {
		if !strings.Contains(records, want) {
			t.Errorf("records have no %q", want)
		}
	}
}
