package ui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
// in split view (bt-58yw regression fix; bt-fxbl chrome unification).
//
// Post bt-fxbl chrome rows are:
//   1. RenderTitledPanel top border
//   2. renderSearchRow (always 1 row, bridges all FilterStates)
//   3. renderSplitView column header row
//
// The Bubbles phantom title row is gone — l.SetShowFilter(false) +
// l.SetShowTitle(false) skips the titleView branch entirely in list.View().
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

// TestHandleMouseClick_DetailFocusCommitsFilter verifies clicking the detail
// pane while the search input is in Filtering state commits the filter to
// FilterApplied (bt-ocmw). Without this, all global hotkeys gated on
// FilterState != Filtering stay blocked even though no one is typing in the
// search input - the user is locked into mouse-only navigation.
func TestHandleMouseClick_DetailFocusCommitsFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(60, 30)
	m.focused = focusList

	// Simulate the user opening search and typing.
	m.list.SetFilterText("first")
	m.list.SetFilterState(list.Filtering)
	if got := m.list.FilterState(); got != list.Filtering {
		t.Fatalf("precondition: filter state not Filtering, got %v", got)
	}

	// Click into the detail pane (right side, past list boundary).
	listBoundary := m.list.Width() + 4
	msg := tea.MouseClickMsg{X: listBoundary + 10, Y: 5, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)

	if got.focused != focusDetail {
		t.Fatalf("click on detail pane should focus detail, got %v", got.focused)
	}
	if state := got.list.FilterState(); state != list.FilterApplied {
		t.Fatalf("filter should commit to FilterApplied on detail focus, got %v", state)
	}
	if val := got.list.FilterValue(); val != "first" {
		t.Fatalf("filter value should be preserved on commit, got %q want %q", val, "first")
	}
}

// TestSplitViewChromeHeight_StableAcrossFilterStates is the core bt-fxbl
// regression guard: the chrome height (= Y of first list item) MUST be
// identical across Unfiltered, Filtering, and FilterApplied so the column
// header doesn't visibly shift as the user types/commits/clears the filter.
//
// Before bt-fxbl: chrome differed by 1 row between Filtering (Bubbles
// rendered FilterInput in titleView, below our column header) and
// FilterApplied (our renderSearchPill rendered above the column header),
// jarring the user's eye every time the state transitioned.
func TestSplitViewChromeHeight_StableAcrossFilterStates(t *testing.T) {
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

	// Unfiltered baseline.
	hUnfiltered := m.splitViewListChromeHeight()

	// Filtering: simulate the user opening / and typing.
	m.list.SetFilterText("first")
	m.list.SetFilterState(list.Filtering)
	hFiltering := m.splitViewListChromeHeight()

	// FilterApplied: committed.
	m.list.SetFilterState(list.FilterApplied)
	hApplied := m.splitViewListChromeHeight()

	if hUnfiltered != hFiltering {
		t.Errorf("chrome height changed between Unfiltered (%d) and Filtering (%d) — column header would shift",
			hUnfiltered, hFiltering)
	}
	if hUnfiltered != hApplied {
		t.Errorf("chrome height changed between Unfiltered (%d) and FilterApplied (%d) — column header would shift",
			hUnfiltered, hApplied)
	}
	if hFiltering != hApplied {
		t.Errorf("chrome height changed between Filtering (%d) and FilterApplied (%d) — column header would shift",
			hFiltering, hApplied)
	}

	// Sanity: chrome height is panel border (1) + search row (1) + column header (1) = 3.
	const expectedChrome = 3
	if hUnfiltered != expectedChrome {
		t.Errorf("expected chrome height %d (panel border + search row + column header), got %d",
			expectedChrome, hUnfiltered)
	}
}

// TestRenderSearchRow_AlwaysOneRow verifies the search row is fixed-height
// (1 terminal row) across all FilterStates. This is the precondition for
// the chrome stability above (bt-fxbl).
func TestRenderSearchRow_AlwaysOneRow(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(80, 30)

	cases := []struct {
		name  string
		setup func()
	}{
		{
			name:  "Unfiltered",
			setup: func() { /* default */ },
		},
		{
			name: "Filtering with query",
			setup: func() {
				m.list.SetFilterText("first")
				m.list.SetFilterState(list.Filtering)
			},
		},
		{
			name: "FilterApplied with query",
			setup: func() {
				m.list.SetFilterText("first")
				m.list.SetFilterState(list.FilterApplied)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			row := m.renderSearchRow(m.list.Width())
			if row == "" {
				t.Fatalf("renderSearchRow returned empty string in state %s — chrome height would shift", tc.name)
			}
			// Use lipgloss.Height to count rows (defense in depth: a styled
			// row that wrapped would count as >1).
			if h := lipgloss.Height(row); h != 1 {
				t.Errorf("renderSearchRow returned %d-row output in state %s; want exactly 1 row", h, tc.name)
			}
		})
	}
}

// TestRenderSearchRow_ClipsToWidth verifies the search row never overflows the
// requested width even with long typed queries (bt-m6cd). Without clipping, a
// query like `"tftj","fxbl","l5xu","l5zk","0mxw"` plus the match-count exceeds
// narrow pane widths, causing lipgloss to wrap to a second line and breaking
// the 1-row chrome invariant in splitViewListChromeHeight - which then causes
// list rows to render with truncated content.
func TestRenderSearchRow_ClipsToWidth(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(80, 30)

	longQuery := `"tftj","fxbl","l5xu","l5zk","0mxw"`
	m.list.SetFilterText(longQuery)

	for _, state := range []list.FilterState{list.Filtering, list.FilterApplied} {
		t.Run(state.String(), func(t *testing.T) {
			m.list.SetFilterState(state)
			for _, width := range []int{30, 40, 50, 60, 80} {
				row := m.renderSearchRow(width)
				if got := lipgloss.Width(row); got > width {
					t.Errorf("state=%v width=%d: row width %d exceeds limit", state, width, got)
				}
				if h := lipgloss.Height(row); h != 1 {
					t.Errorf("state=%v width=%d: row wrapped to %d lines, want 1", state, width, h)
				}
			}
		})
	}
}
