package main

import "testing"

func TestCASTICMissionNameTraining(t *testing.T) {
	yes := []string{
		"vSFG-7_CAS-LGB_TIC_Training V1.8.miz",
		"vSFG-7_Training_Mission_CA V1.42 BASE FILE.miz",
		"vSFG-7_Training_Mission_CA V1.42.VFR 3.miz",
		"vSFG-7_Night_Training_Mission_CA V1.43 BASE FILE.miz",
	}
	no := []string{
		"vSFG-7_Training_Mission_CA V1.41 BASE FILE.miz",
		"vSFG-7_Night_Training_Mission_CA V1.41.VFR 1.miz",
		"Foothold_Syria.miz",
	}
	for _, n := range yes {
		if !casTICMissionName(n) {
			t.Errorf("%s should be CAS TIC", n)
		}
	}
	for _, n := range no {
		if casTICMissionName(n) {
			t.Errorf("%s should not be CAS TIC", n)
		}
	}
}
