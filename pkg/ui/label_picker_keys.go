package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleLabelPickerKeys handles keyboard input when label picker is focused
// (bv-126). Letter keys are NOT used for navigation - they go to the text
// input for search. Only arrow keys and ctrl combos navigate. Space toggles
// multi-select.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.LabelPicker (split into
// LabelPickerNavKeys + LabelPickerSearchKeys per ADR-004 Decision 7) lands
// in bt-ift6.9.
func (m Model) handleLabelPickerKeys(msg tea.KeyMsg) Model {
	// Two-mode keymap (bt-wnda): when the search input is focused, typed
	// characters route to the input and only Esc/Enter/up-down/etc affect
	// the modal. When unfocused (the default on open), typing letters is a
	// no-op and "/" focuses the search bar. Esc has different semantics
	// in each mode: blur-search vs close-modal. This mirrors the issues-
	// list pattern.
	key := msg.String()
	switch key {
	case "esc":
		if m.labelPicker.IsSearchFocused() {
			m.labelPicker.BlurSearch()
			return m
		}
		m.closeModal()
		m.focused = focusList
	case "l":
		// Press the same key to close (toggle behavior). Only when search
		// is not focused — when search owns input, "l" is a literal letter.
		if m.labelPicker.IsSearchFocused() {
			m.labelPicker.UpdateInput(msg)
			return m
		}
		m.closeModal()
		m.focused = focusList
	case "/":
		// "/" enters search mode when not already there. Once search is
		// focused, "/" falls through to the input so it can be typed
		// literally as part of a query.
		if !m.labelPicker.IsSearchFocused() {
			m.labelPicker.FocusSearch()
			return m
		}
		m.labelPicker.UpdateInput(msg)
	case "down", "ctrl+n":
		m.labelPicker.MoveDown()
	case "up", "ctrl+p":
		m.labelPicker.MoveUp()
	case "j":
		// j is text input when typing a query, navigation otherwise.
		if m.labelPicker.IsSearchFocused() {
			m.labelPicker.UpdateInput(msg)
		} else {
			m.labelPicker.MoveDown()
		}
	case "k":
		if m.labelPicker.IsSearchFocused() {
			m.labelPicker.UpdateInput(msg)
		} else {
			m.labelPicker.MoveUp()
		}
	case "left":
		m.labelPicker.PageUp()
	case "right":
		m.labelPicker.PageDown()
	case "pgup":
		m.labelPicker.PageUp()
	case "pgdown":
		m.labelPicker.PageDown()
	case "space":
		// Space toggles label selection in nav mode; in search mode it
		// inserts a literal space into the query.
		if m.labelPicker.IsSearchFocused() {
			m.labelPicker.UpdateInput(msg)
		} else {
			m.labelPicker.ToggleSelected()
		}
	case "enter":
		selected := m.labelPicker.SelectedLabels()
		if len(selected) == 0 {
			// Two distinct paths produce 0 selected labels:
			//   (a) the user opened the modal with an active filter and
			//       deselected all the checkmarks -- they want to clear back
			//       to "all".
			//   (b) the user opened the modal cold (no filter active) and
			//       just pressed Enter on a label without space-toggling --
			//       the long-standing shortcut applies the cursor label as
			//       a single-select filter.
			// Distinguished via OpenedWithFilter().
			if m.labelPicker.OpenedWithFilter() {
				m.filter.labelFilter = ""
				m.applyFilter()
				m.setStatus("Cleared label filter")
			} else if cursor := m.labelPicker.SelectedLabel(); cursor != "" {
				m.filter.labelFilter = cursor
				m.applyFilter()
				m.setStatus(fmt.Sprintf("Filtered by label: %s", cursor))
			}
		} else if len(selected) == 1 {
			m.filter.labelFilter = selected[0]
			m.applyFilter()
			m.setStatus(fmt.Sprintf("Filtered by label: %s", selected[0]))
		} else {
			// Multi-select: comma-separated labels
			m.filter.labelFilter = strings.Join(selected, ",")
			m.applyFilter()
			m.setStatus(fmt.Sprintf("Filtered by %d labels", len(selected)))
		}
		m.closeModal()
		m.focused = focusList
	default:
		// Only forward unknown keys (letters, backspace, etc.) to the text
		// input when search is focused. In nav mode they're dropped silently
		// so a stray "g" doesn't silently start filtering.
		if m.labelPicker.IsSearchFocused() {
			m.labelPicker.UpdateInput(msg)
		}
	}
	return m
}
