package tail

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

func TestParseActor(t *testing.T) {
	got := ParseActor("bt-abc,!bt-def, ,!bt-ghi")
	want := []ActorMatcher{
		{Want: "bt-abc", Negate: false},
		{Want: "bt-def", Negate: true},
		{Want: "bt-ghi", Negate: true},
	}
	if len(got) != len(want) {
		t.Fatalf("ParseActor length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ParseActor[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	if ParseActor("") != nil {
		t.Error("ParseActor(\"\") should be nil")
	}
}

func TestParseKinds(t *testing.T) {
	got, err := ParseKinds("commented,closed")
	if err != nil {
		t.Fatalf("ParseKinds: %v", err)
	}
	if len(got) != 2 || got[0] != events.EventCommented || got[1] != events.EventClosed {
		t.Errorf("ParseKinds = %v", got)
	}
	if _, err := ParseKinds("nonsense"); err == nil {
		t.Error("ParseKinds(nonsense) should error")
	}
}

func TestFilterMatch_ActorIncludeExclude(t *testing.T) {
	f := Filter{ActorMatchers: ParseActor("bt-abc,!bt-def")}
	if !f.Match(evt("bt-1", events.EventCommented, "bt-abc"), nil) {
		t.Error("bt-abc should match include")
	}
	if f.Match(evt("bt-1", events.EventCommented, "bt-def"), nil) {
		t.Error("bt-def should be excluded")
	}
	if f.Match(evt("bt-1", events.EventCommented, "bt-xyz"), nil) {
		t.Error("bt-xyz not in include set should fail")
	}
}

func TestFilterMatch_ActorExcludeOnly(t *testing.T) {
	f := Filter{ActorMatchers: ParseActor("!bt-self")}
	if f.Match(evt("bt-1", events.EventCommented, "bt-self"), nil) {
		t.Error("bt-self should be excluded")
	}
	if !f.Match(evt("bt-1", events.EventCommented, "bt-other"), nil) {
		t.Error("bt-other should pass when only exclude is set")
	}
	if !f.Match(evt("bt-1", events.EventCommented, ""), nil) {
		t.Error("empty actor should pass exclude-only filter")
	}
}

func TestFilterMatch_KindAndBead(t *testing.T) {
	f := Filter{
		BeadIDs: []string{"bt-1"},
		Kinds:   []events.EventKind{events.EventClosed},
	}
	if !f.Match(evt("bt-1", events.EventClosed, ""), nil) {
		t.Error("bt-1/closed should match")
	}
	if f.Match(evt("bt-1", events.EventEdited, ""), nil) {
		t.Error("bt-1/edited should fail kind filter")
	}
	if f.Match(evt("bt-2", events.EventClosed, ""), nil) {
		t.Error("bt-2/closed should fail bead filter")
	}
}

func TestEpicChildren(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-epic"},
		{ID: "bt-child1", Dependencies: []*model.Dependency{{DependsOnID: "bt-epic", Type: model.DepParentChild}}},
		{ID: "bt-child2", Dependencies: []*model.Dependency{{DependsOnID: "bt-epic", Type: model.DepParentChild}}},
		{ID: "bt-grandchild", Dependencies: []*model.Dependency{{DependsOnID: "bt-child1", Type: model.DepParentChild}}},
		{ID: "bt-unrelated"},
		{ID: "bt-related", Dependencies: []*model.Dependency{{DependsOnID: "bt-epic", Type: model.DepRelated}}},
	}
	got := EpicChildren("bt-epic", issues)
	for _, want := range []string{"bt-epic", "bt-child1", "bt-child2", "bt-grandchild"} {
		if _, ok := got[want]; !ok {
			t.Errorf("EpicChildren missing %s", want)
		}
	}
	for _, neg := range []string{"bt-unrelated", "bt-related"} {
		if _, ok := got[neg]; ok {
			t.Errorf("EpicChildren should not contain %s", neg)
		}
	}
	if EpicChildren("", issues) != nil {
		t.Error("empty epic should yield nil map")
	}
}

func TestStream_LiveDiffJSONL(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	initial := []model.Issue{{
		ID:        "bt-1",
		Title:     "first",
		Status:    model.StatusOpen,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}}
	afterCreate := append([]model.Issue{}, initial...)
	afterCreate = append(afterCreate, model.Issue{
		ID:        "bt-2",
		Title:     "second",
		Status:    model.StatusOpen,
		Assignee:  "bt-author",
		CreatedAt: now,
		UpdatedAt: now,
	})
	afterClose := make([]model.Issue, len(afterCreate))
	for i := range afterCreate {
		afterClose[i] = afterCreate[i].Clone()
	}
	closedAt := now.Add(time.Second)
	afterClose[1].Status = model.StatusClosed
	afterClose[1].ClosedAt = &closedAt

	snapshots := [][]model.Issue{initial, afterCreate, afterClose, afterClose}
	var (
		mu    sync.Mutex
		index int
	)
	loader := func(ctx context.Context) ([]model.Issue, error) {
		mu.Lock()
		defer mu.Unlock()
		s := snapshots[min(index, len(snapshots)-1)]
		index++
		return append([]model.Issue{}, s...), nil
	}

	buf := &bytes.Buffer{}
	s, err := New(Config{
		Loader:       loader,
		Filter:       Filter{},
		Format:       FormatJSONL,
		Writer:       buf,
		PollInterval: 5 * time.Millisecond,
		IdleExit:     200 * time.Millisecond,
		Now:          func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	lines := splitLines(buf.String())
	if len(lines) < 2 {
		t.Fatalf("want at least 2 events, got %d: %q", len(lines), buf.String())
	}

	kinds := make([]string, 0, len(lines))
	for _, ln := range lines {
		var w wireEvent
		if err := json.Unmarshal([]byte(ln), &w); err != nil {
			t.Fatalf("parse %q: %v", ln, err)
		}
		kinds = append(kinds, w.Kind)
	}
	if !contains(kinds, "created") {
		t.Errorf("expected 'created' event, got %v", kinds)
	}
	if !contains(kinds, "closed") {
		t.Errorf("expected 'closed' event, got %v", kinds)
	}
}

func TestStream_FilterSuppressesEvents(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	prior := []model.Issue{{ID: "bt-1", Title: "t1", Status: model.StatusOpen, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}}
	next := append([]model.Issue{}, prior[0])
	next2 := []model.Issue{
		prior[0],
		{ID: "bt-2", Title: "t2", Status: model.StatusOpen, Assignee: "bt-self", CreatedAt: now, UpdatedAt: now},
	}
	snapshots := [][]model.Issue{prior, next2, next2}
	var (
		mu sync.Mutex
		i  int
	)
	_ = next
	loader := func(ctx context.Context) ([]model.Issue, error) {
		mu.Lock()
		defer mu.Unlock()
		s := snapshots[min(i, len(snapshots)-1)]
		i++
		return append([]model.Issue{}, s...), nil
	}

	buf := &bytes.Buffer{}
	s, err := New(Config{
		Loader: loader,
		Filter: Filter{
			ActorMatchers: ParseActor("!bt-self"),
		},
		Format:       FormatJSONL,
		Writer:       buf,
		PollInterval: 5 * time.Millisecond,
		IdleExit:     150 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "" {
		t.Errorf("expected no events through exclude filter, got: %q", buf.String())
	}
	if s.Buffer().Len() == 0 {
		t.Errorf("ring buffer should still retain the event even if suppressed from writer")
	}
}

func TestStream_SinceReplaysRecentEdits(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	closedAt := now.Add(-30 * time.Second)
	issues := []model.Issue{
		{ID: "bt-recent", Title: "recent", Status: model.StatusClosed,
			CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-30 * time.Second),
			ClosedAt: &closedAt},
		{ID: "bt-stale", Title: "stale", Status: model.StatusOpen,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour)},
	}
	loader := func(ctx context.Context) ([]model.Issue, error) {
		return issues, nil
	}
	buf := &bytes.Buffer{}
	s, err := New(Config{
		Loader:       loader,
		Format:       FormatJSONL,
		Writer:       buf,
		PollInterval: 10 * time.Millisecond,
		IdleExit:     50 * time.Millisecond,
		SinceAgo:     2 * time.Minute,
		Now:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "bt-recent") {
		t.Errorf("expected bt-recent in replay output, got %q", buf.String())
	}
	if strings.Contains(buf.String(), "bt-stale") {
		t.Errorf("did not expect bt-stale (outside window) in output: %q", buf.String())
	}
}

func TestWriteEvent_Formats(t *testing.T) {
	e := events.Event{
		ID:      "abc",
		Kind:    events.EventCommented,
		BeadID:  "bt-1",
		Repo:    "bt",
		Title:   "hello",
		Summary: "a comment",
		Actor:   "bt-author",
		At:      time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		Source:  events.SourceDolt,
	}
	cases := []struct {
		name    string
		fmt     Format
		wantSub string
	}{
		{"jsonl", FormatJSONL, `"kind":"commented"`},
		{"json", FormatJSON, `"bead_id":"bt-1"`},
		{"compact", FormatCompact, "commented bt-1 bt-author a comment"},
		{"human", FormatHuman, "commented"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			if err := WriteEvent(buf, c.fmt, e); err != nil {
				t.Fatalf("WriteEvent: %v", err)
			}
			if !strings.Contains(buf.String(), c.wantSub) {
				t.Errorf("%s output missing %q: got %q", c.name, c.wantSub, buf.String())
			}
			if !strings.HasSuffix(buf.String(), "\n") {
				t.Errorf("%s output missing trailing newline: %q", c.name, buf.String())
			}
		})
	}
}

// helpers

func evt(bead string, kind events.EventKind, actor string) events.Event {
	return events.Event{
		ID:     "test-" + bead,
		Kind:   kind,
		BeadID: bead,
		Actor:  actor,
		At:     time.Now(),
	}
}

func splitLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
