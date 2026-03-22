[CmdletBinding()]
param(
    [switch]$IncludeACPClientIntegration
)

& (Join-Path $PSScriptRoot "runner.ps1") -Task "backend-integration" @PSBoundParameters
