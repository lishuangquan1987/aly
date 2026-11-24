@echo off
rem ===========================================================================
rem  Build aly-client (32-bit, Windows XP compatible).
rem  Requires Go 1.10 and a GOPATH layout:
rem    client/aly-client/ -> %GOPATH%/src/aly/client/aly-client/
rem  Usage:
rem    1. copy client/ to %GOPATH%/src/aly/client/
rem    2. cd %GOPATH%/src/aly/client/aly-client
rem    3. run build.bat
rem
rem  KEEP THIS FILE PURE ASCII AND CRLF: UTF-8 Chinese in a .bat makes cmd.exe
rem  lose its line offset and execute comment fragments as commands.
rem ===========================================================================

rem  ---- enforce Go 1.10.x (XP cannot load anything built by Go 1.11+) ----
for /f "tokens=3" %%v in ('go version') do set GOVER=%%v
echo %GOVER% | findstr /b /c:"go1.10." >nul
if errorlevel 1 (
    echo [ERROR] Go 1.10.x is required, found %GOVER%
    echo         Windows XP support needs Go 1.10 (last release supporting XP)
    echo         Switch to Go 1.10.8 and retry; under WSL: /usr/local/go/bin/go
    exit /b 1
)

echo Building aly-client.exe (GOARCH=386, XP compatible) ...
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o aly-client.exe .
if %errorlevel% neq 0 (
    echo Build FAILED.
    exit /b %errorlevel%
)
echo Build OK: aly-client.exe (32-bit)
