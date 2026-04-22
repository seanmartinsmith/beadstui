# Changelog

Development log for beadstui. Each entry covers one Claude Code session's work, keyed by date.

For architectural decisions, see `docs/adr/`. For issue tracking, use `bd list`.

---

## 2026-04-22 - Blurb v2, JSONL sunset, refactor epic closeout (bt-yqgn, bt-jlp, bt-if3w)

**Two shipped beads plus an epic close-out. Agents dispatched for in-flight work; decision debt on the pkg/ui refactor epic cleared.**

### What shipped

- **bt-yqgn** — `pkg/agents/blurb.go` rewritten v1 → v2. New markers (`<!-- bt-agent-instructions-v2 -->`), dual-family recognition (both `bv-` v1 and `bt-` v2 markers detected so v1-installed projects upgrade on next run), content fully rewritten for 2026-era `bt robot <subcmd>` surface (triage/next/portfolio/pairs/refs/plan/impact/insights/…), `--global`/`--shape`/`--compact`/`--full`, `$CLAUDE_SESSION_ID`, `bd prime`, mandatory reads, positional `bd create`, atomic `--claim`, Summary/Change/Files/Verify/Risk/Notes close template, project P0–P4 table, correct sync idiom (`bd dolt push && git push`). Five new tests: TestUpgradeV1ToV2, TestFreshInstallIsV2, TestAgentBlurbNoBdSync (regression guard), TestAgentBlurbNoLegacyBvMarkers, TestBlurbMarkersAreV2. Agent-dispatched; verified build + vet + `go test ./pkg/agents/...` clean before commit.
- **bt-jlp** — JSONL persistence audit for the beads v1.0.1 opt-in migration. Classified as Case A (redundant): both Dolt and JSONL were active, `refs/dolt/data` present on origin, JSONL auto-export actively failing with `auto-export: git add failed: exit status 1` on every bd write. `export.auto=false`, `.beads/issues.jsonl` untracked via `git rm --cached` + gitignore pattern. Dolt is now canonical. Parent `dotfiles-jlp` epic updated with bt status.
- **bt-if3w epic closed (re-scoped)** — Phases 0–3 + 1.5 shipped the refactor value (mechanical Charm v2 migration, test foundation, Model decomposition with ViewMode/DataState/FilterState/ModalType enums + Update() split, AdaptiveColor kill across 174 occurrences, Cobra migration main.go 1708→13 lines + 35+ robot subcommands, footer extraction). Residual hygiene decoupled from the epic frame and tracked as standalone P2 polish beads: `bt-t82t` (Phase 4 cleanup — stale `bv-` refs + golden regen + test validation) and `bt-if3w.1` (sprint view extraction, same pattern as the closed bt-oim6 footer). Hot-path style pre-computation **cut** (YAGNI — noted as "needs profiling first", no profiling evidence exists).
- **bt-bo4a gate closed** — 8-day-old "what's Phase 4 vs what gets cut" gate resolved with the scoping decision above. Decision debt cleared.

### ADR-002 Stream 4

Updated to reflect epic closure and the decoupled follow-up beads. Stream 4 is now fully DONE at the stream level; the two open children compete against the broader backlog on their own merits rather than inheriting urgency from the epic frame.

### Risk

Low. bt-yqgn is content + version bump with backward-compat marker detection. bt-jlp removed a broken code path (JSONL export was already erroring on every write). bt-if3w closure is a re-scope of status, not a code change.

### Commits

- `b76f2a1` chore(data): disable JSONL export, Dolt is canonical (bt-jlp)
- `72544630` docs(agents): rewrite blurb v1 -> v2 (bt-yqgn)

---

## 2026-04-21 - Pairs/refs v2 docs + labeled corpus + FPR gate (bt-vxu9, Phases 4-5)

**Closed the pairs+refs v2 plan: expanded the bt-side convention + design docs into cold-readable references, landed a 32-issue labeled corpus with a pre-commit sanitization gate, and shipped an FPR threshold test asserting the v2 readers stay under their agreed-on false-positive budgets.** Phases 4 + 5 + 6 of the plan; closes bt-vxu9.

### What shipped

- **`.beads/conventions/cross-project.md`** — Phase 1 skeleton replaced with per-mode sigil vocabulary, intent_source/sigil_kind semantics, when-to-pin `--schema=v1` scenarios, full invocation examples (default, v1 pin, sigil modes, `--orphaned`, env-var forms), and migration guidance pointing at bt-xgba for removal.
- **`docs/design/2026-04-21-pairs-refs-v2.md`** — authoritative labeling rubric, BFS-over-connected-components rationale (vs union-find at N=400), per-mode sigil tables with concrete prose examples, "why schema-bump not envelope-additive", "why dep-edge as sole signal", "why verb default", rollback semantics, and file:line cites to shipped code. Flagged one dead-code observation (`hasVerbWithinProximity` at pkg/analysis/sigils.go:546 is unreferenced).
- **`scripts/audit-corpus.sh`** — 117-line jq-based pre-commit scanner. Denylist: password/secret/token/api_key, AWS/GitHub/Slack token shapes, `.env`, `localhost:port`, Windows/macOS user paths, emails outside the sms@seanmartinsmith.com / @users.noreply.github.com allowlist. Hard-requires jq (no grep-over-JSON fallback — would false-positive on key names).
- **`pkg/view/testdata/corpus/labeled_corpus.json`** — 32 sanitized issues modeled on real shared-Dolt state. 11 candidate pair records (5 intentional: byk 4-way / zsy8 / 2il / x08 / fjip 3-way; 6 suffix-collision negatives: 153 / 1hk / 52t / 9c9 / cv6 / dyg). 27 labeled ref candidates exercising each sigil mode with placeholder negatives (`bt-xxx`) and English-slug negatives (`-only`, `-side`) that strict/verb correctly suppress.
- **`pkg/view/fpr_gate_test.go`** — three subtests: corpus-load (malformed fixture / missing truth / N<10 candidates / memory-delta >10 MiB all fail loudly); pair.v2 FPR gate at <10%; ref.v2 broken-flag FPR gated at ≤5% under verb mode with strict + permissive reporting informational readouts.

### Measured numbers

Baseline vs v2 on the labeled corpus:
- **Pair FPR**: v1 ~83% (~24 of 29 dogfood pairs were suffix collisions) → **v2 0.00%** (5/5 emitted records intentional, far under 10% gate).
- **Ref broken-flag FPR**: v1 ~30% (dogfood baseline) → **v2 0.00% across all three modes** (strict: 9 records total, 1 broken, 0/1 FP; verb: 21 records, 1 broken, 0/1 FP; permissive: 26 records, 1 broken, 0/1 FP).

The single broken ref in each mode is the intentionally-broken `external:bd:nonexistent` dep on `bt-refbroken` — detector is correctly flagging a genuinely dangling external ref.

### Risk

Low. Phase 4 is docs-only; Phase 5 adds one test file + one fixture + one shell script. No production code touched. FPR gate is additive and runs alongside existing goldens; breakage would fail `go test ./pkg/view/` loudly without affecting readers.

### Deferred follow-ups

- **bt-xgba** (P2): remove `--schema=v1` fallback one release after ship. Filed Phase 3.
- **--explain-refs observability mode** (bt-113x, P3, discovered-from bt-vxu9): emits rejection reasons per prose candidate span so FPR regressions are debuggable.
- **`pkg/analysis/sigils.go:546` dead code** — unreferenced `hasVerbWithinProximity` helper from an earlier draft; actual resolution is inlined in `processLineSigil`. Flagged in bt-vxu9 close notes, not a separate bead (trivial cleanup).

---

## 2026-04-21 - Refs v2 + sigils detector + default schema flip (bt-vxu9)

**Ship the ref.v2 reader with a hand-rolled sigil tokenizer, and flip the default `--schema` from v1 to v2 for both `bt robot pairs` and `bt robot refs`.** Both v2 readers now live side-by-side with their v1 counterparts; `--schema=v1` remains as an opt-in fallback. Phase 3 of bt-vxu9 / bt-gkyn.

### What shipped

- **`pkg/analysis/sigils.go`** — hand-rolled iterative tokenizer. `DetectSigils(body, mode)` walks bytes once, emits `SigilMatch{ID, Kind, Offset, Truncated}`. Three recognizer sets:
  - **strict**: markdown links `[bead-id](url)`, inline code `` `bead-id` ``, `ref:` / `refs:` keyword (case-insensitive, optional single space).
  - **verb**: strict + fixed verb list (`see`, `paired with`, `blocks`, `closes`, `fixes`, `mirrors`) with same-line 32-char inclusive proximity. Markdown format chars (`*`, `_`, `~`) stripped before proximity counts.
  - **permissive**: every bead-shaped ID outside fenced or inline code emits `bare_mention`.
  - Fence stack capped at 32 frames; 1 MiB per-body cap with `truncated: true` flag propagated to every match.
  - Verb-proximity post-pass uses a two-pointer sliding window so the proximity resolver is O(verbs + ids + matches) rather than O(V × I) — caught by the linear-scaling test on 100 KB inputs.
- **`pkg/view/ref_record.go`** — `ComputeRefRecordsV2`, `RefRecordV2`, `RefRecordSchemaV2 = "ref.v2"`. Prose scan delegates to `analysis.DetectSigils` under the caller-chosen mode; each prose field is wrapped in `defer recover()` so one adversarial body logs + skips rather than crashing `--global` for the rest of the corpus. v1's cross-project-only filter is preserved — the load-bearing FP guard. `SigilKind` on each record carries the sigil that established intent; `Truncated` flags records from oversize bodies. Dep-derived records use new `external_dep` / `bare_dep` kinds.
- **`cmd/bt/robot_refs.go`** — `refsOutputV2` and `emitRefsV2` mirror the pairs v2 inline-dispatch pattern. The Phase 1 stub `--schema=v2 not yet implemented` is gone.
- **`cmd/bt/robot_output.go`** — envelope gains optional `sigil_mode` (omitted when empty). Existing envelope goldens stay byte-identical because the field is omitempty.
- **`cmd/bt/robot_schema_flag.go`** — `robotSchemaDefault` flipped from `robotSchemaV1` to `robotSchemaV2`. This is the coordinated default-flip both v2 readers were waiting on.
- **`.beads/conventions/cross-project.md`** — "Phase 1 default: v1" language replaced with "Default: v2 as of Phase 3 of bt-vxu9".

### Tests

- `pkg/analysis/sigils_test.go` — 28 unit tests per sigil kind + full pathological set (invalid UTF-8, lone surrogates, RTL override, ZWJ, 100 KB single-line, 100 K inline-code storm, 10 K nested fences, 10 K link sequence, 1 MB boundary, adversarial combined sweep). Linear-scaling test asserts runtime(100 KB) ≤ 15 × runtime(10 KB). Four benchmarks (`Benchmark_DetectSigils_{10KB,100KB,PathologicalNestedFences,PathologicalInlineCodeStorm}`).
- `pkg/view/ref_record_test.go` — per-mode unit tests: strict markdown link / inline code / ref keyword; verb proximity inclusive-32 boundary; markdown-stripping (`**see**`); multiple-verbs-one-ID collapsed; permissive bare mention + inline-code suppression; fenced-code suppressed in all modes; cross-project filter; external-dep broken/resolved; truncated flag on >1 MB body; sorted output; flag-order invariant.
- `pkg/view/testdata/fixtures/ref_v2_*.json` + matching goldens — 6 fixtures covering each mode × 2 scenarios; registered in `TestRefRecordV2Golden` with mode inferred from filename prefix.
- `cmd/bt/robot_refs_test.go` — `TestRefsOutputV2_SchemaAndSigilMode`, `TestRefsOutputV2_SigilKindPresent`, `TestRefsOutputV2_CrossProjectOnly`, `TestRefsOutputV2_EmptyReturnsArray`, `TestRobotRefs_SigilModesResolveClean`, `TestRobotRefs_EnvVarOverride`. Existing validator tests updated for the flipped default.

### Smoke (real shared Dolt corpus)

- `bt robot refs --global` default → `schema: "ref.v2"`, `sigil_mode: "verb"`, 108 records.
- `bt robot refs --global --schema=v1` → `ref.v1`, 531 records (unchanged).
- `bt robot refs --global --sigils=strict` → 57 records (inline_code only on this corpus; 0 slug FPs for `-only`/`-side`/`-show`/`-level`).
- `bt robot refs --global --sigils=permissive` → 485 records (still < v1 because fenced + inline code now excluded).
- `BT_SIGIL_MODE=strict bt robot refs --global --sigils=verb` → verb wins.
- `bt robot pairs --global` → `pair.v2`, 22 records (default flipped).
- Total `--global` runtime ~175 ms, well under the 800 ms budget.

### Risk

Low. Both v1 readers untouched and test-covered. v2 readers isolated behind dispatch; `--schema=v1` retains the frozen wire shape for one release. Panic safety at the call site is belt-and-braces — the adversarial test sweep confirms `DetectSigils` itself doesn't panic, but the wrapper guards against future changes or unexpected gremlins in real data.

### Follow-ups open

- Phases 4 + 5 + 6 shipped in a follow-up session (2026-04-21 evening). See the next entry.
- Remove `--schema=v1` fallback — bt-xgba triggers one release after this ship.

---

## 2026-04-21 - Source filter (bt-mhwy.6)

**New `--source <project>[,<project>...]` persistent robot flag scopes output to one or more source projects.** Agents can now ask "show me only bt + cass beads" without BQL or workspace-level setup. Closes bt-mhwy.6, the last of seven children of the mhwy epic.

### What shipped

- **`--source` persistent flag** on `robotCmd` (cmd/bt/cobra_robot.go). Comma-separated projects; case-insensitive. Surfaced as `query.source` in the `robot list` envelope echo.
- **`filterBySource` helper** (cmd/bt/helpers.go). Exact project-name matching on either the ID prefix (`SplitID(issue.ID)`) or the `SourceRepo` field. Unknown projects produce empty results (silent — the plan's stated behavior).
- **Applied in two places**: inside `robotPreRun()` before the BQL / label-scope / recipe chain so `triage`, `next`, `insights`, `plan`, `priority`, `alerts`, `suggest`, etc. all filter consistently; inside `robot list`'s `RunE` separately because list bypasses `robotPreRun` and reads `appCtx.issues` directly. The compact reverse-map computation also uses the source-filtered slice so `children_count` / `unblocks_count` reflect the scoped graph.
- **Docs in `robot_help.go`** — `--source` section explains the flag, notes the `source_repo` field surfaces in compact.v1 / pair.v1 / portfolio.v1, and shows two agent-friendly examples. The committed JSON schemas at `pkg/view/schemas/*.v1.json` already document `source_repo`, so the schema-side story is complete without new machinery.

### Scope note

bt-mhwy.6 shrank over the course of the mhwy series: `source_repo` was already surfacing in compact (.1), portfolio (.4), and pair (.2) output before this task ran, so .6 is primarily a CLI ergonomics feature rather than a data-exposure feature. The 2026-04-21 color comment on the bead spelled this out.

### Tests

- `cmd/bt/helpers_source_test.go` — 8 unit cases: empty filter, single prefix, comma-separated, unknown prefix yields empty, SourceRepo fallback, case-insensitive, whitespace trimming, empty tokens dropped.
- `cmd/bt/robot_source_test.go` — 3 contract cases via the real binary: `--source=cass` filters down + echoes `query.source`, `--source=nonexistent` exits 0 with `count: 0`, `--source=bt,cass` matches both.

### Smoke

`bt robot list --source=bt --limit=3` returns 3 bt-* beads with `query.source: "bt"`. `--source=nonexistent` returns `count: 0` cleanly. `--source=cass,bt --limit=5 --global` mixes cass and bt records as expected. `bt robot triage --source=cass --global` scopes quick_ref counts to cass.

### Cross-project constellation (status)

**7 of 7 mhwy children closed.** Epic bt-mhwy delivered: compact output (.1), external dep resolution (.5), portfolio (.4), pairs (.2), refs (.3), source filter (.6). Remaining follow-ups: bt-gkyn (pairs v2 intent identity), bt-vxu9 (refs v2 sigil identity).

---

## 2026-04-21 - Refs subcommand (bt-mhwy.3)

**New `bt robot refs --global` subcommand validates cross-project bead references in prose and dep fields.** Scans description, notes, comments, and unresolved `external:` dependencies for bead IDs whose prefix differs from the source's project, validates each against the global set, and emits flags for `broken` / `stale` / `orphaned_child`. Agents can now see stale cross-refs that rot silently.

### What shipped

- **`pkg/view/ref_record.go`** — new projection (`ref.v1`). `ComputeRefRecords(issues)` is a pure function: builds a known-issues map + known-prefix set + closed-parent-of-open-child map once, then scans each source issue's deps (unresolved `external:` form only — resolved ones become normal graph edges via `rc.analysisIssues()`), description, notes, and joined comments. Records sort by `(Source, Target, Location)` for diff stability. `RefRecord` carries `{source, target, location, flags}` with location ∈ `{description, notes, comments, deps}`.

- **Cross-project only in v1** — same-prefix refs are intra-project and already handled by the dep graph. This is a scope tightening from the AC's literal "scan all refs" — eliminates the largest false-positive class (suffix collisions within a project).

- **Prefix scoping** — on top of the cross-project filter, prose matches are restricted to prefixes present in the loaded issue set. Slug-shaped tokens like `round-trip`, `per-issue`, `cross-project`, `batch-closing` split into valid `(prefix, suffix)` but their "prefix" corresponds to no known project, so they drop out. Dogfooded: this cut the broken-ref false-positive rate from ~85% (naive regex) to ~30% (residual comes from placeholder text like `bt-xxx`, `cass-xyz` in docs).

- **Flag order (fixed, for diff stability)**: `broken`, `stale`, `orphaned_child`, `cross_project`. Every v1 record carries `cross_project` because v1 only emits cross-project refs — kept explicit so v2 can relax the identity rule without changing the flag's meaning.

- **URL stripping** — `https?://...` spans get replaced with whitespace before the regex runs, so `github.com/foo-bar/baz` doesn't fire `foo-bar`. Markdown-aware parsing (code blocks, inline code) deferred to v2.

- **`cmd/bt/robot_refs.go`** — `rc.runRefs()` handler with a pure `refsOutput()` helper that builds the wire payload without hitting stdout or `os.Exit`. Same `BT_TEST_MODE=1`-driven pattern as pairs: binary-level `--global` tests can't run through Dolt discovery, so the helper lets in-process contract tests exercise the projection end-to-end.

- **Mandatory `--global`** — without cross-project data, ref detection is definitionally empty. Exits 1 with a clean error message rather than silently emitting `[]`.

- **Cobra wiring** in `cmd/bt/cobra_robot.go` — `robotRefsCmd` registered alongside `pairs`. No new flags. `--shape` is inherited but no-op (envelope.schema is unconditionally `ref.v1`).

Full rationale: `docs/design/2026-04-20-bt-mhwy-3-refs.md`.

### Tests

- `pkg/view/ref_record_test.go` — 18 unit cases: empty/nil safety, same-prefix skipped, cross-project found, broken, stale, orphaned_child, external-dep resolved vs broken, dedup within a single location, multiple locations per source, malformed IDs silently skipped, URL stripping, word boundaries (`see bd-a.`, `(bd-a)`, `bd-a,` match; `abt-a`, `x-bd-a` don't), dotted suffix (`bd-mhwy.2`), flag-order across multiple co-occurring flags, deterministic sorting, schema constant, unknown-prefix skipped.
- `pkg/view/projections_test.go` — 4 new golden fixtures exercised via `TestRefRecordGolden`: `ref_empty`, `ref_single_broken`, `ref_mixed` (broken + stale + orphaned_child + valid), `ref_external_deps` (resolvable vs broken `external:` forms). Plus `TestRefRecordSchemaFileExists` for the committed JSON Schema.
- `cmd/bt/robot_refs_test.go` — contract: `--global` enforcement (binary), envelope required fields, `schema == "ref.v1"`, cross-project-only (intra-project ref in fixture must not leak), flag order across the full fixture, empty array on no refs, deterministic ordering.
- `cmd/bt/robot_all_subcommands_test.go` — refs added to flag-acceptance matrix (4 permutations; `--global` carried).

### Smoke

`bt robot refs --global` against the real shared server returns 408 ref records across 20+ projects: 119 `broken`, 116 `stale`, 21 `orphaned_child`. Residual false-positive rate on `broken` (~30%) comes from placeholder text in docs (`bt-xxx`, `cass-xyz`, etc.) and slug-like suffixes under known prefixes (`-only`, `-side`, `-show`). Stale and orphaned_child flags are high-signal because the target actually exists — agents can filter to those for a cleaner pass. Sigil-based v2 identity (require `ref:` keyword or markdown link) is the natural next step and the follow-up bead filed in this session.

### Cross-project constellation (status)

Shipped in this session series: **bt-mhwy.5** (external dep resolution) → **bt-mhwy.4** (portfolio) → **bt-mhwy.2** (pairs) → **bt-mhwy.3** (refs). Six of seven mhwy children now closed. Remaining: **bt-mhwy.6** (provenance `--source` filter — scope shrunk by earlier children: `source_repo` already surfaces in compact/portfolio/pair output, so .6 is primarily the filter flag + schema docs).

---

## 2026-04-20 - Pairs subcommand (bt-mhwy.2)

**New `bt robot pairs --global` subcommand surfaces cross-project paired beads.** When `bt-zsy8` and `bd-zsy8` describe the same logical work from two projects, pairs detects the relationship, picks a canonical, and reports which dimensions have drifted. Agents can create paired IDs today (`bd create --id=<prefix>-<suffix>`) — this is the missing read path.

### What shipped

- **`pkg/view/pair_record.go`** — new projection (`pair.v1`). `ComputePairRecords(issues)` is a pure function: buckets by ID suffix, filters to groups with ≥2 distinct prefixes, sorts by `CreatedAt` (tie-broken by prefix, then full ID) to pick canonical, compares mirrors against canonical for drift, returns records sorted by suffix. `PairRecord` carries `{suffix, canonical, mirrors, drift?}`; `PairMember` is a 5-field compact slot (id/title/status/priority/source_repo).

- **Drift dimensions (v1, fixed output order)**: `status`, `priority`, `closed_open`, `title`. `closed_open` is a sharper sub-signal that always co-occurs with `status` when either side straddles the closed boundary, letting agents filter directly for "one side shipped, other didn't." Title drift is exact string equality — deliberate "no fuzzy" interpretation of the AC (matches bt-mhwy.5's philosophy).

- **`pkg/analysis.SplitID`** (promoted from private `splitID`) — pair detection is the second legitimate consumer of the same primitive. Reimplementing in `pkg/view` would have risked the two definitions drifting. Exporting costs nothing: one private call site, one test, mechanical rename.

- **`cmd/bt/robot_pairs.go`** — `rc.runPairs()` handler with a pure `pairsOutput()` helper that builds the wire payload without hitting stdout or `os.Exit`. The separation lets in-process contract tests exercise the projection end-to-end: binary-level `--global` tests can't run in `BT_TEST_MODE=1` (which deliberately blocks Dolt discovery).

- **Mandatory `--global`** — pair detection is definitionally empty without cross-project data; invoking without `--global` exits 1 with a clean error message rather than silently emitting `[]` (which would collide with the legitimate "no pairs exist" signal). With `--global` set and no pairs in the data, emits `"pairs": []` and exits 0 per the AC.

- **Cobra wiring** in `cmd/bt/cobra_robot.go` — `robotPairsCmd` registered alongside portfolio. No new flags. `--shape` is inherited but no-op (envelope.schema is unconditionally `pair.v1`).

Full rationale: `docs/design/2026-04-20-bt-mhwy-2-pairs.md`.

### Tests

- `pkg/view/pair_record_test.go` — 10 unit cases: empty/nil safety, in-sync single pair, drift across each dimension + fixed output order, 3-way pair ordering, canonical tie-break, same-prefix anomaly dropped, malformed IDs silently skipped, dotted suffix (`bt-mhwy.2` + `bd-mhwy.2`), records sorted by suffix, schema constant.
- `pkg/view/projections_test.go` — 4 new golden fixtures via `TestPairRecordGolden`: `pair_empty`, `pair_single_in_sync`, `pair_single_drifted`, `pair_multi_way`. Plus `TestPairRecordSchemaFileExists` for the committed JSON Schema.
- `cmd/bt/robot_pairs_test.go` — contract: `--global` enforcement (binary), envelope required fields, `schema == "pair.v1"`, drift detection across all 4 dimensions, empty array on no pairs, pairs sorted by suffix, canonical = first-created with prefix-sorted mirrors.
- `cmd/bt/robot_all_subcommands_test.go` — pairs added to flag-acceptance matrix (4 permutations; `--global` carried since pairs requires it).

### Smoke

`bt robot pairs --global` against the real shared server returns 29 paired sets across 14+ projects, including the known `bt-zsy8`/`bd-zsy8` pair. 24 of the 29 surface drift — mostly title divergence (expected: mirrored beads typically get project-scoped titles). Three-way pairs (`byk`, `fjip`, `g5q`) surface cleanly with canonical + 2 mirrors.

### Cross-project constellation (status)

Shipped in this session series: **bt-mhwy.5** (external dep resolution) → **bt-mhwy.4** (portfolio) → **bt-mhwy.2** (pairs). Next in sequence per the epic: `bt-mhwy.3` (refs — cross-project reference resolution, consumes the pair pattern), then `bt-mhwy.6` (provenance surfacing).

---

## 2026-04-20 - Portfolio subcommand (bt-mhwy.4)

**New `bt robot portfolio` subcommand answers "which project needs attention?" at the org level.** One PortfolioRecord per project with counts, priority breakdown, velocity with trend, composite health score, top blocker, and stalest issue.

### What shipped

- **`pkg/view/portfolio_record.go`** — new projection (`portfolio.v1`). `ComputePortfolioRecord(project, projectIssues, allIssues, pagerank, now)` is a pure function; `allIssues` lets the Blocked count see cross-project blockers under `--global` after bt-mhwy.5 external dep resolution.

- **Shared reverse-map helpers** — extracted `buildChildrenMap`, `buildUnblocksMap`, `buildOpenBlockersMap` from `CompactAll`'s single-pass loop. Both `CompactAll` and `ComputePortfolioRecord` consume them; behavior-identical refactor (CompactIssue golden fixtures unchanged).

- **`cmd/bt/robot_portfolio.go`** — `rc.runPortfolio()` handler. Groups issues by `SourceRepo` under `--global`; single-project mode emits exactly one record keyed by `rc.repoName` (falls back to a uniform SourceRepo, then `"local"`). Empty SourceRepo in global mode buckets to `"unknown"` so agents never lose data.

- **Cobra wiring** in `cmd/bt/cobra_robot.go` — `robotPortfolioCmd` registered alongside other robot subcommands. No new flags. `--shape` is inherited but no-op (envelope.schema is unconditionally `portfolio.v1` because the payload IS a versioned projection).

### Design

- **Health formula**: equal-weight mean of `closure_ratio`, `(1 − blocker_ratio)`, `(1 − stale_norm)` with clamping to `[0,1]` and 3-decimal rounding. Simple, explainable, no magic weights.
- **Trend classifier**: recent 2-week window vs prior 4-week window normalized to 2-week-equivalent, with ±20% thresholds — smoother than raw week-over-week.
- **Top blocker**: PageRank among project-scoped open/in_progress issues with `unblocks_count > 0` — excludes isolated leaves with high PageRank that aren't holding anyone hostage.

Full rationale in `docs/design/2026-04-20-bt-mhwy-4-portfolio.md`.

### Tests

- `pkg/view/portfolio_record_test.go` — unit tests for empty project, counts, trend classifier with boundary cases (±20%), health-score formula, top-blocker isolated-leaf filter, stalest selection.
- `pkg/view/projections_test.go` — 4 new golden fixtures exercised via `TestPortfolioRecordGolden`: empty, single healthy, single unhealthy, multi-project (cross-project blocker).
- `cmd/bt/robot_portfolio_test.go` — contract: envelope shape, `schema == "portfolio.v1"` across all `--shape` variants, `--shape=compact` ≡ `--shape=full` byte-identical (no-op), single-project mode returns exactly one record, projects sorted by name.
- `cmd/bt/robot_all_subcommands_test.go` — portfolio added to the flag-acceptance matrix (4 permutations).

### Smoke

`bt robot portfolio --global` ranks 15 real projects side-by-side with sensible health scores (0.464–0.985), per-project trends, and cross-project TopBlocker detection.

---

## 2026-04-20 - Compact output for robot subcommands (bt-mhwy.1)

**Default `bt robot list` output shape changes from full issues to compact projections.** 3 commits, 1 new package (`pkg/view/`), 1 bellwether integration, 1 compact projection for `robot diff`, 70+ new tests.

### Breaking change (pre-alpha)

- **Default `bt robot list` shape is now `compact`.** Full-body output is opt-in via `--full` (or `--shape=full`, or `BT_OUTPUT_SHAPE=full`). Rationale: `bt robot list --global` dropped from 383KB to 38KB on a 100-issue sample (~90% reduction) — agents were burning context windows on `description`/`design`/`acceptance_criteria`/`notes`/`comments`/`close_reason` bodies they never read.

### What shipped

- **`pkg/view/` package** (Commit 1) - Home for graph-derived consumer-facing projections. `CompactIssue` is the first resident. Ships with a reusable golden-file harness (`projections_test.go`), a committed JSON Schema (`schemas/compact_issue.v1.json`), and projection-pattern conventions in `doc.go`. Future projections (portfolio records, pair records, reference records) follow the same file-per-projection, schema-versioned pattern.

- **`robot list` bellwether** (Commit 2) - Persistent `--shape` / `--compact` / `--full` flags on `robotCmd` (inherited by every subcommand) with `BT_OUTPUT_SHAPE` env var. New `schema` field on `RobotEnvelope` (`omitempty`) carries `"compact.v1"` in compact mode and is absent in full mode, keeping `--full` byte-identical to pre-change output. Compact projection computed over the full pre-filter issue set so reverse-graph counts (`children_count`, `unblocks_count`, `is_blocked`) reflect the real graph regardless of `--status` / `--priority` / `--type` / `--has-label` / `--limit`.

- **`robot diff` compact projection** (Commit 3) - Projects the four `[]model.Issue` slots on `analysis.SnapshotDiff` (`new_issues`, `closed_issues`, `removed_issues`, `reopened_issues`) into `[]view.CompactIssue` when `shape=compact`. Reverse-graph counts computed over the UNION of historical and current issues so `children_count` / `unblocks_count` / `is_blocked` stay accurate across snapshots. `--full` keeps the original `*analysis.SnapshotDiff` wire shape.

- **15 other robot subcommands** - `triage`, `next`, `insights`, `plan`, `priority`, `alerts`, `search`, `suggest`, `drift`, `blocker-chain`, `impact-network`, `causality`, `related`, `impact`, `orphans` all inherit the persistent `--shape` flag and accept it without flag-parse errors. These subcommands' outputs use purpose-built wrapper types (`Recommendation`, `TopPick`, `PlanItem`, `EnhancedPriorityRecommendation`, `BlockerChainEntry`, `NetworkNode`, `RelatedWorkBead`, `AffectedBead`, `CausalChain`, `OrphanCandidate`) that are already compact-by-construction and emit no fat body fields, so no per-subcommand projection was needed.

### Flag resolution order

1. `--shape=compact` / `--shape=full` (explicit)
2. `--compact` / `--full` (alias; errors if combined with conflicting `--shape`)
3. `BT_OUTPUT_SHAPE` env var
4. `compact` default

### Tests

- `pkg/view/compact_issue_test.go` — unit (7 cases): nil/empty safety, field copying, labels aliasing, reverse-map correctness, `is_blocked` semantics across open/closed/in-progress/external blockers, `relates_count` local-only, metadata bridge, schema-constant check.
- `pkg/view/projections_test.go` — golden-file harness exercising 5 fixtures (minimal, fully-populated, blocked, epic-with-children, global-multiproject). Regenerate with `GENERATE_GOLDEN=1`.
- `cmd/bt/robot_compact_flag_test.go` — 14 flag-resolution cases (defaults, explicit, aliases, env, conflicts, bad values).
- `cmd/bt/robot_list_compact_test.go` — contract suite: no forbidden body fields leak, `--full` restores bodies, all flag/env permutations resolve consistently, reverse-graph counts (`is_blocked`, `parent_id`, `blockers_count`, `relates_count`), `--full` key regression.
- `cmd/bt/robot_all_subcommands_test.go` — 64 subtests across 16 subcommands × 4 flag permutations verifying flag acceptance, plus compact/full contract tests for `robot diff`.

### Blocks / unblocks

- **Unblocks**: bt-mhwy.2 (pairs), bt-mhwy.3 (refs), bt-mhwy.4 (portfolio), bt-mhwy.5 (external dep resolution), bt-mhwy.6 (provenance surfacing).
- **Prerequisites** (both landed earlier this session): bt-uc6k (schema-drift audit), bt-mhwy.0 (column catchup for `metadata` + `closed_by_session`).

---

## 2026-04-14 - Quick wins, footer extraction, label picker redesign

**Bug fixes + footer decomposition + label picker UX overhaul.** 17 commits, 4 bugs fixed, 1 refactor, 12 new tests.

### Bug fixes

- **Label picker freeze** (bt-eorx, P1) - Label picker lacked the early-return pattern used by other modals. Typed characters (g, i, a) were intercepted by global handlers that triggered expensive operations on 2500+ issues. Fix: added early return for ModalLabelPicker before global key handlers.

- **Status bar message not displaying** (bt-6k0f, P2) - `handleKeyPress` cleared `statusMsg` on every keypress but did not reset `statusSetAt`. New messages set via direct assignment had a stale timestamp from a previous message, causing `renderFooter`'s auto-dismiss to clear them before they rendered. Fix: reset `statusSetAt` in the clear-on-keypress block. Also migrated y-key copy handlers to use `setStatus()`/`setStatusError()`.

- **Label dashboard leaves split view disabled** (bt-trqo, P2) - Global `esc` handler for ViewLabelDashboard set `mode=ViewList` but forgot to restore `isSplitView=true`. Global `q` handler had no ViewLabelDashboard check at all (fell through to `tea.Quit`). Fix: added `isSplitView=true` to both global handlers.

### Refactor

- **Footer extraction** (bt-oim6, P2) - Extracted 650-line `renderFooter()` into `FooterData` value struct + `Render()` method. `Model.footerData()` extracts ~35 Model fields into plain values, `FooterData.Render()` does pure rendering with no Model access. 12 tests cover status bar, badges, worker levels, alerts, time travel, hint truncation.

### Skipped

- **bt-8jds** (wisp toggle key conflict) - Blocked by bt-tkhq (keybinding research, human gate). Both `w` and `W` are taken, needs keybinding audit before choosing a new key.

### Refactor epic status (bt-if3w)

5/7 children complete (oim6 closed this session). Remaining: bt-t82t (stale refs/golden files), bt-if3w.1 (sprint view extraction).

### Label picker redesign (bt-36h7, dogfooded)

- **Overlay compositing** - converted from full-screen replacement to OverlayCenter overlay, matching project filter pattern
- **RenderTitledPanel** - round borders with "Filter by Label" title in border
- **Search input** - all letter keys go to text input (no j/k/h/l navigation conflicts), arrow keys only for nav
- **Multi-select** - space toggles labels (checkmarks), enter applies compound OR filter
- **Composing filters** - label filter is now independent of status filter (open + area:tui works)
- **Selected labels pinned** - toggled labels stay at top of list even when filtered by search
- **Stable modal** - fixed width (computed from all labels), fixed height (padded to maxVisible), page-aligned windowing
- **Page navigation** - left/right arrows, PageUp lands at top, PageDown at bottom

### UX improvements

- **Filter toggle** - o/c/r keys now toggle (press again to revert to "all")
- **Sort cycle** - reordered to updated -> created newest -> created oldest -> priority
- **Esc clears everything** - status filter, label filter, sort mode, search all reset on esc

**All tests pass. Build clean.**

---

## 2026-04-13 - Beads Feature Surfacing Wave 4: Wisps, Swarm, Capabilities (bt-9kdo, bt-1knw, bt-t0z6)

**Final session of the 4-wave feature surfacing plan.** 3 commits (parallel subagents), 740 lines added across 12 files, 20 new tests.

### What shipped
- **Wisp visibility toggle** (bt-9kdo) - `w` key hides/shows ephemeral issues. Default: hidden (matches `bd ready`). Wisps render dimmed+italic when visible. Footer badge shows state. Filter applied across all view paths (list, board, graph, BQL, recipes).
- **Swarm wave visualization** (bt-1knw) - `s` key in graph view shells to `bd swarm validate --json`, colors nodes by wave (green=wave 0/ready, yellow=wave 1, blue=wave 2+). Metrics panel shows wave position, max parallelism, estimated sessions. 5-second timeout with graceful error handling.
- **Capability map** (bt-t0z6) - Parses `export:`, `provides:`, `external:<project>:<cap>` labels. Detail panel shows capabilities section in workspace/global mode. `aggregateCapabilities()` builds cross-project edge graph with unresolved dependency detection.

### Key design decisions
- Wisp `w` key reuses the existing global handler - fires wisp toggle in non-workspace mode, project picker in workspace mode
- Swarm data loaded via `exec.CommandContext` (same pattern as other bd integrations) - no direct Dolt writes
- Capability map is a detail panel section, not a new ViewMode - lower effort, 80% of the value

### Parent epic: bt-53du (beads feature surfacing)
All 4 waves complete. Sessions 0-1 (data model), Session 2 (gate indicators), Session 3 (stale/epic/state dims), Session 4 (wisps/swarm/capabilities).

---

## 2026-04-12 - Temporal Infrastructure: Dolt AS OF queries + TemporalCache (bt-ph1z.7)

**Foundation for cross-project trending features.** 4 commits, 955 lines added across 8 files, 13 new tests.

### What shipped
- **`LoadIssuesAsOf(timestamp)`** on `GlobalDoltReader` - queries each database individually using Dolt `AS OF` syntax. Per-database error handling: if one database has no commit at the requested timestamp, it's skipped with a warning (others still load).
- **`TemporalCache`** in `pkg/analysis/temporal.go` - stores `map[time.Time][]model.Issue` snapshots. TTL-based staleness (default 1hr, configurable via `BT_TEMPORAL_CACHE_TTL`). Max 30 snapshots cap (`BT_TEMPORAL_MAX_SNAPSHOTS`). Concurrent populate guard. Oldest-first eviction.
- **`SnapshotMetrics`** - lightweight summary struct (open/blocked/closed counts, 7-day velocity) computed per snapshot. `ComputeMetricsSeries()` produces a time-ordered series from cache data.
- **Background worker integration** - `startTemporalCacheLoop()` goroutine runs on the cache TTL cadence (hourly), independent of the 3-second UI poll. 5-second startup delay to avoid competing with main data load. `TemporalCacheReadyMsg` notifies the UI.
- **`DataSnapshot.TemporalCache`** field carries the cache reference to the UI layer.

### Key design decisions
- Per-database queries (not UNION ALL) for AS OF - databases have different commit histories
- Background goroutine separate from poll loop - slow cadence, own connection
- `IssueLoader` interface on `TemporalCache.Populate()` - testable without a live Dolt server
- Timestamps, not commit refs - simpler across databases with different commit cadences

### What this unlocks
- bt-ph1z.2: Sparkline snapshots (needs TemporalCache data)
- bt-ph1z.3: Diff mode (needs LoadIssuesAsOf for two-snapshot comparison)
- bt-ph1z.4: Timeline view (needs SnapshotMetrics series)

**All 1483 package tests pass. Build clean.**

---

## 2026-04-10c - Phase 2 + Phase 3: Theme redesign + Cobra CLI (bt-k5zs, bt-oim6, bt-zt9q)

**Two parallel refactors shipped**: Phase 2 (theme/color system) and Phase 3 (CLI structure) executed as parallel worktree agents since they touch disjoint file sets (pkg/ui/ vs cmd/bt/).

### Phase 2: AdaptiveColor kill + resolved color system (bt-k5zs)
- **174 `AdaptiveColor` occurrences eliminated** across 25 files. All color fields now use `color.Color` (resolved at load time based on `isDarkBackground`).
- **Dark mode detection**: `tea.BackgroundColorMsg` in Init()/Update() - the canonical Charm v2 pattern. Replaces the Phase 0 shim that defaulted to dark.
- **Theme struct redesigned**: All color fields changed from `AdaptiveColor` to `color.Color`. `resolveColor(light, dark)` helper resolves based on `isDarkBackground`.
- **styles.go**: All 52 package-level `Color*` vars changed to `color.Color`. New `resolveColors()` function rebuilds everything when dark/light changes.
- **theme_loader.go**: `AdaptiveHex.toColor()` resolves at load time. Fallback maps provide light/dark defaults for partial YAML overrides.
- **Glamour**: Style selection now dynamic (`"dark"`/`"light"`) based on `isDarkBackground`.
- **`adaptive_color.go` deleted**. The Phase 0 compatibility shim is gone.

### Phase 3: Cobra CLI migration (bt-zt9q)
- **main.go: 1,708 -> 13 lines**. Just `rootCmd.Execute()`.
- **Cobra subcommand tree**: `bt robot triage`, `bt robot graph`, `bt export pages`, `bt agents add`, `bt baseline check`, `bt version`, etc.
- **35+ robot subcommands** migrated from `--robot-*` flags to `bt robot *` subcommands.
- **Bare `bt` launches TUI** (not help). Uses `rootCmd.Run` + `SilenceUsage: true`.
- **Data loading deferred**: Only commands that need data call `loadIssues()`. `bt version`, `bt robot recipes`, `bt robot schema` skip it entirely.
- **Clean break**: No backward compat for old `--robot-*` flags (pre-alpha, one consumer).
- **Tests updated** for new subcommand syntax.

**Steps deferred to Phase 4 (bt-t82t)**: Pre-compute hot-path styles (optimization, needs profiling), footer extraction as FooterData (Phase 1.5, bt-oim6 - separate decomposition concern).

**All tests green. Build clean. 26 packages pass.**

---

## 2026-04-10b - Phase 1: Model decomposition (bt-98v9)

**Core refactor shipped**: 4 commits, 21 files, 3,235 insertions / 3,030 deletions.

**Step 1.1 - ViewMode enum**: Replaced 7 mutually exclusive boolean view flags (`isBoardView`, `isGraphView`, etc.) with an 11-value `ViewMode` enum. All routing (View(), Update(), key dispatch) now switches on `m.mode`.

**Step 1.2 - State extraction**: Moved ~50 fields from Model into focused sub-structs: `DataState` (pointer, issues/snapshot/worker), `FilterState` (pointer, filters/BQL/recipes), `AnalysisCache` (pointer, triage scores/counts). `DoltState` and `WorkspaceState` embedded as value types. Model copy per frame: ~1.6KB -> ~240 bytes.

**Step 1.3 - Modal state**: Replaced 19 `show*` booleans with single `activeModal ModalType` enum (16 values). Added `modalActive()`, `openModal()`, `closeModal()` helpers.

**Step 1.4 - Update() decomposition**: Split 2,387-line Update() into 147-line thin router + 3 handler files: `model_update_data.go` (871 lines), `model_update_input.go` (1,217 lines), `model_update_analysis.go` (348 lines). model.go: 3,684 -> 1,438 lines.

**Step 1.5 deferred**: Footer extraction (bt-oim6) - `model_footer.go` touches 35+ Model fields. Natural to bundle with Phase 2 theme redesign.

**Process**: 2 worker agents, ~65 min wall clock. Worker 1 exhausted context on a sed overshoot (replaced `m.issues` in FlowMatrixModel/InsightsModel receivers). Monitor caught it early. Worker 2 finished cleanly through Step 1.4.

**All 24 test packages green. Build clean.**

---

## 2026-04-03c - Global hub data layer (bt-6wbd phase 1)

**GlobalDoltReader shipped**: `internal/datasource/global_dolt.go` - connects to shared Dolt server without a database in the DSN, enumerates all beads project databases, loads issues via UNION ALL with backtick-quoted `database.table` syntax.

**Key implementation**:
- `DiscoverSharedServer()` reads `~/.beads/shared-server/dolt-server.port`, env override via `BT_GLOBAL_DOLT_PORT`
- `EnumerateDatabases()` uses `information_schema.tables` (single query, not N validation queries), filters system DBs
- `LoadIssues()` via UNION ALL across all databases, `SourceRepo` set from database name (overrides column)
- Batch labels/deps/comments via 3 UNION ALL queries (not N+1 per-issue)
- `GetLastModified()` via aggregated `MAX(MAX(updated_at))` across all databases
- Partial failure: broken DBs skipped with `slog.Warn`, healthy DBs loaded

**Source type integration**: `SourceTypeDoltGlobal` added to source.go, `RepoFilter` field on `DataSource`, `LoadFromSource` dispatch case in load.go.

**Poll loop**: `globalDoltPollOnce()` in background_worker.go, dispatched when source type is `SourceTypeDoltGlobal`. Reconnect does TCP dial only (no auto-start, shared server is user-managed).

**CLI**: `--global` flag, mutually exclusive with `--workspace` and `--as-of`. `--repo` filters database list at enumeration (before UNION ALL). Workspace mode UI activates automatically (badges, picker, prefilter).

**Shared column list**: Extracted `IssuesColumns` constant to `columns.go`, used by both `DoltReader` and `GlobalDoltReader`.

**Tests**: 16 new unit tests in `global_dolt_test.go` (query building, system DB filtering, backtick quoting, discovery, DSN construction). Full suite: 27 packages, 0 failures.

**Files created**: `internal/datasource/global_dolt.go`, `internal/datasource/global_dolt_test.go`, `internal/datasource/columns.go`
**Files modified**: `internal/datasource/dolt.go`, `internal/datasource/source.go`, `internal/datasource/load.go`, `pkg/ui/background_worker.go`, `cmd/bt/main.go`

**Bead closed**: bt-6wbd

## 2026-04-03b - BQL bug fixes + global hub planning

**BQL bugs (bt-bjk4)**: Fixed all 5 bugs from gap analysis:
1. Status enum validation - added `ValidStatusValues` map, catches typos like `status=opne`
2. `--robot-bql` envelope - now uses `RobotEnvelope` + `robotEncoder` (adds metadata, TOON support)
3. Dead code removal - removed unused `WithReadySQL` from sql.go
4. Date equality semantics - `created_at = today` now matches any time on that day (truncates to midnight)
5. ISO date parsing - `created_at > 2026-01-15` now works in lexer, parser, and executor

Tests added for all fixes. Full suite passes (27 packages, 0 failures).

**Triage**: bt-dx7k reopened (blocked, not in-progress), bt-28g8 closed (audit done), bt-2bns deferred (Charm v2), bt-xft1 closed (resolved by shared server architecture).

**Global hub design verification**: Verified 5 assumptions from the beads session's design doc against actual codebase. Updated open questions with findings. Key correction: poll system needs real refactoring, not just a query swap.

**Global hub data layer plan**: `docs/plans/2026-04-03-feat-global-hub-data-layer-plan.md` - 4-phase implementation plan for GlobalDoltReader. Batch N+1 queries into 3 UNION ALL, single aggregated MAX for poll, --global flag, workspace UI reuse.

**Beads closed**: bt-bjk4 (BQL bugs), bt-28g8 (keybinding audit), bt-xft1 (data separation)
**ADR-002 updated**: Stream 2 bugs all checked off, Stream 1 robot-bql checked off

## 2026-04-03 - Parallel audit swarm

Burned expiring weekly credits on 5 parallel research agents. All read-only, no code changes.

**Reports produced**:
- `docs/audit/test-suite-audit.md` - 268 test files: 93% KEEP, 0% REMOVE, 1 Windows P1
- `docs/audit/cli-ergonomics-audit.md` - 97 flags inventoried, 3 critical robot-mode envelope bugs
- `docs/audit/charm-v2-migration-scout.md` - 76 files affected, 60% mechanical, theme system is the hard part
- `docs/audit/bql-gap-analysis.md` - corrected stale memory (--bql/--robot-bql already shipped), found 5 bugs
- `docs/drafts/README-draft.md` - complete prose rewrite draft

**Beads closed**: bt-79eg (test audit), bt-pfic (CLI audit)
**Beads created**: bt-0cht (P1, robot-mode fixes), bt-5dvl (P2, test fixes), bt-bjk4 (P2, BQL bugs), bt-iuqy (P2, README review)
**ADR-001 closed out**, ADR-002 created as new project spine. Changelog extracted to this file.

## 2026-04-01 - BQL composable search

New package `pkg/bql/` - BQL parser vendored from zjrosen/perles (MIT), adapted for bt.

- Parser layer: lexer, parser, AST, tokens, validator, SQL builder (~1,500 LOC)
- MemoryExecutor: in-memory evaluation against model.Issue (522 LOC, 28 tests)
- TUI integration: `:` keybind opens BQL modal, dedicated `applyBQL()` filter path
- CLI: `--bql` and `--robot-bql` flags
- Syntax: =, !=, <, >, <=, >=, ~, !~, IN, NOT IN, AND/OR/NOT, parens, P0-P4, date literals, ORDER BY, EXPAND

22 files, ~3,950 lines, 27 packages pass, 0 failures.

## 2026-03-16b - Cross-platform test suite fixes

39 failing Windows tests -> 0 failures across all 26 packages.

- Phase 1: Renamed bv->bt stragglers in 8 files
- Phase 1b: Fixed ComputeUnblocks (filter blocking edges only), slug collision expectations
- Phase 2: filepath.FromSlash/Join in cass and tree test expectations
- Phase 3: configHome override for tutorial progress + wizard config (HOME env doesn't work on Windows)
- Phase 4: runtime.GOOS skip guards for 6 Unix-only permission tests
- Phase 5: Shell-dependent hooks tests skipped on Windows; fixed -r shorthand conflict
- Phase 6: .exe suffix for drift test binaries; file locking fix (defer order)
- Phase 7: Normalized \r\n in golden file comparison

**Closed**: bt-s3xg, bt-zclt, bt-3ju6, bt-7y06, bt-ri5b, bt-dwbl, bt-kmxe, bt-mo7r (8 issues)

## 2026-03-16a - Dolt lifecycle adaptation

New module `internal/doltctl/` for Dolt server management.

- EnsureServer: detects running server (TCP dial) or starts via `bd dolt start`
- StopIfOwned: PID-based ownership check before `bd dolt stop`
- Auto-reconnect: poll loop retries EnsureServer after 3 consecutive failures
- Port discovery chain: BEADS_DOLT_SERVER_PORT > BT_DOLT_PORT > .beads/dolt-server.port > config.yaml > 3307
- Database identity check: `SHOW TABLES LIKE 'issues'` after connecting
- Dead code removed: touchDoltActivity keepalive

11 doltctl tests + 6 metadata tests. **Closed**: bt-07jp (P1), bt-tebr (P2, subsumed)

## 2026-03-12 - Brainstorm + audit planning

No code changes. Post-takeover roadmap brainstorm + codebase audit design.

- Defined 4 phases: Audit -> Stabilize -> Polish -> Interactive
- Key decision: CRUD via bd shell-out (no beads fork needed)
- Designed 8-team parallel codebase audit (~190k LOC)
- Created 8 dogfood beads from TUI usage
- Docs: `docs/brainstorms/2026-03-12-post-takeover-roadmap.md`, `docs/plans/2026-03-12-codebase-audit-plan.md`

## 2026-03-11b - Dolt freshness + responsive help

- **bt-3ynd**: Fixed false STALE indicator - freshness tracks last successful poll, not snapshot build time
- **bt-aog1**: Responsive help overlay - 4x2 grid (wide), 2x4 (medium), single column (narrow)
- **bt-xavk**: Created help system redesign plan (docs/plans/help-system-redesign.md)

## 2026-03-11a - Dogfood polish

- Absolute timestamps in details pane + expanded card
- Priority shows P0-P4 text next to icon
- Status bar auto-clear after 3s
- Help overlay: centered titles, auto-sized panels, 4x2 grid, status indicators panel
- Board: auto-hide empty columns on card expand
- Shortcut audit: found 22 undocumented keys

## 2026-03-07 - Beads migration

Renamed issue prefix bv->bt (553 issues). Set beads.role=maintainer. Local folder renamed. Memory migrated.

## 2026-03-05c - ADR review cleanup

Fixed 14 stale `bv` CLI refs in AGENTS.md. Fixed insights detail panel viewport off-by-one.

## 2026-03-05b - Titled panels

Converted insights, board, and help overlay to RenderTitledPanel. Added BorderColor/TitleColor overrides. Board cards use RoundedBorder + border-only selection.

## 2026-03-05a - Tomorrow Night theme

Visual overhaul: Tomorrow Night + matcha-dark-sea teal. Theme config system (embedded defaults, layered loading). TitledPanel helper. Swapped all Color* vars. 18 new tests.

## 2026-02-25 to 2026-03-04 - Fork takeover

See [ADR-001](docs/adr/001-btui-fork-takeover.md) for detailed session-by-session changelog of the fork takeover work (streams 1-4: Dolt verification, rename, data migration, spring cleaning).
