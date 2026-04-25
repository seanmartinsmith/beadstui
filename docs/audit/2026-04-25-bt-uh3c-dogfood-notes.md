# bt-uh3c brainstorm dogfood log — 2026-04-25

Live consumer scenarios run against bt during the bt-uh3c brainstorm session
(cc session 143fbd31-5878-4067-9be8-5538abca7e7c). Each scenario tested a
real cross-project consumer use case and captured concrete findings (F1-F13).

These findings drove:
- bt-111w fix (F11)
- bt-vhn2 close-as-superseded (F2-F10 reshape)
- bt-08sh, bt-z5jj, bt-uahv new beads (architectural sub-stream)
- bt-mhcv P0 audit (systematic backlog audit)
- AGENTS.md "Beads architecture awareness" section

Preserved here from /tmp because bt-mhcv (Phase A inventory) will reference
these scenarios when classifying beads, and audit work benefits from the
raw scenario logs not just the synthesized findings.

---

=== S1: Resolve single foreign bead by ID (the canonical cass-uh3c use case) ===

--- bt --global robot list --bql 'id="cass-uh3c"' (the workaround path) ---
stdout bytes: 38130, stderr bytes: 0, exit: 0

--- attempted bt robot get cass-uh3c (does it exist?) ---
exit: 0, stderr first line: 

=== S1 findings ===
F1 (covered by bt-uh3c item 5): BQL 'id="cass-uh3c"' silently no-ops. Returns 38KB unfiltered list. No envelope echo of bql, no error.
F2 (NEW GAP): unknown 'bt robot get' subcommand prints help text to STDOUT, exits 0. Robot mode contract says stdout=structured-data only. A consumer piping to jq sees garbage. Not in bt-uh3c.

=== S2: Get full event history of a foreign bead ===
--- bt --global robot history cass-uh3c (commit-correlations, not what we want) ---
exit: 1, stdout: 0, stderr: 67
first 300 chars stdout: 

--- bd history cass-uh3c --json (does this even work from bt repo?) ---
exit: 0, stdout: 37, stderr first line: 
first 300 chars stdout: No history found for issue cass-uh3c

=== Recon agent A findings (parallel beads in flight) ===
- bt-5hl9 (P2): hydrate created_by_session/claimed_by_session from Dolt. DUPLICATES bt-uh3c item 3 — drop from this design.
- bt-we7z (P3): robot list --ids <csv>. PARTIAL OVERLAP with item 1 — different shape (filtered array vs single object). For trace tooling that resolves MANY IDs, --ids may be the better primitive.
- bt-dhqw (P3): robot pair show <suffix>. CONVENTION PRECEDENT — single-thing-show uses verb 'show' not 'get'. bt-uh3c item 1 should be `bt robot show <id>` to match.
- bt-4jyd, bt-a3sb, bt-yqh0, bt-92ic, bt-lt2h: adjacent, not overlapping.

=== S2 findings ===
F3 (covered by bt-uh3c item 2): bt --global robot history cass-uh3c errors with 'no beads file found in <bt repo>/.beads/' — --global is NOT respected by the history subcommand. Confirms history is local-scope and probably commit-correlation only, not bead-event timeline.
F4 (NEW GAP, beads side): bd history cass-uh3c --json returned plain text 'No history found for issue cass-uh3c' — it ignored --json and emitted prose. Upstream bd bug, but worth filing as a paired bead in beads project so it doesn't trip future cross-project consumers.

=== S3: Get comments on a foreign bead ===
--- bt --global robot ??? get comments on cass-uh3c (does any subcommand do this?) ---
no comment-related subcommand in bt robot
--- bd comments cass-uh3c (from bt repo, no --global on bd) ---
exit: 1, stdout: 0, stderr: Error: resolving cass-uh3c: no issue found matching "cass-uh3c"
stdout sample: 

=== S4: Walk dep tree of a foreign bead ===
bt --global robot related cass-uh3c: exit 1, stdout 0, stderr: Error: no beads file found in C:\Users\sms\System\tools\bt/.beads/
first 300 chars stdout: 

=== S5: stderr/stdout cleanliness across the entire robot subcommand surface ===
  list: exit=0 stdout=2227B stderr=0B clean
  triage: exit=1 stdout=0B stderr=22B STDERR-NOT-EMPTY: unknown flag: --limit
  next: exit=1 stdout=0B stderr=22B STDERR-NOT-EMPTY: unknown flag: --limit
  plan: exit=1 stdout=0B stderr=22B STDERR-NOT-EMPTY: unknown flag: --limit
  portfolio: exit=1 stdout=0B stderr=22B STDERR-NOT-EMPTY: unknown flag: --limit
  pairs: exit=1 stdout=0B stderr=22B STDERR-NOT-EMPTY: unknown flag: --limit
  refs: exit=1 stdout=0B stderr=22B STDERR-NOT-EMPTY: unknown flag: --limit
  related: exit=1 stdout=0B stderr=22B STDERR-NOT-EMPTY: unknown flag: --limit

=== S3 findings ===
F5 (NEW GAP, not in bt-uh3c): NO bt robot subcommand exists for retrieving comments. Consumers cannot get foreign-bead comments through any path today. bt-uh3c item 1 says 'include comments' in the get response — but that's bundled. A standalone `bt robot comments <id> --global` may be valuable too for trace tools that paginate.
F6 (covered by bt-uh3c item 1): bd comments cass-uh3c errors 'no issue found matching cass-uh3c' from bt repo — bd is hard-scoped local. Confirms bt is the only path for cross-project comment retrieval.

=== S4 findings ===
F7 (covered by bt-uh3c item 2 implicitly): bt --global robot related cass-uh3c errors 'no beads file found' — multiple subcommands ignore --global routing for single-ID lookups. This is a SYSTEMIC pattern: history, related, and likely others. Item 2's fix probably has to land alongside a broader --global ID-routing fix that touches multiple subcommands. Worth flagging in the design.

=== S5 findings (stdio cleanliness across robot surface) ===
F8 (NEW GAP, not in bt-uh3c): unknown SUBCOMMAND ('bt robot get cass-uh3c') prints help to STDOUT exits 0. Cobra default. Robot mode contract: stdout=structured-data only. Need to override unknown-subcommand handling to emit error JSON to stderr and exit 1. PARTIAL precedent: unknown FLAGS already correctly emit to stderr exit 1 (good). The inconsistency is in the unknown-subcommand path.
F9 (process-of-elimination): On `--limit` rejected by triage/next/plan/portfolio/pairs/refs/related — these subcommands either don't accept --limit or fail differently. Not a robot-mode-cleanliness issue (stderr/exit are correct). Just an ergonomic/discoverability gap. Consumers need a way to know which flags are valid where. Maybe `bt robot schema --command <name>`? File separately if it doesn't exist.
F10 (CONFIRMS bt-uh3c item 4 is mostly already done): None of the tested robot subcommands (list, triage, next, plan, portfolio, pairs, refs, related) leaked log output to stdout. Item 4's discovery-log claim might have been a transient bug already fixed (Stream 1 says 2026-04-10), or specific to a subcommand path not yet tested. Need code recon (Agent C) to confirm.

=== Agent B (cass transcript recon) findings ===
- d53f9292 transcript CONFIRMS: log line leak was real on that day; consumer had to [0] out of issues array; created_by_session and claimed_by_session never appeared in samples (consistent with bt-uh3c item 3 ambiguity).
- bt-uh3c is the FIRST proposal of robot get — net-new, not a re-tread. Worth getting right because no calcified alternatives exist.
- cass-uh3c is the FIRST third-party consumer of bt --global. The shape we pick now sets the precedent for future consumers.
- Trace consumers join SESSION STAMPS per event onto cass session index. Item 2 timeline must include {timestamp, action, actor, session_id} per event — that's the load-bearing payload.

=== S6: Does bt-we7z (--ids csv) already ship? Could it replace item 1 for trace tooling? ===
exit: 1, stdout: 0, stderr: unknown flag: --ids

=== S7: --shape=full reveals more fields. Does it include comments/history/all session columns? ===
shape=full exit: 0, stdout: 497859, stderr: 
  contains 'comments' key: 1
  contains 'history' key: 0
0
  contains 'created_by_session': 0
0
  contains 'claimed_by_session': 0
0
  contains 'closed_by_session': 1

=== S8: Pick a bead known to have closed_by_session populated. Does compact.v1 include it? Does full? ===
mkt-ci1 compact stdout: 38130, stderr: 
mkt-ci1 full stdout: 497859, stderr: 
  compact contains 'closed_by_session': 1
  compact contains 'created_by_session': 0
  full contains 'closed_by_session': 1
  full contains 'created_by_session': 0

=== Agent C (code recon) findings ===
- CompactIssue (pkg/view/compact_issue.go L45-47): all 3 session fields present with omitempty. created_by_session and claimed_by_session sourced from Metadata JSON blob (bt-mhwy.0), NOT direct columns. closed_by_session is direct. Resolution: item 3 IS just null-elision; mkt-ci1 (created pre-bd-34v) has closed_by_session but no created_by_session because its Metadata blob doesn't carry it.
- No bd shell-out precedent (Dolt-direct for all reads). robot get should query Dolt, not wrap bd.
- --global UNIONs ALL issues at load. No prefix-routing by ID. Single-ID subcommands (related, history, forecast, blocker-chain, impact-network, causality, sprint show) ALL fail with "no beads file found" under --global. Item 1's fix probably belongs alongside a SYSTEMIC --global fix for single-ID subcommands.
- slog goes to stderr by default (not redirected). stdlib log is io.Discard'd. So discovery logs appear on stderr, not stdout. My S5 confirmed: clean stderr in fresh runs, suggesting discovery happens earlier or is now suppressed in the listed paths.
- BQL `id="..."` is parsed and applied AFTER load. But my S1a/S7 evidence shows BQL filter doesn't actually narrow output — count=100 returned for a single-id BQL. NEW BUG: BQL filter not applied in --global path.
- Bead-event timeline is HARD: Dolt has no bead-event log table. To deliver item 2 we'd need (a) dolt_log/dolt_diff_log SQL queries on the issues table (real engineering), (b) wrapper around bd history (which has its own JSON bug per F4), or (c) upstream schema additions.

=== S6 findings ===
F11 (NEW BUG, possibly highest-priority): BQL filter is silently no-op'd in --global path — `bt --global robot list --bql 'id="cass-uh3c"'` returns 100 issues, not 1. Worse than F1 thought: the filter is parsed and (per code recon) supposedly executed, but the output ignores it. Need to confirm if it's --global-specific or all-paths. This breaks the documented workaround for item 1.

=== S7-S8 findings ===
F12 (CONFIRMS item 3 = null elision): mkt-ci1 has closed_by_session in both shapes; created_by_session absent in both. Reason: mkt-ci1's Metadata blob predates bd-34v Phase 1a. Item 3 reduces to schema documentation: explain Metadata-sourced session fields appear iff Metadata blob carries them.
F13 (item 1 alternative): --shape=full envelope already includes 'comments' key per record. So item 1's "include comments" requirement is already met by --shape=full --bql 'id="X"'. The remaining gap is: (a) BQL filter doesn't work (F11); (b) shape is still array-wrapped not single-object; (c) no 'history' key. So shape=full is closer than I thought — fixing F11 plus adding a single-object envelope would deliver most of item 1 without a new subcommand.

=== Cross-check status: F2, F4, F5, F8, F9, F11 are NEW gaps not in bt-uh3c ===
F2: unknown robot subcommand prints help to stdout exit 0 — NEW, file as bt bead
F4: bd history --json emits prose, not JSON — NEW, file as beads (bd-) bead
F5: no bt robot subcommand for comments alone — covered by item 1's bundled response, but standalone may be wanted
F8: same as F2, dedup
F9: --limit accepted by list, rejected by 7+ subcommands; consumers can't discover valid flags per-subcommand — possibly covered by `bt robot schema`?, file as bt bead
F11: BQL filter silently no-ops in --global path — NEW, P2 bug, file as bt bead
