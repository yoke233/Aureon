param(
    [string]$OutputPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..\..")
Set-Location -LiteralPath $repoRoot

if (-not $OutputPath) {
    $binaryName = "ai-flow"
    if ($IsWindows) {
        $binaryName += ".exe"
    }
    $OutputPath = Join-Path $repoRoot ".runtime\bin\$binaryName"
}

$outputDir = Split-Path -Parent $OutputPath
if (-not (Test-Path -LiteralPath $outputDir)) {
    New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
}

Write-Host "==> Building ai-flow dev binary" -ForegroundColor Cyan
Write-Host "Output: $OutputPath"

$env:CGO_ENABLED = "0"
go build -tags dev -o $OutputPath ./cmd/ai-flow
if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}

Write-Host "<== Build completed" -ForegroundColor Green
