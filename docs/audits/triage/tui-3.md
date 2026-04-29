# Triage: tui-3

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-oiaj | TUI read/write: create and edit beads from bt | GREEN | Architecture is "shell out to bd"; storage-agnostic by design and consistent with Dolt-only. | None. |
| bt-q4tn | Clickable bead refs in detail pane + back/forward navigation | GREEN | Pure TUI navigation feature; no storage assumptions. | None. |
| bt-qk1x | Display externally-filed beads differently in TUI | GREEN | Visual distinction based on provenance metadata; storage-agnostic. | None. |
| bt-qzgl | [epic] Graph view overhaul | GREEN | TUI graph rendering/scaling work; data-layer-agnostic. | None. |
| bt-rhqs | Label dashboard: key dispatch broken - h/H/L fall through | GREEN | TUI key dispatch + scoping bug; no storage backend assumptions. | None. |
| bt-s4b7 | Redesign project navigation: filtering vs switching vs context | GREEN | Project picker UX redesign; references Dolt database names as the registry source, consistent with Dolt-only. | None. |
| bt-s9sg | Notifications: cross-session activity via Source field | GREEN | Notifications feed render; depends on provenance beads but framing is storage-agnostic. | None. |
| bt-spzz | Smarter reload status: show what changed (added/updated/closed) | GREEN | Diffs in-memory snapshots; explicitly "reloads data from Dolt", aligned with current architecture. | None. |
| bt-t8g6 | Theming: improve and document theme system | GREEN | Pure styling/config work; no storage layer involvement. | None. |
| bt-tkhq | Research TUI keybinding conventions | GREEN | Research task on TUI conventions; storage-agnostic. | None. |
| bt-ty44 | Explore and document what each bt view does | GREEN | Documentation/exploration of TUI views; storage-agnostic. | None. |
| bt-vhhh | Detail-only view: can't navigate between issues with arrow keys | GREEN | Pure TUI navigation bug. | None. |
| bt-vk3v | Board view: G goes to bottom but no keybind to return to top | GREEN | Pure TUI keybinding bug. | None. |
| bt-vs7w | Attention view (]): barebones table, needs visual redesign | GREEN | TUI visual redesign of an existing view; storage-agnostic. | None. |
| bt-vv7o | TUI feature: blocked/waiting queue view (distinct from bd ready) | GREEN | New TUI view consuming existing analyzer; storage-agnostic. | None. |
| bt-wfss | Rethink 'Issues' panel title border | GREEN | Visual polish of panel border. | None. |
| bt-x47u | Standardize modal footer format across all overlays | GREEN | Pure TUI consistency task. | None. |
| bt-xavk | Redesign help system: layered, task-oriented, context-aware | GREEN | TUI help/onboarding redesign + shared keybinding registry; storage-agnostic. | None. |
| bt-xron | Filter keys (o/c/r) should toggle off when pressed again | GREEN | Pure TUI key toggle bug. | None. |
| bt-y0fv | Responsive layout: adapt views for small terminal dimensions | GREEN | Layout/responsive work; storage-agnostic. | None. |
| bt-yi2t | Theme system + settings UI (future) | GREEN | Theme/settings UI placeholder; storage-agnostic. | None. |
| bt-yjc0 | Revisit TYPE column icons - unclear what they mean | GREEN | Visual polish on TYPE icons. | None. |
| bt-z9ei | Lazydev vision: bt as lazygit-style project workspace | GREEN | Vision bead; storage-agnostic, frames bt as bd frontend (consistent with Dolt-only). | None. |
| bt-zdjr | Board view: poor use of screen space with empty columns | GREEN | Pure TUI layout work. | None. |

## Bucket totals
- GREEN: 24
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations

- Entire bucket is TUI/UX work that operates on already-loaded snapshots or shells out to `bd`. None of these beads bake in JSONL/sprint/correlator/SQLite assumptions in their acceptance criteria.
- bt-oiaj and bt-s4b7 explicitly call out the "shell out to bd, never write to Dolt directly" architecture, which is the right pattern for the Dolt-only world.
- bt-spzz mentions polling Dolt for changes (correct), and bt-s4b7 references "Dolt database names" as the project-registry source — both already grounded in the current architecture.
- bt-s9sg/bt-q4tn are recent (2026-04-26), already created by a session that knows the current architecture.
- No duplicates spotted within this bucket.
