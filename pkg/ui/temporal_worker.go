package ui

import (
	"log/slog"
	"time"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
)

// TemporalCacheReadyMsg is sent when the temporal cache has been populated
// or refreshed. The UI can use this to update sparklines, diff views, etc.
type TemporalCacheReadyMsg struct {
	SnapshotCount int
	Err           error
}

// startTemporalCacheLoop runs a background goroutine that populates
// the temporal cache on a slow cadence (default: hourly).
// Only runs in global Dolt mode where AS OF queries are available.
//
// The loop:
// 1. Waits briefly for the main data load to complete
// 2. Opens a fresh GlobalDoltReader for AS OF queries
// 3. Loads snapshots from 30 days ago to now at daily intervals
// 4. Sends TemporalCacheReadyMsg to the UI
// 5. Sleeps until the cache TTL expires, then repeats
func (w *BackgroundWorker) startTemporalCacheLoop() {
	if w.temporalCache == nil || w.dataSource == nil || w.dataSource.Type != datasource.SourceTypeDoltGlobal {
		return
	}

	// Initial population runs 5s after startup to let the main data
	// load complete first. This avoids competing for Dolt connections.
	initialDelay := time.NewTimer(5 * time.Second)
	defer initialDelay.Stop()

	select {
	case <-w.ctx.Done():
		return
	case <-initialDelay.C:
	}

	w.populateTemporalCache()

	ttl := w.temporalCache.TTL()
	ticker := time.NewTicker(ttl)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if w.temporalCache.IsStale() {
				w.populateTemporalCache()
			}
		}
	}
}

// populateTemporalCache loads historical snapshots via Dolt AS OF queries.
func (w *BackgroundWorker) populateTemporalCache() {
	reader, err := datasource.NewGlobalDoltReader(*w.dataSource)
	if err != nil {
		slog.Warn("temporal cache: cannot connect to shared server", "error", err)
		w.send(TemporalCacheReadyMsg{Err: err})
		return
	}
	defer reader.Close()

	now := time.Now().UTC().Truncate(24 * time.Hour)
	from := now.AddDate(0, 0, -30) // 30 days of history
	interval := 24 * time.Hour     // daily snapshots

	loaded, err := w.temporalCache.Populate(reader, from, now, interval)

	slog.Info("temporal cache populated",
		"loaded", loaded,
		"total", w.temporalCache.SnapshotCount(),
		"error", err)

	w.send(TemporalCacheReadyMsg{
		SnapshotCount: w.temporalCache.SnapshotCount(),
		Err:           err,
	})
}

// GetTemporalCache returns the temporal cache, or nil if not in global mode.
func (w *BackgroundWorker) GetTemporalCache() *analysis.TemporalCache {
	return w.temporalCache
}
