# aly-client 编译说明

本目录产出**两个**可执行文件：

| 产物 | 源码位置 | 说明 |
|------|----------|------|
| `aly-client.exe` | `client/aly-client/` | 更新器本体（控制台程序） |
| `aly-client-ui.exe` | `client/aly-client/ui/winui/` | 可选图形前端（GUI，右下角 topmost，见 `ui/winui/DEVELOPMENT_PLAN.md`） |

## 环境要求

- **Go 1.10.8**（必须；Go 1.11+ 产出的二进制 Windows XP **无法加载**，
  因为 PE `MajorSubsystemVersion` 会变成 5.1 以上。`build.bat` 里有版本断言）
- 项目位于 GOPATH 下：
  - `client/aly-client/`     → `%GOPATH%/src/aly/client/aly-client/`
  - `client/aly-client-sdk/` → `%GOPATH%/src/aly/client/aly-client-sdk/`

## 完整编译流程

### 1. 删除 GOPATH 下的旧源码

```batch
rmdir /s /q %GOPATH%\src\aly\client\aly-client
```

### 2. 将工作目录的源码复制到 GOPATH

```batch
xcopy /e /i /y client\aly-client %GOPATH%\src\aly\client\aly-client
xcopy /e /i /y client\aly-client-sdk %GOPATH%\src\aly\client\aly-client-sdk
```

> 注意：当前命令在项目根目录（`Aly/`）下执行。

### 3. 编译（32 位，兼容 Windows XP）

**更新器（控制台）：**

```batch
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
set CGO_ENABLED=0
go build -ldflags="-s -w" -o aly-client.exe aly/client/aly-client
```

**图形前端（GUI，无控制台窗口）：**

```batch
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client/ui/winui
```

> **注意**：必须使用 import path `aly/client/aly-client`，不能使用绝对路径（如 `C:\xxx`），否则 Go 1.10 在 GOPATH 模式下会找不到包。
>
> **`-H=windowsgui` 不能省**：否则会附带一个黑色控制台窗口。
>
> **不要删除 `ui/winui/rsrc.syso`**：它嵌入了 ComCtl32 v6 manifest。
> `go build` 会自动链接**同包目录**下的 `*.syso`；缺失时 UI 启动即失败
> （`create window failed: TTM_ADDTOOL failed`，进程 exit 1）。

编译参数说明：

| 参数 | 说明 |
|------|------|
| `GOOS=windows` | 目标操作系统 Windows |
| `GOARCH=386` | 32 位架构，兼容 Windows XP |
| `GO111MODULE=off` | 关闭 Go Modules，使用 GOPATH 模式 |
| `CGO_ENABLED=0` | 纯 Go，避免外部 C 依赖（交叉编译默认即如此） |
| `-ldflags="-s -w"` | 去除调试符号和 DWARF 表，减小体积 |
| `-ldflags="-H=windowsgui"` | 仅 UI 使用：生成 GUI 子系统程序，不带控制台窗口 |

### 4. 将 exe 复制回工作目录

更新器与图形前端都是随包交付物，**两者回拷方式一致**（放在同一目录，供部署与
UI 自更新的"主程序目录 → UpdateFolder"比对使用）：

```batch
copy /y aly-client.exe    client\aly-client\aly-client.exe
copy /y aly-client-ui.exe client\aly-client\aly-client-ui.exe
del aly-client.exe
del aly-client-ui.exe
```

> 用 `deploy-client.bat` 时会自动完成这两步回拷；手工构建时才需要执行上面的命令。
> 不再把 UI 放到 `ui\winui\` 下（该目录的 exe 已被 `.gitignore` 忽略）。

## 一键编译脚本

推荐直接用仓库根的 `deploy-client.go`（WSL + Go 1.10.8），它会一次构建两个 exe
并部署到 `config.json` 里配置的所有目标目录：

```batch
deploy-client.bat
```

或分别使用下面的两个脚本。

### `build.bat` —— 更新器

`aly-client/build.bat`（已内置 Go 1.10.x 版本断言，非 1.10 直接拒绝并 exit 1）：

```batch
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
```

> 完整内容以仓库里的 `build.bat` 为准（上面为摘要）。

### `ui/winui/build-ui.bat` —— 图形前端

同样是 Go 1.10.x 断言 + `-H=windowsgui`，构建 `aly-client-ui.exe`。
详细用法见脚本头部注释。

## 常见问题

### Q: 为什么不能用 `go build` 在项目根目录直接编译？

因为源码中的 import 路径是 `aly/client/aly-client/...` 和 `aly/client/aly-client-sdk/...`，需要在 GOPATH 模式下通过 `$GOPATH/src/aly/` 解析。直接在项目根目录用相对路径编译会找不到这些内部包。

### Q: 为什么 exe 时间戳没有更新？

检查是否用了绝对路径作为 package 参数（如 `C:\Users\xxx`）。Go 1.10 GOPATH 模式下，绝对路径会被拼接到 `$GOROOT/src/` 和 `$GOPATH/src/` 后面，导致找不到包或编译的是旧文件。应始终使用 import path `aly/client/aly-client`。

### Q: `aly-client-ui.exe` 启动后立刻退出，退出码 1？

说明创建窗口失败。最常见原因是 **manifest 没被链接**：确认
`client/aly-client/ui/winui/rsrc.syso` 存在，且是从**包目录**构建
（`go build aly/client/aly-client/ui/winui`）。用控制台方式构建（去掉
`-H=windowsgui`）可以直接看到错误信息：

```batch
go build -ldflags="-s -w" -o ui-console.exe aly/client/aly-client/ui/winui
ui-console.exe -mode check
:: 正常应能看到 create window failed: TTM_ADDTOOL failed 之类的具体原因
```

### Q: 怎么验证产物真的能跑在 XP 上？

两个硬指标（`build.bat` / CI 都会检查）：

1. PE 头 `Machine=0x014C`（32 位）且 `MajorSubsystemVersion <= 5`。
   Go 1.11+ 构建出来是 5.1/6.1，XP 加载器直接拒绝。
2. `aly-client-ui.exe` 内含 `Microsoft.Windows.Common-Controls` 字符串
   （ComCtl32 v6 manifest）。

```
python deploy-client.go  之外的检查可参考 .github/workflows/release.yml
里的 "Verify XP compatibility" 步骤。
```

### Q: 编译出的 exe 体积多大？

- `aly-client.exe`：约 3.8 MB（32 位，已 strip 调试符号）。
- `aly-client-ui.exe`：约 4.6 MB（含 walk / lxn/win 及其依赖，已 strip）。

若不用 `-ldflags="-s -w"`，体积会明显增大（含调试信息）。
