[CmdletBinding()]
param(
    # LiteBox runner 路径（可选，传入后会额外校验文件存在）
    [Parameter(Mandatory = $false)]
    [string]$RunnerPath = '',

    # 是否检查“构建 runner”所需依赖
    [Parameter(Mandatory = $false)]
    [switch]$ForBuildRunner,

    # 是否检查“运行 litebox-acp / acp-smoke”所需依赖
    [Parameter(Mandatory = $false)]
    [switch]$ForACP
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function New-CheckResult {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,
        [Parameter(Mandatory = $true)]
        [bool]$Required,
        [Parameter(Mandatory = $true)]
        [bool]$Passed,
        [Parameter(Mandatory = $true)]
        [string]$Detail,
        [Parameter(Mandatory = $false)]
        [string]$Suggestion = ''
    )

    [pscustomobject]@{
        Name       = $Name
        Required   = $Required
        Status     = if ($Passed) { 'OK' } else { 'MISSING' }
        Detail     = $Detail
        Suggestion = $Suggestion
        Passed     = $Passed
    }
}

function Resolve-CommandPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $cmd) {
        return ''
    }
    return $cmd.Source
}

function Find-VSBuildToolsWithVC {
    $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
    if (-not (Test-Path -LiteralPath $vswhere)) {
        return ''
    }

    $path = & $vswhere `
        -latest `
        -products * `
        -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 `
        -property installationPath

    if (-not $path) {
        return ''
    }
    return ($path | Select-Object -First 1).ToString().Trim()
}

function Add-ToolCheck {
    param(
        [Parameter(Mandatory = $true)]
        [System.Collections.Generic.List[object]]$Results,
        [Parameter(Mandatory = $true)]
        [string]$ToolName,
        [Parameter(Mandatory = $true)]
        [bool]$Required,
        [Parameter(Mandatory = $false)]
        [string]$Suggestion = ''
    )

    $path = Resolve-CommandPath -Name $ToolName
    $ok = -not [string]::IsNullOrWhiteSpace($path)
    $detail = if ($ok) { $path } else { '未在 PATH 中找到' }
    $Results.Add((New-CheckResult -Name $ToolName -Required $Required -Passed $ok -Detail $detail -Suggestion $Suggestion))
}

$results = [System.Collections.Generic.List[object]]::new()

$isWindowsHost = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
$results.Add((New-CheckResult `
    -Name 'OS' `
    -Required $true `
    -Passed $isWindowsHost `
    -Detail ("当前系统：{0}, 架构：{1}" -f [System.Runtime.InteropServices.RuntimeInformation]::OSDescription, $arch) `
    -Suggestion 'LiteBox Windows runner 需要在 Windows x64 环境下运行'))

$isX64 = $arch -eq 'X64'
$results.Add((New-CheckResult `
    -Name 'Windows x64' `
    -Required $true `
    -Passed $isX64 `
    -Detail ("检测到架构：{0}" -f $arch) `
    -Suggestion '请使用 x64 Windows（amd64）环境'))

Add-ToolCheck -Results $results -ToolName 'pwsh' -Required $true -Suggestion '安装 PowerShell 7'
Add-ToolCheck -Results $results -ToolName 'tar' -Required $true -Suggestion 'Windows 10/11 通常内置 tar（bsdtar）'
Add-ToolCheck -Results $results -ToolName 'git' -Required $false -Suggestion '建议安装 Git，便于同步 LiteBox 与本项目代码'
Add-ToolCheck -Results $results -ToolName 'wsl' -Required $false -Suggestion 'WSL 不是必需，但在构建 Linux rootfs/调试 Linux 程序时很有帮助'

if ($ForBuildRunner) {
    Add-ToolCheck -Results $results -ToolName 'rustup' -Required $false -Suggestion '建议安装 Rustup（https://rustup.rs），便于统一管理 toolchain'
    Add-ToolCheck -Results $results -ToolName 'cargo' -Required $true -Suggestion '通过 rustup 安装 cargo（默认随 Rust toolchain 提供）'
    Add-ToolCheck -Results $results -ToolName 'rustc' -Required $true -Suggestion '安装 Rust 编译器 rustc（通常与 cargo 同时安装）'

    $vcPath = Find-VSBuildToolsWithVC
    $vcOK = -not [string]::IsNullOrWhiteSpace($vcPath)
    $vcDetail = ''
    if ($vcOK) {
        $vcDetail = $vcPath
    }
    else {
        $vcDetail = '未检测到带 VC 工具链的 VS Build Tools'
    }
    $results.Add((New-CheckResult `
        -Name 'MSVC C++ Build Tools' `
        -Required $true `
        -Passed $vcOK `
        -Detail $vcDetail `
        -Suggestion '安装 Visual Studio Build Tools，并勾选 “Desktop development with C++”'))
}

if ($ForACP) {
    Add-ToolCheck -Results $results -ToolName 'go' -Required $true -Suggestion '安装 Go（建议与仓库 go.mod 中版本对齐）'
}

if (-not [string]::IsNullOrWhiteSpace($RunnerPath)) {
    $runnerExists = Test-Path -LiteralPath $RunnerPath
    $runnerDetail = ''
    if ($runnerExists) {
        $runnerDetail = (Resolve-Path -LiteralPath $RunnerPath).Path
    }
    else {
        $runnerDetail = "文件不存在：$RunnerPath"
    }
    $results.Add((New-CheckResult `
        -Name 'LiteBox Runner File' `
        -Required $true `
        -Passed $runnerExists `
        -Detail $runnerDetail `
        -Suggestion '先执行 build-runner.windows.ps1 构建 runner，或修正 RunnerPath'))
}

Write-Host ''
Write-Host '=== LiteBox Windows 依赖检查 ===' -ForegroundColor Cyan
Write-Host ('ForBuildRunner={0}, ForACP={1}' -f $ForBuildRunner.IsPresent, $ForACP.IsPresent)
Write-Host ''

$results |
    Select-Object Name, Required, Status, Detail |
    Format-Table -AutoSize

$requiredMissing = @($results | Where-Object { -not $_.Passed -and $_.Required })
$optionalMissing = @($results | Where-Object { -not $_.Passed -and -not $_.Required })

Write-Host ''
if ($requiredMissing.Count -eq 0) {
    Write-Host '结论：必需依赖检查通过。' -ForegroundColor Green
}
else {
    Write-Host ("结论：缺少 {0} 项必需依赖。" -f $requiredMissing.Count) -ForegroundColor Red
    foreach ($item in $requiredMissing) {
        Write-Host ("- {0}: {1}" -f $item.Name, $item.Suggestion)
    }
}

if ($optionalMissing.Count -gt 0) {
    Write-Host ''
    Write-Host ("可选缺失：{0} 项（不阻塞当前模式）" -f $optionalMissing.Count) -ForegroundColor Yellow
    foreach ($item in $optionalMissing) {
        Write-Host ("- {0}: {1}" -f $item.Name, $item.Suggestion)
    }
}

if ($requiredMissing.Count -gt 0) {
    exit 1
}
exit 0
