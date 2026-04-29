# TOON Integration Brief: beads_viewer (bv)

**Bead:** bd-19e
**Author:** RedStone (claude-code / opus-4.5)
**Date:** 2026-01-23
**Status:** Complete

---

## 1. Files & Functions for JSON Output

### Core Output Infrastructure

| File | Key Functions/Types | Purpose |
|------|-------------------|---------|
| `cmd/bv/main.go:6583-6592` | `newRobotEncoder(w io.Writer)` | Central JSON encoder factory |
| `cmd/bv/main.go:57-169` | 25+ `--robot-*` flags | Robot mode flag definitions |

### Current Encoding Pattern

```go
// cmd/bv/main.go:6583-6592
func newRobotEncoder(w io.Writer) *json.Encoder {
    encoder := json.NewEncoder(w)
    if os.Getenv("BV_PRETTY_JSON") == "1" {
        encoder.SetIndent("", "  ")
    }
    return encoder
}
```

### Robot Flags (25+ commands)

| Flag | Line | Data Type | Purpose |
|------|------|-----------|---------|
| `--robot-triage` | 61 | `TriageResult` | Unified triage for agents |
| `--robot-next` | 64 | `TopPick` | Single top recommendation |
| `--robot-insights` | 58 | `[]Insight` | Graph analysis insights |
| `--robot-plan` | 59 | `[]PlanItem` | Execution plan |
| `--robot-priority` | 60 | `[]Recommendation` | Priority recommendations |
| `--robot-recipes` | 66 | `[]RecipeSummary` | Available recipes |
| `--robot-diff` | 65 | `DiffResult` | Changes since historical point |
| `--robot-label-health` | 67 | `LabelHealth` | Label health metrics |
| `--robot-label-flow` | 68 | `LabelFlow` | Cross-label dependencies |
| `--robot-label-attention` | 69 | `[]LabelAttention` | Attention-ranked labels |
| `--robot-alerts` | 71 | `[]Alert` | Drift + proactive alerts |
| `--robot-metrics` | 72 | `Metrics` | Performance metrics |
| `--robot-suggest` | 74 | `[]Suggestion` | Smart suggestions |
| `--robot-graph` | 79 | `Graph` | Dependency graph |
| `--robot-search` | 100 | `[]SearchResult` | Semantic search results |
| `--robot-drift` | 116 | `DriftResult` | Drift check results |
| `--robot-history` | 117 | `[]Correlation` | Bead-commit correlations |
| `--robot-orphans` | 130 | `[]OrphanCandidate` | Orphan commit candidates |
| `--robot-file-beads` | 133 | `[]FileBead` | Beads that touched a file |
| `--robot-file-hotspots` | 135 | `[]FileHotspot` | High-activity files |
| `--robot-impact` | 138 | `ImpactAnalysis` | File modification impact |
| `--robot-file-relations` | 140 | `[]FileRelation` | Co-change detection |
| `--robot-related` | 144 | `RelatedWork` | Related beads |
| `--robot-blocker-chain` | 149 | `BlockerChain` | Full blocker chain |
| `--robot-impact-network` | 151 | `ImpactNetwork` | Bead impact network |
| `--robot-causality` | 154 | `CausalChain` | Temporal causality |
| `--robot-sprint-list` | 156 | `[]Sprint` | Sprint list |
| `--robot-sprint-show` | 157 | `SprintDetails` | Sprint details |
| `--robot-forecast` | 159 | `Forecast` | ETA forecast |
| `--robot-capacity` | 164 | `CapacitySimulation` | Capacity projection |
| `--robot-burndown` | 168 | `BurndownData` | Sprint burndown |

### Key Data Structures (`pkg/analysis/triage.go`)

```go
type TriageResult struct {
    Meta            TriageMeta       `json:"meta"`
    QuickRef        QuickRef         `json:"quick_ref"`
    Recommendations []Recommendation `json:"recommendations"`
    QuickWins       []QuickWin       `json:"quick_wins"`
    BlockersToClear []BlockerItem    `json:"blockers_to_clear"`
    ProjectHealth   ProjectHealth    `json:"project_health"`
    Alerts          []Alert          `json:"alerts,omitempty"`
    Commands        CommandHelpers   `json:"commands"`
    RecommendationsByTrack []TrackRecommendationGroup `json:"recommendations_by_track,omitempty"`
    RecommendationsByLabel []LabelRecommendationGroup `json:"recommendations_by_label,omitempty"`
}
```

---

## 2. Proposed OutputFormat & CLI Flag Placement

### New Global Flag (`cmd/bv/main.go`)

```go
// Add near line 57 with other robot flags
outputFormat := flag.String("format", "",
    "Structured output format for --robot-* commands: json or toon")
```

### Environment Variable Support

```go
// In newRobotEncoder or new helper
func getRobotFormat() string {
    if format := os.Getenv("BV_OUTPUT_FORMAT"); format != "" {
        return format
    }
    if format := os.Getenv("TOON_DEFAULT_FORMAT"); format != "" {
        return format
    }
    return "json"
}
```

### Format Precedence

1. CLI flag `--format toon` (highest)
2. Environment variable `BV_OUTPUT_FORMAT=toon`
3. Environment variable `TOON_DEFAULT_FORMAT=toon`
4. Default: `json`

---

## 3. Strategy: Use toon_rust Encoder Binary (`tru`) (Canonical)

**Key Decision:** Do not embed a Go encoder. Shell out to the toon_rust `tru`
binary (installed as `tru` to avoid conflicting with coreutils `tr`) for
canonical TOON output across tools. **Never use the Node.js `toon` CLI.**

### Recommended Approach: Shell Out to `tru` (not Node `toon`)

Use the helper from `/data/projects/templates/toon_go_template.go` to resolve
`TOON_TRU_BIN` and verify the binary:

```go
// pkg/robot/toon.go
package robot

func encodeToon(payload any) (string, error) {
    jsonBytes, err := json.Marshal(payload)
    if err != nil {
        return "", err
    }
    trPath, err := getToonBinary()
    if err != nil {
        return "", err
    }
    cmd := exec.Command(trPath)
    cmd.Stdin = bytes.NewReader(jsonBytes)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("toon_rust encode failed: %s", strings.TrimSpace(stderr.String()))
    }
    return stdout.String(), nil
}
```

### FormatAuto Pattern (toon_rust-backed)

```go
// pkg/robot/format.go
type Format string

const (
    FormatJSON Format = "json"
    FormatTOON Format = "toon"
    FormatAuto Format = "auto" // Try TOON, fall back to JSON on errors
)

func Encode(payload any, format Format, w io.Writer) error {
    switch format {
    case FormatTOON:
        output, err := encodeToon(payload)
        if err != nil {
            return err // Caller decides fallback
        }
        _, err = w.Write([]byte(output))
        return err
    case FormatAuto:
        output, err := encodeToon(payload)
        if err != nil {
            // Fall back to JSON if toon_rust fails
            return json.NewEncoder(w).Encode(payload)
        }
        _, err = w.Write([]byte(output))
        return err
    default:
        return json.NewEncoder(w).Encode(payload)
    }
}
```

### Modified newRobotEncoder

```go
func newRobotEncoder(w io.Writer, format string) *robotEncoder {
    return &robotEncoder{
        w:      w,
        format: Format(format),
        pretty: os.Getenv("BV_PRETTY_JSON") == "1",
    }
}

type robotEncoder struct {
    w      io.Writer
    format Format
    pretty bool
}

func (e *robotEncoder) Encode(v any) error {
    switch e.format {
    case FormatTOON, FormatAuto:
        return robot.Encode(v, e.format, e.w)
    default:
        enc := json.NewEncoder(e.w)
        if e.pretty {
            enc.SetIndent("", "  ")
        }
        return enc.Encode(v)
    }
}
```

---

## 4. JSONL / Export Strategy

### Keep JSON for File Exports

Files written to disk (not stdout) should remain JSON:
- `triage.json` in `--agent-brief`
- `insights.json` in `--agent-brief`
- Interactive graph exports

**Rationale:**
- File exports may be consumed by other tools expecting JSON
- TOON is primarily for reducing tokens in agent context windows
- Files aren't affected by token limits

### TOON for Stdout Only

TOON format applies only to robot mode stdout output when `--format toon` is specified.

---

## 5. Doc Insertion Points

### --help Output (`cmd/bv/main.go`)

Add to flag definitions:
```go
outputFormat := flag.String("format", "",
    "Structured output format for --robot-* commands: json or toon")
```

### README.md (if exists)

```markdown
### Token-Optimized Output (TOON)

For AI agents, TOON format reduces token usage by ~50%:

```bash
# TOON output (compact, agent-friendly)
bv --robot-triage --format toon
bv --robot-insights --format toon

# Environment variable
export BV_OUTPUT_FORMAT=toon
bv --robot-next  # Uses TOON automatically
```
```

### --robot-help Output

Add to robot help text:
```
OUTPUT FORMAT
  --format toon    Token-optimized output (~50% fewer tokens than JSON)
  BV_OUTPUT_FORMAT=toon   Environment variable alternative
```

---

## 6. Sample Outputs for Fixtures

### Fixture: `bv --robot-triage --format toon`

**JSON (current):**
```json
{
  "meta": {
    "version": "2.0",
    "generated_at": "2026-01-23T22:00:00Z",
    "phase2_ready": true,
    "issue_count": 158,
    "compute_time_ms": 42
  },
  "quick_ref": {
    "open_count": 95,
    "actionable_count": 49,
    "blocked_count": 47,
    "in_progress_count": 8,
    "top_picks": [
      {"id": "bd-r9m", "title": "Define TOON conventions", "score": 0.95, "reasons": ["foundation", "unblocks 3"], "unblocks": 3}
    ]
  },
  "recommendations": [...]
}
```

**Expected TOON:**
```
meta:
  version: 2.0
  generated_at: 2026-01-23T22:00:00Z
  phase2_ready: true
  issue_count: 158
  compute_time_ms: 42
quick_ref:
  open_count: 95
  actionable_count: 49
  blocked_count: 47
  in_progress_count: 8
  top_picks[1]{id,reasons,score,title,unblocks}:
   bd-r9m,"[\"foundation\",\"unblocks 3\"]",0.95,Define TOON conventions,3
recommendations[N]{...}:
  ...
```

### Fixture: `bv --robot-next --format toon`

**JSON:**
```json
{"id":"bd-r9m","title":"Define TOON conventions","score":0.95,"reasons":["foundation","unblocks 3"],"unblocks":3}
```

**TOON:**
```
id: bd-r9m
title: Define TOON conventions
score: 0.95
reasons[2]: foundation,unblocks 3
unblocks: 3
```

### Fixture: `bv --robot-insights --format toon` (array)

**Expected TOON (tabular):**
```
[5]{category,id,message,severity}:
 graph,insight-1,Cycle detected in dependencies,warning
 health,insight-2,47 blocked issues (30%),info
 velocity,insight-3,Closure rate increased 20%,info
 priority,insight-4,3 high-priority blockers,warning
 stale,insight-5,12 issues unchanged >30d,info
```

---

## 7. Recommended Implementation Changes

### Phase 1: Core Infrastructure (3 files)

| File | Change |
|------|--------|
| `pkg/robot/toon.go` | NEW: toon_rust `tru` wrapper (see `templates/toon_go_template.go`) |
| `pkg/robot/format.go` | NEW: Format enum, Encode function |
| `cmd/bv/main.go` | Add `--format` flag, modify `newRobotEncoder` |

### Phase 2: Incremental Rollout

The existing pattern is consistent across all 35+ robot commands:
```go
encoder := newRobotEncoder(os.Stdout)
if err := encoder.Encode(output); err != nil { ... }
```

After modifying `newRobotEncoder` to accept format parameter:
```go
encoder := newRobotEncoder(os.Stdout, *robotFormat)
```

All 35+ callsites need this one-line change.

### Phase 3: Testing

1. **Round-trip test**: Encode JSON → TOON → decode → compare to original
2. **Regression test**: `--format json` output unchanged
3. **Token count test**: Verify TOON uses fewer tokens

---

## 8. Ergonomics & Non-Regression Checklist

- [x] JSON output remains the default
- [x] `BV_PRETTY_JSON=1` behavior unchanged
- [x] All existing robot flags work identically
- [x] File exports (--agent-brief, --export-*) remain JSON
- [x] New flag is optional and additive
- [x] Environment variable support for automation

---

## 9. Token Savings Estimate

Based on typical bv output patterns:

| Command | JSON tokens (est.) | TOON tokens (est.) | Savings |
|---------|--------------------|--------------------|---------|
| `--robot-triage` (full) | ~3000 | ~1400 | 53% |
| `--robot-next` | ~80 | ~40 | 50% |
| `--robot-insights` (10) | ~600 | ~280 | 53% |
| `--robot-plan` (20 items) | ~1200 | ~550 | 54% |
| `--robot-priority` (10) | ~800 | ~380 | 52% |
| `--robot-alerts` (5) | ~400 | ~190 | 52% |

**Aggregate estimate**: 50-55% token reduction across robot commands.

For agent-heavy workflows querying bv frequently (triage, next, priority), this significantly reduces context window consumption.

---

## 10. Dependency Configuration

### Option A: Use toon_rust `tru` Binary (recommended)

Use the helper from `/data/projects/templates/toon_go_template.go` to resolve the
toon_rust `tru` binary via `TOON_TRU_BIN`, and shell out for
encoding. This keeps the encoder canonical and avoids a Go reimplementation.

### Option B: Shared Go Module (future)

If TOON becomes a shared Go module:
```go
import "github.com/Dicklesworthstone/toon-go"
```

Recommendation: **Option A** for now. It keeps bv aligned with toon_rust and
avoids divergence from the canonical implementation.

---

## 11. Test Planning

### Unit Tests

```go
// pkg/robot/toon_test.go
func TestTriageResultTOON(t *testing.T) {
    triage := analysis.TriageResult{...}
    output, err := encodeToon(triage)
    require.NoError(t, err)
    // Verify output structure
}

func TestTabularArray(t *testing.T) {
    recs := []analysis.Recommendation{{...}, {...}}
    output, err := encodeToon(recs)
    require.NoError(t, err)
    require.Contains(t, output, "[2]{")  // Tabular header
}

func TestRoundTrip(t *testing.T) {
    // Encode to TOON, decode with tru, compare to original JSON
}
```

### E2E Script

```bash
#!/bin/bash
set -euo pipefail

# Compare JSON vs TOON semantic equivalence
bv --robot-triage > /tmp/triage.json
bv --robot-triage --format toon > /tmp/triage.toon
TOON_TRU_BIN="${TOON_TRU_BIN:-tru}"
"$TOON_TRU_BIN" --decode /tmp/triage.toon > /tmp/triage_decoded.json
diff <(jq -S . /tmp/triage.json) <(jq -S . /tmp/triage_decoded.json)

echo "TOON round-trip OK"
```

---

## 12. Migration Path (toon_rust canonical)

We no longer copy the ntm encoder. Use the toon_rust `tru` binary instead:

1. **Full spec support**: primitives, arrays, nested objects
2. **Tabular arrays**: Uniform object arrays → `[N]{fields}: rows`
3. **Canonical output**: consistent across tools/languages

This covers all bv robot output shapes:
- `TriageResult` → nested object ✓
- `[]Recommendation` → tabular array ✓
- `TopPick` → simple object ✓
- `ProjectHealth` → nested object ✓

**No custom encoder needed**. Wire `encodeToon` into `newRobotEncoder` and let toon_rust handle the format.
