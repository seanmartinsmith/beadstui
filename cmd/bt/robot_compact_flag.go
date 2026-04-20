package main

import (
	"fmt"
	"os"
	"strings"
)

// Shape constants used by --shape and BT_OUTPUT_SHAPE. The default is
// compact: agent workflows are scan -> drill -> act, and compact projections
// cut the default `bt robot list` payload by ~80% on 100 issues.
const (
	robotShapeCompact = "compact"
	robotShapeFull    = "full"
)

// Package-level state resolved by resolveRobotOutputShape at robotCmd's
// PersistentPreRunE. Any subcommand that projects []model.Issue consults
// robotOutputShape directly via robotCtx.projectIssues.
var (
	robotOutputShape = robotShapeCompact

	// Flag-bound values. Populated by cobra; resolved against aliases and
	// env vars in resolveRobotOutputShape.
	robotFlagShape   string
	robotFlagCompact bool
	robotFlagFull    bool
)

// resolveRobotOutputShape computes the effective shape from CLI flags,
// `--compact`/`--full` aliases, the BT_OUTPUT_SHAPE env var, and the
// compact default, in that order. Returns an error for conflicting aliases
// or unknown values.
func resolveRobotOutputShape(cliShape string, compactAlias, fullAlias bool) (string, error) {
	if compactAlias && fullAlias {
		return "", fmt.Errorf("--compact and --full are mutually exclusive")
	}

	shape := strings.ToLower(strings.TrimSpace(cliShape))
	if shape == "" {
		switch {
		case compactAlias:
			shape = robotShapeCompact
		case fullAlias:
			shape = robotShapeFull
		}
	} else if compactAlias || fullAlias {
		// Don't let the alias contradict the explicit --shape value.
		aliasShape := robotShapeCompact
		if fullAlias {
			aliasShape = robotShapeFull
		}
		if shape != aliasShape {
			return "", fmt.Errorf("--shape=%s conflicts with --%s", shape, aliasShape)
		}
	}
	if shape == "" {
		shape = strings.ToLower(strings.TrimSpace(os.Getenv("BT_OUTPUT_SHAPE")))
	}
	if shape == "" {
		shape = robotShapeCompact
	}

	switch shape {
	case robotShapeCompact, robotShapeFull:
		return shape, nil
	default:
		return "", fmt.Errorf("invalid --shape %q (expected compact|full)", shape)
	}
}
