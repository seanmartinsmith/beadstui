package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchesBVPattern(t *testing.T) {
	tests := []struct {
		line    string
		matches bool
	}{
		// Should match
		{".bv", true},
		{".bv/", true},
		{".bv/*", true},
		{".bv/**", true},
		{".bv/**/*", true},
		{"/.bv", true}, // Leading slash should be normalized
		{"/.bv/", true},

		// Should not match
		{"", false},
		{"#.bv", false}, // Comment
		{".bv2", false},
		{".bvx", false},
		{"bv/", false},
		{".beads/", false},
		{"node_modules/", false},
		{".bv-backup", false},
		{"*.bv", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := matchesBVPattern(tt.line)
			if got != tt.matches {
				t.Errorf("matchesBVPattern(%q) = %v, want %v", tt.line, got, tt.matches)
			}
		})
	}
}

func TestIsBVInGitignore(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty file",
			content:  "",
			expected: false,
		},
		{
			name:     "has .bv",
			content:  "node_modules/\n.bv\n*.log\n",
			expected: true,
		},
		{
			name:     "has .bv/",
			content:  "node_modules/\n.bv/\n*.log\n",
			expected: true,
		},
		{
			name:     "has .bv/*",
			content:  ".bv/*\n",
			expected: true,
		},
		{
			name:     "has /.bv/",
			content:  "/.bv/\n",
			expected: true,
		},
		{
			name:     "commented out",
			content:  "# .bv/\n",
			expected: false,
		},
		{
			name:     "different pattern",
			content:  ".beads/\nnode_modules/\n",
			expected: false,
		},
		{
			name:     "similar but not matching",
			content:  ".bv2/\n.bvx\nbv/\n",
			expected: false,
		},
		{
			name:     "with whitespace",
			content:  "  .bv/  \n",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gitignorePath := filepath.Join(tmpDir, ".gitignore")

			if err := os.WriteFile(gitignorePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			got, err := isBVInGitignore(gitignorePath)
			if err != nil {
				t.Fatalf("isBVInGitignore() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("isBVInGitignore() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsBVInGitignore_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	gitignorePath := filepath.Join(tmpDir, ".gitignore")

	_, err := isBVInGitignore(gitignorePath)
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error, got %v", err)
	}
}

func TestAppendToGitignore(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		pattern         string
		wantContains    []string
		wantPrefix      string // expected prefix of the file (for checking no leading blank line)
	}{
		{
			name:            "new file",
			existingContent: "",
			pattern:         ".bv/",
			wantContains:    []string{"# bv (beads viewer)", ".bv/"},
			wantPrefix:      "#", // should start with comment, not blank line
		},
		{
			name:            "existing file with newline",
			existingContent: "node_modules/\n",
			pattern:         ".bv/",
			wantContains:    []string{"node_modules/", "# bv (beads viewer)", ".bv/"},
			wantPrefix:      "node_modules/",
		},
		{
			name:            "existing file without trailing newline",
			existingContent: "node_modules/",
			pattern:         ".bv/",
			wantContains:    []string{"node_modules/", "# bv (beads viewer)", ".bv/"},
			wantPrefix:      "node_modules/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gitignorePath := filepath.Join(tmpDir, ".gitignore")

			// Create existing file if content is provided
			if tt.existingContent != "" {
				if err := os.WriteFile(gitignorePath, []byte(tt.existingContent), 0644); err != nil {
					t.Fatalf("failed to write existing file: %v", err)
				}
			}

			if err := appendToGitignore(gitignorePath, tt.pattern); err != nil {
				t.Fatalf("appendToGitignore() error = %v", err)
			}

			content, err := os.ReadFile(gitignorePath)
			if err != nil {
				t.Fatalf("failed to read result: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(string(content), want) {
					t.Errorf("result missing %q, got:\n%s", want, content)
				}
			}

			// Check prefix (no unexpected leading blank lines)
			if tt.wantPrefix != "" && !strings.HasPrefix(string(content), tt.wantPrefix) {
				t.Errorf("expected file to start with %q, got:\n%s", tt.wantPrefix, content)
			}
		})
	}
}

func TestEnsureBVInGitignore(t *testing.T) {
	t.Run("creates gitignore if not exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := EnsureBVInGitignore(tmpDir); err != nil {
			t.Fatalf("EnsureBVInGitignore() error = %v", err)
		}

		content, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		if !strings.Contains(string(content), ".bv/") {
			t.Errorf("expected .bv/ in .gitignore, got:\n%s", content)
		}
	})

	t.Run("adds to existing gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitignorePath := filepath.Join(tmpDir, ".gitignore")

		// Create existing .gitignore
		if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644); err != nil {
			t.Fatalf("failed to write .gitignore: %v", err)
		}

		if err := EnsureBVInGitignore(tmpDir); err != nil {
			t.Fatalf("EnsureBVInGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		if !strings.Contains(string(content), "node_modules/") {
			t.Error("existing content was lost")
		}
		if !strings.Contains(string(content), ".bv/") {
			t.Errorf("expected .bv/ in .gitignore, got:\n%s", content)
		}
	})

	t.Run("idempotent - doesn't duplicate", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitignorePath := filepath.Join(tmpDir, ".gitignore")

		// Create existing .gitignore with .bv/ already present
		if err := os.WriteFile(gitignorePath, []byte(".bv/\n"), 0644); err != nil {
			t.Fatalf("failed to write .gitignore: %v", err)
		}

		if err := EnsureBVInGitignore(tmpDir); err != nil {
			t.Fatalf("EnsureBVInGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		// Count occurrences of .bv/
		count := strings.Count(string(content), ".bv/")
		if count != 1 {
			t.Errorf("expected exactly 1 occurrence of .bv/, got %d:\n%s", count, content)
		}
	})

	t.Run("recognizes existing .bv pattern", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitignorePath := filepath.Join(tmpDir, ".gitignore")

		// Create existing .gitignore with .bv (without slash)
		if err := os.WriteFile(gitignorePath, []byte(".bv\n"), 0644); err != nil {
			t.Fatalf("failed to write .gitignore: %v", err)
		}

		if err := EnsureBVInGitignore(tmpDir); err != nil {
			t.Fatalf("EnsureBVInGitignore() error = %v", err)
		}

		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		// Should still have just .bv, not add .bv/
		if strings.Contains(string(content), "# bv (beads viewer)") {
			t.Errorf("should not add when .bv already present, got:\n%s", content)
		}
	})
}

func TestEnsureBVInGitignore_UsesCurrentDir(t *testing.T) {
	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Call with empty string - should use current directory
	if err := EnsureBVInGitignore(""); err != nil {
		t.Fatalf("EnsureBVInGitignore() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	if !strings.Contains(string(content), ".bv/") {
		t.Errorf("expected .bv/ in .gitignore, got:\n%s", content)
	}
}
