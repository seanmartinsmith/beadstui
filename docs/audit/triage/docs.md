# Triage: docs

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-95d1 | Document hook security model in README | GREEN | Storage-agnostic README docs about `.bt/hooks.yaml` execution; no data-layer assumptions. | None. |
| bt-ammc | Write user-facing docs for global mode and shared server migration | GREEN | Already framed around shared Dolt server + `bd init --shared-server`; consistent with Dolt-only reality. | None. |
| bt-iuqy | Review and adapt README draft for current state | GREEN | Doc-review work; storage-agnostic and explicitly calls for verifying claims against current shipped state. | None. |
| bt-ph1z.6 | DR: documentation + status indicator | GREEN | Explicitly built on "native Dolt commands" and queries Dolt system tables; Dolt-era framing. | None. |
| bt-qpck | research: prior-art review of perles - positioning, differentiation, cross-pollination | GREEN | Pure research/positioning bead; no data-layer assumptions. | None. |
| bt-uahv | Canonical .beads/ vs .bt/ data-home split: spec + apply | GREEN | Pre-classified landmark per rubric - IS the Dolt-era replacement for the data-home split. | None. |

## Bucket totals
- GREEN: 6
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations
- The docs bucket is uniformly clean. All six beads are either explicitly Dolt-framed (bt-ammc, bt-ph1z.6, bt-uahv) or storage-agnostic doc work (bt-95d1, bt-iuqy, bt-qpck).
- bt-qpck (perles prior-art review) feeds into bt-iuqy (README draft) - tight coupling worth tracking but not a triage concern.
- bt-ammc absorbed bt-bv3a per its own notes; consolidation already documented in-bead.
- No stale-assumption tells (no `.beads/<project>.jsonl`, no `sprints.jsonl`, no metadata-blob session column reads, no SQLite/JSONL-as-backend framing) appeared in any bead in this bucket.
