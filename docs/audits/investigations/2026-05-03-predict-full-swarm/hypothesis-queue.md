# Hypothesis Queue

Ranked list of confirmed/probable findings formatted as testable hypotheses. Each hypothesis is a falsifiable claim with file:line and the source persona's evidence trail. For chain-handoff use (debug, security, fix, ship, scenario).

| Rank | ID | Hypothesis | Confidence | Severity | Location | Source Persona |
|---|---|---|---|---|---|---|
| 1 | H-01 | `bt update` will install an unsigned, non-checksummed binary if the GitHub release is missing `checksums.txt` — silent skip, no error | HIGH | CRITICAL | pkg/updater/updater.go:623-645,657-660 | Reliability (5/5 confirmed) |
| 2 | H-02 | `bt export` in a cloned untrusted repo will execute commands from `.bt/hooks.yaml` under user's UID with no prompt | HIGH | CRITICAL | pkg/hooks/executor.go:120-121, cmd/bt/cobra_export.go:48-51 | Security (4/5 confirmed) |
| 3 | H-03 | The `SourceTypeSQLite` discovery branch picks up stale `.beads/beads.db` files at PrioritySQLite=100 (above JSONL=50) and serves data divergent from current Dolt state | HIGH | HIGH | internal/datasource/source.go:204; load.go:172 | Architecture + Devil's Advocate (5/5 confirmed) |
| 4 | H-04 | `bt robot sprint list` returns empty `[]model.Sprint{}` with no error/diagnostic on every Dolt-only beads install (i.e., every install) | HIGH | HIGH | pkg/loader/sprint.go:28; cmd/bt/robot_sprint.go:22 | Devil's Advocate + Architecture (5/5 confirmed) |
| 5 | H-05 | An agent following AGENTS.md "Key Directories" to find `internal/dolt/` or `internal/models/` will hit phantom paths — both directories do not exist | HIGH | HIGH | AGENTS.md:88-89 vs internal/ listing | Devil's Advocate (5/5 confirmed) |
| 6 | H-06 | `bt robot list` (single-DB path) issues 1+3n queries for n issues, serialized through 5-conn pool; on n=500 the load latency is ~3s | HIGH | HIGH | internal/datasource/dolt.go:273-279,352,371,393 | Performance (4/5 confirmed; severity revised CRITICAL→HIGH) |
| 7 | H-07 | A SHA value beginning with `--` in bead/cache data is passed as a positional arg to `git rev-list/log/show`, treated as a flag (e.g., `--upload-pack=...`, `--output=...`) | HIGH | HIGH | pkg/correlation/incremental.go:200; stream.go:415; temporal.go:143 | Security (4/5 confirmed) |
| 8 | H-08 | A Phase 2 PageRank timeout produces uniform 1/N values that get persisted to `pkg/analysis/cache.go` disk cache; subsequent sessions load degraded ranks as if they were real | HIGH | HIGH | pkg/analysis/graph.go:1649-1662,1871,1982-1984; cache.go:850 | Reliability (4/5 confirmed) |
| 9 | H-09 | `bt` exits via one of the 28 `os.Exit(1)` calls in cmd/bt/root.go after Dolt server start succeeded → orphan Dolt server with `root@tcp(...)/?...` no-password remains running | HIGH | HIGH | cmd/bt/root.go:286-491 | Reliability (4/5 confirmed) |
| 10 | H-10 | `GlobalDoltReader.LoadIssues` returns zero issues when ANY single database in the workspace has a corrupt `issues` table — entire UNION ALL fails | HIGH | HIGH | internal/datasource/global_dolt.go:300-351 | Reliability (4/5 confirmed) |
| 11 | H-11 | `ExtractAllTemporalCorrelations` on m=100 beads averaging k=10 commits each runs ~3000 sequential git subprocesses (~5-15ms each on Windows) on critical path | HIGH | HIGH | pkg/correlation/temporal.go:295-318,143 | Performance (4/5 confirmed) |
| 12 | H-12 | A bt agent running `bt robot triage --shape=full` against a fixture and against current code can produce silently-different JSON for ~26 of ~30 robot subcommands — no golden snapshot regression net | HIGH | HIGH | pkg/view/schemas/ (only 4 types covered) vs cmd/bt/cobra_robot.go (~30 subcommands) | Devil's Advocate (4/5 confirmed) |
| 13 | H-13 | `bt update` without `checksums.txt.sig` accepts ANY tampered binary that returns 0 from `--version` | MEDIUM | MEDIUM | pkg/updater/updater.go:619-661 | Security (5/5 confirmed) — cluster with H-01 |
| 14 | H-14 | The 4 sprint-related cobra subcommands (`bt robot sprint list/show`, `bt robot burndown`, `bt robot forecast --forecast-sprint`) all silently no-op on Dolt-only installs | HIGH | MEDIUM | pkg/loader/sprint.go + cobra_robot.go | Architecture (5/5 confirmed) — cluster with H-04 |
| 15 | H-15 | AGENTS.md claims "AGENTS.md filename hardcoded in 15 Go files" but actual count via grep is 19 across 5 directories (pkg/agents/, pkg/ui/, pkg/baseline/, cmd/bt/, tests/e2e/) | HIGH | MEDIUM | AGENTS.md:84,158 vs Grep | Devil's Advocate (5/5 confirmed) |
| 16 | H-16 | `cobra_robot.go` of 1,301 LOC + 50+ subcommand declarations + 200-line `init()` is an anti-pattern; splitting per-handler-file produces 30 init() functions (different anti-pattern) | HIGH | MEDIUM | cmd/bt/cobra_robot.go | Architecture (4/5 confirmed; recommendation revised) |
| 17 | H-17 | Loading issues for a beads dir has 3 separate code paths (datasource.LoadIssues, datasource.LoadIssuesFromDir, pages.go.loadIssuesFromBeadsDir) with subtly different precedence rules | HIGH | MEDIUM | internal/datasource/load.go; cmd/bt/pages.go:565-599 | Architecture (4/5 confirmed) |
| 18 | H-18 | On Windows, attacker holding write to `.beads/` can create `.beads/.bt.lock` as a symlink/junction to arbitrary file before bt's stale-lock takeover; subsequent Release() follows the link | HIGH | MEDIUM | pkg/instance/lock.go:215-229 | Security (4/5 confirmed) |
| 19 | H-19 | The livereload SSE endpoint at `http://127.0.0.1:9000/__preview__/events` accepts cross-origin EventSource from any web page (CORS=*); DNS-rebinding can read full filesystem path from /__preview__/status | HIGH | MEDIUM | pkg/export/livereload.go:154; preview.go:131-171 | Security (4/5 confirmed) |
| 20 | H-20 | Attacker-controlled `.beads/dolt-server.{pid,port}` files combined with a non-matching `bd dolt start` stdout cause bt to TCP-dial attacker port and issue MySQL queries against attacker service (no password auth) | HIGH | MEDIUM | internal/doltctl/doltctl.go:39-141 | Security (4/5 confirmed) |
| 21 | H-21 | `BatchFileStatsExtractor.cache` grows without bound during long TUI sessions | HIGH | MEDIUM | pkg/correlation/stream.go:339-405 | Performance (4/5 confirmed; framing softened) |
| 22 | H-22 | Phase 2 metric goroutine that times out continues running gonum to completion, holding entire graph reference; repeated watcher ticks pile up abandoned Phase-2 goroutines | HIGH | MEDIUM | pkg/analysis/graph.go:1629-1747 | Performance (4/5 confirmed) |
| 23 | H-23 | `countBlockedBy` is O(n²·d); on 1000-issue graph with chain length 20, ~60,000 dep checks for one blocker chain | HIGH | MEDIUM | pkg/analysis/graph.go:2604-2618 | Performance (4/5 confirmed) |
| 24 | H-24 | livereload debouncer fires on FIRST event and drops trailing events in burst; browser reloads against partially-flushed assets | HIGH | MEDIUM | pkg/export/livereload.go:115-122 | Performance (4/5 confirmed) |
| 25 | H-25 | Phase 2 metric panic (e.g., gonum graph corruption, nil-deref in HITS) is silently recovered with empty body — manifests identical to a slow-timeout to the user | HIGH | MEDIUM | pkg/analysis/graph.go:1633-1640 et al. | Reliability (4/5 confirmed) |
| 26 | H-26 | All 9 git subprocess invocations in pkg/loader/git.go and pkg/correlation/stream.go run without context/timeout; git hang freezes bt forever | HIGH | MEDIUM | pkg/loader/git.go:110,156,167,227,325,381; correlation/stream.go:123,160,417 | Reliability (4/5 confirmed) |
| 27 | H-27 | Tutorial copy + board test + ~98 first-party files reference bv-* bead IDs that no longer resolve in current `.beads/metadata.json:dolt_database="bt"` database | HIGH | MEDIUM | pkg/ui/tutorial.go; pkg/ui/board_test.go:747-748 | Devil's Advocate (4/5 confirmed) |
| 28 | H-28 | GitHub Actions `release.yml` `workflow_dispatch` defaults `snapshot=true`; manual UI re-trigger of a release silently produces snapshot, never real release | HIGH | MEDIUM | .github/workflows/release.yml:13-17,44 | Devil's Advocate (3/5 confirmed) |
| 29 | H-29 | `pkg/ui` Model has 165 method receivers across 16 files and 50+ flat fields; refactor pattern (bt-oim6) is partial | HIGH | MEDIUM | pkg/ui/model.go:464-642 + 16 model_*.go | Architecture (3/5 confirmed; severity revised HIGH→MEDIUM) |
| 30 | H-30 | `pkg/analysis/graph.go` is 2,880 LOC with 111 functions; deprecated map-copy methods present "for backward compatibility" violate AGENTS.md rule #6 | HIGH | MEDIUM | pkg/analysis/graph.go | Architecture (3/5 confirmed) |
| 31 | H-31 | Cross-process stale-lock takeover on Windows can produce two processes both believing they are first-instance via Remove+Rename window | MEDIUM | MEDIUM | pkg/instance/lock.go:166-245 | Reliability (4/5 confirmed) |
| 32 | H-32 | UNION ALL across N databases has no per-DB timeout; one slow DB stalls the entire UNION; LoadIssuesAsOf has resilience LoadIssues lacks | MEDIUM | MEDIUM | internal/datasource/global_dolt.go:300-351 | Performance (3/5 confirmed; severity revised HIGH→MEDIUM) |
| 33 | H-33 | Cross-database SQL escaping via `escapeSQLString`/`backtickQuote` is brittle if MySQL `NO_BACKSLASH_ESCAPES` is off and dbName contains `\'`; multi-tenant Dolt server makes dbName a trust boundary | MEDIUM | MEDIUM | internal/datasource/global_dolt.go:436,455,472,490,507,526-527,807-816 | Security (3/5 confirmed) |
| 34 | H-34 | `Dicklesworthstone/toon-go` pseudo-version dep + vendor/ has no CI gate verifying `go mod vendor` matches go.sum on every PR | MEDIUM | MEDIUM | go.mod | Security (3/5 confirmed) |
| 35 | H-35 | 1,332 `bv-*` sigils across 98 first-party Go files reference a bead prefix that no longer exists in the current beads database | HIGH | LOW | pkg/ui/styles.css (180); pkg/ui/board.go (71); pkg/ui/history.go (92); pkg/ui/model.go (36); + 94 more | Architecture (3/5 confirmed) |

## Minority hypothesis (preserved per anti-herd protocol)

| H-M1 | AR-4 | bt single-binary architecture means every `bt robot <subcmd>` invocation links the entire 38.9k-LOC pkg/ui dependency closure (charm.land/* + clipboard); architectural decision was never explicitly made in any ADR | HIGH | MEDIUM | cmd/bt/root.go:37 | Architecture (1/5 confirmed) |

## Cluster groupings (for chain-handoff)

| Cluster | Hypotheses | Suggested chain |
|---|---|---|
| **Self-update integrity** | H-01, H-13 | `--chain security` then `--chain fix` |
| **SQLite legacy / data layer** | H-03, H-17 | `--chain fix` (bt-05zt is unblocked) |
| **Sprint feature stale** | H-04, H-14 | `--chain fix` after bt-z5jj decision |
| **Phase 2 timeout machinery** | H-08, H-22, H-25 | `--chain debug` to validate panic vs timeout distinguishing |
| **Doc/code drift** | H-05, H-15, H-27, H-28 | `--chain fix` (low-effort, high-leverage) |
| **Robot output schemas** | H-12 | `--chain fix` to bt-ah53 (currently OPEN P1) |
| **Git shell-out hardening** | H-07, H-11, H-26 | `--chain security` then `--chain fix` (validateSHA helper centralizes) |
| **Stale-lock Windows** | H-18, H-31 | `--chain fix` (one PR for both) |
| **Live-reload server** | H-19, H-24 | `--chain fix` |
| **N+1 / perf hot paths** | H-06, H-21, H-22, H-23 | `--chain debug` to bench, then `--chain fix` |
| **Hooks RCE** | H-02 | `--chain security` (full STRIDE on hooks subsystem) |

## Empirical evidence rule reminder

If a downstream chain (debug, security) disproves a hypothesis (e.g., `validateSHA` is already enforced by the upstream regex resolution), log as `H-NN DISPROVEN by <tool> loop` in the downstream report. **Predictions are starting points, not conclusions.** Empirical loops always override swarm consensus.
