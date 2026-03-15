#!/usr/bin/env bash
# get-thread.sh — Get thread details including summary.
#
# Usage:
#   ./get-thread.sh <thread-id>
#
# Environment:
#   AI_WORKFLOW_SERVER_ADDR, AI_WORKFLOW_API_TOKEN

set -euo pipefail

THREAD_ID="${1:?Usage: get-thread.sh <thread-id>}"

SERVER="${AI_WORKFLOW_SERVER_ADDR:?AI_WORKFLOW_SERVER_ADDR is required}"
TOKEN="${AI_WORKFLOW_API_TOKEN:-}"

AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

RESPONSE=$(curl -sf -X GET \
  "${SERVER}/api/threads/${THREAD_ID}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} 2>&1) || {
  echo "Error getting thread: ${RESPONSE}" >&2
  exit 1
}

echo "$RESPONSE"
