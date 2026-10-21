package deques

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

func TestAgingPushPop(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](time.Second)
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
	})
}

func TestAgingEvictsByAge(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](100 * time.Millisecond)
		d.Push(1)
		time.Sleep(50 * time.Millisecond)
		d.Push(2)
		if d.Len() != 2 {
			t.Errorf("Len() at t=50ms = %d, want 2", d.Len())
		}
		time.Sleep(60 * time.Millisecond) // entry 1 is now 110ms old
		if d.Len() != 1 {
			t.Errorf("Len() at t=110ms = %d, want 1", d.Len())
		}
		if v, ok := d.Oldest(); !ok || v != 2 {
			t.Errorf("Oldest() = %d, %v; want 2, true", v, ok)
		}
		time.Sleep(60 * time.Millisecond) // entry 2 is now 110ms old
		if d.Len() != 0 {
			t.Errorf("Len() after all expired = %d, want 0", d.Len())
		}
	})
}

func TestAgingEmpty(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](time.Second)
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
	})
}

func TestAgingOldestNewest(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](time.Second)
		d.Push(1)
		d.Push(2)
		d.Push(3)
		if v, ok := d.Oldest(); !ok || v != 1 {
			t.Errorf("Oldest() = %d; want 1", v)
		}
		if v, ok := d.Newest(); !ok || v != 3 {
			t.Errorf("Newest() = %d; want 3", v)
		}
	})
}

func TestAgingAt(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](time.Second)
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
	})
}

func TestAgingClear(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](time.Second)
		d.Push(1)
		d.Push(2)
		d.Clear()
		if d.Len() != 0 {
			t.Errorf("Len() after Clear = %d, want 0", d.Len())
		}
		d.Push(3)
		if v, ok := d.Oldest(); !ok || v != 3 {
			t.Errorf("Oldest() after Clear+Push = %d; want 3", v)
		}
	})
}

func TestAgingSetMaxAge(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](time.Second)
		d.Push(1)
		time.Sleep(200 * time.Millisecond)
		d.Push(2)
		// Shrink the window so entry 1 (age 200ms) expires.
		d.SetMaxAge(100 * time.Millisecond)
		if d.MaxAge() != 100*time.Millisecond {
			t.Errorf("MaxAge() = %v, want 100ms", d.MaxAge())
		}
		if d.Len() != 1 {
			t.Errorf("Len() after SetMaxAge shrink = %d, want 1", d.Len())
		}
		if v, ok := d.Oldest(); !ok || v != 2 {
			t.Errorf("Oldest() after shrink = %d; want 2", v)
		}
	})
}

func TestAgingZeroMaxAge(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](0)
		d.Push(1)
		if d.Len() != 0 {
			t.Errorf("Len() with maxAge=0 = %d, want 0", d.Len())
		}
		d2 := NewAging[int](-time.Second)
		if d2.MaxAge() != 0 {
			t.Errorf("MaxAge() with negative = %v, want 0", d2.MaxAge())
		}
	})
}

func TestAgingAll(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](time.Second)
		for _, v := range []int{1, 2, 3} {
			d.Push(v)
		}
		got := slices.Collect(d.All())
		want := []int{3, 2, 1}
		if !slices.Equal(got, want) {
			t.Errorf("All() = %v, want %v", got, want)
		}
	})
}

func TestAgingAllSkipsExpired(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := NewAging[int](100 * time.Millisecond)
		d.Push(1)
		time.Sleep(60 * time.Millisecond)
		d.Push(2)
		time.Sleep(60 * time.Millisecond) // entry 1 is now 120ms old
		got := slices.Collect(d.All())
		want := []int{2}
		if !slices.Equal(got, want) {
			t.Errorf("All() = %v, want %v", got, want)
		}
	})
}
