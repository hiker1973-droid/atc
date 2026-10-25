package main

import (
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/rolecfg"
	"github.com/vsfg7/atc/pkg/startgen"
)

// Settings > Roles in three sections: an ATIS added at a field with none, a
// start-script tanker changed and removed, tankers added, a tower renamed -
// and what Command and the tower will run from it.
func TestRoleSections(t *testing.T) {
	withRoot(t)
	o := startgen.Defaults()
	o.Carrier = true
	if _, err := generateStartFiles("caucasus", o); err != nil {
		t.Fatal(err)
	}
	for i := range roles { // a start-script tanker, as the vSFG-7 Command bat has
		if rolecfg.OwnerOf(batFlags(roles[i])) == rolecfg.OwnerCommand {
			roles[i].cmd += " --tanker-net 253.1/Shell-1-1/22/288/boom/charlie"
		}
	}
	cfg := func() rolecfg.File {
		f, err := rolecfg.Read(rolesCfgPath())
		if err != nil {
			t.Fatal(err)
		}
		return f
	}

	v := theatreRoles("caucasus", cfg())
	if !v.HasATIS || !v.HasCommand || len(v.Nets) != 1 || v.Nets[0].Key != "Shell-1-1" {
		t.Fatalf("view %+v", v)
	}
	noATIS := ""
	for _, f := range v.NoATIS {
		noATIS += f.ICAO + " "
	}
	if noATIS != "UGTB " {
		t.Fatalf("fields with no ATIS: %q", noATIS)
	}

	// ATIS: a new one at Tbilisi, not on a frequency already in use.
	if err := setATIS("caucasus", "UGTB", rolecfg.ATIS{Added: true, Freq: 230}); err == nil || !strings.Contains(err.Error(), "Batumi ATIS") {
		t.Fatalf("Batumi ATIS's 230.000 taken twice: %v", err)
	}
	if err := setATIS("caucasus", "UGTB", rolecfg.ATIS{Added: true, Freq: 235, Name: "Tbilisi ATIS"}); err != nil {
		t.Fatal(err)
	}
	if a := cfg().Theatres["caucasus"].ATIS["UGTB"]; !a.Added || a.Freq != 235 || a.Name != "" {
		t.Fatalf("Tbilisi ATIS stored as %+v (the default name is not stored)", a)
	}
	// Batumi's: off the air; its frequency stays the Frequencies table's.
	if err := setATIS("caucasus", "UGSB", rolecfg.ATIS{Off: true, Freq: 231}); err != nil {
		t.Fatal(err)
	}
	if a := cfg().Theatres["caucasus"].ATIS["UGSB"]; !a.Off || a.Added || a.Freq != 0 {
		t.Fatalf("Batumi ATIS stored as %+v", a)
	}
	// Removing the added one forgets it.
	if err := setATIS("caucasus", "UGTB", rolecfg.ATIS{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg().Theatres["caucasus"].ATIS["UGTB"]; ok {
		t.Fatal("removed Tbilisi ATIS still stored")
	}

	// Tankers: the script's Shell moved, a new Arco added, then removed.
	if err := setNet("caucasus", "Shell-1-1", rolecfg.Override{Freq: 253.4, Eleven: "charlie"}); err != nil {
		t.Fatal(err)
	}
	if err := setTanker("caucasus", "", rolecfg.TankerNet{Name: "Arco-1-1", Freq: 253.4, Angels: 20, KIAS: 280}, false); err == nil {
		t.Fatal("a second tanker on Shell's frequency")
	}
	if err := setTanker("caucasus", "", rolecfg.TankerNet{Name: "Shell-1-1", Freq: 253.6, Angels: 20, KIAS: 280}, false); err == nil {
		t.Fatal("a second Shell-1-1")
	}
	if err := setTanker("caucasus", "", rolecfg.TankerNet{Name: "Arco 1", Freq: 253.6, Angels: 20, KIAS: 280}, false); err == nil {
		t.Fatal("a callsign with a space")
	}
	if err := setTanker("caucasus", "", rolecfg.TankerNet{Name: "Arco-1-1", Freq: 253.6, Angels: 20, KIAS: 280, Boom: true}, false); err != nil {
		t.Fatal(err)
	}
	nets := cfg().TankerNets("caucasus", []string{"253.1/Shell-1-1/22/288/boom/charlie"})
	if got := strings.Join(nets, " "); got != "253.4/Shell-1-1/22/288/boom/charlie 253.6/Arco-1-1/20/280/boom/eric" {
		t.Fatalf("Command's tankers: %s", got)
	}
	if err := setTanker("caucasus", "Arco-1-1", rolecfg.TankerNet{}, true); err != nil {
		t.Fatal(err)
	}
	if err := setNet("caucasus", "Shell-1-1", rolecfg.Override{Off: true}); err != nil {
		t.Fatal(err)
	}
	if nets := cfg().TankerNets("caucasus", []string{"253.1/Shell-1-1/22/288/boom/charlie"}); len(nets) != 0 {
		t.Fatalf("tankers after removing both: %v", nets)
	}

	// A tower's name, with spaces: what it calls itself.
	if err := setRoleOverride("caucasus", "", "UGKS", rolecfg.Override{Name: "Senaki Approach"}); err != nil {
		t.Fatal(err)
	}
	fl := map[string]string{"callsign": "", "tts-voice": "nova"}
	res, err := cfg().Apply("caucasus", rolecfg.OwnerTower, "UGKS", mapFlags(fl))
	if err != nil || fl["callsign"] != "Senaki Approach" {
		t.Fatalf("tower flags %v %v %v", fl, res, err)
	}
	// Back to the default name: nothing stored.
	if err := setRoleOverride("caucasus", "", "UGKS", rolecfg.Override{Name: "Senaki Tower"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg().Theatres["caucasus"].Towers["UGKS"]; ok {
		t.Fatal("the default tower name was stored")
	}
}

type mapFlags map[string]string

func (m mapFlags) Get(name string) (string, bool) { v, ok := m[name]; return v, ok }
func (m mapFlags) Set(name, value string) error   { m[name] = value; return nil }
