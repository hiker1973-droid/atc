package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestMain lets the test binary stand in for DCS-SR-ExternalAudio.exe: run
// with SKYEYE_FAKE_EA set it behaves like ExternalAudio instead of testing.
func TestMain(m *testing.M) {
	if os.Getenv("SKYEYE_FAKE_EA") != "" {
		os.Exit(fakeExternalAudio())
	}
	os.Exit(m.Run())
}

// fakeExternalAudio records each call (the clip's length and the GC setting it
// was started with) and crashes the first one 4 s in, the way the real one
// does, with its real output.
func fakeExternalAudio() int {
	file := ""
	for i, a := range os.Args {
		if a == "--file" && i+1 < len(os.Args) {
			file = os.Args[i+1]
		}
	}
	b, _ := os.ReadFile(file)
	ff, _ := exec.LookPath("ffmpeg")
	d, _ := mp3DurationSec(ff, b)
	logPath := os.Getenv("SKYEYE_FAKE_EA")
	prev, _ := os.ReadFile(logPath)
	call := strings.Count(string(prev), "\n") + 1
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	fmt.Fprintf(f, "%.1f %s\n", d, os.Getenv("DOTNET_GCgen0size"))
	f.Close()
	if os.Getenv("SKYEYE_FAKE_EA_REJECT_GC") != "" && os.Getenv("DOTNET_GCgen0size") != "" {
		fmt.Println("GC heap initialization failed")
		return 1
	}
	fmt.Println("INFO|Sending Audio... Please Wait")
	fmt.Println("INFO|Playing audio - sent 2000ms - 20%")
	if call == 1 {
		fmt.Println("INFO|Playing audio - sent 4000ms - 40%")
		fmt.Println("Process terminated.")
		fmt.Println("A callback was made on a garbage collected delegate of type 'DCS-SR-ExternalAudio!Cabhishek.Timers.Timer+MMTimerProc::Invoke'.")
		return 1
	}
	return 0
}

func TestExternalAudioCrashResendsTheRest(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	defer func(f string, r bool) { flagFFmpeg, flagRadioEffect = f, r }(flagFFmpeg, flagRadioEffect)
	flagFFmpeg, flagRadioEffect = ff, false

	dir := t.TempDir()
	clip := filepath.Join(dir, "atis.mp3")
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=10",
		"-c:a", "libmp3lame", "-ar", "16000", clip).CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v %s", err, out)
	}
	mp3, _ := os.ReadFile(clip)
	calls := filepath.Join(dir, "calls.txt")
	t.Setenv("SKYEYE_FAKE_EA", calls)

	self, _ := os.Executable()
	transmitExternalAudioFile(context.Background(), mp3, 230.0, "Batumi ATIS", "", "5008", self)

	raw, _ := os.ReadFile(calls)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want the crash then one resend, got %d calls: %q", len(lines), lines)
	}
	first, _ := strconv.ParseFloat(strings.Fields(lines[0])[0], 64)
	second, _ := strconv.ParseFloat(strings.Fields(lines[1])[0], 64)
	if first < 9.5 || second < 5.5 || second > 6.5 {
		t.Errorf("clip %.1f s then resend %.1f s; want ~10 then ~6 (from the 4000 ms it reached)", first, second)
	}
	for _, l := range lines {
		if !strings.HasSuffix(l, "0x20000000") {
			t.Errorf("ExternalAudio started without the GC setting: %q", l)
		}
	}
}

func TestExternalAudioProgress(t *testing.T) {
	cases := []struct {
		out     string
		sent    int
		midPlay bool
	}{
		{"Sending Audio... Please Wait\nPlaying audio - sent 2000ms - 4%\nPlaying audio - sent 14000ms - 26%\nProcess terminated.", 14000, true},
		{"Sending Audio... Please Wait\nProcess terminated.", 0, true},
		{"TryConnect @ 127.0.0.1:5008\nUnable to connect", 0, false},
	}
	for _, c := range cases {
		sent, mid := externalAudioProgress(c.out)
		if sent != c.sent || mid != c.midPlay {
			t.Errorf("externalAudioProgress(%q) = %d, %v; want %d, %v", c.out, sent, mid, c.sent, c.midPlay)
		}
	}
}

// An ExternalAudio that will not start with the GC setting: one retry without
// it goes out, and the setting stays off for the session.
func TestExternalAudioRejectsGCSetting(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	defer func(f string, r bool) { flagFFmpeg, flagRadioEffect = f, r }(flagFFmpeg, flagRadioEffect)
	flagFFmpeg, flagRadioEffect = ff, false
	defer externalAudioGCOn.Store(true)

	dir := t.TempDir()
	clip := filepath.Join(dir, "tx.mp3")
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:a", "libmp3lame", "-ar", "16000", clip).CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v %s", err, out)
	}
	mp3, _ := os.ReadFile(clip)
	calls := filepath.Join(dir, "calls.txt")
	t.Setenv("SKYEYE_FAKE_EA", calls)
	t.Setenv("SKYEYE_FAKE_EA_REJECT_GC", "1")
	// Pretend this is not the first call, so the fake does not crash mid-play.
	os.WriteFile(calls, []byte("seed\n"), 0o644)

	self, _ := os.Executable()
	transmitExternalAudioFile(context.Background(), mp3, 261.0, "UGKS-ATC", "", "5008", self)
	if externalAudioGCOn.Load() {
		t.Error("GC setting still on after ExternalAudio rejected it")
	}
	raw, _ := os.ReadFile(calls)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")[1:]
	if len(lines) != 2 || strings.HasSuffix(lines[1], "0x20000000") {
		t.Errorf("want a rejected call then one without the setting, got %q", lines)
	}
}
