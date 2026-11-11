@echo off

chcp 65001 >nul

rem 本文件必须保持 CRLF 行尾：cmd 对 LF 行尾 + 中文注释会误拆行，把注释碎片
rem 当命令执行（报 "is not recognized as an internal ..."）。实测同一份内容：
rem LF 版 1 处误拆，CRLF 版零报错。
rem
rem 三个部署 bat 只做本机构建，不交叉编译：
rem   - publish-cli 只编 windows/amd64；GOOS/GOARCH/CGO_ENABLED 显式钉住
rem     （与 CI 的 build env 一致），机器环境里残留的 GOOS=linux/GOARCH=arm64
rem     不再影响本次构建；
rem   - linux 的发布 CLI 走 CI 产物 publish-gui\artifacts\staging，本机不再
rem     `set GOOS=linux` + go build（那两个产物以前其实是 windows 构建）；
rem   - linux 的客户端走本机现成产物 client\aly-client-ui\dist\linux-*；
rem   - windows 的客户端走 deploy-client.bat 回拷的仓库产物。
rem
rem payload 落点 = GUI 程序目录（AlyPublish.exe 旁，Debug 输出 bin\Debug\net8.0）：
rem 与运行时 PkHelper 的探测一致 —— payload.json、publish-cli/{rid}/aly-publish(.exe)、
rem client/{rid}/aly-client(-ui)(.exe)。rid 与 installer\payload.json 的约定一致：
rem publish-cli = windows-amd64 / linux-amd64 / linux-arm64，
rem client       = windows-x86 / linux-amd64 / linux-arm64。

set GOOS=windows

set GOARCH=amd64

set CGO_ENABLED=0

rem MISS：payload 有缺项；GUIFAIL：publish-gui 没重编译成（常见原因：GUI 正在运行）。
rem 两个标志都只影响末态提示；除 windows 项缺或复制失败外都不中断。

set MISS=

set GUIFAIL=

setlocal

set SCRIPT_DIR=%~dp0

echo ===== [1/3] 编译 publish-cli（windows/amd64，本机）====

cd /d "%SCRIPT_DIR%publish-cli"

go build -ldflags="-s -w" -o aly-publish.exe .

if %ERRORLEVEL% neq 0 (

    echo [失败] publish-cli 编译失败

    exit /b %ERRORLEVEL%

)

echo [成功] publish-cli 编译完成（windows/amd64）

echo ===== [2/3] 编译 publish-gui（Debug）====

cd /d "%SCRIPT_DIR%publish-gui\src\AlyPublish"

dotnet build -c Debug --no-restore

if %ERRORLEVEL% neq 0 (

    echo [警告] publish-gui 编译未完成（常见原因：AlyPublish 正在运行，exe 被占用）

    echo         payload 仍会铺到程序目录，用的是现有 exe；要重编译请关掉 GUI 后重跑

    set GUIFAIL=1

)

echo [完成] publish-gui 步骤结束

echo ===== [3/3] 铺 payload 到 GUI 程序目录 ====

rem 与 CI 的 staging 步同一套 10 项。缺 windows 项或一般复制失败立即失败；
rem 缺 linux 发布 CLI 只警告并记 MISS（linux 由 CI 产出，本机不交叉编译）。

set PAYLOAD_DIR=%SCRIPT_DIR%publish-gui\src\AlyPublish\bin\Debug\net8.0

set STAGING_DIR=%SCRIPT_DIR%publish-gui\artifacts\staging

mkdyr "%PAYLOAD_DIR%\publish-cli\windows-amd64"

mkdyr "%PAYLOAD_DIR%\publish-cli\linux-amd64"

mkdyr "%PAYLOAD_DIR%\publish-cli\linux-arm64"

mkdyr "%PAYLOAD_DIR%\client\windows-x86"

mkdyr "%PAYLOAD_DIR%\client\linux-amd64"

mkdyr "%PAYLOAD_DIR%\client\linux-arm64"

rem ── 发布 CLI ──

copy /y "%SCRIPT_DIR%publish-cli\aly-publish.exe" "%PAYLOAD_DIR%\publish-cli\windows-amd64\aly-publish.exe" >nul

if exist "%STAGING_DIR%\aly-publish-linux-amd64" copy /y "%STAGING_DIR%\aly-publish-linux-amd64" "%PAYLOAD_DIR%\publish-cli\linux-amd64\aly-publish" >nul

if not exist "%STAGING_DIR%\aly-publish-linux-amd64" echo [警告] 缺 linux/amd64 发布 CLI：%STAGING_DIR%\aly-publish-linux-amd64

if not exist "%STAGING_DIR%\aly-publish-linux-amd64" set MISS=1

if exist "%STAGING_DIR%\aly-publish-linux-arm64" copy /y "%STAGING_DIR%\aly-publish-linux-arm64" "%PAYLOAD_DIR%\publish-cli\linux-arm64\aly-publish" >nul

if not exist "%STAGING_DIR%\aly-publish-linux-arm64" echo [警告] 缺 linux/arm64 发布 CLI：%STAGING_DIR%\aly-publish-linux-arm64

if not exist "%STAGING_DIR%\aly-publish-linux-arm64" set MISS=1

rem 自检提示：staging 里的两个 linux 发布 CLI 必须是 ELF 头（7F 45 4C 46）。
rem 若它们的头是 MZ、且字节数与 windows 那份相同，那就是 windows 构建混进来了，
rem 需从 CI 重取后再重跑（本机不交叉编译，铺进去是什么就是什么）。

rem ── 客户端 ──

copy /y "%SCRIPT_DIR%..\client\aly-client\aly-client.exe" "%PAYLOAD_DIR%\client\windows-x86\aly-client.exe" >nul

copy /y "%SCRIPT_DIR%..\client\aly-client-ui\aly-client-winui\aly-client-ui.exe" "%PAYLOAD_DIR%\client\windows-x86\aly-client-ui.exe" >nul

copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-amd64\aly-client" "%PAYLOAD_DIR%\client\linux-amd64\aly-client" >nul

copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-amd64\aly-client-ui" "%PAYLOAD_DIR%\client\linux-amd64\aly-client-ui" >nul

copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-arm64\aly-client" "%PAYLOAD_DIR%\client\linux-arm64\aly-client" >nul

copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-arm64\aly-client-ui" "%PAYLOAD_DIR%\client\linux-arm64\aly-client-ui" >nul

rem ── 清单 ──

copy /y "%SCRIPT_DIR%publish-gui\installer\payload.json" "%PAYLOAD_DIR%\payload.json" >nul

if %ERRORLEVEL% neq 0 (

    echo [失败] payload 铺装失败（源文件缺失、不可写，或缺 windows 项）

    exit /b %ERRORLEVEL%

)

echo ---- payload 落点自检（10 项）----

if exist "%PAYLOAD_DIR%\publish-cli\windows-amd64\aly-publish.exe" echo   OK publish-cli\windows-amd64\aly-publish.exe

if not exist "%PAYLOAD_DIR%\publish-cli\windows-amd64\aly-publish.exe" set MISS=1

if not exist "%PAYLOAD_DIR%\publish-cli\windows-amd64\aly-publish.exe" echo   缺 publish-cli\windows-amd64\aly-publish.exe

if exist "%PAYLOAD_DIR%\publish-cli\linux-amd64\aly-publish" echo   OK publish-cli\linux-amd64\aly-publish

if not exist "%PAYLOAD_DIR%\publish-cli\linux-amd64\aly-publish" set MISS=1

if not exist "%PAYLOAD_DIR%\publish-cli\linux-amd64\aly-publish" echo   缺 publish-cli\linux-amd64\aly-publish

if exist "%PAYLOAD_DIR%\publish-cli\linux-arm64\aly-publish" echo   OK publish-cli\linux-arm64\aly-publish

if not exist "%PAYLOAD_DIR%\publish-cli\linux-arm64\aly-publish" set MISS=1

if not exist "%PAYLOAD_DIR%\publish-cli\linux-arm64\aly-publish" echo   缺 publish-cli\linux-arm64\aly-publish

if exist "%PAYLOAD_DIR%\client\windows-x86\aly-client.exe" echo   OK client\windows-x86\aly-client.exe

if not exist "%PAYLOAD_DIR%\client\windows-x86\aly-client.exe" set MISS=1

if not exist "%PAYLOAD_DIR%\client\windows-x86\aly-client.exe" echo   缺 client\windows-x86\aly-client.exe

if exist "%PAYLOAD_DIR%\client\windows-x86\aly-client-ui.exe" echo   OK client\windows-x86\aly-client-ui.exe

if not exist "%PAYLOAD_DIR%\client\windows-x86\aly-client-ui.exe" set MISS=1

if not exist "%PAYLOAD_DIR%\client\windows-x86\aly-client-ui.exe" echo   缺 client\windows-x86\aly-client-ui.exe

if exist "%PAYLOAD_DIR%\client\linux-amd64\aly-client" echo   OK client\linux-amd64\aly-client

if not exist "%PAYLOAD_DIR%\client\linux-amd64\aly-client" set MISS=1

if not exist "%PAYLOAD_DIR%\client\linux-amd64\aly-client" echo   缺 client\linux-amd64\aly-client

if exist "%PAYLOAD_DIR%\client\linux-amd64\aly-client-ui" echo   OK client\linux-amd64\aly-client-ui

if not exist "%PAYLOAD_DIR%\client\linux-amd64\aly-client-ui" set MISS=1

if not exist "%PAYLOAD_DIR%\client\linux-amd64\aly-client-ui" echo   缺 client\linux-amd64\aly-client-ui

if exist "%PAYLOAD_DIR%\client\linux-arm64\aly-client" echo   OK client\linux-arm64\aly-client

if not exist "%PAYLOAD_DIR%\client\linux-arm64\aly-client" set MISS=1

if not exist "%PAYLOAD_DIR%\client\linux-arm64\aly-client" echo   缺 client\linux-arm64\aly-client

if exist "%PAYLOAD_DIR%\client\linux-arm64\aly-client-ui" echo   OK client\linux-arm64\aly-client-ui

if not exist "%PAYLOAD_DIR%\client\linux-arm64\aly-client-ui" set MISS=1

if not exist "%PAYLOAD_DIR%\client\linux-arm64\aly-client-ui" echo   缺 client\linux-arm64\aly-client-ui

if exist "%PAYLOAD_DIR%\payload.json" echo   OK payload.json

if not exist "%PAYLOAD_DIR%\payload.json" set MISS=1

if not exist "%PAYLOAD_DIR%\payload.json" echo   缺 payload.json

if defined MISS echo [警告] payload 不完整（见上面缺失项）：windows 平台可用；linux 发布 CLI 需从 CI 取到 %STAGING_DIR% 后重跑本脚本。

if defined GUIFAIL echo [警告] publish-gui 未重编译（exe 被占用或编译报错），payload 里的 AlyPublish.exe 是现有那份。

echo [完成] payload 落点：%PAYLOAD_DIR%（10 项自检见上面 OK/缺 行）

echo ===== 全部完成 ====

echo   GUI:      见 %PAYLOAD_DIR%\AlyPublish.exe

echo   CLI:      同目录 publish-cli\ 下按平台分目录（windows 本机编译，linux 走 CI 产物）

echo   更新器:   同目录 client\ 下按平台分目录

echo   清单:     同目录 payload.json

endlocal
