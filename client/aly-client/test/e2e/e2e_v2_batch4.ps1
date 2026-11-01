# ============================================================
# aly 第四批修复 E2E 测试（T 系列）
#
# 覆盖《aly-全流程测试计划-20261101.md》中 B 组（本批新特性）+
# A 组（publish→client 正常流程，用 aly-publish 真实 CLI）+
# D 组边界（版本倒退/格式/中文路径等）。
# S 系列既有回归由 update_e2e.ps1 单独覆盖，本脚本不重复。
#
# 场景：
#   T1  aly-publish config init / status / add / push 全流程（真实 CLI）
#   T2  差量删除：服务端删文件 → 客户端 apply 后删除；运行时数据保护
#   T3  首周期保守不删 + 清单缓存固化（release_manifest.json）
#   T4  回滚后清单缓存清除
#   T5  缓存损坏禁用删除同步
#   T6  服务端清单过滤口径（.merging/.chunks/ignore 不入清单）
#   T7  单文件 PROGRESS 行（大文件下载期间 3s 节流输出）
#   T8  中文/空格路径 + 深层嵌套
#   T9  版本倒退拒绝 + 非法版本号拒绝
#   T10 downloaded 状态 + 服务器不可达 → 不上报待应用（快速复验）
#
# 用法（在仓库根目录）：
#   powershell -ExecutionPolicy Bypass -File client/aly-client/test/e2e/e2e_v2_batch4.ps1
#
# ⚠ 编码要求：本文件必须保持 UTF-8 **带 BOM**（同 update_e2e.ps1）。
# ============================================================
param(
    [switch]$KeepRunning,
    [switch]$Debug
)
$script:E2EDebug = [bool]$Debug

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")).Path
$Work = Join-Path $env:TEMP ("aly-e2e4-" + [guid]::NewGuid().ToString("N").Substring(0, 8))
$Pass = 0
$Fail = 0
$ServerProc = $null

# ── 环境自愈：清除大小写重复的代理变量 ──────────────────────
# 原因：机器/用户注册表可能同时存在 HTTP_PROXY 与 http_proxy（大小写两份），
# Windows 环境块构建时两者都进入进程，Start-Process 内部用不区分大小写的
# 字典收集环境变量 → "已添加项" ArgumentException（2026-11-01 实测踩坑）。
# 本脚本与所有子进程（go build / server.exe / aly-client.exe）都不需要代理。
foreach ($k in @("HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy")) {
    Remove-Item "Env:$k" -ErrorAction SilentlyContinue
}

# ── 工具链选择 ──────────────────────────────────────────────
# PATH 里的 go 是 1.26（amd64），仅用于 server / publish-cli（二者 go.mod 口径）。
# aly-client 必须用 go1.10.8 + GOARCH=386 构建（XP 兼容），不能用 PATH 的 1.26。
$script:Go126 = "D:\Install\go\bin\go.exe"
$script:Go1108 = "C:\Users\5001494\sdk\go1.10.8\bin\go.exe"
foreach ($g in @($script:Go126, $script:Go1108)) {
    if (-not (Test-Path $g)) { throw "Go 工具链缺失: $g" }
}

function Log($msg) { Write-Host "[e2e4] $msg" -ForegroundColor Cyan }
function Ok($msg)   { Write-Host "  PASS: $msg" -ForegroundColor Green; $script:Pass++ }
function Bad($msg)  { Write-Host "  FAIL: $msg" -ForegroundColor Red; $script:Fail++ }
function Assert($cond, $msg) { if ($cond) { Ok $msg } else { Bad $msg } }

function New-TempWork() {
    New-Item -ItemType Directory -Force -Path $Work | Out-Null
    New-Item -ItemType Directory -Force -Path "$Work\server-data" | Out-Null
    New-Item -ItemType Directory -Force -Path "$Work\client" | Out-Null
    New-Item -ItemType Directory -Force -Path "$Work\gopath\src\aly\client" | Out-Null
}

function Build-All() {
    Log "构建 server..."
    Push-Location "$Root\server"
    & $script:Go126 build -o "$Work\server.exe" . | Out-Null
    Pop-Location

    Log "构建 aly-publish..."
    Push-Location "$Root\publish\publish-cli"
    & $script:Go126 build -o "$Work\aly-publish.exe" . | Out-Null
    Pop-Location

    Log "构建 aly-client (go1.10.8, GOARCH=386, GOPATH 模式，XP 兼容口径)..."
    Copy-Item -Recurse -Force "$Root\client\aly-client" "$Work\gopath\src\aly\client\aly-client"
    $env:GOPATH = "$Work\gopath"
    $env:GO111MODULE = "off"
    $env:GOOS = "windows"
    $env:GOARCH = "386"
    $env:CGO_ENABLED = "0"
    Push-Location "$Work\gopath\src\aly\client\aly-client"
    # 必须用 go1.10.8：PATH 里的 go 是 1.26，其 386 产物不保证 XP 兼容
    & $script:Go1108 build -o "$Work\client\aly-client.exe" aly/client/aly-client | Out-Null
    Pop-Location
    Remove-Item Env:GOPATH, Env:GO111MODULE, Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue

    foreach ($f in @("$Work\server.exe", "$Work\aly-publish.exe", "$Work\client\aly-client.exe")) {
        if (-not (Test-Path $f)) { throw "构建产物缺失: $f" }
    }
    Log "构建完成"
}

function Start-Server([int]$ReusePort = 0) {
    # ReusePort > 0 时复用指定端口（T10 断网复验需要：客户端 shared.json 已固化
    # 旧端口，重启后必须回到同一端口，否则"网络恢复"断言必然失败）
    if ($ReusePort -gt 0) {
        $script:Port = $ReusePort
    } else {
        $script:Port = Get-Random -Minimum 23000 -Maximum 32000
    }
    $db = "$Work\e2e.db"
    Log "启动服务端 :$Port (db=$db)"
    $script:ServerLog = "$Work\server.log"
    $script:ServerProc = Start-Process -FilePath "$Work\server.exe" -ArgumentList "-p", "$Port", "-db", $db -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput $script:ServerLog -RedirectStandardError "$Work\server.err"
    $ready = $false
    for ($i = 0; $i -lt 30; $i++) {
        Start-Sleep -Milliseconds 500
        try {
            $r = Invoke-WebRequest -Uri "http://127.0.0.1:$Port/api/server/info" -UseBasicParsing -TimeoutSec 2
            if ($r.StatusCode -eq 200) { $ready = $true; break }
        } catch { }
    }
    if (-not $ready) { throw "服务端启动失败（$Port）" }
    Log "服务端就绪"
}

function Stop-Server() {
    if ($script:ServerProc -and -not $script:ServerProc.HasExited) {
        Stop-Process -Id $script:ServerProc.Id -Force -ErrorAction SilentlyContinue
        $script:ServerProc = $null
        Log "服务端已停止"
    }
}

function Api-Get($path) {
    return Invoke-RestMethod -Uri "http://127.0.0.1:$Port$path" -Method Get -TimeoutSec 30
}
function Api-Post($path, $body) {
    return Invoke-RestMethod -Uri "http://127.0.0.1:$Port$path" -Method Post -ContentType "application/json" -Body ($body | ConvertTo-Json -Depth 6) -TimeoutSec 30
}

function Set-JsonFile($path, $obj) {
    $json = $obj | ConvertTo-Json -Depth 8
    [System.IO.File]::WriteAllText($path, $json, (New-Object System.Text.UTF8Encoding($false)))
}

# 客户端环境：pkg/UpdateFolder(client.json+version.json+aly-client.exe) + pkg/ApplicationFolder(主程序+shared.json)
function New-ClientEnv($version, $status) {
    $pkg = "$Work\client\pkg"
    if (Test-Path $pkg) { Remove-Item -Recurse -Force $pkg }
    New-Item -ItemType Directory -Force -Path "$pkg\UpdateFolder" | Out-Null
    New-Item -ItemType Directory -Force -Path "$pkg\ApplicationFolder\.updator" | Out-Null

    Copy-Item "$Work\client\aly-client.exe" "$pkg\UpdateFolder\aly-client.exe"

    Set-JsonFile "$pkg\UpdateFolder\client.json" @{ main_exe_relative_path = "../ApplicationFolder/app.exe"; must_close_process_name = @("fake_main") }
    Set-JsonFile "$pkg\UpdateFolder\version.json" @{ version_previous = "1.0.0"; version = $version; version_status = $status }
    Set-JsonFile "$pkg\ApplicationFolder\.updator\shared.json" @{ server_url = "http://127.0.0.1:$Port"; project_name = "e2e-app"; ignore_folders = @(); ignore_files = @() }

    return $pkg
}

# 运行 aly-client 命令，返回逐行 JSON 对象数组（保留进度行）。
function Invoke-ClientLines($pkg, $clientArgs) {
    $exe = "$pkg\UpdateFolder\aly-client.exe"
    $out = & $exe @clientArgs 2>$null
    $all = @()
    foreach ($item in @($out)) {
        if ($null -eq $item) { continue }
        foreach ($line in ([string]$item -split "`r?`n")) {
            $t = $line.Trim()
            if ($t -ne "") { $all += $t }
        }
    }
    return $all
}

# 取最后一行（结果行）
function Invoke-Client($pkg, $clientArgs) {
    $lines = Invoke-ClientLines $pkg $clientArgs
    if ($lines.Count -eq 0) { return $null }
    try { return ($lines[-1] | ConvertFrom-Json) } catch { return $lines[-1] }
}

function Read-VersionJson($pkg) {
    try { return (Get-Content "$pkg\UpdateFolder\version.json" -Raw -ErrorAction Stop | ConvertFrom-Json) }
    catch { return $null }
}

function Read-ClientLog($pkg) {
    $p = "$pkg\UpdateFolder\download.log"
    if (Test-Path $p) { return (Get-Content $p -Raw) } else { return "" }
}

# aly-publish 包装（在指定工作目录执行，继承 server/project 参数）。
# 全程 --json 模式：失败也走 stdout（isSuccess:false 的 JSON 行），
# 不向 stderr 打 "Error: ..."（非 json 模式的失败路径，见 root.go outputResult）——
# PS5.1 Stop 偏好下 stderr 会被包装成 NativeCommandError 直接终止脚本
# （S14 复跑失败的同一机理，2026-11-01 排查结论）。
# 返回拼接文本（stdout 全部行）。
function Invoke-Publish($workDir, $pubArgs) {
    Push-Location $workDir
    try {
        $out = & "$Work\aly-publish.exe" --server "http://127.0.0.1:$Port" --project "e2e-app" --json @pubArgs 2>$null
        $text = (@($out) | ForEach-Object { [string]$_ }) -join "`n"
        if ($script:E2EDebug) { Log "publish 输出: $text" }
        return $text
    } finally { Pop-Location }
}

# 从 --json 输出中解析最终结果行（isSuccess 存在的行，跳过 PROGRESS 行）
function Get-PublishResult($text) {
    $result = $null
    foreach ($line in ($text -split "`r?`n")) {
        $t = $line.Trim()
        if ($t -eq "") { continue }
        try {
            $o = $t | ConvertFrom-Json
            if ($null -ne $o.isSuccess) { $result = $o }
        } catch { }
    }
    return $result
}

function New-BigFile($path, $sizeMB) {
    $dir = Split-Path $path -Parent
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $fs = [System.IO.File]::Create($path)
    $fs.SetLength([int64]$sizeMB * 1MB)
    $fs.Close()
}

# ============================================================
# 主流程
# ============================================================
try {
    New-TempWork
    Build-All
    Start-Server

    # 创建项目
    $proj = Api-Post "/api/project/create_project" @{
        name = "e2e-app"; title = "E2E4 测试项目"; isForceUpdate = $false; ignoreFolders = @(); ignoreFiles = @()
    }
    Assert $proj.isSuccess "创建项目 e2e-app"

    # ============================================================
    # T1 aly-publish 全流程（真实 CLI：init → status → add → push）
    # ============================================================
    Log "=== T1 aly-publish 全流程 ==="
    $pub = "$Work\publish-src"
    New-Item -ItemType Directory -Force -Path "$pub\.updator" | Out-Null
    New-Item -ItemType Directory -Force -Path "$pub\sub" | Out-Null
    New-Item -ItemType Directory -Force -Path "$pub\a\b\c\d\e\f" | Out-Null

    Set-Content "$pub\app.exe" "main-v1-content" -Encoding UTF8
    Set-Content "$pub\sub\lib.dll" "lib-v1" -Encoding UTF8
    Set-Content "$pub\old.txt" "old-file-v1" -Encoding UTF8
    Set-Content "$pub\a\b\c\d\e\f\deep.txt" "deep-file" -Encoding UTF8
    Set-Content "$pub\中 文件.txt" "chinese-content-中文" -Encoding UTF8

    $t = Invoke-Publish $pub @("config", "init")
    Assert (Test-Path "$pub\.updator\shared.json") "T1 config init: shared.json 生成"
    Assert (Test-Path "$pub\.updator\publish.json") "T1 config init: publish.json 生成"

    $t = Invoke-Publish $pub @("status")
    $st = Get-PublishResult $t
    $unstagedPaths = @($st.data.unstaged | ForEach-Object { $_.relativePath })
    Assert ($st.isSuccess -eq $true) "T1 status: 命令成功"
    Assert ($unstagedPaths -contains "app.exe") "T1 status: 列出未暂存文件 app.exe"
    $t = Invoke-Publish $pub @("add", "--all")
    $t = Invoke-Publish $pub @("status")
    $st = Get-PublishResult $t
    Assert (@($st.data.unstaged).Count -eq 0) "T1 add --all: 全部暂存（unstaged 为空）"

    $t = Invoke-Publish $pub @("push", "--version", "1.0.0", "--message", "T1 初始版本")
    $pr = Get-PublishResult $t
    Assert ($pr.isSuccess -eq $true) "T1 push 1.0.0: 推送成功"

    # 服务端文件验证
    $files = Api-Get "/api/file/get_all_files/e2e-app"
    $paths = @($files.data | ForEach-Object { $_.fileRelativePath })
    Assert ($paths -contains "app.exe") "T1 服务端有 app.exe"
    Assert ($paths -contains "old.txt") "T1 服务端有 old.txt"
    Assert ($paths -contains "a/b/c/d/e/f/deep.txt") "T1 服务端有深层嵌套文件"
    Assert ($paths -contains "中 文件.txt") "T1 服务端有中文文件名"

    # ============================================================
    # T2/T3: 客户端首周期 download（差量删除首周期 = 保守不删 + 缓存固化）
    # ============================================================
    Log "=== T3 首周期 download：保守不删 + 清单缓存固化 ==="
    $pkg = New-ClientEnv "1.0.0" "applied"
    Copy-Item "$pub\app.exe" "$pkg\ApplicationFolder\app.exe"
    Copy-Item "$pub\sub\lib.dll" "$pkg\ApplicationFolder\sub\lib.dll" -Force
    New-Item -ItemType Directory -Force -Path "$pkg\ApplicationFolder\sub" | Out-Null
    Copy-Item "$pub\old.txt" "$pkg\ApplicationFolder\old.txt" -Force
    # 主程序运行时生成文件（不在服务端清单）
    New-Item -ItemType Directory -Force -Path "$pkg\ApplicationFolder\runtime" | Out-Null
    Set-Content "$pkg\ApplicationFolder\runtime\user.cfg" "runtime-data-1" -Encoding UTF8

    # 发布 1.0.1：删除 old.txt（服务端删文件），新增 new.txt
    Remove-Item "$pub\old.txt"
    Set-Content "$pub\new.txt" "new-file-v1" -Encoding UTF8
    $t = Invoke-Publish $pub @("add", "--all")
    $t = Invoke-Publish $pub @("push", "--version", "1.0.1", "--message", "删 old 增 new")

    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "T3 download 1.0.1 成功"
    # 首周期无缓存：应跳过删除（old.txt 仍在版本目录——apply 后仍在）
    $log = Read-ClientLog $pkg
    Assert ($log -match "no cached manifest \(first cycle\), skip delete-sync") "T3 首周期日志：跳过删除同步"
    # 清单缓存已固化
    $cached = "$pkg\UpdateFolder\release_manifest.json"
    Assert (Test-Path $cached) "T3 清单缓存已生成（release_manifest.json）"
    $cachedObj = Get-Content $cached -Raw | ConvertFrom-Json
    Assert (@($cachedObj | Where-Object { $_ -eq "app.exe" }).Count -eq 1) "T3 缓存含 app.exe"
    Assert (@($cachedObj | Where-Object { $_ -eq "new.txt" }).Count -eq 1) "T3 缓存含 new.txt"
    Assert (@($cachedObj | Where-Object { $_ -eq "old.txt" }).Count -eq 0) "T3 缓存不含已删的 old.txt"
    Assert (-not (Test-Path "$cached.tmp")) "T3 无 .tmp 残留"

    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "T3 apply 1.0.1 成功"
    # 首周期未删：old.txt 保留（1.0.0 拷入版本目录的 old.txt 本轮不清）
    Assert (Test-Path "$pkg\ApplicationFolder\old.txt") "T3 首周期保守：old.txt 未被删除"
    # 运行时数据保护
    Assert ((Get-Content "$pkg\ApplicationFolder\runtime\user.cfg" -Raw).Trim() -eq "runtime-data-1") "T3 运行时数据 user.cfg 完整保留"

    # ============================================================
    # T2 差量删除第二周期：服务端删除 → 客户端同步删除 + 运行时数据保护
    # ============================================================
    Log "=== T2 差量删除：服务端删 new.txt → 第二周期同步删除 ==="
    Remove-Item "$pub\new.txt"
    Set-Content "$pub\app.exe" "main-v2-content" -Encoding UTF8
    Set-Content "$pkg\ApplicationFolder\runtime\user.cfg" "runtime-data-2" -Encoding UTF8
    $t = Invoke-Publish $pub @("add", "--all")
    $t = Invoke-Publish $pub @("push", "--version", "1.0.2", "--message", "删 new 改 app")

    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "T2 download 1.0.2 成功"
    $log = Read-ClientLog $pkg
    Assert ($log -match "delete-sync removed 1 file\(s\) deleted by server: new\.txt") "T2 删除日志：仅删 new.txt"
    Assert ($log -notmatch "runtime") "T2 删除日志不含运行时文件"

    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "T2 apply 1.0.2 成功"
    Assert (-not (Test-Path "$pkg\ApplicationFolder\new.txt")) "T2 apply 后 new.txt 已删除（服务端删除同步）"
    Assert ((Get-Content "$pkg\ApplicationFolder\runtime\user.cfg" -Raw).Trim() -eq "runtime-data-2") "T2 运行时数据 user.cfg 保留（数据丢失防护核心断言）"
    Assert ((Get-Content "$pkg\ApplicationFolder\app.exe" -Raw).Trim() -eq "main-v2-content") "T2 apply 后 app.exe 为 v2 内容"
    # 版本目录里有 old.txt（首周期物化拷入）——差量集不含它（不在任何清单），应保留
    # （主目录经 apply 重命名后 old.txt 随 1.0.1 目录内容进入 MainFolder，服务端已删但本轮差量基线是 1.0.1 清单——不含 old.txt，故不删）
    $logAfter = Read-ClientLog $pkg
    Assert ($logAfter -notmatch "old\.txt") "T2 old.txt 未进入任何删除集"

    # ============================================================
    # T4 回滚后清单缓存清除
    # ============================================================
    Log "=== T4 回滚后清单缓存清除 ==="
    $rb = Invoke-Client $pkg @("rollback", "--version", "1.0.1")
    Assert ($rb.isSuccess -eq $true) "T4 rollback 1.0.1 成功"
    Assert (-not (Test-Path "$pkg\UpdateFolder\release_manifest.json")) "T4 回滚后清单缓存已清除"

    # ============================================================
    # T5 缓存损坏禁用删除同步
    # ============================================================
    Log "=== T5 缓存损坏 → 删除同步禁用 ==="
    # 重新 download 建立缓存
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "T5 重建缓存 download 成功"
    Assert (Test-Path "$pkg\UpdateFolder\release_manifest.json") "T5 缓存重建"
    # 写坏缓存
    [System.IO.File]::WriteAllText("$pkg\UpdateFolder\release_manifest.json", "{broken json", (New-Object System.Text.UTF8Encoding($false)))
    # 服务端再删一个文件
    Remove-Item "$pub\sub\lib.dll"
    $t = Invoke-Publish $pub @("add", "--all")
    $t = Invoke-Publish $pub @("push", "--version", "1.0.3", "--message", "删 lib")
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "T5 缓存损坏时 download 仍成功"
    $log = Read-ClientLog $pkg
    Assert ($log -match "read cached manifest failed \(delete-sync disabled\)") "T5 日志：缓存损坏禁用删除同步"
    # 坏缓存应被好缓存覆盖（本轮末尾固化）
    $cachedNow = Get-Content "$pkg\UpdateFolder\release_manifest.json" -Raw | ConvertFrom-Json
    Assert (@($cachedNow | Where-Object { $_ -eq "app.exe" }).Count -eq 1) "T5 损坏缓存被本轮固化覆盖"
    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "T5 apply 1.0.3 成功"

    # ============================================================
    # T6 服务端清单过滤口径（.merging/.chunks 不入清单）
    # ============================================================
    Log "=== T6 服务端清单过滤口径 ==="
    # 服务端数据目录 = server.exe 所在目录/data/{project}（GetProjectWorkPath）
    $dataDir = Join-Path $Work "data\e2e-app"
    if (Test-Path $dataDir) {
        New-Item -ItemType Directory -Force -Path "$dataDir\probe.chunks" | Out-Null
        Set-Content "$dataDir\probe.merging" "partial" -Encoding UTF8
        Set-Content "$dataDir\x.tmp" "temp" -Encoding UTF8
        # 触发重新发布固化清单
        Set-Content "$pub\app.exe" "main-v3-content" -Encoding UTF8
        $t = Invoke-Publish $pub @("add", "--all")
        $t = Invoke-Publish $pub @("push", "--version", "1.0.4", "--message", "T6 清单口径")
        $m = Api-Get "/api/file/get_release_manifest/e2e-app"
        $mfiles = @($m.data.files)
        Assert ($mfiles -notcontains "probe.merging") "T6 清单不含 .merging"
        Assert (($mfiles | Where-Object { $_ -like "*probe.chunks*" }).Count -eq 0) "T6 清单不含 .chunks"
        # x.tmp 是合法项目文件（IsUploadTempPath 只拦 .chunks/.merging），
        # 手工注入数据目录时它会入清单——此断言验证"过滤口径与实现一致"
        Assert (($mfiles | Where-Object { $_ -eq "x.tmp" }).Count -eq 1) "T6 普通文件 x.tmp 正常入清单（口径与实现一致）"
        Assert ($mfiles -notcontains ".updator_manifest.json") "T6 清单自排除（.updator_manifest.json 不在清单内）"
        # 清理探测残留
        Remove-Item "$dataDir\probe.chunks" -Recurse -Force
        Remove-Item "$dataDir\probe.merging", "$dataDir\x.tmp" -Force
    } else {
        Log "T6 服务端数据目录未找到: $dataDir"
        Bad "T6 未执行（数据目录未找到）"
    }

    # ============================================================
    # T7 单文件 PROGRESS 行（大文件 >3s 下载）
    # ============================================================
    Log "=== T7 单文件 PROGRESS 行 ==="
    New-BigFile "$pub\big.bin" 30
    $t = Invoke-Publish $pub @("add", "--all")
    $t = Invoke-Publish $pub @("push", "--version", "2.0.0", "--message", "T7 大文件")
    $pkg7 = New-ClientEnv "1.0.4" "applied"
    Copy-Item "$pub\app.exe" "$pkg7\ApplicationFolder\app.exe" -Force
    # 本地内容与 v1 不同，确保重新下载 big.bin
    $lines = Invoke-ClientLines $pkg7 @("download_update")
    $jsonLines = @()
    foreach ($l in $lines) { try { $jsonLines += ($l | ConvertFrom-Json) } catch { } }
    $resultLine = $jsonLines[-1]
    Assert ($resultLine.isSuccess -eq $true) "T7 download 成功"
    $progressLines = @($jsonLines | Where-Object { $_.data -and $_.data.status -eq "PROGRESS" })
    Assert ($progressLines.Count -ge 1) "T7 出现 PROGRESS 行（实际 $($progressLines.Count) 行）"
    if ($progressLines.Count -ge 1) {
        $p0 = $progressLines[0].data
        Assert ($p0.index -ge 1 -and $p0.total -ge 1) "T7 PROGRESS 行含 index/total"
        Assert ($p0.downloaded_bytes -gt 0) "T7 PROGRESS 行含 downloaded_bytes > 0"
        Assert ($p0.total_bytes -ge 30MB) "T7 PROGRESS 行 total_bytes 正确"
        Assert ($p0.speed_bps -gt 0) "T7 PROGRESS 行 speed_bps > 0"
    }
    # 完成语义：最后一行 data 为 null（isSuccess:true）
    Assert ($resultLine.data -eq $null) "T7 末行 data:null 完成语义保持"

    # ============================================================
    # T8 中文/空格路径 + 深层嵌套（下载侧验证，上传侧已在 T1）
    # ============================================================
    Log "=== T8 中文/空格路径下载侧 ==="
    $du = Invoke-Client $pkg7 @("download_update")  # 无更高版本则跳过
    $pkg8 = New-ClientEnv "2.0.0" "applied"
    # MainFolder 放旧内容触发差异下载
    Set-Content "$pkg8\ApplicationFolder\app.exe" "main-v0-content" -Encoding UTF8
    $du = Invoke-Client $pkg8 @("download_update")
    Assert ($du.isSuccess -eq $true) "T8 download（含中文/深路径）成功"
    $au = Invoke-Client $pkg8 @("apply_update")
    Assert ($au.isSuccess -eq $true) "T8 apply 成功"
    Assert (Test-Path "$pkg8\ApplicationFolder\中 文件.txt") "T8 中文文件名落地"
    $cn = Get-Content "$pkg8\ApplicationFolder\中 文件.txt" -Raw -Encoding UTF8
    Assert ($cn -match "chinese-content") "T8 中文内容完整"
    Assert (Test-Path "$pkg8\ApplicationFolder\a\b\c\d\e\f\deep.txt") "T8 深层嵌套落地"

    # ============================================================
    # T9 版本倒退 + 非法版本号（服务端校验）+ 合法格式矩阵（真实 CLI push）
    # ============================================================
    Log "=== T9 版本校验 ==="
    # 倒退：当前最新 2.0.0，push 1.0.9 应拒绝（--json 模式失败也是 stdout JSON 行，
    # 解析 isSuccess/errorMsg 断言，不依赖人读文本；PS5.1 Stop 偏好下 stderr 会终止脚本）
    $t = Invoke-Publish $pub @("add", "--all")
    $t = Invoke-Publish $pub @("push", "--version", "1.0.9", "--message", "倒退测试")
    $pr = Get-PublishResult $t
    Assert ($pr.isSuccess -eq $false) "T9 版本倒退 push 被拒绝（isSuccess:false）"
    Assert ($pr.errorMsg -match "不大于|倒退") "T9 拒绝原因含防倒退提示（实际: $($pr.errorMsg)）"
    # 非法版本号（直接打服务端 API 绕过 CLI 前置校验，测服务端正则与时间校验；
    # Api-Post 不消费暂存区文件，后续 add --all 重新暂存，无残留错位问题）
    # 注意字段名是 time（服务端 PublishVersion 的 json tag 为 `time`），写成 timeStr 会被
    # gin 静默忽略 → 服务端 time 为空 → 时间格式校验被跳过，断言形同虚设。
    $r = Api-Post "/api/project/publish_version" @{ projectName = "e2e-app"; version = "1..9"; logs = @("x"); time = (Get-Date -Format "yyyy-MM-dd HH:mm:ss") }
    Assert ($r.isSuccess -eq $false) "T9 非法版本号 1..9 被拒绝"
    $r = Api-Post "/api/project/publish_version" @{ projectName = "e2e-app"; version = "abc"; logs = @("x"); time = (Get-Date -Format "yyyy-MM-dd HH:mm:ss") }
    Assert ($r.isSuccess -eq $false) "T9 非法版本号 abc 被拒绝"
    $r = Api-Post "/api/project/publish_version" @{ projectName = "e2e-app"; version = ""; logs = @("x"); time = (Get-Date -Format "yyyy-MM-dd HH:mm:ss") }
    Assert ($r.isSuccess -eq $false) "T9 空版本号被拒绝"
    $r = Api-Post "/api/project/publish_version" @{ projectName = "e2e-app"; version = "2.0.1"; logs = @("x"); time = "bad-time-format" }
    Assert ($r.isSuccess -eq $false) "T9 非法时间格式被拒绝"
    Assert ($r.errorMsg -match "时间格式") "T9 拒绝原因含时间格式提示（实际: $($r.errorMsg)）"
    # 倒退失败后暂存区保留（staging.Clear 仅在成功后调用）：暂存的仍是 2.0.0 源
    # 文件（app.exe 为 main-v3-content），下一个 push 2.1.0 消费同一批文件。
    # 合法格式矩阵（真实 CLI push，同时覆盖 PB 组 CLI 正常发版链路）：
    # 序列严格递增：2.0.0 → 2.1.0 → 2.1.1-beta → 2.1.2+build.4 → 2.2.0
    foreach ($v in @("2.1.0", "2.1.1-beta", "2.1.2+build.4", "2.2.0")) {
        Set-Content "$pub\app.exe" "ver-$v" -Encoding UTF8
        $t = Invoke-Publish $pub @("add", "--all")
        $t = Invoke-Publish $pub @("push", "--version", $v, "--message", "v$v")
        $pr = Get-PublishResult $t
        Assert ($pr.isSuccess -eq $true) "T9 合法版本 $v push 成功（isSuccess:true）"
    }

    # ============================================================
    # T11 aly-publish CLI 边界（PB 组自动化子集：A5/A6/B2/B8）
    # ============================================================
    Log "=== T11 publish CLI 边界 ==="
    # PB-A5：push 后 status 应干净（本地与服务端一致，无 unstaged/staged）
    $t = Invoke-Publish $pub @("status")
    $st = Get-PublishResult $t
    Assert ($st.isSuccess -eq $true) "T11 PB-A5 status 成功"
    Assert (@($st.data.unstaged).Count -eq 0 -and @($st.data.staged).Count -eq 0) "T11 PB-A5 push 后工作区干净（无差异）"

    # PB-A6：dry-run 不实际上传、不发布（服务端项目版本仍是 2.2.0）
    Set-Content "$pub\app.exe" "dryrun-content" -Encoding UTF8
    $t = Invoke-Publish $pub @("add", "--all")
    $t = Invoke-Publish $pub @("push", "--version", "2.3.0", "--message", "dry-run 测试", "--dry-run")
    $pr = Get-PublishResult $t
    # dry-run 按实现返回 false（仅校验预演），但不应有上传副作用
    $projInfo = Api-Get "/api/project/get_project_by_name/e2e-app"
    Assert ($projInfo.data.version -eq "2.2.0") "T11 PB-A6 dry-run 未改变服务端版本（仍 2.2.0）"
    # 恢复（后续 T10 需要 2.2.0 最新版）
    Set-Content "$pub\app.exe" "ver-2.2.0" -Encoding UTF8
    $t = Invoke-Publish $pub @("add", "--all")

    # PB-B2：push 缺 --message 拒绝
    $t = Invoke-Publish $pub @("push", "--version", "2.3.0")
    $pr = Get-PublishResult $t
    Assert ($pr.isSuccess -eq $false) "T11 PB-B2 缺 message 被拒绝"
    Assert ($pr.errorMsg -match "message|日志") "T11 PB-B2 拒绝原因指向 message（实际: $($pr.errorMsg)）"

    # PB-B8：暂存后修改文件 → push MD5 复核冲突拒绝
    Set-Content "$pub\app.exe" "conflict-content" -Encoding UTF8   # add 之后改文件（staging 记录的 MD5 失配）
    $t = Invoke-Publish $pub @("push", "--version", "2.3.0", "--message", "冲突测试")
    $pr = Get-PublishResult $t
    Assert ($pr.isSuccess -eq $false) "T11 PB-B8 暂存后修改文件 push 被拒绝（MD5 复核）"
    Assert ($pr.errorMsg -match "修改|重新 add|conflict") "T11 PB-B8 拒绝原因指向 MD5 复核（实际: $($pr.errorMsg)）"
    # 恢复供 T10 使用
    Set-Content "$pub\app.exe" "ver-2.2.0" -Encoding UTF8
    $t = Invoke-Publish $pub @("add", "--all")

    # ============================================================
    # T10 服务器不可达：downloaded 不上报待应用
    # ============================================================
    Log "=== T10 服务器不可达 ==="
    $pkg10 = New-ClientEnv "2.1.0" "applied"
    Set-Content "$pkg10\ApplicationFolder\app.exe" "main-v9-content" -Encoding UTF8
    $du = Invoke-Client $pkg10 @("download_update")
    Assert ($du.isSuccess -eq $true) "T10 download 2.2.0 成功（注：此处最新为 2.2.0）"
    # 断网：downloaded 状态 + 服务器不可达 → isSuccess:true + has_update:false
    #（checkUpdatePending 断网分支语义：不挂死、不报待应用、内容保留待联网续跑）
    $serverPort = $Port
    Stop-Server
    $cu = Invoke-Client $pkg10 @("check_update")
    Assert ($cu.isSuccess -eq $true) "T10 断网 check 命令本身成功（isSuccess:true 不挂死）"
    Assert ($cu.data.has_update -eq $false) "T10 断网不报待应用（has_update:false）"
    Assert ($cu.data.current_version -eq "2.1.0") "T10 断网 current_version 为实际内容版本 2.1.0"
    # 网络恢复：复用原端口重启（客户端 shared.json 固化了原端口 URL）
    Start-Server -ReusePort $serverPort
    $cu = Invoke-Client $pkg10 @("check_update")
    Assert ($cu.isSuccess -eq $true) "T10 网络恢复 check 成功"
    Assert ($cu.data.has_update -eq $true) "T10 网络恢复上报待应用（has_update:true，待 apply 2.2.0）"
    Assert ($cu.data.new_version -eq "2.2.0") "T10 网络恢复 new_version 为 2.2.0"

    # ============================================================
    # 汇总
    # ============================================================
    Log ("===== 完成: PASS=$Pass FAIL=$Fail =====")
} finally {
    if (-not $KeepRunning) {
        Stop-Server
        if ($Fail -eq 0) {
            Log "清理工作目录 $Work"
            Remove-Item -Recurse -Force $Work -ErrorAction SilentlyContinue
        } else {
            Log "存在失败用例，保留工作目录供排查: $Work"
        }
    } else {
        Log "KeepRunning: 保留 $Work (端口 $Port)"
    }
}
if ($Fail -gt 0) { exit 1 } else { exit 0 }
