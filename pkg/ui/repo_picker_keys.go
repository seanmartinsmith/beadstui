package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleRepoPickerKeys handles keyboard input when repo picker is focused
// (workspace mode).
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.RepoPicker lands in bt-ift6.9.
func (m Model) handleRepoPickerKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.repoPicker.MoveDown()
	case "k", "up":
		m.repoPicker.MoveUp()
	case "space":
		m.repoPicker.ToggleSelected()
	case "a":
		m.repoPicker.ToggleAll()
	case "esc", "q", "w":
		m.closeModal()
		m.focused = focusList
	case "enter":
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
