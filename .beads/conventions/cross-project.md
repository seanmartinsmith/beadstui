# Cross-project convention (bt side)

> Pointer doc. The canonical convention lives in the marketplace repo at
> `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`.
> That doc governs the hook + agent-facing rules (when to pair, how to stamp
> deps, `--id` workflow). This file describes how **bt** *reads* cross-project
> data on the consumer side — the shape of the projections it emits, the
> flags that tune detection, and how to pin to older schemas.

## Reader surface (bt)

`bt robot pairs --global` and `bt robot refs --global` consume cross-project
graph data and surface it through versioned projections:

- `pair.v1` — suffix collision detector. Bucket issues by ID suffix, emit
  every bucket with ≥2 prefixes. ~5× false-positive rate on real multi-project
  corpora because suffixes collide across unrelated work. Kept for one release
  as `--schema=v1`.
- `pair.v2` — intent-based. Requires a cross-prefix dep edge (`bd dep add`)
  between members of the paired group. Connected-component BFS over the
  adjacency map; cycles and bidirectional edges collapse. Shipped in Phase 2
  of bt-gkyn.
- `ref.v1` — prefix-scoping heuristic over prose. ~30% false-positive rate on
  broken-flag. Kept as `--schema=v1`.
- `ref.v2` — intent-based via syntactic sigils. Mode-tunable:
  `--sigils=strict|verb|permissive` (default `verb`). Shipped in Phase 3 of
  bt-vxu9.

### Per-mode vocabulary (ref.v2)

| Mode | Recognizes |
|---|---|
| `strict` | markdown links where the link text is a valid bead ID, inline code (single backtick around ID), `ref:` / `refs:` keyword (case-insensitive) |
| `verb` | everything in `strict`, **plus** a fixed verb list (`see`, `paired with`, `blocks`, `closes`, `fixes`, `mirrors`) within 32-char same-line proximity preceding the ID. Markdown formatting (`**`, `_`) stripped before counting. Multiple verbs near one ID collapse to one record; closest verb wins and is recorded in `sigil_kind`. |
| `permissive` | v1's known-prefix scoping, **plus** fenced-code-block + inline-code exclusion. No sigil required — closest thing to v1 semantics while still suppressing the worst false-positive class. |

All three modes share the same tokenizer (fenced blocks, inline spans,
markdown links, plain prose) and the same 1MB per-body truncation cap —
oversized bodies emit `truncated: true` on every record produced from them.

See the [design doc](../../docs/design/2026-04-21-pairs-refs-v2.md) for the
full algorithm rationale, labeling rubric, and the FPR gate that anchors
these defaults.

## Data shape semantics

**`intent_source`** (pair.v2, per-record and envelope): the structural signal
that justified emission. Currently always the literal `"dep"` — the only v2
pair signal is a cross-prefix dep edge. Declared as an enum so future signals
(e.g., an upstream `paired_with` column, a notes-line fallback) can extend it
without breaking consumers that pattern-match on known values.

**`sigil_kind`** (ref.v2, per-record): the syntactic feature that triggered
the match. Values:

- `markdown_link` — `[bt-x](...)` in prose
- `inline_code` — `` `bt-x` `` in prose
- `ref_keyword` — `ref:` / `refs:` followed by a bead ID
- `verb` — one of the fixed verbs within proximity of the ID (verb mode only);
  the actual verb text is recorded for downstream filtering
- `external_dep` — dep edge across prefixes (consumed alongside refs when
  ref scanning sees a dep expressed in prose)
- `bare_dep` — dep edge same-prefix but referenced in prose without a sigil
- `bare_mention` — permissive-mode-only: a known-prefix ID appearing outside
  a sigil or code span

**`sigil_mode`** (ref.v2, envelope only): the mode the reader ran under.
Echoed so downstream consumers can see what filter level produced the output
without having to inspect per-record `sigil_kind` values.

## Flags and env vars

- `--schema v1|v2` (pairs + refs) / `BT_OUTPUT_SCHEMA`. Default: **v2** as of
  Phase 3 of bt-vxu9. `--schema=v1` remains as an opt-in fallback for one
  release; bt-xgba tracks removal.
- `--sigils strict|verb|permissive` (refs only) / `BT_SIGIL_MODE`. Requires
  `--schema=v2`. Combining with `--schema=v1` hard-errors with a resolution
  message pointing at `--schema=v2`.
- `--orphaned` (pairs only, v1). Emits a JSONL checklist (stdout) + summary
  (stderr) of v1-detected pairs missing the cross-prefix dep edge v2 requires.
  Read-only — it lists `bd dep add` commands for operator review. Used during
  migration and re-run any time a new paired bead is created without proper
  deps.

Precedence is flag > env > default. Invalid values (flag or env) exit 1 with
stderr listing the valid set.

## When to pin `--schema=v1`

v1 is retained for one release after v2 ships. Pin to it explicitly when:

- A frozen external consumer parses the v1 envelope shape and hasn't been
  updated. The v2 envelope adds `intent_source` / `sigil_mode` fields; the
  record shape also changes. Consumers that strictly validate unknown fields
  will break on v2.
- You're debugging a regression and want to compare v1 vs v2 output on the
  same corpus to isolate whether the diff is detection-layer or
  transport-layer.
- You're re-running `--orphaned` during a future backfill cycle (e.g., after
  someone creates a paired bead manually without proper deps). `--orphaned`
  is v1-only by design — it's the tool that migrates v1-visible pairs into
  v2-compliant ones.

## Invocation examples

Default (v2 everywhere):

```bash
bt robot pairs --global
bt robot refs --global                  # implicit --sigils=verb
```

Pin to v1:

```bash
bt robot pairs --global --schema=v1
bt robot refs --global --schema=v1
```

Tune ref sigil mode:

```bash
bt robot refs --global --sigils=strict
bt robot refs --global --sigils=verb
bt robot refs --global --sigils=permissive
```

Backfill helper:

```bash
bt robot pairs --global --schema=v1 --orphaned --robot-format=jsonl
# stdout: JSONL checklist of pairs missing dep edges
# stderr: summary count + reminder that `bd dep add` is a write
```

Env-var forms (useful in CI or when wrapping bt in another tool):

```bash
BT_OUTPUT_SCHEMA=v1 bt robot pairs --global
BT_SIGIL_MODE=strict bt robot refs --global
```

Flag beats env when both are set:

```bash
BT_SIGIL_MODE=strict bt robot refs --global --sigils=verb   # verb wins
```

## Migration guidance

The `--schema=v1` → `v2` default flip landed in Phase 3 of bt-vxu9. The
transition window is **one release**:

- **During the window:** v1 and v2 both work. Default is v2. Consumers that
  can't handle the v2 envelope yet should pin explicitly with `--schema=v1`
  or `BT_OUTPUT_SCHEMA=v1`.
- **After bt-xgba lands:** v1 is removed. Consumers must update or break.
  The `--orphaned` helper ships with v1 and will be removed alongside it;
  by that point every intentional pair should carry a dep edge and no
  backfill should be outstanding.

Backfill is a one-time operator task per paired group: run `--orphaned`,
review the suggested `bd dep add` commands, apply them manually
(cross-project writes require human authorization per global CLAUDE.md).
There is no automated apply mode and there won't be one.

## Cross-references

- Plan: `docs/plans/2026-04-21-feat-cross-project-intent-taxonomy-pairs-refs-v2.md`
- Design: `docs/design/2026-04-21-pairs-refs-v2.md` — algorithm rationale,
  labeling rubric, FPR gate
- Brainstorm: `docs/brainstorms/2026-04-21-cross-project-intent-taxonomy.md`
- Beads:
  - `bt-gkyn` — pairs v2 reader (closed after Phase 2)
  - `bt-vxu9` — refs v2 reader (open through Phase 6)
  - `bt-xgba` — `--schema=v1` removal (scheduled one release post-ship)
  - `mkt-gkyn` — marketplace hook (pair-stamping side)
  - `mkt-vxu9` — marketplace skill extension (sigil vocabulary)
  - `dotfiles-dth` — dotfiles-side cross-project convention expansion
- Marketplace canonical skill: `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`
