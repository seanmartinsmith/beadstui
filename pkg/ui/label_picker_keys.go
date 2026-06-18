package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleLabelPickerKeys handles keyboard input when label picker is focused
// (bv-126). Letter keys are NOT used for navigation -- they go to the text
// input for search. Only arrow keys and ctrl combos navigate. Space toggles
// multi-select.
//
// Dispatches via key.Matches against m.keys.LabelPickerNav (nav sub-state)
// or m.keys.LabelPickerSearch (search sub-state) per ADR-004 Decision 7 and
// bt-ift6.9.
func (m Model) handleLabelPickerKeys(msg tea.KeyMsg) Model {
	// Two-mode keymap (bt-wnda): when the search input is focused, typed
	// characters route to the input and only Esc/Enter/up-down/etc affect
	// the modal. When unfocused (the default on open), typing letters is a
	// no-op and "/" focuses the search bar. Esc has different semantics
	// in each mode: blur-search vs close-modal. This mirrors the issues-
	// list pattern.
	if m.labelPicker.IsSearchFocused() {
		return m.handleLabelPickerSearchKeys(msg)
	}
	return m.handleLabelPickerNavKeys(msg)
}

// handleLabelPickerNavKeys handles keys in label picker nav sub-state
// (!m.labelPicker.IsSearchFocused()). Dispatches via key.Matches against
// m.keys.LabelPickerNav.
func (m Model) handleLabelPickerNavKeys(msg tea.KeyMsg) Model {
	kk := m.keys.LabelPickerNav
	switch {
	case key.Matches(msg, kk.Cancel):
		m.closeModal()
		m.focused = focusList

	case key.Matches(msg, kk.Close):
		// Press the same key that opens it to close (toggle behavior).
		m.closeModal()
		m.focused = focusList

	case key.Matches(msg, kk.FocusSearch):
		// "/" enters search mode. Once search is focused, "/" falls through
		// to the input so it can be typed literally as part of a query.
		m.labelPicker.FocusSearch()

	case key.Matches(msg, kk.Up):
		m.labelPicker.MoveUp()

	case key.Matches(msg, kk.Down):
		m.labelPicker.MoveDown()

	case key.Matches(msg, kk.PageUp):
		m.labelPicker.PageUp()

	case key.Matches(msg, kk.PageDown):
		m.labelPicker.PageDown()

	case key.Matches(msg, kk.Toggle):
		// Space toggles label selection in nav mode.
		m.labelPicker.ToggleSelected()

	case key.Matches(msg, kk.Apply):
		m = m.applyLabelPickerSelection()

	default:
		// In nav mode, unknown keys (letters, etc.) are dropped silently
		// so a stray "g" doesn't silently start filtering.
	}
	return m
}

// handleLabelPickerSearchKeys handles keys in label picker search sub-state
// (m.labelPicker.IsSearchFocused()). Dispatches via key.Matches against
// m.keys.LabelPickerSearch for control keys; forwards everything else to
// the text input.
func (m Model) handleLabelPickerSearchKeys(msg tea.KeyMsg) Model {
	kk := m.keys.LabelPickerSearch
	switch {
	case key.Matches(msg, kk.BlurSearch):
		m.labelPicker.BlurSearch()

	case key.Matches(msg, kk.Apply):
		m = m.applyLabelPickerSelection()

	case key.Matches(msg, kk.ResultUp):
		m.labelPicker.MoveUp()

	case key.Matches(msg, kk.ResultDown):
		m.labelPicker.MoveDown()

	default:
		// Forward unknown keys (letters, backspace, etc.) to the text
		// input when search is focused.
		m.labelPicker.UpdateInput(msg)
	}
	return m
}

// applyLabelPickerSelection commits the current label picker state to the
// active filter and closes the modal. Shared by nav and search sub-states.
func (m Model) applyLabelPickerSelection() Model {
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
	return m
}
