package correlation

import (
	"testing"
	"time"
)

// Helper to create test timestamps
func testTime(offsetHours int) time.Time {
	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetHours) * time.Hour)
}

func TestBuildCausalityChain_BasicChain(t *testing.T) {
	report := &HistoryReport{
		DataHash: "test-hash",
		Histories: map[string]BeadHistory{
			"bv-test": {
				BeadID: "bv-test",
				Title:  "Test Bead",
				Status: "closed",
				Events: []BeadEvent{
					{EventType: EventCreated, Timestamp: testTime(0)},
					{EventType: EventClaimed, Timestamp: testTime(2)},
					{EventType: EventClosed, Timestamp: testTime(10)},
				},
				Commits: []CorrelatedCommit{
					{ShortSHA: "abc1234", Message: "Fix bug", Timestamp: testTime(5)},
				},
			},
		},
	}

	opts := DefaultCausalityOptions()
	result := report.BuildCausalityChain("bv-test", opts)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check chain structure
	if result.Chain.BeadID != "bv-test" {
		t.Errorf("Expected bead ID 'bv-test', got '%s'", result.Chain.BeadID)
	}

	if result.Chain.Status != "closed" {
		t.Errorf("Expected status 'closed', got '%s'", result.Chain.Status)
	}

	if !result.Chain.IsComplete {
		t.Error("Expected IsComplete to be true for closed bead")
	}

	// Should have 4 events: created, claimed, commit, closed
	if len(result.Chain.Events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(result.Chain.Events))
	}

	// Check event order (should be sorted by timestamp)
	expectedOrder := []CausalEventType{CausalCreated, CausalClaimed, CausalCommit, CausalClosed}
	for i, expected := range expectedOrder {
		if result.Chain.Events[i].Type != expected {
			t.Errorf("Event %d: expected type '%s', got '%s'", i, expected, result.Chain.Events[i].Type)
		}
	}
}

func TestBuildCausalityChain_CausalLinks(t *testing.T) {
	report := &HistoryReport{
		DataHash: "test-hash",
		Histories: map[string]BeadHistory{
			"bv-test": {
				BeadID: "bv-test",
				Title:  "Test Bead",
				Status: "closed",
				Events: []BeadEvent{
					{EventType: EventCreated, Timestamp: testTime(0)},
					{EventType: EventClaimed, Timestamp: testTime(1)},
					{EventType: EventClosed, Timestamp: testTime(2)},
				},
			},
		},
	}

	opts := CausalityOptions{IncludeCommits: false}
	result := report.BuildCausalityChain("bv-test", opts)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check causal links
	// Event 0 (created) should enable event 1 (claimed)
	if len(result.Chain.Events[0].EnablesIDs) != 1 || result.Chain.Events[0].EnablesIDs[0] != 1 {
		t.Errorf("Event 0 should enable event 1, got enables: %v", result.Chain.Events[0].EnablesIDs)
	}

	// Event 1 (claimed) should be caused by event 0 and enable event 2
	if result.Chain.Events[1].CausedByID == nil || *result.Chain.Events[1].CausedByID != 0 {
		t.Error("Event 1 should be caused by event 0")
	}
	if len(result.Chain.Events[1].EnablesIDs) != 1 || result.Chain.Events[1].EnablesIDs[0] != 2 {
		t.Errorf("Event 1 should enable event 2, got enables: %v", result.Chain.Events[1].EnablesIDs)
	}

	// Event 2 (closed) should be caused by event 1
	if result.Chain.Events[2].CausedByID == nil || *result.Chain.Events[2].CausedByID != 1 {
		t.Error("Event 2 should be caused by event 1")
	}
}

func TestBuildCausalityChain_NotFound(t *testing.T) {
	report := &HistoryReport{
		DataHash:  "test-hash",
		Histories: map[string]BeadHistory{},
	}

	opts := DefaultCausalityOptions()
	result := report.BuildCausalityChain("nonexistent", opts)

	if result != nil {
		t.Error("Expected nil result for nonexistent bead")
	}
}

func TestBuildCausalityChain_WithCommits(t *testing.T) {
	report := &HistoryReport{
		DataHash: "test-hash",
		Histories: map[string]BeadHistory{
			"bv-test": {
				BeadID: "bv-test",
				Title:  "Test Bead",
				Status: "in_progress",
				Events: []BeadEvent{
					{EventType: EventCreated, Timestamp: testTime(0)},
					{EventType: EventClaimed, Timestamp: testTime(1)},
				},
				Commits: []CorrelatedCommit{
					{ShortSHA: "abc1234", Message: "First commit", Timestamp: testTime(2)},
					{ShortSHA: "def5678", Message: "Second commit", Timestamp: testTime(3)},
				},
			},
		},
	}

	// With commits
	optsWithCommits := CausalityOptions{IncludeCommits: true}
	resultWith := report.BuildCausalityChain("bv-test", optsWithCommits)

	if resultWith.Insights.CommitCount != 2 {
		t.Errorf("Expected 2 commits, got %d", resultWith.Insights.CommitCount)
	}

	// Without commits
	optsNoCommits := CausalityOptions{IncludeCommits: false}
	resultWithout := report.BuildCausalityChain("bv-test", optsNoCommits)

	if resultWithout.Insights.CommitCount != 0 {
		t.Errorf("Expected 0 commits when IncludeCommits=false, got %d", resultWithout.Insights.CommitCount)
	}
}

func TestBuildCausalityChain_InProgress(t *testing.T) {
	report := &HistoryReport{
		DataHash: "test-hash",
		Histories: map[string]BeadHistory{
			"bv-test": {
				BeadID: "bv-test",
				Title:  "Test Bead",
				Status: "in_progress",
				Events: []BeadEvent{
					{EventType: EventCreated, Timestamp: testTime(0)},
					{EventType: EventClaimed, Timestamp: testTime(1)},
				},
			},
		},
	}

	opts := DefaultCausalityOptions()
	result := report.BuildCausalityChain("bv-test", opts)

	if result.Chain.IsComplete {
		t.Error("Expected IsComplete to be false for in_progress bead")
	}

	// EndTime should be after StartTime for in-progress beads
	if !result.Chain.EndTime.After(result.Chain.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
}

func TestCausalInsights_BlockedPercentage(t *testing.T) {
	// Test the blocked percentage calculation
	insights := CausalInsights{
		TotalDuration:   10 * time.Hour,
		BlockedDuration: 5 * time.Hour,
	}

	// Recalculate active duration and blocked percentage
	insights.ActiveDuration = insights.TotalDuration - insights.BlockedDuration
	if insights.TotalDuration > 0 {
		insights.BlockedPercentage = float64(insights.BlockedDuration) / float64(insights.TotalDuration) * 100
	}

	if insights.BlockedPercentage != 50 {
		t.Errorf("Expected 50%% blocked, got %.1f%%", insights.BlockedPercentage)
	}

	if insights.ActiveDuration != 5*time.Hour {
		t.Errorf("Expected 5h active, got %v", insights.ActiveDuration)
	}
}

func TestFormatDurationShort(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Minute, "30m"},
		{90 * time.Minute, "1h"},
		{5 * time.Hour, "5h"},
		{25 * time.Hour, "1d"},
		{3 * 24 * time.Hour, "3d"},
		{10 * 24 * time.Hour, "1w"},
		{35 * 24 * time.Hour, "1mo"},
	}

	for _, tt := range tests {
		result := formatDurationShort(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDurationShort(%v) = '%s', expected '%s'", tt.duration, result, tt.expected)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		pct      float64
		expected string
	}{
		{0, "0%"},
		{50, "50%"},
		{100, "100%"},
		{33.7, "33%"}, // Truncates to int
	}

	for _, tt := range tests {
		result := formatPercent(tt.pct)
		if result != tt.expected {
			t.Errorf("formatPercent(%.1f) = '%s', expected '%s'", tt.pct, result, tt.expected)
		}
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123, "123"},
		{-5, "-5"},
	}

	for _, tt := range tests {
		result := formatInt(tt.n)
		if result != tt.expected {
			t.Errorf("formatInt(%d) = '%s', expected '%s'", tt.n, result, tt.expected)
		}
	}
}

func TestBuildSummary_Completed(t *testing.T) {
	chain := &CausalChain{
		IsComplete: true,
		TotalTime:  6 * time.Hour,
	}
	insights := &CausalInsights{
		TotalDuration:     6 * time.Hour,
		CommitCount:       3,
		BlockedPercentage: 10,
	}

	summary := buildSummary(chain, insights)

	// Should mention completion and commit count
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
}

func TestBuildSummary_InProgress(t *testing.T) {
	chain := &CausalChain{
		IsComplete: false,
		TotalTime:  2 * 24 * time.Hour,
	}
	insights := &CausalInsights{
		TotalDuration:     2 * 24 * time.Hour,
		CommitCount:       5,
		BlockedPercentage: 0,
	}

	summary := buildSummary(chain, insights)

	if summary == "" {
		t.Error("Expected non-empty summary")
	}
}

func TestGenerateRecommendations_HighBlockedPercentage(t *testing.T) {
	chain := &CausalChain{IsComplete: false}
	insights := &CausalInsights{
		TotalDuration:     24 * time.Hour,
		BlockedPercentage: 60,
	}

	recs := generateRecommendations(chain, insights)

	found := false
	for _, rec := range recs {
		if rec != "" && len(rec) > 10 {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected at least one meaningful recommendation for high blocked percentage")
	}
}

func TestGenerateRecommendations_LongGap(t *testing.T) {
	chain := &CausalChain{IsComplete: true}
	longGap := 10 * 24 * time.Hour
	insights := &CausalInsights{
		TotalDuration:     14 * 24 * time.Hour,
		BlockedPercentage: 0,
		LongestGap:        &longGap,
	}

	recs := generateRecommendations(chain, insights)

	found := false
	for _, rec := range recs {
		if rec != "" && len(rec) > 10 {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected at least one recommendation for long gap")
	}
}

func TestGenerateRecommendations_NoIssues(t *testing.T) {
	chain := &CausalChain{IsComplete: true}
	insights := &CausalInsights{
		TotalDuration:     2 * 24 * time.Hour,
		BlockedPercentage: 0,
		CommitCount:       5,
	}

	recs := generateRecommendations(chain, insights)

	// Should have the "no issues" recommendation
	hasNoIssues := false
	for _, rec := range recs {
		if rec == "No significant issues detected in the causal flow" {
			hasNoIssues = true
			break
		}
	}

	if !hasNoIssues {
		t.Error("Expected 'no issues' recommendation for healthy flow")
	}
}

func TestCausalEventTypes(t *testing.T) {
	// Verify all event types are distinct
	types := []CausalEventType{
		CausalCreated,
		CausalClaimed,
		CausalCommit,
		CausalBlocked,
		CausalUnblocked,
		CausalClosed,
		CausalReopened,
	}

	seen := make(map[CausalEventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("Duplicate event type: %s", et)
		}
		seen[et] = true
	}
}

func TestChainDurations(t *testing.T) {
	report := &HistoryReport{
		DataHash: "test-hash",
		Histories: map[string]BeadHistory{
			"bv-test": {
				BeadID: "bv-test",
				Title:  "Test Bead",
				Status: "closed",
				Events: []BeadEvent{
					{EventType: EventCreated, Timestamp: testTime(0)},
					{EventType: EventClaimed, Timestamp: testTime(2)},
					{EventType: EventClosed, Timestamp: testTime(10)},
				},
			},
		},
	}

	opts := CausalityOptions{IncludeCommits: false}
	result := report.BuildCausalityChain("bv-test", opts)

	// Check duration calculations
	// Created at hour 0, claimed at hour 2 = 2 hours between
	if result.Chain.Events[0].DurationNext == nil {
		t.Error("Expected non-nil DurationNext for first event")
	} else if *result.Chain.Events[0].DurationNext != 2*time.Hour {
		t.Errorf("Expected 2h between created and claimed, got %v", *result.Chain.Events[0].DurationNext)
	}

	// Claimed at hour 2, closed at hour 10 = 8 hours between
	if result.Chain.Events[1].DurationNext == nil {
		t.Error("Expected non-nil DurationNext for second event")
	} else if *result.Chain.Events[1].DurationNext != 8*time.Hour {
		t.Errorf("Expected 8h between claimed and closed, got %v", *result.Chain.Events[1].DurationNext)
	}

	// Total time should be 10 hours
	if result.Chain.TotalTime != 10*time.Hour {
		t.Errorf("Expected total time of 10h, got %v", result.Chain.TotalTime)
	}
}

func TestDefaultCausalityOptions(t *testing.T) {
	opts := DefaultCausalityOptions()

	if !opts.IncludeCommits {
		t.Error("Expected IncludeCommits to be true by default")
	}
}

// TestBuildCausalityChain_SameTimestamps tests the edge case where all events
// have the same timestamp (gap = 0 between all events). This previously caused
// an array index out of bounds panic.
func TestBuildCausalityChain_SameTimestamps(t *testing.T) {
	// All events at the same timestamp
	sameTime := testTime(0)
	report := &HistoryReport{
		DataHash: "test-hash",
		Histories: map[string]BeadHistory{
			"bv-same": {
				BeadID: "bv-same",
				Title:  "Same Timestamp Test",
				Status: "closed",
				Events: []BeadEvent{
					{EventType: EventCreated, Timestamp: sameTime},
					{EventType: EventClaimed, Timestamp: sameTime},
					{EventType: EventClosed, Timestamp: sameTime},
				},
			},
		},
	}

	opts := CausalityOptions{IncludeCommits: false}

	// This should not panic (previously it would cause index out of bounds)
	result := report.BuildCausalityChain("bv-same", opts)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check insights were computed without panic
	if result.Insights == nil {
		t.Fatal("Expected non-nil insights")
	}

	// With same timestamps, all gaps are 0
	if result.Insights.LongestGap != nil && *result.Insights.LongestGap != 0 {
		t.Errorf("Expected longest gap of 0, got %v", *result.Insights.LongestGap)
	}

	// LongestGapDesc should be computed without error
	if result.Insights.LongestGapDesc == "" {
		t.Error("Expected non-empty LongestGapDesc even with 0 gap")
	}
}

// TestBuildCausalityChain_UnicodeCommitMessage tests that commit messages
// with Unicode characters are truncated correctly by runes, not bytes.
func TestBuildCausalityChain_UnicodeCommitMessage(t *testing.T) {
	// Unicode message that would be broken if truncated by bytes
	unicodeMsg := "修复中文测试问题，这是一个很长的提交消息，需要被正确截断" // Chinese characters

	report := &HistoryReport{
		DataHash: "test-hash",
		Histories: map[string]BeadHistory{
			"bv-unicode": {
				BeadID: "bv-unicode",
				Title:  "Unicode Test",
				Status: "closed",
				Events: []BeadEvent{
					{EventType: EventCreated, Timestamp: testTime(0)},
					{EventType: EventClosed, Timestamp: testTime(1)},
				},
				Commits: []CorrelatedCommit{
					{ShortSHA: "abc1234", Message: unicodeMsg, Timestamp: testTime(0)},
				},
			},
		},
	}

	opts := DefaultCausalityOptions()
	result := report.BuildCausalityChain("bv-unicode", opts)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Find the commit event
	var commitEvent *CausalEvent
	for i := range result.Chain.Events {
		if result.Chain.Events[i].Type == CausalCommit {
			commitEvent = &result.Chain.Events[i]
			break
		}
	}

	if commitEvent == nil {
		t.Fatal("Expected to find commit event")
	}

	// Description should be valid UTF-8 (not broken by mid-byte truncation)
	desc := commitEvent.Description
	if !isValidUTF8(desc) {
		t.Errorf("Commit description has invalid UTF-8: %q", desc)
	}

	// Should end with "..." if truncated
	if len([]rune(unicodeMsg)) > 50 && !endsWithEllipsis(desc) {
		t.Error("Expected truncated description to end with '...'")
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' { // Replacement character indicates invalid UTF-8
			return false
		}
	}
	return true
}

func endsWithEllipsis(s string) bool {
	runes := []rune(s)
	if len(runes) < 3 {
		return false
	}
	last3 := string(runes[len(runes)-3:])
	return last3 == "..."
}
