package hooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// HookResult contains the result of a hook execution
type HookResult struct {
	Hook     Hook
	Phase    HookPhase
	Success  bool
	Stdout   string
	Stderr   string
	Duration time.Duration
	Error    error
}

// Executor runs hooks with proper environment and timeout handling
type Executor struct {
	config  *Config
	context ExportContext
	results []HookResult
	logger  func(string)

	// Trust-gate state. Populated by NewExecutor + SetTrustGate. The gate is
	// enforced at the top of RunPreExport/RunPostExport: we refuse to run any
	// command unless allowHooks==true OR the hooksFilePath is registered in
	// the trust DB with a matching content hash.
	hooksFilePath string
	allowHooks    bool

	// stderr is where pre-run "would run hook" warnings are emitted. Defaults
	// to os.Stderr; tests substitute a buffer.
	stderr io.Writer
}

// NewExecutor creates a new hook executor with the trust gate engaged.
// hooksFilePath should be the absolute path to the .bt/hooks.yaml whose
// contents produced cfg. When allowHooks is true the trust gate is bypassed
// (CI escape hatch); otherwise the executor refuses to run any hook unless
// the file is registered in the user's trust DB with a matching hash.
//
// hooksFilePath="" disables the trust check entirely. This is intended only
// for callers that synthesize Config directly in tests or in-process pipelines
// that never read user-controlled YAML; production export paths must always
// pass a real path.
func NewExecutor(config *Config, ctx ExportContext) *Executor {
	return &Executor{
		config:  config,
		context: ctx,
		results: make([]HookResult, 0),
		logger:  func(string) {}, // No-op default
		stderr:  os.Stderr,
	}
}

// SetTrustGate configures the trust-gate state. hooksFilePath is the absolute
// path to the hooks.yaml that produced this executor's config. When
// allowHooks is true, the gate is bypassed entirely. Calling SetTrustGate is
// required for any Executor that loaded its config from a user-controlled
// YAML file; in-process tests with synthesized configs may leave it unset.
func (e *Executor) SetTrustGate(hooksFilePath string, allowHooks bool) {
	e.hooksFilePath = hooksFilePath
	e.allowHooks = allowHooks
}

// SetLogger sets the logger function for hook execution details
func (e *Executor) SetLogger(logger func(string)) {
	if logger == nil {
		e.logger = func(string) {}
		return
	}
	e.logger = logger
}

// SetStderr overrides the writer used for pre-run "would run hook" warnings.
// Primarily for tests.
func (e *Executor) SetStderr(w io.Writer) {
	if w == nil {
		e.stderr = os.Stderr
		return
	}
	e.stderr = w
}

// checkTrust enforces the gate. Returns nil if execution may proceed. Returns
// *UntrustedHooksError when the file is not trusted. allowHooks=true and an
// empty hooksFilePath both bypass the gate.
func (e *Executor) checkTrust() error {
	if e.allowHooks {
		return nil
	}
	if e.hooksFilePath == "" {
		// In-process / test path with no user-controlled YAML to gate.
		return nil
	}
	hash, err := HashHooksFile(e.hooksFilePath)
	if err != nil {
		return fmt.Errorf("hash hooks file: %w", err)
	}
	trusted, err := IsTrusted(e.hooksFilePath, hash)
	if err != nil {
		return fmt.Errorf("check trust: %w", err)
	}
	if !trusted {
		return &UntrustedHooksError{Path: e.hooksFilePath, Hash: hash}
	}
	return nil
}

// announceHook prints a defense-in-depth pre-run warning to stderr so users
// see what bt is about to execute even when the file is trusted. Format:
//
//	bt: would run hook '<phase>': <command>
func (e *Executor) announceHook(phase HookPhase, hook Hook) {
	if e.stderr == nil {
		return
	}
	fmt.Fprintf(e.stderr, "bt: would run hook '%s': %s\n", phase, hook.Command)
}

// RunPreExport executes all pre-export hooks
// Returns error if any hook fails with on_error="fail"
func (e *Executor) RunPreExport() error {
	if e.config == nil {
		return nil
	}
	if len(e.config.Hooks.PreExport) == 0 {
		return nil
	}
	if err := e.checkTrust(); err != nil {
		return err
	}

	for _, hook := range e.config.Hooks.PreExport {
		e.announceHook(PreExport, hook)
		e.logger(fmt.Sprintf("Running pre-export hook %q: %s", hook.Name, hook.Command))
		result := e.runHook(hook, PreExport)
		e.results = append(e.results, result)

		if !result.Success && hook.OnError == "fail" {
			return fmt.Errorf("pre-export hook %q failed: %w", hook.Name, result.Error)
		}
	}

	return nil
}

// RunPostExport executes all post-export hooks
// Errors are logged but don't fail (unless on_error="fail")
func (e *Executor) RunPostExport() error {
	if e.config == nil {
		return nil
	}
	if len(e.config.Hooks.PostExport) == 0 {
		return nil
	}
	if err := e.checkTrust(); err != nil {
		return err
	}

	var firstError error
	for _, hook := range e.config.Hooks.PostExport {
		e.announceHook(PostExport, hook)
		e.logger(fmt.Sprintf("Running post-export hook %q: %s", hook.Name, hook.Command))
		result := e.runHook(hook, PostExport)
		e.results = append(e.results, result)

		if !result.Success && hook.OnError == "fail" && firstError == nil {
			firstError = fmt.Errorf("post-export hook %q failed: %w", hook.Name, result.Error)
		}
	}

	return firstError
}

// getShellCommand returns the shell and flag to use for executing commands
func getShellCommand() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/C"
	}
	return "sh", "-c"
}

// runHook executes a single hook with timeout and environment
func (e *Executor) runHook(hook Hook, phase HookPhase) HookResult {
	result := HookResult{
		Hook:  hook,
		Phase: phase,
	}

	start := time.Now()

	// Create context with timeout
	timeout := hook.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create command - use shell to interpret the command
	shell, flag := getShellCommand()
	cmd := exec.CommandContext(ctx, shell, flag, hook.Command)

	// Build environment
	cmd.Env = os.Environ()

	// Add export context variables
	cmd.Env = append(cmd.Env, e.context.ToEnv()...)

	// Add hook-specific env vars (with ${VAR} expansion from current env)
	// Sort keys for deterministic environment order
	envKeys := make([]string, 0, len(hook.Env))
	for k := range hook.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	for _, key := range envKeys {
		value := hook.Env[key]
		// Use custom expansion that sees both OS env and context variables
		expandedValue := expandEnv(value, cmd.Env)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, expandedValue))
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("timeout after %v", timeout)
		} else {
			result.Error = err
		}
		result.Success = false
	} else {
		result.Success = true
	}

	return result
}

// expandEnv replaces ${VAR} or $VAR in the string using values from the env slice
// Note: This only supports standard shell variable expansion. It does not support
// complex shell parameter expansion like ${VAR:-default} or ${VAR:offset}.
func expandEnv(s string, env []string) string {
	return os.Expand(s, func(key string) string {
		// Search env slice from end to beginning (to respect precedence if duplicates exist)
		prefix := key + "="
		for i := len(env) - 1; i >= 0; i-- {
			if strings.HasPrefix(env[i], prefix) {
				return env[i][len(prefix):]
			}
		}
		return ""
	})
}

// Results returns all hook execution results
func (e *Executor) Results() []HookResult {
	return e.results
}

// Summary returns a human-readable summary of hook execution
func (e *Executor) Summary() string {
	if len(e.results) == 0 {
		return "No hooks executed"
	}

	var sb strings.Builder
	var succeeded, failed int

	for _, r := range e.results {
		if r.Success {
			succeeded++
			sb.WriteString(fmt.Sprintf("  [OK] %s (%v)\n", r.Hook.Name, r.Duration.Round(time.Millisecond)))
		} else {
			failed++
			sb.WriteString(fmt.Sprintf("  [FAIL] %s: %v\n", r.Hook.Name, r.Error))
			if r.Stderr != "" {
				sb.WriteString(fmt.Sprintf("         stderr: %s\n", truncate(r.Stderr, 200)))
			}
		}
	}

	header := fmt.Sprintf("Hook execution: %d succeeded, %d failed\n", succeeded, failed)
	return header + sb.String()
}

// truncate shortens a string to max length with ellipsis
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

// RunHooks is a convenience that loads .bt/hooks.yaml from projectDir and
// returns a configured Executor with the trust gate engaged. Returns
// (nil, nil) when there is no hooks.yaml or no hooks configured.
//
// allowHooks=true bypasses the trust gate for the returned Executor (CI
// escape hatch). Without it, the Executor refuses to run any hook unless the
// resolved hooks.yaml is registered in ~/.bt/hook-trust.json with a matching
// content hash.
func RunHooks(projectDir string, ctx ExportContext, allowHooks bool) (*Executor, error) {
	loader := NewLoader(WithProjectDir(projectDir))
	if err := loader.Load(); err != nil {
		return nil, fmt.Errorf("loading hooks: %w", err)
	}

	if !loader.HasHooks() {
		return nil, nil
	}

	hooksPath, err := filepath.Abs(filepath.Join(projectDir, ".bt", "hooks.yaml"))
	if err != nil {
		return nil, fmt.Errorf("resolve hooks path: %w", err)
	}

	executor := NewExecutor(loader.Config(), ctx)
	executor.SetTrustGate(hooksPath, allowHooks)
	return executor, nil
}
