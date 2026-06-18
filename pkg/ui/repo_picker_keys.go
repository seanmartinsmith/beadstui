package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleRepoPickerKeys handles keyboard input when repo picker is focused
// (workspace mode).
//
// Dispatches via key.Matches against m.keys.RepoPicker per bt-ift6.9.
func (m Model) handleRepoPickerKeys(msg tea.KeyMsg) Model {
	kk := m.keys.RepoPicker
	switch {
	case key.Matches(msg, kk.Up):
		m.repoPicker.MoveUp()

	case key.Matches(msg, kk.Down):
		m.repoPicker.MoveDown()

	case key.Matches(msg, kk.Toggle):
		m.repoPicker.ToggleSelected()

	case key.Matches(msg, kk.ToggleAll):
		m.repoPicker.ToggleAll()

	case key.Matches(msg, kk.Cancel), key.Matches(msg, kk.Close):
		m.closeModal()
		m.focused = focusList

	case key.Matches(msg, kk.Apply):
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
	}
	return m
}
