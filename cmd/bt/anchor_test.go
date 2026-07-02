package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsAnchorInvalidError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "canonical no active beads workspace",
			err:  errors.New("Error: no active beads workspace found"),
			want: true,
		},
		{
			name: "cannot use -C directory (nonexistent anchor)",
			err:  errors.New("Error: cannot use -C directory /tmp/gone: some stat failure"),
			want: true,
		},
		{
			name: "cannot use -C directory, mixed case",
			err:  errors.New("Error: Cannot Use -C Directory C:\\gone: The system cannot find the path specified."),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAnchorInvalidError(tt.err); got != tt.want {
				t.Errorf("isAnchorInvalidError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsEphemeralAnchorPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	tmp := os.TempDir()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "path under OS temp dir",
			path: filepath.Join(tmp, "agent-fixture-123", "project"),
			want: true,
		},
		{
			name: "path under .claude/jobs",
			path: filepath.Join(home, ".claude", "jobs", "abc123", "tmp", "project"),
			want: true,
		},
		{
			name: "normal project path",
			path: filepath.Join(home, "System", "tools", "bt"),
			want: false,
		},
		{
			name: "sibling directory that merely shares the temp dir prefix",
			path: filepath.Clean(tmp) + "-not-actually-temp",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEphemeralAnchorPath(tt.path); got != tt.want {
				t.Errorf("isEphemeralAnchorPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}

	t.Run("case-insensitive temp dir match", func(t *testing.T) {
		upper := strings.ToUpper(filepath.Join(tmp, "fixture"))
		if !isEphemeralAnchorPath(upper) {
			t.Errorf("isEphemeralAnchorPath(%q) = false, want true (case-insensitive match)", upper)
		}
	})
}
