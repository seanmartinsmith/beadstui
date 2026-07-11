package ui

import (
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"

	"github.com/charmbracelet/x/ansi"
)

// TestCalcExpandedColumnWidths_MixedEmptyAndPopulated verifies that when
// only some visible columns hold cards, the populated columns are given a
// much larger share of the board than the equal-division normalWidth,
// while empty columns shrink (but stay non-zero) to free up that space
// (bt-lgbz).
func TestCalcExpandedColumnWidths_MixedEmptyAndPopulated(t *testing.T) {
	// 4 visible columns, only 1 populated (e.g. OPEN has cards, the rest
	// - IN_PROGRESS/BLOCKED/CLOSED - are empty). This is exactly the bug
	// scenario: normalWidth caps the expanded card to 1/4 of the board.
	boardWidth := 160
	numCols := 4
	populatedCols := 1
	normalWidth := 38
	borderOverhead := 2

	populatedWidth, emptyWidth := calcExpandedColumnWidths(boardWidth, numCols, populatedCols, normalWidth, borderOverhead)

	if populatedWidth != 110 {
		t.Errorf("populatedWidth = %d, want 110", populatedWidth)
	}
	if emptyWidth != 14 {
		t.Errorf("emptyWidth = %d, want 14", emptyWidth)
	}
	if populatedWidth <= normalWidth {
		t.Errorf("populatedWidth (%d) should be wider than normalWidth (%d) when columns are mostly empty", populatedWidth, normalWidth)
	}
	if emptyWidth <= 0 {
		t.Errorf("emptyWidth (%d) must stay positive - empty columns must not be hidden", emptyWidth)
	}

	// Total rendered width (including borders) must never exceed the
	// board width, or the row will overflow into the next panel (#116-class
	// corruption).
	total := populatedCols*(populatedWidth+borderOverhead) + (numCols-populatedCols)*(emptyWidth+borderOverhead)
	if total > boardWidth {
		t.Errorf("total column width %d exceeds boardWidth %d", total, boardWidth)
	}
}

// TestCalcExpandedColumnWidths_TwoOfFourPopulated covers a mixed case with
// more than one populated column to make sure the redistribution divides
// the reclaimed space across all of them, not just one.
func TestCalcExpandedColumnWidths_TwoOfFourPopulated(t *testing.T) {
	boardWidth := 160
	numCols := 4
	populatedCols := 2
	normalWidth := 38
	borderOverhead := 2

	populatedWidth, emptyWidth := calcExpandedColumnWidths(boardWidth, numCols, populatedCols, normalWidth, borderOverhead)

	if populatedWidth != 62 {
		t.Errorf("populatedWidth = %d, want 62", populatedWidth)
	}
	if emptyWidth != 14 {
		t.Errorf("emptyWidth = %d, want 14", emptyWidth)
	}
}

// TestCalcExpandedColumnWidths_AllPopulatedNoChange verifies the normal
// (non-expanded-relevant) case where every visible column has cards: there
// is nothing to redistribute, so both widths must equal normalWidth
// exactly, leaving today's equal-division layout untouched.
func TestCalcExpandedColumnWidths_AllPopulatedNoChange(t *testing.T) {
	boardWidth := 160
	numCols := 4
	populatedCols := 4
	normalWidth := 38
	borderOverhead := 2

	populatedWidth, emptyWidth := calcExpandedColumnWidths(boardWidth, numCols, populatedCols, normalWidth, borderOverhead)

	if populatedWidth != normalWidth {
		t.Errorf("populatedWidth = %d, want unchanged normalWidth %d", populatedWidth, normalWidth)
	}
	if emptyWidth != normalWidth {
		t.Errorf("emptyWidth = %d, want unchanged normalWidth %d", emptyWidth, normalWidth)
	}
}

// TestCalcExpandedColumnWidths_ZeroPopulatedGuard verifies the
// divide-by-zero guard: when every visible column is empty (populatedCols
// == 0, e.g. a completely empty board), the function must not panic and
// must fall back to normalWidth for both results.
func TestCalcExpandedColumnWidths_ZeroPopulatedGuard(t *testing.T) {
	boardWidth := 160
	numCols := 4
	populatedCols := 0
	normalWidth := 38
	borderOverhead := 2

	populatedWidth, emptyWidth := calcExpandedColumnWidths(boardWidth, numCols, populatedCols, normalWidth, borderOverhead)

	if populatedWidth != normalWidth {
		t.Errorf("populatedWidth = %d, want fallback normalWidth %d", populatedWidth, normalWidth)
	}
	if emptyWidth != normalWidth {
		t.Errorf("emptyWidth = %d, want fallback normalWidth %d", emptyWidth, normalWidth)
	}
}

// TestCalcExpandedColumnWidths_NeverBelowNormal is a regression guard: on a
// very narrow board there may be no slack to redistribute. In that case the
// populated width must never shrink below the existing normalWidth used
// today - the fix must only ever help, never regress the width the
// expanded card already gets.
func TestCalcExpandedColumnWidths_NeverBelowNormal(t *testing.T) {
	cases := []struct {
		boardWidth, numCols, populatedCols, normalWidth, borderOverhead int
	}{
		{40, 4, 1, 8, 2},   // very narrow terminal
		{160, 4, 1, 38, 2}, // bug scenario
		{160, 4, 3, 38, 2}, // 3 of 4 populated
		{200, 5, 1, 38, 2}, // more columns
	}
	for _, c := range cases {
		populatedWidth, emptyWidth := calcExpandedColumnWidths(c.boardWidth, c.numCols, c.populatedCols, c.normalWidth, c.borderOverhead)
		if populatedWidth < c.normalWidth {
			t.Errorf("case %+v: populatedWidth %d < normalWidth %d (regression)", c, populatedWidth, c.normalWidth)
		}
		if emptyWidth > c.normalWidth {
			t.Errorf("case %+v: emptyWidth %d exceeds normalWidth %d", c, emptyWidth, c.normalWidth)
		}
		if emptyWidth <= 0 {
			t.Errorf("case %+v: emptyWidth %d must stay positive", c, emptyWidth)
		}
	}
}

// TestExpandedCardKeepsEmptyColumnsVisible is an integration-level check
// that board.go's View() actually wires up calcExpandedColumnWidths
// (bt-lgbz). With 4 visible status columns and only OPEN populated,
// expanding a card must not hide the other 3 empty columns (they must
// stay onscreen, just narrower), and the rendered row must still fit
// within the requested board width - no overflow/corruption (#116-class).
func TestExpandedCardKeepsEmptyColumnsVisible(t *testing.T) {
	theme := DefaultTheme()
	issues := []model.Issue{
		{ID: "wide-1", Title: "First Issue", Status: model.StatusOpen},
		{ID: "wide-2", Title: "Second Issue", Status: model.StatusOpen},
	}
	b := NewBoardModel(issues, theme)

	const boardWidth = 160
	before := b.View(boardWidth, 40)
	if got := strings.Count(before, "(empty)"); got != 3 {
		t.Fatalf("before expand: expected 3 empty column placeholders, got %d\n%s", got, before)
	}

	b.ToggleExpand()
	if b.VisibleColumnCount() != 4 {
		t.Fatalf("expanding a card must not hide empty columns, got VisibleColumnCount=%d", b.VisibleColumnCount())
	}

	after := b.View(boardWidth, 40)
	if got := strings.Count(after, "(empty)"); got != 3 {
		t.Errorf("expected empty columns to remain visible (not hidden) while a card is expanded, got %d placeholders\n%s", got, after)
	}

	for i, line := range strings.Split(after, "\n") {
		if w := ansi.StringWidth(line); w > boardWidth {
			t.Errorf("line %d overflows board width: got %d, want <= %d\nline: %q", i, w, boardWidth, line)
		}
	}
}
