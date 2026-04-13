package analysis

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// SnapshotMetrics summarizes issue state at a point in time.
// Lightweight alternative to storing full []model.Issue per snapshot.
type SnapshotMetrics struct {
	Timestamp    time.Time `json:"timestamp"`
	OpenCount    int       `json:"open_count"`
	BlockedCount int       `json:"blocked_count"`
	ClosedCount  int       `json:"closed_count"`
	TotalCount   int       `json:"total_count"`
	Velocity7d   int       `json:"velocity_7d"`  // Issues closed in the 7 days before this snapshot
	CycleCount   int       `json:"cycle_count"`   // Dependency cycles at this point
}

// ComputeSnapshotMetrics computes metrics for a set of issues at a given timestamp.
// The velocity field counts issues with ClosedAt in the 7 days before ts.
func ComputeSnapshotMetrics(issues []model.Issue, ts time.Time) SnapshotMetrics {
	m := SnapshotMetrics{
		Timestamp:  ts,
		TotalCount: len(issues),
	}

	sevenDaysAgo := ts.AddDate(0, 0, -7)

	for i := range issues {
		issue := &issues[i]
		switch issue.Status {
		case model.StatusClosed, model.StatusTombstone:
			m.ClosedCount++
			if issue.ClosedAt != nil && issue.ClosedAt.After(sevenDaysAgo) && !issue.ClosedAt.After(ts) {
				m.Velocity7d++
			}
		case model.StatusBlocked:
			m.BlockedCount++
			m.OpenCount++
		default:
			m.OpenCount++
		}
	}

	return m
}

// ComputeMetricsSeries computes SnapshotMetrics for each snapshot in a TemporalCache.
// Results are sorted by timestamp ascending.
func ComputeMetricsSeries(cache *TemporalCache) []SnapshotMetrics {
	if cache == nil {
		return nil
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	series := make([]SnapshotMetrics, 0, len(cache.snapshots))
	for ts, issues := range cache.snapshots {
		series = append(series, ComputeSnapshotMetrics(issues, ts))
	}

	sort.Slice(series, func(i, j int) bool {
		return series[i].Timestamp.Before(series[j].Timestamp)
	})

	return series
}

// IssueLoader is an interface for loading issues at a point in time.
// GlobalDoltReader.LoadIssuesAsOf satisfies this.
type IssueLoader interface {
	LoadIssuesAsOf(timestamp time.Time) ([]model.Issue, error)
}

// TemporalCache stores historical issue snapshots keyed by timestamp.
// Snapshots are loaded via Dolt AS OF queries and cached with TTL-based
// invalidation. Background population runs on a separate goroutine at
// a slow cadence (hourly default), never on the 3-second UI poll cycle.
type TemporalCache struct {
	mu        sync.RWMutex
	snapshots map[time.Time][]model.Issue // keyed by truncated timestamp

	// Configuration
	maxSnapshots int           // upper bound on stored snapshots (default 30)
	ttl          time.Duration // how long before cache is stale (default 1hr)

	// State
	lastPopulated time.Time // when the cache was last fully populated
	populating    bool      // true while background population is running
	populateErr   error     // last population error (if any)
}

// TemporalCacheConfig configures a TemporalCache.
type TemporalCacheConfig struct {
	MaxSnapshots int           // max snapshots to store (default 30)
	TTL          time.Duration // cache staleness threshold (default 1hr)
}

// DefaultTemporalCacheConfig returns the default configuration.
// Reads BT_TEMPORAL_CACHE_TTL from the environment (format: "30m", "2h", etc.)
func DefaultTemporalCacheConfig() TemporalCacheConfig {
	cfg := TemporalCacheConfig{
		MaxSnapshots: 30,
		TTL:          1 * time.Hour,
	}

	if v := os.Getenv("BT_TEMPORAL_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.TTL = d
		} else {
			slog.Warn("invalid BT_TEMPORAL_CACHE_TTL, using default", "value", v)
		}
	}

	if v := os.Getenv("BT_TEMPORAL_MAX_SNAPSHOTS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			cfg.MaxSnapshots = n
		}
	}

	return cfg
}

// NewTemporalCache creates a new cache with the given configuration.
func NewTemporalCache(cfg TemporalCacheConfig) *TemporalCache {
	if cfg.MaxSnapshots <= 0 {
		cfg.MaxSnapshots = 30
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 1 * time.Hour
	}

	return &TemporalCache{
		snapshots:    make(map[time.Time][]model.Issue),
		maxSnapshots: cfg.MaxSnapshots,
		ttl:          cfg.TTL,
	}
}

// IsStale returns true if the cache needs repopulation.
func (tc *TemporalCache) IsStale() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.lastPopulated.IsZero() || time.Since(tc.lastPopulated) > tc.ttl
}

// IsPopulating returns true if a background population is in progress.
func (tc *TemporalCache) IsPopulating() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.populating
}

// SnapshotCount returns the number of cached snapshots.
func (tc *TemporalCache) SnapshotCount() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.snapshots)
}

// LastPopulated returns when the cache was last fully populated.
func (tc *TemporalCache) LastPopulated() time.Time {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.lastPopulated
}

// LastError returns the error from the most recent population attempt.
func (tc *TemporalCache) LastError() error {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.populateErr
}

// Timestamps returns all cached snapshot timestamps, sorted ascending.
func (tc *TemporalCache) Timestamps() []time.Time {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	ts := make([]time.Time, 0, len(tc.snapshots))
	for t := range tc.snapshots {
		ts = append(ts, t)
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i].Before(ts[j]) })
	return ts
}

// Get returns the issues snapshot at the given timestamp, or nil if not cached.
func (tc *TemporalCache) Get(ts time.Time) []model.Issue {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.snapshots[ts]
}

// Populate loads snapshots from the given loader at regular intervals within [from, to].
// This is designed to run in a background goroutine. It respects maxSnapshots by
// adjusting the interval or evicting oldest snapshots.
//
// Returns the number of snapshots successfully loaded.
func (tc *TemporalCache) Populate(loader IssueLoader, from, to time.Time, interval time.Duration) (int, error) {
	if from.After(to) {
		return 0, fmt.Errorf("temporal cache: from (%s) is after to (%s)", from, to)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("temporal cache: interval must be positive, got %s", interval)
	}

	tc.mu.Lock()
	if tc.populating {
		tc.mu.Unlock()
		return 0, fmt.Errorf("temporal cache: population already in progress")
	}
	tc.populating = true
	tc.mu.Unlock()

	defer func() {
		tc.mu.Lock()
		tc.populating = false
		tc.mu.Unlock()
	}()

	// Generate timestamps to query
	var timestamps []time.Time
	for t := from; !t.After(to) && len(timestamps) < tc.maxSnapshots; t = t.Add(interval) {
		timestamps = append(timestamps, t)
	}

	loaded := 0
	var lastErr error

	for _, ts := range timestamps {
		// Skip if already cached
		tc.mu.RLock()
		_, exists := tc.snapshots[ts]
		tc.mu.RUnlock()
		if exists {
			loaded++
			continue
		}

		issues, err := loader.LoadIssuesAsOf(ts)
		if err != nil {
			slog.Warn("temporal cache: failed to load snapshot",
				"timestamp", ts, "error", err)
			lastErr = err
			continue
		}

		tc.mu.Lock()
		tc.snapshots[ts] = issues
		tc.mu.Unlock()
		loaded++
	}

	// Evict oldest if over capacity
	tc.evictOldest()

	tc.mu.Lock()
	tc.lastPopulated = time.Now()
	tc.populateErr = lastErr
	tc.mu.Unlock()

	return loaded, lastErr
}

// evictOldest removes the oldest snapshots when cache exceeds maxSnapshots.
func (tc *TemporalCache) evictOldest() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	for len(tc.snapshots) > tc.maxSnapshots {
		// Find the oldest timestamp
		var oldest time.Time
		first := true
		for ts := range tc.snapshots {
			if first || ts.Before(oldest) {
				oldest = ts
				first = false
			}
		}
		delete(tc.snapshots, oldest)
	}
}

// Clear removes all cached snapshots.
func (tc *TemporalCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.snapshots = make(map[time.Time][]model.Issue)
	tc.lastPopulated = time.Time{}
	tc.populateErr = nil
}
