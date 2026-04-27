# Triage: data

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-05zt | ADR-003 implementation: SQLite reader removal + SourceType abstraction | GREEN | Pre-classified landmark; ADR-003 IS the Dolt-era cleanup. | None. |
| bt-2cvx | Session author provenance: track which session/project filed a bead | YELLOW | Scope still lists "new Dolt columns vs metadata JSON" as an open question — written pre bd-34v Phase 1a, where session columns are now first-class on the issues table. Display/cross-project identity work is still valid. | Append corrective comment: scope question is resolved (use first-class columns via bt-5hl9 hydration, not metadata blob); reframe as TUI display feature. |
| bt-2gvf | Security: drop DSN topology from Dolt connection error wrapping | GREEN | Bug is purely against the live Dolt connection wrapper; framing is Dolt-only. | None. |
| bt-5hl9 | feature: hydrate created_by_session / claimed_by_session from Dolt | YELLOW | Pre-classified landmark; already grounded post bd-34v. | None (already grounded). |
| bt-689s | Investigate events-based polling as optimization over MAX(updated_at) | GREEN | Investigation explicitly references native `events` / `wisp_events` tables and `GetAllEventsSince` — aligned with current Dolt-only primitives. | None. |
| bt-dtuv | Security: regex-validate database names in global Dolt UNION-ALL queries | GREEN | Defense-in-depth on `internal/datasource/global_dolt.go`; framing assumes shared Dolt server. | None. |
| bt-h5jz | Add first-class support for type=decision: schema, filter, dedicated view | GREEN | IssueType enum + filter + view work; storage-agnostic and consistent with upstream's unified data model. | None. |
| bt-ssk7 | Implement bt --global cross-database federation | GREEN | Audit confirms feature is implemented against shared Dolt server with UNION ALL; remaining scope is UX refinement. Framing matches Dolt-only reality. | None. |
| bt-thpq | Investigate Dolt changelog/history view in bt | GREEN | Investigation targets `dolt_log` / `dolt_diff_*` / `dolt_status` system tables — pure Dolt-era. | None. |
| bt-wjzk | Establish periodic schema-drift detection between bt and upstream beads | GREEN | Drift detector against upstream migrations vs `IssuesColumns`; storage-agnostic and forward-looking. | None. |
| bt-xpv9 | Security: honor BT_DOLT_USER and BT_DOLT_PASSWORD env vars | GREEN | Adds env-var escape hatch to Dolt DSN config; framing is Dolt-only. | None. |

## Bucket totals
- GREEN: 9
- YELLOW: 2
- RED: 0

## Notes / cross-bucket observations

- No RED in this bucket. The data-area beads are mostly post-Dolt or storage-agnostic, which tracks: this slice is heavy on security findings (bt-2gvf / bt-dtuv / bt-xpv9) authored 2026-04-27 against current code, plus landmarks (bt-05zt, bt-5hl9) already aligned with Dolt-only.
- bt-2cvx is the only meaningfully stale framing: it predates bd-34v's first-class session columns and still treats "Dolt columns vs metadata JSON" as an open decision. The work itself (cross-project provenance display) is still valid — bt-5hl9 already owns the hydration path, so bt-2cvx just needs a comment pointing the data side at bt-5hl9 and reducing its scope to the TUI/search surface.
- Three security beads (bt-2gvf, bt-dtuv, bt-xpv9) all share parent bt-6cdi and trace to the 2026-04-27 STRIDE/OWASP audit. They could be batched into a single PR sweep against `internal/datasource/` since they touch adjacent code (DSN wrapping, DB-name validation, env-var auth).
- bt-689s and bt-thpq are both Dolt-system-table investigations (events table vs dolt_log/dolt_diff). Worth coordinating — a single recon session could answer both, since the underlying questions overlap (per-DB vs shared, granularity, performance at scale).
- bt-ssk7's "current state (audit 2026-04-08)" notes already document that global mode works against the shared Dolt server; the bead is ripe for either re-scoping to its actual remaining UX work or splitting/closing once that work is itemized — orthogonal to this audit, flagging only.
