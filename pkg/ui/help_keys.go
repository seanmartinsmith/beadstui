package ui

import tea "charm.land/bubbletea/v2"

// restoreFocusFromHelp returns the appropriate focus based on current view
// state. This fixes the bug where dismissing help would always return to
// focusList, even when the user was in a specialized view (graph, board,
// insights, etc.).
//
// Helper for handleHelpKeys, kept alongside the handler.
func (m Model) restoreFocusFromHelp() focus {
	// Full-screen detail view (not split mode)
	if m.showDetails && !m.isSplitView {
		return focusDetail
	}
	// Map ViewMode to the correct focus state
	switch m.mode {
	case ViewGraph:
		return focusGraph
	case ViewBoard:
		return focusBoard
	case ViewActionable:
		return focusActionable
	case ViewHistory:
		return focusHistory
	case ViewInsights, ViewAttention:
		return focusInsights
	case ViewLabelDashboard:
		return focusLabelDashboard
	case ViewEpics:
		return focusEpics
	case ViewFlowMatrix:
		return focusFlowMatrix
	case ViewTree:
		return focusTree
	}
	// Check for other focus states using stored focusBeforeHelp
	// (m.focused is focusHelp while help is open, so we use the saved value)
	if m.focusBeforeHelp == focusLabelPicker {
		return focusLabelPicker
	}
	if m.focusBeforeHelp == focusTimeTravelInput {
		return focusTimeTravelInput
	}
	// Default: return to list
	return focusList
}

// handleHelpKeys handles keyboard input when the help overlay is focused.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.Help lands in bt-ift6.9.
func (m Model) handleHelpKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.helpScroll++
		if max := m.helpScrollMax(); m.helpScroll > max {
			m.helpScroll = max
		}
	case "k", "up":
		if m.helpScroll > 0 {
			m.helpScroll--
		}
	case "ctrl+d":
		m.helpScroll += 10
		if max := m.helpScrollMax(); m.helpScroll > max {
			m.helpScroll = max
		}
	case "ctrl+u":
		m.helpScroll -= 10
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
	case "home", "g":
		m.helpScroll = 0
	case "G", "end":
		m.helpScroll = m.helpScrollMax()
	case "q", "esc", "?", "f1":
		// Close help overlay and restore previous focus
		m.closeModal()
		m.helpScroll = 0
		m.focused = m.restoreFocusFromHelp()
	case "space": // Space opens interactive tutorial (bv-0trk, bv-8y31)
		m.closeModal()
		m.helpScroll = 0
		m.openModal(ModalTutorial)
		m.tutorialModel.SetSize(m.width, m.height)
		m.focused = focusTutorial
	default:
		// Any other key dismisses help and restores previous focus
		m.closeModal()
		m.helpScroll = 0
		m.focused = m.restoreFocusFromHelp()
	}
	return m
}
