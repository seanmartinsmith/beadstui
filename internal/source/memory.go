package source

import (
	"encoding/json"
	"fmt"
)

// schemaVersionKey is the one confirmed non-memory sibling `bd memories
// --json` emits alongside the flat key->body map (verified live against the
// bt project's own database and its empty beads_global namespace - spec
// §4.2, §9). Its value is a JSON integer; real memory bodies are JSON
// strings, so it never round-trips as a Memory even without the name check
// below - the explicit skip-by-name is kept anyway as the documented,
// literal guard the design calls for.
const schemaVersionKey = "schema_version"

// Memory is one bd-managed memory (`bd remember`), tagged with the Origin of
// the source it was read from. This is the seam bt-2ea7t.3 aggregates
// multiple sources' memories over - see docs/design/2026-07-15-cross-project-
// read-layer-and-memories.md §4.2.
type Memory struct {
	// Key is the memory's name (bd memories --json's map key).
	Key string
	// Body is the memory's full text.
	Body string
	// Origin is the source this memory was read from.
	Origin Origin
}

// ParseMemoriesJSON parses `bd memories --json`'s wire shape - a flat
// {"<key>": "<body>", ...} object with one stray "schema_version" sibling
// (an integer, not a memory) - into Origin-tagged Memory records.
//
// Every key is skipped except when its value is a JSON string: this covers
// the confirmed schema_version sibling (an int) and defensively covers any
// future non-memory sibling bd might add with a non-string value, matching
// the codebase's general "skip the malformed record, don't hard-fail the
// read" convention (e.g. loader.ParseIssues, the Dolt readers' scan-error
// `continue`). Only a genuinely unparseable top-level payload (bd's stdout
// was not a JSON object at all) returns an error - that distinction matters
// to the adapter layer, which maps a parse error to an "unavailable" source
// (spec §8) rather than a silent empty result.
func ParseMemoriesJSON(data []byte, origin Origin) ([]Memory, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse `bd memories --json` output: %w", err)
	}

	memories := make([]Memory, 0, len(raw))
	for key, val := range raw {
		if key == schemaVersionKey {
			continue
		}
		var body string
		if err := json.Unmarshal(val, &body); err != nil {
			// Not a string value - not a memory body. Skip rather than fail
			// the whole read (see func doc).
			continue
		}
		memories = append(memories, Memory{Key: key, Body: body, Origin: origin})
	}
	return memories, nil
}
