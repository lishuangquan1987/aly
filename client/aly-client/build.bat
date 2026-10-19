@echo off
REM 构建 aly-client（32位，兼容 XP）
REM 需要 Go 1.10，项目必须位于 GOPATH 下：
REM   client/aly-client/ → %GOPATH%/src/aly/client/aly-client/
REM
REM 使用方法：
REM   1. 将 client/ 目录复制到 %GOPATH%/src/aly/client/
REM   2. cd %GOPATH%/src/aly/client/aly-client
REM   3. 运行 build.bat

REM ---- 强制 Go 1.10.x（XP 兼容；Go 1.11+ 产出的二进制 XP 无法加载）----
for /f "tokens=3" %%v in ('go version') do set GOVER=%%v
echo %GOVER% | findstr /b /c:"go1.10." >nul
if errorlevel 1 (
    echo [ERROR] 必须使用 Go 1.10.x，当前为 %GOVER%
    echo         XP 兼容要求 Go 1.10（Go 1.10 是最后一个支持 XP 的版本）
    echo         请切换到 Go 1.10.8 后重试；WSL 下为 /usr/local/go/bin/go
    exit /b 1
)

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
