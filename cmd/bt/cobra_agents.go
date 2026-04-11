package main

import (
	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage AGENTS.md workflow instructions",
}

// bt agents check
var agentsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check AGENTS.md blurb status",
	Run: func(cmd *cobra.Command, args []string) {
		robotMode := isRobotMode()
		runAgentsCommand(false, false, false, true, false, false, robotMode)
	},
}

// bt agents add
var agentsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add beads workflow instructions to AGENTS.md",
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		robotMode := isRobotMode()
		runAgentsCommand(true, false, false, false, dryRun, force, robotMode)
	},
}

// bt agents remove
var agentsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove beads workflow instructions from AGENTS.md",
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		robotMode := isRobotMode()
		runAgentsCommand(false, true, false, false, dryRun, force, robotMode)
	},
}

// bt agents update
var agentsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update beads workflow instructions to latest version",
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		robotMode := isRobotMode()
		runAgentsCommand(false, false, true, false, dryRun, force, robotMode)
	},
}

func isRobotMode() bool {
	return flagFormat != "" || robotOutputFormat == "toon"
}

func init() {
	// Shared flags for add/remove/update.
	for _, cmd := range []*cobra.Command{agentsAddCmd, agentsRemoveCmd, agentsUpdateCmd} {
		cmd.Flags().Bool("dry-run", false, "Show what would happen without executing")
		cmd.Flags().Bool("force", false, "Skip confirmation prompts")
	}

	agentsCmd.AddCommand(agentsCheckCmd)
	agentsCmd.AddCommand(agentsAddCmd)
	agentsCmd.AddCommand(agentsRemoveCmd)
	agentsCmd.AddCommand(agentsUpdateCmd)

	rootCmd.AddCommand(agentsCmd)
}
