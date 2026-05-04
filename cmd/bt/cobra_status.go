package main

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/internal/diagnostics"
	"github.com/seanmartinsmith/beadstui/pkg/ui"
)

// statusCmd surfaces a read-only diagnostic summary for humans:
// version, on-disk caches, and the events log size/entry count. The
// machine-facing twin is `bt robot health` (see robot_health.go) —
// both commands share the same probe layer in internal/diagnostics so
// the JSON and the lipgloss renderer can never disagree about what's
// on disk. (bt-uu73)
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show bt diagnostic summary (version, caches, events log)",
	Long: "Print a read-only diagnostic of the running bt: version, OS/arch, " +
		"the events.jsonl size + entry count + age range, and the on-disk size " +
		"of the .bt/semantic and .bt/baseline caches.\n\n" +
		"For machine-readable output use 'bt robot health'. The events log at " +
		"~/.bt/events.jsonl is intentionally append-only with no truncation; " +
		"this command is the surface for spotting runaway growth.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus(os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

// runStatus is split out so it's testable without going through cobra.
// It writes a multi-line lipgloss-styled summary to w. Any per-probe
// error is surfaced inline rather than aborting the whole command —
// the goal is "show as much as we can" since this is a diagnostic.
func runStatus(w *os.File) error {
	bin := diagnostics.ProbeBinary()
	eventLog, eventErr := diagnostics.ProbeEventLog()
	cwd, _ := os.Getwd()
	cache, cacheErr := diagnostics.ProbeCache(cwd)

	header := lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true)
	label := lipgloss.NewStyle().Foreground(ui.ColorSubtext)
	muted := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	value := lipgloss.NewStyle().Foreground(ui.ColorText)

	// We compose the output in a single buffered write rather than
	// streaming to stdout line-by-line so partial-render-on-error is
	// not a concern — either the whole summary lands or none of it.
	var b strings.Builder

	fmt.Fprintln(&b, header.Render("bt status"))
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "  %s %s %s\n",
		label.Render("version:"),
		value.Render(bin.Version),
		muted.Render(fmt.Sprintf("(%s/%s, %s)", bin.OS, bin.Arch, bin.GoVersion)),
	)
	fmt.Fprintln(&b)

	// Events log — the load-bearing piece. Always show even on error
	// (the probe returns partial data on errors that aren't "missing
	// file"; missing-file is not an error and the formatter already
	// renders the first-run message).
	//
	// We strip the "events log: " prefix from the formatter output to
	// avoid duplicating it under the section header. The formatter is
	// also used in single-line contexts (e.g., direct from scripts)
	// where the prefix is desirable, so it stays in the helper.
	summary := strings.TrimPrefix(diagnostics.FormatEventLogSummary(eventLog), "events log: ")
	fmt.Fprintln(&b, label.Render("events log:"))
	fmt.Fprintf(&b, "  %s\n", value.Render(summary))
	fmt.Fprintf(&b, "  %s\n", muted.Render("path: "+eventLog.Path))
	if eventErr != nil {
		fmt.Fprintf(&b, "  %s\n", lipgloss.NewStyle().Foreground(ui.ColorWarning).Render("warning: "+eventErr.Error()))
	}
	fmt.Fprintln(&b)

	// On-disk caches (project-scoped under cwd/.bt/). Zero bytes is
	// rendered, not omitted, because the user asking "how much does bt
	// use" expects to see the bookkeeping even on a fresh project.
	fmt.Fprintln(&b, label.Render("caches:"))
	fmt.Fprintf(&b, "  %s %s %s\n",
		label.Render("semantic index:"),
		value.Render(diagnostics.HumanizeBytes(cache.SemanticIndexBytes)),
		muted.Render("("+cache.SemanticIndexPath+")"),
	)
	fmt.Fprintf(&b, "  %s %s %s\n",
		label.Render("baseline:      "),
		value.Render(diagnostics.HumanizeBytes(cache.BaselineBytes)),
		muted.Render("("+cache.BaselinePath+")"),
	)
	if cacheErr != nil {
		fmt.Fprintf(&b, "  %s\n", lipgloss.NewStyle().Foreground(ui.ColorWarning).Render("warning: "+cacheErr.Error()))
	}

	if _, err := w.WriteString(b.String()); err != nil {
		return fmt.Errorf("write status: %w", err)
	}
	return nil
}
