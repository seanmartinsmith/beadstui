# Triage: no-area

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action | Suggested label |
|----|----|----|----|----|----|
| bt-byk | Gastown comparison: cross-session patterns for bt TUI | GREEN | Cross-session TUI surface evaluation; storage-agnostic, no Dolt assumptions. | None. | area:tui |
| bt-uh3c | bt robot surface improvements for cross-project trace consumers | GREEN | Pre-classified landmark — architecturally unblocked per phase-5 comment. | None. | area:cli |
| bt-zko2 | History view: 'Showing N/M' denominator uses global count when filtered | GREEN | TUI denominator scoping bug; data-source-agnostic, references active project filter not legacy paths. | None. | area:tui |

## Bucket totals
- GREEN: 3
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations
- Small bucket (3 beads). All three are framed in storage-agnostic terms — none reference JSONL paths, sprints, SQLite, metadata-blob session columns, or `--global` routing misframings.
- bt-uh3c is a pre-classified landmark (GREEN per rubric); included for completeness with the rubric-mandated suggested label.
- bt-byk and bt-zko2 are both TUI-shaped concerns (cross-session surface, history view denominator). `area:tui` fits both. bt-uh3c is robot-mode subcommand work, so `area:cli` fits better than `area:tui`.
- No cross-bucket duplicates spotted from this slice.
