#!/usr/bin/env bash
# audit-corpus.sh - Pre-commit denylist scanner for labeled corpus JSON.
#
# USAGE:
#   scripts/audit-corpus.sh [path/to/corpus.json]
#   (default path: pkg/view/testdata/corpus/labeled_corpus.json)
#
# SCHEMA (per docs/plans/2026-04-21-feat-cross-project-intent-taxonomy-pairs-refs-v2-plan.md):
#   {
#     "issues":         [ { "id", "description", "notes", "close_reason",
#                           "dependencies": [...], "comments": [ "..." | {...} ] } ],
#     "expected_pairs": [ { "suffix", "members", "intentional", "reason" } ],
#     "expected_refs":  [ { "source", "target", "intentional", "reason" } ]
#   }
#
# Scans all issue prose (description, notes, close_reason, comments, dependencies)
# for denylisted secret/PII/private-identifier patterns. Intended to run pre-commit
# before the corpus lands in the PUBLIC beadstui repo.
#
# EXIT CODES:
#   0 - clean, no denylist hits
#   1 - at least one denylist hit (details on stderr)
#   2 - usage/file error
#
# DEPENDENCIES:
#   jq (preferred; strict JSON extraction of prose fields)
#   If jq is missing the script aborts with exit 2 rather than grep-falling-back;
#   a grep fallback would produce too many false positives against JSON structure
#   (e.g. the literal string "description" in keys) to be useful as a gate.

set -euo pipefail

CORPUS="${1:-pkg/view/testdata/corpus/labeled_corpus.json}"

if [[ ! -f "$CORPUS" ]]; then
  echo "audit-corpus: file not found: $CORPUS" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "audit-corpus: jq is required but not installed" >&2
  exit 2
fi

# Extract prose as three fields per record separated by RS (ASCII 30, non-printable),
# records separated by newline. jq's @tsv doubles backslashes; we need raw bytes so
# regex checks for Windows paths (C:\Users\...) work correctly.
RS=$'\x1e'
PROSE="$(jq -r --arg rs "$RS" '
  .issues[]? as $i
  | ($i.id // "<no-id>") as $id
  | (
      (if $i.description  then [$id, "description",  $i.description]  else empty end),
      (if $i.notes        then [$id, "notes",        $i.notes]        else empty end),
      (if $i.close_reason then [$id, "close_reason", $i.close_reason] else empty end),
      (if ($i.dependencies|type) == "array" and ($i.dependencies|length) > 0
         then [$id, "dependencies", ($i.dependencies | map(tostring) | join(", "))]
         else empty end),
      (if ($i.comments|type) == "array"
         then ($i.comments[] | [$id, "comments",
               (if type=="string" then . else (.body // .text // tostring) end)])
         else empty end)
    )
  | join($rs)
' "$CORPUS")"

ISSUE_COUNT="$(jq '.issues | length' "$CORPUS")"
HITS=0

check() {
  local name="$1" regex="$2" flags="${3:-}"
  local id field val snippet
  while IFS=$'\x1e' read -r id field val; do
    [[ -z "$id" ]] && continue
    if grep -E $flags -q -- "$regex" <<< "$val"; then
      snippet="$(grep -E $flags -o -- "$regex" <<< "$val" | head -1)"
      echo "[DENYLIST] $name matched in issue=$id field=$field: $snippet" >&2
      HITS=$((HITS+1))
    fi
  done <<< "$PROSE"
}

check "password"         'password'                       '-i'
check "secret"           'secret'                         '-i'
check "token"            'token'                          '-i'
check "api_key"          'api[_-]key'                     '-i'
check "aws_access_key"   'AKIA[0-9A-Z]{16}'               ''
check "github_pat"       'ghp_[A-Za-z0-9]{36}'            ''
check "slack_token"      'xox[bp]-[A-Za-z0-9-]+'          ''
check "dotenv_ref"       '\.env([^a-zA-Z0-9]|$)'          ''
check "localhost_port"   'localhost:[0-9]+'               ''
check "windows_userpath" 'C:[\]Users[\][A-Za-z]+'         '-i'
check "macos_userpath"   '/Users/[A-Za-z]+'               '-i'

# Emails: extract all, drop the two allowed domains, anything left is a hit.
while IFS=$'\x1e' read -r id field val; do
  [[ -z "$id" ]] && continue
  while read -r email; do
    [[ -z "$email" ]] && continue
    case "$email" in
      *@seanmartinsmith.com)           : ;;
      *@users.noreply.github.com)      : ;;
      *)
        echo "[DENYLIST] email_external matched in issue=$id field=$field: $email" >&2
        HITS=$((HITS+1))
        ;;
    esac
  done < <(echo "$val" | grep -Eo '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' || true)
done <<< "$PROSE"

if [[ "$HITS" -gt 0 ]]; then
  echo "[audit-corpus] $HITS denylist hit(s) across $ISSUE_COUNT issues" >&2
  exit 1
fi

echo "[audit-corpus] scanned $ISSUE_COUNT issues, 0 denylist hits"
exit 0
