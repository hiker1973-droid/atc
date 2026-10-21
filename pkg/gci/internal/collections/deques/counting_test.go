package deques

import (
	"slices"
	"testing"
)

func TestCountingPushPop(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](3)
	d.Push(1)
	d.Push(2)
	d.Push(3)
	if d.Len() != 3 {
		t.Errorf("Len() = %d, want 3", d.Len())
	}
	for _, want := range []int{1, 2, 3} {
		got, ok := d.Pop()
		if !ok || got != want {
			t.Errorf("Pop() = %d, %v; want %d, true", got, ok, want)
		}
	}
	if d.Len() != 0 {
		t.Errorf("Len() after drain = %d, want 0", d.Len())
	}
}

func TestCountingEvictsOldest(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](2)
	d.Push(1)
	d.Push(2)
	d.Push(3)
	if d.Len() != 2 {
		t.Errorf("Len() = %d, want 2", d.Len())
	}
	if v, ok := d.Oldest(); !ok || v != 2 {
		t.Errorf("Oldest() = %d, %v; want 2, true", v, ok)
	}
	if v, ok := d.Newest(); !ok || v != 3 {
		t.Errorf("Newest() = %d, %v; want 3, true", v, ok)
	}
}

func TestCountingEmpty(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](3)
	if v, ok := d.Pop(); ok {
		t.Errorf("Pop() on empty = %d, %v; want zero, false", v, ok)
	}
	if v, ok := d.Oldest(); ok {
		t.Errorf("Oldest() on empty = %d, %v; want zero, false", v, ok)
	}
	if v, ok := d.Newest(); ok {
		t.Errorf("Newest() on empty = %d, %v; want zero, false", v, ok)
	}
	if v, ok := d.At(0); ok {
		t.Errorf("At(0) on empty = %d, %v; want zero, false", v, ok)
	}
	if d.Len() != 0 {
		t.Errorf("Len() on empty = %d, want 0", d.Len())
	}
}

func TestCountingAt(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](5)
	for _, v := range []int{10, 20, 30} {
		d.Push(v)
	}
	for i, want := range []int{30, 20, 10} {
		got, ok := d.At(i)
		if !ok || got != want {
			t.Errorf("At(%d) = %d, %v; want %d, true", i, got, ok, want)
		}
	}
	if _, ok := d.At(-1); ok {
		t.Errorf("At(-1) should return false")
	}
	if _, ok := d.At(3); ok {
		t.Errorf("At(out of range) should return false")
	}
}

func TestCountingClear(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](3)
	d.Push(1)
	d.Push(2)
	d.Clear()
	if d.Len() != 0 {
		t.Errorf("Len() after Clear = %d, want 0", d.Len())
	}
	d.Push(3)
	if v, ok := d.Oldest(); !ok || v != 3 {
		t.Errorf("Oldest() after Clear+Push = %d, %v; want 3, true", v, ok)
	}
}

func TestCountingSetCap(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](5)
	for _, v := range []int{1, 2, 3, 4, 5} {
		d.Push(v)
	}
	d.SetCap(3)
	if d.Cap() != 3 {
		t.Errorf("Cap() = %d, want 3", d.Cap())
	}
	if d.Len() != 3 {
		t.Errorf("Len() after SetCap shrink = %d, want 3", d.Len())
	}
	if v, ok := d.Oldest(); !ok || v != 3 {
		t.Errorf("Oldest() after shrink = %d; want 3", v)
	}
	d.SetCap(10)
	if d.Cap() != 10 {
		t.Errorf("Cap() after grow = %d, want 10", d.Cap())
	}
	if d.Len() != 3 {
		t.Errorf("Len() after grow = %d, want 3", d.Len())
	}
}

func TestCountingZeroCap(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](0)
	d.Push(1)
	if d.Len() != 0 {
		t.Errorf("Len() with cap=0 = %d, want 0", d.Len())
	}
	d2 := NewCounting[int](-5)
	if d2.Cap() != 0 {
		t.Errorf("Cap() with negative = %d, want 0", d2.Cap())
	}
}

func TestCountingAll(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](3)
	for _, v := range []int{1, 2, 3} {
		d.Push(v)
	}
	got := slices.Collect(d.All())
	want := []int{3, 2, 1}
	if !slices.Equal(got, want) {
		t.Errorf("All() = %v, want %v", got, want)
	}
}

func TestCountingAllEarlyTermination(t *testing.T) {
	t.Parallel()
	d := NewCounting[int](5)
	for _, v := range []int{1, 2, 3, 4, 5} {
		d.Push(v)
	}
	count := 0
	for range d.All() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("expected iteration to stop at 2, got %d", count)
	}
}
