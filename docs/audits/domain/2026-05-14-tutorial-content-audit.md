# Tutorial Content Audit: 30 in-TUI tutorial pages

- Date: 2026-05-14
- SHA: 29cee184
- Sources audited:
  - `pkg/ui/tutorial.go` (markdown content constants + `defaultTutorialPages()`)
  - `pkg/ui/tutorial_content.go` (`structuredTutorialPages()` and component model)
- Parent bead: bt-8fjp
- Audit scope: per-page content drift against current bt codebase + remediation plan.
  No tutorial source edits in this audit.

## Page count

**Actual: 30 pages. Matches the bead's stated count.**

Two parallel definitions exist in the tutorial source:

1. `defaultTutorialPages()` in `pkg/ui/tutorial.go` builds 30 `TutorialPage`
   structs whose `Content` field is markdown (the `*Content` const strings
   below the function body, lines ~1019-2532).
2. `structuredTutorialPages()` in `pkg/ui/tutorial_content.go` builds 30
   `StructuredTutorialPage` structs using the typed-component system.

Both return pages with the same 30 IDs in the same order. `renderContent`
in `tutorial.go` looks the page up by ID via `getStructuredPage(page.ID)`
and prefers the structured render. The markdown constants in tutorial.go
**are only rendered if the structured lookup misses**, which today it
never does. They are effectively dead at runtime today but live in the
file (the bt-krwp close-reason mentions the dual-path).

**The user-visible tutorial is the structured pages.** Audit findings
below reflect what the user actually sees; drift findings in the
markdown constants are called out as second-tier where they diverge.

## Executive summary

| Severity | Count |
|---|---|
| broken | 12 |
| stale  | 11 |
| minor  | 5  |
| keep   | 2  |
| unknown| 0  |

(`keep` = no material drift detected; `unknown` not used because every
claim was reachable in source.)

### Cross-page drift patterns (top 5)

1. **Robot CLI: every `--robot-*` flag reference is wrong** (~7 hits
   across 4 pages). `bt --robot-triage`, `bt --robot-next`,
   `bt --robot-plan`, `bt --robot-alerts`, `bt --robot-insights`,
   `bt --robot-diff`, `bt --robot-label-health`, `bt --robot-label-flow`,
   `bt --robot-label-attention` all became Cobra subcommands under
   `bt robot <subcmd>` (Phase 3, 2026-04-10). All hits live in tutorial
   pages that frame bt for agents.

2. **Static-site export commands are doubly wrong**: the tutorial uses
   `bt --pages` AND `bv --export-pages` / `bv --preview-pages`. Today the
   wizard is `bt pages` (Cobra subcommand) and the export/preview
   subcommands are `bt export pages` / `bt export preview`. Both the
   binary name (`bv` → `bt`) and the flag-vs-subcommand shape are stale.

3. **Data layer language is pre-Dolt.** Tutorial repeatedly cites
   `.beads/issues.jsonl` as the storage backend (concepts-beads page
   shows a sample JSONL line, anatomy diagram). ADR-003 (2026-04-25)
   makes Dolt the system of record and JSONL opt-in export-only.

4. **Legacy binary/prefix references survive in markdown constants:**
   `bv` (~10 hits) and `br` (~7 hits) appear in `tutorial.go`'s content
   constants. The structured pages have mostly been swept to `bt`, but
   not entirely: structured pages still reference `bv-tests`,
   `bv-endpoint`, `bv-feature1`, `bv-bug1` (sample IDs in Code blocks).

5. **Wrong keybindings in structured pages.** Several pages name keys
   that don't exist or are bound differently:
   - **`L` (uppercase) for label picker** — actual binding is lowercase
     `l` (keys/global.go LabelPicker). Tutorial uses `L` on three pages.
   - **`Shift+L` to filter by label** — no such binding exists.
   - **`R` (capital) to open recipe picker** — actual recipe-picker key
     is `'` (single quote). `R` is `RecipeTriage` (apply the triage
     recipe), a different action.
   - **`C` to copy issue ID** — actual binding is `y` (CopyID). `C` is
     CopyIssue (full clipboard copy).
   - **`p` to change priority / `s` to change status** in
     concepts-priorities — neither binding mutates state. `p` toggles
     PriorityHints display, `s` cycles sort or swimlanes by view.
   - **`m` to move issue between board columns** — viewsBoardContent
     (markdown) claims this but board_keys.go has no `m` handler.
   - **`d` to show diff** in History view — actual binding is `o` (open
     commit in browser). There is no in-view diff toggle.
   - **`f` to focus subgraph** in Graph view — graph_keys.go has no `f`;
     `f` is the global FlowMatrix view binding.

### Pages affected by post-fork features that are NOT mentioned

The tutorial pre-dates several shipped features and never picked them
up:

- **Alerts modal** (`!`, global) — bt-1xxx series. Not in tutorial.
- **BQL filter** (`:`, global) — not in tutorial.
- **Time-travel `--as-of` flag and the broader `bt --as-of` global
  flag** — only the in-TUI `t`/`T` modal is covered.
- **Project / workspace picker** (`w` is projects/wisps picker now, `W`
  is home/all projects). Tutorial's workspace page assumes
  `.beads/workspace.json` config; current path is `.bt/workspace.yaml`
  with completely different schema.
- **Capability map** (`pkg/ui/helpers.go` `parseCapabilities`) —
  cross-project capability graph. Not in tutorial.
- **Swarm wave visualization** (`s` in graph view loads swarm
  visualization for an epic) — not in tutorial.
- **Tree view** (`E`, global) — not in tutorial.
- **Attention dashboard** (`]`, global) — not in tutorial.
- **Flow matrix** (`f`, global) — tutorial uses `f` for graph subgraph
  focus that doesn't exist, but doesn't mention the actual flow-matrix
  view.
- **Quit-confirm modal copy convention** (bt-yly4, 2026-05-07) — the
  tutorial's `q | Quit bt` rows should mirror "Quit?" terseness or be
  updated together with related bt-ift6.1 keybind work (see bt-8fjp
  comment thread).
- **Federation, agent detection, hooks trust, baseline** — entire
  feature surfaces with no tutorial coverage.

## Per-page assessment

Page numbers reflect the order returned by `defaultTutorialPages()` /
`structuredTutorialPages()` (both return the same order).

| #  | ID                          | Title                          | Section       | Severity | Drift summary |
|----|-----------------------------|--------------------------------|---------------|----------|---------------|
| 1  | intro-welcome               | Welcome                        | Introduction  | keep     | Naming OK ("beadstui", "bt"). Markdown variant has an ASCII-banner with stale spacing but content sound. |
| 2  | intro-philosophy            | The Beads Philosophy           | Introduction  | broken   | Tells user that data is stored as plain JSONL ("Diffable and Greppable - Issues stored as plain JSONL"). Calls out `--robot-*` flags as the agent surface. Both pre-Dolt and pre-Cobra. |
| 3  | intro-audience              | Who Is This For?               | Introduction  | broken   | Bullets `bt --robot-triage` and `bt --robot-plan` examples in the Code block. Markdown variant says "where bv shines". |
| 4  | intro-quickstart            | Quick Start                    | Introduction  | minor    | Markdown variant has "**`q`** — Quit bt" (bt-yly4 convention shift to terser). KeyTable lists Tutorial as opened by Space-in-help; correct. Section title says "**`** (backtick) — Jump to tutorial" — correct. |
| 5  | concepts-beads              | What Are Beads?                | Core Concepts | broken   | Storage section claims `.beads/issues.jsonl` is the live backend; markdown variant shows a sample JSONL line. Both are pre-Dolt. ID example "bt-abc123" looks current. |
| 6  | concepts-dependencies       | Dependencies & Blocking        | Core Concepts | stale    | Mostly correct (ready filter, bd ready). `bd dep add ...` example syntax is correct. Markdown variant uses emoji 🔴/🟢 which match current visual story. |
| 7  | concepts-labels             | Labels & Organization          | Core Concepts | broken   | KeyTable claims `L` opens label picker and `Shift+L` filters by label. Actual binding is `l` (lowercase) for picker, no Shift+L. `[` for label dashboard correct. |
| 8  | concepts-priorities         | Priorities & Status            | Core Concepts | broken   | KeyTable claims `p` changes priority and `s` changes status. Neither binding does that. `p` is PriorityHints toggle, `s` is sort/swimlane cycle. Status flow diagram is structurally correct. |
| 9  | concepts-graph              | The Dependency Graph           | Core Concepts | stale    | Tree example uses bt-001..006 IDs (synthetic, fine). Visual encoding table fine. Doesn't mention swarm wave visualization (`s` in graph view) or that `g` may not be enabled when graph is empty. |
| 10 | views-nav-fundamentals      | Navigation Fundamentals        | Views         | minor    | All keys correct (j/k/h/l, g/G, Ctrl+d/u, ?, Esc, Enter, q). q description ("Quit bt") will need terser "Quit" per bt-yly4. |
| 11 | views-list                  | List View                      | Views         | minor    | Filter keys (o/c/r/a) correct. Search keys (/, Ctrl+S, H) correct. Sort (s/S) correct. This page was partially updated in bt-krwp. Solid. |
| 12 | views-detail                | Detail View                    | Views         | broken   | `C — Copy issue ID to clipboard` is wrong (`C` copies the full issue; `y` copies the ID). `O` open in editor correct. No mention of `~` for context help. |
| 13 | views-split                 | Split View                     | Views         | keep     | Tab toggles focus, j/k navigate, Esc returns. All correct. |
| 14 | views-board                 | Board View                     | Views         | broken   | Markdown variant claims `m` moves issue across columns (no such binding). Structured variant ("Inline card expansion" `d`) matches. Tab toggles detail (`board.ToggleDetail`) correct. Missing: column-jump keys (`1-4`, `H`/`L`, `0`/`$`, gg combo), `y` to copy ID, `/` to search. |
| 15 | views-graph                 | Graph View                     | Views         | broken   | `f` to focus subgraph does not exist in graph_keys.go (it's global FlowMatrix). Reading-the-graph claims (arrows, node size, color) match graph rendering. No mention of `s` swarm-wave toggle (shipped post-fork). |
| 16 | views-insights              | Insights Panel                 | Views         | stale    | `m` heatmap toggle correct. Doesn't mention `e` (explanations) or `x` (calculations) toggles, or `h/l/Tab` for panel navigation. Priority Score Factors text is approximate but reasonable. |
| 17 | views-history               | History View                   | Views         | broken   | KeyTable: `v` (toggle Bead/Git) correct, `f` (file tree) correct, `Tab` cycles focus correct. Markdown variant claims `d` shows diff — wrong; `o` opens commit in browser. "Time-travel preview via Enter" is current behavior. |
| 18 | advanced-semantic-search    | Semantic + Hybrid Search       | Advanced      | minor    | This page got the bt-krwp rewrite. Modes table correct (/, Ctrl+S cycle, H preset, quoted exact). BT_SEARCH_MODE / BT_SEARCH_PRESET / BT_SEARCH_WEIGHTS env vars are valid. |
| 19 | advanced-time-travel        | Time Travel                    | Advanced      | broken   | `t` and `T` bindings correct. Last code block recommends `bt --robot-diff --diff-since HEAD~50` — `--robot-diff` flag no longer exists; current is `bt robot diff`. Ref syntax (HEAD~5, main, tag, `@{2.weeks.ago}`) handled by git, fine. |
| 20 | advanced-label-analytics    | Label Analytics                | Advanced      | broken   | Markdown variant ends with three `bt --robot-label-*` flag examples that no longer exist. Health-indicator vocabulary (✓ / ⚠ / ⛔) hasn't been verified against label dashboard (`[`) — assume close enough but should be confirmed during rewrite. |
| 21 | advanced-export             | Export & Deployment            | Advanced      | broken   | `bt --pages` (wizard) is now `bt pages`. `bv --export-pages` / `bv --preview-pages` are now `bt export pages` / `bt export preview`. Both shape (Cobra vs flag) and binary name are wrong. |
| 22 | advanced-workspace          | Workspace Mode                 | Advanced      | broken   | Config path wrong (`.beads/workspace.json` → actual is `.bt/workspace.yaml`). Config schema entirely different. `w`/`W` semantics changed (now projects/wisps + home/all-projects, not workspace picker + workspace-search). Cross-repo deps example uses correct `bd dep add` syntax. Markdown variant also has stale `bt --robot-triage` / `bt --robot-plan` block. |
| 23 | advanced-recipes            | Recipes                        | Advanced      | broken   | Both variants claim **R** opens the recipe picker — wrong, `R` is RecipeTriage (apply triage recipe). Recipe picker is `'`. Recipe schema (recipes.json, fields like priority_min/max, status, labels) is roughly current. Last code block uses stale `bv --recipe ... --robot-triage`. |
| 24 | advanced-ai                 | AI Agent Integration           | Advanced      | broken   | Entire page is the worst-drift page: every example uses legacy `bt --robot-*` flags (`--robot-triage`, `--robot-next`, `--robot-plan`, `--robot-insights`, `--robot-alerts`). The "br update" / "br create" agent CLI is gone — current is `bd update --claim`, `bd close`, etc. JSON envelope shape is approximately correct for triage but predates the `scope` block (mode: cross-project / project / workspace) that all robot envelopes now carry. |
| 25 | workflow-new-feature        | Starting a New Feature         | Workflows     | broken   | Markdown variant: every code block uses `br update`, `br create`. `bd dep add bv-tests bv-endpoint` (uses `bv-` prefix instead of `bt-`). Structured variant has `bv-tests`/`bv-endpoint` too. `bd sync` doesn't exist (replaced by Dolt push). Step 1 also still pipes legacy `bt --robot-triage | jq` in markdown variant. |
| 26 | workflow-bug-triage         | Triaging a Bug Report          | Workflows     | broken   | Markdown variant: `br create`, `br update`. Triage-suggestions modal (Step 2, `S` key) doesn't exist. Sample IDs use `bv-` (structured uses `bv-feature1`/`bv-bug1`). Label-picker key `L` (uppercase) is wrong (use `l`). |
| 27 | workflow-sprint-planning    | Sprint Planning Session        | Workflows     | broken   | Page has the only Insights-panel-content claim (open/blocked counts, top blockers, priority distribution); these are loose paraphrases and would need verification against current insights renderer. Final code block has `bt --robot-diff --diff-since HEAD~50` — stale flag. |
| 28 | workflow-onboarding         | Onboarding New Team Members    | Workflows     | broken   | Markdown variant: `bv` launches tutorial automatically (binary name wrong). `br list --label=good-first-issue` and `br update ID --status=in_progress` (legacy). Step 2 references the `\`` backtick correctly. `bd sync` mentioned (gone). |
| 29 | workflow-stakeholder-review | Stakeholder Reviews / Weekly Review | Workflows | broken | Same flaw as advanced-export: `bt --pages` flag + `bv --export-pages` subcommand. CI/CD yaml example uses `bv --export-pages` in the step. Whole flow needs rewriting around `bt pages` and `bt export pages`. |
| 30 | ref-keyboard                | Keyboard Reference             | Reference     | stale    | Both variants got the bt-krwp search-mode update. Global section: `b/g/i/h` switch views — correct, but undersells the surface (missing `[`, `]`, `E`, `f`, `a`, `w`, `W`, `!`, `:`, `'`, `1`, `x`, `l`). q description ("Quit") fine. |

## Remediation plan

### Pages requiring substantive rewrites (severity = broken)

12 pages. Group them into child beads to avoid filing 12 separate
rewrite tasks. Recommended grouping:

1. **Robot CLI surface refresh (4 pages)** — pages 24 (AI Agent
   Integration), 19 (Time Travel robot-diff), 20 (Label Analytics
   robot-*), 27 (Sprint Planning robot-diff). One bead. Pull current
   `bt robot <subcmd>` list from `docs/robot/README.md` and the help
   matrix in `docs/agents.md`. Mention the `scope` envelope.

2. **Static-site export rewrite (3 pages)** — pages 21 (Export &
   Deployment), 22 (Workspace mode, which also has stale robot-* and
   workspace-json), 29 (Stakeholder Review). One bead. Pivot to
   `bt pages` (wizard), `bt export pages <dir>`, `bt export preview <dir>`.

3. **Agent workflow CLI rewrite (3 pages)** — pages 25 (New Feature),
   26 (Bug Triage), 28 (Onboarding). One bead. Replace all `br create`
   / `br update` / `bd sync` with current `bd create` / `bd update
   --claim` / Dolt push. Update sample IDs from `bv-*` to `bt-*`.

4. **Core-concepts keybinding correction (3 pages)** — pages 7 (Labels)
   `L`→`l`, no Shift+L; page 8 (Priorities) drop the "p change priority"
   /"s change status" claim entirely; page 12 (Detail) fix `C`→`y` for
   copy ID. One bead.

5. **View-specific keybinding rewrite (3 pages)** — page 14 (Board)
   missing column-jump keys + remove `m`; page 15 (Graph) remove `f`
   subgraph claim, add swarm-wave note; page 17 (History) remove `d`
   diff claim, add `o` browser-open. One bead. (Could fold into bead 4
   if scope allows.)

6. **Data layer rewrite (2 pages)** — pages 2 (Philosophy "issues
   stored as JSONL"), 5 (What Are Beads — "issues.jsonl" example). One
   bead. Pull current Dolt-only story from ADR-003; mention JSONL is
   opt-in export.

7. **Recipe picker keybinding correction (1 page)** — page 23 (Recipes)
   `R`→`'` for recipe picker. Could fold into bead 4.

### Pages needing only terminology/cosmetic updates (severity = stale or minor)

11 stale + 5 minor pages. Group as a **single naming sweep bead**:

- Eliminate residual `bv` and `br` mentions in markdown content
  constants (~17 hits across tutorial.go).
- Apply bt-yly4 modal-copy convention ("Quit" not "Quit bt").
- Add references to post-fork features where contextually relevant
  (e.g. mention `[`/`]`/`E`/`!`/`:`/`'` in ref-keyboard page; add
  swimlane-cycle reminder in board page).
- Pre-condition: bt-ift6.1+ keybind Maps land first, so the rewrite can
  consume `Help.Desc` strings from `pkg/ui/keys/` for drift-proofing.

### Pages that need no edits (severity = keep)

- Page 1 (intro-welcome)
- Page 13 (views-split)

### Architectural recommendation (out of scope; flag for future)

The audit's "compare each tutorial assertion to source-of-truth" loop
is mechanical. Once `pkg/ui/keys/` is fully populated (bt-ift6.1
through .9), the tutorial's KeyTable entries should consume
`Help.Desc` strings programmatically rather than hand-typing them.
This makes the tutorial structurally drift-proof, the same way the
help surfaces become post-bt-ift6 (single source of truth, surfaces
consume it). Recommend folding this into bt-xavk (help redesign)
rather than spinning out a separate bead.

## Child beads filed

All child beads filed under bt-8fjp, type=task, P3, labels
`area:tui,workflow:docs`:

- **bt-8fjp.1** — Tutorial rewrite: robot CLI surface refresh
  (pages 19, 20, 24, 27)
- **bt-8fjp.2** — Tutorial rewrite: static-site export + workspace
  mode (pages 21, 22, 29)
- **bt-8fjp.3** — Tutorial rewrite: agent workflow CLI
  (pages 25, 26, 28)
- **bt-8fjp.4** — Tutorial fix: wrong keybindings
  (pages 7, 8, 12, 14, 15, 17, 23)
- **bt-8fjp.5** — Tutorial rewrite: data-layer (Dolt-only)
  (pages 2, 5)
- **bt-8fjp.6** — Tutorial sweep: bv/br residue + modal-copy
  convention + ref-page expansion (covers stale + minor pages plus
  cross-cutting naming/copy work flagged by the audit)

## Verification trail (what was checked)

- Built bt at HEAD (29cee184): `go build ./cmd/bt/`. Verified
  `bt --robot-triage` returns "unknown flag" — legacy flags removed.
- Read `pkg/ui/keys/global.go` for global key bindings.
- Read `pkg/ui/keys/list.go` for list-view normal key bindings.
- Inspected `pkg/ui/board_keys.go`, `pkg/ui/graph_keys.go`,
  `pkg/ui/history_keys.go`, `pkg/ui/insights_keys.go`,
  `pkg/ui/help_keys.go`, `pkg/ui/model_update_input.go` for per-view
  handler bodies.
- Inspected `pkg/ui/board.go` for swimlane mode definitions.
- Inspected `docs/adr/003-data-source-architecture-post-dolt.md` for
  the canonical Dolt-only language.
- Confirmed `bd ready`, `bd update --claim`, `bd close`, `bd create
  --title`, `bd dep add` exist; `bd sync` is gone.
- Confirmed `bt export pages`, `bt export preview`, `bt pages`
  subcommands exist (Cobra) and the `--pages` / `--export-pages` /
  `--preview-pages` flags are gone from `bt --help`.
- Confirmed `.beads/` contains a `dolt/` subdirectory; no
  `issues.jsonl` at runtime.
