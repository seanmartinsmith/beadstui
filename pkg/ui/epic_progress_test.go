package ui

import (
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// pcDep builds a parent_child dependency edge (child depends-on parent).
func pcDep(child, parent string) []*model.Dependency {
	return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
}

func epicProgressFixture() []model.Issue {
	return []model.Issue{
		{ID: "ep", IssueType: model.TypeEpic, Status: model.StatusOpen, Title: "The Epic"},
		{ID: "ep.1", Title: "first child", Status: model.StatusClosed, Priority: 1, Dependencies: pcDep("ep.1", "ep")},
		{ID: "ep.2", Title: "second child", Status: model.StatusInProgress, Priority: 0, Dependencies: pcDep("ep.2", "ep")},
		{ID: "ep.10", Title: "tenth child", Status: model.StatusOpen, Priority: 2, Dependencies: pcDep("ep.10", "ep")},
	}
}

func TestBuildEpicProgressANSI(t *testing.T) {
	all := epicProgressFixture()
	epic := all[0]

	out := buildEpicProgressANSI(epic, all, -1, 0)

	// It must be lipgloss (ANSI SGR), not markdown — the whole point of
	// bt-gfxhz.3 / the renderSection ANSI track (bt-x5xc4).
	if !strings.Contains(out, "\x1b") {
		t.Errorf("output has no ESC byte; expected lipgloss-styled ANSI, got %q", out)
	}
	// Parity: the old per-status markdown styling is gone.
	if strings.Contains(out, "~~") {
		t.Errorf("output contains markdown strikethrough literal ~~: %q", out)
	}
	if strings.Contains(out, "**") {
		t.Errorf("output contains markdown bold literal **: %q", out)
	}

	// Summary: 1 of 3 closed = 33%.
	if !strings.Contains(out, "1 / 3 children complete (33%)") {
		t.Errorf("summary line missing/incorrect: %q", out)
	}

	// Natural-numeric order: .1 before .2 before .10 (not Dolt load order).
	i1 := strings.Index(out, "ep.1 ")
	i2 := strings.Index(out, "ep.2")
	i10 := strings.Index(out, "ep.10")
	if !(i1 >= 0 && i2 > i1 && i10 > i2) {
		t.Errorf("children not in natural order (.1<.2<.10): idx ep.1=%d ep.2=%d ep.10=%d", i1, i2, i10)
	}

	// Childless epic -> empty so callers skip the section + heading.
	childless := model.Issue{ID: "lonely", IssueType: model.TypeEpic}
	if got := buildEpicProgressANSI(childless, []model.Issue{childless}, -1, 0); got != "" {
		t.Errorf("childless epic should render empty, got %q", got)
	}
}

func TestBuildEpicProgressANSI_Cursor(t *testing.T) {
	all := epicProgressFixture()
	epic := all[0]

	// selectedIdx = 1 -> the ▸ cursor is on the second child (ep.2) only.
	out := buildEpicProgressANSI(epic, all, 1, 0)
	lines := strings.Split(out, "\n")

	var ep2Line, ep1Line, ep10Line string
	for _, ln := range lines {
		switch {
		case strings.Contains(ln, "ep.10"):
			ep10Line = ln
		case strings.Contains(ln, "ep.2"):
			ep2Line = ln
		case strings.Contains(ln, "ep.1 "):
			ep1Line = ln
		}
	}

	if !strings.Contains(ep2Line, "▸") {
		t.Errorf("selected child (ep.2) row missing ▸ cursor: %q", ep2Line)
	}
	if strings.Contains(ep1Line, "▸") {
		t.Errorf("unselected child (ep.1) row should not have ▸ cursor: %q", ep1Line)
	}
	if strings.Contains(ep10Line, "▸") {
		t.Errorf("unselected child (ep.10) row should not have ▸ cursor: %q", ep10Line)
	}
}

func TestBuildEpicProgressANSI_TitleTruncation(t *testing.T) {
	all := []model.Issue{
		{ID: "ep", IssueType: model.TypeEpic, Status: model.StatusOpen},
		{ID: "ep.1", Title: "this is a very long child title that should be truncated to fit", Status: model.StatusOpen, Priority: 2, Dependencies: pcDep("ep.1", "ep")},
	}
	// Narrow width forces truncation; the ellipsis proves the budget applied.
	out := buildEpicProgressANSI(all[0], all, -1, 40)
	if !strings.Contains(out, "…") {
		t.Errorf("expected ellipsis from title truncation at width 40, got %q", out)
	}
}
