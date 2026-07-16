package ui

import (
	tea "charm.land/bubbletea/v2"
)

// handleMemoriesKeys handles keyboard input while the Memories view has
// focus. Single entry point reached from two call sites (mirrors
// handleHistoryKeys): the early "in-view typing sub-state" guard in
// model_update_input.go (while search is active, so letters that collide
// with global hotkeys are typed instead of switching views) and the
// bottom focus-specific dispatch switch (once search is inactive).
//
// Direct msg.String() matching rather than a dedicated keys.MemoriesKeys /
// help.KeyMap pair (mirrors LabelDashboardModel, not HistoryModel) - the
// view has a small, stable key surface. The L1 footer and ; sidebar fall
// back to the Global map for this view, same as LabelDashboard/Attention.
func (m Model) handleMemoriesKeys(msg tea.KeyMsg) Model {
	if m.memories.IsSearchActive() {
		switch msg.String() {
		case "esc":
			m.memories.CancelSearch()
		case "enter":
			m.memories.ConfirmSearch()
		default:
			m.memories.UpdateSearchInput(msg)
		}
		return m
	}

	switch msg.String() {
	case "/":
		m.memories.StartSearch()
	case "j", "down":
		if m.memories.detailFocused && m.memories.isSplitWidth() {
			m.memories.ScrollDetailDown()
		} else {
			m.memories.MoveDown()
		}
	case "k", "up":
		if m.memories.detailFocused && m.memories.isSplitWidth() {
			m.memories.ScrollDetailUp()
		} else {
			m.memories.MoveUp()
		}
	case "tab":
		m.memories.ToggleDetailFocus()
	case "enter":
		m.memories.FocusDetail()
	case "backspace":
		// Single-pane detail -> back to master (split mode already has both
		// panes visible; tab handles focus there).
		if !m.memories.isSplitWidth() {
			m.memories.detailFocused = false
		}
	}
	return m
}
