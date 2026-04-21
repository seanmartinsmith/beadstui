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

// PairRecordSchemaV2 identifies the wire shape produced by ComputePairRecordsV2.
// v2 narrows identity to intent-based detection: a pair surfaces only when
// members share an ID suffix across distinct prefixes AND at least one
// cross-prefix dep edge connects them. BFS over those edges collapses
// bidirectional deps, cycles, and partially connected groups into one record
// per connected component.
//
// Wire differences from v1: adds `intent_source` (currently always "dep"),
// drops `title` from the drift-flag enum (no-signal on real corpus per
// bt-gkyn dogfooding).
const PairRecordSchemaV2 = "pair.v2"

// PairIntentSourceDep is the only intent channel implemented in v2 — an
// explicit cross-prefix dep edge between pair members. Future channels
// (upstream `paired_with` column, hook-stamped markers) land as new values.
const PairIntentSourceDep = "dep"

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

// PairRecordV2 is the intent-based pair projection. Mirrors PairRecord's
// compact layout but swaps the identity semantics: members arrive here only
// after BFS over cross-prefix dep edges has linked them. `title` drift is
// absent by design (bt-gkyn dogfooding showed it fired on 24/29 v1 pairs,
// dominated by suffix collisions rather than real drift). `IntentSource`
// names the channel that established intent — always "dep" in v2.
type PairRecordV2 struct {
	Suffix       string       `json:"suffix"`
	Canonical    PairMember   `json:"canonical"`
	Mirrors      []PairMember `json:"mirrors"`
	Drift        []string     `json:"drift,omitempty"`
	IntentSource string       `json:"intent_source"`
}

// ComputePairRecordsV2 groups issues by ID suffix, builds an undirected graph
// per bucket from cross-prefix dep edges (any dep type, either direction),
// walks connected components, and emits one record per component with ≥2
// members spanning ≥2 distinct prefixes. Nil/empty inputs return nil.
//
// Compared to v1:
//   - v1: suffix match alone surfaces a pair (noise-prone; ~5× FPR on real corpus).
//   - v2: suffix match + cross-prefix dep edge = intent. Components without
//     an edge drop out. The --orphaned helper on `bt robot pairs --schema=v1`
//     surfaces exactly the set v2 would drop, for manual backfill triage.
//
// Canonical selection reuses v1 semantics (CreatedAt ascending; tie-break by
// prefix ascending, then full ID). Dangling deps (target not in the bucket
// or not a valid cross-prefix edge) are silently skipped rather than counted.
// Records are sorted by (suffix, canonical ID) so multiple components inside
// one suffix bucket emit in a stable order.
func ComputePairRecordsV2(issues []model.Issue) []PairRecordV2 {
	if len(issues) == 0 {
		return nil
	}

	// Index issues by ID once; bucket by suffix.
	idToIdx := make(map[string]int, len(issues))
	buckets := make(map[string][]int)
	for i := range issues {
		_, suffix, ok := analysis.SplitID(issues[i].ID)
		if !ok {
			continue
		}
		idToIdx[issues[i].ID] = i
		buckets[suffix] = append(buckets[suffix], i)
	}

	records := make([]PairRecordV2, 0)

	for suffix, bucket := range buckets {
		if len(bucket) < 2 {
			continue
		}

		// Set for fast in-bucket membership tests.
		inBucket := make(map[int]bool, len(bucket))
		for _, idx := range bucket {
			inBucket[idx] = true
		}

		// Undirected adjacency; an edge exists iff two bucket members have
		// different prefixes AND at least one dep edge connects them in
		// either direction. Dep type doesn't matter — any declared
		// relationship is evidence of intent.
		adj := make(map[int]map[int]bool, len(bucket))
		for _, idx := range bucket {
			adj[idx] = make(map[int]bool)
		}
		for _, idx := range bucket {
			srcPrefix, _, _ := analysis.SplitID(issues[idx].ID)
			for _, dep := range issues[idx].Dependencies {
				if dep == nil {
					continue
				}
				otherIdx, ok := idToIdx[dep.DependsOnID]
				if !ok || !inBucket[otherIdx] {
					continue
				}
				dstPrefix, _, _ := analysis.SplitID(issues[otherIdx].ID)
				if dstPrefix == srcPrefix {
					continue
				}
				adj[idx][otherIdx] = true
				adj[otherIdx][idx] = true
			}
		}

		components := bfsComponents(bucket, adj)
		for _, comp := range components {
			if len(comp) < 2 {
				continue
			}
			members := make([]model.Issue, 0, len(comp))
			for _, idx := range comp {
				members = append(members, issues[idx])
			}
			if !hasMultiplePrefixes(members) {
				continue
			}
			rec, ok := buildPairRecordV2(suffix, members)
			if !ok {
				continue
			}
			records = append(records, rec)
		}
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Suffix != records[j].Suffix {
			return records[i].Suffix < records[j].Suffix
		}
		return records[i].Canonical.ID < records[j].Canonical.ID
	})
	if len(records) == 0 {
		return nil
	}
	return records
}

// bfsComponents walks the undirected graph defined by adj restricted to the
// given node set and returns connected components. Nodes are enumerated in
// `nodes` order so the outer loop stays deterministic even across map
// iteration; within a component BFS order doesn't affect the membership set.
func bfsComponents(nodes []int, adj map[int]map[int]bool) [][]int {
	visited := make(map[int]bool, len(nodes))
	var components [][]int
	for _, start := range nodes {
		if visited[start] {
			continue
		}
		queue := []int{start}
		visited[start] = true
		var comp []int
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			comp = append(comp, n)
			// Sort neighbors for determinism: map iteration is random.
			neighbors := make([]int, 0, len(adj[n]))
			for nb := range adj[n] {
				neighbors = append(neighbors, nb)
			}
			sort.Ints(neighbors)
			for _, nb := range neighbors {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		components = append(components, comp)
	}
	return components
}

// buildPairRecordV2 mirrors buildPairRecord but emits the v2 shape. The sort
// ordering is identical so canonical selection behaves the same way when
// given the same member set. Returns (_, false) only if the bucket can't
// resolve a canonical (every ID failed SplitID — pre-vetted by the caller).
func buildPairRecordV2(suffix string, members []model.Issue) (PairRecordV2, bool) {
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

	return PairRecordV2{
		Suffix:       suffix,
		Canonical:    canonical,
		Mirrors:      mirrors,
		Drift:        computeDriftV2(sorted[0], sorted[1:]),
		IntentSource: PairIntentSourceDep,
	}, true
}

// computeDriftV2 is computeDrift minus the title check. Drift flags emit in
// fixed order: status, priority, closed_open. Title drift is dropped in v2
// because the dogfood corpus showed it no-signal — it fires on nearly every
// v1 pair (dominated by suffix collisions where members are unrelated work
// with unrelated titles, not pairs where one mirror has drifted).
func computeDriftV2(canonical model.Issue, mirrors []model.Issue) []string {
	if len(mirrors) == 0 {
		return nil
	}
	var statusDrift, priorityDrift, closedOpenDrift bool
	canonicalClosed := canonical.Status == model.StatusClosed
	for _, m := range mirrors {
		if m.Status != canonical.Status {
			statusDrift = true
		}
		if m.Priority != canonical.Priority {
			priorityDrift = true
		}
		if (m.Status == model.StatusClosed) != canonicalClosed {
			closedOpenDrift = true
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
	return flags
}
