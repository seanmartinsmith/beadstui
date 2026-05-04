// Package datasource resolves the data source for a beadstui project.
//
// Architecture (ADR-003): there are exactly three source types: a
// per-project Dolt server (the upstream system of record since beads
// v1.0.1), a shared global Dolt server enumerated across multiple
// project databases, and a JSONL fallback for legacy projects that
// haven't migrated to Dolt yet. Discovery is a simple decision: try
// Dolt-resolution first; if Dolt is configured but unreachable, return
// ErrDoltRequired; otherwise fall back to JSONL if a file is present;
// else error.
//
// The pre-ADR-003 multi-source discovery + priority + selection pipeline
// was collapsed because it had outlived its purpose: with Dolt as the
// only live backend and JSONL as opt-in legacy export, there is nothing
// to compare or rank.
package datasource

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/seanmartinsmith/beadstui/pkg/loader"
)

// ErrDoltRequired is returned when metadata declares backend=dolt but the
// Dolt server is not reachable. This prevents silent fallback to stale
// JSONL data.
var ErrDoltRequired = errors.New("Dolt server required but not reachable")

// ErrNoSource is returned when neither Dolt nor a JSONL fallback file
// can be resolved in the beads directory.
var ErrNoSource = errors.New("no data source found")

// SourceType identifies the type of data source. Per ADR-003 there are
// exactly three values; do not add more without revisiting that decision.
type SourceType string

const (
	// SourceTypeDolt is a per-project Dolt SQL server.
	SourceTypeDolt SourceType = "dolt"
	// SourceTypeDoltGlobal is a shared Dolt server hosting multiple databases.
	SourceTypeDoltGlobal SourceType = "dolt_global"
	// SourceTypeJSONLFallback is a legacy JSONL file used when Dolt is not
	// configured for the project.
	SourceTypeJSONLFallback SourceType = "jsonl_fallback"
)

// DataSource represents the resolved source of beads data for a project.
type DataSource struct {
	// Type identifies the source type.
	Type SourceType `json:"type"`
	// Path is the absolute path to the source file (or DSN for Dolt sources).
	Path string `json:"path"`
	// ModTime is the last modification time of the source.
	ModTime time.Time `json:"mod_time"`
	// Valid indicates whether the source passed validation.
	Valid bool `json:"valid"`
	// ValidationError describes why validation failed (if Valid is false).
	ValidationError string `json:"validation_error,omitempty"`
	// IssueCount is the number of issues in the source (set during validation).
	IssueCount int `json:"issue_count"`
	// Size is the file size in bytes (JSONL only).
	Size int64 `json:"size"`
	// RepoFilter narrows database enumeration in global mode (case-insensitive match).
	RepoFilter string `json:"repo_filter,omitempty"`
}

// String returns a human-readable description of the source.
func (s DataSource) String() string {
	status := "valid"
	if !s.Valid {
		status = fmt.Sprintf("invalid: %s", s.ValidationError)
	}
	return fmt.Sprintf("%s (%s, mod=%s, issues=%d, %s)",
		s.Path, s.Type, s.ModTime.Format(time.RFC3339), s.IssueCount, status)
}

// DiscoveryOptions configures source discovery behavior.
type DiscoveryOptions struct {
	// BeadsDir is the .beads directory path (optional, auto-detected if empty).
	BeadsDir string
	// RepoPath is the repository root path (optional, uses cwd if empty).
	RepoPath string
	// ValidateAfterDiscovery runs validation on the discovered source.
	ValidateAfterDiscovery bool
	// Verbose enables detailed logging during discovery.
	Verbose bool
	// Logger receives log messages when Verbose is true.
	Logger func(msg string)
}

// DiscoverSource resolves the single canonical data source for the project,
// applying the ADR-003 decision: Dolt if configured, else JSONL fallback.
//
// Returns ErrDoltRequired if metadata declares backend=dolt but the server
// is unreachable. Returns ErrNoSource if no source can be resolved at all.
func DiscoverSource(opts DiscoveryOptions) (DataSource, error) {
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}

	beadsDir, err := resolveBeadsDir(opts)
	if err != nil {
		return DataSource{}, err
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Discovering source in: %s", beadsDir))
	}

	// Dolt configured? Try it. If declared but unreachable, fail loudly so
	// we don't silently serve stale JSONL data.
	if cfg, ok := ReadDoltConfig(beadsDir); ok {
		src, ok := tryDoltSource(cfg, opts)
		if ok {
			if opts.ValidateAfterDiscovery {
				_ = ValidateSource(&src)
			}
			return src, nil
		}
		return DataSource{}, ErrDoltRequired
	}

	// No Dolt configured: fall back to legacy JSONL.
	src, ok := tryJSONLFallback(beadsDir, opts)
	if !ok {
		return DataSource{}, ErrNoSource
	}
	if opts.ValidateAfterDiscovery {
		_ = ValidateSource(&src)
	}
	return src, nil
}

// resolveBeadsDir applies the precedence: explicit option > BEADS_DIR env
// var > <repoPath>/.beads.
func resolveBeadsDir(opts DiscoveryOptions) (string, error) {
	if opts.BeadsDir != "" {
		return opts.BeadsDir, nil
	}
	if envDir := os.Getenv("BEADS_DIR"); envDir != "" {
		return envDir, nil
	}
	repoPath := opts.RepoPath
	if repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		repoPath = cwd
	}
	return filepath.Join(repoPath, ".beads"), nil
}

// tryDoltSource pings the configured Dolt server. Returns false if the
// server is not reachable.
func tryDoltSource(cfg DoltConfig, opts DiscoveryOptions) (DataSource, bool) {
	dsn := cfg.DSN()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Dolt: cannot open connection: %v", err))
		}
		return DataSource{}, false
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Dolt: server not reachable at %s:%d: %v", cfg.Host, cfg.Port, err))
		}
		return DataSource{}, false
	}

	// Use the most recent issue update as ModTime when available; fall back
	// to time.Now so callers always get a meaningful timestamp.
	var modTime time.Time
	var updatedAt sql.NullTime
	if err := db.QueryRow("SELECT MAX(updated_at) FROM issues").Scan(&updatedAt); err == nil && updatedAt.Valid {
		modTime = updatedAt.Time
	} else {
		modTime = time.Now()
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Found Dolt server: %s:%d db=%s (mod=%s)",
			cfg.Host, cfg.Port, cfg.Database, modTime.Format(time.RFC3339)))
	}

	return DataSource{
		Type:    SourceTypeDolt,
		Path:    dsn,
		ModTime: modTime,
	}, true
}

// tryJSONLFallback locates a usable legacy JSONL file. Honors metadata.json's
// declared jsonl_export path when present, otherwise uses loader.FindJSONLPath
// to apply the canonical filename-priority search.
func tryJSONLFallback(beadsDir string, opts DiscoveryOptions) (DataSource, bool) {
	// Honor metadata.json's declared jsonl_export path first.
	if path, ok := readJSONLExport(beadsDir); ok {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if opts.Verbose {
				opts.Logger(fmt.Sprintf("Found JSONL fallback (from metadata.json): %s", path))
			}
			return DataSource{
				Type:    SourceTypeJSONLFallback,
				Path:    path,
				ModTime: info.ModTime(),
				Size:    info.Size(),
			}, true
		}
	}

	// Otherwise fall back to canonical filename discovery.
	jsonlPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		if opts.Verbose {
			opts.Logger(fmt.Sprintf("JSONL discovery: %v", err))
		}
		return DataSource{}, false
	}

	info, err := os.Stat(jsonlPath)
	if err != nil {
		return DataSource{}, false
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Found JSONL fallback: %s (mod=%s)",
			jsonlPath, info.ModTime().Format(time.RFC3339)))
	}

	return DataSource{
		Type:    SourceTypeJSONLFallback,
		Path:    jsonlPath,
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}, true
}
