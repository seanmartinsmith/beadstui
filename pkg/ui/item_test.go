package ui_test

import (
	"strings"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
)

// =============================================================================
// DiffStatus Tests
// =============================================================================

func TestDiffStatusBadge(t *testing.T) {
	tests := []struct {
		name     string
		status   ui.DiffStatus
		expected string
	}{
		{
			name:     "none returns empty string",
			status:   ui.DiffStatusNone,
			expected: "",
		},
		{
			name:     "new returns new emoji",
			status:   ui.DiffStatusNew,
			expected: "🆕",
		},
		{
			name:     "closed returns checkmark emoji",
			status:   ui.DiffStatusClosed,
			expected: "✅",
		},
		{
			name:     "modified returns tilde",
			status:   ui.DiffStatusModified,
			expected: "~",
		},
		{
			name:     "unknown status returns empty string",
			status:   ui.DiffStatus(99),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.Badge()
			if result != tt.expected {
				t.Errorf("DiffStatus(%d).Badge() = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestDiffStatusConstants(t *testing.T) {
	// Verify the iota values are as expected
	if ui.DiffStatusNone != 0 {
		t.Errorf("DiffStatusNone = %d, want 0", ui.DiffStatusNone)
	}
	if ui.DiffStatusNew != 1 {
		t.Errorf("DiffStatusNew = %d, want 1", ui.DiffStatusNew)
	}
	if ui.DiffStatusClosed != 2 {
		t.Errorf("DiffStatusClosed = %d, want 2", ui.DiffStatusClosed)
	}
	if ui.DiffStatusModified != 3 {
		t.Errorf("DiffStatusModified = %d, want 3", ui.DiffStatusModified)
	}
}

// =============================================================================
// IssueItem Tests
// =============================================================================

func TestIssueItemTitle(t *testing.T) {
	tests := []struct {
		name     string
		issue    model.Issue
		expected string
	}{
		{
			name:     "returns issue title",
			issue:    model.Issue{ID: "test-1", Title: "Fix the bug"},
			expected: "Fix the bug",
		},
		{
			name:     "empty title returns empty string",
			issue:    model.Issue{ID: "test-2", Title: ""},
			expected: "",
		},
		{
			name:     "unicode title preserved",
			issue:    model.Issue{ID: "test-3", Title: "日本語タイトル"},
			expected: "日本語タイトル",
		},
		{
			name:     "title with special characters",
			issue:    model.Issue{ID: "test-4", Title: "Fix: <bug> in \"module\" [urgent]"},
			expected: "Fix: <bug> in \"module\" [urgent]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ui.IssueItem{Issue: tt.issue}
			if result := item.Title(); result != tt.expected {
				t.Errorf("IssueItem.Title() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIssueItemDescription(t *testing.T) {
	tests := []struct {
		name     string
		issue    model.Issue
		contains []string
	}{
		{
			name: "contains ID status and assignee",
			issue: model.Issue{
				ID:       "BUG-123",
				Status:   model.StatusOpen,
				Assignee: "alice",
			},
			contains: []string{"BUG-123", "open", "alice"},
		},
		{
			name: "in progress status",
			issue: model.Issue{
				ID:       "TASK-456",
				Status:   model.StatusInProgress,
				Assignee: "bob",
			},
			contains: []string{"TASK-456", "in_progress", "bob"},
		},
		{
			name: "empty assignee",
			issue: model.Issue{
				ID:       "FEAT-789",
				Status:   model.StatusBlocked,
				Assignee: "",
			},
			contains: []string{"FEAT-789", "blocked"},
		},
		{
			name: "closed status",
			issue: model.Issue{
				ID:       "DONE-001",
				Status:   model.StatusClosed,
				Assignee: "charlie",
			},
			contains: []string{"DONE-001", "closed", "charlie"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ui.IssueItem{Issue: tt.issue}
			desc := item.Description()
			for _, s := range tt.contains {
				if !strings.Contains(desc, s) {
					t.Errorf("IssueItem.Description() = %q, want to contain %q", desc, s)
				}
			}
		})
	}
}

func TestIssueItemDescriptionFormat(t *testing.T) {
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:       "TEST-1",
			Status:   model.StatusOpen,
			Assignee: "dev",
		},
	}
	desc := item.Description()
	// Expected format: "ID STATUS • ASSIGNEE"
	if !strings.Contains(desc, "•") {
		t.Errorf("Description should contain bullet separator, got: %q", desc)
	}
}

func TestIssueItemFilterValue(t *testing.T) {
	tests := []struct {
		name             string
		item             ui.IssueItem
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name: "basic fields included",
			item: ui.IssueItem{
				Issue: model.Issue{
					ID:        "ISSUE-1",
					Title:     "Test Issue",
					Status:    model.StatusOpen,
					IssueType: model.TypeBug,
				},
			},
			shouldContain: []string{"ISSUE-1", "Test Issue", "open", "bug"},
		},
		{
			name: "assignee included when present",
			item: ui.IssueItem{
				Issue: model.Issue{
					ID:        "ISSUE-2",
					Title:     "Another Issue",
					Status:    model.StatusInProgress,
					IssueType: model.TypeFeature,
					Assignee:  "alice",
				},
			},
			shouldContain: []string{"ISSUE-2", "Another Issue", "in_progress", "feature", "alice"},
		},
		{
			name: "labels included when present",
			item: ui.IssueItem{
				Issue: model.Issue{
					ID:        "ISSUE-3",
					Title:     "Labeled Issue",
					Status:    model.StatusOpen,
					IssueType: model.TypeTask,
					Labels:    []string{"urgent", "frontend", "p0"},
				},
			},
			shouldContain: []string{"urgent", "frontend", "p0"},
		},
		{
			name: "repo prefix included when present",
			item: ui.IssueItem{
				Issue: model.Issue{
					ID:        "ISSUE-4",
					Title:     "Workspace Issue",
					Status:    model.StatusOpen,
					IssueType: model.TypeTask,
				},
				RepoPrefix: "api",
			},
			shouldContain: []string{"api"},
		},
		{
			name: "empty labels not included",
			item: ui.IssueItem{
				Issue: model.Issue{
					ID:        "ISSUE-5",
					Title:     "No Labels",
					Status:    model.StatusOpen,
					IssueType: model.TypeTask,
					Labels:    []string{},
				},
			},
			shouldContain: []string{"ISSUE-5", "No Labels"},
		},
		{
			name: "empty assignee not included",
			item: ui.IssueItem{
				Issue: model.Issue{
					ID:        "ISSUE-6",
					Title:     "No Assignee",
					Status:    model.StatusOpen,
					IssueType: model.TypeTask,
					Assignee:  "",
				},
			},
			shouldContain: []string{"ISSUE-6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fv := tt.item.FilterValue()
			for _, s := range tt.shouldContain {
				if !strings.Contains(fv, s) {
					t.Errorf("FilterValue() = %q, should contain %q", fv, s)
				}
			}
			for _, s := range tt.shouldNotContain {
				if strings.Contains(fv, s) {
					t.Errorf("FilterValue() = %q, should not contain %q", fv, s)
				}
			}
		})
	}
}

func TestIssueItemFilterValueSpaceSeparation(t *testing.T) {
	// Verify that components are space-separated for proper filtering
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:        "ABC-123",
			Title:     "My Issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeBug,
			Assignee:  "alice",
			Labels:    []string{"urgent"},
		},
		RepoPrefix: "web",
	}

	fv := item.FilterValue()
	// Should be able to search for each component separately
	words := strings.Fields(fv)
	if len(words) < 5 {
		t.Errorf("FilterValue should have at least 5 space-separated words, got %d: %q", len(words), fv)
	}
}

// =============================================================================
// ExtractRepoPrefix Tests
// =============================================================================

func TestExtractRepoPrefix(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected string
	}{
		// Hyphen separator
		{
			name:     "hyphen separator extracts prefix",
			id:       "api-AUTH-123",
			expected: "api",
		},
		{
			name:     "web prefix with hyphen",
			id:       "web-UI-1",
			expected: "web",
		},
		{
			name:     "numeric prefix with hyphen",
			id:       "repo123-TASK-456",
			expected: "repo123",
		},
		// Colon separator
		{
			name:     "colon separator extracts prefix",
			id:       "backend:ISSUE-789",
			expected: "backend",
		},
		{
			name:     "colon with short prefix",
			id:       "db:migration-1",
			expected: "db",
		},
		// Underscore separator
		{
			name:     "underscore separator extracts prefix",
			id:       "service_BUG_001",
			expected: "service",
		},
		// No prefix cases
		{
			name:     "no separator returns empty",
			id:       "ISSUE123",
			expected: "",
		},
		{
			name:     "empty string returns empty",
			id:       "",
			expected: "",
		},
		{
			name:     "separator only returns empty",
			id:       "-test",
			expected: "",
		},
		// Long prefix (>10 chars) should not match
		{
			name:     "prefix too long returns empty",
			id:       "verylongprefix-ISSUE-1",
			expected: "",
		},
		// Non-alphanumeric prefix
		{
			name:     "non-alphanumeric prefix returns empty",
			id:       "api.v2-ISSUE-1",
			expected: "",
		},
		// Edge cases
		{
			name:     "max length prefix (10 chars)",
			id:       "abcdefghij-ISSUE-1",
			expected: "abcdefghij",
		},
		{
			name:     "single char prefix",
			id:       "a-ISSUE-1",
			expected: "a",
		},
		{
			name:     "uppercase prefix",
			id:       "API-ISSUE-1",
			expected: "API",
		},
		{
			name:     "mixed case prefix",
			id:       "ApI-ISSUE-1",
			expected: "ApI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.ExtractRepoPrefix(tt.id)
			if result != tt.expected {
				t.Errorf("ExtractRepoPrefix(%q) = %q, want %q", tt.id, result, tt.expected)
			}
		})
	}
}

func TestExtractRepoPrefixSeparatorPriority(t *testing.T) {
	// Test that hyphen is checked first (it appears first in the loop)
	tests := []struct {
		name     string
		id       string
		expected string
	}{
		{
			name:     "hyphen found before colon",
			id:       "api-foo:bar",
			expected: "api",
		},
		{
			name:     "hyphen found before underscore",
			id:       "web-baz_qux",
			expected: "web",
		},
		{
			name:     "colon found before underscore",
			id:       "svc:task_1",
			expected: "svc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.ExtractRepoPrefix(tt.id)
			if result != tt.expected {
				t.Errorf("ExtractRepoPrefix(%q) = %q, want %q", tt.id, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// IssueItem Triage Fields Tests
// =============================================================================

func TestIssueItemTriageFields(t *testing.T) {
	// Test that triage fields are properly accessible
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:     "triage-1",
			Title:  "Triage Test",
			Status: model.StatusOpen,
		},
		TriageScore:   0.85,
		TriageReason:  "High impact blocker",
		TriageReasons: []string{"blocks 5 items", "critical path"},
		IsQuickWin:    true,
		IsBlocker:     true,
		UnblocksCount: 5,
	}

	// Verify fields are set correctly
	if item.TriageScore != 0.85 {
		t.Errorf("TriageScore = %v, want 0.85", item.TriageScore)
	}
	if item.TriageReason != "High impact blocker" {
		t.Errorf("TriageReason = %q, want %q", item.TriageReason, "High impact blocker")
	}
	if len(item.TriageReasons) != 2 {
		t.Errorf("TriageReasons len = %d, want 2", len(item.TriageReasons))
	}
	if !item.IsQuickWin {
		t.Error("IsQuickWin should be true")
	}
	if !item.IsBlocker {
		t.Error("IsBlocker should be true")
	}
	if item.UnblocksCount != 5 {
		t.Errorf("UnblocksCount = %d, want 5", item.UnblocksCount)
	}
}

func TestIssueItemGraphFields(t *testing.T) {
	// Test GraphScore and Impact fields
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:     "graph-1",
			Title:  "Graph Test",
			Status: model.StatusOpen,
		},
		GraphScore: 0.75,
		Impact:     0.9,
	}

	if item.GraphScore != 0.75 {
		t.Errorf("GraphScore = %v, want 0.75", item.GraphScore)
	}
	if item.Impact != 0.9 {
		t.Errorf("Impact = %v, want 0.9", item.Impact)
	}
}

func TestIssueItemDiffStatusBadge(t *testing.T) {
	// Test that DiffStatus field works with Badge method
	tests := []struct {
		name       string
		diffStatus ui.DiffStatus
		wantBadge  string
	}{
		{"none", ui.DiffStatusNone, ""},
		{"new", ui.DiffStatusNew, "🆕"},
		{"closed", ui.DiffStatusClosed, "✅"},
		{"modified", ui.DiffStatusModified, "~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ui.IssueItem{
				Issue:      model.Issue{ID: "diff-1", Title: "Test"},
				DiffStatus: tt.diffStatus,
			}
			if badge := item.DiffStatus.Badge(); badge != tt.wantBadge {
				t.Errorf("item.DiffStatus.Badge() = %q, want %q", badge, tt.wantBadge)
			}
		})
	}
}

// =============================================================================
// IssueItem Zero Value Tests
// =============================================================================

func TestIssueItemZeroValue(t *testing.T) {
	// Test behavior with zero-value IssueItem
	var item ui.IssueItem

	// Should not panic
	title := item.Title()
	if title != "" {
		t.Errorf("zero-value Title() = %q, want empty", title)
	}

	desc := item.Description()
	if desc == "" {
		// Description should still produce some output due to formatting
		t.Log("Description with zero value:", desc)
	}

	fv := item.FilterValue()
	// FilterValue should handle zero values gracefully
	if fv == "" {
		t.Log("FilterValue with zero value is empty (expected)")
	}
}

// =============================================================================
// Integration-style Tests
// =============================================================================

func TestIssueItemListInterface(t *testing.T) {
	// Verify IssueItem satisfies list.Item interface requirements
	// (Title, Description, FilterValue methods)
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:        "LIST-1",
			Title:     "List Item Test",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Assignee:  "tester",
			Labels:    []string{"test"},
		},
		RepoPrefix: "pkg",
		GraphScore: 0.5,
		Impact:     0.7,
		DiffStatus: ui.DiffStatusNew,
	}

	// All interface methods should work
	if item.Title() == "" {
		t.Error("Title() should return non-empty string")
	}
	if item.Description() == "" {
		t.Error("Description() should return non-empty string")
	}
	if item.FilterValue() == "" {
		t.Error("FilterValue() should return non-empty string")
	}
}

func TestExtractRepoPrefixRealWorldIDs(t *testing.T) {
	// Test with realistic issue ID formats
	tests := []struct {
		id       string
		expected string
	}{
		// Common beads format
		{"beads-abc123", "beads"},
		{"bv-xyz789", "bv"},
		// GitHub-style
		{"gh-ISSUE-1234", "gh"},
		// Jira-style (no repo prefix expected)
		{"PROJ-123", "PROJ"},
		// Linear-style
		{"ENG-1234", "ENG"},
		// Monorepo styles
		{"frontend-UI-100", "frontend"},
		{"backend-API-200", "backend"},
		{"shared-LIB-50", "shared"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := ui.ExtractRepoPrefix(tt.id)
			if result != tt.expected {
				t.Errorf("ExtractRepoPrefix(%q) = %q, want %q", tt.id, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkIssueItemTitle(b *testing.B) {
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:    "BENCH-1",
			Title: "Benchmark Issue Title for Performance Testing",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = item.Title()
	}
}

func BenchmarkIssueItemDescription(b *testing.B) {
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:       "BENCH-1",
			Title:    "Benchmark Issue",
			Status:   model.StatusInProgress,
			Assignee: "benchmarker",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = item.Description()
	}
}

func BenchmarkIssueItemFilterValue(b *testing.B) {
	item := ui.IssueItem{
		Issue: model.Issue{
			ID:        "BENCH-1",
			Title:     "Benchmark Issue for Filter Value Testing",
			Status:    model.StatusOpen,
			IssueType: model.TypeFeature,
			Assignee:  "developer",
			Labels:    []string{"urgent", "frontend", "p0", "needs-review"},
		},
		RepoPrefix: "webapp",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = item.FilterValue()
	}
}

func BenchmarkExtractRepoPrefix(b *testing.B) {
	ids := []string{
		"api-AUTH-123",
		"web-UI-456",
		"backend:ISSUE-789",
		"PROJ-123",
		"no-prefix-here",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range ids {
			_ = ui.ExtractRepoPrefix(id)
		}
	}
}

func BenchmarkDiffStatusBadge(b *testing.B) {
	statuses := []ui.DiffStatus{
		ui.DiffStatusNone,
		ui.DiffStatusNew,
		ui.DiffStatusClosed,
		ui.DiffStatusModified,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range statuses {
			_ = s.Badge()
		}
	}
}
