package export

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestSQLiteExporter_InsertsMetricsAndTriageRecommendations(t *testing.T) {
	now := time.Now().UTC()
	issues := []*model.Issue{
		{
			ID:          "A",
			Title:       "Issue A",
			Description: "A desc",
			Status:      model.StatusOpen,
			Priority:    1,
			IssueType:   model.TypeTask,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "B",
			Title:       "Issue B",
			Description: "B desc",
			Status:      model.StatusOpen,
			Priority:    2,
			IssueType:   model.TypeTask,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	deps := []*model.Dependency{
		{
			IssueID:     "A",
			DependsOnID: "B",
			Type:        model.DepBlocks,
			CreatedAt:   now,
			CreatedBy:   "test",
		},
	}

	stats := analysis.NewGraphStatsForTest(
		map[string]float64{"A": 0.25, "B": 0.75}, // pageRank
		map[string]float64{"A": 0.10, "B": 0.20}, // betweenness
		nil, nil, nil,
		map[string]float64{"A": 2, "B": 1}, // criticalPathScore
		map[string]int{"A": 1, "B": 0},     // outDegree
		map[string]int{"A": 0, "B": 1},     // inDegree
		nil, 0, []string{"B", "A"},         // cycles, density, topo
	)

	triage := &analysis.TriageResult{
		Recommendations: []analysis.Recommendation{
			{
				ID:          "A",
				Title:       "Issue A",
				Type:        "task",
				Status:      "open",
				Priority:    1,
				Score:       0.9,
				Action:      "work",
				Reasons:     []string{"high impact", "unblocks B"},
				UnblocksIDs: []string{"X"},
				BlockedBy:   []string{"B"},
			},
		},
	}

	exporter := NewSQLiteExporter(issues, deps, stats, triage)
	exporter.Config.IncludeRobotOutputs = false

	outDir := t.TempDir()
	if err := exporter.Export(outDir); err != nil {
		t.Fatalf("Export returned error: %v", err)
	}

	dbPath := filepath.Join(outDir, "beads.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var pr, bw, score float64
	var cp, blocks, blockedBy int
	row := db.QueryRow(`SELECT pagerank, betweenness, critical_path_depth, triage_score, blocks_count, blocked_by_count FROM issue_metrics WHERE issue_id = ?`, "A")
	if err := row.Scan(&pr, &bw, &cp, &score, &blocks, &blockedBy); err != nil {
		t.Fatalf("Scan issue_metrics for A: %v", err)
	}
	if math.Abs(pr-0.25) > 1e-9 {
		t.Fatalf("pagerank expected 0.25, got %v", pr)
	}
	if math.Abs(bw-0.10) > 1e-9 {
		t.Fatalf("betweenness expected 0.10, got %v", bw)
	}
	if cp != 2 {
		t.Fatalf("critical_path_depth expected 2, got %d", cp)
	}
	if math.Abs(score-0.9) > 1e-9 {
		t.Fatalf("triage_score expected 0.9, got %v", score)
	}
	// A depends on B, so A is blocked by B (blocks=0, blockedBy=1)
	if blocks != 0 {
		t.Fatalf("blocks_count for A expected 0, got %d", blocks)
	}
	if blockedBy != 1 {
		t.Fatalf("blocked_by_count for A expected 1, got %d", blockedBy)
	}

	row = db.QueryRow(`SELECT triage_score, blocks_count, blocked_by_count FROM issue_metrics WHERE issue_id = ?`, "B")
	if err := row.Scan(&score, &blocks, &blockedBy); err != nil {
		t.Fatalf("Scan issue_metrics for B: %v", err)
	}
	if math.Abs(score-0.0) > 1e-9 {
		t.Fatalf("triage_score for B expected 0.0, got %v", score)
	}
	// A depends on B, so B blocks A (blocks=1, blockedBy=0)
	if blocks != 1 {
		t.Fatalf("blocks_count for B expected 1, got %d", blocks)
	}
	if blockedBy != 0 {
		t.Fatalf("blocked_by_count for B expected 0, got %d", blockedBy)
	}

	var action string
	var reasonsJSON, unblocksJSON, blockedByJSON string
	row = db.QueryRow(`SELECT action, reasons, unblocks_ids, blocked_by_ids FROM triage_recommendations WHERE issue_id = ?`, "A")
	if err := row.Scan(&action, &reasonsJSON, &unblocksJSON, &blockedByJSON); err != nil {
		t.Fatalf("Scan triage_recommendations for A: %v", err)
	}
	if action != "work" {
		t.Fatalf("action expected %q, got %q", "work", action)
	}
	var reasons []string
	if err := json.Unmarshal([]byte(reasonsJSON), &reasons); err != nil {
		t.Fatalf("Unmarshal reasons: %v", err)
	}
	if len(reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(reasons))
	}
	var unblocks []string
	if err := json.Unmarshal([]byte(unblocksJSON), &unblocks); err != nil {
		t.Fatalf("Unmarshal unblocks_ids: %v", err)
	}
	if len(unblocks) != 1 || unblocks[0] != "X" {
		t.Fatalf("unexpected unblocks_ids: %+v", unblocks)
	}
	var blockedByIDs []string
	if err := json.Unmarshal([]byte(blockedByJSON), &blockedByIDs); err != nil {
		t.Fatalf("Unmarshal blocked_by_ids: %v", err)
	}
	if len(blockedByIDs) != 1 || blockedByIDs[0] != "B" {
		t.Fatalf("unexpected blocked_by_ids: %+v", blockedByIDs)
	}
}

func TestSQLiteExporter_writeRobotOutputs_WritesExpectedFiles(t *testing.T) {
	now := time.Now().UTC()
	issues := []*model.Issue{{
		ID:          "A",
		Title:       "Issue A",
		Description: "A desc",
		Status:      model.StatusOpen,
		Priority:    1,
		IssueType:   model.TypeTask,
		CreatedAt:   now,
		UpdatedAt:   now,
	}}

	triage := &analysis.TriageResult{
		ProjectHealth: analysis.ProjectHealth{
			Counts: analysis.HealthCounts{
				Total: 1,
				Open:  1,
			},
		},
		Recommendations: []analysis.Recommendation{
			{
				ID:     "A",
				Title:  "Issue A",
				Score:  0.5,
				Action: "work",
			},
		},
	}

	exporter := NewSQLiteExporter(issues, nil, (*analysis.GraphStats)(nil), triage)
	exporter.Config.Title = "Test Title"
	exporter.SetGitHash("deadbeef")

	dataDir := t.TempDir()
	if err := exporter.writeRobotOutputs(dataDir); err != nil {
		t.Fatalf("writeRobotOutputs returned error: %v", err)
	}

	for _, name := range []string{"triage.json", "project_health.json", "meta.json"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err != nil {
			t.Fatalf("Expected %s to exist: %v", name, err)
		}
	}
}
