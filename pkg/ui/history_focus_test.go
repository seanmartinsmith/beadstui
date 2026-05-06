package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// createRichHistoryReport builds a report with lifecycle events on every
// bead -- exercises the LIFECYCLE block in renderDetailPanel that the thin
// fixture used by createTestHistoryReport never reaches. Closer to the
// dogfood scenario where pane drift was visible.
func createRichHistoryReport() *correlation.HistoryReport {
	now := time.Now()
	mkEvents := func(beadID string) []correlation.BeadEvent {
		return []correlation.BeadEvent{
			{BeadID: beadID, EventType: correlation.EventCreated, Timestamp: now.Add(-72 * time.Hour), Author: "Dev One", CommitSHA: "aaa1111111"},
			{BeadID: beadID, EventType: correlation.EventClaimed, Timestamp: now.Add(-48 * time.Hour), Author: "Dev One", CommitSHA: "bbb2222222"},
			{BeadID: beadID, EventType: correlation.EventClosed, Timestamp: now.Add(-2 * time.Hour), Author: "Dev One", CommitSHA: "ccc3333333"},
		}
	}
	mkCommits := func() []correlation.CorrelatedCommit {
		var out []correlation.CorrelatedCommit
		for i := 0; i < 12; i++ {
			out = append(out, correlation.CorrelatedCommit{
				SHA:        "0123456789abcdef0123456789abcdef" + intToStr(i) + "0",
				ShortSHA:   "abc1234",
				Message:    "fix: lots of changes here, repeated commit text to ensure long content",
				Author:     "Dev One",
				Timestamp:  now.Add(time.Duration(-i) * time.Hour),
				Method:     correlation.MethodCoCommitted,
				Confidence: 0.9,
				Files: []correlation.FileChange{
					{Path: "pkg/foo/file.go", Action: "M", Insertions: 10, Deletions: 5},
					{Path: "pkg/bar/baz.go", Action: "M", Insertions: 3, Deletions: 2},
				},
			})
		}
		return out
	}

	beadIDs := []string{"bv-1", "bv-2", "bv-3"}
	hists := make(map[string]correlation.BeadHistory, len(beadIDs))
	for i, id := range beadIDs {
		hists[id] = correlation.BeadHistory{
			BeadID:  id,
			Title:   "Rich title for bead " + id,
			Status:  []string{"closed", "open", "in_progress"}[i],
			Commits: mkCommits(),
			Events:  mkEvents(id),
		}
	}

	return &correlation.HistoryReport{
		GeneratedAt: now,
		Stats: correlation.HistoryStats{
			TotalBeads:       len(beadIDs),
			BeadsWithCommits: len(beadIDs),
			TotalCommits:     12 * len(beadIDs),
			UniqueAuthors:    1,
		},
		Histories: hists,
	}
}

// TestHistoryFocusCycleStableDimensions covers bt-5224.
//
// Pressing `tab` in the History view to cycle focus through panes must NOT
// change the rendered output's outer dimensions or per-pane row heights.
// Only colors (border + title) should change. Dogfooding 2026-05-06 showed
// pane heights drifting between focus states, with content from the previous
// frame leaking through where the new frame didn't fully cover.
//
// Strategy: render the View at each focus state, measure (a) total row count,
// (b) total column width per row, (c) the border-row positions (where the
// horizontal border characters appear). All three must match across states.
func TestHistoryFocusCycleStableDimensions(t *testing.T) {
	report := createRichHistoryReport()
	theme := testTheme()

	h := NewHistoryModel(report, theme)
	// Use a width comfortably above the wide-layout threshold (>150) so we
	// exercise the four-pane wide layout where the symptom was reported.
	h.SetSize(180, 50)

	type frameStats struct {
		rows         int
		maxCol       int
		rowWidths    []int  // visible width of each row, in order
		borderRowSet string // serialized set of row indices that contain the horizontal border char
	}

	captureFrame := func(label string) frameStats {
		out := h.View()
		if out == "" {
			t.Fatalf("%s: empty View output", label)
		}
		lines := strings.Split(out, "\n")
		// Drop trailing empty line(s) that result from a final \n in content.
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		stats := frameStats{rows: len(lines)}
		var borderRows []int
		for i, line := range lines {
			w := lipgloss.Width(line)
			stats.rowWidths = append(stats.rowWidths, w)
			if w > stats.maxCol {
				stats.maxCol = w
			}
			// A row that contains the border horizontal char ("─") and no
			// pane content is a top/bottom-of-pane border row. We use a
			// looser heuristic: any row dominated by horizontal-border or
			// corner glyphs counts as a border row.
			if strings.ContainsAny(line, "─╭╮╰╯┌┐└┘") {
				// Only count rows where the horizontal border is the
				// majority of visible content. Distinguishes panel borders
				// from incidental box-drawing chars used inside content.
				stripped := stripANSI(line)
				borderCount := strings.Count(stripped, "─")
				if borderCount > 5 {
					borderRows = append(borderRows, i)
				}
			}
		}
		// Stable serialization of border row set for comparison.
		var ids []string
		for _, r := range borderRows {
			ids = append(ids, intToStr(r))
		}
		stats.borderRowSet = strings.Join(ids, ",")
		return stats
	}

	rowWidthsEqual := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	// Cycle through all three focus states and capture each.
	h.focused = historyFocusList
	listStats := captureFrame("focus=list")

	h.focused = historyFocusMiddle
	middleStats := captureFrame("focus=middle")

	h.focused = historyFocusDetail
	detailStats := captureFrame("focus=detail")

	// Total row count must be identical across focus states. Any drift means
	// pane heights are responding to focus.
	if listStats.rows != middleStats.rows {
		t.Errorf("row count drift list->middle: %d vs %d", listStats.rows, middleStats.rows)
	}
	if listStats.rows != detailStats.rows {
		t.Errorf("row count drift list->detail: %d vs %d", listStats.rows, detailStats.rows)
	}

	// Max column width per state must match — any drift means a pane is
	// rendering at a different inner width when focused.
	if listStats.maxCol != middleStats.maxCol {
		t.Errorf("max col drift list->middle: %d vs %d", listStats.maxCol, middleStats.maxCol)
	}
	if listStats.maxCol != detailStats.maxCol {
		t.Errorf("max col drift list->detail: %d vs %d", listStats.maxCol, detailStats.maxCol)
	}

	// Border-row positions must match — these are the load-bearing signal
	// that pane TOP/BOTTOM rows haven't shifted vertically.
	if listStats.borderRowSet != middleStats.borderRowSet {
		t.Errorf("border row drift list->middle:\n  list:   %s\n  middle: %s",
			listStats.borderRowSet, middleStats.borderRowSet)
	}
	if listStats.borderRowSet != detailStats.borderRowSet {
		t.Errorf("border row drift list->detail:\n  list:   %s\n  detail: %s",
			listStats.borderRowSet, detailStats.borderRowSet)
	}

	// Per-row widths must match exactly. This is the strongest test: any
	// row that's wider/narrower in one focus state than another means a
	// pane content line has expanded or contracted, which would make the
	// next frame fail to fully cover the previous one and produce the
	// "stale residue" symptom from the dogfood images.
	if !rowWidthsEqual(listStats.rowWidths, middleStats.rowWidths) {
		t.Errorf("per-row width drift list->middle (rows: %d vs %d)", len(listStats.rowWidths), len(middleStats.rowWidths))
	}
	if !rowWidthsEqual(listStats.rowWidths, detailStats.rowWidths) {
		t.Errorf("per-row width drift list->detail (rows: %d vs %d)", len(listStats.rowWidths), len(detailStats.rowWidths))
	}
}

// TestHistoryTabCycleAtModelLevel covers bt-5224 at the Model.View boundary
// (one level above HistoryModel.View). Catches dimension drift introduced by
// the wrapping composition path in model_view.go - sidebar overlays, footer
// height accounting, finalStyle truncation - that the HistoryModel-level test
// would miss.
//
// Pre-populates a HistoryModel so we can exercise focus cycling without
// having to wait for an async load.
func TestHistoryTabCycleAtModelLevel(t *testing.T) {
	// Build a model the proper way (NewModel initializes the DataState
	// pointer and other internals; a bare Model{} literal panics in View
	// because m.data is *DataState). Then prime the historyView with our
	// rich fixture and force ViewHistory.
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test 1", Status: model.StatusClosed},
		{ID: "bv-2", Title: "Test 2", Status: model.StatusOpen},
		{ID: "bv-3", Title: "Test 3", Status: model.StatusInProgress},
	}
	m := NewModel(issues, nil, "", nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	report := createRichHistoryReport()
	m.historyView = NewHistoryModel(report, m.theme)
	m.historyView.SetSize(180, 49)
	m.mode = ViewHistory
	m.focused = focusHistory
	m.historyLoading = false

	captureRows := func(label string) int {
		view := m.View()
		content := view.Content
		if content == "" {
			t.Fatalf("%s: empty View content", label)
		}
		lines := strings.Split(content, "\n")
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return len(lines)
	}

	m.historyView.focused = historyFocusList
	listRows := captureRows("focus=list")

	m.historyView.focused = historyFocusMiddle
	middleRows := captureRows("focus=middle")

	m.historyView.focused = historyFocusDetail
	detailRows := captureRows("focus=detail")

	if listRows != middleRows {
		t.Errorf("Model.View row drift list->middle: %d vs %d", listRows, middleRows)
	}
	if listRows != detailRows {
		t.Errorf("Model.View row drift list->detail: %d vs %d", listRows, detailRows)
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = '0' + byte(n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
