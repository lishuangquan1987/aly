@echo off
REM 构建 zap-update（32位，兼容 XP）
REM 需要 Go 1.10，且项目位于 GOPATH/src/clientupdator/client 下
REM
REM 使用方法：
REM   1. 将 client 目录复制到 %GOPATH%/src/clientupdator/client/
REM   2. 运行 build.bat

echo 正在构建 zap-update.exe (GOARCH=386, 兼容 XP) ...
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o zap-update.exe .
if %errorlevel% neq 0 (
    echo 构建失败！
    exit /b %errorlevel%
)
echo 构建成功：zap-update.exe (32位)
