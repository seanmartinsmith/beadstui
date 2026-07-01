package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/correlation"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
)

// resolveCorrelator constructs a *correlation.Correlator for the robot
// history-family subcommands, mirroring the dispatch LoadHistoryCmd uses in
// the TUI (pkg/ui/model.go, bt-ydjw phase 2). On JSONL-tracked repos it is
// equivalent to the legacy NewCorrelator(repoPath, beadsPath) construction.
// On Dolt-only repos (bt itself, post commit 90d8432d) it opens an ephemeral
// connection through the active DataSource and wires the correlator via
// NewCorrelatorWithDolt so GenerateReport dispatches to DoltExtractor.
//
// Dispatch order:
//
//  1. JSONL on disk      -> NewCorrelator (unchanged path).
//  2. SourceTypeDolt     -> open via datasource.NewDoltReader, borrow
//     reader.DB(), construct via NewCorrelatorWithDolt.
//  3. SourceTypeDoltGlobal + currentProjectDB -> sql.Open against
//     datasource.PerDBDSN(globalDSN, currentProjectDB), Ping to verify
//     reachability, construct via NewCorrelatorWithDolt.
//
// The returned closer is nil when no Dolt connection was opened; callers
// always defer it before invoking GenerateReport.
//
// err is returned only when none of the dispatch paths is available
// (no JSONL on disk AND no usable Dolt DataSource). Replaces the misleading
// "no beads file found in <repo>/.beads/" wedge that bt-5s3u closes.
func resolveCorrelator(repoPath string) (*correlation.Correlator, func(), error) {
	// Embedded (in-process Dolt) projects have neither git-tracked JSONL nor a
	// queryable Dolt SQL connection, so correlation cannot run. Surface a clear
	// "not available yet" error (bt-5uaxh) instead of the misleading generic
	// "no Dolt source" wedge below (bt-ij71a).
	if appCtx.selectedSource != nil && appCtx.selectedSource.Type == datasource.SourceTypeEmbeddedDolt {
		return nil, nil, correlation.ErrEmbeddedModeUnavailable
	}

	// The .git existence check that ValidateRepository used to perform is
	// still meaningful — the correlator wants co-commit context. The
	// beads-file half of that check is what the sweep drops.
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("not a git repository: %s", repoPath)
	}

	// Best-effort JSONL path discovery. On Dolt-only repos FindJSONLPath
	// returns an error; the empty beadsPath is fine because DoltExtractor
	// does not consult it.
	var beadsPath string
	if beadsDir, err := loader.GetBeadsDir(""); err == nil {
		if p, perr := loader.FindJSONLPath(beadsDir); perr == nil {
			beadsPath = p
		}
	}

	if correlation.HasJSONLOnDisk(repoPath) {
		return correlation.NewCorrelator(repoPath, beadsPath), nil, nil
	}

	if appCtx.selectedSource != nil {
		var (
			doltDB *sql.DB
			closer func()
		)
		switch appCtx.selectedSource.Type {
		case datasource.SourceTypeDolt:
			if reader, rerr := datasource.NewDoltReader(*appCtx.selectedSource); rerr == nil {
				doltDB = reader.DB()
				closer = func() { _ = reader.Close() }
			}
		case datasource.SourceTypeDoltGlobal:
			if appCtx.currentProjectDB != "" {
				dsn := datasource.PerDBDSN(appCtx.selectedSource.Path, appCtx.currentProjectDB)
				if db, oerr := sql.Open("mysql", dsn); oerr == nil {
					if perr := db.Ping(); perr == nil {
						doltDB = db
						closer = func() { _ = db.Close() }
					} else {
						_ = db.Close()
					}
				}
			}
		}
		if doltDB != nil {
			return correlation.NewCorrelatorWithDolt(repoPath, doltDB, beadsPath), closer, nil
		}
	}

	return nil, nil, fmt.Errorf("no beads data: %s has no .beads/*.jsonl on disk and no Dolt source is available", repoPath)
}
