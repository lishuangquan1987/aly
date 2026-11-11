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

echo ===== 铺 payload 到 publish-gui 输出目录 ====

REM 包内布局：publish-cli/<rid>/aly-publish 与 client/<rid>/aly-client(-ui)，再由 payload.json 指平台。
REM 更新器来源是本机现成产物：client\aly-client\aly-client.exe、
REM client\aly-client-ui\aly-client-winui\aly-client-ui.exe、client\aly-client-ui\dist\linux-*。
REM 与 CI 内联铺装同一套 9 个文件名与两份清单内容（CI 用内联 pwsh，本机用 copy）。

set PAYLOAD_DIR=%SCRIPT_DIR%publish-gui\src\AlyPublish\bin\Debug\net8.0

mkdyr "%PAYLOAD_DIR%\publish-cli\windows-amd64"
mkdyr "%PAYLOAD_DIR%\publish-cli\linux-amd64"
mkdyr "%PAYLOAD_DIR%\publish-cli\linux-arm64"
mkdyr "%PAYLOAD_DIR%\client\windows-x86"
mkdyr "%PAYLOAD_DIR%\client\linux-amd64"
mkdyr "%PAYLOAD_DIR%\client\linux-arm64"

copy /y "%SCRIPT_DIR%publish-cli\aly-publish.exe" "%PAYLOAD_DIR%\publish-cli\windows-amd64\aly-publish.exe" >nul
copy /y "%SCRIPT_DIR%publish-cli\aly-publish-linux-amd64" "%PAYLOAD_DIR%\publish-cli\linux-amd64\aly-publish" >nul
copy /y "%SCRIPT_DIR%publish-cli\aly-publish-linux-arm64" "%PAYLOAD_DIR%\publish-cli\linux-arm64\aly-publish" >nul

copy /y "%SCRIPT_DIR%..\client\aly-client\aly-client.exe" "%PAYLOAD_DIR%\client\windows-x86\aly-client.exe" >nul
copy /y "%SCRIPT_DIR%..\client\aly-client-ui\aly-client-winui\aly-client-ui.exe" "%PAYLOAD_DIR%\client\windows-x86\aly-client-ui.exe" >nul

copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-amd64\aly-client" "%PAYLOAD_DIR%\client\linux-amd64\aly-client" >nul
copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-amd64\aly-client-ui" "%PAYLOAD_DIR%\client\linux-amd64\aly-client-ui" >nul
copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-arm64\aly-client" "%PAYLOAD_DIR%\client\linux-arm64\aly-client" >nul
copy /y "%SCRIPT_DIR%..\client\aly-client-ui\dist\linux-arm64\aly-client-ui" "%PAYLOAD_DIR%\client\linux-arm64\aly-client-ui" >nul

copy /y "%SCRIPT_DIR%publish-gui\installer\payload.json" "%PAYLOAD_DIR%\payload.json" >nul

if %ERRORLEVEL% neq 0 (

    echo [失败] payload 铺装失败

    exit /b %ERRORLEVEL%

)

echo [成功] payload 铺装完成（10 项：9 个二进制 + payload.json）

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
