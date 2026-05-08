package datasource

import (
	"fmt"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestImplicitParentID(t *testing.T) {
	cases := []struct {
		id     string
		parent string
		ok     bool
	}{
		{"bt-ift6.0", "bt-ift6", true},
		{"bt-ift6.13", "bt-ift6", true},
		{"bt-foo.1.1", "bt-foo.1", true}, // deep nesting: walks LastIndex
		{"bt-zsy8", "", false},           // cross-prefix paired ID, no dot
		{"bd-zsy8", "", false},
		{"bt-foo.bar", "", false},     // non-numeric suffix
		{"bt-foo.", "", false},        // trailing dot, no suffix
		{".bt-foo", "", false},        // leading dot, no parent
		{"", "", false},               // empty
		{"bt-ift6.13a", "", false},    // mostly-numeric tail still rejected
		{"bt-ift6.-1", "bt-ift6", true},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			parent, ok := implicitParentID(tc.id)
			if ok != tc.ok || parent != tc.parent {
				t.Fatalf("implicitParentID(%q) = (%q, %v), want (%q, %v)",
					tc.id, parent, ok, tc.parent, tc.ok)
			}
		})
	}
}

// TestSynthesizeHierarchyDeps_Bt_ift6Fixture mirrors the bt-ift6 epic + 14
// children (.0 through .13) shape that triggered bt-cuyiz: no explicit
// parent_child deps in source data, but bd treats them as children. After
// synthesis every child has a parent_child dep to bt-ift6.
func TestSynthesizeHierarchyDeps_Bt_ift6Fixture(t *testing.T) {
	issues := []model.Issue{{ID: "bt-ift6"}}
	for n := 0; n <= 13; n++ {
		issues = append(issues, model.Issue{ID: fmt.Sprintf("bt-ift6.%d", n)})
	}

	out := synthesizeHierarchyDeps(issues)
	if len(out) != len(issues) {
		t.Fatalf("synthesis dropped issues: in=%d out=%d", len(issues), len(out))
	}

	// Parent gets no synthesized dep.
	if got := len(out[0].Dependencies); got != 0 {
		t.Fatalf("parent bt-ift6 got %d deps, want 0", got)
	}

	// Each child gets exactly one parent_child dep to bt-ift6.
	for i := 1; i < len(out); i++ {
		child := out[i]
		if len(child.Dependencies) != 1 {
			t.Fatalf("child %s has %d deps, want 1: %#v",
				child.ID, len(child.Dependencies), child.Dependencies)
		}
		dep := child.Dependencies[0]
		if dep.IssueID != child.ID || dep.DependsOnID != "bt-ift6" || dep.Type != model.DepParentChild {
			t.Fatalf("child %s synthesized dep wrong: %#v", child.ID, dep)
		}
	}
}

// TestSynthesizeHierarchyDeps_DeepNesting covers two-level nesting:
// bt-foo + bt-foo.1 + bt-foo.1.1. Synthesis must produce both edges.
func TestSynthesizeHierarchyDeps_DeepNesting(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-foo"},
		{ID: "bt-foo.1"},
		{ID: "bt-foo.1.1"},
	}
	out := synthesizeHierarchyDeps(issues)

	// bt-foo.1 -> bt-foo
	if !depEq(out[1].Dependencies, "bt-foo", model.DepParentChild) {
		t.Fatalf("bt-foo.1 missing parent_child dep to bt-foo: %#v", out[1].Dependencies)
	}
	// bt-foo.1.1 -> bt-foo.1
	if !depEq(out[2].Dependencies, "bt-foo.1", model.DepParentChild) {
		t.Fatalf("bt-foo.1.1 missing parent_child dep to bt-foo.1: %#v", out[2].Dependencies)
	}
}

// TestSynthesizeHierarchyDeps_CrossPrefixPaired guards the load-bearing
// edge case from feedback_cross_project_bead_pairing memory: bt-zsy8 +
// bd-zsy8 share a suffix but neither has a dot, so synthesis must not
// trigger. Both render as roots.
func TestSynthesizeHierarchyDeps_CrossPrefixPaired(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-zsy8"},
		{ID: "bd-zsy8"},
	}
	out := synthesizeHierarchyDeps(issues)
	for _, iss := range out {
		if len(iss.Dependencies) != 0 {
			t.Fatalf("paired ID %s got synthesized deps %#v, want none", iss.ID, iss.Dependencies)
		}
	}
}

// TestSynthesizeHierarchyDeps_OrphanSuffix: child has `.N` suffix but
// parent is not in the loaded set. Defensive skip; child renders as root.
func TestSynthesizeHierarchyDeps_OrphanSuffix(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-orphan.5"},
		{ID: "bt-other"},
	}
	out := synthesizeHierarchyDeps(issues)
	for _, iss := range out {
		if len(iss.Dependencies) != 0 {
			t.Fatalf("orphan-suffix issue %s got synthesized deps %#v, want none",
				iss.ID, iss.Dependencies)
		}
	}
}

// TestSynthesizeHierarchyDeps_PreExistingExplicitDep: bt-w8j8.1 already
// has an explicit parent_child dep (filed via `--parent`). Synthesis must
// not duplicate the edge.
func TestSynthesizeHierarchyDeps_PreExistingExplicitDep(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-w8j8"},
		{
			ID: "bt-w8j8.1",
			Dependencies: []*model.Dependency{
				{IssueID: "bt-w8j8.1", DependsOnID: "bt-w8j8", Type: model.DepParentChild, CreatedBy: "human"},
			},
		},
	}
	out := synthesizeHierarchyDeps(issues)
	if got := len(out[1].Dependencies); got != 1 {
		t.Fatalf("pre-existing explicit dep duplicated: got %d deps, want 1: %#v",
			got, out[1].Dependencies)
	}
	if out[1].Dependencies[0].CreatedBy != "human" {
		t.Fatalf("explicit dep replaced by synthesized one: %#v", out[1].Dependencies[0])
	}
}

// TestSynthesizeHierarchyDeps_NonNumericSuffix: bt-foo.bar must not be
// treated as a child of bt-foo.
func TestSynthesizeHierarchyDeps_NonNumericSuffix(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-foo"},
		{ID: "bt-foo.bar"},
	}
	out := synthesizeHierarchyDeps(issues)
	if len(out[1].Dependencies) != 0 {
		t.Fatalf("non-numeric suffix bt-foo.bar got synthesized deps: %#v", out[1].Dependencies)
	}
}

// TestSynthesizeHierarchyDeps_PreservesOtherDeps: synthesis appends to the
// existing Dependencies slice without dropping unrelated edges (e.g.,
// sequential blocks chains like bt-ift6.3 blocked by bt-ift6.2).
func TestSynthesizeHierarchyDeps_PreservesOtherDeps(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-ift6"},
		{ID: "bt-ift6.2"},
		{
			ID: "bt-ift6.3",
			Dependencies: []*model.Dependency{
				{IssueID: "bt-ift6.3", DependsOnID: "bt-ift6.2", Type: model.DepBlocks},
			},
		},
	}
	out := synthesizeHierarchyDeps(issues)
	deps := out[2].Dependencies
	if len(deps) != 2 {
		t.Fatalf("bt-ift6.3 should have 2 deps after synthesis (1 blocks + 1 parent_child): %#v", deps)
	}
	hasBlocks := false
	hasParent := false
	for _, dep := range deps {
		if dep.Type == model.DepBlocks && dep.DependsOnID == "bt-ift6.2" {
			hasBlocks = true
		}
		if dep.Type == model.DepParentChild && dep.DependsOnID == "bt-ift6" {
			hasParent = true
		}
	}
	if !hasBlocks || !hasParent {
		t.Fatalf("bt-ift6.3 missing one of the expected deps: blocks=%v parent_child=%v deps=%#v",
			hasBlocks, hasParent, deps)
	}
}

// TestSynthesizeHierarchyDeps_EmptyInput: degenerate input does not panic.
func TestSynthesizeHierarchyDeps_EmptyInput(t *testing.T) {
	if got := synthesizeHierarchyDeps(nil); got != nil {
		t.Fatalf("synthesizeHierarchyDeps(nil) = %v, want nil", got)
	}
	if got := synthesizeHierarchyDeps([]model.Issue{}); len(got) != 0 {
		t.Fatalf("synthesizeHierarchyDeps([]) = %v, want empty", got)
	}
}

// depEq reports whether deps contains a dep with the given target and type.
func depEq(deps []*model.Dependency, target string, typ model.DependencyType) bool {
	for _, dep := range deps {
		if dep != nil && dep.DependsOnID == target && dep.Type == typ {
			return true
		}
	}
	return false
}
