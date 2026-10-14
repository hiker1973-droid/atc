package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The 2026-10-14 truncations: Kobuleti English 0.4s, Senaki 1s, a Russian
// pass 1.6s — against ~55-word reads that normally run ~19s.
func TestATISAudioTooShort(t *testing.T) {
	read := strings.Repeat("word ", 55)
	cases := []struct {
		sec  float64
		want bool
	}{
		{0.4, true},
		{1.6, true},
		{8.0, true},   // 6.9 words/s
		{9.5, false},  // 5.8 words/s — fast but plausible
		{19.0, false}, // normal
		{60.0, false},
	}
	for _, c := range cases {
		if got := atisAudioTooShort(read, c.sec); got != c.want {
			t.Errorf("55 words in %.1fs: tooShort=%v, want %v", c.sec, got, c.want)
		}
	}
	if atisAudioTooShort("", 0) {
		t.Error("empty text must never be rejected")
	}
}

func TestParseFFmpegTime(t *testing.T) {
	out := "size=N/A time=00:00:05.12 bitrate=N/A speed= 500x\r" +
		"[out#0/null] video:0KiB audio:2KiB\nsize=N/A time=00:01:02.53 bitrate=N/A speed= 700x\n"
	sec, err := parseFFmpegTime(out)
	if err != nil || sec < 62.52 || sec > 62.54 {
		t.Fatalf("got %v, %v; want 62.53 (the last time=)", sec, err)
	}
	if _, err := parseFFmpegTime("no progress here"); err == nil {
		t.Error("want an error when ffmpeg printed no time=")
	}
}

// Decodes a real cached ATIS file, so the parser is checked against the
// output of the ffmpeg actually installed. Skipped where there is none.
func TestMP3DurationSecRealFile(t *testing.T) {
	// Flags are not parsed under go test, so flagFFmpeg is empty here; try
	// the --ffmpeg default, then PATH.
	ff, err := exec.LookPath(`C:\ffmpeg-master-latest-win64-gpl\bin\ffmpeg.exe`)
	if err != nil {
		ff, err = exec.LookPath("ffmpeg")
	}
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	data, err := os.ReadFile(`C:\SkyeyeATC\atis_cache\atis_UGSB_bilingual.mp3`)
	if err != nil {
		t.Skip("no cached ATIS file on this machine")
	}
	sec, err := mp3DurationSec(ff, data)
	if err != nil {
		t.Fatal(err)
	}
	if sec < 10 || sec > 120 {
		t.Fatalf("decoded %.2fs, outside any real ATIS length", sec)
	}
	t.Logf("UGSB cache decodes to %.2fs", sec)
}
