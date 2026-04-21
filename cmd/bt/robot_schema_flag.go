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
// Phase 1 of bt-gkyn/bt-vxu9 ships the flag surface but NOT the v2
// readers (Phase 2 lands ComputePairRecordsV2; Phase 3 lands v2 ref).
// Until those ship, the default stays v1 so `bt robot pairs --global`
// and `bt robot refs --global` keep emitting pair.v1 / ref.v1. Phase 2
// flips this to robotSchemaV2 in a one-line change.
const robotSchemaDefault = robotSchemaV1

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
