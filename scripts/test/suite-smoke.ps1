[CmdletBinding()]
param(
    [switch]$SkipTerminologyGate,
    [switch]$SkipGoTests
)

& (Join-Path $PSScriptRoot "runner.ps1") -Task "suite-smoke" @PSBoundParameters
