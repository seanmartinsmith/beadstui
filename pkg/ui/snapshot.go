// Package ui provides the terminal user interface for beads_viewer.
// This file implements the DataSnapshot type for thread-safe UI rendering.
package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/recipe"
)

type datasetTier int

const (
	datasetTierUnknown datasetTier = iota
	datasetTierSmall
	datasetTierMedium
	datasetTierLarge
	datasetTierHuge
)

func datasetTierForIssueCount(total int) datasetTier {
	switch {
	case total <= 0:
		return datasetTierUnknown
	case total < 1000:
		return datasetTierSmall
	case total < 5000:
		return datasetTierMedium
	case total < 20000:
		return datasetTierLarge
	default:
		return datasetTierHuge
	}
}

func (t datasetTier) String() string {
	switch t {
	case datasetTierSmall:
		return "small"
	case datasetTierMedium:
		return "medium"
	case datasetTierLarge:
		return "large"
	case datasetTierHuge:
		return "huge"
	default:
		return "unknown"
	}
}

func isClosedLikeStatus(status model.Status) bool {
	return status == model.StatusClosed || status == model.StatusTombstone
}

type snapshotBuildConfig struct {
	PrecomputeTriage      bool
	PrecomputeTree        bool
	PrecomputeBoard       bool
	PrecomputeGraphLayout bool
	PrecomputeInsights    bool
	SkipPhase2            bool
}

func snapshotBuildConfigDefault() snapshotBuildConfig {
	return snapshotBuildConfig{
		PrecomputeTriage:      true,
		PrecomputeTree:        true,
		PrecomputeBoard:       true,
		PrecomputeGraphLayout: true,
		PrecomputeInsights:    true,
		SkipPhase2:            false,
	}
}

func snapshotBuildConfigForTier(tier datasetTier) snapshotBuildConfig {
	cfg := snapshotBuildConfigDefault()
	switch tier {
	case datasetTierLarge:
		cfg.PrecomputeTriage = false
		cfg.PrecomputeTree = false
		cfg.PrecomputeBoard = false
		cfg.PrecomputeGraphLayout = false
		cfg.PrecomputeInsights = false
	case datasetTierHuge:
		cfg.PrecomputeTriage = false
		cfg.PrecomputeTree = false
		cfg.PrecomputeBoard = false
		cfg.PrecomputeGraphLayout = false
		cfg.PrecomputeInsights = false
		cfg.SkipPhase2 = true
	}
	return cfg
}

func compactCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%dm", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

const incrementalListMaxChangeRatio = 0.2

// IssueDiffStats summarizes the change volume between snapshots.
type IssueDiffStats struct {
	Changed int
	Total   int
	Ratio   float64
}

// DataSnapshot is an immutable, self-contained representation of all data
// the UI needs to render. Once created, it never changes - this is critical
// for thread safety when the background worker is building the next snapshot.
//
// The UI thread reads exclusively from its current snapshot pointer.
// When a new snapshot is ready, the UI swaps the pointer atomically.
type DataSnapshot struct {
	// Core data
	Issues   []model.Issue           // All issues (sorted)
	IssueMap map[string]*model.Issue // Lookup by ID
	// pooledIssues holds pooled backing structs used during parse.
	// It must be returned to the pool when the snapshot is replaced.
	pooledIssues []*model.Issue
	// ViewIssues are the issues included in the current view context (e.g. recipe).
	// When empty, callers should fall back to Issues.
	ViewIssues []model.Issue

	// Graph analysis
	Analyzer *analysis.Analyzer
	Analysis *analysis.GraphStats
	Insights analysis.Insights

	// Computed statistics
	CountOpen    int
	CountReady   int
	CountBlocked int
	CountClosed  int

	// Pre-computed UI data (Phase 3 will populate these)
	// For now, they're nil and the UI computes on demand
	ListItems     []IssueItem // Pre-built list items with scores
	TriageScores  map[string]float64
	TriageReasons map[string]analysis.TriageReasons
	QuickWinSet   map[string]bool
	BlockerSet    map[string]bool
	UnblocksMap   map[string][]string
	// TreeRoots and TreeNodeMap contain a pre-built parent/child tree for the Tree view.
	// These are computed off-thread by SnapshotBuilder to avoid UI-thread work when
	// entering the tree view for large datasets.
	TreeRoots   []*IssueTreeNode
	TreeNodeMap map[string]*IssueTreeNode
	// BoardState contains pre-built Kanban board columns for each swimlane mode (bv-guxz).
	BoardState *BoardState
	// GraphLayout contains pre-built graph view data (blockers/dependents, sorted IDs, ranks)
	// to avoid rebuilding graph structures on the UI thread (bv-za8z).
	GraphLayout *GraphLayout

	// Metadata
	CreatedAt  time.Time // When this snapshot was built
	DataHash   string    // Hash of source data for cache validation
	RecipeName string    // Active recipe name for this snapshot (bv-2h40)
	RecipeHash string    // Fingerprint of active recipe for this snapshot (bv-4ilb)
	// DatasetTier is a tiered performance mode for large datasets (bv-9thm).
	// When unknown, normal behavior applies.
	DatasetTier datasetTier
	// SourceIssueCountHint is an approximate total issue count from the source file
	// (e.g., JSONL line count). This may be 0 if unavailable.
	SourceIssueCountHint int
	// LoadedOpenOnly indicates the snapshot intentionally excluded closed/tombstone
	// issues for performance (huge tier).
	LoadedOpenOnly bool
	// TruncatedCount is an approximate count of issues excluded by load policy.
	// This may include invalid/empty lines when computed from a line count hint.
	TruncatedCount int
	// LargeDatasetWarning is a short, user-facing warning to show in the footer.
	LargeDatasetWarning string
	// LoadWarningCount is the number of non-fatal parse warnings encountered while loading.
	// In TUI mode, warnings must not be printed to stderr during render.
	LoadWarningCount int

	// Phase 2 analysis status
	// Phase2Ready is true when expensive metrics (PageRank, Betweenness, etc.) are computed
	// UI can render immediately with Phase 1 data, then refresh when Phase 2 completes
	Phase2Ready bool

	// Incremental update metadata (bv-5mzz).
	IssueDiff      *analysis.IssueDiff
	IssueDiffStats IssueDiffStats
	// IncrementalListUsed reports whether list items were rebuilt incrementally.
	IncrementalListUsed bool

	// Error state (for graceful degradation)
	LoadError    error     // Non-nil if last load had recoverable errors
	ErrorTime    time.Time // When error occurred
	StaleWarning bool      // True if data is from previous successful load
}

// GraphLayout contains precomputed data used by the graph view.
// This intentionally focuses on the current ASCII graph view needs (relationships + ranks),
// not geometric coordinates.
type GraphLayout struct {
	// Relationships (blocks/dependents)
	Blockers   map[string][]string // What each issue depends on (blocks this issue)
	Dependents map[string][]string // What depends on each issue (this issue blocks)

	// Navigation order (all IDs in the snapshot)
	SortedIDs []string

	// Metric ranks (1 = best, higher = worse). Missing ranks imply "unknown".
	RankPageRank     map[string]int
	RankBetweenness  map[string]int
	RankEigenvector  map[string]int
	RankHubs         map[string]int
	RankAuthorities  map[string]int
	RankCriticalPath map[string]int
	RankInDegree     map[string]int
	RankOutDegree    map[string]int
}

// BoardState contains precomputed Kanban columns for each swimlane mode.
// This lets the UI swap board data in O(1) when the full dataset is shown.
type BoardState struct {
	ByStatus   [4][]model.Issue
	ByPriority [4][]model.Issue
	ByType     [4][]model.Issue
}

func (s *BoardState) ColumnsForMode(mode SwimLaneMode) [4][]model.Issue {
	if s == nil {
		return [4][]model.Issue{}
	}
	switch mode {
	case SwimByPriority:
		return s.ByPriority
	case SwimByType:
		return s.ByType
	default:
		return s.ByStatus
	}
}

func (l *GraphLayout) UpdatePhase2Ranks(stats *analysis.GraphStats) {
	if l == nil || stats == nil {
		return
	}

	// Phase 2 ranks may become available later (AnalyzeAsync).
	l.RankPageRank = stats.PageRankRank()
	l.RankBetweenness = stats.BetweennessRank()
	l.RankEigenvector = stats.EigenvectorRank()
	l.RankHubs = stats.HubsRank()
	l.RankAuthorities = stats.AuthoritiesRank()
	l.RankCriticalPath = stats.CriticalPathRank()

	// Rebuild SortedIDs using the new critical-path ranking, preserving determinism.
	l.SortedIDs = orderIssueIDsByRank(l.SortedIDs, l.RankCriticalPath)
}

// SnapshotBuilder constructs DataSnapshots from raw data.
// This is used by the BackgroundWorker to build new snapshots.
type SnapshotBuilder struct {
	issues   []model.Issue
	analyzer *analysis.Analyzer
	analysis *analysis.GraphStats
	recipe   *recipe.Recipe
	cfg      snapshotBuildConfig

	prevSnapshot *DataSnapshot
	diff         *analysis.IssueDiff
	diffStats    IssueDiffStats
}

// NewSnapshotBuilder creates a builder for constructing a DataSnapshot.
func NewSnapshotBuilder(issues []model.Issue) *SnapshotBuilder {
	return &SnapshotBuilder{
		issues:   issues,
		analyzer: analysis.NewAnalyzer(issues),
		cfg:      snapshotBuildConfigDefault(),
	}
}

// WithAnalysis sets the pre-computed analysis (for when we have cached results).
func (b *SnapshotBuilder) WithAnalysis(a *analysis.GraphStats) *SnapshotBuilder {
	b.analysis = a
	return b
}

func (b *SnapshotBuilder) WithRecipe(r *recipe.Recipe) *SnapshotBuilder {
	b.recipe = r
	return b
}

func (b *SnapshotBuilder) WithBuildConfig(cfg snapshotBuildConfig) *SnapshotBuilder {
	b.cfg = cfg
	return b
}

// WithPreviousSnapshot enables incremental list-item rebuilds when possible.
func (b *SnapshotBuilder) WithPreviousSnapshot(prev *DataSnapshot, diff *analysis.IssueDiff) *SnapshotBuilder {
	b.prevSnapshot = prev
	b.diff = diff
	b.diffStats = issueDiffStats(diff)
	return b
}

// Build constructs the final immutable DataSnapshot.
// This performs all necessary computations that should happen in the background.
// Uses AnalyzeAsync() so Phase 2 metrics compute in background - check Phase2Ready
// or call GetGraphStats().WaitForPhase2() if you need Phase 2 data immediately.
func (b *SnapshotBuilder) Build() *DataSnapshot {
	issues := b.issues

	// Apply default sorting to match the legacy reload path:
	// Open first, then priority (ascending), then created date (newest first).
	sort.Slice(issues, func(i, j int) bool {
		iClosed := isClosedLikeStatus(issues[i].Status)
		jClosed := isClosedLikeStatus(issues[j].Status)
		if iClosed != jClosed {
			return !iClosed
		}
		if issues[i].Priority != issues[j].Priority {
			return issues[i].Priority < issues[j].Priority
		}
		return issues[i].CreatedAt.After(issues[j].CreatedAt)
	})

	// Compute analysis if not provided
	// Use AnalyzeAsync to allow Phase 2 to run in background
	graphStats := b.analysis
	if graphStats == nil {
		if b.cfg.SkipPhase2 {
			// Still compute Phase 1 metrics, but skip expensive Phase 2 work.
			cfg := analysis.ConfigForSize(len(issues), 0)
			cfg.ComputePageRank = false
			cfg.ComputeBetweenness = false
			cfg.ComputeEigenvector = false
			cfg.ComputeHITS = false
			cfg.ComputeCriticalPath = false
			cfg.ComputeCycles = false
			graphStats = b.analyzer.AnalyzeAsyncWithConfig(context.Background(), cfg)
		} else {
			graphStats = b.analyzer.AnalyzeAsync(context.Background())
		}
	}

	// Build lookup map
	issueMap := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]
	}

	// Compute statistics
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

	viewIssues := issues
	if b.recipe != nil {
		viewIssues = make([]model.Issue, 0, len(issues))
		for i := range issues {
			if issueMatchesRecipe(issues[i], issueMap, b.recipe) {
				viewIssues = append(viewIssues, issues[i])
			}
		}
		sortIssuesByRecipe(viewIssues, graphStats, b.recipe)
	}

	// Build list items with graph scores (respecting recipe filtering/sorting when present).
	listItemsIncremental := false
	statsForListItems := graphStats
	// If analysis was computed asynchronously for this snapshot build, treat Phase 2 scores
	// as not-yet-available to keep list items deterministic (they will be refreshed when
	// Phase 2 completes via Phase2ReadyMsg).
	if b.analysis == nil {
		statsForListItems = nil
	}
	listItems := buildListItems(viewIssues, statsForListItems)
	if shouldUseIncrementalList(b.prevSnapshot, b.diff, b.recipe, b.diffStats) {
		listItems = buildListItemsIncremental(viewIssues, statsForListItems, b.prevSnapshot.ListItems, b.diff)
		listItemsIncremental = true
	}

	var (
		triageScores  map[string]float64
		triageReasons map[string]analysis.TriageReasons
		quickWinSet   map[string]bool
		blockerSet    map[string]bool
		unblocksMap   map[string][]string
	)

	// Compute triage insights (may be skipped for large/huge datasets; bv-9thm).
	if b.cfg.PrecomputeTriage {
		triageResult := analysis.ComputeTriageFromAnalyzer(b.analyzer, graphStats, issues, analysis.TriageOptions{}, time.Now())
		triageScores = make(map[string]float64, len(triageResult.Recommendations))
		triageReasons = make(map[string]analysis.TriageReasons, len(triageResult.Recommendations))
		quickWinSet = make(map[string]bool, len(triageResult.QuickWins))
		blockerSet = make(map[string]bool, len(triageResult.BlockersToClear))
		unblocksMap = make(map[string][]string, len(triageResult.Recommendations))

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

		// Update list items with triage data
		for i := range listItems {
			id := listItems[i].Issue.ID
			listItems[i].TriageScore = triageScores[id]
			if reasons, exists := triageReasons[id]; exists {
				listItems[i].TriageReason = reasons.Primary
				listItems[i].TriageReasons = reasons.All
			}
			listItems[i].IsQuickWin = quickWinSet[id]
			listItems[i].IsBlocker = blockerSet[id]
			listItems[i].UnblocksCount = len(unblocksMap[id])
		}
	}

	var (
		treeRoots   []*IssueTreeNode
		treeNodeMap map[string]*IssueTreeNode
	)
	if b.cfg.PrecomputeTree {
		treeRoots, treeNodeMap = buildIssueTreeNodes(issues)
	}

	var boardState *BoardState
	if b.cfg.PrecomputeBoard {
		boardState = buildBoardState(issues)
	}

	insights := analysis.Insights{Stats: graphStats, ClusterDensity: graphStats.Density}
	if b.cfg.PrecomputeInsights {
		insights = graphStats.GenerateInsights(len(issues))
	}

	var graphLayout *GraphLayout
	if b.cfg.PrecomputeGraphLayout {
		graphLayout = buildGraphLayout(issues, graphStats)
	}

	return &DataSnapshot{
		Issues:        issues,
		IssueMap:      issueMap,
		ViewIssues:    viewIssues,
		Analyzer:      b.analyzer,
		Analysis:      graphStats,
		Insights:      insights,
		CountOpen:     cOpen,
		CountReady:    cReady,
		CountBlocked:  cBlocked,
		CountClosed:   cClosed,
		ListItems:     listItems,
		TriageScores:  triageScores,
		TriageReasons: triageReasons,
		QuickWinSet:   quickWinSet,
		BlockerSet:    blockerSet,
		UnblocksMap:   unblocksMap,
		TreeRoots:     treeRoots,
		TreeNodeMap:   treeNodeMap,
		BoardState:    boardState,
		GraphLayout:   graphLayout,
		CreatedAt:     time.Now(),
		Phase2Ready:   graphStats.IsPhase2Ready(),
		IssueDiff:     b.diff,
		IssueDiffStats: IssueDiffStats{
			Changed: b.diffStats.Changed,
			Total:   b.diffStats.Total,
			Ratio:   b.diffStats.Ratio,
		},
		IncrementalListUsed: listItemsIncremental,
	}
}

func issueDiffStats(diff *analysis.IssueDiff) IssueDiffStats {
	if diff == nil {
		return IssueDiffStats{}
	}
	changed := len(diff.Added) + len(diff.Removed) + len(diff.Modified)
	total := changed + len(diff.Unchanged)
	ratio := 0.0
	if total > 0 {
		ratio = float64(changed) / float64(total)
	}
	return IssueDiffStats{
		Changed: changed,
		Total:   total,
		Ratio:   ratio,
	}
}

func shouldUseIncrementalList(prev *DataSnapshot, diff *analysis.IssueDiff, r *recipe.Recipe, stats IssueDiffStats) bool {
	if prev == nil || diff == nil || len(prev.ListItems) == 0 {
		return false
	}

	currentRecipeName := ""
	currentRecipeHash := ""
	if r != nil {
		currentRecipeName = r.Name
		currentRecipeHash = recipeFingerprint(r)
	}

	if prev.RecipeName != currentRecipeName || prev.RecipeHash != currentRecipeHash {
		return false
	}
	if stats.Total == 0 {
		return false
	}
	return stats.Ratio <= incrementalListMaxChangeRatio
}

func buildListItems(issues []model.Issue, stats *analysis.GraphStats) []IssueItem {
	listItems := make([]IssueItem, len(issues))
	for i := range issues {
		listItems[i] = buildIssueItemForSnapshot(issues[i], stats)
	}
	return listItems
}

func buildListItemsIncremental(issues []model.Issue, stats *analysis.GraphStats, prevItems []IssueItem, diff *analysis.IssueDiff) []IssueItem {
	if len(prevItems) == 0 || diff == nil {
		return buildListItems(issues, stats)
	}
	prevByID := make(map[string]IssueItem, len(prevItems))
	for _, item := range prevItems {
		prevByID[item.Issue.ID] = item
	}
	changed := make(map[string]struct{}, len(diff.Added)+len(diff.Modified))
	for _, id := range diff.Added {
		changed[id] = struct{}{}
	}
	for _, id := range diff.Modified {
		changed[id] = struct{}{}
	}

	listItems := make([]IssueItem, len(issues))
	for i := range issues {
		issue := issues[i]
		item, ok := prevByID[issue.ID]
		if !ok || isChangedID(changed, issue.ID) {
			item = IssueItem{}
		}
		resetIssueItemForSnapshot(&item, issue, stats)
		listItems[i] = item
	}
	return listItems
}

func buildIssueItemForSnapshot(issue model.Issue, stats *analysis.GraphStats) IssueItem {
	item := IssueItem{}
	resetIssueItemForSnapshot(&item, issue, stats)
	return item
}

func resetIssueItemForSnapshot(item *IssueItem, issue model.Issue, stats *analysis.GraphStats) {
	item.Issue = issue
	if stats != nil {
		item.GraphScore = stats.GetPageRankScore(issue.ID)
		item.Impact = stats.GetCriticalPathScore(issue.ID)
	} else {
		item.GraphScore = 0
		item.Impact = 0
	}
	item.RepoPrefix = ExtractRepoPrefix(issue.ID)
	item.DiffStatus = DiffStatusNone

	item.SearchScore = 0
	item.SearchTextScore = 0
	item.SearchComponents = nil
	item.SearchScoreSet = false

	item.TriageScore = 0
	item.TriageReason = ""
	item.TriageReasons = nil
	item.IsQuickWin = false
	item.IsBlocker = false
	item.UnblocksCount = 0
}

func isChangedID(changed map[string]struct{}, id string) bool {
	_, ok := changed[id]
	return ok
}

func issueMatchesRecipe(issue model.Issue, issueMap map[string]*model.Issue, r *recipe.Recipe) bool {
	if r == nil {
		return true
	}

	// Status filter
	if len(r.Filters.Status) > 0 {
		statusMatch := false
		for _, s := range r.Filters.Status {
			if matchesRecipeStatus(issue.Status, s) {
				statusMatch = true
				break
			}
		}
		if !statusMatch {
			return false
		}
	}

	// Priority filter
	if len(r.Filters.Priority) > 0 {
		prioMatch := false
		for _, p := range r.Filters.Priority {
			if issue.Priority == p {
				prioMatch = true
				break
			}
		}
		if !prioMatch {
			return false
		}
	}

	// Tags filter (must have ALL specified tags)
	if len(r.Filters.Tags) > 0 {
		for _, required := range r.Filters.Tags {
			found := false
			for _, label := range issue.Labels {
				if label == required {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Actionable filter (true = no open blockers)
	if r.Filters.Actionable != nil && *r.Filters.Actionable {
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				return false
			}
		}
	}

	return true
}

func sortIssuesByRecipe(issues []model.Issue, stats *analysis.GraphStats, r *recipe.Recipe) {
	if r == nil || r.Sort.Field == "" {
		return
	}

	desc := r.Sort.Direction == "desc"
	field := r.Sort.Field

	sort.Slice(issues, func(i, j int) bool {
		ii := issues[i]
		jj := issues[j]

		var cmp int
		switch field {
		case "priority":
			switch {
			case ii.Priority < jj.Priority:
				cmp = -1
			case ii.Priority > jj.Priority:
				cmp = 1
			}
		case "created", "created_at":
			switch {
			case ii.CreatedAt.Before(jj.CreatedAt):
				cmp = -1
			case ii.CreatedAt.After(jj.CreatedAt):
				cmp = 1
			}
		case "updated", "updated_at":
			switch {
			case ii.UpdatedAt.Before(jj.UpdatedAt):
				cmp = -1
			case ii.UpdatedAt.After(jj.UpdatedAt):
				cmp = 1
			}
		case "impact":
			if stats == nil {
				switch {
				case ii.Priority < jj.Priority:
					cmp = -1
				case ii.Priority > jj.Priority:
					cmp = 1
				}
				break
			}
			iScore := stats.GetCriticalPathScore(ii.ID)
			jScore := stats.GetCriticalPathScore(jj.ID)
			switch {
			case iScore < jScore:
				cmp = -1
			case iScore > jScore:
				cmp = 1
			}
		case "pagerank":
			if stats == nil {
				switch {
				case ii.Priority < jj.Priority:
					cmp = -1
				case ii.Priority > jj.Priority:
					cmp = 1
				}
				break
			}
			iScore := stats.GetPageRankScore(ii.ID)
			jScore := stats.GetPageRankScore(jj.ID)
			switch {
			case iScore < jScore:
				cmp = -1
			case iScore > jScore:
				cmp = 1
			}
		default:
			switch {
			case ii.Priority < jj.Priority:
				cmp = -1
			case ii.Priority > jj.Priority:
				cmp = 1
			}
		}

		// Tie-breaker for determinism.
		if cmp == 0 {
			return ii.ID < jj.ID
		}

		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func buildGraphLayout(issues []model.Issue, stats *analysis.GraphStats) *GraphLayout {
	size := len(issues)
	ids := make([]string, 0, size)
	blockers := make(map[string][]string, size)
	dependents := make(map[string][]string, size)

	for i := range issues {
		issue := &issues[i]
		ids = append(ids, issue.ID)

		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			blockers[issue.ID] = append(blockers[issue.ID], dep.DependsOnID)
			dependents[dep.DependsOnID] = append(dependents[dep.DependsOnID], issue.ID)
		}
	}

	layout := &GraphLayout{
		Blockers:   blockers,
		Dependents: dependents,
	}

	if stats != nil {
		layout.RankInDegree = stats.InDegreeRank()
		layout.RankOutDegree = stats.OutDegreeRank()
		layout.RankPageRank = stats.PageRankRank()
		layout.RankBetweenness = stats.BetweennessRank()
		layout.RankEigenvector = stats.EigenvectorRank()
		layout.RankHubs = stats.HubsRank()
		layout.RankAuthorities = stats.AuthoritiesRank()
		layout.RankCriticalPath = stats.CriticalPathRank()
	}

	layout.SortedIDs = orderIssueIDsByRank(ids, layout.RankCriticalPath)
	return layout
}

func buildBoardState(issues []model.Issue) *BoardState {
	if len(issues) == 0 {
		return nil
	}
	return &BoardState{
		ByStatus:   groupIssuesByMode(issues, SwimByStatus),
		ByPriority: groupIssuesByMode(issues, SwimByPriority),
		ByType:     groupIssuesByMode(issues, SwimByType),
	}
}

func orderIssueIDsByRank(ids []string, ranks map[string]int) []string {
	if len(ids) == 0 {
		return nil
	}

	// If we have a rank map, rebuild in O(n) without sorting (ranks are 1..N).
	if len(ranks) > 0 {
		ordered := make([]string, len(ids))
		var missing []string

		for _, id := range ids {
			rank := ranks[id]
			if rank < 1 || rank > len(ordered) || ordered[rank-1] != "" {
				missing = append(missing, id)
				continue
			}
			ordered[rank-1] = id
		}

		// Compact any gaps then append missing IDs deterministically.
		out := make([]string, 0, len(ids))
		for _, id := range ordered {
			if id != "" {
				out = append(out, id)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			out = append(out, missing...)
		}
		if len(out) > 0 {
			return out
		}
	}

	// Fallback: stable alphabetical ordering.
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return sorted
}

// GetGraphStats returns the GraphStats pointer for Phase 2 waiting.
// Callers can use stats.WaitForPhase2() to block until Phase 2 completes.
func (s *DataSnapshot) GetGraphStats() *analysis.GraphStats {
	if s == nil {
		return nil
	}
	return s.Analysis
}

// IsEmpty returns true if the snapshot has no issues.
func (s *DataSnapshot) IsEmpty() bool {
	return s == nil || len(s.Issues) == 0
}

// GetIssue returns an issue by ID, or nil if not found.
func (s *DataSnapshot) GetIssue(id string) *model.Issue {
	if s == nil || s.IssueMap == nil {
		return nil
	}
	return s.IssueMap[id]
}

// Age returns how long ago this snapshot was created.
func (s *DataSnapshot) Age() time.Duration {
	if s == nil {
		return 0
	}
	return time.Since(s.CreatedAt)
}
