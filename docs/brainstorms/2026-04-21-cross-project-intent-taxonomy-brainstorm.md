# Cross-Project Intent Taxonomy (bt-gkyn + bt-vxu9)

**Date:** 2026-04-21
**Status:** Brainstorm captured, open questions below
**Primary beads:** bt-gkyn (pairs v2, P2), bt-vxu9 (refs v2, P2)
**Sibling beads:** bt-ushd (epic), bt-2cvx (session provenance), bt-k9mp (cross-project filing), bt-6cfg (TUI pair rendering)
**Upstream parallels:** bd-fjip (session_history), bd-e6p (notification hook), bd-k8b (closed — same-ID linking)
**Origin session:** bf532eb2-fb5d-4227-9d50-e76b90049d30

## Context

Pairs v1 (shipped in bt-mhwy.2) and refs v1 (shipped in bt-mhwy.3) surface cross-project bead identity by pattern-matching on strings. Dogfooding on the real shared Dolt corpus showed the FP rates this produces:

- Pairs: 29 detected, ~24 fire `title` drift, ~5 are intentional (~5× FP rate).
- Refs: 408 detected, ~119 broken, ~30% broken-flag FPR driven by placeholder IDs (`bt-xxx`) and English slugs (`-only`, `-side`).

Both bead authors filed v2 follow-ups anticipating schema bumps and intent-based identity. The open question was *what intent looks like*.

This brainstorm reframes the scope: **bt-gkyn and bt-vxu9 are readers in a broader cross-project taxonomy the user has been building across bt + beads + marketplace.** The real decision is where intent is *stamped* and what conventions *persist across sessions*. The readers are thin.

## Prior Art

- `bt-ushd` (open P1 epic): "bt is the cross-project layer; beads is the backend, deliberately stops at single-project."
- `bd-ftb` (closed research, 2026-04-13): "the building blocks exist — `bd-cxd` and `bd-k8b` need **convention/config, not new primitives**."
- `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`: canonical skill that already prescribes paired-ID creation + `bd dep add` cross-linking. **Aspirational in current form — agents don't always add the dep.**
- `bd-fjip` upstream comment: "Enforcement that sessions actually provide it lives in marketplace as a PreToolUse hook" — the established enforcement pattern for metadata stamping.

## What We're Building

**Scope split across two repos.** bt ships the readers and its own convention pointer. The marketplace repo owns the hook and the canonical skill extension — collaborative surface, filed separately.

### In bt (this project)

#### 1. Reader layer — pair.v2 and ref.v2

Both keep the `pkg/view/` projection pattern established by bt-mhwy.1.

**pair.v2:**
- Identity rule: a pair exists iff two same-suffix cross-prefix beads have a dep edge between them. Any direction, any `DependencyType`. The dep graph IS the decision signal — zero string parsing for detection.
- Notes line (`Paired-With: <other-id>`) is **output-only provenance**, not a detection channel. It's surfaced in the record for human reading but does NOT rescue pairs that lack the dep edge. If the hook fails to add the dep, that's a hook bug to surface, not a reader workaround to hide.
- `title` drift removed from the flag set (memory: carries no actionable signal). Flags reduce to `status`, `priority`, `closed_open`.
- Output adds per-record `intent_source`. v2 emits `"dep"` always (single channel); field reserved for future channels.
- Consequence for existing corpus: the ~5 real pairs (including the memory-cited `bt-zsy8` / `bd-zsy8`) are currently invisible to v2 because none have dep edges. Before shipping v2 as the default, manually run `bd dep add` for each of the ~5 known intentional pairs. This is the "forward-only backfill" decision made concrete.

**ref.v2:**
- Tunable `--sigils=strict|verb|permissive` flag + `BT_REFS_SIGILS` env. Default = verb. All three modes ship; dogfooding drives the default's evolution.
- `strict`: markdown link (any `[<id>](...)` where link text is a valid bead ID, regardless of target URL) + inline code (single backtick around ID) + `ref:` / `refs:` keyword (case-insensitive, optional single space, `ref:bt-x` or `ref: bt-x` both valid) only.
- `verb`: strict set + fixed verb list (`see`, `paired with`, `blocks`, `closes`, `fixes`, `mirrors`). Verb-to-ID proximity: same line, verb within 32 chars preceding the ID. Case-insensitive.
- `permissive`: v1 known-prefix scoping retained, PLUS fenced code block exclusion (triple-backtick and triple-tilde regions) and inline code exclusion. Only hardening over v1, no additional sigil requirement.
- Lightweight hand-rolled markdown tokenizer for fenced-block + inline-code + link recognition (target ~50 LOC, no new deps).
- Output adds per-record `sigil_kind` enum: `"markdown_link"`, `"inline_code"`, `"ref_keyword"`, `"verb"`, `"external_dep"`, `"bare_dep"`, `"bare_mention"` (permissive mode only).

#### 2. Schema

Both subcommands bump to `pair.v2` / `ref.v2`. Envelope gains `intent_source` (pairs) / `sigil_mode` (refs) describing how filtering was applied. Records gain per-item provenance fields. `--schema=v1` fallback retained for at least one release so agents can pin while migrating.

#### 3. bt-side convention doc

New `.beads/conventions/cross-project.md` in bt. Thin pointer doc:

- References `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md` as the canonical convention.
- Documents the bt-specific reader surface: what each `--sigils` mode recognizes, what `intent_source` / `sigil_kind` values mean, how to pin to `--schema=v1`.
- bt sessions discover it via the existing `.beads/conventions/` pattern (labels.md + reference.md live there already).

#### 4. Acceptance harness — two artifacts, two roles

**Goldens (existing pattern)** — `pkg/view/testdata/golden/pair_*.json` + `ref_*.json`. Snapshot the v2 output on curated synthetic fixtures. Byte-diff on every test run. Regenerate with `GENERATE_GOLDEN=1` whenever rules legitimately change. Catches mechanical regressions; no human judgment needed.

**Labeled corpus (new artifact)** — `pkg/view/testdata/labeled_corpus.json`. ~50-100 real issues pulled from the shared Dolt server, sanitized (PII scrub, URL redaction), with a `truth` field per expected pair / ref capturing whether the detection is correct. Drives a separate FPR test: run each `--sigils` mode + default pair detection against the corpus, assert FPR against the truth labels. Thresholds: ≤5% broken-flag FPR on refs (bt-vxu9 AC), <10% total-pair FPR on pairs. Corpus is re-labeled only when we learn a truth label was wrong.

Clean separation: goldens regenerate freely; truth labels change rarely. Rule tweaks don't force re-labeling the corpus.

### In marketplace (separate, collaborative)

Two paired beads filed in the marketplace repo, each mirroring a bt-side bead via the `--id` suffix convention:

- **`mkt-gkyn`** (paired with `bt-gkyn`) — the pair-intent substrate: PreToolUse hook + canonical skill extension for pair rules. Hook matches `Bash` tool calls with command shape `bd create --id=<suffix>` where the suffix is already present in another loaded project's Dolt database. On match, runs `bd dep add` + appends `Paired-With:` notes line. May consolidate with `bt-2cvx` / `bd-fjip` session-provenance hook if those land together.
- **`mkt-vxu9`** (paired with `bt-vxu9`) — refs-side skill extension only. Adds the sigils matrix (strict / verb / permissive vocab + verb list + proximity rule) to `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`. No hook — refs intent is prose-embedded; there's no `bd create --id=<suffix>` equivalent to hook into. Smaller scope than its pair.

bt's readers ship WITHOUT waiting for the marketplace hook — they operate on whatever intent data exists, and the bt-side convention doc explains the manual path (`bd dep add` by hand) for the ~5 intentional pairs on the current corpus.

## Why This Approach

**Pair intent is structural, ref intent is syntactic.** Pairs live at bead-level (the `bd create --id=<suffix>` gesture); refs live inside prose (agents/humans type them as they write). Trying to force shared infrastructure on them would be premature unification. Keeping them separate aligns with where intent actually lives.

**Convention + hook + readers is the dogfood-shaped wedge.** The marketplace-owned convention and hook evolve on one schedule; bt-owned readers evolve on another. Each piece ships on its own cadence without blocking the others. Readers tolerate a pre-hook world by emitting less data, not by failing.

**Deferring the upstream beads column** is the right cost/benefit call right now. We're still learning the taxonomy. Committing to an upstream schema change before dogfooding would freeze decisions we're not ready to make. Once the hook + readers are stable, an upstream PR is mechanical.

**Tunable sigils over single-choice commitment.** The user's "these all seem like they have pros and cons" answer reflects real uncertainty. Shipping all three modes behind a flag converts "pick the right default" into "run three tests on the real corpus and see." Zero code-change cost to flip the default later.

## Key Decisions

1. **Pairs and refs are two mechanisms, not one shared parser.** Pair intent = structural (dep edge). Ref intent = syntactic (prose sigil). The only shared surface is the `pkg/view/` projection pattern and the cross-project convention.
2. **Scope splits across two repos.** bt ships readers + its own convention pointer + acceptance harness. Marketplace owns the hook and the canonical `cross-project` skill extension. Readers ship without the hook — agents can add dep edges manually for the ~5 real pairs.
3. **Upstream beads column deferred.** Convention + hook in marketplace now; upstream PR is a separate parallel track once the convention stabilizes.
4. **Pair v2 reader rule = dep edge alone is the decision signal.** Hook stamps both the dep edge and a `Paired-With: <id>` notes line; readers use ONLY the dep graph for detection. Notes line is output-only provenance for human eyes in `bd show` — not a rescue channel for missing dep edges. Hook failures surface as bugs, not hidden by reader OR-logic.
5. **`title` drift removed from pair flags.** Memory confirms: it carries no actionable signal. Flags are `status`, `priority`, `closed_open`.
6. **Ref v2 sigil scope = runtime tunable, default = verb.** `--sigils=strict|verb|permissive` + `BT_REFS_SIGILS` env. Ship all three; sigil vocab and verb-to-ID proximity rule documented in the spec section.
7. **Markdown parsing = hand-rolled lightweight tokenizer, no new deps.** Skip fenced code blocks (triple-backtick / triple-tilde), recognize inline code (single backtick), recognize markdown link syntax. Goldmark is overkill for the scope.
8. **Schema bumps to v2 for both.** Envelope captures mode; records carry per-item provenance. `--schema=v1` fallback retained for one release.
9. **Backfill is forward-only.** v2 readers ignore ambiguous suffix-collision pairs. The ~5 real pairs get `bd dep add` manually before v2 becomes the default (including `bt-zsy8` / `bd-zsy8`, the memory-cited canonical example). No migration harness.
10. **Acceptance harness = two artifacts.** Goldens (`pkg/view/testdata/golden/`) for mechanical snapshot regression. Labeled corpus (`pkg/view/testdata/labeled_corpus.json`, ~50-100 sanitized real issues with human truth labels) for FPR gate asserting ≤5% broken-flag FPR on refs and <10% total-pair FPR on pairs.
11. **Hook is pair-specific.** Refs intent is entirely prose-parsed because refs are prose-embedded by nature; there is no equivalent of `bd create --id=<suffix>` for refs.
12. **Marketplace-side split = two paired beads.** `mkt-gkyn` (pair hook + skill) pairs with `bt-gkyn`; `mkt-vxu9` (refs skill extension only, no hook) pairs with `bt-vxu9`. Each bt-side bead has its marketplace mirror via the `--id` suffix convention.

## Resolved Questions

- **Is there real shared infrastructure between pairs and refs?** No — only the `pkg/view/` projection pattern and the convention doc. Intent mechanisms are fundamentally different.
- **Where does the enforcement layer live?** Marketplace PreToolUse hook + extended `cross-project` skill. Upstream beads column later.
- **Pair marker format?** Hook stamps both dep edge + notes line. Reader uses dep edge alone as detection signal; notes line is output-only provenance.
- **Ref sigil scope?** Tunable, default verb. All three modes ship.
- **Schema versioning?** Bump to v2, envelope + per-record provenance, v1 fallback retained.
- **Sequencing / scope split.** Hooks belong in the **marketplace repo** (collaborative cross-project surface). This project (bt) ships: pair v2 reader, ref v2 reader, bt-side convention doc, acceptance harness. Hook work is filed / tracked in marketplace as `mkt-gkyn`, NOT blocking bt shipment.
- **Convention doc location.** Both. Harness skill remains canonical (extended in marketplace under `mkt-gkyn` / `mkt-vxu9`). bt adds `.beads/conventions/cross-project.md` as a thin discoverability pointer + bt-specific reader notes (what `--sigils` modes do, what `intent_source` field means, how to invoke each reader, how to pin `--schema=v1`).
- **Backfill.** Forward-only. v2 readers ignore ambiguous suffix-collision pairs. The ~5 real pairs on the current corpus get `bd dep add` added manually before v2 becomes the default. `--schema=v1` fallback covers any transitional consumer that needs the old broad semantics. No migration harness.
- **Acceptance methodology.** Two artifacts, two roles. Goldens for snapshot stability (existing pattern). Labeled corpus (~50-100 sanitized issues with human truth labels) for FPR gate: ≤5% broken-flag FPR on refs, <10% total-pair FPR on pairs. Separate files keep regeneration cheap and truth-labeling rare.
- **Marketplace bead pairing.** Two paired beads (`mkt-gkyn` + `mkt-vxu9`) via `--id` suffix convention, mirroring the bt-side pair. `mkt-gkyn` covers hook + pair skill text; `mkt-vxu9` covers refs skill extension only (no hook — refs intent is prose-embedded).

## Open Questions

- **Dep type for manually-added edges (and future hook-added edges).** Use existing `related` / `blocks` / `depends_on`? Introduce a new `mirrors` / `paired` type if upstream vocab supports it? This is resolvable at implementation time by reading beads' `DependencyType` enum — not a brainstorm blocker.
- **Corpus sanitization scope.** The frozen snapshot is pulled from the real shared Dolt server. Any PII / secret text in descriptions needs scrubbing before the fixture is committed. Decide scrubbing approach at implementation (diff against fixtures or pattern-based redaction) — not a brainstorm blocker.

## Cross-Project Constellation

| Bead | Project | Role |
|---|---|---|
| **bt-gkyn** | bt | Pairs v2 reader |
| **bt-vxu9** | bt | Refs v2 reader |
| **bt-ushd** | bt | Parent epic — cross-project beads OS |
| **bt-2cvx** | bt | Session provenance — shares hook surface |
| **bt-k9mp** | bt | Cross-project filing — sibling convention |
| **bt-6cfg** | bt | TUI pair render — downstream consumer |
| **bt-mhwy.2** | bt | Pairs v1 (shipped) |
| **bt-mhwy.3** | bt | Refs v1 (shipped) |
| **bd-fjip** | beads | Session_history storage — upstream pair in spirit |
| **bd-e6p** | beads | Notification hook — adjacent enforcement surface |
| **mkt-gkyn** | marketplace | New, paired w/ bt-gkyn — pair hook + pair skill text |
| **mkt-vxu9** | marketplace | New, paired w/ bt-vxu9 — refs skill extension (no hook) |
| **harness/skills/cross-project** | marketplace | Canonical convention skill extended under mkt-gkyn/mkt-vxu9 |

## References

- bead: `bd show bt-gkyn` (pairs v2)
- bead: `bd show bt-vxu9` (refs v2)
- design doc: `docs/design/2026-04-20-bt-mhwy-2-pairs.md`
- design doc: `docs/design/2026-04-20-bt-mhwy-3-refs.md`
- skill: `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`
- memory: `project_pair_suffix_collisions.md`
- memory: `feedback_cross_project_bead_pairing.md`
- upstream research: `bd show bd-ftb`
- upstream epic (migrated): `bd show bd-mh6` → `bd show bt-ushd`
