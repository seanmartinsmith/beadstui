package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
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

	if strings.Contains(out, "api-123") {
		t.Fatalf("render output should omit redundant repo prefix: %q", out)
	}
	if !strings.Contains(out, "123") {
		t.Fatalf("render output missing compact issue id: %q", out)
	}
	if !strings.Contains(out, "↑") {
		t.Fatalf("render output missing priority hint arrow: %q", out)
	}
	if !strings.Contains(out, "[API]") {
		t.Fatalf("render output missing repo badge [API]: %q", out)
	}
	if !strings.Contains(out, activeGlyphs.New) {
		t.Fatalf("render output missing diff badge for new item: %q", out)
	}
	if !strings.Contains(out, activeGlyphs.Comment+"1") {
		t.Fatalf("render output missing comment count badge: %q", out)
	}
}

func TestIssueDelegate_RenderSingleProjectUsesCompactIDWithoutBadge(t *testing.T) {
	item := newTestIssueItem("portfolio-hhg1r.1")
	item.RepoPrefix = "portfolio"
	delegate := IssueDelegate{Theme: DefaultTheme()}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(80)

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if strings.Contains(out, "portfolio-hhg1r.1") {
		t.Fatalf("render output should omit project prefix: %q", out)
	}
	if !strings.Contains(out, "hhg1r.1") {
		t.Fatalf("render output missing compact issue id: %q", out)
	}
	if strings.Contains(out, "[PORT]") {
		t.Fatalf("single-project output should not contain repo badge: %q", out)
	}
	if got := issueIDForClipboard(item); got != "portfolio-hhg1r.1" {
		t.Fatalf("clipboard ID = %q, want full canonical ID", got)
	}
}

// TestIssueDelegate_RenderAliasesAtlasNamespaceBadge guards bt-z1pzj: the
// beads_global namespace's bare ID-prefix "global" (RepoPrefix is always
// ID-derived, see ExtractRepoPrefix) must render as the "atlas" display
// alias in the workspace-mode repo badge, not the raw "beads_global" or
// "global" spelling.
func TestIssueDelegate_RenderAliasesAtlasNamespaceBadge(t *testing.T) {
	item := newTestIssueItem("global-42")
	item.RepoPrefix = "global"
	delegate := IssueDelegate{Theme: DefaultTheme(), WorkspaceMode: true}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(120)

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if strings.Contains(out, "[GLOB") {
		t.Fatalf("render output should not show raw beads_global badge: %q", out)
	}
	if !strings.Contains(out, "[ATLA]") {
		t.Fatalf("render output missing aliased atlas badge [ATLA]: %q", out)
	}
}

func TestIssueDelegate_CompactIDWidthFlowsToTitle(t *testing.T) {
	const (
		width           = 80
		fixedWithoutID  = 20
		rightWidth      = 10
		canonicalID     = "portfolio-hhg1r.1"
		compactID       = "hhg1r.1"
		idCellSeparator = 1
	)

	fullLeftWidth := fixedWithoutID + lipgloss.Width(canonicalID) + idCellSeparator
	compactLeftWidth := fixedWithoutID + lipgloss.Width(compactID) + idCellSeparator
	fullTitleWidth := issueListTitleWidth(width, fullLeftWidth, rightWidth)
	compactTitleWidth := issueListTitleWidth(width, compactLeftWidth, rightWidth)

	wantGain := lipgloss.Width(canonicalID) - lipgloss.Width(compactID)
	if got := compactTitleWidth - fullTitleWidth; got != wantGain {
		t.Fatalf("title width gain = %d, want %d", got, wantGain)
	}
	if got := issueListTitleWidth(0, 100, 100); got != 5 {
		t.Fatalf("minimum title width = %d, want 5", got)
	}
}

func TestIssueListColumnHeaderUsesCompactIDCell(t *testing.T) {
	for _, workspaceMode := range []bool{false, true} {
		header := issueListColumnHeader(workspaceMode)
		if !strings.Contains(header, "ID    TITLE") {
			t.Fatalf("workspaceMode=%t header does not use compact ID cell: %q", workspaceMode, header)
		}
		if workspaceMode != strings.Contains(header, "REPO") {
			t.Fatalf("workspaceMode=%t header repo column mismatch: %q", workspaceMode, header)
		}
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
	if !strings.Contains(out, activeGlyphs.Pencil) {
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

// Row rendering must never emit a [0.NN] score badge. The hybrid score is
// now consumed only by the detail-pane Search Scores section (bt-gfxhz.6).
// bt-r3zxj decided row badges are noise for humans; agents continue to
// receive scores via bt robot search JSON.
func TestIssueDelegate_RenderOmitsSearchScoreBadge(t *testing.T) {
	item := newTestIssueItem("NOBADGE-1")
	item.SearchScoreSet = true
	item.SearchScore = 0.48 // well above the former 0.05 threshold
	theme := DefaultTheme()
	delegate := IssueDelegate{Theme: theme}

	l := list.New([]list.Item{item}, delegate, 0, 0)
	l.SetWidth(120)

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)
	out := buf.String()

	if strings.Contains(out, "[0.48]") {
		t.Fatalf("render output should not contain row score badge: %q", out)
	}
	if strings.Contains(out, "[0.") {
		t.Fatalf("render output contains a [0.NN]-shaped score badge: %q", out)
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
