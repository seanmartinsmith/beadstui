# Footer + Notification Center Redesign

**Bead**: bt-d5wr
**Status**: Design approved, ready for implementation planning
**Date**: 2026-04-23
**Author**: brainstorm session with sms

## Context

bt-m9te closed with tactical polish (inline status path, auto-dismiss tick, project badge fix, width-aware compression tiers) but left the underlying "information hierarchy is intentional" design bullet untouched. The footer has ~15 distinct badge styles that accreted over time; notifications share a single inline slot with key hints; there is no dedicated notification surface. Additionally, the user wants a real-time view of bead activity across projects as sessions run and modify beads concurrently.

This spec covers three intertwined subsystems brainstormed together so each design choice composes with the others:

1. **Event model** — how bead changes become structured events
2. **Notification center modal** — where event history is inspected (`1` key)
3. **Footer redesign** — how events surface in the ambient footer and how the footer's visual vocabulary tightens up

## Scope

**In scope:**

- Event schema, kinds, source field, collapsing semantics, retention model
- Notification center modal (structure, filtering, sorting, grouping, keybindings, accent)
- Footer structural changes (ticker slot, count badge, refresh pulse, tier integration)
- Typography pass on footer (selective adoption of direction B's text-first style)
- Badge vocabulary reduction

**Out of scope (deferred to future sessions):**

- Theme system overhaul beyond the existing color tokens → bt-yi2t
- Settings UI → bt-yi2t
- User-configurable keybindings → bt-yi2t
- CASS live session stream as event source — reserve the `EventSource` enum slot, do not implement
- Modal footer style extraction to a shared module → bt-x47u (last step after this redesign lands)

## Design

### 1. Event model

Events are emitted by diffing the data snapshot between Dolt polls. bt already polls every 5s via `BackgroundWorker.startDoltPollLoop` and receives `SnapshotReadyMsg` when content changes. The diff runs in the `handleSnapshotReady` path and emits events into an in-memory ring buffer on `Model`.

**Schema:**

```go
package events

type EventKind int

const (
    EventCreated EventKind = iota
    EventEdited
    EventClosed
    EventCommented
)

type EventSource int

const (
    SourceDolt EventSource = iota
    SourceCass // reserved for future; do not emit in v1
)

type Event struct {
    ID        string      // stable hash of (BeadID, Kind, At) for dedup/dismissal
    Kind      EventKind
    BeadID    string      // "bt-1u3"
    Repo      string      // "bt" (derived from ID prefix)
    Title     string      // snapshot title at event time
    Summary   string      // kind-dependent: "+ 3 fields", "Audio callback spike", comment excerpt
    Actor     string      // assignee / last-updated-by, if available; "" otherwise
    At        time.Time
    Source    EventSource
    Dismissed bool
}
```

**Kind derivation from snapshot diff:**

| Kind             | Trigger                                                           | `Summary` format                    |
|------------------|-------------------------------------------------------------------|-------------------------------------|
| `EventCreated`   | BeadID present in new, absent in prior                            | current title                       |
| `EventClosed`    | status transitioned `open → closed`                               | current title                       |
| `EventEdited`    | any field other than `updated_at` changed                         | named fields if ≤3 changed, else `+ N fields` |
| `EventCommented` | comment count increased                                           | first 80 chars of new comment       |

Field-change detection compares each field individually. When ≤3 fields changed, `Summary` lists them as `+ priority, + label, + title`. When >3 changed, `Summary` is the aggregate `+ N fields`.

**Actor resolution**: use the bead's `assignee` if populated, else `last_updated_by` if the schema exposes it, else `""`. Actor is informational in v1 — it renders in the modal row when non-empty but does not participate in filtering.

**Retention**: in-memory ring buffer, session-scoped, capacity 500 events. On capacity, oldest event evicted. Not persisted across bt restarts. Future work: optional Dolt-backed persistence.

**Storage**: `events []Event` on `Model`, protected by `sync.RWMutex`. Writes only from `handleSnapshotReady`. Reads from footer render, modal render, and ticker collapse transform.

**Collapsing** is a *view* on raw events, not a storage transform:

- **Storage**: every detected change is an independent `Event`. Three quick edits to `bt-1u3` create three records.
- **Modal**: renders the raw list verbatim. Full granularity because the modal is an intentional inspection surface.
- **Ticker**: applies `collapseForTicker(events, 30s)` — same `BeadID` + same `Kind` within a 30s window fold into the most recent event with an aggregated `Summary` (e.g., `+ 3 fields`). Dismissing from the ticker dismisses the entire group.
- **Count badge**: reflects raw event count, matching what the modal will display. No "opening the modal reveals more than the badge suggested" surprise.

### 2. Notification center modal

Fork the existing alerts modal scaffold (`pkg/ui/` alerts panel render + filter row + list + page navigation). Swap data source to the event ring buffer, relabel the filter axes, change the accent color.

**Keybindings:**
- `1` — open
- `esc` / `1` / `q` — close
- `j/k` — navigate list; `g/G` — top/bottom; page keys follow alerts behavior
- `⏎` — close modal, jump to bead in the main list (global-mode aware)
- `d` — dismiss current row
- `a` — dismiss all (confirm prompt)
- `t` — cycle event type filter (`all → created → edited → closed → commented → all`)
- `r` — cycle repo filter (`all → repo1 → repo2 → ... → all`)
- `s` — cycle since-filter (`all time → last hour → today → session → all time`)
- `o` — toggle sort (`newest ↔ oldest`)
- `g` — cycle grouping (`flat → by repo → by type → flat`)

Filters compose. Header displays the active filter summary.

**Header format:**

```
Notifications · 42 total · 12 created · 18 edited · 8 closed · 4 commented
```

With filters active:

```
Notifications · 4 match · filter: type=edited repo=bt
```

**Row format:**

```
▸ [edit]   bt-1u3       Footer tier compression landed           2s ago
  [new]    portal-349   Audio callback spike                     15s ago
  [comm]   cass-4yj     "Index rebuild finished"                 1m ago
  [done]   bt-d8d1      Mouse click-to-focus                     3m ago
```

Cursor row prefixed with `▸` + subtle highlight. Kind tag color-coded per section 4's color roles. Bead ID bold. Title truncated to available width. Relative time muted, right-aligned.

**Empty state**: `No notifications yet — bead activity will appear here.` centered in the modal body.

**Jump behavior**: `⏎` closes the modal and navigates the main list to the event's bead. If the current list filter would hide the bead (e.g., active filter is `closed` and the bead is open, or active repo filter excludes the bead's repo), the filter resets to `all` / all-repos before the cursor moves. Better to land the user on the bead than preserve a filter they'll have to clear anyway. Event is not auto-dismissed — only `d` dismisses.

**Accent**: teal (`ColorPrimary`) border + header, distinct from alerts' red. Event-kind tag colors inherit from section 4.

**Dismissal persistence**: dismissal state lives alongside the event in the ring buffer. Persists across modal close/reopen within a session. Resets on bt restart (matches ring buffer retention).

### 3. Footer redesign

Footer has three visual states: idle, ticker-active, idle-with-pending.

**Idle** (no recent events, count = 0):

```
 ALL   bt   ○1 ◉2 ◈3 ●4            35          ⏎ details   t diff   S triage   ?
```

Key hints render as `<bold-key> <dim-action>` pairs separated by two spaces. No chrome background on hints, no `│` separators. Identity anchored by filter badge.

**Ticker-active** (event within last 3s):

```
 ALL   bt   ○1 ◉2 ◈3 ●4            35    ✓ bt-1u3 edited          🔔 3
```

Ticker slot overlays the key-hints region (reuses bt-y0k7 inline status path). Format: `<kind-icon> <bead-id> <kind-text>`. Truncates bead title before it truncates the kind glyph when space is tight.

**Idle with pending unread** (ticker window expired, count > 0):

```
 ALL   bt   ○1 ◉2 ◈3 ●4            35          ⏎ details   t diff   S triage   🔔 3
```

Count badge parks on the far right. `🔔 0` is never rendered — when count drops to zero (via dismiss-all or jump-and-clear), the badge disappears.

**Refresh pulse**: on every `SnapshotReadyMsg`, set `lastRefreshAt = time.Now()` on Model. Footer render includes a muted `↻` glyph when `time.Since(lastRefreshAt) < 500ms`. Position: immediately left of the count badge (or of the `?` hint if count = 0). Fades on the next `statusTick` (1s cadence already in place). No chrome, single char, `ColorMuted` foreground.

The `↻` pulse replaces the current `Reloaded N issues` persistent status. That message becomes obsolete — pulse is the "fresh data" signal, events are the "what changed" signal.

**Tier integration**:
- Count badge `🔔 N`: **tier 0** (always visible when N > 0; hidden when N = 0).
- Ticker slot: ephemeral, does not participate in tier compression (overlays tier-0 hints; already truncates internally).
- Refresh pulse: ephemeral, does not participate in tier compression.

**Footer element trims** (cumulative with what already landed today in bt-m9te):
- `Reloaded N issues` persistent status → dropped (subsumed by pulse + events)
- `issues` suffix on total count → `35 issues` becomes `35`
- Key hints: swap chromed `<key>` + subtext action to `<bold-key> <dim-action>` pairs, `│` separators replaced with two-space gap

### 4. Visual polish & color tokens

Reuse existing color tokens. No new palette.

**Color roles:**

| Token                 | Role                                                                   |
|-----------------------|------------------------------------------------------------------------|
| `ColorPrimary`        | filter badge, notification modal accent, `bt` identity presence        |
| `ColorPrioCritical`   | alerts modal accent, critical badges                                   |
| `ColorSuccess`        | `✓` ticker prefix, `[edit]` kind tag, open stats glyph                 |
| `ColorInfo`           | `[new]` kind tag, sessions badge, phase2 hint                          |
| `ColorWarning`        | `[comm]` kind tag, worker warning state, blocked stats glyph           |
| `ColorMuted`          | timestamps, key hints (dim half), `↻` pulse, `[done]` kind, closed stats |
| `ColorSubtext`        | deprecated role; migrated to `ColorMuted` where used in footer hints   |
| `ColorText`           | bead IDs in ticker/modal, bold half of key hints, default row text     |

**Typography pass** (selective direction-B adoption):

- Key hints: `<bold-key> <dim-action>`, two-space separator, no `│`, no chrome background
- Count suffixes dropped where icon/glyph carries meaning (`35` not `35 issues`; `📦 16` not `📦 16 projects`)
- Bead IDs in any context: bold, default color
- Timestamps: `ColorMuted`, lower-case relative (`2s ago`, `15s ago`, `1m ago`)

**Badge vocabulary reduction** (target: ~7 primitives from current ~15):

| Primitive                   | Used by                                                                       |
|-----------------------------|-------------------------------------------------------------------------------|
| background-badge            | filter, stats quad, alerts, notification count, worker (warn/crit), dataset, phase2 |
| foreground-badge (no bg)    | project, search, sort, wisp, update                                           |
| muted-text                  | timestamps, watcher, workspace summary, refresh pulse                         |
| bold-key                    | key hint keys                                                                 |
| dim-action                  | key hint actions                                                              |
| stats-group                 | stats quad (inherits background-badge, internal glyph colors)                 |
| ticker-line                 | event ticker (no bg, icon + bead-id-bold + summary)                           |

**Empty footer state**: never empty. Minimal is filter badge + stats + one hint + `?`. Clean without hand-holding.

### 5. Implementation sequencing

All follow-on work tracked under the bt-d5wr design umbrella. Concrete bead IDs assigned during plan phase.

1. **bt-d5wr** (this spec) — closes when design doc is committed and user-approved.
2. **Event pipeline bead** — `pkg/ui/events/` package: `Event`, `EventKind`, `EventSource`, ring buffer, snapshot diff function, collapse helper. Wire into `handleSnapshotReady`. No UI changes. Fully testable independently. Unblocks everything downstream.
3. **Footer redesign bead** — structural + typography work from sections 3 and 4. Independent of event pipeline but best landed after so ticker and badge slots are live. Absorbs bt-spzz's acceptance criteria.
4. **Notification center modal bead** — fork alerts modal scaffold, swap data source to event ring buffer, implement filter/sort/group mechanics. Depends on event pipeline.
5. **bt-x47u** (existing) — extract tier/compression/style primitives into a shared helper, apply to modal footers. Last step.

**Closures triggered by this design:**
- **bt-spzz** ("Smarter reload status: show what changed") — acceptance criteria fully subsumed by the event pipeline + footer redesign beads. Close as duplicate when event pipeline lands.

**Deferred:**
- **bt-yi2t** — theme system + settings UI, future session.

## Risk

- **Snapshot diff performance**: comparing issue lists on every poll in global mode (3000+ issues across 16 projects) is a linear scan per field per issue. Profile early; consider hashing field-sets per issue for O(1) change detection if it shows up in bench.
- **Dismissal UX**: dismissing events one-by-one in a 500-event ring could be tedious. `a` dismiss-all is the escape valve. Real-world usage will clarify whether finer-grained bulk operations (`d repo:bt` dismiss-by-filter) are needed.
- **CASS reservation**: adding `EventSource` now prevents churn later but adds a field that ships unused in v1. Acceptable — single enum field, zero runtime cost.
- **Key `1` collision**: no current binding on `1` at the top level. Confirmed against the keybindings audit (`docs/audit/keybindings-audit.md`).

## Verification plan

Each implementation bead has its own verification, but the end-to-end acceptance for this design:

- Trigger `bd update bt-some-id --priority P0` in a second shell while bt is open
- Within ~5s (poll cycle), the footer shows `✓ bt-some-id edited` as a ticker
- After 3s, ticker clears; count badge shows `🔔 1`
- Press `1` — modal opens with one row: `[edit] bt-some-id <title> <time>`
- Press `⏎` — modal closes, main list cursor is on `bt-some-id`
- Do 3 rapid `bd update` on same bead — modal shows 3 rows (raw), ticker showed 1 collapsed row (only the last one visible)
- Press `1`, `d`, `esc` — bead dismissed, count decremented
- Restart bt — notification center is empty (ring buffer is session-scoped)
