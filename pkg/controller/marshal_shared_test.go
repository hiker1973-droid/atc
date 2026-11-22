package controller

import (
	"testing"

	"github.com/vsfg7/atc/pkg/airfield"
)

func TestAssignMarshalAngelsShared(t *testing.T) {
	c := NewATCController("Marshal", &airfield.Airfield{})
	// One per altitude (Case 3): angels 2 taken, so 3.
	if got := c.AssignMarshalAngelsShared(2, 9, []int{2}, 1); got != 3 {
		t.Errorf("cap 1: got %d, want 3", got)
	}
	// Case I: three jets already at 2, a fourth shares it.
	if got := c.AssignMarshalAngelsShared(2, 9, []int{2, 2, 2}, 4); got != 2 {
		t.Errorf("cap 4, three at 2: got %d, want 2", got)
	}
	// ...and a fifth goes to 3.
	if got := c.AssignMarshalAngelsShared(2, 9, []int{2, 2, 2, 2}, 4); got != 3 {
		t.Errorf("cap 4, four at 2: got %d, want 3", got)
	}
	if got := c.AssignMarshalAngels(2, 9, []int{2}); got != 3 {
		t.Errorf("AssignMarshalAngels: got %d, want 3", got)
	}
}
