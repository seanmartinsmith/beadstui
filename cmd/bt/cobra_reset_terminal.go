package main

import (
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

// resetSequence disables mouse-tracking (1000/1002/1003/1006), bracketed
// paste (2004), the alternate screen (1049), and shows the cursor (25h).
// Idempotent: emitting it on a healthy terminal is a no-op.
const resetSequence = "\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[?2004l\x1b[?1049l\x1b[?25h"

var resetTerminalCmd = &cobra.Command{
	Use:     "reset-terminal",
	Aliases: []string{"reset"},
	Short:   "Restore terminal state after a bt crash (mouse leak, hidden cursor)",
	Long: `Emit escape sequences to disable mouse tracking, bracketed paste, the
alternate screen buffer, and re-show the cursor.

Use this when bt was hard-killed (taskkill /F, SIGKILL, parent process crash)
and left the terminal flooding mouse-event escapes as text. bt's normal exit
paths handle this automatically; reset-terminal exists for the SIGKILL case
that no in-process handler can intercept.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Write to the controlling terminal (CONOUT$/dev/tty) so the bytes
		// reach the real screen even when invoked under a wrapping process
		// (CC, tmux, redirection) that captures stdout.
		if in, out, err := tea.OpenTTY(); err == nil {
			if in != nil {
				_ = in.Close()
			}
			defer out.Close()
			if _, werr := io.WriteString(out, resetSequence); werr == nil {
				return nil
			}
		}
		_, err := io.WriteString(os.Stderr, resetSequence)
		return err
	},
}

func init() {
	rootCmd.AddCommand(resetTerminalCmd)
}
