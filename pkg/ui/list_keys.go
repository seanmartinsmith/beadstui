package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// handleListKeys handles keyboard input when the list is focused.
//
// Body otherwise unchanged from the pre-bt-ift6.1 model_keys.go split, with
// one targeted exception: the "tab", "<", ">" cases were moved INTO this
// handler from the global dispatcher per ADR-004 Decision 1's "no
// match-and-fall-through" rule. Each view that wants split-view detail
// toggle / resize declares its own bindings; the prior global cases are
// removed from model_update_input.go's dispatcher in the same change.
//
// Conversion to dispatch via key.Matches against m.keys.List (split into
// ListNormalKeys + ListSearchKeys per ADR-004 Decision 7) lands in
// bt-ift6.2 (the spine).
func (m Model) handleListKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "enter":
		if !m.isSplitView {
			m.showDetails = true
			m.focused = focusDetail
			m.viewport.GotoTop() // Reset scroll position for new issue
			m.updateViewportContent()
		}
	case "home":
		m.list.Select(0)
	case "G", "end":
		if len(m.list.Items()) > 0 {
			m.list.Select(len(m.list.Items()) - 1)
		}
	case "ctrl+d":
		// Page down
		itemCount := len(m.list.Items())
		if itemCount > 0 {
			currentIdx := m.list.Index()
			newIdx := currentIdx + m.height/3
			if newIdx >= itemCount {
				newIdx = itemCount - 1
			}
			m.list.Select(newIdx)
		}
	case "ctrl+u":
		// Page up
		if len(m.list.Items()) > 0 {
			currentIdx := m.list.Index()
			newIdx := currentIdx - m.height/3
			if newIdx < 0 {
				newIdx = 0
			}
			m.list.Select(newIdx)
		}
	case "o":
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "open" {
			m.filter.currentFilter = "all"
			m.setStatus("Filter: All issues")
		} else {
			m.filter.currentFilter = "open"
			m.setStatus("Filter: Open issues")
		}
		m.applyFilter()
	case "c":
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "closed" {
			m.filter.currentFilter = "all"
			m.setStatus("Filter: All issues")
		} else {
			m.filter.currentFilter = "closed"
			m.setStatus("Filter: Closed issues")
		}
		m.applyFilter()
	case "r":
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "ready" {
			m.filter.currentFilter = "all"
			m.setStatus("Filter: All issues")
		} else {
			m.filter.currentFilter = "ready"
			m.setStatus("Filter: Ready (no blockers)")
		}
		m.applyFilter()
	case "a":
		m.filter.activeBQLExpr = nil
		m.filter.currentFilter = "all"
		m.applyFilter()
	case "t":
		// Toggle time-travel mode off, or show prompt for custom revision
		if m.timeTravelMode {
			m.exitTimeTravelMode()
		} else {
			// Show input prompt for revision
			m.openModal(ModalTimeTravelInput)
			m.timeTravelInput.SetValue("")
			m.timeTravelInput.Focus()
			m.focused = focusTimeTravelInput
		}
	case "T":
		// Quick time-travel with default HEAD~5
		if m.timeTravelMode {
			m.exitTimeTravelMode()
		} else {
			m.enterTimeTravelMode("HEAD~5")
		}
	case "C":
		// Copy selected issue to clipboard
		m.copyIssueToClipboard()
	case "O":
		// Open beads.jsonl in editor
		m.openInEditor()
	// History view (h) is handled exclusively by the global key router in
	// model_update_input.go so it can return the async LoadHistoryCmd into
	// the tea.Batch (bt-uizm). handleListKeys cannot return a tea.Cmd, so
	// no-op duplicate here.
	case "R":
		// Apply triage recipe - sort by triage score (bt-ktcr: moved from S to free S for reverse sort)
		if r := m.filter.recipeLoader.Get("triage"); r != nil {
			m.setActiveRecipe(r)
			m.applyRecipe(r)
		}
	case "S":
		// Cycle sort mode reverse (bt-ktcr: matches alerts-modal s/S forward/reverse convention)
		m.cycleSortModeReverse()
	case "s":
		// Cycle sort mode (bv-3ita)
		m.cycleSortMode()
	case "V":
		// Show cass session preview modal (bv-5bqh)
		m.showCassSessionModal()
	case "U":
		// Show self-update modal (bv-182)
		m.showSelfUpdateModal()
	case "y":
		// Copy ID to clipboard (consistent with board view - bv-yg39)
		selectedItem := m.list.SelectedItem()
		if selectedItem == nil {
			m.setStatusError("No issue selected")
		} else if issueItem, ok := selectedItem.(IssueItem); ok {
			if err := clipboard.WriteAll(issueItem.Issue.ID); err != nil {
				m.setStatusError(fmt.Sprintf("Clipboard error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Copied %s to clipboard", issueItem.Issue.ID))
			}
		}
	case "tab":
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
	case "<":
		// Shrink list pane (move divider left). Moved from the global
		// dispatcher per ADR-004 Decision 1.
		if m.isSplitView {
			m.splitPaneRatio -= 0.05
			if m.splitPaneRatio < 0.2 {
				m.splitPaneRatio = 0.2
			}
			m.recalculateSplitPaneSizes()
		}
	case ">":
		// Expand list pane (move divider right). Moved from the global
		// dispatcher per ADR-004 Decision 1.
		if m.isSplitView {
			m.splitPaneRatio += 0.05
			if m.splitPaneRatio > 0.8 {
				m.splitPaneRatio = 0.8
			}
			m.recalculateSplitPaneSizes()
		}
	}
	return m
}
