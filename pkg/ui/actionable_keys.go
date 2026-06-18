package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleActionableKeys handles keyboard input when actionable view is focused.
//
// Dispatches via key.Matches against m.keys.Actionable per ADR-004
// Decision 1. Converted from switch msg.String() in bt-ift6.7.
func (m Model) handleActionableKeys(msg tea.KeyMsg) Model {
	k := m.keys.Actionable
	switch {
	case key.Matches(msg, k.Down):
		m.actionableView.MoveDown()
	case key.Matches(msg, k.Up):
		m.actionableView.MoveUp()
	case key.Matches(msg, k.Enter):
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
