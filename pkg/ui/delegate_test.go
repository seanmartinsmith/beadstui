package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	"charm.land/bubbles/v2/list"
)

// Build a minimal issue item used across delegate tests.
func newTestIssueItem(id string) IssueItem {
	now := time.Now().Add(-2 * time.Hour) // deterministic-ish age string (e.g. "2h")
	return IssueItem{
		Issue: model.Issue{
			ID:        id,
			Title:     "Short title for testing",
			Status:    model.StatusOpen,
			IssueType: model.TypeFeature,
			Priority:  1,
			Assignee:  "alice",
			Labels:    []string{"one", "two"},
			Comments: []*model.Comment{
				{ID: "1", IssueID: id, Author: "bob", Text: "hello", CreatedAt: now},
			},
			CreatedAt: now,
		},
		DiffStatus: DiffStatusNone,
		RepoPrefix: "",
	}
}

func TestIssueDelegate_RenderWorkspaceWithPriorityHints(t *testing.T) {
	item := newTestIssueItem("api-123")
	item.RepoPrefix = "api"         // exercise workspace badge branch
	item.DiffStatus = DiffStatusNew // exercise diff badge branch
	theme := DefaultTheme()

	delegate := IssueDelegate{
		Theme:             theme,
		ShowPriorityHints: true,
		PriorityHints: map[string]*analysis.PriorityRecommendation{
			item.Issue.ID: {IssueID: item.Issue.ID, Direction: "increase"},
		},
		WorkspaceMode: true,
	}

	items := []list.Item{item}
	l := list.New(items, delegate, 0, 0)
	l.SetWidth(120) // wide enough to render right-side columns

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if !strings.Contains(out, "api-123") {
		t.Fatalf("render output missing issue id: %q", out)
	}
	if !strings.Contains(out, "↑") {
		t.Fatalf("render output missing priority hint arrow: %q", out)
	}
	if !strings.Contains(out, "[API]") {
		t.Fatalf("render output missing repo badge [API]: %q", out)
	}
	if !strings.Contains(out, "🆕") {
		t.Fatalf("render output missing diff badge for new item: %q", out)
	}
	if !strings.Contains(out, "💬1") {
		t.Fatalf("render output missing comment count badge: %q", out)
	}
}

func TestIssueDelegate_RenderFallsBackWidthAndNoPanic(t *testing.T) {
	item := newTestIssueItem("TASK-1")
	theme := DefaultTheme()
	delegate := IssueDelegate{Theme: theme}

	l := list.New([]list.Item{item}, delegate, 0, 0) // width defaults to 0 → delegate fallback

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if out == "" {
		t.Fatal("render output should not be empty")
	}
	if !strings.Contains(out, "TASK-1") {
		t.Fatalf("render output missing id after fallback width handling: %q", out)
	}
}

func TestIssueDelegate_RenderUltraWide(t *testing.T) {
	item := newTestIssueItem("WIDE-1")
	// Assignee and Labels require width thresholds >100 and >140
	theme := DefaultTheme()
	delegate := IssueDelegate{Theme: theme}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(160) // Ultra-wide

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if !strings.Contains(out, "@alice") {
		t.Fatalf("ultra-wide output missing assignee @alice: %q", out)
	}
	if !strings.Contains(out, "one,two") { // joined labels
		t.Fatalf("ultra-wide output missing labels 'one,two': %q", out)
	}
}

// Author column renders at width > 120 when Author differs from Assignee.
// Prefix (✎) + 10-char left-padded author ID. bt-aw4h.
func TestIssueDelegate_RenderShowsAuthor(t *testing.T) {
	item := newTestIssueItem("AUTH-1")
	item.Issue.Author = "bt-7d42e" // shorthand session ID
	theme := DefaultTheme()
	delegate := IssueDelegate{Theme: theme}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(140) // > 120 threshold

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if !strings.Contains(out, "bt-7d42e") {
		t.Fatalf("width=140 output should include author 'bt-7d42e': %q", out)
	}
	if !strings.Contains(out, "✎") {
		t.Fatalf("width=140 output should include author prefix ✎: %q", out)
	}
}

// Author == Assignee case: column is suppressed to avoid duplication.
func TestIssueDelegate_RenderSuppressesAuthorWhenSameAsAssignee(t *testing.T) {
	item := newTestIssueItem("SAME-1")
	item.Issue.Author = "alice" // matches Assignee from newTestIssueItem
	theme := DefaultTheme()
	delegate := IssueDelegate{Theme: theme}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(140)

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if strings.Contains(out, "✎") {
		t.Fatalf("author==assignee should NOT render author column: %q", out)
	}
}

// Author column hidden below width threshold.
func TestIssueDelegate_RenderHidesAuthorAtNarrowWidth(t *testing.T) {
	item := newTestIssueItem("NARR-AUTH-1")
	item.Issue.Author = "bt-7d42e"
	theme := DefaultTheme()
	delegate := IssueDelegate{Theme: theme}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(110) // between Assignee threshold (>100) and Author threshold (>120)

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if strings.Contains(out, "bt-7d42e") {
		t.Fatalf("width=110 should hide author column: %q", out)
	}
}

func TestIssueDelegate_RenderNarrow(t *testing.T) {
	item := newTestIssueItem("NARROW-1")
	theme := DefaultTheme()
	delegate := IssueDelegate{Theme: theme}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(50) // Very narrow

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if !strings.Contains(out, "NARROW-1") {
		t.Fatalf("narrow output missing id: %q", out)
	}
	// Should NOT contain right-side metadata
	if strings.Contains(out, "@alice") {
		t.Fatalf("narrow output should hide assignee: %q", out)
	}
	if strings.Contains(out, "💬") {
		t.Fatalf("narrow output should hide comments count: %q", out)
	}
}
