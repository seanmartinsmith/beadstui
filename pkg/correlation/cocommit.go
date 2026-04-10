// Package correlation provides extraction of co-committed files for bead correlation.
package correlation

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// renamePattern matches git's brace notation for renames: {old => new}
var renamePattern = regexp.MustCompile(`\{[^}]* => ([^}]*)\}`)

// CoCommitExtractor extracts files that were changed in the same commit as bead changes
type CoCommitExtractor struct {
	repoPath string
}

// NewCoCommitExtractor creates a new co-commit extractor
func NewCoCommitExtractor(repoPath string) *CoCommitExtractor {
	return &CoCommitExtractor{repoPath: repoPath}
}

// codeFileExtensions lists file extensions considered "code files"
var codeFileExtensions = map[string]bool{
	".go":    true,
	".py":    true,
	".js":    true,
	".ts":    true,
	".jsx":   true,
	".tsx":   true,
	".rs":    true,
	".java":  true,
	".kt":    true,
	".swift": true,
	".c":     true,
	".cpp":   true,
	".h":     true,
	".hpp":   true,
	".rb":    true,
	".php":   true,
	".cs":    true,
	".scala": true,
	".yaml":  true,
	".yml":   true,
	".json":  true,
	".toml":  true,
	".md":    true,
	".sql":   true,
	".sh":    true,
	".bash":  true,
	".zsh":   true,
}

// excludedPaths lists path prefixes that should be excluded
var excludedPaths = []string{
	".beads/",
	".bv/",
	".git/",
	"node_modules/",
	"vendor/",
	"__pycache__/",
	".venv/",
	"venv/",
	"dist/",
	"build/",
	".next/",
}

// ExtractCoCommittedFiles extracts code files changed in the same commit as a bead event
func (c *CoCommitExtractor) ExtractCoCommittedFiles(event BeadEvent) ([]FileChange, error) {
	// Get file list with status
	files, err := c.getFilesChanged(event.CommitSHA)
	if err != nil {
		return nil, err
	}

	// Get line stats
	stats, err := c.getLineStats(event.CommitSHA)
	if err != nil {
		// Non-fatal: continue without stats
		stats = make(map[string]lineStats)
	}

	// Filter to code files only
	var codeFiles []FileChange
	for _, f := range files {
		if !isCodeFile(f.Path) {
			continue
		}
		if isExcludedPath(f.Path) {
			continue
		}

		// Add line stats if available
		if s, ok := stats[f.Path]; ok {
			f.Insertions = s.insertions
			f.Deletions = s.deletions
		}

		codeFiles = append(codeFiles, f)
	}

	return codeFiles, nil
}

// CreateCorrelatedCommit creates a CorrelatedCommit with confidence scoring
func (c *CoCommitExtractor) CreateCorrelatedCommit(event BeadEvent, files []FileChange) CorrelatedCommit {
	confidence := c.calculateConfidence(event, files)
	reason := c.generateReason(event, files, confidence)

	return CorrelatedCommit{
		BeadID:      event.BeadID,
		SHA:         event.CommitSHA,
		ShortSHA:    shortSHA(event.CommitSHA),
		Message:     event.CommitMsg,
		Author:      event.Author,
		AuthorEmail: event.AuthorEmail,
		Timestamp:   event.Timestamp,
		Files:       files,
		Method:      MethodCoCommitted,
		Confidence:  confidence,
		Reason:      reason,
	}
}

// lineStats holds insertion/deletion counts for a file
type lineStats struct {
	insertions int
	deletions  int
}

// getFilesChanged runs git show --name-status to get changed files
func (c *CoCommitExtractor) getFilesChanged(sha string) ([]FileChange, error) {
	cmd := exec.Command("git", "show", "--name-status", "--format=", sha)
	cmd.Dir = c.repoPath

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show --name-status failed: %w", err)
	}

	var files []FileChange
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Format: "M\tpath/to/file" or "R100\told\tnew"
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		action := parts[0]
		path := parts[1]

		// Handle renames: R100\told\tnew
		if len(parts) == 3 && strings.HasPrefix(action, "R") {
			path = parts[2] // Use new name
			action = "R"
		}

		// Normalize action to single char
		if len(action) > 1 {
			action = string(action[0])
		}

		files = append(files, FileChange{
			Path:   path,
			Action: action,
		})
	}

	return files, scanner.Err()
}

// getLineStats runs git show --numstat to get insertion/deletion counts
func (c *CoCommitExtractor) getLineStats(sha string) (map[string]lineStats, error) {
	cmd := exec.Command("git", "show", "--numstat", "--format=", sha)
	cmd.Dir = c.repoPath

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show --numstat failed: %w", err)
	}

	stats := make(map[string]lineStats)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Format: "42\t10\tpath/to/file" or "-\t-\tbinary/file"
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		insertions := 0
		deletions := 0

		// Binary files show "-" instead of numbers
		if parts[0] != "-" {
			insertions, _ = strconv.Atoi(parts[0])
		}
		if parts[1] != "-" {
			deletions, _ = strconv.Atoi(parts[1])
		}

		// Handle renames: path might be "old => new" format
		path := parts[2]
		if strings.Contains(path, " => ") {
			// Extract new path from "old => new" or "{old => new}" format
			path = extractNewPath(path)
		}

		stats[path] = lineStats{
			insertions: insertions,
			deletions:  deletions,
		}
	}

	return stats, scanner.Err()
}

// extractNewPath handles git's rename notation in numstat output
func extractNewPath(path string) string {
	// Handle "{prefix/}{old => new}{/suffix}" format
	if strings.Contains(path, "{") {
		// Complex case: "pkg/{old => new}/file.go"
		path = renamePattern.ReplaceAllString(path, "$1")
		// Fix potential double slashes if a segment was removed (e.g. "{old => }")
		return strings.ReplaceAll(path, "//", "/")
	}

	// Simple case: "old => new"
	if idx := strings.Index(path, " => "); idx != -1 {
		return path[idx+4:]
	}

	return path
}

// calculateConfidence computes the confidence score for a co-commit correlation
func (c *CoCommitExtractor) calculateConfidence(event BeadEvent, files []FileChange) float64 {
	// Base confidence for co-committed files
	confidence := 0.95

	// Bonus: commit message mentions bead ID
	if containsBeadID(event.CommitMsg, event.BeadID) {
		confidence += 0.04
	}

	// Penalty: shotgun commit (>20 files)
	if len(files) > 20 {
		confidence -= 0.10
	}

	// Penalty: only test files
	if allTestFiles(files) {
		confidence -= 0.05
	}

	// Clamp to [0, 1]
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// generateReason creates a human-readable explanation for the correlation
func (c *CoCommitExtractor) generateReason(event BeadEvent, files []FileChange, confidence float64) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Co-committed with bead status change to %s", event.EventType))

	if containsBeadID(event.CommitMsg, event.BeadID) {
		parts = append(parts, "commit message references bead ID")
	}

	if len(files) > 20 {
		parts = append(parts, fmt.Sprintf("large commit (%d files)", len(files)))
	}

	if allTestFiles(files) {
		parts = append(parts, "contains only test files")
	}

	return strings.Join(parts, "; ")
}

// isCodeFile checks if a file path is a code file based on extension
func isCodeFile(path string) bool {
	// Handle git quoting (e.g. "path/with spaces.go")
	if len(path) > 2 && path[0] == '"' && path[len(path)-1] == '"' {
		// Basic unquote: strip quotes.
		// Git might use C-style escapes (e.g. \t, \n, \"), but for extension checking
		// simply stripping the surrounding quotes handles the common case of spaces.
		// For complex escapes, we accept that filepath.Ext might be imperfect,
		// but this covers 99% of "filename with space.go" cases.
		path = path[1 : len(path)-1]
	}

	ext := strings.ToLower(filepath.Ext(path))
	return codeFileExtensions[ext]
}

// isExcludedPath checks if a path should be excluded
func isExcludedPath(path string) bool {
	// Check for direct prefix (fast path for root dirs)
	for _, prefix := range excludedPaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Check for nested directories (e.g. src/node_modules/...)
	// We look for "/dirname/" in the path
	for _, prefix := range excludedPaths {
		// Only check directory exclusions (ending in /)
		if strings.HasSuffix(prefix, "/") {
			// Check for "/prefix" anywhere in path
			// We prepend / to ensure we match a directory boundary
			if strings.Contains(path, "/"+prefix) {
				return true
			}
		}
	}
	return false
}

// containsBeadID checks if text contains the bead ID
func containsBeadID(text, beadID string) bool {
	if beadID == "" {
		return false
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(beadID))
}

// allTestFiles returns true if all files are test files
func allTestFiles(files []FileChange) bool {
	if len(files) == 0 {
		return false
	}

	testPatterns := []string{"_test.go", ".test.js", ".test.ts", ".spec.js", ".spec.ts", "_test.py", "test_"}

	for _, f := range files {
		isTest := false
		lowerPath := strings.ToLower(f.Path)
		for _, pattern := range testPatterns {
			if strings.Contains(lowerPath, pattern) {
				isTest = true
				break
			}
		}
		if !isTest {
			return false
		}
	}
	return true
}

// shortSHA returns the first 7 characters of a SHA
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// ExtractAllCoCommits extracts co-committed files for all events with status changes
func (c *CoCommitExtractor) ExtractAllCoCommits(events []BeadEvent) ([]CorrelatedCommit, error) {
	var commits []CorrelatedCommit
	fileCache := make(map[string][]FileChange) // Cache file lookups by SHA

	for _, event := range events {
		// Only process status change events
		if event.EventType != EventClaimed && event.EventType != EventClosed {
			continue
		}

		// Use cached files if available, otherwise fetch from git
		files, cached := fileCache[event.CommitSHA]
		if !cached {
			var err error
			files, err = c.ExtractCoCommittedFiles(event)
			if err != nil {
				// Non-fatal: skip this commit
				continue
			}
			fileCache[event.CommitSHA] = files
		}

		// Only create correlation if there are code files
		if len(files) == 0 {
			continue
		}

		commit := c.CreateCorrelatedCommit(event, files)
		commits = append(commits, commit)
	}

	return commits, nil
}
