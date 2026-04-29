# Triage: cli

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-113x | robot refs: --explain-refs observability mode | GREEN | Detector observability flag; storage-agnostic, layered on shipped v2 refs. | None. |
| bt-3qfa | bt robot schema: per-subcommand input flag manifest for agents | GREEN | Pure CLI introspection feature; no data-layer assumptions. | None. |
| bt-70cd | Unknown 'bt robot' subcommand prints help to stdout, exits 0 | GREEN | Cobra contract bug; storage-agnostic. | None. |
| bt-7712 | Security: strip newlines from event Summary/Title in bt tail formats | GREEN | Output-sanitization bug in tail formatters; data-layer-agnostic. | None. |
| bt-82w8 | bt robot comments <id> --global: standalone comment retrieval | GREEN | New robot subcommand; depends on existing --global routing, no JSONL assumption. | None. |
| bt-92ic | bt robot pairs --orphaned: enrich members with title/status/source_repo | GREEN | Enriches existing v1 pairs output shape; storage-agnostic. | None. |
| bt-94a7 | Broad upstream audit: what bt should expose beyond bt-uc6k | GREEN | Audit task scoped against current upstream (Dolt) schema/CLI surface. | None. |
| bt-9prn | Cross-project blocker detection: suffix match w/ asymmetric evidence | GREEN | Reader-only heuristic over issue corpus; no stale storage assumption. | None. |
| bt-ah53 | Robot mode I/O contract: documented invariants + verify-test sweep | GREEN | Contract documentation + test sweep; storage-agnostic. | None. |
| bt-br02 | Security: stop discarding errors at 3 known sites | GREEN | Error-handling hygiene at named call sites; data-layer-agnostic. | None. |
| bt-dhqw | bt robot pair show <suffix>: drill-in subcommand for pair triage | GREEN | Reader subcommand over issue corpus via analysis.SplitID; no JSONL assumption. | None. |
| bt-hq1a | bt doctor: health-check subcommand for bt's integration points | GREEN | Already scoped against Dolt server reachability + .beads/.bt split (delegates schema to bd). | None. |
| bt-lt2h | Human-readable CLI output mode (non-TUI, non-robot) | GREEN | UX surface decision; storage-agnostic, references current --global path. | None. |
| bt-mxz9 | Cold boot from non-workspace dir fails: bd dolt start workspace | GREEN | Pre-classified landmark (retrospective audit pattern bead). | None. |
| bt-s6xg | bt robot list --global silently truncates at --limit=100 default | GREEN | CLI default-flag bug; works against current Dolt corpus. | None. |
| bt-tq60 | Root --diff-since auto-JSON leaks log output to stderr | GREEN | Robot-mode log-suppression routing bug; storage-agnostic. | None. |
| bt-we7z | bt robot list --ids <csv>: direct ID filter | GREEN | New filter flag on existing list path; storage-agnostic. | None. |
| bt-x685 | Root --diff-since auto-JSON leaks log output to stderr | YELLOW | Duplicate of bt-tq60 — same bug, same proposed fix, same skipped test reference. | Append corrective comment noting duplication; close-as-superseded by bt-tq60. |
| bt-xgba | Remove --schema=v1 fallback on robot pairs + refs | GREEN | Cleanup bead deleting transitional surface; storage-agnostic. | None. |
| bt-z5jj | bt sprint feature: rebuild against Dolt or retire | GREEN | Pre-classified landmark; IS the Dolt-era replacement decision bead. | None. |
| bt-zq9k | Security: wrap runTUIProgram with defer recover to reset terminal | GREEN | TUI panic-recovery hardening; data-layer-agnostic. | None. |
| bt-zr9n | Improve startup info output: format, usefulness, clarity | GREEN | Cosmetic/UX of startup messages; storage-agnostic. | None. |

## Bucket totals
- GREEN: 21
- YELLOW: 1
- RED: 0

## Notes / cross-bucket observations

- **Duplicate detected**: `bt-x685` and `bt-tq60` describe the exact same bug (root-level `--diff-since` auto-JSON leaks the global-mode-discovery INFO line to stderr, both reference the same skipped test `TestDiffSinceAutoJSON_MalformedIssues_NoStderr` in `tests/e2e/robot_diff_test.go`, both reference bt-0cht and bt-rzuf, both propose the same fix in `cmd/bt/root.go` RunE). bt-tq60 is the longer/more detailed write-up; bt-x685 looks like a re-file. Not a Dolt-architecture issue — flagging as YELLOW for duplication housekeeping rather than stale-architecture grounds. Recommend close-as-superseded on bt-x685 with pointer to bt-tq60.
- **bt-z5jj**: pre-classified landmark; the bead itself IS the Dolt-era decision capture (Option A retire / B metadata-column / C bt-canonical-sprints). Description correctly identifies the stale `LoadSprints` call as the symptom. Already grounded.
- **bt-mxz9**: pre-classified landmark. Description has been updated with bd-mxz9 grounding (correctly notes `BEADS_DOLT_SHARED_SERVER` was always mode-selection, never workspace-bypass; correctly identifies `~/.beads/registry.json` as a deliberately-ignored dead artifact from the removed daemon subsystem). No further architectural correction needed.
- **bt-94a7**: scoped explicitly against current Dolt-only upstream — Tier-1 covers schema columns, IssueType enum, `bd` slash-command surface. Explicitly references that bt-uc6k closed and bt-jov1 supersedes for monitoring. Clean Dolt-era framing.
- **bt-hq1a**: doctor checks include "Dolt server running + reachable", "Dolt schema version matches", "delegates Dolt/schema/database checks to bd". Properly grounded in current architecture.
- **Cluster theme — robot-mode contract slipped**: bt-70cd, bt-ah53, bt-tq60/x685 form a coherent cluster around stdout/stderr/exit invariants under robot mode. The bt-uh3c brainstorm spawned several of these on the same day (2026-04-25). They reinforce each other; bt-ah53 is the meta-fix that would catch the others.
- **Cluster theme — pairs/refs v2 ecosystem**: bt-92ic, bt-dhqw, bt-9prn, bt-xgba all extend or clean up the pairs/refs v2 work. All scoped against the current shared-server corpus (~2770 issues). No JSONL/SQLite assumptions.
- **Security cluster (2026-04-27 STRIDE/OWASP audit)**: bt-7712, bt-br02, bt-zq9k all reference the same audit findings.md file and target specific call sites. Storage-layer-irrelevant, classification-clean.
