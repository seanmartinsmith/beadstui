---
title: "Bangout arc: ship daily-use wins, then earn back foundation"
type: arc
status: active
date: 2026-04-27
origin: dogfood session 2026-04-27 (search UX audit + workload triage)
bead: none (cross-cutting plan; references many beads)
---

# Bangout arc: ship daily-use wins, then earn back foundation

## Strategic frame

Inverted from a foundation-first plan: **ship the user-facing wins first, then do foundation when it's actually unblocking the next round.** Foundation work that doesn't unblock something visible can wait.

Three principles:

1. **Information-producing work runs in parallel** (worktree subagents) so it doesn't block execution.
2. **Touch each file region once per arc** — sequence so adjacent work co-locates in the same files.
3. **Match motivation to load** — search UX context is hot from the audit; ship while it's in head.

## Pre-flight checks (run before claiming any bead in Phase 1)

```bash
# 1. Confirm bd state (avoid stepping on a stalled in-progress bead from a prior session)
bd list --status=in_progress

# 2. Confirm Dolt server is up and pushable
bd dolt status

# 3. Confirm BD_ACTOR is set correctly in this shell
#    NOTE: BD_ACTOR=sms is set in PowerShell profile (per memory file), NOT in bash.
#    If running bash, export it explicitly OR rely on git user.name fallback (which produces "@sms" anyway).
echo "BD_ACTOR=${BD_ACTOR:-(unset, fallback to git user.name)}"

# 4. Snapshot cross-project state for the session-author chain (Phase 3 prep)
cd ~/System/tools/cass && bd show cass-ynoq | head -30 ; cd -

# 5. Verify nothing is mid-claim in this workspace
bd list --status=in_progress --owner sms
```

If `bd list --status=in_progress` shows beads owned by you that aren't your current task, investigate before claiming new work — that's likely abandoned state from a prior session per AGENTS.md "Session Rules."

## Orientation block

This plan was written after a session that did extensive recon and minor shipping. Net change to ground truth as of 2026-04-27:

### Shipped in originating session
- **bt-jwo3** (closed) — TUI search: comma-separated multi-token OR. Wrapper added in `pkg/ui/id_bucket_filter.go`. Wired at 4 sites. Tests added.
- **bt-treo** (closed) — Detail pane: `/` teleports back to search. Single intercept added in `pkg/ui/model_update_input.go:1269`.
- **bt-uahv** (closed) — Decision: `.beads/` vs `.bt/` data-home spec. Implementation tracked in **bt-v6rw**.

### Decisions deferred (do not work blind)
- **bt-z5jj** (open, `human` label) — Sprint feature: A (retire) vs D (repurpose against molecules). Decision waits on **bt-72l8.1** ghost-features audit results.
- **bt-3suf** (open, `human` label, blocked-by bt-z5jj) — Sprint retire impl. Don't start until z5jj closes.

### Filed in originating session (decisions / brainstorms / audits)
- **bt-72l8.1** (P2, child of bt-72l8) — Ghost-features audit (broad: every `--robot-*` + every TUI mode)
- **bt-72l8.1.1** (P2, child of bt-72l8.1) — TUI-specific ghost audit (per-view completeness pass)
- **bt-krwp** (P2) — Search UX overhaul: cycle Ctrl+S, repurpose H, quoted-exact, status clarity, `[0.00]` threshold, multi-token cap in hybrid
- **bt-ja2y** (P2) — Search defaults reform: pick boot mode + surface why other modes exist
- **bt-gf3d.1** (P2, child of bt-gf3d) — Hotkey/feature audit: which keys deserve top-level binding
- **bt-fd3k** (P3 epic) — TUI settings/config surface
- **bt-6q8c** (P2 — bumped from P3) — TUI labels reform: `view:*` sub-area dimension
- **bt-v7um** (P3) — Detail pane meta: surface Updated + brainstorm rest
- **bt-rbha** (P2) — TUI surface for type=gate + human-labeled beads (sister of bt-mbjg)
- **bt-t8mu** (P3) — Natural-language surface for wisp/molecule grouping
- **bt-54c3** (P3) — Themes: in-TUI picker

### Filed cross-project
- **cass-ynoq** (P2 in cass) — Stable session-ID surface for cross-tool consumers
- **dotfiles-qew** (P3 in .files) — Document session-id-as-author convention in global CLAUDE.md

### Wired
- **bt-8jds** linked as child of **bt-gf3d** (it was always a symptom of the keybind overload epic)
- **bt-mbjg** updated: confirmed default-hide gates, gated on bt-rbha being filed (done)
- **bt-ba9f** updated: do NOT close-supersede; remaining scope is CLI flag `bt --ids=...` + dedicated modal entry point
- **bt-2cvx + bt-5hl9** paired-bead notes added linking to cass-ynoq + dotfiles-qew

### Existing infrastructure to use
- **`docs/audit/keybindings-audit.md`** — keybinding map from 2026-04-23. Stale in spots but the canonical reference. Update as bt-gf3d.1 progresses.
- **bt-gf3d** (P2 epic, open) — Keybinding consistency overhaul. Parent for bt-8jds, bt-gf3d.1, and any keybind-derived follow-ups.
- **bt-72l8** (P1 epic, open) — Jeffrey-era audit. Parent of bt-72l8.1 and bt-72l8.1.1.
- **ADR-002** (`docs/adr/002-stabilize-and-ship.md`) — Spine doc; update stream statuses as work lands.

---

## Phase 1: bug bangouts (1 session, mostly autonomous)

Independent file regions. Sequential within one session is fine; total ~2-3 hours. No worktree overhead needed for sub-30-minute fixes.

### Starter quartet (no design input needed beyond verification)

| Bead | Hurts | Files | Estimated load |
|---|---|---|---|
| **bt-cl2m** (P2 bug) | Background data refresh closes open modals | `pkg/ui/model_update_data.go` | Small. Guard refresh-driven re-renders with `m.activeModal != ModalNone` (see existing usage at `model_update_input.go:1148`); add a helper if not already present. NOTE: `m.modalActive()` does NOT exist — earlier draft of this plan was wrong. |
| **bt-70cd** (P2 bug) | Unknown `bt robot` subcommand prints help to stdout, exits 0 | `cmd/bt/cobra_robot.go`, `cmd/bt/root.go` | Small. Configure cobra to write unknown-command errors to stderr + exit non-zero. Verify `bt robot bogus 2>/dev/null` is empty and `$?` is non-zero. |
| **bt-nyjj** (P2 bug, child of bt-19vp) | History view: red 'git log failed' on cold boot from non-git cwd | `pkg/correlation/`, history view | Small. Detection mechanism: use `git rev-parse --is-inside-work-tree` (exit 0 = inside repo, non-zero = not). On non-zero, silent fallback (no banner). Reserve red banner for actual git-invocation failures (binary missing, permissions, etc.). |
| **bt-foit** (P2 bug) | Undocumented `<>` keybinds + label column alignment break when list pane widened | `pkg/ui/model_keys.go`, delegate.go | Medium. Two parts: (1) document the keybind in help/sidebar, (2) fix the alignment regression. Independent of search UX work — safe to bundle in Phase 1. |

**Why bt-foit replaces bt-8jds in the quartet**: bt-8jds is at `pkg/ui/model_update_input.go:1135` — the SAME file as bt-krwp's search-mode handlers (lines 615-710). Phase 1 + Phase 2 would collide on this file. bt-8jds also benefits from waiting on bt-gf3d.1 audit (per `docs/audit/keybindings-audit.md`, `w`/`W` overload is in structural-recommendation territory; picking a key now risks a second rename). Defer bt-8jds to Phase 5 (post-audit).

**Verification per bug:** run the affected path in TUI, confirm fix.

### Optional Phase 1 extras (if time)

- **bt-mxz9** (P2 bug) — cold boot from non-workspace dir fails. Has design components (what should `bt` do when launched outside any beads project?). Defer if not in mood for design call.

**Mandatory Phase 1 verification before push** (AGENTS.md rule 7):

```bash
go build ./... && go vet ./... && go test ./pkg/ui/
```

All three must be clean before `git push`.

---

## Phase 2: search UX while context is hot (1-2 sessions, NEEDS YOUR INPUT)

The search audit context is in your head. Ship before it decays. Three coordinated beads.

### bt-krwp — Search UX overhaul (the big one)

**Design questions to answer before coding:**

1. **Cycle order on Ctrl+S**: fuzzy → semantic → hybrid → fuzzy, OR fuzzy → hybrid → semantic → fuzzy?
   - Recommendation: fuzzy → hybrid → semantic → fuzzy. Hybrid is more useful than semantic-text-only as a daily mode; cycling to it sooner means fewer keystrokes for the common case.

2. **What does H do when not in hybrid mode?**
   - Option A: Status message "Press Ctrl+S to enter hybrid mode" (no-op)
   - Option B: Auto-jumps to hybrid mode (chains Ctrl+S + H)
   - Option C: Repurpose H entirely as preset cycle, only meaningful when in hybrid mode (today's alt+H behavior)
   - Recommendation: C. Single Ctrl+S cycle removes the dead-corner state; H becomes preset cycle. alt+H goes away. Cleaner key surface.

3. **Quoted-exact syntax**: `"foo bar"` only, or also `=foo` shorthand?
   - Recommendation: quotes only. Covers single and multi-word, no new symbol, natural to users.

4. **Hybrid badge threshold**: hide `[0.00]` below abs(score) < ?
   - Recommendation: 0.05. Tune based on dogfood.

**Files**: `pkg/ui/id_bucket_filter.go`, `pkg/ui/model_update_input.go` (lines 615-710 are the search-mode toggle handlers), `pkg/ui/model_footer.go`, `pkg/ui/delegate.go`, `pkg/ui/semantic_search.go`. Same files I touched for bt-jwo3 — overlap is good.

**File-collision warning**: this work owns `pkg/ui/model_update_input.go` lines 615-710. Do NOT run any keybind work (bt-8jds, bt-gf3d.1 follow-ups) in parallel with this — they'll converge on the same file. Phase 1 quartet was deliberately built without bt-8jds for this reason.

**⚠️ AGENTS.md rule 1 (no deletion without permission)**: removing `alt+h` keybind handler at `model_update_input.go:653-674` counts as code deletion. **Confirm with user before stripping.** If user prefers preserving alt+h as legacy / chord, leave the handler with a no-op or status-message redirect to the new H key.

**Test impact** (do NOT skip — bt-krwp will break tests if not updated):

```bash
grep -nE 'semanticHybridEnabled|semanticSearchEnabled|"alt\+h"|"H"' pkg/ui/*_test.go
grep -nE '"Fuzzy search enabled"|"Semantic search: text-only"|"Hybrid search enabled"' pkg/ui/*_test.go
```

Update or rewrite assertions to match the new 3-state cycle. Known hits include `model_test.go:515` (key sequence enumeration) and any test asserting on the current status messages.

**Verification screenshots already attached to bt-krwp** as a comment with the four concrete bugs the dogfood surfaced.

### bt-ja2y — Search defaults reform (paired with bt-krwp)

**Design questions:**

1. **Boot mode default**: fuzzy always, or hybrid-when-index-exists?
   - Recommendation: hybrid-when-index-exists, fall back to fuzzy with status "Hybrid index not built — using fuzzy. Press Ctrl+S to build." Auto-upgrades over time.

2. **Mode-purpose copy**: where does it live?
   - Recommendation: short status-bar copy on mode change (one sentence, e.g. "Semantic: finds items by meaning, slower but smarter.") + a help overlay section. The first-run nudge can come later (bt-xavk territory).

**Coordinate with bt-krwp**: they can ship together (one PR) OR sequentially with bt-krwp first.

### bt-v7um Part 1 — Updated cell (autonomous, no input)

**Two call sites**, not one — earlier draft was wrong:

1. **Markdown export table header** at `pkg/ui/model_filter.go:845` (`sb.WriteString("| ID | Status | Priority | Author | Assignee | Created |...")`) — add Updated column.
2. **Live TUI list-row delegate** — locate via `grep -nE 'FormatTimeAbs\(item\.CreatedAt\)' pkg/ui/`. The render-side use of `FormatTimeRel(item.UpdatedAt)` already exists around line 960 of `model_filter.go` for *descriptions*, but the row-level cell needs adding separately. Confirm in-TUI before closing.

Format: `FormatTimeRel` ("2d ago") for Updated. Created stays absolute (`FormatTimeAbs`). Ships standalone.

Part 2 (brainstorm rest) waits on bt-2cvx landing for the Reporter cell.

---

## Phase 3: foundation that unblocks visible Phase 4 wins (2-3 sessions, autonomous)

### bt-5hl9 — Hydrate session columns from Dolt

Per CLAUDE.md: beads has shipped first-class `created_by_session`, `claimed_by_session`, `closed_by_session` columns since bd-34v Phase 1a (PR #3401, merged 2026-04-24). bt is still reading from the old metadata blob.

**This is the wedge** that unblocks bt-2cvx + bt-v7um Part 2 + the cross-project session-author work.

Files: `pkg/loader/`, `pkg/model/types.go` (CompactIssue), `internal/dolt/`. Touches the same model layer bt-08sh will need later — useful learning.

**Cross-project coordination**: cass-ynoq (cass-side, P2) defines the contract for how bt should consume session IDs. **Don't start bt-5hl9 cold** — first check whether cass-ynoq has progressed (`cd ~/System/tools/cass && bd show cass-ynoq`).

**Pin the minimum cass-ynoq surface in bt-5hl9 acceptance criteria, not as a runtime check** (architect review finding). If cass-ynoq stalls, the trap is bt-5hl9 silently falling back to reading the metadata blob — exactly the stale assumption flagged in `AGENTS.md` "Beads architecture awareness." Concretely, before starting bt-5hl9, lock these in its acceptance:

- Column names assumed: `created_by_session`, `claimed_by_session`, `closed_by_session` (per bd-34v Phase 1a, PR #3401)
- Type: `TEXT NULL` (session-id format may evolve; bt should not parse beyond opaque-string)
- NULL semantics: NULL means "not set" (no session attribution available); display fallback to `@<author>`
- Read path: direct column read via the Dolt reader, NOT `metadata` JSON blob

If cass-ynoq evolves the format later, that's a separate migration. Pin the assumption now so bt-5hl9 can ship deterministically.

### bt-2cvx — Session author display

After bt-5hl9 lands, the Author cell can show session that filed the bead instead of `@sms`. Files: `pkg/ui/delegate.go`, `pkg/ui/model_filter.go`.

**Convention dependency**: dotfiles-qew (P3) documents the session-id-as-author convention in global CLAUDE.md. Doesn't block bt-2cvx but ideally lands within the same week so the convention is documented and supported in lockstep.

### bt-v7um Part 2 — Detail meta brainstorm (NEEDS YOUR INPUT)

Per-field decisions: Labels inline? Reporter cell (now possible after bt-2cvx)? Due/Estimate/External-ref/Defer/Wisp/Gate as conditional cells? 5 minutes of yes/no per field.

### bt-v6rw — Apply data-layout spec (DEFERRABLE)

**Honest call**: this is invisible to daily use. Move it to Phase 5 unless we're about to start bt-08sh (correlator migration). Decision is already made (bt-uahv closed); not blocking anything visible.

Run only if the right session shape comes up: small, mechanical, atomic. Or fold into the next correlator-touching session.

---

## Phase 4: information-producing audits (parallel dispatch with caveats, autonomous)

These can run as **worktree subagents** while you work on Phase 2/3 in the main session. Pure recon, no judgment calls until output review.

**⚠️ Worktree isolation is filesystem-only — the bd Dolt server is global.** Subagents that file new beads or update existing beads share the same `.beads/dolt/` server as the main session. Implications:

- **bt-72l8.1, bt-72l8.1.1, bt-gf3d.1** (audits that file new follow-up beads): safe to parallel — they create new IDs, no contention.
- **bt-6q8c** (TUI labels reform — bulk-retags ~50 existing `area:tui` beads): MUST run AFTER bt-72l8.1 + bt-72l8.1.1 land, otherwise the new beads filed by those audits land without `view:*` taxonomy and need a second sweep. Sequential, not parallel.

### bt-72l8.1 / bt-72l8.1.1 — Ghost-features audits

bt-72l8.1 is broad (every `--robot-*` + every TUI mode); bt-72l8.1.1 is the TUI-deeper pass. Together they produce:
- A classification doc per surface
- Follow-up beads per gap

Output enables **bt-z5jj decision** (A vs D) plus likely 2-4 other "should we keep this" calls. Run before Phase 5 reframes them as a class decision.

### bt-gf3d.1 — Hotkey/feature audit

Triage every top-level keybind for: frequency, discoverability, reversibility, single-letter scarcity. Per-key Keep/Demote/Document/Retire classification. Updates `docs/audit/keybindings-audit.md`. Feeds bt-gf3d (consistency epic), bt-xavk (help redesign), bt-fd3k (settings).

### bt-6q8c — TUI labels reform (`view:*` taxonomy)

Add `view:*` sub-area dimension (view:list, view:detail, view:search, view:graph, view:history, etc.) to `.beads/conventions/labels.md`. Bulk-retag existing `area:tui` beads. Verify candidate list against actual ViewMode enum + pkg/ui structure.

**Why this matters**: after this lands, you can `bd ready --label view:graph` to bang through graph-view beads as a unit. Combined with `bd mol wisp <name> --var label=view:graph` for ephemeral session grouping.

---

## Phase 5: decisions revisited with audit data in hand

### bt-z5jj — Sprint A vs D (NEEDS YOUR INPUT after Phase 4)

If bt-72l8.1 surfaces 3+ ghost features, batch-retire (Option A across the class). If sprint stands alone with a salvageable display layer, repurpose against molecules (D). Decision goes in close_reason.

**Blast radius is wider than just bt-3suf** (architect review finding): per ADR-002 Stream 1 (`docs/adr/002-stabilize-and-ship.md` line 56), `sprint show` is one of the correlator/sprint-bound subcommands tied to ADR-003's Phase 2 collapse. If bt-z5jj sits open across multiple sessions, it blocks any further data-source abstraction work touching the sprint loader. Resolve before starting Phase 5 of ADR-003 work.

**Reversibility check** (engineer review finding): nothing in Phases 1-4 forecloses option A or D. Phase 1 quartet doesn't touch sprint code. Phase 2 search work is independent. Phase 3 session-author work doesn't read sprint state. Phase 4 audits are read-only against sprint code. Decision can land cleanly post-audit either direction.

### bt-3suf — auto-action on z5jj decision

If A: claim and execute (it's the impl bead). If D: close-supersede; file new D-shaped impl bead.

### bt-rbha — TUI gates/human surface (sister of bt-mbjg)

Three options sketched (dedicated view / inline filter toggle / notifications-tab integration). Pick one based on which other TUI work is in flight.

### bt-mbjg — Default-hide gates (close once bt-rbha ships)

Implementation is small (5-10 lines in list filter). Gated on bt-rbha so hide-without-surface doesn't create invisibility.

---

## Parking lot — defer until something forces them

| Bead | Why parked |
|---|---|
| **bt-08sh** | Correlator Dolt migration. Bigger lift. Needs bt-v6rw to give it a `.bt/` target. **2nd-order risk**: any Phase 1-3 bead touching `pkg/correlation/` (e.g. bt-nyjj history, future related/causality work) renders against pre-Dolt assumptions until bt-08sh lands. Revisit if Phase 1-3 surfaces correlator bugs the deferral can't absorb. |
| **bt-fd3k** | Settings epic. Too many configurables in flux until search lands. |
| **bt-54c3** | Themes. Pure cosmetic. Wait for settings persistence. |
| **bt-d5wr** | Footer redesign. Needs design judgment session. |
| **bt-6cdi** | Security epic. Separate workstream. |
| **bt-t8mu** | Natural-language wisp/mol surface. Brainstorm; runs alongside bt-fd3k. |
| **bt-94a7** | Broad upstream audit. After Phase 4 audits land. |
| **All P1 product epics** (bt-53du, bt-19vp, bt-ushd) | Leave as parents that absorb children from above work. |
| **Cross-project / global mode work** (bt-ssk7, bt-dcby, bt-3ltq) | Separate workstream arc; do not interleave. |

---

## Cross-cutting

### Parallelization

**True parallel candidates** (different files, no shared state, dispatch-able as worktree subagents):
- Phase 1 bug batch — bt-cl2m (`model_update_data.go`), bt-70cd (`cmd/bt/cobra_robot.go`), bt-nyjj (`pkg/correlation/`), bt-foit (`model_keys.go`) — all different files. Safe to parallel.
- Phase 4 audits **except bt-6q8c** (bt-72l8.1, bt-72l8.1.1, bt-gf3d.1) — pure recon, file new beads only, no contention. **bt-6q8c must run AFTER these** (per Phase 4 sequencing note — re-tags would miss new audit-filed beads otherwise).

**Looks parallel but watch for shallow overlap**:
- bt-krwp + bt-v7um Part 1 — bt-krwp touches `pkg/ui/delegate.go` (the `[0.00]` badge threshold) AND bt-v7um touches the row-render path. Same file, different concerns. Safe ONLY if bt-v7um Part 1 ships first (smaller diff, faster), then bt-krwp rebases. Don't dispatch concurrently.

**Strictly sequential** (each unblocks next):
- cass-ynoq → bt-5hl9 → bt-2cvx → bt-v7um Part 2
- bt-72l8.1 → bt-z5jj decision → bt-3suf disposition
- bt-krwp ships → A/B test → tune → bt-ja2y default-mode decision

### User-input touchpoints

Total ~10-15 micro-decisions across the arc, each ~30 seconds:

- bt-krwp: 4 design calls (cycle order, H semantics, quote syntax, threshold)
- bt-ja2y: 2 design calls (boot default, copy location)
- bt-v7um Part 2: per-field judgment (~6-8 yes/no)
- bt-z5jj: 1 strategic call after audit
- bt-rbha: 1 option pick (A/B/C)
- All bug verifications: eyeball after fix

### Cross-project coordination

Three-way chain for the session-author workstream:

```
cass-ynoq (data contract)
    → bt-5hl9 (hydrate session columns)
        → bt-2cvx (display in detail pane)
            → bt-v7um Part 2 (Reporter cell)
                
dotfiles-qew (CLAUDE.md convention) — runs alongside, lands together
```

When working any of these, glance at the others' state with `cd <repo> && bd show <id>` first.

---

## Per-phase done checklists

### Phase 1 done when
- [ ] **`go build ./... && go vet ./... && go test ./pkg/ui/` all green** (AGENTS.md rule 7 — mandatory before push)
- [ ] bt-cl2m, bt-70cd, bt-nyjj, bt-foit all closed with reasons
- [ ] CHANGELOG.md updated
- [ ] `git push` succeeds and `git status` shows up-to-date with origin
- [ ] ADR-002 stream statuses updated if applicable

### Phase 2 done when
- [ ] **`go build ./... && go vet ./... && go test ./pkg/ui/` all green** (especially: existing tests for semanticHybridEnabled / semanticSearchEnabled / status messages updated, not skipped)
- [ ] bt-krwp closed (search UX shipped, no dead-corner state, quoted-exact works, hybrid threshold tuned, alt+h removal confirmed with user per AGENTS.md rule 1)
- [ ] bt-ja2y closed (boot default decided + microcopy in place)
- [ ] bt-v7um Part 1 done (Updated cell visible in BOTH markdown export AND live TUI list row)
- [ ] Dogfood verified: search feels good
- [ ] CHANGELOG.md updated
- [ ] `git push` succeeds and `git status` shows up-to-date

### Phase 3 done when
- [ ] **`go build ./... && go vet ./... && go test ./pkg/ui/ ./pkg/loader/ ./internal/dolt/` all green** (Dolt + loader paths covered)
- [ ] cass-ynoq is at least scoped (you've read its current state) AND minimum surface pinned in bt-5hl9 acceptance
- [ ] bt-5hl9 closed (CompactIssue reads direct session columns, NOT metadata blob; matches the surface pinned in acceptance)
- [ ] bt-2cvx closed (Author cell shows session)
- [ ] bt-v7um Part 2 closed (per-field decisions made + cells implemented)
- [ ] dotfiles-qew CLAUDE.md update merged
- [ ] CHANGELOG.md updated
- [ ] `git push` succeeds and `git status` shows up-to-date

### Phase 4 done when
- [ ] **No code changes expected** — these are read-only audits producing docs + follow-up beads. If anything DID change code, run `go build ./... && go vet ./...`.
- [ ] bt-72l8.1 audit doc committed + follow-up beads filed
- [ ] bt-72l8.1.1 TUI completeness doc committed + follow-up beads filed
- [ ] bt-gf3d.1 keybind triage table appended to docs/audit/keybindings-audit.md
- [ ] bt-6q8c labels.md updated with view:* taxonomy + bulk-retag complete (AFTER bt-72l8.1 + bt-72l8.1.1 land, per Phase 4 sequencing note)
- [ ] CHANGELOG.md updated
- [ ] `git push` succeeds and `git status` shows up-to-date

### Phase 5 done when
- [ ] **`go build ./... && go vet ./... && go test ./...` all green** (whichever of A or D shipped)
- [ ] bt-z5jj decision (A vs D) recorded + bt-3suf dispatched accordingly
- [ ] bt-8jds resolved (was deferred from Phase 1 quartet; now safe post-bt-gf3d.1 audit)
- [ ] bt-rbha shipped (gates surface)
- [ ] bt-mbjg closed (default-hide implemented)
- [ ] CHANGELOG.md updated
- [ ] `git push` succeeds and `git status` shows up-to-date

---

## Concrete next-session starting move

**Pick this exact quartet:** bt-cl2m + bt-70cd + bt-nyjj + bt-foit. Sequential in one session. Total estimated ~2 hours.

(bt-8jds was deferred from this quartet because it touches `pkg/ui/model_update_input.go:1135` — same file as bt-krwp's Phase 2 work at lines 615-710. Will resolve in Phase 5 after bt-gf3d.1 audit.)

After verification, push, and start Phase 2 design conversation on bt-krwp (4 questions above).

Optional parallel: if you want max throughput, dispatch bt-72l8.1.1 (TUI ghost audit) as a worktree subagent at the start of the session. It returns a doc + follow-up beads while you work the bugs. **No coordination needed** — the audit doesn't write to any file you're touching.

Run pre-flight checks first (top of plan).

---

## What this leaves the codebase looking like

After full execution (~6-8 sessions across the arc):

- **No more dead-state UX**: search modes cycle clearly, hotkeys are documented, gates have a surface
- **Session-aware**: detail pane shows who-and-when, cross-project beads trace origin
- **Discoverable**: `view:*` labels make TUI work batchable, audit docs catalog what's actually wired
- **Decisions made with evidence**: sprint feature kept or retired based on class context, not blind
- **Codebase mergeable at every step**: no half-built features, no broken tests, no "TODO finish this"

The post-arc state is a tool you actually want to use daily, with the context to make the next round of decisions cleanly.
