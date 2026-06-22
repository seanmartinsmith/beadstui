package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// handleGraphKeys handles keyboard input when the graph view is focused.
//
// Dispatches via key.Matches against m.keys.Graph per ADR-004 Decision 1.
// Converted from switch msg.String() in bt-ift6.4.
func (m Model) handleGraphKeys(msg tea.KeyMsg) Model {
	k := m.keys.Graph
	switch {
	case key.Matches(msg, k.MoveLeft):
		m.graphView.MoveLeft()
	case key.Matches(msg, k.MoveRight):
		m.graphView.MoveRight()
	case key.Matches(msg, k.Down):
		m.graphView.MoveDown()
	case key.Matches(msg, k.Up):
		m.graphView.MoveUp()
	case key.Matches(msg, k.PageDown):
		m.graphView.PageDown()
	case key.Matches(msg, k.PageUp):
		m.graphView.PageUp()
	case key.Matches(msg, k.JumpToIssue):
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
	case key.Matches(msg, k.SwarmToggle):
		// bt-1knw: toggle swarm wave visualization
		g := &m.graphView
		if g.swarmEnabled {
			g.clearSwarmData()
			m.setStatus("swarm: off")
		} else {
			selected := g.SelectedIssue()
			if selected == nil {
				m.setNotice("select an issue to enable swarm view")
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
					m.setFailure(fmt.Sprintf("swarm: %v", err))
				} else {
					g.swarmEnabled = true
					m.setStatus(fmt.Sprintf("swarm: %s (∥%d, ~%d sessions)", epicID, g.maxParallel, g.estSessions))
				}
			}
		}
	}
	return m
}
