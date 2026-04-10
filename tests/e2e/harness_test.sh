#!/usr/bin/env bash
# Tests for the E2E harness functionality.
# Run: bash tests/e2e/harness_test.sh

set -euo pipefail
cd "$(dirname "$0")/../.."

# Disable auto-init for controlled testing
export BV_E2E_NO_INIT=1

# Source the harness
source tests/e2e/harness.sh

# Track test results
HARNESS_TEST_PASS=0
HARNESS_TEST_FAIL=0

pass() {
  (( ++HARNESS_TEST_PASS )) || true
  echo "  ✓ $1"
}

fail() {
  (( ++HARNESS_TEST_FAIL )) || true
  echo "  ✗ $1: $2"
}

echo "=== Harness Unit Tests ==="
echo

# Test: ts() returns ISO-8601 format
echo "--- Timestamp tests ---"
ts_output=$(ts)
[[ "$ts_output" == *"T"* ]] && pass "ts() has T separator" || fail "ts() has T separator" "$ts_output"
[[ "$ts_output" == *"Z"* ]] && pass "ts() has Z suffix" || fail "ts() has Z suffix" "$ts_output"

epoch_output=$(ts_epoch_ms)
[[ "$epoch_output" =~ ^[0-9]+$ ]] && pass "ts_epoch_ms() is numeric" || fail "ts_epoch_ms() is numeric" "$epoch_output"

# Test: log level filtering
echo
echo "--- Log level tests ---"
export BV_E2E_LOG_LEVEL=INFO
_should_log ERROR && pass "ERROR passes INFO threshold" || fail "ERROR passes" ""
_should_log INFO && pass "INFO passes INFO threshold" || fail "INFO passes" ""
_should_log DEBUG 2>/dev/null && fail "DEBUG should fail INFO threshold" "" || pass "DEBUG filtered at INFO level"

export BV_E2E_LOG_LEVEL=DEBUG
_should_log DEBUG && pass "DEBUG passes DEBUG threshold" || fail "DEBUG passes" ""
export BV_E2E_LOG_LEVEL=INFO

# Test: JSON log format
echo
echo "--- JSON format tests ---"
export BV_E2E_LOG_FORMAT=json
json_output=$(_log INFO "test message" 2>&1)
[[ "$json_output" == *'"ts":'* ]] && pass "JSON has ts field" || fail "JSON has ts" "$json_output"
[[ "$json_output" == *'"level":"INFO"'* ]] && pass "JSON has level field" || fail "JSON has level" "$json_output"
[[ "$json_output" == *'"msg":"test message"'* ]] && pass "JSON has msg field" || fail "JSON has msg" "$json_output"
export BV_E2E_LOG_FORMAT=text

# Test: context capture
echo
echo "--- Context capture tests ---"
context_json=$(_capture_context)
[[ "$context_json" == *'"working_dir":'* ]] && pass "context has working_dir" || fail "context has working_dir" ""
[[ "$context_json" == *'"branch":'* ]] && pass "context has git.branch" || fail "context has git.branch" ""
[[ "$context_json" == *'"environment":'* ]] && pass "context has environment" || fail "context has environment" ""

# Test: test lifecycle
echo
echo "--- Test lifecycle tests ---"
_e2e_init 2>/dev/null

test_start "lifecycle_test" 2>/dev/null
[[ "$BV_E2E_TEST_NAME" == "lifecycle_test" ]] && pass "test name set" || fail "test name set" "$BV_E2E_TEST_NAME"
[[ "$BV_E2E_TEST_PHASE" == "setup" ]] && pass "test phase set" || fail "test phase set" "$BV_E2E_TEST_PHASE"

test_phase "execution"
[[ "$BV_E2E_TEST_PHASE" == "execution" ]] && pass "phase updated" || fail "phase updated" "$BV_E2E_TEST_PHASE"

BV_E2E_PASS_COUNT=0
BV_E2E_RESULTS=()
test_pass "lifecycle_test" 2>/dev/null
[[ "$BV_E2E_PASS_COUNT" == "1" ]] && pass "pass count incremented" || fail "pass count" "$BV_E2E_PASS_COUNT"
[[ -z "$BV_E2E_TEST_NAME" ]] && pass "test name cleared" || fail "test name cleared" "$BV_E2E_TEST_NAME"

# Test fail tracking
test_start "fail_test" 2>/dev/null
BV_E2E_FAIL_COUNT=0
test_fail "fail_test" "test reason" 2>/dev/null
[[ "$BV_E2E_FAIL_COUNT" == "1" ]] && pass "fail count incremented" || fail "fail count" "$BV_E2E_FAIL_COUNT"

# Test skip tracking
test_start "skip_test" 2>/dev/null
BV_E2E_SKIP_COUNT=0
test_skip "skip_test" "skipped" 2>/dev/null
[[ "$BV_E2E_SKIP_COUNT" == "1" ]] && pass "skip count incremented" || fail "skip count" "$BV_E2E_SKIP_COUNT"

# Test: run function creates artifacts
echo
echo "--- Run function tests ---"
test_start "run_test" 2>/dev/null
run "echo_run" echo "hello" 2>/dev/null
[[ -f "$BV_E2E_RUN_DIR/run_test/echo_run.out" ]] && pass "run creates .out file" || fail "run creates .out" ""
[[ -f "$BV_E2E_RUN_DIR/run_test/echo_run.err" ]] && pass "run creates .err file" || fail "run creates .err" ""
[[ -f "$BV_E2E_RUN_DIR/run_test/echo_run.cmd" ]] && pass "run creates .cmd file" || fail "run creates .cmd" ""

out_content=$(cat "$BV_E2E_RUN_DIR/run_test/echo_run.out")
[[ "$out_content" == "hello" ]] && pass "run captures stdout" || fail "run captures stdout" "$out_content"
test_pass 2>/dev/null

# Test: assertions
echo
echo "--- Assertion tests ---"
assert_eq "a" "a" "test" 2>/dev/null && pass "assert_eq passes on equal" || fail "assert_eq passes" ""
assert_eq "a" "b" "test" 2>/dev/null && fail "assert_eq fails on unequal" "" || pass "assert_eq fails on unequal"

assert_file_exists "tests/e2e/harness.sh" "test" 2>/dev/null && pass "assert_file_exists passes" || fail "assert_file_exists passes" ""
assert_file_exists "nonexistent" "test" 2>/dev/null && fail "assert_file_exists fails" "" || pass "assert_file_exists fails on missing"

# Test: summary generation
echo
echo "--- Summary tests ---"
BV_E2E_PASS_COUNT=2
BV_E2E_FAIL_COUNT=1
BV_E2E_SKIP_COUNT=1
BV_E2E_RESULTS=("PASS|test1|100" "PASS|test2|200" "FAIL|test3|150|reason" "SKIP|test4|0|skipped")
summary_json=$(_generate_summary_json)
[[ "$summary_json" == *'"total": 4'* ]] && pass "summary has totals" || fail "summary has totals" ""
[[ "$summary_json" == *'"passed": 2'* ]] && pass "summary has passed" || fail "summary has passed" ""
[[ "$summary_json" == *'"results": ['* ]] && pass "summary has results array" || fail "summary has results" ""

echo "$summary_json" | jq . >/dev/null 2>&1 && pass "summary JSON is valid" || fail "summary JSON is valid" ""

# Test: JUnit XML generation
echo
echo "--- JUnit XML tests ---"
export BV_E2E_JUNIT_XML="/tmp/harness_test_junit.xml"
_generate_junit_xml 2>/dev/null
[[ -f "$BV_E2E_JUNIT_XML" ]] && pass "JUnit XML created" || fail "JUnit XML created" ""

junit_content=$(cat "$BV_E2E_JUNIT_XML")
[[ "$junit_content" == *'<testsuites'* ]] && pass "JUnit has testsuites" || fail "JUnit has testsuites" ""
[[ "$junit_content" == *'<testsuite'* ]] && pass "JUnit has testsuite" || fail "JUnit has testsuite" ""
[[ "$junit_content" == *'<testcase'* ]] && pass "JUnit has testcase" || fail "JUnit has testcase" ""
[[ "$junit_content" == *'<failure'* ]] && pass "JUnit has failure" || fail "JUnit has failure" ""

rm -f "$BV_E2E_JUNIT_XML"

# Summary
echo
echo "=== Results ==="
echo "Passed: $HARNESS_TEST_PASS"
echo "Failed: $HARNESS_TEST_FAIL"

# Cleanup
rm -rf "$BV_E2E_RUN_DIR"

if [[ $HARNESS_TEST_FAIL -gt 0 ]]; then
  exit 1
fi
exit 0
