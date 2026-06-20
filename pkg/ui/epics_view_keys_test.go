package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// epicsTestModel builds a fully-initialized ViewEpics model (keys, filter,
// data all wired via NewModel) from the given issues, then builds the tree.
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

// headerLine returns the first rendered line (the EPICS header).
func headerLine(viewText string) string {
	return strings.SplitN(viewText, "\n", 2)[0]
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

func TestHandleEpicsTree_Navigation(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	// Default flatten: 2 lane headers + 2 collapsed epics = 4 rows; the cursor
	// traverses every row (headers included).
	rows := m.epicsTree.rows()
	if len(rows) != 4 {
		t.Fatalf("expected 4 tree rows (2 headers + 2 epics), got %d", len(rows))
	}
	if m.epicsTree.cursor != 0 {
		t.Fatalf("cursor should start at 0, got %d", m.epicsTree.cursor)
	}

	for want := 1; want <= 3; want++ {
		m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
		if m.epicsTree.cursor != want {
			t.Errorf("after j: cursor=%d, want %d", m.epicsTree.cursor, want)
		}
	}
	// Clamp at the last row.
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.epicsTree.cursor != 3 {
		t.Errorf("after extra j: cursor=%d, want 3 (clamped)", m.epicsTree.cursor)
	}
	// Back to the top.
	for i := 0; i < 5; i++ {
		m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	}
	if m.epicsTree.cursor != 0 {
		t.Errorf("after k*5: cursor=%d, want 0 (clamped)", m.epicsTree.cursor)
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

func TestHandleEpicsTree_ExpandRevealsChildrenAndFocusesSubtree(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	// Move from the lane header onto the ep1 epic row.
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if r, _ := m.epicsTree.cursorRow(); r.kind != rowEpic {
		t.Fatalf("expected cursor on an epic row, got kind %d", r.kind)
	}
	// Expand (l) reveals children and focuses the subtree (cursor onto a child).
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if i, _ := rowByIssue(m.epicsTree.rows(), "ep1.a"); i == -1 {
		t.Errorf("expand should reveal ep1.a\n%s", dumpRows(m.epicsTree.rows()))
	}
	if r, _ := m.epicsTree.cursorRow(); r.kind != rowChild {
		t.Errorf("expand should focus the subtree (cursor on a child), got kind %d", r.kind)
	}
}

func TestHandleEpicsTree_CollapseHidesChildren(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"}) // onto ep1
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'l', Text: "l"}) // expand -> cursor on child
	// h on a child jumps to the parent epic; h again collapses it.
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if r, _ := m.epicsTree.cursorRow(); r.kind != rowEpic || r.issue == nil || r.issue.ID != "ep1" {
		t.Fatalf("h on a child should jump to parent ep1, got %+v", r)
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if i, _ := rowByIssue(m.epicsTree.rows(), "ep1.a"); i != -1 {
		t.Errorf("collapse should hide ep1.a (found at %d)", i)
	}
}

func TestHandleEpicsTree_EnterDrillsChildToDetail(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"}) // onto ep1
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'l', Text: "l"}) // expand -> cursor on ep1.a
	r, _ := m.epicsTree.cursorRow()
	if r.kind != rowChild {
		t.Fatalf("setup: expected cursor on a child, got kind %d", r.kind)
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != ViewList {
		t.Errorf("drilling a child should switch to ViewList, got mode %v", m.mode)
	}
	if m.focused != focusDetail {
		t.Errorf("drilling a child should focus the detail pane, got %v", m.focused)
	}
}

func TestHandleEpicsTree_EnterExpandsEpic(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"}) // onto ep1 epic
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: tea.KeyEnter})   // enter on epic -> expand
	if i, _ := rowByIssue(m.epicsTree.rows(), "ep1.a"); i == -1 {
		t.Errorf("enter on an epic should expand it (reveal ep1.a)")
	}
	if m.mode != ViewEpics {
		t.Errorf("enter on an epic should stay in ViewEpics, got %v", m.mode)
	}
}

func TestHandleEpicsTree_CardZoom(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"}) // onto ep1 epic
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'v', Text: "v"})
	if m.activeModal != ModalEpicCard {
		t.Errorf("v should open the epic focus card, activeModal=%v", m.activeModal)
	}
	if m.epicCardID != "ep1" {
		t.Errorf("card should target the cursor epic ep1, got %q", m.epicCardID)
	}
}

func TestHandleEpicsTree_CardZoomFromChildUsesParentEpic(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"}) // onto ep1
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'l', Text: "l"}) // expand -> cursor on child
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'v', Text: "v"})
	if m.activeModal != ModalEpicCard || m.epicCardID != "ep1" {
		t.Errorf("v on a child should zoom its parent epic ep1, got modal=%v id=%q", m.activeModal, m.epicCardID)
	}
}

func TestHandleEpicsTree_CollapseAllReturnsToLaneOverview(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'j', Text: "j"}) // onto ep1
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'l', Text: "l"}) // expand
	if len(m.epicsTree.rows()) <= 4 {
		t.Fatalf("expand should add child rows, got %d", len(m.epicsTree.rows()))
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if got := len(m.epicsTree.rows()); got != 4 {
		t.Errorf("collapseAll should return to the 4-row lane overview, got %d", got)
	}
}

func TestEpicsTree_RenderBasic(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	result := m.epicsViewText

	if !containsStr(result, "EPICS") {
		t.Error("should contain the EPICS header")
	}
	if !containsStr(headerLine(result), "active") {
		t.Error("header should show the 'active' mode")
	}
	if !containsStr(result, "ep1") {
		t.Error("should list epic ep1")
	}
	// ep1 has 1 closed of 2 children -> progress 1/2.
	if !containsStr(result, "1/2") {
		t.Error("should show ep1 progress 1/2")
	}
}

func TestEpicsTree_RenderEmpty(t *testing.T) {
	m := epicsTestModel([]model.Issue{
		{ID: "task-1", Title: "Not an epic", Status: model.StatusOpen, IssueType: model.TypeTask},
	})
	if m.epicsTree.epicCount() != 0 {
		t.Fatalf("expected 0 epics, got %d", m.epicsTree.epicCount())
	}
	if !containsStr(m.epicsViewText, "No epics in scope") {
		t.Error("empty overview should show 'No epics in scope'")
	}
}

func TestEpicsTree_RenderAtRisk(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	// ep1.b is in_progress and 5 days stale -> ep1 row carries an at-risk marker.
	if !containsStr(m.epicsViewText, "⚠") {
		t.Error("overview should show an at-risk marker for the stale child")
	}
}

func TestEpicsTree_NarrowWidth(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	m.width = 30 // very narrow
	m.refreshEpicsForCurrentFilter()
	if m.epicsViewText == "" {
		t.Error("should produce output even with narrow width")
	}
}

func TestEpicsTree_DefaultSortProgressAscending(t *testing.T) {
	pc := func(child, parent string) []*model.Dependency {
		return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
	}
	// ep-high is listed first but is 80% done; ep-low is 20%. Both share the "ep"
	// prefix -> one lane; within the lane the least-complete epic sorts first.
	issues := []model.Issue{
		{ID: "ep-high", Title: "Almost done", IssueType: model.TypeEpic, Status: model.StatusOpen},
		{ID: "ep-high.1", Status: model.StatusClosed, Dependencies: pc("ep-high.1", "ep-high")},
		{ID: "ep-high.2", Status: model.StatusClosed, Dependencies: pc("ep-high.2", "ep-high")},
		{ID: "ep-high.3", Status: model.StatusClosed, Dependencies: pc("ep-high.3", "ep-high")},
		{ID: "ep-high.4", Status: model.StatusClosed, Dependencies: pc("ep-high.4", "ep-high")},
		{ID: "ep-high.5", Status: model.StatusOpen, Dependencies: pc("ep-high.5", "ep-high")},
		{ID: "ep-low", Title: "Just started", IssueType: model.TypeEpic, Status: model.StatusOpen},
		{ID: "ep-low.1", Status: model.StatusClosed, Dependencies: pc("ep-low.1", "ep-low")},
		{ID: "ep-low.2", Status: model.StatusOpen, Dependencies: pc("ep-low.2", "ep-low")},
		{ID: "ep-low.3", Status: model.StatusOpen, Dependencies: pc("ep-low.3", "ep-low")},
		{ID: "ep-low.4", Status: model.StatusOpen, Dependencies: pc("ep-low.4", "ep-low")},
		{ID: "ep-low.5", Status: model.StatusOpen, Dependencies: pc("ep-low.5", "ep-low")},
	}
	m := epicsTestModel(issues)
	rows := m.epicsTree.rows()
	// Find the two epic rows in flatten order.
	var epicIDs []string
	for _, r := range rows {
		if r.kind == rowEpic && r.issue != nil {
			epicIDs = append(epicIDs, r.issue.ID)
		}
	}
	if len(epicIDs) != 2 {
		t.Fatalf("want 2 epic rows, got %d (%v)", len(epicIDs), epicIDs)
	}
	if epicIDs[0] != "ep-low" || epicIDs[1] != "ep-high" {
		t.Errorf("progress-asc order = %v, want [ep-low ep-high]", epicIDs)
	}
}

func TestEpicsTree_StatusCycleHeader(t *testing.T) {
	m := epicsTestModel(epicsFixture())
	if !containsStr(headerLine(m.epicsViewText), "active") {
		t.Error("default header should show 'active' mode")
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	if !containsStr(headerLine(m.epicsViewText), "all") {
		t.Error("after s, header should show 'all' mode")
	}
	m = m.handleEpicsKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	if !containsStr(headerLine(m.epicsViewText), "completed") {
		t.Error("after 2nd s, header should show 'completed' mode")
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
	return strings.Contains(s, substr)
}
