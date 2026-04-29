# Recommendations - beadstui (bt)

**Date:** 2026-04-09
**Priority-ordered mitigations for confirmed findings**

---

## Priority 1 - Medium (Fix Before Ship)

### 1. Fix Windows `cmd /c start` URL Injection
**Finding:** [Windows Command Injection](./findings.md#medium-finding-1-windows-command-injection-via-cmd-c-start)
**Effort:** Minimal (3 lines per file)
**Files:** `pkg/export/github.go:610`, `pkg/export/cloudflare.go:347`

```go
// Before (vulnerable on Windows)
case "windows":
    cmd = exec.Command("cmd", "/c", "start", url)

// After (safe - already used in pkg/ui/model_export.go:79)
case "windows":
    cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
```

### 2. Add BQL Parser Recursion Depth Limit
**Finding:** [BQL Parser Stack Overflow](./findings.md#medium-finding-3-bql-parser-stack-overflow-via-recursion)
**Effort:** Minimal (add depth counter to Parser and evalExpr)
**Files:** `pkg/bql/parser.go`, `pkg/bql/memory_executor.go`

```go
// In Parser struct, add: depth int
// In parseFactor(), add at top:
p.depth++
defer func() { p.depth-- }()
if p.depth > 100 {
    return nil, fmt.Errorf("query too deeply nested (max 100 levels)")
}

// In evalExpr(), add depth parameter or use a similar counter
```

### 3. Parameterize BQL dateToSQL
**Finding:** [SQL Injection in dateToSQL](./findings.md#medium-finding-4-sql-injection-in-bql-datetosql-latent)
**Effort:** Minimal (change fmt.Sprintf to parameterized query)
**File:** `pkg/bql/sql.go:219-228`

```go
// Before
return fmt.Sprintf("DATE_SUB(CURDATE(), INTERVAL %s DAY)", value)

// After
b.params = append(b.params, value)
return "DATE_SUB(CURDATE(), INTERVAL ? DAY)"
```

Do this for all three date branches (DAY, HOUR, MONTH). Must be done before DoltExecutor is activated.

### 4. Validate Git SHA Arguments
**Finding:** [Git Argument Injection](./findings.md#medium-finding-5-git-argument-injection-via-unvalidated-shas)
**Effort:** Small (add validation function, call at each site)
**Files:** `pkg/correlation/incremental.go`, `stream.go`, `reverse.go`, `temporal.go`, `cocommit.go`

```go
var validSHA = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

func validateSHA(sha string) error {
    if !validSHA.MatchString(sha) {
        return fmt.Errorf("invalid git SHA: %q", sha)
    }
    return nil
}
```

### 5. Document Hook Security Model
**Finding:** [Hook Command Execution](./findings.md#medium-finding-2-arbitrary-command-execution-via-hook-config-files)
**Effort:** Minimal (documentation)

Add a security note to the README and/or a first-run warning:
- `.bt/hooks.yaml` can execute arbitrary commands
- Same threat model as git hooks
- Consider `--no-hooks` flag for untrusted repositories

---

## Priority 2 - Low (Fix This Sprint)

### 6. Harden BQL fieldToColumn
**File:** `pkg/bql/sql.go:186`
```go
// Before
return "i." + field

// After
return "", fmt.Errorf("unknown BQL field: %q", field)
```

### 7. Add Regex Validation to Global Dolt DB Names
**File:** `internal/datasource/global_dolt.go` (before buildIssuesQuery call)
```go
var validDBName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
for _, db := range databases {
    if !validDBName.MatchString(db) {
        continue // skip suspicious database names
    }
    // ... existing query building
}
```

### 8. Cap BQL IN List Size
**File:** `pkg/bql/parser.go:216-230`
```go
const maxINValues = 1000
// After appending a value:
if len(values) > maxINValues {
    return nil, fmt.Errorf("IN clause exceeds maximum of %d values", maxINValues)
}
```

### 9. Add io.LimitReader to Updater Extraction
**File:** `pkg/updater/updater.go:548`
```go
const maxBinarySize = 200 * 1024 * 1024 // 200MB
if _, err := io.Copy(out, io.LimitReader(tr, maxBinarySize)); err != nil {
```

### 10. Remove DSN from Error Messages
**File:** `internal/datasource/load.go:159`
Wrap errors with generic message instead of including connection string.

### 11. Apply regexp.QuoteMeta to Git --grep
**File:** `pkg/correlation/explicit.go:233`
```go
"--grep=" + regexp.QuoteMeta(pattern),
```

### 12. Handle Swallowed Errors
**Files:** `cmd/bt/main.go:494`, `pkg/hooks/config.go:93`, `pkg/recipe/loader.go:74`
At minimum: `slog.Warn("failed to ...", "error", err)` instead of `_ =`.

---

## Priority 3 - Info (Hygiene / Monitor)

### 13. Update edwards25519
```bash
go get filippo.io/edwards25519@v1.1.1 && go mod tidy && go mod vendor
```

### 14. Update golang.org/x/image
```bash
go get golang.org/x/image@v0.38.0 && go mod tidy && go mod vendor
```

### 15. Monitor toon-go Dependency
Nascent dependency (2 stars, shells out to `tru` binary). Low risk now but review periodically.
