package cass

import (
	"sync"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	c := NewCache()

	if c.maxSize != DefaultResultCacheSize {
		t.Errorf("maxSize = %d, want %d", c.maxSize, DefaultResultCacheSize)
	}
	if c.ttl != DefaultResultCacheTTL {
		t.Errorf("ttl = %v, want %v", c.ttl, DefaultResultCacheTTL)
	}
	if c.Size() != 0 {
		t.Errorf("Size() = %d, want 0", c.Size())
	}
}

func TestNewCacheWithOptions(t *testing.T) {
	c := NewCacheWithOptions(
		WithResultCacheSize(50),
		WithResultCacheTTL(5*time.Minute),
	)

	if c.maxSize != 50 {
		t.Errorf("maxSize = %d, want 50", c.maxSize)
	}
	if c.ttl != 5*time.Minute {
		t.Errorf("ttl = %v, want 5m", c.ttl)
	}
}

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache()

	hint := &CorrelationHint{
		BeadID:      "bv-test1",
		QueryUsed:   "test query",
		ResultCount: 5,
	}

	c.Set("bv-test1", hint)

	got := c.Get("bv-test1")
	if got == nil {
		t.Fatal("Get() returned nil, want hint")
	}
	if got.BeadID != hint.BeadID {
		t.Errorf("BeadID = %q, want %q", got.BeadID, hint.BeadID)
	}
	if got.QueryUsed != hint.QueryUsed {
		t.Errorf("QueryUsed = %q, want %q", got.QueryUsed, hint.QueryUsed)
	}
}

func TestCache_GetMiss(t *testing.T) {
	c := NewCache()

	got := c.Get("nonexistent")
	if got != nil {
		t.Errorf("Get(nonexistent) = %v, want nil", got)
	}

	stats := c.Stats()
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
}

func TestCache_GetHit(t *testing.T) {
	c := NewCache()

	c.Set("bv-test", &CorrelationHint{BeadID: "bv-test"})
	c.Get("bv-test")
	c.Get("bv-test")

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Errorf("Hits = %d, want 2", stats.Hits)
	}
}

func TestCache_SetUpdate(t *testing.T) {
	c := NewCache()

	c.Set("bv-test", &CorrelationHint{QueryUsed: "query1"})
	c.Set("bv-test", &CorrelationHint{QueryUsed: "query2"})

	got := c.Get("bv-test")
	if got.QueryUsed != "query2" {
		t.Errorf("QueryUsed = %q, want \"query2\"", got.QueryUsed)
	}

	if c.Size() != 1 {
		t.Errorf("Size() = %d, want 1 (update shouldn't add new entry)", c.Size())
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	currentTime := time.Now()

	c := NewCacheWithOptions(WithResultCacheTTL(100 * time.Millisecond))
	c.now = func() time.Time { return currentTime }

	c.Set("bv-test", &CorrelationHint{BeadID: "bv-test"})

	// Should hit
	got := c.Get("bv-test")
	if got == nil {
		t.Error("Get() before expiry returned nil")
	}

	// Advance time past TTL
	currentTime = currentTime.Add(200 * time.Millisecond)

	// Should miss (expired)
	got = c.Get("bv-test")
	if got != nil {
		t.Error("Get() after expiry should return nil")
	}

	stats := c.Stats()
	if stats.Evictions != 1 {
		t.Errorf("Evictions = %d, want 1", stats.Evictions)
	}
}

func TestCache_LRUEviction(t *testing.T) {
	c := NewCacheWithOptions(WithResultCacheSize(3))

	// Fill cache
	c.Set("bv-1", &CorrelationHint{BeadID: "bv-1"})
	c.Set("bv-2", &CorrelationHint{BeadID: "bv-2"})
	c.Set("bv-3", &CorrelationHint{BeadID: "bv-3"})

	// Access bv-1 to make it recently used
	c.Get("bv-1")

	// Add bv-4, should evict bv-2 (least recently used)
	c.Set("bv-4", &CorrelationHint{BeadID: "bv-4"})

	// bv-2 should be evicted
	if c.Get("bv-2") != nil {
		t.Error("bv-2 should have been evicted")
	}

	// bv-1, bv-3, bv-4 should still exist
	if c.Get("bv-1") == nil {
		t.Error("bv-1 should still exist")
	}
	if c.Get("bv-3") == nil {
		t.Error("bv-3 should still exist")
	}
	if c.Get("bv-4") == nil {
		t.Error("bv-4 should still exist")
	}
}

func TestCache_LRUOrder(t *testing.T) {
	c := NewCacheWithOptions(WithResultCacheSize(3))

	c.Set("bv-1", &CorrelationHint{BeadID: "bv-1"})
	c.Set("bv-2", &CorrelationHint{BeadID: "bv-2"})
	c.Set("bv-3", &CorrelationHint{BeadID: "bv-3"})

	// Access order: bv-3, bv-1, bv-2
	// LRU order (oldest first): bv-3, bv-1, bv-2
	c.Get("bv-3") // Moves bv-3 to back
	c.Get("bv-1") // Moves bv-1 to back
	c.Get("bv-2") // Moves bv-2 to back

	// Add new entry - should evict bv-3 (now oldest)
	c.Set("bv-4", &CorrelationHint{BeadID: "bv-4"})

	if c.Get("bv-3") != nil {
		t.Error("bv-3 should have been evicted (LRU)")
	}
}

func TestCache_Invalidate(t *testing.T) {
	c := NewCache()

	c.Set("bv-test", &CorrelationHint{BeadID: "bv-test"})
	c.Invalidate("bv-test")

	if c.Get("bv-test") != nil {
		t.Error("Get() after Invalidate() should return nil")
	}
	if c.Size() != 0 {
		t.Errorf("Size() = %d, want 0", c.Size())
	}
}

func TestCache_InvalidateNonexistent(t *testing.T) {
	c := NewCache()

	// Should not panic
	c.Invalidate("nonexistent")
}

func TestCache_Clear(t *testing.T) {
	c := NewCache()

	c.Set("bv-1", &CorrelationHint{BeadID: "bv-1"})
	c.Set("bv-2", &CorrelationHint{BeadID: "bv-2"})
	c.Set("bv-3", &CorrelationHint{BeadID: "bv-3"})

	c.Clear()

	if c.Size() != 0 {
		t.Errorf("Size() after Clear() = %d, want 0", c.Size())
	}
	if c.Get("bv-1") != nil {
		t.Error("Get() after Clear() should return nil")
	}
}

func TestCache_Stats(t *testing.T) {
	c := NewCacheWithOptions(WithResultCacheSize(50), WithResultCacheTTL(5*time.Minute))

	c.Set("bv-1", &CorrelationHint{BeadID: "bv-1"})
	c.Set("bv-2", &CorrelationHint{BeadID: "bv-2"})
	c.Get("bv-1") // Hit
	c.Get("bv-2") // Hit
	c.Get("bv-3") // Miss

	stats := c.Stats()

	if stats.Size != 2 {
		t.Errorf("Size = %d, want 2", stats.Size)
	}
	if stats.MaxSize != 50 {
		t.Errorf("MaxSize = %d, want 50", stats.MaxSize)
	}
	if stats.Hits != 2 {
		t.Errorf("Hits = %d, want 2", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.TTL != 5*time.Minute {
		t.Errorf("TTL = %v, want 5m", stats.TTL)
	}
}

func TestCache_EvictExpiredFirst(t *testing.T) {
	currentTime := time.Now()

	c := NewCacheWithOptions(WithResultCacheSize(3), WithResultCacheTTL(100*time.Millisecond))
	c.now = func() time.Time { return currentTime }

	// Fill cache
	c.Set("bv-1", &CorrelationHint{BeadID: "bv-1"})
	c.Set("bv-2", &CorrelationHint{BeadID: "bv-2"})
	c.Set("bv-3", &CorrelationHint{BeadID: "bv-3"})

	// Expire bv-1 and bv-2
	currentTime = currentTime.Add(150 * time.Millisecond)

	// Set bv-2 again (refreshes TTL)
	c.Set("bv-2", &CorrelationHint{BeadID: "bv-2-updated"})

	// Add bv-4 - should evict expired entries first (bv-1, bv-3)
	c.Set("bv-4", &CorrelationHint{BeadID: "bv-4"})

	// Only bv-2 (refreshed) and bv-4 should exist
	if c.Get("bv-1") != nil {
		t.Error("bv-1 should have been evicted (expired)")
	}
	if c.Get("bv-3") != nil {
		t.Error("bv-3 should have been evicted (expired)")
	}
	if c.Get("bv-2") == nil {
		t.Error("bv-2 should still exist (refreshed)")
	}
	if c.Get("bv-4") == nil {
		t.Error("bv-4 should exist")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache()

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "bv-" + string(rune('a'+id%26))
			c.Set(key, &CorrelationHint{BeadID: key})
		}(i)
	}

	// Readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "bv-" + string(rune('a'+id%26))
			c.Get(key)
		}(i)
	}

	// Invalidators
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "bv-" + string(rune('a'+id%26))
			c.Invalidate(key)
		}(i)
	}

	wg.Wait()
	// Test passes if no panics or race conditions detected by -race flag
}

func TestCache_ConcurrentSetSameKey(t *testing.T) {
	c := NewCache()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			c.Set("shared-key", &CorrelationHint{ResultCount: val})
		}(i)
	}
	wg.Wait()

	// Should have exactly one entry
	if c.Size() != 1 {
		t.Errorf("Size() = %d, want 1", c.Size())
	}

	got := c.Get("shared-key")
	if got == nil {
		t.Error("Get() returned nil")
	}
}

func TestCache_ZeroOptions(t *testing.T) {
	// Zero/negative values should use defaults
	c := NewCacheWithOptions(
		WithResultCacheSize(0),
		WithResultCacheTTL(0),
	)

	if c.maxSize != DefaultResultCacheSize {
		t.Errorf("maxSize = %d, want %d (default)", c.maxSize, DefaultResultCacheSize)
	}
	if c.ttl != DefaultResultCacheTTL {
		t.Errorf("ttl = %v, want %v (default)", c.ttl, DefaultResultCacheTTL)
	}
}

func TestCache_Len(t *testing.T) {
	c := NewCache()

	c.Set("bv-1", &CorrelationHint{})
	c.Set("bv-2", &CorrelationHint{})

	if c.Len() != 2 {
		t.Errorf("Len() = %d, want 2", c.Len())
	}
	if c.Len() != c.Size() {
		t.Error("Len() and Size() should return same value")
	}
}

func TestCache_WithResults(t *testing.T) {
	c := NewCache()

	results := []SearchResult{
		{SourcePath: "/session1.json", Score: 0.9},
		{SourcePath: "/session2.json", Score: 0.7},
	}

	hint := &CorrelationHint{
		BeadID:      "bv-test",
		Results:     results,
		ResultCount: 2,
	}

	c.Set("bv-test", hint)

	got := c.Get("bv-test")
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if len(got.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2", len(got.Results))
	}
	if got.Results[0].Score != 0.9 {
		t.Errorf("Results[0].Score = %f, want 0.9", got.Results[0].Score)
	}
}

// BenchmarkCache_Get benchmarks cache hit performance.
func BenchmarkCache_Get(b *testing.B) {
	c := NewCache()
	c.Set("bv-test", &CorrelationHint{BeadID: "bv-test"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("bv-test")
	}
}

// BenchmarkCache_Set benchmarks cache set performance.
func BenchmarkCache_Set(b *testing.B) {
	c := NewCache()
	hint := &CorrelationHint{BeadID: "bv-test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("bv-test", hint)
	}
}

// BenchmarkCache_GetMiss benchmarks cache miss performance.
func BenchmarkCache_GetMiss(b *testing.B) {
	c := NewCache()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("nonexistent")
	}
}
