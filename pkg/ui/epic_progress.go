package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// buildEpicProgressANSI renders an epic's progress summary line plus one
// status-pill row per child, in natural-numeric order (epicChildrenSorted). It
// is the single source of truth shared by the detail-pane Epic Progress block
// and the tier-2 epic focus card (bt-gfxhz.3): build once, render both.
//
// selectedIdx highlights one child row with a ▸ cursor (the focus card's
// selection); pass -1 for the static detail-pane embed. width is the available
// content width — child titles truncate to whatever remains after the fixed
// cursor/pill/id segments (0 disables truncation). Returns "" when the epic has
// no children so callers can skip the section (and its heading) entirely.
//
// Pills reuse RenderStatusBadge / RenderPriorityBadge (styles.go). Closed
// children render their id+title as a single Faint span so completed work
// recedes (replacing the old markdown strikethrough) while the grey DONE pill
// still conveys status; the colored pills on active children pop. The summary +
// done count match the detail block's prior epicProgress semantics
// (Status.IsClosed). The output contains ANSI SGR sequences and MUST be routed
// through addANSI (renderSection's ANSI track), never addMD — lipgloss cannot
// survive Glamour's chroma code-fence path (bt-x5xc4).
func buildEpicProgressANSI(epic model.Issue, allIssues []model.Issue, selectedIdx, width int) string {
	children := epicChildrenSorted(epic.ID, allIssues)
	if len(children) == 0 {
		return ""
	}

	done := 0
	for _, c := range children {
		if c.Status.IsClosed() {
			done++
		}
	}
	total := len(children)
	pct := done * 100 / total

	summaryStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	idStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
	cursorStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	faintStyle := lipgloss.NewStyle().Faint(true)

	lines := []string{
		summaryStyle.Render(fmt.Sprintf("%d / %d children complete (%d%%)", done, total, pct)),
		"",
	}

	for i, child := range children {
		statusPill := RenderStatusBadge(string(child.Status))
		prioPill := RenderPriorityBadge(child.Priority)

		cursor := "  "
		if i == selectedIdx {
			cursor = cursorStyle.Render("▸ ")
		}

		title := child.Title
		if width > 0 {
			// Title budget: width minus the fixed leading segments. Pills carry
			// background SGR but their display width is the label text width, so
			// lipgloss.Width measures the on-screen cells correctly.
			fixed := lipgloss.Width(cursor) + lipgloss.Width(statusPill) + 1 +
				lipgloss.Width(prioPill) + 1 + lipgloss.Width(child.ID) + 3 // " — "
			budget := width - fixed
			if budget < 0 {
				budget = 0
			}
			title = truncateString(child.Title, budget)
		}

		// Closed children: id + title as one Faint span (no nested SGR reset to
		// corrupt the dim). Active children: secondary-colored id, plain title.
		var idAndTitle string
		if child.Status.IsClosed() {
			body := child.ID
			if title != "" {
				body += " — " + title
			}
			idAndTitle = faintStyle.Render(body)
		} else {
			idAndTitle = idStyle.Render(child.ID)
			if title != "" {
				idAndTitle += " — " + title
			}
		}

		lines = append(lines, cursor+statusPill+" "+prioPill+" "+idAndTitle)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
