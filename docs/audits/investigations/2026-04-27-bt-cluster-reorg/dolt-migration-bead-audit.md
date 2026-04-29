# Dolt-Migration Bead Audit (bt-mhcv)

**Date**: 2026-04-27
**Scope**: All 169 open bt beads, classified against the post-v0.56.1 Dolt-only beads architecture.
**Method**: 14 parallel triage subagents (one per `area:*` bucket; `area:tui` split into 3 chunks). Each agent classified beads in its bucket against the AGENTS.md "Beads architecture awareness" rubric. Per-bucket outputs at `docs/audit/triage/<bucket>.md`.

## TL;DR

**163 GREEN / 6 YELLOW / 0 RED** out of 169 open beads. The backlog is in much better shape than the bead's worst-case framing assumed — the late-April cleanup arc (bt-uh3c brainstorm + 2026-04-25 data-source survey + AGENTS.md awareness section + ADR-003 proposal) caught most of the rot before this audit ran. No bead requires close-as-superseded for stale-architecture reasons. One non-architectural duplicate (bt-x685 ↔ bt-tq60) was incidentally surfaced.

## Per-bucket totals

| Bucket | Count | GREEN | YELLOW | RED |
|---|---:|---:|---:|---:|
| analysis | 14 | 13 | 1 | 0 |
| bql | 5 | 5 | 0 | 0 |
| cli | 22 | 21 | 1 | 0 |
| correlation | 5 | 5 | 0 | 0 |
| data | 11 | 9 | 2 | 0 |
| docs | 6 | 6 | 0 | 0 |
| export | 2 | 2 | 0 | 0 |
| infra | 20 | 19 | 1 | 0 |
| no-area | 3 | 3 | 0 | 0 |
| search | 3 | 3 | 0 | 0 |
| tests | 6 | 6 | 0 | 0 |
| tui-1 | 24 | 24 | 0 | 0 |
| tui-2 | 24 | 23 | 1 | 0 |
| tui-3 | 24 | 24 | 0 | 0 |
| **TOTAL** | **169** | **163** | **6** | **0** |

## YELLOW beads — actions

| Bead | Bucket | Issue | Action taken (Phase B) |
|---|---|---|---|
| **bt-2cvx** | data | Scope still lists "new Dolt columns vs metadata JSON" as an open question; resolved by bd-34v Phase 1a (session columns first-class on issues table). | Corrective comment grounding in bd-34v; point hydration path at bt-5hl9; reframe as TUI/search display work. |
| **bt-ldq4** | analysis | Quotes `Issue.Metadata["created_by_session"]` as the bridge data source; the "transparent swap later" framing presumes a future migration that has now landed. | Corrective comment: read direct columns per bd-34v; the swap IS bt-5hl9. |
| **bt-v0mq** | infra | Frames bug around `.beads/issues.jsonl` as system-of-record; in Dolt-only era this file is opt-in export, not authoritative. | Corrective comment: clarify JSONL is opt-in; auto-export decision belongs in bt-uahv before fixing the symptom. |
| **bt-if3w.1** | tui-2 | Sprint-view extraction is gated on bt-z5jj's rebuild-vs-retire decision; doing the work first risks rework. | Corrective comment: add bt-z5jj as a blocker; sequence after that decision. |
| **bt-x685** | cli | NOT stale-architecture — duplicate of bt-tq60 (same bug, same skipped test, same proposed fix). | Close-as-duplicate with paper trail to bt-tq60. |
| bt-5hl9 | data | Pre-classified landmark; flagged YELLOW for paper-trail consistency but already grounded post bd-34v Phase 1a. | None — already aligned. |

## What we did NOT find (and what that means)

- **No RED beads.** No bead was so architecturally stale that it needed reshape + close-as-superseded. Every bead in the backlog either:
  - Is post-Dolt-only (recent work; correctly framed), OR
  - Is a pre-Dolt landmark whose stale framing was already grounded by an earlier comment / brainstorm pass, OR
  - Touches a layer (TUI, BQL, search, tests, docs) that is genuinely storage-agnostic.

- **No new replacement beads needed.** The replacement landmarks (bt-08sh, bt-z5jj, bt-uahv, bt-5hl9, bt-05zt, bt-3ltq) already cover the architectural deltas the audit was scoped to find. The triage tables surface a handful of cluster patterns that may merit consolidation in future sessions, but these are sequencing observations, not stale-architecture findings.

## Cross-bucket observations

These were noted by the triage agents in their per-bucket "notes" sections. Not actionable for this audit; capturing for future sessions:

1. **History-view cluster** (bt-ezk8, bt-nyjj, bt-npnh — all tui-2 GREEN): all three are surface symptoms of the per-cwd-git-repo / no-global-mode gap that bt-08sh + bt-3ltq own at the data layer. Worth sequencing the band-aids after the structural fix.
2. **Modal-rendering cluster** (bt-dp41, bt-menk, bt-lin9 — tui-2 GREEN): all point at the same OverlayCenter vs lipgloss.Place inconsistency. A single fix probably resolves all three.
3. **Mouse-support cluster** (bt-fbx6, bt-km6d, bt-ks0w — tui-2 GREEN): share a chrome-measurement / bubblezone design question; the design spike is shared.
4. **Robot-mode contract cluster** (bt-70cd, bt-ah53, bt-tq60 — cli GREEN): coherent group around stdout/stderr/exit invariants under robot mode. bt-ah53 is the meta-fix.
5. **Pairs/refs v2 ecosystem** (bt-92ic, bt-dhqw, bt-9prn, bt-xgba — cli GREEN): all extend or clean up the v2 work; clean Dolt-era framing.
6. **Security cluster (2026-04-27 STRIDE/OWASP)** spans buckets (bt-7712 / bt-br02 / bt-zq9k in cli; bt-2gvf / bt-dtuv / bt-xpv9 in data; bt-eg2c / bt-m8fo / bt-x2ap in infra; bt-kc2t in correlation). All trace back to bt-6cdi. Adjacent code in `internal/datasource/` could be batched.
7. **bt-689s ↔ bt-thpq** (both data GREEN): both are Dolt-system-table investigations (events table vs dolt_log/dolt_diff). A single recon could answer both.
8. **bt-search consolidation** (bt-hazr ↔ bt-ox4a — search GREEN): possible consolidation flagged.
9. **bt-x685 ↔ bt-tq60** (cli): incidental duplicate — handled in Phase B.

## Forward-prevention status (Phase C, partial credit from 2026-04-25)

These were already in place before this session:
- **AGENTS.md** has a "Beads architecture awareness (verified 2026-04-25)" section with the current beads architecture, stale-assumption checklist, and links to all related beads.
- **MEMORY.md** has a "Dolt-migration awareness (bt blast radius)" pointer to `project_dolt_migration_awareness.md`.

This audit doc + the pre-existing artifacts complete Phase C.

## Methodology notes (for future audits)

- **Per-area parallelism worked well**: 14 buckets, ~25 beads per bucket max, mean wall time ~60 sec per agent. Total parallel wall time ~2 minutes for 169 beads.
- **JSON dossiers beat `bd show`**: each agent got a JSON file with full bead descriptions; they did NOT call `bd show` per bead. Cheap, fast, no Dolt-server thrash.
- **Pre-classified landmarks reduced noise**: telling agents "these 9 beads are GREEN, don't re-classify" prevented spurious YELLOW votes on the bt-mhcv / bt-08sh / bt-z5jj / bt-uahv decision-capture beads.
- **Bias toward GREEN was correct**: the rubric explicitly told agents "for ambiguous cases lean toward GREEN; the bar for YELLOW/RED is a clear stale assumption you can quote." Without that, YELLOW would have been over-applied to e.g. TUI beads that mention "issues.jsonl" in passing without depending on it.
- **Duplicate detection emerged for free**: bt-x685 ↔ bt-tq60 wasn't on the audit's stated agenda but the cli agent caught it because it had read both descriptions in the same pass. Worth keeping in mind: a per-area read-and-summarize pass also surfaces duplication and sequencing concerns even when the primary mission is something else.

## Artifacts

- Per-bucket triage tables: `docs/audit/triage/{analysis,bql,cli,correlation,data,docs,export,infra,no-area,search,tests,tui-1,tui-2,tui-3}.md`
- This audit doc: `docs/audit/2026-04-27-dolt-migration-bead-audit.md`
- Source dossiers (ephemeral; can be deleted): `c:/tmp/audit/*.json` + `c:/tmp/audit/RUBRIC.md`
