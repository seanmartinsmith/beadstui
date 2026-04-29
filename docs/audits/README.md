# Audits

State assessments at a point in time. Each file here is a snapshot — what was true when the audit ran, not what's true now. Audits are written once and rarely revised; if the data has shifted, run a new audit rather than editing an old one.

## Layout

| Folder | What goes here |
|---|---|
| `inbox/` | Holding area for new audits awaiting categorization. Drain to a categorized folder when ≥3 items accumulate or before session handoff. |
| `architecture/` | State snapshots and surface maps. "What does X look like right now" — schema drift, dependency graphs, data-source surveys. |
| `domain/` | Domain-focused audits — CLI ergonomics, keybindings, test suite, etc. Scoped to one area of the project. |
| `gaps/` | Gap analyses and readiness assessments. "What's missing for X to ship" — BQL gaps, global-mode readiness, synthesis docs. |
| `security/` | Security audits. Each is its own dated subdirectory with overview / findings / threat-model / etc. |
| `triage/` | Per-area bead triage from cross-cutting audit efforts. Currently a single 14-file effort; recurring-style folder. |
| `screenshots/` | Image assets referenced by audit findings. |
| `<YYYY-MM-DD>-<bead-or-slug>-<descriptor>/` | Multi-file cluster-investigation snapshots (e.g., `2026-04-27-bt-cluster-reorg/`). One folder per cluster effort. |

## Conventions

- **Filename date prefix.** `YYYY-MM-DD-<slug>.md`. The date is when the audit ran, not when it was filed. Files inside dated cluster folders skip the prefix (the folder carries the date).
- **One audit, one file.** If an audit has multiple coherent sections, file as a multi-file cluster folder, not one giant file.
- **Don't edit old audits.** Period accuracy is the value. If reality has shifted, run a new audit at today's date and reference the prior one.
- **Cluster folders are dated.** Pattern: `<YYYY-MM-DD>-<bead-or-slug>-<descriptor>/`. Examples: `2026-04-27-bt-cluster-reorg/`, `2026-04-29-bt-72l8-bv-era-cleanup/`. The first segment is always a date.
- **Inbox discipline.** New audits land in `inbox/` if their bucket isn't obvious; sort with the user when 3+ items accumulate.
- **AGENTS.md applies as a fallback.** When in doubt, see the *Docs Structure Conventions* table in `AGENTS.md` at repo root.
