// Package bdcmd is the canonical builder for the bd (beads CLI) command lines
// bt shells out. It is the single source of truth for bd argv construction,
// shared by three consumers so none of them re-derives the same flags:
//
//   - robot actions[] emit (cmd/bt): every recommendation ships an exact,
//     ready-to-exec bd argv with declared preconditions (bt-s5zgk.4/.5).
//   - the TUI write executor (pkg/ui): builds the argv it hands to
//     internal/bdexec (bt-oiaj.10's claim slice adopts this later).
//   - the receipts pane (pkg/ui): renders what was (or would be) run.
//
// One package, contract-tested once, instead of three implementations that
// drift (bt-j9r2o, bt-s5zgk.1).
//
// # Windows cp1252 safety by construction
//
// Free text (close reasons, comment bodies, descriptions, designs) is NEVER
// placed inline in argv. On Windows, bd invoked through bash routes non-ASCII
// bytes through cp1252 and corrupts em-dashes / smart quotes / Unicode
// (AGENTS.md rule 8). Every builder that carries free text takes a FILE PATH,
// not a content string - there is deliberately no inline-free-text builder to
// call, so the hazard is unrepresentable, not merely discouraged. Content
// reaches bd only through a file the caller has already written (materialize
// in-memory content with FreeText.Materialize). The single-line structured
// fields (status, priority, title, assignee) go inline because bd exposes no
// file-based flag for them; only the enumerated free-text BODY fields have a
// file route, and those are the fields this guarantee covers.
//
// # Determinism
//
// Every builder emits a fixed-order argv (Command.Args), excluding the "bd"
// program name so the slice drops straight into bdexec.Run(ctx, dir, Args...);
// Command.Argv and Command.String prepend "bd" for display and receipts.
package bdcmd

import (
	"fmt"
	"os"
	"strings"
)

// Program is the bd executable name that Command.Argv / Command.String
// prepend. Command.Args deliberately omits it (bdexec.Run adds it back).
const Program = "bd"

// Verb enumerates the bd operations bdcmd can construct.
type Verb int

const (
	VerbClaim    Verb = iota // update <id> --claim
	VerbClose                // close <id> --reason-file <file>
	VerbUpdate               // update <id> <flag> <value|file>
	VerbComment              // comments add <id> -f <file>
	VerbDoltPush             // dolt push
	VerbDoltPull             // dolt pull
)

// String renders the verb as a stable machine token (used in labels/traces).
func (v Verb) String() string {
	switch v {
	case VerbClaim:
		return "claim"
	case VerbClose:
		return "close"
	case VerbUpdate:
		return "update"
	case VerbComment:
		return "comment"
	case VerbDoltPush:
		return "dolt-push"
	case VerbDoltPull:
		return "dolt-pull"
	default:
		return "unknown"
	}
}

// Field identifies an editable bead field for the update verb. Fields split
// into two transport classes: inline single-line fields (status, priority,
// title, assignee) built via Update, and free-text body fields (description,
// design) built via UpdateFile so their content routes through a file
// (cp1252 safety). Each builder refuses the other class's fields.
type Field int

const (
	FieldStatus      Field = iota // --status <value>
	FieldPriority                 // -p <value>
	FieldTitle                    // --title <value>
	FieldAssignee                 // -a <value>
	FieldDescription              // --body-file <file> (free text)
	FieldDesign                   // --design-file <file> (free text)
)

// fieldSpec records the bd flags and human name for one Field. Exactly one of
// inlineFlag / fileFlag is set: inlineFlag for single-line fields whose value
// is safe to pass in argv, fileFlag for free-text body fields whose content
// must route through a file.
type fieldSpec struct {
	name       string
	inlineFlag string
	fileFlag   string
}

var fieldSpecs = map[Field]fieldSpec{
	FieldStatus:      {name: "status", inlineFlag: "--status"},
	FieldPriority:    {name: "priority", inlineFlag: "-p"},
	FieldTitle:       {name: "title", inlineFlag: "--title"},
	FieldAssignee:    {name: "assignee", inlineFlag: "-a"},
	FieldDescription: {name: "description", fileFlag: "--body-file"},
	FieldDesign:      {name: "design", fileFlag: "--design-file"},
}

// String returns the human field name, or "unknown" for an unrecognized value.
func (f Field) String() string {
	if spec, ok := fieldSpecs[f]; ok {
		return spec.name
	}
	return "unknown"
}

// Precondition is a declared condition that should hold before a Command is
// run. Key is a stable machine token for programmatic checks (e.g. robot
// actions[] consumers); Description is the human-readable form. Preconditions
// are advisory metadata - the builder always produces valid argv; bd remains
// the source of truth for whether the operation is actually permitted.
type Precondition struct {
	Key         string
	Description string
}

// Command is a constructed bd invocation: its verb, the deterministic argv
// (without the "bd" program name), a human-readable label, and the
// preconditions a consumer may surface or check.
type Command struct {
	Verb          Verb
	Args          []string
	Label         string
	Preconditions []Precondition
}

// Argv returns the full argv including the "bd" program name, ready for
// display or a robot action. It always returns a fresh slice.
func (c Command) Argv() []string {
	out := make([]string, 0, len(c.Args)+1)
	out = append(out, Program)
	return append(out, c.Args...)
}

// String renders the invocation as a single command line for a trace log,
// receipts entry, or robot action (e.g. "bd update bt-1 --claim").
func (c Command) String() string {
	return strings.Join(c.Argv(), " ")
}

// FreeText is a body of user- or agent-authored free text (a close reason,
// comment body, or field description) that must reach bd through a file rather
// than inline argv (cp1252 safety - see the package doc). Materialize is the
// only bridge from content to a bd command line.
type FreeText string

// Materialize writes the free text to a temp file under dir (dir == "" uses
// the OS temp dir) and returns the file path plus a cleanup func that removes
// it. Callers pass the returned path to Close / AddComment / UpdateFile. The
// cleanup func is idempotent and safe to defer. This is the sanctioned bridge
// for callers holding content in memory; a caller that already wrote its own
// file (e.g. the TUI long-form editor) passes that path directly instead.
func (t FreeText) Materialize(dir string) (path string, cleanup func() error, err error) {
	f, err := os.CreateTemp(dir, "bdcmd-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("bdcmd: create free-text temp file: %w", err)
	}
	path = f.Name()
	cleanup = func() error {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("bdcmd: remove free-text temp file %s: %w", path, rmErr)
		}
		return nil
	}
	if _, err = f.Write([]byte(t)); err != nil {
		_ = f.Close()
		_ = cleanup()
		return "", nil, fmt.Errorf("bdcmd: write free text to %s: %w", path, err)
	}
	if err = f.Close(); err != nil {
		_ = cleanup()
		return "", nil, fmt.Errorf("bdcmd: close free-text temp file %s: %w", path, err)
	}
	return path, cleanup, nil
}

// preconditions returns a fresh Precondition slice for verb. A fresh slice per
// call keeps every Command's metadata independent (no shared mutable state).
func preconditions(v Verb) []Precondition {
	switch v {
	case VerbClaim:
		return []Precondition{{
			Key:         "issue.claimable",
			Description: "issue must be open and unassigned, or already assigned to you",
		}}
	case VerbClose:
		return []Precondition{{
			Key:         "issue.open",
			Description: "issue must not already be closed",
		}}
	case VerbUpdate, VerbComment:
		return []Precondition{{
			Key:         "issue.exists",
			Description: "issue must exist",
		}}
	case VerbDoltPush:
		return []Precondition{{
			Key:         "dolt.pull-first",
			Description: "run bd dolt pull before push to avoid a non-fast-forward reject",
		}}
	case VerbDoltPull:
		return nil
	default:
		return nil
	}
}

// requireID rejects an empty (or whitespace-only) issue id.
func requireID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("bdcmd: empty issue id")
	}
	return nil
}

// Claim builds `bd update <id> --claim`.
func Claim(id string) (Command, error) {
	if err := requireID(id); err != nil {
		return Command{}, err
	}
	return Command{
		Verb:          VerbClaim,
		Args:          []string{"update", id, "--claim"},
		Label:         fmt.Sprintf("Claim %s", id),
		Preconditions: preconditions(VerbClaim),
	}, nil
}

// Close builds `bd close <id> --reason-file <reasonFile>`. The reason is free
// text and must already be on disk at reasonFile (write it with
// FreeText.Materialize, or pass a file the caller already owns). There is no
// inline-reason builder: inline non-ASCII corrupts through cp1252 on Windows.
func Close(id, reasonFile string) (Command, error) {
	if err := requireID(id); err != nil {
		return Command{}, err
	}
	if strings.TrimSpace(reasonFile) == "" {
		return Command{}, fmt.Errorf("bdcmd: close %s: empty reason-file path (materialize the reason via FreeText.Materialize; inline reasons are unrepresentable for cp1252 safety)", id)
	}
	return Command{
		Verb:          VerbClose,
		Args:          []string{"close", id, "--reason-file", reasonFile},
		Label:         fmt.Sprintf("Close %s", id),
		Preconditions: preconditions(VerbClose),
	}, nil
}

// Update builds `bd update <id> <flag> <value>` for an inline single-line
// field (status, priority, title, assignee). It refuses the free-text body
// fields (description, design) with an error directing the caller to
// UpdateFile, so free-text content can never be passed inline.
func Update(id string, field Field, value string) (Command, error) {
	if err := requireID(id); err != nil {
		return Command{}, err
	}
	spec, ok := fieldSpecs[field]
	if !ok {
		return Command{}, fmt.Errorf("bdcmd: update %s: unknown field %d", id, field)
	}
	if spec.inlineFlag == "" {
		return Command{}, fmt.Errorf("bdcmd: update %s: %s is a free-text field - use UpdateFile so the content routes through a file (cp1252 safety)", id, spec.name)
	}
	if strings.TrimSpace(value) == "" {
		return Command{}, fmt.Errorf("bdcmd: update %s %s: empty value", id, spec.name)
	}
	return Command{
		Verb:          VerbUpdate,
		Args:          []string{"update", id, spec.inlineFlag, value},
		Label:         fmt.Sprintf("Update %s %s", id, spec.name),
		Preconditions: preconditions(VerbUpdate),
	}, nil
}

// UpdateFile builds `bd update <id> <file-flag> <file>` for a free-text body
// field (description -> --body-file, design -> --design-file). The content
// must already be on disk at file. It refuses the inline fields with an error
// directing the caller to Update.
func UpdateFile(id string, field Field, file string) (Command, error) {
	if err := requireID(id); err != nil {
		return Command{}, err
	}
	spec, ok := fieldSpecs[field]
	if !ok {
		return Command{}, fmt.Errorf("bdcmd: update-file %s: unknown field %d", id, field)
	}
	if spec.fileFlag == "" {
		return Command{}, fmt.Errorf("bdcmd: update-file %s: %s is an inline field, not free text - use Update", id, spec.name)
	}
	if strings.TrimSpace(file) == "" {
		return Command{}, fmt.Errorf("bdcmd: update-file %s %s: empty file path", id, spec.name)
	}
	return Command{
		Verb:          VerbUpdate,
		Args:          []string{"update", id, spec.fileFlag, file},
		Label:         fmt.Sprintf("Update %s %s", id, spec.name),
		Preconditions: preconditions(VerbUpdate),
	}, nil
}

// AddComment builds `bd comments add <id> -f <bodyFile>`. The comment body is
// free text and must already be on disk at bodyFile (write it with
// FreeText.Materialize, or pass a file the caller already owns).
func AddComment(id, bodyFile string) (Command, error) {
	if err := requireID(id); err != nil {
		return Command{}, err
	}
	if strings.TrimSpace(bodyFile) == "" {
		return Command{}, fmt.Errorf("bdcmd: comment %s: empty body-file path (materialize the body via FreeText.Materialize; inline comment bodies are unrepresentable for cp1252 safety)", id)
	}
	return Command{
		Verb:          VerbComment,
		Args:          []string{"comments", "add", id, "-f", bodyFile},
		Label:         fmt.Sprintf("Comment on %s", id),
		Preconditions: preconditions(VerbComment),
	}, nil
}

// DoltPush builds `bd dolt push`.
func DoltPush() Command {
	return Command{
		Verb:          VerbDoltPush,
		Args:          []string{"dolt", "push"},
		Label:         "Push beads to the Dolt remote",
		Preconditions: preconditions(VerbDoltPush),
	}
}

// DoltPull builds `bd dolt pull`.
func DoltPull() Command {
	return Command{
		Verb:          VerbDoltPull,
		Args:          []string{"dolt", "pull"},
		Label:         "Pull beads from the Dolt remote",
		Preconditions: preconditions(VerbDoltPull),
	}
}
