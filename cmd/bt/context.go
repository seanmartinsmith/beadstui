package main

import (
	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/internal/doltctl"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"
)

// appContext holds loaded data shared across subcommands.
type appContext struct {
	issues           []model.Issue
	issuesForSearch  []model.Issue // pre-label-scope issues (for search)
	beadsPath        string
	selectedSource   *datasource.DataSource
	serverState      *doltctl.ServerState
	workspaceInfo    *workspace.LoadSummary
	currentProjectDB string

	// Resolved state from loading
	dataHash       string
	asOfResolved   string // resolved commit SHA when --as-of is used
	loadDuration   float64 // seconds

	// Common flags
	globalMode bool
	repoFilter string
	quiet      bool
	verbose    bool
}

// appCtx is the package-level shared context populated during PersistentPreRunE.
var appCtx appContext
