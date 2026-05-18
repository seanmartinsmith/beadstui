package ui

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/search"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type semanticSearchSnapshot struct {
	Ready    bool
	Index    *search.VectorIndex
	Embedder search.Embedder
	IDs      []string
	Docs     map[string]string
	// Version increments whenever IDs content changes. Cached ranks and
	// in-flight async results are tagged with the version they were computed
	// against; mismatch ⇒ stale, drop and recompute.
	Version uint64
}

// cachedRanks pairs a result set with the snapshot version it was computed
// against. Returning ranks from a different version risks panics in
// list.filterItems where items[r.Index] reads past the current targets length.
type cachedRanks struct {
	ranks   []list.Rank
	version uint64
}

// semanticResultCache holds cached filter results and pending state. Capacity
// is bounded to semanticCacheCap entries via FIFO eviction (not LRU — the
// working set is the term currently in the search box, so eviction order
// barely matters; FIFO is simpler than maintaining access timestamps).
type semanticResultCache struct {
	results     map[string]cachedRanks // term -> ranks
	order       []string               // insertion order for FIFO eviction
	pendingTerm string                 // term awaiting async computation
	lastQuery   time.Time              // for debounce
}

const semanticCacheCap = 10

// HybridScoreFloor is the minimum final score an item must have to appear in
// semantic or hybrid search results. Items below this threshold are dropped
// from ComputeSemanticResults output — they are noise at this score level.
// The value (0.05) reuses the former SearchScoreBadgeMinAbs threshold removed
// in bt-r3zxj: the project already decided sub-0.05 hybrid scores were noise
// (they were the gate for showing a score badge before badges were removed).
const HybridScoreFloor = 0.05

type semanticHybridConfig struct {
	Enabled bool
	Preset  search.PresetName
	Weights search.Weights
}

type semanticScoreCache struct {
	term   string
	scores map[string]SemanticScore
}

type metricsCacheHolder struct {
	cache search.MetricsCache
}

// SemanticScore captures semantic/hybrid scoring details for a single issue.
type SemanticScore struct {
	Score      float64
	TextScore  float64
	Components map[string]float64
}

type SemanticSearch struct {
	// snapshotMu serializes read-modify-write on snapshot. atomic.Value is
	// atomic per load/store but does not compose into atomic RMW: two
	// goroutines doing Snapshot()→mutate→Store() can lose updates. Readers
	// stay on atomic.Load (no contention on the hot Filter path); writers
	// (SetIndex, SetIDs, SetDocs) take the mutex around the RMW sequence.
	snapshotMu   sync.Mutex
	snapshot     atomic.Value // semanticSearchSnapshot
	cache        atomic.Value // *semanticResultCache
	scores       atomic.Value // *semanticScoreCache
	hybridConfig atomic.Value // semanticHybridConfig
	metricsCache atomic.Value // *metricsCacheHolder
}

func NewSemanticSearch() *SemanticSearch {
	s := &SemanticSearch{}
	s.snapshot.Store(semanticSearchSnapshot{})
	s.cache.Store(&semanticResultCache{results: make(map[string]cachedRanks)})
	s.scores.Store(&semanticScoreCache{scores: make(map[string]SemanticScore)})
	s.metricsCache.Store(&metricsCacheHolder{})
	defaultWeights, err := search.GetPreset(search.PresetDefault)
	if err != nil {
		defaultWeights = search.Weights{TextRelevance: 1.0}
	}
	s.hybridConfig.Store(semanticHybridConfig{
		Enabled: false,
		Preset:  search.PresetDefault,
		Weights: defaultWeights.Normalize(),
	})
	return s
}

func (s *SemanticSearch) getCache() *semanticResultCache {
	v := s.cache.Load()
	if v == nil {
		return &semanticResultCache{results: make(map[string]cachedRanks)}
	}
	return v.(*semanticResultCache)
}

// GetPendingTerm returns the term awaiting async semantic computation, if any
func (s *SemanticSearch) GetPendingTerm() string {
	return s.getCache().pendingTerm
}

// GetLastQueryTime returns when the last filter query was made (for debouncing)
func (s *SemanticSearch) GetLastQueryTime() time.Time {
	return s.getCache().lastQuery
}

func (s *SemanticSearch) getScores() *semanticScoreCache {
	v := s.scores.Load()
	if v == nil {
		return &semanticScoreCache{scores: make(map[string]SemanticScore)}
	}
	return v.(*semanticScoreCache)
}

// SetScores stores the latest scores for a given term.
func (s *SemanticSearch) SetScores(term string, scores map[string]SemanticScore) {
	if scores == nil {
		s.scores.Store(&semanticScoreCache{term: term, scores: make(map[string]SemanticScore)})
		return
	}
	s.scores.Store(&semanticScoreCache{term: term, scores: scores})
}

// Scores returns scores for a specific term if available.
func (s *SemanticSearch) Scores(term string) (map[string]SemanticScore, bool) {
	cache := s.getScores()
	if cache.term != term || cache.scores == nil {
		return nil, false
	}
	return cache.scores, true
}

// ClearScores clears cached scores.
func (s *SemanticSearch) ClearScores() {
	s.scores.Store(&semanticScoreCache{scores: make(map[string]SemanticScore)})
}

func (s *SemanticSearch) getHybridConfig() semanticHybridConfig {
	v := s.hybridConfig.Load()
	if v == nil {
		return semanticHybridConfig{Enabled: false, Preset: search.PresetDefault, Weights: search.Weights{TextRelevance: 1.0}}
	}
	return v.(semanticHybridConfig)
}

// SetHybridConfig updates hybrid scoring configuration.
func (s *SemanticSearch) SetHybridConfig(enabled bool, preset search.PresetName) {
	weights, err := search.GetPreset(preset)
	if err != nil {
		weights, _ = search.GetPreset(search.PresetDefault)
		preset = search.PresetDefault
	}
	s.hybridConfig.Store(semanticHybridConfig{
		Enabled: enabled,
		Preset:  preset,
		Weights: weights.Normalize(),
	})
}

func (s *SemanticSearch) getMetricsCache() search.MetricsCache {
	v := s.metricsCache.Load()
	if v == nil {
		return nil
	}
	holder := v.(*metricsCacheHolder)
	return holder.cache
}

// SetMetricsCache sets the metrics cache used for hybrid scoring.
func (s *SemanticSearch) SetMetricsCache(cache search.MetricsCache) {
	s.metricsCache.Store(&metricsCacheHolder{cache: cache})
}

// ResetCache clears cached semantic results and scores.
func (s *SemanticSearch) ResetCache() {
	s.cache.Store(&semanticResultCache{results: make(map[string]cachedRanks)})
	s.ClearScores()
}

// SetCachedResults stores semantic filter results and clears pending state if
// matching. Results computed against a stale snapshot (version mismatch) are
// rejected — Index values would be out of range against the current targets.
func (s *SemanticSearch) SetCachedResults(term string, results []list.Rank, version uint64) {
	currentVersion := s.Snapshot().Version
	if version != currentVersion {
		// In-flight result from a previous snapshot. Drop it and clear pending
		// if it matched, so the caller can re-trigger compute against the
		// current snapshot.
		c := s.getCache()
		if c.pendingTerm == term {
			newCache := &semanticResultCache{
				results:     c.results,
				order:       c.order,
				pendingTerm: "",
				lastQuery:   c.lastQuery,
			}
			s.cache.Store(newCache)
		}
		return
	}

	c := s.getCache()

	// Only clear pending if this is the term that was pending
	// Otherwise preserve the current pending term (user may have typed a new query)
	newPendingTerm := c.pendingTerm
	if c.pendingTerm == term {
		newPendingTerm = ""
	}

	newCache := &semanticResultCache{
		results:     make(map[string]cachedRanks, len(c.results)+1),
		order:       make([]string, 0, len(c.order)+1),
		pendingTerm: newPendingTerm,
		lastQuery:   c.lastQuery,
	}
	// Replay existing entries in their insertion order, skipping the term
	// being updated (re-inserted below at the tail) and any orphans whose
	// version no longer matches the current snapshot.
	for _, k := range c.order {
		if k == term {
			continue
		}
		v, ok := c.results[k]
		if !ok || v.version != currentVersion {
			continue
		}
		newCache.results[k] = v
		newCache.order = append(newCache.order, k)
	}
	newCache.results[term] = cachedRanks{ranks: results, version: version}
	newCache.order = append(newCache.order, term)
	// FIFO eviction: drop oldest entries past the cap.
	for len(newCache.order) > semanticCacheCap {
		evict := newCache.order[0]
		newCache.order = newCache.order[1:]
		delete(newCache.results, evict)
	}
	s.cache.Store(newCache)
}

// ClearPending clears the pending term (e.g., when user stops filtering)
func (s *SemanticSearch) ClearPending() {
	c := s.getCache()
	if c.pendingTerm == "" {
		return
	}
	newCache := &semanticResultCache{
		results:     c.results,
		order:       c.order,
		pendingTerm: "",
		lastQuery:   c.lastQuery,
	}
	s.cache.Store(newCache)
}

func (s *SemanticSearch) Snapshot() semanticSearchSnapshot {
	v := s.snapshot.Load()
	if v == nil {
		return semanticSearchSnapshot{}
	}
	return v.(semanticSearchSnapshot)
}

func (s *SemanticSearch) SetIndex(idx *search.VectorIndex, embedder search.Embedder) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	snap := s.Snapshot()
	snap.Index = idx
	snap.Embedder = embedder
	snap.Ready = idx != nil && embedder != nil
	s.snapshot.Store(snap)
}

func (s *SemanticSearch) SetIDs(ids []string) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	snap := s.Snapshot()
	if !slices.Equal(snap.IDs, ids) {
		// Items changed: bump Version so cached ranks (computed against the
		// previous snapshot) are recognized as stale on next Filter call.
		snap.Version++
	}
	cp := make([]string, len(ids))
	copy(cp, ids)
	snap.IDs = cp
	s.snapshot.Store(snap)
}

func (s *SemanticSearch) SetDocs(docs map[string]string) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	snap := s.Snapshot()
	if docs == nil {
		snap.Docs = nil
		s.snapshot.Store(snap)
		return
	}
	cp := make(map[string]string, len(docs))
	for id, doc := range docs {
		cp[id] = doc
	}
	snap.Docs = cp
	s.snapshot.Store(snap)
}

// Filter implements list.FilterFunc, returning ranks sorted by semantic similarity.
// This is non-blocking: returns cached results or fuzzy fallback immediately,
// and marks the term as pending for async computation.
func (s *SemanticSearch) Filter(term string, targets []string) []list.Rank {
	if term == "" {
		// Preserve existing sort order when the user hasn't entered a query yet.
		return list.DefaultFilter(term, targets)
	}

	snap := s.Snapshot()
	if !snap.Ready || snap.Index == nil || snap.Embedder == nil {
		// Fuzzy fallback uses the score-floor ranker (bt-6pzni) so single-word
		// queries on the fallback display are bounded by the per-query floor.
		return fuzzyRankerWithScoreFloor(term, targets)
	}
	if len(snap.IDs) != len(targets) {
		// Snapshot/list desync: targets and snap.IDs are populated by separate
		// code paths; if their lengths disagree the parallel-array contract
		// underpinning the rank Index values is broken. Fall back to fuzzy.
		return fuzzyRankerWithScoreFloor(term, targets)
	}

	// Check cache first. Entries tagged with a stale version reference indices
	// into a previous snapshot and would panic in list.filterItems where
	// items[r.Index] reads past the current targets length.
	c := s.getCache()
	if cached, ok := c.results[term]; ok && cached.version == snap.Version {
		return cached.ranks
	}

	return s.markPendingAndFallback(term, targets)
}

// markPendingAndFallback flags term for async semantic computation and returns
// fuzzy results so the UI stays responsive. Called from both the cache-miss
// path and after a stale-version reject — single fallback policy in one place.
// The fuzzy fallback applies the per-query sahilm score floor (bt-6pzni) so
// the temporary display while semantic computes is bounded by the same
// relevance gate as fuzzy mode.
func (s *SemanticSearch) markPendingAndFallback(term string, targets []string) []list.Rank {
	c := s.getCache()
	newCache := &semanticResultCache{
		results:     c.results,
		order:       c.order,
		pendingTerm: term,
		lastQuery:   time.Now(),
	}
	s.cache.Store(newCache)
	return fuzzyRankerWithScoreFloor(term, targets)
}

// ComputeSemanticResults computes semantic similarity results synchronously.
// This should be called from an async tea.Cmd, not from Filter. Returns the
// snapshot Version captured at the start of the computation so callers can
// reject results that landed after a snapshot change.
func (s *SemanticSearch) ComputeSemanticResults(term string) ([]list.Rank, uint64) {
	snap := s.Snapshot()
	if !snap.Ready || snap.Index == nil || snap.Embedder == nil {
		return nil, snap.Version
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	vecs, err := snap.Embedder.Embed(ctx, []string{term})
	if err != nil || len(vecs) != 1 {
		return nil, snap.Version
	}
	q := vecs[0]

	hybridConfig := s.getHybridConfig()
	var scorer search.HybridScorer
	if hybridConfig.Enabled {
		if cache := s.getMetricsCache(); cache != nil {
			weights := search.AdjustWeightsForQuery(hybridConfig.Weights, term)
			scorer = search.NewHybridScorer(weights, cache)
		}
	}

	type scored struct {
		index     int
		id        string
		score     float64
		textScore float64
		hasVector bool
	}

	scoredItems := make([]scored, len(snap.IDs))
	scoreMap := make(map[string]SemanticScore, len(snap.IDs))
	for i, id := range snap.IDs {
		entry, ok := snap.Index.Get(id)
		textScore := 0.0
		score := 0.0
		if !ok {
			// Item not in index (e.g. new issue before re-indexing).
			// Assign lowest possible score to keep it in the list but at the bottom.
			score = -2.0
			textScore = score
		} else {
			textScore = dotFloat32(q, entry.Vector)
			if doc, ok := snap.Docs[id]; ok {
				textScore += search.ShortQueryLexicalBoost(term, doc)
			}
			score = textScore
		}
		scoredItems[i] = scored{
			index:     i,
			id:        id,
			score:     score,
			textScore: textScore,
			hasVector: ok,
		}
		scoreMap[id] = SemanticScore{
			Score:     score,
			TextScore: textScore,
		}
	}

	limit := 75
	if scorer != nil {
		candidateLimit := search.HybridCandidateLimit(limit, len(scoredItems), term)
		var candidateIDs map[string]struct{}
		if candidateLimit < len(scoredItems) {
			candidates := make([]scored, len(scoredItems))
			copy(candidates, scoredItems)
			sort.Slice(candidates, func(i, j int) bool {
				if candidates[i].textScore == candidates[j].textScore {
					return candidates[i].id < candidates[j].id
				}
				return candidates[i].textScore > candidates[j].textScore
			})
			if candidateLimit < len(candidates) {
				candidates = candidates[:candidateLimit]
			}
			candidateIDs = make(map[string]struct{}, len(candidates))
			for _, item := range candidates {
				if item.hasVector {
					candidateIDs[item.id] = struct{}{}
				}
			}
		}

		for i := range scoredItems {
			item := &scoredItems[i]
			if !item.hasVector {
				continue
			}
			if candidateIDs != nil {
				if _, ok := candidateIDs[item.id]; !ok {
					continue
				}
			}
			hybridScore, err := scorer.Score(item.id, item.textScore)
			if err != nil {
				continue
			}
			item.score = hybridScore.FinalScore
			scoreMap[item.id] = SemanticScore{
				Score:      hybridScore.FinalScore,
				TextScore:  hybridScore.TextScore,
				Components: hybridScore.ComponentScores,
			}
		}
	}

	sort.Slice(scoredItems, func(i, j int) bool {
		if scoredItems[i].score == scoredItems[j].score {
			return scoredItems[i].id < scoredItems[j].id
		}
		return scoredItems[i].score > scoredItems[j].score
	})

	if len(scoredItems) > limit {
		scoredItems = scoredItems[:limit]
	}

	// Change A: relevance floor. Items whose final score falls below
	// HybridScoreFloor are excluded from results. The threshold value
	// (0.05) matches the former SearchScoreBadgeMinAbs constant removed in
	// bt-r3zxj — the project already decided sub-0.05 hybrid scores were
	// noise. Applies to both semantic-only (text score) and hybrid (final
	// score) paths. If the floor empties the result set the caller receives
	// an empty slice, which causes Bubbles to render its built-in "No items."
	// indicator — no additional empty-state surface needed (bt-6pzni).
	out := make([]list.Rank, 0, len(scoredItems))
	for _, it := range scoredItems {
		if it.score < HybridScoreFloor {
			break // sorted descending, so all remaining are also below floor
		}
		out = append(out, list.Rank{Index: it.index})
	}
	s.SetScores(term, scoreMap)
	return out, snap.Version
}

// SemanticIndexReadyMsg is emitted when the semantic index build/update completes.
type SemanticIndexReadyMsg struct {
	Embedder  search.Embedder
	Index     *search.VectorIndex
	IndexPath string
	Loaded    bool
	Stats     search.IndexSyncStats
	Error     error
}

// SemanticFilterResultMsg is emitted when async semantic filter results are
// ready. Version is the snapshot version the results were computed against;
// the consumer rejects results whose version no longer matches the current
// snapshot (the user changed filters mid-flight).
type SemanticFilterResultMsg struct {
	Term    string
	Results []list.Rank
	Version uint64
}

// HybridMetricsReadyMsg is emitted when hybrid metrics are ready for scoring.
type HybridMetricsReadyMsg struct {
	Cache search.MetricsCache
	Error error
}

// ComputeSemanticFilterCmd computes semantic filter results asynchronously.
func ComputeSemanticFilterCmd(s *SemanticSearch, term string) tea.Cmd {
	return func() tea.Msg {
		results, version := s.ComputeSemanticResults(term)
		return SemanticFilterResultMsg{
			Term:    term,
			Results: results,
			Version: version,
		}
	}
}

// BuildHybridMetricsCmd computes metrics for hybrid scoring asynchronously.
func BuildHybridMetricsCmd(issues []model.Issue) tea.Cmd {
	return func() tea.Msg {
		loader := search.NewAnalyzerMetricsLoader(issues).WithCache(analysis.GetGlobalCache())
		metrics, err := loader.LoadMetrics()
		if err != nil {
			return HybridMetricsReadyMsg{Error: err}
		}

		maxBlocker := 0
		for _, metric := range metrics {
			if metric.BlockerCount > maxBlocker {
				maxBlocker = metric.BlockerCount
			}
		}

		cache := &staticMetricsCache{
			metrics:    metrics,
			maxBlocker: maxBlocker,
		}
		return HybridMetricsReadyMsg{Cache: cache}
	}
}

// bootSearchMode picks the initial Ctrl+S cycle position based on whether a
// persisted semantic index exists for this project on disk (bt-ja2y).
//
//   - Index file present → boot in hybrid (best general-purpose mode; the
//     Init dispatch loads the index in the background, search is live within
//     a beat or two of startup).
//   - Index file missing → boot in fuzzy (zero-cost, instant, no index
//     needed). User can press Ctrl+S to upgrade — that path triggers the
//     build.
//
// Reads the project directory from os.Getwd() so the index path matches what
// BuildSemanticIndexCmd produces — symmetric with the build path. Returns
// false when the cwd can't be determined or the file is missing/empty.
// Embedding config is read from BT_SEMANTIC_* env vars.
func bootSearchMode() (mode searchMode, indexExists bool) {
	projectDir, err := os.Getwd()
	if err != nil || projectDir == "" {
		return searchModeFuzzy, false
	}
	cfg := search.EmbeddingConfigFromEnv()
	indexPath := search.DefaultIndexPath(projectDir, cfg)
	info, statErr := os.Stat(indexPath)
	if statErr != nil || info.Size() == 0 {
		return searchModeFuzzy, false
	}
	return searchModeHybrid, true
}

// BuildSemanticIndexCmd builds or updates the semantic index for the given issues.
func BuildSemanticIndexCmd(issues []model.Issue) tea.Cmd {
	return func() tea.Msg {
		cfg := search.EmbeddingConfigFromEnv()
		embedder, err := search.NewEmbedderFromConfig(cfg)
		if err != nil {
			return SemanticIndexReadyMsg{Error: err}
		}

		projectDir, err := os.Getwd()
		if err != nil {
			return SemanticIndexReadyMsg{Error: err}
		}

		indexPath := search.DefaultIndexPath(projectDir, cfg)
		idx, loaded, err := search.LoadOrNewVectorIndex(indexPath, embedder.Dim())
		if err != nil {
			return SemanticIndexReadyMsg{Error: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		docs := search.DocumentsFromIssues(issues)
		stats, err := search.SyncVectorIndex(ctx, idx, embedder, docs, 64)
		if err != nil {
			return SemanticIndexReadyMsg{Error: err}
		}
		if !loaded || stats.Changed() {
			if err := idx.Save(indexPath); err != nil {
				return SemanticIndexReadyMsg{Error: fmt.Errorf("save semantic index: %w", err)}
			}
		}

		return SemanticIndexReadyMsg{
			Embedder:  embedder,
			Index:     idx,
			IndexPath: indexPath,
			Loaded:    loaded,
			Stats:     stats,
		}
	}
}

func dotFloat32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

type staticMetricsCache struct {
	metrics    map[string]search.IssueMetrics
	maxBlocker int
}

func (c *staticMetricsCache) Get(issueID string) (search.IssueMetrics, bool) {
	metric, ok := c.metrics[issueID]
	return metric, ok
}

func (c *staticMetricsCache) GetBatch(issueIDs []string) map[string]search.IssueMetrics {
	results := make(map[string]search.IssueMetrics, len(issueIDs))
	for _, id := range issueIDs {
		if metric, ok := c.metrics[id]; ok {
			results[id] = metric
		}
	}
	return results
}

func (c *staticMetricsCache) Refresh() error {
	return nil
}

func (c *staticMetricsCache) DataHash() string {
	return ""
}

func (c *staticMetricsCache) MaxBlockerCount() int {
	return c.maxBlocker
}

// ════════════════════════════════════════════════════════════════════════════
// Model helper methods for semantic search state management
// (moved from model.go to keep semantic search code co-located)
// ════════════════════════════════════════════════════════════════════════════

func cloneIssuesForAsync(issues []model.Issue) []model.Issue {
	if len(issues) == 0 {
		return nil
	}
	clones := make([]model.Issue, len(issues))
	for i := range issues {
		clones[i] = issues[i].Clone()
	}
	return clones
}

func (m *Model) updateSemanticIDs(items []list.Item) {
	if m.semanticSearch == nil {
		return
	}
	ids := make([]string, 0, len(items))
	docs := make(map[string]string, len(items))
	for _, it := range items {
		if issueItem, ok := it.(IssueItem); ok {
			id := issueItem.Issue.ID
			ids = append(ids, id)
			docs[id] = search.IssueDocument(issueItem.Issue)
		}
	}
	m.semanticSearch.SetIDs(ids)
	m.semanticSearch.SetDocs(docs)
}

func (m *Model) updateListDelegate() {
	m.list.SetDelegate(IssueDelegate{
		Theme:             m.theme,
		ShowPriorityHints: m.ac.showPriorityHints,
		PriorityHints:     m.ac.priorityHints,
		WorkspaceMode:     m.workspaceMode,
	})
}

func (m *Model) applySemanticScores(term string) {
	if m.semanticSearch == nil {
		return
	}
	scores, ok := m.semanticSearch.Scores(term)
	if !ok {
		return
	}
	items := m.list.Items()
	for i := range items {
		issueItem, ok := items[i].(IssueItem)
		if !ok {
			continue
		}
		if score, ok := scores[issueItem.Issue.ID]; ok {
			issueItem.SearchScore = score.Score
			issueItem.SearchTextScore = score.TextScore
			issueItem.SearchComponents = score.Components
			issueItem.SearchScoreSet = true
		} else {
			issueItem.SearchScore = 0
			issueItem.SearchTextScore = 0
			issueItem.SearchComponents = nil
			issueItem.SearchScoreSet = false
		}
		items[i] = issueItem
	}
}

func (m *Model) clearSemanticScores() {
	items := m.list.Items()
	changed := false
	for i := range items {
		issueItem, ok := items[i].(IssueItem)
		if !ok {
			continue
		}
		if issueItem.SearchScoreSet || issueItem.SearchComponents != nil {
			issueItem.SearchScore = 0
			issueItem.SearchTextScore = 0
			issueItem.SearchComponents = nil
			issueItem.SearchScoreSet = false
			items[i] = issueItem
			changed = true
		}
	}
	if changed && m.list.FilterState() != list.Unfiltered {
		prevState := m.list.FilterState()
		currentTerm := m.list.FilterInput.Value()
		m.list.SetFilterText(currentTerm)
		if prevState == list.Filtering {
			m.list.SetFilterState(list.Filtering)
		}
	}
}

func (m *Model) issuesForAsync() []model.Issue {
	if m == nil {
		return nil
	}
	if (m.data.snapshot != nil && len(m.data.snapshot.pooledIssues) > 0) || len(m.data.pooledIssues) > 0 {
		return cloneIssuesForAsync(m.data.issues)
	}
	return m.data.issues
}
