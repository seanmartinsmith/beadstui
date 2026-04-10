#!/bin/bash
# Benchmark script for bv graph analysis
# Usage:
#   ./scripts/benchmark.sh          # Run all benchmarks
#   ./scripts/benchmark.sh baseline # Save as baseline
#   ./scripts/benchmark.sh compare  # Compare against baseline

set -e

BENCHMARK_DIR="benchmarks"
BASELINE_FILE="$BENCHMARK_DIR/baseline.txt"
CURRENT_FILE="$BENCHMARK_DIR/current.txt"
BENCH_PACKAGES=(./pkg/analysis/... ./pkg/ui/... ./pkg/export/...)

mkdir -p "$BENCHMARK_DIR"

run_benchmarks() {
    echo "Running benchmarks..."
    go test -bench=. -benchmem -count=3 "${BENCH_PACKAGES[@]}" 2>&1 | tee "$CURRENT_FILE"
    echo ""
    echo "Results saved to $CURRENT_FILE"
}

save_baseline() {
    echo "Running benchmarks and saving as baseline..."
    go test -bench=. -benchmem -count=3 "${BENCH_PACKAGES[@]}" 2>&1 | tee "$BASELINE_FILE"
    echo ""
    echo "Baseline saved to $BASELINE_FILE"
}

compare_benchmarks() {
    if [ ! -f "$BASELINE_FILE" ]; then
        echo "No baseline found at $BASELINE_FILE"
        echo "Run './scripts/benchmark.sh baseline' first"
        exit 1
    fi

    run_benchmarks

    echo ""
    echo "=== Comparing against baseline ==="
    echo ""

    # Check if benchstat is available
    if command -v benchstat &> /dev/null; then
        benchstat "$BASELINE_FILE" "$CURRENT_FILE"
    else
        echo "benchstat not found. Install with: go install golang.org/x/perf/cmd/benchstat@latest"
        echo ""
        echo "Manual comparison:"
        echo "Baseline: $BASELINE_FILE"
        echo "Current:  $CURRENT_FILE"
    fi
}

# Quick benchmarks for CI (subset of critical tests)
run_quick() {
    echo "Running quick benchmarks (CI mode)..."
    go test -bench='Benchmark(FullAnalysis_(Sparse100|Dense100|ManyCycles20)|GraphModel_Rebuild_Layered1000|GraphSnapshot_BuildLayoutAndRenderSVG_Layered1000)' \
            -benchmem -count=1 "${BENCH_PACKAGES[@]}" 2>&1 | tee "$CURRENT_FILE"
}

case "${1:-run}" in
    baseline)
        save_baseline
        ;;
    compare)
        compare_benchmarks
        ;;
    quick)
        run_quick
        ;;
    run|*)
        run_benchmarks
        ;;
esac
