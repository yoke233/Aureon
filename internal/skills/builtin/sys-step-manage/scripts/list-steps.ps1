#!/usr/bin/env pwsh
# list-steps.ps1 — List all steps for a work item.
#
# Usage:
#   pwsh -NoProfile -File list-steps.ps1 <work-item-id>

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$WorkItemId
)

$ErrorActionPreference = "Stop"

$server = $env:AI_WORKFLOW_SERVER_ADDR
if (-not $server) {
  Write-Error "AI_WORKFLOW_SERVER_ADDR is required"
  exit 1
}

$headers = @{ "Content-Type" = "application/json" }
$token = $env:AI_WORKFLOW_API_TOKEN
if ($token) {
  $headers["Authorization"] = "Bearer $token"
}

try {
  $response = Invoke-WebRequest `
    -Method Get `
    -Uri "$server/api/work-items/$WorkItemId/steps" `
    -Headers $headers `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error listing steps: $($_.Exception.Message)"
  exit 1
}
