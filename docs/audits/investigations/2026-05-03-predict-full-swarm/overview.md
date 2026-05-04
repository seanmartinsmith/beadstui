# Predict Analysis — bt full-swarm

**Date:** 2026-05-03 23:21 UTC
**Scope:** `cmd/**, pkg/**, internal/**` (entire codebase, 248 non-test files / 103,413 LOC)
**Personas:** 5 (Architecture Reviewer, Security Analyst, Performance Engineer, Reliability Engineer, Devil's Advocate)
**Debate Rounds:** 2 of 2 completed
**Commit Hash:** `5b3767830036f23ad425f1223edd62fb1982cf3b` (HEAD at scan time)
**Anti-Herd Status:** ✅ PASSED (flip_rate=0.105, high entropy, 2-round convergence)
**Predict Score:** 578

## Summary

- **Total Findings:** 38 (within 40 budget)
  - Confirmed (≥3/5 personas): 36
  - Probable (2/5): 0
  - Minority (1/5, preserved): 1
  - Discarded: 0
- **Severity Breakdown:** Critical: 2 | High: 11 | Medium: 24 | Low: 1
- **Cross-finding clusters:** 9 (SQLite legacy, Sprint stale, Phase 2 timeout, Self-update, bd dolt regex, Stale-lock Windows, Doc-drift, Live-reload, Git shell-out)

## Top 10 Findings

1. **[CRITICAL] [RE-4] Updater proceeds without verification when checksums.txt absent** — pkg/updater/updater.go:623-645 — 5/5 consensus
2. **[CRITICAL] [SA-1] Arbitrary code execution via .bt/hooks.yaml in cloned repos** — pkg/hooks/executor.go:120-121 + cmd/bt/cobra_export.go:48-51 — 4/5 consensus
3. **[HIGH] [AR-2] Data layer carries dead SourceTypeSQLite branch despite ADR-003** — internal/datasource/load.go:172 — 5/5 consensus
4. **[HIGH] [DA-3] Memory says SQLite export-only; code wires it into runtime** — AGENTS.md:146 vs internal/datasource/sqlite.go — 5/5 consensus
5. **[HIGH] [DA-2] pkg/loader/sprint.go is a stale-Dolt-assumption violation** — pkg/loader/sprint.go:28 — 5/5 consensus
6. **[HIGH] [DA-1] AGENTS.md "Key Directories" lists internal/dolt/ and internal/models/ that don't exist** — AGENTS.md:88-89 — 5/5 consensus
7. **[HIGH] [PE-1] N+1 query storm: 1+3n queries per single-DB issue load** — internal/datasource/dolt.go:273-279 — 4/5 consensus
8. **[HIGH] [SA-2] Git argument injection via unvalidated SHAs (no --end-of-options)** — pkg/correlation/incremental.go:200 + temporal.go:143 + stream.go:415 — 4/5 consensus
9. **[HIGH] [RE-2] Phase 2 timeout fallback (uniform PageRank) silently persisted to disk cache** — pkg/analysis/graph.go:1649-1662,1871,1982-1984 — 4/5 consensus
10. **[HIGH] [RE-1] os.Exit short-circuits Dolt server cleanup, orphaning bd-spawned processes** — cmd/bt/root.go:286-491 — 4/5 consensus

## Recommended action plan (highest leverage first)

### P0 — Ship in next session
- **bt-05zt Phase 1** (mechanical SQLite removal) — addresses AR-2, DA-3, AR-8 simultaneously. ADR-003 already accepted.
- **AGENTS.md correctness sweep** — DA-1, DA-3, DA-4, DA-6 are doc-drift findings closeable in one editor pass. Most agent-toxic findings in the swarm.
- **Sprint feature decision (bt-z5jj)** — resolves DA-2, AR-6 cluster. Either rebuild against Dolt or retire entirely; current limbo is worst-of-both.
- **Hooks security hardening (SA-1)** — invert `--no-hooks` default OR add TOFU trust prompt + `bt hooks list/trust` subcommands. CRITICAL.
- **Updater verification gate (RE-4)** — refuse update when checksums.txt missing; require `--allow-unverified`. CRITICAL.

### P1 — Within 1-2 sessions
- **Robot output schema regression net (DA-7 / bt-ah53 P1)** — golden snapshots for 26+ uncovered subcommands.
- **N+1 batched loader for DoltReader (PE-1)** — mirror GlobalDoltReader pattern. 1-day refactor with measurable wins.
- **Git arg injection helper (SA-2)** — `validateSHA` + `--end-of-options` sweep; 13 callsites identified in prior STRIDE audit not yet addressed.
- **Dolt server cleanup on os.Exit (RE-1)** — replace os.Exit pattern with cleanup-aware return; signal handler registration.
- **GlobalDoltReader.LoadIssues per-DB resilience (RE-3)** — mirror LoadIssuesAsOf shape.

### P2 — Cluster-resolved
- **Phase 2 timeout machinery redesign** — single PR addresses PE-5 + RE-2 + RE-5 with `context.WithTimeout` + cancellation-aware algorithms.
- **Stale-lock Windows hardening** — single PR addresses SA-3 + RE-8 with `LockFileEx` + symlink defense.
- **bd dolt start coordination** — addresses SA-7 + RE-7; coordinate `bd dolt start --json` upstream.

### P3 — Backlog (preserved)
- **AR-4 (single-binary architecture)** — minority finding (1/5). May matter more if bt grows into headless agent-only deployment scenarios.
- **AR-7 (1,332 bv- sigils)** — LOW severity; reframe bt-t82t to include source-code sigils, not just user-facing copy.

## Composite metric breakdown

```
predict_score = findings_confirmed * 15      = 36 * 15 = 540
              + findings_probable * 8        =  0 * 8  =   0
              + minority_preserved * 3       =  1 * 3  =   3
              + (personas_active/total)*20   = (5/5)*20 = 20
              + (rounds_done/planned)*10     = (2/2)*10 = 10
              + anti_herd_passed * 5         =  1 * 5  =   5
              ==================================
                                     TOTAL  = 578
```

Higher = more thorough + more diverse. This run scored well on breadth (5 active personas), depth (full 2 rounds completed), and intellectual diversity (anti-herd passed, 1 minority preserved).

## Files in this report

- [Findings](./findings.md) — all 38 ranked by priority_score with full evidence + persona votes
- [Hypothesis Queue](./hypothesis-queue.md) — 35 testable hypotheses + 1 minority, with chain-handoff cluster groupings
- [Persona Debates](./persona-debates.md) — round 1 + round 2 cross-challenges, revisions, anti-herd metrics, DA mandate compliance
- [Iteration Log](./predict-results.tsv) — per-persona per-round data
- [Knowledge: Codebase Analysis](./codebase-analysis.md) — package map, routes, SQL surface, concurrency, hygiene
- [Knowledge: Dependency Map](./dependency-map.md) — external deps, import graph, call graph, data flows, supply-chain notes
- [Knowledge: Component Clusters](./component-clusters.md) — 10 clusters with risk areas + cross-cluster invariants
- [Handoff JSON](./handoff.json) — machine-readable schema for downstream chain tools

## Chain status

User selected **No chain** — report-only run. Hypothesis-queue.md is ready if you change your mind:
- `--chain debug` to test the high-priority hypotheses empirically
- `--chain security` for STRIDE/OWASP follow-up on H-02, H-07, H-13, H-18, H-19, H-20
- `--chain fix` to auto-queue the P0 cluster (SQLite removal, sprint decision, hooks hardening, updater gate)

## Notes for the next session

- The **doc-drift cluster** (DA-1, DA-3, DA-4, DA-6, AR-7) is the cheapest-to-fix highest-leverage cluster. AGENTS.md is the source of truth agents read first; making it accurate gates downstream agent productivity. Round 2's non-code hypothesis suggested adding a CI grep gate for AGENTS.md/MEMORY.md path/count claims — worth considering.
- The **Phase 2 timeout machinery** has 3 distinct findings (PE-5, RE-2, RE-5) on one piece of code. If touched, address all three together.
- The **`bt-ah53` (robot output schemas)** bead was bumped P2→P1 on 2026-04-28 per OOO synthesis but remains OPEN; this swarm corroborates the bump and recommends it move to P0 lane.
- **Cross-cutting**: 4 of the top-10 findings reference open beads (bt-05zt, bt-z5jj, bt-ah53, bt-72l8). Prioritize closing these before greenfield work.
- The **single SAFE minority preservation** is AR-4 (single-binary linkage). Worth a backlog bead in case bt grows into agent-only deployment scenarios.
