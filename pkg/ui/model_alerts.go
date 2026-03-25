package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	"github.com/charmbracelet/lipgloss"
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

// alertsVisibleLines returns the number of alert items that fit in the
// viewport, accounting for panel chrome (borders, summary, footer, padding).
func (m Model) alertsVisibleLines() int {
	// Cap panel at ~70% of terminal height
	panelMax := m.height * 7 / 10
	// Subtract chrome: top border(1) + summary(1) + blank(1) + blank(1) + footer(1) + bottom border(1) = 6
	lines := panelMax - 6
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

	// Filter out dismissed alerts
	var visibleAlerts []drift.Alert
	for _, a := range m.alerts {
		if !m.dismissedAlerts[alertKey(a)] {
			visibleAlerts = append(visibleAlerts, a)
		}
	}

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
		sb.WriteString(t.Renderer.NewStyle().Foreground(ColorSuccess).Render(" No active alerts"))
		sb.WriteString("\n")
	} else {
		// Summary line
		summaryStyle := t.Renderer.NewStyle().Foreground(t.Secondary)
		summary := fmt.Sprintf("%d total", len(visibleAlerts))
		if m.alertsCritical > 0 {
			summary += fmt.Sprintf(" • %d critical", m.alertsCritical)
		}
		if m.alertsWarning > 0 {
			summary += fmt.Sprintf(" • %d warning", m.alertsWarning)
		}
		if m.alertsInfo > 0 {
			summary += fmt.Sprintf(" • %d info", m.alertsInfo)
		}
		sb.WriteString(" ")
		sb.WriteString(summaryStyle.Render(summary))
		sb.WriteString("\n")

		// Scrollable alert list
		visLines := m.alertsVisibleLines()
		scrollOffset := m.alertsScrollOffset

		// Clamp scroll offset
		if scrollOffset > len(visibleAlerts)-visLines {
			scrollOffset = len(visibleAlerts) - visLines
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}

		endIdx := scrollOffset + visLines
		if endIdx > len(visibleAlerts) {
			endIdx = len(visibleAlerts)
		}

		// Scroll indicator: above
		if scrollOffset > 0 {
			hint := t.Renderer.NewStyle().Foreground(t.Muted).Render(
				fmt.Sprintf(" ▴ %d more above", scrollOffset))
			sb.WriteString(hint)
			sb.WriteString("\n")
		} else {
			sb.WriteString("\n")
		}

		// Render visible window of alerts (centered)
		for i := scrollOffset; i < endIdx; i++ {
			a := visibleAlerts[i]
			selected := i == m.alertsCursor

			// Severity indicator
			var severityStyle lipgloss.Style
			var severityIcon string
			switch a.Severity {
			case drift.SeverityCritical:
				severityStyle = t.Renderer.NewStyle().Foreground(t.Blocked).Bold(true)
				severityIcon = "▲"
			case drift.SeverityWarning:
				severityStyle = t.Renderer.NewStyle().Foreground(t.Feature)
				severityIcon = "△"
			default:
				severityStyle = t.Renderer.NewStyle().Foreground(t.Secondary)
				severityIcon = "○"
			}

			// Cursor indicator
			cursor := "  "
			if selected {
				cursor = "▸ "
			}

			// Alert line
			line := fmt.Sprintf("%s%s %s", cursor, severityIcon, a.Message)
			if lipgloss.Width(line) > innerWidth {
				line = truncateRunesHelper(line, innerWidth, "…")
			}
			if selected {
				line = t.Renderer.NewStyle().Bold(true).Render(line)
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

			// Detail for selected alert: issue title (wraps to ~2 lines max)
			if selected && a.IssueID != "" {
				if title, ok := issueTitles[a.IssueID]; ok && title != "" {
					titleStyle := t.Renderer.NewStyle().Foreground(t.Muted).Italic(true)
					detailMaxWidth := innerWidth - 8 // indent on each side
					wrappedStr := wrapText(title, detailMaxWidth)
					for _, wline := range strings.Split(wrappedStr, "\n") {
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
				}
			}
		}

		// Scroll indicator: below
		remaining := len(visibleAlerts) - endIdx
		if remaining > 0 {
			hint := t.Renderer.NewStyle().Foreground(t.Muted).Render(
				fmt.Sprintf(" ▾ %d more below", remaining))
			sb.WriteString(hint)
		}
	}

	sb.WriteString("\n")
	sb.WriteString(t.Renderer.NewStyle().Foreground(t.Muted).Italic(true).Render(
		" enter: open • c: clear • C: clear all • esc: close"))

	// Pad each content line for inner padding
	paddedContent := padContentLines(sb.String(), 1)

	// Use blocked/red color to match alert severity
	alertColor := t.Blocked

	panel := RenderTitledPanel(t.Renderer, paddedContent, PanelOpts{
		Title:       "Alerts!",
		Width:       panelWidth,
		BorderColor: &alertColor,
		TitleColor:  &alertColor,
	})

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		panel,
	)
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

