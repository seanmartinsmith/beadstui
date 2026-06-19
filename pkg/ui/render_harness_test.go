package ui

// Render harness for agent-in-the-loop TUI work (dogfood SOP, Tier 1).
//
// This is NOT a regression test — it has no assertions. It builds a
// representative fixture, drives the real Model through sizing + state, and
// dumps each scenario to _tmp/render/<name>.txt (ANSI stripped, for layout
// review) and <name>.ansi (raw, for piping into charmbracelet/freeze to get a
// color PNG). It is gated behind BT_RENDER_DUMP so a bare `go test ./...`
// never writes files.
//
// Usage:
//   BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump
//   # then Read _tmp/render/*.txt
//
// Why in-package: the detail-pane markdown render (updateViewportContent) is
// deferred to an async settle tick after WindowSizeMsg (handleWindowSize,
// bt-kfkrb), which an external test can't drive deterministically. In-package
// we call it directly, and can set layout fields (showDetails/focused) to put
// the model in any state without timing games.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func hptr[T any](v T) *T { return &v }

// harnessIssues builds a deliberately varied fixture: an epic with a child, a
// hot P0 bug with full long-form content + comments, a blocked bug, a
// long-prefix ID, a wisp, and a closed item. Ages are relative to now so the
// age column renders realistic values.
func harnessIssues() []model.Issue {
	now := time.Now()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	return []model.Issue{
		{
			ID: "bt-evuf", Title: "Issues pane: type-aware redesign + signal pass",
			Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeEpic,
			Author: "sms", Labels: []string{"area:tui", "ux"},
			CreatedAt: ago(30 * 24 * time.Hour), UpdatedAt: ago(2 * time.Hour),
			Description: "Umbrella for the issues-pane redesign: type-aware cells, signal pass, and a vocabulary system for status/pri/type chips.",
		},
		{
			ID: "bt-evuf.1", Title: "Per-cell vocabulary design (status/pri/type chips)",
			Status: model.StatusOpen, Priority: 2, IssueType: model.TypeFeature,
			Labels:    []string{"area:tui", "ux"},
			CreatedAt: ago(20 * 24 * time.Hour), UpdatedAt: ago(20 * 24 * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "bt-evuf.1", DependsOnID: "bt-evuf", Type: model.DepParentChild},
			},
		},
		{
			ID: "bt-0qzp", Title: "Detail pane dep graph: ANSI escape codes leak as literal characters",
			Status: model.StatusInProgress, Priority: 0, IssueType: model.TypeBug,
			Assignee: "sms", Author: "sms", Labels: []string{"area:tui", "ux"},
			CreatedAt: ago(5 * 24 * time.Hour), UpdatedAt: ago(3 * time.Hour),
			Description:        "ANSI SGR sequences from the lipgloss dep-graph render leak into the viewport as literal escape text instead of color.\n\nRepro: open any bead with children, scroll to the Dependency Graph section.",
			Design:             "Route the dep-graph block through the renderSection ANSI track (placeholder + spliceSections) rather than Glamour's chroma code-fence path.",
			AcceptanceCriteria: "- Dep graph renders with color\n- No literal escape sequences in the viewport\n- Snapshot test covers a 2-level tree",
			Notes:              "Found during 2026-06-11 dogfood pass.",
			Comments: []*model.Comment{
				{Author: "sms", Text: "still repro'ing on 90-col terminals", CreatedAt: ago(2 * time.Hour)},
				{Author: "claude", Text: "confirmed: spliceSections placeholder not emitted for the inverse parent_child branch", CreatedAt: ago(1 * time.Hour)},
			},
		},
		{
			ID: "bt-dx7k", Title: "Help overlay broken/unusable at small terminal sizes",
			Status: model.StatusBlocked, Priority: 2, IssueType: model.TypeBug,
			Labels:    []string{"area:tui", "responsive"},
			CreatedAt: ago(10 * 24 * time.Hour), UpdatedAt: ago(4 * 24 * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "bt-dx7k", DependsOnID: "bt-0qzp", Type: model.DepBlocks},
			},
		},
		{
			ID: "dotfiles-7kf2q", Title: "Long-prefix bead to exercise ID column truncation behaviour at width",
			Status: model.StatusOpen, Priority: 3, IssueType: model.TypeTask,
			Labels:    []string{"area:tui"},
			CreatedAt: ago(40 * 24 * time.Hour), UpdatedAt: ago(40 * 24 * time.Hour),
		},
		{
			ID: "bt-9kdo", Title: "ephemeral wisp scratch note",
			Status: model.StatusOpen, Priority: 4, IssueType: model.TypeTask,
			Ephemeral: hptr(true),
			CreatedAt: ago(6 * time.Hour), UpdatedAt: ago(6 * time.Hour),
		},
		{
			ID: "bt-h5jz", Title: "Add first-class support for decisions: schema, filter, dedicated view",
			Status: model.StatusClosed, Priority: 2, IssueType: model.TypeChore,
			CloseReason: hptr("Shipped schema + filter + view."),
			ClosedAt:    hptr(now.Add(-time.Hour)),
			CreatedAt:   ago(15 * 24 * time.Hour), UpdatedAt: ago(time.Hour),
		},
	}
}

func harnessSelect(m *Model, id string) {
	for i, it := range m.list.Items() {
		if issueItem, ok := it.(IssueItem); ok && issueItem.Issue.ID == id {
			m.list.Select(i)
			return
		}
	}
}

func TestRenderDump(t *testing.T) {
	if os.Getenv("BT_RENDER_DUMP") == "" {
		t.Skip("set BT_RENDER_DUMP=1 to dump TUI renders to _tmp/render")
	}

	outDir := filepath.Join("..", "..", "_tmp", "render")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}

	openDetail := func(id string) func(*Model) {
		return func(m *Model) {
			harnessSelect(m, id)
			m.showDetails = true
			m.focused = focusDetail
			m.updateViewportContent()
		}
	}
	splitOn := func(id string) func(*Model) {
		return func(m *Model) {
			harnessSelect(m, id)
			m.updateViewportContent()
		}
	}
	sprintView := func(m *Model) {
		now := time.Now()
		// Inject a stale in-progress bead so the At-Risk section renders
		// populated (in_progress with no update for > 3 days).
		m.data.issues = append(m.data.issues, model.Issue{
			ID: "bt-stale", Title: "Refactor stalled mid-flight, no update in a week",
			Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeBug,
			CreatedAt: now.Add(-12 * 24 * time.Hour), UpdatedAt: now.Add(-6 * 24 * time.Hour),
		})
		m.sprints = []model.Sprint{{
			ID:        "sprint-w3",
			Name:      "TUI polish + sprint wiring",
			StartDate: now.Add(-5 * 24 * time.Hour),
			EndDate:   now.Add(5 * 24 * time.Hour),
			BeadIDs: []string{
				"bt-evuf", "bt-evuf.1", "bt-0qzp", "bt-dx7k",
				"dotfiles-7kf2q", "bt-9kdo", "bt-h5jz", "bt-stale",
			},
		}}
		m.selectedSprint = &m.sprints[0]
		m.mode = ViewEpics
		m.focused = focusEpics
		m.sprintViewText = m.renderSprintDashboard()
	}

	scenarios := []struct {
		name  string
		w, h  int
		setup func(*Model)
	}{
		{"list_80x24", 80, 24, nil},
		{"list_70x20", 70, 20, nil}, // user's scrunched terminal
		{"list_160x40", 160, 40, nil},
		{"split_120x32", 120, 32, splitOn("bt-0qzp")},
		{"split_160x40", 160, 40, splitOn("bt-0qzp")},
		{"detail_90x28", 90, 28, openDetail("bt-0qzp")},
		{"detail_70x20", 70, 20, openDetail("bt-0qzp")},
		{"detail_epic_90x28", 90, 28, openDetail("bt-evuf")},

		// Sprint dashboard (wired to P via bt-ryi5z).
		{"sprint_100x32", 100, 32, sprintView},
		{"sprint_70x20", 70, 20, sprintView}, // scrunched terminal

		// Modal overlays — composited by View() over the (dimmed) background via
		// activeModal. Proves popups render in-position in the harness. The dim
		// backdrop is a brightness effect lost to ansi.Strip; judge that in a PNG.
		{"modal_help_70x20", 70, 20, func(m *Model) { m.openModal(ModalHelp) }}, // bt-dx7k repro
		{"modal_help_120x40", 120, 40, func(m *Model) { m.openModal(ModalHelp) }},
		{"modal_labelpicker_120x36", 120, 36, func(m *Model) { m.openModal(ModalLabelPicker) }},
		{"modal_recipepicker_120x36", 120, 36, func(m *Model) { m.openModal(ModalRecipePicker) }},
		{"modal_alerts_120x36", 120, 36, func(m *Model) { m.openModal(ModalAlerts) }},
	}

	for _, sc := range scenarios {
		// Isolate each scenario: a panic in one data-dependent modal must not
		// abort the rest of the dump.
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("%-26s PANIC: %v", sc.name, r)
				}
			}()

			issues := harnessIssues()
			m := NewModel(issues, nil, "", nil)
			nm, _ := m.Update(tea.WindowSizeMsg{Width: sc.w, Height: sc.h})
			m = nm.(Model)
			if sc.setup != nil {
				sc.setup(&m)
			}
			content := m.View().Content

			plain := ansi.Strip(content)
			if err := os.WriteFile(filepath.Join(outDir, sc.name+".txt"), []byte(plain), 0o644); err != nil {
				t.Fatalf("write %s.txt: %v", sc.name, err)
			}
			if err := os.WriteFile(filepath.Join(outDir, sc.name+".ansi"), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s.ansi: %v", sc.name, err)
			}
			t.Logf("%-26s %3dx%-3d -> %s.txt (%d lines)", sc.name, sc.w, sc.h, sc.name, strings.Count(plain, "\n")+1)
		}()
	}
}
