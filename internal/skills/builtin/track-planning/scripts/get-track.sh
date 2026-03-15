#!/usr/bin/env bash
# get-track.sh — Get WorkItemTrack details.
#
# Usage:
#   ./get-track.sh <track-id>
#
# Environment:
#   AI_WORKFLOW_SERVER_ADDR, AI_WORKFLOW_API_TOKEN

set -euo pipefail

TRACK_ID="${1:?Usage: get-track.sh <track-id>}"

SERVER="${AI_WORKFLOW_SERVER_ADDR:?AI_WORKFLOW_SERVER_ADDR is required}"
TOKEN="${AI_WORKFLOW_API_TOKEN:-}"

AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

RESPONSE=$(curl -sf -X GET \
  "${SERVER}/api/tracks/${TRACK_ID}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} 2>&1) || {
  echo "Error getting track: ${RESPONSE}" >&2
  exit 1
}

echo "$RESPONSE"
