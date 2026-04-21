package main

import (
	"fmt"
	"os"
	"strings"
)

// Sigil mode constants used by --sigils and BT_SIGIL_MODE on
// `bt robot refs`. Only meaningful under --schema=v2; under v1 the
// flag errors at resolution time with a clear message.
const (
	robotSigilStrict     = "strict"
	robotSigilVerb       = "verb"
	robotSigilPermissive = "permissive"
)

// robotSigilDefault is the default --sigils value when nothing is set.
// `verb` is the brainstorm-chosen default — strict under-counts and
// permissive keeps v1's FP profile. Dogfooding drives any future flip
// via a one-line change here.
const robotSigilDefault = robotSigilVerb

// Flag-bound value populated by cobra. Resolved against the env var
// and default in resolveSigilsMode.
var robotFlagSigils string

// resolveSigilsMode computes the effective sigils mode from the
// --sigils flag, BT_SIGIL_MODE env, and the default, in that order.
// Returns an error for unknown values (flag or env).
//
// Phase 1 note: the resolved mode is only consumed under --schema=v2
// (Phase 3 scope). resolveSigilsMode is decoupled from schema
// resolution so the runRefs handler can detect conflicting pairings
// (--schema=v1 with --sigils=*) with a specific error.
func resolveSigilsMode(cliSigils string) (string, error) {
	sigils := strings.ToLower(strings.TrimSpace(cliSigils))
	if sigils == "" {
		sigils = strings.ToLower(strings.TrimSpace(os.Getenv("BT_SIGIL_MODE")))
	}
	if sigils == "" {
		sigils = robotSigilDefault
	}

	switch sigils {
	case robotSigilStrict, robotSigilVerb, robotSigilPermissive:
		return sigils, nil
	default:
		return "", fmt.Errorf("invalid --sigils %q (expected strict|verb|permissive)", sigils)
	}
}

// sigilsFlagExplicit returns true when the user explicitly opted into
// a sigil mode — either via the --sigils flag or BT_SIGIL_MODE env var.
// Used by runRefs to detect the --schema=v1 + --sigils=* conflict
// without false-positiving on the default mode.
func sigilsFlagExplicit(cliSigils string) bool {
	if strings.TrimSpace(cliSigils) != "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("BT_SIGIL_MODE")) != "" {
		return true
	}
	return false
}
