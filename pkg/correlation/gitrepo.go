// Package correlation: gitrepo.go provides helpers for probing whether a path
// is inside a git working tree, so callers can distinguish between "not a git
// repo" (an expected condition that should fail silently) and real git
// invocation failures (binary missing, permission errors, etc.) which should
// surface as errors.
package correlation

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// IsInsideWorkTree reports whether repoPath is inside a git working tree.
//
// Return contract:
//   - (true, nil)  -> path is inside a git work tree; safe to invoke `git log`.
//   - (false, nil) -> path is not inside a git work tree (e.g. plain home dir).
//     This is an expected condition; callers should fall back silently.
//   - (false, err) -> a real failure occurred (git binary missing, permission
//     error, or any unexpected stderr). Callers should surface this.
//
// Detection mechanism: `git rev-parse --is-inside-work-tree`. Git exits 128
// with stderr "fatal: not a git repository ..." when the path isn't in a repo;
// we recognize that as the silent-fallback case. Any other error (most
// importantly exec.ErrNotFound when `git` is missing from PATH) is propagated.
func IsInsideWorkTree(repoPath string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = repoPath

	out, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)) == "true", nil
	}

	// Distinguish "git binary missing" / startup failure from "not a git repo".
	// exec.ErrNotFound (and other non-ExitError failures) means git itself
	// could not run -> real error.
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false, fmt.Errorf("running git rev-parse: %w", err)
	}

	// We have an ExitError. Inspect stderr/stdout to recognize the
	// "not a git repository" case, which git reports on stderr but
	// CombinedOutput merges into `out`.
	combined := strings.ToLower(string(out))
	if strings.Contains(combined, "not a git repository") ||
		strings.Contains(combined, "not a working tree") {
		return false, nil
	}

	// Any other non-zero exit is a real error (corrupt repo, permission
	// issue, etc.) -- surface it.
	return false, fmt.Errorf("git rev-parse failed: %s", strings.TrimSpace(string(out)))
}
