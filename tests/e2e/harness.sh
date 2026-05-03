#!/usr/bin/env bash
# Enhanced E2E test harness for bt with comprehensive logging.
# Features: log levels, JSON format, artifact management, timing, CI integration.
set -euo pipefail

###############################################################################
# Configuration
###############################################################################
BT_E2E_LOG_DIR="${BT_E2E_LOG_DIR:-$(pwd)/.e2e-logs}"
BT_E2E_LOG_LEVEL="${BT_E2E_LOG_LEVEL:-INFO}"  # DEBUG, INFO, WARN, ERROR
BT_E2E_LOG_FORMAT="${BT_E2E_LOG_FORMAT:-text}" # text, json
BT_E2E_ARTIFACTS="${BT_E2E_ARTIFACTS:-}"      # Path for artifact storage
BT_E2E_JUNIT_XML="${BT_E2E_JUNIT_XML:-}"      # Path for JUnit XML output
BT_E2E_CI="${BT_E2E_CI:-}"                    # Set in CI environments (github, gitlab)

# Create unique run directory with timestamp
BT_E2E_RUN_ID="${BT_E2E_RUN_ID:-$(date -u +%Y%m%d-%H%M%S)-$$}"
BT_E2E_RUN_DIR="$BT_E2E_LOG_DIR/$BT_E2E_RUN_ID"

# Test tracking
declare -g BT_E2E_TEST_NAME=""
declare -g BT_E2E_TEST_PHASE=""
declare -g BT_E2E_TEST_START=""
declare -g BT_E2E_PASS_COUNT=0
declare -g BT_E2E_FAIL_COUNT=0
declare -g BT_E2E_SKIP_COUNT=0
declare -ga BT_E2E_RESULTS=()

# Exit codes
readonly E2E_EXIT_OK=0
readonly E2E_EXIT_TEST_FAIL=1
readonly E2E_EXIT_SETUP_FAIL=2
readonly E2E_EXIT_TIMEOUT=3
readonly E2E_EXIT_SKIP=4

###############################################################################
# Initialization
###############################################################################
_e2e_init() {
  mkdir -p "$BT_E2E_RUN_DIR"

  # Capture initial context
  _capture_context > "$BT_E2E_RUN_DIR/context.json"

  log_info "E2E harness initialized"
  log_debug "Run ID: $BT_E2E_RUN_ID"
  log_debug "Log dir: $BT_E2E_RUN_DIR"
}

###############################################################################
# Timestamps
###############################################################################
ts() {
  # Millisecond precision timestamp
  if date --version >/dev/null 2>&1; then
    # GNU date
    date -u +"%Y-%m-%dT%H:%M:%S.%3NZ"
  else
    # macOS date (no milliseconds support)
    date -u +"%Y-%m-%dT%H:%M:%S.000Z"
  fi
}

ts_epoch_ms() {
  # Epoch milliseconds for duration calculation
  if date --version >/dev/null 2>&1; then
    echo $(($(date +%s%N)/1000000))
  else
    # macOS fallback
    echo $(($(date +%s)*1000))
  fi
}

###############################################################################
# Log Levels
###############################################################################
_log_level_num() {
  case "$1" in
    DEBUG) echo 0 ;;
    INFO)  echo 1 ;;
    WARN)  echo 2 ;;
    ERROR) echo 3 ;;
    *)     echo 1 ;;
  esac
}

_should_log() {
  local level="$1"
  local threshold
  threshold=$(_log_level_num "$BT_E2E_LOG_LEVEL")
  local current
  current=$(_log_level_num "$level")
  [[ $current -ge $threshold ]]
}

###############################################################################
# Logging Functions
###############################################################################
_log() {
  local level="$1"; shift
  local msg="$*"

  _should_log "$level" || return 0

  local timestamp
  timestamp=$(ts)
  local context=""
  [[ -n "$BT_E2E_TEST_NAME" ]] && context="$BT_E2E_TEST_NAME"
  [[ -n "$BT_E2E_TEST_PHASE" ]] && context="${context:+$context/}$BT_E2E_TEST_PHASE"

  if [[ "$BT_E2E_LOG_FORMAT" == "json" ]]; then
    local json_msg
    json_msg=$(printf '%s' "$msg" | sed 's/"/\\"/g; s/	/\\t/g')
    printf '{"ts":"%s","level":"%s","test":"%s","phase":"%s","msg":"%s"}\n' \
      "$timestamp" "$level" "$BT_E2E_TEST_NAME" "$BT_E2E_TEST_PHASE" "$json_msg" >&2
  else
    local prefix="[e2e $timestamp]"
    [[ -n "$context" ]] && prefix="$prefix [$context]"
    printf '%s %s: %s\n' "$prefix" "$level" "$msg" >&2
  fi

  # GitHub Actions annotations
  if [[ "$BT_E2E_CI" == "github" ]]; then
    case "$level" in
      ERROR) echo "::error ::$msg" ;;
      WARN)  echo "::warning ::$msg" ;;
    esac
  fi
}

log_debug() { _log DEBUG "$@"; }
log_info()  { _log INFO "$@"; }
log_warn()  { _log WARN "$@"; }
log_error() { _log ERROR "$@"; }

# Legacy compatibility
log() { log_info "$@"; }

###############################################################################
# Context Capture
###############################################################################
_capture_context() {
  local git_branch="" git_commit="" git_dirty=""

  if command -v git >/dev/null && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
    git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo "")
    git_dirty=$(git diff --quiet 2>/dev/null && echo "false" || echo "true")
  fi

  # Sanitize environment (remove secrets)
  local env_json="{"
  local first=true
  while IFS='=' read -r key value; do
    # Skip sensitive variables
    case "$key" in
      *TOKEN*|*SECRET*|*PASSWORD*|*KEY*|*CREDENTIAL*) continue ;;
    esac
    # Skip very long values
    [[ ${#value} -gt 200 ]] && continue
    # Escape JSON
    value=$(printf '%s' "$value" | sed 's/"/\\"/g; s/	/\\t/g' | tr -d '\n')
    if $first; then first=false; else env_json+=","; fi
    env_json+="\"$key\":\"$value\""
  done < <(env | grep -E '^(BT_|PATH|HOME|USER|SHELL|TERM|PWD|GOPATH|GOROOT)=' || true)
  env_json+="}"

  cat <<EOF
{
  "timestamp": "$(ts)",
  "run_id": "$BT_E2E_RUN_ID",
  "working_dir": "$(pwd)",
  "hostname": "$(hostname 2>/dev/null || echo 'unknown')",
  "user": "${USER:-unknown}",
  "shell": "${SHELL:-unknown}",
  "git": {
    "branch": "$git_branch",
    "commit": "$git_commit",
    "dirty": $git_dirty
  },
  "environment": $env_json
}
EOF
}

###############################################################################
# Test Lifecycle
###############################################################################
test_start() {
  local name="$1"
  local phase="${2:-}"

  BT_E2E_TEST_NAME="$name"
  BT_E2E_TEST_PHASE="${phase:-setup}"
  BT_E2E_TEST_START=$(ts_epoch_ms)

  mkdir -p "$BT_E2E_RUN_DIR/$name"

  log_info "Starting test: $name"

  # GitHub Actions group
  [[ "$BT_E2E_CI" == "github" ]] && echo "::group::Test: $name" || true
}

test_phase() {
  local phase="$1"
  BT_E2E_TEST_PHASE="$phase"
  log_debug "Phase: $phase"
}

test_pass() {
  local name="${1:-$BT_E2E_TEST_NAME}"
  local duration=$(_duration_ms)

  (( ++BT_E2E_PASS_COUNT )) || true
  BT_E2E_RESULTS+=("PASS|$name|$duration")

  log_info "PASS: $name (${duration}ms)"

  # End GitHub Actions group
  [[ "$BT_E2E_CI" == "github" ]] && echo "::endgroup::" || true

  _reset_test_state
}

test_fail() {
  local name="${1:-$BT_E2E_TEST_NAME}"
  local reason="${2:-}"
  local duration=$(_duration_ms)

  (( ++BT_E2E_FAIL_COUNT )) || true
  BT_E2E_RESULTS+=("FAIL|$name|$duration|$reason")

  log_error "FAIL: $name (${duration}ms)${reason:+ - $reason}"

  # End GitHub Actions group
  [[ "$BT_E2E_CI" == "github" ]] && echo "::endgroup::" || true

  _reset_test_state
}

test_skip() {
  local name="${1:-$BT_E2E_TEST_NAME}"
  local reason="${2:-}"
  local duration=$(_duration_ms)

  (( ++BT_E2E_SKIP_COUNT )) || true
  BT_E2E_RESULTS+=("SKIP|$name|$duration|$reason")

  log_warn "SKIP: $name${reason:+ - $reason}"

  # End GitHub Actions group
  [[ "$BT_E2E_CI" == "github" ]] && echo "::endgroup::" || true

  _reset_test_state
}

_duration_ms() {
  local end
  end=$(ts_epoch_ms)
  echo $((end - BT_E2E_TEST_START))
}

_reset_test_state() {
  BT_E2E_TEST_NAME=""
  BT_E2E_TEST_PHASE=""
  BT_E2E_TEST_START=""
}

###############################################################################
# Command Execution
###############################################################################
# run <name> <cmd...>
# Captures stdout/stderr to files in test-specific directory.
run() {
  local name="$1"; shift
  local test_dir="$BT_E2E_RUN_DIR/${BT_E2E_TEST_NAME:-_global}"
  mkdir -p "$test_dir"

  local out="$test_dir/${name}.out"
  local err="$test_dir/${name}.err"
  local cmd_file="$test_dir/${name}.cmd"

  # Save command for debugging
  echo "$*" > "$cmd_file"

  log_debug "RUN $name: $*"
  test_phase "$name"

  local start
  start=$(ts_epoch_ms)

  if "$@" >"$out" 2>"$err"; then
    local dur=$(($(ts_epoch_ms) - start))
    log_debug "OK $name (${dur}ms)"
    return 0
  else
    local code=$?
    local dur=$(($(ts_epoch_ms) - start))
    log_error "FAIL $name (exit $code, ${dur}ms)"
    log_error "  stdout: $out"
    log_error "  stderr: $err"

    # Show tail of stderr for context
    if [[ -s "$err" ]]; then
      log_error "  stderr tail:"
      tail -5 "$err" | while read -r line; do
        log_error "    $line"
      done
    fi

    return $code
  fi
}

# run_timeout <timeout_secs> <name> <cmd...>
# Run with timeout, returning E2E_EXIT_TIMEOUT on timeout.
run_timeout() {
  local timeout="$1"; shift
  local name="$1"; shift

  log_debug "RUN (timeout=${timeout}s) $name: $*"

  if command -v timeout >/dev/null; then
    if timeout "$timeout" "$@"; then
      return 0
    else
      local code=$?
      if [[ $code -eq 124 ]]; then
        log_error "TIMEOUT $name after ${timeout}s"
        return $E2E_EXIT_TIMEOUT
      fi
      return $code
    fi
  else
    # Fallback without timeout command
    log_warn "timeout command not available, running without timeout"
    run "$name" "$@"
  fi
}

###############################################################################
# Assertions
###############################################################################
# jq_field <file> <jq expression>
jq_field() {
  local file="$1"; shift
  local expr="$*"

  if ! jq -e "$expr" "$file" >/dev/null 2>&1; then
    log_error "Assertion failed: jq '$expr' on $file"
    return 1
  fi
}

# assert_eq <expected> <actual> [message]
assert_eq() {
  local expected="$1"
  local actual="$2"
  local msg="${3:-}"

  if [[ "$expected" != "$actual" ]]; then
    log_error "Assertion failed${msg:+: $msg}"
    log_error "  expected: $expected"
    log_error "  actual:   $actual"
    return 1
  fi
}

# assert_contains <haystack> <needle> [message]
assert_contains() {
  local haystack="$1"
  local needle="$2"
  local msg="${3:-}"

  if [[ "$haystack" != *"$needle"* ]]; then
    log_error "Assertion failed${msg:+: $msg}"
    log_error "  expected to contain: $needle"
    log_error "  actual: $haystack"
    return 1
  fi
}

# assert_file_exists <path> [message]
assert_file_exists() {
  local path="$1"
  local msg="${2:-}"

  if [[ ! -f "$path" ]]; then
    log_error "Assertion failed${msg:+: $msg}"
    log_error "  file does not exist: $path"
    return 1
  fi
}

# assert_json_valid <file> [message]
assert_json_valid() {
  local file="$1"
  local msg="${2:-}"

  if ! jq empty "$file" 2>/dev/null; then
    log_error "Assertion failed${msg:+: $msg}"
    log_error "  not valid JSON: $file"
    return 1
  fi
}

###############################################################################
# Artifact Management
###############################################################################
# save_artifact <source> <name>
# Copy file to artifacts directory with test context.
save_artifact() {
  local source="$1"
  local name="${2:-$(basename "$source")}"
  local dest_dir="$BT_E2E_RUN_DIR/${BT_E2E_TEST_NAME:-_global}/artifacts"

  mkdir -p "$dest_dir"
  cp "$source" "$dest_dir/$name"
  log_debug "Saved artifact: $dest_dir/$name"
}

# capture_output <name>
# Capture stdout of following command to artifact.
capture_output() {
  local name="$1"
  local dest="$BT_E2E_RUN_DIR/${BT_E2E_TEST_NAME:-_global}/artifacts/$name"
  mkdir -p "$(dirname "$dest")"
  cat > "$dest"
  log_debug "Captured output: $dest"
}

###############################################################################
# Section Headers
###############################################################################
section() {
  local title="$*"
  log_info "━━━━━ $title ━━━━━"

  # GitHub Actions group for sections
  [[ "$BT_E2E_CI" == "github" ]] && echo "::group::$title" || true
}

section_end() {
  [[ "$BT_E2E_CI" == "github" ]] && echo "::endgroup::" || true
}

###############################################################################
# Summary & Reporting
###############################################################################
e2e_summary() {
  local total=$((BT_E2E_PASS_COUNT + BT_E2E_FAIL_COUNT + BT_E2E_SKIP_COUNT))

  section "Test Summary"

  log_info "Total:  $total"
  log_info "Passed: $BT_E2E_PASS_COUNT"
  [[ $BT_E2E_FAIL_COUNT -gt 0 ]] && log_error "Failed: $BT_E2E_FAIL_COUNT" || true
  [[ $BT_E2E_SKIP_COUNT -gt 0 ]] && log_warn "Skipped: $BT_E2E_SKIP_COUNT" || true

  # List failures
  if [[ $BT_E2E_FAIL_COUNT -gt 0 ]]; then
    log_error "Failed tests:"
    for result in "${BT_E2E_RESULTS[@]}"; do
      if [[ "$result" == FAIL* ]]; then
        IFS='|' read -r status name duration reason <<< "$result"
        log_error "  - $name${reason:+ ($reason)}"
      fi
    done
  fi

  # Generate JUnit XML if requested
  [[ -n "$BT_E2E_JUNIT_XML" ]] && _generate_junit_xml || true

  # Generate summary JSON
  _generate_summary_json > "$BT_E2E_RUN_DIR/summary.json"

  section_end

  log_info "Artifacts: $BT_E2E_RUN_DIR"

  # Return appropriate exit code
  if [[ $BT_E2E_FAIL_COUNT -gt 0 ]]; then
    return $E2E_EXIT_TEST_FAIL
  fi
  return $E2E_EXIT_OK
}

_generate_summary_json() {
  cat <<EOF
{
  "run_id": "$BT_E2E_RUN_ID",
  "timestamp": "$(ts)",
  "totals": {
    "total": $((BT_E2E_PASS_COUNT + BT_E2E_FAIL_COUNT + BT_E2E_SKIP_COUNT)),
    "passed": $BT_E2E_PASS_COUNT,
    "failed": $BT_E2E_FAIL_COUNT,
    "skipped": $BT_E2E_SKIP_COUNT
  },
  "results": [
EOF

  local first=true
  for result in "${BT_E2E_RESULTS[@]}"; do
    IFS='|' read -r status name duration reason <<< "$result"
    $first || printf ',\n'
    first=false
    # Escape reason for JSON
    local escaped_reason
    escaped_reason=$(printf '%s' "$reason" | sed 's/"/\\"/g; s/	/\\t/g')
    if [[ -n "$reason" ]]; then
      printf '    {"status":"%s","name":"%s","duration_ms":%s,"reason":"%s"}' \
        "$status" "$name" "$duration" "$escaped_reason"
    else
      printf '    {"status":"%s","name":"%s","duration_ms":%s}' \
        "$status" "$name" "$duration"
    fi
  done
  echo

  cat <<EOF
  ]
}
EOF
}

_generate_junit_xml() {
  local output="${BT_E2E_JUNIT_XML}"
  local total=$((BT_E2E_PASS_COUNT + BT_E2E_FAIL_COUNT + BT_E2E_SKIP_COUNT))

  cat > "$output" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="bt-e2e" tests="$total" failures="$BT_E2E_FAIL_COUNT" skipped="$BT_E2E_SKIP_COUNT">
  <testsuite name="e2e" tests="$total" failures="$BT_E2E_FAIL_COUNT" skipped="$BT_E2E_SKIP_COUNT">
EOF

  for result in "${BT_E2E_RESULTS[@]}"; do
    IFS='|' read -r status name duration reason <<< "$result"
    local time_sec
    time_sec=$(echo "scale=3; $duration / 1000" | bc 2>/dev/null || echo "0")

    echo "    <testcase name=\"$name\" time=\"$time_sec\">" >> "$output"

    case "$status" in
      FAIL)
        echo "      <failure message=\"${reason:-Test failed}\"/>" >> "$output"
        ;;
      SKIP)
        echo "      <skipped message=\"${reason:-Skipped}\"/>" >> "$output"
        ;;
    esac

    echo "    </testcase>" >> "$output"
  done

  cat >> "$output" <<EOF
  </testsuite>
</testsuites>
EOF

  log_debug "Generated JUnit XML: $output"
}

###############################################################################
# Auto-initialization
###############################################################################
# Initialize on source (can be disabled with BT_E2E_NO_INIT=1)
if [[ -z "${BT_E2E_NO_INIT:-}" ]]; then
  _e2e_init
fi
