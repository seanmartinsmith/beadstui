# Test Baseline - 2026-04-10

Pre-refactor test baseline captured before Phase 1 of the pkg/ui decomposition.

## Context

Phase 0.5 fixed e2e Dolt isolation (bt-hpq6): `DiscoverSharedServer()` now returns
early when `BT_TEST_MODE=1` is set, preventing e2e tests from connecting to the
developer's shared Dolt server instead of their JSONL fixtures. Also fixed
`cmd/bt/main_test.go` and `cmd/bt/main_robot_test.go` to pass `BT_TEST_MODE=1`
when shelling out to the bt binary.

## Package Results

All 27 packages pass (`go test ./pkg/... ./internal/... ./cmd/bt/... -timeout 120s -short -count=1`).

| Package | Status | Time |
|---------|--------|------|
| cmd/bt | PASS | 3.8s |
| internal/datasource | PASS | 0.5s |
| internal/doltctl | PASS | 0.9s |
| pkg/agents | PASS | 0.7s |
| pkg/analysis | PASS | 1.2s |
| pkg/baseline | PASS | 0.6s |
| pkg/bql | PASS | 0.5s |
| pkg/cass | PASS | 1.3s |
| pkg/correlation | PASS | 1.6s |
| pkg/debug | PASS | 0.4s |
| pkg/drift | PASS | 0.6s |
| pkg/export | PASS | 17.1s |
| pkg/hooks | PASS | 10.9s |
| pkg/instance | PASS | 0.7s |
| pkg/loader | PASS | 20.5s |
| pkg/metrics | PASS | 0.4s |
| pkg/model | PASS | 0.5s |
| pkg/recipe | PASS | 0.5s |
| pkg/search | PASS | 0.7s |
| pkg/testutil | PASS | 0.5s |
| pkg/testutil/proptest | PASS | 0.8s |
| pkg/ui | PASS | 22.4s |
| pkg/updater | PASS | 1.1s |
| pkg/util/topk | PASS | 0.5s |
| pkg/version | (no test files) | - |
| pkg/watcher | PASS | 2.1s |
| pkg/workspace | PASS | 0.7s |

**Total wall time:** ~90s (parallel execution)
**Slow packages:** pkg/loader (20.5s), pkg/ui (22.4s), pkg/export (17.1s), pkg/hooks (10.9s)

### E2E Tests (tests/e2e)

The targeted robot-mode e2e tests pass:
- TestRobotTriageContract: PASS (0.35s)
- TestRobotEnvelopeConsistency: PASS (1.88s, 7 subtests)
- TestRobotUsageHintsPresent: PASS (0.81s, 5 subtests)

One unrelated e2e test (`TestCloudflare_OverwriteExistingExport`) times out at 120s
due to a file-copy issue in `export_pages_test.go`. This is a pre-existing bug
unrelated to the refactor.

## Phase 1 Blast Radius

Phase 1 (Model struct decomposition) affects 6 test files that directly access
private Model fields. All other test files (~45) use `NewModel()` + public
accessor methods and require zero changes.

### Files Requiring Updates

| File | Impact | Details |
|------|--------|---------|
| `pkg/ui/coverage_extra_test.go` | Direct field access | `m.currentFilter`, `m.showDetails` |
| `pkg/ui/context_test.go` | Direct field access | `m.showDetails` |
| `pkg/ui/snapshot_test.go` | Direct field access | `m.issues` (via internal field) |
| `pkg/ui/tree_bench_test.go` | Direct field access | `m.isBoardView`, `m.isGraphView` |
| `pkg/ui/triage_preservation_test.go` | Direct field access | `m.currentFilter` |
| `pkg/ui/sprint_view_keys_test.go` | Struct literals | 17 `Model{...}` literal constructions |

### Fields That Move Into Sub-Structs

| Current Field | Target Sub-Struct | Accessor Method |
|---------------|-------------------|-----------------|
| `m.isBoardView` | ViewMode enum | `IsBoardView()` (exists) |
| `m.isGraphView` | ViewMode enum | `IsGraphView()` (exists) |
| `m.showDetails` | PaneFocus state | `ShowDetails()` (added in Phase 0.5) |
| `m.currentFilter` | FilterState | `CurrentFilter()` (added in Phase 0.5) |
| `m.issues` | DataState | `Issues()` (added in Phase 0.5) |

### Safe Patterns (Zero Impact)

- `NewModel(issues, nil, "", nil)` - 91 call sites, constructor signature preserved
- Public accessor method calls (`IsBoardView()`, `FilteredIssues()`, etc.) - ~40 files
- E2E tests (shell out to binary) - 46 files
