package rolecfg

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Settings page, Roles in three sections - ATIS, Towers and Other - with Add,
// Remove and the names (operator 2026-10-25: "3 sections ATIS, Towers and
// other (Tanker, Command ect)", "the ability to add/remove edit the name in
// those sections"; Add picks from a list).
//
//   - ATIS: each field's broadcast can be renamed, revoiced or taken off the
//     air, and a field the theatre gives none can get one (Added, on its own
//     frequency).
//   - Tankers beyond the catalog's two: the bat's --tanker-net lines can be
//     changed or removed (Nets, by callsign), and new ones added (Tankers).

// ATIS is one field's ATIS changes.
type ATIS struct {
	Added bool    `json:"added,omitempty"` // a field with no ATIS of its own gets one
	Off   bool    `json:"off,omitempty"`
	Name  string  `json:"name,omitempty"`  // "Batumi ATIS" when unset
	Voice string  `json:"voice,omitempty"` // OpenAI voice; ATIS speaks OpenAI
	Freq  float64 `json:"freq,omitempty"`  // an Added one's frequency; the others' are in the Frequencies table
}

// TankerNet is one tanker net as --tanker-net gives it:
// "253.1/Shell-1-1/22/288/boom/charlie".
type TankerNet struct {
	Freq   float64 `json:"freq"`
	Name   string  `json:"name"` // SRS name and callsign, "Shell-1-1"
	Angels int     `json:"angels"`
	KIAS   int     `json:"kias"`
	Boom   bool    `json:"boom,omitempty"`   // a KC-135 boom, not a basket
	Eleven string  `json:"eleven,omitempty"` // an ElevenLabs voice: a --tanker-net name (charlie) or a voice ID
}

// DefaultTankerVoice is an added tanker's voice when none is picked.
const DefaultTankerVoice = "eric"

var reTankerName = regexp.MustCompile(`^[A-Za-z]+(-[0-9]+)+$`)

// Spec is the --tanker-net value.
func (t TankerNet) Spec() string {
	kind := "basket"
	if t.Boom {
		kind = "boom"
	}
	voice := t.Eleven
	if voice == "" {
		voice = DefaultTankerVoice
	}
	return fmt.Sprintf("%s/%s/%d/%d/%s/%s", fmtFreq(t.Freq), t.Name, t.Angels, t.KIAS, kind, voice)
}

// Validate checks a tanker before it is stored.
func (t TankerNet) Validate() error {
	switch {
	case t.Freq < 30 || t.Freq > 400:
		return fmt.Errorf("%.3f MHz is outside 30-400", t.Freq)
	case !reTankerName.MatchString(t.Name) || len(t.Name) > 30:
		return fmt.Errorf("callsign %q: a word and numbers with dashes, e.g. Arco-1-1", t.Name)
	case t.Angels < 1 || t.Angels > 45:
		return fmt.Errorf("altitude %d: give thousands of feet, 1-45", t.Angels)
	case t.KIAS < 150 || t.KIAS > 400:
		return fmt.Errorf("speed %d: give knots indicated, 150-400", t.KIAS)
	case strings.ContainsAny(t.Eleven, " /\t\"") || len(t.Eleven) > 40:
		return fmt.Errorf("%q is not an ElevenLabs voice", t.Eleven)
	}
	return nil
}

// ParseTankerNet reads a --tanker-net value (cmd/atc parseTankerNet runs it).
func ParseTankerNet(spec string) (TankerNet, error) {
	p := strings.Split(strings.TrimSpace(spec), "/")
	if len(p) != 6 {
		return TankerNet{}, fmt.Errorf("want FREQ/Callsign/angels/KIAS/basket|boom/voice, got %q", spec)
	}
	freq, err1 := strconv.ParseFloat(p[0], 64)
	angels, err2 := strconv.Atoi(p[2])
	kias, err3 := strconv.Atoi(p[3])
	if err1 != nil || err2 != nil || err3 != nil {
		return TankerNet{}, fmt.Errorf("bad number in %q", spec)
	}
	return TankerNet{Freq: freq, Name: p[1], Angels: angels, KIAS: kias, Boom: p[4] == "boom", Eleven: p[5]}, nil
}

// TankerNets is the --tanker-net list Command runs: the bat's, with the
// Settings changes and removals, then the added tankers.
func (f File) TankerNets(theatre string, bat []string) []string {
	th := f.Theatres[theatre]
	out := []string{}
	for _, spec := range bat {
		t, err := ParseTankerNet(spec)
		if err != nil {
			out = append(out, spec) // Command reports it
			continue
		}
		o, ok := th.Nets[t.Name]
		if !ok {
			out = append(out, spec)
			continue
		}
		if o.Off {
			continue
		}
		if o.Freq > 0 {
			t.Freq = o.Freq
		}
		if o.Name != "" {
			t.Name = o.Name
		}
		if o.Eleven != "" {
			t.Eleven = o.Eleven
		}
		out = append(out, t.Spec())
	}
	for _, t := range th.Tankers {
		out = append(out, t.Spec())
	}
	return out
}

// ValidateATIS checks a field's ATIS changes before they are stored.
func ValidateATIS(a ATIS) error {
	if a.Added && !a.Off && (a.Freq < 30 || a.Freq > 400) {
		return fmt.Errorf("give the new ATIS a frequency, 30-400 MHz")
	}
	if a.Freq != 0 && !a.Added {
		return fmt.Errorf("this ATIS's frequency is in the Frequencies table")
	}
	if strings.ContainsAny(a.Name, "\"%") || len(a.Name) > 40 {
		return fmt.Errorf("name %q has quotes or %% or is too long", a.Name)
	}
	if a.Voice != "" && !ValidOpenAIVoice(a.Voice) {
		return fmt.Errorf("%q is not an OpenAI voice", a.Voice)
	}
	return nil
}
