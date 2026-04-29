# 72l8 §4: Test fixtures + sample data

## What was scanned

Fixture / sample-data directories under the bt repo (excluding `_tmp/perles/**` and `.claude/worktrees/**`, both of which are out-of-tree scratch / agent worktrees, not first-party fixtures):

- `testdata/graphs/` — synthetic graph topology fixtures (chain_10, complex_20, cycle_5, diamond_5, star_10).
- `testdata/expected/` — corresponding expected metrics JSON.
- `testdata/golden/graph_render/` — Mermaid + SVG golden renders.
- `tests/testdata/` — `minimal.jsonl`, `synthetic_complex.jsonl`, `search_hybrid.jsonl`, plus `tests/testdata/real/{cass,srps}.jsonl` (real-world snapshots from the maintainer's other projects).
- `pkg/analysis/testdata/` — `startup_baseline.json`, `external/two_project_chain.json`, `external/unresolved_external.json`.
- `pkg/view/testdata/` — `fixtures/`, `golden/`, and `corpus/labeled_corpus.json` (full pairs/refs v2 FPR-gate corpus).

No fuzz corpora exist outside of `pkg/view/testdata/corpus/` (Go's standard `testdata/fuzz/` directory is not present anywhere in the tree).

No `pkg/loader/testdata/` directory exists.

## Findings

- (none) — what was found — severity

Greps for `beads_viewer`, `Dicklesworthstone`, `Jeffrey`, `s070681`, the literal `bv-` ID prefix, and a path-based `beads_viewer` substring across all of the above directories returned **zero hits**. The fixtures use current-era prefixes (bt, bd, cass, mkt, dotfiles, tpane, cnvs, cctui, sh-, sess-, n0..n9, A/B, bd-101, plus the long-form coding_agent_session_search-* and system_resource_protection_script-* IDs in the `real/` snapshots).

The `bv-*` references that *do* exist in the repo are in `tests/e2e/*.go` test source code (test names, comment tags, and `repoDir/bv-pages` export-output directory names) — those are out of scope for §4 (Test fixtures + sample data) and belong to §1 (Go source + module paths).

## Stale schema concerns

None in the in-scope fixtures.

Spot-checked the obvious risk surface (session columns + metadata blob):

- `pkg/view/testdata/fixtures/fully_populated_issue.json` and its golden counterpart correctly model **post-Dolt** schema: session data lives in first-class columns (`created_by_session`, `claimed_by_session`, `closed_by_session`) at the top level of the issue object. The fixture's `metadata` blob carries an `unrelated_key` whose explicit purpose is verifying that the projection ignores it — this is the *correct* post-bd-34v / post-bt-5hl9 contract under bt-mhcv's audit framework.
- `tests/testdata/real/{cass,srps}.jsonl` contain no `metadata` field at all — they are pre-session-column real snapshots, but they exercise input parsing only and do not encode any stale schema assumption.
- No fixture references `.beads/sprints.jsonl` or assumes its existence.
- No fixture relies on `--global` failure semantics (bt-vhn2 framing).

## Remediation beads filed

(none — nothing to remediate)

## Notes

- Methodology was read-only grep + spot-read. No files modified.
- Out-of-scope artifacts that did surface during the sweep but belong to other §:
  - `tests/e2e/**/*.go` — extensive `bv-*` references in Go test source (issue IDs in inline JSONL strings, test-name comments, hard-coded `bv-pages` export dir names, `BV_E2E_*` env var names in `harness.sh`, the `<testsuites name="bv-e2e" ...>` JUnit root element). All belong to §1 / §3 follow-up.
  - `tests/e2e/harness.sh` — `BV_E2E_FAIL_COUNT`, `BV_E2E_SKIP_COUNT`, `bv-e2e` suite name. Belongs to §3 (build + packaging) or §1.
- §4 itself can be marked clean with no follow-up beads.
