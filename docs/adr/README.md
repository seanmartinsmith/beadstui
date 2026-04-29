# Architecture Decision Records

This directory holds the project's permanent decision records. ADRs are never archived; superseded ones are marked **Superseded** with a pointer to whatever replaced them.

## Index

| ID | Title | Status | Spine? |
|----|-------|--------|--------|
| [001](001-btui-fork-takeover.md) | btui fork takeover | Accepted (closed) | — |
| [002](002-stabilize-and-ship.md) | Stabilize and ship | Active | **Yes** — current spine |
| [003](003-data-source-architecture-post-dolt.md) | Data source architecture post-Dolt | Accepted | — |

## Reading order for new sessions

1. **Start with ADR-002** — the active spine. It tracks open work streams, audit references, and which decisions are still open.
2. **Read ADR-003 next** — it explains how the data layer is shaped after the beads/Dolt migration. Touches anything that reads from the data source.
3. **Skim ADR-001 only as needed** — fork-takeover history. Useful when investigating "why does this look like a Jeffrey-era artifact?" but not for ongoing work.

## When to write a new ADR

- A decision is non-reversible without significant work
- The decision affects more than one work stream or session
- A future reader, six months from now, would otherwise have to reverse-engineer the rationale

If the decision is small or local, prefer a beads `decision` issue (`bd decision record ...`) over a new ADR.
