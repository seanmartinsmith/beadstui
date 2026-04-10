package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/search"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type semanticSearchSnapshot struct {
	Ready    bool
	Index    *search.VectorIndex
	Embedder search.Embedder
	IDs      []string
	Docs     map[string]string
}

// semanticResultCache holds cached filter results and pending state
type semanticResultCache struct {
	results     map[string][]list.Rank // term -> ranks
	pendingTerm string                 // term awaiting async computation
	lastQuery   time.Time              // for debounce
}

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
	snapshot     atomic.Value // semanticSearchSnapshot
	cache        atomic.Value // *semanticResultCache
	scores       atomic.Value // *semanticScoreCache
	hybridConfig atomic.Value // semanticHybridConfig
	metricsCache atomic.Value // *metricsCacheHolder
}

func NewSemanticSearch() *SemanticSearch {
	s := &SemanticSearch{}
	s.snapshot.Store(semanticSearchSnapshot{})
	s.cache.Store(&semanticResultCache{results: make(map[string][]list.Rank)})
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
		return &semanticResultCache{results: make(map[string][]list.Rank)}
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
	s.cache.Store(&semanticResultCache{results: make(map[string][]list.Rank)})
	s.ClearScores()
}

// SetCachedResults stores semantic filter results and clears pending state if matching
func (s *SemanticSearch) SetCachedResults(term string, results []list.Rank) {
	c := s.getCache()

	// Only clear pending if this is the term that was pending
	// Otherwise preserve the current pending term (user may have typed a new query)
	newPendingTerm := c.pendingTerm
	if c.pendingTerm == term {
		newPendingTerm = ""
	}

	newCache := &semanticResultCache{
		results:     make(map[string][]list.Rank),
		pendingTerm: newPendingTerm,
		lastQuery:   c.lastQuery,
	}
	// Copy existing cache entries (keep a small LRU-like cache)
	for k, v := range c.results {
		newCache.results[k] = v
	}
	// Limit cache size to prevent memory bloat
	if len(newCache.results) > 20 {
		// Clear old entries (simple approach: clear all)
		newCache.results = make(map[string][]list.Rank)
	}
	newCache.results[term] = results
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
	snap := s.Snapshot()
	snap.Index = idx
	snap.Embedder = embedder
	snap.Ready = idx != nil && embedder != nil
	s.snapshot.Store(snap)
}

func (s *SemanticSearch) SetIDs(ids []string) {
	snap := s.Snapshot()
	cp := make([]string, len(ids))
	copy(cp, ids)
	snap.IDs = cp
	s.snapshot.Store(snap)
}

func (s *SemanticSearch) SetDocs(docs map[string]string) {
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
		return list.DefaultFilter(term, targets)
	}
	if len(snap.IDs) != len(targets) {
		// If we don't have a stable ID mapping, fall back to fuzzy filtering.
		return list.DefaultFilter(term, targets)
	}

	// Check cache first - return immediately if we have cached results
	c := s.getCache()
	if cached, ok := c.results[term]; ok {
		return cached
	}

	// No cached results - mark as pending and return fuzzy results
	// The async computation will be triggered by the model
	newCache := &semanticResultCache{
		results:     c.results,
		pendingTerm: term,
		lastQuery:   time.Now(),
	}
	s.cache.Store(newCache)

	// Return fuzzy results immediately so UI stays responsive
	return list.DefaultFilter(term, targets)
}

// ComputeSemanticResults computes semantic similarity results synchronously.
// This should be called from an async tea.Cmd, not from Filter.
func (s *SemanticSearch) ComputeSemanticResults(term string) []list.Rank {
	snap := s.Snapshot()
	if !snap.Ready || snap.Index == nil || snap.Embedder == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	vecs, err := snap.Embedder.Embed(ctx, []string{term})
	if err != nil || len(vecs) != 1 {
		return nil
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
	out := make([]list.Rank, 0, len(scoredItems))
	for _, it := range scoredItems {
		out = append(out, list.Rank{Index: it.index})
	}
	s.SetScores(term, scoreMap)
	return out
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

// SemanticFilterResultMsg is emitted when async semantic filter results are ready.
type SemanticFilterResultMsg struct {
	Term    string
	Results []list.Rank
}

// HybridMetricsReadyMsg is emitted when hybrid metrics are ready for scoring.
type HybridMetricsReadyMsg struct {
	Cache search.MetricsCache
	Error error
}

// ComputeSemanticFilterCmd computes semantic filter results asynchronously.
func ComputeSemanticFilterCmd(s *SemanticSearch, term string) tea.Cmd {
	return func() tea.Msg {
		results := s.ComputeSemanticResults(term)
		return SemanticFilterResultMsg{
			Term:    term,
			Results: results,
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
