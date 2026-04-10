package datasource

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// SourceWatcher monitors data sources for changes and triggers callbacks
type SourceWatcher struct {
	watcher     *fsnotify.Watcher
	sources     []DataSource
	callback    func(changed DataSource)
	debounce    time.Duration
	lastChange  map[string]time.Time
	mu          sync.Mutex
	done        chan struct{}
	verbose     bool
	logger      func(msg string)
}

// WatcherOptions configures the source watcher
type WatcherOptions struct {
	// Debounce is the minimum time between callbacks for the same file
	// Default: 100ms
	Debounce time.Duration
	// Verbose enables detailed logging
	Verbose bool
	// Logger receives log messages when Verbose is true
	Logger func(msg string)
}

// DefaultWatcherOptions returns sensible default watcher options
func DefaultWatcherOptions() WatcherOptions {
	return WatcherOptions{
		Debounce: 100 * time.Millisecond,
		Verbose:  false,
		Logger:   func(string) {},
	}
}

// NewSourceWatcher creates a watcher for the given sources
func NewSourceWatcher(sources []DataSource, callback func(DataSource), opts WatcherOptions) (*SourceWatcher, error) {
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}
	if opts.Debounce == 0 {
		opts.Debounce = 100 * time.Millisecond
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	sw := &SourceWatcher{
		watcher:    watcher,
		sources:    sources,
		callback:   callback,
		debounce:   opts.Debounce,
		lastChange: make(map[string]time.Time),
		done:       make(chan struct{}),
		verbose:    opts.Verbose,
		logger:     opts.Logger,
	}

	// Add all source paths to the watcher
	for _, source := range sources {
		if err := watcher.Add(source.Path); err != nil {
			if opts.Verbose {
				opts.Logger(fmt.Sprintf("Cannot watch %s: %v", source.Path, err))
			}
			// Continue with other sources
		} else if opts.Verbose {
			opts.Logger(fmt.Sprintf("Watching: %s", source.Path))
		}
	}

	return sw, nil
}

// Start begins watching for file changes
func (sw *SourceWatcher) Start() {
	go sw.run()
}

// Stop stops watching for file changes
func (sw *SourceWatcher) Stop() {
	close(sw.done)
	sw.watcher.Close()
}

// run is the main event loop for the watcher
func (sw *SourceWatcher) run() {
	for {
		select {
		case <-sw.done:
			return

		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}

			// Only care about write and create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Find matching source
			var changedSource *DataSource
			for i := range sw.sources {
				if sw.sources[i].Path == event.Name {
					changedSource = &sw.sources[i]
					break
				}
			}

			if changedSource == nil {
				continue
			}

			// Check debounce
			sw.mu.Lock()
			lastTime := sw.lastChange[event.Name]
			now := time.Now()
			if now.Sub(lastTime) < sw.debounce {
				sw.mu.Unlock()
				continue
			}
			sw.lastChange[event.Name] = now
			sw.mu.Unlock()

			if sw.verbose {
				sw.logger(fmt.Sprintf("Source changed: %s", event.Name))
			}

			// Refresh source info
			if err := RefreshSourceInfo(changedSource); err != nil {
				if sw.verbose {
					sw.logger(fmt.Sprintf("Failed to refresh source info: %v", err))
				}
			}

			// Call the callback
			if sw.callback != nil {
				sw.callback(*changedSource)
			}

		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			if sw.verbose {
				sw.logger(fmt.Sprintf("Watcher error: %v", err))
			}
		}
	}
}

// AddSource adds a new source to watch
func (sw *SourceWatcher) AddSource(source DataSource) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Check if already watching
	for _, s := range sw.sources {
		if s.Path == source.Path {
			return nil
		}
	}

	if err := sw.watcher.Add(source.Path); err != nil {
		return fmt.Errorf("failed to watch %s: %w", source.Path, err)
	}

	sw.sources = append(sw.sources, source)
	if sw.verbose {
		sw.logger(fmt.Sprintf("Added watch: %s", source.Path))
	}

	return nil
}

// RemoveSource stops watching a source
func (sw *SourceWatcher) RemoveSource(path string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if err := sw.watcher.Remove(path); err != nil {
		return fmt.Errorf("failed to remove watch %s: %w", path, err)
	}

	// Remove from sources list
	for i, s := range sw.sources {
		if s.Path == path {
			sw.sources = append(sw.sources[:i], sw.sources[i+1:]...)
			break
		}
	}

	delete(sw.lastChange, path)
	if sw.verbose {
		sw.logger(fmt.Sprintf("Removed watch: %s", path))
	}

	return nil
}

// Sources returns the list of watched sources
func (sw *SourceWatcher) Sources() []DataSource {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	result := make([]DataSource, len(sw.sources))
	copy(result, sw.sources)
	return result
}

// AutoRefreshManager manages automatic source revalidation and reselection
type AutoRefreshManager struct {
	watcher        *SourceWatcher
	currentSource  *DataSource
	sources        []DataSource
	onSourceChange func(newSource DataSource, reason string)
	mu             sync.RWMutex
	opts           SelectionOptions
}

// AutoRefreshOptions configures the auto-refresh manager
type AutoRefreshOptions struct {
	// WatcherOptions for the underlying file watcher
	WatcherOptions WatcherOptions
	// SelectionOptions for source reselection
	SelectionOptions SelectionOptions
	// OnSourceChange is called when the active source changes
	OnSourceChange func(newSource DataSource, reason string)
}

// NewAutoRefreshManager creates a manager that automatically re-selects sources on changes
func NewAutoRefreshManager(sources []DataSource, opts AutoRefreshOptions) (*AutoRefreshManager, error) {
	manager := &AutoRefreshManager{
		sources:        sources,
		onSourceChange: opts.OnSourceChange,
		opts:           opts.SelectionOptions,
	}

	// Initial source selection
	selected, err := SelectBestSourceWithOptions(sources, opts.SelectionOptions)
	if err != nil {
		return nil, err
	}
	manager.currentSource = &selected

	// Create watcher
	watcher, err := NewSourceWatcher(sources, manager.handleChange, opts.WatcherOptions)
	if err != nil {
		return nil, err
	}
	manager.watcher = watcher

	return manager, nil
}

// Start begins automatic refresh monitoring
func (m *AutoRefreshManager) Start() {
	m.watcher.Start()
}

// Stop stops automatic refresh monitoring
func (m *AutoRefreshManager) Stop() {
	m.watcher.Stop()
}

// CurrentSource returns the currently selected source
func (m *AutoRefreshManager) CurrentSource() DataSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentSource == nil {
		return DataSource{}
	}
	return *m.currentSource
}

// handleChange is called when any source changes
func (m *AutoRefreshManager) handleChange(changed DataSource) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-validate the changed source
	for i := range m.sources {
		if m.sources[i].Path == changed.Path {
			m.sources[i].ModTime = changed.ModTime
			m.sources[i].Size = changed.Size
			ValidateSource(&m.sources[i])
			break
		}
	}

	// Re-select best source
	newSelected, err := SelectBestSourceWithOptions(m.sources, m.opts)
	if err != nil {
		return
	}

	// Check if selection changed
	if m.currentSource != nil && m.currentSource.Path == newSelected.Path &&
		m.currentSource.ModTime.Equal(newSelected.ModTime) {
		return
	}

	oldPath := ""
	if m.currentSource != nil {
		oldPath = m.currentSource.Path
	}

	m.currentSource = &newSelected

	// Notify callback
	if m.onSourceChange != nil {
		reason := "source updated"
		if oldPath != newSelected.Path {
			reason = fmt.Sprintf("switched from %s", oldPath)
		}
		m.onSourceChange(newSelected, reason)
	}
}

// ForceRefresh triggers a manual refresh of all sources
func (m *AutoRefreshManager) ForceRefresh() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Refresh and revalidate all sources
	for i := range m.sources {
		RefreshSourceInfo(&m.sources[i])
		ValidateSource(&m.sources[i])
	}

	// Re-select
	newSelected, err := SelectBestSourceWithOptions(m.sources, m.opts)
	if err != nil {
		return err
	}

	if m.currentSource == nil || m.currentSource.Path != newSelected.Path {
		m.currentSource = &newSelected
		if m.onSourceChange != nil {
			m.onSourceChange(newSelected, "force refresh")
		}
	}

	return nil
}
