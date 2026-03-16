# Audit Report: UI Views & Rendering

**Team**: 1a
**Scope**: pkg/ui/ - visual features (board, tree, graph, insights, history, tutorial, flow matrix, panels, theming)
**Lines scanned**: ~33,286 (production, excluding board.go duplicate in wc count and model.go which is Team 1b scope)

## Architecture Summary

The UI rendering layer in `pkg/ui/` follows a model-per-view pattern where each major view (board, tree, graph, insights, history, flow matrix, tutorial) has its own model struct with `View()` rendering method, navigation methods, and data setters. These are composed into the central `Model` in `model.go` (Team 1b scope) which dispatches rendering based on the current `focus` enum.

Styling is centralized through a three-layer theme system: embedded YAML defaults (`defaults/theme.yaml`), user-level overrides (`~/.config/bt/theme.yaml`), and project-level overrides (`.bt/theme.yaml`). The `Theme` struct holds pre-computed Lipgloss styles and `AdaptiveColor` tokens for light/dark terminals. Package-level `Color*` variables serve as a global bridge so older code paths can reference tokens without passing `Theme` around. Terminal color profile detection (`colorprofile.Detect`) gates TrueColor backgrounds and ANSI256 foregrounds to avoid clashing on limited terminals.

Panel rendering uses `RenderTitledPanel` as the shared primitive for bordered boxes with inlined titles. It supports three border variants (normal/thick/double), center/left title alignment, color overrides, and height/width clamping. This is used consistently across board columns, insights panels, the split detail view, and help overlays. Modals (agent prompt, cass session, update, label picker, recipe picker, repo picker) each render their own centered box using Lipgloss borders directly rather than `RenderTitledPanel`, which creates a subtle visual inconsistency - modal borders use `NormalBorder()` while panels use hand-drawn box-drawing characters.

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| Board (Kanban) | board.go | 1725 | Yes | Yes | Yes | 4-column adaptive layout, swimlane modes (status/priority/type), search, detail panel, empty column toggle. Pre-built BoardState from snapshot. |
| Tree (Hierarchy) | tree.go | 1064 | Yes | Yes | Yes | Parent-child hierarchy, expand/collapse with persistence to `.beads/tree-state.json`, windowed O(viewport) rendering, cycle detection. |
| Graph (Dependencies) | graph.go | 1018 | Yes | Yes | Yes | ASCII art ego-centric graph with blockers/dependents, 8-metric panel (PageRank, Betweenness, etc.), node list sidebar, pre-built GraphLayout from snapshot. |
| Insights | insights.go | 2193 | Yes | Yes | Yes | 10 metric panels (bottlenecks, keystones, influencers, hubs, authorities, cores, cut points, slack, cycles, priority), explanation toggle, calculation proof, heatmap mode. Uses RenderTitledPanel consistently. |
| History | history.go | 3447 | Partial | Yes | Yes | Bead-centric and Git-centric modes, 2/3/4-pane responsive layouts, timeline visualization, file tree panel, search with 5 modes, cass session integration. Depends on `pkg/correlation` not Dolt directly. |
| Tutorial | tutorial.go | 2538 | Yes | Yes | Yes | Full interactive tutorial system with 30 pages across 5 sections, TOC navigation, Glamour markdown rendering, page-level progress tracking, context filtering. |
| Tutorial Content | tutorial_content.go | 941 | N/A | No | Yes | Structured page definitions using component system. 30 pages: Intro(4), Core Concepts(5), Views(8), Advanced(7), Workflows(5), Reference(1). |
| Tutorial Components | tutorial_components.go | 525 | N/A | No | Yes | 15 renderable element types: Paragraph, Section, KeyTable, Tip, StatusFlow, Code, Bullet, Spacer, Divider, Tree, InfoBox, ValueProp, Warning, Note, StyledTable, ProgressIndicator, Highlight. Uses lipgloss/table and lipgloss/tree. |
| Tutorial Progress | tutorial_progress.go | 290 | N/A | Yes | Yes | Singleton manager with mutex-protected persistence to `~/.config/bt/tutorial-progress.json`. Atomic temp-file writes. configHome override for testability. |
| Flow Matrix | flow_matrix.go | 905 | Yes | Yes | Yes | Cross-label dependency dashboard, label list with bar chart, detail panel with blocking power score, drill-down to issues. Own Update() handles keys. |
| Panel System | panel.go | 183 | N/A | Yes | Yes | `RenderTitledPanel` - shared bordered box primitive. 3 variants, center/left title, color overrides, height padding/truncation, runewidth-safe truncation. |
| Theme Loader | theme_loader.go | 342 | N/A | Yes | Yes | 3-layer YAML merge (embedded, user, project). `ApplyThemeToGlobals` + `ApplyThemeToThemeStruct` dual-path for global Color* vars and Theme struct. |
| Theme Types | theme.go | 193 | N/A | Yes | Yes | Theme struct with 30+ color tokens, pre-computed styles, DefaultTheme (Tomorrow Night), terminal profile detection with ThemeBg/ThemeFg fallbacks. |
| Styles | styles.go | 263 | N/A | Yes | Yes | All Color* global variables, badge rendering (status/priority), mini-bar, rank badge, dividers. Well-organized design tokens section. |
| Helpers | helpers.go | 310 | N/A | Yes | Yes | Time formatting (relative/absolute), truncation (runewidth-aware), padding, dependency tree builder/renderer, status/priority icons. |
| Visuals | visuals.go | 178 | N/A | Yes | Yes | Sparkline rendering, heatmap gradient colors (8-level), repo color assignment (hash-based), repo badge. |
| Markdown Renderer | markdown.go | 459 | N/A | Yes | Yes | Glamour wrapper with theme-aware custom StyleConfig. Handles dark/light, width changes, Chroma syntax highlighting mapped to theme palette. |
| Velocity Comparison | velocity_comparison.go | 334 | Yes | Yes | Yes | 4-week velocity table with sparklines, trend indicators (accelerating/decelerating/stable/erratic), per-label rows sorted by avg velocity. |
| Label Dashboard | label_dashboard.go | 252 | Yes | Yes | Yes | Health table with bar chart, blocked count, velocity 7d/30d, stale count. Sorted by health level (critical first). Scrollable. |
| Sprint View | sprint_view.go | 298 | Yes | No (method on Model) | Yes | Sprint dashboard with progress bar, status breakdown, ASCII burndown chart, at-risk items. Rendered as modal overlay. |
| Attention View | attention.go | 55 | Yes | Yes | Yes | Pre-rendered label attention table. Minimal - just formats analysis output. |
| Semantic Search | semantic_search.go | 564 | Yes | Yes | Yes | Vector index integration, hybrid scoring, async computation with cache, debounce. Not a view per se but provides search UI behavior. |
| Agent Prompt Modal | agent_prompt_modal.go | 271 | N/A | Yes | Yes | AGENTS.md enhancement prompt with 3 buttons (yes/no/never), preview box, centered overlay. |
| Cass Session Modal | cass_session_modal.go | 386 | N/A | Yes | Yes | Session correlation display with match reasons, snippets, clipboard copy, cross-platform clipboard support. |
| Update Modal | update_modal.go | 405 | N/A | Yes | Yes | Version update flow with confirm/download/verify/install/success/error states, progress bar, spinner. |
| Label Picker | label_picker.go | 368 | Yes | Yes | Yes | Fuzzy search popup with fzf-style scoring (exact/prefix/contains/subsequence), count display, centered overlay. |
| Recipe Picker | recipe_picker.go | 168 | Yes | Yes | Yes | Simple recipe list with descriptions, centered overlay. |
| Repo Picker | repo_picker.go | 180 | Yes | Yes | Yes | Workspace repo filter with checkbox toggle, select-all, centered overlay. |
| Shortcuts Sidebar | shortcuts_sidebar.go | 332 | N/A | Yes | Yes | Context-aware shortcut panel, scrollable, 8 sections with context filtering. |
| Context Help | context_help.go | 418 | N/A | Yes | Yes | 16 context-specific help content strings (list, graph, board, insights, history, detail, split, filter, label picker, recipe picker, help, time travel, label dashboard, attention, agent prompt, cass session). |
| Snapshot | snapshot.go | 909 | Yes | Yes | Yes | DataSnapshot builder with incremental list rebuilds, tiered performance (small/medium/large/huge), recipe filtering, graph layout precomputation. Not a view but drives all views. |
| Item | item.go | 120 | Yes | Yes | Yes | IssueItem struct implementing bubbles list.Item interface, with triage/graph/search score fields. |
| Workspace Repos | workspace_repos.go | 63 | N/A | Yes | Yes | Repo prefix normalization and formatting helpers. |
| Capslock | capslock.go | 205 | N/A | Yes | Yes | Caps lock detection for warning users. |
| Context | context.go | 284 | N/A | Yes | Yes | Context enum for 16+ view states, used by help and shortcuts systems. |
| Background Worker | background_worker.go | 2092 | Yes | Yes | Yes | Async snapshot building, Dolt polling, incremental updates. Infrastructure, not rendering. |

## Dependencies

- **Depends on**: `pkg/model` (Issue, Status, IssueType, Dependency types), `pkg/analysis` (Insights, GraphStats, LabelHealth, Triage, Velocity, CrossLabelFlow), `pkg/correlation` (HistoryReport, BeadHistory), `pkg/cass` (ScoredResult, CorrelationResult), `pkg/search` (VectorIndex, Embedder, HybridScorer), `pkg/recipe` (Recipe), `pkg/agents` (AgentBlurb), `pkg/updater` (GetLatestRelease, PerformUpdate), `pkg/version`
- **External**: charmbracelet/bubbletea, charmbracelet/lipgloss, charmbracelet/bubbles (viewport, textinput, list), charmbracelet/glamour, charmbracelet/lipgloss/table, charmbracelet/lipgloss/tree, mattn/go-runewidth, gopkg.in/yaml.v3, charmbracelet/colorprofile
- **Depended on by**: `cmd/bt/main.go` (creates Model), any consumer of the public types (DataSnapshot, Theme, IssueItem, etc.)

## Dead Code Candidates

1. **`FlowMatrixView` function** (flow_matrix.go:890-905) - Explicitly marked as "legacy function for backward compatibility". Returns a simple text summary. The interactive `FlowMatrixModel` has replaced it. Check if anything calls it.

2. **`PanelStyle` / `FocusedPanelStyle` package vars** (styles.go:100-110) - These are rebuilt in `ApplyThemeToGlobals` but it is unclear how many call sites still use them vs. using `RenderTitledPanel`. They exist alongside the hand-drawn panel system, creating two border rendering paths.

3. **`BoardModel.searchMode` / `searchQuery` / `searchMatches` / `searchCursor`** (board.go:43-46) and **`waitingForG`** (board.go:49) - Board has its own search and vim combo state. Verify whether these are wired to the main Update loop or orphaned from a refactor.

4. **`TreeViewMode` / `TreeModeBlocking`** (tree.go:160-166) - `TreeModeBlocking` is defined but the comment says "future". Only `TreeModeHierarchy` is used. The `mode` field on `TreeModel` is never set to anything else.

5. **Duplicate icon/color functions** - `GetStatusIcon` (helpers.go:232) uses emoji circles, while `getStatusIcon` (graph.go:877) uses different emoji. `GetPriorityIcon` (helpers.go:247) and `getPriorityIcon` (graph.go:921) use different emoji sets for the same concept. `getTypeIcon` (graph.go:936) and `Theme.GetTypeIcon` (theme.go:176) are near-duplicates with slightly different emoji choices.

6. **`ensureVisible()` on GraphModel** (graph.go:261) - Empty method body. `ScrollLeft()`/`ScrollRight()` (graph.go:258-259) are also empty. Navigation works because `MoveLeft/MoveRight` delegate to `MoveUp/MoveDown`.

7. **`Divider` tutorial component** (tutorial_components.go:240-254) - Defined but not used in any `structuredTutorialPages()` content.

8. **`InfoBox`, `Highlight`, `Warning`, `Note`, `StyledTable`, `ProgressIndicator` tutorial components** - Defined in tutorial_components.go but none appear in the actual tutorial content in tutorial_content.go. They are available but unused.

## Notable Findings

### Strengths

1. **Snapshot architecture is impressive.** DataSnapshot is immutable and thread-safe. SnapshotBuilder precomputes tree roots, board state, graph layout, and triage scores off the UI thread. Tiered performance mode (small/medium/large/huge) skips expensive computations for large datasets. Incremental list rebuilds minimize work on poll updates.

2. **Tutorial system is substantial and well-designed.** 30 pages, 15 renderable component types, persistent progress tracking, Glamour markdown rendering, context-aware page filtering, TOC navigation. The structured component system (`TutorialElement` interface) is a clean design that separates content from presentation.

3. **History view is the most feature-rich view at 3447 lines.** Dual modes (bead-centric / git-centric), responsive layout (2/3/4 panes based on terminal width), timeline visualization with lifecycle events + commits + cass sessions, file tree panel, 5 search modes, compact timeline rendering. This is a serious feature.

4. **Theme system is thorough.** 3-layer YAML merge, adaptive light/dark colors, terminal color profile detection (TrueColor/ANSI256/16-color), per-token fallbacks, pre-computed styles to avoid per-frame allocation. Tomorrow Night palette is well-chosen.

5. **RenderTitledPanel is used consistently** across board columns, insights panels, split view, and help overlays (17 call sites in production code). It handles title truncation, height padding, width clamping, and unicode width correctly.

### Concerns

1. **Stale `bv` references in tutorial content.** `tutorial_content.go` still references `bv` in several places: "Welcome to beadstui" intro mentions "bv brings issue tracking", Quick Start says "You're already running bv!", multiple `bv` references in CLI examples throughout. The rename to `bt`/`bd` was not fully propagated into tutorial text.

2. **Two competing panel rendering approaches.** `RenderTitledPanel` draws hand-crafted box-drawing borders, while modals use Lipgloss `Border(lipgloss.NormalBorder())`. The visual output is similar but not identical - Lipgloss borders handle padding differently and don't support inlined titles. This creates subtle inconsistency.

3. **Duplicate icon/color helper functions.** Graph.go defines private `getStatusIcon`, `getStatusColor`, `getPriorityIcon`, `getTypeIcon` that overlap with public helpers in `helpers.go` and `theme.go`. The emoji choices differ between duplicates (e.g., helpers says open=green circle, graph says open=blue circle). This should be consolidated.

4. **History view complexity.** At 3447 lines, history.go handles data model, navigation, filtering, search, multiple layouts, timeline building, and rendering all in one file. The rendering portion alone (starting at `View()` on line 1257) is ~750 lines spread across many private render methods. This could benefit from extraction.

5. **Tutorial content has naming inconsistencies.** The tutorial content references `bv` and `br` as CLI commands in many places, but the project has renamed to `bt` and `bd`. Examples: "bv --pages", "bd ready" (correct), "br update ID" (stale), "bv --export-pages" (stale). Some pages reference `bt` correctly while others still say `bv`.

6. **Sprint view is a method on Model, not a standalone model.** `renderSprintDashboard()` is defined as a method on `Model` in sprint_view.go, unlike every other view which has its own model struct. It reads `m.selectedSprint`, `m.issues`, `m.sprints` directly. This breaks the pattern and makes it harder to test in isolation.

7. **Board search state appears orphaned.** BoardModel has `searchMode`, `searchQuery`, `searchMatches`, `searchCursor` fields and a `waitingForG` vim combo tracker, but these are managed entirely within board.go. The board has its own full search implementation separate from the main list search, which is reasonable for Kanban but worth verifying is fully wired.

### Hidden Gems

1. **Smart ID truncation** (`smartTruncateID` in graph.go:953-1018) - Abbreviates multi-part IDs by taking first char of non-last parts (e.g., `frontend_auth_login` -> `f_a_login`). Handles separators (`_`, `-`) and edge cases gracefully.

2. **Compact timeline rendering** (`renderCompactTimeline` in history.go:1697) - Single-line visual timeline using Unicode characters: `○──●──├──├──├──✓  5d cycle, 3 commits`. Very clean visualization.

3. **Heatmap gradient system** (visuals.go:74-130) - 8-level color gradient with both foreground and background variants, terminal-profile-aware fallbacks. `GetHeatGradientColorBg` returns matched fg/bg pairs for contrast.

4. **Fuzzy scoring** (label_picker.go:152-207) - fzf-style scoring with exact match (1000), prefix (500+), contains (200+), and subsequence matching with consecutive and word-boundary bonuses.

## Questions for Synthesis

1. **Tutorial content stale references**: How many `bv`/`br` references remain in tutorial_content.go? Is there a tracking bead for this? The rename stream marked "DONE" but tutorial text was apparently not included.

2. **FlowMatrixView legacy function**: Is the standalone `FlowMatrixView()` function at the bottom of flow_matrix.go called anywhere? If not, it can be removed along with the "backward compatibility" comment.

3. **Sprint view as Model method**: Should `renderSprintDashboard` and `handleSprintKeys` be extracted into a `SprintModel` struct to match the pattern of every other view? The current approach mixes sprint state into the main Model.

4. **Board search vs main search**: Is the board's independent search system intentional? It has its own `searchMode`, `searchQuery`, `searchMatches`, and `searchCursor` fields, completely separate from the main list's search infrastructure.

5. **Empty graph navigation methods**: `ensureVisible()`, `ScrollLeft()`, `ScrollRight()` on GraphModel are empty. Are these stubs for planned horizontal scrolling, or dead code from an abandoned feature?

6. **Unused tutorial components**: 8 of 15 tutorial component types (Divider, InfoBox, Highlight, Warning, Note, StyledTable, ProgressIndicator, plus Warning and Note overlap conceptually) are never used in the actual tutorial content. Keep for future content expansion, or trim?

7. **History view size**: At 3447 lines, history.go is the largest view file. Cross-team question: does this warrant splitting into history_model.go (data/navigation) and history_view.go (rendering), or is the current organization acceptable?
