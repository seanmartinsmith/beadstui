package ui

// Selection peek strip (bt-evuf.3).
//
// A fixed 4-row block pinned below the issues list showing the selected bead's
// untruncated title plus the facts a row has no width to carry. It updates as
// the selection moves.
//
// Why a strip and not an expanding row: bubbles/list takes a single
// delegate-wide Height() and uses it for pagination and mouse hit-testing, so
// expanding only the selected row would desync both. A strip below the list
// costs its rows once regardless of list length, and sits at a fixed screen
// position so the eye doesn't chase it while arrowing.
//
// Shown only when the issues pane is fullscreen ("2"). In split view the
// details pane already shows all of this, so the strip would spend four rows
// repeating the pane next to it.

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// peekStripHeight is the fixed row cost of the strip: one rule, two title
// lines, one meta line. Fixed rather than content-dependent so the list height
// never changes as the selection moves, which would make the list jump under
// the cursor.
const peekStripHeight = 4

// peekTitleLines is how many of the strip's rows the title may occupy.
const peekTitleLines = 2

// showPeekStrip reports whether the selection peek strip is part of the
// current layout. Kept as one predicate so the renderer and the height
// arithmetic in applyListDetailSizing can never disagree about whether the
// strip is present — a mismatch would shift every list row by four.
func (m Model) showPeekStrip() bool {
	return m.fullscreen == fullscreenIssues
}

// renderPeekStrip draws the strip at the given inner width. Returns "" when the
// strip is not part of the layout or there is no selection, and always renders
// exactly peekStripHeight rows otherwise.
func (m Model) renderPeekStrip(width int) string {
	if !m.showPeekStrip() || width <= 0 {
		return ""
	}
	item, ok := m.list.SelectedItem().(IssueItem)
	if !ok {
		return ""
	}
	t := m.theme
	issue := item.Issue

	rule := lipgloss.NewStyle().
		Foreground(t.Border).
		Render(strings.Repeat("─", width))

	// Title, wrapped rather than truncated: showing the whole title is the
	// entire reason the strip exists.
	titleBlock := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Width(width).
		Render(issue.Title)
	titleRows := strings.Split(titleBlock, "\n")
	if len(titleRows) > peekTitleLines {
		// Mark the elision so a clipped long title doesn't read as the whole
		// one.
		titleRows = titleRows[:peekTitleLines]
		last := titleRows[peekTitleLines-1]
		titleRows[peekTitleLines-1] = truncateRunesHelper(last, width-1, "") + activeGlyphs.Ellipsis
	}
	for len(titleRows) < peekTitleLines {
		titleRows = append(titleRows, "")
	}

	meta := lipgloss.NewStyle().
		Foreground(t.Muted).
		Width(width).
		MaxHeight(1).
		Render(m.peekMetaLine(item))

	rows := append([]string{rule}, titleRows...)
	rows = append(rows, meta)
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// peekMetaLine builds the strip's single fact line: the fields a narrow row
// cannot afford, in decreasing order of how often they decide whether to open
// the bead.
func (m Model) peekMetaLine(item IssueItem) string {
	issue := item.Issue
	sep := " " + activeGlyphs.Sep + " "

	var parts []string
	if m.workspaceMode && item.RepoPrefix != "" {
		parts = append(parts, model.DisplayRepoName(item.RepoPrefix))
	}
	parts = append(parts, issue.ID)
	parts = append(parts, fmt.Sprintf("%s P%d", strings.ReplaceAll(string(issue.Status), "_", " "), issue.Priority))
	parts = append(parts, FormatTimeRel(issue.UpdatedAt))

	if issue.Assignee != "" {
		parts = append(parts, "@"+issue.Assignee)
	}
	// Blocker impact is the fact most likely to change what you pick next, so
	// it earns a slot ahead of labels.
	if item.UnblocksCount > 0 {
		parts = append(parts, fmt.Sprintf("unblocks %d", item.UnblocksCount))
	}
	if len(issue.Labels) > 0 {
		parts = append(parts, strings.Join(issue.Labels, " "))
	}
	return strings.Join(parts, sep)
}
