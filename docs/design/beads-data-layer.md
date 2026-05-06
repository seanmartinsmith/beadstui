# Beads data layer (Dolt-only era)

Reference for any work in `bt` that touches the data layer, correlations,
sprints, session columns, or git-history-derived features. Verified
2026-04-25; updated 2026-05-06.

Some `bt` code and bt beads predate the beads v0.56.1 / v1.0.1 migration
and assume the older JSONL-backed layout. **Verify against the current
architecture below before scoping or implementing.** A systematic audit
of all open bt beads against this reality is tracked in **bt-mhcv (P0)**.

## Upstream URL migration (April 2026)

The canonical beads repo moved from `github.com/steveyegge/beads` to
`github.com/gastownhall/beads` (community stewardship). The old URL
still redirects, but new release work, issue trackers, and PRs live at
the gastownhall path. When citing upstream, link to `gastownhall/beads`.
Historical references in `docs/archive/` retain the original URL by
design (period-accurate). Recorded as a project decision; see `bd
decision list`.

## Current beads architecture

- **Storage**: Dolt is the only backend. JSONL export is opt-in for
  portability, not the system of record. The Dolt server data lives in
  `.beads/dolt/`.
- **Session columns**: `created_by_session`, `claimed_by_session`,
  `closed_by_session` are first-class columns on the `issues` table
  (upstream `0033_add_session_columns.up.sql`; Phase 1a merged
  2026-04-24 via bd-34v). **NOT** sourced from the `metadata` JSON blob
  - that pattern is now stale code (bt-5hl9 tracks the bt-side
  migration).
- **Events**: Beads has a native `events` table
  (`Storage.GetEvents` at `internal/storage/storage.go:76`) with
  columns `id, issue_id, event_type, actor, old_value, new_value,
  comment, created_at`. This is the upstream primitive for bead-event
  audit trails.
- **History**: `bd history <id>` queries `dolt_history_issues` for
  per-commit issue snapshots (with full session columns per snapshot).
  Note: bd-3gb tracks an empty-result `--json` bug being PR'd upstream.
- **Sprints**: NOT a beads concept upstream - no `sprints` table or
  subcommand. Any sprint-related code in bt is a bt-only feature
  (tracked in bt-z5jj - rebuild against Dolt or retire).
- **Correlations**: NOT a beads concept upstream. Purely bt's domain
  (tracked in bt-08sh - migrate from JSONL+git-diff witness to
  `dolt_log` + `dolt_history_issues`).
- **Data dirs**: `.beads/` is shared with bd's Dolt server + bd
  metadata. `.bt/` is bt-only cache (baseline, semantic search index).
  The split is partly accidental and being canonicalized in bt-uahv.

## Stale-assumption checklist

When scoping or auditing any bt bead, ask:

- [ ] Does it assume `.beads/<project>.jsonl` exists? (Dolt-only
      installs don't produce one.)
- [ ] Does it assume `.beads/sprints.jsonl` exists? (Beads doesn't
      produce one - sprints aren't upstream.)
- [ ] Does it read session columns from the `metadata` blob? (Should
      read direct columns; bt-5hl9 tracks the migration.)
- [ ] Does it expect `--global` to fail for any single-ID lookup?
      (bt-vhn2 was misframed this way - actual root cause was the
      correlator, not routing.)
- [ ] Does its acceptance criteria reference pre-Dolt invariants?
      (Likely needs rescoping.)

If suspect, leave a comment with the recon finding rather than diving
in. Cross-reference bt-mhcv for the systematic audit.

## Related beads

- **bt-mhcv** (P0) - systematic audit of all open bt beads
- **bt-08sh** (P2) - correlator Dolt migration
- **bt-z5jj** (P3) - sprint feature decision
- **bt-uahv** (P3) - `.beads/` vs `.bt/` canonical split
- **bt-5hl9** (P2) - CompactIssue session column migration
- **bd-3gb** (in beads repo) - bd history `--json` empty-result bug
