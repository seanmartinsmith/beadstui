package doltctl

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestParseBdDoltStartOutput(t *testing.T) {
	pid, port, err := parseBdDoltStartOutput("Dolt server started (PID 17056, port 47591)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 17056 {
		t.Errorf("expected PID 17056, got %d", pid)
	}
	if port != 47591 {
		t.Errorf("expected port 47591, got %d", port)
	}
}

func TestParseBdDoltStartOutput_Fallback(t *testing.T) {
	// Unexpected format should return an error, not panic
	_, _, err := parseBdDoltStartOutput("some unexpected output from bd")
	if err == nil {
		t.Fatal("expected error for unexpected output format")
	}
}

func TestParseBdDoltStartOutput_MultiLine(t *testing.T) {
	// bd might emit warnings before the actual output
	output := "warning: something\nDolt server started (PID 9999, port 13309)\n"
	pid, port, err := parseBdDoltStartOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 9999 {
		t.Errorf("expected PID 9999, got %d", pid)
	}
	if port != 13309 {
		t.Errorf("expected port 13309, got %d", port)
	}
}

func TestStopIfOwned_NotStartedByBT(t *testing.T) {
	s := &ServerState{
		StartedByBT: false,
		Port:        12345,
	}
	err := s.StopIfOwned()
	if err != nil {
		t.Fatalf("expected nil error for non-owned server, got: %v", err)
	}
}

func TestStopIfOwned_PIDChanged(t *testing.T) {
	// bt started the server with PID 100, but now the PID file says 200
	// (someone else restarted). bt should NOT stop it.
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "dolt-server.pid")
	if err := os.WriteFile(pidPath, []byte("200"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &ServerState{
		StartedByBT: true,
		ServerPID:   100,
		BeadsDir:    tmpDir,
	}
	// StopIfOwned should return nil (skip stopping) because PID doesn't match
	err := s.StopIfOwned()
	if err != nil {
		t.Fatalf("expected nil error when PID changed, got: %v", err)
	}
}

func TestStopIfOwned_PIDFileGone(t *testing.T) {
	// PID file was deleted (server already stopped externally). bt should not error.
	tmpDir := t.TempDir()

	s := &ServerState{
		StartedByBT: true,
		ServerPID:   100,
		BeadsDir:    tmpDir,
		// no PID file exists
	}
	err := s.StopIfOwned()
	if err != nil {
		t.Fatalf("expected nil error when PID file gone, got: %v", err)
	}
}

func TestStopIfOwned_PIDMatches(t *testing.T) {
	// bt started the server and PID still matches. Should call bd dolt stop.
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "dolt-server.pid")
	if err := os.WriteFile(pidPath, []byte("100"), 0644); err != nil {
		t.Fatal(err)
	}

	called := false
	s := &ServerState{
		StartedByBT: true,
		ServerPID:   100,
		BeadsDir:    tmpDir,
		stopFunc: func() error {
			called = true
			return nil
		},
	}
	err := s.StopIfOwned()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected stopFunc to be called when PID matches")
	}
}

func TestStopIfOwned_PIDMatches_StopFails(t *testing.T) {
	// When bd dolt stop fails, StopIfOwned should return an error but not panic.
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "dolt-server.pid")
	if err := os.WriteFile(pidPath, []byte("100"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &ServerState{
		StartedByBT: true,
		ServerPID:   100,
		BeadsDir:    tmpDir,
		stopFunc: func() error {
			return errors.New("bd dolt stop failed")
		},
	}
	err := s.StopIfOwned()
	if err == nil {
		t.Fatal("expected error when stopFunc fails")
	}
}

func TestEnsureServer_BdNotOnPath(t *testing.T) {
	// When bd is not on PATH, EnsureServer should return a clear error.
	s, err := EnsureServer(t.TempDir(), func(name string) (string, error) {
		return "", errors.New("not found")
	})
	if err == nil {
		t.Fatal("expected error when bd not found")
	}
	if s != nil {
		t.Fatal("expected nil ServerState when bd not found")
	}
}

func TestReadPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "dolt-server.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(42)), 0644); err != nil {
		t.Fatal(err)
	}

	pid, err := readPIDFile(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 42 {
		t.Errorf("expected PID 42, got %d", pid)
	}
}

func TestReadPIDFile_Missing(t *testing.T) {
	_, err := readPIDFile(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing PID file")
	}
}
