// pkg/ui/events/persist.go
// Append-only JSONL persistence for the notifications ring buffer.
// One event per line, written through on Append/AppendMany when the
// owning RingBuffer has a persist path set. Hydrate() on boot replays
// the file (filtered by max age) into an empty buffer.
//
// Failures during write are logged via pkg/debug and never crash the
// caller — the in-memory ring is the source of truth for the live
// session. Failures during load are returned to the caller so it can
// decide whether to start fresh or surface the error.
//
// On-disk format: the Event struct's default Go JSON encoding. Field
// renames in Event become breaking changes for existing on-disk files;
// readers will silently drop fields that no longer exist, and writers
// will produce files older readers can't fully reconstruct. Acceptable
// for this single-user per-machine store; revisit if the persistence
// surface ever escapes the local machine.
package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultModalDisplayAge is the maximum event age loaded from disk into
// the in-memory ring buffer at TUI boot. Events older than this are
// skipped at hydration time so the modal doesn't display years of
// history. Matches the rough "last week of activity" expectation users
// carry into the tab.
//
// This is a TUI-only display window, NOT a retention policy. The
// on-disk file (~/.bt/events.jsonl) is append-only with no expiry —
// long-horizon consumers (e.g. bt robot activity for "what did we do
// september 2025" queries) should call LoadPersisted(path, 0) to
// bypass the filter and read all persisted events regardless of age.
//
// Three layered windows in this package:
//   - On-disk file: unbounded, append-only.
//   - Hydration window (this constant): TUI boot-time filter, default 7 days.
//   - In-memory ring (DefaultCapacity in ring.go): runtime cap of 500 events.
const DefaultModalDisplayAge = 7 * 24 * time.Hour

// DefaultPersistPath returns the canonical user-global path for the
// notifications JSONL store: ~/.bt/events.jsonl. Returns ("", err) when
// the user home directory cannot be resolved.
//
// The directory is NOT created here — the caller (or the persister on
// first write) takes care of mkdir.
func DefaultPersistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".bt", "events.jsonl"), nil
}

// LoadPersisted reads events from a JSONL file at path, dropping any
// whose At is older than now-maxAge. Returns the surviving events
// oldest-first (matching the on-disk order).
//
// Pass maxAge=0 to disable the age filter and return every persisted
// event regardless of age — the long-horizon read path for callers
// like bt robot activity that need the full append-only history rather
// than the modal hydration window.
//
// Missing file is not an error: returns an empty slice and nil. Corrupt
// individual lines are skipped silently — the goal is to recover what
// we can rather than refuse to start. A complete read failure (e.g.
// permission denied on the file itself) IS returned.
func LoadPersisted(path string, maxAge time.Duration) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	cutoff := time.Now().Add(-maxAge)
	var out []Event
	scanner := bufio.NewScanner(f)
	// Allow long lines for events with verbose summaries (comment text
	// up to commentSummaryLimit + JSON envelope overhead). 1 MiB cap is
	// generous and keeps us well clear of pathological lines that would
	// indicate a corrupt file.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // corrupt line, drop and keep going
		}
		if maxAge > 0 && e.At.Before(cutoff) {
			continue
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		return out, fmt.Errorf("scan events file: %w", err)
	}
	return out, nil
}

// filePersister append-writes events to a JSONL file. Owned 1:1 by a
// RingBuffer; not exported because callers configure persistence via
// RingBuffer.SetPersistPath rather than constructing one of these
// directly.
type filePersister struct {
	mu   sync.Mutex
	path string
}

// appendOne writes a single event as a JSON line. Creates the parent
// directory on first write. Returns an error so the caller can decide
// whether to surface it; callers in the Append hot path swallow errors
// after logging.
func (p *filePersister) appendOne(e Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writeLines([]Event{e})
}

// appendMany writes a batch of events. Cheaper than calling appendOne
// in a loop because it amortizes the open/close and the mkdir check.
func (p *filePersister) appendMany(events []Event) error {
	if len(events) == 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writeLines(events)
}

func (p *filePersister) writeLines(events []Event) error {
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return fmt.Errorf("mkdir events dir: %w", err)
	}
	f, err := os.OpenFile(p.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
	}
	return nil
}
