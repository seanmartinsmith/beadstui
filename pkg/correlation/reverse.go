// Package correlation provides reverse lookup from commits to beads.
package correlation

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommitBeadResult represents the result of a commit-to-bead lookup.
type CommitBeadResult struct {
	CommitSHA    string        `json:"commit_sha"`
	ShortSHA     string        `json:"short_sha"`
	Message      string        `json:"message"`
	Author       string        `json:"author"`
	AuthorEmail  string        `json:"author_email"`
	Timestamp    time.Time     `json:"timestamp"`
	RelatedBeads []RelatedBead `json:"related_beads"`
	IsOrphan     bool          `json:"is_orphan"` // True if no beads found
}

// RelatedBead represents a bead related to a commit.
type RelatedBead struct {
	BeadID     string            `json:"bead_id"`
	BeadTitle  string            `json:"bead_title"`
	BeadStatus string            `json:"bead_status"`
	Method     CorrelationMethod `json:"method"`
	Confidence float64           `json:"confidence"`
	Reason     string            `json:"reason"`
}

// ReverseLookup provides reverse lookup from commits to beads.
type ReverseLookup struct {
	repoPath string
	index    CommitIndex                   // SHA -> []BeadID
	details  map[string][]CorrelatedCommit // SHA -> commits with full details
	beads    map[string]BeadHistory        // BeadID -> history
}

// NewReverseLookup creates a new reverse lookup from a history report.
func NewReverseLookup(report *HistoryReport) *ReverseLookup {
	rl := &ReverseLookup{
		index:   report.CommitIndex,
		beads:   report.Histories,
		details: make(map[string][]CorrelatedCommit),
	}

	// Build details map for quick access
	for _, history := range report.Histories {
		for _, commit := range history.Commits {
			rl.details[commit.SHA] = append(rl.details[commit.SHA], commit)
		}
	}

	return rl
}

// NewReverseLookupWithRepo creates a reverse lookup that can also query git.
func NewReverseLookupWithRepo(report *HistoryReport, repoPath string) *ReverseLookup {
	rl := NewReverseLookup(report)
	rl.repoPath = repoPath
	return rl
}

// LookupByCommit finds all beads related to a commit.
func (rl *ReverseLookup) LookupByCommit(sha string) (*CommitBeadResult, error) {
	// Normalize SHA (handle short SHAs)
	fullSHA := rl.normalizeSHA(sha)

	result := &CommitBeadResult{
		CommitSHA:    fullSHA,
		ShortSHA:     shortSHA(fullSHA),
		RelatedBeads: []RelatedBead{},
	}

	// Try to get commit info from our details
	if commits, ok := rl.details[fullSHA]; ok && len(commits) > 0 {
		first := commits[0]
		result.Message = first.Message
		result.Author = first.Author
		result.AuthorEmail = first.AuthorEmail
		result.Timestamp = first.Timestamp
	} else if rl.repoPath != "" {
		// Fall back to git for commit info
		info, err := rl.getCommitInfo(fullSHA)
		if err == nil {
			result.Message = info.Message
			result.Author = info.Author
			result.AuthorEmail = info.AuthorEmail
			result.Timestamp = info.Timestamp
		}
	}

	// Find related beads
	beadIDs := rl.index[fullSHA]
	if len(beadIDs) == 0 {
		// Try prefix match for short SHAs
		for indexSHA := range rl.index {
			if strings.HasPrefix(indexSHA, sha) {
				beadIDs = rl.index[indexSHA]
				result.CommitSHA = indexSHA
				result.ShortSHA = shortSHA(indexSHA)
				break
			}
		}
	}

	if len(beadIDs) == 0 {
		result.IsOrphan = true
		return result, nil
	}

	// Build related beads with details
	for _, beadID := range beadIDs {
		history, ok := rl.beads[beadID]
		if !ok {
			continue
		}

		// Find the correlation details for this commit
		var method CorrelationMethod
		var confidence float64
		var reason string

		for _, commit := range history.Commits {
			if commit.SHA == result.CommitSHA {
				method = commit.Method
				confidence = commit.Confidence
				reason = commit.Reason
				break
			}
		}

		result.RelatedBeads = append(result.RelatedBeads, RelatedBead{
			BeadID:     beadID,
			BeadTitle:  history.Title,
			BeadStatus: history.Status,
			Method:     method,
			Confidence: confidence,
			Reason:     reason,
		})
	}

	return result, nil
}

// normalizeSHA tries to expand a short SHA to full SHA if found in index.
func (rl *ReverseLookup) normalizeSHA(sha string) string {
	// Already in index
	if _, ok := rl.index[sha]; ok {
		return sha
	}

	// Try prefix match
	for indexSHA := range rl.index {
		if strings.HasPrefix(indexSHA, sha) {
			return indexSHA
		}
	}

	return sha
}

// getCommitInfo retrieves commit info from git.
// Uses commitInfo type from extractor.go
func (rl *ReverseLookup) getCommitInfo(sha string) (*commitInfo, error) {
	if rl.repoPath == "" {
		return nil, fmt.Errorf("no repo path configured")
	}

	cmd := exec.Command("git", "log", "-1", "--format="+gitLogHeaderFormat, sha)
	cmd.Dir = rl.repoPath

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	line := strings.TrimSpace(string(out))
	info, err := parseCommitInfo(line)
	if err != nil {
		return nil, fmt.Errorf("parse git log output: %w", err)
	}

	return &info, nil
}

// OrphanCommit represents a commit with no associated bead.
type OrphanCommit struct {
	SHA         string    `json:"sha"`
	ShortSHA    string    `json:"short_sha"`
	Message     string    `json:"message"`
	Author      string    `json:"author"`
	AuthorEmail string    `json:"author_email"`
	Timestamp   time.Time `json:"timestamp"`
}

// OrphanStats provides statistics about orphan commits.
type OrphanStats struct {
	TotalCommits   int     `json:"total_commits"`      // All code commits in period
	OrphanCommits  int     `json:"orphan_commits"`     // Commits with no bead
	CorrelatedCmts int     `json:"correlated_commits"` // Commits with at least one bead
	OrphanRatio    float64 `json:"orphan_ratio"`       // orphan / total
}

// FindOrphanCommits finds commits that don't correlate to any bead.
func (rl *ReverseLookup) FindOrphanCommits(opts ExtractOptions) ([]OrphanCommit, *OrphanStats, error) {
	if rl.repoPath == "" {
		return nil, nil, fmt.Errorf("no repo path configured for orphan detection")
	}

	// Get all code commits in the time range
	allCommits, err := rl.getAllCodeCommits(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("getting code commits: %w", err)
	}

	// Find orphans
	var orphans []OrphanCommit
	correlated := 0

	for _, commit := range allCommits {
		if _, ok := rl.index[commit.SHA]; ok {
			correlated++
			continue
		}
		orphans = append(orphans, commit)
	}

	stats := &OrphanStats{
		TotalCommits:   len(allCommits),
		OrphanCommits:  len(orphans),
		CorrelatedCmts: correlated,
	}

	if stats.TotalCommits > 0 {
		stats.OrphanRatio = float64(stats.OrphanCommits) / float64(stats.TotalCommits)
	}

	return orphans, stats, nil
}

// getAllCodeCommits gets all code commits (excluding merge commits and beads-only changes).
func (rl *ReverseLookup) getAllCodeCommits(opts ExtractOptions) ([]OrphanCommit, error) {
	args := []string{
		"log",
		"--no-merges",
		"--format=" + gitLogHeaderFormat,
	}

	// Add time filters
	if opts.Since != nil {
		args = append(args, fmt.Sprintf("--since=%s", opts.Since.Format(time.RFC3339)))
	}
	if opts.Until != nil {
		args = append(args, fmt.Sprintf("--until=%s", opts.Until.Format(time.RFC3339)))
	}
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", opts.Limit))
	}

	// Exclude beads-only commits
	args = append(args, "--", ":(exclude).beads/*")

	cmd := exec.Command("git", args...)
	cmd.Dir = rl.repoPath

	out, err := cmd.Output()
	if err != nil {
		// Try without exclusion pattern (older git versions)
		args = args[:len(args)-2]
		cmd = exec.Command("git", args...)
		cmd.Dir = rl.repoPath
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git log failed: %w", err)
		}
	}

	var commits []OrphanCommit
	scanner := bufio.NewScanner(bytes.NewReader(out))
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, gitLogMaxScanTokenSize)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		info, err := parseCommitInfo(line)
		if err != nil {
			continue
		}

		commits = append(commits, OrphanCommit{
			SHA:         info.SHA,
			ShortSHA:    shortSHA(info.SHA),
			Message:     info.Message,
			Author:      info.Author,
			AuthorEmail: info.AuthorEmail,
			Timestamp:   info.Timestamp,
		})
	}

	return commits, scanner.Err()
}

// GetCorrelatedCommitCount returns the number of commits that have at least one bead association.
func (rl *ReverseLookup) GetCorrelatedCommitCount() int {
	return len(rl.index)
}

// GetAllBeadIDs returns all bead IDs that have correlations.
func (rl *ReverseLookup) GetAllBeadIDs() []string {
	ids := make([]string, 0, len(rl.beads))
	for id := range rl.beads {
		ids = append(ids, id)
	}
	return ids
}

// BeadCommitsSummary provides a summary of commits per bead.
type BeadCommitsSummary struct {
	BeadID      string  `json:"bead_id"`
	BeadTitle   string  `json:"bead_title"`
	CommitCount int     `json:"commit_count"`
	AvgConfid   float64 `json:"avg_confidence"`
	TopMethod   string  `json:"top_method"` // Most common correlation method
}

// GetBeadCommitSummaries returns summaries of commits per bead.
func (rl *ReverseLookup) GetBeadCommitSummaries() []BeadCommitsSummary {
	var summaries []BeadCommitsSummary

	for beadID, history := range rl.beads {
		if len(history.Commits) == 0 {
			continue
		}

		// Calculate average confidence and count methods
		var totalConfidence float64
		methodCounts := make(map[string]int)

		for _, commit := range history.Commits {
			totalConfidence += commit.Confidence
			methodCounts[commit.Method.String()]++
		}

		// Find top method
		topMethod := ""
		topCount := 0
		for method, count := range methodCounts {
			if count > topCount {
				topMethod = method
				topCount = count
			}
		}

		summaries = append(summaries, BeadCommitsSummary{
			BeadID:      beadID,
			BeadTitle:   history.Title,
			CommitCount: len(history.Commits),
			AvgConfid:   totalConfidence / float64(len(history.Commits)),
			TopMethod:   topMethod,
		})
	}

	return summaries
}
