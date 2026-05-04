package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestHandleMouseClick_NoModalRequired(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.activeModal = ModalHelp
	msg := tea.MouseClickMsg{X: 50, Y: 10, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.focused != m.focused {
		t.Fatalf("focus should not change when modal is open")
	}
}

func TestHandleMouseClick_SplitViewSwitchesFocus(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	// Force a known listInnerWidth so we can reason about boundary.
	m.list.SetSize(60, 30)
	m.focused = focusList

	// Click to the right of the list boundary → should focus detail.
	rightClick := tea.MouseClickMsg{X: 150, Y: 5, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(rightClick)
	if got.focused != focusDetail {
		t.Fatalf("expected focusDetail after click on detail pane, got %v", got.focused)
	}

	// Click on the left side → should focus list.
	leftClick := tea.MouseClickMsg{X: 2, Y: 5, Button: tea.MouseLeft}
	got.focused = focusDetail
	got, _ = got.handleMouseClick(leftClick)
	if got.focused != focusList {
		t.Fatalf("expected focusList after click on list pane, got %v", got.focused)
	}
}

func TestHandleMouseClick_RightButtonIgnored(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.focused = focusList

	msg := tea.MouseClickMsg{X: 150, Y: 5, Button: tea.MouseRight}
	got, _ := m.handleMouseClick(msg)
	if got.focused != focusList {
		t.Fatalf("right-click should not change focus, got %v", got.focused)
	}
}

func TestHandleMouseClick_FooterIgnored(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.focused = focusList

	// Click on the footer row (last line) → should be a no-op.
	msg := tea.MouseClickMsg{X: 150, Y: 39, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.focused != focusList {
		t.Fatalf("footer click should not change focus, got %v", got.focused)
	}
}

func TestHandleMouseClick_NonListModeIgnored(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewBoard // not ViewList
	m.isSplitView = true
	m.focused = focusBoard

	msg := tea.MouseClickMsg{X: 150, Y: 5, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.focused != focusBoard {
		t.Fatalf("non-list mode should not change focus, got %v", got.focused)
	}
}

// TestHandleMouseClick_RowMathMatchesChrome verifies the Y-to-row offset
// computation accounts for all vertical chrome above the first list item
// in split view (bt-58yw regression fix). The chrome rows are:
//   1. RenderTitledPanel top border
//   2. Optional search pill (not present in this test; FilterState = Unfiltered)
//   3. renderSplitView column header row
//   4. Bubbles list phantom title row emitted because SetShowTitle(false)
//      is not enough to suppress the titleView branch in bubbles/v2/list/list.go
//      when SetFilteringEnabled(true) is also in effect.
func TestHandleMouseClick_RowMathMatchesChrome(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "docs: first PR", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "epic: docs", Status: model.StatusOpen},
		{ID: "cass-z95i", Title: "[epic] Build order", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(60, 30)
	m.focused = focusDetail

	// Ask the implementation where chrome ends — this is the row of the
	// first list item.
	firstItemY := m.splitViewListChromeHeight()

	// Click on the first visible row — should select index 0 (bd-cc0).
	msg := tea.MouseClickMsg{X: 10, Y: firstItemY, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.focused != focusList {
		t.Fatalf("click on list pane should focus list, got %v", got.focused)
	}
	if got.list.Index() != 0 {
		t.Fatalf("click on first row Y=%d should select index 0 (bd-cc0), got index %d",
			firstItemY, got.list.Index())
	}

	// Click on the third visible row — should select index 2 (cass-z95i).
	// This is the exact bug from the dogfood screenshot: clicking bd-cc0
	// was selecting cass-z95i. We now require that clicking the third row
	// actually selects the third row.
	msg2 := tea.MouseClickMsg{X: 10, Y: firstItemY + 2, Button: tea.MouseLeft}
	got2, _ := got.handleMouseClick(msg2)
	if got2.list.Index() != 2 {
		t.Fatalf("click on third row Y=%d should select index 2 (cass-z95i), got index %d",
			firstItemY+2, got2.list.Index())
	}
}

// TestHandleMouseClick_BelowLastVisibleRow_NoSelectionChange verifies clicks
// below the last rendered row do not change list selection (bt-0lsm). Regression
// guard for the unfiltered-Items() vs VisibleItems() bounds-check bug surfaced
// via dogfood with a 3/455-match search filter.
func TestHandleMouseClick_BelowLastVisibleRow_NoSelectionChange(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
		{ID: "cass-z95i", Title: "third", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(60, 30)
	m.focused = focusList

	firstItemY := m.splitViewListChromeHeight()

	// Pre-condition: select index 0 explicitly so we can detect spurious changes.
	m.list.Select(0)
	if m.list.Index() != 0 {
		t.Fatalf("precondition: list.Select(0) failed, got index %d", m.list.Index())
	}

	// Click well below the last rendered row (3 items rendered → rows 0,1,2).
	// Y=firstItemY+7 lands in empty viewport space below the items.
	msg := tea.MouseClickMsg{X: 10, Y: firstItemY + 7, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)

	if got.list.Index() != 0 {
		t.Fatalf("click below last visible row should not change selection, expected index 0, got %d",
			got.list.Index())
	}
}
