@echo off
rem ---------------------------------------------------------------------------
rem Keep CRLF line endings: cmd mis-splits LF-only files that carry non-ASCII
rem text and then runs the comment fragments as commands. This file is
rem deliberately ASCII-only; only real cmd built-ins are used (mkdir, copy).
rem ---------------------------------------------------------------------------
rem Dev/local deploy for the publish toolchain:
rem   1) build aly-publish for windows/amd64 and linux/amd64 + linux/arm64
rem   2) stage the package payload layout into the GUI output dir
rem      (9 binaries + payload.json), same file names as the CI does
rem   3) build the GUI (AlyPublish.csproj)
rem Package layout written here:
rem   <gui-out>\payload.json
rem   <gui-out>\publish-cli\<rid>\aly-publish(.exe)
rem   <gui-out>\client\<rid>\aly-client(-ui)(.exe)
chcp 65001 >nul
setlocal
set SCRIPT_DIR=%~dp0
echo ==== build publish-cli (windows/amd64) ====
cd /d "%SCRIPT_DIR%publish-cli"
go build -o aly-publish.exe .
if %ERRORLEVEL% neq 0 (
    echo [fail] publish-cli build failed
    exit /b %ERRORLEVEL%
)
echo [ok] publish-cli built
echo ==== cross build publish-cli (linux/amd64, linux/arm64) ====
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -o aly-publish-linux-amd64 .
if %ERRORLEVEL% neq 0 ( echo [fail] linux/amd64 build failed & exit /b %ERRORLEVEL% )
set GOARCH=arm64
go build -o aly-publish-linux-arm64 .
if %ERRORLEVEL% neq 0 ( echo [fail] linux/arm64 build failed & exit /b %ERRORLEVEL% )
echo [ok] three publish CLIs built
echo ==== stage the payload into the GUI output dir ====
set PAYLOAD_DIR=%SCRIPT_DIR%publish-gui\src\AlyPublish\bin\Debug\net8.0
mkdir "%PAYLOAD_DIR%\publish-cli\windows-amd64"
mkdir "%PAYLOAD_DIR%\publish-cli\linux-amd64"
mkdir "%PAYLOAD_DIR%\publish-cli\linux-arm64"
mkdir "%PAYLOAD_DIR%\client\windows-x86"
mkdir "%PAYLOAD_DIR%\client\linux-amd64"
mkdir "%PAYLOAD_DIR%\client\linux-arm64"
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
if %ERRORLEVEL% neq 0 ( echo [fail] payload staging failed & exit /b %ERRORLEVEL% )
echo [ok] payload staged (9 binaries + payload.json)
echo ==== build publish-gui ====
cd /d "%SCRIPT_DIR%publish-gui\src\AlyPublish"
dotnet build -c Debug --no-restore
if %ERRORLEVEL% neq 0 ( echo [fail] publish-gui build failed & exit /b %ERRORLEVEL% )
echo [ok] publish-gui built
echo ==== done ====
endlocal
