// Package watcher provides file watching with debouncing and fallback polling.
package watcher

import (
	"sync"
	"time"
)

// DefaultDebounceDuration is the default debounce window.
const DefaultDebounceDuration = 250 * time.Millisecond

// Debouncer coalesces rapid events into a single callback invocation.
// When Trigger is called multiple times within the debounce duration,
// only the last callback is executed after the duration elapses.
type Debouncer struct {
	duration time.Duration
	timer    *time.Timer
	mu       sync.Mutex
	seq      uint64
}

// NewDebouncer creates a new Debouncer with the specified duration.
// If duration is 0, DefaultDebounceDuration is used.
func NewDebouncer(duration time.Duration) *Debouncer {
	if duration == 0 {
		duration = DefaultDebounceDuration
	}
	return &Debouncer{
		duration: duration,
	}
}

// Trigger schedules the callback to be called after the debounce duration.
// If Trigger is called again before the duration elapses, the previous
// scheduled callback is cancelled and a new one is scheduled.
func (d *Debouncer) Trigger(callback func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seq++
	seq := d.seq

	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.duration, func() {
		shouldRun := func() bool {
			d.mu.Lock()
			defer d.mu.Unlock()

			// Only run the most recently scheduled callback. This avoids races where
			// Stop() returns false because the timer has already fired and the old
			// callback starts running concurrently.
			if seq != d.seq {
				return false
			}
			d.timer = nil
			return true
		}()
		if !shouldRun {
			return
		}

		callback()
	})
}

// Cancel cancels any pending callback.
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Invalidate any callback that might already be executing due to timer races.
	d.seq++

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// Duration returns the debounce duration.
func (d *Debouncer) Duration() time.Duration {
	return d.duration
}
