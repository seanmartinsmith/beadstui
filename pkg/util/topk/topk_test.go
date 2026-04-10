package topk

import (
	"math/rand"
	"sort"
	"testing"
)

// testItem is a simple struct for testing generic behavior.
type testItem struct {
	ID    string
	Value int
}

func TestNew(t *testing.T) {
	t.Run("positive k", func(t *testing.T) {
		c := New[int](5, nil)
		if c.K() != 5 {
			t.Errorf("K() = %d, want 5", c.K())
		}
		if c.Len() != 0 {
			t.Errorf("Len() = %d, want 0", c.Len())
		}
	})

	t.Run("zero k", func(t *testing.T) {
		c := New[int](0, nil)
		if c.K() != 0 {
			t.Errorf("K() = %d, want 0", c.K())
		}
	})

	t.Run("negative k normalizes to zero", func(t *testing.T) {
		c := New[int](-5, nil)
		if c.K() != 0 {
			t.Errorf("K() = %d, want 0", c.K())
		}
	})
}

func TestAdd_EdgeCases(t *testing.T) {
	t.Run("k=0 rejects all", func(t *testing.T) {
		c := New[int](0, nil)
		if c.Add(1, 100.0) {
			t.Error("Add should return false when k=0")
		}
		if c.Len() != 0 {
			t.Errorf("Len() = %d, want 0", c.Len())
		}
	})

	t.Run("k=1 keeps highest", func(t *testing.T) {
		c := New[int](1, nil)

		if !c.Add(1, 10.0) {
			t.Error("Add should return true for first item")
		}
		if !c.Add(2, 20.0) {
			t.Error("Add should return true for higher score")
		}
		if c.Add(3, 5.0) {
			t.Error("Add should return false for lower score")
		}

		results := c.Results()
		if len(results) != 1 {
			t.Fatalf("Results len = %d, want 1", len(results))
		}
		if results[0] != 2 {
			t.Errorf("Results[0] = %d, want 2", results[0])
		}
	})

	t.Run("k=n exact capacity", func(t *testing.T) {
		c := New[int](3, nil)
		c.Add(1, 1.0)
		c.Add(2, 2.0)
		c.Add(3, 3.0)

		if c.Len() != 3 {
			t.Errorf("Len() = %d, want 3", c.Len())
		}

		results := c.Results()
		if len(results) != 3 {
			t.Fatalf("Results len = %d, want 3", len(results))
		}
		// Should be sorted descending by score
		expected := []int{3, 2, 1}
		for i, want := range expected {
			if results[i] != want {
				t.Errorf("Results[%d] = %d, want %d", i, results[i], want)
			}
		}
	})

	t.Run("k>n accepts all items", func(t *testing.T) {
		c := New[int](10, nil)
		c.Add(1, 1.0)
		c.Add(2, 2.0)
		c.Add(3, 3.0)

		if c.Len() != 3 {
			t.Errorf("Len() = %d, want 3", c.Len())
		}

		results := c.Results()
		if len(results) != 3 {
			t.Fatalf("Results len = %d, want 3", len(results))
		}
	})
}

func TestAdd_ScoreOrdering(t *testing.T) {
	t.Run("maintains top-K highest scores", func(t *testing.T) {
		c := New[int](3, nil)

		// Add items in random order
		scores := []float64{5.0, 3.0, 8.0, 1.0, 9.0, 2.0, 7.0}
		for i, score := range scores {
			c.Add(i, score)
		}

		results := c.ResultsWithScores()
		if len(results) != 3 {
			t.Fatalf("Results len = %d, want 3", len(results))
		}

		// Should be top 3 scores: 9.0, 8.0, 7.0
		expectedScores := []float64{9.0, 8.0, 7.0}
		for i, want := range expectedScores {
			if results[i].Score != want {
				t.Errorf("Results[%d].Score = %f, want %f", i, results[i].Score, want)
			}
		}
	})

	t.Run("equal scores without less function", func(t *testing.T) {
		c := New[int](3, nil)
		c.Add(1, 5.0)
		c.Add(2, 5.0)
		c.Add(3, 5.0)

		results := c.Results()
		if len(results) != 3 {
			t.Fatalf("Results len = %d, want 3", len(results))
		}

		// All items should be present (order arbitrary without less func)
		seen := make(map[int]bool)
		for _, v := range results {
			seen[v] = true
		}
		for i := 1; i <= 3; i++ {
			if !seen[i] {
				t.Errorf("Missing item %d from results", i)
			}
		}
	})
}

func TestDeterministicOrdering(t *testing.T) {
	// less function for testItem: sort by ID ascending
	lessFunc := func(a, b testItem) bool {
		return a.ID < b.ID
	}

	t.Run("equal scores use less function", func(t *testing.T) {
		c := New[testItem](3, lessFunc)

		// All same score, should be ordered by ID
		c.Add(testItem{ID: "charlie", Value: 3}, 5.0)
		c.Add(testItem{ID: "alpha", Value: 1}, 5.0)
		c.Add(testItem{ID: "bravo", Value: 2}, 5.0)

		results := c.Results()
		if len(results) != 3 {
			t.Fatalf("Results len = %d, want 3", len(results))
		}

		// Should be sorted by ID: alpha, bravo, charlie
		expected := []string{"alpha", "bravo", "charlie"}
		for i, want := range expected {
			if results[i].ID != want {
				t.Errorf("Results[%d].ID = %s, want %s", i, results[i].ID, want)
			}
		}
	})

	t.Run("score takes precedence over less function", func(t *testing.T) {
		c := New[testItem](3, lessFunc)

		c.Add(testItem{ID: "zulu", Value: 1}, 10.0) // Highest score, should be first
		c.Add(testItem{ID: "alpha", Value: 2}, 5.0) // Lower score
		c.Add(testItem{ID: "bravo", Value: 3}, 5.0) // Same lower score

		results := c.Results()
		if len(results) != 3 {
			t.Fatalf("Results len = %d, want 3", len(results))
		}

		// zulu first (highest score), then alpha, bravo (equal scores, sorted by ID)
		if results[0].ID != "zulu" {
			t.Errorf("Results[0].ID = %s, want zulu", results[0].ID)
		}
		if results[1].ID != "alpha" {
			t.Errorf("Results[1].ID = %s, want alpha", results[1].ID)
		}
		if results[2].ID != "bravo" {
			t.Errorf("Results[2].ID = %s, want bravo", results[2].ID)
		}
	})

	t.Run("tie-breaking affects eviction", func(t *testing.T) {
		// With k=2, when we have equal scores, the less function determines
		// which items are kept
		c := New[testItem](2, lessFunc)

		c.Add(testItem{ID: "bravo", Value: 2}, 5.0)
		c.Add(testItem{ID: "charlie", Value: 3}, 5.0)
		// Now at capacity with bravo and charlie

		// alpha has same score but comes first in ordering
		// Since less(alpha, bravo) = true, alpha should replace the min
		c.Add(testItem{ID: "alpha", Value: 1}, 5.0)

		results := c.Results()
		if len(results) != 2 {
			t.Fatalf("Results len = %d, want 2", len(results))
		}

		// Should have alpha and bravo (the two "smallest" by ID)
		seen := make(map[string]bool)
		for _, r := range results {
			seen[r.ID] = true
		}

		if !seen["alpha"] {
			t.Error("Expected alpha to be in results")
		}
		if !seen["bravo"] {
			t.Error("Expected bravo to be in results")
		}
	})
}

func TestResultsWithScores(t *testing.T) {
	t.Run("empty collector returns nil", func(t *testing.T) {
		c := New[int](5, nil)
		results := c.ResultsWithScores()
		if results != nil {
			t.Errorf("ResultsWithScores() = %v, want nil", results)
		}
	})

	t.Run("returns items with scores", func(t *testing.T) {
		c := New[int](3, nil)
		c.Add(1, 10.0)
		c.Add(2, 20.0)
		c.Add(3, 30.0)

		results := c.ResultsWithScores()
		if len(results) != 3 {
			t.Fatalf("Results len = %d, want 3", len(results))
		}

		// Should be descending by score
		if results[0].Score != 30.0 || results[0].Item != 3 {
			t.Errorf("Results[0] = %+v, want {Item:3, Score:30}", results[0])
		}
		if results[1].Score != 20.0 || results[1].Item != 2 {
			t.Errorf("Results[1] = %+v, want {Item:2, Score:20}", results[1])
		}
		if results[2].Score != 10.0 || results[2].Item != 1 {
			t.Errorf("Results[2] = %+v, want {Item:1, Score:10}", results[2])
		}
	})
}

func TestReset(t *testing.T) {
	t.Run("clears collector for reuse", func(t *testing.T) {
		c := New[int](3, nil)
		c.Add(1, 10.0)
		c.Add(2, 20.0)
		c.Add(3, 30.0)

		if c.Len() != 3 {
			t.Errorf("Len() before reset = %d, want 3", c.Len())
		}

		c.Reset()

		if c.Len() != 0 {
			t.Errorf("Len() after reset = %d, want 0", c.Len())
		}
		if c.K() != 3 {
			t.Errorf("K() after reset = %d, want 3", c.K())
		}

		results := c.Results()
		if len(results) != 0 {
			t.Errorf("Results after reset = %v, want empty", results)
		}
	})

	t.Run("can add items after reset", func(t *testing.T) {
		c := New[int](2, nil)
		c.Add(1, 10.0)
		c.Add(2, 20.0)
		c.Reset()

		c.Add(100, 100.0)
		c.Add(200, 200.0)

		results := c.Results()
		if len(results) != 2 {
			t.Fatalf("Results len = %d, want 2", len(results))
		}
		if results[0] != 200 || results[1] != 100 {
			t.Errorf("Results = %v, want [200, 100]", results)
		}
	})
}

func TestResults_DoesNotModifyCollector(t *testing.T) {
	c := New[int](3, nil)
	c.Add(1, 10.0)
	c.Add(2, 20.0)
	c.Add(3, 30.0)

	// Call Results multiple times
	r1 := c.Results()
	r2 := c.Results()
	r3 := c.ResultsWithScores()

	// All should be equal
	if len(r1) != len(r2) || len(r1) != len(r3) {
		t.Error("Multiple Results() calls return different lengths")
	}

	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("Results mismatch at %d: %d vs %d", i, r1[i], r2[i])
		}
	}

	// Collector should still have items
	if c.Len() != 3 {
		t.Errorf("Len() = %d after Results(), want 3", c.Len())
	}
}

func TestLargeDataset(t *testing.T) {
	const n = 10000
	const k = 100

	c := New[int](k, func(a, b int) bool { return a < b })

	// Add n items with random scores
	rng := rand.New(rand.NewSource(42))
	type item struct {
		val   int
		score float64
	}
	allItems := make([]item, n)
	for i := 0; i < n; i++ {
		score := rng.Float64() * 1000
		allItems[i] = item{val: i, score: score}
		c.Add(i, score)
	}

	// Verify we have k results
	results := c.ResultsWithScores()
	if len(results) != k {
		t.Fatalf("Results len = %d, want %d", len(results), k)
	}

	// Verify scores are in descending order
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("Results not sorted: [%d].Score=%f > [%d].Score=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}

	// Verify we actually got the top k scores
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].score > allItems[j].score
	})

	topKScores := make(map[float64]bool)
	for i := 0; i < k; i++ {
		topKScores[allItems[i].score] = true
	}

	for _, r := range results {
		if !topKScores[r.Score] {
			t.Errorf("Result score %f is not in top %d", r.Score, k)
		}
	}
}

func TestNegativeScores(t *testing.T) {
	c := New[int](3, nil)

	c.Add(1, -10.0)
	c.Add(2, -5.0)
	c.Add(3, -20.0)
	c.Add(4, 0.0)
	c.Add(5, -1.0)

	results := c.ResultsWithScores()
	if len(results) != 3 {
		t.Fatalf("Results len = %d, want 3", len(results))
	}

	// Top 3 highest: 0.0, -1.0, -5.0
	expectedScores := []float64{0.0, -1.0, -5.0}
	for i, want := range expectedScores {
		if results[i].Score != want {
			t.Errorf("Results[%d].Score = %f, want %f", i, results[i].Score, want)
		}
	}
}

// Benchmarks

func BenchmarkTopK_Add(b *testing.B) {
	// Benchmark adding n items to collect top-100
	const k = 100
	rng := rand.New(rand.NewSource(42))

	b.Run("n=1000", func(b *testing.B) {
		scores := make([]float64, 1000)
		for i := range scores {
			scores[i] = rng.Float64()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c := New[int](k, nil)
			for j, score := range scores {
				c.Add(j, score)
			}
		}
	})

	b.Run("n=10000", func(b *testing.B) {
		scores := make([]float64, 10000)
		for i := range scores {
			scores[i] = rng.Float64()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c := New[int](k, nil)
			for j, score := range scores {
				c.Add(j, score)
			}
		}
	})

	b.Run("n=100000", func(b *testing.B) {
		scores := make([]float64, 100000)
		for i := range scores {
			scores[i] = rng.Float64()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c := New[int](k, nil)
			for j, score := range scores {
				c.Add(j, score)
			}
		}
	})
}

func BenchmarkTopK_VsSortSlice(b *testing.B) {
	// Compare top-K collector vs sort.Slice approach
	const n = 10000
	const k = 100

	rng := rand.New(rand.NewSource(42))
	type item struct {
		val   int
		score float64
	}
	items := make([]item, n)
	for i := range items {
		items[i] = item{val: i, score: rng.Float64()}
	}

	b.Run("TopK_Collector", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c := New[int](k, nil)
			for _, it := range items {
				c.Add(it.val, it.score)
			}
			_ = c.Results()
		}
	})

	b.Run("SortSlice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Copy slice to avoid modifying original
			tmp := make([]item, len(items))
			copy(tmp, items)

			sort.Slice(tmp, func(i, j int) bool {
				return tmp[i].score > tmp[j].score
			})

			// Take top k
			result := make([]int, k)
			for j := 0; j < k; j++ {
				result[j] = tmp[j].val
			}
			_ = result
		}
	})
}

func BenchmarkTopK_WithLessFunc(b *testing.B) {
	const n = 10000
	const k = 100

	rng := rand.New(rand.NewSource(42))
	scores := make([]float64, n)
	for i := range scores {
		scores[i] = rng.Float64()
	}

	lessFunc := func(a, b int) bool { return a < b }

	b.Run("without_less", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c := New[int](k, nil)
			for j, score := range scores {
				c.Add(j, score)
			}
			_ = c.Results()
		}
	})

	b.Run("with_less", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c := New[int](k, lessFunc)
			for j, score := range scores {
				c.Add(j, score)
			}
			_ = c.Results()
		}
	})
}

func BenchmarkTopK_VaryingK(b *testing.B) {
	const n = 10000

	rng := rand.New(rand.NewSource(42))
	scores := make([]float64, n)
	for i := range scores {
		scores[i] = rng.Float64()
	}

	for _, k := range []int{10, 50, 100, 500, 1000} {
		b.Run(
			"k="+string(rune('0'+k/100))+string(rune('0'+(k/10)%10))+string(rune('0'+k%10)),
			func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					c := New[int](k, nil)
					for j, score := range scores {
						c.Add(j, score)
					}
					_ = c.Results()
				}
			})
	}
}
