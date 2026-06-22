# Smart Footer Phase 4 - Notifications (toast + unread bell)

**Date**: 2026-06-22
**Status**: Design approved (brainstorm 2026-06-22); ready for implementation plan
**Beads**: `bt-a3zi3.1` (this phase) under umbrella `bt-a3zi3`
**Refines**: [docs/design/2026-06-21-tui-footer-smart-redesign.md](2026-06-21-tui-footer-smart-redesign.md) section 5
**Follow-ups (out of scope, tracked)**: `bt-vhzia` (cross-session unread), `bt-s2duc` (multi-select modal triage)

<!-- Related: bt-a3zi3.1 bt-a3zi3 bt-vhzia bt-s2duc bt-46p6.10 -->

---

## 1. Thesis

The footer carries two kinds of notification, and they are different things:

- a **transient toast** - ephemeral session status (a refresh result, a write
  error) that borrows the hint slot, then yields it back; and
- a **permanent unread bell** (`🔔N`) - a persistent attention signal sourced from
  the events ring buffer, whose full feed lives in the existing alerts modal.

They are correlated but distinct channels. A refresh fires a success *toast* and
the new beads it pulled in independently bump the *bell* (each new bead is already
a ring-buffer event). The toast is momentary; the bell is the record.

This refines section 5 of the parent design, which named the toast and the bell
but left the taxonomy, timing, and badge semantics to this brainstorm.

---

## 2. Toast taxonomy (severity-driven)

The toast's lifetime and footprint are driven by severity, not chosen ad hoc.
Four severities, three distinct lifetime behaviors (glyphs still differ):

| Severity | Examples (real call sites) | Glyph | Footer lifetime | Bell entry? |
|---|---|---|---|---|
| Success | `reloaded +3 -1`, `Copied bt-123`, `Filter: Open issues` | `✓` | ~3s auto-fade | no |
| Notice (T1) | `No issue selected`, `No git remote configured`, `select an issue to enable swarm view` | (none / subtle) | ~3s auto-fade | no |
| Failure (T2) | `write failed: db locked`, `Export failed`, `Time-travel failed`, `Semantic search unavailable`, `Failed to open editor` | `✗` | ~8s auto-fade | **yes** |
| Degraded (T3) | `Dolt server unreachable (retrying)`, `Reload error (retrying)` | `⚠` | sticky until the condition **self-resolves** | **yes** |

Key rules:

- **Success and Notice never enter the bell.** They have no recovery value; the
  bell stays free of `No issue selected`-style noise. Success toasts also need no
  bell entry of their own because the underlying bead changes already produced
  ring-buffer events.
- **Failure** is a one-shot operation failure. It auto-fades after a longer window
  (~8s) AND appends an `EventSystem` to the ring buffer, so it is recoverable in
  the alerts modal after the toast is gone.
- **Degraded** is a live, ongoing condition. It is sticky - but sticky *until the
  condition resolves*, not until the user acknowledges. The recovery path (a
  successful reload / reconnect) clears it. It also appends an `EventSystem`.
- **Precedence**: a live Degraded toast is not overwritten by an incoming Success
  or Notice. Among non-sticky toasts, newest wins (current last-write behavior).

### Why "sticky until resolved" instead of "sticky until acknowledged"

Because Failure and Degraded both land in the bell, the toast is no longer the
error's only lifeline - nothing is lost if it fades. That removes the need for an
ack gesture (and the "how do I dismiss this?" question). The only genuinely-sticky
case is a live condition, and the natural thing to clear it is the condition
ending, not a keypress.

---

## 3. Severity classification of existing call sites

The codebase already distinguishes these behaviors via three setters
(`setStatus`, `setStatusError`, `setInlineTransientStatus` in
`pkg/ui/model_footer.go`). Phase 4 assigns each error a severity. Reference
classification of the 41 `setStatusError` sites (the plan applies this
mechanically):

- **Notice (T1)** - validation / rejection, nothing happened:
  `No issue selected`, `No commit selected`, `No bead selected`,
  `select an issue to enable swarm view`, `No git remote configured`,
  `Clipboard error` (board/history/list), `No issue selected` (export),
  `Invalid item type`.
- **Failure (T2)** - one-shot operation failed:
  `Export failed`, `Time-travel failed` (model_modes), `Failed to open editor`
  and the `$EDITOR`/`.beads` config errors (model_editor),
  `Failed to update <file>` (model_update_input write path),
  `Semantic search unavailable`, `Hybrid search unavailable`,
  `History load failed` (model_update_analysis), `Reload failed`,
  `Could not open browser`.
- **Degraded (T3)** - live, retrying condition:
  `Dolt server unreachable (retrying in Ns)`, `Reload error (retrying)`
  (model_update_data).

The ~90 `setStatus` / 5 `setInlineTransientStatus` sites are Success/Notice and
keep their auto-fade, no-bell behavior.

---

## 4. The bell badge - seen vs dismissed (two states, both true)

The ring buffer is already an inbox: the alerts modal shows un-dismissed events,
`d` reveals the dismissed archive, `c` dismisses the row under the cursor, `C`
dismisses all (`handleNotificationsKey`, `pkg/ui/model_update_input.go`).

The footer bell and the modal inbox answer different questions, so they track
different state:

- **Footer `🔔N` = "anything new since I last looked?"** Cleared by *opening* the
  notifications view.
- **Modal list = manual-triage inbox.** Cleared only by explicit dismiss (`c`/`C`).
  Opening to peek must NOT auto-archive items you still mean to handle.

### Mechanism (Phase 4, session-scoped)

A single session high-water-mark timestamp `alertsSeenAt` on the Model:

- `🔔N` = count of ring-buffer events with `At` after `alertsSeenAt` and
  `!Dismissed`.
- Opening the notifications view sets `alertsSeenAt = now` -> the footer zeroes.
  The modal's dismiss logic is untouched; items remain for triage.
- `alertsSeenAt` initializes at **boot**, so the badge starts at 0 each session
  and counts this session's activity. Notifications that arrived while bt was
  closed do not surface in Phase 4 - that durability is deliberately deferred to
  `bt-vhzia` (persisted per-event seen flag), so this phase stays footer-scoped.

This is literally "events since you last cleared," matching the maintainer's
framing. No new field on `events.Event`, no persistence change.

### Rendering

- Always render `🔔`; append `N` only when `N > 0`. The bell is both the activity
  signal and the affordance to open the alerts modal.
- Pinned in the right zone, last to drop under width pressure, alongside `?`
  (per the Phase 1 degradation engine).

---

## 5. Rendering and invariants

- The toast renders in the center/hint zone as a severity-glyphed override
  (`✓ ... +N -M` / `✗ ...` / `⚠ ...`) and yields back to `ShortHelp()` hints when
  its timer expires (Degraded yields back only when the condition clears).
- Both the toast and the `🔔N` badge route through the existing Phase 1 never-wrap
  degradation engine in `FooterData.Render()` and the bt-yyked footer-pin
  invariant (the footer always owns exactly the bottom row). The toast and badge
  must survive both: never wrap, never clip, always one row.
- Under width pressure the toast degrades like any center content (label -> glyph
  -> drop); the bell is pinned with `?` as a last survivor.

---

## 6. Components touched

- `pkg/ui/model_footer.go` - severity-aware toast model (extends the existing
  `statusMsg` / `statusIsError` / `statusIsInline` + `FooterData.StatusMsg` /
  `StatusIsErr` / `StatusIsInline`); the bell badge in the right zone of
  `FooterData.Render()`.
- `pkg/ui/events/ring.go` - no change to the type; Failure/Degraded toasts call
  `Append` with an `EventSystem` event (the kind already exists for ambient
  bt-emitted signals).
- The ~41 `setStatusError` call sites - reclassified per section 3.
- The Model - new `alertsSeenAt time.Time`; set on opening the notifications view.
- `handleNotificationsKey` / modal open path - set `alertsSeenAt = now` (footer
  clear) without touching dismiss state.

---

## 7. Testing

- **Render harness** (`pkg/ui/render_harness_test.go`, `BT_RENDER_DUMP=1`): footer
  scenarios at 60/70/80/100/120/160 across list/detail/board/graph for:
  idle, success toast, failure toast, degraded toast, badge N=0, badge N>0.
  Assert (a) footer is exactly 1 row, (b) no mid-pill clip, (c) the bell is
  present and `?` survives, (d) the toast yields back to hints.
- **Unit**: severity -> (glyph, lifetime, bell-append) mapping; Failure/Degraded
  append an `EventSystem`, Success/Notice do not; `alertsSeenAt` clears the footer
  count but leaves modal dismiss state intact; Degraded self-clears on the
  recovery path.

---

## 8. Out of scope (tracked, not lost)

- **Cross-session "unread while away"** - a persisted per-event seen flag so the
  bell survives restarts. `bt-vhzia` (P2). Phase 4's `alertsSeenAt` boot-init is
  the deliberate punt this bead revisits.
- **Multi-select batch dismiss in the modal** - mark several, dismiss the
  selection, keep the rest. `bt-s2duc` (P3). Per-item (`c`) and all (`C`) already
  exist; this is the batch gap.
- Reworking the modal's dismiss/archive data model.

---

## 9. Decisions deferred to the implementation plan

- Setter API shape: a single `setToast(msg, severity)` vs adding
  `setNotice`/`setFailure`/`setDegraded` alongside the existing setters. The
  classification in section 3 is independent of this choice.
- Exact recovery hook for clearing a Degraded toast (where the successful
  reload/reconnect path signals "condition resolved").
- Whether the alerts-tab (not just the notifications-tab) open should also clear
  `alertsSeenAt`.
