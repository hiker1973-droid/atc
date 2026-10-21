package sets

import (
	"slices"
	"testing"
)

func TestOfAndContains(t *testing.T) {
	t.Parallel()
	s := Of(1, 2, 3)
	for _, e := range []int{1, 2, 3} {
		if !Contains(s, e) {
			t.Errorf("expected set to contain %d", e)
		}
	}
	if Contains(s, 4) {
		t.Errorf("expected set to not contain 4")
	}
	if Len(s) != 3 {
		t.Errorf("expected len 3, got %d", Len(s))
	}
}

func TestAddRemove(t *testing.T) {
	t.Parallel()
	s := New[string]()
	Add(s, "a")
	Add(s, "a")
	if Len(s) != 1 {
		t.Errorf("expected len 1 after duplicate add, got %d", Len(s))
	}
	Remove(s, "a")
	if Contains(s, "a") {
		t.Errorf("expected 'a' to be removed")
	}
	Remove(s, "missing") // no-op
}

func TestClone(t *testing.T) {
	t.Parallel()
	a := Of(1, 2, 3)
	b := Clone(a)
	Add(b, 4)
	if Contains(a, 4) {
		t.Errorf("clone should not affect original")
	}
	if !Contains(b, 4) {
		t.Errorf("clone should have new element")
	}
}

func TestUnion(t *testing.T) {
	t.Parallel()
	a := Of(1, 2, 3)
	b := Of(3, 4, 5)
	got := Union(a, b)
	want := Of(1, 2, 3, 4, 5)
	if !Equal(got, want) {
		t.Errorf("Union = %v, want %v", got, want)
	}
}

func TestIntersection(t *testing.T) {
	t.Parallel()
	a := Of(1, 2, 3, 4)
	b := Of(3, 4, 5)
	got := Intersection(a, b)
	want := Of(3, 4)
	if !Equal(got, want) {
		t.Errorf("Intersection = %v, want %v", got, want)
	}

	// Disjoint sets produce empty intersection.
	got = Intersection(Of(1, 2), Of(3, 4))
	if Len(got) != 0 {
		t.Errorf("expected empty intersection, got %v", got)
	}
}

func TestDifference(t *testing.T) {
	t.Parallel()
	a := Of(1, 2, 3)
	b := Of(2, 3, 4)
	got := Difference(a, b)
	want := Of(1)
	if !Equal(got, want) {
		t.Errorf("Difference = %v, want %v", got, want)
	}
}

func TestSymmetricDifference(t *testing.T) {
	t.Parallel()
	a := Of(1, 2, 3)
	b := Of(2, 3, 4)
	got := SymmetricDifference(a, b)
	want := Of(1, 4)
	if !Equal(got, want) {
		t.Errorf("SymmetricDifference = %v, want %v", got, want)
	}
}

func TestIsSubset(t *testing.T) {
	t.Parallel()
	if !IsSubset(Of(1, 2), Of(1, 2, 3)) {
		t.Errorf("expected subset")
	}
	if IsSubset(Of(1, 2, 3), Of(1, 2)) {
		t.Errorf("expected not subset (larger)")
	}
	if IsSubset(Of(1, 4), Of(1, 2, 3)) {
		t.Errorf("expected not subset (disjoint element)")
	}
	if !IsSubset(New[int](), Of(1, 2)) {
		t.Errorf("empty set should be subset of any set")
	}
}

func TestIsSuperset(t *testing.T) {
	t.Parallel()
	if !IsSuperset(Of(1, 2, 3), Of(1, 2)) {
		t.Errorf("expected superset")
	}
	if IsSuperset(Of(1, 2), Of(1, 2, 3)) {
		t.Errorf("expected not superset (smaller)")
	}
	if !IsSuperset(Of(1, 2), Of(1, 2)) {
		t.Errorf("equal sets should be supersets of each other")
	}
	if !IsSuperset(Of(1, 2), New[int]()) {
		t.Errorf("any set should be superset of empty set")
	}
}

func TestIsDisjoint(t *testing.T) {
	t.Parallel()
	if !IsDisjoint(Of(1, 2), Of(3, 4)) {
		t.Errorf("expected disjoint")
	}
	if IsDisjoint(Of(1, 2), Of(2, 3)) {
		t.Errorf("expected not disjoint")
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	if !Equal(Of(1, 2, 3), Of(3, 2, 1)) {
		t.Errorf("expected equal")
	}
	if Equal(Of(1, 2), Of(1, 2, 3)) {
		t.Errorf("expected not equal (different sizes)")
	}
	if Equal(Of(1, 2), Of(1, 3)) {
		t.Errorf("expected not equal (different elements)")
	}
}

func TestAllIterator(t *testing.T) {
	t.Parallel()
	s := Of(1, 2, 3)
	var got []int
	for e := range All(s) {
		got = append(got, e)
	}
	slices.Sort(got)
	want := []int{1, 2, 3}
	if !slices.Equal(got, want) {
		t.Errorf("All() yielded %v, want %v", got, want)
	}
}

func TestAllEarlyTermination(t *testing.T) {
	t.Parallel()
	s := Of(1, 2, 3, 4, 5)
	count := 0
	for range All(s) {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("expected iteration to stop at 2, got %d", count)
	}
}

func TestCollect(t *testing.T) {
	t.Parallel()
	a := Of(1, 2, 3)
	b := Collect(All(a))
	if !Equal(a, b) {
		t.Errorf("Collect round-trip failed: %v vs %v", a, b)
	}
}
