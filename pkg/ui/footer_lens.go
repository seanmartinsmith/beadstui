package ui

// Footer Zone 1 — the "lens" (bt-2vshd). The footer's left zone reads as a
// sentence: where am I -> what's filtered -> how it's ordered.
//
//	scope · st:<status> · lb:<label> · /<query> · recipe:<name> · by:<order>
//
// Scope is leftmost and always survives; the filter/order chips degrade away
// under width pressure with placeholders (lb:- , /-) dropping first. Two tiers
// render from one grammar: the ascii tier uses the doc's verbatim text prefixes
// (st: / lb: / / / by:), the Nerd Font tier swaps those prefixes for the glyph
// table's icons (Tag / Search / Sort, and folder/globe on scope). The status
// chip renders bare in the NF tier - the fa-filter funnel read as a stuck
// down-arrow at terminal size and was removed (bt-n9gn5, dogfood 2026-07-17).
// Design: docs/design/2026-07-16-footer-lens-redesign.md.

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// lensLevel controls how much of the lens sentence renders under width pressure.
// Higher = terser. The footer degradation cascade (FooterData.Render) advances
// the level in the approved drop order: placeholders first, then the filter
// words, with scope surviving to the very end.
type lensLevel int

const (
	lensFull       lensLevel = iota // scope · st: · lb:(-) · /(-) · recipe · by:
	lensNoPlace                     // same, minus the lb:- / /- space-holders
	lensStatusOnly                  // scope · <bare status word> (drops lb / / / recipe / by / the st: prefix)
	lensScopeOnly                   // scope alone
)

// asciiTier reports whether the active glyph tier is the pure-ASCII fallback.
// The lens uses it to pick text chip prefixes over Nerd Font icons. It compares
// against the canonical ascii table so it tracks BT_GLYPHS (and setGlyphs swaps
// in tests) without threading a tier flag through render state.
func asciiTier() bool { return activeGlyphs.Sep == asciiGlyphs.Sep }

// lensSortLabel maps a sort mode to the lens order token (pure ASCII, so the
// ascii-tier footer stays ASCII). SortDefault returns "" so the order chip is
// hidden when no explicit sort is active — the order bucket space-holds only via
// its silence, never a placeholder. The created pair shares one bare token;
// direction rides the chip's icon slot / ascii suffix (bt-uxyel), never a
// "-old" suffix.
func lensSortLabel(s SortMode) string {
	switch s {
	case SortUpdated:
		return "updated"
	case SortCreatedDesc, SortCreatedAsc:
		return "created"
	case SortPriority:
		return "priority"
	case SortProgress:
		return "progress"
	default: // SortDefault
		return ""
	}
}

// lensSortDir maps a sort mode to its direction key: "asc" / "desc" for the
// directional created pair, "" for single-direction sorts. The NF order chip
// renders the direction as its icon (up/down arrow replacing the static sort
// mark); the ascii tier marks only the non-default ascending direction with a
// ^ suffix (by:created^ vs the unmarked newest-first by:created).
func lensSortDir(s SortMode) string {
	switch s {
	case SortCreatedAsc:
		return "asc"
	case SortCreatedDesc:
		return "desc"
	default:
		return ""
	}
}

// lensChip renders one filter chip. In the ascii tier it is "<asciiPrefix><value>"
// (verbatim doc form: st:open, lb:-, /query); in the Nerd Font tier the prefix is
// dropped for the table icon: "<icon><value>".
func lensChip(asciiPrefix, icon, value string, ascii bool) string {
	if ascii {
		return asciiPrefix + value
	}
	if icon == "" {
		return value
	}
	return icon + value
}

// lensBareStatus returns the terse status word shown at lensStatusOnly. "all"
// (and "") narrows nothing, so it renders empty — collapsing that level to
// scope-only rather than showing an uninformative "all".
func lensBareStatus(status string) string {
	switch status {
	case "", "all":
		return ""
	case "in_progress":
		return "in-progress"
	default:
		return status
	}
}

// lensFilterChips builds the status/label/search/recipe/order chips for the
// full and no-placeholder levels. placeholders controls whether the label and
// search space-holders (lb:- , /-) render when those dimensions are inactive.
func lensFilterChips(fd FooterData, ascii, placeholders bool) []string {
	g := activeGlyphs
	var chips []string

	// Status: shown whenever a plain status filter owns membership. Empty when a
	// BQL query or a recipe owns membership instead (they render in their own
	// slots below), so the lens never double-states the same filter.
	if fd.StatusFilter != "" {
		val := fd.StatusFilter
		if val == "in_progress" {
			val = "in-progress"
		}
		// Bare value in the NF tier (no icon): the fa-filter funnel read as a
		// stuck down-arrow at terminal size (bt-n9gn5). Ascii keeps st:.
		chips = append(chips, lensChip("st:", "", val, ascii))
	}

	// Label (independent membership dimension).
	switch {
	case fd.LabelFilterText != "":
		chips = append(chips, lensChip("lb:", g.Tag, fd.LabelFilterText, ascii))
	case placeholders:
		chips = append(chips, lensChip("lb:", g.Tag, "-", ascii))
	}

	// Search / BQL query.
	switch {
	case fd.SearchQuery != "":
		chips = append(chips, lensChip("/", g.Search, fd.SearchQuery, ascii))
	case placeholders:
		chips = append(chips, lensChip("/", g.Search, "-", ascii))
	}

	// Recipe joins the bucket only while active (no space-holder).
	if fd.RecipeName != "" {
		chips = append(chips, lensChip("recipe:", g.FilterRecipe, fd.RecipeName, ascii))
	}

	// Order bucket: only for an explicit (non-default) sort. A directional
	// sort pair (created) carries its direction in the icon slot in the NF
	// tier and as a ^ suffix for the non-default ascending direction in ascii
	// (bt-uxyel); non-directional sorts keep the static sort mark.
	if fd.OrderLabel != "" {
		if ascii {
			val := fd.OrderLabel
			if fd.OrderDir == "asc" {
				val += g.SortAsc
			}
			chips = append(chips, "by:"+val)
		} else {
			icon := g.Sort
			switch fd.OrderDir {
			case "asc":
				icon = g.SortAsc
			case "desc":
				icon = g.SortDesc
			}
			chips = append(chips, icon+fd.OrderLabel)
		}
	}

	return chips
}

// renderLens composes the Zone-1 lens at the given degradation level. Glyphs are
// read live from activeGlyphs so a tier swap (setGlyphs) renders the right tier.
func renderLens(fd FooterData, lvl lensLevel) string {
	g := activeGlyphs
	ascii := asciiTier()

	scopeStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	chipStyle := lipgloss.NewStyle().Foreground(ColorSubtext)
	sep := lipgloss.NewStyle().Foreground(ColorMuted).Render(" " + g.Sep + " ")

	var segs []string

	// Scope (where am I). The Nerd Font tier prefixes it with the folder/globe
	// icon; the ascii tier shows the bare label (doc mockup: "bt" / "ALL(19)").
	if fd.ScopeLabel != "" {
		scope := fd.ScopeLabel
		if !ascii {
			icon := g.ScopeProject
			if fd.ScopeCrossProject {
				icon = g.ScopeAll
			}
			scope = icon + scope
		}
		segs = append(segs, scopeStyle.Render(scope))
	}

	switch lvl {
	case lensScopeOnly:
		// scope alone
	case lensStatusOnly:
		if w := lensBareStatus(fd.StatusFilter); w != "" {
			segs = append(segs, chipStyle.Render(w))
		}
	default: // lensFull, lensNoPlace
		for _, c := range lensFilterChips(fd, ascii, lvl == lensFull) {
			segs = append(segs, chipStyle.Render(c))
		}
	}

	return strings.Join(segs, sep)
}

// renderStaticHints returns the Zone-3 discoverability pair (bt-2vshd). Full form
// is "? help · ; keys"; the compact form drops the labels to "? ;" under width
// pressure. The ? overlay and ; sidebar are the two surfaces that actually teach
// navigation, so the per-view action pills were retired from the footer chrome —
// the per-view key.Maps now feed only those two surfaces.
func renderStaticHints(compact bool) string {
	keyStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	if compact {
		return keyStyle.Render("?") + " " + keyStyle.Render(";")
	}
	labelStyle := lipgloss.NewStyle().Foreground(ColorSubtext)
	sep := lipgloss.NewStyle().Foreground(ColorMuted).Render(" " + activeGlyphs.Sep + " ")
	return keyStyle.Render("?") + labelStyle.Render(" help") + sep +
		keyStyle.Render(";") + labelStyle.Render(" keys")
}
