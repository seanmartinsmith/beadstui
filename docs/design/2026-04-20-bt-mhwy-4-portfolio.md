# Design: `bt robot portfolio` — per-project health aggregates

<!-- Related: bt-mhwy.4, bt-mhwy (epic), bt-mhwy.1, bt-mhwy.5, bt-ph1z, bt-8f34 -->
<!-- Schema: portfolio.v1 -->

- **Status**: Design complete, implementation underway (same session)
- **Owner**: sms
- **Primary bead**: bt-mhwy.4
- **Prior art**:
  - `docs/design/2026-04-20-bt-mhwy-1-compact-output.md` — projection pattern, `pkg/view/` conventions, schema versioning
  - `docs/design/2026-04-20-bt-mhwy-5-external-dep-resolution.md` — `rc.analysisIssues()` composition rule relied on here for cross-project blockers under `--global`

## Problem

No tool today answers **"which project needs attention?"** at the org level. `bd` and `bt` both operate per-project — you can see bt's health or cass's health, but not rank them side by side. Velocity, blocker ratios, P0/P1 counts, and stalest-issue age all exist per-project already but are not aggregated into a portfolio view. Agents working across projects need this to prioritize where to intervene.

The data already loads via `--global`, and `bt-mhwy.5` just made the graph cross-project-aware for analysis consumers. `robot portfolio` is the next step per the `bt-mhwy` epic's `5 → 4 → 2 → 3 → 6` sequencing — aggregate those per-project metrics into one flat record per project, sized for agent context.

## Decision summary

Ship `bt robot portfolio` as a new subcommand that emits a flat array of `view.PortfolioRecord` objects — one per project — carrying counts, priority breakdown, velocity with trend, composite health score, top blocker, and stalest issue. `--global` widens scope across projects; without it, a single record for the current project is returned (matches every other robot subcommand). The projection lives in `pkg/view/` following the `pkg/view/doc.go` pattern established by `CompactIssue`. Velocity delegates to the existing `analysis.ComputeProjectVelocity`. Top-blocker ranking uses PageRank from the analyzer's standard run. The reverse-edge pass from `CompactAll` is extracted into package-private helpers that both callers share.

No new flags for v1. `--shape` is inherited but effectively a no-op (records are compact-by-construction). Envelope `schema` field is always set to `portfolio.v1` because the payload IS a versioned projection.

## Architecture

### Package layout

```
pkg/view/
├── portfolio_record.go                    NEW — struct + ComputePortfolioRecord + helpers
├── portfolio_record_test.go               NEW — unit tests
├── compact_issue.go                       amended — extract shared reverse-map helpers
├── schemas/
│   └── portfolio_record.v1.json           NEW — JSON Schema
└── testdata/
    ├── fixtures/portfolio_*.json          NEW — 4 fixtures
    └── golden/portfolio_*.json            NEW — 4 golden outputs

cmd/bt/
├── robot_portfolio.go                     NEW — rc.runPortfolio() handler
├── robot_portfolio_test.go                NEW — contract tests
├── cobra_robot.go                         amended — register robotPortfolioCmd
└── robot_all_subcommands_test.go          amended — add `portfolio` to matrix
```

### Dependency rule

`pkg/view/portfolio_record.go` imports `pkg/model` and `pkg/analysis`, matching the existing `pkg/view/doc.go` rule. It does NOT import `cmd/bt`. Keeps projections callable from any consumer (CLI, TUI, WASM, tests).

### `PortfolioRecord` schema (v1)

```go
const PortfolioRecordSchemaV1 = "portfolio.v1"

type PortfolioRecord struct {
    Project     string              `json:"project"`
    Counts      PortfolioCounts     `json:"counts"`
    Priority    PortfolioPriority   `json:"priority"`
    Velocity    PortfolioVelocity   `json:"velocity"`
    HealthScore float64             `json:"health_score"`
    TopBlocker  *PortfolioBeadRef   `json:"top_blocker,omitempty"`
    Stalest     *PortfolioStaleRef  `json:"stalest,omitempty"`
}

type PortfolioCounts struct {
    Open       int `json:"open"`
    Blocked    int `json:"blocked"`      // IsBlocked semantics from compact_issue.go
    InProgress int `json:"in_progress"`
    Closed30d  int `json:"closed_30d"`
}

type PortfolioPriority struct {
    P0 int `json:"p0"`                   // open only
    P1 int `json:"p1"`                   // open only
}

type PortfolioVelocity struct {
    Closures7d  int    `json:"closures_7d"`
    Closures30d int    `json:"closures_30d"`
    Trend       string `json:"trend"`    // "up" | "down" | "flat"
    Estimated   bool   `json:"estimated,omitempty"`
}

type PortfolioBeadRef struct {
    ID       string  `json:"id"`
    Title    string  `json:"title"`
    Priority int     `json:"priority"`
    Score    float64 `json:"pagerank,omitempty"`
}

type PortfolioStaleRef struct {
    ID       string `json:"id"`
    Title    string `json:"title"`
    Priority int    `json:"priority"`
    AgeDays  int    `json:"age_days"`
}
```

Field selection matches the acceptance criteria on `bd show bt-mhwy.4`. `TopBlocker` and `Stalest` are pointers so projects with no candidates omit them from the wire.

### Computation pipeline

Entry point: `rc.runPortfolio()` in `cmd/bt/robot_portfolio.go`.

```
issues        := rc.analysisIssues()                 // cross-project resolved under --global
analyzer      := analysis.NewAnalyzer(issues)
stats         := analyzer.Analyze()                  // PageRank lives on stats
now           := time.Now().UTC()

// Group by project key.
groups := groupBySourceRepo(issues, flagGlobal, rc.repoName)

// Build a record per group.
records := make([]view.PortfolioRecord, 0, len(groups))
for project, projectIssues := range groups {
    records = append(records, view.ComputePortfolioRecord(
        project, projectIssues, issues, stats, now))
}

sort.Slice(records, func(i, j int) bool {
    return records[i].Project < records[j].Project
})
```

### `view.ComputePortfolioRecord(project, projectIssues, allIssues, stats, now)`

Pure function. `projectIssues` is already filtered to this project's `SourceRepo`. `allIssues` is the full set — needed because under `--global` a bt issue can be blocked by an open issue living outside `projectIssues` (resolved via bt-mhwy.5). Match compact_issue.go's `statusByID` pattern: look up blocker status in the full set. `stats` carries the PageRank map used for TopBlocker ranking, computed once over the global graph.

- **Counts**:
  - `Open` = count of `status == open`
  - `InProgress` = count of `status == in_progress`
  - `Blocked` = count of open/in_progress issues with any open/in_progress incoming blocker (shared helper; see "Shared reverse-map helpers" below)
  - `Closed30d` = count of closed issues where `closed_at >= now-30d` (fallback to `updated_at`, mark `Estimated=true` — mirrors `analysis.ComputeProjectVelocity`)

- **Priority** (open only):
  - `P0` = count of open issues with priority == 0
  - `P1` = count of open issues with priority == 1

- **Velocity**: delegate to `analysis.ComputeProjectVelocity(projectIssues, now, 8)`.
  - `Closures7d` = `v.ClosedLast7Days`
  - `Closures30d` = `v.ClosedLast30Days`
  - `Estimated` = `v.Estimated`
  - `Trend` — derived from `v.Weekly` (newest first, 8 entries). Compare a recent 2-week window against a prior 4-week window normalized to a 2-week equivalent, so one outlier week does not flip the trend:
    - `recent`     = `sum(Weekly[0..1])`       (last 2 weeks)
    - `priorTotal` = `sum(Weekly[2..5])`       (the 4 weeks before that)
    - `prior`      = `priorTotal / 2`          (scaled to 2-week equivalent)
    - `delta`      = `(recent - prior) / max(prior, 1)`
    - `"up"`       if `delta >= +0.20`
    - `"down"`     if `delta <= -0.20`
    - `"flat"`     otherwise (includes `prior == 0` case)

- **HealthScore** — equal-weight mean (user-confirmed formula):
  ```
  closure_ratio = closed30d / (closed30d + open)     // 1.0 if denominator == 0
  blocker_ratio = blocked / open                      // 0.0 if open == 0
  stale_norm    = min(stalest_age_days, 180) / 180    // 0.0 if no stalest
  score         = (closure_ratio + (1 - blocker_ratio) + (1 - stale_norm)) / 3
  ```
  Clamp final to `[0, 1]`. Round to 3 decimals on output.

- **TopBlocker** — PageRank over project-scoped open/in_progress issues that block ≥1 other issue. API: `stats.PageRank()` returns `map[string]float64` (`pkg/analysis/graph.go:594`). There is no `stats.UnblocksCount`; build the reverse map the same way `CompactAll` does via the shared helper `buildUnblocksMap(allIssues)`.
  ```
  unblocksBy  = buildUnblocksMap(allIssues)   // map[id]int
  pagerankAll = stats.PageRank()
  candidates  = {i in projectIssues
                 where i.Status in {open, in_progress}
                 and unblocksBy[i.ID] > 0}
  top         = argmax(candidates, key=pagerankAll[i.ID])
  ```
  Returns nil if no candidates.

- **Stalest** — open/in_progress issue with oldest `updated_at` in the project. nil if no candidates. `age_days = int((now - updated_at).Hours() / 24)`.

### Shared reverse-map helpers

`pkg/view/compact_issue.go:74-101` builds two logical reverse maps in one pass:
- `openBlockers[srcID]` — count of open/in_progress blockers pointing at `srcID` (for `IsBlocked`)
- `unblocksCount[targetID]` — count of issues blocked-by `targetID` (for `UnblocksCount`)

`PortfolioRecord` needs both: the Blocked count uses IsBlocked semantics, and TopBlocker filters by unblocks count. Duplicating the walk would rot. Extract two package-private helpers from the same loop:

```go
// buildOpenBlockersMap returns srcID -> count of its incoming blockers whose
// target is open or in_progress. Used by CompactAll and by PortfolioRecord for
// the Blocked count.
func buildOpenBlockersMap(issues []model.Issue) map[string]int

// buildUnblocksMap returns targetID -> count of issues blocked by targetID.
// Used by CompactAll for UnblocksCount and by PortfolioRecord for TopBlocker
// candidate filtering.
func buildUnblocksMap(issues []model.Issue) map[string]int
```

Both can share the statusByID pass. `CompactAll` is refactored to call these and preserve existing behavior; golden files must remain unchanged.

### Project grouping

```go
// In global mode, partition by SourceRepo (set on every issue by
// GlobalDoltReader from the Dolt database name). Empty SourceRepo maps to
// "unknown" — logged at debug but not dropped.
//
// In single-project mode everything is one group keyed by:
//   - rc.repoName if populated, else
//   - the uniform SourceRepo if all issues share one, else
//   - "local" fallback.
```

### Output envelope

```go
output := struct {
    RobotEnvelope
    Projects []view.PortfolioRecord `json:"projects"`
}{
    RobotEnvelope: NewRobotEnvelope(rc.dataHash),
    Projects:      records,
}
output.Schema = view.PortfolioRecordSchemaV1   // always "portfolio.v1"
enc.Encode(output)
```

`--shape` flag handling: the envelope `schema` field is set unconditionally because the payload IS a versioned projection — there is no full-mode alternate. Documented in the subcommand's `Long` help and in `pkg/view/doc.go` under the scope-boundary section.

### Cobra wiring

```go
var robotPortfolioCmd = &cobra.Command{
    Use:   "portfolio",
    Short: "Output per-project health aggregates as JSON",
    Long:  "Returns one record per project with counts, priority breakdown, velocity with trend, composite health score, top blocker, and stalest issue. Use --global to aggregate across all projects.",
    RunE: func(cmd *cobra.Command, args []string) error {
        rc, err := robotPreRun()
        if err != nil { return err }
        rc.runPortfolio()
        return nil
    },
}
// in init():
robotCmd.AddCommand(robotPortfolioCmd)
```

No subcommand-specific flags for v1. Callers use `--global`, `--bql`, etc. from `robotCmd` persistent flags.

## Tests

### Unit (`pkg/view/portfolio_record_test.go`)

- `TestComputePortfolioRecord_EmptyProject` — zero counts, nil TopBlocker/Stalest, health_score = 1.0 semantics at empty state.
- `TestComputePortfolioRecord_Counts` — verifies Open/Blocked/InProgress/Closed30d counts; P0/P1 breakdown counts only open issues.
- `TestComputePortfolioRecord_Velocity_Trend` — three subtests (up, down, flat) with known Weekly buckets; boundary cases at ±20%.
- `TestComputePortfolioRecord_HealthScore` — asserts formula against hand-computed values; clamping, divide-by-zero (open=0, closed30d=0), rounding.
- `TestComputePortfolioRecord_TopBlocker` — PageRank ranking with `unblocks_count > 0` filter; isolated-leaf beads excluded even with high PageRank.
- `TestComputePortfolioRecord_Stalest` — oldest `updated_at` among open wins; closed issues excluded; no-open-issues returns nil.
- `TestPortfolioRecord_SchemaConstant` — `PortfolioRecordSchemaV1 == "portfolio.v1"`.

### Golden (extends `pkg/view/projections_test.go`)

Four new fixtures + four matching golden files. The existing `TestCompactIssueGolden` harness iterates over all JSON fixtures; drop in with a sibling `TestPortfolioRecordGolden` that filters by the `portfolio_` prefix. Regenerate with `GENERATE_GOLDEN=1 go test ./pkg/view/`.

- `portfolio_empty.json` — no issues; output is an empty `[]`
- `portfolio_single_healthy.json` — one project, good health signals
- `portfolio_single_unhealthy.json` — one project, bad health (stale, blocked, slow)
- `portfolio_multi_project.json` — cross-project set with varying health

### Contract (`cmd/bt/robot_portfolio_test.go`)

- `TestRobotPortfolio_BasicEnvelope` — envelope shape and required fields.
- `TestRobotPortfolio_SchemaIsPortfolioV1` — `schema == "portfolio.v1"` regardless of `--shape`.
- `TestRobotPortfolio_ShapeFlagNoop` — `--shape=compact` and `--shape=full` produce the same bytes (no full-mode alternate).
- `TestRobotPortfolio_ProjectsSortedByName` — deterministic ordering.
- `TestRobotPortfolio_SingleProjectMode` — no `--global` = exactly one record.

### Flag-acceptance matrix

Extend `cmd/bt/robot_all_subcommands_test.go` with `{"portfolio", []string{"robot", "portfolio"}}` so the persistent `--shape`/`--compact`/`--full` flags are exercised against portfolio for free.

## Verification

```bash
go test ./pkg/view/...
go test ./cmd/bt/...
go build ./cmd/bt/ && go install ./cmd/bt/
bt robot portfolio --full | head -20
bt robot portfolio --global | head -40
bt robot portfolio --global | jq '.projects | length'
bt robot portfolio --global | jq '.projects[] | {project, health_score, counts}'
```

Pre-existing failure baseline (NOT caused by this change):
- `pkg/drift/` 3 failures tracked in bt-5e99
- `bt robot blocker-chain` nil panic tracked in bt-kt7x

## Design decisions & rationale

### Why equal-weight mean for HealthScore

Simplest formula satisfying "composite in [0..1]"; explainable to agents; no magic weights to tune. Ship v1 and iterate if smoke testing reveals signal issues. Reasoning recorded here so a future revisit knows the v1 starting point.

### Why TopBlocker filters by `unblocks_count > 0`

Literal AC reads "highest-centrality open issue" which would admit isolated-leaf beads with high PageRank. The unblocks filter matches the semantic intent: "beads holding other work hostage." PageRank is already computed in every analyzer run — no extra cost.

### Why `--shape` is a no-op here

`PortfolioRecord` is compact-by-construction — no body fields to strip. Setting `schema` unconditionally is correct because the payload IS a versioned projection. Distinct from `robot list`, where `--shape=full` returns raw `[]model.Issue` with fat bodies and the envelope's `schema` is omitted. Documented in the Long help and in `pkg/view/doc.go`.

### Why single-project mode emits exactly one record

Consistent with every other robot subcommand; `--global` widens scope but is not required. Enables single-project CI health checks without the caller plumbing a list-of-one unwrap.

### Why `"unknown"` bucket for empty SourceRepo in global mode

Losing data silently is worse than a visible "unknown" bucket. Logged at debug so noisy environments can investigate. Reversible if real-world usage shows the bucket is noise.

## Deferred / out of scope

- **Trend thresholds**: ±20% with 2-week/4-week-normalized comparison. Tighten to ±10% if smoke testing shows the band is too loose.
- **Stalest scope**: includes all open issue types. Epics naturally have older `updated_at` and may dominate every project's stalest slot. If that happens, exclude `issue_type == epic` in v2.
- **Cross-project rankings inside the output**: caller sorts/filters. Keeps the projection flat.
- **Dashboards or charts**: TUI/web territory, not this bead.
- **Historical trend beyond 30d**: v1 caps at 30d rolling window.

## Open questions

None blocking. Design is implementable as written.

## Cross-project constellation

| Bead | Project | Relationship |
|---|---|---|
| **bt-mhwy.4** | bt | This work |
| **bt-mhwy.1** | bt | Depends on (shipped) — establishes `pkg/view/` pattern and `--shape` flags |
| **bt-mhwy.5** | bt | Depends on (shipped) — `rc.analysisIssues()` gives cross-project graph under `--global`; portfolio's TopBlocker benefits for free |
| **bt-mhwy.2** | bt | Next in sequence — `robot pairs` |
| **bt-mhwy.3** | bt | Next in sequence — `robot refs` |
| **bt-mhwy.6** | bt | Next in sequence — provenance surfacing |
| **bt-ph1z** | bt | Parent epic concern — cross-project management gaps (user feedback from GH#3008) |
| **bt-8f34** | bt | Related — project registry; future work feeds authoritative project names into portfolio output |

## References

- Bead: `bd show bt-mhwy.4`
- Epic: `bd show bt-mhwy`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-1-compact-output.md`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-5-external-dep-resolution.md`
- Velocity: `pkg/analysis/triage.go:130-235` (`Velocity`, `VelocityWeek`, `ComputeProjectVelocity`)
- Compact projection prior art: `pkg/view/compact_issue.go:59-108` (reverse-map pass source)
- PageRank API: `pkg/analysis/graph.go:594` (`PageRank()` map), `:268` (`PageRankValue(id)`)
- Robot context: `cmd/bt/robot_ctx.go:82-87` (`rc.analysisIssues()`)
- Cobra registration point: `cmd/bt/cobra_robot.go:952+` (`init()`)
- ADR: `docs/adr/002-stabilize-and-ship.md` — Stream 1 (Robot-mode hardening)
