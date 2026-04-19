package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestNoRawListSetItems enforces that m.list.SetItems is only called from
// setListItems in model_filter.go. Direct calls bypass filter preservation
// and reintroduce bt-nzsy (search results disappear on background refresh).
//
// If this test fails, route the call through m.setListItems(items) instead.
func TestNoRawListSetItems(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read pkg dir: %v", err)
	}

	needle := []byte("m.list.SetItems(")
	const allowedFile = "model_filter.go"
	const allowedFunc = "func (m *Model) setListItems"

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(".", e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}

		if !containsBytes(data, needle) {
			continue
		}

		if e.Name() != allowedFile {
			t.Errorf("%s calls m.list.SetItems directly — use m.setListItems to preserve filter (bt-nzsy)", e.Name())
			continue
		}

		// In the allowed file, the only match must be inside setListItems.
		source := string(data)
		funcIdx := strings.Index(source, allowedFunc)
		if funcIdx < 0 {
			t.Fatalf("setListItems not found in %s", e.Name())
		}
		// Find the closing brace of the setListItems function. Match \n}\n or \n}\r\n
		// so this works regardless of line endings.
		normalized := strings.ReplaceAll(source, "\r\n", "\n")
		normFuncIdx := strings.Index(normalized, allowedFunc)
		funcEnd := strings.Index(normalized[normFuncIdx:], "\n}\n")
		if funcEnd < 0 {
			t.Fatalf("setListItems body end not found in %s", e.Name())
		}
		funcEnd += normFuncIdx
		source = normalized
		funcIdx = normFuncIdx

		// Any SetItems outside this range is a bug.
		for pos := 0; pos < len(source); {
			idx := strings.Index(source[pos:], string(needle))
			if idx < 0 {
				break
			}
			abs := pos + idx
			if abs < funcIdx || abs > funcEnd {
				t.Errorf("%s:%d has m.list.SetItems outside setListItems — route through the wrapper (bt-nzsy)",
					e.Name(), lineOf(source, abs))
			}
			pos = abs + len(needle)
		}
	}
}

func containsBytes(haystack, needle []byte) bool {
	return strings.Contains(string(haystack), string(needle))
}

func lineOf(s string, offset int) int {
	return strings.Count(s[:offset], "\n") + 1
}

// TestSetListItemsPreservesFilter_Filtering asserts the wrapper re-applies the
// filter match against new items when the user is mid-typing (Filtering state).
func TestSetListItemsPreservesFilter_Filtering(t *testing.T) {
	m := filterTestModel(t)
	m.list.SetFilterText("2h8")
	m.list.SetFilterState(list.Filtering)

	// Simulate a background refresh replacing items.
	newItems := []list.Item{
		IssueItem{Issue: model.Issue{ID: "cass-2h8", Title: "Strategic: cass positioning", Status: model.StatusInProgress}},
		IssueItem{Issue: model.Issue{ID: "cass-abc", Title: "Unrelated issue", Status: model.StatusOpen}},
	}
	m.setListItems(newItems)

	if got := m.list.FilterState(); got != list.Filtering && got != list.FilterApplied {
		t.Fatalf("filter state lost after setListItems: got %v", got)
	}
	if got := m.list.FilterValue(); got != "2h8" {
		t.Fatalf("filter value lost: got %q want %q", got, "2h8")
	}
	// VisibleItems returns the current filtered slice; should match cass-2h8 only.
	visible := m.list.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(visible))
	}
	if item, ok := visible[0].(IssueItem); !ok || item.Issue.ID != "cass-2h8" {
		t.Fatalf("wrong filtered item: %+v", visible[0])
	}
}

// TestSetListItemsPreservesFilter_FilterApplied asserts the wrapper also works
// after the user pressed Enter to commit the filter (FilterApplied state).
func TestSetListItemsPreservesFilter_FilterApplied(t *testing.T) {
	m := filterTestModel(t)
	m.list.SetFilterText("2h8")
	m.list.SetFilterState(list.FilterApplied)

	newItems := []list.Item{
		IssueItem{Issue: model.Issue{ID: "cass-2h8", Title: "Strategic: cass positioning", Status: model.StatusInProgress}},
		IssueItem{Issue: model.Issue{ID: "cass-abc", Title: "Unrelated issue", Status: model.StatusOpen}},
	}
	m.setListItems(newItems)

	if got := m.list.FilterState(); got != list.FilterApplied {
		t.Fatalf("FilterApplied not preserved: got %v", got)
	}
	visible := m.list.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(visible))
	}
}

// TestRefreshListItemsPhase2_DoesNotPanicWithNarrowFilter reproduces the
// crash from the bt-nzsy dogfood session: when a search narrowed the list to
// a single match whose unfiltered index was past PerPage, Phase2's
// refreshListItemsPhase2 restored selection by index, driving Paginator.Page
// out of bounds and panicking during the next View() pass.
func TestRefreshListItemsPhase2_DoesNotPanicWithNarrowFilter(t *testing.T) {
	// Build a Model with enough items to span multiple pages.
	issues := make([]model.Issue, 60)
	for i := range issues {
		issues[i] = model.Issue{
			ID:        "proj-" + randID(i),
			Title:     "Issue " + randID(i),
			Status:    model.StatusOpen,
			CreatedAt: time.Now(),
		}
	}
	// Make sure the filter has a narrow match late in the list.
	issues[55].ID = "proj-tjq0"
	issues[55].Title = "target tjq0"

	items := make([]list.Item, len(issues))
	for i := range issues {
		items[i] = IssueItem{Issue: issues[i]}
	}

	cached := analysis.NewCachedAnalyzer(issues, nil)
	lst := list.New(items, list.NewDefaultDelegate(), 80, 20)
	lst.SetFilteringEnabled(true)
	m := Model{
		filter:   &FilterState{currentFilter: "all"},
		data:     &DataState{issues: issues, analyzer: cached.Analyzer, analysis: cached.AnalyzeAsync(context.Background())},
		ac:       &AnalysisCache{},
		list:     lst,
		theme:    DefaultTheme(),
		renderer: NewMarkdownRendererWithTheme(80, DefaultTheme()),
	}

	// Put the cursor on the late-index match via unfiltered Select, then
	// activate the filter that matches only that one item.
	m.list.Select(55)
	m.list.SetFilterText("tjq0")
	m.list.SetFilterState(list.FilterApplied)

	// refreshListItemsPhase2 must not panic during the subsequent View() pass.
	m.refreshListItemsPhase2()

	// View() exercises populatedView -> Paginator.GetSliceBounds, where the
	// old bug crashed with slice bounds out of range.
	_ = m.list.View()
}

func randID(i int) string {
	return string(rune('a'+(i%26))) + string(rune('0'+(i%10)))
}

// TestPhase2ReadyPreservesFilter is the integration test for bt-nzsy: it
// asserts a real Phase2ReadyMsg processed via Update() does not clobber an
// active list filter.
func TestPhase2ReadyPreservesFilter(t *testing.T) {
	m := filterTestModel(t)
	// Set an active filter that matches one of the test issues.
	m.list.SetFilterText("2h8")
	m.list.SetFilterState(list.FilterApplied)

	// Sanity: filter is populated.
	if len(m.list.VisibleItems()) != 1 {
		t.Fatalf("precondition: filter not applied, visible=%d", len(m.list.VisibleItems()))
	}

	// Drive Phase2ReadyMsg through Update. This exercises the
	// handlePhase2Ready -> applyFilter path that clobbered filter before bt-nzsy.
	ins := analysis.Insights{}
	if m.data.analysis != nil {
		ins = m.data.analysis.GenerateInsights(len(m.data.issues))
	}
	newM, _ := m.Update(Phase2ReadyMsg{Stats: m.data.analysis, Insights: ins})
	m2, ok := newM.(Model)
	if !ok {
		t.Fatalf("Update returned wrong model type: %T", newM)
	}

	if got := m2.list.FilterState(); got != list.Filtering && got != list.FilterApplied {
		t.Fatalf("Phase2Ready wiped filter state: got %v", got)
	}
	if got := m2.list.FilterValue(); got != "2h8" {
		t.Fatalf("Phase2Ready wiped filter value: got %q", got)
	}
	visible := m2.list.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("Phase2Ready wiped filter matches: visible=%d", len(visible))
	}
}

// filterTestModel returns a minimal Model wired with two test issues and the
// filter subsystem ready to use. Shared across filter preservation tests.
func filterTestModel(t *testing.T) Model {
	t.Helper()
	issues := []model.Issue{
		{ID: "cass-2h8", Title: "Strategic: cass positioning", Status: model.StatusInProgress, CreatedAt: time.Now()},
		{ID: "cass-abc", Title: "Unrelated issue", Status: model.StatusOpen, CreatedAt: time.Now()},
	}
	cached := analysis.NewCachedAnalyzer(issues, nil)
	items := make([]list.Item, len(issues))
	for i := range issues {
		items[i] = IssueItem{Issue: issues[i]}
	}
	lst := list.New(items, list.NewDefaultDelegate(), 80, 20)
	lst.SetFilteringEnabled(true)
	m := Model{
		filter: &FilterState{currentFilter: "all"},
		data: &DataState{
			issues: issues,
			issueMap: map[string]*model.Issue{
				"cass-2h8": &issues[0],
				"cass-abc": &issues[1],
			},
			analyzer: cached.Analyzer,
			analysis: cached.AnalyzeAsync(context.Background()),
		},
		ac:       &AnalysisCache{},
		list:     lst,
		theme:    DefaultTheme(),
		renderer: NewMarkdownRendererWithTheme(80, DefaultTheme()),
	}
	return m
}
