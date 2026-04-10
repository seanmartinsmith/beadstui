package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLock_FirstInstance(t *testing.T) {
	tmpDir := t.TempDir()

	lock, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("NewLock failed: %v", err)
	}
	defer lock.Release()

	if !lock.IsFirstInstance() {
		t.Error("expected first instance, got secondary")
	}

	// Verify lock file exists
	lockPath := filepath.Join(tmpDir, LockFileName)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file was not created")
	}

	// Verify lock file contains valid JSON with our PID
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("reading lock file: %v", err)
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("parsing lock file: %v", err)
	}

	if info.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), info.PID)
	}
}

func TestNewLock_SecondInstance(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first lock
	lock1, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("first NewLock failed: %v", err)
	}
	defer lock1.Release()

	if !lock1.IsFirstInstance() {
		t.Error("lock1 should be first instance")
	}

	// Try to create second lock
	lock2, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("second NewLock failed: %v", err)
	}
	defer lock2.Release()

	if lock2.IsFirstInstance() {
		t.Error("lock2 should NOT be first instance")
	}

	// Second instance should know the PID of the holder
	if lock2.HolderPID() != os.Getpid() {
		t.Errorf("expected holder PID %d, got %d", os.Getpid(), lock2.HolderPID())
	}
}

func TestNewLock_StaleLock(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a lock file with a fake (non-existent) PID
	lockPath := filepath.Join(tmpDir, LockFileName)
	info := LockInfo{
		PID:       99999999, // Highly unlikely to be a real PID
		StartedAt: time.Now().Add(-time.Hour),
		Hostname:  "fake-host",
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshaling fake lock: %v", err)
	}
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatalf("writing fake lock: %v", err)
	}

	// NewLock should detect stale lock and take over
	lock, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("NewLock failed: %v", err)
	}
	defer lock.Release()

	// Should have taken over the stale lock
	if !lock.IsFirstInstance() {
		t.Error("should have taken over stale lock and become first instance")
	}

	if lock.HolderPID() != os.Getpid() {
		t.Errorf("expected our PID %d after takeover, got %d", os.Getpid(), lock.HolderPID())
	}
}

func TestLock_Release(t *testing.T) {
	tmpDir := t.TempDir()

	lock, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("NewLock failed: %v", err)
	}

	lockPath := filepath.Join(tmpDir, LockFileName)

	// Verify lock file exists
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should exist before release")
	}

	// Release the lock
	lock.Release()

	// Verify lock file is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should be removed after release")
	}

	// Should be safe to release again
	lock.Release()
}

func TestLock_ReleaseAllowsNewLock(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and release first lock
	lock1, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("first NewLock failed: %v", err)
	}
	lock1.Release()

	// Should be able to create new lock
	lock2, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("second NewLock failed: %v", err)
	}
	defer lock2.Release()

	if !lock2.IsFirstInstance() {
		t.Error("lock2 should be first instance after lock1 released")
	}
}

func TestLock_Path(t *testing.T) {
	tmpDir := t.TempDir()

	lock, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("NewLock failed: %v", err)
	}
	defer lock.Release()

	expected := filepath.Join(tmpDir, LockFileName)
	if lock.Path() != expected {
		t.Errorf("expected path %q, got %q", expected, lock.Path())
	}
}

func TestIsProcessAlive(t *testing.T) {
	// Our own process should be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("our own process should be reported as alive")
	}

	// PID 1 (init/systemd) should be alive on most systems
	// Skip this check as it may not work in all containers
	// if !isProcessAlive(1) {
	// 	t.Error("PID 1 should be alive")
	// }

	// Non-existent PID should not be alive
	if isProcessAlive(99999999) {
		t.Error("fake PID should not be alive")
	}

	// Invalid PIDs
	if isProcessAlive(0) {
		t.Error("PID 0 should not be alive")
	}
	if isProcessAlive(-1) {
		t.Error("negative PID should not be alive")
	}
}

func TestLockInfo_JSON(t *testing.T) {
	info := LockInfo{
		PID:       12345,
		StartedAt: time.Now().Truncate(time.Second),
		Hostname:  "test-host",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed LockInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.PID != info.PID {
		t.Errorf("PID mismatch: expected %d, got %d", info.PID, parsed.PID)
	}
	if !parsed.StartedAt.Equal(info.StartedAt) {
		t.Errorf("StartedAt mismatch: expected %v, got %v", info.StartedAt, parsed.StartedAt)
	}
	if parsed.Hostname != info.Hostname {
		t.Errorf("Hostname mismatch: expected %q, got %q", info.Hostname, parsed.Hostname)
	}
}

func TestNewLock_CorruptedLockFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a corrupted lock file
	lockPath := filepath.Join(tmpDir, LockFileName)
	if err := os.WriteFile(lockPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("writing corrupted lock: %v", err)
	}

	// Should handle corrupted lock gracefully
	lock, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("NewLock should not fail on corrupted lock: %v", err)
	}
	defer lock.Release()

	// With corrupted data (no PID), should try to take over
	// The behavior depends on whether it can detect the "owner" is dead
}

func TestNewLock_EmptyLockFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an empty lock file
	lockPath := filepath.Join(tmpDir, LockFileName)
	if err := os.WriteFile(lockPath, []byte{}, 0644); err != nil {
		t.Fatalf("writing empty lock: %v", err)
	}

	// Should handle empty lock gracefully
	lock, err := NewLock(tmpDir)
	if err != nil {
		t.Fatalf("NewLock should not fail on empty lock: %v", err)
	}
	defer lock.Release()
}

func TestNewLock_ConcurrentStaleTakeover(t *testing.T) {
	// Test that concurrent attempts to take over a stale lock don't both succeed
	// (verifies the atomic rename + verify fix for the TOCTOU race condition)
	tmpDir := t.TempDir()

	// Create a stale lock file with a fake PID
	staleLockPath := filepath.Join(tmpDir, LockFileName)
	info := LockInfo{
		PID:       99999999,
		StartedAt: time.Now().Add(-time.Hour),
		Hostname:  "stale-host",
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshaling stale lock: %v", err)
	}
	if err := os.WriteFile(staleLockPath, data, 0644); err != nil {
		t.Fatalf("writing stale lock: %v", err)
	}

	// Simulate concurrent takeover attempts using goroutines
	// Each goroutine creates a Lock and tries to take over
	const numGoroutines = 10
	type result struct {
		lock    *Lock
		isFirst bool
	}
	results := make(chan result, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			lock, err := NewLock(tmpDir)
			if err != nil {
				results <- result{nil, false}
				return
			}
			// Don't release here - we need to keep locks held until all goroutines finish
			results <- result{lock, lock.IsFirstInstance()}
		}()
	}

	// Collect all results and count first instances
	locks := make([]*Lock, 0, numGoroutines)
	firstCount := 0
	for i := 0; i < numGoroutines; i++ {
		r := <-results
		if r.lock != nil {
			locks = append(locks, r.lock)
		}
		if r.isFirst {
			firstCount++
		}
	}

	// Release all locks after counting
	for _, lock := range locks {
		lock.Release()
	}

	// Exactly one goroutine should claim to be first instance
	if firstCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to be first instance, got %d", firstCount)
	}
}
