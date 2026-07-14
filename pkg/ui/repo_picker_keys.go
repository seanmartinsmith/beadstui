package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleRepoPickerKeys handles keyboard input when the repo picker is focused
// (workspace mode).
//
// Two-mode keymap (Wave 2, bt-9lpib), mirroring the label picker: when the
// search input is focused, typed characters route to the input and only
// control keys affect the modal. When unfocused (the default on open), typing
// a letter is a no-op and "/" focuses the search bar. Esc differs per mode:
// blur-search vs close-modal. Dispatches via key.Matches against
// m.keys.RepoPickerNav / m.keys.RepoPickerSearch.
func (m Model) handleRepoPickerKeys(msg tea.KeyMsg) Model {
	if m.repoPicker.IsSearchFocused() {
		return m.handleRepoPickerSearchKeys(msg)
	}
	return m.handleRepoPickerNavKeys(msg)
}

// handleRepoPickerNavKeys handles keys in the repo picker nav sub-state
// (!m.repoPicker.IsSearchFocused()). Dispatches via key.Matches against
// m.keys.RepoPickerNav.
func (m Model) handleRepoPickerNavKeys(msg tea.KeyMsg) Model {
	kk := m.keys.RepoPickerNav
	switch {
	case key.Matches(msg, kk.Cancel), key.Matches(msg, kk.Close):
		m.closeModal()
		m.focused = focusList

	case key.Matches(msg, kk.FocusSearch):
		// "/" enters search mode. Once search is focused, "/" falls through
		// to the input so it can be typed literally as part of a query.
		m.repoPicker.FocusSearch()

	case key.Matches(msg, kk.Up):
		m.repoPicker.MoveUp()

	case key.Matches(msg, kk.Down):
		m.repoPicker.MoveDown()

	case key.Matches(msg, kk.PageUp):
		m.repoPicker.PageUp()

	case key.Matches(msg, kk.PageDown):
		m.repoPicker.PageDown()

	case key.Matches(msg, kk.Toggle):
		m.repoPicker.ToggleSelected()

	case key.Matches(msg, kk.ToggleAll):
		m.repoPicker.ToggleAll()

	case key.Matches(msg, kk.Apply):
		m = m.applyRepoPickerSelection()

	default:
		// In nav mode, unknown keys (letters, etc.) are dropped silently so a
		// stray "g" doesn't leak to a global handler or start filtering.
	}
	return m
}

// handleRepoPickerSearchKeys handles keys in the repo picker search sub-state
// (m.repoPicker.IsSearchFocused()). Dispatches via key.Matches against
// m.keys.RepoPickerSearch for control keys; forwards everything else to the
// text input.
func (m Model) handleRepoPickerSearchKeys(msg tea.KeyMsg) Model {
	kk := m.keys.RepoPickerSearch
	switch {
	case key.Matches(msg, kk.BlurSearch):
		m.repoPicker.BlurSearch()

	case key.Matches(msg, kk.Apply):
		m = m.applyRepoPickerSelection()

	case key.Matches(msg, kk.ResultUp):
		m.repoPicker.MoveUp()

	case key.Matches(msg, kk.ResultDown):
		m.repoPicker.MoveDown()

	default:
		// Forward unknown keys (letters, backspace, etc.) to the text input
		// when search is focused.
		m.repoPicker.UpdateInput(msg)
	}
	return m
}

// applyRepoPickerSelection commits the current repo picker state to the active
// project filter and closes the modal. Shared by nav and search sub-states.
func (m Model) applyRepoPickerSelection() Model {
	selected := m.repoPicker.SelectedRepos()

	if m.repoPicker.NoneSelected() {
		// No checkmarks: enter jumps to the cursor project (single-project switch)
		cursorRepo := m.repoPicker.CursorRepo()
		if cursorRepo != "" {
			m.activeRepos = map[string]bool{cursorRepo: true}
			m.setStatus(fmt.Sprintf("Project filter: %s", cursorRepo))
		}
	} else if len(selected) == len(m.availableRepos) {
		// All selected: clear filter (nil = all)
		m.activeRepos = nil
		m.setStatus("Project filter: all projects")
	} else {
		m.activeRepos = selected
		m.setStatus(fmt.Sprintf("Project filter: %s", formatRepoList(sortedRepoKeys(selected), 3)))
	}

	// Apply filter to views
	if m.filter.activeRecipe != nil {
		m.applyRecipe(m.filter.activeRecipe)
	} else {
		m.applyFilter()
	}

	m.closeModal()
	m.focused = focusList
	return m
}
