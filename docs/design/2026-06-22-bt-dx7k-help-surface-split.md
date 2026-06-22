# Help-surface split: `?` global, `;` view-specific (bt-dx7k)

Date: 2026-06-22
Beads: bt-dx7k (P1 bug, primary), bt-xavk (parent epic), bt-yvr4g (P2, adjacent)
Status: Design locked, pending implementation

## Problem

The `?` help overlay is unusable at the scrunched terminal sizes the maintainer
actually uses (14-30 rows). Two distinct defects compound:

1. **Scoping.** `renderHelpOverlay()` (`pkg/ui/model_view.go:653`) hardcodes all
   ten view key.Maps into `panels[]` (Global / List / Board / Graph / Insights /
   History / Actionable / Tree / Flow Matrix / Epics) plus a status-glyph legend,
   then river-packs them. That is the "everything at once" wall the maintainer
   called "pretty bad ... doesn't work in smaller dimensions at all." bt-ift6.11
   made it taller (8 thematic panels -> 10 per-map panels), worsening the bug.

2. **The scroll is dead.** `handleHelpKeys()` (`pkg/ui/help_keys.go`) mutates
   `m.helpScroll` on j/k/ctrl+d/G, but `renderHelpOverlay()` never reads it -- it
   ends with `lipgloss.Place(m.width, m.height-1, Center, Center, fullContent)`,
   which *centers* oversized content and clips both top and bottom. So content
   overflows with no way to pan. This is independent of scoping.

## Decision

Stop showing everything on both surfaces, and stop the form-vs-form redundancy
(`?` and `;` would otherwise show identical `Global ++ view` content in different
chrome). Split the content along the axis the form factors already imply:

| | `?` overlay | `;` sidebar |
|---|---|---|
| content | **Global map only** | **active view's map only** |
| form | transient, full-screen modal | persistent, docked, narrow |
| framing | task-oriented headers + status legend | view actions, raw/compact |
| job | "how do I drive this app" (orientation) | "what can I do on this screen" (do) |
| cross-ref | footer: "; for this screen" | footer: "? for global" |

### Rationale

- **Form follows content.** A modal you open, read, and dismiss is the right home
  for the stable, memorize-once global set (nav between the 10 views, search, BQL,
  recipes, refresh, quit). A panel pinned *beside your work* is the right home for
  the view-specific actions you reference repeatedly while working in that view.
  This is the form-factor-coherent split, not the intuitive-but-backwards
  "globals on the persistent surface."

- **Each surface stays self-sufficient enough.** Global content *is* the
  TUI-driving knowledge a newcomer needs; the per-view delta is small and one
  keystroke away on `;`. Two symmetric one-line cross-references close the loop --
  no surface silently strands the user.

- **`?` is the universal front door.** "Press ? for help" is the convention every
  user reaches for first; it carries the universal (global) content and *teaches*
  the companion (`;`). The "teaching element" is literally one hint string in the
  overlay footer, nothing more.

- **Bounded content makes responsiveness tractable.** The global map is ~31
  bindings in 4 stable groups. Laying that out responsively is far easier than the
  old variable 10-map dump -- the content decision and the responsive fix
  reinforce each other.

### What this supersedes

`docs/plans/2026-03-11-help-system-redesign.md` is stale: it predates the `;`
sidebar and the bt-ift6 per-view-Map / FullHelp() architecture, and proposes
`?`=compact-cheat-sheet plus `??`=full-reference-tabbed. This design replaces that
`?`/`??` shape. There is **no `??` layer**: the persistent `;` sidebar (L1.5)
carries the per-view content the old plan put in L3, and the global `?` carries
the orientation content. The old plan is left in place as historical context.

## Responsive behaviour (`?` overlay)

The two levers:

1. **Column count from width.** The global map's 4 task groups lay out as columns
   that collapse as width shrinks:

   ```
   width >= ~120   -> 4 task columns side by side   (~8-10 rows tall)
   width ~80-120   -> 2 columns                      (~16 rows)
   width < ~80     -> 1 column, essentials-first
   ```

2. **Scroll windowing when content > height.** Wire `m.helpScroll` into the render
   so it actually pans (clamped to content height). This is the dead-scroll fix.
   Essentials-first ordering (Views / navigation at the top) means the most useful
   keys are on screen even before any scroll.

The same responsive rendering is reused for `;` at its narrow docked width.

Exact width/height thresholds are tuned in the render harness against the
maintainer's 14-30 row sizes, not fixed in this doc.

## Task-oriented headers

Nearly free: the per-view Maps' `FullHelp()` already returns semantically grouped
columns (Global: "Help & Chrome / Views / Workspace / Actions"). Today they render
as unlabelled, blank-line-separated groups. The work is *labelling* those existing
groups with intent-oriented headers (e.g. SWITCH VIEWS / FIND / DO THINGS / CHROME),
not restructuring the Maps.

## Single-source-of-truth invariant (preserve)

The L1 footer (ShortHelp), `;` sidebar (FullHelp), and `?` overlay (FullHelp) all
consume the same per-view key.Map source, so drift is structurally impossible
(bt-ift6). This design keeps that: `?` consumes `m.keys.Global.FullHelp()`; `;`
consumes `viewSpecificKeyMap().FullHelp()` (dropping the Global prefix it currently
composes in via `sidebarHelpGroups()`).

## Implementation outline

(Full step plan is the job of the writing-plans pass; this is the shape.)

1. **`?` -> global only.** `renderHelpOverlay()` renders `m.keys.Global.FullHelp()`
   with task-oriented group headers + the status-glyph legend. Drop the 10-map
   `panels[]`.
2. **Responsive layout + live scroll.** Column count from width; wire `helpScroll`
   into the render with clamping; essentials-first group ordering.
3. **`;` -> view only.** `sidebarHelpGroups()` returns `viewSpecificKeyMap()`'s
   groups without the Global prefix. Fallback line for views with no view-specific
   map (Attention, LabelDashboard): "no actions here -- press ? for global".
4. **Cross-reference footers.** `?` footer mentions `;`; `;` footer mentions `?`.
5. **Reuse the panel/header rendering** built for `?` to render `;`'s view groups.

## Testing

- Tier 1 render harness (`render_harness_test.go`, `BT_RENDER_DUMP=1`): assert `?`
  fits and is non-clipping across the scrunched matrix (e.g. 70x20, 50x14, 30x20,
  120x40). Existing `modal_help_70x20` case is the bt-dx7k repro -- extend it.
- Assert content scoping: `?` over a given view shows global bindings and NOT that
  view's view-specific bindings; `;` shows the inverse.
- Assert `helpScroll` changes the rendered window (it currently does not).
- Live `bt` run at scrunched sizes before any push (dogfood-before-public-main).

## Out of scope (stays in bt-xavk)

- CRUD write-key vs read-key visual distinction (CRUD not shipped; bt-oiaj).
- L1 status-bar contextual hints redesign (footer redesign is its own bead, bt-d5wr).
- Run-context / disposition-vocabulary surfacing (bt-npxi integration target).
- Any `??` full-reference layer (explicitly dropped; see "What this supersedes").

## Acceptance (closes bt-dx7k)

- `?` overlay is usable and non-clipping at narrow widths / short heights.
- `?` shows global shortcuts only, responsively laid out, with working scroll.
- `;` shows the active view's shortcuts only, with a sensible empty-view fallback.
- Both surfaces carry a one-line cross-reference to the other.
- Single-source-of-truth invariant intact (no string tables reintroduced).
