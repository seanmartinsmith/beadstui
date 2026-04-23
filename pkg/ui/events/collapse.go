// pkg/ui/events/collapse.go
package events

import (
	"fmt"
	"sort"
	"time"
)

// CollapseForTicker returns a view of events where runs of the same
// BeadID + same Kind within the given window fold into their most
// recent event. The aggregated Summary reads "+ N fields" where N is
// the count of collapsed events (the original Summary is discarded
// because it is ambiguous in the collapsed case — the modal shows
// the raw events for full detail).
//
// Input ordering is preserved within non-collapsed groups. Output is
// ordered oldest-first.
//
// This is a pure function — the input slice is not modified, and
// dismissed events are NOT filtered out (callers filter before calling
// if they want to hide dismissed events).
func CollapseForTicker(events []Event, window time.Duration) []Event {
	if len(events) == 0 {
		return nil
	}

	// Sort a copy by (BeadID, Kind, At) so we can scan groups in order.
	sorted := make([]Event, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].BeadID != sorted[j].BeadID {
			return sorted[i].BeadID < sorted[j].BeadID
		}
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].At.Before(sorted[j].At)
	})

	var out []Event
	i := 0
	for i < len(sorted) {
		start := i
		// Advance while BeadID, Kind match AND the event is within
		// `window` of the group's first event.
		for i+1 < len(sorted) &&
			sorted[i+1].BeadID == sorted[start].BeadID &&
			sorted[i+1].Kind == sorted[start].Kind &&
			sorted[i+1].At.Sub(sorted[start].At) <= window {
			i++
		}
		if i == start {
			out = append(out, sorted[start])
		} else {
			// Emit the most recent event with aggregated Summary.
			latest := sorted[i]
			latest.Summary = fmt.Sprintf("+ %d fields", i-start+1)
			out = append(out, latest)
		}
		i++
	}

	// Return output in chronological order (oldest-first).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].At.Before(out[j].At)
	})
	return out
}
