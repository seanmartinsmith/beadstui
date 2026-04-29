# Brainstorm: Post-Takeover Roadmap

**Date**: 2026-03-12 (Session 13)
**Status**: Draft
**Participants**: sms + Claude

## What We're Building

bt's next chapter. The fork takeover (ADR-001) is complete - all 4 streams done. Now we need to figure out what we actually have, what we want, and how to get there.

**Aspiration**: bt becomes the official companion TUI for beads - the go-to interface for visualizing and managing issues. Gets there by being genuinely useful for daily workflow, not by rushing features.

**Near-term reality**: No deadlines. Iterate for personal workflow. Build by using.

## Current State

- **Codebase**: ~88k lines production Go, ~102k lines tests, 431 files
- **Big files**: model.go (8.3k), main.go (8.1k), history.go (3.4k), tutorial.go (2.5k)
- **Inherited features**: Board view, graph view, tree view, insights, triage scoring, label health, semantic search, export, tutorial system, WASM module, flow matrix, history view
- **Unknown**: How much of this works with Dolt, how much is dead, how much is hidden gold
- **14 open beads** across UX polish, Dolt integration, and audit work

## Key Decisions

### 1. Phases: Audit -> Stabilize -> Polish -> Interactive

Four phases, each with internal structure:

**Phase 1: Audit** (2 sessions: scan + synthesis)
- Produce a complete feature inventory + architecture map
- Factual dimensions scored by agents: works? tested? Dolt-compatible? how big? what depends on it?
- Human judgment (sms) applied for keep/cut/improve decisions during synthesis
- Output: feature inventory document + architecture map

**Phase 2: Stabilize** (scope TBD by audit findings, likely multiple sessions)
- Cut dead code, remove/archive features marked for removal
- Fix broken features worth keeping
- Ensure the repo is in good shape to build upon
- May involve multiple cleanup passes (delete -> fix -> verify)
- Output: clean codebase, updated test suite, ADR-002

**Phase 3: UX Polish** (can overlap with Phase 2 for quick wins)
- Nielsen heuristics as the north star
- Visual consistency (rounded borders, theme alignment, contrast fixes)
- Mouse support, smarter status messages, responsive layouts
- Upstream research happens here (beads_viewer issues, competition scan)
- Output: a TUI that feels good to use daily

**Phase 4: Interactive** (the killer feature)
- CRUD from the TUI: create, edit, close, comment on beads without leaving bt
- Architecture: shell out to `bd` for writes, poll for changes via Dolt
- No beads fork needed - `bd` already has full write support
- Output: bt becomes a complete workflow tool, not just a viewer

### 2. Audit Method: Domain-Based Teams

Split the codebase by domain, each team produces a structured report:

| Team | Scope | Key Questions |
|------|-------|---------------|
| **UI Views** | `pkg/ui/` views (~30k lines) | What views exist? Which work with Dolt? Which feel finished? |
| **UI Core** | `pkg/ui/` state/behavior (~28k lines) | Model architecture? Keyboard map? State complexity? |
| **CLI/Entry** | `cmd/bt/` (9k lines) | What flags/modes exist? What's the startup flow? What's dead? |
| **Analysis Engine** | `pkg/analysis/` | Graph, triage, label health, insights - what's useful? What needs Dolt adaptation? |
| **Export** | `pkg/export/` | What export formats exist? Which work? |
| **Data Layer** | `internal/datasource/`, `internal/dolt/` | How does data flow? What's the Dolt integration surface? |
| **Agents** | `pkg/agents/` | What's the robot/agent mode? Is it working? |
| **Tests** | `*_test.go` (102k lines) | What's tested? What's broken? What tests are for dead features? |
| **Misc/Config** | `pkg/drift/`, `bv-graph-wasm/`, root files | What's drift? Is WASM alive? Dependency audit. |

### 3. Audit Rubric: Facts from Agents, Taste from Human

Agents produce factual scores per module/feature:
- **Functional**: Does it run without errors? Does it produce visible output?
- **Dolt-compatible**: Does it use the Dolt data path, or still wired to JSONL/SQLite?
- **Tested**: Does it have tests? Do they pass?
- **Size/complexity**: LOC, cyclomatic complexity, number of dependencies
- **Documentation**: Is it described in help text, AGENTS.md, README?
- **Connectivity**: What other modules depend on it? What does it depend on?

Human (sms) applies judgment:
- Is this feature useful for my workflow?
- Is this feature cool/underutilized/hidden?
- Keep, cut, improve, or investigate?

### 4. Write Path: Shell Out to bd

bt stays a pure UI layer:
- **Reads**: Direct Dolt SQL queries (existing poll mechanism)
- **Writes**: Shell out to `bd create`, `bd update`, `bd close`, etc.
- **Feedback loop**: Dolt poll picks up changes within 5 seconds
- **No beads fork needed**: `bd` already has full CRUD + diff + history

### 5. Multi-Session Alignment

Prevent drift across sessions:
- **ADR-002** becomes the spine document (replaces ADR-001's role)
- Feature inventory is a reference artifact, linked from ADR-002
- Architecture map is a reference artifact, linked from ADR-002
- Each session updates ADR-002 changelog
- Beads issues track individual work items
- Epics group related beads

### 6. Tooling

Use everything available:
- **Parallel subagents/teams** for the audit pass
- **Compound engineering skills** (`/ce:plan`, `/ce:review`, `/ce:work`)
- **Superpowers** (brainstorming, TDD, verification, dispatching parallel agents)
- **Code simplifier** (`/simplify`) for cleanup passes
- **bt itself** for tracking the work (dogfooding)

## Why This Approach

- Audit first because we don't know what we have. Can't make good decisions about what to build/cut without understanding the full surface.
- Domain-based teams because 88k lines is too much for one agent pass. Parallel analysis with structured output lets us synthesize efficiently.
- Facts-from-agents, taste-from-human because UX judgment can't be automated. The agents find what's there; sms decides what matters.
- bd shell-out for writes because it's the clean architecture. Zero beads modification, maintains all invariants (events, audit trail, wisps), and bt stays a UI layer.
- ADR-002 as the spine because ADR-001 proved the pattern works for multi-session alignment.

## Resolved Questions

1. **Audit session structure**: Three sessions - THIS session designs the process, Session A executes the scan (parallel domain teams), Session B synthesizes (review reports, make decisions, create ADR-002).
2. **Tutorial system**: Keep. It's a 30-page walkthrough with pagination, tips, and keyboard hints - the bones are solid. Clean up during UX polish phase, then extend when CRUD lands. It's a differentiator for bt's aspiration as the official companion TUI.
3. **Taste vs automation**: Agents produce facts (works? tested? Dolt-compatible? size? dependencies?). Human applies judgment (keep/cut/improve). Don't try to automate taste.

## Open Questions

1. **beads_viewer issues**: When do we scan Jeffrey's GitHub issues for most-requested features? During UX phase (current plan) or during audit for additional context?
2. **Competition**: Are there other beads TUIs? Worth a quick scan before or during the audit.
3. **WASM module** (bv-graph-wasm/): Is the graph visualization worth keeping? Separate build target, separate audit consideration. (Team 8 will evaluate during Session A.)

## Existing Beads That Feed Into This

### UX Polish cluster
- bt-b8gl (P2): Status bar filter badge contrast
- bt-ks0w (P2): Mouse click support
- bt-spzz (P2): Smarter reload status
- bt-95mp (P2): Alerts panel visual refresh
- bt-46fa (P3): Column header redesign
- bt-2bns (P3): Center divider in details pane
- bt-lgbz (P2): Card expand broken with empty columns

### Dolt Integration cluster
- bt-tebr (P2): Auto-start Dolt on launch
- bt-thpq (P3): Dolt changelog/history view
- bt-ztrz (P3): Manual Dolt refresh keybind

### Audit cluster
- bt-79eg (P3): Test suite audit
- bt-pfic (P3): CLI ergonomics audit

### Standalone
- bt-xavk (P1): Help system redesign (has its own plan)

## Next Steps

1. ~~Design the audit process~~ - DONE: docs/archive/plans/2026-03-12-codebase-audit-plan.md
2. Resolve remaining open questions (above)
3. Execute Phase 1 audit (Session A: scan, Session B: synthesis)
4. Create ADR-002 skeleton (during Session B)
