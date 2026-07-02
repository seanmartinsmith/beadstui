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

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
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
	// injectEpicCorpus enriches the bt lane: a stale child on bt-evuf (At-Risk),
	// a second epic bt-grid with a full done/in-prog/blocked/open spread, and a
	// nested child-epic bt-dnd under it (the tree's drill + composition-bar
	// signal, and the root-epic-dedup case).
	injectEpicCorpus := func(m *Model) {
		now := time.Now()
		pc := func(child, parent string) []*model.Dependency {
			return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
		}
		m.data.issues = append(m.data.issues,
			model.Issue{ID: "bt-evuf.2", Title: "Refactor stalled mid-flight, no update in a week",
				Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeBug,
				CreatedAt: now.Add(-12 * 24 * time.Hour), UpdatedAt: now.Add(-6 * 24 * time.Hour),
				Dependencies: pc("bt-evuf.2", "bt-evuf")},
			model.Issue{ID: "bt-grid", Title: "Board: column virtualization + drag-and-drop",
				Status: model.StatusOpen, IssueType: model.TypeEpic, UpdatedAt: now},
			model.Issue{ID: "bt-grid.1", Title: "Virtualize long columns", Status: model.StatusClosed, UpdatedAt: now, Dependencies: pc("bt-grid.1", "bt-grid")},
			model.Issue{ID: "bt-grid.2", Title: "Drag-and-drop between columns", Status: model.StatusInProgress, UpdatedAt: now, Dependencies: pc("bt-grid.2", "bt-grid")},
			model.Issue{ID: "bt-grid.3", Title: "Keyboard reorder fallback", Status: model.StatusBlocked, UpdatedAt: now, Dependencies: pc("bt-grid.3", "bt-grid")},
			model.Issue{ID: "bt-grid.4", Title: "Persist column order per project", Status: model.StatusOpen, UpdatedAt: now, Dependencies: pc("bt-grid.4", "bt-grid")},
			model.Issue{ID: "bt-dnd", Title: "Drag substrate (nested epic)", Status: model.StatusOpen, IssueType: model.TypeEpic, UpdatedAt: now, Dependencies: pc("bt-dnd", "bt-grid")},
			model.Issue{ID: "bt-dnd.1", Title: "Pointer capture", Status: model.StatusClosed, UpdatedAt: now, Dependencies: pc("bt-dnd.1", "bt-dnd")},
			model.Issue{ID: "bt-dnd.2", Title: "Drop-target hit testing", Status: model.StatusInProgress, UpdatedAt: now, Dependencies: pc("bt-dnd.2", "bt-dnd")},
		)
	}
	injectSymLane := func(m *Model) {
		now := time.Now()
		pc := func(child, parent string) []*model.Dependency {
			return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
		}
		m.data.issues = append(m.data.issues,
			model.Issue{ID: "sym-sync", Title: "Cross-machine session sync", Status: model.StatusOpen, IssueType: model.TypeEpic, UpdatedAt: now},
			model.Issue{ID: "sym-sync.1", Title: "Conflict resolution", Status: model.StatusOpen, UpdatedAt: now, Dependencies: pc("sym-sync.1", "sym-sync")},
			model.Issue{ID: "sym-sync.2", Title: "Delta transport", Status: model.StatusInProgress, UpdatedAt: now, Dependencies: pc("sym-sync.2", "sym-sync")},
			model.Issue{ID: "sym-sync.3", Title: "Bootstrap a fresh box", Status: model.StatusClosed, UpdatedAt: now, Dependencies: pc("sym-sync.3", "sym-sync")},
		)
	}
	enterEpics := func(m *Model) {
		m.mode = ViewEpics
		m.focused = focusEpics
		m.refreshEpicsForCurrentFilter()
	}
	epicsTreeCollapsed := func(m *Model) {
		injectEpicCorpus(m)
		enterEpics(m)
	}
	epicsTreeExpanded := func(m *Model) {
		injectEpicCorpus(m)
		enterEpics(m)
		m.epicsTree.expand("bt-evuf")
		m.epicsTree.expand("bt-grid")
		m.epicsViewText = m.epicsTree.View()
	}
	epicsTreeMultilane := func(m *Model) {
		injectEpicCorpus(m)
		injectSymLane(m)
		enterEpics(m)
	}
	// enterBoard / enterGraph mirror the real toggle handlers (refresh from the
	// current filter) so the Phase 3 footer center zone shows live counts
	// ("4 cols · N cards", "N nodes · M edges").
	enterBoard := func(m *Model) {
		m.mode = ViewBoard
		m.focused = focusBoard
		m.refreshBoardAndGraphForCurrentFilter()
	}
	enterGraph := func(id string) func(*Model) {
		return func(m *Model) {
			harnessSelect(m, id)
			m.mode = ViewGraph
			m.focused = focusGraph
			m.refreshBoardAndGraphForCurrentFilter()
			m.graphView.SelectByID(id)
		}
	}
	// enterActionable / enterFlowMatrix / enterInsights mirror the real toggle
	// handlers so the footer-pin audit (bt-yyked follow-up) has ground-truth
	// dumps for the views the resume note flagged as m.height-2 suspects.
	enterActionable := func(m *Model) {
		m.mode = ViewActionable
		m.focused = focusActionable
	}
	enterFlowMatrix := func(m *Model) {
		cfg := analysis.DefaultLabelHealthConfig()
		flow := analysis.ComputeCrossLabelFlow(m.data.issues, cfg)
		m.mode = ViewFlowMatrix
		m.focused = focusFlowMatrix
		m.flowMatrix = NewFlowMatrixModel(m.theme)
		m.flowMatrix.SetData(&flow, m.data.issues)
	}
	enterInsights := func(m *Model) {
		m.openInsightsView()
	}
	epicCard := func(m *Model) {
		now := time.Now()
		// Same fixture as epicsView: a stale in-progress child plus the
		// fixture's closed/open children, so the card shows mixed pills.
		m.data.issues = append(m.data.issues, model.Issue{
			ID: "bt-evuf.2", Title: "Refactor stalled mid-flight, no update in a week",
			Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeBug,
			CreatedAt: now.Add(-12 * 24 * time.Hour), UpdatedAt: now.Add(-6 * 24 * time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "bt-evuf.2", DependsOnID: "bt-evuf", Type: model.DepParentChild}},
		})
		m.openEpicCard("bt-evuf")
	}

	scenarios := []struct {
		name  string
		w, h  int
		setup func(*Model)
	}{
		{"list_80x24", 80, 24, nil},
		{"list_70x20", 70, 20, nil}, // user's scrunched terminal
		{"list_160x40", 160, 40, nil},
		// Footer status breakdown must scope to the active filter (Phase 2,
		// bt-gcuv generalization): the OPEN filter excludes closed, so the
		// footer's ●closed count and total drop accordingly.
		{"list_openfilter_100x29", 100, 29, func(m *Model) {
			m.filter.currentFilter = "open"
			m.applyFilter()
		}},
		{"list_closedfilter_100x29", 100, 29, func(m *Model) {
			m.filter.currentFilter = "closed"
			m.applyFilter()
		}},
		{"split_120x32", 120, 32, splitOn("bt-0qzp")},
		{"split_160x40", 160, 40, splitOn("bt-0qzp")},
		{"detail_90x28", 90, 28, openDetail("bt-0qzp")},
		{"detail_70x20", 70, 20, openDetail("bt-0qzp")},
		{"detail_epic_90x28", 90, 28, openDetail("bt-evuf")},

		// Phase 3 per-view center meaning: detail = bead id + position (above),
		// board = cols/cards, graph = nodes/edges. 70-col proves the override
		// degrades without wrapping at the user's scrunched width.
		{"board_100x32", 100, 32, enterBoard},
		{"board_70x20", 70, 20, enterBoard},
		{"graph_100x32", 100, 32, enterGraph("bt-0qzp")},
		{"graph_70x20", 70, 20, enterGraph("bt-0qzp")},

		// Footer-pin audit (bt-yyked follow-up): views suspected of a
		// bottom-gap / footer-clip so the dump shows whether the footer lands
		// on the final row at the user's scrunched height.
		{"actionable_100x32", 100, 32, enterActionable},
		{"actionable_70x20", 70, 20, enterActionable},
		{"flowmatrix_100x32", 100, 32, enterFlowMatrix},
		{"insights_100x32", 100, 32, enterInsights},

		// Epics overview, redesigned as a full-sheet project-grouped tree (bt-3ftfm.1).
		{"epics_tree_100x32", 100, 32, epicsTreeCollapsed},
		{"epics_tree_expanded_120x40", 120, 40, epicsTreeExpanded},
		{"epics_tree_70x20", 70, 20, epicsTreeCollapsed}, // scrunched terminal (hard gate)
		{"epics_tree_multilane_120x40", 120, 40, epicsTreeMultilane},

		// Epic focus card (tier 2, bt-gfxhz.3).
		{"epic_card_100x32", 100, 32, epicCard},
		{"epic_card_70x20", 70, 20, epicCard}, // scrunched terminal

		// Modal overlays — composited by View() over the (dimmed) background via
		// activeModal. Proves popups render in-position in the harness. The dim
		// backdrop is a brightness effect lost to ansi.Strip; judge that in a PNG.
		{"modal_help_70x20", 70, 20, func(m *Model) { m.openModal(ModalHelp) }}, // bt-dx7k repro -> now mini
		{"modal_help_120x40", 120, 40, func(m *Model) { m.openModal(ModalHelp) }},
		{"modal_help_50x14", 50, 14, func(m *Model) { m.openModal(ModalHelp) }}, // bt-dx7k hard gate -> mini
		{"modal_help_30x20", 30, 20, func(m *Model) { m.openModal(ModalHelp) }}, // bt-dx7k 1-col -> mini
		// bt-dx7k.1 mini/full tiers: explicit short cases (mini) + a clearly-tall case (full sheet).
		{"modal_help_mini_80x12", 80, 12, func(m *Model) { m.openModal(ModalHelp) }},   // 2-col mini at the maintainer's scrunched size
		{"modal_help_mini_160x14", 160, 14, func(m *Model) { m.openModal(ModalHelp) }}, // wide but short -> still mini
		{"modal_help_full_120x44", 120, 44, func(m *Model) { m.openModal(ModalHelp) }}, // tall -> full grouped sheet
		{"modal_help_full_160x48", 160, 48, func(m *Model) { m.openModal(ModalHelp) }}, // large window -> column count vs vertical fill
		{"modal_help_full_220x56", 220, 56, func(m *Model) { m.openModal(ModalHelp) }}, // very large (approx the maximized window)

		// ; sidebar scoping (bt-dx7k): view-only bindings in List (no Global
		// prefix) and the empty-view fallback in Attention (no view-specific map).
		{"sidebar_list_100x32", 100, 32, func(m *Model) { m.showShortcutsSidebar = true }},
		{"sidebar_attention_100x32", 100, 32, func(m *Model) {
			m.mode = ViewAttention
			m.focused = focusInsights
			m.showShortcutsSidebar = true
		}},
		{"modal_labelpicker_120x36", 120, 36, func(m *Model) { m.openModal(ModalLabelPicker) }},
		{"modal_recipepicker_120x36", 120, 36, func(m *Model) { m.openModal(ModalRecipePicker) }},
		{"modal_alerts_120x36", 120, 36, func(m *Model) { m.openModal(ModalAlerts) }},

		// Footer Phase 4 notification states: toast (success/failure/degraded) and
		// bell badge. Proves the right-zone layout at representative widths.
		{"footer_success_100x24", 100, 24, func(m *Model) { m.setStatus("reloaded +3 -1") }},
		{"footer_failure_100x24", 100, 24, func(m *Model) { m.setFailure("write failed: db locked") }},
		{"footer_degraded_80x24", 80, 24, func(m *Model) { m.setDegraded("Dolt server unreachable (retrying in 5s)") }},
		{"footer_degraded_query_80x24", 80, 24, func(m *Model) {
			m.setDegraded("Dolt poll query failed (retrying in 5s): syntax error near SELECT")
		}},
		{"footer_bell_100x24", 100, 24, func(m *Model) {
			for i := 0; i < 3; i++ {
				m.events.Append(events.NewSystemEvent("activity"))
			}
		}},
		{"footer_bell_60x24", 60, 24, func(m *Model) {
			for i := 0; i < 3; i++ {
				m.events.Append(events.NewSystemEvent("activity"))
			}
		}},
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
