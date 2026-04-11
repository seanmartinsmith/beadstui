package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
)

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Manage metric baselines for drift detection",
}

// bt baseline save
var baselineSaveCmd = &cobra.Command{
	Use:   "save [description]",
	Short: "Save current metrics as baseline with optional description",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		desc := ""
		if len(args) > 0 {
			desc = strings.Join(args, " ")
		}
		forceFullAnalysis, _ := cmd.Flags().GetBool("force-full-analysis")
		projectDir, _ := os.Getwd()
		dataHash := computeDataHash()
		rc := newRobotCtx(appCtx.issues, appCtx.issuesForSearch, dataHash, projectDir, appCtx.beadsPath, projectDir, nil)
		rc.runSaveBaseline(desc, forceFullAnalysis)
		return nil
	},
}

// bt baseline info
var baselineInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show information about the current baseline",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := os.Getwd()
		baselinePath := baseline.DefaultPath(projectDir)
		if !baseline.Exists(baselinePath) {
			fmt.Println("No baseline found.")
			fmt.Println("Create one with: bt baseline save \"description\"")
			return nil
		}
		bl, err := baseline.Load(baselinePath)
		if err != nil {
			return fmt.Errorf("loading baseline: %w", err)
		}
		fmt.Print(bl.Summary())
		return nil
	},
}

// bt baseline check
var baselineCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for drift from baseline",
	Long:  "Exit codes: 0=OK, 1=critical, 2=warning",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		forceFullAnalysis, _ := cmd.Flags().GetBool("force-full-analysis")
		robotDrift, _ := cmd.Flags().GetBool("json")
		projectDir, _ := os.Getwd()
		dataHash := computeDataHash()
		rc := newRobotCtx(appCtx.issues, appCtx.issuesForSearch, dataHash, projectDir, appCtx.beadsPath, projectDir, nil)
		rc.runCheckDrift(robotDrift, forceFullAnalysis)
		return nil
	},
}

func computeDataHash() string {
	return analysis.ComputeDataHash(appCtx.issues)
}

func init() {
	baselineSaveCmd.Flags().Bool("force-full-analysis", false, "Compute all metrics regardless of graph size")
	baselineCheckCmd.Flags().Bool("force-full-analysis", false, "Compute all metrics regardless of graph size")
	baselineCheckCmd.Flags().Bool("json", false, "Output drift check as JSON")

	baselineCmd.AddCommand(baselineSaveCmd)
	baselineCmd.AddCommand(baselineInfoCmd)
	baselineCmd.AddCommand(baselineCheckCmd)

	rootCmd.AddCommand(baselineCmd)
}
