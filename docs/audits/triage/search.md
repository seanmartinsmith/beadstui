# Triage: search

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-7rt4 | Search UX: / should work from details pane + preserve position on exit | GREEN | Pure TUI keybind/focus behavior; no storage-layer assumptions. | None. |
| bt-hazr | Switch default search mode to semantic | GREEN | Default-mode toggle in `pkg/search/`; storage-agnostic about indexed content. | None. |
| bt-ox4a | Decision: default search mode (semantic vs fuzzy vs hybrid) | GREEN | Discusses bt's own FTS5 SQLite search index in `.bt/` and embedding choices; no stale beads-backend assumptions (the removed SQLite was beads', not bt's search index). | None. |

## Bucket totals
- GREEN: 3
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations

- All three beads concern bt's own search subsystem (`pkg/search/`, FTS5 index in `.bt/`), which is fully storage-agnostic about how beads are sourced — the indexer consumes whatever loader produces (now Dolt, formerly JSONL). None of these beads reach into the beads-source-of-truth layer.
- bt-ox4a explicitly references SQLite as bt's FTS5 backend; this is not the removed v0.56.1 beads SQLite backend, so it is not a stale tell.
- bt-hazr and bt-ox4a overlap conceptually (bt-hazr proposes the change, bt-ox4a is the decision-record bead with full tradeoff context). bt-ox4a should likely supersede or absorb bt-hazr once the decision lands — flagging as a possible consolidation, but outside this audit's architectural scope.
