# `?` overlay: mini/full tiers + clean aesthetic (bt-dx7k.1)

Date: 2026-06-29
Beads: bt-dx7k.1 (this), bt-dx7k (parent bug), bt-xavk (epic). Reference: yazi `yazihelp.yazi`.
Status: Design locked, pending implementation
Supersedes: the rendering/aesthetic of the `?` overlay shipped in bt-dx7k Tasks 1-5
(same held branch `worktree-bt-dx7k-help-surface`, unpushed). Keeps that work's
scoping/scroll/column plumbing; replaces its panel rendering.

## Problem

Maintainer dogfood of the bt-dx7k `?` overlay (the just-built global-only,
responsive, scrolling version) found two residual problems:

1. **Scrunched still overflows.** At short heights the four task panels don't
   fit; CHROME clips and you must scroll to reach `esc`. Showing the full global
   set always, then scrolling, is the wrong model for a short screen.
2. **Busy aesthetic.** Four separately-bordered, rainbow-colored panels read as
   noisy even at large sizes.

Yazi's `yazihelp.yazi` keybinding modal solves the analogous problem elegantly:
it keeps a curated **full sheet** and a curated **mini card**, picks between them
by terminal height, and renders both in a clean single-border, dim-monochrome
style. This design ports that pattern to bt's `?` overlay.

## Decision

Make `?` **size-adaptive in content, not just layout**, and adopt yazi's clean
aesthetic (through bt's theme tokens so it tracks the active flavor).

| | mini card | full sheet |
|---|---|---|
| when | terminal too short for the full sheet | otherwise |
| content | ~7 curated global essentials (+1 conditional) | all 4 global task groups |
| layout | fixed 2-column, non-scrolling | responsive 1/2/4 columns by width |
| extras | `↓ expand` nudge + `; per-view` pointer | status legend when vertical room; scroll if it still overflows |

This stays fully compatible with the locked `bt-y8uku` decision: `?` is still
**global-only**; `;` is still the per-view escape hatch; there is still **no
`??` layer**. The mini is a size-variant of the same global surface, not a new
content layer.

### Rationale

- **Form follows size.** A short screen wants the few keys that orient you, not
  the whole set crammed + scrolled. yazi proved the curated-mini approach; the
  maintainer prefers it.
- **One clean box reads better than four colored ones.** The dim monochrome,
  inline-divider style is calmer and scales from 1 to 4 columns without visual
  competition between panels.
- **Single-source-of-truth preserved.** Both tiers still consume
  `m.keys.Global.FullHelp()`. The mini is a *projection* (a fixed list of which
  GlobalKeys fields to show), so its key/desc text still comes from the key.Map
  and cannot drift. No literal string tables are reintroduced.

## Tier selection (height-driven, like yazi)

Mirror yazi's `self._mini = (area.h - 2) < POPUP_H`:

- Compute the full sheet's natural rendered height for the current width.
- If `terminalHeight - chrome < fullSheetHeight` -> render the **mini**.
- Else render the **full sheet**.

Selection is **terminal-size only** (no new keypress, no toggle). Growing the
window past the threshold reveals the full sheet automatically; shrinking it
returns to the mini. The `↓ expand` nudge communicates this ("there's more --
make the window bigger"). Exact thresholds are tuned in the render harness
against the maintainer's 14-30 row sizes, not fixed in this doc.

## Mini card (content)

A curated, navigation-first projection of GlobalKeys, ~7 entries + 1 conditional,
2 columns, non-scrolling:

```
╭───────── shortcuts ─────────╮
│ b board     g graph         │
│ i insights  / search        │
│ l labels    w projects*     │
│ ? help      q quit          │
│   ↓ expand   ·   ; per-view │
╰── esc ── q ── close ────────╯
        * w only when multi-project
```

- **Bindings (by GlobalKeys field, descs sourced from the Map):** `Board`,
  `Graph`, `Insights`, `SearchBounce` (`/` "search" -- the real search-entry key;
  in list view `/` opens the Bubbles list filter, and the binding is always shown
  as "search"), `LabelPicker`, `Help`, `Back` (`q` "back / quit"), and
  `ProjectsOrWisps` **only when the scope is multi-project / cross-project**
  (bt already knows scope mode; in a single-project setup the slot is dropped).
- **NOT `SearchMode` (`^s`).** `^s` cycles the search *ranker*
  (fuzzy/hybrid/semantic; `model_update_input.go:704`), gated to list focus -- it
  does NOT open the search bar and is a no-op in other views. Its help desc
  "search mode" is misleading; it must not be the mini's "search" pointer. See
  the related-fix note below.
- **Dropped from earlier drafts:** BQL (`:`) -- low/zero observed usage; the
  broader "is BQL worth keeping" question is parked (separate bead, not this work).
- **Per-view actions (sort, etc.) are intentionally NOT here** -- they are
  view-specific (`s` "cycle sort" lives on List/Board maps), so they belong on
  `;`, which the mini's `; per-view` pointer sends you to. Surfacing them in the
  global `?` would re-break the `bt-y8uku` split.
- **Nudge line:** `↓ expand   ·   ; per-view` (prose, not Map rows). The bottom
  border weaves `esc ── q ── close`.
- The projection runs through the same enabled/has-help filter as the full sheet,
  so the mini stays truthful to context (a binding disabled in the current state
  is skipped).

## Full sheet (carried forward + restyled)

- **Content/plumbing unchanged from bt-dx7k Tasks 1-5:** global-only, four
  task-headed groups (SWITCH VIEWS / DO THINGS / WORKSPACE / CHROME) essentials-
  first; `helpOverlayColumns` 1/2/4 by width; `helpScrollMax`/handler clamp;
  status legend appended when there's vertical room; top-aligned `Place`.
- **Rendering replaced:** instead of one `RenderTitledPanel` per group (colored
  borders), render a **single** outer rounded box whose interior holds the groups
  as columns, each group introduced by an inline divider header.

## Aesthetic (both tiers) -- via bt theme tokens

Port yazi's look using bt's semantic theme tokens (so flavors retone correctly),
NOT hardcoded greys:

- ONE rounded outer border (the modal). No per-group boxes.
- Dim monochrome: subtle border color, dim section-header dividers, **bold**
  brighter keys, muted descriptions. Drop the per-group rainbow palette.
- Section headers as inline dividers, e.g. `── SWITCH VIEWS ──`, not boxed.
- Single `│` between columns in the full sheet.
- Title woven into the top border (`shortcuts`); dismiss/cross-ref hints on the
  bottom (`esc ── q ── close`; `;` cross-ref in the full sheet footer where there
  is room).

Token mapping (indicative; finalize against `pkg/ui` theme + `RenderTitledPanel`
/ `PanelOpts`): border + header -> a subtle/secondary token; keys -> base
foreground bold; descriptions -> muted/secondary. If `RenderTitledPanel` cannot
weave a bottom-border footer, render the footer as a dim centered last interior
line instead (acceptable).

## Implementation surface

- Reuse: `helpOverlayColumns`, `helpOverlayBodyLines`, `helpOverlayAvailBody`,
  `helpScrollMax` (model_view.go), `handleHelpKeys` clamp (help_keys.go).
- Change: `helpOverlayPanels` / the panel renderer -> a single-box, inline-header,
  theme-token renderer (replaces the per-group `RenderTitledPanel` + rainbow
  `colors`).
- Add: a mini renderer + the curated mini projection (a list of GlobalKeys field
  selectors) + the height-driven mini/full selector in `renderHelpOverlay`, plus
  the multi-project gate for the conditional `w` entry.
- Rename `GlobalKeys.SearchMode` help desc `"search mode"` -> `"cycle search
  ranker"` (`keys/global.go`); update any test asserting the old desc.
- The `;` sidebar (model_footer.go / shortcuts_sidebar.go) is unchanged by this
  spec (its view-only scoping + fallback + cross-ref from bt-dx7k stay).

## Testing

- Tier selection: short height -> mini; tall -> full (unit test on the selector).
- Mini content: the mini renders its curated global keys (sourced from the Map),
  excludes per-view descs (e.g. "cycle sort"), and includes/excludes `w projects`
  by scope mode.
- Aesthetic/fit: harness dumps at 30/50/70/120 wide x short/tall -- mini fits at
  short heights with no clip; full sheet non-clipping; single border, no rainbow.
- Single-source-of-truth: no literal key/desc tables (mini is field projection).
- Live dogfood at scrunched sizes before push (held).

## Related finding: `^s` "search mode" mislabel

Surfaced during this design. `GlobalKeys.SearchMode` (`^s`, help desc "search
mode") does NOT open search; it cycles the search ranker (fuzzy -> hybrid ->
semantic, `model_update_input.go:704`), gated to list focus, and is a no-op
elsewhere. The desc reads as search-entry and misled the design.

**Decision (maintainer, 2026-06-29):** folded into bt-dx7k.1 -- rename the
`GlobalKeys.SearchMode` help desc from `"search mode"` to **`"cycle search
ranker"`** (one-line change in `keys/global.go`). This propagates to every help
surface (L1 footer, `;` sidebar, `?` overlay) via the single key.Map source.
(Whether `/` search and a ranker key should both live in the global Actions
group is a separate, larger question -- not in scope here.)

## Out of scope

- `;` sidebar changes (done in bt-dx7k).
- The "is BQL worth keeping" question (parked; separate bead if pursued).
- CRUD write/read key distinction, L1 hint redesign, run-context surfacing
  (stay in bt-xavk).
- Any `??` full-reference layer (still explicitly dropped).

## Acceptance (closes bt-dx7k.1)

- At short heights the `?` overlay shows a compact, non-clipping mini card with a
  `↓ expand` nudge and a `; per-view` pointer.
- Growing the terminal reveals the full grouped sheet automatically; shrinking
  returns to the mini -- size-driven, no new keys.
- Both tiers render in one clean theme-tokened border (no rainbow per-group
  boxes), dim mono with inline-divider headers.
- `?` remains global-only; mini content is a key.Map projection (no literal
  tables); `w projects` appears only in multi-project scope.
- Verified across the scrunched harness matrix + a live dogfood run (push held).
```
