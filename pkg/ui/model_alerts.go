package ui

import (
	"fmt"
	"os"
	"strings"

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

// visibleAlerts returns alerts filtered by active repo filter and dismissed state.
// In workspace mode with a project filter, only alerts for selected projects are shown.
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
		out = append(out, a)
	}
	return out
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
// Chrome: summary(1) + above-indicator(1) + detail-reserve(2) + below-indicator(1)
//         + page-indicator(1) + blank(1) + footer(1) = 8 lines
// Panel borders consume 2 of the outer height.
func (m Model) alertsVisibleLines() int {
	innerHeight := m.alertsPanelHeight() - 2 // subtract top/bottom border
	lines := innerHeight - 8                 // subtract chrome
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
		summaryStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		summary := fmt.Sprintf("%d total", len(visibleAlerts))
		if critical > 0 {
			summary += fmt.Sprintf(" • %d critical", critical)
		}
		if warning > 0 {
			summary += fmt.Sprintf(" • %d warning", warning)
		}
		if info > 0 {
			summary += fmt.Sprintf(" • %d info", info)
		}
		sb.WriteString(" ")
		sb.WriteString(summaryStyle.Render(summary))
		sb.WriteString("\n")

		// Page-aligned visible window (cursor position determines page)
		pageSize := m.alertsVisibleLines()
		start := (m.alertsCursor / pageSize) * pageSize
		end := start + pageSize
		if end > len(visibleAlerts) {
			end = len(visibleAlerts)
		}

		// Scroll indicator: above
		if start > 0 {
			hint := lipgloss.NewStyle().Foreground(t.Muted).Render(
				fmt.Sprintf(" ▴ %d more above", start))
			sb.WriteString(hint)
			sb.WriteString("\n")
		} else {
			sb.WriteString("\n")
		}

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

			// Cursor indicator
			cursor := "  "
			if selected {
				cursor = "▸ "
			}

			// Alert line (sanitize newlines to prevent panel expansion)
			msg := strings.ReplaceAll(a.Message, "\n", " ")
			line := fmt.Sprintf("%s%s %s", cursor, severityIcon, msg)
			if lipgloss.Width(line) > innerWidth {
				line = truncateRunesHelper(line, innerWidth, "…")
			}
			if selected {
				line = lipgloss.NewStyle().Bold(true).Render(line)
			}
			rendered := severityStyle.Render(line)

			// Center the alert line
			lineWidth := lipgloss.Width(rendered)
			pad := (innerWidth - lineWidth) / 2
			if pad < 0 {
				pad = 0
			}
			sb.WriteString(strings.Repeat(" ", pad))
			sb.WriteString(rendered)
			sb.WriteString("\n")

			// Detail for selected alert: issue title (exactly 2 lines, for fixed height)
			if selected && a.IssueID != "" {
				if title, ok := issueTitles[a.IssueID]; ok && title != "" {
					titleStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
					detailMaxWidth := innerWidth - 8
					wrappedStr := wrapText(title, detailMaxWidth)
					detailLines := strings.Split(wrappedStr, "\n")
					// Cap at 2 lines, truncate if longer
					if len(detailLines) > 2 {
						detailLines[1] = truncateRunesHelper(detailLines[1], detailMaxWidth, "…")
						detailLines = detailLines[:2]
					}
					for _, wline := range detailLines {
						styled := titleStyle.Render("    " + wline)
						styledWidth := lipgloss.Width(styled)
						dPad := (innerWidth - styledWidth) / 2
						if dPad < 0 {
							dPad = 0
						}
						sb.WriteString(strings.Repeat(" ", dPad))
						sb.WriteString(styled)
						sb.WriteString("\n")
					}
					// Pad to exactly 2 detail lines
					for i := len(detailLines); i < 2; i++ {
						sb.WriteString("\n")
					}
				} else {
					// No title found - still emit 2 blank lines
					sb.WriteString("\n\n")
				}
			} else if selected {
				// Selected but no issue ID - emit 2 blank lines
				sb.WriteString("\n\n")
			}
		}

		// Pad remaining lines to fixed page height (visual stability)
		for i := end - start; i < pageSize; i++ {
			sb.WriteString("\n")
		}

		// Scroll indicator: below
		remaining := len(visibleAlerts) - end
		if remaining > 0 {
			hint := lipgloss.NewStyle().Foreground(t.Muted).Render(
				fmt.Sprintf(" ▾ %d more below", remaining))
			sb.WriteString(hint)
		}

		// Page indicator
		if len(visibleAlerts) > pageSize {
			page := m.alertsCursor/pageSize + 1
			totalPages := (len(visibleAlerts) + pageSize - 1) / pageSize
			pageStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
			sb.WriteString("\n")
			sb.WriteString(pageStyle.Render(
				fmt.Sprintf(" %d/%d (%d alerts)", page, totalPages, len(visibleAlerts))))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(t.Muted).Italic(true).Render(
		" enter: open • c: clear • C: clear all • esc: close"))
	if len(visibleAlerts) > m.alertsVisibleLines() {
		sb.WriteString(lipgloss.NewStyle().Foreground(t.Muted).Italic(true).Render(
			" • ←/→: page"))
	}

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

// padContentLines adds horizontal padding to each line of content.
func padContentLines(content string, pad int) string {
	padding := strings.Repeat(" ", pad)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = padding + line
	}
	return strings.Join(lines, "\n")
}

