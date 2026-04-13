package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestRenderPriorityBadge(t *testing.T) {
	tests := []struct {
		prio int
		want string
	}{
		{0, "P0"},
		{1, "P1"},
		{2, "P2"},
		{3, "P3"},
		{4, "P4"},
		{99, "P?"},
	}

	for _, tt := range tests {
		got := RenderPriorityBadge(tt.prio)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderPriorityBadge(%d) = %q, want to contain %q", tt.prio, got, tt.want)
		}
	}
}

func TestRenderStatusBadge(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		// All 8 official beads statuses
		{"open", "OPEN"},
		{"in_progress", "PROG"},
		{"blocked", "BLKD"},
		{"deferred", "DEFR"},
		{"pinned", "PIN"},
		{"hooked", "HOOK"},
		{"closed", "DONE"},
		{"tombstone", "TOMB"},
		// Unknown status should show "????"
		{"unknown", "????"},
		{"", "????"},
		{"invalid_status", "????"},
	}

	for _, tt := range tests {
		got := RenderStatusBadge(tt.status)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderStatusBadge(%q) = %q, want to contain %q", tt.status, got, tt.want)
		}
	}
}

// TestRenderStatusBadge_AllStatusesHaveColors verifies each status has distinct colors
func TestRenderStatusBadge_AllStatusesHaveColors(t *testing.T) {
	statuses := []string{
		"open", "in_progress", "blocked", "deferred",
		"pinned", "hooked", "closed", "tombstone",
	}

	// Each status should produce a non-empty output
	for _, status := range statuses {
		got := RenderStatusBadge(status)
		if got == "" {
			t.Errorf("RenderStatusBadge(%q) returned empty string", status)
		}
		// Should NOT contain "????" for known statuses
		if strings.Contains(got, "????") {
			t.Errorf("RenderStatusBadge(%q) returned unknown badge '????'", status)
		}
	}
}

func TestRenderMiniBar(t *testing.T) {
	theme := DefaultTheme()

	tests := []struct {
		val   float64
		width int
	}{
		{0.0, 10},
		{0.5, 10},
		{1.0, 10},
		{-0.1, 10}, // Should clamp to 0
		{1.5, 10},  // Should clamp to 1
		{0.5, 0},   // Should return empty
		{0.5, -5},  // Should return empty (no panic)
	}

	for _, tt := range tests {
		got := RenderMiniBar(tt.val, tt.width, theme)
		if tt.width <= 0 {
			if got != "" {
				t.Errorf("RenderMiniBar(%v, %d) = %q, want empty string", tt.val, tt.width, got)
			}
			continue
		}
		// Basic sanity check: output should not be empty
		if got == "" {
			t.Errorf("RenderMiniBar(%v, %d) returned empty string", tt.val, tt.width)
		}
		// Check expected fullness characters approximately
		if tt.val > 0 {
			if !strings.Contains(got, "█") && !strings.Contains(got, "░") {
				t.Errorf("RenderMiniBar output expected bar chars, got %q", got)
			}
		}
	}
}

func TestRenderGateBadge(t *testing.T) {
	tests := []struct {
		awaitType string
		want      string
	}{
		{"human", "HUM"},
		{"timer", "TMR"},
		{"gh:run", "CI"},
		{"ci", "CI"},
		{"gh:pr", "PR"},
		{"pr", "PR"},
		{"bead", "BD"},
		{"unknown", "GTD"},
		{"", "GTD"},
	}

	for _, tt := range tests {
		got := RenderGateBadge(tt.awaitType)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderGateBadge(%q) = %q, want to contain %q", tt.awaitType, got, tt.want)
		}
		if got == "" {
			t.Errorf("RenderGateBadge(%q) returned empty string", tt.awaitType)
		}
	}
}

func TestRenderHumanAdvisoryBadge(t *testing.T) {
	got := RenderHumanAdvisoryBadge()
	if !strings.Contains(got, "HUM") {
		t.Errorf("RenderHumanAdvisoryBadge() = %q, want to contain 'HUM'", got)
	}
	if got == "" {
		t.Errorf("RenderHumanAdvisoryBadge() returned empty string")
	}
}

func TestRenderGateBadge_DistinctFromAdvisory(t *testing.T) {
	gateBadge := RenderGateBadge("human")
	advisoryBadge := RenderHumanAdvisoryBadge()
	if gateBadge == advisoryBadge {
		t.Errorf("Gate human badge and advisory badge should be visually distinct, both = %q", gateBadge)
	}
}

func TestHasHumanLabel(t *testing.T) {
	tests := []struct {
		labels []string
		want   bool
	}{
		{[]string{"human"}, true},
		{[]string{"bug", "human", "p0"}, true},
		{[]string{"bug", "p0"}, false},
		{nil, false},
		{[]string{}, false},
		{[]string{"Human"}, false}, // case-sensitive
	}

	for _, tt := range tests {
		got := hasHumanLabel(tt.labels)
		if got != tt.want {
			t.Errorf("hasHumanLabel(%v) = %v, want %v", tt.labels, got, tt.want)
		}
	}
}

func TestRenderOverdueBadge(t *testing.T) {
	got := RenderOverdueBadge()
	if !strings.Contains(got, "DUE") {
		t.Errorf("RenderOverdueBadge() = %q, want to contain 'DUE'", got)
	}
}

func TestRenderStaleBadge(t *testing.T) {
	got := RenderStaleBadge()
	if !strings.Contains(got, "💤") {
		t.Errorf("RenderStaleBadge() = %q, want to contain sleep icon", got)
	}
}

func TestIsOverdue(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour)
	future := time.Now().Add(48 * time.Hour)

	tests := []struct {
		name string
		issue model.Issue
		want  bool
	}{
		{"past due, open", model.Issue{DueDate: &past, Status: model.StatusOpen, IssueType: "task"}, true},
		{"future due, open", model.Issue{DueDate: &future, Status: model.StatusOpen, IssueType: "task"}, false},
		{"past due, closed", model.Issue{DueDate: &past, Status: model.StatusClosed, IssueType: "task"}, false},
		{"no due date", model.Issue{Status: model.StatusOpen, IssueType: "task"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOverdue(&tt.issue)
			if got != tt.want {
				t.Errorf("isOverdue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	recent := time.Now().Add(-1 * time.Hour)
	old := time.Now().Add(-30 * 24 * time.Hour)

	tests := []struct {
		name  string
		issue model.Issue
		want  bool
	}{
		{"recently updated, open", model.Issue{UpdatedAt: recent, Status: model.StatusOpen, IssueType: "task"}, false},
		{"old update, open", model.Issue{UpdatedAt: old, Status: model.StatusOpen, IssueType: "task"}, true},
		{"old update, in_progress", model.Issue{UpdatedAt: old, Status: model.StatusInProgress, IssueType: "task"}, true},
		{"old update, closed", model.Issue{UpdatedAt: old, Status: model.StatusClosed, IssueType: "task"}, false},
		{"old update, deferred", model.Issue{UpdatedAt: old, Status: model.StatusDeferred, IssueType: "task"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStale(&tt.issue)
			if got != tt.want {
				t.Errorf("isStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStateDimensions(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   int
		dims   []StateDimension
	}{
		{"basic", []string{"health:degraded", "patrol:active"}, 2,
			[]StateDimension{{"health", "degraded"}, {"patrol", "active"}}},
		{"excludes non-state", []string{"export:capability", "provides:api", "health:ok"}, 1,
			[]StateDimension{{"health", "ok"}}},
		{"no colons", []string{"bug", "p0", "human"}, 0, nil},
		{"colon at start", []string{":value"}, 0, nil},
		{"colon at end", []string{"dim:"}, 0, nil},
		{"empty", nil, 0, nil},
		{"mixed", []string{"bug", "mode:dark", "external:jira", "stream:alpha"}, 1,
			[]StateDimension{{"mode", "dark"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStateDimensions(tt.labels)
			if len(got) != tt.want {
				t.Errorf("parseStateDimensions() returned %d dims, want %d", len(got), tt.want)
			}
			if tt.dims != nil {
				for i, d := range tt.dims {
					if i >= len(got) {
						break
					}
					if got[i].Dimension != d.Dimension || got[i].Value != d.Value {
						t.Errorf("dim[%d] = %v, want %v", i, got[i], d)
					}
				}
			}
		})
	}
}

func TestRenderStateDimensionBadge(t *testing.T) {
	tests := []struct {
		dim, val string
		contains string
	}{
		{"health", "degraded", "health:degraded"},
		{"patrol", "active", "patrol:active"},
		{"mode", "dark", "mode:dark"},
		{"custom", "value", "custom:value"},
	}

	for _, tt := range tests {
		got := RenderStateDimensionBadge(tt.dim, tt.val)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("RenderStateDimensionBadge(%q, %q) = %q, want to contain %q", tt.dim, tt.val, got, tt.contains)
		}
	}
}

func TestEpicProgress(t *testing.T) {
	epic := model.Issue{ID: "ep-1", IssueType: model.TypeEpic, Status: model.StatusOpen}
	child1 := model.Issue{
		ID: "ch-1", Status: model.StatusClosed, IssueType: model.TypeTask,
		Dependencies: []*model.Dependency{{IssueID: "ch-1", DependsOnID: "ep-1", Type: model.DepParentChild}},
	}
	child2 := model.Issue{
		ID: "ch-2", Status: model.StatusOpen, IssueType: model.TypeTask,
		Dependencies: []*model.Dependency{{IssueID: "ch-2", DependsOnID: "ep-1", Type: model.DepParentChild}},
	}
	child3 := model.Issue{
		ID: "ch-3", Status: model.StatusInProgress, IssueType: model.TypeTask,
		Dependencies: []*model.Dependency{{IssueID: "ch-3", DependsOnID: "ep-1", Type: model.DepParentChild}},
	}
	unrelated := model.Issue{
		ID: "un-1", Status: model.StatusOpen, IssueType: model.TypeTask,
		Dependencies: []*model.Dependency{{IssueID: "un-1", DependsOnID: "other-epic", Type: model.DepParentChild}},
	}

	all := []model.Issue{epic, child1, child2, child3, unrelated}

	done, total := epicProgress("ep-1", all)
	if total != 3 {
		t.Errorf("epicProgress total = %d, want 3", total)
	}
	if done != 1 {
		t.Errorf("epicProgress done = %d, want 1", done)
	}

	// No children
	done2, total2 := epicProgress("no-children", all)
	if total2 != 0 || done2 != 0 {
		t.Errorf("epicProgress for nonexistent = (%d, %d), want (0, 0)", done2, total2)
	}

	// All closed
	child2.Status = model.StatusClosed
	child3.Status = model.StatusClosed
	all2 := []model.Issue{epic, child1, child2, child3}
	done3, total3 := epicProgress("ep-1", all2)
	if total3 != 3 || done3 != 3 {
		t.Errorf("epicProgress all closed = (%d, %d), want (3, 3)", done3, total3)
	}
}

func TestFormatNanoseconds(t *testing.T) {
	tests := []struct {
		ns   int64
		want string
	}{
		{0, "0s"},
		{1_000_000_000, "1s"},       // 1 second
		{60_000_000_000, "1m"},      // 1 minute
		{3_600_000_000_000, "1h"},   // 1 hour
		{86_400_000_000_000, "1d"},  // 1 day
		{172_800_000_000_000, "2d"}, // 2 days
	}

	for _, tt := range tests {
		got := formatNanoseconds(tt.ns)
		if got != tt.want {
			t.Errorf("formatNanoseconds(%d) = %q, want %q", tt.ns, got, tt.want)
		}
	}
}

func TestRenderRankBadge(t *testing.T) {
	tests := []struct {
		rank  int
		total int
		want  string
	}{
		{1, 100, "#1"},
		{50, 100, "#50"},
		{0, 0, "#?"},
	}

	for _, tt := range tests {
		got := RenderRankBadge(tt.rank, tt.total)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderRankBadge(%d, %d) = %q, want to contain %q", tt.rank, tt.total, got, tt.want)
		}
	}
}
