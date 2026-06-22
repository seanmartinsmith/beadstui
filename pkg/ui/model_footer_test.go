package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFooterData_StatusBarOverride(t *testing.T) {
	fd := FooterData{
		Width:       80,
		StatusMsg:   "Copied bt-abc1 to clipboard",
		StatusIsErr: false,
		FilterText:  "OPEN",
		FilterIcon:  "📂",
		TotalItems:  42,
	}
	out := fd.Render()
	if !strings.Contains(out, "Copied bt-abc1 to clipboard") {
		t.Errorf("status message should appear in output")
	}
	// When status is active, filter badge should NOT appear
	if strings.Contains(out, "OPEN") {
		t.Errorf("filter badge should not appear when status message is active")
	}
}

func TestFooterData_ErrorStatusBar(t *testing.T) {
	fd := FooterData{
		Width:       80,
		StatusMsg:   "No issue selected",
		StatusIsErr: true,
		TotalItems:  10,
	}
	out := fd.Render()
	if !strings.Contains(out, "No issue selected") {
		t.Errorf("error status message should appear in output")
	}
}

func TestFooterData_NormalFooter(t *testing.T) {
	fd := FooterData{
		Width:        120,
		FilterText:   "OPEN",
		FilterIcon:   "📂",
		HintText:     "l:labels",
		CountOpen:    10,
		CountReady:   5,
		CountBlocked: 2,
		CountClosed:  3,
		TotalItems:   20,
		Hints:        []FooterHint{{Key: "⏎", Desc: "details"}, {Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "OPEN") {
		t.Errorf("filter badge text should appear")
	}
	if !strings.Contains(out, "20 issues") {
		t.Errorf("issue count should appear")
	}
}

func TestFooterData_WorkspaceBadges(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "ALL",
		FilterIcon:       "📋",
		HintText:         "l:labels",
		WorkspaceMode:    true,
		WorkspaceSummary: "3 repos",
		RepoFilterLabel:  "bt, beads",
		TotalItems:       100,
		Hints:            []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "3 repos") {
		t.Errorf("workspace summary should appear")
	}
	if !strings.Contains(out, "bt, beads") {
		t.Errorf("repo filter label should appear")
	}
}

func TestFooterData_WorkerBadgeLevels(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		level WorkerLevel
		want  bool // should produce non-empty output
	}{
		{"none", "", WorkerLevelNone, false},
		{"info", "⠋ refreshing", WorkerLevelInfo, true},
		{"warning", "⚠ bg poll (5s)", WorkerLevelWarning, true},
		{"critical", "⚠ worker unresponsive", WorkerLevelCritical, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := FooterData{WorkerText: tt.text, WorkerLevel: tt.level}
			out := fd.renderWorkerBadge()
			if tt.want && out == "" {
				t.Errorf("expected non-empty worker badge for level %d", tt.level)
			}
			if !tt.want && out != "" {
				t.Errorf("expected empty worker badge for level %d", tt.level)
			}
		})
	}
}

func TestFooterData_AlertsBadge(t *testing.T) {
	fd := FooterData{AlertCount: 3, CriticalCount: 1, WarningCount: 2}
	out := fd.renderAlertsBadge()
	if !strings.Contains(out, "3 (!)") {
		t.Errorf("alert count and indicator should appear: %s", out)
	}
}

func TestFooterData_NoAlerts(t *testing.T) {
	fd := FooterData{AlertCount: 0}
	out := fd.renderAlertsBadge()
	if out != "" {
		t.Errorf("no alerts should produce empty badge")
	}
}

func TestFooterData_TimeTravelOverridesStats(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "OPEN",
		FilterIcon:       "📂",
		HintText:         "l:labels",
		TimeTravelActive: true,
		TimeTravelStats:  "⏱ 3d: +5 ✅2 ~3",
		TotalItems:       50,
		Hints:            []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "⏱ 3d") {
		t.Errorf("time travel stats should appear")
	}
}

func TestFooterData_SearchBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		SearchMode: "semantic",
		TotalItems: 30,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "semantic") {
		t.Errorf("search mode should appear in output")
	}
}

func TestFooterData_SortBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		SortLabel:  "priority",
		TotalItems: 30,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "priority") {
		t.Errorf("sort label should appear in output")
	}
}

func TestFooterData_ProgressiveHintTruncation(t *testing.T) {
	// Provide many hints in a narrow terminal — should truncate to fit
	fd := FooterData{
		Width:      40, // very narrow
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		TotalItems: 10,
		Hints: []FooterHint{
			{Key: "⏎", Desc: "details"},
			{Key: "t", Desc: "diff"},
			{Key: "S", Desc: "triage"},
			{Key: "l", Desc: "labels"},
			{Key: "?", Desc: "help"},
		},
	}
	out := fd.Render()
	// Just verify it renders without panic and produces output
	if lipgloss.Width(out) == 0 {
		t.Errorf("footer should produce non-empty output even when narrow")
	}
}

// TestFooterData_NeverWrapsAcrossWidths is the core guarantee of the smart
// footer: at every terminal width the rendered footer occupies exactly one row
// and never exceeds the column count (which the terminal would wrap, stealing a
// content line). Mirrors the real cross-project pain — 4921 issues, large stat
// numbers, an active alerts badge — that the 7-issue render harness understates.
func TestFooterData_NeverWrapsAcrossWidths(t *testing.T) {
	base := FooterData{
		FilterText:    "ALL",
		FilterIcon:    "📋",
		HintText:      "l:labels",
		CountOpen:     1811,
		CountReady:    1684,
		CountBlocked:  0,
		CountClosed:   3110,
		TotalItems:    4921,
		AlertCount:    1410,
		WarningCount:  1410,
		CriticalCount: 0,
		Hints: []FooterHint{
			{Key: "⏎", Desc: "open detail"},
			{Key: "o", Desc: "open issues"},
			{Key: "c", Desc: "copy"},
			{Key: "t", Desc: "diff"},
			{Key: "?", Desc: "help"},
		},
	}
	for w := 24; w <= 220; w++ {
		fd := base
		fd.Width = w
		out := fd.Render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width=%d: footer contains a newline (wrapped to 2 rows): %q", w, out)
		}
		if got := ansi.StringWidth(out); got > w {
			t.Fatalf("width=%d: footer display width %d exceeds terminal width (would wrap): %q",
				w, got, ansi.Strip(out))
		}
	}
}

// TestFooterData_NeverWrapsPathologicalBadge ensures a single oversized badge
// (e.g. a long BQL filter string) can't defeat the one-row guarantee — the
// final ANSI-aware truncate is the backstop.
func TestFooterData_NeverWrapsPathologicalBadge(t *testing.T) {
	fd := FooterData{
		Width:      50,
		FilterText: "BQL: status=open AND label=area:tui AND priority<=2 AND assignee=sms",
		FilterIcon: "🔍",
		HintText:   "l:labels",
		TotalItems: 4921,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if strings.Contains(out, "\n") {
		t.Fatalf("pathological filter wrapped the footer: %q", out)
	}
	if got := ansi.StringWidth(out); got > fd.Width {
		t.Fatalf("pathological filter overran width: %d > %d: %q", got, fd.Width, ansi.Strip(out))
	}
}

// TestFooterCounts_ScopedToActiveFilter proves the footer's status breakdown
// reflects exactly what the list shows (active scope + filter), not the global
// corpus — the bt-gcuv generalization. The breakdown is computed at the
// setListItems chokepoint, so applying a filter must reshape it.
func TestFooterCounts_ScopedToActiveFilter(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = nm.(Model)

	// All: open + closed accounts for every visible item.
	m.filter.currentFilter = "all"
	m.applyFilter()
	if got := m.ac.countOpen + m.ac.countClosed; got != len(m.list.Items()) {
		t.Fatalf("all filter: open+closed=%d but %d items visible", got, len(m.list.Items()))
	}

	// Open: the scoped breakdown must contain zero closed.
	m.filter.currentFilter = "open"
	m.applyFilter()
	if m.ac.countClosed != 0 {
		t.Errorf("open filter: scoped breakdown should show 0 closed, got %d", m.ac.countClosed)
	}
	if m.ac.countOpen != len(m.list.Items()) {
		t.Errorf("open filter: countOpen=%d should equal visible items %d", m.ac.countOpen, len(m.list.Items()))
	}

	// Closed: the scoped breakdown must contain zero open/ready/blocked.
	m.filter.currentFilter = "closed"
	m.applyFilter()
	if m.ac.countOpen != 0 || m.ac.countReady != 0 || m.ac.countBlocked != 0 {
		t.Errorf("closed filter: scoped breakdown should be all-closed, got open=%d ready=%d blocked=%d",
			m.ac.countOpen, m.ac.countReady, m.ac.countBlocked)
	}
}

// TestFooterData_CenterOverride proves the Phase 3 per-view center override
// replaces the scoped status stats + count: the override string appears and the
// "N issues" count badge is suppressed (the override carries its own count).
func TestFooterData_CenterOverride(t *testing.T) {
	fd := FooterData{
		Width:          120,
		FilterText:     "bt",
		FilterIcon:     "📂",
		HintText:       "l:labels",
		CountOpen:      163,
		CountReady:     2,
		CountClosed:    4,
		TotalItems:     169,
		CenterOverride: "bt-0qzp · 3/169",
		Hints:          []FooterHint{{Key: "esc", Desc: "back"}, {Key: "?", Desc: "help"}},
	}
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "bt-0qzp · 3/169") {
		t.Errorf("center override should appear in footer: %q", out)
	}
	if strings.Contains(out, "169 issues") {
		t.Errorf("count badge should be suppressed when a center override is set: %q", out)
	}
	// The scoped status glyphs belong to the default center; the override takes
	// their place, so the open-count number should not surface as a stat segment.
	if strings.Contains(out, "○163") {
		t.Errorf("scoped status stats should not render alongside a center override: %q", out)
	}
}

// TestFooterData_CenterOverrideTimeTravelPrecedence ensures the corpus-wide time
// travel diff out-ranks a per-view center override (the diff is the more
// important signal while time-travelling).
func TestFooterData_CenterOverrideTimeTravelPrecedence(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "bt",
		FilterIcon:       "📂",
		HintText:         "l:labels",
		TimeTravelActive: true,
		TimeTravelStats:  "⏱ 3d: +5 ✅2 ~3",
		CenterOverride:   "47 nodes · 61 edges",
		TotalItems:       169,
		Hints:            []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "⏱ 3d") {
		t.Errorf("time travel diff should win over center override: %q", out)
	}
	if strings.Contains(out, "47 nodes") {
		t.Errorf("center override should yield to active time travel: %q", out)
	}
}

// TestFooterData_CenterOverrideNeverWraps extends the one-row guarantee to the
// override center zone: at every width the footer stays a single row within the
// column count, and the override drops cleanly under extreme pressure.
func TestFooterData_CenterOverrideNeverWraps(t *testing.T) {
	base := FooterData{
		FilterText:     "bt",
		FilterIcon:     "📂",
		HintText:       "l:labels",
		TotalItems:     169,
		CenterOverride: "bt-0qzp · 3/169",
		Hints: []FooterHint{
			{Key: "esc", Desc: "back"},
			{Key: "C", Desc: "copy"},
			{Key: "O", Desc: "edit"},
			{Key: "?", Desc: "help"},
		},
	}
	for w := 24; w <= 220; w++ {
		fd := base
		fd.Width = w
		out := fd.Render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width=%d: footer with center override wrapped to 2 rows: %q", w, out)
		}
		if got := ansi.StringWidth(out); got > w {
			t.Fatalf("width=%d: footer with center override overran width %d: %q", w, got, ansi.Strip(out))
		}
	}
}

// TestFooterCenter_PerView proves footerCenter() returns view-appropriate
// meaning: detail = bead id + position, graph = nodes/edges, board =
// columns/cards, and plain list = "" (keeps the default scoped counts).
func TestFooterCenter_PerView(t *testing.T) {
	newModel := func() Model {
		m := NewModel(harnessIssues(), nil, "", nil)
		nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		return nm.(Model)
	}

	// Plain list keeps the default (no override).
	t.Run("list", func(t *testing.T) {
		m := newModel()
		if got := m.footerCenter(); got != "" {
			t.Errorf("list view should have no center override, got %q", got)
		}
	})

	// Detail (full-screen) = selected bead id + 1-based position / visible total.
	t.Run("detail", func(t *testing.T) {
		m := newModel()
		harnessSelect(&m, "bt-0qzp")
		m.showDetails = true
		m.focused = focusDetail
		got := m.footerCenter()
		if !strings.HasPrefix(got, "bt-0qzp · ") {
			t.Errorf("detail center should lead with the selected bead id, got %q", got)
		}
		total := len(m.list.VisibleItems())
		if want := fmt.Sprintf("bt-0qzp · %d/%d", m.list.Index()+1, total); got != want {
			t.Errorf("detail center = %q, want %q", got, want)
		}
	})

	// Graph = nodes/edges.
	t.Run("graph", func(t *testing.T) {
		m := newModel()
		m.mode = ViewGraph
		m.focused = focusGraph
		m.refreshBoardAndGraphForCurrentFilter()
		want := fmt.Sprintf("%s · %s",
			countLabel(m.graphView.TotalCount(), "node"),
			countLabel(m.graphView.EdgeCount(), "edge"))
		if got := m.footerCenter(); got != want {
			t.Errorf("graph center = %q, want %q", got, want)
		}
	})

	// Board = visible columns / cards.
	t.Run("board", func(t *testing.T) {
		m := newModel()
		m.mode = ViewBoard
		m.focused = focusBoard
		m.refreshBoardAndGraphForCurrentFilter()
		want := fmt.Sprintf("%s · %s",
			countLabel(m.board.VisibleColumnCount(), "col"),
			countLabel(m.board.TotalCount(), "card"))
		if got := m.footerCenter(); got != want {
			t.Errorf("board center = %q, want %q", got, want)
		}
	})

	// A modal over any view suppresses the override (keep underlying counts).
	t.Run("modal_suppresses", func(t *testing.T) {
		m := newModel()
		m.mode = ViewGraph
		m.focused = focusGraph
		m.refreshBoardAndGraphForCurrentFilter()
		m.activeModal = ModalHelp
		if got := m.footerCenter(); got != "" {
			t.Errorf("modal should suppress center override, got %q", got)
		}
	})
}

// TestCountLabel_Pluralization covers the singular/plural boundary.
func TestCountLabel_Pluralization(t *testing.T) {
	cases := []struct {
		n    int
		word string
		want string
	}{
		{0, "node", "0 nodes"},
		{1, "node", "1 node"},
		{2, "edge", "2 edges"},
		{4, "col", "4 cols"},
		{1, "card", "1 card"},
	}
	for _, c := range cases {
		if got := countLabel(c.n, c.word); got != c.want {
			t.Errorf("countLabel(%d, %q) = %q, want %q", c.n, c.word, got, c.want)
		}
	}
}

// TestFooterPinnedToBottomRow asserts the View()-level invariant that the
// footer is always the final terminal row, across every view mode. It guards
// the bt-yyked fix: under-filling views (detail/actionable) used to leave the
// footer floating mid-screen, and over-filling views (graph/insights) used to
// push it past the bottom where it was clipped away entirely.
func TestFooterPinnedToBottomRow(t *testing.T) {
	cases := []struct {
		name  string
		h     int
		setup func(*Model)
	}{
		{"list", 24, nil},
		{"detail", 28, func(m *Model) {
			harnessSelect(m, "bt-0qzp")
			m.showDetails = true
			m.focused = focusDetail
			m.updateViewportContent()
		}},
		{"graph", 32, func(m *Model) {
			m.mode = ViewGraph
			m.focused = focusGraph
			m.refreshBoardAndGraphForCurrentFilter()
		}},
		{"graph_scrunched", 20, func(m *Model) {
			m.mode = ViewGraph
			m.focused = focusGraph
			m.refreshBoardAndGraphForCurrentFilter()
		}},
		{"board", 32, func(m *Model) {
			m.mode = ViewBoard
			m.focused = focusBoard
			m.refreshBoardAndGraphForCurrentFilter()
		}},
		{"actionable", 32, func(m *Model) {
			m.mode = ViewActionable
			m.focused = focusActionable
		}},
		{"insights", 32, func(m *Model) { m.openInsightsView() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := NewModel(harnessIssues(), nil, "", nil)
			nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: c.h})
			m = nm.(Model)
			if c.setup != nil {
				c.setup(&m)
			}
			content := ansi.Strip(m.View().Content)
			lines := strings.Split(content, "\n")
			if len(lines) != c.h {
				t.Fatalf("rendered %d rows, want exactly terminal height %d", len(lines), c.h)
			}
			gotLast := strings.TrimRight(lines[len(lines)-1], " ")
			wantFooter := strings.TrimRight(ansi.Strip(m.renderFooter()), " ")
			if gotLast != wantFooter {
				t.Errorf("footer is not the final row.\n last row: %q\n footer:   %q", gotLast, wantFooter)
			}
		})
	}
}

func TestFooterData_UpdateBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "l:labels",
		UpdateTag:  "v0.2.0",
		TotalItems: 10,
		Hints:      []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "v0.2.0") {
		t.Errorf("update tag should appear in output")
	}
}

func TestFooterData_SecondaryInstance(t *testing.T) {
	fd := FooterData{
		Width:        120,
		FilterText:   "ALL",
		FilterIcon:   "📋",
		HintText:     "l:labels",
		SecondaryPID: 12345,
		TotalItems:   10,
		Hints:        []FooterHint{{Key: "?", Desc: "help"}},
	}
	out := fd.Render()
	if !strings.Contains(out, "12345") {
		t.Errorf("secondary PID should appear in output")
	}
}
