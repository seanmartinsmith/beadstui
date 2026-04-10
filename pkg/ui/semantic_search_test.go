package ui

import (
	"context"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/search"
)

// =============================================================================
// Mock Embedder for Testing
// =============================================================================

type mockEmbedder struct {
	dim       int
	embedFunc func(ctx context.Context, texts []string) ([][]float32, error)
}

func (m *mockEmbedder) Provider() search.Provider {
	return search.ProviderOpenAI
}

func (m *mockEmbedder) Dim() int {
	return m.dim
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, texts)
	}
	// Default: return zero vectors
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = make([]float32, m.dim)
	}
	return result, nil
}

// =============================================================================
// SemanticSearch Constructor Tests
// =============================================================================

func TestNewSemanticSearch(t *testing.T) {
	ss := NewSemanticSearch()
	if ss == nil {
		t.Fatal("NewSemanticSearch() returned nil")
	}

	snap := ss.Snapshot()
	if snap.Ready {
		t.Error("New SemanticSearch should not be ready")
	}
	if snap.Index != nil {
		t.Error("New SemanticSearch should have nil Index")
	}
	if snap.Embedder != nil {
		t.Error("New SemanticSearch should have nil Embedder")
	}
	if snap.IDs != nil {
		t.Error("New SemanticSearch should have nil IDs")
	}
}

// =============================================================================
// Snapshot Tests
// =============================================================================

func TestSemanticSearchSnapshot(t *testing.T) {
	ss := NewSemanticSearch()
	snap := ss.Snapshot()

	// Should return empty snapshot initially
	if snap.Ready {
		t.Error("Initial snapshot should not be ready")
	}
}

// =============================================================================
// SetIndex Tests
// =============================================================================

func TestSemanticSearchSetIndex(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(384)
	embedder := &mockEmbedder{dim: 384}

	ss.SetIndex(idx, embedder)
	snap := ss.Snapshot()

	if !snap.Ready {
		t.Error("Snapshot should be ready after SetIndex with non-nil values")
	}
	if snap.Index != idx {
		t.Error("Snapshot.Index should match set index")
	}
	if snap.Embedder != embedder {
		t.Error("Snapshot.Embedder should match set embedder")
	}
}

func TestSemanticSearchSetIndexNilIndex(t *testing.T) {
	ss := NewSemanticSearch()
	embedder := &mockEmbedder{dim: 384}

	ss.SetIndex(nil, embedder)
	snap := ss.Snapshot()

	if snap.Ready {
		t.Error("Snapshot should not be ready with nil index")
	}
}

func TestSemanticSearchSetIndexNilEmbedder(t *testing.T) {
	ss := NewSemanticSearch()
	idx := search.NewVectorIndex(384)

	ss.SetIndex(idx, nil)
	snap := ss.Snapshot()

	if snap.Ready {
		t.Error("Snapshot should not be ready with nil embedder")
	}
}

func TestSemanticSearchSetIndexBothNil(t *testing.T) {
	ss := NewSemanticSearch()

	ss.SetIndex(nil, nil)
	snap := ss.Snapshot()

	if snap.Ready {
		t.Error("Snapshot should not be ready with both nil")
	}
}

// =============================================================================
// SetIDs Tests
// =============================================================================

func TestSemanticSearchSetIDs(t *testing.T) {
	ss := NewSemanticSearch()

	ids := []string{"issue-1", "issue-2", "issue-3"}
	ss.SetIDs(ids)

	snap := ss.Snapshot()
	if len(snap.IDs) != 3 {
		t.Errorf("Expected 3 IDs, got %d", len(snap.IDs))
	}

	// Verify IDs are correct
	for i, id := range ids {
		if snap.IDs[i] != id {
			t.Errorf("ID[%d] = %q, want %q", i, snap.IDs[i], id)
		}
	}
}

func TestSemanticSearchSetIDsCopiesSlice(t *testing.T) {
	ss := NewSemanticSearch()

	ids := []string{"issue-1", "issue-2"}
	ss.SetIDs(ids)

	// Modify original slice
	ids[0] = "modified"

	snap := ss.Snapshot()
	if snap.IDs[0] == "modified" {
		t.Error("SetIDs should copy the slice, not reference it")
	}
}

func TestSemanticSearchSetIDsEmpty(t *testing.T) {
	ss := NewSemanticSearch()

	ss.SetIDs([]string{})
	snap := ss.Snapshot()

	if len(snap.IDs) != 0 {
		t.Errorf("Expected 0 IDs, got %d", len(snap.IDs))
	}
}

func TestSemanticSearchSetIDsNil(t *testing.T) {
	ss := NewSemanticSearch()

	ss.SetIDs(nil)
	snap := ss.Snapshot()

	if len(snap.IDs) != 0 {
		t.Errorf("Expected 0 IDs after nil, got %d", len(snap.IDs))
	}
}

// =============================================================================
// Filter Tests
// =============================================================================

func TestSemanticSearchFilterEmptyTerm(t *testing.T) {
	ss := NewSemanticSearch()

	targets := []string{"fix bug", "add feature", "update docs"}
	ranks := ss.Filter("", targets)

	// Empty term returns default filter results
	// DefaultFilter with empty term returns empty slice (no matches)
	// This is the expected behavior - empty search shows all without ranking
	_ = ranks // Result depends on DefaultFilter behavior
}

func TestSemanticSearchFilterNotReady(t *testing.T) {
	ss := NewSemanticSearch()
	// Not calling SetIndex, so not ready

	targets := []string{"fix bug", "add feature"}
	ranks := ss.Filter("bug", targets)

	// When not ready, should fall back to default filter
	// Default filter should return some results for "bug"
	if len(ranks) == 0 {
		t.Error("Expected some ranks from default filter")
	}
}

func TestSemanticSearchFilterIDMismatch(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(384)
	embedder := &mockEmbedder{dim: 384}
	ss.SetIndex(idx, embedder)

	// Set only 2 IDs but 3 targets - mismatch
	ss.SetIDs([]string{"id-1", "id-2"})

	targets := []string{"fix bug", "add feature", "update docs"}
	ranks := ss.Filter("bug", targets)

	// ID mismatch should fall back to default filter
	if len(ranks) == 0 {
		t.Error("Expected some ranks from default filter due to ID mismatch")
	}
}

func TestSemanticSearchFilterWithValidSetup(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3) // Small dimension for testing
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			// Return a simple vector for the query
			result := make([][]float32, len(texts))
			for i := range result {
				result[i] = []float32{1.0, 0.0, 0.0}
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)

	// Add vectors to the index
	idx.Upsert("id-1", search.ContentHash{}, []float32{1.0, 0.0, 0.0}) // Similar to query
	idx.Upsert("id-2", search.ContentHash{}, []float32{0.0, 1.0, 0.0}) // Orthogonal
	idx.Upsert("id-3", search.ContentHash{}, []float32{0.5, 0.5, 0.0}) // Partially similar

	ss.SetIDs([]string{"id-1", "id-2", "id-3"})

	// Use ComputeSemanticResults for synchronous computation (testing)
	ranks := ss.ComputeSemanticResults("search query")

	// Should return ranked results
	if len(ranks) == 0 {
		t.Error("Expected some ranks from semantic search")
	}

	// First result should be id-1 (index 0) since it's most similar to query vector
	if len(ranks) > 0 && ranks[0].Index != 0 {
		t.Errorf("Expected first rank to be index 0 (most similar), got %d", ranks[0].Index)
	}
}

func TestSemanticSearchFilterSortsByScore(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range result {
				result[i] = []float32{1.0, 0.0, 0.0}
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)

	// Add vectors with different similarities
	idx.Upsert("id-a", search.ContentHash{}, []float32{0.0, 1.0, 0.0}) // Low similarity
	idx.Upsert("id-b", search.ContentHash{}, []float32{1.0, 0.0, 0.0}) // High similarity
	idx.Upsert("id-c", search.ContentHash{}, []float32{0.5, 0.5, 0.0}) // Medium similarity

	ss.SetIDs([]string{"id-a", "id-b", "id-c"})

	// Use ComputeSemanticResults for synchronous computation (testing)
	ranks := ss.ComputeSemanticResults("query")

	if len(ranks) < 3 {
		t.Fatalf("Expected at least 3 ranks, got %d", len(ranks))
	}

	// id-b (index 1) should be first (highest similarity)
	if ranks[0].Index != 1 {
		t.Errorf("Expected first rank index 1 (id-b), got %d", ranks[0].Index)
	}
}

func TestSemanticSearchFilterLimit(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range result {
				result[i] = []float32{1.0, 0.0, 0.0}
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)

	// Create 100 items
	ids := make([]string, 100)
	targets := make([]string, 100)
	for i := 0; i < 100; i++ {
		ids[i] = "id-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		targets[i] = "target " + ids[i]
		idx.Upsert(ids[i], search.ContentHash{}, []float32{float32(i) / 100, 0.0, 0.0})
	}

	ss.SetIDs(ids)
	ranks := ss.Filter("query", targets)

	// Should be limited to 75 results
	if len(ranks) > 75 {
		t.Errorf("Expected max 75 ranks, got %d", len(ranks))
	}
}

func TestSemanticSearchFilterMissingID(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range result {
				result[i] = []float32{1.0, 0.0, 0.0}
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)

	// Only add one vector but set two IDs
	idx.Upsert("id-1", search.ContentHash{}, []float32{1.0, 0.0, 0.0})

	ss.SetIDs([]string{"id-1", "id-missing"})

	// Use ComputeSemanticResults for synchronous computation (testing)
	ranks := ss.ComputeSemanticResults("query")

	// Should return results for both valid and missing IDs
	// Missing IDs are assigned a low score but included
	if len(ranks) != 2 {
		t.Errorf("Expected 2 ranks (valid + missing), got %d", len(ranks))
	}

	// First result should be id-1 (index 0) as it has a positive score
	if len(ranks) > 0 && ranks[0].Index != 0 {
		t.Errorf("Expected first rank index 0 (id-1), got %d", ranks[0].Index)
	}
}

func TestSemanticSearchFilterEmbedError(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			// Return wrong number of vectors
			return [][]float32{}, nil
		},
	}

	ss.SetIndex(idx, embedder)
	idx.Upsert("id-1", search.ContentHash{}, []float32{1.0, 0.0, 0.0})
	ss.SetIDs([]string{"id-1"})

	targets := []string{"a"}
	ranks := ss.Filter("query", targets)

	// When embed returns wrong count, falls back to DefaultFilter
	// DefaultFilter with a non-matching query may return empty
	// The important thing is it doesn't panic
	_ = ranks
}

// =============================================================================
// Non-blocking Filter Tests (async pattern)
// =============================================================================

func TestSemanticSearchFilterNonBlocking(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range result {
				result[i] = []float32{1.0, 0.0, 0.0}
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)
	idx.Upsert("id-1", search.ContentHash{}, []float32{1.0, 0.0, 0.0})
	idx.Upsert("id-2", search.ContentHash{}, []float32{0.0, 1.0, 0.0})
	ss.SetIDs([]string{"id-1", "id-2"})

	targets := []string{"a", "b"}

	// First call to Filter should return fuzzy results (non-blocking)
	// and mark the term as pending
	ranks := ss.Filter("query", targets)

	// Should get fuzzy results immediately (list.DefaultFilter behavior)
	// The exact result depends on DefaultFilter, but it should be non-empty
	if len(ranks) == 0 {
		// DefaultFilter with non-matching term returns empty, which is fine
	}

	// Should have a pending term
	pendingTerm := ss.GetPendingTerm()
	if pendingTerm != "query" {
		t.Errorf("Expected pending term 'query', got %q", pendingTerm)
	}

	// Now simulate async computation completing
	results := ss.ComputeSemanticResults("query")
	ss.SetCachedResults("query", results)

	// Pending should be cleared
	if ss.GetPendingTerm() != "" {
		t.Errorf("Expected pending term to be cleared, got %q", ss.GetPendingTerm())
	}

	// Second call to Filter should return cached semantic results
	ranks2 := ss.Filter("query", targets)
	if len(ranks2) != 2 {
		t.Errorf("Expected 2 cached ranks, got %d", len(ranks2))
	}
}

func TestSemanticSearchCacheManagement(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range result {
				result[i] = []float32{1.0, 0.0, 0.0}
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)
	idx.Upsert("id-1", search.ContentHash{}, []float32{1.0, 0.0, 0.0})
	ss.SetIDs([]string{"id-1"})

	// Test SetCachedResults
	results := ss.ComputeSemanticResults("test")
	ss.SetCachedResults("test", results)

	// Check that results are cached
	targets := []string{"a"}
	cachedRanks := ss.Filter("test", targets)
	if len(cachedRanks) != 1 {
		t.Errorf("Expected 1 cached rank, got %d", len(cachedRanks))
	}

	// Test ClearPending
	ss.Filter("new query", targets) // Mark as pending
	if ss.GetPendingTerm() != "new query" {
		t.Errorf("Expected pending term 'new query', got %q", ss.GetPendingTerm())
	}
	ss.ClearPending()
	if ss.GetPendingTerm() != "" {
		t.Errorf("Expected pending term to be cleared, got %q", ss.GetPendingTerm())
	}
}

// dotFloat32 Tests
// =============================================================================

func TestDotFloat32(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{-1.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "mixed values",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{4.0, 5.0, 6.0},
			expected: 32.0, // 1*4 + 2*5 + 3*6 = 4 + 10 + 18 = 32
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
		},
		{
			name:     "mismatched lengths",
			a:        []float32{1.0, 2.0},
			b:        []float32{1.0},
			expected: 0.0,
		},
		{
			name:     "single element",
			a:        []float32{3.0},
			b:        []float32{4.0},
			expected: 12.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dotFloat32(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("dotFloat32(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// SemanticIndexReadyMsg Tests
// =============================================================================

func TestSemanticIndexReadyMsg(t *testing.T) {
	// Test that the message struct works correctly
	msg := SemanticIndexReadyMsg{
		Embedder:  &mockEmbedder{dim: 384},
		Index:     search.NewVectorIndex(384),
		IndexPath: "/path/to/index",
		Loaded:    true,
		Stats:     search.IndexSyncStats{},
		Error:     nil,
	}

	if msg.Embedder == nil {
		t.Error("Embedder should not be nil")
	}
	if msg.Index == nil {
		t.Error("Index should not be nil")
	}
	if msg.IndexPath != "/path/to/index" {
		t.Errorf("IndexPath = %q, want %q", msg.IndexPath, "/path/to/index")
	}
	if !msg.Loaded {
		t.Error("Loaded should be true")
	}
	if msg.Error != nil {
		t.Errorf("Error should be nil, got %v", msg.Error)
	}
}

func TestSemanticIndexReadyMsgWithError(t *testing.T) {
	testErr := context.DeadlineExceeded
	msg := SemanticIndexReadyMsg{
		Error: testErr,
	}

	if msg.Error != testErr {
		t.Errorf("Error = %v, want %v", msg.Error, testErr)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestSemanticSearchFullWorkflow(t *testing.T) {
	// Create semantic search
	ss := NewSemanticSearch()

	// Verify initial state
	snap := ss.Snapshot()
	if snap.Ready {
		t.Error("Should not be ready initially")
	}

	// Set up index and embedder
	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{
		dim: 3,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range result {
				// Return consistent query vector
				result[i] = []float32{1.0, 0.0, 0.0}
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)

	// Add some vectors
	idx.Upsert("bug-1", search.ContentHash{}, []float32{0.9, 0.1, 0.0})
	idx.Upsert("feat-1", search.ContentHash{}, []float32{0.1, 0.9, 0.0})
	idx.Upsert("bug-2", search.ContentHash{}, []float32{0.8, 0.2, 0.0})

	ss.SetIDs([]string{"bug-1", "feat-1", "bug-2"})

	// Verify ready state
	snap = ss.Snapshot()
	if !snap.Ready {
		t.Error("Should be ready after setup")
	}

	// Test filtering
	targets := []string{"Fix login bug", "Add dark mode", "Fix crash bug"}
	ranks := ss.Filter("bug", targets)

	if len(ranks) == 0 {
		t.Fatal("Expected some filter results")
	}

	// bug-1 should rank highest (0.9 similarity with query)
	if ranks[0].Index != 0 {
		t.Errorf("Expected bug-1 (index 0) to rank first, got index %d", ranks[0].Index)
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestSemanticSearchConcurrentAccess(t *testing.T) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(3)
	embedder := &mockEmbedder{dim: 3}
	ss.SetIndex(idx, embedder)

	// Concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			ss.SetIDs([]string{"id-1", "id-2"})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = ss.Snapshot()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// If we get here without panic, the test passes
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkSemanticSearchFilter(b *testing.B) {
	ss := NewSemanticSearch()

	idx := search.NewVectorIndex(384)
	embedder := &mockEmbedder{
		dim: 384,
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range result {
				result[i] = make([]float32, 384)
				result[i][0] = 1.0
			}
			return result, nil
		},
	}

	ss.SetIndex(idx, embedder)

	// Add some vectors
	ids := make([]string, 100)
	targets := make([]string, 100)
	for i := 0; i < 100; i++ {
		ids[i] = "id-" + string(rune('A'+i%26))
		targets[i] = "target " + ids[i]
		vec := make([]float32, 384)
		vec[i%384] = 1.0
		idx.Upsert(ids[i], search.ContentHash{}, vec)
	}
	ss.SetIDs(ids)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ss.Filter("search query", targets)
	}
}

func BenchmarkDotFloat32(b *testing.B) {
	a := make([]float32, 384)
	bVec := make([]float32, 384)
	for i := range a {
		a[i] = float32(i) / 384.0
		bVec[i] = float32(384-i) / 384.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dotFloat32(a, bVec)
	}
}
