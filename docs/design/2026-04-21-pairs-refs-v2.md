# pairs v2 + refs v2 — implementation design

> **Status:** Phase 1 skeleton. Expanded in Phase 4 of bt-gkyn/bt-vxu9.
> Plan: `docs/plans/2026-04-21-feat-cross-project-intent-taxonomy-pairs-refs-v2-plan.md`.

## Scope

Shipping v2 readers for `bt robot pairs` and `bt robot refs`. v1 surfaces
string-pattern matches; v2 requires structural (pairs: dep edge) or syntactic
(refs: sigils) intent before emitting a record. This doc captures the
implementation-level decisions that don't fit in the plan.

## Projection schemas

- `pkg/view/schemas/pair_record.v1.json` — v1, frozen
- `pkg/view/schemas/pair_record.v2.json` — v2, adds `intent_source` per record
  (values: `"dep"`); envelope adds top-level `intent_source`
- `pkg/view/schemas/ref_record.v1.json` — v1, frozen
- `pkg/view/schemas/ref_record.v2.json` — v2, adds `sigil_kind` per record;
  envelope adds `sigil_mode`

Drift dimensions in pair.v2 drop `title` (confirmed no-signal on dogfood
corpus — bead titles drift legitimately across projects).

## Dispatch

`pairsOutput` and `refsOutput` receive a resolved schema version and inline-
dispatch to v1 vs v2 helpers. No generic `DispatchSchema[T]` helper — two
call sites don't justify a generic. Phase 1 decision, locked.

## Flag resolution

- `resolveSchemaVersion(flag)` — enum v1|v2, env `BT_OUTPUT_SCHEMA`, default
  v1 in Phase 1 (flips to v2 in Phase 2).
- `resolveSigilsMode(flag)` — enum strict|verb|permissive, env
  `BT_SIGIL_MODE`, default verb.
- Validation happens in cobra's `RunE` before `robotPreRun` so flag errors
  surface without loading data. Contract tests run under `BT_TEST_MODE=1`
  and observe the error without tripping Dolt discovery.

## Pair detection algorithm (Phase 2)

1. Bucket issues by ID suffix (`analysis.SplitID`).
2. For each bucket with ≥2 distinct prefixes, build an undirected adjacency
   map of cross-prefix dep edges. Dep type irrelevant — any edge counts.
3. Compute connected components via BFS.
4. Each component with ≥2 distinct prefixes → one `PairRecordV2`. Cycles and
   bidirectional edges fine.
5. Canonical = first-created (unchanged from v1). Mirrors sorted by prefix.
6. Drift flags against canonical, minus `title`.

<!-- TODO(Phase 2 of bt-gkyn): flesh out edge-case handling -->
<!-- (dangling deps, components with all members sharing a prefix, -->
<!-- partial-component drift). -->

## Sigil detection (Phase 3)

Hand-rolled iterative tokenizer in `pkg/analysis/sigils.go`. Bounded stack
(32 frames), 1MB per-body cap with `truncated: true` flag, panic-recover
wrapper at the call site.

Per-mode recognizers:

- `strict` — markdown link, inline code, `ref:`/`refs:` keyword
- `verb` — strict + fixed verb list (`see`, `paired with`, `blocks`, `closes`,
  `fixes`, `mirrors`) with 32-char same-line proximity
- `permissive` — v1 prefix-scoping + fenced-code/inline-code exclusion

<!-- TODO(Phase 3 of bt-vxu9): expand with benchmarks, pathological -->
<!-- input tests, and mode comparison on labeled corpus. -->

## Labeled corpus + FPR gate (Phase 5)

`pkg/view/testdata/corpus/labeled_corpus.json` — ≥30 sanitized real issues
from the shared Dolt server with truth labels. Pre-commit denylist scan
(passwords, tokens, secrets, URLs, emails outside allowed list) as a commit
gate.

Thresholds:
- Pair FPR <10% (requires N≥10 candidate pairs)
- Ref broken-flag FPR ≤5% under verb mode (strict/permissive informational)
- Memory delta <10MB for corpus load

### Labeling rubric

- A pair is `intentional: true` iff a human read both beads' descriptions and
  determined they describe the same logical work across projects.
- A ref is `intentional: true` iff the author clearly intended to reference
  the target bead. Placeholder text (`bt-xxx`) and English slugs (`-only`,
  `-side`) are `false`.
- Disputes resolved via comment on the bead itself.

### Sanitization script

<!-- TODO(Phase 5 of bt-gkyn): implement scripts/audit-corpus.sh with -->
<!-- regex denylist: password|secret|token|api_key| -->
<!-- AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{36}|xox[bp]-[A-Za-z0-9-]+| -->
<!-- \.env|localhost:[0-9]+|C:\\Users\\[a-z]+ -->
<!-- Scan description, notes, comments, close_reason, dependencies. -->
<!-- Fail pre-commit on hit; security label is fallback, not primary. -->

## Open items

- [ ] Phase 2: ship `ComputePairRecordsV2`, flip default to v2
- [ ] Phase 3: ship `pkg/analysis/sigils.go` + `ComputeRefRecordsV2`, flip default
- [ ] Phase 4: expand the TODO sections in this doc and the convention pointer
- [ ] Phase 5: corpus + FPR gate
- [ ] Post-ship: `--explain-refs` observability (new bead), upstream
  `paired_with` column PR (new upstream bead, after dogfood)

## References

- Plan: `docs/plans/2026-04-21-feat-cross-project-intent-taxonomy-pairs-refs-v2-plan.md`
- Brainstorm: `docs/brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md`
- v1 pairs design: `docs/design/2026-04-20-bt-mhwy-2-pairs.md`
- v1 refs design: `docs/design/2026-04-20-bt-mhwy-3-refs.md`
