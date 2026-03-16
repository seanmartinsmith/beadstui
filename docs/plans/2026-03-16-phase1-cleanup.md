# Phase 1: Mechanical Cleanup - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove ~1.5k LOC of dead code, ~7.5k LOC of dead Rust artifacts, fix stale `bv` naming in ~15 locations, and rename 327 `bv` references across 42 E2E test files.

**Architecture:** Pure deletion and renaming - no behavior changes. Partitioned into 8 independent tasks by file ownership so they can run as parallel agents without conflicts. Each task owns its files exclusively.

**Tech Stack:** Go 1.25, no new dependencies (only removing dead code)

**Parent doc:** `docs/plans/2026-03-16-post-audit-roadmap.md` (Phase 1)
**Audit reports:** `docs/audit/team-*.md` (source of truth for line numbers and justifications)

---

## Pre-flight

Before launching any tasks, the orchestrator must verify:

```bash
cd C:/Users/sms/System/tools/bt
go build ./cmd/bt/           # must pass
go test ./... 2>&1 | tail -5 # must show 0 failures
go vet ./...                 # must be clean
```

## Task Partitioning

Tasks are partitioned by file ownership. No two tasks touch the same file.

| Task | Domain | Files Owned | Parallel-Safe |
|------|--------|-------------|---------------|
| 1 | internal/datasource | watch.go, diff.go + their tests | Yes |
| 2 | pkg/analysis | triage.go, priority.go, advanced_insights.go + their tests | Yes |
| 3 | pkg/ui (dead code + naming) | model.go, flow_matrix.go, graph.go, tree.go, tutorial_content.go | Yes |
| 4 | pkg/export (dead code + naming) | markdown.go, sqlite_export.go, cloudflare.go, viewer_embed.go, all pkg/export/*.go doc comments | Yes |
| 5 | pkg/agents + pkg/correlation + pkg/cass | detect.go, file.go, orphan.go, cass/correlation.go | Yes |
| 6 | cmd/bt + pkg/loader | main.go, gitignore.go | Yes |
| 7 | Root files + scripts + config | Makefile, bv-graph-wasm/, flake.nix, install.sh, install.ps1, scripts/*.sh | Yes |
| 8 | tests/e2e | All 46 *_test.go files in tests/e2e/ | Yes |

---

### Task 1: internal/datasource - Delete Dead Files

**Files:**
- Delete: `internal/datasource/watch.go`
- Delete: `internal/datasource/diff.go`
- Check: all other `internal/datasource/*.go` files for imports of deleted types

**Step 1: Verify no external callers of watch.go exports**

Search the entire codebase for `SourceWatcher`, `AutoRefreshManager`, `NewSourceWatcher`, `NewAutoRefreshManager`, `ForceRefresh` outside of watch.go itself. Expect zero production callers (test callers may exist - delete those too).

**Step 2: Verify no external callers of diff.go exports**

Search for `SourceDiff`, `DetectInconsistencies`, `CompareSources`, `CheckAllSourcesConsistent`, `InconsistencyReport`, `GenerateInconsistencyReport` outside of diff.go. Expect zero callers.

**Step 3: Check fsnotify import status**

Search `internal/datasource/*.go` (excluding watch.go) for `fsnotify`. If no other file imports it, removing watch.go cleanly removes fsnotify from this package. Note: `pkg/watcher/` also uses fsnotify, so the go.mod dependency stays.

**Step 4: Delete the files**

Delete `internal/datasource/watch.go` and `internal/datasource/diff.go`. If any test files reference their exports, update or delete those tests too.

**Step 5: Verify**

```bash
go build ./cmd/bt/ && go test ./internal/datasource/... && go vet ./internal/datasource/...
```

**Step 6: Commit**

```bash
git add internal/datasource/
git commit -m "refactor: remove dead datasource watch.go and diff.go (624 LOC)

SourceWatcher/AutoRefreshManager (watch.go) were never instantiated.
SourceDiff/InconsistencyReport (diff.go) were never called outside their own file.
TUI uses Dolt poll loop for change detection, not file watchers."
```

---

### Task 2: pkg/analysis - Delete Deprecated Functions

**Files:**
- Modify: `pkg/analysis/triage.go` - delete `computeCounts`, `buildBlockersToClear`, `ComputeTriageScores`, `ComputeTriageScoresWithOptions`, `GetTopTriageScores`
- Modify: `pkg/analysis/priority.go` - delete `ComputeImpactScore`, `TopImpactScores`
- Modify: `pkg/analysis/advanced_insights.go` - delete `ParallelGain` placeholder
- Modify: test files that call any of the above

**Step 1: For each function, search for callers**

Search the entire codebase (not just pkg/analysis/) for each function name. The audit said "no production callers" but some may have test callers. List all callers found.

**Step 2: Delete the functions**

Remove each function from its source file. For any test callers found in Step 1, delete those test functions too. Do NOT delete test functions that test other (live) code in the same test file - only delete tests that specifically exercise the dead functions.

**Step 3: Verify**

```bash
go build ./cmd/bt/ && go test ./pkg/analysis/... && go vet ./pkg/analysis/...
```

**Step 4: Commit**

```bash
git add pkg/analysis/
git commit -m "refactor: remove deprecated analysis functions (~200 LOC)

Removed: computeCounts, buildBlockersToClear (deprecated, superseded by
WithContext variants), ComputeImpactScore, TopImpactScores (unused wrappers),
ComputeTriageScores variants (standalone wrappers, pipeline uses internal
path), GetTopTriageScores (unused), ParallelGain (permanent placeholder)."
```

---

### Task 3: pkg/ui - Dead Code + Stale Naming

**Files:**
- Modify: `pkg/ui/model.go` - delete `ReadyTimeoutMsg`/`ReadyTimeoutCmd`, fix "Quit bv?" -> "Quit bt?"
- Modify: `pkg/ui/flow_matrix.go` - delete `FlowMatrixView` (after confirming dead)
- Modify: `pkg/ui/graph.go` - delete `ensureVisible()`, `ScrollLeft()`, `ScrollRight()`
- Modify: `pkg/ui/tree.go` - delete `TreeModeBlocking`
- Modify: `pkg/ui/tutorial_content.go` - fix `bv`/`br` references -> `bt`/`bd`
- Modify: any test files that reference deleted items

**Step 1: Confirm FlowMatrixView is dead**

Search entire codebase for `FlowMatrixView`. If there are callers outside flow_matrix.go, do NOT delete it - note the callers instead. If zero external callers, proceed with deletion.

**Step 2: Delete dead code from model.go**

Remove `ReadyTimeoutMsg` type, `ReadyTimeoutCmd` function, and any handler case for `ReadyTimeoutMsg` in the Update switch. Search for references first to confirm nothing uses them.

**Step 3: Fix "Quit bv?" in model.go**

Find the quit confirmation text (around line 4779) and change "bv" to "bt".

**Step 4: Delete empty stubs from graph.go**

Remove `ensureVisible()`, `ScrollLeft()`, `ScrollRight()` methods on GraphModel. These are empty method bodies.

**Step 5: Delete TreeModeBlocking from tree.go**

Remove the `TreeModeBlocking` constant. Verify the `mode` field on TreeModel is still valid with only `TreeModeHierarchy`.

**Step 6: Fix tutorial_content.go naming**

Read the file and replace CLI references: `bv` -> `bt`, `br` -> `bd`. Be careful to only change CLI command references, not words that happen to contain those letters. Look for patterns like "bv --", "bv brings", "running bv", "br update", "br show".

**Step 7: Clean up tests**

Delete any test functions that test the removed items (ReadyTimeoutMsg, FlowMatrixView, empty stubs, TreeModeBlocking).

**Step 8: Verify**

```bash
go build ./cmd/bt/ && go test ./pkg/ui/... && go vet ./pkg/ui/...
```

**Step 9: Commit**

```bash
git add pkg/ui/
git commit -m "refactor: remove dead UI code + fix stale bv naming in pkg/ui

Dead code removed: ReadyTimeoutMsg/Cmd (unused), FlowMatrixView (legacy),
empty GraphModel stubs, TreeModeBlocking (never set).
Naming fixed: 'Quit bv?' -> 'Quit bt?', tutorial text bv/br -> bt/bd."
```

---

### Task 4: pkg/export - Dead Code + Stale Naming

**Files:**
- Modify: `pkg/export/markdown.go` - delete `GeneratePriorityBrief`
- Modify: `pkg/export/sqlite_export.go` - delete `stringSliceContains`
- Modify: `pkg/export/cloudflare.go` - delete `DeployToCloudflarePages`
- Modify: `pkg/export/viewer_embed.go` - delete `AddGitHubWorkflowToBundle`
- Modify: all `pkg/export/*.go` files - fix "bv" in package doc comments -> "bt"
- Modify: any test files that reference deleted items

**Step 1: For each function, search for callers**

Search codebase for `GeneratePriorityBrief` (not `GeneratePriorityBriefFromTriageJSON` - that one stays), `stringSliceContains`, `DeployToCloudflarePages` (not `DeployToCloudflareWithAutoCreate`), `AddGitHubWorkflowToBundle`.

**Step 2: Delete the functions and their tests**

Remove each dead function. Remove test functions that test only the dead functions.

**Step 3: Fix package doc comments**

Read each .go file in pkg/export/. Find package-level doc comments that say "bv" and change to "bt". These are typically the `// Package export provides...` lines at the top of files.

**Step 4: Verify**

```bash
go build ./cmd/bt/ && go test ./pkg/export/... && go vet ./pkg/export/...
```

**Step 5: Commit**

```bash
git add pkg/export/
git commit -m "refactor: remove dead export functions + fix bv naming in doc comments

Removed: GeneratePriorityBrief (stub), stringSliceContains (unused),
DeployToCloudflarePages (superseded by WithAutoCreate), AddGitHubWorkflowToBundle
(redundant wrapper). Fixed package doc comments bv -> bt."
```

---

### Task 5: pkg/agents + pkg/correlation + pkg/cass - Dead Code + Naming

**Files:**
- Modify: `pkg/agents/detect.go` - delete `AgentFileExists()`
- Modify: `pkg/agents/file.go` - change `.bv-atomic-*` temp prefix to `.bt-atomic-*`
- Modify: `pkg/correlation/orphan.go` - add `bt-` to regex patterns alongside `bv-`
- Modify: `pkg/cass/correlation.go` - add `bt-` to bead ID regex alongside `bv-`
- Modify: any test files that reference deleted/changed items

**Step 1: Delete AgentFileExists and its test callers**

Search for `AgentFileExists` across the codebase. Remove the function from detect.go. Remove any test functions that call it (these are the only callers per the audit).

**Step 2: Rename temp file prefix in file.go**

Find `.bv-atomic-` in pkg/agents/file.go and change to `.bt-atomic-`. This is purely internal (temp file naming during atomic writes).

**Step 3: Update orphan.go regex patterns**

In `pkg/correlation/orphan.go`:
- Line 40: `orphanMessagePatterns` - update the bead ID pattern to match both `bv-` and `bt-` prefixes (e.g., `\b(?:bv|bt)-[a-z0-9]+\b`)
- Line 45: `orphanBeadIDPattern` - same, match both prefixes
- Line 375: string construction - handle both `bv-` and `bt-` prefixed IDs

**Step 4: Update cass/correlation.go regex**

In `pkg/cass/correlation.go` line 626: update the bead ID regex to match both `bv-` and `bt-` prefixes.

**Step 5: Update tests**

If any tests for orphan detection or cass correlation validate specific regex behavior, update them to expect the new pattern. Add a test case with a `bt-` prefixed ID if one doesn't exist.

**Step 6: Verify**

```bash
go build ./cmd/bt/ && go test ./pkg/agents/... ./pkg/correlation/... ./pkg/cass/... && go vet ./pkg/agents/... ./pkg/correlation/... ./pkg/cass/...
```

**Step 7: Commit**

```bash
git add pkg/agents/ pkg/correlation/ pkg/cass/
git commit -m "refactor: fix stale bv naming in agents, correlation, cass

Removed AgentFileExists (dead, test-only callers). Renamed .bv-atomic
temp prefix to .bt-atomic. Updated orphan detection and cass correlation
regexes to match both bv- and bt- bead ID prefixes."
```

---

### Task 6: cmd/bt + pkg/loader - Dead Code + Naming

**Files:**
- Modify: `cmd/bt/main.go` - delete `RobotMeta`, build tag guards (lines 241-260), `medianMinutes`; rename `EnsureBVInGitignore` call
- Modify: `pkg/loader/gitignore.go` - rename `EnsureBVInGitignore` to `EnsureBTInGitignore`
- Modify: any test files that reference the renamed function

**Step 1: Delete dead code from main.go**

- Delete the `RobotMeta` struct definition (search for `type RobotMeta struct`)
- Delete the build tag guard block (lines ~241-260, the `_ = exportPages` etc. assignments)
- Delete or fix `medianMinutes` dead variable (search for `medianMinutes` - either delete the declaration or wire it in if there's an obvious use site; the audit says it's dead so delete + remove the `_ = medianMinutes` suppression)

**Step 2: Rename EnsureBVInGitignore**

In `pkg/loader/gitignore.go`: rename the function from `EnsureBVInGitignore` to `EnsureBTInGitignore`. Update the doc comment and any internal comments that say "bv".

In `cmd/bt/main.go`: find all call sites of `EnsureBVInGitignore` and update to `EnsureBTInGitignore`.

Search the rest of the codebase for any other callers of `EnsureBVInGitignore`.

**Step 3: Update tests**

If `pkg/loader/` has tests that reference `EnsureBVInGitignore` by name, update them.

**Step 4: Verify**

```bash
go build ./cmd/bt/ && go test ./cmd/bt/... ./pkg/loader/... && go vet ./cmd/bt/... ./pkg/loader/...
```

**Step 5: Commit**

```bash
git add cmd/bt/ pkg/loader/
git commit -m "refactor: clean dead code from main.go + rename EnsureBVInGitignore

Removed: RobotMeta (unused struct), build tag guard block (no build tags
exist), medianMinutes (dead variable). Renamed EnsureBVInGitignore ->
EnsureBTInGitignore in pkg/loader and its call site in main.go."
```

---

### Task 7: Root Files + Scripts + Config

**Files:**
- Delete: `Makefile`
- Delete: `bv-graph-wasm/` (entire directory)
- Modify: `flake.nix` - version "0.14.4" -> "0.0.1"
- Modify: `install.sh` - Go version minimum 1.21 -> 1.25
- Modify: `install.ps1` - Go version minimum 1.21 -> 1.25
- Modify: `scripts/coverage.sh` - fix "bv" in comment header
- Modify: `scripts/benchmark.sh` - fix "bv" in comment header

**Step 1: Delete Makefile**

Remove the root `Makefile`. It references `bv`/`cmd/bv` which don't exist.

**Step 2: Delete bv-graph-wasm/**

Remove the entire `bv-graph-wasm/` directory. This is ~7.5k LOC of Rust that isn't built by CI and whose algorithms are reimplemented in Go. The pre-compiled WASM artifacts in `pkg/export/viewer_assets/vendor/` are unaffected.

Verify the viewer assets are untouched:
```bash
ls pkg/export/viewer_assets/vendor/bv_graph.js pkg/export/viewer_assets/vendor/bv_graph_bg.wasm
```

**Step 3: Fix flake.nix version**

Find `version = "0.14.4"` in flake.nix and change to `version = "0.0.1"`.

**Step 4: Fix install script Go versions**

In `install.sh`: find the minimum Go version check (likely a comparison against "1.21") and update to "1.25".

In `install.ps1`: same - find Go version minimum and update to "1.25".

**Step 5: Fix script comment headers**

In `scripts/coverage.sh`: fix comment header referencing "bv" to "bt".
In `scripts/benchmark.sh`: same.
Check `scripts/benchmark_quick.sh` and `scripts/benchmark_compare.sh` for similar references.

**Step 6: Verify**

```bash
go build ./cmd/bt/ && go test ./... 2>&1 | tail -5
```

**Step 7: Commit**

```bash
git add -A Makefile bv-graph-wasm/ flake.nix install.sh install.ps1 scripts/
git commit -m "refactor: remove Makefile + bv-graph-wasm, fix config versions

Deleted broken Makefile (referenced bv/cmd/bv). Deleted bv-graph-wasm/
Rust source (~7.5k LOC, not built by CI, algorithms in Go, compiled
WASM artifacts in viewer_assets/ unaffected). Fixed flake.nix version
0.14.4 -> 0.0.1. Updated install scripts Go minimum 1.21 -> 1.25.
Fixed bv -> bt in script comment headers."
```

---

### Task 8: E2E Test Naming (327 renames across 46 files)

**Files:**
- Modify: all `tests/e2e/*_test.go` files (46 files)

**Step 1: Identify all rename targets**

Search `tests/e2e/` for these patterns and build the full list:
- `bvBinaryPath` -> `btBinaryPath`
- `bvBinaryDir` -> `btBinaryDir`
- `buildBvBinary` -> `buildBtBinary`
- `buildBvOnce` -> `buildBtOnce`
- `runBVCommand` -> `runBTCommand`
- `runBVCommandJSON` -> `runBTCommandJSON`
- Any other `bv`/`BV`/`Bv` references in variable names, function names, and comments that refer to the binary (NOT bead IDs like `bv-123` which are valid upstream beads issue ID prefixes)

**Step 2: Perform the renames**

For each file in `tests/e2e/`, apply all renames. Use replace-all operations for efficiency. Be careful NOT to rename:
- Bead issue IDs like `bv-123`, `bv-xxx` (these are valid beads ID prefixes in test fixtures)
- The string `"bv"` in JSONL fixture data that represents beads issue content
- References to `bv-graph-wasm` (though this directory is being deleted in Task 7, test references to its name in strings should be left for now)

**Step 3: Verify common_test.go compiles**

`tests/e2e/common_test.go` defines the shared helpers (`buildBtBinary`, `runBTCommand`, etc.). This is the most critical file - if the rename is consistent here, all other files should follow.

```bash
go build ./cmd/bt/ && go test -run TestNothing ./tests/e2e/... 2>&1 | head -5
```

(Use `-run TestNothing` to just compile without running the full E2E suite which takes ~7 minutes)

**Step 4: Run full E2E tests**

```bash
go test ./tests/e2e/... -timeout 600s 2>&1 | tail -20
```

This takes ~7 minutes. All tests should pass.

**Step 5: Commit**

```bash
git add tests/e2e/
git commit -m "refactor: rename bv -> bt in E2E test infrastructure (327 renames)

Renamed internal variable/function names in 46 E2E test files:
bvBinaryPath -> btBinaryPath, buildBvBinary -> buildBtBinary,
runBVCommand -> runBTCommand, etc. Test fixture data with bv- bead
ID prefixes intentionally preserved (valid upstream beads IDs)."
```

---

## Post-flight

After all 8 tasks complete:

```bash
# Full verification
go build ./cmd/bt/
go vet ./...
go test ./... -timeout 600s

# Sanity check: no stale bv references in production code
# (test fixtures and bead IDs like bv-123 are expected)
grep -r "EnsureBV" --include="*.go" .
grep -r "Quit bv" --include="*.go" .
grep -r '"bv-atomic' --include="*.go" .
```

If all passes, the orchestrator creates a final merge commit (if using worktrees) or verifies the commit log, then pushes:

```bash
git push
```

**Phase 1 is complete when**: all listed items are deleted/renamed, `go build` passes, `go test ./...` shows 0 failures, and `go vet ./...` is clean.
