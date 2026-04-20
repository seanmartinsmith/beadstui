package main

import (
	"fmt"
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// pairsOutput is the wire payload for `bt robot pairs`. Pure function: no
// os.Exit, no stdout writes, no flag reads. Exposed at package level so
// contract tests can exercise the projection directly — binary-level tests
// can't drive --global because BT_TEST_MODE=1 disables shared Dolt server
// discovery (a deliberate guard to keep JSONL-fixture tests from touching
// real databases).
//
// Empty result surfaces as `"pairs": []` (never `null`): ComputePairRecords
// returns nil when no pairs exist, but the wire contract is an array.
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

// runPairs emits one view.PairRecord per cross-project paired set — groups
// of issues sharing an ID suffix (e.g. "zsy8" from bt-zsy8 + bd-zsy8) across
// distinct ID prefixes. Canonical member is the first-created bead; mirrors
// are the rest. Drift flags surface divergence of mirrors from canonical.
//
// Scope: strictly a --global subcommand. Without --global the issue set is
// single-project and pair detection is definitionally empty; we error
// cleanly rather than silently emit `[]` (which would collide with the
// legitimate "no pairs exist" signal).
//
// The --shape flag is inherited but effectively a no-op: PairRecord is
// compact-by-construction (no body fields to strip). The envelope's `schema`
// is set unconditionally to pair.v1 because the payload IS a versioned
// projection.
func (rc *robotCtx) runPairs() {
	if !flagGlobal {
		fmt.Fprintln(os.Stderr, "Error: bt robot pairs requires --global (pair detection needs cross-project data)")
		os.Exit(1)
	}

	output := pairsOutput(rc.analysisIssues(), rc.dataHash)

	enc := rc.newEncoder()
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding pairs: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
