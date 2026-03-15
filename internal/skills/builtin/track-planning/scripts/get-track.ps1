#!/usr/bin/env pwsh
# get-track.ps1 — Get WorkItemTrack details.
#
# Usage:
#   pwsh -NoProfile -File get-track.ps1 <track-id>

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$TrackId
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
    -Uri "$server/api/tracks/$TrackId" `
    -Headers $headers `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error getting track: $($_.Exception.Message)"
  exit 1
}
