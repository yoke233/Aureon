#!/usr/bin/env pwsh
# get-step.ps1 — Get details of a specific step.
#
# Usage:
#   pwsh -NoProfile -File get-step.ps1 <step-id>

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
  $response = Invoke-WebRequest `
    -Method Get `
    -Uri "$server/api/steps/$StepId" `
    -Headers $headers `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error getting step: $($_.Exception.Message)"
  exit 1
}
