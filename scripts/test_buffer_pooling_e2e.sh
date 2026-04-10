#!/bin/bash
# test_buffer_pooling_e2e.sh
# Comprehensive E2E test for buffer pooling optimization
#
# Usage: ./scripts/test_buffer_pooling_e2e.sh
#
# Exit codes:
#   0 - All tests passed
#   1 - Test failure
#   2 - Build failure

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_FILE="${PROJECT_ROOT}/test_results_e2e_$(date +%Y%m%d_%H%M%S).log"

# Test thresholds based on baseline + expected improvement
# Baseline Sample100: ~199,548 allocs, ~29.5MB
# After pooling: ~149,262 allocs (25% reduction), ~11.8MB (60% reduction)
ALLOC_THRESHOLD=160000  # Allow 7% margin above observed 149,262
MEM_THRESHOLD_MB=15     # Allow 27% margin above observed 11.8MB

# ============================================================================
# Logging Functions
# ============================================================================
log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    echo "$msg" | tee -a "$LOG_FILE"
}

log_section() {
    echo "" | tee -a "$LOG_FILE"
    echo "============================================================" | tee -a "$LOG_FILE"
    log "$1"
    echo "============================================================" | tee -a "$LOG_FILE"
}

log_pass() {
    log "PASS: $1"
}

log_fail() {
    log "FAIL: $1"
}

# ============================================================================
# Test 1: Build and Unit Tests
# ============================================================================
test_build_and_unit() {
    log_section "Test 1: Build and Unit Tests"

    cd "$PROJECT_ROOT"

    log "Building project..."
    if ! go build ./...; then
        log_fail "Build failed"
        return 1
    fi
    log_pass "Build succeeded"

    log "Running unit tests..."
    if ! go test ./pkg/analysis/... -count=1 -v 2>&1 | tee -a "$LOG_FILE" | tail -5; then
        log_fail "Unit tests failed"
        return 1
    fi
    log_pass "Unit tests passed"

    return 0
}

# ============================================================================
# Test 2: Race Condition Detection
# ============================================================================
test_race_detection() {
    log_section "Test 2: Race Condition Detection"

    cd "$PROJECT_ROOT"

    log "Running tests with race detector..."
    if ! go test -race ./pkg/analysis/... -run "TestBuffer|TestApprox" -count=1 2>&1 | tee -a "$LOG_FILE" | tail -10; then
        log_fail "Race detection test failed - potential data race"
        return 1
    fi
    log_pass "No race conditions detected"

    return 0
}

# ============================================================================
# Test 3: Allocation Threshold Verification
# ============================================================================
test_allocation_threshold() {
    log_section "Test 3: Allocation Threshold Verification"

    cd "$PROJECT_ROOT"

    log "Running allocation benchmark..."
    local bench_output
    bench_output=$(go test -bench="BenchmarkApproxBetweenness_500nodes_Sample100" \
                          -benchmem -count=1 ./pkg/analysis/... 2>&1)
    echo "$bench_output" | tee -a "$LOG_FILE"

    # Extract allocations/op from output (field before "allocs/op")
    local allocs
    allocs=$(echo "$bench_output" | grep "BenchmarkApproxBetweenness_500nodes_Sample100" | awk '{for(i=1;i<=NF;i++) if($i ~ /allocs\/op/) print $(i-1)}' | head -1)

    if [[ -z "$allocs" ]]; then
        log_fail "Could not extract allocation count from benchmark"
        return 1
    fi

    log "Allocations per operation: $allocs (threshold: $ALLOC_THRESHOLD)"

    if [[ "$allocs" -gt "$ALLOC_THRESHOLD" ]]; then
        log_fail "Allocations ($allocs) exceed threshold ($ALLOC_THRESHOLD)"
        return 1
    fi
    log_pass "Allocations within threshold"

    # Extract memory/op (field before "B/op")
    local mem_bytes
    mem_bytes=$(echo "$bench_output" | grep "BenchmarkApproxBetweenness_500nodes_Sample100" | awk '{for(i=1;i<=NF;i++) if($i ~ /B\/op/) print $(i-1)}' | head -1)

    if [[ -z "$mem_bytes" ]]; then
        log_fail "Could not extract memory bytes from benchmark"
        return 1
    fi

    local mem_mb=$((mem_bytes / 1048576))

    log "Memory per operation: ${mem_mb}MB (threshold: ${MEM_THRESHOLD_MB}MB)"

    if [[ "$mem_mb" -gt "$MEM_THRESHOLD_MB" ]]; then
        log_fail "Memory (${mem_mb}MB) exceeds threshold (${MEM_THRESHOLD_MB}MB)"
        return 1
    fi
    log_pass "Memory within threshold"

    return 0
}

# ============================================================================
# Test 4: Determinism Verification
# ============================================================================
test_determinism() {
    log_section "Test 4: Determinism Verification"

    cd "$PROJECT_ROOT"

    log "Running determinism tests..."
    if ! go test ./pkg/analysis/... -run "TestApproxBetweenness" -count=3 -v 2>&1 | tee -a "$LOG_FILE" | tail -10; then
        log_fail "Determinism test failed"
        return 1
    fi
    log_pass "Results are deterministic"

    return 0
}

# ============================================================================
# Test 5: Buffer Pool Correctness
# ============================================================================
test_buffer_pool_correctness() {
    log_section "Test 5: Buffer Pool Correctness"

    cd "$PROJECT_ROOT"

    log "Running buffer pool correctness tests..."
    local tests=(
        "TestBrandesBuffersInitialization"
        "TestResetClearsAllValues"
        "TestResetRetainsPredCapacity"
        "TestResetTriggersClearOnOversizedMaps"
        "TestPoolReturnsNonNilBuffer"
        "TestPoolPreallocation"
        "TestResetEquivalentToFreshAllocation"
    )

    for test in "${tests[@]}"; do
        log "  Running $test..."
        local test_output
        test_output=$(go test ./pkg/analysis/... -run "$test" -v 2>&1)
        echo "$test_output" >> "$LOG_FILE"
        if ! echo "$test_output" | command grep -q "^--- PASS"; then
            log_fail "$test failed"
            return 1
        fi
    done

    log_pass "All buffer pool correctness tests passed"
    return 0
}

# ============================================================================
# Test 6: Concurrent Stress Test
# ============================================================================
test_concurrent_stress() {
    log_section "Test 6: Concurrent Stress Test"

    cd "$PROJECT_ROOT"

    log "Running concurrent stress tests with race detector..."
    if ! go test -race ./pkg/analysis/... -run "TestBufferPoolConcurrentAccess|TestConcurrentPoolGetPut" -count=5 2>&1 | tee -a "$LOG_FILE" | tail -15; then
        log_fail "Concurrent stress test failed"
        return 1
    fi
    log_pass "Concurrent stress tests passed"

    return 0
}

# ============================================================================
# Main
# ============================================================================
main() {
    log_section "Buffer Pooling E2E Test Suite"
    log "Project root: $PROJECT_ROOT"
    log "Log file: $LOG_FILE"
    log "Git SHA: $(git rev-parse HEAD 2>/dev/null || echo 'unknown')"
    log ""

    local failed=0
    local passed=0
    local total=0

    # Run all tests
    tests=(
        "test_build_and_unit"
        "test_race_detection"
        "test_allocation_threshold"
        "test_determinism"
        "test_buffer_pool_correctness"
        "test_concurrent_stress"
    )

    for test_func in "${tests[@]}"; do
        total=$((total + 1))
        if $test_func; then
            passed=$((passed + 1))
        else
            failed=$((failed + 1))
        fi
    done

    # Summary
    log_section "Test Summary"
    log "Total: $total"
    log "Passed: $passed"
    log "Failed: $failed"
    log ""
    log "Log file: $LOG_FILE"

    if [[ $failed -gt 0 ]]; then
        log "OVERALL: FAILED"
        return 1
    fi

    log "OVERALL: PASSED"
    return 0
}

main "$@"
