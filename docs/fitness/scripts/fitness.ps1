[CmdletBinding()]
param(
  [switch]$DryRun,
  [switch]$VerboseOutput
)

$ErrorActionPreference = "Stop"

function Get-ProjectRoot {
  return (Resolve-Path (Join-Path $PSScriptRoot "..\..\..")).Path
}

function Get-FrontmatterObject {
  param(
    [string]$Path
  )

  $raw = Get-Content -Raw -Path $Path
  if ($raw -notmatch "(?s)^---\r?\n(.*?)\r?\n---") {
    return $null
  }

  $yaml = $Matches[1]
  $lines = $yaml -split "`r?`n"
  $result = [ordered]@{}
  $currentMap = $null
  $currentList = $null
  $currentItem = $null

  foreach ($rawLine in $lines) {
    if ([string]::IsNullOrWhiteSpace($rawLine)) {
      continue
    }

    $line = $rawLine.TrimEnd()

    if ($line -match '^([A-Za-z0-9_]+):\s*(.*)$' -and -not $rawLine.StartsWith(' ')) {
      $key = $Matches[1]
      $value = $Matches[2].Trim()

      if ([string]::IsNullOrWhiteSpace($value)) {
        if ($key -eq "metrics") {
          $result[$key] = [System.Collections.ArrayList]::new()
          $currentList = $result[$key]
          $currentMap = $null
          $currentItem = $null
        } else {
          $result[$key] = [ordered]@{}
          $currentMap = $result[$key]
          $currentList = $null
          $currentItem = $null
        }
      } else {
        $result[$key] = Convert-ScalarValue $value
        $currentMap = $null
        $currentList = $null
        $currentItem = $null
      }
      continue
    }

    if ($rawLine -match '^\s{2}([A-Za-z0-9_]+):\s*(.*)$' -and $null -ne $currentMap) {
      $currentMap[$Matches[1]] = Convert-ScalarValue $Matches[2]
      continue
    }

    if ($rawLine -match '^\s{2}-\s+([A-Za-z0-9_]+):\s*(.*)$' -and $null -ne $currentList) {
      $item = [ordered]@{}
      $item[$Matches[1]] = Convert-ScalarValue $Matches[2]
      [void]$currentList.Add([pscustomobject]$item)
      $currentItem = $currentList[$currentList.Count - 1]
      continue
    }

    if ($rawLine -match '^\s{4}([A-Za-z0-9_]+):\s*(.*)$' -and $null -ne $currentItem) {
      $currentItem | Add-Member -NotePropertyName $Matches[1] -NotePropertyValue (Convert-ScalarValue $Matches[2]) -Force
      continue
    }
  }

  return [pscustomobject]$result
}

function Convert-ScalarValue {
  param(
    [string]$Value
  )

  $trimmed = $Value.Trim()
  if ($trimmed -eq "true") { return $true }
  if ($trimmed -eq "false") { return $false }
  if ($trimmed -match '^\d+(\.\d+)?$') { return [double]$trimmed }
  return $trimmed
}

function Invoke-FitnessMetric {
  param(
    [object]$Metric,
    [string]$ProjectRoot,
    [switch]$DryRun,
    [switch]$VerboseOutput
  )

  $name = [string]$Metric.name
  $command = [string]$Metric.command
  $pattern = [string]$Metric.pattern

  if ($DryRun) {
    return [pscustomobject]@{
      Name = $name
      Passed = $true
      Output = "[DRY-RUN] Would run: $command"
    }
  }

  $output = & pwsh -NoProfile -Command $command 2>&1 | Out-String
  $exitCode = $LASTEXITCODE

  $passed = $true
  if (-not [string]::IsNullOrWhiteSpace($pattern)) {
    $passed = $output -match $pattern
  } else {
    $passed = ($exitCode -eq 0)
  }

  if (-not $passed -and -not $VerboseOutput -and $output.Length -gt 600) {
    $output = $output.Substring(0, 600)
  }

  return [pscustomobject]@{
    Name = $name
    Passed = $passed
    Output = $output.Trim()
  }
}

$projectRoot = Get-ProjectRoot
$fitnessDir = Join-Path $projectRoot "docs\fitness"
$files = Get-ChildItem -Path $fitnessDir -Filter "*.md" | Where-Object { $_.Name -ne "README.md" } | Sort-Object Name

$hardGateFailed = @()
[double]$totalScore = 0
[double]$totalWeight = 0

Write-Output ("=" * 60)
Write-Output "FITNESS FUNCTION REPORT"
if ($DryRun) {
  Write-Output "(DRY-RUN MODE)"
}
Write-Output ("=" * 60)

Push-Location $projectRoot
try {
  foreach ($file in $files) {
    $frontmatter = Get-FrontmatterObject -Path $file.FullName
    if ($null -eq $frontmatter -or $null -eq $frontmatter.metrics) {
      continue
    }

    $dimension = [string]$frontmatter.dimension
    $weight = [double]$frontmatter.weight
    $metrics = @($frontmatter.metrics)

    Write-Output ""
    Write-Output ("## {0} (weight: {1}%)" -f $dimension.ToUpperInvariant(), $weight)
    Write-Output ("   Source: {0}" -f $file.Name)

    $passedCount = 0
    $totalCount = 0

    foreach ($metric in $metrics) {
      $result = Invoke-FitnessMetric -Metric $metric -ProjectRoot $projectRoot -DryRun:$DryRun -VerboseOutput:$VerboseOutput
      $hardGate = [bool]$metric.hard_gate
      $status = if ($result.Passed) { "PASS" } else { "FAIL" }
      $suffix = if ($hardGate) { " [HARD GATE]" } else { "" }

      Write-Output ("   - {0}: {1}{2}" -f $result.Name, $status, $suffix)

      if (-not $result.Passed -and ($VerboseOutput -or $hardGate)) {
        Write-Output ("     Command: {0}" -f [string]$metric.command)
        if (-not [string]::IsNullOrWhiteSpace($result.Output)) {
          $result.Output -split "`r?`n" | Select-Object -First 12 | ForEach-Object {
            Write-Output ("     > {0}" -f $_)
          }
        }
      }

      if (-not $result.Passed -and $hardGate) {
        $hardGateFailed += $result.Name
      }

      if ($result.Passed) {
        $passedCount++
      }
      $totalCount++
    }

    if ($totalCount -gt 0) {
      $dimensionScore = ($passedCount / $totalCount) * 100
      $totalScore += ($dimensionScore * $weight)
      $totalWeight += $weight
      Write-Output ("   Score: {0:N0}%" -f $dimensionScore)
    }
  }
}
finally {
  Pop-Location
}

Write-Output ""
Write-Output ("=" * 60)

if ($hardGateFailed.Count -gt 0) {
  Write-Output ("HARD GATES FAILED: {0}" -f ($hardGateFailed -join ", "))
  exit 2
}

if ($totalWeight -gt 0) {
  $finalScore = $totalScore / $totalWeight
  Write-Output ("FINAL SCORE: {0:N1}%" -f $finalScore)

  if ($finalScore -ge 90) {
    Write-Output "PASS"
  } elseif ($finalScore -ge 80) {
    Write-Output "WARN"
  } else {
    Write-Output "BLOCK"
    exit 1
  }
}

Write-Output ("=" * 60)
