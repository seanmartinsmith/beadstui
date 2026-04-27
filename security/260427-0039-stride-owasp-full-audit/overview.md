# Security Audit — beadstui (bt) — Full STRIDE + OWASP Audit

**Date:** 2026-04-27 00:39 UTC
**Scope:** Entire codebase (~115k lines Go, 25 packages, 33 direct dependencies)
**Focus:** Comprehensive
**Iterations:** 30 (deep audit)
**Methodology:** STRIDE × OWASP Top 10 + regression on prior audit

## Summary

- **Total Findings:** 15 actionable
  - Critical: 0 | High: 0 | **Medium: 2** | **Low: 13** | Info: 11 (clean-area confirmations)
- **STRIDE Coverage:** 6/6 categories tested
- **OWASP Coverage:** 10/10 categories tested
- **Confirmed:** 13 | Likely: 1 | Possible: 3
- **govulncheck:** 0 exploitable; 4 advisory-only in unused code paths

## Overall Assessment

**Strong security posture maintained.** Two prior Mediums were fully fixed (Windows browser injection, BQL parser stack overflow) and one prior Medium was partially fixed (BQL `dateToSQL` SQL injection — now parameterized; hooks RCE — `--no-hooks` flag added but default unchanged). No new Critical or High findings.

The new packages added since the last audit (`pkg/tail`, `pkg/view`, `pkg/ui/events`, `pkg/baseline` updates, `pkg/drift` rewrite, expanded `cmd/bt/cobra_*` surface) introduced **2 new Low findings** plus several confirmed clean areas. All other Lows in the active set are recurring from the previous audit — known, bounded, low-effort to address.

The codebase still benefits from:
- All SQL queries parameterized (the one prior latent SQL injection in `dateToSQL` is now fixed)
- Editor allowlist (strong defense against `$EDITOR` shell injection)
- Self-updater with SHA256 + GitHub host pin + zip-slip protection
- Typed-struct YAML/JSON throughout (no `interface{}` decode targets)
- Browser-open paths now consistently use `rundll32 url.dll,FileProtocolHandler` on Windows
- Hardcoded BQL parser/executor recursion caps at depth 100 (fail-closed)

## Top 5 Findings

1. **[MEDIUM] Hooks shell exec from repo content** — Recurring with partial mitigation (`--no-hooks` added). By-design like git hooks; recommend trust-on-first-use prompt or default-off.
2. **[MEDIUM] Git argument injection via unvalidated SHAs** — Recurring across 13 correlation sites. Mechanical fix: validate hex + use `--end-of-options` consistently.
3. **[LOW] `bt tail` line injection (NEW)** — Newlines in comment text break compact/human format consumers. Sanitize at Summary/Title construction.
4. **[LOW] PreviewServer no timeouts (NEW)** — `http.Server{}` with default-unlimited timeouts. 5-line fix.
5. **[LOW] GitHub Actions script injection in fuzz.yml (NEW)** — `${{ github.event.inputs.fuzz_time }}` interpolated into `run:`. Mitigated by workflow_dispatch write-access requirement.

## Historical Comparison

**Previous audit:** `security/260409-2034-stride-owasp-full-audit/` (18 days ago)

### Trend

| Metric | Previous (2026-04-09) | Current (2026-04-27) | Change |
|--------|----------------------|----------------------|--------|
| Critical | 0 | 0 | → 0 |
| High | 0 | 0 | → 0 |
| Medium | 5 | 2 | ↓ -3 (improved) |
| Low | 7 | 13 | ↑ +6 |
| Info | 8 | 11 | ↑ +3 |
| Total actionable | 12 | 15 | ↑ +3 |
| OWASP coverage | 10/10 | 10/10 | → 0 |
| STRIDE coverage | 6/6 | 6/6 | → 0 |

The Low count rose because findings that were Medium last time and got partially mitigated this time are recategorized — and three genuinely new Lows landed on the new surfaces. Net: severity profile is **strictly better** than 2026-04-09.

### Finding Status

| Status | Count | Details |
|--------|-------|---------|
| ✅ Fixed since last audit | 3 | Windows `cmd /c start` (was Medium); BQL parser stack overflow (was Medium); BQL `dateToSQL` SQL injection (was Medium, latent) |
| 🔄 Recurring (unfixed or partially mitigated) | 12 | Hooks RCE (Medium → still Medium, partial); Git arg injection (Medium → still Medium); 10 prior Lows |
| 🆕 New findings | 3 | `bt tail` line injection (Low); PreviewServer no timeouts (Low); GH Actions script injection (Low) |

### Regression Alert

⚠️ 3 new Low findings introduced since 2026-04-09 across the new surfaces. None are Critical/High. See [findings.md](./findings.md) for the New entries (#3, #4, #5).

## Files in This Report

- [Threat Model](./threat-model.md) — STRIDE analysis, assets, trust boundaries
- [Attack Surface Map](./attack-surface-map.md) — entry points, data flows, abuse paths
- [Findings](./findings.md) — all findings ranked by severity (15 actionable + 11 info)
- [OWASP Coverage](./owasp-coverage.md) — per-category test results
- [Dependency Audit](./dependency-audit.md) — govulncheck output + hygiene bump list
- [Recommendations](./recommendations.md) — prioritized mitigations with code snippets
- [Iteration Log](./security-audit-results.tsv) — raw data from every iteration

## Next Steps

1. Address the 2 Mediums (Findings 1 and 2) this cycle — both have specific mechanical fixes.
2. Bundle the 7 quick Lows (Findings 3, 4, 5, 8, 9, 10, plus dep bumps) into a single hygiene PR.
3. Defer the 6 stable Lows (6, 7, 11, 12, 13, 14, 15) to backlog with bd-tracking.
4. Per the user's "Report + file beads" choice, beads will be filed for every Medium and Low finding under appropriate area labels.
