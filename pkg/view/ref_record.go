package view

import (
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// RefRecordSchemaV1 identifies the wire shape produced by ComputeRefRecords.
// Emitted on the robot output envelope's `schema` field. Like PairRecord and
// PortfolioRecord the payload is compact-by-construction, so the schema is
// set unconditionally regardless of --shape.
const RefRecordSchemaV1 = "ref.v1"

// RefRecordSchemaV2 identifies the wire shape produced by ComputeRefRecordsV2.
// v2 narrows the intent surface: prose refs require a sigil (markdown link,
// inline code, ref: keyword, verb proximity, or a bare mention in permissive
// mode) to emit. The active sigil mode rides in the envelope's sigil_mode
// field so consumers can correlate record shape with detection policy. v1's
// cross-project-only filter is retained — removing it regresses FP rate to
// ~85% per prior dogfooding.
const RefRecordSchemaV2 = "ref.v2"

// Sigil-kind constants used on dep-derived v2 records. Prose sigils come
// from pkg/analysis.SigilKind*; dep records mint their own kinds because
// dep detection is about the dep-field shape, not prose tokenization.
const (
	sigilKindExternalDep = "external_dep"
	sigilKindBareDep     = "bare_dep"
)

// RefRecord is one detected cross-project bead reference. Emitted per
// (Source, Target, Location) tuple; duplicate mentions of the same target
// inside a single location collapse to one record, but the same target
// appearing in multiple locations emits multiple records.
//
// See docs/design/2026-04-20-bt-mhwy-3-refs.md for the authority on scope
// (cross-project only in v1), URL stripping, and flag semantics.
type RefRecord struct {
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Location string   `json:"location"`
	Flags    []string `json:"flags"`
}

// Location constants.
const (
	refLocationDescription = "description"
	refLocationNotes       = "notes"
	refLocationComments    = "comments"
	refLocationDeps        = "deps"
)

// Flag constants. Order here is the fixed output order inside RefRecord.Flags
// so agents can diff two runs byte-for-byte.
const (
	refFlagBroken         = "broken"
	refFlagStale          = "stale"
	refFlagOrphanedChild  = "orphaned_child"
	refFlagCrossProject   = "cross_project"
)

// idPattern matches bead-shaped identifiers with word boundaries. Pattern:
// letter-prefix, hyphen, alnum-suffix optionally followed by dotted numeric
// segments (e.g. "mhwy.2", "mhwy.2.1"). Uses a preceding/following
// non-alphanumeric guard instead of \b because Go RE2 treats `-` as a word
// character in some contexts; explicit boundaries remove the ambiguity.
//
// Match group 1 captures the full ID. The outer spans are sacrificial
// boundary guards that the scanner discards.
var idPattern = regexp.MustCompile(`(?:^|[^a-zA-Z0-9-])([a-z][a-z0-9]*-[a-z0-9]+(?:\.[0-9]+)*)(?:$|[^a-zA-Z0-9-])`)

// urlPattern matches http/https URL spans for pre-scan stripping. Crude on
// purpose: markdown parsing is deferred (see design doc).
var urlPattern = regexp.MustCompile(`https?://\S+`)

// ComputeRefRecords scans issues for cross-project bead references in deps,
// description, notes, and comments. Refs whose prefix matches the source's
// prefix are skipped (same-project refs are handled by the dep graph, not
// this subcommand). The prose scanner is also scoped to prefixes present in
// the input set — a token like "cross-project" that happens to split into
// ("cross", "project") isn't a real ref because no bead has the prefix
// "cross". This cuts the prose false-positive rate by ~90% on the real
// corpus (dogfooded 2026-04-21) at the cost of missing refs whose target
// project isn't loaded in this global view. Unresolved external: deps use
// a different branch (scanDeps) and bypass the prefix-scope filter because
// the external: shape itself is an unambiguous ref marker.
//
// Word-boundary-aware ID regex plus analysis.SplitID validation; URL spans
// stripped before matching. Nil/empty inputs return nil.
func ComputeRefRecords(issues []model.Issue) []RefRecord {
	if len(issues) == 0 {
		return nil
	}

	known := make(map[string]model.Issue, len(issues))
	knownPrefixes := make(map[string]struct{}, 8)
	for i := range issues {
		known[issues[i].ID] = issues[i]
		if prefix, _, ok := analysis.SplitID(issues[i].ID); ok {
			knownPrefixes[prefix] = struct{}{}
		}
	}
	parentClosed := buildClosedParentMap(issues, known)

	var out []RefRecord
	for i := range issues {
		src := issues[i]
		srcPrefix, _, ok := analysis.SplitID(src.ID)
		if !ok {
			continue
		}
		scanDeps(&out, src, srcPrefix, known, parentClosed)
		scanProse(&out, src, srcPrefix, src.Description, refLocationDescription, known, knownPrefixes, parentClosed)
		scanProse(&out, src, srcPrefix, src.Notes, refLocationNotes, known, knownPrefixes, parentClosed)
		scanProse(&out, src, srcPrefix, joinComments(src.Comments), refLocationComments, known, knownPrefixes, parentClosed)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Location < out[j].Location
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildClosedParentMap returns a set of issue IDs whose DepParentChild parent
// is closed. Used to flag orphaned_child. Only records the child when the
// child is still open (closed children can't be orphaned in this sense).
func buildClosedParentMap(issues []model.Issue, known map[string]model.Issue) map[string]bool {
	orphaned := make(map[string]bool)
	for i := range issues {
		child := issues[i]
		if child.Status.IsClosed() {
			continue
		}
		for _, dep := range child.Dependencies {
			if dep == nil || dep.Type != model.DepParentChild {
				continue
			}
			parent, ok := known[dep.DependsOnID]
			if !ok {
				continue
			}
			if parent.Status.IsClosed() {
				orphaned[child.ID] = true
				break
			}
		}
	}
	return orphaned
}

// scanDeps emits a broken ref for every unresolved external: dep. "Resolved"
// here means the canonical target exists in the input set — so a caller that
// ran `analysis.ResolveExternalDeps` upstream will have no `external:` forms
// left and this function emits nothing for resolvable refs. A caller that
// didn't resolve still gets a correct answer: if the canonical would have
// resolved, we skip it; otherwise we flag `broken`.
func scanDeps(out *[]RefRecord, src model.Issue, srcPrefix string, known map[string]model.Issue, _ map[string]bool) {
	const externalPrefix = "external:"
	seen := make(map[string]struct{})
	for _, dep := range src.Dependencies {
		if dep == nil {
			continue
		}
		if !strings.HasPrefix(dep.DependsOnID, externalPrefix) {
			continue
		}
		project, suffix, ok := parseExternalRefDep(dep.DependsOnID)
		if !ok {
			continue
		}
		if project == srcPrefix {
			continue
		}
		if _, dup := seen[dep.DependsOnID]; dup {
			continue
		}
		seen[dep.DependsOnID] = struct{}{}

		if _, resolvable := lookupExternalCanonical(project, suffix, known); resolvable {
			continue
		}

		*out = append(*out, RefRecord{
			Source:   src.ID,
			Target:   dep.DependsOnID,
			Location: refLocationDeps,
			Flags:    []string{refFlagBroken, refFlagCrossProject},
		})
	}
}

// scanProse runs the ID pattern over a single text body after URL stripping.
// Dedups targets within the (source, location) scope. Only emits refs whose
// prefix is in knownPrefixes — keeps "round-trip" / "per-issue" / etc. out
// of output because no loaded project has those prefixes.
func scanProse(out *[]RefRecord, src model.Issue, srcPrefix, body, location string, known map[string]model.Issue, knownPrefixes map[string]struct{}, parentClosed map[string]bool) {
	if body == "" {
		return
	}
	stripped := urlPattern.ReplaceAllString(body, " ")
	matches := idPattern.FindAllStringSubmatch(stripped, -1)
	if len(matches) == 0 {
		return
	}
	seen := make(map[string]struct{})
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		target := m[1]
		prefix, _, ok := analysis.SplitID(target)
		if !ok {
			continue
		}
		if prefix == srcPrefix {
			continue
		}
		if _, known := knownPrefixes[prefix]; !known {
			continue
		}
		if _, dup := seen[target]; dup {
			continue
		}
		seen[target] = struct{}{}

		flags := computeRefFlags(target, known, parentClosed)
		*out = append(*out, RefRecord{
			Source:   src.ID,
			Target:   target,
			Location: location,
			Flags:    flags,
		})
	}
}

// computeRefFlags builds the flag slice for a cross-project prose ref,
// preserving the fixed output order.
func computeRefFlags(target string, known map[string]model.Issue, parentClosed map[string]bool) []string {
	var flags []string
	iss, found := known[target]
	if !found {
		flags = append(flags, refFlagBroken)
	} else {
		if iss.Status.IsClosed() {
			flags = append(flags, refFlagStale)
		}
		if parentClosed[target] {
			flags = append(flags, refFlagOrphanedChild)
		}
	}
	flags = appendCrossProject(flags)
	return flags
}

// appendCrossProject adds the cross_project sentinel at the end so every
// emitted record includes it (v1 invariant, see design doc).
func appendCrossProject(flags []string) []string {
	return append(flags, refFlagCrossProject)
}

// parseExternalRefDep parses "external:<project>:<suffix>" the same way the
// resolver does. Duplicated shape here instead of exporting from pkg/analysis
// because the view-side scanner only needs the split, not the resolution.
func parseExternalRefDep(ref string) (project, suffix string, ok bool) {
	const externalPrefix = "external:"
	rest := strings.TrimPrefix(ref, externalPrefix)
	if len(rest) == len(ref) {
		return "", "", false
	}
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 || colon == len(rest)-1 {
		return "", "", false
	}
	project = rest[:colon]
	suffix = rest[colon+1:]
	if strings.ContainsRune(project, ':') || strings.ContainsRune(suffix, ':') {
		return "", "", false
	}
	return project, suffix, true
}

// lookupExternalCanonical resolves an external:<project>:<suffix> pair to a
// known issue by exact suffix match under the target prefix.
func lookupExternalCanonical(project, suffix string, known map[string]model.Issue) (model.Issue, bool) {
	for id, iss := range known {
		prefix, sfx, ok := analysis.SplitID(id)
		if !ok {
			continue
		}
		if prefix == project && sfx == suffix {
			return iss, true
		}
	}
	return model.Issue{}, false
}

// joinComments flattens a comment slice into a single text body for scanning.
// Comment boundaries are lost, but refs detection is per (source, location)
// and comment-level provenance isn't a v1 requirement.
func joinComments(comments []*model.Comment) string {
	if len(comments) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range comments {
		if c == nil || c.Text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(c.Text)
	}
	return b.String()
}

// RefRecordV2 is the intent-based ref projection. Mirrors RefRecord's shape
// but adds two fields: `SigilKind` names the syntactic evidence that
// established intent (markdown_link, inline_code, ref_keyword, verb,
// bare_mention, external_dep, bare_dep), and `Truncated` flags records
// whose source body exceeded the 1MB per-body sigil-detection cap.
type RefRecordV2 struct {
	Source    string   `json:"source"`
	Target    string   `json:"target"`
	Location  string   `json:"location"`
	Flags     []string `json:"flags"`
	SigilKind string   `json:"sigil_kind"`
	Truncated bool     `json:"truncated,omitempty"`
}

// ComputeRefRecordsV2 is the v2 reader: same cross-project scope + v1 flag
// semantics as ComputeRefRecords, but prose scanning is delegated to
// analysis.DetectSigils under the caller-chosen SigilMode. Records carry
// the sigil_kind that established intent, plus a truncated flag when the
// source body exceeded the 1MB sigil-detection cap.
//
// Panic safety: each issue's sigil detection is wrapped in a defer/recover
// so a single malformed or adversarial body logs and skips rather than
// crashing `--global` for the rest of the corpus. The recover path emits
// no records for that issue.
//
// Nil/empty inputs return nil. Sort order matches v1:
// (source, target, location) ascending.
func ComputeRefRecordsV2(issues []model.Issue, mode analysis.SigilMode) []RefRecordV2 {
	if len(issues) == 0 {
		return nil
	}

	known := make(map[string]model.Issue, len(issues))
	knownPrefixes := make(map[string]struct{}, 8)
	for i := range issues {
		known[issues[i].ID] = issues[i]
		if prefix, _, ok := analysis.SplitID(issues[i].ID); ok {
			knownPrefixes[prefix] = struct{}{}
		}
	}
	parentClosed := buildClosedParentMap(issues, known)

	var out []RefRecordV2
	for i := range issues {
		src := issues[i]
		srcPrefix, _, ok := analysis.SplitID(src.ID)
		if !ok {
			continue
		}
		scanDepsV2(&out, src, srcPrefix, known)
		scanIssueProseV2(&out, src, srcPrefix, known, knownPrefixes, parentClosed, mode)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Location < out[j].Location
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

// scanIssueProseV2 runs DetectSigils against each prose field on the source
// issue and converts matches into RefRecordV2. Each field is wrapped in a
// recover so an adversarial body in one field (or one issue) can't tank
// the --global walk.
func scanIssueProseV2(out *[]RefRecordV2, src model.Issue, srcPrefix string, known map[string]model.Issue, knownPrefixes map[string]struct{}, parentClosed map[string]bool, mode analysis.SigilMode) {
	fields := []struct {
		location string
		body     string
	}{
		{refLocationDescription, src.Description},
		{refLocationNotes, src.Notes},
		{refLocationComments, joinComments(src.Comments)},
	}
	for _, f := range fields {
		if f.body == "" {
			continue
		}
		scanProseV2Safe(out, src, srcPrefix, f.body, f.location, known, knownPrefixes, parentClosed, mode)
	}
}

// scanProseV2Safe wraps a single prose scan in a recover so a panic from
// adversarial input skips the body with a debug log rather than killing
// the whole projection run.
func scanProseV2Safe(out *[]RefRecordV2, src model.Issue, srcPrefix, body, location string, known map[string]model.Issue, knownPrefixes map[string]struct{}, parentClosed map[string]bool, mode analysis.SigilMode) {
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("ref.v2 sigil detection recovered from panic",
				"issue", src.ID,
				"location", location,
				"panic", fmt.Sprintf("%v", r),
			)
		}
	}()

	matches := analysis.DetectSigils(body, mode)
	if len(matches) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		prefix, _, ok := analysis.SplitID(m.ID)
		if !ok {
			continue
		}
		if prefix == srcPrefix {
			continue
		}
		if _, knownPrefix := knownPrefixes[prefix]; !knownPrefix {
			continue
		}
		if _, dup := seen[m.ID]; dup {
			continue
		}
		seen[m.ID] = struct{}{}
		*out = append(*out, RefRecordV2{
			Source:    src.ID,
			Target:    m.ID,
			Location:  location,
			Flags:     computeRefFlags(m.ID, known, parentClosed),
			SigilKind: m.Kind,
			Truncated: m.Truncated,
		})
	}
}

// scanDepsV2 mirrors scanDeps but emits RefRecordV2 with sigil_kind set.
// Unresolved external: deps get SigilKindExternalDep. Same resolvability
// rules as v1: if the canonical form exists in the known set, no record
// fires.
func scanDepsV2(out *[]RefRecordV2, src model.Issue, srcPrefix string, known map[string]model.Issue) {
	const externalPrefix = "external:"
	seen := make(map[string]struct{})
	for _, dep := range src.Dependencies {
		if dep == nil {
			continue
		}
		if !strings.HasPrefix(dep.DependsOnID, externalPrefix) {
			continue
		}
		project, suffix, ok := parseExternalRefDep(dep.DependsOnID)
		if !ok {
			continue
		}
		if project == srcPrefix {
			continue
		}
		if _, dup := seen[dep.DependsOnID]; dup {
			continue
		}
		seen[dep.DependsOnID] = struct{}{}

		if _, resolvable := lookupExternalCanonical(project, suffix, known); resolvable {
			continue
		}

		*out = append(*out, RefRecordV2{
			Source:    src.ID,
			Target:    dep.DependsOnID,
			Location:  refLocationDeps,
			Flags:     []string{refFlagBroken, refFlagCrossProject},
			SigilKind: sigilKindExternalDep,
		})
	}
}
