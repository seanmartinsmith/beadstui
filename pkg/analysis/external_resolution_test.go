package analysis

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// externalFixture is the on-disk shape for testdata/external/*.json.
type externalFixture struct {
	Description string        `json:"description"`
	Issues      []model.Issue `json:"issues"`
}

func loadExternalFixture(t *testing.T, name string) []model.Issue {
	t.Helper()
	path := filepath.Join("testdata", "external", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var fx externalFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return fx.Issues
}

// getIssue locates an issue by ID for assertion purposes. Named with a
// package-unique suffix to avoid colliding with findIssue in
// label_suggest_test.go.
func getIssue(t *testing.T, issues []model.Issue, id string) model.Issue {
	t.Helper()
	for _, issue := range issues {
		if issue.ID == id {
			return issue
		}
	}
	t.Fatalf("issue %q not present in slice", id)
	return model.Issue{}
}

func TestResolveExternalDeps_NilAndEmpty(t *testing.T) {
	if got := ResolveExternalDeps(nil); got != nil {
		t.Errorf("nil input: got %#v, want nil", got)
	}
	empty := []model.Issue{}
	got := ResolveExternalDeps(empty)
	if len(got) != 0 {
		t.Errorf("empty input: got len %d, want 0", len(got))
	}
}

func TestResolveExternalDeps_RewritesResolvedRefs(t *testing.T) {
	issues := loadExternalFixture(t, "two_project_chain.json")
	got := ResolveExternalDeps(issues)

	bta := getIssue(t, got, "bt-a")
	if len(bta.Dependencies) != 1 {
		t.Fatalf("bt-a deps: got %d, want 1", len(bta.Dependencies))
	}
	if bta.Dependencies[0].DependsOnID != "cass-x" {
		t.Errorf("bt-a dep rewrite: got %q, want %q", bta.Dependencies[0].DependsOnID, "cass-x")
	}
	if bta.Dependencies[0].Type != model.DepBlocks {
		t.Errorf("bt-a dep type not preserved: got %q", bta.Dependencies[0].Type)
	}

	// cass-x → cass-y is not external; should remain untouched.
	cassx := getIssue(t, got, "cass-x")
	if len(cassx.Dependencies) != 1 || cassx.Dependencies[0].DependsOnID != "cass-y" {
		t.Errorf("cass-x deps altered: %+v", cassx.Dependencies)
	}

	// The resolved set must form a graph Analyzer can traverse end to end.
	analyzer := NewAnalyzer(got)
	stats := analyzer.Analyze()
	if stats.NodeCount != len(got) {
		t.Errorf("stats node count: got %d, want %d", stats.NodeCount, len(got))
	}
	// With resolution, the chain from bt-a must reach cass-x / cass-y.
	// Without resolution, bt-a would be an island.
	chain := analyzer.GetBlockerChain("bt-a")
	if chain == nil {
		t.Fatalf("GetBlockerChain(bt-a) returned nil")
	}
	if !chain.IsBlocked {
		t.Fatalf("GetBlockerChain(bt-a).IsBlocked = false; expected bt-a to be blocked via cross-project chain")
	}
	found := false
	for _, hop := range chain.Chain {
		if hop.ID == "cass-y" || hop.ID == "cass-x" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("BlockerChain(bt-a) did not cross into cass project: %+v", chain.Chain)
	}
}

func TestResolveExternalDeps_DropsUnresolvedAndMalformed(t *testing.T) {
	issues := loadExternalFixture(t, "unresolved_external.json")
	got := ResolveExternalDeps(issues)

	bta := getIssue(t, got, "bt-a")
	// Only the bt-b dep should survive: unresolved external:cass:ghost
	// and all three malformed forms get dropped.
	if len(bta.Dependencies) != 1 {
		t.Fatalf("bt-a deps after drop: got %d, want 1 (bt-b only)", len(bta.Dependencies))
	}
	if bta.Dependencies[0].DependsOnID != "bt-b" {
		t.Errorf("bt-a surviving dep: got %q, want bt-b", bta.Dependencies[0].DependsOnID)
	}
}

func TestResolveExternalDeps_DoesNotMutateInput(t *testing.T) {
	issues := loadExternalFixture(t, "two_project_chain.json")

	// Capture a deep snapshot via JSON round-trip — the only cheap way to
	// check every nested field including *Dependency.DependsOnID after the
	// resolver runs.
	before, err := json.Marshal(issues)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}

	_ = ResolveExternalDeps(issues)

	after, err := json.Marshal(issues)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("input mutated by resolver\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestResolveExternalDeps_Idempotent(t *testing.T) {
	issues := loadExternalFixture(t, "two_project_chain.json")
	once := ResolveExternalDeps(issues)
	twice := ResolveExternalDeps(once)

	onceJSON, _ := json.Marshal(once)
	twiceJSON, _ := json.Marshal(twice)
	if !bytes.Equal(onceJSON, twiceJSON) {
		t.Errorf("resolver not idempotent\n--- once ---\n%s\n--- twice ---\n%s", onceJSON, twiceJSON)
	}
}

func TestResolveExternalDeps_StructuralEquivalenceWithoutExternals(t *testing.T) {
	// Input with zero external refs: resolver must be a structural no-op.
	// Guards the byte-identical single-project promise from future resolver
	// drift that mutates deps unnecessarily.
	now := "2026-04-10T10:00:00Z"
	parse := func(s string) (t model.Issue) { _ = json.Unmarshal([]byte(s), &t); return }
	issues := []model.Issue{
		parse(`{"id":"bt-a","title":"A","status":"open","priority":1,"issue_type":"task","created_at":"` + now + `","updated_at":"` + now + `","dependencies":[{"issue_id":"bt-a","depends_on_id":"bt-b","type":"blocks","created_at":"` + now + `","created_by":"sms"}]}`),
		parse(`{"id":"bt-b","title":"B","status":"open","priority":1,"issue_type":"task","created_at":"` + now + `","updated_at":"` + now + `"}`),
	}

	got := ResolveExternalDeps(issues)

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(issues)
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Errorf("structural equivalence broken\n--- got ---\n%s\n--- want ---\n%s", gotJSON, wantJSON)
	}
}

func TestResolveExternalDeps_LogsUnresolvedAtDebug(t *testing.T) {
	issues := loadExternalFixture(t, "unresolved_external.json")

	var buf bytes.Buffer
	// Install a debug-level JSON handler and restore afterwards.
	prev := slog.Default()
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(prev)

	_ = ResolveExternalDeps(issues)

	// Expect exactly one debug line for bt-a aggregating all dropped refs.
	// Scan lines and count matches — handler writes one JSON object per call.
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	debugLines := 0
	var matched map[string]any
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("parse log line: %v\nline=%s", err, line)
		}
		if entry["level"] != "DEBUG" {
			t.Errorf("unexpected log level %v on line %s", entry["level"], line)
			continue
		}
		debugLines++
		matched = entry
	}
	if debugLines != 1 {
		t.Fatalf("expected 1 debug line, got %d: %s", debugLines, buf.String())
	}
	if matched["issue"] != "bt-a" {
		t.Errorf("log issue field: got %v, want bt-a", matched["issue"])
	}
	if n, _ := matched["count"].(float64); int(n) != 4 {
		t.Errorf("log count: got %v, want 4", matched["count"])
	}
}

func TestParseExternalRef(t *testing.T) {
	cases := []struct {
		in           string
		wantProject  string
		wantSuffix   string
		wantOK       bool
	}{
		{"external:cass:x", "cass", "x", true},
		{"external:bt:mhwy.5", "bt", "mhwy.5", true},
		{"external:", "", "", false},
		{"external::", "", "", false},
		{"external:bt:", "", "", false},
		{"external::x", "", "", false},
		{"external:a:b:c", "", "", false},
		{"cass-x", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			project, suffix, ok := parseExternalRef(tc.in)
			if ok != tc.wantOK || project != tc.wantProject || suffix != tc.wantSuffix {
				t.Errorf("parseExternalRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, project, suffix, ok, tc.wantProject, tc.wantSuffix, tc.wantOK)
			}
		})
	}
}

func TestSplitID(t *testing.T) {
	cases := []struct {
		in           string
		wantPrefix   string
		wantSuffix   string
		wantOK       bool
	}{
		{"bt-mhwy.5", "bt", "mhwy.5", true},
		{"cass-x", "cass", "x", true},
		{"", "", "", false},
		{"noHyphen", "", "", false},
		{"-leadingHyphen", "", "", false},
		{"trailingHyphen-", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			prefix, suffix, ok := SplitID(tc.in)
			if ok != tc.wantOK || prefix != tc.wantPrefix || suffix != tc.wantSuffix {
				t.Errorf("SplitID(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, prefix, suffix, ok, tc.wantPrefix, tc.wantSuffix, tc.wantOK)
			}
		})
	}
}

// TestResolveExternalDeps_OtherIssuesUntouched guards that resolving an issue
// with external deps does not touch the dependency slices of sibling issues
// that had no external refs — a common trap when copy-on-write paths are
// wired incorrectly.
func TestResolveExternalDeps_OtherIssuesUntouched(t *testing.T) {
	issues := loadExternalFixture(t, "two_project_chain.json")
	// Snapshot cass-x's dep pointer array header before resolution.
	cassxBefore := getIssue(t, issues, "cass-x").Dependencies
	got := ResolveExternalDeps(issues)
	cassxAfter := getIssue(t, got, "cass-x").Dependencies

	// cass-x had no external deps; the output should alias the caller's
	// slice rather than allocate a new one.
	if !reflect.DeepEqual(cassxBefore, cassxAfter) {
		t.Errorf("cass-x deps altered during resolution\nbefore=%+v\nafter=%+v", cassxBefore, cassxAfter)
	}
}
