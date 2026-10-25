package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/license"
)

// The problem report carries the logs and settings but never a key: not in
// settings.txt, not in a log line that printed one, not the licence key.
func TestBugReportHasNoKeys(t *testing.T) {
	root := withRoot(t)
	oldLic := newLicenceChecker
	newLicenceChecker = func() *license.Checker {
		return &license.Checker{Dir: root, Store: &memLicStore{start: time.Now()}, Now: time.Now}
	}
	t.Cleanup(func() { newLicenceChecker = oldLic; licChecked = time.Time{} })
	licChecked = time.Time{}

	const openai, eleven = "sk-proj-abcdefghijklmnopqrstuvwxyz0123", "el_0123456789abcdef0123"
	oldEnv := userEnv
	userEnv = func(name string) string {
		switch name {
		case "OPENAI_API_KEY":
			return openai
		case "ELEVENLABS_API_KEY":
			return eleven
		case "SKYEYE_SRS":
			return "localhost:5002"
		}
		return ""
	}
	t.Cleanup(func() { userEnv = oldEnv })

	os.WriteFile(filepath.Join(root, "licence.json"), []byte(`{"key":"SKYEYE1-secretlicencekey.sig"}`), 0o644)
	os.MkdirAll(filepath.Join(root, "logs"), 0o755)
	os.WriteFile(filepath.Join(root, "logs", "atc-ugks.log"), []byte(
		`{"level":"info","text":"Senaki tower, Raider 302, ready for taxi.","message":"recognized"}`+"\n"+
			`{"level":"error","error":"401 bad key `+eleven+`","message":"ElevenLabs TTS failed"}`+"\n"+
			`{"level":"error","error":"Incorrect API key provided: sk-other-zzzzzzzzzzzzzzzzzzzz","message":"Whisper transcription error"}`+"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "frequencies.json"), []byte(`{"UGKS":{"tower":"132.000"}}`), 0o644)

	var buf bytes.Buffer
	if err := writeBugReport(&buf, "Senaki didn't answer", "Sat 24 Oct 19:20", "Senaki Tower", time.Now()); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range z.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}
	for _, want := range []string{"report.txt", "settings.txt", "status/roles.json", "config/frequencies.json", "logs/atc-ugks.log"} {
		if _, ok := files[want]; !ok {
			t.Errorf("no %s in the zip: %v", want, files)
		}
	}
	if _, ok := files["config/licence.json"]; ok {
		t.Error("licence.json is in the zip")
	}
	for name, body := range files {
		for _, secret := range []string{openai, eleven, "SKYEYE1-secretlicencekey", "sk-other-zzzz"} {
			if strings.Contains(body, secret) {
				t.Errorf("%s holds a key: %q", name, secret)
			}
		}
	}
	if !strings.Contains(files["settings.txt"], "OpenAI API key: (set)") || !strings.Contains(files["settings.txt"], "localhost:5002") {
		t.Errorf("settings.txt:\n%s", files["settings.txt"])
	}
	if !strings.Contains(files["report.txt"], "Senaki didn't answer") || !strings.Contains(files["logs/atc-ugks.log"], "ready for taxi") {
		t.Errorf("report or log missing:\n%s", files["report.txt"])
	}
}
