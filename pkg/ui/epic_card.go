package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// renderEpicCard renders the tier-2 epic focus card (ModalEpicCard): the epic's
// children as status pills (the shared buildEpicProgressANSI), with a cursor on
// the drillable child. Composited via OverlayCenterDimBackdrop in View() per
// docs/design/tui-modal-compositing.md, so this returns just the titled panel.
//
// The child rows are windowed around the cursor (with ↑/↓ "N more" indicators)
// so a large epic still fits a scrunched terminal - the user routinely runs
// 14-30 row windows. buildEpicProgressANSI stays the single source of truth for
// the row styling; the windowing lives here (bt-gfxhz.3).
func (m Model) renderEpicCard() string {
	epic, ok := m.data.issueMap[m.epicCardID]
	if !ok || epic == nil {
		return ""
	}

	// Box width: content-comfortable, narrows on small terminals.
	boxWidth := min(70, m.width-6)
	if boxWidth < 36 {
		boxWidth = 36
	}
	// Inner content width (RenderTitledPanel borders take 2; leave a breathing
	// column each side for title truncation in buildEpicProgressANSI).
	innerWidth := boxWidth - 4
	if innerWidth < 24 {
		innerWidth = 24
	}

	footer := lipgloss.NewStyle().Foreground(ColorSecondary).Italic(true).
		Render("j/k move · ⏎ drill · esc close")

	body := buildEpicProgressANSI(*epic, m.data.issues, m.epicCardCursor, innerWidth)

	// Body height cap: ~80% of the terminal (the user runs scrunched windows),
	// never taller than the screen. The box then sizes DOWN to its content so a
	// small epic gets a snug card instead of a tall mostly-empty one.
	maxBox := m.height * 8 / 10
	if maxBox > m.height-2 {
		maxBox = m.height - 2
	}
	if maxBox < 8 {
		maxBox = 8
	}

	const headerRows = 2 // summary + blank (from buildEpicProgressANSI)
	const footerRows = 2 // blank separator + key hint

	var content string
	var boxHeight int
	if body == "" {
		content = lipgloss.NewStyle().Foreground(ColorMuted).Render("No children.") + "\n\n" + footer
		boxHeight = 2 + 1 + footerRows // borders + the "No children." line + footer
	} else {
		// buildEpicProgressANSI returns: summary, blank, then one row per child.
		lines := strings.Split(body, "\n")
		header := lines[:headerRows]
		rows := lines[headerRows:]

		// Snug box: just tall enough for every row, capped at maxBox.
		boxHeight = 2 + headerRows + len(rows) + footerRows
		if boxHeight > maxBox {
			boxHeight = maxBox
		}
		if boxHeight < 8 {
			boxHeight = 8
		}

		// slots = rows inside the borders available for children + indicators.
		slots := boxHeight - 2 - headerRows - footerRows
		if slots < 1 {
			slots = 1
		}

		start, end := 0, len(rows)
		scrolling := len(rows) > slots
		if scrolling {
			// Reserve both indicator rows so the window never overruns, then
			// center it on the cursor.
			win := slots - 2
			if win < 1 {
				win = 1
			}
			if m.epicCardCursor >= win {
				start = m.epicCardCursor - win + 1
			}
			end = start + win
			if end > len(rows) {
				end = len(rows)
				start = max(0, end-win)
			}
		}

		muted := lipgloss.NewStyle().Foreground(ColorMuted)
		var sb strings.Builder
		for _, h := range header {
			sb.WriteString(h)
			sb.WriteString("\n")
		}
		if scrolling && start > 0 {
			sb.WriteString(muted.Render(fmt.Sprintf("  ↑ %d more", start)))
			sb.WriteString("\n")
		}
		for i := start; i < end; i++ {
			sb.WriteString(rows[i])
			sb.WriteString("\n")
		}
		if scrolling && end < len(rows) {
			sb.WriteString(muted.Render(fmt.Sprintf("  ↓ %d more", len(rows)-end)))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(footer)
		content = sb.String()
	}

	return RenderTitledPanel(content, PanelOpts{
		Title:   "Epic " + epic.ID,
		Width:   boxWidth,
		Height:  boxHeight,
		Focused: true,
	})
}

// handleEpicCardKeys handles keyboard input while the epic focus card is open
// (activeModal == ModalEpicCard). j/k move the child cursor; enter drills into
// the selected child (jump + focus detail, the alerts-modal mechanism); esc
// closes the card and restores the underlying surface. All other keys are
// swallowed, per the modal contract. bt-gfxhz.3.
func (m Model) handleEpicCardKeys(msg tea.KeyMsg) Model {
	k := m.keys.EpicCard
	children := epicChildrenSorted(m.epicCardID, m.data.issues)
	switch {
	case key.Matches(msg, k.Down):
		if m.epicCardCursor < len(children)-1 {
			m.epicCardCursor++
		}
	case key.Matches(msg, k.Up):
		if m.epicCardCursor > 0 {
			m.epicCardCursor--
		}
	case key.Matches(msg, k.Open):
		if len(children) > 0 && m.epicCardCursor < len(children) {
			childID := children[m.epicCardCursor].ID
			m.closeModal()
			if m.selectIssueByID(childID) {
				m.focusDetailAfterJump()
			} else {
				m.setStatus("Child " + childID + " not in current view")
			}
		}
	case key.Matches(msg, k.Exit):
		m.closeModal()
	}
	return m
}
