package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A fresh install (no OpenAI key, no SRS address) is offered Setup; a rig
// that has both, like the three live ones, is not; finishing Setup stops the
// offer even if a key is later removed.
func TestSetupNeeded(t *testing.T) {
	oldRoot, oldEnv, oldRoles := rootDir, userEnv, roles
	rootDir = t.TempDir()
	t.Cleanup(func() { rootDir, userEnv, roles = oldRoot, oldEnv, oldRoles })
	os.WriteFile(filepath.Join(rootDir, "start_towers_caucasus.bat"),
		[]byte(`start "Batumi Tower" cmd /c "%~dp0atc.exe --airfield UGSB --tts-voice nova"`), 0o644)
	discoverRoles()

	env := map[string]string{}
	userEnv = func(name string) string { return env[name] }
	if s := setupSnapshot(); s["needsSetup"] != true {
		t.Fatal("fresh install not offered Setup")
	}
	env["OPENAI_API_KEY"], env["SKYEYE_SRS"] = "sk-test", "localhost:5002"
	if s := setupSnapshot(); s["needsSetup"] != false {
		t.Fatal("set-up rig offered Setup")
	}
	ts := setupSnapshot()["theatres"].([]map[string]any)
	if len(ts) != 1 || ts[0]["key"] != "caucasus" || ts[0]["testFreq"] != 260.0 {
		t.Fatalf("theatres %v", ts)
	}
	delete(env, "SKYEYE_SRS")
	if err := writeSetup(setupState{Done: true, Theatre: "caucasus"}); err != nil {
		t.Fatal(err)
	}
	if s := setupSnapshot(); s["needsSetup"] != false || s["theatre"] != "caucasus" {
		t.Fatalf("finished setup %v", s)
	}
}
