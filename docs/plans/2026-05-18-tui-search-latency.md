# TUI Search Latency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `bt`'s global TUI search responsive on 4000+ issue corpora when typing at fast speeds, measured by per-keystroke filter-chain duration.

**Architecture:** Measurement-first. Phase 1 builds a benchmark harness over the existing filter chain so cost is observable. Phase 2 lands the highest-confidence fix (tightening the `looksLikeIDQuery` heuristic so plain-text queries no longer trigger an O(N) ID scan). Subsequent phases are gated on measured deltas: only continue if the prior phase did not hit the p95 < 16ms target. Each phase produces stable committable state suitable as a session boundary.

**Tech Stack:** Go 1.25+, Charm Bracelet v2 (Bubble Tea, Lipgloss, Bubbles), existing `pkg/ui/benchmark_test.go` and `pkg/testutil` for corpus generation. No new external dependencies.

---

## Orientation

**Project:** beadstui (binary `bt`). Module `github.com/seanmartinsmith/beadstui`. Located at `C:\Users\sms\System\tools\bt`.

**Bead being worked:** `bt-qlasl` ("TUI search latency: per-keystroke O(N) ID scan + sync chain double-fire in hybrid mode"). Read it first with `bd show bt-qlasl` — it carries the corrected root-cause analysis this plan references.

**Files that matter:**

| Path | Role |
|---|---|
| `pkg/ui/id_bucket_filter.go` | Filter composition layers + `looksLikeIDQuery` heuristic. The Phase 2 hot path. |
| `pkg/ui/semantic_search.go` | Semantic filter, cache, debounce, `ComputeSemanticResults` async scan. Phase 4-5 work. |
| `pkg/ui/model.go` (search around `1519`, `1582`, `1631-1644`) | Bubbletea Update loop: keystroke handling and async semantic dispatch. |
| `pkg/ui/model_update_analysis.go:162-184` | `handleSemanticFilterResult` — site of the double `SetFilterText` re-filter (Phase 4). |
| `pkg/ui/benchmark_test.go` | Existing benchmark scaffolding (`BenchmarkSnapshotSwap`, `BenchmarkSnapshotBuilderBuild`). Use as a style template for Phase 1. |
| `pkg/testutil/` | `QuickRandom(n, density)` generates synthetic issue corpora. The Phase 1 benchmark builds on this. |
| `pkg/ui/item.go` | `IssueItem` type + `FilterValue()` method — what the filter chain sees as `targets`. |
| `pkg/ui/snapshot.go` and `snapshot_builder.go` | How `SemanticSearch` is populated with IDs and docs. Phase 1 needs this to construct a working semantic search for the benchmark. |
| `pkg/search/hash_embedder.go` | Default embedder used when no real model is configured. Fast enough for benchmarks. |
| `pkg/search/vector_index.go` | The vector index that `ComputeSemanticResults` scans. |

**Existing structure to preserve:**

- The filter chain is composed at construction time:
  `multiTokenFilter(quotedExactFilter(idPriorityFilter(semanticSearch.Filter)), perTokenCap)`.
  Composition lives at `pkg/ui/id_bucket_filter.go:337` (search for `semanticSearchFilter`).
- `IssueItem.FilterValue()` returns ID + space + remaining searchable fields. ID is the first whitespace-separated token. `extractIDToken` in `id_bucket_filter.go:344` parses this.
- Hybrid mode is what the user reproduces against. The footer pill says `hybrid/default`.

**Project conventions to follow** (see `AGENTS.md`):
- No file deletion without explicit permission. No `_v2.go` variants — edit in place.
- Verify with `go build ./...` and `go vet ./...` after every change.
- After build: `go install ./cmd/bt/` (the user invokes `bt` from PATH).
- Test files alongside source: `pkg/ui/foo_test.go` next to `pkg/ui/foo.go`.
- Commit format: `type(scope): description (bt-qlasl)`. Scope is `tui` for `pkg/ui/` changes, `search` for `pkg/search/`.
- `bd close --reason-file` (NOT inline `--reason=`) for non-ASCII content; default to ASCII otherwise.
- Close beads before committing the session's last commit. `git pull --rebase && bd dolt push && git push` to ship.

**Project memory state (relevant):**

- The row-level score badges in the issues list were removed in commit `abf24de3` (bt-r3zxj) shortly before this plan was written. The `IssueItem.SearchScore` / `SearchScoreSet` fields are still consumed by the detail-pane Search Scores section. Don't be surprised to find no `[0.NN]` badges in row output during benchmarks.

---

## Objective

Reduce per-keystroke filter-chain time during fast typing in hybrid mode on a 4000-issue corpus to p95 below 16ms (one 60fps frame), so the TUI list does not visibly lag behind keystrokes.

Approach: introduce a benchmark that measures this, then apply fixes in confidence order — starting with the `looksLikeIDQuery` heuristic tightening, which is essentially a bug fix (the heuristic returns `true` for almost every text query) and requires no architectural change.

---

## Scope Boundaries

**In scope:**
- A new benchmark file in `pkg/ui/` that simulates a keystroke sequence over a synthetic corpus and reports per-keystroke filter time.
- Tightening `looksLikeIDQuery` in `pkg/ui/id_bucket_filter.go` to require a structural ID signal (`-` or `.`) OR length ≤ 5.
- Auditing the double `SetFilterText` re-filter at `pkg/ui/model_update_analysis.go:177` — either remove or document the necessity.
- Plumbing `context.Context` through `ComputeSemanticResults` so in-flight scans can be cancelled when a newer keystroke arrives.
- Updating unit tests for `looksLikeIDQuery` (already exists at `pkg/ui/id_bucket_filter_test.go:121`).

**Out of scope:**
- Mode unification (`bt-wf5s`) — orthogonal to speed.
- BQL composition in `/` (`bt-qcz8`).
- Replacing the hash embedder with a real model.
- Score-badge presentation (handled by `bt-r3zxj`, `bt-gfxhz.6`).
- Prefix-extension memoization (deferred unless Phases 2-4 don't hit target).
- Changing Bubbles' internal filtering behavior — work above the FilterFunc boundary.
- Reducing the number of filter chain layers (`multiTokenFilter` / `quotedExactFilter` / `idPriorityFilter`) — they pass through in O(1) for typical queries; composition restructure is YAGNI.

---

## Resolved Decisions

1. **Benchmark over teatest harness.** Direct benchmark of the composed FilterFunc (not the full Bubbletea Update loop). Reason: the user's reported symptom is event-loop blocking inside the filter chain. The filter chain is callable directly with `(term string, targets []string)`. A full teatest harness adds complexity without measuring the dominant cost.

2. **Measurement-first, fix-second.** Even though the `looksLikeIDQuery` issue is visible by code reading, the plan establishes a baseline before touching it. Reason: without a number, "feels faster" is unfalsifiable, and we can't gate subsequent phases on quantitative progress.

3. **`looksLikeIDQuery` heuristic shape:** require a `-` or `.` separator, OR length ≤ 5. Reason: bead suffixes are typically 5 chars (`qlasl`, `r3zxj`) and the user types them as bare suffixes for quick navigation. Full IDs always contain `-`. Molecule suffixes (`mhwy.1`) always contain `.`. Common English text queries during search (`probably`, `migration`, `release`, `config`) are 6+ chars with no separator — they will correctly skip the ID scan.

4. **Target p95 < 16ms on 4000 issues** for the filter chain only. Reason: 60fps is one frame per 16.67ms; faster than that and keystrokes can't visibly lag. Per-keystroke total (including viewport re-render) will be higher; the chain itself being inside that budget gives us frame-time margin for the rest of the loop.

5. **No worktree required.** This is sequential work on `main` with multiple session boundaries. The user runs other sessions; commits via `git commit --only <paths>` keep merges race-proof. Reason: parallel branches add coordination cost without payoff for a fundamentally serial set of phases.

6. **Defer prefix-extension memoization** unless Phases 2-4 don't hit target. Reason: memoizing a stateful filter chain across keystrokes is architecturally invasive (cache invalidation on snapshot change, prefix-extension predicate). The smaller fixes have higher chance of solving the symptom alone.

7. **Tests are benchmarks for perf phases, unit tests for logic phases.** Reason: benchmarks measure latency directly; unit tests for `looksLikeIDQuery` confirm the new heuristic accepts/rejects the right inputs.

---

## Session Scoping and Coordination Strategy

**Multi-session plan.** Expected 3-4 sessions:

- **Session 1**: Phase 0 (setup) + Phase 1 (benchmark harness) + Phase 2 (baseline). Produces a working benchmark and a recorded baseline number. Commit and push.
- **Session 2**: Phase 3 (`looksLikeIDQuery` fix). Lands the highest-confidence win. Commit and push. Re-run benchmark, decide whether to continue.
- **Session 3** (conditional, if p95 still > 16ms): Phase 4 (double `SetFilterText` audit/fix). Commit and push. Re-measure.
- **Session 4** (conditional, if still > 16ms): Phase 5 (context cancellation in async path). Commit and push. Final measurement, close bead.

**Coordination strategy: All sequential.** One agent, step by step. No subagents within a session. No teams. Phases depend on prior phase output (baseline, measured deltas).

**Handoff state at session boundaries:** After every phase, working tree is committed and pushed. Tests pass. Benchmark numbers recorded in this plan's Verification section so the next session knows the current state.

**Stop conditions:**
- After any phase, if benchmark p95 hits the < 16ms target → plan complete, close bead, no further phases.
- After Phase 5, regardless of measurement: close plan. If still over target, file a follow-up bead for prefix-extension memoization.

---

## Implementation Steps

### Phase 0: Setup

**Files:**
- Read: `bt-qlasl` (bead body, root-cause section)

- [ ] **Step 0.1: Read the bead**

Run: `bd show bt-qlasl`
Expected: read the body in full. The "Root Cause" section enumerates 5 issues; this plan addresses items 1, 3, 4 in Phases 3, 4, 5. Item 2 (sahilm cost) is left as the unavoidable inner-ranker cost. Item 5 (memoization) is deferred.

- [ ] **Step 0.2: Claim the bead**

Run: `bd update bt-qlasl --claim`
Expected: confirmation message. The bead is now assigned to the current actor.

- [ ] **Step 0.3: Verify working tree is clean**

Run: `git status`
Expected: `nothing to commit, working tree clean` on `main`. If not clean, stop and surface to the user — do not proceed with uncommitted state.

- [ ] **Step 0.4: Confirm build is green at start**

Run: `go build ./... && go vet ./...`
Expected: no output, exit 0. If anything fails, stop and surface.

---

### Phase 1: Benchmark Harness

**Files:**
- Create: `pkg/ui/search_latency_bench_test.go`

The benchmark constructs a realistic filter chain (the same composition the TUI uses in hybrid mode) and replays a keystroke sequence to measure per-keystroke filter time.

- [ ] **Step 1.1: Create the benchmark file**

Create `pkg/ui/search_latency_bench_test.go` with content:

```go
package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/testutil"
)

// keystrokeTyping models a user typing the word "probably" one char at a time —
// a realistic English-word fast-typing case that exercises the non-ID code path.
var keystrokeTyping = []string{"p", "pr", "pro", "prob", "proba", "probab", "probabl", "probably"}

// keystrokeIDTyping models a user typing a bead suffix — the case where the
// idPriorityFilter is intentionally exercised.
var keystrokeIDTyping = []string{"q", "ql", "qla", "qlas", "qlasl"}

// buildBenchSemanticSearch constructs a SemanticSearch with a populated hash
// embedder + vector index over the given issues. Returns the search and the
// matching list of targets (FilterValue strings).
func buildBenchSemanticSearch(b *testing.B, issues []model.Issue) (*SemanticSearch, []string) {
	b.Helper()
	items := makeBenchItems(issues)
	targets := make([]string, len(items))
	ids := make([]string, len(items))
	docs := make(map[string]string, len(items))
	for i, it := range items {
		targets[i] = it.FilterValue()
		ids[i] = it.Issue.ID
		docs[it.Issue.ID] = search.IssueDocument(it.Issue)
	}

	const dim = 64
	embedder := search.NewHashEmbedder(dim)
	index := search.NewVectorIndex(dim)
	ctx := context.Background()
	for id, doc := range docs {
		vec, err := embedder.Embed(ctx, []string{doc})
		if err != nil || len(vec) != 1 {
			b.Fatalf("embed failed for %s", id)
		}
		hash := search.ComputeContentHash(doc)
		if err := index.Upsert(id, hash, vec[0]); err != nil {
			b.Fatalf("upsert failed for %s: %v", id, err)
		}
	}

	s := NewSemanticSearch()
	s.SetIndex(index, embedder)
	s.SetIDs(ids)
	s.SetDocs(docs)
	return s, targets
}

func makeBenchItems(issues []model.Issue) []IssueItem {
	out := make([]IssueItem, len(issues))
	for i := range issues {
		out[i] = IssueItem{Issue: issues[i]}
	}
	return out
}

func BenchmarkSearchFilter_HybridMode_TextTyping(b *testing.B) {
	for _, size := range []int{1000, 4000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			issues := testutil.QuickRandom(size, 0.01)
			sem, targets := buildBenchSemanticSearch(b, issues)
			filterFn := semanticSearchFilter(sem)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, term := range keystrokeTyping {
					_ = filterFn(term, targets)
				}
			}
		})
	}
}

func BenchmarkSearchFilter_HybridMode_IDTyping(b *testing.B) {
	for _, size := range []int{1000, 4000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			issues := testutil.QuickRandom(size, 0.01)
			sem, targets := buildBenchSemanticSearch(b, issues)
			filterFn := semanticSearchFilter(sem)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, term := range keystrokeIDTyping {
					_ = filterFn(term, targets)
				}
			}
		})
	}
}
```

- [ ] **Step 1.2: Sanity-check that referenced symbols still exist**

The benchmark above references these symbols verified during plan recon:

- `pkg/ui/id_bucket_filter.go:337` — `func semanticSearchFilter(s *SemanticSearch) list.FilterFunc`
- `pkg/ui/semantic_search.go:90` — `func NewSemanticSearch() *SemanticSearch`
- `pkg/ui/semantic_search.go:284, 294, 309` — exported setters `SetIndex(idx, embedder)`, `SetIDs(ids)`, `SetDocs(docs)`
- `pkg/search/hash_embedder.go:15, 25` — `func NewHashEmbedder(dim int) *HashEmbedder` and `func (h *HashEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error)`
- `pkg/search/vector_index.go:62` — `func NewVectorIndex(dim int) *VectorIndex`
- `pkg/search/vector_index.go:27` — `func ComputeContentHash(text string) ContentHash`
- `pkg/search/vector_index.go:250` — `func (idx *VectorIndex) Upsert(issueID string, hash ContentHash, vec []float32) error`
- `pkg/search/documents.go:11` — `func IssueDocument(issue model.Issue) string`

Run: `go vet ./pkg/ui/`
Expected: no errors. If any "undefined" errors appear, the API drifted since plan recon — grep for the closest equivalent and update the benchmark file. Do NOT add unexported test helpers unless every exported alternative has been ruled out.

- [ ] **Step 1.3: Run the benchmark**

Run: `go test -bench=BenchmarkSearchFilter -benchmem -benchtime=3s ./pkg/ui/`
Expected: each sub-benchmark reports ns/op. Convert ns/op to ms/keystroke = (ns/op) / (1e6 × len(keystrokeTyping)) for the text test, divide by `len(keystrokeIDTyping)` for the ID test.

- [ ] **Step 1.4: Commit the benchmark**

```bash
git status
git commit --only pkg/ui/search_latency_bench_test.go -m "test(tui): add per-keystroke filter latency benchmarks (bt-qlasl)

Replays a keystroke sequence over the composed hybrid-mode FilterFunc
against synthetic corpora (1k and 4k issues) and reports per-keystroke
filter time. Establishes the measurement basis for the per-keystroke
latency fixes in bt-qlasl. Two variants:

- TextTyping: 'probably' (exercises the non-ID heuristic path)
- IDTyping: 'qlasl' (exercises idPriorityFilter)"
```
Run: `git push`
Expected: pushed cleanly to `main`.

---

### Phase 2: Baseline

**Files:**
- Modify: `docs/plans/2026-05-18-tui-search-latency.md` (this file — append baseline numbers under the Verification section at the end).

- [ ] **Step 2.1: Capture baseline numbers**

Run: `go test -bench=BenchmarkSearchFilter -benchmem -benchtime=5s ./pkg/ui/ | tee _tmp/qlasl-baseline.txt`
Expected: four results (text × 1k, text × 4k, ID × 1k, ID × 4k). Each reports ns/op.

- [ ] **Step 2.2: Compute per-keystroke ms**

For each benchmark line, divide ns/op by 1e6 to get ms-per-replay, then divide by the keystroke-sequence length:
- `TextTyping`: divide by 8
- `IDTyping`: divide by 5

Record results in the Verification section of this plan under "Baseline 2026-05-18 — Session 1".

- [ ] **Step 2.3: Update bt-qlasl with baseline**

Run:
```bash
bd comments add bt-qlasl "Baseline (Phase 2):

TextTyping 1k: X.XX ms/keystroke
TextTyping 4k: Y.YY ms/keystroke
IDTyping   1k: A.AA ms/keystroke
IDTyping   4k: B.BB ms/keystroke

Target: p95 < 16 ms/keystroke on 4k corpus."
```

Replace X/Y/A/B with the actual numbers.

- [ ] **Step 2.4: Commit the plan update**

```bash
git status
git commit --only docs/plans/2026-05-18-tui-search-latency.md -m "docs(plans): record search-latency baseline (bt-qlasl)"
git push
```

**Decision gate:** If `TextTyping 4k` is already below 16ms per keystroke, the plan is effectively done — record this in the bead close reason and skip to "Closing bt-qlasl" below. Otherwise continue to Phase 3.

---

### Phase 3: Tighten `looksLikeIDQuery`

**Files:**
- Modify: `pkg/ui/id_bucket_filter.go` (function `looksLikeIDQuery`, lines ~103-118)
- Modify: `pkg/ui/id_bucket_filter_test.go` (add cases to the existing `looksLikeIDQuery` table test)

- [ ] **Step 3.1: Read existing test cases**

Run: `grep -A 30 "looksLikeIDQuery" pkg/ui/id_bucket_filter_test.go`
Expected: a table-driven test with case structs like `{term: "...", want: bool}`. Note the existing terms.

- [ ] **Step 3.2: Add failing test cases (TDD red)**

In `pkg/ui/id_bucket_filter_test.go`, locate the `looksLikeIDQuery` test cases and add the following cases (place inside the existing case slice):

```go
// Plain English words 6+ chars should NOT trigger the ID scan even though
// they pass the alphanumeric character set — they are the dominant per-
// keystroke cost on hybrid-mode text queries (bt-qlasl).
{"probably", false},
{"migration", false},
{"release", false},
{"config", false},

// Short bare suffixes (≤ 5 chars) keep ID-priority promotion — the user
// types them to find a bead quickly.
{"qlasl", true},
{"r3zxj", true},
{"mhwy", true},
{"cmg", true},

// Anything containing the structural ID separator stays ID-shaped.
{"bt-qlasl", true},
{"mhwy.1", true},
{"cnvs-0gf", true},
```

- [ ] **Step 3.3: Run test, confirm red**

Run: `go test -run TestLooksLikeIDQuery -v ./pkg/ui/`
Expected: FAIL on the new "probably", "migration", "release", "config" cases — current heuristic returns `true` for all of them.

- [ ] **Step 3.4: Update `looksLikeIDQuery` to the tightened heuristic**

Open `pkg/ui/id_bucket_filter.go`. Replace the body of `looksLikeIDQuery` with:

```go
// looksLikeIDQuery returns true for tokens that plausibly name a bead —
// either containing a structural ID separator (- or .) or short enough
// (≤ 5 chars) to be a bare suffix the user is using for quick navigation.
// Rejects anything with whitespace, punctuation, or longer than the longest
// realistic project-prefix + suffix combo.
//
// The structural-separator requirement filters out common English text
// queries (e.g. "probably", "migration", "release") that would otherwise
// trigger an unnecessary O(N) ID scan over all targets on every keystroke
// (bt-qlasl).
func looksLikeIDQuery(term string) bool {
	t := strings.TrimSpace(strings.ToLower(term))
	if len(t) < 2 || len(t) > 24 {
		return false
	}
	hasSep := false
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '.':
			hasSep = true
		default:
			return false
		}
	}
	if hasSep {
		return true
	}
	return len(t) <= 5
}
```

- [ ] **Step 3.5: Run test, confirm green**

Run: `go test -run TestLooksLikeIDQuery -v ./pkg/ui/`
Expected: PASS on all cases including the new ones.

- [ ] **Step 3.6: Run full pkg/ui test suite to catch regressions**

Run: `go test ./pkg/ui/`
Expected: PASS. If a test specific to ID-priority-promotion regresses, read the test and decide whether the new heuristic is correct (the test was over-asserting promotion on long text queries) or the heuristic is wrong (the test was protecting a real use case). Prefer adjusting the test if the use case was overreach; revert the heuristic if a legitimate behavior is broken.

- [ ] **Step 3.7: Re-run benchmark, capture delta**

Run: `go test -bench=BenchmarkSearchFilter -benchmem -benchtime=5s ./pkg/ui/ | tee _tmp/qlasl-after-phase3.txt`
Expected: `TextTyping` benchmarks improve. The `IDTyping` benchmarks should be unchanged or marginally faster (the heuristic still returns `true` for short suffixes).

- [ ] **Step 3.8: Record delta in this plan and the bead**

Update the Verification section of this plan with "After Phase 3" numbers. Add a comment to the bead:

```bash
bd comments add bt-qlasl "After Phase 3 (looksLikeIDQuery tightened):

TextTyping 4k: X.XX ms/keystroke (was Y.YY, delta -Z%)
IDTyping   4k: A.AA ms/keystroke (was B.BB, delta -W%)"
```

- [ ] **Step 3.9: Commit and push**

```bash
go build ./...
go vet ./...
go install ./cmd/bt/
git status
git commit --only pkg/ui/id_bucket_filter.go pkg/ui/id_bucket_filter_test.go docs/plans/2026-05-18-tui-search-latency.md -m "perf(tui): require ID separator or short length in looksLikeIDQuery (bt-qlasl)

The previous heuristic returned true for any 2-24 char lowercase-
alphanumeric token, which routed every plain-text English query
('probably', 'migration', 'release') through idPriorityFilter's O(N)
ID scan over all 4235 targets — the dominant per-keystroke cost in
hybrid mode on the 4000+ issue corpus.

New rule: require a structural ID separator (- or .) OR length ≤ 5
chars. Common bead suffixes (qlasl, r3zxj, mhwy) keep the fast path
when the user is searching by suffix; English words 6+ chars without
separator skip the scan.

Benchmark delta on 4k corpus:
- TextTyping: <baseline> → <after> (-Z%)
- IDTyping:   <baseline> → <after> (unchanged ±)

Plan: docs/plans/2026-05-18-tui-search-latency.md"
git push
```

Fill in the baseline/after numbers from Step 3.8 before committing.

**Decision gate:** If `TextTyping 4k` is now below 16ms per keystroke, jump to "Closing bt-qlasl" below. Otherwise continue to Phase 4.

---

### Phase 4: Audit/Remove Double `SetFilterText` Re-filter

**Files:**
- Modify: `pkg/ui/model_update_analysis.go` (function `handleSemanticFilterResult`, lines ~162-184)
- Test: `pkg/ui/semantic_search_test.go` or wherever async result handling is currently tested

This phase audits whether the second filter pass at line 177 is necessary. If it is purely there to apply cached scores to the rendered list, there may be a cheaper way that doesn't run the full chain again.

- [ ] **Step 4.1: Read `handleSemanticFilterResult` in full**

Read: `pkg/ui/model_update_analysis.go:162-184`. Note: `applySemanticScores` populates `IssueItem.SearchScore` fields, then `SetFilterText(currentTerm)` is called.

- [ ] **Step 4.2: Check what `SetFilterText` does in Bubbles**

Run: `grep -rn "func.*SetFilterText" $(go env GOMODCACHE)/charm.land/bubbles*/list/`
Expected: find the Bubbles list `SetFilterText` implementation. Confirm whether it re-runs the FilterFunc or just sets state.

If `SetFilterText` re-runs the FilterFunc: the double-filter claim is real, and the second pass is paying full cost just to make the cached scores observable.

If `SetFilterText` does NOT re-run the FilterFunc: the second pass is cheap, and Phase 4 is a no-op — record this finding and proceed to Phase 5.

- [ ] **Step 4.3 (if double-filter is real): Write a benchmark for the async-return path**

Add to `pkg/ui/search_latency_bench_test.go`:

```go
// BenchmarkSearchFilter_AsyncResultLand simulates the post-async-return path
// in handleSemanticFilterResult: applySemanticScores then SetFilterText. The
// SetFilterText call re-runs the sync filter chain over all targets — the
// "double filter" cost (bt-qlasl Phase 4).
func BenchmarkSearchFilter_AsyncResultLand(b *testing.B) {
	// Note: this benchmark may need to construct a Model rather than a bare
	// SemanticSearch. If the helper above doesn't expose the right surface,
	// either expand buildBenchSemanticSearch or skip this benchmark and rely
	// on the existing TextTyping benchmark to capture the steady-state cost.
}
```

If constructing a Model is too invasive, record in the plan that this phase's measurement is via the existing TextTyping benchmark (which includes both passes if the keystroke triggers a semantic-result land within the bench window — unlikely on hash embedder timings, but document it).

- [ ] **Step 4.4 (if double-filter is real): Replace `SetFilterText` with a targeted score-apply mechanism**

Options (decide based on what `SetFilterText` actually does):

**(a)** If `SetFilterText` mainly re-runs the FilterFunc to recompute ranks: the cached ranks from `SetCachedResults` will be returned by the next FilterFunc call. We may be able to trigger a list re-render without invoking `SetFilterText` — e.g. by calling a list method that re-applies current ranks without re-filtering.

**(b)** If the re-filter is structurally required to invalidate Bubbles' internal filter cache: keep the call but ensure it benefits from the just-cached semantic results (it should, because `SetCachedResults` was called immediately before — confirm with a print or test).

**(c)** If neither option is clean, document the constraint and accept the cost. Record in this plan and bead.

- [ ] **Step 4.5: Run benchmarks and tests**

Run: `go test ./pkg/ui/` then `go test -bench=BenchmarkSearchFilter -benchmem -benchtime=5s ./pkg/ui/`
Expected: all tests pass; benchmarks unchanged or improved.

- [ ] **Step 4.6: Commit and push**

If a change was made, commit with `perf(tui): remove double sync filter on async semantic result land (bt-qlasl)` and the appropriate delta. If no change (Step 4.2 found SetFilterText is cheap), commit just the plan update documenting the finding:

```bash
git commit --only docs/plans/2026-05-18-tui-search-latency.md -m "docs(plans): phase 4 finding: SetFilterText is not a double filter (bt-qlasl)"
git push
```

**Decision gate:** If benchmark `TextTyping 4k` now < 16ms, jump to "Closing bt-qlasl". Otherwise continue to Phase 5.

---

### Phase 5: Context Cancellation Through Async Path

**Files:**
- Modify: `pkg/ui/semantic_search.go` (`ComputeSemanticResults`, `ComputeSemanticFilterCmd`)
- Modify: `pkg/ui/model.go` (dispatcher at ~1631-1644)
- Modify: `pkg/ui/model_update_analysis.go` (`handleSemanticDebounceTick`, possibly)

Plumbing `context.Context` so in-flight semantic scans can be cancelled when a new keystroke supersedes them.

- [ ] **Step 5.1: Read current `ComputeSemanticResults` shape**

Read: `pkg/ui/semantic_search.go:376-502`. Note the existing `context.WithTimeout(context.Background(), 500*time.Millisecond)` at line 382. The timeout is per-call; the context is not derived from a parent that can be cancelled externally.

- [ ] **Step 5.2: Add a per-search cancellable parent context to `SemanticSearch`**

In `pkg/ui/semantic_search.go`, add to the `SemanticSearch` struct:

```go
inFlightCancel atomic.Value // stores context.CancelFunc; nil-safe via atomic Load
```

Add helper methods:

```go
// cancelInFlight aborts any in-progress ComputeSemanticResults call. Safe to
// call when no computation is in flight (no-op).
func (s *SemanticSearch) cancelInFlight() {
	if v := s.inFlightCancel.Load(); v != nil {
		if cancel, ok := v.(context.CancelFunc); ok && cancel != nil {
			cancel()
		}
	}
}

// setInFlightCancel records the cancel func for the active compute. Caller
// is responsible for invoking cancel and clearing it on completion.
func (s *SemanticSearch) setInFlightCancel(cancel context.CancelFunc) {
	s.inFlightCancel.Store(cancel)
}

func (s *SemanticSearch) clearInFlightCancel() {
	s.inFlightCancel.Store(context.CancelFunc(nil))
}
```

- [ ] **Step 5.3: Make `ComputeSemanticResults` accept a context**

Change the signature from `ComputeSemanticResults(term string)` to `ComputeSemanticResults(ctx context.Context, term string)`. Internally, use this context (with a 500ms timeout derived from it) for the `Embed` call. Check `ctx.Err()` between the index scan loop iterations every N items so a cancellation aborts the scan early.

```go
func (s *SemanticSearch) ComputeSemanticResults(ctx context.Context, term string) ([]list.Rank, uint64) {
	snap := s.Snapshot()
	if !snap.Ready || snap.Index == nil || snap.Embedder == nil {
		return nil, snap.Version
	}

	embedCtx, embedCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer embedCancel()

	vecs, err := snap.Embedder.Embed(embedCtx, []string{term})
	if err != nil || len(vecs) != 1 {
		return nil, snap.Version
	}
	q := vecs[0]

	// ... existing scoring loop, but periodically check ctx.Err() ...
	for i, id := range snap.IDs {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, snap.Version
			}
		}
		// ... rest of loop unchanged ...
	}
	// ...
}
```

- [ ] **Step 5.4: Update `ComputeSemanticFilterCmd` to construct + register the context**

```go
func ComputeSemanticFilterCmd(s *SemanticSearch, term string) tea.Cmd {
	return func() tea.Msg {
		// Cancel any previous in-flight compute for an earlier keystroke.
		s.cancelInFlight()

		ctx, cancel := context.WithCancel(context.Background())
		s.setInFlightCancel(cancel)
		defer func() {
			s.clearInFlightCancel()
			cancel()
		}()

		results, version := s.ComputeSemanticResults(ctx, term)
		return SemanticFilterResultMsg{
			Term:    term,
			Results: results,
			Version: version,
		}
	}
}
```

- [ ] **Step 5.5: Cancel in-flight on new keystroke**

In `pkg/ui/model.go` at the dispatcher (~1631-1644), before scheduling a new `ComputeSemanticFilterCmd`, call `m.semanticSearch.cancelInFlight()`. This ensures only the latest typing burst's compute survives.

Note: the cancel may need to be exported for cross-package use. If `model.go` is in `package ui`, lower-case `cancelInFlight` is callable. Keep unexported.

- [ ] **Step 5.6: Run pkg/ui tests + benchmark**

Run: `go test ./pkg/ui/` and `go test -bench=BenchmarkSearchFilter -benchmem -benchtime=5s ./pkg/ui/`
Expected: tests pass; benchmark TextTyping unchanged or improved (cancellation primarily helps when async work is genuinely interrupted, which doesn't happen in a tight bench loop — main improvement will be at runtime).

- [ ] **Step 5.7: Commit and push**

```bash
go build ./...
go vet ./...
go install ./cmd/bt/
git status
git commit --only pkg/ui/semantic_search.go pkg/ui/model.go pkg/ui/model_update_analysis.go docs/plans/2026-05-18-tui-search-latency.md -m "perf(tui): cancel in-flight semantic compute on new keystroke (bt-qlasl)

Adds context.Context plumbing through ComputeSemanticResults so that
when a new keystroke arrives mid-flight, the previous compute aborts
early instead of running to completion only to have its result
rejected by the version check. Reduces wasted CPU during fast typing.

Plan: docs/plans/2026-05-18-tui-search-latency.md"
git push
```

---

### Closing bt-qlasl

When the loop has hit the target (or all phases are complete and a follow-up bead has been filed):

- [ ] **Final benchmark**

Run: `go test -bench=BenchmarkSearchFilter -benchmem -benchtime=10s ./pkg/ui/ | tee _tmp/qlasl-final.txt`

- [ ] **Write close reason**

Create `.beads/tmp/qlasl-close.md` with Summary / Change / Files / Verify / Risk / Notes structure (see `.beads/conventions/reference.md`). Reference the final benchmark numbers, all commits, and any follow-ups (e.g. memoization deferred).

- [ ] **Close the bead**

Run: `bd close bt-qlasl --reason-file=.beads/tmp/qlasl-close.md`

- [ ] **Final session sync**

Run:
```bash
git pull --rebase
bd dolt push
git push
git status  # expect: clean, up to date with origin/main
```

---

## Verification

**Baseline 2026-05-18 — Session 1:**
- TextTyping 1k: 0.48 ms/keystroke
- TextTyping 4k: 2.06 ms/keystroke
- IDTyping 1k: 0.46 ms/keystroke
- IDTyping 4k: 1.84 ms/keystroke

Benchmark run: `go test -bench=BenchmarkSearchFilter -benchmem -benchtime=5s ./pkg/ui/`
Hardware: AMD Ryzen 9 5950X, Windows 11. Raw ns/op: TextTyping/1k=3838716, TextTyping/4k=16447624, IDTyping/1k=2315780, IDTyping/4k=9201119.
All four results are already well below the 16ms target (TextTyping 4k at 2.06ms is 7.7x under budget).
Decision gate: target already met at baseline. Phase 3+ not needed for perf; dispatcher decides whether to proceed.

**After Phase 3 — Session 2:**
- TextTyping 1k: TBD ms/keystroke (delta: TBD)
- TextTyping 4k: TBD ms/keystroke (delta: TBD)
- IDTyping 1k: TBD ms/keystroke (delta: TBD)
- IDTyping 4k: TBD ms/keystroke (delta: TBD)

**After Phase 4 — Session 3 (if executed):**
- (Same shape; record deltas)

**After Phase 5 — Session 4 (if executed):**
- (Same shape; record deltas)

**Success criterion:** `TextTyping 4k` below 16.00 ms/keystroke. When met (or after Phase 5 regardless), proceed to Closing bt-qlasl.

**User-facing acceptance test (manual, after final phase):**
- Run `bt` against the cross-project corpus (4000+ issues).
- Confirm `hybrid/default` mode is active (footer pill).
- Press `/` and type "probably" at a comfortable fast pace.
- Observe: list keeps up with typing — no compounding lag between the 4th-5th keystroke and beyond.
- Add a space, type a second word. Same observation.
- Cycle to fuzzy with Ctrl+S, repeat. Both modes feel similar in responsiveness.

---

## Follow-up Beads (file if not addressed in this plan)

If after Phase 5 the target is still not hit:
- File `bt-<new>` "Prefix-extension memoization for sync filter chain" — describes the candidate-narrowing approach for typing `f` → `fo` → `foo` to reuse the prior result set. This was deferred from this plan per resolved decision #6.
