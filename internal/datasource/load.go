package datasource

import (
	"errors"
	"fmt"

	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// LoadIssues resolves the canonical data source for the project and loads
// issues from it. Per ADR-003 there is no priority ranking: Dolt wins
// when configured, JSONL fallback otherwise.
//
// Returns ErrDoltRequired (without falling back) when metadata declares
// backend=dolt but the server is unreachable, so callers can take
// corrective action (e.g. start the server) instead of silently serving
// stale JSONL data.
func LoadIssues(repoPath string) ([]model.Issue, error) {
	beadsDir, err := loader.GetBeadsDir(repoPath)
	if err != nil {
		return nil, err
	}

	src, err := DiscoverSource(DiscoveryOptions{
		BeadsDir:               beadsDir,
		RepoPath:               repoPath,
		ValidateAfterDiscovery: true,
	})
	if err != nil {
		if errors.Is(err, ErrDoltRequired) {
			return nil, err
		}
		// Last-resort fallback to legacy JSONL-only loader for environments
		// the new discovery doesn't cover (e.g. exotic file layouts).
		issues, lerr := loader.LoadIssues(repoPath)
		if lerr != nil {
			return nil, lerr
		}
		return synthesizeHierarchyDeps(issues), nil
	}

	return LoadFromSource(src)
}

// LoadIssuesFromDir resolves and loads from a known beads directory.
func LoadIssuesFromDir(beadsDir string) ([]model.Issue, error) {
	src, err := DiscoverSource(DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: true,
	})
	if err != nil {
		if errors.Is(err, ErrDoltRequired) {
			return nil, err
		}
		jsonlPath, ferr := loader.FindJSONLPath(beadsDir)
		if ferr != nil {
			return nil, fmt.Errorf("no source in %s: %w", beadsDir, err)
		}
		issues, lerr := loader.LoadIssuesFromFile(jsonlPath)
		if lerr != nil {
			return nil, lerr
		}
		return synthesizeHierarchyDeps(issues), nil
	}
	return LoadFromSource(src)
}

// LoadResult pairs loaded issues with the source they came from.
type LoadResult struct {
	Issues []model.Issue
	Source DataSource
}

// LoadIssuesWithSource is like LoadIssues but also returns the resolved
// DataSource so callers can route refreshes/reloads through the correct
// backend.
func LoadIssuesWithSource(repoPath string) (LoadResult, error) {
	beadsDir, err := loader.GetBeadsDir(repoPath)
	if err != nil {
		return LoadResult{}, err
	}

	src, err := DiscoverSource(DiscoveryOptions{
		BeadsDir:               beadsDir,
		RepoPath:               repoPath,
		ValidateAfterDiscovery: true,
	})
	if err != nil {
		if errors.Is(err, ErrDoltRequired) {
			return LoadResult{}, err
		}
		// Last-resort legacy loader fallback.
		issues, lerr := loader.LoadIssues(repoPath)
		if lerr != nil {
			return LoadResult{}, lerr
		}
		jsonlPath, _ := loader.FindJSONLPath(beadsDir)
		return LoadResult{
			Issues: synthesizeHierarchyDeps(issues),
			Source: DataSource{Type: SourceTypeJSONLFallback, Path: jsonlPath},
		}, nil
	}

	issues, err := LoadFromSource(src)
	if err != nil {
		return LoadResult{}, err
	}
	return LoadResult{Issues: issues, Source: src}, nil
}

// LoadFromSource loads issues from a specific DataSource, dispatching to
// the appropriate reader based on source type. Synthesizes implicit
// parent_child Dependencies from `.N`-suffix IDs (bt-cuyiz) so consumers
// see the same hierarchy bd computes from ID structure.
func LoadFromSource(source DataSource) ([]model.Issue, error) {
	var (
		issues []model.Issue
		err    error
	)
	switch source.Type {
	case SourceTypeDolt:
		reader, derr := NewDoltReader(source)
		if derr != nil {
			return nil, fmt.Errorf("failed to open Dolt source %s: %w", source.Path, derr)
		}
		defer reader.Close()
		issues, err = reader.LoadIssues()

	case SourceTypeDoltGlobal:
		reader, derr := NewGlobalDoltReader(source)
		if derr != nil {
			return nil, derr
		}
		defer reader.Close()
		issues, err = reader.LoadIssues()

	case SourceTypeJSONLFallback:
		issues, err = loader.LoadIssuesFromFile(source.Path)

	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
	if err != nil {
		return nil, err
	}
	return synthesizeHierarchyDeps(issues), nil
}
