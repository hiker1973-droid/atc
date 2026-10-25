package rolecfg

import (
	"fmt"
	"regexp"
	"strings"
)

// Stations added on the Settings page beyond tankers (operator 2026-10-25:
// "in addition to tanker add JTAC, COMMAND or other"; JTAC, a second AWACS
// and a second Command chosen):
//
//   - a JTAC is one more net inside the theatre's Command (--jtac-net);
//   - a second AWACS or Command is one more atc.exe, which the launcher
//     starts from start_added_<theatre>.bat, written from Added.

// JTACNet is one added JTAC: "63/Hitman-3-1/all/george".
type JTACNet struct {
	Freq   float64 `json:"freq"`
	Name   string  `json:"name"`             // SRS name and callsign, "Hitman-3-1"
	Sector string  `json:"sector,omitempty"` // a range sector; "" = the whole range
	Eleven string  `json:"eleven,omitempty"` // a voice name (george) or an ElevenLabs voice ID
}

// DefaultJTACVoice is an added JTAC's voice when none is picked.
const DefaultJTACVoice = "george"

var reSector = regexp.MustCompile(`^[A-Za-z0-9 '-]{1,30}$`)

// Spec is the --jtac-net value.
func (j JTACNet) Spec() string {
	sector, voice := j.Sector, j.Eleven
	if sector == "" {
		sector = "all"
	}
	if voice == "" {
		voice = DefaultJTACVoice
	}
	return fmt.Sprintf("%s/%s/%s/%s", fmtFreq(j.Freq), j.Name, sector, voice)
}

// Validate checks a JTAC before it is stored.
func (j JTACNet) Validate() error {
	switch {
	case j.Freq < 30 || j.Freq > 400:
		return fmt.Errorf("%.3f MHz is outside 30-400", j.Freq)
	case !reTankerName.MatchString(j.Name) || len(j.Name) > 30:
		return fmt.Errorf("callsign %q: a word and numbers with dashes, e.g. Hitman-3-1", j.Name)
	case j.Sector != "" && !reSector.MatchString(j.Sector):
		return fmt.Errorf("sector %q: letters, digits, spaces and dashes", j.Sector)
	case strings.ContainsAny(j.Eleven, " /\t\"") || len(j.Eleven) > 40:
		return fmt.Errorf("%q is not an ElevenLabs voice", j.Eleven)
	}
	return nil
}

// JTACNets is the --jtac-net list the theatre's Command adds.
func (f File) JTACNets(theatre string) []string {
	var out []string
	for _, j := range f.Theatres[theatre].JTACs {
		out = append(out, j.Spec())
	}
	return out
}

// Kinds of added station that run as their own atc.exe.
const (
	AddedGCI     = "gci"
	AddedCommand = "command"
)

// AddedStation is a second AWACS or Command.
type AddedStation struct {
	ID     string  `json:"id"`   // "gci2", "command2": its window title, log and instance
	Kind   string  `json:"kind"` // AddedGCI | AddedCommand
	Freq   float64 `json:"freq"`
	Name   string  `json:"name"`             // GCI: the spoken callsign ("Magic"); Command: the SRS name
	Voice  string  `json:"voice,omitempty"`  // OpenAI voice
	Eleven string  `json:"eleven,omitempty"` // ElevenLabs voice ID
}

// One word: it goes on the start line unquoted.
var reCallsign = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,29}$`)

// Validate checks an added station before it is stored.
func (a AddedStation) Validate() error {
	switch {
	case a.Kind != AddedGCI && a.Kind != AddedCommand:
		return fmt.Errorf("kind %q: want gci or command", a.Kind)
	case a.Freq < 30 || a.Freq > 400:
		return fmt.Errorf("%.3f MHz is outside 30-400", a.Freq)
	case a.Kind == AddedGCI && !reCallsign.MatchString(a.Name):
		return fmt.Errorf("callsign %q: a word, e.g. Magic", a.Name)
	case a.Kind == AddedCommand && !reCallsign.MatchString(a.Name):
		return fmt.Errorf("SRS name %q: no spaces, e.g. Second-Command", a.Name)
	case a.Voice != "" && !ValidOpenAIVoice(a.Voice):
		return fmt.Errorf("%q is not an OpenAI voice", a.Voice)
	case strings.ContainsAny(a.Eleven, " /\t\"") || len(a.Eleven) > 40:
		return fmt.Errorf("%q is not an ElevenLabs voice ID", a.Eleven)
	}
	return nil
}
