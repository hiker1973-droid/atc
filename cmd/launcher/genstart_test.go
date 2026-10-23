package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/rolecfg"
	"github.com/vsfg7/atc/pkg/startgen"
)

func withRoot(t *testing.T) string {
	t.Helper()
	oldRoot, oldRoles := rootDir, roles
	rootDir = t.TempDir()
	t.Cleanup(func() { rootDir, roles = oldRoot, oldRoles })
	return rootDir
}

// Setup on a fresh install: the generated files give the launcher one role
// per station, all in the theatre, all started by its region bat.
func TestGenerateStartFiles(t *testing.T) {
	root := withRoot(t)
	o := startgen.Defaults()
	o.Carrier = true
	written, err := generateStartFiles("caucasus", o)
	if err != nil || len(written) != 2 {
		t.Fatalf("%v %v", written, err)
	}
	var names, owners []string
	for _, r := range roles {
		if r.Region != "caucasus" {
			t.Errorf("%s in region %s", r.Name, r.Region)
		}
		names = append(names, r.Name)
		owners = append(owners, rolecfg.OwnerOf(batFlags(r)))
	}
	sort.Strings(owners)
	want := "atis command deckboss gci marshal tower tower tower tower tower tower"
	if got := strings.Join(owners, " "); got != want {
		t.Fatalf("owners %q\nroles %v", got, names)
	}
	if got := regionStartRoles("caucasus"); len(got) != len(roles) {
		t.Fatalf("region start covers %d of %d roles", len(got), len(roles))
	}
	b, _ := os.ReadFile(filepath.Join(root, "start_skyeye_caucasus.bat"))
	if !strings.Contains(string(b), "\r\n") || !strings.Contains(string(b), "--dashboard-port 6016") {
		t.Fatal("roles bat not CRLF, or Mozdok without its dashboard port")
	}
	// Setup run again rewrites its own files.
	if _, err := generateStartFiles("caucasus", startgen.Defaults()); err != nil {
		t.Fatal(err)
	}
	if len(roles) != 9 { // ATIS, 6 towers, Command, GCI
		t.Fatalf("after regenerating without the carrier: %d roles", len(roles))
	}
}

// The vSFG-7 pack (any hand-made bat for the theatre) is never overwritten.
func TestGenerateKeepsHandMadeFiles(t *testing.T) {
	root := withRoot(t)
	mine := filepath.Join(root, "start_towers_syria.bat")
	os.WriteFile(mine, []byte("@echo off\r\n:: vSFG-7\r\n"), 0o644)
	os.WriteFile(filepath.Join(root, "start_launcher.bat"), []byte("@echo off\r\n"), 0o644)
	if _, err := generateStartFiles("syria", startgen.Defaults()); err == nil {
		t.Fatal("generated over a hand-made Syria bat")
	}
	if _, err := os.Stat(filepath.Join(root, "start_skyeye_syria.bat")); err == nil {
		t.Fatal("wrote a file anyway")
	}
	// start_launcher.bat is the launcher's own: Persian Gulf is still free.
	if _, err := generateStartFiles("pg", startgen.Defaults()); err != nil {
		t.Fatal(err)
	}
}
