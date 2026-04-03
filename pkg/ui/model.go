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
	"github.com/seanmartinsmith/beadstui/pkg/debug"
	"github.com/seanmartinsmith/beadstui/pkg/instance"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/updater"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// SortMode represents the current list sorting mode (bv-3ita)
type SortMode int

const (
	SortDefault     SortMode = iota // Priority asc, then created desc (original default)
	SortCreatedAsc                  // By creation date, oldest first
	SortCreatedDesc                 // By creation date, newest first
	SortPriority                    // By priority only (ascending)
	SortUpdated                     // By last update, newest first
	numSortModes                    // Keep this last - used for cycling
)

// String returns a human-readable label for the sort mode
func (s SortMode) String() string {
	switch s {
	case SortCreatedAsc:
		return "Created ↑"
	case SortCreatedDesc:
		return "Created ↓"
	case SortPriority:
		return "Priority"
	case SortUpdated:
		return "Updated"
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

// Model is the main Bubble Tea model for the beads viewer
type Model struct {
	// Data
	issues       []model.Issue
	pooledIssues []*model.Issue // Issue pool refs for sync reloads (return to pool on replace)
	issueMap     map[string]*model.Issue
	analyzer     *analysis.Analyzer
	analysis     *analysis.GraphStats
	beadsPath  string                  // Path to beads.jsonl for reloading
	dataSource *datasource.DataSource // Selected data source for refresh routing
	watcher    *watcher.Watcher       // File watcher for live reload
	instanceLock *instance.Lock   // Multi-instance coordination lock

	// Background Worker (Phase 2 architecture - bv-m7v8)
	// snapshot is the current immutable data snapshot from BackgroundWorker.
	// Access is safe without locks because Bubble Tea ensures Update() and View()
	// don't run concurrently. When nil, the UI uses legacy m.issues/m.issueMap fields.
	snapshot *DataSnapshot
	// snapshotInitPending is true until we receive the first BackgroundWorker snapshot
	// (or an error), allowing a polished cold-start loading screen (bv-tspo).
	snapshotInitPending bool
	// backgroundWorker manages async data loading (nil if background mode disabled)
	backgroundWorker *BackgroundWorker
	workerSpinnerIdx int // Spinner frame for background worker activity (bv-9nfy)
	lastForceRefresh time.Time

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

	// Focus and View State
	focused                  focus
	focusBeforeHelp          focus // Stores focus before opening help overlay
	isSplitView              bool
	splitPaneRatio           float64 // Ratio of list pane width (0.2-0.8), default 0.4
	isBoardView              bool
	isGraphView              bool
	isActionableView         bool
	isHistoryView            bool
	showDetails              bool
	showHelp                 bool
	helpScroll               int // Scroll offset for help overlay
	showQuitConfirm          bool
	ready                    bool
	width                    int
	height                   int
	showLabelHealthDetail    bool
	showLabelDrilldown       bool
	labelHealthDetail        *analysis.LabelHealth
	labelHealthDetailFlow    labelFlowSummary
	labelDrilldownLabel      string
	labelDrilldownIssues     []model.Issue
	labelDrilldownCache      map[string][]model.Issue
	showLabelGraphAnalysis   bool
	labelGraphAnalysisResult *LabelGraphAnalysisResult
	showAttentionView        bool
	showShortcutsSidebar     bool // bv-3qi5 toggleable shortcuts sidebar
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

	// Filter and sort state
	currentFilter          string
	sortMode               SortMode // bv-3ita: current sort mode
	semanticSearchEnabled  bool
	semanticIndexBuilding  bool
	semanticSearch         *SemanticSearch
	semanticHybridEnabled  bool
	semanticHybridPreset   search.PresetName
	semanticHybridBuilding bool
	semanticHybridReady    bool
	lastSearchTerm         string

	// Stats (cached)
	countOpen    int
	countReady   int
	countBlocked int
	countClosed  int

	// Priority hints
	showPriorityHints bool
	priorityHints     map[string]*analysis.PriorityRecommendation // issueID -> recommendation

	// Triage insights (bv-151)
	triageScores  map[string]float64                // issueID -> triage score
	triageReasons map[string]analysis.TriageReasons // issueID -> reasons
	unblocksMap   map[string][]string               // issueID -> IDs that would be unblocked
	quickWinSet   map[string]bool                   // issueID -> true if quick win
	blockerSet    map[string]bool                   // issueID -> true if significant blocker

	// Recipe picker
	showRecipePicker bool
	recipePicker     RecipePickerModel
	activeRecipe     *recipe.Recipe
	recipeLoader     *recipe.Loader

	// BQL query modal
	showBQLQuery  bool
	bqlQuery      BQLQueryModal
	bqlEngine     *bql.MemoryExecutor
	activeBQLExpr *bql.Query // Parsed BQL expression (nil = no BQL filter active)

	// Label picker (bv-126)
	showLabelPicker bool
	labelPicker     LabelPickerModel

	// Repo picker (workspace mode)
	showRepoPicker bool
	repoPicker     RepoPickerModel

	// Time-travel mode
	timeTravelMode   bool
	timeTravelDiff   *analysis.SnapshotDiff
	timeTravelSince  string
	newIssueIDs      map[string]bool // Issues in diff.NewIssues
	closedIssueIDs   map[string]bool // Issues in diff.ClosedIssues
	modifiedIssueIDs map[string]bool // Issues in diff.ModifiedIssues

	// Time-travel input prompt
	timeTravelInput      textinput.Model
	showTimeTravelPrompt bool

	// Status message (for temporary feedback)
	statusMsg     string
	statusIsError bool
	statusSeq     uint64 // incremented on each status set; used for auto-clear

	// Dolt connection state (bt-3ynd)
	lastDoltVerified time.Time // Last successful Dolt poll (even if no data changed)
	doltConnected    bool      // True when Dolt poll loop is healthy
	doltServer       DoltServerStopper // Dolt server lifecycle handle (bt-07jp); nil if not managed
	doltShutdownMsg  string            // Message to print after TUI exits (bt-llek)

	// Workspace mode state
	workspaceMode    bool            // True when viewing multiple repos
	availableRepos   []string        // List of repo prefixes available
	activeRepos      map[string]bool // Which repos are currently shown (nil = all)
	workspaceSummary string          // Summary text for footer (e.g., "3 repos")

	// Alerts panel (bv-168)
	alerts          []drift.Alert
	alertsCritical  int
	alertsWarning   int
	alertsInfo      int
	showAlertsPanel    bool
	alertsCursor       int
	alertsScrollOffset int
	dismissedAlerts    map[string]bool

	// Sprint view (bv-161)
	sprints        []model.Sprint
	selectedSprint *model.Sprint
	isSprintView   bool
	sprintViewText string

	// Project identity
	projectName string // Display name for the current project (directory basename)

	// AGENTS.md integration (bv-i8dk)
	showAgentPrompt  bool
	agentPromptModal AgentPromptModal
	workDir          string // Working directory for agent file detection

	// Tutorial integration (bv-8y31)
	showTutorial  bool
	tutorialModel TutorialModel

	// Cass session preview modal (bv-5bqh)
	showCassModal  bool
	cassModal      CassSessionModal
	cassCorrelator *cass.Correlator

	// Self-update modal (bv-182)
	showUpdateModal bool
	updateModal     UpdateModal
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
	flow := analysis.ComputeCrossLabelFlow(m.issues, cfg)
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
	for _, iss := range m.issues {
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
	theme := DefaultTheme(lipgloss.NewRenderer(os.Stdout))
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
	// Clear all default styles that might add extra lines
	l.Styles.Title = lipgloss.NewStyle()
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(theme.Primary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(theme.Primary)
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
	vp := viewport.New(defaultWidth, defaultHeight-2)

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
	ti.Width = 40
	ti.Prompt = "⏱️  Revision: "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Base.GetForeground())

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

	isDolt := ds != nil && ds.Type == datasource.SourceTypeDolt

	// Compute beadsDir for reconnect and port resolution
	workerBeadsDir, _ := loader.GetBeadsDir("")

	// Dolt sources always use the background worker for polling since there are
	// no files to watch. JSONL sources require explicit opt-in via BT_BACKGROUND_MODE.
	bgEnabled := (beadsPath != "" || isDolt) && (backgroundModeRequested || isDolt)
	if bgEnabled {
		bw, err := NewBackgroundWorker(WorkerConfig{
			BeadsPath:  beadsPath,
			BeadsDir:   workerBeadsDir,
			DataSource: ds,
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

	// Precompute drift/health alerts (bv-168)
	alerts, alertsCritical, alertsWarning, alertsInfo := computeAlerts(issues, graphStats, analyzer)

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

	return Model{
		issues:                 issues,
		issueMap:               issueMap,
		analyzer:               analyzer,
		analysis:               graphStats,
		beadsPath:              beadsPath,
		dataSource:             ds,
		watcher:                fileWatcher,
		snapshotInitPending:    backgroundWorker != nil,
		backgroundWorker:       backgroundWorker,
		instanceLock:           instLock,
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
		currentFilter:          "all",
		semanticSearch:         semanticSearch,
		semanticHybridEnabled:  false,
		semanticHybridPreset:   search.PresetDefault,
		semanticHybridBuilding: false,
		semanticHybridReady:    false,
		lastSearchTerm:         "",
		focused:                focusList,
		splitPaneRatio:         0.4, // Default: list pane gets 40% of width
		// Initialize as ready with default dimensions to eliminate "Initializing..." phase
		ready:               true,
		width:               defaultWidth,
		height:              defaultHeight,
		countOpen:           cOpen,
		countReady:          cReady,
		countBlocked:        cBlocked,
		countClosed:         cClosed,
		priorityHints:       priorityHints,
		showPriorityHints:   false, // Off by default, toggle with 'p'
		triageScores:        triageScores,
		triageReasons:       triageReasons,
		unblocksMap:         unblocksMap,
		quickWinSet:         quickWinSet,
		blockerSet:          blockerSet,
		recipeLoader:        recipeLoader,
		recipePicker:        recipePicker,
		activeRecipe:        activeRecipe,
		bqlQuery:            bqlQueryModal,
		bqlEngine:           bqlEngine,
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
	m.issues = newIssues
	cachedAnalyzer := analysis.NewCachedAnalyzer(newIssues, nil)
	m.analyzer = cachedAnalyzer.Analyzer
	m.analysis = cachedAnalyzer.AnalyzeAsync(context.Background())
	m.labelHealthCached = false
	m.attentionCached = false

	// Rebuild lookup map
	m.issueMap = make(map[string]*model.Issue, len(newIssues))
	for i := range m.issues {
		m.issueMap[m.issues[i].ID] = &m.issues[i]
	}

	// Clear stale priority hints
	m.priorityHints = make(map[string]*analysis.PriorityRecommendation)

	// Recompute counts
	m.countOpen, m.countReady, m.countBlocked, m.countClosed = 0, 0, 0, 0
	for i := range m.issues {
		issue := &m.issues[i]
		if isClosedLikeStatus(issue.Status) {
			m.countClosed++
			continue
		}
		m.countOpen++
		if issue.Status == model.StatusBlocked {
			m.countBlocked++
			continue
		}
		isBlocked := false
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				isBlocked = true
				break
			}
		}
		if !isBlocked {
			m.countReady++
		}
	}

	// Recompute alerts
	m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.issues, m.analysis, m.analyzer)
	m.dismissedAlerts = make(map[string]bool)
	m.showAlertsPanel = false

	// Rebuild list items
	items := make([]list.Item, len(m.issues))
	for i := range m.issues {
		item := IssueItem{
			Issue:      m.issues[i],
			GraphScore: m.analysis.GetPageRankScore(m.issues[i].ID),
			Impact:     m.analysis.GetCriticalPathScore(m.issues[i].ID),
			RepoPrefix: ExtractRepoPrefix(m.issues[i].ID),
		}
		item.TriageScore = m.triageScores[m.issues[i].ID]
		if reasons, exists := m.triageReasons[m.issues[i].ID]; exists {
			item.TriageReason = reasons.Primary
			item.TriageReasons = reasons.All
		}
		item.IsQuickWin = m.quickWinSet[m.issues[i].ID]
		item.IsBlocker = m.blockerSet[m.issues[i].ID]
		item.UnblocksCount = len(m.unblocksMap[m.issues[i].ID])
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
	m.list.SetItems(items)

	// Invalidate label-derived caches
	m.labelHealthCached = false
	m.labelDrilldownCache = make(map[string][]model.Issue)
	m.updateViewportContent()
}

// isDoltSource returns true if the model's datasource is a Dolt server.
func (m *Model) isDoltSource() bool {
	return m.dataSource != nil && m.dataSource.Type == datasource.SourceTypeDolt
}

// reloadFromDataSource returns a Cmd that reloads issues from the stored DataSource.
func (m *Model) reloadFromDataSource() tea.Cmd {
	ds := m.dataSource
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
		WaitForPhase2Cmd(m.analysis),
	}
	if m.backgroundWorker != nil {
		cmds = append(cmds, StartBackgroundWorkerCmd(m.backgroundWorker))
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		cmds = append(cmds, workerPollTickCmd())
	} else if m.watcher != nil {
		cmds = append(cmds, WatchFileCmd(m.watcher))
	}
	// Start loading history in background
	if len(m.issues) > 0 {
		cmds = append(cmds, LoadHistoryCmd(m.issuesForAsync(), m.beadsPath))
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

	if m.backgroundWorker != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			m.backgroundWorker.recordActivity()
		}
	}

	switch msg := msg.(type) {
	case UpdateMsg:
		m.updateAvailable = true
		m.updateTag = msg.TagName
		m.updateURL = msg.URL

	case UpdateCompleteMsg:
		// Forward to the update modal
		if m.showUpdateModal {
			m.updateModal, cmd = m.updateModal.Update(msg)
			cmds = append(cmds, cmd)
		}

	case UpdateProgressMsg:
		// Forward to the update modal
		if m.showUpdateModal {
			m.updateModal, cmd = m.updateModal.Update(msg)
			cmds = append(cmds, cmd)
		}

	case statusClearMsg:
		// Only clear if no newer status has been set since this timer was scheduled
		if msg.seq == m.statusSeq {
			m.statusMsg = ""
			m.statusIsError = false
		}

	case SemanticIndexReadyMsg:
		m.semanticIndexBuilding = false
		if msg.Error != nil {
			// If indexing fails, revert to fuzzy mode for predictable behavior.
			m.semanticSearchEnabled = false
			m.list.Filter = list.DefaultFilter
			m.statusMsg = fmt.Sprintf("Semantic search unavailable: %v", msg.Error)
			m.statusIsError = true
			break
		}
		if m.semanticSearch != nil {
			m.semanticSearch.SetIndex(msg.Index, msg.Embedder)
		}
		if !msg.Loaded {
			m.statusMsg = fmt.Sprintf("Semantic index built (%d embedded)", msg.Stats.Embedded)
		} else if msg.Stats.Changed() {
			m.statusMsg = fmt.Sprintf("Semantic index updated (+%d ~%d -%d)", msg.Stats.Added, msg.Stats.Updated, msg.Stats.Removed)
		} else {
			m.statusMsg = "Semantic index up to date"
		}
		m.statusIsError = false

		// Refresh current filter view if the user is actively searching.
		if m.semanticSearchEnabled && m.list.FilterState() != list.Unfiltered {
			prevState := m.list.FilterState()
			filterText := m.list.FilterInput.Value()
			m.list.SetFilterText(filterText)
			if prevState == list.Filtering {
				m.list.SetFilterState(list.Filtering)
			}
		}

	case HybridMetricsReadyMsg:
		m.semanticHybridBuilding = false
		if msg.Error != nil {
			m.semanticHybridEnabled = false
			m.semanticHybridReady = false
			if m.semanticSearch != nil {
				m.semanticSearch.SetMetricsCache(nil)
				m.semanticSearch.SetHybridConfig(false, m.semanticHybridPreset)
			}
			m.statusMsg = fmt.Sprintf("Hybrid search unavailable: %v", msg.Error)
			m.statusIsError = true
			break
		}
		if m.semanticSearch != nil && msg.Cache != nil {
			m.semanticSearch.SetMetricsCache(msg.Cache)
		}
		m.semanticHybridReady = msg.Cache != nil
		m.statusMsg = fmt.Sprintf("Hybrid search ready (%s)", m.semanticHybridPreset)
		m.statusIsError = false

		// Recompute semantic results if hybrid is enabled and search is active.
		if m.semanticHybridEnabled && m.semanticSearchEnabled && m.list.FilterState() != list.Unfiltered {
			currentTerm := m.list.FilterInput.Value()
			if currentTerm != "" {
				m.semanticSearch.ResetCache()
				cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm))
			}
		}

	case SemanticFilterResultMsg:
		// Async semantic filter results arrived - cache and refresh list
		if m.semanticSearch != nil && msg.Results != nil {
			m.semanticSearch.SetCachedResults(msg.Term, msg.Results)

			// Refresh list if still filtering with the same term
			currentTerm := m.list.FilterInput.Value()
			if m.semanticSearchEnabled && currentTerm == msg.Term {
				m.applySemanticScores(msg.Term)
				prevState := m.list.FilterState()
				m.list.SetFilterText(currentTerm)
				if prevState == list.Filtering {
					m.list.SetFilterState(list.Filtering)
				}
			}
		}

	case semanticDebounceTickMsg:
		// Debounce timer expired - check if we should trigger semantic computation
		if m.semanticSearchEnabled && m.semanticSearch != nil && m.list.FilterState() != list.Unfiltered {
			pendingTerm := m.semanticSearch.GetPendingTerm()
			if pendingTerm != "" && time.Since(m.semanticSearch.GetLastQueryTime()) >= 150*time.Millisecond {
				return m, ComputeSemanticFilterCmd(m.semanticSearch, pendingTerm)
			}
		}

	case workerPollTickMsg:
		if m.backgroundWorker != nil {
			state := m.backgroundWorker.State()
			if state == WorkerProcessing {
				m.workerSpinnerIdx = (m.workerSpinnerIdx + 1) % len(workerSpinnerFrames)
			} else {
				m.workerSpinnerIdx = 0
			}
			if state != WorkerStopped {
				cmds = append(cmds, workerPollTickCmd())
			}
		}

	case Phase2ReadyMsg:
		// Ignore stale Phase2 completions (from before a file reload)
		if msg.Stats != m.analysis {
			return m, nil
		}

		// Mark snapshot as Phase 2 ready for consistency with Phase2UpdateMsg (bv-e3ub)
		if m.snapshot != nil {
			m.snapshot.Phase2Ready = true
		}

		// Phase 2 analysis complete - update insights with full data (computed off-thread).
		ins := msg.Insights
		if m.snapshot != nil {
			m.snapshot.Insights = ins
		}
		m.insightsPanel.SetInsights(ins)
		m.insightsPanel.issueMap = m.issueMap
		bodyHeight := m.height - 1
		if bodyHeight < 5 {
			bodyHeight = 5
		}
		m.insightsPanel.SetSize(m.width, bodyHeight)
		if m.snapshot != nil {
			if m.snapshot.GraphLayout != nil {
				m.snapshot.GraphLayout.UpdatePhase2Ranks(msg.Stats)
			}
			m.graphView.SetSnapshot(m.snapshot)
		} else {
			m.graphView.SetIssues(m.issues, &ins)
		}

		// Generate triage for priority panel (bv-91) - reuse existing analyzer/stats (bv-runn.12)
		triage := analysis.ComputeTriageFromAnalyzer(m.analyzer, m.analysis, m.issues, analysis.TriageOptions{}, time.Now())
		triageScores := make(map[string]float64, len(triage.Recommendations))
		triageReasons := make(map[string]analysis.TriageReasons, len(triage.Recommendations))
		quickWinSet := make(map[string]bool, len(triage.QuickWins))
		blockerSet := make(map[string]bool, len(triage.BlockersToClear))
		unblocksMap := make(map[string][]string, len(triage.Recommendations))

		for _, rec := range triage.Recommendations {
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
		for _, qw := range triage.QuickWins {
			quickWinSet[qw.ID] = true
		}
		for _, bl := range triage.BlockersToClear {
			blockerSet[bl.ID] = true
		}

		m.triageScores = triageScores
		m.triageReasons = triageReasons
		m.quickWinSet = quickWinSet
		m.blockerSet = blockerSet
		m.unblocksMap = unblocksMap

		m.insightsPanel.SetTopPicks(triage.QuickRef.TopPicks)

		// Set full recommendations with breakdown for priority radar (bv-93)
		dataHash := fmt.Sprintf("v%s@%s#%d", triage.Meta.Version, triage.Meta.GeneratedAt.Format("15:04:05"), triage.Meta.IssueCount)
		m.insightsPanel.SetRecommendations(triage.Recommendations, dataHash)

		// Generate priority recommendations now that Phase 2 is ready
		recommendations := m.analyzer.GenerateRecommendations()
		m.priorityHints = make(map[string]*analysis.PriorityRecommendation, len(recommendations))
		for i := range recommendations {
			m.priorityHints[recommendations[i].IssueID] = &recommendations[i]
		}

		// Refresh alerts now that full Phase 2 metrics (cycles, etc.) are available
		m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.issues, m.analysis, m.analyzer)

		// Invalidate label health cache since we have new graph metrics (criticality)
		m.labelHealthCached = false
		if m.focused == focusLabelDashboard {
			cfg := analysis.DefaultLabelHealthConfig()
			m.labelHealthCache = analysis.ComputeAllLabelHealth(m.issues, cfg, time.Now().UTC(), m.analysis)
			m.labelHealthCached = true
			m.labelDashboard.SetData(m.labelHealthCache.Labels)
			m.statusMsg = fmt.Sprintf("Labels: %d total • critical %d • warning %d", m.labelHealthCache.TotalLabels, m.labelHealthCache.CriticalCount, m.labelHealthCache.WarningCount)
		}

		// Re-sort issues if sorting by Phase 2 metrics (impact/pagerank)
		if m.activeRecipe != nil {
			switch m.activeRecipe.Sort.Field {
			case "impact", "pagerank":
				field := m.activeRecipe.Sort.Field
				descending := m.activeRecipe.Sort.Direction == "desc"
				sort.Slice(m.issues, func(i, j int) bool {
					ii := m.issues[i]
					jj := m.issues[j]

					var iScore, jScore float64
					if m.analysis != nil {
						if field == "impact" {
							iScore = m.analysis.GetCriticalPathScore(ii.ID)
							jScore = m.analysis.GetCriticalPathScore(jj.ID)
						} else {
							iScore = m.analysis.GetPageRankScore(ii.ID)
							jScore = m.analysis.GetPageRankScore(jj.ID)
						}
					}

					var cmp int
					switch {
					case iScore < jScore:
						cmp = -1
					case iScore > jScore:
						cmp = 1
					}
					if cmp == 0 {
						return ii.ID < jj.ID
					}
					if descending {
						return cmp > 0
					}
					return cmp < 0
				})
				// Rebuild issueMap after re-sort (pointers become stale after sorting)
				for i := range m.issues {
					m.issueMap[m.issues[i].ID] = &m.issues[i]
				}
			}
		}

		// Re-apply recipe filter if active (to update scores while preserving filter)
		// Otherwise, update list respecting current filter (open/ready/etc.)
		if m.activeRecipe != nil {
			m.applyRecipe(m.activeRecipe)
		} else if m.currentFilter == "" || m.currentFilter == "all" {
			m.refreshListItemsPhase2()
		} else {
			m.applyFilter()
		}

	case Phase2UpdateMsg:
		// BackgroundWorker notifies that Phase 2 analysis is complete (bv-e3ub)
		// Verify this update matches the current snapshot using DataHash
		if m.snapshot == nil || m.snapshot.DataHash != msg.DataHash {
			// Stale update - ignore
			if m.backgroundWorker != nil {
				return m, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker)
			}
			return m, nil
		}

		// Mark snapshot as Phase 2 ready
		m.snapshot.Phase2Ready = true

		// Note: Phase2ReadyMsg handler (via WaitForPhase2Cmd) already handles
		// all the UI updates (insights, graph view, alerts, etc.). This message
		// is a complementary notification from the BackgroundWorker that Phase 2
		// completed. If Phase2ReadyMsg hasn't fired yet, it will handle the full
		// UI refresh. If it already fired (race condition), this is a no-op.
		if m.backgroundWorker != nil {
			return m, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker)
		}
		return m, nil

	case HistoryLoadedMsg:
		// Background history loading completed
		m.historyLoading = false
		if msg.Error != nil {
			m.historyLoadFailed = true
			m.statusMsg = fmt.Sprintf("History load failed: %v", msg.Error)
			m.statusIsError = true
		} else if msg.Report != nil {
			m.historyView = NewHistoryModel(msg.Report, m.theme)
			m.historyView.SetSize(m.width, m.height-1)
			// Refresh detail pane if visible
			if m.isSplitView || m.showDetails {
				m.updateViewportContent()
			}
		}

	case AgentFileCheckMsg:
		// AGENTS.md integration check (bv-i8dk)
		if msg.ShouldPrompt && msg.FilePath != "" {
			m.showAgentPrompt = true
			m.agentPromptModal = NewAgentPromptModal(msg.FilePath, msg.FileType, m.theme)
			m.focused = focusAgentPrompt
		}

	case SnapshotReadyMsg:
		// Background worker has a new snapshot ready (bv-m7v8)
		// This is the atomic pointer swap - O(1), sub-microsecond
		if msg.Snapshot == nil {
			if m.backgroundWorker != nil {
				return m, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker)
			}
			return m, nil
		}

		firstSnapshot := m.snapshotInitPending && m.snapshot == nil
		m.snapshotInitPending = false

		// Clear ephemeral overlays tied to old data
		m.clearAttentionOverlay()

		// Exit time-travel mode if active (file changed, show current state)
		if m.timeTravelMode {
			m.timeTravelMode = false
			m.timeTravelDiff = nil
			m.timeTravelSince = ""
			m.newIssueIDs = nil
			m.closedIssueIDs = nil
			m.modifiedIssueIDs = nil
		}

		// Store selected issue ID to restore position after swap
		var selectedID string
		if sel := m.list.SelectedItem(); sel != nil {
			if item, ok := sel.(IssueItem); ok {
				selectedID = item.Issue.ID
			}
		}

		// Preserve board selection by issue ID (bv-6n4c).
		var boardSelectedID string
		if m.focused == focusBoard {
			if sel := m.board.SelectedIssue(); sel != nil {
				boardSelectedID = sel.ID
			}
		}

		oldSnapshot := m.snapshot

		// Swap snapshot pointer
		m.snapshot = msg.Snapshot
		if m.backgroundWorker != nil {
			latencyStart := msg.FileChangeAt
			if latencyStart.IsZero() {
				latencyStart = msg.SentAt
			}
			if !latencyStart.IsZero() {
				m.backgroundWorker.recordUIUpdateLatency(time.Since(latencyStart))
			}
		}
		if oldSnapshot != nil && len(oldSnapshot.pooledIssues) > 0 {
			go loader.ReturnIssuePtrsToPool(oldSnapshot.pooledIssues)
		}

		// Update legacy fields for backwards compatibility during migration
		// Eventually these will be removed when all code reads from snapshot
		m.issues = msg.Snapshot.Issues
		m.issueMap = msg.Snapshot.IssueMap
		m.analyzer = msg.Snapshot.Analyzer
		m.analysis = msg.Snapshot.Analysis
		m.countOpen = msg.Snapshot.CountOpen
		m.countReady = msg.Snapshot.CountReady
		m.countBlocked = msg.Snapshot.CountBlocked
		m.countClosed = msg.Snapshot.CountClosed
		if len(m.pooledIssues) > 0 {
			go loader.ReturnIssuePtrsToPool(m.pooledIssues)
			m.pooledIssues = nil
		}
		// Preserve existing triage data unless the snapshot has Phase 2 results.
		// Avoid flicker when Phase 1 snapshots arrive without triage data.
		if msg.Snapshot.Phase2Ready || len(msg.Snapshot.TriageScores) > 0 {
			m.triageScores = msg.Snapshot.TriageScores
			m.triageReasons = msg.Snapshot.TriageReasons
			m.unblocksMap = msg.Snapshot.UnblocksMap
			m.quickWinSet = msg.Snapshot.QuickWinSet
			m.blockerSet = msg.Snapshot.BlockerSet
		}

		// Clear caches that need recomputation
		m.labelHealthCached = false
		m.attentionCached = false
		m.priorityHints = make(map[string]*analysis.PriorityRecommendation)
		m.labelDrilldownCache = make(map[string][]model.Issue)

		// Recompute alerts for refreshed dataset
		m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.issues, m.analysis, m.analyzer)
		m.dismissedAlerts = make(map[string]bool)
		m.showAlertsPanel = false

		// Reset semantic caches for the new dataset.
		if m.semanticSearch != nil {
			m.semanticSearch.ResetCache()
			m.semanticSearch.SetMetricsCache(nil)
		}
		m.semanticHybridReady = false
		m.semanticHybridBuilding = false
		if m.semanticHybridEnabled {
			m.semanticHybridBuilding = true
			cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
		}

		// Regenerate sub-views (Phase 1 data; Phase 2 will update via Phase2ReadyMsg)
		m.insightsPanel.SetInsights(m.snapshot.Insights)
		m.insightsPanel.issueMap = m.issueMap
		bodyHeight := m.height - 1
		if bodyHeight < 5 {
			bodyHeight = 5
		}
		m.insightsPanel.SetSize(m.width, bodyHeight)

		// Update list/board/graph views while preserving the current recipe/filter state.
		if m.activeRecipe != nil {
			// If the snapshot already includes recipe filtering/sorting, use it directly (bv-cwwd).
			if msg.Snapshot.RecipeName == m.activeRecipe.Name && msg.Snapshot.RecipeHash == recipeFingerprint(m.activeRecipe) {
				filteredItems := make([]list.Item, 0, len(msg.Snapshot.ListItems))
				filteredIssues := make([]model.Issue, 0, len(msg.Snapshot.ListItems))

				for _, item := range msg.Snapshot.ListItems {
					issue := item.Issue

					// Workspace repo filter (nil = all repos)
					if m.workspaceMode && m.activeRepos != nil {
						repoKey := strings.ToLower(item.RepoPrefix)
						if repoKey != "" && !m.activeRepos[repoKey] {
							continue
						}
					}

					filteredItems = append(filteredItems, item)
					filteredIssues = append(filteredIssues, issue)
				}

				m.list.SetItems(filteredItems)
				m.updateSemanticIDs(filteredItems)
				m.board.SetIssues(filteredIssues)

				recipeIns := analysis.Insights{}
				if m.analysis != nil {
					recipeIns = m.analysis.GenerateInsights(len(filteredIssues))
				}
				m.graphView.SetIssues(filteredIssues, &recipeIns)

				m.currentFilter = "recipe:" + m.activeRecipe.Name

				// Keep selection in bounds
				if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
					m.list.Select(0)
				}
			} else {
				m.applyRecipe(m.activeRecipe)
			}
		} else {
			var filteredItems []list.Item
			var filteredIssues []model.Issue

			filteredItems = make([]list.Item, 0, len(msg.Snapshot.ListItems))
			filteredIssues = make([]model.Issue, 0, len(msg.Snapshot.ListItems))

			for _, item := range msg.Snapshot.ListItems {
				issue := item.Issue

				// Workspace repo filter (nil = all repos)
				if m.workspaceMode && m.activeRepos != nil {
					repoKey := strings.ToLower(item.RepoPrefix)
					if repoKey != "" && !m.activeRepos[repoKey] {
						continue
					}
				}

				include := false
				switch m.currentFilter {
				case "all":
					include = true
				case "open":
					include = !isClosedLikeStatus(issue.Status)
				case "closed":
					include = isClosedLikeStatus(issue.Status)
				case "ready":
					// Ready = Open/InProgress AND NO Open Blockers
					if !isClosedLikeStatus(issue.Status) && issue.Status != model.StatusBlocked {
						isBlocked := false
						for _, dep := range issue.Dependencies {
							if dep == nil || !dep.Type.IsBlocking() {
								continue
							}
							if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
								isBlocked = true
								break
							}
						}
						include = !isBlocked
					}
				default:
					if strings.HasPrefix(m.currentFilter, "label:") {
						label := strings.TrimPrefix(m.currentFilter, "label:")
						for _, l := range issue.Labels {
							if l == label {
								include = true
								break
							}
						}
					}
				}

				if include {
					filteredItems = append(filteredItems, item)
					filteredIssues = append(filteredIssues, issue)
				}
			}

			m.sortFilteredItems(filteredItems, filteredIssues)
			m.list.SetItems(filteredItems)
			m.updateSemanticIDs(filteredItems)
			if m.snapshot != nil && m.snapshot.BoardState != nil && (!m.workspaceMode || m.activeRepos == nil) && len(filteredIssues) == len(m.snapshot.Issues) {
				m.board.SetSnapshot(m.snapshot)
			} else {
				m.board.SetIssues(filteredIssues)
			}
			if m.snapshot != nil && m.snapshot.GraphLayout != nil && len(filteredIssues) == len(m.snapshot.Issues) {
				m.graphView.SetSnapshot(m.snapshot)
			} else {
				m.graphView.SetIssues(filteredIssues, &m.snapshot.Insights)
			}

			// Restore selection if possible
			if selectedID != "" {
				for i, it := range filteredItems {
					if item, ok := it.(IssueItem); ok && item.Issue.ID == selectedID {
						m.list.Select(i)
						break
					}
				}
			}

			// Keep selection in bounds
			if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
				m.list.Select(0)
			}
		}

		// Restore selection in recipe mode (applyRecipe rebuilds list items)
		if m.activeRecipe != nil && selectedID != "" {
			items := m.list.Items()
			for i := range items {
				if item, ok := items[i].(IssueItem); ok && item.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
		}

		// Restore board selection after SetIssues/applyRecipe rebuilds columns (bv-6n4c).
		if boardSelectedID != "" {
			_ = m.board.SelectIssueByID(boardSelectedID)
		}

		// If the tree view is active, rebuild it from the new snapshot while preserving
		// user state (selection + persisted expand/collapse) (bv-6n4c).
		if m.focused == focusTree {
			m.tree.BuildFromSnapshot(m.snapshot)
			m.tree.SetSize(m.width, m.height-2)
		}

		// Refresh detail pane if visible
		if m.isSplitView || m.showDetails {
			m.updateViewportContent()
		}

		// Keep semantic index current when enabled.
		if m.semanticSearchEnabled && !m.semanticIndexBuilding {
			m.semanticIndexBuilding = true
			cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
		}

		// Reload sprints (bv-161)
		if m.beadsPath != "" {
			beadsDir := filepath.Dir(m.beadsPath)
			if loaded, err := loader.LoadSprintsFromFile(filepath.Join(beadsDir, loader.SprintsFileName)); err == nil {
				m.sprints = loaded
				// If we have a selected sprint, try to refresh it
				if m.selectedSprint != nil {
					found := false
					for i := range m.sprints {
						if m.sprints[i].ID == m.selectedSprint.ID {
							m.selectedSprint = &m.sprints[i]
							m.sprintViewText = m.renderSprintDashboard()
							found = true
							break
						}
					}
					if !found {
						m.selectedSprint = nil
						m.sprintViewText = "Sprint not found"
					}
				}
			}
		}

		if firstSnapshot {
			// For the initial background snapshot, avoid flashing "Reloaded" at startup.
			if msg.Snapshot.LoadWarningCount > 0 {
				cmds = append(cmds, m.setTransientStatus(
					fmt.Sprintf("Loaded %d issues (%d warnings)", len(m.issues), msg.Snapshot.LoadWarningCount), 3*time.Second))
			} else {
				m.statusMsg = ""
			}
		} else if msg.Snapshot.LoadWarningCount > 0 {
			cmds = append(cmds, m.setTransientStatus(
				fmt.Sprintf("Reloaded %d issues (%d warnings)", len(m.issues), msg.Snapshot.LoadWarningCount), 3*time.Second))
		} else {
			cmds = append(cmds, m.setTransientStatus(
				fmt.Sprintf("Reloaded %d issues", len(m.issues)), 3*time.Second))
		}

		// Wait for Phase 2 if not ready
		if msg.Snapshot.Analysis != nil {
			cmds = append(cmds, WaitForPhase2Cmd(msg.Snapshot.Analysis))
		}

		if m.backgroundWorker != nil {
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		}

		return m, tea.Batch(cmds...)

	case SnapshotErrorMsg:
		// Background worker encountered an error loading/processing data
		// If recoverable, we'll try again on next file change.
		if m.snapshotInitPending && m.snapshot == nil {
			m.snapshotInitPending = false
		}
		if msg.Err != nil {
			if msg.Recoverable {
				m.statusMsg = fmt.Sprintf("Reload error (retrying): %s", shortError(msg.Err))
			} else {
				m.statusMsg = fmt.Sprintf("Reload error: %s", shortError(msg.Err))
			}
			m.statusIsError = true
		}
		if m.backgroundWorker != nil {
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		}
		return m, tea.Batch(cmds...)

	case DataSourceReloadMsg:
		// Async reload from a non-file datasource (e.g. Dolt) completed.
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Reload failed: %s", shortError(msg.Err))
			m.statusIsError = true
			return m, tea.Batch(cmds...)
		}
		m.replaceIssues(msg.Issues)
		cmds = append(cmds, m.setTransientStatus(
			fmt.Sprintf("Reloaded %d issues", len(msg.Issues)), 3*time.Second))
		cmds = append(cmds, WaitForPhase2Cmd(m.analysis))
		return m, tea.Batch(cmds...)

	case DoltVerifiedMsg:
		// Dolt poll succeeded - data is verified current (bt-3ynd).
		m.lastDoltVerified = msg.At
		m.doltConnected = true
		if m.backgroundWorker != nil {
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		}
		return m, tea.Batch(cmds...)

	case DoltConnectionStatusMsg:
		// Dolt poll loop reporting connectivity change (bv-1p3a).
		if msg.Connected {
			m.doltConnected = true
			m.statusMsg = "Dolt server reconnected"
			m.statusIsError = false
		} else {
			m.doltConnected = false
			m.statusMsg = fmt.Sprintf("Dolt server unreachable (retrying in %ds)", msg.BackoffSeconds)
			m.statusIsError = true
		}
		if m.backgroundWorker != nil {
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		}
		return m, tea.Batch(cmds...)

	case FileChangedMsg:
		// File changed on disk - reload issues and recompute analysis
		// In background mode the BackgroundWorker owns file watching and snapshot building.
		if m.backgroundWorker != nil {
			if m.watcher != nil {
				cmds = append(cmds, WatchFileCmd(m.watcher))
			}
			return m, tea.Batch(cmds...)
		}
		if m.beadsPath == "" {
			// Re-start watch for next change
			if m.watcher != nil {
				cmds = append(cmds, WatchFileCmd(m.watcher))
			}
			return m, tea.Batch(cmds...)
		}
		reloadStart := time.Now()
		profileRefresh := debug.Enabled()
		var refreshTimings map[string]time.Duration
		recordTiming := func(name string, d time.Duration) {
			if !profileRefresh {
				return
			}
			if refreshTimings == nil {
				refreshTimings = make(map[string]time.Duration, 12)
			}
			refreshTimings[name] = d
			debug.LogTiming("refresh."+name, d)
		}
		if profileRefresh {
			debug.Log("refresh: file change detected path=%s", m.beadsPath)
		}

		// Clear ephemeral overlays tied to old data
		m.clearAttentionOverlay()

		// Exit time-travel mode if active (file changed, show current state)
		if m.timeTravelMode {
			m.timeTravelMode = false
			m.timeTravelDiff = nil
			m.timeTravelSince = ""
			m.newIssueIDs = nil
			m.closedIssueIDs = nil
			m.modifiedIssueIDs = nil
		}

		// Reload issues from disk
		// Use custom warning handler to prevent stderr pollution during TUI render (bv-fix)
		var reloadWarnings []string
		var loadStart time.Time
		if profileRefresh {
			loadStart = time.Now()
		}
		loadedIssues, err := loader.LoadIssuesFromFileWithOptionsPooled(m.beadsPath, loader.ParseOptions{
			WarningHandler: func(msg string) {
				reloadWarnings = append(reloadWarnings, msg)
			},
			BufferSize: envMaxLineSizeBytes(),
		})
		if profileRefresh {
			recordTiming("load_issues", time.Since(loadStart))
		}
		if err != nil {
			m.statusMsg = fmt.Sprintf("Reload error: %v", err)
			m.statusIsError = true
			// Re-start watch for next change
			if m.watcher != nil {
				cmds = append(cmds, WatchFileCmd(m.watcher))
			}
			return m, tea.Batch(cmds...)
		}
		if len(m.pooledIssues) > 0 {
			loader.ReturnIssuePtrsToPool(m.pooledIssues)
		}
		m.pooledIssues = loadedIssues.PoolRefs
		newIssues := loadedIssues.Issues

		// Store selected issue ID to restore position after reload
		var selectedID string
		if sel := m.list.SelectedItem(); sel != nil {
			if item, ok := sel.(IssueItem); ok {
				selectedID = item.Issue.ID
			}
		}

		// Apply default sorting (Open first, Priority, Date)
		var sortStart time.Time
		if profileRefresh {
			sortStart = time.Now()
		}
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
		if profileRefresh {
			recordTiming("sort_issues", time.Since(sortStart))
		}

		// Recompute analysis (async Phase 1/Phase 2) with caching
		m.issues = newIssues
		var analysisStart time.Time
		if profileRefresh {
			analysisStart = time.Now()
		}
		cachedAnalyzer := analysis.NewCachedAnalyzer(newIssues, nil)
		m.analyzer = cachedAnalyzer.Analyzer
		m.analysis = cachedAnalyzer.AnalyzeAsync(context.Background())
		cacheHit := cachedAnalyzer.WasCacheHit()
		if profileRefresh {
			recordTiming("phase1_setup", time.Since(analysisStart))
			debug.Log("refresh.phase1_cache_hit=%t issues=%d", cacheHit, len(newIssues))
		}
		m.labelHealthCached = false
		m.attentionCached = false

		// Rebuild lookup map
		var mapStart time.Time
		if profileRefresh {
			mapStart = time.Now()
		}
		m.issueMap = make(map[string]*model.Issue, len(newIssues))
		for i := range m.issues {
			m.issueMap[m.issues[i].ID] = &m.issues[i]
		}
		if profileRefresh {
			recordTiming("issue_map", time.Since(mapStart))
		}

		// Clear stale priority hints (will be repopulated after Phase 2)
		m.priorityHints = make(map[string]*analysis.PriorityRecommendation)

		// Recompute stats
		var statsStart time.Time
		if profileRefresh {
			statsStart = time.Now()
		}
		m.countOpen, m.countReady, m.countBlocked, m.countClosed = 0, 0, 0, 0
		for i := range m.issues {
			issue := &m.issues[i]
			if isClosedLikeStatus(issue.Status) {
				m.countClosed++
				continue
			}
			m.countOpen++
			if issue.Status == model.StatusBlocked {
				m.countBlocked++
				continue
			}
			isBlocked := false
			for _, dep := range issue.Dependencies {
				if dep == nil || !dep.Type.IsBlocking() {
					continue
				}
				if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
					isBlocked = true
					break
				}
			}
			if !isBlocked {
				m.countReady++
			}
		}
		if profileRefresh {
			recordTiming("counts", time.Since(statsStart))
		}

		// Recompute alerts for refreshed dataset
		var alertsStart time.Time
		if profileRefresh {
			alertsStart = time.Now()
		}
		m.alerts, m.alertsCritical, m.alertsWarning, m.alertsInfo = computeAlerts(m.issues, m.analysis, m.analyzer)
		if profileRefresh {
			recordTiming("alerts", time.Since(alertsStart))
		}
		m.dismissedAlerts = make(map[string]bool)
		m.showAlertsPanel = false

		// Rebuild list items (preserve triage data to avoid flicker)
		var listStart time.Time
		if profileRefresh {
			listStart = time.Now()
		}
		items := make([]list.Item, len(m.issues))
		for i := range m.issues {
			item := IssueItem{
				Issue:      m.issues[i],
				GraphScore: m.analysis.GetPageRankScore(m.issues[i].ID),
				Impact:     m.analysis.GetCriticalPathScore(m.issues[i].ID),
				RepoPrefix: ExtractRepoPrefix(m.issues[i].ID),
			}
			item.TriageScore = m.triageScores[m.issues[i].ID]
			if reasons, exists := m.triageReasons[m.issues[i].ID]; exists {
				item.TriageReason = reasons.Primary
				item.TriageReasons = reasons.All
			}
			item.IsQuickWin = m.quickWinSet[m.issues[i].ID]
			item.IsBlocker = m.blockerSet[m.issues[i].ID]
			item.UnblocksCount = len(m.unblocksMap[m.issues[i].ID])
			items[i] = item
		}
		if profileRefresh {
			recordTiming("list_items", time.Since(listStart))
		}
		m.updateSemanticIDs(items)
		m.clearSemanticScores()
		if m.semanticSearch != nil {
			m.semanticSearch.ResetCache()
			m.semanticSearch.SetMetricsCache(nil)
		}
		m.semanticHybridReady = false
		m.semanticHybridBuilding = false
		if m.semanticHybridEnabled {
			m.semanticHybridBuilding = true
			cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
		}
		m.list.SetItems(items)

		// Restore selection position
		if selectedID != "" {
			for i, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
		}

		// Regenerate sub-views (with Phase 1 data; Phase 2 will update via Phase2ReadyMsg)
		// Preserve triage data already computed to avoid UI flicker.
		needsInsights := m.focused == focusInsights && !m.showAttentionView
		needsGraph := m.isGraphView
		var ins analysis.Insights
		if needsInsights || needsGraph {
			var insightsStart time.Time
			if profileRefresh {
				insightsStart = time.Now()
			}
			ins = m.analysis.GenerateInsights(len(m.issues))
			if profileRefresh {
				recordTiming("insights_generate", time.Since(insightsStart))
			}
		}
		if needsInsights {
			oldTopPicks := m.insightsPanel.topPicks
			oldRecs := m.insightsPanel.recommendations
			oldRecMap := m.insightsPanel.recommendationMap
			oldHash := m.insightsPanel.triageDataHash

			m.insightsPanel = NewInsightsModel(ins, m.issueMap, m.theme)
			m.insightsPanel.topPicks = oldTopPicks
			m.insightsPanel.recommendations = oldRecs
			m.insightsPanel.recommendationMap = oldRecMap
			m.insightsPanel.triageDataHash = oldHash
			bodyHeight := m.height - 1
			if bodyHeight < 5 {
				bodyHeight = 5
			}
			m.insightsPanel.SetSize(m.width, bodyHeight)
		}
		if m.showAttentionView {
			var attentionStart time.Time
			if profileRefresh {
				attentionStart = time.Now()
			}
			cfg := analysis.DefaultLabelHealthConfig()
			m.attentionCache = analysis.ComputeLabelAttentionScores(m.issues, cfg, time.Now().UTC())
			m.attentionCached = true
			attText, _ := ComputeAttentionView(m.issues, max(40, m.width-4))
			m.insightsPanel = NewInsightsModel(analysis.Insights{}, m.issueMap, m.theme)
			m.insightsPanel.labelAttention = m.attentionCache.Labels
			m.insightsPanel.extraText = attText
			panelHeight := m.height - 2
			if panelHeight < 3 {
				panelHeight = 3
			}
			m.insightsPanel.SetSize(m.width, panelHeight)
			if profileRefresh {
				recordTiming("attention_view", time.Since(attentionStart))
			}
		}
		if needsGraph || m.isBoardView {
			var graphStart time.Time
			if profileRefresh {
				graphStart = time.Now()
			}
			m.refreshBoardAndGraphForCurrentFilter()
			if profileRefresh {
				recordTiming("board_graph", time.Since(graphStart))
			}
		}

		// Re-apply recipe filter if active
		if m.activeRecipe != nil {
			m.applyRecipe(m.activeRecipe)
		}

		// Re-apply BQL filter if active
		if m.activeBQLExpr != nil && strings.HasPrefix(m.currentFilter, "bql:") {
			queryStr := strings.TrimPrefix(m.currentFilter, "bql:")
			m.applyBQL(m.activeBQLExpr, queryStr)
		}

		// Reload sprints (bv-161)
		if m.beadsPath != "" {
			beadsDir := filepath.Dir(m.beadsPath)
			if loaded, err := loader.LoadSprintsFromFile(filepath.Join(beadsDir, loader.SprintsFileName)); err == nil {
				m.sprints = loaded
				// If we have a selected sprint, try to refresh it
				if m.selectedSprint != nil {
					found := false
					for i := range m.sprints {
						if m.sprints[i].ID == m.selectedSprint.ID {
							m.selectedSprint = &m.sprints[i]
							m.sprintViewText = m.renderSprintDashboard()
							found = true
							break
						}
					}
					if !found {
						m.selectedSprint = nil
						m.sprintViewText = "Sprint not found"
					}
				}
			}
		}

		// Keep semantic index current when enabled.
		if m.semanticSearchEnabled && !m.semanticIndexBuilding {
			m.semanticIndexBuilding = true
			cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
		}

		if cacheHit {
			m.statusMsg = fmt.Sprintf("Reloaded %d issues (cached)", len(newIssues))
		} else {
			m.statusMsg = fmt.Sprintf("Reloaded %d issues", len(newIssues))
		}
		if len(reloadWarnings) > 0 {
			m.statusMsg += fmt.Sprintf(" (%d warnings)", len(reloadWarnings))
		}
		reloadDuration := time.Since(reloadStart)
		if profileRefresh {
			recordTiming("total", reloadDuration)
		}
		if reloadDuration >= 500*time.Millisecond {
			m.statusMsg += fmt.Sprintf(" in %s", formatReloadDuration(reloadDuration))
		}
		if profileRefresh && len(refreshTimings) > 0 {
			addTiming := func(label, key string) {
				if d, ok := refreshTimings[key]; ok && d > 0 {
					m.statusMsg += fmt.Sprintf(" %s=%s", label, formatReloadDuration(d))
				}
			}
			m.statusMsg += " [debug"
			addTiming("load", "load_issues")
			addTiming("sort", "sort_issues")
			addTiming("phase1", "phase1_setup")
			addTiming("alerts", "alerts")
			addTiming("list", "list_items")
			addTiming("graph", "board_graph")
			addTiming("total", "total")
			m.statusMsg += "]"
		}
		// Auto-enable background mode after slow sync reloads (opt-out via BT_BACKGROUND_MODE=0).
		autoEnabled := false
		slowReload := reloadDuration >= time.Second
		if slowReload && m.backgroundWorker == nil && m.beadsPath != "" {
			autoAllowed := true
			if v := strings.TrimSpace(os.Getenv("BT_BACKGROUND_MODE")); v != "" {
				switch strings.ToLower(v) {
				case "0", "false", "no", "off":
					autoAllowed = false
				}
			}
			if autoAllowed {
				autoBeadsDir := ""
				if m.beadsPath != "" {
					autoBeadsDir = filepath.Dir(m.beadsPath)
				}
				bw, err := NewBackgroundWorker(WorkerConfig{
					BeadsPath:     m.beadsPath,
					BeadsDir:      autoBeadsDir,
					DataSource:    m.dataSource,
					DebounceDelay: 200 * time.Millisecond,
				})
				if err == nil {
					if m.watcher != nil {
						m.watcher.Stop()
					}
					m.watcher = nil
					m.backgroundWorker = bw
					m.snapshotInitPending = true
					autoEnabled = true
					cmds = append(cmds, StartBackgroundWorkerCmd(m.backgroundWorker))
					cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
				} else {
					m.statusMsg += fmt.Sprintf("; background mode unavailable: %v", err)
				}
			}
		}
		if slowReload {
			if autoEnabled {
				m.statusMsg += "; background mode auto-enabled"
			} else {
				m.statusMsg += "; consider BT_BACKGROUND_MODE=1"
			}
		}
		m.statusIsError = false
		// Schedule auto-clear of the reload status message
		m.statusSeq++
		seq := m.statusSeq
		cmds = append(cmds, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
			return statusClearMsg{seq: seq}
		}))
		// Invalidate label-derived caches
		m.labelHealthCached = false
		m.labelDrilldownCache = make(map[string][]model.Issue)
		m.updateViewportContent()

		// Re-start watching for next change + wait for Phase 2
		if m.watcher != nil && !autoEnabled {
			cmds = append(cmds, WatchFileCmd(m.watcher))
		}
		cmds = append(cmds, WaitForPhase2Cmd(m.analysis))
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Clear status message on any keypress
		m.statusMsg = ""
		m.statusIsError = false

		// Handle AGENTS.md prompt modal (bv-i8dk)
		if m.showAgentPrompt {
			m.agentPromptModal, cmd = m.agentPromptModal.Update(msg)
			cmds = append(cmds, cmd)

			// Check if user made a decision
			switch m.agentPromptModal.Result() {
			case AgentPromptAccept:
				// User accepted - add blurb to file
				filePath := m.agentPromptModal.FilePath()
				if err := agents.AppendBlurbToFile(filePath); err != nil {
					m.statusMsg = "Failed to update " + filepath.Base(filePath) + ": " + err.Error()
					m.statusIsError = true
				} else {
					m.statusMsg = "✓ Added beads instructions to " + filepath.Base(filePath)
					// Record acceptance
					_ = agents.RecordAccept(m.workDir)
				}
				m.showAgentPrompt = false
				m.focused = focusList
			case AgentPromptDecline:
				// User declined - just dismiss, may ask again next time
				m.showAgentPrompt = false
				m.focused = focusList
			case AgentPromptNeverAsk:
				// User chose "don't ask again" - save preference
				_ = agents.RecordDecline(m.workDir, true)
				m.showAgentPrompt = false
				m.focused = focusList
			}
			return m, tea.Batch(cmds...)
		}

		// Handle cass session modal (bv-5bqh)
		if m.showCassModal {
			m.cassModal, cmd = m.cassModal.Update(msg)
			cmds = append(cmds, cmd)

			// Check for dismiss keys
			switch msg.String() {
			case "V", "esc", "enter", "q":
				m.showCassModal = false
				m.focused = focusList
				return m, tea.Batch(cmds...)
			}
			return m, tea.Batch(cmds...)
		}

		// Handle self-update modal (bv-182)
		if m.showUpdateModal {
			m.updateModal, cmd = m.updateModal.Update(msg)
			cmds = append(cmds, cmd)

			// Handle modal state changes
			switch msg.String() {
			case "esc", "q":
				// Always allow escape to close
				if !m.updateModal.IsInProgress() {
					m.showUpdateModal = false
					m.focused = focusList
					return m, tea.Batch(cmds...)
				}
			case "enter":
				// Close on enter if complete or if cancelled
				if m.updateModal.IsComplete() {
					m.showUpdateModal = false
					m.focused = focusList
					return m, tea.Batch(cmds...)
				}
				// If confirming and cancelled, close
				if m.updateModal.IsConfirming() && m.updateModal.IsCancelled() {
					m.showUpdateModal = false
					m.focused = focusList
					return m, tea.Batch(cmds...)
				}
			case "n", "N":
				// Quick cancel
				if m.updateModal.IsConfirming() {
					m.showUpdateModal = false
					m.focused = focusList
					return m, tea.Batch(cmds...)
				}
			}
			return m, tea.Batch(cmds...)
		}

		// Close label health detail modal if open
		if m.showLabelHealthDetail {
			s := msg.String()
			if s == "esc" || s == "q" || s == "enter" || s == "h" {
				m.showLabelHealthDetail = false
				m.labelHealthDetail = nil
				return m, nil
			}
			if s == "d" && m.labelHealthDetail != nil {
				// open drilldown from detail modal
				m.labelDrilldownLabel = m.labelHealthDetail.Label
				m.labelDrilldownIssues = m.filterIssuesByLabel(m.labelDrilldownLabel)
				m.showLabelDrilldown = true
				m.showLabelHealthDetail = false
				return m, nil
			}
		}

		// Handle label drilldown modal if open
		if m.showLabelDrilldown {
			s := msg.String()
			switch s {
			case "enter":
				// Apply label filter to main list and close drilldown
				if m.labelDrilldownLabel != "" {
					m.currentFilter = "label:" + m.labelDrilldownLabel
					m.applyFilter()
					m.focused = focusList
				}
				m.showLabelDrilldown = false
				m.labelDrilldownLabel = ""
				m.labelDrilldownIssues = nil
				return m, nil
			case "g":
				// Show graph analysis sub-view (bv-109)
				if m.labelDrilldownLabel != "" {
					sg := analysis.ComputeLabelSubgraph(m.issues, m.labelDrilldownLabel)
					pr := analysis.ComputeLabelPageRank(sg)
					cp := analysis.ComputeLabelCriticalPath(sg)
					m.labelGraphAnalysisResult = &LabelGraphAnalysisResult{
						Label:        m.labelDrilldownLabel,
						Subgraph:     sg,
						PageRank:     pr,
						CriticalPath: cp,
					}
					m.showLabelGraphAnalysis = true
				}
				return m, nil
			case "esc", "q", "d":
				m.showLabelDrilldown = false
				m.labelDrilldownLabel = ""
				m.labelDrilldownIssues = nil
				return m, nil
			}
		}

		// Handle label graph analysis sub-view (bv-109)
		if m.showLabelGraphAnalysis {
			s := msg.String()
			switch s {
			case "esc", "q", "g":
				m.showLabelGraphAnalysis = false
				m.labelGraphAnalysisResult = nil
				return m, nil
			}
		}

		// Handle attention view quick jumps (bv-117)
		if m.showAttentionView {
			s := msg.String()
			switch {
			case s == "esc" || s == "q" || s == "d":
				m.showAttentionView = false
				m.insightsPanel.extraText = ""
				return m, nil
			case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
				if len(m.attentionCache.Labels) == 0 {
					return m, nil
				}
				idx := int(s[0] - '1')
				if idx >= 0 && idx < len(m.attentionCache.Labels) {
					label := m.attentionCache.Labels[idx].Label
					m.currentFilter = "label:" + label
					m.applyFilter()
					m.statusMsg = fmt.Sprintf("Filtered to label %s (attention #%d)", label, idx+1)
					m.statusIsError = false
				}
				return m, nil
			}
		}

		// Handle alerts panel modal if open (bv-168)
		if m.showAlertsPanel {
			// Build list of active (non-dismissed) alerts
			var activeAlerts []drift.Alert
			for _, a := range m.alerts {
				if !m.dismissedAlerts[alertKey(a)] {
					activeAlerts = append(activeAlerts, a)
				}
			}
			s := msg.String()
			switch s {
			case "j", "down":
				if m.alertsCursor < len(activeAlerts)-1 {
					m.alertsCursor++
					// Scroll down if cursor moves past visible area
					visLines := m.alertsVisibleLines()
					if visLines > 0 && m.alertsCursor >= m.alertsScrollOffset+visLines {
						m.alertsScrollOffset = m.alertsCursor - visLines + 1
					}
				}
				return m, nil
			case "k", "up":
				if m.alertsCursor > 0 {
					m.alertsCursor--
					// Scroll up if cursor moves above visible area
					if m.alertsCursor < m.alertsScrollOffset {
						m.alertsScrollOffset = m.alertsCursor
					}
				}
				return m, nil
			case "enter":
				// Jump to the issue referenced by the selected alert
				if m.alertsCursor < len(activeAlerts) {
					issueID := activeAlerts[m.alertsCursor].IssueID
					if issueID != "" {
						// Find the issue in the list and select it
						for i, item := range m.list.Items() {
							if it, ok := item.(IssueItem); ok && it.Issue.ID == issueID {
								m.list.Select(i)
								break
							}
						}
					}
				}
				m.showAlertsPanel = false
				return m, nil
			case "c":
				// Clear the selected alert
				if m.alertsCursor < len(activeAlerts) {
					key := alertKey(activeAlerts[m.alertsCursor])
					m.dismissedAlerts[key] = true
					// Adjust cursor if needed
					remaining := 0
					for _, a := range m.alerts {
						if !m.dismissedAlerts[alertKey(a)] {
							remaining++
						}
					}
					if m.alertsCursor >= remaining {
						m.alertsCursor = remaining - 1
					}
					if m.alertsCursor < 0 {
						m.alertsCursor = 0
					}
					// Scroll offset may need adjusting
					if m.alertsScrollOffset > m.alertsCursor {
						m.alertsScrollOffset = m.alertsCursor
					}
					// Close panel if no alerts left
					if remaining == 0 {
						m.showAlertsPanel = false
					}
				}
				return m, nil
			case "C":
				// Clear all alerts
				for _, a := range activeAlerts {
					m.dismissedAlerts[alertKey(a)] = true
				}
				m.alertsCursor = 0
				m.alertsScrollOffset = 0
				m.showAlertsPanel = false
				return m, nil
			case "esc", "q", "!":
				m.showAlertsPanel = false
				return m, nil
			}
			return m, nil
		}

		// Handle repo picker overlay (workspace mode) before global keys (esc/q/etc.)
		if m.showRepoPicker {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m = m.handleRepoPickerKeys(msg)
			return m, nil
		}

		// Handle BQL query modal before global keys
		if m.showBQLQuery {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m, cmd = m.handleBQLQueryKeys(msg)
			return m, cmd
		}

		// Handle recipe picker overlay before global keys (esc/q/etc.)
		if m.showRecipePicker {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m = m.handleRecipePickerKeys(msg)
			return m, nil
		}

		// Handle quit confirmation first
		if m.showQuitConfirm {
			switch msg.String() {
			case "esc", "y", "Y":
				return m, tea.Quit
			default:
				m.showQuitConfirm = false
				m.focused = focusList
				return m, nil
			}
		}

		// Handle help overlay toggle (? or F1)
		if (msg.String() == "?" || msg.String() == "f1") && m.list.FilterState() != list.Filtering {
			m.showHelp = !m.showHelp
			if m.showHelp {
				m.focusBeforeHelp = m.focused // Store current focus before switching to help
				m.focused = focusHelp
				m.helpScroll = 0 // Reset scroll position when opening help
			} else {
				m.focused = m.restoreFocusFromHelp()
			}
			return m, nil
		}

		// Handle tutorial toggle (backtick `) - bv-8y31
		if msg.String() == "`" && m.list.FilterState() != list.Filtering {
			m.showTutorial = !m.showTutorial
			if m.showTutorial {
				m.showHelp = false // Close help if open
				m.tutorialModel.SetSize(m.width, m.height)
				m.focused = focusTutorial
			} else {
				m.focused = focusList
			}
			return m, nil
		}

		// Force refresh (bv-4auz): Ctrl+R / F5 triggers an immediate reload.
		if (msg.String() == "ctrl+r" || msg.String() == "f5") && m.list.FilterState() != list.Filtering {
			now := time.Now()
			if !m.lastForceRefresh.IsZero() && now.Sub(m.lastForceRefresh) < time.Second {
				return m, nil
			}
			m.lastForceRefresh = now

			m.statusMsg = "Refreshing…"
			m.statusIsError = false

			if m.backgroundWorker != nil {
				m.backgroundWorker.ForceRefresh()
				cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
				return m, tea.Batch(cmds...)
			}

			if m.beadsPath == "" && m.watcher == nil && !m.isDoltSource() {
				m.statusMsg = "Refresh unavailable"
				m.statusIsError = true
				return m, nil
			}

			// Dolt sources without background worker use async reload
			if m.isDoltSource() && m.beadsPath == "" {
				cmds = append(cmds, m.reloadFromDataSource())
				return m, tea.Batch(cmds...)
			}

			cmds = append(cmds, func() tea.Msg { return FileChangedMsg{} })
			return m, tea.Batch(cmds...)
		}

		// Handle shortcuts sidebar toggle (; or F2) - bv-3qi5
		if (msg.String() == ";" || msg.String() == "f2") && m.list.FilterState() != list.Filtering {
			m.showShortcutsSidebar = !m.showShortcutsSidebar
			if m.showShortcutsSidebar {
				m.shortcutsSidebar.ResetScroll()
				m.statusMsg = "Shortcuts sidebar: ; hide | ctrl+j/k scroll"
				m.statusIsError = false
			} else {
				m.statusMsg = ""
			}
			return m, nil
		}

		// Handle shortcuts sidebar scrolling (Ctrl+j/k when sidebar visible) - bv-3qi5
		if m.showShortcutsSidebar && m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "ctrl+j":
				m.shortcutsSidebar.ScrollDown()
				return m, nil
			case "ctrl+k":
				m.shortcutsSidebar.ScrollUp()
				return m, nil
			}
		}

		// Hybrid search toggle/preset cycle (bv-xbar.6)
		if m.focused == focusList && m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "H":
				m.statusIsError = false
				m.semanticHybridEnabled = !m.semanticHybridEnabled
				if m.semanticSearch == nil {
					m.semanticHybridEnabled = false
					m.statusMsg = "Hybrid search unavailable"
					m.statusIsError = true
					return m, nil
				}
				m.semanticSearch.SetHybridConfig(m.semanticHybridEnabled, m.semanticHybridPreset)
				m.semanticSearch.ResetCache()
				m.clearSemanticScores()
				if m.semanticHybridEnabled && !m.semanticHybridReady && !m.semanticHybridBuilding {
					m.semanticHybridBuilding = true
					m.statusMsg = "Hybrid search: computing metrics…"
					cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
				} else if m.semanticHybridEnabled {
					m.statusMsg = fmt.Sprintf("Hybrid search enabled (%s)", m.semanticHybridPreset)
				} else {
					m.statusMsg = "Semantic search: text-only"
				}
				if m.semanticSearchEnabled && m.list.FilterState() != list.Unfiltered {
					currentTerm := m.list.FilterInput.Value()
					if currentTerm != "" && !m.semanticHybridBuilding {
						cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm))
					}
				}
				m.updateListDelegate()
				return m, tea.Batch(cmds...)
			case "alt+h", "alt+H":
				m.statusIsError = false
				m.semanticHybridPreset = nextHybridPreset(m.semanticHybridPreset)
				if m.semanticSearch != nil {
					m.semanticSearch.SetHybridConfig(m.semanticHybridEnabled, m.semanticHybridPreset)
					m.semanticSearch.ResetCache()
				}
				m.clearSemanticScores()
				if m.semanticHybridEnabled {
					m.statusMsg = fmt.Sprintf("Hybrid preset: %s", m.semanticHybridPreset)
				} else {
					m.statusMsg = fmt.Sprintf("Hybrid preset set (%s)", m.semanticHybridPreset)
				}
				if m.semanticSearchEnabled && m.semanticHybridEnabled && m.list.FilterState() != list.Unfiltered {
					currentTerm := m.list.FilterInput.Value()
					if currentTerm != "" && !m.semanticHybridBuilding {
						cmds = append(cmds, ComputeSemanticFilterCmd(m.semanticSearch, currentTerm))
					}
				}
				m.updateListDelegate()
				return m, tea.Batch(cmds...)
			}
		}

		// Semantic search toggle (bv-9gf.3)
		if msg.String() == "ctrl+s" && m.focused == focusList {
			m.statusIsError = false
			m.semanticSearchEnabled = !m.semanticSearchEnabled
			if m.semanticSearchEnabled {
				if m.semanticSearch != nil {
					m.list.Filter = m.semanticSearch.Filter
					if !m.semanticSearch.Snapshot().Ready && !m.semanticIndexBuilding {
						m.semanticIndexBuilding = true
						m.statusMsg = "Semantic search: building index…"
						cmds = append(cmds, BuildSemanticIndexCmd(m.issuesForAsync()))
					} else if !m.semanticSearch.Snapshot().Ready && m.semanticIndexBuilding {
						m.statusMsg = "Semantic search: indexing…"
					} else {
						m.statusMsg = "Semantic search enabled"
					}
				} else {
					m.semanticSearchEnabled = false
					m.list.Filter = list.DefaultFilter
					m.statusMsg = "Semantic search unavailable"
					m.statusIsError = true
				}
				if m.semanticHybridEnabled && !m.semanticHybridReady && !m.semanticHybridBuilding {
					m.semanticHybridBuilding = true
					cmds = append(cmds, BuildHybridMetricsCmd(m.issuesForAsync()))
				}
			} else {
				m.list.Filter = list.DefaultFilter
				m.statusMsg = "Fuzzy search enabled"
				m.clearSemanticScores()
			}

			// Refresh the current list filter results immediately.
			prevState := m.list.FilterState()
			filterText := m.list.FilterInput.Value()
			if prevState != list.Unfiltered {
				m.list.SetFilterText(filterText)
				if prevState == list.Filtering {
					m.list.SetFilterState(list.Filtering)
				}
			}

			m.updateListDelegate()
			return m, tea.Batch(cmds...)
		}

		// If help is showing, handle navigation keys for scrolling
		if m.focused == focusHelp {
			m = m.handleHelpKeys(msg)
			return m, nil
		}

		// If tutorial is showing, route input to tutorial model (bv-8y31)
		if m.focused == focusTutorial && m.showTutorial {
			var tutorialCmd tea.Cmd
			m.tutorialModel, tutorialCmd = m.tutorialModel.Update(msg)
			// Check if tutorial wants to close
			if m.tutorialModel.ShouldClose() {
				m.showTutorial = false
				m.focused = focusList
				m.tutorialModel = NewTutorialModel(m.theme) // Reset for next time
			}
			return m, tutorialCmd
		}

		// Handle time-travel input first (before global keys intercept letters)
		// But allow ctrl+c to always quit
		if m.focused == focusTimeTravelInput {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m = m.handleTimeTravelInputKeys(msg)
			return m, nil
		}

		// Handle keys when not filtering
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit

			case "q":
				// q closes current view or quits if at top level
				if m.showDetails && !m.isSplitView {
					m.showDetails = false
					m.focused = focusList
					return m, nil
				}
				if m.focused == focusInsights {
					m.focused = focusList
					return m, nil
				}
				if m.focused == focusFlowMatrix {
					if m.flowMatrix.showDrilldown {
						m.flowMatrix.showDrilldown = false
						return m, nil
					}
					m.focused = focusList
					return m, nil
				}
				if m.isGraphView {
					m.isGraphView = false
					m.focused = focusList
					return m, nil
				}
				if m.isBoardView {
					m.isBoardView = false
					m.focused = focusList
					return m, nil
				}
				return m, tea.Quit

			case "esc":
				// Escape closes modals and goes back
				if m.showDetails && !m.isSplitView {
					m.showDetails = false
					m.focused = focusList
					return m, nil
				}
				if m.focused == focusInsights {
					m.focused = focusList
					return m, nil
				}
				if m.focused == focusFlowMatrix {
					if m.flowMatrix.showDrilldown {
						m.flowMatrix.showDrilldown = false
						return m, nil
					}
					m.focused = focusList
					return m, nil
				}
				if m.isGraphView {
					m.isGraphView = false
					m.focused = focusList
					return m, nil
				}
				if m.isBoardView {
					m.isBoardView = false
					m.focused = focusList
					return m, nil
				}
				if m.isActionableView {
					m.isActionableView = false
					m.focused = focusList
					return m, nil
				}
				if m.isHistoryView {
					m.isHistoryView = false
					m.focused = focusList
					return m, nil
				}
				// Close label picker if open (bv-126 fix)
				if m.showLabelPicker {
					m.showLabelPicker = false
					m.focused = focusList
					return m, nil
				}
				// Close label dashboard if open
				if m.focused == focusLabelDashboard {
					m.focused = focusList
					return m, nil
				}
				// At main list - first ESC clears filters, second shows quit confirm
				if m.hasActiveFilters() {
					m.clearAllFilters()
					return m, nil
				}
				// No filters active - show quit confirmation
				m.showQuitConfirm = true
				m.focused = focusQuitConfirm
				return m, nil

			case "tab":
				if m.isSplitView && !m.isBoardView {
					if m.focused == focusList {
						m.focused = focusDetail
					} else {
						m.focused = focusList
					}
				}

			case "<":
				// Shrink list pane (move divider left)
				if m.isSplitView {
					m.splitPaneRatio -= 0.05
					if m.splitPaneRatio < 0.2 {
						m.splitPaneRatio = 0.2
					}
					m.recalculateSplitPaneSizes()
				}

			case ">":
				// Expand list pane (move divider right)
				if m.isSplitView {
					m.splitPaneRatio += 0.05
					if m.splitPaneRatio > 0.8 {
						m.splitPaneRatio = 0.8
					}
					m.recalculateSplitPaneSizes()
				}

			case "b":
				m.clearAttentionOverlay()
				m.isBoardView = !m.isBoardView
				m.isGraphView = false
				m.isActionableView = false
				m.isHistoryView = false
				if m.isBoardView {
					m.focused = focusBoard
					m.refreshBoardAndGraphForCurrentFilter()
				} else {
					m.focused = focusList
				}
				return m, nil

			case "g":
				// Toggle graph view
				m.clearAttentionOverlay()
				m.isGraphView = !m.isGraphView
				m.isBoardView = false
				m.isActionableView = false
				m.isHistoryView = false
				if m.isGraphView {
					m.focused = focusGraph
					m.refreshBoardAndGraphForCurrentFilter()
				} else {
					m.focused = focusList
				}
				return m, nil

			case "a":
				// Toggle actionable view
				m.clearAttentionOverlay()
				m.isActionableView = !m.isActionableView
				m.isGraphView = false
				m.isBoardView = false
				m.isHistoryView = false
				if m.isActionableView {
					// Build execution plan
					analyzer := analysis.NewAnalyzer(m.issues)
					plan := analyzer.GetExecutionPlan()
					m.actionableView = NewActionableModel(plan, m.theme)
					m.actionableView.SetSize(m.width, m.height-2)
					m.focused = focusActionable
				} else {
					m.focused = focusList
				}
				return m, nil

			case "E":
				// Toggle hierarchical tree view (bv-gllx)
				m.clearAttentionOverlay()
				if m.focused == focusTree {
					m.focused = focusList
				} else {
					m.isGraphView = false
					m.isBoardView = false
					m.isActionableView = false
					m.isHistoryView = false
					// Build tree from snapshot when available (bv-t435)
					if m.snapshot != nil {
						m.tree.BuildFromSnapshot(m.snapshot)
					} else {
						m.tree.Build(m.issues)
					}
					m.tree.SetSize(m.width, m.height-2)
					m.focused = focusTree
				}
				return m, nil

			case "i":
				m.clearAttentionOverlay()
				if m.focused == focusInsights {
					m.focused = focusList
				} else {
					m.isGraphView = false
					m.isBoardView = false
					m.isActionableView = false
					m.isHistoryView = false
					m.focused = focusInsights
					// Refresh insights using the current snapshot when available (bv-mpqz).
					var ins analysis.Insights
					hasInsights := false
					if m.snapshot != nil {
						ins = m.snapshot.Insights
						hasInsights = true
					} else if m.analysis != nil {
						ins = m.analysis.GenerateInsights(len(m.issues))
						hasInsights = true
					}
					if hasInsights {
						m.insightsPanel = NewInsightsModel(ins, m.issueMap, m.theme)
						// Include priority triage (bv-91) - reuse existing analyzer/stats (bv-runn.12)
						triage := analysis.ComputeTriageFromAnalyzer(m.analyzer, m.analysis, m.issues, analysis.TriageOptions{}, time.Now())
						m.insightsPanel.SetTopPicks(triage.QuickRef.TopPicks)
						// Set full recommendations with breakdown for priority radar (bv-93)
						dataHash := fmt.Sprintf("v%s@%s#%d", triage.Meta.Version, triage.Meta.GeneratedAt.Format("15:04:05"), triage.Meta.IssueCount)
						m.insightsPanel.SetRecommendations(triage.Recommendations, dataHash)
						panelHeight := m.height - 2
						if panelHeight < 3 {
							panelHeight = 3
						}
						m.insightsPanel.SetSize(m.width, panelHeight)
					}
				}
				return m, nil

			case "p":
				// Toggle priority hints
				m.showPriorityHints = !m.showPriorityHints
				// Update delegate with new state
				m.updateListDelegate()
				// Show explanatory status message
				if m.showPriorityHints {
					count := len(m.priorityHints)
					if count > 0 {
						m.statusMsg = fmt.Sprintf("Priority hints: ↑ increase ↓ decrease (%d suggestions)", count)
					} else {
						m.statusMsg = "Priority hints: No misalignments detected (analysis ongoing)"
					}
				} else {
					m.statusMsg = ""
				}
				return m, nil

			case "h":
				// Toggle history view
				m.clearAttentionOverlay()
				m.isHistoryView = !m.isHistoryView
				m.isGraphView = false
				m.isBoardView = false
				m.isActionableView = false
				if m.isHistoryView {
					// Ensure history model has latest sizing
					bodyHeight := m.height - 1
					if bodyHeight < 5 {
						bodyHeight = 5
					}
					m.historyView.SetSize(m.width, bodyHeight)
					m.focused = focusHistory
				} else {
					m.focused = focusList
				}
				return m, nil

			case "[", "f3":
				// Open label dashboard (phase 1: table view)
				m.clearAttentionOverlay()
				m.isGraphView = false
				m.isBoardView = false
				m.isActionableView = false
				m.isHistoryView = false
				m.focused = focusLabelDashboard
				// Compute label health (fast; phase1 metrics only needed) with caching
				if !m.labelHealthCached {
					cfg := analysis.DefaultLabelHealthConfig()
					m.labelHealthCache = analysis.ComputeAllLabelHealth(m.issues, cfg, time.Now().UTC(), m.analysis)
					m.labelHealthCached = true
				}
				m.labelDashboard.SetData(m.labelHealthCache.Labels)
				m.labelDashboard.SetSize(m.width, m.height-1)
				m.statusMsg = fmt.Sprintf("Labels: %d total • critical %d • warning %d", m.labelHealthCache.TotalLabels, m.labelHealthCache.CriticalCount, m.labelHealthCache.WarningCount)
				m.statusIsError = false
				return m, nil

			case "]", "f4":
				// Attention view: compute attention scores (cached) and render as text
				if !m.attentionCached {
					cfg := analysis.DefaultLabelHealthConfig()
					m.attentionCache = analysis.ComputeLabelAttentionScores(m.issues, cfg, time.Now().UTC())
					m.attentionCached = true
				}
				attText, _ := ComputeAttentionView(m.issues, max(40, m.width-4))
				m.isGraphView = false
				m.isBoardView = false
				m.isActionableView = false
				m.isHistoryView = false
				m.focused = focusInsights
				m.showAttentionView = true
				m.insightsPanel = NewInsightsModel(analysis.Insights{}, m.issueMap, m.theme)
				m.insightsPanel.labelAttention = m.attentionCache.Labels
				m.insightsPanel.extraText = attText
				panelHeight := m.height - 2
				if panelHeight < 3 {
					panelHeight = 3
				}
				m.insightsPanel.SetSize(m.width, panelHeight)
				return m, nil

			case "f":
				// Flow matrix view (cross-label dependencies)
				m.clearAttentionOverlay()
				cfg := analysis.DefaultLabelHealthConfig()
				flow := analysis.ComputeCrossLabelFlow(m.issues, cfg)
				m.isGraphView = false
				m.isBoardView = false
				m.isActionableView = false
				m.isHistoryView = false
				m.focused = focusFlowMatrix
				m.flowMatrix = NewFlowMatrixModel(m.theme)
				m.flowMatrix.SetData(&flow, m.issues)
				panelHeight := m.height - 2
				if panelHeight < 3 {
					panelHeight = 3
				}
				m.flowMatrix.SetSize(m.width, panelHeight)
				return m, nil

			case "!":
				// Toggle alerts panel (bv-168)
				// Only show if there are active alerts
				activeCount := 0
				for _, a := range m.alerts {
					if !m.dismissedAlerts[alertKey(a)] {
						activeCount++
					}
				}
				if activeCount > 0 {
					m.showAlertsPanel = !m.showAlertsPanel
					m.alertsCursor = 0       // Reset cursor when opening
					m.alertsScrollOffset = 0 // Reset scroll position
				} else {
					m.statusMsg = "No active alerts"
					m.statusIsError = false
				}
				return m, nil

			case ":":
				// Open BQL query modal
				m.bqlQuery.SetSize(m.width, m.height-1)
				m.bqlQuery.Reset()
				m.showBQLQuery = true
				m.focused = focusBQLQuery
				return m, m.bqlQuery.Focus()

			case "'":
				// Toggle recipe picker overlay
				m.showRecipePicker = !m.showRecipePicker
				if m.showRecipePicker {
					m.recipePicker.SetSize(m.width, m.height-1)
					m.focused = focusRecipePicker
				} else {
					m.focused = focusList
				}
				return m, nil

			case "w":
				// Toggle repo picker overlay (workspace mode)
				if !m.workspaceMode || len(m.availableRepos) == 0 {
					m.statusMsg = "Repo filter available only in workspace mode"
					m.statusIsError = false
					return m, nil
				}
				m.showRepoPicker = !m.showRepoPicker
				if m.showRepoPicker {
					m.repoPicker = NewRepoPickerModel(m.availableRepos, m.theme)
					m.repoPicker.SetActiveRepos(m.activeRepos)
					m.repoPicker.SetSize(m.width, m.height-1)
					m.focused = focusRepoPicker
				} else {
					m.focused = focusList
				}
				return m, nil

			case "x":
				// Export to Markdown file
				m.exportToMarkdown()
				return m, nil

			case "l":
				// Open label picker for quick filter (bv-126)
				if len(m.issues) == 0 {
					return m, nil
				}
				// Update labels in case they changed
				labelExtraction := analysis.ExtractLabels(m.issues)
				labelCounts := extractLabelCounts(labelExtraction.Stats)
				m.labelPicker.SetLabels(labelExtraction.Labels, labelCounts)
				m.labelPicker.Reset()
				m.labelPicker.SetSize(m.width, m.height-1)
				m.showLabelPicker = true
				m.focused = focusLabelPicker
				return m, nil

			}

			// Focus-specific key handling
			switch m.focused {
			case focusBQLQuery:
				// BQL modal already handled in overlay dispatch above; no-op here
				return m, nil

			case focusRecipePicker:
				m = m.handleRecipePickerKeys(msg)

			case focusRepoPicker:
				m = m.handleRepoPickerKeys(msg)

			case focusLabelPicker:
				m = m.handleLabelPickerKeys(msg)

			case focusInsights:
				m = m.handleInsightsKeys(msg)

			case focusBoard:
				m = m.handleBoardKeys(msg)

			case focusLabelDashboard:
				if selectedLabel, cmd := m.labelDashboard.Update(msg); selectedLabel != "" {
					// Filter list by selected label and jump back to list view
					m.currentFilter = "label:" + selectedLabel
					m.applyFilter()
					m.focused = focusList
					return m, cmd
				}
				// Open detail modal on 'h'
				if msg.String() == "h" && len(m.labelDashboard.labels) > 0 {
					idx := m.labelDashboard.cursor
					if idx >= 0 && idx < len(m.labelDashboard.labels) {
						lh := m.labelDashboard.labels[idx]
						m.showLabelHealthDetail = true
						m.labelHealthDetail = &lh
						// Precompute cross-label flows for this label
						m.labelHealthDetailFlow = m.getCrossFlowsForLabel(lh.Label)
						return m, nil
					}
				}
				// Open drilldown overlay on 'd'
				if msg.String() == "d" && len(m.labelDashboard.labels) > 0 {
					idx := m.labelDashboard.cursor
					if idx >= 0 && idx < len(m.labelDashboard.labels) {
						lh := m.labelDashboard.labels[idx]
						m.labelDrilldownLabel = lh.Label
						m.labelDrilldownIssues = m.filterIssuesByLabel(lh.Label)
						m.showLabelDrilldown = true
						return m, nil
					}
				}

			case focusGraph:
				m = m.handleGraphKeys(msg)

			case focusTree:
				m = m.handleTreeKeys(msg)

			case focusActionable:
				m = m.handleActionableKeys(msg)

			case focusHistory:
				m = m.handleHistoryKeys(msg)

			case focusSprint:
				m = m.handleSprintKeys(msg)

			case focusFlowMatrix:
				m = m.handleFlowMatrixKeys(msg)

			case focusList:
				m = m.handleListKeys(msg)

			case focusDetail:
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.MouseMsg:
		// Intercept mouse wheel when alerts panel is open
		if m.showAlertsPanel {
			var activeAlerts []drift.Alert
			for _, a := range m.alerts {
				if !m.dismissedAlerts[alertKey(a)] {
					activeAlerts = append(activeAlerts, a)
				}
			}
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				if m.alertsCursor > 0 {
					m.alertsCursor--
					if m.alertsCursor < m.alertsScrollOffset {
						m.alertsScrollOffset = m.alertsCursor
					}
				}
			case tea.MouseButtonWheelDown:
				if m.alertsCursor < len(activeAlerts)-1 {
					m.alertsCursor++
					visLines := m.alertsVisibleLines()
					if visLines > 0 && m.alertsCursor >= m.alertsScrollOffset+visLines {
						m.alertsScrollOffset = m.alertsCursor - visLines + 1
					}
				}
			}
			return m, nil
		}

		// Handle mouse wheel scrolling
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			// Scroll up based on current focus
			switch m.focused {
			case focusList:
				if m.list.Index() > 0 {
					m.list.Select(m.list.Index() - 1)
					// Sync detail panel in split view mode
					if m.isSplitView {
						m.updateViewportContent()
					}
				}
			case focusDetail:
				m.viewport.ScrollUp(3)
			case focusInsights:
				m.insightsPanel.MoveUp()
			case focusBoard:
				m.board.MoveUp()
			case focusGraph:
				m.graphView.PageUp()
			case focusTree:
				m.tree.MoveUp()
			case focusActionable:
				m.actionableView.MoveUp()
			case focusHistory:
				m.historyView.MoveUp()
			case focusFlowMatrix:
				m.flowMatrix.MoveUp()
			}
			return m, nil
		case tea.MouseButtonWheelDown:
			// Scroll down based on current focus
			switch m.focused {
			case focusList:
				if m.list.Index() < len(m.list.Items())-1 {
					m.list.Select(m.list.Index() + 1)
					// Sync detail panel in split view mode
					if m.isSplitView {
						m.updateViewportContent()
					}
				}
			case focusDetail:
				m.viewport.ScrollDown(3)
			case focusInsights:
				m.insightsPanel.MoveDown()
			case focusBoard:
				m.board.MoveDown()
			case focusGraph:
				m.graphView.PageDown()
			case focusTree:
				m.tree.MoveDown()
			case focusActionable:
				m.actionableView.MoveDown()
			case focusHistory:
				m.historyView.MoveDown()
			case focusFlowMatrix:
				m.flowMatrix.MoveDown()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.isSplitView = msg.Width > SplitViewThreshold
		m.ready = true
		bodyHeight := m.height - 1 // keep 1 row for footer
		if bodyHeight < 5 {
			bodyHeight = 5
		}

		if m.isSplitView {
			// Calculate dimensions accounting for 2 panels with borders(2)+padding(2) = 4 overhead each
			// Total overhead = 8
			availWidth := msg.Width - 8
			if availWidth < 10 {
				availWidth = 10
			}

			// Use configurable split ratio (default 0.4, adjustable via [ and ])
			listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
			detailInnerWidth := availWidth - listInnerWidth

			// listHeight fits header (1) + page line (1) inside a panel with Border (2)
			listHeight := bodyHeight - 4
			if listHeight < 3 {
				listHeight = 3
			}

			m.list.SetSize(listInnerWidth, listHeight)
			m.viewport = viewport.New(detailInnerWidth, bodyHeight-2) // Account for border

			m.renderer.SetWidthWithTheme(detailInnerWidth, m.theme)
		} else {
			listHeight := bodyHeight - 2
			if listHeight < 3 {
				listHeight = 3
			}
			m.list.SetSize(msg.Width, listHeight)
			m.viewport = viewport.New(msg.Width, bodyHeight-1)

			// Update renderer for full width
			m.renderer.SetWidthWithTheme(msg.Width, m.theme)
		}

		m.updateListDelegate()

		// Resize label dashboard table and modal overlay sizing
		m.labelDashboard.SetSize(m.width, bodyHeight)

		m.insightsPanel.SetSize(m.width, bodyHeight)
		m.updateViewportContent()
	}

	// Update list for navigation, but NOT for WindowSizeMsg
	// (we handle sizing ourselves to account for header/footer)
	// Only forward keyboard messages to list when list has focus (bv-hmkz fix)
	// This prevents j/k keys in detail view from changing list selection
	if m.focused == focusList {
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
	if m.backgroundWorker != nil {
		m.backgroundWorker.Stop()
	}
	if m.watcher != nil {
		m.watcher.Stop()
	}
	if m.instanceLock != nil {
		m.instanceLock.Release()
	}
	if len(m.pooledIssues) > 0 {
		loader.ReturnIssuePtrsToPool(m.pooledIssues)
		m.pooledIssues = nil
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
	if m.backgroundWorker != nil && reconnectFn != nil {
		m.backgroundWorker.SetDoltReconnectFn(reconnectFn)
	}
}

// DoltShutdownMsg returns a message to print after the TUI exits,
// indicating whether bt stopped its Dolt server (bt-llek).
func (m *Model) DoltShutdownMsg() string {
	return m.doltShutdownMsg
}

// clearAttentionOverlay hides the attention overlay and clears its rendered text.
func (m *Model) clearAttentionOverlay() {
	if m.showAttentionView {
		m.showAttentionView = false
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
