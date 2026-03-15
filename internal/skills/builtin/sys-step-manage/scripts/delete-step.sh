#!/usr/bin/env bash
# delete-step.sh — Delete a pending step.
#
# Usage:
#   ./delete-step.sh <step-id>
#
# Only pending steps can be deleted.
#
# Environment:
#   AI_WORKFLOW_SERVER_ADDR, AI_WORKFLOW_API_TOKEN

set -euo pipefail

STEP_ID="${1:?Usage: delete-step.sh <step-id>}"

SERVER="${AI_WORKFLOW_SERVER_ADDR:?AI_WORKFLOW_SERVER_ADDR is required}"
TOKEN="${AI_WORKFLOW_API_TOKEN:-}"

AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
  AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
  "${SERVER}/api/steps/${STEP_ID}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER:+-H "$AUTH_HEADER"} 2>/dev/null || echo "000")

if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
  echo "{\"deleted\":true,\"step_id\":${STEP_ID}}"
else
  echo "Error deleting step: HTTP ${HTTP_CODE}" >&2
  exit 1
fi
