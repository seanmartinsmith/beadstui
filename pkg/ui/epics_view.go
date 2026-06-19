package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// refreshEpicsForCurrentFilter rebuilds the epics overview rows from the
// current scope + label filter and re-renders the cached view text. It is the
// epics analog of refreshBoardAndGraphForCurrentFilter: called on view-switch,
// on filter/recipe change, and on data reload (only while ViewEpics is active).
//
// Sourcing note (the one place the "projection over filteredIssuesForActiveView"
// rule is overridden): the progress bar must count closed children in full, but
// the list's status filter (open/ready) drops closed issues. So we source from
// the scope + label + wisp-filtered set WITHOUT the status filter, and let
// m.epicsStatusMode (active/all/completed) decide which epics to list. See the
// epics-view design's "status filter override".
func (m *Model) refreshEpicsForCurrentFilter() {
	if m.mode != ViewEpics {
		return
	}

	issues := m.workspacePrefilter(m.data.issues)
	scoped := make([]model.Issue, 0, len(issues))
	for _, issue := range issues {
		// Skip wisps when hidden (bt-9kdo), mirroring filteredIssuesForActiveView.
		if !m.showWisps && issue.Ephemeral != nil && *issue.Ephemeral {
			continue
		}
		if m.filter.labelFilter != "" && !matchesLabelFilter(issue, m.filter.labelFilter) {
			continue
		}
		scoped = append(scoped, issue)
	}

	rows := epicsOverviewRows(scoped, m.epicsStatusMode, time.Now())
	// Default sort: progress % ascending (least-complete first) - instant,
	// bd-native, and stable so equal-progress epics keep load order.
	sort.SliceStable(rows, func(i, j int) bool {
		return epicProgressFraction(rows[i]) < epicProgressFraction(rows[j])
	})
	m.epicsRows = rows
	if m.epicsCursor >= len(rows) {
		m.epicsCursor = 0
	}
	m.epicsViewText = m.renderEpicsOverview()
}

// renderEpicsOverview renders the all-epics overview: one row per epic with a
// progress bar, status counts, and an at-risk marker. The status filter is
// reinterpreted as which epics to list (m.epicsStatusMode). Repurposed from the
// dead sprint dashboard - the dates/burndown blocks are dropped (epics have no
// time window). bt-ryi5z.
func (m Model) renderEpicsOverview() string {
	t := m.theme

	boxWidth := min(80, m.width-4)
	if boxWidth < 40 {
		boxWidth = 40
	}
	innerWidth := boxWidth - 6
	if innerWidth < 30 {
		innerWidth = 30
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(t.Muted)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Epics (%s)", m.epicsStatusMode.label())))
	sb.WriteString("\n")

	if len(m.epicsRows) == 0 {
		sb.WriteString(mutedStyle.Render("0 epics"))
		sb.WriteString("\n\n")
		sb.WriteString(mutedStyle.Render("  No epics in scope."))
		sb.WriteString("\n")
	} else {
		// Scroll window centered on the cursor. Reserve rows for the header
		// (count line + blank), footer (blank + hint), and the two optional
		// "+N more" indicators.
		visible := m.height - 11
		if visible < 1 {
			visible = 1
		}
		start := 0
		if m.epicsCursor >= visible {
			start = m.epicsCursor - visible + 1
		}
		end := start + visible
		if end > len(m.epicsRows) {
			end = len(m.epicsRows)
			start = max(0, end-visible)
		}

		sb.WriteString(mutedStyle.Render(fmt.Sprintf("%d epics", len(m.epicsRows))))
		sb.WriteString("\n\n")

		if start > 0 {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
			sb.WriteString("\n")
		}
		for i := start; i < end; i++ {
			sb.WriteString(m.renderEpicRow(m.epicsRows[i], i == m.epicsCursor, innerWidth))
			sb.WriteString("\n")
		}
		if end < len(m.epicsRows) {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(m.epicsRows)-end)))
			sb.WriteString("\n")
		}
	}

	// Footer
	sb.WriteString("\n")
	sb.WriteString(mutedStyle.Italic(true).Render(
		"j/k nav · s: active/all/completed · ⏎ open · esc back"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(boxWidth).
		MaxHeight(m.height - 2)

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Top,
		boxStyle.Render(sb.String()),
	)
}

// renderEpicRow renders a single epic line: cursor, ID, progress bar, status
// counts, at-risk marker, and a title truncated to whatever width remains. The
// title budget is computed from the plain-text width of the fixed segments so
// styling (which adds zero display width) never causes box overflow.
func (m Model) renderEpicRow(r EpicRow, selected bool, innerWidth int) string {
	t := m.theme
	const barWidth = 8

	risk := ""
	if r.AtRisk > 0 {
		risk = fmt.Sprintf(" ⚠%d", r.AtRisk)
	}
	plainStat := fmt.Sprintf("%d/%d ✓%d ◐%d ⊘%d ○%d%s",
		r.Done, r.Total, r.Done, r.InProgress, r.Blocked, r.Open, risk)

	// Fixed (non-title) width: cursor(2) + id + " " + bar + " " + stat + "  ".
	fixed := 2 + lipgloss.Width(r.Epic.ID) + 1 + barWidth + 1 + lipgloss.Width(plainStat) + 2
	titleBudget := innerWidth - fixed
	if titleBudget < 0 {
		titleBudget = 0
	}
	title := truncateString(r.Epic.Title, titleBudget)

	// Styled segments (display widths match the plain layout above).
	cursor := "  "
	idStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	titleStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	if selected {
		cursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("▶ ")
		idStyle = idStyle.Bold(true)
		titleStyle = titleStyle.Bold(true)
	}

	filled := 0
	if r.Total > 0 {
		filled = int(float64(barWidth) * float64(r.Done) / float64(r.Total))
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := lipgloss.NewStyle().Foreground(t.Open).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(t.Muted).Render(strings.Repeat("░", barWidth-filled))

	var sb strings.Builder
	sb.WriteString(cursor)
	sb.WriteString(idStyle.Render(r.Epic.ID))
	sb.WriteString(" ")
	sb.WriteString(bar)
	sb.WriteString(" ")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Base.GetForeground()).Render(fmt.Sprintf("%d/%d", r.Done, r.Total)))
	sb.WriteString(" ")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Open).Render(fmt.Sprintf("✓%d", r.Done)))
	sb.WriteString(" ")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Feature).Render(fmt.Sprintf("◐%d", r.InProgress)))
	sb.WriteString(" ")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Blocked).Render(fmt.Sprintf("⊘%d", r.Blocked)))
	sb.WriteString(" ")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Muted).Render(fmt.Sprintf("○%d", r.Open)))
	if r.AtRisk > 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Feature).Render(fmt.Sprintf(" ⚠%d", r.AtRisk)))
	}
	if title != "" {
		sb.WriteString("  ")
		sb.WriteString(titleStyle.Render(title))
	}
	return sb.String()
}

// handleEpicsKeys handles keyboard input when the epics overview is focused.
// Dispatches via key.Matches against m.keys.Epics per ADR-004 Decision 1.
func (m Model) handleEpicsKeys(msg tea.KeyMsg) Model {
	k := m.keys.Epics
	switch {
	case key.Matches(msg, k.Down):
		if m.epicsCursor < len(m.epicsRows)-1 {
			m.epicsCursor++
			m.epicsViewText = m.renderEpicsOverview()
		}
	case key.Matches(msg, k.Up):
		if m.epicsCursor > 0 {
			m.epicsCursor--
			m.epicsViewText = m.renderEpicsOverview()
		}
	case key.Matches(msg, k.CycleStatus):
		m.epicsStatusMode = m.epicsStatusMode.next()
		m.epicsCursor = 0
		m.refreshEpicsForCurrentFilter()
	case key.Matches(msg, k.Open):
		// Tier-2 epic focus card lands in Phase 2 (bt-gfxhz.3 deliverable).
		m.setStatus("Epic focus card coming in Phase 2")
	case key.Matches(msg, k.Exit):
		m.mode = ViewList
		m.focused = focusList
	}
	return m
}
