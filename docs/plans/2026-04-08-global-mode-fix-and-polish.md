# Plan: Fix and Polish Global Mode

**Date:** 2026-04-08
**Parent bead:** bt-ssk7 (global federation)
**Parent design problem:** sms-ncb (global beads)
**Audit doc:** docs/audit/global-mode-readiness.md

## Orientation

Global mode (`bt --global`) exists and loads 2168 issues across 13 databases on the shared Dolt server (port 3308). However, dogfooding revealed it's half-baked:

- Dependencies don't load at all (bt-ktig, P1)
- Polling/refresh doesn't work (bt-pjvi, P1)
- Labels and comments may also be broken (untested, part of bt-ktig)
- UX needs work at 2k+ issue scale

The code is in place - `loadAllDependencies`, `globalDoltPollOnce`, workspace mode UI all exist. The bugs are likely small (schema mismatch, wiring issue, scan error being swallowed). This is a fix-and-validate session, not a build-from-scratch session.

## Key Files

- `internal/datasource/global_dolt.go` - GlobalDoltReader, UNION ALL queries, loadAllDependencies/Labels/Comments
- `internal/datasource/global_dolt.go:249` - buildDependenciesQuery
- `internal/datasource/global_dolt.go:437` - loadAllDependencies (the broken one)
- `pkg/ui/background_worker.go:2086-2115` - globalDoltPollOnce
- `pkg/ui/background_worker.go:597-613` - poll loop startup (check if global path activates)
- `cmd/bt/main.go:530-565` - global mode loading branch
- `cmd/bt/main.go:552` - `beadsPath = ""` (may disable watcher/poll)

## Phase 1: Fix Data Loading (P1 blockers)

### Step 1: Diagnose dependency loading (bt-ktig)

The UNION ALL query works when run directly against Dolt. `loadAllDependencies` runs without error but attaches nothing. Likely causes:

1. **Schema mismatch across databases** - some DBs may have extra columns or different `type` column that breaks the UNION ALL. Test: run the exact generated query against the shared server.
2. **Scan error swallowed** - line 451 `continue` on scan error. Add temporary logging to see if rows are scanned at all.
3. **issueMap key mismatch** - the dep's `issue_id` may not match the key format in issueMap. Compare what the query returns vs what's in the map.

Diagnostic approach:
```go
// Temporarily add to loadAllDependencies after rows.Next():
var count int
// ... in the loop:
count++
slog.Info("global dep", "issue_id", issueID, "depends_on", dependsOnID, "found_in_map", issueMap[issueID] != nil)
// After loop:
slog.Info("global deps loaded", "total_rows", count)
```

If total_rows is 0, the query itself is failing silently. If rows exist but `found_in_map` is false, it's a key mismatch.

### Step 2: Validate labels and comments

Same pattern - check if `loadAllLabels` and `loadAllComments` are also silently failing. Quick test:
```bash
bt --robot-bql --bql "id = bt-ssk7"           # single-repo: check labels
bt --global --robot-bql --bql "id = bt-ssk7"   # global: compare
```

### Step 3: Fix polling (bt-pjvi)

Check if the poll loop even starts for global mode:
- `background_worker.go:597-613`: does `SourceTypeDoltGlobal` match the condition?
- `main.go:552`: `beadsPath = ""` - does this skip the watcher AND the poll loop?
- Add a log line to `globalDoltPollOnce` to confirm it's being called.

If the poll loop isn't starting, the fix is ensuring the `startDoltPollLoop()` goroutine launches for global sources.

### Step 4: End-to-end validation

After fixes, verify ALL data paths work in global mode:
```bash
# Issues load
bt --global --robot-bql --bql "status = open" | python -c "import sys,json; print(json.load(sys.stdin)['count'])"

# Dependencies load
bt --global --robot-bql --bql "id = bt-ammc" | python -c "import sys,json; i=json.load(sys.stdin)['issues'][0]; print(f'deps: {len(i.get(\"dependencies\",[]))}')"

# Graph has edges
bt --global --robot-graph | python -c "import sys,json; d=json.load(sys.stdin); print(f'nodes: {d[\"nodes\"]}, edges: {d[\"edges\"]}')"

# Polling works (create an issue in another terminal, check if bt refreshes)

# Labels load (check if any issue shows labels)
bt --global --robot-bql --bql "id = bt-ssk7" | python -c "import sys,json; i=json.load(sys.stdin)['issues'][0]; print(f'labels: {i.get(\"labels\",[])}')"
```

## Phase 2: UX Polish (after Phase 1 is solid)

### Step 5: Auto-global default (bt-vbac)

Change `main.go` loading logic:
1. Before the existing three branches, check for shared server existence
2. If shared server exists AND cwd has `.beads/metadata.json`: connect to shared server, load all, but set `activeRepos` to only the current project's database name
3. If shared server exists AND no `.beads/metadata.json`: global mode, all projects visible
4. `--global` flag overrides to "all projects visible"
5. `w` key still works to toggle scope

This is the biggest UX change - test it carefully.

### Step 6: Rename to "project" (bt-hgav)

Codebase-wide rename:
- `workspaceMode` -> keep internal name for now, just change user-facing strings
- "Repo Filter" modal title -> "Project Filter"
- Footer "13 repos" -> "13 projects"
- `w` key hint: "w repos" -> "w projects"
- Help overlay: "Repo picker" -> "Project picker"

### Step 7: Project picker visual refresh (bt-714y)

- Use RoundedBorder (match panel.go convention)
- Show issue count per project: `beads (16)  bt (610)  cass (83)`
- Consider color coding by project health or issue count

### Step 8: Alerts at scale (bt-5mgs)

Needs design thought. Options:
- Group by project, show counts not individual issues
- Filter by severity in the modal
- Only show critical by default, expand to see warnings
- Add a "dismiss all warnings" action

## Beads Reference

| Bead | Priority | What | Phase |
|------|----------|------|-------|
| bt-ktig | P1 | Deps don't load in global mode | 1 |
| bt-pjvi | P1 | Polling broken in global mode | 1 |
| bt-vbac | P2 | Auto-global default + mode toggle | 2 |
| bt-hgav | P2 | Rename to "project" | 2 |
| bt-5mgs | P2 | Alerts at scale | 2 |
| bt-714y | P3 | Project picker visual refresh | 2 |

## Cross-Project Deps (bonus, after Phase 2)

The beads session implemented a fix for `bd dep add` with cross-prefix targets (gastownhall/beads#3134). Once that PR lands upstream:
- `bd dep add bt-ssk7 bd-fk1` will store `depends_on_id = "bd-fk1"`
- bt's graph builder should resolve it in global mode automatically (both issues in issueMap)
- Test this after Phase 1 fixes dependency loading

## What NOT to Do

- Don't refactor the monolith (bt-if3w) - that's a separate effort
- Don't build TUI CRUD (bt-oiaj) - depends on global mode being stable first
- Don't redesign the help system (bt-xavk) - separate effort, not blocking
- Don't touch the Charm v2 migration - wrong time
