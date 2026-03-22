[CmdletBinding()]
param()

& (Join-Path $PSScriptRoot "runner.ps1") -Task "backend-e2e"
