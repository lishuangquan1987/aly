@echo off
REM 仅用于本地编译校验；正式构建/部署请用仓库根的 deploy-client.go（WSL + Go 1.10.8）
REM 用法：先 cd 到 %GOPATH%/src/aly/client/aly-client，再运行本脚本
setlocal

REM ---- 强制 Go 1.10.x（XP 兼容；Go 1.11+ 产出的二进制 XP 无法加载）----
for /f "tokens=3" %%v in ('go version') do set GOVER=%%v
echo %GOVER% | findstr /b /c:"go1.10." >nul
if errorlevel 1 (
    echo [ERROR] 必须使用 Go 1.10.x，当前为 %GOVER%
    echo         XP 兼容要求 Go 1.10（Go 1.10 是最后一个支持 XP 的版本）
    echo         请切换到 Go 1.10.8 后重试；WSL 下为 /usr/local/go/bin/go
    exit /b 1
)

set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
set GO111MODULE=off
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client/ui/winui
if %errorlevel% neq 0 (
    echo 构建失败！
    exit /b %errorlevel%
)
echo 构建成功：aly-client-ui.exe (32位)
