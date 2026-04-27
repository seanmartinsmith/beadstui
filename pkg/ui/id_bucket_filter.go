package ui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
)

// idPriorityFilter wraps a FilterFunc so that bead-ID matches pre-empt the
// underlying ranker (bt-i4yn). When the user types a short identifier-shaped
// token (e.g. `cmg`, `bt-i4yn`, `mhwy.1`), we short-circuit the ranker: every
// item whose ID contains the query lands at the top of the result list —
// ordered exact > suffix-exact > contains — and the inner ranker's remaining
// results are appended below, with duplicates filtered out.
//
// Shape:
//   - Works for fuzzy, semantic, and hybrid modes (wraps whichever inner is set).
//   - Only activates when the query looks like an ID token AND at least one item
//     matches on ID. Otherwise the inner ranker's output is returned unchanged,
//     so plain text queries like "pagerank bottleneck" are unaffected.
//
// Assumes IssueItem.FilterValue() emits the ID as the first whitespace-
// separated token (see item.go).
func idPriorityFilter(inner list.FilterFunc) list.FilterFunc {
	return func(term string, targets []string) []list.Rank {
		baseRanks := inner(term, targets)
		if !looksLikeIDQuery(term) {
			return baseRanks
		}

		lowered := strings.ToLower(strings.TrimSpace(term))
		if lowered == "" {
			return baseRanks
		}

		type idMatch struct {
			index    int
			score    int // lower = better: 0 exact, 1 suffix-exact, 2 substring
			matchPos int
			matchLen int
		}

		var matches []idMatch
		seen := make(map[int]bool)
		for i, target := range targets {
			id := extractIDToken(target)
			if id == "" {
				continue
			}
			idLower := strings.ToLower(id)

			switch {
			case idLower == lowered:
				matches = append(matches, idMatch{i, 0, 0, len(id)})
				seen[i] = true
			default:
				// Suffix after last '-' (bead ID suffix: `dotfiles-cmg` → `cmg`).
				dash := strings.LastIndexByte(idLower, '-')
				if dash >= 0 && dash+1 < len(idLower) && idLower[dash+1:] == lowered {
					matches = append(matches, idMatch{i, 1, dash + 1, len(lowered)})
					seen[i] = true
					continue
				}
				if pos := strings.Index(idLower, lowered); pos >= 0 {
					matches = append(matches, idMatch{i, 2, pos, len(lowered)})
					seen[i] = true
				}
			}
		}

		if len(matches) == 0 {
			return baseRanks
		}

		// Stable sort keeps equal-score matches in target order (→ items appear
		// in whatever the caller's base ordering was, e.g. by ID lexical).
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].score < matches[j].score
		})

		result := make([]list.Rank, 0, len(matches)+len(baseRanks))
		for _, m := range matches {
			matched := make([]int, m.matchLen)
			for k := 0; k < m.matchLen; k++ {
				matched[k] = m.matchPos + k
			}
			result = append(result, list.Rank{Index: m.index, MatchedIndexes: matched})
		}
		for _, r := range baseRanks {
			if !seen[r.Index] {
				result = append(result, r)
			}
		}
		return result
	}
}

// looksLikeIDQuery returns true for short tokens that plausibly name a bead —
// lowercase alphanumerics plus '-' and '.' (dot supported for molecule child
// suffixes like `bt-mhwy.1`). Rejects anything with whitespace, punctuation,
// or longer than the longest realistic project-prefix + suffix combo.
func looksLikeIDQuery(term string) bool {
	t := strings.TrimSpace(strings.ToLower(term))
	if len(t) < 2 || len(t) > 24 {
		return false
	}
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}

// multiTokenFilter wraps a FilterFunc so that comma-separated terms are treated
// as an OR query (bt-jwo3). Each non-empty trimmed token runs through the inner
// filter and the union of results is returned, deduped by target index.
// MatchedIndexes from multiple tokens that hit the same target are merged so
// per-token highlighting still renders.
//
// Comma is the separator because space already means "fuzzy substring within
// a term" in the underlying ranker. A term with no comma passes through to
// the inner filter unchanged, so single-token search is identical to today.
//
// Examples:
//   "z5jj, uahv" → both bt-z5jj and bt-uahv populate the list
//   "bt-z5jj"    → identical to inner(term, targets)
//   "z5jj,,uahv" → empty middle token is silently skipped
//
// Wrap order: this should sit OUTSIDE idPriorityFilter so per-token ID-priority
// bucket promotion still applies to each token independently.
func multiTokenFilter(inner list.FilterFunc) list.FilterFunc {
	return func(term string, targets []string) []list.Rank {
		tokens := splitCommaTokens(term)
		if len(tokens) <= 1 {
			return inner(term, targets)
		}

		result := make([]list.Rank, 0)
		seen := make(map[int]int)
		for _, tok := range tokens {
			for _, r := range inner(tok, targets) {
				if pos, exists := seen[r.Index]; exists {
					result[pos].MatchedIndexes = mergeMatchedIndexes(
						result[pos].MatchedIndexes, r.MatchedIndexes)
					continue
				}
				seen[r.Index] = len(result)
				result = append(result, r)
			}
		}
		return result
	}
}

// splitCommaTokens splits term on commas and returns trimmed non-empty tokens.
// A term with no comma is returned as a single-element slice (or nil if empty
// after trim) so callers can short-circuit on len <= 1.
func splitCommaTokens(term string) []string {
	if !strings.ContainsRune(term, ',') {
		t := strings.TrimSpace(term)
		if t == "" {
			return nil
		}
		return []string{t}
	}
	parts := strings.Split(term, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// mergeMatchedIndexes returns the sorted union of two matched-index slices.
// Used when multiple comma-separated tokens hit the same target — we want
// highlight rendering to cover every matched position, not just the first
// token's hit.
func mergeMatchedIndexes(a, b []int) []int {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	seen := make(map[int]bool, len(a)+len(b))
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		seen[v] = true
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// extractIDToken returns the first whitespace-separated token of target,
// provided it looks like a bead ID (contains '-' separating prefix and suffix).
// IssueItem.FilterValue() places the ID first for exactly this purpose.
func extractIDToken(target string) string {
	sp := strings.IndexByte(target, ' ')
	var candidate string
	switch sp {
	case -1:
		candidate = target
	case 0:
		return ""
	default:
		candidate = target[:sp]
	}
	if !strings.ContainsRune(candidate, '-') {
		return ""
	}
	return candidate
}
