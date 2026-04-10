package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_CoalescesRapidTriggers(t *testing.T) {
	d := NewDebouncer(50 * time.Millisecond)

	var callCount atomic.Int32

	// Trigger rapidly 10 times
	for i := 0; i < 10; i++ {
		d.Trigger(func() {
			callCount.Add(1)
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to complete
	time.Sleep(100 * time.Millisecond)

	if count := callCount.Load(); count != 1 {
		t.Errorf("expected 1 callback invocation, got %d", count)
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	d := NewDebouncer(50 * time.Millisecond)

	var called atomic.Bool

	d.Trigger(func() {
		called.Store(true)
	})

	// Cancel before debounce completes
	d.Cancel()

	time.Sleep(100 * time.Millisecond)

	if called.Load() {
		t.Error("callback should not have been invoked after cancel")
	}
}

func TestDebouncer_DefaultDuration(t *testing.T) {
	d := NewDebouncer(0)
	if d.Duration() != DefaultDebounceDuration {
		t.Errorf("expected default duration %v, got %v", DefaultDebounceDuration, d.Duration())
	}
}

func TestWatcher_DetectsFileChange(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	var (
		changeMu sync.Mutex
		changed  bool
	)

	w, err := NewWatcher(tmpFile,
		WithDebounceDuration(50*time.Millisecond),
		WithOnChange(func() {
			changeMu.Lock()
			changed = true
			changeMu.Unlock()
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// Give watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Modify file
	if err := os.WriteFile(tmpFile, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for change detection
	time.Sleep(300 * time.Millisecond)

	changeMu.Lock()
	wasChanged := changed
	changeMu.Unlock()

	if !wasChanged {
		t.Error("expected change to be detected")
	}
}

func TestWatcher_PollingFallback(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	var (
		changeMu sync.Mutex
		changed  bool
	)

	w, err := NewWatcher(tmpFile,
		WithDebounceDuration(50*time.Millisecond),
		WithPollInterval(100*time.Millisecond),
		WithForcePoll(true),
		WithOnChange(func() {
			changeMu.Lock()
			changed = true
			changeMu.Unlock()
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if !w.IsPolling() {
		t.Error("expected watcher to be in polling mode")
	}

	// Give polling time to start
	time.Sleep(50 * time.Millisecond)

	// Modify file
	if err := os.WriteFile(tmpFile, []byte("modified via polling"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for polling to detect change
	time.Sleep(300 * time.Millisecond)

	changeMu.Lock()
	wasChanged := changed
	changeMu.Unlock()

	if !wasChanged {
		t.Error("expected change to be detected via polling")
	}
}

func TestWatcher_ChangedChannel(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(tmpFile,
		WithDebounceDuration(50*time.Millisecond),
		WithPollInterval(100*time.Millisecond),
		WithForcePoll(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// Modify file
	go func() {
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(tmpFile, []byte("new content"), 0644)
	}()

	// Wait for change via channel
	select {
	case <-w.Changed():
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for change notification")
	}
}

func TestWatcher_EnvForcePolling(t *testing.T) {
	t.Setenv("BV_FORCE_POLLING", "1")

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(tmpFile,
		WithDebounceDuration(10*time.Millisecond),
		WithPollInterval(25*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if !w.IsPolling() {
		t.Fatal("expected watcher to be in polling mode when BV_FORCE_POLLING is set")
	}
}

func TestWatcher_RemoteFilesystem_UsesPolling(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	orig := detectFilesystemTypeFunc
	detectFilesystemTypeFunc = func(string) FilesystemType { return FSTypeNFS }
	t.Cleanup(func() { detectFilesystemTypeFunc = orig })

	w, err := NewWatcher(tmpFile,
		WithDebounceDuration(10*time.Millisecond),
		WithPollInterval(25*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if !w.IsPolling() {
		t.Fatal("expected watcher to use polling on remote filesystem")
	}
	if got := w.FilesystemType(); got != FSTypeNFS {
		t.Fatalf("expected filesystem type %v, got %v", FSTypeNFS, got)
	}
}

func TestWatcher_FileRemoved(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	var (
		errMu    sync.Mutex
		gotError error
	)

	w, err := NewWatcher(tmpFile,
		WithDebounceDuration(50*time.Millisecond),
		WithPollInterval(100*time.Millisecond),
		WithForcePoll(true),
		WithOnError(func(err error) {
			errMu.Lock()
			gotError = err
			errMu.Unlock()
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Remove file
	if err := os.Remove(tmpFile); err != nil {
		t.Fatal(err)
	}

	// Wait for error detection
	time.Sleep(300 * time.Millisecond)

	errMu.Lock()
	receivedError := gotError
	errMu.Unlock()

	if receivedError != ErrFileRemoved {
		t.Errorf("expected ErrFileRemoved, got %v", receivedError)
	}
}

func TestWatcher_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if w.IsStarted() {
		t.Error("watcher should not be started initially")
	}

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	if !w.IsStarted() {
		t.Error("watcher should be started after Start()")
	}

	// Double start should error
	if err := w.Start(); err != ErrAlreadyStarted {
		t.Errorf("expected ErrAlreadyStarted, got %v", err)
	}

	w.Stop()

	if w.IsStarted() {
		t.Error("watcher should not be started after Stop()")
	}

	// Double stop should be safe
	w.Stop()
}

func TestWatcher_Path(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	absPath, _ := filepath.Abs(tmpFile)
	if w.Path() != absPath {
		t.Errorf("expected path %s, got %s", absPath, w.Path())
	}
}

func TestWatcher_PollInterval(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	customInterval := 500 * time.Millisecond
	w, err := NewWatcher(tmpFile, WithPollInterval(customInterval))
	if err != nil {
		t.Fatal(err)
	}

	if got := w.PollInterval(); got != customInterval {
		t.Errorf("expected poll interval %v, got %v", customInterval, got)
	}
}

func TestFilesystemType_String(t *testing.T) {
	tests := []struct {
		fsType   FilesystemType
		expected string
	}{
		{FSTypeUnknown, "unknown"},
		{FSTypeLocal, "local"},
		{FSTypeNFS, "nfs"},
		{FSTypeSMB, "smb"},
		{FSTypeSSHFS, "sshfs"},
		{FSTypeFUSE, "fuse"},
		{FilesystemType(99), "unknown"}, // invalid type
	}

	for _, tc := range tests {
		if got := tc.fsType.String(); got != tc.expected {
			t.Errorf("FilesystemType(%d).String() = %q, expected %q", tc.fsType, got, tc.expected)
		}
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"YES", true},
		{"y", true},
		{"Y", true},
		{"on", true},
		{"ON", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"", false},
		{"invalid", false},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("TEST_ENV_BOOL", tc.value)
			if got := envBool("TEST_ENV_BOOL"); got != tc.expected {
				t.Errorf("envBool(%q) = %v, expected %v", tc.value, got, tc.expected)
			}
		})
	}
}

func TestEnvBool_Unset(t *testing.T) {
	// Ensure the variable is not set
	os.Unsetenv("TEST_UNSET_VAR")
	if got := envBool("TEST_UNSET_VAR"); got != false {
		t.Errorf("envBool for unset var = %v, expected false", got)
	}
}

func TestDetectFilesystemType_EmptyPath(t *testing.T) {
	if got := DetectFilesystemType(""); got != FSTypeUnknown {
		t.Errorf("DetectFilesystemType(\"\") = %v, expected FSTypeUnknown", got)
	}
}

func TestDetectFilesystemType_NonExistentPath(t *testing.T) {
	// Should fall back to parent directory detection
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does_not_exist.txt")
	// Should not panic, should return some valid type
	_ = DetectFilesystemType(nonExistent)
}

func TestWatcher_EnvForcePoll(t *testing.T) {
	// Test BV_FORCE_POLL (alternative env var)
	t.Setenv("BV_FORCE_POLL", "true")

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(tmpFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher(tmpFile,
		WithDebounceDuration(10*time.Millisecond),
		WithPollInterval(25*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if !w.IsPolling() {
		t.Fatal("expected watcher to be in polling mode when BV_FORCE_POLL is set")
	}
}
