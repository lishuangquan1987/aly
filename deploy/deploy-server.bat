@echo off
rem ===========================================================================
rem  KEEP THIS FILE PURE ASCII AND CRLF. Never paste non-ASCII text here.
rem  cmd.exe decodes batch bytes with the OEM codepage (936/GBK on Chinese
rem  Windows); UTF-8 Chinese makes it lose its line offset and run fragments of
rem  comments as commands, while splitting "set" lines in half.
rem  (This file used to have a doubled CR on every line - CRCRLF - which left a
rem  trailing \r inside every set value, e.g. GOROOT, hence "path not found".)
rem ===========================================================================
chcp 65001 >nul

rem  Clear inherited cross-compile vars: with GOOS/GOARCH exported go produces
rem  an extension-less binary and Windows reports
rem  "executable file not found in %PATH%".
set GOOS=
set GOARCH=
rem  Clear GOROOT so the go.exe on PATH (1.27.3) derives its own root; leaving a
rem  1.10.8 root behind makes its own compile report per-package "-lang" errors.
set GOROOT=
rem  server is a go.mod module (gin/ent/...): modules must stay enabled so deps
rem  are fetched (clearing GO111MODULE leaves it at the default "on" for
rem  modules); GOPATH only holds the module cache and the source tree.
if not defined GOPATH set GOPATH=%USERPROFILE%\go
set GO111MODULE=

cd /d "%~dp0"
go run "%~dp0deploy-server.go" %*

rem  Capture the exit code before pause: pause resets errorlevel to 0.
set RC=%ERRORLEVEL%
pause
exit /b %RC%
