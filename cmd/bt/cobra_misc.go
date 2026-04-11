package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
)

// bt pages (wizard)
var pagesWizardCmd = &cobra.Command{
	Use:   "pages",
	Short: "Launch interactive Pages deployment wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		if err := runPagesWizard(appCtx.beadsPath); err != nil {
			return err
		}
		return nil
	},
}

// bt feedback
var feedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Manage recommendation feedback",
}

var feedbackAcceptCmd = &cobra.Command{
	Use:   "accept [issue-id]",
	Short: "Record accept feedback for issue ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := ""
		if len(args) > 0 {
			issueID = args[0]
		}
		if issueID == "" {
			return fmt.Errorf("issue ID required")
		}
		runFeedback(issueID, "", false, false)
		return nil
	},
}

var feedbackIgnoreCmd = &cobra.Command{
	Use:   "ignore [issue-id]",
	Short: "Record ignore feedback for issue ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := ""
		if len(args) > 0 {
			issueID = args[0]
		}
		if issueID == "" {
			return fmt.Errorf("issue ID required")
		}
		runFeedback("", issueID, false, false)
		return nil
	},
}

var feedbackResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset all feedback data to defaults",
	Run: func(cmd *cobra.Command, args []string) {
		runFeedback("", "", true, false)
	},
}

var feedbackShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current feedback status and weight adjustments",
	Run: func(cmd *cobra.Command, args []string) {
		runFeedback("", "", false, true)
	},
}

// bt brief priority
var briefCmd = &cobra.Command{
	Use:   "brief",
	Short: "Generate briefs and bundles",
}

var briefPriorityCmd = &cobra.Command{
	Use:   "priority [output-file]",
	Short: "Export priority brief to Markdown file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		outputFile := ""
		if len(args) > 0 {
			outputFile = args[0]
		}
		if outputFile == "" {
			outputFile = "brief.md"
		}
		dataHash := analysis.ComputeDataHash(appCtx.issues)
		projectDir, _ := os.Getwd()
		rc := newRobotCtx(appCtx.issues, appCtx.issuesForSearch, dataHash, projectDir, appCtx.beadsPath, projectDir, nil)
		rc.runPriorityBrief(outputFile)
		return nil
	},
}

var briefAgentCmd = &cobra.Command{
	Use:   "agent [output-dir]",
	Short: "Export agent brief bundle to directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		outputDir := ""
		if len(args) > 0 {
			outputDir = args[0]
		}
		if outputDir == "" {
			return fmt.Errorf("output directory required")
		}
		dataHash := analysis.ComputeDataHash(appCtx.issues)
		projectDir, _ := os.Getwd()
		rc := newRobotCtx(appCtx.issues, appCtx.issuesForSearch, dataHash, projectDir, appCtx.beadsPath, projectDir, nil)
		rc.runAgentBrief(outputDir)
		return nil
	},
}

// bt emit-script
var emitScriptCmd = &cobra.Command{
	Use:   "emit-script",
	Short: "Emit shell script for top-N recommendations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		scriptLimit, _ := cmd.Flags().GetInt("limit")
		scriptFormat, _ := cmd.Flags().GetString("format")
		dataHash := analysis.ComputeDataHash(appCtx.issues)
		projectDir, _ := os.Getwd()
		rc := newRobotCtx(appCtx.issues, appCtx.issuesForSearch, dataHash, projectDir, appCtx.beadsPath, projectDir, nil)
		rc.runEmitScript(scriptLimit, scriptFormat)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pagesWizardCmd)

	feedbackCmd.AddCommand(feedbackAcceptCmd)
	feedbackCmd.AddCommand(feedbackIgnoreCmd)
	feedbackCmd.AddCommand(feedbackResetCmd)
	feedbackCmd.AddCommand(feedbackShowCmd)
	rootCmd.AddCommand(feedbackCmd)

	briefCmd.AddCommand(briefPriorityCmd)
	briefCmd.AddCommand(briefAgentCmd)
	rootCmd.AddCommand(briefCmd)

	emitScriptCmd.Flags().Int("limit", 5, "Limit number of items in emitted script")
	emitScriptCmd.Flags().String("format", "bash", "Script format: bash, fish, or zsh")
	rootCmd.AddCommand(emitScriptCmd)
}
