# Findings — bt full-swarm prediction

Ranked by `priority_score = severity_weight × 0.4 + confidence_boost × 0.2 + consensus_ratio × 0.4`.
Severity weights: CRITICAL=4, HIGH=3, MEDIUM=2, LOW=1.
Confidence boost: HIGH=1.0, MEDIUM=0.6, LOW=0.3.
Consensus ratio: confirms / 5.

**Status legend (post-debate):**
- **Confirmed** = ≥3 of 5 personas confirm
- **Probable** = 2 of 5 confirm
- **Minority** = 1 of 5 confirm (preserved per anti-herd protocol)

**Cluster cross-links** are noted at the top of each finding when multiple personas raised related issues.

---

## 1. RE-4 — Updater proceeds without verification when checksums.txt asset is absent

**Severity:** CRITICAL  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 2.20**
**Cluster:** Self-update integrity (with SA-6).
**Location:** `pkg/updater/updater.go:623-645, 657-660`

**Evidence:** `downloadAndApply` only verifies if checksumAsset is present (line 624 `if checksumAsset != nil`). When a release ships without a `checksums.txt` asset (operator forgot, name mismatch in `FindChecksumAsset` line 391-394, partial GitHub upload), the entire verification block is skipped. bt then downloads, extracts, runs the binary with `--version` (line 659), and atomically replaces the running binary on rename success (line 673). `runCommand` (line 718-722) has zero timeout — a malicious binary that hangs blocks forever; one that returns 0 with arbitrary content sails through. No signature verification, no fallback to refusal, no warning.

**Recommendation:** Refuse to apply when checksumAsset is nil. Return `release missing checksums.txt; refusing to apply unverified update` and require `--allow-unverified` to override. Add timeout to `runCommand`. Long-term: sign release binaries (cosign/minisign) and verify signatures, not just hashes.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | "Default-allow on missing artifact" is the architectural defect |
| Security Analyst | confirm | Cross-link with SA-6 (self-update no signature) |
| Performance Engineer | confirm | `runCommand` no timeout = hang |
| Reliability Engineer | confirm | Origin finding |
| Devil's Advocate | concede with conditions | Reframe as "deployment-time defense in depth" — scenario hasn't happened yet but silent-skip logic is wrong |

---

## 2. SA-1 — Arbitrary code execution via `.bt/hooks.yaml` in cloned repos

**Severity:** CRITICAL  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (PE abstain)  **Score: 2.12**
**Location:** `pkg/hooks/config.go:101, pkg/hooks/executor.go:120-121, cmd/bt/cobra_export.go:48-51,103,133`

**Evidence:** `Loader.Load()` reads `.bt/hooks.yaml` from `projectDir` (defaulting to `os.Getwd()`) and passes resulting `Hook.Command` strings into `sh -c` / `cmd /C` via `exec.CommandContext`. The `--no-hooks` flag default is `false` (cobra_export.go:405,413) so `bt export` (and `bt export-md` / `bt export pages`) execute pre/post hooks BY DEFAULT. Trust boundary is the project working directory: cloning any untrusted repo and running `bt export` (or being phished into doing so) yields arbitrary shell command execution under the user's UID. No signature, allowlist, sandbox, prompt, or attestation. Standard "git-config style" RCE vector.

**Recommendation:** Default to no-execute on first encounter. Hash the file, store allow-record in `~/.bt/hook-trust.json` keyed by abs path + content hash, refuse execution until user runs `bt hooks trust` (or `--allow-hooks`). At minimum: invert flag default (`--no-hooks=true` becomes safe default; require `--allow-hooks` to opt in) and print one-line warning naming each hook command before running. Add `bt hooks list`. Document in `SECURITY.md`.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Trust-boundary inversion; recommend TOFU prompt rather than disable-by-default UX cliff |
| Security Analyst | confirm | Origin finding |
| Performance Engineer | abstain | Outside performance domain |
| Reliability Engineer | confirm | Adds: no rollback on hook failure — pre-export hook can destroy local state and post-export never runs |
| Devil's Advocate | concede with conditions | Trust boundary is right; reframe as "high-impact tail event" since bt is solo-dev tool — but still CRITICAL |

---

## 3. AR-2 — Data layer carries dead `SourceTypeSQLite` branch on every load despite ADR-003 acceptance

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 1.80**
**Cluster:** SQLite legacy (with DA-3 — same root cause, different framing).
**Location:** `internal/datasource/source.go:33-34,147-151,203-224`; `internal/datasource/load.go:172-178`; `internal/datasource/sqlite.go:1-397`; `cmd/bt/pages.go:483,550,580-585`; `cmd/bt/root.go:247`; `internal/datasource/source_test.go` (14 references)

**Evidence:** ADR-003 was accepted 2026-04-25 with Option (b): collapse SourceType to `Dolt | DoltGlobal | JSONLFallback`. Implementation tracked in bt-05zt. At HEAD: `discoverSQLiteSources` (source.go:204) is still in pipeline, 5-element enum intact, `LoadFromSource` still dispatches to `NewSQLiteReader`. Beads removed SQLite as backend at v0.56.1 (March 2026); per ADR-003, projects on current `bd` will never have `.beads/beads.db`. So every cold load runs `os.Stat(beadsDir + "/beads.db")` for nothing, plus 397 LOC of reader, plus the priority-math machinery that picked between co-equal backends. `pages.go` even hand-rolls its own `metadataPreferredSource` (lines 525-563) re-implementing priority resolution outside the discovery pipeline.

**Recommendation:** Land bt-05zt Phase 1 (mechanical SQLite removal) — unblocked, ADR accepted. Then Phase 2 (collapse enum). While in there, fold pages.go's hand-rolled metadataPreferredSource into the canonical discovery pipeline.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Origin finding |
| Security Analyst | confirm | A stale `.beads/beads.db` at PrioritySQLite=100 outranks JSONL=50 — could feed bt mismatched data |
| Performance Engineer | confirm | Eliminates dead `os.Stat` on every load |
| Reliability Engineer | confirm | Removes a code path that diverges from `bd list` |
| Devil's Advocate | concede with edge case | "Maybe keep reader for explicit `--source sqlite=...`, drop discovery only" — counter-design worth weighing in bt-05zt |

---

## 4. DA-3 — Project memory says SQLite is export-only; code wires `SourceTypeSQLite` into runtime

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 1.80**
**Cluster:** SQLite legacy (with AR-2). Cross-link AR-2 — same code, this is the doc-drift framing.
**Location:** `AGENTS.md:146` + project MEMORY.md "Pure-Go SQLite" note vs `internal/datasource/load.go:172`, `internal/datasource/source.go:212`, `cmd/bt/pages.go:580-582`, `cmd/bt/root.go:247`

**Evidence:** AGENTS.md:146 states "Pure-Go SQLite ... is used only by the SQLite **export** artifact ... There is no SQLite at runtime." Code contradicts: source.go:204-223 actively discovers `.beads/beads.db` at PrioritySQLite=100; load.go:172-173 dispatches `SourceTypeSQLite`; root.go:247 lists SQLite as fallback; pages.go:580-582 instantiates SQLiteReader directly. Operationally: stale beads.db files from pre-v0.56.1 era get picked at PrioritySQLite=100 over JSONL=50, returning data that diverges from live Dolt.

**Recommendation:** Until ADR-003/bt-05zt lands, AGENTS.md:146 must be corrected to "SQLite reader still wired (pre-Dolt legacy); ADR-003 collapse pending in bt-05zt." Right answer: ship bt-05zt Phase 1 now.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Same root cause as AR-2 |
| Security Analyst | confirm | Documentation drift means agents trust wrong invariants |
| Performance Engineer | confirm | Dead-load cost noted in AR-2 |
| Reliability Engineer | confirm | Diverged data sources are a reliability hazard |
| Devil's Advocate | confirm (origin) | Self-revise: if SQLite reader is kept-but-deprecated-in-discovery, doc line should be "explicit `--source sqlite=...` use only" |

---

## 5. DA-2 — `pkg/loader/sprint.go` is a textbook stale-Dolt-assumption violation per AGENTS.md checklist

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 1.80**
**Cluster:** Sprint feature stale (with AR-6).
**Location:** `pkg/loader/sprint.go:15,28` + `cmd/bt/robot_sprint.go:22` + `AGENTS.md:125`

**Evidence:** AGENTS.md:125 explicitly: "Does it assume `.beads/sprints.jsonl` exists? (Beads doesn't produce one.)" Yet `sprint.go:15` declares `const SprintsFileName = "sprints.jsonl"` and line 28 reads `filepath.Join(repoPath, ".beads", SprintsFileName)`. cobra_robot.go:1208-1210 wires `bt robot sprint list/show`, plus `bt robot burndown` and `bt robot forecast --forecast-sprint`. On any Dolt-only beads install (every install per AGENTS.md:72), the file never exists; `LoadSprints` silently returns `[]model.Sprint{}` with no diagnostic. Operationally: agents trying `bt robot sprint list` get empty results and no error. bt-z5jj is open. Audit-vs-implementation lag is itself the bug.

**Recommendation:** Either (a) gate `runSprintListOrShow`/`runBurndown`/`--forecast-sprint` to error with a clear "sprints not supported on Dolt-only beads; tracked in bt-z5jj" message, or (b) remove sprint subcommands from cobra registration in cobra_robot.go init() and keep loader as dead code pending bt-z5jj. Doing nothing is worst option.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Pairs with AR-6 |
| Security Analyst | confirm (light) | Silent-empty-result is a reliability/UX hazard; not a sec finding directly |
| Performance Engineer | confirm | Dead code is dead cost |
| Reliability Engineer | confirm | Silent-empty without diagnostic is anti-CLAUDE.md "fail loudly" rule |
| Devil's Advocate | confirm (origin) | Either rebuild against Dolt or delete; current limbo is worst-of-both-worlds |

---

## 6. DA-1 — AGENTS.md "Key Directories" lists `internal/dolt/` and `internal/models/` that do not exist

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 1.80**
**Cluster:** Doc-drift cluster (with DA-3, DA-4, DA-6, AR-7).
**Location:** `AGENTS.md:88-89` vs `internal/` (only `datasource/`, `doltctl/`, `settings/`)

**Evidence:** AGENTS.md:88 declares `internal/dolt/   # Dolt-specific reader` and AGENTS.md:89 declares `internal/models/   # Issue data structures`. Actual: Dolt reader lives at `internal/datasource/dolt.go` and `global_dolt.go`; issue data structures at `pkg/model/issue.go`. Canonical onboarding doc points agents to two phantom packages and omits real ones. Any agent following AGENTS.md to find data-layer code wastes the first lookup.

**Recommendation:** Update AGENTS.md "Key Directories" to: replace `internal/dolt/` with `internal/datasource/` + `internal/doltctl/`; replace `internal/models/` with `pkg/model/`. Add `pkg/view/` (CompactIssue projections) since it's load-bearing for robot output.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Onboarding doc accuracy is foundational |
| Security Analyst | confirm | Misleading docs lead agents to wrong assumptions |
| Performance Engineer | confirm | Wasted lookups are minor but real |
| Reliability Engineer | confirm | Agent-experience reliability finding |
| Devil's Advocate | confirm (origin) + extend | Most agent-toxic finding in the swarm; should be P0 because it BLOCKS agent productivity |

---

## 7. PE-1 — N+1 query storm on every single-DB issue load (3 round-trips per issue)

**Severity:** HIGH (revised from CRITICAL after DA challenge)  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (DA dispute on severity, confirm on existence)  **Score: 1.72**
**Location:** `internal/datasource/dolt.go:273-279`; helpers at lines 352, 371, 393

**Evidence:** `DoltReader.LoadIssuesFiltered` runs the issues SELECT (line 125), then in the per-row loop calls `r.loadLabels(issue.ID)` (273), `r.loadDependencies(issue.ID)` (276), `r.loadComments(issue.ID)` (279). Each helper executes `db.Query("... WHERE issue_id = ?", issueID)`. For n issues, total queries = 1 + 3n. Connection pool is `SetMaxOpenConns(5)` (line 46), so when n>5 queries serialize through 5 conns, multiplying RTT by ~n/5. The global path solves this with batched UNION-ALL loads (`loadAllLabels`, `loadAllDependencies`, `loadAllComments` in global_dolt.go:706-804) — the single-DB path doesn't. No benchmark covers this.

**Recommendation:** Add `loadAllLabels(issueIDs)`, `loadAllDependencies(issueIDs)`, `loadAllComments(issueIDs)` to `DoltReader` with batched `WHERE issue_id IN (?,?,...)`. Mirror the global reader's `issueMap` pattern. O(n) → O(1) wire RTT. Add benchmark with n in {100, 500, 2000}.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Make GlobalDoltReader's batched API canonical |
| Security Analyst | confirm | Connection-pool exhaustion is also a DoS adjacency |
| Performance Engineer | confirm | Origin finding |
| Reliability Engineer | confirm | Pool exhaustion under N+1 → goroutine pile-up → deadlock-adjacent |
| Devil's Advocate | dispute on severity | Real-world n usually <500; CRITICAL → HIGH |

---

## 8. SA-2 — Git argument injection via unvalidated SHAs (no `--end-of-options`)

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (PE abstain)  **Score: 1.72**
**Location:** `pkg/correlation/incremental.go:200, 226`; `pkg/correlation/temporal.go:143`; `pkg/correlation/stream.go:113-123, 160, 415-417`

**Evidence:** SHAs originating from bead payloads, JSONL legacy data, or correlation cache flow to git subprocess invocations as positional args without (a) hex validation or (b) `--end-of-options`. Concrete cases: `incremental.go:200` `exec.Command("git", "rev-list", "--reverse", fmt.Sprintf("%s..HEAD", sinceSHA))` — `sinceSHA` appended unverified; if it begins with `--` git treats it as a flag (e.g., `--upload-pack=evil`, `--output=/tmp/x`). `stream.go:415` `args = append(args, shas...)` then `exec.Command("git", args...)` — every SHA can be attacker-controlled flag. `temporal.go:143` `exec.Command("git", "show", "--name-only", "--format=", sha)` no validation. Attack model: any path letting attacker influence a SHA value (Dolt INSERT via shared server, JSONL fixture, malicious `--diff-since`) gains arbitrary git flag injection. The 260427 STRIDE audit flagged this with a recommended `validateSHA()` helper; not implemented (recon docs: "Δ since last audit: None"). Only `pkg/loader/git.go:156` was hardened.

**Recommendation:** Add `func validateSHA(s string) error` requiring `^[0-9a-fA-F]{4,64}$` (matching git's abbrev-min and full-SHA256 max). Call at every site where SHA enters from bead/cache data. Insert `--end-of-options` immediately before the first positional ref in each `exec.Command("git", ...)`. Audit checklist in `docs/audits/security/260427-0039-stride-owasp-full-audit/recommendations.md`.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Single helper centralizes the discipline |
| Security Analyst | confirm | Origin finding |
| Performance Engineer | abstain | Outside domain |
| Reliability Engineer | confirm | Pairs with RE-6 (no timeouts) — same shell-out surface |
| Devil's Advocate | confirm + extend | Prior STRIDE audit recommended this and it wasn't shipped — surface to ADR-002 stream |

---

## 9. RE-2 — Phase 2 timeout fallback (uniform PageRank) is silently persisted to disk cache

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (DA abstain on this specific framing)  **Score: 1.72**
**Cluster:** Phase 2 timeout machinery (with PE-5, RE-5).
**Location:** `pkg/analysis/graph.go:1649-1662, 1871, 1982-1984`; `pkg/analysis/cache.go:850`

**Evidence:** When PageRank's timer fires, code substitutes uniform 1/N PageRank (lines 1652-1655) and sets `profile.PageRankTO=true`. Execution continues; Betweenness/HITS/Cycles run, then atomic block at line 1851 sets `stats.phase2Ready=true` at line 1871. `computePhase2` (line 1982) unconditionally calls `putRobotDiskCachedStats`. cache.go:850 only checks `IsPhase2Ready()` — true even when results are degraded fallbacks. Result: a transient slow run gets baked into on-disk cache; subsequent invocations load uniform-PageRank as if it were real.

**Recommendation:** Skip `putRobotDiskCachedStats` when ANY `profile.*TO` is true OR when status indicates timeout/panic. Or include timeout status in cache key.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Cache invariant violated |
| Security Analyst | abstain | No security angle |
| Performance Engineer | confirm | Cached degraded results poison perf budget for next session |
| Reliability Engineer | confirm | Origin finding |
| Devil's Advocate | confirm | Part of Phase 2 systemic redesign hypothesis |

---

## 10. RE-1 — `os.Exit` short-circuits Dolt server cleanup, orphaning bd-spawned processes

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (PE abstain)  **Score: 1.72**
**Location:** `cmd/bt/root.go:286-491`

**Evidence:** `runRootTUI()` at line 286 has no `defer` for `appCtx.serverState.StopIfOwned()`. `loadIssues()` (called at 311) sets `appCtx.serverState` via `doltctl.EnsureServer` (root.go:231-235); the only StopIfOwned is on the load-failure path (line 238). The function then runs through ~13 `os.Exit` calls (313, 334, 343, 347, 374, 393, 485, 491, 515, 522, 581, 596, 610). `os.Exit` bypasses deferred functions per the Go spec. Any path that started a Dolt server but exits via `os.Exit` (BQL parse error, recipe lookup, terminal detection failure) leaves the bd-started Dolt server running. PID file at `.beads/dolt-server.pid` persists; next bt run sees a still-live PID and attaches as non-owner, never cleaning up.

**Recommendation:** Wrap `runRootTUI` with `defer func() { if appCtx.serverState != nil { appCtx.serverState.StopIfOwned() } }()` immediately after `loadIssues` succeeds. Better: replace `os.Exit` in this function with `return` + single exit at top of `Run` that runs cleanup. Best: register cleanup with signal handlers (SIGINT/SIGTERM also bypass deferred cleanup if not registered).

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Same lifecycle anti-pattern that AR-3 cobra_robot god-file finding implies |
| Security Analyst | confirm | Orphan Dolt with `root@tcp(...)/?...` no-password = local-attack escalator |
| Performance Engineer | abstain | Outside domain |
| Reliability Engineer | confirm | Origin finding |
| Devil's Advocate | confirm + extend | Affects upstream beads design — file co-bead in beads repo |

---

## 11. RE-3 — `GlobalDoltReader.LoadIssues` fails entire load if any single database errors

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (PE abstain on framing)  **Score: 1.72**
**Location:** `internal/datasource/global_dolt.go:300-351`

**Evidence:** `LoadIssues` builds one UNION ALL across all enumerated databases (line 306-311), executes once (311), returns first error wrapped (313). MySQL UNION ALL is all-or-nothing: any single DB with corrupt issues table, broken Dolt state, or column type mismatch (NULL-substitution can't paper over) errors the whole query — bt returns no issues from any database. Compare `LoadIssuesAsOf` at line 360-399 which queries each DB individually, accumulates `dbErrors`, and only fails when all DBs fail. Asymmetry: historical view is more reliable than live view — opposite of user expectation. User with 12 working + 1 broken project sees zero issues live but full history with `--as-of`.

**Recommendation:** Refactor `LoadIssues` to mirror `LoadIssuesAsOf`'s per-DB loop: query each DB individually, log+skip per-DB errors, succeed if ≥1 DB loaded. Expose failed databases in result so UI can surface degraded state.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Symmetric with `LoadIssuesAsOf` is the architectural fix |
| Security Analyst | abstain | No security angle |
| Performance Engineer | confirm | Pairs with PE-2 (UNION ALL no per-DB timeout) |
| Reliability Engineer | confirm | Origin finding |
| Devil's Advocate | confirm + extend | Live-vs-history reliability asymmetry is itself a doc-worthy invariant |

---

## 12. PE-4 — Temporal correlator runs git subprocesses sequentially per bead with nested fanout

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (SA abstain)  **Score: 1.72**
**Location:** `pkg/correlation/temporal.go:295-318`

**Evidence:** `ExtractAllTemporalCorrelations` iterates `histories` (n = beads with completed Claimed→Closed) and calls `FindCommitsInWindow` synchronously (line 308). Each spawns one `git log` subprocess (66-69). Inside, for every matching commit it calls `t.touchesBeadsFile(sha)` which spawns `git show --name-only` (143), then `t.coCommitter.ExtractCoCommittedFiles(...)` which is another git call. Process cost: m beads × (1 + k_m commits × 2 git subprocesses), all sequential. Subprocess fork+exec on Windows is ~5-15ms. For m=100 beads averaging k=10 commits: ~3000 git subprocess invocations on the critical path, sequentially. No `errgroup`, no parallelism. The `BatchFileStatsExtractor` already exists at `stream.go:364` and uses `git log --no-walk <shas...>` to fetch many commits in one subprocess — built but not used here.

**Recommendation:** (a) Parallelize the outer loop over beads with `errgroup.Group` capped at `runtime.NumCPU()`. (b) Replace per-commit `git show` + per-commit file-extract calls with `BatchFileStatsExtractor.ExtractBatch(shas)` — infrastructure already built.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Existing batch helper unused = abstraction-layering gap |
| Security Analyst | abstain | Outside domain |
| Performance Engineer | confirm | Origin finding |
| Reliability Engineer | confirm | Pairs with RE-6 (no timeouts) — same git-shell-out surface |
| Devil's Advocate | confirm | The batch helper exists but isn't called — that's the bug |

---

## 13. DA-7 — Robot output schemas exist for only 4 types; ~26 subcommands have no regression net

**Severity:** HIGH  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (PE abstain)  **Score: 1.72**
**Location:** `pkg/view/schemas/` (only 6 .json files for pair/ref/portfolio/compact_issue) vs `cmd/bt/cobra_robot.go` listing ~30 robot subcommands; `cmd/bt/robot_schema_flag.go:13-15`

**Evidence:** AGENTS.md:144 says "Robot-first API: All `bt robot <subcmd>` invocations emit deterministic JSON to stdout" and ADR-002 Stream 1 says "Robot mode I/O contract: documented invariants + verify-test sweep (bt-ah53, bumped P2 → P1 on 2026-04-28)." That bead is OPEN. Inventory: pkg/view/schemas/ contains only `compact_issue.v1.json`, `pair_record.{v1,v2}.json`, `portfolio_record.v1.json`, `ref_record.{v1,v2}.json`. But cobra_robot.go registers ~30 subcommands. `bt robot schema` (cobra_robot.go:499-547) generates schemas at runtime via `generateRobotSchemas()` rather than checking against frozen files — so any field rename/type change/nil-vs-empty-array switch in runtime output passes its own schema check trivially. Spot-check: zero `*.golden.json` snapshot files under cmd/bt/. Operationally: most agent-facing surface has no regression net for ~26/30 subcommands.

**Recommendation:** Acknowledge bt-ah53 framing is right; prioritize. At minimum, capture golden snapshots of every `bt robot <subcmd> --shape=full` and `--shape=compact` against fixed fixtures (helpers exist in `robot_all_subcommands_test.go`); diff current output against frozen JSON in `go test`.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Contract enforcement = wire-protocol discipline |
| Security Analyst | confirm | Schema regressions are integrity vulnerabilities for downstream consumers |
| Performance Engineer | abstain | Outside domain |
| Reliability Engineer | confirm | "Trust me" plan = anti-CLAUDE.md |
| Devil's Advocate | confirm (origin) + extend | Highest-leverage long-term find — every future subcommand benefits from the guardrail |

---

## 14. SA-6 — Self-update verifies new binary by executing it before installation but does not verify GPG/cosign signature

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 1.40**
**Cluster:** Self-update integrity (with RE-4).
**Location:** `pkg/updater/updater.go:619-661`

**Evidence:** Flow: HTTPS download from GitHub release → checksums.txt download → SHA256 verify → extract → run new binary with `--version` (659) → install. Good: HTTPS, redirect-handling strips `Authorization` on non-GitHub hosts (410-417), checksum verified before run. Gap: `checksums.txt` from same release, not signature-verified. Compromise of GitHub release (stolen PAT, account takeover, supply-chain on goreleaser) replaces both `bt_*.tar.gz` AND `checksums.txt` — checksum gives zero protection because both come from same trust root. Self-update silent (no `--yes` bypass), atomically overwrites binary. Combined with absence of sigstore/cosign/minisign: single GitHub credential compromise = persistent RCE on every user who runs `bt update`. Also: `runCommand` running new binary with `--version` is moment of execution — proves nothing useful.

**Recommendation:** Sign artifacts with cosign keyless (Sigstore — works with GitHub OIDC). Verify signature in `PerformUpdate` before extraction. Interim: fetch `checksums.txt.sig` (GPG) and verify against pubkey embedded in binary. Document in SECURITY.md and require user confirmation on first update after key rotation. Stop running new binary with `--version` for "verification" — hands attacker-controlled code execution earlier than necessary.

**Persona votes:** All 5 confirm. Cross-link RE-4 (the more acute version: skip-verify when checksums missing).

---

## 15. AR-6 — `pkg/loader/sprint.go` reads `.beads/sprints.jsonl` — architecture references a file beads never produces

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 1.40**
**Cluster:** Sprint feature stale (with DA-2).
**Location:** `pkg/loader/sprint.go:14-46`; `pkg/model/types.go:332`; `cmd/bt/robot_sprint.go`; `pkg/ui/sprint_view.go`; `pkg/ui/model.go:622-624`

**Evidence:** AGENTS.md "Beads architecture awareness": "Sprints: NOT a beads concept upstream — no `sprints` table or subcommand. Any sprint-related code in bt is bt-only (tracked in bt-z5jj — rebuild against Dolt or retire)." But sprint.go:28 hardcodes `filepath.Join(repoPath, ".beads", "sprints.jsonl")` — a file beads-the-tool never writes. Sprint type in pkg/model. TUI has dedicated focusSprint mode, ViewSprint enum, sprint_view.go renderer, dedicated state on Model. Robot subcommand wired in cobra_robot.go. Complete user-facing feature pretending to be a domain primitive but pointing at empty filesystem location.

**Recommendation:** bt-z5jj's call. Either retire the sprint feature entirely (delete loader, type, TUI mode, robot subcommand) or rebuild against Dolt as bt-owned table. Current state is worse than either outcome — broken feature path stays in binary, TUI exposes mode that always shows zero sprints.

**Persona votes:** All 5 confirm. Same finding as DA-2 (different framing — architectural vs convention-violation). Both ranked.

---

## 16. DA-4 — AGENTS.md claims AGENTS.md filename is hardcoded in 15 Go files; actual count is 19 across 5 directories

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 5/5 confirmed  **Score: 1.40**
**Cluster:** Doc-drift cluster.
**Location:** `AGENTS.md:84,158` vs Grep for `AGENTS\.md` across `*.go`

**Evidence:** AGENTS.md:84: `pkg/agents/  # AGENTS.md filename hardcoded in 15 Go files`. AGENTS.md:158 repeats: "AGENTS.md filename is hardcoded in `pkg/agents/` (15 Go files) — content can change, filename must stay." Reality: 19 Go files reference the literal string; only 8 live under pkg/agents/. The other 11 are in pkg/ui/ (model.go, model_update_input.go, model_update_analysis.go, tutorial.go, tutorial_content.go, agent_prompt_modal.go, agent_prompt_modal_test.go), pkg/baseline/baseline.go, cmd/bt/cobra_agents.go, cmd/bt/cli_agents.go, tests/e2e/agents_integration_e2e_test.go. An agent told "filename is hardcoded in pkg/agents/" who attempts a rename leaves 11 broken references in TUI/baseline.

**Recommendation:** Update AGENTS.md to: "AGENTS.md filename is referenced in 19 Go files across pkg/agents/, pkg/ui/, pkg/baseline/, cmd/bt/, tests/e2e/." Better: factor filename into single `agents.AgentsFileName` constant and import it everywhere — turns claim into verifiable single-point invariant.

**Persona votes:** All 5 confirm. The constant-extraction recommendation is the architecturally clean fix.

---

## 17. AR-3 — `cmd/bt/cobra_robot.go` is a 1,301-line registration god-file holding 50+ subcommand definitions

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (DA challenges recommendation)  **Score: 1.32**
**Location:** `cmd/bt/cobra_robot.go` (1,301 LOC; 50+ `var robot*Cmd` declarations from line 190-1110, plus 200+ line aggregating `init()` from line 1114)

**Evidence:** cobra_robot.go declares parent `robotCmd` plus every leaf: triage, next, insights, plan, priority, graph, alerts, search, bql, history, suggest, diff, metrics, recipes, schema, docs, help, drift, sprint+2 children, labels+3 children, files+3 children, correlation+4 children, impact, related, blocker-chain, impact-network, causality, orphans, portfolio, pairs, refs, forecast, burndown, capacity, baseline+2 children. Handler `RunE` bodies live in sibling files but command-tree wiring is centralized here. Adding any subcommand mutates this file. Inconsistent: parent groups exist as separate files for some (robot_pairs.go, robot_portfolio.go, robot_refs.go) but their `Cmd` declarations stay in cobra_robot.go.

**Recommendation (revised after DA round 2):** Split BY COMMAND GROUP, not by individual subcommand. Each group (search, history, sprint, labels, files, correlation, baseline) gets its own file with the group's commands and their flag wiring. The `bt robot` parent stays in cobra_robot.go. Avoids the `init()` spaghetti DA flagged.

**Persona votes:**
| Persona | Vote | Note |
|---|---|---|
| Architecture Reviewer | confirm | Origin finding |
| Security Analyst | abstain | No security surface |
| Performance Engineer | abstain | No perf impact |
| Reliability Engineer | confirm | Big files = big merge conflicts |
| Devil's Advocate | confirm with revised recommendation | "Split per subcommand → 30 init() functions" is wrong direction — split per group |

---

## 18. AR-8 — Workspace/repo loading split across 4 packages with no clear ownership boundary

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (SA abstain)  **Score: 1.32**
**Location:** `pkg/loader/loader.go`; `pkg/loader/sprint.go`; `internal/datasource/load.go:152-186`; `internal/datasource/source.go:101-201`; `pkg/workspace/loader.go`; `internal/doltctl/doltctl.go`; `cmd/bt/root.go:200-265`; `cmd/bt/pages.go:478-599`

**Evidence:** "Load issues from somewhere" requires going through ≥3 packages. internal/datasource is canonical orchestrator. pkg/workspace exists for multi-repo (ADR-003: bt-3ltq blocked on workspace loader being Dolt-aware). pkg/loader still owns JSONL parsing AND sprint loader AND git rev resolution. internal/doltctl owns server lifecycle but LoadFromSource opens its own MySQL conn. cmd/bt/pages.go:565-599 has its own `loadIssuesFromBeadsDir` that re-implements dispatch (Dolt → SQLite → JSONL) outside canonical pipeline. Three different "how to load issues from a beads dir" code paths with subtly different precedence rules. Test coverage thin (5 tests for 9 datasource files).

**Recommendation:** Use ADR-003 Phase 2 as forcing function. After SourceType collapses to `Dolt | DoltGlobal | JSONLFallback`, audit every "load issues" call site and route through single entry point. pages.go's metadataPreferredSource + loadIssuesFromBeadsDir disappear into datasource (or thin wrapper). pkg/workspace becoming Dolt-aware is bt-3ltq; file dependency explicitly so it doesn't accidentally become a fourth load path.

**Persona votes:** AR confirm (origin), SA abstain, PE confirm (deduplication = perf), RE confirm (consolidation = fewer divergence bugs), DA confirm.

---

## 19. SA-3 / RE-8 — Stale-lock takeover races on Windows

**Severity:** MEDIUM  **Confidence:** HIGH (SA-3) / MEDIUM (RE-8)  **Consensus:** 4/5 confirmed (PE abstain)  **Score: 1.32 (SA-3) / 1.24 (RE-8)**
**Cluster:** Same root cause, two framings (security TOCTOU + reliability multi-instance).
**Location:** `pkg/instance/lock.go:215-229` (SA-3); `pkg/instance/lock.go:166-245` (RE-8)

**SA-3 evidence:** Atomic-rename design correct on POSIX but `checkStale()` falls back to `os.Remove(l.path)` followed by `os.Rename(tmpPath, l.path)` on Windows (219-225). Between Remove and Rename, attacker holding read access can create `.beads/.bt.lock` as symlink/junction pointing to arbitrary file (Windows supports for unprivileged users since Win10 with Developer Mode). Subsequent `os.Rename(tmpPath, l.path)` and any later `Release()` `os.Remove(l.path)` follows the link. The post-rename `verifyInfo.PID != os.Getpid()` check (235) detects lost-rename race but not symlink redirection. Trust boundary: anyone with write access to `.beads/`.

**RE-8 evidence:** Cross-process takeover on Windows: process A removes lock (220), process B (also detecting stale at same instant) calls os.Rename which succeeds because file is gone, process A's os.Rename succeeds too — overwriting B's lock. Verify step (234-238) catches for B but file content is now A's PID. If process A crashes between Remove (220) and Rename (221), lock file is gone entirely; subsequent process gets `isFirst=true` via O_EXCL path at 77 — multiple "first instance" processes coexist transiently. Windows OpenProcess can return success for recently-exited recycled PIDs, blocking takeover even though no bt is running.

**Recommendation:** Hold lock file open with OS-level advisory lock (flock POSIX, LockFileEx Windows) for process lifetime — unifies "process alive" with "lock held"; OS releases on crash. Plus: `os.Lstat` before Remove, refuse if `info.Mode()&os.ModeSymlink != 0` (TOCTOU defense). Add takeover-race regression test on Windows with concurrent `NewLock` callers.

**Persona votes:** SA confirm (origin SA-3), AR confirm (lock lifecycle is architectural), PE abstain, RE confirm (origin RE-8), DA confirm + merge-fix recommendation.

---

## 20. SA-4 — Live-reload SSE endpoint sets `Access-Control-Allow-Origin: *` with no Origin/CSRF check

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 4/5 confirmed (PE confirms with perf adjacency)  **Score: 1.32**
**Location:** `pkg/export/livereload.go:154`

**Evidence:** `SSEHandler` unconditionally writes `w.Header().Set("Access-Control-Allow-Origin", "*")`. Preview server binds to 127.0.0.1:9000-9100 (preview.go:60, 188), but `*` plus no `Origin` / `Sec-Fetch-Site` validation means any web page can `new EventSource("http://127.0.0.1:9000/__preview__/events")` and receive: (a) timing oracle for "is sms running bt right now," (b) presence oracle for fingerprinting. `/__preview__/status` (preview.go:131-171) returns absolute filesystem path — readable cross-origin under right conditions. DNS-rebinding: attacker domain rebinds to 127.0.0.1, bypasses SOP, reads SSE stream + `bundle_path` (full local FS path including username).

**Recommendation:** Drop `Access-Control-Allow-Origin: *`. Add `Origin` header check on `/__preview__/*`: reject if Origin set and not `http://127.0.0.1:<port>` or `http://localhost:<port>`. Add `Host` header check: reject if doesn't match `127.0.0.1:<port>` (DNS-rebinding defense). Stop returning absolute bundle path in `/__preview__/status` — return relative basename or omit.

---

## 21. SA-7 / RE-7 — `bd dolt start` stdout-regex parsing fails open via attacker-controlled `.beads/dolt-server.{pid,port}` files

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 4/5 confirmed  **Score: 1.32**
**Cluster:** Same root cause, security + reliability framings.
**Location:** `internal/doltctl/doltctl.go:39-141`

**SA-7 evidence:** EnsureServer resolves `bd` via `lookPath("bd")` (75) — first PATH match wins (PATH-hijacking). Once started, `parseBdDoltStartOutput` (43) requires exact upstream `Dolt server started (PID NNNN, port NNNN)`. On miss, falls back to `dolt-server.pid` / `dolt-server.port` (113-122) under `.beads/`. Attacker-prepared files combined with non-matching upstream output (intentional or accidental) causes bt to wait for live TCP on attacker-supplied port (124-141), attach as if it were Dolt, issue all SQL against wrong endpoint. SQL credentials are `root@tcp(...)/?...` no password (global_dolt.go:103) — attacker speaking MySQL protocol intercepts everything.

**RE-7 evidence:** Beads is upstream-controlled (gastownhall/beads), 30+ releases in 4 months — success message format isn't stable API. If beads stops writing pid/port files (deemed bd-internal scratch), fallback returns "cannot determine PID/port" — misleading since bd actually succeeded. 10s post-start TCP-dial loop only handles slow startup AFTER parsed port.

**Recommendation:** (1) After connect on file-fallback port, send `SELECT @@version`; require Dolt version string before treating as authoritative. (2) Validate `lookPath("bd")` returns absolute path outside cwd; warn if it resolves under pwd (PATH-hijack defense). (3) Treat parse failure as hard error rather than file-fallback for "started" path. (4) Coordinate upstream for `bd dolt start --json` or `bd dolt status`.

---

## 22-29. Medium-severity findings (Score 1.32)

### 22. PE-3 — BatchFileStatsExtractor cache is unbounded
**Location:** `pkg/correlation/stream.go:339-405` | **Consensus:** 4/5 (DA challenges hyperbolic framing)
**Evidence:** `cache map[string][]FileChange` (344) written for every SHA in `extractBatchFiles` (400). No eviction, no TTL, no size cap — only explicit `ClearCache()` (517). Long TUI sessions accumulate. Compare to `incrementalGraphStatsCache` in graph.go:877-952 which has LRU + 8-entry cap + 5-min TTL — the bounded pattern exists.
**Recommendation:** Mirror `incrementalGraphStatsCache` pattern: max-size cap + LRU eviction (e.g., 1000 entries).

### 23. PE-5 — Phase 2 metric timeouts leak goroutines
**Location:** `pkg/analysis/graph.go:1629-1747` | **Consensus:** 4/5 (SA abstain) | **Cluster:** Phase 2 (with RE-2, RE-5)
**Evidence:** PageRank/Betweenness/HITS/Cycles each spawn `go func()` racing against `time.NewTimer`. On timeout, function returns but **goroutine continues running gonum to completion**, holding entire graph reference. Repeated re-analysis (each watcher tick) piles up abandoned Phase-2 goroutines. The 500ms timeout describes when caller gives up, not when work stops.
**Recommendation:** Replace timer with `context.WithTimeout`. Have inner algorithm honor cancellation. computePageRank's loop (graph.go:2690) can check `ctx.Err()` per iteration cheaply. Or hard cap on Phase-2 goroutine concurrency.

### 24. PE-6 — `countBlockedBy` is O(n²·d) inside BFS loop
**Location:** `pkg/analysis/graph.go:2604-2618` | **Consensus:** 4/5
**Evidence:** Linear scan of every issue + every dependency. Called inside `GetBlockerChain`'s BFS loop (2576) and entry-point (2519). Chain of length L visiting v unique blockers = O(v·n·d). On 1000-issue graph, 3 deps avg, chain length 20 = ~60,000 dep checks for one `bt show <id>` blocker chain. `stats.InDegree[id]` (graph.go:1588) is exactly this count for blocking edges — but chain code re-derives.
**Recommendation:** Pre-compute `blockedByCount map[string]int` once at Analyzer construction (or reuse stats.InDegree if dep-type filter matches). O(n·d) one-time, then O(1). Memoize repeated `GetOpenBlockers` calls in the BFS.

### 25. PE-7 — Live-reload debouncer drops trailing event in write burst (wrong direction)
**Location:** `pkg/export/livereload.go:115-122` | **Consensus:** 4/5 (SA confirms with severity bump suggestion)
**Evidence:** Leading-edge skip pattern: first event in burst fires immediately, subsequent events within 200ms window SKIPPED. For typical export-rebuild writing index.html then several .css/.js files within ~50ms, browser receives ONE reload from FIRST file (often before rebuild complete) and ZERO for trailing settled state. Browser reloads against partially-written assets. `pkg/watcher/debouncer.go:36-65` correctly implements trailing-edge debouncing — right primitive exists, just not used here.
**Recommendation:** Replace leading-edge skip with trailing-edge `Debouncer` from pkg/watcher/debouncer.go. Fire `notifyClients` only after `debounce` elapses with no further events. Add 100ms grace delay so partially-flushed bundles aren't served.

### 26. RE-5 — Panicking Phase 2 metric goroutines silently fall through to timeout fallback
**Location:** `pkg/analysis/graph.go:1633-1640, 1669-1674, 1723-1728, 1785-1790` | **Consensus:** 4/5 | **Cluster:** Phase 2
**Evidence:** Each metric goroutine defers `recover()` with EMPTY body (1635-1637 verbatim: `if r := recover(); r != nil { // Panic -> implicitly causes timeout in parent }`). On panic, goroutine recovers silently and exits without sending on done channel. Parent select blocks until timeout fires. Real algorithmic bugs hidden behind benign-looking timeout flag — anti-CLAUDE.md "fail loudly."
**Recommendation:** Inside each recover, send sentinel (atomic flag, panicCh consumed in parent select) so parent distinguishes panic from timeout. Log panic with `debug.Stack()`. Mark `profile.*Panic=true` in MetricStatus.Reason.

### 27. RE-6 — All git subprocess invocations run without context/timeout
**Location:** `pkg/loader/git.go:110, 156, 167, 227, 325, 381`; `pkg/correlation/stream.go:123, 160, 417` | **Consensus:** 4/5
**Evidence:** Every git invocation in pkg/loader/git.go uses `exec.Command` (no ctx). Same in pkg/correlation/stream.go. None pass `exec.CommandContext` with timeout. Git hangs (corrupt repo, .git/index.lock, network FS stall, slow GPG verification) blocks bt forever. TUI freezes; robot mode hangs agent past any timeout. Compare doltctl.go:103 which correctly uses `exec.CommandContext` with 30s. `git log -p --follow` (stream.go:160) can stream tens of MB hitting scanner 10MB max line buffer.
**Recommendation:** Pass through `context.Context` to all loader/correlation entry points. Use `exec.CommandContext` with per-call timeout (15-30s one-shot, longer streaming). For streaming reads, ctx-aware io.Copy with deadline. Add global `BT_GIT_TIMEOUT` env var.

### 28. DA-6 — Tutorial and TUI render code still references `bv-*` (beads_viewer-era) bead IDs as live comments
**Location:** `pkg/ui/tutorial.go:20,41,44,81,132,183,464,553,715,720`; `pkg/ui/board_test.go:747-748` + many | **Consensus:** 4/5 | **Cluster:** Doc-drift (also AR-7).
**Evidence:** Project identity is clear (memory + AGENTS.md:67-68): "Project: beadstui (fork of beads_viewer)... Binary: bt." Tutorial code still has `bv-wdsd`, `bv-lb0h` references. board_test.go:747-748 seeds example IDs `bv-abc`/`bv-def` for board test — leak into golden snapshots and screenshots. ADR-002 Stream 4 tracks bt-t82t Phase 4 still OPEN. bt-72l8 audit dated 2026-04-29 but code hasn't caught up. `.beads/metadata.json:dolt_database="bt"` — bv project no longer exists in this beads database. Agent-experience: `bv-wdsd` follows to dead bead reference.
**Recommendation:** Either close bt-t82t by sweeping bv-* refs in non-archive code paths, OR explicitly downgrade bt-t82t and accept refs as historical attribution. Don't leave both open and untouched. Quickest sweep: `bv-` in pkg/ui/ and pkg/baseline/ only.

### 29. DA-5 — Release pipeline `workflow_dispatch` defaults `snapshot=true`
**Location:** `.github/workflows/release.yml:13-17,44` | **Consensus:** 3/5 (SA, PE abstain)
**Evidence:** `workflow_dispatch: inputs: snapshot: type: boolean default: true`, expanded to `args: release --clean ${{ inputs.snapshot && '--snapshot' || '' }}`. Manual GitHub Actions dispatch ALWAYS produces snapshot (no GitHub Release created) unless operator flips toggle. Combined with project memory ("Manual release: push git tag, goreleaser builds"): only path to real release is `git push origin v*` — no manual UI escape hatch produces release. If tag-push fails, operator's reflex is "re-run from Actions UI" — silent snapshot. ADR-002 Stream 9 says "DONE — pre-tag gates cleared" but never tested manual-dispatch real-release path.
**Recommendation:** Either (a) flip `snapshot: false` so manual dispatch is dangerous-by-default but fixable, with separate `snapshot-test` workflow; or (b) add release runbook documenting the snapshot-default trap. Update MEMORY.md "Manual release" line.

---

## 30-37. Lower-priority findings (Score 1.16-1.24)

### 30. AR-1 — pkg/ui Model is a god object
**Location:** `pkg/ui/model.go:464-642` + 16 model_*.go files (165 method receivers) | **Severity:** MEDIUM (revised from HIGH) | **Consensus:** 3/5
**Recommendation:** Continue bt-oim6/bt-if3w.1 component-extraction pattern. Group: AlertsModel, SemanticSearchState, LabelDashboardState, TimeTravelState. Target: ~10 field groups + router, not 50+ flat fields.

### 31. AR-5 — `pkg/analysis/graph.go` is 2,880 LOC with 111 functions
**Location:** `pkg/analysis/graph.go` | **Severity:** MEDIUM | **Consensus:** 3/5
**Recommendation:** Split per-algorithm: `pagerank.go`, `eigenvector.go`, `kcore.go`. Move rank helpers to `ranks.go`. Move Phase 2 orchestration to `phase2.go`. Delete deprecated map-copy methods (CLAUDE.md rule #6).

### 32. RE-8 — Stale lock takeover races on Windows (reliability framing)
**Location:** `pkg/instance/lock.go:166-245` | **Severity:** MEDIUM | **Confidence:** MEDIUM | **Consensus:** 4/5
See finding 19 cluster — same root cause as SA-3. Cross-process race + recycled-PID detection issue.

### 33. PE-2 — UNION ALL across all databases scales linearly with workspace count
**Location:** `internal/datasource/global_dolt.go:300-351` | **Severity:** MEDIUM (revised from HIGH after DA challenge) | **Consensus:** 3/5
**Evidence:** Single concatenated UNION ALL query, no per-DB timeout. For N databases: O(N) plan, sequential exec inside Dolt. One slow DB stalls entire UNION; one disconnect aborts everything. Compare `LoadIssuesAsOf` which deliberately splits per-DB.
**Recommendation:** Per-DB queries in parallel `errgroup` with per-DB `context.WithTimeout`, merge in Go (mirror LoadIssuesAsOf shape but concurrent). Or add `readTimeout=Ns` to DSN. Raise SetMaxOpenConns past 5. Bench with N in {1, 5, 20, 50}.

### 34. SA-5 — Single-quote escaping in cross-database UNION ALL queries depends on dbName validation
**Location:** `internal/datasource/global_dolt.go:436, 455, 472, 490, 507, 526-527`; helpers 807-816 | **Severity:** MEDIUM | **Confidence:** MEDIUM | **Consensus:** 3/5
**Evidence:** `escapeSQLString()` doubles `'` (correct for default sql_mode). Risks: (1) MySQL `NO_BACKSLASH_ESCAPES` default-off — dbName containing `\'` could close literal in older sql_modes. (2) `tsStr` interpolation (line 437) — if user-supplied as-of value reaches without re-formatting, same escaping applies. (3) dbName trust boundary: from `EnumerateDatabases` (information_schema) — controlled by whoever can `bd init` on shared Dolt server. Multi-tenant Dolt server with attacker naming DB `evil'); DROP TABLE issues;-- ` = injection.
**Recommendation:** Use `?` placeholders for dbName in string-literal positions. Identifier positions (after FROM) keep backtickQuote but assert in EnumerateDatabases that names exclude `\x00`, `\n`, `\r`, `` ` ``. `SET sql_mode='NO_BACKSLASH_ESCAPES,ANSI_QUOTES'` on connection open.

### 35. SA-8 — Supply-chain risk: pseudo-version dependency on personal author's repo
**Location:** `go.mod` | **Severity:** MEDIUM | **Confidence:** MEDIUM | **Consensus:** 3/5
**Evidence:** `Dicklesworthstone/toon-go v0.0.0-20260124...` is Go pseudo-version (untagged commit) from beads_viewer original author's personal GitHub. Single-author, no SemVer, no changelog. `git.sr.ht/~sbinet/gg v0.7.0` on sourcehut. `ajstarks/svgo` no recent dated tag. vendor/ committed (good for determinism) but no automatic CI gate verifies vendor/ matches go.sum on every PR. Malicious `go mod vendor` could backdoor any function. bt ships precompiled binaries via `bt update` — backdoor in any dep becomes user-installed code.
**Recommendation:** Pin `Dicklesworthstone/toon-go` to tagged release if exists; otherwise vendor a frozen copy under `pkg/toon/` and stop tracking upstream. CI step: `go mod verify` and `go mod vendor && git diff --exit-code -- vendor/`. Add `govulncheck ./...` to CI. Document supply-chain posture in SECURITY.md.

---

## 38. AR-7 — Legacy `bv-` sigils embedded in 98 first-party files

**Severity:** LOW  **Confidence:** HIGH  **Consensus:** 3/5 (SA, PE abstain)  **Score: 0.84**
**Cluster:** Doc-drift cluster (with DA-6 — DA-6 is subset).
**Location:** 1,332 occurrences across 98 first-party files. Hot spots: pkg/ui/styles.css (180), pkg/ui/board.go (71), pkg/ui/history.go (92), pkg/ui/model.go (36), pkg/correlation/network_test.go (89), pkg/correlation/related_test.go (91)

**Evidence:** Rename was bv → bt (binary "bt", module "beadstui"). bt-t82t (ADR-002 Stream 4) tracks "stale `bv-` references in live tutorial/history/UI text" as P2 cleanup. Sigils appear in code identifiers, comment IDs (`focusTree // (bv-gllx)` in model.go:57; `SortMode bv-3ita` in model.go:420), enum docstrings, even "current bead-pattern marker" in graph.go:251 (`bv-4jfr` annotates a CURRENT pattern, not historical). IDs no longer resolvable via `bd show <id>` because beads' prefix is now `bt-`. New maintainers and AI agents cannot trace these references back to decisions.

**Recommendation:** Reframe bt-t82t (or file sibling) to include source-code sigil sweep: every `bv-XXXX` in a Go comment is unresolvable bead reference. Either (a) translate to current bead IDs by lookup in beads history, (b) drop parenthetical entirely if bead is closed and historical context isn't load-bearing, (c) prefix with "originally tracked in bv-XXXX, see archive" if historical link matters.

---

## Minority finding (preserved per anti-herd protocol)

### AR-4 — `cmd/bt` directly imports `pkg/ui`, coupling the CLI binary to the entire 38.9k-LOC TUI surface

**Severity:** MEDIUM  **Confidence:** HIGH  **Consensus:** 1/5 (AR only — others abstain or dispute on relevance)  **Score: 1.08**
**Location:** `cmd/bt/root.go:37` imports `pkg/ui`; `cmd/bt/root.go:285-on` `runRootTUI` constructs `ui.NewModel`

**Evidence:** Only root.go imports pkg/ui — but every `bt robot <subcmd>` invocation pulls pkg/ui into binary's dependency closure (38.9k LOC, 58 files, all of charm.land/* + atotto/clipboard transitively). For agent automation running robot subcommands, dead weight at link time. No `cmd/bt-robot` or build tag separating TUI from CLI. The architectural decision "bt is single binary doing both" was never explicitly made in ADR-001/002/003.

**Recommendation:** Either (a) accept single-binary model explicitly in an ADR and document pkg/ui is unconditionally linked; or (b) wire pkg/ui behind `tui` build tag and provide `bt-min` (or `bt --no-tui`). Option (b) is overkill if agents always have terminals; option (a) at least removes the implicit "we just kept it that way" status.

**Why preserved:** Per anti-herd protocol, minority findings are "frequently right on non-obvious issues that majorities anchor away from." This finding may matter more than the majority recognizes if/when bt grows into agent-only deployment scenarios (CI runners, headless build agents). Worth a backlog bead for future re-evaluation.

---

## Summary statistics

| Status | Count |
|---|---|
| **Confirmed** (≥3/5 personas) | 36 |
| **Probable** (2/5) | 0 |
| **Minority** (1/5, preserved) | 1 (AR-4) |
| **Discarded** (0/5) | 0 |
| **Total** | 38 (within 40-finding budget) |

| Severity | Count |
|---|---|
| CRITICAL | 2 |
| HIGH | 11 |
| MEDIUM | 24 |
| LOW | 1 |

| Cluster | Findings |
|---|---|
| SQLite legacy | AR-2, DA-3 |
| Sprint feature stale | AR-6, DA-2 |
| Phase 2 timeout machinery | PE-5, RE-2, RE-5 |
| Self-update integrity | RE-4, SA-6 |
| bd dolt start regex | SA-7, RE-7 |
| Stale-lock Windows races | SA-3, RE-8 |
| Doc/code drift | DA-1, DA-3, DA-4, DA-6, AR-7 |
| Live-reload (export preview) | SA-4, PE-7 |
| Git shell-out hardening | SA-2, RE-6 |
