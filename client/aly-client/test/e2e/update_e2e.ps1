# ============================================================
# aly-client 更新机制端到端测试（E2E）
#
# 使用真实服务端（server/）与真实 aly-client.exe（386 构建）进程，
# 覆盖以下场景：
#   S1 正常更新：check -> download -> apply 成功，版本切换/备份/状态机正确
#   S2 杀进程：apply 时目录被进程占用（explorer 子文件夹 / 文件独占 / cmd CWD）
#       -> 探测并击杀占用者 -> 更新成功
#   S3 更新失败：apply 阶段被独占文件卡住 -> 回退 downloaded + 旧版仍可用
#   S4 崩溃恢复：applying + 主目录缺失 + 版本目录存在 -> apply 恢复成功
#   S5 下载后服务端又发新版：已下载 V2 未应用，服务端发 V3 -> 重新下载 V3
#   S6 回滚：apply 后 list_rollback_versions + rollback 到指定历史版本
#   S7 服务端 get_all_files 缓存：连续两次请求，第二次命中缓存（含 md5/sha256）
#
# 用法（在仓库根目录）：
#   powershell -ExecutionPolicy Bypass -File client/aly-client/test/e2e/update_e2e.ps1
#
# 依赖：go 工具链（构建 server + client 386 + fake_main）、curl.exe
# 注意：脚本会启动/停止一个临时服务端进程（随机端口、临时 db），不影响现有数据。
#
# ⚠ 编码要求：本文件必须保持 UTF-8 **带 BOM**。
#   Windows PowerShell 5.1 读取无 BOM 的 .ps1 会按本地代码页(GBK)解码，
#   中文注释变乱码导致解析失败（Missing ')' in function parameter list）。
#   若编辑后报解析错误，用 Python 补回 BOM：
#     python -c "p=r'...\update_e2e.ps1';d=open(p,'rb').read();open(p,'wb').write(b'\xef\xbb\xbf'+d)"
# ============================================================
param(
    [switch]$KeepRunning,  # 保留服务端进程与工作目录（调试用）
    [switch]$Debug         # 打印 aly-client 原始输出（调试用）
)
$script:E2EDebug = [bool]$Debug

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")).Path   # 仓库根
$Work = Join-Path $env:TEMP ("aly-e2e-" + [guid]::NewGuid().ToString("N").Substring(0, 8))
$Pass = 0
$Fail = 0
$ServerProc = $null

function Log($msg) { Write-Host "[e2e] $msg" -ForegroundColor Cyan }
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
    go build -o "$Work\server.exe" . | Out-Null
    Pop-Location

    Log "构建 aly-client (GOARCH=386, GOPATH 模式)..."
    Copy-Item -Recurse -Force "$Root\client\aly-client" "$Work\gopath\src\aly\client\aly-client"
    $env:GOPATH = "$Work\gopath"
    $env:GO111MODULE = "off"
    $env:GOOS = "windows"
    $env:GOARCH = "386"
    Push-Location "$Work\gopath\src\aly\client\aly-client"
    go build -o "$Work\client\aly-client.exe" aly/client/aly-client | Out-Null
    Pop-Location
    Remove-Item Env:GOPATH, Env:GO111MODULE, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue

    Log "构建 fake_main（GOPATH 模式，避免仓库根无 go.mod 的模块报错）..."
    # 复制 fake_main 到临时 GOPATH 下用 import path 构建（与 aly-client 一致）
    New-Item -ItemType Directory -Force -Path "$Work\gopath\src\aly\e2e\fake_main" | Out-Null
    Copy-Item "$Root\client\aly-client\test\e2e\fake_main\main.go" "$Work\gopath\src\aly\e2e\fake_main\main.go"
    $env:GOPATH = "$Work\gopath"
    $env:GO111MODULE = "off"
    $env:GOOS = "windows"
    $env:GOARCH = "386"
    Push-Location "$Work\gopath\src\aly\e2e\fake_main"
    go build -o "$Work\fake_main.exe" aly/e2e/fake_main | Out-Null
    Pop-Location
    Remove-Item Env:GOPATH, Env:GO111MODULE, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue

    foreach ($f in @("$Work\server.exe", "$Work\client\aly-client.exe", "$Work\fake_main.exe")) {
        if (-not (Test-Path $f)) { throw "构建产物缺失: $f" }
    }
    Log "构建完成"
}

function Start-Server() {
    $script:Port = Get-Random -Minimum 23000 -Maximum 32000
    $db = "$Work\e2e.db"
    Log "启动服务端 :$Port (db=$db)"
    $script:ServerProc = Start-Process -FilePath "$Work\server.exe" -ArgumentList "-p", "$Port", "-db", $db -PassThru -WindowStyle Hidden
    # 等待就绪
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
function Api-Upload($project, $relFile, $localFile) {
    $r = & curl.exe -s -X POST "http://127.0.0.1:$Port/api/file/upload_file" -F "projectName=$project" -F "relativeFileName=$relFile" -F "file=@$localFile"
    $obj = $r | ConvertFrom-Json
    return $obj.isSuccess
}

# 写 UTF-8 无 BOM 的 JSON 文件（Windows PowerShell Set-Content -Encoding UTF8 会带 BOM，
# Go json.Unmarshal 不处理 BOM 会解析失败）
function Set-JsonFile($path, $obj) {
    $json = $obj | ConvertTo-Json -Depth 8
    [System.IO.File]::WriteAllText($path, $json, (New-Object System.Text.UTF8Encoding($false)))
}

# 布置客户端环境：pkg/UpdateFolder(client.json+version.json+aly-client.exe) + pkg/ApplicationFolder(主程序+shared.json)
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

# 运行 aly-client 命令，返回 JSON 对象。
# 注意：不使用 2>&1（Windows PowerShell 会把 stderr 变 ErrorRecord 混入），
# 只取 stdout；stdout 可能被拆成数组或合并为含 \r\n 的单字符串，统一按行拆分后取最后一行 JSON。
function Invoke-Client($pkg, $clientArgs) {
    $exe = "$pkg\UpdateFolder\aly-client.exe"
    $out = & $exe @clientArgs 2>$null
    # 归一化为字符串数组（处理"单字符串含换行"与"已拆分数组"两种形态）
    $all = @()
    foreach ($item in @($out)) {
        if ($null -eq $item) { continue }
        $s = [string]$item
        foreach ($line in ($s -split "`r?`n")) {
            $t = $line.Trim()
            if ($t -ne "") { $all += $t }
        }
    }
    if ($script:E2EDebug) { Log "client 输出: $($all -join ' | ')" }
    if ($all.Count -eq 0) { return $null }
    $last = $all[-1]
    try { return ($last | ConvertFrom-Json) } catch { return $last }
}

# ---------- 服务端准备：建项目 + 发布版本 ----------
function Publish-Version($version, $mainContent, $extraFile) {
    # 覆盖主程序文件
    Set-Content -Path "$Work\fake-main-content.txt" -Value $mainContent -Encoding UTF8
    Assert (Api-Upload "e2e-app" "app.exe" "$Work\fake-main-content.txt") "发布 $version : 上传 app.exe"
    if ($extraFile) {
        Set-Content -Path "$Work\fake-extra.txt" -Value $extraFile -Encoding UTF8
        Assert (Api-Upload "e2e-app" "data\config.ini" "$Work\fake-extra.txt") "发布 $version : 上传 config.ini"
    }
    $r = Api-Post "/api/project/publish_version" @{
        projectName = "e2e-app"; version = $version; logs = @("v$version"); timeStr = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
    }
    Assert $r.isSuccess "发布 $version : publish_version"
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
        name = "e2e-app"; title = "E2E 测试项目"; isForceUpdate = $false; ignoreFolders = @(); ignoreFiles = @()
    }
    Assert $proj.isSuccess "创建项目 e2e-app"

    # 发布 1.0.0（初始版本）
    Publish-Version "1.0.0" "main-v1-content"

    # ---------- S1 正常更新 ----------
    Log "=== S1 正常更新: check -> download -> apply ==="
    $pkg = New-ClientEnv "1.0.0" "applied"
    # 初始 ApplicationFolder 里放 V1 主程序
    Copy-Item "$Work\fake_main.exe" "$pkg\ApplicationFolder\app.exe"

    Publish-Version "1.0.1" "main-v2-content" "config-v2"

    $cu = Invoke-Client $pkg @("check_update")
    Assert ($cu.data.has_update -eq $true) "S1 check_update: 检测到新版本"
    Assert ($cu.data.new_version -eq "1.0.1") "S1 check_update: new_version=1.0.1"

    $cd = Invoke-Client $pkg @("check_diff")
    Assert ($cd.data.files.Count -ge 1) "S1 check_diff: 有差异文件"

    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "S1 download_update: 成功"
    Assert (Test-Path "$pkg\ApplicationFolder_1.0.1\app.exe") "S1 download_update: 版本目录已创建"
    # version.json 状态应为 downloaded
    $vj = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj.version_status -eq "downloaded") "S1 download 后 version_status=downloaded"

    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "S1 apply_update: 成功"
    # 版本切换断言
    $vj2 = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj2.version -eq "1.0.1") "S1 apply 后 version=1.0.1"
    Assert ($vj2.version_status -eq "applied") "S1 apply 后 version_status=applied"
    Assert (-not (Test-Path "$pkg\ApplicationFolder_1.0.1")) "S1 apply 后版本目录已并入主目录"
    Assert (Test-Path "$pkg\ApplicationFolder_1.0.0") "S1 apply 后旧版本已备份为 ApplicationFolder_1.0.0"
    # 主程序内容已是 V2
    $mainContent = Get-Content "$pkg\ApplicationFolder\app.exe" -Raw
    Assert ($mainContent -like "*main-v2-content*") "S1 apply 后主程序内容为 V2"

    # ---------- S2 杀进程 ----------
    Log "=== S2 杀进程: explorer 子文件夹占用 ==="
    Publish-Version "1.0.2" "main-v3-content" "config-v3"
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "S2 download_update: 成功"
    # 用 explorer 打开主目录的子文件夹制造占用
    $subDir = "$pkg\ApplicationFolder_1.0.2\sub"
    if (Test-Path $subDir) { New-Item -ItemType Directory -Force -Path $subDir | Out-Null; Set-Content "$subDir\keep.txt" "x" }
    $exp = Start-Process explorer.exe -ArgumentList "`"$subDir`"" -PassThru
    Start-Sleep -Seconds 3
    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "S2 apply_update: explorer 占用下仍成功"
    $vj3 = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj3.version -eq "1.0.2") "S2 apply 后 version=1.0.2"
    # 关闭 explorer 窗口（清理）
    if ($exp -and -not $exp.HasExited) { Stop-Process -Id $exp.Id -Force -ErrorAction SilentlyContinue }

    # ---------- S2b 文件独占 ----------
    Log "=== S2b 杀进程: 目标版本目录文件被独占 ==="
    Publish-Version "1.0.3" "main-v4-content" "config-v4"
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "S2b download_update: 成功"
    # 独占打开版本目录里已下载的文件
    $lockFile = "$pkg\ApplicationFolder_1.0.3\app.exe"
    $ps = "`$s=[System.IO.File]::Open('$lockFile',[System.IO.FileMode]::Open,[System.IO.FileAccess]::ReadWrite,[System.IO.FileShare]::None); Start-Sleep -Seconds 60"
    $locker = Start-Process powershell.exe -ArgumentList "-NoProfile","-NonInteractive","-Command",$ps -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 2
    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "S2b apply_update: 文件独占下仍成功"
    $vj4 = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj4.version -eq "1.0.3") "S2b apply 后 version=1.0.3"
    if ($locker -and -not $locker.HasExited) { Stop-Process -Id $locker.Id -Force -ErrorAction SilentlyContinue }

    # ---------- S2c cmd CWD ----------
    # 注意：用 32 位 cmd（SysWOW64）——32 位客户端的 CWD 探测只能读 32 位进程的 PEB，
    # 64 位 cmd 需要深扫（全系统句柄枚举，10-60s，慢），不适合 e2e 主链路；
    # 32 位 cmd 是浅扫即可发现的真实场景，秒级完成。
    Log "=== S2c 杀进程: cmd(32位) 工作目录占用 ==="
    Publish-Version "1.0.4" "main-v5-content" "config-v5"
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "S2c download_update: 成功"
    $cwdDir = "$pkg\ApplicationFolder_1.0.4"
    $sysWOW = "$env:SystemRoot\SysWOW64\cmd.exe"
    $cmdExe = "cmd.exe"
    if (Test-Path $sysWOW) { $cmdExe = $sysWOW }
    $cmdProc = Start-Process $cmdExe -ArgumentList "/c", "ping -n 60 127.0.0.1 >nul" -WorkingDirectory $cwdDir -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 1
    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "S2c apply_update: cmd CWD 占用下仍成功"
    $vj5 = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj5.version -eq "1.0.4") "S2c apply 后 version=1.0.4"
    # 清理：apply 应已杀 cmd；若残留（ping 子进程继承 CWD），一并清理
    Get-Process cmd, PING -ErrorAction SilentlyContinue | Where-Object { $_.StartTime -gt (Get-Date).AddMinutes(-2) } | Stop-Process -Force -ErrorAction SilentlyContinue
    if ($cmdProc -and -not $cmdProc.HasExited) { Stop-Process -Id $cmdProc.Id -Force -ErrorAction SilentlyContinue }

    # ---------- S3 更新失败 ----------
    Log "=== S3 更新失败: 复制阶段被独占文件卡住 -> 回退 downloaded ==="
    Publish-Version "1.0.5" "main-v6-content" "config-v6"
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "S3 download_update: 成功"
    # 独占锁定当前主目录（ApplicationFolder）中的文件，使复制阶段失败。
    # 注意：CopyDirWithExclude 的 CopyFile(overwrite=false) 会跳过已存在的目标，
    # 因此必须先删除 versionDir 里的目标文件，复制才会真正读被锁的源文件。
    # 用 -EncodedCommand（base64）避免 -Command 引号转义破坏 File.Open 参数。
    $verDir5 = "$pkg\ApplicationFolder_1.0.5"
    if (Test-Path "$verDir5\app.exe") { Remove-Item -Force "$verDir5\app.exe" }
    $curFile = "$pkg\ApplicationFolder\app.exe"
    $psScript3 = "`$s=[System.IO.File]::Open('$curFile',[System.IO.FileMode]::Open,[System.IO.FileAccess]::ReadWrite,[System.IO.FileShare]::None); Start-Sleep -Seconds 60"
    $enc3 = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($psScript3))
    $locker3 = Start-Process powershell.exe -ArgumentList "-NoProfile","-NonInteractive","-EncodedCommand",$enc3 -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 2
    # 验证锁进程确实存活（占用生效）
    $lockAlive = $locker3 -and -not $locker3.HasExited
    if (-not $lockAlive) { Bad "S3 前置：锁进程未存活（占用未生效）" } else { Ok "S3 前置：锁进程存活" }
    $au = Invoke-Client $pkg @("apply_update")
    # 注意：当前实现不会为复制失败去杀占用者（只有 rename 失败才查杀），
    # 因此 apply 应失败并回退 downloaded、旧版仍可运行。
    $vj6 = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj6.version_status -eq "downloaded") "S3 apply 失败后 version_status=downloaded"
    Assert ($vj6.version -eq "1.0.5") "S3 apply 失败后 version 仍为待应用的 1.0.5"
    Assert (Test-Path "$pkg\ApplicationFolder\app.exe") "S3 apply 失败后旧主目录仍存在"
    if ($locker3 -and -not $locker3.HasExited) { Stop-Process -Id $locker3.Id -Force -ErrorAction SilentlyContinue }
    # S3 可能已把主目录 rename 走（若 apply 意外成功），为后续场景重建干净的 applied 环境
    $pkg = New-ClientEnv "1.0.5" "applied"
    Copy-Item "$Work\fake_main.exe" "$pkg\ApplicationFolder\app.exe"

    # ---------- S4 崩溃恢复 ----------
    Log "=== S4 崩溃恢复: applying + 主目录缺失 + 版本目录存在 ==="
    # 构造崩溃现场（真实场景）：apply 中途崩溃——MainFolder 已被改名走（或删除），
    # 但 versionDir 已就绪且含 .updator/shared.json（download 时复制了配置）。
    # 注意：不能用整个 $pkg（S3 已把其主目录 rename 走），用独立环境 $pkg4。
    Publish-Version "1.0.6" "main-v7-content" "config-v7"
    $pkg4 = New-ClientEnv "1.0.6" "applying"
    # 构造：MainFolder 缺失；versionDir 存在且含 .updator/shared.json + 主程序
    Remove-Item -Recurse -Force "$pkg4\ApplicationFolder" -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path "$pkg4\ApplicationFolder_1.0.6\.updator" | Out-Null
    Set-JsonFile "$pkg4\ApplicationFolder_1.0.6\.updator\shared.json" @{ server_url = "http://127.0.0.1:$Port"; project_name = "e2e-app"; ignore_folders = @(); ignore_files = @() }
    Copy-Item "$Work\fake_main.exe" "$pkg4\ApplicationFolder_1.0.6\app.exe"
    # 崩溃恢复：apply 应识别 applying + 主目录缺失 + 版本目录存在，直接重命名恢复
    $au = Invoke-Client $pkg4 @("apply_update")
    Assert ($au.isSuccess -eq $true) "S4 崩溃恢复 apply: 成功"
    $vj7 = Get-Content "$pkg4\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj7.version_status -eq "applied") "S4 恢复后 version_status=applied"
    Assert (Test-Path "$pkg4\ApplicationFolder\app.exe") "S4 恢复后主目录存在"

    # ---------- S5 下载后服务端又发新版 ----------
    Log "=== S5 下载后服务端又发新版 ==="
    Publish-Version "1.0.7" "main-v8-content" "config-v8"
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "S5 download_update 1.0.7: 成功"
    # 已下载 1.0.7 未 apply，服务端又发 1.0.8
    Publish-Version "1.0.8" "main-v9-content" "config-v9"
    $cu = Invoke-Client $pkg @("check_update")
    Assert ($cu.data.has_update -eq $true) "S5 check_update: 检测到服务端更新"
    Assert ($cu.data.new_version -eq "1.0.8") "S5 check_update: new_version=1.0.8"
    Assert ($cu.data.need_download_update -eq $true) "S5 check_update: 需要重新下载"
    $du = Invoke-Client $pkg @("download_update")
    Assert ($du.isSuccess -eq $true) "S5 download_update 1.0.8: 成功"
    $vj8 = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj8.version -eq "1.0.8") "S5 download 后 version=1.0.8"

    # ---------- S6 回滚 ----------
    Log "=== S6 回滚 ==="
    $au = Invoke-Client $pkg @("apply_update")
    Assert ($au.isSuccess -eq $true) "S6 apply 1.0.8: 成功"
    $lr = Invoke-Client $pkg @("list_rollback_versions")
    Assert ($lr.data.versions -contains "1.0.7") "S6 list_rollback: 含 1.0.7"
    $rb = Invoke-Client $pkg @("rollback", "--version", "1.0.7")
    Assert ($rb.isSuccess -eq $true) "S6 rollback 1.0.7: 成功"
    $vj9 = Get-Content "$pkg\UpdateFolder\version.json" -Raw | ConvertFrom-Json
    Assert ($vj9.version -eq "1.0.7") "S6 rollback 后 version=1.0.7"

    # ---------- S7 服务端缓存 ----------
    Log "=== S7 服务端 get_all_files 缓存 ==="
    # 连续两次请求，第二次应命中缓存（更快）
    $t1 = Measure-Command { $null = Api-Get "/api/file/get_all_files/e2e-app" }
    $t2 = Measure-Command { $null = Api-Get "/api/file/get_all_files/e2e-app" }
    Log "第一次(重建): $([math]::Round($t1.TotalMilliseconds))ms, 第二次(缓存): $([math]::Round($t2.TotalMilliseconds))ms"
    Assert ($t2.TotalMilliseconds -lt $t1.TotalMilliseconds) "S7 第二次 get_all_files 快于第一次（缓存命中）"
    $files = Api-Get "/api/file/get_all_files/e2e-app"
    Assert ($files.data.Count -ge 2) "S7 get_all_files 返回文件列表"
    $hasMd5 = $true
    foreach ($f in $files.data) { if (-not $f.md5 -or -not $f.sha256) { $hasMd5 = $false } }
    Assert $hasMd5 "S7 文件列表含 md5/sha256"

} finally {
    if (-not $KeepRunning) {
        Stop-Server
        if (Test-Path $Work) { Remove-Item -Recurse -Force $Work -ErrorAction SilentlyContinue }
    } else {
        Log "保留运行环境: Work=$Work (服务端仍在 :$Port)"
    }
}

Write-Host ""
Write-Host "================ 结果汇总 ================" -ForegroundColor Cyan
Write-Host "  通过: $Pass  失败: $Fail" -ForegroundColor $(if ($Fail -eq 0) { "Green" } else { "Red" })
if ($Fail -gt 0) { exit 1 } else { exit 0 }
