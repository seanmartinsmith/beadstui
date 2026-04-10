package analysis

import (
	"container/heap"
	"context"
	"sort"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// intHeap implements heap.Interface for a min-heap of ints.
// Used for deterministic O(log n) extraction in Kahn's algorithm.
type intHeap []int

func (h intHeap) Len() int           { return len(h) }
func (h intHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h intHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *intHeap) Push(x any) { *h = append(*h, x.(int)) }
func (h *intHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// AdvancedInsightsConfig holds caps and limits for advanced analysis features.
// All caps ensure deterministic, bounded outputs suitable for agents.
type AdvancedInsightsConfig struct {
	// TopK caps
	TopKSetLimit     int `json:"topk_set_limit"`     // Max items in top-k unlock set (default 5)
	CoverageSetLimit int `json:"coverage_set_limit"` // Max items in coverage set (default 5)

	// Path caps
	KPathsLimit   int `json:"k_paths_limit"`   // Max number of critical paths (default 5)
	PathLengthCap int `json:"path_length_cap"` // Max path length before truncation (default 50)

	// Cycle break caps
	CycleBreakLimit int `json:"cycle_break_limit"` // Max cycle break suggestions (default 5)

	// Parallel analysis caps
	ParallelCutLimit int `json:"parallel_cut_limit"` // Max parallel cut suggestions (default 5)
}

// DefaultAdvancedInsightsConfig returns safe defaults for all caps.
func DefaultAdvancedInsightsConfig() AdvancedInsightsConfig {
	return AdvancedInsightsConfig{
		TopKSetLimit:     5,
		CoverageSetLimit: 5,
		KPathsLimit:      5,
		PathLengthCap:    50,
		CycleBreakLimit:  5,
		ParallelCutLimit: 5,
	}
}

// AdvancedInsights provides structured, capped outputs for advanced graph analysis.
// Each feature includes status tracking and usage hints for agent consumption.
type AdvancedInsights struct {
	// TopKSet: Best set of k beads maximizing downstream unlocks (submodular selection)
	TopKSet *TopKSetResult `json:"topk_set,omitempty"`

	// CoverageSet: Minimal set covering all critical paths
	CoverageSet *CoverageSetResult `json:"coverage_set,omitempty"`

	// KPaths: K-shortest critical paths through the dependency graph
	KPaths *KPathsResult `json:"k_paths,omitempty"`

	// ParallelCut: Suggestions for maximizing parallel work
	ParallelCut *ParallelCutResult `json:"parallel_cut,omitempty"`

	// ParallelGain: Parallelization gain metrics for top recommendations
	ParallelGain *ParallelGainResult `json:"parallel_gain,omitempty"`

	// CycleBreak: Suggestions for breaking cycles with minimal collateral impact
	CycleBreak *CycleBreakResult `json:"cycle_break,omitempty"`

	// Config: Caps and limits used for this analysis
	Config AdvancedInsightsConfig `json:"config"`

	// UsageHints: Agent-friendly guidance for each feature
	UsageHints map[string]string `json:"usage_hints"`
}

// FeatureStatus tracks computation state for a single advanced feature.
type FeatureStatus struct {
	State   string `json:"state"`             // available|pending|skipped|error
	Reason  string `json:"reason,omitempty"`  // Explanation when skipped/error
	Capped  bool   `json:"capped,omitempty"`  // True if results were truncated
	Count   int    `json:"count,omitempty"`   // Number of results returned
	Limited int    `json:"limited,omitempty"` // Original count before capping
}

// TopKSetResult represents the optimal set of issues to complete for maximum unlock.
type TopKSetResult struct {
	Status       FeatureStatus `json:"status"`
	Items        []TopKSetItem `json:"items,omitempty"`         // Ordered by selection sequence
	TotalGain    int           `json:"total_gain"`              // Total issues unlocked by set
	MarginalGain []int         `json:"marginal_gain,omitempty"` // Gain per item added
	HowToUse     string        `json:"how_to_use"`
}

// TopKSetItem represents one issue in the top-k unlock set.
type TopKSetItem struct {
	ID           string   `json:"id"`
	Title        string   `json:"title,omitempty"`
	MarginalGain int      `json:"marginal_gain"`      // Additional unlocks from this pick
	Unblocks     []string `json:"unblocks,omitempty"` // IDs directly unblocked
}

// CoverageSetResult represents minimal set covering all dependency edges (vertex cover).
// Uses greedy 2-approximation algorithm for bounded, deterministic output (bv-152).
type CoverageSetResult struct {
	Status        FeatureStatus  `json:"status"`
	Items         []CoverageItem `json:"items,omitempty"`
	EdgesCovered  int            `json:"edges_covered"`  // Number of edges covered by this set
	TotalEdges    int            `json:"total_edges"`    // Total edges in the dependency graph
	CoverageRatio float64        `json:"coverage_ratio"` // EdgesCovered / TotalEdges (0.0-1.0)
	Rationale     string         `json:"rationale"`      // Explanation of selection strategy
	HowToUse      string         `json:"how_to_use"`
}

// CoverageItem represents one issue in the coverage set.
type CoverageItem struct {
	ID           string `json:"id"`
	Title        string `json:"title,omitempty"`
	EdgesAdded   int    `json:"edges_added"`   // Edges newly covered by including this node
	TotalDegree  int    `json:"total_degree"`  // Total edges incident to this node
	SelectionSeq int    `json:"selection_seq"` // Order in which this was selected (1-indexed)
}

// KPathsResult represents K-shortest critical paths.
type KPathsResult struct {
	Status   FeatureStatus  `json:"status"`
	Paths    []CriticalPath `json:"paths,omitempty"`
	HowToUse string         `json:"how_to_use"`
}

// CriticalPath represents one critical path through the graph.
type CriticalPath struct {
	Rank      int      `json:"rank"`                // 1-indexed path rank
	Length    int      `json:"length"`              // Number of nodes in path
	IssueIDs  []string `json:"issue_ids"`           // Path from source to sink
	Truncated bool     `json:"truncated,omitempty"` // True if path was capped
}

// ParallelCutResult represents suggestions for parallel work maximization.
type ParallelCutResult struct {
	Status      FeatureStatus     `json:"status"`
	Suggestions []ParallelCutItem `json:"suggestions,omitempty"`
	MaxParallel int               `json:"max_parallel"` // Maximum parallelism achievable
	HowToUse    string            `json:"how_to_use"`
}

// ParallelCutItem represents one parallel cut suggestion.
type ParallelCutItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title,omitempty"`
	ParallelGain  int      `json:"parallel_gain"`            // Additional parallel streams enabled
	EnabledTracks []string `json:"enabled_tracks,omitempty"` // Track IDs enabled
}

// ParallelGainResult provides parallelization metrics for top recommendations.
type ParallelGainResult struct {
	Status   FeatureStatus      `json:"status"`
	Metrics  []ParallelGainItem `json:"metrics,omitempty"`
	HowToUse string             `json:"how_to_use"`
}

// ParallelGainItem represents parallelization gain for one issue.
type ParallelGainItem struct {
	ID                string  `json:"id"`
	Title             string  `json:"title,omitempty"`
	CurrentParallel   int     `json:"current_parallel"`   // Current parallel streams
	PotentialParallel int     `json:"potential_parallel"` // After completion
	GainPercent       float64 `json:"gain_percent"`       // Percentage improvement
}

// CycleBreakResult provides suggestions for breaking cycles.
type CycleBreakResult struct {
	Status      FeatureStatus    `json:"status"`
	Suggestions []CycleBreakItem `json:"suggestions,omitempty"`
	CycleCount  int              `json:"cycle_count"` // Total cycles detected
	HowToUse    string           `json:"how_to_use"`
	Advisory    string           `json:"advisory"` // Important warning text
}

// CycleBreakItem represents one cycle break suggestion.
type CycleBreakItem struct {
	EdgeFrom   string `json:"edge_from"`  // Source node of edge to remove
	EdgeTo     string `json:"edge_to"`    // Target node of edge to remove
	Impact     int    `json:"impact"`     // Number of cycles broken
	Collateral int    `json:"collateral"` // Dependents affected
	InCycles   []int  `json:"in_cycles"`  // Cycle indices containing this edge
	Rationale  string `json:"rationale"`  // Why this edge is suggested
}

// DefaultUsageHints returns agent-friendly guidance for each feature.
func DefaultUsageHints() map[string]string {
	return map[string]string{
		"topk_set":      "Best k issues to complete for max downstream unlock. Work these in order.",
		"coverage_set":  "Small vertex cover touching all dependency edges. Use for breadth coverage.",
		"k_paths":       "K-shortest critical paths. Focus on issues appearing in multiple paths.",
		"parallel_cut":  "Issues that enable parallel work. Complete to maximize team throughput.",
		"parallel_gain": "Parallelization improvement from completing each issue.",
		"cycle_break":   "Structural fix suggestions. Apply BEFORE working on cycle members.",
	}
}

// GenerateAdvancedInsights creates the advanced insights structure with current data.
// Features that aren't yet implemented return status=pending.
func (a *Analyzer) GenerateAdvancedInsights(config AdvancedInsightsConfig) *AdvancedInsights {
	insights := &AdvancedInsights{
		Config:     config,
		UsageHints: DefaultUsageHints(),
	}

	// TopK Set - greedy submodular selection for maximum unlock (bv-145)
	insights.TopKSet = a.generateTopKSet(config.TopKSetLimit)

	// Coverage Set - greedy 2-approx vertex cover (bv-152)
	insights.CoverageSet = a.generateCoverageSet(config.CoverageSetLimit)

	// K-Paths - top k longest/critical paths through the dependency graph (bv-153)
	insights.KPaths = a.generateKPaths(config.KPathsLimit, config.PathLengthCap)

	// Parallel Cut - suggestions for maximizing parallel work (bv-154)
	insights.ParallelCut = a.generateParallelCut(config.ParallelCutLimit)

	// Parallel Gain - placeholder until bv-129 implements
	insights.ParallelGain = &ParallelGainResult{
		Status: FeatureStatus{
			State:  "pending",
			Reason: "Awaiting implementation (bv-129)",
		},
		HowToUse: DefaultUsageHints()["parallel_gain"],
	}

	// Cycle Break - implement basic version using existing cycle detection
	insights.CycleBreak = a.generateCycleBreakSuggestions(config.CycleBreakLimit)

	return insights
}

// generateCycleBreakSuggestions creates cycle break suggestions from existing cycle data.
func (a *Analyzer) generateCycleBreakSuggestions(limit int) *CycleBreakResult {
	stats := a.AnalyzeAsync(context.Background())
	stats.WaitForPhase2()
	cycles := stats.Cycles()

	if len(cycles) == 0 {
		return &CycleBreakResult{
			Status: FeatureStatus{
				State: "available",
				Count: 0,
			},
			CycleCount: 0,
			HowToUse:   DefaultUsageHints()["cycle_break"],
			Advisory:   "No cycles detected - dependency graph is a proper DAG.",
		}
	}

	// Build edge frequency map across cycles
	type edgeKey struct{ from, to string }
	edgeFreq := make(map[edgeKey][]int) // edge -> cycle indices

	for i, cycle := range cycles {
		if len(cycle) < 2 {
			continue
		}
		// Handle special markers
		if cycle[0] == "CYCLE_DETECTION_TIMEOUT" || cycle[0] == "..." {
			continue
		}
		for j := 0; j < len(cycle)-1; j++ {
			key := edgeKey{from: cycle[j], to: cycle[j+1]}
			edgeFreq[key] = append(edgeFreq[key], i)
		}
		// Close the cycle
		key := edgeKey{from: cycle[len(cycle)-1], to: cycle[0]}
		edgeFreq[key] = append(edgeFreq[key], i)
	}

	// Rank edges by frequency (breaking highest-frequency edges affects most cycles)
	type edgeRank struct {
		key    edgeKey
		cycles []int
		count  int
	}
	var ranked []edgeRank
	for k, cycs := range edgeFreq {
		ranked = append(ranked, edgeRank{key: k, cycles: cycs, count: len(cycs)})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		// Deterministic tie-break by edge lexicographically
		if ranked[i].key.from != ranked[j].key.from {
			return ranked[i].key.from < ranked[j].key.from
		}
		return ranked[i].key.to < ranked[j].key.to
	})

	// Cap and build suggestions
	suggestions := make([]CycleBreakItem, 0, limit)
	for i, r := range ranked {
		if i >= limit {
			break
		}
		suggestions = append(suggestions, CycleBreakItem{
			EdgeFrom:   r.key.from,
			EdgeTo:     r.key.to,
			Impact:     r.count,
			Collateral: a.countDependents(r.key.to),
			InCycles:   r.cycles,
			Rationale:  "Appears in most cycles; removing minimizes structural damage.",
		})
	}

	capped := len(ranked) > limit
	return &CycleBreakResult{
		Status: FeatureStatus{
			State:   "available",
			Count:   len(suggestions),
			Capped:  capped,
			Limited: len(ranked),
		},
		Suggestions: suggestions,
		CycleCount:  len(cycles),
		HowToUse:    DefaultUsageHints()["cycle_break"],
		Advisory:    "Structural fix—apply cycle breaks BEFORE executing dependents.",
	}
}

// countDependents returns the number of issues that depend on the given issue.
func (a *Analyzer) countDependents(issueID string) int {
	count := 0
	nodeID, exists := a.idToNode[issueID]
	if !exists {
		return 0
	}
	to := a.g.To(nodeID)
	for to.Next() {
		count++
	}
	return count
}

// generateTopKSet implements greedy submodular selection to find the best k issues
// that maximize downstream unlocks when completed together (bv-145).
func (a *Analyzer) generateTopKSet(k int) *TopKSetResult {
	if k <= 0 {
		k = 5 // default
	}

	// Get actionable (non-closed) issues as candidates
	var candidates []string
	for id, issue := range a.issueMap {
		if !isClosedLikeStatus(issue.Status) {
			candidates = append(candidates, id)
		}
	}
	sort.Strings(candidates) // deterministic ordering

	if len(candidates) == 0 {
		return &TopKSetResult{
			Status: FeatureStatus{
				State:  "available",
				Count:  0,
				Reason: "No actionable issues",
			},
			HowToUse: DefaultUsageHints()["topk_set"],
		}
	}

	// Track which issues we've "completed" in our greedy selection
	completed := make(map[string]bool)
	var items []TopKSetItem
	var marginalGains []int
	totalGain := 0

	// Greedy selection: pick k items with highest marginal gain
	for i := 0; i < k && len(candidates) > 0; i++ {
		bestID := ""
		bestGain := -1
		var bestUnblocks []string

		// Evaluate each remaining candidate
		for _, candID := range candidates {
			if completed[candID] {
				continue
			}
			unblocks := a.computeMarginalUnblocks(candID, completed)
			gain := len(unblocks)
			// Tie-break by ID for determinism
			if gain > bestGain || (gain == bestGain && (bestID == "" || candID < bestID)) {
				bestID = candID
				bestGain = gain
				bestUnblocks = unblocks
			}
		}

		if bestID == "" {
			break // no more candidates
		}

		// Select this candidate
		completed[bestID] = true
		title := ""
		if issue, exists := a.issueMap[bestID]; exists {
			title = issue.Title
		}
		items = append(items, TopKSetItem{
			ID:           bestID,
			Title:        title,
			MarginalGain: bestGain,
			Unblocks:     bestUnblocks,
		})
		marginalGains = append(marginalGains, bestGain)
		totalGain += bestGain
	}

	return &TopKSetResult{
		Status: FeatureStatus{
			State:   "available",
			Count:   len(items),
			Capped:  len(items) >= k && len(candidates) > k,
			Limited: len(candidates),
		},
		Items:        items,
		TotalGain:    totalGain,
		MarginalGain: marginalGains,
		HowToUse:     DefaultUsageHints()["topk_set"],
	}
}

// computeMarginalUnblocks computes which issues would become actionable if we complete
// the given issue, assuming the issues in 'alreadyCompleted' are also done.
func (a *Analyzer) computeMarginalUnblocks(issueID string, alreadyCompleted map[string]bool) []string {
	var unblocks []string

	for _, issue := range a.issueMap {
		// Skip closed issues
		if isClosedLikeStatus(issue.Status) {
			continue
		}
		// Skip if already "completed" in our simulation
		if alreadyCompleted[issue.ID] {
			continue
		}
		// Skip if this is the candidate itself
		if issue.ID == issueID {
			continue
		}

		// Check if this issue would become unblocked
		wouldBeBlocked := false
		hasThisBlocker := false

		for _, dep := range issue.Dependencies {
			if dep == nil {
				continue
			}
			if dep.Type != model.DepBlocks {
				continue
			}

			if dep.DependsOnID == issueID {
				hasThisBlocker = true
				continue
			}

			// Check if there's another open blocker (not already completed)
			if blocker, exists := a.issueMap[dep.DependsOnID]; exists {
				if !isClosedLikeStatus(blocker.Status) && !alreadyCompleted[dep.DependsOnID] {
					wouldBeBlocked = true
					break
				}
			}
		}

		// If this issue depends on issueID and would become unblocked
		if hasThisBlocker && !wouldBeBlocked {
			unblocks = append(unblocks, issue.ID)
		}
	}

	sort.Strings(unblocks)
	return unblocks
}

// generateCoverageSet computes a greedy vertex cover (2-approx) over blocking edges.
// Uses only open issues; returns deterministic ordering with caps.
func (a *Analyzer) generateCoverageSet(limit int) *CoverageSetResult {
	if limit <= 0 {
		limit = 5
	}

	// Build edge list of blocking deps between non-closed issues
	type edge struct{ from, to string }
	var edges []edge
	for id, issue := range a.issueMap {
		if isClosedLikeStatus(issue.Status) {
			continue
		}
		for _, dep := range issue.Dependencies {
			if dep == nil || dep.Type != model.DepBlocks {
				continue
			}
			if target, ok := a.issueMap[dep.DependsOnID]; ok && !isClosedLikeStatus(target.Status) {
				edges = append(edges, edge{from: id, to: dep.DependsOnID})
			}
		}
	}
	totalEdges := len(edges)
	if totalEdges == 0 {
		return &CoverageSetResult{
			Status: FeatureStatus{
				State:  "available",
				Count:  0,
				Reason: "No blocking edges to cover",
			},
			EdgesCovered:  0,
			TotalEdges:    0,
			CoverageRatio: 1.0,
			Rationale:     "Graph has no blocking dependencies.",
			HowToUse:      DefaultUsageHints()["coverage_set"],
		}
	}

	// Track uncovered edges and degrees
	uncovered := make(map[int]edge, len(edges))
	for i, e := range edges {
		uncovered[i] = e
	}

	var items []CoverageItem
	selection := 0
	edgesCovered := 0

	for len(uncovered) > 0 && len(items) < limit {
		// recompute degree from uncovered edges
		deg := make(map[string]int)
		for _, e := range uncovered {
			deg[e.from]++
			deg[e.to]++
		}

		// pick node with highest degree (tie-break lexicographically)
		bestID := ""
		bestDeg := -1
		for id, d := range deg {
			if d > bestDeg || (d == bestDeg && (bestID == "" || id < bestID)) {
				bestID, bestDeg = id, d
			}
		}
		if bestID == "" {
			break
		}

		// remove all edges incident to bestID
		added := 0
		for idx, e := range uncovered {
			if e.from == bestID || e.to == bestID {
				delete(uncovered, idx)
				added++
			}
		}
		edgesCovered += added
		selection++

		title := ""
		if issue, ok := a.issueMap[bestID]; ok {
			title = issue.Title
		}

		items = append(items, CoverageItem{
			ID:           bestID,
			Title:        title,
			EdgesAdded:   added,
			TotalDegree:  bestDeg,
			SelectionSeq: selection,
		})
	}

	capped := len(uncovered) > 0
	return &CoverageSetResult{
		Status: FeatureStatus{
			State:   "available",
			Count:   len(items),
			Capped:  capped,
			Limited: len(edges),
		},
		Items:         items,
		EdgesCovered:  edgesCovered,
		TotalEdges:    totalEdges,
		CoverageRatio: float64(edgesCovered) / float64(totalEdges),
		Rationale:     "Greedy vertex cover (2-approx): iteratively pick highest uncovered degree until edges are covered or cap is reached.",
		HowToUse:      DefaultUsageHints()["coverage_set"],
	}
}

// generateKPaths finds the k longest critical paths through the dependency graph (bv-153).
// Uses topological sort with DP to compute longest path distances, then reconstructs
// paths from nodes with highest distances. Only considers blocking edges between open issues.
func (a *Analyzer) generateKPaths(k int, pathLengthCap int) *KPathsResult {
	if k <= 0 {
		k = 5
	}
	if pathLengthCap <= 0 {
		pathLengthCap = 50
	}

	// Build adjacency list of blocking deps between non-closed issues
	// adj[from] = list of nodes that depend on 'from' (i.e., from blocks them)
	type nodeInfo struct {
		id    string
		index int
	}
	var nodes []nodeInfo
	idToIndex := make(map[string]int)

	// Collect non-closed issues
	for id, issue := range a.issueMap {
		if !isClosedLikeStatus(issue.Status) {
			idToIndex[id] = len(nodes)
			nodes = append(nodes, nodeInfo{id: id, index: len(nodes)})
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].id < nodes[j].id })
	// Re-index after sorting for determinism
	for i, n := range nodes {
		idToIndex[n.id] = i
	}

	n := len(nodes)
	if n == 0 {
		return &KPathsResult{
			Status: FeatureStatus{
				State:  "available",
				Count:  0,
				Reason: "No open issues",
			},
			HowToUse: DefaultUsageHints()["k_paths"],
		}
	}

	// Build adjacency: adj[i] = nodes that i blocks (i.e., they depend on i)
	adj := make([][]int, n)
	inDegree := make([]int, n)

	for _, node := range nodes {
		issue := a.issueMap[node.id]
		for _, dep := range issue.Dependencies {
			if dep == nil || dep.Type != model.DepBlocks {
				continue
			}
			// dep.DependsOnID blocks node.id
			fromIdx, ok := idToIndex[dep.DependsOnID]
			if !ok {
				continue // blocker is closed or not in graph
			}
			toIdx := idToIndex[node.id]
			adj[fromIdx] = append(adj[fromIdx], toIdx)
			inDegree[toIdx]++
		}
	}

	// Sort adjacency lists for determinism
	for i := range adj {
		sort.Ints(adj[i])
	}

	// Kahn's algorithm for topological sort using min-heap for determinism
	// Min-heap gives O(log k) per operation vs O(k log k) for sorting each iteration
	var topoOrder []int
	pq := &intHeap{}
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			*pq = append(*pq, i)
		}
	}
	heap.Init(pq) // O(k) heapify

	tempInDegree := make([]int, n)
	copy(tempInDegree, inDegree)

	for pq.Len() > 0 {
		// Pop smallest index for deterministic processing - O(log k)
		u := heap.Pop(pq).(int)
		topoOrder = append(topoOrder, u)

		for _, v := range adj[u] {
			tempInDegree[v]--
			if tempInDegree[v] == 0 {
				heap.Push(pq, v) // O(log k)
			}
		}
	}

	// If topoOrder doesn't include all nodes, there's a cycle - handle gracefully
	// If topoOrder doesn't include all nodes, the graph has cycles; we still
	// proceed with a partial ordering for path computation.

	// DP for longest path distances and predecessor tracking
	dist := make([]int, n) // dist[i] = length of longest path ending at i
	pred := make([]int, n) // pred[i] = predecessor on longest path (-1 if source)
	for i := range pred {
		pred[i] = -1
	}

	// Process in topological order
	for _, u := range topoOrder {
		for _, v := range adj[u] {
			if dist[u]+1 > dist[v] {
				dist[v] = dist[u] + 1
				pred[v] = u
			} else if dist[u]+1 == dist[v] && (pred[v] == -1 || u < pred[v]) {
				// Tie-break: prefer smaller index predecessor for determinism
				pred[v] = u
			}
		}
	}

	// Find nodes with longest paths (these are our path endpoints)
	type pathEnd struct {
		idx    int
		length int
		id     string
	}
	var pathEnds []pathEnd
	for i := 0; i < n; i++ {
		pathEnds = append(pathEnds, pathEnd{idx: i, length: dist[i], id: nodes[i].id})
	}

	// Sort by length (descending), then by ID (ascending) for determinism
	sort.Slice(pathEnds, func(i, j int) bool {
		if pathEnds[i].length != pathEnds[j].length {
			return pathEnds[i].length > pathEnds[j].length
		}
		return pathEnds[i].id < pathEnds[j].id
	})

	// Reconstruct paths from top k endpoints
	var paths []CriticalPath
	usedSources := make(map[int]bool) // Avoid returning duplicate paths (same source)

	for _, pe := range pathEnds {
		if len(paths) >= k {
			break
		}
		// Skip trivial paths (single node, no dependencies)
		if pe.length == 0 {
			continue
		}

		// Reconstruct path by walking predecessors
		var pathIndices []int
		curr := pe.idx
		for curr != -1 {
			pathIndices = append(pathIndices, curr)
			curr = pred[curr]
		}

		// Path is in reverse order (sink to source), reverse it
		for i, j := 0, len(pathIndices)-1; i < j; i, j = i+1, j-1 {
			pathIndices[i], pathIndices[j] = pathIndices[j], pathIndices[i]
		}

		// Check if we already have a path from this source
		if len(pathIndices) > 0 {
			source := pathIndices[0]
			if usedSources[source] {
				continue // Skip duplicate source paths
			}
			usedSources[source] = true
		}

		// Convert indices to issue IDs
		truncated := false
		if len(pathIndices) > pathLengthCap {
			pathIndices = pathIndices[:pathLengthCap]
			truncated = true
		}

		issueIDs := make([]string, len(pathIndices))
		for i, idx := range pathIndices {
			issueIDs[i] = nodes[idx].id
		}

		paths = append(paths, CriticalPath{
			Rank:      len(paths) + 1,
			Length:    len(issueIDs),
			IssueIDs:  issueIDs,
			Truncated: truncated,
		})
	}

	// Count total non-trivial paths for status
	totalPaths := 0
	for _, pe := range pathEnds {
		if pe.length > 0 {
			totalPaths++
		}
	}

	return &KPathsResult{
		Status: FeatureStatus{
			State:   "available",
			Count:   len(paths),
			Capped:  len(paths) >= k && totalPaths > k,
			Limited: totalPaths,
		},
		Paths:    paths,
		HowToUse: DefaultUsageHints()["k_paths"],
	}
}

// generateParallelCut finds nodes that maximize parallel work opportunities (bv-154).
// A node has positive "parallel gain" if completing it would unblock more than one
// dependent, increasing the number of items that can be worked on in parallel.
func (a *Analyzer) generateParallelCut(limit int) *ParallelCutResult {
	if limit <= 0 {
		limit = 5
	}

	// Build map of non-closed issues
	openIssues := make(map[string]bool)
	for id, issue := range a.issueMap {
		if !isClosedLikeStatus(issue.Status) {
			openIssues[id] = true
		}
	}

	if len(openIssues) == 0 {
		return &ParallelCutResult{
			Status: FeatureStatus{
				State:  "available",
				Count:  0,
				Reason: "No open issues",
			},
			MaxParallel: 0,
			HowToUse:    DefaultUsageHints()["parallel_cut"],
		}
	}

	// Build dependency graph: blockerOf[A] = list of issues that A blocks
	blockerOf := make(map[string][]string)
	blockedBy := make(map[string][]string) // blockedBy[B] = list of issues blocking B

	for id, issue := range a.issueMap {
		if !openIssues[id] {
			continue
		}
		for _, dep := range issue.Dependencies {
			if dep == nil || dep.Type != model.DepBlocks {
				continue
			}
			if openIssues[dep.DependsOnID] {
				blockerOf[dep.DependsOnID] = append(blockerOf[dep.DependsOnID], id)
				blockedBy[id] = append(blockedBy[id], dep.DependsOnID)
			}
		}
	}

	// Count current actionable issues (no open blockers)
	currentActionable := 0
	for id := range openIssues {
		if len(blockedBy[id]) == 0 {
			currentActionable++
		}
	}

	// Calculate parallel gain for each open issue
	type parallelCandidate struct {
		id            string
		parallelGain  int
		newActionable int
		enabledTracks []string
	}
	var candidates []parallelCandidate

	for id := range openIssues {
		// Count how many dependents would become actionable if this issue is completed
		var newlyActionable []string

		for _, depID := range blockerOf[id] {
			if !openIssues[depID] {
				continue
			}
			// Check if all other blockers of depID are closed (or would be after removing id)
			allOthersClosed := true
			for _, blockerID := range blockedBy[depID] {
				if blockerID != id && openIssues[blockerID] {
					allOthersClosed = false
					break
				}
			}
			if allOthersClosed {
				newlyActionable = append(newlyActionable, depID)
			}
		}

		// Parallel gain = newly actionable - 1 (the completed node leaves the actionable pool)
		// Positive gain means net increase in parallel work opportunities
		parallelGain := len(newlyActionable) - 1

		if parallelGain > 0 {
			sort.Strings(newlyActionable)
			candidates = append(candidates, parallelCandidate{
				id:            id,
				parallelGain:  parallelGain,
				newActionable: len(newlyActionable),
				enabledTracks: newlyActionable,
			})
		}
	}

	// Sort by parallel gain descending, then by ID for determinism
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].parallelGain != candidates[j].parallelGain {
			return candidates[i].parallelGain > candidates[j].parallelGain
		}
		return candidates[i].id < candidates[j].id
	})

	// Cap to limit
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	// Build suggestions
	suggestions := make([]ParallelCutItem, len(candidates))
	for i, c := range candidates {
		title := ""
		if issue, ok := a.issueMap[c.id]; ok {
			title = issue.Title
		}
		suggestions[i] = ParallelCutItem{
			ID:            c.id,
			Title:         title,
			ParallelGain:  c.parallelGain,
			EnabledTracks: c.enabledTracks,
		}
	}

	// Calculate max parallel achievable
	maxParallel := currentActionable
	for _, c := range candidates {
		maxParallel += c.parallelGain
	}

	return &ParallelCutResult{
		Status: FeatureStatus{
			State:   "available",
			Count:   len(suggestions),
			Capped:  len(suggestions) >= limit && len(candidates) >= limit,
			Limited: len(candidates),
		},
		Suggestions: suggestions,
		MaxParallel: maxParallel,
		HowToUse:    DefaultUsageHints()["parallel_cut"],
	}
}
