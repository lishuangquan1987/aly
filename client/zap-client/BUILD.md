# zap-update 编译说明

## 环境要求

- Go 1.10（必须，项目中使用了 Go 1.10 特性）
- 项目位于 GOPATH 下：
  - `client/zap-client/`     → `%GOPATH%/src/zap/client/zap-client/`
  - `client/zap-client-sdk/` → `%GOPATH%/src/zap/client/zap-client-sdk/`

## 完整编译流程

### 1. 删除 GOPATH 下的旧源码

```batch
rmdir /s /q %GOPATH%\src\zap\client\zap-client
rmdir /s /q %GOPATH%\src\zap\client\zap-client-sdk
```

### 2. 将工作目录的源码复制到 GOPATH

```batch
xcopy /e /i /y client\zap-client %GOPATH%\src\zap\client\zap-client
xcopy /e /i /y client\zap-client-sdk %GOPATH%\src\zap\client\zap-client-sdk
```

> 注意：当前命令在项目根目录（`Zap/`）下执行。

### 3. 编译（32 位，兼容 Windows XP）

```batch
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o zap-update.exe zap/client/zap-client
```

> **注意**：必须使用 import path `zap/zap-client`，不能使用绝对路径（如 `C:\xxx`），否则 Go 1.10 在 GOPATH 模式下会找不到包。

编译参数说明：

| 参数 | 说明 |
|------|------|
| `GOOS=windows` | 目标操作系统 Windows |
| `GOARCH=386` | 32 位架构，兼容 Windows XP |
| `GO111MODULE=off` | 关闭 Go Modules，使用 GOPATH 模式 |
| `-ldflags="-s -w"` | 去除调试符号和 DWARF 表，减小体积 |

### 4. 将 exe 复制回工作目录

```batch
copy /y zap-update.exe client\zap-client\zap-update.exe
del zap-update.exe
```

## 一键编译脚本

`zap-client/build.bat`：

```batch
@echo off
REM 构建 zap-update（32位，兼容 XP）
REM 需要 Go 1.10，项目必须位于 GOPATH 下：
REM   zap-client/     → %GOPATH%/src/zap/client/zap-client/
REM   zap-client-sdk/ → %GOPATH%/src/zap/client/zap-client-sdk/

echo 正在清理 GOPATH 旧源码...
rmdir /s /q %GOPATH%\src\zap\client\zap-client
rmdir /s /q %GOPATH%\src\zap\client\zap-client-sdk

echo 正在复制源码到 GOPATH...
xcopy /e /i /y client\zap-client %GOPATH%\src\zap\client\zap-client >nul
xcopy /e /i /y client\zap-client-sdk %GOPATH%\src\zap\client\zap-client-sdk >nul

echo 正在编译 zap-update.exe (GOARCH=386) ...
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o zap-update.exe zap/client/zap-client
if %errorlevel% neq 0 (
    echo 编译失败！
    exit /b %errorlevel%
)

echo 正在复制 exe 到 zap-client\ 目录...
copy /y zap-update.exe client\zap-client\zap-update.exe >nul
del zap-update.exe

echo 构建成功：client\zap-client\zap-update.exe
```

## 常见问题

### Q: 为什么不能用 `go build` 在项目根目录直接编译？

因为源码中的 import 路径是 `zap/zap-client/...` 和 `zap/zap-client-sdk/...`，需要在 GOPATH 模式下通过 `$GOPATH/src/zap/` 解析。直接在项目根目录用相对路径编译会找不到这些内部包。

### Q: 为什么 exe 时间戳没有更新？

检查是否用了绝对路径作为 package 参数（如 `C:\Users\xxx`）。Go 1.10 GOPATH 模式下，绝对路径会被拼接到 `$GOROOT/src/` 和 `$GOPATH/src/` 后面，导致找不到包或编译的是旧文件。应始终使用 import path `zap/client/zap-client`。

### Q: 编译出的 exe 体积多大？

约 4.7 MB（32 位，已 strip 调试符号）。如果使用 `-ldflags="-s -w"` 可以去掉调试信息，否则约 6.1 MB。
