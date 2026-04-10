// Package correlation provides the Correlator for building complete bead history reports.
package correlation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Correlator orchestrates the extraction and correlation of bead history data
type Correlator struct {
	repoPath    string
	extractor   *Extractor
	coCommitter *CoCommitExtractor
}

// NewCorrelator creates a new correlator for the given repository.
// beadsFilePath is optional and forwarded to the extractor so history follows
// the correct beads file; variadic form preserves compatibility with older
// single-argument callers.
func NewCorrelator(repoPath string, beadsFilePath ...string) *Correlator {
	return &Correlator{
		repoPath:    repoPath,
		extractor:   NewExtractor(repoPath, beadsFilePath...),
		coCommitter: NewCoCommitExtractor(repoPath),
	}
}

// CorrelatorOptions controls how the history report is generated
type CorrelatorOptions struct {
	BeadID string     // Filter to single bead ID (empty = all)
	Since  *time.Time // Only events after this time
	Until  *time.Time // Only events before this time
	Limit  int        // Max commits to process (0 = no limit)
}

// GenerateReport generates a complete history report
func (c *Correlator) GenerateReport(beads []BeadInfo, opts CorrelatorOptions) (*HistoryReport, error) {
	// Build extract options
	extractOpts := ExtractOptions{
		Since:  opts.Since,
		Until:  opts.Until,
		Limit:  opts.Limit,
		BeadID: opts.BeadID,
	}

	// Extract lifecycle events from git history
	events, err := c.extractor.Extract(extractOpts)
	if err != nil {
		return nil, fmt.Errorf("extracting events: %w", err)
	}

	// Extract co-committed files
	commits, err := c.coCommitter.ExtractAllCoCommits(events)
	if err != nil {
		return nil, fmt.Errorf("extracting co-commits: %w", err)
	}

	// Build bead histories
	histories := c.buildHistories(beads, events, commits)

	// Apply bead filter if specified
	if opts.BeadID != "" {
		filtered := make(map[string]BeadHistory)
		if h, ok := histories[opts.BeadID]; ok {
			filtered[opts.BeadID] = h
		}
		histories = filtered
	}

	// Build commit index
	commitIndex := c.buildCommitIndex(histories)

	// Calculate stats
	stats := c.calculateStats(histories, commits)

	// Build git range description
	gitRange := c.describeGitRange(opts)

	// Calculate data hash
	dataHash := c.calculateDataHash(beads)

	// Get latest commit SHA for incremental updates
	latestCommitSHA := c.findLatestCommitSHA(events, commits)

	return &HistoryReport{
		GeneratedAt:     time.Now().UTC(),
		DataHash:        dataHash,
		GitRange:        gitRange,
		LatestCommitSHA: latestCommitSHA,
		Stats:           stats,
		Histories:       histories,
		CommitIndex:     commitIndex,
	}, nil
}

// findLatestCommitSHA finds the most recent commit SHA from events and commits
func (c *Correlator) findLatestCommitSHA(events []BeadEvent, commits []CorrelatedCommit) string {
	var latest time.Time
	var latestSHA string

	// Check events
	for _, e := range events {
		if e.Timestamp.After(latest) {
			latest = e.Timestamp
			latestSHA = e.CommitSHA
		}
	}

	// Check commits
	for _, commit := range commits {
		if commit.Timestamp.After(latest) {
			latest = commit.Timestamp
			latestSHA = commit.SHA
		}
	}

	return latestSHA
}

// BeadInfo is minimal bead information needed for correlation
type BeadInfo struct {
	ID     string
	Title  string
	Status string
}

// buildHistories constructs BeadHistory for each bead
func (c *Correlator) buildHistories(beads []BeadInfo, events []BeadEvent, commits []CorrelatedCommit) map[string]BeadHistory {
	histories := make(map[string]BeadHistory)

	// Initialize histories from bead list
	for _, bead := range beads {
		histories[bead.ID] = BeadHistory{
			BeadID:  bead.ID,
			Title:   bead.Title,
			Status:  bead.Status,
			Events:  []BeadEvent{},
			Commits: []CorrelatedCommit{},
		}
	}

	// Group events by bead ID
	eventsByBead := make(map[string][]BeadEvent)
	for _, event := range events {
		eventsByBead[event.BeadID] = append(eventsByBead[event.BeadID], event)
	}

	// Group commits by bead ID
	commitsByBead := make(map[string][]CorrelatedCommit)
	for _, commit := range commits {
		if commit.BeadID != "" {
			commitsByBead[commit.BeadID] = append(commitsByBead[commit.BeadID], commit)
		}
	}

	// Build complete histories
	for beadID, history := range histories {
		history.Events = eventsByBead[beadID]
		history.Commits = dedupCommits(commitsByBead[beadID])

		// Calculate milestones
		history.Milestones = GetBeadMilestones(history.Events)

		// Calculate cycle time
		history.CycleTime = CalculateCycleTime(history.Milestones)

		// Set last author
		if len(history.Commits) > 0 {
			history.LastAuthor = history.Commits[len(history.Commits)-1].Author
		} else if len(history.Events) > 0 {
			history.LastAuthor = history.Events[len(history.Events)-1].Author
		}

		histories[beadID] = history
	}

	return histories
}

// dedupCommits removes duplicate commits by SHA
func dedupCommits(commits []CorrelatedCommit) []CorrelatedCommit {
	seen := make(map[string]bool)
	var result []CorrelatedCommit
	for _, c := range commits {
		if !seen[c.SHA] {
			seen[c.SHA] = true
			result = append(result, c)
		}
	}
	return result
}

// buildCommitIndex creates a reverse lookup from commit SHA to bead IDs
func (c *Correlator) buildCommitIndex(histories map[string]BeadHistory) CommitIndex {
	index := make(CommitIndex)

	for beadID, history := range histories {
		for _, commit := range history.Commits {
			index[commit.SHA] = append(index[commit.SHA], beadID)
		}
	}

	return index
}

// calculateStats computes aggregate statistics
func (c *Correlator) calculateStats(histories map[string]BeadHistory, commits []CorrelatedCommit) HistoryStats {
	stats := HistoryStats{
		TotalBeads:         len(histories),
		MethodDistribution: make(map[string]int),
	}

	// Track unique authors and commits
	authors := make(map[string]bool)
	uniqueCommits := make(map[string]bool)

	// Collect cycle times for average
	var cycleTimes []time.Duration

	for _, history := range histories {
		if len(history.Commits) > 0 {
			stats.BeadsWithCommits++
		}

		for _, commit := range history.Commits {
			uniqueCommits[commit.SHA] = true
			authors[commit.Author] = true
			stats.MethodDistribution[commit.Method.String()]++
		}

		for _, event := range history.Events {
			authors[event.Author] = true
		}

		// Collect cycle time
		if history.CycleTime != nil && history.CycleTime.ClaimToClose != nil {
			cycleTimes = append(cycleTimes, *history.CycleTime.ClaimToClose)
		}
	}

	stats.TotalCommits = len(uniqueCommits)
	stats.UniqueAuthors = len(authors)

	if stats.BeadsWithCommits > 0 {
		stats.AvgCommitsPerBead = float64(stats.TotalCommits) / float64(stats.BeadsWithCommits)
	}

	// Calculate average cycle time
	if len(cycleTimes) > 0 {
		var total time.Duration
		for _, ct := range cycleTimes {
			total += ct
		}
		avgDays := total.Hours() / 24 / float64(len(cycleTimes))
		stats.AvgCycleTimeDays = &avgDays
	}

	return stats
}

// describeGitRange creates a human-readable description of the git range
func (c *Correlator) describeGitRange(opts CorrelatorOptions) string {
	parts := []string{}

	if opts.Since != nil {
		parts = append(parts, fmt.Sprintf("since %s", opts.Since.Format("2006-01-02")))
	}
	if opts.Until != nil {
		parts = append(parts, fmt.Sprintf("until %s", opts.Until.Format("2006-01-02")))
	}
	if opts.Limit > 0 {
		parts = append(parts, fmt.Sprintf("limit %d commits", opts.Limit))
	}

	if len(parts) == 0 {
		return "all history"
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ", "
		}
		result += part
	}
	return result
}

// calculateDataHash creates a hash of the input beads for consistency checking
func (c *Correlator) calculateDataHash(beads []BeadInfo) string {
	h := sha256.New()
	for _, b := range beads {
		h.Write([]byte(b.ID))
		h.Write([]byte(b.Status))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// ValidateRepository checks if the repository is valid for correlation
func ValidateRepository(repoPath string) error {
	// Check if git directory exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", repoPath)
	}

	// Check if any beads file exists (multiple possible names)
	beadsFiles := []string{
		filepath.Join(repoPath, ".beads", "issues.jsonl"),
		filepath.Join(repoPath, ".beads", "beads.jsonl"),
		filepath.Join(repoPath, ".beads", "beads.base.jsonl"),
	}

	found := false
	for _, f := range beadsFiles {
		if _, err := os.Stat(f); err == nil {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("no beads file found in %s/.beads/", repoPath)
	}

	return nil
}
