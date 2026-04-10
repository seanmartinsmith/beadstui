package analysis

import (
	"fmt"
	"testing"
)

// createLargeGraphStats creates a GraphStats with n entries for benchmarking.
func createLargeGraphStats(n int) *GraphStats {
	pageRank := make(map[string]float64, n)
	betweenness := make(map[string]float64, n)
	eigenvector := make(map[string]float64, n)
	hubs := make(map[string]float64, n)
	authorities := make(map[string]float64, n)
	criticalPath := make(map[string]float64, n)
	outDegree := make(map[string]int, n)
	inDegree := make(map[string]int, n)

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("issue-%d", i)
		pageRank[id] = float64(i) / float64(n)
		betweenness[id] = float64(n-i) / float64(n)
		eigenvector[id] = float64(i%100) / 100.0
		hubs[id] = float64(i%50) / 50.0
		authorities[id] = float64(i%75) / 75.0
		criticalPath[id] = float64(i % 10)
		outDegree[id] = i % 5
		inDegree[id] = (n - i) % 5
	}

	return NewGraphStatsForTest(
		pageRank,
		betweenness,
		eigenvector,
		hubs,
		authorities,
		criticalPath,
		outDegree,
		inDegree,
		nil, // no cycles
		0.01,
		nil, // no topo order
	)
}

// BenchmarkPageRank_MapCopy benchmarks the old map copy approach.
// This copies O(n) data on each call.
func BenchmarkPageRank_MapCopy(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		stats := createLargeGraphStats(n)
		targetID := fmt.Sprintf("issue-%d", n/2)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				m := stats.PageRank()
				_ = m[targetID]
			}
		})
	}
}

// BenchmarkPageRank_SingleValue benchmarks the new O(1) single-value approach.
func BenchmarkPageRank_SingleValue(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		stats := createLargeGraphStats(n)
		targetID := fmt.Sprintf("issue-%d", n/2)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = stats.PageRankValue(targetID)
			}
		})
	}
}

// BenchmarkBetweenness_MapCopy benchmarks the old map copy approach for betweenness.
func BenchmarkBetweenness_MapCopy(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		stats := createLargeGraphStats(n)
		targetID := fmt.Sprintf("issue-%d", n/2)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				m := stats.Betweenness()
				_ = m[targetID]
			}
		})
	}
}

// BenchmarkBetweenness_SingleValue benchmarks the new O(1) single-value approach.
func BenchmarkBetweenness_SingleValue(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		stats := createLargeGraphStats(n)
		targetID := fmt.Sprintf("issue-%d", n/2)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = stats.BetweennessValue(targetID)
			}
		})
	}
}

// BenchmarkIteration_MapCopy benchmarks iterating via map copy.
func BenchmarkIteration_MapCopy(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		stats := createLargeGraphStats(n)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				m := stats.PageRank()
				sum := 0.0
				for _, v := range m {
					sum += v
				}
				_ = sum
			}
		})
	}
}

// BenchmarkIteration_All benchmarks iterating via All() method.
func BenchmarkIteration_All(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, n := range sizes {
		stats := createLargeGraphStats(n)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sum := 0.0
				stats.PageRankAll(func(id string, score float64) bool {
					sum += score
					return true
				})
				_ = sum
			}
		})
	}
}

// BenchmarkMultipleAccess_MapCopy benchmarks accessing multiple values via map copy.
// This simulates a hot path that needs 5 different metric values for one issue.
func BenchmarkMultipleAccess_MapCopy(b *testing.B) {
	n := 1000
	stats := createLargeGraphStats(n)
	targetID := fmt.Sprintf("issue-%d", n/2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stats.PageRank()[targetID]
		_ = stats.Betweenness()[targetID]
		_ = stats.Eigenvector()[targetID]
		_ = stats.Hubs()[targetID]
		_ = stats.Authorities()[targetID]
	}
}

// BenchmarkMultipleAccess_SingleValue benchmarks accessing multiple values via single-value accessors.
func BenchmarkMultipleAccess_SingleValue(b *testing.B) {
	n := 1000
	stats := createLargeGraphStats(n)
	targetID := fmt.Sprintf("issue-%d", n/2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = stats.PageRankValue(targetID)
		_, _ = stats.BetweennessValue(targetID)
		_, _ = stats.EigenvectorValue(targetID)
		_, _ = stats.HubValue(targetID)
		_, _ = stats.AuthorityValue(targetID)
	}
}
