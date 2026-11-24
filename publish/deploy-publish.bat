@echo off
chcp 65001 >nul
setlocal
set SCRIPT_DIR=%~dp0
rem Clear GOROOT: the go.exe on PATH (1.27.3) derives its own root from its
rem executable path. A stale root from another toolchain (1.10.8) makes this
rem one fail with per-package "-lang" errors. Never hardcode a machine path.
set GOROOT=
rem GOPATH must hold the source trees the import paths resolve to
rem (<GOPATH>\src\aly\publish-cli for aly/publish-cli, ...).
if not defined GOPATH set GOPATH=%USERPROFILE%\go
rem publish-cli is a go.mod module (cobra/pflag): modules must stay enabled.
rem The client build is the one that needs the GOPATH mode (GO111MODULE=off).
set GO111MODULE=
echo ===== Building publish-cli =====
cd /d "%SCRIPT_DIR%publish-cli"
go build -o aly-publish.exe .
if %ERRORLEVEL% neq 0 (
    echo [FAIL] publish-cli build failed
    exit /b %ERRORLEVEL%
)
echo [OK] publish-cli built
echo ===== Copy aly-publish.exe to publish-gui-go output dir =====
if not exist "%SCRIPT_DIR%publish-gui-go\build\windows-amd64\" mkdir "%SCRIPT_DIR%publish-gui-go\build\windows-amd64\"
copy /y "%SCRIPT_DIR%publish-cli\aly-publish.exe" "%SCRIPT_DIR%publish-gui-go\build\windows-amd64\" >nul
if %ERRORLEVEL% neq 0 (
    echo [FAIL] copy aly-publish.exe failed
    exit /b %ERRORLEVEL%
)
echo [OK] copied to publish-gui-go output dir
echo ===== Copy aly-client.exe / aly-client-ui.exe to publish-gui-go output dir =====
REM Both exe files are built by deploy/deploy-client.bat (WSL + Go 1.10) and copied to the repo root:
REM client\aly-client\ and client\aly-client-ui\aly-client-winui\ - run deploy-client.bat first. When publish-gui-go initializes
REM a project, FindClientExecutable prefers the copy next to the GUI, so a copy in the program dir is used.
if not exist "%SCRIPT_DIR%..\client\aly-client\aly-client.exe" (
    echo [WARN] client\aly-client\aly-client.exe not found in repo root; run deploy/deploy-client.bat first
    goto copy_client_done
)
copy /y "%SCRIPT_DIR%..\client\aly-client\aly-client.exe" "%SCRIPT_DIR%publish-gui-go\build\windows-amd64\" >nul
if %ERRORLEVEL% neq 0 (
    echo [FAIL] copy aly-client.exe failed
    exit /b %ERRORLEVEL%
)
echo [OK] copied aly-client.exe
if not exist "%SCRIPT_DIR%..\client\aly-client-ui\aly-client-winui\aly-client-ui.exe" (
    echo [WARN] client\aly-client-ui\aly-client-winui\aly-client-ui.exe not found; run deploy/deploy-client.bat first
    goto copy_client_done
)
copy /y "%SCRIPT_DIR%..\client\aly-client-ui\aly-client-winui\aly-client-ui.exe" "%SCRIPT_DIR%publish-gui-go\build\windows-amd64\" >nul
if %ERRORLEVEL% neq 0 (
    echo [FAIL] copy aly-client-ui.exe failed
    exit /b %ERRORLEVEL%
)
echo [OK] copied aly-client-ui.exe
:copy_client_done
echo ===== Building publish-gui-go =====
cd /d "%SCRIPT_DIR%publish-gui-go"
go build -trimpath -ldflags "-H windowsgui" -o build\windows-amd64\AlyPublish.exe .
if %ERRORLEVEL% neq 0 (
    echo [FAIL] publish-gui-go build failed
    exit /b %ERRORLEVEL%
)
echo [OK] publish-gui-go built
echo ===== All done =====
endlocal
