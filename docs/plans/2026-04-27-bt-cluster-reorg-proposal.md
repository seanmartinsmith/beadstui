# bt cluster reorganization proposal

**Date**: 2026-04-27
**Author**: synthesis from 4 parallel recon agents
**Inputs**:
- `.bt/tmp/bt-cluster-map.md` — bead graph + structural problems (418 lines)
- `.bt/tmp/bd-surface-map.md` — bd's 70+ subcommand surface
- `.bt/tmp/tui-productization-gap.md` — bt vs lazygit/k9s/lazyjj gap analysis (480 lines)
- `.bt/tmp/writable-tui-design-surface.md` — writable-TUI architecture (1028 lines)

---

## 1. Diagnosis: why "filed months ago, never acted on"

Six structural failures combined to freeze the cluster:

1. **Audits define methodologies but were never claimed.** bt-94a7 (parity audit, 4d old) and bt-72l8.1 (ghost-features audit, 0d) both have well-scoped methodologies. Neither has produced its audit doc. Nobody owns them.

2. **Gap beads were filed AHEAD of the audit, not as products OF it.** Five beads (a3sb, vv7o, h5jz, qcz8, 7czu) hang off bt-94a7 via `discovered-from` only — a weaker edge than parent-child. When 94a7 closes, they orphan.

3. **20+ orphan beads with no umbrella.** Entire writable-TUI surface (rbha, mbjg, ba9f, yqh0, 4ew7), data-layer cluster (3ltq, 08sh, mxz9, ah53), and search/decision pairs (qcz8, ox4a, hazr) all float. Fresh sessions must keyword-search instead of scanning an epic.

4. **Vague acceptance criteria on vision-tier beads.** bt-oiaj's acceptance is "Users can perform basic CRUD operations" — un-shipable. bt-z9ei: "Read/write capabilities are integrated" — ungated. bt-lt2h: "At least one human-readable list command works" — trivial bar that doesn't reflect actual scope.

5. **Three umbrellas overlap without explicit relationships.** bt-53du (product vision), bt-94a7 (parity audit), bt-72l8.1 (ghost-features audit), bt-ushd (cross-project) all touch overlapping territory. bt-lt2h sits under 53du but its scope is ushd's mandate. bt-94a7 should logically be a child of 53du. None of these structural hierarchies are wired.

6. **The productization foundation isn't there for writes to land on.** Per the productization gap report: bt has no unified verb-entry primitive (no command palette / leader key), no action-context model (panels are passive renderers), no mutation-feedback patterns. **The keymap will fight every new verb if these don't get built first.**

---

## 2. The proposed canonical hierarchy

Apex: **bt-53du (Product Vision: bt v1)**. Already P1 epic with three pillars. Make the hierarchy explicit and add a fourth foundational pillar:

```
bt-53du (P1 epic — Product Vision: bt v1)
├── PILLAR 1: Writable TUI (daily-driver maintainer experience)
│   ├── bt-oiaj (re-scoped per writable-TUI §4)
│   │   ├── bt-oiaj.1 — internal/bdcli wrapper package [NEW]
│   │   ├── bt-oiaj.2 — claim/close/reopen hotkeys [NEW]
│   │   ├── bt-oiaj.3 — create modal [NEW]
│   │   ├── bt-oiaj.4 — quick-capture (q) [NEW]
│   │   ├── bt-oiaj.5 — inline title/priority/status/assignee [NEW]
│   │   ├── bt-oiaj.6 — long-form modals via tempfile [NEW]
│   │   ├── bt-oiaj.7 — label picker write mode [NEW]
│   │   ├── bt-oiaj.8 — write-aware help overlay [NEW]
│   │   └── bt-oiaj.9 — cross-DB write routing [NEW]
│   ├── bt-z9ei (lazydev vision)
│   ├── PRODUCTIZATION FOUNDATIONS [NEW BEADS]
│   │   ├── bt-XXX1 — Command palette / leader-key verb primitive
│   │   ├── bt-XXX2 — Action-context model (panels-as-actions)
│   │   └── bt-XXX3 — Mutation feedback patterns (spinner, toast, $EDITOR suspend)
│   ├── TUI VIEWS [re-parented]
│   │   ├── bt-a3sb (project-grouped)
│   │   ├── bt-vv7o (blocked queue)
│   │   ├── bt-7czu (by-assignee)
│   │   ├── bt-ba9f (explicit ID list)
│   │   ├── bt-yqh0 (suffix aggregation)
│   │   └── bt-4ew7 (multi-select)
│   └── SEARCH UX [re-parented]
│       ├── bt-7rt4 (/ from details)
│       ├── bt-qcz8 (search composition)
│       ├── bt-ox4a (default mode decision)
│       └── bt-hazr (switch to semantic) [depends-on bt-ox4a]
│
├── PILLAR 2: bd surface coverage (read + write parity)
│   ├── bt-94a7 (re-scoped to BOTH read AND write axes)
│   │   ├── bt-h5jz (decision type — Phase 4 now required)
│   │   ├── bt-rbha (gate surface — re-framed as decision with Option D)
│   │   ├── bt-mbjg (default-hide gates)
│   │   ├── bt-4fxz (audit/stats — split off --via upstream proposal)
│   │   ├── bt-5hl9 (session columns)
│   │   └── NEW bd-surface gap beads:
│   │       ├── Memory system (memories/recall/remember/forget)
│   │       ├── Operational state (bd state / set-state)
│   │       ├── Promote action (wisp → bead)
│   │       ├── Swarm management
│   │       ├── Merge-slot coordination
│   │       ├── Find-duplicates (semantic dup detection)
│   │       ├── Lint (template completeness)
│   │       └── Stale view (bd stale wrapper)
│   ├── bt-72l8.1 (re-scoped to include mutation paths + read-only-by-design tag)
│   │   ├── bt-72l8.1.1 (per-view audit)
│   │   ├── bt-08sh (correlator → Dolt-native) — root cause of 3 broken robot subcommands
│   │   ├── bt-ah53 (robot I/O contract)
│   │   ├── bt-z5jj (sprint A vs D decision)
│   │   ├── bt-3suf (sprint retire — execution leg)
│   │   └── bt-if3w.1 (sprint extraction — superseded by z5jj)
│   ├── bt-lt2h (human CLI — read-only by explicit design)
│   ├── bt-jov1 (upstream sync monitor)
│   ├── bt-hq1a (bt doctor)
│   ├── bt-wjzk (CI-side schema drift — automated successor to 94a7)
│   └── bt-5hkm (molecule lifecycle decision)
│
├── PILLAR 3: Cross-project / Global (bt-ushd as sub-pillar)
│   ├── bt-ushd (cross-project beads OS)
│   │   ├── bt-ssk7 (cross-DB federation — exposes per-issue DB mapping)
│   │   ├── bt-3ltq (multi-repo correlation) [depends-on bt-08sh, bt-mxz9]
│   │   ├── bt-mxz9 (cold-boot from non-workspace)
│   │   ├── bt-ghbl (no raw SQL)
│   │   ├── bt-k9mp (cross-project filing)
│   │   ├── bt-2cvx (session author provenance)
│   │   ├── bt-6cfg (cross-prefix linking)
│   │   ├── bt-8f34 (project registry)
│   │   ├── bt-4jyd (global cross-project audit)
│   │   ├── bt-7l5m (alert scope decision — affects ushd analytics)
│   │   ├── GLOBAL-MODE BUG CLUSTER [re-parented]:
│   │   │   ├── bt-dcby (project filter not respected)
│   │   │   ├── bt-lwdy (filter reset on poll)
│   │   │   └── bt-gcuv (priority hints global vs filtered)
│   │   └── bt-ammc (docs)
│   ├── bt-ph1z (cross-project mgmt gaps — sibling to ushd)
│   └── bt-19vp (history view dogfood — cross-cuts global mode)
│       ├── bt-ezk8 (history broken in global)
│       ├── bt-zko2 (denominator wrong)
│       ├── bt-npnh (history small dimensions)
│       └── bt-thpq (Dolt changelog view)
│
└── PILLAR 4: Foundational / cross-cutting [BLOCKS WRITES]
    ├── bt-tkhq (keybinding research) — ELEVATE P2 → P1
    ├── bt-gf3d (keybinding consistency epic)
    │   ├── bt-gf3d.1 (hotkey audit)
    │   ├── bt-8jds, bt-4dam, bt-rhqs, bt-xron, bt-k8rk (keybind bugs)
    │   └── bt-6q8c (label reform — view:* dimension)
    ├── bt-xavk (help system redesign) — write/read column required
    ├── bt-ks0w (mouse click — no destructive on click)
    └── bt-72l8 (Jeffrey-era leftovers epic — superset of 72l8.1)
```

**Loose / out-of-cluster but related** (not under 53du):
- **bt-h1hl** (DoltLite events brainstorm) — pure brainstorm, leave floating
- **bt-8lz1** (workspace stack vision) — vision-tier sibling to 53du, link via `relates`

---

## 3. Action list (concrete refactor moves)

### 3.1 Re-parent operations (bd link --type parent-child)

| Move | Reason |
|---|---|
| `bd link bt-94a7 bt-53du --type parent-child` | Pillar 2 lives under apex |
| `bd link bt-72l8.1 bt-53du --type parent-child` | Pillar 2 |
| `bd link bt-ushd bt-53du --type parent-child` | Pillar 3 (already epic; document subordination) |
| `bd link bt-h5jz bt-94a7 --type parent-child` | Currently disc-from only |
| `bd link bt-qcz8 bt-94a7 --type parent-child` | Currently disc-from only |
| `bd link bt-a3sb bt-94a7 --type parent-child` | Currently disc-from only |
| `bd link bt-vv7o bt-94a7 --type parent-child` | Currently disc-from only |
| `bd link bt-7czu bt-94a7 --type parent-child` | Currently disc-from only |
| `bd link bt-rbha bt-94a7 --type parent-child` | Gate surface = read-side bd primitive |
| `bd link bt-mbjg bt-94a7 --type parent-child` | Pair with rbha |
| `bd link bt-08sh bt-72l8.1 --type parent-child` | Root cause of 3 broken robot subcommands = ghost-feature material |
| `bd link bt-ah53 bt-72l8.1 --type parent-child` | I/O contract = functional parity |
| `bd link bt-z5jj bt-72l8.1 --type parent-child` | Cited in 72l8.1 as the prior example |
| `bd link bt-3suf bt-72l8.1 --type parent-child` | Sprint execution leg |
| `bd link bt-if3w.1 bt-72l8.1 --type parent-child` | Stale framing, audit feeds rescoping |
| `bd link bt-3ltq bt-ushd --type parent-child` | Natural global companion |
| `bd link bt-mxz9 bt-ushd --type parent-child` | Cross-project bootstrap |
| `bd link bt-ssk7 bt-ushd --type parent-child` | Federation core |
| `bd link bt-dcby bt-ushd --type parent-child` | Global-mode bug |
| `bd link bt-lwdy bt-ushd --type parent-child` | Global-mode bug |
| `bd link bt-gcuv bt-ushd --type parent-child` | Global-mode bug |
| `bd link bt-ba9f bt-53du --type parent-child` (Pillar 1 view) | Cross-project view feature |
| `bd link bt-yqh0 bt-53du --type parent-child` | Suffix aggregation view |
| `bd link bt-4ew7 bt-53du --type parent-child` | Multi-select foundation |
| `bd link bt-7rt4 bt-53du --type parent-child` | Search UX |
| `bd link bt-ox4a bt-53du --type parent-child` | Search decision |
| `bd link bt-hazr bt-53du --type parent-child` | Search default switch |
| `bd link bt-ezk8 bt-19vp --type parent-child` | History dogfood child |
| `bd link bt-zko2 bt-19vp --type parent-child` | History denominator bug |
| `bd link bt-thpq bt-19vp --type parent-child` | Dolt history overlap |
| `bd link bt-19vp bt-ushd --type parent-child` | History dogfood crosscuts global mode |

### 3.2 Add missing dependency edges (bd dep)

| Edge | Reason |
|---|---|
| `bd dep add bt-3ltq bt-08sh` | Multi-repo correlation requires Dolt-backed correlator |
| `bd dep add bt-3ltq bt-mxz9` | registry.json must be populated |
| `bd dep add bt-hazr bt-ox4a` | Switch-default depends on default-decision |
| `bd dep add bt-rbha bt-mbjg` | Gate surface depends on default-hide |
| `bd dep add bt-7czu bt-5hl9` | By-assignee view requires session columns |
| `bd dep add bt-oiaj bt-ssk7` | Cross-DB write routing depends on federation |
| `bd dep add bt-oiaj bt-tkhq` | Write keybinds blocked on key research |
| `bd dep add bt-z5jj bt-72l8.1` | Sprint A-vs-D decision waits on audit data |

### 3.3 Scope updates (bd update --acceptance, --description, --priority)

| Bead | Change |
|---|---|
| **bt-oiaj** | Replace acceptance with the 11-item done-when list from writable-TUI report §4. Description should reference `.bt/tmp/writable-tui-design-surface.md`. |
| **bt-94a7** | Add write-axis to acceptance: each bd top-level subcommand gets `read|write|both|skip` classification. The catalog from writable-TUI §1 is the input. |
| **bt-72l8.1** | Add fifth classification "read-only-by-design" + add mutation-paths axis (which bdcli call sites bt makes). |
| **bt-72l8.1.1** | Acceptance includes write-decision row per ViewMode. |
| **bt-h5jz** | Phase 4 (mark superseded from TUI) stops being optional under writable-TUI commit. |
| **bt-rbha** | Re-frame as `type=decision` with Option D added: gate resolve writes. |
| **bt-mbjg** | Pair with rbha re-frame. |
| **bt-ssk7** | Description note: "Provides per-issue source-database mapping for write routing (consumed by bt-oiaj.9)." |
| **bt-4fxz** | Split: read-side stays scoped; add proposal-upstream sub-task for `--via` flag (dual attribution). |
| **bt-tkhq** | **Priority P2 → P1**. No longer optional research; blocks every write key. |
| **bt-gf3d** | Description: "reserve write-side keys before any non-write rebinding lands." |
| **bt-xavk** | Acceptance: visible distinction between read and write keybinds in help overlay. |
| **bt-ks0w** | Acceptance: no destructive operations bound to mouse-only gestures. |
| **bt-vv7o** | Acceptance: add unblock action (close blocking dep, supersede stale dep). |
| **bt-a3sb** | Acceptance: add create-bead-in-this-project from view (cwd routing). |
| **bt-7czu** | Acceptance: add claim/release from view. |
| **bt-lt2h** | Acceptance: explicit "human CLI stays read-only by design; writes go through TUI or robot mode." |
| **bt-19vp** | Add post-writable acceptance: verify bt-side writes show up correctly in history view. |
| **bt-3ltq** | First acceptance bullet has hidden dependency on bt-mxz9 — make it explicit. |

### 3.4 Type changes (bd update --type)

| Bead | From | To | Reason |
|---|---|---|---|
| bt-rbha | feature | decision | Filed as Options A/B/C |
| bt-qcz8 | task | decision | Filed as Options A/B/C |

### 3.5 New beads to file

**A. Split bt-oiaj into 9 phase children** (`.1` through `.9`):

1. `bt-oiaj.1` — internal/bdcli wrapper package
2. `bt-oiaj.2` — claim/close/reopen hotkeys
3. `bt-oiaj.3` — create modal
4. `bt-oiaj.4` — quick-capture (`q`)
5. `bt-oiaj.5` — inline title/priority/status/assignee edit
6. `bt-oiaj.6` — long-form modals via tempfile
7. `bt-oiaj.7` — label picker write mode
8. `bt-oiaj.8` — write-aware help overlay
9. `bt-oiaj.9` — cross-database write routing

**B. Productization foundation beads (Pillar 1)** — these are the "fix before writes land" prerequisites identified by the productization-gap agent:

1. **Command palette / leader-key verb primitive** (P1 feature) — addresses "no unified verb-entry primitive" finding. Once writes ship, single-letter mnemonics are exhausted; `:` is consumed by BQL. Need leader-key chord support.
2. **Action-context model refactor** (P1 task) — detail panel becomes an action-context, not a passive renderer. Lazygit pattern. Required for write verbs to have a natural home.
3. **Mutation feedback patterns** (P1 task) — spinner architecture, toast for non-modal mutations, `$EDITOR` suspension primitive (`tea.ExecProcess`), shell-out trace ("here's the bd command bt just ran").

**C. bd-surface gap beads (Pillar 2)** — bd subcommand families with zero bt presence:

1. Memory system surface (`bd memories`/`recall`/`remember`/`forget`)
2. Operational state surface (`bd state`/`set-state`)
3. Gate write path (`bd gate resolve/create/cancel`)
4. Promote action (`bd promote` wisp→bead)
5. Swarm management (`bd swarm`)
6. Merge-slot (`bd merge-slot`)
7. Find-duplicates (`bd find-duplicates` — overlaps semantic search)
8. Lint (`bd lint` — template completeness)
9. Stale view (`bd stale` wrapper)
10. Dep mutation from TUI (`bd dep add/remove`/`bd link/unlink`/`bd duplicate`/`bd supersede`)

(File these as gap beads under bt-94a7 once the audit produces them, OR file ahead per the user's existing pattern.)

### 3.6 Beads to retire / supersede

| Bead | Action |
|---|---|
| **bt-if3w.1** | Mark as superseded once bt-z5jj decision lands (sprint extraction is stale framing). |

---

## 4. Sequencing — what ships first

The "shipped-first cascade" identified across all four reports:

### Phase 0 (decision phase, parallel — no code yet)

These are unblockers that need a human pick before any engineering moves:

1. **bt-ox4a** — default search mode (semantic vs hybrid) — unblocks bt-hazr
2. **bt-z5jj** — sprint A (retire) vs D (rebuild) — unblocks bt-3suf
3. **bt-rbha** — gate surface options A/B/C/D (after re-frame) — unblocks gate work
4. **bt-qcz8** — search composition options A/B/C — unblocks any search refactor
5. **bt-7l5m** — alert scope (project-only vs global) — affects ushd analytics
6. **bt-5hkm** — molecule lifecycle mapping — unblocks any wisp/molecule TUI

### Phase 1 (the wedge — 3 beads, ~1 week of agent work)

Ship these and the rest of the cluster becomes assembly:

1. **bt-94a7** (claim + ship the audit doc) → produces parented backlog, dissolves the orphan problem
2. **bt-72l8.1** (claim + ship the audit doc) → retires/rebuilds dead code, collapses sprint trio
3. **bt-08sh** (correlator → Dolt-native) → fixes 3 broken robot subcommands, unblocks bt-3ltq

### Phase 2 (foundational TUI before writes)

Per the productization-gap report's load-bearing claim ("if foundational items are fixed before bt-oiaj lands, writes become a feature delivery"):

1. **bt-tkhq** (key research, NOW P1) → unblocks every write keybind
2. **Command palette / leader-key bead** (NEW) → write verbs need a home
3. **Action-context model refactor** (NEW) → panels-as-actions
4. **Mutation feedback patterns** (NEW) → spinner + toast + $EDITOR

### Phase 3 (writable TUI proper)

bt-oiaj phase children in the order proposed by writable-TUI §4:

1. **bt-oiaj.1** (bdcli wrapper) — the substrate
2. **bt-oiaj.2** (claim/close/reopen) — first end-to-end write
3. **bt-oiaj.4** (quick-capture) — fastest second deliverable, validates create path
4. **bt-oiaj.3** (full create modal)
5. **bt-oiaj.7** (label picker write)
6. **bt-oiaj.5** (inline edits)
7. **bt-oiaj.6** (long-form modals)
8. **bt-oiaj.8** (write-aware help)
9. **bt-oiaj.9** (cross-DB routing — depends on bt-ssk7 stability)

### Phase 4 (Pillar 2 + 3 fill-in)

After writes land, the audit-surfaced gap beads (memory, state, gate writes, promote, swarm, etc.) are assembly work — pick from the parented backlog.

---

## 5. Fresh-session handoff prompt

The format below is what an L8 architect+PM hands to a fresh L8 engineer. Self-contained, ADR-shaped, points to artifacts.

```
You're picking up bt (a Go TUI for the beads issue tracker).

ORIENTATION — read in order, ~10min:
1. AGENTS.md — project conventions, including "Beads architecture awareness"
2. docs/adr/002-stabilize-and-ship.md — the spine
3. .bt/tmp/bt-cluster-reorg-proposal.md — this proposal
4. bt-53du (apex epic, P1) — the v1 product vision

CURRENT STATE:
- bt is read-only TUI today; bt-oiaj tracks transition to writable
- Backend integrity boundary: bt shells out to bd CLI for ALL writes;
  reads via Dolt MySQL protocol against bd's server
- Apex epic is bt-53du, four pillars (TUI, bd-surface coverage,
  cross-project, foundational)

WHAT'S READY TO CLAIM (P1 unblockers, in order):

1. bt-94a7 (parity audit) — produces the backlog every other gap bead
   needs as a parent. Acceptance: docs/audit/2026-04-23-bt-upstream-
   capability-audit.md committed; per-axis findings; gap beads filed
   AND parent-linked back. This bead has been unstarted for 4 days —
   that's the structural rot we're fixing.

2. bt-72l8.1 (ghost-features audit) — sister to 94a7. Produces
   classification of every --robot-* subcommand and ViewMode as
   working/stub/ghost/partial/read-only-by-design.

3. bt-08sh (correlator → Dolt-native) — root cause of 3 broken
   robot subcommands. Independent of audits; can run in parallel.

DECISION-WAITING (need a human pick before code):
- bt-ox4a, bt-z5jj, bt-rbha, bt-qcz8, bt-7l5m, bt-5hkm

DO NOT START until Phase 1 ships:
- Any bt-oiaj.* phase child (writes need foundation first)
- Any new view bead (a3sb, vv7o, 7czu) — wait for 94a7's audit doc
- Any productization foundation bead — wait for bt-tkhq research

WORKING POLICY:
- Read close_reason on related beads before starting (they're often
  decision-recorded already — see the project's bd memory feature)
- Use bd update <id> --claim before working
- Close with the structured Summary/Change/Files/Verify/Risk/Notes
  format from .beads/conventions/reference.md
- Push at end of session: git pull --rebase; bd dolt push; git push

WHEN IN DOUBT:
- Cross-reference AGENTS.md "Stale-assumption checklist" — many
  pre-Dolt-only beads have stale framing
- Use bt-mhcv (if still open) as the audit ground-truth for Dolt
  migration awareness
- /ce:plan or /ce:brainstorm if scope is unclear before coding
```

---

## 6. Effort estimate (agent-slots × sequential depth)

Per CLAUDE.md framing — no time estimates, just parallelism × depth.

**Phase 0 (decisions)**: 1 slot × 6 sequential decisions. User-driven; ~1 session if user is engaged.

**Phase 1 (audit wedge)**: 2 parallel slots (94a7, 72l8.1) + 1 separate slot (08sh) × ~2 sequential depths each. ~2 agent sessions.

**Phase 2 (productization foundations)**: 1 slot × ~4 sequential beads (tkhq → palette → action-context → mutation feedback). Could parallelize tkhq and the design phase of the others. ~3-4 sessions.

**Phase 3 (writable TUI)**: 9 oiaj phase children. bdcli (oiaj.1) is sequential blocker; .2 and .4 can run in parallel after; .3, .5, .6, .7, .8 fan out; .9 waits on bt-ssk7. ~6-8 sessions.

**Phase 4 (Pillar 2/3 fill-in)**: assembly work. Parallelize freely.

Total: writable v1 lands in ~12-15 agent sessions if Phase 0 decisions are made promptly. Without Phase 0, the cluster stays frozen.

---

## 7. The systemic fix (write this once, applies forever)

Add a "writable-TUI scope guard" boilerplate to AGENTS.md. Same shape as the existing Dolt-only stale-assumption checklist. Every new TUI-area bead must include either:
- A write-side acceptance row, OR
- An explicit "Out of scope for write path: <reason>" note

This prevents the read-era-acceptance drift that hit ~14 existing beads.

---

## Appendix: source reports

- `.bt/tmp/bt-cluster-map.md` (418L)
- `.bt/tmp/bd-surface-map.md`
- `.bt/tmp/tui-productization-gap.md` (~480L)
- `.bt/tmp/writable-tui-design-surface.md` (1028L)
