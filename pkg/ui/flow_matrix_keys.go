package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleFlowMatrixKeys handles keyboard input when flow matrix view is focused.
//
// Dispatches via key.Matches against m.keys.FlowMatrix per ADR-004 Decision 1.
// Converted from switch msg.String() in bt-ift6.8.
func (m Model) handleFlowMatrixKeys(msg tea.KeyMsg) Model {
	k := m.keys.FlowMatrix
	switch {
	case key.Matches(msg, k.Close):
		// If in drilldown mode, close drilldown first
		if m.flowMatrix.showDrilldown {
			m.flowMatrix.showDrilldown = false
			return m
		}
		// Close flow matrix view
		m.focused = focusList
	case key.Matches(msg, k.Down):
		m.flowMatrix.MoveDown()
	case key.Matches(msg, k.Up):
		m.flowMatrix.MoveUp()
	case key.Matches(msg, k.TogglePanel):
		m.flowMatrix.TogglePanel()
	case key.Matches(msg, k.Enter):
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
	case key.Matches(msg, k.JumpBottom):
		m.flowMatrix.GoToEnd()
	case key.Matches(msg, k.JumpTop):
		m.flowMatrix.GoToStart()
	}
	return m
}
