package cass

import (
	"container/list"
	"sync"
	"time"
)

// DefaultResultCacheSize is the default maximum number of cache entries.
const DefaultResultCacheSize = 100

// DefaultResultCacheTTL is the default time-to-live for cache entries.
const DefaultResultCacheTTL = 10 * time.Minute

// CorrelationHint represents cached correlation data for a bead.
// This will be extended when the correlation engine is implemented.
type CorrelationHint struct {
	BeadID      string         // The bead this hint is for
	Results     []SearchResult // Correlated search results
	QueryUsed   string         // The query that produced these results
	ResultCount int            // Total results (may be > len(Results) if truncated)
}

// CacheEntry wraps a cached hint with timing metadata.
type CacheEntry struct {
	Hint      *CorrelationHint
	Key       string
	CachedAt  time.Time
	ExpiresAt time.Time
}

// CacheStats contains cache statistics for monitoring.
type CacheStats struct {
	Size      int           // Current number of entries
	MaxSize   int           // Maximum capacity
	Hits      int64         // Total cache hits
	Misses    int64         // Total cache misses
	Evictions int64         // Total evictions (TTL + LRU)
	TTL       time.Duration // Current TTL setting
}

// Cache provides an LRU cache for cass correlation results.
// It is safe for concurrent use.
type Cache struct {
	entries   map[string]*list.Element // key -> list element
	order     *list.List               // LRU order (front = oldest)
	maxSize   int
	ttl       time.Duration
	mu        sync.RWMutex
	hits      int64
	misses    int64
	evictions int64

	// For testing: allow overriding time
	now func() time.Time
}

// NewCache creates a new Cache with default settings.
func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*list.Element),
		order:   list.New(),
		maxSize: DefaultResultCacheSize,
		ttl:     DefaultResultCacheTTL,
		now:     time.Now,
	}
}

// CacheOption configures a Cache.
type CacheOption func(*Cache)

// WithResultCacheSize sets the maximum cache size.
func WithResultCacheSize(size int) CacheOption {
	return func(c *Cache) {
		if size > 0 {
			c.maxSize = size
		}
	}
}

// WithResultCacheTTL sets the cache entry TTL.
func WithResultCacheTTL(ttl time.Duration) CacheOption {
	return func(c *Cache) {
		if ttl > 0 {
			c.ttl = ttl
		}
	}
}

// NewCacheWithOptions creates a Cache with custom options.
func NewCacheWithOptions(opts ...CacheOption) *Cache {
	c := NewCache()
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Get retrieves a cached hint by bead ID.
// Returns nil if not found or expired.
// This is an O(1) operation.
func (c *Cache) Get(beadID string) *CorrelationHint {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[beadID]
	if !ok {
		c.misses++
		return nil
	}

	entry := elem.Value.(*CacheEntry)

	// Check expiration
	if c.now().After(entry.ExpiresAt) {
		c.removeElement(elem)
		c.evictions++
		c.misses++
		return nil
	}

	// Move to back (most recently used)
	c.order.MoveToBack(elem)
	c.hits++

	return entry.Hint
}

// Set stores a correlation hint in the cache.
// If the cache is full, it evicts expired entries first, then LRU.
// O(1) when no eviction needed; O(n) worst case when eviction scans for expired entries.
func (c *Cache) Set(beadID string, hint *CorrelationHint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()

	// If already exists, update it
	if elem, ok := c.entries[beadID]; ok {
		entry := elem.Value.(*CacheEntry)
		entry.Hint = hint
		entry.CachedAt = now
		entry.ExpiresAt = now.Add(c.ttl)
		c.order.MoveToBack(elem)
		return
	}

	// Evict if necessary
	c.evictIfNeeded()

	// Create new entry
	entry := &CacheEntry{
		Key:       beadID,
		Hint:      hint,
		CachedAt:  now,
		ExpiresAt: now.Add(c.ttl),
	}

	elem := c.order.PushBack(entry)
	c.entries[beadID] = elem
}

// Invalidate removes a specific entry from the cache.
func (c *Cache) Invalidate(beadID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[beadID]; ok {
		c.removeElement(elem)
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*list.Element)
	c.order.Init()
}

// Stats returns current cache statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:      len(c.entries),
		MaxSize:   c.maxSize,
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		TTL:       c.ttl,
	}
}

// Size returns the current number of cached entries.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictIfNeeded removes entries until there's room for one more.
// Caller must hold c.mu (write lock).
func (c *Cache) evictIfNeeded() {
	// First pass: remove expired entries
	c.removeExpired()

	// Second pass: LRU eviction if still full
	for len(c.entries) >= c.maxSize {
		// Remove from front (least recently used)
		oldest := c.order.Front()
		if oldest == nil {
			break
		}
		c.removeElement(oldest)
		c.evictions++
	}
}

// removeExpired removes all expired entries.
// Caller must hold c.mu (write lock).
func (c *Cache) removeExpired() {
	now := c.now()
	var toRemove []*list.Element

	for elem := c.order.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*CacheEntry)
		if now.After(entry.ExpiresAt) {
			toRemove = append(toRemove, elem)
		}
	}

	for _, elem := range toRemove {
		c.removeElement(elem)
		c.evictions++
	}
}

// removeElement removes an element from both map and list.
// Caller must hold c.mu (write lock).
func (c *Cache) removeElement(elem *list.Element) {
	entry := elem.Value.(*CacheEntry)
	delete(c.entries, entry.Key)
	c.order.Remove(elem)
}

// Len returns the number of entries (alias for Size for list.List compatibility).
func (c *Cache) Len() int {
	return c.Size()
}
