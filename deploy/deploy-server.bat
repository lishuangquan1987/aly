@echo off
rem ---------------------------------------------------------------------------
rem Keep CRLF line endings in this file. cmd mis-splits LF-only files that
rem carry non-ASCII text and then runs the comment fragments as commands,
rem which shows up as "is not recognized as an internal ..." errors.
rem This file is deliberately ASCII-only so no decoding quirk can break it.
rem ---------------------------------------------------------------------------
rem Host build must land on the machine default target (windows/amd64).
rem If cross-compile vars leak in from an outer shell (GOOS=linux GOARCH=arm64),
rem go emits a product without .exe and Windows exec lookup reports
rem "executable file not found in %PATH%". Clear them to fall back to defaults.
rem Cross-compile targets are set explicitly inside deploy-server.go.
set GOOS=
set GOARCH=
cd /d "%~dp0"
go run "%~dp0deploy-server.go" %*
pause
