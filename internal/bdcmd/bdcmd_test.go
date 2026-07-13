package bdcmd

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestBuilders is the table-driven contract for every supported verb: exact
// argv (without and with the "bd" program name), verb tag, label, and that a
// precondition set is present where one is defined.
func TestBuilders(t *testing.T) {
	cases := []struct {
		name      string
		build     func() (Command, error)
		wantArgs  []string
		wantVerb  Verb
		wantLabel string
		wantPre   bool // expect at least one precondition
	}{
		{
			name:      "claim",
			build:     func() (Command, error) { return Claim("bt-1") },
			wantArgs:  []string{"update", "bt-1", "--claim"},
			wantVerb:  VerbClaim,
			wantLabel: "Claim bt-1",
			wantPre:   true,
		},
		{
			name:      "close",
			build:     func() (Command, error) { return Close("bt-2", "/tmp/reason.txt") },
			wantArgs:  []string{"close", "bt-2", "--reason-file", "/tmp/reason.txt"},
			wantVerb:  VerbClose,
			wantLabel: "Close bt-2",
			wantPre:   true,
		},
		{
			name:      "update-status",
			build:     func() (Command, error) { return Update("bt-3", FieldStatus, "in_progress") },
			wantArgs:  []string{"update", "bt-3", "--status", "in_progress"},
			wantVerb:  VerbUpdate,
			wantLabel: "Update bt-3 status",
			wantPre:   true,
		},
		{
			name:      "update-priority",
			build:     func() (Command, error) { return Update("bt-3", FieldPriority, "1") },
			wantArgs:  []string{"update", "bt-3", "-p", "1"},
			wantVerb:  VerbUpdate,
			wantLabel: "Update bt-3 priority",
			wantPre:   true,
		},
		{
			name:      "update-title",
			build:     func() (Command, error) { return Update("bt-3", FieldTitle, "new title") },
			wantArgs:  []string{"update", "bt-3", "--title", "new title"},
			wantVerb:  VerbUpdate,
			wantLabel: "Update bt-3 title",
			wantPre:   true,
		},
		{
			name:      "update-assignee",
			build:     func() (Command, error) { return Update("bt-3", FieldAssignee, "alice") },
			wantArgs:  []string{"update", "bt-3", "-a", "alice"},
			wantVerb:  VerbUpdate,
			wantLabel: "Update bt-3 assignee",
			wantPre:   true,
		},
		{
			name:      "update-description-file",
			build:     func() (Command, error) { return UpdateFile("bt-4", FieldDescription, "/tmp/body.txt") },
			wantArgs:  []string{"update", "bt-4", "--body-file", "/tmp/body.txt"},
			wantVerb:  VerbUpdate,
			wantLabel: "Update bt-4 description",
			wantPre:   true,
		},
		{
			name:      "update-design-file",
			build:     func() (Command, error) { return UpdateFile("bt-4", FieldDesign, "/tmp/design.txt") },
			wantArgs:  []string{"update", "bt-4", "--design-file", "/tmp/design.txt"},
			wantVerb:  VerbUpdate,
			wantLabel: "Update bt-4 design",
			wantPre:   true,
		},
		{
			name:      "comment",
			build:     func() (Command, error) { return AddComment("bt-5", "/tmp/comment.txt") },
			wantArgs:  []string{"comments", "add", "bt-5", "-f", "/tmp/comment.txt"},
			wantVerb:  VerbComment,
			wantLabel: "Comment on bt-5",
			wantPre:   true,
		},
		{
			name:      "dolt-push",
			build:     func() (Command, error) { return DoltPush(), nil },
			wantArgs:  []string{"dolt", "push"},
			wantVerb:  VerbDoltPush,
			wantLabel: "Push beads to the Dolt remote",
			wantPre:   true,
		},
		{
			name:      "dolt-pull",
			build:     func() (Command, error) { return DoltPull(), nil },
			wantArgs:  []string{"dolt", "pull"},
			wantVerb:  VerbDoltPull,
			wantLabel: "Pull beads from the Dolt remote",
			wantPre:   false, // pull has no precondition
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := tc.build()
			if err != nil {
				t.Fatalf("build: unexpected error: %v", err)
			}
			if !slices.Equal(cmd.Args, tc.wantArgs) {
				t.Errorf("Args = %v, want %v", cmd.Args, tc.wantArgs)
			}
			if cmd.Verb != tc.wantVerb {
				t.Errorf("Verb = %v, want %v", cmd.Verb, tc.wantVerb)
			}
			if cmd.Label != tc.wantLabel {
				t.Errorf("Label = %q, want %q", cmd.Label, tc.wantLabel)
			}
			// Argv prepends the program name; String is the joined form.
			wantArgv := append([]string{Program}, tc.wantArgs...)
			if !slices.Equal(cmd.Argv(), wantArgv) {
				t.Errorf("Argv() = %v, want %v", cmd.Argv(), wantArgv)
			}
			if got, want := cmd.String(), strings.Join(wantArgv, " "); got != want {
				t.Errorf("String() = %q, want %q", got, want)
			}
			if tc.wantPre && len(cmd.Preconditions) == 0 {
				t.Errorf("expected at least one precondition, got none")
			}
		})
	}
}

// TestArgvExcludesProgramName pins the Args-vs-Argv split: Args must never lead
// with "bd" (so it drops straight into bdexec.Run, which prepends the name),
// while Argv must.
func TestArgvExcludesProgramName(t *testing.T) {
	cmd, err := Claim("bt-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) > 0 && cmd.Args[0] == Program {
		t.Errorf("Args must not include the program name: %v", cmd.Args)
	}
	if got := cmd.Argv(); len(got) == 0 || got[0] != Program {
		t.Errorf("Argv() must lead with %q: %v", Program, got)
	}
}

// nonASCIIBody is a free-text body loaded with exactly the characters that
// corrupt through cp1252 on Windows: an em-dash, smart quotes, accented
// letters, and an emoji.
const nonASCIIBody = "Fixed the em—dash and “smart” quotes; café naïve résumé ☃"

// TestFreeTextNeverInArgv is the cp1252 hazard guard. It builds every verb,
// routing all free text through a materialized file loaded with non-ASCII
// content, and asserts that (1) no non-ASCII byte ever appears in the argv of
// any verb, and (2) the free-text content specifically is absent from argv -
// it lives in the file, whose path (not content) is what reaches the command
// line.
func TestFreeTextNeverInArgv(t *testing.T) {
	dir := t.TempDir()
	path, cleanup, err := FreeText(nonASCIIBody).Materialize(dir)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	// Confirm the routing actually happened: the file holds the content.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized file: %v", err)
	}
	if string(got) != nonASCIIBody {
		t.Fatalf("materialized content = %q, want %q", got, nonASCIIBody)
	}

	// Every verb, with free text routed via the non-ASCII file where relevant.
	builders := []struct {
		name  string
		build func() (Command, error)
	}{
		{"claim", func() (Command, error) { return Claim("bt-1") }},
		{"close", func() (Command, error) { return Close("bt-1", path) }},
		{"update-status", func() (Command, error) { return Update("bt-1", FieldStatus, "in_progress") }},
		{"update-priority", func() (Command, error) { return Update("bt-1", FieldPriority, "0") }},
		{"update-title", func() (Command, error) { return Update("bt-1", FieldTitle, "ascii title") }},
		{"update-assignee", func() (Command, error) { return Update("bt-1", FieldAssignee, "alice") }},
		{"update-description", func() (Command, error) { return UpdateFile("bt-1", FieldDescription, path) }},
		{"update-design", func() (Command, error) { return UpdateFile("bt-1", FieldDesign, path) }},
		{"comment", func() (Command, error) { return AddComment("bt-1", path) }},
		{"dolt-push", func() (Command, error) { return DoltPush(), nil }},
		{"dolt-pull", func() (Command, error) { return DoltPull(), nil }},
	}

	for _, b := range builders {
		t.Run(b.name, func(t *testing.T) {
			cmd, err := b.build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			line := cmd.String()
			if strings.Contains(line, nonASCIIBody) {
				t.Errorf("free-text content leaked into argv: %q", line)
			}
			for i, arg := range cmd.Args {
				if !isASCII(arg) {
					t.Errorf("arg[%d] = %q contains non-ASCII bytes", i, arg)
				}
			}
		})
	}
}

// isASCII reports whether every byte of s is in the 7-bit ASCII range.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// TestBuilderErrors covers the validation rejections: empty ids, empty
// paths/values, and - critically - that the transport-class guards make inline
// free text unrepresentable (Update refuses a body field; UpdateFile refuses an
// inline field).
func TestBuilderErrors(t *testing.T) {
	cases := []struct {
		name  string
		build func() (Command, error)
	}{
		{"claim-empty-id", func() (Command, error) { return Claim("") }},
		{"close-empty-id", func() (Command, error) { return Close("", "/tmp/r.txt") }},
		{"close-empty-file", func() (Command, error) { return Close("bt-1", "  ") }},
		{"update-empty-id", func() (Command, error) { return Update("", FieldStatus, "open") }},
		{"update-empty-value", func() (Command, error) { return Update("bt-1", FieldStatus, "  ") }},
		{"update-unknown-field", func() (Command, error) { return Update("bt-1", Field(99), "x") }},
		{"update-freetext-field-inline", func() (Command, error) { return Update("bt-1", FieldDescription, "x") }},
		{"updatefile-empty-id", func() (Command, error) { return UpdateFile("", FieldDescription, "/tmp/b.txt") }},
		{"updatefile-inline-field", func() (Command, error) { return UpdateFile("bt-1", FieldStatus, "/tmp/b.txt") }},
		{"updatefile-empty-file", func() (Command, error) { return UpdateFile("bt-1", FieldDescription, "") }},
		{"updatefile-unknown-field", func() (Command, error) { return UpdateFile("bt-1", Field(99), "/tmp/b.txt") }},
		{"comment-empty-id", func() (Command, error) { return AddComment("", "/tmp/c.txt") }},
		{"comment-empty-file", func() (Command, error) { return AddComment("bt-1", "") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.build(); err == nil {
				t.Errorf("expected an error, got nil")
			}
		})
	}
}

// TestFreeTextMaterialize verifies the content bridge: the file holds the exact
// bytes, sits under the requested dir, and the cleanup func removes it and is
// idempotent.
func TestFreeTextMaterialize(t *testing.T) {
	content := "line one\nline two with an em—dash\n"
	dir := t.TempDir()

	path, cleanup, err := FreeText(content).Materialize(dir)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Errorf("path %q not under dir %q", path, dir)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}

	if err := cleanup(); err != nil {
		t.Errorf("cleanup: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after cleanup (stat err = %v)", err)
	}
	// Idempotent: a second cleanup on an already-removed file is not an error.
	if err := cleanup(); err != nil {
		t.Errorf("second cleanup: %v", err)
	}
}

// TestVerbString and TestFieldString pin the stable tokens used in labels and
// traces.
func TestVerbString(t *testing.T) {
	cases := map[Verb]string{
		VerbClaim:    "claim",
		VerbClose:    "close",
		VerbUpdate:   "update",
		VerbComment:  "comment",
		VerbDoltPush: "dolt-push",
		VerbDoltPull: "dolt-pull",
		Verb(99):     "unknown",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("Verb(%d).String() = %q, want %q", v, got, want)
		}
	}
}

func TestFieldString(t *testing.T) {
	cases := map[Field]string{
		FieldStatus:      "status",
		FieldPriority:    "priority",
		FieldTitle:       "title",
		FieldAssignee:    "assignee",
		FieldDescription: "description",
		FieldDesign:      "design",
		Field(99):        "unknown",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Errorf("Field(%d).String() = %q, want %q", f, got, want)
		}
	}
}
