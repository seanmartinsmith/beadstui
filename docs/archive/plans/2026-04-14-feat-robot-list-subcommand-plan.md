---
title: "feat: Add bt robot list subcommand for agent-consumable issue listing"
type: feat
status: completed
date: 2026-04-14
bead: bt-kkql
origin: docs/brainstorms/2026-04-12-cross-project-management.md
---

# bt robot list

Agent-consumable issue listing with filters. The 80% use case that `bt robot bql` handles but with simpler, memorable flags instead of a query language.

## Acceptance Criteria

- [x] `bt robot list` returns issues as JSON envelope (works in single-project and `--global` mode)
- [x] Filter flags: `--status`, `--priority`, `--type`, `--has-label` (new), `--repo` and `--global` (existing root persistent flags)
- [x] Truncation metadata in response: `total`, `truncated`, `limit`
- [x] `--limit N` flag with sensible default (100), `--limit 0` for unlimited
- [x] Output uses standard `RobotEnvelope` + `newRobotEncoder` (JSON and TOON)
- [x] No Bubble Tea dependency in the code path (headless-only)

## Implementation

### Pattern to follow: `robotBQLCmd` (cobra_robot.go:285-333)

This is the closest existing subcommand - it loads issues, filters them, and outputs a JSON envelope with a count. `robot list` does the same thing but with flag-based filtering instead of BQL parsing.

Key pattern:
1. Call `loadIssues()` directly (not `robotPreRun()`). `robotPreRun()` applies persistent flags like `--bql` and `--label` (subgraph scoping) which are wrong semantics for a simple list command. Persistent root flags (`--global`, `--repo`, `--format`) still apply because they're resolved in `rootPersistentPreRun`.
2. Filter in-memory
3. Build anonymous struct embedding `RobotEnvelope`
4. Encode with `newRobotEncoder(os.Stdout)`
5. `os.Exit(0)`

### New file: `cmd/bt/robot_list.go`

Contains:
- `robotListCmd` cobra command definition
- `filterIssuesForList()` function applying `--status`, `--priority`, `--type`, `--has-label` flags
- Output struct with truncation metadata

### Registration: `cobra_robot.go` init()

Add alongside existing subcommands:
```
robotListCmd.Flags().String("status", "", "Filter by status (open, blocked, in_progress, closed)")
robotListCmd.Flags().String("priority", "", "Filter by priority range (e.g., '0-1', '2')")  
robotListCmd.Flags().String("type", "", "Filter by issue type (bug, feature, task)")
robotListCmd.Flags().String("has-label", "", "Filter by label (exact match)")
robotListCmd.Flags().Int("limit", 100, "Max issues to return (0 = unlimited)")
robotCmd.AddCommand(robotListCmd)
```

Note: `--repo`, `--global`, and `--format` are already root persistent flags - they apply automatically.

### Output shape

```json
{
  "generated_at": "2026-04-14T12:00:00Z",
  "data_hash": "abc123",
  "output_format": "json",
  "version": "0.0.1",
  "query": {
    "status": "open",
    "priority": "0-1",
    "type": "",
    "has_label": "",
    "repo": "cass",
    "global": true,
    "limit": 100
  },
  "total": 247,
  "truncated": true,
  "limit": 100,
  "count": 100,
  "issues": [...]
}
```

`total` = count before limit applied. `count` = len(issues) in response. `truncated` = total > limit. This addresses the gastownhall/beads#3280 gap where bd list --json has no truncation indicator.

### Filter semantics

- `--status`: comma-separated, match any (e.g., `--status open,blocked`). Default: all (tombstone already excluded at SQL layer via `WHERE status != 'tombstone'` in `buildIssuesQuery`).
- `--priority`: single int or range with dash (e.g., `0`, `0-1`). Default: all.
- `--type`: exact match on issue_type field. Default: all.
- `--has-label`: exact match, issue must have this label. Named `--has-label` (not `--label`) because `robotCmd` has a persistent `--label` flag (`robotFlagLabelScope`) that does subgraph scoping - pulling in all transitively connected issues. Different semantics. The persistent `--label` still works if someone passes it alongside, but `robot list` doesn't call `robotPreRun()` so it would be ignored.

## Context

- Brainstorm origin: `docs/brainstorms/2026-04-12-cross-project-management.md` - portfolio health dashboard needs this data layer
- Cross-project refs: cass-zzi (cass wants cross-project bead awareness), cass-ked (daily digest), bd-la5 (agents must not use raw SQL)
- Bead labels should be `area:cli` (not `area:global` or `area:robot` which don't exist in the taxonomy)

## Sources

- Bead: bt-kkql (full description has architecture pointers)
- Closest pattern: `robotBQLCmd` at `cmd/bt/cobra_robot.go:285-333`
- Global data layer: `internal/datasource/global_dolt.go`
- Robot envelope: `cmd/bt/robot_output.go:23-38`
- Issue model: `pkg/model/types.go:9-39`
- Status constants: `pkg/model/types.go:161-169` (open, in_progress, blocked, deferred, pinned, hooked, review, closed, tombstone)
- Existing filter: `cmd/bt/helpers.go:28` (`filterByRepo`)
