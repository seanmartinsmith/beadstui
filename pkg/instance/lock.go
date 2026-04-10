// Package instance provides multi-instance coordination for bv.
// It detects when multiple bv instances are running on the same repository
// and provides mechanisms for coordination and user feedback.
package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// staleLockMu serializes stale lock takeover attempts within the same process.
// This prevents multiple goroutines from simultaneously claiming the same stale lock.
var staleLockMu sync.Mutex

// LockInfo contains metadata about the process holding the lock.
type LockInfo struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Hostname  string    `json:"hostname,omitempty"`
}

// Lock represents an instance lock for a beads directory.
// It uses a lock file to detect multiple concurrent instances.
type Lock struct {
	path     string
	lockFile *os.File
	pid      int
	isFirst  bool
}

// LockFileName is the name of the lock file created in .beads directory.
const LockFileName = ".bv.lock"

// NewLock creates a new instance lock for the given beads directory.
// If this is the first instance, it creates and holds the lock.
// If another instance already holds the lock, it returns a Lock with isFirst=false.
func NewLock(beadsDir string) (*Lock, error) {
	lockPath := filepath.Join(beadsDir, LockFileName)

	// Try to create lock file with exclusive access
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists - read it to see who owns it
			existing, readErr := readLockFile(lockPath)
			lock := &Lock{
				path:    lockPath,
				isFirst: false,
			}
			if readErr == nil {
				lock.pid = existing.PID
			}

			// Check if the existing lock is stale
			lock.checkStale()

			return lock, nil
		}
		return nil, fmt.Errorf("creating lock file: %w", err)
	}

	// We got the lock - write our info
	lock := &Lock{
		path:     lockPath,
		lockFile: file,
		isFirst:  true,
		pid:      os.Getpid(),
	}
	if err := lock.writeLockInfo(); err != nil {
		file.Close()
		os.Remove(lockPath)
		return nil, fmt.Errorf("writing lock info: %w", err)
	}

	return lock, nil
}

// writeLockInfo writes the current process info to the lock file.
func (l *Lock) writeLockInfo() error {
	if l.lockFile == nil {
		return nil
	}

	hostname, _ := os.Hostname()
	info := LockInfo{
		PID:       os.Getpid(),
		StartedAt: time.Now(),
		Hostname:  hostname,
	}

	// Truncate and seek to beginning before writing
	if err := l.lockFile.Truncate(0); err != nil {
		return err
	}
	if _, err := l.lockFile.Seek(0, 0); err != nil {
		return err
	}

	return json.NewEncoder(l.lockFile).Encode(info)
}

// readLockFile reads lock info from an existing lock file.
func readLockFile(path string) (*LockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// IsFirstInstance returns true if this is the first/primary instance.
func (l *Lock) IsFirstInstance() bool {
	return l.isFirst
}

// HolderPID returns the PID of the process holding the lock.
// Returns 0 if unknown.
func (l *Lock) HolderPID() int {
	return l.pid
}

// checkStale checks if the existing lock is stale (held by a dead process)
// and takes it over if so. Uses atomic rename to avoid TOCTOU race conditions.
func (l *Lock) checkStale() {
	if l.isFirst {
		return
	}

	// Serialize stale lock takeover attempts within the same process
	staleLockMu.Lock()
	defer staleLockMu.Unlock()

	// Re-read the lock file to get current state (another goroutine may have taken over)
	existing, err := readLockFile(l.path)
	if err != nil {
		// Can't read lock file - might have been deleted
		return
	}
	currentPID := existing.PID

	// Check if the process holding the lock is still alive
	if isProcessAlive(currentPID) {
		// Lock is held by a live process - update our record and don't take over
		l.pid = currentPID
		return
	}

	// Stale lock detected - attempt atomic takeover using rename
	// This avoids the race condition of delete + create with O_EXCL
	tmpPath := fmt.Sprintf("%s.%d", l.path, os.Getpid())

	// Create temp file with our lock info
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}

	hostname, _ := os.Hostname()
	info := LockInfo{
		PID:       os.Getpid(),
		StartedAt: time.Now(),
		Hostname:  hostname,
	}

	if err := json.NewEncoder(file).Encode(info); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return
	}
	file.Sync() // Ensure written to disk before rename
	file.Close()

	// Atomic rename to take over the lock
	// On most filesystems, rename is atomic and will overwrite the existing file.
	// Windows does not allow rename over an existing file, so fall back to remove + rename.
	if err := os.Rename(tmpPath, l.path); err != nil {
		if runtime.GOOS == "windows" {
			if rmErr := os.Remove(l.path); rmErr == nil {
				if err2 := os.Rename(tmpPath, l.path); err2 == nil {
					// Success after Windows-safe fallback.
					goto verify
				}
			}
		}
		os.Remove(tmpPath)
		return
	}

verify:
	// Verify we won the race by re-reading and checking our PID
	// This handles the case where two processes both rename simultaneously
	verifyInfo, err := readLockFile(l.path)
	if err != nil || verifyInfo.PID != os.Getpid() {
		// Lost the race to another process - don't claim ownership
		return
	}

	// Successfully took over the stale lock.
	// The lock is enforced by file existence + content, so we don't need to keep
	// the file open after takeover (it will be closed by the OS on exit anyway).
	l.isFirst = true
	l.pid = os.Getpid()
}

// isProcessAlive is implemented in platform-specific files:
// - lock_unix.go for Unix/Linux/macOS (uses signal 0)
// - lock_windows.go for Windows (uses OpenProcess API)

// Release releases the lock file and cleans up.
// This should be called when the application exits.
func (l *Lock) Release() {
	if l.lockFile != nil {
		l.lockFile.Close()
		l.lockFile = nil
	}

	// Only remove the lock file if we own it
	if l.isFirst {
		os.Remove(l.path)
	}

	l.isFirst = false
}

// Path returns the path to the lock file.
func (l *Lock) Path() string {
	return l.path
}
