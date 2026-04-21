# Cross-project convention (bt side)

> Pointer doc. The canonical convention lives in the marketplace repo at
> `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`.
> That doc governs the hook + agent-facing rules; this file describes how bt
> *reads* cross-project data.

## Reader surface (bt)

`bt robot pairs --global` and `bt robot refs --global` consume cross-project
graph data and surface it through versioned projections:

- `pair.v1` — suffix collision detector (v1). ~5× false-positive rate on real
  multi-project corpora because suffixes collide across unrelated work. Kept
  for one release as `--schema=v1`.
- `pair.v2` — intent-based. Requires a cross-prefix dep edge (`bd dep add`)
  between members of the paired group. Shipped in Phase 2 of bt-gkyn.
- `ref.v1` — prefix-scoping heuristic over prose. ~30% false-positive rate on
  broken-flag. Kept as `--schema=v1`.
- `ref.v2` — intent-based via syntactic sigils. Tunable mode:
  `--sigils=strict|verb|permissive` (default `verb`). Shipped in Phase 3 of
  bt-vxu9.

## Flags and env vars

- `--schema v1|v2` (pairs + refs) / `BT_OUTPUT_SCHEMA`. Default: **v2** as of
  Phase 3 of bt-vxu9 (both v2 readers live). `--schema=v1` remains as an
  opt-in fallback for one release.
- `--sigils strict|verb|permissive` (refs only) / `BT_SIGIL_MODE`. Requires
  `--schema=v2`; paired with `--schema=v1` errors with a clear resolution.
- `--orphaned` (pairs only, v1). Emits a JSONL checklist (stdout) + summary
  (stderr) of v1-detected pairs missing the cross-prefix dep edge v2 requires.
  Read-only — lists `bd dep add` commands for operator review.

## `--schema=v1` transition

v1 is retained for one release after v2 ships. Pin to it explicitly if a
consumer needs the frozen wire shape. A follow-up bead tracks removal.

## Forward-only backfill

Before the v2 default flips (post-Phase-1), every intentional cross-project
pair must have at least one cross-prefix dep edge. `bt robot pairs --global
--schema=v1 --orphaned` produces the checklist. Operator runs each
`bd dep add --type=related` manually — cross-project writes require human
authorization per global CLAUDE.md.

<!-- TODO(Phase 4 of bt-gkyn/bt-vxu9): expand this section with -->
<!-- labeling rubric pointer, per-sigil recognition vocabulary, -->
<!-- FPR thresholds, and rollback guidance. -->

## Cross-references

- Plan: `docs/plans/2026-04-21-feat-cross-project-intent-taxonomy-pairs-refs-v2-plan.md`
- Design: `docs/design/2026-04-21-pairs-refs-v2.md`
- Brainstorm: `docs/brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md`
- Beads: bt-gkyn (pairs v2 reader), bt-vxu9 (refs v2 reader), mkt-gkyn (hook), mkt-vxu9 (skill extension)
- Marketplace canonical skill: `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`
