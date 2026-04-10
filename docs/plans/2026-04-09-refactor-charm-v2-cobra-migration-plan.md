---
title: "Refactor: pkg/ui Decomposition + Charm v2 Migration + Cobra CLI"
type: refactor
status: active
date: 2026-04-09
origin: docs/brainstorms/2026-04-09-product-vision-brainstorm.md
bead: bt-if3w
---

# Refactor: pkg/ui Decomposition + Charm v2 Migration + Cobra CLI

## Deepening Summary

**Deepened on:** 2026-04-09
**Reviewers:** Architecture Strategist, Code Simplicity Reviewer, Performance Oracle

### Key Improvements from Review
1. Split FilterState into FilterState + AnalysisCache (triage data is derived, not filter state)
2. Dropped StatusState (3 fields don't warrant a struct), renamed ModalStack to flat activeModal field
3. Added explicit focus enum disposition (ViewMode + PaneFocus replaces overlapping systems)
4. Fixed Phase 2 acceptance criteria (global `lipgloss.NewStyle()` IS correct in v2)
5. Added cobra flag taxonomy, data-loading architecture, bare-`bt`-launches-TUI test
6. Added pre-computed hot-path styles target for Phase 2 (board: 26/frame, footer: 40/frame)
7. Removed YAGNI: global_ls.go placeholder, model_update_misc.go, optional backward-compat aliases

### Performance Finding
Model decomposition is a net performance win: Model copy shrinks from ~1.6KB to ~240 bytes per frame. No performance blockers identified.

---

Three interleaved refactors executed as one stream. They touch the same files, so doing them separately means touching everything twice.

## Overview

The beadstui codebase inherited a monolithic architecture from beads_viewer. This plan decomposes it into a maintainable, idiomatic Go TUI using Charm v2 best practices while simultaneously migrating from Charm v1 and restructuring the CLI from a 142-flag pflag monolith to cobra subcommands.

**Scope:**
- pkg/ui/model.go (3,626 lines, 200-field Model struct, 2,396-line Update())
- pkg/ui/ (50 production files, ~92k lines, 74 importing Charm v1)
- cmd/bt/main.go (1,695 lines, 142 pflag registrations)
- cmd/bt/robot_*.go (8 files handling CLI dispatch)

**Non-scope:**
- pkg/analysis/, pkg/bql/, pkg/export/, internal/ - these are already well-structured
- Feature work (CRUD, project nav redesign, BQL completion) - separate beads
- Test rewrites beyond what compilation requires

## Problem Statement

1. **model.go is unnavigable.** 200 fields, 2,396-line Update() switch, 7 mutually exclusive boolean view flags that should be an enum. Every change requires understanding the whole file.
2. **Charm v1 is EOL.** 76 files need migration. 161 AdaptiveColor occurrences require theme system redesign. The longer we wait, the harder it gets.
3. **CLI is a flag soup.** 142 flags in one function with no subcommand structure. Adding `bt global ls` or `bt robot bql` is impossible without cobra.
4. **Ownership.** The architecture reflects Jeffrey's design decisions. Refactoring replaces inherited assumptions with intentional ones.

## Technical Approach

### Architecture Target

**State Machine Pattern** (from Charm v2 best practices):
```go
type ViewMode int
const (
    ViewList ViewMode = iota
    ViewBoard
    ViewGraph
    ViewTree
    ViewActionable
    ViewHistory
    ViewSprint
    ViewInsights
)
```

Note: ViewAttention and ViewFlowMatrix are overlays on insights, not distinct navigation targets. They stay as modal types or sub-states of ViewInsights.

**ViewMode replaces AND subsumes the boolean flags AND parts of the `focus` enum.** The existing 22-value `focus` enum overlaps significantly with the boolean flags. After refactor:
- **ViewMode**: primary navigation (which screen am I on)
- **PaneFocus**: secondary state within a view (which pane has keyboard focus - e.g. list vs detail within ViewList)
- **ModalState**: overlay that intercepts input regardless of ViewMode

The 22-value `focus` enum gets split: view-level values (focusBoard, focusGraph, etc.) become ViewMode checks. Pane-level values (focusList, focusDetail) become PaneFocus. Modal-level values (focusHelp, focusBQLQuery, etc.) become ModalState checks.

**Sub-Model Composition** with centralized routing:
```go
type Model struct {
    // Core state
    mode      ViewMode
    pane      PaneFocus       // which pane has keyboard focus within current view
    data      *DataState      // issues, issueMap, snapshot
    filter    *FilterState    // currentFilter, sortMode, BQL, recipes
    analysis  *AnalysisCache  // triage scores, priority hints, counts (derived data)
    dolt      DoltState       // connection health, server lifecycle (value type - 4 fields)
    workspace WorkspaceState  // workspaceMode, activeRepos (value type - 5 fields)

    // Status message (3 fields, flat on Model - too small for a struct)
    statusMsg     string
    statusIsError bool
    statusSeq     uint64

    // Active modal tracking (single-active, not a stack)
    activeModal ModalType

    // View models (each owns its own state)
    board     BoardModel
    graph     GraphModel
    tree      TreeModel
    // ... etc (modal model instances stay here, not inside ModalState)

    // Shared components
    list      list.Model
    viewport  viewport.Model
    theme     Theme
    width, height int
}
```

Key design decisions from review:
- **StatusState dropped** - 3 fields don't warrant a struct, `setTransientStatus()` already encapsulates behavior
- **DoltState and WorkspaceState are value types** (embedded, not pointers) - small structs, avoid unnecessary heap allocation
- **DataState and FilterState are pointers** - larger structs, keep Model copy cheap (~240 bytes vs ~1.6KB)
- **AnalysisCache separated from FilterState** - triage scores, priority hints, and counts are derived analysis data populated from Phase2ReadyMsg, not filter state
- **Modal model instances stay on Model** - ModalState only tracks which modal is active, not the modal data itself
- **activeModal is a flat field** - no Push/Pop (modals never nest), just `Open(t)` and `Close()`

**Message Dispatcher Pattern** (replaces 2,396-line switch):
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Global handlers (always run)
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        return m.handleResize(msg)
    case tea.KeyPressMsg:
        if cmd := m.handleGlobalKeys(msg); cmd != nil {
            return m, cmd
        }
    }

    // Modal intercept (if modal is open, it gets the message)
    if m.modals.Active() {
        return m.modals.Update(msg)
    }

    // Route to active view
    switch m.mode {
    case ViewBoard:
        return m.updateBoard(msg)
    case ViewGraph:
        return m.updateGraph(msg)
    // ...
    }
    return m, nil
}
```

**Cobra CLI Structure**:
```
bt                    # launch TUI (default)
bt global             # launch TUI in global mode
bt global ls          # human-readable table (future)
bt global stats       # aggregate stats (future)
bt robot bql "query"  # robot JSON output
bt robot triage       # robot triage output
bt robot insights     # robot insights output
```

### Implementation Phases

---

## Phase 0: Prep Pass (Mechanical Charm v2)

**Goal:** Get the codebase compiling on Charm v2 with minimal structural changes. Pure find-replace. One commit.

**Effort:** 1 agent session, low risk.

### Steps

1. **Update go.mod**: All `github.com/charmbracelet/*` to `charm.land/*/v2`
2. **Find-replace imports** across all 76 files
3. **Rename message types**: `tea.KeyMsg` -> `tea.KeyPressMsg` (223 occurrences, 22 files)
4. **Fix key comparisons**: `msg.String()` stays (compatible), `" "` -> `"space"` (1 occurrence)
5. **Update View() signatures**: `View() string` -> `View() tea.View` with `tea.NewView()` wrapper (16 methods)
6. **Move program options**: `tea.WithAltScreen()` -> `v.AltScreen = true` in main View()
7. **Update mouse handling**: `tea.MouseMsg` -> `tea.MouseClickMsg` etc. (9 occurrences, 2 files)
8. **Viewport field->method**: `.Width = x` -> `.SetWidth(x)`, `.Width` -> `.Width()` (15 sites, 8 files)
9. **Viewport constructor**: `viewport.New(w, h)` -> `viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))` (7 sites)
10. **Textinput changes**: Check 4 instances for style/field API changes
11. **List changes**: `list.DefaultStyles()` may need `isDark bool` param
12. **Glamour**: Update imports, check `WithAutoStyle()` API (2 files)
13. **Huh**: Update imports, `ThemeDracula()` -> `ThemeDracula(isDark)` (1 file)
14. **Colorprofile**: Check if absorbed into lipgloss v2 (2 files)
15. **Compile, fix errors, run tests**

### What this does NOT touch
- AdaptiveColor (161 occurrences) - deferred to Phase 2 with theme redesign
- Model struct decomposition - deferred to Phase 1

### Important: Renderer removal simplifies things
`*lipgloss.Renderer` is completely removed in lipgloss v2 (not just renamed). This means:
- All 475 `t.Renderer.NewStyle()` calls become `lipgloss.NewStyle()` - a mechanical find-replace
- The `Renderer *lipgloss.Renderer` field on Theme struct gets removed
- All 75 global `lipgloss.NewStyle()` calls are already correct for v2
- This is Phase 0 work (mechanical), not Phase 2 (architectural)

### Acceptance Criteria
- [ ] `go build ./cmd/bt/` succeeds on Charm v2
- [ ] `go test ./...` passes (regenerate golden files as needed)
- [ ] TUI launches and basic navigation works
- [ ] All 16 View() methods return tea.View

---

## Phase 0.5: Test Foundation

**Goal:** Get the test suite to a 27/27 green baseline and document the blast radius before Phase 1 restructures the Model struct.

**Effort:** 1 agent session, low risk. Added 2026-04-10 after gap analysis revealed 2 pre-existing test failures and undocumented blast radius.

**Origin:** [docs/brainstorms/2026-04-10-phase-0.5-test-foundation-brainstorm.md](../brainstorms/2026-04-10-phase-0.5-test-foundation-brainstorm.md)

### Context

Phase 0 (Charm v2) landed successfully but 2 packages fail: `cmd/bt` (4 board tests) and `tests/e2e` (robot contract tests). Both fail because bt's datasource auto-discovery finds the shared Dolt server at `~/.beads/shared-server/dolt-server.port` (priority 110) and uses global mode instead of the JSONL fixtures the tests create in temp dirs (priority 50).

Blast radius analysis shows Phase 1's Model struct changes only affect ~6 test files directly. The other ~45 test files use `NewModel()` factory or public accessor methods and will work unchanged if those APIs are preserved.

### Steps

0.5.1. **Fix e2e Dolt isolation (bt-hpq6)**: Make datasource `DiscoverSources()` skip shared Dolt server probe when `BT_TEST_MODE=1` is set. E2e tests already set this env var. Change in `internal/datasource/global_dolt.go` `DiscoverSharedServer()` - return early if `BT_TEST_MODE=1`. Also check `internal/datasource/source.go` `DiscoverSources()` for the same guard.

0.5.2. **Document test baseline**: Add a test baseline section to this plan (or as a separate doc) recording: all 27 packages green, timing per package, the 6 files Phase 1 will affect, the specific fields accessed in each.

0.5.3. **Verify public accessor methods**: Check that public getter methods exist for every Model field that Phase 1 will move into sub-structs. The critical fields from the blast radius analysis:
- `m.isBoardView` -> needs `IsBoardView()` (may already exist)
- `m.isGraphView` -> needs `IsGraphView()` (may already exist)
- `m.showDetails` -> needs `ShowDetails()` or similar
- `m.currentFilter` -> needs `CurrentFilter()` (may already exist)
- `m.issues` -> needs `Issues()` or `FilteredIssues()` (may already exist)

Add any missing accessors. These are the stable API that keeps 40+ test files working through the refactor.

### What This Does NOT Touch
- Test rewrites or restructuring (post-Phase 1 audit)
- Dolt e2e test infrastructure (bt-kvk0, separate workstream)
- Test suite consolidation or optimization (post-refactor)

### Acceptance Criteria
- [ ] `go test ./...` passes 27/27 packages (excluding known e2e data scale test)
- [ ] cmd/bt tests pass (board robot tests no longer hit global server)
- [ ] tests/e2e robot contract tests pass
- [ ] Blast radius documented (which 6 files, which fields)
- [ ] Public accessor methods verified for all Phase 1 target fields

---

## Phase 1: Model Decomposition

**Goal:** Break the 200-field Model struct and 2,396-line Update() into focused sub-structures with a state machine routing pattern.

**Effort:** 2-3 agent sessions, high context. This is the core refactor.

### Step 1.1: ViewMode Enum

Replace 7 mutually exclusive boolean flags with a single `ViewMode` enum.

**Files:** model.go, model_modes.go, model_view.go, model_keys.go

**Before:**
```go
isSplitView      bool
isBoardView      bool
isGraphView      bool
isActionableView bool
isHistoryView    bool
isSprintView     bool
showDetails      bool
```

**After:**
```go
type ViewMode int
const (
    ViewList ViewMode = iota  // was isSplitView
    ViewBoard                  // was isBoardView
    ViewGraph                  // was isGraphView
    ViewTree
    ViewActionable
    ViewHistory
    ViewSprint
    ViewInsights
    ViewFlowMatrix
    ViewLabelDashboard
    ViewAttention
)
```

Update all `m.isBoardView = true` patterns to `m.mode = ViewBoard`.
Update View() routing from boolean checks to switch on m.mode.
Update key dispatch from boolean checks to switch on m.mode.

### Step 1.2: Extract State Groups

Extract Model fields into focused sub-structs. These stay inside pkg/ui/ (not separate packages yet).

**DataState** (pointer - large, keeps Model copy cheap):
```go
type DataState struct {
    issues       []model.Issue
    pooledIssues []*model.Issue
    issueMap     map[string]*model.Issue
    analyzer     *analysis.Analyzer
    analysis     *analysis.GraphStats
    beadsPath    string
    dataSource   *datasource.DataSource
    watcher      *watcher.Watcher
    instanceLock *instance.Lock
    snapshot     *DataSnapshot
    snapshotInitPending bool
    backgroundWorker    *BackgroundWorker
    workerSpinnerIdx    int
    lastForceRefresh    time.Time
}
```

**FilterState** (pointer - medium size, filter/sort/search concerns):
```go
type FilterState struct {
    currentFilter string
    sortMode      SortMode
    semantic      *SemanticSearchState
    activeRecipe  *recipe.Recipe
    recipeLoader  *recipe.Loader
    bqlEngine     *bql.MemoryExecutor
    activeBQLExpr *bql.Query
}
```

**AnalysisCache** (pointer - derived data from Phase2ReadyMsg, NOT filter state):
```go
type AnalysisCache struct {
    countOpen     int
    countReady    int
    countBlocked  int
    countClosed   int
    triageScores  map[string]float64
    triageReasons map[string]analysis.TriageReasons
    unblocksMap   map[string][]string
    quickWinSet   map[string]bool
    blockerSet    map[string]bool
    priorityHints map[string]*analysis.PriorityRecommendation
    showPriorityHints bool
}
```

**DoltState** (value type, embedded - only 4 fields):
```go
type DoltState struct {
    lastDoltVerified time.Time
    doltConnected    bool
    doltServer       DoltServerStopper
    doltShutdownMsg  string
}
```

**WorkspaceState** (value type, embedded - only 5 fields):
```go
type WorkspaceState struct {
    workspaceMode    bool
    availableRepos   []string
    activeRepos      map[string]bool
    workspaceSummary string
    currentProjectDB string
}
```

**StatusState dropped** - 3 fields (`statusMsg`, `statusIsError`, `statusSeq`) stay flat on Model. `setTransientStatus()` already encapsulates the behavior.

Consider embedding DoltState and WorkspaceState to minimize access-pattern churn (`m.doltConnected` still works vs `m.dolt.doltConnected`). Check for field name collisions before choosing.

### Step 1.3: Modal State (Single-Active, Not a Stack)

Replace 19 `show*` booleans with a single `activeModal ModalType` field.

```go
type ModalType int
const (
    ModalNone ModalType = iota
    ModalHelp
    ModalQuitConfirm
    ModalRecipePicker
    ModalBQLQuery
    ModalLabelPicker
    ModalRepoPicker
    ModalTimeTravelInput
    ModalAgentPrompt
    ModalTutorial
    ModalCassSession
    ModalUpdate
    ModalAlerts
    // Label drill-down chain: these are flat transitions, not nested
    ModalLabelHealthDetail
    ModalLabelDrilldown
    ModalLabelGraphAnalysis
)
```

**No ModalStack struct.** The `activeModal` field lives directly on Model. Modals never nest - opening one closes any previous one. Label drill-downs (detail -> drilldown -> graph analysis) are flat transitions: `m.activeModal = ModalLabelDrilldown` replaces `ModalLabelHealthDetail`.

**Modal model instances stay on Model** (not grouped into a container). The existing `bqlQuery BQLQueryModal`, `recipePicker RecipePickerModel`, etc. fields remain where they are. `activeModal` just tracks which one is visible.

```go
func (m Model) modalActive() bool { return m.activeModal != ModalNone }
func (m *Model) openModal(t ModalType) { m.activeModal = t }
func (m *Model) closeModal()           { m.activeModal = ModalNone }
```

### Step 1.4: Decompose Update()

Split the 2,396-line switch into focused handler methods. Each handles one category of messages.

**New files:**
- `model_update_data.go` - SnapshotReadyMsg, SnapshotErrorMsg, DataSourceReloadMsg, FileChangedMsg, DoltVerifiedMsg, DoltConnectionStatusMsg
- `model_update_analysis.go` - Phase2ReadyMsg, Phase2UpdateMsg, SemanticIndexReadyMsg, HybridMetricsReadyMsg, SemanticFilterResultMsg, statusClearMsg, UpdateMsg, HistoryLoadedMsg, AgentFileCheckMsg, workerPollTickMsg (small handlers grouped by "system events")
- `model_update_input.go` - tea.KeyPressMsg routing, tea.MouseClickMsg, tea.WindowSizeMsg

No `model_update_misc.go` - "misc" is a non-name. Small handlers absorb into the system events file or stay in model.go alongside the thin router.

The main Update() becomes a thin router:
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Activity recording
    if m.data.backgroundWorker != nil {
        switch msg.(type) {
        case tea.KeyPressMsg, tea.MouseClickMsg:
            m.data.backgroundWorker.recordActivity()
        }
    }

    // Modal intercept
    if m.modals.Active() {
        return m.updateModal(msg)
    }

    // Dispatch by message type
    switch msg := msg.(type) {
    case SnapshotReadyMsg:
        return m.handleSnapshot(msg)
    case tea.KeyPressMsg:
        return m.handleKeyPress(msg)
    case tea.WindowSizeMsg:
        return m.handleResize(msg)
    // ... one line per message type, delegating to handler methods
    }

    return m, nil
}
```

### Step 1.5: Extract Footer as Component

`model_footer.go` is 719 lines reaching into 15+ Model fields. Extract into a proper footer component that receives a `FooterData` struct rather than accessing Model directly. Don't convert style calls to renderer-scoped yet - Phase 2 removes the renderer entirely, so that would be double work.

### Step 1.6: Test Migration

Blast radius analysis (Phase 0.5) found only ~6 test files need explicit updates. The other ~45 use `NewModel()` factory or public accessor methods and will work unchanged.

**Files that need updates** (direct field access or struct literals):
- `sprint_view_keys_test.go` - 17 `Model{}` struct literals (must use NewModel or builder)
- `coverage_extra_test.go` - 29 `NewModel()` + ~12 direct field assignments (`m.currentFilter`, `m.isGraphView`, etc.)
- `context_test.go` - 7 uses of custom `newTestModel()` helper + 5 direct field accesses
- `snapshot_test.go` - 3 direct field reads (`m.currentFilter`, `m.issues`)
- `tree_bench_test.go` - 1 direct `m.issues` read
- `triage_preservation_test.go` - 2 direct `m.issues` reads

**Strategy:**
- Extract incrementally (one sub-struct at a time), running tests after each
- `NewModel()` is the test factory (91 calls across 10 files) - keep its signature stable
- Public accessor methods added in Phase 0.5: `ShowDetails()`, `CurrentFilter()`, `Issues()` plus pre-existing `IsBoardView()`, `IsGraphView()`, `FilteredIssues()`
- For the 6 affected files: update direct field access to use accessors, or add setter methods where tests need to set up state
- No separate `testModel()` helper needed - `NewModel()` already serves this role

See: `docs/audit/test-baseline-2026-04-10.md` and `docs/brainstorms/2026-04-10-phase-0.5-test-foundation-brainstorm.md`

### Acceptance Criteria
- [ ] Model struct has ViewMode enum (no boolean view flags)
- [ ] `focus` enum split into ViewMode + PaneFocus (document valid pairs)
- [ ] Model struct uses sub-state structs (DataState, FilterState, AnalysisCache)
- [ ] `activeModal ModalType` replaces 19 show* booleans
- [ ] Update() is under 100 lines (routing only)
- [ ] Handler methods are in separate files by concern
- [ ] All tests pass (test helper for Model construction)
- [ ] TUI behavior unchanged

---

## Phase 2: Theme System Redesign + AdaptiveColor Kill

**Goal:** Replace 161 AdaptiveColor occurrences with Charm v2's `LightDark()` pattern. Redesign the theme system to be v2-native.

**Effort:** 1-2 agent sessions. Concentrated in theme.go, styles.go, theme_loader.go.

### Steps

2.1. **Add dark mode detection** via `tea.BackgroundColorMsg` (the canonical v2 pattern):
```go
func (m Model) Init() tea.Cmd {
    return tea.Batch(
        tea.RequestBackgroundColor,
        // ... existing init commands
    )
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case tea.BackgroundColorMsg:
        m.theme.IsDark = msg.IsDark()
        // Rebuild all styles with new isDark
        m.theme.Resolve()
        // Update Bubbles component styles
        m.list.Styles = list.DefaultStyles(m.theme.IsDark)
}
```

This replaces both `lipgloss.HasDarkBackground()` and `colorprofile.Detect()`. Enables runtime light/dark switching if the terminal changes.

2.2. **Redesign Theme struct** to hold resolved colors (not AdaptiveColor):
```go
type Theme struct {
    IsDark   bool
    Renderer *lipgloss.Renderer

    // Resolved color tokens (no AdaptiveColor)
    Primary    lipgloss.Color
    Secondary  lipgloss.Color
    Text       lipgloss.Color
    TextMuted  lipgloss.Color
    Bg         lipgloss.Color
    BgSubtle   lipgloss.Color
    Border     lipgloss.Color
    // ... all 30+ tokens
}
```

2.3. **Update theme_loader.go** to resolve colors at load time based on isDark, not produce AdaptiveColor values.

2.4. **Kill all 161 AdaptiveColor occurrences** across 22 files. Replace with theme token references.

2.5. **Remove `*lipgloss.Renderer` from Theme struct entirely.** The Renderer type is completely gone in lipgloss v2. All 475 `t.Renderer.NewStyle()` calls become plain `lipgloss.NewStyle()`. This is actually simpler than it sounds - massive find-replace plus removing the Renderer field. Color downsampling now happens at the output layer automatically in Bubble Tea, or via `lipgloss.Sprint()` for robot mode CLI output.

2.6. **Update Glamour calls**: `WithAutoStyle()` is removed in v2. Replace with `WithStandardStyle("dark")` / `WithStandardStyle("light")` using isDark from BackgroundColorMsg. Affects `board.go` and `markdown.go`.

2.7. **Update Bubbles component styles**: `list.DefaultStyles(isDark)`, `textinput.DefaultStyles(isDark)`, `help.DefaultStyles(isDark)` all need the isDark bool now.

2.8. **Robot mode output**: For non-TUI output (robot JSON, future human CLI), color downsampling is no longer automatic in `Style.Render()`. Must use `lipgloss.Sprint()` or `lipgloss.Println()` for standalone styled output.

2.9. **Consolidate duplicate helpers**: `GetStatusIcon` (helpers.go) vs `getStatusIcon` (graph.go) - pick one.

2.7. **Consolidate panel rendering**: Choose one approach (RenderTitledPanel hand-drawn vs Lipgloss NormalBorder). Standardize.

### Step 2.10: Pre-compute Hot Path Styles

During the theme redesign, extend pre-computed styles beyond the current Theme delegate styles. Target:
- Board `renderCard` path: 26 style constructions per frame -> pre-compute ~6-8 card style variants on Theme (selected, blocking, search-match, etc.)
- Footer render path: 40 style constructions -> pre-compute stable footer styles on Theme
- This eliminates ~400+ style constructions per frame in board view (bounded by visible cards, not total issues)

### Acceptance Criteria
- [ ] Zero `lipgloss.AdaptiveColor` in codebase
- [ ] `*lipgloss.Renderer` removed from Theme struct (gone in v2)
- [ ] `lipgloss.NewStyle()` is the correct global pattern in v2 (no renderer-scoping needed)
- [ ] Theme struct holds resolved colors based on isDark bool
- [ ] Dark/light mode detected via tea.BackgroundColorMsg
- [ ] Theme YAML loader produces resolved colors
- [ ] One panel rendering approach throughout
- [ ] Hot-path styles (board cards, footer) pre-computed on Theme

---

## Phase 3: Cobra CLI Migration

**Goal:** Replace 142-flag pflag monolith with cobra subcommand architecture.

**Effort:** 1-2 agent sessions. Mechanical but wide.

### Target Structure

```
cmd/bt/
  root.go           # root command (no subcommand = TUI)
  global.go          # bt global (TUI in global mode)
  global_ls.go       # bt global ls (future, placeholder)
  robot.go           # bt robot (parent for robot subcommands)
  robot_triage.go    # bt robot triage (existing --robot-triage)
  robot_insights.go  # bt robot insights
  robot_bql.go       # bt robot bql "query"
  robot_graph.go     # bt robot graph
  robot_history.go   # bt robot history
  robot_search.go    # bt robot search
  robot_sprint.go    # bt robot sprint
  robot_alerts.go    # bt robot alerts
  robot_labels.go    # bt robot labels
  export.go          # bt export (markdown, pages, graph)
  agents.go          # bt agents (add/remove/update AGENTS.md)
  baseline.go        # bt baseline (save/check/drift)
  version.go         # bt version
  helpers.go         # shared infrastructure
```

### Steps

3.1. **Add cobra dependency**, scaffold root.go with `bt` as root command.

3.2. **Root command** (no args = TUI): Preserve current behavior. `bt` alone launches TUI. `bt -g` or `bt --global` launches global TUI.

3.3. **Extract robot subcommands**: Each existing `robot_*.go` becomes a cobra subcommand under `bt robot`. Flags move from global to per-subcommand. For example:
```
bt robot triage --by-track --by-label
bt robot bql "status=open AND priority<P2" --format json
bt robot graph --format dot --root bt-ssk7 --depth 3
```

3.4. **Extract export subcommands**: `bt export md`, `bt export pages`, `bt export graph`.

3.5. **Global subcommand**: `bt global` launches TUI in global mode. No placeholder files for future subcommands (YAGNI - add `bt global ls` when it's built, not before).

3.6. **Backward compatibility**: Optional. This is a pre-alpha tool with one robot-mode consumer (the beads plugin, also authored by the maintainer). A clean break with a changelog entry may be simpler than maintaining hidden aliases + deprecation warnings. Decide at implementation time based on effort.

3.7. **Short flags**: `-g` for `--global`, `-q` for `--quiet`, `-v` for `--verbose`.

3.8. **Flag taxonomy**: Classify all 142 flags as persistent (inherited by subcommands) vs local (per-command):
- Persistent on root: `--format`, `--quiet`, `--verbose`, `--global`, `--repo`
- Local to `bt robot *`: all analysis-specific flags (--by-track, --by-label, --graph-format, etc.)
- Local to `bt export *`: export-specific flags (--pages-title, --no-hooks, etc.)

3.9. **Data loading architecture**: Use `PersistentPreRunE` on root command to load issue data into a shared context struct. Subcommands access loaded data from cobra command context. This replaces the current pattern where main.go loads data once then branches.

3.10. **Critical test**: Bare `bt` with no args must launch TUI, not show help. Requires `rootCmd.Run` (not just `RunE`) and `rootCmd.SilenceUsage = true`.

### Acceptance Criteria
- [ ] `bt` launches TUI (not help)
- [ ] `bt -g` launches global TUI
- [ ] `bt robot triage` produces same JSON as old `bt --robot-triage`
- [ ] `bt robot bql "query"` works
- [ ] `bt --help` shows subcommand structure
- [ ] main.go is under 100 lines
- [ ] Flag taxonomy documented (persistent vs local per subcommand)

---

## Opportunistic Fixes (During Refactor)

These are small, well-scoped fixes from the 2026-04-09 security audit that touch files we're already modifying:

- **bt-8frs** (P2): Windows `cmd /c start` URL injection in `model_export.go` - 3 lines. Fix during Phase 1 when extracting export logic.
- **bt-lgpg** (P2): BQL parser recursion depth limit in `pkg/bql/parser.go` - 10 lines. Fix during Phase 0 or 1 since BQL is touched for filter decomposition.
- **bt-gk83** (P3): Parameterize `dateToSQL` in `pkg/bql/sql.go` - dead code (SQLBuilder not wired up). Fix when DoltExecutor work starts, not now.

See: `security/260409-2034-stride-owasp-full-audit/overview.md` for full audit.

## Phase 4: Cleanup Pass

**Goal:** Remove dead code, fix stale references, update tests.

**Effort:** 1 agent session, low risk.

### Steps

4.1. **Remove dead code**: Sprint view pattern violation (method on Model instead of standalone), orphaned board search state, any code made unreachable by refactor.

4.2. **Fix stale references**: `bv-` prefixes in comments, old CLI names in tutorial_content.go (`bv`, `br`).

4.3. **Update test golden files**: Regenerate any snapshot tests broken by Charm v2 output changes.

4.4. **Run full test suite**: `go test ./... -timeout 300s`. All 27 packages, 268 files must pass.

4.5. **Build verification**: `go build ./cmd/bt/` and smoke test TUI + robot commands.

### Acceptance Criteria
- [ ] Zero `bv-` references in non-historical contexts
- [ ] No dead code flagged by `go vet` or `staticcheck`
- [ ] Full test suite passes
- [ ] Binary builds and runs on Windows

---

## Risk Analysis & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Phase 0 breaks something subtle | Medium | Compile + full test suite before any structural changes |
| Phase 1 state decomposition misses a coupling | High | Extract incrementally, test after each sub-struct extraction |
| AdaptiveColor kill touches 22 files | Medium | One file at a time, test after each |
| Cobra migration breaks robot mode consumers | High | Keep old flags as hidden aliases, deprecation warnings |
| Background worker race conditions during refactor | High | Don't change worker logic, only move it. Snapshot pattern is already safe. |
| Test golden files need mass regeneration | Low | Expected. Regenerate once after Phase 0, once after Phase 2. |

## Dependencies & Prerequisites

**Blocks:**
- bt-oiaj (CRUD) - needs the refactored Model + cobra to add write commands
- bt-zta9 (Charm v2 planning) - subsumed by this plan
- bt-lt2h (human CLI output) - needs cobra subcommand structure

**Blocked by:** Nothing. This is unblocked and ready to start.

**Related:**
- bt-s4b7 (project nav redesign) - can proceed in parallel, feeds into Phase 3
- bt-tkhq (keybinding research) - informs Phase 1 key dispatch redesign
- bt-y0k7, bt-ihm0 (poll flash bugs) - may be naturally fixed by Phase 1 Update() decomposition

## Execution Strategy

**Sequential through Phase 1, then parallel:**
- Phase 0 (Charm v2 mechanical) - **DONE** (2026-04-10, commit 4348e829)
- Phase 0.5 (test foundation) - next, establishes 27/27 green baseline
- Phase 1 (Model decomposition) - sequential after 0.5, core refactor
- **Post-Phase 1 test audit** - optimize test suite for the new structure
- Phase 2 (Theme/AdaptiveColor) + Phase 3 (Cobra CLI) - can run in parallel after Phase 1
- Phase 4 (cleanup) - last, after everything lands

**Each phase is a git branch + PR.** Don't accumulate all four phases into one giant changeset.

**Each phase ends with green tests.** No "we'll fix the tests later." Phase 0.5 establishes the green baseline; subsequent phases maintain it.

**Agent parallelism:** Phase 3 (cobra, touches `cmd/bt/`) can run in parallel with Phase 2 (theme, touches `pkg/ui/` styles). Phases 0 -> 0.5 -> 1 are strictly sequential.

## Sources & References

### Origin
- **Brainstorm document:** [docs/brainstorms/2026-04-09-product-vision-brainstorm.md](../brainstorms/2026-04-09-product-vision-brainstorm.md) - Key decisions: combined refactor+Charm v2 stream, proper Go best practices, cobra migration, project-first global mode.

### Internal References
- Charm v2 scout report: [docs/audit/charm-v2-migration-scout.md](../audit/charm-v2-migration-scout.md)
- UI views audit: [docs/audit/team-1a-ui-views.md](../audit/team-1a-ui-views.md)
- UI core audit: [docs/audit/team-1b-ui-core.md](../audit/team-1b-ui-core.md)
- CLI audit: [docs/audit/team-2-cli.md](../audit/team-2-cli.md)
- Model struct: `pkg/ui/model.go:315-514`
- Update() method: `pkg/ui/model.go:1152-3547`
- Key dispatch: `pkg/ui/model_keys.go`
- Footer rendering: `pkg/ui/model_footer.go`
- Theme system: `pkg/ui/theme.go`, `pkg/ui/theme_loader.go`, `pkg/ui/styles.go`
- CLI entry: `cmd/bt/main.go`

### External References
- Charm v2 migration: charm-tui-design plugin MIGRATION.md
- Bubble Tea architecture patterns: charm-tui-design plugin BUBBLETEA.md
- TUI design principles: charm-tui-design plugin DESIGN-PRINCIPLES.md
- Cobra CLI framework: github.com/spf13/cobra

### Related Beads
- bt-if3w: Refactor pkg/ui monolith + Charm v2 migration (this plan's tracking bead)
- bt-zta9: Charm v2 migration planning (subsumed)
- bt-53du: Product vision epic (parent)
