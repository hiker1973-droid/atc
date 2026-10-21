package deques

import "iter"

// Counting is a FIFO queue that drops the oldest entry whenever a Push
// would cause the queue to exceed its maximum length.
//
// Internally it uses a ring buffer for O(1) push, pop, and indexed access.
type Counting[T any] struct {
	buf    []T
	head   int
	count  int
	maxLen int
}

// NewCounting returns a Counting queue with the given maximum length.
// A non-positive maxLen is treated as 0 (the queue accepts no entries).
func NewCounting[T any](maxLen int) *Counting[T] {
	if maxLen < 0 {
		maxLen = 0
	}
	d := &Counting[T]{maxLen: maxLen}
	if maxLen > 0 {
		d.buf = make([]T, maxLen)
	}
	return d
}

// Push adds v to the back of the queue, evicting the oldest entry if the
// queue is at capacity.
func (d *Counting[T]) Push(v T) {
	if d.maxLen == 0 {
		return
	}
	idx := (d.head + d.count) % d.maxLen
	d.buf[idx] = v
	if d.count == d.maxLen {
		d.head = (d.head + 1) % d.maxLen
	} else {
		d.count++
	}
}

// Pop removes and returns the oldest entry. The second return value is
// false if the queue is empty.
func (d *Counting[T]) Pop() (T, bool) {
	var zero T
	if d.count == 0 {
		return zero, false
	}
	v := d.buf[d.head]
	d.buf[d.head] = zero
	d.head = (d.head + 1) % d.maxLen
	d.count--
	return v, true
}

// Oldest returns the oldest entry without removing it. The second return
// value is false if the queue is empty.
func (d *Counting[T]) Oldest() (T, bool) {
	var zero T
	if d.count == 0 {
		return zero, false
	}
	return d.buf[d.head], true
}

// Newest returns the most recently pushed entry without removing it. The
// second return value is false if the queue is empty.
func (d *Counting[T]) Newest() (T, bool) {
	var zero T
	if d.count == 0 {
		return zero, false
	}
	idx := (d.head + d.count - 1) % d.maxLen
	return d.buf[idx], true
}

// At returns the entry at the given index, where 0 is the newest entry.
// The second return value is false if i < 0 or i >= Len().
func (d *Counting[T]) At(i int) (T, bool) {
	var zero T
	if i < 0 || i >= d.count {
		return zero, false
	}
	return d.buf[(d.head+d.count-1-i)%d.maxLen], true
}

// Len returns the current number of entries.
func (d *Counting[T]) Len() int {
	return d.count
}

// Clear removes all entries from the queue.
func (d *Counting[T]) Clear() {
	clear(d.buf)
	d.head = 0
	d.count = 0
}

// Cap returns the maximum length configured for the queue.
func (d *Counting[T]) Cap() int {
	return d.maxLen
}

// SetCap updates the maximum length, evicting the oldest entries
// immediately if the new cap is smaller than the current length.
// A non-positive maxLen is treated as 0.
func (d *Counting[T]) SetCap(maxLen int) {
	if maxLen < 0 {
		maxLen = 0
	}
	newBuf := make([]T, maxLen)
	keep := min(d.count, maxLen)
	skip := d.count - keep
	for i := range keep {
		newBuf[i] = d.buf[(d.head+skip+i)%d.maxLen]
	}
	d.buf = newBuf
	d.head = 0
	d.count = keep
	d.maxLen = maxLen
}

// All returns an iterator over the entries from newest to oldest.
func (d *Counting[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := d.count - 1; i >= 0; i-- {
			if !yield(d.buf[(d.head+i)%d.maxLen]) {
				return
			}
		}
	}
}
