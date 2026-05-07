package ui

import tea "charm.land/bubbletea/v2"

// handleFlowMatrixKeys handles keyboard input when flow matrix view is focused.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.FlowMatrix lands in bt-ift6.5.
func (m Model) handleFlowMatrixKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "f", "q", "esc":
		// If in drilldown mode, close drilldown first
		if m.flowMatrix.showDrilldown {
			m.flowMatrix.showDrilldown = false
			return m
		}
		// Close flow matrix view
		m.focused = focusList
	case "j", "down":
		m.flowMatrix.MoveDown()
	case "k", "up":
		m.flowMatrix.MoveUp()
	case "tab":
		m.flowMatrix.TogglePanel()
	case "enter":
		// Open drilldown or jump to issue
		if m.flowMatrix.showDrilldown {
			// Jump to selected issue from drilldown
			if selectedIssue := m.flowMatrix.SelectedDrilldownIssue(); selectedIssue != nil {
				for i, item := range m.list.Items() {
					if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedIssue.ID {
						m.list.Select(i)
						break
					}
				}
				m.focused = focusList
				if m.isSplitView {
					m.focused = focusDetail
				} else {
					m.showDetails = true
					m.focused = focusDetail
					m.viewport.GotoTop()
				}
				m.updateViewportContent()
			}
		} else {
			// Open drilldown for selected label
			m.flowMatrix.OpenDrilldown()
		}
	case "G", "end":
		m.flowMatrix.GoToEnd()
	case "g", "home":
		m.flowMatrix.GoToStart()
	}
	return m
}
