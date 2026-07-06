package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestHandleMouseClick_NoModalRequired(t *testing.T) {
	m := NewModel(nil, nil, "", nil, nil)
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
	m := NewModel(nil, nil, "", nil, nil)
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
	m := NewModel(nil, nil, "", nil, nil)
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
	m := NewModel(nil, nil, "", nil, nil)
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
	m := NewModel(nil, nil, "", nil, nil)
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
	m := NewModel(issues, nil, "", nil, nil)
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
	m := NewModel(issues, nil, "", nil, nil)
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

// TestHandleMouseClick_BelowLastRenderedRow_Unfiltered_NoPageJump verifies that
// at large unfiltered lists, clicking in the empty viewport region between the
// last rendered row and the footer does NOT trigger a page advance (bt-9kj7,
// sister of bt-0lsm). With 1000+ items the bt-0lsm bound (`row < len(visible)`)
// passes for any plausible Y; the fix bounds against rows actually rendered on
// the current page.
func TestHandleMouseClick_BelowLastRenderedRow_Unfiltered_NoPageJump(t *testing.T) {
	issues := make([]model.Issue, 0, 1000)
	for i := 0; i < 1000; i++ {
		issues = append(issues, model.Issue{
			ID:     "bd-aa" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)),
			Title:  "row",
			Status: model.StatusOpen,
		})
	}
	m := NewModel(issues, nil, "", nil, nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(60, 30)
	m.focused = focusList

	// Pre-condition: index 0 selected, page 0.
	m.list.Select(0)
	if m.list.Index() != 0 {
		t.Fatalf("precondition: list.Select(0) failed, got %d", m.list.Index())
	}
	startPage := m.list.Paginator.Page
	if startPage != 0 {
		t.Fatalf("precondition: expected page 0, got %d", startPage)
	}

	// Click in the empty viewport region between the last rendered row and
	// the footer. List height=30, chrome=3, PerPage=30 -> page 0 rendered
	// rows occupy Y=3..32. Footer is at Y=39 (m.height-1). Y=35 lands in
	// the empty gap that the bt-0lsm bound failed to protect.
	msg := tea.MouseClickMsg{X: 10, Y: 35, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)

	if got.list.Index() != 0 {
		t.Fatalf("click below last rendered row should not change selection, expected index 0, got %d",
			got.list.Index())
	}
	if got.list.Paginator.Page != startPage {
		t.Fatalf("click below last rendered row should not advance page, expected page %d, got %d",
			startPage, got.list.Paginator.Page)
	}
}

// TestHandleMouseClick_SearchRowReopensFilter verifies clicking on the
// always-present search row (chrome row Y=1, post bt-fxbl) transitions the
// list to Filtering state regardless of starting state (bt-49nn). Without
// this, mouse-driven users have no way to re-edit a committed query short
// of pressing `/` — the search bar is visible but inert to clicks.
func TestHandleMouseClick_SearchRowReopensFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
	}

	cases := []struct {
		name      string
		setup     func(m *Model)
		wantValue string
	}{
		{
			name:      "FilterApplied -> Filtering",
			setup:     func(m *Model) { m.list.SetFilterText("first"); m.list.SetFilterState(list.FilterApplied) },
			wantValue: "first",
		},
		{
			name:      "Unfiltered -> Filtering",
			setup:     func(m *Model) { /* default */ },
			wantValue: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(issues, nil, "", nil, nil)
			m.width = 200
			m.height = 40
			m.mode = ViewList
			m.isSplitView = true
			m.list.SetSize(60, 30)
			m.focused = focusList
			tc.setup(&m)

			// Search row is at Y=1 (chrome layer 2 of 3, post bt-fxbl).
			msg := tea.MouseClickMsg{X: 10, Y: 1, Button: tea.MouseLeft}
			got, _ := m.handleMouseClick(msg)

			if state := got.list.FilterState(); state != list.Filtering {
				t.Fatalf("expected FilterState=Filtering after search-row click, got %v", state)
			}
			if got.focused != focusList {
				t.Fatalf("expected focusList after search-row click, got %v", got.focused)
			}
			if val := got.list.FilterValue(); val != tc.wantValue {
				t.Fatalf("expected filter value preserved as %q, got %q", tc.wantValue, val)
			}
		})
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
	m := NewModel(issues, nil, "", nil, nil)
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

// TestCommitFilterIfTyping_EmptyResetsFilter verifies that committing an empty
// filter buffer resets the filter (returns to Unfiltered) instead of applying
// an empty FilterApplied state — the latter renders as "No items" in Bubbles
// even when the underlying list is populated (bt-5q51). Sister test to
// TestHandleMouseClick_DetailFocusCommitsFilter (which covers the non-empty
// case).
func TestCommitFilterIfTyping_EmptyResetsFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(60, 30)
	m.focused = focusList

	// Simulate the user clicking the search row (bt-49nn) and not typing.
	m.list.SetFilterState(list.Filtering)
	if got := m.list.FilterState(); got != list.Filtering {
		t.Fatalf("precondition: filter state not Filtering, got %v", got)
	}

	// Click into the detail pane without typing.
	listBoundary := m.list.Width() + 4
	msg := tea.MouseClickMsg{X: listBoundary + 10, Y: 5, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)

	if state := got.list.FilterState(); state != list.Unfiltered {
		t.Fatalf("empty-buffer commit should reset to Unfiltered, got %v", state)
	}
	if visible := len(got.list.VisibleItems()); visible != len(issues) {
		t.Fatalf("after empty-buffer commit all items should be visible, got %d want %d",
			visible, len(issues))
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
	m := NewModel(issues, nil, "", nil, nil)
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
	m := NewModel(issues, nil, "", nil, nil)
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
	m := NewModel(issues, nil, "", nil, nil)
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

// TestHandleMouseClick_SearchRowEntry_PreservesVisibleItems is the bt-r2ev
// Bug A regression guard: clicking the search row to enter Filtering must
// keep all underlying items visible (matching the keyboard `/` path).
//
// Pre-fix: the click path called m.list.SetFilterState(list.Filtering)
// directly, bypassing Bubbles' filter-begin setup that populates
// filteredItems = itemsAsFilterItems() when the filter buffer is empty.
// The list then rendered as "no matches" because filteredItems was empty
// while filterState was Filtering. The fix forwards a synthetic "/"
// KeyPressMsg through m.list.Update so Bubbles runs the same setup it
// runs for the keyboard path.
func TestHandleMouseClick_SearchRowEntry_PreservesVisibleItems(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
		{ID: "cass-z95i", Title: "third", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(60, 30)
	m.focused = focusList

	// Sanity: all items visible before any click.
	if got := len(m.list.VisibleItems()); got != len(issues) {
		t.Fatalf("precondition: expected %d visible items before click, got %d", len(issues), got)
	}

	// Click the search row at Y=1 with empty filter buffer.
	msg := tea.MouseClickMsg{X: 10, Y: 1, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)

	// State transitioned to Filtering (covered by bt-49nn test, but assert here too).
	if state := got.list.FilterState(); state != list.Filtering {
		t.Fatalf("expected FilterState=Filtering after search-row click, got %v", state)
	}
	// Critical Bug A invariant: items must still be visible during empty Filtering.
	if visible := len(got.list.VisibleItems()); visible != len(issues) {
		t.Fatalf("Bug A regression: expected %d visible items during empty Filtering, got %d "+
			"(SetFilterState alone bypasses Bubbles' filter-begin setup)",
			len(issues), visible)
	}
}

// TestHandleMouseClick_ListPaneClick_CommitsActiveFilter is the bt-r2ev
// Bug B regression guard: clicking inside the issues pane while the filter
// is in Filtering state must commit the filter.
//
// Pre-fix: only clicking into the detail pane fired commitFilterIfTyping
// (bt-ocmw). A click on a row OR in the gap below the last row kept focus
// on focusList and left the filter stuck in Filtering — the user had to
// click to detail, press `/`, or hit Esc to escape.
//
// Covers both branches:
//   - empty buffer → ResetFilter (Unfiltered, bt-5q51 path)
//   - non-empty buffer → FilterApplied (bt-ocmw path)
//
// And both click locations:
//   - on a rendered row → commit then Select(row)
//   - in the gap below last row → commit only (no selection change)
func TestHandleMouseClick_ListPaneClick_CommitsActiveFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
		{ID: "cass-z95i", Title: "third", Status: model.StatusOpen},
	}

	// Chrome is 3 rows (panel border + search row + column header), so
	// row 0 renders at Y=3, row 1 at Y=4, row 2 at Y=5. A click far
	// below (e.g. Y=20) lands in the gap between last row and footer.
	const firstRowY = 3
	const gapY = 20

	cases := []struct {
		name      string
		filterBuf string
		clickY    int
		wantState list.FilterState
		wantValue string
	}{
		{
			name:      "row click with non-empty buffer commits to FilterApplied",
			filterBuf: "first",
			clickY:    firstRowY,
			wantState: list.FilterApplied,
			wantValue: "first",
		},
		{
			name:      "row click with empty buffer resets to Unfiltered",
			filterBuf: "",
			clickY:    firstRowY,
			wantState: list.Unfiltered,
			wantValue: "",
		},
		{
			name:      "gap click with non-empty buffer commits to FilterApplied",
			filterBuf: "first",
			clickY:    gapY,
			wantState: list.FilterApplied,
			wantValue: "first",
		},
		{
			name:      "gap click with empty buffer resets to Unfiltered",
			filterBuf: "",
			clickY:    gapY,
			wantState: list.Unfiltered,
			wantValue: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(issues, nil, "", nil, nil)
			m.width = 200
			m.height = 40
			m.mode = ViewList
			m.isSplitView = true
			m.list.SetSize(60, 30)
			m.focused = focusList

			// Simulate active Filtering state with the test's buffer.
			if tc.filterBuf != "" {
				m.list.SetFilterText(tc.filterBuf)
			}
			m.list.SetFilterState(list.Filtering)
			if state := m.list.FilterState(); state != list.Filtering {
				t.Fatalf("precondition: filter not Filtering, got %v", state)
			}

			msg := tea.MouseClickMsg{X: 10, Y: tc.clickY, Button: tea.MouseLeft}
			got, _ := m.handleMouseClick(msg)

			if state := got.list.FilterState(); state != tc.wantState {
				t.Fatalf("Bug B regression: filter state after click = %v, want %v",
					state, tc.wantState)
			}
			if val := got.list.FilterValue(); val != tc.wantValue {
				t.Fatalf("filter value after click = %q, want %q", val, tc.wantValue)
			}
		})
	}
}

// singlePaneTestIssues is a tiny fixture shared by the single-pane click tests.
func singlePaneTestIssues() []model.Issue {
	return []model.Issue{
		{ID: "bd-cc0", Title: "first", Status: model.StatusOpen},
		{ID: "bd-cgh", Title: "second", Status: model.StatusOpen},
		{ID: "cass-z95i", Title: "third", Status: model.StatusOpen},
	}
}

// TestHandleMouseClick_SinglePaneChromeHeight pins the single-pane list chrome
// to 2 rows (search row at Y=0 + column header at Y=1, no panel top border),
// so the first list item is at Y=2 (bt-bxu6u). This is the single-pane analogue
// of splitViewListChromeHeight's 3 rows.
func TestHandleMouseClick_SinglePaneChromeHeight(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.list.SetSize(m.bodyWidth(), 24)
	if got := m.singlePaneListChromeHeight(); got != 2 {
		t.Fatalf("singlePaneListChromeHeight = %d, want 2 (search row + column header)", got)
	}
}

// TestSinglePaneListRenderGeometry pins singlePaneListChromeHeight to the
// actual renderListWithHeader output: search row on line 0, column header on
// line 1, first list item on line 2. If renderListWithHeader's chrome ever
// shifts, this fails alongside the click math that depends on it (bt-bxu6u).
func TestSinglePaneListRenderGeometry(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 90 // single-pane (< SplitViewThreshold)
	m.height = 24
	m.mode = ViewList
	m.isSplitView = false
	m.list.SetSize(m.bodyWidth(), m.height-3)
	m.updateListDelegate()

	lines := strings.Split(m.renderListWithHeader(), "\n")
	chrome := m.singlePaneListChromeHeight()
	if len(lines) <= chrome {
		t.Fatalf("rendered %d lines, need at least %d", len(lines), chrome+1)
	}
	if !strings.Contains(lines[0], "Search:") {
		t.Fatalf("line 0 should be the search row, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "TYPE") {
		t.Fatalf("line 1 should be the column header, got %q", lines[1])
	}
	if !strings.Contains(lines[chrome], "bd-cc0") {
		t.Fatalf("first list item (bd-cc0) should render on line %d (singlePaneListChromeHeight), got %q",
			chrome, lines[chrome])
	}
}

// TestHandleMouseClick_SinglePaneNarrowSelectsRow is the core bt-bxu6u
// regression: in the narrow single-pane layout (width < SplitViewThreshold, no
// detail pane), clicking a list row used to be a dead no-op. It must now select
// the clicked row, matching split-view behavior.
func TestHandleMouseClick_SinglePaneNarrowSelectsRow(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 80 // below SplitViewThreshold (100): single-pane
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.showDetails = false
	m.list.SetSize(m.bodyWidth(), 24)
	m.focused = focusList

	firstItemY := m.singlePaneListChromeHeight()

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: firstItemY, Button: tea.MouseLeft})
	if got.list.Index() != 0 {
		t.Fatalf("click on first row Y=%d should select index 0, got %d", firstItemY, got.list.Index())
	}

	got2, _ := got.handleMouseClick(tea.MouseClickMsg{X: 5, Y: firstItemY + 2, Button: tea.MouseLeft})
	if got2.list.Index() != 2 {
		t.Fatalf("click on third row Y=%d should select index 2, got %d", firstItemY+2, got2.list.Index())
	}
}

// TestHandleMouseClick_SinglePaneWideMaximizedSelectsRow covers bt-bxu6u's
// scope-widening note: the dead gate also fired for the WIDE single-pane list
// (full terminal width, details pane intentionally hidden, so isSplitView is
// false even with room to spare). Clicks must select there too.
func TestHandleMouseClick_SinglePaneWideMaximizedSelectsRow(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 200 // well above SplitViewThreshold, but details hidden
	m.height = 50
	m.mode = ViewList
	m.isSplitView = false
	m.showDetails = false
	m.list.SetSize(m.bodyWidth(), 44)
	m.focused = focusList

	firstItemY := m.singlePaneListChromeHeight()

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: firstItemY + 1, Button: tea.MouseLeft})
	if got.list.Index() != 1 {
		t.Fatalf("wide single-pane click on second row Y=%d should select index 1, got %d",
			firstItemY+1, got.list.Index())
	}
}

// TestHandleMouseClick_SinglePaneSearchRowReopensFilter verifies a click on the
// single-pane search row (Y=0, no panel top border) reopens the filter input,
// mirroring the split-view search-row click (which lives at Y=1) (bt-bxu6u).
func TestHandleMouseClick_SinglePaneSearchRowReopensFilter(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.showDetails = false
	m.list.SetSize(m.bodyWidth(), 24)
	m.focused = focusList

	if m.list.FilterState() == list.Filtering {
		t.Fatalf("precondition: list should not start in Filtering state")
	}

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: 0, Button: tea.MouseLeft})
	if got.list.FilterState() != list.Filtering {
		t.Fatalf("click on single-pane search row (Y=0) should reopen filter, got state %v",
			got.list.FilterState())
	}
}

// TestHandleMouseClick_SinglePaneShowDetailsNoOp verifies that when the
// single-pane layout is showing the full-screen detail viewport (showDetails,
// no list rendered), a click does not select a list row (bt-bxu6u).
func TestHandleMouseClick_SinglePaneShowDetailsNoOp(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.showDetails = true // full-screen detail viewport, list not visible
	m.list.SetSize(m.bodyWidth(), 24)
	m.list.Select(0)

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: 4, Button: tea.MouseLeft})
	if got.list.Index() != 0 {
		t.Fatalf("showDetails viewport has no list rows; click should not change selection, got %d",
			got.list.Index())
	}
}

// TestHandleMouseClick_SinglePaneSidebarClickIgnored verifies clicks landing in
// the shortcuts-sidebar columns (X past bodyWidth) do not select a list row in
// single-pane layout (bt-bxu6u).
func TestHandleMouseClick_SinglePaneSidebarClickIgnored(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 120
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.showDetails = false
	m.showShortcutsSidebar = true
	m.shortcutsSidebar.SetSize(24, 28) // bodyWidth shrinks to 120-24 = 96
	m.list.SetSize(m.bodyWidth(), 24)
	m.list.Select(0)

	firstItemY := m.singlePaneListChromeHeight()
	// X at the first sidebar column (== bodyWidth) is past the list body.
	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: m.bodyWidth(), Y: firstItemY + 1, Button: tea.MouseLeft})
	if got.list.Index() != 0 {
		t.Fatalf("click in sidebar columns should not select a list row, got index %d", got.list.Index())
	}
}

// --- Double-click to open (bt-f3zbz) ---

// TestHandleMouseClick_SinglePaneDoubleClickOpensDetail verifies a second click
// on the same row within listDoubleClickWindow opens the full-screen detail in
// single-pane (Enter equivalent); the first click only selects (bt-f3zbz).
func TestHandleMouseClick_SinglePaneDoubleClickOpensDetail(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.showDetails = false
	m.list.SetSize(m.bodyWidth(), 24)
	m.focused = focusList

	firstItemY := m.singlePaneListChromeHeight()
	click := tea.MouseClickMsg{X: 5, Y: firstItemY, Button: tea.MouseLeft}

	m, _ = m.handleMouseClick(click)
	if m.showDetails {
		t.Fatalf("first click should select only, not open detail")
	}
	if m.list.Index() != 0 {
		t.Fatalf("first click should select index 0, got %d", m.list.Index())
	}

	m, _ = m.handleMouseClick(click)
	if !m.showDetails {
		t.Fatalf("double-click on same row should open detail (showDetails=true)")
	}
	if m.focused != focusDetail {
		t.Fatalf("double-click should focus detail, got %v", m.focused)
	}
}

// TestHandleMouseClick_DoubleClickDifferentRowDoesNotOpen: two clicks on
// different rows within the window select but do not open (bt-f3zbz).
func TestHandleMouseClick_DoubleClickDifferentRowDoesNotOpen(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.list.SetSize(m.bodyWidth(), 24)
	m.focused = focusList

	firstItemY := m.singlePaneListChromeHeight()
	m, _ = m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: firstItemY, Button: tea.MouseLeft})
	m, _ = m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: firstItemY + 1, Button: tea.MouseLeft})
	if m.showDetails {
		t.Fatalf("clicks on different rows should not open detail")
	}
	if m.list.Index() != 1 {
		t.Fatalf("second click should select index 1, got %d", m.list.Index())
	}
}

// TestHandleMouseClick_DoubleClickExpiredWindowDoesNotOpen: a second click on
// the same row after the window has elapsed re-selects but does not open
// (bt-f3zbz).
func TestHandleMouseClick_DoubleClickExpiredWindowDoesNotOpen(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.list.SetSize(m.bodyWidth(), 24)
	m.focused = focusList

	firstItemY := m.singlePaneListChromeHeight()
	click := tea.MouseClickMsg{X: 5, Y: firstItemY, Button: tea.MouseLeft}
	m, _ = m.handleMouseClick(click)
	// Simulate the double-click window expiring before the second click.
	m.lastListClickAt = time.Now().Add(-2 * listDoubleClickWindow)
	m, _ = m.handleMouseClick(click)
	if m.showDetails {
		t.Fatalf("second click after window expiry should not open detail")
	}
}

// TestHandleMouseClick_SplitViewDoubleClickFocusesDetail: in split view the
// detail pane is already visible, so double-click focuses it rather than
// toggling showDetails (bt-f3zbz).
func TestHandleMouseClick_SplitViewDoubleClickFocusesDetail(t *testing.T) {
	m := NewModel(singlePaneTestIssues(), nil, "", nil, nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(60, 30)
	m.focused = focusList

	firstItemY := m.splitViewListChromeHeight()
	click := tea.MouseClickMsg{X: 5, Y: firstItemY, Button: tea.MouseLeft}

	m, _ = m.handleMouseClick(click)
	if m.focused != focusList {
		t.Fatalf("first click in split list pane should focus list, got %v", m.focused)
	}
	m, _ = m.handleMouseClick(click)
	if m.focused != focusDetail {
		t.Fatalf("double-click in split view should focus detail, got %v", m.focused)
	}
	if m.showDetails {
		t.Fatalf("split view double-click should not set showDetails (detail already visible)")
	}
}
