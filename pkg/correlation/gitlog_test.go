package correlation

import (
	"strings"
	"testing"
)

func TestGitLogConstants(t *testing.T) {
	// Verify the header format contains expected placeholders
	// %H = full commit hash
	// %aI = author date ISO 8601
	// %an = author name
	// %ae = author email
	// %s = subject
	format := gitLogHeaderFormat

	if !strings.Contains(format, "%H") {
		t.Error("expected %H (commit hash) in format")
	}
	if !strings.Contains(format, "%aI") {
		t.Error("expected %aI (ISO date) in format")
	}
	if !strings.Contains(format, "%an") {
		t.Error("expected %an (author name) in format")
	}
	if !strings.Contains(format, "%ae") {
		t.Error("expected %ae (author email) in format")
	}
	if !strings.Contains(format, "%s") {
		t.Error("expected subject placeholder in format")
	}

	// Verify null separator is used
	if !strings.Contains(format, "%x00") {
		t.Error("expected null separator placeholder in format")
	}
}

func TestGitLogMaxScanTokenSize(t *testing.T) {
	// Verify the max scan token size is reasonable
	const minExpected = 1024 * 1024       // At least 1MB
	const maxExpected = 100 * 1024 * 1024 // At most 100MB

	if gitLogMaxScanTokenSize < minExpected {
		t.Errorf("gitLogMaxScanTokenSize too small: %d < %d", gitLogMaxScanTokenSize, minExpected)
	}
	if gitLogMaxScanTokenSize > maxExpected {
		t.Errorf("gitLogMaxScanTokenSize too large: %d > %d", gitLogMaxScanTokenSize, maxExpected)
	}
}
