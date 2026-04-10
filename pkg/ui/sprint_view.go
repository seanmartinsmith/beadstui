package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// renderSprintDashboard renders the sprint view with progress, burndown, and at-risk items (bv-161)
func (m Model) renderSprintDashboard() string {
	t := m.theme
	if m.selectedSprint == nil {
		return "No sprint selected"
	}
	sprint := m.selectedSprint

	innerWidth := m.width - 6
	if innerWidth < 40 {
		innerWidth = 40
	}

	var sb strings.Builder

	// Title
	titleStyle := t.Renderer.NewStyle().Bold(true).Foreground(t.Primary)
	sb.WriteString(titleStyle.Render(fmt.Sprintf("📅 Sprint: %s", sprint.Name)))
	sb.WriteString("\n\n")

	// Date range and days remaining
	now := time.Now()
	labelStyle := t.Renderer.NewStyle().Foreground(t.Secondary).Bold(true)
	valStyle := t.Renderer.NewStyle().Foreground(t.Base.GetForeground())

	// Calculate days remaining
	var daysRemaining int
	var sprintDuration int
	var daysPassed int
	if !sprint.EndDate.IsZero() {
		daysRemaining = int(sprint.EndDate.Sub(now).Hours() / 24)
		if daysRemaining < 0 {
			daysRemaining = 0
		}
		if !sprint.StartDate.IsZero() {
			sprintDuration = int(sprint.EndDate.Sub(sprint.StartDate).Hours() / 24)
			daysPassed = int(now.Sub(sprint.StartDate).Hours() / 24)
			if daysPassed < 0 {
				daysPassed = 0
			}
			if daysPassed > sprintDuration {
				daysPassed = sprintDuration
			}
		}
	}

	// Sprint dates line
	dateInfo := fmt.Sprintf("%s → %s",
		sprint.StartDate.Format("Jan 2"),
		sprint.EndDate.Format("Jan 2"))
	sb.WriteString(labelStyle.Render("Dates:    "))
	sb.WriteString(valStyle.Render(dateInfo))
	sb.WriteString("\n")

	// Days remaining with visual indicator
	daysStyle := valStyle
	if daysRemaining <= 2 && daysRemaining > 0 {
		daysStyle = t.Renderer.NewStyle().Foreground(t.Feature) // Warning
	} else if daysRemaining == 0 {
		daysStyle = t.Renderer.NewStyle().Foreground(t.Blocked) // Critical
	}
	sb.WriteString(labelStyle.Render("Remaining:"))
	sb.WriteString(daysStyle.Render(fmt.Sprintf(" %d days", daysRemaining)))
	sb.WriteString("\n\n")

	// Compute bead stats
	var totalBeads, closedBeads, openBeads, blockedBeads, inProgressBeads int
	var sprintIssues []model.Issue
	beadIDSet := make(map[string]bool)
	for _, id := range sprint.BeadIDs {
		beadIDSet[id] = true
	}
	for _, iss := range m.issues {
		if beadIDSet[iss.ID] {
			totalBeads++
			sprintIssues = append(sprintIssues, iss)
			if isClosedLikeStatus(iss.Status) {
				closedBeads++
				continue
			}
			switch iss.Status {
			case model.StatusBlocked:
				blockedBeads++
			case model.StatusInProgress:
				inProgressBeads++
			}
			openBeads++
		}
	}

	// Progress bar
	sb.WriteString(labelStyle.Render("Progress: "))
	progressPct := 0.0
	if totalBeads > 0 {
		progressPct = float64(closedBeads) / float64(totalBeads)
	}
	barWidth := innerWidth - 20
	if barWidth < 10 {
		barWidth = 10
	}
	filled := int(float64(barWidth) * progressPct)
	if filled > barWidth {
		filled = barWidth
	}
	barStyle := t.Renderer.NewStyle().Foreground(t.Open)
	emptyStyle := t.Renderer.NewStyle().Foreground(t.Muted)
	sb.WriteString(barStyle.Render(strings.Repeat("█", filled)))
	sb.WriteString(emptyStyle.Render(strings.Repeat("░", barWidth-filled)))
	sb.WriteString(fmt.Sprintf(" %d/%d (%.0f%%)\n", closedBeads, totalBeads, progressPct*100))

	// Status breakdown
	sb.WriteString(labelStyle.Render("Status:   "))
	sb.WriteString(t.Renderer.NewStyle().Foreground(t.Open).Render(fmt.Sprintf("✓%d ", closedBeads)))
	sb.WriteString(t.Renderer.NewStyle().Foreground(t.Feature).Render(fmt.Sprintf("⏳%d ", inProgressBeads)))
	sb.WriteString(t.Renderer.NewStyle().Foreground(t.Blocked).Render(fmt.Sprintf("⛔%d ", blockedBeads)))
	sb.WriteString(valStyle.Render(fmt.Sprintf("○%d", openBeads-inProgressBeads-blockedBeads)))
	sb.WriteString("\n\n")

	// Simple burndown chart (ASCII)
	sb.WriteString(labelStyle.Render("Burndown:"))
	sb.WriteString("\n")
	if sprintDuration > 0 && totalBeads > 0 {
		// Ideal line: from totalBeads to 0 over sprintDuration days
		// Current: totalBeads - closedBeads remaining on day daysPassed
		chartHeight := 5
		chartWidth := min(sprintDuration, 20)
		actualRemaining := float64(totalBeads - closedBeads)

		// Create simple ASCII chart
		for row := chartHeight - 1; row >= 0; row-- {
			threshold := float64(totalBeads) * float64(row+1) / float64(chartHeight)
			var line strings.Builder
			line.WriteString("  ")
			for col := 0; col <= chartWidth; col++ {
				dayFrac := float64(col) / float64(chartWidth)
				idealVal := float64(totalBeads) * (1 - dayFrac)
				passedFrac := float64(daysPassed) / float64(sprintDuration)

				if idealVal >= threshold-0.5 && idealVal < threshold+float64(totalBeads)/float64(chartHeight) {
					// Ideal line
					line.WriteString(t.Renderer.NewStyle().Foreground(t.Secondary).Render("·"))
				} else if col <= int(float64(chartWidth)*passedFrac) && actualRemaining >= threshold-0.5 && actualRemaining < threshold+float64(totalBeads)/float64(chartHeight) {
					// Actual current point
					line.WriteString(t.Renderer.NewStyle().Foreground(t.Primary).Bold(true).Render("●"))
				} else {
					line.WriteString(" ")
				}
			}
			sb.WriteString(line.String())
			sb.WriteString("\n")
		}
		sb.WriteString("  ")
		sb.WriteString(strings.Repeat("─", chartWidth+1))
		sb.WriteString("\n")
		sb.WriteString(t.Renderer.NewStyle().Foreground(t.Muted).Italic(true).Render("  · ideal  ● actual"))
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(valStyle.Render("  (insufficient data)"))
		sb.WriteString("\n\n")
	}

	// At-risk items (in_progress for more than X days without update)
	sb.WriteString(labelStyle.Render("At Risk:"))
	sb.WriteString("\n")
	const staleThresholdDays = 3
	var atRisk []model.Issue
	for _, iss := range sprintIssues {
		if iss.Status == model.StatusInProgress {
			daysSinceUpdate := int(now.Sub(iss.UpdatedAt).Hours() / 24)
			if daysSinceUpdate >= staleThresholdDays {
				atRisk = append(atRisk, iss)
			}
		}
	}
	if len(atRisk) == 0 {
		sb.WriteString(t.Renderer.NewStyle().Foreground(t.Open).Render("  ✓ No at-risk items"))
		sb.WriteString("\n")
	} else {
		for i, iss := range atRisk {
			if i >= 5 {
				sb.WriteString(valStyle.Render(fmt.Sprintf("  … +%d more", len(atRisk)-5)))
				sb.WriteString("\n")
				break
			}
			daysSinceUpdate := int(now.Sub(iss.UpdatedAt).Hours() / 24)
			sb.WriteString(t.Renderer.NewStyle().Foreground(t.Feature).Render(
				fmt.Sprintf("  ⚠ %s - %s (%dd stale)\n", iss.ID, truncateStrSprint(iss.Title, 30), daysSinceUpdate)))
		}
	}
	sb.WriteString("\n")

	// Sprint beads list (abbreviated)
	sb.WriteString(labelStyle.Render("Beads in Sprint:"))
	sb.WriteString("\n")
	displayLimit := min(10, len(sprintIssues))
	for i := 0; i < displayLimit; i++ {
		iss := sprintIssues[i]
		statusIcon := "○"
		statusStyle := valStyle
		if isClosedLikeStatus(iss.Status) {
			statusIcon = "✓"
			statusStyle = t.Renderer.NewStyle().Foreground(t.Open)
		} else {
			switch iss.Status {
			case model.StatusInProgress:
				statusIcon = "⏳"
				statusStyle = t.Renderer.NewStyle().Foreground(t.Feature)
			case model.StatusBlocked:
				statusIcon = "⛔"
				statusStyle = t.Renderer.NewStyle().Foreground(t.Blocked)
			}
		}
		sb.WriteString(statusStyle.Render(fmt.Sprintf("  %s %s - %s\n", statusIcon, iss.ID, truncateStrSprint(iss.Title, 40))))
	}
	if len(sprintIssues) > displayLimit {
		sb.WriteString(valStyle.Render(fmt.Sprintf("  … +%d more", len(sprintIssues)-displayLimit)))
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("\n")
	sb.WriteString(t.Renderer.NewStyle().Foreground(t.Muted).Italic(true).Render(
		"P: close sprint view • j/k: navigate sprints"))

	// Wrap in a box
	boxStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(min(80, m.width-4)).
		MaxHeight(m.height - 2)

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Top,
		boxStyle.Render(sb.String()),
	)
}

// truncateStrSprint truncates a string to maxLen runes, adding ellipsis if needed.
// Uses rune-based counting to safely handle UTF-8 multi-byte characters.
func truncateStrSprint(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

// handleSprintKeys handles keyboard input when in sprint view (bv-161)
func (m Model) handleSprintKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "P", "esc":
		// Exit sprint view
		m.isSprintView = false
		m.focused = focusList
	case "j", "down":
		// Next sprint
		if len(m.sprints) > 1 && m.selectedSprint != nil {
			for i, s := range m.sprints {
				if s.ID == m.selectedSprint.ID && i < len(m.sprints)-1 {
					m.selectedSprint = &m.sprints[i+1]
					m.sprintViewText = m.renderSprintDashboard()
					break
				}
			}
		}
	case "k", "up":
		// Previous sprint
		if len(m.sprints) > 1 && m.selectedSprint != nil {
			for i, s := range m.sprints {
				if s.ID == m.selectedSprint.ID && i > 0 {
					m.selectedSprint = &m.sprints[i-1]
					m.sprintViewText = m.renderSprintDashboard()
					break
				}
			}
		}
	}
	return m
}
