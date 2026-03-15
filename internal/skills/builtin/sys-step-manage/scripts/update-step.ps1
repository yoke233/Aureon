#!/usr/bin/env pwsh
# update-step.ps1 — Update a pending step.
#
# Usage:
#   pwsh -NoProfile -File update-step.ps1 <step-id> '<json-payload>'
#
# Only pending steps can be edited.

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$StepId,

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
    -Method Put `
    -Uri "$server/api/steps/$StepId" `
    -Headers $headers `
    -Body $Payload `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error updating step: $($_.Exception.Message)"
  exit 1
}
