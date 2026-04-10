package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/recipe"
)

func TestFilterByRepo_CaseInsensitiveAndFlexibleSeparators(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", SourceRepo: "services/api"},
		{ID: "web:UI-2", SourceRepo: "apps/web"},
		{ID: "lib_UTIL_3", SourceRepo: "libs/util"},
		{ID: "misc-4", SourceRepo: "misc"},
	}

	tests := []struct {
		filter   string
		expected int
	}{
		{"API", 1},      // case-insensitive, matches api-
		{"web", 1},      // flexible with ':' separator
		{"lib", 1},      // flexible with '_' separator
		{"missing", 0},  // no match
		{"misc-", 1},    // exact prefix
		{"services", 1}, // matches SourceRepo when ID lacks prefix
	}

	for _, tt := range tests {
		got := filterByRepo(issues, tt.filter)
		if len(got) != tt.expected {
			t.Errorf("filterByRepo(%q) = %d issues, want %d", tt.filter, len(got), tt.expected)
		}
	}
}

func TestRobotFlagsOutputJSON(t *testing.T) {
	tmpDir := t.TempDir()
	beads := `{"id":"A","title":"Root","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Blocked","status":"blocked","priority":2,"issue_type":"task","dependencies":[{"depends_on_id":"A","type":"blocks"}]}`

	if err := os.WriteFile(filepath.Join(tmpDir, ".beads.jsonl"), []byte(beads), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".beads"), 0755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".beads", "beads.jsonl"), []byte(beads), 0644); err != nil {
		t.Fatalf("write beads dir: %v", err)
	}

	// Build a temporary bv binary using the repo module
	bin := filepath.Join(tmpDir, "bv")
	build := exec.Command("go", "build", "-C", repoRoot(t), "-o", bin, "./cmd/bv")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build bv: %v\n%s", err, out)
	}

	run := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = tmpDir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
		return out
	}

	for _, flag := range [][]string{
		{"--robot-plan"},
		{"--robot-insights"},
		{"--robot-priority"},
		{"--robot-recipes"},
	} {
		out := run(flag...)
		if !json.Valid(out) {
			t.Fatalf("%v did not return valid JSON: %s", flag, string(out))
		}
	}
}

func TestApplyRecipeFilters_ActionableAndHasBlockers(t *testing.T) {
	now := time.Now()
	a := model.Issue{ID: "A", Title: "Root", Status: model.StatusOpen, Priority: 2, CreatedAt: now}
	b := model.Issue{
		ID:     "B",
		Title:  "Blocked by A",
		Status: model.StatusOpen,
		Dependencies: []*model.Dependency{
			{DependsOnID: "A", Type: model.DepBlocks},
		},
		CreatedAt: now.Add(-time.Hour),
	}
	issues := []model.Issue{a, b}

	r := &recipe.Recipe{
		Filters: recipe.FilterConfig{
			Actionable: ptrBool(true),
		},
	}
	actionable := applyRecipeFilters(issues, r)
	if len(actionable) != 1 || actionable[0].ID != "A" {
		t.Fatalf("expected only A actionable, got %#v", actionable)
	}

	r.Filters.Actionable = nil
	r.Filters.HasBlockers = ptrBool(true)
	blocked := applyRecipeFilters(issues, r)
	if len(blocked) != 1 || blocked[0].ID != "B" {
		t.Fatalf("expected only B when HasBlockers=true, got %#v", blocked)
	}
}

func TestApplyRecipeFilters_TitleAndPrefix(t *testing.T) {
	issues := []model.Issue{
		{ID: "UI-1", Title: "Add login button"},
		{ID: "API-2", Title: "Login endpoint"},
		{ID: "API-3", Title: "Health check"},
	}
	r := &recipe.Recipe{
		Filters: recipe.FilterConfig{
			TitleContains: "login",
			IDPrefix:      "API",
		},
	}
	got := applyRecipeFilters(issues, r)
	if len(got) != 1 || got[0].ID != "API-2" {
		t.Fatalf("expected API-2 only, got %#v", got)
	}
}

func TestApplyRecipeFilters_TagsAndDates(t *testing.T) {
	now := time.Now()
	old := now.Add(-48 * time.Hour)
	issues := []model.Issue{
		{ID: "T1", Title: "Tagged", Labels: []string{"backend", "p0"}, CreatedAt: now, UpdatedAt: now},
		{ID: "T2", Title: "Old", Labels: []string{"backend"}, CreatedAt: old, UpdatedAt: old},
	}
	r := &recipe.Recipe{
		Filters: recipe.FilterConfig{
			Tags:         []string{"backend"},
			ExcludeTags:  []string{"p0"},
			CreatedAfter: "1d",
			UpdatedAfter: "1d",
		},
	}
	got := applyRecipeFilters(issues, r)
	if len(got) != 0 {
		t.Fatalf("expected all filtered out (exclude p0 and date), got %#v", got)
	}
}

func TestApplyRecipeFilters_DatesBlockersAndPrefix(t *testing.T) {
	now := time.Now()
	early := now.Add(-72 * time.Hour)
	issues := []model.Issue{
		{ID: "API-1", Title: "Fresh", CreatedAt: now, UpdatedAt: now},
		{ID: "API-2", Title: "Stale", CreatedAt: early, UpdatedAt: early,
			Dependencies: []*model.Dependency{{DependsOnID: "API-1", Type: model.DepBlocks}}},
	}
	r := &recipe.Recipe{Filters: recipe.FilterConfig{
		CreatedBefore: "1h",
		UpdatedBefore: "1h",
		HasBlockers:   ptrBool(true),
		IDPrefix:      "API-2",
	}}
	got := applyRecipeFilters(issues, r)
	if len(got) != 1 || got[0].ID != "API-2" {
		t.Fatalf("expected only API-2 to match blockers/date/prefix filters, got %#v", got)
	}

	r.Filters.HasBlockers = ptrBool(false)
	got = applyRecipeFilters(issues, r)
	if len(got) != 0 {
		t.Fatalf("expected blockers=false to exclude API-2, got %#v", got)
	}
}

func TestApplyRecipeSort_DefaultsAndFields(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "A", Title: "zzz", Priority: 2, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-30 * time.Minute)},
		{ID: "B", Title: "aaa", Priority: 0, CreatedAt: now, UpdatedAt: now},
	}

	// Priority default ascending
	r := &recipe.Recipe{Sort: recipe.SortConfig{Field: "priority"}}
	sorted := applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "B" {
		t.Fatalf("priority sort expected B first, got %s", sorted[0].ID)
	}

	// Created default descending (newest first)
	r.Sort = recipe.SortConfig{Field: "created"}
	sorted = applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "B" {
		t.Fatalf("created sort expected newest (B) first, got %s", sorted[0].ID)
	}

	// Title ascending explicit desc
	r.Sort = recipe.SortConfig{Field: "title", Direction: "desc"}
	sorted = applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "A" {
		t.Fatalf("title desc expected A (zzz) first, got %s", sorted[0].ID)
	}

	// Status ascending (string compare)
	r.Sort = recipe.SortConfig{Field: "status"}
	sorted = applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "A" { // both open; stable sort keeps original order
		t.Fatalf("status sort expected A first, got %s", sorted[0].ID)
	}

	// ID natural sort
	idIssues := []model.Issue{
		{ID: "bv-10"},
		{ID: "bv-2"},
		{ID: "bv-1"},
	}
	r.Sort = recipe.SortConfig{Field: "id"}
	sortedIDs := applyRecipeSort(append([]model.Issue{}, idIssues...), r)
	if sortedIDs[0].ID != "bv-1" || sortedIDs[1].ID != "bv-2" || sortedIDs[2].ID != "bv-10" {
		t.Fatalf("id natural sort failed: got %v", []string{sortedIDs[0].ID, sortedIDs[1].ID, sortedIDs[2].ID})
	}

	// Unknown field should preserve order
	r.Sort = recipe.SortConfig{Field: "unknown"}
	sorted = applyRecipeSort(append([]model.Issue{}, issues...), r)
	if sorted[0].ID != "A" || sorted[1].ID != "B" {
		t.Fatalf("unknown sort field should keep original order, got %v", []string{sorted[0].ID, sorted[1].ID})
	}
}

func TestFormatCycle(t *testing.T) {
	if got := formatCycle(nil); got != "(empty)" {
		t.Fatalf("expected (empty), got %q", got)
	}
	c := []string{"X", "Y", "Z"}
	want := "X → Y → Z → X"
	if got := formatCycle(c); got != want {
		t.Fatalf("formatCycle mismatch: got %q want %q", got, want)
	}
}

func ptrBool(b bool) *bool { return &b }

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod above %s", dir)
		}
		dir = parent
	}
}
