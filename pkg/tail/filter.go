// Package tail implements a headless live bead event stream. It reuses
// pkg/ui/events for the Event type, RingBuffer, and snapshot-diff logic,
// and exposes a poll-loop consumer that writes events to an io.Writer
// for CLI subscribers (`bt tail`).
//
// The package has no Bubble Tea dependency; it is safe to consume from
// any host (CLI, daemon, tests). The TUI consumes events through its own
// path — this package does not participate in TUI state transitions.
package tail

import (
	"fmt"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// Filter selects which events a Stream emits. Zero value matches everything.
// Fields are ANDed; within a field slice, entries are ORed. Filter state is
// immutable after Stream.Run — construct a new one to change selection.
type Filter struct {
	// BeadIDs restricts events to those whose BeadID is in the set.
	// Empty means "no bead filter".
	BeadIDs []string

	// Epic is an issue ID whose children (via parent-child dependencies)
	// plus the epic itself are included. Combined with an explicit BeadIDs
	// set via union — agents can compose both filters without grep.
	Epic string

	// Kinds restricts events to the given kinds. Empty means "all kinds".
	Kinds []events.EventKind

	// Actor matches against Event.Actor. A leading '!' negates
	// (exclude-matching). Empty means "no actor filter". Multiple actors
	// are supported via comma-separated input upstream; this field holds
	// a single normalized matcher set derived by ParseActor.
	ActorMatchers []ActorMatcher
}

// ActorMatcher is one equality check against Event.Actor. If Negate is true,
// the event matches when Actor != Want (and Want is non-empty).
type ActorMatcher struct {
	Want   string
	Negate bool
}

// ParseActor normalizes a raw --actor CLI value into a set of matchers.
// Accepts comma-separated lists; each entry may be bare (include) or
// prefixed with '!' (exclude). Empty input yields a nil slice.
func ParseActor(raw string) []ActorMatcher {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]ActorMatcher, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		neg := false
		if strings.HasPrefix(p, "!") {
			neg = true
			p = p[1:]
		}
		out = append(out, ActorMatcher{Want: p, Negate: neg})
	}
	return out
}

// ParseKinds normalizes a comma-separated kind list ("created,closed") into
// the event.EventKind slice used by Filter.Kinds. Unknown names yield an
// error — callers surface it to the user rather than silently dropping.
func ParseKinds(raw string) ([]events.EventKind, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]events.EventKind, 0, len(parts))
	seen := make(map[events.EventKind]bool, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		k, ok := kindFromString(p)
		if !ok {
			return nil, fmt.Errorf("unknown event kind %q (valid: created, edited, closed, commented, bulk)", p)
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}

func kindFromString(s string) (events.EventKind, bool) {
	switch s {
	case "created":
		return events.EventCreated, true
	case "edited":
		return events.EventEdited, true
	case "closed":
		return events.EventClosed, true
	case "commented":
		return events.EventCommented, true
	case "bulk":
		return events.EventBulk, true
	}
	return 0, false
}

// Match reports whether e passes the filter under the given epic children
// index. childrenOfEpic is the set of bead IDs whose parent-child chain
// rolls up to Filter.Epic (inclusive of the epic itself). When Filter.Epic
// is empty, the caller may pass nil.
func (f Filter) Match(e events.Event, childrenOfEpic map[string]struct{}) bool {
	if len(f.Kinds) > 0 {
		hit := false
		for _, k := range f.Kinds {
			if k == e.Kind {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	// Bead-ID selection is the union of explicit BeadIDs and the epic set.
	// When both are empty the bead-ID axis is unconstrained.
	if len(f.BeadIDs) > 0 || len(childrenOfEpic) > 0 {
		hit := false
		for _, id := range f.BeadIDs {
			if id == e.BeadID {
				hit = true
				break
			}
		}
		if !hit {
			_, hit = childrenOfEpic[e.BeadID]
		}
		if !hit {
			return false
		}
	}
	if len(f.ActorMatchers) > 0 {
		includes := make([]ActorMatcher, 0, len(f.ActorMatchers))
		excludes := make([]ActorMatcher, 0, len(f.ActorMatchers))
		for _, m := range f.ActorMatchers {
			if m.Negate {
				excludes = append(excludes, m)
			} else {
				includes = append(includes, m)
			}
		}
		for _, m := range excludes {
			if e.Actor == m.Want {
				return false
			}
		}
		if len(includes) > 0 {
			hit := false
			for _, m := range includes {
				if e.Actor == m.Want {
					hit = true
					break
				}
			}
			if !hit {
				return false
			}
		}
	}
	return true
}

// EpicChildren returns the set of issue IDs that transitively roll up to
// epic via parent-child dependencies, including epic itself. Cycles (which
// are illegal but possible in corrupted data) are tolerated via a visited
// set. When epic is empty, the returned map is nil.
func EpicChildren(epic string, issues []model.Issue) map[string]struct{} {
	if epic == "" {
		return nil
	}
	// Build a reverse index: parent -> children.
	children := make(map[string][]string, len(issues))
	for _, iss := range issues {
		for _, dep := range iss.Dependencies {
			if dep == nil || dep.Type != model.DepParentChild {
				continue
			}
			children[dep.DependsOnID] = append(children[dep.DependsOnID], iss.ID)
		}
	}
	out := make(map[string]struct{})
	var walk func(id string)
	walk = func(id string) {
		if _, seen := out[id]; seen {
			return
		}
		out[id] = struct{}{}
		for _, c := range children[id] {
			walk(c)
		}
	}
	walk(epic)
	return out
}
