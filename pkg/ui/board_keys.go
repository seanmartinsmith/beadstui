package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// handleBoardKeys handles keyboard input when the board is focused (bv-yg39).
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.Board (split into BoardNormalKeys
// + BoardSearchKeys per ADR-004 Decision 7) lands in bt-ift6.3.
func (m Model) handleBoardKeys(msg tea.KeyMsg) Model {
	key := msg.String()

	// ═══════════════════════════════════════════════════════════════════════════
	// Search mode input handling (bv-yg39)
	// ═══════════════════════════════════════════════════════════════════════════
	if m.board.IsSearchMode() {
		switch key {
		case "esc":
			m.board.CancelSearch()
		case "enter":
			// Keep search results but exit search mode
			m.board.FinishSearch()
		case "backspace":
			m.board.BackspaceSearch()
		case "n":
			m.board.NextMatch()
		case "N":
			m.board.PrevMatch()
		default:
			// Append printable characters to search query
			if len(key) == 1 {
				m.board.AppendSearchChar(rune(key[0]))
			}
		}
		return m
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// Vim 'gg' combo handling (bv-yg39)
	// ═══════════════════════════════════════════════════════════════════════════
	if m.board.IsWaitingForG() {
		m.board.ClearWaitingForG()
		if key == "g" {
			m.board.MoveToTop()
			return m
		}
		// Not a second 'g', fall through to normal handling
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// Normal key handling (bv-yg39 enhanced)
	// ═══════════════════════════════════════════════════════════════════════════
	switch key {
	// Basic navigation (existing)
	case "h", "left":
		m.board.MoveLeft()
	case "l", "right":
		m.board.MoveRight()
	case "j", "down":
		m.board.MoveDown()
	case "k", "up":
		m.board.MoveUp()
	case "home":
		m.board.MoveToTop()
	case "G", "end":
		m.board.MoveToBottom()
	case "ctrl+d":
		m.board.PageDown(m.height / 3)
	case "ctrl+u":
		m.board.PageUp(m.height / 3)

	// Column jumping (bv-yg39)
	case "1":
		m.board.JumpToColumn(ColOpen)
	case "2":
		m.board.JumpToColumn(ColInProgress)
	case "3":
		m.board.JumpToColumn(ColBlocked)
	case "4":
		m.board.JumpToColumn(ColClosed)
	case "H":
		m.board.JumpToFirstColumn()
	case "L":
		m.board.JumpToLastColumn()

	// Vim-style navigation (bv-yg39)
	case "g":
		m.board.SetWaitingForG() // Wait for second 'g'
	case "0":
		m.board.MoveToTop() // First item in column
	case "$":
		m.board.MoveToBottom() // Last item in column

	// Search (bv-yg39)
	case "/":
		m.board.StartSearch()

	// Search navigation when not in search mode (bv-yg39)
	case "n":
		if m.board.SearchMatchCount() > 0 {
			m.board.NextMatch()
		}
	case "N":
		if m.board.SearchMatchCount() > 0 {
			m.board.PrevMatch()
		}

	// Copy ID to clipboard (bv-yg39)
	case "y":
		if selected := m.board.SelectedIssue(); selected != nil {
			if err := clipboard.WriteAll(selected.ID); err != nil {
				m.setStatusError(fmt.Sprintf("Clipboard error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Copied %s to clipboard", selected.ID))
			}
		}

	// Global filter keys (bv-naov) - toggle: press again to revert to all
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

	// Swimlane mode cycling (bv-wjs0)
	case "s":
		m.board.CycleSwimLaneMode()
		modeName := m.board.GetSwimLaneModeName()
		m.setStatus(fmt.Sprintf("🔀 Swimlane: %s", modeName))

	// Empty column visibility toggle (bv-tf6j)
	case "e":
		m.board.ToggleEmptyColumns()
		visMode := m.board.GetEmptyColumnVisibilityMode()
		hidden := m.board.HiddenColumnCount()
		if hidden > 0 {
			m.setStatus(fmt.Sprintf("👁 Empty columns: %s (%d hidden)", visMode, hidden))
		} else {
			m.setStatus(fmt.Sprintf("👁 Empty columns: %s", visMode))
		}

	// Inline card expansion (bv-i3ii)
	case "d":
		m.board.ToggleExpand()
		if m.board.HasExpandedCard() {
			m.setStatus("📋 Card expanded (d=collapse, j/k=auto-collapse)")
		} else {
			m.setStatus("📋 Card collapsed")
		}

	// Detail panel (bv-r6kh)
	case "tab":
		m.board.ToggleDetail()
	case "ctrl+j":
		if m.board.IsDetailShown() {
			m.board.DetailScrollDown(3)
		}
	case "ctrl+k":
		if m.board.IsDetailShown() {
			m.board.DetailScrollUp(3)
		}

	// Exit to detail view
	case "enter":
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
