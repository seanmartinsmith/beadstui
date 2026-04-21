package main

import (
	"fmt"
	"os"
	"strings"
)

// Schema version constants used by --schema and BT_OUTPUT_SCHEMA on
// `bt robot pairs` and `bt robot refs`. The v1 shapes are frozen wire
// contracts in pkg/view/schemas/{pair,ref}_record.v1.json; v2 adds
// intent_source (pairs) and sigil_kind (refs) plus envelope fields.
const (
	robotSchemaV1 = "v1"
	robotSchemaV2 = "v2"
)

// robotSchemaDefault is the default --schema value when nothing is set.
//
// Flipped to v2 in Phase 3 of bt-vxu9 once both v2 readers
// (ComputePairRecordsV2 from Phase 2 + ComputeRefRecordsV2 from Phase 3)
// exist. `--schema=v1` remains available as an opt-in fallback for one
// release while downstream consumers migrate; removal is tracked in a
// follow-up bead.
const robotSchemaDefault = robotSchemaV2

// Flag-bound values populated by cobra. Resolved against the env var
// and default in resolveSchemaVersion.
var robotFlagSchema string

// resolveSchemaVersion computes the effective schema version from the
// --schema flag, BT_OUTPUT_SCHEMA env, and the default, in that order.
// Returns an error for unknown values (flag or env).
func resolveSchemaVersion(cliSchema string) (string, error) {
	schema := strings.ToLower(strings.TrimSpace(cliSchema))
	if schema == "" {
		schema = strings.ToLower(strings.TrimSpace(os.Getenv("BT_OUTPUT_SCHEMA")))
	}
	if schema == "" {
		schema = robotSchemaDefault
	}

	switch schema {
	case robotSchemaV1, robotSchemaV2:
		return schema, nil
	default:
		return "", fmt.Errorf("invalid --schema %q (expected v1|v2)", schema)
	}
}
