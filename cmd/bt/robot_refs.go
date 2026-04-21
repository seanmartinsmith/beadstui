package main

import (
	"fmt"
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// refsOutput is the wire payload for `bt robot refs --schema=v1`.
// Pure function: no os.Exit, no stdout writes, no flag reads. Exposed
// at package level so contract tests can exercise the projection
// directly — binary-level tests can't drive --global because
// BT_TEST_MODE=1 disables shared Dolt server discovery.
//
// Empty result surfaces as `"refs": []` (never `null`): ComputeRefRecords
// returns nil when no cross-project refs exist, but the wire contract
// is an array.
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

// refsValidate resolves and cross-checks the refs-subcommand flags.
// Returns the effective schema + sigils mode, or a user-facing error
// (printed verbatim on stderr after the "Error: " prefix). Pure: no
// I/O, no os.Exit, safe to call under BT_TEST_MODE=1.
func refsValidate(flagSchema, flagSigils string) (schema, sigils string, err error) {
	schema, err = resolveSchemaVersion(flagSchema)
	if err != nil {
		return "", "", err
	}
	sigils, err = resolveSigilsMode(flagSigils)
	if err != nil {
		return "", "", err
	}
	if schema == robotSchemaV1 && sigilsFlagExplicit(flagSigils) {
		return "", "", fmt.Errorf("--sigils requires --schema=v2. Run with --schema=v2, or omit --sigils to use v1 defaults")
	}
	return schema, sigils, nil
}

// runRefs emits one view.RefRecord per cross-project bead reference
// detected in deps (unresolved external: form only), description,
// notes, and comments.
//
// Scope: strictly a --global subcommand. Without --global the issue
// set is single-project and cross-project detection is definitionally
// empty; we error cleanly rather than silently emit `[]`.
//
// --schema dispatch: v1 (default in Phase 1) routes to refsOutput.
// --schema=v2 errors with a "Phase 3 scope" notice until the v2
// reader + sigil detector ship. Flag validation runs in cobra's
// RunE before robotPreRun, so this handler receives an already-
// resolved schema value.
func (rc *robotCtx) runRefs(schema string) {
	if !flagGlobal {
		fmt.Fprintln(os.Stderr, "Error: bt robot refs requires --global (cross-project ref validation needs cross-project data)")
		os.Exit(1)
	}

	switch schema {
	case robotSchemaV1:
		rc.emitRefsV1()
	case robotSchemaV2:
		fmt.Fprintln(os.Stderr, "Error: --schema=v2 not yet implemented (Phase 3 of bt-vxu9 ships ref.v2 reader + sigil detector). Use --schema=v1.")
		os.Exit(1)
	}
}

func (rc *robotCtx) emitRefsV1() {
	output := refsOutput(rc.analysisIssues(), rc.dataHash)
	enc := rc.newEncoder()
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding refs: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
