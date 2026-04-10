package correlation

import (
	"testing"
	"time"
)

func TestCacheKey_String(t *testing.T) {
	key := CacheKey{
		HeadSHA:   "abc123",
		BeadsHash: "def456",
		Options:   "opt789",
	}

	expected := "abc123:def456:opt789"
	if got := key.String(); got != expected {
		t.Errorf("CacheKey.String() = %q, want %q", got, expected)
	}
}

func TestNewHistoryCache(t *testing.T) {
	cache := NewHistoryCache("/tmp/test")

	if cache.maxAge != DefaultCacheMaxAge {
		t.Errorf("maxAge = %v, want %v", cache.maxAge, DefaultCacheMaxAge)
	}
	if cache.maxSize != DefaultCacheMaxSize {
		t.Errorf("maxSize = %d, want %d", cache.maxSize, DefaultCacheMaxSize)
	}
	if cache.Size() != 0 {
		t.Errorf("Size() = %d, want 0", cache.Size())
	}
}

func TestNewHistoryCacheWithOptions(t *testing.T) {
	maxAge := 10 * time.Minute
	maxSize := 5

	cache := NewHistoryCacheWithOptions("/tmp/test", maxAge, maxSize)

	if cache.maxAge != maxAge {
		t.Errorf("maxAge = %v, want %v", cache.maxAge, maxAge)
	}
	if cache.maxSize != maxSize {
		t.Errorf("maxSize = %d, want %d", cache.maxSize, maxSize)
	}
}

func TestNewHistoryCacheWithOptions_DefaultsOnInvalid(t *testing.T) {
	cache := NewHistoryCacheWithOptions("/tmp/test", 0, 0)

	if cache.maxAge != DefaultCacheMaxAge {
		t.Errorf("maxAge with 0 = %v, want %v", cache.maxAge, DefaultCacheMaxAge)
	}
	if cache.maxSize != DefaultCacheMaxSize {
		t.Errorf("maxSize with 0 = %d, want %d", cache.maxSize, DefaultCacheMaxSize)
	}
}

func TestHistoryCache_PutAndGet(t *testing.T) {
	cache := NewHistoryCache("/tmp/test")

	key := CacheKey{HeadSHA: "abc", BeadsHash: "def", Options: "ghi"}
	report := &HistoryReport{
		Stats: HistoryStats{TotalBeads: 5},
	}

	// Get on empty cache should miss
	if _, ok := cache.Get(key); ok {
		t.Error("Get on empty cache should return false")
	}

	// Put and Get
	cache.Put(key, report)

	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("Get after Put should return true")
	}
	if got.Stats.TotalBeads != 5 {
		t.Errorf("Got report with TotalBeads = %d, want 5", got.Stats.TotalBeads)
	}

	if cache.Size() != 1 {
		t.Errorf("Size() = %d, want 1", cache.Size())
	}
}

func TestHistoryCache_PutUpdate(t *testing.T) {
	cache := NewHistoryCache("/tmp/test")

	key := CacheKey{HeadSHA: "abc", BeadsHash: "def", Options: "ghi"}
	report1 := &HistoryReport{Stats: HistoryStats{TotalBeads: 5}}
	report2 := &HistoryReport{Stats: HistoryStats{TotalBeads: 10}}

	cache.Put(key, report1)
	cache.Put(key, report2) // Update same key

	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("Get after update should return true")
	}
	if got.Stats.TotalBeads != 10 {
		t.Errorf("Got TotalBeads = %d, want 10 (updated value)", got.Stats.TotalBeads)
	}

	// Size should still be 1 (not 2)
	if cache.Size() != 1 {
		t.Errorf("Size() after update = %d, want 1", cache.Size())
	}
}

func TestHistoryCache_LRUEviction(t *testing.T) {
	cache := NewHistoryCacheWithOptions("/tmp/test", 5*time.Minute, 3)

	// Add 3 entries
	for i := 0; i < 3; i++ {
		key := CacheKey{HeadSHA: string(rune('a' + i)), BeadsHash: "b", Options: "c"}
		cache.Put(key, &HistoryReport{Stats: HistoryStats{TotalBeads: i}})
	}

	if cache.Size() != 3 {
		t.Errorf("Size() = %d, want 3", cache.Size())
	}

	// Add 4th entry - should evict oldest (key 'a')
	key4 := CacheKey{HeadSHA: "d", BeadsHash: "b", Options: "c"}
	cache.Put(key4, &HistoryReport{Stats: HistoryStats{TotalBeads: 99}})

	if cache.Size() != 3 {
		t.Errorf("Size() after eviction = %d, want 3", cache.Size())
	}

	// First key should be evicted
	keyFirst := CacheKey{HeadSHA: "a", BeadsHash: "b", Options: "c"}
	if _, ok := cache.Get(keyFirst); ok {
		t.Error("First entry should have been evicted")
	}

	// Fourth key should exist
	if _, ok := cache.Get(key4); !ok {
		t.Error("Fourth entry should exist")
	}
}

func TestHistoryCache_LRUAccessOrder(t *testing.T) {
	cache := NewHistoryCacheWithOptions("/tmp/test", 5*time.Minute, 3)

	// Add 3 entries
	key1 := CacheKey{HeadSHA: "a", BeadsHash: "b", Options: "c"}
	key2 := CacheKey{HeadSHA: "b", BeadsHash: "b", Options: "c"}
	key3 := CacheKey{HeadSHA: "c", BeadsHash: "b", Options: "c"}

	cache.Put(key1, &HistoryReport{})
	cache.Put(key2, &HistoryReport{})
	cache.Put(key3, &HistoryReport{})

	// Access key1 to move it to end
	cache.Get(key1)

	// Add 4th entry - should evict key2 (oldest accessed)
	key4 := CacheKey{HeadSHA: "d", BeadsHash: "b", Options: "c"}
	cache.Put(key4, &HistoryReport{})

	// key2 should be evicted (was oldest accessed)
	if _, ok := cache.Get(key2); ok {
		t.Error("key2 should have been evicted (was oldest accessed)")
	}

	// key1 should still exist (was recently accessed)
	if _, ok := cache.Get(key1); !ok {
		t.Error("key1 should still exist (was recently accessed)")
	}
}

func TestHistoryCache_Expiration(t *testing.T) {
	// Use very short maxAge for testing
	cache := NewHistoryCacheWithOptions("/tmp/test", 10*time.Millisecond, 10)

	key := CacheKey{HeadSHA: "abc", BeadsHash: "def", Options: "ghi"}
	cache.Put(key, &HistoryReport{})

	// Should hit immediately
	if _, ok := cache.Get(key); !ok {
		t.Error("Get immediately after Put should hit")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should miss after expiration
	if _, ok := cache.Get(key); ok {
		t.Error("Get after expiration should miss")
	}

	// Entry should be removed
	if cache.Size() != 0 {
		t.Errorf("Size() after expiration get = %d, want 0", cache.Size())
	}
}

func TestHistoryCache_Invalidate(t *testing.T) {
	cache := NewHistoryCache("/tmp/test")

	// Add some entries
	for i := 0; i < 5; i++ {
		key := CacheKey{HeadSHA: string(rune('a' + i)), BeadsHash: "b", Options: "c"}
		cache.Put(key, &HistoryReport{})
	}

	if cache.Size() != 5 {
		t.Errorf("Size() before invalidate = %d, want 5", cache.Size())
	}

	cache.Invalidate()

	if cache.Size() != 0 {
		t.Errorf("Size() after invalidate = %d, want 0", cache.Size())
	}
}

func TestHistoryCache_InvalidateForHead(t *testing.T) {
	cache := NewHistoryCache("/tmp/test")

	// Add entries with different HEAD SHAs
	key1 := CacheKey{HeadSHA: "head1", BeadsHash: "b", Options: "c"}
	key2 := CacheKey{HeadSHA: "head1", BeadsHash: "d", Options: "c"}
	key3 := CacheKey{HeadSHA: "head2", BeadsHash: "b", Options: "c"}

	cache.Put(key1, &HistoryReport{})
	cache.Put(key2, &HistoryReport{})
	cache.Put(key3, &HistoryReport{})

	// Invalidate for head2
	cache.InvalidateForHead("head1")

	// head2 entry should be removed
	if _, ok := cache.Get(key3); ok {
		t.Error("head2 entry should have been invalidated")
	}

	// head1 entries should remain
	if _, ok := cache.Get(key1); !ok {
		t.Error("head1 entry 1 should remain")
	}
	if _, ok := cache.Get(key2); !ok {
		t.Error("head1 entry 2 should remain")
	}
}

func TestHistoryCache_Stats(t *testing.T) {
	cache := NewHistoryCacheWithOptions("/tmp/test", 5*time.Minute, 10)

	// Initially empty
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Stats.Size = %d, want 0", stats.Size)
	}
	if stats.MaxSize != 10 {
		t.Errorf("Stats.MaxSize = %d, want 10", stats.MaxSize)
	}
	if stats.OldestEntry != nil {
		t.Error("Stats.OldestEntry should be nil when empty")
	}

	// Add entries
	cache.Put(CacheKey{HeadSHA: "a"}, &HistoryReport{})
	time.Sleep(time.Millisecond)
	cache.Put(CacheKey{HeadSHA: "b"}, &HistoryReport{})

	stats = cache.Stats()
	if stats.Size != 2 {
		t.Errorf("Stats.Size = %d, want 2", stats.Size)
	}
	if stats.OldestEntry == nil {
		t.Error("Stats.OldestEntry should not be nil")
	}
	if stats.NewestEntry == nil {
		t.Error("Stats.NewestEntry should not be nil")
	}
}

func TestHashBeads(t *testing.T) {
	beads1 := []BeadInfo{
		{ID: "bv-1", Status: "open"},
		{ID: "bv-2", Status: "closed"},
	}
	beads2 := []BeadInfo{
		{ID: "bv-1", Status: "open"},
		{ID: "bv-2", Status: "closed"},
	}
	beads3 := []BeadInfo{
		{ID: "bv-1", Status: "closed"}, // Different status
		{ID: "bv-2", Status: "closed"},
	}

	hash1 := hashBeads(beads1)
	hash2 := hashBeads(beads2)
	hash3 := hashBeads(beads3)

	// Same input should produce same hash
	if hash1 != hash2 {
		t.Errorf("Same beads should produce same hash: %s != %s", hash1, hash2)
	}

	// Different input should produce different hash
	if hash1 == hash3 {
		t.Error("Different beads should produce different hash")
	}

	// Hash should be 12 chars
	if len(hash1) != 12 {
		t.Errorf("Hash length = %d, want 12", len(hash1))
	}
}

func TestHashOptions(t *testing.T) {
	now := time.Now()
	opts1 := CorrelatorOptions{BeadID: "bv-1", Limit: 100}
	opts2 := CorrelatorOptions{BeadID: "bv-1", Limit: 100}
	opts3 := CorrelatorOptions{BeadID: "bv-2", Limit: 100}
	opts4 := CorrelatorOptions{BeadID: "bv-1", Since: &now}

	hash1 := hashOptions(opts1)
	hash2 := hashOptions(opts2)
	hash3 := hashOptions(opts3)
	hash4 := hashOptions(opts4)

	// Same options should produce same hash
	if hash1 != hash2 {
		t.Errorf("Same options should produce same hash: %s != %s", hash1, hash2)
	}

	// Different options should produce different hash
	if hash1 == hash3 {
		t.Error("Different BeadID should produce different hash")
	}
	if hash1 == hash4 {
		t.Error("Different Since should produce different hash")
	}
}

func TestCachedCorrelator_CacheHitAndMiss(t *testing.T) {
	// Skip if not in a git repo
	if _, err := getGitHead("."); err != nil {
		t.Skip("Not in a git repository")
	}

	correlator := NewCachedCorrelator(".")
	beads := []BeadInfo{{ID: "test-1", Status: "open"}}
	opts := CorrelatorOptions{Limit: 10}

	// First call should miss
	_, err := correlator.GenerateReport(beads, opts)
	if err != nil {
		t.Fatalf("First GenerateReport failed: %v", err)
	}

	stats := correlator.CacheStats()
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.Hits != 0 {
		t.Errorf("Hits = %d, want 0", stats.Hits)
	}

	// Second call with same params should hit
	_, err = correlator.GenerateReport(beads, opts)
	if err != nil {
		t.Fatalf("Second GenerateReport failed: %v", err)
	}

	stats = correlator.CacheStats()
	if stats.Misses != 1 {
		t.Errorf("Misses after hit = %d, want 1", stats.Misses)
	}
	if stats.Hits != 1 {
		t.Errorf("Hits after hit = %d, want 1", stats.Hits)
	}
	if stats.HitRate != 0.5 {
		t.Errorf("HitRate = %f, want 0.5", stats.HitRate)
	}
}

func TestCachedCorrelator_DifferentOptionsMiss(t *testing.T) {
	// Skip if not in a git repo
	if _, err := getGitHead("."); err != nil {
		t.Skip("Not in a git repository")
	}

	correlator := NewCachedCorrelator(".")
	beads := []BeadInfo{{ID: "test-1", Status: "open"}}

	// First call
	_, err := correlator.GenerateReport(beads, CorrelatorOptions{Limit: 10})
	if err != nil {
		t.Fatalf("First GenerateReport failed: %v", err)
	}

	// Second call with different options should miss
	_, err = correlator.GenerateReport(beads, CorrelatorOptions{Limit: 20})
	if err != nil {
		t.Fatalf("Second GenerateReport failed: %v", err)
	}

	stats := correlator.CacheStats()
	if stats.Misses != 2 {
		t.Errorf("Misses = %d, want 2 (different options should miss)", stats.Misses)
	}
}

func TestCachedCorrelator_InvalidateCache(t *testing.T) {
	// Skip if not in a git repo
	if _, err := getGitHead("."); err != nil {
		t.Skip("Not in a git repository")
	}

	correlator := NewCachedCorrelator(".")
	beads := []BeadInfo{{ID: "test-1", Status: "open"}}
	opts := CorrelatorOptions{Limit: 10}

	// Populate cache
	_, _ = correlator.GenerateReport(beads, opts)

	stats := correlator.CacheStats()
	if stats.CacheSize != 1 {
		t.Errorf("CacheSize = %d, want 1", stats.CacheSize)
	}

	// Invalidate
	correlator.InvalidateCache()

	stats = correlator.CacheStats()
	if stats.CacheSize != 0 {
		t.Errorf("CacheSize after invalidate = %d, want 0", stats.CacheSize)
	}
}

func TestNewCachedCorrelatorWithOptions(t *testing.T) {
	correlator := NewCachedCorrelatorWithOptions("/tmp/test", 10*time.Minute, 20)

	if correlator.cache.maxAge != 10*time.Minute {
		t.Errorf("maxAge = %v, want 10m", correlator.cache.maxAge)
	}
	if correlator.cache.maxSize != 20 {
		t.Errorf("maxSize = %d, want 20", correlator.cache.maxSize)
	}
}

func TestBuildCacheKey_Error(t *testing.T) {
	// Should fail without a git repo
	_, err := BuildCacheKey("/nonexistent/path", nil, CorrelatorOptions{})
	if err == nil {
		t.Error("BuildCacheKey should fail for invalid repo path")
	}
}

func TestCacheKey_Empty(t *testing.T) {
	key := CacheKey{}
	if key.String() != "::" {
		t.Errorf("Empty CacheKey.String() = %q, want '::'", key.String())
	}
}

func TestHistoryCache_GetNonexistent(t *testing.T) {
	cache := NewHistoryCache("/tmp/test")
	key := CacheKey{HeadSHA: "nonexistent", BeadsHash: "hash", Options: "opts"}

	_, ok := cache.Get(key)
	if ok {
		t.Error("Get should return false for nonexistent key")
	}
}

func TestHistoryCache_RemoveEntryOrdering(t *testing.T) {
	cache := NewHistoryCacheWithOptions("/tmp/test", 5*time.Minute, 5)

	// Add multiple entries
	for i := 0; i < 3; i++ {
		key := CacheKey{HeadSHA: string(rune('a' + i))}
		cache.Put(key, &HistoryReport{})
	}

	// Verify order
	if len(cache.order) != 3 {
		t.Errorf("order length = %d, want 3", len(cache.order))
	}

	// Remove middle entry
	cache.Invalidate()

	if len(cache.order) != 0 {
		t.Errorf("order after invalidate = %d, want 0", len(cache.order))
	}
}
