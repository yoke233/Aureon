#!/usr/bin/env bash
# submit-review.sh — Submit the planning output for review.
#
# Advances the track from draft/planning to reviewing.
#
# Usage:
#   ./submit-review.sh <track-id> '<summary>' '<planner-output-json>'
#
# Environment:
#   AI_WORKFLOW_SERVER_ADDR, AI_WORKFLOW_API_TOKEN

set -euo pipefail

TRACK_ID="${1:?Usage: submit-review.sh <track-id> '<summary>' '<planner-output-json>'}"
SUMMARY="${2:?Usage: submit-review.sh <track-id> '<summary>' '<planner-output-json>'}"
PLANNER_OUTPUT="${3:?Usage: submit-review.sh <track-id> '<summary>' '<planner-output-json>'}"

SERVER="${AI_WORKFLOW_SERVER_ADDR:?AI_WORKFLOW_SERVER_ADDR is required}"
TOKEN="${AI_WORKFLOW_API_TOKEN:-}"

AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

# Build the request payload.
# Use a temp file to avoid shell escaping issues with complex JSON.
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

cat > "$TMPFILE" <<EOFPAYLOAD
{
  "latest_summary": $(printf '%s' "$SUMMARY" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))' 2>/dev/null || printf '"%s"' "$SUMMARY"),
  "planner_output_json": ${PLANNER_OUTPUT}
}
EOFPAYLOAD

RESPONSE=$(curl -sf -X POST \
  "${SERVER}/api/tracks/${TRACK_ID}/submit-review" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} \
  -d @"$TMPFILE" 2>&1) || {
  echo "Error submitting review: ${RESPONSE}" >&2
  exit 1
}

echo "$RESPONSE"
