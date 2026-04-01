# Brainstorm: BQL Import from Perles

**Date**: 2026-04-01 (Session 16)
**Status**: Ready for planning
**Participants**: sms + Claude

## What We're Building

Composable structured search for bt. Vendor the BQL (Beads Query Language) parser from zjrosen/perles (MIT licensed), write a bt-native executor, wire it into the TUI as a new `:` command modal.

BQL adds the ability to write queries like `status:open priority<2 label:bug` - boolean logic, date filters, relationship traversal - on top of bt's existing search stack.

## Why This Approach

bt already has deep search (fuzzy, semantic embeddings, hybrid scoring with PageRank/betweenness/triage). What it lacks is **composability** - no way to combine filter dimensions freely from the keyboard. BQL is the composability glue.

Building a query language parser from scratch is expensive. Perles already has one (MIT, hand-rolled recursive descent, zero external dependencies, 5,147 LOC of tests). The parser layer is pure Go - copy it, adapt 6 log calls, done. The executor needs rewriting against bt's data model regardless, which is the right seam.

### Competitive context

- **perles** has BQL but no CRUD and no workspace/multi-project support
- **lazybeads** is DOA (not maintained)
- If bt ships BQL + workspace auto-discovery + write mode, that's meaningful differentiation - not just feature parity with perles

## Key Decisions

### 1. Activation: `:` opens BQL modal

New keybind `:` opens a dedicated BQL input overlay. Coexists with existing search:

```
/    -> fuzzy/semantic text search (unchanged)
:    -> BQL query modal (new)
o    -> filter open (unchanged, becomes shortcut for :status:open)
l    -> label picker (unchanged)
'    -> recipe picker (unchanged)
```

Existing search engines (`/` fuzzy, `ctrl+s` semantic) stay exactly as-is. BQL is additive.

### 2. Scope: BQL replaces global filters

BQL queries are the complete filter expression. Status keys (`o`/`c`/`r`/`a`) become shortcuts for common BQL queries:

- `o` = `:status:open`
- `c` = `:status:closed`
- `r` = `:status:open NOT blocked:true`

Recipes evolve into saved BQL queries (future - not this sprint).

### 3. Executor: in-memory now, SQL interface for later

**Sprint ships**: in-memory executor that walks `m.issues` (Go structs already loaded). Matches how every other filter in bt works today. Covers single-project and current workspace mode.

**Interface designed for**: SQL/Dolt executor that pushes BQL-generated WHERE clauses to Dolt. Needed when global beads scales beyond what fits in memory.

```go
type BQLExecutor interface {
    Execute(expr ast.Expr, opts ExecuteOpts) ([]model.Issue, error)
}

// Sprint: MemoryExecutor walks m.issues
// Future: DoltExecutor generates SQL, queries Dolt
```

### 4. What to copy from perles

| File | LOC | Action |
|------|-----|--------|
| ast.go | 129 | Copy as-is |
| token.go | 168 | Copy as-is |
| lexer.go | 189 | Copy as-is |
| parser.go | 453 | Copy, swap 6 internal/log calls |
| validator.go | 240 | Copy as-is |
| sql.go | 377 | Copy, strip SQLite dialect (bt is Dolt-only). Not used in sprint but needed for future SQL executor. |
| executor.go | 923 | Skip - rewrite as bt-native MemoryExecutor (~300 LOC) |
| styles.go | 64 | Skip for now - rewrite later against bt's theme system |
| syntax_adapter.go | 153 | Skip for now |

**Tests**: lexer/parser/validator tests are portable (~2,000 LOC). Executor tests need rewriting.

### 5. BQL syntax (from perles)

- Operators: `=`, `!=`, `<`, `>`, `<=`, `>=`, `~` (contains), `!~`
- Set operators: `IN`, `NOT IN`
- Boolean: `AND`, `OR`, `NOT`, parentheses
- Date literals: `today`, `yesterday`, `-7d`, `-24h`, `-3m`
- Priority shorthand: `P0`-`P4`
- Sort: `ORDER BY field ASC|DESC`
- Graph traversal: `EXPAND up/down/all DEPTH n` (dependency traversal)

## Future Integration: Global Beads Command Center

This sprint's BQL is single-project (and workspace-mode via existing multi-repo loading). The architecture is designed to scale to global beads - here's how.

### The vision

bt as the global command center: see all issues across all projects, filter with BQL (`project:bt status:open priority<2`), drill down into a single project, pop back out to global view.

### What beads offers upstream

- `bd repo add/sync` - one project becomes a "hub", hydrates issues from all others. This is the supported upstream path.
- Federation infrastructure exists but isn't wired: `federation_peers` table, `external:beads:<id>` format, `internal/storage/federation.go`
- Open proposal #2826: `bd repo add dolthub://org/repo` for Dolt remote cache (not landed)
- INTEGRATION_CHARTER.md explicitly says cross-project orchestration is out of beads scope. **bt owns this layer.**

### What beads does NOT offer

- **No global project discovery.** `~/.beads/registry.json` is dead code - it was a daemon PID registry (tracking running Dolt server processes), and the entire daemon system was removed in Feb 2026 (~24k lines deleted). The codebase explicitly skips it in `hasBeadsProjectFiles()`. Auto-discovery is 100% bt's problem to solve.

### Two paths for global BQL

**Option A: bt loads N databases directly**
- Current workspace.yaml approach + auto-discovery layer
- BQL runs across the merged in-memory set (small scale) or pushes SQL to each embedded Dolt instance (large scale)
- bt owns the full stack
- Embedded Dolt is default since beads v0.63.x - no server process per project, makes this viable since bt doesn't need to coordinate Dolt servers

**Option B: bt points at a single bd repo hub**
- One project aggregates everything via `bd repo add/sync`
- BQL runs against one database
- Simpler query model but adds sync step and dependency on hub being current

**Likely answer**: Option A for bt's use case (independence from upstream sync mechanisms, works offline, bt controls the UX). Option B could work as an alternative for teams using hub repos.

### Navigation model (future)

- Global view: all projects, filterable by BQL with `project:` field
- Drill-down: select a project, switch to single-project view with full board/graph/detail
- Pop-back: return to global view preserving filter state
- Current workspace mode's repo picker (`w` key) is a prototype of this

## Open Questions

None for this sprint. Global beads design questions are captured above for a future brainstorm.

## Sprint Scope Summary

1. Copy BQL parser layer from perles (~1,500 LOC)
2. Write bt-native MemoryExecutor (~300 LOC)
3. Define BQLExecutor interface for future SQL executor
4. Build `:` modal in TUI for BQL input
5. Wire BQL filter results into existing list/board/graph views
6. Port relevant tests from perles (~2,000 LOC)
