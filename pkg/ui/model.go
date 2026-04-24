package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/agents"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/bql"
	"github.com/seanmartinsmith/beadstui/pkg/cass"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/instance"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
	"github.com/seanmartinsmith/beadstui/pkg/updater"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// DoltServerStopper is implemented by doltctl.ServerState. Keeps ui decoupled
// from the doltctl package. Only the StopIfOwned method is called at shutdown.
type DoltServerStopper interface {
	StopIfOwned() (stopped bool, err error)
}

// View width thresholds for adaptive layout
const (
	SplitViewThreshold     = 100
	WideViewThreshold      = 140
	UltraWideViewThreshold = 180
)

// focus represents which UI element has keyboard focus
type focus int

const (
	focusList focus = iota
	focusDetail
	focusBoard
	focusGraph
	focusTree // Hierarchical tree view (bv-gllx)
	focusLabelDashboard
	focusInsights
	focusActionable
	focusRecipePicker
	focusRepoPicker
	focusHelp
	focusQuitConfirm
	focusTimeTravelInput
	focusHistory
	focusAttention
	focusLabelPicker
	focusSprint      // Sprint dashboard view (bv-161)
	focusAgentPrompt // AGENTS.md integration prompt (bv-i8dk)
	focusFlowMatrix  // Cross-label flow matrix view
	focusTutorial    // Interactive tutorial (bv-8y31)
	focusCassModal   // Cass session preview modal (bv-5bqh)
	focusUpdateModal // Self-update modal (bv-182)
	focusBQLQuery    // BQL composable search modal
)

// ViewMode represents which primary view is active. Only one view mode
// can be active at a time. Layout concerns (isSplitView, showDetails)
// are orthogonal and tracked separately.
type ViewMode int

const (
	ViewList           ViewMode = iota // Default list view
	ViewBoard                          // Kanban board
	ViewGraph                          // Dependency graph
	ViewTree                           // Hierarchical tree view
	ViewActionable                     // Actionable/execution plan
	ViewHistory                        // Git history correlation
	ViewSprint                         // Sprint dashboard
	ViewInsights                       // Insights panel
	ViewFlowMatrix                     // Cross-label flow matrix
	ViewLabelDashboard                 // Label health dashboard
	ViewAttention                      // Attention scores view
)

// ModalType identifies which modal overlay (if any) is currently active.
// Only one modal can be active at a time - opening one closes any previous one.
type ModalType int

const (
	ModalNone               ModalType = iota // No modal active
	ModalHelp                                // Help overlay
	ModalQuitConfirm                         // Quit confirmation dialog
	ModalRecipePicker                        // Recipe/filter picker
	ModalBQLQuery                            // BQL query input
	ModalLabelPicker                         // Label filter picker
	ModalRepoPicker                          // Repository/project picker
	ModalTimeTravelInput                     // Time-travel date input prompt
	ModalAgentPrompt                         // AGENTS.md integration prompt
	ModalTutorial                            // Interactive tutorial
	ModalCassSession                         // Cass session preview
	ModalUpdate                              // Self-update dialog
	ModalAlerts                              // Alerts panel
	ModalLabelHealthDetail                   // Label health detail drill-down
	ModalLabelDrilldown                      // Label issue drill-down
	ModalLabelGraphAnalysis                  // Label graph analysis
)

// ModalTab identifies which tab the shared alerts/notifications modal is
// currently showing (bt-46p6.10).
type ModalTab int

const (
	TabAlerts ModalTab = iota
	TabNotifications
)

// modalActive returns true when any modal overlay is open.
func (m Model) modalActive() bool { return m.activeModal != ModalNone }

// openModal sets the active modal, closing any previously open modal.
func (m *Model) openModal(t ModalType) { m.activeModal = t }

// closeModal dismisses the currently active modal.
func (m *Model) closeModal() { m.activeModal = ModalNone }

// SortMode represents the current list sorting mode (bv-3ita)
type SortMode int

const (
	SortDefault     SortMode = iota // Priority asc, then created desc (original default)
	SortUpdated                     // By last update, newest first
	SortCreatedDesc                 // By creation date, newest first
	SortCreatedAsc                  // By creation date, oldest first
	SortPriority                    // By priority only (ascending)
	SortProgress                    // By status lifecycle: in_progress first, closed/tombstone last (bt-lm2h)
	numSortModes                    // Keep this last - used for cycling
)

// String returns a human-readable label for the sort mode
func (s SortMode) String() string {
	switch s {
	case SortUpdated:
		return "Updated"
	case SortCreatedDesc:
		return "Created ↓"
	case SortCreatedAsc:
		return "Created ↑"
	case SortPriority:
		return "Priority"
	case SortProgress:
		return "Progress"
	default:
		return "Default"
	}
}

// LabelGraphAnalysisResult holds label-specific graph analysis results (bv-109)
type LabelGraphAnalysisResult struct {
	Label        string
	Subgraph     analysis.LabelSubgraph
	PageRank     analysis.LabelPageRankResult
	CriticalPath analysis.LabelCriticalPathResult
}

// UpdateMsg is sent when a new version is available
type UpdateMsg struct {
	TagName string
	URL     string
}

// Phase2ReadyMsg is sent when async graph analysis Phase 2 completes
type Phase2ReadyMsg struct {
	Stats    *analysis.GraphStats // The stats that completed, to detect stale messages
	Insights analysis.Insights    // Precomputed insights for Phase 2 metrics
}

// WaitForPhase2Cmd returns a command that waits for Phase 2 and sends Phase2ReadyMsg
func WaitForPhase2Cmd(stats *analysis.GraphStats) tea.Cmd {
	return func() tea.Msg {
		if stats == nil {
			return Phase2ReadyMsg{}
		}
		stats.WaitForPhase2()
		ins := stats.GenerateInsights(stats.NodeCount)
		return Phase2ReadyMsg{Stats: stats, Insights: ins}
	}
}

// FileChangedMsg is sent when the beads file changes on disk
type FileChangedMsg struct{}

// DataSourceReloadMsg is sent when a non-file datasource (e.g. Dolt) finishes reloading.
type DataSourceReloadMsg struct {
	Issues []model.Issue
	Err    error
}

// semanticDebounceTickMsg is sent after debounce delay to trigger semantic computation
type semanticDebounceTickMsg struct{}

// statusClearMsg is sent after a delay to auto-clear transient status messages.
type statusClearMsg struct{ seq uint64 }

// statusTickMsg is a recurring tick that forces auto-dismiss of idle status
// messages even when the app is otherwise idle (bt-m9te, bt-y0k7).
type statusTickMsg struct{}

// workerPollTickMsg drives a small background-mode status refresh (spinner + freshness) (bv-9nfy).
type workerPollTickMsg struct{}

var workerSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	freshnessErrorRetries = 3
)

func freshnessWarnThreshold() time.Duration {
	return envDurationSeconds("BT_FRESHNESS_WARN_S", 30*time.Second)
}

func freshnessStaleThreshold() time.Duration {
	return envDurationSeconds("BT_FRESHNESS_STALE_S", 2*time.Minute)
}

func workerPollTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return workerPollTickMsg{}
	})
}

// WatchFileCmd returns a command that waits for file changes and sends FileChangedMsg
func WatchFileCmd(w *watcher.Watcher) tea.Cmd {
	return func() tea.Msg {
		<-w.Changed()
		return FileChangedMsg{}
	}
}

// StartBackgroundWorkerCmd starts the background worker and triggers an initial refresh.
func StartBackgroundWorkerCmd(w *BackgroundWorker) tea.Cmd {
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		if err := w.Start(); err != nil {
			return SnapshotErrorMsg{Err: fmt.Errorf("starting background worker: %w", err), Recoverable: false}
		}
		w.TriggerRefresh()
		return nil
	}
}

// WaitForBackgroundWorkerMsgCmd waits for the next BackgroundWorker message.
func WaitForBackgroundWorkerMsgCmd(w *BackgroundWorker) tea.Cmd {
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		select {
		case msg := <-w.Messages():
			return msg
		case <-w.Done():
			return nil
		}
	}
}

// CheckUpdateCmd returns a command that checks for updates
func CheckUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		tag, url, err := updater.CheckForUpdates()
		if err == nil && tag != "" {
			return UpdateMsg{TagName: tag, URL: url}
		}
		return nil
	}
}

// HistoryLoadedMsg is sent when background history loading completes
type HistoryLoadedMsg struct {
	Report *correlation.HistoryReport
	Error  error
}

// AgentFileCheckMsg is sent after checking for AGENTS.md integration (bv-i8dk)
type AgentFileCheckMsg struct {
	ShouldPrompt bool
	FilePath     string
	FileType     string
}

// CheckAgentFileCmd returns a command that checks if we should prompt for AGENTS.md
func CheckAgentFileCmd(workDir string) tea.Cmd {
	return func() tea.Msg {
		if workDir == "" {
			return AgentFileCheckMsg{ShouldPrompt: false}
		}

		// Check if we should prompt based on preferences
		if !agents.ShouldPromptForAgentFile(workDir) {
			return AgentFileCheckMsg{ShouldPrompt: false}
		}

		// Detect agent file
		detection := agents.DetectAgentFile(workDir)

		// Only prompt if file exists but doesn't have our blurb
		if detection.Found() && detection.NeedsBlurb() {
			return AgentFileCheckMsg{
				ShouldPrompt: true,
				FilePath:     detection.FilePath,
				FileType:     detection.FileType,
			}
		}

		return AgentFileCheckMsg{ShouldPrompt: false}
	}
}

// LoadHistoryCmd returns a command that loads history data in the background
func LoadHistoryCmd(issues []model.Issue, beadsPath string) tea.Cmd {
	return func() tea.Msg {
		var repoPath string
		var err error

		if beadsPath != "" {
			// If beadsPath is provided (single-repo mode), derive repo root from it.
			// Try to resolve absolute path first.
			if absPath, e := filepath.Abs(beadsPath); e == nil {
				dir := filepath.Dir(absPath)
				// Standard layout: <repo_root>/.beads/<file.jsonl>
				if filepath.Base(dir) == ".beads" {
					repoPath = filepath.Dir(dir)
				} else {
					// Legacy/Flat layout: <repo_root>/<file.jsonl>
					repoPath = dir
				}
			}
		}

		// Fallback to CWD if beadsPath is empty (workspace mode) or Abs failed
		if repoPath == "" {
			repoPath, err = os.Getwd()
			if err != nil {
				return HistoryLoadedMsg{Error: err}
			}
		}

		// Convert model.Issue to correlation.BeadInfo
		beads := make([]correlation.BeadInfo, len(issues))
		for i, issue := range issues {
			beads[i] = correlation.BeadInfo{
				ID:     issue.ID,
				Title:  issue.Title,
				Status: string(issue.Status),
			}
		}

		correlator := correlation.NewCorrelator(repoPath, beadsPath)
		opts := correlation.CorrelatorOptions{
			Limit: 500, // Reasonable limit for TUI performance
		}

		report, err := correlator.GenerateReport(beads, opts)
		return HistoryLoadedMsg{Report: report, Error: err}
	}
}

// DoltState holds Dolt connection lifecycle state. Embedded in Model
// so field access (m.doltConnected) stays unchanged.
type DoltState struct {
	lastDoltVerified time.Time         // Last successful Dolt poll (even if no data changed)
	doltConnected    bool              // True when Dolt poll loop is healthy
	doltServer       DoltServerStopper // Dolt server lifecycle handle (bt-07jp); nil if not managed
	doltShutdownMsg  string            // Message to print after TUI exits (bt-llek)
}

// WorkspaceState holds multi-project workspace state. Embedded in Model
// so field access (m.workspaceMode) stays unchanged.
type WorkspaceState struct {
	workspaceMode    bool            // True when viewing multiple repos
	availableRepos   []string        // List of repo prefixes available
	activeRepos      map[string]bool // Which repos are currently shown (nil = all)
	workspaceSummary string          // Summary text for footer (e.g., "3 projects")
	currentProjectDB string          // Auto-detected project DB name for W toggle (empty = no home project)
}

// FilterState holds filter, sort, search, recipe, and BQL state.
// Pointer on Model to keep Model copies cheap.
type FilterState struct {
	currentFilter string
	labelFilter   string   // independent label filter, composes with currentFilter (e.g. "area:tui" or "area:tui,area:cli")
	sortMode      SortMode // bv-3ita: current sort mode
	activeRecipe  *recipe.Recipe
	recipeLoader  *recipe.Loader
	bqlEngine     *bql.MemoryExecutor
	activeBQLExpr *bql.Query // Parsed BQL expression (nil = no BQL filter active)
}

// AnalysisCache holds derived data computed from graph analysis. Not filter state -
// this is cached analysis output that gets recomputed when issues change.
// Pointer on Model to keep Model copies cheap.
type AnalysisCache struct {
	countOpen         int
	countReady        int
	countBlocked      int
	countClosed       int
	triageScores      map[string]float64                          // issueID -> triage score
	triageReasons     map[string]analysis.TriageReasons           // issueID -> reasons
	unblocksMap       map[string][]string                         // issueID -> IDs that would be unblocked
	quickWinSet       map[string]bool                             // issueID -> true if quick win
	blockerSet        map[string]bool                             // issueID -> true if significant blocker
	priorityHints     map[string]*analysis.PriorityRecommendation // issueID -> recommendation
	showPriorityHints bool
}

// DataState holds the core issue data, analysis engine, and data loading infrastructure.
// Pointer on Model to keep Model copies cheap (largest sub-struct).
type DataState struct {
	issues              []model.Issue
	pooledIssues        []*model.Issue // Issue pool refs for sync reloads (return to pool on replace)
	issueMap            map[string]*model.Issue
	analyzer            *analysis.Analyzer
	analysis            *analysis.GraphStats
	beadsPath           string                 // Path to beads.jsonl for reloading
	dataSource          *datasource.DataSource // Selected data source for refresh routing
	watcher             *watcher.Watcher       // File watcher for live reload
	instanceLock        *instance.Lock         // Multi-instance coordination lock
	snapshot            *DataSnapshot
	snapshotInitPending bool              // true until first BackgroundWorker snapshot received
	backgroundWorker    *BackgroundWorker // manages async data loading (nil if background mode disabled)
	workerSpinnerIdx    int               // Spinner frame for background worker activity
	lastForceRefresh    time.Time
}

// Model is the main Bubble Tea model for the beads viewer
type Model struct {
	// Core data, analysis engine, and data loading infrastructure.
	data *DataState

	// UI Components
	list               list.Model
	viewport           viewport.Model
	renderer           *MarkdownRenderer
	board              BoardModel
	labelDashboard     LabelDashboardModel
	velocityComparison VelocityComparisonModel // bv-125
	shortcutsSidebar   ShortcutsSidebar        // bv-3qi5
	graphView          GraphModel
	tree               TreeModel // Hierarchical tree view (bv-gllx)
	insightsPanel      InsightsModel
	flowMatrix         FlowMatrixModel // Cross-label flow matrix
	theme              Theme

	// Update State
	updateAvailable bool
	updateTag       string
	updateURL       string

	// Modal state - only one modal can be active at a time
	activeModal ModalType

	// Focus and View State
	mode                     ViewMode // Active view mode (ViewList, ViewBoard, ViewGraph, etc.)
	focused                  focus
	focusBeforeHelp          focus // Stores focus before opening help overlay
	focusBeforeSearch        focus // Stores focus before / entered search from a non-list pane (bt-cd3x)
	isSplitView              bool
	splitPaneRatio           float64 // Ratio of list pane width (0.2-0.8), default 0.4
	showDetails              bool
	helpScroll               int // Scroll offset for help overlay
	ready                    bool
	width                    int
	height                   int
	labelHealthDetail        *analysis.LabelHealth
	labelHealthDetailFlow    labelFlowSummary
	labelDrilldownLabel      string
	labelDrilldownIssues     []model.Issue
	labelDrilldownCache      map[string][]model.Issue
	labelGraphAnalysisResult *LabelGraphAnalysisResult
	showShortcutsSidebar     bool // bv-3qi5 toggleable shortcuts sidebar
	showWisps                bool // bt-9kdo: toggle wisp visibility (default: hide)
	labelHealthCached        bool
	labelHealthCache         analysis.LabelAnalysisResult
	attentionCached          bool
	attentionCache           analysis.LabelAttentionResult

	// Actionable view
	actionableView ActionableModel

	// History view
	historyView       HistoryModel
	historyLoading    bool // True while history is being loaded in background
	historyLoadFailed bool // True if history loading failed

	// Filter, sort, search, recipe, BQL state
	filter *FilterState

	// Semantic search state (stays flat - tightly coupled to list component)
	semanticSearchEnabled  bool
	semanticIndexBuilding  bool
	semanticSearch         *SemanticSearch
	semanticHybridEnabled  bool
	semanticHybridPreset   search.PresetName
	semanticHybridBuilding bool
	semanticHybridReady    bool
	lastSearchTerm         string

	// Derived analysis data (cached, recomputed when issues change)
	ac *AnalysisCache

	// Recipe picker (modal UI stays on Model, modal visibility via activeModal)
	recipePicker RecipePickerModel

	// BQL query modal (modal UI stays on Model, modal visibility via activeModal)
	bqlQuery BQLQueryModal

	// Label picker (bv-126)
	labelPicker LabelPickerModel

	// Project picker (multi-project mode)
	repoPicker RepoPickerModel

	// Time-travel mode
	timeTravelMode   bool
	timeTravelDiff   *analysis.SnapshotDiff
	timeTravelSince  string
	newIssueIDs      map[string]bool // Issues in diff.NewIssues
	closedIssueIDs   map[string]bool // Issues in diff.ClosedIssues
	modifiedIssueIDs map[string]bool // Issues in diff.ModifiedIssues

	// Time-travel input prompt (modal visibility via activeModal)
	timeTravelInput textinput.Model

	// Status message (for temporary feedback)
	statusMsg      string
	statusIsError  bool
	statusIsInline bool      // true = render subtly in footer hint slot; false = full-width banner (bt-y0k7)
	statusSeq      uint64    // incremented on each status set; used for auto-clear
	statusSetAt    time.Time // when statusMsg was last set; used for auto-dismiss (bt-zdae)

	// Activity event ring buffer (bt-d5wr). Populated by handleSnapshotReady
	// via events.Diff; consumed by the footer ticker + count badge and the
	// notification center modal (both implemented in later beads). Session-
	// scoped; not persisted across bt restarts.
	events *events.RingBuffer

	// Dolt connection state (bt-3ynd). Embedded to keep m.doltConnected access pattern.
	DoltState

	// Workspace mode state. Embedded to keep m.workspaceMode access pattern.
	WorkspaceState

	// Alerts panel (bv-168, modal visibility via activeModal)
	alerts          []drift.Alert
	alertsCritical  int
	alertsWarning   int
	alertsInfo      int
	alertsCursor    int
	dismissedAlerts map[string]bool

	// Alert filters (bt-46p6.5): stackable severity/type/project/sort
	alertFilterSeverity string // "" = all, or "critical"/"warning"/"info"
	alertFilterType     string // "" = all, or an AlertType string
	alertFilterProject  string // "" = all, or a project prefix
	alertSortOrder      int    // 0=default, 1=oldest-first, 2=newest-first

	// Tab-scoped state for the shared alerts/notifications modal (bt-46p6.10).
	// activeTab is which tab is visible; alertsCursor applies to alerts tab,
	// notificationsCursor to notifications tab. Flipping tabs preserves both.
	activeTab           ModalTab
	notificationsCursor int

	// Double-click detection for modal mouse activation (bt-46p6.14).
	// Updated on every MouseClickMsg inside the alerts modal; a second click
	// at the same (X,Y) within modalDoubleClickWindow promotes to activate.
	lastModalClickAt time.Time
	lastModalClickX  int
	lastModalClickY  int

	// pendingCommentScroll is a one-shot signal from the notifications-tab
	// deep-link path (bt-46p6.16). When non-zero, the next call to
	// updateViewportContent locates the comment with this CreatedAt in the
	// rendered detail view and aligns the viewport to it. Cleared as soon as
	// the scroll is applied (or when no matching comment is found).
	pendingCommentScroll time.Time

	// Sprint view (bv-161)
	sprints        []model.Sprint
	selectedSprint *model.Sprint
	sprintViewText string

	// Project identity
	projectName string // Display name for the current project (directory basename)

	// AGENTS.md integration (bv-i8dk, modal visibility via activeModal)
	agentPromptModal AgentPromptModal
	workDir          string // Working directory for agent file detection

	// Tutorial integration (bv-8y31, modal visibility via activeModal)
	tutorialModel TutorialModel

	// Cass session preview modal (bv-5bqh, modal visibility via activeModal)
	cassModal      CassSessionModal
	cassCorrelator *cass.Correlator

	// Self-update modal (bv-182, modal visibility via activeModal)
	updateModal UpdateModal
}

// labelCount is a simple label->count pair for display
type labelCount struct {
	Label string
	Count int
}

type labelFlowSummary struct {
	Incoming []labelCount
	Outgoing []labelCount
}

// getCrossFlowsForLabel returns outgoing cross-label dependency counts for a label
func (m Model) getCrossFlowsForLabel(label string) labelFlowSummary {
	cfg := analysis.DefaultLabelHealthConfig()
	flow := analysis.ComputeCrossLabelFlow(m.data.issues, cfg)
	out := labelFlowSummary{}
	inCounts := make(map[string]int)
	outCounts := make(map[string]int)

	for _, dep := range flow.Dependencies {
		if dep.ToLabel == label {
			inCounts[dep.FromLabel] += dep.IssueCount
		}
		if dep.FromLabel == label {
			outCounts[dep.ToLabel] += dep.IssueCount
		}
	}

	for lbl, c := range inCounts {
		out.Incoming = append(out.Incoming, labelCount{Label: lbl, Count: c})
	}
	for lbl, c := range outCounts {
		out.Outgoing = append(out.Outgoing, labelCount{Label: lbl, Count: c})
	}

	sort.Slice(out.Incoming, func(i, j int) bool {
		if out.Incoming[i].Count == out.Incoming[j].Count {
			return out.Incoming[i].Label < out.Incoming[j].Label
		}
		return out.Incoming[i].Count > out.Incoming[j].Count
	})
	sort.Slice(out.Outgoing, func(i, j int) bool {
		if out.Outgoing[i].Count == out.Outgoing[j].Count {
			return out.Outgoing[i].Label < out.Outgoing[j].Label
		}
		return out.Outgoing[i].Count > out.Outgoing[j].Count
	})

	return out
}

// filterIssuesByLabel returns issues that contain the given label (case-sensitive match)
func (m Model) filterIssuesByLabel(label string) []model.Issue {
	if m.labelDrilldownCache != nil {
		if cached, ok := m.labelDrilldownCache[label]; ok {
			return cached
		}
	}

	var out []model.Issue
	for _, iss := range m.data.issues {
		for _, l := range iss.Labels {
			if l == label {
				out = append(out, iss)
				break
			}
		}
	}

	if m.labelDrilldownCache != nil {
		m.labelDrilldownCache[label] = out
	}
	return out
}

// extractLabelCounts converts LabelStats map to a simple count map for the label picker
func extractLabelCounts(stats map[string]*analysis.LabelStats) map[string]int {
	counts := make(map[string]int)
	for label, stat := range stats {
		if stat != nil {
			counts[label] = stat.TotalCount
		}
	}
	return counts
}

// WorkspaceInfo contains workspace loading metadata for TUI display
type WorkspaceInfo struct {
	Enabled      bool
	RepoCount    int
	FailedCount  int
	TotalIssues  int
	RepoPrefixes []string
}

// NewModel creates a new Model from the given issues.
// beadsPath is the path to the beads.jsonl file for live reload support.
// ds is the selected DataSource for routing refresh through the correct backend (nil for historical/test).
func NewModel(issues []model.Issue, activeRecipe *recipe.Recipe, beadsPath string, ds *datasource.DataSource) Model {
	// Graph Analysis - Phase 1 is instant, Phase 2 runs in background
	analyzer := analysis.NewAnalyzer(issues)
	graphStats := analyzer.AnalyzeAsync(context.Background())

	// Sort issues
	if activeRecipe != nil && activeRecipe.Sort.Field != "" {
		r := activeRecipe
		descending := r.Sort.Direction == "desc"

		sort.Slice(issues, func(i, j int) bool {
			less := false
			switch r.Sort.Field {
			case "priority":
				less = issues[i].Priority < issues[j].Priority
			case "created", "created_at":
				less = issues[i].CreatedAt.Before(issues[j].CreatedAt)
			case "updated", "updated_at":
				less = issues[i].UpdatedAt.Before(issues[j].UpdatedAt)
			case "impact":
				less = graphStats.GetCriticalPathScore(issues[i].ID) < graphStats.GetCriticalPathScore(issues[j].ID)
			case "pagerank":
				less = graphStats.GetPageRankScore(issues[i].ID) < graphStats.GetPageRankScore(issues[j].ID)
			default:
				less = issues[i].Priority < issues[j].Priority
			}
			if descending {
				return !less
			}
			return less
		})
	} else {
		// Default Sort: Open first, then by Priority (ascending), then by date (newest first)
		sort.Slice(issues, func(i, j int) bool {
			iClosed := isClosedLikeStatus(issues[i].Status)
			jClosed := isClosedLikeStatus(issues[j].Status)
			if iClosed != jClosed {
				return !iClosed // Open issues first
			}
			if issues[i].Priority != issues[j].Priority {
				return issues[i].Priority < issues[j].Priority // Lower priority number = higher priority
			}
			return issues[i].CreatedAt.After(issues[j].CreatedAt) // Newer first
		})
	}

	// Build lookup map
	issueMap := make(map[string]*model.Issue, len(issues))

	// Build list items - scores may be 0 until Phase 2 completes
	items := make([]list.Item, len(issues))
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]

		items[i] = IssueItem{
			Issue:      issues[i],
			GraphScore: graphStats.GetPageRankScore(issues[i].ID),
			Impact:     graphStats.GetCriticalPathScore(issues[i].ID),
			RepoPrefix: ExtractRepoPrefix(issues[i].ID),
		}
	}

	// Compute stats
	cOpen, cReady, cBlocked, cClosed := 0, 0, 0, 0
	for i := range issues {
		issue := &issues[i]
		if isClosedLikeStatus(issue.Status) {
			cClosed++
			continue
		}

		cOpen++
		if issue.Status == model.StatusBlocked {
			cBlocked++
			continue
		}

		// Check if blocked by open dependencies
		isBlocked := false
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				isBlocked = true
				break
			}
		}
		if !isBlocked {
			cReady++
		}
	}

	// Theme: load YAML overrides, apply to globals and theme struct
	themeConfig := LoadTheme()
	ApplyThemeToGlobals(themeConfig)
	theme := DefaultTheme()
	ApplyThemeToThemeStruct(&theme, themeConfig)

	// Default dimensions for immediate ready state (updated when WindowSizeMsg arrives)
	// This eliminates the "Initializing..." phase entirely, fixing slow startup issues
	// in tmux, SSH, and slow terminal emulators where the terminal may delay sending size.
	const defaultWidth = 120
	const defaultHeight = 40

	// List setup - initialize with default dimensions so UI is immediately usable
	delegate := IssueDelegate{Theme: theme, WorkspaceMode: false}
	l := list.New(items, delegate, defaultWidth, defaultHeight-3)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	// The Bubbles list ships a "Filter: " prompt; bt's affordance is a search
	// bar (/), and the footer shows fuzzy/semantic/hybrid search modes — so the
	// prompt text matches the user's mental model (bt-imcn).
	l.FilterInput.Prompt = "Search: "
	// Pre-empt the ranker with exact-ID matches across all search modes (bt-i4yn).
	l.Filter = idPriorityFilter(list.DefaultFilter)
	// Clear all default styles that might add extra lines
	l.Styles.Title = lipgloss.NewStyle()
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.Styles.Filter.Focused.Prompt = lipgloss.NewStyle().Foreground(theme.Primary)
	l.Styles.Filter.Focused.Text = lipgloss.NewStyle().Foreground(theme.Primary)
	l.Styles.StatusBar = lipgloss.NewStyle()
	l.Styles.StatusEmpty = lipgloss.NewStyle()
	l.Styles.StatusBarActiveFilter = lipgloss.NewStyle()
	l.Styles.StatusBarFilterCount = lipgloss.NewStyle()
	l.Styles.NoItems = lipgloss.NewStyle()
	l.Styles.PaginationStyle = lipgloss.NewStyle()
	l.Styles.HelpStyle = lipgloss.NewStyle()

	// Theme-aware markdown renderer
	renderer := NewMarkdownRendererWithTheme(80, theme)

	// Initialize viewport with default dimensions
	vp := viewport.New(viewport.WithWidth(defaultWidth), viewport.WithHeight(defaultHeight-2))

	// Initialize sub-components
	board := NewBoardModel(issues, theme)
	labelDashboard := NewLabelDashboardModel(theme)
	labelDashboard.SetSize(defaultWidth, defaultHeight-1)
	velocityComparison := NewVelocityComparisonModel(theme) // bv-125
	shortcutsSidebar := NewShortcutsSidebar(theme)          // bv-3qi5
	ins := graphStats.GenerateInsights(len(issues))         // allow UI to show as many as fit
	insightsPanel := NewInsightsModel(ins, issueMap, theme)
	insightsPanel.SetSize(defaultWidth, defaultHeight-1)
	graphView := NewGraphModel(issues, &ins, theme)

	// Priority hints are generated asynchronously when Phase 2 completes
	// This avoids blocking startup on expensive graph analysis
	priorityHints := make(map[string]*analysis.PriorityRecommendation)

	// Compute triage insights (bv-151) - reuse existing analyzer/stats (bv-runn.12)
	triageResult := analysis.ComputeTriageFromAnalyzer(analyzer, graphStats, issues, analysis.TriageOptions{}, time.Now())
	triageScores := make(map[string]float64, len(triageResult.Recommendations))
	triageReasons := make(map[string]analysis.TriageReasons, len(triageResult.Recommendations))
	quickWinSet := make(map[string]bool, len(triageResult.QuickWins))
	blockerSet := make(map[string]bool, len(triageResult.BlockersToClear))
	unblocksMap := make(map[string][]string, len(triageResult.Recommendations))

	for _, rec := range triageResult.Recommendations {
		triageScores[rec.ID] = rec.Score
		if len(rec.Reasons) > 0 {
			triageReasons[rec.ID] = analysis.TriageReasons{
				Primary:    rec.Reasons[0],
				All:        rec.Reasons,
				ActionHint: rec.Action,
			}
		}
		unblocksMap[rec.ID] = rec.UnblocksIDs
	}
	for _, qw := range triageResult.QuickWins {
		quickWinSet[qw.ID] = true
	}
	for _, bl := range triageResult.BlockersToClear {
		blockerSet[bl.ID] = true
	}

	// Update items with triage data
	for i := range items {
		if issueItem, ok := items[i].(IssueItem); ok {
			issueItem.TriageScore = triageScores[issueItem.Issue.ID]
			if reasons, exists := triageReasons[issueItem.Issue.ID]; exists {
				issueItem.TriageReason = reasons.Primary
				issueItem.TriageReasons = reasons.All
			}
			issueItem.IsQuickWin = quickWinSet[issueItem.Issue.ID]
			issueItem.IsBlocker = blockerSet[issueItem.Issue.ID]
			issueItem.UnblocksCount = len(unblocksMap[issueItem.Issue.ID])
			items[i] = issueItem
		}
	}

	// Initialize recipe loader
	recipeLoader := recipe.NewLoader()
	_ = recipeLoader.Load() // Load recipes (errors are non-fatal, will just show empty)
	recipePicker := NewRecipePickerModel(recipeLoader.List(), theme)

	// Initialize BQL query modal
	bqlQueryModal := NewBQLQueryModal(theme)
	bqlEngine := bql.NewMemoryExecutor()

	// Initialize label picker (bv-126)
	labelExtraction := analysis.ExtractLabels(issues)
	labelCounts := extractLabelCounts(labelExtraction.Stats)
	labelPicker := NewLabelPickerModel(labelExtraction.Labels, labelCounts, theme)

	// Initialize time-travel input
	ti := textinput.New()
	ti.Placeholder = "HEAD~5, main, v1.0.0, 2024-01-01..."
	ti.CharLimit = 100
	ti.SetWidth(40)
	ti.Prompt = "⏱️  Revision: "
	tiStyles := ti.Styles()
	tiStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	tiStyles.Focused.Text = lipgloss.NewStyle().Foreground(theme.Base.GetForeground())
	ti.SetStyles(tiStyles)

	// Initialize file watcher for live reload
	var fileWatcher *watcher.Watcher
	var watcherErr error
	var backgroundWorker *BackgroundWorker
	var backgroundModeErr error
	backgroundModeRequested := false
	if v := strings.TrimSpace(os.Getenv("BT_BACKGROUND_MODE")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			backgroundModeRequested = true
		case "0", "false", "no", "off":
			backgroundModeRequested = false
		}
	}

	isDolt := ds != nil && (ds.Type == datasource.SourceTypeDolt || ds.Type == datasource.SourceTypeDoltGlobal)

	// Compute beadsDir for reconnect and port resolution
	workerBeadsDir, _ := loader.GetBeadsDir("")

	// Dolt sources always use the background worker for polling since there are
	// no files to watch. JSONL sources require explicit opt-in via BT_BACKGROUND_MODE.
	bgEnabled := (beadsPath != "" || isDolt) && (backgroundModeRequested || isDolt)
	if bgEnabled {
		bw, err := NewBackgroundWorker(WorkerConfig{
			BeadsPath:     beadsPath,
			BeadsDir:      workerBeadsDir,
			DataSource:    ds,
			DebounceDelay: 200 * time.Millisecond,
		})
		if err != nil {
			backgroundModeErr = err
		} else {
			backgroundWorker = bw
		}
	}

	if beadsPath != "" && backgroundWorker == nil {
		w, err := watcher.NewWatcher(beadsPath,
			watcher.WithDebounceDuration(200*time.Millisecond),
		)
		if err != nil {
			watcherErr = err
		} else if err := w.Start(); err != nil {
			watcherErr = err
		} else {
			fileWatcher = w
		}
	}

	// Initialize instance lock for multi-instance coordination (bv-vrvn)
	var instLock *instance.Lock
	if beadsPath != "" {
		beadsDir := filepath.Dir(beadsPath)
		lock, err := instance.NewLock(beadsDir)
		if err == nil {
			instLock = lock
		}
		// Lock creation failure is non-fatal - we just won't have coordination
	}

	// Semantic search (bv-9gf.3): initialized lazily on first toggle.
	semanticSearch := NewSemanticSearch()
	semanticIDs := make([]string, 0, len(items))
	for _, it := range items {
		if issueItem, ok := it.(IssueItem); ok {
			semanticIDs = append(semanticIDs, issueItem.Issue.ID)
		}
	}
	semanticSearch.SetIDs(semanticIDs)

	// Build initial status message if watcher failed
	var initialStatus string
	var initialStatusErr bool
	if backgroundWorker != nil {
		initialStatus = "Background mode enabled"
		initialStatusErr = false
	} else if backgroundModeRequested && backgroundModeErr != nil {
		initialStatus = fmt.Sprintf("Background mode unavailable: %v (using sync reload)", backgroundModeErr)
		initialStatusErr = true
	} else if watcherErr != nil {
		initialStatus = fmt.Sprintf("Live reload unavailable: %v", watcherErr)
		initialStatusErr = true
	}

	// Precompute drift/health alerts (bv-168). At init, workspace mode has
	// not yet been decided — EnableWorkspaceMode runs after NewModel returns.
	// Pass global=false here; the next data refresh triggered by
	// EnableWorkspaceMode recomputes with the correct scope.
	alerts, alertsCritical, alertsWarning, alertsInfo := computeAlerts(issues, false)

	// Load sprints from the same directory as beadsPath (bv-161)
	var sprints []model.Sprint
	if beadsPath != "" {
		beadsDir := filepath.Dir(beadsPath)
		if loaded, err := loader.LoadSprintsFromFile(filepath.Join(beadsDir, loader.SprintsFileName)); err == nil {
			sprints = loaded
		}
	}

	// Tree view state should persist alongside the beads directory (e.g. BEADS_DIR overrides).
	treeModel := NewTreeModel(theme)
	if beadsPath != "" {
		treeModel.SetBeadsDir(filepath.Dir(beadsPath))
	}

	// Notification ring buffer with optional cross-restart persistence
	// (bt-6ool Part A). Hydrate before the model is wired so the live
	// pipeline starts on top of a buffer pre-populated with the last
	// session's events. Disabled by BT_NO_EVENT_PERSIST=1, by
	// BT_TEST_MODE=1 (so tests don't bleed in real ~/.bt/events.jsonl
	// state from the dev machine), or when the home directory cannot
	// be resolved.
	eventsBuf := events.NewRingBuffer(events.DefaultCapacity)
	if os.Getenv("BT_NO_EVENT_PERSIST") != "1" && os.Getenv("BT_TEST_MODE") == "" {
		if path, err := events.DefaultPersistPath(); err == nil {
			if loaded, lerr := events.LoadPersisted(path, events.DefaultMaxPersistAge); lerr == nil {
				eventsBuf.Hydrate(loaded)
			}
			eventsBuf.SetPersistPath(path)
		}
	}

	return Model{
		data: &DataState{
			issues:              issues,
			issueMap:            issueMap,
			analyzer:            analyzer,
			analysis:            graphStats,
			beadsPath:           beadsPath,
			dataSource:          ds,
			watcher:             fileWatcher,
			snapshotInitPending: backgroundWorker != nil,
			backgroundWorker:    backgroundWorker,
			instanceLock:        instLock,
		},
		filter: &FilterState{
			currentFilter: "all",
			recipeLoader:  recipeLoader,
			activeRecipe:  activeRecipe,
			bqlEngine:     bqlEngine,
		},
		ac: &AnalysisCache{
			countOpen:         cOpen,
			countReady:        cReady,
			countBlocked:      cBlocked,
			countClosed:       cClosed,
			priorityHints:     priorityHints,
			showPriorityHints: false, // Off by default, toggle with 'p'
			triageScores:      triageScores,
			triageReasons:     triageReasons,
			unblocksMap:       unblocksMap,
			quickWinSet:       quickWinSet,
			blockerSet:        blockerSet,
		},
		list:                   l,
		viewport:               vp,
		renderer:               renderer,
		board:                  board,
		labelDashboard:         labelDashboard,
		velocityComparison:     velocityComparison,
		shortcutsSidebar:       shortcutsSidebar,
		graphView:              graphView,
		tree:                   treeModel,
		insightsPanel:          insightsPanel,
		theme:                  theme,
		semanticSearch:         semanticSearch,
		semanticHybridEnabled:  false,
		semanticHybridPreset:   search.PresetDefault,
		semanticHybridBuilding: false,
		semanticHybridReady:    false,
		lastSearchTerm:         "",
		focused:                focusList,
		events:                 eventsBuf,
		splitPaneRatio:         0.4, // Default: list pane gets 40% of width
		// Initialize as ready with default dimensions to eliminate "Initializing..." phase
		ready:               true,
		width:               defaultWidth,
		height:              defaultHeight,
		recipePicker:        recipePicker,
		bqlQuery:            bqlQueryModal,
		labelPicker:         labelPicker,
		labelDrilldownCache: make(map[string][]model.Issue),
		timeTravelInput:     ti,
		statusMsg:           initialStatus,
		statusIsError:       initialStatusErr,
		historyLoading:      len(issues) > 0, // Will be loaded in Init()
		// Alerts panel (bv-168)
		alerts:          alerts,
		alertsCritical:  alertsCritical,
		alertsWarning:   alertsWarning,
		alertsInfo:      alertsInfo,
		dismissedAlerts: make(map[string]bool),
		// Sprint view (bv-161)
		sprints: sprints,
		// AGENTS.md integration (bv-i8dk) - workDir derived from beadsPath
		workDir: func() string {
			if beadsPath != "" {
				// beadsPath is like /path/to/project/.beads/beads.jsonl
				// workDir is /path/to/project
				return filepath.Dir(filepath.Dir(beadsPath))
			}
			return ""
		}(),
		// Tutorial integration (bv-8y31)
		tutorialModel: NewTutorialModel(theme),
	}
}

// replaceIssues swaps the model's issue set, recomputing analysis, maps, counts,
// list items, and sub-views. Used by DataSourceReloadMsg and other reload paths.
func (m *Model) replaceIssues(newIssues []model.Issue) {
	// Sort: open first, priority ascending, newest first
	sort.Slice(newIssues, func(i, j int) bool {
		iClosed := isClosedLikeStatus(newIssues[i].Status)
		jClosed := isClosedLikeStatus(newIssues[j].Status)
		if iClosed != jClosed {
			return !iClosed
		}
		if newIssues[i].Priority != newIssues[j].Priority {
			return newIssues[i].Priority < newIssues[j].Priority
		}
		return newIssues[i].CreatedAt.After(newIssues[j].CreatedAt)
	})

	// Recompute analysis
	m.data.issues = newIssues
	cachedAnalyzer := analysis.NewCachedAnalyzer(newIssues, nil)
	m.data.analyzer = cachedAnalyzer.Analyzer
	m.data.analysis = cachedAnalyzer.AnalyzeAsync(context.Background())
	m.labelHealthCached = false
	m.attentionCached = false

	// Rebuild lookup map
	m.data.issueMap = make(map[string]*model.Issue, len(newIssues))
	for i := range m.data.issues {
		m.data.issueMap[m.data.issues[i].ID] = &m.data.issues[i]
	}

	// Clear stale priority hints
	m.ac.priorityHints = make(map[string]*analysis.PriorityRecommendation)

	// Recompute counts
	m.ac.countOpen, m.ac.countReady, m.ac.countBlocked, m.ac.countClosed = 0, 0, 0, 0
	for i := range m.data.issues {
		issue := &m.data.issues[i]
		if isClosedLikeStatus(issue.Status) {
			m.ac.countClosed++
			continue
		}
		m.ac.countOpen++
		if issue.Status == model.StatusBlocked {
			m.ac.countBlocked++
			continue
		}
		isBlocked := false
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := m.data.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				isBlocked = true
				break
			}
		}
		if !isBlocked {
			m.ac.countReady++
		}
	}

	// Recompute alerts
	m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.data.issues, m.workspaceMode)
	m.dismissedAlerts = make(map[string]bool)
	if m.activeModal == ModalAlerts {
		m.closeModal()
	}

	// Rebuild list items
	items := make([]list.Item, len(m.data.issues))
	for i := range m.data.issues {
		item := IssueItem{
			Issue:      m.data.issues[i],
			GraphScore: m.data.analysis.GetPageRankScore(m.data.issues[i].ID),
			Impact:     m.data.analysis.GetCriticalPathScore(m.data.issues[i].ID),
			RepoPrefix: ExtractRepoPrefix(m.data.issues[i].ID),
		}
		item.TriageScore = m.ac.triageScores[m.data.issues[i].ID]
		if reasons, exists := m.ac.triageReasons[m.data.issues[i].ID]; exists {
			item.TriageReason = reasons.Primary
			item.TriageReasons = reasons.All
		}
		item.IsQuickWin = m.ac.quickWinSet[m.data.issues[i].ID]
		item.IsBlocker = m.ac.blockerSet[m.data.issues[i].ID]
		item.UnblocksCount = len(m.ac.unblocksMap[m.data.issues[i].ID])
		items[i] = item
	}
	m.updateSemanticIDs(items)
	m.clearSemanticScores()
	if m.semanticSearch != nil {
		m.semanticSearch.ResetCache()
		m.semanticSearch.SetMetricsCache(nil)
	}
	m.semanticHybridReady = false
	m.semanticHybridBuilding = false
	m.setListItems(items)

	// Invalidate label-derived caches
	m.labelHealthCached = false
	m.labelDrilldownCache = make(map[string][]model.Issue)
	m.updateViewportContent()
}

// isDoltSource returns true if the model's datasource is a Dolt server
// (single-repo or global).
func (m *Model) isDoltSource() bool {
	return m.data.dataSource != nil && (m.data.dataSource.Type == datasource.SourceTypeDolt || m.data.dataSource.Type == datasource.SourceTypeDoltGlobal)
}

// reloadFromDataSource returns a Cmd that reloads issues from the stored DataSource.
func (m *Model) reloadFromDataSource() tea.Cmd {
	ds := m.data.dataSource
	if ds == nil {
		return nil
	}
	src := *ds
	return func() tea.Msg {
		issues, err := datasource.LoadFromSource(src)
		return DataSourceReloadMsg{Issues: issues, Err: err}
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		CheckUpdateCmd(),
		WaitForPhase2Cmd(m.data.analysis),
		tea.RequestBackgroundColor,
	}
	if m.data.backgroundWorker != nil {
		cmds = append(cmds, StartBackgroundWorkerCmd(m.data.backgroundWorker))
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.data.backgroundWorker))
		cmds = append(cmds, workerPollTickCmd())
	} else if m.data.watcher != nil {
		cmds = append(cmds, WatchFileCmd(m.data.watcher))
	}
	cmds = append(cmds, statusTickCmd())
	// Start loading history in background
	if len(m.data.issues) > 0 {
		cmds = append(cmds, LoadHistoryCmd(m.issuesForAsync(), m.data.beadsPath))
	}
	// Check for AGENTS.md integration prompt (bv-i8dk)
	if m.workDir != "" && !m.workspaceMode {
		cmds = append(cmds, CheckAgentFileCmd(m.workDir))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.data.backgroundWorker != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			m.data.backgroundWorker.recordActivity()
		}
	}

	switch msg := msg.(type) {
	case UpdateMsg:
		m = m.handleUpdateMsg(msg)

	case UpdateCompleteMsg:
		m, cmd = m.handleUpdateCompleteMsg(msg)
		cmds = append(cmds, cmd)

	case UpdateProgressMsg:
		m, cmd = m.handleUpdateProgressMsg(msg)
		cmds = append(cmds, cmd)

	case statusClearMsg:
		m = m.handleStatusClear(msg)

	case statusTickMsg:
		m, cmd = m.handleStatusTick(msg)
		cmds = append(cmds, cmd)

	case SemanticIndexReadyMsg:
		var done bool
		m, cmd, done = m.handleSemanticIndexReady(msg)
		if done {
			return m, cmd
		}
		cmds = append(cmds, cmd)

	case HybridMetricsReadyMsg:
		m, cmd = m.handleHybridMetricsReady(msg)
		cmds = append(cmds, cmd)

	case SemanticFilterResultMsg:
		m = m.handleSemanticFilterResult(msg)

	case semanticDebounceTickMsg:
		var done bool
		m, cmd, done = m.handleSemanticDebounceTick()
		if done {
			return m, cmd
		}

	case workerPollTickMsg:
		m, cmd = m.handleWorkerPollTick()
		cmds = append(cmds, cmd)

	case Phase2ReadyMsg:
		m, cmd = m.handlePhase2Ready(msg)
		cmds = append(cmds, cmd)

	case Phase2UpdateMsg:
		return m.handlePhase2Update(msg)

	// -- Background worker channel messages (bt-6l2c) --
	// These message types arrive via w.send() -> WaitForBackgroundWorkerMsgCmd.
	// Only ONE subscriber is active at a time. Each handler MUST re-subscribe
	// via WaitForBackgroundWorkerMsgCmd or the subscription chain dies silently
	// and the poll loop, snapshots, and connection status all stop updating.

	case SnapshotReadyMsg:
		return m.handleSnapshotReady(msg)

	case SnapshotErrorMsg:
		return m.handleSnapshotError(msg)

	case DoltVerifiedMsg:
		return m.handleDoltVerified(msg)

	case DoltConnectionStatusMsg:
		return m.handleDoltConnectionStatus(msg)

	case TemporalCacheReadyMsg:
		return m.handleTemporalCacheReady(msg)

	// -- End background worker channel messages --

	case HistoryLoadedMsg:
		m = m.handleHistoryLoaded(msg)

	case AgentFileCheckMsg:
		m = m.handleAgentFileCheck(msg)

	case DataSourceReloadMsg:
		return m.handleDataSourceReload(msg)

	case FileChangedMsg:
		return m.handleFileChanged(msg)

	case tea.KeyPressMsg:
		m, cmd = m.handleKeyPress(msg)
		cmds = append(cmds, cmd)

	case tea.MouseWheelMsg:
		m, cmd = m.handleMouseWheel(msg)
		cmds = append(cmds, cmd)

	case tea.MouseClickMsg:
		m, cmd = m.handleMouseClick(msg)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m = m.handleWindowSize(msg)

	case tea.BackgroundColorMsg:
		isDark := msg.IsDark()
		isDarkBackground = isDark
		resolveColors()
		m.theme = DefaultTheme()
		tf := LoadTheme()
		ApplyThemeToGlobals(tf)
		ApplyThemeToThemeStruct(&m.theme, tf)
	}

	// Update list for navigation, but NOT for WindowSizeMsg
	// (we handle sizing ourselves to account for header/footer)
	// Only forward keyboard messages to list when list has focus (bv-hmkz fix)
	// This prevents j/k keys in detail view from changing list selection
	if m.focused == focusList && m.activeModal == ModalNone {
		if _, isWindowSize := msg.(tea.WindowSizeMsg); !isWindowSize {
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		}
		currentTerm := m.list.FilterInput.Value()
		if currentTerm != m.lastSearchTerm {
			m.lastSearchTerm = currentTerm
			if m.semanticSearchEnabled {
				m.clearSemanticScores()
			}
		}
		if m.semanticSearchEnabled && m.semanticHybridEnabled && m.list.FilterState() != list.Unfiltered {
			if strings.TrimSpace(currentTerm) != "" {
				m.applySemanticScores(currentTerm)
			}
		}
		m.updateListDelegate()

		// Restore prior focus after search cancel (bt-cd3x): if the user entered
		// search via / from another pane and then escaped out to Unfiltered,
		// bounce focus back to where they came from.
		if m.focusBeforeSearch != focusList && m.list.FilterState() == list.Unfiltered {
			m.focused = m.focusBeforeSearch
			m.focusBeforeSearch = focusList
			if m.isSplitView && m.focused == focusDetail {
				m.updateViewportContent()
			}
		}
	}

	// Update viewport if list selection changed in split view
	if m.isSplitView && m.focused == focusList {
		m.updateViewportContent()
	}

	// Trigger async semantic computation if needed (debounced)
	if m.semanticSearchEnabled && m.semanticSearch != nil && m.list.FilterState() != list.Unfiltered {
		pendingTerm := m.semanticSearch.GetPendingTerm()
		if pendingTerm != "" {
			// Debounce: only compute if 150ms since last query change
			if time.Since(m.semanticSearch.GetLastQueryTime()) >= 150*time.Millisecond {
				cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, pendingTerm))
			} else {
				// Schedule a tick to check again after debounce period
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
					return semanticDebounceTickMsg{}
				}))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// Stop cleans up resources (file watcher, instance lock, background worker, etc.)
// Should be called when the program exits
func (m *Model) Stop() {
	if m.data.backgroundWorker != nil {
		m.data.backgroundWorker.Stop()
	}
	if m.data.watcher != nil {
		m.data.watcher.Stop()
	}
	if m.data.instanceLock != nil {
		m.data.instanceLock.Release()
	}
	if len(m.data.pooledIssues) > 0 {
		loader.ReturnIssuePtrsToPool(m.data.pooledIssues)
		m.data.pooledIssues = nil
	}
	// Stop Dolt server if bt started it (bt-07jp)
	if m.doltServer != nil {
		if stopped, err := m.doltServer.StopIfOwned(); err != nil {
			log.Printf("WARN: failed to stop Dolt server: %v", err)
		} else if stopped {
			m.doltShutdownMsg = "Stopped Dolt server."
		}
	}
}

// SetDoltServer sets the Dolt server lifecycle handle for shutdown cleanup (bt-07jp).
// Also wires auto-reconnect into the background worker's poll loop if one exists.
func (m *Model) SetDoltServer(s DoltServerStopper, reconnectFn func(beadsDir string) error) {
	m.doltServer = s
	if m.data.backgroundWorker != nil && reconnectFn != nil {
		m.data.backgroundWorker.SetDoltReconnectFn(reconnectFn)
	}
}

// DoltShutdownMsg returns a message to print after the TUI exits,
// indicating whether bt stopped its Dolt server (bt-llek).
func (m *Model) DoltShutdownMsg() string {
	return m.doltShutdownMsg
}

// clearAttentionOverlay hides the attention overlay and clears its rendered text.
func (m *Model) clearAttentionOverlay() {
	if m.mode == ViewAttention {
		m.mode = ViewList
		m.insightsPanel.extraText = ""
	}
}

// RenderDebugView renders a specific view for debugging purposes.
// This is used by --debug-render to capture TUI output without running interactively.
func (m *Model) RenderDebugView(viewName string, width, height int) string {
	m.width = width
	m.height = height
	m.ready = true

	switch viewName {
	case "insights":
		m.insightsPanel.SetSize(width, height-1)
		return m.insightsPanel.View()
	case "board":
		return m.board.View(width, height-1)
	case "history":
		m.historyView.SetSize(width, height-1)
		return m.historyView.View()
	default:
		return "Unknown view: " + viewName
	}
}

func formatReloadDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
