# bt Open Issues Audit Report

**Date**: 2026-04-14
**Issues audited**: 97 (96 open, 1 in_progress)
**Source**: bt-audit.jsonl export

## 1. Summary Statistics

### By Quality Tier

| Tier | Count | % | Meaning |
|------|-------|---|---------|
| A | 0 | 0% | Meets all criteria |
| B | 87 | 90% | Missing labels (but description is clear) |
| C | 6 | 6% | Missing structure, vague, or needs rewrite |
| D | 4 | 4% | Closeable, superseded, or subsumed |

### By Priority

| Priority | Count |
|----------|-------|
| P1 | 11 |
| P2 | 43 |
| P3 | 43 |

### By Type

| Type | Count |
|------|-------|
| Feature | 47 |
| Bug | 28 |
| Task | 17 |
| Epic | 4 |
| Spike | 1 |

### Key Finding: Labels

**Zero issues have `area:*` labels.** The taxonomy defines 12 area labels (area:cli, area:tui, area:bql, etc.) but none are in use. 15 issues have ad-hoc labels (tui, ux, responsive, etc.) that don't follow the taxonomy convention. The remaining 82 issues have no labels at all.

---

## 2. Full Issue List with Tier and Reason

### Tier D - Closeable / Superseded (4)

| ID | Title | Reason |
|----|-------|--------|
| bt-3y81 | Document what repo picker (w) does and whether it's useful | Subsumed by bt-s4b7 (project navigation redesign), confirmed in today's session |
| bt-714y | Project picker modal needs visual refresh | Subsumed by bt-s4b7 work, confirmed in today's session |
| bt-u33c | Investigate beads ecosystem (gastown, wasteland, gascity) | Duplicates bt-8qd1 (Gas Town/City/Wasteland adoption spike) - same investigation, different framing |
| bt-koz8 | Dolt-native cross-repo data hydration for global mode | Largely superseded by bt-ssk7 (--global federation) which already implements UNION ALL across databases |

### Tier C - Needs Rewrite (6)

| ID | Title | Reason |
|----|-------|--------|
| bt-if3w | Refactor pkg/ui monolith + Charm v2 migration | No ## structure at all, no acceptance criteria. The body is useful context but reads like a brain dump, not a bead. In-progress but the description doesn't match the standard. |
| bt-8f34 | Project registry: surface what each repo is and who owns what | Uses ## Problem instead of ## Why, no Acceptance Criteria section. "What's needed" is vague - could mean a config file, a CLI command, a TUI panel, or a beads feature. Two agents would disagree on what to build. |
| bt-vs7w | Attention view (]): barebones table, needs visual redesign | Typed as bug but is actually a feature/polish request. Description mixes two separate issues (attention view layout + label dashboard broken). The label dashboard part is already tracked in bt-rhqs. |
| bt-eiec | Board view cards need redesign - labels should do more heavy lifting | Description conflates board card redesign, label display, and CRUD interaction states into one bead. Scope is unclear - "labels do more heavy lifting" is subjective. Should be split or scoped tighter. |
| bt-ph1z | Cross-project management gaps - user feedback from GH#3008 | Reads as a checklist of things to verify rather than a concrete deliverable. "Verify bt covers or plans to cover" - this is triage work, not a bead. Should be an audit task or decomposed into the specific gaps. |
| bt-ghbl | Cross-project beads: no raw SQL against shared server | More of a convention/guardrail than a deliverable. Acceptance criteria ("no raw SQL mutations") is a rule, not something to build. Should be a convention doc or AGENTS.md update, not a feature bead. |

### Tier B - Needs Minor Update (87)

All remaining issues have clear descriptions with proper ## structure and acceptance criteria, but are missing `area:*` labels. Listed by priority.

#### P1 (9 issues at Tier B)

| ID | Title | Missing |
|----|-------|---------|
| bt-0cht | Fix critical robot-mode CLI bugs from ergonomics audit | area:cli label |
| bt-53du | Product vision: bt v1 (epic) | area:tui label (or multi-area) |
| bt-dcby | Global mode: features don't respect activeRepos project filter | area:tui label |
| bt-eorx | Label picker freezes terminal when typing filter text | area:tui label |
| bt-s4b7 | Redesign project navigation: filtering vs switching vs context | area:tui label |
| bt-ssk7 | Implement bt --global cross-database federation | area:data label |
| bt-t82t | Phase 4: Stale refs, golden files, test validation | area:infra label |
| bt-ushd | [epic] Cross-project beads operating system | area:infra label (or multi-area) |
| bt-8qd1 | Research: evaluate Gas Town / Gas City / Wasteland adoption | area:infra label, workflow:investigate |

#### P2 (36 issues at Tier B)

| ID | Title | Missing |
|----|-------|---------|
| bt-2cvx | Session author provenance: track which session/project filed a bead | area:data |
| bt-4dam | Graph view: missing filter keys (a/o/c/r) - falls through | area:tui |
| bt-4jyd | Global cross-project bead audit | area:analysis |
| bt-5dvl | Fix test suite issues from audit (P1-P3) | area:tests |
| bt-5mgs | Alerts panel unusable at global scale (195 alerts, no pagination) | area:tui |
| bt-6fn2 | Surface 'human' label as a workflow signal - not just a tag | area:tui |
| bt-6k0f | Statusbar message not displaying after copy (y key) - regression | area:tui |
| bt-7rt4 | Search UX: / should work from details pane + preserve position | area:search |
| bt-8col | Graph view: list selection doesn't carry to graph ego node | area:tui |
| bt-8jds | Wisp toggle (w) inaccessible in workspace/global mode | area:tui |
| bt-ammc | Write user-facing docs for global mode and shared server migration | area:docs |
| bt-dx7k | Help overlay broken/unusable at small terminal sizes | area:tui |
| bt-eiec | Board view cards need redesign | area:tui |
| bt-gcuv | Priority hints computed from global issues, not filtered project | area:analysis |
| bt-gf3d | [epic] Keybinding consistency overhaul | area:tui |
| bt-if3w | Refactor pkg/ui monolith + Charm v2 migration | area:tui |
| bt-if3w.1 | Extract sprint view as standalone component | area:tui |
| bt-iuqy | Review and adapt README draft for current state | area:docs |
| bt-jov1 | Beads upstream sync: daily hook for bd/schema changes | area:infra |
| bt-k8rk | h/H key behavior is buggy and confusing | area:tui |
| bt-k9mp | Cross-project bead filing: agents file beads where they belong | area:infra |
| bt-ks0w | Mouse click support: panel focus + issue selection | area:tui |
| bt-lgbz | Card expand broken when empty columns are visible | area:tui |
| bt-lt2h | Human-readable CLI output mode (non-TUI, non-robot) | area:cli |
| bt-m9te | Footer status bar: rethink layout, notifications, hierarchy | area:tui |
| bt-mer9 | Alerts panel shows all projects even when filtered to one | area:tui |
| bt-npnh | History view broken at smaller terminal dimensions | area:tui |
| bt-nzsy | Search shows 'no issues found' after background bead update | area:search |
| bt-oiaj | TUI read/write: create and edit beads from bt | area:tui |
| bt-oim6 | Phase 1.5: Extract footer as component (FooterData struct) | area:tui |
| bt-qzgl | [epic] Graph view overhaul | area:tui |
| bt-rhqs | Label dashboard: key dispatch broken - h/H/L fall through | area:tui |
| bt-spzz | Smarter reload status: show what changed | area:tui |
| bt-tkhq | Research TUI keybinding conventions | area:tui |
| bt-trqo | Exiting label dashboard ([) leaves split view disabled | area:tui |
| bt-ty44 | Explore and document what each bt view does | area:tui |
| bt-vhhh | Detail-only view: can't navigate between issues with arrow keys | area:tui |
| bt-xron | Filter keys (o/c/r) should toggle off when pressed again | area:tui |
| bt-y0fv | Responsive layout: adapt views for small terminal dimensions | area:tui |
| bt-y0k7 | Poll refresh disrupts TUI views - flash + status bar clobber | area:tui |
| bt-z9ei | Lazydev vision: bt as lazygit-style project workspace | area:tui |
| bt-ghbl | Cross-project beads: no raw SQL against shared server | area:data |

#### P3 (42 issues at Tier B)

| ID | Title | Missing |
|----|-------|---------|
| bt-46fa | Redesign issue list column header | area:tui |
| bt-5fbd | Flow matrix: keyboard input conflicts with navigation | area:tui |
| bt-5glp | Research: inter-session messaging for Claude Code | area:infra, workflow:investigate |
| bt-689s | Investigate events-based polling as optimization | area:data, workflow:investigate |
| bt-6cfg | Same-ID cross-prefix bead linking in global view | area:tui |
| bt-6yjh | Actionable view (a): wasteful layout, needs density rethink | area:tui |
| bt-8lz1 | Workspace stack vision: bt + tpane + cnvs | area:infra, workflow:brainstorm |
| bt-95d1 | Document hook security model in README | area:docs |
| bt-bv3a | Document local-to-shared server migration path | area:docs |
| bt-d8ty | Scroll acceleration / speed ramp on main issues list | area:tui |
| bt-e30n | Labels: better visual display + explore as navigation | area:tui |
| bt-ezk8 | History view doesn't work in global mode | area:tui |
| bt-faaw | BQL syntax highlighting in query modal | area:bql |
| bt-fbx6 | Recipes: no mouse support in picker modal | area:tui |
| bt-hazr | Switch default search mode to semantic | area:search |
| bt-iu56 | Prompt management panel in bt | area:tui, workflow:brainstorm |
| bt-j65a | Surface high-connectivity beads: recipe or sort | area:analysis |
| bt-kvk0 | Dolt e2e test infrastructure | area:tests |
| bt-lin9 | Shortcuts bar (;): layout broken, overlaps | area:tui |
| bt-n7i5 | Investigate x/export keybind - unclear UX | area:export |
| bt-nb7o | Recipes: underexposed - need better discoverability | area:tui |
| bt-ph1z.1 | Portfolio health scoreboard | area:analysis |
| bt-ph1z.2 | Temporal trending: sparkline snapshots | area:analysis |
| bt-ph1z.3 | Temporal trending: diff mode | area:analysis |
| bt-ph1z.4 | Temporal trending: timeline view | area:analysis |
| bt-ph1z.5 | Cross-project dependency graphs | area:analysis |
| bt-ph1z.6 | DR: documentation + status indicator | area:docs |
| bt-qk1x | Display externally-filed beads differently in TUI | area:tui |
| bt-sytt | Evolve recipes into saved BQL queries | area:bql |
| bt-t8g6 | Theming: improve and document theme system | area:tui |
| bt-thpq | Investigate Dolt changelog/history view in bt | area:data, workflow:investigate |
| bt-vk3v | Board view: G goes to bottom but no keybind to return to top | area:tui |
| bt-wfss | Rethink 'Issues' panel title border | area:tui |
| bt-xavk | Redesign help system: layered, task-oriented | area:tui |
| bt-yjc0 | Revisit TYPE column icons - unclear what they mean | area:tui |
| bt-zdjr | Board view: poor use of screen space with empty columns | area:tui |
| bt-zko2 | History view: 'Showing N/M' denominator uses global count | area:tui |
| bt-zr9n | Improve startup info output: format, usefulness, clarity | area:cli |
| bt-ztrz | Investigate: manual Dolt refresh/reconnect keybind | area:tui |
| bt-8f34 | Project registry: surface what each repo is and who owns what | area:infra |

---

## 3. Consolidation Proposals

### Group A: Global Scoping Bug (bt-dcby cluster)

**Root issue**: bt-dcby - features don't respect activeRepos filter

**Children that are symptoms of the same bug**:
- bt-mer9 - Alerts panel shows all projects when filtered
- bt-gcuv - Priority hints computed from global issues
- bt-zko2 - History view denominator uses global count
- bt-6yjh - Actionable view has global scoping bug (noted in description)
- bt-rhqs - Label dashboard scoping mismatch (labels from global, filter from project)

**Recommendation**: Keep bt-dcby as the parent. bt-mer9, bt-gcuv, bt-zko2 are already parent-child. The children track per-view symptoms, which is appropriate because each view needs its own fix. No merge needed, but bt-6yjh and bt-rhqs should be formally linked as parent-child if not already.

### Group B: Keybinding Bugs (bt-gf3d cluster)

**Root issue**: bt-gf3d - [epic] Keybinding consistency overhaul

**Children**:
- bt-tkhq - Research keybinding conventions (blocks everything)
- bt-8jds - Wisp toggle (w) inaccessible in workspace mode
- bt-4dam - Graph view filter keys fall through
- bt-5fbd - Flow matrix keyboard conflicts
- bt-k8rk - h/H key behavior buggy
- bt-xron - Filter keys should toggle
- bt-vk3v - Board view missing go-to-top keybind

**Recommendation**: Already well-organized as an epic. No merge needed. bt-8jds is noted as getting worse today (w now also closes picker) - this should be P1 given it's actively regressing.

### Group C: Project Picker / Navigation (bt-s4b7 cluster)

**Overlap**:
- bt-s4b7 - Redesign project navigation (the parent work)
- bt-3y81 - Document what repo picker does (**CLOSE** - subsumed by s4b7)
- bt-714y - Project picker visual refresh (**CLOSE** - subsumed by s4b7)

**Recommendation**: Close bt-3y81 and bt-714y with "superseded by bt-s4b7" close reason.

### Group D: Board View Issues

**Overlap**:
- bt-lgbz - Card expand broken with empty columns
- bt-zdjr - Board view poor use of screen space with empty columns
- bt-eiec - Board view cards need redesign

**Recommendation**: bt-lgbz and bt-zdjr are essentially the same problem (empty columns waste space). Merge into one. bt-eiec is broader (card redesign) and should reference the merged empty-column bead.

### Group E: Responsive / Small Terminal

**Overlap**:
- bt-y0fv - Responsive layout: adapt views for small terminal dimensions (parent)
- bt-dx7k - Help overlay broken at small terminal sizes
- bt-npnh - History view broken at smaller terminal dimensions
- bt-vhhh - Detail-only view can't navigate between issues

**Recommendation**: Already organized with parent-child. Good structure.

### Group F: Recipes

**Overlap**:
- bt-nb7o - Recipes: underexposed, need better discoverability
- bt-fbx6 - Recipes: no mouse support in picker
- bt-sytt - Evolve recipes into saved BQL queries

**Recommendation**: bt-nb7o covers UX/discoverability, bt-fbx6 is a specific bug within the picker, bt-sytt is an evolution. These are distinct enough to keep separate. Consider making bt-nb7o the parent.

### Group G: Gas Town / Ecosystem Research

**Overlap**:
- bt-8qd1 - Research: evaluate Gas Town / Gas City / Wasteland adoption (spike, P1)
- bt-u33c - Investigate beads ecosystem for bt integration opportunities (task, P3)

**Recommendation**: **Close bt-u33c** - it's a subset of bt-8qd1. The spike covers the same ground with more detail and higher priority.

### Group H: Cross-Project Filing and Provenance

**Overlap**:
- bt-ushd - [epic] Cross-project beads operating system (parent)
- bt-k9mp - Cross-project bead filing
- bt-2cvx - Session author provenance
- bt-qk1x - Display externally-filed beads differently
- bt-6cfg - Same-ID cross-prefix bead linking
- bt-ghbl - No raw SQL against shared server

**Recommendation**: Already organized under bt-ushd epic. Good structure. bt-ghbl is more of a convention than a feature (see Tier C notes).

### Group I: Documentation Cluster

**Overlap**:
- bt-ammc - Write user-facing docs for global mode
- bt-bv3a - Document local-to-shared server migration path
- bt-95d1 - Document hook security model in README
- bt-iuqy - Review and adapt README draft

**Recommendation**: bt-ammc and bt-bv3a overlap significantly (both about documenting global mode/migration). Consider merging into one "global mode user documentation" bead. bt-95d1 and bt-iuqy are distinct.

### Group J: Temporal Trending (bt-ph1z cluster)

**Issues**: bt-ph1z, bt-ph1z.1, bt-ph1z.2, bt-ph1z.3, bt-ph1z.4, bt-ph1z.5, bt-ph1z.6

**Recommendation**: Well-decomposed with parent-child structure and sequential blocking. This is good. The parent (bt-ph1z) is the one that needs rewrite (Tier C) - it reads as a checklist, not a bead.

---

## 4. Issues That Should Be Closed

| ID | Title | Reason to Close |
|----|-------|-----------------|
| bt-3y81 | Document what repo picker (w) does | Subsumed by bt-s4b7 redesign, confirmed today |
| bt-714y | Project picker modal needs visual refresh | Subsumed by bt-s4b7 redesign, confirmed today |
| bt-u33c | Investigate beads ecosystem for bt integration | Duplicate of bt-8qd1 (same investigation, lower priority) |
| bt-koz8 | Dolt-native cross-repo data hydration | Superseded by bt-ssk7 which implements the same approach (UNION ALL across databases). The JSONL sync path described in bt-koz8 is already dead. |

---

## 5. Issues Missing Labels (All 97)

**Critical finding**: Zero issues use the `area:*` taxonomy from `.beads/conventions/labels.md`. This is a systemic gap, not per-issue oversight.

15 issues have ad-hoc labels that don't match the taxonomy:
- `tui`, `ux`, `responsive`, `visual-polish` (should be `area:tui` + concern labels)
- `help-system`, `keybindings`, `navigation` (should be `area:tui`)
- `dolt`, `history` (should be `area:data`)
- `detail-pane`, `labels` (should be `area:tui`)
- `personal-os` (not in taxonomy at all)

### Suggested Label Assignments

Below is the recommended `area:*` label for each issue. Issues that span multiple areas get the primary area where the code change lives.

| Area | Issues |
|------|--------|
| **area:tui** (56) | bt-3y81, bt-4dam, bt-46fa, bt-5fbd, bt-5mgs, bt-6cfg, bt-6fn2, bt-6k0f, bt-6yjh, bt-714y, bt-7rt4, bt-8col, bt-8jds, bt-d8ty, bt-dcby, bt-dx7k, bt-e30n, bt-eiec, bt-eorx, bt-ezk8, bt-fbx6, bt-gf3d, bt-if3w, bt-if3w.1, bt-iu56, bt-k8rk, bt-ks0w, bt-lgbz, bt-lin9, bt-m9te, bt-mer9, bt-nb7o, bt-npnh, bt-oiaj, bt-oim6, bt-qk1x, bt-qzgl, bt-rhqs, bt-s4b7, bt-spzz, bt-t8g6, bt-tkhq, bt-trqo, bt-ty44, bt-vhhh, bt-vk3v, bt-vs7w, bt-wfss, bt-xavk, bt-xron, bt-y0fv, bt-y0k7, bt-yjc0, bt-z9ei, bt-zdjr, bt-zko2, bt-ztrz |
| **area:cli** (3) | bt-0cht, bt-lt2h, bt-zr9n |
| **area:data** (5) | bt-2cvx, bt-689s, bt-koz8, bt-ssk7, bt-thpq |
| **area:infra** (9) | bt-5glp, bt-8f34, bt-8lz1, bt-8qd1, bt-ghbl, bt-jov1, bt-k9mp, bt-t82t, bt-ushd |
| **area:analysis** (8) | bt-4jyd, bt-gcuv, bt-j65a, bt-ph1z.1, bt-ph1z.2, bt-ph1z.3, bt-ph1z.4, bt-ph1z.5 |
| **area:search** (3) | bt-hazr, bt-nzsy, bt-7rt4 |
| **area:bql** (2) | bt-faaw, bt-sytt |
| **area:export** (1) | bt-n7i5 |
| **area:docs** (5) | bt-95d1, bt-ammc, bt-bv3a, bt-iuqy, bt-ph1z.6 |
| **area:tests** (2) | bt-5dvl, bt-kvk0 |
| **Multi-area (epics)** (3) | bt-53du, bt-ph1z, bt-u33c |

### Suggested Cross-Cutting Labels

| Label | Issues |
|-------|--------|
| **ux** | bt-6fn2, bt-7rt4, bt-d8ty, bt-e30n, bt-ks0w, bt-m9te, bt-nb7o, bt-spzz, bt-vhhh, bt-xavk, bt-xron, bt-yjc0, bt-zr9n |
| **performance** | bt-689s, bt-d8ty, bt-eorx |
| **security** | bt-95d1 |
| **workflow:investigate** | bt-5glp, bt-689s, bt-thpq, bt-n7i5, bt-ztrz |
| **workflow:brainstorm** | bt-8lz1, bt-iu56, bt-z9ei |
| **platform:windows** | bt-5dvl (e2e tests panic on Windows) |

---

## 6. Issues That Could Benefit from Dependencies, Gates, or Molecules

### Missing Blocking Dependencies

| Issue | Should Block On | Reason |
|-------|----------------|--------|
| bt-ammc (global mode docs) | bt-ssk7 (--global federation) | Can't document global mode before it's fully implemented |
| bt-oiaj (TUI CRUD) | bt-gf3d (keybinding overhaul) | Write actions need consistent keybindings first |
| bt-spzz (smart reload) | bt-y0k7 (poll refresh flash bug) | Can't show smart reload messages if refresh clobbers the view |
| bt-eiec (board card redesign) | bt-lgbz (card expand broken) | Fix the card rendering bug before redesigning cards |
| bt-nb7o (recipe discoverability) | bt-fbx6 (recipe mouse support) | Fix the picker before trying to drive more users to it |

### Gate Candidates (Human Decision Required)

| Issue | Gate Type | What Needs Deciding |
|-------|-----------|-------------------|
| bt-8qd1 | human | Adopt vs adapt vs ignore Gas Town - strategic direction call |
| bt-s4b7 | human | Project navigation UX pattern - filter vs switch vs context (partially in progress) |
| bt-tkhq | human | Keybinding conventions - esc vs q, toggle patterns (research output needs review) |
| bt-ty44 | human | View audit results - which views to keep/remove/merge |
| bt-n7i5 | human | Export keybind - keep, redesign, or remove |
| bt-if3w | human | Remaining refactor scope - what's Phase 4 vs what gets cut |

### Molecule Workflow Candidates

The **temporal trending** sequence (bt-ph1z.2 -> bt-ph1z.3 -> bt-ph1z.4) is a natural molecule - each step builds on the previous and they share the "Dolt AS OF" performance benchmark gate (bt-ph1z.7). If beads molecules support sequential execution with shared context, this cluster is ideal.

The **keybinding overhaul** (bt-tkhq research -> bt-gf3d children) could also be a molecule - research produces conventions that gate all the individual keybinding fixes.

---

## 7. Observations and Recommendations

### Label Taxonomy Adoption is Zero

The most actionable finding: the label taxonomy exists in `.beads/conventions/labels.md` but has never been applied. This could be a bulk update - the suggested labels above cover all 97 issues.

**Suggested approach**: Run a batch `bd update` for all 97 issues adding their `area:*` label. This is a single sweep that immediately makes the backlog filterable.

### Ad-Hoc Labels Should Be Migrated

The 15 issues with existing labels use non-taxonomy labels. These should be replaced:
- `tui` -> `area:tui`
- `dolt` -> `area:data`
- `help-system` -> `area:tui` (help is part of the TUI)
- `keybindings` -> `area:tui`
- `navigation` -> `area:tui`
- `responsive` -> (cross-cutting, keep or drop)
- `visual-polish` -> `ux`
- `detail-pane` -> `area:tui`
- `labels` -> `area:tui`
- `history` -> `area:tui` or `area:data`
- `personal-os` -> (not in taxonomy, consider adding or dropping)

### Description Quality is High

96 of 97 issues have proper ## structure with acceptance criteria. This is well above average. The main gap is labels, not description quality.

### bt-8jds Needs Priority Bump

bt-8jds (wisp toggle w conflict) was noted as getting worse today - w now also closes the project picker. This is actively regressing and should be P1 or at minimum flagged as a blocker for bt-s4b7 work.

### bt-eorx is a P1 Show-Stopper

Label picker freezing the terminal on keystroke in global mode (15 projects, 1900 issues) is a hard crash. If anyone opens the label picker and types, the app is dead. This should be addressed before any label-related feature work.

### Epic Health

| Epic | Children (open) | Status |
|------|-----------------|--------|
| bt-53du (Product vision v1) | 7 direct children | Healthy - clear vision doc |
| bt-gf3d (Keybinding overhaul) | 7 children | Healthy - research gate in place |
| bt-qzgl (Graph view overhaul) | 1 child (bt-8col) | Thin - other graph bugs not linked |
| bt-ushd (Cross-project OS) | 6 children | Healthy - well-decomposed |
| bt-if3w (Refactor/Charm v2) | 3 children | Needs rewrite - description is unstructured |
| bt-dcby (Global scoping) | Not an epic but acts as one | 5+ children as parent-child |

### Dead Weight Ratio

4 of 97 (4%) should be closed outright. This is low - the backlog is relatively clean for a project at this stage. The main issue is navigability (missing labels), not quality.
