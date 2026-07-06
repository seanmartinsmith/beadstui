// Package bdexec runs bd (the beads CLI) as a child process for bt's write
// path. It is the single choke point through which every bt-initiated bd
// mutation flows, so the exact argv, exit code, and captured output are
// available for user feedback and (future) the always-on command-log receipts
// pane (bt-oiaj.11).
//
// The parent process environment is inherited untouched: BD_ACTOR and session
// identity stamping flow through exactly as the user configured them - bt never
// overrides or strips them (bt-oiaj.10). The canonical bd command-builder
// package (bt-s5zgk.1) is a later extraction from this live write; this package
// stays deliberately small.
package bdexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout bounds a single bd invocation so a wedged child cannot hang
// the caller. Mirrors the bounded-context pattern of the doltctl bd shell-outs.
const DefaultTimeout = 30 * time.Second

// Result captures everything an observer needs about one bd invocation: the
// full argv (with the "bd" program name), the process exit code, captured
// stdout/stderr, the wall-clock duration, and any spawn/timeout error. ExitCode
// is -1 when the process never produced an exit status (spawn failure/timeout).
// Args is always populated - even on spawn failure - so a trace or receipt can
// record what was attempted.
type Result struct {
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error
}

// Argv renders the invocation as a single-line command for a trace log or
// receipts entry (e.g. "bd update bt-1 --claim").
func (r Result) Argv() string {
	return strings.Join(r.Args, " ")
}

// Run executes `bd <args...>` in dir with the DefaultTimeout guard, inheriting
// the parent process environment untouched. dir may be "" to run in the
// caller's current directory.
func Run(ctx context.Context, dir string, args ...string) Result {
	return RunWithTimeout(ctx, dir, DefaultTimeout, args...)
}

// RunWithTimeout is Run with an explicit per-invocation timeout.
func RunWithTimeout(ctx context.Context, dir string, timeout time.Duration, args ...string) Result {
	res := Result{Args: append([]string{"bd"}, args...), ExitCode: -1}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bd", args...)
	cmd.Dir = dir
	// Intentionally do NOT set cmd.Env: the child inherits the parent
	// environment so BD_ACTOR / CLAUDE_CODE_SESSION_ID flow through untouched
	// (bt-oiaj.10).

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	res.Duration = time.Since(start)
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		}
		res.Err = err
		return res
	}
	res.ExitCode = 0
	return res
}
