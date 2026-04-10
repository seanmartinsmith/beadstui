package watcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultPollInterval is the default polling interval for fallback mode.
const DefaultPollInterval = 2 * time.Second

// Common errors.
var (
	ErrFileRemoved    = errors.New("watched file was removed")
	ErrPermission     = errors.New("permission denied")
	ErrAlreadyStarted = errors.New("watcher already started")
)

// WatcherOption configures a Watcher.
type WatcherOption func(*Watcher)

// WithDebounceDuration sets the debounce duration.
func WithDebounceDuration(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.debounceDuration = d
	}
}

// WithPollInterval sets the polling interval for fallback mode.
func WithPollInterval(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.pollInterval = d
	}
}

// WithOnChange sets the callback invoked when the file changes.
func WithOnChange(fn func()) WatcherOption {
	return func(w *Watcher) {
		w.onChange = fn
	}
}

// WithOnError sets the callback invoked on errors.
func WithOnError(fn func(error)) WatcherOption {
	return func(w *Watcher) {
		w.onError = fn
	}
}

// WithForcePoll forces polling mode even if fsnotify is available.
func WithForcePoll(force bool) WatcherOption {
	return func(w *Watcher) {
		w.forcePoll = force
	}
}

// Watcher monitors a file for changes using fsnotify with polling fallback.
type Watcher struct {
	path             string
	debounceDuration time.Duration
	pollInterval     time.Duration
	onChange         func()
	onError          func(error)
	forcePoll        bool
	forcePollEnv     bool
	fsType           FilesystemType

	fsWatcher   *fsnotify.Watcher
	debouncer   *Debouncer
	useFallback bool
	lastMtime   time.Time
	lastSize    int64

	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
	mu       sync.RWMutex
	changeCh chan struct{}
}

// NewWatcher creates a new file watcher for the given path.
func NewWatcher(path string, opts ...WatcherOption) (*Watcher, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		path:             absPath,
		debounceDuration: DefaultDebounceDuration,
		pollInterval:     DefaultPollInterval,
		onChange:         func() {},
		onError:          func(error) {},
		changeCh:         make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(w)
	}

	w.debouncer = NewDebouncer(w.debounceDuration)

	return w, nil
}

// Start begins watching the file for changes.
func (w *Watcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return ErrAlreadyStarted
	}

	w.ctx, w.cancel = context.WithCancel(context.Background())

	// Reset per-start state.
	w.useFallback = false
	w.forcePollEnv = false
	w.fsType = FSTypeUnknown

	if envBool("BV_FORCE_POLLING") || envBool("BV_FORCE_POLL") {
		w.forcePollEnv = true
	}

	w.fsType = DetectFilesystemType(w.path)
	if isRemoteFilesystem(w.fsType) {
		w.useFallback = true
	}

	forcePoll := w.forcePoll || w.forcePollEnv
	if forcePoll {
		w.useFallback = true
	}

	// Get initial file state
	info, err := os.Stat(w.path)
	if err != nil {
		if os.IsPermission(err) {
			return ErrPermission
		}
		// File might not exist yet, that's okay
		w.lastMtime = time.Time{}
		w.lastSize = 0
	} else {
		w.lastMtime = info.ModTime()
		w.lastSize = info.Size()
	}

	// Try to use fsnotify
	if !forcePoll && !w.useFallback {
		fsw, err := fsnotify.NewWatcher()
		if err == nil {
			// Watch the directory containing the file (more reliable for atomic writes)
			dir := filepath.Dir(w.path)
			if err := fsw.Add(dir); err != nil {
				fsw.Close()
				w.useFallback = true
			} else {
				w.fsWatcher = fsw
				w.useFallback = false
				go w.watchFsnotify()
			}
		} else {
			w.useFallback = true
		}
	} else {
		w.useFallback = true
	}

	// Start polling as fallback or primary
	if w.useFallback {
		go w.watchPolling()
	}

	w.started = true
	return nil
}

// Stop stops watching the file.
// Note: The changeCh channel is intentionally NOT closed here. Closing it would
// cause race conditions with notifyChange() and break WatchFileCmd (which would
// receive immediately and potentially loop). Since Stop() is only called at
// program exit, the goroutine blocked on Changed() is cleaned up by process
// termination.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.started {
		return
	}

	if w.cancel != nil {
		w.cancel()
	}

	if w.fsWatcher != nil {
		w.fsWatcher.Close()
		w.fsWatcher = nil
	}

	w.debouncer.Cancel()
	w.started = false
}

// IsPolling returns true if the watcher is using polling mode.
func (w *Watcher) IsPolling() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.useFallback
}

// IsStarted returns true if the watcher is running.
func (w *Watcher) IsStarted() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.started
}

// Changed returns a channel that receives when the file changes.
// This is an alternative to using the OnChange callback.
func (w *Watcher) Changed() <-chan struct{} {
	return w.changeCh
}

// Path returns the watched file path.
func (w *Watcher) Path() string {
	return w.path
}

// FilesystemType returns the best-effort filesystem classification for the watched path.
func (w *Watcher) FilesystemType() FilesystemType {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.fsType
}

// PollInterval returns the polling interval used when polling mode is active.
func (w *Watcher) PollInterval() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.pollInterval
}

func envBool(name string) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// watchFsnotify monitors using fsnotify events.
func (w *Watcher) watchFsnotify() {
	targetFile := filepath.Base(w.path)

	// Capture channel references to avoid race with Stop() setting fsWatcher to nil
	w.mu.RLock()
	if w.fsWatcher == nil {
		w.mu.RUnlock()
		return
	}
	events := w.fsWatcher.Events
	errors := w.fsWatcher.Errors
	w.mu.RUnlock()

	for {
		select {
		case <-w.ctx.Done():
			return

		case event, ok := <-events:
			if !ok {
				return
			}

			// Only care about events for our specific file
			eventFile := filepath.Base(event.Name)
			if eventFile != targetFile {
				continue
			}

			switch {
			case event.Op&fsnotify.Remove != 0:
				w.onError(ErrFileRemoved)

			case event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0:
				w.debouncer.Trigger(w.notifyChange)
			}

		case err, ok := <-errors:
			if !ok {
				return
			}
			w.onError(err)
		}
	}
}

// watchPolling monitors using periodic stat checks.
func (w *Watcher) watchPolling() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return

		case <-ticker.C:
			info, err := os.Stat(w.path)
			if err != nil {
				if os.IsNotExist(err) {
					// Only report if file existed before
					w.mu.RLock()
					hadFile := !w.lastMtime.IsZero()
					w.mu.RUnlock()
					if hadFile {
						w.onError(ErrFileRemoved)
					}
				} else if os.IsPermission(err) {
					w.onError(ErrPermission)
				} else {
					w.onError(err)
				}
				continue
			}

			w.mu.Lock()
			changed := info.ModTime().After(w.lastMtime) || info.Size() != w.lastSize
			if changed {
				w.lastMtime = info.ModTime()
				w.lastSize = info.Size()
			}
			w.mu.Unlock()

			if changed {
				w.debouncer.Trigger(w.notifyChange)
			}
		}
	}
}

// notifyChange invokes the onChange callback and signals the change channel.
func (w *Watcher) notifyChange() {
	w.mu.RLock()
	started := w.started
	w.mu.RUnlock()

	// Don't notify if watcher has been stopped - avoid calling callbacks
	// after Stop() has been called. This is best-effort; there's a small
	// race window, but callbacks are idempotent so it's harmless.
	if !started {
		return
	}

	w.onChange()

	// Non-blocking send to change channel
	select {
	case w.changeCh <- struct{}{}:
	default:
	}
}
