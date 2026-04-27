# Changelog

Development log for beadstui. Each entry covers one Claude Code session's work, keyed by date.

For architectural decisions, see `docs/adr/`. For issue tracking, use `bd list`.

---

## 2026-04-27 (evening) — Phase 2 search UX complete: bt-v7um Part 1 + bt-krwp + bt-ja2y

**Phase 2 of the bangout-arc plan: detail-meta Updated cell, search UX overhaul (Ctrl+S cycle / quoted-exact / badge threshold), and search defaults reform (boot-as-hybrid-when-index-exists + mode-purpose copy). All three closed in one session, four follow-ups filed.**

### What shipped

- **bt-v7um Part 1** (P3 feature, CLOSED) — Detail-pane meta table widened to include Updated alongside Created (both absolute via `FormatTimeAbs`, chosen for column symmetry). List-row age cell now reads from `UpdatedAt` (matches existing `board.go` convention; previously inconsistent — list used `CreatedAt`). Beads edited since creation (`UpdatedAt != CreatedAt`) prefix the age with `~` and render via new `MutedTextItalic` style; never-edited beads stay plain. Cell width bumped 8 → 9 to fit worst-case `~11mo ago`. Italic is a soft signal that degrades gracefully on terminals/fonts that don't render it (the `~` prefix carries alone). (`pkg/ui/model_filter.go`, `pkg/ui/delegate.go`, `pkg/ui/theme.go`, `pkg/ui/theme_loader.go`)
- **bt-krwp** (P2 feature, CLOSED) — Search UX overhaul. **Ctrl+S** now cycles `fuzzy → hybrid → semantic → fuzzy` (single key, three modes, no dead-corner state). **H** repurposed as hybrid-preset cycle (only meaningful in hybrid mode; status hint redirects to Ctrl+S otherwise). **alt+h hard-removed** per AGENTS.md rule 6. New **`quotedExactFilter`** wrapper: `"foo bar"` matches the literal phrase; multiple quoted phrases AND-joined; comma-separated mixed-mode composes via `multiTokenFilter`. New **`capPerCallFilter` (n=25)** caps per-token semantic/hybrid output to reduce noise. Score badge gated by `abs(score) >= 0.05` (new `SearchScoreBadgeMinAbs` const) — hides `[0.00]` items pulled in by graph weight. Status messages cleaned up. Composition factored into `fuzzySearchFilter()` / `semanticSearchFilter(s)` helpers used at all four call sites. Footer hint updated. Tutorial pages updated. Nine pkg/ui files touched, no test breakage.
- **bt-ja2y** (P2 feature, CLOSED) — Search defaults reform. New `bootSearchMode()` helper stats `.bt/semantic/index-<provider>-<dim>.bvvi` via `search.DefaultIndexPath(os.Getwd(), search.EmbeddingConfigFromEnv())`. Index present + size > 0 → boot in hybrid; otherwise fuzzy. `NewModel` calls it once at construction, sets `semanticSearchEnabled` / `semanticHybridEnabled` accordingly, swaps the list filter to `semanticSearchFilter(s)`, and primes `SetHybridConfig(true, PresetDefault)`. `Init()` dispatches `BuildSemanticIndexCmd` at startup when hybrid is selected — index loads from disk in background, search lives within a beat. Cycle status messages enriched with mode-purpose copy: `"Fuzzy search — fast substring/character match, best for IDs and known phrases"`, `"Semantic search — finds items by meaning, use when fuzzy misses the right bead"`, `"Hybrid search [preset: <name>] — semantic + graph weight, best general-purpose mode"`. Hybrid auto-boot also surfaces the purpose copy as initial status. Tutorial "When to Use Each Mode" section expanded to cover all four modes (fuzzy / hybrid / semantic / exact-phrase). (`pkg/ui/semantic_search.go`, `pkg/ui/model.go`, `pkg/ui/model_update_input.go`, `pkg/ui/tutorial.go`)

### Filed

- **bt-pxbc** (P3 chore, OPEN, area:tui) — Theme system debt: the project's primary teal accent is sourced from `pkg/ui/defaults/theme.yaml` in theory but duplicated across ~7 hardcoded hex literals in render code. Discovered while picking the visual treatment for bt-v7um Part 1. Linked to **bt-54c3** + **bt-fd3k**.
- **bt-1od1** (P3 feature, OPEN, area:tui) — Surface hybrid preset purpose in status message on H cycle. Each of the 5 presets (default / bug-hunting / sprint-planning / impact-first / text-only) gets a one-line purpose copy. Filed during bt-krwp dogfood when user noted preset names alone aren't self-explanatory. Linked to **bt-krwp** + **bt-fd3k**.
- **bt-fkba** (P1 feature, OPEN, area:tui) — Detail pane: seamless navigation to referenced beads (parent / child / related / dep graph). Today's gap: detail-pane dep-graph IDs render as text but aren't keyboard-jumpable; navigating to a related bead is a 5-step process. Bumped from initial P2 to P1 per direct user input ("higher priority imo feature"). Three interaction-pattern sketches (numbered jump targets / cursor-based / hybrid). Linked to **bt-vhhh** (list-position arrow nav, adjacent layer) + **bt-xavk** (help redesign).
- **bt-14wc** (P3 bug, OPEN — partial fix shipped, audit pending, area:tui) — Status messages persist beyond the 5s `statusAutoDismissAge` intent. Two collaborating root causes: (1) ~20 sites use direct `m.statusMsg = ...` instead of the `setStatus()` helper that primes `statusSetAt`, and (2) periodic background sync (`SemanticIndexReadyMsg`) re-emits identical messages, resetting the lazy timer. **Minimum fix shipped same-day**: `handleSemanticIndexReady` no longer sets a status in the no-change "up to date" branch (silent success on background re-syncs). Kills the user-visible persistence symptom. Remaining acceptance — full audit of direct-assignment sites + helper conversion — stays open under bt-14wc. Linked to **bt-d5wr** (footer redesign).

### Notes

- bt-v7um Part 2 (per-field brainstorm for Reporter / Due / Estimate / External-ref / Defer / Wisp / Gate cells in the detail meta table) was NOT shipped — it gates on bt-2cvx + bt-5hl9 (session-author hydration from Dolt session columns). Will be re-filed as a fresh bead when those prereqs land per the bangout-arc Phase 3 plan.
- bt-ja2y dogfood gotcha: first cut derived workDir from `beadsPath` via `filepath.Dir(filepath.Dir(beadsPath))`. In Dolt-only setups (default since beads v1.0.1), `beadsPath` is `""` — only set in legacy JSONL/SQLite branches at `cmd/bt/root.go:267-268`. Fix was to use `os.Getwd()` directly, matching what `BuildSemanticIndexCmd` does at `semantic_search.go:489`. Captured in bt-ja2y close notes for posterity.
- Test gotcha during bt-ja2y: 3 footer tests construct a Model with empty issues and assert on phase-2 / worker / READY footer indicators when `statusMsg` is empty. Initial fuzzy-boot status hint clobbered that precondition. Dropped the fuzzy hint, kept the hybrid one — hybrid is the affirmative signal worth surfacing; fuzzy is the always-safe default that doesn't need a per-boot nudge (footer label + help overlay already cover discovery).
- Visual treatment of the edited-age signal (italic + `~`) flagged for possible revisit later. Combined gives belt-and-suspenders signal across terminals/fonts.
- bt-krwp dogfood: semantic and hybrid modes are noticeably slower than fuzzy. Architectural — embedding compute + graph metric computation — not introduced by bt-krwp. If it becomes blocking, separate perf bead.

---

## 2026-04-27 (afternoon) — Phase 1 bug bangouts (4 ships, parallel worktree dispatch)

**Phase 1 of the bangout-arc plan: 4 P2 bugs fixed in parallel via worktree subagents, all merged onto main and pushed in a single PM session.**

### What shipped

- **bt-cl2m** (P2 bug, CLOSED) — Background data refresh no longer dismisses open modals. Added `m.shouldDeferRefresh()` helper and guarded the three watcher-driven refresh paths (`handleSnapshotReady`, `handleDataSourceReload`, `handleFileChanged`) so they re-emit themselves via `tea.Tick(200ms)` when an interactive modal is active. Data isn't dropped — it lands when the modal closes. ModalAlerts intentionally exempt to preserve live-update visibility. (`pkg/ui/model.go`, `pkg/ui/model_update_data.go`; commit 596950ce)
- **bt-70cd** (P2 bug, CLOSED) — Unknown `bt robot` subcommands now write to stderr and exit non-zero. Added shared `unknownSubcommandRunE` helper wired onto every parent-only group under `bt robot` with `SilenceUsage: true` + `SilenceErrors: true`. Bare `bt robot` still shows help; nested groups (`bt robot sprint bogus`) also fixed. (`cmd/bt/cobra_robot.go`; commit 5b86f457)
- **bt-nyjj** (P2 bug, CLOSED — child of bt-19vp) — History view shows friendly empty state (no red banner) when launched from a non-git cwd. New `pkg/correlation/gitrepo.go` with `IsInsideWorkTree()` using `git rev-parse --is-inside-work-tree`; distinguishes silent-fallback (path not in repo) from real failures (binary missing, permissions). `correlator.GenerateReport` probes the work tree first and returns an empty `HistoryReport` with nil error when not in a repo. (`pkg/correlation/gitrepo.go` new, `pkg/correlation/gitrepo_test.go` new, `pkg/correlation/correlator.go`; commit ab6d341e)
- **bt-foit** (P2 bug, CLOSED) — `<` and `>` pane-resize keys documented in help overlay (`?`) and shortcuts sidebar (`;`); label column alignment no longer drifts when list pane widens. Root cause: delegate appended right-side columns (assignee, author, labels) only when populated, with no blank-padded reservation, so rows without values had different rightWidth and the title pad-to-fill produced variable left-padding. Fix: every row reserves the same cell count once each column threshold is crossed (14 assignee, 12 author, 23 labels). (`pkg/ui/delegate.go`, `pkg/ui/model_view.go`, `pkg/ui/shortcuts_sidebar.go`; commit de0f4641)

### Process

- All 4 dispatched as parallel worktree subagents in a single message; each returned green (`go build ./... && go vet ./... && go test ./pkg/ui/`).
- PM cherry-picked each commit onto main (subagent branches each based on the original main commit, so only the first ff-merge succeeded; remaining three needed cherry-pick to land on the freshly-advanced main).
- Final verify on merged main green; pushed to origin.
- Pre-existing `pkg/view` golden test failures observed by 2 subagents and confirmed unrelated (fail on baseline main without these changes). Not in Phase 1 scope.

### Notes

- Wall time ~10 min for parallel dispatch + ~5 min PM merge/verify/push vs ~2h sequential estimate.
- bt-foit replaced bt-8jds in the original quartet to avoid Phase 2 file collision on `model_update_input.go`.
- ADR-002 Stream 6 (TUI polish): all 4 Phase 1 quartet items done.
- Next: Phase 2 (search UX overhaul — bt-krwp + bt-ja2y + bt-v7um Part 1) after design-question pass with user.

---

## 2026-04-27 — Search UX dogfood + bangout-arc planning (2 ships, 12 beads filed/updated, cross-project)

**Dogfood-driven session: started as a check-in on two decision beads (bt-z5jj sprint, bt-uahv data layout), turned into a search-UX audit when the user noticed multi-token queries weren't supported. Net: 2 small ships, 12 beads filed/updated across 3 repos, and a consolidated arc plan written for the next 6-8 sessions.**

### What shipped

- **bt-jwo3** (P3, feature, CLOSED) — TUI search: comma-separated multi-token OR. Wrapper `multiTokenFilter` added to `pkg/ui/id_bucket_filter.go` alongside the existing `idPriorityFilter` chain. Wired at 4 sites (`model.go:847`, `model_update_analysis.go:76`, `model_update_input.go:683/695/704`). 8 new tests; full pkg/ui suite passes (22.7s). Typing `z5jj, uahv` now populates both beads in the list.
- **bt-treo** (P3, bug, CLOSED) — Detail pane intercepts `/` and teleports to the search bar instead of forwarding to viewport. Single 5-line intercept in `pkg/ui/model_update_input.go:1269`. Filed and shipped same session.

### Decisions recorded

- **bt-uahv** (P3, task, CLOSED) — Decision: `.beads/` = bd-owned (Dolt server, bd config, conventions, hooks, push/export state); `.bt/` = bt-owned, regenerable or local-only (caches, indexes, audit logs). Migration files: `correlation_feedback.jsonl`, `interactions.jsonl`, `feedback.json` to move with one-release read-fallback. Implementation in **bt-v6rw** (P3, filed).
- **bt-z5jj** (P3, feature, REOPENED — `human` label) — Sprint feature decision (A retire vs D repurpose against molecules) deferred pending **bt-72l8.1** ghost-features audit. Investigation surfaced that sprint code originated in Jeffrey's beads_viewer (epic `bv-134`, closed 2025-12) with the producer side never built — Jeffrey's bv-156 commit explicitly said "Full sprint CRUD requires bd CLI changes" that never landed upstream. Three options reframed: A (retire), D (repurpose against molecules — beads' native multi-step grouping primitive), or original C (bt as canonical sprint store). Decision waits on audit data.
- **bt-3suf** (P3, task, OPEN — blocked-by bt-z5jj, `human` label) — Sprint retire impl, gated.

### Beads filed (this session)

**bt-side audits + features:**
- **bt-72l8.1** (P2, child of bt-72l8) — Ghost-features audit: classify every `--robot-*` and TUI mode as working/stub/ghost/partial.
- **bt-72l8.1.1** (P2, child of bt-72l8.1) — TUI-specific deeper pass (per-view completeness).
- **bt-krwp** (P2) — Search UX overhaul: collapse Ctrl+S/H into single mode cycle, repurpose H as preset cycle, add quoted-exact, fix status clarity, threshold `[0.00]` badge, cap multi-token in hybrid. Verification comment attached with concrete dogfood evidence (4 distinct UX bugs documented from screenshots).
- **bt-ja2y** (P2) — Search defaults reform: pick boot mode + surface why other modes exist.
- **bt-gf3d.1** (P2, child of bt-gf3d) — Hotkey/feature audit: which keys deserve top-level binding.
- **bt-fd3k** (P3, epic) — TUI settings/config surface (39+ BT_* env vars + 5 hybrid presets + theme + thresholds — almost none have an in-TUI surface today).
- **bt-6q8c** (P2 — bumped from P3) — TUI labels reform: add `view:*` sub-area dimension.
- **bt-v7um** (P3) — Detail pane meta: surface Updated cell + brainstorm rest.
- **bt-rbha** (P2) — TUI surface for type=gate + human-labeled beads (sister of bt-mbjg; gates the close).
- **bt-t8mu** (P3) — Natural-language surface for wisp/molecule grouping (de-jargon the operational primitives).
- **bt-54c3** (P3) — Themes: in-TUI picker for the existing theme system.

**Cross-project pairs (filed in their own repos, paired-bead notes added bt-side):**
- **cass-ynoq** (P2 in `~/System/tools/cass`) — Stable session-ID surface for cross-tool consumers. Data contract for the bt session-author display chain.
- **dotfiles-qew** (P3 in `~/.files`) — Document session-id-as-author convention in global CLAUDE.md (currently missing — convention is "cd over and file" but doesn't specify how origin gets traced).

### Beads updated (existing)

- **bt-mbjg** — confirmed default-hide gates decision; gated on bt-rbha being filed (done).
- **bt-ba9f** — kept open (not close-superseded). Search-bar portion shipped via bt-jwo3; remaining scope is CLI flag `bt --ids=...` + dedicated modal.
- **bt-2cvx + bt-5hl9** — paired-bead notes added linking to cass-ynoq + dotfiles-qew so the 3-way coordination chain is discoverable.
- **bt-8jds** — linked as child of **bt-gf3d** (it was always a symptom of the keybinding overload epic).
- **bt-72l8** (P1 epic) — description updated with Section 9 pointer to bt-72l8.1 ghost-features audit, extending the epic from "Jeffrey-era attribution" to "Jeffrey-era leftovers (attribution + ghost features)".

### Discoveries

- **Sprint feature provenance**: complete READER for a JSONL no producer writes. Jeffrey shipped bv-134/155/156/159/161 (display side); the bv-156 commit message explicitly said producer side requires bd CLI changes that never happened. Now post-Dolt, beads has molecules as the native multi-step grouping primitive — Option D (repurpose burndown/forecast/dashboard against molecules) is the strongest path if not retired.
- **Hybrid search has a dead-corner UX**: H toggle without semantic on flips a hidden bit; status messages contain "text-only" with two unrelated meanings; preset cycle alt+H is no-op without an active query. Verification screenshots attached to bt-krwp.
- **Hotkey infrastructure already exists**: `docs/audit/keybindings-audit.md` (2026-04-23) is the canonical map. Stale in spots but bt-gf3d is the parent epic for fixing it. bt-8jds is a known case of `w` overload (project picker vs wisp toggle).
- **Search modes have 5 hybrid presets** (default, bug-hunting, sprint-planning, impact-first, text-only) but no UI surface to discover or pick — only alt+H cycle. Surfaced as gap motivating bt-fd3k.

### Plan written

- **`docs/plans/2026-04-27-bangout-arc.md`** — consolidated 5-phase arc plan (bug bangouts → search UX → foundation that unblocks → parallel audits → decisions revisited). Includes orientation block (full ground-truth state), per-phase done checklists, parallelization map, user-input touchpoints, cross-project coordination chain. Self-contained handoff for next session(s).

### Process notes

- **Cross-project bead filing**: per global CLAUDE.md convention, used `cd <repo> && bd create` for cass + .files beads. Paired-bead notes added bt-side via `--append-notes` since cross-prefix `bd link` doesn't resolve across separate Dolt instances.
- **bd close `--reason` non-ASCII corruption**: re-confirmed the 2026-04-27 lesson from bt-mhcv. Em-dashes in bt-jwo3 close reason got mangled to `â€"` via Windows bash cp1252 layer. Re-closed with ASCII-only (`--`) — issue persists, no code-side fix yet.
- **Hook sandboxing pattern**: planning skill activates when entering plan mode; behavioral rules in CLAUDE.md still apply if skill doesn't load.

---

## 2026-04-27 — bt-mhcv Dolt-migration bead audit (163/6/0 GREEN/YELLOW/RED across 169 open beads)

**Phase A + B + C of the systematic audit of all open bt beads against post-v0.56.1 Dolt-only beads architecture. Subagent-driven: 14 parallel triage agents, one per `area:*` bucket (`area:tui` split into 3 chunks of 24). Total parallel wall time ~2 minutes for 169 beads. Outcome: backlog is in much better shape than the bead's worst-case framing assumed — the late-April cleanup arc (bt-uh3c brainstorm + 2026-04-25 data-source survey + AGENTS.md awareness section + ADR-003 proposal) caught most of the rot before this audit ran. Zero RED, six YELLOW, all addressable with corrective comments.**

### What shipped

- **bt-mhcv** (P0, task, CLOSED) — Phase A inventory + classification of all 169 open beads, Phase B corrective comments on YELLOWs + close-as-duplicate on bt-x685, Phase C retrospective doc. Per-bucket triage tables at `docs/audit/triage/<bucket>.md`; master audit at `docs/audit/2026-04-27-dolt-migration-bead-audit.md`.
- **5 corrective comments** posted (Phase B):
  - **bt-2cvx** — scope's "Dolt columns vs metadata JSON" decision is closed by bd-34v Phase 1a; reframe as TUI/search display work, point hydration at bt-5hl9.
  - **bt-ldq4** — "transparent swap later" framing is stale; the swap IS bt-5hl9. Read direct columns, not metadata blob.
  - **bt-v0mq** — `.beads/issues.jsonl` is opt-in export, not system-of-record; auto-export decision belongs in bt-uahv before fixing the symptom.
  - **bt-if3w.1** — sprint-view extraction gated on bt-z5jj rebuild-vs-retire decision; added bt-z5jj as blocker.
  - **bt-tq60** — paper-trail comment that bt-x685 was closed as its duplicate.
- **1 close-as-duplicate**: **bt-x685** closed as duplicate of bt-tq60 (incidental finding — same bug, same skipped test, same proposed fix; not a stale-architecture issue).

### Per-bucket totals

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
| tui-1/2/3 | 72 | 71 | 1 | 0 |
| **TOTAL** | **169** | **163** | **6** | **0** |

### Methodology lessons (worth keeping)

- **Per-area parallelism is cheap and fast**: 14 buckets x ~25 beads each completed in ~2 minutes wall time. Worth reusing for any future backlog-scale audit.
- **JSON dossiers beat per-bead `bd show`**: each agent got a JSON file with full descriptions; agents did NOT call `bd show` per bead. No Dolt-server thrash, much cheaper.
- **Pre-classified landmarks reduced noise**: telling agents "these 9 beads are GREEN, don't re-classify" prevented spurious YELLOW votes on the bt-mhcv / bt-08sh / bt-z5jj / bt-uahv decision-capture beads.
- **Bias toward GREEN was correct**: rubric explicitly told agents "for ambiguous cases lean toward GREEN; the bar for YELLOW/RED is a clear stale assumption you can quote." Without that, YELLOW would have been over-applied.
- **Duplicate detection emerged for free**: bt-x685 ↔ bt-tq60 wasn't on the agenda but the cli agent caught it because it had read both descriptions in the same pass.
- **bd close --reason via bash command line corrupts non-ASCII**: em-dashes round-tripped through bash's cp1252 layer became mojibake in storage. Comments via `bd comments add -f file.txt` (UTF-8 file) are clean. For future close reasons with non-ASCII, prefer ASCII-only or find a file-input path.

### Cross-bucket clusters surfaced (sequencing notes for later)

1. History-view cluster (bt-ezk8 / bt-nyjj / bt-npnh) — surface symptoms of the multi-repo correlator gap that bt-08sh + bt-3ltq own at the data layer.
2. Modal-rendering cluster (bt-dp41 / bt-menk / bt-lin9) — likely one OverlayCenter fix.
3. Mouse-support cluster (bt-fbx6 / bt-km6d / bt-ks0w) — share a chrome-measurement design question.
4. Robot-mode contract cluster (bt-70cd / bt-ah53 / bt-tq60) — bt-ah53 is the meta-fix.
5. Pairs/refs v2 ecosystem (bt-92ic / bt-dhqw / bt-9prn / bt-xgba) — clean Dolt-era framing.
6. Security cluster (2026-04-27 STRIDE/OWASP) — adjacent code in `internal/datasource/` could batch.
7. bt-689s ↔ bt-thpq — both Dolt-system-table investigations; one recon could answer both.
8. bt-search consolidation — bt-hazr ↔ bt-ox4a flagged.

### Files touched

- `docs/audit/2026-04-27-dolt-migration-bead-audit.md` (new)
- `docs/audit/triage/{analysis,bql,cli,correlation,data,docs,export,infra,no-area,search,tests,tui-1,tui-2,tui-3}.md` (14 new)
- `CHANGELOG.md` (this entry)
- bd database: 5 comments posted, 1 dependency added, 1 issue closed, bt-mhcv updated.

---

## 2026-04-26 — bt-46p6 alerts redesign epic closed (centrality nav + dismissed filter + mouse + cursor-row cleanup)

**Closes the alerts redesign epic at 17/20 children. All 8 acceptance criteria were already met as of 2026-04-25; this session shipped the only genuine v1 gap (.12) and the crisp piece of v2 (.13's dismissed-events filter), then detached 3 post-AC threads to standalone beads. Live dogfooding surfaced two more bugs (mouse off-by-one, cursor-row cluttered with generic descriptions) and a feature opportunity (graph-scope alerts hide their Details), all fixed in the same session.**

### What shipped

- **bt-46p6.12** (P3, task, CLOSED) — PageRank/centrality exposure. Enter on a `centrality_change` alert now opens the insights view (graph-scope alerts have no single-issue target — value is in the rankings themselves). Added a Centrality section to the issue detail panel showing PageRank rank+value, betweenness rank+value, and in/out degree, gated on Phase 2 readiness. AC2 (alert usefulness): KEEP — the detector flags meaningful graph-position shifts and now has an actionable destination via enter→insights.

- **bt-46p6.13** (P3, feature, CLOSED — scope-narrowed) — Notifications tab v2 phase 1: dismissed-events filter. `d` key toggles visibility of dismissed events; default unchanged (hidden), so v1 callers see no behavior change. Dismissed rows render with a leading `✕ ` marker when surfaced. Footer hint flips between `d: show dismissed` / `d: hide dismissed`. Phases 2-4 re-homed: bt-s9sg (cross-session via Source, blocked on bt-k9mp + bt-2cvx), bt-5gnr (alerts-dismissed-log spike), bt-mo70 P4 (derived-signal notifications design).

- **bt-t5wy** (P2, bug, FIXED in same session) — Modal mouse off-by-one in both alerts and notifications tabs, surfaced during .13 dogfooding. `modalChromeAboveItems` was 5 (assuming a vertical pad row that `padContentLines` doesn't actually emit — it adds horizontal padding only). Corrected to 4 with `TestModalChromeAboveItems_MatchesRender` — a render-anchored guard test that asserts the first item lands on the row identified by the constant, so future drift trips a visible failure rather than the silent off-by-one.

- **bt-7ye5** (P2, bug, FIXED in same session) — Graph-scope alerts hid their Details. Selecting a `dependency_loop` alert showed `2 new cycle(s) detected` with no indication of which beads were in the loop; same class for `centrality_change` showing `N PageRank changes detected` without listing them. The data was on the alert (`Alert.Details`), the renderer just dropped it. Now: when `IssueID == ""` and `Details` is non-empty, the cursor-row detail line shows the first entry with `(+N more)` when there's more.

- **bt-xyjd** (P2, task, FIXED in same session) — Removed the inline italic alert-type definition under the cursor row. Generic metadata that didn't help once the user knew the type, plus variable cursor-row height (definition always + title sometimes = 2 lines vs 1 elsewhere) was the second source of mouse hit-test misalignment. Type explanations now belong in the dedicated `?` help modal (filed as bt-i20z). Cursor row is now consistent: 1 row by default, 2 when there's a title or Details preview.

- **bt-46p6** (P1, EPIC, CLOSED) — All 8 AC met. The 3 remaining open children (.15 row density, .18 global aggregation deferred, .19 cross-project nav) detached to standalone beads since each has its own scope, blockers, and design questions.

### New beads filed

- **bt-t5wy** (P2, bug) — Mouse off-by-one (closed in same session)
- **bt-7ye5** (P2, bug) — Graph-scope alerts hide Details (closed in same session)
- **bt-xyjd** (P2, task) — Remove inline alert-type definitions (closed in same session)
- **bt-q4tn** (P3, feature) — Clickable bead refs in detail pane + back/forward navigation (user feature request)
- **bt-i20z** (P3, feature) — `?` key opens dedicated alert-type help modal (replaces removed inline definitions)
- **bt-s9sg** (P3, feature) — Notifications: cross-session activity via Source field (blocked on bt-k9mp + bt-2cvx)
- **bt-5gnr** (P3, task) — Alerts-dismissed-log: decide audit-trail vs restore-log semantics (design spike)
- **bt-mo70** (P4, task) — Notifications: density/centrality/count-delta signals — design spike

### Verify

- `go build ./...` clean, `go vet ./...` clean
- `go test ./... -count=1 -short` — all packages green incl. 89s e2e
- New tests: `TestActivateAlert_CentralityChangeOpensInsights`, `TestActivateAlert_StaleAlertJumpsToBead`, `TestNotifications_DismissedFilterToggle`, `TestModalChromeAboveItems_MatchesRender` (covers both tabs), `TestAlertsRender_DependencyLoopShowsCyclePath`, `TestAlertsRender_CentralityChangeShowsFirstChange`

### Why this matters

bt-46p6 was the longest-running epic on the board (filed 2026-04-15). Its acceptance was already satisfied 11 days ago; what kept the parent open was a mix of post-AC scope expansion (notification center, mouse, deep-links, alert-type definitions) and a few honest follow-ups that emerged during the work. Today's wrap closed the last v1 gap (.12), shipped the user's explicit v2 ask (.13's filter), and broke the scope-creep into clean standalone beads.

Live dogfooding paid for itself three times in this session: caught the mouse off-by-one (bt-t5wy, now guarded by a render-anchored probe), exposed that graph-scope alerts hid their Details (bt-7ye5, the user couldn't see which beads were in a dep loop), and surfaced that the inline alert-type definitions were both UX clutter and a second source of mouse misalignment (bt-xyjd → replaced by bt-i20z `?` help modal). Type explanations now belong on demand, not always-on.

---

## 2026-04-25 — BQL --bql filter fix + bt-uh3c brainstorm reshape (post-Dolt architecture audit)

**Two distinct outcomes from one session: a P1 bug fix shipped, and a major scope correction surfaced through brainstorm-driven recon — bt's correlator/sprint stack is stale relative to the post-v0.56.1 Dolt-only beads era.**

### What shipped

- **bt-111w** (P1, bug, FIXED) — `bt robot list --bql 'id="X"'` was silently dropping the BQL filter and returning the unfiltered list (universal — both `--global` and local paths). Root cause: `cmd/bt/robot_list.go` intentionally bypasses `robotPreRun` to skip label/recipe analysis, but the bypass also dropped `--bql` filtering since that's where it lives (`cmd/bt/cobra_robot.go:107-121`).
  - Fix in `cmd/bt/robot_list.go`: applies BQL inline alongside the existing `--source` filter, in the same order as `robotPreRun` (source → BQL → simple flags). Echoes `bql` in the `listQuery` envelope so consumers can confirm the filter was applied.
  - Tests: 5 new regression tests in `cmd/bt/robot_bql_test.go` covering `id=`, `priority=`, `status=` equality, no-match, and BQL composed with `--source`.
  - Live verification: `bt --global robot list --bql 'id="cass-uh3c"'` previously returned 38KB (count=100, total=3206) — now returns 660B (count=1, total=1) with bql echoed in envelope. Local-repo path also fixed.
  - Commit `80d9d571`.

### What was filed (brainstorm-driven discovery)

bt-uh3c brainstorm (claim: "let's claim and work bt-uh3c") evolved into a multi-phase architectural audit when ground-truth recon revealed that beads' v1.0.1 migration to Dolt-only (March 2026) had left bt's pre-v0.56.1 assumptions in place. Phase 1 dispatched 3 parallel agents (recent bd beads recon, beads upstream source recon, bt blast-radius scan) and surfaced:

- **bt's correlator (`pkg/correlation/`)** uses `git diff` against `.beads/<project>.jsonl` as a witness file. Beads no longer produces this file. Result: `history`, `related`, `causality` subcommands fail universally (not just under `--global`).
- **bt's sprint loader (`pkg/loader/sprint.go`)** reads `.beads/sprints.jsonl`. Beads upstream has no `sprints` table — sprints were always bt-only metadata, and the JSONL was a bt-bt construction. `forecast` and `sprint show` are stuck.
- **bt's `CompactIssue` mapping (`pkg/view/compact_issue.go:187-188`)** still reads `created_by_session` and `claimed_by_session` from the metadata JSON blob, while `bd-34v` Phase 1a (merged 2026-04-24) provides direct columns upstream.
- **bt's `.beads/` vs `.bt/` data-home split** is partly accidental (`tree-state.json` is bt-only UI state but lives in shared `.beads/`).
- **Beads has an upstream `events` table** (`Storage.GetEvents`) — the load-bearing finding for bt-uh3c item 2 (events timeline). Means bt composes upstream primitives instead of rolling its own `dolt_log` queries.

### New beads filed (with proper relations)

- **bt-111w** (P1, bug, FIXED above)
- **bt-vhn2** (P2, bug, **CLOSED-AS-SUPERSEDED** in this session) — original `--global routing` framing was wrong
- **bt-ah53** (P2, task) — Robot mode I/O contract: documented stdout/stderr/exit invariants + verify-test sweep
- **bt-70cd** (P2, bug) — Unknown `bt robot` subcommand prints help to stdout exit 0 (Cobra default)
- **bt-82w8** (P3, feat) — `bt robot comments <id> --global`: standalone subcommand
- **bt-3qfa** (P3, feat) — Per-subcommand input flag manifest in `bt robot schema`
- **bt-llh2** (P3, feat) — BQL parse-error hints for `id:` syntax
- **bt-kv7d** (P3, **CLOSED-AS-OBVIATED** in this session — merged into bt-5hl9)
- **bt-08sh** (P2, feat, NEW) — Correlator Dolt migration: replace JSONL+git-diff witness with `dolt_log`/`dolt_history_issues`
- **bt-z5jj** (P3, feat, NEW) — Sprint feature: rebuild against Dolt or retire (decision needed)
- **bt-uahv** (P3, task, NEW) — Canonical `.beads/` vs `.bt/` data-home split (ADR-flavored)
- **bd-3gb** (P2, in beads repo) — Promoted to load-bearing: `bd history --json` returns prose, breaking bt's planned wrapping for events timeline

### Existing beads reshaped

- **bt-uh3c** — Hard block on bt-vhn2 removed; soft relations to bt-08sh / bt-z5jj. Item 2 implementation path simplified to compose upstream `Storage.GetEvents` + `bd history` (no rolling our own `dolt_log` queries).
- **bt-5hl9** — Rescope confirmed: bt-side hydration of session columns (Phase 1a now actionable post-bd-34v merge). Absorbs bt-kv7d's scope.

### What didn't ship — deferred to follow-on sessions

- bt-uh3c's actual `show <id>` and `events <id>` implementation (now architecturally unblocked, awaiting design pass)
- bt-08sh, bt-z5jj, bt-uahv work (all `workflow:investigate` until decisions land)
- bt-5hl9 Phase 1 implementation (Phase 1a upstream-merged, bt-side hydration ready to start)
- Robot mode I/O contract verify-test (bt-ah53)

### Process note (for future sessions)

This session's brainstorm-then-recon-then-reframe loop produced more value in beads-graph cleanup than in code shipped. The key lesson: when a bug feels architecturally noisy ("--global doesn't work for these 7 subcommands"), check whether the framing assumes a stale architecture. In this case, beads's Dolt migration was the load-bearing context I was missing — a single user prompt ("how does Dolt affect this?") triggered the recon that found the actual root causes. Adding Dolt-migration-awareness to the AGENTS.md or auto-memory would help future sessions hit this earlier.

---

## 2026-04-24 — Notification persistence across bt restarts (bt-6ool Part A)

**Notifications now survive `bt` exit and re-launch. The ring buffer write-throughs each event as one JSONL line at `~/.bt/events.jsonl` and replays the file (filtered by max age) on boot.**

### What shipped

- **bt-6ool Part A** (P3, feature) — JSONL persistence layer for the notifications ring buffer.
  - `pkg/ui/events/persist.go` (new) — `LoadPersisted(path, maxAge)`, `DefaultPersistPath()`, internal `filePersister` with append-batched writes. Missing file is not an error; corrupt JSON lines are skipped silently; complete read failures (permissions etc.) propagate. Mutex-guarded so concurrent ring writers serialize through one disk write per batch.
  - `pkg/ui/events/ring.go` — `RingBuffer` gains `SetPersistPath(path)`, `Hydrate(events)`, and write-through inside `Append`/`AppendMany`. Persistence happens after the lock is released to keep the in-memory hot path fast; write failures log via `pkg/debug` but never propagate (in-memory ring is the source of truth for the live session).
  - `pkg/ui/model.go` — `NewModel` now hydrates the ring from `~/.bt/events.jsonl` (filtered to `DefaultMaxPersistAge` = 7 days) and enables write-through. Disabled by `BT_NO_EVENT_PERSIST=1` (user opt-out) or `BT_TEST_MODE` (so `pkg/ui` tests don't bleed in real machine state).
  - Tests: 6 focused tests in `pkg/ui/events/persist_test.go` covering round-trip, max-age filter, hydrate cap-respect, corrupt-line resilience, missing-file tolerance, opt-out.

### Out of scope / Part B

Part B (offline capture — emitting synthetic events for activity that happened while bt was closed) is filed as a separate bead. It builds on Part A's persistence layer but needs its own decisions around baseline-snapshot storage, dedup against any in-flight events, and "too stale to backfill" thresholds.

A potential file-growth concern: the JSONL is append-only with no rotation. At ~1 KB/event, 100 events/day, after 30 days the file reaches ~3 MB while only the last 7 days hydrate. Acceptable for v1; revisit if dogfooding shows the file becoming meaningfully large.

### On-disk format note

The persisted format is the `events.Event` struct's default Go JSON encoding. Renaming a field is a breaking change for existing on-disk files. Comment on `persist.go` calls this out for future engineers; acceptable risk for a single-user per-machine store.

---

## 2026-04-24 — Notifications filter sister-fix + RepoKey moved to pkg/model (bt-gydd)

**Fixes the same key-derivation mismatch as bt-ci7b but in the notifications-tab filter site, and consolidates repo-key derivation into a single canonical helper at the `pkg/model` layer so future filter sites can't drift.**

### What shipped

- **bt-gydd** (P3, bug, sister to bt-ci7b) — `events.Event.Repo` was populated via `repoFromBeadID(issue.ID)` (raw lowercase-untouched ID prefix). The notifications filter at `pkg/ui/model_alerts.go:133-136` looked up `m.activeRepos[snap[i].Repo]`. For divergent repos (DB name `marketplace`, IDs `mkt-xxx`), the lookup always missed → notifications silently hidden.
  - **Refactor**: moved `RepoKey(issue)` and `ExtractRepoPrefix(id)` into `pkg/model/repokey.go`. `pkg/ui`'s `IssueRepoKey` and `ExtractRepoPrefix` are now thin wrappers. `pkg/ui/events` can now use `model.RepoKey` directly without an import cycle.
  - **Fix**: `newCreatedEvent`, `newClosedEvent`, `newCommentedEvent`, `newEditedEvent` all derive `Repo` via `model.RepoKey(issue)` — same key derivation as the `activeRepos` map. `repoFromBeadID` retired (no remaining callers; the test fixtures that synthesized fake events now call `model.ExtractRepoPrefix`).
  - **Tests**: `TestVisibleNotifications_HonorsSourceRepo` exercises the marketplace ↔ mkt divergent case end-to-end through `Update(SnapshotReadyMsg)` → `events.Diff` → `visibleNotifications` (verified failing pre-fix with both assertions firing — Event.Repo was "mkt", visible count was 0). Existing `TestDiff_Created` still passes because synthetic test issues with no SourceRepo fall through to ID-prefix derivation, which matches the old behavior for repos whose DB name == ID prefix.

### Why this matters

ci7b's close note flagged this as the obvious sister bug. The right move was filing it and fixing it the same session, not deferring. Now both the issue list and the notifications tab honor the same key derivation, and the helper lives at the `pkg/model` layer so the next time a new filter site appears (alerts cross-project nav from .19, notifications v2 from .13) the canonical key is right there with no opportunity to silently re-introduce the same bug.

---

## 2026-04-24 — Workspace filter no longer nukes the list on Dolt refresh (bt-ci7b)

**Fixes the workspace-mode regression where filtering to a single project (where the workspace DB name differs from the bead-ID prefix, e.g. `marketplace` ↔ `mkt-xxx`) caused every Dolt refresh to drop the list to "No items." until the user toggled filters.**

### What shipped

- **bt-ci7b** (P2, bug) — Root cause: `handleSnapshotReady`'s two filter loops (recipe-mode at lines 180-193, no-recipe at lines 221-272) computed the workspace lookup key as `strings.ToLower(item.RepoPrefix)`, which is purely ID-derived. But `m.activeRepos` is keyed by the workspace DB name. `IssueRepoKey(issue)` already handles this correctly — it consults `issue.SourceRepo` first and falls back to ID-prefix parsing — and is used everywhere else (`applyFilter`, alert filters, notification filters). The snapshot handler was the lone outlier.
  - Fix: replace both call sites with `IssueRepoKey(issue)`. 4-line diff.
  - Tests: `TestHandleSnapshotReady_WorkspaceFilterHonorsSourceRepo` exercises the marketplace ↔ mkt divergent case (verified failing pre-fix, passing post-fix). `TestHandleSnapshotReady_WorkspaceFilterAlsoRespectsIDPrefix` guards the SourceRepo-empty fallback so the fix doesn't regress the simple case.

### Why this manifested as "flash then sometimes recover"

Pure key mismatch — deterministic for divergent repos. The "recovers most of the time" observation was almost certainly user actions triggering `applyFilter` (which uses the correct key derivation). The "stuck" case is the canonical behavior; nothing in the snapshot path retries until the user changes filter state. Global mode worked because `activeRepos == nil` short-circuits the filter entirely.

### Out of scope (filed for follow-up if needed)

- Notification ring buffer's `m.activeRepos[snap[i].Repo]` lookup at `pkg/ui/model_alerts.go:134` uses `events.Event.Repo` which is `repoFromBeadID` — same class of ID-vs-DB mismatch can hide notifications for divergent repos. Not the reported bug, but worth a separate bead if dogfooding surfaces it.

---

## 2026-04-24 — Notification deep-link to comment (bt-46p6.16)

**Pressing enter on a comment notification now opens the bead AND scrolls the detail viewport to the specific comment that fired the event, instead of landing at the top.**

### What shipped

- **bt-46p6.16** (P3, feature) — Comment-event deep-linking via Option B (timestamp).
  - `pkg/ui/events/events.go` — Added `Event.CommentAt time.Time`, populated only for `EventCommented`. Stable, no upstream beads schema dependency, no comment-index drift on deletion.
  - `pkg/ui/events/diff.go` — `newCommentedEvent` now copies `CommentAt = latest.CreatedAt` from the most recently added comment.
  - `pkg/ui/model.go` — New `pendingCommentScroll time.Time` model field. One-shot signal: when non-zero, `updateViewportContent` aligns the viewport to the matching comment, then clears the field.
  - `pkg/ui/model_update_input.go` — `activateCurrentModalItem` now sets `pendingCommentScroll = notif.CommentAt` for `EventCommented` notifications before calling `focusDetailAfterJump`. Keyboard-enter notification path collapsed into `activateCurrentModalItem` so it shares semantics with double-click activation; the duplicated workspace-reveal block is gone.
  - `pkg/ui/model_filter.go` — `updateViewportContent` records `(CreatedAt, byteOffset)` for every comment as it builds the markdown source. When `pendingCommentScroll` is set, it slices the source up to the matching comment, renders that prefix through the same Glamour renderer, counts newlines, and calls `viewport.SetYOffset(line)`. Same-renderer prefix render avoids ANSI-styling line-count drift.
  - Tests: `TestDiff_CommentAtCarriesCreatedAt` in `pkg/ui/events`; `TestUpdateViewportContent_ScrollsToCommentAt`, `TestUpdateViewportContent_NoScrollWhenPendingZero`, `TestActivateNotification_NonCommentEventDoesNotQueueScroll` in `pkg/ui`. Together they prove the signal flows from diff time → event → activation → viewport offset, and that opt-in semantics hold (no scroll without an explicit pending field).

### Why this matters

v1 of the notifications tab (bt-46p6.10) already showed comment events with the comment's first 80 runes as Summary. But pressing enter just opened the bead at the top, leaving users to scroll through long comment threads to find the one that fired. Closes the UX loop: events know which comment they were for, the model carries that signal across the modal-close boundary, and the renderer aligns the viewport without depending on string searches in styled output.

### Out of scope (deferred)

- Cross-project deep links into beads in repos that aren't hydrated. Behavior falls back to "open at top" — same as before this bead.
- Other event kinds (closed, edited). They open the bead at top; no natural scroll target exists.

---

## 2026-04-24 — Alert-type definitions surfaced in TUI + CLI (bt-46p6.17)

**Closes the discoverability gap left by the bt-46p6.4 rename: every alert type now carries a plain-English definition, exposed across pkg/drift, the TUI alerts modal, and a new `bt robot alerts --describe-types` JSON emitter.**

### What shipped

- **bt-46p6.17** (P3, task) — Single source of truth for alert-type meanings.
  - `pkg/drift/drift.go` — `AllAlertTypes()` returns the canonical 13-entry list (defensive copy); `AlertTypeDefinition(t)` looks up plain-English text from `alertTypeDefinitions` map and falls back to the raw type string for unknown values. Test `TestAllAlertTypesHaveDefinitions` guards the invariant that every registered type has a non-empty definition and that callers can't mutate the canonical slice.
  - `pkg/ui/model_alerts.go` — Selected alert row now renders a definition line (italic muted, indented) above the existing issue-title line. `alertsVisibleLines()` chrome reserve bumped from 7 → 8 so the page stays stable when the focused row expands to two detail lines.
  - `cmd/bt/robot_alerts.go` — `runDescribeAlertTypes()` emits `{generated_at, data_hash, types: [{type, definition}], usage_hints}` JSON; `alertTypeFilterHelp()` builds the `--alert-type` cobra help text dynamically from `drift.AllAlertTypes()` so help and code can't drift.
  - `cmd/bt/cobra_robot.go` — New `--describe-types` boolean flag on `robot alerts`; takes precedence over filter flags and exits after emitting taxonomy.
  - `cmd/bt/robot_help.go` + `cmd/bt/robot_graph.go` — Robot-help footer and the robot-graph command manifest both mention `--describe-types`.

### Why this matters

The bt-46p6.4 rename (`new_cycle → dependency_loop`, `pagerank_change → centrality_change`, etc.) gave alerts more honest names but didn't explain what each detector measures. Users saw `coupling_growth` in the modal with zero way to learn what it meant without reading source. This bead closes that loop by making definitions a first-class field that every consumer surface (TUI, CLI help, JSON output) reads from the same map.

---

## 2026-04-24 — Alert taxonomy rename + Progress sort + scope design (bt-46p6.4, bt-46p6.11, bt-lm2h, bt-7l5m)

**Locks the alert type taxonomy to user-facing names, retires the bt-46p6.11 coordination bead, ships a Progress sort mode for the list view, and records the Option C decision (bt-7l5m) that the parallel session (44b78454) executed as bt-46p6.8.**

### What shipped

- **bt-46p6.4** (P3, task, `333fd381`) — Renamed 7 AlertType constants + string values to user-facing names: `new_cycle → dependency_loop`, `blocking_cascade → high_leverage`, `stale_issue → stale`, `density_growth → coupling_growth`, `pagerank_change → centrality_change`, `node_count_change → issue_count_change`, `edge_count_change → dependency_change`. Unchanged: `blocked_increase`, `actionable_change`, `velocity_drop`, `high_impact_unblock`, `abandoned_claim`, `potential_duplicate`. Convention established across the taxonomy: `_change` = bidirectional drift, `_increase`/`_growth` = one-directional drift, bare noun = state. TUI short-form labels in `pkg/ui/model_alerts.go#alertTypeLabel` updated to match (`cycle → dep loop`, `density → coupling`, `nodes → issues`, `edges → deps`, `cascade → leverage`). Clean break per AGENTS.md rule 6; no backward-compat shim. 12 files touched, 110/110 insertions/deletions.

- **bt-46p6.11** (P2, task) — Closed as coordination-superseded. The bead tracked CLI-side parity for sibling beads `.4`/`.6`/`.7`/`.8`; each sibling's acceptance absorbed its own CLI surface concerns, so `.11` had no code of its own. Retired rather than abandoned — the coordination-bead pattern didn't pay rent.

- **bt-lm2h** (P3, feature, `73f7d132`) — Progress sort mode, 6th entry in the `s`/`S` cycle. Order: `in_progress → review → open → hooked → blocked → pinned → deferred → closed → tombstone`. Ties broken by priority asc then updated desc. Added `SortProgress` constant + `String()` case in `pkg/ui/model.go`; added sort case + `progressOrdinal` helper in `pkg/ui/model_filter.go`. Answers "what's actively moving right now?" — no existing mode covered this. Gate beads (`type=gate`) intentionally NOT given a special tier; the gate-clutter concern is tracked separately as **bt-mbjg** (default-hide `type=gate` from the list).

- **`030917ff`** — One-line e2e test drift fix for `tests/e2e/drift_test.go` assertion. 44b78454's `b4dcd7f6` commit changed the CLI drift renderer output from uppercase `"CRITICAL"` to lowercase `"critical"` in the new `Drift: N critical, …` summary; the assertion was still checking for the old format. Attribution noted in commit message.

### Decisions + deferred work filed

- **bt-7l5m** (decision, open) — Alert scope computation = project-scoped only, no global aggregates. Global view = union of per-project alerts tagged with `SourceProject`, filterable by project. Cross-project `external:` deps resolved before graph construction so each project's graph includes its real cross-project edges. Rejected Option A (scope-aware with global aggregate pass) and Option B (dual per-project + global always) because both preserved a "global aggregate" computation that bt-46p6.8's own problem statement argues is semantically incoherent across unrelated dependency graphs. Executed same day by 44b78454 — see their separate entry for bt-46p6.8 implementation specifics.

- **bt-46p6.17** (P3, filed, now unblocked since .4 shipped) — Surface natural-language definitions for each alert type in TUI modal + CLI `--describe-types` flag + optional JSON `definition` field. Names like `centrality_change` and `coupling_growth` don't self-explain; inline definitions close the opacity gap introduced by the rename.

- **bt-46p6.18** (P4, filed, deferred) — Global-scope aggregate metrics. Parked until upstream beads backend gains federated or canonical-scope primitives that make cross-project aggregates interpretable. Explicit revisit trigger recorded.

- **bt-46p6.19** (P3, filed, blocked by `.8`) — TUI cross-project alert navigation. After `.8`'s `SourceProject` attribution shipped (done), navigation from alert details into the target project's view still needs its own implementation.

- **bt-mbjg** (P2, filed) — Default-hide `type=gate` beads from list view; surface via explicit filter or dedicated view. Surfaced during this session's Progress-sort brainstorm — gates are coordination metadata, not work, and they clutter the "what can I pick up?" signal.

- **cass-ylx6** (P2, cross-project filed in `cass`) — `cass whoami --source env` returns stale session ID after `/clear` rotates the Claude Code transcript. CC doesn't re-export `CLAUDE_SESSION_ID` on `/clear`, so any tool that reads the env var after rotation gets the pre-`/clear` ID. Fix options: prefer active-transcript signal over env, or reconcile + warn on mismatch. Also a Claude Code-side bug at root.

### Context notes

- Auto-memory `reference_bd_dolt_push_windows.md` removed — `bd-nft` closed upstream in dolt 1.86.4; the manual `cd && dolt push` workaround is no longer needed.
- Comment added to **bt-yqh0** (cross-project paired-bead aggregation) with the 2026-04-24 `fjip` cluster dogfood data point (`mkt-fjip` + `bd-fjip` + `cass-fjip` — second concrete cluster after the original `96y` example, confirms P2 priority is right).
- Ran in parallel with session `44b78454` (bt-46p6.8 execution). Zero file overlap. `git commit --only` used every time for atomic staging against the shared `.git/index` (pattern captured as project memory `feedback_multi_session_git_scope`).
- `/clear` rotation observed mid-session: the bt-46p6.8 handoff ran in a CC process that had been `/clear`'d, landing the new transcript in `44b78454-040b-463f-9bad-fe60839eb272.jsonl` while the process's `$CLAUDE_SESSION_ID` still reported `842a70ba` — source of the cass-ylx6 filing.

---

## 2026-04-24 - Scope-aware alert computation + baseline schema v2 (bt-46p6.8)

**Locks in bt-7l5m's Option C: alerts are always computed at project scope. Global view is the union of per-project alerts tagged with `SourceProject` and filterable by project. No global-aggregate density / PageRank / cycle metrics — those are incoherent across unrelated dependency graphs. Ships per-project baseline sections so drift-delta alerts (centrality_change, coupling_growth, blocked_increase, etc.) fire correctly for every project in global mode.**

### What shipped

- **bt-46p6.8** (P2, task) — Two-commit implementation.

  Commit 1 (`710352d4`): `drift.Alert.SourceProject` field populated on every alert. New `drift.ProjectAlerts` helper partitions issues by `SourceRepo` (global mode) or collapses to one group keyed by fallbackProject (single-project mode), runs one `Calculator` per project, tags results, and returns the union in stable alphabetical project order. Partition-only interpretation of bt-7l5m's "each project graph includes its real cross-project edges" — per-project analyzers see only that project's issues; satellite-node inclusion (Option B) is tracked as follow-up audit in **bt-53vw** (P3, related). `cmd/bt/robot_alerts.go` rewired to `ProjectAlerts` with a per-project baseline loader. `pkg/ui/model_alerts.go#computeAlerts` signature changed to `(issues, workspaceMode)` and 5 TUI call sites updated; the precomputed `stats`/`analyzer` args are gone because per-project aggregates are re-analyzed per group (by design).

  Commit 2 (`b4dcd7f6`): Baseline schema v2. `baseline.Baseline` now holds metadata (CreatedAt, CommitSHA, Branch, Description) + `Projects map[string]*ProjectSection`. `ProjectSection` holds `Stats` / `TopMetrics` / `Cycles`. `baseline.New(projects, description)` takes the projects map; `bl.Project(name)` returns the section or nil. `drift.Calculator` retargeted to `*ProjectSection` instead of `*Baseline`. New `drift.SnapshotProjects(allIssues, global, fallback)` builds one `ProjectSection` per project using the same partition/analyzer pipeline as alert computation. `cli_baseline.go#runSaveBaseline` calls it; `runCheckDrift` delegates to `ProjectAlerts` with `bl.Project` as the loader; human drift output tags each alert with its source project. `baseline.Load` rejects v1 baselines with a remediation error per AGENTS.md rule 6 (pre-alpha, no migration path).

### Anchor: partition-only vs satellite nodes

bt-7l5m's wording "each project graph includes its real cross-project edges" was interpreted literally as referring to the *existing* bt-mhwy.5 resolver plumbing used by global-graph subcommands (`insights`, `blocker-chain`, `impact-network`, `graph`), **not** as satellite-node inclusion inside per-project alert analyzers. Per-project alert graphs are pure partitions. Alternative interpretation documented on bt-7l5m and tracked as bt-53vw standing audit hook; switching to satellite nodes later is additive and doesn't break current call sites.

### Scope decisions

- Cross-project external dep resolution (`bt-mhwy.5`) already landed — reused via `rc.analysisIssues()`; not reimplemented.
- Cross-project structural metrics (density, PageRank, cycles spanning projects) explicitly not surfaced in alerts. Deferred work tracked in **bt-46p6.18** (P4, global aggregate metrics) and **bt-46p6.19** (P3, TUI cross-project alert nav).
- Schema v1 baseline files rejected without migration per AGENTS.md rule 6.

### Verify

- `go build ./...`, `go vet ./...`, `go test ./...` all green (incl. 87s e2e)
- `bt baseline save --global` — writes 16-project section snapshot with correct per-project density/PR/stats
- `bt baseline check --global` — surfaces 98 critical / 122 warning / 9 info alerts tagged per project (e.g. `[beads] centrality_change — bd-cxd dropped from top`, `[bt] stale — bt-46p6 inactive for 9 days`)
- `bt robot alerts --global --shape=full | jq '.alerts | group_by(.source_project)'` — buckets cleanly into 14+ projects; cycle path `bt-xavk → bt-ty44 → bt-xavk` correctly attributed to `source_project=bt`
- `bt robot alerts --alert-type=dependency_loop --global` — filter still works; summary totals consistent
- v1 baseline file on disk rejected: `Error loading baseline: baseline at X is schema v1; current is v2. Regenerate with: bt --save-baseline "..."`

### Stream alignment

Slots into ADR-002 under the bt-46p6 cluster (alerts system redesign). Unblocks **bt-46p6.19** (TUI cross-project alert nav, P3, blocked by this). Related open children: **bt-46p6.11** (CLI alert system alignment, P2), **bt-46p6.17** (surface natural-language alert-type definitions, P3, blocked by bt-46p6.4).

### Commits

- `feat(analysis): per-project scoped alert computation (bt-46p6.8)` — 710352d4
- `feat(analysis): baseline schema v2 with per-project sections (bt-46p6.8)` — b4dcd7f6

---

## 2026-04-23 - Event pipeline data layer (bt-nexz, part of bt-d5wr)

**TDD-driven implementation of the activity-event capture pipeline from the footer/notification redesign spec (bt-d5wr). Pure data layer: no UI changes. Produces a tested ring buffer + snapshot diff that later beads (footer redesign, notification modal) consume.**

### What shipped

- **bt-nexz** (P2, feature) — New `pkg/ui/events/` package with `Event`, `EventKind` (Created/Edited/Closed/Commented/Bulk), `EventSource` (Dolt/Cass — Cass reserved), a session-scoped `RingBuffer` (default capacity 500, evict-oldest on overflow, `sync.RWMutex`), and `Diff(prior, next)` that detects creations, explicit closes (open→closed transitions), reopens-as-edits, comment-count increases, and per-field edits with either named (≤3 fields) or aggregated (`+ N fields`) summaries. Comment summaries truncate at 80 runes with `"..."` suffix. Diffs producing >100 events collapse into a single `EventBulk` marker to protect ring retention from bulk migrations. `CollapseForTicker(events, window)` folds same-BeadID+same-Kind runs within a time window into the most recent event with a `+ N fields` aggregate — pure function for ticker rendering. `Model` grows an `events *events.RingBuffer` field initialized via `events.NewRingBuffer(events.DefaultCapacity)` in `NewModel`. `handleSnapshotReady` now captures `wasTimeTravel` before the time-travel reset, then after the snapshot pointer swap emits `events.Diff(old.Issues, new.Issues, now, SourceDolt)` into the ring when `!firstSnapshot && oldSnapshot != nil && m.events != nil && !wasTimeTravel` — gating prevents bootstrap floods and historical-vs-live time-travel spurious events.

### Scope decisions

- Pure data layer only: no footer ticker, no notification modal, no keybindings. Those land in follow-on beads under the bt-d5wr design umbrella.
- `EventSource.SourceCass` enum slot reserved but never emitted in v1 (per spec section 1).
- Actor field populated from `Issue.Assignee` only; actor-based filtering deferred until an attribution model exists.

### Plan-vs-code deviation

- Task 3 of the plan expected `TestDiff_ReopenIsEdit` to pass alongside the created/closed tests, but Task 3's stubbed `editSummary` returns false, so a status-only change (closed→open) emits nothing. Moved the test to Task 4 (where the real `editSummary` lands) so each task's commit stays green. Final state identical to the plan — just a test-ordering fix. Flagged and resolved in-session rather than silently adapting. Also fixed a minor compile error in the plan's Task 5 test: indexing a Go string returns `byte`, not `rune`, so `events[0].Summary[...] != '…'` was decoded via `[]rune(events[0].Summary)` first.

### Verify

- `go build ./...`, `go vet ./...` clean
- `go test ./pkg/ui/events/ -race -count=1` green (25 tests, ~1.3s)
- `go test ./pkg/ui/ -count=1 -timeout 180s` green (22.6s, including 3 new `TestHandleSnapshotReady_*` integration tests)
- `go install ./cmd/bt/` clean

### Stream alignment

Slots into ADR-002 under the bt-d5wr cluster (footer + notification redesign). Unblocks the footer-redesign bead and the notification-center modal bead. bt-d5wr umbrella stays open — closes only when all three implementation beads land.

### Commits

- `feat(events): scaffold events package with Event/EventKind/EventSource types (bt-nexz)`
- `feat(events): add RingBuffer for session-scoped event retention (bt-nexz)`
- `feat(events): Diff emits EventCreated and EventClosed on snapshot compare (bt-nexz)`
- `feat(events): detect edited fields with named/aggregated Summary (bt-nexz)`
- `feat(events): detect new comments and truncate Summary at 80 runes (bt-nexz)`
- `feat(events): cap per-diff emission at 100 events with EventBulk marker (bt-nexz)`
- `feat(events): CollapseForTicker folds same-bead same-kind runs within window (bt-nexz)`
- `feat(ui): attach events.RingBuffer to Model (bt-nexz)`
- `feat(ui): emit activity events from handleSnapshotReady snapshot diff (bt-nexz)`

---

## 2026-04-23 - Footer cluster + mouse integration (bt-y0k7, bt-m9te, bt-d8d1)

**Two work streams closed in one session: footer consolidation (status clobber fix + 4 dogfood polish items) and click-to-focus mouse handling. All from the 2026-04-23 session plan `declarative-seeking-manatee.md`.**

### What shipped

- **bt-y0k7** (P2, bug) — Background-initiated reloads (Dolt poll, file-watcher reload, DataSourceReload) no longer clobber footer key hints. Added `statusIsInline bool` to `Model` and a new `setInlineTransientStatus` helper. `FooterData.Render()` now distinguishes banner mode (errors, user-initiated) from inline mode (overrides `HintText` with a prefixed status string; all other badges/hints remain visible). `handleSnapshotReady`, `handleDataSourceReload`, and the sync file-watcher reload path all route through inline. Full-screen "flash" on snapshot replacement is out of scope — that's a bubbletea redraw concern.

- **bt-m9te** (P2, feature) — Four footer polish deliverables from the 2026-04-23 dogfood compile: **(1)** Idle auto-dismiss via a new recurring `statusTickMsg` (1s cadence, scheduled in `Init`, re-armed in `handleStatusTick`). Moved the render-time dismiss out of `footerData()` — now idle sessions clear expired status without requiring a keypress. **(2)** Project badge smush — added `Background(ColorBgHighlight)` to `projectBadge` so its `Padding(0, 1)` cells render as a visible gutter; dropped the leading tilde since the background demarcates the badge on its own. **(3)** Workspace summary trimmed from `N projects` to `N` in `model_modes.go` — the `📦` icon carries the meaning. **(4)** Width-aware footer compression: replaced the flat assembly with a tiered priority system. Tier 0 (alerts, instance, worker, stats, filter, counts, hints) always keeps. Tier 1 (update, dataset, watcher, phase2) drops first. Tier 2 (workspace, repoFilter, session) drops second. Tier 3 (project, search, sort, wisp, labelFilter) drops third. Measured iteratively until footer fits; no line-wrapping at narrow widths. Test widths bumped from 80/120/160 to 200 for suites asserting presence of tier-1/2 badges.

- **bt-d8d1** (P2, feature) — Mouse click-to-focus in split view's list/detail panes. New `handleMouseClick` in `model_update_input.go` wired into `Update()` via `tea.MouseClickMsg` dispatch. Left-click above the footer row, in `ViewList + isSplitView` — X coordinate compared to list pane boundary (`m.list.Width() + 4` for panel chrome). Left of boundary focuses list and maps Y to a row index (accounting for header + optional search pill offset). Right of boundary focuses detail and refreshes the viewport. Modals, right/middle clicks, non-list modes, single-pane views all pass-through. Wheel scroll was already implemented. Mouse mode is enabled per-view at `model_view.go:163` (`tea.View.MouseMode = tea.MouseModeCellMotion`), not via a program option — the plan's suggested `tea.WithMouseCellMotion()` does not exist in bubbletea v2.

### Scope decisions

- bt-spzz (smarter reload status) is now unblocked by bt-m9te but deferred to a later session.
- bt-x47u (modal footer consistency) untouched — worth extracting the tier system into a shared module when bt-x47u is picked up.
- Ctrl+click and drag-to-resize explicitly skipped per bt-d8d1 scope guards.

### Verify

- `go build ./...`, `go vet ./...` clean after each close
- `go test ./pkg/ui/ -count=1` green (22.7s, new `TestHandleMouseClick_*` tests added)
- `go install ./cmd/bt/` clean

### Stream alignment

Both streams slot into ADR-002 Stream 6 (Polish / dogfooding). No open-decisions table entries touched.

---

## 2026-04-23 - TUI dogfood cluster 1: search UX (bt-imcn, bt-031h, bt-cd3x, bt-i4yn)

**Four closes on the search UX cluster surfaced during the 2026-04-23 dogfood session. Shared surface in `pkg/ui/` list/search wiring, ramping complexity from a one-line string rename to a filter-wrapper architecture. Ships together because each builds on the previous one's context.**

### What shipped

- **bt-imcn** (P3, bug) — Renamed the Bubbles list's `Filter: ` prompt to `Search: ` in `pkg/ui/model.go:NewModel` via `l.FilterInput.Prompt = "Search: "`, overriding the library default (bubbles v2 `list/list.go:217`). Matches the user's mental model: the affordance is a `/` search bar, the footer says "fuzzy/semantic/hybrid search" — the only remaining odd word out was the prompt itself. Other `Filter:` surfaces (label picker, project filter, `setStatus` filter messages, `history.go` active-filter pill) untouched — those are legitimate filter dimensions.

- **bt-031h** (P2, bug) — Added a persistent search indicator that survives focus changes. Problem: Bubbles' internal `titleView` only renders `FilterInput` while `filterState == Filtering`; tabbing to the details pane commits the filter to `FilterApplied` and the input disappears from the list header, leaving the user without any visual signal that the list is still filtered. Fix: new `Model.renderSearchPill(width)` helper in `pkg/ui/model_view.go` that returns a styled `Search: <query>   <visible>/<total> matches` line only when `FilterApplied`. Prepended above the column header in both `renderListWithHeader` (single-pane) and `renderSplitView` (split view). Chose Option B from the bead (condensed pill on applied state) over Option A (always-rendered dimmed input) — keeps the live-editing ergonomics untouched.

- **bt-cd3x** (P3, feature) — `/` now enters search from any pane in the split-view list layout, not just when focus is on the list. Problem: the outer router only forwards keys to `m.list.Update` when `m.focused == focusList`, so pressing `/` in the details pane did nothing. Fix: new `focusBeforeSearch focus` field on `Model` (zero value = `focusList` = sentinel for "no saved focus") and a tight intercept at the top of `handleKeyPress`: when `mode == ViewList`, `isSplitView`, no modal, list not already `Filtering`, and focus isn't the list — save prior focus, switch to `focusList`, return `(m, nil)`. The Update router tail then forwards `/` to the list, which enters `Filtering` with the (bt-imcn) `Search: ` prompt. Restore logic after the list's Update bounces focus back when `FilterState == Unfiltered` (user hit esc, or cleared an applied filter). In split view, focus restoring to details triggers `updateViewportContent` so the detail pane repaints the current selection. Scope guards: skipped when list isn't visible (non-split with `showDetails`), when any modal is open, and when the list is already in `Filtering` (so `/` remains a literal character inside the search input).

- **bt-i4yn** (P2, bug) — Exact-ID matches now land at position 0 across fuzzy, semantic, and hybrid modes via a pre-empt bucket that sits ABOVE the ranker. Dogfood evidence: `cmg` query on the 104-issue dotfiles corpus put `dotfiles-3mm` at position 1 (its body references `dotfiles-cmg`) and buried `dotfiles-cmg` at ~position 13. Root cause: BM25 and semantic similarity treat IDs as ordinary text tokens; body mentions win over actual ID ownership. Fix: new `pkg/ui/id_bucket_filter.go` with `idPriorityFilter(inner list.FilterFunc) list.FilterFunc` wrapper that, for ID-shaped queries (lowercase alphanumeric + `-`/`.`, 2-24 chars, no whitespace), emits every ID-matching item as `list.Rank`s FIRST, sorted `exact > suffix-exact > substring`, then appends the inner ranker's remaining results deduplicated. Non-ID queries (e.g. `pagerank bottleneck`) fall through unchanged. `IssueItem.FilterValue()` reordered to emit `Issue.ID` as the first whitespace-separated token, which lets the wrapper extract the ID from the opaque `targets []string` without ambiguity, and incidentally nudges sahilm/fuzzy to score ID-bearing beads higher on short queries. All four `m.list.Filter = …` assignments wrapped: `pkg/ui/model.go` (initial), `pkg/ui/model_update_analysis.go:60` (semantic-unavailable fallback), and `pkg/ui/model_update_input.go:647/659/668` (ctrl+s semantic toggle branches). Seven tests in `pkg/ui/id_bucket_filter_test.go` lock acceptance criteria including cross-project ambiguous-suffix grouping.

### Scope decisions

- Deferred bt-ba9f, bt-yqh0, bt-d8d1, bt-ox4a per session-start scope guard: independent surfaces, separate session each. bt-i4yn explicitly unblocks bt-ox4a (default-search-mode decision) — with ID bypass in place, the default-mode choice reduces to "which ranker works best for TEXT queries" and decouples cleanly.
- bt-cd3x scoped to split-view only. Non-split `showDetails` hides the list entirely; bouncing focus to a hidden list and showing a filter prompt would be confusing. Can relax later if dogfooding shows demand.
- bt-031h's pill consumes one row inside the list pane. Outer `MaxHeight` will clip the `pageLine` when the pane is at minimum height and the pill is active — accepted tradeoff since query visibility > pagination while searching.

### Verify

- `go build ./...`, `go vet ./...` clean after each close
- `go test ./pkg/ui/ ./pkg/search/ ./pkg/model/ ./pkg/analysis/` green (pkg/ui 22.5s including 7 new id_bucket tests)
- `go install ./cmd/bt/` clean after each close
- Pre-existing `tests/e2e` Windows path-length panic in `copyDirRecursive` unrelated (ADR-002 stream 3, P1, tracked separately)

### Stream alignment

All four beads slot into Stream 6 (Polish / dogfooding) per ADR-002. No open-decisions table entries touched — these were already-decided UX polish. Dogfood session 2026-04-23 continues; cluster 2 (bt-ba9f et al.) is a separate session.

---

## 2026-04-23 - Stream 9 release-engineering gates cleared + vet baseline (bt-ncu7, bt-brid, bt-bntv, bt-lz7d, bt-4f7g)

**Five closes on Stream 9, one re-enable deferral bead filed. Pre-tag gates cleared end-to-end; binaries-only release path ready for a real `v*` tag push.**

### What shipped

- **bt-ncu7** — Replaced two `fmt.Sprintf("%s:%d", host, port)` address builders with `net.JoinHostPort(host, strconv.Itoa(port))` (the canonical stdlib idiom, IPv6-safe via auto-bracketing). Locations: `internal/doltctl/doltctl.go:86` (EnsureServer TCP dial) and `cmd/bt/root.go:870` (post-start wait loop). Both files already imported `net` and `strconv`; zero import churn. `go vet ./...` now exits clean — warnings had been flagged since 2026-03-16 (110b33d9), surfaced during bt-yqgn agent verification, unactioned until now.
- **bt-brid** — Decision closed with Option 2: strip `brews:` and `scoops:` blocks from `.goreleaser.yaml`, drop `HOMEBREW_TAP_GITHUB_TOKEN` env from `.github/workflows/release.yml`. Reasoning: no user demand signal for brew/scoop install paths, maintaining two external publishing repos preemptively adds CI surface without benefit, and the `HOMEBREW_TAP_GITHUB_TOKEN` secret wasn't even verified in repo secrets. Full re-enable checklist preserved in **bt-zgzq** (P4, new) for when bt hits the subjective v1 bar (dogfood-clean TUI, feature-complete to maintainer's standard).
- **bt-bntv** — Closed as not-applicable. The broken brew formula `test:` stanza (`bt --version` flag vs `bt version` subcommand) disappeared when `brews:` was stripped. The one-line fix is documented in bt-zgzq step 7 so it gets applied automatically when brew publishing resumes — won't be re-discovered as a new bug.
- **bt-lz7d** — Migrated `.goreleaser.yaml` to v2 format: added `version: 2` header, changed `archives[0].format: tar.gz` → `archives[0].formats: [tar.gz]`, removed `snapshot.name_template` and added `snapshot.version_template: "{{ .Tag }}-next"` per v2 deprecation guidance. Pinned `.github/workflows/release.yml` goreleaser action `version: latest` → `version: "~> v2.15"` with a deliberate-bump rationale comment. `goreleaser check` now exits clean (was flagging 4 warnings). Scope-guarded against the `brews:` → `homebrew_casks:` schema migration — that's deferred to bt-zgzq where it can get its own verification pass.
- **bt-4f7g** — Fixed double-v in `bt version` snapshot output by switching ldflags from `-X ...version=v{{.Version}}` to `-X ...version={{.Tag}}` (Option A per bead recommendation). Root cause: goreleaser v2's snapshot mode resolves `.Version` with a `v` prefix already present, so the literal `v` in the template compounded to `vv0.0.0-next`. `.Tag` carries the `v` prefix consistently across snapshot + release modes. Added inline comment documenting the why. Option B (Go-side normalization) considered and rejected — over-engineering for a cosmetic issue.
- **bt-zgzq** (new, P4) — Re-enable brew tap + scoop bucket publishing post-v1. Captures the full restoration checklist: create both external repos, add `HOMEBREW_TAP_GITHUB_TOKEN` secret, restore `brews:`/`scoops:` blocks (migrated to `homebrew_casks:`), restore env in workflow, fix the brew test stanza. Works through a pre-release tag before cutting real v1.

### Smoke-test results

`goreleaser release --snapshot --clean` run against the migrated config:

- Build: 5 binaries (linux/darwin/windows × amd64/arm64, minus windows/arm64), ~8s total (down from ~40s in the 2026-04-22 baseline because no brew/scoop work).
- Artifacts: 5 tar.gz archives (`bt_v0.0.0-next_<os>_<arch>.tar.gz`), `checksums.txt`, `artifacts.json`, `metadata.json`, `config.yaml`.
- `./dist/homebrew/` and `./dist/scoop/` do not exist (strip confirmed).
- `./dist/bt_windows_amd64_v1/bt.exe version` → `bt v0.0.0` (single-v, no double-v regression).
- `goreleaser check` → `1 configuration file(s) validated` (was 4 warnings).
- `go build ./...`, `go vet ./...`, version tests → all clean.

### ADR-002 Stream 9

Status updated from "Pipeline wired, pre-tag gates open" → **DONE**, with all five bead outcomes recorded inline and the bt-zgzq re-enable bead linked. Real tag push now triggers GitHub Release + 5 cross-compiled archives + checksums only, no external package-manager publish.

### Risk

Low. Five file changes total across two files (`.goreleaser.yaml`, `.github/workflows/release.yml`), plus two one-line Go edits for bt-ncu7. All exercised end-to-end via the snapshot smoke test. The publishing strip replaces undefined behavior (tap/bucket repos + token unverified since rename) with defined behavior (binaries-only releases).

### Commits

- `1b51a02d` fix(infra): use net.JoinHostPort for IPv6-safe address formatting (bt-ncu7)
- (this session) chore(infra): strip brew/scoop, migrate goreleaser v1→v2, fix ldflags (bt-brid, bt-bntv, bt-lz7d, bt-4f7g)

---

## 2026-04-22 - Blurb v2, JSONL sunset, refactor epic closeout (bt-yqgn, bt-jlp, bt-if3w)

**Two shipped beads plus an epic close-out. Agents dispatched for in-flight work; decision debt on the pkg/ui refactor epic cleared.**

### What shipped

- **bt-yqgn** — `pkg/agents/blurb.go` rewritten v1 → v2. New markers (`<!-- bt-agent-instructions-v2 -->`), dual-family recognition (both `bv-` v1 and `bt-` v2 markers detected so v1-installed projects upgrade on next run), content fully rewritten for 2026-era `bt robot <subcmd>` surface (triage/next/portfolio/pairs/refs/plan/impact/insights/…), `--global`/`--shape`/`--compact`/`--full`, `$CLAUDE_SESSION_ID`, `bd prime`, mandatory reads, positional `bd create`, atomic `--claim`, Summary/Change/Files/Verify/Risk/Notes close template, project P0–P4 table, correct sync idiom (`bd dolt push && git push`). Five new tests: TestUpgradeV1ToV2, TestFreshInstallIsV2, TestAgentBlurbNoBdSync (regression guard), TestAgentBlurbNoLegacyBvMarkers, TestBlurbMarkersAreV2. Agent-dispatched; verified build + vet + `go test ./pkg/agents/...` clean before commit.
- **bt-jlp** — JSONL persistence audit for the beads v1.0.1 opt-in migration. Classified as Case A (redundant): both Dolt and JSONL were active, `refs/dolt/data` present on origin, JSONL auto-export actively failing with `auto-export: git add failed: exit status 1` on every bd write. `export.auto=false`, `.beads/issues.jsonl` untracked via `git rm --cached` + gitignore pattern. Dolt is now canonical. Parent `dotfiles-jlp` epic updated with bt status.
- **bt-if3w epic closed (re-scoped)** — Phases 0–3 + 1.5 shipped the refactor value (mechanical Charm v2 migration, test foundation, Model decomposition with ViewMode/DataState/FilterState/ModalType enums + Update() split, AdaptiveColor kill across 174 occurrences, Cobra migration main.go 1708→13 lines + 35+ robot subcommands, footer extraction). Residual hygiene decoupled from the epic frame and tracked as standalone P2 polish beads: `bt-t82t` (Phase 4 cleanup — stale `bv-` refs + golden regen + test validation) and `bt-if3w.1` (sprint view extraction, same pattern as the closed bt-oim6 footer). Hot-path style pre-computation **cut** (YAGNI — noted as "needs profiling first", no profiling evidence exists).
- **bt-bo4a gate closed** — 8-day-old "what's Phase 4 vs what gets cut" gate resolved with the scoping decision above. Decision debt cleared.

### ADR-002 Stream 4

Updated to reflect epic closure and the decoupled follow-up beads. Stream 4 is now fully DONE at the stream level; the two open children compete against the broader backlog on their own merits rather than inheriting urgency from the epic frame.

### Risk

Low. bt-yqgn is content + version bump with backward-compat marker detection. bt-jlp removed a broken code path (JSONL export was already erroring on every write). bt-if3w closure is a re-scope of status, not a code change.

### Commits

- `b76f2a1` chore(data): disable JSONL export, Dolt is canonical (bt-jlp)
- `72544630` docs(agents): rewrite blurb v1 -> v2 (bt-yqgn)
- `02c46407` docs(adr): close bt-if3w epic + cut hot-path styles; changelog for 2026-04-22

### Release-readiness findings (goreleaser snapshot smoke test)

Ran `goreleaser release --snapshot --clean` locally (goreleaser v2.15.4 via `go install`) to verify the release pipeline works post-rename. Cross-compile succeeded for linux/darwin/windows × amd64/arm64 (5 binaries, ~14 MiB each, ~40s). Archives + checksums + brew formula + scoop manifest all generated to `./dist/`. But four real issues surfaced that would bite a real tag push:

- **bt-bntv** (P2, bug) — brew formula `test:` stanza calls `bt --version`. bt uses `bt version` (subcommand), not a flag. Would fail `brew test` on formula validation.
- **bt-4f7g** (P3, bug) — `bt version` prints `bt vv0.0.0-next` (double-v) under goreleaser snapshot mode. Root cause: ldflags template `v{{.Version}}` combined with goreleaser v2's snapshot semantics where `.Version` already carries the `v`. Cosmetic; real tag builds likely fine but the template is fragile.
- **bt-lz7d** (P2, task) — `.goreleaser.yaml` is v1-format; installed goreleaser is v2 and flags three deprecations (`snapshot.name_template`, `archives.format`, `brews` → `homebrew_casks`). Snapshot still succeeds, but `.github/workflows/release.yml` pins `version: latest`, so a future goreleaser major release could break CI without notice.
- **bt-brid** (P2, task) — `.goreleaser.yaml` configures publishing to `seanmartinsmith/homebrew-tap` and `seanmartinsmith/scoop-bucket` repos. Neither has been verified to exist since the rename, and `HOMEBREW_TAP_GITHUB_TOKEN` hasn't been confirmed in repo secrets. Decision bead: publish for v0.1, strip for v0.1 and defer, or full-publishing later. Recommended strip for v0.1 (no users yet requesting brew/scoop install).

All four are cross-linked via `relates-to`. No code changes made this session — findings filed for separate decisions/fixes before any `git push --tags v*` actually happens.

---

## 2026-04-21 - Pairs/refs v2 docs + labeled corpus + FPR gate (bt-vxu9, Phases 4-5)

**Closed the pairs+refs v2 plan: expanded the bt-side convention + design docs into cold-readable references, landed a 32-issue labeled corpus with a pre-commit sanitization gate, and shipped an FPR threshold test asserting the v2 readers stay under their agreed-on false-positive budgets.** Phases 4 + 5 + 6 of the plan; closes bt-vxu9.

### What shipped

- **`.beads/conventions/cross-project.md`** — Phase 1 skeleton replaced with per-mode sigil vocabulary, intent_source/sigil_kind semantics, when-to-pin `--schema=v1` scenarios, full invocation examples (default, v1 pin, sigil modes, `--orphaned`, env-var forms), and migration guidance pointing at bt-xgba for removal.
- **`docs/design/2026-04-21-pairs-refs-v2.md`** — authoritative labeling rubric, BFS-over-connected-components rationale (vs union-find at N=400), per-mode sigil tables with concrete prose examples, "why schema-bump not envelope-additive", "why dep-edge as sole signal", "why verb default", rollback semantics, and file:line cites to shipped code. Flagged one dead-code observation (`hasVerbWithinProximity` at pkg/analysis/sigils.go:546 is unreferenced).
- **`scripts/audit-corpus.sh`** — 117-line jq-based pre-commit scanner. Denylist: password/secret/token/api_key, AWS/GitHub/Slack token shapes, `.env`, `localhost:port`, Windows/macOS user paths, emails outside the sms@seanmartinsmith.com / @users.noreply.github.com allowlist. Hard-requires jq (no grep-over-JSON fallback — would false-positive on key names).
- **`pkg/view/testdata/corpus/labeled_corpus.json`** — 32 sanitized issues modeled on real shared-Dolt state. 11 candidate pair records (5 intentional: byk 4-way / zsy8 / 2il / x08 / fjip 3-way; 6 suffix-collision negatives: 153 / 1hk / 52t / 9c9 / cv6 / dyg). 27 labeled ref candidates exercising each sigil mode with placeholder negatives (`bt-xxx`) and English-slug negatives (`-only`, `-side`) that strict/verb correctly suppress.
- **`pkg/view/fpr_gate_test.go`** — three subtests: corpus-load (malformed fixture / missing truth / N<10 candidates / memory-delta >10 MiB all fail loudly); pair.v2 FPR gate at <10%; ref.v2 broken-flag FPR gated at ≤5% under verb mode with strict + permissive reporting informational readouts.

### Measured numbers

Baseline vs v2 on the labeled corpus:
- **Pair FPR**: v1 ~83% (~24 of 29 dogfood pairs were suffix collisions) → **v2 0.00%** (5/5 emitted records intentional, far under 10% gate).
- **Ref broken-flag FPR**: v1 ~30% (dogfood baseline) → **v2 0.00% across all three modes** (strict: 9 records total, 1 broken, 0/1 FP; verb: 21 records, 1 broken, 0/1 FP; permissive: 26 records, 1 broken, 0/1 FP).

The single broken ref in each mode is the intentionally-broken `external:bd:nonexistent` dep on `bt-refbroken` — detector is correctly flagging a genuinely dangling external ref.

### Risk

Low. Phase 4 is docs-only; Phase 5 adds one test file + one fixture + one shell script. No production code touched. FPR gate is additive and runs alongside existing goldens; breakage would fail `go test ./pkg/view/` loudly without affecting readers.

### Deferred follow-ups

- **bt-xgba** (P2): remove `--schema=v1` fallback one release after ship. Filed Phase 3.
- **--explain-refs observability mode** (bt-113x, P3, discovered-from bt-vxu9): emits rejection reasons per prose candidate span so FPR regressions are debuggable.
- **`pkg/analysis/sigils.go:546` dead code** — unreferenced `hasVerbWithinProximity` helper from an earlier draft; actual resolution is inlined in `processLineSigil`. Flagged in bt-vxu9 close notes, not a separate bead (trivial cleanup).

---

## 2026-04-21 - Refs v2 + sigils detector + default schema flip (bt-vxu9)

**Ship the ref.v2 reader with a hand-rolled sigil tokenizer, and flip the default `--schema` from v1 to v2 for both `bt robot pairs` and `bt robot refs`.** Both v2 readers now live side-by-side with their v1 counterparts; `--schema=v1` remains as an opt-in fallback. Phase 3 of bt-vxu9 / bt-gkyn.

### What shipped

- **`pkg/analysis/sigils.go`** — hand-rolled iterative tokenizer. `DetectSigils(body, mode)` walks bytes once, emits `SigilMatch{ID, Kind, Offset, Truncated}`. Three recognizer sets:
  - **strict**: markdown links `[bead-id](url)`, inline code `` `bead-id` ``, `ref:` / `refs:` keyword (case-insensitive, optional single space).
  - **verb**: strict + fixed verb list (`see`, `paired with`, `blocks`, `closes`, `fixes`, `mirrors`) with same-line 32-char inclusive proximity. Markdown format chars (`*`, `_`, `~`) stripped before proximity counts.
  - **permissive**: every bead-shaped ID outside fenced or inline code emits `bare_mention`.
  - Fence stack capped at 32 frames; 1 MiB per-body cap with `truncated: true` flag propagated to every match.
  - Verb-proximity post-pass uses a two-pointer sliding window so the proximity resolver is O(verbs + ids + matches) rather than O(V × I) — caught by the linear-scaling test on 100 KB inputs.
- **`pkg/view/ref_record.go`** — `ComputeRefRecordsV2`, `RefRecordV2`, `RefRecordSchemaV2 = "ref.v2"`. Prose scan delegates to `analysis.DetectSigils` under the caller-chosen mode; each prose field is wrapped in `defer recover()` so one adversarial body logs + skips rather than crashing `--global` for the rest of the corpus. v1's cross-project-only filter is preserved — the load-bearing FP guard. `SigilKind` on each record carries the sigil that established intent; `Truncated` flags records from oversize bodies. Dep-derived records use new `external_dep` / `bare_dep` kinds.
- **`cmd/bt/robot_refs.go`** — `refsOutputV2` and `emitRefsV2` mirror the pairs v2 inline-dispatch pattern. The Phase 1 stub `--schema=v2 not yet implemented` is gone.
- **`cmd/bt/robot_output.go`** — envelope gains optional `sigil_mode` (omitted when empty). Existing envelope goldens stay byte-identical because the field is omitempty.
- **`cmd/bt/robot_schema_flag.go`** — `robotSchemaDefault` flipped from `robotSchemaV1` to `robotSchemaV2`. This is the coordinated default-flip both v2 readers were waiting on.
- **`.beads/conventions/cross-project.md`** — "Phase 1 default: v1" language replaced with "Default: v2 as of Phase 3 of bt-vxu9".

### Tests

- `pkg/analysis/sigils_test.go` — 28 unit tests per sigil kind + full pathological set (invalid UTF-8, lone surrogates, RTL override, ZWJ, 100 KB single-line, 100 K inline-code storm, 10 K nested fences, 10 K link sequence, 1 MB boundary, adversarial combined sweep). Linear-scaling test asserts runtime(100 KB) ≤ 15 × runtime(10 KB). Four benchmarks (`Benchmark_DetectSigils_{10KB,100KB,PathologicalNestedFences,PathologicalInlineCodeStorm}`).
- `pkg/view/ref_record_test.go` — per-mode unit tests: strict markdown link / inline code / ref keyword; verb proximity inclusive-32 boundary; markdown-stripping (`**see**`); multiple-verbs-one-ID collapsed; permissive bare mention + inline-code suppression; fenced-code suppressed in all modes; cross-project filter; external-dep broken/resolved; truncated flag on >1 MB body; sorted output; flag-order invariant.
- `pkg/view/testdata/fixtures/ref_v2_*.json` + matching goldens — 6 fixtures covering each mode × 2 scenarios; registered in `TestRefRecordV2Golden` with mode inferred from filename prefix.
- `cmd/bt/robot_refs_test.go` — `TestRefsOutputV2_SchemaAndSigilMode`, `TestRefsOutputV2_SigilKindPresent`, `TestRefsOutputV2_CrossProjectOnly`, `TestRefsOutputV2_EmptyReturnsArray`, `TestRobotRefs_SigilModesResolveClean`, `TestRobotRefs_EnvVarOverride`. Existing validator tests updated for the flipped default.

### Smoke (real shared Dolt corpus)

- `bt robot refs --global` default → `schema: "ref.v2"`, `sigil_mode: "verb"`, 108 records.
- `bt robot refs --global --schema=v1` → `ref.v1`, 531 records (unchanged).
- `bt robot refs --global --sigils=strict` → 57 records (inline_code only on this corpus; 0 slug FPs for `-only`/`-side`/`-show`/`-level`).
- `bt robot refs --global --sigils=permissive` → 485 records (still < v1 because fenced + inline code now excluded).
- `BT_SIGIL_MODE=strict bt robot refs --global --sigils=verb` → verb wins.
- `bt robot pairs --global` → `pair.v2`, 22 records (default flipped).
- Total `--global` runtime ~175 ms, well under the 800 ms budget.

### Risk

Low. Both v1 readers untouched and test-covered. v2 readers isolated behind dispatch; `--schema=v1` retains the frozen wire shape for one release. Panic safety at the call site is belt-and-braces — the adversarial test sweep confirms `DetectSigils` itself doesn't panic, but the wrapper guards against future changes or unexpected gremlins in real data.

### Follow-ups open

- Phases 4 + 5 + 6 shipped in a follow-up session (2026-04-21 evening). See the next entry.
- Remove `--schema=v1` fallback — bt-xgba triggers one release after this ship.

---

## 2026-04-21 - Source filter (bt-mhwy.6)

**New `--source <project>[,<project>...]` persistent robot flag scopes output to one or more source projects.** Agents can now ask "show me only bt + cass beads" without BQL or workspace-level setup. Closes bt-mhwy.6, the last of seven children of the mhwy epic.

### What shipped

- **`--source` persistent flag** on `robotCmd` (cmd/bt/cobra_robot.go). Comma-separated projects; case-insensitive. Surfaced as `query.source` in the `robot list` envelope echo.
- **`filterBySource` helper** (cmd/bt/helpers.go). Exact project-name matching on either the ID prefix (`SplitID(issue.ID)`) or the `SourceRepo` field. Unknown projects produce empty results (silent — the plan's stated behavior).
- **Applied in two places**: inside `robotPreRun()` before the BQL / label-scope / recipe chain so `triage`, `next`, `insights`, `plan`, `priority`, `alerts`, `suggest`, etc. all filter consistently; inside `robot list`'s `RunE` separately because list bypasses `robotPreRun` and reads `appCtx.issues` directly. The compact reverse-map computation also uses the source-filtered slice so `children_count` / `unblocks_count` reflect the scoped graph.
- **Docs in `robot_help.go`** — `--source` section explains the flag, notes the `source_repo` field surfaces in compact.v1 / pair.v1 / portfolio.v1, and shows two agent-friendly examples. The committed JSON schemas at `pkg/view/schemas/*.v1.json` already document `source_repo`, so the schema-side story is complete without new machinery.

### Scope note

bt-mhwy.6 shrank over the course of the mhwy series: `source_repo` was already surfacing in compact (.1), portfolio (.4), and pair (.2) output before this task ran, so .6 is primarily a CLI ergonomics feature rather than a data-exposure feature. The 2026-04-21 color comment on the bead spelled this out.

### Tests

- `cmd/bt/helpers_source_test.go` — 8 unit cases: empty filter, single prefix, comma-separated, unknown prefix yields empty, SourceRepo fallback, case-insensitive, whitespace trimming, empty tokens dropped.
- `cmd/bt/robot_source_test.go` — 3 contract cases via the real binary: `--source=cass` filters down + echoes `query.source`, `--source=nonexistent` exits 0 with `count: 0`, `--source=bt,cass` matches both.

### Smoke

`bt robot list --source=bt --limit=3` returns 3 bt-* beads with `query.source: "bt"`. `--source=nonexistent` returns `count: 0` cleanly. `--source=cass,bt --limit=5 --global` mixes cass and bt records as expected. `bt robot triage --source=cass --global` scopes quick_ref counts to cass.

### Cross-project constellation (status)

**7 of 7 mhwy children closed.** Epic bt-mhwy delivered: compact output (.1), external dep resolution (.5), portfolio (.4), pairs (.2), refs (.3), source filter (.6). Remaining follow-ups: bt-gkyn (pairs v2 intent identity), bt-vxu9 (refs v2 sigil identity).

---

## 2026-04-21 - Refs subcommand (bt-mhwy.3)

**New `bt robot refs --global` subcommand validates cross-project bead references in prose and dep fields.** Scans description, notes, comments, and unresolved `external:` dependencies for bead IDs whose prefix differs from the source's project, validates each against the global set, and emits flags for `broken` / `stale` / `orphaned_child`. Agents can now see stale cross-refs that rot silently.

### What shipped

- **`pkg/view/ref_record.go`** — new projection (`ref.v1`). `ComputeRefRecords(issues)` is a pure function: builds a known-issues map + known-prefix set + closed-parent-of-open-child map once, then scans each source issue's deps (unresolved `external:` form only — resolved ones become normal graph edges via `rc.analysisIssues()`), description, notes, and joined comments. Records sort by `(Source, Target, Location)` for diff stability. `RefRecord` carries `{source, target, location, flags}` with location ∈ `{description, notes, comments, deps}`.

- **Cross-project only in v1** — same-prefix refs are intra-project and already handled by the dep graph. This is a scope tightening from the AC's literal "scan all refs" — eliminates the largest false-positive class (suffix collisions within a project).

- **Prefix scoping** — on top of the cross-project filter, prose matches are restricted to prefixes present in the loaded issue set. Slug-shaped tokens like `round-trip`, `per-issue`, `cross-project`, `batch-closing` split into valid `(prefix, suffix)` but their "prefix" corresponds to no known project, so they drop out. Dogfooded: this cut the broken-ref false-positive rate from ~85% (naive regex) to ~30% (residual comes from placeholder text like `bt-xxx`, `cass-xyz` in docs).

- **Flag order (fixed, for diff stability)**: `broken`, `stale`, `orphaned_child`, `cross_project`. Every v1 record carries `cross_project` because v1 only emits cross-project refs — kept explicit so v2 can relax the identity rule without changing the flag's meaning.

- **URL stripping** — `https?://...` spans get replaced with whitespace before the regex runs, so `github.com/foo-bar/baz` doesn't fire `foo-bar`. Markdown-aware parsing (code blocks, inline code) deferred to v2.

- **`cmd/bt/robot_refs.go`** — `rc.runRefs()` handler with a pure `refsOutput()` helper that builds the wire payload without hitting stdout or `os.Exit`. Same `BT_TEST_MODE=1`-driven pattern as pairs: binary-level `--global` tests can't run through Dolt discovery, so the helper lets in-process contract tests exercise the projection end-to-end.

- **Mandatory `--global`** — without cross-project data, ref detection is definitionally empty. Exits 1 with a clean error message rather than silently emitting `[]`.

- **Cobra wiring** in `cmd/bt/cobra_robot.go` — `robotRefsCmd` registered alongside `pairs`. No new flags. `--shape` is inherited but no-op (envelope.schema is unconditionally `ref.v1`).

Full rationale: `docs/design/2026-04-20-bt-mhwy-3-refs.md`.

### Tests

- `pkg/view/ref_record_test.go` — 18 unit cases: empty/nil safety, same-prefix skipped, cross-project found, broken, stale, orphaned_child, external-dep resolved vs broken, dedup within a single location, multiple locations per source, malformed IDs silently skipped, URL stripping, word boundaries (`see bd-a.`, `(bd-a)`, `bd-a,` match; `abt-a`, `x-bd-a` don't), dotted suffix (`bd-mhwy.2`), flag-order across multiple co-occurring flags, deterministic sorting, schema constant, unknown-prefix skipped.
- `pkg/view/projections_test.go` — 4 new golden fixtures exercised via `TestRefRecordGolden`: `ref_empty`, `ref_single_broken`, `ref_mixed` (broken + stale + orphaned_child + valid), `ref_external_deps` (resolvable vs broken `external:` forms). Plus `TestRefRecordSchemaFileExists` for the committed JSON Schema.
- `cmd/bt/robot_refs_test.go` — contract: `--global` enforcement (binary), envelope required fields, `schema == "ref.v1"`, cross-project-only (intra-project ref in fixture must not leak), flag order across the full fixture, empty array on no refs, deterministic ordering.
- `cmd/bt/robot_all_subcommands_test.go` — refs added to flag-acceptance matrix (4 permutations; `--global` carried).

### Smoke

`bt robot refs --global` against the real shared server returns 408 ref records across 20+ projects: 119 `broken`, 116 `stale`, 21 `orphaned_child`. Residual false-positive rate on `broken` (~30%) comes from placeholder text in docs (`bt-xxx`, `cass-xyz`, etc.) and slug-like suffixes under known prefixes (`-only`, `-side`, `-show`). Stale and orphaned_child flags are high-signal because the target actually exists — agents can filter to those for a cleaner pass. Sigil-based v2 identity (require `ref:` keyword or markdown link) is the natural next step and the follow-up bead filed in this session.

### Cross-project constellation (status)

Shipped in this session series: **bt-mhwy.5** (external dep resolution) → **bt-mhwy.4** (portfolio) → **bt-mhwy.2** (pairs) → **bt-mhwy.3** (refs). Six of seven mhwy children now closed. Remaining: **bt-mhwy.6** (provenance `--source` filter — scope shrunk by earlier children: `source_repo` already surfaces in compact/portfolio/pair output, so .6 is primarily the filter flag + schema docs).

---

## 2026-04-20 - Pairs subcommand (bt-mhwy.2)

**New `bt robot pairs --global` subcommand surfaces cross-project paired beads.** When `bt-zsy8` and `bd-zsy8` describe the same logical work from two projects, pairs detects the relationship, picks a canonical, and reports which dimensions have drifted. Agents can create paired IDs today (`bd create --id=<prefix>-<suffix>`) — this is the missing read path.

### What shipped

- **`pkg/view/pair_record.go`** — new projection (`pair.v1`). `ComputePairRecords(issues)` is a pure function: buckets by ID suffix, filters to groups with ≥2 distinct prefixes, sorts by `CreatedAt` (tie-broken by prefix, then full ID) to pick canonical, compares mirrors against canonical for drift, returns records sorted by suffix. `PairRecord` carries `{suffix, canonical, mirrors, drift?}`; `PairMember` is a 5-field compact slot (id/title/status/priority/source_repo).

- **Drift dimensions (v1, fixed output order)**: `status`, `priority`, `closed_open`, `title`. `closed_open` is a sharper sub-signal that always co-occurs with `status` when either side straddles the closed boundary, letting agents filter directly for "one side shipped, other didn't." Title drift is exact string equality — deliberate "no fuzzy" interpretation of the AC (matches bt-mhwy.5's philosophy).

- **`pkg/analysis.SplitID`** (promoted from private `splitID`) — pair detection is the second legitimate consumer of the same primitive. Reimplementing in `pkg/view` would have risked the two definitions drifting. Exporting costs nothing: one private call site, one test, mechanical rename.

- **`cmd/bt/robot_pairs.go`** — `rc.runPairs()` handler with a pure `pairsOutput()` helper that builds the wire payload without hitting stdout or `os.Exit`. The separation lets in-process contract tests exercise the projection end-to-end: binary-level `--global` tests can't run in `BT_TEST_MODE=1` (which deliberately blocks Dolt discovery).

- **Mandatory `--global`** — pair detection is definitionally empty without cross-project data; invoking without `--global` exits 1 with a clean error message rather than silently emitting `[]` (which would collide with the legitimate "no pairs exist" signal). With `--global` set and no pairs in the data, emits `"pairs": []` and exits 0 per the AC.

- **Cobra wiring** in `cmd/bt/cobra_robot.go` — `robotPairsCmd` registered alongside portfolio. No new flags. `--shape` is inherited but no-op (envelope.schema is unconditionally `pair.v1`).

Full rationale: `docs/design/2026-04-20-bt-mhwy-2-pairs.md`.

### Tests

- `pkg/view/pair_record_test.go` — 10 unit cases: empty/nil safety, in-sync single pair, drift across each dimension + fixed output order, 3-way pair ordering, canonical tie-break, same-prefix anomaly dropped, malformed IDs silently skipped, dotted suffix (`bt-mhwy.2` + `bd-mhwy.2`), records sorted by suffix, schema constant.
- `pkg/view/projections_test.go` — 4 new golden fixtures via `TestPairRecordGolden`: `pair_empty`, `pair_single_in_sync`, `pair_single_drifted`, `pair_multi_way`. Plus `TestPairRecordSchemaFileExists` for the committed JSON Schema.
- `cmd/bt/robot_pairs_test.go` — contract: `--global` enforcement (binary), envelope required fields, `schema == "pair.v1"`, drift detection across all 4 dimensions, empty array on no pairs, pairs sorted by suffix, canonical = first-created with prefix-sorted mirrors.
- `cmd/bt/robot_all_subcommands_test.go` — pairs added to flag-acceptance matrix (4 permutations; `--global` carried since pairs requires it).

### Smoke

`bt robot pairs --global` against the real shared server returns 29 paired sets across 14+ projects, including the known `bt-zsy8`/`bd-zsy8` pair. 24 of the 29 surface drift — mostly title divergence (expected: mirrored beads typically get project-scoped titles). Three-way pairs (`byk`, `fjip`, `g5q`) surface cleanly with canonical + 2 mirrors.

### Cross-project constellation (status)

Shipped in this session series: **bt-mhwy.5** (external dep resolution) → **bt-mhwy.4** (portfolio) → **bt-mhwy.2** (pairs). Next in sequence per the epic: `bt-mhwy.3` (refs — cross-project reference resolution, consumes the pair pattern), then `bt-mhwy.6` (provenance surfacing).

---

## 2026-04-20 - Portfolio subcommand (bt-mhwy.4)

**New `bt robot portfolio` subcommand answers "which project needs attention?" at the org level.** One PortfolioRecord per project with counts, priority breakdown, velocity with trend, composite health score, top blocker, and stalest issue.

### What shipped

- **`pkg/view/portfolio_record.go`** — new projection (`portfolio.v1`). `ComputePortfolioRecord(project, projectIssues, allIssues, pagerank, now)` is a pure function; `allIssues` lets the Blocked count see cross-project blockers under `--global` after bt-mhwy.5 external dep resolution.

- **Shared reverse-map helpers** — extracted `buildChildrenMap`, `buildUnblocksMap`, `buildOpenBlockersMap` from `CompactAll`'s single-pass loop. Both `CompactAll` and `ComputePortfolioRecord` consume them; behavior-identical refactor (CompactIssue golden fixtures unchanged).

- **`cmd/bt/robot_portfolio.go`** — `rc.runPortfolio()` handler. Groups issues by `SourceRepo` under `--global`; single-project mode emits exactly one record keyed by `rc.repoName` (falls back to a uniform SourceRepo, then `"local"`). Empty SourceRepo in global mode buckets to `"unknown"` so agents never lose data.

- **Cobra wiring** in `cmd/bt/cobra_robot.go` — `robotPortfolioCmd` registered alongside other robot subcommands. No new flags. `--shape` is inherited but no-op (envelope.schema is unconditionally `portfolio.v1` because the payload IS a versioned projection).

### Design

- **Health formula**: equal-weight mean of `closure_ratio`, `(1 − blocker_ratio)`, `(1 − stale_norm)` with clamping to `[0,1]` and 3-decimal rounding. Simple, explainable, no magic weights.
- **Trend classifier**: recent 2-week window vs prior 4-week window normalized to 2-week-equivalent, with ±20% thresholds — smoother than raw week-over-week.
- **Top blocker**: PageRank among project-scoped open/in_progress issues with `unblocks_count > 0` — excludes isolated leaves with high PageRank that aren't holding anyone hostage.

Full rationale in `docs/design/2026-04-20-bt-mhwy-4-portfolio.md`.

### Tests

- `pkg/view/portfolio_record_test.go` — unit tests for empty project, counts, trend classifier with boundary cases (±20%), health-score formula, top-blocker isolated-leaf filter, stalest selection.
- `pkg/view/projections_test.go` — 4 new golden fixtures exercised via `TestPortfolioRecordGolden`: empty, single healthy, single unhealthy, multi-project (cross-project blocker).
- `cmd/bt/robot_portfolio_test.go` — contract: envelope shape, `schema == "portfolio.v1"` across all `--shape` variants, `--shape=compact` ≡ `--shape=full` byte-identical (no-op), single-project mode returns exactly one record, projects sorted by name.
- `cmd/bt/robot_all_subcommands_test.go` — portfolio added to the flag-acceptance matrix (4 permutations).

### Smoke

`bt robot portfolio --global` ranks 15 real projects side-by-side with sensible health scores (0.464–0.985), per-project trends, and cross-project TopBlocker detection.

---

## 2026-04-20 - Compact output for robot subcommands (bt-mhwy.1)

**Default `bt robot list` output shape changes from full issues to compact projections.** 3 commits, 1 new package (`pkg/view/`), 1 bellwether integration, 1 compact projection for `robot diff`, 70+ new tests.

### Breaking change (pre-alpha)

- **Default `bt robot list` shape is now `compact`.** Full-body output is opt-in via `--full` (or `--shape=full`, or `BT_OUTPUT_SHAPE=full`). Rationale: `bt robot list --global` dropped from 383KB to 38KB on a 100-issue sample (~90% reduction) — agents were burning context windows on `description`/`design`/`acceptance_criteria`/`notes`/`comments`/`close_reason` bodies they never read.

### What shipped

- **`pkg/view/` package** (Commit 1) - Home for graph-derived consumer-facing projections. `CompactIssue` is the first resident. Ships with a reusable golden-file harness (`projections_test.go`), a committed JSON Schema (`schemas/compact_issue.v1.json`), and projection-pattern conventions in `doc.go`. Future projections (portfolio records, pair records, reference records) follow the same file-per-projection, schema-versioned pattern.

- **`robot list` bellwether** (Commit 2) - Persistent `--shape` / `--compact` / `--full` flags on `robotCmd` (inherited by every subcommand) with `BT_OUTPUT_SHAPE` env var. New `schema` field on `RobotEnvelope` (`omitempty`) carries `"compact.v1"` in compact mode and is absent in full mode, keeping `--full` byte-identical to pre-change output. Compact projection computed over the full pre-filter issue set so reverse-graph counts (`children_count`, `unblocks_count`, `is_blocked`) reflect the real graph regardless of `--status` / `--priority` / `--type` / `--has-label` / `--limit`.

- **`robot diff` compact projection** (Commit 3) - Projects the four `[]model.Issue` slots on `analysis.SnapshotDiff` (`new_issues`, `closed_issues`, `removed_issues`, `reopened_issues`) into `[]view.CompactIssue` when `shape=compact`. Reverse-graph counts computed over the UNION of historical and current issues so `children_count` / `unblocks_count` / `is_blocked` stay accurate across snapshots. `--full` keeps the original `*analysis.SnapshotDiff` wire shape.

- **15 other robot subcommands** - `triage`, `next`, `insights`, `plan`, `priority`, `alerts`, `search`, `suggest`, `drift`, `blocker-chain`, `impact-network`, `causality`, `related`, `impact`, `orphans` all inherit the persistent `--shape` flag and accept it without flag-parse errors. These subcommands' outputs use purpose-built wrapper types (`Recommendation`, `TopPick`, `PlanItem`, `EnhancedPriorityRecommendation`, `BlockerChainEntry`, `NetworkNode`, `RelatedWorkBead`, `AffectedBead`, `CausalChain`, `OrphanCandidate`) that are already compact-by-construction and emit no fat body fields, so no per-subcommand projection was needed.

### Flag resolution order

1. `--shape=compact` / `--shape=full` (explicit)
2. `--compact` / `--full` (alias; errors if combined with conflicting `--shape`)
3. `BT_OUTPUT_SHAPE` env var
4. `compact` default

### Tests

- `pkg/view/compact_issue_test.go` — unit (7 cases): nil/empty safety, field copying, labels aliasing, reverse-map correctness, `is_blocked` semantics across open/closed/in-progress/external blockers, `relates_count` local-only, metadata bridge, schema-constant check.
- `pkg/view/projections_test.go` — golden-file harness exercising 5 fixtures (minimal, fully-populated, blocked, epic-with-children, global-multiproject). Regenerate with `GENERATE_GOLDEN=1`.
- `cmd/bt/robot_compact_flag_test.go` — 14 flag-resolution cases (defaults, explicit, aliases, env, conflicts, bad values).
- `cmd/bt/robot_list_compact_test.go` — contract suite: no forbidden body fields leak, `--full` restores bodies, all flag/env permutations resolve consistently, reverse-graph counts (`is_blocked`, `parent_id`, `blockers_count`, `relates_count`), `--full` key regression.
- `cmd/bt/robot_all_subcommands_test.go` — 64 subtests across 16 subcommands × 4 flag permutations verifying flag acceptance, plus compact/full contract tests for `robot diff`.

### Blocks / unblocks

- **Unblocks**: bt-mhwy.2 (pairs), bt-mhwy.3 (refs), bt-mhwy.4 (portfolio), bt-mhwy.5 (external dep resolution), bt-mhwy.6 (provenance surfacing).
- **Prerequisites** (both landed earlier this session): bt-uc6k (schema-drift audit), bt-mhwy.0 (column catchup for `metadata` + `closed_by_session`).

---

## 2026-04-14 - Quick wins, footer extraction, label picker redesign

**Bug fixes + footer decomposition + label picker UX overhaul.** 17 commits, 4 bugs fixed, 1 refactor, 12 new tests.

### Bug fixes

- **Label picker freeze** (bt-eorx, P1) - Label picker lacked the early-return pattern used by other modals. Typed characters (g, i, a) were intercepted by global handlers that triggered expensive operations on 2500+ issues. Fix: added early return for ModalLabelPicker before global key handlers.

- **Status bar message not displaying** (bt-6k0f, P2) - `handleKeyPress` cleared `statusMsg` on every keypress but did not reset `statusSetAt`. New messages set via direct assignment had a stale timestamp from a previous message, causing `renderFooter`'s auto-dismiss to clear them before they rendered. Fix: reset `statusSetAt` in the clear-on-keypress block. Also migrated y-key copy handlers to use `setStatus()`/`setStatusError()`.

- **Label dashboard leaves split view disabled** (bt-trqo, P2) - Global `esc` handler for ViewLabelDashboard set `mode=ViewList` but forgot to restore `isSplitView=true`. Global `q` handler had no ViewLabelDashboard check at all (fell through to `tea.Quit`). Fix: added `isSplitView=true` to both global handlers.

### Refactor

- **Footer extraction** (bt-oim6, P2) - Extracted 650-line `renderFooter()` into `FooterData` value struct + `Render()` method. `Model.footerData()` extracts ~35 Model fields into plain values, `FooterData.Render()` does pure rendering with no Model access. 12 tests cover status bar, badges, worker levels, alerts, time travel, hint truncation.

### Skipped

- **bt-8jds** (wisp toggle key conflict) - Blocked by bt-tkhq (keybinding research, human gate). Both `w` and `W` are taken, needs keybinding audit before choosing a new key.

### Refactor epic status (bt-if3w)

5/7 children complete (oim6 closed this session). Remaining: bt-t82t (stale refs/golden files), bt-if3w.1 (sprint view extraction).

### Label picker redesign (bt-36h7, dogfooded)

- **Overlay compositing** - converted from full-screen replacement to OverlayCenter overlay, matching project filter pattern
- **RenderTitledPanel** - round borders with "Filter by Label" title in border
- **Search input** - all letter keys go to text input (no j/k/h/l navigation conflicts), arrow keys only for nav
- **Multi-select** - space toggles labels (checkmarks), enter applies compound OR filter
- **Composing filters** - label filter is now independent of status filter (open + area:tui works)
- **Selected labels pinned** - toggled labels stay at top of list even when filtered by search
- **Stable modal** - fixed width (computed from all labels), fixed height (padded to maxVisible), page-aligned windowing
- **Page navigation** - left/right arrows, PageUp lands at top, PageDown at bottom

### UX improvements

- **Filter toggle** - o/c/r keys now toggle (press again to revert to "all")
- **Sort cycle** - reordered to updated -> created newest -> created oldest -> priority
- **Esc clears everything** - status filter, label filter, sort mode, search all reset on esc

**All tests pass. Build clean.**

---

## 2026-04-13 - Beads Feature Surfacing Wave 4: Wisps, Swarm, Capabilities (bt-9kdo, bt-1knw, bt-t0z6)

**Final session of the 4-wave feature surfacing plan.** 3 commits (parallel subagents), 740 lines added across 12 files, 20 new tests.

### What shipped
- **Wisp visibility toggle** (bt-9kdo) - `w` key hides/shows ephemeral issues. Default: hidden (matches `bd ready`). Wisps render dimmed+italic when visible. Footer badge shows state. Filter applied across all view paths (list, board, graph, BQL, recipes).
- **Swarm wave visualization** (bt-1knw) - `s` key in graph view shells to `bd swarm validate --json`, colors nodes by wave (green=wave 0/ready, yellow=wave 1, blue=wave 2+). Metrics panel shows wave position, max parallelism, estimated sessions. 5-second timeout with graceful error handling.
- **Capability map** (bt-t0z6) - Parses `export:`, `provides:`, `external:<project>:<cap>` labels. Detail panel shows capabilities section in workspace/global mode. `aggregateCapabilities()` builds cross-project edge graph with unresolved dependency detection.

### Key design decisions
- Wisp `w` key reuses the existing global handler - fires wisp toggle in non-workspace mode, project picker in workspace mode
- Swarm data loaded via `exec.CommandContext` (same pattern as other bd integrations) - no direct Dolt writes
- Capability map is a detail panel section, not a new ViewMode - lower effort, 80% of the value

### Parent epic: bt-53du (beads feature surfacing)
All 4 waves complete. Sessions 0-1 (data model), Session 2 (gate indicators), Session 3 (stale/epic/state dims), Session 4 (wisps/swarm/capabilities).

---

## 2026-04-12 - Temporal Infrastructure: Dolt AS OF queries + TemporalCache (bt-ph1z.7)

**Foundation for cross-project trending features.** 4 commits, 955 lines added across 8 files, 13 new tests.

### What shipped
- **`LoadIssuesAsOf(timestamp)`** on `GlobalDoltReader` - queries each database individually using Dolt `AS OF` syntax. Per-database error handling: if one database has no commit at the requested timestamp, it's skipped with a warning (others still load).
- **`TemporalCache`** in `pkg/analysis/temporal.go` - stores `map[time.Time][]model.Issue` snapshots. TTL-based staleness (default 1hr, configurable via `BT_TEMPORAL_CACHE_TTL`). Max 30 snapshots cap (`BT_TEMPORAL_MAX_SNAPSHOTS`). Concurrent populate guard. Oldest-first eviction.
- **`SnapshotMetrics`** - lightweight summary struct (open/blocked/closed counts, 7-day velocity) computed per snapshot. `ComputeMetricsSeries()` produces a time-ordered series from cache data.
- **Background worker integration** - `startTemporalCacheLoop()` goroutine runs on the cache TTL cadence (hourly), independent of the 3-second UI poll. 5-second startup delay to avoid competing with main data load. `TemporalCacheReadyMsg` notifies the UI.
- **`DataSnapshot.TemporalCache`** field carries the cache reference to the UI layer.

### Key design decisions
- Per-database queries (not UNION ALL) for AS OF - databases have different commit histories
- Background goroutine separate from poll loop - slow cadence, own connection
- `IssueLoader` interface on `TemporalCache.Populate()` - testable without a live Dolt server
- Timestamps, not commit refs - simpler across databases with different commit cadences

### What this unlocks
- bt-ph1z.2: Sparkline snapshots (needs TemporalCache data)
- bt-ph1z.3: Diff mode (needs LoadIssuesAsOf for two-snapshot comparison)
- bt-ph1z.4: Timeline view (needs SnapshotMetrics series)

**All 1483 package tests pass. Build clean.**

---

## 2026-04-10c - Phase 2 + Phase 3: Theme redesign + Cobra CLI (bt-k5zs, bt-oim6, bt-zt9q)

**Two parallel refactors shipped**: Phase 2 (theme/color system) and Phase 3 (CLI structure) executed as parallel worktree agents since they touch disjoint file sets (pkg/ui/ vs cmd/bt/).

### Phase 2: AdaptiveColor kill + resolved color system (bt-k5zs)
- **174 `AdaptiveColor` occurrences eliminated** across 25 files. All color fields now use `color.Color` (resolved at load time based on `isDarkBackground`).
- **Dark mode detection**: `tea.BackgroundColorMsg` in Init()/Update() - the canonical Charm v2 pattern. Replaces the Phase 0 shim that defaulted to dark.
- **Theme struct redesigned**: All color fields changed from `AdaptiveColor` to `color.Color`. `resolveColor(light, dark)` helper resolves based on `isDarkBackground`.
- **styles.go**: All 52 package-level `Color*` vars changed to `color.Color`. New `resolveColors()` function rebuilds everything when dark/light changes.
- **theme_loader.go**: `AdaptiveHex.toColor()` resolves at load time. Fallback maps provide light/dark defaults for partial YAML overrides.
- **Glamour**: Style selection now dynamic (`"dark"`/`"light"`) based on `isDarkBackground`.
- **`adaptive_color.go` deleted**. The Phase 0 compatibility shim is gone.

### Phase 3: Cobra CLI migration (bt-zt9q)
- **main.go: 1,708 -> 13 lines**. Just `rootCmd.Execute()`.
- **Cobra subcommand tree**: `bt robot triage`, `bt robot graph`, `bt export pages`, `bt agents add`, `bt baseline check`, `bt version`, etc.
- **35+ robot subcommands** migrated from `--robot-*` flags to `bt robot *` subcommands.
- **Bare `bt` launches TUI** (not help). Uses `rootCmd.Run` + `SilenceUsage: true`.
- **Data loading deferred**: Only commands that need data call `loadIssues()`. `bt version`, `bt robot recipes`, `bt robot schema` skip it entirely.
- **Clean break**: No backward compat for old `--robot-*` flags (pre-alpha, one consumer).
- **Tests updated** for new subcommand syntax.

**Steps deferred to Phase 4 (bt-t82t)**: Pre-compute hot-path styles (optimization, needs profiling), footer extraction as FooterData (Phase 1.5, bt-oim6 - separate decomposition concern).

**All tests green. Build clean. 26 packages pass.**

---

## 2026-04-10b - Phase 1: Model decomposition (bt-98v9)

**Core refactor shipped**: 4 commits, 21 files, 3,235 insertions / 3,030 deletions.

**Step 1.1 - ViewMode enum**: Replaced 7 mutually exclusive boolean view flags (`isBoardView`, `isGraphView`, etc.) with an 11-value `ViewMode` enum. All routing (View(), Update(), key dispatch) now switches on `m.mode`.

**Step 1.2 - State extraction**: Moved ~50 fields from Model into focused sub-structs: `DataState` (pointer, issues/snapshot/worker), `FilterState` (pointer, filters/BQL/recipes), `AnalysisCache` (pointer, triage scores/counts). `DoltState` and `WorkspaceState` embedded as value types. Model copy per frame: ~1.6KB -> ~240 bytes.

**Step 1.3 - Modal state**: Replaced 19 `show*` booleans with single `activeModal ModalType` enum (16 values). Added `modalActive()`, `openModal()`, `closeModal()` helpers.

**Step 1.4 - Update() decomposition**: Split 2,387-line Update() into 147-line thin router + 3 handler files: `model_update_data.go` (871 lines), `model_update_input.go` (1,217 lines), `model_update_analysis.go` (348 lines). model.go: 3,684 -> 1,438 lines.

**Step 1.5 deferred**: Footer extraction (bt-oim6) - `model_footer.go` touches 35+ Model fields. Natural to bundle with Phase 2 theme redesign.

**Process**: 2 worker agents, ~65 min wall clock. Worker 1 exhausted context on a sed overshoot (replaced `m.issues` in FlowMatrixModel/InsightsModel receivers). Monitor caught it early. Worker 2 finished cleanly through Step 1.4.

**All 24 test packages green. Build clean.**

---

## 2026-04-03c - Global hub data layer (bt-6wbd phase 1)

**GlobalDoltReader shipped**: `internal/datasource/global_dolt.go` - connects to shared Dolt server without a database in the DSN, enumerates all beads project databases, loads issues via UNION ALL with backtick-quoted `database.table` syntax.

**Key implementation**:
- `DiscoverSharedServer()` reads `~/.beads/shared-server/dolt-server.port`, env override via `BT_GLOBAL_DOLT_PORT`
- `EnumerateDatabases()` uses `information_schema.tables` (single query, not N validation queries), filters system DBs
- `LoadIssues()` via UNION ALL across all databases, `SourceRepo` set from database name (overrides column)
- Batch labels/deps/comments via 3 UNION ALL queries (not N+1 per-issue)
- `GetLastModified()` via aggregated `MAX(MAX(updated_at))` across all databases
- Partial failure: broken DBs skipped with `slog.Warn`, healthy DBs loaded

**Source type integration**: `SourceTypeDoltGlobal` added to source.go, `RepoFilter` field on `DataSource`, `LoadFromSource` dispatch case in load.go.

**Poll loop**: `globalDoltPollOnce()` in background_worker.go, dispatched when source type is `SourceTypeDoltGlobal`. Reconnect does TCP dial only (no auto-start, shared server is user-managed).

**CLI**: `--global` flag, mutually exclusive with `--workspace` and `--as-of`. `--repo` filters database list at enumeration (before UNION ALL). Workspace mode UI activates automatically (badges, picker, prefilter).

**Shared column list**: Extracted `IssuesColumns` constant to `columns.go`, used by both `DoltReader` and `GlobalDoltReader`.

**Tests**: 16 new unit tests in `global_dolt_test.go` (query building, system DB filtering, backtick quoting, discovery, DSN construction). Full suite: 27 packages, 0 failures.

**Files created**: `internal/datasource/global_dolt.go`, `internal/datasource/global_dolt_test.go`, `internal/datasource/columns.go`
**Files modified**: `internal/datasource/dolt.go`, `internal/datasource/source.go`, `internal/datasource/load.go`, `pkg/ui/background_worker.go`, `cmd/bt/main.go`

**Bead closed**: bt-6wbd

## 2026-04-03b - BQL bug fixes + global hub planning

**BQL bugs (bt-bjk4)**: Fixed all 5 bugs from gap analysis:
1. Status enum validation - added `ValidStatusValues` map, catches typos like `status=opne`
2. `--robot-bql` envelope - now uses `RobotEnvelope` + `robotEncoder` (adds metadata, TOON support)
3. Dead code removal - removed unused `WithReadySQL` from sql.go
4. Date equality semantics - `created_at = today` now matches any time on that day (truncates to midnight)
5. ISO date parsing - `created_at > 2026-01-15` now works in lexer, parser, and executor

Tests added for all fixes. Full suite passes (27 packages, 0 failures).

**Triage**: bt-dx7k reopened (blocked, not in-progress), bt-28g8 closed (audit done), bt-2bns deferred (Charm v2), bt-xft1 closed (resolved by shared server architecture).

**Global hub design verification**: Verified 5 assumptions from the beads session's design doc against actual codebase. Updated open questions with findings. Key correction: poll system needs real refactoring, not just a query swap.

**Global hub data layer plan**: `docs/plans/2026-04-03-feat-global-hub-data-layer-plan.md` - 4-phase implementation plan for GlobalDoltReader. Batch N+1 queries into 3 UNION ALL, single aggregated MAX for poll, --global flag, workspace UI reuse.

**Beads closed**: bt-bjk4 (BQL bugs), bt-28g8 (keybinding audit), bt-xft1 (data separation)
**ADR-002 updated**: Stream 2 bugs all checked off, Stream 1 robot-bql checked off

## 2026-04-03 - Parallel audit swarm

Burned expiring weekly credits on 5 parallel research agents. All read-only, no code changes.

**Reports produced**:
- `docs/audit/test-suite-audit.md` - 268 test files: 93% KEEP, 0% REMOVE, 1 Windows P1
- `docs/audit/cli-ergonomics-audit.md` - 97 flags inventoried, 3 critical robot-mode envelope bugs
- `docs/audit/charm-v2-migration-scout.md` - 76 files affected, 60% mechanical, theme system is the hard part
- `docs/audit/bql-gap-analysis.md` - corrected stale memory (--bql/--robot-bql already shipped), found 5 bugs
- `docs/drafts/README-draft.md` - complete prose rewrite draft

**Beads closed**: bt-79eg (test audit), bt-pfic (CLI audit)
**Beads created**: bt-0cht (P1, robot-mode fixes), bt-5dvl (P2, test fixes), bt-bjk4 (P2, BQL bugs), bt-iuqy (P2, README review)
**ADR-001 closed out**, ADR-002 created as new project spine. Changelog extracted to this file.

## 2026-04-01 - BQL composable search

New package `pkg/bql/` - BQL parser vendored from zjrosen/perles (MIT), adapted for bt.

- Parser layer: lexer, parser, AST, tokens, validator, SQL builder (~1,500 LOC)
- MemoryExecutor: in-memory evaluation against model.Issue (522 LOC, 28 tests)
- TUI integration: `:` keybind opens BQL modal, dedicated `applyBQL()` filter path
- CLI: `--bql` and `--robot-bql` flags
- Syntax: =, !=, <, >, <=, >=, ~, !~, IN, NOT IN, AND/OR/NOT, parens, P0-P4, date literals, ORDER BY, EXPAND

22 files, ~3,950 lines, 27 packages pass, 0 failures.

## 2026-03-16b - Cross-platform test suite fixes

39 failing Windows tests -> 0 failures across all 26 packages.

- Phase 1: Renamed bv->bt stragglers in 8 files
- Phase 1b: Fixed ComputeUnblocks (filter blocking edges only), slug collision expectations
- Phase 2: filepath.FromSlash/Join in cass and tree test expectations
- Phase 3: configHome override for tutorial progress + wizard config (HOME env doesn't work on Windows)
- Phase 4: runtime.GOOS skip guards for 6 Unix-only permission tests
- Phase 5: Shell-dependent hooks tests skipped on Windows; fixed -r shorthand conflict
- Phase 6: .exe suffix for drift test binaries; file locking fix (defer order)
- Phase 7: Normalized \r\n in golden file comparison

**Closed**: bt-s3xg, bt-zclt, bt-3ju6, bt-7y06, bt-ri5b, bt-dwbl, bt-kmxe, bt-mo7r (8 issues)

## 2026-03-16a - Dolt lifecycle adaptation

New module `internal/doltctl/` for Dolt server management.

- EnsureServer: detects running server (TCP dial) or starts via `bd dolt start`
- StopIfOwned: PID-based ownership check before `bd dolt stop`
- Auto-reconnect: poll loop retries EnsureServer after 3 consecutive failures
- Port discovery chain: BEADS_DOLT_SERVER_PORT > BT_DOLT_PORT > .beads/dolt-server.port > config.yaml > 3307
- Database identity check: `SHOW TABLES LIKE 'issues'` after connecting
- Dead code removed: touchDoltActivity keepalive

11 doltctl tests + 6 metadata tests. **Closed**: bt-07jp (P1), bt-tebr (P2, subsumed)

## 2026-03-12 - Brainstorm + audit planning

No code changes. Post-takeover roadmap brainstorm + codebase audit design.

- Defined 4 phases: Audit -> Stabilize -> Polish -> Interactive
- Key decision: CRUD via bd shell-out (no beads fork needed)
- Designed 8-team parallel codebase audit (~190k LOC)
- Created 8 dogfood beads from TUI usage
- Docs: `docs/brainstorms/2026-03-12-post-takeover-roadmap.md`, `docs/plans/2026-03-12-codebase-audit-plan.md`

## 2026-03-11b - Dolt freshness + responsive help

- **bt-3ynd**: Fixed false STALE indicator - freshness tracks last successful poll, not snapshot build time
- **bt-aog1**: Responsive help overlay - 4x2 grid (wide), 2x4 (medium), single column (narrow)
- **bt-xavk**: Created help system redesign plan (docs/plans/help-system-redesign.md)

## 2026-03-11a - Dogfood polish

- Absolute timestamps in details pane + expanded card
- Priority shows P0-P4 text next to icon
- Status bar auto-clear after 3s
- Help overlay: centered titles, auto-sized panels, 4x2 grid, status indicators panel
- Board: auto-hide empty columns on card expand
- Shortcut audit: found 22 undocumented keys

## 2026-03-07 - Beads migration

Renamed issue prefix bv->bt (553 issues). Set beads.role=maintainer. Local folder renamed. Memory migrated.

## 2026-03-05c - ADR review cleanup

Fixed 14 stale `bv` CLI refs in AGENTS.md. Fixed insights detail panel viewport off-by-one.

## 2026-03-05b - Titled panels

Converted insights, board, and help overlay to RenderTitledPanel. Added BorderColor/TitleColor overrides. Board cards use RoundedBorder + border-only selection.

## 2026-03-05a - Tomorrow Night theme

Visual overhaul: Tomorrow Night + matcha-dark-sea teal. Theme config system (embedded defaults, layered loading). TitledPanel helper. Swapped all Color* vars. 18 new tests.

## 2026-02-25 to 2026-03-04 - Fork takeover

See [ADR-001](docs/adr/001-btui-fork-takeover.md) for detailed session-by-session changelog of the fork takeover work (streams 1-4: Dolt verification, rename, data migration, spring cleaning).
