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
		m = m.syncTreeSelectionToDetail()
	}
	return m
}

// syncTreeSelectionToDetail moves the main list's selection to the tree
// view's currently selected issue and focuses the detail pane. This is the
// tree view's only "open a bead" action -- ViewTree has no full-screen
// detail viewport of its own (that surface belongs to bt-krx1), so both the
// SyncDetail key (Tab) and the mouse double-click gesture (bt-w8j8.2) route
// through this shared helper rather than duplicating the lookup-and-focus
// logic. No-op when split view is inactive, since there is no detail pane
// to jump to.
func (m Model) syncTreeSelectionToDetail() Model {
	if !m.isSplitView {
		return m
	}
	selected := m.tree.SelectedIssue()
	if selected == nil {
		return m
	}
	for i, item := range m.list.Items() {
		if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selected.ID {
			m.list.Select(i)
			break
		}
	}
	m.updateViewportContent()
	m.focused = focusDetail
	return m
}
