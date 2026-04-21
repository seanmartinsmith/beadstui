package analysis

import (
	"sort"
	"strings"
)

// SigilMode selects the DetectSigils recognizer set. Strict only recognizes
// explicit markdown sigils (markdown_link, inline_code, ref_keyword). Verb
// adds natural-language verb-proximity. Permissive ignores sigil contexts
// entirely and emits bare_mention for every bead-ID that is not inside a
// fenced block or inline-code span.
type SigilMode int

// Sigil mode enum values. Kept as iota constants so callers can switch
// exhaustively and so zero-value SigilMode is strict (safest default for a
// schema-v2 consumer that forgot to configure a mode).
const (
	SigilModeStrict SigilMode = iota
	SigilModeVerb
	SigilModePermissive
)

// MaxSigilBodyBytes bounds DetectSigils input. Bodies longer than this are
// truncated to the first MaxSigilBodyBytes before scanning, and every
// emitted match from the truncated body carries Truncated=true. 1 MiB is
// two orders of magnitude larger than any real bead description; past that
// threshold we treat the body as adversarial and stop reading.
const MaxSigilBodyBytes = 1 << 20

// maxFenceDepth bounds the fence stack against pathological nested-fence
// inputs. Depth above this silently ignores further openers (content still
// scans as non-code). 32 frames exceeds any realistic markdown document.
const maxFenceDepth = 32

// verbProximityChars is the inclusive same-line character budget between the
// nearest verb and the bead-ID for SigilKindVerb emission. Measured in
// markdown-stripped chars (*, _, ~ removed before counting).
const verbProximityChars = 32

// Sigil-kind constants. Match the enum in pkg/view/schemas/ref_record.v2.json.
// The reader in pkg/view sets these on RefRecordV2.SigilKind; external_dep
// and bare_dep are dep-only kinds emitted there, not by DetectSigils.
const (
	SigilKindMarkdownLink = "markdown_link"
	SigilKindInlineCode   = "inline_code"
	SigilKindRefKeyword   = "ref_keyword"
	SigilKindVerb         = "verb"
	SigilKindBareMention  = "bare_mention"
)

// sigilKindPriority is the dedup order when multiple kinds fire for the same
// ID inside one body. Higher wins. markdown_link is strongest (author wrapped
// the ID in explicit link syntax); bare_mention is weakest (permissive
// fallback). Priorities are only used for in-body dedup — cross-body dedup
// happens at the caller.
var sigilKindPriority = map[string]int{
	SigilKindMarkdownLink: 5,
	SigilKindInlineCode:   4,
	SigilKindRefKeyword:   3,
	SigilKindVerb:         2,
	SigilKindBareMention:  1,
}

// verbList enumerates the natural-language verbs SigilModeVerb attaches to
// nearby bead-IDs. Lowercase; the scanner lowercases source bytes before
// comparing. Entries may contain an internal space ("paired with") but no
// other punctuation.
var verbList = []string{
	"paired with",
	"blocks",
	"closes",
	"fixes",
	"mirrors",
	"see",
}

// SigilMatch is one bead-ID reference detected inside a prose body. Offset
// is the byte offset within the (possibly truncated) body at which the ID
// starts; callers use it for stable sort order in dedup, not for slicing.
// Truncated is true iff the input body exceeded MaxSigilBodyBytes and was
// clipped before scanning.
type SigilMatch struct {
	ID        string
	Kind      string
	Offset    int
	Truncated bool
}

// DetectSigils walks body once and emits SigilMatch records per the mode's
// recognizer set. Returns nil for empty bodies; returns a non-nil (possibly
// empty) slice for non-empty bodies that matched no sigils. Bodies larger
// than MaxSigilBodyBytes are truncated; every emitted match from a truncated
// body carries Truncated=true.
//
// Invariants:
//   - Single forward pass over the byte stream; no backtracking primitives.
//   - Fence stack capped at maxFenceDepth frames (excess openers ignored).
//   - Invalid UTF-8 tolerated: the scanner operates on bytes and ignores
//     multi-byte sequences that don't match an ASCII sigil char.
//   - Per-ID dedup within one body keeps the highest-priority kind; ties
//     prefer the earliest offset.
func DetectSigils(body string, mode SigilMode) []SigilMatch {
	if body == "" {
		return nil
	}
	truncated := false
	if len(body) > MaxSigilBodyBytes {
		body = body[:MaxSigilBodyBytes]
		truncated = true
	}
	s := sigilScanner{
		body:      body,
		mode:      mode,
		truncated: truncated,
		byID:      make(map[string]SigilMatch),
	}
	s.run()
	return s.sortedMatches()
}

// sigilScanner carries DetectSigils state for one call.
type sigilScanner struct {
	body       string
	mode       SigilMode
	truncated  bool
	fenceStack []byte // each entry is '`' or '~'; capped at maxFenceDepth
	byID       map[string]SigilMatch
}

// run peels off lines, toggles fence state, and dispatches non-fence lines
// to processLine. Loop is a single forward sweep over the body; each byte
// is visited at most twice (once to find line-end, once by processLine).
func (s *sigilScanner) run() {
	i := 0
	for i < len(s.body) {
		lineEnd := findLineEnd(s.body, i)
		line := s.body[i:lineEnd]
		if ch, _ := parseFence(line); ch != 0 {
			s.toggleFence(ch)
		} else if len(s.fenceStack) == 0 {
			s.processLine(i, line)
		}
		i = lineEnd
		if i < len(s.body) && s.body[i] == '\n' {
			i++
		}
	}
}

// toggleFence pushes or pops the fence stack based on the top frame. No-op
// when the stack would exceed maxFenceDepth.
func (s *sigilScanner) toggleFence(ch byte) {
	if n := len(s.fenceStack); n > 0 && s.fenceStack[n-1] == ch {
		s.fenceStack = s.fenceStack[:n-1]
		return
	}
	if len(s.fenceStack) >= maxFenceDepth {
		return
	}
	s.fenceStack = append(s.fenceStack, ch)
}

// processLine walks one non-fence line. In permissive mode it emits
// bare_mention for every bead-ID outside inline-code spans. In strict/verb
// mode it recognizes markdown links, inline code, ref: keywords, and
// (verb-only) verb proximity.
func (s *sigilScanner) processLine(lineStart int, line string) {
	if s.mode == SigilModePermissive {
		s.processLinePermissive(lineStart, line)
		return
	}
	s.processLineSigil(lineStart, line)
}

// processLinePermissive walks the line, skipping inline-code spans, and
// emits bare_mention for every bead-ID found outside them. Markdown link
// syntax is not special-cased — the IDs inside a `[bt-x](url)` surface as
// bare_mention, per spec ("no additional sigil requirement").
func (s *sigilScanner) processLinePermissive(lineStart int, line string) {
	i := 0
	for i < len(line) {
		c := line[i]
		if c == '`' {
			end := strings.IndexByte(line[i+1:], '`')
			if end < 0 {
				i++
				continue
			}
			i = i + 1 + end + 1
			continue
		}
		if isIDStartChar(c) && isBoundaryBefore(line, i) {
			if id, n := parseBeadID(line[i:]); n > 0 && isBoundaryAfter(line, i+n) {
				s.record(id, SigilKindBareMention, lineStart+i)
				i += n
				continue
			}
		}
		i++
	}
}

// verbHit records the stripped-space [start, end) range of a verb match on
// the current line. Used only in verb-mode proximity resolution.
type verbHit struct{ start, end int }

// processLineSigil is the strict + verb walker. It strips markdown format
// chars (`*`, `_`, `~`) into a parallel buffer so verb matching and
// proximity counting ignore them; IDs found anywhere map back to original
// byte offsets via stripLine's position table.
func (s *sigilScanner) processLineSigil(lineStart int, line string) {
	stripped, origPos := stripLine(line)

	var verbHits []verbHit

	// pending IDs are those seen in the main walk without an attached sigil
	// context. Verb mode resolves them against verbHits at line end; strict
	// drops them.
	type pendingID struct{ id string; origStart, stripStart, stripEnd int }
	var pending []pendingID

	i := 0
	for i < len(stripped) {
		c := stripped[i]

		// Inline-code span: `...` on the same line. Emit inline_code for
		// every bead-ID inside the span in both strict and verb modes.
		if c == '`' {
			close := strings.IndexByte(stripped[i+1:], '`')
			if close < 0 {
				i++
				continue
			}
			closeAbs := i + 1 + close
			content := stripped[i+1:closeAbs]
			s.scanSegmentForIDs(content, origPos, i+1, lineStart, SigilKindInlineCode)
			i = closeAbs + 1
			continue
		}

		// Markdown link: [text](url). Link text is required to be exactly
		// a bead-ID for markdown_link emission. URL contents are skipped
		// entirely — the closing paren marks resume position.
		if c == '[' {
			if n, ok := s.tryMarkdownLink(stripped, i, origPos, lineStart); ok {
				i += n
				continue
			}
		}

		// ref:/refs: keyword, case-insensitive, optional single space.
		if (c == 'r' || c == 'R') && looksLikeRefKeyword(stripped, i) {
			if n, ok := s.tryRefKeyword(stripped, i, origPos, lineStart); ok {
				i += n
				continue
			}
		}

		// Verb match (verb mode only). Record the stripped-space range;
		// proximity is resolved in the post-pass.
		if s.mode == SigilModeVerb {
			if n := matchVerb(stripped, i); n > 0 {
				verbHits = append(verbHits, verbHit{start: i, end: i + n})
				i += n
				continue
			}
		}

		// Bead-ID at a word boundary. Stash into pending for verb-mode
		// proximity resolution; strict mode drops pending at line end.
		if isIDStartChar(c) && isBoundaryBefore(stripped, i) {
			if id, n := parseBeadID(stripped[i:]); n > 0 && isBoundaryAfter(stripped, i+n) {
				pending = append(pending, pendingID{
					id:         id,
					origStart:  lineStart + origPos[i],
					stripStart: i,
					stripEnd:   i + n,
				})
				i += n
				continue
			}
		}

		i++
	}

	if s.mode != SigilModeVerb || len(verbHits) == 0 {
		return
	}
	// Two-pointer sliding window. verbHits and pending are both in
	// ascending walk order; proximity bound is a fixed 32 chars. Any verb
	// whose end falls below (pending.stripStart - 32) stays pruned for all
	// subsequent IDs, so vIdx only advances forward. Total work is
	// O(verbs + ids + matches) — avoids the O(verbs × ids) product that
	// surfaced on the 100KB linear-scaling benchmark.
	vIdx := 0
	for _, p := range pending {
		lowBound := p.stripStart - verbProximityChars
		for vIdx < len(verbHits) && verbHits[vIdx].end < lowBound {
			vIdx++
		}
		highBound := p.stripEnd + verbProximityChars
		for j := vIdx; j < len(verbHits); j++ {
			v := verbHits[j]
			if v.start > highBound {
				break
			}
			var dist int
			switch {
			case v.end <= p.stripStart:
				dist = p.stripStart - v.end
			case p.stripEnd <= v.start:
				dist = v.start - p.stripEnd
			default:
				dist = 0
			}
			if dist <= verbProximityChars {
				s.record(p.id, SigilKindVerb, p.origStart)
				break
			}
		}
	}
}

// tryMarkdownLink tries to consume a `[text](url)` sequence starting at
// stripped[i] = '['. Returns (consumed, true) when the link text is exactly
// a bead-ID and the URL parses cleanly; records markdown_link and the caller
// advances past the whole link. Returns (_, false) when the syntax doesn't
// parse — caller falls through.
func (s *sigilScanner) tryMarkdownLink(stripped string, i int, origPos []int, lineStart int) (int, bool) {
	if i+1 >= len(stripped) {
		return 0, false
	}
	close := strings.IndexByte(stripped[i+1:], ']')
	if close < 0 {
		return 0, false
	}
	closeAbs := i + 1 + close
	if closeAbs+1 >= len(stripped) || stripped[closeAbs+1] != '(' {
		return 0, false
	}
	linkText := stripped[i+1 : closeAbs]
	id, n := parseBeadID(linkText)
	if n == 0 || n != len(linkText) {
		return 0, false
	}
	endParen := strings.IndexByte(stripped[closeAbs+2:], ')')
	if endParen < 0 {
		return 0, false
	}
	endParenAbs := closeAbs + 2 + endParen
	s.record(id, SigilKindMarkdownLink, lineStart+origPos[i+1])
	return endParenAbs + 1 - i, true
}

// tryRefKeyword consumes `ref:<space?><id>` or `refs:<space?><id>` anchored
// at stripped[i]. Case-insensitive. Emits ref_keyword and returns the total
// consumed length when it parses; (_, false) when it doesn't.
func (s *sigilScanner) tryRefKeyword(stripped string, i int, origPos []int, lineStart int) (int, bool) {
	if !isBoundaryBefore(stripped, i) {
		return 0, false
	}
	kwLen := 0
	switch {
	case hasPrefixFold(stripped[i:], "refs:"):
		kwLen = 5
	case hasPrefixFold(stripped[i:], "ref:"):
		kwLen = 4
	default:
		return 0, false
	}
	p := i + kwLen
	if p < len(stripped) && stripped[p] == ' ' {
		p++
	}
	if p >= len(stripped) {
		return 0, false
	}
	id, n := parseBeadID(stripped[p:])
	if n == 0 || !isBoundaryAfter(stripped, p+n) {
		return 0, false
	}
	s.record(id, SigilKindRefKeyword, lineStart+origPos[p])
	return p + n - i, true
}

// scanSegmentForIDs walks a stripped substring (segment) for bead-IDs and
// emits the given sigil kind for each match. segStartStrip is the offset of
// the segment's first byte within the outer stripped line; origPos is that
// outer line's stripped→original position map.
func (s *sigilScanner) scanSegmentForIDs(segment string, origPos []int, segStartStrip, lineStart int, kind string) {
	i := 0
	for i < len(segment) {
		c := segment[i]
		if isIDStartChar(c) && isBoundaryBefore(segment, i) {
			if id, n := parseBeadID(segment[i:]); n > 0 && isBoundaryAfter(segment, i+n) {
				s.record(id, kind, lineStart+origPos[segStartStrip+i])
				i += n
				continue
			}
		}
		i++
	}
}

// record stores a match, enforcing per-ID priority: a higher-priority kind
// replaces a lower one for the same ID; ties prefer the earliest offset.
// Every recorded match carries the scanner-wide Truncated flag.
func (s *sigilScanner) record(id, kind string, offset int) {
	newMatch := SigilMatch{ID: id, Kind: kind, Offset: offset, Truncated: s.truncated}
	existing, found := s.byID[id]
	if !found {
		s.byID[id] = newMatch
		return
	}
	if sigilKindPriority[kind] > sigilKindPriority[existing.Kind] {
		s.byID[id] = newMatch
		return
	}
	if sigilKindPriority[kind] == sigilKindPriority[existing.Kind] && offset < existing.Offset {
		s.byID[id] = newMatch
	}
}

// sortedMatches returns the byID map as a slice sorted by Offset ascending
// (then by ID for tie-breaks). Empty body (byID nil/empty) returns nil to
// distinguish "no body" from "body with no matches" — the caller treats
// both as "no refs" so the distinction is cosmetic but consistent with
// Go's idiomatic "empty slice or nil" split.
func (s *sigilScanner) sortedMatches() []SigilMatch {
	if len(s.byID) == 0 {
		return nil
	}
	out := make([]SigilMatch, 0, len(s.byID))
	for _, m := range s.byID {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Offset != out[j].Offset {
			return out[i].Offset < out[j].Offset
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// parseFence detects a commonmark-light fenced-code delimiter at line start:
// up to 3 leading spaces, then 3+ of '`' or '~', ending with arbitrary
// info-string chars through the rest of the line. Returns (delimiter, count)
// when matched; (0, 0) otherwise. Only the delimiter char is returned — the
// fence stack tracks char, not run length, because Commonmark close-len
// matching is out of scope for intent detection.
func parseFence(line string) (byte, int) {
	p := 0
	for p < len(line) && p < 4 && line[p] == ' ' {
		p++
	}
	if p > 3 || p >= len(line) {
		return 0, 0
	}
	ch := line[p]
	if ch != '`' && ch != '~' {
		return 0, 0
	}
	count := 0
	for p < len(line) && line[p] == ch {
		p++
		count++
	}
	if count < 3 {
		return 0, 0
	}
	return ch, count
}

// findLineEnd returns the byte index of the next '\n' at or after i, or
// len(body) if none. Does not include the newline itself.
func findLineEnd(body string, i int) int {
	if idx := strings.IndexByte(body[i:], '\n'); idx >= 0 {
		return i + idx
	}
	return len(body)
}

// stripLine returns (stripped, origPos) where stripped is line with every
// '*', '_', and '~' removed, and origPos[k] is the byte offset in line of
// stripped[k]. Scan-once; allocates once.
func stripLine(line string) (string, []int) {
	hasFmt := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '*' || c == '_' || c == '~' {
			hasFmt = true
			break
		}
	}
	if !hasFmt {
		pos := make([]int, len(line))
		for i := range pos {
			pos[i] = i
		}
		return line, pos
	}
	buf := make([]byte, 0, len(line))
	pos := make([]int, 0, len(line))
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '*' || c == '_' || c == '~' {
			continue
		}
		buf = append(buf, c)
		pos = append(pos, i)
	}
	return string(buf), pos
}

// matchVerb returns the byte length of the verb starting at line[i], or 0.
// Case-insensitive; requires word boundaries on both sides. Spaces inside
// the verb ("paired with") match a single literal space.
func matchVerb(line string, i int) int {
	if !isBoundaryBefore(line, i) {
		return 0
	}
	for _, verb := range verbList {
		if i+len(verb) > len(line) {
			continue
		}
		candidate := line[i : i+len(verb)]
		if !equalFold(candidate, verb) {
			continue
		}
		if !isBoundaryAfter(line, i+len(verb)) {
			continue
		}
		return len(verb)
	}
	return 0
}

// looksLikeRefKeyword is a cheap gate before tryRefKeyword — matches only
// when the next few bytes could plausibly start "ref:" or "refs:" under
// case-insensitive comparison.
func looksLikeRefKeyword(stripped string, i int) bool {
	return hasPrefixFold(stripped[i:], "ref:") || hasPrefixFold(stripped[i:], "refs:")
}

// hasPrefixFold is ASCII-only case-insensitive HasPrefix. Avoids the full
// unicode.ToLower path since all sigil keywords are ASCII.
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		a := s[i]
		b := prefix[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

// equalFold is ASCII-only case-insensitive Equal. Same rationale as
// hasPrefixFold.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x := a[i]
		y := b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// parseBeadID consumes a bead-ID token starting at s[0]. Returns (id,
// lengthConsumed); returns ("", 0) if no ID at this position. Matches the
// shape enforced by analysis.SplitID plus optional dotted numeric segments
// for epic-child IDs like "mhwy.2.1". Does not validate prefix existence —
// that's the caller's job (ComputeRefRecordsV2 filters by knownPrefixes).
func parseBeadID(s string) (string, int) {
	if len(s) == 0 || !isLowerAlpha(s[0]) {
		return "", 0
	}
	i := 1
	for i < len(s) && isLowerAlphaNum(s[i]) {
		i++
	}
	if i >= len(s) || s[i] != '-' {
		return "", 0
	}
	i++
	if i >= len(s) || !isLowerAlphaNum(s[i]) {
		return "", 0
	}
	for i < len(s) && isLowerAlphaNum(s[i]) {
		i++
	}
	for i+1 < len(s) && s[i] == '.' && isDigit(s[i+1]) {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	return s[:i], i
}

// isIDStartChar reports whether c can begin a bead-ID (lowercase alpha).
func isIDStartChar(c byte) bool { return c >= 'a' && c <= 'z' }

// isLowerAlpha reports whether c is [a-z].
func isLowerAlpha(c byte) bool { return c >= 'a' && c <= 'z' }

// isLowerAlphaNum reports whether c is [a-z0-9].
func isLowerAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// isDigit reports whether c is [0-9].
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isAlphaNum reports whether c is [A-Za-z0-9].
func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// isBoundaryBefore reports whether position i in s is a valid bead-ID
// boundary start (beginning of line or preceded by a non-[alnum-] char).
func isBoundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	c := s[i-1]
	return !isAlphaNum(c) && c != '-'
}

// isBoundaryAfter reports whether position i in s is a valid bead-ID
// end boundary (end of string or followed by a non-[alnum-] char).
func isBoundaryAfter(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	c := s[i]
	return !isAlphaNum(c) && c != '-'
}
