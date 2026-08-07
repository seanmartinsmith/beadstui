package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// fullscreenIssuesModel returns a Model maximized on the issues pane, which is
// the only layout the peek strip participates in.
func fullscreenIssuesModel(t *testing.T, w, h int) Model {
	t.Helper()
	m := sizedFullscreenModel(t, w, h)
	m.toggleFullscreenPane(fullscreenIssues) // focus
	if m.fullscreen != fullscreenIssues {
		m.toggleFullscreenPane(fullscreenIssues) // maximize (two-stage in split)
	}
	if m.fullscreen != fullscreenIssues {
		t.Fatalf("could not reach fullscreen issues, got %d", m.fullscreen)
	}
	return m
}

// TestPeekStripHeightMatchesSizing is the invariant that keeps the list inside
// its panel. renderPeekStrip and applyListDetailSizing are gated on the same
// predicate and must agree on the row cost; if the renderer emits more rows
// than the sizing subtracts, the list overflows and every row shifts.
func TestPeekStripHeightMatchesSizing(t *testing.T) {
	m := fullscreenIssuesModel(t, 120, 32)

	strip := m.renderPeekStrip(m.list.Width())
	if strip == "" {
		t.Fatal("expected a peek strip in fullscreen issues")
	}
	if got := lipgloss.Height(strip); got != peekStripHeight {
		t.Errorf("rendered strip is %d rows, but sizing reserves %d", got, peekStripHeight)
	}
}

// TestPeekStripAbsentOutsideFullscreenIssues guards the scoping decision: in
// split view the details pane already shows everything the strip would, so
// spending four rows to repeat the pane beside it would be a regression.
func TestPeekStripAbsentOutsideFullscreenIssues(t *testing.T) {
	split := sizedFullscreenModel(t, 120, 32)
	if !split.isSplitView {
		t.Fatal("expected split view at 120 wide")
	}
	if split.showPeekStrip() {
		t.Error("peek strip should not participate in split view")
	}
	if got := split.renderPeekStrip(split.list.Width()); got != "" {
		t.Errorf("expected no strip in split view, got %q", got)
	}

	details := sizedFullscreenModel(t, 120, 32)
	details.toggleFullscreenPane(fullscreenDetails)
	details.toggleFullscreenPane(fullscreenDetails)
	if details.fullscreen == fullscreenDetails && details.showPeekStrip() {
		t.Error("peek strip should not participate in fullscreen details")
	}
}

// TestPeekStripShowsUntruncatedTitle is the strip's whole purpose: the row it
// sits under truncates titles to whatever width is left after the columns, and
// this is where the full text becomes readable.
func TestPeekStripShowsUntruncatedTitle(t *testing.T) {
	m := fullscreenIssuesModel(t, 120, 32)
	item, ok := m.list.SelectedItem().(IssueItem)
	if !ok {
		t.Fatal("no selection")
	}
	strip := m.renderPeekStrip(m.list.Width())

	// The fixture's selected title is long enough to be truncated in a row but
	// must appear whole here.
	if !strings.Contains(strip, item.Issue.Title) {
		t.Errorf("strip does not contain the full title %q", item.Issue.Title)
	}
	// And the meta line must carry the ID, which is what makes the strip
	// actionable rather than decorative.
	if !strings.Contains(strip, item.Issue.ID) {
		t.Errorf("strip does not contain the bead ID %q", item.Issue.ID)
	}
}

// TestPeekStripElidesOverlongTitle checks that a title too long for the strip's
// two lines is marked as clipped rather than silently cut, which would read as
// the whole title.
func TestPeekStripElidesOverlongTitle(t *testing.T) {
	m := fullscreenIssuesModel(t, 120, 32)
	items := m.list.Items()
	if len(items) == 0 {
		t.Fatal("no items")
	}
	item := items[0].(IssueItem)
	item.Issue.Title = strings.Repeat("very long title segment ", 30)
	m.list.SetItem(0, item)
	m.list.Select(0)

	strip := m.renderPeekStrip(40)
	if got := lipgloss.Height(strip); got != peekStripHeight {
		t.Fatalf("overlong title made the strip %d rows, want %d", got, peekStripHeight)
	}
	if !strings.Contains(strip, activeGlyphs.Ellipsis) {
		t.Error("clipped title is not marked with an ellipsis")
	}
}

// TestPeekStripNarrowWidthDoesNotPanic covers the degenerate widths the layout
// can hand a renderer mid-resize.
func TestPeekStripNarrowWidthDoesNotPanic(t *testing.T) {
	m := fullscreenIssuesModel(t, 120, 32)
	for _, w := range []int{-5, 0, 1, 2, 5} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("width %d panicked: %v", w, r)
				}
			}()
			_ = m.renderPeekStrip(w)
		}()
	}
}
