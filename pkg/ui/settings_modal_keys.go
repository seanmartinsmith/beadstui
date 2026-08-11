package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleSettingsModalKeys handles input while the settings screen is open.
//
// Dispatches via key.Matches against m.keys.Settings, matching the pattern the
// other modals use (ADR-004: modals own their dispatch).
func (m Model) handleSettingsModalKeys(msg tea.KeyMsg) Model {
	kk := m.keys.Settings

	switch {
	case key.Matches(msg, kk.Up):
		m.settingsModal.MoveUp()

	case key.Matches(msg, kk.Down):
		m.settingsModal.MoveDown()

	case key.Matches(msg, kk.Prev):
		if it := m.settingsModal.Selected(); it != nil {
			it.Prev(&m)
		}

	case key.Matches(msg, kk.Next):
		if it := m.settingsModal.Selected(); it != nil {
			it.Next(&m)
		}

	case key.Matches(msg, kk.Keep):
		// Values applied as the user moved through them, so there is nothing
		// to commit -- keeping is just leaving without reverting.
		m.closeModal()
		m.focused = focusList

	case key.Matches(msg, kk.Cancel):
		// Restore whatever was rendering when the screen opened. A theme
		// repaints the entire UI as you scroll past it, so leaving the last
		// one you happened to land on would make cancel indistinguishable
		// from keep.
		if orig := m.settingsModal.OriginalTheme(); orig != "" {
			m.applyThemeLive(orig)
		}
		m.closeModal()
		m.focused = focusList
	}
	return m
}
