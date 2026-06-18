package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleTreeKeys handles keyboard input when tree view is focused (bv-gllx).
//
// Dispatches via key.Matches against m.keys.Tree per ADR-004 Decision 1.
// Converted from switch msg.String() in bt-ift6.8.
func (m Model) handleTreeKeys(msg tea.KeyMsg) Model {
	k := m.keys.Tree
	switch {
	case key.Matches(msg, k.Down):
		m.tree.MoveDown()
	case key.Matches(msg, k.Up):
		m.tree.MoveUp()
	case key.Matches(msg, k.Toggle):
		m.tree.ToggleExpand()
	case key.Matches(msg, k.Collapse):
		m.tree.CollapseOrJumpToParent()
	case key.Matches(msg, k.Expand):
		m.tree.ExpandOrMoveToChild()
	case key.Matches(msg, k.JumpTop):
		// Jump to top (vim-style)
		m.tree.JumpToTop()
	case key.Matches(msg, k.JumpBottom):
		m.tree.JumpToBottom()
	case key.Matches(msg, k.ExpandAll):
		m.tree.ExpandAll()
	case key.Matches(msg, k.CollapseAll):
		m.tree.CollapseAll()
	case key.Matches(msg, k.PageDown):
		m.tree.PageDown()
	case key.Matches(msg, k.PageUp):
		m.tree.PageUp()
	case key.Matches(msg, k.Back):
		// Return to list view
		m.focused = focusList
	case key.Matches(msg, k.SyncDetail):
		// Toggle detail panel (sync selection and jump to detail)
		if m.isSplitView {
			if selected := m.tree.SelectedIssue(); selected != nil {
				// Sync detail panel with tree selection
				for i, item := range m.list.Items() {
					if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selected.ID {
						m.list.Select(i)
						break
					}
				}
				m.updateViewportContent()
				m.focused = focusDetail
			}
		}
	}
	return m
}
