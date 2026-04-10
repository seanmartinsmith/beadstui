package loader

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// BeadsDirEnvVar is the name of the environment variable for custom beads directory
const BeadsDirEnvVar = "BEADS_DIR"

// PreferredJSONLNames defines the priority order for looking up beads data files.
// Priority order matches bd's canonical naming (beads.jsonl) to ensure bv watches
// the same file that bd writes to in stealth/direct mode. Fixes bv-96.
var PreferredJSONLNames = []string{"beads.jsonl", "issues.jsonl", "beads.base.jsonl"}

// GetBeadsDir returns the beads directory path, respecting BEADS_DIR env var.
// If BEADS_DIR is set, it is used directly.
// Otherwise, falls back to .beads in the given repoPath (or cwd if empty).
// For git worktrees, this automatically detects and uses the main repository root.
func GetBeadsDir(repoPath string) (string, error) {
	// Check BEADS_DIR environment variable first
	if envDir := os.Getenv(BeadsDirEnvVar); envDir != "" {
		return envDir, nil
	}

	// Fall back to .beads in repo path
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
	}

	// Check for .beads in the given path first
	beadsDir := filepath.Join(repoPath, ".beads")
	if _, err := os.Stat(beadsDir); err == nil {
		return beadsDir, nil
	}

	// If not found, check if we're in a git worktree and look in the main repo
	mainRepoRoot, err := getMainRepoRoot(repoPath)
	if err == nil && mainRepoRoot != "" && mainRepoRoot != repoPath {
		mainBeadsDir := filepath.Join(mainRepoRoot, ".beads")
		if _, err := os.Stat(mainBeadsDir); err == nil {
			return mainBeadsDir, nil
		}
	}

	// Return the original path even if .beads doesn't exist
	// (caller will handle the error)
	return beadsDir, nil
}

// getMainRepoRoot returns the root directory of the main git repository.
// For regular repos, this returns the repo root.
// For worktrees, this returns the main repository root (not the worktree root).
func getMainRepoRoot(repoPath string) (string, error) {
	// First, check if we're in a git repository at all
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = repoPath
	topLevelOut, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	worktreeRoot := strings.TrimSpace(string(topLevelOut))

	// Check if this is a worktree by looking at the git-common-dir
	// For regular repos: git-common-dir == git-dir
	// For worktrees: git-common-dir points to main repo's .git
	cmd = exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = repoPath
	commonDirOut, err := cmd.Output()
	if err != nil {
		// Fallback: not a worktree or old git version
		return worktreeRoot, nil
	}
	commonDir := strings.TrimSpace(string(commonDirOut))

	cmd = exec.Command("git", "rev-parse", "--path-format=absolute", "--git-dir")
	cmd.Dir = repoPath
	gitDirOut, err := cmd.Output()
	if err != nil {
		return worktreeRoot, nil
	}
	gitDir := strings.TrimSpace(string(gitDirOut))

	// If git-common-dir == git-dir, we're in a regular repo
	if commonDir == gitDir {
		return worktreeRoot, nil
	}

	// We're in a worktree. The main repo root is the parent of git-common-dir.
	// git-common-dir typically points to /path/to/main-repo/.git
	mainRepoRoot := filepath.Dir(commonDir)

	return mainRepoRoot, nil
}

// FindJSONLPath locates the beads JSONL file in the given directory.
// Prefers beads.jsonl (canonical per bd) over issues.jsonl (legacy) to match
// the file that bd writes to in stealth/direct mode. Fixes bv-96.
// Skips backup files and merge artifacts.
func FindJSONLPath(beadsDir string) (string, error) {
	return FindJSONLPathWithWarnings(beadsDir, nil)
}

// FindJSONLPathWithWarnings is like FindJSONLPath but optionally reports warnings
// about detected merge artifacts via the provided callback.
func FindJSONLPathWithWarnings(beadsDir string, warnFunc func(msg string)) (string, error) {
	entries, err := os.ReadDir(beadsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read beads directory: %w", err)
	}

	var candidates []string
	var mergeArtifacts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()

		// Must be a .jsonl file
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Skip backups, merge artifacts, and deletion manifests
		if strings.Contains(name, ".backup") ||
			strings.Contains(name, ".orig") ||
			strings.Contains(name, ".merge") ||
			name == "deletions.jsonl" {
			continue
		}

		// Skip git merge conflict artifacts (beads.left.jsonl, beads.right.jsonl)
		// These are OURS/THEIRS sides during a merge conflict
		if strings.HasPrefix(name, "beads.left") || strings.HasPrefix(name, "beads.right") {
			mergeArtifacts = append(mergeArtifacts, name)
			continue
		}

		candidates = append(candidates, name)
	}

	// Warn about detected merge artifacts
	if len(mergeArtifacts) > 0 && warnFunc != nil {
		warnFunc(fmt.Sprintf("Merge artifact files detected: %s. Consider running 'br clean' to remove them.",
			strings.Join(mergeArtifacts, ", ")))
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no beads JSONL file found in %s", beadsDir)
	}

	// Priority order for beads files - matches bd's canonical naming (bv-96):
	// 1. beads.jsonl (canonical - what bd writes to in stealth/direct mode)
	// 2. issues.jsonl (legacy from steveyegge/beads pre-commit hook)
	// 3. beads.base.jsonl (fallback, may be present during merge resolution)
	// 4. First candidate
	preferredNames := PreferredJSONLNames

	for _, preferred := range preferredNames {
		for _, name := range candidates {
			if name == preferred {
				path := filepath.Join(beadsDir, name)
				// Check if file has content (skip empty files)
				if info, err := os.Stat(path); err == nil && info.Size() > 0 {
					return path, nil
				}
			}
		}
	}

	// Fall back to first non-empty candidate
	for _, name := range candidates {
		path := filepath.Join(beadsDir, name)
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path, nil
		}
	}

	// Last resort: return first candidate even if empty
	return filepath.Join(beadsDir, candidates[0]), nil
}

// LoadIssues reads issues from the beads directory.
// Respects BEADS_DIR environment variable, otherwise uses .beads in repoPath.
// Automatically finds the correct JSONL file (issues.jsonl preferred, beads.jsonl fallback).
func LoadIssues(repoPath string) ([]model.Issue, error) {
	beadsDir, err := GetBeadsDir(repoPath)
	if err != nil {
		return nil, err
	}

	jsonlPath, err := FindJSONLPath(beadsDir)
	if err != nil {
		return nil, err
	}

	return LoadIssuesFromFile(jsonlPath)
}

// DefaultMaxBufferSize is the default buffer size for the scanner (10MB).
const DefaultMaxBufferSize = 1024 * 1024 * 10

// ParseOptions configures the behavior of ParseIssues.
type ParseOptions struct {
	// WarningHandler is called with warning messages (e.g., malformed JSON).
	// If nil, warnings are printed to os.Stderr.
	WarningHandler func(string)

	// BufferSize sets the maximum line size (in bytes) to read at once.
	// Lines longer than this are skipped with a warning.
	// If 0, uses DefaultMaxBufferSize (10MB).
	BufferSize int

	// IssueFilter optionally filters parsed issues. Return true to include.
	// When nil, all valid issues are included.
	IssueFilter func(*model.Issue) bool
}

// LoadIssuesFromFileWithOptions reads issues from a file with custom options.
func LoadIssuesFromFileWithOptions(path string, opts ParseOptions) ([]model.Issue, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no beads issues found at %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open issues file: %w", err)
	}
	defer file.Close()

	return ParseIssuesWithOptions(file, opts)
}

// LoadIssuesFromFileWithOptionsPooled reads issues from a file with pooling enabled.
// The caller must return pooled issues via ReturnIssuePtrsToPool when no longer needed.
func LoadIssuesFromFileWithOptionsPooled(path string, opts ParseOptions) (PooledIssues, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return PooledIssues{}, fmt.Errorf("no beads issues found at %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return PooledIssues{}, fmt.Errorf("failed to open issues file: %w", err)
	}
	defer file.Close()

	return ParseIssuesWithOptionsPooled(file, opts)
}

// LoadIssuesFromFile reads issues directly from a specific JSONL file path.
func LoadIssuesFromFile(path string) ([]model.Issue, error) {
	return LoadIssuesFromFileWithOptions(path, ParseOptions{})
}

// LoadIssuesFromFilePooled reads issues directly from a JSONL file path with pooling enabled.
func LoadIssuesFromFilePooled(path string) (PooledIssues, error) {
	return LoadIssuesFromFileWithOptionsPooled(path, ParseOptions{})
}

// ParseIssues parses JSONL content from a reader into issues.
// Handles UTF-8 BOM stripping, large lines, and validation.
func ParseIssues(r io.Reader) ([]model.Issue, error) {
	return ParseIssuesWithOptions(r, ParseOptions{})
}

// ParseIssuesWithOptions parses JSONL content with custom options.
func ParseIssuesWithOptions(r io.Reader, opts ParseOptions) ([]model.Issue, error) {
	issues, _, err := parseIssuesWithOptions(r, opts, false)
	return issues, err
}

// ParseIssuesWithOptionsPooled parses JSONL content with pooling enabled.
// The caller must return pooled issues via ReturnIssuePtrsToPool when no longer needed.
func ParseIssuesWithOptionsPooled(r io.Reader, opts ParseOptions) (PooledIssues, error) {
	issues, poolRefs, err := parseIssuesWithOptions(r, opts, true)
	if err != nil {
		return PooledIssues{}, err
	}
	return PooledIssues{Issues: issues, PoolRefs: poolRefs}, nil
}

func parseIssuesWithOptions(r io.Reader, opts ParseOptions, usePool bool) ([]model.Issue, []*model.Issue, error) {
	var issues []model.Issue
	var poolRefs []*model.Issue
	if f, ok := r.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			// Heuristic: average issue line ~2KB. Prefer conservative underestimation to
			// avoid large over-allocations for big files.
			const avgIssueBytes = 2 * 1024
			const minCap = 64
			const maxCap = 200_000

			est := int(info.Size() / avgIssueBytes)
			if est < minCap && info.Size() > 0 {
				est = minCap
			}
			if est > maxCap {
				est = maxCap
			}
			if est > 0 {
				issues = make([]model.Issue, 0, est)
				if usePool {
					poolRefs = make([]*model.Issue, 0, est)
				}
			}
		}
	}

	// Determine buffer size
	maxCapacity := opts.BufferSize
	if maxCapacity <= 0 {
		maxCapacity = DefaultMaxBufferSize
	}

	reader := bufio.NewReaderSize(r, maxCapacity)

	// Default warning handler prints to stderr (suppressed in robot mode).
	warn := opts.WarningHandler
	if warn == nil {
		if os.Getenv("BV_ROBOT") == "1" {
			warn = func(string) {}
		} else {
			warn = func(msg string) {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
			}
		}
	}

	lineNum := 0
	for {
		lineNum++
		// ReadLine returns a single line, not including the end-of-line bytes.
		// If the line was too long for the buffer then isPrefix is set and the
		// beginning of the line is returned.
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			if usePool {
				ReturnIssuePtrsToPool(poolRefs)
			}
			return nil, nil, fmt.Errorf("error reading issues stream at line %d: %w", lineNum, err)
		}

		if isPrefix {
			// Line too long. Discard the rest of the line.
			warn(fmt.Sprintf("skipping line %d: line too long (exceeds %d bytes)", lineNum, maxCapacity))
			for isPrefix {
				_, isPrefix, err = reader.ReadLine()
				if err != nil && err != io.EOF {
					if usePool {
						ReturnIssuePtrsToPool(poolRefs)
					}
					return nil, nil, fmt.Errorf("error skipping long line at line %d: %w", lineNum, err)
				}
				if err == io.EOF {
					break
				}
			}
			continue
		}

		if len(line) == 0 {
			continue
		}

		// Strip UTF-8 BOM if present on the first line
		if lineNum == 1 {
			line = stripBOM(line)
		}

		if usePool {
			issue := GetIssue()
			if err := json.Unmarshal(line, issue); err != nil {
				PutIssue(issue)
				// Skip malformed lines but warn
				warn(fmt.Sprintf("skipping malformed JSON on line %d: %v", lineNum, err))
				continue
			}

			issue.Status = normalizeIssueStatus(issue.Status)

			// Validate issue
			if err := issue.Validate(); err != nil {
				PutIssue(issue)
				// Skip invalid issues
				warn(fmt.Sprintf("skipping invalid issue on line %d: %v", lineNum, err))
				continue
			}

			if opts.IssueFilter != nil && !opts.IssueFilter(issue) {
				PutIssue(issue)
				continue
			}

			issues = append(issues, *issue)
			poolRefs = append(poolRefs, issue)
		} else {
			var issue model.Issue
			if err := json.Unmarshal(line, &issue); err != nil {
				// Skip malformed lines but warn
				warn(fmt.Sprintf("skipping malformed JSON on line %d: %v", lineNum, err))
				continue
			}

			issue.Status = normalizeIssueStatus(issue.Status)

			// Validate issue
			if err := issue.Validate(); err != nil {
				// Skip invalid issues
				warn(fmt.Sprintf("skipping invalid issue on line %d: %v", lineNum, err))
				continue
			}

			if opts.IssueFilter != nil && !opts.IssueFilter(&issue) {
				continue
			}

			issues = append(issues, issue)
		}
	}

	return issues, poolRefs, nil
}

// stripBOM removes the UTF-8 Byte Order Mark if present
func stripBOM(b []byte) []byte {
	if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		return b[3:]
	}
	return b
}

func normalizeIssueStatus(status model.Status) model.Status {
	trimmed := strings.TrimSpace(string(status))
	if trimmed == "" {
		return status
	}
	return model.Status(strings.ToLower(trimmed))
}
