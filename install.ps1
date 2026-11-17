# Sunshine Windows 一键安装脚本（PowerShell 5.1+）
#
# 用法（装最新版）：
#   irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex
#
# 装指定版本：
#   $env:SUNSHINE_VERSION = 'v0.5.19'; irm https://raw.githubusercontent.com/linrong007/Sunshine/main/install.ps1 | iex
#   .\install.ps1 -Version v0.5.19
#
# 装到指定目录（默认 $env:LOCALAPPDATA\Sunshine\bin）：
#   .\install.ps1 -InstallDir "$env:USERPROFILE\bin"
#
# 只支持 Windows x64（Release 只有 windows amd64 包）。

param(
    [string]$Version = '',
    [string]$InstallDir = ''
)

$ErrorActionPreference = 'Stop'

$Repo = 'linrong007/Sunshine'
$Prog = 'sunshine'

function Die($msg) { throw "install.ps1: $msg" }

# GitHub 要求 TLS 1.2；老版 PowerShell 5.1 默认可能更低，先加上。
try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}

# ---- 版本：参数 > 环境变量 > 最新 ----
if (-not $Version) { $Version = $env:SUNSHINE_VERSION }
if (-not $InstallDir) { $InstallDir = $env:SUNSHINE_INSTALL_DIR }
if ($Version -eq 'latest' -or -not $Version) {
    Write-Host '正在查询最新版本……'
    $Version = ''
    try {
        $latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        $Version = $latest.tag_name
    } catch {}
    if (-not $Version) {
        # API 走不通时跟 releases/latest 的跳转取 tag（只读一跳就够了，
        # 仓库已是 linrong007，跳转直达 /tag/vX.Y.Z）。
        $handler = New-Object System.Net.Http.HttpClientHandler
        $handler.AllowAutoRedirect = $false
        $client = New-Object System.Net.Http.HttpClient($handler)
        try {
            $resp = $client.GetAsync("https://github.com/$Repo/releases/latest").GetAwaiter().GetResult()
            $loc = if ($resp.Headers.Location) { $resp.Headers.Location.ToString() } else { '' }
            if ($loc -match '/tag/(v[0-9][^/]*)') { $Version = $Matches[1] }
        } catch {}
        finally { $client.Dispose(); $handler.Dispose() }
    }
    if (-not $Version) { Die '查不到最新版本，请检查网络，或用 $env:SUNSHINE_VERSION = "v0.5.19" 指定版本' }
}
$Tag = $Version
if ($Tag -notmatch '^v') { $Tag = "v$Tag" }
$Ver = $Tag.TrimStart('v')

# ---- 架构：只认 x64 ----
if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
    Die "不支持的架构 $($env:PROCESSOR_ARCHITECTURE)（只支持 x64；Release 没有 ARM64 Windows 包）"
}

$Archive = "${Prog}_${Ver}_windows_amd64.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Archive"
Write-Host "安装 $Prog $Tag（windows/amd64）……"

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("sunshine-install-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $Tmp -Force | Out-Null
try {
    # ---- 下载 + 校验 ----
    $ZipPath = Join-Path $Tmp $Archive
    Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
    $SumsPath = Join-Path $Tmp 'checksums.txt'
    Invoke-WebRequest -Uri "https://github.com/$Repo/releases/download/$Tag/checksums.txt" -OutFile $SumsPath -UseBasicParsing
    $line = (Get-Content $SumsPath | Where-Object { $_ -match " $Archive$" }) | Select-Object -First 1
    if (-not $line) { Die "校验文件里找不到 $Archive" }
    $want = ($line -split '\s+')[0].ToLower()
    $got = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash.ToLower()
    if ($got -ne $want) { Die '校验失败：下载的文件与 checksums.txt 不一致' }

    Expand-Archive -Path $ZipPath -DestinationPath $Tmp -Force
    $exe = Join-Path $Tmp "$Prog.exe"
    if (-not (Test-Path $exe)) { Die "解压后找不到可执行的 $Prog.exe" }

    # ---- 落盘：默认 $env:LOCALAPPDATA\Sunshine\bin ----
    if (-not $InstallDir) { $InstallDir = Join-Path $env:LOCALAPPDATA 'Sunshine\bin' }
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $dest = Join-Path $InstallDir "$Prog.exe"
    Copy-Item $exe $dest -Force

    # ---- 加进用户级 PATH（不写系统 PATH，不用管理员） ----
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { $userPath = '' }
    $parts = $userPath -split ';' | Where-Object { $_ -ne '' }
    if ($parts -notcontains $InstallDir) {
        $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "已把 $InstallDir 加进用户 PATH（新开的终端生效）"
    }
    if (($env:Path -split ';') -notcontains $InstallDir) { $env:Path += ";$InstallDir" }

    # ---- 验证 ----
    & $dest --version
    Write-Host "安装完成：$dest"
}
finally {
    if (Test-Path $Tmp) { Remove-Item $Tmp -Recurse -Force -ErrorAction SilentlyContinue }
}
