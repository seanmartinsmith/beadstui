# Architecture Decision Records

This directory holds the project's permanent decision records. ADRs are never archived; superseded ones are marked **Superseded** with a pointer to whatever replaced them.

## Index

| ID | Title | Status | Spine? |
|----|-------|--------|--------|
| [001](001-btui-fork-takeover.md) | btui fork takeover | Accepted (closed) | — |
| [002](002-stabilize-and-ship.md) | Stabilize and ship | Superseded (retired as spine 2026-07-14) | — |
| [003](003-data-source-architecture-post-dolt.md) | Data source architecture post-Dolt | Accepted | — |
| [004](004-bubbles-key-adoption.md) | Bubbles key adoption | Accepted | — |

There is **no active spine ADR** any more. Live planning lives in **beads (`bd list` / `bd ready`) + `docs/plans/`**; ADR-002 was retired as the spine on 2026-07-14 (its open work streams are all bead-tracked). The `docs/adr/` directory now holds standalone decision records only.

## Reading order for new sessions

1. **Start with `bd ready` / `bd list` and `docs/plans/`** — this is where active work and its rationale live.
2. **Read ADR-003** — how the data layer is shaped after the beads/Dolt migration. Touches anything that reads from the data source. **ADR-004** covers the Bubbles key-binding adoption.
3. **Skim ADR-002 / ADR-001 only as history** — ADR-002 is the retired stabilize-and-ship spine (frozen stream snapshot); ADR-001 is fork-takeover history, useful when investigating "why does this look like a Jeffrey-era artifact?"

## When to write a new ADR

- A decision is non-reversible without significant work
- The decision affects more than one work stream or session
- A future reader, six months from now, would otherwise have to reverse-engineer the rationale

If the decision is small or local, prefer a beads `decision` issue (`bd decision record ...`) over a new ADR.
