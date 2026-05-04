package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"

	"charm.land/lipgloss/v2"
)

// ════════════════════════════════════════════════════════════════════════════
// ALERTS PANEL (bv-168)
// ════════════════════════════════════════════════════════════════════════════

// computeAlerts calculates per-project scoped drift alerts (bt-46p6.8).
//
// In workspace mode, alerts are partitioned by SourceRepo and tagged with
// SourceProject. In single-project mode, all alerts carry the uniform project
// key (SourceRepo / "local"). The pre-computed stats/analyzer passed by
// legacy callers is no longer used — ProjectAlerts re-analyzes per project
// because cross-project aggregate metrics are incoherent (see bt-7l5m).
func computeAlerts(issues []model.Issue, workspaceMode bool) ([]drift.Alert, int, int, int) {
	if len(issues) == 0 {
		return nil, 0, 0, 0
	}

	projectDir, _ := os.Getwd()
	driftConfig, err := drift.LoadConfig(projectDir)
	if err != nil {
		driftConfig = drift.DefaultConfig()
	}

	// Baseline comparisons are deliberately not wired in the TUI path — the
	// TUI has never persisted a baseline file. Commit 2 of bt-46p6.8 adds
	// per-project baseline sections; when that lands, the TUI can opt in by
	// supplying a loader here.
	alerts := drift.ProjectAlerts(issues, workspaceMode, "", driftConfig, nil)

	critical, warning, info := 0, 0, 0
	for _, a := range alerts {
		switch a.Severity {
		case drift.SeverityCritical:
			critical++
		case drift.SeverityWarning:
			warning++
		case drift.SeverityInfo:
			info++
		}
	}

	return alerts, critical, warning, info
}

// alertKey generates a unique key for an alert (for dismissal tracking)
func alertKey(a drift.Alert) string {
	return fmt.Sprintf("%s:%s:%s", a.Type, a.Severity, a.IssueID)
}

// visibleAlerts returns alerts filtered by dismissed state, repo filter,
// and stackable alert filters (severity, type, project, sort).
func (m Model) visibleAlerts() []drift.Alert {
	var out []drift.Alert
	for _, a := range m.alerts {
		if m.dismissedAlerts[alertKey(a)] {
			continue
		}
		// Filter by active repo when in workspace mode with a project filter
		if m.workspaceMode && m.activeRepos != nil && a.IssueID != "" {
			issue, ok := m.data.issueMap[a.IssueID]
			if ok {
				repoKey := IssueRepoKey(*issue)
				if repoKey != "" && !m.activeRepos[repoKey] {
					continue
				}
			}
		}
		// Stackable filters (bt-46p6.5)
		if m.alertFilterSeverity != "" && string(a.Severity) != m.alertFilterSeverity {
			continue
		}
		if m.alertFilterType != "" && string(a.Type) != m.alertFilterType {
			continue
		}
		if m.alertFilterProject != "" && a.IssueID != "" {
			issue, ok := m.data.issueMap[a.IssueID]
			if !ok || IssueRepoKey(*issue) != m.alertFilterProject {
				continue
			}
		}
		out = append(out, a)
	}

	// Sort (bt-46p6.5) - use issue UpdatedAt for meaningful ordering
	// (DetectedAt is always time.Now() so it's useless for sorting)
	if m.alertSortOrder != 0 {
		// Pre-build time lookup to avoid repeated map hits in comparator
		times := make([]time.Time, len(out))
		for i, a := range out {
			times[i] = m.alertIssueTime(a)
		}
		sort.Slice(out, func(i, j int) bool {
			if m.alertSortOrder == 1 { // oldest first
				return times[i].Before(times[j])
			}
			return times[i].After(times[j]) // newest first
		})
	}
	return out
}

// visibleNotifications returns ring-buffer events filtered by dismissed state
// and active-repo filter, newest-first. v1 hides dismissed; v2 (bt-46p6.13)
// exposes them when notifShowDismissed is set. In workspace mode with an
// active repo filter, events whose Repo isn't in activeRepos are hidden —
// mirrors the alerts tab's project-scoping so 'single project' /
// 'multi-project select' / 'global' views produce the right notification set.
func (m Model) visibleNotifications() []events.Event {
	snap := m.events.Snapshot()
	out := make([]events.Event, 0, len(snap))
	// Snapshot is oldest-first; reverse to newest-first.
	for i := len(snap) - 1; i >= 0; i-- {
		if snap[i].Dismissed && !m.notifShowDismissed {
			continue
		}
		// Respect workspace-mode project filter. activeRepos == nil means
		// "all projects" (global); a non-nil map means "only these repos".
		if m.workspaceMode && m.activeRepos != nil && snap[i].Repo != "" {
			if !m.activeRepos[snap[i].Repo] {
				continue
			}
		}
		out = append(out, snap[i])
	}
	return out
}

// alertProjectKey returns the project prefix for an alert's issue.
func (m Model) alertProjectKey(a drift.Alert) string {
	if a.IssueID == "" {
		return ""
	}
	if issue, ok := m.data.issueMap[a.IssueID]; ok {
		return IssueRepoKey(*issue)
	}
	return ""
}

// alertActiveTypes returns the set of alert types present in the current (unfiltered) alerts.
func (m Model) alertActiveTypes() []string {
	seen := make(map[string]bool)
	var types []string
	for _, a := range m.alerts {
		if m.dismissedAlerts[alertKey(a)] {
			continue
		}
		t := string(a.Type)
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	sort.Strings(types)
	return types
}

// alertActiveProjects returns project prefixes present in the current alerts.
func (m Model) alertActiveProjects() []string {
	seen := make(map[string]bool)
	var projects []string
	for _, a := range m.alerts {
		if m.dismissedAlerts[alertKey(a)] {
			continue
		}
		p := m.alertProjectKey(a)
		if p != "" && !seen[p] {
			seen[p] = true
			projects = append(projects, p)
		}
	}
	sort.Strings(projects)
	return projects
}

// alertIssueTime returns the issue's UpdatedAt for sort ordering.
// Falls back to DetectedAt for alerts without an issue reference.
func (m Model) alertIssueTime(a drift.Alert) time.Time {
	if a.IssueID != "" {
		if issue, ok := m.data.issueMap[a.IssueID]; ok {
			if !issue.UpdatedAt.IsZero() {
				return issue.UpdatedAt
			}
			return issue.CreatedAt
		}
	}
	return a.DetectedAt
}

// resetAlertFilters clears all alert filter state.
func (m *Model) resetAlertFilters() {
	m.alertFilterSeverity = ""
	m.alertFilterType = ""
	m.alertFilterProject = ""
	m.alertSortOrder = 0
}

// alertsPanelHeight returns the fixed outer height of the alerts panel
// (including borders). Capped at ~70% of terminal height.
func (m Model) alertsPanelHeight() int {
	h := m.height * 7 / 10
	if h < 12 {
		h = 12
	}
	return h
}

// alertsPanelWidth returns the outer width of the shared alerts/notifications
// modal. The modal spans the terminal (minus a small gutter) so the underlying
// detail pane is fully occluded — a cap at 80 cells previously left bg content
// visible flanking the modal at typical split-view widths (bt-l5xu). The
// 4-cell gutter matches the centering math in OverlayCenter and lets the
// background show as a thin frame around the modal.
func (m Model) alertsPanelWidth() int {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	return w
}

// alertsVisibleLines returns the number of alert items that fit in one page,
// accounting for all panel chrome so the content never overflows.
// Chrome: summary(1) + blank(1) + above+filter(1) + detail-reserve(1)
//         + below+page(1) + blank(1) + footer(1) = 7 lines
// detail-reserve is 1 because the selected row expands to show the issue
// title or first Details entry (bt-46p6.17's inline alert-type definition
// was removed by bt-xyjd; explanations now live behind the ? help modal,
// bt-i20z, instead of cluttering every alert).
// Panel borders consume 2 of the outer height.
func (m Model) alertsVisibleLines() int {
	innerHeight := m.alertsPanelHeight() - 2 // subtract top/bottom border
	lines := innerHeight - 7                 // subtract chrome
	if lines < 3 {
		lines = 3
	}
	return lines
}

// renderAlertsTab renders the alerts-tab inner content (no panel frame, no
// outer padding). The shared modal frame lives in renderAlertsPanel
// (bt-46p6.10); this function produces only the body visible on the alerts
// tab.
func (m Model) renderAlertsTab() string {
	t := m.theme

	panelWidth := m.alertsPanelWidth()

	visibleAlerts := m.visibleAlerts()

	// Inner content width (panel width minus borders and padding)
	innerWidth := panelWidth - 4 // 2 border + 2 padding

	// Build issue title lookup for detail line
	issueTitles := make(map[string]string)
	for _, item := range m.list.Items() {
		if it, ok := item.(IssueItem); ok {
			issueTitles[it.Issue.ID] = it.Issue.Title
		}
	}

	var sb strings.Builder

	if len(visibleAlerts) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorSuccess).Render(" No active alerts"))
		sb.WriteString("\n")
	} else {
		// Summary line with counts from the visible (filtered) set
		critical, warning, info := 0, 0, 0
		for _, a := range visibleAlerts {
			switch a.Severity {
			case drift.SeverityCritical:
				critical++
			case drift.SeverityWarning:
				warning++
			case drift.SeverityInfo:
				info++
			}
		}
		totalStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		critStyle := lipgloss.NewStyle().Foreground(t.Blocked).Bold(true)
		warnStyle := lipgloss.NewStyle().Foreground(t.Feature)
		infoStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		sepStyle := lipgloss.NewStyle().Foreground(t.Muted)
		sep := sepStyle.Render(" • ")

		sb.WriteString(" ")
		sb.WriteString(totalStyle.Render(fmt.Sprintf("%d total", len(visibleAlerts))))
		if critical > 0 {
			sb.WriteString(sep)
			sb.WriteString(critStyle.Render(fmt.Sprintf("%d critical", critical)))
		}
		if warning > 0 {
			sb.WriteString(sep)
			sb.WriteString(warnStyle.Render(fmt.Sprintf("%d warning", warning)))
		}
		if info > 0 {
			sb.WriteString(sep)
			sb.WriteString(infoStyle.Render(fmt.Sprintf("%d info", info)))
		}
		sb.WriteString("\n\n")

		// Build filter label for display on the "above" indicator line
		var filterParts []string
		if m.alertFilterSeverity != "" {
			filterParts = append(filterParts, m.alertFilterSeverity)
		}
		if m.alertFilterType != "" {
			filterParts = append(filterParts, alertTypeLabel(drift.AlertType(m.alertFilterType)))
		}
		if m.alertFilterProject != "" {
			filterParts = append(filterParts, m.alertFilterProject)
		}
		sortNames := []string{"", "oldest", "newest"}
		if m.alertSortOrder > 0 {
			filterParts = append(filterParts, sortNames[m.alertSortOrder])
		}
		filterLabel := ""
		if len(filterParts) > 0 {
			filterLabel = "filter: " + strings.Join(filterParts, " • ")
		}

		// Page-aligned visible window (cursor position determines page)
		pageSize := m.alertsVisibleLines()
		start := (m.alertsCursor / pageSize) * pageSize
		end := start + pageSize
		if end > len(visibleAlerts) {
			end = len(visibleAlerts)
		}

		// Above indicator line: "▴ N more above" left, filter label right
		aboveHint := ""
		if start > 0 {
			aboveHint = fmt.Sprintf(" ▴ %d more above", start)
		}
		if aboveHint != "" || filterLabel != "" {
			leftPart := lipgloss.NewStyle().Foreground(t.Muted).Render(aboveHint)
			rightPart := lipgloss.NewStyle().Foreground(t.Feature).Italic(true).Render(filterLabel)
			leftW := lipgloss.Width(leftPart)
			rightW := lipgloss.Width(rightPart)
			gap := innerWidth - leftW - rightW
			if gap < 1 {
				gap = 1
			}
			sb.WriteString(leftPart + strings.Repeat(" ", gap) + rightPart)
		}
		sb.WriteString("\n")

		// Render visible page of alerts (centered)
		for i := start; i < end; i++ {
			a := visibleAlerts[i]
			selected := i == m.alertsCursor

			// Severity indicator
			var severityStyle lipgloss.Style
			var severityIcon string
			switch a.Severity {
			case drift.SeverityCritical:
				severityStyle = lipgloss.NewStyle().Foreground(t.Blocked).Bold(true)
				severityIcon = "▲"
			case drift.SeverityWarning:
				severityStyle = lipgloss.NewStyle().Foreground(t.Feature)
				severityIcon = "△"
			default:
				severityStyle = lipgloss.NewStyle().Foreground(t.Secondary)
				severityIcon = "○"
			}

			// Cursor indicator (neutral color so it stands out from severity)
			cursor := "  "
			if selected {
				cursor = lipgloss.NewStyle().Foreground(t.Muted).Bold(true).Render("▸ ")
			}

			// Alert line (sanitize newlines to prevent panel expansion)
			msg := strings.ReplaceAll(a.Message, "\n", " ")
			typeTag := fmt.Sprintf("[%s]", alertTypeLabel(a.Type))
			line := fmt.Sprintf("%s %s %s", severityIcon, typeTag, msg)
			if lipgloss.Width(cursor)+lipgloss.Width(line) > innerWidth {
				line = truncateRunesHelper(line, innerWidth-lipgloss.Width(cursor), "…")
			}
			if selected {
				severityStyle = severityStyle.Bold(true)
			}
			rendered := cursor + severityStyle.Render(line)

			// Center the alert line
			lineWidth := lipgloss.Width(rendered)
			pad := (innerWidth - lineWidth) / 2
			if pad < 0 {
				pad = 0
			}
			sb.WriteString(strings.Repeat(" ", pad))
			sb.WriteString(rendered)
			sb.WriteString("\n")

			// Detail for selected alert: issue title for single-issue alerts,
			// or first Details entry for graph-scope alerts. The inline
			// alert-type definition was removed by bt-xyjd because variable
			// row height (definition + title = 2 cursor lines vs 1 elsewhere)
			// broke mouse hit-test, and most users found the always-on
			// description more clutter than help. Type explanations now live
			// behind the ? help modal (bt-i20z).
			if selected {
				detailStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
				detailMaxWidth := innerWidth - 8

				if a.IssueID != "" {
					if title, ok := issueTitles[a.IssueID]; ok && title != "" {
						title = truncateRunesHelper(title, detailMaxWidth, "…")
						styled := detailStyle.Render("    " + title)
						styledWidth := lipgloss.Width(styled)
						dPad := (innerWidth - styledWidth) / 2
						if dPad < 0 {
							dPad = 0
						}
						sb.WriteString(strings.Repeat(" ", dPad))
						sb.WriteString(styled)
						sb.WriteString("\n")
					}
				} else if len(a.Details) > 0 {
					// Graph-scope alerts (dependency_loop, centrality_change,
					// coupling_growth, etc.) carry their payload in Details
					// rather than IssueID — the message is just a count
					// ("2 new cycle(s) detected"). Surface the first entry
					// with a "+N more" suffix so users see what's actually
					// looping or shifting (bt-7ye5).
					first := a.Details[0]
					if len(a.Details) > 1 {
						first = fmt.Sprintf("%s  (+%d more)", first, len(a.Details)-1)
					}
					first = truncateRunesHelper(first, detailMaxWidth, "…")
					styled := detailStyle.Render("    " + first)
					styledWidth := lipgloss.Width(styled)
					dPad := (innerWidth - styledWidth) / 2
					if dPad < 0 {
						dPad = 0
					}
					sb.WriteString(strings.Repeat(" ", dPad))
					sb.WriteString(styled)
					sb.WriteString("\n")
				}
			}
		}

		// Pad remaining lines to fixed page height (visual stability)
		for i := end - start; i < pageSize; i++ {
			sb.WriteString("\n")
		}

		// Below indicator line: "▾ N more below" left, page indicator right
		belowHint := ""
		remaining := len(visibleAlerts) - end
		if remaining > 0 {
			belowHint = fmt.Sprintf(" ▾ %d more below", remaining)
		}
		pageLabel := ""
		if len(visibleAlerts) > pageSize {
			page := m.alertsCursor/pageSize + 1
			totalPages := (len(visibleAlerts) + pageSize - 1) / pageSize
			pageLabel = fmt.Sprintf("%d/%d (%d alerts)", page, totalPages, len(visibleAlerts))
		}
		if belowHint != "" || pageLabel != "" {
			leftPart := lipgloss.NewStyle().Foreground(t.Muted).Render(belowHint)
			rightPart := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true).Render(pageLabel)
			leftW := lipgloss.Width(leftPart)
			rightW := lipgloss.Width(rightPart)
			gap := innerWidth - leftW - rightW
			if gap < 1 {
				gap = 1
			}
			sb.WriteString(leftPart + strings.Repeat(" ", gap) + rightPart)
		}
	}

	// Footer: centered help text (with breathing room above)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
	helpText := helpStyle.Render("filter: s/t/p/o (\u21e7:prev) reset: r • open: enter clear: c (\u21e7:all)")
	helpW := lipgloss.Width(helpText)
	helpPad := (innerWidth - helpW) / 2
	if helpPad < 0 {
		helpPad = 0
	}
	sb.WriteString("\n\n")
	sb.WriteString(strings.Repeat(" ", helpPad) + helpText)

	// Panel frame applied by renderAlertsPanel (bt-46p6.10).
	return sb.String()
}

// formatNotificationRow renders a single ring-buffer event as a one-line
// notification. Format: "15:04 closed bt-46p6.1 • Fix: modal expands…"
// Single space between time/kind/id; " • " separates id from title.
// Columns are unaligned (intentional — tighter spacing over grid alignment).
// Title is sanitized (newlines → spaces) and truncated at runtime width.
func formatNotificationRow(e events.Event, width int) string {
	timeStr := e.At.Format("15:04")
	kindStr := e.Kind.String()
	idStr := e.BeadID
	// Dismissed prefix only renders when the dismissed-filter is on (v2,
	// bt-46p6.13); v1 callers never pass dismissed events so the prefix is
	// effectively gated by visibleNotifications.
	prefix := ""
	if e.Dismissed {
		prefix = "✕ "
	}
	title := strings.ReplaceAll(e.Title, "\n", " ")
	// System events (bt-9u39) carry no BeadID — render as "15:04 system • Title"
	// without the empty id slot to avoid a double-space gap.
	if e.Kind == events.EventSystem || idStr == "" {
		consumed := len(prefix) + len(timeStr) + 1 + len(kindStr) + 3
		titleWidth := width - consumed
		if titleWidth < 10 {
			titleWidth = 10
		}
		return prefix + timeStr + " " + kindStr + " • " + truncate(title, titleWidth)
	}
	// timeStr(5) + " " + kindStr + " " + idStr + " • " (3) + optional prefix
	consumed := len(prefix) + len(timeStr) + 1 + len(kindStr) + 1 + len(idStr) + 3
	titleWidth := width - consumed
	if titleWidth < 10 {
		titleWidth = 10
	}
	return prefix + timeStr + " " + kindStr + " " + idStr + " • " + truncate(title, titleWidth)
}

// renderNotificationsTab builds the notifications tab body. Reads from
// events.RingBuffer, hides dismissed events, and renders newest-first with
// pagination matching the alerts tab. Returns inner content only; the
// shared modal frame is applied in renderAlertsPanel.
func (m Model) renderNotificationsTab() string {
	t := m.theme
	panelWidth := m.alertsPanelWidth()
	innerWidth := panelWidth - 4

	active := m.visibleNotifications()

	var sb strings.Builder

	if len(active) == 0 {
		// Center "No notifications" both vertically and horizontally in the
		// panel body. inner = panel inner area (Height - top/bottom borders).
		// RenderTitledPanel pads content to inner rows, so we write vPad
		// blanks + centered text + bottom blanks to land dead center.
		msg := lipgloss.NewStyle().Foreground(ColorSuccess).Render("No notifications")
		msgW := lipgloss.Width(msg)
		hPad := (innerWidth - msgW) / 2
		if hPad < 0 {
			hPad = 0
		}
		inner := m.alertsPanelHeight() - 2
		if inner < 1 {
			inner = 1
		}
		vPadTop := (inner - 1) / 2
		vPadBottom := inner - 1 - vPadTop
		for i := 0; i < vPadTop; i++ {
			sb.WriteString("\n")
		}
		sb.WriteString(strings.Repeat(" ", hPad) + msg)
		for i := 0; i < vPadBottom; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	// Summary line: per-kind breakdown (mirrors alerts' "N total · K critical · …").
	// Total is already rendered in the border's RightLabel, so omit it here.
	var created, edited, closed, commented, bulk, system int
	for _, e := range active {
		switch e.Kind {
		case events.EventCreated:
			created++
		case events.EventEdited:
			edited++
		case events.EventClosed:
			closed++
		case events.EventCommented:
			commented++
		case events.EventBulk:
			bulk++
		case events.EventSystem:
			system++
		}
	}
	kindStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	sepStyle := lipgloss.NewStyle().Foreground(t.Muted)
	sep := sepStyle.Render(" • ")
	sb.WriteString(" ")
	first := true
	writeKind := func(n int, label string) {
		if n == 0 {
			return
		}
		if !first {
			sb.WriteString(sep)
		}
		sb.WriteString(kindStyle.Render(fmt.Sprintf("%d %s", n, label)))
		first = false
	}
	writeKind(created, "created")
	writeKind(edited, "edited")
	writeKind(closed, "closed")
	writeKind(commented, "commented")
	writeKind(bulk, "bulk")
	writeKind(system, "system")
	sb.WriteString("\n\n")

	// Leave one row of the page for the cursor-expand line (hover-expand
	// shows Event.Summary beneath the selected row). Matches the tradeoff
	// used by the alerts tab's selected-detail line.
	pageSize := m.alertsVisibleLines() - 1
	if pageSize < 2 {
		pageSize = 2
	}
	start := (m.notificationsCursor / pageSize) * pageSize
	end := start + pageSize
	if end > len(active) {
		end = len(active)
	}

	mutedStyle := lipgloss.NewStyle().Foreground(t.Muted)

	// Above-indicator line (matches alerts' above-hint row).
	if start > 0 {
		sb.WriteString(mutedStyle.Render(fmt.Sprintf(" ▴ %d more above", start)))
	}
	sb.WriteString("\n")

	cursorStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	summaryStyle := mutedStyle.Italic(true)

	// Usable width for the row content after our "▸ " / "   " prefix (3)
	// and a right-side margin (2) to keep text from kissing the border.
	rowWidth := innerWidth - 5
	if rowWidth < 20 {
		rowWidth = 20
	}

	rowsWritten := 0
	for i := start; i < end; i++ {
		row := formatNotificationRow(active[i], rowWidth)
		if i == m.notificationsCursor {
			sb.WriteString(" " + cursorStyle.Render("▸ "+row))
			sb.WriteString("\n")
			rowsWritten++
			// Sanitize Summary: strip newlines so the hover-expand stays on
			// a single line (commit/comment summaries may include line breaks).
			s := strings.ReplaceAll(active[i].Summary, "\n", " ")
			s = strings.TrimSpace(s)
			if s != "" {
				sb.WriteString("    " + summaryStyle.Render(truncate(s, rowWidth-2)))
				sb.WriteString("\n")
				rowsWritten++
			}
		} else {
			sb.WriteString("   " + rowStyle.Render(row))
			sb.WriteString("\n")
			rowsWritten++
		}
	}

	// Pad item rows to pageSize for visual stability — the page indicator
	// below lands at the same row regardless of how many items are on this
	// page (matches alerts tab's padding at renderAlertsTab).
	for i := rowsWritten; i < pageSize; i++ {
		sb.WriteString("\n")
	}

	// Below-indicator + page counter on a single line: " ▾ N more below" left,
	// "N/M" right. Mirrors the alerts tab's above/below pattern.
	belowHint := ""
	if end < len(active) {
		belowHint = fmt.Sprintf(" ▾ %d more below", len(active)-end)
	}
	pageLabel := fmt.Sprintf("%d/%d", m.notificationsCursor+1, len(active))
	leftPart := mutedStyle.Render(belowHint)
	rightPart := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true).Render(pageLabel)
	leftW := lipgloss.Width(leftPart)
	rightW := lipgloss.Width(rightPart)
	gap := innerWidth - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	sb.WriteString(leftPart + strings.Repeat(" ", gap) + rightPart)

	// Footer: centered help text (matches alerts-tab layout at the bottom).
	// `d` toggles dismissed-event visibility (bt-46p6.13); the label flips
	// to reflect the next action so users know what the toggle does.
	hintStyle := mutedStyle.Italic(true)
	dismissToggleLabel := "d: show dismissed"
	if m.notifShowDismissed {
		dismissToggleLabel = "d: hide dismissed"
	}
	hintText := hintStyle.Render(fmt.Sprintf("j/k: nav  enter: open  c: dismiss  C: dismiss all  %s  esc: close", dismissToggleLabel))
	hintW := lipgloss.Width(hintText)
	hintPad := (innerWidth - hintW) / 2
	if hintPad < 0 {
		hintPad = 0
	}
	sb.WriteString("\n\n")
	sb.WriteString(strings.Repeat(" ", hintPad) + hintText)

	return sb.String()
}

// renderAlertsPanel is the public entry point for the shared alerts /
// notifications modal (bt-46p6.10). Title + count live in the panel border
// (no in-body tab strip); dispatches body to the active tab.
func (m Model) renderAlertsPanel() string {
	t := m.theme
	panelWidth := m.alertsPanelWidth()

	var title, rightLabel string
	titleColor := t.Blocked
	if m.activeTab == TabAlerts {
		title = "Alerts!"
		rightLabel = fmt.Sprintf("(%d)", len(m.visibleAlerts()))
	} else {
		title = "Notifications"
		titleColor = t.Primary
		// Use visibleNotifications so the count honors the active-repo filter,
		// matching alerts' len(m.visibleAlerts()) behavior (bt-46p6.10).
		rightLabel = fmt.Sprintf("(%d)", len(m.visibleNotifications()))
	}

	var body string
	if m.activeTab == TabAlerts {
		body = m.renderAlertsTab()
	} else {
		body = m.renderNotificationsTab()
	}

	return RenderTitledPanel(padContentLines(body, 1), PanelOpts{
		Title:       title,
		RightLabel:  rightLabel,
		Width:       panelWidth,
		Height:      m.alertsPanelHeight(),
		BorderColor: titleColor,
		TitleColor:  titleColor,
	})
}

// alertTypeLabel returns a short human-readable label for an alert type.
func alertTypeLabel(t drift.AlertType) string {
	switch t {
	case drift.AlertDependencyLoop:
		return "dep loop"
	case drift.AlertCentralityChange:
		return "centrality"
	case drift.AlertCouplingGrowth:
		return "coupling"
	case drift.AlertIssueCountChange:
		return "issues"
	case drift.AlertDependencyChange:
		return "deps"
	case drift.AlertBlockedIncrease:
		return "blocked"
	case drift.AlertActionableChange:
		return "actionable"
	case drift.AlertStale:
		return "stale"
	case drift.AlertVelocityDrop:
		return "velocity"
	case drift.AlertHighLeverage:
		return "leverage"
	case drift.AlertHighImpactUnblock:
		return "unblock"
	case drift.AlertAbandonedClaim:
		return "abandoned"
	case drift.AlertPotentialDuplicate:
		return "duplicate"
	default:
		return string(t)
	}
}

// padContentLines adds horizontal padding to each line of content.
func padContentLines(content string, pad int) string {
	padding := strings.Repeat(" ", pad)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = padding + line
	}
	return strings.Join(lines, "\n")
}
