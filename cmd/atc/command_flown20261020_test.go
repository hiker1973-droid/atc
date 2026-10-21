package main

import (
	"strings"
	"testing"

	"github.com/paulmach/orb"

	"github.com/vsfg7/atc/pkg/airfield"
)

// Day 2026-10-20, Venom out of Senaki: check-ins Command missed or answered
// with the wrong callsign.
func TestCommandCheckInsFlown20261020(t *testing.T) {
	for _, c := range []struct{ text, cs, want string }{
		{"Command, Venom 201, flight of three F-16s, check it in.", "Venom 201", "Venom 201 flight, "},
		{"Command, Venom 020 on channel.", "Venom 020", "Venom 020, "},
		{"Command, Venom 2-0-0 checking in.", "Venom 200", "Venom 200, "},
	} {
		got := commandResponse(c.text, c.cs, "vSFG-7-Command")
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("%q: reply %q, want it to start %q", c.text, got, c.want)
		}
	}
	// Intra-flight chatter on the net stays unanswered.
	if got := commandResponse("Venom 3, Venom 1.", "Venom 3", "vSFG-7-Command"); got != "" {
		t.Errorf("intra-flight call answered: %q", got)
	}

	store := newTacviewPositions()
	store.Set("Venom 203", orb.Point{42, 42})
	store.Set("Venom 200", orb.Point{42, 42})
	cs := joinCallsignDigits(extractCallsignSkippingAddress("Command and 203 checking in.", commandAddresses...))
	if got := commandCallsign(cs, store); got != "Venom 203" {
		t.Errorf("%q -> %q, want Venom 203", cs, got)
	}
	if got := commandCallsign("and 207", store); got != "207" {
		t.Errorf("unknown number: %q, want the number alone", got)
	}
	if got := commandCallsign("Venom 200", store); got != "Venom 200" {
		t.Errorf("real callsign changed: %q", got)
	}
}

// Venom 201 launched from Senaki and was handed to Kutaisi tower on the way
// to work. A pilot first seen near his field recovers there.
func TestCommandRecoveryHomeField(t *testing.T) {
	fields := airfield.FieldsForMap("caucasus")
	var senaki, kutaisi *airfield.Airfield
	for _, f := range fields {
		switch f.ICAO {
		case "UGKS":
			senaki = f
		case "UGKO":
			kutaisi = f
		}
	}
	if senaki == nil || kutaisi == nil {
		t.Fatal("Senaki or Kutaisi missing from the Caucasus fields")
	}
	// Checked in 7 nm east of Senaki, after the tower handoff.
	if h := homeField(offsetNm(senaki.Center, 0, 7), fields); h != senaki {
		t.Fatalf("home = %v, want Senaki", h)
	}
	// Later, 25 nm east of Kutaisi, ~55 from Senaki: still Senaki.
	pos := offsetNm(kutaisi.Center, 0, 25)
	rec := chooseRecovery(pos, fields, senaki, orb.Point{}, false, false)
	if rec.name != senaki.Name+" tower" || rec.distNm <= handoffThresholdNm {
		t.Errorf("recovery %q at %.1f nm, want Senaki tower outside %v nm", rec.name, rec.distNm, handoffThresholdNm)
	}
	// First seen far from every field: nearest field, as before.
	if h := homeField(offsetNm(senaki.Center, 60, 0), fields); h != nil {
		t.Errorf("home = %s 60 nm out, want none", h.Name)
	}
}
