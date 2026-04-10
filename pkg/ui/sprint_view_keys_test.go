package ui

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestHandleSprintKeys_Exit(t *testing.T) {
	m := Model{
		isSprintView: true,
		focused:      focusDetail,
		theme:        DefaultTheme(lipgloss.NewRenderer(nil)),
		width:        100,
		height:       40,
	}
	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	if m.isSprintView {
		t.Fatalf("expected sprint view to exit")
	}
	if m.focused != focusList {
		t.Fatalf("focused=%v; want focusList", m.focused)
	}
}

func TestHandleSprintKeys_NextPrevSprint(t *testing.T) {
	now := time.Now().UTC()
	sprints := []model.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: now.AddDate(0, 0, -7), EndDate: now.AddDate(0, 0, -1), BeadIDs: []string{"A"}},
		{ID: "s2", Name: "Sprint 2", StartDate: now.AddDate(0, 0, -1), EndDate: now.AddDate(0, 0, 7), BeadIDs: []string{"A"}},
	}

	m := Model{
		isSprintView:   true,
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		issues:         []model.Issue{{ID: "A", Title: "Issue A", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask}},
		sprints:        sprints,
		selectedSprint: &sprints[0],
	}

	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.selectedSprint == nil || m.selectedSprint.ID != "s2" {
		t.Fatalf("after j: selected=%v; want s2", m.selectedSprint)
	}
	if m.sprintViewText == "" {
		t.Fatalf("expected sprintViewText to be populated after navigation")
	}

	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.selectedSprint == nil || m.selectedSprint.ID != "s1" {
		t.Fatalf("after k: selected=%v; want s1", m.selectedSprint)
	}
}

// =============================================================================
// truncateStrSprint Tests
// =============================================================================

func TestTruncateStrSprint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string truncated",
			input:    "hello world",
			maxLen:   8,
			expected: "hello w…",
		},
		{
			name:     "maxLen 3 no ellipsis",
			input:    "hello",
			maxLen:   3,
			expected: "hel",
		},
		{
			name:     "maxLen 2 no ellipsis",
			input:    "hello",
			maxLen:   2,
			expected: "he",
		},
		{
			name:     "maxLen 1 no ellipsis",
			input:    "hello",
			maxLen:   1,
			expected: "h",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "maxLen 0",
			input:    "hello",
			maxLen:   0,
			expected: "",
		},
		{
			name:     "unicode string truncation",
			input:    "日本語テスト",
			maxLen:   4,
			expected: "日本語…",
		},
		{
			name:     "mixed unicode",
			input:    "hello世界",
			maxLen:   6,
			expected: "hello…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateStrSprint(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateStrSprint(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// renderSprintDashboard Tests
// =============================================================================

func TestRenderSprintDashboard_NoSprintSelected(t *testing.T) {
	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		selectedSprint: nil,
	}

	result := m.renderSprintDashboard()
	if result != "No sprint selected" {
		t.Errorf("renderSprintDashboard() = %q, want 'No sprint selected'", result)
	}
}

func TestRenderSprintDashboard_BasicSprint(t *testing.T) {
	now := time.Now().UTC()
	sprint := model.Sprint{
		ID:        "s1",
		Name:      "Sprint 1",
		StartDate: now.AddDate(0, 0, -7),
		EndDate:   now.AddDate(0, 0, 7),
		BeadIDs:   []string{"A", "B"},
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		selectedSprint: &sprint,
		issues: []model.Issue{
			{ID: "A", Title: "Issue A", Status: model.StatusClosed, Priority: 1, IssueType: model.TypeTask},
			{ID: "B", Title: "Issue B", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask},
		},
	}

	result := m.renderSprintDashboard()

	// Should contain sprint name
	if !containsStr(result, "Sprint 1") {
		t.Error("Should contain sprint name 'Sprint 1'")
	}

	// Should contain progress elements
	if !containsStr(result, "Progress") {
		t.Error("Should contain 'Progress' label")
	}

	// Should contain status breakdown
	if !containsStr(result, "Status") {
		t.Error("Should contain 'Status' label")
	}
}

func TestRenderSprintDashboard_AllStatusTypes(t *testing.T) {
	now := time.Now().UTC()
	sprint := model.Sprint{
		ID:        "s1",
		Name:      "Test Sprint",
		StartDate: now.AddDate(0, 0, -14),
		EndDate:   now.AddDate(0, 0, 7),
		BeadIDs:   []string{"A", "B", "C", "D"},
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         50,
		selectedSprint: &sprint,
		issues: []model.Issue{
			{ID: "A", Title: "Closed Issue", Status: model.StatusClosed, Priority: 1, IssueType: model.TypeTask},
			{ID: "B", Title: "Open Issue", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask},
			{ID: "C", Title: "In Progress", Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeTask, UpdatedAt: now},
			{ID: "D", Title: "Blocked Issue", Status: model.StatusBlocked, Priority: 1, IssueType: model.TypeTask},
		},
	}

	result := m.renderSprintDashboard()

	// Should not panic and should produce output
	if result == "" {
		t.Error("Should produce non-empty output")
	}

	// Should contain burndown section
	if !containsStr(result, "Burndown") {
		t.Error("Should contain 'Burndown' label")
	}
}

func TestRenderSprintDashboard_AtRiskItems(t *testing.T) {
	now := time.Now().UTC()
	sprint := model.Sprint{
		ID:        "s1",
		Name:      "Sprint With Stale",
		StartDate: now.AddDate(0, 0, -14),
		EndDate:   now.AddDate(0, 0, 7),
		BeadIDs:   []string{"A", "B"},
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         50,
		selectedSprint: &sprint,
		issues: []model.Issue{
			{ID: "A", Title: "Fresh In Progress", Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeTask, UpdatedAt: now},
			{ID: "B", Title: "Stale In Progress", Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeTask, UpdatedAt: now.AddDate(0, 0, -5)},
		},
	}

	result := m.renderSprintDashboard()

	// Should contain at-risk section
	if !containsStr(result, "At Risk") {
		t.Error("Should contain 'At Risk' label")
	}

	// Should show the stale item (B was updated 5 days ago, threshold is 3)
	if !containsStr(result, "Stale In Progress") {
		t.Error("Should show stale item in at-risk section")
	}
}

func TestRenderSprintDashboard_NoAtRiskItems(t *testing.T) {
	now := time.Now().UTC()
	sprint := model.Sprint{
		ID:        "s1",
		Name:      "Sprint All Fresh",
		StartDate: now.AddDate(0, 0, -7),
		EndDate:   now.AddDate(0, 0, 7),
		BeadIDs:   []string{"A"},
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         50,
		selectedSprint: &sprint,
		issues: []model.Issue{
			{ID: "A", Title: "Fresh Item", Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeTask, UpdatedAt: now},
		},
	}

	result := m.renderSprintDashboard()

	// Should show "No at-risk items"
	if !containsStr(result, "No at-risk items") {
		t.Error("Should show 'No at-risk items' message")
	}
}

func TestRenderSprintDashboard_NarrowWidth(t *testing.T) {
	now := time.Now().UTC()
	sprint := model.Sprint{
		ID:        "s1",
		Name:      "Narrow Sprint",
		StartDate: now.AddDate(0, 0, -7),
		EndDate:   now.AddDate(0, 0, 7),
		BeadIDs:   []string{"A"},
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          30, // Very narrow
		height:         40,
		selectedSprint: &sprint,
		issues: []model.Issue{
			{ID: "A", Title: "Test Issue", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask},
		},
	}

	// Should not panic with narrow width
	result := m.renderSprintDashboard()
	if result == "" {
		t.Error("Should produce output even with narrow width")
	}
}

func TestRenderSprintDashboard_ZeroDaysRemaining(t *testing.T) {
	now := time.Now().UTC()
	sprint := model.Sprint{
		ID:        "s1",
		Name:      "Sprint Ending",
		StartDate: now.AddDate(0, 0, -14),
		EndDate:   now, // Ending today
		BeadIDs:   []string{"A"},
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		selectedSprint: &sprint,
		issues: []model.Issue{
			{ID: "A", Title: "Test Issue", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask},
		},
	}

	result := m.renderSprintDashboard()

	// Should show 0 days remaining
	if !containsStr(result, "0 days") {
		t.Error("Should show '0 days' remaining")
	}
}

func TestRenderSprintDashboard_ManyBeads(t *testing.T) {
	now := time.Now().UTC()
	beadIDs := make([]string, 15)
	issues := make([]model.Issue, 15)
	for i := 0; i < 15; i++ {
		beadIDs[i] = string(rune('A' + i))
		issues[i] = model.Issue{
			ID:        beadIDs[i],
			Title:     "Issue " + beadIDs[i],
			Status:    model.StatusOpen,
			Priority:  1,
			IssueType: model.TypeTask,
		}
	}

	sprint := model.Sprint{
		ID:        "s1",
		Name:      "Large Sprint",
		StartDate: now.AddDate(0, 0, -7),
		EndDate:   now.AddDate(0, 0, 7),
		BeadIDs:   beadIDs,
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         50,
		selectedSprint: &sprint,
		issues:         issues,
	}

	result := m.renderSprintDashboard()

	// Should show truncated list with "+X more"
	if !containsStr(result, "more") {
		t.Error("Should show '+X more' for large bead lists")
	}
}

func TestRenderSprintDashboard_InsufficientData(t *testing.T) {
	now := time.Now().UTC()
	sprint := model.Sprint{
		ID:      "s1",
		Name:    "Sprint No Dates",
		BeadIDs: []string{},
		// No start/end dates
	}

	m := Model{
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		selectedSprint: &sprint,
		issues:         []model.Issue{},
	}
	_ = now // unused but kept for consistency

	result := m.renderSprintDashboard()

	// Should show "insufficient data" for burndown
	if !containsStr(result, "insufficient data") {
		t.Error("Should show 'insufficient data' message")
	}
}

func TestHandleSprintKeys_EscExit(t *testing.T) {
	m := Model{
		isSprintView: true,
		focused:      focusDetail,
		theme:        DefaultTheme(lipgloss.NewRenderer(nil)),
		width:        100,
		height:       40,
	}

	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyEsc})
	if m.isSprintView {
		t.Error("Esc should exit sprint view")
	}
	if m.focused != focusList {
		t.Errorf("focused=%v; want focusList", m.focused)
	}
}

func TestHandleSprintKeys_DownArrow(t *testing.T) {
	now := time.Now().UTC()
	sprints := []model.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: now.AddDate(0, 0, -7), EndDate: now.AddDate(0, 0, -1), BeadIDs: []string{"A"}},
		{ID: "s2", Name: "Sprint 2", StartDate: now.AddDate(0, 0, -1), EndDate: now.AddDate(0, 0, 7), BeadIDs: []string{"A"}},
	}

	m := Model{
		isSprintView:   true,
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		issues:         []model.Issue{{ID: "A", Title: "Issue A", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask}},
		sprints:        sprints,
		selectedSprint: &sprints[0],
	}

	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyDown})
	if m.selectedSprint == nil || m.selectedSprint.ID != "s2" {
		t.Errorf("after down: selected=%v; want s2", m.selectedSprint)
	}
}

func TestHandleSprintKeys_UpArrow(t *testing.T) {
	now := time.Now().UTC()
	sprints := []model.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: now.AddDate(0, 0, -7), EndDate: now.AddDate(0, 0, -1), BeadIDs: []string{"A"}},
		{ID: "s2", Name: "Sprint 2", StartDate: now.AddDate(0, 0, -1), EndDate: now.AddDate(0, 0, 7), BeadIDs: []string{"A"}},
	}

	m := Model{
		isSprintView:   true,
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		issues:         []model.Issue{{ID: "A", Title: "Issue A", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask}},
		sprints:        sprints,
		selectedSprint: &sprints[1],
	}

	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyUp})
	if m.selectedSprint == nil || m.selectedSprint.ID != "s1" {
		t.Errorf("after up: selected=%v; want s1", m.selectedSprint)
	}
}

func TestHandleSprintKeys_BoundaryAtFirst(t *testing.T) {
	now := time.Now().UTC()
	sprints := []model.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: now.AddDate(0, 0, -7), EndDate: now.AddDate(0, 0, -1), BeadIDs: []string{"A"}},
	}

	m := Model{
		isSprintView:   true,
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		issues:         []model.Issue{{ID: "A", Title: "Issue A", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask}},
		sprints:        sprints,
		selectedSprint: &sprints[0],
	}

	// Try to go up when already at first
	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyUp})
	if m.selectedSprint == nil || m.selectedSprint.ID != "s1" {
		t.Errorf("Should stay at s1 when at boundary")
	}
}

func TestHandleSprintKeys_BoundaryAtLast(t *testing.T) {
	now := time.Now().UTC()
	sprints := []model.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: now.AddDate(0, 0, -7), EndDate: now.AddDate(0, 0, -1), BeadIDs: []string{"A"}},
	}

	m := Model{
		isSprintView:   true,
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		issues:         []model.Issue{{ID: "A", Title: "Issue A", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask}},
		sprints:        sprints,
		selectedSprint: &sprints[0],
	}

	// Try to go down when already at last
	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyDown})
	if m.selectedSprint == nil || m.selectedSprint.ID != "s1" {
		t.Errorf("Should stay at s1 when at boundary")
	}
}

func TestHandleSprintKeys_NilSelectedSprint(t *testing.T) {
	now := time.Now().UTC()
	sprints := []model.Sprint{
		{ID: "s1", Name: "Sprint 1", StartDate: now.AddDate(0, 0, -7), EndDate: now.AddDate(0, 0, -1), BeadIDs: []string{"A"}},
	}

	m := Model{
		isSprintView:   true,
		theme:          DefaultTheme(lipgloss.NewRenderer(nil)),
		width:          100,
		height:         40,
		sprints:        sprints,
		selectedSprint: nil, // No sprint selected
	}

	// Should not panic
	m = m.handleSprintKeys(tea.KeyMsg{Type: tea.KeyDown})
	if m.selectedSprint != nil {
		t.Errorf("Should remain nil when no sprint selected")
	}
}

// Helper function
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
