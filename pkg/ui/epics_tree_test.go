package ui

import (
	"strconv"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// epicsTreeFixture builds a 2-project corpus exercising every Build concern:
//   - project "bt": epic bt-A (mixed children) containing a NESTED child-epic
//     bt-B (parent-child child of bt-A), plus a more-complete epic bt-C.
//   - project "sym": epic sym-X.
//
// bt-B must appear only nested under bt-A, never as a top-level lane row.
func epicsTreeFixture() []model.Issue {
	now := time.Now()
	pc := func(child, parent string) []*model.Dependency {
		return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
	}
	epic := func(id, title string, deps []*model.Dependency) model.Issue {
		return model.Issue{ID: id, Title: title, IssueType: model.TypeEpic, Status: model.StatusOpen, UpdatedAt: now, Dependencies: deps}
	}
	kid := func(id string, st model.Status, parent string) model.Issue {
		return model.Issue{ID: id, Title: id, Status: st, UpdatedAt: now, Dependencies: pc(id, parent)}
	}
	return []model.Issue{
		epic("bt-A", "Alpha epic", nil),
		kid("bt-A.1", model.StatusClosed, "bt-A"),
		kid("bt-A.2", model.StatusInProgress, "bt-A"),
		kid("bt-A.3", model.StatusOpen, "bt-A"),
		// bt-B is an epic AND a parent-child child of bt-A (the nesting case).
		epic("bt-B", "Beta nested epic", pc("bt-B", "bt-A")),
		kid("bt-B.1", model.StatusClosed, "bt-B"),
		kid("bt-B.2", model.StatusOpen, "bt-B"),
		// bt-C: more complete than bt-A (both children closed).
		epic("bt-C", "Gamma epic", nil),
		kid("bt-C.1", model.StatusClosed, "bt-C"),
		kid("bt-C.2", model.StatusClosed, "bt-C"),
		// Second project lane.
		epic("sym-X", "Xi epic", nil),
		kid("sym-X.1", model.StatusOpen, "sym-X"),
		kid("sym-X.2", model.StatusInProgress, "sym-X"),
	}
}

// rowByIssue returns the first flattened row whose issue has the given ID, or
// (-1, zero) if absent.
func rowByIssue(rows []epicTreeRow, id string) (int, epicTreeRow) {
	for i, r := range rows {
		if r.issue != nil && r.issue.ID == id {
			return i, r
		}
	}
	return -1, epicTreeRow{}
}

func TestEpicsTree_DefaultFlatten(t *testing.T) {
	var e EpicsTreeModel
	e.Build(epicsTreeFixture(), EpicsAll, time.Now())
	rows := e.rows()

	// Default: project headers expanded, epics collapsed -> headers + root epics
	// only, no child rows.
	want := []struct {
		kind    epicTreeRowKind
		project string
		issueID string
	}{
		{rowProjectHeader, "bt", ""},
		{rowEpic, "bt", "bt-A"},  // within-lane progress asc: bt-A (25%) before bt-C (100%)
		{rowEpic, "bt", "bt-C"},
		{rowProjectHeader, "sym", ""},
		{rowEpic, "sym", "sym-X"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d:\n%s", len(rows), len(want), dumpRows(rows))
	}
	for i, w := range want {
		if rows[i].kind != w.kind || rows[i].project != w.project {
			t.Errorf("row %d: kind=%d project=%q, want kind=%d project=%q", i, rows[i].kind, rows[i].project, w.kind, w.project)
		}
		if w.issueID != "" && (rows[i].issue == nil || rows[i].issue.ID != w.issueID) {
			t.Errorf("row %d: issue=%v, want %q", i, rows[i].issue, w.issueID)
		}
	}

	// Root-epic dedup: bt-B is a child-epic of bt-A; it must NOT be a top row.
	if i, _ := rowByIssue(rows, "bt-B"); i != -1 {
		t.Errorf("bt-B should be nested, not a top-level row (found at %d)", i)
	}

	// Lane order: bt (2 epics) before sym (1 epic).
	if rows[0].project != "bt" || rows[3].project != "sym" {
		t.Errorf("lane order wrong: %s then %s, want bt then sym", rows[0].project, rows[3].project)
	}
}

func TestEpicsTree_ExpandEpicRevealsChildrenAndNestedEpic(t *testing.T) {
	var e EpicsTreeModel
	e.Build(epicsTreeFixture(), EpicsAll, time.Now())
	e.expand("bt-A")
	rows := e.rows()

	// bt-A's children appear (natural order: bt-A.1, .2, .3, then nested epic bt-B).
	for _, id := range []string{"bt-A.1", "bt-A.2", "bt-A.3", "bt-B"} {
		if i, _ := rowByIssue(rows, id); i == -1 {
			t.Errorf("expanded bt-A should reveal %s\n%s", id, dumpRows(rows))
		}
	}

	// Plain children are rowChild; the nested epic bt-B is a rowEpic with kids.
	if _, r := rowByIssue(rows, "bt-A.1"); r.kind != rowChild {
		t.Errorf("bt-A.1 kind=%d, want rowChild(%d)", r.kind, rowChild)
	}
	_, bRow := rowByIssue(rows, "bt-B")
	if bRow.kind != rowEpic {
		t.Errorf("nested bt-B kind=%d, want rowEpic(%d)", bRow.kind, rowEpic)
	}
	if !bRow.hasKids {
		t.Errorf("nested bt-B should report hasKids (it has bt-B.1/.2)")
	}
	// bt-B collapsed by default -> its children are NOT shown yet.
	if i, _ := rowByIssue(rows, "bt-B.1"); i != -1 {
		t.Errorf("bt-B.1 should be hidden while bt-B is collapsed (found at %d)", i)
	}

	// Connector flags: bt-B is the LAST child of bt-A; bt-A.1 is NOT last.
	if n := len(bRow.lastKid); n == 0 || !bRow.lastKid[n-1] {
		t.Errorf("bt-B lastKid=%v, want final flag true (it is bt-A's last child)", bRow.lastKid)
	}
	_, a1 := rowByIssue(rows, "bt-A.1")
	if n := len(a1.lastKid); n == 0 || a1.lastKid[n-1] {
		t.Errorf("bt-A.1 lastKid=%v, want final flag false", a1.lastKid)
	}

	// Drill deeper: expanding bt-B reveals its own children.
	e.expand("bt-B")
	rows = e.rows()
	if i, _ := rowByIssue(rows, "bt-B.1"); i == -1 {
		t.Errorf("expanding bt-B should reveal bt-B.1\n%s", dumpRows(rows))
	}
}

func TestEpicsTree_WithinLaneSortAndHeaderRollup(t *testing.T) {
	var e EpicsTreeModel
	e.Build(epicsTreeFixture(), EpicsAll, time.Now())
	rows := e.rows()

	// bt lane header rollup = sum of bt-A + bt-C counts.
	// bt-A: 4 children (1 done); bt-C: 2 children (2 done) -> Done 3 / Total 6.
	hdr := rows[0]
	if hdr.kind != rowProjectHeader {
		t.Fatalf("row 0 not a header")
	}
	if hdr.counts.Done != 3 || hdr.counts.Total != 6 {
		t.Errorf("bt lane rollup = %d/%d, want 3/6", hdr.counts.Done, hdr.counts.Total)
	}
}

func TestEpicsTree_CollapseAll(t *testing.T) {
	var e EpicsTreeModel
	e.Build(epicsTreeFixture(), EpicsAll, time.Now())
	e.expand("bt-A")
	e.expand("bt-B")
	if got := len(e.rows()); got <= 5 {
		t.Fatalf("expanded tree should have >5 rows, got %d", got)
	}
	e.collapseAll()
	rows := e.rows()
	// Back to the lane overview: headers + root epics, no child rows.
	if len(rows) != 5 {
		t.Fatalf("after collapseAll: %d rows, want 5 (lane overview)\n%s", len(rows), dumpRows(rows))
	}
	for _, id := range []string{"bt-A.1", "bt-B", "bt-B.1"} {
		if i, _ := rowByIssue(rows, id); i != -1 {
			t.Errorf("after collapseAll, %s should be hidden (found at %d)", id, i)
		}
	}
}

func TestEpicsTree_CycleGuardTerminates(t *testing.T) {
	now := time.Now()
	pc := func(child, parent string) []*model.Dependency {
		return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
	}
	// A (root) -> B -> C -> B : a cycle in the subtree below the root A.
	issues := []model.Issue{
		{ID: "cyc-A", Title: "A", IssueType: model.TypeEpic, Status: model.StatusOpen, UpdatedAt: now},
		{ID: "cyc-B", Title: "B", IssueType: model.TypeEpic, Status: model.StatusOpen, UpdatedAt: now, Dependencies: pc("cyc-B", "cyc-A")},
		{ID: "cyc-C", Title: "C", IssueType: model.TypeEpic, Status: model.StatusOpen, UpdatedAt: now, Dependencies: pc("cyc-C", "cyc-B")},
	}
	// Close the loop: cyc-B is also a parent-child child of cyc-C.
	issues[1].Dependencies = append(issues[1].Dependencies, &model.Dependency{IssueID: "cyc-B", DependsOnID: "cyc-C", Type: model.DepParentChild})

	var e EpicsTreeModel
	e.Build(issues, EpicsAll, now) // must terminate, not infinite-loop
	e.expand("cyc-A")
	e.expand("cyc-B")
	e.expand("cyc-C")
	rows := e.rows()
	if len(rows) == 0 {
		t.Fatal("cycle fixture produced no rows")
	}
	if len(rows) > 50 {
		t.Fatalf("cycle guard failed: row count exploded to %d", len(rows))
	}
}

// dumpRows renders rows compactly for failure messages.
func dumpRows(rows []epicTreeRow) string {
	var b []byte
	for i, r := range rows {
		id := "-"
		if r.issue != nil {
			id = r.issue.ID
		}
		b = append(b, []byte(
			"  ["+strconv.Itoa(i)+"] kind="+strconv.Itoa(int(r.kind))+" depth="+strconv.Itoa(r.depth)+" project="+r.project+" id="+id+"\n")...)
	}
	return string(b)
}
