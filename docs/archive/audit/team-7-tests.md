# Audit Report: Tests & Quality

**Team**: 7
**Scope**: All *_test.go files + tests/ directory (~102k lines)
**Lines scanned**: 102,774 across 222 test files in 26 packages

## Architecture Summary

The test suite is split across three tiers: unit tests co-located with production code (most packages under `pkg/` and `internal/`), integration tests within packages that exercise cross-component behavior, and a dedicated E2E suite (`tests/e2e/`) that compiles the `bt` binary and exercises it via subprocess invocation. The E2E suite uses a shared `TestMain` that builds the binary once and provides helpers (`buildBvBinary`, `runBVCommand`, `TestFixture`, `DetailedLogger`) in `common_test.go`.

Test infrastructure is well-organized. `pkg/testutil/` provides graph fixture generators (Chain, Star, Diamond, Cycle, Tree, RandomDAG, etc.) with deterministic seeds, issue model constructors, golden file helpers with cross-platform line ending normalization, and assertion helpers (AssertNoCycles, AssertStatusCounts, AssertJSONEqual, etc.). `pkg/testutil/proptest/` wraps `pgregory.net/rapid` for property-based comparison testing of old vs new implementations. Three packages have `TestMain` functions (`pkg/ui`, `pkg/export`, `tests/e2e`) that set `BT_NO_BROWSER=1` and `BT_TEST_MODE=1` safety environment variables.

The overall quality is high. Tests are predominantly table-driven with clear names. 153 benchmarks cover performance-critical paths (graph analysis, search scoring, tree rendering). 7 fuzz tests in `pkg/loader/fuzz_test.go` exercise JSONL parser robustness. Platform-specific behavior is handled via build tags (`executor_unix_test.go`, `executor_windows_test.go`) and runtime Skip guards. The suite passes with 0 failures on Windows (26/26 packages OK, 1 package has no test files: `pkg/version`).

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| Graph fixture generator | pkg/testutil/generator.go + _test.go | 554 | N/A (test infra) | Yes | Yes | 12 topology types, deterministic seeds |
| Test assertions library | pkg/testutil/assertions.go | 381 | N/A (test infra) | Indirect | Yes | Golden files, JSONL helpers, issue assertions |
| Property-based testing | pkg/testutil/proptest/ | 249 | N/A (test infra) | Yes | Yes | Generic CompareImplementations with rapid |
| E2E common harness | tests/e2e/common_test.go | 655 | Yes | Yes | Yes | Binary build, fixture builders, TUI script harness |
| E2E robot contract tests | tests/e2e/robot_contract_test.go | 557 | Yes | Yes | Yes | JSON schema validation for robot flags |
| E2E board tests | tests/e2e/board_e2e_test.go | 494 | Yes | Yes | Yes | TUI board + robot-triage validation |
| E2E export pages | tests/e2e/export_pages_test.go | 1,916 | Yes | Yes | Yes | Largest E2E file; export pipeline coverage |
| UI board model tests | pkg/ui/board_test.go | 1,843 | Yes | Yes | Yes | 61 tests; selection, navigation, swimlanes |
| UI history tests | pkg/ui/history_test.go | 2,798 | Yes | Yes | Yes | 85 tests; timeline, commit navigation |
| UI background worker | pkg/ui/background_worker_test.go | 1,975 | Yes | Yes | Yes | 40 tests; debounce, reload, stress tests |
| UI tree view tests | pkg/ui/tree_test.go | 1,588 | Yes | Yes | Yes | 45 tests; build, navigate, collapse/expand |
| UI coverage extra | pkg/ui/coverage_extra_test.go | 1,364 | Yes | Yes | Yes | 40 miscellaneous coverage gap fillers |
| UI tutorial tests | pkg/ui/tutorial_test.go | 1,222 | Yes | Yes | Yes | 50 tests; page navigation, completion tracking |
| Analysis label health | pkg/analysis/label_health_test.go | 2,597 | Yes | Yes | Yes | 84 tests; scoring, velocity, freshness |
| Analysis triage | pkg/analysis/triage_test.go | 1,454 | Yes | Yes | Yes | 56 tests; recommendations, tombstone handling |
| Analysis advanced insights | pkg/analysis/advanced_insights_test.go | 1,133 | Yes | Yes | Yes | 40 tests; signals, bottlenecks |
| Analysis benchmarks | pkg/analysis/bench_*.go (4 files) | 934 | N/A (bench) | Yes | Yes | 63 benchmarks across topology types |
| Correlation file index | pkg/correlation/file_index_test.go | 1,048 | Yes | Yes | Yes | 23 tests; file-to-bead mapping |
| Export markdown | pkg/export/markdown_test.go | 1,609 | Yes | Yes | Yes | 52 tests; Mermaid sanitization, rendering |
| Export SVG snapshots | pkg/export/graph_snapshot_svg_test.go | 1,004 | Yes | Yes | Yes | 22 tests; SVG generation, golden files |
| Export SQLite | pkg/export/sqlite_export_test.go | 525 | Partial | Yes | Yes | Tests export-to-SQLite (bt feature, not upstream) |
| Drift detection | pkg/drift/drift_test.go | 1,494 | Yes | Yes | Yes | 47 tests; baseline comparison, alerts |
| Loader JSONL parsing | pkg/loader/loader_test.go | 1,075 | Yes | Yes | Yes | 51 tests; file discovery, parsing, validation |
| Loader fuzz tests | pkg/loader/fuzz_test.go | 502 | Yes | Yes | Yes | 7 fuzz targets; parser robustness |
| Cass safety tests | pkg/cass/safety_test.go | 905 | N/A | Yes | Yes | 26 tests; invisible-when-absent guarantee |
| Hooks executor | pkg/hooks/executor_test.go | 682 | Yes | Yes | Yes | 21 tests; shell execution, timeout, env vars |
| Search hybrid scorer | pkg/search/ (17 files) | ~2,800 | Yes | Yes | Yes | Embedding, scoring, presets, index sync |
| Dolt config/lifecycle | internal/doltctl/doltctl_test.go | 178 | Yes | Yes | Yes | 11 tests; PID parse, ownership, port discovery |
| Datasource discovery | internal/datasource/ (3 files) | 1,161 | Yes | Yes | Yes | 42 tests; SQLite/JSONL/Dolt source detection |
| Updater | pkg/updater/ (6 files) | 1,078 | Yes | Yes | Yes | 37 tests; version comparison, download, network |
| Model types | pkg/model/types_test.go | 812 | Yes | Yes | Yes | 22 tests; status, type, validation |
| TOON format | cmd/bt/main_robot_test.go | 279 | Yes | Skipped | Yes | 4 tests skip when `tru` binary absent |

## Dependencies

- **Depends on**: `pgregory.net/rapid` (property-based testing), `modernc.org/sqlite` (SQLite driver for export tests), `gonum.org/v1/gonum` (graph algorithms for benchmarks), `gopkg.in/yaml.v3` (config parsing in hook tests), Charm Bracelet libraries (lipgloss, bubbletea for UI tests)
- **Depended on by**: `pkg/testutil/` is imported by `pkg/analysis/`, `pkg/ui/`, `tests/e2e/` and others. `tests/e2e/common_test.go` provides shared infrastructure for all 48 E2E test files.

## Per-Package Summary

| Package | Files | LOC | Tests | Benchmarks | Notes |
|---------|-------|-----|-------|-----------|-------|
| pkg/analysis | 48 | ~18,600 | ~600 | 63 | Largest test surface. Extensive bench suite. |
| pkg/ui | 50 | ~20,100 | ~700 | 30 | Second largest. Internal + external tests. |
| tests/e2e | 48 | ~16,500 | ~280 | 4 | Binary-level integration. Heavy use of JSONL fixtures. |
| pkg/export | 25 | ~8,200 | ~270 | 4 | SQLite export + graph rendering + golden files. |
| pkg/correlation | 19 | ~7,400 | ~220 | 0 | Git-commit correlation engine. |
| pkg/search | 17 | ~2,800 | ~60 | 10 | Hybrid search scoring. |
| pkg/cass | 5 | ~2,980 | ~100 | 9 | External tool integration safety tests. |
| pkg/hooks | 7 | ~1,000 | ~35 | 0 | Platform-split tests (Unix/Windows). |
| pkg/loader | 10 | ~2,400 | ~72 | 2 | JSONL parsing + 7 fuzz targets. |
| pkg/drift | 2 | ~1,590 | ~50 | 0 | Baseline comparison. |
| internal/datasource | 3 | ~1,160 | ~42 | 0 | Source discovery (SQLite, JSONL, Dolt). |
| pkg/updater | 6 | ~1,080 | ~37 | 2 | Self-update mechanism. |
| pkg/model | 1 | 812 | 22 | 0 | Core type validation. |
| pkg/testutil | 2 | ~800 | 23 | 5 | Fixture generators. |
| pkg/agents | 6 | ~2,080 | ~60 | 0 | Agent file detection and preferences. |
| cmd/bt | 4 | ~870 | ~25 | 0 | CLI entry point + robot output. |
| pkg/workspace | 2 | ~1,080 | ~33 | 0 | Multi-repo workspace handling. |
| Other (7 pkgs) | ~10 | ~2,300 | ~110 | 18 | debug, baseline, instance, metrics, recipe, watcher, util/topk |

## Dead Code Candidates

1. **`bv` naming remnants in E2E harness** (cosmetic, not dead): `bvBinaryPath`, `bvBinaryDir`, `buildBvBinary`, `buildBvOnce`, `runBVCommand`, `runBVCommandJSON` - 327 occurrences across 42 E2E files. The binary is correctly named `bt` in the build, but all internal variable/function names still use `bv`. Comments reference "bv binary" throughout. This is functional but inconsistent with the completed rename (Stream 2).

2. **`bv-` prefixed test data IDs**: Test fixtures in `pkg/correlation/`, `pkg/ui/history_test.go`, and `pkg/loader/fuzz_test.go` use `bv-123`, `bv-1`, `bv-2` etc. as issue IDs in test data. These are valid (upstream beads IDs use `bv-` prefix as legacy), but they could be confusing given the rename.

3. **SQLite/JSONL discovery tests in `internal/datasource/source_test.go`**: 23 tests covering SQLite and JSONL source discovery. Since upstream beads v0.56.1 removed SQLite and JSONL backends (Dolt-only), these tests cover legacy code paths. However, bt still uses JSONL for local fixture loading and SQLite for export, so these tests may still be relevant for backward compatibility. Needs cross-team clarification.

4. **TOON format tests** (`cmd/bt/main_robot_test.go`): 4 tests that always skip because `tru` binary is not available. These test `--format=toon` output. If TOON support is not a priority, these are permanently skipped tests adding no value.

5. **`pkg/export/gh_pages_e2e_test.go`**: Single test gated behind `BT_TEST_GH_PAGES_E2E=1` env var. Requires GitHub Pages infrastructure. Likely never runs in normal CI.

6. **`pkg/analysis/e2e_startup_test.go`**: 10 tests, 2 gated behind `PERF_TEST=1` env var, 1 behind real beads file existence. Performance regression tests that require manual opt-in.

7. **Stress tests in `pkg/ui/background_worker_test.go`**: Lines 1609-1975 contain 3 stress tests gated behind `PERF_TEST=1`. These are designed for manual performance verification, not CI.

8. **`pkg/watcher/fsdetect_linux_test.go`**: 154 lines, 9 tests. Build-tagged `linux` only. On Windows, these are entirely invisible. Not dead code per se, but worth noting for platform coverage.

## Notable Findings

1. **Comprehensive property-based testing infrastructure**: The `pkg/testutil/proptest/` package with `pgregory.net/rapid` integration is a rare and valuable testing asset. It enables systematic comparison of old vs new implementations during refactoring. Currently only self-tested; no production code uses it yet. This is a readiness asset for the Stabilize phase.

2. **E2E tests use JSONL fixtures exclusively**: All 48 E2E tests create `.beads/beads.jsonl` files for test data. None test the Dolt path. Since upstream is now Dolt-only, there are no E2E tests that exercise the actual production data path. The robot mode tests verify JSON output structure, but always against JSONL-loaded data.

3. **TUI testing uses `script` command harness**: E2E tests on Linux/macOS use the Unix `script` command to provide a pseudo-TTY for TUI tests. On Windows, all TUI tests are skipped (`skipIfNoScript`). This means TUI rendering is never tested on the primary development platform.

4. **Golden file system is cross-platform ready**: `pkg/testutil/assertions.go` normalizes `\r\n` to `\n` in golden file comparisons. 11 golden files exist in `testdata/golden/graph_render/` (Mermaid, SVG, ASCII formats).

5. **Extremely thorough analysis benchmarks**: 63 benchmarks across 4 dedicated benchmark files test the analysis pipeline at multiple graph sizes (100, 500, 1000 nodes) and topologies (sparse, dense, chain, wide, deep, disconnected, pathological). This is production-quality performance regression infrastructure.

6. **Fuzz testing for parser hardening**: 7 fuzz targets in `pkg/loader/fuzz_test.go` cover JSONL parsing, issue validation, timestamps, dependencies, comments, and large lines. Seed corpus includes adversarial inputs (incomplete JSON, invalid UTF-8, injection strings, huge priority values).

7. **Safety-first cass testing**: `pkg/cass/safety_test.go` (905 lines, 26 tests) explicitly tests the guarantee that cass integration is invisible when cass is not installed. Tests verify no blocking, no errors surfaced to users, no crashes. This is a good pattern for optional feature integration.

8. **Export tests cover SQLite output**: 3 test files (525 + 212 + 499 = 1,236 lines) test SQLite export functionality. This is a bt-specific feature (exporting beads data to SQLite for external viewers), not the upstream SQLite backend that was removed.

9. **3,154 top-level test/benchmark/fuzz functions**: Across 222 files. The ratio of test code to production code is approximately 1.2:1 (~102k test vs ~88k production), which is healthy for a TUI application.

10. **Test isolation is well-handled**: Tests use `t.TempDir()` consistently, `t.Setenv()` for environment variable testing (Go 1.17+), and `t.Cleanup()` for resource management. No global state mutation was observed outside of the `TestMain` functions.

## Questions for Synthesis

1. **JSONL fixture dependency in E2E tests**: Since upstream is Dolt-only, should E2E tests be adapted to test against Dolt, or is JSONL-based testing acceptable as a testing convenience? The loader still supports JSONL for bt's own use, but this means the Dolt data path has zero E2E coverage.

2. **`bv` naming cleanup scope**: The 327 `bv` references in E2E test code are purely internal (variable names, function names, comments). They don't affect functionality. Is renaming these a priority, or cosmetic debt to defer?

3. **SQLite datasource tests**: `internal/datasource/source_test.go` tests SQLite and JSONL discovery. The `internal/datasource/` package is used by the export pipeline (not for reading upstream beads data). Team 1a or Team 3 should clarify whether these source types are still reachable in production code paths.

4. **TOON tests permanently skipped**: 4 tests in `cmd/bt/main_robot_test.go` skip when `tru` binary is absent. Is TOON format (`--format=toon`) a supported feature? If not, these tests and the feature code are dead weight.

5. **No Dolt integration tests**: `internal/doltctl/doltctl_test.go` tests parsing and ownership logic but doesn't start an actual Dolt server. No test in the suite connects to a real Dolt database. This is the most significant coverage gap given that Dolt is the production data path.

6. **Platform gap for TUI tests**: TUI rendering tests via `script` only run on Linux/macOS. On Windows (the primary dev platform), all TUI-specific E2E tests are skipped. Worth addressing if Windows CI is a goal.
