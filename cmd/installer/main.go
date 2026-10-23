// SkyEye-Setup.exe: installs SkyEye ATC on a PC that has none of the vSFG-7
// setup (operator 2026-10-23: "are we building a .exe", "i can install on my
// laptop"). It carries atc.exe and launcher.exe (build_installer.bat puts
// them in payload/), copies them to %LOCALAPPDATA%\SkyEye (no admin needed),
// adds Start-menu and desktop shortcuts, checks the prerequisites, starts the
// launcher and opens its Setup page, which does the rest: keys, SRS server,
// squadron, theatre (generated start files) and a test call.
//
//	SkyEye-Setup.exe               install or update, then start
//	SkyEye-Setup.exe --uninstall   remove the programs and shortcuts (settings and logs stay)
package main

import (
	"bufio"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	flagQuiet     = flag.Bool("quiet", false, "Ask nothing: no start-with-Windows shortcut, no pause at the end")
	flagNoStart   = flag.Bool("no-start", false, "Install only; do not start the launcher")
	flagNoLinks   = flag.Bool("no-shortcuts", false, "Add no shortcuts (for testing)")
	flagUpdate    = flag.Bool("update", false, "Run by the launcher's Update: ask nothing, keep the shortcuts, restart the launcher, open no page")
)

func main() {
	flag.Parse()
	if *flagUpdate {
		*flagQuiet = true
	}
	fmt.Printf("%s setup %s (%s)\n\n", appName, productVersion, version)
	var err error
	if *flagUninstall {
		err = uninstall(*flagDir)
	} else {
		err = install(*flagDir)
	}
	if err != nil {
		fmt.Printf("\nSetup failed: %v\n", err)
	}
	if !*flagQuiet {
		fmt.Print("\nPress Enter to close.")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
	if err != nil {
		os.Exit(1)
	}
}

func install(dir string) error {
	for _, p := range programs {
		if _, err := fs.Stat(payload, "payload/"+p); err != nil {
			return fmt.Errorf("this Readback-Setup.exe was built without %s - rebuild it with build_installer.bat", p)
		}
	}
	fmt.Printf("Installing to %s\n", dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if n := stopRunning(dir); n > 0 {
		fmt.Printf("  closed %d running Readback program(s) from this folder\n", n)
		time.Sleep(time.Second) // let Windows release the files
	}
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
		fmt.Printf("  %s\n", p)
	}
	for _, g := range guides {
		if data, err := payload.ReadFile("payload/" + g); err == nil {
			os.WriteFile(filepath.Join(dir, g), data, 0o644)
		}
	}
	os.WriteFile(filepath.Join(dir, "version.txt"), []byte(productVersion+" ("+version+")\r\n"), 0o644)
	self, _ := os.Executable()
	copyFile(self, filepath.Join(dir, "Readback-Setup.exe")) // kept for --uninstall

	launcher := filepath.Join(dir, "launcher.exe")
	args := launcherArgs()
	switch {
	case *flagNoLinks:
		fmt.Println("\nNo shortcuts (--no-shortcuts)")
	case *flagUpdate:
		// An update keeps whatever shortcuts the install has.
	default:
		addShortcuts(dir, launcher, args)
	}

	checkPrerequisites()

	if *flagNoStart {
		fmt.Printf("\nInstalled. Start it from the Start menu: %s\n", appName)
		return nil
	}
	if portInUse(listenPort) {
		fmt.Printf("\nSomething is already using port %d - another launcher? Opening it.\n", listenPort)
	} else {
		fmt.Println("\nStarting the launcher...")
		cmd := exec.Command("cmd", append([]string{"/c", "start", appName + " launcher", "/min", launcher}, strings.Fields(args)...)...)
		cmd.Dir = dir
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("could not start the launcher: %w", err)
		}
		for i := 0; i < 30 && !portInUse(listenPort); i++ {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if *flagUpdate {
		// The dashboard that started the update reconnects by itself.
		fmt.Printf("Updated to %s.\n", productVersion)
		return nil
	}
	url := fmt.Sprintf("http://localhost:%d/setup", listenPort)
	openURL(url)
	fmt.Printf("Done. Setup is open in your browser: %s\n", url)
	return nil
}

// addShortcuts: Start menu and desktop, an uninstall entry, and on request
// Startup so SkyEye runs when Windows starts.
func addShortcuts(dir, launcher, args string) {
	fmt.Println("\nShortcuts")
	for _, where := range []string{startMenuDir(), desktopDir()} {
		lnk := filepath.Join(where, appName+".lnk")
		if err := shortcut(lnk, launcher, args, dir, "Start Readback ATC and open its dashboard"); err != nil {
			fmt.Printf("  could not add %s: %v\n", lnk, err)
		} else {
			fmt.Printf("  %s\n", lnk)
		}
	}
	for _, g := range guides {
		if exists(filepath.Join(dir, g)) {
			shortcut(filepath.Join(startMenuDir(), strings.TrimSuffix(g, ".pdf")+".lnk"), filepath.Join(dir, g), "", dir, g)
		}
	}
	shortcut(filepath.Join(startMenuDir(), "Uninstall "+appName+".lnk"),
		filepath.Join(dir, "Readback-Setup.exe"), "--uninstall", dir, "Remove Readback ATC")
	if !*flagQuiet && ask("Start Readback ATC when Windows starts? [y/N] ", false) {
		if err := shortcut(filepath.Join(startupDir(), appName+".lnk"), launcher, args, dir, appName); err == nil {
			fmt.Println("  added to Startup")
		}
	}
}

// launcherArgs: a single-PC install. The fleet list is this PC only (the
// launcher's own default names the vSFG-7 rigs); SRS and Tacview health
// follow what Setup saves.
func launcherArgs() string {
	return fmt.Sprintf("--listen :%d --fleet this=This-PC@127.0.0.1:%d", listenPort, listenPort)
}

func uninstall(dir string) error {
	fmt.Printf("Removing %s from %s\n", appName, dir)
	if n := stopRunning(dir); n > 0 {
		fmt.Printf("  closed %d running Readback program(s)\n", n)
		time.Sleep(time.Second)
	}
	for _, p := range []string{
		filepath.Join(startMenuDir(), appName+".lnk"),
		filepath.Join(startMenuDir(), "Uninstall "+appName+".lnk"),
		filepath.Join(desktopDir(), appName+".lnk"),
		filepath.Join(startupDir(), appName+".lnk"),
		filepath.Join(startMenuDir(), "Readback ATC Installation Guide.lnk"),
		filepath.Join(startMenuDir(), "Readback ATC User Manual.lnk"),
	} {
		if err := os.Remove(p); err == nil {
			fmt.Printf("  removed %s\n", p)
		}
	}
	for _, p := range append(append(programs, guides...), "version.txt") {
		if err := os.Remove(filepath.Join(dir, p)); err == nil {
			fmt.Printf("  removed %s\n", p)
		} else if !errors.Is(err, fs.ErrNotExist) {
			fmt.Printf("  could not remove %s: %v\n", p, err)
		}
	}
	fmt.Printf("\nYour settings, start files and logs are still in %s.\nDelete that folder to remove them too.\n", dir)
	return nil
}

// stopRunning closes SkyEye programs started from dir, and only those: a
// vSFG-7 rig or another install on the same PC is left alone.
func stopRunning(dir string) int {
	script := fmt.Sprintf(`$d = '%s\*'; $p = Get-Process launcher,atc -ErrorAction SilentlyContinue | Where-Object { $_.Path -like $d }; $p | Stop-Process -Force; ($p | Measure-Object).Count`,
		strings.ReplaceAll(dir, "'", "''"))
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
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
	if out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func startMenuDir() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`)
}
func startupDir() string { return filepath.Join(startMenuDir(), "Startup") }
func desktopDir() string { return filepath.Join(os.Getenv("USERPROFILE"), "Desktop") }

// checkPrerequisites says what else the PC needs; the Setup page repeats it.
const (
	dotnetURL = "https://dotnet.microsoft.com/download/dotnet/10.0"
	srsURL    = "https://github.com/ciribob/DCS-SimpleRadioStandalone/releases"
)

func checkPrerequisites() {
	fmt.Println("\nChecking this PC")
	dotnet, _ := filepath.Glob(`C:\Program Files\dotnet\shared\Microsoft.NETCore.App\10.*`)
	report(len(dotnet) > 0, "Microsoft .NET 10",
		"needed by DCS-SRS 2.4 and its installer: install the .NET 10 Desktop Runtime (Windows x64) from "+dotnetURL)
	srs := exists(`C:\Program Files\Eagle Dynamics\SRS\ExternalAudio\DCS-SR-ExternalAudio.exe`)
	report(srs, "DCS-SRS (ExternalAudio)",
		"Readback talks on SRS through it: install DCS-SRS (after .NET 10) from "+srsURL)
	report(exists(`C:\ffmpeg-master-latest-win64-gpl\bin\ffmpeg.exe`), "ffmpeg (radio effect)",
		"optional: without it calls go out clean. Unzip an ffmpeg build to C:\\ffmpeg-master-latest-win64-gpl")
	// The console's links cannot be clicked: offer to open the pages.
	if len(dotnet) == 0 && !*flagQuiet && ask("\nOpen the .NET 10 download page now? [Y/n] ", true) {
		openURL(dotnetURL)
	}
	if !srs && !*flagQuiet && ask("Open the DCS-SRS download page now? [Y/n] ", true) {
		openURL(srsURL)
	}
}

func openURL(u string) { exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start() }

func report(ok bool, what, fix string) {
	if ok {
		fmt.Printf("  ok    %s\n", what)
	} else {
		fmt.Printf("  MISSING  %s - %s\n", what, fix)
	}
}

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

// ask is a yes/no question; Enter alone gives def.
func ask(q string, def bool) bool {
	fmt.Print(q)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch a := strings.ToLower(strings.TrimSpace(line)); {
	case a == "":
		return def
	default:
		return strings.HasPrefix(a, "y")
	}
}
