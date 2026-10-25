package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/rolecfg"
)

// TestHelperFlagDefaults is not a test: TestRoleCatalogDefaults runs it in a
// child process, because registering the flags resets every flag global.
func TestHelperFlagDefaults(t *testing.T) {
	if os.Getenv("SKYEYE_DUMP_FLAGS") == "" {
		t.Skip("helper")
	}
	out := map[string]string{}
	newRootCommand().Flags().VisitAll(func(f *pflag.Flag) { out[f.Name] = f.DefValue })
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func flagDefaults(t *testing.T) map[string]string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperFlagDefaults$")
	cmd.Env = append(os.Environ(), "SKYEYE_DUMP_FLAGS=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		var m map[string]string
		if json.Unmarshal([]byte(line), &m) == nil {
			return m
		}
	}
	t.Fatalf("no flag dump in %q", out)
	return nil
}

// The Settings page shows rolecfg.Defaults for a flag a bat does not name, and
// writes the catalog's flag names: both must be atc.exe's.
func TestRoleCatalogDefaults(t *testing.T) {
	defs := flagDefaults(t)
	same := func(a, b string) bool {
		fa, ea := strconv.ParseFloat(a, 64)
		fb, eb := strconv.ParseFloat(b, 64)
		if ea == nil && eb == nil {
			return fa == fb
		}
		return a == b
	}
	for name, want := range rolecfg.Defaults {
		got, ok := defs[name]
		if !ok {
			t.Errorf("rolecfg.Defaults names %q, which atc.exe does not have", name)
		} else if !same(got, want) {
			t.Errorf("--%s: atc.exe default %q, rolecfg.Defaults %q", name, got, want)
		}
	}
	for _, s := range rolecfg.Stations {
		for _, f := range append([]string{s.Freq, s.Name, s.Voice, s.Eleven, s.Speed}, append(s.FreqRefs, s.NameRefs...)...) {
			if f == "" {
				continue
			}
			if _, ok := defs[f]; !ok {
				t.Errorf("station %s: atc.exe has no --%s", s.Key, f)
			}
		}
	}
}

// A Settings change reaches the flags as the bat would have given them:
// Marshal moved to 306.5 changes Marshal and Command's handoff, but not Red
// Command's "no Marshal" 0; Hitman 2-1 switched off; Command's move carries
// the towers' handoff along.
func TestRoleConfigApply(t *testing.T) {
	p := filepath.Join(t.TempDir(), "roles.json")
	os.WriteFile(p, []byte(`{"theatres":{"caucasus":{
		"stations":{"marshal":{"freq":306.5,"eleven":"abc"},"hitman2":{"off":true},"command":{"freq":283,"voice":"sage"},
		            "gci":{"name":"Magic"}},
		"towers":{"XRMF":{"voice":"echo","speed":1.3}}}}}`), 0o644)
	old := rolesPath
	rolesPath = func() string { return p }
	t.Cleanup(func() { rolesPath = old })

	run := func(args ...string) (*pflag.FlagSet, rolecfg.Result) {
		fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
		fs.String("map", "pg", "")
		fs.String("airfield", "OMDM", "")
		fs.Bool("command-only", false, "")
		fs.Bool("marshal-only", false, "")
		fs.String("coalition", "blue", "")
		fs.String("command-freq", "0", "")
		fs.String("command-voice", "onyx", "")
		fs.String("command-gci-name", "", "")
		fs.Float64("jtac2-freq", 0, "")
		fs.Float64("handoff-marshal-freq", 306.3, "")
		fs.String("marshal-freq", "0", "")
		fs.String("elevenlabs-marshal-voice", "x", "")
		fs.String("tts-voice", "nova", "")
		fs.Float64("tts-speed", 1.2, "")
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		f, _ := rolecfg.Read(p)
		given := map[string]string{}
		fs.Visit(func(fl *pflag.Flag) { given[fl.Name] = fl.Value.String() })
		th := airfield.TheatreKey(fs.Lookup("map").Value.String())
		if !fs.Changed("map") {
			th = airfield.TheatreOfICAO(fs.Lookup("airfield").Value.String())
		}
		res, err := f.Apply(th, rolecfg.OwnerOf(given), fs.Lookup("airfield").Value.String(), pflagFlags{fs})
		if err != nil {
			t.Fatal(err)
		}
		return fs, res
	}
	get := func(fs *pflag.FlagSet, n string) string { return fs.Lookup(n).Value.String() }

	fs, _ := run("--command-only", "--map", "caucasus", "--command-freq", "282.0", "--jtac2-freq", "62", "--command-gci-name", "Darkstar")
	if get(fs, "command-freq") != "283" || get(fs, "command-voice") != "sage" || get(fs, "jtac2-freq") != "0" ||
		get(fs, "handoff-marshal-freq") != "306.5" || get(fs, "command-gci-name") != "Magic" {
		t.Fatalf("blue Command: %s %s %s %s %s", get(fs, "command-freq"), get(fs, "command-voice"),
			get(fs, "jtac2-freq"), get(fs, "handoff-marshal-freq"), get(fs, "command-gci-name"))
	}
	fs, _ = run("--command-only", "--coalition", "red", "--map", "caucasus", "--command-freq", "280.0", "--handoff-marshal-freq", "0")
	if get(fs, "command-freq") != "280.0" || get(fs, "command-voice") != "onyx" || get(fs, "handoff-marshal-freq") != "0" {
		t.Fatalf("Red Command took blue's settings: %s %s %s", get(fs, "command-freq"), get(fs, "command-voice"), get(fs, "handoff-marshal-freq"))
	}
	fs, _ = run("--marshal-only", "--airfield", "UGSB", "--marshal-freq", "306.3")
	if get(fs, "marshal-freq") != "306.5" || get(fs, "elevenlabs-marshal-voice") != "abc" {
		t.Fatalf("Marshal: %s %s", get(fs, "marshal-freq"), get(fs, "elevenlabs-marshal-voice"))
	}
	fs, _ = run("--airfield", "XRMF")
	if get(fs, "tts-voice") != "echo" || get(fs, "tts-speed") != "1.3" {
		t.Fatalf("Mozdok Tower: %s %s", get(fs, "tts-voice"), get(fs, "tts-speed"))
	}
	fs, _ = run("--marshal-only", "--airfield", "LCRA", "--marshal-freq", "306.3")
	if get(fs, "marshal-freq") != "306.3" {
		t.Fatal("Caucasus settings reached Syria's Marshal")
	}
}

func TestRoleOff(t *testing.T) {
	f := rolecfg.File{Theatres: map[string]rolecfg.Theatre{"caucasus": {Stations: map[string]rolecfg.Override{"marshal": {Off: true}}}}}
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.String("marshal-freq", "306.3", "")
	res, err := f.Apply("caucasus", rolecfg.OwnerMarshal, "UGSB", pflagFlags{fs})
	if err != nil || !res.Off {
		t.Fatalf("%+v %v", res, err)
	}
	// A field the squadron unticked in Setup: its tower does not start; the
	// others do.
	f.Theatres["caucasus"] = rolecfg.Theatre{Towers: map[string]rolecfg.Override{"UGTB": {Off: true}}}
	tw := pflag.NewFlagSet("t", pflag.ContinueOnError)
	tw.String("tts-voice", "nova", "")
	if res, _ := f.Apply("caucasus", rolecfg.OwnerTower, "UGTB", pflagFlags{tw}); !res.Off {
		t.Fatal("Tbilisi Tower switched off but starts")
	}
	if res, _ := f.Apply("caucasus", rolecfg.OwnerTower, "UGSB", pflagFlags{tw}); res.Off {
		t.Fatal("Batumi Tower off too")
	}
}

// Command moved in Settings: towers handing off to "command" follow it, a
// field with its own handoff in the Frequencies table keeps that.
func TestCommandHandoffFollows(t *testing.T) {
	savedB, savedK := *airfield.UGSB, *airfield.UG5X
	oldCfg, oldTh, oldOv := roleCfg, roleTheatre, freqOverrides
	t.Cleanup(func() {
		*airfield.UGSB, *airfield.UG5X = savedB, savedK
		roleCfg, roleTheatre, freqOverrides = oldCfg, oldTh, oldOv
	})
	roleCfg = rolecfg.File{Theatres: map[string]rolecfg.Theatre{"caucasus": {Stations: map[string]rolecfg.Override{"command": {Freq: 283}}}}}
	roleTheatre = "caucasus"
	freqOverrides = airfield.FreqOverrides{Fields: map[string]airfield.FreqOverride{"UG5X": {Handoff: 290}}}
	airfield.UG5X.HandoffFreqMHz = 290
	commandHandoffFollows()
	if airfield.UGSB.HandoffFreqMHz != 283 || airfield.UG5X.HandoffFreqMHz != 290 {
		t.Fatalf("Batumi %v Kobuleti %v", airfield.UGSB.HandoffFreqMHz, airfield.UG5X.HandoffFreqMHz)
	}
}

// Settings > Roles > ATIS: Batumi taken off the air, Senaki renamed and
// revoiced, and Tbilisi (no ATIS of its own) given one on 235.000.
func TestApplyATISRoles(t *testing.T) {
	oldCfg, oldTheatre := roleCfg, roleTheatre
	tbilisi := airfield.ByICAO("UGTB")
	oldFreq := tbilisi.ATISFreqMHz
	t.Cleanup(func() { roleCfg, roleTheatre, tbilisi.ATISFreqMHz = oldCfg, oldTheatre, oldFreq })
	roleTheatre = "caucasus"
	roleCfg = rolecfg.File{Theatres: map[string]rolecfg.Theatre{"caucasus": {ATIS: map[string]rolecfg.ATIS{
		"UGSB": {Off: true},
		"UGKS": {Name: "Senaki Information", Voice: "sage"},
		"UGTB": {Added: true, Freq: 235},
		"ETAR": {Added: true, Freq: 240}, // another theatre's field: ignored
	}}}}
	got := applyATISRoles(atisStationsForMap("caucasus"))
	var names []string
	for _, st := range got {
		names = append(names, fmt.Sprintf("%s %.3f %s", st.Name, st.FreqMHz, st.Voice))
	}
	want := "Kobuleti ATIS 232.000 shimmer|Kutaisi ATIS 233.000 alloy|Senaki Information 234.000 sage|Mozdok ATIS 237.000 onyx|Tbilisi ATIS 235.000 alloy"
	if s := strings.Join(names, "|"); s != want {
		t.Fatalf("stations\n got %s\nwant %s", s, want)
	}
	if tbilisi.ATISFreqMHz != 235 || got[len(got)-1].Advisory == "" {
		t.Fatalf("Tbilisi's ATIS frequency %v, advisory %q", tbilisi.ATISFreqMHz, got[len(got)-1].Advisory)
	}
}

// TestHelperParseArgs is not a test: TestAddedStationLines runs it in a child
// process with SKYEYE_PARSE_ARGS, because parsing sets the flag globals.
func TestHelperParseArgs(t *testing.T) {
	args := os.Getenv("SKYEYE_PARSE_ARGS")
	if args == "" {
		t.Skip("helper")
	}
	cmd := newRootCommand()
	if err := cmd.ParseFlags(strings.Fields(args)); err != nil {
		fmt.Println(`{"err":` + strconv.Quote(err.Error()) + `}`)
		return
	}
	var jtacs []string
	for _, c := range jtacConfigs() {
		jtacs = append(jtacs, fmt.Sprintf("%s/%g/%s", c.Spoken, c.FreqMHz, c.Sector))
	}
	b, _ := json.Marshal(map[string]any{"added": flagAddedStation, "log": logNameOr("x"), "watch": flagHandoffWatch,
		"overlord": flagOverlordName, "callsign": flagGCICallsign, "jtacs": jtacs})
	fmt.Println(string(b))
}

// The lines the launcher writes for an added AWACS / Command / JTAC
// (cmd/launcher addedroles.go) parse, and mean what they should.
func TestAddedStationLines(t *testing.T) {
	parse := func(args string) map[string]any {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperParseArgs$")
		cmd.Env = append(os.Environ(), "SKYEYE_PARSE_ARGS="+args)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			var m map[string]any
			if json.Unmarshal([]byte(strings.TrimSpace(line)), &m) == nil {
				if m["err"] != nil {
					t.Fatalf("%s: %v", args, m["err"])
				}
				return m
			}
		}
		t.Fatalf("no output: %s", out)
		return nil
	}
	g := parse("--gci-only --map caucasus --airfield UGSB --gci-picture-interval 5m --srs-addr x:1 --gci-callsign Magic --gci-freq 265 --gci-voice sage --log-name gci-gci2 --added-station gci2")
	if g["callsign"] != "Magic" || g["added"] != "gci2" || g["log"] != "gci-gci2" {
		t.Errorf("AWACS %v", g)
	}
	c := parse("--command-only --map caucasus --command-freq 283 --command-name Second-Command --handoff-marshal-freq 0 --overlord-name= --handoff-watch=false --log-name command-command2 --added-station command2")
	if c["watch"] != false || c["overlord"] != "" || c["log"] != "command-command2" {
		t.Errorf("Command %v", c)
	}
	j := parse("--command-only --command-freq 282 --jtac-freq 61 --jtac2-freq 62 --jtac-net 63/Hitman-3-1/all/george --jtac-net 64/Hitman-4-1/Cherkessk/adam")
	if got := fmt.Sprint(j["jtacs"]); got != "[Hitman one one/61/Kuban Valley Hitman two one/62/Cherkessk Hitman three one/63/ Hitman four one/64/Cherkessk]" {
		t.Errorf("JTACs %s", got)
	}
}
