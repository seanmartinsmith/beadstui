# bt cluster map: bt↔bd parity / writable-TUI productization

Snapshot: 2026-04-27. Inputs: `.bt/tmp/epics-full.json`, `.bt/tmp/p1-full.json`, `.bt/tmp/p2-full.json` (re-dumped without --limit), `bd show` for hot beads, upstream `bd --help` and `~/System/tools/beads/claude-plugin/commands/`.

The cluster sits across three umbrellas (bt-53du, bt-94a7, bt-72l8.1) with secondary anchoring on bt-ushd (cross-project epic) and bt-19vp (history dogfood). It bleeds into the keybind/help epics (bt-gf3d, bt-xavk) wherever a parity gap also implies a new keybind or a new view-type that needs help-system support.

---

## 1. The cluster

Columns: ID · pri · type · age (days since updated) · parent · disc/rel · area labels · one-line cover.

### Under bt-53du — Product vision: bt v1

| ID | P | Type | Age | Parent / link | Labels | What it covers |
|---|---|---|---|---|---|---|
| bt-53du | 1 | epic | 14d | — | area:tui | Three-pillar v1 vision (TUI / CLI / Analytics) |
| bt-oiaj | 2 | feature | 14d | parent=53du | area:tui | **Writable TUI**: phased CRUD via shelled-out bd. Phase 1 internal/bdcli; Phase 2 status keys; Phase 3 create modal; Phase 4 inline edit |
| bt-lt2h | 2 | feature | 14d | parent=53du; rel=mhwy.1 | area:cli | Human-readable CLI output mode (table/--list, --global --list, etc.). Comment 2026-04-21 says wrap upstream `bd audit/stats/list` |
| bt-jov1 | 2 | task | 14d | parent=53du; rel=94a7 | area:infra | Continuous upstream-sync monitoring (schema + CLI flag drift) |
| bt-z9ei | 2 | feature | 14d | parent=53du | area:tui, brainstorm | Lazydev/lazygit-style panel workspace (vision umbrella) |
| bt-s4b7 | 1 | feature | 14d | parent=53du; blocks=dcby | area:tui | Project navigation redesign (filter vs switch vs context) |
| bt-z9ei sibling: bt-8lz1 | 3 | feature | 14d | (no parent link) | area:infra, brainstorm | bt + tpane + cnvs workspace-stack vision |

### Under bt-94a7 — Broad upstream audit (read parity)

| ID | P | Type | Age | Link | Labels | What it covers |
|---|---|---|---|---|---|---|
| bt-94a7 | 2 | task | 4d | rel=jov1, uc6k(closed), wjzk | area:cli, data | One-time tier-1 broad audit of bd surface (29 slash commands + Dolt schema) |
| bt-h5jz | 2 | feature | 4d | disc-from=94a7 | area:data, tui | First-class type=decision (enum, filter, dedicated view, supersede lifecycle) |
| bt-a3sb | 2 | feature | 4d | disc-from=94a7; rel=7czu, vv7o, h5jz | area:tui | Project-grouped list view (cross-project swimlane) |
| bt-vv7o | 2 | feature | 4d | disc-from=94a7; rel=a3sb, h5jz | area:tui | Blocked/waiting queue view (inverse of `bd ready`) |
| bt-7czu | 3 | feature | 4d | disc-from=94a7; rel=a3sb, h5jz | area:tui | By-assignee view (multi-agent reality) |
| bt-qcz8 | 2 | task | 0d | disc-from=94a7 | area:cli, search, brainstorm | **NEW today**: bt search lacks bd-style structured filters (assignee/label/priority/date) |
| bt-4fxz | 2 | feature | 6d | parent=ushd | area:tui | bd audit --actor / bd stats --group-by=actor → bt insights + robot |
| bt-wjzk | 3 | feature | 7d | (no parent) | area:data | Periodic schema-drift detection (the automated successor to 94a7) |

### Under bt-72l8.1 — Ghost-features audit (functional parity)

| ID | P | Type | Age | Parent | Labels | What it covers |
|---|---|---|---|---|---|---|
| bt-72l8 | 1 | epic | 0d | — | area:infra | Sweep for Jeffrey-era leftovers (10 sections; section 9 = bt-72l8.1 ghost-features) |
| bt-72l8.1 | 2 | task | 0d | parent=72l8 | area:infra, tests, investigate | Classify every --robot-* and ViewMode as working/stub/ghost/partial |
| bt-72l8.1.1 | 2 | task | 0d | parent=72l8.1; rel=6q8c | area:infra, tests, tui | TUI per-view completeness pass + keybind sweep |
| bt-z5jj | 3 | feature | 0d | (rel pending) | area:tui, brainstorm | Sprint feature: rebuild against Dolt or retire (concrete prior example feeding 72l8.1 methodology) |
| bt-3suf | 3 | task | 0d | (no parent) | area:tui | Retire bt sprint feature (preview action of 72l8.1) |
| bt-if3w.1 | 2 | task | 14d | parent=if3w; blocks=z5jj | area:tui | Extract sprint view as standalone component (stale framing — sprints aren't upstream) |
| bt-6q8c | 2 | task | 0d | (rel from 72l8.1.1) | area:docs, tui, brainstorm | TUI labels reform: add `view:*` sub-area dimension |

### Under bt-ushd — Cross-project beads operating system (write & cross-project)

| ID | P | Type | Age | Parent | Labels | What it covers |
|---|---|---|---|---|---|---|
| bt-ushd | 1 | epic | 14d | — | area:infra | Cross-project filing, paired IDs, global visibility, agent conventions |
| bt-2cvx | 2 | feature | 0d | parent=ushd; rel=mhwy.6 | area:data | Session author provenance — pairs with bd-34v Phase 1 |
| bt-5hl9 | 2 | feature | 0d | parent=ushd; rel=mhcv, kv7d | area:data | Hydrate created_by_session / claimed_by_session from Dolt direct columns (post-bd-34v) |
| bt-ghbl | 2 | task | 13d | parent=ushd; blocks=ssk7 | area:infra | Cross-project beads: no raw SQL against shared server (use bd-only) |
| bt-k9mp | 2 | feature | 14d | parent=ushd; blocks=2cvx | area:infra | Cross-project bead filing (agents file in right project) |
| bt-4fxz | 2 | feature | 6d | parent=ushd | area:tui | (also under 94a7) bd audit/stats integration |
| bt-ssk7 | 1 | feature | 14d | (rel from dcby cluster) | area:data | Implement bt --global cross-DB federation. Note: description marks itself "already fully implemented"; remaining = UX |
| bt-dcby | 1 | bug | 14d | rel=lwdy; blocks=gcuv, mer9 | area:tui | Global mode: features don't respect activeRepos project filter |
| bt-lwdy | 2 | bug | 4d | parent=y0k7; rel=dcby, nzsy | area:tui | Project filter reset by Dolt poll refresh |
| bt-gcuv | 2 | bug | 14d | parent=dcby | area:analysis | Priority hints computed from global, not filtered project |
| bt-yqh0 | 2 | feature | 4d | rel=ba9f | area:correlation, tui | Cross-project bead aggregation by shared ID suffix (paired-ID surface, with v2 detection guard) |
| bt-ba9f | 2 | feature | 0d | rel=i4yn(closed), yqh0 | area:tui | Pull up specific beads by explicit ID list (`I` modal + `--ids=…` flag) |
| bt-3ltq | 2 | feature | 2d | (no parent) | area:correlation, tui | **Multi-repo git correlation in global mode** — data-layer companion to global Dolt mode |
| bt-mxz9 | 2 | bug | 2d | (no parent; rel mentioned in 3ltq, 08sh) | area:cli, data, investigate | Cold-boot from non-workspace dir fails — needs registry.json populated, port-file trust |
| bt-08sh | 2 | feature | 2d | rel=mhcv, uahv, uh3c, vhn2 | area:correlation, data | Correlator migration to dolt_log / dolt_history_issues (root-cause for 3 robot subcommands) |
| bt-ammc | 2 | task | 13d | (rel=ssk7 blocks) | area:docs | User-facing docs for global mode + shared-server migration |
| bt-4jyd | 2 | feature | 14d | rel=mhwy.3 | area:analysis, personal-os | Global cross-project bead audit |
| bt-6cfg | 3 | feature | 14d | parent=ushd | (cross-prefix linking) | Same-ID cross-prefix bead linking in global view |
| bt-8f34 | 3 | feature | 14d | (no parent) | (project registry) | Project registry: who owns what, surfaces to cross-project agents |

### Under bt-gf3d — Keybinding consistency (touches every new view's bindings)

| ID | P | Type | Age | Parent | Labels | What it covers |
|---|---|---|---|---|---|---|
| bt-gf3d | 2 | epic | 14d | — | area:tui | Unified keybinding architecture |
| bt-tkhq | 2 | task | 14d | parent=gf3d | area:tui | Research keybind conventions (esc vs q, toggle patterns) — research dependency for many children |
| bt-gf3d.1 | 2 | task | 0d | parent=gf3d | area:tui, investigate | Hotkey/feature audit: which keys deserve top-level binding |
| bt-8jds | 1 | bug | 13d | parent=gf3d; blocks=tkhq | area:tui | Wisp toggle (w) inaccessible in workspace/global mode |
| bt-4dam | 2 | bug | 14d | parent=gf3d | area:tui | Graph view filter keys fall through |
| bt-rhqs | 2 | bug | 14d | parent=dcby | area:tui | Label dashboard h/H/L fall through |
| bt-xron | 2 | bug | 14d | parent=gf3d | area:tui | Filter keys should toggle off |
| bt-k8rk | 2 | bug | 14d | parent=gf3d | area:tui, help | h/H key behavior buggy/confusing |

### Under bt-19vp — History view dogfood (hits the writable + global parity surface)

| ID | P | Type | Age | Parent | Labels | What it covers |
|---|---|---|---|---|---|---|
| bt-19vp | 2 | epic | 2d | — | area:tui, investigate | History view focused dogfood pass |
| bt-3ltq | 2 | feature | 2d | (also above) | area:correlation, tui | (referenced by 19vp's intro) |
| bt-ezk8 | 3 | feature | 14d | (no parent link) | (history+global) | History view broken in global mode (no git context) — should fold into 19vp or 3ltq |
| bt-zko2 | 3 | bug | 14d | (no parent) | (history) | "Showing N/M" denominator wrong when filtered |
| bt-npnh | 2 | bug | 14d | parent=y0fv | area:tui | History view broken at small dimensions |
| bt-thpq | 3 | feature | 14d | (no parent) | (Dolt history) | Investigate Dolt changelog/history view in bt — overlaps 3ltq, 08sh |

### ORPHANS — concrete parity/CRUD/global gaps with no umbrella link

These are concrete and obviously belong somewhere, but currently have no parent edge.

| ID | P | Type | Age | Labels | What it covers | Where it should live |
|---|---|---|---|---|---|---|
| bt-qcz8 | 2 | task | 0d | area:cli, search, brainstorm | Search composition gap | 94a7 (already disc-from) — but no parent edge yet, just discovered-from |
| bt-h5jz | 2 | feature | 4d | area:data, tui | type=decision support | 94a7 (disc-from only) |
| bt-a3sb / bt-vv7o / bt-7czu | 2/2/3 | feature | 4d | area:tui | New view types | 94a7 (disc-from only) |
| bt-rbha | 2 | feature | 0d | area:tui | TUI surface for type=gate (follow-up to bt-mbjg hide-by-default) | 94a7 / 72l8.1 — gate is a bd primitive bt should expose |
| bt-mbjg | 2 | feature | 0d | area:tui, ux | Default-hide type=gate beads | Same |
| bt-yqh0 / bt-ba9f / bt-4ew7 | 2/2/3 | feature | 4d/0d/0d | area:tui | Cross-project ID aggregation; explicit ID list; multi-select bulk | ushd or 53du |
| bt-3ltq | 2 | feature | 2d | area:correlation, tui | Multi-repo git correlation | ushd (it's the natural global companion) or 19vp |
| bt-mxz9 | 2 | bug | 2d | area:cli, data | Cold-boot from non-workspace dir | ushd (cross-project bootstrap) |
| bt-08sh | 2 | feature | 2d | area:correlation, data | Correlator → Dolt-native migration | 72l8.1 (this IS a ghost-feature root cause) or its own data-layer epic |
| bt-ah53 | 2 | task | 2d | area:cli, docs, tests | Robot mode I/O contract + verify-test sweep | 72l8.1 (functional parity = stable contracts) |
| bt-4fxz | 2 | feature | 6d | area:tui | bd audit/stats integration | Has parent=ushd, but ALSO listed as known gap in 94a7 — correct linkage is parent ushd + relates 94a7. Currently no rel edge to 94a7. |
| bt-h1hl | 4 | feature | 0d | (brainstorm) | DoltLite events store | Brainstorm — no umbrella, by design |
| bt-8lz1 | 3 | feature | 14d | area:infra, brainstorm | Workspace stack (bt+tpane+cnvs) | Loosely under z9ei but no edge |
| bt-7rt4 | 2 | feature | 14d | area:search, ux | / from details pane + preserve position | gf3d or qcz8 sister |
| bt-ox4a | 3 | task | 4d | (decision) | Default search-mode decision | Should block bt-hazr; should relate to qcz8 |
| bt-hazr | 3 | feature | 14d | (search) | Switch default search to semantic | Same — depends on ox4a |
| bt-5hkm | 2 | task | 6d | area:tui, brainstorm | Decision: map molecule lifecycle | Probably under 72l8.1 (ghost surface) or its own semantic-features umbrella |
| bt-hq1a | 2 | feature | 6d | area:cli | bt doctor health-check | 53du / 94a7 sister to jov1 |
| bt-7l5m | 2 | decision | 3d | area:analysis,cli,infra,tui | Alert scope: project-only, no global aggregates | Cross-cutting decision; affects ushd analytics |

---

## 2. The umbrella graph

### bt-53du — Product vision (4 declared children + 1 missing)

Declared children (parent=53du): bt-oiaj, bt-lt2h, bt-jov1, bt-z9ei, bt-s4b7. The epic description's stream list also names:

- "Refactor + Charm v2 (bt-if3w)" — bt-if3w not in current open set (likely closed); bt-if3w.1 still open as parent of sprint extraction (stale framing).
- "Lazydev vision (bt-z9ei)" — child link present.

**Scope overlaps:**

- **53du ↔ ushd**: 53du's "CLI/Robot outputs … especially the global layer" overlaps 100% with ushd's "Global visibility … bt queries across all project databases." bt-lt2h (child of 53du) wraps the same `bd audit/stats` surface that bt-4fxz (child of ushd) integrates. **Pick one home for cross-project CLI work.**
- **53du ↔ 94a7**: 53du Pillar 2 (CLI/Robot outputs) is exactly what 94a7's audit produces gap-beads for. 94a7 should arguably be a child of 53du.
- **53du ↔ 72l8.1**: Pillar 1 (TUI) overlaps with the TUI per-view audit (72l8.1.1). 72l8.1 reaching its acceptance criteria (every robot-* + ViewMode classified) directly feeds 53du Pillar 1+2 deliverables.

**Children that should be re-parented:**

- **bt-z9ei → 53du** (already child) — fine. **bt-8lz1** is a sibling-vision bead with NO parent link; either child of 53du or its own meta-vision epic, not floating.
- **bt-3ltq** (multi-repo git correlation) is described in its own bead as "the natural data-layer companion to global Dolt mode" — that's ushd territory, not floating.

### bt-94a7 — Broad upstream audit (5 disc-from edges, 0 children)

Discovered-from edges from 94a7: bt-7czu, bt-a3sb, bt-h5jz, bt-qcz8, bt-vv7o.

**Critical structural problem**: discovered-from is a weaker edge than parent-child. None of the gap beads inherit 94a7's acceptance/closure semantics. When 94a7 closes, the gap beads are orphans.

The bead's own scope guard says "each gap gets a targeted bead" — but the audit itself has not been claimed since it was filed 2026-04-23. Tier-1 deliverables (per-axis findings doc, gap list, parent-child links wiring new beads back to 94a7) are not done. The disc-from edges are ad-hoc filings that PRECEDED the audit, not products OF it.

**Scope overlaps:**

- 94a7 names "bd audit / bd stats features — bt-4fxz tracks this specifically" but bt-4fxz has no rel edge to 94a7.
- 94a7 names "session-id columns (bt-5hl9)" but bt-5hl9 has no rel edge to 94a7.
- 94a7 names "decision type (bt-h5jz)" — disc-from edge present.

**Re-parent candidates:** All five disc-from beads should also gain rel-edges to 94a7 (so 94a7 stays the umbrella) AND, as the ADR-style umbrella for "what bd capabilities does bt expose," should arguably get parent-child links to bt-4fxz, bt-5hl9, and the new bt-rbha (gate surface). bt-rbha is the writable version of "how does bt surface a bd primitive," same pattern.

### bt-72l8.1 — Ghost-features audit (1 child, 0 disc-from yet)

Children (parent=72l8.1): bt-72l8.1.1.

The audit is unstarted; 72l8.1.1 (per-view sweep) is the only declared sub-task. Once executed, this audit will surface follow-up beads for every Stub/Ghost/Partial feature — and those should auto-parent here. The methodology section ("invoke each entry point, observe output, classify") is sound; the audit just hasn't run.

**Scope overlap with 94a7**: 94a7 audits **what bd surface bt should expose** (positive gaps). 72l8.1 audits **what bt code already pretends to expose but doesn't actually work** (negative ghosts). Both are read-side. Together they cover the read-parity surface fully.

**Re-parent candidates:**

- **bt-z5jj** (sprint feature decision) — explicitly cited as the prior example seeding 72l8.1's methodology. Should be a child or rel.
- **bt-3suf** (retire sprint code) — if z5jj decides "retire," 3suf is its execution leg. Both should hang under 72l8.1.
- **bt-if3w.1** (extract sprint view) — written under read-only sprint assumption + pre-Dolt-only context; needs rescoping (per the AGENTS.md stale-checklist) and should come under 72l8.1.
- **bt-08sh** (correlator → Dolt-native) — bt-08sh's own description says it's the root cause of bt-vhn2's mis-framed bug AND that 3 of 7 robot subcommands are broken because of this. That's textbook ghost-feature material. Should be under 72l8.1.
- **bt-ah53** (robot I/O contract) — same shape: contract violations surface as functionally-broken subcommands. Should be under 72l8.1.

### bt-ushd — Cross-project beads OS (9 children, 22% complete)

Declared children: bt-2cvx, bt-4fxz, bt-5hl9, bt-6cfg, bt-ghbl, bt-k9mp, bt-koz8 (closed), bt-mhwy (closed), bt-qk1x.

**Scope overlap with 53du Pillar 2**: bt-lt2h is a 53du child but its post-2026-04-21 comment explicitly scopes it as "wrap upstream commands" for cross-project CLI — that's ushd's mandate. Two epics touching the same line is fine; the work itself needs to know which epic blocks its acceptance.

**Children that should be added:**

- bt-3ltq (multi-repo git correlation)
- bt-mxz9 (cold-boot from non-workspace)
- bt-yqh0 (suffix aggregation)
- bt-ba9f (explicit ID list)
- bt-ssk7 (cross-DB federation — note: marked "already implemented") and bt-dcby/bt-lwdy/bt-gcuv (the global-mode bug cluster)

The lack of these edges is why a fresh session looking at "cross-project work" has to keyword-search instead of scan an epic.

### bt-ph1z — Cross-project mgmt gaps (validates ushd direction)

Per its own description, bt-ph1z mirrors capabilities a real user (pewejekubam, GH#3008) built on the same shared-server architecture. Functionally a sibling-epic to ushd: ushd is bt's roadmap, ph1z is "what the GH-user shipped that we don't." Children bt-ph1z.1..7 already exist (.7 closed). **No structural fix required**, but worth flagging that ph1z and ushd should cross-reference each other explicitly — currently no edge.

### bt-19vp — History view dogfood (no children declared yet)

The epic explicitly enumerates expected children but doesn't have parent edges from existing beads:

- "Children to seed at filing time: bt-XXXX — global-mode error framing" — likely **bt-ezk8** (history broken in global mode, no parent)
- "Related (separate substantial work): bt-XXXX — global git history as a data layer" — that's **bt-3ltq** (no parent)
- "decomposition may be one of the dogfood findings" — implicit follow-up bead pending

---

## 3. Structural problems (rank-ordered)

### 1. The big audits never moved (P0 structural risk)

- **bt-94a7** filed 2026-04-23, never claimed. Its acceptance is a `docs/audit/2026-04-23-...md` report that doesn't exist. Five disc-from gap beads were filed AHEAD of the audit, undermining the "audit produces gaps, gaps get parented back" loop the bead defined.
- **bt-72l8.1** filed 2026-04-27 (today), already has a sub-bead (72l8.1.1) and a methodology, but no claim or output yet.
- **bt-72l8** parent epic also unclaimed.

Blast radius: every parity-style bead (a3sb, vv7o, h5jz, qcz8, 7czu, rbha, mbjg, ah53, 08sh, z5jj, 3suf, if3w.1) is a "gap" filed ad hoc with no central inventory. New gap beads will keep landing in disc-from / floating posture, and a fresh session will keep re-deriving the same map. **These two audits are the highest-leverage unblockers in the cluster.**

### 2. Vague acceptance criteria on vision-tier beads

- **bt-oiaj** (writable TUI). Acceptance is "Users can perform basic CRUD operations on beads from within bt." Definition of "basic" not pinned; the description correctly phases the work (1: bdcli pkg; 2: status keys; 3: create modal; 4: inline edit) but doesn't gate acceptance on a specific phase landing. **Risk under writable-TUI reframe**: fine, this IS the writable-TUI bead — but it should be split into per-phase deliverable beads, with oiaj as the umbrella.
- **bt-z9ei** (lazydev). Acceptance is "bt provides a lazygit-style panel workflow … Read/write capabilities are integrated." Ungated, no measurable threshold. As a vision/brainstorm bead this is acceptable, but it's tagged P2 and labeled feature, not brainstorm.
- **bt-lt2h** (human CLI). Acceptance is "At least one human-readable list command works." Trivial bar; the comment from 2026-04-21 (wrap upstream) is the actual scope and isn't reflected in the acceptance criteria.
- **bt-jov1** (upstream sync). Acceptance is "A mechanism exists to detect upstream beads changes." Very soft. bt-wjzk (P3) is a more concrete sub-shape (CI test) without parent edge.
- **bt-3ltq** (multi-repo correlation). Long description, four ranked acceptance bullets, but the first ("From any cwd, History view shows commit history…") depends on registry.json being populated, which is the substance of bt-mxz9 + a missing bd-side gap. Acceptance hides a hidden dependency.

### 3. Orphans (concrete gap beads, no umbrella parent)

Beads of clear parity/CRUD/global concern with no parent-child edge to any umbrella:

- bt-a3sb, bt-vv7o, bt-h5jz, bt-7czu (have disc-from to 94a7, no parent — see problem 1)
- bt-qcz8 (today; same)
- bt-rbha, bt-mbjg (gate surface) — no edge to anything
- bt-yqh0, bt-ba9f, bt-4ew7 (cross-project / multi-select) — no edge
- bt-3ltq, bt-mxz9, bt-08sh, bt-ah53 (data-layer + correlator + I/O contract) — no edge
- bt-7rt4 (search UX) — no edge
- bt-ox4a + bt-hazr (search default decision) — no edge, no dep relationship between them despite being decision/implementation pair
- bt-hq1a (bt doctor) — no edge
- bt-5hkm (molecule lifecycle decision) — no edge
- bt-z5jj, bt-3suf, bt-if3w.1 (sprint trio) — only the parent-child to bt-if3w (closed/missing) holds them
- bt-8f34, bt-6cfg (cross-prefix linking, project registry) — 6cfg has parent=ushd; 8f34 floats
- bt-7l5m (alert scope decision) — affects ushd analytics, no edge

Blast radius: 20+ floaters. A fresh session finds them only via search-by-keyword or by scanning the full P2 dump.

### 4. Overlap (two beads describing the same gap with different framing)

- **bt-3ltq** (Global git history as a data layer) and **bt-08sh** (correlator → Dolt-native) overlap heavily. 3ltq is "make the existing correlator multiplex over N repos in global mode," 08sh is "rebuild the correlator on Dolt instead of JSONL." 08sh BLOCKS 3ltq — without Dolt-backed correlator, multi-repo correlation just multiplies a broken correlator. **No dep edge between them.**
- **bt-3ltq + bt-ezk8 + bt-thpq + bt-zko2** all live in the "history view doesn't work right in global / Dolt-only world" space. Four beads, no shared parent. 19vp is the natural umbrella.
- **bt-yqh0** (cross-project aggregation by suffix) and **bt-ba9f** (explicit ID list) overlap on "pull up specific beads side-by-side." ba9f's scope notes call yqh0 a "companion." A rel edge exists; no parent.
- **bt-94a7** (broad audit) and **bt-72l8.1** (ghost-features audit) split the read-parity audit into two passes (positive vs negative) — overlap is intentional and correct.
- **bt-mbjg** (hide gate by default) and **bt-rbha** (surface gate via filter/view) are two halves of one feature. mbjg is filed as the precursor and rbha is the explicit follow-up — but they're siblings with no parent linking them.

### 5. Missing dependencies

- **bt-3ltq** depends-on **bt-08sh** (correlator must be Dolt-backed before multi-repo makes sense). No edge.
- **bt-3ltq** depends-on **bt-mxz9** (registry.json populated is in the discovery priority list). Mxz9 has no edge from 3ltq.
- **bt-hazr** (switch default to semantic) depends-on **bt-ox4a** (decision: which default). No edge.
- **bt-rbha** depends-on **bt-mbjg** (rbha is the surface for what mbjg hides). No edge.
- **bt-7czu** (by-assignee view) explicitly says "depends on bt-5hl9" in its description, but no dep edge.
- **bt-oiaj** description states "Depends on global mode (bt-ssk7) being stable first." No dep edge.
- **bt-h5jz** Phase 4 (lifecycle) is gated on upstream confirming `superseded_by` schema — no upstream tracking bead.

### 6. Hung decisions / brainstorms blocking children

- **bt-ox4a** (P3 decision: default search mode) is a hung decision blocking bt-hazr.
- **bt-5hkm** (P2 decision: map molecule lifecycle to bt status) — molecules are a beads primitive (`bd promote`, formula/wisp). Decision pending; nothing downstream filed yet, but if the user wants TUI molecule support it'll need this decision first.
- **bt-h1hl** (P4 brainstorm: DoltLite events store) — pure brainstorm, nothing blocks it. Correctly priced.
- **bt-z5jj** (P3 sprint A vs D decision) — explicitly waiting on 72l8.1 audit data. Dep-edge missing; declared in the bead's notes section.
- **bt-7l5m** (P2 decision: alert scope) — affects ushd analytics direction but no descendents wired.
- **bt-rbha** is filed as Option A/B/C — it's effectively a decision dressed as a feature. Either pick before scheduling, or retag.
- **bt-qcz8** filed today with three options A/B/C and a recommendation — same shape as rbha. Should land a decision before someone starts coding.

### 7. Acceptance criteria that won't survive the writable-TUI reframe

The writable-TUI architecture (bt-oiaj: shell out to bd; dual attribution; Phase 1-4 phasing) was committed but several read-era beads write acceptance under read-only assumptions:

- **bt-94a7** acceptance enumerates schema columns, IssueType enum, slash-command surface — purely read parity. Doesn't mention which bd subcommands bt should be able to **invoke for writes** (create, close, comment, dep, label, gate, promote, etc.). The "29 slash commands" list is read by humans/agents; the bd top-level surface (`bd --help`) is what bt's bdcli wrapper would invoke. 94a7 should be re-stated to cover both axes.
- **bt-72l8.1** acceptance enumerates `--robot-*` subcommands and ViewMode enum. Both are read surfaces. A writable-TUI reframe adds a third axis: **mutation paths** (which bd CLI calls bt makes, what the contract is per call, what error states surface in TUI). This third classification dimension isn't in 72l8.1 today.
- **bt-h5jz** Phase 1 is read-only. Phases 2/3 introduce filter/view; Phase 4 (mark superseded) is the write path — currently optional. With writable-TUI committed, "mark superseded from TUI" stops being optional.
- **bt-rbha** options A/B/C are all read surfaces ("show me gates"). With writable-TUI, the natural fourth option is "resolve a gate from TUI" (which writes via bd). Not in the bead.
- **bt-vv7o** (blocked queue) acceptance is "shows blocked issues." Writable reframe adds: "and lets me unblock from this view" (close the blocking dep, supersede a stale dep, etc.).
- **bt-a3sb** (project-grouped view) acceptance covers grouping. Writable reframe adds: "create a bead in a specific project's group from this view" (which bdcli call, what cwd context).
- **bt-7czu** (by-assignee) acceptance covers filtering. Writable reframe adds: "claim from this view" (one of bt-oiaj Phase 2's keystrokes).
- **bt-lt2h** (human CLI) acceptance is read-shaped (table output). Writable reframe says: agent CLI for writes lands in robot mode; human CLI is read-only by design — confirm the explicit boundary in the bead.
- **bt-3ltq, bt-08sh** (correlator work) — read-only by nature. No reframe issue, but writable surfaces (status changes, claim flow) eventually need a witness path through correlation; flag as out-of-scope.

### 8. Stale (>30d) — not many, the cluster is mostly fresh

Most beads in the cluster are 0-14d. Older outliers:

- **bt-z9ei**, **bt-8lz1**, **bt-jov1**, **bt-oiaj**, **bt-lt2h** — created 2026-03-12 to 2026-04-13, updated 2026-04-13 (so ~14d). Not strictly stale, but they're vision-tier beads that have not been touched since the writable-TUI commit. If 14d → 30d threshold drift continues, these will go stale.
- **bt-xavk** (help redesign, P1) — created 2026-03-12, updated 2026-04-23. The bead correctly notes "with CRUD coming (bt-oiaj), the help system also needs to teach write actions" — already reframe-aware. Good.
- **bt-72l8** epic — created 2026-04-23, updated 2026-04-27. Fresh; section 9 is the ghost-features link.

True stale (>30d, never claimed, no comment since creation): none in the open cluster.

### 9. Misnamed / mistyped beads

- **bt-rbha** is filed as `feature` but reads as a decision (Option A/B/C, "Pick option A/B/C (or hybrid)"). Should be `type=decision`. Same for **bt-qcz8** (today).
- **bt-z9ei** and **bt-8lz1** are `feature` but the labels include `workflow:brainstorm`. Should be either `type=task` with brainstorm label or split into vision-doc + execution beads.
- **bt-h1hl** (today) is correctly a brainstorm — no fix.

---

## 4. Gaps not yet filed (bd subcommand families with zero or weak bt presence)

Cross-referencing `bd --help` top-level subcommands and `~/System/tools/beads/claude-plugin/commands/` against bt's open bead set:

| bd subcommand | bt coverage today | Bead mentioning it? |
|---|---|---|
| `bd assign` / `bd priority` / `bd update` / `bd close` / `bd reopen` / `bd comment(s)` / `bd note` / `bd tag` / `bd label` | NONE in TUI; **bt-oiaj** umbrella covers all | **bt-oiaj** (P2) - phased plan |
| `bd create` / `bd create-form` / `bd q` (quick capture) | NONE in TUI; covered by bt-oiaj Phase 3 | **bt-oiaj** |
| `bd edit` (in $EDITOR) | NONE; bt-oiaj Phase 4 inline-edit is the alternative | bt-oiaj implicit |
| `bd dep` / `bd link` / `bd duplicate` / `bd supersede` | NONE; bt renders graph but cannot mutate edges | **GAP: no bead** for "manage deps from TUI"; oiaj scope notes mention "manage dependencies" |
| `bd gate` (async coordination gates) | hide-by-default and surface-via-filter via **bt-mbjg** + **bt-rbha**; no resolve/create/cancel from TUI | mbjg + rbha cover read; **GAP: no bead** for gate write path |
| `bd merge-slot` (serialized conflict resolution) | NONE | **GAP: no bead** |
| `bd promote` (wisp → permanent bead) | NONE; wisp toggle exists for visibility | **GAP: no bead** for promote action |
| `bd swarm` (structured epic management) | NONE; **bt-8bvm** (referenced in qzgl) was about swarm error UX — likely closed | **GAP: no bead** for swarm management |
| `bd duplicates` / `bd find-duplicates` (semantic dup detection) | None visible | **GAP: no bead**; could overlap with bt search semantic engine |
| `bd memories` / `bd recall` / `bd remember` / `bd forget` (persistent memories) | NONE | **GAP: no bead**; this is a major bd surface bt is silent on |
| `bd kv` (key-value store) | NONE | **GAP: no bead** |
| `bd federation` (peer-to-peer federation) | NONE; bt's global mode is server-shared, not federation-protocol | **GAP: no bead**; **bt-jov1** mentions "Federation protocol changes" as a monitor target but no surface bead |
| `bd lint` (template section check) | NONE | **GAP: no bead** |
| `bd stale` (issues not updated recently) | partial: **bt-8zgy** (IN_PROGRESS freshness) covers staleness signal in bt; no `bd stale` wrapping | bt-8zgy adjacent; **GAP: no bead** for explicit stale-view |
| `bd preflight` (PR readiness checklist) | NONE | **GAP: no bead** |
| `bd context` / `bd where` / `bd info` / `bd doctor` / `bd ping` | partial: **bt-hq1a** (P2) proposes `bt doctor`; nothing for context/where/info | **bt-hq1a** is the only one |
| `bd query` (simple query DSL) | bt has BQL (its own DSL); no integration | **GAP**; bt-qcz8 partially covers ("could use BQL syntax") but doesn't address `bd query` interop |
| `bd search` | bt has its own hybrid search; **bt-qcz8** explicitly covers the integration gap | bt-qcz8 |
| `bd workflow` / `bd template` / `bd quickstart` / `bd prime` / `bd onboard` | NONE | **GAP: no bead**; some are agent-targeted and may not need TUI surfaces |
| `bd config` / `bd dolt` / `bd init` / `bd setup` / `bd hooks` / `bd bootstrap` / `bd upgrade` | NONE; bt has its own config (settings epic bt-fd3k) | bt-fd3k adjacent |
| `bd backup` / `bd compact` / `bd flatten` / `bd gc` / `bd prune` / `bd purge` / `bd migrate` / `bd vc` / `bd branch` / `bd worktree` / `bd sql` | NONE | Maintenance-tier; no bead, by-design opt-out is plausible (bd is the right surface) |
| `bd export` / `bd import` / `bd restore` | partial: bt has its own export; **GAP**: no bd-export integration |
| `bd diff` / `bd history` / `bd graph` / `bd count` / `bd statuses` / `bd types` / `bd state` / `bd set-state` | partial: bt history view, bt insights, bt graph view; `bd state` / `bd set-state` (operational state) NOT in bt | **GAP: no bead** for set-state / state-dimension surface |
| `bd children` (list children of parent) | NONE; bt renders dep tree | bt-fkba (P1, navigation) addresses navigation through children; no dedicated query |
| `bd ready` | partial: bt has analysis equivalents; bt-vv7o is the inverse view | adjacent |
| `bd blocked` | partial: **bt-vv7o** is the bt counterpart | bt-vv7o |
| `bd audit` / `bd stats` | partial: **bt-4fxz** integrates them | bt-4fxz |
| `bd rules` (audit and compact Claude rules) | NONE | **GAP: no bead** |
| `bd rename-prefix` / `bd batch` | NONE | **GAP: no bead**; rename-prefix may not need bt UI |

**Hot gap clusters with no bead at all:**

1. **Memory system** (`bd memories`, `bd recall`, `bd remember`, `bd forget`) — entirely missing from bt. This is a major bd surface for agent workflows; not having a TUI view or a robot wrapper is a parity hole.
2. **Gate write path** — read covered by mbjg+rbha, write (resolve gate, set timeout, create gate, cancel) not filed.
3. **Dep mutation** — bt renders deps but cannot mutate from TUI. oiaj implicitly includes it but no Phase-specific bead.
4. **Operational state** (`bd state` / `bd set-state`) — first-class bd primitive, completely absent from bt.
5. **Federation protocol** — bt-jov1 mentions monitoring it; no surface bead.
6. **Promote / swarm / merge-slot** — wisp/molecule/coordination primitives. `bd promote` and `bd swarm` deserve their own surface beads (closer to ushd than 53du since they're cross-project coordination).

---

## 5. Sequencing observation

**Top 2-3 unblockers (ship these first, the most other work falls out):**

1. **bt-94a7 + bt-72l8.1** as a paired audit pair (filed as P2 task, both unclaimed). Together they produce: a documented inventory of (a) what bd surface bt should expose [94a7] and (b) what bt code already pretends to expose but is dead [72l8.1]. Outputs: two audit docs + a parented set of follow-up beads, each with clear acceptance. The current orphan/disc-from cluster (a3sb, vv7o, h5jz, qcz8, rbha, mbjg, etc.) gets a real home. **Highest leverage in the cluster.** Estimated agent slots: 2 parallel audits × 1-2 sequential depths each.

2. **bt-08sh (correlator → Dolt-native).** Identified in its own description as the root cause of 3 of 7 broken robot subcommands and a blocker for bt-3ltq. Fixing this resurrects `bt robot history`, `bt robot related`, `bt robot impact-network` end-to-end on Dolt-only projects, and unblocks 3ltq's multi-repo extension. Sequential, 1 agent slot.

3. **bt-oiaj Phase 1 (internal/bdcli package).** The 30-line MVP path through bdcli (centralized arg builder + error parser + timeout) is the wedge for every writable feature in the cluster: status changes (oiaj Phase 2), gate resolution (rbha write half), supersede (h5jz Phase 4), promote, merge-slot, etc. Once bdcli exists, every other bead that says "shells out to bd" gets a trivial implementation path. Without it, every writable bead has to relitigate the wrapper design. 1 agent slot, low-medium depth.

**Beads waiting on a decision (not on code):**

- **bt-ox4a** decides default search mode → unblocks bt-hazr.
- **bt-5hkm** decides molecule lifecycle mapping → unblocks any wisp/molecule TUI surface.
- **bt-z5jj** decides A (retire) vs D (rebuild) sprint feature → unblocks bt-3suf or a rebuild bead. Itself waits on 72l8.1 audit data.
- **bt-7l5m** decides alert scope (project-only) → affects ushd analytics direction. No code blocked yet.
- **bt-rbha** is filed as a feature with Options A/B/C → effectively waits on a pick.
- **bt-qcz8** (today) is filed similarly with Options A/B/C → waits on a pick before code starts.
- **bt-oiaj attribution decision** — the bead's NOTES section asks "does bd support structured actor metadata?" — answered upstream (BD_ACTOR is `<rig>/<role>/<name>`; session_id is orthogonal). The bead hasn't been updated to reflect that resolution.

**The "shipped-first cascade" if 94a7 + 72l8.1 + 08sh + oiaj-Phase-1 land:**

- 94a7 produces a real backlog → orphans get parents.
- 72l8.1 retires/rebuilds dead code → if3w.1, z5jj, 3suf collapse into a single retirement decision.
- 08sh fixes broken robot subcommands → 3ltq becomes implementable; ah53's verify-test stops false-positive-ing on already-broken subcommands.
- oiaj Phase 1 gives every "shells out to bd" bead a concrete substrate.
- After that, the v1 vision (53du Pillar 1+2) is mostly assembly: pick from the parented backlog, ship per-phase oiaj.

---

## 6. Writable-TUI reframe impact

Beads written under read-only assumptions that need scope re-statement to include write paths now that bt-oiaj is committed:

| Bead | Current scope | Add for writable-TUI |
|---|---|---|
| **bt-94a7** | Audit `--robot-*`, schema columns, slash commands (read parity) | Add: which bd top-level subcommands bt should INVOKE (write parity); enumerate per `bd --help`; mark per subcommand `read|write|both|skip` |
| **bt-72l8.1** | Classify `--robot-*` and ViewMode as working/stub/ghost/partial (read paths only) | Add: classify mutation paths bt makes (today: zero; tomorrow: bdcli call sites). Acceptance includes "no write path is silently dead" |
| **bt-h5jz** | Decision type read support (enum, filter, view); Phase 4 lifecycle (write) is "optional" | Phase 4 stops being optional; "mark superseded from TUI" is core in writable v1 |
| **bt-vv7o** | Blocked queue read view | Add: unblock action (close blocking dep, supersede stale dep) — lands as one of oiaj Phase 2 keystrokes |
| **bt-a3sb** | Project-grouped read view | Add: create-bead-in-this-project from view (which cwd; how bdcli passes project context) |
| **bt-7czu** | By-assignee read view + filter | Add: claim/release from view (BD_ACTOR + session) — Phase 2 oiaj keystroke |
| **bt-rbha** | Surface gate beads (read) — Options A/B/C | Add Option D: resolve/create/cancel gate (write). Currently a feature with three read options; should be reframed as decision with read+write options |
| **bt-mbjg** | Hide gate by default (filter predicate, read) | No write impact directly; pairs with rbha's reframe |
| **bt-oiaj** | Already the writable-TUI bead | Already reframe-aware; needs split into per-phase deliverable beads (Phase 1 internal/bdcli; Phase 2 status keys; Phase 3 create modal; Phase 4 inline edit) — not currently sub-beaded |
| **bt-yqh0** | Cross-project aggregation by suffix (read) | Add: bulk-edit across the bundle (Phase 5+ writable surface; out of v1 scope) |
| **bt-ba9f** | Pull up specific bead IDs (read filter) | Add: bulk action across the pulled-up set (overlaps bt-4ew7) |
| **bt-4ew7** (today) | Multi-select for bulk copy (read clip) | Already foreshadows bulk-edit ("once selection state exists, the natural follow-on is bulk-edit") — sister to ba9f bulk |
| **bt-lt2h** | Human-readable CLI table output (read) | Confirm explicit boundary: human CLI stays read-only; writes flow through TUI or robot mode. Not a scope expansion, just an explicit guard |
| **bt-jov1** | Monitor schema/CLI drift (informational) | Add: when drift detected, surface in bt-doctor (bt-hq1a) AND in TUI as a status indicator. Reading drift = read; surfacing it = TUI write to status pane |
| **bt-h1hl** | DoltLite events store (brainstorm) | Already write-shaped (events are writes); brainstorm is reframe-neutral |
| **bt-3ltq, bt-08sh, bt-ah53** | Read-side correlator/contract work | Mostly reframe-neutral; once writes ship, witness path through correlation needs verification (bdcli writes should show up in dolt_log + dolt_history_issues). Out of scope for these beads, but should be a follow-on bead |

**One systemic fix**: a "writable-TUI scope guard" boilerplate could be added to AGENTS.md so future read-side beads name the write counterpart explicitly (or write `Out of scope for write path: <reason>`). Same shape as the existing Dolt-only stale-assumption checklist.

---

## Appendix: file paths referenced

- `C:\Users\sms\System\tools\bt\.bt\tmp\epics-full.json`
- `C:\Users\sms\System\tools\bt\.bt\tmp\p1-full.json`
- `C:\Users\sms\System\tools\bt\.bt\tmp\p2-full.json`
- `C:\Users\sms\System\tools\bt\.bt\tmp\details.txt` (raw bd-show dumps for hot beads)
- `C:\Users\sms\System\tools\bt\.bt\tmp\details2.txt` (additional dumps)
- `C:\Users\sms\System\tools\beads\claude-plugin\commands\` (29 slash-command source)
- `C:\Users\sms\System\tools\bt\docs\adr\002-stabilize-and-ship.md` (spine)
- `C:\Users\sms\System\tools\bt\AGENTS.md` (Dolt stale-assumption checklist)
