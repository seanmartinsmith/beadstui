// Package datasource provides intelligent multi-source data detection and selection
// for beadstui. It discovers, validates, and selects the freshest valid source
// from a Dolt server (when configured) or worktree/local JSONL files.
package datasource

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// ErrDoltRequired is returned when metadata declares backend=dolt but the
// Dolt server is not reachable. This prevents silent fallback to stale
// JSONL data.
var ErrDoltRequired = errors.New("Dolt server required but not reachable")

// SourceType identifies the type of data source
type SourceType string

const (
	// SourceTypeDolt is a running Dolt SQL server
	SourceTypeDolt SourceType = "dolt"
	// SourceTypeDoltGlobal is a shared Dolt server with multiple databases
	SourceTypeDoltGlobal SourceType = "dolt_global"
	// SourceTypeJSONLWorktree is a JSONL file from a git worktree
	SourceTypeJSONLWorktree SourceType = "jsonl_worktree"
	// SourceTypeJSONLLocal is a local JSONL file
	SourceTypeJSONLLocal SourceType = "jsonl_local"
)

// Priority values for source types (higher = more authoritative)
const (
	PriorityDolt          = 110
	PriorityJSONLWorktree = 80
	PriorityJSONLLocal    = 50
)

// DataSource represents a potential source of beads data
type DataSource struct {
	// Type identifies the source type
	Type SourceType `json:"type"`
	// Path is the absolute path to the source file (or DSN for Dolt sources)
	Path string `json:"path"`
	// Priority determines preference when timestamps are equal (higher = preferred)
	Priority int `json:"priority"`
	// ModTime is the last modification time of the source
	ModTime time.Time `json:"mod_time"`
	// Valid indicates whether the source passed validation
	Valid bool `json:"valid"`
	// ValidationError describes why validation failed (if Valid is false)
	ValidationError string `json:"validation_error,omitempty"`
	// IssueCount is the number of issues in the source (set during validation)
	IssueCount int `json:"issue_count"`
	// Size is the file size in bytes
	Size int64 `json:"size"`
	// RepoFilter narrows database enumeration in global mode (case-insensitive match)
	RepoFilter string `json:"repo_filter,omitempty"`
}

// String returns a human-readable description of the source
func (s DataSource) String() string {
	status := "valid"
	if !s.Valid {
		status = fmt.Sprintf("invalid: %s", s.ValidationError)
	}
	return fmt.Sprintf("%s (%s, priority=%d, mod=%s, issues=%d, %s)",
		s.Path, s.Type, s.Priority, s.ModTime.Format(time.RFC3339), s.IssueCount, status)
}

// DiscoveryOptions configures source discovery behavior
type DiscoveryOptions struct {
	// BeadsDir is the .beads directory path (optional, auto-detected if empty)
	BeadsDir string
	// RepoPath is the repository root path (optional, uses cwd if empty)
	RepoPath string
	// ValidateAfterDiscovery runs validation on each discovered source
	ValidateAfterDiscovery bool
	// IncludeInvalid includes sources that failed validation in results
	IncludeInvalid bool
	// Verbose enables detailed logging during discovery
	Verbose bool
	// Logger receives log messages when Verbose is true
	Logger func(msg string)
	// RequireDolt skips JSONL/worktree discovery entirely.
	// When true, only Dolt is attempted; if unreachable, ErrDoltRequired is returned.
	RequireDolt bool
}

// DiscoverSources finds all potential data sources in the beads directory
func DiscoverSources(opts DiscoveryOptions) ([]DataSource, error) {
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}

	// Determine beads directory
	beadsDir := opts.BeadsDir
	if beadsDir == "" {
		// Check BEADS_DIR environment variable
		if envDir := os.Getenv("BEADS_DIR"); envDir != "" {
			beadsDir = envDir
		} else {
			// Use repo path or current directory
			repoPath := opts.RepoPath
			if repoPath == "" {
				var err error
				repoPath, err = os.Getwd()
				if err != nil {
					return nil, fmt.Errorf("failed to get current directory: %w", err)
				}
			}
			beadsDir = filepath.Join(repoPath, ".beads")
		}
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Discovering sources in: %s", beadsDir))
	}

	var sources []DataSource

	// Discover Dolt server (highest priority - authoritative when available)
	doltSources, err := discoverDoltSources(beadsDir, opts)
	if err != nil && opts.Verbose {
		opts.Logger(fmt.Sprintf("Dolt discovery warning: %v", err))
	}
	sources = append(sources, doltSources...)

	// When metadata declares backend=dolt, don't fall through to stale backends
	if opts.RequireDolt {
		if len(doltSources) == 0 {
			return nil, ErrDoltRequired
		}
		// Skip JSONL/worktree entirely
	} else {
		// Discover local JSONL files
		localSources, err := discoverLocalJSONLSources(beadsDir, opts)
		if err != nil && opts.Verbose {
			opts.Logger(fmt.Sprintf("Local JSONL discovery warning: %v", err))
		}
		sources = append(sources, localSources...)

		// Discover worktree JSONL files
		worktreeSources, err := discoverWorktreeSources(opts.RepoPath, opts)
		if err != nil && opts.Verbose {
			opts.Logger(fmt.Sprintf("Worktree discovery warning: %v", err))
		}
		sources = append(sources, worktreeSources...)
	}

	// Validate sources if requested
	if opts.ValidateAfterDiscovery {
		for i := range sources {
			if err := ValidateSource(&sources[i]); err != nil && opts.Verbose {
				opts.Logger(fmt.Sprintf("Validation failed for %s: %v", sources[i].Path, err))
			}
		}
	}

	// Filter out invalid sources if not including them
	if opts.ValidateAfterDiscovery && !opts.IncludeInvalid {
		var validSources []DataSource
		for _, s := range sources {
			if s.Valid {
				validSources = append(validSources, s)
			}
		}
		sources = validSources
	}

	// Sort by priority and mod time
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].ModTime.Equal(sources[j].ModTime) {
			return sources[i].Priority > sources[j].Priority
		}
		return sources[i].ModTime.After(sources[j].ModTime)
	})

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Discovered %d sources", len(sources)))
	}

	return sources, nil
}

// discoverLocalJSONLSources finds JSONL files in the beads directory
func discoverLocalJSONLSources(beadsDir string, opts DiscoveryOptions) ([]DataSource, error) {
	var sources []DataSource

	entries, err := os.ReadDir(beadsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read beads directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()

		// Must be a .jsonl file
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Skip backups, merge artifacts, and deletion manifests
		if strings.Contains(name, ".backup") ||
			strings.Contains(name, ".orig") ||
			strings.Contains(name, ".merge") ||
			name == "deletions.jsonl" ||
			strings.HasPrefix(name, "beads.left") ||
			strings.HasPrefix(name, "beads.right") {
			continue
		}

		path := filepath.Join(beadsDir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}

		sources = append(sources, DataSource{
			Type:     SourceTypeJSONLLocal,
			Path:     path,
			Priority: PriorityJSONLLocal,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
		})

		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Found local JSONL: %s (mod=%s)", path, info.ModTime().Format(time.RFC3339)))
		}
	}

	return sources, nil
}

// discoverWorktreeSources finds JSONL files in git worktree beads directories
func discoverWorktreeSources(repoPath string, opts DiscoveryOptions) ([]DataSource, error) {
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Find git directory
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		// Not a git repository
		return nil, nil
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoPath, gitDir)
	}

	// Look for beads-worktrees directory
	worktreesDir := filepath.Join(gitDir, "beads-worktrees")
	if _, err := os.Stat(worktreesDir); err != nil {
		// No worktrees directory
		return nil, nil
	}

	var sources []DataSource

	// Enumerate worktree directories
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read worktrees directory: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		wtDir := filepath.Join(worktreesDir, e.Name())

		// Look for issues.jsonl in this worktree
		jsonlPath := filepath.Join(wtDir, "issues.jsonl")
		info, err := os.Stat(jsonlPath)
		if err != nil {
			continue
		}

		sources = append(sources, DataSource{
			Type:     SourceTypeJSONLWorktree,
			Path:     jsonlPath,
			Priority: PriorityJSONLWorktree,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
		})

		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Found worktree JSONL: %s (mod=%s)", jsonlPath, info.ModTime().Format(time.RFC3339)))
		}
	}

	return sources, nil
}

// discoverDoltSources checks if the beads project uses a Dolt backend and
// the server is reachable. Returns a DataSource with the DSN in the Path field.
func discoverDoltSources(beadsDir string, opts DiscoveryOptions) ([]DataSource, error) {
	cfg, ok := ReadDoltConfig(beadsDir)
	if !ok {
		return nil, nil
	}

	dsn := cfg.DSN()

	// Ping the server to see if it's actually running
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Dolt: cannot open connection: %v", err))
		}
		return nil, nil
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Dolt: server not reachable at %s:%d: %v", cfg.Host, cfg.Port, err))
		}
		return nil, nil
	}

	// Get the most recent update time to use as ModTime
	var modTime time.Time
	var updatedAt sql.NullTime
	if err := db.QueryRow("SELECT MAX(updated_at) FROM issues").Scan(&updatedAt); err == nil && updatedAt.Valid {
		modTime = updatedAt.Time
	} else {
		modTime = time.Now()
	}

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Found Dolt server: %s:%d db=%s (mod=%s)", cfg.Host, cfg.Port, cfg.Database, modTime.Format(time.RFC3339)))
	}

	return []DataSource{{
		Type:     SourceTypeDolt,
		Path:     dsn,
		Priority: PriorityDolt,
		ModTime:  modTime,
	}}, nil
}
