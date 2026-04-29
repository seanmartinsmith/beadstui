# OWASP Top 10 Coverage - beadstui (bt)

**Date:** 2026-04-09
**Iterations:** 30

## Coverage Matrix

| ID | Category | Tested | Findings | Status |
|----|----------|--------|----------|--------|
| A01 | Broken Access Control | Yes | 0 | Clean - no multi-user access model; editor allowlist solid |
| A02 | Cryptographic Failures | Yes | 0 | Clean - SHA256 for updates; no weak crypto in production code; token scoping correct |
| A03 | Injection | Yes | 6 | Issues found (2 Medium, 4 Low) |
| A04 | Insecure Design | Yes | 4 | Issues found (1 Medium, 3 Low) |
| A05 | Security Misconfiguration | Yes | 3 | Issues found (1 Medium, 2 Low) |
| A06 | Vulnerable and Outdated Components | Yes | 2 | Info only (CVEs not exploitable in bt) |
| A07 | Identification and Authentication Failures | Yes | 1 | Low (hardcoded root, acceptable for local Dolt) |
| A08 | Software and Data Integrity Failures | Yes | 1 | Low (missing io.LimitReader, mitigated by checksum) |
| A09 | Security Logging and Monitoring Failures | Yes | 0 | Acceptable - CLI/TUI app with git-based audit trail |
| A10 | Server-Side Request Forgery | Yes | 0 | Clean - no user-controlled URL fetching |

**Coverage: 10/10 categories tested**

## Per-Category Details

### A01 - Broken Access Control
- [x] IDOR on parameterized routes - N/A (no HTTP server, no multi-user model)
- [x] Missing authorization middleware - N/A (local-only application)
- [x] Horizontal privilege escalation - N/A (single-user)
- [x] Vertical privilege escalation - Editor allowlist prevents shell escalation
- [x] Directory traversal on file operations - All paths from well-known anchors
- [x] CORS misconfiguration - N/A (no HTTP server)
- [x] Missing function-level access control - N/A

### A02 - Cryptographic Failures
- [x] Sensitive data in plaintext - No passwords stored; Dolt uses no auth by design
- [x] Weak hashing algorithms - Only SHA256 in production code (updater checksum)
- [x] Hardcoded secrets/API keys - None found; tokens from env vars only
- [x] Missing encryption at rest/in transit - Dolt uses localhost TCP; GitHub uses HTTPS
- [x] Weak random number generation - No security-sensitive random in production code
- [x] Exposed config with secrets - No secrets in config files

### A03 - Injection
- [x] SQL injection - **dateToSQL unparameterized (LATENT)**, fieldToColumn fallback, global Dolt db names
- [x] Command injection - **Windows `cmd /c start` URL injection**, hook commands by design
- [x] XSS - N/A (terminal application, no HTML rendering to users)
- [x] Template injection - N/A
- [x] Path injection - Safe (all paths from anchors)
- [x] Header injection - N/A (no HTTP server)
- [x] Argument injection - **Git SHA/grep injection from bead data**

### A04 - Insecure Design
- [x] Missing rate limiting - N/A (local application)
- [x] No account lockout - N/A
- [x] Race conditions - **Windows lock TOCTOU** (narrow), background worker well-synchronized
- [x] Missing CSRF protection - N/A
- [x] Resource exhaustion - **BQL recursion DoS**, unbounded IN list
- [x] Predictable identifiers - N/A

### A05 - Security Misconfiguration
- [x] Debug mode - Gated behind BT_DEBUG env var
- [x] Default credentials - **Dolt hardcoded root** (by design for local)
- [x] Verbose error messages - **DSN in error messages**
- [x] Missing security headers - N/A (no HTTP server)
- [x] Stack traces in errors - Truncated by shortError() before display
- [x] **Hook execution from cloned repos** - by-design, same as git hooks

### A06 - Vulnerable and Outdated Components
- [x] Known CVEs - **edwards25519 v1.1.0** (not called), **x/image v0.35.0** (TIFF not used)
- [x] Outdated frameworks - Charm v1 (v2 available, migration tracked in ADR)
- [x] Unmaintained dependencies - **toon-go** nascent but low-use
- [x] govulncheck - 0 symbol-level vulnerabilities

### A07 - Identification and Authentication Failures
- [x] Weak password policies - N/A
- [x] Session management - N/A
- [x] JWT vulnerabilities - N/A
- [x] Authentication - **Dolt uses root/no-password** (standard for local servers)

### A08 - Software and Data Integrity Failures
- [x] CI/CD pipeline integrity - goreleaser with token-based publishing
- [x] Unsigned dependencies - Go module checksums (go.sum) provide integrity
- [x] Insecure deserialization - All unmarshalling into typed structs
- [x] Update integrity - **SHA256 verified**, auth-header stripped on redirects
- [x] Archive extraction - **Missing io.LimitReader** but checksum + size check mitigate

### A09 - Security Logging and Monitoring Failures
- [x] Audit logs - Git history provides indirect audit trail for data changes
- [x] Failed auth logging - N/A (no authentication)
- [x] Sensitive data in logs - Worker trace logs may contain error details; no secrets
- [x] Log injection - slog structured logging prevents injection

### A10 - Server-Side Request Forgery
- [x] Unvalidated URLs - Only updater makes HTTP requests to hardcoded GitHub URL
- [x] DNS rebinding - N/A
- [x] Missing allowlist - GitHub URL is hardcoded (baseURL constant)
- [x] Proxy/redirect - Redirect-aware auth stripping prevents SSRF-adjacent token leakage
