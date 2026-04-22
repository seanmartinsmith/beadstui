// Package agents provides AGENTS.md integration for AI coding agents.
// It handles detection, content injection, and preference storage for
// automatically adding beadstui usage instructions to agent configuration files.
package agents

import (
	"regexp"
	"strconv"
	"strings"
)

// BlurbVersion is the current version of the agent instructions blurb.
// Increment this when making breaking changes to the blurb format.
//
// History:
//   - v1: original bv-prefixed markers, pre-rename (beadstui was "bv").
//   - v2: bt-prefixed markers, 2026-era `bt robot <subcmd>` surface.
const BlurbVersion = 2

// BlurbStartMarker marks the beginning of injected agent instructions (current format).
const BlurbStartMarker = "<!-- bt-agent-instructions-v2 -->"

// BlurbEndMarker marks the end of injected agent instructions (current format).
const BlurbEndMarker = "<!-- end-bt-agent-instructions -->"

// Legacy v1 markers (pre-rename from bv -> bt). Recognized for upgrade;
// never emitted by AppendBlurb.
const (
	legacyV1StartMarkerPrefix = "<!-- bv-agent-instructions-v"
	legacyV1EndMarker         = "<!-- end-bv-agent-instructions -->"
	currentStartMarkerPrefix  = "<!-- bt-agent-instructions-v"
)

// AgentBlurb contains the instructions to be appended to AGENTS.md / CLAUDE.md files
// when bt self-installs into a project. Target audience: AI coding agents (Claude,
// Codex, etc.) working in a beads-tracked repo via the bt binary.
const AgentBlurb = `<!-- bt-agent-instructions-v2 -->

---

## bt + beads for AI Agents

This project uses [beads](https://github.com/steveyegge/beads) (the ` + "`bd`" + ` CLI) for
issue tracking and [beadstui](https://github.com/seanmartinsmith/beadstui) (the ` + "`bt`" + `
binary) as the agent-facing analysis surface over those issues.

**Do not launch bare ` + "`bt`" + ` — it opens a blocking TUI.** Agents use ` + "`bt robot <subcommand>`" + `,
which emits deterministic JSON/TOON on stdout.

### Mandatory reads before touching issues

1. ` + "`AGENTS.md`" + ` (this file, or the project root equivalent) — project-specific rules
2. ` + "`.beads/conventions/reference.md`" + ` — issue lifecycle, field triggers, close format
3. ` + "`.beads/conventions/labels.md`" + ` — valid label taxonomy (do not invent labels)

Onboard a fresh session with ` + "`bd prime`" + ` — it dumps AI-optimized project context
(ready work, in-progress claims, recent closes).

### bt robot — the agent surface

All subcommands accept ` + "`--shape=compact|full`" + ` (aliases: ` + "`--compact`" + `, ` + "`--full`" + `; env:
` + "`BT_OUTPUT_SHAPE`" + `). Compact is the default — prefer it unless you need the full
graph payload. Add ` + "`--global`" + ` to aggregate across every project on the shared Dolt
server (required for ` + "`pairs`" + ` and ` + "`refs`" + `, which are cross-project by design).

Common scoping flags: ` + "`--label <name>`" + `, ` + "`--source <prefix,prefix>`" + `, ` + "`--as-of <ref>`" + `,
` + "`--recipe actionable|high-impact`" + `, ` + "`--bql '<query>'`" + `.

` + "```bash" + `
# Entry points
bt robot triage             # ranked recs, quick wins, blockers, project health
bt robot next               # single top pick + the claim command
bt robot list --status=open # filtered JSON list (use --bql for richer filters)
bt robot plan               # parallel execution tracks
bt robot portfolio --global # per-project roll-up across the Dolt server

# Analysis
bt robot insights           # full graph metrics (PageRank, betweenness, HITS)
bt robot impact <id>        # downstream reach of a change
bt robot blocker-chain <id> # what's blocking what, transitively
bt robot impact-network <id> --depth 2
bt robot causality <id>     # why this bead exists / what it unblocks
bt robot related <id>       # semantically + structurally similar beads
bt robot orphans            # beads the graph thinks are stranded
bt robot pairs --global     # cross-project paired beads (same suffix, different prefix)
bt robot refs  --global     # cross-project reference validation

# Signals
bt robot alerts             # stale issues, blocking cascades
bt robot priority           # P-misalignment suspects
bt robot drift              # agent/bead convention drift
bt robot suggest            # hygiene: dup candidates, missing deps, cycles
bt robot diff --diff-since <ref>
bt robot search --query "..." --mode hybrid
` + "```" + `

### Session provenance

When running under Claude Code, ` + "`$CLAUDE_SESSION_ID`" + ` is set in the environment.
Propagate it through bd writes so the audit log attributes work to the right
session:

` + "```bash" + `
bd update <id> --claim --session "$CLAUDE_SESSION_ID" \
  --set-metadata claimed_by_session="$CLAUDE_SESSION_ID" action=claimed
bd close  <id> --session "$CLAUDE_SESSION_ID" --reason="..."
` + "```" + `

### Writes: use bd (not bt)

bt is strictly read/analysis — all mutations go through ` + "`bd`" + `.

` + "```bash" + `
bd ready                                 # unblocked work, priority-ordered
bd show <id>                             # full issue + close_reason history
bd search "<query>"                      # text search
bd create "<Title>" -t task -p 2 \
  --labels="area:tui,ux" \
  --description="..."                    # title is POSITIONAL, not --title
bd update <id> --claim                   # atomic claim (sets in_progress + assignee)
bd update <id> --status=blocked          # for non-claim state transitions
bd close  <id> --reason="<template>"     # see close template below
bd comments add <id> "<note>"            # session handoffs, progress checkpoints
bd dep add <id> <depends-on>             # blocking dependency
bd human <id>                            # flag for human decision (don't invent patterns)
bd dolt push && git push                 # sync to remote (there is no single-step sync command)
` + "```" + `

Issue types: ` + "`bug, feature, task, epic, chore`" + ` (plus gastown types where supported:
` + "`spike, gate, human, event`" + `). No ` + "`question`" + ` or ` + "`docs`" + ` type — use ` + "`task`" + ` with
` + "`area:docs`" + ` label instead.

Priority semantics (pre-release single-maintainer project):

| P | Meaning |
|---|---|
| P0 | Data loss or blocks all work |
| P1 | Core command/view broken |
| P2 | Meaningful improvement or confusing behavior |
| P3 | Minor friction or polish |
| P4 | Backlog / someday-maybe |

### Close template

Use literal newlines with blank lines between fields. Floor is Summary + Change +
Files; add Verify / Risk / Notes for non-trivial changes.

` + "```bash" + `
bd close <id> --reason="Summary: <one sentence>

Change: <what changed, blast radius>

Files: <paths touched>

Verify: <command or check that confirms it holds>

Risk: <landmines, if any>

Notes: <gotchas worth saving for the next agent>"
` + "```" + `

### Session protocol

Work is not complete until ` + "`git push`" + ` succeeds:

` + "```bash" + `
git pull --rebase
bd dolt push
git push
git status   # must show 'up to date with origin'
` + "```" + `

### Discipline

- Read ` + "`close_reason`" + ` on any bead you touch — avoid re-solving closed work
- Check ` + "`bd list --status=in_progress`" + ` at session start for abandoned claims
- Close beads before committing code that addresses them
- Commit format: ` + "`type(scope): description (bt-xxx)`" + ` — bead ref in parens
- Never bypass hooks (` + "`--no-verify`" + `) or skip signing without explicit permission

<!-- end-bt-agent-instructions -->`

// SupportedAgentFiles lists the filenames that can contain agent instructions.
var SupportedAgentFiles = []string{
	"AGENTS.md",
	"CLAUDE.md",
	"agents.md",
	"claude.md",
}

// blurbVersionRegex extracts the version number from a blurb marker.
// Matches BOTH the current `bt-` marker family and the legacy `bv-` marker family
// (pre-rename). This lets NeedsUpdate detect v1-installed projects and trigger
// a v1 -> v2 upgrade on the next bt run.
var blurbVersionRegex = regexp.MustCompile(`<!-- b[tv]-agent-instructions-v(\d+) -->`)

// LegacyBlurbPatterns are markers that identify the PRE-v1 blurb format — the
// very old "### Using bt as an AI sidecar" blob, NOT the bv-prefixed v1. These
// are distinct concerns; do not merge them.
var LegacyBlurbPatterns = []string{
	"### Using bt as an AI sidecar",
	"--robot-insights",
	"--robot-plan",
	"bt already computes the hard parts",
}

// legacyBlurbStartPattern matches the beginning of the pre-v1 legacy blurb.
var legacyBlurbStartPattern = regexp.MustCompile(`(?m)^#{2,3}\s*Using bt as an AI sidecar`)

// legacyBlurbEndPattern matches content near the end of the pre-v1 legacy blurb.
// Uses non-capturing group to make the entire triple-backtick sequence optional.
var legacyBlurbEndPattern = regexp.MustCompile(`(?m)bt already computes the hard parts[^\n]*(?:\n*` + "```" + `)?\n*`)

// legacyBlurbNextSectionPattern matches the start of a new section after the legacy blurb.
// Used as fallback when the end pattern isn't found.
var legacyBlurbNextSectionPattern = regexp.MustCompile(`(?m)^#{1,2}\s+[^#]`)

// ContainsBlurb checks if the content already contains a beadstui agent blurb
// in either the current `bt-` format or the transitional `bv-` v1 format.
func ContainsBlurb(content string) bool {
	return strings.Contains(content, currentStartMarkerPrefix) ||
		strings.Contains(content, legacyV1StartMarkerPrefix)
}

// ContainsLegacyBlurb checks if the content contains the PRE-v1 blurb format
// (free-form "Using bt as an AI sidecar" blob, no HTML markers). This is NOT
// the `bv-` -> `bt-` migration — that's handled by ContainsBlurb +
// GetBlurbVersion + NeedsUpdate.
// Requires all 4 legacy patterns to match to avoid false positives on content
// that merely references robot flags.
func ContainsLegacyBlurb(content string) bool {
	if !legacyBlurbStartPattern.MatchString(content) {
		return false
	}
	matchCount := 0
	for _, pattern := range LegacyBlurbPatterns {
		if strings.Contains(content, pattern) {
			matchCount++
		}
	}
	// Require all patterns - the key differentiator is "bt already computes the hard parts"
	// which only appears in the legacy blurb, not in current documentation.
	return matchCount == len(LegacyBlurbPatterns)
}

// ContainsAnyBlurb checks if the content contains any recognized blurb format
// (current bt-, transitional bv-, or pre-v1 free-form).
func ContainsAnyBlurb(content string) bool {
	return ContainsBlurb(content) || ContainsLegacyBlurb(content)
}

// GetBlurbVersion extracts the version number from existing blurb content.
// Returns 0 if no recognized marker is present. Handles both `bt-` and `bv-`
// marker families (first match wins; `bt-` is searched via the combined regex).
func GetBlurbVersion(content string) int {
	matches := blurbVersionRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return 0
	}
	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return version
}

// NeedsUpdate checks if the content has an older version of the blurb that
// should be updated. Returns true for:
//   - pre-v1 free-form legacy blurb
//   - v1 (`bv-` marker) — needs upgrade to v2 (`bt-` marker)
//   - any future vN where N < BlurbVersion
func NeedsUpdate(content string) bool {
	if ContainsLegacyBlurb(content) {
		return true
	}
	if !ContainsBlurb(content) {
		return false
	}
	return GetBlurbVersion(content) < BlurbVersion
}

// AppendBlurb appends the agent blurb to the given content.
func AppendBlurb(content string) string {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n"
	content += AgentBlurb
	content += "\n"
	return content
}

// RemoveBlurb removes an existing blurb from the content. Handles both the
// current `bt-` format and the transitional `bv-` v1 format — whichever is
// present gets stripped. If both are present (degenerate case), only the
// first-found region is removed; a follow-up call will catch the second.
func RemoveBlurb(content string) string {
	// Try current format first.
	if startIdx := strings.Index(content, currentStartMarkerPrefix); startIdx != -1 {
		if endIdx := strings.Index(content, BlurbEndMarker); endIdx != -1 {
			endIdx += len(BlurbEndMarker)
			return stripRange(content, startIdx, endIdx)
		}
	}
	// Fall back to legacy v1 (bv-) format.
	if startIdx := strings.Index(content, legacyV1StartMarkerPrefix); startIdx != -1 {
		if endIdx := strings.Index(content, legacyV1EndMarker); endIdx != -1 {
			endIdx += len(legacyV1EndMarker)
			return stripRange(content, startIdx, endIdx)
		}
	}
	return content
}

// stripRange removes content[startIdx:endIdx] along with adjacent newlines,
// so removal doesn't leave orphan blank lines on either side.
func stripRange(content string, startIdx, endIdx int) string {
	for endIdx < len(content) && (content[endIdx] == '\n' || content[endIdx] == '\r') {
		endIdx++
	}
	for startIdx > 0 && (content[startIdx-1] == '\n' || content[startIdx-1] == '\r') {
		startIdx--
	}
	return content[:startIdx] + content[endIdx:]
}

// RemoveLegacyBlurb removes the PRE-v1 blurb (no HTML markers) from content.
func RemoveLegacyBlurb(content string) string {
	if !ContainsLegacyBlurb(content) {
		return content
	}
	startLoc := legacyBlurbStartPattern.FindStringIndex(content)
	if startLoc == nil {
		return content
	}
	startIdx := startLoc[0]
	endLoc := legacyBlurbEndPattern.FindStringIndex(content[startIdx:])
	var endIdx int
	if endLoc != nil {
		endIdx = startIdx + endLoc[1]
	} else {
		// Fallback: find the next major section heading
		nextLoc := legacyBlurbNextSectionPattern.FindStringIndex(content[startIdx+10:])
		if nextLoc != nil {
			endIdx = startIdx + 10 + nextLoc[0]
		} else {
			endIdx = len(content)
		}
	}
	for endIdx < len(content) && (content[endIdx] == '\n' || content[endIdx] == '\r') {
		endIdx++
	}
	for startIdx > 0 && (content[startIdx-1] == '\n' || content[startIdx-1] == '\r') {
		startIdx--
	}
	if startIdx > 0 {
		startIdx++
	}
	return content[:startIdx] + content[endIdx:]
}

// UpdateBlurb replaces any existing blurb (pre-v1 legacy, v1 bv-, or a
// stale v2+ bt-) with the current version.
func UpdateBlurb(content string) string {
	content = RemoveLegacyBlurb(content)
	content = RemoveBlurb(content)
	return AppendBlurb(content)
}
