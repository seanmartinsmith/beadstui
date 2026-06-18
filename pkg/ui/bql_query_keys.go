package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/bql"
)

// handleBQLQueryKeys handles keyboard input when BQL query modal is focused.
// Returns (Model, tea.Cmd) -- distinct from other handlers because the modal
// owns asynchronous filter execution.
//
// Dispatches via key.Matches against m.keys.BQLQuery per bt-ift6.9.
// Letter keys are NOT matched here; the textinput component owns them via
// the default branch.
func (m Model) handleBQLQueryKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	kk := m.keys.BQLQuery
	switch {
	case key.Matches(msg, kk.Apply):
		query := m.bqlQuery.Value()
		if query == "" {
			// Empty query = clear BQL filter, show all
			m.filter.activeBQLExpr = nil
			m.filter.currentFilter = "all"
			m.applyFilter()
		} else {
			// Parse and validate
			parsed, err := bql.Parse(query)
			if err != nil {
				m.bqlQuery.SetError(err.Error())
				return m, nil // Stay in modal
			}
			if err := bql.Validate(parsed); err != nil {
				m.bqlQuery.SetError(err.Error())
				return m, nil // Stay in modal
			}
			// Clear stale filter state from other filter types
			m.setActiveRecipe(nil)
			m.list.ResetFilter()
			// Apply BQL via dedicated path
			m.filter.activeBQLExpr = parsed
			m.applyBQL(parsed, query)
			m.bqlQuery.AddToHistory(query)
		}
		m.closeModal()
		m.focused = focusList
		m.setStatus("BQL: " + query)
		return m, nil

	case key.Matches(msg, kk.Cancel):
		m.closeModal()
		m.focused = focusList
		return m, nil

	case key.Matches(msg, kk.HistoryPrev):
		m.bqlQuery.HistoryPrev()
		return m, nil

	case key.Matches(msg, kk.HistoryNext):
		m.bqlQuery.HistoryNext()
		return m, nil

	default:
		var cmd tea.Cmd
		m.bqlQuery, cmd = m.bqlQuery.Update(msg)
		return m, cmd
	}
}
