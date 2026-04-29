# Triage: bql

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-338n | Security: cap BQL IN-list at 1000 values | GREEN | Parser-level resource cap in `pkg/bql/parser.go`; storage-agnostic, no Dolt assumptions. | None. |
| bt-faaw | BQL syntax highlighting in query modal | GREEN | TUI cosmetic feature against BQL grammar; no data-layer assumptions. | None. |
| bt-hmt9 | Security: BQL fieldToColumn rejects unknown fields or Build calls Validate | GREEN | SQL-injection hardening in `pkg/bql/sql.go`; targets BQL builder, not storage backend. | None. |
| bt-llh2 | BQL parse-error hints: suggest id="X" form when : or unquoted RHS is used | GREEN | Parser error-message UX; references bt-uh3c (already GREEN landmark) and is data-layer-agnostic. | None. |
| bt-sytt | Evolve recipes into saved BQL queries | GREEN | Recipe format change in `pkg/recipe/`; BQL expression strings, no JSONL/SQLite assumptions. | None. |

## Bucket totals
- GREEN: 5
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations
- Entire bucket is BQL-layer work (parser, SQL builder, recipe format, TUI highlighting, error messages). None of these reach into data-source loading, session columns, sprints, correlations, or `--global` routing — i.e., none of the stale-assumption tells from the rubric apply.
- bt-338n and bt-hmt9 are siblings (both children of bt-6cdi, security audit parent) and `relates-to` each other; consistent classification.
- bt-llh2 references bt-uh3c (pre-classified GREEN) and remains valid even after item 1 of that bead lands — error-message hints help all BQL usage, not just identity lookup.
- bt-sytt is the only forward-looking architectural change in the bucket (unify recipes onto BQL); still purely above the storage layer.
