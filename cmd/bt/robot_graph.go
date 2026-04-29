package main

import (
	"github.com/seanmartinsmith/beadstui/pkg/version"
)

// generateRobotDocs returns machine-readable documentation for AI agents (bd-2v50).
// Topics: guide, commands, examples, env, exit-codes, all.
func generateRobotDocs(topic string) map[string]interface{} {
	now := timeNowUTCRFC3339()
	result := map[string]interface{}{
		"generated_at":  now,
		"output_format": robotOutputFormat,
		"version":       version.Version,
		"topic":         topic,
	}

	guide := map[string]interface{}{
		"description": "bt (Beads TUI) provides structural analysis of the beads issue tracker DAG. It is the primary interface for AI agents to understand project state, plan work, and discover high-impact tasks.",
		"quickstart": []string{
			"bt robot triage                  # Full triage with recommendations",
			"bt robot next                    # Single top pick for immediate work",
			"bt robot plan                    # Dependency-respecting execution plan",
			"bt robot insights                # Deep graph analysis (PageRank, betweenness, etc.)",
			"bt robot schema                  # JSON Schema definitions for all commands",
			"bt tail --robot-format jsonl     # Live bead event stream (headless; Monitor-tool compatible)",
		},
		"data_source": "Dolt issues table + git history (correlations)",
		"output_modes": map[string]string{
			"json": "Default structured output",
			"toon": "Token-optimized notation (saves ~30-50% tokens)",
		},
		"output_shapes": map[string]string{
			"compact": "Default. Index projection: id/title/status/priority/type/labels/relationship counts. Envelope carries \"schema\": \"compact.v1\". Drills in via `bd show <id>`.",
			"full":    "Pre-compact shape with description/design/acceptance_criteria/notes/comments/close_reason. Envelope omits `schema` (byte-identical to pre-bt-mhwy.1 output).",
		},
	}

	type cmdDoc struct {
		Flag        string   `json:"flag"`
		Description string   `json:"description"`
		KeyFields   []string `json:"key_fields,omitempty"`
		Params      []string `json:"params,omitempty"`
		NeedsIssues bool     `json:"needs_issues"`
	}

	commands := map[string]cmdDoc{
		"robot-triage": {
			Flag: "bt robot triage", Description: "Unified triage: top picks, recommendations, quick wins, blockers, project health, velocity. Use --by-track or --by-label for grouped output.",
			KeyFields:   []string{"triage.quick_ref.top_picks", "triage.recommendations", "triage.quick_wins", "triage.blockers_to_clear", "triage.project_health"},
			Params:      []string{"--by-track", "--by-label"},
			NeedsIssues: true,
		},
		"robot-next": {
			Flag: "bt robot next", Description: "Single top recommendation with claim/show commands.",
			KeyFields:   []string{"id", "title", "score", "reasons", "unblocks", "claim_command", "show_command"},
			NeedsIssues: true,
		},
		"robot-plan": {
			Flag: "bt robot plan", Description: "Dependency-respecting execution plan with parallel tracks.",
			KeyFields:   []string{"tracks", "items", "unblocks", "summary"},
			NeedsIssues: true,
		},
		"robot-insights": {
			Flag: "bt robot insights", Description: "Deep graph analysis: PageRank, betweenness, HITS, eigenvector, k-core, cycle detection.",
			KeyFields:   []string{"pagerank", "betweenness", "hits", "eigenvector", "k_core", "cycles"},
			NeedsIssues: true,
		},
		"robot-priority": {
			Flag: "bt robot priority", Description: "Priority misalignment detection: items whose graph importance differs from assigned priority.",
			KeyFields:   []string{"misalignments", "suggestions"},
			NeedsIssues: true,
		},
		"robot-alerts": {
			Flag: "bt robot alerts", Description: "Stale issues, blocking cascades, priority mismatches.",
			KeyFields:   []string{"alerts", "severity", "affected_issues"},
			Params:      []string{"--severity info|warning|critical", "--alert-type <type>", "--alert-label <label>", "--describe-types"},
			NeedsIssues: true,
		},
		"robot-suggest": {
			Flag: "bt robot suggest", Description: "Smart suggestions: potential duplicates, missing dependencies, label assignments, cycle warnings.",
			KeyFields:   []string{"suggestions", "type", "confidence"},
			Params:      []string{"--type duplicate|dependency|label|cycle", "--min-confidence 0.0-1.0", "--bead <id>"},
			NeedsIssues: true,
		},
		"robot-schema": {
			Flag: "bt robot schema", Description: "JSON Schema definitions for all robot command outputs.",
			KeyFields:   []string{"schema_version", "envelope", "commands"},
			Params:      []string{"--command <cmd>"},
			NeedsIssues: false,
		},
		"robot-docs": {
			Flag: "bt robot docs [topic]", Description: "Machine-readable JSON documentation. Topics: guide, commands, examples, env, exit-codes, all.",
			NeedsIssues: false,
		},
		"robot-history": {
			Flag: "bt robot history", Description: "Bead-to-commit correlations from git history.",
			KeyFields:   []string{"correlations", "confidence", "commit_sha", "bead_id"},
			Params:      []string{"--bead-history <id>", "--history-since <date>", "--history-limit <n>", "--min-confidence 0.0-1.0"},
			NeedsIssues: true,
		},
		"robot-diff": {
			Flag: "bt robot diff", Description: "Changes since a historical point (commit, branch, tag, or date).",
			Params:      []string{"--since <ref>"},
			NeedsIssues: true,
		},
		"robot-search": {
			Flag: "bt robot search", Description: "Semantic vector search over issue titles and descriptions.",
			Params:      []string{"--search <query>", "--search-limit <n>", "--search-mode text|hybrid"},
			NeedsIssues: true,
		},
		"robot-label-health": {
			Flag: "bt robot labels health", Description: "Per-label health metrics: open/closed counts, velocity, staleness.",
			NeedsIssues: true,
		},
		"robot-label-flow": {
			Flag: "bt robot labels flow", Description: "Cross-label dependency flow analysis.",
			NeedsIssues: true,
		},
		"robot-label-attention": {
			Flag: "bt robot labels attention", Description: "Attention-ranked labels requiring focus.",
			Params:      []string{"--limit <n>"},
			NeedsIssues: true,
		},
		"robot-graph": {
			Flag: "bt robot graph", Description: "Dependency graph export in JSON, DOT, or Mermaid format.",
			Params:      []string{"--graph-format json|dot|mermaid", "--graph-root <id>", "--graph-depth <n>"},
			NeedsIssues: true,
		},
		"robot-metrics": {
			Flag: "bt robot metrics", Description: "Performance metrics: timing, cache hit rates, memory usage.",
			NeedsIssues: true,
		},
		"robot-orphans": {
			Flag: "bt robot orphans", Description: "Orphan commit candidates that should be linked to beads.",
			Params:      []string{"--min-score 0-100"},
			NeedsIssues: true,
		},
		"robot-file-beads": {
			Flag: "bt robot files beads <path>", Description: "Beads that touched a specific file path.",
			Params:      []string{"--limit <n>"},
			NeedsIssues: true,
		},
		"robot-file-hotspots": {
			Flag: "bt robot files hotspots", Description: "Files touched by the most beads.",
			Params:      []string{"--limit <n>"},
			NeedsIssues: true,
		},
		"robot-file-relations": {
			Flag: "bt robot files relations <path>", Description: "Files that frequently co-change with a given file.",
			Params:      []string{"--threshold 0.0-1.0", "--limit <n>"},
			NeedsIssues: true,
		},
		"robot-related": {
			Flag: "bt robot related <id>", Description: "Beads related to a specific bead ID.",
			Params:      []string{"--min-relevance 0-100", "--max-results <n>", "--include-closed"},
			NeedsIssues: true,
		},
		"robot-blocker-chain": {
			Flag: "bt robot blocker-chain <id>", Description: "Full blocker chain analysis for an issue.",
			NeedsIssues: true,
		},
		"robot-impact-network": {
			Flag: "bt robot impact-network [<id>|all]", Description: "Impact network graph (full or subnetwork for a bead).",
			Params:      []string{"--depth 1-3"},
			NeedsIssues: true,
		},
		"robot-causality": {
			Flag: "bt robot causality <id>", Description: "Causal chain analysis for a bead.",
			NeedsIssues: true,
		},
		"robot-sprint-list": {
			Flag: "bt robot sprint list", Description: "List all sprints as JSON.",
			NeedsIssues: true,
		},
		"robot-sprint-show": {
			Flag: "bt robot sprint show <id>", Description: "Show details for a specific sprint.",
			NeedsIssues: true,
		},
		"robot-forecast": {
			Flag: "bt robot forecast <id|all>", Description: "ETA predictions for bead completion.",
			Params:      []string{"--forecast-label <label>", "--forecast-sprint <id>", "--forecast-agents <n>"},
			NeedsIssues: true,
		},
		"robot-capacity": {
			Flag: "bt robot capacity", Description: "Capacity simulation and completion projections.",
			Params:      []string{"--agents <n>", "--capacity-label <label>"},
			NeedsIssues: true,
		},
		"robot-burndown": {
			Flag: "bt robot burndown <sprint|current>", Description: "Sprint burndown data.",
			NeedsIssues: true,
		},
		"robot-drift": {
			Flag: "bt robot drift", Description: "Drift detection from saved baseline.",
			NeedsIssues: true,
		},
	}

	examples := []map[string]string{
		{"description": "Get top 3 picks for immediate work", "command": "bt robot triage | jq '.triage.quick_ref.top_picks[:3]'"},
		{"description": "Claim the top recommendation", "command": "bt robot next | jq -r '.claim_command' | sh"},
		{"description": "Find high-impact blockers to clear", "command": "bt robot triage | jq '.triage.blockers_to_clear | map(.id)'"},
		{"description": "Get bug-only recommendations", "command": "bt robot triage | jq '.triage.recommendations[] | select(.type == \"bug\")'"},
		{"description": "Multi-agent: top pick per parallel track", "command": "bt robot triage --by-track | jq '.triage.recommendations_by_track[].top_pick'"},
		{"description": "Find beads related to a specific file", "command": "bt robot files beads src/main.rs"},
		{"description": "Search for issues by keyword", "command": "bt robot search 'authentication'"},
		{"description": "Get TOON output (saves tokens)", "command": "bt robot triage --format toon"},
		{"description": "Use env for default format", "command": "BT_OUTPUT_FORMAT=toon bt robot triage"},
		{"description": "Show token savings estimate", "command": "bt robot triage --format toon --stats"},
	}

	envVars := map[string]string{
		// Output format and shape
		"BT_OUTPUT_FORMAT":    "Default output format: json or toon (overridden by --format)",
		"BT_OUTPUT_SHAPE":     "Default output shape: compact or full (overridden by --shape / --compact / --full)",
		"BT_OUTPUT_SCHEMA":    "Default projection schema on `bt robot pairs` and `bt robot refs`: v1 or v2 (overridden by --schema)",
		"BT_SIGIL_MODE":       "Default sigil recognition mode on `bt robot refs --schema=v2`: strict, verb, or permissive (overridden by --sigils)",
		"TOON_DEFAULT_FORMAT": "Fallback format if BT_OUTPUT_FORMAT not set",
		"TOON_STATS":          "Set to 1 to show JSON vs TOON token estimates on stderr",
		"TOON_KEY_FOLDING":    "TOON key folding mode",
		"TOON_INDENT":         "TOON indentation level (0-16)",
		"BT_PRETTY_JSON":      "Set to 1 for indented JSON output",
		"BT_ROBOT":            "Set to 1 to force robot mode (clean stdout)",

		// Semantic search
		"BT_SEARCH_MODE":      "Search ranking mode: text or hybrid",
		"BT_SEARCH_PRESET":    "Hybrid search preset name (overridden by --preset)",
		"BT_SEARCH_WEIGHTS":   "Hybrid search weights as JSON (overridden by --weights)",
		"BT_SEMANTIC_EMBEDDER": "Embedding provider for semantic search (default: hash)",
		"BT_SEMANTIC_MODEL":    "Embedding model identifier (provider-specific)",
		"BT_SEMANTIC_DIM":      "Embedding vector dimension (default: provider default)",

		// Data sources and Dolt connectivity
		"BEADS_DIR":              "Directory containing beads data (overrides auto-discovery)",
		"BEADS_DOLT_SERVER_HOST": "Dolt server host (beads-native, highest priority)",
		"BEADS_DOLT_SERVER_PORT": "Dolt server port (beads-native, highest priority)",
		"BEADS_DOLT_SERVER_USER": "Dolt server user (beads-native, highest priority)",
		"BT_DOLT_PORT":           "bt-specific Dolt port override for testing or non-standard setups",
		"BT_GLOBAL_DOLT_PORT":    "Global-mode Dolt port (overrides ~/.beads/shared-server/dolt-server.port)",

		// Operational / runtime
		"BT_CACHE_DIR":              "Base directory for the analysis cache (default: <project>/.bt/cache)",
		"BT_DEBUG":                  "Set to 1 to enable debug logging to stderr",
		"BT_METRICS":                "Set to 0 to disable internal timing-metric collection",
		"BT_BACKGROUND_MODE":        "Internal: set by bt itself (1 when running in background/daemon mode)",
		"BT_NO_BROWSER":             "Set to 1 to suppress browser-opening (tests, headless environments)",
		"BT_NO_SAVED_CONFIG":        "Set to 1 to skip reading the saved export wizard configuration",
		"BT_TEST_MODE":              "Set to 1 to enable test-mode guards (e.g. fail fast in global-mode Dolt discovery)",
		"BT_STALE_DAYS":             "Staleness threshold in days for TUI highlighting (default: 14)",
		"BT_INSIGHTS_MAP_LIMIT":     "Per-map size limit in `bt robot insights` output (reduces payload size)",
		"BT_TEMPORAL_CACHE_TTL":     "Cache TTL for temporal analysis snapshots (e.g. '30m', '2h')",
		"BT_TEMPORAL_MAX_SNAPSHOTS": "Maximum snapshots retained by the temporal analyzer",
		"BT_TUI_AUTOCLOSE_MS":       "Auto-close the TUI after N milliseconds (used by tests / demos)",
		"BT_WORKER_LOG_LEVEL":       "Background-worker log level: debug, info, warn, error",
		"BT_WORKER_TRACE":           "Path to write a background-worker trace log (empty to disable)",

		// Build-time (not read at runtime, but documented for completeness)
		"BT_BUILD_HYBRID_WASM": "Build flag: set to non-empty to require wasm-pack when building the hybrid-search WASM module",
	}

	exitCodes := map[string]string{
		"0": "Success",
		"1": "Error (general failure, drift critical)",
		"2": "Invalid arguments or drift warning",
	}

	switch topic {
	case "guide":
		result["guide"] = guide
	case "commands":
		result["commands"] = commands
	case "examples":
		result["examples"] = examples
	case "env":
		result["environment_variables"] = envVars
	case "exit-codes":
		result["exit_codes"] = exitCodes
	case "all":
		result["guide"] = guide
		result["commands"] = commands
		result["examples"] = examples
		result["environment_variables"] = envVars
		result["exit_codes"] = exitCodes
	default:
		result["error"] = "Unknown topic: " + topic
		result["available_topics"] = []string{"guide", "commands", "examples", "env", "exit-codes", "all"}
	}

	return result
}

// RobotSchemas holds JSON Schema definitions for all robot commands
type RobotSchemas struct {
	SchemaVersion string                            `json:"schema_version"`
	GeneratedAt   string                            `json:"generated_at"`
	Envelope      map[string]interface{}            `json:"envelope"`
	Commands      map[string]map[string]interface{} `json:"commands"`
}

// generateRobotSchemas creates JSON Schema definitions for robot command outputs
func generateRobotSchemas() RobotSchemas {
	now := timeNowUTCRFC3339()

	// Common envelope schema (present in all robot outputs)
	envelope := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"generated_at": map[string]interface{}{
				"type":        "string",
				"format":      "date-time",
				"description": "ISO 8601 timestamp when output was generated",
			},
			"data_hash": map[string]interface{}{
				"type":        "string",
				"description": "Fingerprint of source beads.jsonl for cache validation",
			},
			"output_format": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"json", "toon"},
				"description": "Output format used (json or toon)",
			},
			"version": map[string]interface{}{
				"type":        "string",
				"description": "bt version that generated this output",
			},
			"schema": map[string]interface{}{
				"type":        "string",
				"description": "Projection schema carried in the payload (e.g., compact.v1). Absent when the payload is the full/default shape. See bt-mhwy.1 and pkg/view/schemas/compact_issue.v1.json.",
			},
		},
		"required": []string{"generated_at", "data_hash"},
	}

	commands := map[string]map[string]interface{}{
		"robot-triage": {
			"$schema":     "https://json-schema.org/draft/2020-12/schema",
			"title":       "Robot Triage Output",
			"description": "Unified triage recommendations with quick picks, blockers, and project health",
			"type":        "object",
			"properties": map[string]interface{}{
				"generated_at": map[string]interface{}{"type": "string", "format": "date-time"},
				"data_hash":    map[string]interface{}{"type": "string"},
				"triage": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"meta": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"version":      map[string]interface{}{"type": "string"},
								"generated_at": map[string]interface{}{"type": "string"},
								"phase2_ready": map[string]interface{}{"type": "boolean"},
								"issue_count":  map[string]interface{}{"type": "integer"},
							},
						},
						"quick_ref": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"open_count":        map[string]interface{}{"type": "integer"},
								"actionable_count":  map[string]interface{}{"type": "integer"},
								"blocked_count":     map[string]interface{}{"type": "integer"},
								"in_progress_count": map[string]interface{}{"type": "integer"},
								"top_picks": map[string]interface{}{
									"type":  "array",
									"items": map[string]interface{}{"$ref": "#/$defs/recommendation"},
								},
							},
						},
						"recommendations": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"$ref": "#/$defs/recommendation"},
						},
						"quick_wins":        map[string]interface{}{"type": "array"},
						"blockers_to_clear": map[string]interface{}{"type": "array"},
						"project_health":    map[string]interface{}{"type": "object"},
						"commands":          map[string]interface{}{"type": "object"},
					},
				},
				"usage_hints": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			},
			"$defs": map[string]interface{}{
				"recommendation": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":       map[string]interface{}{"type": "string"},
						"title":    map[string]interface{}{"type": "string"},
						"type":     map[string]interface{}{"type": "string"},
						"status":   map[string]interface{}{"type": "string"},
						"priority": map[string]interface{}{"type": "integer"},
						"labels":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"score":    map[string]interface{}{"type": "number"},
						"reasons":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"unblocks": map[string]interface{}{"type": "integer"},
					},
					"required": []string{"id", "title", "score"},
				},
			},
		},
		"robot-next":     {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Next Output", "description": "Single top pick recommendation with claim command", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "id": map[string]interface{}{"type": "string"}, "title": map[string]interface{}{"type": "string"}, "score": map[string]interface{}{"type": "number"}, "reasons": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}}, "unblocks": map[string]interface{}{"type": "integer"}, "claim_command": map[string]interface{}{"type": "string"}, "show_command": map[string]interface{}{"type": "string"}}, "required": []string{"generated_at", "data_hash", "id", "title", "score"}},
		"robot-plan":     {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Plan Output", "description": "Dependency-respecting execution plan with parallel tracks", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "plan": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"phases": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"phase": map[string]interface{}{"type": "integer"}, "issues": map[string]interface{}{"type": "array"}}}}, "summary": map[string]interface{}{"type": "object"}}}, "status": map[string]interface{}{"type": "object"}, "usage_hints": map[string]interface{}{"type": "array"}}},
		"robot-insights": {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Insights Output", "description": "Full graph analysis metrics including PageRank, betweenness, HITS, cycles", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "Stats": map[string]interface{}{"type": "object"}, "Cycles": map[string]interface{}{"type": "array"}, "Keystones": map[string]interface{}{"type": "array"}, "Bottlenecks": map[string]interface{}{"type": "array"}, "Influencers": map[string]interface{}{"type": "array"}, "Hubs": map[string]interface{}{"type": "array"}, "Authorities": map[string]interface{}{"type": "array"}, "Orphans": map[string]interface{}{"type": "array"}, "Cores": map[string]interface{}{"type": "object"}, "Articulation": map[string]interface{}{"type": "array"}, "Slack": map[string]interface{}{"type": "object"}, "Velocity": map[string]interface{}{"type": "object"}, "status": map[string]interface{}{"type": "object"}, "advanced_insights": map[string]interface{}{"type": "object"}, "usage_hints": map[string]interface{}{"type": "array"}}},
		"robot-priority": {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Priority Output", "description": "Priority misalignment detection with recommendations", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "recommendations": map[string]interface{}{"type": "array"}, "status": map[string]interface{}{"type": "object"}, "usage_hints": map[string]interface{}{"type": "array"}}},
		"robot-graph":    {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Graph Output", "description": "Dependency graph in JSON/DOT/Mermaid format", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "format": map[string]interface{}{"type": "string", "enum": []string{"json", "dot", "mermaid"}}, "nodes": map[string]interface{}{"type": "array"}, "edges": map[string]interface{}{"type": "array"}, "stats": map[string]interface{}{"type": "object"}}},
		"robot-diff":     {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Diff Output", "description": "Changes since a historical point (commit, branch, date)", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "since": map[string]interface{}{"type": "string"}, "since_commit": map[string]interface{}{"type": "string"}, "new": map[string]interface{}{"type": "array"}, "closed": map[string]interface{}{"type": "array"}, "modified": map[string]interface{}{"type": "array"}, "cycles": map[string]interface{}{"type": "object"}}},
		"robot-alerts":   {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Alerts Output", "description": "Stale issues, blocking cascades, priority mismatches", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "alerts": map[string]interface{}{"type": "array"}, "summary": map[string]interface{}{"type": "object"}}},
		"robot-suggest":  {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Suggest Output", "description": "Smart suggestions for duplicates, dependencies, labels, cycle breaks", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "suggestions": map[string]interface{}{"type": "array"}, "counts": map[string]interface{}{"type": "object"}}},
		"robot-burndown": {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Burndown Output", "description": "Sprint burndown data with scope changes and at-risk items", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "sprint_id": map[string]interface{}{"type": "string"}, "burndown": map[string]interface{}{"type": "array"}, "scope_changes": map[string]interface{}{"type": "array"}, "at_risk": map[string]interface{}{"type": "array"}}},
		"robot-forecast": {"$schema": "https://json-schema.org/draft/2020-12/schema", "title": "Robot Forecast Output", "description": "ETA predictions with dependency-aware scheduling", "type": "object", "properties": map[string]interface{}{"generated_at": map[string]interface{}{"type": "string", "format": "date-time"}, "data_hash": map[string]interface{}{"type": "string"}, "forecasts": map[string]interface{}{"type": "array"}, "methodology": map[string]interface{}{"type": "object"}}},
		"robot-pairs": {
			"$schema":     "https://json-schema.org/draft/2020-12/schema",
			"title":       "Robot Pairs Output",
			"description": "Cross-project paired beads sharing an ID suffix across prefixes. --schema=v1 surfaces every suffix collision (noisy); --schema=v2 requires a cross-prefix dep edge as intent signal. v2 adds intent_source. See pkg/view/schemas/pair_record.v{1,2}.json for full record shapes.",
			"type":        "object",
			"properties": map[string]interface{}{
				"generated_at": map[string]interface{}{"type": "string", "format": "date-time"},
				"data_hash":    map[string]interface{}{"type": "string"},
				"schema":       map[string]interface{}{"type": "string", "enum": []string{"pair.v1", "pair.v2"}, "description": "Projection schema carried in the payload. Selected via --schema / BT_OUTPUT_SCHEMA. v1 retained for one release."},
				"pairs":        map[string]interface{}{"type": "array", "description": "PairRecord items; see pair_record.v{1,2}.json for per-record shape."},
			},
			"required": []string{"generated_at", "data_hash", "schema", "pairs"},
			"flags": map[string]interface{}{
				"--schema":   map[string]interface{}{"type": "string", "enum": []string{"v1", "v2"}, "description": "Projection schema. Default v1 in Phase 1 of bt-gkyn; flips to v2 once pair.v2 reader ships."},
				"--orphaned": map[string]interface{}{"type": "boolean", "description": "Under --schema=v1, emit a JSONL checklist (stdout) + summary (stderr) of v1-detected pairs missing the cross-prefix dep edge v2 requires. Read-only backfill helper."},
			},
		},
		"robot-refs": {
			"$schema":     "https://json-schema.org/draft/2020-12/schema",
			"title":       "Robot Refs Output",
			"description": "Cross-project bead references detected in deps, description, notes, and comments. --schema=v1 uses prefix-scoping heuristics; --schema=v2 requires a sigil per the tunable --sigils mode. v2 adds sigil_kind per record and sigil_mode on the envelope. See pkg/view/schemas/ref_record.v{1,2}.json for full record shapes.",
			"type":        "object",
			"properties": map[string]interface{}{
				"generated_at": map[string]interface{}{"type": "string", "format": "date-time"},
				"data_hash":    map[string]interface{}{"type": "string"},
				"schema":       map[string]interface{}{"type": "string", "enum": []string{"ref.v1", "ref.v2"}, "description": "Projection schema carried in the payload. Selected via --schema / BT_OUTPUT_SCHEMA. v1 retained for one release."},
				"sigil_mode":   map[string]interface{}{"type": "string", "enum": []string{"strict", "verb", "permissive"}, "description": "Active sigil mode. Present on v2 envelopes only."},
				"refs":         map[string]interface{}{"type": "array", "description": "RefRecord items; see ref_record.v{1,2}.json for per-record shape."},
			},
			"required": []string{"generated_at", "data_hash", "schema", "refs"},
			"flags": map[string]interface{}{
				"--schema": map[string]interface{}{"type": "string", "enum": []string{"v1", "v2"}, "description": "Projection schema. Default v1 in Phase 1 of bt-vxu9; flips to v2 once ref.v2 reader ships."},
				"--sigils": map[string]interface{}{"type": "string", "enum": []string{"strict", "verb", "permissive"}, "description": "Sigil recognition mode. Requires --schema=v2 (conflict errors if paired with --schema=v1)."},
			},
		},
	}

	return RobotSchemas{
		SchemaVersion: "1.0.0",
		GeneratedAt:   now,
		Envelope:      envelope,
		Commands:      commands,
	}
}
