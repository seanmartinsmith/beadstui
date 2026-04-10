// Package correlation provides file-to-bead reverse index functionality.
package correlation

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BeadReference links a bead to a file via commits.
type BeadReference struct {
	BeadID       string    `json:"bead_id"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`        // open/in_progress/closed
	CommitSHAs   []string  `json:"commit_shas"`   // which commits linked this bead to this file
	LastTouch    time.Time `json:"last_touch"`    // most recent commit timestamp
	TotalChanges int       `json:"total_changes"` // sum of insertions + deletions across commits
}

// FileBeadIndex provides O(1) lookup from file path to beads that touched it.
type FileBeadIndex struct {
	// FileToBeads maps normalized file paths to beads that modified them
	FileToBeads map[string][]BeadReference `json:"file_to_beads"`

	// Stats provides aggregate information about the index
	Stats FileIndexStats `json:"stats"`
}

// FileIndexStats contains aggregate statistics about the file index.
type FileIndexStats struct {
	TotalFiles             int `json:"total_files"`               // number of unique files
	TotalBeadLinks         int `json:"total_bead_links"`          // sum of all bead references
	FilesWithMultipleBeads int `json:"files_with_multiple_beads"` // files touched by >1 bead
}

// FileBeadLookupResult is the result of looking up beads for a file.
type FileBeadLookupResult struct {
	FilePath    string          `json:"file_path"`
	OpenBeads   []BeadReference `json:"open_beads"`   // currently open beads
	ClosedBeads []BeadReference `json:"closed_beads"` // recently closed beads
	TotalBeads  int             `json:"total_beads"`
}

// FileLookup provides file-to-bead lookup functionality.
type FileLookup struct {
	index    *FileBeadIndex
	beads    map[string]BeadHistory // BeadID -> history for status lookups
	coChange *CoChangeMatrix        // Co-change matrix for related files
}

// BuildFileIndex creates a file index from a history report.
// It extracts all file paths from correlated commits and maps them to beads.
func BuildFileIndex(report *HistoryReport) *FileBeadIndex {
	if report == nil {
		return &FileBeadIndex{
			FileToBeads: make(map[string][]BeadReference),
		}
	}

	// fileBeadMap: file -> beadID -> reference (for deduplication)
	fileBeadMap := make(map[string]map[string]*BeadReference)

	for beadID, history := range report.Histories {
		for _, commit := range history.Commits {
			for _, file := range commit.Files {
				// Normalize path (remove leading ./ and normalize separators)
				normalizedPath := normalizePath(file.Path)

				if fileBeadMap[normalizedPath] == nil {
					fileBeadMap[normalizedPath] = make(map[string]*BeadReference)
				}

				ref := fileBeadMap[normalizedPath][beadID]
				if ref == nil {
					ref = &BeadReference{
						BeadID:     beadID,
						Title:      history.Title,
						Status:     history.Status,
						CommitSHAs: []string{},
						LastTouch:  commit.Timestamp,
					}
					fileBeadMap[normalizedPath][beadID] = ref
				}

				// Add commit SHA if not already present
				found := false
				for _, sha := range ref.CommitSHAs {
					if sha == commit.ShortSHA {
						found = true
						break
					}
				}
				if !found {
					ref.CommitSHAs = append(ref.CommitSHAs, commit.ShortSHA)
				}

				// Update last touch time if this commit is more recent
				if commit.Timestamp.After(ref.LastTouch) {
					ref.LastTouch = commit.Timestamp
				}

				// Accumulate changes
				ref.TotalChanges += file.Insertions + file.Deletions
			}
		}
	}

	// Convert to final structure
	result := &FileBeadIndex{
		FileToBeads: make(map[string][]BeadReference),
	}

	totalLinks := 0
	multipleBeadsCount := 0

	for filePath, beadMap := range fileBeadMap {
		refs := make([]BeadReference, 0, len(beadMap))
		for _, ref := range beadMap {
			refs = append(refs, *ref)
		}

		// Sort by last touch time (most recent first)
		sort.Slice(refs, func(i, j int) bool {
			return refs[i].LastTouch.After(refs[j].LastTouch)
		})

		result.FileToBeads[filePath] = refs
		totalLinks += len(refs)
		if len(refs) > 1 {
			multipleBeadsCount++
		}
	}

	result.Stats = FileIndexStats{
		TotalFiles:             len(result.FileToBeads),
		TotalBeadLinks:         totalLinks,
		FilesWithMultipleBeads: multipleBeadsCount,
	}

	return result
}

// NewFileLookup creates a file lookup from a history report.
func NewFileLookup(report *HistoryReport) *FileLookup {
	if report == nil {
		return &FileLookup{
			index:    BuildFileIndex(nil),
			beads:    make(map[string]BeadHistory),
			coChange: BuildCoChangeMatrix(nil),
		}
	}
	return &FileLookup{
		index:    BuildFileIndex(report),
		beads:    report.Histories,
		coChange: BuildCoChangeMatrix(report),
	}
}

// LookupByFile finds all beads that have touched a given file.
// The path can be exact or a prefix (for directory lookups).
func (fl *FileLookup) LookupByFile(path string) *FileBeadLookupResult {
	normalizedPath := normalizePath(path)

	result := &FileBeadLookupResult{
		FilePath:    path,
		OpenBeads:   []BeadReference{},
		ClosedBeads: []BeadReference{},
	}

	// Try exact match first
	if refs, ok := fl.index.FileToBeads[normalizedPath]; ok {
		for _, ref := range refs {
			// Get current status from beads map (may have changed)
			status := ref.Status
			if history, ok := fl.beads[ref.BeadID]; ok {
				ref.Status = history.Status
				ref.Title = history.Title
				status = history.Status
			}

			bucket, skip := classifyBeadStatus(status)
			if skip {
				continue
			}
			if bucket == "closed" {
				result.ClosedBeads = append(result.ClosedBeads, ref)
			} else {
				result.OpenBeads = append(result.OpenBeads, ref)
			}
		}
		sortBeadRefs(result.OpenBeads)
		sortBeadRefs(result.ClosedBeads)
		result.TotalBeads = len(result.OpenBeads) + len(result.ClosedBeads)
		return result
	}

	// Try prefix match for directory lookups
	// Note: normalizePath converts all backslashes to forward slashes, so we only need to check "/"
	for filePath, refs := range fl.index.FileToBeads {
		if strings.HasPrefix(filePath, normalizedPath+"/") {
			for _, ref := range refs {
				// Get current status
				status := ref.Status
				if history, ok := fl.beads[ref.BeadID]; ok {
					ref.Status = history.Status
					ref.Title = history.Title
					status = history.Status
				}

				bucket, skip := classifyBeadStatus(status)
				if skip {
					continue
				}

				// Avoid duplicates across files in directory
				if bucket == "closed" {
					if !containsBeadRef(result.ClosedBeads, ref.BeadID) {
						result.ClosedBeads = append(result.ClosedBeads, ref)
					}
				} else {
					if !containsBeadRef(result.OpenBeads, ref.BeadID) {
						result.OpenBeads = append(result.OpenBeads, ref)
					}
				}
			}
		}
	}

	sortBeadRefs(result.OpenBeads)
	sortBeadRefs(result.ClosedBeads)
	result.TotalBeads = len(result.OpenBeads) + len(result.ClosedBeads)
	return result
}

// LookupByFileGlob finds beads for files matching a glob pattern.
func (fl *FileLookup) LookupByFileGlob(pattern string) *FileBeadLookupResult {
	result := &FileBeadLookupResult{
		FilePath:    pattern,
		OpenBeads:   []BeadReference{},
		ClosedBeads: []BeadReference{},
	}

	// Track seen beads to avoid duplicates
	seenOpen := make(map[string]bool)
	seenClosed := make(map[string]bool)

	for filePath, refs := range fl.index.FileToBeads {
		matched, err := filepath.Match(pattern, filePath)
		if err != nil || !matched {
			continue
		}

		for _, ref := range refs {
			// Get current status
			status := ref.Status
			if history, ok := fl.beads[ref.BeadID]; ok {
				ref.Status = history.Status
				ref.Title = history.Title
				status = history.Status
			}

			bucket, skip := classifyBeadStatus(status)
			if skip {
				continue
			}
			if bucket == "closed" {
				if !seenClosed[ref.BeadID] {
					result.ClosedBeads = append(result.ClosedBeads, ref)
					seenClosed[ref.BeadID] = true
				}
			} else {
				if !seenOpen[ref.BeadID] {
					result.OpenBeads = append(result.OpenBeads, ref)
					seenOpen[ref.BeadID] = true
				}
			}
		}
	}

	sortBeadRefs(result.OpenBeads)
	sortBeadRefs(result.ClosedBeads)
	result.TotalBeads = len(result.OpenBeads) + len(result.ClosedBeads)
	return result
}

// GetAllFiles returns all files in the index, sorted by path.
func (fl *FileLookup) GetAllFiles() []string {
	files := make([]string, 0, len(fl.index.FileToBeads))
	for path := range fl.index.FileToBeads {
		files = append(files, path)
	}
	sort.Strings(files)
	return files
}

// GetStats returns statistics about the file index.
func (fl *FileLookup) GetStats() FileIndexStats {
	return fl.index.Stats
}

// GetRelatedFiles returns files that frequently co-change with the given file.
// threshold is the minimum correlation (0.0-1.0) to include (default 0.5 if <= 0).
// limit is the maximum number of related files to return (default 10 if <= 0).
func (fl *FileLookup) GetRelatedFiles(filePath string, threshold float64, limit int) *CoChangeResult {
	return fl.coChange.GetRelatedFiles(filePath, threshold, limit)
}

// GetCoChangeMatrix returns the underlying co-change matrix for advanced queries.
func (fl *FileLookup) GetCoChangeMatrix() *CoChangeMatrix {
	return fl.coChange
}

// GetHotspots returns files touched by the most beads (potential conflict zones).
func (fl *FileLookup) GetHotspots(limit int) []FileHotspot {
	type fileBeadCount struct {
		path  string
		count int
		refs  []BeadReference
	}

	var counts []fileBeadCount
	for path, refs := range fl.index.FileToBeads {
		counts = append(counts, fileBeadCount{
			path:  path,
			count: len(refs),
			refs:  refs,
		})
	}

	// Sort by count descending
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	// Take top N
	if limit <= 0 || limit > len(counts) {
		limit = len(counts)
	}

	hotspots := make([]FileHotspot, 0, limit)
	for i := 0; i < limit; i++ {
		c := counts[i]

		// Count open vs closed using current status from fl.beads
		openCount := 0
		for _, ref := range c.refs {
			// Get current status from beads map (may have changed since index was built)
			status := ref.Status
			if history, ok := fl.beads[ref.BeadID]; ok {
				status = history.Status
			}
			if status != "closed" {
				openCount++
			}
		}

		hotspots = append(hotspots, FileHotspot{
			FilePath:    c.path,
			TotalBeads:  c.count,
			OpenBeads:   openCount,
			ClosedBeads: c.count - openCount,
		})
	}

	return hotspots
}

// FileHotspot represents a file that has been touched by many beads.
type FileHotspot struct {
	FilePath    string `json:"file_path"`
	TotalBeads  int    `json:"total_beads"`
	OpenBeads   int    `json:"open_beads"`
	ClosedBeads int    `json:"closed_beads"`
}

// CoChangeEntry represents a file that frequently co-changes with another file.
type CoChangeEntry struct {
	FilePath      string   `json:"file_path"`       // The related file
	CoChangeCount int      `json:"co_change_count"` // Number of commits where both files changed
	TotalCommits  int      `json:"total_commits"`   // Total commits touching the source file
	Correlation   float64  `json:"correlation"`     // co_change_count / total_commits (0.0 - 1.0)
	SampleCommits []string `json:"sample_commits"`  // Up to 3 sample commit SHAs
}

// CoChangeResult is the result of looking up files that co-change with a given file.
type CoChangeResult struct {
	FilePath     string          `json:"file_path"`     // The queried file
	TotalCommits int             `json:"total_commits"` // Total commits touching this file
	RelatedFiles []CoChangeEntry `json:"related_files"` // Files that co-change, sorted by correlation
	Threshold    float64         `json:"threshold"`     // Minimum correlation threshold used
}

// CoChangeMatrix stores co-change relationships between files.
// Key is normalized file path, value is map of related file -> commit count.
type CoChangeMatrix struct {
	// Matrix maps file -> related file -> count of commits where both changed
	Matrix map[string]map[string]int `json:"matrix"`
	// FileCommitCounts maps file -> total commits touching that file
	FileCommitCounts map[string]int `json:"file_commit_counts"`
	// CommitFiles maps commit SHA -> files changed in that commit (for sampling)
	CommitFiles map[string][]string `json:"-"` // Not serialized, internal use
}

// BuildCoChangeMatrix creates a co-change matrix from a history report.
// It analyzes which files frequently change together in the same commits.
func BuildCoChangeMatrix(report *HistoryReport) *CoChangeMatrix {
	matrix := &CoChangeMatrix{
		Matrix:           make(map[string]map[string]int),
		FileCommitCounts: make(map[string]int),
		CommitFiles:      make(map[string][]string),
	}

	if report == nil {
		return matrix
	}

	// Track unique commits to avoid counting the same commit multiple times
	// (a commit may appear in multiple bead histories if it touches multiple beads)
	processedCommits := make(map[string]bool)

	for _, history := range report.Histories {
		for _, commit := range history.Commits {
			if processedCommits[commit.SHA] {
				continue
			}
			processedCommits[commit.SHA] = true

			// Normalize all file paths in this commit
			var files []string
			for _, fc := range commit.Files {
				normalized := normalizePath(fc.Path)
				if normalized != "" {
					files = append(files, normalized)
				}
			}

			// Store files for this commit (for sampling later)
			matrix.CommitFiles[commit.ShortSHA] = files

			// Update file commit counts
			for _, file := range files {
				matrix.FileCommitCounts[file]++
			}

			// Build co-change relationships (all pairs of files in this commit)
			for i := 0; i < len(files); i++ {
				for j := 0; j < len(files); j++ {
					if i == j {
						continue // Skip self-relationships
					}
					fileA, fileB := files[i], files[j]
					if matrix.Matrix[fileA] == nil {
						matrix.Matrix[fileA] = make(map[string]int)
					}
					matrix.Matrix[fileA][fileB]++
				}
			}
		}
	}

	return matrix
}

// GetRelatedFiles returns files that frequently co-change with the given file.
// threshold is the minimum correlation (0.0-1.0) to include (default 0.5 if <= 0).
// limit is the maximum number of related files to return (default 10 if <= 0).
func (m *CoChangeMatrix) GetRelatedFiles(filePath string, threshold float64, limit int) *CoChangeResult {
	if threshold <= 0 {
		threshold = 0.5 // Default: 50% co-occurrence
	}
	if limit <= 0 {
		limit = 10
	}

	normalizedPath := normalizePath(filePath)
	result := &CoChangeResult{
		FilePath:     filePath,
		TotalCommits: m.FileCommitCounts[normalizedPath],
		RelatedFiles: []CoChangeEntry{},
		Threshold:    threshold,
	}

	if result.TotalCommits == 0 {
		return result // File not found in history
	}

	related := m.Matrix[normalizedPath]
	if related == nil {
		return result // No co-changes found
	}

	// Build list of related files with correlation
	var entries []CoChangeEntry
	for relatedFile, count := range related {
		correlation := float64(count) / float64(result.TotalCommits)
		if correlation >= threshold {
			entry := CoChangeEntry{
				FilePath:      relatedFile,
				CoChangeCount: count,
				TotalCommits:  result.TotalCommits,
				Correlation:   correlation,
				SampleCommits: []string{},
			}

			// Find sample commits where both files changed together
			sampleCount := 0
			for sha, files := range m.CommitFiles {
				if sampleCount >= 3 {
					break
				}
				hasSource, hasRelated := false, false
				for _, f := range files {
					if f == normalizedPath {
						hasSource = true
					}
					if f == relatedFile {
						hasRelated = true
					}
				}
				if hasSource && hasRelated {
					entry.SampleCommits = append(entry.SampleCommits, sha)
					sampleCount++
				}
			}

			entries = append(entries, entry)
		}
	}

	// Sort by correlation descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Correlation > entries[j].Correlation
	})

	// Apply limit
	if len(entries) > limit {
		entries = entries[:limit]
	}

	result.RelatedFiles = entries
	return result
}

// ImpactResult is the result of analyzing what beads might be affected by file changes.
type ImpactResult struct {
	Files         []string       `json:"files"`
	AffectedBeads []AffectedBead `json:"affected_beads"`
	RiskLevel     string         `json:"risk_level"`
	RiskScore     float64        `json:"risk_score"`
	Warnings      []string       `json:"warnings"`
	Summary       string         `json:"summary"`
}

// AffectedBead represents a bead that touches one or more of the analyzed files.
type AffectedBead struct {
	BeadID       string    `json:"bead_id"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`
	OverlapFiles []string  `json:"overlap_files"`
	OverlapCount int       `json:"overlap_count"`
	LastActivity time.Time `json:"last_activity"`
	Relevance    float64   `json:"relevance"`
	TotalChanges int       `json:"total_changes"`
}

// ImpactAnalysis analyzes what beads might be affected if the given files are modified.
func (fl *FileLookup) ImpactAnalysis(files []string) *ImpactResult {
	result := &ImpactResult{
		Files:         []string{},
		AffectedBeads: []AffectedBead{},
		RiskLevel:     "low",
		RiskScore:     0.0,
		Warnings:      []string{},
	}

	if len(files) == 0 {
		result.Summary = "No files to analyze"
		return result
	}

	// Normalize, filter empty/whitespace strings, and deduplicate file paths
	seen := make(map[string]bool)
	normalizedFiles := make([]string, 0, len(files))
	for _, f := range files {
		norm := strings.TrimSpace(normalizePath(f))
		if norm == "" {
			continue // Skip empty or whitespace-only paths
		}
		if !seen[norm] {
			seen[norm] = true
			normalizedFiles = append(normalizedFiles, norm)
		}
	}

	if len(normalizedFiles) == 0 {
		result.Summary = "No valid files to analyze"
		return result
	}

	result.Files = normalizedFiles
	beadMap := make(map[string]*AffectedBead)
	now := time.Now()

	for _, filePath := range normalizedFiles {
		lookup := fl.LookupByFile(filePath)

		for _, ref := range lookup.OpenBeads {
			ab := beadMap[ref.BeadID]
			if ab == nil {
				ab = &AffectedBead{
					BeadID:       ref.BeadID,
					Title:        ref.Title,
					Status:       ref.Status,
					OverlapFiles: []string{},
					LastActivity: ref.LastTouch,
				}
				beadMap[ref.BeadID] = ab
			}
			ab.OverlapFiles = append(ab.OverlapFiles, filePath)
			ab.OverlapCount = len(ab.OverlapFiles)
			ab.TotalChanges += ref.TotalChanges
			if ref.LastTouch.After(ab.LastActivity) {
				ab.LastActivity = ref.LastTouch
			}
		}

		for _, ref := range lookup.ClosedBeads {
			if now.Sub(ref.LastTouch) > 7*24*time.Hour {
				continue
			}
			ab := beadMap[ref.BeadID]
			if ab == nil {
				ab = &AffectedBead{
					BeadID:       ref.BeadID,
					Title:        ref.Title,
					Status:       ref.Status,
					OverlapFiles: []string{},
					LastActivity: ref.LastTouch,
				}
				beadMap[ref.BeadID] = ab
			}
			ab.OverlapFiles = append(ab.OverlapFiles, filePath)
			ab.OverlapCount = len(ab.OverlapFiles)
			ab.TotalChanges += ref.TotalChanges
			if ref.LastTouch.After(ab.LastActivity) {
				ab.LastActivity = ref.LastTouch
			}
		}
	}

	openCount := 0
	inProgressCount := 0
	recentClosedCount := 0

	for _, ab := range beadMap {
		daysSince := now.Sub(ab.LastActivity).Hours() / 24
		recencyScore := 1.0 - (daysSince / 7.0)
		if recencyScore < 0 {
			recencyScore = 0
		}
		overlapScore := float64(ab.OverlapCount) / float64(len(normalizedFiles))
		statusMultiplier := 0.5
		if ab.Status == "in_progress" {
			statusMultiplier = 1.0
			inProgressCount++
		} else if ab.Status == "open" {
			statusMultiplier = 0.8
			openCount++
		} else {
			recentClosedCount++
		}
		ab.Relevance = (recencyScore*0.4 + overlapScore*0.4 + statusMultiplier*0.2)
		result.AffectedBeads = append(result.AffectedBeads, *ab)
	}

	sort.Slice(result.AffectedBeads, func(i, j int) bool {
		statusPriority := map[string]int{"in_progress": 0, "open": 1, "closed": 2}
		pi, pj := statusPriority[result.AffectedBeads[i].Status], statusPriority[result.AffectedBeads[j].Status]
		if pi != pj {
			return pi < pj
		}
		return result.AffectedBeads[i].Relevance > result.AffectedBeads[j].Relevance
	})

	result.RiskScore = float64(inProgressCount)*0.4 + float64(openCount)*0.2 + float64(recentClosedCount)*0.05
	if len(normalizedFiles) > 3 {
		result.RiskScore += 0.1
	}
	if result.RiskScore > 1.0 {
		result.RiskScore = 1.0
	}

	switch {
	case result.RiskScore >= 0.7:
		result.RiskLevel = "critical"
	case result.RiskScore >= 0.4:
		result.RiskLevel = "high"
	case result.RiskScore >= 0.2:
		result.RiskLevel = "medium"
	default:
		result.RiskLevel = "low"
	}

	if inProgressCount > 0 {
		result.Warnings = append(result.Warnings, "Active work in progress on these files - coordinate before making changes")
	}
	if openCount > 0 {
		result.Warnings = append(result.Warnings, "Open beads touch these files - review before modifying")
	}

	total := inProgressCount + openCount + recentClosedCount
	if total == 0 {
		result.Summary = "No beads found touching these files - safe to proceed"
	} else {
		parts := []string{}
		if inProgressCount > 0 {
			parts = append(parts, fmt.Sprintf("%d %s in progress", inProgressCount, pluralize(inProgressCount, "bead")))
		}
		if openCount > 0 {
			parts = append(parts, fmt.Sprintf("%d open %s", openCount, pluralize(openCount, "bead")))
		}
		if recentClosedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d recently closed %s", recentClosedCount, pluralize(recentClosedCount, "bead")))
		}
		prefix := "Found "
		if inProgressCount > 0 {
			prefix = "⚠️ Conflict risk: "
		}
		result.Summary = prefix + strings.Join(parts, ", ") + " touching these files"
	}

	return result
}

// Helper functions

// normalizePath normalizes a file path for consistent lookup.
func normalizePath(path string) string {
	// Normalize backslashes to forward slashes first (before prefix removal)
	path = strings.ReplaceAll(path, "\\", "/")

	// Remove leading ./ or ./
	path = strings.TrimPrefix(path, "./")

	// Remove trailing slash
	path = strings.TrimSuffix(path, "/")

	return path
}

func classifyBeadStatus(status string) (bucket string, skip bool) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "tombstone":
		return "", true
	case "closed":
		return "closed", false
	default:
		return "open", false
	}
}

func sortBeadRefs(refs []BeadReference) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].LastTouch.Equal(refs[j].LastTouch) {
			return refs[i].BeadID < refs[j].BeadID
		}
		return refs[i].LastTouch.After(refs[j].LastTouch)
	})
}

// containsBeadRef checks if a slice contains a bead reference with the given ID.
func containsBeadRef(refs []BeadReference, beadID string) bool {
	for _, ref := range refs {
		if ref.BeadID == beadID {
			return true
		}
	}
	return false
}

// pluralize returns the singular or plural form of a word based on count.
func pluralize(count int, singular string) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}
