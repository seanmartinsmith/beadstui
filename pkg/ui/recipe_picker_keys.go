package ui

import tea "charm.land/bubbletea/v2"

// handleRecipePickerKeys handles keyboard input when recipe picker is focused.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.RecipePicker lands in bt-ift6.9.
func (m Model) handleRecipePickerKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.recipePicker.MoveDown()
	case "k", "up":
		m.recipePicker.MoveUp()
	case "esc":
		m.closeModal()
		m.focused = focusList
	case "enter":
		// Apply selected recipe
		if selected := m.recipePicker.SelectedRecipe(); selected != nil {
			m.setActiveRecipe(selected)
			m.applyRecipe(selected)
		}
		m.closeModal()
		m.focused = focusList
	}
	return m
}
