package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleTimeTravelInputKeys handles keyboard input for the time-travel
// revision prompt.
//
// Dispatches via key.Matches against m.keys.TimeTravelInput per bt-ift6.9.
// Letter keys are NOT matched here; the textinput component owns them via
// the default branch.
func (m Model) handleTimeTravelInputKeys(msg tea.KeyMsg) Model {
	kk := m.keys.TimeTravelInput
	switch {
	case key.Matches(msg, kk.Submit):
		// Submit the revision
		revision := strings.TrimSpace(m.timeTravelInput.Value())
		if revision == "" {
			revision = "HEAD~5" // Default if empty
		}
		m.closeModal()
		m.timeTravelInput.Blur()
		m.focused = focusList
		m.enterTimeTravelMode(revision)

	case key.Matches(msg, kk.Cancel):
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
