package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/airfield"
)

// The same words with another letter are the same recording; a new runway or
// altimeter is not. A kept recording comes back with the letter it says.
func TestATISLibrary(t *testing.T) {
	st := &atisStation{Name: "Senaki ATIS", ICAO: "UGKS", FreqMHz: 251.0} // paired: the runway is read
	a := &atisState{altimeter: 29.92, windFrom: 270, windKts: 8, activeRwy: "27", ident: 3}
	b := &atisState{altimeter: 29.92, windFrom: 270, windKts: 8, activeRwy: "27", ident: 9}
	k := atisLibraryKey(st, a, "Russian", true)
	if k != atisLibraryKey(st, b, "Russian", true) {
		t.Error("the information letter changed the key")
	}
	c := &atisState{altimeter: 29.92, windFrom: 270, windKts: 8, activeRwy: "09"}
	d := &atisState{altimeter: 29.91, windFrom: 270, windKts: 8, activeRwy: "27"}
	if k == atisLibraryKey(st, c, "Russian", true) || k == atisLibraryKey(st, d, "Russian", true) {
		t.Error("a new runway or altimeter kept the key")
	}
	if k == atisLibraryKey(st, a, "Russian", false) {
		t.Error("English only kept the bilingual key")
	}

	t.Cleanup(func() {
		files, _ := os.ReadDir(atisLibraryDir())
		for _, f := range files {
			if len(f.Name()) > 7 && f.Name()[:7] == "ZZTEST_" {
				os.Remove(atisLibraryDir() + string(os.PathSeparator) + f.Name())
			}
		}
	})
	if _, _, ok := atisLibraryGet("ZZTEST", k); ok {
		t.Fatal("found a recording never kept")
	}
	audio := []byte("ID3 fake mp3")
	atisLibraryPut("ZZTEST", k, 3, audio)
	got, ident, ok := atisLibraryGet("ZZTEST", k)
	if !ok || ident != 3 || !bytes.Equal(got, audio) {
		t.Fatalf("got %v %d %q", ok, ident, got)
	}
}

// One light-wind runway slot a day, changing at 10:00 UTC (04:00 Mountain).
func TestCalmSlotDaily(t *testing.T) {
	before := time.Date(2026, 10, 27, 9, 59, 0, 0, time.UTC)
	after := before.Add(2 * time.Minute)
	evening := time.Date(2026, 10, 28, 3, 0, 0, 0, time.UTC) // 21:00 MDT the same flying day
	if airfield.CalmSlot(after) != airfield.CalmSlot(before)+1 {
		t.Error("slot did not roll at 10:00 UTC")
	}
	if airfield.CalmSlot(evening) != airfield.CalmSlot(after) {
		t.Error("slot rolled during the evening")
	}
}
