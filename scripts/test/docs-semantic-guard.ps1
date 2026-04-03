[CmdletBinding()]
param(
    [string]$ChangedFilesPath
)

$params = @{
    Task = "docs-semantic-guard"
}

if ($ChangedFilesPath) {
    $params.ChangedFilesPath = $ChangedFilesPath
}

& (Join-Path $PSScriptRoot "runner.ps1") @params
