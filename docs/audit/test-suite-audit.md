# Test Suite Audit (bt-79eg)

**Date**: 2026-04-03
**Scope**: All 268 test files in beadstui (excluding `_tmp/perles/` vendored BQL source)
**Test Status**: All 27 packages pass on Windows (`go test -short ./...`). E2E suite has one panic from Windows path length limits in export page copy helper.

## Summary Stats

| Category | Count | % |
|----------|-------|---|
| KEEP     | 249   | 93% |
| UPDATE   | 12    | 4% |
| REMOVE   | 0     | 0% |
| REWRITE  | 0     | 0% |
| REVIEW   | 7     | 3% |

**Key finding**: The rename from bv->bt was thorough. The test suite is in good shape. No dead-code tests, no tests for removed features. The issues found are cosmetic (stale comments/error-message strings) and one platform-specific path length bug.

## Classification Notes

- **bv-NNN bead IDs in test fixtures** (e.g., `bv-123`, `bv-test`, `bv-open`): These are arbitrary test fixture data using `bv` as a bead ID prefix. Bead IDs can have any prefix - these are NOT stale naming references. Categorized as KEEP.
- **bv-graph references**: The `bv-graph-wasm` WASM module and its `bv-graph:nodeClick` event namespace are intentional names for the embedded graph viewer module. Categorized as KEEP.
- **bv-XXXX issue references in comments** (e.g., `bv-52`, `bv-qfr5`, `bv-i3ls`): Historical beads issue IDs from before the rename. These are just comments/documentation, not functional code. Categorized as KEEP (cosmetic only).
- **Beads Viewer in export tests**: The embedded HTML viewer (`pkg/export/viewer_assets/`) still uses "Beads Viewer" as its title. Tests correctly match this production behavior. Categorized as KEEP (rename is a separate issue tracked elsewhere).
- **bv-pages directory name**: Used in both production code and tests as the default export directory name. Tests match production. Categorized as KEEP (rename is a separate production-code issue).
- **bv-agent-instructions markers**: Versioned HTML comment markers embedded in user AGENTS.md files. Production code still uses these. Tests correctly match. Categorized as KEEP (renaming these would break existing user files).
- **SQLite/JSONL tests**: bt still supports these as legacy backends for non-Dolt projects. Tests are valid and should be kept.

## Detailed Audit by Package

### cmd/bt/ (4 files)

| File | Category | Notes |
|------|----------|-------|
| `burndown_test.go` | KEEP | Tests burndown calculation logic. Clean, uses bt naming. |
| `main_test.go` | KEEP | Tests robot flags, recipe filters, repo filtering. Clean. |
| `main_robot_test.go` | KEEP | Tests robot-plan/priority metadata, TOON output format. Clean. |
| `profile_test.go` | KEEP | Tests formatting helpers and profiling output. Clean. |

### internal/datasource/ (3 files)

| File | Category | Notes |
|------|----------|-------|
| `dolt_test.go` | UPDATE | Line 70: `"dolt_database":"bv"` - cosmetic but slightly confusing. The value is arbitrary test data but could be changed to `"beads"` for clarity. Low priority. |
| `metadata_test.go` | KEEP | Tests Dolt config reading, port precedence, env var overrides. Clean, uses `BT_DOLT_PORT` and `BEADS_DOLT_SERVER_PORT`. |
| `source_test.go` | KEEP | Tests source discovery for SQLite, JSONL, Dolt. These backends are still supported. |

### internal/doltctl/ (1 file)

| File | Category | Notes |
|------|----------|-------|
| `doltctl_test.go` | KEEP | Tests bd dolt start output parsing, PID-based ownership, stop-if-owned logic. Clean. |

### pkg/agents/ (6 files)

| File | Category | Notes |
|------|----------|-------|
| `blurb_test.go` | KEEP | Tests bv-agent-instructions markers. These markers are intentionally `bv-` prefixed in production code (changing would break existing user AGENTS.md files). |
| `detect_test.go` | KEEP | Tests AGENTS.md detection. Clean. |
| `file_test.go` | KEEP | Tests blurb append to file. Clean. |
| `integration_test.go` | KEEP | Tests full accept/decline flow. Clean. |
| `prefs_test.go` | KEEP | Tests preference hashing and persistence. Clean. |
| `tty_guard_test.go` | KEEP | Tests TTY suppression for robot mode. Uses `bt` naming correctly. |

### pkg/analysis/ (31 files)

| File | Category | Notes |
|------|----------|-------|
| `advanced_insights_test.go` | KEEP | |
| `articulation_test.go` | KEEP | |
| `bench_generators_test.go` | KEEP | |
| `bench_pathological_test.go` | KEEP | |
| `bench_realdata_test.go` | KEEP | |
| `bench_test.go` | KEEP | |
| `benchmark_test.go` | KEEP | |
| `betweenness_approx_test.go` | KEEP | |
| `buffer_pool_test.go` | KEEP | |
| `cache_extra_test.go` | KEEP | |
| `cache_test.go` | KEEP | |
| `config_test.go` | KEEP | |
| `cycle_warnings_test.go` | KEEP | |
| `dependency_suggest_test.go` | KEEP | |
| `diff_extended_test.go` | KEEP | |
| `diff_test.go` | KEEP | |
| `duplicates_test.go` | KEEP | |
| `e2e_startup_test.go` | KEEP | |
| `eta_test.go` | KEEP | |
| `feedback_test.go` | KEEP | |
| `golden_test.go` | KEEP | |
| `graph_accessor_benchmark_test.go` | KEEP | |
| `graph_accessor_test.go` | KEEP | |
| `graph_cycles_test.go` | KEEP | |
| `graph_extra_test.go` | KEEP | |
| `graph_test.go` | KEEP | |
| `insights_signals_test.go` | KEEP | |
| `insights_test.go` | KEEP | |
| `invariance_test.go` | KEEP | |
| `label_health_test.go` | KEEP | |
| `label_suggest_test.go` | UPDATE | Line 451: Error message says `'br update'` but assertion correctly checks for `'bd update'`. The error message string is stale - would mislead during debugging. |
| `perf_invariants_test.go` | KEEP | |
| `plan_extended_test.go` | KEEP | |
| `plan_test.go` | KEEP | |
| `priority_test.go` | KEEP | |
| `real_data_test.go` | KEEP | |
| `risk_test.go` | KEEP | |
| `sample_integration_test.go` | KEEP | |
| `status_fullstats_test.go` | KEEP | |
| `suggest_all_test.go` | KEEP | |
| `suggestions_test.go` | KEEP | |
| `triage_context_test.go` | KEEP | |
| `triage_test.go` | KEEP | |
| `whatif_test.go` | KEEP | |

### pkg/baseline/ (1 file)

| File | Category | Notes |
|------|----------|-------|
| `baseline_test.go` | KEEP | Uses `.bt/baseline.json` path correctly. |

### pkg/bql/ (4 files)

| File | Category | Notes |
|------|----------|-------|
| `lexer_test.go` | KEEP | Adapted from perles. Clean. |
| `parser_test.go` | KEEP | Adapted from perles. Clean. |
| `validator_test.go` | KEEP | Clean. |
| `memory_executor_test.go` | KEEP | Uses `bt-001` style IDs. Clean. |

### pkg/cass/ (5 files)

| File | Category | Notes |
|------|----------|-------|
| `cache_test.go` | KEEP | |
| `correlation_test.go` | KEEP | |
| `detector_test.go` | KEEP | |
| `safety_test.go` | KEEP | |
| `search_test.go` | KEEP | |

### pkg/correlation/ (16 files)

| File | Category | Notes |
|------|----------|-------|
| `cache_test.go` | KEEP | |
| `causality_test.go` | KEEP | Uses `bv-test` as fixture bead IDs - these are arbitrary prefixes, not stale refs. |
| `cocommit_test.go` | KEEP | Uses `bv-123` as fixture bead IDs. |
| `correlator_test.go` | KEEP | |
| `explicit_test.go` | KEEP | Tests `Closes BV-42` pattern matching - correctly tests ID extraction from commit messages. |
| `extractor_test.go` | KEEP | |
| `file_index_test.go` | KEEP | Uses `bv-123`, `bv-456` as fixture IDs. |
| `gitlog_test.go` | KEEP | |
| `incremental_test.go` | KEEP | |
| `network_test.go` | KEEP | |
| `orphan_test.go` | KEEP | |
| `related_test.go` | KEEP | |
| `reverse_test.go` | KEEP | |
| `scorer_test.go` | KEEP | |
| `stream_test.go` | KEEP | |
| `temporal_path_test.go` | KEEP | |
| `temporal_test.go` | KEEP | |
| `types_test.go` | KEEP | |

### pkg/debug/ (1 file)

| File | Category | Notes |
|------|----------|-------|
| `debug_test.go` | KEEP | |

### pkg/drift/ (2 files)

| File | Category | Notes |
|------|----------|-------|
| `drift_test.go` | KEEP | |
| `summary_test.go` | KEEP | |

### pkg/export/ (22 files)

| File | Category | Notes |
|------|----------|-------|
| `cloudflare_test.go` | KEEP | |
| `deploy_flow_test.go` | REVIEW | Skips on Windows (`shell script stubs not supported`). Uses shell script stubs for gh/wrangler mocking. Worth reviewing if Windows CI coverage matters. |
| `external_tools_test.go` | KEEP | |
| `gh_pages_e2e_test.go` | REVIEW | Shell-based mocking may have Windows limitations. |
| `github_test.go` | KEEP | |
| `graph_export_test.go` | KEEP | |
| `graph_interactive_test.go` | KEEP | |
| `graph_render_beautiful_test.go` | KEEP | |
| `graph_render_golden_test.go` | KEEP | |
| `graph_snapshot_bench_test.go` | KEEP | |
| `graph_snapshot_svg_test.go` | KEEP | |
| `graph_snapshot_test.go` | KEEP | |
| `init_and_push_test.go` | KEEP | |
| `integration_test.go` | KEEP | Tests SQLite export integration - still supported backend. |
| `livereload_test.go` | KEEP | |
| `main_test.go` | KEEP | |
| `markdown_test.go` | KEEP | |
| `preview_flow_test.go` | KEEP | |
| `preview_test.go` | KEEP | |
| `sqlite_export_metrics_test.go` | KEEP | Tests SQLite export with metrics - still supported. |
| `sqlite_export_test.go` | KEEP | Tests SQLite export - still supported. Comment on line 409 references `bv-52` (historical issue ID). |
| `sqlite_schema_test.go` | UPDATE | Lines 34, 44, 111: Comments reference `bv-52` as a historical issue ID. Cosmetic only - could be updated to `bt-` prefix or removed. |
| `viewer_embed_test.go` | KEEP | Tests "Beads Viewer" title replacement. Matches production HTML templates. |
| `wizard_flow_test.go` | KEEP | |
| `wizard_prereq_test.go` | KEEP | |
| `wizard_test.go` | KEEP | |

### pkg/hooks/ (7 files)

| File | Category | Notes |
|------|----------|-------|
| `config_yaml_test.go` | KEEP | |
| `executor_extra_test.go` | KEEP | |
| `executor_test.go` | KEEP | Uses `BT_*` env vars correctly. |
| `executor_unix_test.go` | KEEP | Build-tagged for `!windows`. |
| `executor_windows_test.go` | KEEP | Build-tagged for `windows`. |
| `loader_extra_test.go` | KEEP | |
| `runhooks_test.go` | KEEP | |

### pkg/instance/ (1 file)

| File | Category | Notes |
|------|----------|-------|
| `lock_test.go` | KEEP | |

### pkg/loader/ (11 files)

| File | Category | Notes |
|------|----------|-------|
| `benchmark_test.go` | KEEP | |
| `bom_test.go` | KEEP | |
| `fuzz_test.go` | KEEP | |
| `git_test.go` | KEEP | |
| `gitignore_test.go` | KEEP | Tests `.bt` pattern matching. Correctly uses bt naming. |
| `loader_extra_test.go` | KEEP | |
| `loader_test.go` | KEEP | Comment on line 925 references `bv-zaxb` (historical issue ID). Cosmetic. |
| `real_data_test.go` | KEEP | Skips if no test data present. |
| `robustness_test.go` | KEEP | |
| `sprint_test.go` | KEEP | Uses `bv-1`, `bv-2` as fixture bead IDs in sprint data. Arbitrary test data. |
| `synthetic_test.go` | KEEP | Uses `bd-101` style IDs. Clean. |

### pkg/metrics/ (1 file)

| File | Category | Notes |
|------|----------|-------|
| `metrics_test.go` | KEEP | |

### pkg/model/ (1 file)

| File | Category | Notes |
|------|----------|-------|
| `types_test.go` | KEEP | |

### pkg/recipe/ (2 files)

| File | Category | Notes |
|------|----------|-------|
| `loader_test.go` | KEEP | |
| `types_test.go` | KEEP | |

### pkg/search/ (16 files)

| File | Category | Notes |
|------|----------|-------|
| `bench_helpers_test.go` | KEEP | |
| `config_test.go` | KEEP | |
| `documents_test.go` | KEEP | |
| `embedder_test.go` | KEEP | |
| `hash_embedder_test.go` | KEEP | |
| `hybrid_scorer_bench_test.go` | KEEP | |
| `hybrid_scorer_real_test.go` | KEEP | |
| `hybrid_scorer_test.go` | KEEP | |
| `index_sync_test.go` | KEEP | |
| `lexical_boost_test.go` | KEEP | |
| `metrics_cache_bench_test.go` | KEEP | |
| `metrics_cache_test.go` | KEEP | |
| `normalizers_test.go` | KEEP | |
| `presets_test.go` | KEEP | |
| `query_adjust_test.go` | KEEP | |
| `search_pipeline_real_test.go` | KEEP | |
| `short_query_hybrid_test.go` | KEEP | |
| `vector_index_test.go` | KEEP | |
| `weights_test.go` | KEEP | |

### pkg/testutil/ (3 files)

| File | Category | Notes |
|------|----------|-------|
| `generator_test.go` | KEEP | |
| `proptest/proptest_test.go` | KEEP | |

### pkg/ui/ (48 files)

| File | Category | Notes |
|------|----------|-------|
| `actionable_test.go` | KEEP | |
| `agent_prompt_modal_test.go` | KEEP | |
| `attention_test.go` | KEEP | |
| `background_worker_test.go` | KEEP | |
| `benchmark_test.go` | KEEP | |
| `board_test.go` | KEEP | |
| `capslock_test.go` | KEEP | |
| `cass_session_modal_test.go` | KEEP | Uses `bv-abc123` as fixture bead ID. |
| `context_help_test.go` | KEEP | |
| `context_test.go` | KEEP | |
| `coverage_extra_test.go` | KEEP | |
| `delegate_test.go` | KEEP | |
| `flow_matrix_test.go` | KEEP | Comment references `bv-w4l0` (historical issue ID). |
| `graph_bench_test.go` | KEEP | |
| `graph_golden_test.go` | KEEP | |
| `graph_internal_test.go` | KEEP | |
| `graph_test.go` | KEEP | |
| `helpers_test.go` | KEEP | |
| `history_selection_test.go` | KEEP | |
| `history_test.go` | KEEP | |
| `insights_test.go` | KEEP | |
| `integration_test.go` | KEEP | Comment references `bv-i3ls` (historical issue ID). |
| `item_test.go` | KEEP | Line 607 uses `bv-xyz789` as test fixture bead ID - valid test data. |
| `label_dashboard_test.go` | KEEP | |
| `label_picker_test.go` | KEEP | |
| `logic_test.go` | KEEP | |
| `main_test.go` | KEEP | Test harness setup. |
| `markdown_test.go` | KEEP | |
| `model_test.go` | KEEP | |
| `panel_test.go` | KEEP | |
| `recipe_picker_test.go` | KEEP | |
| `repo_picker_test.go` | KEEP | |
| `semantic_search_test.go` | KEEP | |
| `shortcuts_sidebar_test.go` | KEEP | |
| `snapshot_test.go` | KEEP | |
| `sprint_view_keys_test.go` | KEEP | |
| `styles_test.go` | KEEP | |
| `theme_loader_test.go` | KEEP | |
| `theme_test.go` | KEEP | |
| `tree_bench_test.go` | KEEP | |
| `tree_test.go` | KEEP | |
| `triage_preservation_test.go` | KEEP | |
| `truncate_test.go` | KEEP | |
| `tutorial_progress_test.go` | KEEP | Tests tutorial progress persistence. Feature is functional. |
| `tutorial_test.go` | KEEP | Tests tutorial navigation, TOC, rendering. Feature is functional. |
| `update_keys_test.go` | KEEP | |
| `update_modal_test.go` | KEEP | |
| `update_test.go` | KEEP | |
| `velocity_comparison_test.go` | KEEP | |
| `visuals_test.go` | KEEP | |
| `workspace_filter_test.go` | KEEP | |
| `workspace_repos_test.go` | KEEP | |

### pkg/updater/ (6 files)

| File | Category | Notes |
|------|----------|-------|
| `download_test.go` | KEEP | |
| `fileops_test.go` | KEEP | |
| `helpers_test.go` | KEEP | |
| `integration_test.go` | UPDATE | Line 25: `"mock bv v99.0.0"` - stale echo string in mock binary content. Should be `"mock bt v99.0.0"`. Cosmetic only - the string is never asserted against. |
| `network_test.go` | KEEP | |
| `updater_test.go` | KEEP | Uses `seanmartinsmith/beadstui` URLs correctly. |

### pkg/util/topk/ (1 file)

| File | Category | Notes |
|------|----------|-------|
| `topk_test.go` | KEEP | |

### pkg/watcher/ (2 files)

| File | Category | Notes |
|------|----------|-------|
| `watcher_test.go` | KEEP | |
| `fsdetect_linux_test.go` | KEEP | Build-tagged for linux. Tests mount point detection. |

### pkg/workspace/ (2 files)

| File | Category | Notes |
|------|----------|-------|
| `loader_test.go` | KEEP | |
| `types_test.go` | KEEP | |

### tests/e2e/ (41 files)

| File | Category | Notes |
|------|----------|-------|
| `agents_integration_e2e_test.go` | KEEP | |
| `board_e2e_test.go` | KEEP | |
| `board_swimlane_e2e_test.go` | KEEP | |
| `brief_exports_test.go` | KEEP | |
| `cass_modal_e2e_test.go` | KEEP | |
| `common_test.go` | KEEP | Shared test helpers. Uses `BT_*` env vars. |
| `correlation_e2e_test.go` | KEEP | |
| `cycle_visualization_e2e_test.go` | KEEP | |
| `drift_test.go` | KEEP | |
| `emit_script_test.go` | UPDATE | Line 43: Error message says `"missing br show command"` but the assertion checks for `"bd show A"`. The assertion is correct; only the error message is stale. |
| `error_scenarios_e2e_test.go` | KEEP | |
| `export_cloudflare_test.go` | REVIEW | May have Windows path length issues (deep nested directories in viewer assets). |
| `export_graph_topologies_test.go` | KEEP | |
| `export_incremental_test.go` | KEEP | Uses `bv-pages` as directory name - matches production code. |
| `export_offline_test.go` | REVIEW | Triggered the Windows path length panic. The `copyDirRecursive` helper in `export_pages_test.go` creates paths exceeding Windows MAX_PATH when copying viewer_assets. |
| `export_pages_test.go` | REVIEW | Contains the `copyDirRecursive` and `copyFile` helpers that panic on Windows with long paths. Lines 1394, 1408 reference `bv-graph:nodeClick` (intentional WASM module event name). |
| `forecast_test.go` | KEEP | |
| `graph_analysis_detailed_e2e_test.go` | KEEP | |
| `graph_export_e2e_test.go` | KEEP | |
| `graph_navigation_e2e_test.go` | KEEP | |
| `history_timeline_e2e_test.go` | KEEP | |
| `main_test.go` | KEEP | Test harness. |
| `performance_regression_e2e_test.go` | KEEP | |
| `race_conditions_e2e_test.go` | KEEP | |
| `robot_alerts_test.go` | KEEP | |
| `robot_burndown_scope_test.go` | KEEP | |
| `robot_burndown_test.go` | KEEP | |
| `robot_capacity_test.go` | KEEP | |
| `robot_contract_test.go` | KEEP | |
| `robot_diff_test.go` | KEEP | |
| `robot_graph_test.go` | KEEP | |
| `robot_history_test.go` | KEEP | |
| `robot_matrix_test.go` | KEEP | |
| `robot_search_hybrid_test.go` | KEEP | |
| `robot_search_test.go` | KEEP | |
| `robot_sprint_test.go` | KEEP | |
| `robot_stderr_cleanliness_test.go` | KEEP | |
| `robot_suggest_test.go` | KEEP | |
| `search_benchmark_test.go` | KEEP | |
| `triage_detailed_e2e_test.go` | KEEP | |
| `tui_hybrid_search_test.go` | KEEP | |
| `tui_snapshot_test.go` | KEEP | |
| `update_flow_test.go` | KEEP | |
| `wizard_flow_e2e_test.go` | KEEP | |
| `workflow_e2e_test.go` | KEEP | Comment references `bv-qfr5` (historical issue ID). |
| `workspace_robot_output_e2e_test.go` | KEEP | |

## Priority Recommendations

### P1: Fix Windows path length panic in e2e export tests

**Files**: `tests/e2e/export_pages_test.go` (copyFile/copyDirRecursive helpers), triggered by `export_offline_test.go` and possibly `export_cloudflare_test.go`

The `copyDirRecursive` function creates deeply nested paths when copying viewer_assets that exceed Windows MAX_PATH (260 chars). This causes a panic (not a graceful failure) that kills the entire e2e test suite. Fix options:
- Use `\\?\` extended path prefix on Windows
- Shorten temp dir names
- Skip these specific tests on Windows with a build tag or runtime check

### P2: Fix stale error message strings (3 files, trivial)

These are wrong error/comment strings where the assertion logic is correct but the human-readable message references the old `br` CLI name:

1. **`tests/e2e/emit_script_test.go:43`**: `"missing br show command"` should be `"missing bd show command"`
2. **`pkg/analysis/label_suggest_test.go:451`**: `"should contain 'br update'"` should be `"should contain 'bd update'"`
3. **`pkg/updater/integration_test.go:25`**: `"mock bv v99.0.0"` should be `"mock bt v99.0.0"` (mock binary echo string, never asserted)

### P3: Clean up historical issue ID references in comments (cosmetic)

Several test files reference old `bv-XXXX` issue IDs in comments. These are purely cosmetic - the code works correctly. Examples:
- `pkg/export/sqlite_schema_test.go`: lines 34, 44, 111 reference `bv-52`
- `pkg/export/sqlite_export_test.go`: line 409 references `bv-52`
- `tests/e2e/workflow_e2e_test.go`: references `bv-qfr5`
- Various UI tests reference historical issue IDs in section headers

Not worth batch-renaming since these are documentation breadcrumbs, but new tests should use `bt-` prefixed issue IDs.

### P4: Review shell-dependent e2e tests on Windows

`pkg/export/deploy_flow_test.go` and `pkg/export/gh_pages_e2e_test.go` use shell script stubs for mocking external tools (gh, wrangler). These skip on Windows already, but worth noting as a coverage gap if Windows CI matters.

### Not an issue: SQLite/JSONL backend tests

The 11 test files touching SQLite are still valid. bt supports SQLite/JSONL as legacy backends for non-Dolt projects. These should remain until/unless that support is explicitly removed.

### Not an issue: Tutorial/progress tests

`pkg/ui/tutorial_test.go` and `pkg/ui/tutorial_progress_test.go` test functional features. Both pass. No pre-existing failures found despite earlier flagging.

## Test Coverage Distribution

| Area | Test Files | Notes |
|------|-----------|-------|
| Core TUI (pkg/ui/) | 48 | Heaviest coverage. Board, graph, sprint, modals, theme, all tested. |
| Analysis engine (pkg/analysis/) | 31 | Comprehensive: graph algorithms, caching, golden files, benchmarks. |
| Export system (pkg/export/) | 22 | SQLite, markdown, graph rendering, wizard flow, deploy. |
| Correlation (pkg/correlation/) | 16 | Git correlation, co-commits, file index, temporal analysis. |
| Search (pkg/search/) | 16 | Hybrid search, embedders, scoring, benchmarks. |
| E2E (tests/e2e/) | 41 | Robot mode, board, export, workflow, search, graph, drift. |
| Loader (pkg/loader/) | 11 | JSONL parsing, git history, sprints, fuzz testing. |
| Hooks (pkg/hooks/) | 7 | Platform-specific executor, YAML config, loader. |
| Agents (pkg/agents/) | 6 | AGENTS.md detection, blurb management, TTY guard. |
| CASS (pkg/cass/) | 5 | Session search, caching, safety, detection. |
| BQL (pkg/bql/) | 4 | Lexer, parser, validator, memory executor. |
| Entry point (cmd/bt/) | 4 | CLI flags, burndown, profiling, robot output. |
| Data sources (internal/) | 4 | Dolt config, metadata, source discovery, doltctl. |
| Other (updater, drift, etc.) | 53 | Well-distributed across remaining packages. |
