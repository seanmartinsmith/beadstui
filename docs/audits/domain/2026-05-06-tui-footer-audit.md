# TUI Footer Audit - 2026-05-06

**Bead**: bt-ugbp  
**Status**: Recon only - no source edits made  
**Scope**: All top-level views in `pkg/ui/`, footers measured at widths 60, 80, 100, 120, 140, 160

---

## Summary

The TUI footer is a single horizontal bar assembled in `FooterData.Render()` (`pkg/ui/model_footer.go:559`).
It is rendered once per frame via `m.renderFooter()` and joined to the view body with `lipgloss.JoinVertical`
in `View()` (`pkg/ui/model_view.go:178`). The footer has two compression mechanisms:

1. **Key hint truncation** (lines 779-786): drops middle hints until the keys section fits within
   `width - countBadge - 2`.
2. **Badge tier dropping** (lines 829-838): progressively drops optional badges by tier (3=first, 1=last)
   until total width fits.

Despite these mechanisms, several views overflow at all practical widths (60-120 cols) because the
*always-present* content (filterBadge + labelHint + statsSection + countBadge + keysSection) exceeds
the width even with all optional badges dropped.

When `JoinHorizontal` output exceeds the terminal width, the terminal emulator wraps it to a second
line - consuming an extra row of content space. The outer `finalStyle` uses `MaxHeight(m.height)` so
the overflow row pushes content off screen rather than growing the frame.

---

## Footer Architecture

All views share a single footer path through `Model.renderFooter()`. View-specific behavior comes from
two extraction methods:

- **`extractFilterBadge()`** - produces the left-anchored badge. In `focusLabelDashboard` mode,
  this encodes the entire nav instruction set as filter text (e.g. `"LABELS: j/k nav • h detail ..."`),
  making the badge 60 chars wide.
- **`extractKeyHints()`** - produces the right-anchored hint pills. View-specific branches for
  `ViewInsights`, `ViewGraph`, `ViewBoard`, `ViewHistory`, `ViewActionable`, `ViewFlowMatrix`.
  All other views (list, detail, split, sprint, tree, attention) fall through to the default list-mode hints.
- **`extractHintText()`** - produces the center label hint. Only `ViewBoard` and `ViewAttention` have
  custom text; all others use `"l:labels"` (10 chars).

Fixed always-present components (single-project, no optional badges):

| Component | Width |
|---|---|
| `filterBadge` (OPEN) | 9 |
| `labelHint` (l:labels) | 10 |
| `statsSection` (4 counters) | 17 |
| `countBadge` (N issues) | 12 |
| `projectBadge` (tier 3, dropped first) | 4 |
| Separator overhead | 1 |

---

## Per-View Measurements

Measurements below assume single-project mode (project badge present), no optional state badges
(no filter active, no sort, no wisp, no alerts, no worker, no watcher, no update, no dataset warning).
"total_w" is the sum of all always-present content; "min_fit" is the minimum terminal width where
the footer fits in 1 row.

### list (normal/global, unfiltered)

**Key hints** (6): `⏎ details` | `t diff` | `S triage` | `l labels` | `Ctrl+R refresh` | `? help`  
**Label hint**: `l:labels`  
**Filter badge**: `📂 OPEN` (9 chars)

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 89 | no (wraps) | 4/6 |
| 80 | 100 | no (wraps) | 5/6 |
| 100 | 117 | no (wraps) | 6/6 |
| 120 | 117 | yes | 6/6 |
| 140 | 122 | yes | 6/6 |
| 160 | 122 | yes | 6/6 |

**Min fit width**: ~117 chars (wraps at 60-100).  
**Problem**: 6 key hints total 68 chars wide - too many for common terminal widths.  
**Notes**: Workspace mode adds `w projects` hint (+12 chars) and workspace/repo badges (+optional tier).

---

### list (split view)

**Key hints** (5): `tab focus` | `C copy` | `x export` | `Ctrl+R refresh` | `? help`  
**Label hint**: `l:labels`

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 89 | no (wraps) | 4/5 |
| 80 | 106 | no (wraps) | 5/5 |
| 100 | 106 | no (wraps) | 5/5 |
| 120 | 111 | yes | 5/5 |
| 140 | 111 | yes | 5/5 |
| 160 | 111 | yes | 5/5 |

**Min fit width**: ~111 chars (wraps at 60-100).

---

### detail (focus mode)

**Key hints** (5): `esc back` | `C copy` | `O edit` | `Ctrl+R refresh` | `? help`  
**Label hint**: `l:labels`

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 86 | no (wraps) | 4/5 |
| 80 | 103 | no (wraps) | 5/5 |
| 100 | 103 | no (wraps) | 5/5 |
| 120 | 108 | yes | 5/5 |
| 140 | 108 | yes | 5/5 |
| 160 | 108 | yes | 5/5 |

**Min fit width**: ~108 chars (wraps at 60-100).

---

### insights / attention (same panel dispatch)

**Key hints** (6): `h/l panels` | `e explain` | `⏎ jump` | `? help` | `A attention` | `F flow`  
**Label hint**: `l:labels`  
**Filter badge**: `📂 OPEN` (9 chars)

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 91 | no (wraps) | 4/6 |
| 80 | 114 | no (wraps) | 6/6 |
| 100 | 114 | no (wraps) | 6/6 |
| 120 | 119 | yes | 6/6 |
| 140 | 119 | yes | 6/6 |
| 160 | 119 | yes | 6/6 |

**Min fit width**: ~119 chars (wraps at 60-100). This is the view flagged in dogfood.  
**Problem**: 6 hints including `A attention` and `F flow` as separate entries. The view-navigation
hints (h/l panels, e explain) are all insightful, but "attention" and "flow" are mode-switch hints
shared with the default list hint set - they inflate hint count without adding unique info at this view.

**Attention view** (uses `extractHintText()` → `"A:attention • 1-9 filter • esc close"` as label hint,
38 chars, but falls through to default list key hints):

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 145 | no | - |
| 80 | 145 | no | - |
| 100 | 145 | no | - |
| 120 | 145 | no | - |
| 140 | 145 | no | - |
| 160 | 145 | yes | - |

**Min fit width**: 145+ chars. The `labelHint` carries a 38-char nav string while the key hints
simultaneously show the full 6-hint list-mode defaults - duplicated information in two different slots.

---

### board (kanban)

**Key hints** (4): `hjkl nav` | `G bottom` | `⏎ view` | `b list`  
**Label hint**: `1-4:col • o/c/r:filter • l:labels • /:search • ?:help` (55 chars - very long)  
**Filter badge**: `📋 ALL` (8 chars)

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 132 | no (wraps) | 4/4 |
| 80 | 132 | no (wraps) | 4/4 |
| 100 | 132 | no (wraps) | 4/4 |
| 120 | 132 | no (wraps) | 4/4 |
| 140 | 137 | yes | 4/4 |
| 160 | 137 | yes | 4/4 |

**Min fit width**: ~137 chars. **Board is the worst single-view problem.**  
**Root cause**: The `labelHint` slot carries a 55-char instruction string (`"1-4:col • o/c/r:filter • ..."`).
This is redundant - the right-side key pills already show `hjkl nav`, `⏎ view`, `b list`. The
label hint duplicates keyboard reference in both center and right slots simultaneously.

---

### history

**Key hints** (4): `j/k nav` | `tab focus` | `⏎ jump` | `h close`  
**Label hint**: `l:labels`

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 89 | no (wraps) | 4/4 |
| 80 | 89 | no (wraps) | 4/4 |
| 100 | 94 | yes | 4/4 |
| 120 | 94 | yes | 4/4 |
| 140 | 94 | yes | 4/4 |
| 160 | 94 | yes | 4/4 |

**Min fit width**: ~94 chars. Relatively lean - fits at 100+ with all 4 hints. Still wraps at 60-80.

---

### graph

**Key hints** (4): `hjkl nav` | `H/L scroll` | `⏎ view` | `g list`  
**Label hint**: `l:labels`

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 90 | no (wraps) | 4/4 |
| 80 | 90 | no (wraps) | 4/4 |
| 100 | 95 | yes | 4/4 |
| 120 | 95 | yes | 4/4 |
| 140 | 95 | yes | 4/4 |
| 160 | 95 | yes | 4/4 |

**Min fit width**: ~95 chars. Fits at 100+ with all hints. Wraps only at 60-80 (narrow terminal).

---

### actionable

**Key hints** (4): `j/k nav` | `⏎ view` | `a list` | `? help`  
**Label hint**: `A:attention • 1-9 filter • esc close` (38 chars)

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 113 | no (wraps) | 4/4 |
| 80 | 113 | no (wraps) | 4/4 |
| 100 | 113 | no (wraps) | 4/4 |
| 120 | 118 | yes | 4/4 |
| 140 | 118 | yes | 4/4 |
| 160 | 118 | yes | 4/4 |

**Min fit width**: ~118 chars. The label hint carries navigation instructions for the attention panel
(38 chars) while the key hints show actionable-specific hints - the `labelHint` text is stale, it
still says "attention" navigation context when the view is now "actionable". These two different
sets of hints are architecturally unrelated but co-occupy the footer.

---

### flow_matrix

**Key hints** (5): `j/k nav` | `tab panel` | `⏎ drill` | `esc back` | `f close`  
**Label hint**: `l:labels`

| Width | Total | Fits | Hints shown |
|---|---|---|---|
| 60 | 90 | no (wraps) | 4/5 |
| 80 | 101 | no (wraps) | 5/5 |
| 100 | 101 | no (wraps) | 5/5 |
| 120 | 106 | yes | 5/5 |
| 140 | 106 | yes | 5/5 |
| 160 | 106 | yes | 5/5 |

**Min fit width**: ~106 chars. Fits at 120+ with all 5 hints. Wraps at 60-100.

---

### label_dashboard

**Filter badge** in `focusLabelDashboard` mode: `"🏷️ LABELS: j/k nav • h detail • d drilldown • enter filter"` (60 chars)  
**Key hints**: falls to default list-mode hints (6 hints, 68 chars)  
**Label hint**: `l:labels`

| Width | Total | Fits |
|---|---|---|
| 60 | 168 | no (wraps at all widths) |
| 80 | 168 | no |
| 100 | 168 | no |
| 120 | 168 | no |
| 140 | 168 | no |
| 160 | 168 | no (still 8 chars over) |

**Min fit width**: ~168 chars. **Wraps at every practical terminal width.**  
**Root cause**: Navigation instructions are encoded in the *filter badge slot* (`extractFilterBadge()`
line 292-293) rather than in the key hints slot. The filter badge has no compression logic - it is
always fully rendered. The 60-char instruction string plus the normal hint set makes this footer
unrenderable in a single row at any terminal width up to 168 columns.

The label_drilldown and label_graph modals have similar problems (59 chars and 34 chars for the
filter badge respectively), placing them at 167 and 142 chars minimum.

---

### sprint

**Key hints** (6): default list hints (`⏎ details`, `t diff`, `S triage`, `l labels`, `Ctrl+R refresh`, `? help`)  
**Label hint**: `l:labels`

| Width | Total | Fits |
|---|---|---|
| 60 | 117 | no |
| 80 | 117 | no |
| 100 | 117 | no |
| 120 | 117 | yes |
| 140 | 117 | yes |
| 160 | 117 | yes |

**Min fit width**: ~117 chars. Sprint view has no custom key hints - it falls through to list-mode
defaults. The sprint-specific keys (`P close`, `j/k navigate sprints`) are not surfaced in the
shared footer at all; they appear only in the sprint dashboard panel's inline footer text.

---

### tree

**Key hints** (6): default list hints (same as sprint)  
**Label hint**: `l:labels`

Same measurements as sprint (117 chars min fit). Tree view also has no custom key hints.

---

## Cross-View Summary Table

| View | Min fit (cols) | Status at 80 | Status at 120 | Key problems |
|---|---|---|---|---|
| label_dashboard | 168 | wraps | wraps | Filter badge encodes full nav (60 chars); never fits |
| attention | 145 | wraps | wraps | Long labelHint + full default key hints; duplicate info |
| board | 137 | wraps | wraps | labelHint is 55 chars of redundant nav text |
| list (normal) | 117 | wraps | fits | 6 hints; Ctrl+R is 8 chars alone |
| list (split) | 111 | wraps | fits | 5 hints |
| detail | 108 | wraps | fits | 5 hints |
| sprint | 117 | wraps | fits | No custom hints; inherits full list set |
| tree | 117 | wraps | fits | No custom hints; inherits full list set |
| actionable | 118 | wraps | fits | Long labelHint (38 chars); label hint text is mismatched context |
| flow_matrix | 106 | wraps | fits | 5 hints |
| insights | 119 | wraps | fits | 6 hints; A+F are mode-switch hints that inflate count |
| history | 94 | wraps | fits | 4 lean hints |
| graph | 95 | wraps | fits | 4 lean hints |

**Views already fitting at 100+**: history (94), graph (95)  
**Views fitting at 120+**: flow_matrix (106), split (111), detail (108), list (117), sprint (117), tree (117), insights (119), actionable (118)  
**Views still broken at 140+**: board (137 - fits just barely), attention (145 - needs 160), label_dashboard (168 - never fits)

---

## Content Inventory and Redundancy Analysis

### Redundant information patterns

1. **labelHint carries keyboard navigation** (board, actionable, attention): The center `labelHint`
   slot renders instruction text (`"1-4:col • o/c/r:filter..."`, `"A:attention • 1-9 filter..."`).
   This text is long, not compressible by the tier-dropping system, and duplicates what could be in
   key pills. Every character in labelHint that encodes a key binding is a character that belongs
   in the right-anchored key pills instead.

2. **filterBadge encodes navigation in label_dashboard**: When `m.focused == focusLabelDashboard`,
   `extractFilterBadge()` returns the full nav string as badge text. The filter badge has no
   truncation and is never dropped by the tier system (it's always included). The label dashboard
   nav belongs in key pills, not the filter slot.

3. **Default hints shown in views with no relevant actions**: Sprint and tree views show the full
   list-mode key hints (`⏎ details`, `t diff`, `S triage`, `l labels`, `Ctrl+R refresh`, `? help`),
   none of which are sprint/tree-specific. Sprint has its own keys (`P close`, `j/k nav`) that are
   surfaced only in the view panel, not in the shared footer.

4. **Insights shows mode-switch hints as key pills**: `A attention` and `F flow` are rendered as
   separate pills in the key hints section, adding 2 entries (and ~20 chars) for navigation actions
   that are not unique to insights - the user can reach attention/flow from any view. A single
   `? help` pill already covers discoverability.

### Always-present overhead assessment

The always-present, non-compressible overhead is:
- `filterBadge` (min 9 chars) + `labelHint` (min 10 chars) + `statsSection` (17 chars) + `countBadge` (12 chars) = **48 chars minimum**

The key hints section adds 36-68 chars depending on view. The minimum functional footer (filter +
stats + count + 2 key hints) would be about 48 + 30 = 78 chars. Current footers range from 94
(history) to 168 (label_dashboard) due to accumulated hint and labelHint overhead.

### Proposed compression targets

Target: footer fits in 1 row at 80 cols minimum for all views, 60 cols for views with 4 or fewer hints.

| View | Target min fit | Required reduction |
|---|---|---|
| label_dashboard | 80 | -88 chars (move nav to key pills, drop filter badge long text) |
| attention | 80 | -65 chars (drop duplicate labelHint nav; use pills only) |
| board | 80 | -57 chars (move labelHint nav to pills or drop to "?" hint) |
| list (normal) | 100 | -17 chars (drop 2 hints, shorten Ctrl+R to icon) |
| insights | 100 | -19 chars (drop A+F pills, consolidate to 4 hints) |
| sprint | 100 | -17 chars (custom sprint hints instead of list defaults) |
| tree | 100 | -17 chars (custom tree hints instead of list defaults) |
| actionable | 100 | -18 chars (fix labelHint mismatch; shorter hint text) |
| flow_matrix | 100 | -6 chars (minor; already close) |
| split / detail | 100 | -6 to -11 chars (already close; 1-2 hint reduction) |

---

## Compression Proposals (Per View)

### label_dashboard

**Current**: filterBadge = 60-char nav string; key hints = 6 default list hints.  
**Proposed**: filterBadge = `"🏷️ LABELS"` (5 chars); key hints = `j/k nav | h detail | d drilldown | ⏎ filter | esc close` (5 pills).  
**Result**: ~9 + 10 + 17 + 12 + ~52 = ~100 chars (fits at 100).  
**Files**: `pkg/ui/model_footer.go` `extractFilterBadge()` + `extractKeyHints()`.

### board

**Current**: labelHint = `"1-4:col • o/c/r:filter • l:labels • /:search • ?:help"` (55 chars).  
**Proposed**: labelHint = `"l:labels"` (10 chars); merge board-specific nav into key pills: `1-4:col | o/c/r:filter | /:search | ⏎ view | b list`.  
**Result**: ~8 + 10 + 17 + 12 + ~55 = ~102 chars (fits at 100+, currently needs 137).  
**Files**: `pkg/ui/model_footer.go` `extractHintText()` + `extractKeyHints()`.

### attention / actionable

**Current**: labelHint = `"A:attention • 1-9 filter • esc close"` (38 chars); key hints = 4-6 default list pills.  
**Proposed**: labelHint = `"l:labels"` or empty; move attention-specific nav to key pills only.  
**Files**: `pkg/ui/model_footer.go` `extractHintText()`.

### insights

**Current**: 6 key hints including `A attention` and `F flow`.  
**Proposed**: 4 hints: `h/l panels | e explain | ⏎ jump | ? help`. Drop `A attention` and `F flow` - mode-switch actions are discoverable via `?`.  
**Result**: ~9 + 10 + 17 + 12 + ~45 = ~93 chars (fits at 100).  
**Files**: `pkg/ui/model_footer.go` `extractKeyHints()` ViewInsights branch.

### list (normal)

**Current**: 6 hints including `Ctrl+R refresh` (expensive in char count).  
**Proposed**: 5 hints: `⏎ details | t diff | S triage | l labels | ? help`. Drop `Ctrl+R` (F5 works too, not worth 12+ chars of footer space).  
**Result**: ~9 + 10 + 17 + 12 + ~55 = ~103 chars (fits at 100+).  
**Files**: `pkg/ui/model_footer.go` `extractKeyHints()` default branch.

### sprint / tree

**Current**: Inherits 6 default list hints, none sprint/tree-specific.  
**Proposed sprint**: 3 hints: `j/k nav | P close | ? help`.  
**Proposed tree**: 3-4 hints: `j/k nav | ⏎ expand | t list | ? help`.  
**Result**: ~9 + 10 + 17 + 12 + ~36 = ~84 chars (fits at 80+).  
**Files**: `pkg/ui/model_footer.go` `extractKeyHints()` - add `ViewSprint` and `ViewTree` branches.

### history / graph / flow_matrix

History (94 chars) and graph (95 chars) already fit at 100+ with 4 hints each. No urgent action needed, but minor trimming to fit at 80 would improve narrow terminal experience.

- History: already minimal, wraps only at 60-80. Fine as-is, or shorten one hint.
- Graph: same profile as history.
- Flow_matrix (106 chars): fits at 120+; 1-hint reduction would bring to 100+.

---

## Source Files

All footer logic lives in `pkg/ui/model_footer.go`:

- `extractFilterBadge()` (line 291) - filter badge content; label_dashboard nav is here
- `extractHintText()` (line 345) - center labelHint; board/attention overrides here
- `extractKeyHints()` (line 484) - right key pills; all view-specific hint lists here
- `FooterData.Render()` (line 559) - assembly, tier-dropping compression, filler layout

The tier-dropping system (lines 799-838) does not help with the core problem: the tier system only
drops *optional state badges* (project name, search mode, sort, wisp, label filter, etc.). The
structural content - filterBadge, labelHint, statsSection, countBadge, keysSection - is never
dropped. Fixing the overflow requires reducing the character count of one or more of those five
structural components per view.

---

## Measurement Methodology

Measurements computed analytically using `charm.land/lipgloss/v2` `lipgloss.Width()` against the
actual rendered badge strings, mirroring the compression logic in `FooterData.Render()`. The
measurement programs are in `_tmp/measure_footer2.go`, `_tmp/measure_label_dash.go`, and
`_tmp/measure_more.go` (gitignored scratch).

Note: "wraps" means the rendered output width exceeds the terminal column count. Terminal emulators
visually wrap the overflow to a second line. The `finalStyle.MaxHeight(m.height)` in `View()` clips
the combined `body + footer` to terminal height, so the second wrapped row consumes one row of
content area. The footer has no intrinsic height beyond 1 row - the problem is purely content width.

---

## Open Questions for Implementation

1. **labelHint slot purpose**: Should it remain as a context-aware hint (currently `"l:labels"` for
   most views) or become a status indicator only? The board/attention/actionable cases suggest it
   has been repurposed as an overflow slot for navigation text that doesn't fit in the filter or
   key pills. Clarifying intent before implementation prevents the same drift.

2. **Minimum supported width**: The bead target is "1 row at all supported widths". What is the
   minimum supported width? If it's 80, the targets above are achievable. If it's 60, every view
   needs more aggressive reduction (likely: drop stats section when narrow, or hide count badge).

3. **Stats section compressibility**: The stats section (`○N ◉N ◈N ●N`) is always rendered and
   is 17 chars. At very narrow widths (60), even with a minimal footer, the fixed overhead
   (filter + hint + stats + count = 48 chars) limits the remaining hint budget to ~12 chars for
   60-col terminals. Consider making stats a Tier 1 optional badge instead of always-present.
