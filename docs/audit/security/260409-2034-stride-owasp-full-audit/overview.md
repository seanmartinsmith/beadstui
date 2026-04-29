# Security Audit - beadstui (bt) - Full STRIDE + OWASP Audit

**Date:** 2026-04-09 20:34 UTC
**Scope:** Entire codebase (~92k lines Go, 28 packages, 33 direct dependencies)
**Focus:** Comprehensive
**Iterations:** 30 (deep audit)

## Summary

- **Total Findings:** 20
  - Critical: 0 | High: 0 | Medium: 5 | Low: 7 | Info: 8
- **STRIDE Coverage:** 6/6 categories tested
- **OWASP Coverage:** 10/10 categories tested
- **Confirmed:** 17 | Likely: 1 | Possible: 2
- **govulncheck:** 0 exploitable vulnerabilities (2 advisory-only in unused code paths)

## Overall Assessment

**The codebase has a strong security posture for a local CLI/TUI application.** No critical or high-severity vulnerabilities were found. The 5 medium findings are all bounded in impact - most require either local file access, Windows-specific conditions, or activation of currently-dead code paths.

Key strengths:
- All SQL queries properly parameterized (except one latent dead-code path)
- YAML/JSON parsing uses typed structs throughout (no `interface{}` targets)
- Editor launch uses strict allowlist blocking shells and interpreters
- Self-updater has SHA256 verification with redirect-aware auth stripping
- Concurrency model is well-designed with proper synchronization
- GitHub token scoped to GitHub domains only

## Top 5 Findings

1. **[Windows `cmd /c start` URL injection](./findings.md#medium-finding-1-windows-command-injection-via-cmd-c-start)** - Shell metacharacters in URLs can execute arbitrary commands on Windows. Fix: use `rundll32` pattern already present in `model_export.go`.

2. **[Hook config arbitrary execution](./findings.md#medium-finding-2-arbitrary-command-execution-via-hook-config-files)** - `.bt/hooks.yaml` in cloned repos executes commands without confirmation. By-design (like git hooks) but worth documenting and adding `--no-hooks`.

3. **[BQL parser stack overflow](./findings.md#medium-finding-3-bql-parser-stack-overflow-via-recursion)** - No recursion depth limit. Deeply nested `--bql` CLI input causes DoS. Fix: add depth counter (5 lines).

4. **[BQL dateToSQL unparameterized](./findings.md#medium-finding-4-sql-injection-in-bql-datetosql-latent)** - Latent SQL injection in dead code. Must be fixed before DoltExecutor activation.

5. **[Git SHA argument injection](./findings.md#medium-finding-5-git-argument-injection-via-unvalidated-shas)** - Bead data SHAs flow to git commands without hex validation. Fix: regex check + `--end-of-options`.

## Files in This Report

- [Threat Model](./threat-model.md) - STRIDE analysis, assets, trust boundaries
- [Attack Surface Map](./attack-surface-map.md) - entry points, data flows, abuse paths
- [Findings](./findings.md) - all 20 findings ranked by severity with code evidence
- [OWASP Coverage](./owasp-coverage.md) - per-category test results (10/10 covered)
- [Dependency Audit](./dependency-audit.md) - govulncheck results + 33 dependency review
- [Recommendations](./recommendations.md) - prioritized mitigations with code snippets
- [Iteration Log](./security-audit-results.tsv) - raw data from all 30 iterations

## Coverage

```
=== Security Audit Coverage (iteration 30) ===
STRIDE Coverage: S[x] T[x] R[x] I[x] D[x] E[x] - 6/6
OWASP Coverage:  A01[x] A02[x] A03[x] A04[x] A05[x] A06[x] A07[x] A08[x] A09[x] A10[x] - 10/10
Findings: 0 Critical, 0 High, 5 Medium, 7 Low, 8 Info
Confirmed: 17 | Likely: 1 | Possible: 2
Metric: (10/10)*50 + (6/6)*30 + min(20,20) = 50 + 30 + 20 = 100/100
```

## Findings by Severity

| # | Severity | Finding | OWASP | STRIDE | Location |
|---|----------|---------|-------|--------|----------|
| 1 | Medium | Windows cmd /c start URL injection | A03 | EoP | pkg/export/github.go:610, cloudflare.go:347 |
| 2 | Medium | Hook command execution from config | A05 | EoP | pkg/hooks/executor.go:121 |
| 3 | Medium | BQL parser recursion DoS | A04 | DoS | pkg/bql/parser.go:125-153 |
| 4 | Medium | BQL dateToSQL no parameterization (latent) | A03 | Tampering | pkg/bql/sql.go:219-229 |
| 5 | Medium | Git SHA args not validated as hex | A03 | Tampering | pkg/correlation/ (7 files) |
| 6 | Low | BQL fieldToColumn fallback (latent) | A03 | Tampering | pkg/bql/sql.go:186 |
| 7 | Low | Global Dolt DB name interpolation | A03 | Tampering | internal/datasource/global_dolt.go:229 |
| 8 | Low | BQL unbounded IN list | A04 | DoS | pkg/bql/parser.go:216 |
| 9 | Low | Updater missing io.LimitReader | A08 | DoS | pkg/updater/updater.go:548 |
| 10 | Low | DSN in error messages | A05 | InfoDisc | internal/datasource/load.go:159 |
| 11 | Low | Git --grep bead ID not escaped | A03 | Tampering | pkg/correlation/explicit.go:231 |
| 12 | Low | Swallowed critical errors | A05 | Repudiation | cmd/bt/main.go:494 + 2 more |
| 13 | Low | Windows lock TOCTOU | A04 | DoS | pkg/instance/lock_windows.go:218 |
| 14 | Low | Dolt hardcoded root user | A07 | Spoofing | internal/datasource/metadata.go:63 |
| 15 | Low | TUI unrecovered main thread panics | A04 | DoS | pkg/ui/ |
| 16 | Info | edwards25519 CVE (not exploitable) | A06 | - | go.mod |
| 17 | Info | x/image CVE (not exploitable) | A06 | - | go.mod |
| 18 | Info | Editor allowlist (well-mitigated) | A01 | - | pkg/ui/model_editor.go |
| 19 | Info | Self-updater integrity (solid) | A08 | - | pkg/updater/updater.go |
| 20 | Info | Clean areas (YAML, JSON, clipboard, etc.) | - | - | Multiple |
