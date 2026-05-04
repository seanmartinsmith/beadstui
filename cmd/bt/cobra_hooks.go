package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/pkg/hooks"
)

// hooksCmd is the parent command for trust-DB management of .bt/hooks.yaml.
var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage .bt/hooks.yaml trust",
	Long: `Manage trust for .bt/hooks.yaml files.

bt refuses to execute hooks unless their hooks.yaml is registered in the
user's trust DB (~/.bt/hook-trust.json) with a matching SHA256. Editing the
file or moving the project resets trust.

Use 'bt hooks list' to see what would run before granting trust. Use
'bt hooks trust' to authorize execution after review.`,
}

var hooksListCmd = &cobra.Command{
	Use:   "list [path]",
	Short: "List configured hooks with trust status",
	Long: `Render .bt/hooks.yaml for review (without executing anything).

Defaults to ./.bt/hooks.yaml when no path is given. Each hook is shown with
its phase, name, command, and the file's overall trust status.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHooksList,
}

var hooksTrustCmd = &cobra.Command{
	Use:   "trust [path]",
	Short: "Trust the .bt/hooks.yaml at path (default: ./.bt/hooks.yaml)",
	Long: `Compute the SHA256 of a hooks.yaml file and record it as trusted.

After running 'bt hooks trust', export commands will execute the hooks in that
file until its contents change. Editing the file or moving the project resets
trust.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHooksTrust,
}

func resolveHooksPath(args []string) (string, error) {
	var raw string
	if len(args) > 0 {
		raw = args[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get cwd: %w", err)
		}
		raw = filepath.Join(cwd, ".bt", "hooks.yaml")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", raw, err)
	}
	return abs, nil
}

func runHooksList(cmd *cobra.Command, args []string) error {
	absPath, err := resolveHooksPath(args)
	if err != nil {
		return err
	}

	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("no hooks.yaml at %s\n", absPath)
			return nil
		}
		return fmt.Errorf("stat hooks file: %w", err)
	}

	// Trust badge.
	hash, err := hooks.HashHooksFile(absPath)
	if err != nil {
		return err
	}
	trusted, err := hooks.IsTrusted(absPath, hash)
	if err != nil {
		return err
	}
	badge := "[untrusted]"
	if trusted {
		badge = "[trusted]"
	} else {
		// Distinguish "never trusted" from "trusted under different hash".
		db, dberr := hooks.LoadTrustDB()
		if dberr == nil {
			if rec, ok := db.Trusted[absPath]; ok && rec.SHA256 != hash {
				badge = "[hash mismatch]"
			}
		}
	}

	fmt.Printf("%s %s\n", badge, absPath)
	fmt.Printf("sha256: %s\n", hash)

	// Load and render hook contents.
	projectDir := filepath.Dir(filepath.Dir(absPath))
	loader := hooks.NewLoader(hooks.WithProjectDir(projectDir))
	if err := loader.Load(); err != nil {
		return fmt.Errorf("load hooks: %w", err)
	}

	pre := loader.GetHooks(hooks.PreExport)
	post := loader.GetHooks(hooks.PostExport)

	if len(pre) == 0 && len(post) == 0 {
		fmt.Println("(no hooks configured)")
		return nil
	}

	if len(pre) > 0 {
		fmt.Println()
		fmt.Println("pre-export:")
		for _, h := range pre {
			fmt.Printf("  - %s\n", h.Name)
			fmt.Printf("    command: %s\n", h.Command)
			if h.OnError != "" {
				fmt.Printf("    on_error: %s\n", h.OnError)
			}
			if h.Timeout > 0 {
				fmt.Printf("    timeout: %s\n", h.Timeout)
			}
		}
	}
	if len(post) > 0 {
		fmt.Println()
		fmt.Println("post-export:")
		for _, h := range post {
			fmt.Printf("  - %s\n", h.Name)
			fmt.Printf("    command: %s\n", h.Command)
			if h.OnError != "" {
				fmt.Printf("    on_error: %s\n", h.OnError)
			}
			if h.Timeout > 0 {
				fmt.Printf("    timeout: %s\n", h.Timeout)
			}
		}
	}

	for _, w := range loader.Warnings() {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	return nil
}

func runHooksTrust(cmd *cobra.Command, args []string) error {
	absPath, err := resolveHooksPath(args)
	if err != nil {
		return err
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("stat hooks file: %w", err)
	}

	hash, err := hooks.HashHooksFile(absPath)
	if err != nil {
		return err
	}

	if err := hooks.RegisterTrust(absPath, hash); err != nil {
		return fmt.Errorf("register trust: %w", err)
	}

	short := hash
	if len(short) > 12 {
		short = short[:12]
	}
	fmt.Printf("trusted %s (%s)\n", absPath, short)
	return nil
}

func init() {
	hooksCmd.AddCommand(hooksListCmd)
	hooksCmd.AddCommand(hooksTrustCmd)
	rootCmd.AddCommand(hooksCmd)
}
