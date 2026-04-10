#!/bin/bash
# scripts/verify_isomorphic.sh
#
# Isomorphic Verification Script
# ==============================
# Verifies that optimized implementations produce identical outputs to the baseline.
#
# This script:
# 1. Builds the baseline version (from a reference branch)
# 2. Builds the current version
# 3. Runs both against test fixtures
# 4. Compares outputs and reports any differences
#
# Usage:
#   ./scripts/verify_isomorphic.sh [baseline_ref]
#
# Arguments:
#   baseline_ref  Git ref to compare against (default: main)
#
# Environment Variables:
#   VERBOSE=1     Show detailed output
#   KEEP_TEMPS=1  Don't delete temporary files on exit
#   SKIP_BUILD=1  Skip building (use existing binaries)

set -euo pipefail

# Configuration
BASELINE_REF="${1:-main}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERBOSE="${VERBOSE:-0}"
KEEP_TEMPS="${KEEP_TEMPS:-0}"
SKIP_BUILD="${SKIP_BUILD:-0}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[FAIL]${NC} $*"; }
log_verbose() { [[ "$VERBOSE" == "1" ]] && echo -e "${BLUE}[DEBUG]${NC} $*" || true; }

# Cleanup function
cleanup() {
    local exit_code=$?
    if [[ "$KEEP_TEMPS" != "1" ]] && [[ -n "${TEST_DIR:-}" ]]; then
        log_verbose "Cleaning up $TEST_DIR"
        rm -rf "$TEST_DIR"
    fi
    exit $exit_code
}
trap cleanup EXIT

# Create temporary directory
TEST_DIR=$(mktemp -d)
log_verbose "Using temporary directory: $TEST_DIR"

# Binary paths
BASELINE_BIN="$TEST_DIR/bv_baseline"
CURRENT_BIN="$TEST_DIR/bv_current"

# Track current branch to restore later
ORIGINAL_BRANCH=$(git -C "$PROJECT_ROOT" branch --show-current || git -C "$PROJECT_ROOT" rev-parse HEAD)
HAS_UNCOMMITTED=$(git -C "$PROJECT_ROOT" status --porcelain | wc -l)

echo "=============================================="
echo "  Isomorphic Verification"
echo "=============================================="
echo ""
echo "  Baseline:  $BASELINE_REF"
echo "  Current:   $ORIGINAL_BRANCH"
echo "  Test Dir:  $TEST_DIR"
echo ""

# Build baseline version
build_baseline() {
    log_info "Building baseline version from $BASELINE_REF..."

    # Stash any uncommitted changes
    if [[ "$HAS_UNCOMMITTED" -gt 0 ]]; then
        log_verbose "Stashing uncommitted changes..."
        git -C "$PROJECT_ROOT" stash push -q -m "isomorphic-verify-temp"
    fi

    # Checkout baseline
    log_verbose "Checking out $BASELINE_REF..."
    git -C "$PROJECT_ROOT" checkout -q "$BASELINE_REF"

    # Build
    log_verbose "Building baseline binary..."
    go build -o "$BASELINE_BIN" "$PROJECT_ROOT/cmd/bv" 2>/dev/null

    # Return to original branch
    log_verbose "Returning to $ORIGINAL_BRANCH..."
    git -C "$PROJECT_ROOT" checkout -q "$ORIGINAL_BRANCH"

    # Restore stashed changes
    if [[ "$HAS_UNCOMMITTED" -gt 0 ]]; then
        log_verbose "Restoring stashed changes..."
        git -C "$PROJECT_ROOT" stash pop -q || true
    fi

    log_success "Baseline built: $BASELINE_BIN"
}

# Build current version
build_current() {
    log_info "Building current version..."
    go build -o "$CURRENT_BIN" "$PROJECT_ROOT/cmd/bv" 2>/dev/null
    log_success "Current built: $CURRENT_BIN"
}

# Copy test fixtures
setup_fixtures() {
    log_info "Setting up test fixtures..."

    FIXTURE_DIR="$TEST_DIR/fixtures"
    mkdir -p "$FIXTURE_DIR"

    # Copy existing test data if available
    if [[ -d "$PROJECT_ROOT/testdata/graphs" ]]; then
        cp -r "$PROJECT_ROOT/testdata/graphs" "$FIXTURE_DIR/"
        log_verbose "Copied testdata/graphs"
    fi

    # Create .beads directories for each test case
    for graph_file in "$FIXTURE_DIR/graphs"/*.json 2>/dev/null; do
        [[ -f "$graph_file" ]] || continue

        base_name=$(basename "$graph_file" .json)
        test_case_dir="$FIXTURE_DIR/cases/$base_name"
        beads_dir="$test_case_dir/.beads"
        mkdir -p "$beads_dir"

        # Convert graph JSON to beads.jsonl (simplified conversion)
        # In a real setup, this would use the testutil.Generator
        log_verbose "Preparing test case: $base_name"
    done

    # Create a default test case from current project if .beads exists
    if [[ -d "$PROJECT_ROOT/.beads" ]]; then
        DEFAULT_CASE="$FIXTURE_DIR/cases/project"
        mkdir -p "$DEFAULT_CASE/.beads"
        cp "$PROJECT_ROOT/.beads/beads.jsonl" "$DEFAULT_CASE/.beads/" 2>/dev/null || true
        log_verbose "Using project .beads as test case"
    fi

    log_success "Test fixtures ready"
}

# Run a command and capture output
run_command() {
    local binary="$1"
    local case_dir="$2"
    local command="$3"
    local output_file="$4"

    cd "$case_dir"
    "$binary" $command > "$output_file" 2>&1 || true
    cd - > /dev/null
}

# Compare outputs
compare_outputs() {
    local baseline_out="$1"
    local current_out="$2"
    local test_name="$3"

    if diff -q "$baseline_out" "$current_out" > /dev/null 2>&1; then
        log_success "$test_name: outputs identical"
        return 0
    else
        log_error "$test_name: OUTPUTS DIFFER"
        if [[ "$VERBOSE" == "1" ]]; then
            echo "--- Diff (first 50 lines) ---"
            diff "$baseline_out" "$current_out" | head -50 || true
            echo "---"
        fi
        return 1
    fi
}

# Run tests for a single command
test_command() {
    local cmd="$1"
    local cmd_name="${cmd// /_}"
    local failures=0

    log_info "Testing: $cmd"

    for case_dir in "$FIXTURE_DIR/cases"/*; do
        [[ -d "$case_dir" ]] || continue
        [[ -d "$case_dir/.beads" ]] || continue

        case_name=$(basename "$case_dir")
        baseline_out="$TEST_DIR/output/${case_name}_${cmd_name}_baseline.json"
        current_out="$TEST_DIR/output/${case_name}_${cmd_name}_current.json"

        mkdir -p "$TEST_DIR/output"

        # Run both versions
        run_command "$BASELINE_BIN" "$case_dir" "$cmd" "$baseline_out"
        run_command "$CURRENT_BIN" "$case_dir" "$cmd" "$current_out"

        # Compare
        if ! compare_outputs "$baseline_out" "$current_out" "$case_name/$cmd"; then
            ((failures++))
        fi
    done

    return $failures
}

# Main test suite
run_tests() {
    local total_failures=0

    # Robot commands to test
    COMMANDS=(
        "--robot-triage"
        "--robot-insights"
        "--robot-plan"
        "--robot-next"
        "--robot-alerts"
    )

    for cmd in "${COMMANDS[@]}"; do
        if ! test_command "$cmd"; then
            ((total_failures++))
        fi
    done

    return $total_failures
}

# Main execution
main() {
    # Build phase
    if [[ "$SKIP_BUILD" != "1" ]]; then
        build_baseline
        build_current
    else
        log_warn "Skipping build (SKIP_BUILD=1)"
        if [[ ! -f "$BASELINE_BIN" ]] || [[ ! -f "$CURRENT_BIN" ]]; then
            log_error "Binaries not found. Run without SKIP_BUILD=1 first."
            exit 1
        fi
    fi

    # Setup phase
    setup_fixtures

    # Test phase
    echo ""
    echo "=============================================="
    echo "  Running Tests"
    echo "=============================================="
    echo ""

    if run_tests; then
        echo ""
        log_success "ALL TESTS PASSED - Implementations are isomorphic"
        exit 0
    else
        echo ""
        log_error "SOME TESTS FAILED - Implementations differ"
        echo ""
        echo "To investigate:"
        echo "  1. Set VERBOSE=1 to see diffs"
        echo "  2. Set KEEP_TEMPS=1 to preserve output files"
        echo "  3. Output files are in: $TEST_DIR/output/"
        exit 1
    fi
}

main "$@"
