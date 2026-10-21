package main

import (
	"testing"
	"time"
)

func TestMergePilots(t *testing.T) {
	now := time.Date(2026, 10, 21, 20, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return now.Add(time.Duration(h) * time.Hour) }
	dabs := PilotSession{Server: "Foothold", UCID: "u1", Name: "Venom 212 | Dabs", Aircraft: "F-16C_50",
		HopOn: at(-5), HopOff: at(-3), Takeoffs: 1, Landings: 1, Kills: 2, KillsAir: 1, KillsGnd: 1}
	// Day and Night share one DCSServerBot database: both return the same row.
	day := PilotsRaw{Rig: "Day", Configured: true, Sessions: []PilotSession{dabs,
		{Server: "Day", UCID: "u2", Name: "Raider 302 | Jedi", Aircraft: "FA-18C_hornet", HopOn: at(-1), Takeoffs: 1}},
		Traps: []PilotTrap{{Server: "Day", UCID: "u2", Name: "Raider 302 | Jedi", Grade: "OK", Points: 4, Wire: 3, Time: at(-1)},
			{Server: "Day", UCID: "u2", Name: "Raider 302 | Jedi", Grade: "(OK)", Points: 3, Wire: 2, Time: at(-2)}}}
	night := PilotsRaw{Rig: "Night", Configured: true, Sessions: []PilotSession{dabs}}
	foothold := PilotsRaw{Rig: "Foothold", Err: "SKYEYE_DCSSB_DB is not set on this rig"}
	rep := mergePilots([]PilotsRaw{day, night, foothold}, now)

	if len(rep.Pilots) != 2 || len(rep.Rigs) != 3 || rep.Rigs[2].Err == "" {
		t.Fatalf("pilots %d rigs %+v", len(rep.Pilots), rep.Rigs)
	}
	var d, j PilotStats
	for _, p := range rep.Pilots {
		if p.UCID == "u1" {
			d = p
		} else {
			j = p
		}
	}
	if d.Hours != 2 || d.Sessions != 1 || d.Kills != 2 || d.Callsign != "Venom 212" || d.Online {
		t.Errorf("Dabs counted twice or wrong: %+v", d)
	}
	if !j.Online || j.Hours != 1 || j.Traps != 2 || j.TrapPoints != 3.5 {
		t.Errorf("Jedi (still in a slot, two traps): %+v", j)
	}
	if rep.Pilots[0].UCID != "u1" {
		t.Error("not sorted by hours")
	}
	// A slot never closed (server crashed) a day ago counts no time.
	stale := mergePilots([]PilotsRaw{{Sessions: []PilotSession{{UCID: "u3", HopOn: at(-30)}}}}, now)
	if p := stale.Pilots[0]; p.Hours != 0 || p.Online {
		t.Errorf("stale open slot: %+v", p)
	}
}

func TestRedactURL(t *testing.T) {
	url := "postgres://skyeye_read:s3cret@127.0.0.1:5432/dcsserverbot"
	if got := redactURL("failed to connect to user=skyeye_read password=s3cret", url); got != "failed to connect to user=skyeye_read password=***" {
		t.Errorf("redact: %q", got)
	}
}
