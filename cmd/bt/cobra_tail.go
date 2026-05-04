package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/tail"
)

// Flag state for `bt tail`. Declared at package level so the init() wiring
// matches the pattern used by other cobra_*.go commands.
var (
	flagTailBead         string
	flagTailEpic         string
	flagTailKinds        string
	flagTailActor        string
	flagTailIdleExit     time.Duration
	flagTailPollInterval time.Duration
	flagTailRobotFormat  string
	flagTailSince        string
)

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Stream live bead events (headless; Monitor-tool compatible)",
	Long: `Stream bead events from the active data source to stdout.

The event stream reuses the same Dolt snapshot-diff pipeline that drives the
TUI notification center (bt-nexz, bt-46p6.10). Filter by bead, epic, kind, or
actor; compose with --robot-format=jsonl for Claude Code's Monitor tool or
shell pipelines.

Recipe — orchestrator watching an executor session:

  bt tail --epic bt-46p6 --kind commented,closed \
          --actor '!bt-<self-shorthand>' --robot-format jsonl

Filters AND together server-side; no downstream grep/jq required.`,
	RunE: runTail,
}

func init() {
	f := tailCmd.Flags()
	f.StringVar(&flagTailBead, "bead", "", "Filter to events touching a single bead ID (comma-separated for multiple)")
	f.StringVar(&flagTailEpic, "epic", "", "Filter to events on an epic + all its parent-child descendants")
	f.StringVar(&flagTailKinds, "kind", "", "Comma-separated event kinds: created, edited, closed, commented, bulk (default: all)")
	f.StringVar(&flagTailActor, "actor", "", "Filter by actor (authoring session/assignee). Prefix '!' to exclude; comma-separate for multiple")
	f.DurationVar(&flagTailIdleExit, "idle-exit", 0, "Exit after this duration with no matching events (e.g. 30s, 5m; 0 = never)")
	f.DurationVar(&flagTailPollInterval, "poll-interval", time.Second, "Snapshot poll interval (default 1s)")
	f.StringVar(&flagTailRobotFormat, "robot-format", "", "Output format: human (default for TTY), jsonl, json, compact")
	f.StringVar(&flagTailSince, "since", "", "Replay events from this window before live stream (e.g. 5m, 1h; 0 = none)")

	rootCmd.AddCommand(tailCmd)
}

func runTail(cmd *cobra.Command, args []string) error {
	// Parse --robot-format. Default to jsonl when stdout is piped (mirrors
	// the robot-mode auto-detect in root.go), human otherwise.
	formatRaw := flagTailRobotFormat
	if formatRaw == "" {
		if !isStdoutTTY() {
			formatRaw = "jsonl"
		} else {
			formatRaw = "human"
		}
	}
	format, err := tail.ParseFormat(formatRaw)
	if err != nil {
		return err
	}

	kinds, err := tail.ParseKinds(flagTailKinds)
	if err != nil {
		return err
	}

	var beadIDs []string
	if flagTailBead != "" {
		for _, p := range strings.Split(flagTailBead, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				beadIDs = append(beadIDs, p)
			}
		}
	}

	var since time.Duration
	if flagTailSince != "" {
		since, err = time.ParseDuration(flagTailSince)
		if err != nil {
			return fmt.Errorf("invalid --since %q: %w", flagTailSince, err)
		}
	}

	// Establish the data source once. This primes appCtx.selectedSource
	// (Dolt / JSONL discovery, shared-server start-if-needed).
	if err := loadIssues(); err != nil {
		return err
	}

	loader, err := buildTailLoader()
	if err != nil {
		return err
	}

	stream, err := tail.New(tail.Config{
		Loader: loader,
		Filter: tail.Filter{
			BeadIDs:       beadIDs,
			Epic:          flagTailEpic,
			Kinds:         kinds,
			ActorMatchers: tail.ParseActor(flagTailActor),
		},
		Format:       format,
		Writer:       os.Stdout,
		PollInterval: flagTailPollInterval,
		IdleExit:     flagTailIdleExit,
		SinceAgo:     since,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	return stream.Run(ctx)
}

// buildTailLoader returns a LoaderFunc bound to whatever source loadIssues()
// selected. Re-loading through the same source avoids re-running discovery
// on every poll and keeps Dolt connections pooled.
func buildTailLoader() (tail.LoaderFunc, error) {
	if appCtx.selectedSource != nil {
		src := *appCtx.selectedSource
		return func(ctx context.Context) ([]model.Issue, error) {
			return datasource.LoadFromSource(src)
		}, nil
	}
	// Fallback: re-run smart discovery on each poll. Rare path — usually
	// loadIssues populates selectedSource.
	repoPath := ""
	return func(ctx context.Context) ([]model.Issue, error) {
		return datasource.LoadIssues(repoPath)
	}, nil
}
