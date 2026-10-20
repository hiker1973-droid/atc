package main

import (
	"reflect"
	"testing"
	"time"
)

func TestMissionStatusTheatre(t *testing.T) {
	cau := []Role{{Name: "Marshal (Caucasus)", Region: "caucasus"}, {Name: "Batumi Tower", Region: "caucasus"}}
	syr := []Role{{Name: "Marshal (Syria)", Region: "syria"}}

	m := missionStatus("vSFG-7_Training_Mission_CA V1.32.VFR 2 Towering Cumulus (Winter).miz", "Caucasus", cau)
	if m.Region != "caucasus" || m.RegionGuessed || m.Mismatch != nil {
		t.Fatalf("caucasus on caucasus: %+v", m)
	}
	m = missionStatus("Foothold_SY_5.2.0_vSFG7_v1.12_04_VFR_Khamsin_Dust_Afternoon.miz", "Syria", cau)
	if !reflect.DeepEqual(m.Mismatch, []string{"caucasus"}) {
		t.Fatalf("syria mission, caucasus roles: want mismatch, got %+v", m)
	}
	// Older rig launchers send no theatre: fall back to the file name.
	m = missionStatus("Foothold_SY_5.2.0_vSFG7_v1.12_04_VFR_Khamsin_Dust_Afternoon.miz", "", syr)
	if m.Region != "syria" || !m.RegionGuessed || m.Mismatch != nil {
		t.Fatalf("guessed syria: %+v", m)
	}
	m = missionStatus("vSFG-7_Night_Training_Mission_CA V1.32.VFR 1 Clear Night (Fall).miz", "", syr)
	if !reflect.DeepEqual(m.Mismatch, []string{"syria"}) {
		t.Fatalf("guessed caucasus vs syria roles: %+v", m)
	}
	// Unknown map or no roles running: never a mismatch.
	if m := missionStatus("Some Mission.miz", "", cau); m.Region != "" || m.Mismatch != nil {
		t.Fatalf("unknown: %+v", m)
	}
	if m := missionStatus("x.miz", "Syria", nil); m.Mismatch != nil {
		t.Fatalf("no roles: %+v", m)
	}
}

func TestSummariseAir(t *testing.T) {
	p := AirPicture{Connected: true, At: time.Now(), OnGround: 4, Contacts: []Contact{
		{Callsign: "Shell 1-1", Aircraft: "KC-135", Color: "Blue", AltFt: 22000, Airborne: true},
		{Callsign: "Arco 1-1", Aircraft: "A6E", Color: "Blue", AltFt: 15000, Airborne: true},
		{Callsign: "Bandit", Aircraft: "MiG-29S", Color: "Red", AltFt: 20000, Airborne: true},
		{Callsign: "Taxiing", Aircraft: "FA-18C_hornet", Color: "Blue", Airborne: false},
	}}
	a := summariseAir(p)
	if a.Airborne != 3 || a.Blue != 2 || a.Red != 1 || a.OnGround != 4 {
		t.Fatalf("counts: %+v", a)
	}
	if a.Contacts[0].Callsign != "Arco 1-1" || a.Contacts[2].Color != "Red" {
		t.Fatalf("order: %+v", a.Contacts)
	}
}
