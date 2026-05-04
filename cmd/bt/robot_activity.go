// cmd/bt/robot_activity.go
// `bt robot activity` — long-horizon CLI consumer of ~/.bt/events.jsonl.
//
// Bypasses Dolt and the issue loader entirely. Reads the append-only events
// file via events.LoadPersisted(path, 0) and applies date / kind / bead /
// repo / actor filters before projecting and emitting JSON or TOON.
//
// See bt-1puf for design notes. Distinct from `bd list --updated-after`,
// which answers state-as-of queries — activity returns the *stream* of
// captured changes (one row per Created/Edited/Closed/Commented event).

package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/pkg/recipe"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// ActivitySchemaV1 is the envelope schema marker for compact activity output.
// Full output (--shape full) omits the schema field, matching the convention
// established by CompactIssue.
const ActivitySchemaV1 = "activity.v1"

// CompactActivityEvent is the agent-facing index projection of events.Event.
// Drops CommentAt, Source, Dismissed (modal-only signal). Field order matches
// the example in the bt-1puf brief so on-the-wire JSON keys read naturally.
type CompactActivityEvent struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Bead    string    `json:"bead,omitempty"`
	Repo    string    `json:"repo,omitempty"`
	Title   string    `json:"title,omitempty"`
	Summary string    `json:"summary,omitempty"`
	Actor   string    `json:"actor,omitempty"`
}

// activityQuery records the filters applied, echoed back in the response
// envelope so agents can verify the binary parsed flags as expected.
type activityQuery struct {
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
	Preset  string `json:"preset,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Bead    string `json:"bead,omitempty"`
	Repo    string `json:"repo,omitempty"`
	Actor   string `json:"actor,omitempty"`
	Reverse bool   `json:"reverse,omitempty"`
	Limit   int    `json:"limit"`
}

// activityFlags bundles the parsed CLI inputs so filter logic stays pure
// (testable without cobra) and the cobra RunE stays thin.
type activityFlags struct {
	since   string
	until   string
	kind    string
	bead    string
	repo    string
	actor   string
	limit   int
	reverse bool

	// Preset flags. Mutually exclusive with each other and with since/until.
	today     bool
	thisWeek  bool
	thisMonth bool
	thisYear  bool
	lastMonth bool
	lastYear  bool
	inMonth   string // YYYY-MM
}

// parsedRange is the resolved time window after preset/since/until resolution.
// If both endpoints are zero the filter is open on both ends.
type parsedRange struct {
	since  time.Time
	until  time.Time
	label  string // human-readable preset label echoed back, e.g. "this-month"
}

// robotActivityCmd is the cobra entry point. Wired in init() at the bottom
// of cobra_robot.go's command-registration block? No — registered in this
// file's init() to keep activity-related code colocated, mirroring how
// robot_list.go owns its own command struct.
var robotActivityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Output bead-activity stream from ~/.bt/events.jsonl as JSON",
	Long: "Long-horizon consumer of the notifications event stream. Returns " +
		"individual Created/Edited/Closed/Commented events (not collapsed " +
		"per-bead state) for queries like 'what did we do this month?' or " +
		"'what beads were touched in september 2025?'. Reads the unbounded " +
		"on-disk file at ~/.bt/events.jsonl directly, bypassing the TUI's " +
		"7-day modal hydration filter. The --global flag is accepted and " +
		"ignored — events.jsonl is user-global by construction.",
	RunE: func(cmd *cobra.Command, args []string) error {
		af := readActivityFlags(cmd)

		rng, err := resolveActivityRange(af, time.Now())
		if err != nil {
			return err
		}

		path, err := events.DefaultPersistPath()
		if err != nil {
			return fmt.Errorf("resolving events path: %w", err)
		}

		// maxAge=0 → return every persisted event. The TUI hydration window
		// (DefaultModalDisplayAge) is intentionally bypassed here: the whole
		// point of this subcommand is the long-horizon read path.
		all, err := events.LoadPersisted(path, 0)
		if err != nil {
			return fmt.Errorf("loading events: %w", err)
		}

		filtered := filterActivity(all, rng, af)

		envelope := NewRobotEnvelope("")
		if robotOutputShape == robotShapeCompact {
			envelope.Schema = ActivitySchemaV1
		}

		query := activityQuery{
			Since:   af.since,
			Until:   af.until,
			Preset:  rng.label,
			Kind:    af.kind,
			Bead:    af.bead,
			Repo:    af.repo,
			Actor:   af.actor,
			Reverse: af.reverse,
			Limit:   af.limit,
		}

		var payload any
		if robotOutputShape == robotShapeCompact {
			payload = projectActivityCompact(filtered)
		} else {
			payload = filtered
		}

		// We deliberately don't carry a data_hash — events.jsonl is an
		// append-only stream, not a snapshot. Agents reasoning about it
		// should key off (since, until, count) rather than a hash.
		output := struct {
			RobotEnvelope
			Query  activityQuery `json:"query"`
			Count  int           `json:"count"`
			Events any           `json:"events"`
		}{
			RobotEnvelope: envelope,
			Query:         query,
			Count:         len(filtered),
			Events:        payload,
		}

		enc := newRobotEncoder(os.Stdout)
		if err := enc.Encode(output); err != nil {
			return fmt.Errorf("encoding robot-activity: %w", err)
		}
		return nil
	},
}

// readActivityFlags pulls every flag value off the cobra command into a
// plain struct. Keeps RunE small and the flag→struct mapping in one place.
func readActivityFlags(cmd *cobra.Command) activityFlags {
	af := activityFlags{}
	af.since, _ = cmd.Flags().GetString("since")
	af.until, _ = cmd.Flags().GetString("until")
	af.kind, _ = cmd.Flags().GetString("kind")
	af.bead, _ = cmd.Flags().GetString("bead")
	af.repo, _ = cmd.Flags().GetString("repo")
	af.actor, _ = cmd.Flags().GetString("actor")
	af.limit, _ = cmd.Flags().GetInt("limit")
	af.reverse, _ = cmd.Flags().GetBool("reverse")

	af.today, _ = cmd.Flags().GetBool("today")
	af.thisWeek, _ = cmd.Flags().GetBool("this-week")
	af.thisMonth, _ = cmd.Flags().GetBool("this-month")
	af.thisYear, _ = cmd.Flags().GetBool("this-year")
	af.lastMonth, _ = cmd.Flags().GetBool("last-month")
	af.lastYear, _ = cmd.Flags().GetBool("last-year")
	af.inMonth, _ = cmd.Flags().GetString("in")
	return af
}

// resolveActivityRange picks at most one preset flag (or --in YYYY-MM) and
// computes the [since, until) window, OR honors explicit --since/--until.
// The two flag families are mutually exclusive: mixing returns an error.
//
// `now` is parameterized so tests don't depend on wall-clock time.
func resolveActivityRange(af activityFlags, now time.Time) (parsedRange, error) {
	presetCount := 0
	var presetLabel string
	if af.today {
		presetCount++
		presetLabel = "today"
	}
	if af.thisWeek {
		presetCount++
		presetLabel = "this-week"
	}
	if af.thisMonth {
		presetCount++
		presetLabel = "this-month"
	}
	if af.thisYear {
		presetCount++
		presetLabel = "this-year"
	}
	if af.lastMonth {
		presetCount++
		presetLabel = "last-month"
	}
	if af.lastYear {
		presetCount++
		presetLabel = "last-year"
	}
	if af.inMonth != "" {
		presetCount++
		presetLabel = "in:" + af.inMonth
	}

	if presetCount > 1 {
		return parsedRange{}, fmt.Errorf("preset flags are mutually exclusive (got %d)", presetCount)
	}
	if presetCount == 1 && (af.since != "" || af.until != "") {
		return parsedRange{}, fmt.Errorf("preset flags are mutually exclusive with --since/--until")
	}

	// Preset path.
	if presetCount == 1 {
		return computePresetRange(af, now, presetLabel)
	}

	// Explicit since/until path. Both are optional; absent endpoints stay
	// zero (= unbounded on that side).
	rng := parsedRange{}
	if af.since != "" {
		t, err := parseActivityTime(af.since, now)
		if err != nil {
			return parsedRange{}, fmt.Errorf("--since: %w", err)
		}
		rng.since = t
	}
	if af.until != "" {
		t, err := parseActivityTime(af.until, now)
		if err != nil {
			return parsedRange{}, fmt.Errorf("--until: %w", err)
		}
		rng.until = t
	}
	if !rng.since.IsZero() && !rng.until.IsZero() && rng.until.Before(rng.since) {
		return parsedRange{}, fmt.Errorf("--until (%s) is before --since (%s)", af.until, af.since)
	}
	return rng, nil
}

// computePresetRange computes [start, end) for a preset label. Boundaries
// land on local-timezone calendar edges — "this month" starts at midnight on
// the 1st in the user's TZ, not UTC midnight. Matches the recipe parser's
// existing behavior for date-only ISO strings.
func computePresetRange(af activityFlags, now time.Time, label string) (parsedRange, error) {
	loc := now.Location()
	rng := parsedRange{label: label}

	switch {
	case af.today:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		rng.since = start
		rng.until = start.AddDate(0, 0, 1)
	case af.thisWeek:
		// ISO-week-style Monday start. Go's Weekday() reports Sunday=0.
		offset := int(now.Weekday()) - 1
		if offset < 0 {
			offset = 6 // Sunday → 6 days back to last Monday
		}
		start := time.Date(now.Year(), now.Month(), now.Day()-offset, 0, 0, 0, 0, loc)
		rng.since = start
		rng.until = start.AddDate(0, 0, 7)
	case af.thisMonth:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		rng.since = start
		rng.until = start.AddDate(0, 1, 0)
	case af.thisYear:
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
		rng.since = start
		rng.until = start.AddDate(1, 0, 0)
	case af.lastMonth:
		start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, loc)
		rng.since = start
		rng.until = start.AddDate(0, 1, 0)
	case af.lastYear:
		start := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, loc)
		rng.since = start
		rng.until = start.AddDate(1, 0, 0)
	case af.inMonth != "":
		t, err := time.ParseInLocation("2006-01", af.inMonth, loc)
		if err != nil {
			return parsedRange{}, fmt.Errorf("--in: invalid YYYY-MM %q: %w", af.inMonth, err)
		}
		rng.since = t
		rng.until = t.AddDate(0, 1, 0)
	}
	return rng, nil
}

// activityRelativePattern matches relative durations like "7d", "2w", "1mo",
// "1m" (single-letter month form), "1y". Layered on top of the recipe parser
// because the brief mandates `1mo` syntax, which the recipe parser doesn't
// recognize on its own.
var activityRelativePattern = regexp.MustCompile(`^(?i)(\d+)(d|w|mo|m|y)$`)

// parseActivityTime accepts the same inputs as recipe.ParseRelativeTime plus
// the brief-mandated `1mo` form for months. Anything that doesn't match the
// extended relative grammar falls through to the recipe parser, which handles
// ISO 8601 date / RFC3339 / single-letter relative.
func parseActivityTime(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	if matches := activityRelativePattern.FindStringSubmatch(s); matches != nil {
		// Convert "1mo" → "1m" and delegate; recipe parser owns the
		// canonical relative→absolute math.
		unit := strings.ToLower(matches[2])
		if unit == "mo" {
			unit = "m"
		}
		return recipe.ParseRelativeTime(matches[1]+unit, now)
	}
	return recipe.ParseRelativeTime(s, now)
}

// filterActivity applies the resolved filter pipeline to the raw event slice
// and returns the surviving events in the requested order. Pure function;
// safe to call from tests without any I/O.
func filterActivity(all []events.Event, rng parsedRange, af activityFlags) []events.Event {
	// 1. Date range. Half-open [since, until). until.IsZero() = unbounded above.
	var dated []events.Event
	for _, e := range all {
		if !rng.since.IsZero() && e.At.Before(rng.since) {
			continue
		}
		if !rng.until.IsZero() && !e.At.Before(rng.until) {
			continue
		}
		dated = append(dated, e)
	}

	// 2. Kind filter (comma-separated).
	if af.kind != "" {
		kinds := parseKindFilter(af.kind)
		var kept []events.Event
		for _, e := range dated {
			if kinds[e.Kind] {
				kept = append(kept, e)
			}
		}
		dated = kept
	}

	// 3. Bead / repo / actor filters.
	if af.bead != "" {
		var kept []events.Event
		for _, e := range dated {
			if e.BeadID == af.bead {
				kept = append(kept, e)
			}
		}
		dated = kept
	}
	if af.repo != "" {
		var kept []events.Event
		for _, e := range dated {
			if e.Repo == af.repo {
				kept = append(kept, e)
			}
		}
		dated = kept
	}
	if af.actor != "" {
		var kept []events.Event
		for _, e := range dated {
			if e.Actor == af.actor {
				kept = append(kept, e)
			}
		}
		dated = kept
	}

	// 4. Sort. LoadPersisted preserves on-disk (oldest-first) order, but
	// nothing in the contract guarantees the file is sorted (the persister
	// appends in emission order, and clock skew on a single machine could
	// in principle reorder). Sort defensively here.
	sort.SliceStable(dated, func(i, j int) bool {
		return dated[i].At.Before(dated[j].At)
	})
	if af.reverse {
		// Reverse in place. Avoids pulling in slices.Reverse for one call site.
		for i, j := 0, len(dated)-1; i < j; i, j = i+1, j-1 {
			dated[i], dated[j] = dated[j], dated[i]
		}
	}

	// 5. Limit.
	if af.limit > 0 && len(dated) > af.limit {
		dated = dated[:af.limit]
	}
	return dated
}

// parseKindFilter parses a comma-separated list of EventKind names into a
// set keyed by EventKind. Unknown names are dropped silently, matching the
// "empty result for unknown filter" convention used by --source on robot
// list. Case-insensitive.
func parseKindFilter(s string) map[events.EventKind]bool {
	out := map[events.EventKind]bool{}
	for _, raw := range strings.Split(s, ",") {
		k, ok := eventKindFromString(strings.ToLower(strings.TrimSpace(raw)))
		if ok {
			out[k] = true
		}
	}
	return out
}

// eventKindFromString reverses the EventKind.String() mapping. Kept private
// to this file so the events package doesn't grow a parser API for one
// CLI consumer.
func eventKindFromString(s string) (events.EventKind, bool) {
	switch s {
	case "created":
		return events.EventCreated, true
	case "edited":
		return events.EventEdited, true
	case "closed":
		return events.EventClosed, true
	case "commented":
		return events.EventCommented, true
	case "bulk":
		return events.EventBulk, true
	case "system":
		return events.EventSystem, true
	}
	return 0, false
}

// projectActivityCompact converts events.Event records into the compact wire
// shape. Drops CommentAt, Source, Dismissed (modal-only signal that doesn't
// help long-horizon orientation queries). Output schema marker:
// ActivitySchemaV1.
func projectActivityCompact(in []events.Event) []CompactActivityEvent {
	if len(in) == 0 {
		// Return an empty slice (not nil) so the JSON wire form is `[]`
		// rather than `null`. Robot-mode convention: empty result is `[]`.
		return []CompactActivityEvent{}
	}
	out := make([]CompactActivityEvent, len(in))
	for i, e := range in {
		out[i] = CompactActivityEvent{
			ID:      e.ID,
			At:      e.At,
			Kind:    e.Kind.String(),
			Bead:    e.BeadID,
			Repo:    e.Repo,
			Title:   e.Title,
			Summary: e.Summary,
			Actor:   e.Actor,
		}
	}
	return out
}

func init() {
	robotActivityCmd.Flags().String("since", "", "Events at or after this time (relative like 7d/2w/1mo/1y or ISO date)")
	robotActivityCmd.Flags().String("until", "", "Events strictly before this time (relative or ISO date; default: unbounded)")
	robotActivityCmd.Flags().String("kind", "", "Filter by event kind: comma-separated list of created,edited,closed,commented,bulk,system")
	robotActivityCmd.Flags().String("bead", "", "Filter to one bead's activity timeline (exact ID match, e.g. bt-1puf)")
	robotActivityCmd.Flags().String("repo", "", "Filter by bead-prefix / source repo (e.g. bt, bd, mkt)")
	robotActivityCmd.Flags().String("actor", "", "Filter by actor name (exact match; events without actor are excluded)")
	robotActivityCmd.Flags().Int("limit", 500, "Cap result count (0 = unlimited)")
	robotActivityCmd.Flags().Bool("reverse", false, "Sort newest-first (default: oldest-first)")

	robotActivityCmd.Flags().Bool("today", false, "Preset: events from start of today onward (mutually exclusive with --since/--until)")
	robotActivityCmd.Flags().Bool("this-week", false, "Preset: events from this Monday onward")
	robotActivityCmd.Flags().Bool("this-month", false, "Preset: events from the 1st of this month onward")
	robotActivityCmd.Flags().Bool("this-year", false, "Preset: events from January 1st of this year onward")
	robotActivityCmd.Flags().Bool("last-month", false, "Preset: events from the previous calendar month")
	robotActivityCmd.Flags().Bool("last-year", false, "Preset: events from the previous calendar year")
	robotActivityCmd.Flags().String("in", "", "Preset: events from a specific calendar month (format: YYYY-MM)")

	robotCmd.AddCommand(robotActivityCmd)
}
