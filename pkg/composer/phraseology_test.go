package composer

import (
	"strings"
	"testing"
)

// Everything this file asserts is fed straight to TTS, so the tests are about
// what a pilot actually hears, not about internal formatting.

func TestFormatWindSpellsDigits(t *testing.T) {
	cases := []struct {
		name     string
		dir, kts float64
		want     string
	}{
		{"below three knots is calm", 270, 2, "calm"},
		{"direction and speed spelled", 270, 15, "two seven zero at one five"},
		{"single-digit speed has no leading zero", 70, 8, "zero seven zero at eight"},
		{"north is three six zero, not zero zero zero", 360, 10, "three six zero at one zero"},
		{"zero degrees normalises to three six zero", 0, 10, "three six zero at one zero"},
		{"direction rounds to nearest ten", 274, 12, "two seven zero at one two"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatWind(tc.dir, tc.kts); got != tc.want {
				t.Errorf("formatWind(%v, %v) = %q, want %q", tc.dir, tc.kts, got, tc.want)
			}
		})
	}
}

func TestFormatWindEmitsNoNumerals(t *testing.T) {
	// A numeral anywhere in the wind string means TTS will voice it as a
	// cardinal number ("two hundred seventy") instead of separate digits.
	got := formatWind(270, 15)
	if strings.ContainsAny(got, "0123456789") {
		t.Errorf("formatWind emitted numerals: %q", got)
	}
	if strings.Contains(got, "knots") {
		t.Errorf("FAA phraseology omits the unit; got %q", got)
	}
}

func TestFormatAltimeterHasNoDecimalPoint(t *testing.T) {
	cases := []struct {
		inHg float64
		want string
	}{
		{29.92, "two niner niner two"},
		{30.01, "three zero zero one"},
		{29.00, "two niner zero zero"},
		{30.15, "three zero one five"},
	}
	for _, tc := range cases {
		if got := formatAltimeter(tc.inHg); got != tc.want {
			t.Errorf("formatAltimeter(%v) = %q, want %q", tc.inHg, got, tc.want)
		}
	}
	if strings.Contains(formatAltimeter(29.92), "point") {
		t.Error(`altimeter settings are voiced as four digits with no "point"`)
	}
}

func TestSpellDigits3StillWrapsHeadings(t *testing.T) {
	// spellDigits3 is used for headings/radials, where 360 canonicalises to 000.
	// The wind path deliberately does not go through it.
	if got := spellDigits3(360); got != "zero zero zero" {
		t.Errorf("spellDigits3(360) = %q, want %q", got, "zero zero zero")
	}
	if got := spellDigits3(87); got != "zero eight seven" {
		t.Errorf("spellDigits3(87) = %q, want %q", got, "zero eight seven")
	}
}

func TestLandingClearanceCarriesRequiredElements(t *testing.T) {
	c := NewATCComposer("Al Minhad Tower")
	// 7110.65 3-10-5: RUNWAY (number), WIND (direction and velocity),
	// CLEARED TO LAND — and the clearance is the last element.
	for i := 0; i < 40; i++ {
		got := c.ClearedToLand("Raider 032", "27", 270, 15, false)
		if !strings.Contains(got, "runway two seven") {
			t.Fatalf("landing clearance missing runway: %q", got)
		}
		if !strings.Contains(got, "two seven zero at one five") {
			t.Fatalf("landing clearance missing wind: %q", got)
		}
		if !strings.HasSuffix(got, "cleared to land.") {
			t.Fatalf("clearance must be the last element: %q", got)
		}
	}
}

func TestWheelsCheckAppearsOnlyWhenOwed(t *testing.T) {
	c := NewATCComposer("Al Minhad Tower")
	for i := 0; i < 40; i++ {
		with := c.ClearedToLand("Raider 032", "27", 270, 15, true)
		if !strings.Contains(with, "check wheels down") {
			t.Fatalf("wheels check requested but absent: %q", with)
		}
		if !strings.HasSuffix(with, "cleared to land.") {
			t.Fatalf("wheels check must not displace the clearance: %q", with)
		}
		without := c.ClearedToLand("Raider 032", "27", 270, 15, false)
		if strings.Contains(without, "wheels") {
			t.Fatalf("wheels check suppressed but present: %q", without)
		}

		base := c.BaseAck("Raider 032", "27", 2, true)
		if !strings.Contains(base, "check wheels down") {
			t.Fatalf("base ack missing wheels check: %q", base)
		}
		if strings.Contains(c.BaseAck("Raider 032", "27", 2, false), "wheels") {
			t.Fatal("base ack issued a suppressed wheels check")
		}
	}
}

func TestInboundAckCarriesPatternElements(t *testing.T) {
	c := NewATCComposer("Al Minhad Tower")
	// 7110.65 3-10-12 requires pattern altitude and direction of traffic on
	// the arrival call, then a report at initial.
	for i := 0; i < 40; i++ {
		got := c.InboundAck("Raider 032", "27", 270, 15, 29.92, 1500, "left", 0)
		for _, want := range []string{
			"runway two seven",
			"two seven zero at one five",
			"two niner niner two",
			"pattern altitude one thousand five hundred",
			"left turns",
			"Report initial.",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("inbound ack missing %q: %q", want, got)
			}
		}
	}
}

func TestPatternClauseDropsUnknownElements(t *testing.T) {
	cases := []struct {
		name string
		ft   int
		dir  string
		want string
	}{
		{"both known", 1500, "left", ", pattern altitude one thousand five hundred, left turns"},
		{"no direction", 2000, "", ", pattern altitude two thousand"},
		{"bad direction dropped", 2000, "sideways", ", pattern altitude two thousand"},
		{"no altitude", 0, "right", ", right turns"},
		{"neither", 0, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := patternClause(tc.ft, tc.dir); got != tc.want {
				t.Errorf("patternClause(%d, %q) = %q, want %q", tc.ft, tc.dir, got, tc.want)
			}
		})
	}
}

func TestSpellAltitudeFtEmitsNoNumerals(t *testing.T) {
	cases := []struct {
		ft   int
		want string
	}{
		{1500, "one thousand five hundred"},
		{2000, "two thousand"},
		{6000, "six thousand"},
		{2500, "two thousand five hundred"},
		{10000, "ten thousand"},
		{15000, "one five thousand"},
		{23400, "two three thousand four hundred"},
		{0, "unknown"},
	}
	for _, tc := range cases {
		got := spellAltitudeFt(tc.ft)
		if got != tc.want {
			t.Errorf("spellAltitudeFt(%d) = %q, want %q", tc.ft, got, tc.want)
		}
		if strings.ContainsAny(got, "0123456789") {
			t.Errorf("spellAltitudeFt(%d) emitted a numeral: %q", tc.ft, got)
		}
	}
}

func TestHandoffNamesFacilityBeforeFrequency(t *testing.T) {
	c := NewATCComposer("Al Minhad Tower")
	// 7110.65 2-1-17: CONTACT (facility), (frequency).
	for i := 0; i < 40; i++ {
		got := c.Handoff("Raider 032", "vSFG-7-Command", 282.0, "channel four", "Good day.")
		facility := strings.Index(got, "vSFG-7-Command")
		freq := strings.Index(got, "two eight two")
		if facility < 0 || freq < 0 {
			t.Fatalf("handoff missing facility or frequency: %q", got)
		}
		if facility > freq {
			t.Fatalf("facility must precede frequency: %q", got)
		}
		if !strings.Contains(got, "channel four") {
			t.Fatalf("handoff missing preset: %q", got)
		}
	}
	// A destination with no assigned preset or frequency still reads cleanly.
	bare := c.Handoff("Raider 032", "Paddles", 0, "", "")
	if strings.Contains(bare, ", .") || strings.Contains(bare, "  ") {
		t.Errorf("bare handoff has dangling punctuation: %q", bare)
	}
}

// The Deckboss → Marshal → Command departure chain. Both phrases below are
// spoken on every launch, so they are worth pinning: the frequency has to be
// spelled digit-by-digit (a numeral reaches TTS as "three hundred six"), and
// the handoff has to degrade to the pre-2026-08-16 bare ack when disabled.
func TestDeckbossCopyAirborneCarriesMarshalHandoff(t *testing.T) {
	c := NewATCComposer("Deckboss")
	for i := 0; i < 40; i++ {
		got := c.DeckbossCopyAirborne("Raider 032", "Marshal", 306.3)
		if !strings.Contains(got, "copy airborne") {
			t.Fatalf("ack lost the airborne readback: %q", got)
		}
		if !strings.Contains(got, "three zero six point three") {
			t.Fatalf("frequency not spelled as digits: %q", got)
		}
		marshal := strings.Index(got, "Marshal")
		freq := strings.Index(got, "three zero six")
		if marshal < 0 || freq < 0 || marshal > freq {
			t.Fatalf("facility must precede frequency: %q", got)
		}
	}
	// Handoff disabled (--handoff-marshal-freq=0) restores the bare ack.
	bare := c.DeckbossCopyAirborne("Raider 032", "Marshal", 0)
	if bare != "Raider 032, Deckboss, copy airborne." {
		t.Errorf("disabled handoff should give the bare ack, got %q", bare)
	}
}

func TestMarshalDepartureClearReleasesZoneAndPushesCommand(t *testing.T) {
	c := NewATCComposer("Union Marshal")
	for i := 0; i < 40; i++ {
		got := c.MarshalDepartureClear("Raider 032", "Union", "vSFG-7-Command", 282.0, "channel four")
		if !strings.Contains(got, "clear of Union control zone") {
			t.Fatalf("missing zone release: %q", got)
		}
		if !strings.Contains(got, "for tasking") {
			t.Fatalf("missing tasking push: %q", got)
		}
		if !strings.Contains(got, "two eight two point zero") {
			t.Fatalf("frequency not spelled as digits: %q", got)
		}
		zone := strings.Index(got, "control zone")
		dest := strings.Index(got, "vSFG-7-Command")
		if zone > dest {
			t.Fatalf("zone release must precede the push: %q", got)
		}
	}
	// The short form a rig gets with --handoff-command-freq=0 and no preset.
	bare := c.MarshalDepartureClear("Raider 032", "Union", "command", 0, "")
	if !strings.Contains(bare, "push command, for tasking.") &&
		!strings.Contains(bare, "switch command, for tasking.") &&
		!strings.Contains(bare, "contact command, for tasking") {
		t.Errorf("bare form reads wrong: %q", bare)
	}
	if strings.Contains(bare, ", ,") || strings.Contains(bare, "  ") {
		t.Errorf("bare form has dangling punctuation: %q", bare)
	}
}

func TestDepartureChainEmitsNoNumerals(t *testing.T) {
	d := NewATCComposer("Deckboss")
	m := NewATCComposer("Union Marshal")
	// Callsigns legitimately contain digits, so check only the text after the
	// callsign — that is the part the composer itself generates.
	for i := 0; i < 40; i++ {
		for _, got := range []string{
			d.DeckbossCopyAirborne("Raider", "Marshal", 306.3),
			m.MarshalDepartureClear("Raider", "Union", "Command", 282.0, "channel four"),
		} {
			if strings.ContainsAny(got, "0123456789") {
				t.Errorf("numeral would be voiced as a cardinal number: %q", got)
			}
		}
	}
}
