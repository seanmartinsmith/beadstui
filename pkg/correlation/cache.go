// Package correlation provides caching for history reports to avoid expensive git operations.
package correlation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CacheKey uniquely identifies a cached history report
type CacheKey struct {
	HeadSHA   string // Current git HEAD
	BeadsHash string // Hash of beads content
	Options   string // Serialized options
}

// String returns a string representation of the cache key
func (k CacheKey) String() string {
	return k.HeadSHA + ":" + k.BeadsHash + ":" + k.Options
}

// CacheEntry holds a cached report with metadata
type CacheEntry struct {
	Key        CacheKey
	Report     *HistoryReport
	CreatedAt  time.Time
	AccessedAt time.Time
}

// HistoryCache provides thread-safe caching of history reports with LRU eviction
type HistoryCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	order    []string // LRU order (oldest first)
	maxAge   time.Duration
	maxSize  int
	repoPath string
}

// DefaultCacheMaxAge is the default maximum age for cache entries
const DefaultCacheMaxAge = 5 * time.Minute

// DefaultCacheMaxSize is the default maximum number of cache entries
const DefaultCacheMaxSize = 10

// NewHistoryCache creates a new cache with default settings
func NewHistoryCache(repoPath string) *HistoryCache {
	return &HistoryCache{
		entries:  make(map[string]*CacheEntry),
		order:    make([]string, 0, DefaultCacheMaxSize),
		maxAge:   DefaultCacheMaxAge,
		maxSize:  DefaultCacheMaxSize,
		repoPath: repoPath,
	}
}

// NewHistoryCacheWithOptions creates a cache with custom settings
func NewHistoryCacheWithOptions(repoPath string, maxAge time.Duration, maxSize int) *HistoryCache {
	if maxAge <= 0 {
		maxAge = DefaultCacheMaxAge
	}
	if maxSize <= 0 {
		maxSize = DefaultCacheMaxSize
	}
	return &HistoryCache{
		entries:  make(map[string]*CacheEntry),
		order:    make([]string, 0, maxSize),
		maxAge:   maxAge,
		maxSize:  maxSize,
		repoPath: repoPath,
	}
}

// Get retrieves a cached report if available and valid
func (c *HistoryCache) Get(key CacheKey) (*HistoryReport, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyStr := key.String()
	entry, ok := c.entries[keyStr]
	if !ok {
		return nil, false
	}

	// Check if entry is expired
	if time.Since(entry.CreatedAt) > c.maxAge {
		c.removeEntryLocked(keyStr)
		return nil, false
	}

	// Update access time and move to end of LRU order
	entry.AccessedAt = time.Now()
	c.moveToEndLocked(keyStr)

	return entry.Report, true
}

// Put stores a report in the cache
func (c *HistoryCache) Put(key CacheKey, report *HistoryReport) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyStr := key.String()
	now := time.Now()

	// If key already exists, update it
	if _, exists := c.entries[keyStr]; exists {
		c.entries[keyStr].Report = report
		c.entries[keyStr].CreatedAt = now
		c.entries[keyStr].AccessedAt = now
		c.moveToEndLocked(keyStr)
		return
	}

	// Evict oldest entry if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldestLocked()
	}

	// Add new entry
	c.entries[keyStr] = &CacheEntry{
		Key:        key,
		Report:     report,
		CreatedAt:  now,
		AccessedAt: now,
	}
	c.order = append(c.order, keyStr)
}

// Invalidate removes all entries (called when HEAD or beads change)
func (c *HistoryCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.order = make([]string, 0, c.maxSize)
}

// InvalidateForHead removes entries that don't match the current HEAD
func (c *HistoryCache) InvalidateForHead(currentHead string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toRemove []string
	for keyStr, entry := range c.entries {
		if entry.Key.HeadSHA != currentHead {
			toRemove = append(toRemove, keyStr)
		}
	}

	for _, keyStr := range toRemove {
		c.removeEntryLocked(keyStr)
	}
}

// Size returns the number of cached entries
func (c *HistoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stats returns cache statistics
func (c *HistoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxSize,
		MaxAge:  c.maxAge,
	}

	var oldestTime, newestTime time.Time
	for _, entry := range c.entries {
		if oldestTime.IsZero() || entry.CreatedAt.Before(oldestTime) {
			oldestTime = entry.CreatedAt
		}
		if newestTime.IsZero() || entry.CreatedAt.After(newestTime) {
			newestTime = entry.CreatedAt
		}
	}

	if !oldestTime.IsZero() {
		stats.OldestEntry = &oldestTime
	}
	if !newestTime.IsZero() {
		stats.NewestEntry = &newestTime
	}

	return stats
}

// CacheStats provides information about cache state
type CacheStats struct {
	Size        int
	MaxSize     int
	MaxAge      time.Duration
	OldestEntry *time.Time
	NewestEntry *time.Time
}

// removeEntryLocked removes an entry (caller must hold lock)
func (c *HistoryCache) removeEntryLocked(keyStr string) {
	delete(c.entries, keyStr)

	// Remove from order slice
	for i, k := range c.order {
		if k == keyStr {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// moveToEndLocked moves an entry to the end of the LRU order (caller must hold lock)
func (c *HistoryCache) moveToEndLocked(keyStr string) {
	// Find and remove from current position
	for i, k := range c.order {
		if k == keyStr {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	// Add to end
	c.order = append(c.order, keyStr)
}

// evictOldestLocked removes the oldest entry (caller must hold lock)
func (c *HistoryCache) evictOldestLocked() {
	if len(c.order) == 0 {
		return
	}

	oldestKey := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, oldestKey)
}

// BuildCacheKey creates a cache key for the given parameters
func BuildCacheKey(repoPath string, beads []BeadInfo, opts CorrelatorOptions) (CacheKey, error) {
	headSHA, err := getGitHead(repoPath)
	if err != nil {
		return CacheKey{}, err
	}

	beadsHash := hashBeads(beads)
	optsHash := hashOptions(opts)

	return CacheKey{
		HeadSHA:   headSHA,
		BeadsHash: beadsHash,
		Options:   optsHash,
	}, nil
}

// getGitHead returns the current HEAD SHA
func getGitHead(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hashBeads creates a hash of bead IDs and statuses
func hashBeads(beads []BeadInfo) string {
	h := sha256.New()
	for _, b := range beads {
		h.Write([]byte(b.ID))
		h.Write([]byte(b.Status))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// hashOptions creates a hash of correlator options
func hashOptions(opts CorrelatorOptions) string {
	// Serialize options to JSON for consistent hashing
	data, err := json.Marshal(struct {
		BeadID string
		Since  *time.Time
		Until  *time.Time
		Limit  int
	}{
		BeadID: opts.BeadID,
		Since:  opts.Since,
		Until:  opts.Until,
		Limit:  opts.Limit,
	})
	if err != nil {
		return "default"
	}

	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// CachedCorrelator wraps a Correlator with caching support
type CachedCorrelator struct {
	correlator *Correlator
	cache      *HistoryCache
	hits       int64 // Cache hit count (for stats)
	misses     int64 // Cache miss count (for stats)
	mu         sync.Mutex
}

// NewCachedCorrelator creates a correlator with caching enabled
func NewCachedCorrelator(repoPath string) *CachedCorrelator {
	return &CachedCorrelator{
		correlator: NewCorrelator(repoPath),
		cache:      NewHistoryCache(repoPath),
	}
}

// NewCachedCorrelatorWithOptions creates a correlator with custom cache settings
func NewCachedCorrelatorWithOptions(repoPath string, maxAge time.Duration, maxSize int) *CachedCorrelator {
	return &CachedCorrelator{
		correlator: NewCorrelator(repoPath),
		cache:      NewHistoryCacheWithOptions(repoPath, maxAge, maxSize),
	}
}

// GenerateReport generates a history report, using cache when possible
func (c *CachedCorrelator) GenerateReport(beads []BeadInfo, opts CorrelatorOptions) (*HistoryReport, error) {
	// Build cache key
	key, err := BuildCacheKey(c.cache.repoPath, beads, opts)
	if err != nil {
		// If we can't build a cache key, fall back to uncached
		return c.correlator.GenerateReport(beads, opts)
	}

	// Check cache
	if report, ok := c.cache.Get(key); ok {
		c.mu.Lock()
		c.hits++
		c.mu.Unlock()
		return report, nil
	}

	// Cache miss - generate report
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()

	report, err := c.correlator.GenerateReport(beads, opts)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.cache.Put(key, report)

	return report, nil
}

// InvalidateCache clears all cached entries
func (c *CachedCorrelator) InvalidateCache() {
	c.cache.Invalidate()
}

// CacheStats returns cache statistics
func (c *CachedCorrelator) CacheStats() CachedCorrelatorStats {
	c.mu.Lock()
	hits := c.hits
	misses := c.misses
	c.mu.Unlock()

	cacheStats := c.cache.Stats()

	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return CachedCorrelatorStats{
		Hits:      hits,
		Misses:    misses,
		HitRate:   hitRate,
		CacheSize: cacheStats.Size,
		MaxSize:   cacheStats.MaxSize,
		MaxAge:    cacheStats.MaxAge,
	}
}

// CachedCorrelatorStats provides statistics about cache performance
type CachedCorrelatorStats struct {
	Hits      int64
	Misses    int64
	HitRate   float64
	CacheSize int
	MaxSize   int
	MaxAge    time.Duration
}
