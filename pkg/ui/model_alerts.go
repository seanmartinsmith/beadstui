package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	"charm.land/lipgloss/v2"
)

// ════════════════════════════════════════════════════════════════════════════
// ALERTS PANEL (bv-168)
// ════════════════════════════════════════════════════════════════════════════

// computeAlerts calculates drift alerts for the current issues using the
// already-computed graph stats/analyzer to avoid redundant work.
func computeAlerts(issues []model.Issue, stats *analysis.GraphStats, analyzer *analysis.Analyzer) ([]drift.Alert, int, int, int) {
	if len(issues) == 0 || stats == nil || analyzer == nil {
		return nil, 0, 0, 0
	}

	projectDir, _ := os.Getwd()
	driftConfig, err := drift.LoadConfig(projectDir)
	if err != nil {
		driftConfig = drift.DefaultConfig()
	}

	openCount, closedCount, blockedCount := 0, 0, 0
	for _, issue := range issues {
		switch {
		case isClosedLikeStatus(issue.Status):
			closedCount++
		case issue.Status == model.StatusBlocked:
			blockedCount++
		default:
			openCount++
		}
	}

	curStats := baseline.GraphStats{
		NodeCount:       stats.NodeCount,
		EdgeCount:       stats.EdgeCount,
		Density:         stats.Density,
		OpenCount:       openCount,
		ClosedCount:     closedCount,
		BlockedCount:    blockedCount,
		CycleCount:      len(stats.Cycles()),
		ActionableCount: len(analyzer.GetActionableIssues()),
	}

	bl := &baseline.Baseline{Stats: curStats}
	cur := &baseline.Baseline{Stats: curStats, Cycles: stats.Cycles()}

	calc := drift.NewCalculator(bl, cur, driftConfig)
	calc.SetIssues(issues)
	result := calc.Calculate()

	critical, warning, info := 0, 0, 0
	for _, a := range result.Alerts {
		switch a.Severity {
		case drift.SeverityCritical:
			critical++
		case drift.SeverityWarning:
			warning++
		case drift.SeverityInfo:
			info++
		}
	}

	return result.Alerts, critical, warning, info
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

// alertsVisibleLines returns the number of alert items that fit in one page,
// accounting for all panel chrome so the content never overflows.
// Chrome: summary(1) + blank(1) + above+filter(1) + detail-reserve(1)
//         + below+page(1) + blank(1) + footer(1) = 7 lines
// Panel borders consume 2 of the outer height.
func (m Model) alertsVisibleLines() int {
	innerHeight := m.alertsPanelHeight() - 2 // subtract top/bottom border
	lines := innerHeight - 7                 // subtract chrome
	if lines < 3 {
		lines = 3
	}
	return lines
}

// renderAlertsPanel renders the alerts overlay panel using RenderTitledPanel
// for visual consistency with the rest of the TUI.
func (m Model) renderAlertsPanel() string {
	t := m.theme

	panelWidth := min(80, m.width-4)

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

			// Detail for selected alert: issue title (1 line, truncated)
			if selected && a.IssueID != "" {
				if title, ok := issueTitles[a.IssueID]; ok && title != "" {
					titleStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
					detailMaxWidth := innerWidth - 8
					title = truncateRunesHelper(title, detailMaxWidth, "…")
					styled := titleStyle.Render("    " + title)
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

	// Pad each content line for inner padding
	paddedContent := padContentLines(sb.String(), 1)

	// Use blocked/red color to match alert severity
	panel := RenderTitledPanel(paddedContent, PanelOpts{
		Title:       "Alerts!",
		Width:       panelWidth,
		Height:      m.alertsPanelHeight(),
		BorderColor: t.Blocked,
		TitleColor:  t.Blocked,
	})

	return panel
}

// alertTypeLabel returns a short human-readable label for an alert type.
func alertTypeLabel(t drift.AlertType) string {
	switch t {
	case drift.AlertNewCycle:
		return "cycle"
	case drift.AlertPageRankChange:
		return "centrality"
	case drift.AlertDensityGrowth:
		return "density"
	case drift.AlertNodeCountChange:
		return "nodes"
	case drift.AlertEdgeCountChange:
		return "edges"
	case drift.AlertBlockedIncrease:
		return "blocked"
	case drift.AlertActionableChange:
		return "actionable"
	case drift.AlertStaleIssue:
		return "stale"
	case drift.AlertVelocityDrop:
		return "velocity"
	case drift.AlertBlockingCascade:
		return "cascade"
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
