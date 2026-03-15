#!/usr/bin/env bash
# list-messages.sh — List recent messages from a thread.
#
# Usage:
#   ./list-messages.sh <thread-id> [limit]
#
# Default limit is 50 messages.
#
# Environment:
#   AI_WORKFLOW_SERVER_ADDR, AI_WORKFLOW_API_TOKEN

set -euo pipefail

THREAD_ID="${1:?Usage: list-messages.sh <thread-id> [limit]}"
LIMIT="${2:-50}"

SERVER="${AI_WORKFLOW_SERVER_ADDR:?AI_WORKFLOW_SERVER_ADDR is required}"
TOKEN="${AI_WORKFLOW_API_TOKEN:-}"

AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

RESPONSE=$(curl -sf -X GET \
  "${SERVER}/api/threads/${THREAD_ID}/messages?limit=${LIMIT}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} 2>&1) || {
  echo "Error listing messages: ${RESPONSE}" >&2
  exit 1
}

echo "$RESPONSE"
