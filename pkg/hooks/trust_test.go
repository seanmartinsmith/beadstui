package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withIsolatedTrustDB redirects trustDBPath to a tmp file for the duration of
// the test, restoring the original on cleanup. Returns the abs trust DB path.
func withIsolatedTrustDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hook-trust.json")
	orig := trustDBPath
	trustDBPath = func() (string, error) { return dbPath, nil }
	t.Cleanup(func() { trustDBPath = orig })
	return dbPath
}

// writeTempHooksFile writes a minimal hooks.yaml under tmp/.bt/hooks.yaml and
// returns its absolute path.
func writeTempHooksFile(t *testing.T, tmp, content string) string {
	t.Helper()
	bvDir := filepath.Join(tmp, ".bt")
	if err := os.MkdirAll(bvDir, 0o755); err != nil {
		t.Fatalf("mkdir .bt: %v", err)
	}
	path := filepath.Join(bvDir, "hooks.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write hooks.yaml: %v", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs hooks path: %v", err)
	}
	return abs
}

func TestIsTrustedUnknownPath(t *testing.T) {
	withIsolatedTrustDB(t)

	ok, err := IsTrusted("/nonexistent/path/.bt/hooks.yaml", "deadbeef")
	if err != nil {
		t.Fatalf("IsTrusted unknown path: unexpected error %v", err)
	}
	if ok {
		t.Fatalf("IsTrusted unknown path: expected false, got true")
	}
}

func TestRegisterAndIsTrustedMatchingHash(t *testing.T) {
	withIsolatedTrustDB(t)
	tmp := t.TempDir()
	hooksPath := writeTempHooksFile(t, tmp, "hooks:\n  pre-export:\n    - command: echo ok\n")

	hash, err := HashHooksFile(hooksPath)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if err := RegisterTrust(hooksPath, hash); err != nil {
		t.Fatalf("RegisterTrust: %v", err)
	}

	ok, err := IsTrusted(hooksPath, hash)
	if err != nil {
		t.Fatalf("IsTrusted: %v", err)
	}
	if !ok {
		t.Fatalf("expected trusted, got false")
	}
}

func TestIsTrustedMismatchedHash(t *testing.T) {
	withIsolatedTrustDB(t)
	tmp := t.TempDir()
	hooksPath := writeTempHooksFile(t, tmp, "hooks:\n  pre-export:\n    - command: echo a\n")

	hash, err := HashHooksFile(hooksPath)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := RegisterTrust(hooksPath, hash); err != nil {
		t.Fatalf("RegisterTrust: %v", err)
	}

	// Mutate file -> hash changes -> trust should fail.
	if err := os.WriteFile(hooksPath, []byte("hooks:\n  pre-export:\n    - command: rm -rf /\n"), 0o644); err != nil {
		t.Fatalf("rewrite hooks.yaml: %v", err)
	}
	newHash, err := HashHooksFile(hooksPath)
	if err != nil {
		t.Fatalf("rehash: %v", err)
	}
	if newHash == hash {
		t.Fatalf("expected hash change after mutation")
	}

	ok, err := IsTrusted(hooksPath, newHash)
	if err != nil {
		t.Fatalf("IsTrusted mismatched: %v", err)
	}
	if ok {
		t.Fatalf("expected mismatched hash to fail trust check")
	}
}

func TestTrustDBRoundTripPersistence(t *testing.T) {
	withIsolatedTrustDB(t)

	db := &TrustDB{
		Version: trustDBVersion,
		Trusted: map[string]TrustedRecord{
			"/abs/proj/.bt/hooks.yaml": {SHA256: "abc123", TrustedAt: time.Now().UTC().Truncate(time.Second)},
		},
	}
	if err := SaveTrustDB(db); err != nil {
		t.Fatalf("SaveTrustDB: %v", err)
	}

	loaded, err := LoadTrustDB()
	if err != nil {
		t.Fatalf("LoadTrustDB: %v", err)
	}
	if loaded.Version != trustDBVersion {
		t.Fatalf("version mismatch: %d", loaded.Version)
	}
	rec, ok := loaded.Trusted["/abs/proj/.bt/hooks.yaml"]
	if !ok {
		t.Fatalf("trusted entry missing after round trip")
	}
	if rec.SHA256 != "abc123" {
		t.Fatalf("sha256 mismatch: %q", rec.SHA256)
	}
}

func TestLoadTrustDBMissingReturnsEmpty(t *testing.T) {
	withIsolatedTrustDB(t)

	db, err := LoadTrustDB()
	if err != nil {
		t.Fatalf("LoadTrustDB on missing: %v", err)
	}
	if db == nil {
		t.Fatalf("expected non-nil db on missing file")
	}
	if len(db.Trusted) != 0 {
		t.Fatalf("expected empty trusted map, got %d entries", len(db.Trusted))
	}
}

// --- executor-level trust gate tests ---

func TestExecutorTrustGateUntrustedRefuses(t *testing.T) {
	withIsolatedTrustDB(t)
	tmp := t.TempDir()
	hooksPath := writeTempHooksFile(t, tmp, "hooks:\n  pre-export:\n    - command: echo hi\n")

	loader := NewLoader(WithProjectDir(tmp))
	if err := loader.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	exec := NewExecutor(loader.Config(), ExportContext{})
	exec.SetTrustGate(hooksPath, false)

	err := exec.RunPreExport()
	if err == nil {
		t.Fatalf("expected refusal for untrusted hooks")
	}
	var ute *UntrustedHooksError
	if !errors.As(err, &ute) {
		t.Fatalf("expected UntrustedHooksError, got %T: %v", err, err)
	}
	if ute.Path != hooksPath {
		t.Fatalf("UntrustedHooksError.Path = %q, want %q", ute.Path, hooksPath)
	}

	// Confirm no hook actually ran.
	if got := len(exec.Results()); got != 0 {
		t.Fatalf("expected zero results when refused, got %d", got)
	}
}

func TestExecutorTrustGateAllowHooksBypass(t *testing.T) {
	withIsolatedTrustDB(t)
	tmp := t.TempDir()
	hooksPath := writeTempHooksFile(t, tmp, "hooks:\n  pre-export:\n    - command: echo hi\n")

	loader := NewLoader(WithProjectDir(tmp))
	if err := loader.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	exec := NewExecutor(loader.Config(), ExportContext{})
	exec.SetTrustGate(hooksPath, true) // bypass

	if err := exec.RunPreExport(); err != nil {
		t.Fatalf("expected success with --allow-hooks bypass, got %v", err)
	}
	if got := len(exec.Results()); got != 1 {
		t.Fatalf("expected 1 hook to run, got %d", got)
	}
}

func TestExecutorTrustGateTrustedFile(t *testing.T) {
	withIsolatedTrustDB(t)
	tmp := t.TempDir()
	hooksPath := writeTempHooksFile(t, tmp, "hooks:\n  pre-export:\n    - command: echo hi\n")

	hash, err := HashHooksFile(hooksPath)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := RegisterTrust(hooksPath, hash); err != nil {
		t.Fatalf("RegisterTrust: %v", err)
	}

	loader := NewLoader(WithProjectDir(tmp))
	if err := loader.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	exec := NewExecutor(loader.Config(), ExportContext{})
	exec.SetTrustGate(hooksPath, false)

	if err := exec.RunPreExport(); err != nil {
		t.Fatalf("expected success for trusted file, got %v", err)
	}
	if got := len(exec.Results()); got != 1 {
		t.Fatalf("expected 1 hook to run, got %d", got)
	}
}

func TestRunHooksWiresTrustGate(t *testing.T) {
	withIsolatedTrustDB(t)
	tmp := t.TempDir()
	writeTempHooksFile(t, tmp, "hooks:\n  pre-export:\n    - command: echo hi\n")

	// Without --allow-hooks and no trust grant, RunHooks returns an executor
	// that refuses on first invocation.
	exec, err := RunHooks(tmp, ExportContext{}, false)
	if err != nil {
		t.Fatalf("RunHooks: %v", err)
	}
	if exec == nil {
		t.Fatalf("expected executor")
	}
	if err := exec.RunPreExport(); err == nil {
		t.Fatalf("expected refusal without trust grant")
	} else {
		var ute *UntrustedHooksError
		if !errors.As(err, &ute) {
			t.Fatalf("expected UntrustedHooksError, got %T: %v", err, err)
		}
	}

	// With --allow-hooks, RunHooks returns an executor that runs without
	// consulting trust DB.
	exec2, err := RunHooks(tmp, ExportContext{}, true)
	if err != nil {
		t.Fatalf("RunHooks allow: %v", err)
	}
	if err := exec2.RunPreExport(); err != nil {
		t.Fatalf("expected success with allowHooks bypass, got %v", err)
	}
}
