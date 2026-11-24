@echo off
rem ===========================================================================
rem  KEEP THIS FILE PURE ASCII AND CRLF. Never paste non-ASCII text here.
rem
rem  cmd.exe decodes batch bytes with the OEM codepage (936/GBK on Chinese
rem  Windows). UTF-8 Chinese makes cmd lose its line offset: it executes
rem  fragments of the comment lines as commands ("... is not recognized as an
rem  internal or external command") and splits "set" lines in half, so
rem  GOARCH / GOROOT never get set and "path not found" follows.
rem  Chinese rationale: client/aly-client/BUILD.md.
rem ===========================================================================
rem  deploy-client.go prints UTF-8 Chinese; the default console codepage (936)
rem  renders it as mojibake, so switch to 65001 first.
chcp 65001 >nul

rem  Clear inherited cross-compile vars. With GOOS/GOARCH exported (e.g.
rem  linux/arm64) go produces an extension-less binary, and Windows reports
rem  "executable file not found in %PATH%".
set GOOS=
set GOARCH=
rem  Clear GOROOT: the go.exe on PATH (1.27.3) derives its own root from its
rem  executable path. A stale GOROOT from another toolchain (1.10.8) makes this
rem  one fail with per-package "-lang" errors. Do not hardcode a machine path
rem  here - it breaks on every other machine.
set GOROOT=

rem  The XP artifact is built inside this image; deploy-client.go asserts the
rem  go version in it is 1.10.x and refuses anything else.
if not defined ALY_GO_IMAGE set ALY_GO_IMAGE=golang:1.10.8-stretch

cd /d "%~dp0"
go run "%~dp0deploy-client.go"

rem  Capture the exit code before pause: pause resets errorlevel to 0, and the
rem  batch must exit with the build/verify/deploy result.
set RC=%ERRORLEVEL%
pause
exit /b %RC%
