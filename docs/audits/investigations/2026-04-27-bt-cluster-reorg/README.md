# 2026-04-27 bt cluster reorg audit

Multi-file cluster-investigation snapshot from 2026-04-27. Driven by the cluster-reorg planning effort (`docs/plans/2026-04-27-bt-cluster-reorg-proposal.md`), which examined bt's relationship with `bd` and the writable-TUI productization arc.

## Files

| File | What |
|---|---|
| `bd-surface-map.md` | Exhaustive enumeration of `bd` v1.0.3 commands, flags, output shapes, and TUI-fit assessments. Built from `bd <cmd> --help` and upstream source. |
| `bt-cluster-map.md` | Snapshot of bt's epic / P1 / P2 cluster state with bt↔bd parity analysis. |
| `dolt-migration-bead-audit.md` | bt-mhcv audit: 169 open bt beads checked against current beads/Dolt architecture. 163/6/0 GREEN/YELLOW/RED. |
| `tui-productization-gap.md` | Gap analysis between bt's current TUI and what writable-TUI productization needs. |
| `writable-tui-design-surface.md` | Exhaustive design analysis for bt's read-only → read/write transition. |

## Status

Reference snapshot. Several of these informed implementation work that's now in flight or shipped (e.g., bt-mhcv audit findings drove the Dolt-migration awareness section in AGENTS.md). Don't edit the files in place; if a follow-up audit is needed, run a new one with today's date.

## Related

- Plan: `docs/plans/2026-04-27-bt-cluster-reorg-proposal.md`
- Companion plans: `docs/plans/2026-04-27-bangout-arc.md`, `docs/plans/2026-04-27-phase-1-dispatch.md`
