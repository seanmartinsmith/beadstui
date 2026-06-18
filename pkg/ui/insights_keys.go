package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// handleInsightsKeys handles keyboard input when insights panel is focused.
//
// Dispatches via key.Matches against m.keys.Insights per ADR-004 Decision 1
// (bt-ift6.5). No switch msg.String() remains for keybind matching.
func (m Model) handleInsightsKeys(msg tea.KeyMsg) Model {
	k := m.keys.Insights
	switch {
	case key.Matches(msg, k.Exit):
		m.focused = focusList
	case key.Matches(msg, k.Down):
		m.insightsPanel.MoveDown()
	case key.Matches(msg, k.Up):
		m.insightsPanel.MoveUp()
	case key.Matches(msg, k.ScrollDetailDown):
		// Scroll detail panel down
		m.insightsPanel.ScrollDetailDown()
	case key.Matches(msg, k.ScrollDetailUp):
		// Scroll detail panel up
		m.insightsPanel.ScrollDetailUp()
	case key.Matches(msg, k.PrevPanel):
		m.insightsPanel.PrevPanel()
	case key.Matches(msg, k.NextPanel):
		m.insightsPanel.NextPanel()
	case key.Matches(msg, k.Explanations):
		// Toggle explanations
		m.insightsPanel.ToggleExplanations()
	case key.Matches(msg, k.Calculation):
		// Toggle calculation details
		m.insightsPanel.ToggleCalculation()
	case key.Matches(msg, k.Heatmap):
		// Toggle heatmap view (bv-95) - "m" for heatMap
		m.insightsPanel.ToggleHeatmap()
	case key.Matches(msg, k.JumpToIssue):
		// Jump to selected issue in list view
		selectedID := m.insightsPanel.SelectedIssueID()
		if selectedID != "" {
			for i, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
			// Capture insights cursor before leaving so the next `i` toggle
			// returns to the same pane and row (bt-fdwz).
			panel := m.insightsPanel.FocusedPanel()
			m.insightsCursor = insightsCursor{
				panel: panel,
				index: m.insightsPanel.SelectedIndexFor(panel),
				valid: true,
			}
			// Leave insights mode (bt-fdwz fix 1): without this, the user
			// stays on the insights view despite the list cursor jumping,
			// requiring a second `i` keypress to actually see the bead.
			m.mode = ViewList
			m.focused = focusList
			if m.isSplitView {
				m.focused = focusDetail
			} else {
				m.showDetails = true
				m.focused = focusDetail
				m.viewport.GotoTop()
			}
			m.updateViewportContent()
		}
	}
	return m
}
