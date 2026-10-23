package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The region bat starts the towers and Command but not the on-demand RED
// card, so "all running" must not wait for Red Command.
func TestRegionStartRolesSkipsOnDemand(t *testing.T) {
	oldRoot := rootDir
	rootDir = t.TempDir()
	t.Cleanup(func() { rootDir = oldRoot })
	bat := "call \"%~dp0start_towers_caucasus.bat\"\r\ncall \"%~dp0start_command_caucasus.bat\"\r\n"
	if err := os.WriteFile(filepath.Join(rootDir, "start_region_caucasus.bat"), []byte(bat), 0o644); err != nil {
		t.Fatal(err)
	}
	rolesMu.Lock()
	oldRoles := roles
	roles = []Role{
		{Name: "Senaki Tower", Bat: "start_towers_caucasus.bat", Region: "caucasus"},
		{Name: "vSFG-7 Command (Caucasus)", Bat: "start_command_caucasus.bat", Region: "caucasus"},
		{Name: "vSFG-7 Red Command (Caucasus)", Bat: "start_red_caucasus.bat", Region: "caucasus"},
		{Name: "Incirlik Tower", Bat: "start_towers_syria.bat", Region: "syria"},
	}
	rolesMu.Unlock()
	t.Cleanup(func() { rolesMu.Lock(); roles = oldRoles; rolesMu.Unlock() })

	got := strings.Join(regionStartRoles("caucasus"), ", ")
	if got != "Senaki Tower, vSFG-7 Command (Caucasus)" {
		t.Fatalf("region start roles = %q", got)
	}
}
