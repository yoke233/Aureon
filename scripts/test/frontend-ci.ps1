[CmdletBinding()]
param(
    [switch]$WithE2E
)

& (Join-Path $PSScriptRoot "runner.ps1") -Task "frontend-ci" @PSBoundParameters
