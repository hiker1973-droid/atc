// Package rolecfg is the launcher Settings page's Roles section (operator
// 2026-10-23: "setting your frequncys", then "Should we allow to set a voice
// type?" - yes). Each radio station SkyEye speaks as - Command, Hitman, the
// tankers, Marshal, the towers and so on - is set up today by flags on its
// atc.exe line in a start bat. The Settings page stores changes to those flags
// per theatre in roles.json next to atc.exe; atc.exe applies them at startup,
// before it reads any flag, so the bats stay the defaults and keep working.
//
// A station's frequency can also appear in other roles: Marshal 306.3 is on
// Marshal's own line and on Command's and Deckboss's as their handoff. Moving
// a station moves every reference to it (FreqRefs), so one change keeps the
// whole theatre consistent. A reference is only rewritten where it is already
// set: Red Command's --handoff-marshal-freq 0 (no Marshal) stays 0.
package rolecfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// Owners: which atc.exe process speaks a station (see OwnerOf).
const (
	OwnerCommand    = "command"     // --command-only, blue
	OwnerRedCommand = "red-command" // --command-only --coalition red
	OwnerMarshal    = "marshal"
	OwnerDeckboss   = "deckboss"
	OwnerLSO        = "lso"
	OwnerGCI        = "gci"
	OwnerATIS       = "atis"
	OwnerScudwatch  = "scudwatch"
	OwnerTower      = "tower"
)

// Station is one voice on the radio and the atc.exe flags that set it up.
// An empty flag name means that part cannot be changed.
type Station struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Owner string `json:"owner"`
	// Main is the process's own station: Off keeps the process from starting,
	// and Speed (the process-wide --tts-speed) belongs to it. A station that is
	// not Main lives inside its owner (Hitman inside Command) and Off silences
	// just it, by setting its frequency to 0 (or its name to "" when it has no
	// frequency of its own).
	Main   bool   `json:"main"`
	Freq   string `json:"freqFlag,omitempty"`
	Name   string `json:"nameFlag,omitempty"`
	Voice  string `json:"voiceFlag,omitempty"`  // OpenAI voice, the fallback when ElevenLabs is off
	Eleven string `json:"elevenFlag,omitempty"` // ElevenLabs voice ID
	Speed  string `json:"speedFlag,omitempty"`
	// NameIsCallsign: the name is spoken (Darkstar), not just an SRS client
	// name (vSFG-7-Command), so it may hold spaces.
	NameIsCallsign bool `json:"nameIsCallsign,omitempty"`
	// FreqRefs / NameRefs are flags in any process that point at this station.
	FreqRefs []string `json:"-"`
	NameRefs []string `json:"-"`
	Note     string   `json:"note,omitempty"`
}

// Stations is the catalog, in the order the page shows them.
var Stations = []Station{
	{Key: "command", Label: "Command", Owner: OwnerCommand, Main: true,
		Freq: "command-freq", Name: "command-name", Voice: "command-voice", Eleven: "elevenlabs-command-voice", Speed: "tts-speed",
		FreqRefs: []string{"handoff-command-freq"}, NameRefs: []string{"handoff-command-name"},
		Note: "Towers whose handoff is Command follow its frequency."},
	{Key: "overlord", Label: "Overlord Actual", Owner: OwnerCommand,
		Name: "overlord-name", Voice: "overlord-voice", Eleven: "elevenlabs-overlord-voice",
		Note: "Speaks on Command's frequency: permission to engage, range calls."},
	{Key: "hitman1", Label: "Hitman 1-1 (JTAC)", Owner: OwnerCommand,
		Freq: "jtac-freq", Name: "jtac-name", Voice: "jtac-voice", Eleven: "elevenlabs-jtac-voice"},
	{Key: "hitman2", Label: "Hitman 2-1 (JTAC)", Owner: OwnerCommand,
		Freq: "jtac2-freq", Name: "jtac2-name", Voice: "jtac2-voice", Eleven: "elevenlabs-jtac2-voice"},
	{Key: "tanker1", Label: "Tanker 1", Owner: OwnerCommand,
		Freq: "tanker-freq", Name: "tanker-name", Voice: "tanker-voice", Eleven: "elevenlabs-tanker-voice"},
	{Key: "tanker2", Label: "Tanker 2", Owner: OwnerCommand,
		Freq: "tanker2-freq", Name: "tanker2-name", Voice: "tanker2-voice", Eleven: "elevenlabs-tanker2-voice"},
	{Key: "gci", Label: "GCI (AWACS)", Owner: OwnerGCI, Main: true, NameIsCallsign: true,
		Freq: "gci-freq", Name: "gci-callsign", Voice: "gci-voice", Eleven: "elevenlabs-gci-voice", Speed: "tts-speed",
		FreqRefs: []string{"command-gci-freq"}, NameRefs: []string{"command-gci-name"}},
	{Key: "marshal", Label: "Marshal", Owner: OwnerMarshal, Main: true,
		Freq: "marshal-freq", Voice: "marshal-voice", Eleven: "elevenlabs-marshal-voice",
		FreqRefs: []string{"handoff-marshal-freq"}},
	{Key: "deckboss", Label: "Deckboss", Owner: OwnerDeckboss, Main: true,
		Freq: "deckboss-freq", Voice: "deckboss-voice", Eleven: "elevenlabs-deckboss-voice",
		FreqRefs: []string{"lso-deckboss-freq"}},
	{Key: "paddles", Label: "Paddles (LSO)", Owner: OwnerLSO, Main: true,
		Freq: "lso-freq", Name: "lso-name", Voice: "lso-voice", Eleven: "elevenlabs-paddles-voice", Speed: "tts-speed"},
	{Key: "red-command", Label: "Red Command", Owner: OwnerRedCommand, Main: true,
		Freq: "command-freq", Name: "command-name", Voice: "command-voice", Eleven: "elevenlabs-command-voice", Speed: "tts-speed",
		Note: "Red towers hand departures to it."},
}

// Tower flags: a tower's frequency is the Frequencies table's, so a tower
// station has voices and speed only.
const (
	TowerVoiceFlag  = "tts-voice"
	TowerElevenFlag = "elevenlabs-tower-voice"
	TowerSpeedFlag  = "tts-speed"
)

// Defaults are atc.exe's own flag defaults, for a station whose bat line does
// not name the flag. cmd/atc's TestRoleCatalogDefaults keeps them in step.
var Defaults = map[string]string{
	"command-freq": "0", "command-name": "vSFG-7-Command", "command-voice": "onyx", "elevenlabs-command-voice": "FF59babHL8N8gfTgtBMT",
	"overlord-name": "Overlord-Actual", "overlord-voice": "ash", "elevenlabs-overlord-voice": "d7KZ4gJXiEWKJV0PMsGG",
	"jtac-freq": "0", "jtac-name": "Hitman-1-1", "jtac-voice": "ash", "elevenlabs-jtac-voice": "7QoDxbTwBuOlgOCahTNI",
	"jtac2-freq": "0", "jtac2-name": "Hitman-2-1", "jtac2-voice": "echo", "elevenlabs-jtac2-voice": "9JbYHzFp74No6NSJoI0w",
	"tanker-freq": "0", "tanker-name": "Texaco-1-1", "tanker-voice": "echo", "elevenlabs-tanker-voice": "cjVigY5qzO86Huf0OWal",
	"tanker2-freq": "0", "tanker2-name": "Texaco-3-1", "tanker2-voice": "verse", "elevenlabs-tanker2-voice": "nPczCjzI2devNBz1zQrb",
	"gci-freq": "264", "gci-callsign": "Darkstar", "gci-voice": "shimmer", "elevenlabs-gci-voice": "vChnJZ1Cu89g2XXumPfT",
	"marshal-freq": "0", "marshal-voice": "ballad", "elevenlabs-marshal-voice": "6PBHdlpKVQ70t6tnDUYF",
	"deckboss-freq": "0", "deckboss-voice": "shimmer", "elevenlabs-deckboss-voice": "ljX1ZrXuDIIRVcmiVSyR",
	"lso-freq": "128.3", "lso-name": "Paddles", "lso-voice": "ash", "elevenlabs-paddles-voice": "KvaCHcJoMXfOzwtsULOZ",
	"tts-voice": "nova", "elevenlabs-tower-voice": "", "tts-speed": "1.2",
}

// OpenAIVoices are the gpt-4o-mini-tts voices, with a word on each for the
// picker.
var OpenAIVoices = []struct{ ID, Desc string }{
	{"alloy", "neutral, even"}, {"ash", "male, clear"}, {"ballad", "male, soft"},
	{"coral", "female, warm"}, {"echo", "male, steady"}, {"fable", "male, British-leaning"},
	{"nova", "female, bright"}, {"onyx", "male, deep"}, {"sage", "female, calm"},
	{"shimmer", "female, light"}, {"verse", "male, expressive"},
}

// ValidOpenAIVoice reports whether v is one of OpenAIVoices.
func ValidOpenAIVoice(v string) bool {
	for _, o := range OpenAIVoices {
		if o.ID == v {
			return true
		}
	}
	return false
}

// ByKey finds a catalog station.
func ByKey(key string) (Station, bool) {
	for _, s := range Stations {
		if s.Key == key {
			return s, true
		}
	}
	return Station{}, false
}

// Override is one station's changes; zero values keep the bat's.
type Override struct {
	Freq   float64 `json:"freq,omitempty"`
	Name   string  `json:"name,omitempty"`
	Voice  string  `json:"voice,omitempty"`
	Eleven string  `json:"eleven,omitempty"`
	Speed  float64 `json:"speed,omitempty"`
	Off    bool    `json:"off,omitempty"`
}

// Theatre is one theatre's changes: catalog stations by key, towers by ICAO.
type Theatre struct {
	Stations map[string]Override `json:"stations,omitempty"`
	Towers   map[string]Override `json:"towers,omitempty"`
}

// File is roles.json.
type File struct {
	Theatres map[string]Theatre `json:"theatres"`
}

// Read reads roles.json; a missing file is no changes.
func Read(path string) (File, error) {
	f := File{Theatres: map[string]Theatre{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return File{Theatres: map[string]Theatre{}}, err
	}
	if f.Theatres == nil {
		f.Theatres = map[string]Theatre{}
	}
	return f, nil
}

// Station returns a theatre's override for a catalog station.
func (f File) Station(theatre, key string) (Override, bool) {
	o, ok := f.Theatres[theatre].Stations[key]
	return o, ok
}

// Validate checks an override before it is stored.
func Validate(s *Station, o Override) error {
	if o.Freq != 0 && (o.Freq < 30 || o.Freq > 400) {
		return fmt.Errorf("%.3f MHz is outside 30-400", o.Freq)
	}
	if o.Speed != 0 && (o.Speed < 0.7 || o.Speed > 2.0) {
		return fmt.Errorf("speed %.2f is outside 0.70-2.00", o.Speed)
	}
	if o.Voice != "" && !ValidOpenAIVoice(o.Voice) {
		return fmt.Errorf("%q is not an OpenAI voice", o.Voice)
	}
	if strings.ContainsAny(o.Eleven, " \t\"") || len(o.Eleven) > 40 {
		return fmt.Errorf("%q is not an ElevenLabs voice ID", o.Eleven)
	}
	if o.Name != "" {
		if strings.ContainsAny(o.Name, "\"%") || len(o.Name) > 40 {
			return fmt.Errorf("name %q has quotes or %% or is too long", o.Name)
		}
		if (s == nil || !s.NameIsCallsign) && strings.Contains(o.Name, " ") {
			return fmt.Errorf("an SRS name cannot hold spaces: use dashes, e.g. Hitman-1-1")
		}
	}
	if s != nil {
		if o.Freq != 0 && s.Freq == "" {
			return fmt.Errorf("%s has no frequency of its own", s.Label)
		}
		if o.Name != "" && s.Name == "" {
			return fmt.Errorf("%s has no name to change", s.Label)
		}
		if o.Speed != 0 && s.Speed == "" {
			return fmt.Errorf("%s has a fixed speed", s.Label)
		}
	}
	return nil
}

// truthy reads a bool flag as a bat or a FlagSet gives it: present with no
// value, or anything but "false".
func truthy(flags map[string]string, name string) bool {
	v, ok := flags[name]
	return ok && v != "false"
}

func nonzero(v string) bool {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return err == nil && f != 0
}

// OwnerOf names the process an atc.exe command line starts, from the flags it
// was given (names without "--", values as typed). Mirrors the mode switch in
// cmd/atc/main.go.
func OwnerOf(flags map[string]string) string {
	switch {
	case truthy(flags, "command-only") && strings.EqualFold(flags["coalition"], "red"):
		return OwnerRedCommand
	case truthy(flags, "command-only"):
		return OwnerCommand
	case truthy(flags, "atis-only"):
		return OwnerATIS
	case truthy(flags, "scudwatch-only"):
		return OwnerScudwatch
	case truthy(flags, "marshal-only"):
		return OwnerMarshal
	case truthy(flags, "lso-only"):
		return OwnerLSO
	case truthy(flags, "gci-only"):
		return OwnerGCI
	case nonzero(flags["deckboss-freq"]):
		return OwnerDeckboss
	}
	return OwnerTower
}

// Flags is what Apply needs of a flag set: read a flag's current value
// (ok=false if this program has no such flag) and set one.
type Flags interface {
	Get(name string) (string, bool)
	Set(name, value string) error
}

// Result is what Apply did.
type Result struct {
	Changed []string // "flag=value", for the log
	Off     bool     // this process's Main station is turned off
}

func fmtFreq(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// Apply puts a theatre's changes onto one process's flags. owner is OwnerOf
// its flags and icao its --airfield (for a tower).
func (f File) Apply(theatre, owner, icao string, fl Flags) (Result, error) {
	var res Result
	set := func(name, value string) error {
		if cur, ok := fl.Get(name); !ok || cur == value {
			return nil
		}
		if err := fl.Set(name, value); err != nil {
			return fmt.Errorf("%s=%s: %w", name, value, err)
		}
		res.Changed = append(res.Changed, name+"="+value)
		return nil
	}
	th := f.Theatres[theatre]
	for _, s := range Stations {
		o, ok := th.Stations[s.Key]
		if !ok {
			continue
		}
		if s.Owner == owner {
			if o.Off {
				if s.Main {
					res.Off = true
					return res, nil
				}
				var err error
				if s.Freq != "" {
					err = set(s.Freq, "0")
				} else if s.Name != "" {
					err = set(s.Name, "")
				}
				if err != nil {
					return res, err
				}
				continue
			}
			for _, e := range []struct {
				flag, val string
				when      bool
			}{
				{s.Freq, fmtFreq(o.Freq), o.Freq > 0 && s.Freq != "" && isSetFreq(fl, s.Freq)},
				{s.Name, o.Name, o.Name != "" && s.Name != "" && isSetName(fl, s.Name)},
				{s.Voice, o.Voice, o.Voice != "" && s.Voice != ""},
				{s.Eleven, o.Eleven, o.Eleven != "" && s.Eleven != ""},
				{s.Speed, fmtFreq(o.Speed), o.Speed > 0 && s.Speed != ""},
			} {
				if e.when {
					if err := set(e.flag, e.val); err != nil {
						return res, err
					}
				}
			}
		}
		if o.Off {
			continue
		}
		// References from other roles, only where they are already set.
		if o.Freq > 0 {
			for _, ref := range s.FreqRefs {
				if isSetFreq(fl, ref) {
					if err := set(ref, fmtFreq(o.Freq)); err != nil {
						return res, err
					}
				}
			}
		}
		if o.Name != "" {
			for _, ref := range s.NameRefs {
				if isSetName(fl, ref) {
					if err := set(ref, o.Name); err != nil {
						return res, err
					}
				}
			}
		}
	}
	if owner == OwnerTower {
		if o, ok := th.Towers[strings.ToUpper(icao)]; ok {
			if o.Off { // a field the squadron does not use (Setup page)
				res.Off = true
				return res, nil
			}
			if o.Voice != "" {
				if err := set(TowerVoiceFlag, o.Voice); err != nil {
					return res, err
				}
			}
			if o.Eleven != "" {
				if err := set(TowerElevenFlag, o.Eleven); err != nil {
					return res, err
				}
			}
			if o.Speed > 0 {
				if err := set(TowerSpeedFlag, fmtFreq(o.Speed)); err != nil {
					return res, err
				}
			}
		}
	}
	return res, nil
}

// isSetFreq: the flag exists here and is not 0 (a station this process runs,
// or a reference it makes). A station switched off in the bat stays off.
func isSetFreq(fl Flags, name string) bool {
	v, ok := fl.Get(name)
	return ok && nonzero(v)
}

func isSetName(fl Flags, name string) bool {
	v, ok := fl.Get(name)
	return ok && strings.TrimSpace(v) != ""
}
