# Smart Footer Redesign

**Date**: 2026-06-21
**Status**: Design approved (visual direction); spec under review
**Beads**: supersedes stale `bt-ugbp.1-.6`; subsumes `bt-d5wr` / `bt-d5wr.1` (footer visual + IA),
generalizes `bt-gcuv` (scoped numbers), consumes `bt-ift6` per-view `key.Map`s.

---

## One-line thesis

**One chrome line (the footer), three zones (lens / scoped-numbers / hints). Both the numbers and
the hints adapt per view. The footer never wraps or clips - it degrades through width tiers.
Notifications are a transient toast plus a permanent unread count; the full feed stays in the
alerts modal.**

---

## The mockups (the design)

### Smart by size - list view, cross-project / 4921-issue case

```
160| 🌐 ALL │ ○1811 ◉1684 ◈0 ●3110 · 4921           ⏎ open · o issues · c copy · t diff · ? help   🔔3
100| 🌐 ALL │ ○1811 ◉1684 ●3110 · 4921        ⏎ open · o issues · ? help   🔔3
 80| 🌐 ALL │ 4921 open          ⏎ open · ? help   🔔3
 70| 🌐 │ 4921        ⏎ · ?  🔔3
 60| 🌐 │ 4921    ?  🔔3
```

Degradation order as width tightens:
1. drop zero-count stat segments (`◈0`)
2. drop verbose per-status stats, keep the total
3. collapse hint labels to keys (`⏎ open` -> `⏎`)
4. drop the filter word, scope-icon only
5. last survivors: `total · ? · 🔔`

### Smart by view - center zone changes *meaning*, not just the hints (width ~90)

```
LIST  │ 🌐 ALL │ ○1811 ◉1684 ●3110 · 4921        ⏎ open · o issues · ? help   🔔3
DETAIL│ 📂 bt  │ bt-0qzp · 3/169                  esc back · C copy · O edit · ? help   🔔
BOARD │ 📂 bt  │ 4 cols · 169 cards               hjkl nav · ⏎ view · b list · ? help   🔔
GRAPH │ 📂 bt  │ 47 nodes · 61 edges              hjkl nav · ⏎ view · g list · ? help   🔔
```

"What am I looking at" adapts: detail = which bead + position, graph = nodes/edges, list/board =
scoped corpus counts.

### Context numbers - bt-gcuv generalized: numbers reflect scope + filter

```
cross-project, all : 🌐 ALL                    │ ○1811 ◉1684 ●3110 · 4921
project bt, all    : 📂 bt                      │ ○163 ◉2 ●4 · 169
project bt, OPEN   : 📂 bt · OPEN               │ ○163 · 163 open
project bt, label  : 📂 bt · 🏷 area:tui · OPEN  │ ○38 · 38 open
```

Left zone names the lens; center counts only what is inside that lens.

### Notifications = signal, not content (list, ~100)

```
idle    │ 🌐 ALL │ ○1811 ◉1684 ●3110 · 4921     ⏎ open · o issues · ? help   🔔3
refresh │ 🌐 ALL │ ○1811 ◉1684 ●3110 · 4924     ✓ reloaded  +3 −1            🔔4   (toast ~3s, fades to hints)
error   │ 🌐 ALL │ ○1811 ◉1684 ●3110 · 4921     ✗ write failed: db locked    🔔5   (sticky until acknowledged)
```

Toast borrows the hint slot briefly, then gives it back. `🔔N` is the only permanent notification
footprint; the actual feed lives in the existing alerts modal.

---

## Why this, grounded in the current code

Dogfood at the user's scrunched widths (render harness, `BT_RENDER_DUMP=1`):

- **detail @ 70x20**: footer wraps to 2 lines (eats a content row) and shows *list* hints (wrong view).
- **list @ 70-100**: footer clips mid-pill (`│ o open` / `│ c` dangling).
- **most views**: empty L1 hint slot - only ViewList + ViewTree route to a `key.Map`.

Root cause (`pkg/ui/model_footer.go`):
- `Render()` key-hint truncation (lines ~770-777) has a hard floor of 2 pills and never trims pill
  *text*. Tier-dropping (lines ~820-829) only drops *optional* badges; the always-present
  filterBadge + labelHint + statsSection + countBadge + keysSection are never reduced. At
  4921-issue scale the stats segment alone is ~21 cols, so even 2 pills overflow -> wrap/clip.
- `viewKeyMap()` only dispatches ViewTree + ViewList; `modalKeyMap()` returns nil. Every other view
  falls through to nil (empty hints) or inherits list hints.

What is *already built* and just needs wiring:
- Every per-view `key.Map` exists in `pkg/ui/keys/` (Board, Graph, Insights, History, FlowMatrix,
  Actionable, Epics, EpicCard) plus modal maps (LabelPicker, RecipePicker, BQLQuery,
  TimeTravelInput, RepoPicker), each with `ShortHelp()` / `FullHelp()`. `NewAppKeys()` constructs
  them all; `m.keys` holds them.
- Notification data source: `m.events` ring buffer (bt-46p6.10) - `UnreadCount()`, `Snapshot()`,
  `Dismiss()` - already feeds the alerts modal.

So the stale `bt-ugbp.1-.6` per-view hint-reduction beads are obsolete: hints now come from
`ShortHelp()`, not a 12-branch string table. The fix is structural (degradation engine + wiring),
not per-view string surgery.

---

## Architecture

Three concerns, kept separate:

### 1. Zone model (rendering)

`FooterData.Render()` becomes a three-zone composer:

- **Left (lens)**: scope glyph + scope name + active filter chips (status filter, label filter).
  Replaces today's `filterBadge` + scattered filter chips.
- **Center (scoped numbers)**: a view-supplied summary string. Default = scoped stats + total.
  Per-view overrides via a small interface (see zone-meaning below).
- **Right (affordances)**: per-view `ShortHelp()` pills + the permanent `🔔N` badge.

Optional state badges (worker, watcher, update, dataset, session, instance, workspace) keep their
existing tier system but are re-homed between left and center, and drop first under pressure.

### 2. Degradation engine (the never-wrap guarantee)

A single `fit(width)` pass that, in priority order, reduces content until the assembled line
`lipgloss.Width(...) <= width`, then hard-truncates as a final safety net so wrapping is
*structurally impossible*:

```
tiers (drop/shrink in this order until it fits):
  T1  optional state badges (existing tier 1-3)
  T2  zero-count stat segments (◈0)
  T3  verbose per-status stats -> total only
  T4  hint labels -> keys (key.Binding.Help().Key only, drop .Desc)
  T5  drop hints from the middle, floor now 1 (not 2), then 0
  T6  filter word -> scope icon only
  SAFETY  ansi-aware truncate-to-width on the final string (never reached in practice)
```

The `🔔N` badge and the `?` hint are pinned (last to drop) so discoverability + activity signal
always survive.

### 3. Context-aware numbers (bt-gcuv generalized)

The center summary is computed from the **active filtered/scoped issue set**, not the global corpus.
This means threading the already-filtered slice (the set the view is rendering) into `footerData()`
rather than the global `m.issues`. Where a view already holds its filtered set, use it; the scope
mode (`.scope.mode`) selects cross-project vs project labeling.

### 4. Per-view zone meaning

A small per-view hook supplies the center string. Default implementation returns scoped stats.
Detail/graph/board override. Candidate shape: a `footerCenter(m) string` switch on `m.mode`
(mirrors `viewKeyMap()`), or a method on each view. Decision deferred to the plan; the switch is
simplest and matches the existing `viewKeyMap()` pattern.

### 5. Notifications

> Refined by the 2026-06-22 brainstorm into a full Phase 4 spec:
> [2026-06-22-footer-phase4-notifications.md](2026-06-22-footer-phase4-notifications.md).
> That doc supersedes the sketch below (notably: errors are severity-tiered and
> recoverable in the bell, and "sticky" means *until the condition resolves*, not
> until the user acknowledges).

- **Transient toast**: severity-driven (Success / Notice / Failure / Degraded). Renders as a
  glyphed center override (`✓ ... +N −M` / `✗ ...` / `⚠ ...`) that yields back to hints. Success
  and Notice auto-fade and never touch the bell; Failure auto-fades (longer) and Degraded is sticky
  until the live condition self-resolves - both append an `EventSystem` so they are recoverable in
  the alerts modal.
- **Permanent badge**: `🔔N` = ring-buffer events since `alertsSeenAt` (a session high-water-mark),
  pinned in the right zone, opens the alerts modal. Opening clears the footer (sets `alertsSeenAt`)
  without dismissing the modal inbox - "seen" and "dismissed" are separate states. Cross-session
  durability is deferred to `bt-vhzia`.

---

## Phasing

Each phase ships independently and is verifiable via the render harness.

- **Phase 1 - never-wrap + three-zone layout + per-view hint wiring.** The structural core.
  Rewrite `Render()` as zone composer + degradation engine; route `viewKeyMap()` / `modalKeyMap()`
  to the existing maps. Delivers: correct per-view hints, one-row guarantee at all widths. Biggest
  visible win; kills the wrap/clip/empty-hint bugs. Supersedes bt-ugbp.1-.6.
- **Phase 2 - context-aware numbers.** Thread the scoped/filtered set into the center summary.
  Closes bt-gcuv. Cross-project vs project labeling via scope mode.
- **Phase 3 - per-view center meaning.** Detail position, graph nodes/edges, board columns.
- **Phase 4 - notification toast zone + permanent badge.** Designed toast + `🔔N` from the ring
  buffer. Closes the footer half of bt-d5wr's notification scope.

Phase 1 is the gate; 2-4 layer on without re-touching the composer.

## Testing

- Render harness (`pkg/ui/render_harness_test.go`): add footer-focused scenarios at 60/70/80/100/120/160
  across list/detail/board/graph, asserting (a) footer is exactly 1 row, (b) no mid-pill clip,
  (c) correct view hints present. `BT_RENDER_DUMP=1` for visual diffing.
- Unit test the degradation engine directly on `FooterData` at each tier boundary.
- Existing footer tests migrate to the new zone model.

## Out of scope

- Help overlay (`?`) and `;` sidebar redesign (bt-xavk; they consume the same maps).
- A permanent top line / project tabs (YAGNI; revisit only if a global switcher is needed).
- An accumulating activity ticker in the footer (rejected: steals a row at narrow widths).

## Resolved design questions

1. **Degradation order**: verbose stats drop before hints; total count survives as long as hints;
   last survivors `total · ? · 🔔`.
2. **Center-zone-meaning-per-view**: yes, phased (Phase 3).
