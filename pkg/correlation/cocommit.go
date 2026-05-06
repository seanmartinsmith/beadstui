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
	".bt/",
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

	return mergeAndFilterCodeFiles(files, stats), nil
}

// mergeAndFilterCodeFiles attaches line stats to file changes and filters to code files.
// Used by both the per-event path (ExtractCoCommittedFiles) and the batched path
// (prefetchCoCommittedFiles); both must produce byte-identical output for a given SHA.
func mergeAndFilterCodeFiles(files []FileChange, stats map[string]lineStats) []FileChange {
	var codeFiles []FileChange
	for _, f := range files {
		if !isCodeFile(f.Path) {
			continue
		}
		if isExcludedPath(f.Path) {
			continue
		}

		if s, ok := stats[f.Path]; ok {
			f.Insertions = s.insertions
			f.Deletions = s.deletions
		}

		codeFiles = append(codeFiles, f)
	}
	return codeFiles
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

// ExtractAllCoCommits extracts co-committed files for all events with status changes.
//
// Performance: rather than spawning two `git show` subprocesses per status-change
// event (the historical per-event path), all unique SHAs are pre-fetched in two
// batched `git log --no-walk` invocations -- one for --name-status, one for
// --numstat. This collapses O(N) subprocess spawns to O(1) regardless of event
// count, which matters on Windows where each spawn is ~50-200ms.
//
// The batched path is byte-identical to the per-event path: same FileChange
// list, same actions, same insertion/deletion counts, same rename normalization.
// On batch failure the function falls back to the per-event path so a single
// invalid SHA in the input cannot tank the whole report.
func (c *CoCommitExtractor) ExtractAllCoCommits(events []BeadEvent) ([]CorrelatedCommit, error) {
	var commits []CorrelatedCommit

	// Collect status-change SHAs up front so we can pre-fetch them all together.
	var statusSHAs []string
	for _, event := range events {
		if event.EventType != EventClaimed && event.EventType != EventClosed {
			continue
		}
		if event.CommitSHA == "" {
			continue
		}
		statusSHAs = append(statusSHAs, event.CommitSHA)
	}

	// Pre-fetch in O(1) git invocations. On batch error, leave fileCache empty
	// and fall through to the per-event path below.
	fileCache, _ := c.prefetchCoCommittedFiles(statusSHAs)
	if fileCache == nil {
		fileCache = make(map[string][]FileChange)
	}

	for _, event := range events {
		// Only process status change events
		if event.EventType != EventClaimed && event.EventType != EventClosed {
			continue
		}

		// Use prefetched files if available, otherwise fetch from git per-SHA.
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

// batchSHAChunkSize bounds the number of SHAs passed in a single git log
// invocation. Each SHA + separator is ~41 bytes; Windows CreateProcess command
// line is ~32KB; 200 SHAs gives ~8KB worth of args with comfortable headroom
// for the rest of the command line.
const batchSHAChunkSize = 200

// prefetchCoCommittedFiles fetches name-status and numstat for all given SHAs
// in O(1) git subprocess calls (modulo chunking) and returns a SHA-keyed map
// of post-filter FileChange lists. The output for any given SHA is byte-identical
// to what ExtractCoCommittedFiles would return for that SHA.
//
// Returns nil with the underlying error if either batched git call fails;
// callers should fall back to the per-event path on nil.
func (c *CoCommitExtractor) prefetchCoCommittedFiles(shas []string) (map[string][]FileChange, error) {
	if len(shas) == 0 {
		return map[string][]FileChange{}, nil
	}

	// Dedupe to avoid passing the same SHA more than once.
	seen := make(map[string]bool, len(shas))
	unique := make([]string, 0, len(shas))
	for _, s := range shas {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		unique = append(unique, s)
	}
	if len(unique) == 0 {
		return map[string][]FileChange{}, nil
	}

	allFiles := make(map[string][]FileChange, len(unique))
	allStats := make(map[string]map[string]lineStats, len(unique))

	for start := 0; start < len(unique); start += batchSHAChunkSize {
		end := start + batchSHAChunkSize
		if end > len(unique) {
			end = len(unique)
		}
		chunk := unique[start:end]

		nameStatusOut, err := c.runBatchGitLog(chunk, "--name-status")
		if err != nil {
			return nil, fmt.Errorf("batch git log --name-status: %w", err)
		}
		numStatOut, err := c.runBatchGitLog(chunk, "--numstat")
		if err != nil {
			return nil, fmt.Errorf("batch git log --numstat: %w", err)
		}

		parseBatchNameStatus(nameStatusOut, allFiles)
		parseBatchNumStat(numStatOut, allStats)
	}

	out := make(map[string][]FileChange, len(unique))
	for _, sha := range unique {
		stats := allStats[sha]
		if stats == nil {
			stats = map[string]lineStats{}
		}
		out[sha] = mergeAndFilterCodeFiles(allFiles[sha], stats)
	}
	return out, nil
}

// runBatchGitLog runs `git log --no-walk <mode> --format=%H <shas>...` and
// returns the raw stdout. mode is one of "--name-status" or "--numstat".
func (c *CoCommitExtractor) runBatchGitLog(shas []string, mode string) ([]byte, error) {
	args := make([]string, 0, len(shas)+4)
	args = append(args, "log", "--no-walk", mode, "--format=%H")
	args = append(args, shas...)

	cmd := exec.Command("git", args...)
	cmd.Dir = c.repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// parseBatchNameStatus parses streamed `git log --no-walk --name-status --format=%H`
// output into a SHA-keyed map of FileChange lists (without line stats). Output
// shape mirrors what getFilesChanged produces for a single SHA.
//
// Stream format:
//
//	<sha line: 40 hex chars on its own line>
//	<blank>
//	<name-status line: ACTION\tpath  or  R<score>\told\tnew>
//	...
//	<sha line>
//	...
func parseBatchNameStatus(data []byte, out map[string][]FileChange) {
	var currentSHA string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Allow up to 10MB lines for very wide diffs.
	const maxLine = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLine)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if isCommitSHALine(line) {
			currentSHA = line
			if _, ok := out[currentSHA]; !ok {
				out[currentSHA] = nil
			}
			continue
		}
		if currentSHA == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		action := parts[0]
		path := parts[1]

		if len(parts) == 3 && strings.HasPrefix(action, "R") {
			path = parts[2]
			action = "R"
		}

		if len(action) > 1 {
			action = string(action[0])
		}

		out[currentSHA] = append(out[currentSHA], FileChange{
			Path:   path,
			Action: action,
		})
	}
}

// parseBatchNumStat parses streamed `git log --no-walk --numstat --format=%H`
// output into a SHA-keyed map of path-keyed lineStats. Mirrors getLineStats.
func parseBatchNumStat(data []byte, out map[string]map[string]lineStats) {
	var currentSHA string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	const maxLine = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLine)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if isCommitSHALine(line) {
			currentSHA = line
			if _, ok := out[currentSHA]; !ok {
				out[currentSHA] = make(map[string]lineStats)
			}
			continue
		}
		if currentSHA == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		insertions := 0
		deletions := 0
		if parts[0] != "-" {
			insertions, _ = strconv.Atoi(parts[0])
		}
		if parts[1] != "-" {
			deletions, _ = strconv.Atoi(parts[1])
		}

		path := parts[2]
		if strings.Contains(path, " => ") {
			path = extractNewPath(path)
		}

		out[currentSHA][path] = lineStats{
			insertions: insertions,
			deletions:  deletions,
		}
	}
}

// isCommitSHALine reports whether line is a 40-char hex SHA on its own.
// Used to distinguish commit-header lines from name-status / numstat content
// in batched git log output.
func isCommitSHALine(line string) bool {
	if len(line) != 40 {
		return false
	}
	for i := 0; i < 40; i++ {
		c := line[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
