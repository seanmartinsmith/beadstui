# pairs v2 + refs v2 — implementation design

> **Status:** Active (Phase 4 consolidation). Phases 2–3 shipped; Phase 5 (corpus + FPR gate) outstanding.
> Plan: `docs/plans/2026-04-21-feat-cross-project-intent-taxonomy-pairs-refs-v2-plan.md`.
> Brainstorm: `docs/brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md`.
> Convention pointer: `.beads/conventions/cross-project.md`.

## Scope

Shipping v2 readers for `bt robot pairs` and `bt robot refs`. v1 surfaces
string-pattern matches; v2 requires structural (pairs: dep edge) or syntactic
(refs: sigils) intent before emitting a record. This doc captures the
implementation-level decisions that don't fit in the plan — rationale,
invariants, and the labeling rubric Phase 5 consumes.

The plan is authoritative on what ships and when. This doc consolidates the
decisions inside the shipped code, names their anchor points, and hosts the
artifacts Phase 5 depends on.

## Projection schemas

- `pkg/view/schemas/pair_record.v1.json` — v1, frozen
- `pkg/view/schemas/pair_record.v2.json` — v2, adds `intent_source` per record
  (values: `"dep"`); envelope adds top-level `intent_source`
- `pkg/view/schemas/ref_record.v1.json` — v1, frozen
- `pkg/view/schemas/ref_record.v2.json` — v2, adds `sigil_kind` per record;
  envelope adds `sigil_mode`

Drift dimensions in pair.v2 drop `title` (confirmed no-signal on dogfood
corpus — bead titles drift legitimately across projects).

### Why schema bump, not envelope-additive

`--schema=v1` and `--schema=v2` are distinct wire contracts, not an
envelope-flag-gated extension. The semantic change (which records emit, which
records suppress) is large enough that a consumer built against v1 will see
different counts under v2 with the same data. Hiding that behind an optional
envelope field would let strict consumers silently drift. The schema bump is
the load-bearing signal: "this is not a superset — pin if you depend on the
old shape." The `--schema=v1` fallback is retained for one release as a
migration ramp, not as an ongoing compatibility mode.

## Dispatch

`pairsOutput` and `refsOutput` receive a resolved schema version and inline-
dispatch to v1 vs v2 helpers. No generic `DispatchSchema[T]` helper — two
call sites don't justify a generic. Phase 1 decision, locked.

## Flag resolution

- `resolveSchemaVersion(flag)` — enum v1|v2, env `BT_OUTPUT_SCHEMA`, default
  v2 (flipped in Phase 3 when the v2 refs reader landed alongside v2 pairs).
- `resolveSigilsMode(flag)` — enum strict|verb|permissive, env
  `BT_SIGIL_MODE`, default verb.
- Validation happens in cobra's `RunE` before `robotPreRun` so flag errors
  surface without loading data. Contract tests run under `BT_TEST_MODE=1`
  and observe the error without tripping Dolt discovery.

## Pair detection algorithm (Phase 2, shipped)

Shipped in `ComputePairRecordsV2` at `pkg/view/pair_record.go:256-343`.

1. Bucket issues by ID suffix (`analysis.SplitID`).
2. For each bucket with ≥2 distinct prefixes, build an undirected adjacency
   map of cross-prefix dep edges. Dep type irrelevant — any edge counts.
3. Compute connected components via BFS (`bfsComponents` at
   `pkg/view/pair_record.go:349-379`).
4. Each component with ≥2 distinct prefixes → one `PairRecordV2`. Cycles and
   bidirectional edges fine.
5. Canonical = first-created (unchanged from v1). Mirrors sorted by prefix.
6. Drift flags against canonical, minus `title` (`computeDriftV2` at
   `pkg/view/pair_record.go:421-449`).

### Why BFS, not union-find

Union-find is the textbook answer for connected components, but at projected
scale (N≈400 issues, per-bucket component size typically 2–4) it buys
nothing. A plain BFS with a visited map is ~20 LOC, reads top-to-bottom, and
is cheap to reason about in review. Union-find adds rank/path-compression
plumbing that pays off above maybe N=10⁴. The performance review flagged
this explicitly; readability won.

### Shipped edge cases

- **Bidirectional deps collapse.** If `bt-x blocks bd-x` AND `bd-x blocks
  bt-x`, the adjacency map records one undirected edge `{bt-x, bd-x}` — see
  the unconditional both-directions set at `pkg/view/pair_record.go:308-309`.
  One record emitted.
- **3-way cycle = one record.** `bt-x → bd-x → cass-x → bt-x` is one
  connected component under BFS → one `PairRecordV2`.
- **Dangling dep skipped silently.** If an issue's dep points at an ID not in
  the bucket (different suffix, missing issue, bead deletion race), the edge
  is skipped — `pkg/view/pair_record.go:301` (`!inBucket[otherIdx]`). Matches
  v1 tolerance; matches the "target resolution fails → record dropped"
  pattern in refs.
- **All-same-prefix bucket dropped.** A bucket of two `bt-x` issues with
  different `source_repo` is a data anomaly, not a pair. `hasMultiplePrefixes`
  at `pkg/view/pair_record.go:117-135` filters it out, and the check runs
  again on each BFS component at `pkg/view/pair_record.go:322` so a component
  that collapses to one prefix after edge filtering is also dropped.
- **Multiple components per bucket emit stably.** A bucket with two
  disconnected cross-prefix groups sharing a suffix (rare but possible if
  suffix reuse collides with intentional pairing) emits two records, sorted
  by `(suffix, canonical ID)` — `pkg/view/pair_record.go:333-338`.

### Why dep-edge is the sole pair signal

The brainstorm considered an OR-channel: dep edge OR `Paired-With:` notes
line. The reader would fall back to the notes line when the dep was absent.
This was rejected (plan Alternatives Considered, line 464):

- **Hides hook-stamping failures.** If the `mkt-gkyn` hook breaks and stops
  writing dep edges, a notes-line fallback keeps the reader green. That's a
  bug being masked.
- **Adds parser surface** for a channel that's already redundant when the
  hook works.
- **Operator coherence.** A single signal is easier to reason about: if the
  dep exists, it's a pair; if not, run `--orphaned` and add it. Two signals
  means two failure modes and a rule-of-precedence decision for every
  operator.

The `Paired-With:` notes line remains an output-only provenance marker (the
hook writes it for humans reading `bd show`), not a reader input.

## Sigil detection (Phase 3, shipped)

Hand-rolled iterative tokenizer in `pkg/analysis/sigils.go`. Bounded stack
(32 frames, `maxFenceDepth` at `pkg/analysis/sigils.go:34`), 1 MiB per-body
cap (`MaxSigilBodyBytes` at `pkg/analysis/sigils.go:29`), `Truncated=true`
flag on every match from a clipped body. Panic-recover wrapper at the
`ComputeRefRecordsV2` call site in `pkg/view/ref_record.go`.

### Per-mode vocabulary

Three modes shipped. The tokenizer (`sigilScanner` at
`pkg/analysis/sigils.go:122-149`) walks each line once; mode selects the
recognizer set. Per-ID priority dedup keeps the strongest kind per body
(`sigilKindPriority` at `pkg/analysis/sigils.go:57-63`).

#### `strict` mode

Only explicit syntactic marks count. No natural-language matching.

| Kind | Recognizer | Example |
|---|---|---|
| `markdown_link` | `[<id>](url)` where link text is exactly a bead-ID | `See [bt-vxu9](https://example.com) for context.` |
| `inline_code` | `` `<id>` `` within a single-backtick span | `The reader lives in `bt-gkyn`.` |
| `ref_keyword` | `ref:` or `refs:` (case-insensitive, optional single space) followed by a bead-ID | `ref: bd-iis, refs:mkt-gkyn` |

Implementation: `processLineSigil` at `pkg/analysis/sigils.go:212-324` with
helpers `tryMarkdownLink` (`:331`) and `tryRefKeyword` (`:360`).

#### `verb` mode (default)

Everything in `strict`, PLUS a fixed verb list with 32-char same-line
proximity (inclusive) to the bead-ID. Markdown formatting (`*`, `_`, `~`) is
stripped before counting proximity chars, so `**see**` counts as 3 not 7.

| Kind | Recognizer | Example |
|---|---|---|
| `verb` (plus all strict kinds) | One of `see`, `paired with`, `blocks`, `closes`, `fixes`, `mirrors` within 32 chars of an ID on the same line | `This closes bt-vxu9 once the corpus lands.` |
| | | `Paired with bd-iis upstream.` |
| | | `Blocks mkt-gkyn — ships before hook.` |

Verb list at `pkg/analysis/sigils.go:69-76`. Two-pointer proximity
resolution at `pkg/analysis/sigils.go:288-323` to stay O(verbs + ids) rather
than O(verbs × ids) on long lines.

Multiple verbs before the same ID → one record, highest-priority-kind wins
(verb loses to any strict kind if both fire). Verbs after the ID also count
within the window.

#### `permissive` mode

v1-style: any bead-ID outside a fenced block or inline-code span surfaces as
`bare_mention`. No sigil requirement. Retains the cross-project-only filter
(non-negotiable — v1 history shows removing it regresses to ~85% FPR).

| Kind | Recognizer | Example |
|---|---|---|
| `bare_mention` | Any bead-ID at a word boundary, outside inline code and fenced blocks | `The bt-gkyn reader lands alongside bt-vxu9.` |

Implementation: `processLinePermissive` at `pkg/analysis/sigils.go:180-202`.

### Dep-only kinds (emitted by reader, not tokenizer)

`ComputeRefRecordsV2` in `pkg/view/ref_record.go` scans dependencies and
emits two more `sigil_kind` values the tokenizer never produces:

- `external_dep` — dep edge with `external:` prefix (e.g.
  `external:seanmartinsmith/other-repo#42`)
- `bare_dep` — bare cross-prefix dep edge (e.g. `bt-x` → `bd-x`)

These are structural, not prose-derived. `DetectSigils` has no view into
them.

### Why verb is the default, not strict

Strict is the lowest-FPR mode but also drops a large chunk of real intent
because the corpus contains a lot of prose like `"closes bt-vxu9 next"` that
never wraps the ID in code or link syntax. On the dogfood corpus, strict
filters the 408 v1 refs down too aggressively — enough that agents would
miss actual cross-project links. Verb adds the six verbs that carry
near-unambiguous intent when co-located with an ID; the 32-char window keeps
noise from unrelated verbs elsewhere on the line.

Permissive exists for consumers who want v1-style recall and will handle the
FPR themselves. It is explicitly a fallback, not a recommended mode.

The choice is empirical and revisitable: the FPR gate (Phase 5) reports
strict/verb/permissive numbers on the labeled corpus, and a future data
point may flip the default to strict once the corpus is big enough to
confirm strict's recall is acceptable.

## Labeled corpus + FPR gate (Phase 5)

`pkg/view/testdata/corpus/labeled_corpus.json` — ≥30 sanitized real issues
from the shared Dolt server with truth labels. Pre-commit denylist scan
(passwords, tokens, secrets, URLs, emails outside allowed list) as a commit
gate.

Thresholds:
- Pair FPR <10% (requires N≥10 candidate pairs)
- Ref broken-flag FPR ≤5% under verb mode (strict/permissive informational)
- Memory delta <10 MB for corpus load

### Labeling rubric

This is the canonical labeling rubric. The plan defers to this doc; the
corpus consumes these definitions.

#### What makes a pair intentional

A pair is `intentional: true` iff a human read both beads' descriptions and
determined they describe the same logical work across projects — not merely
the same domain, not merely a coincident suffix.

- **Intentional:** created via `bd create --id=<suffix>` pairing workflow;
  mirrors the same commitment across repos (one repo's slice of shared
  work); the author explicitly calls out the pairing in notes or close
  reasons.
- **Suffix collision (not intentional):** both beads happen to share a
  3–4 char suffix because beads auto-generates IDs per project. Example: a
  `bt-153` "K-shortest critical paths" and a `mkt-153` "Rich description
  nudge hook" share the suffix `153` but describe unrelated work. These
  dominate v1 output.

Test: "If these two beads were created in the same repo, would the author
have filed them as a single issue or marked them duplicates?" If yes,
intentional. If no, collision.

#### What makes a ref intentional

A ref is `intentional: true` iff the author clearly meant to reference the
target bead — not as placeholder text, not as a coincidental slug, not as an
incidental substring of a longer identifier.

- **Intentional:** the prose mentions the target bead as a real thing (a
  dependency, a sibling, a prior-art pointer). The target resolves to an
  actual bead in the corpus.
- **Placeholder (not intentional):** literal `bt-xxx`, `cass-xyz`,
  `foo-abc1` in design docs or templates where the author meant "insert ID
  here later."
- **English slug (not intentional):** a lowercase word prefix followed by a
  hyphenated word that happens to match the bead-ID shape. Examples under
  known prefixes: `-only`, `-side`, `-show`, `-level`. Context disambiguates
  — `the bt-side reader` is a slug, not `bt-side` the bead.
- **Prose about the prefix, not a bead:** `the bt tool...` mentions bt but
  not a bead.

Test: substitute a different bead-ID at the same position. Does the sentence
still parse as making the same kind of claim (just about a different bead)?
If yes, intentional. If it becomes nonsense (`the bt-xxxx reader` reads as
placeholder, `the cass-only reader` reads as nonsense), not intentional.

#### Agent-driven labeling procedure

The corpus labels will be applied by an agent with human review at
ambiguity points. Procedure:

1. For each candidate pair surfaced by `bt robot pairs --global --schema=v1`:
   - Run `bd show <canonical>` and `bd show <mirror>` for every member.
   - Apply the intentional/collision test above using only each bead's
     description + close reason.
   - Label `intentional: true|false` with a one-sentence `reason` field.
   - If uncertain after reading both descriptions, escalate — do not guess.
2. For each candidate ref surfaced by `bt robot refs --global --schema=v1`:
   - Inspect the prose span that triggered the match (location field).
   - Apply the intentional/placeholder/slug test.
   - Label + one-sentence reason.
   - If the target doesn't exist in the corpus, the ref has an empty
     intentionality judgment — the broken-flag test doesn't need it.
3. Escalation for ambiguous cases: post a comment on the bead itself
   (`bd comments add <id> "labeling: unclear because..."`) and skip the
   record until the author or reviewer resolves it.

#### Dispute resolution

Disputes resolve on the bead itself — comment on the bead justifying the
re-label, then update the corpus. The audit trail lives where the work
lives, not in an external review doc. This follows the project's broader
pattern of keeping provenance with its subject.

#### Prefix-aliasing policy

The corpus references real cross-project beads (`bt-zsy8`, `bd-zsy8`,
Gastown 4-way `bt-byk` + `cass-byk` + `tpane-byk` + `cnvs-byk`). The bt repo
is public. Before committing the corpus:

- Confirm every prefix referenced (`bt`, `bd`, `cass`, `tpane`, `cnvs`,
  `mkt`, `dotfiles`) is intended to be publicly disclosed as an existing
  project.
- If any prefix should stay private, alias it in the corpus
  (`tpane-x` → `proj1-x`) and document the mapping in a top-level
  `alias_map` field in `labeled_corpus.json`.
- Aliases are stable — once committed, don't rename. New aliases append.
- Aliasing applies to IDs only, not to prose content. Prose that names a
  private project by name gets redacted or the issue is excluded.

The alias map is a corpus-level artifact, not per-record. The sanitization
script checks it.

### Sanitization script

Cross-ref: `scripts/audit-corpus.sh` (Phase 5 deliverable) is the pre-commit
gate. It runs the denylist regex scan against every serialized issue prose
field (`description`, `notes`, `comments`, `close_reason`) and every
`dependencies` entry. Any hit fails the commit; the `security` label skip is
a fallback, not the primary mechanism.

Denylist regex categories (authoritative list lives in the script, not
here):

- **Credentials:** passwords, generic `token` / `secret` / `api_key`
  substrings
- **Cloud keys:** AWS access key IDs (`AKIA…`), GitHub personal access
  tokens (`ghp_…`), Slack tokens (`xox[bp]-…`)
- **Local paths:** `.env` filenames, Windows user home paths
  (`C:\Users\<name>`), localhost URLs with ports
- **Emails outside allowlist:** anything not `@seanmartinsmith.com` or
  `@users.noreply.github.com`
- **Private repos:** URLs pointing to non-public repos

The script's regex list is the source of truth; this doc lists categories
for reader orientation only. See the script for exact patterns.

## Rollback

If v2 default produces unexpected results post-ship:

1. **Consumer workaround (immediate):** pin with `--schema=v1` flag or
   `BT_OUTPUT_SCHEMA=v1` env. No code change.
2. **Default flip (one line):** revert `resolveSchemaVersion` default from
   `"v2"` to `"v1"`. Existing v2 invocations keep working; new ones default
   v1.
3. **Full revert:** `git revert` the v2 dispatch commit. v1 code paths were
   never modified, so revert is mechanical.

The FPR gate is a quality threshold, not a correctness gate. A failing gate
skips (`t.Skip` under investigation) while the reader keeps working. No
data migration, no consumer breakage, no state to clean up.

## Open items

- [x] Phase 2: ship `ComputePairRecordsV2`, flip default to v2
- [x] Phase 3: ship `pkg/analysis/sigils.go` + `ComputeRefRecordsV2`, flip default
- [x] Phase 4: expand this doc; convention pointer fleshed out
- [ ] Phase 5: corpus + FPR gate + `scripts/audit-corpus.sh`
- [ ] Post-ship: `--explain-refs` observability (new bead), upstream
  `paired_with` column PR (new upstream bead, after dogfood)

## Cross-references

### Plans and conventions

- Plan: `docs/plans/2026-04-21-feat-cross-project-intent-taxonomy-pairs-refs-v2-plan.md`
- Brainstorm: `docs/brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md`
- bt-side convention: `.beads/conventions/cross-project.md`
- v1 pairs design: `docs/design/2026-04-20-bt-mhwy-2-pairs.md`
- v1 refs design: `docs/design/2026-04-20-bt-mhwy-3-refs.md`

### Beads

- `bt-gkyn` — pairs v2 reader (CLOSED after Phase 2)
- `bt-vxu9` — refs v2 reader (open through Phase 6)
- `mkt-gkyn` — marketplace-side pair hook + skill text (parallel track)
- `mkt-vxu9` — marketplace-side refs skill extension (parallel track)
- `bd-iis` — upstream beads context for the pair notion

### Shipped code

- `pkg/view/pair_record.go` — `ComputePairRecordsV2`, `PairRecordV2`,
  `bfsComponents`, `computeDriftV2`
- `pkg/analysis/sigils.go` — `DetectSigils`, `SigilMode` enum, per-mode
  recognizers, fence stack, truncation handling
- `pkg/view/ref_record.go` — `ComputeRefRecordsV2`, `RefRecordV2`,
  `scanIssueProseV2` (calls `analysis.DetectSigils` with panic-recover
  wrapper)
- `cmd/bt/robot_pairs.go` — `pairsOutput` schema dispatch
- `cmd/bt/robot_refs.go` — `refsOutput` schema + sigil dispatch
