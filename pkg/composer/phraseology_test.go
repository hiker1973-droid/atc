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
