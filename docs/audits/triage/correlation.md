# Triage: correlation

| ID | Title (truncated to ~70 chars) | Class | Rationale (one line) | Suggested action |
|----|----|----|----|----|
| bt-08sh | bt correlator: migrate from .beads/<project>.jsonl + git-diff witness to dolt_log/dolt_history_issues | GREEN | Pre-classified landmark; IS the Dolt-era replacement for the JSONL+git-diff correlator. | None. |
| bt-3ltq | Global git history as a data layer: multi-repo correlation in global mode | GREEN | Pre-classified landmark; workspace-loader JSONL pin acknowledged in notes and gated behind ADR-003 (bt-05zt). | None. |
| bt-kc2t | Security: validate git SHAs and add --end-of-options at correlation call sites | GREEN | Argument-injection hardening on `exec.Command("git", ...)` call sites; storage-agnostic — applies regardless of whether SHAs come from JSONL or Dolt. | None. |
| bt-kqn4 | Security: regexp.QuoteMeta on git --grep pattern in correlation/explicit.go | GREEN | One-line `regexp.QuoteMeta` fix on a `git log --grep` call site; entirely storage-agnostic. | None. |
| bt-yqh0 | TUI + CLI feature: explicit cross-project bead aggregation by shared ID suffix | GREEN | Feature reuses pair.v2 detection over the loaded global bead set; no JSONL/SQLite/correlator-witness assumptions in framing. | None. |

## Bucket totals
- GREEN: 5
- YELLOW: 0
- RED: 0

## Notes / cross-bucket observations
- bt-08sh is the canonical Dolt-era replacement for the entire correlator stack; bt-kc2t and bt-kqn4 are security hardening on the existing call sites and remain valid both pre- and post-migration (the `exec.Command("git", ...)` pattern survives the witness change). After bt-08sh lands, the file paths in bt-kc2t's "13 sites" list will shift but the requirement (validate SHAs, use `--end-of-options`) carries forward — worth a comment on bt-kc2t once bt-08sh is in flight so the site list is re-verified post-migration.
- bt-3ltq explicitly calls out the workspace-loader JSONL pin (`pkg/workspace/loader.go:158-162` via `loader.FindJSONLPath`) in its notes and gates that work behind ADR-003 (bt-05zt). Framing is already grounded in current reality, matching its pre-classified GREEN landmark status.
- bt-yqh0 is technically a "correlation" bead by label but it's really a pair-detection/UX feature riding on `pkg/analysis/pairs.go`, not the git-correlation stack. Worth flagging that area:correlation is being used loosely here — could be re-labeled area:tui+area:analysis without architectural impact, but that's outside this audit's scope.
- No RED beads in this bucket, which is notable given the bucket-level warning that correlation is one of the highest-risk areas. The reason: the obvious RED candidate (the original JSONL+git-diff correlator framing) is already captured *as the replacement bead* (bt-08sh), and the remaining open beads are either security fixes on storage-agnostic call sites or features layered above the correlator that don't depend on its internal data source.
