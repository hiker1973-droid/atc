package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paulmach/orb"
)

// footholdSaveSample is written the way Utils.saveTable / serializeValue
// write it: a statement per key, tables with [key]= entries, %q strings.
const footholdSaveSample = `zonePersistance = {}
zonePersistance["zones"] = {}
zonePersistance["zones"]["Akrotiri"] = {
  ["side"]=2,
  ["active"]=true,
  ["level"]=3,
  ["remainingUnits"]={
    [1]={
      [1]="Hawk ln",
    },
  },
  ["lat_long"]={
    ["latitude"]=34.59,
    ["longitude"]=32.98,
    ["altitude"]=23.5,
  },
  ["triggers"]={
  },
}
zonePersistance["zones"]["Bassel Al-Assad"] = {
  ["side"]=1,
  ["active"]=true,
  ["remainingUnits"]={
    [1]={
      [1]="Kub 2P25 ln",
      [2]="Kub 1S91 str",
    },
    [2]={
      [1]="Osa 9A33 ln",
      [2]="ZSU-23-4 Shilka",
      [3]="Ural-375",
    },
  },
  ["lat_long"]={
    ["latitude"]=35.40,
    ["longitude"]=35.95,
    ["altitude"]=-nan(ind),
  },
  ["flavor"]="line one\
line two \"quoted\"",
}
zonePersistance["zones"]["Hama"] = {
  ["side"]=1,
  ["active"]=true,
  ["remainingUnits"]={
    [1]={
      [1]="SA-11 Buk LN 9A310M1",
    },
  },
  ["lat_long"]={
    ["latitude"]=35.12,
    ["longitude"]=36.71,
    ["altitude"]=1.#INF,
  },
}
zonePersistance["zones"]["Empty Red"] = {
  ["side"]=1,
  ["active"]=true,
  ["remainingUnits"]={
  },
  ["lat_long"]={
    ["latitude"]=35.0,
    ["longitude"]=34.0,
  },
}
zonePersistance["zones"]["Dormant"] = {
  ["side"]=1,
  ["active"]=false,
  ["remainingUnits"]={
    [1]={
      [1]="T-72B",
    },
  },
  ["lat_long"]={
    ["latitude"]=34.8,
    ["longitude"]=33.5,
  },
}
zonePersistance["accounts"] = {
  [1]=1200,
  [2]=5e+15,
}
zonePersistance["customFlags"] = {
  ["StartNormal"]=true,
}
`

func TestParseFootholdSave(t *testing.T) {
	zones, err := parseFootholdSave(footholdSaveSample)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 5 {
		t.Fatalf("got %d zones, want 5", len(zones))
	}
	byName := map[string]footholdZone{}
	for _, z := range zones {
		byName[z.Name] = z
	}
	b := byName["Bassel Al-Assad"]
	if b.Side != 1 || !b.Active || b.Pos != (orb.Point{35.95, 35.40}) {
		t.Errorf("Bassel parsed as %+v", b)
	}
	if strings.Join(b.Units, "|") != "Kub 2P25 ln|Kub 1S91 str|Osa 9A33 ln|ZSU-23-4 Shilka|Ural-375" {
		t.Errorf("Bassel units %v", b.Units)
	}
	if byName["Dormant"].Active {
		t.Error("Dormant should be inactive")
	}
}

func TestFootholdThreats(t *testing.T) {
	got := strings.Join(footholdThreats([]string{"Ural-375", "ZSU-23-4 Shilka", "Osa 9A33 ln", "Kub 1S91 str", "SA-18 Igla manpad"}), ", ")
	if got != "SA-6, SA-8, Shilka, MANPADS" {
		t.Errorf("threats %q", got)
	}
}

func TestPickFootholdZone(t *testing.T) {
	zones, _ := parseFootholdSave(footholdSaveSample)
	// No position: the red zone nearest blue ground (Akrotiri) with units.
	z, ok := pickFootholdZone(zones, nil, "", orb.Point{}, false)
	if !ok || z.Name != "Bassel Al-Assad" {
		t.Fatalf("front pick = %q %v, want Bassel Al-Assad", z.Name, ok)
	}
	// A caller sitting over Hama gets Hama (both are on the front set).
	z, _ = pickFootholdZone(zones, nil, "", orb.Point{36.7, 35.1}, true)
	if z.Name != "Hama" {
		t.Errorf("nearest pick = %q, want Hama", z.Name)
	}
	// Held by another flight: the other one.
	z, _ = pickFootholdZone(zones, map[string]bool{"Bassel Al-Assad": true}, "", orb.Point{}, false)
	if z.Name != "Hama" {
		t.Errorf("held pick = %q, want Hama", z.Name)
	}
	// Everything held: still a zone rather than nothing.
	if _, ok := pickFootholdZone(zones, map[string]bool{"Bassel Al-Assad": true, "Hama": true}, "", orb.Point{}, false); !ok {
		t.Error("all held: want a fallback zone")
	}
}

func TestFootholdTaskingExchange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "footholdSyria_Extended_0.1.lua")
	if err := os.WriteFile(path, []byte(footholdSaveSample), 0o644); err != nil {
		t.Fatal(err)
	}
	f := newFootholdTasking(&footholdSave{paths: func() []string { return []string{path} }})
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	f.now = func() time.Time { return now }

	got := f.Response("Command, Raider 32, request tasking", "Raider 32", "Command", nil)
	want := "Raider 32, Command, tasking Bassel Al Assad. Threats SA-6, SA-8, Shilka."
	if got != want {
		t.Fatalf("tasking:\n got %q\nwant %q", got, want)
	}
	if got := f.Response("Raider 32, copy", "Raider 32", "Command", nil); got != "Raider 32, Command, Bassel Al Assad, cleared hot." {
		t.Errorf("readback: %q", got)
	}
	// Raider asks again: not the zone it just had.
	if got := f.Response("Command, Raider 32, request new tasking", "Raider 32", "Command", nil); !strings.Contains(got, "tasking Hama") {
		t.Errorf("repeat request: %q", got)
	}
	// A second flight is not given the zone Raider now holds.
	if got := f.Response("Command, Venom 11, request tasking", "Venom 11", "Command", nil); !strings.Contains(got, "tasking Bassel") {
		t.Errorf("second flight: %q", got)
	}
}

func TestFootholdTaskingNoSave(t *testing.T) {
	f := newFootholdTasking(&footholdSave{paths: func() []string { return []string{filepath.Join(t.TempDir(), "missing.lua")} }})
	if got := f.Response("Command, Raider 32, request tasking", "Raider 32", "Command", nil); got != "Raider 32, Command, negative tasking at this time." {
		t.Errorf("no save: %q", got)
	}
}
