// Package ui provides the history view for displaying bead-to-commit correlations.
package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/seanmartinsmith/beadstui/pkg/cass"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
)

// historyFocus tracks which pane has focus in the history view
type historyFocus int

const (
	historyFocusList   historyFocus = iota // Left pane (beads or commits)
	historyFocusMiddle                     // Middle pane for 3-pane layout (bv-xrfh)
	historyFocusDetail                     // Right pane (details)
)

// historyLayout tracks the responsive layout mode (bv-xrfh)
type historyLayout int

const (
	layoutNarrow   historyLayout = iota // < 100 cols: two-pane optimized
	layoutStandard                      // 100-150 cols: three-pane standard
	layoutWide                          // > 150 cols: three-pane with timeline
)

// Layout breakpoints (bv-xrfh)
const (
	layoutBreakpointStandard = 100 // Width to switch to 3-pane
	layoutBreakpointWide     = 150 // Width to switch to wide mode
)

// historyViewMode tracks bead-centric vs git-centric view (bv-tl3n)
type historyViewMode int

const (
	historyModeBead historyViewMode = iota // Default: beads on left, commits for selected bead
	historyModeGit                         // Git mode: commits on left, related beads for selected commit
)

// CommitListEntry represents a commit in git-centric mode (bv-tl3n)
type CommitListEntry struct {
	SHA       string
	ShortSHA  string
	Message   string
	Author    string
	Timestamp string
	FileCount int
	BeadIDs   []string // Beads related to this commit
}

// historySearchMode tracks what type of search is active (bv-nkrj)
type historySearchMode int

const (
	searchModeOff    historySearchMode = iota // No search active
	searchModeAll                             // Search across all fields
	searchModeCommit                          // Search commit messages only
	searchModeSHA                             // Search by SHA prefix
	searchModeBead                            // Search bead ID/title
	searchModeAuthor                          // Search by author
)

// TimelineEntryType categorizes timeline entries (bv-1x6o)
type timelineEntryType int

const (
	timelineEntryEvent   timelineEntryType = iota // Lifecycle event (created, claimed, closed)
	timelineEntryCommit                           // Code commit
	timelineEntrySession                          // Cass coding session (bv-pr1l)
)

// TimelineEntry represents a single entry in the timeline visualization (bv-1x6o)
type TimelineEntry struct {
	Timestamp  time.Time
	EntryType  timelineEntryType
	Label      string  // Event type name or commit SHA
	Detail     string  // Full message or event detail
	Confidence float64 // For commits: correlation confidence (0-1)
	EventType  string  // For events: "created", "claimed", "closed", etc.

	// Session fields (bv-pr1l)
	SessionAgent        string  // For sessions: "claude", "cursor", etc.
	SessionMessageCount int     // For sessions: number of messages in session
	SessionPath         string  // For sessions: path to session file
	SessionScore        float64 // For sessions: correlation score
}

// FileTreeNode represents a node in the file tree (bv-190l)
type FileTreeNode struct {
	Name        string          // File or directory name
	Path        string          // Full path
	IsDir       bool            // True if directory
	Children    []*FileTreeNode // Child nodes (for directories)
	ChangeCount int             // Number of commits touching this path
	Expanded    bool            // True if directory is expanded
	Level       int             // Nesting depth for indentation
}

// HistoryContext describes the runtime context the History view was opened
// in. It is what lets the empty-state message be specific instead of vague
// when bt is launched outside a project's git tree (bt-ezk8).
//
// In single-project mode WorkspaceMode is false and ActiveProjects is empty.
// In global/workspace mode WorkspaceMode is true and ActiveProjects holds
// the current project filter (DB names, sorted; empty/nil means "all
// projects" because no filter is applied).
type HistoryContext struct {
	WorkspaceMode  bool
	ActiveProjects []string
	// CursorPrefix is the project prefix of the bead under the cursor (e.g.
	// "bt", "bd"). Empty when no bead is selected: header rows, separators,
	// empty lists, or unfocused-list states.
	CursorPrefix string
}

// HistoryModel represents the TUI view for bead history and code correlations
type HistoryModel struct {
	// Data
	report    *correlation.HistoryReport
	histories []correlation.BeadHistory // Filtered and sorted list
	beadIDs   []string                  // Sorted bead IDs for navigation

	// Navigation state
	selectedBead       int // Index into beadIDs
	selectedCommit     int // Index into selected bead's commits
	scrollOffset       int // For scrolling the bead list
	detailScrollOffset int // For scrolling the detail panel content (bt-npnh)
	focused            historyFocus

	// Git-centric mode state (bv-tl3n)
	viewMode            historyViewMode
	commitList          []CommitListEntry // All commits sorted by recency
	selectedGitCommit   int               // Index into commitList
	selectedRelatedBead int               // Index into selected commit's BeadIDs
	gitScrollOffset     int               // For scrolling the commit list

	// Three-pane middle panel scroll state (bv-xrfh)
	middleScrollOffset int // Scroll offset for middle pane content

	// Timeline panel state (bv-1x6o)
	timelineScrollOffset int // Scroll offset for timeline panel

	// Filters
	authorFilter  string  // Filter by author (empty = all)
	minConfidence float64 // Minimum confidence threshold (0-1)

	// Search state (bv-nkrj)
	searchInput     textinput.Model   // Text input for search query
	searchMode      historySearchMode // Current search mode
	searchActive    bool              // Whether search input is focused
	lastSearchQuery string            // Cache for detecting query changes
	filteredCommits []CommitListEntry // Filtered commit list for git mode

	// Display state
	width  int
	height int
	theme  Theme

	// Expanded state tracking
	expandedBeads map[string]bool // Track which beads have commits expanded

	// File tree state (bv-190l)
	showFileTree    bool            // Whether file tree panel is visible
	fileTree        []*FileTreeNode // Root-level nodes of the file tree
	flatFileList    []*FileTreeNode // Flattened visible nodes for navigation
	selectedFileIdx int             // Index in flatFileList
	fileTreeScroll  int             // Scroll offset for file tree
	fileFilter      string          // Current file filter (empty = no filter)
	fileTreeFocus   bool            // True when file tree has focus

	// Cass session integration state (bv-pr1l)
	sessionCache map[string][]cass.ScoredResult // Cached sessions per bead ID

	// View mode transition state (bv-kvlx)
	modeChangedAt time.Time // Timestamp of last mode toggle for transition animation

	// Workspace/global-mode context (bt-ezk8). Used only by the empty-state
	// renderer to explain *why* history is empty when the cwd is not a git
	// tree (e.g. global mode launched from $HOME). Has no effect on data
	// flow; the report is still authoritative for what's shown.
	context HistoryContext
}

// NewHistoryModel creates a new history view from a correlation report
func NewHistoryModel(report *correlation.HistoryReport, theme Theme) HistoryModel {
	// Initialize search input (bv-nkrj)
	ti := textinput.New()
	ti.Placeholder = "Search commits, beads, authors..."
	ti.CharLimit = 100
	ti.SetWidth(40)

	h := HistoryModel{
		report:        report,
		theme:         theme,
		focused:       historyFocusList,
		minConfidence: 0.0, // Show all by default
		expandedBeads: make(map[string]bool),
		searchInput:   ti,
		searchMode:    searchModeOff,
		sessionCache:  make(map[string][]cass.ScoredResult), // bv-pr1l
	}
	h.rebuildFilteredList()
	return h
}

// SetReport updates the history data
func (h *HistoryModel) SetReport(report *correlation.HistoryReport) {
	h.report = report
	h.rebuildFilteredList()
}

// SetContext records the workspace/global-mode context the History view was
// opened in (bt-ezk8). Used only by the empty-state renderer to produce an
// actionable message when cwd is not a git tree.
func (h *HistoryModel) SetContext(ctx HistoryContext) {
	h.context = ctx
}

// SetSessionsForBead stores correlated sessions for a bead in the cache (bv-pr1l)
// This is called when sessions are loaded asynchronously from the main model.
func (h *HistoryModel) SetSessionsForBead(beadID string, sessions []cass.ScoredResult) {
	if h.sessionCache == nil {
		h.sessionCache = make(map[string][]cass.ScoredResult)
	}
	h.sessionCache[beadID] = sessions
}

// HasSessionsForBead returns true if sessions are cached for the given bead (bv-pr1l)
func (h *HistoryModel) HasSessionsForBead(beadID string) bool {
	if h.sessionCache == nil {
		return false
	}
	_, ok := h.sessionCache[beadID]
	return ok
}

// GetSessionsForBead returns cached sessions for a bead (bv-pr1l)
func (h *HistoryModel) GetSessionsForBead(beadID string) []cass.ScoredResult {
	if h.sessionCache == nil {
		return nil
	}
	return h.sessionCache[beadID]
}

// ClearSessionCache clears all cached sessions (bv-pr1l)
func (h *HistoryModel) ClearSessionCache() {
	h.sessionCache = make(map[string][]cass.ScoredResult)
}

// rebuildFilteredList rebuilds the filtered and sorted list of histories
func (h *HistoryModel) rebuildFilteredList() {
	// Capture current selection
	var selectedID string
	if h.selectedBead < len(h.beadIDs) {
		selectedID = h.beadIDs[h.selectedBead]
	}

	h.histories = nil
	h.beadIDs = nil

	if h.report == nil {
		return
	}

	// Filter and collect histories
	for beadID, history := range h.report.Histories {
		// Skip beads with no commits
		if len(history.Commits) == 0 {
			continue
		}

		// Apply author filter
		if h.authorFilter != "" {
			authorMatch := false
			for _, c := range history.Commits {
				if strings.Contains(strings.ToLower(c.Author), strings.ToLower(h.authorFilter)) ||
					strings.Contains(strings.ToLower(c.AuthorEmail), strings.ToLower(h.authorFilter)) {
					authorMatch = true
					break
				}
			}
			if !authorMatch {
				continue
			}
		}

		// Apply confidence filter - keep only commits meeting threshold
		if h.minConfidence > 0 {
			var filtered []correlation.CorrelatedCommit
			for _, c := range history.Commits {
				if c.Confidence >= h.minConfidence {
					filtered = append(filtered, c)
				}
			}
			if len(filtered) == 0 {
				continue
			}
			history.Commits = filtered
		}

		// Apply file filter (bv-190l) - keep only commits touching the filtered path
		if h.fileFilter != "" {
			var filtered []correlation.CorrelatedCommit
			for _, c := range history.Commits {
				for _, file := range c.Files {
					// Match if file path equals filter or starts with filter (directory match)
					if file.Path == h.fileFilter || strings.HasPrefix(file.Path, h.fileFilter+"/") {
						filtered = append(filtered, c)
						break
					}
				}
			}
			if len(filtered) == 0 {
				continue
			}
			history.Commits = filtered
		}

		h.histories = append(h.histories, history)
		h.beadIDs = append(h.beadIDs, beadID)
	}

	// Sort by most commits first
	sort.Slice(h.histories, func(i, j int) bool {
		if len(h.histories[i].Commits) != len(h.histories[j].Commits) {
			return len(h.histories[i].Commits) > len(h.histories[j].Commits)
		}
		return h.histories[i].BeadID < h.histories[j].BeadID
	})

	// Rebuild beadIDs to match sorted order
	h.beadIDs = make([]string, len(h.histories))
	for i, hist := range h.histories {
		h.beadIDs[i] = hist.BeadID
	}

	// Restore selection if possible
	found := false
	if selectedID != "" {
		for i, id := range h.beadIDs {
			if id == selectedID {
				h.selectedBead = i
				found = true
				break
			}
		}
	}

	if found {
		// Clamp selected commit as commit list might have shrunk
		numCommits := len(h.histories[h.selectedBead].Commits)
		if h.selectedCommit >= numCommits {
			if numCommits > 0 {
				h.selectedCommit = numCommits - 1
			} else {
				h.selectedCommit = 0
			}
		}
	} else {
		// Reset selection if out of bounds or lost
		h.selectedBead = 0
		h.selectedCommit = 0
	}
}

// SetSize updates the view dimensions
func (h *HistoryModel) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// SetAuthorFilter sets the author filter and rebuilds the list
func (h *HistoryModel) SetAuthorFilter(author string) {
	h.authorFilter = author
	h.rebuildFilteredList()
}

// SetMinConfidence sets the minimum confidence threshold and rebuilds the list
func (h *HistoryModel) SetMinConfidence(conf float64) {
	h.minConfidence = conf
	h.rebuildFilteredList()
}

// File tree methods (bv-190l)

// buildFileTree constructs a tree from all files in the history report
func (h *HistoryModel) buildFileTree() {
	if h.report == nil {
		h.fileTree = nil
		h.flatFileList = nil
		return
	}

	// Count changes per file path
	fileChanges := make(map[string]int)
	for _, hist := range h.report.Histories {
		for _, commit := range hist.Commits {
			for _, file := range commit.Files {
				fileChanges[file.Path]++
			}
		}
	}

	// Build tree structure
	root := make(map[string]*FileTreeNode)

	for path, count := range fileChanges {
		parts := strings.Split(path, "/")

		// Create/update nodes for each part of the path
		for i := range parts {
			isLast := i == len(parts)-1
			fullPath := strings.Join(parts[:i+1], "/")
			name := parts[i]

			if _, exists := root[fullPath]; !exists {
				root[fullPath] = &FileTreeNode{
					Name:        name,
					Path:        fullPath,
					IsDir:       !isLast,
					ChangeCount: 0,
					Expanded:    false,
					Level:       i,
				}
			}

			if isLast {
				root[fullPath].ChangeCount = count
			}
		}
	}

	// Link children to parents
	for path, node := range root {
		if node.Level == 0 {
			continue
		}
		parentPath := strings.Join(strings.Split(path, "/")[:node.Level], "/")
		if parent, exists := root[parentPath]; exists {
			parent.Children = append(parent.Children, node)
		}
	}

	// Extract root level nodes
	h.fileTree = nil
	for _, node := range root {
		if node.Level == 0 {
			h.sortTreeNode(node)
			h.fileTree = append(h.fileTree, node)
		}
	}

	// Sort root level
	sort.Slice(h.fileTree, func(i, j int) bool {
		if h.fileTree[i].IsDir != h.fileTree[j].IsDir {
			return h.fileTree[i].IsDir
		}
		return h.fileTree[i].Name < h.fileTree[j].Name
	})

	h.rebuildFlatFileList()
}

// sortTreeNode recursively sorts a tree node's children
func (h *HistoryModel) sortTreeNode(node *FileTreeNode) {
	if node.Children == nil {
		return
	}
	for _, child := range node.Children {
		h.sortTreeNode(child)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
}

// rebuildFlatFileList creates a flat list of visible nodes for navigation
func (h *HistoryModel) rebuildFlatFileList() {
	h.flatFileList = nil
	for _, node := range h.fileTree {
		h.addToFlatList(node)
	}
}

// addToFlatList recursively adds nodes to the flat list
func (h *HistoryModel) addToFlatList(node *FileTreeNode) {
	h.flatFileList = append(h.flatFileList, node)
	if node.IsDir && node.Expanded {
		for _, child := range node.Children {
			h.addToFlatList(child)
		}
	}
}

// ToggleFileTree toggles the file tree panel visibility
func (h *HistoryModel) ToggleFileTree() {
	h.showFileTree = !h.showFileTree
	if h.showFileTree && h.fileTree == nil {
		h.buildFileTree()
	}
}

// IsFileTreeVisible returns whether the file tree panel is visible
func (h *HistoryModel) IsFileTreeVisible() bool {
	return h.showFileTree
}

// FileTreeHasFocus returns whether the file tree has focus
func (h *HistoryModel) FileTreeHasFocus() bool {
	return h.fileTreeFocus
}

// SetFileTreeFocus sets the file tree focus state
func (h *HistoryModel) SetFileTreeFocus(focus bool) {
	h.fileTreeFocus = focus
}

// MoveUpFileTree moves selection up in the file tree
func (h *HistoryModel) MoveUpFileTree() {
	if h.selectedFileIdx > 0 {
		h.selectedFileIdx--
	}
}

// MoveDownFileTree moves selection down in the file tree
func (h *HistoryModel) MoveDownFileTree() {
	if h.selectedFileIdx < len(h.flatFileList)-1 {
		h.selectedFileIdx++
	}
}

// ToggleExpandFile expands or collapses the selected directory
func (h *HistoryModel) ToggleExpandFile() {
	if h.selectedFileIdx >= len(h.flatFileList) {
		return
	}
	node := h.flatFileList[h.selectedFileIdx]
	if node.IsDir {
		node.Expanded = !node.Expanded
		h.rebuildFlatFileList()
	}
}

// SelectFile sets the file filter to the selected file
func (h *HistoryModel) SelectFile() {
	if h.selectedFileIdx >= len(h.flatFileList) {
		return
	}
	node := h.flatFileList[h.selectedFileIdx]
	if h.fileFilter == node.Path {
		h.fileFilter = ""
	} else {
		h.fileFilter = node.Path
	}
	h.rebuildFilteredList()
}

// ClearFileFilter clears the file filter
func (h *HistoryModel) ClearFileFilter() {
	h.fileFilter = ""
	h.rebuildFilteredList()
}

// GetFileFilter returns the current file filter
func (h *HistoryModel) GetFileFilter() string {
	return h.fileFilter
}

// SelectedFileName returns the name of the selected file/directory
func (h *HistoryModel) SelectedFileName() string {
	if h.selectedFileIdx >= len(h.flatFileList) {
		return ""
	}
	return h.flatFileList[h.selectedFileIdx].Name
}

// SelectedFileNode returns the currently selected file tree node
func (h *HistoryModel) SelectedFileNode() *FileTreeNode {
	if h.selectedFileIdx >= len(h.flatFileList) {
		return nil
	}
	return h.flatFileList[h.selectedFileIdx]
}

// CollapseFileNode collapses the selected node if it's an expanded directory
func (h *HistoryModel) CollapseFileNode() {
	if h.selectedFileIdx >= len(h.flatFileList) {
		return
	}
	node := h.flatFileList[h.selectedFileIdx]
	if node.IsDir && node.Expanded {
		node.Expanded = false
		h.rebuildFlatFileList()
	}
}

// Navigation methods

// MoveUp moves selection up in the current focus pane
func (h *HistoryModel) MoveUp() {
	if h.focused == historyFocusList {
		if h.selectedBead > 0 {
			h.selectedBead--
			h.selectedCommit = 0
			h.middleScrollOffset = 0 // Reset middle scroll when changing bead (bv-xrfh)
			h.detailScrollOffset = 0 // Reset detail scroll when changing bead (bt-npnh)
			h.ensureBeadVisible()
		}
	} else {
		// In middle or detail pane, move to previous commit
		if h.selectedCommit > 0 {
			h.selectedCommit--
			// Update middle pane scroll if in three-pane layout (bv-xrfh)
			if h.focused == historyFocusMiddle && h.selectedBead < len(h.histories) {
				h.ensureMiddleScrollVisible(h.selectedCommit, len(h.histories[h.selectedBead].Commits))
			}
		}
	}
}

// MoveDown moves selection down in the current focus pane
func (h *HistoryModel) MoveDown() {
	if h.focused == historyFocusList {
		if h.selectedBead < len(h.histories)-1 {
			h.selectedBead++
			h.selectedCommit = 0
			h.middleScrollOffset = 0 // Reset middle scroll when changing bead (bv-xrfh)
			h.detailScrollOffset = 0 // Reset detail scroll when changing bead (bt-npnh)
			h.ensureBeadVisible()
		}
	} else {
		// In middle or detail pane, move to next commit
		if h.selectedBead < len(h.histories) {
			commits := h.histories[h.selectedBead].Commits
			if h.selectedCommit < len(commits)-1 {
				h.selectedCommit++
				// Update middle pane scroll if in three-pane layout (bv-xrfh)
				if h.focused == historyFocusMiddle {
					h.ensureMiddleScrollVisible(h.selectedCommit, len(commits))
				}
			}
		}
	}
}

// ToggleFocus cycles through panes based on current layout (bv-xrfh)
func (h *HistoryModel) ToggleFocus() {
	panes := h.paneCount()
	if panes == 3 {
		// Three-pane: List -> Middle -> Detail -> List
		switch h.focused {
		case historyFocusList:
			h.focused = historyFocusMiddle
		case historyFocusMiddle:
			h.focused = historyFocusDetail
		default:
			h.focused = historyFocusList
		}
	} else {
		// Two-pane: List <-> Detail
		if h.focused == historyFocusList {
			h.focused = historyFocusDetail
		} else {
			h.focused = historyFocusList
		}
	}
}

// IsDetailFocused returns true if the detail pane has focus (bv-190l)
func (h *HistoryModel) IsDetailFocused() bool {
	return h.focused == historyFocusDetail
}

// NextCommit moves to the next commit within the selected bead (J key)
func (h *HistoryModel) NextCommit() {
	if h.selectedBead >= len(h.histories) {
		return
	}
	commits := h.histories[h.selectedBead].Commits
	if h.selectedCommit < len(commits)-1 {
		h.selectedCommit++
	}
}

// PrevCommit moves to the previous commit within the selected bead (K key)
func (h *HistoryModel) PrevCommit() {
	if h.selectedCommit > 0 {
		h.selectedCommit--
	}
}

// CycleConfidence cycles through common confidence thresholds (0, 0.5, 0.75, 0.9)
func (h *HistoryModel) CycleConfidence() {
	thresholds := []float64{0, 0.5, 0.75, 0.9}
	// Find current threshold index
	currentIdx := 0
	for i, t := range thresholds {
		if h.minConfidence >= t-0.01 && h.minConfidence <= t+0.01 {
			currentIdx = i
			break
		}
	}
	// Move to next threshold (wrap around)
	nextIdx := (currentIdx + 1) % len(thresholds)
	h.SetMinConfidence(thresholds[nextIdx])
}

// GetMinConfidence returns the current minimum confidence threshold
func (h *HistoryModel) GetMinConfidence() float64 {
	return h.minConfidence
}

// ToggleExpand expands/collapses the commits for the selected bead
func (h *HistoryModel) ToggleExpand() {
	if h.selectedBead < len(h.beadIDs) {
		beadID := h.beadIDs[h.selectedBead]
		h.expandedBeads[beadID] = !h.expandedBeads[beadID]
	}
}

// Search and Filter methods (bv-nkrj)

// StartSearch activates the search input
func (h *HistoryModel) StartSearch() {
	h.searchActive = true
	h.searchMode = searchModeAll
	h.searchInput.Focus()
}

// StartSearchWithMode activates search with a specific mode
func (h *HistoryModel) StartSearchWithMode(mode historySearchMode) {
	h.searchActive = true
	h.searchMode = mode
	h.searchInput.Focus()

	// Set appropriate placeholder based on mode
	switch mode {
	case searchModeCommit:
		h.searchInput.Placeholder = "Search commit messages..."
	case searchModeSHA:
		h.searchInput.Placeholder = "Enter SHA prefix..."
	case searchModeBead:
		h.searchInput.Placeholder = "Search bead ID or title..."
	case searchModeAuthor:
		h.searchInput.Placeholder = "Search by author..."
	default:
		h.searchInput.Placeholder = "Search commits, beads, authors..."
	}
}

// CancelSearch cancels the search and clears the query
func (h *HistoryModel) CancelSearch() {
	h.searchActive = false
	h.searchInput.Blur()
	h.searchInput.SetValue("")
	h.searchMode = searchModeOff
	h.lastSearchQuery = ""
	h.applySearchFilter()
}

// ClearSearch clears the search query but keeps search mode active
func (h *HistoryModel) ClearSearch() {
	h.searchInput.SetValue("")
	h.lastSearchQuery = ""
	h.applySearchFilter()
}

// IsSearchActive returns whether search input is active
func (h *HistoryModel) IsSearchActive() bool {
	return h.searchActive
}

// SearchQuery returns the current search query
func (h *HistoryModel) SearchQuery() string {
	return h.searchInput.Value()
}

// UpdateSearchInput updates the search input model (call from Update)
func (h *HistoryModel) UpdateSearchInput(msg interface{}) {
	h.searchInput, _ = h.searchInput.Update(msg)

	// Check if query changed and apply filter
	currentQuery := h.searchInput.Value()
	if currentQuery != h.lastSearchQuery {
		h.lastSearchQuery = currentQuery
		h.applySearchFilter()
	}
}

// applySearchFilter filters the data based on current search query
func (h *HistoryModel) applySearchFilter() {
	// Always rebuild base filtered list first (applies author/confidence filters)
	// This ensures we always filter from the complete set, not an already-filtered list
	// (bv-nkrj fix: backspacing to relax filter now works correctly)
	h.rebuildFilteredList()
	if h.viewMode == historyModeGit {
		h.buildCommitList()
	}

	query := strings.TrimSpace(h.searchInput.Value())
	if query == "" {
		h.filteredCommits = nil // Use full commitList in git mode
		return
	}

	// Apply search filter on top of base filters
	if h.viewMode == historyModeGit {
		h.filterCommitList(query)
	} else {
		h.filterBeadList(query)
	}
}

// filterCommitList filters commits in git mode based on search query
func (h *HistoryModel) filterCommitList(query string) {
	if len(h.commitList) == 0 {
		h.filteredCommits = nil
		return
	}

	query = strings.ToLower(query)
	var filtered []CommitListEntry

	for _, commit := range h.commitList {
		if h.commitMatchesQuery(commit, query) {
			filtered = append(filtered, commit)
		}
	}

	h.filteredCommits = filtered
	// Reset selection if out of bounds
	if h.selectedGitCommit >= len(filtered) {
		h.selectedGitCommit = 0
		h.selectedRelatedBead = 0
	}
	h.gitScrollOffset = 0
}

// commitMatchesQuery checks if a commit matches the search query
func (h *HistoryModel) commitMatchesQuery(commit CommitListEntry, query string) bool {
	switch h.searchMode {
	case searchModeSHA:
		return strings.HasPrefix(strings.ToLower(commit.SHA), query) ||
			strings.HasPrefix(strings.ToLower(commit.ShortSHA), query)
	case searchModeCommit:
		return strings.Contains(strings.ToLower(commit.Message), query)
	case searchModeAuthor:
		return strings.Contains(strings.ToLower(commit.Author), query)
	case searchModeBead:
		for _, beadID := range commit.BeadIDs {
			if strings.Contains(strings.ToLower(beadID), query) {
				return true
			}
			// Also check bead title if available
			if h.report != nil {
				if hist, ok := h.report.Histories[beadID]; ok {
					if strings.Contains(strings.ToLower(hist.Title), query) {
						return true
					}
				}
			}
		}
		return false
	default: // searchModeAll - search across all fields
		if strings.HasPrefix(strings.ToLower(commit.SHA), query) ||
			strings.HasPrefix(strings.ToLower(commit.ShortSHA), query) {
			return true
		}
		if strings.Contains(strings.ToLower(commit.Message), query) {
			return true
		}
		if strings.Contains(strings.ToLower(commit.Author), query) {
			return true
		}
		for _, beadID := range commit.BeadIDs {
			if strings.Contains(strings.ToLower(beadID), query) {
				return true
			}
		}
		return false
	}
}

// filterBeadList filters beads in bead mode based on search query
func (h *HistoryModel) filterBeadList(query string) {
	if h.report == nil {
		return
	}

	query = strings.ToLower(query)
	var filteredHistories []correlation.BeadHistory
	var filteredIDs []string

	for _, beadID := range h.beadIDs {
		if hist, ok := h.report.Histories[beadID]; ok {
			if h.beadMatchesQuery(beadID, hist, query) {
				filteredHistories = append(filteredHistories, hist)
				filteredIDs = append(filteredIDs, beadID)
			}
		}
	}

	h.histories = filteredHistories
	h.beadIDs = filteredIDs

	// Reset selection if out of bounds
	if h.selectedBead >= len(h.beadIDs) {
		h.selectedBead = 0
		h.selectedCommit = 0
	}
	h.scrollOffset = 0
}

// beadMatchesQuery checks if a bead matches the search query
func (h *HistoryModel) beadMatchesQuery(beadID string, hist correlation.BeadHistory, query string) bool {
	switch h.searchMode {
	case searchModeBead:
		return strings.Contains(strings.ToLower(beadID), query) ||
			strings.Contains(strings.ToLower(hist.Title), query)
	case searchModeCommit:
		for _, commit := range hist.Commits {
			if strings.Contains(strings.ToLower(commit.Message), query) {
				return true
			}
		}
		return false
	case searchModeSHA:
		for _, commit := range hist.Commits {
			if strings.HasPrefix(strings.ToLower(commit.SHA), query) ||
				strings.HasPrefix(strings.ToLower(commit.ShortSHA), query) {
				return true
			}
		}
		return false
	case searchModeAuthor:
		for _, commit := range hist.Commits {
			if strings.Contains(strings.ToLower(commit.Author), query) {
				return true
			}
		}
		return false
	default: // searchModeAll
		// Check bead ID and title
		if strings.Contains(strings.ToLower(beadID), query) ||
			strings.Contains(strings.ToLower(hist.Title), query) {
			return true
		}
		// Check commits
		for _, commit := range hist.Commits {
			if strings.Contains(strings.ToLower(commit.Message), query) ||
				strings.Contains(strings.ToLower(commit.Author), query) ||
				strings.HasPrefix(strings.ToLower(commit.ShortSHA), query) {
				return true
			}
		}
		return false
	}
}

// GetFilteredCommitList returns the filtered commit list for git mode
func (h *HistoryModel) GetFilteredCommitList() []CommitListEntry {
	if h.filteredCommits != nil {
		return h.filteredCommits
	}
	return h.commitList
}

// GetSearchModeName returns a human-readable name for the current search mode
func (h *HistoryModel) GetSearchModeName() string {
	switch h.searchMode {
	case searchModeCommit:
		return "msg"
	case searchModeSHA:
		return "sha"
	case searchModeBead:
		return "bead"
	case searchModeAuthor:
		return "author"
	default:
		return "all"
	}
}

// Git-Centric View Mode methods (bv-tl3n)

// ToggleViewMode switches between Bead mode and Git mode
func (h *HistoryModel) ToggleViewMode() {
	// Track mode change time for transition animation (bv-kvlx)
	h.modeChangedAt = time.Now()

	if h.viewMode == historyModeBead {
		h.viewMode = historyModeGit
		h.buildCommitList()
		h.selectedGitCommit = 0
		h.selectedRelatedBead = 0
		h.gitScrollOffset = 0
	} else {
		h.viewMode = historyModeBead
		h.selectedBead = 0
		h.selectedCommit = 0
		h.scrollOffset = 0
	}
	// Re-apply search filter if active (bv-nkrj fix: filter persists across mode toggle)
	if h.searchActive && h.searchInput.Value() != "" {
		h.applySearchFilter()
	}
}

// IsGitMode returns true if in git-centric view mode
func (h *HistoryModel) IsGitMode() bool {
	return h.viewMode == historyModeGit
}

// buildCommitList constructs the sorted commit list for git mode
func (h *HistoryModel) buildCommitList() {
	if h.report == nil {
		h.commitList = nil
		return
	}

	seen := make(map[string]bool)
	var entries []CommitListEntry

	// Collect all commits from all bead histories
	for beadID, hist := range h.report.Histories {
		for _, commit := range hist.Commits {
			if seen[commit.SHA] {
				// Already have this commit, just add the bead ID
				for i := range entries {
					if entries[i].SHA == commit.SHA {
						// Check if bead already in list
						found := false
						for _, bid := range entries[i].BeadIDs {
							if bid == beadID {
								found = true
								break
							}
						}
						if !found {
							entries[i].BeadIDs = append(entries[i].BeadIDs, beadID)
						}
						break
					}
				}
				continue
			}
			seen[commit.SHA] = true

			entries = append(entries, CommitListEntry{
				SHA:       commit.SHA,
				ShortSHA:  commit.ShortSHA,
				Message:   commit.Message,
				Author:    commit.Author,
				Timestamp: commit.Timestamp.Format("2006-01-02 15:04"),
				FileCount: len(commit.Files),
				BeadIDs:   []string{beadID},
			})
		}
	}

	// Sort by timestamp descending (most recent first)
	// Note: We parse from formatted string since we stored it that way
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	h.commitList = entries
}

// MoveUpGit moves selection up in git mode
func (h *HistoryModel) MoveUpGit() {
	if h.focused == historyFocusList {
		if h.selectedGitCommit > 0 {
			h.selectedGitCommit--
			h.selectedRelatedBead = 0
			h.middleScrollOffset = 0 // Reset middle scroll when changing commit (bv-xrfh)
			h.ensureGitCommitVisible()
		}
	} else {
		// In middle or detail pane, move to previous related bead
		if h.selectedRelatedBead > 0 {
			h.selectedRelatedBead--
			// Update middle pane scroll if in three-pane layout (bv-xrfh)
			if h.focused == historyFocusMiddle {
				commit := h.SelectedGitCommit()
				if commit != nil {
					h.ensureMiddleScrollVisible(h.selectedRelatedBead, len(commit.BeadIDs))
				}
			}
		}
	}
}

// MoveDownGit moves selection down in git mode
func (h *HistoryModel) MoveDownGit() {
	commits := h.GetFilteredCommitList() // Use filtered list (bv-nkrj)
	if h.focused == historyFocusList {
		if h.selectedGitCommit < len(commits)-1 {
			h.selectedGitCommit++
			h.selectedRelatedBead = 0
			h.middleScrollOffset = 0 // Reset middle scroll when changing commit (bv-xrfh)
			h.ensureGitCommitVisible()
		}
	} else {
		// In middle or detail pane, move to next related bead
		if h.selectedGitCommit < len(commits) {
			beadCount := len(commits[h.selectedGitCommit].BeadIDs)
			if h.selectedRelatedBead < beadCount-1 {
				h.selectedRelatedBead++
				// Update middle pane scroll if in three-pane layout (bv-xrfh)
				if h.focused == historyFocusMiddle {
					h.ensureMiddleScrollVisible(h.selectedRelatedBead, beadCount)
				}
			}
		}
	}
}

// NextRelatedBead moves to the next related bead in git mode (J key)
func (h *HistoryModel) NextRelatedBead() {
	commits := h.GetFilteredCommitList() // Use filtered list (bv-nkrj)
	if h.selectedGitCommit >= len(commits) {
		return
	}
	beadCount := len(commits[h.selectedGitCommit].BeadIDs)
	if h.selectedRelatedBead < beadCount-1 {
		h.selectedRelatedBead++
	}
}

// PrevRelatedBead moves to the previous related bead in git mode (K key)
func (h *HistoryModel) PrevRelatedBead() {
	if h.selectedRelatedBead > 0 {
		h.selectedRelatedBead--
	}
}

// SelectedGitCommit returns the selected commit in git mode
func (h *HistoryModel) SelectedGitCommit() *CommitListEntry {
	commits := h.GetFilteredCommitList() // Use filtered list (bv-nkrj)
	if h.selectedGitCommit < len(commits) {
		return &commits[h.selectedGitCommit]
	}
	return nil
}

// SelectedRelatedBeadID returns the currently selected related bead ID in git mode
func (h *HistoryModel) SelectedRelatedBeadID() string {
	commit := h.SelectedGitCommit()
	if commit != nil && h.selectedRelatedBead < len(commit.BeadIDs) {
		return commit.BeadIDs[h.selectedRelatedBead]
	}
	return ""
}

// ensureGitCommitVisible adjusts scroll offset to keep selected commit visible
func (h *HistoryModel) ensureGitCommitVisible() {
	visibleItems := h.listHeight()
	if visibleItems < 1 {
		visibleItems = 1
	}

	if h.selectedGitCommit < h.gitScrollOffset {
		h.gitScrollOffset = h.selectedGitCommit
	} else if h.selectedGitCommit >= h.gitScrollOffset+visibleItems {
		h.gitScrollOffset = h.selectedGitCommit - visibleItems + 1
	}
}

// ensureBeadVisible adjusts scroll offset to keep selected bead visible
func (h *HistoryModel) ensureBeadVisible() {
	visibleItems := h.listHeight()
	if visibleItems < 1 {
		visibleItems = 1
	}

	if h.selectedBead < h.scrollOffset {
		h.scrollOffset = h.selectedBead
	} else if h.selectedBead >= h.scrollOffset+visibleItems {
		h.scrollOffset = h.selectedBead - visibleItems + 1
	}
}

// ensureMiddleScrollVisible adjusts middle pane scroll offset (bv-xrfh)
func (h *HistoryModel) ensureMiddleScrollVisible(selectedIdx, itemCount int) {
	// Use similar height calculation as middle pane (accounting for header/border)
	visibleItems := h.height - 7 // Header, separator, border padding
	if visibleItems < 1 {
		visibleItems = 1
	}

	if selectedIdx < h.middleScrollOffset {
		h.middleScrollOffset = selectedIdx
	} else if selectedIdx >= h.middleScrollOffset+visibleItems {
		h.middleScrollOffset = selectedIdx - visibleItems + 1
	}

	// Clamp to valid range
	maxScroll := itemCount - visibleItems
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.middleScrollOffset > maxScroll {
		h.middleScrollOffset = maxScroll
	}
}

// listHeight returns the number of visible items in the list
func (h *HistoryModel) listHeight() int {
	// Reserve 3 lines for header/filter bar
	return h.height - 3
}

// SelectedBeadID returns the currently selected bead ID
func (h *HistoryModel) SelectedBeadID() string {
	if h.selectedBead < len(h.beadIDs) {
		return h.beadIDs[h.selectedBead]
	}
	return ""
}

// SelectedHistory returns the currently selected bead history
func (h *HistoryModel) SelectedHistory() *correlation.BeadHistory {
	if h.selectedBead < len(h.histories) {
		return &h.histories[h.selectedBead]
	}
	return nil
}

// SelectedCommit returns the currently selected commit
func (h *HistoryModel) SelectedCommit() *correlation.CorrelatedCommit {
	hist := h.SelectedHistory()
	if hist != nil && h.selectedCommit < len(hist.Commits) {
		return &hist.Commits[h.selectedCommit]
	}
	return nil
}

// GetHistoryForBead returns the history for a specific bead ID
func (h *HistoryModel) GetHistoryForBead(beadID string) *correlation.BeadHistory {
	if h.report == nil {
		return nil
	}
	hist, ok := h.report.Histories[beadID]
	if !ok {
		return nil
	}
	return &hist
}

// HasReport returns true if history data is loaded
func (h *HistoryModel) HasReport() bool {
	return h.report != nil
}

// determineLayout returns the appropriate layout based on terminal width (bv-xrfh)
func (h *HistoryModel) determineLayout() historyLayout {
	if h.width < layoutBreakpointStandard {
		return layoutNarrow
	} else if h.width < layoutBreakpointWide {
		return layoutStandard
	}
	return layoutWide
}

// paneCount returns the number of visible panes for the current layout (bv-xrfh)
func (h *HistoryModel) paneCount() int {
	switch h.determineLayout() {
	case layoutNarrow:
		return 2
	case layoutStandard, layoutWide:
		return 3
	default:
		return 2
	}
}

// View renders the history view
func (h *HistoryModel) View() string {
	if h.report == nil {
		return h.renderEmpty("No history data loaded")
	}

	// Genuinely-empty data path: only show the full-screen empty state when
	// the underlying report has no usable commits. When data exists but the
	// active filters (search, author, confidence, file) narrow the list to
	// zero, fall through to the normal pane chrome -- the list pane shows
	// "No matches" inline (bt-z63i). Without this split the same empty
	// path fired for both, so typing a search query that filtered to zero
	// collapsed the entire view to the no-data message and the user lost
	// their navigation context.
	if !h.hasAnyHistoryData() {
		if h.viewMode == historyModeGit {
			return h.renderEmpty(h.emptyStateMessage("No commits with bead correlations found"))
		}
		return h.renderEmpty(h.emptyStateMessage("No beads with commit correlations found"))
	}

	// Dispatch to layout-specific renderer (bv-xrfh)
	layout := h.determineLayout()
	switch layout {
	case layoutStandard, layoutWide:
		return h.renderThreePaneView()
	default:
		return h.renderTwoPaneView()
	}
}

// hasAnyHistoryData reports whether the loaded report has at least one bead
// with any commit. Used to distinguish "no data" (full-screen empty state)
// from "filtered to zero" (in-chrome empty list with "No matches"). See
// bt-z63i.
func (h *HistoryModel) hasAnyHistoryData() bool {
	if h.report == nil {
		return false
	}
	for _, hist := range h.report.Histories {
		if len(hist.Commits) > 0 {
			return true
		}
	}
	return false
}

// historyPanelHeight returns the rows available for the panel block, given
// the actual rendered header height. Using a fixed h.height-2 (pre-bt-peo7
// follow-up) overflowed the available area whenever the header rendered as
// 3-4 rows (titleLine + statsLine + filterLine + separator), clipping the
// bottom of every panel.
func (h *HistoryModel) historyPanelHeight(header string) int {
	headerRows := lipgloss.Height(header)
	if headerRows < 1 {
		headerRows = 1
	}
	panelH := h.height - headerRows
	if panelH < 3 {
		panelH = 3
	}
	return panelH
}

// renderTwoPaneView renders the narrow two-pane layout (bv-xrfh)
func (h *HistoryModel) renderTwoPaneView() string {
	// Calculate panel widths (45% list, 55% detail for narrow)
	listWidth := int(float64(h.width) * 0.45)
	detailWidth := h.width - listWidth

	// Render header
	header := h.renderHeader()
	panelHeight := h.historyPanelHeight(header)

	// Render panels based on view mode (bv-tl3n)
	var listPanel, detailPanel string
	if h.viewMode == historyModeGit {
		listPanel = h.renderGitCommitListPanel(listWidth, panelHeight)
		detailPanel = h.renderGitDetailPanel(detailWidth, panelHeight)
	} else {
		listPanel = h.renderListPanel(listWidth, panelHeight)
		detailPanel = h.renderDetailPanel(detailWidth, panelHeight)
	}

	// Combine panels
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detailPanel)

	return lipgloss.JoinVertical(lipgloss.Left, header, panels)
}

// renderThreePaneView renders the three-pane layout for wider terminals (bv-xrfh)
// In wide mode (>150 cols), adds a fourth timeline pane (bv-1x6o)
func (h *HistoryModel) renderThreePaneView() string {
	layout := h.determineLayout()

	// Render header
	header := h.renderHeader()
	panelHeight := h.historyPanelHeight(header)

	// Wide layout: 4 panes with timeline (bv-1x6o)
	if layout == layoutWide && h.viewMode != historyModeGit {
		// Wide bead mode: 20% beads | 22% timeline | 25% commits | 33% details
		listWidth := int(float64(h.width) * 0.20)
		timelineWidth := int(float64(h.width) * 0.22)
		middleWidth := int(float64(h.width) * 0.25)
		detailWidth := h.width - listWidth - timelineWidth - middleWidth

		listPanel := h.renderListPanel(listWidth, panelHeight)
		timelinePanel := h.renderTimelinePanel(timelineWidth, panelHeight)
		middlePanel := h.renderCommitMiddlePanel(middleWidth, panelHeight)
		detailPanel := h.renderDetailPanel(detailWidth, panelHeight)

		panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, timelinePanel, middlePanel, detailPanel)
		return lipgloss.JoinVertical(lipgloss.Left, header, panels)
	}

	// Standard 3-pane layout (also used for git mode in wide)
	var listWidth, middleWidth, detailWidth int
	if layout == layoutWide {
		// Wide git mode: 25% | 30% | 45%
		listWidth = int(float64(h.width) * 0.25)
		middleWidth = int(float64(h.width) * 0.30)
		detailWidth = h.width - listWidth - middleWidth
	} else {
		// Standard: 30% | 35% | 35%
		listWidth = int(float64(h.width) * 0.30)
		middleWidth = int(float64(h.width) * 0.35)
		detailWidth = h.width - listWidth - middleWidth
	}

	// Render panels based on view mode
	var listPanel, middlePanel, detailPanel string

	if h.viewMode == historyModeGit {
		// Git mode: commits on left, related beads in middle, detail on right
		listPanel = h.renderGitCommitListPanel(listWidth, panelHeight)
		middlePanel = h.renderGitBeadListPanel(middleWidth, panelHeight)
		detailPanel = h.renderGitDetailPanel(detailWidth, panelHeight)
	} else {
		// Bead mode: beads on left, commits in middle, detail on right
		listPanel = h.renderListPanel(listWidth, panelHeight)
		middlePanel = h.renderCommitMiddlePanel(middleWidth, panelHeight)
		detailPanel = h.renderDetailPanel(detailWidth, panelHeight)
	}

	// Combine panels
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, middlePanel, detailPanel)

	return lipgloss.JoinVertical(lipgloss.Left, header, panels)
}

// buildTimeline creates timeline entries from a bead's history (bv-1x6o)
func (h *HistoryModel) buildTimeline(hist correlation.BeadHistory) []TimelineEntry {
	var entries []TimelineEntry

	// Add lifecycle events from milestones (more reliable than Events slice)
	if hist.Milestones.Created != nil {
		entries = append(entries, TimelineEntry{
			Timestamp: hist.Milestones.Created.Timestamp,
			EntryType: timelineEntryEvent,
			Label:     "○ Created",
			Detail:    hist.Title,
			EventType: "created",
		})
	}
	if hist.Milestones.Claimed != nil {
		entries = append(entries, TimelineEntry{
			Timestamp: hist.Milestones.Claimed.Timestamp,
			EntryType: timelineEntryEvent,
			Label:     "● Claimed",
			Detail:    fmt.Sprintf("by %s", hist.Milestones.Claimed.Author),
			EventType: "claimed",
		})
	}
	if hist.Milestones.Reopened != nil {
		entries = append(entries, TimelineEntry{
			Timestamp: hist.Milestones.Reopened.Timestamp,
			EntryType: timelineEntryEvent,
			Label:     "↻ Reopened",
			Detail:    "",
			EventType: "reopened",
		})
	}
	if hist.Milestones.Closed != nil {
		entries = append(entries, TimelineEntry{
			Timestamp: hist.Milestones.Closed.Timestamp,
			EntryType: timelineEntryEvent,
			Label:     "✓ Closed",
			Detail:    "",
			EventType: "closed",
		})
	}

	// Add commits
	for _, commit := range hist.Commits {
		entries = append(entries, TimelineEntry{
			Timestamp:  commit.Timestamp,
			EntryType:  timelineEntryCommit,
			Label:      commit.ShortSHA,
			Detail:     commit.Message,
			Confidence: commit.Confidence,
		})
	}

	// Add sessions from cache if available (bv-pr1l)
	if sessions, ok := h.sessionCache[hist.BeadID]; ok {
		for _, session := range sessions {
			entries = append(entries, TimelineEntry{
				Timestamp:           session.Timestamp,
				EntryType:           timelineEntrySession,
				Label:               fmt.Sprintf("📎 %s session", capitalizeFirst(session.Agent)),
				Detail:              session.Title,
				SessionAgent:        session.Agent,
				SessionMessageCount: 0, // Message count not available from SearchResult
				SessionPath:         session.SourcePath,
				SessionScore:        session.FinalScore,
			})
		}
	}

	// Sort chronologically, with commits before sessions on timestamp ties
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Timestamp.Equal(entries[j].Timestamp) {
			// On ties: events first, then commits, then sessions
			return entries[i].EntryType < entries[j].EntryType
		}
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries
}

// capitalizeFirst capitalizes the first letter of a string (bv-pr1l)
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatTimelineTimestamp formats a timestamp for the timeline (bv-1x6o)
func (h *HistoryModel) formatTimelineTimestamp(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < 24*time.Hour:
		return t.Format("3:04 PM")
	case diff < 7*24*time.Hour:
		return t.Format("Mon 3PM")
	case diff < 365*24*time.Hour:
		return t.Format("Jan 2")
	default:
		return t.Format("Jan '06")
	}
}

// renderTimelinePanel renders the timeline visualization panel (bv-1x6o)
func (h *HistoryModel) renderTimelinePanel(width, height int) string {
	t := h.theme

	// Get selected bead
	if len(h.beadIDs) == 0 || h.selectedBead >= len(h.beadIDs) {
		content := lipgloss.NewStyle().Foreground(t.Secondary).Render("Select a bead to view timeline")
		return RenderTitledPanel(content, PanelOpts{
			Title:  "TIMELINE",
			Width:  width,
			Height: height,
		})
	}

	beadID := h.beadIDs[h.selectedBead]
	hist, ok := h.report.Histories[beadID]
	if !ok {
		content := lipgloss.NewStyle().Foreground(t.Secondary).Render("No history data")
		return RenderTitledPanel(content, PanelOpts{
			Title:  "TIMELINE",
			Width:  width,
			Height: height,
		})
	}

	// Build timeline entries
	entries := h.buildTimeline(hist)

	var b strings.Builder

	if len(entries) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.Secondary).Render("No events recorded"))
	} else {
		// Render timeline entries
		maxVisible := height - 4 // Account for borders and summary line
		if maxVisible < 3 {
			maxVisible = 3
		}

		// Apply scroll offset
		startIdx := h.timelineScrollOffset
		if startIdx > len(entries)-maxVisible {
			startIdx = len(entries) - maxVisible
		}
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(entries) {
			endIdx = len(entries)
		}

		// Timeline line style
		lineColor := t.Border

		for i := startIdx; i < endIdx; i++ {
			entry := entries[i]
			b.WriteString("\n")

			// Timestamp on left
			timestamp := h.formatTimelineTimestamp(entry.Timestamp)
			timestampStyle := lipgloss.NewStyle().
				Foreground(t.Subtext).
				Width(8).
				Align(lipgloss.Right)
			b.WriteString(timestampStyle.Render(timestamp))

			// Vertical line
			b.WriteString(lipgloss.NewStyle().Foreground(lineColor).Render(" ┃ "))

			// Entry content
			switch entry.EntryType {
			case timelineEntryEvent:
				// Event marker with appropriate color
				var eventColor color.Color
				switch entry.EventType {
				case "created":
					eventColor = t.Secondary
				case "claimed":
					eventColor = t.InProgress
				case "closed":
					eventColor = t.Closed
				case "reopened":
					eventColor = t.Open
				default:
					eventColor = t.Secondary
				}
				eventStyle := lipgloss.NewStyle().Foreground(eventColor).Bold(true)
				b.WriteString(eventStyle.Render(entry.Label))
				if entry.Detail != "" {
					b.WriteString(" ")
					detailStyle := lipgloss.NewStyle().Foreground(t.Subtext)
					// Truncate detail if needed
					maxDetail := width - 22
					detail := truncateRunesHelper(entry.Detail, maxDetail, "...")
					b.WriteString(detailStyle.Render(detail))
				}

			case timelineEntryCommit:
				// Commit with confidence coloring
				var confColor color.Color
				if entry.Confidence >= 0.8 {
					confColor = t.Closed // Green for high confidence
				} else if entry.Confidence >= 0.5 {
					confColor = t.InProgress // Yellow for medium
				} else {
					confColor = t.Subtext // Gray for low
				}
				shaStyle := lipgloss.NewStyle().Foreground(confColor).Bold(true)
				b.WriteString("├─ ")
				b.WriteString(shaStyle.Render(entry.Label))
				b.WriteString(" ")

				// Confidence percentage
				confPct := int(entry.Confidence * 100)
				confStyle := lipgloss.NewStyle().Foreground(confColor)
				b.WriteString(confStyle.Render(fmt.Sprintf("%d%%", confPct)))

				// Truncate message (UTF-8 safe using runewidth)
				maxMsg := width - 28
				msg := strings.Split(entry.Detail, "\n")[0] // First line only
				msg = truncateRunesHelper(msg, maxMsg, "...")
				if msg != "" {
					b.WriteString("\n")
					b.WriteString(timestampStyle.Render(""))
					b.WriteString(lipgloss.NewStyle().Foreground(lineColor).Render(" ┃   "))
					msgStyle := lipgloss.NewStyle().Foreground(t.Subtext).Italic(true)
					b.WriteString(msgStyle.Render(msg))
				}

			case timelineEntrySession:
				// Session entry with score-based coloring (bv-pr1l)
				var sessionColor color.Color
				if entry.SessionScore >= 80 {
					sessionColor = t.Primary // High relevance - primary color
				} else if entry.SessionScore >= 50 {
					sessionColor = t.InProgress // Medium relevance
				} else {
					sessionColor = t.Subtext // Lower relevance
				}
				sessionStyle := lipgloss.NewStyle().Foreground(sessionColor).Bold(true)
				b.WriteString(sessionStyle.Render(entry.Label))

				// Show detail (session title) if available
				if entry.Detail != "" {
					b.WriteString("\n")
					b.WriteString(timestampStyle.Render(""))
					b.WriteString(lipgloss.NewStyle().Foreground(lineColor).Render(" ┃   "))
					// Truncate title if needed
					maxTitle := width - 16
					title := truncateRunesHelper(entry.Detail, maxTitle, "...")
					titleStyle := lipgloss.NewStyle().Foreground(t.Subtext).Italic(true)
					b.WriteString(titleStyle.Render(title))
				}
			}
		}

		// Scroll indicator if needed
		if len(entries) > maxVisible {
			b.WriteString("\n")
			scrollInfo := fmt.Sprintf("↕ %d-%d of %d", startIdx+1, endIdx, len(entries))
			scrollStyle := lipgloss.NewStyle().Foreground(t.Subtext).Italic(true)
			// Pad for timestamp column alignment
			b.WriteString(lipgloss.NewStyle().Width(8).Render(""))
			b.WriteString(lipgloss.NewStyle().Foreground(lineColor).Render(" ┃ "))
			b.WriteString(scrollStyle.Render(scrollInfo))
		}
	}

	// Add cycle time summary at bottom if available
	if hist.CycleTime != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(t.Border).Render(strings.Repeat("─", width-6)))
		b.WriteString("\n")

		summaryStyle := lipgloss.NewStyle().Foreground(t.Subtext)
		if hist.CycleTime.CreateToClose != nil {
			b.WriteString(summaryStyle.Render(fmt.Sprintf("Cycle: %s", formatDuration(*hist.CycleTime.CreateToClose))))
		}
		if len(hist.Commits) > 0 {
			avgConf := 0.0
			for _, c := range hist.Commits {
				avgConf += c.Confidence
			}
			avgConf /= float64(len(hist.Commits))
			b.WriteString(summaryStyle.Render(fmt.Sprintf(" │ %d commits (avg %d%%)", len(hist.Commits), int(avgConf*100))))
		}
	}

	return RenderTitledPanel(b.String(), PanelOpts{
		Title:      "TIMELINE",
		RightLabel: beadID,
		Width:      width,
		Height:     height,
	})
}

// formatDuration formats a duration in a human-readable way (bv-1x6o)
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d"
	}
	return fmt.Sprintf("%dd", days)
}

// renderCompactTimeline generates a single-line timeline visualization (bv-1x6o)
// Example: ○──●──├──├──├──✓  5d cycle, 3 commits
func (h *HistoryModel) renderCompactTimeline(hist correlation.BeadHistory, maxWidth int) string {
	t := h.theme

	var markers []string
	var startTime, endTime time.Time

	// Add event markers
	if hist.Milestones.Created != nil {
		markers = append(markers, "○")
		startTime = hist.Milestones.Created.Timestamp
	}
	if hist.Milestones.Claimed != nil {
		markers = append(markers, "●")
		if startTime.IsZero() {
			startTime = hist.Milestones.Claimed.Timestamp
		}
	}

	// Add commit markers (limited to avoid overflow)
	commitCount := len(hist.Commits)
	maxCommitMarkers := 5
	if commitCount > maxCommitMarkers {
		// Show first few + ellipsis indicator
		for i := 0; i < maxCommitMarkers-1; i++ {
			markers = append(markers, "├")
		}
		markers = append(markers, "…")
	} else {
		for i := 0; i < commitCount; i++ {
			markers = append(markers, "├")
		}
	}

	// Add close marker
	if hist.Milestones.Closed != nil {
		markers = append(markers, "✓")
		endTime = hist.Milestones.Closed.Timestamp
	}

	if len(markers) == 0 {
		return lipgloss.NewStyle().Foreground(t.Subtext).Render("(no timeline data)")
	}

	// Build the timeline string
	timeline := strings.Join(markers, "──")

	// Add summary info
	var summary []string
	if hist.CycleTime != nil && hist.CycleTime.CreateToClose != nil {
		summary = append(summary, formatDuration(*hist.CycleTime.CreateToClose)+" cycle")
	}
	if commitCount > 0 {
		if commitCount == 1 {
			summary = append(summary, "1 commit")
		} else {
			summary = append(summary, fmt.Sprintf("%d commits", commitCount))
		}
	}

	result := timeline
	if len(summary) > 0 {
		result += "  " + strings.Join(summary, ", ")
	}

	// Add date range if we have both
	if !startTime.IsZero() && !endTime.IsZero() {
		dateRange := fmt.Sprintf("%s ─ %s",
			startTime.Format("Jan 2"),
			endTime.Format("Jan 2"))
		// Only add date range if we have room
		if len(result)+len(dateRange)+4 < maxWidth {
			result += "\n" + lipgloss.NewStyle().Foreground(t.Subtext).Render(dateRange)
		}
	}

	// Truncate if needed (UTF-8 safe using runewidth)
	result = truncateRunesHelper(result, maxWidth, "...")

	return result
}

// renderEmpty renders an empty state message
func (h *HistoryModel) renderEmpty(msg string) string {
	t := h.theme
	style := lipgloss.NewStyle().
		Width(h.width).
		Height(h.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(t.Secondary)

	return style.Render(msg + "\n\nPress h to close")
}

// emptyStateMessage builds the message shown when the History view has no
// commits to render. The fallback case (cwd is a git tree, but no commits
// matched) returns the caller's default. The interesting cases are when bt
// was launched outside any git tree (bt-ezk8): explain why and what the
// user can do.
//
// The History data layer needs git context for `git log`; the shared Dolt
// server hands bt project DB names but no filesystem-path mapping, so a
// global-mode launch from $HOME has no path to fall back to. The clean
// architectural fix is to migrate the correlator to dolt_log + dolt_history_issues
// (tracked by bt-08sh); until then this empty state explains the constraint
// instead of pretending the project simply has no commits.
func (h *HistoryModel) emptyStateMessage(defaultMsg string) string {
	if h.report == nil {
		return defaultMsg
	}
	// Inside a git work tree -> the data path actually ran, the report is
	// just empty for legitimate reasons. Use the caller's default.
	if h.report.RepoStatus.InsideWorkTree {
		return defaultMsg
	}

	// Cursor-driven path: most specific signal, fires first.
	if h.context.CursorPrefix != "" {
		return fmt.Sprintf(
			"History needs a git repository.\n\n"+
				"No registered path for project %q. Launch bt from\n"+
				"inside that project's git repo once to register it:\n\n"+
				"    cd <path-to-project> && bt",
			h.context.CursorPrefix)
	}

	// Outside a git work tree. Tailor the message by whether a project
	// filter is active in workspace/global mode.
	switch {
	case h.context.WorkspaceMode && len(h.context.ActiveProjects) == 1:
		project := h.context.ActiveProjects[0]
		return fmt.Sprintf(
			"History needs a git repository.\n\n"+
				"Project filter %q is active, but bt cannot map a project\n"+
				"DB name to a filesystem path. To see history for %q,\n"+
				"launch bt from inside that project's git repo:\n\n"+
				"    cd <path-to-project> && bt",
			project, project)
	case h.context.WorkspaceMode && len(h.context.ActiveProjects) > 1:
		return "History needs a single-project git context.\n\n" +
			"Filter to a single project via [w] or move the cursor\n" +
			"onto a specific bead to scope history to that project."
	case h.context.WorkspaceMode:
		return "History needs a git repository.\n\n" +
			"Select a bead or apply a single-project filter via [w],\n" +
			"and launch bt from inside that project's git repo to see\n" +
			"history."
	default:
		return "History needs a git repository.\n\n" +
			"Run bt from inside a git repo to see commit history."
	}
}

// renderHeader renders the filter bar, statistics, and title (bv-y5sx)
func (h *HistoryModel) renderHeader() string {
	t := h.theme

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Padding(0, 1)

	// Show view mode indicator with icons (bv-tl3n, bv-kvlx)
	// Icons: ◉ for git-centric (commits), ◈ for bead-centric (beads)
	var modeIcon, modeLabel string
	if h.viewMode == historyModeGit {
		modeIcon = "◉"
		modeLabel = "Git"
	} else {
		modeIcon = "◈"
		modeLabel = "Beads"
	}

	// Check for transition flash effect (bv-kvlx)
	// Show highlight for 150ms after mode toggle.
	// Note: Since Bubble Tea only re-renders on Update, the flash shows during the
	// render immediately after toggle, and clears on subsequent renders. If no user
	// input occurs, the flash persists until next action - this is acceptable TUI behavior.
	isTransitioning := !h.modeChangedAt.IsZero() && time.Since(h.modeChangedAt) <= 150*time.Millisecond

	modeStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	if isTransitioning {
		// Flash effect: bright background with contrasting text
		modeStyle = modeStyle.
			Background(t.Primary).
			Foreground(ColorBgContrast)
	} else {
		// Normal mode: just colored text
		modeStyle = modeStyle.Foreground(t.InProgress)
	}

	modeIndicator := fmt.Sprintf("%s %s", modeIcon, modeLabel)
	title := titleStyle.Render("History") + modeStyle.Render(modeIndicator)

	// Search input or close hint (bv-nkrj)
	// Renders inline on the title row in both states. No external border —
	// the bordered box added 2 rows of height and shifted the entire view
	// when search activated (bt-wyut). Both states are now exactly 1 row.
	var rightContent string
	if h.searchActive {
		modeStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		escStyle := lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 1)

		modeLabel := modeStyle.Render(fmt.Sprintf("[%s] ", h.GetSearchModeName()))
		inputView := h.searchInput.View()
		escHint := escStyle.Render("[Esc] cancel")

		rightContent = modeLabel + inputView + escHint
	} else {
		// Show close hint and search hint
		rightContent = lipgloss.NewStyle().
			Foreground(t.Muted).
			Padding(0, 1).
			Render("[/] search  [h] close")
	}

	// Combine title line with spacing
	titleLineSpacerWidth := h.width - lipgloss.Width(title) - lipgloss.Width(rightContent)
	if titleLineSpacerWidth < 1 {
		titleLineSpacerWidth = 1
	}
	titleLineSpacer := strings.Repeat(" ", titleLineSpacerWidth)
	titleLine := lipgloss.JoinHorizontal(lipgloss.Top, title, titleLineSpacer, rightContent)

	// Build stats line (bv-y5sx)
	statsLine := h.renderStatsLine()

	// Build filter status line (bv-y5sx)
	filterLine := h.renderFilterLine()

	// Add separator line
	separatorWidth := h.width
	if separatorWidth < 1 {
		separatorWidth = 1
	}
	separator := lipgloss.NewStyle().
		Foreground(t.Muted).
		Width(h.width).
		Render(strings.Repeat("─", separatorWidth))

	return lipgloss.JoinVertical(lipgloss.Left, titleLine, statsLine, filterLine, separator)
}

// renderStatsLine renders the statistics badges line (bv-y5sx)
func (h *HistoryModel) renderStatsLine() string {
	if h.report == nil {
		return ""
	}

	t := h.theme
	stats := h.report.Stats

	// Badge style - subtle background with contrasting text
	badgeStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Padding(0, 1)

	// Value style - highlighted
	valueStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// Build stats badges
	var badges []string

	// Beads with commits
	beadsBadge := badgeStyle.Render(valueStyle.Render(fmt.Sprintf("%d", stats.BeadsWithCommits)) + " beads")
	badges = append(badges, beadsBadge)

	// Total commits
	commitsBadge := badgeStyle.Render(valueStyle.Render(fmt.Sprintf("%d", stats.TotalCommits)) + " commits")
	badges = append(badges, commitsBadge)

	// Unique authors
	authorsBadge := badgeStyle.Render(valueStyle.Render(fmt.Sprintf("%d", stats.UniqueAuthors)) + " authors")
	badges = append(badges, authorsBadge)

	// Average cycle time (if available)
	if stats.AvgCycleTimeDays != nil {
		cycleStr := formatCycleTime(*stats.AvgCycleTimeDays)
		cycleBadge := badgeStyle.Render("⌀ " + valueStyle.Render(cycleStr) + " cycle")
		badges = append(badges, cycleBadge)
	}

	// Commits per bead
	if stats.AvgCommitsPerBead > 0 {
		cpdBadge := badgeStyle.Render(valueStyle.Render(fmt.Sprintf("%.1f", stats.AvgCommitsPerBead)) + " commits/bead")
		badges = append(badges, cpdBadge)
	}

	// Join with bullet separator
	separator := lipgloss.NewStyle().Foreground(t.Muted).Render(" • ")
	return strings.Join(badges, separator)
}

// renderFilterLine renders the current filter status (bv-y5sx)
func (h *HistoryModel) renderFilterLine() string {
	t := h.theme

	filterStyle := lipgloss.NewStyle().
		Foreground(t.Muted).
		Italic(true).
		Padding(0, 1)

	activeFilterStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Padding(0, 1)

	var parts []string

	// Build active filters list
	var activeFilters []string
	if h.authorFilter != "" {
		activeFilters = append(activeFilters, fmt.Sprintf("@%s", h.authorFilter))
	}
	if h.minConfidence > 0 {
		activeFilters = append(activeFilters, fmt.Sprintf("≥%.0f%% conf", h.minConfidence*100))
	}
	if h.searchActive && h.searchInput.Value() != "" {
		activeFilters = append(activeFilters, fmt.Sprintf("\"%s\"", h.searchInput.Value()))
	}

	// Show filter status
	if len(activeFilters) > 0 {
		parts = append(parts, activeFilterStyle.Render("Filter: "+strings.Join(activeFilters, ", ")))
	}

	// Show count based on mode
	if h.viewMode == historyModeGit {
		commits := h.GetFilteredCommitList()
		totalCommits := len(h.commitList)
		if len(commits) != totalCommits {
			parts = append(parts, filterStyle.Render(fmt.Sprintf("Showing %d/%d commits", len(commits), totalCommits)))
		} else {
			parts = append(parts, filterStyle.Render(fmt.Sprintf("Showing all %d commits", totalCommits)))
		}
	} else {
		totalBeads := 0
		if h.report != nil {
			totalBeads = len(h.report.Histories)
		}
		if len(h.histories) != totalBeads {
			parts = append(parts, filterStyle.Render(fmt.Sprintf("Showing %d/%d beads", len(h.histories), totalBeads)))
		} else {
			parts = append(parts, filterStyle.Render(fmt.Sprintf("Showing all %d beads with commits", len(h.histories))))
		}
	}

	return strings.Join(parts, "  │  ")
}

// formatCycleTime formats cycle time in days to a human-readable string
func formatCycleTime(days float64) string {
	if days < 1 {
		hours := days * 24
		if hours < 1 {
			return fmt.Sprintf("%.0fm", hours*60)
		}
		return fmt.Sprintf("%.1fh", hours)
	}
	if days < 7 {
		return fmt.Sprintf("%.1fd", days)
	}
	weeks := days / 7
	return fmt.Sprintf("%.1fw", weeks)
}

// renderListPanel renders the left panel with bead list
func (h *HistoryModel) renderListPanel(width, height int) string {
	visibleItems := height - 2 // inner height: title is in the border chrome
	if visibleItems < 1 {
		visibleItems = 1
	}

	var content string
	if len(h.histories) == 0 {
		// Filtered-to-zero state. The genuinely-empty case is intercepted in
		// View() and renders a full-screen empty (bt-z63i). Reaching here
		// means data exists but filters narrowed the list to nothing — keep
		// the chrome and tell the user inline.
		t := h.theme
		content = lipgloss.NewStyle().Foreground(t.Muted).Italic(true).Render("No matches")
	} else {
		var lines []string
		for i := h.scrollOffset; i < len(h.histories) && i < h.scrollOffset+visibleItems; i++ {
			hist := h.histories[i]
			line := h.renderBeadLine(i, hist, width-4)
			lines = append(lines, line)
		}
		content = strings.Join(lines, "\n")
	}

	return RenderTitledPanel(content, PanelOpts{
		Title:   "BEADS WITH HISTORY",
		Width:   width,
		Height:  height,
		Focused: h.focused == historyFocusList,
	})
}

// renderBeadLine renders a single bead in the list
func (h *HistoryModel) renderBeadLine(idx int, hist correlation.BeadHistory, width int) string {
	t := h.theme

	selected := idx == h.selectedBead

	// Indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// Status icon
	statusIcon := "○"
	switch hist.Status {
	case "closed":
		statusIcon = "✓"
	case "in_progress":
		statusIcon = "●"
	}

	// Commit count
	commitCount := fmt.Sprintf("%d commits", len(hist.Commits))

	// Event count badge (bv-7k8p) - shows lifecycle events if any
	eventBadge := ""
	if len(hist.Events) > 0 {
		eventBadge = renderCompactEventBadge(len(hist.Events), t)
	}

	// Calculate space for event badge
	eventBadgeWidth := lipgloss.Width(eventBadge)
	if eventBadgeWidth > 0 {
		eventBadgeWidth += 1 // Space before badge
	}

	// Field-priority layout: indicator + status icon + bead ID are the
	// always-shown core. commitCount and title are dropped progressively
	// as the pane narrows (bt-peo7). Without progressive drop, narrow
	// panes (e.g., 20% allocation in 4-pane wide layout) used to render a
	// fixed-width line wider than the box, which RenderTitledPanel then
	// truncated mid-ID and produced the "dotfil" stub.
	const idDisplayWidth = 12
	coreWidth := len(indicator) + len(statusIcon) + 1 + idDisplayWidth // +1 for space after icon
	remaining := width - coreWidth - 1                                  // -1 for trailing space safety

	showCount := remaining >= len(commitCount)+eventBadgeWidth+1
	titleBudget := 0
	if showCount {
		titleBudget = remaining - len(commitCount) - eventBadgeWidth - 2 // 2 spaces between fields
	} else {
		titleBudget = remaining
	}
	if titleBudget < 0 {
		titleBudget = 0
	}

	title := hist.Title
	if titleBudget == 0 {
		title = ""
	} else if len(title) > titleBudget {
		title = title[:titleBudget-1] + "…"
	}

	// Build line
	idStyle := lipgloss.NewStyle().Foreground(t.Secondary).Width(idDisplayWidth)
	countStyle := lipgloss.NewStyle().Foreground(t.Muted).Align(lipgloss.Right)

	if selected && h.focused == historyFocusList {
		idStyle = idStyle.Bold(true).Foreground(t.Primary)
	}

	parts := []string{indicator + statusIcon + " " + idStyle.Render(hist.BeadID)}

	if title != "" {
		titleStyle := lipgloss.NewStyle()
		if selected && h.focused == historyFocusList {
			titleStyle = titleStyle.Bold(true)
		}
		parts = append(parts, titleStyle.Render(title))
	}

	if showCount {
		countPart := countStyle.Render(commitCount)
		if eventBadge != "" {
			countPart = countPart + " " + eventBadge
		}
		parts = append(parts, countPart)
	}

	return strings.Join(parts, " ")
}

// renderFileTreePanel renders the file tree panel (bv-190l)
func (h *HistoryModel) renderFileTreePanel(width, height int) string {
	t := h.theme

	// Panel border style based on focus
	borderColor := t.Muted
	if h.fileTreeFocus {
		borderColor = t.Primary
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2)

	// Header with filter indicator
	headerText := "FILES"
	if h.fileFilter != "" {
		headerText = fmt.Sprintf("FILES [%s]", truncate(h.fileFilter, 15))
	}
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(width - 4)
	header := headerStyle.Render(headerText)

	var lines []string
	lines = append(lines, header)
	sepWidth := width - 4
	if sepWidth < 1 {
		sepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", sepWidth))

	// Build flat file list if needed
	if len(h.flatFileList) == 0 && len(h.fileTree) > 0 {
		h.rebuildFlatFileList()
	}

	visibleItems := height - 5
	if visibleItems < 1 {
		visibleItems = 1
	}

	// Adjust scroll to keep selection visible
	if h.selectedFileIdx < h.fileTreeScroll {
		h.fileTreeScroll = h.selectedFileIdx
	}
	if h.selectedFileIdx >= h.fileTreeScroll+visibleItems {
		h.fileTreeScroll = h.selectedFileIdx - visibleItems + 1
	}

	for i := h.fileTreeScroll; i < len(h.flatFileList) && i < h.fileTreeScroll+visibleItems; i++ {
		node := h.flatFileList[i]
		line := h.renderFileTreeLine(i, node, width-4)
		lines = append(lines, line)
	}

	// Pad with empty lines
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(content)
}

// renderFileTreeLine renders a single file tree node (bv-190l)
func (h *HistoryModel) renderFileTreeLine(idx int, node *FileTreeNode, width int) string {
	t := h.theme

	selected := idx == h.selectedFileIdx
	isFiltered := node.Path == h.fileFilter

	// Indentation
	indent := strings.Repeat("  ", node.Level)

	// Indicator
	indicator := "  "
	if selected && h.fileTreeFocus {
		indicator = "▸ "
	}

	// Expand/collapse icon for directories
	icon := "  "
	if node.IsDir {
		if node.Expanded {
			icon = "▼ "
		} else {
			icon = "▶ "
		}
	} else {
		icon = "  "
	}

	// Change count
	countStr := fmt.Sprintf("(%d)", node.ChangeCount)

	// Calculate max name length
	maxNameLen := width - len(indent) - len(indicator) - len(icon) - len(countStr) - 2
	if maxNameLen < 5 {
		maxNameLen = 5
	}

	name := node.Name
	if len(name) > maxNameLen {
		name = name[:maxNameLen-1] + "…"
	}

	// Styling
	nameStyle := lipgloss.NewStyle()
	countStyle := lipgloss.NewStyle().Foreground(t.Muted)

	if node.IsDir {
		nameStyle = nameStyle.Foreground(t.Secondary)
	}
	if isFiltered {
		nameStyle = nameStyle.Bold(true).Foreground(t.Closed) // Green for active filter
	}
	if selected && h.fileTreeFocus {
		nameStyle = nameStyle.Bold(true)
		if !isFiltered {
			nameStyle = nameStyle.Foreground(t.Primary)
		}
	}

	line := fmt.Sprintf("%s%s%s%s %s",
		indent,
		indicator,
		icon,
		nameStyle.Render(name),
		countStyle.Render(countStr),
	)

	return line
}

// renderDetailPanel renders the right panel with commit details (bv-9fk1 enhanced)
func (h *HistoryModel) renderDetailPanel(width, height int) string {
	t := h.theme

	hist := h.SelectedHistory()
	if hist == nil {
		return RenderTitledPanel("No bead selected", PanelOpts{
			Title:   "COMMIT DETAILS",
			Width:   width,
			Height:  height,
			Focused: h.focused == historyFocusDetail,
		})
	}

	// Bead info with status indicator
	statusIcon := "○"
	switch hist.Status {
	case "closed":
		statusIcon = "✓"
	case "in_progress":
		statusIcon = "●"
	}
	beadInfo := fmt.Sprintf("%s %s: %s", statusIcon, hist.BeadID, hist.Title)
	if width > 10 && len(beadInfo) > width-6 {
		beadInfo = beadInfo[:width-7] + "…"
	} else if width <= 10 && len(beadInfo) > 5 {
		beadInfo = beadInfo[:4] + "…"
	}
	beadInfoStyle := lipgloss.NewStyle().Foreground(t.Secondary)

	// Build header (always shown, fixed at top); title is in the border chrome
	detailSepWidth := width - 4
	if detailSepWidth < 1 {
		detailSepWidth = 1
	}
	headerLines := []string{
		beadInfoStyle.Render(beadInfo),
		strings.Repeat("─", detailSepWidth),
	}

	// Build scrollable content region (events + commits)
	var contentLines []string

	// === LIFECYCLE EVENTS SECTION (bv-7k8p) ===
	// Show compact timeline of lifecycle events if available
	if len(hist.Events) > 0 {
		// Limit events section to ~4 lines to leave room for commits
		maxEventLines := 5
		eventLines := h.renderEventsSection(hist.Events, width-4, maxEventLines)
		contentLines = append(contentLines, eventLines...)
		contentLines = append(contentLines, "") // Spacer before commits
	}

	// Render commits
	for i, commit := range hist.Commits {
		isSelected := i == h.selectedCommit && h.focused == historyFocusDetail
		commitLines := h.renderCommitDetail(commit, width-4, isSelected)
		contentLines = append(contentLines, commitLines...)
		if i < len(hist.Commits)-1 {
			contentLines = append(contentLines, "") // Spacer between commits
		}
	}

	// Calculate aggregate stats for footer (bv-9fk1)
	var totalFiles, totalAdd, totalDel int
	var totalConf float64
	uniqueFiles := make(map[string]bool)
	for _, commit := range hist.Commits {
		totalConf += commit.Confidence
		for _, f := range commit.Files {
			uniqueFiles[f.Path] = true
			totalAdd += f.Insertions
			totalDel += f.Deletions
		}
	}
	totalFiles = len(uniqueFiles)
	avgConf := 0.0
	if len(hist.Commits) > 0 {
		avgConf = totalConf / float64(len(hist.Commits))
	}

	// Build footer (always shown, fixed at bottom)
	statsStyle := lipgloss.NewStyle().Foreground(t.Muted)
	confStyle := lipgloss.NewStyle()
	switch {
	case avgConf >= 0.8:
		confStyle = confStyle.Foreground(t.Open)
	case avgConf >= 0.5:
		confStyle = confStyle.Foreground(t.Secondary)
	default:
		confStyle = confStyle.Foreground(t.Muted)
	}

	var statsItems []string
	statsItems = append(statsItems, fmt.Sprintf("%d commits", len(hist.Commits)))
	statsItems = append(statsItems, fmt.Sprintf("%d files", totalFiles))
	if totalAdd > 0 || totalDel > 0 {
		addStr := lipgloss.NewStyle().Foreground(t.Open).Render(fmt.Sprintf("+%d", totalAdd))
		delStr := lipgloss.NewStyle().Foreground(t.Closed).Render(fmt.Sprintf("-%d", totalDel))
		statsItems = append(statsItems, addStr+"/"+delStr)
	}
	statsItems = append(statsItems, confStyle.Render(fmt.Sprintf("%.0f%% avg", avgConf*100)))

	statsLine := statsStyle.Render(strings.Join(statsItems, " • "))

	// Navigation hint (bv-xf4p: added o and g keys; bt-npnh: scroll hint)
	hintStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
	hintText := "J/K:nav  y:copy  o:open  g:graph"
	if h.focused == historyFocusDetail {
		hintText = "J/K:nav  C-d/C-u:scroll  y:copy  o:open  g:graph"
	}

	footerLines := []string{
		strings.Repeat("─", detailSepWidth),
		statsLine,
		hintStyle.Render(hintText),
	}

	// Determine visible window for scrollable content.
	// Inner panel height = height - 2 (border). Subtract header + footer for content area.
	contentHeight := height - 2 - len(headerLines) - len(footerLines)
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Clamp scroll offset to valid range based on actual content size.
	maxOffset := len(contentLines) - contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if h.detailScrollOffset > maxOffset {
		h.detailScrollOffset = maxOffset
	}
	if h.detailScrollOffset < 0 {
		h.detailScrollOffset = 0
	}

	// Slice content window
	end := h.detailScrollOffset + contentHeight
	if end > len(contentLines) {
		end = len(contentLines)
	}
	visibleContent := append([]string(nil), contentLines[h.detailScrollOffset:end]...)

	// Pad content so footer always sits at the bottom even when content is short.
	for len(visibleContent) < contentHeight {
		visibleContent = append(visibleContent, "")
	}

	// Compose: header + visible content + footer
	allLines := make([]string, 0, len(headerLines)+len(visibleContent)+len(footerLines))
	allLines = append(allLines, headerLines...)
	allLines = append(allLines, visibleContent...)
	allLines = append(allLines, footerLines...)

	content := strings.Join(allLines, "\n")
	return RenderTitledPanel(content, PanelOpts{
		Title:   "COMMIT DETAILS",
		Width:   width,
		Height:  height,
		Focused: h.focused == historyFocusDetail,
	})
}

// detailContentLines computes the total number of lines in the scrollable
// content region of the detail panel for the currently-selected bead. Used by
// scroll methods to clamp offsets without rendering. (bt-npnh)
func (h *HistoryModel) detailContentLines(width int) int {
	hist := h.SelectedHistory()
	if hist == nil {
		return 0
	}
	total := 0
	if len(hist.Events) > 0 {
		// renderEventsSection caps at maxEventLines (5) plus spacer
		eventLines := h.renderEventsSection(hist.Events, width-4, 5)
		total += len(eventLines) + 1 // +1 spacer
	}
	for i, commit := range hist.Commits {
		commitLines := h.renderCommitDetail(commit, width-4, false)
		total += len(commitLines)
		if i < len(hist.Commits)-1 {
			total++ // spacer
		}
	}
	return total
}

// detailVisibleHeight returns the number of content lines visible in the detail
// panel given the current panel height. (bt-npnh)
func (h *HistoryModel) detailVisibleHeight(height int) int {
	// header = 2 lines (beadInfo + sep), footer = 3 lines, inner panel = height - 2 (border)
	v := height - 2 - 2 - 3
	if v < 1 {
		return 1
	}
	return v
}

// ScrollDetailDown scrolls the detail panel content down by one line. (bt-npnh)
func (h *HistoryModel) ScrollDetailDown() {
	h.detailScrollOffset++
	// Render-time clamp will fix overflow. We can also clamp here to keep
	// state consistent for tests that don't render.
	h.clampDetailScroll()
}

// ScrollDetailUp scrolls the detail panel content up by one line. (bt-npnh)
func (h *HistoryModel) ScrollDetailUp() {
	if h.detailScrollOffset > 0 {
		h.detailScrollOffset--
	}
}

// ScrollDetailHalfPageDown scrolls the detail panel content down by half a
// visible page. (bt-npnh)
func (h *HistoryModel) ScrollDetailHalfPageDown() {
	visible := h.detailVisibleHeight(h.height)
	h.detailScrollOffset += visible / 2
	if visible/2 == 0 {
		h.detailScrollOffset++
	}
	h.clampDetailScroll()
}

// ScrollDetailHalfPageUp scrolls the detail panel content up by half a
// visible page. (bt-npnh)
func (h *HistoryModel) ScrollDetailHalfPageUp() {
	visible := h.detailVisibleHeight(h.height)
	step := visible / 2
	if step == 0 {
		step = 1
	}
	h.detailScrollOffset -= step
	if h.detailScrollOffset < 0 {
		h.detailScrollOffset = 0
	}
}

// clampDetailScroll bounds detailScrollOffset to [0, max] using a best-effort
// estimate of total content height for the current panel width. (bt-npnh)
func (h *HistoryModel) clampDetailScroll() {
	// Use the detail panel's likely width from the current layout. The render
	// path will re-clamp using exact width, this is just a state guard.
	width := h.width
	if width <= 0 {
		// No layout yet; rely on render-time clamp.
		return
	}
	// Rough detail-pane width by layout (matches renderTwoPaneView /
	// renderThreePaneView splits). Used only for clamp; render uses exact.
	var detailWidth int
	switch h.determineLayout() {
	case layoutNarrow:
		detailWidth = width - int(float64(width)*0.45)
	case layoutWide:
		detailWidth = width - int(float64(width)*0.20) - int(float64(width)*0.22) - int(float64(width)*0.25)
	default:
		detailWidth = width - int(float64(width)*0.30) - int(float64(width)*0.35)
	}
	total := h.detailContentLines(detailWidth)
	visible := h.detailVisibleHeight(h.height)
	max := total - visible
	if max < 0 {
		max = 0
	}
	if h.detailScrollOffset > max {
		h.detailScrollOffset = max
	}
	if h.detailScrollOffset < 0 {
		h.detailScrollOffset = 0
	}
}

// renderCommitDetail renders details for a single commit (bv-9fk1 enhanced)
func (h *HistoryModel) renderCommitDetail(commit correlation.CorrelatedCommit, width int, selected bool) []string {
	t := h.theme
	var lines []string

	// Selection indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// === COMMIT HEADER ===
	// Type icon + SHA + relative time
	typeIcon := commitTypeIndicator(commit.Message)
	if typeIcon != "" {
		typeIcon += " "
	}

	shaStyle := lipgloss.NewStyle().Foreground(t.Primary)
	if selected {
		shaStyle = shaStyle.Bold(true)
	}

	relTime := relativeTime(commit.Timestamp)
	relTimeStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)

	// Header line: [indicator] [icon] SHA (relative time)
	headerLine := fmt.Sprintf("%s%s%s %s",
		indicator,
		typeIcon,
		shaStyle.Render(commit.ShortSHA),
		relTimeStyle.Render("("+relTime+")"),
	)
	lines = append(lines, headerLine)

	// === AUTHOR LINE ===
	// [Initials] Author Name • absolute date
	initials := authorInitials(commit.Author)
	initialsStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground()).
		Background(t.Muted).
		Padding(0, 1).
		Bold(true)
	authorStyle := lipgloss.NewStyle().Foreground(t.Secondary)
	dateStr := commit.Timestamp.Format("2006-01-02 15:04")

	authorLine := fmt.Sprintf("    %s %s • %s",
		initialsStyle.Render(initials),
		authorStyle.Render(commit.Author),
		dateStr,
	)
	// Use lipgloss.Width for accurate visual width (handles ANSI escape codes)
	if width > 10 && lipgloss.Width(authorLine) > width-2 {
		// Truncate author name if needed
		maxAuthor := width - 30
		if maxAuthor < 10 {
			maxAuthor = 10
		}
		authorName := commit.Author
		if len(authorName) > maxAuthor {
			authorName = authorName[:maxAuthor-1] + "…"
		}
		authorLine = fmt.Sprintf("    %s %s • %s",
			initialsStyle.Render(initials),
			authorStyle.Render(authorName),
			dateStr,
		)
	}
	lines = append(lines, authorLine)

	// === MESSAGE ===
	// Parse conventional commit for better display
	cc := parseConventionalCommit(commit.Message)

	if cc.IsConventional {
		// Show type badge + subject
		typeBadgeStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		var scopeStr string
		if cc.Scope != "" {
			scopeStr = "(" + cc.Scope + ")"
		}
		breakingStr := ""
		if cc.Breaking {
			breakingStr = lipgloss.NewStyle().Foreground(t.Closed).Bold(true).Render("!")
		}
		typeLine := fmt.Sprintf("    %s%s%s: %s",
			typeBadgeStyle.Render(cc.Type),
			scopeStr,
			breakingStr,
			truncate(cc.Subject, width-len(cc.Type)-len(scopeStr)-10),
		)
		lines = append(lines, typeLine)
	} else {
		// Non-conventional: just show the message
		msgLine := fmt.Sprintf("    %s", truncate(cc.Subject, width-6))
		lines = append(lines, msgLine)
	}

	// === CONFIDENCE & METHOD ===
	confStyle := lipgloss.NewStyle()
	switch {
	case commit.Confidence >= 0.8:
		confStyle = confStyle.Foreground(t.Open) // Green
	case commit.Confidence >= 0.5:
		confStyle = confStyle.Foreground(t.Secondary) // Yellow/neutral
	default:
		confStyle = confStyle.Foreground(t.Muted) // Gray
	}

	methodStr := methodLabel(commit.Method)
	confLine := fmt.Sprintf("    %s %s",
		confStyle.Render(fmt.Sprintf("%.0f%% confidence", commit.Confidence*100)),
		lipgloss.NewStyle().Foreground(t.Muted).Render(methodStr),
	)
	lines = append(lines, confLine)

	// === FILE CHANGES ===
	if len(commit.Files) > 0 {
		// File summary header
		var totalAdd, totalDel int
		for _, f := range commit.Files {
			totalAdd += f.Insertions
			totalDel += f.Deletions
		}

		fileSummary := fmt.Sprintf("    %d file(s)",
			len(commit.Files),
		)
		if totalAdd > 0 || totalDel > 0 {
			addStyle := lipgloss.NewStyle().Foreground(t.Open)
			delStyle := lipgloss.NewStyle().Foreground(t.Closed)
			fileSummary += fmt.Sprintf(" %s %s",
				addStyle.Render(fmt.Sprintf("+%d", totalAdd)),
				delStyle.Render(fmt.Sprintf("-%d", totalDel)),
			)
		}
		lines = append(lines, fileSummary)

		// Group files by directory and show (max 5 files)
		groups := groupFilesByDirectory(commit.Files)
		fileCount := 0
		maxFiles := 5

		for _, group := range groups {
			if fileCount >= maxFiles {
				moreCount := len(commit.Files) - fileCount
				moreStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
				lines = append(lines, moreStyle.Render(fmt.Sprintf("      +%d more files...", moreCount)))
				break
			}

			for _, f := range group.Files {
				if fileCount >= maxFiles {
					break
				}

				// Get just filename from path
				filename := f.Path
				lastSlash := strings.LastIndex(f.Path, "/")
				if lastSlash >= 0 && lastSlash < len(f.Path)-1 {
					filename = f.Path[lastSlash+1:]
				}

				// Action icon and color
				actionIcon := fileActionIcon(f.Action)
				actionColor := fileActionColor(f.Action, t)
				actionStyle := lipgloss.NewStyle().Foreground(actionColor)

				// +/- stats if available
				statsStr := ""
				if f.Insertions > 0 || f.Deletions > 0 {
					addStr := lipgloss.NewStyle().Foreground(t.Open).Render(fmt.Sprintf("+%d", f.Insertions))
					delStr := lipgloss.NewStyle().Foreground(t.Closed).Render(fmt.Sprintf("-%d", f.Deletions))
					statsStr = fmt.Sprintf(" %s/%s", addStr, delStr)
				}

				fileLine := fmt.Sprintf("      %s %s%s",
					actionStyle.Render(actionIcon),
					truncate(filename, width-15),
					statsStr,
				)
				lines = append(lines, fileLine)
				fileCount++
			}
		}
	}

	return lines
}

// Helper functions

func methodLabel(method correlation.CorrelationMethod) string {
	switch method {
	case correlation.MethodCoCommitted:
		return "(co-committed)"
	case correlation.MethodExplicitID:
		return "(explicit ID)"
	case correlation.MethodTemporalAuthor:
		return "(temporal)"
	default:
		return ""
	}
}

// Commit detail enhancement helpers (bv-9fk1)

// authorInitials extracts initials from an author name (e.g., "John Doe" -> "JD")
func authorInitials(name string) string {
	if name == "" {
		return "??"
	}
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "??"
	}
	if len(parts) == 1 {
		// Single name - take first two runes (Unicode-safe)
		runes := []rune(parts[0])
		if len(runes) >= 2 {
			return strings.ToUpper(string(runes[:2]))
		}
		return strings.ToUpper(string(runes))
	}
	// Multi-part name - first rune of first and last parts
	first := string([]rune(parts[0])[0])
	last := string([]rune(parts[len(parts)-1])[0])
	return strings.ToUpper(first + last)
}

// relativeTime formats a time as a relative string (e.g., "2d ago", "3h ago")
func relativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 0 {
		return "in future"
	}

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		return fmt.Sprintf("%dw ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		return fmt.Sprintf("%dmo ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		return fmt.Sprintf("%dy ago", years)
	}
}

// conventionalCommit holds parsed conventional commit info
type conventionalCommit struct {
	Type           string // feat, fix, docs, etc.
	Scope          string // optional scope in parentheses
	Breaking       bool   // has ! after type/scope
	Subject        string // the description after the colon
	Body           string // everything after first line
	IsConventional bool   // true if successfully parsed
}

// parseConventionalCommit parses a conventional commit message
func parseConventionalCommit(msg string) conventionalCommit {
	result := conventionalCommit{IsConventional: false}

	lines := strings.SplitN(msg, "\n", 2)
	firstLine := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		result.Body = strings.TrimSpace(lines[1])
	}

	// Match pattern: type(scope)!: description or type!: description or type: description
	// Common types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
	patterns := []string{
		"feat", "fix", "docs", "style", "refactor", "perf", "test",
		"build", "ci", "chore", "revert", "wip",
	}

	for _, prefix := range patterns {
		if strings.HasPrefix(strings.ToLower(firstLine), prefix) {
			rest := firstLine[len(prefix):]

			// Check for scope
			if strings.HasPrefix(rest, "(") {
				endParen := strings.Index(rest, ")")
				if endParen > 0 {
					result.Scope = rest[1:endParen]
					rest = rest[endParen+1:]
				}
			}

			// Check for breaking change indicator
			if strings.HasPrefix(rest, "!") {
				result.Breaking = true
				rest = rest[1:]
			}

			// Check for colon
			if strings.HasPrefix(rest, ":") {
				result.Type = prefix
				result.Subject = strings.TrimSpace(rest[1:])
				result.IsConventional = true
				return result
			}
		}
	}

	// Not conventional - use whole first line as subject
	result.Subject = firstLine
	return result
}

// commitTypeIndicator returns an icon/badge for commit type
func commitTypeIndicator(msg string) string {
	lowerMsg := strings.ToLower(msg)

	// Check for merge commit
	if strings.HasPrefix(lowerMsg, "merge ") {
		return "⊕" // merge symbol
	}

	// Check for revert
	if strings.HasPrefix(lowerMsg, "revert ") {
		return "↩" // revert symbol
	}

	// Check conventional commit type
	cc := parseConventionalCommit(msg)
	if cc.IsConventional {
		switch cc.Type {
		case "feat":
			return "✨" // sparkles for feature
		case "fix":
			return "🐛" // bug for fix
		case "docs":
			return "📝" // docs
		case "refactor":
			return "♻" // refactor
		case "perf":
			return "⚡" // performance
		case "test":
			return "🧪" // test
		case "chore":
			return "🔧" // chore
		case "ci":
			return "🔄" // CI
		case "build":
			return "📦" // build
		case "style":
			return "💄" // style
		}
	}

	return "" // no special indicator
}

// filesByDirectory groups files by their parent directory
type fileGroup struct {
	Dir   string
	Files []correlation.FileChange
}

func groupFilesByDirectory(files []correlation.FileChange) []fileGroup {
	dirMap := make(map[string][]correlation.FileChange)
	var dirOrder []string

	for _, f := range files {
		dir := "."
		lastSlash := strings.LastIndex(f.Path, "/")
		if lastSlash >= 0 {
			dir = f.Path[:lastSlash]
		}

		if _, exists := dirMap[dir]; !exists {
			dirOrder = append(dirOrder, dir)
		}
		dirMap[dir] = append(dirMap[dir], f)
	}

	var groups []fileGroup
	for _, dir := range dirOrder {
		groups = append(groups, fileGroup{
			Dir:   dir,
			Files: dirMap[dir],
		})
	}
	return groups
}

// fileActionColor returns the appropriate theme color for a file action
func fileActionColor(action string, t Theme) color.Color {
	switch action {
	case "A":
		return t.Open // green for added
	case "D":
		return t.Closed // red for deleted
	case "M":
		return t.InProgress // yellow/orange for modified
	case "R":
		return t.Secondary // neutral for renamed
	default:
		return t.Muted
	}
}

// fileActionIcon returns an icon for file action
func fileActionIcon(action string) string {
	switch action {
	case "A":
		return "+"
	case "D":
		return "-"
	case "M":
		return "~"
	case "R":
		return "→"
	default:
		return "?"
	}
}

// === Lifecycle Event Helpers (bv-7k8p) ===

// eventTypeIcon returns an icon for a lifecycle event type
func eventTypeIcon(et correlation.EventType) string {
	switch et {
	case correlation.EventCreated:
		return "🆕"
	case correlation.EventClaimed:
		return "👤"
	case correlation.EventClosed:
		return "✓"
	case correlation.EventReopened:
		return "↺"
	case correlation.EventModified:
		return "✎"
	default:
		return "•"
	}
}

// eventTypeColor returns the appropriate theme color for an event type
func eventTypeColor(et correlation.EventType, t Theme) color.Color {
	switch et {
	case correlation.EventCreated:
		return t.Primary // new items get primary highlight
	case correlation.EventClaimed:
		return t.InProgress // claimed = in progress
	case correlation.EventClosed:
		return t.Open // closed = success/green
	case correlation.EventReopened:
		return t.Secondary // reopened = warning
	case correlation.EventModified:
		return t.Muted // modifications are low-key
	default:
		return t.Muted
	}
}

// eventTypeLabel returns a human-readable label for an event type
func eventTypeLabel(et correlation.EventType) string {
	switch et {
	case correlation.EventCreated:
		return "Created"
	case correlation.EventClaimed:
		return "Claimed"
	case correlation.EventClosed:
		return "Closed"
	case correlation.EventReopened:
		return "Reopened"
	case correlation.EventModified:
		return "Modified"
	default:
		return string(et)
	}
}

// renderEventsSection renders a compact timeline of lifecycle events (bv-7k8p)
// Returns at most maxLines lines total (header + events + optional "more" line)
func (h *HistoryModel) renderEventsSection(events []correlation.BeadEvent, width int, maxLines int) []string {
	if len(events) == 0 {
		return nil
	}

	t := h.theme
	var lines []string

	// Section header (takes 1 line)
	headerStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
	lines = append(lines, headerStyle.Render(fmt.Sprintf("LIFECYCLE (%d)", len(events))))

	// Timeline style
	timeStyle := lipgloss.NewStyle().Foreground(t.Muted).Width(8)
	authorStyle := lipgloss.NewStyle().Foreground(t.Secondary)

	// Calculate how many events we can show:
	// - 1 line for header
	// - If more events exist than we can show, reserve 1 line for "+N more"
	// - Remaining lines are for events
	availableForEvents := maxLines - 1 // subtract header
	needsMoreLine := len(events) > availableForEvents
	if needsMoreLine {
		availableForEvents-- // reserve line for "+N more"
	}

	// Show most recent events first (reverse chronological for timeline)
	displayed := 0
	for i := len(events) - 1; i >= 0 && displayed < availableForEvents; i-- {
		event := events[i]

		// Event icon with color
		icon := eventTypeIcon(event.EventType)
		iconColor := eventTypeColor(event.EventType, t)
		iconStyle := lipgloss.NewStyle().Foreground(iconColor)
		coloredIcon := iconStyle.Render(icon)

		// Relative time
		timeStr := relativeTime(event.Timestamp)
		if len(timeStr) > 7 {
			timeStr = timeStr[:7]
		}

		// Author initials
		initials := authorInitials(event.Author)

		// Build event line: "│ ✓ 2d ago JD"
		// Use unicode box drawing for timeline
		connector := "│"
		if i == 0 {
			connector = "└" // Last event (first chronologically)
		}

		eventLine := fmt.Sprintf("%s %s %s %s",
			connector,
			coloredIcon,
			timeStyle.Render(timeStr),
			authorStyle.Render(initials),
		)

		// Truncate if needed
		if lipgloss.Width(eventLine) > width-2 {
			// Simplified version without author
			eventLine = fmt.Sprintf("%s %s %s",
				connector,
				coloredIcon,
				timeStyle.Render(timeStr),
			)
		}

		lines = append(lines, eventLine)
		displayed++
	}

	// Show "+N more" if we couldn't display all events
	if needsMoreLine {
		remaining := len(events) - displayed
		moreStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
		lines = append(lines, moreStyle.Render(fmt.Sprintf("  +%d more", remaining)))
	}

	return lines
}

// renderCompactEventBadge renders a compact event count badge for list items (bv-7k8p)
func renderCompactEventBadge(eventCount int, t Theme) string {
	if eventCount == 0 {
		return ""
	}

	badgeStyle := lipgloss.NewStyle().
		Foreground(t.Secondary)

	return badgeStyle.Render(fmt.Sprintf("⚡%d", eventCount))
}

// Git Mode rendering functions (bv-tl3n)

// renderGitCommitListPanel renders the left panel with commit list in git mode
func (h *HistoryModel) renderGitCommitListPanel(width, height int) string {
	visibleItems := height - 2 // inner height: title is in the border chrome
	if visibleItems < 1 {
		visibleItems = 1
	}

	var lines []string
	// Use filtered list if search is active (bv-nkrj)
	commits := h.GetFilteredCommitList()
	for i := h.gitScrollOffset; i < len(commits) && i < h.gitScrollOffset+visibleItems; i++ {
		commit := commits[i]
		line := h.renderGitCommitLine(i, commit, width-4)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return RenderTitledPanel(content, PanelOpts{
		Title:   "COMMITS",
		Width:   width,
		Height:  height,
		Focused: h.focused == historyFocusList,
	})
}

// renderGitCommitLine renders a single commit in git mode list
func (h *HistoryModel) renderGitCommitLine(idx int, commit CommitListEntry, width int) string {
	t := h.theme

	selected := idx == h.selectedGitCommit

	// Indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// Bead count badge
	beadCount := fmt.Sprintf("[%d]", len(commit.BeadIDs))

	// Truncate message
	maxMsgLen := width - len(indicator) - len(commit.ShortSHA) - len(beadCount) - 6
	if maxMsgLen < 10 {
		maxMsgLen = 10
	}
	msg := commit.Message
	if len(msg) > maxMsgLen {
		msg = msg[:maxMsgLen-1] + "…"
	}

	// Build line
	shaStyle := lipgloss.NewStyle().Foreground(t.Primary)
	msgStyle := lipgloss.NewStyle()
	countStyle := lipgloss.NewStyle().Foreground(t.Secondary)

	if selected && h.focused == historyFocusList {
		shaStyle = shaStyle.Bold(true)
		msgStyle = msgStyle.Bold(true)
	}

	line := fmt.Sprintf("%s%s %s %s",
		indicator,
		shaStyle.Render(commit.ShortSHA),
		msgStyle.Render(msg),
		countStyle.Render(beadCount),
	)

	return line
}

// renderGitDetailPanel renders the right panel with related beads and commit details in git mode
func (h *HistoryModel) renderGitDetailPanel(width, height int) string {
	t := h.theme

	commit := h.SelectedGitCommit()
	if commit == nil {
		return RenderTitledPanel("No commit selected", PanelOpts{
			Title:   "COMMIT DETAILS",
			Width:   width,
			Height:  height,
			Focused: h.focused == historyFocusDetail,
		})
	}

	var lines []string

	// Sub-header: Related Beads section
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary)
	lines = append(lines, headerStyle.Render("RELATED BEADS"))

	detailSepWidth := width - 4
	if detailSepWidth < 1 {
		detailSepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", detailSepWidth))

	// List related beads
	for i, beadID := range commit.BeadIDs {
		isSelected := i == h.selectedRelatedBead && h.focused == historyFocusDetail

		indicator := "  "
		if isSelected {
			indicator = "▸ "
		}

		// Get bead info from report
		beadStyle := lipgloss.NewStyle()
		statusIcon := "○"
		title := beadID

		if h.report != nil {
			if hist, ok := h.report.Histories[beadID]; ok {
				title = hist.Title
				switch hist.Status {
				case "closed":
					statusIcon = "✓"
				case "in_progress":
					statusIcon = "●"
				}
			}
		}

		if isSelected {
			beadStyle = beadStyle.Bold(true).Foreground(t.Primary)
		}

		// Truncate title
		maxLen := width - 8
		if maxLen < 10 {
			maxLen = 10
		}
		if len(title) > maxLen {
			title = title[:maxLen-1] + "…"
		}

		beadLine := fmt.Sprintf("%s%s %s %s", indicator, statusIcon, beadID, beadStyle.Render(title))
		lines = append(lines, beadLine)
	}

	// Separator before commit details section
	lines = append(lines, "")
	lines = append(lines, strings.Repeat("─", detailSepWidth))

	// Commit details
	shaLine := fmt.Sprintf("SHA: %s", commit.SHA)
	if width > 10 && len(shaLine) > width-6 {
		shaLine = shaLine[:width-7] + "…"
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(t.Primary).Render(shaLine))

	authorLine := fmt.Sprintf("Author: %s", commit.Author)
	if width > 10 && len(authorLine) > width-6 {
		authorLine = authorLine[:width-7] + "…"
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(t.Secondary).Render(authorLine))

	dateLine := fmt.Sprintf("Date: %s", commit.Timestamp)
	lines = append(lines, lipgloss.NewStyle().Foreground(t.Muted).Render(dateLine))

	filesLine := fmt.Sprintf("Files: %d changed", commit.FileCount)
	lines = append(lines, lipgloss.NewStyle().Foreground(t.Muted).Render(filesLine))

	// Message
	lines = append(lines, "")
	msgStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	msgLines := strings.Split(commit.Message, "\n")
	for _, ml := range msgLines {
		if width > 6 && len(ml) > width-6 {
			ml = ml[:width-7] + "…"
		}
		lines = append(lines, msgStyle.Render(ml))
	}

	// Reserve space for footer hint (bv-xf4p)
	footerHeight := 2
	contentHeight := height - 2 - footerHeight
	if contentHeight < 1 {
		contentHeight = 1 // Minimum content height to avoid negative slicing
	}

	// Pad with empty lines
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	// Truncate if too many lines
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	// Add footer hint (bv-xf4p)
	lines = append(lines, strings.Repeat("─", detailSepWidth))
	hintStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
	lines = append(lines, hintStyle.Render("J/K:bead  y:copy  o:open  g:graph"))

	content := strings.Join(lines, "\n")
	return RenderTitledPanel(content, PanelOpts{
		Title:   "COMMIT DETAILS",
		Width:   width,
		Height:  height,
		Focused: h.focused == historyFocusDetail,
	})
}

// renderCommitMiddlePanel renders commits for selected bead in middle pane (bv-xrfh)
func (h *HistoryModel) renderCommitMiddlePanel(width, height int) string {
	t := h.theme

	hist := h.SelectedHistory()
	if hist == nil {
		return RenderTitledPanel("Select a bead to view commits", PanelOpts{
			Title:   "COMMITS",
			Width:   width,
			Height:  height,
			Focused: h.focused == historyFocusMiddle,
		})
	}

	var lines []string

	visibleItems := height - 2 // inner height: title is in the border chrome
	if visibleItems < 1 {
		visibleItems = 1
	}

	// Use scroll offset for middle pane (bv-xrfh fix)
	totalCommits := len(hist.Commits)
	startIdx := h.middleScrollOffset
	if startIdx >= totalCommits {
		startIdx = 0
	}
	endIdx := startIdx + visibleItems
	if endIdx > totalCommits {
		endIdx = totalCommits
	}

	for i := startIdx; i < endIdx; i++ {
		commit := hist.Commits[i]
		isSelected := i == h.selectedCommit && h.focused == historyFocusMiddle

		indicator := "  "
		if isSelected {
			indicator = "▸ "
		}

		shaStyle := lipgloss.NewStyle().Foreground(t.Primary)
		if isSelected {
			shaStyle = shaStyle.Bold(true)
		}

		maxMsgLen := width - len(commit.ShortSHA) - 8
		if maxMsgLen < 10 {
			maxMsgLen = 10
		}
		msg := commit.Message
		if len(msg) > maxMsgLen {
			msg = msg[:maxMsgLen-1] + "…"
		}

		line := fmt.Sprintf("%s%s %s", indicator, shaStyle.Render(commit.ShortSHA), msg)
		lines = append(lines, line)
	}

	// Add scroll indicator if needed (bv-xrfh)
	if totalCommits > visibleItems {
		scrollInfo := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
		scrollPct := 0
		maxScroll := totalCommits - visibleItems
		if maxScroll > 0 {
			scrollPct = h.middleScrollOffset * 100 / maxScroll
		}
		lines = append(lines, scrollInfo.Render(fmt.Sprintf("↕ %d/%d (%d%%)", endIdx, totalCommits, scrollPct)))
	}

	content := strings.Join(lines, "\n")
	return RenderTitledPanel(content, PanelOpts{
		Title:   "COMMITS",
		Width:   width,
		Height:  height,
		Focused: h.focused == historyFocusMiddle,
	})
}

// renderGitBeadListPanel renders related beads for selected commit in middle pane (bv-xrfh)
func (h *HistoryModel) renderGitBeadListPanel(width, height int) string {
	t := h.theme

	commit := h.SelectedGitCommit()
	if commit == nil {
		return RenderTitledPanel("Select a commit to view beads", PanelOpts{
			Title:   "RELATED BEADS",
			Width:   width,
			Height:  height,
			Focused: h.focused == historyFocusMiddle,
		})
	}

	var lines []string

	visibleItems := height - 2 // inner height: title is in the border chrome
	if visibleItems < 1 {
		visibleItems = 1
	}

	// Use scroll offset for middle pane (bv-xrfh fix)
	totalBeads := len(commit.BeadIDs)
	startIdx := h.middleScrollOffset
	if startIdx >= totalBeads {
		startIdx = 0
	}
	endIdx := startIdx + visibleItems
	if endIdx > totalBeads {
		endIdx = totalBeads
	}

	for i := startIdx; i < endIdx; i++ {
		beadID := commit.BeadIDs[i]
		isSelected := i == h.selectedRelatedBead && h.focused == historyFocusMiddle

		indicator := "  "
		if isSelected {
			indicator = "▸ "
		}

		beadStyle := lipgloss.NewStyle()
		statusIcon := "○"
		title := beadID

		if h.report != nil {
			if hist, ok := h.report.Histories[beadID]; ok {
				title = hist.Title
				switch hist.Status {
				case "closed":
					statusIcon = "✓"
				case "in_progress":
					statusIcon = "●"
				}
			}
		}

		if isSelected {
			beadStyle = beadStyle.Bold(true).Foreground(t.Primary)
		}

		maxLen := width - 12
		if maxLen < 10 {
			maxLen = 10
		}
		if len(title) > maxLen {
			title = title[:maxLen-1] + "…"
		}

		beadLine := fmt.Sprintf("%s%s %s", indicator, statusIcon, beadStyle.Render(title))
		lines = append(lines, beadLine)
	}

	// Add scroll indicator if needed (bv-xrfh)
	if totalBeads > visibleItems {
		scrollInfo := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
		scrollPct := 0
		maxScroll := totalBeads - visibleItems
		if maxScroll > 0 {
			scrollPct = h.middleScrollOffset * 100 / maxScroll
		}
		lines = append(lines, scrollInfo.Render(fmt.Sprintf("↕ %d/%d (%d%%)", endIdx, totalBeads, scrollPct)))
	}

	content := strings.Join(lines, "\n")
	return RenderTitledPanel(content, PanelOpts{
		Title:   "RELATED BEADS",
		Width:   width,
		Height:  height,
		Focused: h.focused == historyFocusMiddle,
	})
}
