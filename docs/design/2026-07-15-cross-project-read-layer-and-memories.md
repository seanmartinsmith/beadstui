<!-- Related: bt-2ea7t (tracking epic) ; bt-wxrr5 (memories view), bt-gkda (mindmap consolidator), bt-gd69o (lane decision), bt-423ir (Surface D) -->

# Cross-project read layer + memories surface

- **Status:** Draft (design spec, pending review)
- **Date:** 2026-07-15
- **Author:** design session pc:bt:5e3d0948
- **Type:** Architecture design (feeds a later implementation plan)

## 1. Why

bt aims to be a world-class beads frontend. Two established decisions point the
same direction:

- **bt-gd69o** (Decision): bt does NOT build the agent-orchestration cockpit
  (agent tree / convoy / event stream) — Gas City's dashboard and the
  `gt feed`-class tools own that lane. bt's lane is *reasoning over the bead
  graph* and *consolidating it across projects/rigs*.
- **bt-gkda** (Epic): the cross-project bead-dependency mindmap ("Obsidian
  graph, but beads") — the maintainer's stated #1 want — is blocked on
  cross-project data being second-class.

Both need the same missing thing: a way for bt to read and reason across
**multiple bead sources** with clean origin tagging. This spec designs that
**cross-project read layer** and proves it with the cleanest first payload —
**memories** (`bd remember`) — which today bt cannot see at all. Memories is
greenfield, in bt's reasoning lane, and its cross-project scoping *is* the
consolidator's data layer prototyped small. Shipping it de-risks the graph.

Reference/north-star (informal): a private, unreleased k9s-style TUI cockpit
for Gas City ("g9s", author "neutron") the maintainer flagged as directional —
a Work Board grouped by orchestration *stage*, a live *Sessions* roster, a
*Details* pane of `gc.*` orchestration fields, and a live *Events* stream.
That surface is the **orchestration** lane bt cedes (see Non-goals); it is
recorded here only as the reference that clarified where bt should *not* go.

## 2. Goals / Non-goals

### In scope (v1)
- A **source resolver + detector**: discover candidate scopes, classify each
  as a bd-managed source or a Gas City source, select a read adapter.
- **bd source adapters** for every Dolt mode bt already reads: embedded,
  per-project server, shared-server, and `beads_global`.
- **Origin tagging** rich enough to group/filter results by their source.
- A **memories payload + view** (bd-only): master/detail, grouped by origin
  project, searchable; sourced from `bd memories --json`.
- **Gas City detection-and-exclusion**: recognize gc sources so they do NOT
  flood the bd-centric main views. No gc reading in v1.

### Out of scope (v1) — deliberately deferred
- The **Gas City lens** (a dedicated gc/city view) and its cross-OS
  (Windows bt <-> WSL gc) read adapter. Ships with the graph work, when gc
  usage has clarified what it should show.
- The **mindmap / graph consolidator** (bt-gkda) itself. This layer unblocks
  it; it is not this layer.
- Any **scheduler / cron / reminder** (a separate concern; see bt-nee0a).
- **Gas Town** bespoke support. Gas Town is being wound down in favor of Gas
  City; gt is supported only as a *free rider* (if a gt project reduces to a
  beads/Dolt store the bd or gc paths already read, it works incidentally —
  otherwise skipped).
- **Writing** memories from the TUI (read-only v1).

## 3. Background: the source-kind state space

Two recon passes (bt-side cross-project machinery; gc-side bead access)
established the state space. Key findings:

| Source kind | Store shape | Read path | Detect by | Has memories? |
|---|---|---|---|---|
| bd embedded (default) | `.beads/embeddeddolt`, in-process | shell `bd export` (no server attach) | `.beads/` + `metadata.json` `dolt_mode=embedded`, **no** ancestor `city.toml` | Yes (config table) |
| bd per-project server | `.beads/dolt`, dolt sql-server | Dolt MySQL (bt's `dolt.go`) | `.beads/dolt-server.port`, `dolt_mode=server`, no `city.toml` | Yes |
| bd shared-server | `~/.beads/shared-server` :3308, many DBs | Dolt MySQL, enumerate DBs | shared-server port file/env | Yes (per DB) |
| beads_global | the github-synced global DB | enumerated DB (`bd --global`) | db name `beads_global` (aliased "atlas") | Yes |
| **Gas City city** | per-city Dolt server, **one DB per scope** (`hq` + one per rig, named by prefix) | `gc beads --json` › `/v0` API › direct Dolt | **ancestor `city.toml` (+ `.gc/`)**; at DB-enum layer, `hq`/`gc.*` metadata | **No** (gc never writes `kv.memory.*`) |

Two findings are load-bearing and reshaped the design:

1. **Gas City is NOT one prefix-scoped store.** The accepted gc contract is
   one Dolt server per city with **one database per scope** (`hq` + one per
   rig, database named from the rig's bead prefix; beads prefix-disjoint
   across them). So gc rigs are *separate databases* — which bt's existing
   "enumerate databases on a server, tag by database name" model already fits.
   The thing bt lacks is not prefix-within-DB separation; it is *detecting*
   that a source is gc-managed.

2. **Memories is a bd-only payload.** gc bundles beadslib but its Store
   interface exposes no config/kv/memory methods and gc never reads/writes
   `kv.memory.*`; there is no `/v0` memories endpoint and no `gc beads`
   memories subcommand (gc *does* expose `/v0/city/{city}/config`, but that
   serves its own `city.toml` pack config, not beads kv). A gc city's memory
   namespace is empty. Therefore
   "memories across everything" was never going to include gc, which is
   exactly why memories cleanly proves the *bd half* of the resolver and gc
   correctly lives behind its own (later) lens.

### bt's current cross-project machinery (what exists; the gaps)
- **No unified scope object.** Launch mode is implicit flag precedence in
  `cmd/bt/root.go` `loadIssues()` (`--as-of` › `--workspace` › `--global` ›
  auto-global-when-outside-a-project › local). The only `scope.Mode`
  (`cmd/bt/robot_output.go`) is a *descriptive robot-output label*, not a
  driver.
- **Separation key is coarse:** `issue.SourceRepo` = Dolt **database name**
  only. No source-kind and no prefix-within-database granularity.
- **Two disjoint cross-project pipelines:** shared Dolt server UNION-ALL
  (`internal/datasource/global_dolt.go`, tags rows with `SourceRepo` = db
  name) vs workspace JSONL (`pkg/workspace/loader.go`, namespaces via
  `QualifyID`). No shared model.
- **Two fragmented registries:** `~/.bt/projects.json` (prefix->path, reads /
  History view, weak `.git`-only validation) and `~/.bt/settings.json`
  `project_paths` (db-name->path, writes). Neither records `dolt_mode`.
- **Dolt modes** are discovered in two distinct places (the resolver leans on
  this split): `internal/datasource/source.go` `DiscoverSource()` handles the
  *local* project only -- embedded (via `ReadEmbeddedConfig`, detected first to
  avoid the server-attach deadlock) -> per-project server (`tryDoltSource` /
  `db.Ping`) -> JSONL fallback. **Shared-server discovery is a SEPARATE path**
  (`DiscoverSharedServer`, `global_dolt.go`) invoked from `loadIssues`'
  `--global` / auto-global branches, NOT from `DiscoverSource()`.
- **Zero Gas City awareness.** No rig/town/city handling; the prefix-scoped
  single-store model was explicitly rejected as "not our path"
  (`docs/archive/plans/2026-04-03-global-hub-design.md`).

## 4. Architecture

A **source resolver** sits between scope discovery and the views. It is new
construction (there is no scope object to extend today), but it wraps bt's
existing readers rather than replacing them.

```
scope discovery ── candidate scopes
        │
        ▼
   detector ──────────────► SourceKind (bd-embedded | bd-server |
        │                     bd-shared | beads-global | gascity)
        ▼
   adapter select
        │
   ┌────┴─────────────────────────────────────────┐
   │ bd adapters (embedded/server/shared/global)   │ gascity: DETECT-ONLY (v1)
   │  - list beads (existing readers)              │  - recognized, then
   │  - list memories (bd memories --json)         │    EXCLUDED from bd
   └───────────────┬───────────────────────────────┘    aggregation
                   ▼
        origin-tagged records ──► payload assembler ──► views (memories, …)
```

### 4.1 Components

- **Scope discovery** — gathers candidate sources from: the current project
  (cwd), the shared Dolt server's enumerated databases, and the projects
  registry. (Workspace-JSONL remains its own path for now; unifying it is
  future work, not v1.)
- **Detector** — classifies each candidate into a `SourceKind`. Two entry
  points, *not* co-equal (filesystem primary; DB-enum a cheap defensive guard --
  see 4.3):
  - *Filesystem path:* walk up from a source's directory to the nearest
    `city.toml` → gascity; else `.beads/` + `metadata.json` `dolt_mode` →
    bd-embedded/bd-server.
  - *DB-enumeration path:* when classifying enumerated databases on a shared
    Dolt server (no filesystem context), use an in-store signal — database
    named `hq`, or presence of `gc.*` keys in `issues.metadata` — to mark a
    database as gascity and exclude it.
- **Adapters** — one per source kind, behind a small read interface. bd
  adapters wrap the existing `internal/datasource` readers (embedded shells
  `bd export`; server/shared use the Dolt readers) and add a memories read
  (`bd memories --json`). The gascity adapter in v1 implements detection only;
  its read methods are a designed interface with a not-implemented body.
- **Origin tagging** — every record is tagged with an `Origin{ SourceKind,
  Scope (db/rig name), Prefix, DisplayName }`. This is a superset of today's
  `SourceRepo`; existing consumers keep working off `SourceRepo`, new views use
  the richer `Origin`.
- **Payload abstraction** — a payload (memories, later beads/graph) declares
  which `SourceKind`s it supports. The resolver only invokes a payload's read
  on supporting sources. Memories declares `{bd-embedded, bd-server,
  bd-shared, beads-global}` — gascity is not a supporting source, so gc is
  skipped for memories by construction (not by special-case).

### 4.2 Memories payload + view
- **Read:** `bd memories --json` per bd source (stable wire; NOT `bd config
  show`, which leaks memories unfiltered, and NOT raw config SQL).
- **Shape:** master/detail — a left list of memory keys grouped by origin
  project, a right reading pane with the full body. Justification: memory
  bodies are 162–514-char single-paragraph prose with **no** metadata column
  (no timestamps/recency available today), so a flat one-line list
  would truncate to uselessness and there is nothing else to put in columns.
- **Search:** across keys + bodies, within the current scope.
- **Not beads:** no status/priority/deps/graph; do not route memories through
  the issue-list machinery.
- **Recency:** deferred. A "last-referenced" affordance waits on upstream per-key-timestamp / validity-window
  work (issue refs #4605/#3539 from earlier recon, unverified offline); bt
  cannot source recency today regardless.

### 4.3 Gas City detection-and-exclusion (the only gc-facing v1 work)
The single correctness requirement: gc sources must not silently flood the
bd-centric views. Detection has two entry points, but they are NOT co-equal:

- **Filesystem (primary, and the real gc encounter).** A gascity *rig* has its
  own `.beads/` that looks identical to a standalone bd project in isolation,
  so the detector must **walk up** to the nearest ancestor `city.toml` to know
  it is gc-managed. This is decidable by `os.Stat` alone (no DB connection),
  and it is how bt actually meets a gc source: cwd inside a gc rig, or a gc rig
  previously stamped into `~/.bt/projects.json`.
- **Shared-server enumeration (defensive, near-dead in practice).** bt
  enumerates only its OWN shared server (`~/.beads/shared-server`); a gascity
  city runs a *separate* per-city Dolt server on a dynamic WSL port that bt
  never connects to, so under normal topology bt's enumeration never sees gc
  databases. This path only fires if a gc rig were deliberately
  `bd init --shared-server`'d onto bt's shared server (unusual), and it is NOT
  read-free (it needs a live connection + `SHOW DATABASES`). Build it as at
  most a **cheap defensive `hq`-name guard**, not co-equal machinery; child
  bead #1's scope is narrowed accordingly.

**Detection is heuristic, not authoritative -- optimize against false-positive
exclusion.** Every signal is individually weak: `city.toml` is a plain filename
anyone can create; `hq` is not a reserved database name and collides with any
project literally named `hq`; `gc.*` metadata is stamped contextually
(routing / sessions), never at bead creation, so un-routed beads carry none.
For v1 detect-and-*exclude*, a **false positive** (hiding a real bd project
named `hq`) is more harmful than a false negative (a gc source leaking into the
bd view). So the detector must lean toward *not* excluding on a lone weak
signal, prefer combining signals, and rely on the section 8 "N Gas City sources
hidden" note as the safety net that makes any mistaken exclusion visible and
recoverable.

Excluded gc sources are *remembered* (surfaced later by the gc lens), not
discarded.

## 5. Data flow (memories view load)
1. View requests memories for the current scope.
2. Resolver runs scope discovery → candidate sources.
3. Detector classifies each → `SourceKind`; gascity sources excluded (memories
   is bd-only).
4. For each supporting bd source, the adapter runs `bd memories --json`.
5. Records are origin-tagged and assembled; empty sources yield nothing (no
   error).
6. View renders master/detail grouped by origin; search filters keys+bodies.

## 6. Scope selection UX
Reuse bt's existing posture — do NOT invent a new selector. Cross-project is
already bt's default (shared-server enumeration + the projects registry), so
the memories view inherits the scope the user is already in, and the existing
repo-picker component (`pkg/ui/repo_picker.go`) is reused to filter which
origins are shown -- though it is currently workspace-mode-wired
(`SetActiveRepos`), so wiring it to memory origins is real integration work,
not free. This
keeps memories consistent with how beads are already scoped and avoids a second
scope mechanism.

## 7. Dolt-mode coverage
All four bd modes bt already reads: embedded, per-project server, shared-server,
`beads_global`. No subsetting — the resolver rides bt's existing readers, so
coverage is a wiring concern, not new datastore work. gascity = detect-and-
exclude.

## 8. Error handling
- **Unreachable per-project server:** existing `doltctl.EnsureServer` path
  applies; a source that cannot be read is reported as an unavailable origin,
  not a hard failure of the whole view.
- **Embedded deadlock avoidance:** keep the "never attach a server; shell
  `bd`" rule for embedded (bt-qrt2u).
- **`bd` binary missing:** memories read degrades to "unavailable" for that
  source with a visible note; other sources still load.
- **Empty memories:** a bd source with zero memories contributes nothing;
  the view shows an empty state only when *all* sources are empty.
- **gascity source encountered:** detected and excluded from memories with a
  one-line "N Gas City sources hidden (own lens, coming later)" note, so
  exclusion is visible, not silent.

## 9. Testing
- **Detector unit tests:** each `SourceKind` from fixtures — standalone
  `.beads/`; a gc rig `.beads/` with an ancestor `city.toml` (must classify
  gascity); a shared-server enumeration including an `hq` database (must
  exclude); `beads_global`.
- **Memories adapter tests:** parse `bd memories --json` (flat key→value with
  the stray `schema_version` sibling); empty namespace; multi-source
  aggregation with origin tags.
- **View tests:** master/detail render via the text harness
  (render_harness_test.go / BT_RENDER_DUMP=1); grouping by origin; search;
  empty state; the "gc sources hidden" note.
- **No-regression:** existing `SourceRepo`-based consumers unaffected by the
  added `Origin`.

## 10. Open decisions (resolved in design)
- Population target: **unified** (bd sources now; gascity via its own lens
  later) — not a blend.
- gc in the main viewer: **no** — separate lens; v1 detect-and-exclude only.
- Proof payload: **memories** (bd-only) — cleanly proves the bd half; gc is
  proven later by the beads/graph payload, where it belongs.
- gc lens depth: **detect + route now, view later** — keeps cross-OS transport
  off the memories critical path. (That transport is a *discovery* problem, not
  just reachability: gc's Dolt server binds `127.0.0.1` in WSL on a dynamic port
  derived from the city path, so a Windows bt reader cannot even discover the
  port without a WSL-side artifact -- the later gc-adapter bead must scope port
  *discovery*, not just connection.)
- Gas Town: **free rider only.**

## 11. Staging toward the north star
1. **This spec (v1):** resolver + bd adapters + origin tagging + memories view
   + gc detect-and-exclude.
2. **Next:** beads payload on the same resolver (exercises real cross-project
   beads), then the gascity adapter + Gas City lens (with the cross-OS
   transport decision).
3. **North star:** the cross-project bead-dependency mindmap (bt-gkda) rides
   the same resolver + origin model.

Each step ships standalone value and reuses the layer beneath it.
