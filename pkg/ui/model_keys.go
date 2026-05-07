package ui

// Per-view key handlers were extracted from this file in bt-ift6.1 (the
// pkg/ui/keys/ foundation child of bt-ift6, which adopts bubbles/v2/key
// for unified dispatch and help surfaces).
//
// Each handler now lives in its own pkg/ui/<view>_keys.go file (board_keys.go,
// graph_keys.go, tree_keys.go, ...). The split exists so bt-ift6.3-.9 can
// each operate on disjoint files and parallel cherry-picks don't collide.
//
// Bodies were moved unchanged in .1 (still switch msg.String()) with the
// targeted exception that "tab", "<", ">" cases moved into list_keys.go
// from the global dispatcher per ADR-004 Decision 1's no-match-and-fall-
// through rule. Conversion of each handler's body to key.Matches against
// the matching pkg/ui/keys Map is the scope of bt-ift6.2-.9.
