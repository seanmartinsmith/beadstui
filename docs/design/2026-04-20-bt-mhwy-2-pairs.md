# Design: `bt robot pairs` — cross-project paired bead detection

<!-- Related: bt-mhwy.2, bt-mhwy (epic), bt-mhwy.1, bt-mhwy.4, bt-mhwy.5, bt-6cfg -->
<!-- Schema: pair.v1 -->

- **Status**: Design complete, implementation underway (same session)
- **Owner**: sms
- **Primary bead**: bt-mhwy.2
- **Prior art**:
  - `docs/design/2026-04-20-bt-mhwy-1-compact-output.md` — projection pattern,
    `pkg/view/` conventions, schema versioning, the "Cross-project
    constellation" framing for paired suffixes
  - `docs/design/2026-04-20-bt-mhwy-4-portfolio.md` — closest precedent: a
    compact-by-construction projection plus a subcommand that groups over the
    global issue set
  - `docs/design/2026-04-20-bt-mhwy-5-external-dep-resolution.md` — establishes
    the `rc.analysisIssues()` composition rule followed here, and the
    "no-fuzzy-matching" philosophy applied to drift detection

## Problem

Agents can already *create* paired beads across projects with
`bd create --id=<prefix>-<suffix>` — the two memory entries
`feedback_cross_project_bead_pairing.md` and `feedback_cross_prefix_deps.md`
document this workflow. `bt-zsy8` and `bd-zsy8` then describe the same
logical work from each project's perspective.

Nothing surfaces the relationship after the fact. Given a mixed global-mode
issue set, there's no query that answers:

- Which ID suffixes show up in multiple projects?
- For each paired set, which bead was the canonical (first-created)?
- Have the pair's status, priority, or title diverged since creation?

The data is already available. Global mode (`--global`) already loads every
Dolt database's issues into one slice. Pair detection is pure suffix
matching on that set. The missing piece is the subcommand that emits a
stable, agent-readable projection over those pairs.

## Decision summary

Ship `bt robot pairs` as a new global-only subcommand that emits a flat
array of `view.PairRecord` objects — one per paired set — each carrying
the suffix, canonical bead, mirror beads, and drift flags. The projection
lives in `pkg/view/` following the `pkg/view/doc.go` pattern. Pair identity
is exact suffix match across distinct ID prefixes. Canonical is
first-created (`CreatedAt` ascending, tie-broken by prefix). Drift flags
cover `status`, `priority`, `closed_open`, and `title`.

The subcommand errors cleanly when invoked without `--global` (pair
detection is definitionally empty without cross-project data). Envelope
`schema` is always `pair.v1`. No subcommand-specific flags for v1.
`--shape` is inherited but effectively a no-op (records are
compact-by-construction).

This is the first cross-project projection in `pkg/view/` and validates the
pattern for `bt-mhwy.3` (refs).

## Architecture

### Package layout

```
pkg/analysis/
├── external_resolution.go                    amended — splitID → exported SplitID
├── external_resolution_test.go               amended — callsite rename

pkg/view/
├── pair_record.go                            NEW — struct + ComputePairRecords + helpers
├── pair_record_test.go                       NEW — unit tests
├── projections_test.go                       amended — new TestPairRecordGolden harness
├── schemas/
│   └── pair_record.v1.json                   NEW — JSON Schema
└── testdata/
    ├── fixtures/pair_*.json                  NEW — 4 fixtures
    └── golden/pair_*.json                    NEW — 4 golden outputs

cmd/bt/
├── robot_pairs.go                            NEW — rc.runPairs() handler
├── robot_pairs_test.go                       NEW — contract tests + setupPairsFixture
├── cobra_robot.go                            amended — register robotPairsCmd
└── robot_all_subcommands_test.go             amended — add `pairs` to matrix

docs/design/
└── 2026-04-20-bt-mhwy-2-pairs.md             NEW — this doc
```

### Dependency rule

`pkg/view/pair_record.go` imports `pkg/model` and `pkg/analysis` (for
`SplitID`), matching the existing `pkg/view/doc.go` rule. It does NOT
import `cmd/bt`. Projections stay callable from any consumer.

### `PairRecord` schema (v1)

```go
const PairRecordSchemaV1 = "pair.v1"

type PairRecord struct {
    Suffix    string       `json:"suffix"`
    Canonical PairMember   `json:"canonical"`
    Mirrors   []PairMember `json:"mirrors"`
    Drift     []string     `json:"drift,omitempty"`
}

type PairMember struct {
    ID         string       `json:"id"`
    Title      string       `json:"title"`
    Status     model.Status `json:"status"`
    Priority   int          `json:"priority"`
    SourceRepo string       `json:"source_repo,omitempty"`
}
```

One record per paired set. A 3-way pair (bt+bd+cass sharing the suffix
`zsy8`) emits one record with canonical + 2 mirrors. Field selection
matches the acceptance criteria on `bd show bt-mhwy.2`. `Drift` is
`omitempty` so in-sync pairs emit a clean envelope.

### Pair identity rule

Two issues pair when they share an **ID suffix** (everything after the
first `-`) AND have **different ID prefixes**. The split reuses
`analysis.SplitID` — promoted from the previously private `splitID`
because pair detection is a second legitimate consumer of the same
primitive. Duplicating the parse across packages risks the two definitions
drifting silently.

- `bt-zsy8` + `bd-zsy8` → pair on suffix `zsy8` ✓
- `bt-mhwy.2` + `bd-mhwy.2` → pair on suffix `mhwy.2` ✓ (dotted suffixes
  are just strings after the first hyphen)
- Two `bt-zsy8` rows from different `source_repo` values → **not** a pair
  (single-prefix bucket; treated as a data anomaly and dropped from
  output rather than crashing or silently collapsing)
- Groups of size 1 → dropped
- Groups where every member shares one prefix → dropped

### Canonical rule

**First-created wins.** Sort bucket by `CreatedAt` ascending; ties break
by prefix ascending, then by full ID. The head is canonical; the tail is
mirrors. Rationale: the cross-project workflow recorded in
`feedback_cross_project_bead_pairing.md` describes creating the bead
normally first, then using `--id` to mirror it into another project. The
earliest `CreatedAt` is the convention's originator — the source of truth
everyone else is mirroring.

Inside a suffix-bucket every member shares the suffix by construction, so
sorting on prefix after sorting on `CreatedAt` fully determines full-ID
order for the tie-break.

### Drift dimensions (v1)

`Drift` is a `[]string` comparing each mirror against the canonical. The
canonical is the source of truth; mirrors diverge from it. v1 dimensions,
in fixed output order:

1. **`"status"`** — any mirror's `Status` differs from canonical's `Status`.
2. **`"priority"`** — any mirror's `Priority` differs from canonical's
   `Priority`.
3. **`"closed_open"`** — canonical and at least one mirror straddle the
   closed / not-closed boundary (`Status == closed` vs anything else).
   Always co-occurs with `"status"` when it fires — it's a sharper
   sub-signal letting agents filter directly for "one side shipped while
   the other didn't" without reparsing statuses. Both flags appear; the
   presence of `"closed_open"` narrows the meaning of a `"status"` hit.
4. **`"title"`** — any mirror's `Title` differs from canonical's by exact
   string equality.

Flags appear in that fixed order so diffs between runs are stable. Empty
slice is omitted via `omitempty` tag.

### AC interpretation: "title significantly diverged"

The acceptance criterion says "title significantly diverged". v1
interprets this as **exact string inequality**. The "no fuzzy matching"
philosophy from `bt-mhwy.5` (external dep resolution) applies here: fuzzy
similarity thresholds introduce tunable knobs and false-positive
debugging, and v1 ships a tool, not a research project. If smoke testing
shows this is too noisy (e.g. trailing period changes firing drift),
bt-mhwy.2 follow-up can add a similarity threshold. Documented here so
reviewers know the call is intentional, not a shortcut.

### Deterministic ordering

- Records are sorted by `Suffix` ascending at the top level.
- Within a record, mirrors are sorted by `ID` prefix ascending (since all
  share the same suffix, prefix order fully determines ID order).
- Drift flags appear in the fixed order above.

Agents can diff two `pairs` runs byte-for-byte to see what changed.

### Computation pipeline (pure function in `pkg/view`)

```go
// ComputePairRecords groups issues by ID suffix, filters to groups with
// ≥2 distinct ID prefixes, and emits one record per group. Records are
// sorted by suffix. Nil/empty inputs return nil.
func ComputePairRecords(issues []model.Issue) []PairRecord
```

Algorithm:

1. Bucket issues by `suffix`: `map[string][]model.Issue`. Skip issues
   whose ID doesn't split (empty prefix or empty suffix).
2. For each bucket with ≥2 members, collect the distinct set of ID
   prefixes. Skip buckets with <2 distinct prefixes (handles the
   same-prefix-twice data anomaly).
3. Sort the bucket: `CreatedAt` asc, tie by prefix, tie by full ID. Head
   is canonical; tail is mirrors.
4. Compute `Drift` by comparing each mirror against canonical across the
   4 dimensions. Build the flag slice in fixed order.
5. Build the `PairRecord` with compact `PairMember` entries.
6. Sort records by suffix and return.

Runtime: O(n log n) overall. Bucketing is O(n). Per-bucket sort is
O(m log m) where m is bucket size (typically 2). Drift comparison is O(m)
per bucket. Final record sort dominates. Pairs are rare, so m is tiny in
practice.

### Subcommand handler (`cmd/bt/robot_pairs.go`)

```go
func (rc *robotCtx) runPairs() {
    if !flagGlobal {
        fmt.Fprintln(os.Stderr,
            "Error: bt robot pairs requires --global (pair detection needs cross-project data)")
        os.Exit(1)
    }

    issues := rc.analysisIssues()  // composition rule: single entry point
    pairs := view.ComputePairRecords(issues)

    envelope := NewRobotEnvelope(rc.dataHash)
    envelope.Schema = view.PairRecordSchemaV1

    output := struct {
        RobotEnvelope
        Pairs []view.PairRecord `json:"pairs"`
    }{
        RobotEnvelope: envelope,
        Pairs:         pairs,
    }

    enc := rc.newEncoder()
    if err := enc.Encode(output); err != nil {
        fmt.Fprintf(os.Stderr, "Error encoding pairs: %v\n", err)
        os.Exit(1)
    }
    os.Exit(0)
}
```

Following the `rc.analysisIssues()` composition rule established in
bt-mhwy.5: even though pair detection doesn't need external dep
resolution, the rule is "call this instead of `rc.issues`." Future
preprocessing composes inside `analysisIssues`, not alongside it.

### Why `--global` is mandatory

Without `--global`, the issue set is single-project; every issue shares a
single prefix, so pair detection is definitionally empty. We could emit
`"pairs": []` silently, but that collides with the legitimate "no pairs
exist under --global" signal — agents can't distinguish "you forgot
--global" from "nothing to report." Erroring cleanly disambiguates:

```
Error: bt robot pairs requires --global (pair detection needs cross-project data)
```

Exit code 1. With `--global` set and no paired sets in the data, emit a
success envelope with `"pairs": []` and exit 0 — satisfies the AC
literally ("Zero paired sets = empty array, not error").

### Why `--shape` is a no-op here

`PairRecord` is compact-by-construction — no body fields to strip (titles
are already part of the compact contract for members). Setting `schema`
unconditionally is correct because the payload IS a versioned projection.
Same pattern as `portfolio.v1`. Documented in the subcommand's `Long`
help and in `pkg/view/doc.go`.

### Cobra wiring

```go
var robotPairsCmd = &cobra.Command{
    Use:   "pairs",
    Short: "Detect cross-project paired beads (same ID suffix, different prefixes)",
    Long: "Returns one PairRecord per paired set — canonical bead (first-created) plus mirrors, " +
        "with drift flags for status, priority, closed/open, and title mismatches. " +
        "Requires --global because pair detection is inherently cross-project. " +
        "--shape is accepted but no-op; envelope.schema is always pair.v1.",
    RunE: func(cmd *cobra.Command, args []string) error {
        rc, err := robotPreRun()
        if err != nil { return err }
        rc.runPairs()
        return nil
    },
}
// in init():
robotCmd.AddCommand(robotPairsCmd)
```

### Why promote `splitID` to `SplitID`

`splitID` at `pkg/analysis/external_resolution.go:100` parses
`"bt-mhwy.5"` into `("bt", "mhwy.5", true)`. Pair detection needs the
same parse. Reimplementing it in `pkg/view` would mean two definitions of
"what is a valid bead ID" that can drift silently. Exporting costs
nothing: the private name is used in exactly one file (the resolver) plus
a test. Callsites update mechanically. The exported contract matches the
existing private one; the only change is the identifier.

## Tests

### Unit (`pkg/view/pair_record_test.go`)

- `TestComputePairRecords_Empty` — nil / zero-pair input returns nil.
- `TestComputePairRecords_SinglePair_InSync` — 2 members, identical
  status/priority/title → empty `Drift`.
- `TestComputePairRecords_SinglePair_DriftAllDimensions` — subtests for
  status drift, priority drift, closed_open drift, title drift.
- `TestComputePairRecords_ThreeWay` — 3 members (bt+bd+cass) sharing
  suffix; canonical by `CreatedAt`; 2 mirrors sorted by prefix.
- `TestComputePairRecords_CanonicalTieBreak` — identical `CreatedAt`;
  fallback to prefix ascending, then full ID.
- `TestComputePairRecords_SamePrefixDropped` — two `bt-x` from different
  source_repos → dropped (single-prefix bucket).
- `TestComputePairRecords_MalformedIDsSkipped` — IDs without hyphen or
  empty prefix/suffix are skipped silently.
- `TestComputePairRecords_DottedSuffix` — `bt-mhwy.2` + `bd-mhwy.2` pairs
  on `mhwy.2`.
- `TestPairRecord_SchemaConstant` — `PairRecordSchemaV1 == "pair.v1"`.

### Golden (extends `pkg/view/projections_test.go`)

Four new fixtures + four golden files. Harness `TestPairRecordGolden`
filters by `pair_` prefix (sibling of `TestPortfolioRecordGolden`).
Regenerate with `GENERATE_GOLDEN=1 go test ./pkg/view/`.

- `pair_empty.json` — no pairs.
- `pair_single_in_sync.json` — one pair, all fields equal.
- `pair_single_drifted.json` — one pair, one mirror drifted across 3
  dimensions.
- `pair_multi_way.json` — two pairs, one 3-way, one 2-way.

### JSON schema (`pkg/view/schemas/pair_record.v1.json`)

Committed JSON Schema describing the wire shape for external consumers.
`TestPairRecordSchemaFileExists` parallels the existing compact/portfolio
tests.

### Contract (`cmd/bt/robot_pairs_test.go`)

New `setupPairsFixture` (richer than the list fixture — needs 2+
prefixes) plus:

- `TestRobotPairs_RequiresGlobal` — no `--global` exits non-zero with the
  expected stderr message; `--global` succeeds.
- `TestRobotPairs_BasicEnvelope` — required envelope fields plus `pairs`
  is an array.
- `TestRobotPairs_SchemaIsPairV1` — `schema == "pair.v1"` across every
  `--shape` permutation.
- `TestRobotPairs_ShapeFlagNoop` — `--shape=compact` vs `--shape=full`
  produce byte-identical output (strip `generated_at`).
- `TestRobotPairs_DriftDetection` — fixture with known drift; asserts one
  pair surfaces with the expected drift flags in the expected order.
- `TestRobotPairs_EmptyReturnsArray` — fixture with no pairs emits
  `"pairs": []` and exit 0.
- `TestRobotPairs_PairsSortedBySuffix` — deterministic ordering.

### Flag-acceptance matrix

Extend `cmd/bt/robot_all_subcommands_test.go` with
`{"pairs", []string{"robot", "pairs", "--global"}}`. The matrix probes
flag parsing only; `--global` is carried because pairs requires it.

## Verification

```bash
go test ./pkg/analysis/...     # SplitID rename
go test ./pkg/view/...         # unit + golden
go test ./cmd/bt/...           # contract + flag matrix
go build ./cmd/bt/ && go install ./cmd/bt/

# sanity — no pairs mode
bt robot pairs 2>&1            # expect: Error: requires --global, exit 1

# sanity — with --global against real shared server
bt robot pairs --global | jq '.schema'                 # "pair.v1"
bt robot pairs --global | jq '.pairs | length'
bt robot pairs --global | jq '.pairs[] | {suffix, drift}' | head -40
bt robot pairs --global | jq '[.pairs[] | select(.drift)] | length'

# known pair from memory:
bt robot pairs --global | jq '.pairs[] | select(.suffix=="zsy8")'
```

Pre-existing failure baseline (NOT caused by this change):
- `pkg/drift/` 3 failures tracked in bt-5e99
- `bt robot blocker-chain` nil panic tracked in bt-kt7x

## Design decisions & rationale

### Why exact string equality for title drift

Listed under "AC interpretation" above. Short version: fuzzy matching
introduces knobs and false positives. v1 ships a tool.

### Why canonical = first-created

The cross-project workflow (memory: `feedback_cross_project_bead_pairing.md`)
creates one bead first, then uses `--id` to mirror it elsewhere. Earliest
`CreatedAt` is the originator. Tied timestamps are resolved by prefix for
determinism rather than picking one project as privileged.

### Why error on missing `--global` instead of silently emitting `[]`

Empty pairs output is a valid "no pairs exist" signal under `--global`.
Without `--global`, emitting `[]` silently makes those two cases
indistinguishable. A clean error disambiguates and teaches the flag.

### Why a separate `setupPairsFixture` instead of reusing `setupListFixture`

The list fixture is a small single-prefix project. Pairs tests need ≥2
prefixes by definition. Folding pair-ready data into the list fixture
would push every list test to think about cross-prefix setup. Separate
fixture keeps each contract surface focused.

### Why promote `splitID` rather than copy it

Single source of truth for "what is a valid bead ID split." Export costs
nothing — one private call site, one test. Duplicating the parse would
rot silently. Follows the immutability pattern in `ResolveExternalDeps`:
primitives that have a second legitimate consumer get exported, not
copied.

### Why no per-dimension granular drift (e.g. source_repo drift)

v1 covers the 4 dimensions called out in the AC. `source_repo` is
expected to differ by construction (the whole point of a mirrored pair
is that it lives in multiple repos). Label, description, owner, and
dependency-graph divergence all have legitimate cases but each would
expand drift semantics and flag ordering — deferred.

## Deferred / out of scope

- **Title similarity heuristics**: exact string equality only in v1.
- **Description / label / owner / dep-graph drift**: 4 dimensions in v1.
- **Dependency-graph linking between paired beads**: surfacing "these
  should have a `related` edge" is policy; bt-6cfg and `robot refs`
  (bt-mhwy.3) are the right places.
- **Auto-syncing pairs**: surface drift, don't fix it. Enforcement is
  higher-level policy.
- **Non-global mode**: mandatory `--global` is v1. Could emit per-project
  "would-be pairs if you ran --global" diagnostics later; no demand yet.
- **Canonical override flag**: if the first-created heuristic picks the
  wrong bead, add `--canonical-project=<prefix>`. Not needed v1.

## Open questions

None blocking. Design is implementable as written.

## Cross-project constellation

| Bead | Project | Relationship |
|---|---|---|
| **bt-mhwy.2** | bt | This work |
| **bt-mhwy.1** | bt | Depends on (shipped) — establishes `pkg/view/` pattern and `--shape` flags |
| **bt-mhwy.4** | bt | Shipped — compact-by-construction projection + subcommand template |
| **bt-mhwy.5** | bt | Shipped — `rc.analysisIssues()` composition rule; provides `SplitID` primitive |
| **bt-mhwy.3** | bt | Next in sequence — `robot refs` (cross-project reference resolution; consumes the pair pattern) |
| **bt-mhwy.6** | bt | Next in sequence — provenance surfacing |
| **bt-6cfg** | bt | Related — TUI linking of same-suffix beads (consumes `PairRecord` in future) |

## References

- Bead: `bd show bt-mhwy.2`
- Epic: `bd show bt-mhwy`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-1-compact-output.md`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-4-portfolio.md`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-5-external-dep-resolution.md`
- Memory: `feedback_cross_project_bead_pairing.md` (canonical = first-created rationale)
- Memory: `feedback_cross_prefix_deps.md` (cross-project dep semantics)
- SplitID source: `pkg/analysis/external_resolution.go:100`
- Projection conventions: `pkg/view/doc.go`
- Robot context composition rule: `cmd/bt/robot_ctx.go:82-87`
- Cobra registration point: `cmd/bt/cobra_robot.go` (`init()` near line 1116)
- ADR: `docs/adr/002-stabilize-and-ship.md` — Stream 1 (Robot-mode hardening)
