package analysis

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// mockLoader implements IssueLoader for testing.
type mockLoader struct {
	mu       sync.Mutex
	calls    int
	issues   map[time.Time][]model.Issue
	failAt   map[time.Time]bool
	callback func(time.Time) // called on each LoadIssuesAsOf
}

func newMockLoader() *mockLoader {
	return &mockLoader{
		issues: make(map[time.Time][]model.Issue),
		failAt: make(map[time.Time]bool),
	}
}

func (m *mockLoader) addSnapshot(ts time.Time, issues []model.Issue) {
	m.issues[ts] = issues
}

func (m *mockLoader) failAtTimestamp(ts time.Time) {
	m.failAt[ts] = true
}

func (m *mockLoader) LoadIssuesAsOf(ts time.Time) ([]model.Issue, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()

	if m.callback != nil {
		m.callback(ts)
	}

	if m.failAt[ts] {
		return nil, fmt.Errorf("simulated failure at %s", ts)
	}

	if issues, ok := m.issues[ts]; ok {
		return issues, nil
	}

	// Return empty issues for any timestamp not explicitly configured
	return nil, nil
}

func makeIssues(open, blocked, closed int) []model.Issue {
	var issues []model.Issue
	for i := 0; i < open; i++ {
		issues = append(issues, model.Issue{
			ID:     fmt.Sprintf("open-%d", i),
			Status: model.StatusOpen,
		})
	}
	for i := 0; i < blocked; i++ {
		issues = append(issues, model.Issue{
			ID:     fmt.Sprintf("blocked-%d", i),
			Status: model.StatusBlocked,
		})
	}
	for i := 0; i < closed; i++ {
		issues = append(issues, model.Issue{
			ID:     fmt.Sprintf("closed-%d", i),
			Status: model.StatusClosed,
		})
	}
	return issues
}

func TestComputeSnapshotMetrics(t *testing.T) {
	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	threeDaysAgo := now.AddDate(0, 0, -3)
	tenDaysAgo := now.AddDate(0, 0, -10)

	issues := []model.Issue{
		{ID: "a", Status: model.StatusOpen},
		{ID: "b", Status: model.StatusBlocked},
		{ID: "c", Status: model.StatusClosed, ClosedAt: &threeDaysAgo}, // within 7d
		{ID: "d", Status: model.StatusClosed, ClosedAt: &tenDaysAgo},   // outside 7d
		{ID: "e", Status: model.StatusTombstone},
		{ID: "f", Status: model.StatusInProgress},
	}

	m := ComputeSnapshotMetrics(issues, now)

	if m.TotalCount != 6 {
		t.Errorf("TotalCount = %d, want 6", m.TotalCount)
	}
	if m.OpenCount != 3 { // a + b (blocked counts as open) + f
		t.Errorf("OpenCount = %d, want 3", m.OpenCount)
	}
	if m.BlockedCount != 1 {
		t.Errorf("BlockedCount = %d, want 1", m.BlockedCount)
	}
	if m.ClosedCount != 3 { // c + d + e (tombstone)
		t.Errorf("ClosedCount = %d, want 3", m.ClosedCount)
	}
	if m.Velocity7d != 1 { // only c is within 7 days
		t.Errorf("Velocity7d = %d, want 1", m.Velocity7d)
	}
	if !m.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", m.Timestamp, now)
	}
}

func TestComputeSnapshotMetrics_Empty(t *testing.T) {
	m := ComputeSnapshotMetrics(nil, time.Now())
	if m.TotalCount != 0 || m.OpenCount != 0 || m.ClosedCount != 0 {
		t.Errorf("empty input should yield zero metrics: %+v", m)
	}
}

func TestNewTemporalCache(t *testing.T) {
	cfg := TemporalCacheConfig{MaxSnapshots: 10, TTL: 30 * time.Minute}
	tc := NewTemporalCache(cfg)

	if tc.maxSnapshots != 10 {
		t.Errorf("maxSnapshots = %d, want 10", tc.maxSnapshots)
	}
	if tc.ttl != 30*time.Minute {
		t.Errorf("ttl = %v, want 30m", tc.ttl)
	}
	if tc.SnapshotCount() != 0 {
		t.Error("new cache should be empty")
	}
	if !tc.IsStale() {
		t.Error("new cache should be stale")
	}
}

func TestNewTemporalCache_Defaults(t *testing.T) {
	tc := NewTemporalCache(TemporalCacheConfig{})
	if tc.maxSnapshots != 30 {
		t.Errorf("default maxSnapshots = %d, want 30", tc.maxSnapshots)
	}
	if tc.ttl != time.Hour {
		t.Errorf("default ttl = %v, want 1h", tc.ttl)
	}
}

func TestTemporalCache_Populate(t *testing.T) {
	loader := newMockLoader()
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	interval := 24 * time.Hour

	// Add snapshots for each day
	for d := 0; d <= 6; d++ {
		ts := from.AddDate(0, 0, d)
		loader.addSnapshot(ts, makeIssues(10-d, d, d*2))
	}

	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 30, TTL: time.Hour})
	loaded, err := tc.Populate(loader, from, to, interval)

	if err != nil {
		t.Fatalf("Populate error: %v", err)
	}
	if loaded != 7 {
		t.Errorf("loaded = %d, want 7", loaded)
	}
	if tc.SnapshotCount() != 7 {
		t.Errorf("SnapshotCount = %d, want 7", tc.SnapshotCount())
	}
	if tc.IsStale() {
		t.Error("cache should not be stale immediately after population")
	}

	// Verify data
	issues := tc.Get(from)
	if len(issues) != 10 { // 10 open + 0 blocked + 0 closed
		t.Errorf("issues at from = %d, want 10", len(issues))
	}

	// Timestamps should be sorted ascending
	timestamps := tc.Timestamps()
	if len(timestamps) != 7 {
		t.Fatalf("timestamps count = %d, want 7", len(timestamps))
	}
	for i := 1; i < len(timestamps); i++ {
		if !timestamps[i].After(timestamps[i-1]) {
			t.Errorf("timestamps not sorted: %v after %v", timestamps[i], timestamps[i-1])
		}
	}
}

func TestTemporalCache_PopulatePartialFailure(t *testing.T) {
	loader := newMockLoader()
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	interval := 24 * time.Hour

	// Day 1: works, Day 2: fails, Day 3: works
	loader.addSnapshot(from, makeIssues(5, 0, 0))
	loader.failAtTimestamp(from.AddDate(0, 0, 1))
	loader.addSnapshot(from.AddDate(0, 0, 2), makeIssues(3, 0, 0))

	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 30, TTL: time.Hour})
	loaded, err := tc.Populate(loader, from, to, interval)

	// Should report the failure but still load what it can
	if err == nil {
		t.Error("expected error from partial failure")
	}
	if loaded != 2 {
		t.Errorf("loaded = %d, want 2 (skipping failed database)", loaded)
	}
	if tc.SnapshotCount() != 2 {
		t.Errorf("SnapshotCount = %d, want 2", tc.SnapshotCount())
	}
}

func TestTemporalCache_MaxSnapshots(t *testing.T) {
	loader := newMockLoader()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	interval := 24 * time.Hour // 365 days, but max 5 snapshots

	for d := 0; d < 365; d++ {
		ts := from.AddDate(0, 0, d)
		loader.addSnapshot(ts, makeIssues(1, 0, 0))
	}

	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 5, TTL: time.Hour})
	loaded, _ := tc.Populate(loader, from, to, interval)

	// Should cap at maxSnapshots
	if loaded > 5 {
		t.Errorf("loaded = %d, should not exceed maxSnapshots (5)", loaded)
	}
	if tc.SnapshotCount() > 5 {
		t.Errorf("SnapshotCount = %d, should not exceed 5", tc.SnapshotCount())
	}
}

func TestTemporalCache_SkipsCached(t *testing.T) {
	loader := newMockLoader()
	ts := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	loader.addSnapshot(ts, makeIssues(5, 0, 0))

	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 30, TTL: time.Hour})

	// First populate
	tc.Populate(loader, ts, ts, 24*time.Hour)
	firstCalls := loader.calls

	// Second populate - should skip already-cached timestamp
	tc.Populate(loader, ts, ts, 24*time.Hour)
	if loader.calls != firstCalls {
		t.Errorf("second populate should not re-query cached timestamps, calls: %d -> %d",
			firstCalls, loader.calls)
	}
}

func TestTemporalCache_ConcurrentPopulate(t *testing.T) {
	loader := newMockLoader()
	ts := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	loader.addSnapshot(ts, makeIssues(5, 0, 0))

	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 30, TTL: time.Hour})

	// First populate starts
	var wg sync.WaitGroup
	wg.Add(1)
	loader.callback = func(_ time.Time) {
		// While first populate is running, try second
		_, err := tc.Populate(loader, ts, ts, 24*time.Hour)
		if err == nil {
			t.Error("concurrent populate should return error")
		}
		wg.Done()
		loader.callback = nil // clear to avoid infinite recursion
	}

	tc.Populate(loader, ts, ts, 24*time.Hour)
	wg.Wait()
}

func TestTemporalCache_Clear(t *testing.T) {
	loader := newMockLoader()
	ts := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	loader.addSnapshot(ts, makeIssues(5, 0, 0))

	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 30, TTL: time.Hour})
	tc.Populate(loader, ts, ts, 24*time.Hour)

	if tc.SnapshotCount() == 0 {
		t.Fatal("should have snapshots before clear")
	}

	tc.Clear()

	if tc.SnapshotCount() != 0 {
		t.Error("should have no snapshots after clear")
	}
	if !tc.IsStale() {
		t.Error("should be stale after clear")
	}
}

func TestTemporalCache_PopulateInvalidRange(t *testing.T) {
	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 30, TTL: time.Hour})

	to := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	from := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC) // from after to

	_, err := tc.Populate(nil, from, to, 24*time.Hour)
	if err == nil {
		t.Error("should reject from > to")
	}

	_, err = tc.Populate(nil, to, from, 0)
	if err == nil {
		t.Error("should reject zero interval")
	}
}

func TestComputeMetricsSeries(t *testing.T) {
	loader := newMockLoader()
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	// 3 days of snapshots with increasing closed counts
	for d := 0; d < 3; d++ {
		ts := from.AddDate(0, 0, d)
		loader.addSnapshot(ts, makeIssues(10-d, d, d*5))
	}

	tc := NewTemporalCache(TemporalCacheConfig{MaxSnapshots: 30, TTL: time.Hour})
	tc.Populate(loader, from, from.AddDate(0, 0, 2), 24*time.Hour)

	series := ComputeMetricsSeries(tc)
	if len(series) != 3 {
		t.Fatalf("series length = %d, want 3", len(series))
	}

	// Should be sorted ascending
	for i := 1; i < len(series); i++ {
		if !series[i].Timestamp.After(series[i-1].Timestamp) {
			t.Error("series not sorted ascending")
		}
	}

	// Check counts on first snapshot: 10 open, 0 blocked, 0 closed
	if series[0].OpenCount != 10 {
		t.Errorf("series[0].OpenCount = %d, want 10", series[0].OpenCount)
	}
	if series[0].ClosedCount != 0 {
		t.Errorf("series[0].ClosedCount = %d, want 0", series[0].ClosedCount)
	}
}

func TestComputeMetricsSeries_Nil(t *testing.T) {
	series := ComputeMetricsSeries(nil)
	if series != nil {
		t.Error("nil cache should return nil series")
	}
}
