package ui

import (
	"fmt"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/bql"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// handleBoardKeys handles keyboard input when the board is focused (bv-yg39)
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
		m.statusMsg = fmt.Sprintf("🔀 Swimlane: %s", modeName)
		m.statusIsError = false

	// Empty column visibility toggle (bv-tf6j)
	case "e":
		m.board.ToggleEmptyColumns()
		visMode := m.board.GetEmptyColumnVisibilityMode()
		hidden := m.board.HiddenColumnCount()
		if hidden > 0 {
			m.statusMsg = fmt.Sprintf("👁 Empty columns: %s (%d hidden)", visMode, hidden)
		} else {
			m.statusMsg = fmt.Sprintf("👁 Empty columns: %s", visMode)
		}
		m.statusIsError = false

	// Inline card expansion (bv-i3ii)
	case "d":
		m.board.ToggleExpand()
		if m.board.HasExpandedCard() {
			m.statusMsg = "📋 Card expanded (d=collapse, j/k=auto-collapse)"
		} else {
			m.statusMsg = "📋 Card collapsed"
		}
		m.statusIsError = false

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

// handleGraphKeys handles keyboard input when the graph view is focused
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

// handleTreeKeys handles keyboard input when tree view is focused (bv-gllx)
func (m Model) handleTreeKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.tree.MoveDown()
	case "k", "up":
		m.tree.MoveUp()
	case "enter", "space":
		m.tree.ToggleExpand()
	case "h", "left":
		m.tree.CollapseOrJumpToParent()
	case "l", "right":
		m.tree.ExpandOrMoveToChild()
	case "g":
		// Jump to top (vim-style)
		m.tree.JumpToTop()
	case "G":
		m.tree.JumpToBottom()
	case "o":
		m.tree.ExpandAll()
	case "O":
		m.tree.CollapseAll()
	case "ctrl+d", "pgdown":
		m.tree.PageDown()
	case "ctrl+u", "pgup":
		m.tree.PageUp()
	case "E", "esc":
		// Return to list view
		m.focused = focusList
	case "tab":
		// Toggle detail panel (sync selection and jump to detail)
		if m.isSplitView {
			if selected := m.tree.SelectedIssue(); selected != nil {
				// Sync detail panel with tree selection
				for i, item := range m.list.Items() {
					if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selected.ID {
						m.list.Select(i)
						break
					}
				}
				m.updateViewportContent()
				m.focused = focusDetail
			}
		}
	}
	return m
}

// handleActionableKeys handles keyboard input when actionable view is focused
func (m Model) handleActionableKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.actionableView.MoveDown()
	case "k", "up":
		m.actionableView.MoveUp()
	case "enter":
		// Jump to selected issue in list view
		selectedID := m.actionableView.SelectedIssueID()
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
	}
	return m
}

// handleHistoryKeys handles keyboard input when history view is focused
func (m Model) handleHistoryKeys(msg tea.KeyMsg) Model {
	// Handle search input when active (bv-nkrj)
	if m.historyView.IsSearchActive() {
		switch msg.String() {
		case "esc":
			m.historyView.CancelSearch()
			m.statusMsg = "🔍 Search cancelled"
			m.statusIsError = false
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
				m.statusMsg = fmt.Sprintf("🔍 Filtering: %s", query)
			} else {
				m.statusMsg = "🔍 Type to search..."
			}
			m.statusIsError = false
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
					m.statusMsg = fmt.Sprintf("📁 Filtering by: %s", name)
					m.statusIsError = false
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
				m.statusMsg = "📁 File filter cleared"
			} else {
				m.historyView.SetFileTreeFocus(false)
				m.statusMsg = "📁 File tree: press Tab to return focus"
			}
			m.statusIsError = false
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
		m.statusMsg = "🔍 Type to search commits, beads, authors..."
		m.statusIsError = false
	case "v":
		// Toggle between Bead mode and Git mode (bv-tl3n)
		m.historyView.ToggleViewMode()
		if m.historyView.IsGitMode() {
			m.statusMsg = "🔀 Git Mode: commits on left, related beads on right"
		} else {
			m.statusMsg = "📦 Bead Mode: beads on left, commits on right"
		}
		m.statusIsError = false
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
				m.statusMsg = "🔍 Showing all commits"
			} else {
				m.statusMsg = fmt.Sprintf("🔍 Confidence filter: ≥%.0f%%", conf*100)
			}
			m.statusIsError = false
		}
	case "f", "F":
		// Toggle file tree panel (bv-190l)
		m.historyView.ToggleFileTree()
		if m.historyView.IsFileTreeVisible() {
			m.statusMsg = "📁 File tree: j/k navigate, Enter select, Esc close"
		} else {
			m.statusMsg = "📁 File tree hidden"
		}
		m.statusIsError = false
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
					m.statusMsg = fmt.Sprintf("❌ Could not open browser: %v", err)
					m.statusIsError = true
				} else {
					// Safely truncate SHA for display (bv-xf4p fix)
					shortSHA := sha
					if len(sha) > 7 {
						shortSHA = sha[:7]
					}
					m.statusMsg = fmt.Sprintf("🌐 Opened %s in browser", shortSHA)
					m.statusIsError = false
				}
			} else {
				m.statusMsg = "❌ No git remote configured"
				m.statusIsError = true
			}
		} else {
			m.statusMsg = "❌ No commit selected"
			m.statusIsError = true
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
			m.statusMsg = fmt.Sprintf("📊 Graph view: %s", selectedID)
			m.statusIsError = false
		} else {
			m.statusMsg = "❌ No bead selected"
			m.statusIsError = true
		}
	case "h", "esc":
		// Exit history view
		m.mode = ViewList
		m.focused = focusList
	}
	return m
}

// handleFlowMatrixKeys handles keyboard input when flow matrix view is focused
func (m Model) handleFlowMatrixKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "f", "q", "esc":
		// If in drilldown mode, close drilldown first
		if m.flowMatrix.showDrilldown {
			m.flowMatrix.showDrilldown = false
			return m
		}
		// Close flow matrix view
		m.focused = focusList
	case "j", "down":
		m.flowMatrix.MoveDown()
	case "k", "up":
		m.flowMatrix.MoveUp()
	case "tab":
		m.flowMatrix.TogglePanel()
	case "enter":
		// Open drilldown or jump to issue
		if m.flowMatrix.showDrilldown {
			// Jump to selected issue from drilldown
			if selectedIssue := m.flowMatrix.SelectedDrilldownIssue(); selectedIssue != nil {
				for i, item := range m.list.Items() {
					if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedIssue.ID {
						m.list.Select(i)
						break
					}
				}
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
		} else {
			// Open drilldown for selected label
			m.flowMatrix.OpenDrilldown()
		}
	case "G", "end":
		m.flowMatrix.GoToEnd()
	case "g", "home":
		m.flowMatrix.GoToStart()
	}
	return m
}

// handleRecipePickerKeys handles keyboard input when recipe picker is focused
func (m Model) handleRecipePickerKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.recipePicker.MoveDown()
	case "k", "up":
		m.recipePicker.MoveUp()
	case "esc":
		m.closeModal()
		m.focused = focusList
	case "enter":
		// Apply selected recipe
		if selected := m.recipePicker.SelectedRecipe(); selected != nil {
			m.setActiveRecipe(selected)
			m.applyRecipe(selected)
		}
		m.closeModal()
		m.focused = focusList
	}
	return m
}

// handleRepoPickerKeys handles keyboard input when repo picker is focused (workspace mode).
func (m Model) handleRepoPickerKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.repoPicker.MoveDown()
	case "k", "up":
		m.repoPicker.MoveUp()
	case "space":
		m.repoPicker.ToggleSelected()
	case "a":
		m.repoPicker.ToggleAll()
	case "esc", "q", "w":
		m.closeModal()
		m.focused = focusList
	case "enter":
		selected := m.repoPicker.SelectedRepos()

		if m.repoPicker.NoneSelected() {
			// No checkmarks: enter jumps to the cursor project (single-project switch)
			cursorRepo := m.repoPicker.CursorRepo()
			if cursorRepo != "" {
				m.activeRepos = map[string]bool{cursorRepo: true}
				m.statusMsg = fmt.Sprintf("Project filter: %s", cursorRepo)
			}
		} else if len(selected) == len(m.availableRepos) {
			// All selected: clear filter (nil = all)
			m.activeRepos = nil
			m.statusMsg = "Project filter: all projects"
		} else {
			m.activeRepos = selected
			m.statusMsg = fmt.Sprintf("Project filter: %s", formatRepoList(sortedRepoKeys(selected), 3))
		}
		m.statusIsError = false

		// Apply filter to views
		if m.filter.activeRecipe != nil {
			m.applyRecipe(m.filter.activeRecipe)
		} else {
			m.applyFilter()
		}

		m.closeModal()
		m.focused = focusList
	}
	return m
}

// handleBQLQueryKeys handles keyboard input when BQL query modal is focused.
func (m Model) handleBQLQueryKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		query := m.bqlQuery.Value()
		if query == "" {
			// Empty query = clear BQL filter, show all
			m.filter.activeBQLExpr = nil
			m.filter.currentFilter = "all"
			m.applyFilter()
		} else {
			// Parse and validate
			parsed, err := bql.Parse(query)
			if err != nil {
				m.bqlQuery.SetError(err.Error())
				return m, nil // Stay in modal
			}
			if err := bql.Validate(parsed); err != nil {
				m.bqlQuery.SetError(err.Error())
				return m, nil // Stay in modal
			}
			// Clear stale filter state from other filter types
			m.setActiveRecipe(nil)
			m.list.ResetFilter()
			// Apply BQL via dedicated path
			m.filter.activeBQLExpr = parsed
			m.applyBQL(parsed, query)
			m.bqlQuery.AddToHistory(query)
		}
		m.closeModal()
		m.focused = focusList
		m.statusMsg = "BQL: " + query
		m.statusIsError = false
		return m, nil

	case "esc":
		m.closeModal()
		m.focused = focusList
		return m, nil

	case "up":
		m.bqlQuery.HistoryPrev()
		return m, nil

	case "down":
		m.bqlQuery.HistoryNext()
		return m, nil

	default:
		var cmd tea.Cmd
		m.bqlQuery, cmd = m.bqlQuery.Update(msg)
		return m, cmd
	}
}

// handleLabelPickerKeys handles keyboard input when label picker is focused (bv-126)
// Letter keys are NOT used for navigation - they go to the text input for search.
// Only arrow keys and ctrl combos navigate. Space toggles multi-select.
func (m Model) handleLabelPickerKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "esc":
		m.closeModal()
		m.focused = focusList
	case "down", "ctrl+n":
		m.labelPicker.MoveDown()
	case "up", "ctrl+p":
		m.labelPicker.MoveUp()
	case "left":
		m.labelPicker.PageUp()
	case "right":
		m.labelPicker.PageDown()
	case "space":
		m.labelPicker.ToggleSelected()
	case "enter":
		selected := m.labelPicker.SelectedLabels()
		if len(selected) == 0 {
			// No checkmarks: enter applies the cursor label (single-select)
			if cursor := m.labelPicker.SelectedLabel(); cursor != "" {
				m.filter.labelFilter = cursor
				m.applyFilter()
				m.setStatus(fmt.Sprintf("Filtered by label: %s", cursor))
			}
		} else if len(selected) == 1 {
			m.filter.labelFilter = selected[0]
			m.applyFilter()
			m.setStatus(fmt.Sprintf("Filtered by label: %s", selected[0]))
		} else {
			// Multi-select: comma-separated labels
			m.filter.labelFilter = strings.Join(selected, ",")
			m.applyFilter()
			m.setStatus(fmt.Sprintf("Filtered by %d labels", len(selected)))
		}
		m.closeModal()
		m.focused = focusList
	default:
		// Pass all other keys (including letters) to text input for search
		m.labelPicker.UpdateInput(msg)
	}
	return m
}

// handleInsightsKeys handles keyboard input when insights panel is focused
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

// handleListKeys handles keyboard input when the list is focused
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
	case "h":
		// Toggle history view
		if m.mode != ViewHistory {
			m.enterHistoryView()
		}
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
	}
	return m
}

// handleTimeTravelInputKeys handles keyboard input for the time-travel revision prompt
func (m Model) handleTimeTravelInputKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "enter":
		// Submit the revision
		revision := strings.TrimSpace(m.timeTravelInput.Value())
		if revision == "" {
			revision = "HEAD~5" // Default if empty
		}
		m.closeModal()
		m.timeTravelInput.Blur()
		m.focused = focusList
		m.enterTimeTravelMode(revision)
	case "esc":
		// Cancel
		m.closeModal()
		m.timeTravelInput.Blur()
		m.focused = focusList
	default:
		// Update the textinput
		m.timeTravelInput, _ = m.timeTravelInput.Update(msg)
	}
	return m
}

// restoreFocusFromHelp returns the appropriate focus based on current view state.
// This fixes the bug where dismissing help would always return to focusList,
// even when the user was in a specialized view (graph, board, insights, etc.).
func (m Model) restoreFocusFromHelp() focus {
	// Full-screen detail view (not split mode)
	if m.showDetails && !m.isSplitView {
		return focusDetail
	}
	// Map ViewMode to the correct focus state
	switch m.mode {
	case ViewGraph:
		return focusGraph
	case ViewBoard:
		return focusBoard
	case ViewActionable:
		return focusActionable
	case ViewHistory:
		return focusHistory
	case ViewInsights, ViewAttention:
		return focusInsights
	case ViewLabelDashboard:
		return focusLabelDashboard
	case ViewSprint:
		return focusSprint
	case ViewFlowMatrix:
		return focusFlowMatrix
	case ViewTree:
		return focusTree
	}
	// Check for other focus states using stored focusBeforeHelp
	// (m.focused is focusHelp while help is open, so we use the saved value)
	if m.focusBeforeHelp == focusLabelPicker {
		return focusLabelPicker
	}
	if m.focusBeforeHelp == focusTimeTravelInput {
		return focusTimeTravelInput
	}
	// Default: return to list
	return focusList
}

// handleHelpKeys handles keyboard input when the help overlay is focused
func (m Model) handleHelpKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.helpScroll++
	case "k", "up":
		if m.helpScroll > 0 {
			m.helpScroll--
		}
	case "ctrl+d":
		m.helpScroll += 10
	case "ctrl+u":
		m.helpScroll -= 10
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
	case "home", "g":
		m.helpScroll = 0
	case "G", "end":
		// Will be clamped in render
		m.helpScroll = 999
	case "q", "esc", "?", "f1":
		// Close help overlay and restore previous focus
		m.closeModal()
		m.helpScroll = 0
		m.focused = m.restoreFocusFromHelp()
	case "space": // Space opens interactive tutorial (bv-0trk, bv-8y31)
		m.closeModal()
		m.helpScroll = 0
		m.openModal(ModalTutorial)
		m.tutorialModel.SetSize(m.width, m.height)
		m.focused = focusTutorial
	default:
		// Any other key dismisses help and restores previous focus
		m.closeModal()
		m.helpScroll = 0
		m.focused = m.restoreFocusFromHelp()
	}
	return m
}
