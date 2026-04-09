# Threat Model - beadstui (bt)

**Date:** 2026-04-09
**Scope:** Entire codebase (~92k lines Go)
**Application Type:** Local CLI/TUI issue tracker backed by Dolt (MySQL protocol), SQLite, and JSONL

## Assets

| Asset Type | Asset | Description | Priority |
|------------|-------|-------------|----------|
| **Data stores** | Dolt database | Issue data via MySQL protocol on localhost | Critical |
| **Data stores** | SQLite database | Legacy/alternative issue storage | High |
| **Data stores** | JSONL files | Flat-file issue storage | Medium |
| **Data stores** | Config files | YAML configs: theme, hooks, recipes, drift, workspace | Medium |
| **Authentication** | Dolt DSN | `root@tcp(127.0.0.1:<port>)/<db>` - no password | Medium |
| **Authentication** | GitHub token | `GITHUB_TOKEN`/`GH_TOKEN` env vars for updater | High |
| **External services** | GitHub API | Release checking and self-update downloads | High |
| **External services** | `bd` CLI | Beads CLI for write operations (shelled out) | Critical |
| **External services** | `git` CLI | History, correlation, revision resolution | High |
| **External services** | `gh` / `wrangler` CLI | GitHub Pages / Cloudflare Pages export | Medium |
| **User input surfaces** | BQL parser | Hand-rolled query language parser | High |
| **User input surfaces** | CLI flags | `--bql`, `--robot-bql`, `--as-of`, `--from`, `--to` | High |
| **User input surfaces** | TUI keyboard input | Modal dialogs, search, editor launch | Medium |
| **Configuration** | Environment variables | `BT_*`, `BEADS_*`, `EDITOR`, `VISUAL` | Medium |
| **Configuration** | Hook system | `.bt/hooks.yaml` - arbitrary command execution | Critical |
| **Static assets** | Self-updater binary | Downloaded from GitHub releases | High |

## Trust Boundaries

```
Trust Boundaries:
  ├── User TUI input ←→ BQL parser (untrusted input → structured query)
  ├── CLI flags ←→ Application logic (untrusted strings → git/bd args)
  ├── Config files ←→ Application (local files → commands, settings)
  │   ├── .bt/hooks.yaml → shell command execution
  │   ├── .bt/theme.yaml → YAML parsing
  │   └── .beads/metadata.json → DSN construction
  ├── Application ←→ Dolt DB (MySQL protocol, localhost, no auth)
  ├── Application ←→ Git CLI (arguments constructed from data)
  ├── Application ←→ bd CLI (arguments constructed from data)
  ├── Application ←→ GitHub API (HTTPS, token-authenticated)
  ├── JSONL/SQLite data ←→ Application (untrusted data → display + git args)
  ├── GitHub releases ←→ Self-updater (downloaded binary, SHA256 verified)
  └── Application ←→ System clipboard (write-only, non-sensitive data)
```

## STRIDE Threat Matrix

### Spoofing

| Threat | Assets Affected | Risk | Mitigation |
|--------|----------------|------|------------|
| Attacker spoofs Dolt server on expected port | Dolt DB | Medium | Connection validates `SHOW TABLES LIKE 'issues'` |
| Malicious repo with crafted `.bt/hooks.yaml` | Hook system | High | By design - same model as git hooks |
| GitHub release impersonation | Self-updater | Low | SHA256 checksum verification |

### Tampering

| Threat | Assets Affected | Risk | Mitigation |
|--------|----------------|------|------------|
| Malicious JSONL with crafted bead IDs | JSONL data, Git CLI | Medium | IDs flow into git args without full validation |
| Crafted BQL query injecting SQL | BQL parser, future DoltExecutor | Medium (latent) | Currently memory-only; SQL path has gaps |
| Modified config files injecting commands | Hook system | High | No integrity checking on config files |
| Tampered update archive | Self-updater | Low | SHA256 verification, base-name extraction |

### Repudiation

| Threat | Assets Affected | Risk | Mitigation |
|--------|----------------|------|------------|
| No audit logging of bt actions | All operations | Low | Git history provides indirect audit trail |
| Hook execution not logged | Hook system | Medium | No record of which hooks ran or their output |

### Information Disclosure

| Threat | Assets Affected | Risk | Mitigation |
|--------|----------------|------|------------|
| DSN in error messages | Dolt connection | Low | `shortError()` truncates, but DSN visible |
| GitHub token leakage on redirect | GitHub token | Low | Auth header stripped on non-GitHub redirects |
| Stack traces in worker errors | Internal state | Low | `shortError()` truncates before display |

### Denial of Service

| Threat | Assets Affected | Risk | Mitigation |
|--------|----------------|------|------------|
| Deeply nested BQL query (stack overflow) | BQL parser | Medium | No recursion depth limit |
| Unbounded IN list in BQL | BQL parser | Low | Linear memory growth |
| Decompression bomb in update archive | Self-updater | Low | Expected size check on download |

### Elevation of Privilege

| Threat | Assets Affected | Risk | Mitigation |
|--------|----------------|------|------------|
| Hook command injection via cloned repo | Hook system | High | No sandboxing - executes as user |
| `cmd /c start <url>` injection on Windows | Browser launch | Medium | URL not sanitized for shell metacharacters |
| Editor env var injection | EDITOR/VISUAL | Low | Allowlist blocks shells/interpreters |
