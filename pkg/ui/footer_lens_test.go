package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// richLensFixture is a FooterData exercising every lens slot: cross-project
// scope, an explicit status, a label filter, a placeholder search slot, and an
// explicit sort — plus the center counts and a bell, so the whole footer (not
// just the lens) participates in the width sweep.
func richLensFixture(width int) FooterData {
	return FooterData{
		Width:           width,
		ScopeLabel:      "bt",
		StatusFilter:    "open",
		LabelFilterText: "area:tui",
		OrderLabel:      "updated",
		CountReady:      14,
		CountInFlight:   7,
		CountBlocked:    9,
		TotalItems:      169,
		BellCount:       4,
	}
}

// TestLensFullSentenceASCII locks the doc's verbatim ascii mockup form at a wide
// width: scope · st:<status> · lb:<label> · /- · by:<order>.
func TestLensFullSentenceASCII(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	out := ansi.Strip(richLensFixture(160).Render())
	for _, want := range []string{"bt", "st:open", "lb:area:tui", "/-", "by:updated"} {
		if !strings.Contains(out, want) {
			t.Errorf("ascii lens missing %q in %q", want, out)
		}
	}
	// No echo anywhere: the footer must never announce what the lens already shows.
	if strings.Contains(out, "Filter:") {
		t.Errorf("footer must not carry a Filter: echo: %q", out)
	}
	// The status appears once (in the lens), not duplicated by a right-side badge.
	if n := strings.Count(out, "open"); n != 1 {
		t.Errorf("status word should appear exactly once (in the lens), got %d: %q", n, out)
	}
}

// TestLensFullSentenceNerdFont proves the Nerd Font tier swaps the text prefixes
// for the table icons: the chip VALUES survive but the "st:" / "lb:" / "by:"
// text prefixes do not, and the scope carries the folder/globe glyph. The
// status chip renders bare - its funnel icon was removed (bt-n9gn5).
func TestLensFullSentenceNerdFont(t *testing.T) {
	setGlyphs(t, nerdfontGlyphs)
	out := ansi.Strip(richLensFixture(160).Render())
	for _, want := range []string{"bt", "open", "area:tui", "updated"} {
		if !strings.Contains(out, want) {
			t.Errorf("nerdfont lens missing %q in %q", want, out)
		}
	}
	for _, unwanted := range []string{"st:", "lb:", "by:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("nerdfont tier must use icons, not the %q text prefix: %q", unwanted, out)
		}
	}
}

// TestLensCrossProjectScope proves the all-projects scope shows its project
// count. (It renders bare - no globe glyph - per bt-41gr8.)
func TestLensCrossProjectScope(t *testing.T) {
	setGlyphs(t, nerdfontGlyphs)
	fd := richLensFixture(160)
	fd.ScopeLabel = "ALL(19)"
	fd.ScopeCrossProject = true
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "ALL(19)") {
		t.Errorf("cross-project scope should show the count: %q", out)
	}
}

// TestLensSingleRowAllWidthsBothTiers is the core layout guarantee: at every
// design width in both tiers the footer is one row within the terminal width,
// the scope always survives, and no filter echo leaks in.
func TestLensSingleRowAllWidthsBothTiers(t *testing.T) {
	widths := []int{50, 70, 100, 130, 160}
	for _, tier := range []struct {
		name string
		g    GlyphSet
	}{{"nerdfont", nerdfontGlyphs}, {"ascii", asciiGlyphs}} {
		t.Run(tier.name, func(t *testing.T) {
			setGlyphs(t, tier.g)
			for _, w := range widths {
				out := richLensFixture(w).Render()
				if strings.Contains(out, "\n") {
					t.Errorf("width %d: footer wrapped to >1 row:\n%q", w, out)
				}
				if got := ansi.StringWidth(out); got > w {
					t.Errorf("width %d: footer display width %d exceeds terminal width", w, got)
				}
				plain := ansi.Strip(out)
				if !strings.Contains(plain, "bt") {
					t.Errorf("width %d: scope must survive every width; got %q", w, plain)
				}
				if strings.Contains(plain, "Filter:") {
					t.Errorf("width %d: no Filter: echo allowed; got %q", w, plain)
				}
			}
		})
	}
}

// TestLensPlaceholdersDropBeforeStatus proves the doc's drop order: the lb:- /
// /- space-holders are present at a wide width but are the first lens content to
// go under pressure, while the status word and scope survive.
func TestLensPlaceholdersDropBeforeStatus(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	// No label filter, no query -> both slots are placeholders at the full level.
	fd := FooterData{ScopeLabel: "bt", StatusFilter: "open", TotalItems: 169}

	fd.Width = 160
	wide := ansi.Strip(fd.Render())
	if !strings.Contains(wide, "lb:-") || !strings.Contains(wide, "/-") {
		t.Fatalf("placeholders should hold space at a wide width: %q", wide)
	}

	// A width that fits scope + status but not the placeholders: placeholders
	// gone, status + scope intact.
	fd.Width = 24
	narrow := ansi.Strip(fd.Render())
	if strings.Contains(narrow, "lb:-") || strings.Contains(narrow, "/-") {
		t.Errorf("placeholders must drop first under pressure: %q", narrow)
	}
	if !strings.Contains(narrow, "bt") {
		t.Errorf("scope must survive: %q", narrow)
	}
}

// TestLensScopeSurvivesToScopeOnly proves scope is the last lens content standing
// at an extreme width.
func TestLensScopeSurvivesToScopeOnly(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	fd := richLensFixture(18)
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "bt") {
		t.Errorf("scope must survive at extreme narrow width: %q", out)
	}
	if strings.Contains(out, "\n") {
		t.Errorf("must stay one row at extreme narrow width: %q", out)
	}
}

// TestLensSearchAndBQLQuery proves both a fuzzy query and a BQL query surface in
// the lens "/" slot (no filter state the lens cannot show).
func TestLensSearchAndBQLQuery(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	fd := FooterData{Width: 160, ScopeLabel: "bt", StatusFilter: "open", SearchQuery: "dep graph", TotalItems: 12}
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "/dep graph") {
		t.Errorf("active search query should show in the lens: %q", out)
	}
}

// TestLensRecipeChip proves an active recipe joins the filter bucket.
func TestLensRecipeChip(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	fd := FooterData{Width: 160, ScopeLabel: "bt", RecipeName: "triage", TotalItems: 8}
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "recipe:triage") {
		t.Errorf("active recipe should show in the lens: %q", out)
	}
}

// TestLensAnomalyBadgeAbsentAtZero locks the dark-cockpit contract for the
// right-zone anomaly badge (bt-9gjt0): no attention-worthy drift -> no badge.
func TestLensAnomalyBadgeAbsentAtZero(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	fd := richLensFixture(160)
	fd.CriticalCount = 0
	fd.WarningCount = 0
	out := ansi.Strip(fd.Render())
	// The bell (*/ ! sigil family) still renders; the anomaly sigil "!N" must not.
	if strings.Contains(out, activeGlyphs.Warning+"1") || strings.Contains(out, activeGlyphs.Bolt+"1") {
		t.Errorf("anomaly badge must be absent at zero: %q", out)
	}
	// With one attention-worthy drift it lights up.
	fd.CriticalCount = 1
	lit := ansi.Strip(fd.Render())
	if !strings.Contains(lit, activeGlyphs.Warning+"1") {
		t.Errorf("one critical drift should light the anomaly badge: %q", lit)
	}
}

// TestLensStaticHintsDegrade proves the Zone-3 pair degrades from labels to bare
// keys under width pressure but never disappears.
func TestLensStaticHintsDegrade(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	wide := ansi.Strip(richLensFixture(160).Render())
	if !strings.Contains(wide, "? help") || !strings.Contains(wide, "; shortcuts") {
		t.Errorf("wide footer should show the full ? help · ; shortcuts pair: %q", wide)
	}
	narrow := ansi.Strip(richLensFixture(46).Render())
	if !strings.Contains(narrow, "?") || !strings.Contains(narrow, ";") {
		t.Errorf("narrow footer must still show the bare ? ; pair: %q", narrow)
	}
	if strings.Contains(narrow, "help") {
		t.Errorf("narrow footer should drop the hint labels: %q", narrow)
	}
}

// TestStatusCycleKeyEndToEnd drives the '#' key through the real update loop and
// proves it cycles the full status set, that the lens reflects each stop, and
// that it emits no "Filter:" toast echo (bt-gpvwe absorbed by bt-2vshd).
func TestStatusCycleKeyEndToEnd(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "bt", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = nm.(Model)

	if m.filter.currentFilter != "all" {
		t.Fatalf("precondition: default filter should be all, got %q", m.filter.currentFilter)
	}

	// One '#' per expected stop after "all".
	want := []string{"open", "in_progress", "blocked", "closed", "deferred", "all"}
	for _, expect := range want {
		nm, _ = m.Update(tea.KeyPressMsg{Code: '#', Text: "#"})
		m = nm.(Model)
		if m.filter.currentFilter != expect {
			t.Fatalf("after '#' expected filter %q, got %q", expect, m.filter.currentFilter)
		}
		// The lens reflects the active status; no Filter: toast echo.
		if m.statusMsg != "" {
			t.Errorf("status cycle must not set a toast echo; got %q", m.statusMsg)
		}
		fd := m.footerData()
		if fd.StatusFilter != expect {
			t.Errorf("lens StatusFilter = %q, want %q", fd.StatusFilter, expect)
		}
	}
}

// TestPopulateLensScope covers the scope derivation: single-project name vs
// cross-project ALL(N).
func TestPopulateLensScope(t *testing.T) {
	single := NewModel(harnessIssues(), nil, "", nil, nil)
	single.SetProjectName("bt")
	if got := single.footerData().ScopeLabel; got != "bt" {
		t.Errorf("single-project scope = %q, want %q", got, "bt")
	}
	if single.footerData().ScopeCrossProject {
		t.Errorf("single-project scope must not be cross-project")
	}
}

// --- bt-n9gn5: the NF status chip drops the funnel icon ---

// TestLensStatusChipNoFunnelNerdFont locks the funnel removal (bt-n9gn5): the
// static fa-filter mark read as a stuck down-arrow in live dogfood
// (2026-07-17), so the NF status chip renders its bare value with no icon.
func TestLensStatusChipNoFunnelNerdFont(t *testing.T) {
	setGlyphs(t, nerdfontGlyphs)
	out := ansi.Strip(richLensFixture(160).Render())
	if strings.Contains(out, "") {
		t.Errorf("NF status chip must not carry the funnel icon: %q", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("NF status chip must keep its bare value: %q", out)
	}
}

// --- bt-uxyel: sort direction is a glyph, not a "-old" suffix ---

// TestLensSortLabelCreatedPair: both created sort modes share the bare
// "created" token; direction is carried by the order chip's direction glyph,
// never a "-old" suffix.
func TestLensSortLabelCreatedPair(t *testing.T) {
	if got := lensSortLabel(SortCreatedDesc); got != "created" {
		t.Errorf("SortCreatedDesc label = %q, want %q", got, "created")
	}
	if got := lensSortLabel(SortCreatedAsc); got != "created" {
		t.Errorf("SortCreatedAsc label = %q, want %q", got, "created")
	}
}

// TestLensOrderChipDirectionNerdFont: the NF order chip's icon slot carries
// the direction for the created pair (down for the newest-first default, up
// for oldest-first); non-directional sorts keep the static sort mark.
func TestLensOrderChipDirectionNerdFont(t *testing.T) {
	setGlyphs(t, nerdfontGlyphs)
	fd := richLensFixture(160)
	fd.OrderLabel = "created"

	fd.OrderDir = "asc"
	if out := ansi.Strip(fd.Render()); !strings.Contains(out, activeGlyphs.SortAsc+"created") {
		t.Errorf("asc order chip should read %q: %q", activeGlyphs.SortAsc+"created", out)
	}
	fd.OrderDir = "desc"
	if out := ansi.Strip(fd.Render()); !strings.Contains(out, activeGlyphs.SortDesc+"created") {
		t.Errorf("desc order chip should read %q: %q", activeGlyphs.SortDesc+"created", out)
	}
	fd.OrderLabel, fd.OrderDir = "updated", ""
	if out := ansi.Strip(fd.Render()); !strings.Contains(out, activeGlyphs.Sort+"updated") {
		t.Errorf("non-directional order chip should keep the sort mark: %q", out)
	}
}

// TestLensOrderChipDirectionASCII: the ascii tier marks only the non-default
// ascending direction (by:created^); the newest-first default stays unmarked,
// and the -old suffix is gone everywhere.
func TestLensOrderChipDirectionASCII(t *testing.T) {
	setGlyphs(t, asciiGlyphs)
	fd := richLensFixture(160)
	fd.OrderLabel = "created"

	fd.OrderDir = "asc"
	if out := ansi.Strip(fd.Render()); !strings.Contains(out, "by:created^") {
		t.Errorf("ascii asc order chip should read by:created^: %q", out)
	}
	fd.OrderDir = "desc"
	out := ansi.Strip(fd.Render())
	if !strings.Contains(out, "by:created") || strings.Contains(out, "by:created^") {
		t.Errorf("ascii desc order chip should be the unmarked by:created: %q", out)
	}
	if strings.Contains(out, "created-old") {
		t.Errorf("the -old suffix must be gone: %q", out)
	}
}

// --- bt-41gr8: the scope segment renders bare and unbolded ---

// lensBoldRe matches an SGR sequence opening with the bold parameter.
var lensBoldRe = regexp.MustCompile("\x1b\\[1[;m]")

// TestLensScopeBareNerdFont locks the scope treatment from live dogfood
// 2026-07-17 (bt-41gr8): the NF folder/globe icons read as oversized dots at
// terminal size and the bold styling read poorly, so the scope renders as the
// bare label in both tiers, distinguished from the dim chips by normal-
// brightness color only.
func TestLensScopeBareNerdFont(t *testing.T) {
	setGlyphs(t, nerdfontGlyphs)
	fd := FooterData{ScopeLabel: "bt"}
	raw := renderLens(fd, lensScopeOnly)
	if got := ansi.Strip(raw); got != "bt" {
		t.Errorf("NF scope must render bare (no folder icon): %q", got)
	}
	if lensBoldRe.MatchString(raw) {
		t.Errorf("scope must not render bold: %q", raw)
	}
	fd = FooterData{ScopeLabel: "ALL(19)", ScopeCrossProject: true}
	if got := ansi.Strip(renderLens(fd, lensScopeOnly)); got != "ALL(19)" {
		t.Errorf("cross-project scope must render bare (no globe icon): %q", got)
	}
}

// --- bt-x5lvp: the sidebar hint says what it opens ---

// TestStaticHintsShortcutsLabel: the wide-width hint pair reads
// "? help · ; shortcuts" - "keys" named the key, not the surface
// (maintainer pick, 2026-07-17). Compact tier stays "? ;".
func TestStaticHintsShortcutsLabel(t *testing.T) {
	wide := ansi.Strip(renderStaticHints(false))
	if !strings.Contains(wide, "; shortcuts") {
		t.Errorf("wide hints should read \"; shortcuts\": %q", wide)
	}
	if strings.Contains(wide, "; keys") {
		t.Errorf("the old \"; keys\" label must be gone: %q", wide)
	}
	if compact := ansi.Strip(renderStaticHints(true)); strings.Contains(compact, "shortcuts") {
		t.Errorf("compact hints must stay the bare pair: %q", compact)
	}
}

// TestPopulateLensOrderDir: the model threads the created pair's direction
// into FooterData alongside the shared bare label.
func TestPopulateLensOrderDir(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	cases := []struct {
		mode  SortMode
		label string
		dir   string
	}{
		{SortCreatedAsc, "created", "asc"},
		{SortCreatedDesc, "created", "desc"},
		{SortUpdated, "updated", ""},
	}
	for _, c := range cases {
		m.filter.sortMode = c.mode
		fd := m.footerData()
		if fd.OrderLabel != c.label || fd.OrderDir != c.dir {
			t.Errorf("sortMode %v: got (%q, %q), want (%q, %q)",
				c.mode, fd.OrderLabel, fd.OrderDir, c.label, c.dir)
		}
	}
}
