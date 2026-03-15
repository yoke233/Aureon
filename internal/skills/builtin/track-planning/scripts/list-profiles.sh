#!/usr/bin/env bash
# list-profiles.sh — List all agent profiles with their roles and capabilities.
#
# Usage:
#   ./list-profiles.sh
#
# Environment:
#   AI_WORKFLOW_SERVER_ADDR, AI_WORKFLOW_API_TOKEN

set -euo pipefail

SERVER="${AI_WORKFLOW_SERVER_ADDR:?AI_WORKFLOW_SERVER_ADDR is required}"
TOKEN="${AI_WORKFLOW_API_TOKEN:-}"

AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

RESPONSE=$(curl -sf -X GET \
  "${SERVER}/api/agents/profiles" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} 2>&1) || {
  echo "Error listing profiles: ${RESPONSE}" >&2
  exit 1
}

echo "$RESPONSE"
