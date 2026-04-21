package main

import (
	"fmt"
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
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

// refsOutputV2 is the wire payload for `bt robot refs --schema=v2`. Same
// shape rules as refsOutput: empty result surfaces as `"refs": []`. The
// envelope's `schema` field carries ref.v2, and `sigil_mode` carries the
// resolved mode (strict|verb|permissive) so consumers can pin per-mode.
func refsOutputV2(issues []model.Issue, dataHash, sigilMode string) any {
	mode, ok := sigilModeFromString(sigilMode)
	if !ok {
		mode = analysis.SigilModeVerb
	}
	refs := view.ComputeRefRecordsV2(issues, mode)
	if refs == nil {
		refs = []view.RefRecordV2{}
	}
	envelope := NewRobotEnvelope(dataHash)
	envelope.Schema = view.RefRecordSchemaV2
	envelope.SigilMode = sigilMode
	return struct {
		RobotEnvelope
		Refs []view.RefRecordV2 `json:"refs"`
	}{
		RobotEnvelope: envelope,
		Refs:          refs,
	}
}

// sigilModeFromString maps the resolved flag string onto the
// analysis.SigilMode enum. Invalid strings (should never reach here —
// refsValidate filters first) fall back to verb, with ok=false so the
// caller can detect the anomaly in tests.
func sigilModeFromString(s string) (analysis.SigilMode, bool) {
	switch s {
	case robotSigilStrict:
		return analysis.SigilModeStrict, true
	case robotSigilVerb:
		return analysis.SigilModeVerb, true
	case robotSigilPermissive:
		return analysis.SigilModePermissive, true
	}
	return analysis.SigilModeVerb, false
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

// runRefs emits one view.RefRecord (v1) or RefRecordV2 (v2) per
// cross-project bead reference detected in deps, description, notes, and
// comments. v1 surfaces IDs by regex + known-prefix scope. v2 delegates
// prose scanning to analysis.DetectSigils under the resolved sigil mode
// (strict|verb|permissive) and retains v1's cross-project-only filter as a
// load-bearing FP guard.
//
// Scope: strictly a --global subcommand. Without --global the issue set is
// single-project and cross-project detection is definitionally empty; we
// error cleanly rather than silently emit `[]`.
//
// Flag validation runs in cobra's RunE (refsValidate) before robotPreRun,
// so this handler receives already-resolved schema and sigils values.
func (rc *robotCtx) runRefs(schema, sigils string) {
	if !flagGlobal {
		fmt.Fprintln(os.Stderr, "Error: bt robot refs requires --global (cross-project ref validation needs cross-project data)")
		os.Exit(1)
	}

	switch schema {
	case robotSchemaV1:
		rc.emitRefsV1()
	case robotSchemaV2:
		rc.emitRefsV2(sigils)
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

func (rc *robotCtx) emitRefsV2(sigils string) {
	output := refsOutputV2(rc.analysisIssues(), rc.dataHash, sigils)
	enc := rc.newEncoder()
	if err := enc.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding refs: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
