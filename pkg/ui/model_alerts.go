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

// renderAlertsPanel renders the alerts overlay panel
func (m Model) renderAlertsPanel() string {
	t := m.theme

	boxStyle := t.Renderer.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(min(80, m.width-4)).
		MaxHeight(m.height - 4)

	titleStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		MarginBottom(1)

	// Filter out dismissed alerts
	var visibleAlerts []drift.Alert
	for _, a := range m.alerts {
		if !m.dismissedAlerts[alertKey(a)] {
			visibleAlerts = append(visibleAlerts, a)
		}
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("🔔 Alerts Panel"))
	sb.WriteString("\n\n")

	if len(visibleAlerts) == 0 {
		sb.WriteString(t.Renderer.NewStyle().Foreground(ColorSuccess).Render("✓ No active alerts"))
		sb.WriteString("\n\n")
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
		sb.WriteString(summaryStyle.Render(summary))
		sb.WriteString("\n\n")

		// Render each alert
		for i, a := range visibleAlerts {
			selected := i == m.alertsCursor

			// Severity indicator
			var severityStyle lipgloss.Style
			var severityIcon string
			switch a.Severity {
			case drift.SeverityCritical:
				severityStyle = t.Renderer.NewStyle().Foreground(t.Blocked).Bold(true)
				severityIcon = "⚠"
			case drift.SeverityWarning:
				severityStyle = t.Renderer.NewStyle().Foreground(t.Feature)
				severityIcon = "⚡"
			default:
				severityStyle = t.Renderer.NewStyle().Foreground(t.Secondary)
				severityIcon = "ℹ"
			}

			// Cursor indicator
			cursor := "  "
			if selected {
				cursor = "▸ "
			}

			// Alert line
			line := fmt.Sprintf("%s%s %s", cursor, severityIcon, a.Message)
			if selected {
				line = t.Renderer.NewStyle().Bold(true).Render(line)
			}
			sb.WriteString(severityStyle.Render(line))
			sb.WriteString("\n")

			// Show issue ID if available and selected
			if selected && a.IssueID != "" {
				issueHint := t.Renderer.NewStyle().Foreground(t.Muted).Italic(true).Render(
					fmt.Sprintf("     Issue: %s (press Enter to jump)", a.IssueID))
				sb.WriteString(issueHint)
				sb.WriteString("\n")
			}

			// Show unblocks info for blocking cascade alerts
			if selected && a.UnblocksCount > 0 {
				unblockHint := t.Renderer.NewStyle().Foreground(t.Open).Render(
					fmt.Sprintf("     Unblocks %d items (priority sum: %d)", a.UnblocksCount, a.DownstreamPrioritySum))
				sb.WriteString(unblockHint)
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(t.Renderer.NewStyle().Foreground(t.Muted).Italic(true).Render(
		"j/k: navigate • Enter: jump to issue • d: dismiss • Esc: close"))

	content := boxStyle.Render(sb.String())

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
