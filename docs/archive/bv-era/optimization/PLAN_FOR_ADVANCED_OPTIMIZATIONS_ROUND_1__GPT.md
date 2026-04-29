# PLAN_FOR_ADVANCED_OPTIMIZATIONS_ROUND_1__GPT.md

This doc is the “round 1” performance plan for `bv`, focused on **needle-moving latency/throughput** improvements while staying **provably isomorphic** (same outputs for the same inputs), with one explicit relaxation:

> The user accepts “extremely close numerical stuff” as “the same” **within epsilon**.

Accordingly, this plan uses a strict workflow:

**Baseline → Profile → Hotspot proof → Opportunity matrix → Minimal diff → Guardrails**

Primary scope:
- `cmd/bv` (CLI + robot protocol)
- `pkg/analysis` (graph + metrics + scoring)
- `pkg/loader` (JSONL loading)
- `pkg/ui` (TUI; background worker + snapshot builder)

## Executive Summary (hybridized with OPUS plan)

### What the data says (baseline + profiles)
- The dominant hotspot is **approximate betweenness**: `pkg/analysis.ApproxBetweenness` → `pkg/analysis.singleSourceBetweenness`.
  - In alloc profiles for `BenchmarkRobotTriage_Sparse500`, `singleSourceBetweenness` is ~70% of alloc_space, and CPU is dominated by GC + map growth sourced from that routine.
- For “typical” repos this project is already very fast, but large graphs still show **gross headroom**:
  - Real data (570 issues): p50 ~52.6ms; peak RSS ~37.5MB.
  - Synthetic large (5000 issues): p95 ~1.52s; peak RSS ~429MB; `--profile-startup` shows Phase 2 dominated by **k-core (~263ms)** and **betweenness (~179ms)** plus slack/eigenvector.

### Recommended Round 1 strategy (one lever per change)
We take the best “minimal diff” lever from `PLAN_FOR_ADVANCED_OPTIMIZATIONS_ROUND_1__OPUS.md` and combine it with the broader scaling view from this doc:

1) **Change 1A (lowest risk, highest ROI): buffer pooling for Brandes’ inner loop**
   - Reuse the 4 per-pivot maps + BFS slices in `singleSourceBetweenness` via `sync.Pool`.
   - This is the smallest diff that attacks the alloc/GC root cause and is easiest to prove isomorphic.
2) **Change 1B (bigger win if still hot): array-based Brandes + cached adjacency**
   - Dense node indexing + precomputed sorted adjacency lists, eliminating most remaining per-pivot work (including gonum iterator churn).
3) **Only after re-profiling**: address secondary large-graph costs (k-core graph building / slack reuse) if they become top-3 hotspots post betweenness fix.

### Expected impact (conservative, based on measured shares)
- Since `singleSourceBetweenness` is ~70% of alloc_space in triage-like profiles, even a conservative 60–80% reduction in that component should cut overall allocation volume by ~40–55%, collapsing GC time and improving p95/throughput.
- If post-Change 1A profiles show gonum iterator churn becomes dominant, Change 1B (cached adjacency) is the next “clean” lever.

---

## 0) Hard Constraints (repo + methodology)

### Repo invariant (ABSOLUTE)
- **Do not delete any file or directory** unless the user provides the **exact command** in this session. (Repo policy.)
- Avoid broad refactors; prefer **one performance lever per change**.

### Methodology invariants (A→G)
A) **Baseline first**: tests + representative workload; record p50/p95/p99 latency, throughput, peak memory (exact commands).  
B) **Profile before proposing**: capture CPU + allocation + I/O; identify top 3–5 hotspots by % time/bytes.  
C) **Equivalence oracle**: explicit golden outputs + invariants; for large input spaces, use metamorphic/property tests.  
D) **Isomorphism proof per change**: short proof sketch covering ordering, tie-breaking, floats, RNG, “now”.  
E) **Opportunity matrix**: rank by (Impact × Confidence) / Effort, focus on p95+ and throughput wins.  
F) **Minimal diffs**: one lever per change; include rollback guidance.  
G) **Regression guardrails**: benchmarks/thresholds or CI hooks to prevent backsliding.

---

## 1) Architecture Snapshot (how the system actually executes)

### 1A) Data plane (robot + TUI share the same engine)
1) **Load issues** from `.beads/issues.jsonl` (preferred) or `.beads/beads.jsonl` fallback:
   - `pkg/loader/loader.go` (`GetBeadsDir`, `FindJSONLPath`, `LoadIssues`, `ParseIssuesWithOptions`)
2) **Build graph** + compute metrics via Analyzer:
   - `pkg/analysis/graph.go` (`NewAnalyzer`, `AnalyzeAsync` / `AnalyzeWithProfile`)
3) **Derive higher-level outputs**:
   - unified triage: `pkg/analysis/triage.go` (`ComputeTriageFromAnalyzer`)
   - priority tuning + feedback: `pkg/analysis/priority.go`, `pkg/analysis/feedback.go`
4) **Presentation**:
   - robot JSON: `cmd/bv/main.go` (`--robot-*`)
   - TUI snapshot pipeline: `pkg/ui/background_worker.go` → `pkg/ui/snapshot.go` (build snapshot, run Phase 2, swap pointer)

### 1B) Two-phase analysis contract (critical for “responsiveness”)
- **Phase 1 (sync, “instant”)**: degree, topo sort, density (enough for initial render and many filters).
- **Phase 2 (async)**: PageRank, Betweenness, Eigenvector, HITS, Critical Path, Cycles (plus extras: k-core, slack, articulation).
- Size-aware config selection: `pkg/analysis/config.go` (`ConfigForSize`).
- Robot outputs include `status` so downstream consumers can tell computed vs approx vs timeout vs skipped.

---

## 2) Baselines (A): commands + recorded numbers (this environment)

### 2A) Environment
Exact commands:
```bash
go version
grep -m1 'model name' /proc/cpuinfo
nproc
free -h | head -n 2
```
Observed:
- Go: `go1.25.5 linux/amd64`
- CPU: `AMD Ryzen Threadripper PRO 5975WX 32-Cores`
- `nproc`: `64`
- RAM: `499Gi`

### 2B) Correctness baseline (tests)
Exact command:
```bash
BV_NO_BROWSER=1 BV_TEST_MODE=1 go test ./...
```
Observed: PASS (notably `tests/e2e` took ~132s in this run).

### 2C) Representative workload definition(s)
We baseline three “robot triage” datasets to cover realistic + scaling:

1) **Real project data (this repo)**:
- `.beads/issues.jsonl`: `570` issues, `1.5MB`

2) **Synthetic medium**:
- `tests/testdata/benchmark/medium.jsonl`: `1000` issues, `3.4MB`

3) **Synthetic large**:
- `tests/testdata/benchmark/large.jsonl`: `5000` issues, `32MB`

For synthetic datasets we copy the JSONL into a temp BEADS_DIR with a single `issues.jsonl` to avoid loader auto-selecting another file.

### 2D) Build (no overwriting repo artifacts)
Exact command:
```bash
go build -o /tmp/bv_round1 ./cmd/bv
```

### 2E) Latency distribution + throughput (p50/p95/p99)
Measurement harness (exact shape used):
```bash
python3 - <<'PY'
import math
import os
import statistics
import subprocess
import time

cmd = ['/tmp/bv_round1', '--robot-triage']
env = os.environ.copy()
env.update({'BV_ROBOT': '1', 'BV_NO_BROWSER': '1', 'BV_TEST_MODE': '1'})
# optionally set env['BEADS_DIR'] = ... for synthetic sets

warmup = 5
runs = 50

for _ in range(warmup):
    subprocess.run(
        cmd,
        cwd='/data/projects/beads_viewer',
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=True,
    )

samples = []
for _ in range(runs):
    t0 = time.perf_counter()
    subprocess.run(
        cmd,
        cwd='/data/projects/beads_viewer',
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=True,
    )
    samples.append(time.perf_counter() - t0)

samples.sort()

def pct(p):
    k = (len(samples) - 1) * p / 100.0
    f = math.floor(k)
    c = math.ceil(k)
    if f == c:
        return samples[int(k)]
    d = k - f
    return samples[f] * (1 - d) + samples[c] * d

print('p50_s', pct(50))
print('p95_s', pct(95))
print('p99_s', pct(99))
print('mean_s', statistics.mean(samples))
print('throughput_runs_per_s', runs / sum(samples))
PY
```

Recorded results:

| Dataset | p50 | p95 | p99 | Throughput |
|---|---:|---:|---:|---:|
| `.beads/issues.jsonl` (570) | 52.599ms | 56.733ms | 59.216ms | 18.88 runs/s |
| `benchmark/medium.jsonl` (1000) | 153.632ms | 158.724ms | 163.705ms | 6.53 runs/s |
| `benchmark/large.jsonl` (5000) | 1.457s | 1.518s | 1.540s | 0.68 runs/s |

### 2F) Peak memory (process RSS)
Exact command:
```bash
/usr/bin/time -v env BV_ROBOT=1 BV_NO_BROWSER=1 BV_TEST_MODE=1 /tmp/bv_round1 --robot-triage >/dev/null
```
Recorded “Maximum resident set size”:

| Dataset | Peak RSS |
|---|---:|
| `.beads/issues.jsonl` (570) | 38,388 kB (~37.5MB) |
| `benchmark/medium.jsonl` (1000) | 42,928 kB (~41.9MB) |
| `benchmark/large.jsonl` (5000) | 439,660 kB (~429MB) |

Interpretation:
- Typical repos are extremely fast already (p50 ~53ms).
- Scaling to 5k issues shows a **major memory spike** and >1s latency, meaning there is still “gross” headroom for large projects.

### 2G) Startup breakdown: load vs metrics (`--profile-startup`)
Exact command:
```bash
env BV_ROBOT=1 BV_NO_BROWSER=1 BV_TEST_MODE=1 /tmp/bv_round1 --profile-startup --profile-json
```
Note: In some PTY environments, terminal capability probes may emit escape sequences before the JSON; if you need guaranteed clean JSON, force non-TTY stdout (e.g., pipe/redirect).

Observed (real data, 570 nodes):
- `load_jsonl`: ~12.0ms
- `total_with_load`: ~27.1ms
- `betweenness`: ~9.1ms (≈63% of Phase 2 time)

Observed (synthetic large, 5000 nodes, density≈0.00999):
- `load_jsonl`: ~347.5ms
- `total_with_load`: ~1.123s
- dominant Phase 2 costs: k-core (~262.7ms), betweenness (~178.7ms), slack (~96.4ms), eigenvector (~78.0ms), PageRank (~48.3ms)

### 2H) Extended microbenchmark snapshot (pkg/analysis)
This is the “OPUS plan” strength: a wide bench sweep that makes the bottleneck undeniable, and gives us regression guardrails.

Exact command used (targets a representative subset; `-count=3` for stability):
```bash
BV_NO_BROWSER=1 BV_TEST_MODE=1 go test ./pkg/analysis -run '^$' \
  -bench '^(BenchmarkApproxBetweenness_500nodes_(Exact|Sample100|Sample50)|BenchmarkFullAnalysis_Sparse500|BenchmarkGenerateAllSuggestions_Medium|BenchmarkCycles_ManyCycles20|BenchmarkCycles_ManyCycles30|BenchmarkDetectCycleWarnings_Medium|BenchmarkFindCyclesSafe_Large|BenchmarkSuggestLabels_LargeSet|BenchmarkComplete_Betweenness15|BenchmarkComplete_PageRank20)$' \
  -benchmem -count=3
```

Recorded (mean over `-count=3`, rounded):

| Benchmark | ns/op | allocs/op | B/op |
|---|---:|---:|---:|
| ApproxBetweenness_500nodes_Exact | 66,386,288 | 499,560 | 34,110,547 |
| ApproxBetweenness_500nodes_Sample100 | 13,242,222 | 199,551 | 29,575,730 |
| ApproxBetweenness_500nodes_Sample50 | 5,456,221 | 100,756 | 14,841,992 |
| FullAnalysis_Sparse500 | 11,989,257 | 222,834 | 98,705,820 |
| GenerateAllSuggestions_Medium | 6,622,519 | 49,279 | 3,856,843 |
| DetectCycleWarnings_Medium | 209,281 | 1,435 | 140,584 |
| FindCyclesSafe_Large | 140,131 | 1,417 | 103,872 |
| SuggestLabels_LargeSet | 1,582,094 | 11,342 | 607,904 |
| Complete_Betweenness15 | 258,745 | 4,087 | 1,723,107 |
| Complete_PageRank20 | 32,577 | 513 | 248,696 |
| Cycles_ManyCycles20 | 149,214,974 | 294,827 | 120,980,771 |

Pathology note (`BenchmarkCycles_ManyCycles30`):
- `ns/op` is stable (~500ms) but allocations vary wildly run-to-run (it is intentionally a pathological benchmark):
  - `B/op`: min 1,004,341,976; median 1,998,106,040; max 3,103,384,328
  - `allocs/op`: min 1,904,844; median 3,980,031; max 5,710,612
This is a reminder that some graph problems are inherently “blow up” shaped and must remain guarded by timeouts and caps.

---

## 3) Profiles (B): CPU + alloc + I/O and top hotspots

### 3A) CPU + allocation profiles via pprof (bench-driven)
We use benches as the reproducible profiling harness (faster iteration than sampling full CLI).

Robot-like bench used:
- `pkg/analysis/bench_test.go`: `BenchmarkRobotTriage_Sparse500`

Exact commands (timestamps omitted for readability):
```bash
BV_NO_BROWSER=1 BV_TEST_MODE=1 go test ./pkg/analysis -run '^$' -bench '^BenchmarkRobotTriage_Sparse500$' \
  -benchtime=3s -cpuprofile /tmp/bv_cpu_triage_sparse500.prof -memprofile /tmp/bv_mem_triage_sparse500.prof

go tool pprof -top -cum -nodecount=25 /tmp/bv_cpu_triage_sparse500.prof
go tool pprof -top -alloc_space -nodecount=25 /tmp/bv_mem_triage_sparse500.prof
```
Observed bench summary (this run):
- `BenchmarkRobotTriage_Sparse500`: ~21.59ms/op, ~27.7MB/op, ~89.5k allocs/op

### 3B) Hotspots (top 3–5) — this is the “permission slip” to optimize
Alloc-space (from pprof):
1) `pkg/analysis.singleSourceBetweenness`: **~70%** of alloc_space
2) Gonum iterator allocations inside neighbor iteration: `gonum.org/v1/gonum/graph/iterator.*` (several % each)
3) Secondary allocs: ranking helpers (`computeFloatRanks`), some analyzer/graph building

CPU (from pprof, interpreted causally):
1) `runtime.gcDrain` / `runtime.scanobject` / friends: **GC dominates wall time**
2) `runtime.mapassign_fast64` + `internal/runtime/maps.(*table).grow/rehash`: **map traffic dominates CPU**
3) `analysis.ApproxBetweenness.func2` + `analysis.singleSourceBetweenness`: the algorithmic source of the above GC/map load

Conclusion:
> **Approximate betweenness (`ApproxBetweenness` → `singleSourceBetweenness`) is the dominant, needle-moving hotspot** (time via GC/map churn; space via alloc_space).

### 3D) Root Cause (code-level, why this hotspot is so expensive)
In `pkg/analysis/betweenness_approx.go`, each pivot run of `singleSourceBetweenness` currently allocates and fills multiple per-node maps:

```go
sigma := make(map[int64]float64)  // shortest-path counts
dist  := make(map[int64]int)      // BFS distances
delta := make(map[int64]float64)  // dependencies
pred  := make(map[int64][]int64)  // predecessor lists (dynamic slices)
```

And on top of that, it also allocates repeatedly during traversal:
- `nodes := graph.NodesOf(g.Nodes())` per call (materializes node slice)
- `neighbors` slice per visited node (append + sort) to get deterministic traversal order
- gonum iterator allocations from `g.From(v)` calls
- in `ApproxBetweenness`, each goroutine also allocates `localBC := make(map[int64]float64)` before merging into `partialBC`

This combination creates extreme allocation volume → **GC dominates CPU** and tail latency (p95+) gets worse under load.

### 3C) I/O profile sanity check
Exact command:
```bash
strace -c -f env BV_ROBOT=1 BV_NO_BROWSER=1 BV_TEST_MODE=1 /tmp/bv_round1 --robot-triage >/dev/null
```
Observed: syscalls dominated by `futex`/`nanosleep`; only ~30 `read` calls; effectively **not I/O-bound**.

---

## 4) Equivalence Oracle (C) — updated for epsilon (user request)

### 4A) Input definition
For “same inputs”, we mean:
- same parsed issue set (same JSONL contents after parsing/validation)
- same analysis configuration (dynamic `ConfigForSize` given node/edge count, unless forced)
- same seed for approximate algorithms (approx betweenness uses a seed)
- same “now” when time-based fields are included (triage staleness, etc.)

### 4B) Output definition
We treat outputs in two classes:

**(1) Structural outputs must be exact**
- IDs present and counts
- membership of sets (actionable set, blockers, quick wins, etc.)
- cycle presence and cycle members (when cycles computed)
- JSON schema and field names
- deterministic tie-breaking rules (ID) when not governed by near-ties

**(2) Float outputs are equal within epsilon**
- centralities and composite scores can differ slightly (parallel reduction, iteration order)
- we accept differences within epsilon (abs and/or relative)

### 4C) Practical epsilon policy
Use both absolute and relative tolerance:
- `absEps`: correct near zero
- `relEps`: correct at large magnitudes

Suggested defaults (align with existing golden tests + new epsilon test):
- PageRank: abs `1e-5`
- Eigenvector/HITS/CriticalPath: abs `1e-6`
- Betweenness:
  - exact: abs `1e-6`
  - approximate: abs `1e-6` + rel `1e-12` (or compare top-k rank stability)

### 4D) Ordering under epsilon (lists sorted by score)
If two scores differ by less than epsilon, ordering swaps are treated as equivalent **as long as**:
- the set of items is identical, and
- swapped items were within epsilon of each other.

This preserves correctness while preventing flaky tests when floats drift in low bits.

### 4E) Existing guardrails in this repo (use, don’t reinvent)
- Golden numeric expectations: `pkg/analysis/golden_test.go`
- Metamorphic/invariance: `pkg/analysis/invariance_test.go`
- Loader fuzzing: `pkg/loader/fuzz_test.go`
- Robot contract + perf checks: `tests/e2e/*`
- Epsilon stability for approximate betweenness: `pkg/analysis/betweenness_approx_test.go` (`TestApproxBetweenness_DeterministicWithinEpsilon`)

---

## 5) Opportunity Matrix (E) — ranked candidates

Scoring: **(Impact × Confidence) / Effort**, where Impact is primarily p95+/throughput and peak RSS on large graphs.

| # | Candidate | Impact | Confidence | Effort | Score | Why it matters |
|---:|---|---:|---:|---:|---:|---|
| 1A | **Buffer pooling for Brandes inner loop** (reuse maps + BFS slices) | 0.70 | 0.95 | 0.35 | **1.90** | Minimal diff that attacks ~70% alloc_space root cause |
| 1B | **Array-based Brandes + cached adjacency** (dense indices, no maps) | 0.90 | 0.85 | 0.60 | 1.28 | Bigger win; also eliminates gonum iterator churn and neighbor sorting |
| 2 | **Avoid building a separate undirected graph** for k-core/articulation | 0.25 | 0.85 | 0.35 | 0.61 | On 5k nodes, k-core is ~263ms and allocates significantly |
| 3 | **Slack computation reuse** (reuse topo; avoid per-node prereq slice builds) | 0.15 | 0.90 | 0.30 | 0.45 | On 5k nodes slack ~96ms; easy isomorphic savings |
| 4 | **Replace k-core algorithm** with linear-time peeling | 0.25 | 0.85 | 0.60 | 0.35 | Protects against worst-case O(maxDeg·V) behavior |
| 5 | **Cross-process cache for robot mode** keyed by (`data_hash`,`config_hash`) | 0.15 | 0.60 | 0.70 | 0.13 | Useful for repeated invocations; adds invalidation/storage policy risk |
| 6 | **Data-hash micro-optimization** (same hash, fewer allocs) | 0.10 | 0.60 | 0.30 | 0.20 | Might matter at 5k+ once analysis allocs drop |
| R | **Semiring / matrix-form recast (research track)** | 0.10 | 0.30 | 0.90 | 0.03 | Interesting for 50k+ nodes; not justified for Round 1 |

Round 1 focus: 1A → re-profile → 1B only if still hot → then consider 2/3 if they rise into the top 3–5 hotspots.

---

## 6) Proposed Changes (F) — one lever per change + proof sketches (D)

### Change 1A (Round 1 primary): Buffer pooling for Brandes’ inner loop (minimal diff)
Target:
- `pkg/analysis/betweenness_approx.go` (`singleSourceBetweenness`)

Why this is the best first lever (hybrid rationale):
- It attacks the proven bottleneck (≈70% alloc_space) with minimal surface area.
- It is straightforward to prove isomorphic: we perform the same assignments as before, just into reused buffers.

Plan sketch (pattern from OPUS plan; keep signature stable):
```go
type brandesBuffers struct {
    sigma     map[int64]float64
    dist      map[int64]int
    delta     map[int64]float64
    pred      map[int64][]int64
    queue     []int64
    stack     []int64
    neighbors []int64
}

var brandesPool = sync.Pool{ /* New: allocate once, reuse */ }
```
Key reset logic:
- For every nodeID in `nodes`, explicitly set `sigma=0`, `dist=-1`, `delta=0`, and `pred[nodeID]=pred[nodeID][:0]`.
- Use pooled `neighbors` slice inside the BFS loop (`neighbors = neighbors[:0]`).
- Optional safety: if buffers grew far beyond current graph size, drop/recreate to avoid unbounded memory retention.

What this does NOT address (so we know what to profile next):
- `nodes := graph.NodesOf(g.Nodes())` still allocates per pivot
- `localBC := make(map[int64]float64)` in `ApproxBetweenness` still allocates per pivot
- gonum iterator allocations from `g.From(v)` still occur (though pooled neighbor slice reduces some pressure)

Isomorphism proof sketch (more explicit, OPUS-style):
1) **Initialization equivalence**: every value read by the algorithm is assigned before use for all nodes, so prior buffer contents are irrelevant.
2) **Graph-change safety**: stale entries for node IDs not present in `nodes` are never referenced (only iterated/visited nodes matter).
3) **Predecessor slice equivalence**: `make([]T,0)` and `s[:0]` are identical for algorithm semantics (`len==0`), differing only in allocation behavior.
4) **Deterministic traversal**: neighbor ordering remains sorted by node ID (same as current semantics).
5) **Concurrency safety**: each pivot goroutine gets its own buffers from the pool; no shared mutation across goroutines.
6) **Pool eviction safety**: `sync.Pool` may drop buffers under GC; worst case behavior degenerates to current allocation behavior, never to incorrect results.

Caveats:
- `sync.Pool` can be evicted under memory pressure; reductions may vary with workload and GC.
- If we “clear” maps incorrectly (forget to overwrite a key that will be read), correctness breaks; the reset must be exhaustive over node IDs.

Rollback guidance:
- Localized to one file; revert `brandesPool` usage if it misbehaves.

Guardrails:
- Existing golden + invariance tests.
- Add/keep allocation-focused benches and check that `BenchmarkApproxBetweenness_500nodes_Sample100` allocs/op drops sharply.

Expected gains (bench-driven targets, conservative):
- Baseline (mean, §2H): `BenchmarkApproxBetweenness_500nodes_Sample100` allocs/op ≈ **199,551**, B/op ≈ **29.6MB**.
- Change 1A target (pooling only):
  - allocs/op: **floor** < 80k (≥60% reduction), **stretch** < 50k (≥75% reduction)
  - B/op: **floor** < ~15MB, **stretch** < ~10MB
- If we don’t hit these, it’s a signal that remaining allocs are dominated by `graph.NodesOf(g.Nodes())`, `localBC` allocations, or iterator churn → proceed to Change 1B.

### Change 1B (only if still hot): Array-based Brandes + cached adjacency (bigger but cleaner)
Target:
- `pkg/analysis/betweenness_approx.go` (`ApproxBetweenness`, `singleSourceBetweenness`)

Goal:
- Eliminate most remaining allocations by moving to dense arrays and precomputed adjacency lists.

Strategy:
1) Build a stable `ids []int64` (sorted node IDs) and `index map[int64]int` once per ApproxBetweenness call.
2) Build `adj [][]int` once, where `adj[i]` is sorted neighbor indices (this replaces per-node neighbor slice materialization + sort).
3) Rewrite Brandes inner loop to operate on indices (`[]float64`/`[]int`), reusing slices per worker.
4) Keep pivot sampling deterministic (same seed) and accept epsilon-level float drift due to reduction order (approved by user + guarded by epsilon tests).

Isomorphism proof sketch:
- Same algorithm, same pivots, same adjacency ordering (sorted by ID), same arithmetic; only representation changes.
- Any differences are limited to floating rounding; within epsilon acceptance.

### Change 2 (only if post-#1 profiles justify it): Remove undirected-graph construction for k-core/articulation
Target:
- `pkg/analysis/graph.go` (`computeCoreAndArticulation` + `computeKCore` + `findArticulationPoints`)

Minimal diff idea:
- Build an undirected adjacency view directly from the directed graph edges (treat every directed edge as undirected).
- Run k-core and articulation on that adjacency without allocating `simple.UndirectedGraph` + edges.

Isomorphism proof sketch:
- The current implementation already treats the directed edges as undirected by inserting them into an undirected graph; direct adjacency construction is equivalent.
- Determinism preserved by sorting neighbors (if iteration order matters in DFS).

### Change 3 (deferred unless still hot): Replace k-core peeling with linear-time algorithm
Target:
- `computeKCore` currently loops `k=1..maxDeg` and rescans nodes; worst-case can be large.

Proposed algorithm:
- Batagelj–Zaveršnik linear-time k-core decomposition using bin sorting by degree.

Isomorphism proof sketch:
- Computes the same core number definition; only the algorithmic route changes.

---

## 7) “What we considered” (algorithmic toolbox mapping)

This section explicitly maps the user’s optimization checklist to this codebase.

### Clearly applicable (supported by profiles)
- **Index-based lookup vs linear scans**: dense-index arrays for graph algorithms.
- **Zero-copy / buffer reuse**: per-worker scratch buffers for Brandes.
- **Bounded queues + backpressure**: worker pool rather than per-pivot goroutines (even though sem bounds concurrency, pool enables buffer reuse cleanly).
- **Topological sort and DAG-awareness**: reuse Phase 1 topo order in Phase 2 helpers (slack).
- **Graph traversal with early termination**: cycle detection already uses SCC precheck; keep it.
- **Heaps/priority queues**: not directly needed in current hotspots.
- **Memory layout optimization (SoA vs AoS)**: array-based sigma/dist/delta improves cache locality.
- **Short-circuiting**: avoid work when metrics are skipped by config or timeouts.

### Possibly useful later (post betweenness fix)
- **Serialization format costs**: JSON encoding/decoding costs may become significant on 20k issues; revisit once analysis allocations are fixed.
- **Cross-process caching**: can help repeated `bv --robot-*` on large graphs.

### Not currently justified by evidence
- DP / convex optimization / min-cost flow: no hotspots resemble scheduling/allocation optimization problems.
- Tries, segment trees, spatial indices: not the problem shape here.
- Lock-free data structures: contention is not the limiter; allocations/GC are.

---

## 8) Regression Guardrails (G)

### 8A) Benchmarks to watch (hard performance contracts)
- `pkg/analysis/BenchmarkRobotTriage_Sparse500`
- `pkg/analysis/BenchmarkFullAnalysis_Sparse1000`

Suggested CI guard (concrete options):
- **Benchstat gate**: run `./scripts/benchmark.sh compare` and fail if key benches regress beyond a threshold.
- **Allocation threshold gate** (fast, dependency-free): ensure `BenchmarkApproxBetweenness_500nodes_Sample100` allocs/op stays under a fixed cap after Change 1A.

Example allocation-threshold check:
```bash
go test -run '^$' -bench '^BenchmarkApproxBetweenness_500nodes_Sample100$' -benchmem ./pkg/analysis -count=1 | tee bench.txt
ALLOCS=$(awk '/Sample100/ {print $7}' bench.txt)
if [ "${ALLOCS:-999999999}" -gt 80000 ]; then
  echo "Allocation regression: Sample100 allocs/op=$ALLOCS (cap=80000)"
  exit 1
fi
```

### 8B) “Golden numeric” tests already enforce stability
- `pkg/analysis/golden_test.go` (metric tolerances)
- `pkg/analysis/betweenness_approx_test.go` (epsilon stability)

---

## 9) Success Criteria (what must move)

For Change 1A/1B (betweenness optimization), “success” is defined as all of:
- Meaningful reduction in alloc_space attributed to betweenness (target: from ~70% to “not dominant”).
- CPU profile shows GC time collapsing (runtime.gcDrain no longer dominates).
- For large graphs (5k issues):
  - peak RSS falls drastically from ~429MB
  - p95 latency and/or throughput improves (re-measure p50/p95/p99 as in §2E)
- All tests remain green under `BV_NO_BROWSER=1 BV_TEST_MODE=1`.
