# Audit Report: Supporting Packages

**Team**: 8a
**Scope**: 16 pkg/ directories not covered by other teams (correlation, hooks, loader, cass, model, recipe, search, metrics, baseline, drift, instance, watcher, workspace, updater, debug, util), plus version, testutil, testutil/proptest
**Lines scanned**: ~9,200 production LOC across 19 packages (estimated from file reads)

## Architecture Summary

These 19 packages form the **supporting infrastructure layer** of bt. They divide into several functional clusters:

**Data layer** (model, loader): `pkg/model/` defines the canonical Issue/Sprint/Dependency types used by nearly every other package (~140 importers). `pkg/loader/` handles JSONL parsing, file discovery, git worktree support, git history loading, sprint I/O, and object pooling. These two are the most depended-on packages in this scope.

**Git correlation engine** (correlation): By far the largest package at ~4,500+ production LOC across 18 source files. It correlates beads (issues) with git history using multiple strategies: co-commit detection, explicit ID matching in commit messages, temporal/author correlation, and orphan commit detection. It includes a file-to-bead reverse index, co-change matrix, impact network (graph clustering of bead relationships), causal chain analysis, LRU caching, incremental updates, and a feedback store. This is a sophisticated analytics engine that runs git commands as subprocesses.

**External integrations and CI-oriented features** (cass, hooks, drift, baseline, updater): Cass provides optional integration with the "cass" semantic code search tool. Hooks run pre/post-export shell commands. Drift compares current graph metrics against a saved baseline to detect regressions. Updater does self-update from GitHub releases. These are well-isolated with clear boundaries.

**Operational infrastructure** (metrics, debug, instance, watcher, workspace, search, recipe, version, util/topk, testutil): Metrics provides atomic timing/cache counters. Debug provides conditional stderr logging. Instance implements PID-based lock files to detect multiple bt instances. Watcher monitors JSONL files using fsnotify with polling fallback. Workspace supports multi-repo monorepo aggregation. Search implements hybrid scoring with hash-based embeddings. Recipe provides reusable view configurations loaded from YAML. Version resolves the application version. Topk is a generic heap-based top-K collector. Testutil provides deterministic graph fixture generators and test assertions.

## Feature Inventory

| Feature | Location | LOC (est) | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----------|-----------------|--------|------------|-------|
| Issue/Sprint/Dependency types | pkg/model/types.go | ~327 | Yes | Yes | Yes | Core data model, 140+ importers |
| JSONL loader + file discovery | pkg/loader/loader.go | ~459 | Yes (reads .beads/) | Yes (8 test files) | Yes | BOM stripping, pooling, git worktree support |
| Git history loader | pkg/loader/git.go | ~390 | Yes | Yes | Yes | Load issues from any git revision with caching |
| Sprint I/O | pkg/loader/sprint.go | ~159 | Yes | Yes | Yes | Atomic write via temp+rename |
| Issue object pool | pkg/loader/pool.go | ~131 | Yes | Partial | Yes | sync.Pool for reduced GC pressure |
| Gitignore management | pkg/loader/gitignore.go | ~137 | Yes | Yes | Yes | Auto-adds .bt/ to .gitignore |
| Git correlation engine | pkg/correlation/correlator.go | ~332 | N/A (reads git) | Yes | Yes | Orchestrates all correlation strategies |
| Lifecycle event extractor | pkg/correlation/extractor.go | ~487 | N/A (reads git) | Yes | Yes | Parses git log diffs for bead status changes |
| Co-commit extraction | pkg/correlation/cocommit.go | ~419 | N/A | Yes | Yes | Files changed alongside bead updates |
| Explicit ID matching | pkg/correlation/explicit.go | ~372 | N/A | Yes | Yes | Finds bead IDs in commit messages |
| Temporal correlation | pkg/correlation/temporal.go | ~333 | N/A | Yes | Yes | Author+time window matching |
| File-to-bead index | pkg/correlation/file_index.go | ~791 | N/A | Yes | Yes | Reverse index, co-change matrix, impact analysis |
| Causal chain analysis | pkg/correlation/causality.go | ~452 | N/A | Yes | Yes | Event chains with blocking period analysis |
| Impact network | pkg/correlation/network.go | ~816 | N/A | Yes | Yes | Graph of bead relationships with clustering |
| Orphan commit detection | pkg/correlation/orphan.go | ~498 | N/A | Yes | Yes | Smart heuristics for unlinked commits |
| Related work discovery | pkg/correlation/related.go | ~596 | N/A | Yes | Yes | File overlap, commit overlap, concurrency |
| Reverse commit lookup | pkg/correlation/reverse.go | ~373 | N/A | Yes | Yes | Commit SHA -> bead IDs |
| Confidence scorer | pkg/correlation/scorer.go | ~472 | N/A | Yes | Yes | Multi-signal confidence combining |
| Streaming extractor | pkg/correlation/stream.go | ~522 | N/A | Yes | Yes | Memory-efficient git log parsing |
| Incremental updates | pkg/correlation/incremental.go | ~521 | N/A | Yes | Yes | Delta updates to avoid full repo scans |
| History cache | pkg/correlation/cache.go | ~402 | N/A | Yes | Yes | LRU cache with HEAD invalidation |
| Feedback store | pkg/correlation/feedback.go | ~252 | N/A | Yes | Yes | JSONL-backed correlation feedback |
| Correlation types | pkg/correlation/types.go | ~233 | N/A | Yes | Yes | Comprehensive type system |
| Cass detector | pkg/cass/detector.go | ~218 | Yes | Yes | Yes | Silent degradation when cass absent |
| Cass search client | pkg/cass/search.go | ~276 | Yes | Yes | Yes | Semaphore-limited, timeout-protected |
| Cass result cache | pkg/cass/cache.go | ~253 | Yes | Yes | Yes | LRU with TTL eviction |
| Cass correlation | pkg/cass/correlation.go | ~647 | Yes | Yes | Yes | Multi-strategy bead-to-session matching |
| Hook config loader | pkg/hooks/config.go | ~246 | Yes | Yes | Yes | YAML config with duration parsing |
| Hook executor | pkg/hooks/executor.go | ~243 | Yes | Yes (platform-specific) | Yes | Timeout, env expansion, shell dispatch |
| Recipe types | pkg/recipe/types.go | ~120 | Yes | Yes | Yes | Filter/sort/view/export configs |
| Recipe loader | pkg/recipe/loader.go | ~231 | Yes | Yes | Yes | 3-layer merge: builtin < user < project |
| Hybrid search config | pkg/search/config.go | ~151 | Yes | Yes | Yes | Env-based config for search mode/weights |
| Timing metrics | pkg/metrics/timing.go | ~248 | Yes | Yes | Yes | Atomic CAS-based min/max tracking |
| Cache metrics | pkg/metrics/cache.go | ~174 | Yes | Yes | Yes | Hit/miss/rate tracking + memory stats |
| Baseline snapshots | pkg/baseline/baseline.go | ~232 | Yes | Yes | Yes | Save/load graph metric snapshots |
| Drift detection | pkg/drift/drift.go | ~613 | Yes | Yes | Yes | 8 alert types with configurable thresholds |
| Drift config | pkg/drift/config.go | ~336 | Yes | Yes | Yes | Per-label staleness overrides |
| Instance locking | pkg/instance/lock.go | ~240 | Yes | Yes | Yes | PID-based with stale lock recovery |
| File watcher | pkg/watcher/watcher.go | ~377 | Yes | Yes | Yes | fsnotify + polling fallback, FS detection |
| Debouncer | pkg/watcher/debouncer.go | ~50 (est) | Yes | Yes | Yes | Timer-based change coalescing |
| FS type detection | pkg/watcher/fsdetect*.go | ~100 (est) | Yes | Yes | Yes | Platform-specific (darwin/linux/windows) |
| Workspace config | pkg/workspace/types.go | ~368 | Yes | Yes | Yes | Multi-repo monorepo support |
| Workspace loader | pkg/workspace/loader.go | ~280 | Yes | Yes | Yes | Parallel loading with errgroup |
| Self-updater | pkg/updater/updater.go | ~753 | Yes | Yes (6 test files) | Yes | GitHub release, checksum, rollback |
| Debug logging | pkg/debug/debug.go | ~170 | Yes | Yes | Yes | Zero-overhead when disabled |
| Top-K collector | pkg/util/topk/topk.go | ~192 | Yes | Yes | Yes | Generic heap, O(n log k) |
| Version resolver | pkg/version/version.go | ~59 | Yes | N/A | Yes | ldflags > build info > fallback |
| Test fixture generator | pkg/testutil/generator.go | ~622 | Yes | Yes | Yes | 11 graph topologies + issue conversion |
| Test assertions | pkg/testutil/assertions.go | ~381 | Yes | N/A | Yes | Golden files, cycle detection, status counts |

## Dependencies

- **Depends on**: `pkg/model` is the foundational type package imported by most others. `pkg/loader` is imported by workspace, datasource, and UI layers. `pkg/correlation` imports `pkg/model` (via network.go). `pkg/drift` imports `pkg/analysis` and `pkg/baseline`. `pkg/cass` imports `pkg/model`. `pkg/search` depends on `pkg/util/topk` (vector_index.go). External deps: `fsnotify` (watcher), `gopkg.in/yaml.v3` (hooks, recipe, drift, workspace), `golang.org/x/sync/errgroup` (workspace), `goccy/go-json` (loader).
- **Depended on by**:
  - `pkg/model`: ~140 files (most-imported package in the codebase)
  - `pkg/loader`: ~18 files (cmd/bt, pkg/ui, internal/datasource, pkg/workspace)
  - `pkg/correlation`: ~6 files (cmd/bt, pkg/ui, pkg/export, pkg/analysis)
  - `pkg/cass`: ~5 files (pkg/ui)
  - `pkg/search`: ~7 files (cmd/bt, pkg/ui, tests/e2e)
  - `pkg/recipe`: ~13 files (cmd/bt, pkg/ui)
  - `pkg/metrics`: ~1 file (cmd/bt)
  - `pkg/debug`: ~3 files (pkg/ui)
  - `pkg/version`: ~3 files (cmd/bt, pkg/ui, pkg/updater)
  - `pkg/updater`: ~3 files (cmd/bt, pkg/ui)
  - `pkg/hooks`: ~1 file (cmd/bt)
  - `pkg/baseline`: ~4 files (cmd/bt, pkg/ui, pkg/drift)
  - `pkg/drift`: ~2 files (cmd/bt, pkg/ui)
  - `pkg/instance`: ~1 file (pkg/ui)
  - `pkg/watcher`: ~4 files (cmd/bt, pkg/ui)
  - `pkg/workspace`: ~3 files (cmd/bt)
  - `pkg/util/topk`: ~1 file (pkg/search)
  - `pkg/testutil`: ~9 files (test files across analysis, export, loader, ui)

## Dead Code Candidates

1. **`pkg/correlation/orphan.go` line 45**: `orphanBeadIDPattern` uses `bv-` prefix pattern instead of `bt-`. This is a stale rename artifact - it will never match bt-style bead IDs. Same issue on line 375 where it constructs `"bv-" + strings.ToLower(match[1])`.

2. **`pkg/correlation/orphan.go` line 40**: `orphanMessagePatterns` includes `\bbv-[a-z0-9]+\b` pattern (bv-xxx) with weight 25 - same stale prefix issue.

3. **`pkg/loader/gitignore.go`**: Function name `EnsureBVInGitignore` still uses `BV` naming (comment references "bv" throughout). The actual pattern it writes is correct (`.bt/`), and the pattern matching array correctly checks for `.bt` variants, but the function/file naming is a rename leftover.

4. **`pkg/correlation/incremental.go` line 168**: `NewExtractor(ic.cache.repoPath, "")` passes empty string as beads file path - the empty string parameter is effectively a no-op but is unnecessary.

5. **`pkg/metrics/`**: The `metrics` package is only imported by `cmd/bt/main.go`. The global metric variables (CycleDetection, TopologicalSort, etc.) appear to be intended for use across the codebase but actual `metrics.Timer()` calls need verification (they may be in pkg/analysis or pkg/ui which other teams cover).

6. **`pkg/search/config.go`**: Only `ProviderHash` embedder is implemented. `ProviderPythonSentenceTransformers` and `ProviderOpenAI` return explicit "not implemented" errors. These are placeholder extension points.

7. **`pkg/instance/lock.go` line 37**: `LockFileName` is still `.bv.lock` (not `.bt.lock`). This is a rename leftover.

## Notable Findings

1. **pkg/correlation is massive**: At ~4,500+ LOC across 18 source files with 19 test files, this is the largest package in this scope by a wide margin. It implements a sophisticated git history correlation engine with multiple strategies, caching layers, incremental updates, impact analysis, network clustering, orphan detection, and causal chain analysis. The complexity appears justified for its purpose (linking code commits to issue lifecycle events), but it operates entirely on JSONL files via git - it does NOT query Dolt. Given that upstream beads has removed JSONL backends in favor of Dolt-only storage, the entire correlation engine's data source (git diffs of `.beads/beads.jsonl`) may become irrelevant unless beads continues to commit JSONL snapshots alongside Dolt.

2. **Stale `bv` references in correlation/orphan.go**: The orphan detector regex patterns still look for `bv-` prefixed bead IDs instead of `bt-`. This means orphan detection for bt-style IDs will not match via the bead ID pattern, reducing detection accuracy.

3. **pkg/model is the gravity well**: With ~140 importers, any change to `pkg/model/types.go` has extreme blast radius. The type definitions are clean and well-validated, but the `IssueType.IsValid()` method accepts any non-empty string (deliberately, for extensibility with Gastown types). The `DependencyType.IsBlocking()` returns true for empty strings for backward compatibility.

4. **pkg/loader has production-quality JSONL parsing**: UTF-8 BOM stripping, 10MB line buffer, object pooling via sync.Pool, git worktree detection, file priority ordering matching bd's canonical naming. This is battle-tested infrastructure.

5. **pkg/cass is well-designed for optional integration**: Silent degradation pattern (returns empty results when cass not installed), semaphore-limited concurrent searches, comprehensive doc.go. The correlation engine uses multi-strategy scoring (ID mention > keywords > timestamp). The regex for bead ID mentions in `correlation.go` line 626 uses `bv-` prefix pattern - another stale rename.

6. **pkg/updater has security awareness**: Strips Authorization headers on non-GitHub redirects, respects rate limits gracefully, handles dev/dirty/nightly build versions to avoid false update prompts. The version comparison logic is thorough (262 lines for `compareVersions` alone).

7. **pkg/drift is CI-ready**: Returns exit codes (0/1/2) suitable for CI gates. Per-label staleness overrides (bv-167) and alert disabling provide fine-grained control. Depends on `pkg/analysis` for blocking cascade detection.

8. **pkg/instance uses goto on Windows**: `lock.go` line 197 uses `goto verify` as part of a Windows-specific fallback for atomic file replacement. This is functional but unconventional.

9. **pkg/watcher has platform-specific FS detection**: Separate files for darwin, linux, windows, and "other" to detect remote/network filesystems that don't support fsnotify well. Falls back to polling automatically.

10. **pkg/workspace supports monorepos**: Parallel loading of multiple repos with ID namespacing, cross-repo dependency resolution, and configurable discovery patterns. This is forward-looking infrastructure that would be valuable for multi-project beads setups.

11. **pkg/testutil is a hidden gem**: Provides 11 graph topology generators (chain, star, diamond, cycle, tree, disconnected, complete, random DAG, bipartite, ladder, self-loop) that convert to model.Issue slices. Used by 9 test files across the codebase. The golden file helper normalizes line endings cross-platform.

## Questions for Synthesis

1. **Correlation engine viability**: With beads moving to Dolt-only storage, does the correlation engine's reliance on `git log -p` of JSONL files still work? Does beads still commit JSONL snapshots to git, or is Dolt the sole storage? If JSONL is no longer committed, the entire `pkg/correlation/` package (~4,500 LOC) becomes dead code that should be either adapted to query Dolt or removed.

2. **Stale bv- patterns in orphan/cass**: The `bv-` regex patterns in `pkg/correlation/orphan.go` (lines 40, 45) and `pkg/cass/correlation.go` (line 626) are rename leftovers. Should these be updated to `bt-` patterns? The explicit matcher in `pkg/correlation/explicit.go` correctly includes `bt-` patterns (line 36), so the inconsistency is specifically in orphan detection and cass correlation.

3. **Instance lock filename**: `pkg/instance/lock.go` still uses `.bv.lock` as the lock file name. Is this intentional for backward compatibility, or should it be `.bt.lock`?

4. **Search package scope**: This audit only covers `pkg/search/config.go`. The full search package (15 source files) includes vector indexing, hybrid scoring, hash embeddings, lexical boosting, etc. Is this covered by another team, or should Team 8a have audited the full search package?

5. **Metrics usage**: `pkg/metrics` is only directly imported by `cmd/bt/main.go`. The global timing variables suggest wider usage was intended. Are `metrics.Timer()` calls present in pkg/analysis and pkg/ui code (covered by other teams)?

6. **pkg/loader gitignore naming**: The `EnsureBVInGitignore` function name and surrounding comments still reference "bv". The actual behavior is correct (writes `.bt/`). Low priority but part of the rename cleanup.
