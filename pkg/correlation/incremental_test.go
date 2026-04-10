package correlation

import (
	"testing"
	"time"
)

func TestIncrementalThreshold(t *testing.T) {
	if IncrementalThreshold != 100 {
		t.Errorf("IncrementalThreshold = %d, want 100", IncrementalThreshold)
	}
}

func TestNewIncrementalCorrelator(t *testing.T) {
	ic := NewIncrementalCorrelator("/tmp/test")

	if ic.correlator == nil {
		t.Error("correlator should not be nil")
	}
	if ic.cache == nil {
		t.Error("cache should not be nil")
	}
	if ic.hits != 0 || ic.misses != 0 || ic.increments != 0 || ic.refreshes != 0 {
		t.Error("initial stats should all be 0")
	}
}

func TestNewIncrementalCorrelatorWithOptions(t *testing.T) {
	ic := NewIncrementalCorrelatorWithOptions("/tmp/test", 10*time.Minute, 20)

	if ic.cache.maxAge != 10*time.Minute {
		t.Errorf("maxAge = %v, want 10m", ic.cache.maxAge)
	}
	if ic.cache.maxSize != 20 {
		t.Errorf("maxSize = %d, want 20", ic.cache.maxSize)
	}
}

func TestIncrementalCorrelator_CacheStats(t *testing.T) {
	ic := NewIncrementalCorrelator("/tmp/test")

	stats := ic.CacheStats()

	if stats.Hits != 0 {
		t.Errorf("Hits = %d, want 0", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("Misses = %d, want 0", stats.Misses)
	}
	if stats.IncrementalUpdates != 0 {
		t.Errorf("IncrementalUpdates = %d, want 0", stats.IncrementalUpdates)
	}
	if stats.FullRefreshes != 0 {
		t.Errorf("FullRefreshes = %d, want 0", stats.FullRefreshes)
	}
}

func TestIncrementalCorrelator_InvalidateCache(t *testing.T) {
	ic := NewIncrementalCorrelator("/tmp/test")

	// Add something to cache manually
	key := CacheKey{HeadSHA: "abc", BeadsHash: "def", Options: "ghi"}
	ic.cache.Put(key, &HistoryReport{})

	if ic.cache.Size() != 1 {
		t.Fatal("cache should have 1 entry")
	}

	ic.InvalidateCache()

	if ic.cache.Size() != 0 {
		t.Errorf("cache size after invalidate = %d, want 0", ic.cache.Size())
	}
}

func TestMergeReports_Basic(t *testing.T) {
	existing := &HistoryReport{
		GeneratedAt:     time.Now().Add(-1 * time.Hour),
		DataHash:        "existinghash",
		GitRange:        "abc123..def456",
		LatestCommitSHA: "def456",
		Stats: HistoryStats{
			TotalBeads:         1,
			MethodDistribution: make(map[string]int),
		},
		Histories: map[string]BeadHistory{
			"bv-1": {
				BeadID: "bv-1",
				Title:  "Test Bead",
				Status: "open",
				Events: []BeadEvent{
					{BeadID: "bv-1", EventType: EventCreated, CommitSHA: "abc123"},
				},
				Commits: []CorrelatedCommit{},
			},
		},
		CommitIndex: make(CommitIndex),
	}

	beads := []BeadInfo{
		{ID: "bv-1", Title: "Test Bead Updated", Status: "in_progress"},
	}

	newEvents := []BeadEvent{
		{BeadID: "bv-1", EventType: EventClaimed, CommitSHA: "ghi789", Timestamp: time.Now()},
	}

	merged := mergeReports(existing, beads, newEvents, nil)

	// Check merged report
	if merged.DataHash != existing.DataHash {
		t.Errorf("DataHash changed: %s != %s", merged.DataHash, existing.DataHash)
	}

	if merged.GitRange != "abc123..def456 (incremental)" {
		t.Errorf("GitRange = %s, want 'abc123..def456 (incremental)'", merged.GitRange)
	}

	// Check merged history
	h, ok := merged.Histories["bv-1"]
	if !ok {
		t.Fatal("bv-1 history not found")
	}

	if len(h.Events) != 2 {
		t.Errorf("Events count = %d, want 2", len(h.Events))
	}

	// Status should be updated from beads list
	if h.Status != "in_progress" {
		t.Errorf("Status = %s, want in_progress", h.Status)
	}

	// Title should be updated
	if h.Title != "Test Bead Updated" {
		t.Errorf("Title = %s, want 'Test Bead Updated'", h.Title)
	}
}

func TestMergeReports_NewBeads(t *testing.T) {
	existing := &HistoryReport{
		GeneratedAt:     time.Now(),
		DataHash:        "hash",
		LatestCommitSHA: "abc123",
		Stats: HistoryStats{
			TotalBeads:         1,
			MethodDistribution: make(map[string]int),
		},
		Histories: map[string]BeadHistory{
			"bv-1": {BeadID: "bv-1", Title: "Existing", Status: "open"},
		},
		CommitIndex: make(CommitIndex),
	}

	beads := []BeadInfo{
		{ID: "bv-1", Title: "Existing", Status: "open"},
		{ID: "bv-2", Title: "New Bead", Status: "open"}, // New bead
	}

	merged := mergeReports(existing, beads, nil, nil)

	if len(merged.Histories) != 2 {
		t.Errorf("Histories count = %d, want 2", len(merged.Histories))
	}

	if _, ok := merged.Histories["bv-2"]; !ok {
		t.Error("New bead bv-2 should be in merged report")
	}
}

func TestMergeReports_CommitMerge(t *testing.T) {
	existing := &HistoryReport{
		GeneratedAt:     time.Now(),
		DataHash:        "hash",
		LatestCommitSHA: "abc123",
		Stats: HistoryStats{
			TotalBeads:         1,
			MethodDistribution: make(map[string]int),
		},
		Histories: map[string]BeadHistory{
			"bv-1": {
				BeadID:  "bv-1",
				Commits: []CorrelatedCommit{{SHA: "commit1", Author: "Alice"}},
			},
		},
		CommitIndex: make(CommitIndex),
	}

	beads := []BeadInfo{{ID: "bv-1"}}

	newEvents := []BeadEvent{
		{BeadID: "bv-1", CommitSHA: "commit2"},
	}

	newCommits := []CorrelatedCommit{
		{SHA: "commit2", Author: "Bob", Timestamp: time.Now()},
	}

	merged := mergeReports(existing, beads, newEvents, newCommits)

	h := merged.Histories["bv-1"]
	if len(h.Commits) != 2 {
		t.Errorf("Commits count = %d, want 2", len(h.Commits))
	}

	// Last author should be updated
	if h.LastAuthor != "Bob" {
		t.Errorf("LastAuthor = %s, want Bob", h.LastAuthor)
	}
}

func TestMergeReports_CommitDedup(t *testing.T) {
	existing := &HistoryReport{
		GeneratedAt:     time.Now(),
		DataHash:        "hash",
		LatestCommitSHA: "abc123",
		Stats: HistoryStats{
			TotalBeads:         1,
			MethodDistribution: make(map[string]int),
		},
		Histories: map[string]BeadHistory{
			"bv-1": {
				BeadID:  "bv-1",
				Commits: []CorrelatedCommit{{SHA: "commit1"}},
			},
		},
		CommitIndex: make(CommitIndex),
	}

	beads := []BeadInfo{{ID: "bv-1"}}

	newEvents := []BeadEvent{
		{BeadID: "bv-1", CommitSHA: "commit1"}, // Same commit
	}

	newCommits := []CorrelatedCommit{
		{SHA: "commit1"}, // Duplicate
	}

	merged := mergeReports(existing, beads, newEvents, newCommits)

	h := merged.Histories["bv-1"]
	if len(h.Commits) != 1 {
		t.Errorf("Commits should be deduped: got %d, want 1", len(h.Commits))
	}
}

func TestMergeReports_CommitIndex(t *testing.T) {
	existing := &HistoryReport{
		GeneratedAt:     time.Now(),
		DataHash:        "hash",
		LatestCommitSHA: "abc123",
		Stats: HistoryStats{
			TotalBeads:         2,
			MethodDistribution: make(map[string]int),
		},
		Histories: map[string]BeadHistory{
			"bv-1": {BeadID: "bv-1", Commits: []CorrelatedCommit{{SHA: "c1"}}},
			"bv-2": {BeadID: "bv-2", Commits: []CorrelatedCommit{{SHA: "c2"}}},
		},
		CommitIndex: make(CommitIndex),
	}

	beads := []BeadInfo{{ID: "bv-1"}, {ID: "bv-2"}}

	merged := mergeReports(existing, beads, nil, nil)

	if len(merged.CommitIndex["c1"]) != 1 || merged.CommitIndex["c1"][0] != "bv-1" {
		t.Error("CommitIndex for c1 incorrect")
	}
	if len(merged.CommitIndex["c2"]) != 1 || merged.CommitIndex["c2"][0] != "bv-2" {
		t.Error("CommitIndex for c2 incorrect")
	}
}

func TestMergeReports_Stats(t *testing.T) {
	existing := &HistoryReport{
		GeneratedAt:     time.Now(),
		DataHash:        "hash",
		LatestCommitSHA: "abc123",
		Stats: HistoryStats{
			TotalBeads:         2,
			MethodDistribution: make(map[string]int),
		},
		Histories: map[string]BeadHistory{
			"bv-1": {
				BeadID:  "bv-1",
				Events:  []BeadEvent{{Author: "Alice"}},
				Commits: []CorrelatedCommit{{SHA: "c1", Author: "Alice", Method: MethodCoCommitted}},
			},
			"bv-2": {
				BeadID:  "bv-2",
				Events:  []BeadEvent{{Author: "Bob"}},
				Commits: []CorrelatedCommit{{SHA: "c2", Author: "Bob", Method: MethodExplicitID}},
			},
		},
		CommitIndex: make(CommitIndex),
	}

	beads := []BeadInfo{{ID: "bv-1"}, {ID: "bv-2"}}

	merged := mergeReports(existing, beads, nil, nil)

	if merged.Stats.TotalBeads != 2 {
		t.Errorf("TotalBeads = %d, want 2", merged.Stats.TotalBeads)
	}
	if merged.Stats.TotalCommits != 2 {
		t.Errorf("TotalCommits = %d, want 2", merged.Stats.TotalCommits)
	}
	if merged.Stats.UniqueAuthors != 2 {
		t.Errorf("UniqueAuthors = %d, want 2", merged.Stats.UniqueAuthors)
	}
	if merged.Stats.BeadsWithCommits != 2 {
		t.Errorf("BeadsWithCommits = %d, want 2", merged.Stats.BeadsWithCommits)
	}
}

func TestMergeReports_MilestonesRecalculated(t *testing.T) {
	createdTime := time.Now().Add(-2 * time.Hour)
	claimedTime := time.Now().Add(-1 * time.Hour)
	closedTime := time.Now()

	existing := &HistoryReport{
		GeneratedAt:     time.Now(),
		DataHash:        "hash",
		LatestCommitSHA: "abc123",
		Stats: HistoryStats{
			TotalBeads:         1,
			MethodDistribution: make(map[string]int),
		},
		Histories: map[string]BeadHistory{
			"bv-1": {
				BeadID: "bv-1",
				Events: []BeadEvent{
					{BeadID: "bv-1", EventType: EventCreated, Timestamp: createdTime},
					{BeadID: "bv-1", EventType: EventClaimed, Timestamp: claimedTime},
				},
			},
		},
		CommitIndex: make(CommitIndex),
	}

	beads := []BeadInfo{{ID: "bv-1", Status: "closed"}}

	newEvents := []BeadEvent{
		{BeadID: "bv-1", EventType: EventClosed, Timestamp: closedTime, CommitSHA: "def456"},
	}

	merged := mergeReports(existing, beads, newEvents, nil)

	h := merged.Histories["bv-1"]

	// Check milestones were recalculated
	if h.Milestones.Created == nil {
		t.Error("Created milestone should exist")
	}
	if h.Milestones.Claimed == nil {
		t.Error("Claimed milestone should exist")
	}
	if h.Milestones.Closed == nil {
		t.Error("Closed milestone should exist after merge")
	}

	// Check cycle time was calculated
	if h.CycleTime == nil {
		t.Error("CycleTime should be calculated after close")
	}
	if h.CycleTime.ClaimToClose == nil {
		t.Error("ClaimToClose should be set")
	}
}

func TestIncrementalUpdateResult_Fields(t *testing.T) {
	result := IncrementalUpdateResult{
		Report:            &HistoryReport{},
		WasIncremental:    true,
		NewCommitCount:    5,
		MergedEventCount:  3,
		MergedCommitCount: 2,
		RefreshReason:     "",
	}

	if !result.WasIncremental {
		t.Error("WasIncremental should be true")
	}
	if result.NewCommitCount != 5 {
		t.Errorf("NewCommitCount = %d, want 5", result.NewCommitCount)
	}
	if result.RefreshReason != "" {
		t.Errorf("RefreshReason should be empty for incremental")
	}
}

func TestCanUpdateIncrementally_NoReport(t *testing.T) {
	ok, count, err := CanUpdateIncrementally("/tmp", nil)
	if ok {
		t.Error("Should return false for nil report")
	}
	if count != 0 {
		t.Errorf("Count should be 0, got %d", count)
	}
	if err != nil {
		t.Errorf("Should not return error for nil report, got %v", err)
	}
}

func TestCanUpdateIncrementally_EmptySHA(t *testing.T) {
	report := &HistoryReport{LatestCommitSHA: ""}
	ok, count, err := CanUpdateIncrementally("/tmp", report)
	if ok {
		t.Error("Should return false for empty SHA")
	}
	if count != 0 {
		t.Errorf("Count should be 0, got %d", count)
	}
	if err != nil {
		t.Errorf("Should not return error for empty SHA, got %v", err)
	}
}

func TestIncrementalCorrelator_GenerateReport_FullRefresh(t *testing.T) {
	// Skip if not in a git repo
	if _, err := getGitHead("."); err != nil {
		t.Skip("Not in a git repository")
	}

	ic := NewIncrementalCorrelator(".")
	beads := []BeadInfo{{ID: "test-1", Status: "open"}}
	opts := CorrelatorOptions{Limit: 10}

	// First call should do full refresh
	result, err := ic.GenerateReportWithDetails(beads, opts)
	if err != nil {
		t.Fatalf("GenerateReportWithDetails failed: %v", err)
	}

	if result.WasIncremental {
		t.Error("First call should not be incremental")
	}

	stats := ic.CacheStats()
	if stats.FullRefreshes != 1 {
		t.Errorf("FullRefreshes = %d, want 1", stats.FullRefreshes)
	}
}

func TestIncrementalCorrelator_GenerateReport_CacheHit(t *testing.T) {
	// Skip if not in a git repo
	if _, err := getGitHead("."); err != nil {
		t.Skip("Not in a git repository")
	}

	ic := NewIncrementalCorrelator(".")
	beads := []BeadInfo{{ID: "test-1", Status: "open"}}
	opts := CorrelatorOptions{Limit: 10}

	// First call
	_, err := ic.GenerateReport(beads, opts)
	if err != nil {
		t.Fatalf("First GenerateReport failed: %v", err)
	}

	// Second call should hit cache
	_, err = ic.GenerateReport(beads, opts)
	if err != nil {
		t.Fatalf("Second GenerateReport failed: %v", err)
	}

	stats := ic.CacheStats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}
}

func TestCalculateMergedStats_CycleTime(t *testing.T) {
	claimTime := time.Now().Add(-2 * time.Hour)
	closeTime := time.Now()
	duration := closeTime.Sub(claimTime)

	histories := map[string]BeadHistory{
		"bv-1": {
			BeadID:  "bv-1",
			Commits: []CorrelatedCommit{{SHA: "c1", Author: "Alice", Method: MethodCoCommitted}},
			Events:  []BeadEvent{{Author: "Alice"}},
			CycleTime: &CycleTime{
				ClaimToClose: &duration,
			},
		},
	}

	stats := calculateMergedStats(histories, nil)

	if stats.AvgCycleTimeDays == nil {
		t.Error("AvgCycleTimeDays should be set when cycle times exist")
	}
}
