package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/rolecfg"
)

// Settings > Roles > Other > Add: a second AWACS and a second Command, made
// from the vSFG-7 Caucasus bats, and a third JTAC.
func TestAddedStations(t *testing.T) {
	root := withRoot(t)
	for _, bat := range []string{"start_gci_caucasus.bat", "start_command_caucasus.bat"} {
		b, err := os.ReadFile(filepath.Join("..", "..", bat))
		if err != nil {
			t.Skip("no vSFG-7 bats here: " + err.Error())
		}
		os.WriteFile(filepath.Join(root, bat), b, 0o644)
	}
	discoverRoles()

	if err := setAdded("caucasus", "", rolecfg.AddedStation{Kind: "gci", Name: "Magic", Freq: 264}, false); err == nil {
		t.Fatal("a second AWACS on Darkstar's 264")
	}
	if err := setAdded("caucasus", "", rolecfg.AddedStation{Kind: "gci", Name: "Darkstar", Freq: 265}, false); err == nil {
		t.Fatal("a second Darkstar")
	}
	if err := setAdded("caucasus", "", rolecfg.AddedStation{Kind: "gci", Name: "Magic", Freq: 265, Voice: "sage"}, false); err != nil {
		t.Fatal(err)
	}
	if err := setAdded("caucasus", "", rolecfg.AddedStation{Kind: "command", Name: "Second-Command", Freq: 283}, false); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "start_added_caucasus.bat"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "\r\n") || !strings.Contains(text, addedMarker) || !strings.Contains(text, "set SRS=%SKYEYE_SRS%") {
		t.Fatalf("bat:\n%s", text)
	}

	// The roles: found, in Caucasus, with their own logs, apart from the theatre's own.
	var gci, cmd *Role
	for i := range roles {
		switch roles[i].Name {
		case "Added AWACS Magic (Caucasus)":
			gci = &roles[i]
		case "Added Command Second-Command (Caucasus)":
			cmd = &roles[i]
		}
	}
	if gci == nil || cmd == nil || gci.Region != "caucasus" || gci.LogFile != "atc-gci-gci2.log" || cmd.LogFile != "atc-command-command2.log" {
		t.Fatalf("added roles %+v %+v", gci, cmd)
	}
	gf, cf := batFlags(*gci), batFlags(*cmd)
	if gf["gci-callsign"] != "Magic" || gf["gci-freq"] != "265" || gf["gci-voice"] != "sage" || gf["added-station"] != "gci2" ||
		gf["map"] != "caucasus" || gf["srs-addr"] != "%SRS%" {
		t.Errorf("AWACS flags %v", gf)
	}
	for _, f := range []string{"tanker-freq", "tanker2-freq", "tanker-net", "jtac-freq", "jtac2-freq", "pprof-port"} {
		if _, ok := cf[f]; ok {
			t.Errorf("second Command still has --%s: %s", f, cmd.cmd)
		}
	}
	if cf["command-freq"] != "283" || cf["command-name"] != "Second-Command" || cf["handoff-marshal-freq"] != "0" ||
		cf["handoff-watch"] != "false" || cf["added-station"] != "command2" || !strings.Contains(cmd.cmd, "%TACVIEW_FLAG%") {
		t.Errorf("Command flags %v\n%s", cf, cmd.cmd)
	}
	// The theatre's own stay what Settings shows as Command and GCI.
	v := theatreRoles("caucasus", mustCfg(t))
	for _, s := range v.Stations {
		if s.Key == "command" && s.Role != "vSFG-7 Command (Caucasus)" {
			t.Errorf("Settings' Command is %s", s.Role)
		}
	}
	if len(v.Added) != 2 || !v.CanJTAC {
		t.Fatalf("view: added %v, canJTAC %v", v.Added, v.CanJTAC)
	}
	if dump := os.Getenv("ADDED_BAT_DUMP"); dump != "" {
		os.WriteFile(dump, body, 0o644)
	}

	// A third JTAC, on the whole range.
	if err := setJTAC("caucasus", "", rolecfg.JTACNet{Name: "Hitman-3-1", Freq: 61}, false); err == nil {
		t.Fatal("a JTAC on Hitman 1-1's 61.000")
	}
	if err := setJTAC("caucasus", "", rolecfg.JTACNet{Name: "Hitman-3-1", Freq: 63}, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(mustCfg(t).JTACNets("caucasus"), " "); got != "63/Hitman-3-1/all/george" {
		t.Fatalf("jtac nets %q", got)
	}

	// Removing both added stations removes the bat.
	if err := setAdded("caucasus", "gci2", rolecfg.AddedStation{}, true); err != nil {
		t.Fatal(err)
	}
	if err := setAdded("caucasus", "command2", rolecfg.AddedStation{}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "start_added_caucasus.bat")); !os.IsNotExist(err) {
		t.Fatal("start_added_caucasus.bat still there")
	}
}

func mustCfg(t *testing.T) rolecfg.File {
	t.Helper()
	f, err := rolecfg.Read(rolesCfgPath())
	if err != nil {
		t.Fatal(err)
	}
	return f
}
