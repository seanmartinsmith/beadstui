# Attack Surface Map - beadstui (bt)

**Date:** 2026-04-09

## Entry Points

### User Input (TUI)
- `:` keybind → BQL modal → `bql.Parse()` → `bql.Validate()` → `MemoryExecutor`
- `/` search → text filter (no parsing, substring match)
- `e` edit → launches allowlisted GUI editor with file path arg
- `y` copy → writes to system clipboard via stdin pipe
- Various keybinds → state changes (no external execution)

### CLI Arguments
- `--bql <query>` → same BQL pipeline as TUI
- `--robot-bql <query>` → BQL + JSON envelope output (has envelope bypass bug)
- `--as-of <revision>` → `git rev-parse` (validated with `--end-of-options`)
- `--from <revision>` / `--to <revision>` → `git rev-parse` (validated)
- Positional args → file/directory paths (filesystem resolved)

### Configuration Files
- `.bt/hooks.yaml` → **shell command execution** via `sh -c` / `cmd /C`
- `.bt/theme.yaml` / `~/.config/bt/theme.yaml` → YAML unmarshal to typed struct
- `.bt/recipes.yaml` / `~/.config/bt/recipes.yaml` → YAML unmarshal to typed struct
- `.bt/workspace.yaml` → YAML unmarshal to typed struct
- `.bt/drift.yaml` → YAML unmarshal to typed struct
- `.beads/metadata.json` → JSON unmarshal for DSN construction
- `.beads/dolt/config.yaml` → YAML unmarshal for Dolt config
- `.beads/dolt-server.port` / `.beads/dolt-server.pid` → parsed as integers

### Environment Variables
- `GITHUB_TOKEN` / `GH_TOKEN` → HTTP Authorization header (scoped to GitHub)
- `EDITOR` / `VISUAL` → editor launch (allowlisted)
- `BEADS_DIR` → data directory override (path, not command)
- `BT_DOLT_PORT` / `BEADS_DOLT_SERVER_PORT` → port integers
- `BT_CACHE_DIR` / `BT_WORKER_TRACE` → file paths

### Data Files
- `.beads/issues.jsonl` → JSON unmarshal per line (typed struct, 10MB line cap)
- `.beads/*.db` → SQLite database (read-only mode)
- Dolt database → MySQL protocol queries (localhost only)

## Data Flows

```
User Input Flows:
  BQL query string → Lexer → Parser → Validator → MemoryExecutor → filtered issues
                                                 ↘ (future) SQLBuilder → Dolt SQL query
  CLI --as-of → resolveRevision → git rev-parse --verify --end-of-options → SHA
  CLI positional → filepath.Abs → datasource discovery → JSONL/SQLite/Dolt

Config File Flows:
  .bt/hooks.yaml → YAML parse → hook.Command → exec.Command(shell, "-c", command)
  .bt/theme.yaml → YAML parse → ThemeFile struct → Lipgloss styles
  .beads/metadata.json → JSON parse → DoltConfig → DSN string → sql.Open()

Data → Git CLI Flows:
  bead ID (from JSONL/Dolt) → regexp.QuoteMeta → git -G (escaped)
  bead ID → git --grep=<id> (NOT escaped)
  commit SHA (from git log) → git show <sha> (NOT validated as hex)
  author email (from bead data) → git --author=<email> (NOT validated)

Network Flows:
  GitHub API → HTTPS GET → JSON response → release metadata
  GitHub releases → HTTPS GET → tar.gz download → SHA256 verify → extract binary
  Dolt server → TCP 127.0.0.1:<port> → MySQL protocol → SQL queries

Export Flows:
  repo/project name (user wizard) → exec.Command("gh", "repo", "create", name)
  URL (constructed) → exec.Command("cmd", "/c", "start", url)  [WINDOWS INJECTION RISK]
  URL (constructed) → exec.Command("rundll32", "url.dll,FileProtocolHandler", url)  [SAFE]
```

## Abuse Paths

### Path 1: Malicious Repository Clone (Hook Injection)
```
Attacker crafts repo with .bt/hooks.yaml containing malicious commands
→ User clones repo and runs `bt`
→ Hook executor reads .bt/hooks.yaml
→ exec.Command("sh", "-c", "<malicious command>")
→ Arbitrary code execution as user
```
**Severity**: High (but by-design, same as git hooks)

### Path 2: Windows URL Command Injection
```
User enters project name with shell metacharacters in export wizard
→ URL constructed: "https://.../<name>"
→ exec.Command("cmd", "/c", "start", url)
→ Shell interprets metacharacters (&, |, etc.)
→ Secondary command execution
```
**Severity**: Medium (requires user interaction in wizard)

### Path 3: Malicious JSONL → Git Argument Injection
```
Attacker crafts .beads/issues.jsonl with malicious bead ID
→ bt loads issues, user triggers correlation
→ bead ID flows to git --grep=<id> without escaping
→ Crafted ID could manipulate git behavior (regex, not shell)
```
**Severity**: Low (git argument, not shell; requires local file write access)

### Path 4: BQL Parser DoS via CLI
```
Attacker provides deeply nested BQL via --bql flag
→ Parser recurses without depth limit
→ Goroutine stack grows unboundedly
→ Process crashes with stack overflow or OOM
```
**Severity**: Medium (requires CLI access; TUI impractical for long input)

### Path 5: Latent SQL Injection in BQL SQLBuilder
```
Future: DoltExecutor activates SQLBuilder
→ dateToSQL interpolates value without parameterization
→ If lexer changes allow non-digit chars, SQL injection opens
```
**Severity**: Medium (latent - not exploitable today)
