# Audit Report: Agents & Automation

**Team**: 6
**Scope**: pkg/agents/ - AGENTS.md generation, agent capabilities, robot mode integration
**Lines scanned**: ~2,887 (800 source, 2,087 test across 6 source files and 6 test files)

## Architecture Summary

The `pkg/agents` package provides AGENTS.md/CLAUDE.md file management for AI coding agent integration. Its core responsibility is injecting a versioned "blurb" (a block of markdown instructions teaching AI agents how to use the `bd` beads CLI) into agent configuration files found in project directories. The package has no dependency on the TUI framework, graph analysis, or data layer - it is a pure file-management utility.

The architecture is split across five concerns: (1) `blurb.go` defines the AgentBlurb constant and all string-manipulation functions for detecting, appending, removing, and upgrading blurb content (including legacy format migration); (2) `detect.go` provides filesystem scanning that searches for AGENTS.md, CLAUDE.md, agents.md, or claude.md in a directory or its parents, reads the file, and returns an `AgentFileDetection` struct with blurb presence/version metadata; (3) `file.go` handles the actual file I/O with atomic writes (temp file + rename, with a Windows-specific fallback for rename-over-existing); (4) `prefs.go` manages per-project user preferences (accept/decline/never-ask) stored as JSON files in `~/.config/bt/agent-prompts/`, keyed by a SHA256 hash of the project path; (5) `tty_guard.go` uses a package-level `init()` to suppress Termenv TTY probing in robot mode by setting `CI=1` when `--robot-*` flags, `BT_ROBOT=1`, or `BT_TEST_MODE` are detected.

The package is consumed by three production call sites: `cmd/bt/main.go` (CLI `--agents-add`/`--agents-update`/`--agents-remove` flags), `pkg/ui/model.go` (TUI startup check that may prompt the user), and `pkg/ui/agent_prompt_modal.go` (a Bubble Tea modal dialog for the in-TUI prompt). The `tty_guard.go` init function runs on every bt invocation because Go runs all `init()` functions at startup.

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| AgentBlurb constant (bd CLI instructions) | blurb.go:23-84 | 62 | N/A | Yes | Yes | Contains `bd` commands (ready, list, show, create, update, close, sync, dep add) |
| Blurb detection (current format) | blurb.go:117-119 | 3 | N/A | Yes | Yes | Simple string contains check for HTML comment marker |
| Legacy blurb detection | blurb.go:124-137 | 14 | N/A | Yes | Yes | Requires all 4 legacy patterns to avoid false positives |
| Blurb version extraction | blurb.go:145-155 | 11 | N/A | Yes | Yes | Regex-based, supports multi-digit versions |
| Blurb append/remove/update (string ops) | blurb.go:169-239 | 71 | N/A | Yes | Yes | UpdateBlurb removes legacy then current, then appends |
| Agent file detection (directory scan) | detect.go:51-78 | 28 | N/A | Yes | Yes | Priority: AGENTS.md > CLAUDE.md > agents.md > claude.md |
| Parent directory traversal | detect.go:115-132 | 18 | N/A | Yes | Yes | Walks up with configurable maxLevels |
| AgentFileDetection struct + methods | detect.go:9-46 | 38 | N/A | Yes | Yes | Found(), NeedsBlurb(), NeedsUpgrade() |
| Atomic file write | file.go:89-151 | 63 | N/A | Yes | Yes | Temp file + sync + chmod + rename; Windows remove-then-rename fallback |
| File-level blurb operations | file.go:12-86 | 75 | N/A | Yes | Yes | AppendBlurbToFile, UpdateBlurbInFile, RemoveBlurbFromFile, CreateAgentFile |
| EnsureBlurb (orchestrator) | file.go:157-178 | 22 | N/A | Partial | Yes | Only tested via e2e, not main.go CLI path |
| Per-project preference storage | prefs.go:1-183 | 183 | N/A | Yes | Yes | JSON files in ~/.config/bt/agent-prompts/, SHA256-hashed paths |
| Prompt decision logic | prefs.go:125-150 | 26 | N/A | Yes | Yes | ShouldPromptForAgentFile with accept/decline/never-ask states |
| TTY query suppression (init) | tty_guard.go:1-45 | 45 | N/A | Yes | Yes | Sets CI=1 for robot/test/help/version modes to prevent Termenv OSC sequences |
| Agent prompt modal (TUI) | pkg/ui/agent_prompt_modal.go | 271 | N/A | No | Yes | Bubble Tea modal with Yes/No/Never buttons (outside this package) |

## Dependencies

- **Depends on**: Standard library only (`os`, `path/filepath`, `regexp`, `strconv`, `strings`, `fmt`, `runtime`, `crypto/sha256`, `encoding/hex`, `encoding/json`, `time`). Zero third-party dependencies.
- **Depended on by**:
  - `cmd/bt/main.go` - CLI flags `--agents-add`, `--agents-update`, `--agents-remove`, `--agents` (check mode)
  - `pkg/ui/model.go` - TUI startup agent file check flow
  - `pkg/ui/agent_prompt_modal.go` - Uses `agents.AgentBlurb` for preview, `agents.AppendBlurbToFile`/`RecordAccept`/`RecordDecline`
  - `tests/e2e/agents_integration_e2e_test.go` - E2E test suite

## Dead Code Candidates

1. **`AgentFileExists()`** (detect.go:136-144) - Exported function never called outside tests. Only used internally by no one; `DetectAgentFile()` is used instead for all real call sites since it provides richer detection.

2. **`EnsureBlurb()`** (file.go:157-178) - Exported orchestrator function only called from e2e tests, never from production code. The CLI in main.go implements its own orchestration logic with more granular user feedback rather than using this convenience function.

3. **`ContainsBlurb()`**, **`ContainsAnyBlurb()`**, **`ContainsLegacyBlurb()`**, **`GetBlurbVersion()`**, **`NeedsUpdate()`**, **`RemoveLegacyBlurb()`**, **`UpdateBlurb()`** (blurb.go) - These exported string-manipulation functions are only called within the `agents` package itself (internal to file.go/detect.go) or from tests. They are not dead code per se - they are the building blocks called by the file-level operations - but they are over-exported. None are called directly from outside `pkg/agents/` in production code.

4. **`LoadAgentPromptPreference()`** (prefs.go:68-87) - Exported function only called from e2e tests and internally by `ShouldPromptForAgentFile()`. Never called directly from production code outside the package.

5. **`ClearPreference()`** (prefs.go:173-183) - Exported function only called from e2e tests. No production caller.

## Notable Findings

1. **Stale `bv` naming in HTML markers and temp files**: The blurb version markers use `<!-- bv-agent-instructions-v1 -->` and `<!-- end-bv-agent-instructions -->`, and the atomic write creates temp files with prefix `.bv-atomic-*` (file.go:98). These are remnants of the pre-rename era. The HTML comments are embedded in users' AGENTS.md files in the wild, so the `bv-` prefix in markers is effectively a wire format that cannot be changed without a breaking version bump (BlurbVersion 2+). The `.bv-atomic-*` temp file prefix, however, is purely internal and could be renamed to `.bt-atomic-*` without any compatibility concern.

2. **`tty_guard.go` uses package-level `init()`**: This runs on every `bt` invocation, even when the agents package isn't otherwise needed. The guard sets `CI=1` as a process-wide environment variable, which could have side effects on child processes or other libraries that inspect `CI`. This is documented and intentional (preventing Termenv OSC/DSR probe sequences from corrupting robot-mode JSON output), but it's a global side effect from a domain-specific package.

3. **Atomic write on Windows has a TOCTOU race**: The Windows fallback in `atomicWrite` (file.go:136-145) does `os.Remove(filePath)` followed by `os.Rename(tmpPath, filePath)`. If the process crashes or another process writes between the remove and rename, the original file is lost. This is a known limitation of atomic writes on Windows and is documented in the code.

4. **Test-to-source ratio is ~2.6:1**: The package has approximately 2,087 lines of tests for 800 lines of source. Test coverage is thorough, including edge cases for CRLF line endings, unicode content, symlinks, large files, and permission errors. The integration tests cover full accept/decline/never-ask flows.

5. **SKILL.md exists at repo root but has no connection to pkg/agents/**: SKILL.md is a standalone robot-mode guide (frontmatter-based, describing `--robot-*` CLI flags). It is not referenced by or generated from the agents package. The agents package only manages AGENTS.md/CLAUDE.md blurb injection.

6. **`SupportedAgentFiles` includes `agents.md` and `claude.md` (lowercase)**: These are checked as fallbacks after the uppercase variants. The lowercase forms are unusual in practice; most projects use the uppercase convention.

7. **Preference storage uses path hashing without collision handling**: `projectHash()` truncates SHA256 to 8 bytes (16 hex chars). While collision probability is astronomically low for typical use, two different project paths mapping to the same hash would silently overwrite each other's preferences.

8. **The `contains` helper in prefs_test.go is oddly recursive**: The function at prefs_test.go:237-240 uses recursion to check string containment - an unusual pattern that could stack overflow on long strings. It's only used in one test assertion.

## Questions for Synthesis

1. **Should the `bv-` prefix in HTML markers be migrated to `bt-`?** This would require incrementing BlurbVersion to 2 and adding migration logic. The current markers work fine functionally, but they carry the old project name into every user's AGENTS.md file.

2. **Is the `tty_guard.go` init() in the right package?** It protects robot mode JSON output from TTY probe sequences, which is a cross-cutting concern. Currently it fires because `cmd/bt/main.go` imports `pkg/agents`, but if that import were removed, the guard would stop running. It might belong in `cmd/bt/` or a dedicated `pkg/ttycompat/` package.

3. **Should over-exported functions be unexported?** Many string-manipulation functions in blurb.go (`ContainsBlurb`, `GetBlurbVersion`, `NeedsUpdate`, `RemoveLegacyBlurb`, `UpdateBlurb`, etc.) are only used within the package. They could be unexported to reduce the public API surface, though this would require moving some test cases.

4. **Is the agent prompt modal (pkg/ui/agent_prompt_modal.go) still wanted?** It prompts TUI users to inject beadstui instructions into their AGENTS.md on first launch. This is a growth/adoption feature from the original beads_viewer that may or may not align with bt's current direction as a personal tool.

5. **Cross-team**: The CLI integration in `cmd/bt/main.go` (lines ~880-1075) implements its own orchestration flow for `--agents-add`/`--agents-update`/`--agents-remove` rather than using `EnsureBlurb()`. Should the CLI be refactored to use the package's orchestrator, or is the current approach (more granular feedback) preferable?
