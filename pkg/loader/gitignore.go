// Package loader provides issue loading and file discovery utilities.
// This file handles automatic .gitignore management for the .bv directory.
package loader

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// EnsureBVInGitignore ensures that .bv/ is listed in the project's .gitignore file.
// This prevents bv-specific files (semantic search index, baselines, drift config, etc.)
// from polluting the git repository.
//
// The function is idempotent and safe to call multiple times.
// It will:
//   - Create .gitignore if it doesn't exist
//   - Add ".bv/" if it's not already present (checks for .bv, .bv/, .bv/*, etc.)
//   - Preserve existing file content and formatting
//
// Returns nil on success, or an error if the file cannot be read/written.
func EnsureBVInGitignore(projectDir string) error {
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	gitignorePath := filepath.Join(projectDir, ".gitignore")

	// Check if .bv is already in .gitignore
	alreadyPresent, err := isBVInGitignore(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if alreadyPresent {
		return nil
	}

	// Append .bv/ to .gitignore
	return appendToGitignore(gitignorePath, ".bv/")
}

// isBVInGitignore checks if .bv is already covered by the .gitignore file.
// It returns true if any of these patterns are found:
//   - .bv
//   - .bv/
//   - .bv/*
//   - .bv/**
func isBVInGitignore(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Check for patterns that would cover .bv/
		if matchesBVPattern(line) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// matchesBVPattern checks if a gitignore line covers the .bv directory.
func matchesBVPattern(line string) bool {
	// Normalize: remove leading/trailing slashes for comparison
	normalized := strings.TrimPrefix(line, "/")

	// Exact matches for .bv directory
	patterns := []string{
		".bv",
		".bv/",
		".bv/*",
		".bv/**",
		".bv/**/*",
	}

	for _, pattern := range patterns {
		if normalized == pattern {
			return true
		}
	}

	return false
}

// appendToGitignore appends a pattern to the .gitignore file.
// It creates the file if it doesn't exist.
// It ensures there's a newline before the pattern if the file doesn't end with one.
func appendToGitignore(path string, pattern string) error {
	// Check if file exists and its current content
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Open file for appending (creates if not exists)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Build the content to append based on whether file has existing content
	var toWrite string
	if len(content) == 0 {
		// New file: just add comment and pattern (no leading blank line)
		toWrite = "# bv (beads viewer) local config and caches\n" + pattern + "\n"
	} else {
		// Existing file: ensure proper separation
		if content[len(content)-1] != '\n' {
			// File doesn't end with newline, add one first
			toWrite = "\n"
		}
		// Add blank line separator, comment, and pattern
		toWrite += "\n# bv (beads viewer) local config and caches\n" + pattern + "\n"
	}

	if _, err := file.WriteString(toWrite); err != nil {
		return err
	}

	return nil
}
