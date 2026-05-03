package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestRobotList_BQLFilter_IDEquality — bt-111w regression test.
//
// `bt robot list --bql 'id="X"'` must return exactly the matching issue,
// not the full unfiltered list. Before the fix, robot_list.go bypassed
// robotPreRun (where BQL filtering lives) so --bql was silently dropped.
func TestRobotList_BQLFilter_IDEquality(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "list", "--bql", `id="cass-a"`)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --bql 'id=\"cass-a\"' failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	count, _ := payload["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only cass-a should match)", count)
	}

	issues, _ := payload["issues"].([]any)
	if len(issues) != 1 {
		t.Fatalf("issues len = %d, want 1", len(issues))
	}
	first, _ := issues[0].(map[string]any)
	if id, _ := first["id"].(string); id != "cass-a" {
		t.Errorf("filtered id = %q, want cass-a", id)
	}

	// Query envelope must echo the bql string so consumers can confirm the
	// filter was applied. Before the fix the envelope omitted bql entirely.
	q, _ := payload["query"].(map[string]any)
	bql, _ := q["bql"].(string)
	if !strings.Contains(bql, "cass-a") {
		t.Errorf("query.bql = %q, want it to contain the filter expression", bql)
	}
}

// TestRobotList_BQLFilter_PriorityEquality — bt-111w regression test for
// priority equality. Two issues match priority=1 (bt-a, cass-a).
func TestRobotList_BQLFilter_PriorityEquality(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "list", "--bql", "priority=1")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --bql 'priority=1' failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	count, _ := payload["count"].(float64)
	if int(count) != 2 {
		t.Errorf("count = %v, want 2 (bt-a and cass-a are P1)", count)
	}
}

// TestRobotList_BQLFilter_StatusEquality — bt-111w regression test for
// status filtering via BQL. All three fixture issues are open.
func TestRobotList_BQLFilter_StatusEquality(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "list", "--bql", `status="open"`)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --bql 'status=\"open\"' failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	count, _ := payload["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3 (all fixture issues are open)", count)
	}
}

// TestRobotList_BQLFilter_NoMatches — BQL that matches nothing returns
// count=0 with empty issues array, not the unfiltered list.
func TestRobotList_BQLFilter_NoMatches(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "list", "--bql", `id="does-not-exist"`)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --bql with no matches should exit 0; err=%v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	count, _ := payload["count"].(float64)
	if int(count) != 0 {
		t.Errorf("count = %v, want 0 for no-match BQL", count)
	}
}

// TestRobotBQL_Limit — bt-s2bq sub-issue #1: --limit caps the result set.
// Fixture has 3 issues (bt-a P1, bt-b P2, cass-a P1); querying all open
// issues with --limit 2 must return exactly 2, with total_count=3.
func TestRobotBQL_Limit(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "bql", "--query", `status="open"`, "--limit", "2")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bql --limit 2 failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// count reflects the returned window, total_count reflects the full match set.
	count, _ := payload["count"].(float64)
	if int(count) != 2 {
		t.Errorf("count = %v, want 2 (--limit 2)", count)
	}
	totalCount, _ := payload["total_count"].(float64)
	if int(totalCount) != 3 {
		t.Errorf("total_count = %v, want 3 (all 3 open issues matched)", totalCount)
	}
	issues, _ := payload["issues"].([]any)
	if len(issues) != 2 {
		t.Errorf("issues len = %d, want 2", len(issues))
	}
}

// TestRobotBQL_Offset — --offset skips the first N issues.
func TestRobotBQL_Offset(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	// All 3 open issues; offset=2 should return exactly 1.
	cmd := exec.Command(exe, "robot", "bql", "--query", `status="open"`, "--offset", "2")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bql --offset 2 failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	count, _ := payload["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (3 total - offset 2)", count)
	}
	totalCount, _ := payload["total_count"].(float64)
	if int(totalCount) != 3 {
		t.Errorf("total_count = %v, want 3", totalCount)
	}
}

// TestRobotBQL_LimitZeroUnlimited — --limit 0 returns all results (unlimited).
func TestRobotBQL_LimitZeroUnlimited(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "bql", "--query", `status="open"`, "--limit", "0")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bql --limit 0 failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	count, _ := payload["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3 (limit=0 is unlimited)", count)
	}
}

// TestRobotList_BQLFilter_CombinedWithSource — BQL + --source compose
// correctly: source narrows first, then BQL filters within the scoped set.
func TestRobotList_BQLFilter_CombinedWithSource(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	// --source=cass narrows to {cass-a}; BQL priority=1 inside that set
	// matches cass-a. Without the fix, BQL was dropped entirely so the
	// pre-source list of all 3 issues would be returned.
	cmd := exec.Command(exe, "robot", "list", "--source", "cass", "--bql", "priority=1")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --source=cass --bql 'priority=1' failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}

	count, _ := payload["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only cass-a is P1 within --source=cass)", count)
	}
	issues, _ := payload["issues"].([]any)
	first, _ := issues[0].(map[string]any)
	if id, _ := first["id"].(string); id != "cass-a" {
		t.Errorf("filtered id = %q, want cass-a", id)
	}
}
