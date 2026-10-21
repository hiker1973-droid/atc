// Package sets provides operations on sets implemented as map[T]struct{}.
//
// It complements the standard library's slices and maps packages by
// offering Union, Intersection, Difference, and other common set
// operations along with iter.Seq-compatible iterators.
package sets

import "iter"

// Set is a generic set backed by a map with empty-struct values.
type Set[T comparable] map[T]struct{}

// New returns an empty set.
func New[T comparable]() Set[T] {
	return make(Set[T])
}

// Of returns a set containing the given elements.
func Of[T comparable](elements ...T) Set[T] {
	s := make(Set[T], len(elements))
	for _, e := range elements {
		s[e] = struct{}{}
	}
	return s
}

// Add inserts an element into s.
func Add[T comparable](s Set[T], e T) {
	s[e] = struct{}{}
}

// Remove deletes an element from s. It is a no-op if the element is absent.
func Remove[T comparable](s Set[T], e T) {
	delete(s, e)
}

// Contains reports whether s contains e.
func Contains[T comparable](s Set[T], e T) bool {
	_, ok := s[e]
	return ok
}

// Len returns the number of elements in s.
func Len[T comparable](s Set[T]) int {
	return len(s)
}

// Clone returns a shallow copy of s.
func Clone[T comparable](s Set[T]) Set[T] {
	c := make(Set[T], len(s))
	for e := range s {
		c[e] = struct{}{}
	}
	return c
}

// All returns an iterator over the elements of s. Iteration order is
// unspecified, matching Go map iteration semantics.
func All[T comparable](s Set[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := range s {
			if !yield(e) {
				return
			}
		}
	}
}

// Collect returns a set containing the elements yielded by seq.
func Collect[T comparable](seq iter.Seq[T]) Set[T] {
	s := make(Set[T])
	for e := range seq {
		s[e] = struct{}{}
	}
	return s
}

// Union returns a new set containing every element present in a or b.
func Union[T comparable](a, b Set[T]) Set[T] {
	u := make(Set[T], len(a)+len(b))
	for e := range a {
		u[e] = struct{}{}
	}
	for e := range b {
		u[e] = struct{}{}
	}
	return u
}

// Intersection returns a new set containing every element present in both a and b.
func Intersection[T comparable](a, b Set[T]) Set[T] {
	if len(a) > len(b) {
		a, b = b, a
	}
	i := make(Set[T])
	for e := range a {
		if _, ok := b[e]; ok {
			i[e] = struct{}{}
		}
	}
	return i
}

// Difference returns a new set containing elements in a that are not in b.
func Difference[T comparable](a, b Set[T]) Set[T] {
	d := make(Set[T])
	for e := range a {
		if _, ok := b[e]; !ok {
			d[e] = struct{}{}
		}
	}
	return d
}

// SymmetricDifference returns a new set containing elements in either a or b but not both.
func SymmetricDifference[T comparable](a, b Set[T]) Set[T] {
	sd := make(Set[T])
	for e := range a {
		if _, ok := b[e]; !ok {
			sd[e] = struct{}{}
		}
	}
	for e := range b {
		if _, ok := a[e]; !ok {
			sd[e] = struct{}{}
		}
	}
	return sd
}

// IsSubset reports whether every element of a is in b.
func IsSubset[T comparable](a, b Set[T]) bool {
	if len(a) > len(b) {
		return false
	}
	for e := range a {
		if _, ok := b[e]; !ok {
			return false
		}
	}
	return true
}

// IsSuperset reports whether every element of b is in a.
func IsSuperset[T comparable](a, b Set[T]) bool {
	return IsSubset(b, a)
}

// IsDisjoint reports whether a and b share no elements.
func IsDisjoint[T comparable](a, b Set[T]) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	for e := range a {
		if _, ok := b[e]; ok {
			return false
		}
	}
	return true
}

// Equal reports whether a and b contain exactly the same elements.
func Equal[T comparable](a, b Set[T]) bool {
	if len(a) != len(b) {
		return false
	}
	for e := range a {
		if _, ok := b[e]; !ok {
			return false
		}
	}
	return true
}
