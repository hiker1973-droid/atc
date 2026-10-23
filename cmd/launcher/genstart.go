package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/startgen"
)

// Generated start files (pkg/startgen): Setup writes a theatre's standard set
// on an install that has none. They are ordinary bats, so the role list,
// region Start/Stop, Restart and Update work on them unchanged. Nothing here
// overwrites a bat without startgen.Marker: the vSFG-7 pack stays as it is.

// handMade reports whether a file exists and was not written by Setup.
func handMade(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return !strings.Contains(string(b), startgen.Marker)
}

// theatreHasHandMadeFiles: the vSFG-7 pack (or any operator's own bats)
// already covers this theatre.
func theatreHasHandMadeFiles(theatre string) bool {
	entries, _ := os.ReadDir(rootDir)
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if n == "start_launcher.bat" { // the launcher's own, not a theatre's
			continue
		}
		if strings.HasPrefix(n, "start_") && strings.HasSuffix(n, ".bat") &&
			regionForBat(n) == theatre && handMade(filepath.Join(rootDir, e.Name())) {
			return true
		}
	}
	return false
}

// theatreGenerated: Setup wrote this theatre's start files.
func theatreGenerated(theatre string) bool {
	b, err := os.ReadFile(filepath.Join(rootDir, startgen.RolesBat(theatre)))
	return err == nil && strings.Contains(string(b), startgen.Marker)
}

func theatreLabel(theatre string) string {
	for _, t := range settingsTheatres {
		if t.Key == theatre {
			return t.Label
		}
	}
	return theatre
}

// generateStartFiles writes a theatre's standard start files, refusing to
// touch any hand-made one, and rescans the roles.
func generateStartFiles(theatre string, o startgen.Options) ([]string, error) {
	region := regionBats[theatre]
	if region == "" {
		return nil, fmt.Errorf("unknown theatre %q", theatre)
	}
	if theatreHasHandMadeFiles(theatre) {
		return nil, fmt.Errorf("%s already has its own start files - kept as they are", theatreLabel(theatre))
	}
	now := time.Now()
	roles, err := startgen.RolesText(theatre, theatreLabel(theatre), o, now)
	if err != nil {
		return nil, err
	}
	files := []struct{ name, text string }{
		{startgen.RolesBat(theatre), roles},
		{region, startgen.RegionText(theatre, theatreLabel(theatre), now)},
	}
	var written []string
	for _, f := range files {
		text := strings.ReplaceAll(f.text, "\n", "\r\n") // cmd.exe wants CRLF
		if err := os.WriteFile(filepath.Join(rootDir, f.name), []byte(text), 0o644); err != nil {
			return written, err
		}
		written = append(written, f.name)
	}
	discoverRoles()
	return written, nil
}
