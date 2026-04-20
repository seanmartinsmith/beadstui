package main

import (
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/view"
)

// robotCtx holds loaded/computed state shared across robot command handlers.
// Flag values (robotTriage, diffSince, labelFilter, etc.) remain package-level
// pflag vars and are accessed directly by handlers.
type robotCtx struct {
	issues            []model.Issue
	issuesForSearch   []model.Issue         // pre-label-scope issues for search
	analyzer          *analysis.Analyzer    // lazily created if nil
	enc               robotEncoder          // output encoder (json or toon)
	cwd               string                // working directory
	beadsPath         string                // path to beads file (for file-based sources)
	repoName          string                // project/repo name
	dataHash          string                // stable hash of issue data
	labelScopeContext *analysis.LabelHealth // label health context when --label is used
	projectDir        string                // project root (for baselines)
}

// newRobotCtx constructs a robotCtx from loaded/computed state.
func newRobotCtx(issues, issuesForSearch []model.Issue, dataHash, cwd, beadsPath, projectDir string, labelScopeContext *analysis.LabelHealth) *robotCtx {
	return &robotCtx{
		issues:            issues,
		issuesForSearch:   issuesForSearch,
		enc:               newRobotEncoder(os.Stdout),
		cwd:               cwd,
		beadsPath:         beadsPath,
		repoName:          "",
		dataHash:          dataHash,
		labelScopeContext: labelScopeContext,
		projectDir:        projectDir,
	}
}

// newEncoder creates a fresh robot encoder writing to stdout.
func (rc *robotCtx) newEncoder() robotEncoder {
	return newRobotEncoder(os.Stdout)
}

// projectIssues returns the issue slice in the currently selected output
// shape. Under --shape=compact (the default) it returns []view.CompactIssue
// computed in a single O(n) pass over the dependency graph. Under
// --shape=full it returns the input slice untouched so the wire bytes stay
// byte-identical to pre-compact output.
//
// The return type is `any` on purpose: anonymous robot output structs tag
// `issues` once with the JSON key and the shape flips freely between modes.
func (rc *robotCtx) projectIssues(issues []model.Issue) any {
	if robotOutputShape == robotShapeCompact {
		return view.CompactAll(issues)
	}
	return issues
}

// compactSchema returns the schema identifier for the current shape, or ""
// when shape=full. The returned value is meant for RobotEnvelope.Schema,
// which is omitempty — full-mode envelopes stay byte-identical to history.
func (rc *robotCtx) compactSchema() string {
	if robotOutputShape == robotShapeCompact {
		return view.CompactIssueSchemaV1
	}
	return ""
}

// analysisIssues returns the issue slice to feed the analysis engine. In
// global mode, external:<project>:<id> deps are resolved against the global
// set before returning so cross-project blockers become real graph edges. In
// single-project mode returns rc.issues unchanged so wire output stays
// byte-identical to pre-resolution history.
//
// Composition rule: this is the single point that returns the graph-ready
// slice. Future preprocessing (label normalization, ID aliasing, etc.)
// composes INSIDE this function — it wraps the existing chain, it does not
// add a sibling rc.Xissues() helper. One pipeline, not N helpers.
func (rc *robotCtx) analysisIssues() []model.Issue {
	if !flagGlobal {
		return rc.issues
	}
	return analysis.ResolveExternalDeps(rc.issues)
}
