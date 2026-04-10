// Package topk provides a generic, heap-based top-K collector.
//
// The Collector efficiently maintains the K highest-scoring items from a stream,
// using O(n log k) time complexity instead of O(n log n) for sorting.
//
// Example usage:
//
//	collector := topk.New[Issue](10, func(a, b Issue) bool {
//	    return a.ID < b.ID // Deterministic tie-breaking
//	})
//	for _, issue := range issues {
//	    collector.Add(issue, scores[issue.ID])
//	}
//	topIssues := collector.Results()
package topk

import (
	"container/heap"
	"sort"
)

// Scored pairs an item with its score.
type Scored[T any] struct {
	Item  T
	Score float64
}

// Collector collects the top-K highest-scoring items.
// It uses a min-heap internally to efficiently maintain the K highest scores.
//
// Time complexity:
//   - Add: O(log k) per item
//   - Results: O(k log k) to extract sorted results
//   - Total for n items: O(n log k)
type Collector[T any] struct {
	k    int
	h    *minHeap[T]
	less func(a, b T) bool // For deterministic ordering of equal scores
}

// New creates a new Collector for the top k items.
//
// The less function is used for deterministic ordering when scores are equal.
// If less is nil, items with equal scores may be returned in arbitrary order.
//
// If k <= 0, the collector will not collect any items.
func New[T any](k int, less func(a, b T) bool) *Collector[T] {
	if k < 0 {
		k = 0
	}
	h := &minHeap[T]{
		items: make([]Scored[T], 0, k),
		less:  less,
	}
	heap.Init(h)
	return &Collector[T]{
		k:    k,
		h:    h,
		less: less,
	}
}

// Add considers an item for inclusion in the top-K.
// Returns true if the item was added (score high enough or room available).
//
// Time complexity: O(log k)
func (c *Collector[T]) Add(item T, score float64) bool {
	if c.k <= 0 {
		return false
	}

	entry := Scored[T]{Item: item, Score: score}

	if c.h.Len() < c.k {
		heap.Push(c.h, entry)
		return true
	}

	// Compare against minimum (root of min-heap)
	if score > c.h.items[0].Score {
		heap.Pop(c.h)
		heap.Push(c.h, entry)
		return true
	}

	// Handle tie-breaking: if scores are equal, use less function
	if score == c.h.items[0].Score && c.less != nil {
		// If the new item should come before the current minimum in tie-breaking,
		// replace it for determinism
		if c.less(item, c.h.items[0].Item) {
			heap.Pop(c.h)
			heap.Push(c.h, entry)
			return true
		}
	}

	return false
}

// Len returns the current number of items in the collector.
func (c *Collector[T]) Len() int {
	return c.h.Len()
}

// Results returns the top-K items in descending score order.
// Items with equal scores are ordered by the less function if provided.
//
// Time complexity: O(k log k)
func (c *Collector[T]) Results() []T {
	scored := c.ResultsWithScores()
	result := make([]T, len(scored))
	for i, s := range scored {
		result[i] = s.Item
	}
	return result
}

// ResultsWithScores returns the top-K items with their scores in descending order.
// Items with equal scores are ordered by the less function if provided.
//
// Time complexity: O(k log k)
func (c *Collector[T]) ResultsWithScores() []Scored[T] {
	if c.h.Len() == 0 {
		return nil
	}

	// Copy heap contents
	result := make([]Scored[T], c.h.Len())
	copy(result, c.h.items)

	// Sort by score descending, then by less func for ties
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		if c.less != nil {
			return c.less(result[i].Item, result[j].Item)
		}
		return false
	})

	return result
}

// Reset clears the collector for reuse with the same k and less function.
func (c *Collector[T]) Reset() {
	c.h.items = c.h.items[:0]
}

// K returns the maximum number of items this collector will hold.
func (c *Collector[T]) K() int {
	return c.k
}

// minHeap implements heap.Interface for a min-heap of Scored items.
type minHeap[T any] struct {
	items []Scored[T]
	less  func(a, b T) bool
}

func (h *minHeap[T]) Len() int { return len(h.items) }

func (h *minHeap[T]) Less(i, j int) bool {
	// Min-heap: smaller scores at the top
	if h.items[i].Score != h.items[j].Score {
		return h.items[i].Score < h.items[j].Score
	}
	// Tie-breaking for determinism
	if h.less != nil {
		// Items that should come first (less=true) should have lower heap priority
		// so they're MORE likely to stay (they bubble up in the heap)
		return !h.less(h.items[i].Item, h.items[j].Item)
	}
	return false
}

func (h *minHeap[T]) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
}

func (h *minHeap[T]) Push(x any) {
	h.items = append(h.items, x.(Scored[T]))
}

func (h *minHeap[T]) Pop() any {
	old := h.items
	n := len(old)
	x := old[n-1]
	h.items = old[:n-1]
	return x
}
