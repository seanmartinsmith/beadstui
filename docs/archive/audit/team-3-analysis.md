# Audit Report: Analysis Engine

**Team**: 3
**Scope**: pkg/analysis/ - graph metrics, triage scoring, label health, insights, plan generation
**Lines scanned**: ~7,800 production LOC across 24 source files, ~8,800 test LOC across 34 test files (58 .go files total)

## Architecture Summary

The analysis engine is the computational brain of bt. It operates entirely on `[]model.Issue` - the in-memory issue list - with zero dependency on any specific storage backend (JSONL, SQLite, or Dolt). This means every analysis feature is fully Dolt-compatible by design: the data source is irrelevant once issues are loaded into memory.

The package is organized around a central `Analyzer` struct that builds a directed dependency graph (using gonum's graph library) from issue dependency edges. Analysis proceeds in two phases: Phase 1 computes fast O(V+E) metrics (degree centrality, topological sort, density) synchronously on startup, while Phase 2 computes expensive metrics (PageRank, betweenness centrality, eigenvector centrality, HITS, cycles, k-core, articulation points, slack) in a background goroutine. This two-phase design allows the TUI to render immediately with basic graph data while advanced metrics trickle in. Thread-safe accessors with RWMutex protect Phase 2 data.

On top of the graph analysis, the package provides a rich scoring/recommendation layer: impact scoring (8-factor weighted composite), triage scoring (extends impact with unblock/quick-win boosts), execution planning (connected-component tracks), and a suite of "suggestion" detectors (duplicates via Jaccard similarity, missing dependency inference, label suggestions, cycle warnings). The label_health module is the largest single file (~2,000+ lines) providing per-label health scoring with velocity, freshness, flow, and criticality dimensions plus cross-label flow analysis and blockage cascade computation.

## Feature Inventory

| Feature | Location | LOC (approx) | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| Graph construction (Analyzer) | graph.go:1244-1320 | ~80 | Yes | Yes | Yes | Custom compactDirectedGraph avoids gonum's map-based allocation overhead |
| Phase 1 metrics (degree, topo, density) | graph.go:1906-1944 | ~40 | Yes | Yes | Yes | O(V+E), runs synchronously |
| Phase 2 metrics (PR, BW, EV, HITS, cycles, k-core, art, slack) | graph.go:1612-1903 | ~290 | Yes | Yes | Yes | Background goroutine with per-metric timeouts and cancellation |
| PageRank | graph.go (via gonum network.PageRank) | ~40 | Yes | Yes | Yes | Custom wrapper with damping=0.85, convergence=1e-6, timeout fallback to uniform |
| Betweenness centrality (exact) | graph.go (via gonum network.Betweenness) | ~10 | Yes | Yes | Yes | O(V*E), used for small graphs |
| Betweenness centrality (approximate) | betweenness_approx.go | ~481 | Yes | Yes | Yes | Brandes sampling with sync.Pool buffers, parallel goroutines, dense index optimization |
| Eigenvector centrality | graph.go (via gonum) | ~10 | Yes | Yes | Yes | Synchronous, usually fast |
| HITS (hubs & authorities) | graph.go (via gonum network.HITS) | ~30 | Yes | Yes | Yes | With timeout protection |
| Cycle detection | graph_cycles.go | ~143 | Yes | Yes | Yes | Tarjan SCC + iterative DFS per component, bounded by MaxCyclesToStore |
| K-core decomposition | graph.go:2179+ | ~80 | Yes | Yes | Yes | Batagelj-Zaversnik linear-time algorithm on undirected view |
| Articulation points | graph.go:2078-2093 | ~15 | Yes | Yes | Yes | Computed alongside k-core on undirected view |
| Slack (scheduling flexibility) | graph.go:2097-2175 | ~80 | Yes | Yes | Yes | Longest-path slack per node, 0 = on critical path |
| Impact scoring (8-factor) | priority.go | ~500 | Yes | Yes | Yes | PageRank(0.22) + Betweenness(0.20) + BlockerRatio(0.13) + Staleness(0.05) + Priority(0.10) + TimeToImpact(0.10) + Urgency(0.10) + Risk(0.10) |
| Risk signals | risk.go | ~358 | Yes | Yes | Yes | FanVariance + ActivityChurn + CrossRepoRisk + StatusRisk, weighted composite |
| Triage scoring | triage.go:986-1270 | ~280 | Yes | Yes | Yes | Extends impact with UnblockBoost(0.15) + QuickWinBoost(0.15) on base(0.70) |
| Triage result (full) | triage.go:14-481 | ~470 | Yes | Yes | Yes | Unified output for --robot-triage: recommendations, quick wins, blockers, health, alerts, commands |
| TriageContext (caching layer) | triage_context.go | ~312 | Yes | Yes | Yes | Lazy-computed caches for actionable issues, blocker depths, open blockers, unblocks map |
| Triage reason generation | triage.go:1272-1468 | ~200 | Yes | Yes | Yes | Emoji-prefixed, human-readable reasons for AI agents |
| Track/label grouping (multi-agent) | triage.go:1479-1647 | ~170 | Yes | Yes | Yes | BFS-layered topological grouping for parallel agent coordination |
| Project velocity | triage.go:156-260 | ~100 | Yes | Yes | Yes | ISO-week bucketed closure rates with ClosedAt/UpdatedAt fallback |
| Staleness analysis | triage.go:483-535 | ~50 | Yes | Yes | Yes | Requires correlation.HistoryReport (optional) |
| Execution plan | plan.go | ~354 | Yes | Yes | Yes | Connected-component tracks, computeUnblocks with blocking-only filter (recently fixed) |
| What-if analysis | whatif.go | ~267 | Yes | Yes | Yes | Direct/transitive unblocks, depth reduction, parallelization gain |
| Insights summary | insights.go | ~168 | Yes | Yes | Yes | Bottlenecks, keystones, influencers, hubs, authorities, cores, articulation, slack, orphans |
| Advanced insights | advanced_insights.go | ~800+ | Yes | Yes | Yes (5/6) | TopK set (greedy submodular), coverage set (2-approx vertex cover), K-paths, parallel cut, cycle break; ParallelGain is "pending" |
| Label health | label_health.go | ~2000+ | Yes | Partial | Yes | Per-label health (velocity, freshness, flow, criticality), cross-label flow matrix, blockage cascade |
| ETA estimation | eta.go | ~299 | Yes | Yes | Yes | Complexity(type*depth*descLen) / velocity(label-aware) with confidence intervals |
| Duplicate detection | duplicates.go | ~300 | Yes | Yes | Yes | Keyword-based Jaccard similarity with inverted index optimization |
| Missing dependency inference | dependency_suggest.go | ~292 | Yes | Yes | Yes | Keyword overlap + label overlap + ID mention heuristics |
| Label suggestions | label_suggest.go | ~311 | Yes | Yes | Yes | Builtin keyword-to-label mapping + learned patterns from existing labeled issues |
| Cycle warnings | cycle_warnings.go | ~226 | Yes | Yes | Yes | Suggestion generation from cycle detection, WouldCreateCycle validation |
| Unified suggestion system | suggestions.go + suggest_all.go | ~400 | Yes | Yes | Yes | SuggestionSet with filtering, confidence levels, stats; RobotSuggestOutput with jq usage hints |
| Feedback loop | feedback.go | ~336 | Yes | Yes | Yes | Accept/ignore events with exponential smoothing weight adjustments, persisted to feedback.json |
| Analysis config | config.go | ~411 | Yes | Yes | Yes | Size-tiered configs (small/medium/large/XL), env overrides (BT_SKIP_PHASE2, BT_PHASE2_TIMEOUT_S) |
| Cache (in-memory) | cache.go:1-134 | ~134 | Yes | Yes | Yes | Global cache with SHA256 data hash, TTL-based invalidation |
| Cache (incremental) | graph.go:877-952 | ~75 | Yes | Yes | Yes | Structure-hash keyed, 5min TTL, 8 max entries |
| Cache (disk, robot mode) | cache.go:634-800+ | ~180 | Yes | Yes | Yes | JSON file cache with file locking, LRU eviction, 24h max age, 10MB entry limit |
| CachedAnalyzer | cache.go:542-632 | ~90 | Yes | Yes | Yes | Wrapper combining Analyzer + Cache for TUI startup |
| Snapshot diff | diff.go | ~612 | Yes | Yes | Yes | Two-snapshot comparison: new/closed/modified/reopened issues, cycle changes, metric deltas, health trend |
| Issue fingerprint diff | cache.go:303-378 | ~75 | Yes | Yes | Yes | Per-issue content+dependency hashing for incremental diffing |
| File locking (cross-platform) | file_lock_unix.go, file_lock_windows.go | ~40 | Yes | N/A | Yes | Platform-specific flock/LockFileEx for disk cache |
| Compact directed graph | graph.go:961-1166 | ~206 | Yes | Yes | Yes | Low-allocation adjacency list graph replacing gonum's simple.DirectedGraph |
| Thread-safe accessors (Value/All pattern) | graph.go:264-488 | ~225 | Yes | Yes | Yes | O(1) single-value + iterator pattern for Phase 2 data |
| Legacy map-copy accessors | graph.go:589-846 | ~260 | Yes | Yes | Yes | Deprecated but retained for backward compat |

## Dependencies

- **Depends on**:
  - `github.com/seanmartinsmith/beadstui/pkg/model` - Issue, Status, Dependency, Comment types (the only internal dependency)
  - `github.com/seanmartinsmith/beadstui/pkg/correlation` - HistoryReport type (used only by ComputeStaleness, optional)
  - `gonum.org/v1/gonum/graph` - Graph interfaces, PageRank, Betweenness, HITS, topo.Sort, topo.TarjanSCC
  - `gonum.org/v1/gonum/graph/simple` - simple.Node, simple.Edge, simple.DirectedGraph (largely replaced by compactDirectedGraph)
  - `gonum.org/v1/gonum/graph/network` - network.Betweenness, network.PageRank, network.HITS
  - `gonum.org/v1/gonum/graph/topo` - topo.Sort, topo.TarjanSCC
  - `golang.org/x/sys/unix` / `golang.org/x/sys/windows` - file locking for disk cache
  - Standard library: `crypto/sha256`, `encoding/json`, `math`, `sync`, `sort`, `time`, `os`, `path/filepath`, `regexp`

- **Depended on by** (51 importers found):
  - `cmd/bt/main.go` - CLI entry point
  - `pkg/ui/` (15+ files) - model.go, background_worker.go, insights.go, graph.go, label_dashboard.go, flow_matrix.go, snapshot.go, attention.go, actionable.go, delegate.go, semantic_search.go, velocity_comparison.go, and tests
  - `pkg/export/` (5+ files) - graph_snapshot.go, graph_export.go, graph_interactive.go, sqlite_export.go, and tests
  - `pkg/search/` - metrics_cache_impl.go
  - `pkg/drift/` - drift.go

## Dead Code Candidates

1. **`computeCounts` (triage.go:623-655)**: Explicitly marked `Deprecated` in comment. Superseded by `computeCountsWithContext`. Still called from nowhere in production code (no references outside tests).

2. **`buildBlockersToClear` (triage.go:809-864)**: Explicitly marked `Deprecated` in comment. Superseded by `buildBlockersToClearWithContext`. No production callers.

3. **`ComputeImpactScore` (priority.go:218-226)**: Single-issue wrapper that computes ALL impact scores to find one. Inefficient and appears to have no callers outside tests.

4. **`TopImpactScores` (priority.go:229-235)**: Wrapper around ComputeImpactScores. May have no current callers (triage uses ComputeImpactScoresFromStats directly).

5. **`ComputeTriageScores` / `ComputeTriageScoresWithOptions` (triage.go:1047-1065)**: Standalone functions that create a new Analyzer internally. The triage pipeline uses `computeTriageScoresFromImpact` with a pre-built analyzer. These appear to be convenience wrappers with no production callers.

6. **`GetTopTriageScores` (triage.go:1264-1270)**: Convenience wrapper, likely no production callers.

7. **`GetBlockerDepth` (triage.go:1214-1218)** and `getBlockerDepthRecursive` (triage.go:1220-1253): Instance method on Analyzer. TriageContext.BlockerDepth provides the same computation with caching. The standalone version is less efficient.

8. **`ParallelGain` in advanced insights**: Permanently returns status "pending" (awaiting bv-129 implementation). It is a placeholder with no implementation.

9. **Legacy map-copy accessors** (graph.go:589-846): ~260 lines of deprecated methods (PageRank(), Betweenness(), etc.). They are still used by `insights.go:GenerateInsights` and some UI consumers, but are marked for replacement by the Value/All pattern.

10. **`NewSnapshot` / `NewSnapshotAt` (diff.go:26-52)**: These call `analyzer.Analyze()` synchronously which is expensive. Usage appears limited to diff comparisons. Verify these are actually called from the UI.

## Notable Findings

**Scale of the package**: At ~7,800 production lines across 24 files, this is one of the largest packages in the codebase. The test suite is even larger (~8,800 lines, 34 files) with benchmarks, golden tests, invariance tests, and pathological case tests. The test infrastructure is impressive.

**Genuinely useful features vs academic exercises**:
- **Genuinely useful for beads workflow**: Triage scoring, execution planning, computeUnblocks, what-if analysis, quick wins, blocker chain analysis, project velocity, ETA estimation, duplicate detection. These directly answer "what should I work on next?" and "what happens if I complete this?"
- **Useful but over-engineered**: Label health (2000+ lines for a feature that few beads projects would have enough label data to make meaningful), advanced insights (TopK, coverage set, K-paths are theoretically sound but the projects using beads are typically <200 issues where these analyses provide marginal value over simple sorting by triage score).
- **Academic exercises**: HITS (hubs/authorities) and eigenvector centrality provide little actionable insight beyond what PageRank + betweenness already give. The feedback loop (exponential smoothing weight adjustments) is sophisticated but has no evidence of production use.

**Compact graph implementation**: The `compactDirectedGraph` (graph.go:961-1166) is a custom adjacency-list implementation replacing gonum's `simple.DirectedGraph`. This is a legitimate optimization - gonum's implementation uses map-backed edge sets which allocate heavily during construction. The compact version uses `[][]int64` slices. Good engineering.

**Determinism is taken seriously**: Nearly every function sorts its outputs, uses deterministic tie-breaking (lexicographic by ID), and avoids map iteration order dependencies. This matters because the outputs feed robot/agent consumption where non-determinism causes flaky behavior.

**Recently fixed bug in computeUnblocks (plan.go:83-92)**: The `hasBlockingDep` function now correctly filters by `dep.Type.IsBlocking()`, preventing parent-child edges from being treated as blocking dependencies. The fix is clean and well-documented.

**Performance considerations**:
- `ConfigForSize` (config.go:86-211) provides 4 size tiers with progressively more aggressive algorithm selection. For >2000 nodes, cycles and HITS are disabled for dense graphs.
- `ApproxBetweenness` uses parallel goroutines capped at `runtime.NumCPU()` with sync.Pool'd buffer reuse. Good for large graphs.
- `computeBlockerDepths` (triage.go:1165-1209) uses memoized DFS to avoid recomputation. The TriageContext layer adds another caching level.
- The incremental graph stats cache (graph.go:877-952) avoids recomputing Phase 2 when the graph structure hasn't changed. Smart for the poll-loop use case.

**`buildUnblocksMap` (triage.go:537-619) vs `computeUnblocks` (plan.go:95-154)**: These implement the same semantic but with different approaches. `buildUnblocksMap` is O(E) operating on the graph directly (checking openBlockerCount), while `computeUnblocks` iterates per-issue. Both exist because buildUnblocksMap is optimized for bulk computation during triage, while computeUnblocks is the per-issue version used by plan generation. The dual implementation creates a maintenance risk - any semantic change must be applied to both.

**`CommandHelpers` still references `br` (triage.go:280-286, 964-983)**: The helper commands use `br update`, `br show`, `bd ready`, `bd blocked`. The `br` references may be stale - the MEMORY.md says CLI references should use `bd`. Worth verifying whether `br` is intentional (the beads CLI has both?).

**`label_health.go` is disproportionately large**: At ~2000+ lines, it accounts for roughly 25% of the package's production code. It implements per-label velocity tracking, freshness scoring, cross-label flow matrices, blockage cascade analysis, and historical velocity. This level of sophistication seems premature for the current project state.

## Questions for Synthesis

1. **Are the deprecated functions (`computeCounts`, `buildBlockersToClear`, standalone triage wrappers) actually dead?** The grep for importers found 51 files importing analysis, but I couldn't verify whether these specific functions have callers outside tests without reading all 51 files.

2. **Is `br` vs `bd` intentional in CommandHelpers?** The commands in triage.go:964-983 reference both `br` (update, show) and `bd` (ready, blocked). This may reflect beads CLI having separate binaries, or it may be a rename artifact.

3. **Is the feedback loop (feedback.go) wired to anything?** It has persist/load logic for a `feedback.json` sidecar file, but it's unclear whether any CLI flag actually calls `RecordFeedback` or whether the adjusted weights are consumed by the scoring pipeline.

4. **How much of label_health.go is used by the UI?** The UI imports analysis and has `label_dashboard.go` and `flow_matrix.go`, but the 2000+ lines in label_health.go may significantly exceed what's actually rendered.

5. **Is `ParallelGain` (advanced_insights) ever going to be implemented?** It references bv-129 and has been a placeholder since creation. Consider removing the placeholder or documenting it as deliberately deferred.

6. **The `buildUnblocksMap` / `computeUnblocks` semantic duplication**: Should one be refactored to call the other? Currently a change to unblock semantics must be applied in two places.

7. **gonum dependency weight**: gonum is a large dependency pulled in for graph algorithms. The codebase has already reimplemented betweenness (ApproxBetweenness), k-core, articulation points, and the graph structure itself (compactDirectedGraph). The remaining gonum usage is PageRank, exact Betweenness, HITS, and topo.Sort/TarjanSCC. Is it worth considering eliminating the gonum dependency entirely?
