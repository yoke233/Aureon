#!/usr/bin/env bash
# update-step.sh — Update a pending step.
#
# Usage:
#   ./update-step.sh <step-id> '<json-payload>'
#
# Only pending steps can be edited.
#
# Environment:
#   AI_WORKFLOW_SERVER_ADDR, AI_WORKFLOW_API_TOKEN

set -euo pipefail

STEP_ID="${1:?Usage: update-step.sh <step-id> '<json>'}"
PAYLOAD="${2:?Usage: update-step.sh <step-id> '<json>'}"

SERVER="${AI_WORKFLOW_SERVER_ADDR:?AI_WORKFLOW_SERVER_ADDR is required}"
TOKEN="${AI_WORKFLOW_API_TOKEN:-}"

AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

RESPONSE=$(curl -sf -X PUT \
  "${SERVER}/api/steps/${STEP_ID}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} \
  -d "$PAYLOAD" 2>&1) || {
  echo "Error updating step: ${RESPONSE}" >&2
  exit 1
}

echo "$RESPONSE"
