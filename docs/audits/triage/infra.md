# Triage: infra

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-2q7b | Audit disabled CI workflows: fuzz.yml + flake-update.yml | GREEN | CI/release-engineering scope; storage-agnostic. | None. |
| bt-46p6.15 | Notifications tab: compact/dense row rendering (v1 follow-up) | GREEN | TUI density polish; no data-layer assumptions. | None. |
| bt-46p6.19 | TUI cross-project navigation from alert details | GREEN | TUI nav feature; storage-agnostic. | None. |
| bt-5glp | Research: inter-session messaging for Claude Code | GREEN | Research spike on messaging; not data-layer coupled. | None. |
| bt-6cdi | Security audit follow-up: 2026-04-27 STRIDE+OWASP findings | GREEN | Security epic referencing current 2026-04-27 audit; current. | None. |
| bt-72l8 | Audit: comprehensive sweep for Jeffrey-era leftovers | GREEN | Repo/filesystem hygiene sweep; storage-agnostic. | None. |
| bt-8f34 | Project registry: surface what each repo is for cross-project agents | GREEN | Cross-project registry; framing already references shared Dolt server. | None. |
| bt-8lz1 | Workspace stack vision: bt + tpane + cnvs as cohesive workspace | GREEN | Brainstorm-level vision bead; no data-layer coupling. | None. |
| bt-8qd1 | Research: evaluate Gas Town / Gas City / Wasteland adoption | GREEN | Adoption research spike; orthogonal to bt's data layer. | None. |
| bt-eg2c | Security: add io.LimitReader to self-update tar extraction | GREEN | Self-update hardening in pkg/updater; no beads-storage coupling. | None. |
| bt-ghbl | Cross-project beads: no raw SQL against shared server | GREEN | Convention/guardrail consistent with current Dolt-only shared server. | None. |
| bt-jov1 | Beads upstream sync: daily hook for bd/schema changes | GREEN | Compatibility monitoring system; explicitly aware of Dolt schema as contract. | None. |
| bt-k9mp | Cross-project bead filing: agents file beads where they belong | GREEN | Cross-project filing convention; storage-agnostic. | None. |
| bt-m8fo | Security: per-project consent for .bt/hooks.yaml execution | GREEN | Hooks consent model in pkg/hooks; storage-agnostic. | None. |
| bt-t82t | Phase 4: Stale refs, golden files, test validation | GREEN | Refactor cleanup pass; bv- ref hygiene only. | None. |
| bt-tma8 | Session close: flag formatter-induced diffs outside current task scope | GREEN | Wrapup/session-close ergonomics; storage-agnostic. | None. |
| bt-ushd | [epic] Cross-project beads operating system | GREEN | Epic explicitly framed around shared Dolt server; current. | None. |
| bt-v0mq | Auto-export git add silently fails — issues.jsonl tracked but ignored | YELLOW | Bug is real (gitignore vs tracked file mismatch) but framing centers on `.beads/issues.jsonl` as if it's the system of record; in Dolt-only era this file is opt-in JSONL export, not authoritative. | Append corrective comment: clarify that JSONL is opt-in export, then decide whether bt should auto-export at all (bt-uahv adjacent) before fixing the git add path. |
| bt-x2ap | Security: replace direct expression interpolation in fuzz.yml | GREEN | GitHub Actions hardening; storage-agnostic. | None. |
| bt-zgzq | Re-enable brew tap + scoop bucket publishing post-v1 | GREEN | Release-channel deferred work; storage-agnostic. | None. |

## Bucket totals
- GREEN: 19
- YELLOW: 1
- RED: 0

## Notes / cross-bucket observations

- bt-v0mq is the only stale-framing finding. The underlying bug (gitignore vs tracked-file ambiguity producing `auto-export: git add failed` warnings) is real, but the bead implicitly treats `.beads/issues.jsonl` as canonical. Post-Dolt-only, that file is an opt-in portability artifact, not the system of record — the right question is whether bt should be auto-exporting JSONL at all, which overlaps bt-uahv (`.beads/` vs `.bt/` canonical split). Worth grounding before someone fixes only the symptom.
- bt-ushd, bt-ghbl, bt-k9mp, bt-8f34 form a coherent cross-project epic family and all assume the shared Dolt server model — already aligned with current reality.
- bt-6cdi (and its children bt-eg2c, bt-m8fo, bt-x2ap) are fresh from the 2026-04-27 security audit; framing is current.
- bt-72l8 explicitly notes a Section 8 ("Data layer (Dolt)") for scanning bead records — that section is already grounded in the Dolt-only world.
- bt-t82t mentions cleaning up `bv-` references — purely string hygiene from the rename, no architectural assumption.
