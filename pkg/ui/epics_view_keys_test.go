package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// epicsTestModel builds a fully-initialized ViewEpics model (keys, filter,
// data all wired via NewModel) from the given issues, then refreshes the
// overview rows.
func epicsTestModel(issues []model.Issue) Model {
	m := NewModel(issues, nil, "", nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = nm.(Model)
	m.mode = ViewEpics
	m.focused = focusEpics
	m.refreshEpicsForCurrentFilter()
	return m
}

func epicsFixture() []model.Issue {
	now := time.Now()
	pc := func(child, parent string) []*model.Dependency {
		return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
	}
	return []model.Issue{
		{ID: "ep1", Title: "First epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
		{ID: "ep1.a", Title: "Child A", Status: model.StatusClosed, Dependencies: pc("ep1.a", "ep1")},
		{ID: "ep1.b", Title: "Child B", Status: model.StatusInProgress, UpdatedAt: now.Add(-5 * 24 * time.Hour), Dependencies: pc("ep1.b", "ep1")},
		{ID: "ep2", Title: "Second epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
		{ID: "ep2.a", Title: "Child", Status: model.StatusInProgress, UpdatedAt: now, Dependencies: pc("ep2.a", "ep2")},
	}
}

func TestHandleEpicsKeys_Exit(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.mode == ViewEpics {
		t.Fatalf("expected epics view to exit")
	}
	if m.focused != focusList {
		t.Fatalf("focused=%v; want focusList", m.focused)
	}
}

func TestHandleEpicsKeys_Navigation(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	if len(m.epicsRows) != 2 {
		t.Fatalf("expected 2 epic rows, got %d", len(m.epicsRows))
	}
	if m.epicsCursor != 0 {
		t.Fatalf("cursor should start at 0, got %d", m.epicsCursor)
	}

	// Down moves the cursor.
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.epicsCursor != 1 {
		t.Errorf("after j: cursor=%d, want 1", m.epicsCursor)
	}
	// Clamp at the last row.
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.epicsCursor != 1 {
		t.Errorf("after second j: cursor=%d, want 1 (clamped)", m.epicsCursor)
	}
	// Up moves back.
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.epicsCursor != 0 {
		t.Errorf("after k: cursor=%d, want 0", m.epicsCursor)
	}
	// Clamp at the top.
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.epicsCursor != 0 {
		t.Errorf("after second k: cursor=%d, want 0 (clamped)", m.epicsCursor)
	}
}

func TestHandleEpicsKeys_CycleStatus(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	if m.epicsStatusMode != EpicsActive {
		t.Fatalf("default mode = %v, want EpicsActive", m.epicsStatusMode)
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.epicsStatusMode != EpicsAll {
		t.Errorf("after 1st s: mode=%v, want EpicsAll", m.epicsStatusMode)
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.epicsStatusMode != EpicsCompleted {
		t.Errorf("after 2nd s: mode=%v, want EpicsCompleted", m.epicsStatusMode)
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.epicsStatusMode != EpicsActive {
		t.Errorf("after 3rd s: mode=%v, want EpicsActive (wrapped)", m.epicsStatusMode)
	}
}

func TestRenderEpicsOverview_Basic(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	result := m.epicsViewText

	if !containsStr(result, "Epics (active)") {
		t.Error("should contain the header 'Epics (active)'")
	}
	if !containsStr(result, "ep1") {
		t.Error("should list epic ep1")
	}
	// ep1 has 1 closed of 2 children -> progress 1/2.
	if !containsStr(result, "1/2") {
		t.Error("should show ep1 progress 1/2")
	}
}

func TestRenderEpicsOverview_Empty(t *testing.T) {
	m := epicsTestModel([]model.Issue{
		{ID: "task-1", Title: "Not an epic", Status: model.StatusOpen, IssueType: model.TypeTask},
	})
	if len(m.epicsRows) != 0 {
		t.Fatalf("expected 0 epic rows, got %d", len(m.epicsRows))
	}
	if !containsStr(m.epicsViewText, "No epics in scope") {
		t.Error("empty overview should show 'No epics in scope'")
	}
}

func TestRenderEpicsOverview_AtRisk(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	// ep1.b is in_progress and 5 days stale -> ep1 row carries an at-risk marker.
	if !containsStr(m.epicsViewText, "⚠") {
		t.Error("overview should show an at-risk marker for the stale child")
	}
}

func TestRenderEpicsOverview_NarrowWidth(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m.width = 30 // very narrow
	result := m.renderEpicsOverview()
	if result == "" {
		t.Error("should produce output even with narrow width")
	}
}

// =============================================================================
// truncateString Tests
// =============================================================================

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"long string truncated", "hello world", 8, "hello w…"},
		{"maxLen 3 no ellipsis", "hello", 3, "hel"},
		{"maxLen 2 no ellipsis", "hello", 2, "he"},
		{"maxLen 1 no ellipsis", "hello", 1, "h"},
		{"empty string", "", 10, ""},
		{"maxLen 0", "hello", 0, ""},
		{"unicode string truncation", "日本語テスト", 4, "日本語…"},
		{"mixed unicode", "hello世界", 6, "hello…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

// containsStr reports whether substr occurs in s.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
