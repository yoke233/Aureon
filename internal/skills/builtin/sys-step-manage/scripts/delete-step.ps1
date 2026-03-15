#!/usr/bin/env pwsh
# delete-step.ps1 — Delete a pending step.
#
# Usage:
#   pwsh -NoProfile -File delete-step.ps1 <step-id>
#
# Only pending steps can be deleted.

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$StepId
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
  $null = Invoke-WebRequest `
    -Method Delete `
    -Uri "$server/api/steps/$StepId" `
    -Headers $headers `
    -TimeoutSec 30

  Write-Output "{`"deleted`":true,`"step_id`":$StepId}"
} catch {
  Write-Error "Error deleting step: $($_.Exception.Message)"
  exit 1
}
