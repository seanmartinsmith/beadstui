# Test Suite Perf + Coverage Audit (bt-qp1j)

<!-- Related: bt-qp1j; follow-ups: bt-gmeck bt-vvel8 bt-ma6c4 bt-pgu0h bt-7qx80 bt-xv7wf -->

**Date**: 2026-07-12
**Scope**: Runtime performance and coverage of `go test ./...` across all 37 buildable packages.
**Machine**: Apple Silicon (arm64), Go 1.26.0, macOS.
**Method**: One full `go test ./... -count=1 -json -cover` run (parsed for per-package and per-test elapsed + coverage), a targeted `-cpuprofile` on `pkg/ui`, and source reads of the long-pole test files. Raw data under `_tmp/audit/` (gitignored).

> Sibling audit: `docs/audits/domain/2026-04-03-test-suite-audit.md` (KEEP/UPDATE
> correctness classification). This one is perf + coverage only.

## Summary

- **Total wall time: 159s (~2.7 min)** on this machine, not the ~8.5 min the bead predicted. The 8.5 min figure was from a slower/Windows host or an older run. Sum of per-package elapsed is 367s; the 2.3x gap is package-level parallelism (`go test -p GOMAXPROCS`).
- **The suite is RED, deterministically, on macOS** - two failures unrelated to the drift trio already fixed in bt-5e99. See section 5. This is arguably the headline finding: the perf audit found the suite does not pass green on the maintainer's own platform.
- **The critical path is `tests/e2e` at 130.9s** - ~82% of wall time. It is a shared-binary black-box harness that spawns ~300 `bt` subprocesses strictly serially.
- **The long poles are wait-bound, not compute-bound.** Every top offender's runtime is real sleeps, real git, real subprocess spawns, or a network timeout - not CPU. A `pkg/ui` CPU profile measured only 18.5% on-CPU. There is no accidental O(n^2) hot loop to chase.
- **`pkg/hooks` is no longer a long pole** (1.6s now, vs the bead's stale 11s figure) - its timeout tests were tightened to sub-second since the bead was filed.
- **Coverage is broadly healthy** (all 37 packages have tests; most 70-100%). The genuine gaps cluster on the Dolt data path (`internal/doltctl` 30.8%, `internal/datasource` 33.4%), which is exactly what bt-kvk0 exists to close.

### Per-package runtime + coverage (full run)

| Package | Elapsed | Cov% | Result |
|---|--:|--:|---|
| tests/e2e | 130.89s | n/a* | PASS |
| cmd/bt | 105.00s | 17.1* | PASS |
| pkg/ui | 40.06s | 71.4 | PASS |
| pkg/export | 23.06s | 69.0 | PASS |
| pkg/loader | 19.53s | 88.3 | **FAIL** |
| pkg/tail | 4.00s | 81.0 | PASS |
| pkg/watcher | 3.36s | 87.7 | PASS |
| pkg/correlation | 2.92s | 75.1 | **FAIL** |
| pkg/analysis | 2.72s | 88.9 | PASS |
| pkg/cass | 1.95s | 83.5 | PASS |
| internal/settings | 1.88s | 72.2 | PASS |
| pkg/view | 1.86s | 91.4 | PASS |
| pkg/agents | 1.81s | 81.5 | PASS |
| pkg/bql | 1.81s | 61.9 | PASS |
| pkg/baseline | 1.79s | 90.5 | PASS |
| pkg/testutil | 1.69s | 49.9 | PASS |
| pkg/testutil/proptest | 1.69s | 75.6 | PASS |
| internal/doltctl | 1.67s | 30.8 | PASS |
| pkg/hooks | 1.61s | 82.6 | PASS |
| pkg/recipe | 1.56s | 93.1 | PASS |
| pkg/workspace | 1.48s | 88.8 | PASS |
| pkg/search | 1.47s | 82.3 | PASS |
| pkg/debug | 1.42s | 76.6 | PASS |
| pkg/drift | 1.33s | 70.2 | PASS |
| pkg/util/topk | 1.31s | 100.0 | PASS |
| pkg/version | 1.30s | 65.2 | PASS |
| pkg/model | 1.17s | 71.9 | PASS |
| pkg/metrics | 1.10s | 87.1 | PASS |
| pkg/instance | 1.06s | 76.6 | PASS |
| internal/diagnostics | 0.87s | 84.0 | PASS |
| pkg/updater | 0.77s | 59.5 | PASS |
| pkg/projects | 0.74s | 66.7 | PASS |
| internal/datasource | 0.73s | 33.4 | PASS |
| internal/bdroute | 0.47s | 90.4 | PASS |
| pkg/ui/keys | 0.46s | 100.0 | PASS |
| internal/bdexec | 0.36s | 95.7 | PASS |
| pkg/ui/events | 0.24s | 89.7 | PASS |

\* `cmd/bt` and `tests/e2e` are black-box harnesses: they exercise `bt` via a
subprocess, and Go coverage instrumentation does not cross the process boundary.
`cmd/bt`'s 17.1% is a measurement artifact, not a real coverage gap - most of its
logic is tested through the e2e/robot subprocess assertions. Treat those two
numbers as "not meaningful," not "undertested."

## 1. Runtime profile per package (long poles)

Peak resident set for the whole parallel run was 732 MB (`/usr/bin/time -l`) - no
leak signal. The four packages that dominate:

### tests/e2e - 130.9s (the critical path)

The harness builds the `bt` binary **once** in `TestMain`
(`tests/e2e/common_test.go:195`, cleaned up at exit) - that part is correct and
already amortized. The cost is downstream: ~300 `bt` subprocess invocations (304
`exec.Command` sites, 265 `CombinedOutput`) run **strictly serially** - not one
test in the package calls `t.Parallel`. At ~0.4s per spawn (Go runtime init + data
load per invocation), the serial chain is the whole 130s. e2e runs against JSONL
fixtures, **not** Dolt (per bt-kvk0), so this is pure process-per-assertion cost,
not Dolt spin-up.

- **Where the time goes**: serial subprocess spawns.
- **Class of fix**: bounded `t.Parallel` (tests already use isolated temp dirs +
  a shared read-only binary, so they are structurally parallel-safe), and/or fewer
  redundant invocations. Bound parallelism to avoid the CPU/thermal spike noted in
  bt-qp1j's 2026-06-19 comment. Follow-up: **bt-xv7wf**.

### cmd/bt - 105.0s

Same shape as e2e: 42 `exec.Command`, 17 `CombinedOutput`, serial. Overlaps e2e on
the wall clock (both run concurrently as separate packages), so it is not
separately on the critical path, but it is the second-heaviest package. Covered by
the same class of fix as bt-xv7wf.

### pkg/ui - 40.1s

~17s of the 40 is three `BackgroundWorker` tests whose bodies are trivial:

| Test | Elapsed |
|---|--:|
| `TestBackgroundWorker_CheckHealth_TriggersRecoveryOnMissedHeartbeat` | 7.01s |
| `TestBackgroundWorker_StartStop` | 5.00s |
| `TestBackgroundWorker_AttemptRecovery_GivesUpAfterMaxRecoveries` | 5.00s |

The cost is inside the worker lifecycle, not the test. `Stop()` waits on the loop
goroutine's `done` channel with a hardcoded **5s** fallback
(`pkg/ui/background_worker.go:717-722`), and each recovery attempt waits with a
hardcoded **2s** fallback (`:895-901`). In tests the loop is parked on `time.Hour`
tickers, so it does not observe cancellation promptly, and Stop/recovery pay the
full fallback each time. The rest of pkg/ui's time is ~16 x 200ms debounce sleeps
plus a handful of 2-5s deadline-poll loops.

- **Where the time goes**: hardcoded shutdown/recovery drain fallbacks + debounce sleeps.
- **Class of fix**: make the loop wake on `ctx.Done()` at every park (a real
  production shutdown-latency win, not just a test speedup), or make the drain
  durations injectable via `WorkerConfig`. Follow-up: **bt-pgu0h**.

### pkg/export - 23.1s

Two tests are 17s of it:

| Test | Elapsed |
|---|--:|
| `TestVerifyCloudflareDeployment_Timeout` | 13.03s |
| `TestInitAndPush_MissingBundlePath` | 3.84s |

`TestVerifyCloudflareDeployment_Timeout` calls `VerifyCloudflareDeployment("http://192.0.2.1/", 100, 1*time.Second)`
with a comment claiming "quick timeout" but takes 13s: the 1s timeout is not
reaching the dialer, so a non-routable host (TEST-NET-1) hangs through the retry
loop's 3s sleeps (`cloudflare.go:435,445`). This is both a test-perf bug and a
latent production bug (a non-routable host would hang in real usage too).

- **Where the time goes**: an unhonored timeout against a non-routable IP.
- **Class of fix**: thread the timeout into the `net.Dialer`/`http.Client`.
  Follow-up: **bt-ma6c4**.

### pkg/loader - 19.5s

All of it is 11 `GitLoader` tests at ~1.6s each (`git_test.go`). Each calls
`setupTestGitRepo(t)`, which shells out to real git (`git init` + 3x add/commit) to
build a throwaway 3-commit repo. The tests are **read-only** (LoadAt /
ResolveRevision / ListRevisions against HEAD and HEAD~1) yet each rebuilds the
fixture from scratch, serially, with no `t.Parallel`.

- **Where the time goes**: 11x redundant real-git fixture builds, serial.
- **Class of fix**: build the read-only fixture **once** (TestMain / `sync.Once`)
  and `t.Parallel` the read-only tests - they use `cmd.Dir`, not `os.Chdir`, so
  nothing blocks parallelization. ~19.5s -> ~2s. Follow-up: **bt-vvel8**.

### Note: pkg/hooks is no longer a long pole

The bead lists `pkg/hooks` at 11.0s. It now runs in **1.6s**. Its slow tests
(`sleep`/timeout hooks that spawn real `sh -c` subprocesses) were tightened to
sub-second timeouts (10ms/100ms/500ms) since the bead was filed - e.g.
`TestExecutorHookTimeout` is 0.10s. No action needed; the bead's figure is stale.

## 2. Parallelism audit

Package-level parallelism (`go test -p`, default GOMAXPROCS) is working and is what
buys the 2.3x wall-vs-sum ratio. **Intra-package parallelism is essentially absent**:

| Package | Files with `t.Parallel` | `t.Setenv` | `os.Chdir` | `t.Chdir` |
|---|--:|--:|--:|--:|
| pkg/ui | 2 / 92 | 21 | 17 | 1 |
| pkg/export | 1 / 26 | 43 | 2 | 0 |
| pkg/loader | 0 / 11 | 0 | 4 | 0 |
| pkg/hooks | 0 / 8 | 0 | 0 | 0 |
| tests/e2e | 0 / 44 | 1 | 0 | 0 |
| cmd/bt | 0 / 18 | 12 | 0 | 0 |

**Why packages opt out / what blocks parallelizing them:**

- **pkg/export (43 `t.Setenv`)**: `t.Setenv` is mutually exclusive with
  `t.Parallel` - the runtime panics if a test calls both. These tests cannot be
  parallelized without first removing their reliance on process-global env.
- **pkg/ui (21 `t.Setenv`, 17 `os.Chdir`, 1 `t.Chdir`)**: same `t.Setenv` barrier,
  plus `os.Chdir` is process-global - parallel tests that chdir race each other's
  working directory. `t.Chdir` also forbids `t.Parallel`.
- **pkg/loader (0 `t.Setenv`, 0 `t.Chdir`)**: the git tests use `cmd.Dir`, not
  process chdir, so they are **safely parallelizable today** - the biggest
  low-risk win (bt-vvel8).
- **tests/e2e / cmd/bt**: serial by convention, not by contamination (shared
  read-only binary, isolated temp dirs). Structurally parallel-safe; the blocker is
  CPU/thermal budget, so parallelism should be bounded (bt-xv7wf).

**Contamination summary**: 64 `t.Setenv` + 19 `os.Chdir`/`t.Chdir` across
ui/export/cmd is the process-global-state debt that keeps those packages serial.

## 3. CPU / memory profile

Profiling the top offenders with `-cpuprofile` is the wrong lens, and confirming
that is itself the finding. `pkg/ui` (the largest pure-Go in-process offender):

```
Duration: 27.31s, Total samples = 5050ms (18.49%)
```

Only **18.5% of wall time was on-CPU** - 81% was off-CPU, parked in sleeps,
`time.After`, channel waits, and syscalls. The top on-CPU frames are all runtime
plumbing (`syscall.rawsyscalln`, `pthread_cond_signal/wait`, `madvise`, `kevent`,
GC); the only application frame in the top set is `ansi.stringWidth` at 4.16%
cumulative - expected for a TUI, not a hot loop.

**Conclusion**: there is no accidental O(n^2) hot loop driving the runtime. The
top-4 offenders are wall-clock-bound - real sleeps (pkg/ui), real git (pkg/loader),
real subprocess spawns (e2e/cmd), and a network timeout (pkg/export). The fixes are
about eliminating waits and redundant work, not optimizing compute. Peak RSS
(732 MB for the whole parallel suite) shows no memory pressure or leak.

## 4. Coverage gaps

All 37 buildable packages have tests - there are no zero-coverage packages. Most
sit at 70-100%. The sub-50% and risky-but-mid numbers, triaged "trivial vs risky":

| Package | Cov% | Verdict |
|---|--:|---|
| cmd/bt | 17.1 | **Not a real gap** - black-box artifact (tested via subprocess). |
| internal/doltctl | 30.8 | **Risky** - Dolt server lifecycle (start/stop, PID ownership). Hard to cover without a real Dolt; ties to **bt-kvk0**. |
| internal/datasource | 33.4 | **Risky** - source discovery + the Dolt data path. Also bt-kvk0. |
| pkg/testutil | 49.9 | Fine - test infrastructure itself; low blast radius. |
| pkg/updater | 59.5 | Mostly fine - network/self-update code is env-gated and hard to exercise. |
| pkg/bql | 61.9 | **Worth backfill** - the query parser/executor builds SQL; per labels.md this area is security-sensitive (injection surface). |
| pkg/version, pkg/projects | 65-67 | Fine - thin, low-risk. |

**Risky areas per AGENTS.md, qualitative pass:**

- **Graph traversal nil-safety** (`pkg/analysis`): 88.9% - well covered.
- **Robot JSON contract** (`cmd/bt` + e2e): covered by black-box `robot_contract_test.go`
  and `robot_stderr_cleanliness_test.go` - solid despite the misleading cmd/bt %.
- **Concurrency / worker resubscribe chain** (`pkg/ui` background worker): the
  worker has substantial tests (this audit's slow trio among them), but see section
  5 - some of that coverage is bought with multi-second real-time waits rather than
  fake clocks.
- **Division guards**: not separately measurable from line coverage; no gap flagged.

The one structural coverage gap worth a bead already has one: **bt-kvk0** (real-Dolt
e2e infrastructure). `internal/doltctl` + `internal/datasource` low numbers are the
quantified symptom of that gap; this audit does not file a duplicate.

## 5. Flaky / sensitive tests

**Deterministic failures on this run (suite is RED on macOS):** both reproduce in
isolation - they are not load-flakes.

1. **`pkg/loader` `TestGetBeadsDir_EmptyRepoPath_UsesCwd`** - fails on any macOS
   host. `t.TempDir()` returns `/var/...` while the production path resolves the
   `/var -> /private/var` symlink; the two forms compare unequal. Filesystem-layout
   sensitive. Follow-up: **bt-gmeck** (P1 - it means CI/local is red on macOS).

2. **`pkg/correlation` `TestPrefetchCoCommittedFiles_ByteIdenticalToPerEvent`** -
   asserts the batched co-commit prefetch matches the per-event path, run against
   the **live repo's real git history** (hardcoded real SHAs). For merge commits
   (e.g. `d9b69bb`, a merge) the batched path returns `nil` while per-event returns
   real changes. Two problems: a probable real bug (batched prefetch drops
   merge-commit file changes - `git diff-tree` on a merge needs `-m`/`--first-parent`),
   and a fragile test coupled to whatever history the working copy has. Follow-up:
   **bt-7qx80**.

**Known, already filed:**

3. **`tests/e2e` `TestExportIncremental_MultipleReexports`** - `CombinedOutput`
   pipe-buffer deadlock under full-suite load on Windows (**bt-4v959**, child of
   this bead). The e2e package has 265 `CombinedOutput` sites sharing this shape.

**Wall-time sensitive (not failing, but fragile):**

4. `pkg/ui` `BackgroundWorker` deadline-poll tests (2s/3s/5s deadlines) and the
   5s/2s shutdown-drain fallbacks (section 1 / bt-pgu0h) - correctness depends on
   real clock progress; slow CI could push them toward timeouts.
5. `pkg/export` `TestVerifyCloudflareDeployment_Timeout` - depends on network
   dial-timeout behavior against a non-routable IP (bt-ma6c4).

**Process-env / filesystem-ordering sensitive:** the 64 `t.Setenv` + 19
`os.Chdir`/`t.Chdir` sites (section 2) are the standing pool of ordering-sensitive
state. None are failing today, but they are why those packages can't parallelize
and are the most likely source of future order-dependent flakes.

## 6. Recommendations (tiered)

**Tier 1 - green + cheap wins (do first, ideally same session as this audit):**

- **bt-gmeck** (P1): fix the macOS `/var` symlink comparison in the loader test.
  Gets the suite green on macOS. Trivial (`filepath.EvalSymlinks` both sides).
- **bt-vvel8** (P2): share one read-only git fixture + `t.Parallel` the loader git
  tests. ~19.5s -> ~2s, zero coverage loss, no contamination barrier.
- **bt-ma6c4** (P2): plumb the timeout into the cloudflare dialer. 13s -> <1s, and
  fixes a latent production hang.

**Tier 2 - rewrite the top offenders:**

- **bt-pgu0h** (P2): wake the background-worker loop on cancel (or inject the drain
  timeouts). ~17s off pkg/ui, and a real production shutdown-latency improvement.
- **bt-xv7wf** (P2): bounded-parallel or invocation-reduced e2e/cmd harness. This is
  the 130s critical path; biggest single lever on total wall time, but the largest
  change and needs thermal-aware bounding.

**Tier 3 - correctness + coverage backfill:**

- **bt-7qx80** (P2): resolve the batched-vs-per-event merge-commit divergence
  (likely a real bug) and de-couple the test from live repo history.
- Dolt-path coverage (`internal/doltctl`, `internal/datasource`): tracked by
  **bt-kvk0**; no new bead.
- `pkg/bql` SQL-builder coverage (security-sensitive): noted; file a bead if/when a
  concrete gap is identified.

Adjacent, complementary beads (not filed here): **bt-tjq0** (per-package progress
visibility - would make the 159s run legible while it happens), **bt-5dvl**
(the 2026-04-03 correctness backlog).
