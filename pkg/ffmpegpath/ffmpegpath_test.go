package ffmpegpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWinget(t *testing.T) {
	if _, err := os.Stat(Default); err == nil {
		t.Skip("this PC has ffmpeg at the default path, which Find returns first")
	}
	la := t.TempDir()
	t.Setenv("LOCALAPPDATA", la)
	t.Setenv("PATH", "")
	if p := Find(""); p != "" {
		t.Fatalf("empty PC: Find = %q, want none", p)
	}
	want := filepath.Join(la, "Microsoft", "WinGet", "Packages", "Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe",
		"ffmpeg-8.0-full_build", "bin", "ffmpeg.exe")
	os.MkdirAll(filepath.Dir(want), 0o755)
	os.WriteFile(want, []byte("x"), 0o755)
	if p := Find(""); p != want {
		t.Fatalf("Find = %q, want the winget package %q", p, want)
	}
	beside := t.TempDir()
	os.WriteFile(filepath.Join(beside, "ffmpeg.exe"), []byte("x"), 0o755)
	if p := Find(beside); p != filepath.Join(beside, "ffmpeg.exe") {
		t.Fatalf("Find(dir) = %q, want the copy beside the program", p)
	}
}
