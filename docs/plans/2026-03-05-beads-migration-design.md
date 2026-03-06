# Beads Migration: Prefix, Role, and Folder Rename

**Date:** 2026-03-05
**Status:** Approved, not yet implemented

## Problem

The beadstui project has leftover naming from the Jeffrey-era fork:
- Beads issue prefix is `bv-` (should match binary name `bt`)
- `beads.role` git config not set (v0.58 warning on every `bd` command)
- Local folder is still `beads_viewer` (remote is already `seanmartinsmith/beadstui.git`)

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Issue prefix | `bt-` | Matches binary name, clean break from Jeffrey's `bv-` |
| Jeffrey's 550+ closed issues | Keep | They're invisible in daily workflow, valuable for archaeology |
| Dolt database name (`"bv"` in metadata.json) | Leave as-is | Just a MySQL identifier, changing risks Dolt history |
| beads.role | `maintainer` | User owns this repo |
| Folder name | `bt` | Short, matches binary |
| Re-init beads? | No | Existing database works fine, just needs config fix |

## Implementation Steps

Order matters - prefix rename requires running Dolt server, folder rename restarts it.

### Step 1: Prefix rename (while Dolt is running)

```bash
bd dolt start                  # ensure Dolt is running
bd rename-prefix bt- --dry-run # preview (553 issues renamed)
bd rename-prefix bt-           # execute
bd list --status=open          # verify: bt-xft1 should appear
```

### Step 2: Set role config

```bash
git config beads.role maintainer
bd stats                       # verify: no "role not configured" warning
```

### Step 3: Commit beads changes

The prefix rename modifies the Dolt database. Commit any tracked changes.

```bash
git status                     # check for changes
git add -A && git commit -m "chore: rename beads prefix bv->bt, set role"
git push
```

### Step 4: Stop Dolt server

```bash
bd dolt stop
bd dolt killall                # clean up any orphans
```

### Step 5: Folder rename (user does in PowerShell)

```powershell
cd C:\Users\sms\System\tools
Rename-Item beads_viewer bt
cd bt
```

### Step 6: Post-rename cleanup

```powershell
# Copy Claude auto-memory to new path
$oldPath = "$env:USERPROFILE\.claude\projects\C--Users-sms-System-tools-beads-viewer"
$newPath = "$env:USERPROFILE\.claude\projects\C--Users-sms-System-tools-bt"
Copy-Item -Recurse $oldPath $newPath
```

### Step 7: Verify everything works

```bash
cd /c/Users/sms/System/tools/bt
bd stats                       # auto-starts Dolt on new derived port
bd list --status=open          # verify bt-xft1 loads
go build ./cmd/bt/             # verify build still works
bt                             # smoke test TUI
```

## Side Effects

- ADR changelog, memory files, and git commit messages still reference `bv-xxxx` IDs. These are historical - no need to rewrite.
- Dolt server port will change (hash-derived from path). Auto-start handles this.
- `.beads/dolt/config.yaml` still has old port 13729 - `bd dolt start` overwrites it.

## Risk

Low. The prefix rename is a well-tested beads command. The folder rename is a filesystem operation that doesn't touch any project data. The only manual step is copying the Claude memory directory.
