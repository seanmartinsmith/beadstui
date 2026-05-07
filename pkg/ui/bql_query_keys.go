package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/bql"
)

// handleBQLQueryKeys handles keyboard input when BQL query modal is focused.
// Returns (Model, tea.Cmd) — distinct from other handlers because the modal
// owns asynchronous filter execution.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.BQLQuery lands in bt-ift6.9.
func (m Model) handleBQLQueryKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
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

	case "esc":
		m.closeModal()
		m.focused = focusList
		return m, nil

	case "up":
		m.bqlQuery.HistoryPrev()
		return m, nil

	case "down":
		m.bqlQuery.HistoryNext()
		return m, nil

	default:
		var cmd tea.Cmd
		m.bqlQuery, cmd = m.bqlQuery.Update(msg)
		return m, cmd
	}
}
