package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// handleListKeys handles keyboard input when the main list is focused and
// not in filter-typing mode (the dispatcher's filter-state guard at
// model_update_input.go:822 prevents this from running while
// m.list.FilterState() == list.Filtering).
//
// Dispatches via key.Matches against m.keys.ListNormal per ADR-004
// Decision 1. ListSearchKeys is the help-only sibling Map for filter mode
// — bubbles list owns dispatch there, so this handler does not consult it
// (Decision 7).
//
// History (h) is intentionally absent — it lives on GlobalKeys.History
// because handleListKeys cannot return tea.Cmd and the history switch
// dispatches an async LoadHistoryCmd (bt-uizm).
func (m Model) handleListKeys(msg tea.KeyMsg) Model {
	k := m.keys.ListNormal
	switch {
	case key.Matches(msg, k.Enter):
		if !m.isSplitView {
			m.showDetails = true
			m.focused = focusDetail
			m.viewport.GotoTop() // Reset scroll position for new issue
			m.updateViewportContent()
		}
	case key.Matches(msg, k.EpicCard):
		// Open the tier-2 focus card when the cursor is on an epic; on a
		// non-epic it's a no-op with a hint (bt-gfxhz.3).
		if item, ok := m.list.SelectedItem().(IssueItem); ok {
			if item.Issue.IssueType == model.TypeEpic {
				m.openEpicCard(item.Issue.ID)
			} else {
				m.setStatus("Not an epic")
			}
		}
	case key.Matches(msg, k.JumpTop):
		m.list.Select(0)
	case key.Matches(msg, k.JumpBottom):
		if len(m.list.Items()) > 0 {
			m.list.Select(len(m.list.Items()) - 1)
		}
	case key.Matches(msg, k.PageDown):
		itemCount := len(m.list.Items())
		if itemCount > 0 {
			currentIdx := m.list.Index()
			newIdx := currentIdx + m.height/3
			if newIdx >= itemCount {
				newIdx = itemCount - 1
			}
			m.list.Select(newIdx)
		}
	case key.Matches(msg, k.PageUp):
		if len(m.list.Items()) > 0 {
			currentIdx := m.list.Index()
			newIdx := currentIdx - m.height/3
			if newIdx < 0 {
				newIdx = 0
			}
			m.list.Select(newIdx)
		}
	// o/c/r toggle a single status; # cycles the full status set. None emit a
	// "Filter: X" toast anymore — the footer lens status chip already shows the
	// active filter, so an echo would duplicate it (bt-2vshd).
	case key.Matches(msg, k.FilterOpen):
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "open" {
			m.filter.currentFilter = "all"
		} else {
			m.filter.currentFilter = "open"
		}
		m.applyFilter()
	case key.Matches(msg, k.FilterClosed):
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "closed" {
			m.filter.currentFilter = "all"
		} else {
			m.filter.currentFilter = "closed"
		}
		m.applyFilter()
	case key.Matches(msg, k.FilterReady):
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "ready" {
			m.filter.currentFilter = "all"
		} else {
			m.filter.currentFilter = "ready"
		}
		m.applyFilter()
	case key.Matches(msg, k.CycleStatusFilter):
		m.cycleStatusFilter()
	// No FilterAll case: 'a' is GlobalKeys.Actionable and shadowed by the
	// global view-switch before this handler runs. The pre-.2 `case "a"`
	// here was dead. Reset-to-all is reachable via toggling the active
	// filter key (o/c/r each toggle to "all" on second press).
	case key.Matches(msg, k.TimeTravelInput):
		// Toggle time-travel mode off, or show prompt for custom revision
		if m.timeTravelMode {
			m.exitTimeTravelMode()
		} else {
			m.openModal(ModalTimeTravelInput)
			m.timeTravelInput.SetValue("")
			m.timeTravelInput.Focus()
			m.focused = focusTimeTravelInput
		}
	case key.Matches(msg, k.CopyIssue):
		m.copyIssueToClipboard()
	case key.Matches(msg, k.OpenInEditor):
		m.openInEditor()
	case key.Matches(msg, k.RecipeTriage):
		// Apply triage recipe - sort by triage score (bt-ktcr: moved from S to
		// free S for reverse sort). Re-press toggles off (bt-wfug): if triage
		// is already active, clear the recipe and re-apply the non-recipe
		// filter so the user returns to where they were.
		if m.filter.activeRecipe != nil && m.filter.activeRecipe.Name == "triage" {
			m.setActiveRecipe(nil)
			m.applyFilter()
			m.setStatus("Triage recipe cleared")
		} else if r := m.filter.recipeLoader.Get("triage"); r != nil {
			m.setActiveRecipe(r)
			m.applyRecipe(r)
		}
	case key.Matches(msg, k.CycleSortReverse):
		// Cycle sort mode reverse (bt-ktcr: matches alerts-modal s/S forward/reverse convention)
		m.cycleSortModeReverse()
	case key.Matches(msg, k.CycleSort):
		m.cycleSortMode()
	case key.Matches(msg, k.CassSession):
		m.showCassSessionModal()
	case key.Matches(msg, k.SelfUpdate):
		m.showSelfUpdateModal()
	case key.Matches(msg, k.Claim):
		// First bt write (bt-oiaj.10): open the confirm modal for the selected
		// bead. The confirm's accept path fires the claim (handleKeyPress).
		m.requestClaim()
	case key.Matches(msg, k.FieldEdit):
		// Field edits (bt-oiaj.5): open the field-select modal for the
		// selected bead. Mirrors requestClaim's guard shape (no selection /
		// already-pending refusal).
		m.requestFieldEdit()
	case key.Matches(msg, k.CopyID):
		// Copy ID to clipboard (consistent with board view - bv-yg39)
		selectedItem := m.list.SelectedItem()
		if selectedItem == nil {
			m.setNotice("No issue selected")
		} else if issueItem, ok := selectedItem.(IssueItem); ok {
			id := issueIDForClipboard(issueItem)
			if err := clipboard.WriteAll(id); err != nil {
				m.setNotice(fmt.Sprintf("Clipboard error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Copied %s to clipboard", id))
			}
		}
	case key.Matches(msg, k.SplitFocusToggle):
		// Split-view focus toggle. Moved from the global dispatcher per
		// ADR-004 Decision 1 (no match-and-fall-through). Active in
		// split-view ViewList only; no-op otherwise.
		if m.isSplitView && m.mode == ViewList {
			if m.focused == focusList {
				m.focused = focusDetail
			} else {
				m.focused = focusList
			}
		}
	case key.Matches(msg, k.SplitShrinkLeft):
		// Shrink list pane (move divider left). Moved from the global
		// dispatcher per ADR-004 Decision 1.
		if m.isSplitView {
			m.splitPaneRatio -= 0.05
			if m.splitPaneRatio < 0.2 {
				m.splitPaneRatio = 0.2
			}
			m.recalculateSplitPaneSizes()
		}
	case key.Matches(msg, k.SplitShrinkRight):
		// Expand list pane (move divider right). Moved from the global
		// dispatcher per ADR-004 Decision 1.
		if m.isSplitView {
			m.splitPaneRatio += 0.05
			if m.splitPaneRatio > 0.8 {
				m.splitPaneRatio = 0.8
			}
			m.recalculateSplitPaneSizes()
		}
	case key.Matches(msg, k.PaneFullscreenIssues):
		// Two-stage per-pane focus/fullscreen (bt-566fk, refining bt-530vn):
		// in split view first press focuses the pane, second maximizes it,
		// third exits; at narrow width it's a direct btop-style toggle. Esc/q
		// also exit (dispatcher cascade, model_update_input.go).
		m.toggleFullscreenPane(fullscreenIssues)
	case key.Matches(msg, k.PaneFullscreenDetails):
		m.toggleFullscreenPane(fullscreenDetails)
	}
	return m
}

func issueIDForClipboard(item IssueItem) string {
	return item.Issue.ID
}
