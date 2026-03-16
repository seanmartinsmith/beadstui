# Post-Audit Roadmap

**Date**: 2026-03-16
**Parent**: docs/audit/architecture-map.md (Session 16 audit findings)
**Status**: Design approved, Phase 1 ready for planning

## Context

Session 16 executed a 9-team codebase audit scanning ~190k lines. The audit produced detailed reports for every domain plus an architecture map. This document captures the full roadmap for acting on those findings, organized into three tiers based on decision dependencies.

## Tier 1: Mechanical Cleanup (no strategic ambiguity)

Safe to execute now. No beads ecosystem context needed. All items are deletions, renames, or restructuring with clear right answers.

### Phase 1 - Dead Code, Naming, Artifacts (parallel session)

**Dead code removal (~1.5k LOC):**

Note: deleting functions that are called from tests means also deleting or updating those tests. Executing agents must handle both the function and its test callers.

- `internal/datasource/watch.go` (355 LOC) - SourceWatcher/AutoRefreshManager never instantiated; pulls unused fsnotify dep. Verify no other files in datasource import fsnotify before assuming the dep is cleanly removed.
- `internal/datasource/diff.go` (269 LOC) - SourceDiff/InconsistencyReport never called outside own file
- `pkg/analysis/triage.go` - `computeCounts` and `buildBlockersToClear` (deprecated, no production callers)
- `pkg/analysis/priority.go` - `ComputeImpactScore`, `TopImpactScores` (convenience wrappers, no production callers)
- `pkg/analysis/triage.go` - `ComputeTriageScores`, `ComputeTriageScoresWithOptions`, `GetTopTriageScores` (standalone wrappers, no production callers)
- `pkg/analysis/advanced_insights.go` - `ParallelGain` placeholder (permanently "pending")
- `pkg/ui/model.go` - `ReadyTimeoutMsg`/`ReadyTimeoutCmd` (documented as unused)
- `pkg/ui/flow_matrix.go` - `FlowMatrixView` legacy function. Confirm no callers exist before deleting (Team 1a flagged as likely dead but unverified).
- `pkg/ui/graph.go` - `ensureVisible()`, `ScrollLeft()`, `ScrollRight()` empty stubs
- `pkg/ui/tree.go` - `TreeModeBlocking` (defined but never set)
- `pkg/export/markdown.go` - `GeneratePriorityBrief` (stub with placeholder text)
- `pkg/export/sqlite_export.go` - `stringSliceContains` (unused)
- `pkg/export/cloudflare.go` - `DeployToCloudflarePages` (superseded by WithAutoCreate variant)
- `pkg/export/viewer_embed.go` - `AddGitHubWorkflowToBundle` (redundant wrapper)
- `pkg/agents/detect.go` - `AgentFileExists()` (only test callers)
- `cmd/bt/main.go` - `RobotMeta` struct (defined, never used)
- `cmd/bt/main.go` - build tag guard block lines 241-260 (no build tags exist)
- `cmd/bt/main.go` - `medianMinutes` dead variable in capacity handler
- Root `Makefile` - remove entirely. `go build ./cmd/bt/` is the documented build command, goreleaser handles releases, CI doesn't use it.

**Dead artifact removal (~7.5k Rust):**
- `bv-graph-wasm/` source directory - not built by CI, algorithms reimplemented in Go, pre-compiled WASM artifacts already in `viewer_assets/vendor/`
- Keep: `pkg/export/viewer_assets/vendor/bv_graph.js` + `bv_graph_bg.wasm` (the compiled artifacts the viewer uses)

**Stale naming fixes (~15 locations):**
- `pkg/ui/tutorial_content.go` - `bv`/`br` CLI references in tutorial text -> `bt`/`bd`
- `pkg/correlation/orphan.go` lines 40, 45, 375 - regex patterns `bv-` -> include `bt-`
- `pkg/cass/correlation.go` line 626 - bead ID regex `bv-` -> include `bt-`
- `pkg/loader/gitignore.go` - `EnsureBVInGitignore` -> `EnsureBTInGitignore`
- `cmd/bt/main.go` - `EnsureBVInGitignore` call site rename
- `pkg/ui/model.go` line 4779 - "Quit bv?" -> "Quit bt?"
- `pkg/agents/file.go` - `.bv-atomic-*` temp prefix -> `.bt-atomic-*`
- `pkg/export/` - package doc comments "bv" -> "bt" (multiple files)
- `scripts/coverage.sh`, `scripts/benchmark.sh` - comment headers
- `flake.nix` - version "0.14.4" -> "0.0.1"
- `install.sh`, `install.ps1` - Go version minimum 1.21 -> 1.25

**E2E test naming (327 renames across 42 files):**
- `bvBinaryPath` -> `btBinaryPath`
- `bvBinaryDir` -> `btBinaryDir`
- `buildBvBinary` -> `buildBtBinary`
- `buildBvOnce` -> `buildBtOnce`
- `runBVCommand` -> `runBTCommand`
- `runBVCommandJSON` -> `runBTCommandJSON`
- All related comment references

**Execution strategy**: Launch parallel agents, but partition by file ownership to avoid conflicts. Multiple categories touch the same files (e.g., dead code and stale naming both touch `cmd/bt/main.go`). Agents should be scoped so no two agents edit the same file. Each agent gets:
1. Specific file list + line numbers from audit reports
2. Instruction to make targeted changes only
3. Gate: `go build ./cmd/bt/ && go test ./...` must pass after changes

**Phase 1 is complete when**: all listed items are deleted/renamed, build passes, all tests pass, and `go vet ./...` is clean.

### Phase 2 - Monolith Splitting (needs own brainstorm)

**model.go (8.3k lines)**: Team 1b identified ~12 extractable sections. Key design decisions:
- Which sections become their own files vs. stay in model.go?
- Do any types (messages, commands) deserve their own file?
- How to handle the ~90-field Model struct definition - stays in model.go but methods move?
- Should the 5 mutually exclusive view booleans become a ViewMode enum? (behavior change)

**main.go (8.1k lines)**: Team 2 identified ~2.5k lines of extractable helpers. Key design decisions:
- What stays in `cmd/bt/` as sibling files vs. becomes a new internal package?
- Should robot command dispatch use a registry pattern instead of if-else chain?
- The ~460-line `--robot-help` println block - replace with `--robot-docs` or keep both?

Both need their own brainstorm sessions to make grouping decisions before agents execute.

### Phase 3 - Investigation + Targeted Fixes (needs reading before acting)

**5 robot handlers reloading from disk**: `--robot-file-relations`, `--robot-related`, `--robot-blocker-chain`, `--robot-impact-network`, `--robot-causality` all call `datasource.LoadIssues(cwd)` instead of using pre-loaded `issues` slice. Need to verify if this is intentional (needing unfiltered data) or a bug (bypassing --as-of/--workspace/--repo/--label filters).

**Consolidation items** (moved from Phase 1 - these require choosing which implementation to keep, not just deleting):
- `pkg/export/markdown.go` duplicate `getTypeIcon`/`getTypeEmoji` - different emoji choices, pick canonical set
- `pkg/export/graph_export.go` + `mermaid_generator.go` - duplicate Mermaid generators, decide which to keep
- `pkg/export/` - 3 truncation helper variants (`truncateString`, `truncateRunes`, `truncate`), consolidate to 1
- `pkg/ui/` duplicate icon/color helpers - graph.go private helpers vs. helpers.go/theme.go public helpers use different emoji sets

**Lock file migration**: `pkg/instance/lock.go` `.bv.lock` -> `.bt.lock`. Needs migration path: if bt encounters an existing `.bv.lock`, it should recognize it (check both filenames? rename on startup?). Straight rename risks dual-instance detection failure for users upgrading.

**Legacy sync path in model.go**: Team 1b flagged `FileChangedMsg` handler (model.go:2001-2427, ~420 LOC) as dead - only activates for JSONL sources without background mode, and upstream removed JSONL. Verify no other code path sends `FileChangedMsg` before deleting.

**Duplicate ACFS workflows**: `acfs-checksums-dispatch.yml` and `notify-acfs.yml` overlap. Determine which to keep.

## Tier 2: Strategic Decisions (needs beads audit)

Cannot be decided without understanding beads itself - its current schema, mutation API, CLI surface, ecosystem plans, and where bt adds value vs. duplicates.

### Beads Audit (prerequisite)

Audit upstream beads (steveyegge/beads) to understand:
- Current schema (v6, ~50+ columns on issues table)
- Mutation API - what `bd` commands exist for creating/editing issues
- Dolt system tables available (dolt_log, dolt_diff, dolt_status) for history features
- Ecosystem direction - what's planned, what's stable, what's changing fast
- Where bt fits - what beads intentionally leaves to external tools

### Decisions Blocked on Beads Audit

| Decision | Depends On |
|----------|-----------|
| Keep/cut correlation engine (~4.5k LOC)? | Does beads still commit JSONL to git? Is there a Dolt-native history alternative? |
| Keep/cut deploy pipelines (~1.6k LOC)? | Does beads have its own publishing/sharing story? |
| Keep/cut TOON format? | Is TOON valued in the beads ecosystem? Does upstream use it? |
| Which robot commands matter? | What do beads agents actually call? Which bd commands exist? |
| SQLite reader - remove or keep? | Is backward compat with pre-v0.56.1 beads installations needed? |
| CRUD from TUI - what fields/workflow? | What's the full beads schema? What mutations does bd support? |
| History view - Dolt native? | What dolt_log/dolt_diff queries would give us issue history? |

### Output

After beads audit + decisions:
- Feature inventory with KEEP/CUT/IMPROVE verdicts
- ADR-002 skeleton (new spine document replacing ADR-001)
- Updated beads issues for new work items
- Clear scope for Tier 3

## Tier 3: Build Forward (the goal)

The actual features that make bt useful as the official beads TUI companion.

### CRUD from TUI (highest priority)
- Visual issue creation with structured form fields
- Issue editing (status, priority, description, labels, dependencies)
- Shell out to `bd` for writes, poll Dolt for changes (decided in session 13)
- Needs beads schema knowledge from Tier 2 audit

### Dolt Integration Tests (biggest coverage gap)
- The production data path has zero E2E coverage
- Need test infrastructure that can start/stop a Dolt server
- Validates the actual path users take, not just JSONL fixtures

### Beads-Informed Features
- History view powered by Dolt system tables (dolt_log, dolt_diff)
- Wasteland integration (surface wanted items/reputation from wl CLI)
- Whatever the beads audit reveals as high-value TUI enhancements

## Session Planning

| Session | Scope | Type |
|---------|-------|------|
| Next | Phase 1 execution (dead code, naming, artifacts, E2E renames) | Implementation (parallel agents) |
| Next+1 | Phase 2 brainstorm (model.go + main.go splitting design) | Brainstorm -> Plan |
| Next+2 | Phase 2 execution + Phase 3 investigation | Implementation |
| Can parallel | Beads audit (Tier 2 prerequisite) - pure research, no code changes, can run alongside Phase 2/3 | Research |
| After beads audit | Tier 2 synthesis (keep/cut/improve decisions, ADR-002) | Brainstorm -> Plan |
| After Tier 2 | Tier 3 - CRUD from TUI | Brainstorm -> Plan -> Implementation |
