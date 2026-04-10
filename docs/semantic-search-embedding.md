# Semantic Search — Embedding Approach (bv-9gf.1)

This document records the embedding-generation decision for `bv-9gf` (Semantic Vector Search).

## Goal

Enable natural language search over beads (title + description) by converting text into dense vectors and performing similarity search.

Constraints:
- Cross-platform (macOS/Linux/Windows) releases via GoReleaser
- Keep the `bv` binary small and fast to start
- Local-first by default (no required network)
- Deterministic + cacheable (embed once per content hash)

## Options Considered

### 1) Python subprocess (sentence-transformers)

**Summary:** Call a small Python helper to embed text with a widely used model (e.g. `all-MiniLM-L6-v2`, 384 dims).

Pros:
- High-quality embeddings (best relevance for “search by meaning”)
- Keeps Go binary small (no ML runtime bundled)
- Implementation is straightforward (spawn subprocess, pass JSON, read JSON)

Cons:
- Requires Python + packages (`sentence-transformers`) installed
- First run downloads model weights (can be slow)
- Subprocess overhead (mitigated by batching and caching)

### 2) Go-native embeddings (ONNX / ggml / other runtime)

Pros:
- Fully self-contained (no external runtime)
- Local-only, no API keys, no network

Cons:
- Significant complexity: tokenizer + model execution + cross-platform native libs
- Larger binary and/or extra shared libraries
- Harder to keep installation “curl | bash”-simple

### 3) API-based embeddings (hosted)

Pros:
- Very high quality
- Minimal local dependencies

Cons:
- Requires network + API key
- Privacy concerns (issue text leaves machine)
- Adds operational variability (rate limits, outages)

### 4) sqlite-vec (vector extension)

Pros:
- Efficient vector storage + similarity search in SQLite
- Good fit for incremental indexing in a single file

Cons:
- Does not generate embeddings (still need a model/provider)
- Requires bundling a native extension per-platform (complex releases)

## Decision (MVP)

**Primary (recommended) provider:** Python subprocess using sentence-transformers.

Rationale:
- Best relevance per engineering effort
- Keeps Go binary size small
- Fits P3 scope (optional feature with clear installation steps)

**Fallback provider:** Pure-Go hashed-token embedding.

Rationale:
- Enables deterministic tests and a “no external deps” baseline
- Provides a usable vector format for storage/index work (`bv-9gf.2`) even when Python is unavailable
- Not truly semantic, but acceptable as a fallback and for correctness tests

Implementation note:
- A minimal fallback embedder exists at `pkg/search/hash_embedder.go`.
- The chosen interface for future providers is `pkg/search/embedder.go`.

## Future Extensions

- Proposed configuration knobs (for `bv-9gf.2`/`.3`):
  - `BV_SEMANTIC_EMBEDDER=python-sentence-transformers|openai|hash|none`
  - `BV_SEMANTIC_MODEL=all-MiniLM-L6-v2` (provider-specific)
  - `BV_SEMANTIC_DIM=384` (only for non-model providers like `hash`)

- Optional API provider (e.g. `openai`) behind explicit user opt-in.
- Optional Go-native provider behind build tags once the feature proves valuable.
- Evaluate `sqlite-vec` once vector storage/indexing is stable, focusing on distribution strategy.
