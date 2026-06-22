package ui

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// exercise Phase2Ready and FileChanged branches of Update for coverage.
func TestModelUpdatePhase2AndFileChanged(t *testing.T) {
	issues := []model.Issue{{ID: "A", Title: "Alpha", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)
	m.width, m.height = 120, 40

	// Phase2ReadyMsg should rebuild insights/graph without error
	ins := m.data.analysis.GenerateInsights(len(issues))
	updated, _ := m.Update(Phase2ReadyMsg{Stats: m.data.analysis, Insights: ins})
	m2 := updated.(Model)
	if m2.insightsPanel.insights.Stats == nil {
		t.Fatalf("expected insights to be regenerated")
	}
	if len(m2.ac.priorityHints) == 0 {
		t.Fatalf("expected priority hints populated after Phase2Ready")
	}

	// FileChangedMsg with empty beadsPath should simply re-arm watcher (no panic)
	if updated2, cmd := m2.Update(FileChangedMsg{}); updated2.(Model).statusMsg != m2.statusMsg {
		_ = cmd // command may be nil; just ensure no panic and type matches
	}
}

type badItem struct{}

func (badItem) Title() string       { return "bad" }
func (badItem) Description() string { return "bad" }
func (badItem) FilterValue() string { return "bad" }

func TestCopyIssueToClipboardInvalidItem(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.list.SetItems([]list.Item{badItem{}})
	m.list.Select(0)
	m.copyIssueToClipboard()
	if m.statusSeverity < SeverityFailure || m.statusMsg == "" {
		t.Fatalf("expected error copying invalid item, got %q", m.statusMsg)
	}
}

func TestEnterTimeTravelModeGracefulFailure(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	_ = os.Chdir(tmp)

	m := NewModel(nil, nil, "", nil)
	m.enterTimeTravelMode("HEAD")
	if m.statusSeverity < SeverityFailure {
		t.Fatalf("expected error when not in git repo")
	}
}

func TestInsightsCurrentPanelItemCount(t *testing.T) {
	ins := analysis.Insights{
		Bottlenecks:  []analysis.InsightItem{{ID: "B"}},
		Keystones:    []analysis.InsightItem{{ID: "K"}},
		Influencers:  []analysis.InsightItem{{ID: "I"}},
		Hubs:         []analysis.InsightItem{{ID: "H"}},
		Authorities:  []analysis.InsightItem{{ID: "A"}},
		Cores:        []analysis.InsightItem{{ID: "C"}},
		Articulation: []string{"ART"},
		Slack:        []analysis.InsightItem{{ID: "S"}},
		Cycles:       [][]string{{"X", "Y"}},
		Stats:        analysis.NewGraphStatsForTest(nil, nil, nil, nil, nil, nil, nil, nil, nil, 0, nil),
	}
	m := NewInsightsModel(ins, map[string]*model.Issue{}, DefaultTheme())
	m.SetTopPicks([]analysis.TopPick{{ID: "P1", Score: 1.0}})
	counts := []int{m.currentPanelItemCount()}
	for i := 0; i < int(PanelCount)-1; i++ {
		m.NextPanel()
		counts = append(counts, m.currentPanelItemCount())
	}
	for idx, c := range counts {
		if c == 0 {
			t.Fatalf("panel %d reported zero items unexpectedly", idx)
		}
	}
}

func TestUpdateFileChangedReloadsSelection(t *testing.T) {
	data := `{"id":"ONE","title":"One","status":"open"}`
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(data), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	m := NewModel(nil, nil, beads, nil)
	defer m.Stop()
	m.list.SetItems([]list.Item{IssueItem{Issue: model.Issue{ID: "ONE", Title: "One", Status: model.StatusOpen}}})
	m.list.Select(0)

	updated, cmd := m.Update(FileChangedMsg{})
	_ = cmd
	m2 := updated.(Model)
	if m2.statusSeverity >= SeverityFailure {
		t.Fatalf("expected successful reload, got error %q", m2.statusMsg)
	}
}

func TestNewModel_SetsTreeBeadsDirFromBeadsPath(t *testing.T) {
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(`{"id":"ONE","title":"One","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	m := NewModel(nil, nil, beads, nil)
	defer m.Stop()

	if got, want := m.tree.beadsDir, filepath.Dir(beads); got != want {
		t.Fatalf("expected tree beadsDir %q, got %q", want, got)
	}
}

// TestResizeDebounce_StaleSettleMsgIgnored verifies the generation-counter
// pattern (bt-kfkrb): when N WindowSizeMsgs arrive in a burst, only the
// resizeSettledMsg carrying the latest gen triggers the heavy path. Older
// settled messages are no-ops.
func TestResizeDebounce_StaleSettleMsgIgnored(t *testing.T) {
	issues := []model.Issue{{ID: "bt-1", Title: "Resize Test", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)

	// Send a burst of 3 WindowSizeMsgs with increasing widths.
	const (
		w1 = 120
		w2 = 140
		w3 = 160
	)
	for _, w := range []int{w1, w2, w3} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 40})
		m = updated.(Model)
	}

	// After the burst, resizeGen must be 3 (one per WindowSizeMsg). Phase 1
	// does update m.width and the cheap chrome layout, so m.width is w3 here.
	if m.resizeGen != 3 {
		t.Fatalf("expected resizeGen=3 after 3 WindowSizeMsgs, got %d", m.resizeGen)
	}
	if m.width != w3 {
		t.Fatalf("expected m.width=%d after burst (phase 1 commits dims), got %d", w3, m.width)
	}

	// A stale settle message (gen 1) must be a no-op: resizeGen stays at 3.
	staleSettle := resizeSettledMsg{gen: 1}
	updated, _ := m.Update(staleSettle)
	m2 := updated.(Model)
	if m2.resizeGen != 3 {
		t.Fatalf("stale resizeSettledMsg changed resizeGen: expected 3, got %d", m2.resizeGen)
	}

	// The viewport width after a stale settle is unchanged from after the burst.
	vpWidthBeforeSettle := m2.viewport.Width()

	// The current-gen settle message must run the heavy path: resizeGen unchanged.
	finalSettle := resizeSettledMsg{gen: 3}
	updated2, _ := m2.Update(finalSettle)
	m3 := updated2.(Model)
	if m3.resizeGen != 3 {
		t.Fatalf("final resizeSettledMsg modified resizeGen: expected 3, got %d", m3.resizeGen)
	}
	if m3.viewport.Width() != vpWidthBeforeSettle {
		t.Fatalf("final settle changed viewport width unexpectedly: before=%d after=%d",
			vpWidthBeforeSettle, m3.viewport.Width())
	}
}

// TestResizeDebounce_InsightsDeferredToSettle verifies that the insights
// panel SetSize (which rebuilds a Glamour renderer when width > 120) is
// deferred to phase 2, not run on every WindowSizeMsg (bt-kfkrb regression,
// originally filed as bt-jqst3).
func TestResizeDebounce_InsightsDeferredToSettle(t *testing.T) {
	issues := []model.Issue{{ID: "bt-1", Title: "Resize Test", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)

	// Send a burst of WindowSizeMsgs. The insights panel's width must NOT
	// advance during the burst -- it is only updated when the gen-current
	// settle tick fires.
	const w1, w2, w3 = 140, 160, 180
	for _, w := range []int{w1, w2, w3} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 40})
		m = updated.(Model)
	}
	if m.insightsPanel.width == w3 {
		t.Fatalf("insightsPanel.width=%d after burst, expected unchanged (deferred to settle)", m.insightsPanel.width)
	}

	// A stale settle (older gen) is a no-op for insights too.
	updated, _ := m.Update(resizeSettledMsg{gen: 1})
	m2 := updated.(Model)
	if m2.insightsPanel.width == w3 {
		t.Fatalf("stale settle should not advance insightsPanel.width, got %d", m2.insightsPanel.width)
	}

	// Current-gen settle runs applyWindowSizeHeavy, which now also resizes
	// the insights panel. Width must reflect the latest burst event.
	updated2, _ := m2.Update(resizeSettledMsg{gen: m2.resizeGen})
	m3 := updated2.(Model)
	wantBodyW := m3.bodyWidth()
	if m3.insightsPanel.width != wantBodyW {
		t.Fatalf("after settle, insightsPanel.width=%d want=%d", m3.insightsPanel.width, wantBodyW)
	}
}

// TestResizeDebounce_Phase1LayoutSync verifies that phase 1 (every
// WindowSizeMsg) synchronously updates list size and isSplitView without
// waiting for the settle tick (bt-kfkrb). Existing chrome-layout tests rely
// on this; only the renderer rebuild + viewport content is deferred.
func TestResizeDebounce_Phase1LayoutSync(t *testing.T) {
	m := NewModel(nil, nil, "", nil)

	// Below SplitViewThreshold -- list should consume full body width.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m80 := updated.(Model)
	if m80.isSplitView {
		t.Fatalf("width=80 should not be split view (threshold %d)", SplitViewThreshold)
	}

	// Above SplitViewThreshold -- list width should be a fraction of the body.
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m160 := updated.(Model)
	if !m160.isSplitView {
		t.Fatalf("width=160 should be split view (threshold %d)", SplitViewThreshold)
	}
	if m160.list.Width() >= 160 {
		t.Fatalf("split-view list width (%d) should be < terminal width (160)", m160.list.Width())
	}
	if m160.resizeGen < 1 {
		t.Fatalf("expected resizeGen >= 1 after WindowSizeMsg, got %d", m160.resizeGen)
	}
}
