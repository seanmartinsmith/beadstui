package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	json "github.com/goccy/go-json"

	toon "github.com/Dicklesworthstone/toon-go"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/version"
)

var robotOutputFormat = "json"
var robotToonEncodeOptions = toon.DefaultEncodeOptions()
var robotShowToonStats bool

// RobotEnvelope is the standard envelope for all robot command outputs.
// All robot outputs MUST include these fields for consistency.
type RobotEnvelope struct {
	GeneratedAt  string `json:"generated_at"`            // RFC3339 timestamp
	DataHash     string `json:"data_hash"`               // Fingerprint of source data
	OutputFormat string `json:"output_format,omitempty"` // "json" or "toon"
	Version      string `json:"version,omitempty"`       // bv version (e.g., "1.0.0")
	// Schema identifies the projection shape carried in the payload, e.g.,
	// "compact.v1". Empty (and omitted from the wire) when the payload is
	// the default full shape, so historical full outputs stay byte-identical.
	Schema string `json:"schema,omitempty"`
	// SigilMode identifies the sigil detector mode for ref.v2 payloads
	// (strict|verb|permissive). Omitted on the wire for all other schemas
	// so existing envelope goldens stay byte-identical.
	SigilMode string `json:"sigil_mode,omitempty"`
	// Scope describes which databases/projects the counts and issue lists in
	// the payload cover. Populated automatically from the resolved data source
	// and flag set so every robot subcommand emits the same shape. (bt-sdg2k)
	Scope *RobotScope `json:"scope,omitempty"`
}

// RobotScope captures the effective data-source scope for a robot invocation.
// Agents read this to know whether counts and issue lists are cross-project,
// project-local, or workspace-narrowed without re-deriving from cwd. (bt-sdg2k)
type RobotScope struct {
	// Mode is one of "cross-project", "project", "workspace".
	Mode string `json:"mode"`
	// Databases lists project prefixes contributing to the payload. Populated
	// only when Mode == "cross-project" (derived from observed issue IDs).
	Databases []string `json:"databases,omitempty"`
	// ProjectFilter echoes --source/--repo when set so consumers can confirm
	// the active narrowing.
	ProjectFilter string `json:"project_filter,omitempty"`
	// Workspace echoes the --workspace config path when set.
	Workspace string `json:"workspace,omitempty"`
	// AsOf echoes the resolved commit SHA when --as-of is in effect.
	AsOf string `json:"as_of,omitempty"`
}

// NewRobotEnvelope creates a standard envelope for robot output.
func NewRobotEnvelope(dataHash string) RobotEnvelope {
	return RobotEnvelope{
		GeneratedAt:  timeNowUTCRFC3339(),
		DataHash:     dataHash,
		OutputFormat: robotOutputFormat,
		Version:      version.Version,
		Scope:        currentRobotScope(),
	}
}

// currentRobotScope inspects the resolved data source and active flag set to
// produce the envelope scope block. Returns nil only when the data source
// hasn't been resolved yet (commands that emit envelopes pre-load are rare;
// nil is preferable to a misleading "project" claim in that case). (bt-sdg2k)
func currentRobotScope() *RobotScope {
	scope := &RobotScope{}

	switch {
	case flagWorkspace != "":
		scope.Mode = "workspace"
		scope.Workspace = flagWorkspace
	case appCtx.selectedSource != nil && appCtx.selectedSource.Type == datasource.SourceTypeDoltGlobal:
		scope.Mode = "cross-project"
		scope.Databases = derivePrefixesFromIssues(appCtx.issues)
	default:
		scope.Mode = "project"
	}

	switch {
	case robotFlagSource != "":
		scope.ProjectFilter = robotFlagSource
	case flagRepo != "":
		scope.ProjectFilter = flagRepo
	}

	if appCtx.asOfResolved != "" {
		scope.AsOf = appCtx.asOfResolved
	}

	return scope
}

// derivePrefixesFromIssues returns the sorted unique set of ID prefixes seen
// in issues. Matches what the consumer's payload actually contains; a project
// with zero open issues will not appear, which is the correct semantic for
// "what scope do these counts cover". (bt-sdg2k)
func derivePrefixesFromIssues(issues []model.Issue) []string {
	if len(issues) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, 16)
	for i := range issues {
		id := issues[i].ID
		dash := strings.IndexByte(id, '-')
		if dash <= 0 {
			continue
		}
		seen[id[:dash]] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

type robotEncoder interface {
	Encode(v any) error
}

type toonRobotEncoder struct {
	w io.Writer
}

func (e *toonRobotEncoder) Encode(v any) error {
	if !toon.Available() {
		fmt.Fprintln(os.Stderr, "warning: tru not available; falling back to JSON")
		return newJSONRobotEncoder(e.w).Encode(v)
	}

	out, err := toon.EncodeWithOptions(v, robotToonEncodeOptions)
	if err != nil {
		return err
	}

	// json.Encoder.Encode always terminates with a newline; match that behavior for TOON.
	out = strings.TrimRight(out, "\n")

	if robotShowToonStats {
		if jsonBytes, jerr := json.Marshal(v); jerr == nil {
			jsonTokens := estimateTokens(string(jsonBytes))
			toonTokens := estimateTokens(out)
			savings := 0
			if jsonTokens > 0 && toonTokens <= jsonTokens {
				savings = int((1.0 - (float64(toonTokens) / float64(jsonTokens))) * 100.0)
			}
			fmt.Fprintf(os.Stderr, "[stats] JSON≈%d tok, TOON≈%d tok (%d%% savings)\n", jsonTokens, toonTokens, savings)
		}
	}

	_, err = io.WriteString(e.w, out+"\n")
	return err
}

// newJSONRobotEncoder creates a JSON encoder for robot mode output.
// By default, output is compact (no indentation) for performance.
// Set BT_PRETTY_JSON=1 to enable pretty-printing for human readability.
func newJSONRobotEncoder(w io.Writer) *json.Encoder {
	encoder := json.NewEncoder(w)
	if os.Getenv("BT_PRETTY_JSON") == "1" {
		encoder.SetIndent("", "  ")
	}
	return encoder
}

// newRobotEncoder creates an encoder for robot mode output.
//
// Default output is JSON. Use `--format toon` (or BT_OUTPUT_FORMAT/TOON_DEFAULT_FORMAT)
// to emit TOON for agent-friendly token savings.
func newRobotEncoder(w io.Writer) robotEncoder {
	if robotOutputFormat == "toon" {
		return &toonRobotEncoder{w: w}
	}
	return newJSONRobotEncoder(w)
}

func resolveRobotOutputFormat(cli string) string {
	format := strings.TrimSpace(cli)
	if format == "" {
		format = strings.TrimSpace(os.Getenv("BT_OUTPUT_FORMAT"))
	}
	if format == "" {
		format = strings.TrimSpace(os.Getenv("TOON_DEFAULT_FORMAT"))
	}
	if format == "" {
		format = "json"
	}
	return strings.ToLower(format)
}

func resolveToonEncodeOptionsFromEnv() toon.EncodeOptions {
	opts := toon.DefaultEncodeOptions()

	if v := strings.TrimSpace(os.Getenv("TOON_KEY_FOLDING")); v != "" {
		opts.KeyFolding = v
	}
	if v := strings.TrimSpace(os.Getenv("TOON_INDENT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			// Be conservative; tru supports 0..=16 but clamp to avoid surprising output.
			if n < 0 {
				n = 0
			}
			if n > 16 {
				n = 16
			}
			opts.Indent = n
		}
	}

	return opts
}

func estimateTokens(s string) int {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0
	}
	// Coarse heuristic; good enough for comparing JSON vs TOON output size.
	return (len(trimmed) + 3) / 4
}
