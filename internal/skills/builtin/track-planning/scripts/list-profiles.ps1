#!/usr/bin/env pwsh
# list-profiles.ps1 — List all agent profiles with their roles and capabilities.
#
# Usage:
#   pwsh -NoProfile -File list-profiles.ps1

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
    -Uri "$server/api/agents/profiles" `
    -Headers $headers `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error listing profiles: $($_.Exception.Message)"
  exit 1
}
