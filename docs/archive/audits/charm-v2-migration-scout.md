# Charm v2 Migration Scout Report

**Date**: 2026-04-03
**Scope**: Research-only assessment of migrating beadstui from Charm v1 to Charm v2
**Codebase**: ~92k production Go, ~102k test Go, 76 files importing Charm libraries

---

## 1. Current Charm Dependency Versions

From `go.mod`:

| Package | Current Version | v2 Target |
|---------|----------------|-----------|
| `github.com/charmbracelet/bubbletea` | v1.3.10 | `charm.land/bubbletea/v2` (v2.0.0) |
| `github.com/charmbracelet/lipgloss` | v1.1.1-0.20250404 (pre-release) | `charm.land/lipgloss/v2` (v2.0.0) |
| `github.com/charmbracelet/bubbles` | v0.21.1-0.20250623 (pre-release) | `charm.land/bubbles/v2` (v2.0.0) |
| `github.com/charmbracelet/glamour` | v0.10.0 | `charm.land/glamour/v2` (v2.0.0) |
| `github.com/charmbracelet/huh` | v0.8.0 | `charm.land/huh/v2` (v2.0.0) |
| `github.com/charmbracelet/colorprofile` | v0.4.1 | Absorbed into lipgloss v2 |

All five packages must be migrated together - they have cross-dependencies (bubbles requires bubbletea v2 + lipgloss v2, huh requires all three, glamour requires lipgloss v2).

---

## 2. Charm Import Inventory

### 2a. Bubble Tea (`bubbletea`) - 30 project files

| Pattern | Count | Files | Notes |
|---------|-------|-------|-------|
| `tea.KeyMsg` type switch | 223 | 22 | **Heaviest usage** - renames to `tea.KeyPressMsg` |
| `tea.Cmd` / `tea.Batch` | 95 | ~10 | `tea.Batch` used 26x in model.go alone |
| `tea.WindowSizeMsg` | 11 | 5 | No API change in v2 |
| `tea.MouseMsg` | 9 | 2 | Splits into 4 message types |
| `tea.NewProgram` | 1 | `cmd/bt/main.go` | Options change to View fields |
| `tea.WithAltScreen()` | 1 | `cmd/bt/main.go` | Moves to `View()` return struct |
| `tea.WithMouseCellMotion()` | 1 | `cmd/bt/main.go` | Moves to `View()` return struct |
| `tea.WithoutSignalHandler()` | 1 | `cmd/bt/main.go` | Check if still exists in v2 |
| `View() string` methods | 16 | 16 | Must return `tea.View` in v2 |
| `msg.String()` key matching | 55 | 10 | v2 equivalent: `msg.Keystroke()` |
| `msg.Type` field access | 1 | 1 (`capslock.go`) | Replaced by `msg.Code` in v2 |
| Imperative commands (`tea.EnterAltScreen`, etc.) | 0 | 0 | Not used - good |
| `tea.Sequentially` | 0 | 0 | Not used |

**Key finding**: The codebase uses `msg.String()` as its primary key handling pattern (55 occurrences across 10 files), not `msg.Type` enum matching. This is mostly compatible with v2's `msg.Keystroke()` method, which should return the same strings for most keys.

### 2b. Lipgloss (`lipgloss`) - 58 project files

| Pattern | Count | Files | Notes |
|---------|-------|-------|-------|
| `lipgloss.AdaptiveColor{}` | 161 | 22 | **Biggest migration burden** - removed in v2 |
| `lipgloss.NewStyle()` (global) | 75 | 10 | Must become renderer-scoped or use compat |
| `r.NewStyle()` / `t.Renderer.NewStyle()` | 475 | 26 | Already renderer-aware - good |
| `.Foreground()` / `.Background()` / `.BorderForeground()` | 398 | 56 | API unchanged, but color types change |
| `.Width()` / `.Height()` / sizing | 89 | 18 | API unchanged |
| `.Bold()` / `.Italic()` / text decoration | 104 | 20 | API unchanged |
| `.Border()` | ~20 | ~10 | API unchanged |
| `lipgloss.Place` / `JoinHorizontal` / `JoinVertical` | 66 | 15 | API unchanged |
| `.Render()` | 619 | 32 | API unchanged |
| `lipgloss.HasDarkBackground()` | 2 | 1 (`markdown.go`) | Signature changes to require explicit I/O params |
| `lipgloss.NoColor{}` | 5 | 2 | Check if API changed |
| `lipgloss.ANSIColor()` | 5 | 2 | API unchanged |
| `lipgloss.Color()` | 4 | 2 | API unchanged |
| `lipgloss.NewRenderer()` | ~50 | ~30 (mostly tests) | v2 may change constructor |
| `lipgloss.DefaultRenderer()` | ~20 | ~15 (mostly tests) | May be removed or changed |
| `*lipgloss.Renderer` on Theme struct | 475+ | 26 | Central to the architecture |

**Key finding**: The Theme struct holds a `*lipgloss.Renderer` and nearly all styling goes through `t.Renderer.NewStyle()` (475 occurrences). This is architecturally aligned with v2's direction of explicit renderer usage. The 75 `lipgloss.NewStyle()` global calls (mostly in `styles.go` and `model_footer.go`) will need to be converted to renderer-scoped calls.

### 2c. Bubbles (`bubbles`) - 15 project files

| Component | Files | Usage Pattern |
|-----------|-------|--------------|
| `bubbles/list` | 8 | `list.Model` on main Model, `list.New()`, custom `IssueDelegate`, `list.FilterState()` (35 refs) |
| `bubbles/viewport` | 6 | `viewport.Model` (3 instances), `viewport.New(w, h)` (7 calls), `.Width`/`.Height`/`.YOffset` direct field access (15 sites) |
| `bubbles/textinput` | 5 | `textinput.Model` (4 instances), `textinput.New()`, style fields |
| `bubbles/key` | - | Imported transitively via list |

**Not used**: spinner, paginator, table, textarea, help, filepicker, progress, timer, stopwatch, cursor (directly).

### 2d. Glamour (`glamour`) - 2 project files

| File | Usage |
|------|-------|
| `pkg/ui/board.go` | `glamour.NewTermRenderer()` with `WithAutoStyle()`, `WithWordWrap()` |
| `pkg/ui/markdown.go` | `glamour.NewTermRenderer()` with `WithStylePath()`, `WithWordWrap()` - multiple renderer creation patterns with fallbacks |

### 2e. Huh (`huh`) - 1 project file

| File | Usage |
|------|-------|
| `pkg/export/wizard.go` | `huh.NewForm()`, `huh.NewGroup()`, `huh.NewConfirm()`, `huh.NewInput()`, `huh.NewSelect()`, `huh.NewOption()`, `huh.ThemeDracula()` |

### 2f. Colorprofile (`colorprofile`) - 2 project files

| File | Usage |
|------|-------|
| `pkg/ui/theme.go` | `colorprofile.Detect()`, `colorprofile.Profile` constants for terminal capability detection |
| `pkg/ui/theme_test.go` | Tests for various `colorprofile.Profile` levels |

---

## 3. Breaking Changes and Impact Assessment

### TIER 1: High Impact (requires careful migration)

#### 3.1 AdaptiveColor Removal
- **What**: `lipgloss.AdaptiveColor{Light: "...", Dark: "..."}` no longer exists in lipgloss v2
- **Impact**: **161 occurrences across 22 files** - this is the single largest migration item
- **Heaviest files**: `styles.go` (52), `theme.go` (52), `theme_loader.go` (4), `tutorial_content.go` (6)
- **Migration path**: Use `compat.AdaptiveColor` package for quick port, or redesign to use `lipgloss.LightDark(isDark)` helper
- **Architecture note**: The entire Theme struct (`theme.go`) is built on AdaptiveColor. The theme_loader system (YAML-based, layered merge) produces AdaptiveColor values. This is a structural dependency, not just a find-replace.
- **Decision needed**: Use compat package (quick but impure) vs redesign to pass `isDark bool` through the theme system (correct but touches everything)

#### 3.2 View() Signature Change
- **What**: `View() string` becomes `View() tea.View`
- **Impact**: **16 View() methods across 16 files** plus the main Model
- **Files**: model_view.go, agent_prompt_modal.go, bql_modal.go, cass_session_modal.go, flow_matrix.go, insights.go, history.go, label_dashboard.go, label_picker.go, repo_picker.go, recipe_picker.go, shortcuts_sidebar.go, tree.go, tutorial.go, velocity_comparison.go, update_modal.go
- **Migration**: Wrap return values in `tea.NewView()` and set declarative fields (AltScreen, MouseMode) on the main Model's View
- **Complexity**: Mostly mechanical wrapping, but the main Model's View needs to set `view.AltScreen = true` and `view.MouseMode = tea.MouseModeCellMotion`

#### 3.3 KeyMsg Rename and Restructure
- **What**: `tea.KeyMsg` becomes `tea.KeyPressMsg`, internal fields change
- **Impact**: **223 type assertions across 22 files**, **55 `msg.String()` calls across 10 files**
- **Migration**: The `case tea.KeyMsg:` to `case tea.KeyPressMsg:` rename is mechanical. The `msg.String()` to `msg.Keystroke()` rename is mechanical. **One exception**: `capslock.go` uses `msg.Type == tea.KeyRunes && len(msg.Runes) == 0` which needs redesign with v2 fields.
- **Risk**: Space bar handling - `case " ":` (1 occurrence in model_keys.go) becomes `case "space":` in v2

### TIER 2: Medium Impact (mostly mechanical)

#### 3.4 Import Path Migration
- **What**: All `github.com/charmbracelet/*` imports move to `charm.land/*/v2`
- **Impact**: **76 files** across pkg/ and cmd/
- **Migration**: Pure find-replace, zero risk
- **Specifics**:
  - `github.com/charmbracelet/bubbletea` -> `charm.land/bubbletea/v2`
  - `github.com/charmbracelet/lipgloss` -> `charm.land/lipgloss/v2`
  - `github.com/charmbracelet/bubbles/*` -> `charm.land/bubbles/v2/*`
  - `github.com/charmbracelet/glamour` -> `charm.land/glamour/v2`
  - `github.com/charmbracelet/huh` -> `charm.land/huh/v2`

#### 3.5 Viewport Field-to-Method Changes
- **What**: `viewport.Width`, `.Height`, `.YOffset` fields become getter/setter methods
- **Impact**: **15 direct field assignments across 8 files**
- **Migration**: `.Width = x` -> `.SetWidth(x)`, `.Height = y` -> `.SetHeight(y)`, reads need `.Width()` method calls
- **Files**: board.go, insights.go, model.go, model_filter.go, tree.go, bql_modal.go, history.go, label_picker.go

#### 3.6 Viewport Constructor Change
- **What**: `viewport.New(w, h)` becomes `viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))`
- **Impact**: **7 call sites** (model.go x3, board.go x1, insights.go x1, model_filter.go x1, tree.go x1)
- **Migration**: Mechanical

#### 3.7 Textinput Style/Field Changes
- **What**: Style fields consolidated into `Styles`/`StyleState` structs, Width field becomes method
- **Impact**: 4 `textinput.Model` instances, 4 `textinput.New()` calls
- **Files**: bql_modal.go, history.go, label_picker.go, model.go
- **Migration**: Check each for style customization (likely minimal)

#### 3.8 List Component Changes
- **What**: `list.DefaultStyles()` now takes `isDark bool`, `list.NewModel()` -> `list.New()`
- **Impact**: Already using `list.New()` (correct). `list.DefaultStyles()` usage needs audit.
- **Files**: model.go, delegate.go, context.go, model_filter.go, model_footer.go, semantic_search.go

#### 3.9 Program Options to View Fields
- **What**: `tea.WithAltScreen()`, `tea.WithMouseCellMotion()` move to `View()` return
- **Impact**: 1 file (`cmd/bt/main.go`) for the program creation, plus the main `View()` method
- **Migration**: Remove options from `tea.NewProgram()`, add fields to `View()` return

#### 3.10 lipgloss.NewStyle() Global Calls
- **What**: Global `lipgloss.NewStyle()` should ideally use a renderer
- **Impact**: **75 occurrences across 10 files**
- **Heaviest files**: `model_footer.go` (39), `styles.go` (8), `model_view.go` (5)
- **Migration**: Convert to use `renderer.NewStyle()` or compat layer

### TIER 3: Low Impact

#### 3.11 MouseMsg Split
- **What**: `tea.MouseMsg` splits into `MouseClickMsg`, `MouseReleaseMsg`, `MouseWheelMsg`, `MouseMotionMsg`
- **Impact**: **9 occurrences across 2 files** (model.go, coverage_extra_test.go)
- **Migration**: Update type switches to handle new message types

#### 3.12 Glamour API Changes
- **What**: Import path change, renderer purity changes
- **Impact**: **2 files** (board.go, markdown.go)
- **Migration**: Update imports, check if `WithAutoStyle()` still exists, may need explicit style selection

#### 3.13 Huh API Changes
- **What**: Import path, theme functions take `isDark bool`, accessible mode changes
- **Impact**: **1 file** (pkg/export/wizard.go)
- **Migration**: Update imports, change `huh.ThemeDracula()` to new theme API

#### 3.14 Colorprofile Absorption
- **What**: `colorprofile` package may be absorbed into lipgloss v2
- **Impact**: **2 files** (theme.go, theme_test.go)
- **Migration**: Update imports to new location

#### 3.15 HasDarkBackground Signature
- **What**: `lipgloss.HasDarkBackground()` now requires explicit I/O params
- **Impact**: **2 calls in 1 file** (markdown.go)
- **Migration**: Pass `os.Stdin, os.Stdout` or use Bubble Tea's `tea.RequestBackgroundColor`

---

## 4. Migration Complexity Summary

### Files Requiring Changes

| Category | Files | Nature |
|----------|-------|--------|
| Import path only | ~10 | Mechanical |
| Import + type rename (KeyMsg) | ~22 | Mechanical |
| Import + View() signature | ~16 | Mechanical wrapping |
| Lipgloss style changes | ~10 | Mechanical (NewStyle -> renderer) |
| AdaptiveColor migration | ~22 | **Architectural decision required** |
| Viewport field->method | ~8 | Mechanical |
| Textinput/list API | ~8 | Mostly mechanical |
| Glamour/Huh/main.go | ~4 | Low complexity |
| Test files | ~40+ | Mirrors production changes |

**Total unique files needing changes**: ~76 (every file that imports Charm)
**Files needing architectural decisions**: ~22 (the AdaptiveColor cluster)
**Files with purely mechanical changes**: ~54

### Estimated Effort Profile

- **Mechanical (find-replace safe)**: ~60% of changes - import paths, KeyMsg->KeyPressMsg, msg.String()->msg.Keystroke(), View() wrapping
- **Mechanical but needs care**: ~25% of changes - viewport field->method, textinput style restructuring, list API updates, NewStyle() scoping
- **Requires design decisions**: ~15% of changes - AdaptiveColor strategy, View() declarative configuration, isDark plumbing

---

## 5. Risks and Unknowns

### High Risk
1. **AdaptiveColor is load-bearing architecture** - The Theme struct, ThemeLoader (YAML config system), styles.go color palette, and all 22 files using AdaptiveColor form a connected system. The compat package provides a bridge but the "right" migration would thread `isDark bool` through the entire theme system. This is the only change that could require redesign rather than refactoring.

2. **Vendor directory** - The project vendors dependencies. The vendor directory will need to be completely regenerated after the v2 migration. All vendored Charm code (~20+ files) will change.

3. **Test suite coupling** - 40+ test files construct themes with `lipgloss.NewRenderer(nil)` or `lipgloss.DefaultRenderer()`. These patterns may change in v2 and every test helper will need updating.

### Medium Risk
4. **msg.String() compatibility** - The v2 `msg.Keystroke()` method should return compatible strings for most keys, but edge cases (space bar, special keys, modifier combinations) need verification. The codebase has 55 msg.String() calls.

5. **list.Model internal behavior** - The list component is deeply integrated (filter state checks, custom delegate, semantic search). Behavioral changes in list v2 could cause subtle bugs.

6. **Glamour style detection** - `glamour.WithAutoStyle()` auto-detects dark/light. If this changes in v2, the markdown rendering pipeline needs rework.

### Low Risk / Unknowns
7. **tea.WithoutSignalHandler()** - Used in main.go for custom signal handling. Need to verify this option exists in v2.
8. **capslock.go** - Uses `msg.Type == tea.KeyRunes` which is a v1-specific API. Small file but needs manual conversion.
9. **colorprofile package** - Import location may change. Small surface area (2 files).
10. **huh.ThemeDracula()** - May be renamed or require isDark parameter. Only 1 file affected.

---

## 6. Recommended Migration Order

### Phase 1: Foundation (do first, unblocks everything)
1. **Lipgloss v2** - Migrate colors, styles, renderer. Decide on AdaptiveColor strategy.
   - Sub-decision: compat package (fast, can refine later) vs full isDark plumbing (correct, higher effort)
   - If using compat: mostly mechanical, ~2 sessions
   - If redesigning: touches theme.go, theme_loader.go, styles.go, and all 22 AdaptiveColor consumers

2. **Bubble Tea v2** - Migrate after lipgloss since bubbletea v2 requires lipgloss v2
   - Import paths, KeyMsg->KeyPressMsg, View() signature, program options
   - The msg.String()->msg.Keystroke() rename across model_keys.go (1073 lines) is the bulk

### Phase 2: Components (after foundation)
3. **Bubbles v2** - Requires both bubbletea v2 and lipgloss v2
   - viewport, textinput, list field/constructor changes
   - list.DefaultStyles(isDark) if not using compat

4. **Glamour v2** - Small surface, low risk
5. **Huh v2** - Single file, low risk

### Phase 3: Cleanup
6. **Remove compat usage** (if used as bridge) - Convert to native v2 patterns
7. **Regenerate vendor directory**
8. **Full test suite verification**

### Why This Order
- Lipgloss is the color/style foundation everything depends on
- Bubble Tea is the framework that bubbles and huh depend on
- Bubbles depend on both, so they come after
- Glamour and Huh are leaf dependencies with small surface areas
- The compat package lets you do lipgloss first without immediately solving the isDark plumbing question

---

## 7. Strategic Recommendation

**Use the compat package as a bridge**. The AdaptiveColor question is the only architecturally interesting decision, and the compat package lets you defer it. Migrate mechanically using compat, get everything compiling and tests passing on v2, then do a focused follow-up to replace compat with native v2 patterns (LightDark helper, isDark plumbing).

This separates the "make it work on v2" effort (mostly mechanical, ~76 files) from the "make it idiomatic v2" effort (architectural, ~22 files for AdaptiveColor + theme redesign).

The codebase is well-positioned for this migration:
- Already uses renderer-scoped styles (`t.Renderer.NewStyle()`) for ~475 of ~550 style creations
- No imperative screen commands (EnterAltScreen etc.) to untangle
- No HighPerformanceRendering usage
- Key handling via `msg.String()` maps cleanly to `msg.Keystroke()`
- Small surface area for glamour (2 files) and huh (1 file)

---

## Sources

- [Bubble Tea v2 Upgrade Guide](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md)
- [Bubbles v2 Upgrade Guide](https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md)
- [Lip Gloss v2: What's New](https://github.com/charmbracelet/lipgloss/discussions/506)
- [Bubble Tea v2: What's New](https://github.com/charmbracelet/bubbletea/discussions/1374)
- [Bubble Tea v2.0.0 Release](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0)
- [Huh v2 PR](https://github.com/charmbracelet/huh/pull/609)
- [Glamour v2 on pkg.go.dev](https://pkg.go.dev/charm.land/glamour/v2)
