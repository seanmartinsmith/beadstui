#!/bin/bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
FIXTURE="$ROOT_DIR/tests/testdata/search_hybrid.jsonl"
TMP_DIR=$(mktemp -d)
BEADS_DIR="$TMP_DIR/.beads"
QUERY="auth"
LIMIT=5

log() {
  printf "[%s] %s\n" "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$*"
}

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$BEADS_DIR"
cp "$FIXTURE" "$BEADS_DIR/beads.jsonl"

log "Using BEADS_DIR=$BEADS_DIR"
log "Fixture: $FIXTURE"
log "Query: $QUERY"
log "Limit: $LIMIT"

export BEADS_DIR
export BV_SEMANTIC_EMBEDDER=hash
export BV_SEMANTIC_DIM=384

log "Running text-only search..."
TEXT_JSON=$(bv --search "$QUERY" --search-limit "$LIMIT" --search-mode text --robot-search)
log "Text-only results (top IDs): $(echo "$TEXT_JSON" | jq -r '.results[].issue_id' | paste -sd ',' -)"

log "Running hybrid search (impact-first)..."
HYBRID_JSON=$(bv --search "$QUERY" --search-limit "$LIMIT" --search-mode hybrid --search-preset impact-first --robot-search)
log "Hybrid results (top IDs): $(echo "$HYBRID_JSON" | jq -r '.results[].issue_id' | paste -sd ',' -)"

log "Hybrid metadata: mode=$(echo "$HYBRID_JSON" | jq -r '.mode') preset=$(echo "$HYBRID_JSON" | jq -r '.preset')"
log "Data hashes: text=$(echo "$TEXT_JSON" | jq -r '.data_hash') hybrid=$(echo "$HYBRID_JSON" | jq -r '.data_hash')"

log "Top result comparison:"
TEXT_TOP=$(echo "$TEXT_JSON" | jq -r '.results[0].issue_id')
HYBRID_TOP=$(echo "$HYBRID_JSON" | jq -r '.results[0].issue_id')
log "  text=$TEXT_TOP hybrid=$HYBRID_TOP"

if [ "$TEXT_TOP" != "$HYBRID_TOP" ]; then
  log "✅ Hybrid re-ranking changed the top result"
else
  log "⚠️  Hybrid re-ranking did not change the top result"
fi

log "Text-only full JSON written to $TMP_DIR/text.json"
log "Hybrid full JSON written to $TMP_DIR/hybrid.json"

printf "%s\n" "$TEXT_JSON" > "$TMP_DIR/text.json"
printf "%s\n" "$HYBRID_JSON" > "$TMP_DIR/hybrid.json"

log "Done"
