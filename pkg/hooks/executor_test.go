package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestExportContextToEnv(t *testing.T) {
	ctx := ExportContext{
		ExportPath:   "/tmp/export.md",
		ExportFormat: "markdown",
		IssueCount:   42,
		Timestamp:    time.Date(2025, 11, 30, 10, 30, 0, 0, time.UTC),
	}

	env := ctx.ToEnv()

	expected := map[string]string{
		"BV_EXPORT_PATH":   "/tmp/export.md",
		"BV_EXPORT_FORMAT": "markdown",
		"BV_ISSUE_COUNT":   "42",
		"BV_TIMESTAMP":     "2025-11-30T10:30:00Z",
	}

	for _, e := range env {
		found := false
		for key, val := range expected {
			if e == key+"="+val {
				found = true
				break
			}
		}
		if !found {
			// Check if it's one of our expected keys
			for key := range expected {
				if len(e) > len(key) && e[:len(key)+1] == key+"=" {
					t.Errorf("unexpected value for %s: got %s", key, e)
				}
			}
		}
	}
}

func TestLoaderNoConfig(t *testing.T) {
	// Create a temp directory without hooks.yaml
	tmpDir := t.TempDir()

	loader := NewLoader(WithProjectDir(tmpDir))
	err := loader.Load()
	if err != nil {
		t.Fatalf("expected no error for missing config, got: %v", err)
	}

	if loader.HasHooks() {
		t.Error("expected no hooks when config is missing")
	}
}

func TestLoaderWithValidConfig(t *testing.T) {
	// Create temp directory with .bv/hooks.yaml
	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatalf("failed to create .bv dir: %v", err)
	}

	configContent := `
hooks:
  pre-export:
    - name: validate
      command: echo "validating"
      timeout: 5s
  post-export:
    - name: notify
      command: echo "done"
      timeout: 10s
      env:
        CUSTOM_VAR: custom_value
`
	configPath := filepath.Join(bvDir, "hooks.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader(WithProjectDir(tmpDir))
	err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !loader.HasHooks() {
		t.Error("expected hooks to be loaded")
	}

	preHooks := loader.GetHooks(PreExport)
	if len(preHooks) != 1 {
		t.Fatalf("expected 1 pre-export hook, got %d", len(preHooks))
	}
	if preHooks[0].Name != "validate" {
		t.Errorf("expected hook name 'validate', got %s", preHooks[0].Name)
	}
	if preHooks[0].Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", preHooks[0].Timeout)
	}
	if preHooks[0].OnError != "fail" {
		t.Errorf("expected on_error 'fail' for pre-export, got %s", preHooks[0].OnError)
	}

	postHooks := loader.GetHooks(PostExport)
	if len(postHooks) != 1 {
		t.Fatalf("expected 1 post-export hook, got %d", len(postHooks))
	}
	if postHooks[0].Name != "notify" {
		t.Errorf("expected hook name 'notify', got %s", postHooks[0].Name)
	}
	if postHooks[0].OnError != "continue" {
		t.Errorf("expected on_error 'continue' for post-export, got %s", postHooks[0].OnError)
	}
	if postHooks[0].Env["CUSTOM_VAR"] != "custom_value" {
		t.Errorf("expected CUSTOM_VAR env, got %v", postHooks[0].Env)
	}
}

func TestExecutorRunSimpleHook(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{
					Name:    "echo-test",
					Command: "echo hello",
					Timeout: 5 * time.Second,
					OnError: "fail",
				},
			},
		},
	}

	ctx := ExportContext{
		ExportPath:   "/tmp/test.md",
		ExportFormat: "markdown",
		IssueCount:   10,
		Timestamp:    time.Now(),
	}

	executor := NewExecutor(config, ctx)
	err := executor.RunPreExport()
	if err != nil {
		t.Fatalf("expected hook to succeed, got: %v", err)
	}

	results := executor.Results()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].Success {
		t.Errorf("expected success, got failure: %v", results[0].Error)
	}

	if results[0].Stdout != "hello" {
		t.Errorf("expected stdout 'hello', got %q", results[0].Stdout)
	}
}

func TestExecutorHookFailure(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{
					Name:    "fail-hook",
					Command: "exit 1",
					Timeout: 5 * time.Second,
					OnError: "fail",
				},
			},
		},
	}

	ctx := ExportContext{
		ExportPath:   "/tmp/test.md",
		ExportFormat: "markdown",
		IssueCount:   10,
		Timestamp:    time.Now(),
	}

	executor := NewExecutor(config, ctx)
	err := executor.RunPreExport()
	if err == nil {
		t.Error("expected error for failing pre-export hook")
	}
}

func TestExecutorHookFailureContinue(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PostExport: []Hook{
				{
					Name:    "fail-continue",
					Command: "exit 1",
					Timeout: 5 * time.Second,
					OnError: "continue",
				},
				{
					Name:    "should-run",
					Command: "echo still-running",
					Timeout: 5 * time.Second,
					OnError: "continue",
				},
			},
		},
	}

	ctx := ExportContext{
		ExportPath:   "/tmp/test.md",
		ExportFormat: "markdown",
		IssueCount:   10,
		Timestamp:    time.Now(),
	}

	executor := NewExecutor(config, ctx)
	err := executor.RunPostExport()
	if err != nil {
		t.Errorf("expected no error with on_error=continue, got: %v", err)
	}

	results := executor.Results()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First hook should fail
	if results[0].Success {
		t.Error("expected first hook to fail")
	}

	// Second hook should succeed
	if !results[1].Success {
		t.Errorf("expected second hook to succeed, got: %v", results[1].Error)
	}
	if results[1].Stdout != "still-running" {
		t.Errorf("expected stdout 'still-running', got %q", results[1].Stdout)
	}
}

func TestExecutorHookTimeout(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{
					Name:    "slow-hook",
					Command: "sleep 10",
					Timeout: 100 * time.Millisecond,
					OnError: "fail",
				},
			},
		},
	}

	ctx := ExportContext{
		ExportPath:   "/tmp/test.md",
		ExportFormat: "markdown",
		IssueCount:   10,
		Timestamp:    time.Now(),
	}

	executor := NewExecutor(config, ctx)
	err := executor.RunPreExport()
	if err == nil {
		t.Error("expected timeout error")
	}

	results := executor.Results()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Success {
		t.Error("expected hook to fail due to timeout")
	}

	if results[0].Duration < 100*time.Millisecond {
		t.Errorf("expected duration >= 100ms, got %v", results[0].Duration)
	}
}

func TestExecutorEnvironmentVariables(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{
					Name:    "env-test",
					Command: "echo $BV_EXPORT_PATH $BV_ISSUE_COUNT",
					Timeout: 5 * time.Second,
					OnError: "fail",
				},
			},
		},
	}

	ctx := ExportContext{
		ExportPath:   "/custom/path.md",
		ExportFormat: "markdown",
		IssueCount:   99,
		Timestamp:    time.Now(),
	}

	executor := NewExecutor(config, ctx)
	err := executor.RunPreExport()
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	results := executor.Results()
	if results[0].Stdout != "/custom/path.md 99" {
		t.Errorf("expected env vars in output, got %q", results[0].Stdout)
	}
}

func TestExecutorCustomEnvExpansion(t *testing.T) {
	// Set an env var to be expanded
	os.Setenv("TEST_HOOK_VAR", "expanded_value")
	defer os.Unsetenv("TEST_HOOK_VAR")

	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{
					Name:    "env-expand",
					Command: "echo $CUSTOM_VAR",
					Timeout: 5 * time.Second,
					OnError: "fail",
					Env: map[string]string{
						"CUSTOM_VAR": "${TEST_HOOK_VAR}",
					},
				},
			},
		},
	}

	ctx := ExportContext{
		ExportPath:   "/tmp/test.md",
		ExportFormat: "markdown",
		IssueCount:   10,
		Timestamp:    time.Now(),
	}

	executor := NewExecutor(config, ctx)
	err := executor.RunPreExport()
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	results := executor.Results()
	if results[0].Stdout != "expanded_value" {
		t.Errorf("expected env expansion, got %q", results[0].Stdout)
	}
}

func TestExecutorSummary(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{
					Name:    "success-hook",
					Command: "echo ok",
					Timeout: 5 * time.Second,
					OnError: "continue",
				},
			},
			PostExport: []Hook{
				{
					Name:    "fail-hook",
					Command: "exit 1",
					Timeout: 5 * time.Second,
					OnError: "continue",
				},
			},
		},
	}

	ctx := ExportContext{
		ExportPath:   "/tmp/test.md",
		ExportFormat: "markdown",
		IssueCount:   10,
		Timestamp:    time.Now(),
	}

	executor := NewExecutor(config, ctx)
	_ = executor.RunPreExport()
	_ = executor.RunPostExport()

	summary := executor.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Should mention both success and failure
	if !contains(summary, "1 succeeded") || !contains(summary, "1 failed") {
		t.Errorf("summary should mention success and failure count: %s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLoaderInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatalf("failed to create .bv dir: %v", err)
	}

	// Invalid YAML
	configContent := `
hooks:
  pre-export:
    - name: [invalid yaml
`
	configPath := filepath.Join(bvDir, "hooks.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader(WithProjectDir(tmpDir))
	err := loader.Load()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoaderSkipsEmptyCommands(t *testing.T) {
	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatalf("failed to create .bv dir: %v", err)
	}

	configContent := `
hooks:
  pre-export:
    - name: empty
      command: ""
  post-export:
    - command: "   "
`
	configPath := filepath.Join(bvDir, "hooks.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader(WithProjectDir(tmpDir))
	if err := loader.Load(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loader.HasHooks() {
		t.Error("expected hooks with empty commands to be skipped resulting in no hooks")
	}

	ws := loader.Warnings()
	if len(ws) == 0 {
		t.Fatal("expected warning for empty command")
	}
}

func TestRunPreExportStopsOnFail(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{Name: "fail-fast", Command: "exit 1", Timeout: time.Second, OnError: "fail"},
				{Name: "should-not-run", Command: "echo nope", Timeout: time.Second, OnError: "fail"},
			},
		},
	}

	executor := NewExecutor(config, ExportContext{})
	err := executor.RunPreExport()
	if err == nil {
		t.Fatal("expected error from failing pre-export hook")
	}

	results := executor.Results()
	if len(results) != 1 {
		t.Fatalf("expected only first hook to run, got %d results", len(results))
	}
	if results[0].Success {
		t.Errorf("expected failure result, got success")
	}
}

func TestRunPostExportFailOnErrorStillRunsAll(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PostExport: []Hook{
				{Name: "fail", Command: "exit 1", Timeout: time.Second, OnError: "fail"},
				{Name: "after", Command: "echo ok", Timeout: time.Second, OnError: "continue"},
			},
		},
	}

	executor := NewExecutor(config, ExportContext{})
	err := executor.RunPostExport()
	if err == nil {
		t.Fatal("expected error for post-export hook with on_error=fail")
	}

	results := executor.Results()
	if len(results) != 2 {
		t.Fatalf("expected both hooks to run, got %d", len(results))
	}
	if results[1].Stdout != "ok" {
		t.Errorf("expected second hook to run despite earlier failure, got stdout %q", results[1].Stdout)
	}
}

func TestRunHooksNoHooksConfigured(t *testing.T) {
	projectDir := t.TempDir()

	executor, err := RunHooks(projectDir, ExportContext{}, false)
	if err != nil {
		t.Fatalf("expected no error when no hooks file present, got %v", err)
	}
	if executor != nil {
		t.Fatalf("expected no executor when no hooks configured, got %#v", executor)
	}
}

func TestLoadDefaultNoHooks(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}

	loader, err := LoadDefault()
	if err != nil {
		t.Fatalf("expected no error loading default without config, got %v", err)
	}
	if loader.HasHooks() {
		t.Fatalf("expected no hooks loaded in empty directory")
	}
}

func TestHookUnmarshalYAMLInvalidTimeout(t *testing.T) {
	var h Hook
	err := yaml.Unmarshal([]byte("name: bad\ntimeout: nope\ncommand: echo hi\n"), &h)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestLoaderGetHooksUnknownPhase(t *testing.T) {
	loader := &Loader{
		config: &Config{
			Hooks: HooksByPhase{
				PreExport: []Hook{{Name: "test", Command: "echo ok"}},
			},
		},
	}

	if hooks := loader.GetHooks(HookPhase("unknown")); hooks != nil {
		t.Fatalf("expected nil for unknown phase, got %#v", hooks)
	}
}

func TestExecutorCommandNotFound(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{Name: "missing", Command: "definitely-not-a-real-command-xyz", Timeout: time.Second, OnError: "fail"},
			},
		},
	}

	exec := NewExecutor(config, ExportContext{})
	err := exec.RunPreExport()
	if err == nil {
		t.Fatalf("expected error for missing command")
	}
	results := exec.Results()
	if len(results) != 1 || results[0].Success {
		t.Fatalf("expected failure result for missing command, got %+v", results)
	}
	if results[0].Stderr == "" {
		t.Fatalf("expected stderr to include shell error")
	}
}

func TestExecutorPermissionDenied(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "script.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho nope\n"), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	config := &Config{
		Hooks: HooksByPhase{
			PreExport: []Hook{
				{Name: "perm", Command: script, Timeout: time.Second, OnError: "fail"},
			},
		},
	}

	exec := NewExecutor(config, ExportContext{})
	err := exec.RunPreExport()
	if err == nil {
		t.Fatalf("expected permission error")
	}
	results := exec.Results()
	if len(results) != 1 || results[0].Success {
		t.Fatalf("expected failure result, got %+v", results)
	}
}

func TestExecutorLargeStderrTruncatedInSummary(t *testing.T) {
	config := &Config{
		Hooks: HooksByPhase{
			PostExport: []Hook{
				{
					Name:    "noisy",
					Command: "printf '%0300d' 0 1>&2; exit 1",
					OnError: "continue",
					Timeout: time.Second,
				},
			},
		},
	}

	exec := NewExecutor(config, ExportContext{})
	_ = exec.RunPostExport()

	summary := exec.Summary()
	if summary == "" {
		t.Fatalf("expected summary to include failure")
	}
	if !contains(summary, "stderr:") {
		t.Fatalf("expected stderr line in summary: %s", summary)
	}
	lines := strings.Split(summary, "\n")
	for _, line := range lines {
		if strings.Contains(line, "stderr:") && len(line) > 230 {
			t.Fatalf("expected truncated stderr line, got length %d", len(line))
		}
	}
	if !strings.Contains(summary, "...") {
		t.Fatalf("expected ellipsis indicating truncation")
	}
}
