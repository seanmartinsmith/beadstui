# Beads Migration Session 2: Post-Rename

**Context**: Phases A-C are complete. The beads prefix is `bt-`, `beads.role=maintainer` is set, and the folder has been renamed from `beads_viewer` to `bt`. This doc covers Phases D+E.

**Orientation files**:
- `docs/archive/plans/2026-03-05-beads-migration-design.md` - approved design
- `docs/adr/001-btui-fork-takeover.md` - project spine
- `~/.claude/projects/C--Users-sms-System-tools-bt/memory/MEMORY.md` - auto-memory (new path)

---

## Phase D: Post-Rename Updates

### D1. Verify Dolt auto-starts on new path
```bash
bd stats                       # should auto-start Dolt on new hash-derived port
bd list --status=open          # should show bt-xft1
```
The old port (13729) in `.beads/dolt/config.yaml` gets overwritten by `bd dolt start` with the new derived port.

### D2. Update MEMORY.md
In `~/.claude/projects/C--Users-sms-System-tools-bt/memory/MEMORY.md`:
- Update "Current State" section: folder renamed to `bt`, beads prefix is `bt-`
- Update "Open Beads": `bv-9x36` -> `bt-9x36`, `bv-xft1` -> `bt-xft1`
- Update "Closed Beads": `bv-nkil` -> `bt-nkil`, `bv-1p3a` -> `bt-1p3a`
- Update "If Local Folder Gets Renamed" section: path is now `C--Users-sms-System-tools-bt`
- Update Dolt server restart note

### D3. Update ADR changelog
Add entry to `docs/adr/001-btui-fork-takeover.md`:
```
| 2026-03-07 (session 10) | **Beads migration**: Renamed issue prefix bv->bt (553 issues via `bd rename-prefix`). Set beads.role=maintainer. Local folder renamed beads_viewer->bt. Claude memory copied to new project path. |
```

### D4. Commit post-rename updates
```bash
git add docs/adr/001-btui-fork-takeover.md
git commit -m "docs: update ADR changelog for beads migration"
git push
```

---

## Phase E: Final Verification

### E1. Build and install
```bash
go build ./cmd/bt/             # must compile
go test ./pkg/ui/ -run "TestRenderTitledPanel"  # sanity check
go install ./cmd/bt/           # install binary
```

### E2. Smoke test
```bash
bt                             # launch TUI - verify it loads issues
# Press 'i' for insights, 'b' for board, '?' for help
# Press 'q' to quit
```

### E3. Beads health check
```bash
bd stats                       # no warnings
bd ready                       # should work
bd search "rename"             # should search successfully
```

**Done when**: Build passes, TUI launches, beads commands work without warnings.
