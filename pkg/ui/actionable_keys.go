package ui

import tea "charm.land/bubbletea/v2"

// handleActionableKeys handles keyboard input when actionable view is focused.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.Actionable lands in bt-ift6.7.
func (m Model) handleActionableKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.actionableView.MoveDown()
	case "k", "up":
		m.actionableView.MoveUp()
	case "enter":
		// Jump to selected issue in list view
		selectedID := m.actionableView.SelectedIssueID()
		if selectedID != "" {
			for i, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
			m.mode = ViewList
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
	}
	return m
}
