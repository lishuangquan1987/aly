@echo off
REM 构建 zap-client（32位，兼容 XP）
REM 需要 Go 1.10，项目必须位于 GOPATH 下：
REM   client/zap-client/ → %GOPATH%/src/zap/client/zap-client/
REM
REM 使用方法：
REM   1. 将 client/ 目录复制到 %GOPATH%/src/zap/client/
REM   2. cd %GOPATH%/src/zap/client/zap-client
REM   3. 运行 build.bat

echo 正在构建 zap-client.exe (GOARCH=386, 兼容 XP) ...
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o zap-client.exe .
if %errorlevel% neq 0 (
    echo 构建失败！
    exit /b %errorlevel%
)
echo 构建成功：zap-client.exe (32位)
