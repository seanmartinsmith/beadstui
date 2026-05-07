package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// handleHistoryKeys handles keyboard input when history view is focused.
//
// Body unchanged from the pre-bt-ift6.1 model_keys.go split. Conversion to
// dispatch via key.Matches against m.keys.History (split into HistoryNormalKeys
// + HistorySearchKeys + HistoryFileTreeKeys per ADR-004 Decision 7) lands in
// bt-ift6.6.
func (m Model) handleHistoryKeys(msg tea.KeyMsg) Model {
	// Handle search input when active (bv-nkrj)
	if m.historyView.IsSearchActive() {
		switch msg.String() {
		case "esc":
			m.historyView.CancelSearch()
			m.setStatus("🔍 Search cancelled")
			return m
		case "enter":
			// Confirm search (just blur input, keep filter active)
			m.historyView.CancelSearch() // For now, just close search
			return m
		default:
			// Forward to search input
			m.historyView.UpdateSearchInput(msg)
			query := m.historyView.SearchQuery()
			if query != "" {
				m.setStatus(fmt.Sprintf("🔍 Filtering: %s", query))
			} else {
				m.setStatus("🔍 Type to search...")
			}
			return m
		}
	}

	// Handle file tree navigation when file tree has focus (bv-190l)
	if m.historyView.FileTreeHasFocus() {
		switch msg.String() {
		case "j", "down":
			m.historyView.MoveDownFileTree()
			return m
		case "k", "up":
			m.historyView.MoveUpFileTree()
			return m
		case "enter", "l":
			// Expand directory or select file for filtering
			node := m.historyView.SelectedFileNode()
			if node != nil {
				if node.IsDir {
					m.historyView.ToggleExpandFile()
				} else {
					m.historyView.SelectFile()
					name := m.historyView.SelectedFileName()
					m.setStatus(fmt.Sprintf("📁 Filtering by: %s", name))
				}
			}
			return m
		case "h":
			// Collapse directory
			m.historyView.CollapseFileNode()
			return m
		case "esc":
			// If filter is active, clear it; otherwise close file tree
			if m.historyView.GetFileFilter() != "" {
				m.historyView.ClearFileFilter()
				m.setStatus("📁 File filter cleared")
			} else {
				m.historyView.SetFileTreeFocus(false)
				m.setStatus("📁 File tree: press Tab to return focus")
			}
			return m
		case "tab":
			// Switch focus away from file tree
			m.historyView.SetFileTreeFocus(false)
			return m
		}
	}

	switch msg.String() {
	case "/":
		// Start search (bv-nkrj)
		m.historyView.StartSearch()
		m.setStatus("🔍 Type to search commits, beads, authors...")
	case "v":
		// Toggle between Bead mode and Git mode (bv-tl3n)
		m.historyView.ToggleViewMode()
		if m.historyView.IsGitMode() {
			m.setStatus("🔀 Git Mode: commits on left, related beads on right")
		} else {
			m.setStatus("📦 Bead Mode: beads on left, commits on right")
		}
	case "j", "down":
		if m.historyView.IsGitMode() {
			m.historyView.MoveDownGit()
		} else {
			m.historyView.MoveDown()
		}
	case "k", "up":
		if m.historyView.IsGitMode() {
			m.historyView.MoveUpGit()
		} else {
			m.historyView.MoveUp()
		}
	case "J":
		// In git mode: navigate to next related bead; in bead mode: next commit
		if m.historyView.IsGitMode() {
			m.historyView.NextRelatedBead()
		} else {
			m.historyView.NextCommit()
		}
	case "K":
		// In git mode: navigate to prev related bead; in bead mode: prev commit
		if m.historyView.IsGitMode() {
			m.historyView.PrevRelatedBead()
		} else {
			m.historyView.PrevCommit()
		}
	case "ctrl+d", "pgdown":
		// Half-page scroll down on the detail panel (bt-npnh)
		m.historyView.ScrollDetailHalfPageDown()
	case "ctrl+u", "pgup":
		// Half-page scroll up on the detail panel (bt-npnh)
		m.historyView.ScrollDetailHalfPageUp()
	case "tab":
		// Cycle focus: list -> detail -> file tree (if visible) -> list (bv-190l)
		if m.historyView.IsFileTreeVisible() {
			if m.historyView.FileTreeHasFocus() {
				// File tree -> list
				m.historyView.SetFileTreeFocus(false)
			} else if m.historyView.IsDetailFocused() {
				// Detail -> file tree
				m.historyView.SetFileTreeFocus(true)
			} else {
				// List -> detail
				m.historyView.ToggleFocus()
			}
		} else {
			m.historyView.ToggleFocus()
		}
	case "enter":
		// Jump to selected bead in main list
		var selectedID string
		if m.historyView.IsGitMode() {
			selectedID = m.historyView.SelectedRelatedBeadID()
		} else {
			selectedID = m.historyView.SelectedBeadID()
		}
		if selectedID != "" {
			for i, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedID {
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
	case "y":
		// Copy selected commit SHA to clipboard
		var sha, shortSHA string
		if m.historyView.IsGitMode() {
			if commit := m.historyView.SelectedGitCommit(); commit != nil {
				sha = commit.SHA
				shortSHA = commit.ShortSHA
			}
		} else {
			if commit := m.historyView.SelectedCommit(); commit != nil {
				sha = commit.SHA
				shortSHA = commit.ShortSHA
			}
		}
		if sha != "" {
			if err := clipboard.WriteAll(sha); err != nil {
				m.setStatusError(fmt.Sprintf("Clipboard error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Copied %s to clipboard", shortSHA))
			}
		} else {
			m.setStatusError("No commit selected")
		}
	case "c":
		// Cycle confidence threshold (only in bead mode)
		if !m.historyView.IsGitMode() {
			m.historyView.CycleConfidence()
			conf := m.historyView.GetMinConfidence()
			if conf == 0 {
				m.setStatus("🔍 Showing all commits")
			} else {
				m.setStatus(fmt.Sprintf("🔍 Confidence filter: ≥%.0f%%", conf*100))
			}
		}
	case "f", "F":
		// Toggle file tree panel (bv-190l)
		m.historyView.ToggleFileTree()
		if m.historyView.IsFileTreeVisible() {
			m.setStatus("📁 File tree: j/k navigate, Enter select, Esc close")
		} else {
			m.setStatus("📁 File tree hidden")
		}
	case "o":
		// Open commit in browser (bv-xf4p)
		var sha string
		if m.historyView.IsGitMode() {
			if commit := m.historyView.SelectedGitCommit(); commit != nil {
				sha = commit.SHA
			}
		} else {
			if commit := m.historyView.SelectedCommit(); commit != nil {
				sha = commit.SHA
			}
		}
		if sha != "" {
			url := m.getCommitURL(sha)
			if url != "" {
				if err := openBrowserURL(url); err != nil {
					m.setStatusError(fmt.Sprintf("❌ Could not open browser: %v", err))
				} else {
					// Safely truncate SHA for display (bv-xf4p fix)
					shortSHA := sha
					if len(sha) > 7 {
						shortSHA = sha[:7]
					}
					m.setStatus(fmt.Sprintf("🌐 Opened %s in browser", shortSHA))
				}
			} else {
				m.setStatusError("❌ No git remote configured")
			}
		} else {
			m.setStatusError("❌ No commit selected")
		}
	case "g":
		// Jump to graph view for selected bead (bv-xf4p)
		var selectedID string
		if m.historyView.IsGitMode() {
			selectedID = m.historyView.SelectedRelatedBeadID()
		} else {
			selectedID = m.historyView.SelectedBeadID()
		}
		if selectedID != "" {
			// Find and select the bead in the main list
			for i, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
			// Switch to graph view focused on this bead
			m.mode = ViewGraph
			m.graphView.SelectByID(selectedID)
			m.focused = focusGraph
			m.setStatus(fmt.Sprintf("📊 Graph view: %s", selectedID))
		} else {
			m.setStatusError("❌ No bead selected")
		}
	case "h", "esc":
		// Exit history view
		m.mode = ViewList
		m.focused = focusList
	}
	return m
}
