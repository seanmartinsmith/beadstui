// Package cass provides detection and health checking for the cass search tool.
// Cass is an external binary for semantic code search that may or may not be installed.
package cass

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// Status represents the availability state of cass.
type Status int

const (
	// StatusUnknown indicates detection hasn't been performed yet.
	StatusUnknown Status = iota
	// StatusNotInstalled indicates cass binary is not found in PATH or is not executable.
	StatusNotInstalled
	// StatusNeedsIndex indicates cass is installed but needs indexing before use.
	StatusNeedsIndex
	// StatusHealthy indicates cass is installed, indexed, and ready for searches.
	StatusHealthy
)

// String returns a human-readable status description.
func (s Status) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusNotInstalled:
		return "not installed"
	case StatusNeedsIndex:
		return "needs indexing"
	case StatusHealthy:
		return "healthy"
	default:
		return "unknown"
	}
}

// DefaultCacheTTL is the default duration to cache detection results.
const DefaultCacheTTL = 5 * time.Minute

// DefaultHealthTimeout is the default timeout for health check commands.
const DefaultHealthTimeout = 2 * time.Second

// Detector checks if cass is installed and operational.
// It caches results and is safe for concurrent use.
type Detector struct {
	status        Status
	checkedAt     time.Time
	cacheTTL      time.Duration
	healthTimeout time.Duration
	mu            sync.RWMutex

	// For testing: allow overriding command execution
	lookPath   func(string) (string, error)
	runCommand func(ctx context.Context, name string, args ...string) (int, error)
}

// NewDetector creates a new Detector with default settings.
func NewDetector() *Detector {
	return &Detector{
		status:        StatusUnknown,
		cacheTTL:      DefaultCacheTTL,
		healthTimeout: DefaultHealthTimeout,
		lookPath:      exec.LookPath,
		runCommand:    defaultRunCommand,
	}
}

// Option configures a Detector.
type Option func(*Detector)

// WithCacheTTL sets the cache time-to-live.
func WithCacheTTL(ttl time.Duration) Option {
	return func(d *Detector) {
		d.cacheTTL = ttl
	}
}

// WithHealthTimeout sets the health check timeout.
func WithHealthTimeout(timeout time.Duration) Option {
	return func(d *Detector) {
		d.healthTimeout = timeout
	}
}

// NewDetectorWithOptions creates a new Detector with custom options.
func NewDetectorWithOptions(opts ...Option) *Detector {
	d := NewDetector()
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Status returns the current cached status.
// If the cache is stale or unknown, returns StatusUnknown.
// Use Check() to perform an actual detection.
func (d *Detector) Status() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.status == StatusUnknown {
		return StatusUnknown
	}

	if time.Since(d.checkedAt) > d.cacheTTL {
		return StatusUnknown
	}

	return d.status
}

// IsHealthy returns true if cass is ready to use.
// This is a convenience method that checks the cached status.
func (d *Detector) IsHealthy() bool {
	return d.Status() == StatusHealthy
}

// Check performs detection and returns the current status.
// Results are cached for cacheTTL duration.
// This method is safe for concurrent use.
func (d *Detector) Check() Status {
	// Fast path: return cached result if still valid
	d.mu.RLock()
	if d.status != StatusUnknown && time.Since(d.checkedAt) <= d.cacheTTL {
		status := d.status
		d.mu.RUnlock()
		return status
	}
	d.mu.RUnlock()

	// Slow path: perform detection
	d.mu.Lock()
	defer d.mu.Unlock()

	// Double-check after acquiring write lock
	if d.status != StatusUnknown && time.Since(d.checkedAt) <= d.cacheTTL {
		return d.status
	}

	d.status = d.detect()
	d.checkedAt = time.Now()
	return d.status
}

// Invalidate clears the cached status, forcing a fresh check on next Check() call.
func (d *Detector) Invalidate() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.status = StatusUnknown
	d.checkedAt = time.Time{}
}

// detect performs the actual detection logic.
// Caller must hold d.mu (write lock) to safely update status with the result.
func (d *Detector) detect() Status {
	// Step 1: Check if cass binary exists in PATH
	_, err := d.lookPath("cass")
	if err != nil {
		return StatusNotInstalled
	}

	// Step 2: Run health check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), d.healthTimeout)
	defer cancel()

	exitCode, err := d.runCommand(ctx, "cass", "health")
	if err != nil {
		// Command failed to run (timeout, not found, etc.)
		return StatusNotInstalled
	}

	// Interpret exit code
	switch exitCode {
	case 0:
		return StatusHealthy
	case 1:
		return StatusNeedsIndex
	case 3:
		// Index missing or corrupt
		return StatusNeedsIndex
	default:
		return StatusNotInstalled
	}
}

// defaultRunCommand executes a command and returns its exit code.
func defaultRunCommand(ctx context.Context, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// CheckedAt returns when the last check was performed.
// Returns zero time if never checked.
func (d *Detector) CheckedAt() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.checkedAt
}

// CacheValid returns true if the cached result is still valid.
func (d *Detector) CacheValid() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.status != StatusUnknown && time.Since(d.checkedAt) <= d.cacheTTL
}
