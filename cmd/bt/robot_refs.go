package main

import (
	"fmt"
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// refsOutput is the wire payload for `bt robot refs`. Pure function: no
// os.Exit, no stdout writes, no flag reads. Exposed at package level so
// contract tests can exercise the projection directly — binary-level tests
// can't drive --global because BT_TEST_MODE=1 disables shared Dolt server
// discovery (a deliberate guard to keep JSONL-fixture tests from touching
// real databases).
//
// Empty result surfaces as `"refs": []` (never `null`): ComputeRefRecords
// returns nil when no cross-project refs exist, but the wire contract is an
// array.
func refsOutput(issues []model.Issue, dataHash string) any {
	refs := view.ComputeRefRecords(issues)
	if refs == nil {
		refs = []view.RefRecord{}
	}
	envelope := NewRobotEnvelope(dataHash)
	envelope.Schema = view.RefRecordSchemaV1
	return struct {
		RobotEnvelope
		Refs []view.RefRecord `json:"refs"`
	}{
		RobotEnvelope: envelope,
		Refs:          refs,
	}
}

// runRefs emits one view.RefRecord per cross-project bead reference detected
// in deps (unresolved external: form only), description, notes, and
// comments. v1 surfaces cross-project refs exclusively; same-prefix
// references stay out of scope because the dep graph already owns them.
//
// Scope: strictly a --global subcommand. Without --global the issue set is
// single-project and cross-project detection is definitionally empty; we
// error cleanly rather than silently emit `[]`.
//
// The --shape flag is inherited but effectively a no-op: RefRecord is
// compact-by-construction. The envelope's `schema` is set unconditionally
// to ref.v1 because the payload IS a versioned projection.
func (rc *robotCtx) runRefs() {
	if !flagGlobal {
		fmt.Fprintln(os.Stderr, "Error: bt robot refs requires --global (cross-project ref validation needs cross-project data)")
		os.Exit(1)
	}

	output := refsOutput(rc.analysisIssues(), rc.dataHash)

	enc := rc.newEncoder()
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding refs: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
