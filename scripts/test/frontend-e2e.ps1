param(
  [string]$AppUrl = "http://localhost:5173",
  [string]$Token,
  [switch]$Headed
)

$ErrorActionPreference = "Stop"
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptRoot "..\\..")
Set-Location -LiteralPath $repoRoot

function Get-DotEnvValue {
  param(
    [string]$FilePath,
    [string]$Name
  )

  if (-not (Test-Path $FilePath)) {
    return $null
  }

  foreach ($line in Get-Content $FilePath) {
    if ($line -match "^\s*$Name=(.*)$") {
      return $matches[1].Trim()
    }
  }

  return $null
}

function Get-SecretsAdminToken {
  param(
    [string]$FilePath
  )

  if (-not (Test-Path $FilePath)) {
    return $null
  }

  $inAdminSection = $false
  foreach ($line in Get-Content $FilePath) {
    if ($line -match "^\[tokens\.admin\]\s*$") {
      $inAdminSection = $true
      continue
    }
    if ($inAdminSection -and $line -match "^\[.+\]\s*$") {
      break
    }
    if ($inAdminSection -and $line -match "^\s*token\s*=\s*['""]?([^'""]+)['""]?\s*$") {
      return $matches[1].Trim()
    }
  }

  return $null
}

function Test-TcpPortOpen {
  param(
    [string]$HostName,
    [int]$Port
  )

  $client = New-Object System.Net.Sockets.TcpClient
  try {
    $async = $client.BeginConnect($HostName, $Port, $null, $null)
    if (-not $async.AsyncWaitHandle.WaitOne(1000)) {
      return $false
    }
    $client.EndConnect($async)
    return $true
  } catch {
    return $false
  } finally {
    $client.Dispose()
  }
}

$proxyTarget = $env:VITE_API_PROXY_TARGET
if (-not $proxyTarget) {
  $proxyTarget = Get-DotEnvValue -FilePath (Join-Path $repoRoot "web/.env") -Name "VITE_API_PROXY_TARGET"
}
if (-not $proxyTarget) {
  $proxyTarget = "http://127.0.0.1:8080"
}
$proxyUri = [uri]$proxyTarget
$backendPort = if ($proxyUri.IsDefaultPort) {
  if ($proxyUri.Scheme -eq "https") { 443 } else { 80 }
} else {
  $proxyUri.Port
}
$startedBackend = $false
$backendProcess = $null
$backendStdOut = Join-Path $env:TEMP ("ai-workflow-frontend-e2e-server-{0}.out.log" -f ([guid]::NewGuid().ToString("N")))
$backendStdErr = Join-Path $env:TEMP ("ai-workflow-frontend-e2e-server-{0}.err.log" -f ([guid]::NewGuid().ToString("N")))
$serverGeneratedToken = $null

$resolvedToken = $Token
if (-not $resolvedToken) {
  $resolvedToken = $env:APP_TOKEN
}
if (-not $resolvedToken) {
  $resolvedToken = Get-SecretsAdminToken -FilePath (Join-Path $repoRoot ".ai-workflow/secrets.toml")
}
if (-not $resolvedToken) {
  $resolvedToken = $env:VITE_API_TOKEN
}
if (-not $resolvedToken) {
  $resolvedToken = Get-DotEnvValue -FilePath (Join-Path $repoRoot "web/.env") -Name "VITE_API_TOKEN"
}

if (-not (Test-TcpPortOpen -HostName $proxyUri.Host -Port $backendPort)) {
  Write-Host "[e2e] backend is not reachable at $proxyTarget, starting local ai-flow server..."
  $backendProcess = Start-Process `
    -FilePath "go" `
    -ArgumentList @("run", "./cmd/ai-flow", "server", "--port", "$backendPort") `
    -WorkingDirectory $repoRoot `
    -RedirectStandardOutput $backendStdOut `
    -RedirectStandardError $backendStdErr `
    -PassThru
  $startedBackend = $true

  for ($attempt = 0; $attempt -lt 120; $attempt++) {
    Start-Sleep -Milliseconds 500

    if ($backendProcess.HasExited) {
      $stdout = if (Test-Path $backendStdOut) { Get-Content $backendStdOut -Raw } else { "" }
      $stderr = if (Test-Path $backendStdErr) { Get-Content $backendStdErr -Raw } else { "" }
      throw "Backend server exited during startup.`nSTDOUT:`n$stdout`nSTDERR:`n$stderr"
    }

    if (Test-Path $backendStdOut) {
      $stdoutLines = Get-Content $backendStdOut
      foreach ($line in $stdoutLines) {
        if ($line -match "^Admin token:\s+(.+)$") {
          $serverGeneratedToken = $matches[1].Trim()
        }
      }
      if ($stdoutLines -match "Server started on") {
        break
      }
    }

    if ($attempt -eq 119) {
      $stdout = if (Test-Path $backendStdOut) { Get-Content $backendStdOut -Raw } else { "" }
      $stderr = if (Test-Path $backendStdErr) { Get-Content $backendStdErr -Raw } else { "" }
      throw "Timed out waiting for backend server startup.`nSTDOUT:`n$stdout`nSTDERR:`n$stderr"
    }
  }
}

if ($serverGeneratedToken) {
  $resolvedToken = $serverGeneratedToken
}

$env:APP_TOKEN = $resolvedToken
$env:APP_URL = $AppUrl
if ($resolvedToken -and $AppUrl -notmatch "(?:\?|&)token=") {
  $separator = "?"
  if ($AppUrl.Contains("?")) {
    $separator = "&"
  }
  $env:APP_URL = "$AppUrl$separator" + "token=$([uri]::EscapeDataString($resolvedToken))"
}

$args = @(
  "-y",
  "@playwright/test",
  "test",
  "scripts/test/project-creation.e2e.spec.ts",
  "--workers=1",
  "--reporter=line"
)

if ($Headed) {
  $args += "--headed"
}

Write-Host "[e2e] running playwright project-creation flow..."
Write-Host "[e2e] APP_URL=$env:APP_URL"
if ($env:APP_TOKEN) {
  Write-Host "[e2e] APP_TOKEN=provided"
}
Write-Host "[e2e] suite=frontend-e2e-project-creation"

try {
  & npx @args
  if ($LASTEXITCODE -ne 0) {
    throw "Playwright e2e failed with exit code $LASTEXITCODE"
  }
} finally {
  if ($startedBackend -and $backendProcess -and -not $backendProcess.HasExited) {
    Write-Host "[e2e] stopping local ai-flow server..."
    Stop-Process -Id $backendProcess.Id -Force -ErrorAction SilentlyContinue
    $backendProcess.WaitForExit()
  }
}

Write-Host "[e2e] completed successfully."
