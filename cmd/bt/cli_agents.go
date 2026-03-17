package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/pkg/agents"
)

// runAgentsCommand handles all --agents-* commands (bv-105).
func runAgentsCommand(add, remove, update, check, dryRun, force, robotMode bool) {
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Default to check mode when no explicit action
	isCheck := !add && !remove && !update

	detection := agents.DetectAgentFileInParents(workDir, 3)

	if robotMode {
		// JSON output for AI agents
		result := map[string]interface{}{
			"found":            detection.Found(),
			"file_path":        detection.FilePath,
			"file_type":        detection.FileType,
			"has_blurb":        detection.HasBlurb,
			"has_legacy_blurb": detection.HasLegacyBlurb,
			"blurb_version":    detection.BlurbVersion,
			"current_version":  agents.BlurbVersion,
			"needs_blurb":      detection.Found() && detection.NeedsBlurb(),
			"needs_upgrade":    detection.NeedsUpgrade(),
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		os.Exit(0)
	}

	if isCheck || check {
		runAgentsCheck(detection, workDir)
	}

	if add {
		runAgentsAdd(detection, workDir, dryRun, force)
	}

	if update {
		runAgentsUpdate(detection, dryRun, force)
	}

	if remove {
		runAgentsRemove(detection, dryRun, force)
	}
}

func runAgentsCheck(detection agents.AgentFileDetection, workDir string) {
	if !detection.Found() {
		fmt.Printf("No agent file found (searched up to 3 parent directories from %s)\n", workDir)
		fmt.Println("Run 'bt --agents-add' to create AGENTS.md with beads workflow instructions.")
		os.Exit(0)
	}
	if detection.HasLegacyBlurb {
		fmt.Printf("Found %s at %s (legacy blurb — needs upgrade)\n", detection.FileType, detection.FilePath)
		fmt.Println("Run 'bt --agents-update' to upgrade to the current format.")
		os.Exit(0)
	}
	if detection.HasBlurb && detection.BlurbVersion < agents.BlurbVersion {
		fmt.Printf("Found %s at %s (blurb v%d, current v%d — needs update)\n",
			detection.FileType, detection.FilePath, detection.BlurbVersion, agents.BlurbVersion)
		fmt.Println("Run 'bt --agents-update' to update to the latest version.")
		os.Exit(0)
	}
	if detection.HasBlurb {
		fmt.Printf("Found %s at %s (blurb v%d — up to date)\n",
			detection.FileType, detection.FilePath, detection.BlurbVersion)
		os.Exit(0)
	}
	// File exists but no blurb
	fmt.Printf("Found %s at %s (no beads workflow instructions)\n", detection.FileType, detection.FilePath)
	fmt.Println("Run 'bt --agents-add' to add beads workflow instructions.")
	os.Exit(0)
}

func runAgentsAdd(detection agents.AgentFileDetection, workDir string, dryRun, force bool) {
	if detection.Found() && detection.HasBlurb && detection.BlurbVersion >= agents.BlurbVersion {
		fmt.Printf("%s already has current blurb (v%d) — no action needed.\n", detection.FilePath, detection.BlurbVersion)
		os.Exit(0)
	}
	if detection.Found() && (detection.HasLegacyBlurb || (detection.HasBlurb && detection.BlurbVersion < agents.BlurbVersion)) {
		fmt.Println("Existing blurb found but outdated. Use --agents-update instead.")
		os.Exit(1)
	}

	targetPath := detection.FilePath
	creating := false
	if !detection.Found() {
		targetPath = agents.GetPreferredAgentFilePath(workDir)
		creating = true
	}

	if dryRun {
		if creating {
			fmt.Printf("[dry-run] Would create %s with beads workflow instructions.\n", targetPath)
		} else {
			fmt.Printf("[dry-run] Would append beads workflow instructions to %s.\n", targetPath)
		}
		os.Exit(0)
	}

	if !force {
		action := "Append blurb to"
		if creating {
			action = "Create"
		}
		fmt.Printf("%s %s? [Y/n]: ", action, targetPath)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "" && response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	if creating {
		if err := agents.CreateAgentFile(targetPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating agent file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created %s with beads workflow instructions.\n", targetPath)
	} else {
		if err := agents.AppendBlurbToFile(targetPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error appending blurb: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Appended beads workflow instructions to %s.\n", targetPath)
	}

	ok, _ := agents.VerifyBlurbPresent(targetPath)
	if !ok {
		fmt.Fprintf(os.Stderr, "Warning: verification failed — blurb may not have been written correctly.\n")
		os.Exit(1)
	}
	os.Exit(0)
}

func runAgentsUpdate(detection agents.AgentFileDetection, dryRun, force bool) {
	if !detection.Found() {
		fmt.Println("No agent file found. Use --agents-add to create one.")
		os.Exit(1)
	}
	if !detection.HasBlurb && !detection.HasLegacyBlurb {
		fmt.Printf("%s has no blurb to update. Use --agents-add to add one.\n", detection.FilePath)
		os.Exit(1)
	}
	if detection.HasBlurb && detection.BlurbVersion >= agents.BlurbVersion {
		fmt.Printf("%s already has current blurb (v%d) — no update needed.\n", detection.FilePath, detection.BlurbVersion)
		os.Exit(0)
	}

	if dryRun {
		if detection.HasLegacyBlurb {
			fmt.Printf("[dry-run] Would upgrade legacy blurb to v%d in %s.\n", agents.BlurbVersion, detection.FilePath)
		} else {
			fmt.Printf("[dry-run] Would update blurb from v%d to v%d in %s.\n",
				detection.BlurbVersion, agents.BlurbVersion, detection.FilePath)
		}
		os.Exit(0)
	}

	if !force {
		fmt.Printf("Update blurb in %s? [Y/n]: ", detection.FilePath)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "" && response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	if err := agents.UpdateBlurbInFile(detection.FilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating blurb: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Updated blurb to v%d in %s.\n", agents.BlurbVersion, detection.FilePath)

	ok, _ := agents.VerifyBlurbPresent(detection.FilePath)
	if !ok {
		fmt.Fprintf(os.Stderr, "Warning: verification failed — blurb may not have been written correctly.\n")
		os.Exit(1)
	}
	os.Exit(0)
}

func runAgentsRemove(detection agents.AgentFileDetection, dryRun, force bool) {
	if !detection.Found() {
		fmt.Println("No agent file found — nothing to remove.")
		os.Exit(0)
	}
	if !detection.HasBlurb && !detection.HasLegacyBlurb {
		fmt.Printf("%s has no blurb — nothing to remove.\n", detection.FilePath)
		os.Exit(0)
	}

	if dryRun {
		fmt.Printf("[dry-run] Would remove blurb from %s.\n", detection.FilePath)
		os.Exit(0)
	}

	if !force {
		fmt.Printf("Remove blurb from %s? [Y/n]: ", detection.FilePath)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "" && response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	if err := agents.RemoveBlurbFromFile(detection.FilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing blurb: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed blurb from %s.\n", detection.FilePath)
	os.Exit(0)
}
