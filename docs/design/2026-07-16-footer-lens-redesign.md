# Footer Lens Redesign

**Date**: 2026-07-16
**Status**: Design approved (interactive walk-through with maintainer, session pc:bt:2c14befc)
**Supersedes**: the visual vocabulary and slot inventory of
[2026-06-21-tui-footer-smart-redesign.md](2026-06-21-tui-footer-smart-redesign.md).
The 6-21 doc's zone architecture and degradation engine survive; its glyph choices,
mockups, and left-zone contents are replaced by this doc.
**Prior art shipped before this design**: degradation engine (bt-a3zi3), bell + toast
Phase 4 (bt-a3zi3.1), content re-tiering (bt-ujwiq, PR #30), anomaly-only badge
classification (bt-jhzat, PR #31), dark-cockpit decision (bt-9gjt0).

---

## One-line thesis

The footer is a lens statement plus an actionable summary: *where am I, what's
filtered, how it's ordered, what can I act on* - in plain ASCII, quiet when healthy,
with every evicted piece of chrome given an explicit new home.

## The line (mockups, ASCII default)

```
160| ALL(19) . st:open . lb:- . /- . by:updated     ready 214 . in-flight 41 . blocked 87 . 1903     ? help . ; shortcuts    !4 *383
100| bt . st:open . by:updated        ready 41 . in-flight 4 . blocked 12 . 169        ? . ;    !4 *9
 70| bt . open         ready 41 . blocked 12 . 169         ? ;   !4
 50| bt . open       169       ? ;   !4
```

(Separator rendered as a middle dot at display time; shown as `.` here to keep this
doc greppable. No background fills anywhere - plain text with typographic hierarchy:
bold primary, dim secondary, color for state.)

## Zone 1 - Lens (left)

Reads as a sentence: where am I -> what's filtered -> how it's ordered.

- **Scope, leftmost and informative**: `bt` (single project) or `ALL(19)` (all
  projects, with count). Replaces the uninformative `[19]` box. "Which project am I
  in" is question zero.
- **Filter bucket** (dimensions that change list *membership*), each one-key
  cyclable: status (`st:all|open|in-progress|blocked|closed|deferred` - one key
  cycles, per bt-gpvwe), label (`lb:`), search/BQL (`/query`). Recipe filter joins
  this bucket when active.
- **Order bucket** (changes arrangement only): `by:updated|created|priority|...`
  (canonical list = whatever sort modes exist in code; enumerate at implementation).
- **Defaults hold space at wide widths** (`lb:-`, `/-`) so the lens has a stable
  silhouette; placeholders are the first drop under width pressure.
- Killed outright: the `l:labels` hint chip, the `[19]` mystery badge, and the
  `Filter: Open issues` right-side echo (the toast must never announce what the lens
  already shows).

## Zone 2 - Center (actionable numbers)

- Default: **actionable triad scoped to the lens**: `ready N . in-flight N .
  blocked N . total`. Answers "what can I do right now", not "what exists".
  Zero-count segments never render. Raw per-status tallies (o1903 etc.) die.
- **Per-view meaning overrides** (existing CenterOverride mechanism), granted only
  when the view cannot show the information itself: detail = `bt-0qzp . 3/169`
  (list position), memories = `127 memories . 10 projects`. Board and graph keep
  the default triad - a view counting its own visible elements is low information
  (maintainer read-through 2026-07-16; "cards" vocabulary rejected).
- Toasts borrow this slot briefly and give it back (Phase 4 semantics unchanged).

## Zone 3 - Right (affordances + signal)

- Static discoverability pair: `? help . ; shortcuts` (label amended from
  `; keys` 2026-07-17, bt-x5lvp - name the surface, not the key) - the help overlay and shortcuts
  sidebar are the two surfaces that actually teach navigation (maintainer call,
  2026-07-16). Per-view action pills are GONE from the chrome; the per-view
  `key.Map`s feed `?` and `;` only. Wide widths show labels, narrow degrades to
  bare `? ;`. This deletes the footer's hint-pill degradation machinery and gives
  the reclaimed width to the lens and triad.
- **Anomaly badge `!N`**: absent at zero (dark cockpit). Traceable: activating it
  opens the alerts modal filtered to exactly those N anomalies.
- **Bell `*N`**: unread events (ring buffer). Exact ASCII sigils (`!`, `*`) are
  implementation-adjustable; the contract is: one char, ASCII, distinct.

## Glyph system (cross-cutting, applies TUI-wide)

- One glyph table, two tiers: **nerdfont (DEFAULT)** / ascii (fallback). Single
  table all chrome glyphs read from: status marks, scope marks, bell, anomaly,
  tree/graph connectors. Amended 2026-07-16 after the NF preview: maintainer wants
  nerd font as the default experience. No terminal API reports installed fonts, so
  "detect nerd fonts" is impossible - the shape is starship/yazi precedent: NF on
  by default, `BT_GLYPHS=ascii` (env or config) as the graceful fallback. The
  toggle joins the configurable-knob list (bt-ey2hh) for the settings screen epic
  (bt-fd3k). The doc's mockups show the ASCII tier to stay greppable.
- **Emoji tier: deleted from the codebase**, not defaulted off. Maintainer signal
  (2026-07-16, repeated from earlier sessions): emoji break layout math by
  rendering double-width; get away from them entirely.
- Unicode-but-not-emoji marks (circle dots, return arrow) count as non-ASCII: they
  live in the nerdfont tier only.

## Relocations - where evicted chrome lives

Every eviction has an explicit destination; nothing is silently dropped.

1. **Alerts modal status header ("bt status report")** - a persistent header section
   at the top of the alerts modal:
   - Line 1 reconciles the badge: `4 anomalies . 1483 advisories` (the badge number
     is always explainable here; fixes the 4-vs-1487 confusion, dogfood 2026-07-16).
   - Dolt mode (embedded / server / shared-server / global) + server status.
   - Per-source health - the home of "N sources unavailable" (bt-2ea7t.10 routes here).
   - Watcher / background worker / reload state; data freshness.
   - Corpus scale facts: per-source issue counts, DB sizes, load timings (feeds from
     bt-2ea7t.7 instrumentation). Candidate (scoped, not v1-committed): sync/backup
     awareness - "source X: no Dolt remote / last push 40d ago".
2. **Consequences, not thresholds**: the `large 5k issues` badge dies. If Phase-2
   analysis actually times out or a render degrades, THAT is the alert + status
   line ("phase-2 fell back at 5.6k issues"); a big corpus alone is a status-report
   fact, not a warning. (Reframes bt-ajbxw.)
3. **Notification content -> toast layer** (yazi-style bubble, bt-kuvzj). Footer
   keeps only the bell.
4. **Boot chrome silent on success**: "background mode enabled" and friends speak
   only on failure ("sync reload fallback: <why>").
5. **Server mode in the footer only when degraded** - healthy server = silence.

## Branding

Utilitarian chrome: no bt name/logo in the footer. Version + identity live in the
status report header. (Resolves bt-d5wr's branding question.)

## Degradation and expansion

- Engine unchanged (never wraps; ansi-aware truncate as safety net).
- Drop order: lens placeholders -> daemon/degraded-state badges (only present when
  degraded anyway) -> triad segments (keep total) -> hint labels (`? help . ;
  shortcuts` -> `? ;`) -> lens filter words (scope survives) -> last: `scope .
  total . ? ; . !N`.
- **Expansion is deliberate** (new): as width grows, hints regain labels, the triad
  regains segments, lens placeholders reappear. What expands is specified, not
  incidental.

## Testing

- Render harness scenarios at 50/70/100/130/160 for list/detail/board/graph/memories
  in both glyph tiers; assert 1 row, no mid-token clip, lens correctness per state.
- Unit tests: lens composer (filter/order buckets, placeholder behavior), triad
  computation against the analysis layer, badge absence at zero.

## Phasing (implementation wave - fresh sessions, not this one)

1. **Glyph system + emoji deletion** (cross-cutting prerequisite; touches all views).
2. **Lens zone** (grammar, space-holding defaults, kills l:labels/[19]/filter-echo).
3. **Actionable triad + per-view center extensions** (memories line included).
4. **Alerts status header + traceable badge + consequences-not-thresholds**
   (absorbs bt-ajbxw reframe, bt-2ea7t.10 routing).
5. **Toast layer migration** (bt-kuvzj) + boot-chrome silence.

Phases 2-5 are independent after 1; each ships with harness scenarios.

## Resolved bead questions

- bt-d5wr: visual vocabulary (plain text + typographic hierarchy), density (lens +
  triad + hints only), branding (utilitarian), notification surface (toast + bell) -
  all answered here.
- bt-d5wr.1: slot ordering = scope, filters, order, center, hints, badges - answered.
- bt-9gjt0 / bt-ujwiq / bt-jhzat: unchanged, this design builds on them.
