package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/internal/diagnostics"
)

// robotHealthOutput is the wire shape for `bt robot health`. The
// envelope mirrors every other robot subcommand (RFC3339 timestamp +
// data hash + version) so AI agents can pattern-match without a
// special case here. data_hash is intentionally an empty string —
// `bt robot health` reads disk state, not bead state, so the standard
// "issues fingerprint" doesn't apply. (bt-uu73)
type robotHealthOutput struct {
	RobotEnvelope
	Binary    diagnostics.BinaryInfo    `json:"binary"`
	EventLog  diagnostics.EventLogStats `json:"event_log"`
	Cache     diagnostics.CacheStats    `json:"cache"`
	UsageHints []string                 `json:"usage_hints,omitempty"`
}

// robotHealthCmd is the agent-facing twin of `bt status`. Both commands
// read from internal/diagnostics so the JSON and the human surface
// can never diverge. Unlike most robot subcommands, this one does NOT
// load issues — it's a disk-only probe and runs in <10ms even on
// machines without a beads project. (bt-uu73)
var robotHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Output bt diagnostic state as JSON (version + caches + events log)",
	Long: "Returns binary version, on-disk cache sizes (.bt/semantic, .bt/baseline), " +
		"and the events log size + entry count + age range. Disk-only probe — does " +
		"not load beads or contact Dolt, so it works on first-run installs. The " +
		"data_hash field is empty by design (no issue state is read).",
	RunE: func(cmd *cobra.Command, args []string) error {
		bin := diagnostics.ProbeBinary()
		eventLog, eventErr := diagnostics.ProbeEventLog()
		cwd, _ := os.Getwd()
		cache, cacheErr := diagnostics.ProbeCache(cwd)

		// Per-probe errors are non-fatal at the JSON layer too — emit
		// what we have and surface the error on stderr so the JSON on
		// stdout stays valid for downstream pipes (jq, etc.). The
		// "stdout = data, stderr = noise" contract is the same as
		// every other robot subcommand.
		if eventErr != nil {
			fmt.Fprintf(os.Stderr, "warning: event log probe: %v\n", eventErr)
		}
		if cacheErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cache probe: %v\n", cacheErr)
		}

		out := robotHealthOutput{
			RobotEnvelope: NewRobotEnvelope(""), // disk-only, no bead data hash
			Binary:        bin,
			EventLog:      eventLog,
			Cache:         cache,
			UsageHints: []string{
				"jq '.event_log | {size: .size_bytes, entries: .entry_count}' - events log size",
				"jq '.cache.semantic_index_bytes + .cache.baseline_bytes' - total bt cache bytes",
				"jq '.binary.version' - bt version",
			},
		}
		enc := newRobotEncoder(os.Stdout)
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("encoding robot-health: %w", err)
		}
		return nil
	},
}
