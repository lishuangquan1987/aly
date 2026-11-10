@echo off
cd /d "%~dp0"
go run "%~dp0deploy-server.go" %*
pause
