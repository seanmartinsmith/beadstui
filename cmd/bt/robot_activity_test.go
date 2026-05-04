package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// activityFixedNow is the reference timestamp used by every range-resolution
// test below. Pinning the clock keeps preset boundaries deterministic across
// machines and timezones — tests construct their expected windows from this
// constant rather than time.Now().
//
// Chosen as a Wednesday in mid-month so this-week / this-month / today /
// in-month presets all produce non-degenerate, distinguishable windows.
var activityFixedNow = time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)

// mkEvent is a tiny constructor for events.Event records under test. ID is
// derived from BeadID+Kind+At inside the events package, but tests don't
// care about ID stability — they care about filtering behavior, so we set a
// human-recognizable string and skip the hash dance.
func mkEvent(id string, kind events.EventKind, at time.Time, beadID, repo, actor string) events.Event {
	return events.Event{
		ID:     id,
		Kind:   kind,
		BeadID: beadID,
		Repo:   repo,
		Title:  "title for " + beadID,
		Actor:  actor,
		At:     at,
	}
}

// activityFixtureEvents returns a deterministic sample stream spanning ~5
// months and 3 prefixes. Designed so each filter knob has at least one event
// it should keep and one it should drop.
func activityFixtureEvents() []events.Event {
	t := func(s string) time.Time {
		out, err := time.Parse(time.RFC3339, s)
		if err != nil {
			panic(err) // fixture time string typo - fail loud
		}
		return out
	}
	return []events.Event{
		mkEvent("e1", events.EventCreated, t("2026-01-15T10:00:00Z"), "bt-aaa", "bt", "sms"),
		mkEvent("e2", events.EventEdited, t("2026-02-10T11:00:00Z"), "bt-aaa", "bt", "sms"),
		mkEvent("e3", events.EventCreated, t("2026-03-05T09:00:00Z"), "bd-bbb", "bd", "agent"),
		mkEvent("e4", events.EventClosed, t("2026-04-20T16:00:00Z"), "bt-aaa", "bt", "sms"),
		mkEvent("e5", events.EventCommented, t("2026-04-21T12:00:00Z"), "bd-bbb", "bd", ""),
		mkEvent("e6", events.EventCreated, t("2026-05-01T08:30:00Z"), "mkt-ccc", "mkt", "sms"),
		mkEvent("e7", events.EventBulk, t("2026-05-06T13:00:00Z"), "bt-ddd", "bt", "sms"),
		mkEvent("e8", events.EventSystem, t("2026-05-06T14:00:00Z"), "", "", ""),
	}
}

// TestResolveActivityRange_PresetsLandOnCalendarBoundaries pins each preset's
// [since, until) window against the known clock so we don't accidentally
// regress the boundary math (off-by-one on Sunday-week, off-by-month on
// last-month at year boundaries, etc.).
func TestResolveActivityRange_PresetsLandOnCalendarBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		flags     activityFlags
		wantStart time.Time
		wantEnd   time.Time
		wantLabel string
	}{
		{
			name:      "today",
			flags:     activityFlags{today: true},
			wantStart: time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
			wantLabel: "today",
		},
		{
			name:      "this-week (Wednesday → Monday start)",
			flags:     activityFlags{thisWeek: true},
			wantStart: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), // Mon May 4 2026
			wantEnd:   time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
			wantLabel: "this-week",
		},
		{
			name:      "this-month",
			flags:     activityFlags{thisMonth: true},
			wantStart: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			wantLabel: "this-month",
		},
		{
			name:      "this-year",
			flags:     activityFlags{thisYear: true},
			wantStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantLabel: "this-year",
		},
		{
			name:      "last-month",
			flags:     activityFlags{lastMonth: true},
			wantStart: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			wantLabel: "last-month",
		},
		{
			name:      "last-year",
			flags:     activityFlags{lastYear: true},
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantLabel: "last-year",
		},
		{
			name:      "in YYYY-MM",
			flags:     activityFlags{inMonth: "2025-09"},
			wantStart: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
			wantLabel: "in:2025-09",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveActivityRange(tc.flags, activityFixedNow)
			if err != nil {
				t.Fatalf("resolveActivityRange: %v", err)
			}
			if !got.since.Equal(tc.wantStart) {
				t.Errorf("since: got %s want %s", got.since, tc.wantStart)
			}
			if !got.until.Equal(tc.wantEnd) {
				t.Errorf("until: got %s want %s", got.until, tc.wantEnd)
			}
			if got.label != tc.wantLabel {
				t.Errorf("label: got %q want %q", got.label, tc.wantLabel)
			}
		})
	}
}

// TestResolveActivityRange_PresetMutex enforces both mutex rules: (1) two
// presets together error, (2) any preset combined with --since/--until errors.
func TestResolveActivityRange_PresetMutex(t *testing.T) {
	cases := []struct {
		name  string
		flags activityFlags
	}{
		{"two presets", activityFlags{today: true, thisMonth: true}},
		{"preset + since", activityFlags{thisMonth: true, since: "7d"}},
		{"preset + until", activityFlags{thisYear: true, until: "2026-01-01"}},
		{"--in + this-month", activityFlags{inMonth: "2025-09", thisMonth: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolveActivityRange(tc.flags, activityFixedNow); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

// TestResolveActivityRange_RelativeAndIsoSince exercises the parseActivityTime
// extensions on top of recipe.ParseRelativeTime: ISO date, single-letter
// relative, and the brief-mandated `1mo` form.
func TestResolveActivityRange_RelativeAndIsoSince(t *testing.T) {
	cases := []struct {
		name      string
		since     string
		wantStart time.Time
	}{
		{"ISO date", "2025-09-01", time.Date(2025, 9, 1, 0, 0, 0, 0, activityFixedNow.Location())},
		{"1h", "1h", activityFixedNow.Add(-1 * time.Hour)},
		{"24h", "24h", activityFixedNow.Add(-24 * time.Hour)},
		{"6h", "6h", activityFixedNow.Add(-6 * time.Hour)},
		{"7d", "7d", activityFixedNow.AddDate(0, 0, -7)},
		{"2w", "2w", activityFixedNow.AddDate(0, 0, -14)},
		{"1mo (brief-mandated)", "1mo", activityFixedNow.AddDate(0, -1, 0)},
		{"1m (single-letter)", "1m", activityFixedNow.AddDate(0, -1, 0)},
		{"1y", "1y", activityFixedNow.AddDate(-1, 0, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveActivityRange(activityFlags{since: tc.since}, activityFixedNow)
			if err != nil {
				t.Fatalf("resolveActivityRange: %v", err)
			}
			if !got.since.Equal(tc.wantStart) {
				t.Errorf("since: got %s want %s", got.since, tc.wantStart)
			}
		})
	}
}

// TestResolveActivityRange_UntilBeforeSince catches the "user inverted the
// range" case explicitly so the binary returns a real error rather than
// silently emitting an empty result, which would mask flag typos.
func TestResolveActivityRange_UntilBeforeSince(t *testing.T) {
	flags := activityFlags{since: "2026-05-01", until: "2026-01-01"}
	if _, err := resolveActivityRange(flags, activityFixedNow); err == nil {
		t.Fatalf("expected error for until-before-since, got nil")
	}
}

// TestResolveActivityRange_SinceEqualsUntil — the half-open [since, until)
// window means since == until is a legal but empty range. The CLI shouldn't
// error here; the empty result is the answer.
func TestResolveActivityRange_SinceEqualsUntil(t *testing.T) {
	flags := activityFlags{since: "2026-05-01", until: "2026-05-01"}
	rng, err := resolveActivityRange(flags, activityFixedNow)
	if err != nil {
		t.Fatalf("resolveActivityRange: %v", err)
	}
	got := filterActivity(activityFixtureEvents(), rng, flags)
	if len(got) != 0 {
		t.Errorf("expected 0 events for since==until, got %d", len(got))
	}
}

// TestFilterActivity_DateRange picks a window that should include exactly
// the April events from the fixture (e4, e5).
func TestFilterActivity_DateRange(t *testing.T) {
	flags := activityFlags{since: "2026-04-01", until: "2026-05-01"}
	rng, err := resolveActivityRange(flags, activityFixedNow)
	if err != nil {
		t.Fatalf("resolveActivityRange: %v", err)
	}
	got := filterActivity(activityFixtureEvents(), rng, flags)
	wantIDs := []string{"e4", "e5"}
	gotIDs := eventIDs(got)
	if !equalStrings(gotIDs, wantIDs) {
		t.Errorf("ids: got %v want %v", gotIDs, wantIDs)
	}
}

// TestFilterActivity_KindFilterSingle reduces the fixture to just one kind.
func TestFilterActivity_KindFilterSingle(t *testing.T) {
	flags := activityFlags{kind: "created"}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	wantIDs := []string{"e1", "e3", "e6"}
	if !equalStrings(eventIDs(got), wantIDs) {
		t.Errorf("ids: got %v want %v", eventIDs(got), wantIDs)
	}
}

// TestFilterActivity_KindFilterMulti combines two kinds plus whitespace +
// case noise to confirm the parser is forgiving.
func TestFilterActivity_KindFilterMulti(t *testing.T) {
	flags := activityFlags{kind: "created, CLOSED"}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	wantIDs := []string{"e1", "e3", "e4", "e6"}
	if !equalStrings(eventIDs(got), wantIDs) {
		t.Errorf("ids: got %v want %v", eventIDs(got), wantIDs)
	}
}

// TestFilterActivity_KindUnknownDropped — unknown kind name parses to an
// empty kind set and yields zero events, matching the --source convention.
func TestFilterActivity_KindUnknownDropped(t *testing.T) {
	flags := activityFlags{kind: "totally-bogus"}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	if len(got) != 0 {
		t.Errorf("expected 0 events for unknown kind, got %d", len(got))
	}
}

// TestFilterActivity_BeadFilter narrows to one bead's stream.
func TestFilterActivity_BeadFilter(t *testing.T) {
	flags := activityFlags{bead: "bt-aaa"}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	wantIDs := []string{"e1", "e2", "e4"}
	if !equalStrings(eventIDs(got), wantIDs) {
		t.Errorf("ids: got %v want %v", eventIDs(got), wantIDs)
	}
}

// TestFilterActivity_RepoFilter narrows by repo prefix.
func TestFilterActivity_RepoFilter(t *testing.T) {
	flags := activityFlags{repo: "bd"}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	wantIDs := []string{"e3", "e5"}
	if !equalStrings(eventIDs(got), wantIDs) {
		t.Errorf("ids: got %v want %v", eventIDs(got), wantIDs)
	}
}

// TestFilterActivity_ActorFilter — events without an actor are excluded
// when --actor is set, even if the value matches "" literally; we want
// "set" to mean "drop unset".
func TestFilterActivity_ActorFilter(t *testing.T) {
	flags := activityFlags{actor: "sms"}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	wantIDs := []string{"e1", "e2", "e4", "e6", "e7"}
	if !equalStrings(eventIDs(got), wantIDs) {
		t.Errorf("ids: got %v want %v", eventIDs(got), wantIDs)
	}
}

// TestFilterActivity_Reverse confirms the reverse flag flips the output
// order without otherwise touching the filter result.
func TestFilterActivity_Reverse(t *testing.T) {
	flags := activityFlags{reverse: true, kind: "created"}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	wantIDs := []string{"e6", "e3", "e1"}
	if !equalStrings(eventIDs(got), wantIDs) {
		t.Errorf("ids: got %v want %v", eventIDs(got), wantIDs)
	}
}

// TestFilterActivity_Limit caps the count after sorting; reverse + limit
// should give "newest N" semantics.
func TestFilterActivity_Limit(t *testing.T) {
	flags := activityFlags{limit: 2, reverse: true}
	got := filterActivity(activityFixtureEvents(), parsedRange{}, flags)
	if len(got) != 2 {
		t.Fatalf("len: got %d want 2", len(got))
	}
	wantIDs := []string{"e8", "e7"}
	if !equalStrings(eventIDs(got), wantIDs) {
		t.Errorf("ids: got %v want %v", eventIDs(got), wantIDs)
	}
}

// TestFilterActivity_Empty — passing a window that excludes every event
// returns nil (which projects to []) rather than a partial slice.
func TestFilterActivity_Empty(t *testing.T) {
	flags := activityFlags{since: "2030-01-01"}
	rng, err := resolveActivityRange(flags, activityFixedNow)
	if err != nil {
		t.Fatalf("resolveActivityRange: %v", err)
	}
	got := filterActivity(activityFixtureEvents(), rng, flags)
	if len(got) != 0 {
		t.Errorf("expected 0 events for far-future since, got %d", len(got))
	}
}

// TestProjectActivityCompact_EmptySliceNotNil locks the JSON wire-form
// guarantee: empty result projects to []CompactActivityEvent{} so json
// emits `[]` rather than `null`. Robot mode contract: empty result is `[]`.
func TestProjectActivityCompact_EmptySliceNotNil(t *testing.T) {
	got := projectActivityCompact(nil)
	if got == nil {
		t.Fatalf("got nil; want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("len: got %d want 0", len(got))
	}
	// Round-trip through JSON to confirm wire form.
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("wire form: got %q want %q", string(b), "[]")
	}
}

// TestLoadPersisted_MissingFileEmpty confirms the contract we depend on:
// missing events.jsonl is empty, not error. (Sanity-checks the events
// package contract from the consumer side so a future regression there
// surfaces as a failed activity test, not a silent CLI error.)
func TestLoadPersisted_MissingFileEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := events.LoadPersisted(filepath.Join(dir, "nonexistent.jsonl"), 0)
	if err != nil {
		t.Fatalf("LoadPersisted: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice for missing file, got %d events", len(got))
	}
}

// TestRobotActivity_Binary_GlobalIsNoOp exercises the full CLI path with
// --global passed; events.jsonl is per-user not per-project so --global
// must be silently accepted, not error. Tests the integration contract
// rather than the filter math (which the unit tests above cover).
func TestRobotActivity_Binary_GlobalIsNoOp(t *testing.T) {
	exe := buildTestBinary(t)

	// Override HOME so events.DefaultPersistPath points to a tempdir.
	// The events file does not need to exist — missing-file → [] is part
	// of the contract.
	tmpHome := t.TempDir()
	cmd := exec.Command(exe, "robot", "activity", "--global", "--limit=10")
	cmd.Env = append(os.Environ(), envHome(tmpHome)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bt robot activity --global: %v\nout=%s", err, out)
	}

	var resp struct {
		Schema string                 `json:"schema"`
		Count  int                    `json:"count"`
		Events []CompactActivityEvent `json:"events"`
		Query  activityQuery          `json:"query"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal: %v\nout=%s", err, out)
	}
	if resp.Count != 0 {
		t.Errorf("count: got %d want 0", resp.Count)
	}
	if resp.Schema != ActivitySchemaV1 {
		t.Errorf("schema: got %q want %q", resp.Schema, ActivitySchemaV1)
	}
	if resp.Events == nil {
		// Wire form: should serialize as `[]` not `null`. UnmarshaledNil
		// is acceptable for `[]` if Go's decoder zero-values it; sanity
		// check the raw bytes too.
		if !strings.Contains(string(out), `"events":[]`) {
			t.Errorf("wire form: expected events:[]; got %s", string(out))
		}
	}
}

// TestRobotActivity_Binary_FullShape — --shape full omits the schema marker
// and includes the wider Event struct (CommentAt/Source/Dismissed reachable
// via the JSON tags on the original struct).
func TestRobotActivity_Binary_FullShape(t *testing.T) {
	exe := buildTestBinary(t)

	// Seed a file with one event so we can assert the wire shape.
	tmpHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpHome, ".bt"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	line := `{"ID":"abc","Kind":0,"BeadID":"bt-aaa","Repo":"bt","Title":"hi","Summary":"summary","Actor":"sms","At":"2026-05-01T10:00:00Z","CommentAt":"0001-01-01T00:00:00Z","Source":0,"Dismissed":false}` + "\n"
	if err := os.WriteFile(filepath.Join(tmpHome, ".bt", "events.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

	cmd := exec.Command(exe, "robot", "activity", "--shape=full", "--limit=10")
	cmd.Env = append(os.Environ(), envHome(tmpHome)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bt robot activity --shape=full: %v\nout=%s", err, out)
	}

	var resp struct {
		Schema string `json:"schema"`
		Count  int    `json:"count"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal: %v\nout=%s", err, out)
	}
	if resp.Count != 1 {
		t.Errorf("count: got %d want 1", resp.Count)
	}
	if resp.Schema != "" {
		t.Errorf("full shape should omit schema marker, got %q", resp.Schema)
	}
}

// TestRobotActivity_Binary_TOON confirms --format toon produces TOON output
// (smoke-level only — TOON encoding is exercised in main_robot_test.go for
// the encoder path; we just want to know our subcommand routes through it).
//
// Skips when the `tru` external binary isn't on PATH — matching the pattern
// in main_robot_test.go's TestTOONOutputFormat. The toonRobotEncoder falls
// back to JSON in that case, which would silently pass a naive assertion.
func TestRobotActivity_Binary_TOON(t *testing.T) {
	if _, err := exec.LookPath("tru"); err != nil {
		t.Skip("tru binary not available, skipping TOON test")
	}
	exe := buildTestBinary(t)
	tmpHome := t.TempDir()

	cmd := exec.Command(exe, "robot", "activity", "--format=toon", "--limit=10")
	cmd.Env = append(os.Environ(), envHome(tmpHome)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bt robot activity --format=toon: %v\nout=%s", err, out)
	}
	// TOON output is not valid JSON. If we got back JSON, the format flag
	// didn't take effect.
	if json.Valid(out) {
		t.Errorf("expected TOON (non-JSON) output, got valid JSON: %s", out)
	}
	if !strings.Contains(string(out), "count") {
		t.Errorf("TOON output missing 'count' key: %s", out)
	}
}

// envHome returns a HOME (or USERPROFILE on Windows) override pair so child
// processes resolve events.DefaultPersistPath() into our tempdir. We set
// both to be portable across platforms — events package uses os.UserHomeDir
// which checks USERPROFILE first on Windows.
func envHome(dir string) []string {
	return []string{
		"HOME=" + dir,
		"USERPROFILE=" + dir,
	}
}

// eventIDs is a tiny helper to compare slices of events by their ID field
// without writing the same projection in every test.
func eventIDs(in []events.Event) []string {
	out := make([]string, len(in))
	for i, e := range in {
		out[i] = e.ID
	}
	return out
}

// equalStrings is order-sensitive slice equality. Used because filter
// output order is part of what the tests verify.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
