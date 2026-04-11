package main

import (
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update bt to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		yesFlag, _ := cmd.Flags().GetBool("yes")
		runUpdate(yesFlag)
	},
}

var checkUpdateCmd = &cobra.Command{
	Use:   "check-update",
	Short: "Check if a new version is available",
	Run: func(cmd *cobra.Command, args []string) {
		runCheckUpdate()
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback to the previous version (from backup)",
	Run: func(cmd *cobra.Command, args []string) {
		runRollback()
	},
}

func init() {
	updateCmd.Flags().Bool("yes", false, "Skip confirmation prompts")

	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(checkUpdateCmd)
	rootCmd.AddCommand(rollbackCmd)
}
