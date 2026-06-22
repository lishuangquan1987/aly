@echo off
REM 构建 zap-update（32位，兼容 XP）
REM 需要 Go 1.10，项目必须位于 GOPATH 下：
REM   zap-client/     → %GOPATH%/src/zap/zap-client/
REM   zap-client-sdk/ → %GOPATH%/src/zap/zap-client-sdk/
REM
REM 使用方法：
REM   1. 将 zap-client/ 和 zap-client-sdk/ 复制到 %GOPATH%/src/zap/
REM   2. cd %GOPATH%/src/zap/zap-client
REM   3. 运行 build.bat

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
