#!/bin/bash
# Benchmark script for hybrid search performance
# Usage:
#   ./scripts/benchmark_compare.sh          # Run benchmarks
#   ./scripts/benchmark_compare.sh baseline # Save as baseline
#   ./scripts/benchmark_compare.sh compare  # Compare against baseline
#   ./scripts/benchmark_compare.sh quick    # Quick subset

set -e

BENCHMARK_DIR="benchmarks"
BASELINE_FILE="$BENCHMARK_DIR/search_baseline.txt"
CURRENT_FILE="$BENCHMARK_DIR/search_current.txt"
BENCH_PACKAGES=(./pkg/search/... ./tests/e2e/...)

mkdir -p "$BENCHMARK_DIR"

run_benchmarks() {
    echo "Running hybrid search benchmarks..."
    go test -run=^$ -bench=. -benchmem -count=3 "${BENCH_PACKAGES[@]}" 2>&1 | tee "$CURRENT_FILE"
    echo ""
    echo "Results saved to $CURRENT_FILE"
}

save_baseline() {
    echo "Running benchmarks and saving as baseline..."
    go test -run=^$ -bench=. -benchmem -count=3 "${BENCH_PACKAGES[@]}" 2>&1 | tee "$BASELINE_FILE"
    echo ""
    echo "Baseline saved to $BASELINE_FILE"
}

compare_benchmarks() {
    if [ ! -f "$BASELINE_FILE" ]; then
        echo "No baseline found at $BASELINE_FILE"
        echo "Run './scripts/benchmark_compare.sh baseline' first"
        exit 1
    fi

    run_benchmarks

    echo ""
    echo "=== Comparing against baseline ==="
    echo ""

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

run_quick() {
    echo "Running quick hybrid search benchmarks..."
    go test -run=^$ -bench='Benchmark(SearchTextVsHybrid/|SearchAtScale/n=100$|HybridScorerScore$|MetricsCacheGet$)' \
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
