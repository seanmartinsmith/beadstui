package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	osExec "os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/version"
)

const (
	repoOwner = "Dicklesworthstone"
	repoName  = "beads_viewer"
	baseURL   = "https://api.github.com/repos/" + repoOwner + "/" + repoName
)

// githubToken returns a GitHub personal access token from the environment,
// checking GITHUB_TOKEN first, then GH_TOKEN. Returns empty string if
// neither is set. Using a token raises the API rate limit from 60 to
// 5,000 requests/hour and avoids 403 errors on shared IPs (#117).
func githubToken() string {
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		return tok
	}
	return strings.TrimSpace(os.Getenv("GH_TOKEN"))
}

// isGitHubHost returns true if the given URL points to a github.com or
// githubusercontent.com domain (including subdomains like api.github.com,
// objects.githubusercontent.com, etc.).
func isGitHubHost(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return host == "github.com" || strings.HasSuffix(host, ".github.com") ||
		host == "githubusercontent.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

// setGitHubAuth adds Authorization header to a request if a GitHub token
// is available in the environment (GITHUB_TOKEN or GH_TOKEN) and the
// request targets a GitHub domain. This prevents leaking tokens to
// non-GitHub hosts (e.g. CDN redirects).
func setGitHubAuth(req *http.Request) {
	if tok := githubToken(); tok != "" && isGitHubHost(req.URL) {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}

// Release represents a GitHub release
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset (binary, checksum file, etc.)
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateResult contains information about an update operation
type UpdateResult struct {
	OldVersion  string `json:"old_version"`
	NewVersion  string `json:"new_version"`
	BackupPath  string `json:"backup_path,omitempty"`
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	RequireRoot bool   `json:"require_root,omitempty"`
}

// CheckForUpdates queries GitHub for the latest release.
// Returns the new version tag if an update is available, empty string otherwise.
func CheckForUpdates() (string, string, error) {
	// Set a short timeout to avoid blocking startup for too long
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	return checkForUpdates(client, "https://api.github.com/repos/Dicklesworthstone/beads_viewer/releases/latest")
}

func checkForUpdates(client *http.Client, url string) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	// GitHub recommends sending a UA; some endpoints 403 without it.
	req.Header.Set("User-Agent", "beads-viewer-update-check")
	setGitHubAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// For rate/abuse limits, avoid treating as fatal; just skip update.
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			return "", "", nil
		}
		return "", "", fmt.Errorf("github api returned status: %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", err
	}

	// Compare versions
	// Assumes SemVer with 'v' prefix
	if compareVersions(rel.TagName, version.Version) > 0 {
		return rel.TagName, rel.HTMLURL, nil
	}

	return "", "", nil
}

// compareVersions compares semver-ish strings with optional leading 'v' and optional pre-release
// suffix (e.g., v1.2.3-alpha). Pre-release versions are considered LOWER than their corresponding
// release version per SemVer spec, EXCEPT for development builds.
// Returns 1 if v1>v2, -1 if v1<v2, 0 if equal.
//
// Special handling for development builds:
// Version strings containing dev-like suffixes (e.g. "dev", "dirty", "nightly", "local")
// are considered NEWER than stable releases to prevent false "update available" prompts.
// This applies both to unparseable versions like "dev" and parseable versions like "v0.11.2-dirty".
func compareVersions(v1, v2 string) int {
	type parsed struct {
		parts      []int
		prerelease bool
		preLabel   string
	}

	// isDevLabel returns true if the prerelease label indicates a development build
	isDevLabel := func(label string) bool {
		label = strings.ToLower(label)
		return strings.Contains(label, "dev") ||
			strings.Contains(label, "dirty") ||
			strings.Contains(label, "nightly") ||
			strings.Contains(label, "local") ||
			strings.Contains(label, "snapshot") ||
			strings.Contains(label, "git")
	}

	parse := func(v string) *parsed {
		v = strings.TrimPrefix(v, "v")
		prerelease := false
		preLabel := ""
		if idx := strings.Index(v, "-"); idx != -1 {
			prerelease = true
			preLabel = v[idx+1:]
			v = v[:idx] // compare only main version numbers
		}
		parts := strings.Split(v, ".")
		res := make([]int, 3)

		// Optimization: if no parts are numeric, fail early
		allNonNumeric := true
		for _, part := range parts {
			if _, err := strconv.Atoi(part); err == nil {
				allNonNumeric = false
				break
			}
		}
		if allNonNumeric {
			return nil
		}

		for i := 0; i < len(res) && i < len(parts); i++ {
			if n, err := strconv.Atoi(parts[i]); err == nil {
				res[i] = n
			} else {
				// If a major/minor/patch component isn't a number,
				// treat the whole version as non-semver (e.g. "dev")
				return nil
			}
		}
		return &parsed{parts: res, prerelease: prerelease, preLabel: preLabel}
	}

	p1 := parse(v1)
	p2 := parse(v2)

	// If local version (v2) is a development build (unparseable), consider it newer
	// than any release to prevent downgrade prompts.
	isDev := func(v string) bool {
		v = strings.ToLower(v)
		return strings.Contains(v, "dev") ||
			strings.Contains(v, "dirty") ||
			strings.Contains(v, "nightly") ||
			strings.Contains(v, "local") ||
			strings.Contains(v, "snapshot") ||
			strings.Contains(v, "git")
	}

	if p2 == nil && isDev(v2) {
		return -1 // v2 (dev) > v1 (any)
	}

	// Special case: if local version (v2) has a dev-like prerelease suffix,
	// treat it as newer than a release with the same base version.
	// e.g., v0.11.2-dirty should NOT prompt to update to v0.11.2
	if p1 != nil && p2 != nil && p2.prerelease && isDevLabel(p2.preLabel) {
		// Check if base versions are the same or local is newer
		for i := 0; i < 3; i++ {
			if p1.parts[i] > p2.parts[i] {
				// Remote has higher version - this is a real update
				break
			}
			if p1.parts[i] < p2.parts[i] {
				// Local base version is higher - definitely no update needed
				return -1
			}
		}
		// Base versions are equal; local is a dev build of this version
		// Consider local as newer to prevent false update prompts
		partsEqual := true
		for i := 0; i < 3; i++ {
			if p1.parts[i] != p2.parts[i] {
				partsEqual = false
				break
			}
		}
		if partsEqual {
			return -1 // dev build is considered newer than same-version release
		}
	}

	// If one is parsed (valid semver) and the other is not (dev/custom),
	// treat the parsed one as newer than empty/unknown, but dev/nightly newer than released?
	// For our updater: unparsable (dev/dirty/empty) should NOT trigger downgrade prompts.
	if p1 != nil && p2 == nil {
		return 1 // v1 valid > v2 unknown
	}
	if p1 == nil && p2 != nil {
		return -1 // v1 unknown < v2 valid
	}
	// If both are unparsable (e.g., empty strings), consider equal to avoid upgrade spam
	if p1 == nil && p2 == nil {
		return strings.Compare(v1, v2)
	}

	if p1 != nil && p2 != nil {
		for i := 0; i < 3; i++ {
			if p1.parts[i] > p2.parts[i] {
				return 1
			}
			if p1.parts[i] < p2.parts[i] {
				return -1
			}
		}
		// main versions equal: compare prerelease labels
		if p1.prerelease || p2.prerelease {
			if p1.prerelease && !p2.prerelease {
				return -1 // prerelease is lower than release
			}
			if !p1.prerelease && p2.prerelease {
				return 1
			}
			// both prerelease: compare dot-separated identifiers
			parts1 := strings.Split(p1.preLabel, ".")
			parts2 := strings.Split(p2.preLabel, ".")

			len1 := len(parts1)
			len2 := len(parts2)
			limit := len1
			if len2 < limit {
				limit = len2
			}

			for i := 0; i < limit; i++ {
				part1 := parts1[i]
				part2 := parts2[i]

				// Check if parts are numeric
				num1, err1 := strconv.Atoi(part1)
				num2, err2 := strconv.Atoi(part2)

				isNum1 := err1 == nil
				isNum2 := err2 == nil

				if isNum1 && isNum2 {
					// Both numeric: compare numerically
					if num1 > num2 {
						return 1
					}
					if num1 < num2 {
						return -1
					}
				} else if !isNum1 && !isNum2 {
					// Both non-numeric: compare lexically
					if part1 > part2 {
						return 1
					}
					if part1 < part2 {
						return -1
					}
				} else {
					// One numeric, one non-numeric.
					// SemVer: numeric has lower precedence than non-numeric
					if isNum1 {
						return -1 // part1 is numeric (lower)
					}
					return 1 // part2 is numeric (lower) -> part1 is non-numeric (higher)
				}
			}

			// If all compared parts are equal, larger set of fields has higher precedence
			if len1 > len2 {
				return 1
			}
			if len1 < len2 {
				return -1
			}
		}
		return 0
	}

	// Fallback: both are non-semver (e.g. "dev" vs "dirty").
	// Use lexicographic sort just to be deterministic.
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")
	if v1 > v2 {
		return 1
	} else if v1 < v2 {
		return -1
	}
	return 0
}

// GetLatestRelease fetches full release info including assets
func GetLatestRelease() (*Release, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "beads-viewer-updater")
	setGitHubAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &rel, nil
}

// getAssetName returns the expected asset name for the current platform
func getAssetName(version string) string {
	ver := strings.TrimPrefix(version, "v")
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	return fmt.Sprintf("bv_%s_%s_%s.tar.gz", ver, goos, goarch)
}

// FindPlatformAsset finds the appropriate asset for the current OS/arch
func (r *Release) FindPlatformAsset() *Asset {
	targetName := getAssetName(r.TagName)
	for i := range r.Assets {
		if r.Assets[i].Name == targetName {
			return &r.Assets[i]
		}
	}
	return nil
}

// FindChecksumAsset finds the checksums file
func (r *Release) FindChecksumAsset() *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == "checksums.txt" {
			return &r.Assets[i]
		}
	}
	return nil
}

// downloadFile downloads a file from URL to a local path.
//
// If expectedSize is > 0, the download is size-verified against the HTTP Content-Length
// (when present) and the number of bytes written.
func downloadFile(url, destPath string, expectedSize int64) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			// Strip Authorization header when redirecting to non-GitHub hosts
			// to avoid leaking tokens to third-party CDNs.
			if !isGitHubHost(req.URL) {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "beads-viewer-updater")
	setGitHubAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status: %s", resp.Status)
	}

	if expectedSize > 0 && resp.ContentLength > 0 && resp.ContentLength != expectedSize {
		return fmt.Errorf("size mismatch: expected %d, got header %d", expectedSize, resp.ContentLength)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if expectedSize > 0 && n != expectedSize {
		return fmt.Errorf("downloaded size mismatch: expected %d, got %d", expectedSize, n)
	}

	return nil
}

// parseChecksums parses the checksums.txt file and returns a map of filename -> sha256
func parseChecksums(checksumPath string) (map[string]string, error) {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return nil, err
	}

	checksums := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "<sha256> <whitespace> <filename (may include spaces)>"
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		hash := parts[0]
		if len(line) < len(hash) || !strings.HasPrefix(line, hash) {
			continue
		}

		filename := strings.TrimSpace(line[len(hash):])
		if filename == "" {
			continue
		}

		checksums[filename] = hash
	}
	return checksums, nil
}

// verifyChecksum verifies the SHA256 checksum of a file
func verifyChecksum(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}
	return nil
}

// extractBinary extracts the bv binary from a .tar.gz archive
func extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Look for the bv binary (might be ./bv, bv, or just bv)
		name := filepath.Base(header.Name)
		if name == "bv" || name == "bv.exe" {
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("failed to create binary: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return fmt.Errorf("failed to extract binary: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("binary not found in archive")
}

// GetCurrentBinaryPath returns the path to the currently running binary
func GetCurrentBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// GetBackupPath returns the path for the backup binary
func GetBackupPath(binaryPath string) string {
	return binaryPath + ".backup"
}

// PerformUpdate downloads and installs a new version of bv
// Returns an UpdateResult with details about the operation
func PerformUpdate(release *Release, skipConfirm bool) (*UpdateResult, error) {
	result := &UpdateResult{
		OldVersion: version.Version,
		NewVersion: release.TagName,
	}

	// Check if update is needed
	if compareVersions(release.TagName, version.Version) <= 0 {
		result.Success = true
		result.Message = fmt.Sprintf("Already at version %s (latest: %s)", version.Version, release.TagName)
		return result, nil
	}

	// Find platform-specific asset
	asset := release.FindPlatformAsset()
	if asset == nil {
		return nil, fmt.Errorf("no binary available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Get current binary path
	binaryPath, err := GetCurrentBinaryPath()
	if err != nil {
		return nil, fmt.Errorf("cannot determine binary path: %w", err)
	}

	// Check write permissions
	binaryDir := filepath.Dir(binaryPath)
	testFile := filepath.Join(binaryDir, ".bv-update-test")
	if f, err := os.Create(testFile); err != nil {
		result.RequireRoot = true
		return nil, fmt.Errorf("no write permission to %s (try running with sudo)", binaryDir)
	} else {
		f.Close()
		os.Remove(testFile)
	}

	// Create temp directory for download
	tmpDir, err := os.MkdirTemp("", "bv-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download archive
	archivePath := filepath.Join(tmpDir, asset.Name)
	fmt.Printf("Downloading %s...\n", release.TagName)
	if err := downloadFile(asset.BrowserDownloadURL, archivePath, asset.Size); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Download and verify checksum
	checksumAsset := release.FindChecksumAsset()
	if checksumAsset != nil {
		checksumPath := filepath.Join(tmpDir, "checksums.txt")
		if err := downloadFile(checksumAsset.BrowserDownloadURL, checksumPath, checksumAsset.Size); err != nil {
			return nil, fmt.Errorf("checksum download failed: %w", err)
		}

		checksums, err := parseChecksums(checksumPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse checksums: %w", err)
		}

		expectedHash, ok := checksums[asset.Name]
		if !ok {
			return nil, fmt.Errorf("no checksum found for %s", asset.Name)
		}

		fmt.Println("Verifying checksum...")
		if err := verifyChecksum(archivePath, expectedHash); err != nil {
			return nil, fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract binary to temp location
	newBinaryPath := filepath.Join(tmpDir, "bv-new")
	if runtime.GOOS == "windows" {
		newBinaryPath += ".exe"
	}
	fmt.Println("Extracting...")
	if err := extractBinary(archivePath, newBinaryPath); err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Verify new binary works
	fmt.Println("Verifying new binary...")
	if err := runCommand(newBinaryPath, "--version"); err != nil {
		return nil, fmt.Errorf("new binary verification failed: %w", err)
	}

	// Create backup of current binary
	backupPath := GetBackupPath(binaryPath)
	fmt.Printf("Backing up current binary to %s...\n", backupPath)
	if err := copyFile(binaryPath, backupPath); err != nil {
		return nil, fmt.Errorf("backup failed: %w", err)
	}
	result.BackupPath = backupPath

	// Replace binary
	fmt.Println("Installing new version...")
	if err := os.Rename(newBinaryPath, binaryPath); err != nil {
		// On some systems, rename across filesystems doesn't work
		if err := copyFile(newBinaryPath, binaryPath); err != nil {
			// Restore from backup
			if restoreErr := copyFile(backupPath, binaryPath); restoreErr != nil {
				return nil, fmt.Errorf("installation failed: %w (restore also failed: %v; manual recovery: cp %s %s)", err, restoreErr, backupPath, binaryPath)
			}
			return nil, fmt.Errorf("installation failed (restored from backup): %w", err)
		}
	}

	// Ensure executable permissions
	if err := os.Chmod(binaryPath, 0755); err != nil {
		// Not fatal, but log it
		fmt.Fprintf(os.Stderr, "Warning: could not set permissions: %v\n", err)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Successfully updated from %s to %s", version.Version, release.TagName)
	return result, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// runCommand executes a command and returns any error
func runCommand(name string, args ...string) error {
	cmd := osExec.Command(name, args...)
	return cmd.Run()
}

// Rollback restores the previous version from backup
func Rollback() error {
	binaryPath, err := GetCurrentBinaryPath()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}

	backupPath := GetBackupPath(binaryPath)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found at %s", backupPath)
	}

	fmt.Printf("Rolling back from backup at %s...\n", backupPath)
	if err := copyFile(backupPath, binaryPath); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	fmt.Println("Rollback complete")
	return nil
}

// CheckUpdateAvailable is a convenience wrapper that checks and returns update info
func CheckUpdateAvailable() (available bool, newVersion string, releaseURL string, err error) {
	newVersion, releaseURL, err = CheckForUpdates()
	if err != nil {
		return false, "", "", err
	}
	return newVersion != "", newVersion, releaseURL, nil
}
