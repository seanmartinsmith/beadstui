# Design: External dep resolution for cross-project analysis

<!-- Related: bt-mhwy.5, bt-mhwy (epic), bt-mhwy.1, bt-mhwy.2, bt-mhwy.3, bt-mhwy.4, bt-ph1z.5 -->
<!-- Cross-project: bd-la5 (no raw SQL), dotfiles-dth (cross-project dep linking rules) -->

- **Status**: Design complete, awaiting review
- **Owner**: sms
- **Origin**: bt-mhwy.1 session (2026-04-20); .5 scoped out as the highest-leverage next step in the epic
- **Primary bead**: bt-mhwy.5
- **Prior art**: `docs/design/2026-04-20-bt-mhwy-1-compact-output.md` (shape/flag architecture, pkg/view pattern)

## Problem

`bt`'s analysis engine (`pkg/analysis/`) is the most sophisticated piece in this codebase — graph centrality, articulation points, critical path, cycle detection, k-core, HITS, PageRank, blocker chains, causal chains, impact networks, duplicate detection, triage scoring, ETA forecasting, label health. All of it accepts `[]model.Issue` and operates on the `issue.Dependencies[].DependsOnID` edge set. None of it cares about project boundaries.

Global mode (`--global`) already produces a unified `[]model.Issue` from every Dolt database on the shared server. Same-prefix deps (`bt-xyz` pointing at `bt-abc`) resolve correctly. Bare cross-prefix deps (`bt-xyz` pointing at `bd-klm` when bd-klm exists in the global set) resolve correctly because `DependsOnID` is already a full ID.

What does NOT resolve today: deps of the form `external:<project>:<identifier>`. These live in `DependsOnID` (see `pkg/view/compact_issue_test.go:205` for a concrete example) and are how agents encode "this issue is blocked by something in another project." `analysis.NewAnalyzer` iterates `issue.Dependencies` at `pkg/analysis/graph.go:1276-1288`, looks up `dep.DependsOnID` in `idToNode`, and silently skips anything it can't match (line 1287). External deps fall into this silent-skip bucket because their `DependsOnID` is not a canonical issue ID.

Net effect: even with `--global`, cross-project graph analysis is blind to a specific and intentional class of edges. `robot insights --global` misses cross-project articulation points. `robot blocker-chain --global` stops at project boundaries. `robot impact-network --global` under-reports impact. `robot graph --global` emits an incomplete edge set. The data is loaded, the engine can handle it, the edges just aren't connected.

Ten-plus robot subcommands would produce meaningfully better cross-project output if this one gap were closed. And every future subcommand in the bt-mhwy epic (`pairs`, `refs`, `portfolio`) benefits from the same fix — their graph-derived counts become cross-project-aware for free.

## Decision summary

Add a pre-analysis resolver that transforms an `[]model.Issue` by rewriting `external:<project>:<identifier>` deps to point at the canonical resolved issue in the global set. The resolver lives in `pkg/analysis/` as a sibling to `NewAnalyzer`. Subcommands opt in via a `robotCtx` helper that returns resolver-applied issues when `--global` is active. Single-project mode is untouched. Unresolved external refs are logged at debug level and dropped. The rewrite is in-memory per analysis run — no DB mutation, no TUI-visible change.

The change is plumbing. No new subcommand. No new projection. No new schema version. Its leverage comes from applying to every analysis-consuming subcommand through one integration point.

## Architecture

### Package layout

```
pkg/analysis/
├── external_resolution.go                 NEW — resolver + dep rewriting
├── external_resolution_test.go            NEW — unit tests
├── testdata/external/                     NEW — fixtures for cross-project cases
│   ├── two_project_chain.json             bt blocked by external:cass:xyz, cass-xyz open
│   └── unresolved_external.json           external ref with no matching target + malformed forms
└── graph.go                               UNTOUCHED — resolver runs upstream of NewAnalyzer

cmd/bt/
├── robot_ctx.go                           amended — add analysisIssues() helper
└── robot_analysis.go, robot_graph.go,
    robot_*.go (10 subcommands)            amended — call rc.analysisIssues() instead
                                             of rc.issues when feeding the analyzer
```

### Why `pkg/analysis/` not a new package

External dep resolution is graph plumbing — it prepares the input the graph engine consumes. `pkg/view/` is output projections (wrong direction). `pkg/model/` is domain types (transformations don't belong there). A new `pkg/resolve/` package would be a premature boundary for a single-file concern that imports `pkg/model` and has no other consumers. Colocating with `NewAnalyzer` keeps related plumbing discoverable.

### Dependency rule

`pkg/analysis/external_resolution.go` imports `pkg/model` only. It does NOT import `cmd/bt`, `pkg/view`, or `pkg/ui`. This preserves the existing rule that `pkg/analysis` is consumer-agnostic.

### Resolver API

```go
// ResolveExternalDeps returns a copy of issues with external:<project>:<id>
// dependencies rewritten to point at the canonical issue ID resolved against
// the input slice. Unresolved external refs are dropped from the returned
// issues' Dependencies and logged at debug. The input slice and its issues
// are not mutated. Safe for nil and empty inputs. Idempotent: calling twice
// produces the same result as calling once.
//
// Intended to be called on the full global issue set immediately before
// constructing an Analyzer. Runs in O(n + d) over issues and total deps.
func ResolveExternalDeps(issues []model.Issue) []model.Issue
```

No method on `model.Issue`. Same argument as the `.1` spec's absence of `Issue.Compact()`: resolution requires the global set, and a method signature wouldn't make that dependency visible. Free function keeps the "this needs the full set" contract honest.

No stats variant in v1. Debug logging covers the diagnostic need for unresolved refs. A `ResolveExternalDepsWithStats` signature is trivially additive when a concrete caller (e.g., a `--show-resolution-stats` flag) surfaces a need.

### Canonical form

One supported form: `external:<project>:<identifier>` where `<project>` is the **ID prefix** (the short token at the start of every issue ID, e.g. `bt` from `bt-mhwy.5`). This is what agents type in bead IDs; it's the convention the bead and the epic use.

Not supported: two-part form (`external:<id>`) or SourceRepo-as-project (`external:beadstui:xyz`). The two-part form appears only in a single test fixture. SourceRepo-as-project is a convention the capability-**label** system uses (`pkg/ui/capability_test.go`); the dep system uses ID prefixes. If real data surfaces either, add it in a follow-up bead.

### Resolution algorithm

1. **Build the lookup**. One map: `byIDPrefix map[string]map[string]string` — prefix (everything before the first `-` in `Issue.ID`) → suffix → canonical ID.

2. **Clone issues**. Shallow-copy each issue. Deep-copy the `Dependencies` slice; reuse `*Dependency` pointers for non-external deps and allocate new ones only for rewrites. Never touch the caller's data.

3. **Walk each issue's deps**. For each dep where `DependsOnID` begins with `external:`:
   - Parse `external:<project>:<suffix>`. Any other shape (missing colon, trailing colon, empty segments, extra colons) → log at debug as malformed, drop.
   - Look up `byIDPrefix[project][suffix]`. Hit: allocate a new `*Dependency` with the rewritten `DependsOnID`, preserve all other fields (`IssueID`, `Type`, `CreatedAt`, `CreatedBy`). Miss: omit from the output slice's `Dependencies`.

4. **Preserve dep ordering**. Resolved and non-external deps keep their original relative order. Only unresolved externals are removed.

5. **Log unresolved**. After the walk, emit a single debug log line per issue with any unresolved external refs: `external dep resolution: dropped N refs from <issue.ID>: [<ref1>, <ref2>, ...]`. One line per issue keeps the signal scannable.

Runtime: O(n + d) where n = len(issues) and d = total deps. One pass. Memory: O(n) cloned issue structs plus O(d_external) new `*Dependency` allocations for the rewritten subset.

### Integration at the call site

`robot_ctx.go` gets one new helper, mirroring the `.1` pattern where `projectIssues()` reads the package-level `robotOutputShape` var directly (`cmd/bt/robot_ctx.go:55`). `robotCtx` has no `global` field today and doesn't need one — the helper reads `flagGlobal` from `cmd/bt/root.go:98` directly:

```go
// analysisIssues returns the issue slice to feed the analysis engine.
// In global mode, external:<project>:<id> deps are resolved against the
// global set before returning. In single-project mode, returns rc.issues
// unchanged.
//
// Composition rule: this is the SINGLE point that returns the graph-ready
// slice. Future preprocessing (label normalization, ID aliasing, etc.)
// composes INSIDE this function — it wraps the existing chain, it does not
// add a sibling rc.Xissues() helper. One pipeline, not N helpers.
func (rc *robotCtx) analysisIssues() []model.Issue {
    if !flagGlobal {
        return rc.issues
    }
    return analysis.ResolveExternalDeps(rc.issues)
}
```

Subcommands change one line:

```go
// before:
analyzer := analysis.NewAnalyzer(rc.issues)

// after:
analyzer := analysis.NewAnalyzer(rc.analysisIssues())
```

### Affected subcommands

Concrete inventory, confirmed via `rg "analysis.NewAnalyzer" cmd/bt/`:

| File | Line | Subcommand / handler |
|---|---|---|
| `robot_analysis.go` | 68 | `robot insights` (primary path) |
| `robot_analysis.go` | 181 | `robot plan` / `robot priority` (analysis-derived) |
| `robot_analysis.go` | 251 | `robot triage` (advanced insights) |
| `robot_alerts.go` | 22 | `robot alerts` |
| `robot_sprint.go` | 148 | `robot sprint` (list variant) |
| `robot_sprint.go` | 298 | `robot sprint` (show variant) |
| `cli_misc.go` | 83 | `robot next` path via CLI adapter |
| `cli_misc.go` | 167 | `robot` secondary analysis handler |
| `cli_misc.go` | 516 | `robot` secondary analysis handler |
| `cli_misc.go` | 554 | `robot` secondary analysis handler |

Each call site replaces `rc.issues` with `rc.analysisIssues()`. Ten mechanical one-line edits. `cli_misc.go` lines 432–433 use `analysis.NewSnapshot` / `NewSnapshotAt` (diff path) — these also consume `rc.issues`; extend `rc.analysisIssues()` usage there too so diff-based subcommands behave consistently under `--global`.

`cli_misc.go:515` is `runRobotGraph` — so `robot graph` is covered by the same mechanical swap (confirmed by grep during design).

**Out of scope for this bead** (different execution paths, not robot subcommands):
- `cli_baseline.go:16, 92` — baseline save/info; not invoked via `bt robot`
- `cobra_export.go:156` + `:166-172` and `pages.go:93` + `:105-111` — HTML export pipeline builds its own edge list by iterating `issue.Dependencies` directly; cross-project export visualization is bt-ph1z territory
- `profiling.go:25` — dev-only profiling harness

Robot subcommands with no `NewAnalyzer` call (untouched): `list`, `search`, `bql`, `schema`, `docs`, `help`, `recipes`, `history`, `diff`, `drift`, `labels`, `correlation`, `baseline`, `forecast`, `burndown`, `capacity`. Their output doesn't depend on graph traversal, so cross-project dep resolution is a no-op for them.

### Drop-vs-skip boundary check

The resolver drops unresolved external refs from `Dependencies` instead of leaving them for `NewAnalyzer` to silently skip. The behavior is only observable if some consumer iterates `issue.Dependencies` directly on the transformed slice. By design, `rc.analysisIssues()` feeds only into analysis entry points — the TUI uses `appCtx.issues` (separate path), export uses `exportIssues` (separate path), and `robot list` / compact projection receive `rc.issues` (untransformed). No consumer of the transformed slice iterates `Dependencies` outside the analysis engine. The drop-vs-skip claim holds.

### Gating

`flagGlobal` is the single gate. No new flag. No env var. The resolver is a side-effect-free transformation of in-memory data; enabling it in global mode is always correct because no other code reads the transformed slice.

### What the resolver does NOT do

- **Does not persist**. The rewritten `DependsOnID` exists only in the returned slice. The Dolt DB is untouched. No `bd update` is emitted.
- **Does not modify the TUI's view of data**. TUI loads via its own path (`appCtx.issues`) and does not call `ResolveExternalDeps`. Cross-project TUI treatment is bt-ph1z.5 scope.
- **Does not affect non-analysis robot output**. `robot list`, `robot search`, `robot bql` all see the raw slice. Compact projection counts (`blockers_count` etc.) for those subcommands still reflect the unresolved state, which is correct — those outputs are for scanning issues, not for graph analysis.
- **Does not attempt fuzzy matching**. No edit distance, no typo correction, no label-based capability matching. External refs resolve by prefix+suffix exact match or they don't resolve.
- **Does not touch `external:<project>:<capability>` LABELS**. Those are the capability system (`aggregateCapabilities` in `pkg/ui/helpers.go`), a parallel mechanism for project-level supply/demand. The resolver only touches `DependsOnID` strings. Overlap between the two forms is discussed in Open Questions below.

## Acceptance criteria

Copied from the bead with verification tactics:

- [ ] `bt robot insights --global` surfaces cross-project articulation points that were invisible before.
  - **Verify**: fixture with cross-project chain (bt→external:cass:xyz→cass-xyz→cass-abc). Pre-change: bt-xxx has no articulation-point significance. Post-change: bt-xxx or cass-xyz shows in articulation output.
- [ ] `bt robot blocker-chain --global <id>` follows chains across project boundaries.
  - **Verify**: `bt robot blocker-chain --global <bt-issue-with-external-dep>` returns a chain whose hop crosses project prefixes.
- [ ] `bt robot impact-network --global <id>` includes cross-project impact.
  - **Verify**: impact network JSON for the upstream cass-xyz includes bt-xxx as a downstream node.
- [ ] `bt robot graph --global` shows cross-project edges in JSON output.
  - **Verify**: edge list contains edges whose source and target have different `SourceRepo`.
- [ ] Unresolved `external:` deps logged at debug, not erroring the run.
  - **Verify**: fixture with `external:nonexistent:xyz`, run with log level at debug, confirm one debug line, zero error lines, exit code 0.
- [ ] Single-project mode (no `--global`) behavior unchanged.
  - **Verify**: byte-identical output regression test for every affected subcommand comparing pre-change and post-change in non-global mode.

Implicit criteria from design discipline:

- [ ] Caller input is not mutated — deep-copy assertion in unit test.
- [ ] Safe for nil and empty slices — returns nil / empty without panic.
- [ ] Global mode with zero external deps produces output structurally equal to single-project mode on the same data — guards the byte-identical promise against future resolver mutations.

## Testing strategy

### Organization

```
pkg/analysis/external_resolution_test.go   unit: resolution algorithm
pkg/analysis/testdata/external/            fixtures (JSON)
  two_project_chain.json                   bt-a blocked by external:cass:xyz, cass-xyz exists
  unresolved_external.json                 external ref with no matching target
cmd/bt/robot_external_resolution_test.go   integration: global-mode subcommand behavior
```

### Test categories

| Category | Target | Asserts |
|---|---|---|
| Unit — resolution | `ResolveExternalDeps` | resolved deps rewrite DependsOnID correctly, unresolved deps dropped, malformed drops, other deps untouched |
| Unit — immutability | `ResolveExternalDeps` | input slice unchanged after call (compare before/after with deep-equal) |
| Unit — nil/empty | `ResolveExternalDeps` | nil input returns nil, empty input returns empty, no panic |
| Unit — structural equivalence | `ResolveExternalDeps` on a fixture with zero `external:` deps | output is structurally equal to input (guards byte-identical promise) |
| Integration — insights | `bt robot insights --global` | cross-project articulation points present given a cross-project fixture |
| Integration — blocker-chain | `bt robot blocker-chain --global <id>` | chain crosses project prefix |
| Integration — impact-network | `bt robot impact-network --global <id>` | downstream includes cross-project issues |
| Integration — graph | `bt robot graph --global` | edge list includes cross-project edges |
| Regression — single-project | every affected subcommand | output byte-identical pre/post change when `--global` absent |
| Debug log | resolver call path | one debug line per issue with unresolved externals; zero error lines |

### CI gates

- Unit tests: **blocking**.
- Integration tests: **blocking** (they guard the acceptance criteria).
- Regression tests (single-project byte-identical): **blocking** — the explicit "single-project unchanged" acceptance criterion.
- Debug log assertions: **blocking** — the "no error, logs at debug" acceptance criterion.

### Fixture strategy

Two hand-authored JSON fixtures in `pkg/analysis/testdata/external/`. Format follows `pkg/view/testdata/fixtures/` shape from `.1`:

- **two_project_chain.json**: `bt-a` blocked by `external:cass:x`; `cass-x` exists. Pre-resolution: edge missing. Post-resolution: edge present. Covers the happy-path rewrite.
- **unresolved_external.json**: `bt-a` blocked by `external:cass:ghost`; `cass-ghost` does not exist. Also contains malformed refs (`external:`, `external::`, `external:bt:`) to exercise the drop path. Post-resolution: edges dropped, debug lines emitted.

Additional cross-project integration scenarios (diamonds, longer chains) can be added as fixtures if integration tests reveal gaps, but two fixtures are the floor.

## Deployment

Single PR, single commit. ~200 lines added across `pkg/analysis/external_resolution.go` (resolver + fixtures), `pkg/analysis/external_resolution_test.go` (unit tests), `cmd/bt/robot_ctx.go` (helper), ten one-line swaps across `robot_analysis.go`, `robot_alerts.go`, `robot_sprint.go`, and `cli_misc.go`, and an integration test file.

No bellwether-then-ripple staging. Every call site is the same mechanical one-line swap; heterogeneity is zero. If the resolver has a bug, every subcommand test catches it in one CI run. The `.1` bellwether pattern was load-bearing because shape-flag integration varied per output type — that's not the situation here.

No feature flag. No staged rollout. Pre-alpha project with a single active user; the acceptance criteria include byte-identical single-project behavior, so the blast radius is contained by construction.

## Prerequisites

- **bt-mhwy.1 (shipped 2026-04-20)** — compact output mode, `pkg/view/` established, `robotCtx.shape` field and `--shape` flag family in place. This work piggy-backs on the same `robotCtx` pattern.
- No upstream bd changes required. Resolution is a pure function over already-loaded data.

## Cross-project constellation

| Bead | Project | Relationship |
|---|---|---|
| **bt-mhwy.5** | bt | This work |
| **bt-mhwy.1** | bt | Depends on (shipped) — establishes robotCtx pattern |
| **bt-mhwy.2** | bt | Related — `robot pairs` benefits from graph-aware counts once .5 lands |
| **bt-mhwy.3** | bt | Related — `robot refs` complements .5 (refs scans text; .5 scans deps) |
| **bt-mhwy.4** | bt | Related — `robot portfolio` top_blocker becomes cross-project-aware |
| **bt-ph1z.5** | bt | Complementary — TUI-side treatment of cross-project deps (distinct scope) |
| **bd-la5** | beads | Philosophy — bt is the sanctioned cross-project query layer; .5 lives up to that contract |

### Division of concern

- **bt-mhwy.5 (here)** handles the in-memory graph layer for analysis consumers (robot subcommands, future programmatic consumers).
- **bt-ph1z.5** handles visual treatment in the TUI (nodes and edges styled to show cross-project origin). Different surface, same underlying phenomenon.
- **Upstream beads** needs no change. The `external:<project>:<id>` syntax is already a convention; beads stores it as a string in `DependsOnID`. Resolution is purely a bt-side concern.

## Design decisions & rationale

### Why the rewrite lives in `pkg/analysis/` and not at the datasource boundary

Two layers considered:

- **(A) Datasource layer**: apply `ResolveExternalDeps` immediately after `datasource.LoadFromSource` in `root.go` — one-time transformation, propagates to TUI and robot alike.
- **(B) Analysis boundary**: apply `ResolveExternalDeps` at each analysis-consuming call site via `rc.analysisIssues()`.

(B) wins because the bead explicitly scopes TUI treatment out (`bt-ph1z.5` owns that). If we transformed at the datasource boundary, the TUI would see rewritten deps — a behavior change outside this bead's scope and one that should be designed in ph1z.5 with TUI-specific affordances (edge styling, "this hop crossed a project" indicators). Keeping the rewrite at the analysis boundary confines the blast radius to analysis output, which is exactly where the acceptance criteria live.

### Why a helper on `robotCtx` and not inside `NewAnalyzer`

`NewAnalyzer` is a pure graph constructor and should stay that way. Pushing resolution inside it would couple the analysis engine to a cross-project convention, and would require `NewAnalyzer` to know whether it's being called in global or single-project mode. The `rc.analysisIssues()` helper keeps the coupling at the CLI layer where project-mode awareness belongs.

### Why drop unresolved deps instead of leaving them as-is

Current behavior leaves `external:ghost:xyz` in `issue.Dependencies`, and `NewAnalyzer` silently skips it. The net analysis result is identical whether we drop or leave. But dropping has two benefits:

1. **Visibility**: the resolver emits a debug log for every drop. Leaving as-is means unresolved externals are invisible in every run, forever.
2. **Downstream hygiene**: future consumers of `ResolveExternalDeps`'s output (not just `NewAnalyzer`) won't have to re-implement the "is this an `external:` prefix?" check.

### Why log at debug and not warn

Unresolved externals are a normal condition, not an error. External refs can point to beads that have been closed, compacted, moved, or that haven't been created yet. Warning-level logs would spam every run. Debug is the right level: zero output in normal operation, full diagnostic trail when someone opts in via `BT_LOG_LEVEL=debug` or similar.

If operational experience shows unresolved externals cluster around bugs (e.g., consistently unresolved refs might indicate typos), a future bead can add a `--strict-externals` flag that promotes them to warnings or errors. Not in scope here.

### Why not support capability-label resolution in the same pass

`external:<project>:<capability>` as a LABEL (existing `aggregateCapabilities` system in `pkg/ui/helpers.go`) is a project-level supply/demand model. `external:<project>:<identifier>` as a DEP (this bead's target) is an issue-level blocking relationship. They share syntactic prefix but express different semantics:

- Labels are a SET — "this project consumes capability X." Resolution is many-to-many via `provides:`/`export:` labels.
- Deps are a GRAPH EDGE — "this specific issue is blocked by that specific issue."

Conflating them would make the resolver responsible for two unrelated problems. Keep them separate. If a label-to-dep bridge becomes valuable (e.g., "rewrite `external:cass:search-quality` to depend on the highest-priority `provides:search-quality` issue in cass"), that's a new bead.

### Why not fuzzy match

`external:cass:seaarch-quality` (typo) should NOT resolve to `cass-search-quality`. Fuzzy matching silently fixes input and masks data quality issues. Exact match or drop. If typo correction becomes a product need, it's a separate (opt-in) feature.

### Why no opt-out flag

Resolution is gated on `--global` alone. A `BT_RESOLVE_EXTERNALS=false` opt-out speculates on a need that doesn't exist. The single-project byte-identical acceptance criterion already protects users who don't want cross-project semantics — they simply don't pass `--global`. Adding a flag is additive and cheap when a concrete use case surfaces.

### Why the resolver is dep-type-agnostic

The resolver runs on every dep with an `external:` prefix regardless of `DependencyType`. Related / discovered-from deps aren't graph edges (`IsGraphEdge()` filters them downstream in `types.go:292`), so resolving them is a no-op for analysis output — but keeping the resolver type-agnostic makes it a cleaner primitive. If a future consumer cares about non-graph deps, resolution is already in place.

### Why compacted / wisp targets resolve normally

A compacted (`CompactionLevel > 0`) or wisp (`Ephemeral == true`) target is still a valid graph node. Analysis already handles these cases uniformly. The downstream `IsBlocked` semantics in the `.1` compact projection already check `Status in {open, in_progress}`, so a closed-but-present compacted target correctly shows as not-blocking. No special case.

## Open questions

None blocking. The design is implementable as written.

## Deferred / out of scope

- **TUI treatment of cross-project edges** — `bt-ph1z.5`. Styling, hover, navigation across project boundaries in the graph view.
- **Persistence of resolved edges** — the rewrite is in-memory per run. Persisting resolved edges into Dolt is a different problem (`bd update`-style tooling), outside this bead.
- **Fixup tooling for broken externals** — `external:ghost:xyz` could trigger a suggestion flow ("did you mean cass-xyz?"). Separate bead if useful.
- **Capability-label to dep bridge** — `external:<project>:<capability>` labels staying labels vs being projected onto deps. Documented as intentional in Q1.
- **Cross-project analysis in single-project mode** — not supported. `--global` is required to load the other projects' data; without it, the resolver has no targets to resolve against.

## References

- Origin bead: `bd show bt-mhwy.5`
- Epic: `bd show bt-mhwy`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-1-compact-output.md`
- Analysis engine entry: `pkg/analysis/graph.go:1244` (`NewAnalyzer`), `:1272-1288` (dep iteration and silent skip of unknown targets)
- Capability label system (do not touch): `pkg/ui/helpers.go:154` (`parseCapabilities`), `:198` (`aggregateCapabilities`)
- Test data evidence of `external:` in deps: `pkg/view/compact_issue_test.go:205`
- Root flag: `cmd/bt/root.go:98` (`flagGlobal`)
- Global loading: `cmd/bt/root.go:195-208` (`datasource.LoadFromSource(globalSource)` populates `appCtx.issues`)
- ADR spine: `docs/adr/002-stabilize-and-ship.md` — Stream 1 (Robot-mode hardening)
