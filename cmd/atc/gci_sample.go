package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/martinlindhe/unit"
	"github.com/rs/zerolog/log"

	"github.com/vsfg7/atc/pkg/gci/bearings"
	"github.com/vsfg7/atc/pkg/gci/brevity"
	"github.com/vsfg7/atc/pkg/gci/composer"
)

// renderGCISample speaks a handful of typical GCI calls - composed by
// SkyEye's own composer from made-up contacts - in the GCI voice with the
// radio chain, one file each plus all of them joined, so the voice can be
// heard before the role goes on a rig (operator 2026-10-21: "make me a gci
// sample as well"). atc.exe --gci-sample <dir> [--gci-callsign Overlord].
func renderGCISample(ctx context.Context, apiKey, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	comp := composer.Composer{Callsign: flagGCICallsign}
	mag := func(deg float64) bearings.Bearing { return bearings.NewMagneticBearing(unit.Angle(deg) * unit.Degree) }
	nm := func(n float64) unit.Length { return unit.Length(n) * unit.NauticalMile }
	ft := func(n float64) unit.Length { return unit.Length(n) * unit.Foot }

	flankers := &sampleGroup{contacts: 2, alt: ft(25000), track: brevity.Southwest, aspect: brevity.Hot,
		decl: brevity.Hostile, platforms: []string{"Flanker"},
		braa:  brevity.NewBRAA(mag(45), nm(32), []unit.Length{ft(25000)}, brevity.Hot),
		bulls: brevity.NewBullseye(mag(10), nm(48))}
	fulcrums := &sampleGroup{contacts: 3, alt: ft(18000), track: brevity.West, aspect: brevity.Flank,
		decl: brevity.Hostile, platforms: []string{"Fulcrum"}, bulls: brevity.NewBullseye(mag(80), nm(62))}
	bandit := &sampleGroup{contacts: 2, alt: ft(22000), track: brevity.Southwest, aspect: brevity.Hot,
		decl: brevity.Hostile, platforms: []string{"Flanker"},
		braa: brevity.NewBRAA(mag(40), nm(18), []unit.Length{ft(22000)}, brevity.Hot)}

	calls := []struct {
		name string
		call any
	}{
		{"radio check", brevity.RadioCheckResponse{Callsign: "Venom 2 1 2", RadarContact: true}},
		{"check in", brevity.CheckInResponse{Callsign: "Raider 1 1"}},
		{"bogey dope", brevity.BogeyDopeResponse{Callsign: "Venom 2 1 2", Group: flankers}},
		{"picture", brevity.PictureResponse{Count: 2, Groups: []brevity.Group{flankers, fulcrums}}},
		{"declare", brevity.DeclareResponse{Callsign: "Raider 1 1", Readback: brevity.NewBullseye(mag(80), nm(62)),
			Declaration: brevity.Hostile, Group: fulcrums}},
		{"threat", brevity.ThreatCall{Callsigns: []string{"Venom 2 1 2"}, Group: bandit}},
	}

	vp := gciVoice()
	var parts, script []string
	for i, c := range calls {
		if flagGCISampleOnly != "" && c.name != flagGCISampleOnly {
			continue
		}
		resp, ok := gciCompose(comp, c.call)
		if !ok || resp.Speech == "" {
			return fmt.Errorf("sample %q: composer had nothing to say", c.name)
		}
		mp3, err := synthesizeSpeech(ctx, apiKey, resp.Speech, vp)
		if err != nil {
			return fmt.Errorf("sample %q: %w", c.name, err)
		}
		raw := filepath.Join(dir, fmt.Sprintf("raw_%02d.mp3", i+1))
		if err := os.WriteFile(raw, mp3, 0o644); err != nil {
			return err
		}
		txPath, cleanup := prepareTX(raw, flagGCICallsign)
		out := filepath.Join(dir, fmt.Sprintf("%02d %s %s.mp3", i+1, flagGCICallsign, c.name))
		b, err := os.ReadFile(txPath)
		cleanup()
		os.Remove(raw)
		if err != nil {
			return err
		}
		if err := os.WriteFile(out, b, 0o644); err != nil {
			return err
		}
		parts = append(parts, out)
		script = append(script, fmt.Sprintf("%02d %-12s %s", i+1, c.name, resp.Subtitle))
		log.Info().Str("call", c.name).Str("text", resp.Subtitle).Msg("GCI sample rendered")
	}

	if len(parts) == 0 {
		return fmt.Errorf("no sample call named %q", flagGCISampleOnly)
	}
	if len(parts) == 1 {
		fmt.Println(script[0])
		log.Info().Str("file", parts[0]).Msg("GCI sample written")
		return nil
	}
	// All of them in one file, a second apart.
	all := filepath.Join(dir, flagGCICallsign+" GCI sample.mp3")
	args := []string{"-y", "-loglevel", "error"}
	var filter strings.Builder
	for i, p := range parts {
		args = append(args, "-i", p)
		fmt.Fprintf(&filter, "[%d:a]aresample=44100,apad=pad_dur=1[a%d];", i, i)
	}
	for i := range parts {
		fmt.Fprintf(&filter, "[a%d]", i)
	}
	fmt.Fprintf(&filter, "concat=n=%d:v=0:a=1[out]", len(parts))
	args = append(args, "-filter_complex", filter.String(), "-map", "[out]", "-b:a", "128k", all)
	if out, err := exec.Command(flagFFmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("joining the sample: %w: %s", err, out)
	}
	text := strings.Join(script, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, flagGCICallsign+" GCI sample.txt"), []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Print(text)
	log.Info().Str("file", all).Msg("GCI sample written")
	return nil
}

// sampleGroup is a made-up contact group for the sample.
type sampleGroup struct {
	contacts  int
	alt       unit.Length
	track     brevity.Track
	aspect    brevity.Aspect
	decl      brevity.Declaration
	platforms []string
	braa      *brevity.BRAA
	bulls     *brevity.Bullseye
	threat    bool
	merged    int
}

func (g *sampleGroup) Threat() bool                { return g.threat }
func (g *sampleGroup) SetThreat(t bool)            { g.threat = t }
func (g *sampleGroup) Contacts() int               { return g.contacts }
func (g *sampleGroup) Bullseye() *brevity.Bullseye { return g.bulls }
func (g *sampleGroup) Altitude() unit.Length       { return g.alt }
func (g *sampleGroup) Stacks() []brevity.Stack {
	return []brevity.Stack{{Altitude: g.alt, Count: g.contacts}}
}
func (g *sampleGroup) Track() brevity.Track                 { return g.track }
func (g *sampleGroup) Aspect() brevity.Aspect               { return g.aspect }
func (g *sampleGroup) BRAA() *brevity.BRAA                  { return g.braa }
func (g *sampleGroup) Declaration() brevity.Declaration     { return g.decl }
func (g *sampleGroup) SetDeclaration(d brevity.Declaration) { g.decl = d }
func (g *sampleGroup) Heavy() bool                          { return g.contacts >= 3 }
func (g *sampleGroup) Platforms() []string                  { return g.platforms }
func (g *sampleGroup) High() bool                           { return g.alt.Feet() > 40000 }
func (g *sampleGroup) Fast() bool                           { return false }
func (g *sampleGroup) VeryFast() bool                       { return false }
func (g *sampleGroup) MergedWith() int                      { return g.merged }
func (g *sampleGroup) SetMergedWith(n int)                  { g.merged = n }
func (g *sampleGroup) String() string                       { return fmt.Sprintf("%d %s", g.contacts, g.platforms) }
func (g *sampleGroup) ObjectIDs() []uint64                  { return nil }
