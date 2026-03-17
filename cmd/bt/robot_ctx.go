package main

import (
	"os"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// robotCtx holds loaded/computed state shared across robot command handlers.
// Flag values (robotTriage, diffSince, labelFilter, etc.) remain package-level
// pflag vars and are accessed directly by handlers.
type robotCtx struct {
	issues            []model.Issue
	issuesForSearch   []model.Issue        // pre-label-scope issues for search
	analyzer          *analysis.Analyzer   // lazily created if nil
	enc               robotEncoder         // output encoder (json or toon)
	cwd               string               // working directory
	beadsPath         string               // path to beads file (for file-based sources)
	repoName          string               // project/repo name
	dataHash          string               // stable hash of issue data
	labelScopeContext *analysis.LabelHealth // label health context when --label is used
	projectDir        string               // project root (for baselines)
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
