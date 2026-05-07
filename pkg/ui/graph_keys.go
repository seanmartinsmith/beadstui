package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// handleGraphKeys handles keyboard input when the graph view is focused.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.Graph lands in bt-ift6.4.
func (m Model) handleGraphKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "h", "left":
		m.graphView.MoveLeft()
	case "l", "right":
		m.graphView.MoveRight()
	case "j", "down":
		m.graphView.MoveDown()
	case "k", "up":
		m.graphView.MoveUp()
	case "ctrl+d", "pgdown":
		m.graphView.PageDown()
	case "ctrl+u", "pgup":
		m.graphView.PageUp()
	case "enter":
		if selected := m.graphView.SelectedIssue(); selected != nil {
			// Find and select in list
			for i, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selected.ID {
					m.list.Select(i)
					break
				}
			}
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
	case "s":
		// bt-1knw: toggle swarm wave visualization
		g := &m.graphView
		if g.swarmEnabled {
			g.clearSwarmData()
			m.setStatus("swarm: off")
		} else {
			selected := g.SelectedIssue()
			if selected == nil {
				m.setStatusError("select an issue to enable swarm view")
			} else {
				epicID := selected.ID
				// If not an epic, look for an epic parent in dependents
				if selected.IssueType != model.TypeEpic {
					for _, depID := range g.dependents[selected.ID] {
						if dep, ok := g.issueMap[depID]; ok && dep.IssueType == model.TypeEpic {
							epicID = depID
							break
						}
					}
				}
				if err := g.loadSwarmData(epicID); err != nil {
					m.setStatusError(fmt.Sprintf("swarm: %v", err))
				} else {
					g.swarmEnabled = true
					m.setStatus(fmt.Sprintf("swarm: %s (∥%d, ~%d sessions)", epicID, g.maxParallel, g.estSessions))
				}
			}
		}
	}
	return m
}
