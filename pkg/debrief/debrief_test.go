package debrief

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A short recording: Venom 1-1 (a player) takes off from Senaki, fires an
// AIM-120 that ends beside a MiG-29, which is destroyed, and lands.
const sampleACMI = "\ufeffFileType=text/acmi/tacview\nFileVersion=2.2\n" +
	"0,ReferenceTime=2016-06-21T10:00:00Z,RecordingTime=2026-10-22T18:00:00Z,Title=Caucasus Training\\, V1.24\n" +
	"#0\n" +
	"101,T=42.0|42.2|50,Type=Air+FixedWing,Name=F-16C_50,Pilot=Dabs,Group=Venom 1,Coalition=Allies\n" +
	"202,T=42.5|42.5|6000,Type=Air+FixedWing,Name=MiG-29A,Pilot=Bandit-1-1,Group=Bandit-1,Coalition=Enemies\n" +
	"#60\n" +
	"0,Event=TakenOff|101|Dabs has taken off from Senaki-Kolkhi.\n" +
	"101,T=42.1|42.25|5000\n" +
	"#300\n" +
	"101,T=42.40|42.45|6100\n" +
	"301,T=42.401|42.451|6100,Type=Weapon+Missile,Name=AIM_120C\n" +
	"#310\n" +
	"301,T=42.4995|42.4995|6000\n" +
	"-301\n" +
	"#311\n" +
	"0,Event=Destroyed|202|\n" +
	"-202\n" +
	"#1200\n" +
	"0,Event=Landed|101|Dabs has landed at Senaki-Kolkhi.\n"

func TestParseACMI(t *testing.T) {
	rec, err := ParseACMI(strings.NewReader(sampleACMI))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Title != "Caucasus Training, V1.24" || !rec.Start.Equal(time.Date(2026, 10, 22, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("title %q start %v", rec.Title, rec.Start)
	}
	var dabs *Flight
	for _, f := range rec.Flights {
		if f.Pilot == "Dabs" {
			dabs = f
		}
	}
	if dabs == nil || !dabs.Human {
		t.Fatalf("Dabs not found as a player: %+v", rec.Flights)
	}
	if dabs.Fired["AIM_120C"] != 1 || len(dabs.Kills) != 1 || !strings.Contains(dabs.Kills[0], "MiG-29A") {
		t.Fatalf("Dabs fired %v kills %v", dabs.Fired, dabs.Kills)
	}
	if dabs.Fate != "landed" || len(dabs.Takeoffs) != 1 || !strings.HasSuffix(dabs.Takeoffs[0], "Senaki-Kolkhi") {
		t.Fatalf("Dabs takeoffs %v fate %q", dabs.Takeoffs, dabs.Fate)
	}
	if len(rec.Losses) != 1 || rec.Losses[0].By != "Dabs" {
		t.Fatalf("losses %+v", rec.Losses)
	}
	for _, f := range rec.Flights {
		if f.Pilot == "Bandit-1-1" && (f.Human || !strings.HasPrefix(f.Fate, "destroyed")) {
			t.Fatalf("the MiG: %+v", f)
		}
	}
	text := Build(Inputs{Squadron: "vSFG-7", Tacview: rec})
	for _, want := range []string{"Player aircraft (1)", "Dabs - F-16C_50", "Likely kills", "likely by Dabs"} {
		if !strings.Contains(text, want) {
			t.Errorf("records lack %q:\n%s", want, text)
		}
	}
}

func TestParseZippedACMI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rec.zip.acmi")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("rec.txt.acmi")
	w.Write([]byte(sampleACMI))
	zw.Close()
	f.Close()
	rec, err := ParseACMIFile(path)
	if err != nil || len(rec.Flights) != 2 {
		t.Fatalf("zipped: %v %+v", err, rec)
	}
}

func TestParseDCSLog(t *testing.T) {
	log := "=== Log opened UTC 2026-10-22 21:29:12\n" +
		"2026-10-22 21:29:12.004 WARNING EDCORE (Main): hypervisor is active\n" +
		"2026-10-22 21:30:00.100 INFO    NET (Main): client 2 connected: Dabs\n" +
		"2026-10-22 21:31:00.100 ERROR   Scripting (Main): [string \"x\"]:12: attempt to index nil value\n" +
		"2026-10-22 21:32:00.100 ERROR   Scripting (Main): [string \"x\"]:14: attempt to index nil value\n" +
		"2026-10-22 21:33:00.100 WARNING EDTERRAIN4 (Main): Removed degenerate triangle in collision 'a_crash'\n"
	s, err := ParseDCSLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 1 || s.Problems[0].Count != 2 {
		t.Fatalf("problems %+v", s.Problems)
	}
	if len(s.Events) != 1 || !strings.Contains(s.Events[0], "Dabs") {
		t.Fatalf("events %v", s.Events)
	}
	if s.Levels["ERROR"] != 2 || s.End.Format("15:04") != "21:33" {
		t.Fatalf("levels %v end %v", s.Levels, s.End)
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
		t.Fatalf("kinds %q unheard %d lines %+v", kinds, rad.Unheard, rad.Lines)
	}
}

func TestWrite(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
			return
		}
		w.Write([]byte(`{"content":[{"type":"text","text":"## Summary\nGood session."}],"stop_reason":"end_turn","usage":{"input_tokens":1200,"output_tokens":300}}`))
		_ = got
	}))
	defer srv.Close()
	old := API
	API = srv.URL
	defer func() { API = old }()
	text, u, err := Write(context.Background(), "k", Models[0].ID, "records")
	if err != nil || !strings.Contains(text, "Good session") || u.InputTokens != 1200 {
		t.Fatalf("%q %+v %v", text, u, err)
	}
	if _, _, err := Write(context.Background(), "bad", Models[0].ID, "records"); err == nil || !strings.Contains(err.Error(), "rejected the key") {
		t.Fatalf("bad key: %v", err)
	}
}
