package deques

import (
	"fmt"
	"testing"
)

const benchSizeFmt = "n=%d"

var benchSizes = []int{100, 1_000, 10_000, 100_000, 1_000_000}

var benchSink int

func BenchmarkCountingPushAtCap(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewCounting[int](n)
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

func BenchmarkCountingPushPop(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewCounting[int](n)
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

func BenchmarkCountingPopDrain(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				d := NewCounting[int](n)
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

func BenchmarkCountingPop(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewCounting[int](n)
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

func BenchmarkCountingAt(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewCounting[int](n)
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

func BenchmarkCountingAll(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf(benchSizeFmt, n), func(b *testing.B) {
			d := NewCounting[int](n)
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
