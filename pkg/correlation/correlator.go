// Package correlation provides the Correlator for building complete bead history reports.
package correlation

import (
	"crypto/sha256"
	"database/sql"
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
	// doltDB is an optional, already-open connection to a beads Dolt server.
	// When non-nil and the repo has no JSONL on disk, GenerateReport dispatches
	// to the Dolt-native DoltExtractor instead of the JSONL+git-diff Extractor
	// (bt-08sh.4). Borrowed, not owned: the Correlator does not Close it.
	doltDB *sql.DB
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

// NewCorrelatorWithDolt creates a correlator that can also read events from a
// Dolt server when the repo has migrated off JSONL. Callers that have an
// already-open *datasource.DoltReader pass reader.DB() here; the underlying
// connection is borrowed, not owned.
//
// When the repo still has .beads/*.jsonl on disk, GenerateReport ignores
// doltDB and uses the JSONL+git-diff Extractor (unchanged behavior). When the
// repo is Dolt-only (no JSONL on disk) and doltDB is non-nil, GenerateReport
// dispatches to the Dolt-native DoltExtractor. When the repo is Dolt-only and
// doltDB is nil (no caller opted in), GenerateReport returns an empty events
// list rather than failing — consumers inspect RepoStatus.JSONLTracked to
// distinguish that case from "no events recorded".
func NewCorrelatorWithDolt(repoPath string, doltDB *sql.DB, beadsFilePath ...string) *Correlator {
	c := NewCorrelator(repoPath, beadsFilePath...)
	c.doltDB = doltDB
	return c
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
	// bt-nyjj: probe whether the path is inside a git work tree before invoking
	// `git log`. If it is not (e.g. user launched `bt` from $HOME), return an
	// empty report with no error so the history view shows a friendly empty
	// state instead of a red "git log failed" banner. Real git failures
	// (binary missing, permission errors, repo corruption) still propagate.
	insideRepo, err := IsInsideWorkTree(c.repoPath)
	if err != nil {
		return nil, fmt.Errorf("checking git repository: %w", err)
	}
	if !insideRepo {
		return c.emptyReport(beads, opts), nil
	}
	jsonlTracked := HasJSONLOnDisk(c.repoPath)
	repoStatus := RepoStatus{RepoPath: c.repoPath, InsideWorkTree: true, JSONLTracked: jsonlTracked}

	// Build extract options
	extractOpts := ExtractOptions{
		Since:  opts.Since,
		Until:  opts.Until,
		Limit:  opts.Limit,
		BeadID: opts.BeadID,
	}

	// bt-08sh.4 (Option C of bt-592c): dispatch on whether JSONL is still on
	// disk. JSONL-tracked repos keep the historical extractor (git log over
	// .beads/*.jsonl plus diff witness). Dolt-only repos read events from the
	// upstream events / wisp_events tables via DoltExtractor, but only when a
	// caller opted in by handing us a *sql.DB (NewCorrelatorWithDolt). Callers
	// that constructed the plain NewCorrelator on a Dolt-only repo get back an
	// empty events list rather than a synthetic error — RepoStatus.JSONLTracked
	// tells the consumer which case it is.
	events, err := c.extractEvents(jsonlTracked, extractOpts)
	if err != nil {
		return nil, fmt.Errorf("extracting events: %w", err)
	}

	// Extract co-committed files
	commits, err := c.coCommitter.ExtractAllCoCommits(events)
	if err != nil {
		return nil, fmt.Errorf("extracting co-commits: %w", err)
	}

	// bt-ydjw.5: on the Dolt path, co-commit correlation is structurally
	// impossible (events have no CommitSHA per 592c), so commits would be
	// empty and the History view's COMMITS pane stays blank. Run the
	// explicit-ID matcher against host git history to recover developer-
	// declared bead-to-commit links from commit message subjects. This
	// signal class is intent-based (commit message text), not heuristic
	// (author/temporal proximity), so 592c's rejection of git correlation on
	// the Dolt path -- which targeted MethodTemporalAuthor specifically --
	// does not apply. Errors are non-fatal: the report still renders with
	// events but no commits.
	if !jsonlTracked {
		commits = append(commits, c.explicitCommits(extractOpts, beads)...)
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
		RepoStatus:      repoStatus,
	}, nil
}

// emptyReport builds a HistoryReport for the "cwd is not a git repo" case
// (bt-nyjj). It populates per-bead history shells so the bead list still
// renders, but with no events or commits — the history view's renderEmpty
// path then shows a friendly empty state instead of an error banner.
func (c *Correlator) emptyReport(beads []BeadInfo, opts CorrelatorOptions) *HistoryReport {
	histories := c.buildHistories(beads, nil, nil)

	if opts.BeadID != "" {
		filtered := make(map[string]BeadHistory)
		if h, ok := histories[opts.BeadID]; ok {
			filtered[opts.BeadID] = h
		}
		histories = filtered
	}

	return &HistoryReport{
		GeneratedAt: time.Now().UTC(),
		DataHash:    c.calculateDataHash(beads),
		GitRange:    c.describeGitRange(opts),
		Stats:       c.calculateStats(histories, nil),
		Histories:   histories,
		CommitIndex: c.buildCommitIndex(histories),
		RepoStatus: RepoStatus{
			RepoPath:       c.repoPath,
			InsideWorkTree: false,
			JSONLTracked:   HasJSONLOnDisk(c.repoPath),
		},
	}
}

// explicitCommits runs a single git log scan via ExplicitMatcher and returns
// CorrelatedCommit entries for any commit message that references a bead ID
// in the provided beads list. Used by GenerateReport on the Dolt path
// (bt-ydjw.5); the JSONL path already covers explicit-ID via co-commit's
// containsBeadID confidence bump plus its own MethodExplicitID flow.
//
// Errors from ScanCommits are swallowed deliberately: a malformed git output
// or a repo with no commits is not fatal to the report -- the consumer sees
// events with empty COMMITS, which is the same UX as a fresh Dolt-only repo.
//
// Bead-set filtering: ExplicitMatcher.ScanCommits emits every dashed token
// in commit messages (a deliberate trade for one-pass scan performance), so
// the inner loop drops anything whose normalized ID isn't in the bead list.
// This is what keeps the dispatcher's contract honest: a commit attaches to
// bead X only if bead X exists in the report.
func (c *Correlator) explicitCommits(opts ExtractOptions, beads []BeadInfo) []CorrelatedCommit {
	if len(beads) == 0 {
		return nil
	}
	matcher := NewExplicitMatcher(c.repoPath)
	byBead, err := matcher.ScanCommits(opts)
	if err != nil || len(byBead) == 0 {
		return nil
	}

	known := make(map[string]struct{}, len(beads))
	for _, b := range beads {
		known[b.ID] = struct{}{}
	}

	var commits []CorrelatedCommit
	for beadID, matches := range byBead {
		if _, ok := known[beadID]; !ok {
			continue
		}
		for _, m := range matches {
			// Pass c.coCommitter so the resulting CorrelatedCommit carries
			// file-change metadata. CreateCorrelatedCommit tolerates nil
			// (returns no files), but the bt-h COMMITS pane is much more
			// useful with files attached.
			commits = append(commits, matcher.CreateCorrelatedCommit(m, c.coCommitter))
		}
	}
	return commits
}

// extractEvents routes between the JSONL+git-diff extractor (legacy repos that
// still have .beads/*.jsonl on disk) and the Dolt-native DoltExtractor
// (Dolt-only repos, when the caller opted in by providing a *sql.DB). When the
// repo is Dolt-only and no DB was provided, returns an empty slice — consumers
// distinguish "Dolt-only, no extractor wired" from "JSONL-tracked, no events"
// via RepoStatus.JSONLTracked.
func (c *Correlator) extractEvents(jsonlTracked bool, opts ExtractOptions) ([]BeadEvent, error) {
	if jsonlTracked {
		return c.extractor.Extract(opts)
	}
	if c.doltDB == nil {
		return nil, nil
	}
	return NewDoltExtractor(c.doltDB).Extract(opts)
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
