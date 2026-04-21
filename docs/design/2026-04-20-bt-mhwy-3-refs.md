# Design: `bt robot refs` — cross-project reference validation

<!-- Related: bt-mhwy.3, bt-mhwy (epic), bt-mhwy.1, bt-mhwy.2, bt-mhwy.4, bt-mhwy.5 -->
<!-- Schema: ref.v1 -->

- **Status**: Design complete, implementation in-session
- **Owner**: sms
- **Primary bead**: bt-mhwy.3
- **Prior art**:
  - `docs/design/2026-04-20-bt-mhwy-2-pairs.md` — closest precedent: cross-project
    projection + global-only subcommand + pure `pairsOutput()` helper pattern for
    testing against `BT_TEST_MODE=1` (mandatory because binary-level tests can't
    drive `--global` through Dolt discovery).
  - `docs/design/2026-04-20-bt-mhwy-5-external-dep-resolution.md` — establishes
    the `rc.analysisIssues()` composition rule followed here and the
    `external:<project>:<suffix>` dep shape. Resolved deps fold into the normal
    dep graph; unresolved ones are what refs detection surfaces as `broken`.
  - `docs/design/2026-04-20-bt-mhwy-1-compact-output.md` — `pkg/view/`
    projection pattern, schema versioning, golden harness.

## Problem

Beads reference other beads by ID in prose (description, notes, comments) and
in dependency fields. When the target lives in another project, no
single-project tool can validate the reference. Is `bd-la5` still open? Does
`cass-zzi` exist? Did a parent close and orphan its cross-project children?

Stale cross-refs rot silently. An agent reading a bead sees a reference to
`cass-xyz` and assumes it's current, when it may have been closed months ago.
`external:` dep refs can be resolved by `ResolveExternalDeps`, but unresolved
ones disappear silently (they get dropped by the resolver and logged at
debug). Prose refs don't even get that.

Global mode (`--global`) already loads every Dolt database's issues into one
slice. Reference validation is a pure scan over that set. The missing piece
is the subcommand that emits a stable, agent-readable projection over
detected refs with their validation flags.

## Decision summary

Ship `bt robot refs` as a new global-only subcommand that emits a flat array
of `view.RefRecord` objects — one per `(source, target, location)` tuple —
each carrying the validation flags.

The projection lives in `pkg/view/` following `pkg/view/doc.go`. Refs are
flagged **only when cross-project** (the referenced ID's prefix differs from
the source bead's prefix). This is the biggest scope tightening from the
AC: suffix collisions dominate intra-project false positives (dogfooded in
bt-mhwy.2 pairs v1, ~5× false positive rate), and intra-project refs are
already handled by the dep graph. "cross-project reference validation" is
the AC's stated purpose.

Flags cover `broken`, `stale`, `orphaned_child`, and `cross_project`.
Envelope `schema` is always `ref.v1`. No subcommand-specific flags for v1.
`--shape` is inherited but a no-op (records are compact-by-construction).

## Architecture

### Package layout

```
pkg/view/
├── ref_record.go                             NEW — RefRecord + ComputeRefRecords + scanner helpers
├── ref_record_test.go                        NEW — unit tests
├── projections_test.go                       amended — TestRefRecordGolden + TestRefRecordSchemaFileExists
├── schemas/ref_record.v1.json                NEW
└── testdata/
    ├── fixtures/ref_*.json                   NEW — 4 fixtures
    └── golden/ref_*.json                     NEW — 4 goldens

cmd/bt/
├── robot_refs.go                             NEW — runRefs() + pure refsOutput()
├── robot_refs_test.go                        NEW — setupRefsFixture + contract tests
├── cobra_robot.go                            amended — declare + register robotRefsCmd
└── robot_all_subcommands_test.go             amended — add "refs" to matrix

docs/design/
└── 2026-04-20-bt-mhwy-3-refs.md              NEW — this doc
```

### Dependency rule

`pkg/view/ref_record.go` imports `pkg/model` and `pkg/analysis` (for
`SplitID`). Does NOT import `cmd/bt`. Projections stay callable from any
consumer.

### `RefRecord` schema (v1)

```go
const RefRecordSchemaV1 = "ref.v1"

type RefRecord struct {
    Source   string   `json:"source"`     // bead ID containing the ref
    Target   string   `json:"target"`     // referenced ID (or external:... when unresolved)
    Location string   `json:"location"`   // "description"|"notes"|"comments"|"deps"
    Flags    []string `json:"flags"`
}
```

One record per `(Source, Target, Location)` tuple. Duplicate mentions
within a single location collapse to one record. Same target across
different locations emits multiple records. Deterministic ordering: sort by
`(Source, Target, Location)` ascending.

### Scope: cross-project only

Refs are flagged ONLY when the referenced ID's prefix differs from the
source bead's prefix. A `bt-abc` bead mentioning `bt-xyz` in prose is
intra-project — already handled by existing dep graphs.

Rationale: eliminates the largest false-positive class (suffix collisions
within a project) and matches the AC's stated purpose. Documented as an
AC interpretation, mirroring bt-mhwy.2's "title drift = exact string
equality" choice.

### Scan scope v1

- **Dependencies**: `external:<project>:<suffix>` form only (already
  structured via `ResolveExternalDeps`). Refs that resolved during
  `rc.analysisIssues()` produce NO `broken` flag (they resolved into
  normal dep edges). Unresolved `external:` deps produce `broken` at
  `location=deps`.
- **Prose**: `description`, `notes`, comment bodies (comment text joined
  for a single prose scan per source per location).
- **NOT scanned v1**: `design`, `acceptance_criteria` (doc-level content,
  different review cycle — deferred).
- **Code blocks inside prose**: treated as prose in v1. Markdown parsing
  is deferred; false positives from code-block content are acceptable.

### ID pattern

Word-boundary-aware regex followed by `analysis.SplitID` post-match
validation. URL spans are stripped BEFORE matching so
`https://github.com/foo-bar/baz` doesn't produce a `foo-bar` ref.

Excluded from emission:
- Same-prefix matches (the cross-project rule)
- **Unknown-prefix matches** (see "Prefix scoping" below)
- Malformed prefix/suffix (empty after SplitID)
- Matches inside URLs (stripped pre-match)

### Prefix scoping

Prose matches are scoped to prefixes present in the loaded issue set.
Tokens like `round-trip`, `per-issue`, `cross-project`, `batch-closing`
split into valid `(prefix, suffix)` but their "prefix" corresponds to no
known project, so they can't be validated and are dropped.

Rationale: on the real shared Dolt server the naive regex produced ~85%
false positives from English slugs. Scoping to `knownPrefixes` cut that
to a working rate (~2% residual FPR on dogfood). The tradeoff is that
refs to projects not loaded in the global view are invisible — if
`obs-xyz` appears in prose but no `obs-*` bead is loaded, it's dropped.
For v1 that's the correct behavior: we can only validate what we can
see.

Unresolved `external:<project>:<suffix>` deps bypass this filter because
the `external:` prefix is itself an unambiguous ref marker — those
always emit `broken` when unresolvable.

### Flags (fixed output order)

1. **`broken`** — target ID doesn't exist in the global set.
2. **`stale`** — target exists and is closed. Informational; caller
   decides severity.
3. **`orphaned_child`** — target has a `DepParentChild` edge to a closed
   parent but target itself is still open. Surfaces stranded children.
4. **`cross_project`** — always present on every emitted record under v1
   (which only emits cross-project refs). Kept explicit so v2 can relax
   the identity rule without changing the flag's meaning.

Fixed order for agent diff stability:
`["broken", "stale", "orphaned_child", "cross_project"]`.

### `--global` requirement

Mandatory. Without `--global`, refs detection is definitionally empty
(only one prefix in scope). Error cleanly:

```
Error: bt robot refs requires --global (cross-project ref validation needs cross-project data)
```

Exit code 1. With `--global` and no refs in the data, emit `"refs": []`
and exit 0.

### `--shape` no-op

Compact-by-construction. Envelope schema always `ref.v1`. Same precedent
as `pair.v1` and `portfolio.v1`. Documented in `Long` help.

### Computation pipeline (pure function in `pkg/view`)

```go
// ComputeRefRecords scans issues for cross-project bead references in
// deps, description, notes, and comments. Refs whose prefix matches the
// source's prefix are skipped (same-project refs are handled by the dep
// graph, not this subcommand). Word-boundary-aware ID regex plus
// analysis.SplitID validation; URL spans stripped before matching.
// Nil/empty inputs return nil.
func ComputeRefRecords(issues []model.Issue) []RefRecord
```

Algorithm:

1. Build `knownByID map[string]model.Issue` from the input slice.
2. Build `parentClosed map[string]bool` — target→true iff target has a
   `DepParentChild` edge to a closed parent.
3. For each source issue:
   - Scan unresolved `external:` deps (resolved external deps are gone
     after `rc.analysisIssues()`; only unresolved ones remain, and they
     necessarily target a missing cross-project ID → `broken`).
   - Scan description, notes, joined comments with the ID regex (after
     URL stripping). For each match, SplitID; skip same-prefix; compute
     flags against `knownByID`/`parentClosed`; dedup within (source,
     location).
4. Sort records by `(Source, Target, Location)` ascending.

### Subcommand handler (`cmd/bt/robot_refs.go`)

Mirrors `cmd/bt/robot_pairs.go` exactly. Pure `refsOutput()` helper
(mandatory because binary tests can't drive `--global` through
`BT_TEST_MODE=1`).

```go
func refsOutput(issues []model.Issue, dataHash string) any {
    refs := view.ComputeRefRecords(issues)
    if refs == nil {
        refs = []view.RefRecord{}
    }
    envelope := NewRobotEnvelope(dataHash)
    envelope.Schema = view.RefRecordSchemaV1
    return struct {
        RobotEnvelope
        Refs []view.RefRecord `json:"refs"`
    }{RobotEnvelope: envelope, Refs: refs}
}

func (rc *robotCtx) runRefs() {
    if !flagGlobal {
        fmt.Fprintln(os.Stderr,
            "Error: bt robot refs requires --global (cross-project ref validation needs cross-project data)")
        os.Exit(1)
    }
    output := refsOutput(rc.analysisIssues(), rc.dataHash)
    enc := rc.newEncoder()
    if err := enc.Encode(output); err != nil {
        fmt.Fprintf(os.Stderr, "Error encoding refs: %v\n", err)
        os.Exit(1)
    }
    os.Exit(0)
}
```

## Tests

### Unit (`pkg/view/ref_record_test.go`)

- `TestComputeRefRecords_Empty`, `_SamePrefix_Skipped`,
  `_CrossProject_Found`, `_Broken`, `_Stale`, `_OrphanedChild`,
  `_ExternalDepResolved`, `_ExternalDepBroken`, `_DedupWithinLocation`,
  `_MultipleLocations`, `_MalformedIDsSkipped`, `_URLsSkipped`,
  `_WordBoundaries`, `_DottedSuffix`, `_FlagOrder`, `_SortedOutput`,
  `TestRefRecord_SchemaConstant`.

### Golden (extends `projections_test.go`)

Harness `TestRefRecordGolden` filters `ref_*` fixtures. Regenerate with
`GENERATE_GOLDEN=1 go test ./pkg/view/`. Fixtures:

- `ref_empty.json` — no cross-project refs (single-prefix set)
- `ref_single_broken.json` — one broken cross-project ref in description
- `ref_mixed.json` — mix of broken, stale, resolved-external, orphaned_child
- `ref_external_deps.json` — `external:` refs exercising resolver interaction

Plus `TestRefRecordSchemaFileExists`.

### Contract (`cmd/bt/robot_refs_test.go`)

- `TestRobotRefs_RequiresGlobal` — binary test; exits 1 with expected
  stderr under `BT_TEST_MODE=1` (error fires before Dolt discovery).
- `TestRefsOutput_BasicEnvelope` — envelope shape + refs is array.
- `TestRefsOutput_SchemaIsRefV1`.
- `TestRefsOutput_CrossProjectOnly` — in-prefix refs excluded.
- `TestRefsOutput_FlagOrder` — fixed output order.
- `TestRefsOutput_EmptyReturnsArray` — no refs ⇒ `[]`.
- `TestRefsOutput_SortedOutput` — deterministic ordering.

### Flag-acceptance matrix

`robot_all_subcommands_test.go`:
`{"refs", []string{"robot", "refs", "--global"}}`.

## Verification

```bash
go test ./pkg/view/...
go test ./cmd/bt/...
go build ./cmd/bt/ && go install ./cmd/bt/

bt robot refs 2>&1                                                    # exit 1
bt robot refs --global | jq '.schema'                                 # "ref.v1"
bt robot refs --global | jq '.refs | length'
bt robot refs --global | jq '.refs[] | {source, target, location, flags}' | head -40
bt robot refs --global | jq '[.refs[] | select(.flags[] | contains("broken"))] | length'
bt robot refs --global | jq '[.refs[] | select(.flags[] | contains("orphaned_child"))] | length'
```

**Dogfood check before closing**: eyeball the output. If the false-positive
rate resembles pairs-v1 (~5×), the cross-project scope alone wasn't enough
— add a sigil requirement (markdown link / backticks / `ref:` keyword)
before shipping, or note it in the close body as v2 follow-up.

Pre-existing failure baseline (NOT caused by this change):
- `pkg/drift/` 3 failures tracked in bt-5e99
- `bt robot blocker-chain` nil panic tracked in bt-kt7x
- `tests/e2e` timeout on `TestExportPages_SQLiteDatabase` tracked in bt-eqro

## Design decisions & rationale

### Why cross-project only

Dogfooding bt-mhwy.2 on the real shared Dolt server showed pairs v1 had a
~5× false positive rate driven by intra-project suffix collisions. Naive
prose scanning is exposed to the same trap. The AC names "cross-project
reference validation" as the purpose. Dropping intra-project matches
eliminates the dominant false-positive class and stays faithful to the
AC's stated intent. Filed as bt-gkyn (pairs v2 intent-based identity)
for the relaxation path.

### Why `broken` for unresolved `external:` deps only (not raw `external:...`)

`rc.analysisIssues()` already resolves resolvable external deps into real
dep-graph edges. Only unresolved ones remain visible at scan time. Any
ref that made it through resolution is not "broken" from the dep
perspective. Scanning resolved edges as prose refs would double-count.

### Why `orphaned_child`

The `DepParentChild` relationship is first-class in the dep graph. Closed
parents with open cross-project children are a real hygiene signal —
someone finished the work but the mirror child never caught up. Same
information is in the dep graph, but refs surfaces it at the point the
ref appears, which is the consumer context.

### Why keep `cross_project` explicit even though v1 only emits cross-project

Relaxing the identity rule is a likely v2 change (e.g. if we add sigil
detection and want to surface intra-project refs with a different
confidence). Pre-populating the flag today means agents can filter on
`cross_project` today and get stable semantics when v2 adds
`same_project` or drops the v1 invariant.

### Why error on missing `--global`

Same precedent as pairs. Empty refs output is a valid "no stale refs"
signal under `--global`. Without `--global`, emitting `[]` silently
conflates "you forgot `--global`" with "clean bill of health." A clean
error disambiguates and teaches the flag.

### Why no markdown parsing v1

Stripping URL spans with a regex is crude but covers the most common
false positive (repo paths in URLs). Fuller markdown parsing (code
blocks, inline code, link text vs href) is a research direction. v1
ships a tool.

### Why dedup per (source, location) but emit per location

Two mentions of `bd-x` in one description are one ref from the reader's
perspective. Two mentions across description and notes are two refs
because the validation signal should surface wherever the reader is
looking. Dedup at a finer grain would hide "also referenced in notes,"
which is a useful signal.

## Deferred / out of scope

- **Markdown-aware parsing**: code blocks, inline code, link text.
- **Design / acceptance_criteria scanning**: doc-level content.
- **Fuzzy matching or `ref:` sigil detection**: v1 is a regex tool.
- **Suggested fixes**: surface, don't fix.
- **Non-global mode**: pair of `--global` and "no refs detectable" are
  the two pair states; non-global adds a third ambiguous state.
- **Intra-project refs**: add back if demand appears; bt-gkyn's v2
  identity work may enable safe re-inclusion.

## Open questions

None blocking. Design is implementable as written.

## Cross-project constellation

| Bead | Project | Relationship |
|---|---|---|
| **bt-mhwy.3** | bt | This work |
| **bt-mhwy.1** | bt | Depends on (shipped) — establishes `pkg/view/` + `--shape` |
| **bt-mhwy.5** | bt | Shipped — `rc.analysisIssues()` + unresolved-external semantics |
| **bt-mhwy.4** | bt | Shipped — portfolio template |
| **bt-mhwy.2** | bt | Shipped — pair projection + pure helper pattern |
| **bt-mhwy.6** | bt | Next in sequence — provenance `--source` filter |
| **bt-gkyn** | bt | Follow-up — intent-based identity (informs refs v2) |

## References

- Bead: `bd show bt-mhwy.3` (read the 2026-04-21 suffix-collisions comment)
- Epic: `bd show bt-mhwy`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-2-pairs.md`
- Shipped prerequisite: `docs/design/2026-04-20-bt-mhwy-5-external-dep-resolution.md`
- Memory: `project_pair_suffix_collisions.md` (why cross-project-only scope)
- Memory: `project_bt_test_mode_global_contract.md` (pure helper mandate)
- SplitID source: `pkg/analysis/external_resolution.go:102`
- Composition rule: `cmd/bt/robot_ctx.go:82-87`
- Cobra registration point: `cmd/bt/cobra_robot.go` (`init()` near line 1137)
- ADR: `docs/adr/002-stabilize-and-ship.md` — Stream 1
