@echo off
REM 构建 aly-client（32位，兼容 XP）
REM 需要 Go 1.10，项目必须位于 GOPATH 下：
REM   client/aly-client/ → %GOPATH%/src/aly/client/aly-client/
REM
REM 使用方法：
REM   1. 将 client/ 目录复制到 %GOPATH%/src/aly/client/
REM   2. cd %GOPATH%/src/aly/client/aly-client
REM   3. 运行 build.bat

echo 正在构建 aly-client.exe (GOARCH=386, 兼容 XP) ...
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o aly-client.exe .
if %errorlevel% neq 0 (
    echo 构建失败！
    exit /b %errorlevel%
)
echo 构建成功：aly-client.exe (32位)
