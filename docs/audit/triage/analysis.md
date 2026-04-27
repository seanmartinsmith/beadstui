# Triage: analysis

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-46p6.18 | Global-scope alert aggregation (deferred pending beads backend primitives) | GREEN | Explicitly defers on current backend; trigger conditions reference upstream federation primitives accurately. | None. |
| bt-4jyd | Global cross-project bead audit | GREEN | Uses `bd export` per project — storage-agnostic; works with Dolt-only backend unchanged. | None. |
| bt-53vw | Audit: evaluate satellite-node inclusion in per-project alert graphs | GREEN | Pure graph-analysis design audit; references current resolver plumbing (bt-mhwy.5) correctly. | None. |
| bt-7l5m | Alert scope computation: project-scoped only, no global aggregates | GREEN | Decision record explicitly grounded in upstream having "no first-class global/federated primitives". | None. |
| bt-gcuv | Priority hints computed from global issues, not filtered project | GREEN | UI bug about filter scope of recommendations; data-layer-agnostic. | None. |
| bt-j65a | Surface high-connectivity beads: recipe or sort for 'most blocking' / 'most deps' | GREEN | Surfacing existing graph metrics via BQL/recipe/sort — storage-agnostic. | None. |
| bt-ldq4 | robot session-stats: net beads-opened vs closed delta (global + per-project + TUI) | YELLOW | Data source quotes `Issue.Metadata["created_by_session"]` as bridge; stale post bd-34v Phase 1a (column is first-class now). | Append corrective comment: read `created_by_session` directly per bt-5hl9; drop "transparent swap later" framing since the swap is the work. |
| bt-mo70 | Notifications: density/centrality/count-delta signals — design spike | GREEN | Design-spike about UX signal:noise; storage-agnostic. | None. |
| bt-ph1z | Cross-project management gaps - user feedback from GH#3008 | GREEN | Epic over Dolt-aware children (AS OF temporal infra already landed via bt-ph1z.7); framing is current. | None. |
| bt-ph1z.1 | Portfolio health scoreboard | GREEN | "Builds on existing global Dolt mode data pipeline" — explicitly Dolt-aware. | None. |
| bt-ph1z.2 | Temporal trending: sparkline snapshots | GREEN | Uses Dolt `SELECT AS OF` directly; aligned with current architecture. | None. |
| bt-ph1z.3 | Temporal trending: diff mode | GREEN | Uses Dolt `AS OF` queries; aligned with current architecture. | None. |
| bt-ph1z.4 | Temporal trending: timeline view | GREEN | Capstone on bt-ph1z.7 temporal infra; Dolt-aware by inheritance. | None. |
| bt-ph1z.5 | Cross-project dependency graphs | GREEN | Parses `external:` dep syntax against shared Dolt server; aligned with Dolt-only reality. | None. |

## Bucket totals
- GREEN: 13
- YELLOW: 1
- RED: 0

## Notes / cross-bucket observations

- bt-ldq4 is the only bead in this bucket that quotes a now-stale data-access pattern (metadata-blob bridge for `created_by_session`). The bead anticipated the transparent swap — the swap is just done now. A short corrective comment pointing at bt-5hl9 / bd-34v Phase 1a is enough; the feature design is otherwise sound.
- The bt-ph1z.* family is a coherent Dolt-AS-OF temporal trending track and is uniformly aligned with current architecture. Worth keeping the family intact.
- bt-46p6.18, bt-7l5m, and bt-53vw form a tightly linked decision-and-audit cluster around alert scope. All three are correctly framed against the current "no upstream federation primitives" reality and are mutually consistent — no redundancy, no stale framing.
- bt-4jyd (cross-project audit) overlaps thematically with bt-mhcv (the systematic audit landmark). bt-4jyd is the broader portfolio-aware tool; bt-mhcv is the one-shot Dolt-migration audit. Different scopes, no merge needed, but worth noting they share the "cross-project view" muscle.
- No RED beads in this bucket — `analysis` work is largely graph-derived and storage-agnostic, so the Dolt migration didn't invalidate framing here.
