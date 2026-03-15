#!/usr/bin/env pwsh
# create-step.ps1 — Create a new step for a work item.
#
# Usage:
#   pwsh -NoProfile -File create-step.ps1 <work-item-id> '<json-payload>'

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$WorkItemId,

  [Parameter(Mandatory = $true, Position = 1)]
  [string]$Payload
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
    -Method Post `
    -Uri "$server/api/work-items/$WorkItemId/steps" `
    -Headers $headers `
    -Body $Payload `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error creating step: $($_.Exception.Message)"
  exit 1
}
