package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleTimeTravelInputKeys handles keyboard input for the time-travel
// revision prompt.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.TimeTravelInput lands in
// bt-ift6.9.
func (m Model) handleTimeTravelInputKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "enter":
		// Submit the revision
		revision := strings.TrimSpace(m.timeTravelInput.Value())
		if revision == "" {
			revision = "HEAD~5" // Default if empty
		}
		m.closeModal()
		m.timeTravelInput.Blur()
		m.focused = focusList
		m.enterTimeTravelMode(revision)
	case "esc":
		// Cancel
		m.closeModal()
		m.timeTravelInput.Blur()
		m.focused = focusList
	default:
		// Update the textinput
		m.timeTravelInput, _ = m.timeTravelInput.Update(msg)
	}
	return m
}
