package correlation

import (
	"os"
	"path/filepath"
)

var defaultBeadsFiles = []string{
	".beads/issues.jsonl",
	".beads/beads.jsonl",
	".beads/beads.base.jsonl",
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func pickBeadsFiles(repoPath string, candidates []string) []string {
	if len(candidates) == 0 {
		return nil
	}

	primary := ""
	for _, rel := range candidates {
		if rel == "" {
			continue
		}
		if fileExists(filepath.Join(repoPath, rel)) {
			primary = rel
			break
		}
	}
	if primary == "" {
		return candidates
	}

	out := make([]string, 0, len(candidates))
	out = append(out, primary)
	for _, rel := range candidates {
		if rel == primary {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func prependBeadsFile(primary string, candidates []string) []string {
	if primary == "" {
		return candidates
	}
	out := []string{primary}
	for _, rel := range candidates {
		if rel == primary {
			continue
		}
		out = append(out, rel)
	}
	return out
}
