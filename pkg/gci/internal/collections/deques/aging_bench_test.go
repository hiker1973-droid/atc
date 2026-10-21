package deques

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkAgingPushNoEviction(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewAging[int](time.Hour)
			for i := range n {
				d.Push(i)
			}
			b.ResetTimer()
			for b.Loop() {
				d.Push(0)
			}
		})
	}
}

func BenchmarkAgingPushPop(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewAging[int](time.Hour)
			for i := range n {
				d.Push(i)
			}
			b.ResetTimer()
			for b.Loop() {
				d.Push(0)
				_, _ = d.Pop()
			}
		})
	}
}

func BenchmarkAgingPopDrain(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				d := NewAging[int](time.Hour)
				for i := range n {
					d.Push(i)
				}
				b.StartTimer()
				for range n {
					_, _ = d.Pop()
				}
			}
		})
	}
}

func BenchmarkAgingPop(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewAging[int](time.Hour)
			for i := range n {
				d.Push(i)
			}
			b.ResetTimer()
			for b.Loop() {
				if _, ok := d.Pop(); !ok {
					b.StopTimer()
					for i := range n {
						d.Push(i)
					}
					b.StartTimer()
				}
			}
		})
	}
}

func BenchmarkAgingAt(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewAging[int](time.Hour)
			for i := range n {
				d.Push(i)
			}
			b.ResetTimer()
			i := 0
			for b.Loop() {
				v, _ := d.At(i % n)
				benchSink = v
				i++
			}
		})
	}
}

func BenchmarkAgingAll(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewAging[int](time.Hour)
			for i := range n {
				d.Push(i)
			}
			b.ResetTimer()
			for b.Loop() {
				sum := 0
				for v := range d.All() {
					sum += v
				}
				benchSink = sum
			}
		})
	}
}
