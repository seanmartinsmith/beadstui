package datasource

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrNoValidSources is returned when no valid sources are found
var ErrNoValidSources = errors.New("no valid data sources found")

// SelectionOptions configures source selection behavior
type SelectionOptions struct {
	// PreferFreshest prioritizes ModTime over Priority
	// Default: true
	PreferFreshest bool
	// MinimumValidSources requires at least N valid sources to proceed
	// Default: 1
	MinimumValidSources int
	// MaxAgeDelta ignores sources older than this duration compared to the newest
	// Default: 0 (no limit)
	MaxAgeDelta time.Duration
	// Verbose enables detailed logging during selection
	Verbose bool
	// Logger receives log messages when Verbose is true
	Logger func(msg string)
}

// DefaultSelectionOptions returns sensible default selection options
func DefaultSelectionOptions() SelectionOptions {
	return SelectionOptions{
		PreferFreshest:      true,
		MinimumValidSources: 1,
		MaxAgeDelta:         0,
		Verbose:             false,
		Logger:              func(string) {},
	}
}

// SelectionResult contains the selected source and metadata about the selection
type SelectionResult struct {
	// Selected is the chosen data source
	Selected DataSource
	// Candidates is the list of all valid sources considered
	Candidates []DataSource
	// Reason explains why this source was selected
	Reason string
	// SelectionTime is when the selection was made
	SelectionTime time.Time
}

// SelectBestSource chooses the best data source from the given list
func SelectBestSource(sources []DataSource) (DataSource, error) {
	return SelectBestSourceWithOptions(sources, DefaultSelectionOptions())
}

// SelectBestSourceWithOptions chooses the best data source with custom options
func SelectBestSourceWithOptions(sources []DataSource, opts SelectionOptions) (DataSource, error) {
	result, err := SelectBestSourceDetailed(sources, opts)
	if err != nil {
		return DataSource{}, err
	}
	return result.Selected, nil
}

// SelectBestSourceDetailed chooses the best data source with full details
func SelectBestSourceDetailed(sources []DataSource, opts SelectionOptions) (*SelectionResult, error) {
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}

	// Filter to valid sources only
	var valid []DataSource
	for _, s := range sources {
		if s.Valid {
			valid = append(valid, s)
		}
	}

	if len(valid) == 0 {
		return nil, ErrNoValidSources
	}

	if len(valid) < opts.MinimumValidSources {
		return nil, fmt.Errorf("only %d valid sources, need %d", len(valid), opts.MinimumValidSources)
	}

	// Sort by preference
	if opts.PreferFreshest {
		// Sort by: ModTime desc, then Priority desc
		sort.Slice(valid, func(i, j int) bool {
			if valid[i].ModTime.Equal(valid[j].ModTime) {
				return valid[i].Priority > valid[j].Priority
			}
			return valid[i].ModTime.After(valid[j].ModTime)
		})
	} else {
		// Sort by: Priority desc, then ModTime desc
		sort.Slice(valid, func(i, j int) bool {
			if valid[i].Priority == valid[j].Priority {
				return valid[i].ModTime.After(valid[j].ModTime)
			}
			return valid[i].Priority > valid[j].Priority
		})
	}

	// Apply age delta filter if specified
	if opts.MaxAgeDelta > 0 && len(valid) > 0 {
		newestTime := valid[0].ModTime
		cutoff := newestTime.Add(-opts.MaxAgeDelta)
		var filtered []DataSource
		for _, s := range valid {
			if s.ModTime.After(cutoff) || s.ModTime.Equal(cutoff) {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) > 0 {
			valid = filtered
		}
	}

	selected := valid[0]

	// Build reason string
	reason := buildSelectionReason(selected, valid, opts)

	if opts.Verbose {
		opts.Logger(fmt.Sprintf("Selected: %s (%s)", selected.Path, reason))
	}

	return &SelectionResult{
		Selected:      selected,
		Candidates:    valid,
		Reason:        reason,
		SelectionTime: time.Now(),
	}, nil
}

// buildSelectionReason creates a human-readable explanation for the selection
func buildSelectionReason(selected DataSource, candidates []DataSource, opts SelectionOptions) string {
	if len(candidates) == 1 {
		return "only valid source available"
	}

	reasons := []string{}

	// Check if newest
	isNewest := true
	for _, c := range candidates {
		if c.ModTime.After(selected.ModTime) {
			isNewest = false
			break
		}
	}
	if isNewest {
		reasons = append(reasons, "freshest modification time")
	}

	// Check if highest priority
	isHighestPriority := true
	for _, c := range candidates {
		if c.Priority > selected.Priority {
			isHighestPriority = false
			break
		}
	}
	if isHighestPriority {
		reasons = append(reasons, fmt.Sprintf("highest priority (%d)", selected.Priority))
	}

	// Check source type
	switch selected.Type {
	case SourceTypeSQLite:
		reasons = append(reasons, "SQLite is most authoritative")
	case SourceTypeJSONLWorktree:
		reasons = append(reasons, "synced worktree data")
	case SourceTypeJSONLLocal:
		reasons = append(reasons, "local JSONL file")
	}

	if len(reasons) == 0 {
		return "best available source"
	}

	return fmt.Sprintf("%s", reasons[0])
}

// SelectWithFallback tries sources in order until one succeeds validation and loading
func SelectWithFallback(sources []DataSource, loadFunc func(DataSource) error, opts SelectionOptions) (*DataSource, error) {
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}

	// Sort sources by preference first
	sorted := make([]DataSource, len(sources))
	copy(sorted, sources)

	if opts.PreferFreshest {
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].ModTime.Equal(sorted[j].ModTime) {
				return sorted[i].Priority > sorted[j].Priority
			}
			return sorted[i].ModTime.After(sorted[j].ModTime)
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Priority == sorted[j].Priority {
				return sorted[i].ModTime.After(sorted[j].ModTime)
			}
			return sorted[i].Priority > sorted[j].Priority
		})
	}

	// Try each source in order
	var lastErr error
	for i := range sorted {
		source := &sorted[i]

		// Skip if already known invalid
		if !source.Valid && source.ValidationError != "" {
			if opts.Verbose {
				opts.Logger(fmt.Sprintf("Skipping invalid source: %s (%s)", source.Path, source.ValidationError))
			}
			continue
		}

		// Validate if not already validated
		if !source.Valid {
			if err := ValidateSource(source); err != nil {
				if opts.Verbose {
					opts.Logger(fmt.Sprintf("Validation failed for %s: %v", source.Path, err))
				}
				lastErr = err
				continue
			}
		}

		// Try loading
		if err := loadFunc(*source); err != nil {
			if opts.Verbose {
				opts.Logger(fmt.Sprintf("Load failed for %s: %v", source.Path, err))
			}
			lastErr = err
			continue
		}

		if opts.Verbose {
			opts.Logger(fmt.Sprintf("Successfully loaded from: %s", source.Path))
		}
		return source, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all sources failed, last error: %w", lastErr)
	}
	return nil, ErrNoValidSources
}
