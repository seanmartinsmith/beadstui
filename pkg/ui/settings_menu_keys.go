package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleSettingsMenuKeys handles input while the esc menu is open.
//
// Selecting an entry replaces this modal with the one it names rather than
// stacking on top of it. bt has no modal stack, and the menu has nothing to
// return to -- backing out of Options should land on the list, which is where
// esc came from, not on a menu the user has already finished with.
func (m Model) handleSettingsMenuKeys(msg tea.KeyMsg) Model {
	kk := m.keys.Settings

	switch {
	case key.Matches(msg, kk.Up):
		m.settingsMenu.MoveUp()

	case key.Matches(msg, kk.Down):
		m.settingsMenu.MoveDown()

	case key.Matches(msg, kk.Keep):
		switch m.settingsMenu.SelectedIndex() {
		case menuOptions:
			m.openModal(ModalSettings)
			m.settingsModal.Reset()
			m.settingsModal.SetTheme(m.theme)
			m.settingsModal.SetSize(m.width, m.height-1)
			m.focused = focusSettings

		case menuHelp:
			m.openModal(ModalHelp)
			m.focused = focusHelp

		case menuQuit:
			m.openModal(ModalQuitConfirm)
			m.focused = focusQuitConfirm
		}

	case key.Matches(msg, kk.Cancel):
		m.closeModal()
		m.focused = focusList
	}
	return m
}
