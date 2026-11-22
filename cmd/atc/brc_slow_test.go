package main

import "testing"

func TestSplitBRC(t *testing.T) {
	pre, brc, post, ok := splitBRC("Raider 331, Union Marshal, signal Charlie, BRC zero... nine... six, altimeter two nine nine two.")
	if !ok || pre != "Raider 331, Union Marshal, signal Charlie," || brc != "BRC zero... nine... six" || post != ", altimeter two nine nine two." {
		t.Fatalf("got %q | %q | %q (%v)", pre, brc, post, ok)
	}
	if _, _, _, ok := splitBRC("Raider 331, copy checkin."); ok {
		t.Error("split a line with no BRC")
	}
}
