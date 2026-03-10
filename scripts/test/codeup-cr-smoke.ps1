[CmdletBinding()]
param(
  [string]$Domain = "https://test.rdc.aliyuncs.com",
  [Parameter(Mandatory = $true)][string]$Token,
  [Parameter(Mandatory = $true)][string]$OrganizationId,
  [Parameter(Mandatory = $true)][string]$RepositoryId,
  [long]$ProjectId = 0,
  [long]$SourceProjectId = 0,
  [long]$TargetProjectId = 0,
  [Parameter(Mandatory = $true)][string]$SourceBranch,
  [Parameter(Mandatory = $true)][string]$TargetBranch,
  [string]$Title = "",
  [string]$Description = "Automated Codeup CR smoke by ai-workflow",
  [string[]]$ReviewerUserIds = @(),
  [string]$WorkItemIds = "",
  [bool]$TriggerAIReviewRun = $false,
  [switch]$AutoMerge,
  [string]$MergeType = "no-fast-forward",
  [bool]$RemoveSourceBranch = $false,
  [string]$MergeMessage = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-ProjectIds {
  param(
    [long]$ProjectId,
    [long]$SourceProjectId,
    [long]$TargetProjectId
  )

  if ($SourceProjectId -le 0) {
    $SourceProjectId = $ProjectId
  }
  if ($TargetProjectId -le 0) {
    $TargetProjectId = $ProjectId
  }
  if ($SourceProjectId -le 0 -or $TargetProjectId -le 0) {
    throw "Either -ProjectId or both -SourceProjectId and -TargetProjectId are required."
  }
  return @{
    SourceProjectId = $SourceProjectId
    TargetProjectId = $TargetProjectId
  }
}

function Invoke-CodeupApi {
  param(
    [Parameter(Mandatory = $true)][string]$Method,
    [Parameter(Mandatory = $true)][string]$Url,
    [Parameter(Mandatory = $true)][string]$Token,
    [object]$Body = $null
  )

  $headers = @{
    "x-yunxiao-token" = $Token
  }

  if ($null -eq $Body) {
    $resp = Invoke-WebRequest -Method $Method -Uri $Url -Headers $headers -SkipHttpErrorCheck
  } else {
    $json = $Body | ConvertTo-Json -Depth 20
    $resp = Invoke-WebRequest -Method $Method -Uri $Url -Headers $headers -ContentType "application/json" -Body $json -SkipHttpErrorCheck
  }

  if ($resp.StatusCode -lt 200 -or $resp.StatusCode -ge 300) {
    $bodyText = [string]$resp.Content
    throw "Codeup API failed: $Method $Url (code=$($resp.StatusCode) body=$bodyText)"
  }

  if ([string]::IsNullOrWhiteSpace($resp.Content)) {
    return $null
  }
  return $resp.Content | ConvertFrom-Json
}

function Get-CRField {
  param(
    [Parameter(Mandatory = $true)]$Response,
    [Parameter(Mandatory = $true)][string]$Name
  )

  if ($null -eq $Response) {
    return $null
  }
  if ($Response.PSObject.Properties.Name -contains $Name) {
    return $Response.$Name
  }
  foreach ($container in @("data", "result")) {
    if ($Response.PSObject.Properties.Name -contains $container) {
      $nested = $Response.$container
      if ($null -ne $nested -and $nested.PSObject.Properties.Name -contains $Name) {
        return $nested.$Name
      }
    }
  }
  return $null
}

$ids = Resolve-ProjectIds -ProjectId $ProjectId -SourceProjectId $SourceProjectId -TargetProjectId $TargetProjectId

$encodedOrg = [uri]::EscapeDataString($OrganizationId)
$encodedRepo = [uri]::EscapeDataString($RepositoryId)
$base = ($Domain.TrimEnd("/"))
if ([string]::IsNullOrWhiteSpace($Title)) {
  $ts = Get-Date -Format "yyyyMMdd-HHmmss"
  $Title = "ai-workflow Codeup smoke $ts"
}

$createUrl = "$base/oapi/v1/codeup/organizations/$encodedOrg/repositories/$encodedRepo/changeRequests"
$createBody = @{
  title = $Title
  description = $Description
  sourceBranch = $SourceBranch
  targetBranch = $TargetBranch
  sourceProjectId = $ids.SourceProjectId
  targetProjectId = $ids.TargetProjectId
  triggerAIReviewRun = $TriggerAIReviewRun
}

if ($ReviewerUserIds.Count -gt 0) {
  $createBody["reviewerUserIds"] = $ReviewerUserIds
}
if (-not [string]::IsNullOrWhiteSpace($WorkItemIds)) {
  $createBody["workItemIds"] = $WorkItemIds
}

Write-Host "Creating Codeup change request..."
$cr = Invoke-CodeupApi -Method "POST" -Url $createUrl -Token $Token -Body $createBody
$localId = Get-CRField -Response $cr -Name "localId"
$webUrl = Get-CRField -Response $cr -Name "webUrl"
if ($null -eq $webUrl) {
  $webUrl = Get-CRField -Response $cr -Name "detailUrl"
}

Write-Host "organization_id=$OrganizationId"
Write-Host "repository_id=$RepositoryId"
Write-Host "source_branch=$SourceBranch"
Write-Host "target_branch=$TargetBranch"
Write-Host "title=$Title"
if ($null -ne $localId) {
  Write-Host "local_id=$localId"
}
if (-not [string]::IsNullOrWhiteSpace([string]$webUrl)) {
  Write-Host "cr_url=$webUrl"
}

if (-not $AutoMerge) {
  Write-Host "Create-only smoke completed. Re-run with -AutoMerge to test merge."
  return
}

if ($null -eq $localId -or [int64]$localId -le 0) {
  throw "Cannot auto-merge because localId was not found in create response."
}

$mergeUrl = "$base/oapi/v1/codeup/organizations/$encodedOrg/repositories/$encodedRepo/changeRequests/$localId/merge"
$mergeBody = @{
  mergeType = $MergeType
  removeSourceBranch = $RemoveSourceBranch
}
if (-not [string]::IsNullOrWhiteSpace($MergeMessage)) {
  $mergeBody["mergeMessage"] = $MergeMessage
}

Write-Host "Merging Codeup change request..."
$mergeResp = Invoke-CodeupApi -Method "POST" -Url $mergeUrl -Token $Token -Body $mergeBody
Write-Host "merge_result=$($mergeResp | ConvertTo-Json -Depth 10 -Compress)"
Write-Host "Codeup create+merge smoke completed."

