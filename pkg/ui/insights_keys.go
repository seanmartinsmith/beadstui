package ui

import tea "charm.land/bubbletea/v2"

// handleInsightsKeys handles keyboard input when insights panel is focused.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.Insights lands in bt-ift6.5.
func (m Model) handleInsightsKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "esc":
		m.focused = focusList
	case "j", "down":
		m.insightsPanel.MoveDown()
	case "k", "up":
		m.insightsPanel.MoveUp()
	case "ctrl+j":
		// Scroll detail panel down
		m.insightsPanel.ScrollDetailDown()
	case "ctrl+k":
		// Scroll detail panel up
		m.insightsPanel.ScrollDetailUp()
	case "h", "left":
		m.insightsPanel.PrevPanel()
	case "l", "right", "tab":
		m.insightsPanel.NextPanel()
	case "e":
		// Toggle explanations
		m.insightsPanel.ToggleExplanations()
	case "x":
		// Toggle calculation details
		m.insightsPanel.ToggleCalculation()
	case "m":
		// Toggle heatmap view (bv-95) - "m" for heatMap
		m.insightsPanel.ToggleHeatmap()
	case "enter":
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
