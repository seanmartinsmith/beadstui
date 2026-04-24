package tail

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// LoaderFunc fetches the current issue snapshot. `bt tail` wires this to the
// project's active datasource (JSONL / SQLite / Dolt) via the same path the
// rest of bt uses — the tail package itself is datasource-agnostic.
type LoaderFunc func(context.Context) ([]model.Issue, error)

// Config parameterizes Stream.Run. All durations default to production
// values if left zero; see defaults below.
type Config struct {
	Loader       LoaderFunc
	Filter       Filter
	Format       Format
	Writer       io.Writer
	PollInterval time.Duration // default 1s
	IdleExit     time.Duration // 0 = never exit on idle
	SinceAgo     time.Duration // replay synthesis window (0 = no replay)
	Capacity     int           // ring buffer capacity (default events.DefaultCapacity)
	// Now overrides the clock for tests. Production leaves this nil.
	Now func() time.Time
}

const defaultPollInterval = time.Second

// Stream is a single consumer of the bead event feed. It owns a ring buffer
// (for parity with the TUI's retention semantics) and polls the loader until
// context cancellation or idle-exit.
type Stream struct {
	cfg    Config
	buffer *events.RingBuffer
	now    func() time.Time
}

// New constructs a Stream. Returns an error if required config is missing.
func New(cfg Config) (*Stream, error) {
	if cfg.Loader == nil {
		return nil, fmt.Errorf("tail: Config.Loader required")
	}
	if cfg.Writer == nil {
		return nil, fmt.Errorf("tail: Config.Writer required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = events.DefaultCapacity
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Stream{
		cfg:    cfg,
		buffer: events.NewRingBuffer(cfg.Capacity),
		now:    now,
	}, nil
}

// Buffer exposes the ring buffer for introspection and testing. Callers in
// production do not need this; kept public to let future tooling (e.g. a
// snapshot-on-exit flag) read retained events.
func (s *Stream) Buffer() *events.RingBuffer { return s.buffer }

// Run blocks until ctx is cancelled or (if IdleExit > 0) no events have been
// emitted for IdleExit. The initial snapshot establishes the prior state for
// live diffs; if SinceAgo > 0, synthetic replay events for beads with recent
// UpdatedAt/ClosedAt/last-comment timestamps are emitted before the loop
// starts. Replay synthesis is labeled in the event Source so consumers can
// distinguish "just happened" from "happened before I attached".
func (s *Stream) Run(ctx context.Context) error {
	prior, err := s.cfg.Loader(ctx)
	if err != nil {
		return fmt.Errorf("tail: initial load: %w", err)
	}

	childrenOfEpic := EpicChildren(s.cfg.Filter.Epic, prior)

	if s.cfg.SinceAgo > 0 {
		cutoff := s.now().Add(-s.cfg.SinceAgo)
		replay := synthesizeReplay(prior, cutoff)
		if err := s.emit(replay, childrenOfEpic); err != nil {
			return err
		}
	}

	lastEmit := s.now()
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		next, err := s.cfg.Loader(ctx)
		if err != nil {
			// Transient load errors don't kill the stream — log semantics
			// are caller's responsibility (writer is the event stream, not
			// a log). A future --verbose could surface them on stderr.
			continue
		}

		diff := events.Diff(prior, next, s.now(), events.SourceDolt)
		prior = next

		// Rebuild epic children each poll — parent-child edges can change
		// as the project evolves. Cheap: O(N * deps) scan.
		childrenOfEpic = EpicChildren(s.cfg.Filter.Epic, next)

		emitted, err := s.emitCount(diff, childrenOfEpic)
		if err != nil {
			return err
		}
		if emitted > 0 {
			lastEmit = s.now()
		}

		if s.cfg.IdleExit > 0 && s.now().Sub(lastEmit) >= s.cfg.IdleExit {
			return nil
		}
	}
}

func (s *Stream) emit(evs []events.Event, children map[string]struct{}) error {
	_, err := s.emitCount(evs, children)
	return err
}

func (s *Stream) emitCount(evs []events.Event, children map[string]struct{}) (int, error) {
	n := 0
	for _, e := range evs {
		s.buffer.Append(e)
		if !s.cfg.Filter.Match(e, children) {
			continue
		}
		if err := WriteEvent(s.cfg.Writer, s.cfg.Format, e); err != nil {
			return n, fmt.Errorf("writing event %s: %w", e.ID, err)
		}
		// Flush per-event so attach-to-Monitor consumers see push latency.
		if f, ok := s.cfg.Writer.(interface{ Sync() error }); ok {
			_ = f.Sync()
		}
		n++
	}
	return n, nil
}

// synthesizeReplay walks the current snapshot and emits pseudo-events for
// beads whose recent activity falls within the replay window. Synthesis
// rules (per bead, latest-wins):
//   - CreatedAt >= cutoff        -> EventCreated
//   - ClosedAt  >= cutoff        -> EventClosed
//   - last comment >= cutoff     -> EventCommented
//   - UpdatedAt >= cutoff        -> EventEdited (generic)
//
// Output is sorted oldest-first so consumers see a chronological catch-up
// before the live tail begins. Replay events carry the same ID space as
// live events (computed from BeadID+Kind+timestamp) so dedup still works.
func synthesizeReplay(issues []model.Issue, cutoff time.Time) []events.Event {
	var out []events.Event
	for _, iss := range issues {
		// Created
		if !iss.CreatedAt.IsZero() && !iss.CreatedAt.Before(cutoff) {
			out = append(out, syntheticEvent(iss, events.EventCreated, iss.CreatedAt, iss.Title))
		}
		// Closed
		if iss.ClosedAt != nil && !iss.ClosedAt.Before(cutoff) {
			out = append(out, syntheticEvent(iss, events.EventClosed, *iss.ClosedAt, iss.Title))
			continue
		}
		// Latest comment
		if len(iss.Comments) > 0 {
			latest := iss.Comments[len(iss.Comments)-1]
			if latest != nil && !latest.CreatedAt.IsZero() && !latest.CreatedAt.Before(cutoff) {
				out = append(out, syntheticEvent(iss, events.EventCommented, latest.CreatedAt, truncate(latest.Text, 80)))
				continue
			}
		}
		// Generic edit
		if !iss.UpdatedAt.IsZero() && !iss.UpdatedAt.Before(cutoff) && iss.UpdatedAt != iss.CreatedAt {
			out = append(out, syntheticEvent(iss, events.EventEdited, iss.UpdatedAt, "replay"))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

func syntheticEvent(iss model.Issue, kind events.EventKind, at time.Time, summary string) events.Event {
	return events.Event{
		ID:      syntheticID(iss.ID, kind, at),
		Kind:    kind,
		BeadID:  iss.ID,
		Repo:    repoFromID(iss.ID),
		Title:   iss.Title,
		Summary: summary,
		Actor:   iss.Assignee,
		At:      at,
		Source:  events.SourceDolt,
	}
}

func syntheticID(beadID string, kind events.EventKind, at time.Time) string {
	return fmt.Sprintf("replay-%s-%s-%d", beadID, kind.String(), at.UnixNano())
}

func repoFromID(id string) string {
	for i, r := range id {
		if r == '-' {
			return id[:i]
		}
	}
	return id
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return string(r[:n])
	}
	return string(r[:n-3]) + "..."
}
