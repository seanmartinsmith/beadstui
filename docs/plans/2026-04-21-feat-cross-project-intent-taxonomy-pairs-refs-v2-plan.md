---
title: Cross-project intent taxonomy — pairs v2 + refs v2 readers
type: feat
status: active
date: 2026-04-21
origin: docs/brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md
beads: [bt-gkyn, bt-vxu9]
paired_beads: [mkt-gkyn, mkt-vxu9]
---

# Cross-project intent taxonomy — pairs v2 + refs v2 readers

## Enhancement Summary

**Deepened on:** 2026-04-21
**Sections enhanced:** 8 (added performance gates, security caveats, agent-native refinements, citation fixes)
**Review agents used:** code-simplicity-reviewer, performance-oracle, architecture-strategist, agent-native-reviewer, pattern-recognition-specialist, security-sentinel, superpowers:code-reviewer, git-history-analyzer

### Key revisions from review

1. **Sigil detector relocated** to `pkg/analysis/sigils.go` (was `pkg/view/sigil_detector.go`) — pkg/view is for projections, not parsers. Boundary correctness.
2. **Pair detection algorithm specified as BFS over connected components** (not "fully connected" which was ambiguous; not union-find which is overkill at N=400). One paragraph per the algorithm.
3. **Env vars renamed for consistency**: `BT_REFS_SIGILS` → `BT_SIGIL_MODE` (no subcommand-scoped prefix precedent); `BT_ROBOT_SCHEMA` → `BT_OUTPUT_SCHEMA` (matches `BT_OUTPUT_FORMAT` / `BT_OUTPUT_SHAPE`).
4. **Performance regression gate moved to Phase 3** (where the tokenizer ships) with concrete benchmark thresholds: 100KB body <5ms per mode, 10x scaling test, full corpus pass <800ms.
5. **Tokenizer constraints tightened**: iterative scanner with bounded depth (32-frame stack max), 1MB per-body input cap with `truncated: true` flag, panic-recover wrapper at the call site. Pathological-input tests beyond the 100KB single benchmark.
6. **Corpus sanitization rubric strengthened**: positive denylist scan (passwords/tokens/secrets/usernames/private URLs) as a pre-commit gate, close_reason fields included in scope, security-label skip is a fallback not the primary mechanism.
7. **`--orphaned` output is now agent-native**: emits JSONL records to stdout with `--robot-format=jsonl`, prose guidance to stderr. Agents can consume the migration checklist programmatically.
8. **Citation fix**: env-var docs map lives at `cmd/bt/robot_graph.go:224`, not `cmd/bt/cobra_robot.go:224`. Plan referenced the wrong path in three places.
9. **Deprecation notice dropped**: zero external consumers, CHANGELOG suffices. Removes test surface and a YAGNI corner.
10. **Rollback plan added** (Risk Analysis section).
11. **Load-bearing constraint surfaced**: the cross-project-only filter from v1 is NOT relaxed in v2 — git history confirms removing it regresses FP rate to ~85%. Documented explicitly.

### Decisions held against review

- **All three sigil modes (strict/verb/permissive)** — user explicitly chose during brainstorm despite YAGNI flag. Permissive mode shipping with documented trade-off rather than cut.
- **`--schema=v1` fallback retained for one release** — review flagged as ceremony with zero consumers, but the v1 v2 transition signal teaches the schema-bumping pattern for future evolutions. Cost is bounded.
- **Per-record `intent_source` field** — kept, but enum collapsed to actual emitted values (`"dep"` literal). Reserved-but-unused enum values dropped.

### New constraints discovered

- **Cross-project-only filter from v1 is load-bearing** (git history: removing regresses to ~85% FP). v2 must NOT relax this.
- **mkt-gkyn race risk**: if marketplace hook lands and starts auto-stamping deps before manual backfill is complete, we may double-stamp or race. Added to Risk Analysis.
- **Stderr deprecation notice + `2>&1` merging is fragile** — moot with the deprecation notice dropped.

---

## Overview

`bt robot pairs --global` and `bt robot refs --global` shipped in v1 as string-pattern detectors. Dogfooding on the real shared Dolt corpus (29 detected pairs with ~5× FP rate; 408 refs with ~30% broken-flag FPR) showed the signal isn't strong enough for reliable agent consumption. This plan lands v2: intent-based detection that filters on structural marks (dep edges for pairs) and syntactic marks (tunable sigil modes for refs), with explicit `--schema=v1` fallback and a new labeled-corpus FPR gate.

Two bt-side beads are the readers (`bt-gkyn`, `bt-vxu9`); two marketplace-side beads (`mkt-gkyn`, `mkt-vxu9`, filed in this session) own the hook + convention extension. This plan covers the bt-side work only.

## Problem Statement

### v1 dogfood numbers (the failure mode)

- **Pairs v1** (`bt robot pairs --global` on shared server, 2026-04-20): 29 detected pairs, ~24 fire `title` drift, ~5 are intentional. The ~24 are suffix collisions between unrelated work (`bt-153` "K-shortest critical paths" vs `mkt-153` "Rich description nudge hook"). Beads auto-generates 3–4 char suffixes per project; collisions across unrelated projects are common.
- **Refs v1** (`bt robot refs --global` on shared server, 2026-04-21): 408 records, 119 broken, ~30% broken-flag FPR. Residual comes from placeholder text (`bt-xxx`, `cass-xyz` in design docs) and English slugs under known prefixes (`-only`, `-side`, `-show`, `-level`).

Both readers are useful today (pairs for drift on the 5 intentional pairs; refs for stale/orphaned_child where target resolution succeeds) but not safely consumable at scale for broken/pair flagging.

### Root cause

String patterns lack intent. A suffix is not a pair. A prose mention is not a ref. Both v1s infer intent from structural coincidence. The v2 move is to require *explicit evidence* of intent:

- **Pair intent = structural** — a dep edge between the same-suffix cross-prefix beads (which the cross-project skill already prescribes aspirationally).
- **Ref intent = syntactic** — one of a documented set of prose sigils (markdown link, inline code, keyword, verb), tunable via `--sigils=strict|verb|permissive` with `BT_SIGIL_MODE` env override.

### Why readers only (not also hooks)

Enforcement (a PreToolUse hook that auto-stamps the dep edge + `Paired-With:` notes line on `bd create --id=<suffix>`) belongs in the marketplace repo as `mkt-gkyn` — it's the cross-project-collaborative layer. bt-side readers operate on whatever intent data exists; the ~5 known real pairs get manually dep-added before v2 becomes the default (forward-only backfill).

## Proposed Solution

**Ship pair.v2 + ref.v2 as schema-bumped readers that filter v1 output to intent-bearing records, preserving `--schema=v1` for one-release transitional pinning.** Add a tunable `--sigils` flag for refs with three documented modes. Introduce two test artifacts: existing-pattern goldens (regenerable) plus a new labeled corpus JSON fixture (`pkg/view/testdata/labeled_corpus.json`) driving an FPR-gate test asserting ≤5% broken FPR on refs and <10% total-pair FPR on pairs.

Key decision crystallized from brainstorm: **dep edge alone is the pair detection signal; the `Paired-With:` notes line is output-only provenance**, not an OR-channel. If the hook fails to stamp the dep, that's a hook bug to surface, not a reader workaround to hide (see brainstorm: `docs/brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md` §Key Decisions #4).

## Technical Approach

### Architecture

#### File layout

```
pkg/view/
├── pair_record.go                        amended — add ComputePairRecordsV2, PairRecordV2, PairRecordSchemaV2
├── pair_record_test.go                   amended — new test cases for v2 detection
├── ref_record.go                         amended — add ComputeRefRecordsV2, RefRecordV2, RefRecordSchemaV2
├── ref_record_test.go                    amended — new test cases for each --sigils mode
├── fpr_gate_test.go                      NEW — labeled-corpus FPR threshold test
├── projections_test.go                   amended — register pair_v2_* and ref_v2_* golden harnesses
├── schemas/
│   ├── pair_record.v1.json               unchanged
│   ├── pair_record.v2.json               NEW
│   ├── ref_record.v1.json                unchanged
│   └── ref_record.v2.json                NEW
└── testdata/
    ├── fixtures/                         NEW — pair_v2_*.json + ref_v2_{strict,verb,permissive}_*.json (~10 new)
    ├── golden/                           NEW — matching goldens (~10 new)
    └── corpus/labeled_corpus.json        NEW — ~30–50 sanitized real issues + truth labels

pkg/analysis/
├── sigils.go                             NEW — hand-rolled markdown-aware sigil parser (DetectSigils + SigilMode)
├── sigils_test.go                        NEW — unit + benchmark tests per sigil kind + pathological inputs

cmd/bt/
├── robot_pairs.go                        amended — wire --schema flag, dispatch v1/v2 in runPairs()/pairsOutput()
├── robot_pairs_test.go                   amended — new contract tests for --schema=v1|v2 and --orphaned
├── robot_refs.go                         amended — wire --schema + --sigils flags, dispatch in runRefs()/refsOutput()
├── robot_refs_test.go                    amended — new contract tests for --sigils and --schema
├── robot_schema_flag.go                  NEW — resolveSchemaVersion() helper mirroring robot_compact_flag.go
├── robot_sigils_flag.go                  NEW — resolveSigilsMode() helper mirroring robot_compact_flag.go
├── cobra_robot.go                        amended — register --schema + --sigils on pairs/refs subcommands
├── robot_graph.go                        amended — register env vars in robot-docs map at :224
└── robot_all_subcommands_test.go         amended — add flag-matrix rows for --schema and --sigils

.beads/conventions/
└── cross-project.md                      NEW — bt-side pointer doc with reader surface reference

docs/design/
└── 2026-04-21-pairs-refs-v2.md           NEW — implementation design doc + labeling rubric

CHANGELOG.md                              amended — session entry
docs/adr/002-stabilize-and-ship.md        amended — Stream 1 status line
```

**No `_v2.go` file variants** — per project CLAUDE.md rule. v2 functions live in the existing record files alongside v1 (`ComputePairRecordsV2`, `PairRecordV2`, etc.). `--schema=v1` routes to the unchanged v1 functions; `--schema=v2` routes to the new v2 functions. Dispatch happens in the handler.

### Data Contracts

#### pair.v2 envelope + record shape

```go
// PairRecordSchemaV2 = "pair.v2"
type PairRecordV2 struct {
    Suffix       string       `json:"suffix"`
    Canonical    PairMember   `json:"canonical"`
    Mirrors      []PairMember `json:"mirrors"`
    Drift        []string     `json:"drift,omitempty"`        // reduced: status, priority, closed_open (no title)
    IntentSource string       `json:"intent_source"`          // v2 emits "dep" literal — only currently-emitted value; new values added when new channels exist
}
```

Envelope adds `intent_source` at the top level:
```json
{
  "generated_at": "...",
  "data_hash": "...",
  "version": "...",
  "schema": "pair.v2",
  "intent_source": "dep",
  "pairs": [ ... ]
}
```

#### ref.v2 envelope + record shape

```go
// RefRecordSchemaV2 = "ref.v2"
type RefRecordV2 struct {
    Source     string   `json:"source"`
    Target     string   `json:"target"`
    Location   string   `json:"location"`
    Flags      []string `json:"flags"`        // broken, stale, orphaned_child, cross_project (unchanged)
    SigilKind  string   `json:"sigil_kind"`   // enum: markdown_link, inline_code, ref_keyword, verb, external_dep, bare_dep, bare_mention
}
```

Envelope adds `sigil_mode` at the top level:
```json
{
  "generated_at": "...",
  "data_hash": "...",
  "version": "...",
  "schema": "ref.v2",
  "sigil_mode": "verb",
  "refs": [ ... ]
}
```

#### --schema=v1 fallback semantics

- `--schema=v1` (or `BT_OUTPUT_SCHEMA=v1`) routes to existing `ComputePairRecords` / `ComputeRefRecords` unchanged.
- Envelope emits `schema: "pair.v1"` / `"ref.v1"` with no `intent_source` / `sigil_mode` fields.
- Records emit the v1 shape (no `intent_source` / `sigil_kind` fields).
- Retained for one release. Removal tracked as a new bead filed at v2 ship time.

### Flag wiring

Both new flags mirror the existing `--shape` resolver pattern (`cmd/bt/robot_compact_flag.go:34-70`).

#### `--schema` flag (on pairs + refs subcommands)

- Registered on `robotPairsCmd.Flags()` and `robotRefsCmd.Flags()` separately (per repo-research recommendation: only two commands need it; persistent flag is overkill).
- Enum: `"v1"`, `"v2"`. Default: `"v2"`.
- Env override: `BT_OUTPUT_SCHEMA` (matches existing `BT_OUTPUT_FORMAT` / `BT_OUTPUT_SHAPE` pattern).
- Precedence: flag > env > default.
- Validation: invalid value (flag or env) → exit 1 with stderr listing valid values. No silent fallback.
- Help text: `schema version (v1|v2). v1 retained for one release.` — env-var advertisement lives in the `bt robot docs` env map per `--shape` precedent (no parenthetical in help).

#### `--sigils` flag (on refs subcommand only)

- Registered on `robotRefsCmd.Flags()`.
- Enum: `"strict"`, `"verb"`, `"permissive"`. Default: `"verb"`.
- Env override: `BT_SIGIL_MODE` (flat `BT_<THING>` pattern; no subcommand-scoped prefix precedent in the codebase).
- Precedence: flag > env > default.
- Validation: invalid value → exit 1 with stderr listing valid values.
- Conflict with `--schema=v1`: `--sigils=<any>` with `--schema=v1` errors with stderr `Error: --sigils=<value> requires --schema=v2. Run with --schema=v2, or omit --sigils to use v1 defaults.` and exits 1. Env-set sigils + explicit `--schema=v1` is the same error.
- Help text: `sigil recognition mode (strict|verb|permissive). Ignored under --schema=v1.` — env-var advertisement in `bt robot docs` env map.

#### Env var discoverability

Both new vars (`BT_OUTPUT_SCHEMA`, `BT_SIGIL_MODE`) registered in the env-docs map at `cmd/bt/robot_graph.go:224` alongside `BT_OUTPUT_SHAPE`, so `bt robot docs` surfaces them. The plan previously cited `cobra_robot.go:224` in places — corrected; the map lives in `robot_graph.go`.

### Sigil detection (hand-rolled, in `pkg/analysis/sigils.go`)

Lives in new `pkg/analysis/sigils.go` — NOT in `pkg/view/`. Per architectural review: `pkg/view/` is for graph-derived consumer projections; a markdown sigil parser is a string-domain language primitive that coheres with `pkg/analysis/SplitID`. `pkg/view/ref_record.go` consumes `analysis.DetectSigils()` rather than defining it. Future consumers (e.g., a `--explain-refs` mode) get the parser for free.

Single-pass **iterative** tokenizer (no recursion) preprocesses each prose body into a segmented representation: fenced code blocks (triple-backtick, triple-tilde, 4-space indent), inline code spans (single backtick), markdown links `[text](url)`, and plain prose. Sigil detection runs against the segmented representation.

**Hard limits enforced inside the tokenizer:**
- **Per-body input cap: 1MB.** Bodies exceeding this are truncated to 1MB and emitted with `truncated: true` on every record produced from that body. Prevents 100MB pasted-log bodies from ballooning memory.
- **Bounded fence-stack depth: 32 frames.** Fenced-block recognizer uses an explicit stack with a depth cap; pathological "nested infinitely" input gets clipped to depth 32 and remaining content treated as flat prose.
- **Single-pass guarantee.** All recognizers walk the byte stream forward once; no "scan back to find opener" patterns. The 100KB and 100KB-pathological benchmarks (below) verify O(n).

**Per-mode vocabulary:**

| Mode | Recognizes |
|---|---|
| `strict` | markdown link (link text is valid bead ID; URL free-form), inline code (single backtick around ID), `ref:` / `refs:` keyword (case-insensitive, optional single space) |
| `verb` | everything in strict, PLUS a fixed verb list (`see`, `paired with`, `blocks`, `closes`, `fixes`, `mirrors`) with same-line 32-char proximity (inclusive) preceding the ID, case-insensitive, markdown formatting stripped before counting |
| `permissive` | v1's known-prefix scoping retained, PLUS fenced code block + inline code exclusion. No additional sigil requirement. |

**Proximity rule for verbs (resolving SpecFlow M5):**
- Window: 32 chars inclusive on both ends.
- Multiple verbs before same ID → one record, closest verb wins (recorded in `sigil_kind`).
- Markdown formatting (`**`, `__`, `*`, `_`) stripped before counting chars — `**see**` counts as 3, not 7.

**Performance invariants (verified empirically in `pkg/analysis/sigils_test.go`):**
- O(n) per prose body, n = body length. Asserted via 10x-scaling test: `runtime(body × 10) <= 15 × runtime(body)`.
- Tokenizer benchmarks ship with concrete thresholds:
  - 10KB body: <500µs per mode
  - 100KB body: <5ms per mode
  - 400 issues × 3 fields × 10KB realistic = 12MB total scan: <800ms
- Pathological-input tests (resolving SpecFlow M4 + security review):
  - 100KB single body
  - 100K alternating `` ` ``-prefixed inline-code storm (`\x60a\x60b...`)
  - 10K nested `~~~\n```\n~~~\n```...` fenced regions (verifies stack cap kicks in)
  - 10K `[x](y)` link sequences
  - Single 100KB line, no newlines
  - Unclosed fence followed by 100KB of prose
  - Invalid UTF-8 byte sequences (no panic)
  - Lone surrogates, U+202E (RTL override), zero-width joiners (no panic)
  - 1MB body (boundary; emits truncated: true and stops)
  - Empty body, nil Comments slice
  - Markdown link with empty text `[](url)` or empty URL `[bt-x]()`

**Panic safety:** the call site in `refsOutputV2` wraps `DetectSigils` in `defer recover()` — one malformed issue logs and skips, doesn't crash `--global` for the whole corpus.

### Pair detection canonicalization (resolving SpecFlow H4)

`ComputePairRecordsV2` walks the issue set and:

1. **Bucket** issues by suffix (via `analysis.SplitID`, as in v1). Issues with malformed IDs are skipped.
2. For each bucket with ≥2 distinct prefixes, build an **undirected adjacency map of cross-prefix dep edges**. Both `bt-x blocks bd-x` and `bd-x blocks bt-x` collapse to a single undirected edge `{bt-x, bd-x}`. Edges to issues outside the bucket (different suffix) are ignored. Edges to non-existent issues are skipped silently — matches v1 tolerance for bead deletion races (SpecFlow M6).
3. **Compute connected components via BFS over the adjacency map.** BFS chosen over union-find for readability at N=400 (per performance review); union-find buys nothing at this scale. Each component with ≥2 distinct prefixes becomes one `PairRecordV2`. Component with all members sharing one prefix → dropped (data anomaly).
4. **Cycles are fine.** A 3-way cycle (`bt-x → bd-x → cass-x → bt-x`) is one connected component → one record. Bidirectional deps (`bt-x ↔ bd-x`) collapse to one undirected edge → one record. Acyclicity isn't required.
5. **Canonical = first-created within the component** (unchanged from v1). Mirrors sorted by prefix ascending.
6. **Drift flags** computed against canonical, with `title` drift removed (memory-confirmed no-signal).

Algorithm complexity: O(n) bucketing + O(V+E) BFS per bucket, where V = bucket size (typically 2–4) and E = cross-prefix edges in bucket. Total well under O(n log n) at projected scale.

### Reader handlers

`cmd/bt/robot_pairs.go` and `cmd/bt/robot_refs.go` gain a dispatch:

```go
func pairsOutput(issues []model.Issue, dataHash string, schema string) any {
    if schema == "v1" {
        return pairsOutputV1(issues, dataHash)
    }
    return pairsOutputV2(issues, dataHash)
}
```

Pure helpers (`pairsOutputV2`, `refsOutputV2`) stay testable under `BT_TEST_MODE=1` per `project_bt_test_mode_global_contract.md`.

### Forward-only backfill helper (resolving SpecFlow M1)

New subcommand flag on `bt robot pairs`: `--orphaned`. Under `--schema=v1`, lists pairs detected by v1 but missing the dep edge required by v2 — exactly the checklist of pairs a human OR agent needs to manually `bd dep add`.

**Output is agent-native:**
- **Stdout** (machine-readable): JSONL records `{"suffix": "...", "members": ["bt-x", "bd-x"], "suggested_command": "bd dep add bt-x bd-x --type=related"}`. Agents can pipe into a script that auto-runs the suggested commands after human review.
- **Stderr** (human prose): summary count + reminder that running `bd dep add` is a write operation requiring user approval.

Used during Phase 1 migration, but stays available for any future re-run when someone manually creates a paired bead without proper deps. Not a write tool; we surface the todo list, the operator decides to apply.

Out of scope: an automated "apply" mode. Cross-project writes need human authorization per global CLAUDE.md.

### Deprecation signal (cut from plan; CHANGELOG suffices)

Earlier draft included a stderr deprecation notice on `--schema=v1` invocations. **Cut on review.** Zero external consumers, the `2>&1` merging behavior is fragile, and a CHANGELOG entry is sufficient signal for the actual humans running this. Removal eliminates the `BT_QUIET` suppression flag and `TestRobotPairs_DeprecationNoticeOnV1` test.

Migration discoverability for `--schema=v1` users: `.beads/conventions/cross-project.md` documents the v1 v2 transition, and `bt robot schema` advertises both `pair.v1` and `pair.v2` (so downstream agents reading schema can detect newer versions exist).

### Labeled corpus + FPR gate

New file `pkg/view/testdata/corpus/labeled_corpus.json`. Shape:

```json
{
  "issues": [
    { "id": "...", "description": "...", "dependencies": [...], "notes": "...", "comments": [...], ... }
  ],
  "expected_pairs": [
    { "suffix": "zsy8", "members": ["bt-zsy8", "bd-zsy8"], "intentional": true, "reason": "Paired via --id workflow; mirrors same logical work" }
  ],
  "expected_refs": [
    { "source": "bt-mhwy.3", "target": "bd-la5", "intentional": true, "reason": "Cross-project ref in description" }
  ]
}
```

**Labeling rubric** (resolving SpecFlow M2) lives in `docs/design/2026-04-21-pairs-refs-v2.md`:
- A pair is `intentional: true` iff a human read both beads' descriptions and determined they describe the same logical work across projects. (Suffix collision on unrelated work → `false`.)
- A ref is `intentional: true` iff the author clearly intended to reference the target bead. (Placeholder text `bt-xxx`, English slugs → `false`.)
- Disputes resolved by discussion in the bead itself (add comment justifying re-labeling).

**Sanitization gate (pre-commit, resolving security review item 3):**
A pre-commit script scans the candidate `labeled_corpus.json` for: `password`, `secret`, `token`, `api_key`, `AKIA[0-9A-Z]{16}`, `ghp_[A-Za-z0-9]{36}`, `xox[bp]-[A-Za-z0-9-]+`, `\.env`, `localhost:[0-9]+`, `C:\\Users\\[a-z]+`, email addresses outside `seanmartinsmith.com` / `users.noreply.github.com`. Any hit fails the commit. Scan applies to `description` AND `notes` AND `comments` AND `close_reason` AND `dependencies` fields — all serialized issue prose, not just description. The `security` label skip is a fallback safety net, not the primary mechanism.

**Project-prefix exposure check:**
The corpus references real cross-project beads (`bt-zsy8`, `bd-zsy8`, Gastown 4-way `bt-byk` + `cass-byk` + `tpane-byk` + `cnvs-byk`, etc.). The bt repo is public (`seanmartinsmith/beadstui`). Before committing, confirm prefix names (`cass`, `tpane`, `cnvs`, `mkt`, `dotfiles`) are intended public-disclosed. If any prefix should remain private, alias it in the corpus (`tpane-x` → `proj1-x`) and document the alias mapping in the rubric.

**FPR test** (`pkg/view/fpr_gate_test.go`):
- Runs `ComputePairRecordsV2` and each `--sigils` mode of `ComputeRefRecordsV2` against the corpus.
- Computes FPR = (emitted records where truth says not intentional) / (total emitted records).
- Asserts ≤5% broken-flag FPR on refs under default (verb) mode (gate is mode-specific; `strict` and `permissive` get informational FPR readouts but no hard threshold).
- Asserts <10% total-pair FPR on pairs, AND requires the test corpus to have N≥10 candidate pairs (otherwise the threshold is meaningless — single mislabel = 10%).
- Memory delta enforced: `runtime.ReadMemStats` before/after `json.Unmarshal` of corpus, fails if delta >10MB.
- **Malformed fixture handling (resolving SpecFlow M3):** `json.Unmarshal` errors fail the test with a clear message including file path. Missing `truth` field on an expected record fails the test. No silent skip.

**Corpus selection:** ~30–50 open issues from the current shared Dolt server, selected to cover:
- At least 5 intentional cross-project pairs (including `bt-zsy8` / `bd-zsy8` and the Gastown 4-way `bt-byk` + `cass-byk` + `tpane-byk` + `cnvs-byk`)
- At least 5 suffix-collision unrelated-work cases (negative examples for pairs)
- Real prose samples exercising each sigil mode
- Placeholder text (`bt-xxx`) examples to validate exclusion
- Slug-like prose (`-only`, `-side`) examples

**Corpus sanitization:** v1 of the corpus skips issues flagged with `security` label and issues whose prose contains patterns matching the redaction list (URLs pointing to non-public repos, emails outside the `sms@` / `seanmartinsmith@` family). If no sensitive content is present (likely for bt/bd/cass/mkt beads), commit verbatim. Redaction harness deferred until corpus grows past trivial scope.

### bt-side convention doc

New `.beads/conventions/cross-project.md`. Content:

- Thin pointer to canonical convention: `~/System/marketplace/plugins/harness/skills/cross-project/SKILL.md`.
- bt-side reader surface: what each `--sigils` mode recognizes, what `intent_source` / `sigil_kind` values mean, how to pin to `--schema=v1`.
- How to invoke: `bt robot pairs --global`, `bt robot refs --global [--sigils=strict|verb|permissive]`.
- Migration guidance for the `--schema=v1` transition.
- Cross-references: bt-gkyn, bt-vxu9, mkt-gkyn, mkt-vxu9.

Discoverability: bt sessions encounter it via the existing `.beads/conventions/` pattern (labels.md, reference.md live there already).

### Implementation Phases

Each phase is independently testable and leaves the tree green.

#### Phase 1 — Foundation (schemas, flags, backfill helper, docs skeleton, schema advertisement)

**Deliverables:**
- `pkg/view/schemas/pair_record.v2.json`, `ref_record.v2.json`
- `cmd/bt/robot_schema_flag.go` — resolveSchemaVersion() + flag registration on pairs + refs
- `cmd/bt/robot_sigils_flag.go` — resolveSigilsMode() + flag registration on refs
- `cmd/bt/robot_graph.go:224` amended — env var docs include `BT_SIGIL_MODE`, `BT_OUTPUT_SCHEMA`
- `cmd/bt/cobra_robot.go` amended — `bt robot schema` advertises new envelope/record fields for `pair.v2` and `ref.v2`
- `--orphaned` flag on `bt robot pairs --schema=v1` emitting JSONL checklist on stdout + summary on stderr
- Flag-matrix test rows for `--sigils=strict|verb|permissive` and `--schema=v1|v2`
- Flag-conflict test case: `--schema=v1 --sigils=strict` exits 1 with the new error message including resolution
- `.beads/conventions/cross-project.md` skeleton (pointer + TODO sections)
- `docs/design/2026-04-21-pairs-refs-v2.md` skeleton (with labeling rubric section + denylist scan script)
- **Dispatcher helper extracted up-front**: a `pkg/view/schema_dispatch.go` with `DispatchSchema[T any](version string, v1Fn, v2Fn func() T) T` so Phase 2 + Phase 3 don't reinvent the dispatch shape. (Or: lock the dispatch shape inline in Phase 2 and Phase 3 mirrors verbatim — pick one in implementation; both are fine, but commit to one before Phase 2 starts.)

**Success criteria:** `go build ./...` + `go vet ./...` + `go test ./cmd/bt/... ./pkg/view/...` all pass. `bt robot pairs --global --schema=v1 --orphaned --robot-format=jsonl` emits JSONL records to stdout. `bt robot refs --global --sigils=invalid 2>&1 | grep "valid values"` succeeds. `bt robot refs --global --schema=v1 --sigils=strict 2>&1 | grep "Run with --schema=v2"` succeeds.

**Forward-only backfill work (operator action, not code):** run the `--orphaned` output's checklist and manually `bd dep add --type=related` for each of the ~5 intentional pairs (including `bt-zsy8` / `bd-zsy8`, `bt-byk` / `cass-byk` / `tpane-byk` / `cnvs-byk`, `dotfiles-2il` / `mkt-2il`). Document each addition in the session CHANGELOG. **Must complete BEFORE the v2 default flips** — verified by re-running `--orphaned` and confirming empty output.

#### Phase 2 — pair.v2 reader

**Depends on Phase 1** (schema dispatch shape locked, `--schema` flag registered, backfill complete).

**Deliverables:**
- `pkg/view/pair_record.go` amended — `ComputePairRecordsV2`, `PairRecordV2`, `PairRecordSchemaV2`, BFS-over-connected-components algorithm
- `pkg/view/pair_record_test.go` amended — new unit tests: basic cross-prefix dep, bidirectional dep (collapses to one record), 3-way cycle (one record), 3-way connected with partial edges (one record), 3-way disconnected (two records), dangling dep (skipped), title drift absence, `intent_source: "dep"` on every record
- `pkg/view/testdata/fixtures/pair_v2_*.json` + `testdata/golden/pair_v2_*.json` — 4 new fixtures (empty, single_in_sync, single_drifted, multi_way_connected)
- `pkg/view/projections_test.go` amended — register `pair_v2_*` golden harness
- `cmd/bt/robot_pairs.go` amended — schema dispatch in `pairsOutput()`
- `cmd/bt/robot_pairs_test.go` amended — `TestRobotPairs_SchemaV2Default`, `TestRobotPairs_SchemaV1Fallback`

**Success criteria:** `bt robot pairs --global` on the real shared corpus emits ≤ the ~5 known intentional pairs (after the Phase 1 backfill). `bt robot pairs --global --schema=v1` emits the old ~29. Goldens all pass.

#### Phase 3 — ref.v2 reader + sigil detector + performance gate

**Depends on Phase 1** (dispatch shape, `--sigils` flag, env var). Can run in parallel with Phase 2 if dispatch shape is locked in Phase 1.

**Deliverables:**
- `pkg/analysis/sigils.go` NEW — hand-rolled iterative tokenizer + per-mode recognizer. Single exported `DetectSigils(body string, mode SigilMode) []SigilMatch` + the mode enum. Bounded-depth fence stack (32 frames). 1MB body cap with `truncated: true` flag.
- `pkg/analysis/sigils_test.go` NEW — unit tests per kind + all pathological inputs enumerated above + linear-scaling assertion (`runtime(10x) <= 15 × runtime(1x)`) + benchmarks (`Benchmark_DetectSigils_10KB`, `_100KB`, `_PathologicalNestedFences`, `_PathologicalInlineCodeStorm`)
- `pkg/view/ref_record.go` amended — `ComputeRefRecordsV2`, `RefRecordV2`, `RefRecordSchemaV2`; prose scan delegates to `analysis.DetectSigils`. Call site wraps in `defer recover()` for panic safety on adversarial inputs.
- `pkg/view/ref_record_test.go` amended — unit tests per mode (strict/verb/permissive), proximity window boundary tests, multiple-verbs-one-ID collapsed, markdown-formatting stripping, external-dep and bare-dep `sigil_kind` values, `truncated: true` flag emission on >1MB body
- `pkg/view/testdata/fixtures/ref_v2_{strict,verb,permissive}_*.json` + matching goldens — 6 fixtures covering each mode × 2 scenarios
- `pkg/view/projections_test.go` amended — register `ref_v2_*` golden harness
- `cmd/bt/robot_refs.go` amended — schema + sigil dispatch in `refsOutput()`
- `cmd/bt/robot_refs_test.go` amended — `TestRobotRefs_SigilModes`, `TestRobotRefs_EnvVarOverride`, `TestRobotRefs_InvalidSigil`, `TestRobotRefs_SchemaV1IncompatibleWithSigils`

**Performance regression gate (lands in this phase, not Phase 5):**
- `Benchmark_DetectSigils_100KB` asserts <5ms per mode
- `TestSigilDetector_LinearScaling` asserts `t(body × 10) ≤ 15 × t(body)`
- `TestSigilDetector_NoPanic_AdversarialInputs` confirms no panic on the security-review pathological set
- Total `bt robot refs --global` budget: <800ms on the real corpus (ship measurement in CHANGELOG)

**Success criteria:** `bt robot refs --global --sigils=strict|verb|permissive` all produce deterministic, sorted output with correct `sigil_kind` on every record. All performance gates pass. Tokenizer benchmarks shipped as regression guards.

**Load-bearing constraint (do NOT regress):** v1's cross-project-only filter must remain in v2 ref detection. Removing it regresses FP rate to ~85% per git history (`pkg/view/ref_record.go` initial commit msg). Goldens enforce.

#### Phase 4 — Convention doc + labeling rubric

**Deliverables:**
- `.beads/conventions/cross-project.md` fleshed out (replaces skeleton)
- `docs/design/2026-04-21-pairs-refs-v2.md` fleshed out (replaces skeleton) — includes labeling rubric, full rationale, cross-refs to brainstorm + beads

**Success criteria:** docs read coherently when opened cold. Cross-references resolve.

#### Phase 5 — Labeled corpus + FPR gate

**Depends on Phase 3** (corpus schema locks when ref.v2 shape lands). Can run in parallel with Phase 4 (convention docs) and labeling work can start as soon as Phase 3 lands.

**Deliverables:**
- `pkg/view/testdata/corpus/labeled_corpus.json` — ≥30 sanitized issues with truth labels; MUST contain ≥10 candidate pair records (needed for the <10% pair FPR threshold to be meaningful). Includes all ~5 intentional pairs (post-Phase-1 backfill) + ≥5 negative examples (suffix collisions, placeholder text, English slugs).
- Pre-commit sanitization script at `scripts/audit-corpus.sh` (or similar) running the denylist regex scan. Runs as part of CI and as a local pre-commit hook.
- `pkg/view/fpr_gate_test.go` — FPR-gate test asserting ≤5% refs broken FPR (verb mode only) and <10% pair FPR. Memory delta enforced. Errors-fail on malformed fixture.

**Success criteria:** `go test ./pkg/view/ -run FPR` passes. Running `-v` shows the computed FPR under each sigil mode (strict/verb/permissive informational) — output becomes the basis for future mode-default decisions.

#### Phase 6 — Docs + session close

**Deliverables:**
- CHANGELOG.md session entry summarizing Phases 1–5
- `docs/adr/002-stabilize-and-ship.md` Stream 1 status line
- `bd close bt-gkyn --reason="..."` (structured close template)
- `bd close bt-vxu9 --reason="..."` (structured close template)
- Session-end push: `git pull --rebase && bd dolt push && git push` in bt and marketplace (marketplace push covers the new `mkt-gkyn` / `mkt-vxu9` beads)

**Success criteria:** `git status` shows "up to date with origin" in bt. `bd list --status=open | grep -E "(bt-gkyn|bt-vxu9)"` returns empty.

### Effort framing

Agent-slots × sequential depth: **1 slot, 6 sequential phases, moderate depth.** Phase 5 can parallelize with Phase 4, reducing to 5 sequential steps if desired. Single-session ship is feasible for Phases 1–4 + 6; Phase 5 (corpus collection + labeling) may want its own session for uninterrupted human labeling judgment.

## Alternatives Considered

| Alternative | Why rejected |
|---|---|
| **Pair detection = dep OR notes-line (OR semantics)** | Adds reader complexity (notes parsing), hides hook-stamping failures behind a silent fallback. Single-signal detection is simpler and surfaces hook bugs as bugs. See brainstorm Key Decisions #4. |
| **Notes-only fallback branch** | Same as above — never triggers when hook works; masks hook bugs when hook breaks. |
| **Ship only strict + verb modes (drop permissive)** | YAGNI-borderline, but permissive is cheap to implement and serves the "v1-parity with hardening" case for consumers not ready to adopt sigils. User explicitly chose keep all three. |
| **Import goldmark for markdown parsing** | Heavy dep for a bounded scope (fenced block, inline code, link syntax). Hand-rolled ~80–120 LOC is cheaper and more predictable. Zero new deps rule applies. |
| **Stay at schema v1, signal semantics via envelope mode only** | Semantic change (which records emit) is large enough that schema bump signals "consumer must update or pin." Envelope-only signaling hides breaking change behind an optional field. |
| **Automated migration script for the ~5 intentional pairs** | Cost > benefit at N=5. Manual `bd dep add` is cheaper and leaves no new code to maintain. See brainstorm Key Decisions #9. |
| **Upstream beads `paired_with` column PR first** | Defers v2 ship until upstream merges. Convention isn't dogfooded yet; column shape isn't known. User's 5 recent merged upstream PRs make this viable *later*, but the brainstorm committed to convention-first. |
| **Ship `--explain-refs` observability in v2** | Real need (SpecFlow H3), but adds scope. File as follow-up bead; core v2 can still ship + be debugged via mode toggling. |

## System-Wide Impact

### Interaction Graph

- `bt robot pairs --global` → `robotCmd.PersistentPreRunE` (sets `BT_ROBOT=1`, resolves `--shape`) → `robotPairsCmd.RunE` → `robotPreRun()` (loads data) → `rc.runPairs()` → `pairsOutput(issues, hash, schema)` → dispatch to `pairsOutputV1` or `pairsOutputV2` → `view.ComputePairRecords{V1,V2}` → JSON encode → stdout.
- `bt robot refs --global --sigils=verb` → same preamble → `rc.runRefs()` → `refsOutput(issues, hash, schema, mode)` → `view.ComputeRefRecords{V1,V2}` → for v2, per-issue `view.DetectSigils(body, mode)` → match filtering → JSON encode → stdout.
- Flag resolution: `resolveSchemaVersion()` runs in each subcommand's `RunE` (not PersistentPreRun) to keep the helper scope tight. Conflict check `--schema=v1 --sigils=*` happens in `runRefs()` before any output.
- No callbacks, middleware, observers, or event handlers — this is all synchronous, in-process, read-only query work.

### Error & Failure Propagation

- **Flag validation errors** (`--sigils=invalid`, `--schema=bogus`, conflict pair) → stderr message + `os.Exit(1)` via existing pattern in `robot_compact_flag.go`. No error type wrapping needed; these exit before any business logic.
- **Data load failure** (Dolt unavailable, permissions) → propagates from existing `robotPreRun()` chain. Same behavior as v1. Not new scope.
- **Malformed fixture** in tests → `json.Unmarshal` returns error → test fails with file path. No silent recovery.
- **Dangling dep in pair detection** → match-fails silently (pair not emitted). Matches v1 tolerance. Documented in the pair detection algorithm section above.
- **Corpus schema drift** (labeled_corpus.json structure changes between commits) → `json.Unmarshal` into the typed struct catches it; missing `expected_pairs` or `expected_refs` fields fail the test with clear message.

### State Lifecycle Risks

None. All operations are read-only queries over in-memory slices. No database writes, no file writes outside tests, no state mutation beyond stdout output. `bt robot pairs --schema=v1 --orphaned` does NOT modify data — it only emits a stderr advisory listing pairs needing manual dep-add.

### API Surface Parity

- `bt robot pairs --global` (command) — amended. v2 is default.
- `bt robot refs --global` (command) — amended. v2 is default.
- `bt robot schema` subcommand — **needs update** to advertise the new envelope fields (`intent_source`, `sigil_mode`) and v2 record schemas. Identified by research; tracked as part of Phase 1 or 2 deliverables.
- TUI pair rendering (`bt-6cfg`) — **not yet implemented** (P3, open). When it lands, it consumes pair.v2 directly (authoritative shape). No v1→v2 translation layer needed.
- `bt robot docs` env var map — amended to include `BT_SIGIL_MODE`, `BT_OUTPUT_SCHEMA`.
- No other commands expose equivalent functionality.

### Integration Test Scenarios (resolving SpecFlow M7)

Five cross-layer scenarios for `cmd/bt/robot_all_subcommands_test.go` (flag matrix) and contract tests:

1. **`bt robot pairs --global`** with no explicit schema → default v2 → emits `schema: "pair.v2"` and per-record `intent_source: "dep"`.
2. **`bt robot pairs --global --schema=v1`** → v1 path → emits `schema: "pair.v1"` + stderr deprecation notice.
3. **`bt robot refs --global --schema=v1 --sigils=strict`** → hard error with `--sigils requires --schema=v2` message, exit 1.
4. **`BT_SIGIL_MODE=strict bt robot refs --global`** (no flag) → strict mode applied, `sigil_mode: "strict"` in envelope.
5. **`BT_SIGIL_MODE=strict bt robot refs --global --sigils=verb`** (flag overrides env) → verb mode applied, `sigil_mode: "verb"` in envelope.

Per repo-research, use `runAccepts` in `robot_all_subcommands_test.go:123` which tolerates runtime errors (Dolt unreachable under `BT_TEST_MODE=1`) but fails on flag-parse errors. Add the scenario 3 error message to the whitelist of expected fail-strings.

## Acceptance Criteria

### Functional Requirements

- [ ] `bt robot pairs --global` default behavior emits `schema: "pair.v2"` with detection rule = dep edge between same-suffix cross-prefix beads
- [ ] `bt robot pairs --global --schema=v1` emits v1-shaped output + stderr deprecation notice
- [ ] `bt robot pairs --global --orphaned --robot-format=jsonl` (under v1) emits JSONL records to stdout + summary to stderr; exits 0 even when list is empty
- [ ] `bt robot refs --global` default behavior emits `schema: "ref.v2"`, `sigil_mode: "verb"`
- [ ] `bt robot refs --global --sigils=strict|verb|permissive` each produces deterministic, sorted output
- [ ] `BT_SIGIL_MODE` and `BT_OUTPUT_SCHEMA` env vars are honored with flag > env > default precedence
- [ ] Invalid `--sigils` / `--schema` values (flag or env) exit 1 with stderr listing valid values
- [ ] `--schema=v1 --sigils=*` hard-errors with explicit message
- [ ] Pair detection handles bidirectional deps (collapse to one record), N-way connected components (one record), disconnected same-suffix groups (one record per component), dangling deps (skip silently)
- [ ] Verb proximity rule is same-line, 32-char inclusive window, closest verb wins, markdown formatting stripped
- [ ] Markdown tokenizer handles unbalanced backticks, nested fenced blocks, CRLF, BOM, >10K-char lines without error or pathological slowdown
- [ ] `bt robot schema` advertises `pair.v2`, `ref.v2` and the new envelope/record fields

### Non-Functional Requirements

- [ ] Zero new Go dependencies (sigil tokenizer is hand-rolled)
- [ ] Tokenizer is O(n) per prose body; no regex backtracking
- [ ] Serial execution (no goroutines added; matches v1)
- [ ] Memory: labeled corpus loads in <10MB (enforced by test — split corpus if it grows beyond)
- [ ] Backward compat: all existing `pkg/view/` tests pass unchanged; `--schema=v1` fallback emits structurally equivalent output to v1 baseline (same fields, same sort order, same envelope shape; `generated_at` and `data_hash` differ by definition between runs)

### Quality Gates

- [ ] `go test ./pkg/view/... ./cmd/bt/...` passes (including new tests and regenerated goldens)
- [ ] `go vet ./... && go build ./...` passes
- [ ] `pkg/view/fpr_gate_test.go` asserts ≤5% broken-flag FPR on refs (verb mode) and <10% total-pair FPR (with N≥10 candidate pairs in corpus)
- [ ] Corpus fixture committed with labeling rubric documented in `docs/design/2026-04-21-pairs-refs-v2.md`
- [ ] All ~5 known intentional pairs have manual `bd dep add` edges BEFORE the v2 default flips (verified via `--orphaned` returning empty)
- [ ] Session close per AGENTS.md: CHANGELOG updated, ADR-002 Stream 1 updated, both beads closed, push succeeded

## Success Metrics

| Metric | Baseline (v1) | Target (v2 default / verb mode) |
|---|---|---|
| Pair FPR on real shared corpus | ~24/29 (~83%) | <10% (≤1 of ~5 post-backfill) |
| Refs broken-flag FPR | ~30% | ≤5% |
| `bt robot pairs` / `refs --global` runtime | <1s | <1s (no perf regression) |
| Go dep count in `go.mod` | current | unchanged |
| Consumer-side breakage | n/a | 0 known consumers (bt-6cfg not yet implemented) |
| Backward compat via `--schema=v1` | n/a | Structurally equivalent (envelope shape + record fields + sort order; `generated_at` and `data_hash` exempt by nature) |

## Dependencies & Prerequisites

### Shipped (already landed)

- bt-mhwy.2 — pairs v1 reader
- bt-mhwy.3 — refs v1 reader
- bt-mhwy.5 — external dep resolution + `rc.analysisIssues()` composition rule
- bt-ssk7-era upstream CLI gate fix (bare cross-prefix `bd dep add` works — confirmed via one of your 5 merged upstream PRs)
- `pkg/analysis.SplitID` (exported under bt-mhwy.2)

### Parallel tracks (not blocking)

- `mkt-gkyn` — marketplace-side pair hook + skill text. Filed this session. Not blocking bt-side; readers ship without it. Manual `bd dep add` covers the ~5 intentional pairs until hook lands.
- `mkt-vxu9` — marketplace-side refs skill extension. Filed this session. Convention doc extension; not blocking.

### Deferred follow-ups

- Upstream beads `paired_with` column PR — after convention dogfoods and stabilizes.
- `--explain-refs` observability mode — filed as new bead when planning concludes.
- Remove `--schema=v1` fallback — filed as new bead; trigger is "one release after v2 ship."

## Risk Analysis & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Labeled corpus labor cost overruns | Medium | Delays Phase 5 | Start with curated ~30 records covering AC cases; grow to 50 only if FPR thresholds are hit. Labeling can parallelize with other phases. |
| Default sigil mode (verb) produces wrong trade-off | Low | Wrong mode ships as default | Env var toggle lets runtime override without rebuild; dogfooding drives default-flip via a one-line change later. |
| Markdown tokenizer pathological input | Medium | Runtime panic or slowdown | Edge-case test fixtures enumerated (M4 list); 100KB performance benchmark enforces O(n); hand-rolled scanner has no regex backtracking. |
| Closed pairs like `bt-zsy8`/`bd-zsy8` invisible under v2 after backfill | Low | Historical pairs missed | Phase 1 explicitly includes the backfill step before v2 default flips. Manual `bd dep add --type=related` on the known-5 list. |
| Fixtures drift if `model.Issue` schema changes upstream | Medium | Labeled corpus breaks on unmarshal | Typed Go struct for fixture shape catches drift at compile time; test fails loudly with file path. |
| `bt robot schema` out of sync with new record shapes | Medium | Agent discoverability regression | Phase 1 deliverable explicitly includes `bt robot schema` update. |
| User changes mind on `--schema=v1` retention policy | Low | Extra work removing earlier than planned | Filed as a separate bead with explicit "one release" trigger; no promises inside the plan beyond that. |
| Pair cycle detection has unknown edge case in real data | Low | Emitted records differ from expectation | Connected-component algorithm via union-find; unit tests cover bidirectional, triangular, and disconnected cases. |
| `--orphaned` flag misused as a write tool | Low | Confusing stderr | Docstring explicitly reads "read-only advisory; apply manually via `bd dep add`". JSONL stdout output includes `suggested_command` field; consumer is responsible for review-then-execute. |
| **mkt-gkyn hook lands during ongoing manual backfill** | Low | Double-stamped deps or race | Phase 1 backfill runs and verifies before flipping v2 default. Hook in marketplace is filed but not wired by us. If hook lands first, manual backfill becomes a no-op (hook already added the deps); no double-stamping because `bd dep add` is idempotent for the same edge. |
| **Tokenizer panic on adversarial prose** | Low | One issue's processing aborts | `defer recover()` at the call site logs and skips that issue; rest of `--global` proceeds. Test asserts no panic on enumerated pathological inputs. |
| **Corpus committed with PII / secrets** | Medium (one-shot risk, hard to undo) | Public-repo leak | Pre-commit denylist scan gate; security-label fallback; manual project-prefix exposure check before initial commit. |

## Rollback Plan

If v2 default produces unexpected results post-ship:

1. **User-side workaround (immediate):** any consumer can pin to v1 with `--schema=v1` flag or `BT_OUTPUT_SCHEMA=v1` env. No code change required. This is exactly what the fallback is for.
2. **Code-side default flip (one-line revert):** the default value in `resolveSchemaVersion()` flips from `"v2"` back to `"v1"`. New consumers default to v1; existing v2 invocations still work (just emit v1 envelope).
3. **Full revert:** `git revert` the commit that landed the v2 dispatch in `pairsOutput()` / `refsOutput()`. v1 was never modified, so reverting v2 is mechanical.

The fallback flag and unchanged v1 code path together mean **revert is cheap at every layer**. No data migration, no consumer breakage, no state to clean up.

If the labeled corpus FPR gate fails post-merge, the rollback is even simpler: skip the FPR test (`go test -short` or `t.Skip()`) while investigating, since the gate is a quality threshold not a correctness gate. The reader keeps working.

## Resource Requirements

- **People:** solo dev + AI pair, no external dependencies.
- **Time (agent-slot framing):** ~1 agent-slot spanning 6 sequential phases with one parallelizable step (Phase 5 alongside Phase 4). Single session feasible for Phases 1–4 + 6; Phase 5 can split to its own session for uninterrupted labeling judgment.
- **Infrastructure:** local Dolt shared server (already running). No new infrastructure.
- **Tooling:** Go 1.25+, existing build/test toolchain. No new deps.

## Future Considerations

1. **Upstream `paired_with` column** — once the convention is dogfooded (say, 1–2 weeks post-ship), file upstream PR to add a first-class `paired_with` field on beads. Readers gain a third `intent_source` value; dep-edge channel remains authoritative.
2. **`--explain-refs` mode** — emits rejection reasons per prose candidate span. Makes FPR regressions debuggable. File as a new bead after v2 ships.
3. **TUI consumption** — `bt-6cfg` (open P3) is the downstream TUI pair renderer. When it lands, it consumes `pair.v2` directly.
4. **Intra-project refs** — v1 scoped to cross-project only to avoid suffix-collision FP. With v2 intent-based detection, intra-project refs could re-enter scope safely. File as new bead.
5. **Multi-mode A/B audit harness** — run all three `--sigils` modes on the labeled corpus in one test pass, emit a comparative FPR table. Currently each mode tests independently; a unified comparison helps data-driven default-flip decisions.
6. **Corpus growth** — current plan starts at ~30–50 records. Once stable, consider growing to 200+ with a redaction harness to handle PII-flagged issues. Triggered only if smaller corpus produces unstable FPR measurements.
7. **Consumer-side strict-decoding test** — add a consumer simulator that unmarshals v2 output into a v1-shaped struct, asserts graceful handling of unknown fields. Catches strict-decoder regressions early.

## Documentation Plan

- [ ] `.beads/conventions/cross-project.md` — new, bt-side pointer doc (Phase 4)
- [ ] `docs/design/2026-04-21-pairs-refs-v2.md` — new, implementation design + labeling rubric (Phase 4)
- [ ] `CHANGELOG.md` — session entry (Phase 6)
- [ ] `docs/adr/002-stabilize-and-ship.md` — Stream 1 status (Phase 6)
- [ ] `bt robot pairs --help` / `bt robot refs --help` — updated via flag help strings (Phases 1–3, incidental)
- [ ] `bt robot schema` output — updated to advertise v2 envelope/record shape (Phase 1, locked)
- [ ] `bt robot docs` env var map — `BT_SIGIL_MODE`, `BT_OUTPUT_SCHEMA` added (Phase 1)

Marketplace-side docs (`harness/skills/cross-project/SKILL.md` extensions, sigils matrix) owned by `mkt-vxu9` — out of this plan's scope.

## Sources & References

### Origin

- **Brainstorm:** [`docs/brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md`](../brainstorms/2026-04-21-cross-project-intent-taxonomy-brainstorm.md) — Key decisions carried forward: (1) pair detection = dep edge alone, notes = provenance-only; (2) refs = tunable sigils with default verb; (3) schema bump to v2 with envelope mode + per-record provenance; (4) forward-only backfill for ~5 intentional pairs; (5) labeled corpus + goldens as separate acceptance artifacts; (6) marketplace-side ownership of hook + skill extension under `mkt-gkyn` / `mkt-vxu9`.

### Internal References

- `pkg/view/pair_record.go` — v1 pair reader (template for v2 extension)
- `pkg/view/ref_record.go` — v1 ref reader (template for v2 extension)
- `pkg/analysis/external_resolution.go:102` — `SplitID` primitive
- `cmd/bt/robot_compact_flag.go:34-70` — `--shape` resolver pattern (template for `--schema` and `--sigils` resolvers)
- `cmd/bt/robot_ctx.go:82-87` — `rc.analysisIssues()` composition rule
- `cmd/bt/robot_pairs.go` — v1 pair handler
- `cmd/bt/robot_refs.go` — v1 ref handler
- `cmd/bt/cobra_robot.go:75-87` — flag registration pattern on `robotCmd`
- `cmd/bt/cobra_robot.go:1022` — subcommand init pattern
- `cmd/bt/cobra_robot.go:1175` — subcommand wiring
- `cmd/bt/robot_graph.go:224` — env var docs map (corrected from earlier draft that pointed to `cobra_robot.go:224`)
- `cmd/bt/robot_all_subcommands_test.go:70-141` — flag-matrix test (template for new rows)
- `pkg/view/projections_test.go:57-125` — golden harness (extend for `pair_v2_*` + `ref_v2_*`)
- `pkg/view/schemas/pair_record.v1.json` + `ref_record.v1.json` — single-record schema format (mirror for v2)
- `docs/design/2026-04-20-bt-mhwy-2-pairs.md` — v1 pairs design (authority on identity rule, drift dimensions)
- `docs/design/2026-04-20-bt-mhwy-3-refs.md` — v1 refs design (authority on scope, URL stripping, prefix scoping)
- `docs/brainstorms/2026-04-12-cross-project-management-brainstorm.md` — prior cross-project brainstorm (different scope; portfolio/temporal/deps)

### Related Work

- **Beads:**
  - `bt-gkyn` (pairs v2 reader — primary)
  - `bt-vxu9` (refs v2 reader — primary)
  - `bt-ushd` (epic: cross-project beads OS)
  - `bt-mhwy.2` / `.3` / `.5` (shipped v1 prereqs)
  - `bt-2cvx` (session provenance — shares hook surface with `mkt-gkyn`)
  - `bt-6cfg` (TUI consumer of pair output — downstream)
  - `mkt-gkyn` + `mkt-vxu9` (marketplace-side, filed this session)
  - Upstream: `bd-fjip` (session_history), `bd-e6p` (notification hook), `bd-k8b` (closed — same-ID linking research)

### Memory / conventions

- `project_pair_suffix_collisions.md` — why v1 is noisy
- `feedback_cross_project_bead_pairing.md` — `--id` suffix convention
- `feedback_cross_prefix_deps.md` — bare cross-prefix vs `external:` semantics
- `project_rc_analysis_issues_composition.md` — composition rule
- `project_bt_test_mode_global_contract.md` — pure-helper testing pattern

### External

- No external docs needed. All patterns have mature local precedent.
