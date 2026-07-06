package ui

// Diagnostic soak harness for bt-2ubez.
//
// A long-running interactive bt instance accumulated ~740MB working set and
// 43.5 CPU-minutes (~16% of one core sustained) over 4.2 hours on a busy day
// (many external bd writes -> many Dolt-poll-triggered refreshes). These two
// tests produce compressed, in-process evidence to split the problem into a CPU
// story and a memory story:
//
//   TestSoakIdleTickCPU     - per-idle-frame CPU cost. There is a perpetual
//                             120ms workerPollTickMsg tick (see model.go
//                             workerPollTickCmd + model_update_analysis.go
//                             handleWorkerPollTick); Bubble Tea re-renders View()
//                             after every message, so one idle frame is
//                             Update(tick)+View(). We time the three pieces
//                             separately and derive the sustained single-core
//                             percentage at the real 8.33/s cadence.
//
//   TestSoakReloadRetention - retained-heap growth per data-reload cycle. The
//                             refresh pipeline is BackgroundWorker.buildSnapshot
//                             -> SnapshotReadyMsg -> Model.handleSnapshotReady
//                             (background_worker.go / model_update_data.go). We
//                             run the real (non-Dolt) JSONL load + buildSnapshot
//                             path 300 times, feed each snapshot through the UI
//                             handler on a persistent Model, and sample post-GC
//                             HeapAlloc/HeapObjects/goroutines. If post-GC heap
//                             is flat, the leak is NOT in the UI/snapshot
//                             pipeline - itself a decisive result.
//
// Both are gated behind BT_SOAK so a normal `go test ./...` never runs them.
// The unexported message types (workerPollTickMsg, statusTickMsg) live in
// package ui, so this is an internal test, matching the other pkg/ui tests.
//
// Usage (bash): BT_SOAK=1 go test ./pkg/ui/ -run 'TestSoak' -v -timeout 20m
// Usage (pwsh): $env:BT_SOAK='1'; go test ./pkg/ui/ -run 'TestSoak' -v -timeout 20m

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	rdebug "runtime/debug"
	"runtime/pprof"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// soakDiagSink accumulates rendered-view lengths so the compiler cannot
// dead-code-eliminate the View() calls being timed.
var soakDiagSink int

// genSoakIssues builds a deterministic ~n-issue dataset with realistic field
// fill: multi-prefix IDs, few-hundred-byte descriptions, 1-3 labels, mixed
// statuses/priorities/types, blocking + parent-child edges to earlier issues
// (so the graph analysis has real structure), and comments on a subset. It is a
// scaled-up cousin of render_harness_test.go's harnessIssues().
func genSoakIssues(n int) []model.Issue {
	rng := rand.New(rand.NewSource(0xB7A5)) // fixed seed = deterministic corpus
	prefixes := []string{"bt", "sym", "dotfiles", "mkt"}
	statuses := []model.Status{
		model.StatusOpen, model.StatusInProgress, model.StatusBlocked,
		model.StatusReview, model.StatusClosed,
	}
	types := []model.IssueType{
		model.TypeBug, model.TypeFeature, model.TypeTask, model.TypeEpic, model.TypeChore,
	}
	labelPool := []string{
		"area:tui", "area:cli", "area:data", "area:search", "ux",
		"perf", "responsive", "bug", "tech-debt", "upstream",
	}
	now := time.Now()

	issues := make([]model.Issue, 0, n)
	for i := 0; i < n; i++ {
		prefix := prefixes[i%len(prefixes)]
		id := fmt.Sprintf("%s-%04d", prefix, i)
		title := fmt.Sprintf("Issue %04d: %s subsystem work item with a descriptive summary line", i, prefix)
		desc := fmt.Sprintf(
			"This work item concerns the %s subsystem. It describes a concrete change with "+
				"enough prose to be representative of a real bead body. Repro steps, rationale, "+
				"and acceptance notes live here so the loader and analysis passes see realistic "+
				"field sizes rather than empty strings. Iteration seed %d, prefix %s, index %d.",
			prefix, rng.Intn(100000), prefix, i)

		created := now.Add(-time.Duration(rng.Intn(60*24)) * time.Hour)
		updated := created.Add(time.Duration(rng.Intn(24)) * time.Hour)

		nLabels := 1 + rng.Intn(3)
		labels := make([]string, 0, nLabels)
		for j := 0; j < nLabels; j++ {
			labels = append(labels, labelPool[rng.Intn(len(labelPool))])
		}

		iss := model.Issue{
			ID:          id,
			Title:       title,
			Description: desc,
			Status:      statuses[rng.Intn(len(statuses))],
			Priority:    rng.Intn(5),
			IssueType:   types[rng.Intn(len(types))],
			Author:      "sms",
			Labels:      labels,
			CreatedAt:   created,
			UpdatedAt:   updated,
		}

		if i%3 == 0 {
			iss.Design = "Design: route the change through the existing pipeline; keep the diff surgical and covered by a snapshot test."
			iss.AcceptanceCriteria = "- behavior verified\n- no regressions\n- snapshot updated"
		}
		if i%5 == 0 {
			iss.Notes = fmt.Sprintf("Notes: discovered during dogfood pass %d; see related items.", i)
		}
		if i%7 == 0 {
			iss.Assignee = "sms"
		}

		// ~40% carry an edge to an existing earlier issue.
		if i > 0 && rng.Intn(10) < 4 {
			t := rng.Intn(i)
			target := fmt.Sprintf("%s-%04d", prefixes[t%len(prefixes)], t)
			dtype := model.DepBlocks
			if rng.Intn(2) == 0 {
				dtype = model.DepParentChild
			}
			iss.Dependencies = []*model.Dependency{
				{IssueID: id, DependsOnID: target, Type: dtype, CreatedAt: created},
			}
		}

		if i%4 == 0 {
			iss.Comments = []*model.Comment{
				{ID: fmt.Sprintf("c-%d", i), IssueID: id, Author: "claude", Text: "confirmed repro on the current build", CreatedAt: updated},
			}
		}

		issues = append(issues, iss)
	}
	return issues
}

// writeSoakJSONL serializes issues to JSONL in the format pkg/loader parses
// (one bd-export-shaped object per line).
func writeSoakJSONL(t *testing.T, path string, issues []model.Issue) {
	t.Helper()
	var buf []byte
	for i := range issues {
		b, err := json.Marshal(issues[i])
		if err != nil {
			t.Fatalf("marshal issue %d: %v", i, err)
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write jsonl %s: %v", path, err)
	}
}

func writeSoakHeapProfile(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create heap profile %s: %v", path, err)
	}
	defer f.Close()
	runtime.GC() // up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatalf("write heap profile %s: %v", path, err)
	}
}

// TestSoakIdleTickCPU times the per-idle-frame cost of the perpetual worker
// poll tick against a realistic 1300-issue model at the user's scrunched
// window, and derives the sustained single-core percentage at the real cadence.
func TestSoakIdleTickCPU(t *testing.T) {
	if os.Getenv("BT_SOAK") == "" {
		t.Skip("set BT_SOAK=1 to run the diagnostic soak")
	}

	issues := genSoakIssues(1300)
	m := NewModel(issues, nil, "", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = nm.(Model)

	// Warmup: prime allocations/caches and let NewModel's async Phase 2 settle.
	for i := 0; i < 300; i++ {
		nm, _ := m.Update(workerPollTickMsg{})
		m = nm.(Model)
		soakDiagSink += len(m.View().Content)
	}

	const iters = 3000

	// (1) Update(workerPollTickMsg{}) alone - the tick handler + router dispatch.
	startUpd := time.Now()
	for i := 0; i < iters; i++ {
		nm, _ := m.Update(workerPollTickMsg{})
		m = nm.(Model)
	}
	updDur := time.Since(startUpd)

	// (2) View() alone - the full re-render Bubble Tea forces after each message.
	startView := time.Now()
	for i := 0; i < iters; i++ {
		soakDiagSink += len(m.View().Content)
	}
	viewDur := time.Since(startView)

	// (3) Update + View together - one simulated idle frame.
	startBoth := time.Now()
	for i := 0; i < iters; i++ {
		nm, _ := m.Update(workerPollTickMsg{})
		m = nm.(Model)
		soakDiagSink += len(m.View().Content)
	}
	bothDur := time.Since(startBoth)

	updNs := float64(updDur.Nanoseconds()) / iters
	viewNs := float64(viewDur.Nanoseconds()) / iters
	bothNs := float64(bothDur.Nanoseconds()) / iters

	const workerHz = 1000.0 / 120.0 // perpetual workerPollTickMsg cadence
	const statusHz = 1.0            // statusTickMsg cadence (statusTickInterval = 1s)
	frameSec := bothNs / 1e9
	workerCorePct := frameSec * workerHz * 100.0
	statusCorePct := frameSec * statusHz * 100.0

	t.Logf("dataset: %d issues, window 120x30, iters/timing=%d", len(issues), iters)
	t.Logf("Update(workerPollTickMsg) alone : %10.0f ns/iter", updNs)
	t.Logf("View() alone                    : %10.0f ns/iter", viewNs)
	t.Logf("Update+View (one idle frame)    : %10.0f ns/iter", bothNs)
	t.Logf("derived sustained core @ %.2f Hz (workerPollTick): %.4f %% of one core", workerHz, workerCorePct)
	t.Logf("derived sustained core @ %.2f Hz (statusTick)    : %.4f %% of one core", statusHz, statusCorePct)
	t.Logf("CAVEAT: excludes Bubble Tea's frame diff + ANSI terminal write, so per-frame cost is a lower bound.")
	t.Logf("soakDiagSink=%d (anti-DCE)", soakDiagSink)
}

// TestSoakReloadRetention runs the real load + buildSnapshot + handleSnapshotReady
// pipeline for 300 reload cycles and samples heap state after a double-GC every
// 25 cycles, writing early/late heap profiles for a pprof diff.
//
// It records two distinct things: (a) RETAINED heap (post-GC HeapAlloc/objects)
// to detect a true leak, and (b) the WORKING-SET mechanism - HeapSys (monotonic
// OS heap reservation), HeapIdle, and HeapReleased, plus the peak transient
// HeapAlloc caught right after each buildSnapshot (before the GC pair). The
// HeapIdle-minus-HeapReleased gap is runtime-held-but-unreleased pages, and the
// peak-vs-steady-state gap is the per-cycle transient spike amplitude: together
// these quantify the "high-water-mark from transient per-refresh allocations"
// hypothesis for the ~748MB parked working set (bt-2ubez).
func TestSoakReloadRetention(t *testing.T) {
	if os.Getenv("BT_SOAK") == "" {
		t.Skip("set BT_SOAK=1 to run the diagnostic soak")
	}

	tmp := t.TempDir()
	fixturePath := filepath.Join(tmp, "issues.jsonl")
	issues := genSoakIssues(1300)
	baseTitles := make([]string, len(issues))
	for i := range issues {
		baseTitles[i] = issues[i].Title
	}
	writeSoakJSONL(t, fixturePath, issues)

	// Heap profiles land in repo-root/_tmp/soak (gitignored), matching the
	// render harness's _tmp convention.
	outDir := filepath.Join("..", "..", "_tmp", "soak")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}

	// Minimally-constructed worker exercising the real non-Dolt JSONL path
	// (issue pool, hash dedup, Phase 1 analysis, snapshot diff, list build).
	// ctx + msgCh are supplied so the Phase 2 completion goroutine that
	// buildSnapshot spawns has a valid channel/context (send() never blocks or
	// panics). No watcher is created and Start() is never called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &BackgroundWorker{
		beadsPath: fixturePath,
		ctx:       ctx,
		cancel:    cancel,
		msgCh:     make(chan tea.Msg, 8),
	}

	// Persistent model that receives every snapshot, like a live session. It
	// owns no worker/watcher itself (beadsPath ""), so handleSnapshotReady runs
	// purely off the snapshots we feed it.
	m := NewModel(issues, nil, "", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = nm.(Model)
	m.data.snapshotInitPending = true // first snapshot is the bootstrap

	type sample struct {
		cycle        int
		heapAlloc    uint64 // live heap after the double-GC (retained)
		heapObjects  uint64
		heapSys      uint64 // heap bytes obtained from the OS (monotonic high-water)
		heapIdle     uint64 // bytes in idle (unused) spans
		heapReleased uint64 // idle bytes released back to the OS
		totalAlloc   uint64 // cumulative bytes ever allocated (for churn)
		goroutines   int
	}
	var series []sample

	// Peak transient live heap seen right after buildSnapshot (pre-GC): the
	// per-cycle spike amplitude that ratchets HeapSys / OS working set upward,
	// which is the leak-shaped-but-not-leaking WS story (bt-2ubez).
	var peakHeapAlloc uint64
	var peakHeapAllocCycle int

	const cycles = 300
	for cycle := 1; cycle <= cycles; cycle++ {
		// (1) Mutate the fixture so content-hash dedup sees a real change: bump
		// one issue's updated_at and append a rev counter to its title.
		idx := cycle % len(issues)
		issues[idx].Title = fmt.Sprintf("%s [rev %d]", baseTitles[idx], cycle)
		issues[idx].UpdatedAt = issues[idx].UpdatedAt.Add(time.Duration(cycle) * time.Minute)
		writeSoakJSONL(t, fixturePath, issues)

		// (2) Real load + snapshot build (exercises the issue pool + Phase 1).
		snap := w.buildSnapshot()
		if snap == nil {
			t.Fatalf("cycle %d: buildSnapshot returned nil (dedup or load error)", cycle)
		}

		// Peak transient: capture live heap right after the build, before the UI
		// swap and before the periodic GC pair. ReadMemStats does not collect, so
		// this reflects the transient high-water (parse buffers + the new snapshot
		// while the previous one is still live) that ratchets HeapSys / working
		// set upward even though nothing is permanently retained.
		var pk runtime.MemStats
		runtime.ReadMemStats(&pk)
		if pk.HeapAlloc > peakHeapAlloc {
			peakHeapAlloc = pk.HeapAlloc
			peakHeapAllocCycle = cycle
		}

		// (3) Feed through the UI reload handler on the persistent model. This
		// returns the previous snapshot's pooled issues to the pool.
		nm, _ := m.handleSnapshotReady(SnapshotReadyMsg{Snapshot: snap, SentAt: time.Now()})
		m = nm

		// Mirror process(): the worker holds the latest snapshot as the diff
		// base for the next cycle. Set AFTER handleSnapshotReady so the previous
		// snapshot (whose pool refs the handler just returned) is never the diff
		// base - the base is always the still-live current snapshot.
		w.snapshot = snap

		// (4) Interleaved idle rendering (3 frames per reload).
		for r := 0; r < 3; r++ {
			nm, _ := m.Update(workerPollTickMsg{})
			m = nm.(Model)
			soakDiagSink += len(m.View().Content)
		}

		// (5) Sample retained heap every 25 cycles, post double-GC.
		if cycle%25 == 0 {
			runtime.GC()
			runtime.GC()
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			series = append(series, sample{
				cycle:        cycle,
				heapAlloc:    ms.HeapAlloc,
				heapObjects:  ms.HeapObjects,
				heapSys:      ms.HeapSys,
				heapIdle:     ms.HeapIdle,
				heapReleased: ms.HeapReleased,
				totalAlloc:   ms.TotalAlloc,
				goroutines:   runtime.NumGoroutine(),
			})

			if cycle == 25 {
				writeSoakHeapProfile(t, filepath.Join(outDir, "heap_early.pprof"))
			}
			if cycle == cycles {
				writeSoakHeapProfile(t, filepath.Join(outDir, "heap_late.pprof"))
			}
		}
	}

	kb := func(v uint64) float64 { return float64(v) / 1024.0 }

	t.Logf("=== TestSoakReloadRetention: %d issues, %d reload cycles, window 120x30 ===", len(m.data.issues), cycles)
	t.Logf("all heap columns are POST double-GC (retained); HeapSys is the monotonic OS reservation")
	t.Logf("%-6s %11s %10s %10s %12s %13s %12s %5s",
		"cycle", "HeapAlloc", "HeapSys", "HeapIdle", "HeapReleased", "Idle-Rel", "HeapObjects", "gor")
	t.Logf("%-6s %11s %10s %10s %12s %13s %12s %5s",
		"", "(KB)", "(KB)", "(KB)", "(KB)", "(KB)", "", "")
	for _, s := range series {
		t.Logf("%-6d %11.1f %10.1f %10.1f %12.1f %13.1f %12d %5d",
			s.cycle, kb(s.heapAlloc), kb(s.heapSys), kb(s.heapIdle), kb(s.heapReleased),
			kb(s.heapIdle-s.heapReleased), s.heapObjects, s.goroutines)
	}

	if len(series) >= 2 {
		first := series[0]
		last := series[len(series)-1]
		dCycles := float64(last.cycle - first.cycle)
		slopeKB := (kb(last.heapAlloc) - kb(first.heapAlloc)) / dCycles
		objSlope := (float64(last.heapObjects) - float64(first.heapObjects)) / dCycles
		churnMB := (float64(last.totalAlloc) - float64(first.totalAlloc)) / 1024.0 / 1024.0 / dCycles
		t.Logf("retained-heap slope    : %+.3f KB/cycle (%.1f -> %.1f KB post-GC over cycles %d..%d)  [FLAT = no leak]",
			slopeKB, kb(first.heapAlloc), kb(last.heapAlloc), first.cycle, last.cycle)
		t.Logf("retained-obj slope     : %+.1f objects/cycle", objSlope)
		t.Logf("goroutines             : first sample %d, last sample %d", first.goroutines, last.goroutines)
		t.Logf("allocation churn       : %.2f MB allocated per reload cycle (transient, GC-reclaimed)", churnMB)
		t.Logf("HeapSys high-water     : %.1f KB (final; monotonic OS heap reservation)", kb(last.heapSys))
		t.Logf("idle-not-released      : %.1f KB -> %.1f KB (HeapIdle - HeapReleased; runtime holds, not returned to OS)",
			kb(first.heapIdle-first.heapReleased), kb(last.heapIdle-last.heapReleased))
		t.Logf("OS-backed retained     : %.1f KB (HeapSys - HeapReleased at cycle %d; approx heap contribution to RSS/WS)",
			kb(last.heapSys-last.heapReleased), last.cycle)
	}
	t.Logf("PEAK transient HeapAlloc : %.1f KB at cycle %d (max live heap right after buildSnapshot, pre-GC)",
		kb(peakHeapAlloc), peakHeapAllocCycle)
	t.Logf("heap profiles: %s (cycle 25) and %s (cycle %d)",
		filepath.Join(outDir, "heap_early.pprof"), filepath.Join(outDir, "heap_late.pprof"), cycles)
	t.Logf("diff cmd: go tool pprof -top -nodecount=20 -diff_base=%s %s",
		filepath.Join(outDir, "heap_early.pprof"), filepath.Join(outDir, "heap_late.pprof"))
	t.Logf("soakDiagSink=%d (anti-DCE)", soakDiagSink)
}

// gcABResult holds one sub-run's four working-set numbers.
type gcABResult struct {
	label           string
	gcPercent       int
	heapSysBefore   uint64 // HeapSys at sub-run start (order-effect marker)
	maxHeapAlloc    uint64 // peak live heap seen (GOGC-sensitive, order-independent)
	maxHeapSys      uint64 // peak OS heap reservation seen
	finalHeapSys    uint64
	finalIdleNotRel uint64 // final HeapIdle - HeapReleased
}

// runGCSoakSub runs one fresh 150-cycle reload soak at a fixed GOGC and reports
// the four working-set numbers, reading MemStats WITHOUT forcing GC so the
// collector runs naturally and GOGC actually governs the live-heap peak.
func runGCSoakSub(t *testing.T, label string, gcPercent int) gcABResult {
	t.Helper()

	prev := rdebug.SetGCPercent(gcPercent)
	defer rdebug.SetGCPercent(prev)

	tmp := t.TempDir()
	fixturePath := filepath.Join(tmp, "issues.jsonl")
	issues := genSoakIssues(1300)
	baseTitles := make([]string, len(issues))
	for i := range issues {
		baseTitles[i] = issues[i].Title
	}
	writeSoakJSONL(t, fixturePath, issues)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &BackgroundWorker{
		beadsPath: fixturePath,
		ctx:       ctx,
		cancel:    cancel,
		msgCh:     make(chan tea.Msg, 8),
	}

	m := NewModel(issues, nil, "", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = nm.(Model)
	m.data.snapshotInitPending = true

	// HeapSys at sub-run start, without forcing GC (order-effect marker).
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)
	res := gcABResult{label: label, gcPercent: gcPercent, heapSysBefore: msBefore.HeapSys}

	const cycles = 150
	for cycle := 1; cycle <= cycles; cycle++ {
		idx := cycle % len(issues)
		issues[idx].Title = fmt.Sprintf("%s [rev %d]", baseTitles[idx], cycle)
		issues[idx].UpdatedAt = issues[idx].UpdatedAt.Add(time.Duration(cycle) * time.Minute)
		writeSoakJSONL(t, fixturePath, issues)

		snap := w.buildSnapshot()
		if snap == nil {
			t.Fatalf("%s cycle %d: buildSnapshot returned nil", label, cycle)
		}

		// Sample right after the build (heaviest alloc point) without collecting.
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		if ms.HeapAlloc > res.maxHeapAlloc {
			res.maxHeapAlloc = ms.HeapAlloc
		}
		if ms.HeapSys > res.maxHeapSys {
			res.maxHeapSys = ms.HeapSys
		}

		nm, _ := m.handleSnapshotReady(SnapshotReadyMsg{Snapshot: snap, SentAt: time.Now()})
		m = nm
		w.snapshot = snap

		for r := 0; r < 3; r++ {
			nm, _ := m.Update(workerPollTickMsg{})
			m = nm.(Model)
			soakDiagSink += len(m.View().Content)
		}
	}

	var msFinal runtime.MemStats
	runtime.ReadMemStats(&msFinal)
	res.finalHeapSys = msFinal.HeapSys
	res.finalIdleNotRel = msFinal.HeapIdle - msFinal.HeapReleased

	runtime.KeepAlive(w)
	runtime.KeepAlive(m)
	return res
}

// TestSoakGCPercentAB A/B-tests whether process-wide GOGC=200 -- which
// production applies at worker start via debug.SetGCPercent(200)
// (background_worker.go:616) -- amplifies the transient-churn working-set
// high-water versus GOGC=100. Each sub-run is a fresh 150-cycle reload soak
// (like TestSoakReloadRetention) with NO forced runtime.GC(), so the collector
// runs naturally and GOGC governs the live-heap peak.
//
// HeapSys is monotonic within a process, so running both sub-runs in one process
// makes B inherit A's HeapSys high-water (reported with an order caveat); the
// GOGC-sensitive, order-independent signal is max HeapAlloc, reset per sub-run.
// For a clean HeapSys comparison, run a SINGLE sub-run per fresh process:
//
//	BT_SOAK=1 BT_SOAK_GOGC=200 go test ./pkg/ui/ -run TestSoakGCPercentAB -v
//	BT_SOAK=1 BT_SOAK_GOGC=100 go test ./pkg/ui/ -run TestSoakGCPercentAB -v
//
// With BT_SOAK_GOGC unset, both run sequentially (A at 200, then B at 100) for a
// one-command side-by-side.
func TestSoakGCPercentAB(t *testing.T) {
	if os.Getenv("BT_SOAK") == "" {
		t.Skip("set BT_SOAK=1 to run the diagnostic soak")
	}

	kb := func(v uint64) float64 { return float64(v) / 1024.0 }
	logResult := func(r gcABResult) {
		t.Logf("[%s] GOGC=%d", r.label, r.gcPercent)
		t.Logf("    HeapSys before sub-run   : %10.1f KB", kb(r.heapSysBefore))
		t.Logf("    max HeapAlloc (peak live): %10.1f KB", kb(r.maxHeapAlloc))
		t.Logf("    max HeapSys              : %10.1f KB", kb(r.maxHeapSys))
		t.Logf("    final HeapSys            : %10.1f KB", kb(r.finalHeapSys))
		t.Logf("    final HeapIdle-Released  : %10.1f KB", kb(r.finalIdleNotRel))
	}

	// Single-sub-run mode for a clean (fresh-process) HeapSys comparison.
	switch os.Getenv("BT_SOAK_GOGC") {
	case "200":
		t.Logf("=== TestSoakGCPercentAB single sub-run (fresh process), 1300 issues, 150 cycles, natural GC ===")
		logResult(runGCSoakSub(t, "A", 200))
		t.Logf("soakDiagSink=%d (anti-DCE)", soakDiagSink)
		return
	case "100":
		t.Logf("=== TestSoakGCPercentAB single sub-run (fresh process), 1300 issues, 150 cycles, natural GC ===")
		logResult(runGCSoakSub(t, "B", 100))
		t.Logf("soakDiagSink=%d (anti-DCE)", soakDiagSink)
		return
	}

	// In-process both mode: A (GOGC=200) then B (GOGC=100). max HeapAlloc is
	// reset per sub-run so its A/B comparison is clean; HeapSys carries A's
	// high-water into B (see the fresh-process mode for a clean HeapSys read).
	a := runGCSoakSub(t, "A", 200)
	rdebug.FreeOSMemory() // reset live heap between sub-runs as far as possible
	b := runGCSoakSub(t, "B", 100)

	t.Logf("=== TestSoakGCPercentAB: 1300 issues, 150 cycles/sub-run, natural GC (no forced runtime.GC) ===")
	logResult(a)
	logResult(b)

	ratioAlloc := float64(a.maxHeapAlloc) / float64(b.maxHeapAlloc)
	t.Logf("--- A/B verdict (in-process; trust max HeapAlloc; HeapSys is order-biased toward A) ---")
	t.Logf("max HeapAlloc : A(GOGC=200)=%.1f KB  vs  B(GOGC=100)=%.1f KB   ratio A/B = %.2fx",
		kb(a.maxHeapAlloc), kb(b.maxHeapAlloc), ratioAlloc)
	t.Logf("max HeapSys   : A(GOGC=200)=%.1f KB  vs  B(GOGC=100)=%.1f KB   (B inherits A's monotonic high-water; use fresh-process mode)",
		kb(a.maxHeapSys), kb(b.maxHeapSys))
	t.Logf("HYPOTHESIS: GOGC=200 amplifies peak heap if ratio A/B is well above 1 (toward 1.5-2x); near 1.0x kills the GOGC lever.")
	t.Logf("soakDiagSink=%d (anti-DCE)", soakDiagSink)
}
