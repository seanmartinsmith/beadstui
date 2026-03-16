---
title: "fix: Cross-platform test suite fixes"
type: fix
status: active
date: 2026-03-16
origin: session-14 brainstorm (test failure audit)
---

# Cross-Platform Test Suite Fixes

## Overview

39 tests fail on Windows across 9 root cause categories. All fixes are test-layer or minor production code changes. No feature behavior changes. Goal: all tests pass on Windows, no regressions on Mac/Linux.

## Problem

The test suite was written primarily for Unix. It fails on Windows due to: incomplete bv->bt rename, Unix shell syntax in hook tests, Unix-only file permissions, HOME env var not controlling config paths on Windows, path separator mismatches, file locking during cleanup, golden file line endings, missing .exe suffix in e2e tests, and 2 actual logic bugs.

The hooks tests are particularly disruptive - they trigger Windows "Open With" dialogs by trying to execute .sh files directly.

## Design

### Phase 1: Rename stragglers + logic bugs (9 tests, all platforms)

Fix `bv` -> `bt` in test data and source:
- `cmd/bt`: buildTestBinary() references `./cmd/bv` and omits `.exe` suffix
- `pkg/agents`: AgentBlurb constant still uses `br list/show/create/update` instead of `bd`
- `pkg/correlation`: test data uses `bv-67`, `BV-67`, code checks `bt` prefix
- `pkg/loader`: gitignore pattern matching checks `.bt` but test has `.bv`
- `pkg/updater`: archive contains `path/bv`, extractBinary looks for `bt`

Fix 2 logic bugs (not Windows-specific):
- `pkg/analysis`: ComputeUnblocks includes parent_child edges, should filter to blocks only
- `pkg/export`: slug collision test expectations don't match actual slug generation

### Phase 2: Path separators (2 tests)

Change test expectations to use `filepath.Join` or `filepath.FromSlash` instead of hardcoded `/`:
- `pkg/cass`: TestWorkspaceFromBeadsPath
- `pkg/ui`: TestTreeStatePath

### Phase 3: Config path testability (5 tests)

Refactor config path functions to accept an optional homeDir override:
- `TutorialProgressPath(homeDir string)` - empty string = use `os.UserHomeDir()` default
- `WizardConfigPath(homeDir string)` - same pattern

Tests pass tmpDir directly instead of trying to redirect HOME. Production code passes `""`.

Affected tests:
- `pkg/export`: TestSaveAndLoadWizardConfig
- `pkg/ui`: TestTutorialProgressManager_SaveLoad, LoadNonexistent, LoadInvalidJSON, TestTutorialModel_SaveProgress, HasViewedPage

### Phase 4: Skip guards for Unix-only tests (7 tests)

Add `if runtime.GOOS == "windows" { t.Skip("requires Unix file permissions") }`:
- `pkg/agents`: TestAtomicWritePreservesPermissions, TestAtomicWriteNoPermission
- `pkg/drift`: TestConfigLoadPermissionError, TestConfigSavePermissionError
- `pkg/ui`: TestBackgroundWorker_PreservesSnapshotOnPermissionErrorAndRecovers
- `tests/e2e`: TestBinary_HasProperPermissions

### Phase 5: Hooks test cross-platform (5 tests)

Skip shell-syntax-dependent tests on Windows. Keep 1-2 smoke tests with platform-native commands:
- `pkg/hooks`: TestExecutorEnvironmentVariables, TestExecutorCustomEnvExpansion, TestExecutorPermissionDenied (this one triggers "Open With" dialogs)
- `tests/e2e`: TestExportPages_IncludesHistoryAndRunsHooks
- `tests/e2e`: TestRobotHelp_DocumentsUpdateFeatures, TestStartup_UpdateCheckDoesNotBlock (flag parsing issue with `-robot-*` flags)

### Phase 6: E2E binary suffix + file locking (8 tests)

Add `.exe` suffix on Windows for test binary paths:
- `tests/e2e`: TestEndToEndDriftWorkflow, TestDriftAlerts, TestDriftConfigCustomization, TestDriftErrorHandling

File locking: ensure Model.Stop() / watcher cleanup before TempDir cleanup:
- `pkg/ui`: TestFileChangedPreservesTriageData, TestUpdateFileChangedReloadsSelection, TestNewModel_SetsTreeBeadsDirFromBeadsPath
- `pkg/hooks`: TestLoadDefaultNoHooks (CWD in temp dir issue)

### Phase 7: Golden file regeneration (2 tests)

Normalize line endings in comparison (strip `\r` before comparing) or use platform-aware golden file loading:
- `pkg/export`: TestGraphExport_GoldenMermaid, TestGraphRender_GoldenSVG

## What we're NOT doing

- No changes to production behavior
- No new cross-platform abstractions in the hooks executor (it already works)
- No Windows ACL support for permission tests (skip is the right answer)
- No WSL-specific handling (Linux tests just work there)

## Acceptance Criteria

- `go test ./...` on Windows: 0 failures (down from 39)
- `go test ./...` on Mac/Linux: no regressions
- No "Open With" dialogs triggered during test runs
- All bv -> bt rename stragglers fixed in both test and source code
