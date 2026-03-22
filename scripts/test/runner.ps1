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
        "frontend-ci",
        "suite-p3",
        "suite-smoke"
    )]
    [string]$Task,
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
        [switch]$IncludeACPClientIntegration,
        [switch]$WithE2E,
        [switch]$SkipTerminologyGate,
        [switch]$SkipGoTests
    )

    $params = @{
        Task = $Name
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
