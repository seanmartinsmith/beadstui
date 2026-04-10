#!/bin/bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WASM_SRC="$ROOT_DIR/pkg/export/wasm_scorer"
WASM_OUT="$ROOT_DIR/pkg/export/viewer_assets/wasm"

if ! command -v wasm-pack >/dev/null 2>&1; then
  echo "wasm-pack not found. Install with: cargo install wasm-pack" >&2
  exit 1
fi

if [ ! -d "$WASM_SRC" ]; then
  echo "WASM source directory not found: $WASM_SRC" >&2
  exit 1
fi

mkdir -p "$WASM_OUT"

(cd "$WASM_SRC" && wasm-pack build --release --target web --out-dir "$WASM_OUT")

echo "Hybrid WASM assets built to $WASM_OUT"
