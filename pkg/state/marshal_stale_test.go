package state

import (
	"testing"
	"time"
)

func TestMarshalStackPrunesGhosts(t *testing.T) {
	s := NewMarshalStack()
	s.Enqueue("Ghost 1", 5)
	s.Enqueue("Ghost 2", 5)
	s.Enqueue("Holding 3", 5)
	old := time.Now().Add(-MarshalStackStale - time.Minute)
	s.aircraft["Ghost 1"].UpdatedAt = old
	s.aircraft["Ghost 2"].UpdatedAt = old
	s.aircraft["Ghost 2"].EAT = time.Now().Add(-10 * time.Minute)
	s.aircraft["Holding 3"].UpdatedAt = old
	s.aircraft["Holding 3"].EAT = time.Now().Add(8 * time.Minute) // still to come: keep
	pos, _ := s.Enqueue("New 4", 5)
	if pos != 2 || s.Count() != 2 {
		t.Fatalf("position %d, count %d; want 2, 2 (ghosts pruned, holder kept)", pos, s.Count())
	}
}
