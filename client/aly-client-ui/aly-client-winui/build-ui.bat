@echo off
rem ===========================================================================
rem  Local build check only. For the real build/deploy use
rem  deploy/deploy-client.go (WSL + Go 1.10.8).
rem  Usage: cd %GOPATH%/src/aly/client/aly-client-ui/aly-client-winui, then run
rem  this script.
rem
rem  KEEP THIS FILE PURE ASCII AND CRLF: UTF-8 Chinese in a .bat makes cmd.exe
rem  lose its line offset and execute comment fragments as commands.
rem ===========================================================================
setlocal

rem  ---- enforce Go 1.10.x (XP cannot load anything built by Go 1.11+) ----
for /f "tokens=3" %%v in ('go version') do set GOVER=%%v
echo %GOVER% | findstr /b /c:"go1.10." >nul
if errorlevel 1 (
    echo [ERROR] Go 1.10.x is required, found %GOVER%
    echo         Windows XP support needs Go 1.10 (last release supporting XP)
    echo         Switch to Go 1.10.8 and retry; under WSL: /usr/local/go/bin/go
    exit /b 1
)

set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
set GO111MODULE=off
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client-ui/aly-client-winui
if %errorlevel% neq 0 (
    echo Build FAILED.
    exit /b %errorlevel%
)
echo Build OK: aly-client-ui.exe (32-bit)
