// Package correlation provides temporal correlation of commits to beads based on authorship and time windows.
package correlation

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// TemporalCorrelator finds commits by the same author within a bead's active time window
type TemporalCorrelator struct {
	repoPath     string
	coCommitter  *CoCommitExtractor // For getting file changes
	seenCommits  map[string]bool    // Track commits already correlated by higher-confidence methods
	activeByAuth map[string]int     // Count of active beads per author (for confidence scoring)
}

// NewTemporalCorrelator creates a new temporal correlator
func NewTemporalCorrelator(repoPath string) *TemporalCorrelator {
	return &TemporalCorrelator{
		repoPath:     repoPath,
		coCommitter:  NewCoCommitExtractor(repoPath),
		seenCommits:  make(map[string]bool),
		activeByAuth: make(map[string]int),
	}
}

// SetSeenCommits marks commits that were already correlated via higher-confidence methods
func (t *TemporalCorrelator) SetSeenCommits(commits []CorrelatedCommit) {
	for _, c := range commits {
		t.seenCommits[c.SHA] = true
	}
}

// SetActiveBeadsPerAuthor sets the count of active beads per author for confidence calculation
func (t *TemporalCorrelator) SetActiveBeadsPerAuthor(counts map[string]int) {
	t.activeByAuth = counts
}

// TemporalWindow represents the time window when a bead was actively being worked on
type TemporalWindow struct {
	BeadID      string
	Title       string
	Author      string
	AuthorEmail string
	Start       time.Time // When bead was claimed
	End         time.Time // When bead was closed
}

// FindCommitsInWindow finds commits by the specified author within the given time window
func (t *TemporalCorrelator) FindCommitsInWindow(window TemporalWindow) ([]CorrelatedCommit, error) {
	// Build git log command with author and date filters
	args := []string{
		"log",
		fmt.Sprintf("--author=%s", window.AuthorEmail),
		fmt.Sprintf("--since=%s", window.Start.Format(time.RFC3339)),
		fmt.Sprintf("--until=%s", window.End.Format(time.RFC3339)),
		"--format=" + gitLogHeaderFormat,
		"--no-merges",
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = t.repoPath

	out, err := cmd.Output()
	if err != nil {
		// No commits found is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	// Extract path hints from bead title for confidence scoring
	pathHints := extractPathHints(window.Title)

	// Parse commits
	var commits []CorrelatedCommit
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

		sha := info.SHA

		// Skip commits already correlated via higher-confidence methods
		if t.seenCommits[sha] {
			continue
		}

		// Skip commits that touch beads files (those are handled by co-commit extractor)
		if t.touchesBeadsFile(sha) {
			continue
		}

		// Get file changes for this commit
		files, err := t.coCommitter.ExtractCoCommittedFiles(BeadEvent{CommitSHA: sha})
		if err != nil {
			continue
		}

		// Skip if no code files
		if len(files) == 0 {
			continue
		}

		// Calculate dynamic confidence
		confidence := t.calculateTemporalConfidence(window, files, pathHints)
		reason := t.generateTemporalReason(window, files, pathHints)

		commits = append(commits, CorrelatedCommit{
			SHA:         sha,
			ShortSHA:    shortSHA(sha),
			Message:     info.Message,
			Author:      info.Author,
			AuthorEmail: info.AuthorEmail,
			Timestamp:   info.Timestamp,
			Files:       files,
			Method:      MethodTemporalAuthor,
			Confidence:  confidence,
			Reason:      reason,
		})
	}

	return commits, scanner.Err()
}

// touchesBeadsFile checks if a commit modifies any beads file
func (t *TemporalCorrelator) touchesBeadsFile(sha string) bool {
	cmd := exec.Command("git", "show", "--name-only", "--format=", sha)
	cmd.Dir = t.repoPath

	out, err := cmd.Output()
	if err != nil {
		return false
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		path := scanner.Text()
		if strings.HasPrefix(path, ".beads/") {
			return true
		}
	}
	return false
}

// calculateTemporalConfidence computes dynamic confidence for temporal correlation
func (t *TemporalCorrelator) calculateTemporalConfidence(window TemporalWindow, files []FileChange, pathHints []string) float64 {
	base := 0.50

	// Factor 1: How many beads was this author working on?
	activeBeads := t.activeByAuth[window.AuthorEmail]
	if activeBeads <= 1 {
		base += 0.20 // Only one bead = higher confidence
	} else if activeBeads == 2 {
		base += 0.10
	} else if activeBeads > 3 {
		base -= 0.10 // Many beads = lower confidence
	}

	// Factor 2: How long is the time window?
	windowDuration := window.End.Sub(window.Start)
	if windowDuration < 4*time.Hour {
		base += 0.10 // Short window = more focused
	} else if windowDuration < 24*time.Hour {
		base += 0.05
	} else if windowDuration > 7*24*time.Hour {
		base -= 0.15 // Week+ window = lots of potential commits
	} else if windowDuration > 3*24*time.Hour {
		base -= 0.05
	}

	// Factor 3: Do commit files match path hints from bead title?
	if len(pathHints) > 0 && pathsMatchHints(files, pathHints) {
		base += 0.15 // File paths match keywords in title
	}

	// Clamp to [0.20, 0.85] - temporal correlation should never be too confident
	return clamp(base, 0.20, 0.85)
}

// generateTemporalReason creates a human-readable explanation for the correlation
func (t *TemporalCorrelator) generateTemporalReason(window TemporalWindow, files []FileChange, pathHints []string) string {
	parts := []string{
		fmt.Sprintf("Commit by %s during bead's active window", window.Author),
	}

	windowDuration := window.End.Sub(window.Start)
	if windowDuration < 4*time.Hour {
		parts = append(parts, "short window (<4h)")
	} else if windowDuration > 7*24*time.Hour {
		parts = append(parts, fmt.Sprintf("long window (%dd)", int(windowDuration.Hours()/24)))
	}

	activeBeads := t.activeByAuth[window.AuthorEmail]
	if activeBeads <= 1 {
		parts = append(parts, "author had only this bead active")
	} else if activeBeads > 3 {
		parts = append(parts, fmt.Sprintf("author had %d beads active", activeBeads))
	}

	if len(pathHints) > 0 && pathsMatchHints(files, pathHints) {
		parts = append(parts, "file paths match bead title keywords")
	}

	return strings.Join(parts, "; ")
}

// pathHintPatterns extracts potential file-related keywords from text
var pathHintPattern = regexp.MustCompile(`(?i)\b(?:` +
	// File paths
	`(?:[a-z_][a-z0-9_]*(?:/[a-z_][a-z0-9_]*)+(?:\.[a-z]+)?)|` +
	// Package/module names
	`(?:pkg|src|lib|internal|cmd|app)/[a-z_][a-z0-9_]*|` +
	// Component/feature keywords
	`(?:auth|login|user|api|db|database|config|service|handler|controller|model|view|component|util|helper|test|tests)` +
	`)\b`)

// extractPathHints extracts potential file paths and keywords from bead title
func extractPathHints(title string) []string {
	matches := pathHintPattern.FindAllString(strings.ToLower(title), -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate
	seen := make(map[string]bool)
	var hints []string
	for _, m := range matches {
		m = strings.ToLower(m)
		if !seen[m] {
			seen[m] = true
			hints = append(hints, m)
		}
	}
	return hints
}

// pathsMatchHints checks if any file path contains any of the hints
func pathsMatchHints(files []FileChange, hints []string) bool {
	for _, f := range files {
		lowerPath := strings.ToLower(f.Path)
		for _, hint := range hints {
			if strings.Contains(lowerPath, hint) {
				return true
			}
		}
	}
	return false
}

// clamp restricts a value to the given range
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ExtractWindowFromMilestones creates a TemporalWindow from bead milestones
func ExtractWindowFromMilestones(beadID, title string, milestones BeadMilestones) *TemporalWindow {
	// Need both claimed and closed events to define a window
	if milestones.Claimed == nil || milestones.Closed == nil {
		return nil
	}

	return &TemporalWindow{
		BeadID:      beadID,
		Title:       title,
		Author:      milestones.Claimed.Author,
		AuthorEmail: milestones.Claimed.AuthorEmail,
		Start:       milestones.Claimed.Timestamp,
		End:         milestones.Closed.Timestamp,
	}
}

// ExtractAllTemporalCorrelations finds temporal correlations for all beads with completed windows
func (t *TemporalCorrelator) ExtractAllTemporalCorrelations(histories map[string]BeadHistory) ([]CorrelatedCommit, error) {
	var allCommits []CorrelatedCommit

	// First, calculate active beads per author at any given time
	// This is a simplification - we count all in_progress beads per author
	t.calculateActiveBeadsPerAuthor(histories)

	for beadID, history := range histories {
		window := ExtractWindowFromMilestones(beadID, history.Title, history.Milestones)
		if window == nil {
			continue
		}

		commits, err := t.FindCommitsInWindow(*window)
		if err != nil {
			// Non-fatal: skip this bead
			continue
		}

		allCommits = append(allCommits, commits...)
	}

	return allCommits, nil
}

// calculateActiveBeadsPerAuthor computes how many beads each author had in progress
func (t *TemporalCorrelator) calculateActiveBeadsPerAuthor(histories map[string]BeadHistory) {
	authorCounts := make(map[string]int)

	for _, history := range histories {
		if history.Milestones.Claimed != nil {
			email := history.Milestones.Claimed.AuthorEmail
			authorCounts[email]++
		}
	}

	t.activeByAuth = authorCounts
}
