package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/cass"
	"github.com/seanmartinsmith/beadstui/pkg/export"

	"github.com/atotto/clipboard"
)

// getCommitURL returns the GitHub/GitLab commit URL for a SHA (bv-xf4p)
func (m Model) getCommitURL(sha string) string {
	// Get git remote URL
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = m.workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	remoteURL := strings.TrimSpace(string(output))
	if remoteURL == "" {
		return ""
	}

	// Convert to web URL
	webURL := gitRemoteToWebURL(remoteURL)
	if webURL == "" {
		return ""
	}

	return webURL + "/commit/" + sha
}

// gitRemoteToWebURL converts a git remote URL to a web URL (bv-xf4p)
func gitRemoteToWebURL(remote string) string {
	// Handle SSH URLs: git@github.com:user/repo.git
	if strings.HasPrefix(remote, "git@") {
		// Remove git@ prefix and .git suffix
		remote = strings.TrimPrefix(remote, "git@")
		remote = strings.TrimSuffix(remote, ".git")
		// Replace : with /
		remote = strings.Replace(remote, ":", "/", 1)
		return "https://" + remote
	}

	// Handle HTTPS URLs: https://github.com/user/repo.git
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "http://") {
		remote = strings.TrimSuffix(remote, ".git")
		return remote
	}

	return ""
}

// openBrowserURL opens a URL in the default browser (bv-xf4p)
// Set BT_NO_BROWSER=1 to suppress browser opening (useful for tests).
func openBrowserURL(url string) error {
	// Skip browser opening in test mode or when explicitly disabled
	if os.Getenv("BT_NO_BROWSER") != "" || os.Getenv("BT_TEST_MODE") != "" {
		return nil
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// exportToMarkdown exports all issues to a Markdown file with auto-generated filename
func (m *Model) exportToMarkdown() {
	// Generate smart filename: beads_report_<project>_YYYY-MM-DD.md
	filename := m.generateExportFilename()

	// Export the issues
	err := export.SaveMarkdownToFile(m.issues, filename)
	if err != nil {
		m.statusMsg = fmt.Sprintf("❌ Export failed: %v", err)
		m.statusIsError = true
		return
	}

	m.statusMsg = fmt.Sprintf("✅ Exported %d issues to %s", len(m.issues), filename)
	m.statusIsError = false
}

// generateExportFilename creates a smart filename based on project and date
func (m *Model) generateExportFilename() string {
	// Get project name from current directory
	projectName := "beads"
	if cwd, err := os.Getwd(); err == nil {
		projectName = filepath.Base(cwd)
		// Sanitize: replace spaces and special chars with underscores
		projectName = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
				return r
			}
			return '_'
		}, projectName)
	}

	// Format: beads_report_<project>_YYYY-MM-DD.md
	timestamp := time.Now().Format("2006-01-02")
	return fmt.Sprintf("beads_report_%s_%s.md", projectName, timestamp)
}

// copyIssueToClipboard copies the selected issue to clipboard as Markdown
func (m *Model) copyIssueToClipboard() {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		m.statusMsg = "❌ No issue selected"
		m.statusIsError = true
		return
	}

	issueItem, ok := selectedItem.(IssueItem)
	if !ok {
		m.statusMsg = "❌ Invalid item type"
		m.statusIsError = true
		return
	}
	issue := issueItem.Issue

	// Format issue as Markdown
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s %s\n\n", GetTypeIconMD(string(issue.IssueType)), issue.Title))
	sb.WriteString(fmt.Sprintf("**ID:** %s  \n", issue.ID))
	sb.WriteString(fmt.Sprintf("**Status:** %s  \n", strings.ToUpper(string(issue.Status))))
	sb.WriteString(fmt.Sprintf("**Priority:** P%d  \n", issue.Priority))
	if issue.Assignee != "" {
		sb.WriteString(fmt.Sprintf("**Assignee:** @%s  \n", issue.Assignee))
	}
	sb.WriteString(fmt.Sprintf("**Created:** %s  \n", issue.CreatedAt.Format("2006-01-02")))

	if len(issue.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("**Labels:** %s  \n", strings.Join(issue.Labels, ", ")))
	}

	if issue.Description != "" {
		sb.WriteString(fmt.Sprintf("\n## Description\n\n%s\n", issue.Description))
	}

	if issue.AcceptanceCriteria != "" {
		sb.WriteString(fmt.Sprintf("\n## Acceptance Criteria\n\n%s\n", issue.AcceptanceCriteria))
	}

	// Resolution (for closed issues with close_reason)
	if issue.Status.IsClosed() && issue.CloseReason != nil && *issue.CloseReason != "" {
		sb.WriteString(fmt.Sprintf("\n## Resolution\n\n%s\n", *issue.CloseReason))
	}

	// Dependencies
	if len(issue.Dependencies) > 0 {
		sb.WriteString("\n## Dependencies\n\n")
		for _, dep := range issue.Dependencies {
			if dep == nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", dep.DependsOnID, dep.Type))
		}
	}

	// Copy to clipboard
	err := clipboard.WriteAll(sb.String())
	if err != nil {
		m.statusMsg = fmt.Sprintf("❌ Clipboard error: %v", err)
		m.statusIsError = true
		return
	}

	m.statusMsg = fmt.Sprintf("📋 Copied %s to clipboard", issue.ID)
	m.statusIsError = false
}

// showCassSessionModal shows the cass session preview modal for the selected issue (bv-5bqh)
func (m *Model) showCassSessionModal() {
	// Get the currently selected issue
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		return
	}

	issueItem, ok := selectedItem.(IssueItem)
	if !ok {
		return
	}
	issue := issueItem.Issue

	// Check if cass is available
	if m.cassCorrelator == nil {
		// Initialize correlator lazily
		detector := cass.NewDetector()
		if detector.Check() != cass.StatusHealthy {
			m.statusMsg = "⚠️ cass not available (install it for session correlation)"
			m.statusIsError = false
			return
		}
		searcher := cass.NewSearcher(detector)
		cache := cass.NewCache()
		m.cassCorrelator = cass.NewCorrelator(searcher, cache, m.workDir)
	}

	// Run correlation
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := m.cassCorrelator.Correlate(ctx, &issue)

	// If no sessions found, just show a status message
	if len(result.TopSessions) == 0 {
		m.statusMsg = "No correlated sessions found for " + issue.ID
		m.statusIsError = false
		return
	}

	// Create and show the modal
	m.cassModal = NewCassSessionModal(issue.ID, result, m.theme)
	m.cassModal.SetSize(m.width, m.height)
	m.showCassModal = true
	m.focused = focusCassModal
}

// showSelfUpdateModal shows the self-update modal (bv-182)
func (m *Model) showSelfUpdateModal() {
	// Check if an update is available
	if !m.updateAvailable || m.updateTag == "" {
		m.statusMsg = "No update available - you're running the latest version"
		m.statusIsError = false
		return
	}

	// Create and show the modal
	m.updateModal = NewUpdateModal(m.updateTag, m.updateURL, m.theme)
	m.updateModal.SetSize(m.width, m.height)
	m.showUpdateModal = true
	m.focused = focusUpdateModal
}

// getCassSessionCount returns the cached session count for the selected bead (bv-y836)
// Returns 0 if no sessions found, cass not available, or no bead selected.
// This method only checks the cache - it never triggers new correlation requests.
func (m *Model) getCassSessionCount() int {
	if m.cassCorrelator == nil {
		return 0
	}

	// Get the currently selected issue
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		return 0
	}

	issueItem, ok := selectedItem.(IssueItem)
	if !ok {
		return 0
	}

	// Check the cache for this bead
	if hint := m.cassCorrelator.GetCached(issueItem.Issue.ID); hint != nil {
		return hint.ResultCount
	}
	return 0
}
