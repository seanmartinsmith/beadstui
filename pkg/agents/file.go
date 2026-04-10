package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// AppendBlurbToFile appends the agent blurb to the specified file.
// Uses atomic write to prevent corruption.
func AppendBlurbToFile(filePath string) error {
	// Read existing content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Append blurb using the string function
	newContent := AppendBlurb(string(content))

	// Write atomically
	if err := atomicWrite(filePath, []byte(newContent)); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// UpdateBlurbInFile replaces an existing blurb with the current version.
// Uses atomic write to prevent corruption.
func UpdateBlurbInFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	newContent := UpdateBlurb(string(content))

	if err := atomicWrite(filePath, []byte(newContent)); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// RemoveBlurbFromFile removes the agent blurb from the specified file.
// Uses atomic write to prevent corruption.
func RemoveBlurbFromFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	newContent := RemoveBlurb(string(content))

	if err := atomicWrite(filePath, []byte(newContent)); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// CreateAgentFile creates a new AGENTS.md file with the blurb content.
// The file is created with standard permissions (0644).
func CreateAgentFile(filePath string) error {
	// Create with just the blurb (no existing content)
	content := "# AI Agent Instructions\n\n" + AgentBlurb + "\n"

	// Write atomically
	if err := atomicWrite(filePath, []byte(content)); err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	return nil
}

// VerifyBlurbPresent checks that the blurb was successfully added to a file.
func VerifyBlurbPresent(filePath string) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	return ContainsBlurb(string(content)), nil
}

// atomicWrite writes content to a file atomically using a temp file and rename.
// This prevents partial writes from corrupting the original file.
func atomicWrite(filePath string, content []byte) error {
	// Get file info to preserve permissions
	var mode os.FileMode = 0644
	if info, err := os.Stat(filePath); err == nil {
		mode = info.Mode()
	}

	// Create temp file in same directory (required for atomic rename)
	dir := filepath.Dir(filePath)
	tmp, err := os.CreateTemp(dir, ".bv-atomic-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Cleanup temp file on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write content
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	// Ensure data is flushed to disk
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		// Windows does not allow renaming over an existing file.
		if runtime.GOOS == "windows" {
			if rmErr := os.Remove(filePath); rmErr == nil {
				if err2 := os.Rename(tmpPath, filePath); err2 == nil {
					success = true
					return nil
				} else {
					return fmt.Errorf("rename temp file: %w", err2)
				}
			}
		}
		return fmt.Errorf("rename temp file: %w", err)
	}

	success = true
	return nil
}

// EnsureBlurb ensures the blurb is present in an agent file.
// If the file exists without blurb, appends it.
// If the file has an old version, updates it.
// If the file doesn't exist, creates it.
func EnsureBlurb(workDir string) error {
	detection := DetectAgentFile(workDir)

	if !detection.Found() {
		// No agent file exists - create one
		filePath := GetPreferredAgentFilePath(workDir)
		return CreateAgentFile(filePath)
	}

	if detection.NeedsBlurb() {
		// File exists but no blurb - append
		return AppendBlurbToFile(detection.FilePath)
	}

	if detection.NeedsUpgrade() {
		// File has old blurb - update
		return UpdateBlurbInFile(detection.FilePath)
	}

	// Already has current blurb
	return nil
}
