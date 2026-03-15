#!/usr/bin/env pwsh
# list-messages.ps1 — List recent messages from a thread.
#
# Usage:
#   pwsh -NoProfile -File list-messages.ps1 <thread-id> [limit]
#
# Default limit is 50 messages.

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$ThreadId,

  [Parameter(Position = 1)]
  [int]$Limit = 50
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
    -Uri "$server/api/threads/$ThreadId/messages?limit=$Limit" `
    -Headers $headers `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error listing messages: $($_.Exception.Message)"
  exit 1
}
