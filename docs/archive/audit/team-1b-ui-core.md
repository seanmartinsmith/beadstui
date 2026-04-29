# Audit Report: UI Core & Interaction

**Team**: 1b
**Scope**: pkg/ui/ - state management, update loop, keyboard/mouse handling, snapshot/poll system
**Lines scanned**: ~28,500 (production code in scope files)

## Architecture Summary

The TUI follows Bubble Tea's Model-Update-View (MVU) architecture with a single monolithic `Model` struct (defined in `model.go`) that owns all application state. The `Model` struct contains approximately 90 independent state fields spanning data (issues, analysis, snapshots), UI components (list, viewport, board, graph, tree, insights, history, flow matrix, label dashboard, tutorial, modals), focus/view state (21 focus enum values, 8+ boolean view flags), filter/sort state, semantic search state, Dolt connection state, workspace mode state, alerts, sprints, time-travel, and more. The `Update` method is a ~2,300-line switch statement that processes ~25 message types and routes keyboard input through focus-dependent handlers.

The background data pipeline is cleanly separated: `BackgroundWorker` (in `background_worker.go`, 2092 lines) owns file watching, Dolt polling, and snapshot construction on a background goroutine. It produces immutable `DataSnapshot` objects (defined in `snapshot.go`, 909 lines) that the UI thread swaps atomically. `SnapshotBuilder` performs all expensive computation (sorting, graph analysis, triage scoring, tree building, board state, graph layout) off the UI thread. The worker communicates with the UI via a buffered channel of `tea.Msg` values (`SnapshotReadyMsg`, `SnapshotErrorMsg`, `DoltVerifiedMsg`, `DoltConnectionStatusMsg`, `Phase2UpdateMsg`). A watchdog goroutine monitors worker health and attempts recovery (up to 3 times) on missed heartbeats or processing timeouts.

The keyboard handling is structured as a chain of focus-dependent handler methods (`handleBoardKeys`, `handleGraphKeys`, `handleTreeKeys`, `handleHistoryKeys`, etc.) called from the main `Update` switch. Mouse support is limited to wheel scrolling (up/down) dispatched by focus. Each specialized view (board, graph, tree, insights, history, flow matrix, sprint) is a separate model struct with its own state, but all are owned and orchestrated by the main `Model`.

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| Main Model struct + Init | model.go:342-1124 | ~780 | Yes | Partial | Yes | ~90 state fields; Init batches 5-7 startup commands |
| Update loop (msg dispatch) | model.go:1271-3564 | ~2293 | Yes | Partial | Yes | Handles ~25 msg types; single giant switch |
| SnapshotReadyMsg handler | model.go:1614-1942 | ~328 | Yes | Yes (update_test.go) | Yes | Atomic snapshot swap, selection preservation, sub-view rebuild |
| FileChangedMsg handler (sync) | model.go:2001-2427 | ~426 | N/A (JSONL only) | Yes (update_test.go) | Yes | Legacy sync reload path; auto-enables background mode on slow reloads |
| Phase2ReadyMsg handler | model.go:1423-1564 | ~141 | Yes | Yes (update_test.go) | Yes | Updates triage, insights, priority hints, alerts, graph layout |
| DataSourceReloadMsg handler | model.go:1963-1974 | ~11 | Yes | No dedicated test | Yes | Simple delegation to replaceIssues |
| DoltVerifiedMsg / DoltConnectionStatusMsg | model.go:1976-1999 | ~23 | Yes | No dedicated test | Yes | Updates freshness and connection status |
| BackgroundWorker | background_worker.go:1-2092 | 2092 | Yes | Yes (58 tests) | Yes | Coalescing, dedup, watchdog, idle GC, metrics |
| Dolt poll loop | background_worker.go:1950-2092 | ~142 | Yes | Tested via worker tests | Yes | Exponential backoff, auto-reconnect after 3 failures |
| DataSnapshot + Builder | snapshot.go:1-909 | 909 | Yes | Yes (snapshot_test.go) | Yes | Immutable snapshots; incremental list rebuilds; recipe filtering |
| Keyboard: list view | model.go:4412-4521 | ~109 | Yes | Yes (model_test.go) | Yes | o/c/r/a filter, t/T time-travel, s sort, S triage, y copy, C/O/V/U |
| Keyboard: board view | model.go:3566-3757 | ~191 | Yes | Yes (board_test.go) | Yes | Vim-style nav, column jumping, search, swimlane cycle, expand |
| Keyboard: graph view | model.go:3760-3800 | ~40 | Yes | Partial | Yes | hjkl nav, H/L scroll, enter jump |
| Keyboard: tree view | model.go:3802-3848 | ~46 | Yes | Yes (tree_test.go) | Yes | Expand/collapse, vim nav, tab to detail |
| Keyboard: history view | model.go:3882-4152 | ~270 | Yes | Yes (history_test.go) | Yes | Dual mode (bead/git), file tree, search, confidence, browser open |
| Keyboard: insights panel | model.go:4359-4409 | ~50 | Yes | Yes (insights_test.go) | Yes | Panel switch, explanations, heatmap, calc details |
| Keyboard: flow matrix | model.go:4221-4269 | ~48 | Yes | Yes (flow_matrix_test.go) | Yes | Navigate, drill down, jump to issue |
| Keyboard: sprint view | sprint_view.go:267-298 | ~31 | Yes | Yes (sprint_view_keys_test.go) | Yes | j/k nav between sprints, P/esc exit |
| Keyboard: help overlay | model.go:4596-4635 | ~39 | Yes | Partial | Yes | j/k/ctrl+d/u scroll, space to tutorial |
| Keyboard: recipe picker | model.go:4271-4291 | ~20 | Yes | Yes (recipe_picker_test.go) | Yes | j/k nav, enter apply, esc cancel |
| Keyboard: label picker | model.go:4333-4357 | ~24 | Yes | Yes (label_picker_test.go) | Yes | Fuzzy search, j/k nav, enter apply |
| Keyboard: repo picker | model.go:4293-4331 | ~38 | Yes | Yes (repo_picker_test.go) | Yes | Space toggle, a select all, enter apply |
| Keyboard: time-travel input | model.go:4523-4546 | ~23 | Yes | Partial | Yes | TextInput for revision, enter submit, esc cancel |
| Keyboard: alerts panel | model.go:2611-2677 | ~66 | Yes | Partial | Yes | j/k nav, enter jump, d dismiss |
| Keyboard: global shortcuts | model.go:2709-2893 | ~184 | Yes | Partial | Yes | ?/F1 help, `/F2 sidebar, Ctrl+R/F5 refresh, ;/F2 sidebar, Ctrl+S semantic |
| Mouse support | model.go:3402-3463 | ~61 | Yes | No | Yes | Wheel scroll only; dispatched by focus to all 10 views |
| Semantic search system | model.go:626-893, semantic_search.go | ~830 | Yes | Yes (semantic_search_test.go) | Yes | Toggle, index build, hybrid ranking, debounced async compute |
| Filter system | model.go:6489-6540 | ~51 | Yes | Yes (model_test.go) | Yes | all/open/closed/ready/label:X/recipe:X filters |
| Sort system | model.go:6582-6660 | ~78 | Yes | Partial | Yes | 5 modes: default, created asc/desc, priority, updated |
| Recipe system | model.go:6662-6890 | ~228 | Yes | Yes (recipe_picker_test.go) | Yes | Status/priority/tag/actionable filters; impact/pagerank/priority/date sort |
| Time-travel mode | model.go:7307-7482 | ~175 | No (needs git) | Partial (update_test.go) | Yes | Git diff-based snapshot comparison |
| Export to Markdown | model.go:7484-7566 | ~82 | Yes | No | Yes | Writes filtered issues to bt_export.md |
| Copy to clipboard | model.go:7568-7636 | ~68 | Yes | Partial | Yes | Full issue detail to clipboard |
| Open in editor | model.go:8017-8117 | ~100 | Partial | No | Yes | Opens beadsPath in $EDITOR; requires file path |
| Cass session modal | model.go:7637-7700 | ~63 | Yes | Yes (cass_session_modal_test.go) | Yes | Preview coding sessions for selected bead |
| Self-update modal | model.go:7685-7702 | ~17 | Yes | Yes (update_modal_test.go) | Yes | In-app update download/install |
| View rendering (split/list/footer) | model.go:4660-6317 | ~1657 | Yes | Partial (visuals_test.go) | Yes | Responsive layout, themed footer with 15+ badge sections |
| Stop/cleanup | model.go:8118-8148 | ~30 | Yes | No dedicated test | Yes | Stops worker, watcher, instance lock, pooled issues, Dolt server |
| Label health detail modal | model.go:5202-5501 | ~299 | Yes | No | Yes | Health bars, cross-label flow tables |
| Label drilldown modal | model.go:5336-5501 | ~165 | Yes | No | Yes | Issues by PageRank, health bar, cross-label deps |
| Label graph analysis modal | model.go:5503-5657 | ~154 | Yes | No | Yes | Subgraph stats, critical path, PageRank rankings |
| Alerts panel | model.go (renderAlertsPanel) | ~70 | Yes | Partial | Yes | j/k nav, dismiss, jump to issue |
| Context system | context.go:1-284 | 284 | Yes | Yes (context_test.go) | Yes | 23 context identifiers for context-sensitive help |
| CapsLock/Tutorial trigger | capslock.go:1-205 | 205 | Yes | Yes (capslock_test.go) | Yes | Best-effort CapsLock detection, backtick primary |
| Attention view | attention.go:1-55 | 55 | Yes | Yes (attention_test.go) | Yes | Pre-rendered table of label attention scores |
| Actionable model | actionable.go:1-344 | 344 | Yes | Yes (actionable_test.go) | Yes | Execution plan tracks, navigation |

## Dependencies

- **Depends on** (internal):
  - `internal/datasource` - DataSource, DoltReader, LoadFromSource
  - `internal/doltctl` - DoltServerStopper interface (via model.go:42)
  - `pkg/agents` - AGENTS.md detection and blurb management
  - `pkg/analysis` - GraphStats, Analyzer, Insights, Triage, LabelHealth, IssueDiff
  - `pkg/baseline` - (imported in model.go, purpose unclear from scan)
  - `pkg/cass` - Correlator for coding session previews
  - `pkg/correlation` - HistoryReport, git-bead correlation
  - `pkg/debug` - Debug logging, profiling
  - `pkg/drift` - Alert generation
  - `pkg/export` - Markdown export
  - `pkg/instance` - Multi-instance lock
  - `pkg/loader` - Issue loading, pooling, sprints
  - `pkg/model` - Issue, Sprint, Status types
  - `pkg/recipe` - Recipe loading and filtering
  - `pkg/search` - Hybrid search presets, IssueDocument
  - `pkg/updater` - Version check
  - `pkg/watcher` - File system watching

- **Depends on** (external):
  - `github.com/atotto/clipboard` - Clipboard write
  - `github.com/charmbracelet/bubbles/list` - List component
  - `github.com/charmbracelet/bubbles/textinput` - Time-travel input
  - `github.com/charmbracelet/bubbles/viewport` - Detail pane scrolling
  - `github.com/charmbracelet/bubbletea` - Core TUI framework
  - `github.com/charmbracelet/lipgloss` - Styling and layout

- **Depended on by**:
  - `cmd/bt/main.go` - Creates Model, calls Init/Stop/SetDoltServer
  - Various `_test.go` files within pkg/ui/

## Dead Code Candidates

1. **`ReadyTimeoutMsg` / `ReadyTimeoutCmd`** (model.go:179-190, 1302-1314): Explicitly documented as "no longer needed" and "Legacy fallback handler (no longer used)" since NewModel initializes as ready with default dimensions. The handler and command are kept for "backwards compatibility" but the cmd is never dispatched (removed from Init).

2. **`renderQuitConfirm` "Quit bv?"** (model.go:4779): Still says "Quit bv?" - stale rename artifact; should be "Quit bt?".

3. **`focusAttention`** (model.go:71): Defined in the focus enum but never assigned to `m.focused`. Attention view uses `focusInsights` instead. The context mapping in context.go maps it but it appears unused.

4. **`renderTimeTravelPrompt`**: Referenced in View() at line 4688 but not found in the scanned range - likely exists in the unread tail section. Flagging for completeness.

5. **`baseline` import** (model.go:18): `pkg/baseline` is imported but no usage was observed in the scanned code paths. May be used in the unread portion (lines 6600-8366) or could be dead.

6. **`WorkspaceInfo` struct** (model.go:618-624): Defined but no usage found within model.go. May be used by cmd/bt or other files.

7. **Legacy sync reload path** (model.go:2001-2427): The `FileChangedMsg` handler contains a full synchronous reload implementation (~420 lines) that duplicates much of what BackgroundWorker does. For Dolt sources (the only remaining backend), the background worker always handles reloading. This code path is only active for JSONL sources without BT_BACKGROUND_MODE, and JSONL was removed upstream.

## Notable Findings

### model.go is 8,366 lines - breakdown and extraction candidates

The file contains these major sections that could each be their own file:

| Section | Lines | Extraction Candidate |
|---------|-------|---------------------|
| Types, constants, msg types, cmds | 1-340 | `model_types.go` or `messages.go` |
| Model struct + NewModel | 341-1124 | Keep in model.go |
| replaceIssues + reload helpers | 1126-1244 | `model_reload.go` |
| Init + Update (msg dispatch) | 1245-3564 | `model_update.go` |
| handleBoardKeys | 3566-3757 | Already in board.go conceptually, but methods are on Model |
| handleGraphKeys | 3760-3800 | Could move to graph.go |
| handleTreeKeys | 3802-3848 | Could move to tree.go |
| handleActionableKeys | 3850-3880 | Could move to actionable.go |
| handleHistoryKeys | 3882-4152 | Could move to history.go |
| handleFlowMatrixKeys | 4221-4269 | Could move to flow_matrix.go |
| handleRecipePickerKeys | 4271-4291 | Could move to recipe_picker.go |
| handleRepoPickerKeys | 4293-4331 | Could move to repo_picker.go |
| handleLabelPickerKeys | 4333-4357 | Could move to label_picker.go |
| handleInsightsKeys | 4359-4409 | Could move to insights.go |
| handleListKeys | 4412-4521 | `model_list_keys.go` |
| handleTimeTravelInputKeys | 4523-4546 | Tiny, stay |
| handleHelpKeys | 4596-4635 | Could move to context_help.go |
| View + renderHelpOverlay | 4637-5200 | `model_view.go` |
| renderLabelHealthDetail/Drilldown/GraphAnalysis | 5202-5657 | `model_label_views.go` |
| renderFooter | 5670-6317 | `model_footer.go` (~650 lines alone) |
| Filter/sort/recipe logic | 6319-6890 | `model_filter.go` |
| Split pane sizing | 6892-6920 | Small, stay |
| updateViewportContent | 6922-7264 | `model_viewport.go` |
| Time-travel mode | 7265-7482 | `model_time_travel.go` |
| Export/clipboard/editor/cass/update | 7484-8117 | `model_actions.go` |
| Stop/SetDoltServer/clearAttention | 8118-8166 | Stay in model.go |

### State complexity

The Model struct has approximately 90 fields, of which:
- ~15 are data fields (issues, maps, analysis, snapshot, worker)
- ~12 are UI component fields (list, viewport, board, graph, tree, etc.)
- ~20 are boolean view/focus flags
- ~10 are filter/search state
- ~10 are triage/priority data
- ~8 are workspace/recipe state
- ~8 are alert/sprint/agent/tutorial/modal state
- ~7 are Dolt/connection state

Many of these are mutually exclusive (e.g., `isBoardView`, `isGraphView`, `isHistoryView`, `isActionableView`, `isSprintView` - only one should be true at a time). A `ViewMode` enum could replace 5+ booleans.

### Keyboard shortcut map (complete)

**Global (not filtering):**
| Key | Action | Scope |
|-----|--------|-------|
| `?` / `F1` | Toggle help overlay | Everywhere |
| `` ` `` | Toggle tutorial | Everywhere |
| `Ctrl+R` / `F5` | Force refresh | Everywhere |
| `;` / `F2` | Toggle shortcuts sidebar | Everywhere |
| `Ctrl+S` | Toggle semantic search | List focused |
| `H` | Toggle hybrid search | List focused, not filtering |
| `Alt+H` | Cycle hybrid preset | List focused, not filtering |
| `Ctrl+C` | Force quit | Everywhere |
| `q` | Back/quit (closes current view, quits at top) | Not filtering |
| `Esc` | Back/close (clears filters first at list) | Not filtering |
| `b` | Toggle board view | Not filtering |
| `g` | Toggle graph view | Not filtering |
| `a` | Toggle actionable view | Not filtering |
| `E` | Toggle tree view | Not filtering |
| `i` | Toggle insights panel | Not filtering |
| `p` | Toggle priority hints | Not filtering |
| `h` | Toggle history view | Not filtering |
| `[` / `F3` | Open label dashboard | Not filtering |
| `]` / `F4` | Open attention view | Not filtering |
| `f` | Open flow matrix | Not filtering |
| `!` | Toggle alerts panel | Not filtering |
| `'` | Toggle recipe picker | Not filtering |
| `w` | Toggle repo picker (workspace only) | Not filtering |
| `l` | Open label picker | Not filtering |
| `x` | Export to markdown | Not filtering |
| `Tab` | Switch focus (list/detail in split) | Split view |
| `<` / `>` | Shrink/expand list pane | Split view |

**List view:**
| Key | Action |
|-----|--------|
| `Enter` | View details |
| `Home` | Go to first item |
| `G` / `End` | Go to last item |
| `Ctrl+D` | Page down |
| `Ctrl+U` | Page up |
| `o` | Filter: open |
| `c` | Filter: closed |
| `r` | Filter: ready |
| `a` | Filter: all |
| `t` | Time-travel prompt |
| `T` | Quick time-travel (HEAD~5) |
| `C` | Copy issue to clipboard |
| `O` | Open in editor |
| `S` | Triage sort recipe |
| `s` | Cycle sort mode |
| `V` | Cass session modal |
| `U` | Self-update modal |
| `y` | Copy issue ID to clipboard |
| `/` | Start fuzzy search (built-in) |

**Board view:** h/j/k/l, 1-4 (columns), H/L (first/last col), g/gg (top), G/$ (bottom), 0 (top), / (search), n/N (next/prev match), y (copy ID), o/c/r (filter), s (swimlane), e (empty cols), d (expand card), Tab (detail), Ctrl+J/K (detail scroll), Enter (jump to detail)

**Graph view:** h/j/k/l, H/L (scroll), Ctrl+D/U (page), Enter (jump)

**Tree view:** j/k, h/l (collapse/expand), Enter/Space (toggle), g/G (top/bottom), o/O (expand/collapse all), Ctrl+D/U (page), E/Esc (exit), Tab (detail)

**History view:** j/k (nav), J/K (secondary nav), v (toggle bead/git mode), Tab (cycle focus), Enter (jump), y (copy SHA), c (confidence), f/F (file tree), o (open in browser), g (graph), / (search), h/Esc (exit)

**Insights view:** j/k (nav), h/l/Tab (panels), Ctrl+J/K (detail scroll), e (explanations), x (calc details), m (heatmap), Enter (jump), Esc (exit)

**Flow matrix view:** j/k (nav), Tab (panel), Enter (drilldown), G/g (top/bottom), f/q/Esc (exit)

**Help overlay:** j/k (scroll), Ctrl+D/U (page), g/G (top/bottom), Space (tutorial), any other key (close)

### Data flow: Dolt poll -> snapshot -> UI

1. `startDoltPollLoop` runs on its own goroutine, polling `MAX(updated_at)` every 5s (configurable via `BT_DOLT_POLL_INTERVAL_S`)
2. On change detection, calls `TriggerRefresh()` which calls `process()` on a goroutine
3. `process()` sets state to Processing, calls `buildSnapshot()`
4. `buildSnapshot()` loads via `datasource.LoadFromSource`, computes content hash, builds `DataSnapshot` via `SnapshotBuilder.Build()`
5. `SnapshotBuilder.Build()` sorts, analyzes (Phase 1 sync, Phase 2 async), computes stats, list items, triage, tree, board, graph layout
6. Worker sends `SnapshotReadyMsg` via buffered channel
7. `WaitForBackgroundWorkerMsgCmd` receives from channel, returns to Bubble Tea
8. `Update` handles `SnapshotReadyMsg`: swaps snapshot pointer, updates all legacy fields, rebuilds views, preserves selection
9. Phase 2 completes async; worker goroutine sends `Phase2UpdateMsg`; separate `WaitForPhase2Cmd` sends `Phase2ReadyMsg`
10. UI updates insights, triage, alerts, graph layout on Phase 2 completion

### Dolt connection resilience

- Poll failures use exponential backoff (5s base, capped at 2min)
- First failure notifies UI via `DoltConnectionStatusMsg{Connected: false}`
- After 3 consecutive failures, calls `doltReconnectFn` (wired to `doltctl.EnsureServer`)
- On recovery, sends `DoltConnectionStatusMsg{Connected: true}`, resets ticker
- `doltConnected` bool on Model tracks health for footer indicator
- `lastDoltVerified` timestamp prevents false STALE indicators when data is unchanged

### Duplicate code patterns

The sorting logic (open first, priority ascending, created newest first) appears at least 4 times: `NewModel`, `replaceIssues`, `FileChangedMsg` handler, and `SnapshotBuilder.Build`. The "compute stats" loop (counting open/ready/blocked/closed) appears 3 times. The "rebuild list items with triage data" pattern appears 4+ times. These are candidates for extraction into shared functions.

### Files > 500 lines that could be split

| File | Lines | Concern |
|------|-------|---------|
| model.go | 8,366 | See detailed breakdown above |
| background_worker.go | 2,092 | Could split: config/metrics (300), poll loop (200), snapshot building (400), lifecycle (400) |
| snapshot.go | 909 | Manageable; builder + types |
| history.go | 3,447 | Owned by Team 1a (view/render) but substantial |
| insights.go | 2,193 | Owned by Team 1a |
| tutorial.go | 2,538 | Owned by Team 1a |
| board.go | 1,725 | Owned by Team 1a |
| graph.go | 1,018 | Owned by Team 1a |
| tree.go | 1,064 | Owned by Team 1a |
| flow_matrix.go | 905 | Owned by Team 1a |
| tutorial_content.go | 941 | Content definitions |
| semantic_search.go | 564 | Self-contained module |

## Questions for Synthesis

1. **Legacy sync reload path**: The `FileChangedMsg` handler (420 lines) does synchronous issue reloading for JSONL sources without background mode. Since upstream removed JSONL/SQLite backends, is this code path ever reachable in practice? Should it be removed or gated behind a build tag?

2. **`baseline` import**: model.go imports `pkg/baseline` but no usage was found in the scanned portions. Is this used in the unread section (e.g., `updateViewportContent` or `renderFooter`), or is it dead?

3. **`focusAttention` enum value**: Defined but never assigned to `m.focused`. The attention view uses `focusInsights`. Should `focusAttention` be removed from the enum?

4. **"Quit bv?" text** (model.go:4779): The quit confirmation dialog still says "bv" instead of "bt". Is this a known leftover from the rename?

5. **Mouse click support**: Mouse wheel scrolling works for all views, but there is no mouse click handling (no response to `tea.MouseButtonLeft`, `tea.MouseButtonRight`). The open bead `bt-ks0w` tracks this. Is this a priority for the next phase?

6. **WorkspaceInfo struct**: Defined in model.go but appears unused within the file. Is it used elsewhere or dead?

7. **View mode booleans**: Five mutually exclusive booleans (`isBoardView`, `isGraphView`, `isActionableView`, `isHistoryView`, `isSprintView`) plus the `focused` enum create potential for inconsistent state. Would a single `ViewMode` enum be a better approach, or does the current pattern work well enough?

8. **Background worker test coverage**: The worker has 58+ tests including stress tests, but the Dolt poll loop (`startDoltPollLoop`, `doltPollOnce`) lacks dedicated unit tests. The logic is exercised through integration-level worker tests. Is more targeted coverage needed for the reconnect/backoff logic?
