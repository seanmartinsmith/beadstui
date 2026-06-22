package ui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// handleBoardKeys handles keyboard input when the board is focused.
//
// Dispatches via key.Matches against m.keys.BoardNormal or
// m.keys.BoardSearch depending on m.board.IsSearchMode(), per ADR-004
// Decision 7. The dispatcher in model_update_input.go short-circuits to this
// handler before global view-switch keys when IsSearchMode() is true, so typed
// letters reach board.AppendSearchChar instead of firing global hotkeys (the
// letter-leak class fixed in bt-s2xpy; originally flagged in bt-ift6.3).
//
// gg-combo (IsWaitingForG) remains a conditional inside the normal-mode
// branch -- single keystroke, not a dwellable sub-state, per Decision 7.
func (m Model) handleBoardKeys(msg tea.KeyMsg) Model {
	// ===========================================================================
	// Search mode input handling (bv-yg39)
	// ===========================================================================
	if m.board.IsSearchMode() {
		ks := m.keys.BoardSearch
		switch {
		case key.Matches(msg, ks.Cancel):
			m.board.CancelSearch()
			// Disable n/N in normal mode until next search-start
			m.keys.BoardNormal.NextMatch.SetEnabled(false)
			m.keys.BoardNormal.PrevMatch.SetEnabled(false)
		case key.Matches(msg, ks.Finish):
			// Keep search results but exit search mode; n/N remain enabled
			m.board.FinishSearch()
		case key.Matches(msg, ks.Backspace):
			m.board.BackspaceSearch()
		case key.Matches(msg, ks.NextMatch):
			m.board.NextMatch()
		case key.Matches(msg, ks.PrevMatch):
			m.board.PrevMatch()
		default:
			// Append printable characters to search query
			k := msg.String()
			if len(k) == 1 {
				m.board.AppendSearchChar(rune(k[0]))
			}
		}
		return m
	}

	// ===========================================================================
	// Vim 'gg' combo handling (bv-yg39)
	// ===========================================================================
	kn := m.keys.BoardNormal
	if m.board.IsWaitingForG() {
		m.board.ClearWaitingForG()
		if key.Matches(msg, kn.GotoTop) {
			m.board.MoveToTop()
			return m
		}
		// Not a second 'g', fall through to normal handling
	}

	// ===========================================================================
	// Normal key handling (bv-yg39 enhanced)
	// ===========================================================================
	switch {
	// Column nav
	case key.Matches(msg, kn.Left):
		m.board.MoveLeft()
	case key.Matches(msg, kn.Right):
		m.board.MoveRight()

	// Item nav
	case key.Matches(msg, kn.Down):
		m.board.MoveDown()
	case key.Matches(msg, kn.Up):
		m.board.MoveUp()
	case key.Matches(msg, kn.JumpTop):
		m.board.MoveToTop()
	case key.Matches(msg, kn.JumpBottom):
		m.board.MoveToBottom()
	case key.Matches(msg, kn.PageDown):
		m.board.PageDown(m.height / 3)
	case key.Matches(msg, kn.PageUp):
		m.board.PageUp(m.height / 3)

	// Column jumping
	case key.Matches(msg, kn.JumpFirst):
		m.board.JumpToFirstColumn()
	case key.Matches(msg, kn.JumpLast):
		m.board.JumpToLastColumn()
	case key.Matches(msg, kn.JumpCol1):
		m.board.JumpToColumn(ColOpen)
	case key.Matches(msg, kn.JumpCol2):
		m.board.JumpToColumn(ColInProgress)
	case key.Matches(msg, kn.JumpCol3):
		m.board.JumpToColumn(ColBlocked)
	case key.Matches(msg, kn.JumpCol4):
		m.board.JumpToColumn(ColClosed)

	// Vim-style positional nav
	case key.Matches(msg, kn.GotoTop):
		// First 'g' -- wait for second to complete gg combo
		m.board.SetWaitingForG()
	case key.Matches(msg, kn.JumpColEnd):
		m.board.MoveToBottom()

	// Search
	case key.Matches(msg, kn.Search):
		m.board.StartSearch()
		// Enable n/N now that a search is active
		m.keys.BoardNormal.NextMatch.SetEnabled(true)
		m.keys.BoardNormal.PrevMatch.SetEnabled(true)

	// Search navigation when not in search mode
	case key.Matches(msg, kn.NextMatch):
		if m.board.SearchMatchCount() > 0 {
			m.board.NextMatch()
		}
	case key.Matches(msg, kn.PrevMatch):
		if m.board.SearchMatchCount() > 0 {
			m.board.PrevMatch()
		}

	// Copy ID to clipboard (bv-yg39)
	case key.Matches(msg, kn.CopyID):
		if selected := m.board.SelectedIssue(); selected != nil {
			if err := clipboard.WriteAll(selected.ID); err != nil {
				m.setNotice(fmt.Sprintf("Clipboard error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Copied %s to clipboard", selected.ID))
			}
		}

	// Global filter keys (bv-naov) - toggle: press again to revert to all.
	// Note: o/c/r shadow GlobalKeys candidates in board context; the
	// board-search dispatcher guard ensures these only run in normal mode.
	case key.Matches(msg, m.keys.ListNormal.FilterOpen):
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "open" {
			m.filter.currentFilter = "all"
			m.setStatus("Filter: All issues")
		} else {
			m.filter.currentFilter = "open"
			m.setStatus("Filter: Open issues")
		}
		m.applyFilter()
	case key.Matches(msg, m.keys.ListNormal.FilterClosed):
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "closed" {
			m.filter.currentFilter = "all"
			m.setStatus("Filter: All issues")
		} else {
			m.filter.currentFilter = "closed"
			m.setStatus("Filter: Closed issues")
		}
		m.applyFilter()
	case key.Matches(msg, m.keys.ListNormal.FilterReady):
		m.filter.activeBQLExpr = nil
		if m.filter.currentFilter == "ready" {
			m.filter.currentFilter = "all"
			m.setStatus("Filter: All issues")
		} else {
			m.filter.currentFilter = "ready"
			m.setStatus("Filter: Ready (no blockers)")
		}
		m.applyFilter()

	// Swimlane mode cycling (bv-wjs0)
	case key.Matches(msg, kn.CycleSwim):
		m.board.CycleSwimLaneMode()
		modeName := m.board.GetSwimLaneModeName()
		m.setStatus(fmt.Sprintf("Swimlane: %s", modeName))

	// Empty column visibility toggle (bv-tf6j)
	case key.Matches(msg, kn.ToggleEmpty):
		m.board.ToggleEmptyColumns()
		visMode := m.board.GetEmptyColumnVisibilityMode()
		hidden := m.board.HiddenColumnCount()
		if hidden > 0 {
			m.setStatus(fmt.Sprintf("Empty columns: %s (%d hidden)", visMode, hidden))
		} else {
			m.setStatus(fmt.Sprintf("Empty columns: %s", visMode))
		}

	// Inline card expansion (bv-i3ii)
	case key.Matches(msg, kn.ToggleExpand):
		m.board.ToggleExpand()
		if m.board.HasExpandedCard() {
			m.setStatus("Card expanded (d=collapse, j/k=auto-collapse)")
		} else {
			m.setStatus("Card collapsed")
		}

	// Detail panel (bv-r6kh)
	case key.Matches(msg, kn.DetailToggle):
		m.board.ToggleDetail()
	case key.Matches(msg, kn.DetailDown):
		if m.board.IsDetailShown() {
			m.board.DetailScrollDown(3)
		}
	case key.Matches(msg, kn.DetailUp):
		if m.board.IsDetailShown() {
			m.board.DetailScrollUp(3)
		}

	// Exit to detail view
	case key.Matches(msg, kn.Enter):
		if selected := m.board.SelectedIssue(); selected != nil {
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
	}
	return m
}
