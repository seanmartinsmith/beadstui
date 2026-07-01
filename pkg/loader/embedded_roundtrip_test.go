package loader_test

import (
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// embeddedExportFixture mirrors the exact JSONL shape emitted by `bd export`
// on a schema-v50 embedded-mode project (verified against bd 1.0.5 output,
// 2026-07-01). It is the contract the embedded read path (bt-ij71a) depends
// on: bt shells `bd export` and feeds the stdout through this loader.
//
// The key names here are load-bearing and guard against bd export-format
// drift:
//   - Dependencies use a FLAT `depends_on_id` string, NOT the schema-v50
//     polymorphic `depends_on_issue_id` / `depends_on_external` split (bd
//     coalesces server-side before export).
//   - Issue-level actor is `created_by` (not `author`); GitHub identity is
//     `owner` (ignored, as server-mode ignores it).
//   - Due date is `due_at` (not `due_date`).
//   - Comments are inline with `author`.
const embeddedExportFixture = `{"_type":"issue","id":"emb-a","title":"Issue A","description":"first","status":"open","priority":1,"issue_type":"task","assignee":"alice","owner":"owner@example.com","estimated_minutes":90,"created_at":"2026-07-01T17:03:56Z","created_by":"actor-sms","updated_at":"2026-07-01T17:07:18Z","due_at":"2026-12-31T00:00:00Z","labels":["area:data","area:tui"],"dependencies":[{"issue_id":"emb-a","depends_on_id":"emb-b","type":"blocks","created_at":"2026-07-01T13:04:57Z","created_by":"actor-sms","metadata":"{}"}],"comments":[{"id":"019f1ea3-823c-7d8c-ba76-4b7e919d44f9","issue_id":"emb-a","author":"actor-sms","text":"a comment","created_at":"2026-07-01T17:04:20Z"}],"dependency_count":1,"dependent_count":0,"comment_count":1}
{"_type":"issue","id":"emb-b","title":"Issue B","description":"second","status":"closed","priority":2,"issue_type":"bug","owner":"owner@example.com","created_at":"2026-07-01T17:04:19Z","created_by":"actor-sms","updated_at":"2026-07-01T17:04:58Z","closed_at":"2026-07-01T17:04:58Z","close_reason":"done for fixture","labels":["area:tui"],"dependency_count":0,"dependent_count":1,"comment_count":0}
{"_type":"issue","id":"emb-c","title":"Issue C","description":"third","status":"open","priority":3,"issue_type":"feature","created_at":"2026-07-01T17:04:19Z","created_by":"actor-bob","updated_at":"2026-07-01T17:04:58Z","await_type":"human","await_id":"human-approval","ephemeral":true,"is_template":false,"mol_type":"molecule","metadata":{"action":"claimed"},"created_by_session":"sess-123","claimed_by_session":"sess-456","notes":"some notes","design":"some design","acceptance_criteria":"- [ ] done"}`

// indexByID builds a lookup so assertions don't depend on slice ordering
// (the loader preserves input order, but ordering is not the contract here).
func indexByID(issues []model.Issue) map[string]model.Issue {
	m := make(map[string]model.Issue, len(issues))
	for _, is := range issues {
		m[is.ID] = is
	}
	return m
}

// TestParseIssues_EmbeddedExportRoundTrip is the fidelity guard for the
// embedded read path (bt-ij71a). It feeds real-shape `bd export` v50 output
// through the loader and asserts that labels, dependencies (with the flat
// polymorphic representation), comments, and the v50 field surface all
// populate model.Issue — matching what the server-mode SQL reader produces.
func TestParseIssues_EmbeddedExportRoundTrip(t *testing.T) {
	issues, err := loader.ParseIssues(strings.NewReader(embeddedExportFixture))
	if err != nil {
		t.Fatalf("ParseIssues returned error: %v", err)
	}
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}
	byID := indexByID(issues)

	a, ok := byID["emb-a"]
	if !ok {
		t.Fatal("issue emb-a missing")
	}

	// Actor: bd export emits `created_by`; server-mode maps it to Author.
	if a.Author != "actor-sms" {
		t.Errorf("emb-a Author = %q, want %q (from created_by)", a.Author, "actor-sms")
	}
	// `owner` is a distinct field the server-mode reader ignores; it must not
	// leak into Author or Assignee.
	if a.Assignee != "alice" {
		t.Errorf("emb-a Assignee = %q, want %q", a.Assignee, "alice")
	}
	if a.EstimatedMinutes == nil || *a.EstimatedMinutes != 90 {
		t.Errorf("emb-a EstimatedMinutes = %v, want 90", a.EstimatedMinutes)
	}
	// Due date: bd export emits `due_at`, model tag is `due_date`.
	if a.DueDate == nil {
		t.Error("emb-a DueDate is nil, want 2026-12-31 (from due_at)")
	} else if a.DueDate.Year() != 2026 || a.DueDate.Month() != 12 || a.DueDate.Day() != 31 {
		t.Errorf("emb-a DueDate = %v, want 2026-12-31", a.DueDate)
	}

	// Labels.
	if len(a.Labels) != 2 || a.Labels[0] != "area:data" || a.Labels[1] != "area:tui" {
		t.Errorf("emb-a Labels = %v, want [area:data area:tui]", a.Labels)
	}

	// Dependency: flat depends_on_id + type (the v50 polymorphic-format guard).
	if len(a.Dependencies) != 1 {
		t.Fatalf("emb-a Dependencies len = %d, want 1", len(a.Dependencies))
	}
	dep := a.Dependencies[0]
	if dep.IssueID != "emb-a" {
		t.Errorf("dep IssueID = %q, want emb-a", dep.IssueID)
	}
	if dep.DependsOnID != "emb-b" {
		t.Errorf("dep DependsOnID = %q, want emb-b (flat depends_on_id)", dep.DependsOnID)
	}
	if dep.Type != model.DependencyType("blocks") {
		t.Errorf("dep Type = %q, want blocks", dep.Type)
	}

	// Comment: inline, `author` key.
	if len(a.Comments) != 1 {
		t.Fatalf("emb-a Comments len = %d, want 1", len(a.Comments))
	}
	cm := a.Comments[0]
	if cm.Author != "actor-sms" {
		t.Errorf("comment Author = %q, want actor-sms", cm.Author)
	}
	if cm.Text != "a comment" {
		t.Errorf("comment Text = %q, want %q", cm.Text, "a comment")
	}
	if cm.ID != "019f1ea3-823c-7d8c-ba76-4b7e919d44f9" {
		t.Errorf("comment ID = %q, want UUID", cm.ID)
	}

	// Closed issue: closed_at + close_reason.
	b, ok := byID["emb-b"]
	if !ok {
		t.Fatal("issue emb-b missing")
	}
	if b.Status != model.Status("closed") {
		t.Errorf("emb-b Status = %q, want closed", b.Status)
	}
	if b.ClosedAt == nil {
		t.Error("emb-b ClosedAt is nil, want set")
	}
	if b.CloseReason == nil || *b.CloseReason != "done for fixture" {
		t.Errorf("emb-b CloseReason = %v, want %q", b.CloseReason, "done for fixture")
	}
	if b.Author != "actor-sms" {
		t.Errorf("emb-b Author = %q, want actor-sms (from created_by)", b.Author)
	}

	// v50 field surface: gate, molecule, session-provenance, metadata.
	c, ok := byID["emb-c"]
	if !ok {
		t.Fatal("issue emb-c missing")
	}
	if c.AwaitType == nil || *c.AwaitType != "human" {
		t.Errorf("emb-c AwaitType = %v, want human", c.AwaitType)
	}
	if c.AwaitID == nil || *c.AwaitID != "human-approval" {
		t.Errorf("emb-c AwaitID = %v, want human-approval", c.AwaitID)
	}
	if c.Ephemeral == nil || *c.Ephemeral != true {
		t.Errorf("emb-c Ephemeral = %v, want true", c.Ephemeral)
	}
	if c.IsTemplate == nil || *c.IsTemplate != false {
		t.Errorf("emb-c IsTemplate = %v, want false", c.IsTemplate)
	}
	if c.MolType == nil || *c.MolType != "molecule" {
		t.Errorf("emb-c MolType = %v, want molecule", c.MolType)
	}
	if _, hasAction := c.Metadata["action"]; !hasAction {
		t.Errorf("emb-c Metadata missing 'action' key, got %v", c.Metadata)
	}
	if c.CreatedBySession != "sess-123" {
		t.Errorf("emb-c CreatedBySession = %q, want sess-123", c.CreatedBySession)
	}
	if c.ClaimedBySession != "sess-456" {
		t.Errorf("emb-c ClaimedBySession = %q, want sess-456", c.ClaimedBySession)
	}
	if c.Notes != "some notes" || c.Design != "some design" || c.AcceptanceCriteria != "- [ ] done" {
		t.Errorf("emb-c long-text fields not populated: notes=%q design=%q acceptance=%q", c.Notes, c.Design, c.AcceptanceCriteria)
	}
	if c.Author != "actor-bob" {
		t.Errorf("emb-c Author = %q, want actor-bob (from created_by)", c.Author)
	}
}
