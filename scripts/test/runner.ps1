[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet(
        "backend-unit",
        "backend-integration",
        "backend-e2e",
        "backend-real",
        "frontend-unit",
        "frontend-build",
        "frontend-lint",
        "docs-semantic-guard",
        "frontend-ci",
        "suite-p3",
        "suite-smoke"
    )]
    [string]$Task,
    [string]$ChangedFilesPath,
    [switch]$IncludeACPClientIntegration,
    [switch]$WithE2E,
    [switch]$SkipTerminologyGate,
    [switch]$SkipGoTests
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Enter-RepoRoot -ScriptRoot $PSScriptRoot

function Invoke-RunnerTask {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,
        [string]$ChangedFilesPath,
        [switch]$IncludeACPClientIntegration,
        [switch]$WithE2E,
        [switch]$SkipTerminologyGate,
        [switch]$SkipGoTests
    )

    $params = @{
        Task = $Name
    }
    if ($ChangedFilesPath) {
        $params.ChangedFilesPath = $ChangedFilesPath
    }
    if ($IncludeACPClientIntegration) {
        $params.IncludeACPClientIntegration = $true
    }
    if ($WithE2E) {
        $params.WithE2E = $true
    }
    if ($SkipTerminologyGate) {
        $params.SkipTerminologyGate = $true
    }
    if ($SkipGoTests) {
        $params.SkipGoTests = $true
    }

    & $PSCommandPath @params
}

switch ($Task) {
    "backend-unit" {
        Set-SafeTestEnvironment
        Write-Host "RepoRoot: $repoRoot"
        Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"
        Write-Host "Backend unit target: exclude integration/e2e/real suites"

        Invoke-Step -Name "Backend unit suites" -CheckLastExitCode -Command {
            go test -p 4 -count=1 -timeout $env:GOTEST_TIMEOUT ./... -skip '^Test(Integration|E2E|Real)_'
        }
    }
    "backend-integration" {
        Set-SafeTestEnvironment
        Write-Host "RepoRoot: $repoRoot"
        Write-Host "Backend integration target: TestIntegration_*"
        Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

        if ($IncludeACPClientIntegration) {
            $env:AI_WORKFLOW_RUN_ACPCLIENT_INTEGRATION = "1"
            Write-Host "ACP client integration: enabled via AI_WORKFLOW_RUN_ACPCLIENT_INTEGRATION=1"
        } else {
            Remove-Item Env:AI_WORKFLOW_RUN_ACPCLIENT_INTEGRATION -ErrorAction SilentlyContinue
            Write-Host "ACP client integration: skipped by default (use -IncludeACPClientIntegration to enable)"
        }

        Invoke-Step -Name "Backend integration suites" -CheckLastExitCode -Command {
            go test -p 4 -count=1 -timeout $env:GOTEST_TIMEOUT ./... -run '^TestIntegration_'
        }
    }
    "backend-e2e" {
        Set-SafeTestEnvironment
        Write-Host "RepoRoot: $repoRoot"
        Write-Host "Backend E2E target: TestE2E_*"
        Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

        Invoke-Step -Name "Backend E2E suites" -CheckLastExitCode -Command {
            go test -p 4 -count=1 -timeout $env:GOTEST_TIMEOUT ./... -run '^TestE2E_'
        }
    }
    "backend-real" {
        Set-SafeTestEnvironment
        Write-Host "RepoRoot: $repoRoot"
        Write-Host "Backend real target: TestReal_* with -tags real"
        Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

        Invoke-Step -Name "Backend real suites" -CheckLastExitCode -Command {
            go test -tags real -p 4 -count=1 -timeout $env:GOTEST_TIMEOUT ./... -run '^TestReal_'
        }
    }
    "frontend-unit" {
        Write-Host "RepoRoot: $repoRoot"

        Invoke-Step -Name "Frontend unit tests (vitest run)" -CheckLastExitCode -Command {
            npm --prefix web run test:unit
        }
    }
    "frontend-build" {
        Write-Host "RepoRoot: $repoRoot"

        Invoke-Step -Name "Frontend production build" -CheckLastExitCode -Command {
            npm --prefix web run build
        }
    }
    "frontend-lint" {
        Write-Host "RepoRoot: $repoRoot"

        Invoke-Step -Name "Frontend lint" -CheckLastExitCode -Command {
            npm --prefix web run lint
        }
    }
    "docs-semantic-guard" {
        Write-Host "RepoRoot: $repoRoot"

        Invoke-Step -Name "Spec canonical map guard" -Command {
            $canonicalRelativePath = "docs/spec/semantic-surface-canonical-map.zh-CN.md"
            $canonicalReferenceToken = "semantic-surface-canonical-map.zh-CN.md"
            $canonicalAbsolutePath = Join-Path $repoRoot $canonicalRelativePath
            $summaryReferenceFiles = @(
                "README.md",
                "README.zh-CN.md",
                "docs/spec/README.md"
            )
            $publicSurfaceFiles = @(
                "web/src/App.tsx",
                "web/src/components/app-sidebar.tsx",
                "web/src/pages/AgentsPage.tsx",
                "cmd/ai-flow/root.go",
                "internal/adapters/http/chat.go",
                "internal/adapters/http/handler.go",
                "internal/adapters/http/proposal.go",
                "internal/adapters/http/initiative.go",
                "internal/adapters/http/agents.go"
            )
            $requiredReferenceFiles = @(
                "README.md",
                "README.zh-CN.md",
                "docs/spec/README.md"
            )

            if (-not (Test-Path -LiteralPath $canonicalAbsolutePath)) {
                throw "Canonical map file missing: $canonicalRelativePath"
            }

            $violations = New-Object System.Collections.Generic.List[string]

            foreach ($relativePath in $requiredReferenceFiles) {
                $absolutePath = Join-Path $repoRoot $relativePath
                if (-not (Test-Path -LiteralPath $absolutePath)) {
                    $violations.Add("Missing required reference file: $relativePath")
                    continue
                }

                $content = Get-Content -LiteralPath $absolutePath -Raw
                if ($content -notmatch [regex]::Escape($canonicalReferenceToken)) {
                    $violations.Add("Missing canonical map reference in $relativePath")
                }
            }

            $specFiles = Get-ChildItem -LiteralPath (Join-Path $repoRoot "docs/spec") -File -Filter *.md
            foreach ($file in $specFiles) {
                $normalizedRelativePath = $file.FullName.Substring($repoRoot.Length + 1) -replace "\\", "/"
                if ($normalizedRelativePath -eq $canonicalRelativePath) {
                    continue
                }

                $content = Get-Content -LiteralPath $file.FullName
                $hasStatus = $content | Select-String -Pattern '^> 状态：' -Quiet
                if (-not $hasStatus) {
                    continue
                }

                $raw = [string]::Join([Environment]::NewLine, $content)
                if ($raw -notmatch [regex]::Escape($canonicalReferenceToken)) {
                    $violations.Add("Missing canonical map reference in status-tagged spec: $normalizedRelativePath")
                }
            }

            if ($ChangedFilesPath) {
                $changedFilesAbsolutePath = if ([System.IO.Path]::IsPathRooted($ChangedFilesPath)) {
                    $ChangedFilesPath
                } else {
                    Join-Path $repoRoot $ChangedFilesPath
                }

                if (-not (Test-Path -LiteralPath $changedFilesAbsolutePath)) {
                    $violations.Add("Changed files manifest not found: $ChangedFilesPath")
                } else {
                    $changedFiles = Get-Content -LiteralPath $changedFilesAbsolutePath |
                        ForEach-Object { ($_ -replace "\\", "/").Trim() } |
                        Where-Object { $_ }

                    $changedPublicSurfaceFiles = @(
                        $changedFiles |
                            Where-Object { $publicSurfaceFiles -contains $_ } |
                            Sort-Object -Unique
                    )

                    if ($changedPublicSurfaceFiles.Count -gt 0) {
                        $touchedCanonical = $changedFiles -contains $canonicalRelativePath
                        if (-not $touchedCanonical) {
                            $violations.Add(
                                "Public surface files changed without updating canonical map: " +
                                ($changedPublicSurfaceFiles -join ", ")
                            )
                        }

                        $changedSummaryFiles = @(
                            $changedFiles |
                                Where-Object { $summaryReferenceFiles -contains $_ } |
                                Sort-Object -Unique
                        )

                        if ($changedSummaryFiles.Count -eq 0) {
                            $violations.Add(
                                "Public surface files changed without updating summary docs (README / README.zh-CN / docs/spec/README.md): " +
                                ($changedPublicSurfaceFiles -join ", ")
                            )
                        }
                    }
                }
            }

            if ($violations.Count -gt 0) {
                $violations | ForEach-Object { Write-Host $_ -ForegroundColor Red }
                throw "Spec canonical map guard failed."
            }

            Write-Host "Spec canonical map guard passed."
        }
    }
    "frontend-ci" {
        Write-Host "RepoRoot: $repoRoot"

        Invoke-Step -Name "Frontend local CI baseline" -Command {
            Invoke-RunnerTask -Name "frontend-lint"
            Invoke-RunnerTask -Name "frontend-unit"
            Invoke-RunnerTask -Name "frontend-build"
        }

        if ($WithE2E) {
            Invoke-Step -Name "Frontend browser E2E" -Command {
                & (Join-Path $PSScriptRoot "frontend-e2e.ps1")
            }
        }
    }
    "suite-p3" {
        Set-SafeTestEnvironment
        Write-Host "RepoRoot: $repoRoot"
        Write-Host "Run mode: sequential, no background jobs, no loops."
        Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

        Invoke-Step -Name "Backend unit baseline" -Command {
            Invoke-RunnerTask -Name "backend-unit"
        }

        Invoke-Step -Name "Backend integration baseline" -Command {
            Invoke-RunnerTask -Name "backend-integration"
        }

        Invoke-Step -Name "Backend E2E baseline" -Command {
            Invoke-RunnerTask -Name "backend-e2e"
        }

        Invoke-Step -Name "Frontend unit baseline" -Command {
            Invoke-RunnerTask -Name "frontend-unit"
        }

        Invoke-Step -Name "Frontend build baseline" -Command {
            Invoke-RunnerTask -Name "frontend-build"
        }

        Invoke-Step -Name "Smoke suite baseline" -Command {
            Invoke-RunnerTask -Name "suite-smoke"
        }

        Write-Host ""
        Write-Host "P3 suite completed." -ForegroundColor Green
    }
    "suite-smoke" {
        Set-SafeTestEnvironment
        Write-Host "RepoRoot: $repoRoot"
        Write-Host "Smoke target: buildable current baseline"
        Write-Host "GOMAXPROCS=$env:GOMAXPROCS, GOTEST_TIMEOUT=$env:GOTEST_TIMEOUT"

        if (-not $SkipTerminologyGate) {
            Invoke-Step -Name "Terminology gate (README + docs/spec)" -Command {
                $legacyPattern = '\\b(plan|plans|task|tasks|Run|Runs|dag|secretary)\\b'
                $hits = & rg -n --ignore-case $legacyPattern README.md docs/spec

                if ($LASTEXITCODE -eq 0) {
                    Write-Host $hits
                    throw "Legacy terminology found in README/docs/spec."
                }
                if ($LASTEXITCODE -gt 1) {
                    throw "Failed to run terminology gate with rg."
                }

                Write-Host "Terminology gate passed."
            }
        }

        Invoke-Step -Name "Spec canonical map guard" -Command {
            Invoke-RunnerTask -Name "docs-semantic-guard" -ChangedFilesPath $ChangedFilesPath
        }

        Invoke-Step -Name "Test naming gate" -Command {
            $legacyPattern = 'TestWorkItemE2E_|TestAPI_E2E_|Test.*_E2E\b|real_integration_test\.go|TODO.*integration|needs integration|补集成测试|后续补 E2E'
            $hits = & rg -n --hidden -S $legacyPattern internal cmd web

            if ($LASTEXITCODE -eq 0) {
                Write-Host $hits
                throw "Legacy test naming or legacy test TODO markers found."
            }
            if ($LASTEXITCODE -gt 1) {
                throw "Failed to run test naming gate with rg."
            }

            Write-Host "Test naming gate passed."
        }

        if (-not $SkipGoTests) {
            Invoke-Step -Name "Current backend build smoke" -CheckLastExitCode -Command {
                go build ./...
            }
        }

        Write-Host ""
        Write-Host "Smoke completed." -ForegroundColor Green
    }
}
