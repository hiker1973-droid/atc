package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A recovered ExternalAudio crash (the rest resent) is not an alert; one that
// could not be resent, and other ExternalAudio trouble, still are.
func TestRecoveredExternalAudioCrashDoesNotAlert(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	rootDir = t.TempDir()
	os.MkdirAll(filepath.Join(rootDir, "logs"), 0o755)
	now := time.Now().Format(time.RFC3339)
	lines := []string{
		`{"level":"warn","time":"` + now + `","message":"ExternalAudio crashed mid-transmission - resending the rest"}`,
		`{"level":"info","time":"` + now + `","message":"ExternalAudio crashed at the very end - nothing left to resend"}`,
		`{"level":"error","time":"` + now + `","message":"ExternalAudio file error"}`,
		`{"level":"warn","time":"` + now + `","message":"ExternalAudio file TX timed out — killed"}`,
	}
	os.WriteFile(filepath.Join(rootDir, "logs", "atc-atis.log"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	var got []string
	for _, a := range scanLogAlerts() {
		got = append(got, a.Message)
	}
	joined := strings.Join(got, " | ")
	if strings.Contains(joined, "resend") {
		t.Errorf("recovered crash alerted: %s", joined)
	}
	if !strings.Contains(joined, "ExternalAudio file error") || !strings.Contains(joined, "timed out") {
		t.Errorf("real ExternalAudio faults must still alert: %s", joined)
	}
}
