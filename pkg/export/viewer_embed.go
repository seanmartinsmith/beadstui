// Package export provides viewer asset embedding for static site generation.
package export

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Note: WriteGitHubActionsWorkflow is defined in github.go

// ViewerAssetsFS embeds the viewer_assets directory for static site export.
// This allows the bv binary to include all necessary HTML/JS/CSS assets
// without requiring them to exist on the filesystem.
//
//go:embed viewer_assets
var ViewerAssetsFS embed.FS

// CopyEmbeddedAssets copies all embedded viewer assets to the specified output directory.
// If title is provided, it replaces "Beads Viewer" in index.html.
func CopyEmbeddedAssets(outputDir, title string) error {
	// Walk the embedded filesystem and copy all files
	err := fs.WalkDir(ViewerAssetsFS, "viewer_assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// embed.FS always uses forward slashes, so use strings.TrimPrefix for cross-platform safety
		// (filepath.Rel could have issues on Windows with mixed separators)
		relPath := strings.TrimPrefix(path, "viewer_assets/")
		if relPath == path {
			// This is the root "viewer_assets" directory itself
			return nil
		}

		// Convert to platform-specific path separator for the destination
		destPath := filepath.Join(outputDir, filepath.FromSlash(relPath))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Read the embedded file
		content, err := ViewerAssetsFS.ReadFile(path)
		if err != nil {
			return err
		}

		// Special handling for index.html to replace the title and add cache-busting
		if relPath == "index.html" {
			contentStr := string(content)
			if title != "" {
				contentStr = replaceTitle(contentStr, title)
			}
			// Always add cache-busting to prevent CDN from serving stale JS files
			contentStr = AddScriptCacheBusting(contentStr)
			content = []byte(contentStr)
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Write the file
		return os.WriteFile(destPath, content, 0644)
	})

	if err != nil {
		return err
	}

	// Always add GitHub Actions workflow for reliable Pages deployment
	// This ensures deployments trigger even if the built-in Pages workflow doesn't auto-trigger
	if wfErr := WriteGitHubActionsWorkflow(outputDir); wfErr != nil {
		// Non-fatal - just log a warning (fmt is already imported via other usage)
		fmt.Printf("  Warning: Could not add GitHub Actions workflow: %v\n", wfErr)
	}

	return nil
}

// replaceTitle replaces the default title in HTML content with the provided title.
// It replaces both the <title> tag and the h1 header.
// The title is HTML-escaped to prevent XSS and broken HTML.
func replaceTitle(content, title string) string {
	if title == "" {
		return content
	}

	// Escape the title for safe HTML insertion
	safeTitle := html.EscapeString(title)

	// Replace title in <title> tag
	content = strings.Replace(content, "<title>Beads Viewer</title>", "<title>"+safeTitle+"</title>", 1)

	// Replace title in h1 header
	content = strings.Replace(content, `<h1 class="text-xl font-semibold">Beads Viewer</h1>`, `<h1 class="text-xl font-semibold">`+safeTitle+`</h1>`, 1)

	return content
}

// AddScriptCacheBusting adds a cache-busting query parameter to script src attributes.
// This ensures browsers fetch fresh JS files after deployments, preventing stale code
// from being served by CDN caches (which was causing the "Test Issue 1/2/3" bug where
// old cached viewer.js would use OPFS-cached stale data).
func AddScriptCacheBusting(content string) string {
	// Generate timestamp for cache-busting
	cacheBuster := fmt.Sprintf("?v=%d", time.Now().Unix())

	// List of our JS files that need cache-busting (not vendor files which rarely change)
	jsFiles := []string{
		"viewer.js",
		"charts.js",
		"graph.js",
		"hybrid_scorer.js",
		"wasm_loader.js",
	}

	for _, jsFile := range jsFiles {
		// Replace both src="file.js" and src='file.js' patterns
		oldSrc := fmt.Sprintf(`src="%s"`, jsFile)
		newSrc := fmt.Sprintf(`src="%s%s"`, jsFile, cacheBuster)
		content = strings.Replace(content, oldSrc, newSrc, -1)

		oldSrcSingle := fmt.Sprintf(`src='%s'`, jsFile)
		newSrcSingle := fmt.Sprintf(`src='%s%s'`, jsFile, cacheBuster)
		content = strings.Replace(content, oldSrcSingle, newSrcSingle, -1)
	}

	return content
}

// HasEmbeddedAssets returns true if viewer assets are embedded in the binary.
func HasEmbeddedAssets() bool {
	// Check if we can read the index.html from the embedded FS
	_, err := ViewerAssetsFS.ReadFile("viewer_assets/index.html")
	return err == nil
}

// AddGitHubWorkflowToBundle adds the GitHub Actions workflow to an exported bundle.
// This should be called after CopyEmbeddedAssets to ensure the workflow is present.
func AddGitHubWorkflowToBundle(outputDir string) error {
	return WriteGitHubActionsWorkflow(outputDir)
}
