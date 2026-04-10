package analysis

import (
	"math"
	"testing"
)

// createTestGraphStatsForAccessors creates a GraphStats with known test data.
func createTestGraphStatsForAccessors() *GraphStats {
	return NewGraphStatsForTest(
		map[string]float64{"issue-1": 0.5, "issue-2": 0.3, "issue-3": 0.2},  // pageRank
		map[string]float64{"issue-1": 10.0, "issue-2": 5.0, "issue-3": 2.0}, // betweenness
		map[string]float64{"issue-1": 0.8, "issue-2": 0.6, "issue-3": 0.4},  // eigenvector
		map[string]float64{"issue-1": 0.7, "issue-2": 0.5, "issue-3": 0.3},  // hubs
		map[string]float64{"issue-1": 0.9, "issue-2": 0.7, "issue-3": 0.5},  // authorities
		map[string]float64{"issue-1": 3.0, "issue-2": 2.0, "issue-3": 1.0},  // criticalPathScore
		map[string]int{"issue-1": 2, "issue-2": 1, "issue-3": 3},            // outDegree
		map[string]int{"issue-1": 1, "issue-2": 2, "issue-3": 1},            // inDegree
		[][]string{{"issue-1", "issue-2", "issue-1"}},                       // cycles
		0.5, // density
		[]string{"issue-3", "issue-2", "issue-1"}, // topologicalOrder
	)
}

// floatEq checks if two floats are approximately equal.
func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

// TestPageRankValue tests the single-value PageRank accessor.
func TestPageRankValue(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	t.Run("existing key", func(t *testing.T) {
		val, ok := stats.PageRankValue("issue-1")
		if !ok {
			t.Error("issue-1 should exist")
		}
		if !floatEq(val, 0.5) {
			t.Errorf("expected 0.5, got %f", val)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		val, ok := stats.PageRankValue("nonexistent")
		if ok {
			t.Error("nonexistent should not exist")
		}
		if val != 0 {
			t.Errorf("expected 0, got %f", val)
		}
	})

	t.Run("nil map returns false", func(t *testing.T) {
		emptyStats := &GraphStats{}
		val, ok := emptyStats.PageRankValue("issue-1")
		if ok {
			t.Error("expected false for nil map")
		}
		if val != 0 {
			t.Errorf("expected 0, got %f", val)
		}
	})
}

// TestPageRankAll tests the PageRank iterator.
func TestPageRankAll(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	t.Run("iterates all entries", func(t *testing.T) {
		visited := make(map[string]float64)
		stats.PageRankAll(func(id string, score float64) bool {
			visited[id] = score
			return true
		})

		if len(visited) != 3 {
			t.Errorf("expected 3 entries, got %d", len(visited))
		}
		if !floatEq(visited["issue-1"], 0.5) {
			t.Errorf("issue-1: expected 0.5, got %f", visited["issue-1"])
		}
		if !floatEq(visited["issue-2"], 0.3) {
			t.Errorf("issue-2: expected 0.3, got %f", visited["issue-2"])
		}
		if !floatEq(visited["issue-3"], 0.2) {
			t.Errorf("issue-3: expected 0.2, got %f", visited["issue-3"])
		}
	})

	t.Run("early termination", func(t *testing.T) {
		count := 0
		stats.PageRankAll(func(id string, score float64) bool {
			count++
			return false // Stop after first
		})
		if count != 1 {
			t.Errorf("expected 1 iteration, got %d", count)
		}
	})

	t.Run("nil map no-op", func(t *testing.T) {
		emptyStats := &GraphStats{}
		count := 0
		emptyStats.PageRankAll(func(id string, score float64) bool {
			count++
			return true
		})
		if count != 0 {
			t.Errorf("expected 0 iterations, got %d", count)
		}
	})
}

// TestPageRank_Isomorphic verifies new accessors return same data as old.
func TestPageRank_Isomorphic(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	// Get old map copy
	oldMap := stats.PageRank()

	// Verify every value from old map matches new accessor
	for id, expected := range oldMap {
		actual, ok := stats.PageRankValue(id)
		if !ok {
			t.Errorf("key %s should exist", id)
		}
		if expected != actual {
			t.Errorf("mismatch for %s: expected %f, got %f", id, expected, actual)
		}
	}

	// Verify iterator visits same keys
	iterated := make(map[string]float64)
	stats.PageRankAll(func(id string, score float64) bool {
		iterated[id] = score
		return true
	})
	if len(iterated) != len(oldMap) {
		t.Errorf("expected %d entries, got %d", len(oldMap), len(iterated))
	}
	for id, expected := range oldMap {
		if iterated[id] != expected {
			t.Errorf("iterator mismatch for %s", id)
		}
	}
}

// TestBetweennessValue tests betweenness single-value accessor.
func TestBetweennessValue(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	val, ok := stats.BetweennessValue("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if !floatEq(val, 10.0) {
		t.Errorf("expected 10.0, got %f", val)
	}

	_, ok = stats.BetweennessValue("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

// TestEigenvectorValue tests eigenvector single-value accessor.
func TestEigenvectorValue(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	val, ok := stats.EigenvectorValue("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if !floatEq(val, 0.8) {
		t.Errorf("expected 0.8, got %f", val)
	}

	_, ok = stats.EigenvectorValue("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

// TestHubValue tests hub single-value accessor.
func TestHubValue(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	val, ok := stats.HubValue("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if !floatEq(val, 0.7) {
		t.Errorf("expected 0.7, got %f", val)
	}

	_, ok = stats.HubValue("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

// TestAuthorityValue tests authority single-value accessor.
func TestAuthorityValue(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	val, ok := stats.AuthorityValue("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if !floatEq(val, 0.9) {
		t.Errorf("expected 0.9, got %f", val)
	}

	_, ok = stats.AuthorityValue("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

// TestCriticalPathValue tests critical path single-value accessor.
func TestCriticalPathValue(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	val, ok := stats.CriticalPathValue("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if !floatEq(val, 3.0) {
		t.Errorf("expected 3.0, got %f", val)
	}

	_, ok = stats.CriticalPathValue("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

// TestRankValueAccessors tests the rank accessor methods.
func TestRankValueAccessors(t *testing.T) {
	// Create stats with computed ranks
	stats := createTestGraphStatsForAccessors()
	// Manually populate rank maps since NewGraphStatsForTest doesn't compute them
	stats.pageRankRank = computeFloatRanks(stats.pageRank)
	stats.betweennessRank = computeFloatRanks(stats.betweenness)

	t.Run("PageRankRankValue", func(t *testing.T) {
		// issue-1 has highest pagerank (0.5), so rank 1
		rank, ok := stats.PageRankRankValue("issue-1")
		if !ok {
			t.Error("issue-1 should exist")
		}
		if rank != 1 {
			t.Errorf("expected rank 1, got %d", rank)
		}
	})

	t.Run("BetweennessRankValue", func(t *testing.T) {
		// issue-1 has highest betweenness (10.0), so rank 1
		rank, ok := stats.BetweennessRankValue("issue-1")
		if !ok {
			t.Error("issue-1 should exist")
		}
		if rank != 1 {
			t.Errorf("expected rank 1, got %d", rank)
		}
	})

	t.Run("missing returns false", func(t *testing.T) {
		_, ok := stats.PageRankRankValue("nonexistent")
		if ok {
			t.Error("nonexistent should not exist")
		}
	})
}

// TestIteratorEarlyTermination tests that all iterators respect early termination.
func TestIteratorEarlyTermination(t *testing.T) {
	stats := createTestGraphStatsForAccessors()

	iterators := []struct {
		name string
		fn   func(func(string, float64) bool)
	}{
		{"BetweennessAll", stats.BetweennessAll},
		{"EigenvectorAll", stats.EigenvectorAll},
		{"HubsAll", stats.HubsAll},
		{"AuthoritiesAll", stats.AuthoritiesAll},
		{"CriticalPathAll", stats.CriticalPathAll},
	}

	for _, it := range iterators {
		t.Run(it.name, func(t *testing.T) {
			count := 0
			it.fn(func(id string, score float64) bool {
				count++
				return false // Stop after first
			})
			if count != 1 {
				t.Errorf("%s should stop after first, got %d iterations", it.name, count)
			}
		})
	}
}

// TestCoreNumberValue tests k-core accessor.
func TestCoreNumberValue(t *testing.T) {
	stats := &GraphStats{
		coreNumber: map[string]int{"issue-1": 2, "issue-2": 1},
	}

	val, ok := stats.CoreNumberValue("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	_, ok = stats.CoreNumberValue("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

// TestSlackValue tests slack accessor.
func TestSlackValue(t *testing.T) {
	stats := &GraphStats{
		slack: map[string]float64{"issue-1": 0.0, "issue-2": 2.5},
	}

	val, ok := stats.SlackValue("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if val != 0.0 {
		t.Errorf("expected 0.0, got %f", val)
	}

	val, ok = stats.SlackValue("issue-2")
	if !ok {
		t.Error("issue-2 should exist")
	}
	if val != 2.5 {
		t.Errorf("expected 2.5, got %f", val)
	}

	_, ok = stats.SlackValue("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

// TestIsArticulationPoint tests articulation point accessor.
func TestIsArticulationPoint(t *testing.T) {
	stats := &GraphStats{
		articulation: map[string]bool{"issue-1": true, "issue-2": false},
	}

	isAP, ok := stats.IsArticulationPoint("issue-1")
	if !ok {
		t.Error("issue-1 should exist")
	}
	if !isAP {
		t.Error("issue-1 should be articulation point")
	}

	isAP, ok = stats.IsArticulationPoint("issue-2")
	if !ok {
		t.Error("issue-2 should exist")
	}
	if isAP {
		t.Error("issue-2 should not be articulation point")
	}

	_, ok = stats.IsArticulationPoint("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}
