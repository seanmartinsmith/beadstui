package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/updater"
	"github.com/seanmartinsmith/beadstui/pkg/version"
)

// runCheckUpdate handles --check-update flag (bv-182)
func runCheckUpdate() {
	available, newVersion, releaseURL, err := updater.CheckUpdateAvailable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}
	if available {
		fmt.Printf("New version available: %s (current: %s)\n", newVersion, version.Version)
		fmt.Printf("Download: %s\n", releaseURL)
		fmt.Println("\nRun 'bt --update' to update automatically")
	} else {
		fmt.Printf("bt is up to date (version %s)\n", version.Version)
	}
	os.Exit(0)
}

// runUpdate handles --update flag (bv-182)
func runUpdate(yesFlag bool) {
	release, err := updater.GetLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching release info: %v\n", err)
		os.Exit(1)
	}

	// Check if update is needed
	available, newVersion, _, _ := updater.CheckUpdateAvailable()
	if !available {
		fmt.Printf("bt is already up to date (version %s)\n", version.Version)
		os.Exit(0)
	}

	// Confirm unless --yes is provided
	if !yesFlag {
		fmt.Printf("Update bt from %s to %s? [Y/n]: ", version.Version, newVersion)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "" && response != "y" && response != "yes" {
			fmt.Println("Update cancelled")
			os.Exit(0)
		}
	}

	result, err := updater.PerformUpdate(release, yesFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		if result != nil && result.BackupPath != "" {
			fmt.Fprintf(os.Stderr, "Backup preserved at: %s\n", result.BackupPath)
		}
		os.Exit(1)
	}

	fmt.Println(result.Message)
	if result.BackupPath != "" {
		fmt.Printf("Backup saved to: %s\n", result.BackupPath)
		fmt.Println("Run 'bt --rollback' to restore if needed")
	}
	os.Exit(0)
}

// runRollback handles --rollback flag (bv-182)
func runRollback() {
	if err := updater.Rollback(); err != nil {
		fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
