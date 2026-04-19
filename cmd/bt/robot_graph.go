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
			"bt --robot-triage               # Full triage with recommendations",
			"bt --robot-next                  # Single top pick for immediate work",
			"bt --robot-plan                  # Dependency-respecting execution plan",
			"bt --robot-insights              # Deep graph analysis (PageRank, betweenness, etc.)",
			"bt --robot-triage-by-track       # Parallel work streams for multi-agent coordination",
			"bt --robot-schema                # JSON Schema definitions for all commands",
		},
		"data_source": ".beads/issues.jsonl and git history (correlations)",
		"output_modes": map[string]string{
			"json": "Default structured output",
			"toon": "Token-optimized notation (saves ~30-50% tokens)",
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
			Flag: "--robot-triage", Description: "Unified triage: top picks, recommendations, quick wins, blockers, project health, velocity.",
			KeyFields:   []string{"triage.quick_ref.top_picks", "triage.recommendations", "triage.quick_wins", "triage.blockers_to_clear", "triage.project_health"},
			NeedsIssues: true,
		},
		"robot-next": {
			Flag: "--robot-next", Description: "Single top recommendation with claim/show commands.",
			KeyFields:   []string{"id", "title", "score", "reasons", "unblocks", "claim_command", "show_command"},
			NeedsIssues: true,
		},
		"robot-plan": {
			Flag: "--robot-plan", Description: "Dependency-respecting execution plan with parallel tracks.",
			KeyFields:   []string{"tracks", "items", "unblocks", "summary"},
			NeedsIssues: true,
		},
		"robot-insights": {
			Flag: "--robot-insights", Description: "Deep graph analysis: PageRank, betweenness, HITS, eigenvector, k-core, cycle detection.",
			KeyFields:   []string{"pagerank", "betweenness", "hits", "eigenvector", "k_core", "cycles"},
			NeedsIssues: true,
		},
		"robot-priority": {
			Flag: "--robot-priority", Description: "Priority misalignment detection: items whose graph importance differs from assigned priority.",
			KeyFields:   []string{"misalignments", "suggestions"},
			NeedsIssues: true,
		},
		"robot-triage-by-track": {
			Flag: "--robot-triage-by-track", Description: "Triage grouped by independent parallel execution tracks.",
			KeyFields:   []string{"tracks[].track_id", "tracks[].top_pick", "tracks[].items"},
			NeedsIssues: true,
		},
		"robot-triage-by-label": {
			Flag: "--robot-triage-by-label", Description: "Triage grouped by label for area-focused agents.",
			KeyFields:   []string{"labels[].label", "labels[].top_pick", "labels[].items"},
			NeedsIssues: true,
		},
		"robot-alerts": {
			Flag: "--robot-alerts", Description: "Stale issues, blocking cascades, priority mismatches.",
			KeyFields:   []string{"alerts", "severity", "affected_issues"},
			Params:      []string{"--severity info|warning|critical", "--alert-type <type>", "--alert-label <label>"},
			NeedsIssues: true,
		},
		"robot-suggest": {
			Flag: "--robot-suggest", Description: "Smart suggestions: potential duplicates, missing dependencies, label assignments, cycle warnings.",
			KeyFields:   []string{"suggestions", "type", "confidence"},
			Params:      []string{"--suggest-type duplicate|dependency|label|cycle", "--suggest-confidence 0.0-1.0", "--suggest-bead <id>"},
			NeedsIssues: true,
		},
		"robot-schema": {
			Flag: "--robot-schema", Description: "JSON Schema definitions for all robot command outputs.",
			KeyFields:   []string{"schema_version", "envelope", "commands"},
			Params:      []string{"--schema-command <cmd>"},
			NeedsIssues: false,
		},
		"robot-docs": {
			Flag: "--robot-docs <topic>", Description: "Machine-readable JSON documentation. Topics: guide, commands, examples, env, exit-codes, all.",
			NeedsIssues: false,
		},
		"robot-history": {
			Flag: "--robot-history", Description: "Bead-to-commit correlations from git history.",
			KeyFields:   []string{"correlations", "confidence", "commit_sha", "bead_id"},
			Params:      []string{"--bead-history <id>", "--history-since <date>", "--history-limit <n>", "--min-confidence 0.0-1.0"},
			NeedsIssues: true,
		},
		"robot-diff": {
			Flag: "--robot-diff", Description: "Changes since a historical point (commit, branch, tag, or date).",
			Params:      []string{"--diff-since <ref>"},
			NeedsIssues: true,
		},
		"robot-search": {
			Flag: "--robot-search", Description: "Semantic vector search over issue titles and descriptions.",
			Params:      []string{"--search <query>", "--search-limit <n>", "--search-mode text|hybrid"},
			NeedsIssues: true,
		},
		"robot-label-health": {
			Flag: "--robot-label-health", Description: "Per-label health metrics: open/closed counts, velocity, staleness.",
			NeedsIssues: true,
		},
		"robot-label-flow": {
			Flag: "--robot-label-flow", Description: "Cross-label dependency flow analysis.",
			NeedsIssues: true,
		},
		"robot-label-attention": {
			Flag: "--robot-label-attention", Description: "Attention-ranked labels requiring focus.",
			Params:      []string{"--attention-limit <n>"},
			NeedsIssues: true,
		},
		"robot-graph": {
			Flag: "--robot-graph", Description: "Dependency graph export in JSON, DOT, or Mermaid format.",
			Params:      []string{"--graph-format json|dot|mermaid", "--graph-root <id>", "--graph-depth <n>"},
			NeedsIssues: true,
		},
		"robot-metrics": {
			Flag: "--robot-metrics", Description: "Performance metrics: timing, cache hit rates, memory usage.",
			NeedsIssues: true,
		},
		"robot-orphans": {
			Flag: "--robot-orphans", Description: "Orphan commit candidates that should be linked to beads.",
			Params:      []string{"--orphans-min-score 0-100"},
			NeedsIssues: true,
		},
		"robot-file-beads": {
			Flag: "--robot-file-beads <path>", Description: "Beads that touched a specific file path.",
			Params:      []string{"--file-beads-limit <n>"},
			NeedsIssues: true,
		},
		"robot-file-hotspots": {
			Flag: "--robot-file-hotspots", Description: "Files touched by the most beads.",
			Params:      []string{"--hotspots-limit <n>"},
			NeedsIssues: true,
		},
		"robot-file-relations": {
			Flag: "--robot-file-relations <path>", Description: "Files that frequently co-change with a given file.",
			Params:      []string{"--relations-threshold 0.0-1.0", "--relations-limit <n>"},
			NeedsIssues: true,
		},
		"robot-related": {
			Flag: "--robot-related <id>", Description: "Beads related to a specific bead ID.",
			Params:      []string{"--related-min-relevance 0-100", "--related-max-results <n>", "--related-include-closed"},
			NeedsIssues: true,
		},
		"robot-blocker-chain": {
			Flag: "--robot-blocker-chain <id>", Description: "Full blocker chain analysis for an issue.",
			NeedsIssues: true,
		},
		"robot-impact-network": {
			Flag: "--robot-impact-network [<id>|all]", Description: "Impact network graph (full or subnetwork for a bead).",
			Params:      []string{"--network-depth 1-3"},
			NeedsIssues: true,
		},
		"robot-causality": {
			Flag: "--robot-causality <id>", Description: "Causal chain analysis for a bead.",
			NeedsIssues: true,
		},
		"robot-sprint-list": {
			Flag: "--robot-sprint-list", Description: "List all sprints as JSON.",
			NeedsIssues: true,
		},
		"robot-sprint-show": {
			Flag: "--robot-sprint-show <id>", Description: "Show details for a specific sprint.",
			NeedsIssues: true,
		},
		"robot-forecast": {
			Flag: "--robot-forecast <id|all>", Description: "ETA predictions for bead completion.",
			Params:      []string{"--forecast-label <label>", "--forecast-sprint <id>", "--forecast-agents <n>"},
			NeedsIssues: true,
		},
		"robot-capacity": {
			Flag: "--robot-capacity", Description: "Capacity simulation and completion projections.",
			Params:      []string{"--agents <n>", "--capacity-label <label>"},
			NeedsIssues: true,
		},
		"robot-burndown": {
			Flag: "--robot-burndown <sprint|current>", Description: "Sprint burndown data.",
			NeedsIssues: true,
		},
		"robot-drift": {
			Flag: "--robot-drift", Description: "Drift detection from saved baseline.",
			NeedsIssues: true,
		},
	}

	examples := []map[string]string{
		{"description": "Get top 3 picks for immediate work", "command": "bt --robot-triage | jq '.triage.quick_ref.top_picks[:3]'"},
		{"description": "Claim the top recommendation", "command": "bt --robot-next | jq -r '.claim_command' | sh"},
		{"description": "Find high-impact blockers to clear", "command": "bt --robot-triage | jq '.triage.blockers_to_clear | map(.id)'"},
		{"description": "Get bug-only recommendations", "command": "bt --robot-triage | jq '.triage.recommendations[] | select(.type == \"bug\")'"},
		{"description": "Multi-agent: top pick per parallel track", "command": "bt --robot-triage-by-track | jq '.triage.recommendations_by_track[].top_pick'"},
		{"description": "Find beads related to a specific file", "command": "bt --robot-file-beads src/main.rs"},
		{"description": "Search for issues by keyword", "command": "bt --search 'authentication' --robot-search"},
		{"description": "Get TOON output (saves tokens)", "command": "bt --robot-triage --format toon"},
		{"description": "Use env for default format", "command": "BT_OUTPUT_FORMAT=toon bt --robot-triage"},
		{"description": "Show token savings estimate", "command": "bt --robot-triage --format toon --stats"},
	}

	envVars := map[string]string{
		"BT_OUTPUT_FORMAT":    "Default output format: json or toon (overridden by --format)",
		"TOON_DEFAULT_FORMAT": "Fallback format if BT_OUTPUT_FORMAT not set",
		"TOON_STATS":          "Set to 1 to show JSON vs TOON token estimates on stderr",
		"TOON_KEY_FOLDING":    "TOON key folding mode",
		"TOON_INDENT":         "TOON indentation level (0-16)",
		"BT_PRETTY_JSON":      "Set to 1 for indented JSON output",
		"BT_ROBOT":            "Set to 1 to force robot mode (clean stdout)",
		"BT_SEARCH_MODE":      "Search mode: text or hybrid",
		"BT_SEARCH_PRESET":    "Hybrid search preset name",
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
	}

	return RobotSchemas{
		SchemaVersion: "1.0.0",
		GeneratedAt:   now,
		Envelope:      envelope,
		Commands:      commands,
	}
}
