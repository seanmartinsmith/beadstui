package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IssueDelegate renders issue items in the list
type IssueDelegate struct {
	Theme             Theme
	ShowPriorityHints bool
	PriorityHints     map[string]*analysis.PriorityRecommendation
	WorkspaceMode     bool // When true, shows repo prefix badges
	ShowSearchScores  bool // Show semantic/hybrid score badge when search is active
}

func (d IssueDelegate) Height() int {
	return 1
}

func (d IssueDelegate) Spacing() int {
	return 0
}

func (d IssueDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d IssueDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(IssueItem)
	if !ok {
		return
	}

	t := d.Theme
	width := m.Width()
	if width <= 0 {
		width = 80
	}
	// Reduce width by 1 to prevent terminal wrapping on the exact edge
	width = width - 1

	isSelected := index == m.Index()

	// ══════════════════════════════════════════════════════════════════════════
	// POLISHED ROW LAYOUT - Stripe-level visual hierarchy
	// Layout: [sel] [type] [prio-badge] [status-badge] [ID] [title...] [meta]
	// ══════════════════════════════════════════════════════════════════════════

	// Get all the data
	icon, iconColor := t.GetTypeIcon(string(i.Issue.IssueType))
	idStr := i.Issue.ID
	title := i.Issue.Title
	ageStr := FormatTimeRel(i.Issue.CreatedAt)
	commentCount := len(i.Issue.Comments)

	// Measure actual icon display width (emojis vary: 1-2 cells)
	iconDisplayWidth := lipgloss.Width(icon)

	// Calculate widths for right-side columns (fixed)
	rightWidth := 0
	var rightParts []string

	// Show Age and Comments only if we have reasonable width
	if width > 60 {
		// Age - with subtle styling (using pre-computed style)
		rightParts = append(rightParts, t.MutedText.Render(fmt.Sprintf("%8s", ageStr)))
		rightWidth += 9

		// Comments with icon - use lipgloss.Width for accurate emoji measurement
		if commentCount > 0 {
			commentStr := fmt.Sprintf("💬%d", commentCount)
			rightParts = append(rightParts, t.InfoText.Render(commentStr))
			rightWidth += lipgloss.Width(commentStr) + 1 // +1 for spacing
		} else {
			rightParts = append(rightParts, "   ")
			rightWidth += 3
		}
	}

	// Sparkline (Graph Score) - visualization of importance
	if width > 120 {
		spark := RenderSparkline(i.GraphScore, 5)
		sparkColor := GetHeatmapColor(i.GraphScore, t)
		sparkStyle := t.Renderer.NewStyle().Foreground(sparkColor)
		rightParts = append(rightParts, sparkStyle.Render(spark))
		rightWidth += 6 // 5 + 1 spacing
	}

	// Assignee (if present and we have room)
	if width > 100 && i.Issue.Assignee != "" {
		assignee := truncateRunesHelper(i.Issue.Assignee, 12, "…")
		rightParts = append(rightParts, t.SecondaryText.Render(fmt.Sprintf("@%-12s", assignee)))
		rightWidth += 14
	}

	// Labels (if present and we have room) - render as mini tags
	if width > 140 && len(i.Issue.Labels) > 0 {
		labelStr := truncateRunesHelper(strings.Join(i.Issue.Labels, ","), 20, "…")
		labelStyle := t.Renderer.NewStyle().
			Foreground(ColorPrimary).
			Background(ColorBgSubtle).
			Padding(0, 1)
		rightParts = append(rightParts, labelStyle.Render(labelStr))
		rightWidth += lipgloss.Width(labelStyle.Render(labelStr)) + 1
	}

	// Left side fixed columns with polished badges
	// [selector 2] [repo-badge 0-6] [icon 1-2] [prio-badge 3] [hint 1-2] [status-badge 6] [id dynamic] [space]
	// Use measured iconDisplayWidth instead of hardcoded value for proper alignment
	leftFixedWidth := 2 + iconDisplayWidth + 1 // selector(2) + icon(measured) + space(1)

	// Repo badge width (workspace mode)
	var repoBadge string
	if d.WorkspaceMode && i.RepoPrefix != "" {
		// Create a compact repo badge like [API] or [WEB]
		repoBadge = RenderRepoBadge(i.RepoPrefix)
		leftFixedWidth += lipgloss.Width(repoBadge) + 1
	}

	// Priority badge (polished)
	prioBadge := RenderPriorityBadge(i.Issue.Priority)
	prioBadgeWidth := lipgloss.Width(prioBadge)
	leftFixedWidth += prioBadgeWidth + 1

	// Priority hint indicator
	if d.ShowPriorityHints {
		leftFixedWidth += 2
	}

	// Triage indicator width (bv-151) - use lipgloss.Width for accurate emoji measurement
	if i.IsQuickWin {
		leftFixedWidth += lipgloss.Width("⭐") + 1 // emoji + space
	} else if i.IsBlocker && i.UnblocksCount > 0 {
		leftFixedWidth += lipgloss.Width(fmt.Sprintf("🔓%d", i.UnblocksCount)) + 1 // emoji+count + space
	} else if i.UnblocksCount > 0 {
		leftFixedWidth += lipgloss.Width(fmt.Sprintf("↪%d", i.UnblocksCount)) + 1 // arrow+count + space
	}

	// Status badge (polished)
	statusBadge := RenderStatusBadge(string(i.Issue.Status))
	statusBadgeWidth := lipgloss.Width(statusBadge)
	leftFixedWidth += statusBadgeWidth + 1

	// Search score badge (semantic/hybrid)
	var searchBadge string
	if d.ShowSearchScores && i.SearchScoreSet {
		scoreStr := fmt.Sprintf("%.2f", i.SearchScore)
		searchBadge = t.InfoBold.Render(fmt.Sprintf("[%s]", scoreStr))
		leftFixedWidth += lipgloss.Width(searchBadge) + 1
	}

	// ID width - use actual visual width, but cap reasonably
	idWidth := lipgloss.Width(idStr)
	if idWidth > 35 {
		idWidth = 35
		idStr = truncateRunesHelper(idStr, 35, "…")
	}
	leftFixedWidth += idWidth + 1

	// Diff badge width adjustment
	if badge := i.DiffStatus.Badge(); badge != "" {
		leftFixedWidth += lipgloss.Width(badge) + 1
	}

	// Title gets everything in between
	titleWidth := width - leftFixedWidth - rightWidth - 2
	if titleWidth < 5 {
		titleWidth = 5
	}

	// Truncate title if needed
	title = truncateRunesHelper(title, titleWidth, "…")

	// Pad title to fill space
	currentWidth := lipgloss.Width(title)
	if currentWidth < titleWidth {
		title = title + strings.Repeat(" ", titleWidth-currentWidth)
	}

	// ══════════════════════════════════════════════════════════════════════════
	// BUILD THE ROW
	// ══════════════════════════════════════════════════════════════════════════
	var leftSide strings.Builder

	// Selection indicator with accent color (using pre-computed style)
	if isSelected {
		leftSide.WriteString(t.PrimaryBold.Render("▸ "))
	} else {
		leftSide.WriteString("  ")
	}

	// Repo badge (workspace mode)
	if repoBadge != "" {
		leftSide.WriteString(repoBadge)
		leftSide.WriteString(" ")
	}

	// Type icon with color
	leftSide.WriteString(t.Renderer.NewStyle().Foreground(iconColor).Render(icon))
	leftSide.WriteString(" ")

	// Priority badge (polished)
	leftSide.WriteString(prioBadge)
	leftSide.WriteString(" ")

	// Priority hint indicator (↑/↓) - using pre-computed styles
	if d.ShowPriorityHints && d.PriorityHints != nil {
		if hint, ok := d.PriorityHints[i.Issue.ID]; ok {
			if hint.Direction == "increase" {
				leftSide.WriteString(t.PriorityUpArrow.Render("↑"))
			} else if hint.Direction == "decrease" {
				leftSide.WriteString(t.PriorityDownArrow.Render("↓"))
			}
		} else {
			leftSide.WriteString(" ")
		}
		leftSide.WriteString(" ")
	}

	// Triage indicators (bv-151): Quick win ⭐ and Unblocks count 🔓 - using pre-computed styles
	triageIndicator := ""
	if i.IsQuickWin {
		triageIndicator = t.TriageStar.Render("⭐")
	} else if i.IsBlocker && i.UnblocksCount > 0 {
		triageIndicator = t.TriageUnblocks.Render(fmt.Sprintf("🔓%d", i.UnblocksCount))
	} else if i.UnblocksCount > 0 {
		triageIndicator = t.TriageUnblocksAlt.Render(fmt.Sprintf("↪%d", i.UnblocksCount))
	}
	if triageIndicator != "" {
		leftSide.WriteString(triageIndicator)
		leftSide.WriteString(" ")
	}

	// Status badge (polished)
	leftSide.WriteString(statusBadge)
	leftSide.WriteString(" ")

	// Search score badge (optional)
	if searchBadge != "" {
		leftSide.WriteString(searchBadge)
		leftSide.WriteString(" ")
	}

	// ID with secondary styling (using pre-computed style base)
	idStyle := t.SecondaryText
	if isSelected {
		idStyle = idStyle.Bold(true)
	}
	leftSide.WriteString(idStyle.Render(idStr))
	leftSide.WriteString(" ")

	// Diff badge (time-travel mode)
	if badge := i.DiffStatus.Badge(); badge != "" {
		leftSide.WriteString(badge)
		leftSide.WriteString(" ")
	}

	// Title with emphasis when selected
	titleStyle := t.Renderer.NewStyle()
	if isSelected {
		titleStyle = titleStyle.Foreground(t.Primary).Bold(true)
	} else {
		titleStyle = titleStyle.Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#E8E8E8"})
	}
	leftSide.WriteString(titleStyle.Render(title))

	// Right side
	rightSide := strings.Join(rightParts, " ")

	// Combine: left + padding + right
	leftLen := lipgloss.Width(leftSide.String())
	rightLen := lipgloss.Width(rightSide)
	padding := width - leftLen - rightLen
	if padding < 0 {
		padding = 0
	}

	// Construct the row string
	row := leftSide.String() + strings.Repeat(" ", padding) + rightSide

	// Apply row background for selection and clamp width
	rowStyle := t.Renderer.NewStyle().Width(width).MaxWidth(width)
	if isSelected {
		row = rowStyle.Background(t.Highlight).Render(row)
	} else {
		row = rowStyle.Render(row)
	}

	fmt.Fprint(w, row)
}
