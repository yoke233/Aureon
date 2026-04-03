[CmdletBinding()]
param()

& (Join-Path $PSScriptRoot "runner.ps1") -Task "docs-semantic-guard"
