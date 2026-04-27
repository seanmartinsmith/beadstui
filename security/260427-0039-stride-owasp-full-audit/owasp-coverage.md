# OWASP Top 10 Coverage Matrix

| ID | Category | Tested | Findings | Status |
|----|----------|--------|----------|--------|
| A01 | Broken Access Control | ✓ | 0 | ✅ Clean — no auth surface in bt; hooks privilege threat captured under A05/EoP |
| A02 | Cryptographic Failures | ✓ | 0 | ✅ Clean — no crypto in bt; updater SHA256 verified; no secrets stored locally |
| A03 | Injection | ✓ | 5 | ⚠️ Issues — 1 Medium (git arg), 4 Low (BQL field/db name, --grep, Actions input) |
| A04 | Insecure Design | ✓ | 5 | ⚠️ Issues — BQL IN unbounded, preview timeouts, lock TOCTOU, TUI panic, parser depth (fixed) |
| A05 | Security Misconfiguration | ✓ | 4 | ⚠️ Issues — hooks RCE (Medium), DSN leak, error swallow, Dolt bind trust |
| A06 | Vulnerable Components | ✓ | 2 | ℹ️ Advisory — edwards25519 + x/image CVEs in unused paths (govulncheck confirms 0 exploitable) |
| A07 | Authentication Failures | ✓ | 1 | ⚠️ Low — Dolt hardcoded root user |
| A08 | Software/Data Integrity | ✓ | 1 | ⚠️ Low — self-updater missing io.LimitReader |
| A09 | Logging/Monitoring | ✓ | 1 | ⚠️ Low — bt tail compact/human format newline injection |
| A10 | SSRF | ✓ | 0 | ✅ Clean — no SSRF surface; only outbound is GitHub host-pinned + Cloudflare meta endpoint |

## STRIDE Coverage

| STRIDE | Tested | Findings | Notes |
|--------|--------|----------|-------|
| Spoofing | ✓ | 1 | Dolt root user; Dolt bind trust info-only |
| Tampering | ✓ | 5 | Git arg injection, BQL fields/DB names, --grep, Actions input, tail format |
| Repudiation | ✓ | 1 | Swallowed errors |
| Information Disclosure | ✓ | 1 | DSN error wrapping |
| Denial of Service | ✓ | 5 | Preview timeouts, BQL IN, tar limit, lock race, TUI panic |
| Elevation of Privilege | ✓ | 1 | Hooks RCE (by-design, partial mitigation via --no-hooks) |

## Per-Category Detail

### A01 — Broken Access Control
Not applicable to bt directly: no multi-user model, no protected routes, no roles. Threats in this category bind to the `bd` CLI (out of scope) and the GitHub/Cloudflare delegated tokens (managed by their respective CLIs, not by bt).

### A03 — Injection (5 findings)
- **Finding 2** [Medium]: Git arg injection via unvalidated SHAs across 13 correlation sites
- **Finding 5** [Low]: GitHub Actions script injection in fuzz.yml
- **Finding 6** [Low, latent]: BQL fieldToColumn concatenation
- **Finding 7** [Low]: Global Dolt DB-name interpolation
- **Finding 11** [Low]: git --grep regex not escaped

Inversely, BQL `dateToSQL` (prior Finding 4) is now parameterized — fixed.

### A04 — Insecure Design (5 findings)
- **Finding 4** [Low]: PreviewServer no timeouts
- **Finding 8** [Low]: BQL IN unbounded
- **Finding 13** [Low, possible]: Windows lock TOCTOU
- **Finding 15** [Low, possible]: TUI main thread no panic recovery
- (Fixed) BQL parser stack overflow now bounded at depth 100

### A05 — Security Misconfiguration (4 findings)
- **Finding 1** [Medium, partial mitigation]: Hooks shell exec
- **Finding 10** [Low]: DSN error wrapping
- **Finding 12** [Low]: Swallowed errors at 3 sites
- **Finding (info)**: Dolt bind trust on user filesystem (informational)

### A06 — Vulnerable Components
`govulncheck` reports 0 exploitable vulnerabilities. 4 advisory matches are all in code paths bt doesn't call:
- `filippo.io/edwards25519@v1.1.0` GO-2026-4503 (`MultiScalarMult`) — bt's chain only uses `ScalarBaseMult` via `go-sql-driver/mysql`
- `golang.org/x/image@v0.35.0` GO-2026-4815 (TIFF), GO-2026-4961 (WEBP 32-bit), GO-2026-4962 (SFNT) — bt only imports `font/basicfont`, not decoders

Recommended hygiene bumps:
- `golang.org/x/image v0.35.0 → v0.39.0`
- `filippo.io/edwards25519 v1.1.0 → v1.1.1`

### A09 — Logging and Monitoring Failures
- **Finding 3** [Low]: `bt tail` compact/human formats let newlines from issue Title/Comment text break line-oriented output. Affects downstream pipelines that parse `bt tail` output line-by-line. JSON formats are immune.

### A10 — Server-Side Request Forgery
Not applicable. bt has no inbound API. The only outbound calls are:
- `pkg/updater` → `api.github.com` and `objects.githubusercontent.com` (host-pinned, auth-stripped on cross-host redirects)
- `pkg/export/cloudflare.go:431` → Cloudflare meta endpoint (curl shell-out, 10s timeout, no user-controlled URL component)

Neither path lets an attacker influence the destination.
