package deques

import (
	"iter"
	"time"
)

type agingEntry[T any] struct {
	value   T
	addedAt time.Time
}

// Aging is a FIFO queue that drops entries older than a configured
// maximum age. Eviction happens lazily on every operation.
//
// Internally it uses a head pointer with periodic compaction for
// amortized O(1) push and pop.
type Aging[T any] struct {
	items  []agingEntry[T]
	head   int
	maxAge time.Duration
}

// NewAging returns an Aging queue with the given maximum age. A
// non-positive maxAge is treated as 0 (every entry expires immediately).
func NewAging[T any](maxAge time.Duration) *Aging[T] {
	if maxAge < 0 {
		maxAge = 0
	}
	return &Aging[T]{maxAge: maxAge}
}

// Push adds v to the back of the queue with the current timestamp.
func (d *Aging[T]) Push(v T) {
	d.items = append(d.items, agingEntry[T]{value: v, addedAt: time.Now()})
	d.evict()
}

// Pop removes and returns the oldest non-expired entry. The second
// return value is false if no such entry exists.
func (d *Aging[T]) Pop() (T, bool) {
	d.evict()
	var zero T
	if d.head >= len(d.items) {
		return zero, false
	}
	v := d.items[d.head].value
	d.items[d.head] = agingEntry[T]{}
	d.head++
	d.compact()
	return v, true
}

// Oldest returns the oldest non-expired entry without removing it. The
// second return value is false if no such entry exists.
func (d *Aging[T]) Oldest() (T, bool) {
	d.evict()
	var zero T
	if d.head >= len(d.items) {
		return zero, false
	}
	return d.items[d.head].value, true
}

// Newest returns the most recently pushed non-expired entry without
// removing it. The second return value is false if no such entry exists.
func (d *Aging[T]) Newest() (T, bool) {
	d.evict()
	var zero T
	if d.head >= len(d.items) {
		return zero, false
	}
	return d.items[len(d.items)-1].value, true
}

// At returns the non-expired entry at the given index, where 0 is the
// newest non-expired entry. The second return value is false if
// i < 0 or i >= Len().
func (d *Aging[T]) At(i int) (T, bool) {
	d.evict()
	var zero T
	actual := len(d.items) - 1 - i
	if i < 0 || actual < d.head {
		return zero, false
	}
	return d.items[actual].value, true
}

// Len returns the number of non-expired entries.
func (d *Aging[T]) Len() int {
	d.evict()
	return len(d.items) - d.head
}

// Clear removes all entries from the queue.
func (d *Aging[T]) Clear() {
	clear(d.items)
	d.items = d.items[:0]
	d.head = 0
}

// MaxAge returns the configured maximum entry age.
func (d *Aging[T]) MaxAge() time.Duration {
	return d.maxAge
}

// SetMaxAge updates the maximum entry age. Entries that exceed the new
// age are evicted lazily on the next operation. A non-positive maxAge is
// treated as 0 (every entry expires immediately).
func (d *Aging[T]) SetMaxAge(maxAge time.Duration) {
	if maxAge < 0 {
		maxAge = 0
	}
	d.maxAge = maxAge
	d.evict()
}

// All returns an iterator over the non-expired entries from newest to oldest.
func (d *Aging[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		d.evict()
		for i := len(d.items) - 1; i >= d.head; i-- {
			if !yield(d.items[i].value) {
				return
			}
		}
	}
}

// evict drops every entry whose age exceeds maxAge by advancing the head pointer.
func (d *Aging[T]) evict() {
	live := d.items[d.head:]
	if len(live) == 0 {
		return
	}
	if d.maxAge == 0 {
		clear(d.items)
		d.items = d.items[:0]
		d.head = 0
		return
	}
	now := time.Now()
	evicted := 0
	for evicted < len(live) && now.Sub(live[evicted].addedAt) > d.maxAge {
		live[evicted] = agingEntry[T]{}
		evicted++
	}
	d.head += evicted
	d.compact()
}

// compact reclaims dead space when the head pointer has advanced past
// half the underlying slice.
func (d *Aging[T]) compact() {
	if d.head > 0 && d.head >= len(d.items)/2 {
		n := copy(d.items, d.items[d.head:])
		clear(d.items[n:])
		d.items = d.items[:n]
		d.head = 0
	}
}
