package view

import (
	"sort"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// PairRecordSchemaV1 identifies the wire shape produced by ComputePairRecords.
// Emitted on the robot output envelope's `schema` field. Like PortfolioRecord
// the payload is compact-by-construction, so the schema is set unconditionally
// regardless of --shape.
const PairRecordSchemaV1 = "pair.v1"

// PairRecord describes one cross-project paired set — a group of issues
// sharing an ID suffix (e.g. "zsy8" from bt-zsy8 + bd-zsy8) across distinct
// ID prefixes. The canonical member is the first-created bead; the rest are
// mirrors. Drift flags surface divergence of the mirrors from canonical.
//
// See docs/design/2026-04-20-bt-mhwy-2-pairs.md for the authority on
// identity rules, canonical selection, and drift dimensions.
type PairRecord struct {
	Suffix    string       `json:"suffix"`
	Canonical PairMember   `json:"canonical"`
	Mirrors   []PairMember `json:"mirrors"`
	Drift     []string     `json:"drift,omitempty"`
}

// PairMember is the compact projection of a single bead inside a PairRecord.
// Deliberately narrower than CompactIssue — pair output lists are typically
// small and agents drill into a full bead via `bd show <id>` when needed.
type PairMember struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	Status     model.Status `json:"status"`
	Priority   int          `json:"priority"`
	SourceRepo string       `json:"source_repo,omitempty"`
}

// Drift flag constants. Order here is also the output order inside
// PairRecord.Drift so agents can diff two runs byte-for-byte.
const (
	driftStatus     = "status"
	driftPriority   = "priority"
	driftClosedOpen = "closed_open"
	driftTitle      = "title"
)

// ComputePairRecords groups issues by ID suffix, filters to groups with ≥2
// distinct ID prefixes, and emits one record per group. Records are sorted by
// suffix. Nil/empty inputs return nil.
//
// Pair identity is exact suffix match across distinct ID prefixes. Canonical
// is first-created (CreatedAt ascending; ties broken by prefix, then full ID).
// Drift flags compare each mirror against canonical and appear in fixed order:
// status, priority, closed_open, title.
func ComputePairRecords(issues []model.Issue) []PairRecord {
	if len(issues) == 0 {
		return nil
	}

	buckets := make(map[string][]model.Issue)
	for i := range issues {
		_, suffix, ok := analysis.SplitID(issues[i].ID)
		if !ok {
			continue
		}
		buckets[suffix] = append(buckets[suffix], issues[i])
	}

	records := make([]PairRecord, 0, len(buckets))
	for suffix, members := range buckets {
		if len(members) < 2 {
			continue
		}
		if !hasMultiplePrefixes(members) {
			continue
		}
		rec, ok := buildPairRecord(suffix, members)
		if !ok {
			continue
		}
		records = append(records, rec)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Suffix < records[j].Suffix
	})
	if len(records) == 0 {
		return nil
	}
	return records
}

// hasMultiplePrefixes reports whether the bucket contains at least two
// distinct ID prefixes. Single-prefix buckets (data anomaly where the same ID
// appears across different source_repos) are dropped rather than silently
// collapsed.
func hasMultiplePrefixes(members []model.Issue) bool {
	if len(members) < 2 {
		return false
	}
	first, _, ok := analysis.SplitID(members[0].ID)
	if !ok {
		return false
	}
	for i := 1; i < len(members); i++ {
		prefix, _, ok := analysis.SplitID(members[i].ID)
		if !ok {
			continue
		}
		if prefix != first {
			return true
		}
	}
	return false
}

// buildPairRecord sorts the bucket by (CreatedAt asc, prefix asc, full ID asc),
// takes the head as canonical, and builds the record. Returns (_, false) when
// the bucket can't resolve a canonical (every ID failed SplitID — should never
// happen because hasMultiplePrefixes has already vetted).
func buildPairRecord(suffix string, members []model.Issue) (PairRecord, bool) {
	sorted := make([]model.Issue, len(members))
	copy(sorted, members)
	sort.SliceStable(sorted, func(i, j int) bool {
		ci, cj := sorted[i].CreatedAt, sorted[j].CreatedAt
		if !ci.Equal(cj) {
			return ci.Before(cj)
		}
		pi, _, _ := analysis.SplitID(sorted[i].ID)
		pj, _, _ := analysis.SplitID(sorted[j].ID)
		if pi != pj {
			return pi < pj
		}
		return sorted[i].ID < sorted[j].ID
	})

	canonical := toPairMember(sorted[0])
	mirrors := make([]PairMember, 0, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		mirrors = append(mirrors, toPairMember(sorted[i]))
	}

	return PairRecord{
		Suffix:    suffix,
		Canonical: canonical,
		Mirrors:   mirrors,
		Drift:     computeDrift(sorted[0], sorted[1:]),
	}, true
}

// toPairMember projects a model.Issue into the compact PairMember slot.
func toPairMember(src model.Issue) PairMember {
	return PairMember{
		ID:         src.ID,
		Title:      src.Title,
		Status:     src.Status,
		Priority:   src.Priority,
		SourceRepo: src.SourceRepo,
	}
}

// computeDrift returns the drift flags for a paired set. Canonical is the
// source of truth; mirrors are compared against it. Flags appear in the fixed
// order: status, priority, closed_open, title. Returns nil (not empty slice)
// when no drift is detected so the json tag `drift,omitempty` suppresses the
// field for in-sync pairs.
func computeDrift(canonical model.Issue, mirrors []model.Issue) []string {
	if len(mirrors) == 0 {
		return nil
	}

	var statusDrift, priorityDrift, closedOpenDrift, titleDrift bool
	canonicalClosed := canonical.Status == model.StatusClosed
	for _, m := range mirrors {
		if m.Status != canonical.Status {
			statusDrift = true
		}
		if m.Priority != canonical.Priority {
			priorityDrift = true
		}
		mirrorClosed := m.Status == model.StatusClosed
		if mirrorClosed != canonicalClosed {
			closedOpenDrift = true
		}
		if m.Title != canonical.Title {
			titleDrift = true
		}
	}

	var flags []string
	if statusDrift {
		flags = append(flags, driftStatus)
	}
	if priorityDrift {
		flags = append(flags, driftPriority)
	}
	if closedOpenDrift {
		flags = append(flags, driftClosedOpen)
	}
	if titleDrift {
		flags = append(flags, driftTitle)
	}
	return flags
}
