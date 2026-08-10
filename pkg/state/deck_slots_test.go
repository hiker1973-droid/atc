package state

import (
	"testing"
	"time"
)

// heldBy counts how many cats a callsign currently occupies. A callsign should
// never hold more than one.
func heldBy(ds *DeckbossState, callsign string) int {
	n := 0
	for _, cat := range ds.Cats {
		if cat.Callsign == callsign {
			n++
		}
	}
	return n
}

// The reported symptom: Deckboss answering "all cats engaged" with nothing on
// the deck. One pilot repeating their check-in — because they didn't hear the
// reply, or STT delivered the same TX twice — used to burn a slot per repeat,
// so four calls from one aircraft filled a four-cat deck.
func TestAssignCat_RepeatCheckInDoesNotFillDeck(t *testing.T) {
	ds := NewDeckbossState()
	first := ds.AssignCat("Raider 311")
	for i := 0; i < 5; i++ {
		if got := ds.AssignCat("Raider 311"); got != first {
			t.Fatalf("repeat check-in %d returned cat %d, want %d", i+1, got, first)
		}
	}
	if held := heldBy(ds, "Raider 311"); held != 1 {
		t.Errorf("one aircraft holds %d cats after repeat check-ins, want 1", held)
	}
	if ds.AllCatsBusy() {
		t.Error("deck reads full with a single aircraft on it")
	}
	if got := ds.AssignCat("Raider 312"); got == 0 {
		t.Error("second aircraft got no cat on a deck with three free")
	}
}

// FreeCat cleared only the first matching slot, which is what made the old
// double-assign permanent. It now releases every slot the callsign holds.
func TestFreeCat_ReleasesEverySlotHeld(t *testing.T) {
	ds := NewDeckbossState()
	// Force the corrupt shape the old AssignCat could produce.
	ds.Cats[0].Status, ds.Cats[0].Callsign = CatTaxying, "Raider 311"
	ds.Cats[2].Status, ds.Cats[2].Callsign = CatTaxying, "Raider 311"

	if got := ds.FreeCat("Raider 311"); got != 1 {
		t.Errorf("FreeCat returned %d, want the lowest freed cat (1)", got)
	}
	if held := heldBy(ds, "Raider 311"); held != 0 {
		t.Errorf("callsign still holds %d cats after release, want 0", held)
	}
	for _, cat := range ds.Cats {
		if cat.Status != CatFree {
			t.Errorf("cat %d left in status %v after full release", cat.Number, cat.Status)
		}
	}
}

func TestFreeCat_UnknownCallsignIsNoOp(t *testing.T) {
	ds := NewDeckbossState()
	ds.AssignCat("Raider 311")
	if got := ds.FreeCat("Raider 999"); got != 0 {
		t.Errorf("freeing a callsign that holds nothing returned cat %d, want 0", got)
	}
	if heldBy(ds, "Raider 311") != 1 {
		t.Error("releasing an unknown callsign disturbed another aircraft's slot")
	}
}

// Next-up must be told the cat they were actually assigned. On a partially
// spotted deck the freed cat and the assigned cat differ, and naming the freed
// one sent the pilot to a cat somebody else was sitting on.
func TestReleaseAndPullNext_ReportsTheAssignedCatNotTheFreedOne(t *testing.T) {
	ds := NewDeckbossState()
	// Cat 1 free, cat 3 about to be released.
	ds.Cats[1].Status, ds.Cats[1].Callsign = CatTaxying, "Raider 312"
	ds.Cats[2].Status, ds.Cats[2].Callsign = CatTension, "Raider 313"
	ds.Cats[3].Status, ds.Cats[3].Callsign = CatTaxying, "Raider 314"
	ds.EnqueueConga("Raider 315")

	freed, next, nextCat := ds.ReleaseAndPullNext("Raider 313")
	if freed != 3 {
		t.Errorf("freed cat %d, want 3", freed)
	}
	if next != "Raider 315" {
		t.Errorf("pulled %q from the conga, want Raider 315", next)
	}
	if nextCat != 1 {
		t.Errorf("next-up told cat %d, want the cat actually assigned (1)", nextCat)
	}
	if got := ds.GetCatByCallsign("Raider 315"); got != nextCat {
		t.Errorf("next-up sits on cat %d but was told cat %d", got, nextCat)
	}
	if ds.CongaLen() != 0 {
		t.Errorf("conga still holds %d after the pull, want 0", ds.CongaLen())
	}
}

// §2a and §4 can both fire for one launch. The second is a no-op.
func TestReleaseAndPullNext_DoubleReleaseIsHarmless(t *testing.T) {
	ds := NewDeckbossState()
	ds.AssignCat("Raider 311")
	ds.EnqueueConga("Raider 312")

	if freed, next, _ := ds.ReleaseAndPullNext("Raider 311"); freed == 0 || next != "Raider 312" {
		t.Fatalf("first release freed %d and pulled %q", freed, next)
	}
	freed, next, _ := ds.ReleaseAndPullNext("Raider 311")
	if freed != 0 || next != "" {
		t.Errorf("second release freed %d and pulled %q, want 0 and empty", freed, next)
	}
	if heldBy(ds, "Raider 312") != 1 {
		t.Error("the double release disturbed the aircraft pulled onto the cat")
	}
}

func TestReleaseAndPullNext_EmptyCongaLeavesCatFree(t *testing.T) {
	ds := NewDeckbossState()
	ds.AssignCat("Raider 311")
	freed, next, nextCat := ds.ReleaseAndPullNext("Raider 311")
	if freed != 1 || next != "" || nextCat != 0 {
		t.Errorf("got freed=%d next=%q nextCat=%d, want 1, empty, 0", freed, next, nextCat)
	}
	if ds.AllCatsBusy() {
		t.Error("deck reads busy after the only aircraft launched")
	}
}

// §1c: a pilot re-checking in from the line hears their position, not another
// "join the conga". The result code is what tells those apart.
func TestEnqueueConga_RepeatCheckInReportsPosition(t *testing.T) {
	ds := NewDeckbossState()
	ds.EnqueueConga("Raider 311")
	ds.EnqueueConga("Raider 312")

	pos, res := ds.EnqueueConga("Raider 312")
	if res != CongaAlready {
		t.Errorf("repeat enqueue returned %v, want CongaAlready", res)
	}
	if pos != 2 {
		t.Errorf("repeat enqueue reported position %d, want 2", pos)
	}
	if ds.CongaLen() != 2 {
		t.Errorf("repeat enqueue grew the line to %d, want 2", ds.CongaLen())
	}
}

// §1d: past capacity the caller is told to hold clear of the bow rather than
// joining a line Deckboss will never work through.
func TestEnqueueConga_FullLineRefuses(t *testing.T) {
	ds := NewDeckbossState()
	for i := 0; i < CongaCapacity; i++ {
		if _, res := ds.EnqueueConga(string(rune('a' + i))); res != CongaQueued {
			t.Fatalf("enqueue %d returned %v, want CongaQueued", i, res)
		}
	}
	pos, res := ds.EnqueueConga("Raider 999")
	if res != CongaFull {
		t.Errorf("enqueue past capacity returned %v, want CongaFull", res)
	}
	if pos != 0 {
		t.Errorf("refused enqueue reported position %d, want 0", pos)
	}
	if ds.CongaLen() != CongaCapacity {
		t.Errorf("line grew past capacity to %d", ds.CongaLen())
	}
}

// A pilot who leaves while queued keeps their place and gets handed a cat they
// will never taxi to — one more way the deck fills with nothing on it. The
// monitor evicts them; these are the primitives it uses.
func TestCongaWaitingSince_OnlyReturnsAgedEntries(t *testing.T) {
	ds := NewDeckbossState()
	ds.EnqueueConga("Raider 311")
	ds.EnqueueConga("Raider 312")
	// Age only the first entry.
	ds.congaJoinedAt["Raider 311"] = time.Now().Add(-10 * time.Minute)

	got := ds.CongaWaitingSince(5 * time.Minute)
	if len(got) != 1 || got[0] != "Raider 311" {
		t.Errorf("aged candidates = %v, want [Raider 311]", got)
	}
	if len(ds.CongaWaitingSince(30 * time.Minute)) != 0 {
		t.Error("entries returned for a window nothing is old enough for")
	}
}

func TestRemoveFromConga_EvictsAndPreservesOrder(t *testing.T) {
	ds := NewDeckbossState()
	ds.EnqueueConga("Raider 311")
	ds.EnqueueConga("Raider 312")
	ds.EnqueueConga("Raider 313")

	if !ds.RemoveFromConga("Raider 312") {
		t.Fatal("removing a queued callsign reported it absent")
	}
	if ds.RemoveFromConga("Raider 312") {
		t.Error("removing the same callsign twice reported it present")
	}
	line := ds.GetCongaLine()
	if len(line) != 2 || line[0] != "Raider 311" || line[1] != "Raider 313" {
		t.Errorf("line after eviction = %v, want [Raider 311 Raider 313]", line)
	}
	// The evicted pilot must be able to re-join at the back.
	if pos, res := ds.EnqueueConga("Raider 312"); res != CongaQueued || pos != 3 {
		t.Errorf("re-join returned pos %d, %v; want 3, CongaQueued", pos, res)
	}
}
