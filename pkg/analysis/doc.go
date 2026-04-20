// Package analysis hosts the dependency-graph engine (NewAnalyzer, Analyze,
// centrality, articulation points, critical paths, cycles, impact scoring,
// k-core, PageRank, HITS) and the input-preparation primitives that feed it.
//
// The package covers two concerns:
//
//   - Graph engine: graph.go, *_cycles.go, betweenness_approx.go, etc.
//     Pure functions over []model.Issue that produce Stats and derived
//     reports.
//
//   - Input preparation: external_resolution.go and siblings. Transformations
//     that clean or enrich an []model.Issue slice before it reaches the
//     engine. These live here (not in a separate "prep" package) because they
//     are the primitives the engine consumes — co-locating them keeps the
//     graph-readying pipeline discoverable.
//
// If you are adding a new input-side transformation (label normalization, ID
// aliasing, cross-project resolution, etc.), add it here and compose it into
// the existing pipeline — do not put it somewhere else. Downstream integration
// happens at the CLI layer via rc.analysisIssues() in cmd/bt.
package analysis
