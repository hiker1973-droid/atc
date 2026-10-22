package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/paulmach/orb"

	"github.com/vsfg7/atc/pkg/airfield"
	"github.com/vsfg7/atc/pkg/composer"
)

func withCoalition(t *testing.T, side, fields string) {
	t.Helper()
	oldSide, oldFields, oldMap := flagCoalition, flagCommandFields, flagMap
	flagCoalition, flagCommandFields, flagMap = side, fields, "caucasus"
	t.Cleanup(func() { flagCoalition, flagCommandFields, flagMap = oldSide, oldFields, oldMap })
}

func srsClientCoalition(t *testing.T, msg []byte) float64 {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(msg, &m); err != nil {
		t.Fatal(err)
	}
	return m["Client"].(map[string]interface{})["Coalition"].(float64)
}

// Red Command and Mozdok Tower register and transmit as red; everything else
// stays blue.
func TestSRSCoalition(t *testing.T) {
	withCoalition(t, "blue", "")
	if got := srsClientCoalition(t, buildSync("g", "vSFG-7-Command", 282e6)); got != 2 || srsCoalitionArg() != "2" {
		t.Fatalf("blue registered as %v / %s", got, srsCoalitionArg())
	}
	withCoalition(t, "red", "XRMF")
	if got := srsClientCoalition(t, buildSync("g", "Red-Command", 280e6)); got != 1 {
		t.Fatalf("red sync registered as %v", got)
	}
	if got := srsClientCoalition(t, buildEAM("g", "Red-Command", 280e6, "pw")); got != 1 || srsCoalitionArg() != "1" {
		t.Fatalf("red EAM registered as %v / %s", got, srsCoalitionArg())
	}
}

// Red Command recovers to Mozdok only, never to a blue field.
func TestRedCommandRecoversToMozdok(t *testing.T) {
	withCoalition(t, "red", "XRMF")
	fields := commandFields()
	if len(fields) != 1 || fields[0].ICAO != "XRMF" {
		t.Fatalf("red fields %v", fields)
	}
	// 40 nm west of Mozdok, nearer Nalchik than any blue field but Mozdok is all there is.
	pos := orb.Point{43.75, 43.80}
	rec := chooseRecovery(pos, fields, nil, orb.Point{}, false, false)
	if rec.name != "Mozdok tower" || rec.freqMHz != 266.0 {
		t.Fatalf("recovery %+v", rec)
	}
	if len(divertFields()) != 1 {
		t.Fatal("red divert should offer Mozdok only")
	}
}

// Blue Command is unchanged: the five Caucasus fields, Mozdok not among them.
func TestBlueCommandNeverSendsToMozdok(t *testing.T) {
	withCoalition(t, "blue", "")
	for _, f := range commandFields() {
		if f.ICAO == "XRMF" {
			t.Fatal("Mozdok is in blue's field list")
		}
	}
	if len(commandFields()) != len(airfield.Caucasus) {
		t.Fatalf("blue fields %d", len(commandFields()))
	}
}

func TestRedCommandAddress(t *testing.T) {
	for _, in := range []string{"Red Command, Venom 3-1, checking in.", "Red-Command, Venom 3-1, checking in."} {
		cs := joinCallsignDigits(extractCallsignSkippingAddress(in, commandAddresses...))
		if !strings.EqualFold(cs, "Venom 3-1") && !strings.EqualFold(cs, "Venom 31") {
			t.Errorf("%q -> callsign %q", in, cs)
		}
	}
	withCoalition(t, "red", "XRMF")
	resp := commandResponse("Red Command, Venom 3-1, checking in.", "Venom 3-1", "Red-Command")
	if !strings.Contains(resp, "Red-Command") {
		t.Fatalf("check-in reply %q", resp)
	}
}

// Mozdok's departure call names Red Command and its frequency, with no
// dangling preset.
func TestMozdokHandoffToRedCommand(t *testing.T) {
	af := airfield.ByICAO("XRMF")
	if af == nil || af.TowerFreqMHz != 266.0 || af.HandoffFreqMHz != 280.0 {
		t.Fatalf("XRMF %+v", af)
	}
	c := composer.NewATCComposer("Mozdok Tower")
	for i := 0; i < 20; i++ {
		s := c.HandoffToCommand("Venom 3-1", af.HandoffCallsign, af.HandoffFreqMHz, af.HandoffPreset)
		if !strings.Contains(s, "Red Command") || strings.Contains(s, ", .") || strings.Contains(s, ", ,") {
			t.Fatalf("handoff %q", s)
		}
		d := c.DepartureLeavingAirspace("Venom 3-1", 7, 4, af.HandoffCallsign, af.HandoffFreqMHz, af.HandoffPreset, "")
		if !strings.Contains(d, "Red Command") || strings.Contains(d, ", .") {
			t.Fatalf("departure %q", d)
		}
	}
}
