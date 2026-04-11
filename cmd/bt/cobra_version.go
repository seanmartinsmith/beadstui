package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/pkg/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show bt version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("bt %s\n", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
