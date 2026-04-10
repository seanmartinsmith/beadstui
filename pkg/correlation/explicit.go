// Package correlation provides explicit bead ID matching from commit messages.
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

// ExplicitMatcher finds commits that explicitly reference bead IDs in messages.
type ExplicitMatcher struct {
	repoPath string
	patterns []*regexp.Regexp
}

// DefaultPatterns returns the default set of bead ID patterns.
func DefaultPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		// [ID] format - very explicit
		regexp.MustCompile(`\[([A-Za-z]+-\d+)\]`),

		// Closes/Fixes/Refs keywords with optional # prefix
		// Note: Allow optional colon and whitespace after keyword
		regexp.MustCompile(`(?i)closes?:?\s*#?([A-Za-z]+-\d+)`),
		regexp.MustCompile(`(?i)fix(?:es|ed)?:?\s*#?([A-Za-z]+-\d+)`),
		regexp.MustCompile(`(?i)refs?:?\s*#?([A-Za-z]+-\d+)`),
		regexp.MustCompile(`(?i)resolves?:?\s*#?([A-Za-z]+-\d+)`),

		// beads-123 or bead-123 format (common for this project)
		regexp.MustCompile(`(?i)beads?[-_](\d+)`),
		regexp.MustCompile(`(?i)bv[-_](\d+)`),

		// Generic ID at word boundary (PROJECT-123 style)
		regexp.MustCompile(`\b([A-Z]{2,10}-\d+)\b`),
	}
}

// NewExplicitMatcher creates a new explicit matcher with default patterns.
func NewExplicitMatcher(repoPath string) *ExplicitMatcher {
	return &ExplicitMatcher{
		repoPath: repoPath,
		patterns: DefaultPatterns(),
	}
}

// NewExplicitMatcherWithPatterns creates a matcher with custom patterns.
func NewExplicitMatcherWithPatterns(repoPath string, patterns []*regexp.Regexp) *ExplicitMatcher {
	return &ExplicitMatcher{
		repoPath: repoPath,
		patterns: patterns,
	}
}

// AddPattern adds a custom pattern to the matcher.
func (m *ExplicitMatcher) AddPattern(pattern *regexp.Regexp) {
	m.patterns = append(m.patterns, pattern)
}

// ExplicitMatch represents a bead ID found in a commit message.
type ExplicitMatch struct {
	BeadID      string
	CommitSHA   string
	Message     string
	Author      string
	AuthorEmail string
	Timestamp   time.Time
	MatchType   string // "closes", "fixes", "refs", "bracket", "generic"
	Confidence  float64
}

// ExtractIDsFromMessage extracts all bead IDs from a commit message.
// Ordering: matches are returned in the order patterns are evaluated; we also keep stable ID ordering for predictability.
func (m *ExplicitMatcher) ExtractIDsFromMessage(message string) []IDMatch {
	var matches []IDMatch
	seen := make(map[string]bool)

	for _, pattern := range m.patterns {
		found := pattern.FindAllStringSubmatch(message, -1)
		for _, match := range found {
			if len(match) >= 2 {
				id := normalizeBeadID(match[1])
				if !seen[id] {
					seen[id] = true
					matchType := classifyMatch(match[0])
					matches = append(matches, IDMatch{
						ID:        id,
						MatchType: matchType,
						RawMatch:  match[0],
					})
				}
			}
		}
	}

	return matches
}

// IDMatch represents a single ID match from a message.
type IDMatch struct {
	ID        string
	MatchType string
	RawMatch  string
}

// normalizeBeadID normalizes a bead ID to a consistent format.
func normalizeBeadID(id string) string {
	// Handle numeric-only IDs (from beads-123 pattern)
	if _, err := fmt.Sscanf(id, "%d", new(int)); err == nil {
		return "bv-" + id
	}
	// Convert to lowercase for consistency
	return strings.ToLower(id)
}

// classifyMatch determines the type of match based on the raw match string.
func classifyMatch(raw string) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "close"):
		return "closes"
	case strings.Contains(lower, "fix"):
		return "fixes"
	case strings.Contains(lower, "ref"):
		return "refs"
	case strings.Contains(lower, "resolve"):
		return "resolves"
	case strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]"):
		return "bracket"
	case strings.HasPrefix(lower, "bead") || strings.HasPrefix(lower, "bv"):
		return "bead"
	default:
		return "generic"
	}
}

// CalculateConfidence calculates confidence for an explicit match.
func CalculateConfidence(matchType string, totalMatches int) float64 {
	// Base confidence for explicit ID mention
	base := 0.90

	// Bonus for action keywords
	switch matchType {
	case "closes", "fixes", "resolves":
		base += 0.05 // Strong intent signal
	case "bracket":
		base += 0.02 // Explicit but no action
	case "refs":
		base += 0.01 // Just a reference
	case "bead":
		base += 0.03 // Project-specific format
	}

	// Penalty for multiple IDs in same message (less specific)
	if totalMatches > 1 {
		base -= 0.02 * float64(totalMatches-1)
	}

	// Clamp to reasonable bounds
	if base > 0.99 {
		base = 0.99
	}
	if base < 0.70 {
		base = 0.70
	}

	return base
}

// FindCommitsForBead finds all commits that explicitly reference a bead ID.
func (m *ExplicitMatcher) FindCommitsForBead(beadID string, opts ExtractOptions) ([]ExplicitMatch, error) {
	// Use git log --grep to efficiently find commits mentioning this ID
	patterns := m.buildGrepPatterns(beadID)

	var allMatches []ExplicitMatch
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		matches, err := m.searchWithGrep(pattern, opts)
		if err != nil {
			// Non-fatal: try other patterns
			continue
		}

		for _, match := range matches {
			if !seen[match.CommitSHA] {
				seen[match.CommitSHA] = true
				allMatches = append(allMatches, match)
			}
		}
	}

	return allMatches, nil
}

// buildGrepPatterns creates grep patterns for a bead ID.
func (m *ExplicitMatcher) buildGrepPatterns(beadID string) []string {
	// Normalize the ID
	id := strings.ToLower(beadID)

	// Extract numeric part if present
	var numericPart string
	if idx := strings.LastIndex(id, "-"); idx != -1 {
		numericPart = id[idx+1:]
	}

	patterns := []string{
		// Exact ID
		beadID,
		strings.ToUpper(beadID),
	}

	// If it's a bv-XXX style ID, also search for beads-XXX
	if strings.HasPrefix(id, "bv-") && numericPart != "" {
		patterns = append(patterns,
			"beads-"+numericPart,
			"bead-"+numericPart,
			"BEADS-"+numericPart,
			"BEAD-"+numericPart,
		)
	}

	return patterns
}

// searchWithGrep runs git log --grep and parses results.
func (m *ExplicitMatcher) searchWithGrep(pattern string, opts ExtractOptions) ([]ExplicitMatch, error) {
	args := []string{
		"log",
		"--grep=" + pattern,
		"-i", // Case insensitive
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

	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoPath

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// git log returns exit 0 even with no results, so this is a real error
			return nil, fmt.Errorf("git log --grep failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git log --grep failed: %w", err)
	}

	return m.parseGrepOutput(out, pattern)
}

// parseGrepOutput parses git log output into ExplicitMatch structs.
func (m *ExplicitMatcher) parseGrepOutput(data []byte, searchPattern string) ([]ExplicitMatch, error) {
	var matches []ExplicitMatch

	scanner := bufio.NewScanner(bytes.NewReader(data))
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

		message := info.Message

		// Extract all IDs from this message
		idMatches := m.ExtractIDsFromMessage(message)

		// Calculate confidence based on match type and count
		confidence := 0.90
		var matchType string

		for _, idMatch := range idMatches {
			// Check if this ID matches what we searched for
			if strings.EqualFold(idMatch.ID, searchPattern) ||
				strings.Contains(strings.ToLower(idMatch.RawMatch), strings.ToLower(searchPattern)) {
				matchType = idMatch.MatchType
				confidence = CalculateConfidence(idMatch.MatchType, len(idMatches))
				break
			}
		}

		if matchType == "" {
			matchType = "generic"
		}

		matches = append(matches, ExplicitMatch{
			BeadID:      searchPattern,
			CommitSHA:   info.SHA,
			Message:     message,
			Author:      info.Author,
			AuthorEmail: info.AuthorEmail,
			Timestamp:   info.Timestamp,
			MatchType:   matchType,
			Confidence:  confidence,
		})
	}

	return matches, scanner.Err()
}

// CreateCorrelatedCommit converts an ExplicitMatch to a CorrelatedCommit.
func (m *ExplicitMatcher) CreateCorrelatedCommit(match ExplicitMatch, coCommitter *CoCommitExtractor) CorrelatedCommit {
	// Create a BeadEvent to get file information
	event := BeadEvent{
		BeadID:      match.BeadID,
		CommitSHA:   match.CommitSHA,
		CommitMsg:   match.Message,
		Author:      match.Author,
		AuthorEmail: match.AuthorEmail,
		Timestamp:   match.Timestamp,
	}

	// Try to get file changes
	var files []FileChange
	if coCommitter != nil {
		files, _ = coCommitter.ExtractCoCommittedFiles(event)
	}

	reason := fmt.Sprintf("Commit message explicitly references %s (%s)", match.BeadID, match.MatchType)

	return CorrelatedCommit{
		BeadID:      match.BeadID,
		SHA:         match.CommitSHA,
		ShortSHA:    shortSHA(match.CommitSHA),
		Message:     match.Message,
		Author:      match.Author,
		AuthorEmail: match.AuthorEmail,
		Timestamp:   match.Timestamp,
		Files:       files,
		Method:      MethodExplicitID,
		Confidence:  match.Confidence,
		Reason:      reason,
	}
}

// FindAllExplicitMatches finds explicit references for all known bead IDs.
func (m *ExplicitMatcher) FindAllExplicitMatches(beadIDs []string, opts ExtractOptions) (map[string][]ExplicitMatch, error) {
	results := make(map[string][]ExplicitMatch)

	for _, beadID := range beadIDs {
		matches, err := m.FindCommitsForBead(beadID, opts)
		if err != nil {
			// Non-fatal: continue with other beads
			continue
		}
		if len(matches) > 0 {
			results[beadID] = matches
		}
	}

	return results, nil
}
