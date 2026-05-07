---
title: "ADR-004: bubbles/v2/key adoption for unified key dispatch and help surfaces"
status: proposed
date: 2026-05-07
decision-makers: [seanmartinsmith]
parent: 002-stabilize-and-ship.md
epic-bead: bt-ift6
spike-bead: bt-ift6.0
foundation: docs/audits/domain/2026-04-23-keybindings-audit.md
related-beads: [bt-xavk, bt-oiaj, bt-dx7k, bt-ktcr]
---

# ADR-004: bubbles/v2/key adoption for unified key dispatch and help surfaces

## Status

**Proposed (2026-05-07).** Awaiting user signoff. Once accepted, gates 13 child issues under bt-ift6 (foundation, spine, parallel fan-out, surface rewires, cleanup). The spike branch `bt-ift6.0-spike` carries this ADR plus a worked example; the worked example is reference material, not merged to main. The ADR itself is cherry-picked onto main as part of bt-ift6.1.

### v1 — revisit during UX unification pass

bt is an inherited project still mid-unification. This ADR locks in **the data shape** (the framework, the per-view Map structure, the consumer-side composition pattern). The **specific contents** of the maps — which keys, which help text, how universal-nav is assigned, when bindings get disabled — are explicitly v1 and expected to evolve as the broader UX audit progresses (bt-xavk, bt-w8j8, future view audits). Sections marked **[v1 — revisit]** below are correct *for the current state of bt* but should be re-examined when the UX pass settles. Sections without that marker are load-bearing structural decisions that don't expect to move.

## Context

bt is on Charm v2 (`charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`) but does not import `charm.land/bubbles/v2/key`. Key dispatch is hand-rolled `switch msg.String()` in per-view handlers in `pkg/ui/model_keys.go`; help text is independently declared in `pkg/ui/shortcuts_sidebar.go` (the `;` sidebar) and `pkg/ui/model_view.go`'s `renderHelpOverlay` (the `?` overlay). The two surfaces have demonstrably drifted — see [`docs/audits/domain/2026-04-23-keybindings-audit.md`](../audits/domain/2026-04-23-keybindings-audit.md), which inventories the drift on 2026-04-23 and re-confirmed it on 2026-05-07. Most recently, bt-ktcr's `S/R` swap landed in the `?` overlay only and left the `;` sidebar showing the old binding, three weeks after the audit predicted exactly that failure mode.

bt-xavk's L1.5 reframe (2026-05-07) widens the surface count from two to three: status-bar L1 hints, the persistent `;` sidebar (L1.5), and the on-demand `?` overlay (L2). Hand-maintained string tables across three files is not a tractable target. The right fix is structural: a single source of truth that all three surfaces consume.

`bubbles/v2/key` provides the substrate. `key.Binding` carries both dispatch keys and `Help{Key, Desc}` text. `key.Map` is an interface returning `ShortHelp() []key.Binding` and `FullHelp() [][]key.Binding`. Per-view `key.Map` structs make per-view scoping intrinsic instead of metadata-driven. Help surfaces become consumers, not parallel string tables.

**The structural claim, narrowed honestly.** Adopting `key.Map` eliminates **binding-to-help-listing drift** (the audit's class of failure: a binding's keys change but the help table forgets). It does not eliminate every drift class:

- *Within-binding drift* — `WithKeys("up","k")` paired with `WithHelp("↑/j",...)` typo — survives. Mitigated by code review and the help-rendering being visually scannable; not eliminated.
- *Field-vs-help-method drift* — a new `key.Binding` field added to a Map struct without being included in `FullHelp()` returns dispatches but never appears in help — was the audit's *exact* shape just relocated. Eliminated by the reflection-based tests landed alongside this epic (see Decision 6).
- *Map-vs-registry drift* — a new view Map added to `AppKeys` but forgotten in the help-test registry — surfaces as silent test passes. Eliminated by reflection-based registry validation (see Decision 6).

Net: the *recurring* drift class the audit predicted is structurally eliminated. The remaining drift surfaces are smaller, locally visible, and either tooling-defended or code-review-defended.

This ADR settles the load-bearing semantic decisions before scaffolding lands, so 13 children rest on documented choices instead of discovered-during-implementation assumptions.

## Decision

**Adopt `charm.land/bubbles/v2/key` as the canonical dispatch and help substrate for bt's TUI.** The framework already exists transitively in bt's dependency graph (used by `bubbles/list`, `bubbles/textinput`, etc.); this ADR makes the adoption explicit at the application layer.

Six load-bearing semantic decisions are answered below. Where the framework's behavior is the answer, the answer is grounded in `charm.land/bubbles/v2@v2.1.0/key/key.go` and `.../help/help.go` directly, not assumed.

## Decision points

### 1. Composition semantics — how Global and per-view Maps compose

**Decision (structural, locked): declarative separation; consumer concatenates.** Global keys live in `GlobalKeys`. Per-view keys live in their own Map (`ListKeys`, `BoardKeys`, `GraphKeys`, etc.). Neither Map embeds or inherits the other. Help surfaces and the dispatcher each compose them at the consumption point.

**Dispatch layer architecture.** `handleKeyPress` in `model_update_input.go` is not a flat switch; it is six ordered layers. `bubbles/v2/key` adoption participates at specific layers, leaves others alone:

| Layer | Today's role | After this epic |
|---|---|---|
| 1. Modal early-returns (`:62-555`) | A modal owns dispatch absolutely while open. | Unchanged structurally. Modal handlers consume their own Map (see Decision 4). |
| 2. Special-chord routing (`:558-759`) | `?`/`;`/`Ctrl+R`/`F5`/`Ctrl+S`/`H` etc., intentionally pre-empts the filter-state guard so they work mid-filter. | These bindings move INTO `GlobalKeys`. `key.Matches(msg, m.keys.Global.X)` runs at this layer. |
| 3. Filter-state guard (`:821`) | Skip view-switch globals while the list filter is being typed. | Unchanged. |
| 4. View-switch globals (`:822-1255`) | `b`/`g`/`i`/`h`/`a`/`f`/`E`/`[`/`]` etc. | These bindings move INTO `GlobalKeys`. `key.Matches` runs against the same Map at this layer. |
| 5. Focus-specific dispatch (`:1258+`) | Per-view handlers (`handleListKeys`, etc.). | Per-view Maps consume what's left. |

Critical: **`GlobalKeys` is one Map but participates at two layers** (special-chord at layer 2, view-switch at layer 4). The dispatcher decides which subset is consulted at each layer; the Map itself stays single. This avoids splitting into `GlobalChordKeys` + `GlobalSwitchKeys` (which would create a re-drift surface).

**No match-and-fall-through.** Today some keys (`Tab`, `</>`) match a global case unconditionally, no-op when context isn't right, and crucially have no `return` — they fall through to per-view handlers that ALSO have a `Tab` case. That pattern does not survive `key.Matches`. **Resolution: every key is owned by exactly one Map.** `Tab` and `</>` move to per-view bindings (each view that wants split-view detail-toggle declares its own `Tab` binding; each view with a list/detail split declares its own resize binding). This eliminates the fall-through entirely. The cost is one extra binding declaration per affected view; the win is unambiguous dispatch.

**Help composition:**

| Surface | Bindings rendered | Why |
|---|---|---|
| Status-bar L1 (`ShortHelp`) | active view's `ShortHelp()` only | Transient, tight, focused on view ergonomics. Global keys are background context. |
| `;` sidebar L1.5 (`FullHelp`) | `GlobalKeys.FullHelp() ++ ActiveViewKeys.FullHelp()` (rendered as **vertical sections**, one binding per row, not via `help.FullHelpView()`'s horizontal column layout — bt's sidebar is 34 chars wide and 4 columns at that width is illegible) | Persistent "what can I press now" surface. Both global and view-scoped are relevant. |
| `?` overlay L2 (`FullHelp`) | `GlobalKeys.FullHelp() ++ ActiveViewKeys.FullHelp()` (preserving today's grouping; bt-xavk redesigns layout, not data source) | On-demand reference. Both layers are documented. |

**Surface count is v1 — revisit when bt-xavk lands.** bt-xavk's design includes a `??` (or second-press) Layer 3 for full reference. Layer 2 (compact cheat sheet) and Layer 3 (full reference) likely consume the same `FullHelp()` data with different rendering (compact grouping vs full descriptions/examples). The 3-surface table is correct *for current bt*; expect a 4th row when xavk's UX work decides Layer 3's render contract.

**L1 status-bar wire-up convention.** The existing 12-branch if/else hint chain in `model_footer.go:490-551` is replaced wholesale, not augmented. bt-ift6.1: (a) deletes the chain; (b) calls `help.ShortHelpView()` on the active-view Map (or modal Map when `m.activeModal != ModalNone`); (c) leaves `setInlineTransientStatus` untouched — it pre-empts `ShortHelp()` during its display window same as today. Both surfaces existing post-migration produces re-drift on day one and is the failure this epic prevents.

**Universal-nav-per-view declaration [v1 — revisit].** Universal-nav keys (`j/down`, `k/up`, `h/left`, `l/right`, `enter`, `esc`) are declared *in each view's Map* rather than in `GlobalKeys`. Today's reason: their semantics differ across views (`j` moves list cursor, board card cursor, graph cursor, viewport scroll; `enter` opens detail / applies recipe / submits BQL). Declaring per-view means each view's help accurately reflects its own semantics; declaring once in Global would force identical help text across views, which is false today.

**Why this is v1:** during the UX unification pass, the maintainer may decide that nav semantics *should* converge across views (e.g. "j always moves the cursor; what 'cursor' means is the view's job"). If that happens, hoisting `Up`/`Down`/etc. into `GlobalKeys` is a mechanical refactor — but the call belongs to the UX pass, not to this epic. Lock the *structure* now (per-view Maps exist, surfaces consume them); leave the *placement of universal-nav* open to the wider audit.

**Convention for bt-ift6.2 and downstream (the spine sets the pattern):** **arrows-primary, vim-keys-as-alternate, in display order.** Bindings are declared `WithKeys("up", "k")` (arrow first, vim second) and `WithHelp("↑/k", "move up")` (arrow first in display). This is the maintainer's native ergonomic; vim users get the alternate keys without the help surface privileging them. ListKeys (`bt-ift6.2`) sets the pattern; .3-.9 mirror it. This is also v1 — if a future audit decides to drop vim alternates, removing `"k"` from `WithKeys` is one line per binding.

`GlobalKeys` is reserved for bindings that have **identical OR uniform-context semantics in every view**: `?` (help), `;` (sidebar toggle), `:` (BQL), `'` (recipes), `!` (alerts), `Ctrl+R`/`F5` (refresh), `Ctrl+C` (quit), `Ctrl+S` (semantic search), view switches (`b/g/i/h/a/f/E`), `[`/`]` (label dashboard / attention), `q` (back/quit), `esc` (back/cancel), `w/W` (workspace toggles), `Alt+H` (hybrid preset).

**`q` and `esc` are context-aware globals.** Both are 9-branch state cascades in today's dispatcher (`q`: detail-fullscreen → focusInsights → focusFlowMatrix → ViewGraph → ViewBoard → ViewLabelDashboard → tea.Quit; `esc`: similar plus filter-clear). They get a single `key.Binding` in `GlobalKeys` with a single help text ("back / quit (context-aware)"). The cascade lives in the dispatcher's case body, NOT in the Map. Help shows one row; behavior is honestly cascade. Document this exception explicitly; do not split into per-context bindings.

**GlobalKeys column-layout spec** (bt-ift6.1 implements; sets the convention every Map's `FullHelp()` follows):

| Column | Bindings |
|---|---|
| Help & Chrome | `?`, `;`, `Ctrl+C`, `q`, `esc` |
| Views | `b`, `g`, `i`, `h`, `a`, `f`, `E`, `[`, `]` |
| Workspace | `w`, `W`, `Alt+H` |
| Actions | `Ctrl+R`/`F5`, `Ctrl+S`, `:`, `'`, `!` |

This is the column layout for `GlobalKeys.FullHelp()`. Every per-view Map's `FullHelp()` declares its own columns appropriate to the view (TreeKeys uses Move/Operate/Page/Exit; ListKeys, BoardKeys etc. choose their own).

**Universal-nav consistency convention.** Per-view declaration of universal nav is the right structural call for v1, but two-contributor drift is a real risk (`GraphKeys.Up.Help.Key = "↑/k"` vs `InsightsKeys.Up.Help.Key = "k/up"`). bt-ift6.1 lands a `TestUniversalNav_ConsistentAcrossViews` test that asserts: for any binding name shared across Map types (`Up`, `Down`, `Left`, `Right`, `Enter`, `Esc`, `Back`), the `Help.Key` strings match across all views that declare it. `Help.Desc` is allowed to differ (semantics legitimately vary — tree's `h` is "collapse / jump to parent"). The test is the structural defense that the convention text alone cannot provide.

### 2. Multi-key binding modeling

**Decision: one `key.Binding` per semantic action, with all equivalent keys listed via `WithKeys`.**

```go
Up: key.NewBinding(
    key.WithKeys("up", "k"),
    key.WithHelp("↑/k", "move up"),
),
```

The framework supports this via `WithKeys(keys ...string)` (`key.go:63-67`). `key.Matches` iterates the binding's keys (`key.go:130-140`); a `KeyPressMsg` whose `.String()` matches any of them returns true. `Help` is rendered once per binding, so help surfaces show one row per semantic action — matching the way users think about the binding.

Apply this consistently to:

- `j/down`, `k/up`, `h/left`, `l/right` (vim + arrows)
- `enter/space` (where both confirm — e.g. tree expand)
- `ctrl+d/pgdown`, `ctrl+u/pgup` (page navigation)
- `G/end`, `home/g` (jump to ends, where both are bound)
- Multiple dismiss keys (`E/esc`, `q/esc/h`, etc.) where they share semantics

Use **separate bindings** when keys have semantically distinct meanings even if they share a code path (e.g. `q` = quit at top level, but inside a modal `q` may mean cancel — distinct binding in each Map, distinct help text).

### 3. State-dependent enables — when to use `Disabled` [v1 — revisit]

**Framework behavior** (verified from source):
- `Binding.Enabled() = !disabled && keys != nil` (`key.go:106-108`).
- `Matches` skips disabled bindings (`key.go:134`). Disabled bindings will not dispatch.
- `help.ShortHelpView` and `help.FullHelpView` both skip `!kb.Enabled()` bindings (`help.go:138-140`, `help.go:202-205`). `shouldRenderColumn` skips entire columns where every binding is disabled (`help.go:246-253`). Help is self-managing.

**Decision: case-by-case.** Three categories, each with its own rule:

| Pattern | Use Disabled? | Mechanism |
|---|---|---|
| Structural read/write distinction (bt-oiaj.8: hide write keys when bead is read-only) | **Yes** | Set `Disabled` on write bindings when `m.activeIssue.IsReadOnly()`. Help auto-filters. |
| Modal-only bindings while no modal is open | **Yes (implicit)** | Modal Maps are never consumed by the active-view help surface; see Decision 4. |
| **First-impression transient state** (binding shown in help at startup before its precondition is met — e.g. board's `n/N` "next match" before any search has been run) | **Yes** | Set `Disabled` based on the precondition; flip on state transition. Help auto-hides. |
| Routine transient state (selection-aware copy, filter-aware reset, etc.) | **No** | Handler conditional, today's pattern. Help shows the binding regardless. |

**Why case-by-case, not flip-everything.** The `n/N`-at-startup case is a concrete first-impression failure: a new user opens board view, has never searched, sees `n: next match` in `?` overlay, presses `n`, gets silence. That's the worst UX class — the help is actively wrong about a basic feature. Wiring `Disabled` for it is ~3 lines (board's search-start sets `n.SetEnabled(true)`; search-clear sets it back to `false`; binding starts disabled). The cost is negligible; the help truthfulness for a first-impression case is real.

The routine cases (e.g. `y` copy when no issue is selected) hit a much narrower audience. They survive today as handler conditionals; flipping them on every selection change isn't worth the plumbing without dogfood evidence.

**Criterion for v1**: a binding should be `Disabled`-managed if its precondition is **stable enough that the binding is unreachable from view-entry alone**. Board's search match: yes, it's unreachable until you search. Selection-aware actions: no, you usually have a selection by view-entry. Apply the rule per binding, document the call inline.

bt-oiaj.8 will use `Disabled` for the read/write structural case. Board's `n/N` and similar first-impression cases get wired during the per-view conversion (.3-.10). No central `refreshKeyEnables()` method is committed to the ADR — each binding's flip lives at its own state-transition site, which is the smallest touch-radius. If we accumulate >5 such sites and feel the cost, revisit and consolidate.

### 4. Modal scope handling — preventing leak into main-view help

**Decision: modal Maps are owned by their handler; help surfaces consume the Map matching the *current modal*, not a static merge. Modal Maps embed `ModalChromeKeys` for universal escape hatches.**

When the recipe picker is open, `m.activeModal == ModalRecipePicker`. The help surface (`;` sidebar or `?` overlay) reads `m.activeModal` and renders `RecipePickerKeys.FullHelp()` instead of `<ActiveView>Keys.FullHelp()`.

**`ModalChromeKeys` baseline embed.** Every modal Map embeds a small `ModalChromeKeys` struct exposing the universal escape hatches:

```go
type ModalChromeKeys struct {
    Help     key.Binding   // ?
    Quit     key.Binding   // Ctrl+C
    Cancel   key.Binding   // Esc
}
```

This is the **one place** where embedding is structurally correct (the case Decision 1's "no embedding" rationale explicitly excludes — semantics are universal-by-definition for modal chrome, not view-specific). Each modal Map's `FullHelp()` includes the chrome bindings as their own column ("Help & Exit") plus the modal's own content columns. The modal is free to define rich content beyond the chrome — that's the point.

This means:

- **`?` and `Ctrl+C` work inside every modal** because every modal Map redeclares them via the chrome embed. Decision 1's "no concatenate Global into modals" stays — chrome is owned, not concatenated.
- **Modals own their sidebar content.** A modal's `FullHelp()` returns whatever columns the modal wants to teach. The label picker can show "Type filter labels / ↑↓ move / Space toggle multi-select / Enter apply" plus tips — the sidebar becomes a teaching surface scoped to the user's current task.
- **Hybrid modals (label picker, BQL, history search) get sub-state Maps per Decision 7.** `LabelPickerNavKeys` and `LabelPickerSearchKeys` are distinct Maps, each embedding `ModalChromeKeys`. The dispatcher AND help surfaces read both `m.activeModal` and the in-modal sub-state to pick the right Map. See Decision 7 for the full state-Map model.
- The dispatcher already routes by `m.activeModal` (see `model_update_input.go:62-210`); the new addition is the in-modal sub-state read for hybrid modals.

### 5. Semantic deltas: `switch msg.String()` vs `key.Matches`

**Framework behavior** (verified from `key.go:130-140`):

```go
func Matches[Key fmt.Stringer](k Key, b ...Binding) bool {
    keys := k.String()
    for _, binding := range b {
        for _, v := range binding.keys {
            if keys == v && binding.Enabled() {
                return true
            }
        }
    }
    return false
}
```

**Exact string equality** between `msg.String()` and each binding's keys. No case folding. No modifier normalization. **The matching semantics are bit-for-bit identical to today's `switch msg.String() { case "X": ... }` for any single-key case**, with two additions:

1. `Matches` is variadic over bindings, so `key.Matches(msg, keys.Up)` is one call but `key.Matches(msg, keys.Up, keys.Down)` is also valid (matches if either binding matches).
2. `Matches` skips disabled bindings; today's switch does not. This is the primary semantic delta and it is the one we want.

**Modifier and case empirical truths** (verified from bt's existing handlers and `tea.KeyPressMsg.String()` behavior):

| Key press | `msg.String()` | Binding key |
|---|---|---|
| Plain letter | `"j"` | `"j"` |
| Shift + letter | `"G"` (capital, single char) | `"G"` |
| Ctrl + letter | `"ctrl+d"` | `"ctrl+d"` |
| Alt + letter | `"alt+h"` | `"alt+h"` |
| Function key | `"f5"` | `"f5"` |
| Special | `"ctrl+c"`, `"esc"`, `"enter"`, `"space"`, `"tab"`, `"backspace"`, `"home"`, `"end"`, `"pgup"`, `"pgdown"` | same |

**Migration is safe by construction:** every `case "<string>":` in today's handlers becomes `WithKeys("<string>")` in a binding. No string transforms required. Tests that exercise the dispatcher via `tea.KeyPressMsg` (key strokes synthesized in teatest) continue to work without modification.

**Three behavioral notes:**

1. **Empty bindings never match.** `Enabled()` returns `false` when `keys == nil`. A binding declared with `key.NewBinding(key.WithHelp("X", "Y"))` (no keys) is unreachable. This is desired behavior.
2. **Binding order doesn't affect dispatch correctness, but does affect help rendering.** `FullHelp` returns `[][]key.Binding`; the outer slice defines column order, the inner slice defines row order. Order bindings in each Map's `FullHelp()` to match the help surface layout we want.
3. **`key.Matches` is generic over `fmt.Stringer`.** Works directly with `tea.KeyPressMsg`. No casting, no String() call by hand at the call site.

### 6. Worked example — `handleTreeKeys` end-to-end

**Why `handleTreeKeys`:** medium-sized (~47 LOC, 14 unique cases), exercises every pattern decision 1-5 needs to demonstrate (multi-key bindings, lower/uppercase pairs, ctrl combos, multi-key dismiss), and is unambiguously view-scoped (no Global key collision). `handleListKeys` is reserved for child .2 (the spine) so this spike can settle conventions without touching the most complex handler.

**Handler signature convention:** `func (m Model) handle<View>Keys(msg tea.KeyMsg) Model` for parity with existing handlers. `tea.KeyPressMsg` satisfies the `tea.KeyMsg` interface so the call site stays unchanged. Handlers that need to return `tea.Cmd` (e.g. `handleBQLQueryKeys`) keep the `(Model, tea.Cmd)` signature. Pin this in `.2`'s spine; `.3-.10` mirror.

**Today** (`pkg/ui/model_keys.go:285-331`):

```go
func (m Model) handleTreeKeys(msg tea.KeyMsg) Model {
    switch msg.String() {
    case "j", "down":
        m.tree.MoveDown()
    case "k", "up":
        m.tree.MoveUp()
    case "enter", "space":
        m.tree.ToggleExpand()
    case "h", "left":
        m.tree.CollapseOrJumpToParent()
    case "l", "right":
        m.tree.ExpandOrMoveToChild()
    case "g":
        m.tree.JumpToTop()
    case "G":
        m.tree.JumpToBottom()
    case "o":
        m.tree.ExpandAll()
    case "O":
        m.tree.CollapseAll()
    case "ctrl+d", "pgdown":
        m.tree.PageDown()
    case "ctrl+u", "pgup":
        m.tree.PageUp()
    case "E", "esc":
        m.focused = focusList
    case "tab":
        // ... split-view detail sync ...
    }
    return m
}
```

**After**, in `pkg/ui/keys/tree.go`:

```go
package keys

import "charm.land/bubbles/v2/key"

// TreeKeys are the bindings available when the hierarchical tree view is focused.
// Universal nav (j/k/h/l) is declared here, not in GlobalKeys, because the tree's
// h/l semantics (collapse-to-parent / expand-to-child) are tree-specific.
type TreeKeys struct {
    Up         key.Binding
    Down       key.Binding
    Collapse   key.Binding
    Expand     key.Binding
    Toggle     key.Binding
    JumpTop    key.Binding
    JumpBottom key.Binding
    ExpandAll  key.Binding
    CollapseAll key.Binding
    PageDown   key.Binding
    PageUp     key.Binding
    SyncDetail key.Binding
    Back       key.Binding
}

// NewTreeKeys returns the default tree keymap.
func NewTreeKeys() TreeKeys {
    return TreeKeys{
        // Arrows-primary, vim-keys-as-alternate (ADR-004 Decision 1, ListKeys
        // sets the pattern in bt-ift6.2). WithKeys lists arrow first; help
        // text shows arrow first.
        Up: key.NewBinding(
            key.WithKeys("up", "k"),
            key.WithHelp("↑/k", "move up"),
        ),
        Down: key.NewBinding(
            key.WithKeys("down", "j"),
            key.WithHelp("↓/j", "move down"),
        ),
        Collapse: key.NewBinding(
            key.WithKeys("left", "h"),
            key.WithHelp("←/h", "collapse / jump to parent"),
        ),
        Expand: key.NewBinding(
            key.WithKeys("right", "l"),
            key.WithHelp("→/l", "expand / move to child"),
        ),
        Toggle: key.NewBinding(
            key.WithKeys("enter", "space"),
            key.WithHelp("⏎/␣", "toggle expand"),
        ),
        JumpTop: key.NewBinding(
            key.WithKeys("g"),
            key.WithHelp("g", "jump to top"),
        ),
        JumpBottom: key.NewBinding(
            key.WithKeys("G"),
            key.WithHelp("G", "jump to bottom"),
        ),
        ExpandAll: key.NewBinding(
            key.WithKeys("o"),
            key.WithHelp("o", "expand all"),
        ),
        CollapseAll: key.NewBinding(
            key.WithKeys("O"),
            key.WithHelp("O", "collapse all"),
        ),
        PageDown: key.NewBinding(
            key.WithKeys("ctrl+d", "pgdown"),
            key.WithHelp("⌃d", "page down"),
        ),
        PageUp: key.NewBinding(
            key.WithKeys("ctrl+u", "pgup"),
            key.WithHelp("⌃u", "page up"),
        ),
        SyncDetail: key.NewBinding(
            key.WithKeys("tab"),
            key.WithHelp("⇥", "sync to detail pane"),
        ),
        Back: key.NewBinding(
            key.WithKeys("E", "esc"),
            key.WithHelp("E/esc", "back to list"),
        ),
    }
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Order matters: most ergonomic / most-used first.
func (k TreeKeys) ShortHelp() []key.Binding {
    return []key.Binding{k.Up, k.Down, k.Toggle, k.Back}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Move, Operate, Page, Exit.
func (k TreeKeys) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        {k.Up, k.Down, k.Collapse, k.Expand, k.JumpTop, k.JumpBottom},
        {k.Toggle, k.ExpandAll, k.CollapseAll, k.SyncDetail},
        {k.PageDown, k.PageUp},
        {k.Back},
    }
}
```

**Handler in `pkg/ui/tree_keys.go`** (per .1's file-split convention):

```go
package ui

import (
    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/key"
)

func (m Model) handleTreeKeys(msg tea.KeyMsg) Model {
    k := m.keys.Tree
    switch {
    case key.Matches(msg, k.Down):
        m.tree.MoveDown()
    case key.Matches(msg, k.Up):
        m.tree.MoveUp()
    case key.Matches(msg, k.Toggle):
        m.tree.ToggleExpand()
    case key.Matches(msg, k.Collapse):
        m.tree.CollapseOrJumpToParent()
    case key.Matches(msg, k.Expand):
        m.tree.ExpandOrMoveToChild()
    case key.Matches(msg, k.JumpTop):
        m.tree.JumpToTop()
    case key.Matches(msg, k.JumpBottom):
        m.tree.JumpToBottom()
    case key.Matches(msg, k.ExpandAll):
        m.tree.ExpandAll()
    case key.Matches(msg, k.CollapseAll):
        m.tree.CollapseAll()
    case key.Matches(msg, k.PageDown):
        m.tree.PageDown()
    case key.Matches(msg, k.PageUp):
        m.tree.PageUp()
    case key.Matches(msg, k.Back):
        m.focused = focusList
    case key.Matches(msg, k.SyncDetail):
        if m.isSplitView {
            if selected := m.tree.SelectedIssue(); selected != nil {
                for i, item := range m.list.Items() {
                    if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selected.ID {
                        m.list.Select(i)
                        break
                    }
                }
                m.updateViewportContent()
                m.focused = focusDetail
            }
        }
    }
    return m
}
```

**Reflection-based completeness test in `pkg/ui/keys/tree_test.go`** (replaces the earlier hardcoded-count cardinality test — that approach trips on every legitimate addition or removal of a binding, reintroducing the manual-maintenance loop this epic is supposed to eliminate):

```go
package keys

import (
    "reflect"
    "testing"
    "charm.land/bubbles/v2/key"
)

// TestTreeKeys_AllFieldsRendered asserts every key.Binding declared as a
// field on TreeKeys appears at least once in ShortHelp() ∪ FullHelp().
// This catches "added a field but forgot to put it in FullHelp()" silently —
// the audit's failure mode at a smaller scope.
func TestTreeKeys_AllFieldsRendered(t *testing.T) {
    k := NewTreeKeys()
    rendered := map[string]bool{}
    for _, b := range k.ShortHelp() {
        rendered[b.Help().Key] = true
    }
    for _, group := range k.FullHelp() {
        for _, b := range group {
            rendered[b.Help().Key] = true
        }
    }
    v := reflect.ValueOf(k)
    bindingType := reflect.TypeOf(key.Binding{})
    for i := 0; i < v.NumField(); i++ {
        if v.Field(i).Type() != bindingType {
            continue
        }
        b := v.Field(i).Interface().(key.Binding)
        if !rendered[b.Help().Key] {
            t.Errorf("field %s (key=%q) not present in ShortHelp+FullHelp",
                v.Type().Field(i).Name, b.Help().Key)
        }
    }
}
```

The test intentionally allows a binding to appear in BOTH ShortHelp and FullHelp (common — the most-used bindings show up in both); it only fails when a binding appears in NEITHER. Drop a binding from `FullHelp()` accidentally, or add a new binding to the struct without listing it anywhere → test fails.

**Help.Desc invariant + registry-completeness test in `pkg/ui/keys/keys_test.go`** (covers two invariants — every binding has help text, AND every Map field on `AppKeys` is registered for the help-text test):

```go
package keys

import (
    "reflect"
    "testing"
    "charm.land/bubbles/v2/help"
)

// allMaps lists every help.KeyMap exposed by this package.
// TestAllMapsRegistered_MatchesAppKeys below validates this stays in sync
// with the AppKeys struct — adding a field to AppKeys without adding it
// here is a test failure.
func allMaps() map[string]help.KeyMap {
    return map[string]help.KeyMap{
        "Tree": NewTreeKeys(),
        // "Global": NewGlobalKeys(),  // added by .1
        // "List":   NewListKeys(),    // added by .2
        // ... etc
    }
}

func TestAllBindings_HaveHelpDesc(t *testing.T) {
    for name, m := range allMaps() {
        for col, group := range m.FullHelp() {
            for row, b := range group {
                if b.Help().Desc == "" {
                    t.Errorf("%s.FullHelp()[%d][%d] (key=%q) has empty Help.Desc",
                        name, col, row, b.Help().Key)
                }
            }
        }
        for i, b := range m.ShortHelp() {
            if b.Help().Desc == "" {
                t.Errorf("%s.ShortHelp()[%d] (key=%q) has empty Help.Desc",
                    name, i, b.Help().Key)
            }
        }
    }
}

// TestAllMapsRegistered_MatchesAppKeys catches the "added a Map field to
// AppKeys but forgot allMaps()" drift — silent test passes if not enforced.
func TestAllMapsRegistered_MatchesAppKeys(t *testing.T) {
    app := NewAppKeys()
    v := reflect.ValueOf(app)
    keyMapInterface := reflect.TypeOf((*help.KeyMap)(nil)).Elem()
    registered := allMaps()
    for i := 0; i < v.NumField(); i++ {
        f := v.Field(i)
        name := v.Type().Field(i).Name
        if !f.Type().Implements(keyMapInterface) {
            continue
        }
        if _, ok := registered[name]; !ok {
            t.Errorf("AppKeys.%s implements help.KeyMap but is not in allMaps()", name)
        }
    }
}
```

**Dogfood verification:**

Every keystroke listed below maps to a binding in `TreeKeys`. The user verifies each in a live `bt` session before .1 begins. This is the template for every child's dogfood matrix entry.

| Keys | Expected behavior | Verified |
|---|---|---|
| `j`, `↓` | move down | ☐ |
| `k`, `↑` | move up | ☐ |
| `Enter`, `Space` | toggle expand | ☐ |
| `h`, `←` | collapse / jump to parent | ☐ |
| `l`, `→` | expand / move to child | ☐ |
| `g` | jump to top | ☐ |
| `G` | jump to bottom | ☐ |
| `o` | expand all | ☐ |
| `O` | collapse all | ☐ |
| `Ctrl+d`, `PgDn` | page down | ☐ |
| `Ctrl+u`, `PgUp` | page up | ☐ |
| `E`, `Esc` | back to list | ☐ |
| `Tab` (split view) | sync detail pane | ☐ |

### 7. State-dispatched handlers — one Map per dwellable sub-state

**Decision: a sub-state inside a view or modal gets its own `key.Map` if both conditions hold: (a) dwellable — the user can stay there for >1 keystroke before resolving; (b) meaningfully different keymap — some keys mean different things or are unavailable in this sub-state.** Either fails → conditional inside the parent Map (today's pattern, retained for transient sub-states).

The deeper principle: the audit problem is "two surfaces drift because each maintains its own truth." `key.Map` solves that by making one Map authoritative for both dispatch AND help. The same problem recurs at the within-handler level when a single handler routes one keystroke to different actions based on state. If `LabelPickerKeys` says "j: move down" but the dispatcher routes `j` to text input when search is focused, the Map is lying about what `j` does — same drift class, smaller scope. The structural fix: the Map IS the dispatch table. If state determines dispatch, state determines which Map is consulted.

**Resulting Map list for the state-dispatched handlers** (~5 additional Maps over the original epic plan):

| Handler | Sub-states | Maps | Notes |
|---|---|---|---|
| `handleHistoryKeys` | Normal, Search, FileTreeFocus | `HistoryNormalKeys`, `HistorySearchKeys`, `HistoryFileTreeKeys` | All three dwellable; all three have meaningfully different keymaps. |
| `handleBoardKeys` | Normal, Search | `BoardNormalKeys`, `BoardSearchKeys` | gg-combo fails (a) — single keystroke, not dwellable → conditional inside `BoardNormalKeys`. |
| `handleLabelPickerKeys` | Nav, Search | `LabelPickerNavKeys`, `LabelPickerSearchKeys` | Both embed `ModalChromeKeys` per Decision 4. |
| `handleBQLQueryKeys` | (none) | `BQLQueryKeys` (1 Map) | textinput captures every letter; only `Esc/Enter/up/down/Tab` matter to dispatch. Letters legitimately don't appear in the Map. |

**Dispatcher reads sub-state.** The dispatcher consults `m.activeModal` AND any in-modal/in-view sub-state to pick the right Map. For the label picker: `if m.activeModal == ModalLabelPicker { if m.labelPicker.IsSearchFocused() { use LabelPickerSearchKeys } else { use LabelPickerNavKeys } }`. The help surface uses the same lookup — single source of truth for "what keys do what right now."

**Help is truthful in every state.** When the user is in label-picker search mode, the sidebar shows the search-mode bindings (filter typing, ↑↓ to move through results, Enter to apply). When they leave search and re-focus nav, the sidebar updates to show nav bindings. The "help lie" Decision 3 explicitly tolerates for transient state does NOT apply at the sub-state level — sub-state Maps are the structural defense against it.

**Cost:** ~5 additional Map structs (~30-50 LOC each), 3 additional handler files. Boilerplate is offset by smaller, focused Maps each individually easier to review. The states already exist as conditional branches in the current handlers; they just become explicit Maps with their own help text.

**Affects bt-ift6 child scoping:** `.6` (History) goes from 1 Map to 3, `.4` (Board) goes from 1 to 2, `.9` (Modals/LabelPicker, BQL) goes from 1 to 3. Total Map count: ~10 (original plan) → ~15. Update the epic's child notes accordingly.

## Consequences

### Positive

- **The audit's drift class becomes structurally impossible.** Binding-to-help-listing drift (the recurring failure inventoried in `docs/audits/domain/2026-04-23-keybindings-audit.md`) cannot recur because there is no second place to drift toward. Smaller drift surfaces (within-binding typo, field-vs-help-method) are tooling-defended via reflection-based tests (Decision 6) or code-review-defended.
- **Adding a new key requires editing one location** (the `key.Binding` declaration in the relevant Map). Help surfaces auto-update; reflection tests catch silent omissions.
- **bt-oiaj.8 (write-aware help) becomes a one-line change per binding** — set `Disabled` on write bindings when active issue is read-only; help auto-filters them out.
- **bt-xavk's UX redesign decouples from the data layer.** Layout choices (columns, headers, rivers) operate over the same `FullHelp()` API; the help-data plumbing is solved by this epic, not xavk's. Surface count stays v1-revisitable to accommodate xavk's `??` Layer 3.
- **Help is truthful in every state**, including hybrid modals (label picker search vs nav, history search vs file-tree-focus) — Decision 7 makes sub-state structurally explicit instead of hidden in handler conditionals.
- **Existing keystroke tests continue to work unchanged** — `key.Matches` consumes the same `tea.KeyPressMsg.String()` strings the existing switches do.

### Negative

- **Boilerplate: every binding is ~5 lines of declaration** vs one `case "X":` line today. Estimated ~200-250 LOC added across `pkg/ui/keys/` (revised upward to account for Decision 7's sub-state Maps and Decision 4's `ModalChromeKeys` embed). Counterbalanced by ~200 LOC of legacy string tables deleted in `shortcuts_sidebar.go` + `model_view.go` + `model_footer.go`'s L1 hint chain once .13 lands.
- **One indirection layer at dispatch sites:** `key.Matches(msg, k.Down)` instead of `case "j", "down":`. Costs minor cognitive load; recovers it via help.Desc-self-documenting bindings.
- **Universal-nav keys are declared per-view**, which means `j/k` is repeated in `ListKeys`, `BoardKeys`, `GraphKeys`, `TreeKeys`, etc. The duplication is real (~12 lines × ~8 views) but defended against drift by `TestUniversalNav_ConsistentAcrossViews` (Decision 1). The v1 caveat: hoisting universal-nav into `GlobalKeys` is mechanical for `WithKeys` strings but a UX call for `WithHelp` text (per-view help legitimately differs — tree's `h` is "collapse / jump to parent"). Future UX pass picks this back up with accurate cost framing.
- **State-aware help requires per-binding judgment** (Decision 3, case-by-case): first-impression-failure cases (board's `n/N`) get `Disabled` wiring; routine selection-aware cases stay as handler conditionals. Each call documented inline.
- **Sub-state Map count is higher than a "one Map per view" model** (~15 Maps total vs ~10). Defended in Decision 7 as the structural cost of help truthfulness in every state.

### Risks

- **`tea.KeyPressMsg.String()` could differ from KeyMsg in edge cases.** All existing handlers use `msg.String()` and work; bubbletea v2's `KeyPressMsg.String()` behavior should match. Verified by the worked example's tests passing on the spike branch.
- **Modal scope (Decision 4) defers a UX question to .10/.11.** If "show only modal Map" produces complaints during dogfood, the fix is local to the surface (re-introduce a "press `?` for view help" affordance), not the registry.

## Alternatives considered

### Alt 1: Side-table registry (the original 2026-04-23 framing)

A hand-rolled `map[string][]Binding` indexed by view name. Same shape as today's surface tables, just unified.

**Rejected:** doesn't use the framework idiom that bt already imports transitively. We'd be re-implementing `key.Map` poorly. Loses `Disabled` self-management. Forces a custom matching function instead of `key.Matches`. The framework solution is strictly better and the import is free.

### Alt 2: Single global `KeyMap` with a `context: []string` metadata field per binding

One Map, every binding tagged with which views it applies to. Surfaces filter by `m.activeView`.

**Rejected:** turns per-view scoping into a runtime predicate instead of a structural property. The audit's drift recurrence proves that runtime conventions decay; structural ones don't. With per-view structs, "this binding belongs to the tree view" is a fact about which file it lives in, not a string in a slice.

### Alt 3: Embed `GlobalKeys` in every per-view Map

`type ListKeys struct { GlobalKeys; List specifics }`. Every Map gets globals automatically.

**Rejected:** conflates dispatch composition (where order matters: globals first) with help composition (where the globals belong in a separate column for clarity). Creates name collisions when a per-view binding shadows a global one (e.g. `q` means different things in modals vs main views). Per-Map declarations + consumer-side composition is the cleaner split.

### Alt 4: Defer the conversion until bt-xavk's redesign ships

Build the redesign, then refactor the registry as part of it. The steel-manned version is "design data-shape concurrently with xavk's UX, ship sequentially" — not "do them together."

**Rejected, with acknowledgment.** xavk explicitly depends on this epic per the bead text — it cannot start until the data layer exists. Coupling structural refactor with UX redesign compounds blast radii and makes regression-bisection harder. The 2026-05-07 audit refresh is a fourth empirical case of string-table maintenance failing; another month of waiting accumulates more drift. **However:** the steel-manned concurrent-design version of Alt 4 is partially right — when xavk lands, it may reveal data-shape needs not anticipated here (per-binding L1/L1.5/L2 weights, layout-column tags, filter categories). Mitigation: the surface count is marked v1 (Decision 1); decisions 1, 3 carry `[v1 — revisit]` markers; the Map shape is intentionally minimal (struct fields + `ShortHelp()`/`FullHelp()`) so additive extension is mechanical. Pre-`.1`-merge, xavk's recon should sanity-check the Map shape supports its expected grouping needs — a one-touch coordination step that costs nothing and covers the genuine risk Alt 4 surfaces.

## References

- Framework source: `charm.land/bubbles/v2@v2.1.0/key/key.go` (`Binding`, `Matches`, `WithKeys`, `WithHelp`, `WithDisabled`, `Enabled`, `SetEnabled`).
- Framework source: `charm.land/bubbles/v2@v2.1.0/help/help.go` (`KeyMap` interface, `ShortHelpView`, `FullHelpView`, `shouldRenderColumn`).
- Audit: [`docs/audits/domain/2026-04-23-keybindings-audit.md`](../audits/domain/2026-04-23-keybindings-audit.md) (drift inventory + 2026-05-07 refresh).
- Spike branch: `bt-ift6.0-spike` (this ADR + the worked example).
- Epic: `bt-ift6` (parent).
- Spike bead: `bt-ift6.0` (this ADR).
- Foundation child: `bt-ift6.1` (consumes these decisions, scaffolds `pkg/ui/keys/`).
- Help redesign: `bt-xavk` (depends on this epic).
- Write-aware help: `bt-oiaj.8` (sits on `Disabled` from Decision 3).

## Deferred / Open Questions

### From 2026-05-07 ce-doc-review

This section captures findings from a six-persona review (coherence, feasibility, product-lens, design-lens, scope-guardian, adversarial) of ADR-004's `proposed` state. Each finding is logged here as record; resolutions land in subsequent commits or in a v1-revisit pass.

Severity scale: P0 (cross-persona consensus, structurally load-bearing) → P1 (concrete and consequential) → P2 (calibration) → P3 / FYI (advisory).

#### P0-S1. State-dispatched / hybrid-modal handlers don't fit "one Map per view"

- **Source:** feasibility, adversarial, design-lens, scope-guardian, product-lens (5/6 reviewers, cross-persona promotion to P0).
- **Why it matters:** the worked example (`handleTreeKeys`) was deliberately the simplest handler. Production handlers `handleBoardKeys` (IsSearchMode + IsWaitingForG), `handleHistoryKeys` (IsSearchActive + FileTreeHasFocus), `handleLabelPickerKeys` (IsSearchFocused), `handleBQLQueryKeys` (textinput captures every letter) are nested state machines: the same `msg.String()` value routes to different actions depending on focus or mode. Decision 4's `m.activeModal` rule does not capture focus-within-modal. The ADR doesn't decide whether each state mode gets its own Map (e.g. `HistorySearchKeys`, `HistoryFileTreeKeys`, `HistoryNormalKeys`) or whether one Map suffices with conditionals retained at every `key.Matches` site.
- **Evidence:** `pkg/ui/model_keys.go:22-54` (handleBoardKeys), `:367-432` (handleHistoryKeys), `:800-908` (handleLabelPickerKeys), `:744+` (handleBQLQueryKeys with `(Model, tea.Cmd)` signature).
- **Impact if unresolved:** could blow up the bead count for .3-.9 mid-fan-out, or force re-litigation of Decision 4 once an implementer hits the first hybrid handler.
- **Status:** open, gates resolution before .3 begins.

#### P1-S1. "Drift becomes structurally impossible" is overclaimed

- **Source:** adversarial, scope-guardian, product-lens (3-way agreement).
- **Why it matters:** four drift channels survive the migration: (a) `WithKeys("up","k")` paired with `WithHelp("↑/j",...)` typo — `TestAllBindings_HaveHelpDesc` only checks `Desc != ""`, not Help.Key↔WithKeys correspondence; (b) struct field added without `FullHelp()` entry → dispatches but invisible in help (the audit's exact bug pattern, restructured); (c) hardcoded `total == 13` cardinality test reintroduces the manual-maintenance loop the ADR claims to eliminate; (d) `allMaps()` in `pkg/ui/keys/keys_test.go` is hand-maintained — adding `ListKeys` to `AppKeys` without adding it to `allMaps()` produces no test failure.
- **Evidence:** `pkg/ui/keys/tree_test.go:11,17,24` (hardcoded counts); `pkg/ui/keys/keys_test.go:11-14` (hand-maintained allMaps); `bubbles/v2/key/key.go:43-47` (Binding stores keys and help as independent fields, no cross-validation).
- **Suggested fix:** narrow the claim to "binding-to-help drift is eliminated; key-string-to-help-display drift remains a separate concern." Add a reflection-based test that walks `AppKeys` fields and asserts every `key.Binding` appears exactly once across `ShortHelp()` + `FullHelp()`. Optionally add a `Help.Key` ↔ `WithKeys` token-overlap lint.
- **Status:** open.

#### P1-S2. GlobalKeys integration with multi-layer dispatcher is unspecified

- **Source:** feasibility.
- **Why it matters:** `handleKeyPress` in `model_update_input.go` is not a flat switch — it's at least 6 ordered layers: modal early-returns (`:62-555`), special chord routing for `?`/`;`/`Ctrl+R`/`F5`/`H`/`Ctrl+S` (`:558-759`), filter-state guards (`:821`), view-switch globals (`:822-1255`), focus-specific dispatch (`:1258-1359`). The ADR's "global checked first → per-view fallthrough" sketch collapses these. Where does `GlobalKeys` participate? Above filter-state guard, or below? Do `?`/`Ctrl+R` lose their pre-guard routing? An implementer landing bt-ift6.1 needs an architectural decision the ADR does not provide.
- **Evidence:** `pkg/ui/model_update_input.go:558-630` (special chords intentionally bypass filter-state guard); `:822-1255` (view-switch globals inside guard).
- **Status:** open, scopes bt-ift6.1.

#### P1-S3. Tab-style match-and-fall-through pattern not modeled

- **Source:** feasibility.
- **Why it matters:** today `case "tab":` at `model_update_input.go:922` always matches but only acts when split-view + ViewList; critically there is **no `return`**, so the keystroke ALSO reaches the focus-specific handler (`handleBoardKeys` ToggleDetail, `handleHistoryKeys` cycles file-tree focus). Mechanical conversion to a Global `Tab` binding under "global-first → fall-through-on-no-match" semantics would swallow Tab from per-view handlers and break board/history Tab handling. The pattern recurs for `</>` pane resize.
- **Evidence:** `pkg/ui/model_update_input.go:922-929` (no return); `pkg/ui/model_keys.go:183` (board Tab); `:478-493` (history Tab).
- **Status:** open.

#### P1-S4. L1 status-bar migration path is unspecified vs the existing if/else chain

- **Source:** design-lens.
- **Why it matters:** bt-ift6.1 needs to rewire the L1 surface, but the ADR's Decision 1 says "L1 renders active view's `ShortHelp()` only" without addressing the existing 12-branch if/else hint chain in `model_footer.go:490-551` (keyed on `m.activeModal` and `m.mode`). Does .1 delete the chain and replace with `help.ShortHelpView()`, or leave it as a fallback? Both = re-drift on day one (the exact failure mode the ADR exists to prevent).
- **Evidence:** `pkg/ui/model_footer.go:490-551`.
- **Suggested fix:** add a "L1 wire-up" sub-section under Decision 1 specifying: (a) bt-ift6.1 deletes the if/else chain and calls `help.ShortHelpView()` on the active-view Map; (b) modal-context overrides return the modal's own `ShortHelp()` rather than a separate if-branch; (c) the existing `setInlineTransientStatus` slot is orthogonal and pre-empts `ShortHelp()` during its display window.
- **Status:** open.

#### P2-S1. `q` and `esc` are not "identical semantics in every view"

- **Source:** feasibility.
- **Why it matters:** Decision 1 lists `q` in `GlobalKeys` ("identical semantics in every view"). The actual `q` handler at `model_update_input.go:826-861` is a 9-branch state cascade (detail-fullscreen → focusInsights → focusFlowMatrix-with-drilldown → ViewGraph → ViewBoard → ViewLabelDashboard → tea.Quit). `esc` (`:863-921`) is a similar cascade plus filter-clear-then-quit-confirm. Modeling these as a single binding with single Help.Desc misrepresents behavior.
- **Status:** open.

#### P2-S2. Decision 4: `?` and `Ctrl+C` accessibility in modals

- **Source:** product-lens, adversarial, feasibility (3-way).
- **Why it matters:** "GlobalKeys is not concatenated for modals — modal Maps declare their own esc / enter / q bindings." Does `?` (help) work inside a modal? If yes, the dispatcher catches it before the modal handler, contradicting the "modal Maps own dispatch" framing — and re-introduces a different drift surface. If no, `?` becomes unreachable from modals. The ADR doesn't say.
- **Suggested fix:** sub-decision listing globals (Ctrl+C, `?`, refresh) every modal Map MUST redeclare, OR a baseline `ModalChromeKeys` embed, OR test enforcing the fixed escape-hatch set.
- **Status:** open.

#### P2-S3. Surface table doesn't accommodate xavk's `?`/`??` two-tier

- **Source:** product-lens.
- **Why it matters:** ADR positions itself as foundation for bt-xavk and rejects Alt 4 ("xavk depends on this epic"). But xavk's design is progressive disclosure: `?` for compact cheat sheet (Layer 2), `??` for full reference (Layer 3). The ADR's surface table only enumerates 3 surfaces and binds `?` to a single `FullHelp()` rendering. If xavk needs two distinct render shapes for the same data, the surface composition is temporary, contradicting "structure locked."
- **Suggested fix:** either add a 4th surface row for "Layer 3 detailed reference" with whatever differentiates it from Layer 2, or mark surface count itself as v1.
- **Status:** open.

#### P2-S4. Sidebar render layout is undefined

- **Source:** design-lens.
- **Why it matters:** Decision 1 specifies the L1.5 data formula as `GlobalKeys.FullHelp() ++ ActiveViewKeys.FullHelp()` but doesn't specify how the 34-char-wide sidebar (`shortcuts_sidebar.go:37`) renders the resulting `[][]key.Binding`. `help.FullHelpView()` renders columns horizontally; 4 columns at 34 chars is illegible. bt-ift6.11 needs a render contract.
- **Suggested fix:** Decision 1 note: `;` sidebar renders `FullHelp()` groups as vertical sections (one binding per row), not via `help.FullHelpView()`'s horizontal layout. .11 implements a custom renderer.
- **Status:** open.

#### P2-S5. Universal-nav-per-view is structurally drift-prone

- **Source:** adversarial, product-lens (2-way).
- **Why it matters:** ~8 views × ~4 nav bindings × identical help text = drift surface with no consistency test. Two contributors editing two view Maps in two PRs can produce `GraphKeys.Up = "↑/k move up"` and `InsightsKeys.Up = "k/up move up"` — both technically correct, both inconsistent. The "v1 — revisit, hoisting later is mechanical" framing is true for `WithKeys` but NOT for `WithHelp` strings (tree's `h` is "collapse / jump to parent"; list's `h` may differ). Hoisting requires UX judgment, not just refactor.
- **Suggested fix:** either provide a `keys.NewNavKeys()` helper views compose, or add `TestUniversalNav_ConsistentAcrossViews` asserting `Help.Key` strings match for shared bindings.
- **Status:** open.

#### P2-S6. Decision 3 transient-state lie shows at startup, not just edge cases

- **Source:** adversarial.
- **Why it matters:** ADR frames the help-lie as "small (a binding listed that does nothing on press)." Concrete failure: in board view, before the user has invoked search, `?` overlay lists `n: next match` and `N: prev match`. New user opens `?`, sees `n`, presses `n` — silence. First-impression failure mode the ADR is meant to prevent. Cost-benefit ("refresh cost exceeds benefit") is asserted, not measured — `refreshKeyEnables()` would be called on a small set of state transitions (search-start, search-clear), not on every keypress.
- **Suggested fix:** flip Decision 3's default — transient bindings that gate on a clearly-named state get `Disabled`; refresh hook wired to the small set of transitions.
- **Status:** open.

#### P2-S7. Cardinality tests hardcode totals

- **Source:** scope-guardian, feasibility (2-way).
- **Why it matters:** `total == 13` is brittle. Once conversion is done, the test's drop-detection purpose is served, but every legitimate help-layout change (xavk-driven) trips the test. With 13 children each producing one such test, cumulative drag is real.
- **Suggested fix:** scope to a minimum floor (`total >= N`) or replace with reflection-based completeness check.
- **Status:** open.

#### P2-S8. `refreshKeyEnables` hook named before second use case

- **Source:** scope-guardian.
- **Why it matters:** ADR defines a named method and rule "more than two refresh sites = escalate" while simultaneously saying `Disabled` shouldn't appear in production code until bt-oiaj.8. The hook is forward-compatible design under a doc otherwise careful about YAGNI; naming it in a locked ADR creates mild pressure.
- **Suggested fix:** trim the convention from locked ADR text or move under `[v1 — revisit]`.
- **Status:** open.

#### P2-S9. `AppKeys` aggregator is one-field at spike

- **Source:** scope-guardian.
- **Why it matters:** at spike, `AppKeys.Tree` is the only field. Until 13 children land, the aggregator is a thin wrapper; if any child is deferred, the abstraction lives without earning its overhead.
- **Suggested fix:** bt-ift6.1 wires Global + Tree at minimum before merge.
- **Status:** open.

#### P3-S1. Handler signature convention not pinned

- **Source:** feasibility.
- **Why it matters:** worked example uses `tea.KeyMsg`; some handlers use `tea.KeyPressMsg`; `handleBQLQueryKeys` returns `(Model, tea.Cmd)`. .2-.9 implementers may pick differently.
- **Suggested fix:** Decision 6 sentence pinning `func (m Model) handle<View>Keys(msg tea.KeyMsg) Model` as default; `(Model, tea.Cmd)` for handlers that need it (BQL).
- **Status:** open.

#### P3-S2. No GlobalKeys column-layout spec

- **Source:** design-lens.
- **Why it matters:** `TreeKeys.FullHelp()` specifies "Columns: Move, Operate, Page, Exit." `GlobalKeys` has no equivalent spec; .1 invents it after the fact, missing the same drift-prevention the column spec provides.
- **Suggested fix:** add a GlobalKeys column-layout table to Decision 1 (categories: Help, Workspace, Views, Filters, Actions, etc.).
- **Status:** open.

#### P3-S3. Dogfood matrix only on TreeKeys; not propagated

- **Source:** product-lens.
- **Why it matters:** the dogfood matrix is the most authoritative behavior-preservation check for an internal tool with the maintainer as primary user. The cardinality test catches drops, not behavior regressions. Documented once for TreeKeys, not enforced for fan-out children.
- **Suggested fix:** epic child-close criterion: dogfood matrix attached, every row checked.
- **Status:** open.

#### P3-S4. Alt 4 rejection engages weak version

- **Source:** adversarial.
- **Why it matters:** the steel-manned Alt 4 is "design data-shape concurrently with xavk's UX, ship sequentially" — not "do them together." The ADR's rejection rationale doesn't engage that. Practical consequence: when xavk reveals data-shape needs (per-binding L1/L1.5/L2 weights, layout-column tags), the framework Maps will need extension, re-touching every view.
- **Suggested fix:** acknowledge in Alt 4 that data-shape may need extension at xavk-land, OR add a brief xavk-coordination check before .1 merges.
- **Status:** open.

#### Residual risks (advisory, no action required)

- 8 modals (`ModalQuitConfirm`, `ModalAgentPrompt`, `ModalTutorial`, `ModalCassSession`, `ModalUpdate`, `ModalLabelHealthDetail`, `ModalLabelDrilldown`, `ModalLabelGraphAnalysis`) have no Map mention. .10/.11 will need to enumerate.
- `;` sidebar's `ResetScroll()` may fire on every focus shift under the new Map-driven render — disorienting at small terminal heights.
- Premise scrutiny passes: drift is documented (audit + S/R recurrence), framework is in dep graph, alternatives are cleanly rebutted. The structural adoption is the right move.
- Strategic compounding direction is positive; deeper Charm idiom adoption aligns with Charm v2 trajectory.
