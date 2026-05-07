package ui

import tea "charm.land/bubbletea/v2"

// handleTreeKeys handles keyboard input when tree view is focused (bv-gllx).
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.Tree lands in bt-ift6.8.
func (m Model) handleTreeKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.tree.MoveDown()
	case "k", "up":
		m.tree.MoveUp()
	case "enter", "space":
		m.tree.ToggleExpand()
	case "h", "left":
		m.tree.CollapseOrJumpToParent()
	case "l", "right":
		m.tree.ExpandOrMoveToChild()
	case "g":
		// Jump to top (vim-style)
		m.tree.JumpToTop()
	case "G":
		m.tree.JumpToBottom()
	case "o":
		m.tree.ExpandAll()
	case "O":
		m.tree.CollapseAll()
	case "ctrl+d", "pgdown":
		m.tree.PageDown()
	case "ctrl+u", "pgup":
		m.tree.PageUp()
	case "E", "esc":
		// Return to list view
		m.focused = focusList
	case "tab":
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
