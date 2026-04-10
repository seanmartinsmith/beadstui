.PHONY: build build-release test clean size check fmt clippy

# Development build (faster, larger)
build:
	wasm-pack build --target web --dev

# Release build (optimized)
build-release:
	wasm-pack build --target web --release
	@if command -v wasm-opt >/dev/null 2>&1; then \
		echo "Optimizing with wasm-opt..."; \
		wasm-opt -Os -o pkg/bv_graph_wasm_bg_opt.wasm pkg/bv_graph_wasm_bg.wasm; \
		mv pkg/bv_graph_wasm_bg_opt.wasm pkg/bv_graph_wasm_bg.wasm; \
	else \
		echo "wasm-opt not found, skipping optimization"; \
	fi

# Run Rust unit tests (not WASM)
test:
	cargo test

# Run WASM tests in headless browser
test-wasm:
	wasm-pack test --headless --firefox

# Clean build artifacts
clean:
	cargo clean
	rm -rf pkg/

# Check size
size: build-release
	@echo "WASM size:"
	@wc -c pkg/bv_graph_wasm_bg.wasm
	@echo "Gzipped:"
	@gzip -c pkg/bv_graph_wasm_bg.wasm | wc -c

# Format code
fmt:
	cargo fmt

# Run clippy lints
clippy:
	cargo clippy -- -D warnings

# Check without building
check:
	cargo check
