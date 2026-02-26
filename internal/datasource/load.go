package datasource

import (
	"errors"
	"fmt"

	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// LoadIssues performs smart multi-source detection and loading.
// It discovers all available sources (SQLite, JSONL), validates them, selects
// the freshest valid source, and loads issues from it. SQLite is preferred over
// JSONL when both exist at comparable freshness, since SQLite reflects the most
// recent state (including status changes from br operations).
//
// Falls back to legacy JSONL-only loading via loader.LoadIssues if smart
// detection finds no valid sources.
func LoadIssues(repoPath string) ([]model.Issue, error) {
	beadsDir, err := loader.GetBeadsDir(repoPath)
	if err != nil {
		return nil, err
	}

	issues, smartErr := loadSmart(beadsDir, repoPath)
	if smartErr == nil {
		return issues, nil
	}

	// Don't fall through to JSONL when metadata says Dolt
	if errors.Is(smartErr, ErrDoltRequired) {
		return nil, smartErr
	}

	// Fall back to legacy JSONL-only loading
	return loader.LoadIssues(repoPath)
}

// LoadIssuesFromDir performs smart source detection within a known beads directory.
// This is useful when the caller already knows the .beads path.
func LoadIssuesFromDir(beadsDir string) ([]model.Issue, error) {
	issues, smartErr := loadSmart(beadsDir, "")
	if smartErr == nil {
		return issues, nil
	}

	// Fall back to JSONL
	jsonlPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		return nil, err
	}
	return loader.LoadIssuesFromFile(jsonlPath)
}

// isDoltRequired checks whether metadata.json declares backend=dolt.
func isDoltRequired(beadsDir string) bool {
	_, ok := ReadDoltConfig(beadsDir)
	return ok
}

// loadSmart discovers sources, validates, selects the best, and loads from it.
func loadSmart(beadsDir, repoPath string) ([]model.Issue, error) {
	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		RepoPath:               repoPath,
		ValidateAfterDiscovery: true,
		IncludeInvalid:         false,
		RequireDolt:            isDoltRequired(beadsDir),
	})
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no valid sources discovered")
	}

	best, err := SelectBestSource(sources)
	if err != nil {
		return nil, err
	}

	return LoadFromSource(best)
}

// LoadResult pairs loaded issues with the source they came from.
type LoadResult struct {
	Issues []model.Issue
	Source DataSource
}

// LoadIssuesWithSource performs smart multi-source detection and loading,
// returning both the issues and the selected DataSource. This allows callers
// to route refresh/reload through the correct backend.
func LoadIssuesWithSource(repoPath string) (LoadResult, error) {
	beadsDir, err := loader.GetBeadsDir(repoPath)
	if err != nil {
		return LoadResult{}, err
	}

	result, smartErr := loadSmartWithSource(beadsDir, repoPath)
	if smartErr == nil {
		return result, nil
	}

	// Don't fall through to JSONL when metadata says Dolt
	if errors.Is(smartErr, ErrDoltRequired) {
		return LoadResult{}, smartErr
	}

	// Fall back to legacy JSONL-only loading
	issues, err := loader.LoadIssues(repoPath)
	if err != nil {
		return LoadResult{}, err
	}

	// Construct a synthetic DataSource for the JSONL fallback
	jsonlPath, _ := loader.FindJSONLPath(beadsDir)
	return LoadResult{
		Issues: issues,
		Source: DataSource{Type: SourceTypeJSONLLocal, Path: jsonlPath},
	}, nil
}

// loadSmartWithSource is like loadSmart but also returns the selected DataSource.
func loadSmartWithSource(beadsDir, repoPath string) (LoadResult, error) {
	sources, err := DiscoverSources(DiscoveryOptions{
		BeadsDir:               beadsDir,
		RepoPath:               repoPath,
		ValidateAfterDiscovery: true,
		IncludeInvalid:         false,
		RequireDolt:            isDoltRequired(beadsDir),
	})
	if err != nil {
		return LoadResult{}, err
	}
	if len(sources) == 0 {
		return LoadResult{}, fmt.Errorf("no valid sources discovered")
	}

	best, err := SelectBestSource(sources)
	if err != nil {
		return LoadResult{}, err
	}

	issues, err := LoadFromSource(best)
	if err != nil {
		return LoadResult{}, err
	}
	return LoadResult{Issues: issues, Source: best}, nil
}

// LoadFromSource loads issues from a specific DataSource, dispatching to the
// appropriate reader based on source type.
func LoadFromSource(source DataSource) ([]model.Issue, error) {
	switch source.Type {
	case SourceTypeDolt:
		reader, err := NewDoltReader(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open Dolt source %s: %w", source.Path, err)
		}
		defer reader.Close()
		return reader.LoadIssues()

	case SourceTypeSQLite:
		reader, err := NewSQLiteReader(source)
		if err != nil {
			return nil, fmt.Errorf("failed to open SQLite source %s: %w", source.Path, err)
		}
		defer reader.Close()
		return reader.LoadIssues()

	case SourceTypeJSONLLocal, SourceTypeJSONLWorktree:
		return loader.LoadIssuesFromFile(source.Path)

	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
}
