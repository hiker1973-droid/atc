package debrief

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A short session. The server log: Dabs (player, blue) takes off from
// Senaki, fires an AIM-120C at Bacon (player, red) that hits and kills him,
// fires a 9X at Screwy (blue) that misses, Thrash (red) crashes on his own,
// Dabs lands on the carrier with an OK 3-wire. The recording has the jets
// and the missile.
const sampleLog = `=== Log opened UTC 2026-10-22 18:00:00
2026-10-22 18:00:00.000 INFO    Scripting (Main): event:type=mission start,event_id=1,t=0,linked_event_id=0,
2026-10-22 18:01:00.000 INFO    Scripting (Main): event:type=takeoff,initiatorPilotName=VENOM 1 | DABS,place=Senaki-Kolkhi,t=60,initiator_unit_type=F-16C_50,initiator_coalition=2,initiator_ws_type1=1,event_id=2,linked_event_id=0,
2026-10-22 18:01:30.000 INFO    Scripting (Main): event:type=takeoff,initiatorPilotName=RAIDER 1 | BACON,place=Mozdok,t=90,initiator_unit_type=FA-18C_hornet,initiator_coalition=1,initiator_ws_type1=1,event_id=3,linked_event_id=0,
2026-10-22 18:05:00.000 INFO    Scripting (Main): event:weapon=AIM-120C,initiatorPilotName=VENOM 1 | DABS,target_coalition=1,targetPilotName=RAIDER 1 | BACON,type=shot,target_unit_type=FA-18C_hornet,t=300,initiator_unit_type=F-16C_50,event_id=10,initiator_coalition=2,initiator_ws_type1=1,target_ws_type1=1,linked_event_id=0,
2026-10-22 18:05:10.000 INFO    Scripting (Main): event:weapon=AIM-120C,initiatorPilotName=VENOM 1 | DABS,target_coalition=1,targetPilotName=RAIDER 1 | BACON,type=hit,target_unit_type=FA-18C_hornet,t=310,initiator_unit_type=F-16C_50,event_id=11,initiator_coalition=2,initiator_ws_type1=1,target_ws_type1=1,linked_event_id=10,
2026-10-22 18:05:12.000 INFO    Scripting (Main): event:weapon=AIM-120C,initiatorPilotName=VENOM 1 | DABS,target_coalition=1,targetPilotName=RAIDER 1 | BACON,type=kill,target_unit_type=FA-18C_hornet,t=312,initiator_unit_type=F-16C_50,event_id=12,initiator_coalition=2,initiator_ws_type1=1,target_ws_type1=1,linked_event_id=11,
2026-10-22 18:06:00.000 INFO    Scripting (Main): event:weapon=AIM-9X,initiatorPilotName=VENOM 1 | DABS,target_coalition=2,targetPilotName=VENOM 2 | SCREWY,type=shot,target_unit_type=F-16C_50,t=360,initiator_unit_type=F-16C_50,event_id=13,initiator_coalition=2,initiator_ws_type1=1,target_ws_type1=1,linked_event_id=0,
2026-10-22 18:07:00.000 INFO    Scripting (Main): event:type=crash,initiatorPilotName=RAIDER 2 | THRASH,t=420,initiator_unit_type=FA-18C_hornet,initiator_coalition=1,initiator_ws_type1=1,event_id=14,linked_event_id=0,
2026-10-22 18:20:00.000 INFO    Scripting (Main): event:type=landing quality mark,initiatorPilotName=VENOM 1 | DABS,place=CVN-72,t=1200,initiator_unit_type=F-16C_50,comment=LSO: GRADE:OK : WIRE# 3,initiator_coalition=2,event_id=15,linked_event_id=0,
2026-10-22 18:20:00.000 INFO    Scripting (Main): event:type=land,initiatorPilotName=VENOM 1 | DABS,place=CVN-72,t=1200,initiator_unit_type=F-16C_50,initiator_coalition=2,initiator_ws_type1=1,event_id=16,linked_event_id=0,
2026-10-22 18:21:00.000 ERROR   Scripting (Main): [string "x"]:12: attempt to index nil value
`

const sampleACMI = "\ufeffFileType=text/acmi/tacview\nFileVersion=2.2\n" +
	"0,ReferenceTime=2016-06-21T10:00:00Z\n0,RecordingTime=2026-10-22T18:00:00Z\n0,Title=Caucasus Training\\, V1.24\n" +
	"0,ReferenceLongitude=42\n0,ReferenceLatitude=42\n#0\n" +
	"101,T=0.0|0.2|50,Type=Air+FixedWing,Name=F-16C_50,Pilot=VENOM 1 | DABS,Group=Venom 1,Color=Blue,Coalition=Enemies,AGL=2\n" +
	"202,T=0.5|0.5|6000,Type=Air+FixedWing,Name=FA-18C_hornet,Pilot=RAIDER 1 | BACON,Group=Raider 1,Color=Red,Coalition=Allies,AGL=5000\n" +
	"#60\n101,T=0.0|0.21|200,AGL=150\n" +
	"#300\n101,T=0.40|0.45|6100\n202,T=0.6|0.6|6000\n301,T=0.401|0.451|6100,Type=Weapon+Missile,Name=AIM_120C,Color=Blue\n" +
	"#310\n301,T=0.5995|0.5995|6000\n-301\n#312\n-202\n" +
	"#1200\n101,T=0.4|0.45|20,AGL=1\n"

func session(t *testing.T) *Session {
	t.Helper()
	rec, err := ParseACMI(strings.NewReader(sampleACMI))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := ParseDCSLog(strings.NewReader(sampleLog))
	if err != nil {
		t.Fatal(err)
	}
	return Analyze(rec, srv, time.Time{}, time.Time{}, time.UTC)
}

func TestParseACMI(t *testing.T) {
	rec, err := ParseACMI(strings.NewReader(sampleACMI))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Title != "Caucasus Training, V1.24" || !rec.Start.Equal(time.Date(2026, 10, 22, 18, 0, 0, 0, time.UTC)) || rec.Duration != 1200 {
		t.Fatalf("title %q start %v duration %v", rec.Title, rec.Start, rec.Duration)
	}
	if len(rec.Aircraft) != 2 || len(rec.Weapons) != 1 {
		t.Fatalf("aircraft %d weapons %d", len(rec.Aircraft), len(rec.Weapons))
	}
	dabs := rec.Aircraft[0]
	if dabs.Side() != "blue" || dabs.At(300).Lat != 42.45 || dabs.At(60).AGL != 150 {
		t.Fatalf("Dabs %s %+v %+v", dabs.Side(), dabs.At(300), dabs.At(60))
	}
	if bacon := rec.Objects["202"]; !bacon.Removed || bacon.End != 312 {
		t.Fatalf("Bacon removed %v at %v", bacon.Removed, bacon.End)
	}
}

func TestStreamedZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rec.zip.acmi")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("rec.txt.acmi")
	w.Write([]byte(sampleACMI))
	zw.Close()
	f.Close()
	// Cut the central directory off, as Tacview leaves a recording it is
	// still writing.
	b, _ := os.ReadFile(path)
	os.WriteFile(path, b[:bytes.Index(b, []byte("PK"))], 0o644)
	rec, err := ParseACMIFile(path)
	if err != nil || len(rec.Aircraft) != 2 {
		t.Fatalf("still-recording zip: %v %+v", err, rec)
	}
}

func TestEvents(t *testing.T) {
	srv, _ := ParseDCSLog(strings.NewReader(sampleLog))
	if len(srv.Sessions) != 1 || len(srv.Sessions[0].Events) != 10 || len(srv.Problems) != 1 {
		t.Fatalf("sessions %d events %d problems %d", len(srv.Sessions), len(srv.Sessions[0].Events), len(srv.Problems))
	}
	e := srv.Sessions[0].Events[3]
	if e.Type != "shot" || e.Init.Pilot != "VENOM 1 | DABS" || !e.Init.Human() || e.Target.Side() != "red" || e.Weapon != "AIM-120C" ||
		!e.At.Equal(time.Date(2026, 10, 22, 18, 5, 0, 0, time.UTC)) {
		t.Fatalf("shot %+v", e)
	}
	for _, c := range []struct {
		in, grade string
		wire      int
	}{
		{"LSO: GRADE:_OK_ : WIRE# 2", "_OK_", 2}, {"LSO: GRADE:WO  WOFDIC ", "WOFD", 0}, {"LSO: GRADE:(OK) : WIRE# 4", "(OK)", 4}, {"LSO: GRADE:C", "C", 0},
	} {
		if g := ParseGrade(c.in); g.Grade != c.grade || g.Wire != c.wire {
			t.Errorf("%q -> %+v", c.in, g)
		}
	}
}

func TestShortName(t *testing.T) {
	for in, want := range map[string]string{"RAIDER 031 | BACON": "Bacon", "406-B.P-CVW5": "B.P", "Yoda": "Yoda", "VENOM 1 | DABS": "Dabs", "F-16C_50": "F-16C_50"} {
		if got := ShortName(in); got != want {
			t.Errorf("ShortName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnalyze(t *testing.T) {
	s := session(t)
	if !s.HasEvents || !s.PvP || s.BlueName != "Venom" || s.RedName != "Raider" {
		t.Fatalf("events %v pvp %v names %q %q", s.HasEvents, s.PvP, s.BlueName, s.RedName)
	}
	var dabs *PilotStat
	for _, p := range s.Pilots {
		if p.Name == "Dabs" {
			dabs = p
		}
	}
	if dabs == nil || dabs.Sorties != 1 || dabs.Kills != 1 || dabs.Shots != 2 || dabs.Hits != 1 || dabs.Landings != 1 || dabs.Traps != 1 ||
		dabs.Flight != 19*time.Minute || strings.Join(dabs.Aircraft, ",") != "F-16C" {
		t.Fatalf("Dabs %+v", dabs)
	}
	if len(s.Engagements) != 1 {
		t.Fatalf("engagements %d", len(s.Engagements))
	}
	en := s.Engagements[0]
	if en.LostRed != 1 || en.LostBlue != 0 || en.Winner != "Blue" || len(en.Shots) != 2 {
		t.Fatalf("engagement %+v", en)
	}
	sh := en.Shots[0]
	if sh.Result != "KILL 1" || sh.missile == nil || sh.RangeNM < 5 || sh.RangeNM > 20 || sh.AltFt < 19000 {
		t.Fatalf("shot %+v", sh)
	}
	if r := en.Shots[1].Result; r != "miss (blue-on-blue)" {
		t.Fatalf("friendly shot result %q", r)
	}
	if k := s.Kills[0]; k.Killer != "Dabs" || k.Victim != "Bacon" || k.Lat == 0 {
		t.Fatalf("kill %+v", k)
	}
	if len(s.Missiles) != 1 || s.Missiles[0].Weapon != "AIM-120" || s.Missiles[0].Fired != 1 || s.Missiles[0].Hit != 1 {
		t.Fatalf("missiles %+v (the blue-on-blue 9X belongs in Safety)", s.Missiles)
	}
	kinds := ""
	for _, it := range s.Safety {
		kinds += it.Kind + ";"
	}
	if kinds != "Blue-on-blue;Terrain;" {
		t.Fatalf("safety %q %+v", kinds, s.Safety)
	}
	if len(s.Traps) != 1 || s.Traps[0].Grade.Grade != "OK" || s.Traps[0].Wire != 3 || len(s.Fields) != 3 {
		t.Fatalf("traps %+v fields %+v", s.Traps, s.Fields)
	}
}

func TestReportHTML(t *testing.T) {
	s := session(t)
	in := Inputs{Squadron: "vSFG-7", Session: s}
	r := &Report{Session: s, Facts: Facts(in, []string{"dcs.log"}), ReportNo: ReportNumber(in), Squadron: "vSFG-7", Mission: "Test"}
	preview := r.HTML()
	for _, want := range []string{"AAR-20261022-1800", "Blue (Venom)</span> 1", "Engagement 1: Blue wins", "KILL 1", "<svg", "Greenie board", "written by Claude"} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview lacks %q", want)
		}
	}
	r.AI = &AIText{BLUF: "Venom won.", WhatWon: []Point2{{"Shoot first.", "<b>it worked</b>"}}, Recommendations: []Action{{"Brief it.", "Flight leads"}}}
	page := r.HTML()
	if !strings.Contains(page, "<b>Shoot first.</b> &lt;b&gt;it worked&lt;/b&gt;") || !strings.Contains(page, "[Flight leads]") || strings.Contains(page, "written by Claude") {
		t.Fatal("Claude's words not placed, or not escaped")
	}
	r.AI.DiscordFights = []string{"Dabs took Bacon from 15 nm."}
	r.AI.DiscordOutcome = "**Venom wins, 1-0.**"
	r.AI.DiscordTake = []string{"Shoot first."}
	post := strings.Join(r.DiscordPosts(), "\n")
	for _, want := range []string{"# :airplane: TEST — AAR", "**1 v 2 · Senaki-Kolkhi vs Mozdok · Oct 22**",
		"## :red_circle: KILLS — 1 confirmed (Venom 1, Raider 0)", "T+05:12  Dabs    > Bacon   AIM-120C",
		"## :skull: LOSSES — 1", "## :bar_chart: SHOT STATS — 1 AMRAAMs fired", "Venom: 1 kill per 1 shots.",
		"**Fight 1 (", "Venom 1–0).** Dabs took Bacon", "## :dart: OUTCOME", "- Shoot first."} {
		if !strings.Contains(post, want) {
			t.Errorf("Discord post lacks %q:\n%s", want, post)
		}
	}
	for _, p := range r.DiscordPosts() {
		if len(p) > 2000 {
			t.Fatalf("a post of %d characters", len(p))
		}
	}
	long := []string{strings.Repeat("x", 1500), "```\n" + strings.Repeat("line\n", 600) + "```"}
	for _, p := range pack(long, 1900) {
		if len(p) > 1900 || strings.Count(p, "```")%2 != 0 {
			t.Fatalf("a split post is too long or leaves a code block open: %d chars", len(p))
		}
	}
}

func TestBuildAndParseAI(t *testing.T) {
	s := session(t)
	text := Build(Inputs{Squadron: "vSFG-7", Session: s, Focus: "the AMRAAM shot"})
	for _, want := range []string{"Engagement 1: Blue wins", "Dabs (blue) | Bacon | AIM-120C", "KILL 1", "Blue-on-blue", "GRADE:OK", "The squadron asks: the AMRAAM shot"} {
		if !strings.Contains(text, want) {
			t.Errorf("records lack %q:\n%s", want, text)
		}
	}
	a := ParseAI("Here it is:\n```json\n{\"bluf\": \"Venom won.\", \"sustain\": [\"x\"], \"recommendations\": [{\"text\": \"y\", \"owner\": \"All pilots\"}]}\n```")
	if a.BLUF != "Venom won." || len(a.Sustain) != 1 || a.Recommendations[0].Owner != "All pilots" {
		t.Fatalf("parsed %+v", a)
	}
	if a := ParseAI("no json here"); a.Overview != "no json here" {
		t.Fatalf("fallback %+v", a)
	}
}

func TestReadRadio(t *testing.T) {
	dir := t.TempDir()
	lines := `{"level":"info","message":"Tacview nominal","time":"2026-10-22T17:37:40-06:00"}
{"level":"info","text":"Senaki traffic, Raider 302, comm check.","rmsDb":-29.9,"time":"2026-10-22T17:37:43-06:00","message":"recognized"}
{"level":"info","text":"Raider 302, Senaki Tower, reading you loud and clear.","voice":"alloy","time":"2026-10-22T17:37:43-06:00","message":"TX via OpenAI TTS + ExternalAudio"}
{"level":"info","text":"Venom flight, Senaki Tower, line up and wait.","tts":"elevenlabs","time":"2026-10-22T18:26:45-06:00","message":"Tower TX"}
{"level":"info","kind":"callsign_changed","detail":"same radio, two callsigns","time":"2026-10-22T18:26:45-06:00","message":"comms report"}
{"level":"warn","text":"thank you","time":"2026-10-22T18:27:00-06:00","message":"Whisper hallucination — dropping"}
{"level":"warn","text":"thank you","time":"2026-10-22T18:27:01-06:00","message":"Whisper prompt echo - not retried"}
{"level":"info","text":"too late","time":"2026-10-22T23:00:00-06:00","message":"recognized"}
`
	os.WriteFile(filepath.Join(dir, "atc-ugsb.log"), []byte(lines), 0o644)
	from := time.Date(2026, 10, 22, 23, 0, 0, 0, time.UTC)
	rad, err := ReadRadio(dir, from, from.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	kinds := ""
	for _, l := range rad.Lines {
		kinds += l.Kind + " "
	}
	if kinds != "pilot atc atc report unheard " || rad.Unheard != 1 || rad.Lines[0].Station != "UGSB" {
		t.Fatalf("kinds %q unheard %d", kinds, rad.Unheard)
	}
}

func TestWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
			return
		}
		w.Write([]byte(`{"content":[{"type":"text","text":"{\"bluf\":\"Good session.\"}"}],"stop_reason":"end_turn","usage":{"input_tokens":1200,"output_tokens":300}}`))
	}))
	defer srv.Close()
	old := API
	API = srv.URL
	defer func() { API = old }()
	text, u, err := Write(context.Background(), "k", Models[0].ID, "records")
	if err != nil || ParseAI(text).BLUF != "Good session." || u.InputTokens != 1200 {
		t.Fatalf("%q %+v %v", text, u, err)
	}
	if _, _, err := Write(context.Background(), "bad", Models[0].ID, "records"); err == nil || !strings.Contains(err.Error(), "rejected the key") {
		t.Fatalf("bad key: %v", err)
	}
}

func TestHTML(t *testing.T) {
	md := "## 1. Mission Overview\n**BLUF:** Venom *won*.\n\n- 18:00Z - takeoff\n- 18:05Z - <script>x</script>\n\n" +
		"| A | B |\n|---|---|\n| 1 | `2` |\n\n1. Fix it [Flight leads]\n2. Again\n"
	h := HTML(md)
	for _, want := range []string{"<h2>1. Mission Overview</h2>", "<p><b>BLUF:</b> Venom <i>won</i>.</p>", "&lt;script&gt;", "<td><code>2</code></td>", "<ol>\n<li>Fix it [Flight leads]</li>"} {
		if !strings.Contains(h, want) {
			t.Errorf("HTML lacks %q:\n%s", want, h)
		}
	}
}

func TestAddUp(t *testing.T) {
	srv, _ := ParseDCSLog(strings.NewReader(sampleLog))
	// The same run twice (a log and its rotated copy) counts once.
	runs := append(append([]EventSession{}, srv.Sessions...), srv.Sessions...)
	tot := AddUp(runs, time.Time{}, time.Time{}, time.UTC)
	if tot.Sessions != 1 || len(tot.Pilots) != 3 || len(tot.Passes) != 1 {
		t.Fatalf("sessions %d pilots %d passes %d", tot.Sessions, len(tot.Pilots), len(tot.Passes))
	}
	if p := tot.Pilots[0]; p.Name != "Dabs" || p.Sorties != 1 || p.Kills != 1 || p.Traps != 1 || p.Flight != 19*time.Minute {
		t.Fatalf("Dabs %+v", p)
	}
	b := tot.GreenieBoard()
	if len(b) != 1 || b[0].Pilot != "Dabs" || b[0].Avg != 4 || b[0].Traps != 1 {
		t.Fatalf("board %+v", b)
	}
	if none := AddUp(srv.Sessions, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), time.Time{}, time.UTC); none.Sessions != 0 {
		t.Fatalf("period filter: %d sessions", none.Sessions)
	}
}
