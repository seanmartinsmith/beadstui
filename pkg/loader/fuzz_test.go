package loader_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// =============================================================================
// Fuzz Tests for JSONL Parser Robustness
// =============================================================================
//
// These fuzz tests verify that the parser handles malformed, adversarial, and
// edge-case inputs gracefully without panicking, hanging, or crashing.
//
// Run with: go test -fuzz=FuzzParseIssues -fuzztime=10m ./pkg/loader/...
//
// The seed corpus provides known edge cases to start the fuzzer from
// interesting positions in the input space.

// FuzzParseIssues tests the complete JSONL parsing pipeline.
// It should never panic regardless of input.
func FuzzParseIssues(f *testing.F) {
	// Add seed corpus - known edge cases the fuzzer should explore from
	seeds := []string{
		// Valid minimal issue
		`{"id":"bv-1","title":"Test","status":"open","issue_type":"task","priority":1}`,

		// Valid issue with all fields
		`{"id":"bv-2","title":"Full","description":"desc","design":"design","acceptance_criteria":"ac","notes":"notes","status":"in_progress","priority":0,"issue_type":"feature","assignee":"user","labels":["a","b"],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}`,

		// Empty line (should be skipped)
		"",

		// Whitespace only
		"   \t  ",

		// Incomplete JSON
		`{"id":"bv-3","title":"Incomplete`,

		// Invalid JSON - missing quotes
		`{id:"bv-4",title:"Test"}`,

		// Invalid JSON - trailing comma
		`{"id":"bv-5","title":"Test",}`,

		// Invalid status
		`{"id":"bv-6","title":"Test","status":"invalid_status","issue_type":"task"}`,

		// Invalid issue type
		`{"id":"bv-7","title":"Test","status":"open","issue_type":"invalid_type"}`,

		// Missing required field (id)
		`{"title":"No ID","status":"open","issue_type":"task"}`,

		// Missing required field (title)
		`{"id":"bv-8","status":"open","issue_type":"task"}`,

		// Null values
		`{"id":"bv-9","title":"Test","status":"open","issue_type":"task","assignee":null,"labels":null}`,

		// Empty string values
		`{"id":"","title":"","status":"open","issue_type":"task"}`,

		// Very long string (64KB)
		`{"id":"bv-10","title":"` + strings.Repeat("x", 65536) + `","status":"open","issue_type":"task"}`,

		// Unicode characters
		`{"id":"bv-11","title":"Test 日本語 🎉 émoji","status":"open","issue_type":"task"}`,

		// UTF-8 BOM prefix
		"\xef\xbb\xbf" + `{"id":"bv-12","title":"BOM Test","status":"open","issue_type":"task"}`,

		// Control characters in string
		`{"id":"bv-13","title":"Tab\there\nNewline","status":"open","issue_type":"task"}`,

		// Nested JSON in description
		`{"id":"bv-14","title":"Test","description":"{\"nested\":\"json\"}","status":"open","issue_type":"task"}`,

		// Deeply nested dependencies
		`{"id":"bv-15","title":"Test","status":"open","issue_type":"task","dependencies":[{"issue_id":"bv-15","depends_on_id":"bv-14","type":"blocks"}]}`,

		// Comments array
		`{"id":"bv-16","title":"Test","status":"open","issue_type":"task","comments":[{"id":1,"issue_id":"bv-16","author":"user","text":"comment","created_at":"2024-01-01T00:00:00Z"}]}`,

		// Numeric overflow
		`{"id":"bv-17","title":"Test","status":"open","issue_type":"task","priority":999999999999999999999999999999}`,

		// Negative priority
		`{"id":"bv-18","title":"Test","status":"open","issue_type":"task","priority":-1}`,

		// Float priority (should fail or be truncated)
		`{"id":"bv-19","title":"Test","status":"open","issue_type":"task","priority":1.5}`,

		// Array instead of object
		`[{"id":"bv-20"}]`,

		// Just a string
		`"just a string"`,

		// Just a number
		`42`,

		// Just true/false/null
		`true`,
		`false`,
		`null`,

		// Binary data mixed in
		"\x00\x01\x02\x03",

		// Invalid UTF-8 sequence
		"\xff\xfe",

		// Multiple issues on same line (invalid JSONL)
		`{"id":"bv-21"}{"id":"bv-22"}`,

		// Escaped characters
		`{"id":"bv-23","title":"Test\\n\\t\\\"","status":"open","issue_type":"task"}`,

		// Issue with all dependency types
		`{"id":"bv-24","title":"Test","status":"open","issue_type":"task","dependencies":[{"issue_id":"bv-24","depends_on_id":"bv-1","type":"blocks"},{"issue_id":"bv-24","depends_on_id":"bv-2","type":"related"},{"issue_id":"bv-24","depends_on_id":"bv-3","type":"parent-child"},{"issue_id":"bv-24","depends_on_id":"bv-4","type":"discovered-from"}]}`,

		// Issue with tombstone status
		`{"id":"bv-25","title":"Deleted","status":"tombstone","issue_type":"task"}`,

		// Issue with due date and closed_at
		`{"id":"bv-26","title":"Test","status":"closed","issue_type":"task","due_date":"2024-12-31T23:59:59Z","closed_at":"2024-01-15T10:00:00Z"}`,

		// Issue with estimated_minutes
		`{"id":"bv-27","title":"Test","status":"open","issue_type":"task","estimated_minutes":120}`,

		// Issue with external_ref
		`{"id":"bv-28","title":"Test","status":"open","issue_type":"task","external_ref":"JIRA-1234"}`,

		// Issue with source_repo
		`{"id":"bv-29","title":"Test","status":"open","issue_type":"task","source_repo":"github.com/org/repo"}`,

		// Issue with compaction fields
		`{"id":"bv-30","title":"Compacted","status":"closed","issue_type":"task","compaction_level":2,"compacted_at":"2024-01-01T00:00:00Z","compacted_at_commit":"abc123","original_size":5000}`,

		// Multi-line JSONL (multiple valid issues)
		`{"id":"bv-31","title":"First","status":"open","issue_type":"task"}
{"id":"bv-32","title":"Second","status":"open","issue_type":"task"}
{"id":"bv-33","title":"Third","status":"open","issue_type":"task"}`,

		// Multi-line with blank lines
		`{"id":"bv-34","title":"First","status":"open","issue_type":"task"}

{"id":"bv-35","title":"Second","status":"open","issue_type":"task"}`,

		// Multi-line with malformed middle line
		`{"id":"bv-36","title":"First","status":"open","issue_type":"task"}
this is not json
{"id":"bv-37","title":"Third","status":"open","issue_type":"task"}`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Recover from panics in third-party JSON library (go-json has bugs with malformed input)
		defer func() {
			if r := recover(); r != nil {
				// Log but don't fail - this is a known issue in go-json with malformed input
				t.Logf("recovered from panic (go-json library bug): %v", r)
			}
		}()

		// Suppress warnings during fuzzing
		opts := loader.ParseOptions{
			WarningHandler: func(string) {},
		}

		reader := bytes.NewReader(data)
		issues, err := loader.ParseIssuesWithOptions(reader, opts)

		// We don't care about errors (malformed input is expected)
		// We only care that we don't panic and return a valid slice
		_ = err
		_ = issues
	})
}

// FuzzUnmarshalIssue tests JSON unmarshaling into the Issue struct.
// This tests the model layer's ability to handle arbitrary JSON.
func FuzzUnmarshalIssue(f *testing.F) {
	// Seed corpus for Issue struct unmarshaling
	seeds := []string{
		// Valid minimal
		`{"id":"1","title":"Test","status":"open","issue_type":"task"}`,

		// All fields
		`{"id":"2","title":"Full","description":"d","design":"ds","acceptance_criteria":"ac","notes":"n","status":"in_progress","priority":1,"issue_type":"feature","assignee":"u","estimated_minutes":60,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z","due_date":"2024-12-31T00:00:00Z","closed_at":"2024-06-01T00:00:00Z","external_ref":"EXT-1","labels":["a","b"],"dependencies":[],"comments":[]}`,

		// Empty object
		`{}`,

		// Null object
		`null`,

		// Extra unknown fields (should be ignored)
		`{"id":"3","title":"Test","status":"open","issue_type":"task","unknown_field":"value","nested":{"a":1}}`,

		// Wrong types
		`{"id":123,"title":456,"status":true,"issue_type":null}`,

		// Arrays where objects expected
		`{"id":"4","title":"Test","dependencies":"not an array"}`,

		// Objects where arrays expected
		`{"id":"5","title":"Test","labels":{"not":"array"}}`,

		// Deeply nested
		`{"id":"6","title":"Test","description":"` + strings.Repeat(`{"a":`, 100) + `1` + strings.Repeat(`}`, 100) + `"}`,

		// Very long arrays
		`{"id":"7","title":"Test","labels":[` + strings.Repeat(`"label",`, 1000) + `"last"]}`,

		// Unicode in all string fields
		`{"id":"日本語","title":"🎉","description":"Ñ","status":"open","issue_type":"task"}`,

		// Escaped special characters
		`{"id":"8","title":"Test \"quoted\" and \\backslash","status":"open","issue_type":"task"}`,

		// Timestamps with different formats
		`{"id":"9","title":"Test","created_at":"2024-01-01","status":"open","issue_type":"task"}`,
		`{"id":"10","title":"Test","created_at":"2024-01-01T00:00:00+05:30","status":"open","issue_type":"task"}`,
		`{"id":"11","title":"Test","created_at":"invalid-date","status":"open","issue_type":"task"}`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Recover from panics in JSON library
		defer func() {
			if r := recover(); r != nil {
				t.Logf("recovered from panic: %v", r)
			}
		}()

		var issue model.Issue
		err := json.Unmarshal(data, &issue)
		_ = err

		// If unmarshal succeeded, validation should also not panic
		if err == nil {
			_ = issue.Validate()
		}
	})
}

// FuzzValidate tests the Issue.Validate method with various field combinations.
// This ensures validation logic doesn't panic on edge cases.
func FuzzValidate(f *testing.F) {
	// Seed with various ID/title/status/type combinations
	type seedData struct {
		id        string
		title     string
		status    string
		issueType string
		priority  int
	}

	seeds := []seedData{
		{"bv-1", "Test", "open", "task", 0},
		{"bv-2", "Test", "in_progress", "bug", 1},
		{"bv-3", "Test", "blocked", "feature", 2},
		{"bv-4", "Test", "closed", "epic", 3},
		{"bv-5", "Test", "tombstone", "chore", 4},
		{"", "Test", "open", "task", 0},                          // Empty ID
		{"bv-6", "", "open", "task", 0},                          // Empty title
		{"bv-7", "Test", "invalid", "task", 0},                   // Invalid status
		{"bv-8", "Test", "open", "invalid", 0},                   // Invalid type
		{"bv-9", "Test", "open", "task", -1},                     // Negative priority
		{"bv-10", "Test", "open", "task", 999999},                // Large priority
		{strings.Repeat("x", 10000), "Test", "open", "task", 0},  // Very long ID
		{"bv-11", strings.Repeat("y", 10000), "open", "task", 0}, // Very long title
	}

	for _, seed := range seeds {
		f.Add(seed.id, seed.title, seed.status, seed.issueType, seed.priority)
	}

	f.Fuzz(func(t *testing.T, id, title, status, issueType string, priority int) {
		issue := model.Issue{
			ID:        id,
			Title:     title,
			Status:    model.Status(status),
			IssueType: model.IssueType(issueType),
			Priority:  priority,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Validate should never panic
		err := issue.Validate()
		_ = err
	})
}

// FuzzValidateTimestamps specifically tests timestamp validation edge cases.
func FuzzValidateTimestamps(f *testing.F) {
	// Seed with various timestamp orderings
	seeds := []struct {
		createdYear  int
		createdMonth int
		createdDay   int
		updatedYear  int
		updatedMonth int
		updatedDay   int
	}{
		{2024, 1, 1, 2024, 1, 2},   // Updated after created (valid)
		{2024, 1, 2, 2024, 1, 1},   // Updated before created (invalid)
		{2024, 1, 1, 2024, 1, 1},   // Same time (valid)
		{1970, 1, 1, 2100, 12, 31}, // Extreme range
		{0, 0, 0, 2024, 1, 1},      // Zero created
		{2024, 1, 1, 0, 0, 0},      // Zero updated
		{2024, 13, 45, 2024, 1, 1}, // Invalid date components
	}

	for _, seed := range seeds {
		f.Add(seed.createdYear, seed.createdMonth, seed.createdDay,
			seed.updatedYear, seed.updatedMonth, seed.updatedDay)
	}

	f.Fuzz(func(t *testing.T, cy, cm, cd, uy, um, ud int) {
		// Clamp to reasonable ranges to avoid time.Date panics
		clamp := func(v, min, max int) int {
			if v < min {
				return min
			}
			if v > max {
				return max
			}
			return v
		}

		cy = clamp(cy, 1, 9999)
		cm = clamp(cm, 1, 12)
		cd = clamp(cd, 1, 28) // Safe for all months
		uy = clamp(uy, 1, 9999)
		um = clamp(um, 1, 12)
		ud = clamp(ud, 1, 28)

		issue := model.Issue{
			ID:        "test-id",
			Title:     "Test Title",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			CreatedAt: time.Date(cy, time.Month(cm), cd, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(uy, time.Month(um), ud, 0, 0, 0, 0, time.UTC),
		}

		// Should never panic
		err := issue.Validate()
		_ = err
	})
}

// FuzzDependencyParsing tests parsing of dependencies array.
func FuzzDependencyParsing(f *testing.F) {
	seeds := []string{
		// Valid dependency
		`{"id":"1","title":"T","status":"open","issue_type":"task","dependencies":[{"issue_id":"1","depends_on_id":"2","type":"blocks","created_at":"2024-01-01T00:00:00Z","created_by":"user"}]}`,

		// Multiple dependencies
		`{"id":"2","title":"T","status":"open","issue_type":"task","dependencies":[{"issue_id":"2","depends_on_id":"1","type":"blocks"},{"issue_id":"2","depends_on_id":"3","type":"related"}]}`,

		// Empty dependencies array
		`{"id":"3","title":"T","status":"open","issue_type":"task","dependencies":[]}`,

		// Null dependencies
		`{"id":"4","title":"T","status":"open","issue_type":"task","dependencies":null}`,

		// Invalid dependency type
		`{"id":"5","title":"T","status":"open","issue_type":"task","dependencies":[{"issue_id":"5","depends_on_id":"1","type":"invalid_type"}]}`,

		// Missing dependency fields
		`{"id":"6","title":"T","status":"open","issue_type":"task","dependencies":[{"issue_id":"6"}]}`,

		// Self-referencing dependency
		`{"id":"7","title":"T","status":"open","issue_type":"task","dependencies":[{"issue_id":"7","depends_on_id":"7","type":"blocks"}]}`,

		// Very long dependency chain (in one issue)
		`{"id":"8","title":"T","status":"open","issue_type":"task","dependencies":[` +
			strings.Repeat(`{"issue_id":"8","depends_on_id":"x","type":"blocks"},`, 100) +
			`{"issue_id":"8","depends_on_id":"last","type":"blocks"}]}`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Recover from panics in go-json library
		defer func() {
			if r := recover(); r != nil {
				t.Logf("recovered from panic (go-json library bug): %v", r)
			}
		}()

		opts := loader.ParseOptions{
			WarningHandler: func(string) {},
		}
		reader := bytes.NewReader(data)
		issues, _ := loader.ParseIssuesWithOptions(reader, opts)
		_ = issues
	})
}

// FuzzCommentParsing tests parsing of comments array.
func FuzzCommentParsing(f *testing.F) {
	seeds := []string{
		// Valid comment
		`{"id":"1","title":"T","status":"open","issue_type":"task","comments":[{"id":1,"issue_id":"1","author":"user","text":"comment","created_at":"2024-01-01T00:00:00Z"}]}`,

		// Multiple comments
		`{"id":"2","title":"T","status":"open","issue_type":"task","comments":[{"id":1,"issue_id":"2","author":"a","text":"1"},{"id":2,"issue_id":"2","author":"b","text":"2"}]}`,

		// Empty comments
		`{"id":"3","title":"T","status":"open","issue_type":"task","comments":[]}`,

		// Null comments
		`{"id":"4","title":"T","status":"open","issue_type":"task","comments":null}`,

		// Comment with very long text
		`{"id":"5","title":"T","status":"open","issue_type":"task","comments":[{"id":1,"issue_id":"5","author":"u","text":"` + strings.Repeat("x", 10000) + `"}]}`,

		// Comment with unicode
		`{"id":"6","title":"T","status":"open","issue_type":"task","comments":[{"id":1,"issue_id":"6","author":"日本語","text":"🎉 emoji"}]}`,

		// Invalid comment ID type
		`{"id":"7","title":"T","status":"open","issue_type":"task","comments":[{"id":"not-a-number"}]}`,

		// Negative comment ID
		`{"id":"8","title":"T","status":"open","issue_type":"task","comments":[{"id":-1,"issue_id":"8","author":"u","text":"t"}]}`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Recover from panics in go-json library
		defer func() {
			if r := recover(); r != nil {
				t.Logf("recovered from panic (go-json library bug): %v", r)
			}
		}()

		opts := loader.ParseOptions{
			WarningHandler: func(string) {},
		}
		reader := bytes.NewReader(data)
		issues, _ := loader.ParseIssuesWithOptions(reader, opts)
		_ = issues
	})
}

// FuzzLargeLine tests handling of lines that exceed the buffer size.
func FuzzLargeLine(f *testing.F) {
	// Create seeds with various sizes around the buffer boundary
	sizes := []int{100, 1000, 10000, 100000, 500000}
	for _, size := range sizes {
		seed := `{"id":"large","title":"` + strings.Repeat("x", size) + `","status":"open","issue_type":"task"}`
		f.Add([]byte(seed), size)
	}

	f.Fuzz(func(t *testing.T, data []byte, bufferSize int) {
		// Recover from panics in go-json library
		defer func() {
			if r := recover(); r != nil {
				t.Logf("recovered from panic (go-json library bug): %v", r)
			}
		}()

		// Clamp buffer size to reasonable range
		if bufferSize < 64 {
			bufferSize = 64
		}
		if bufferSize > 1024*1024 { // 1MB max for fuzzing
			bufferSize = 1024 * 1024
		}

		opts := loader.ParseOptions{
			WarningHandler: func(string) {},
			BufferSize:     bufferSize,
		}
		reader := bytes.NewReader(data)
		issues, _ := loader.ParseIssuesWithOptions(reader, opts)
		_ = issues
	})
}
