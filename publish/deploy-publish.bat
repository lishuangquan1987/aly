@echo off

chcp 65001 >nul

setlocal

set SCRIPT_DIR=%~dp0

echo ===== 编译 publish-cli（windows/amd64）====

cd /d "%SCRIPT_DIR%publish-cli"

go build -o aly-publish.exe .

if %ERRORLEVEL% neq 0 (

    echo [失败] publish-cli 编译失败

    exit /b %ERRORLEVEL%

)

echo [成功] publish-cli 编译完成

echo ===== 交叉编译 publish-cli（linux/amd64、linux/arm64）====

set CGO_ENABLED=0

set GOOS=linux

set GOARCH=amd64

go build -o aly-publish-linux-amd64 .

if %ERRORLEVEL% neq 0 ( echo [失败] linux/amd64 编译失败 & exit /b %ERRORLEVEL% )

set GOARCH=arm64

go build -o aly-publish-linux-arm64 .

if %ERRORLEVEL% neq 0 ( echo [失败] linux/arm64 编译失败 & exit /b %ERRORLEVEL% )

echo [成功] 三个平台的发布 CLI 就绪

echo ===== 铺 payload 到 publish-gui 输出目录（含 payload.json）====

REM 本机产物来源：publish-cli 的三个 CLI + client/aly-client 与 client/aly-client-ui 的
REM 现成 exe + client/aly-client-ui/dist/linux-* 的 Linux 更新器。
REM 与 CI 同一个脚本，包内布局与收尾断言完全一致（缺文件即失败）。

powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%publish-gui\installer\stage-payload.ps1" -FromLocal -App "%SCRIPT_DIR%publish-gui\src\AlyPublish\bin\Debug\net8.0"

if %ERRORLEVEL% neq 0 (

    echo [失败] payload 铺装失败

    exit /b %ERRORLEVEL%

)

echo ===== 编译 publish-gui =====

cd /d "%SCRIPT_DIR%publish-gui\src\AlyPublish"

dotnet build -c Debug --no-restore

if %ERRORLEVEL% neq 0 (

    echo [失败] publish-gui 编译失败

    exit /b %ERRORLEVEL%

)

echo [成功] publish-gui 编译完成

echo ===== 全部完成 ====

echo   GUI:      见 publish-gui\src\AlyPublish\bin\Debug\net8.0\AlyPublish.exe
echo   CLI:      同目录 publish-cli\ 下按平台分目录
echo   更新器:   同目录 client\ 下按平台分目录
echo   清单:     同目录 payload.json

endlocal
