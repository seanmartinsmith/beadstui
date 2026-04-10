// Package correlation provides incremental history updates to avoid full repo scans.
package correlation

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IncrementalThreshold defines the maximum number of new commits before falling back to full refresh.
// When there are more than this many new commits, it's more efficient to do a full rebuild.
const IncrementalThreshold = 100

// IncrementalUpdateResult contains the outcome of an incremental update attempt
type IncrementalUpdateResult struct {
	Report            *HistoryReport // The updated report
	WasIncremental    bool           // True if update was incremental, false if full refresh
	NewCommitCount    int            // Number of new commits processed
	MergedEventCount  int            // Number of events merged
	MergedCommitCount int            // Number of commits merged
	RefreshReason     string         // Why full refresh was used (if applicable)
}

// IncrementalCorrelator extends CachedCorrelator with incremental update support
type IncrementalCorrelator struct {
	correlator *Correlator
	cache      *HistoryCache
	hits       int64
	misses     int64
	increments int64 // Count of successful incremental updates
	refreshes  int64 // Count of full refreshes
	mu         sync.Mutex
}

// NewIncrementalCorrelator creates a correlator with incremental update support
func NewIncrementalCorrelator(repoPath string) *IncrementalCorrelator {
	return &IncrementalCorrelator{
		correlator: NewCorrelator(repoPath),
		cache:      NewHistoryCache(repoPath),
	}
}

// NewIncrementalCorrelatorWithOptions creates a correlator with custom cache settings
func NewIncrementalCorrelatorWithOptions(repoPath string, maxAge time.Duration, maxSize int) *IncrementalCorrelator {
	return &IncrementalCorrelator{
		correlator: NewCorrelator(repoPath),
		cache:      NewHistoryCacheWithOptions(repoPath, maxAge, maxSize),
	}
}

// GenerateReport generates a history report, using incremental updates when possible
func (ic *IncrementalCorrelator) GenerateReport(beads []BeadInfo, opts CorrelatorOptions) (*HistoryReport, error) {
	result, err := ic.GenerateReportWithDetails(beads, opts)
	if err != nil {
		return nil, err
	}
	return result.Report, nil
}

// GenerateReportWithDetails generates a report and returns detailed update information
func (ic *IncrementalCorrelator) GenerateReportWithDetails(beads []BeadInfo, opts CorrelatorOptions) (*IncrementalUpdateResult, error) {
	// Build cache key
	key, err := BuildCacheKey(ic.cache.repoPath, beads, opts)
	if err != nil {
		// If we can't build a cache key, do a full refresh
		report, err := ic.correlator.GenerateReport(beads, opts)
		if err != nil {
			return nil, err
		}
		return &IncrementalUpdateResult{
			Report:         report,
			WasIncremental: false,
			RefreshReason:  "failed to build cache key",
		}, nil
	}

	// Check cache
	if cached, ok := ic.cache.Get(key); ok {
		ic.mu.Lock()
		ic.hits++
		ic.mu.Unlock()
		return &IncrementalUpdateResult{
			Report:         cached,
			WasIncremental: true,
			NewCommitCount: 0,
		}, nil
	}

	// Cache miss - try incremental update if we have a cached report with same beads
	existingReport := ic.findExistingReport(beads, opts)
	if existingReport != nil && existingReport.LatestCommitSHA != "" {
		result, err := ic.tryIncrementalUpdate(existingReport, beads, opts)
		if err == nil && result != nil {
			ic.mu.Lock()
			ic.increments++
			ic.mu.Unlock()
			ic.cache.Put(key, result.Report)
			return result, nil
		}
		// If incremental failed, fall through to full refresh
	}

	// Full refresh
	ic.mu.Lock()
	ic.misses++
	ic.refreshes++
	ic.mu.Unlock()

	report, err := ic.correlator.GenerateReport(beads, opts)
	if err != nil {
		return nil, err
	}

	ic.cache.Put(key, report)

	return &IncrementalUpdateResult{
		Report:         report,
		WasIncremental: false,
		RefreshReason:  "no suitable cached report for incremental update",
	}, nil
}

// findExistingReport looks for a cached report that can be incrementally updated
func (ic *IncrementalCorrelator) findExistingReport(beads []BeadInfo, opts CorrelatorOptions) *HistoryReport {
	// Look for any cached report with the same beads hash (different HEAD is OK)
	beadsHash := hashBeads(beads)
	optsHash := hashOptions(opts)

	ic.cache.mu.RLock()
	defer ic.cache.mu.RUnlock()

	for _, entry := range ic.cache.entries {
		// Match on beads and options, but allow different HEAD
		if entry.Key.BeadsHash == beadsHash && entry.Key.Options == optsHash {
			return entry.Report
		}
	}
	return nil
}

// tryIncrementalUpdate attempts to update an existing report incrementally
func (ic *IncrementalCorrelator) tryIncrementalUpdate(existing *HistoryReport, beads []BeadInfo, opts CorrelatorOptions) (*IncrementalUpdateResult, error) {
	// Find new commits since the existing report
	newCommits, err := getCommitsSince(ic.cache.repoPath, existing.LatestCommitSHA)
	if err != nil {
		return nil, fmt.Errorf("finding new commits: %w", err)
	}

	// If too many new commits, fall back to full refresh
	if len(newCommits) > IncrementalThreshold {
		return nil, fmt.Errorf("too many new commits (%d > %d)", len(newCommits), IncrementalThreshold)
	}

	// If no new commits, the existing report is still valid
	if len(newCommits) == 0 {
		return &IncrementalUpdateResult{
			Report:         existing,
			WasIncremental: true,
			NewCommitCount: 0,
		}, nil
	}

	// Extract events from new commits only
	extractor := NewExtractor(ic.cache.repoPath, "")
	newEvents, err := extractEventsFromCommits(extractor, newCommits, opts.BeadID)
	if err != nil {
		return nil, fmt.Errorf("extracting new events: %w", err)
	}

	// Extract co-commits from new events
	coCommitter := NewCoCommitExtractor(ic.cache.repoPath)
	newCorrelatedCommits, err := coCommitter.ExtractAllCoCommits(newEvents)
	if err != nil {
		return nil, fmt.Errorf("extracting co-commits: %w", err)
	}

	// Merge new data with existing report
	merged := mergeReports(existing, beads, newEvents, newCorrelatedCommits)

	return &IncrementalUpdateResult{
		Report:            merged,
		WasIncremental:    true,
		NewCommitCount:    len(newCommits),
		MergedEventCount:  len(newEvents),
		MergedCommitCount: len(newCorrelatedCommits),
	}, nil
}

// getCommitsSince returns commit SHAs since the given commit (exclusive)
func getCommitsSince(repoPath, sinceSHA string) ([]string, error) {
	if sinceSHA == "" {
		return nil, fmt.Errorf("no since SHA provided")
	}

	// Use git rev-list to get commits since the given SHA
	cmd := exec.Command("git", "rev-list", "--reverse", fmt.Sprintf("%s..HEAD", sinceSHA))
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		// Check if the SHA exists
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git rev-list failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil // No new commits
	}

	return lines, nil
}

// countCommitsSince returns the number of commits since the given SHA
func countCommitsSince(repoPath, sinceSHA string) (int, error) {
	if sinceSHA == "" {
		return 0, fmt.Errorf("no since SHA provided")
	}

	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..HEAD", sinceSHA))
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parsing commit count: %w", err)
	}

	return count, nil
}

// extractEventsFromCommits extracts bead events from specific commits
func extractEventsFromCommits(extractor *Extractor, commitSHAs []string, filterBeadID string) ([]BeadEvent, error) {
	if len(commitSHAs) == 0 {
		return nil, nil
	}

	// Use git log with --no-walk to process specific commits exactly as listed.
	// This avoids range semantics (A..B) which can be tricky with root commits
	// or non-linear history segments.
	args := []string{
		"log",
		"-p",
		"--format=" + gitLogHeaderFormat,
		"--no-walk",
	}
	args = append(args, commitSHAs...)
	args = append(args, "--", extractor.primaryBeadsFile())

	cmd := exec.Command("git", args...)
	cmd.Dir = extractor.repoPath

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log for commits failed: %w", err)
	}

	events, err := extractor.parseGitLogOutput(bytes.NewReader(out), filterBeadID)
	if err != nil {
		return nil, err
	}

	// Reverse to chronological order (git log output depends on input order but usually LIFO)
	// We want chronological for the history report
	reverseEvents(events)
	return events, nil
}

// mergeReports creates a new report by merging existing data with new events/commits
func mergeReports(existing *HistoryReport, beads []BeadInfo, newEvents []BeadEvent, newCommits []CorrelatedCommit) *HistoryReport {
	// Create a deep copy of existing histories
	histories := make(map[string]BeadHistory, len(existing.Histories))
	for id, h := range existing.Histories {
		// Deep copy the history
		eventsCopy := make([]BeadEvent, len(h.Events))
		copy(eventsCopy, h.Events)
		commitsCopy := make([]CorrelatedCommit, len(h.Commits))
		copy(commitsCopy, h.Commits)

		histories[id] = BeadHistory{
			BeadID:     h.BeadID,
			Title:      h.Title,
			Status:     h.Status,
			Events:     eventsCopy,
			Milestones: h.Milestones,
			Commits:    commitsCopy,
			CycleTime:  h.CycleTime,
			LastAuthor: h.LastAuthor,
		}
	}

	// Add any new beads that weren't in the existing report
	for _, bead := range beads {
		if _, exists := histories[bead.ID]; !exists {
			histories[bead.ID] = BeadHistory{
				BeadID:  bead.ID,
				Title:   bead.Title,
				Status:  bead.Status,
				Events:  []BeadEvent{},
				Commits: []CorrelatedCommit{},
			}
		}
	}

	// Update bead statuses from current beads list
	for _, bead := range beads {
		if h, exists := histories[bead.ID]; exists {
			h.Title = bead.Title
			h.Status = bead.Status
			histories[bead.ID] = h
		}
	}

	// Merge new events
	eventsByBead := make(map[string][]BeadEvent)
	for _, event := range newEvents {
		eventsByBead[event.BeadID] = append(eventsByBead[event.BeadID], event)
	}

	for beadID, events := range eventsByBead {
		if h, exists := histories[beadID]; exists {
			h.Events = append(h.Events, events...)
			// Recalculate milestones
			h.Milestones = GetBeadMilestones(h.Events)
			h.CycleTime = CalculateCycleTime(h.Milestones)
			histories[beadID] = h
		}
	}

	// Merge new commits
	commitsByBead := make(map[string][]CorrelatedCommit)
	for _, commit := range newCommits {
		// Find which bead(s) this commit relates to
		for _, event := range newEvents {
			if event.CommitSHA == commit.SHA {
				commitsByBead[event.BeadID] = append(commitsByBead[event.BeadID], commit)
			}
		}
	}

	for beadID, commits := range commitsByBead {
		if h, exists := histories[beadID]; exists {
			h.Commits = dedupCommits(append(h.Commits, commits...))
			// Update last author
			if len(h.Commits) > 0 {
				h.LastAuthor = h.Commits[len(h.Commits)-1].Author
			}
			histories[beadID] = h
		}
	}

	// Build new commit index
	commitIndex := make(CommitIndex)
	for beadID, h := range histories {
		for _, commit := range h.Commits {
			commitIndex[commit.SHA] = append(commitIndex[commit.SHA], beadID)
		}
	}

	// Calculate new stats
	stats := calculateMergedStats(histories, newCommits)

	// Find latest commit SHA
	var latestTime time.Time
	var latestSHA string
	for _, event := range newEvents {
		if event.Timestamp.After(latestTime) {
			latestTime = event.Timestamp
			latestSHA = event.CommitSHA
		}
	}
	for _, commit := range newCommits {
		if commit.Timestamp.After(latestTime) {
			latestTime = commit.Timestamp
			latestSHA = commit.SHA
		}
	}
	// Fall back to existing if no new commits
	if latestSHA == "" {
		latestSHA = existing.LatestCommitSHA
	}

	return &HistoryReport{
		GeneratedAt:     time.Now().UTC(),
		DataHash:        existing.DataHash,
		GitRange:        existing.GitRange + " (incremental)",
		LatestCommitSHA: latestSHA,
		Stats:           stats,
		Histories:       histories,
		CommitIndex:     commitIndex,
	}
}

// calculateMergedStats computes statistics for the merged report
func calculateMergedStats(histories map[string]BeadHistory, newCommits []CorrelatedCommit) HistoryStats {
	stats := HistoryStats{
		TotalBeads:         len(histories),
		MethodDistribution: make(map[string]int),
	}

	authors := make(map[string]bool)
	uniqueCommits := make(map[string]bool)
	var cycleTimes []time.Duration

	for _, h := range histories {
		if len(h.Commits) > 0 {
			stats.BeadsWithCommits++
		}

		for _, commit := range h.Commits {
			uniqueCommits[commit.SHA] = true
			authors[commit.Author] = true
			stats.MethodDistribution[commit.Method.String()]++
		}

		for _, event := range h.Events {
			authors[event.Author] = true
		}

		if h.CycleTime != nil && h.CycleTime.ClaimToClose != nil {
			cycleTimes = append(cycleTimes, *h.CycleTime.ClaimToClose)
		}
	}

	stats.TotalCommits = len(uniqueCommits)
	stats.UniqueAuthors = len(authors)

	if stats.BeadsWithCommits > 0 {
		stats.AvgCommitsPerBead = float64(stats.TotalCommits) / float64(stats.BeadsWithCommits)
	}

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

// InvalidateCache clears all cached entries
func (ic *IncrementalCorrelator) InvalidateCache() {
	ic.cache.Invalidate()
}

// CacheStats returns cache and incremental update statistics
func (ic *IncrementalCorrelator) CacheStats() IncrementalCorrelatorStats {
	ic.mu.Lock()
	hits := ic.hits
	misses := ic.misses
	increments := ic.increments
	refreshes := ic.refreshes
	ic.mu.Unlock()

	cacheStats := ic.cache.Stats()

	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	var incrementRate float64
	updates := increments + refreshes
	if updates > 0 {
		incrementRate = float64(increments) / float64(updates)
	}

	return IncrementalCorrelatorStats{
		Hits:               hits,
		Misses:             misses,
		HitRate:            hitRate,
		IncrementalUpdates: increments,
		FullRefreshes:      refreshes,
		IncrementRate:      incrementRate,
		CacheSize:          cacheStats.Size,
		MaxSize:            cacheStats.MaxSize,
		MaxAge:             cacheStats.MaxAge,
	}
}

// IncrementalCorrelatorStats provides statistics about incremental update performance
type IncrementalCorrelatorStats struct {
	Hits               int64
	Misses             int64
	HitRate            float64
	IncrementalUpdates int64
	FullRefreshes      int64
	IncrementRate      float64 // Ratio of incremental updates to total non-cached updates
	CacheSize          int
	MaxSize            int
	MaxAge             time.Duration
}

// CanUpdateIncrementally checks if incremental update is possible for the given cached report
func CanUpdateIncrementally(repoPath string, cachedReport *HistoryReport) (bool, int, error) {
	if cachedReport == nil || cachedReport.LatestCommitSHA == "" {
		return false, 0, nil
	}

	count, err := countCommitsSince(repoPath, cachedReport.LatestCommitSHA)
	if err != nil {
		return false, 0, err
	}

	return count <= IncrementalThreshold, count, nil
}
