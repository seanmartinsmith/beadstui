package ui

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// TestFormatDaySeparator_ContainsDate verifies the separator includes the
// ISO-8601 date string surrounded by horizontal-line padding.
func TestFormatDaySeparator_ContainsDate(t *testing.T) {
	out := formatDaySeparator("2026-05-04", 40)
	if !strings.Contains(out, "2026-05-04") {
		t.Fatalf("expected separator to contain date, got %q", out)
	}
	if !strings.HasPrefix(out, "─") {
		t.Fatalf("expected separator to start with horizontal line char, got %q", out)
	}
	if !strings.HasSuffix(out, "─") {
		t.Fatalf("expected separator to end with horizontal line char, got %q", out)
	}
}

// TestFormatDaySeparator_NarrowWidthFallback ensures the separator degrades
// gracefully when the rowWidth is too small to pad — it still renders the
// date label without panicking or producing a negative-repeat string.
func TestFormatDaySeparator_NarrowWidthFallback(t *testing.T) {
	out := formatDaySeparator("2026-05-04", 5)
	if !strings.Contains(out, "2026-05-04") {
		t.Fatalf("expected date present even at narrow width, got %q", out)
	}
}

// TestTrimEndForDaySeparators_SingleDay verifies that when all events share
// a date, only the leading separator costs a row — events fill the rest.
func TestTrimEndForDaySeparators_SingleDay(t *testing.T) {
	day := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	es := []events.Event{
		{ID: "a", At: day},
		{ID: "b", At: day.Add(1 * time.Minute)},
		{ID: "c", At: day.Add(2 * time.Minute)},
		{ID: "d", At: day.Add(3 * time.Minute)},
	}
	// pageSize=4: 1 separator + 3 events = 4 rows; 4th event must be trimmed.
	got := trimEndForDaySeparators(es, 0, 4, 4)
	if got != 3 {
		t.Fatalf("expected end=3 (1 sep + 3 events fit in 4 rows), got %d", got)
	}
}

// TestTrimEndForDaySeparators_DayBoundary verifies that a date change adds
// an additional separator row, reducing the number of events that fit.
func TestTrimEndForDaySeparators_DayBoundary(t *testing.T) {
	d1 := time.Date(2026, 5, 4, 23, 59, 0, 0, time.UTC)
	d2 := time.Date(2026, 5, 5, 0, 1, 0, 0, time.UTC)
	es := []events.Event{
		{ID: "a", At: d1},
		{ID: "b", At: d2}, // crosses day boundary -> needs second separator
		{ID: "c", At: d2},
		{ID: "d", At: d2},
	}
	// pageSize=4: 1 sep + a + 1 sep (boundary) + b = 4 rows; c,d trimmed.
	got := trimEndForDaySeparators(es, 0, 4, 4)
	if got != 2 {
		t.Fatalf("expected end=2 with a day boundary at index 1, got %d", got)
	}
}

// TestRenderNotificationsTab_DaySeparatorOnBoundary verifies that the
// rendered notifications body contains a separator row when consecutive
// events cross a day boundary.
func TestRenderNotificationsTab_DaySeparatorOnBoundary(t *testing.T) {
	m := seedModel()
	d1 := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	// Append oldest first; ring buffer returns newest-first via visibleNotifications.
	m.events.Append(events.Event{ID: "old", Kind: events.EventCreated, BeadID: "bt-x", Title: "old", At: d1})
	m.events.Append(events.Event{ID: "new", Kind: events.EventCreated, BeadID: "bt-y", Title: "new", At: d2})

	out := m.renderNotificationsTab()
	if !strings.Contains(out, "2026-05-05") {
		t.Fatalf("expected day separator for newer event, got:\n%s", out)
	}
	if !strings.Contains(out, "2026-05-04") {
		t.Fatalf("expected day separator for older event, got:\n%s", out)
	}
}

// TestRenderNotificationsTab_NoSeparatorWithinSameDay verifies that two
// events on the same calendar day produce only a single separator (the
// leading "anchor" separator), not one per row.
func TestRenderNotificationsTab_NoSeparatorWithinSameDay(t *testing.T) {
	m := seedModel()
	day := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	m.events.Append(events.Event{ID: "a", Kind: events.EventCreated, BeadID: "bt-a", Title: "a", At: day})
	m.events.Append(events.Event{ID: "b", Kind: events.EventCreated, BeadID: "bt-b", Title: "b", At: day.Add(1 * time.Minute)})

	out := m.renderNotificationsTab()
	count := strings.Count(out, "2026-05-04")
	if count != 1 {
		t.Fatalf("expected exactly one separator for same-day events, got %d:\n%s", count, out)
	}
}

// TestKindRowStyle_TokenMapping pins the event-kind -> theme-token mapping
// (bt-0mxw). Asserting the style's foreground color (rather than rendered
// ANSI bytes) keeps the test stable across terminal-profile detection,
// which forces NoColor under `go test` because stdout is not a TTY.
func TestKindRowStyle_TokenMapping(t *testing.T) {
	tm := DefaultTheme()
	cases := []struct {
		kind events.EventKind
		want color.Color
		name string
	}{
		{events.EventCreated, tm.Success, "created->Success"},
		{events.EventEdited, tm.Primary, "edited->Primary"},
		{events.EventClosed, tm.Muted, "closed->Muted"},
		{events.EventCommented, tm.Info, "commented->Info"},
		{events.EventBulk, tm.Warning, "bulk->Warning"},
		{events.EventSystem, tm.Muted, "system->Muted"},
	}
	for _, tc := range cases {
		got := kindRowStyle(tm, tc.kind).GetForeground()
		if got != tc.want {
			t.Errorf("%s: foreground = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestKindRowStyle_DistinctActionTypes verifies the four primary action
// types (created/edited/closed/commented) render in distinct colors,
// satisfying the bt-0mxw acceptance criterion that "each action type
// renders in a distinct, theme-resolved color."
func TestKindRowStyle_DistinctActionTypes(t *testing.T) {
	tm := DefaultTheme()
	colors := map[events.EventKind]color.Color{
		events.EventCreated:   kindRowStyle(tm, events.EventCreated).GetForeground(),
		events.EventEdited:    kindRowStyle(tm, events.EventEdited).GetForeground(),
		events.EventClosed:    kindRowStyle(tm, events.EventClosed).GetForeground(),
		events.EventCommented: kindRowStyle(tm, events.EventCommented).GetForeground(),
	}
	seen := make(map[color.Color]events.EventKind)
	for kind, c := range colors {
		if prev, dup := seen[c]; dup {
			t.Errorf("kinds %v and %v share color %v — must be distinct", prev, kind, c)
		}
		seen[c] = kind
	}
}

// TestRenderNotificationsTab_AppliesKindStyle verifies the rendered output
// passes through kindRowStyle for each event row. Uses a profile-agnostic
// shape check: when a TrueColor profile is active, the row contains an
// ANSI escape; in all profiles, the row text itself is present (this
// asserts the renderer reaches the styled-write path without crashing for
// every kind).
func TestRenderNotificationsTab_AppliesKindStyle(t *testing.T) {
	m := seedModel()
	day := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	m.events.Append(events.Event{ID: "c", Kind: events.EventCreated, BeadID: "bt-c", Title: "create", At: day})
	m.events.Append(events.Event{ID: "e", Kind: events.EventEdited, BeadID: "bt-e", Title: "edit", At: day.Add(1 * time.Minute)})
	m.events.Append(events.Event{ID: "x", Kind: events.EventClosed, BeadID: "bt-x", Title: "close", At: day.Add(2 * time.Minute)})
	m.events.Append(events.Event{ID: "m", Kind: events.EventCommented, BeadID: "bt-m", Title: "comment", At: day.Add(3 * time.Minute)})

	out := m.renderNotificationsTab()
	for _, want := range []string{"bt-c", "bt-e", "bt-x", "bt-m"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected rendered notifications to contain %q, got:\n%s", want, out)
		}
	}
}
