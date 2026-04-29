# Recommendations (Priority Order)

## Priority 1 — Medium (address this cycle)

### 1. Validate git SHAs at all correlation call sites + use `--end-of-options`
**Finding:** [Finding 2](./findings.md#medium-finding-2-git-argument-injection-via-unvalidated-shas-)
**Effort:** ~1 hour (~15 sites, 1 helper, mechanical pass)
**Files:** `pkg/correlation/{incremental,cocommit,reverse,temporal,stream,explicit,cache}.go`

```go
// pkg/correlation/sha.go (new)
package correlation

import (
    "fmt"
    "regexp"
)

var validSHAPattern = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)

// ValidateSHA returns an error if s is not a hex string of 4-40 chars.
// Apply to every SHA value before passing to git argv.
func ValidateSHA(s string) error {
    if !validSHAPattern.MatchString(s) {
        return fmt.Errorf("invalid git SHA: %q", s)
    }
    return nil
}
```

At each call site, validate before invoking git, and place `--end-of-options` immediately before any positional commit ref:

```go
// Before:
cmd := exec.Command("git", "rev-list", "--reverse", fmt.Sprintf("%s..HEAD", sinceSHA))

// After:
if err := ValidateSHA(sinceSHA); err != nil {
    return nil, err
}
cmd := exec.Command("git", "rev-list", "--reverse", "--end-of-options", fmt.Sprintf("%s..HEAD", sinceSHA))
```

### 2. Add per-project consent for `.bt/hooks.yaml` (or invert the default)
**Finding:** [Finding 1](./findings.md#medium-finding-1-arbitrary-command-execution-via-hook-config-files-)
**Effort:** ~3 hours (consent prompt UX + persistence file format + test)
**Files:** `pkg/hooks/executor.go`, new `pkg/hooks/trust.go`, `cmd/bt/root.go`

Two routes — recommend the lighter one first since the heavier one is a UX shift:

**Option A (lighter — invert default):** Make hooks opt-in. Change `flagNoHooks` to `flagHooks` defaulting to false. Users explicitly pass `--hooks` to enable. Documented in commit + release notes.

**Option B (preferred — trust-on-first-use):** First time bt detects `.bt/hooks.yaml` in a project, prompt:
```
This repo defines hooks in .bt/hooks.yaml. They will run shell commands when bt
exports. Allow for this project? [y/N]
```
Persist `<project-hash> -> SHA256(hooks.yaml content)` in `~/.bt/hooks-trust.json`. Re-prompt when content hash changes.

Either way, document the threat model in `pkg/hooks/README.md`.

---

## Priority 2 — Low (address next cycle)

### 3. Strip newlines from event Summary/Title in tail output
**Finding:** [Finding 3](./findings.md#low-finding-3-stream-format-line-injection-in-bt-tail-compactrhuman-modes-)
**Effort:** ~10 min
**File:** `pkg/ui/events/diff.go`

```go
func sanitizeForLine(s string) string {
    return strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
}

// In newCommentedEvent / Diff Title-construction sites:
summary = sanitizeForLine(truncateForSummary(latest.Text, commentSummaryLimit))
title := sanitizeForLine(newIssue.Title)
```

### 4. Add timeouts to PreviewServer
**Finding:** [Finding 4](./findings.md#low-finding-4-previewserver-http-server-lacks-timeouts-)
**Effort:** ~5 min
**File:** `pkg/export/preview.go:59-62`

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

### 5. Fix GitHub Actions script injection in fuzz.yml
**Finding:** [Finding 5](./findings.md#low-finding-5-github-actions-script-injection-in-fuzzyml-)
**Effort:** ~10 min
**File:** `.github/workflows/fuzz.yml`

Replace every direct interpolation with env-var indirection (the pattern release-notes.yml already uses):

```yaml
- name: Determine Fuzz Time
  id: fuzz-config
  env:
    INPUT_FUZZ_TIME: ${{ github.event.inputs.fuzz_time }}
    EVENT_NAME: ${{ github.event_name }}
  run: |
    if [ "$EVENT_NAME" = "workflow_dispatch" ]; then
      echo "fuzz_time=${INPUT_FUZZ_TIME}" >> "$GITHUB_OUTPUT"
    else
      echo "fuzz_time=10m" >> "$GITHUB_OUTPUT"
    fi
```

Apply to every `run:` block that references `steps.fuzz-config.outputs.fuzz_time`.

### 6. Add io.LimitReader to self-update tar extraction
**Finding:** [Finding 9](./findings.md#low-finding-9-self-updater-missing-iolimitreader-)
**Effort:** ~5 min
**File:** `pkg/updater/updater.go:548`

```go
const maxBinarySize = 200 * 1024 * 1024 // 200MB
if _, err := io.Copy(out, io.LimitReader(tr, maxBinarySize)); err != nil {
    return fmt.Errorf("failed to extract binary: %w", err)
}
```

### 7. Cap BQL IN-list at 1000 values
**Finding:** [Finding 8](./findings.md#low-finding-8-bql-unbounded-in-value-list-)
**Effort:** ~5 min
**File:** `pkg/bql/parser.go`

```go
const maxInListValues = 1000

// In parseInExpr:
if len(values) >= maxInListValues {
    return nil, fmt.Errorf("IN list exceeds %d values", maxInListValues)
}
```

### 8. Wrap Dolt error with topology-free message
**Finding:** [Finding 10](./findings.md#low-finding-10-dsn-exposed-in-error-messages-)
**Effort:** ~5 min
**File:** `internal/datasource/load.go:159`, `dolt.go:40`

```go
// Drop source.Path from the wrapper:
return nil, fmt.Errorf("failed to open Dolt source: %w", err)
```

### 9. Hygiene dependency bumps
**Finding:** [dep advisories](./dependency-audit.md)
**Effort:** ~5 min
```bash
go get golang.org/x/image@latest
go get filippo.io/edwards25519@latest
go mod tidy && go vet ./... && go test ./...
```

---

## Priority 3 — Low (track but defer)

These are recurring Lows that are stable, low-impact, and have specific known workarounds:

- **Finding 6:** BQL `fieldToColumn` self-validation — only matters if SQLBuilder is used outside the validated path. Reject unknown fields directly in `fieldToColumn` when SQLBuilder enters production.
- **Finding 7:** Dolt DB-name regex validation — defense-in-depth on already-correct escaping.
- **Finding 11:** Apply `regexp.QuoteMeta` in `pkg/correlation/explicit.go:233` (1-line change).
- **Finding 12:** Stop discarding errors at the 3 known sites; minimum is `slog.Warn`.
- **Finding 13:** Windows lock TOCTOU is platform-constrained; current behavior is acceptable.
- **Finding 14:** Honor `BT_DOLT_USER`/`BT_DOLT_PASSWORD` env vars when set (only matters if/when remote Dolt servers are used).
- **Finding 15:** Wrap `runTUIProgram` in a `defer recover` that resets the terminal state before re-raising.

## Filing Plan

The user has chosen "Report + file beads". Each Medium and each Low gets a bt-* bead under the appropriate area label. See "File bt-* beads for findings" in the next phase. Info findings stay in the report only.
