package ui

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// refreshEpicsForCurrentFilter rebuilds the epics tree from the current scope +
// label filter and re-renders the cached view text. It is the epics analog of
// refreshBoardAndGraphForCurrentFilter: called on view-switch, on filter/recipe
// change, on data reload, and on resize (only while ViewEpics is active).
//
// Sourcing note (the one place the "projection over filteredIssuesForActiveView"
// rule is overridden): the progress bars must count closed children in full, but
// the list's status filter (open/ready) drops closed issues. So we source from
// the scope + label + wisp-filtered set WITHOUT the status filter, and let
// m.epicsStatusMode (active/all/completed) decide which epics to list. The
// EpicsTreeModel.Build pipeline handles the rest. See the epics-tree-redesign
// design's "status filter override".
func (m *Model) refreshEpicsForCurrentFilter() {
	if m.mode != ViewEpics {
		return
	}

	issues := m.workspacePrefilter(m.data.issues)
	scoped := make([]model.Issue, 0, len(issues))
	for _, issue := range issues {
		// Skip wisps when hidden (bt-9kdo), mirroring filteredIssuesForActiveView.
		if !m.showWisps && issue.Ephemeral != nil && *issue.Ephemeral {
			continue
		}
		if m.filter.labelFilter != "" && !matchesLabelFilter(issue, m.filter.labelFilter) {
			continue
		}
		scoped = append(scoped, issue)
	}

	m.epicsTree.Build(scoped, m.epicsStatusMode, time.Now())
	m.epicsTree.SetTheme(m.theme)
	m.epicsTree.SetContext(m.epicsScopeLabel(), m.epicsStatusMode.label())
	m.epicsTree.SetSize(m.bodyWidth(), m.height-1)
	m.epicsViewText = m.epicsTree.View()
}

// epicsScopeLabel is the short scope descriptor shown in the tree header
// (EPICS . <scope> . <mode>): the active label filter when set, else the
// workspace/cross-project scope.
func (m Model) epicsScopeLabel() string {
	if m.filter.labelFilter != "" {
		return m.filter.labelFilter
	}
	if m.workspaceMode {
		return "workspace"
	}
	return "cross-project"
}

// handleEpicsKeys handles keyboard input when the epics tree is focused.
// Dispatches via key.Matches against m.keys.Epics per ADR-004 Decision 1.
// (Expand/collapse/drill/zoom bindings are added in bt-3ftfm.1 Task 5.)
func (m Model) handleEpicsKeys(msg tea.KeyMsg) Model {
	k := m.keys.Epics
	switch {
	case key.Matches(msg, k.Down):
		m.epicsTree.moveCursor(1)
		m.epicsViewText = m.epicsTree.View()
	case key.Matches(msg, k.Up):
		m.epicsTree.moveCursor(-1)
		m.epicsViewText = m.epicsTree.View()
	case key.Matches(msg, k.CycleStatus):
		m.epicsStatusMode = m.epicsStatusMode.next()
		m.epicsTree.cursor = 0
		m.refreshEpicsForCurrentFilter()
	case key.Matches(msg, k.Open):
		// Transitional: open the tier-2 epic focus card for the cursor's epic.
		if r, ok := m.epicsTree.cursorRow(); ok && r.kind == rowEpic && r.issue != nil {
			m.openEpicCard(r.issue.ID)
		}
	case key.Matches(msg, k.Exit):
		m.mode = ViewList
		m.focused = focusList
	}
	return m
}
