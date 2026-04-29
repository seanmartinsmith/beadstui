# 72l8 §7: Local filesystem hygiene (out-of-repo, user-level)

Audit date: 2026-04-29
Methodology: read-only `ls` / `cat` / `grep` against user-level paths. No remediation performed; no beads filed in this project.

## ~/.bt/

```
events.jsonl    (601 lines, 199703 bytes — bt event log, expected)
semantic/
  index-hash-384.bvvi    (5.4 MB — bt vector index, .bvvi format is bt-native per AGENTS.md)
settings.json   (59 bytes)
```

`settings.json` content:
```json
{ "anchor_project": "C:\\Users\\sms\\System\\tools\\bt" }
```

No `bv` references. Note: `.bvvi` extension is bt's own vector index format (`pkg/search/vector_index.go`) — superficially looks like "bv-vi" but is unrelated to bv-era branding. Leave as-is.

## ~/.beads/

```
formulas/
  cross-project-workstream.formula.toml
  research-audit.formula.toml
  session-close.formula.toml
registry.json       (2 bytes)
registry.lock       (0 bytes)
shared-server/
  dolt/
  dolt-server.lock
  dolt-server.log
  dolt-server.pid
  dolt-server.port
```

Grep across `~/.beads/formulas/` for `bv|beads_viewer|BV_|bv.exe|bv-graph`: **No matches**. Clean.

## PATH for bv binaries

`~/go/bin/` contains stale bv-era binaries:

```
-rwxr-xr-x  52,933,120 Feb 25 14:48  bv.exe
-rwxr-xr-x  42,469,888 Dec 24 16:10  bv.exe.backup
-rwxr-xr-x  52,932,608 Feb 25 12:04  bv.exe~
```

Plus current `bt.exe` (live). The three `bv.exe*` files are pre-rename leftovers totaling ~148 MB. Since `~/go/bin` is on PATH, typing `bv` still resolves to a working pre-rename binary — risk of muscle-memory invocations of the old fork.

## Shell completions

Searched `~` (depth 5) for `*/Completions/*bv*`, `_bv*`, `bv.ps1`: **none found**. No completion residue.

## ~/.files/powershell/profile.ps1

Grep for `\bbv\b|beads_viewer|BV_|bv\.exe|bv-graph` (case-insensitive, word-boundary): **No matches**.

Profile read in full (112 lines):
- `BD_ACTOR=sms` set (expected per project memory)
- `sms` / `s07` gh auth switch functions present
- No bv aliases, no BV_ env vars, no references to old binaries

Also checked:
- `~/OneDrive/Documents/PowerShell/Microsoft.PowerShell_profile.ps1` — single line `. "$HOME\.files\powershell\profile.ps1"` (sources canonical). Clean.
- `~/.files/archive/Microsoft.PowerShell_profile.ps1` — grep clean.

## Findings

Severity legend: user-flag (informational; user decides), low (safe to ignore), medium (worth cleaning), nil (no action).

| # | Finding | Severity |
|---|---|---|
| 1 | `~/go/bin/bv.exe`, `bv.exe.backup`, `bv.exe~` present (~148 MB total). On PATH; typing `bv` still resolves. | medium |
| 2 | `~/.bt/settings.json`, `events.jsonl`, `semantic/index-hash-384.bvvi` — all bt-native, no bv-era references | nil |
| 3 | `~/.beads/` (formulas, registry, shared-server) — clean | nil |
| 4 | PowerShell profile (`~/.files/powershell/profile.ps1`) — clean | nil |
| 5 | OneDrive PS profile + archive PS profile — clean | nil |
| 6 | No shell completions for `bv` anywhere under `~` | nil |

## Recommendations

User-level cleanup (sms decides; not repo-actionable):

1. **Delete the three bv binaries** in `~/go/bin/`:
   ```pwsh
   Remove-Item C:\Users\sms\go\bin\bv.exe, C:\Users\sms\go\bin\bv.exe.backup, C:\Users\sms\go\bin\bv.exe~
   ```
   Reclaims ~148 MB. Eliminates risk of accidentally invoking the pre-rename binary out of muscle memory. Only impact: `bv` command stops resolving — `bt` is the replacement and is already installed.

2. **No other action needed.** `~/.bt/`, `~/.beads/`, profiles, and completions are all clean.

Out of scope for this section but flagged for the broader bt-72l8 epic if not already covered: the three `bv.exe*` files are the only meaningful bv-era residue on the user's machine outside the repo itself.
