package tail

import (
	"fmt"
	"io"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// Format selects the wire shape for emitted events.
type Format int

const (
	// FormatHuman emits one colorless line per event, roughly matching
	// the TUI notification center formatting. Default when stdout is a TTY.
	FormatHuman Format = iota
	// FormatJSONL emits one newline-delimited JSON object per event. This
	// is the robot-consumable default for pipelines and Monitor attach.
	FormatJSONL
	// FormatJSON is equivalent to FormatJSONL for streaming purposes; the
	// Stream emits one object per event either way. Separate constant so
	// `--robot-format json` parses cleanly without forcing users to learn
	// JSONL terminology.
	FormatJSON
	// FormatCompact emits a terse one-line record:
	//   <kind> <bead_id> <actor> <summary>
	// useful for shell one-liners that don't need the full schema.
	FormatCompact
)

// ParseFormat maps a --robot-format value to a Format. Unknown values return
// an error rather than silently degrading.
func ParseFormat(raw string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "human":
		return FormatHuman, nil
	case "jsonl":
		return FormatJSONL, nil
	case "json":
		return FormatJSON, nil
	case "compact":
		return FormatCompact, nil
	}
	return 0, fmt.Errorf("unknown --robot-format %q (valid: human, jsonl, json, compact)", raw)
}

// wireEvent is the stable JSON shape emitted for --robot-format json/jsonl.
// Adding fields is backwards-compatible; renames are not. Mirrors the schema
// documented in the bt-yhe6 bead body.
type wireEvent struct {
	EventID string `json:"event_id"`
	Kind    string `json:"kind"`
	BeadID  string `json:"bead_id"`
	Repo    string `json:"repo"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Actor   string `json:"actor,omitempty"`
	At      string `json:"at"`
	Source  string `json:"source"`
}

func toWire(e events.Event) wireEvent {
	return wireEvent{
		EventID: e.ID,
		Kind:    e.Kind.String(),
		BeadID:  e.BeadID,
		Repo:    e.Repo,
		Title:   e.Title,
		Summary: e.Summary,
		Actor:   e.Actor,
		At:      e.At.UTC().Format(time.RFC3339Nano),
		Source:  e.Source.String(),
	}
}

// WriteEvent renders e to w under fmt. Each call writes exactly one record
// plus a trailing newline so streamed consumers can line-buffer.
func WriteEvent(w io.Writer, fmtCode Format, e events.Event) error {
	switch fmtCode {
	case FormatJSONL, FormatJSON:
		buf, err := json.Marshal(toWire(e))
		if err != nil {
			return fmt.Errorf("encoding event %s: %w", e.ID, err)
		}
		buf = append(buf, '\n')
		_, err = w.Write(buf)
		return err
	case FormatCompact:
		actor := e.Actor
		if actor == "" {
			actor = "-"
		}
		_, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Kind.String(), e.BeadID, actor, e.Summary)
		return err
	case FormatHuman:
		return writeHuman(w, e)
	}
	return fmt.Errorf("unknown format code %d", fmtCode)
}

func writeHuman(w io.Writer, e events.Event) error {
	ts := e.At.Local().Format("15:04:05")
	actor := e.Actor
	if actor != "" {
		actor = " [" + actor + "]"
	}
	title := e.Title
	if title == "" {
		title = e.Summary
	}
	switch e.Kind {
	case events.EventBulk:
		_, err := fmt.Fprintf(w, "%s  bulk  %s\n", ts, e.Summary)
		return err
	default:
		_, err := fmt.Fprintf(w, "%s  %-9s  %-16s%s  %s\n", ts, e.Kind.String(), e.BeadID, actor, title)
		return err
	}
}
