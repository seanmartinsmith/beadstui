---
title: "ADR-003: Data Source Architecture Post-Dolt-Migration"
status: accepted
date: 2026-04-25
decided: 2026-04-25
decision: option-b-collapse
decision-makers: [seanmartinsmith]
parent: 002-stabilize-and-ship.md
related-beads: [bt-mhcv, bt-08sh, bt-z5jj, bt-uahv, bt-3ltq, bt-05zt]
foundation: docs/audit/2026-04-25-data-source-architecture-survey.md
---

# ADR-003: Data Source Architecture Post-Dolt-Migration

## Status

**Accepted (2026-04-25).** Decision: **Option (b) — Collapse: refactor to `Dolt | DoltGlobal | JSONLFallback`.**

Implementation tracked in **bt-05zt**.

This ADR sits under [ADR-002](002-stabilize-and-ship.md) ("Stabilize and ship") as a child decision document, extending the "post-Dolt migration of bt-derived data layer" sub-stream surfaced 2026-04-25.

## Decision

Option (b). Reasoning matches the recommendation below: the SourceType abstraction has outlived its multi-backend purpose, and aligning code shape with the actual `Dolt or fallback` decision space now is preferable to deleting dead code only to revisit the abstraction in a few months. Option (a) was undersized for the architectural decay; (c) was oversized without evidence of incoming backend pluralism.

The options analysis below is preserved as historical context.

## Context

bt's `internal/datasource/` package was designed in an era when beads supported multiple backends — SQLite, JSONL, and Dolt all coexisted. The package's shape reflects that: a 5-element `SourceType` enum, a priority constant per source, a discovery pipeline that finds all candidates and a selector that picks the freshest, plus per-source readers and validators. The TUI cold-load path goes through this layer correctly today.

That world ended at **beads v0.56.1** (SQLite removal) and was finalized at **v1.0.1** (March 2026: Dolt-only system of record; JSONL is opt-in export, not a backend). Two consequences for bt:

1. **`SourceTypeSQLite` is dead weight.** 397 lines of reader code, plus discovery, validation, priority, and tests, still in the build. Any project on current `bd` will never have `.beads/beads.db`. The branch is considered on every load.

2. **The abstraction's value proposition has collapsed.** Smart multi-source discovery+priority+selection made sense when there were genuinely multiple co-equal backends to choose between. With Dolt as the only system of record and JSONL as opt-in legacy, we're really just answering "Dolt available? use it. Else legacy file present? use it. Else error." That's not a multi-source problem anymore.

See `docs/audit/2026-04-25-data-source-architecture-survey.md` for the full inventory.

## Decision options

Three shapes for the post-decision data layer. Each is a coherent target; the trade is between churn and conceptual clarity.

### Option (a) — Minimal: remove SQLite, leave abstraction intact

**What changes:**
- Delete `internal/datasource/sqlite.go` (397 LOC).
- Remove `SourceTypeSQLite`, `PrioritySQLite`, `discoverSQLiteSources`.
- Remove SQLite branches in `LoadFromSource`, `ValidateSource`, `source_test.go`.
- The 5-element `SourceType` enum becomes 4-element. The abstraction's structure stays.

**Blast radius:** small. ~500 LOC delta concentrated in `internal/datasource/`. No consumer changes. No test rewrites beyond removing SQLite cases.

**Pros:**
- Conservative. Doesn't disturb anything that works.
- Ships fast — could be a single PR.
- Reversible if upstream beads ever brings SQLite back (unlikely, but).

**Cons:**
- Doesn't address the "abstraction without purpose" problem. A 4-source priority system for what's effectively a 2-source decision (Dolt or legacy) carries unnecessary cognitive weight.
- The discovery+priority+selector machinery still runs every load to choose between sources we don't really care to compare.
- Leaves the next maintainer wondering why we have priority constants at all.

### Option (b) — Collapse: refactor to `Dolt | DoltGlobal | JSONLFallback`

**What changes:**
- Everything in option (a), plus:
- `SourceType` becomes 3 values: `Dolt`, `DoltGlobal`, `JSONLFallback` (collapses `JSONLLocal` and `JSONLWorktree`, since the distinction stops mattering once Dolt is the canonical source).
- Priority constants gone. Discovery logic simplifies to: "Dolt configured? try it. Else JSONL file present? use it. Else error."
- `SelectBestSource` either disappears or becomes trivial.
- `RequireDolt` flag goes away — Dolt is always preferred when configured; absence of Dolt config means legacy mode.

**Blast radius:** medium. Touches `internal/datasource/` substantially, plus consumers that switch on `SourceType.Type` (the global mode block in `cmd/bt/root.go`, the poll loop dispatcher in `pkg/ui/background_worker.go:1990`). Test rewrites in `source_test.go`.

**Pros:**
- Code shape matches the actual decision space.
- Removes priority math that doesn't earn its keep.
- Easier mental model for new maintainers and AI agents.
- Less surface area for the kind of stale-architecture bugs bt-mhcv is trying to root out.

**Cons:**
- More churn than (a). Not free.
- Loses the `Worktree` vs `Local` distinction — if there's a use case for treating worktree JSONL differently from regular JSONL, we'd need to recover it elsewhere.
- Touches consumer code; small but non-zero coupling risk.

### Option (c) — Reframe: `Backend` interface + separate `Importer`

**What changes:**
- Everything in option (a), plus:
- Introduce a `Backend` interface for live data sources — only Dolt today (per-project + global both behind the same interface, distinguished by config).
- JSONL becomes a separate concept: an `Importer` for legacy files, used only when no `Backend` is configured. It's not a "source" — it's a different kind of thing (one-shot file load, no live connection, no polling).
- Discovery splits in two: backend resolution (one pass) + importer fallback (only if no backend).
- `SourceType` enum disappears entirely; replaced by `Backend` implementations and an `Importer` value type.

**Blast radius:** large. The biggest churn. Touches `internal/datasource/` deeply, the `model_update_data` poll loop, the load orchestration in `cmd/bt/root.go`, and likely most tests in `internal/datasource/`. May surface refactoring opportunities elsewhere (e.g., the watcher layer becomes `Backend`-aware instead of `SourceType`-switching).

**Pros:**
- Cleanest semantically. Live backends and one-shot importers are different concepts; treating them as both "sources" was always a bit forced.
- Best long-term ergonomics for adding future backend modes (e.g., a federated remote-Dolt mode, or read-only snapshot mode).
- Aligns most naturally with the "Dolt is the system of record, everything else is migration support" reality.

**Cons:**
- Largest scope. Real risk of introducing regressions during the refactor.
- Bikesheddable interface design.
- Probably overkill for a project at bt's current stage. The interface generality doesn't earn its keep until there's a second backend on the horizon, which there isn't.

## Recommendation

**Option (b).** Reasoning:

- (a) is undersized for the actual architectural decay. It deletes dead code without addressing the design rot — the next agent reading `internal/datasource/source.go` still has to puzzle out why we have priority math and a discovery+selection pipeline for what's effectively a binary "Dolt or fallback" choice. We'd ship (a) and re-litigate this in 3 months.
- (c) is oversized for current need. Generality for hypothetical future backends. We don't have evidence that backend pluralism is coming back.
- (b) matches the actual decision shape. The code becomes self-documenting: there's Dolt, there's a legacy fallback, there's nothing else. The discovery pipeline shrinks to what it actually needs to do.

(b) also pairs naturally with bt-mhcv's audit goal: cleaner architecture surfaces stale assumptions earlier and makes future migrations less risky.

**This is a recommendation, not a decision.** The owner should pick.

## Decision criteria for the owner

- **Pick (a) if** you want the smallest possible PR and are okay revisiting in a few months.
- **Pick (b) if** you agree the SourceType abstraction has outlived its purpose and want to align code shape with reality now.
- **Pick (c) if** you have concrete reason to expect a second live backend (federated Dolt, snapshot mode, alternative storage) within the next 6 months.

## Migration path

Independent of which option:

1. Foundational beads land first: bt-08sh (correlator → Dolt), bt-z5jj (sprint decision), bt-uahv (data-home split). These don't depend on this ADR but should not be in flight when the SourceType refactor lands.
2. The implementation bead for this ADR (filed alongside) gets unblocked by the decision.
3. Implementation order, regardless of option:
   - Phase 1: SQLite removal (mechanical, safe under all three options). Could ship independently as a quick win.
   - Phase 2: SourceType refactor per chosen option. (a) is no-op here; (b) and (c) do the actual restructuring.
4. Sweep follow-ups (filed separately if not bundled): `cmd/bt/robot_history.go` JSONL pins (bundle into bt-08sh per the survey), `pkg/workspace/loader.go` Dolt awareness (bt-3ltq blocker).

## Blast radius and test impact

| Option | Production LOC delta | Test files touched | Consumer call sites changed |
|--------|----------------------|--------------------|-----------------------------|
| (a)    | ~500 deleted         | ~3                 | 0                           |
| (b)    | ~700 net (delete + restructure) | ~5–8           | 5–10                        |
| (c)    | ~1000+ net (interface + restructure)  | ~10+    | 15–20                       |

Estimates from the survey doc; refine before implementation lands.

## Out of scope

- The per-feature JSONL pin sweeps (`burndown.go`, `cobra_export.go`, `pages.go`, `profiling.go`, `robot_triage.go`, `model_editor.go`). These are tracked under bt-08sh / bt-mhcv / the implementation bead's follow-up section.
- Upstream beads architectural changes. ADR-003 is bt-only.
- Worktree JSONL semantics in (a) and (b) — if the worktree path is meaningfully different from the local path, that's its own design question.

## Consequences

If accepted (any option):
- ADR-002 sub-stream line gets updated to reference ADR-003 as the canonical home for SourceType refactor.
- The implementation bead becomes ready-to-claim.
- bt-3ltq (global git history) becomes implementable once the workspace loader is Dolt-aware.

If rejected / deferred indefinitely:
- SQLite reader stays as dead weight; bt-mhcv has to keep it on its watchlist.
- bt-3ltq stays blocked on workspace loader pinning.
- Future agents continue to puzzle out the SourceType abstraction's purpose.

## References

- `docs/audit/2026-04-25-data-source-architecture-survey.md` — the foundation
- ADR-002 sub-stream "post-Dolt migration of bt-derived data layer" (line 56)
- AGENTS.md "Beads architecture awareness" section (verified 2026-04-25)
- bt session 190df5ce (2026-04-25) — discovery context
