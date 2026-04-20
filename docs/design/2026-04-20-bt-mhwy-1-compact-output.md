# Design: Compact output mode for bt robot subcommands

<!-- Related: bt-mhwy.1, bt-mhwy.0, bt-uc6k, bt-wjzk, bt-mhwy (epic) -->
<!-- Cross-project: bd-34v, bd-fjip, mkt-fjip, bt-2cvx -->

- **Status**: Design complete, awaiting review
- **Owner**: sms
- **Brainstorm session**: 4c4046f0-1429-46eb-800f-abce77a44871 (bt workspace, 2026-04-20)
- **Primary bead**: bt-mhwy.1
- **Schema version**: `compact.v1`

## Problem

`bt robot list` returns ~184KB for 100 issues because every issue is serialized with full `Description`, `Design`, `AcceptanceCriteria`, `Notes`, and `Comments`. Agents scanning a list burn their context window on data they don't need. The same problem exists across ~17 robot subcommands that emit issue bodies. The epic framing (bt-mhwy) identifies this as the unblocker for the full cross-project intelligence layer — cross-project insights are useless if the output burns the agent's context.

The bt-kkql session (origin) confirmed agent workflows are `scan → drill → act`, not "try to avoid drilling." Excerpts in list output are a false middle ground; compact structure (index + relationships + timestamps) plus explicit drill-in via `bd show` matches how agents actually consume bead data.

## Decision summary

Ship compact output as the default across all 17 robot subcommands that emit issue bodies, with `--full` as an explicit escape hatch. Introduce a reusable projection pattern in a new `pkg/view/` package so `.2`/`.3`/`.4`/etc. follow the same conventions. Envelope carries a `schema` field for agent-facing version discovery. Deploy as a clean sweep in one PR with layered commits for reviewability.

## Architecture

### Package layout

```
pkg/view/                           NEW
├── doc.go                          projection pattern conventions
├── compact_issue.go                CompactIssue struct + CompactAll()
├── compact_issue_test.go           unit tests
├── projections_test.go             golden-file harness (reused for future projections)
├── schemas/
│   └── compact_issue.v1.json       JSON schema (committed)
└── testdata/
    ├── fixtures/*.json             input []model.Issue fixtures
    └── golden/*.json               expected projection outputs

cmd/bt/
├── robot_compact_flag.go           NEW — --shape / --format flag registration
└── robot_ctx.go                    amended — add shape field + projectIssues() helper

internal/datasource/                NOT TOUCHED in this bead
  (bt-mhwy.0 handles upstream schema catchup; separate scope)
```

### Why `pkg/view/` not `pkg/model/`

`CompactIssue` has `children_count`, `blockers_count`, `unblocks_count` — all derived from the dependency graph, not stored on the `Issue` row. Graph-derived projections belong with analysis/view concerns, not the data model. `pkg/model` stays canonical domain types only.

### Dependency rule

`pkg/view` may import `pkg/model` and `pkg/analysis`. It may NOT import `cmd/bt`. Projections must be usable from any consumer — CLI, TUI, WASM, tests.

## `CompactIssue` schema (v1)

| Field | Type | JSON tag | Required | Source |
|---|---|---|---|---|
| `ID` | string | `id` | ✓ | `Issue.ID` |
| `Title` | string | `title` | ✓ | `Issue.Title` |
| `Status` | string | `status` | ✓ | `Issue.Status` |
| `Priority` | int | `priority` | ✓ | `Issue.Priority` |
| `IssueType` | string | `issue_type` | ✓ | `Issue.IssueType` |
| `Labels` | []string | `labels,omitempty` | — | `Issue.Labels` |
| `Assignee` | string | `assignee,omitempty` | — | `Issue.Assignee` |
| `SourceRepo` | string | `source_repo,omitempty` | — | `Issue.SourceRepo` |
| `ParentID` | string | `parent_id,omitempty` | — | `Issue.Dependencies` filtered to type=parent-child |
| `BlockersCount` | int | `blockers_count` | ✓ | `Issue.Dependencies` filtered to type=blocks (incoming) |
| `UnblocksCount` | int | `unblocks_count` | ✓ | reverse map: count of issues blocked by this ID |
| `ChildrenCount` | int | `children_count` | ✓ | reverse map: count of issues where parent_id == this.ID |
| `RelatesCount` | int | `relates_count` | ✓ | `Issue.Dependencies` filtered to type=relates-to |
| `IsBlocked` | bool | `is_blocked` | ✓ | derived: any incoming blocker has status `open` or `in_progress` |
| `CreatedAt` | time.Time | `created_at` | ✓ | `Issue.CreatedAt` |
| `UpdatedAt` | time.Time | `updated_at` | ✓ | `Issue.UpdatedAt` |
| `DueDate` | *time.Time | `due_date,omitempty` | — | `Issue.DueDate` |
| `ClosedAt` | *time.Time | `closed_at,omitempty` | — | `Issue.ClosedAt` |
| `CreatedBySession` | string | `created_by_session,omitempty` | — | `metadata.created_by_session` (bridge, see below) |
| `ClaimedBySession` | string | `claimed_by_session,omitempty` | — | `metadata.claimed_by_session` (bridge) |
| `ClosedBySession` | string | `closed_by_session,omitempty` | — | `Issue.ClosedBySession` (first-class column, post-bt-mhwy.0) |

### Deliberately excluded from v1

- `Description`, `Design`, `AcceptanceCriteria`, `Notes`, `Comments`, `CloseReason` — fat fields; drill-in territory via `bd show`
- `EstimatedMinutes`, `ExternalRef`, `CompactionLevel`, `CompactedAt`, `CompactedAtCommit`, `OriginalSize` — low triage signal
- Gate fields (`AwaitType`, `AwaitID`, `TimeoutNs`) — add in v2 if gating becomes a triage signal
- Molecule fields (`MolType`, `Ephemeral`, `IsTemplate`) — same logic

Re-adding any of these is an additive change (`omitempty` field) — same `compact.v1`, no bump.

### Session ID: aggressive consumption, conservative upstream

Downstream (bt) consumes session IDs today via the metadata bridge; upstream (bd-34v) stays on its first-class-column roadmap. See cross-project constellation section below. `CreatedBySession` / `ClaimedBySession` read from `metadata.<name>` today. When bd-34v lands upstream, the read path switches to first-class columns with no schema change visible to agents (keys are the same).

### `IsBlocked` semantics

Not `blockers_count > 0` — a blocker that's closed doesn't block. Precise definition: *any incoming blocker has status `open` or `in_progress`*. Matches `bd ready`'s `●`/`○` glyph.

## Projection pattern conventions (`pkg/view/doc.go`)

Rules every projection in `pkg/view/` must follow:

1. **File per projection**: one struct, one file, named after the projection (e.g., `compact_issue.go`, `portfolio_record.go`)
2. **Struct + constructor**: struct with `json:"..."` tags and omitempty where appropriate; constructor function taking upstream types and returning the projection
3. **Schema version constant**: `const <Projection>SchemaV1 = "<name>.v1"` in the same file
4. **Versioning policy**:
   - Additive change (new `omitempty` field) → keep version
   - Rename / remove / type change → bump to v2
5. **Envelope integration**: subcommands set `RobotEnvelope.Schema = <constant>` when emitting the projection
6. **Test requirements** (enforced by `projections_test.go` golden-file harness):
   - Golden-file snapshot per fixture in `pkg/view/testdata/golden/`
   - JSON schema file committed at `pkg/view/schemas/<projection>.v1.json`
   - Round-trip test: marshal → validate against schema → assert pass
7. **No `pkg/model` placement**: projections are graph-derived, not domain types
8. **Dependency rule**: `pkg/view` may import `pkg/model` and `pkg/analysis`; not `cmd/bt`

Golden-file updates without a schema version bump are a red-flag signal in code review (not tooling-enforced — a semantic decision).

## Conversion API

```go
// CompactAll produces a compact projection for the full issue set.
// Precomputes reverse maps (children, unblocks) in one O(n) pass,
// then projects each issue in O(1). Safe for nil and empty inputs.
func CompactAll(issues []model.Issue) []CompactIssue

// CompactFrom is deliberately absent. Per-issue compaction requires
// reverse-graph data; a method on Issue would either be wrong or
// silently O(n²).
```

The lack of `Issue.Compact()` method is intentional. Reverse-map dependency must be visible in the signature.

## Flag architecture

Two orthogonal axes, persistent on the `robot` cobra command:

```
--shape   compact | full       (structural — which fields)
--format  json    | toon        (encoding — wire format)
```

| Flag | Env var | Default |
|---|---|---|
| `--shape` | `BT_OUTPUT_SHAPE` | `compact` |
| `--format` | `BT_OUTPUT_FORMAT` / `TOON_DEFAULT_FORMAT` (existing) | `json` |

**Aliases** (ergonomic shorthand): `--compact` → `--shape=compact`; `--full` → `--shape=full`.

**Env var naming rationale**: `BT_OUTPUT_SHAPE` parallels existing `BT_OUTPUT_FORMAT`. Scope-stable (doesn't assume robot mode is the only place shape matters — forward-compat for future WASM/HTTP surfaces). Agent-friendly (predictable pair when scanning env vars).

## Subcommand integration

### Touched (17 subcommands)

`list`, `triage`, `next`, `insights`, `plan`, `priority`, `alerts`, `search`, `suggest`, `diff`, `drift`, `blocker-chain`, `impact-network`, `causality`, `related`, `impact`, `orphans`

### Not touched (data is not `[]Issue` or is user-controlled)

`bql`, `schema`, `docs`, `help`, `recipes`, `labels`, `history`, `metrics`, `forecast`, `burndown`, `capacity`, `baseline`, `sprint`

### Change pattern per touched subcommand

Output struct's `Issues` field is typed as `any`. Before encoding, call helper on `robotCtx`:

```go
// robot_ctx.go
func (rc *robotCtx) projectIssues(issues []model.Issue) any {
    if rc.shape == "compact" {
        return view.CompactAll(issues)
    }
    return issues
}

// robot_ctx.go
func (rc *robotCtx) schemaFor(projection string) string {
    if rc.shape == "compact" {
        return view.CompactSchemaV1  // "compact.v1"
    }
    return ""  // full mode — empty string; envelope field is omitempty so omitted from JSON/TOON
}
```

Each subcommand calls `output.Issues = rc.projectIssues(filteredIssues)` and `output.Schema = rc.schemaFor("compact_issue")` before `enc.Encode(output)`.

### Multiple-issue-array outputs

Some subcommands (`insights`, `plan`) emit issues in multiple slots (top-k, critical path, articulation points). **Rule**: shape applies uniformly to every `[]Issue` slot in the same output. No partially-compact outputs.

### Non-`Issue` projections (v1 scope boundary)

`--shape` applies ONLY to `[]Issue` slots. `Alert`, `Correlation`, `Decision` types keep their current shape. If compact Alert/Correlation becomes valuable later, they get their own projection in `pkg/view/` and their own schema version. Documented in `pkg/view/doc.go`.

## Envelope shape (compact mode)

```json
{
  "generated_at": "2026-04-20T16:15:00Z",
  "data_hash": "ab12...",
  "output_format": "json",
  "version": "0.0.1",
  "schema": "compact.v1",
  "issues": [
    {
      "id": "bt-mhwy",
      "title": "Cross-project agent intelligence layer",
      "status": "open",
      "priority": 0,
      "issue_type": "epic",
      "labels": ["area:cli"],
      "parent_id": "bt-ushd",
      "blockers_count": 0,
      "unblocks_count": 0,
      "children_count": 7,
      "relates_count": 0,
      "is_blocked": false,
      "created_at": "2026-04-15T...",
      "updated_at": "2026-04-20T..."
    }
  ]
}
```

New envelope field: `schema`, JSON-tagged `schema,omitempty`. Absent in `--full` mode output (preserves byte-identical compat with pre-change). Present as `"compact.v1"` in `--shape=compact` mode. Absence in existing consumers' historical outputs is interpretable as "full, pre-schema-versioning."

## Testing strategy

### Organization

```
pkg/view/compact_issue_test.go      unit: CompactAll projection logic
pkg/view/projections_test.go        golden-file harness
pkg/view/testdata/
  fixtures/
    minimal_issue.json              bare fields only
    fully_populated_issue.json      every field set
    blocked_issue.json              has open blocker — is_blocked=true
    epic_with_children.json         tests children_count
    global_multiproject.json        cross-db fixture for global mode
  golden/
    compact_issue_minimal.json
    compact_issue_fully_populated.json
    ...

cmd/bt/robot_compact_flag_test.go   flag parsing, env var precedence
tests/e2e/robot_compact_test.go     e2e contract tests per subcommand
```

### Test categories

| Category | Target | Asserts |
|---|---|---|
| Unit (`CompactAll`) | 100% line coverage | field copying, reverse-map correctness, `is_blocked` semantics, nil/empty safety |
| Golden-file | 1 per fixture | output is byte-identical to committed golden |
| Contract | every touched subcommand | compact output has NONE of: `description`, `design`, `acceptance_criteria`, `notes`, `comments`, `close_reason` |
| Regression (`--full`) | every touched subcommand | byte-identical to pre-change output (envelope `schema` field is omitempty — absent in full mode) |
| Performance | benchmark, reported only | `CompactAll` scales linearly |
| Schema integration | 1 test | envelope `schema == "compact.v1"` when shape=compact; field omitted when shape=full |

### Migration of existing tests

Tests that assert full-shape output must pass `--full` explicitly. Tests that only parse output shape-agnostically pass `--full` to preserve current behavior. New compact assertions live in the new contract tests.

### CI gates

- Golden-file tests + contract tests: **blocking**
- Performance benchmark: **reported, not blocking**
- Schema version drift (golden changed without version bump): caught in code review, not tooling (documented in `pkg/view/doc.go`)

## Deployment

Clean sweep. One bead (bt-mhwy.1), one PR, one merge. Structured as layered commits:

1. `pkg/view/compact_issue.go` + `pkg/view/doc.go` + golden-file harness + tests (no behavior change)
2. `robot list` integration + flag registration (bellwether — validates pattern end-to-end)
3. Remaining 16 subcommands (mechanical, bulk)
4. CHANGELOG entry + docs update

Review walks commit-by-commit. If commit 2 reveals a `CompactAll` bug, fix it before commit 3 ripples out.

## Prerequisites

1. **bt-uc6k** (P2) — schema-drift audit. Produces report identifying all missing columns in bt's `IssuesColumns` vs upstream beads.
2. **bt-mhwy.0** (P0, blocked by bt-uc6k) — implements column catchup. Adds `metadata` JSON column read, `closed_by_session` first-class column, plus anything else the audit surfaces.

Both must land before bt-mhwy.1's session-ID fields are populable. The compact shape + flag architecture could ship without them, but session fields would be empty. Cleaner to ship all together.

## Cross-project constellation

| Bead | Project | Scope |
|---|---|---|
| **bt-mhwy.1** | bt | This design — compact projection + flags + subcommand integration |
| **bt-mhwy.0** | bt | Prerequisite column catchup (blocks .1) |
| **bt-uc6k** | bt | Schema-drift audit (blocks .0) |
| **bt-wjzk** | bt | Periodic drift detection (P3, follow-up) |
| **bt-mhwy.6** | bt | Provenance surfacing in compact output — related, shipped with .1's schema |
| **bt-2cvx** | bt | Session author provenance — TUI surfacing side of the same concern |
| **bd-34v** | beads | Upstream: extend `--session` to `bd create` + `bd update --claim` (per-event first-class columns) |
| **bd-fjip** | beads | Upstream storage: `session_history` support |
| **mkt-fjip** | marketplace | Harness: PreToolUse hook enforces `--set-metadata session_id=$CLAUDE_SESSION_ID` on bd create/claim |

### Division of concern

- **Upstream (bd-34v)** stays conservative: no metadata-bridge patterns in bd's codebase, first-class columns only
- **Harness (mkt-fjip)** stays aggressive: hook populates `metadata.created_by_session` and `claimed_by_session` on every create/claim using the exact naming convention upstream will adopt
- **Consumer (bt-mhwy.1)** stays aggressive: reads metadata keys today, transparent swap to first-class columns post-bd-34v-ship (no schema change visible to agents)

Risk is contained because we control both producer (hook) and consumer (bt). Other consumers of bd's metadata blob are upstream's concern.

## Design decisions & rationale

### Why compact-as-default (breaking change)

Pre-alpha project (v0.0.1), single active user, existing pattern of clean breaks (`BT_*` env rename, no `BV_*` fallback). `--full` provides the escape hatch; CHANGELOG documents the flip.

### Why not `MarshalJSON` method on `Issue`

Implicit global/context state, harder to reason about, TOON encoder may not respect custom marshalers identically. Explicit `CompactIssue` struct is greppable, testable, and makes the schema a documented artifact rather than an encoding side-effect.

### Why golden files, not full JSON schema generation

Proportional to the surface area. 4–5 projections across the epic doesn't justify reflection-based schema generation + dependencies. Golden files catch 80% of drift with `-update` as the explicit intent signal. Revisit at ~10 projections.

### Why `BT_OUTPUT_SHAPE` not `BT_SHAPE` or `BT_ROBOT_SHAPE`

Parallel to existing `BT_OUTPUT_FORMAT`. Scope-stable across future output surfaces (WASM, HTTP). Agent-friendly — two env vars with the same prefix tell the whole wire story.

### Why `any`-typed Issues field, not two explicit fields

`IssuesCompact []CompactIssue` + `IssuesFull []Issue` with shared JSON name causes ambiguity. `any` with projection at encode site is Go-idiomatic and centralized.

## Deferred / out of scope

- **Title-signal convention** (titles scan-dense for agent triage) — captured in note on bt-mhwy epic; candidate for paired beads across bt/beads/marketplace-beads-onboard
- **Compact projections for non-`Issue` types** — v2 scope if needed
- **Full JSON schema generation + enforcement** — follow-up bead at ~10 projections
- **Upstream schema drift detection automation** — bt-wjzk (P3)
- **Harness hook for session ID enforcement** — mkt-fjip

## Open questions

None blocking. The design is implementable as written. If bt-uc6k surfaces schema drift that affects projection field sources, this design may need a small update (which fields source from which columns), but the architecture is stable.

## References

- Brainstorm session: 4c4046f0-1429-46eb-800f-abce77a44871 (bt workspace, 2026-04-20)
- Origin context: bt-kkql session (shipped `bt robot list`, identified 184KB context-burn problem)
- Drill-through evidence: apr 18 bt-46p6 session (user instructed 11 sequential `bd show` calls for one epic tree)
- Upstream decision doc: `009-session-events-architecture.md` (Gas Town org, not in beads repo — referenced second-hand via commit `b362b3682`)
