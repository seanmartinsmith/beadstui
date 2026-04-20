// Package view hosts graph-derived projections of domain types for agent and
// UI consumption. A projection is a narrower, consumer-facing shape computed
// from one or more canonical types in pkg/model plus (optionally) graph data
// from pkg/analysis.
//
// Every projection in this package follows the same conventions so that
// future projections (portfolio records, pair records, reference records,
// etc.) stay boring and reviewable.
//
// # Projection pattern
//
//  1. File per projection. One struct, one file, named after the projection
//     (compact_issue.go, portfolio_record.go, ...). Keep the file scoped to
//     the projection and its constructor.
//
//  2. Struct + constructor. The projection is a plain struct with json tags
//     (omitempty where appropriate) and a free-function constructor that
//     takes upstream domain types and returns the projection. Do not put
//     projection methods on pkg/model types: projections are graph-derived
//     consumer surfaces, not domain concerns.
//
//  3. Schema version constant. Each projection file declares a constant
//     named <Projection>SchemaV1 (e.g. CompactIssueSchemaV1 = "compact.v1").
//     Robot subcommands surface this on the output envelope so agents can
//     discover the shape they received.
//
//  4. Versioning policy. Additive changes (new fields with omitempty) keep
//     the existing version. Rename / remove / type change bumps the schema
//     to v2. A golden-file update without a version bump is a red-flag
//     signal in code review.
//
//  5. Golden-file harness. Projections are tested by round-tripping fixture
//     issues through the constructor and comparing the JSON output to a
//     committed golden file. Fixtures live at testdata/fixtures/*.json,
//     golden outputs at testdata/golden/*.json. Regenerate with:
//
//     GENERATE_GOLDEN=1 go test ./pkg/view/...
//
//  6. JSON schema. Each projection commits a JSON Schema document at
//     schemas/<projection>.v<n>.json describing the wire shape for external
//     consumers.
//
// # Dependency rule
//
// pkg/view may import pkg/model and pkg/analysis. It must NOT import cmd/bt
// or any CLI/TUI surface. Projections are pure and callable from any
// consumer (CLI, TUI, WASM, tests).
//
// # Scope boundary
//
// Projections apply only to the types they name. In particular, the compact
// issue projection applies to []model.Issue slots; it does not apply to
// drift.Alert, correlation records, or decision records. If compact Alert or
// Correlation views become valuable, they get their own projection file and
// their own schema version in this package.
package view
