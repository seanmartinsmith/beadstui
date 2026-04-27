# Triage: tui-1

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-19vp | History view: focused dogfood + bugfix pass | GREEN | TUI dogfood epic on `pkg/ui/history.go`; storage-agnostic. | None. |
| bt-249t | Notification offline capture: synthetic events for activity | GREEN | Uses `~/.bt/events.jsonl` (bt cache) + Dolt-state diff; not beads JSONL backend. | None. |
| bt-36h7 | Label picker redesign: overlay + UX parity with project filter | GREEN | Pure modal/UX redesign; storage-agnostic. | None. |
| bt-46fa | Redesign issue list column header (TYPE PRI STATUS ID TITLE) | GREEN | Header rendering refactor in pkg/ui; storage-agnostic. | None. |
| bt-4dam | Graph view: missing filter keys (a/o/c/r) — fall-through | GREEN | Keyboard handler bug in `handleGraphKeys()`; storage-agnostic. | None. |
| bt-4fxz | feature: integrate bd audit --actor and bd stats --group-by=actor | GREEN | Wraps upstream v1.0.1 primitives; explicitly Dolt-aware. | None. |
| bt-4yn4 | CASS session correlator finds no results for known issue IDs | GREEN | Cass-search tokenization bug, not bt's bd<->git correlator (bt-08sh). | None. |
| bt-53du | Product vision: bt v1 (epic) | GREEN | High-level roadmap epic; data-layer-agnostic framing. | None. |
| bt-5fbd | Flow matrix: keyboard input conflicts with navigation | GREEN | Pure input-routing TUI bug. | None. |
| bt-5gnr | Alerts-dismissed-log: decide audit-trail vs restore-log semantics | GREEN | TUI-local dismissed-alerts state; design decision, no storage assumption. | None. |
| bt-5hkm | decision: map molecule lifecycle to bt status + views | GREEN | References upstream `bd mol current` primitives; rendering-only decision. | None. |
| bt-65dk | BQL query modal UX polish: syntax hints, autocomplete | GREEN | Modal UX polish; storage-agnostic. | None. |
| bt-6cfg | Same-ID cross-prefix bead linking in global view | GREEN | Depends on upstream bd-k8b; concept is data-layer-agnostic. | None. |
| bt-6fn2 | Surface 'human' label as a workflow signal — not just a tag | GREEN | Label rendering / filter UX; storage-agnostic. | None. |
| bt-6yjh | Actionable view (a): wasteful layout, needs density rethink | GREEN | TUI density / layout refactor. | None. |
| bt-7czu | TUI feature: by-assignee view | GREEN | Properly references bt-5hl9 for session-column hydration; framing is current. | None. |
| bt-8col | Graph view: list selection doesn't carry to graph ego node | GREEN | View-switching state-passing fix; storage-agnostic. | None. |
| bt-8jds | Wisp toggle (w) inaccessible in workspace/global mode | GREEN | Keybinding conflict; storage-agnostic. | None. |
| bt-8zgy | TUI: surface IN_PROGRESS freshness in list + details pane | GREEN | Renders existing updated-at timestamp; storage-agnostic. | None. |
| bt-a3sb | TUI feature: project-grouped list view | GREEN | List-layout feature; uses already-available SourceRepo / ID-prefix. | None. |
| bt-arf9 | Mouse support for label filter modal (ModalLabelPicker) | GREEN | Pure TUI input feature on existing modal. | None. |
| bt-ba9f | TUI feature: pull up specific beads by explicit ID list | GREEN | TUI ID-list filter modal; storage-agnostic. | None. |
| bt-cl2m | Background data refresh closes open modals | GREEN | Modal lifecycle bug during Dolt-poll refresh; framing is Dolt-aware. | None. |
| bt-d5wr | Footer visual redesign: hierarchy, vocabulary, branding | GREEN | Pure visual-design pass on footer chrome. | None. |

## Bucket totals
- GREEN: 24
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations
- This bucket is a textbook case of the rubric's note: TUI work is mostly storage-agnostic, so the entire bucket is GREEN.
- bt-249t mentions `~/.bt/events.jsonl` and `~/.bt/baseline.json`, which are bt-only cache paths under `.bt/` (the bt-uahv-canonical bt cache root) — not beads JSONL backend. Framing is consistent with Dolt-only beads + bt-local cache.
- bt-4fxz is explicit about wrapping upstream v1.0.1 `bd audit` / `bd stats --group-by=actor` primitives (no reimplementation), which is the post-Dolt-only correct shape.
- bt-7czu correctly defers session-column integration to bt-5hl9 (the in-progress migration), so its framing tracks current reality.
- bt-4yn4 is a CASS-side correlator (not the bt bd<->git correlator owned by bt-08sh) — different system; classification not affected by the correlator migration.
- No beads in this bucket reference `.beads/<project>.jsonl`, `loader.FindJSONLPath`, `sprints.jsonl`, SQLite as backend, the removed daemon registry, or session columns sourced from `metadata`.
