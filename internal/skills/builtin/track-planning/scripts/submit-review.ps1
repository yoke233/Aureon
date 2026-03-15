#!/usr/bin/env pwsh
# submit-review.ps1 — Submit the planning output for review.
#
# Advances the track from draft/planning to reviewing.
#
# Usage:
#   pwsh -NoProfile -File submit-review.ps1 <track-id> '<summary>' '<planner-output-json>'

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$TrackId,

  [Parameter(Mandatory = $true, Position = 1)]
  [string]$Summary,

  [Parameter(Mandatory = $true, Position = 2)]
  [string]$PlannerOutput
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

# Parse planner output to ensure valid JSON, then build payload.
$plannerObj = $PlannerOutput | ConvertFrom-Json
$payload = @{
  latest_summary      = $Summary
  planner_output_json = $plannerObj
} | ConvertTo-Json -Depth 10 -Compress

try {
  $response = Invoke-WebRequest `
    -Method Post `
    -Uri "$server/api/tracks/$TrackId/submit-review" `
    -Headers $headers `
    -Body $payload `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error submitting review: $($_.Exception.Message)"
  exit 1
}
