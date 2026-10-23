// Readback-Setup.exe: installs Readback ATC on a PC that has none of the
// vSFG-7 setup (operator 2026-10-23: "are we building a .exe", "i can install
// on my laptop"). It carries atc.exe and launcher.exe (build_installer.bat
// puts them in payload/), copies them to %LOCALAPPDATA%\Readback ATC (no
// admin needed), adds Start-menu and desktop shortcuts, checks the
// prerequisites, starts the launcher and opens its Setup page, which does the
// rest: keys, SRS server, squadron, theatre (generated start files) and a
// test call.
//
// It shows its own window (gui.go: a page in an Edge app window; operator
// "make a pretty installer screen instead of prompt"). --quiet and --update
// run with no window and log to %TEMP%\Readback-Setup.log.
//
//	Readback-Setup.exe               install or update
//	Readback-Setup.exe --uninstall   remove the programs and shortcuts (settings and logs stay)
//	Readback-Setup.exe --quiet       install with no window and no questions
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/ffmpegpath"
)

//go:embed payload
var payload embed.FS

// version (git describe) and productVersion (product_version.txt) are set by
// build_installer.bat with -ldflags -X.
var (
	version        = "dev"
	productVersion = ""
)

const (
	appName    = "Readback ATC"
	listenPort = 7000
)

var programs = []string{"atc.exe", "launcher.exe"}

// guides are the PDFs from docs/manual/build_manuals.py, beside the programs
// and in the Start menu.
var guides = []string{"Readback ATC Installation Guide.pdf", "Readback ATC User Manual.pdf"}

var (
	flagDir       = flag.String("dir", filepath.Join(os.Getenv("LOCALAPPDATA"), "Readback ATC"), "Install folder")
	flagUninstall = flag.Bool("uninstall", false, "Remove the programs and shortcuts; settings and logs stay")
	flagQuiet     = flag.Bool("quiet", false, "No window and no questions: install (no start-with-Windows shortcut), start, open Setup")
	flagNoStart   = flag.Bool("no-start", false, "Install only; do not start the launcher")
	flagNoLinks   = flag.Bool("no-shortcuts", false, "Add no shortcuts (for testing)")
	flagUpdate    = flag.Bool("update", false, "Run by the launcher's Update: no window, keep the shortcuts, restart the launcher, open no page")
	flagNoWindow  = flag.Bool("no-window", false, "Serve the setup page but open no window; its address is in the log (for testing)")
)

// The log: every step is a line in it. The window shows it; a run with no
// window writes it to %TEMP%\Readback-Setup.log.
var (
	logMu  sync.Mutex
	logW   io.Writer = os.Stdout
	logged []string
)

func say(format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Fprintln(logW, line)
	logged = append(logged, line)
}

func logLines() []string {
	logMu.Lock()
	defer logMu.Unlock()
	return append([]string(nil), logged...)
}

func main() {
	flag.Parse()
	if *flagUpdate {
		*flagQuiet = true
	}
	if f, err := os.Create(filepath.Join(os.TempDir(), "Readback-Setup.log")); err == nil {
		logW = f
		defer f.Close()
	}
	say("%s setup %s (%s)", appName, productVersion, version)
	var err error
	switch {
	case !*flagQuiet:
		err = runGUI(*flagUninstall)
	case *flagUninstall:
		err = uninstall(*flagDir)
	default:
		err = install(*flagDir, installOptions{Shortcuts: !*flagNoLinks && !*flagUpdate})
		if err == nil && !*flagNoStart {
			err = startLauncher(*flagDir)
		}
		if err == nil && !*flagNoStart && !*flagUpdate {
			openURL(setupURL())
		}
	}
	if err != nil {
		say("Setup failed: %v", err)
		os.Exit(1)
	}
}

type installOptions struct {
	Shortcuts bool // Start menu, desktop, uninstall entry
	Startup   bool // and start with Windows
}

func install(dir string, o installOptions) error {
	for _, p := range programs {
		if _, err := fs.Stat(payload, "payload/"+p); err != nil {
			return fmt.Errorf("this Readback-Setup.exe was built without %s - rebuild it with build_installer.bat", p)
		}
	}
	say("Installing to %s", dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if n := stopRunning(dir); n > 0 {
		say("Closed %d running Readback program(s)", n)
		time.Sleep(time.Second) // let Windows release the files
	}
	retireSkyEye(dir)
	for _, p := range programs {
		data, err := payload.ReadFile("payload/" + p)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, p)
		tmp := dst + ".new"
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("could not replace %s (is it still running?): %w", p, err)
		}
		say("Copied %s", p)
	}
	for _, g := range guides {
		if data, err := payload.ReadFile("payload/" + g); err == nil {
			os.WriteFile(filepath.Join(dir, g), data, 0o644)
		}
	}
	say("Copied the Installation Guide and User Manual")
	os.WriteFile(filepath.Join(dir, "version.txt"), []byte(productVersion+" ("+version+")\r\n"), 0o644)
	self, _ := os.Executable()
	copyFile(self, filepath.Join(dir, "Readback-Setup.exe")) // kept for --uninstall
	if o.Shortcuts {
		addShortcuts(dir, o.Startup)
	}
	say("Installed %s %s", appName, productVersion)
	return nil
}

// installedVersion is the version.txt of an install in dir, or "".
func installedVersion(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "version.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(b), " ", 2)[0])
}

func startupLink() string { return filepath.Join(startupDir(), appName+".lnk") }

// addShortcuts: Start menu and desktop, the guides, an uninstall entry, and
// Startup when asked (removed when not, so unticking it on an update works).
func addShortcuts(dir string, startup bool) {
	launcher, args := filepath.Join(dir, "launcher.exe"), launcherArgs()
	for _, where := range []string{startMenuDir(), desktopDir()} {
		lnk := filepath.Join(where, appName+".lnk")
		if err := shortcut(lnk, launcher, args, dir, "Start Readback ATC and open its dashboard"); err != nil {
			say("Could not add %s: %v", lnk, err)
		}
	}
	for _, g := range guides {
		if exists(filepath.Join(dir, g)) {
			shortcut(filepath.Join(startMenuDir(), strings.TrimSuffix(g, ".pdf")+".lnk"), filepath.Join(dir, g), "", dir, g)
		}
	}
	shortcut(filepath.Join(startMenuDir(), "Uninstall "+appName+".lnk"),
		filepath.Join(dir, "Readback-Setup.exe"), "--uninstall", dir, "Remove Readback ATC")
	say("Added Start menu and desktop shortcuts")
	if startup {
		if err := shortcut(startupLink(), launcher, args, dir, appName); err == nil {
			say("Readback ATC will start with Windows")
		}
	} else if os.Remove(startupLink()) == nil {
		say("Readback ATC will no longer start with Windows")
	}
}

// launcherArgs: a single-PC install. The fleet list is this PC only (the
// launcher's own default names the vSFG-7 rigs); SRS and Tacview health
// follow what Setup saves.
func launcherArgs() string {
	return fmt.Sprintf("--listen :%d --fleet this=This-PC@127.0.0.1:%d", listenPort, listenPort)
}

func setupURL() string { return fmt.Sprintf("http://localhost:%d/setup", listenPort) }

// startLauncher starts the installed launcher (minimised) unless something
// already answers on its port, and waits for it to come up.
func startLauncher(dir string) error {
	if portInUse(listenPort) {
		say("Readback ATC is already running")
		return nil
	}
	say("Starting Readback ATC")
	cmd := exec.Command("cmd", append([]string{"/c", "start", appName + " launcher", "/min", filepath.Join(dir, "launcher.exe")}, strings.Fields(launcherArgs())...)...)
	cmd.Dir = dir
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the launcher: %w", err)
	}
	for i := 0; i < 30 && !portInUse(listenPort); i++ {
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func uninstall(dir string) error {
	say("Removing %s from %s", appName, dir)
	if n := stopRunning(dir); n > 0 {
		say("Closed %d running Readback program(s)", n)
		time.Sleep(time.Second)
	}
	for _, p := range []string{
		filepath.Join(startMenuDir(), appName+".lnk"),
		filepath.Join(startMenuDir(), "Uninstall "+appName+".lnk"),
		filepath.Join(desktopDir(), appName+".lnk"),
		startupLink(),
		filepath.Join(startMenuDir(), "Readback ATC Installation Guide.lnk"),
		filepath.Join(startMenuDir(), "Readback ATC User Manual.lnk"),
	} {
		if err := os.Remove(p); err == nil {
			say("Removed shortcut %s", filepath.Base(p))
		}
	}
	for _, p := range append(append(programs, guides...), "version.txt") {
		if err := os.Remove(filepath.Join(dir, p)); err == nil {
			say("Removed %s", p)
		} else if !errors.Is(err, fs.ErrNotExist) {
			say("Could not remove %s: %v", p, err)
		}
	}
	say("Your settings, start files and logs are still in %s. Delete that folder to remove them too.", dir)
	return nil
}

// retireSkyEye moves a PC off the test builds from before the rename
// (SkyEye-Setup.exe: %LOCALAPPDATA%\SkyEye, "SkyEye ATC" shortcuts). Left in
// place, its Startup shortcut kept starting the old launcher on port 7000, so
// the new install opened the old dashboard (operator 2026-10-23: the name
// "just shows SKYEYE"). It stops the old programs, removes the old shortcuts
// and copies the settings across where the new folder has none; the old
// folder itself stays.
func retireSkyEye(dir string) {
	old := filepath.Join(os.Getenv("LOCALAPPDATA"), "SkyEye")
	if strings.EqualFold(filepath.Clean(old), filepath.Clean(dir)) || !exists(filepath.Join(old, "launcher.exe")) {
		return
	}
	say("Found the old SkyEye ATC test copy in %s", old)
	if n := stopRunning(old); n > 0 {
		say("Closed %d running SkyEye program(s)", n)
		time.Sleep(time.Second)
	}
	for _, p := range []string{
		filepath.Join(startMenuDir(), "SkyEye ATC.lnk"),
		filepath.Join(startMenuDir(), "Uninstall SkyEye ATC.lnk"),
		filepath.Join(desktopDir(), "SkyEye ATC.lnk"),
		filepath.Join(startupDir(), "SkyEye ATC.lnk"),
	} {
		if err := os.Remove(p); err == nil {
			say("Removed the old shortcut %s", filepath.Base(p))
		}
	}
	// Settings and generated start files; the trial and licence carry over
	// through licence.json and the registry.
	var keep []string
	for _, pat := range []string{"*.json", "start_*.bat"} {
		m, _ := filepath.Glob(filepath.Join(old, pat))
		keep = append(keep, m...)
	}
	if m, _ := filepath.Glob(filepath.Join(old, "branding", "*")); len(m) > 0 {
		os.MkdirAll(filepath.Join(dir, "branding"), 0o755)
		keep = append(keep, m...)
	}
	n := 0
	for _, src := range keep {
		rel, _ := filepath.Rel(old, src)
		dst := filepath.Join(dir, rel)
		if !exists(dst) && copyFile(src, dst) == nil {
			n++
		}
	}
	if n > 0 {
		say("Copied %d settings file(s) from the old copy", n)
	}
	say("The old folder can be deleted once Readback ATC is working")
}

// stopRunning closes Readback programs started from dir, and only those: a
// vSFG-7 rig or another install on the same PC is left alone.
func stopRunning(dir string) int {
	script := fmt.Sprintf(`$d = '%s\*'; $p = Get-Process launcher,atc -ErrorAction SilentlyContinue | Where-Object { $_.Path -like $d }; $p | Stop-Process -Force; ($p | Measure-Object).Count`,
		strings.ReplaceAll(dir, "'", "''"))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// shortcut writes a .lnk through Windows' own shell object.
func shortcut(lnk, target, args, workDir, desc string) error {
	os.MkdirAll(filepath.Dir(lnk), 0o755)
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := fmt.Sprintf(`$s = (New-Object -ComObject WScript.Shell).CreateShortcut(%s); $s.TargetPath = %s; $s.Arguments = %s; $s.WorkingDirectory = %s; $s.Description = %s; $s.IconLocation = %s; $s.Save()`,
		q(lnk), q(target), q(args), q(workDir), q(desc), q(target+",0"))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func startMenuDir() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`)
}
func startupDir() string { return filepath.Join(startMenuDir(), "Startup") }
func desktopDir() string { return filepath.Join(os.Getenv("USERPROFILE"), "Desktop") }

// The other programs Readback needs on this PC; the window lists them with a
// button each, and the launcher's Setup page repeats them.
const (
	dotnetURL = "https://dotnet.microsoft.com/download/dotnet/10.0"
	srsURL    = "https://github.com/ciribob/DCS-SimpleRadioStandalone/releases"
	ffmpegURL = "https://github.com/BtbN/FFmpeg-Builds/releases"
)

type prereq struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Why    string `json:"why"`
	Winget bool   `json:"winget,omitempty"` // can be installed from the window
}

func prerequisites(dir string) []prereq {
	dotnet, _ := filepath.Glob(`C:\Program Files\dotnet\shared\Microsoft.NETCore.App\10.*`)
	_, wingetErr := exec.LookPath("winget")
	return []prereq{
		{Key: "dotnet", Name: "Microsoft .NET 10", OK: len(dotnet) > 0,
			Why: "DCS-SRS 2.4 and its installer run on it. Install the .NET 10 Desktop Runtime (Windows x64) first."},
		{Key: "srs", Name: "DCS-SRS", OK: exists(`C:\Program Files\Eagle Dynamics\SRS\ExternalAudio\DCS-SR-ExternalAudio.exe`),
			Why: "Readback talks on the radio through SRS's ExternalAudio. Install DCS-SRS after .NET 10."},
		{Key: "ffmpeg", Name: "ffmpeg", OK: ffmpegpath.Find(dir) != "", Winget: wingetErr == nil,
			Why: "Readback hears your pilots' calls through it."},
	}
}

var prereqURL = map[string]string{"dotnet": dotnetURL, "srs": srsURL, "ffmpeg": ffmpegURL}

// installFFmpeg runs Windows' own package manager (winget) for ffmpeg with no
// window, after the person clicked Install in the setup window (which says
// that this accepts winget's and the package's terms).
func installFFmpeg(dir string) error {
	say("Installing ffmpeg with winget (%s)", ffmpegpath.WingetID)
	cmd := exec.Command("winget", "install", "--id", ffmpegpath.WingetID, "-e", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if ffmpegpath.Find(dir) != "" {
		say("ffmpeg installed")
		return nil
	}
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return fmt.Errorf("winget did not install ffmpeg (%v): %s", err, strings.TrimSpace(lines[len(lines)-1]))
	}
	return fmt.Errorf("winget finished but ffmpeg was not found yet: restart the PC, then check Setup")
}

func openURL(u string) { exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start() }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func copyFile(src, dst string) error {
	if strings.EqualFold(filepath.Clean(src), filepath.Clean(dst)) {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func portInUse(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
