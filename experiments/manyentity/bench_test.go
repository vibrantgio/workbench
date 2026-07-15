package manyentity

import (
	"fmt"
	"image"
	"sort"
	"testing"
	"time"

	"gioui.org/op"
)

const numNodes = 200

var benchSize = image.Point{X: 800, Y: 600}

func warmGraph(n, frames int) *Graph {
	g := NewGraph(n)
	for i := 0; i < frames; i++ {
		g.Tick(1.0)
	}
	return g
}

// BenchmarkNaive200 measures strategy A: per-node clip/paint, all nodes moving.
func BenchmarkNaive200(b *testing.B) {
	g := warmGraph(numNodes, 60)
	ops := new(op.Ops)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Tick(1.0)
		ops.Reset()
		RenderNaive(ops, g, benchSize)
	}
}

// BenchmarkCachedNonEq200 measures strategy B at non-equilibrium (all nodes moving → all cache misses).
func BenchmarkCachedNonEq200(b *testing.B) {
	g := warmGraph(numNodes, 60)
	cache := NewNodeCache(numNodes)
	ops := new(op.Ops)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Tick(1.0)
		ops.Reset()
		cache.Render(ops, g, benchSize)
	}
}

// BenchmarkCachedEq200 measures strategy B at equilibrium (no movement → all cache hits).
func BenchmarkCachedEq200(b *testing.B) {
	g := warmGraph(numNodes, 500) // run longer to approach equilibrium
	cache := NewNodeCache(numNodes)
	ops := new(op.Ops)
	// Prime the cache at current positions.
	ops.Reset()
	cache.Render(ops, g, benchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// No Tick: positions unchanged → all cache hits.
		ops.Reset()
		cache.Render(ops, g, benchSize)
	}
}

// BenchmarkScene200 measures strategy C: single batched path, all nodes moving.
func BenchmarkScene200(b *testing.B) {
	g := warmGraph(numNodes, 60)
	ops := new(op.Ops)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Tick(1.0)
		ops.Reset()
		RenderScene(ops, g, benchSize)
	}
}

// TestHistogram measures wall-clock frame-time distribution for all four scenarios.
// Run with: go test -run TestHistogram -v
func TestHistogram(t *testing.T) {
	const warmFrames = 100
	const measFrames = 1000

	type scenario struct {
		name string
		tick bool
		fn   func()
	}

	var scenarios []scenario

	// A: Naive, non-equilibrium
	{
		g := warmGraph(numNodes, warmFrames)
		ops := new(op.Ops)
		scenarios = append(scenarios, scenario{
			name: "A.Naive-NonEq",
			tick: true,
			fn: func() {
				g.Tick(1.0)
				ops.Reset()
				RenderNaive(ops, g, benchSize)
			},
		})
	}

	// B1: Cached, non-equilibrium (all misses)
	{
		g := warmGraph(numNodes, warmFrames)
		cache := NewNodeCache(numNodes)
		ops := new(op.Ops)
		scenarios = append(scenarios, scenario{
			name: "B1.Cached-NonEq",
			fn: func() {
				g.Tick(1.0)
				ops.Reset()
				cache.Render(ops, g, benchSize)
			},
		})
	}

	// B2: Cached, equilibrium (all hits)
	{
		g := warmGraph(numNodes, 500)
		cache := NewNodeCache(numNodes)
		ops := new(op.Ops)
		ops.Reset()
		cache.Render(ops, g, benchSize) // prime cache
		scenarios = append(scenarios, scenario{
			name: "B2.Cached-Eq",
			fn: func() {
				ops.Reset()
				cache.Render(ops, g, benchSize)
			},
		})
	}

	// C: Scene primitive, non-equilibrium
	{
		g := warmGraph(numNodes, warmFrames)
		ops := new(op.Ops)
		scenarios = append(scenarios, scenario{
			name: "C.Scene-NonEq",
			fn: func() {
				g.Tick(1.0)
				ops.Reset()
				RenderScene(ops, g, benchSize)
			},
		})
	}

	type stats struct {
		p50, p95, p99, max time.Duration
		buckets            [6]int
	}

	bucketEdges := [6]time.Duration{
		2 * time.Millisecond,
		4 * time.Millisecond,
		8 * time.Millisecond,
		12 * time.Millisecond,
		16670 * time.Microsecond,
		1<<62 - 1, // sentinel: >16.67ms
	}
	bucketLabels := [6]string{"<2ms", "2-4ms", "4-8ms", "8-12ms", "12-16.67ms", ">16.67ms"}

	computeStats := func(times []time.Duration) stats {
		sorted := make([]time.Duration, len(times))
		copy(sorted, times)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		n := len(sorted)
		var s stats
		s.p50 = sorted[n*50/100]
		s.p95 = sorted[n*95/100]
		s.p99 = sorted[n*99/100]
		s.max = sorted[n-1]
		for _, d := range sorted {
			for k, edge := range bucketEdges {
				if d < edge {
					s.buckets[k]++
					break
				}
			}
		}
		return s
	}

	fmt.Printf("\n=== Frame-time histogram  n=%d  %d frames  size=%dx%d ===\n\n",
		numNodes, measFrames, benchSize.X, benchSize.Y)
	fmt.Printf("%-20s  %8s  %8s  %8s  %8s  histogram\n",
		"strategy", "p50", "p95", "p99", "max")
	fmt.Printf("%-20s  %8s  %8s  %8s  %8s  ---------\n",
		"--------------------", "--------", "--------", "--------", "--------")

	for _, sc := range scenarios {
		times := make([]time.Duration, measFrames)
		for i := range times {
			t0 := time.Now()
			sc.fn()
			times[i] = time.Since(t0)
		}
		s := computeStats(times)

		hist := ""
		for k, count := range s.buckets {
			if count > 0 {
				hist += fmt.Sprintf("%s:%d ", bucketLabels[k], count)
			}
		}

		fmt.Printf("%-20s  %8s  %8s  %8s  %8s  %s\n",
			sc.name,
			s.p50.Round(time.Microsecond),
			s.p95.Round(time.Microsecond),
			s.p99.Round(time.Microsecond),
			s.max.Round(time.Microsecond),
			hist,
		)
	}
	fmt.Println()
}
