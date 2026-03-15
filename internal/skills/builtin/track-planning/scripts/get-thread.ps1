#!/usr/bin/env pwsh
# get-thread.ps1 — Get thread details including summary.
#
# Usage:
#   pwsh -NoProfile -File get-thread.ps1 <thread-id>

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$ThreadId
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
    -Uri "$server/api/threads/$ThreadId" `
    -Headers $headers `
    -TimeoutSec 30

  Write-Output $response.Content
} catch {
  Write-Error "Error getting thread: $($_.Exception.Message)"
  exit 1
}
