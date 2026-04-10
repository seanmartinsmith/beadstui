// Package correlation provides extraction of bead lifecycle events from git history.
package correlation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ExtractOptions controls which commits and beads to extract events from
type ExtractOptions struct {
	Since  *time.Time // Only commits after this time (nil = no limit)
	Until  *time.Time // Only commits before this time (nil = no limit)
	Limit  int        // Max commits to process (0 = no limit)
	BeadID string     // Filter to single bead ID (empty = all beads)
}

// Extractor extracts bead lifecycle events from git history
type Extractor struct {
	repoPath   string
	beadsFiles []string // Files to track (e.g., .beads/beads.jsonl, .beads/issues.jsonl)
}

// NewExtractor creates a new extractor for the given repository.
// beadsFilePath is optional; when empty the extractor will track the standard
// Beads files inside .beads/. A variadic parameter is used to preserve
// backward compatibility with existing call sites that pass only repoPath.
func NewExtractor(repoPath string, beadsFilePath ...string) *Extractor {
	e := &Extractor{
		repoPath:   repoPath,
		beadsFiles: pickBeadsFiles(repoPath, defaultBeadsFiles),
	}

	// If a specific file is provided, prioritize it
	var beadPath string
	if len(beadsFilePath) > 0 {
		beadPath = beadsFilePath[0]
	}
	if beadPath != "" {
		// Ensure relative path if possible, though absolute usually works with git if inside repo
		// For simplicity, we prepend it to the list so it's picked up by buildGitLogArgs as primary
		rel, err := filepath.Rel(repoPath, beadPath)
		if err == nil {
			e.beadsFiles = prependBeadsFile(rel, e.beadsFiles)
		} else {
			e.beadsFiles = prependBeadsFile(beadPath, e.beadsFiles)
		}
	}

	return e
}

func (e *Extractor) primaryBeadsFile() string {
	if len(e.beadsFiles) > 0 && e.beadsFiles[0] != "" {
		return e.beadsFiles[0]
	}
	return defaultBeadsFiles[0]
}

// commitInfo holds parsed commit metadata
type commitInfo struct {
	SHA         string
	Timestamp   time.Time
	Author      string
	AuthorEmail string
	Message     string
}

// beadSnapshot represents a bead's state at a point in time
type beadSnapshot struct {
	ID     string
	Status string
	Title  string
}

// Extract extracts bead lifecycle events from git history
func (e *Extractor) Extract(opts ExtractOptions) ([]BeadEvent, error) {
	// Build git log command
	logArgs := e.buildGitLogArgs(opts)

	// Inject config to disable colors (essential for parsing)
	args := append([]string{"-c", "color.ui=false"}, logArgs...)

	cmd := exec.Command("git", args...)
	cmd.Dir = e.repoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting git log: %w", err)
	}

	// Parse output stream
	events, parseErr := e.parseGitLogOutput(stdout, opts.BeadID)

	// If parsing failed, ensure we drain the pipe or kill the process to avoid deadlock
	// where git log is blocked writing to full pipe while we wait for it to exit.
	if parseErr != nil {
		// Try to kill the process to unblock the write
		_ = cmd.Process.Kill()
		// We still need to wait to clean up zombies, but now it should exit quickly
		_ = cmd.Wait()
		return nil, fmt.Errorf("parsing git log output: %w", parseErr)
	}

	if err := cmd.Wait(); err != nil {
		// If git log failed (non-zero exit), prefer that error
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git log failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	// Sort chronologically (git log returns newest first)
	reverseEvents(events)

	return events, nil
}

// buildGitLogArgs constructs the git log command arguments
func (e *Extractor) buildGitLogArgs(opts ExtractOptions) []string {
	args := []string{
		"log",
		"-p",                             // Include patch/diff
		"--follow",                       // Track renames; requires a single pathspec (handled below)
		"--format=" + gitLogHeaderFormat, // Custom format for commit info
		"--",
	}

	// Add time filters before "--"
	if opts.Since != nil {
		args = insertBefore(args, "--", fmt.Sprintf("--since=%s", opts.Since.Format(time.RFC3339)))
	}
	if opts.Until != nil {
		args = insertBefore(args, "--", fmt.Sprintf("--until=%s", opts.Until.Format(time.RFC3339)))
	}
	if opts.Limit > 0 {
		args = insertBefore(args, "--", fmt.Sprintf("-n%d", opts.Limit))
	}

	// Optimization: If filtering by BeadID, tell git to only show commits
	// where this ID appears in the diff (added or removed).
	// We use -G with a regex to match the JSON field to avoid false positives on common IDs like "1".
	// Regex matches: "id": [whitespace] "BEAD_ID"
	if opts.BeadID != "" {
		// Escape regex meta-characters in BeadID just in case
		safeID := regexp.QuoteMeta(opts.BeadID)
		regex := fmt.Sprintf(`"id":\s*"%s"`, safeID)
		args = insertBefore(args, "--", fmt.Sprintf("-G%s", regex))
	}

	// Use primary beads file for follow support (git requires single pathspec with --follow)
	primary := ".beads/beads.jsonl"
	if len(e.beadsFiles) > 0 {
		primary = e.beadsFiles[0]
	}
	args = append(args, primary)

	return args
}

// insertBefore inserts a value before a marker in a slice
func insertBefore(slice []string, marker, value string) []string {
	for i, v := range slice {
		if v == marker {
			result := make([]string, 0, len(slice)+1)
			result = append(result, slice[:i]...)
			result = append(result, value)
			result = append(result, slice[i:]...)
			return result
		}
	}
	return slice
}

// parseGitLogOutput parses the combined commit info and diff output from a stream
func (e *Extractor) parseGitLogOutput(r io.Reader, filterBeadID string) ([]BeadEvent, error) {
	var events []BeadEvent

	// Use bufio.Reader instead of Scanner to handle long lines
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	reader := bufio.NewReaderSize(r, maxScanTokenSize)

	var currentCommit *commitInfo
	var diffBuffer bytes.Buffer

	// Helper to process the accumulated commit
	processCommit := func() {
		if currentCommit == nil {
			return
		}
		diffBytes := diffBuffer.Bytes()
		if len(diffBytes) > 0 {
			diffEvents := e.parseDiff(diffBytes, *currentCommit, filterBeadID)
			events = append(events, diffEvents...)
		}
		diffBuffer.Reset()
	}

	for {
		// ReadLine returns a single line, not including the end-of-line bytes.
		lineBytes, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if isPrefix {
			// Line too long. Skip it.
			// Consume until newline or EOF
			for isPrefix {
				_, isPrefix, err = reader.ReadLine()
				if err != nil && err != io.EOF {
					return nil, err
				}
				if err == io.EOF {
					break
				}
			}
			// Skip processing this line as it's incomplete/truncated
			continue
		}

		line := string(lineBytes)

		// Check for commit header
		if commitPattern.MatchString(line) {
			// Finish previous commit
			processCommit()

			// Parse new header
			info, err := parseCommitInfo(line)
			if err != nil {
				// Treat as diff content if parsing fails but regex matched
				// (Shouldn't happen with our regex, but safe fallback)
				diffBuffer.WriteString(line)
				diffBuffer.WriteByte('\n')
				continue
			}

			currentCommit = &info
		} else {
			// Diff content
			if currentCommit != nil {
				diffBuffer.WriteString(line)
				diffBuffer.WriteByte('\n')
			}
		}
	}

	// Process final commit
	processCommit()

	return events, nil
}

// commitPattern matches the start of a commit in our custom log format
var commitPattern = regexp.MustCompile(`(?m)^[0-9a-f]{40}\x00`)

// parseCommitInfo extracts commit metadata from the header line
func parseCommitInfo(line string) (commitInfo, error) {
	parts := strings.SplitN(line, "\x00", 5)
	if len(parts) != 5 {
		return commitInfo{}, fmt.Errorf("invalid commit format: %s", line)
	}

	timestamp, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return commitInfo{}, fmt.Errorf("invalid timestamp: %w", err)
	}

	info := commitInfo{
		SHA:         parts[0],
		Timestamp:   timestamp,
		Author:      parts[2],
		AuthorEmail: parts[3],
		Message:     parts[4],
	}

	return info, nil
}

// parseDiff extracts bead events from a diff section
func (e *Extractor) parseDiff(diffData []byte, info commitInfo, filterBeadID string) []BeadEvent {
	var events []BeadEvent

	// Track old and new bead states for status change detection
	oldBeads := make(map[string]beadSnapshot)
	newBeads := make(map[string]beadSnapshot)
	seenBeads := make(map[string]bool)

	scanner := bufio.NewScanner(bytes.NewReader(diffData))
	// Increase buffer for large diffs
	const maxCapacity = 1024 * 1024 * 10 // 10MB
	// Start with 64KB buffer, grow up to maxCapacity
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and diff metadata lines that start with:
		// '@' = hunk headers (@@)
		// 'd' = diff --git
		// 'i' = index
		// 'n' = new file mode
		// We only care about lines starting with +/- which are actual changes.
		if len(line) == 0 || line[0] == '@' || line[0] == 'd' || line[0] == 'i' || line[0] == 'n' {
			continue
		}

		// Check for removed lines (old state) - JSON starts with {
		if strings.HasPrefix(line, "-{") {
			jsonStr := strings.TrimPrefix(line, "-")
			if snap, ok := parseBeadJSON(jsonStr); ok {
				if filterBeadID == "" || snap.ID == filterBeadID {
					oldBeads[snap.ID] = snap
					seenBeads[snap.ID] = true
				}
			}
			continue
		}

		// Check for added lines (new state) - JSON starts with {
		if strings.HasPrefix(line, "+{") {
			jsonStr := strings.TrimPrefix(line, "+")
			if snap, ok := parseBeadJSON(jsonStr); ok {
				if filterBeadID == "" || snap.ID == filterBeadID {
					newBeads[snap.ID] = snap
					seenBeads[snap.ID] = true
				}
			}
			continue
		}
	}

	// Generate events by comparing old and new states
	for beadID := range seenBeads {
		oldSnap, hadOld := oldBeads[beadID]
		newSnap, hasNew := newBeads[beadID]

		event := BeadEvent{
			BeadID:      beadID,
			Timestamp:   info.Timestamp,
			CommitSHA:   info.SHA,
			CommitMsg:   info.Message,
			Author:      info.Author,
			AuthorEmail: info.AuthorEmail,
		}

		if !hadOld && hasNew {
			// New bead created
			event.EventType = EventCreated
			events = append(events, event)
		} else if hadOld && hasNew {
			// Check for status change
			if oldSnap.Status != newSnap.Status {
				event.EventType = determineStatusEvent(oldSnap.Status, newSnap.Status)
				events = append(events, event)
			} else {
				// Other modification (title, etc.)
				event.EventType = EventModified
				events = append(events, event)
			}
		}
		// Note: We don't track deletions (hadOld && !hasNew) as they're not in our EventType
	}

	return events
}

// parseBeadJSON extracts minimal bead info from a JSON line
func parseBeadJSON(jsonStr string) (beadSnapshot, bool) {
	var partial struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Title  string `json:"title"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &partial); err != nil {
		return beadSnapshot{}, false
	}

	if partial.ID == "" {
		return beadSnapshot{}, false
	}

	return beadSnapshot{
		ID:     partial.ID,
		Status: partial.Status,
		Title:  partial.Title,
	}, true
}

// determineStatusEvent determines the appropriate event type for a status transition
func determineStatusEvent(oldStatus, newStatus string) EventType {
	switch newStatus {
	case "in_progress":
		return EventClaimed
	case "closed":
		return EventClosed
	case "open":
		if oldStatus == "closed" {
			return EventReopened
		}
		return EventModified
	default:
		return EventModified
	}
}

// reverseEvents reverses a slice of events in place
func reverseEvents(events []BeadEvent) {
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
}

// ExtractForBead extracts all events for a specific bead
func (e *Extractor) ExtractForBead(beadID string, opts ExtractOptions) ([]BeadEvent, error) {
	opts.BeadID = beadID
	return e.Extract(opts)
}

// GetBeadMilestones returns the key lifecycle milestones for a bead
func GetBeadMilestones(events []BeadEvent) BeadMilestones {
	var milestones BeadMilestones

	for i := range events {
		event := &events[i]
		switch event.EventType {
		case EventCreated:
			if milestones.Created == nil {
				milestones.Created = event
			}
		case EventClaimed:
			if milestones.Claimed == nil {
				milestones.Claimed = event
			}
		case EventClosed:
			milestones.Closed = event // Keep latest
		case EventReopened:
			milestones.Reopened = event // Keep latest
		}
	}

	return milestones
}

// CalculateCycleTime computes cycle time metrics from milestones
func CalculateCycleTime(milestones BeadMilestones) *CycleTime {
	if milestones.Closed == nil {
		return nil
	}

	ct := &CycleTime{}

	if milestones.Claimed != nil {
		d := milestones.Closed.Timestamp.Sub(milestones.Claimed.Timestamp)
		ct.ClaimToClose = &d
	}

	if milestones.Created != nil {
		d := milestones.Closed.Timestamp.Sub(milestones.Created.Timestamp)
		ct.CreateToClose = &d

		if milestones.Claimed != nil {
			d := milestones.Claimed.Timestamp.Sub(milestones.Created.Timestamp)
			ct.CreateToClaim = &d
		}
	}

	return ct
}
