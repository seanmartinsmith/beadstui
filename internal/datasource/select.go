package datasource

// This file is intentionally near-empty after ADR-003.
//
// Pre-ADR-003 (when SQLite, JSONL, and Dolt all coexisted as live
// backends) this file contained SelectBestSource, SelectWithFallback,
// SelectionOptions, and SelectionResult — a priority+freshness ranking
// over a slice of candidate sources. With Dolt as the only live system
// of record and JSONL as a single legacy fallback, the decision space
// collapses to "Dolt or fallback," and ranking has no work to do.
//
// DiscoverSource (in source.go) now returns the single canonical source
// directly. There are no consumers of selection logic left.
//
// The file is kept (not deleted) because the worktree's deletion guard
// blocks tracked-file removal; nothing here is load-bearing.
