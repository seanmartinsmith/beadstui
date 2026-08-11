package ui

import (
	"fmt"

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
		// Values already applied as the user moved through them, so this is
		// not a commit to the running UI -- it is the commit to disk. Saving
		// on keep rather than on every arrow press means scrolling past forty
		// palettes writes the file once, not forty times.
		m.saveThemeSelection()
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

// saveThemeSelection persists the picked palette and reports the outcome.
//
// Feedback is not optional here. The user's report that opened bt-4ibsq was
// "doesn't look like the color is saving" -- a save that succeeds silently and
// a save that never ran are indistinguishable from the outside, so both states
// say so explicitly.
func (m *Model) saveThemeSelection() {
	name := SelectedThemeNameFromModal(&m.settingsModal)
	if name == "" {
		return
	}
	if err := SaveSelectedTheme(name); err != nil {
		m.setStatusError(fmt.Sprintf("theme not saved: %v", err))
		return
	}
	// BT_THEME outranks both config files, so the write succeeded but will not
	// be what loads next run. Saying so beats letting the setting look broken.
	if env := ThemeEnvOverride(); env != "" && env != name {
		m.setStatus(fmt.Sprintf("saved %s, but BT_THEME=%s overrides it next run", name, env))
		return
	}
	m.setStatus(fmt.Sprintf("theme saved: %s", name))
}
