package main

import (
	"fmt"
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// robotFlagOrphaned is bound by cobra to --orphaned on `bt robot pairs`.
// Under --schema=v1 it switches runPairs to the backfill-helper output
// (JSONL checklist + stderr summary); under --schema=v2 it errors.
var robotFlagOrphaned bool

// pairsValidate resolves and cross-checks the pairs-subcommand flags.
// Returns the effective schema or a user-facing error (the message is
// printed verbatim on stderr after the "Error: " prefix). Pure: no
// I/O, no os.Exit, safe to call under BT_TEST_MODE=1.
func pairsValidate(flagSchema string, flagOrphaned bool) (schema string, err error) {
	schema, err = resolveSchemaVersion(flagSchema)
	if err != nil {
		return "", err
	}
	if flagOrphaned && schema != robotSchemaV1 {
		return "", fmt.Errorf("--orphaned requires --schema=v1 (lists pairs missing the dep edge v2 requires). Run with --schema=v1")
	}
	return schema, nil
}

// pairsOutput is the wire payload for `bt robot pairs --schema=v1`.
// Pure function: no os.Exit, no stdout writes, no flag reads. Exposed
// at package level so contract tests can exercise the projection
// directly — binary-level tests can't drive --global because
// BT_TEST_MODE=1 disables shared Dolt server discovery.
//
// Empty result surfaces as `"pairs": []` (never `null`):
// ComputePairRecords returns nil when no pairs exist, but the wire
// contract is an array.
func pairsOutput(issues []model.Issue, dataHash string) any {
	pairs := view.ComputePairRecords(issues)
	if pairs == nil {
		pairs = []view.PairRecord{}
	}
	envelope := NewRobotEnvelope(dataHash)
	envelope.Schema = view.PairRecordSchemaV1
	return struct {
		RobotEnvelope
		Pairs []view.PairRecord `json:"pairs"`
	}{
		RobotEnvelope: envelope,
		Pairs:         pairs,
	}
}

// OrphanedPair is one row of the --orphaned JSONL checklist: a
// v1-detected pair whose members lack the cross-prefix dep edge that
// v2 requires as the intent signal. Agents consume this to drive
// manual (human-approved) `bd dep add` writes during the Phase 1
// backfill.
type OrphanedPair struct {
	Suffix           string   `json:"suffix"`
	Members          []string `json:"members"`
	SuggestedCommand string   `json:"suggested_command"`
}

// orphanedPairs computes the --orphaned payload: v1-detected pair
// groups whose members have no dep edge (any type, any direction)
// connecting at least two distinct prefixes inside the group. Pure
// function so tests can exercise it without touching stdout or cobra.
func orphanedPairs(issues []model.Issue) []OrphanedPair {
	pairs := view.ComputePairRecords(issues)
	if len(pairs) == 0 {
		return nil
	}

	// Index issues by ID so we can look up dependencies per member.
	byID := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		byID[issues[i].ID] = &issues[i]
	}

	out := make([]OrphanedPair, 0)
	for _, p := range pairs {
		members := allMemberIDs(p)
		if hasCrossPrefixDepEdge(members, byID) {
			continue
		}
		out = append(out, OrphanedPair{
			Suffix:           p.Suffix,
			Members:          members,
			SuggestedCommand: suggestDepAdd(members),
		})
	}
	return out
}

func allMemberIDs(p view.PairRecord) []string {
	ids := make([]string, 0, 1+len(p.Mirrors))
	ids = append(ids, p.Canonical.ID)
	for _, m := range p.Mirrors {
		ids = append(ids, m.ID)
	}
	return ids
}

// hasCrossPrefixDepEdge reports true iff any member of the pair has a
// dep (in either direction) to another member with a different prefix.
// Dep type is irrelevant — any edge demonstrates declared intent.
func hasCrossPrefixDepEdge(members []string, byID map[string]*model.Issue) bool {
	memberSet := make(map[string]string, len(members)) // id -> prefix
	for _, id := range members {
		prefix, _, ok := analysis.SplitID(id)
		if !ok {
			continue
		}
		memberSet[id] = prefix
	}

	for id, prefix := range memberSet {
		issue, ok := byID[id]
		if !ok || issue == nil {
			continue
		}
		for _, dep := range issue.Dependencies {
			if dep == nil {
				continue
			}
			otherPrefix, haveOther := memberSet[dep.DependsOnID]
			if haveOther && otherPrefix != prefix {
				return true
			}
		}
	}

	// Check incoming edges too: another member may point at this one.
	for id, prefix := range memberSet {
		for otherID, otherPrefix := range memberSet {
			if otherID == id || otherPrefix == prefix {
				continue
			}
			otherIssue, ok := byID[otherID]
			if !ok || otherIssue == nil {
				continue
			}
			for _, dep := range otherIssue.Dependencies {
				if dep != nil && dep.DependsOnID == id {
					return true
				}
			}
		}
	}
	return false
}

func suggestDepAdd(members []string) string {
	if len(members) < 2 {
		return ""
	}
	return fmt.Sprintf("bd dep add %s %s --type=related", members[0], members[1])
}

// runPairs emits one view.PairRecord per cross-project paired set —
// groups of issues sharing an ID suffix (e.g. "zsy8" from bt-zsy8 +
// bd-zsy8) across distinct ID prefixes. Canonical member is the
// first-created bead; mirrors are the rest. Drift flags surface
// divergence of mirrors from canonical.
//
// Scope: strictly a --global subcommand. Without --global the issue
// set is single-project and pair detection is definitionally empty;
// we error cleanly rather than silently emit `[]`.
//
// --schema dispatch: v1 (default in Phase 1) routes to pairsOutput.
// v2 errors with a "Phase 2 scope" notice until ComputePairRecordsV2
// ships. --orphaned is v1-only; under v2 it errors with a clear
// message.
func (rc *robotCtx) runPairs(schema string) {
	if !flagGlobal {
		fmt.Fprintln(os.Stderr, "Error: bt robot pairs requires --global (pair detection needs cross-project data)")
		os.Exit(1)
	}

	switch schema {
	case robotSchemaV1:
		if robotFlagOrphaned {
			rc.emitOrphanedPairs()
			return
		}
		rc.emitPairsV1()
	case robotSchemaV2:
		fmt.Fprintln(os.Stderr, "Error: --schema=v2 not yet implemented (Phase 2 of bt-gkyn ships pair.v2 reader). Use --schema=v1.")
		os.Exit(1)
	}
}

func (rc *robotCtx) emitPairsV1() {
	output := pairsOutput(rc.analysisIssues(), rc.dataHash)
	enc := rc.newEncoder()
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding pairs: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// emitOrphanedPairs writes one JSONL record per orphaned pair to
// stdout and a human summary to stderr. Exits 0 even on empty list —
// no orphans is the healthy state post-backfill.
func (rc *robotCtx) emitOrphanedPairs() {
	orphans := orphanedPairs(rc.analysisIssues())

	enc := rc.newEncoder()
	for i := range orphans {
		if err := enc.Encode(orphans[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding orphaned pair: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr,
		"--orphaned: %d pair(s) detected by v1 are missing the cross-prefix dep edge v2 requires.\n"+
			"Review each record on stdout and run the suggested bd dep add command manually.\n"+
			"Cross-project writes require human authorization — this tool does not apply changes.\n",
		len(orphans),
	)
	os.Exit(0)
}
