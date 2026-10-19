# aly-client-ui 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 `subagent-driven-development`（推荐）或 `executing-plans` 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 构建 `aly-client-ui.exe` —— 一个 Go 1.10.8 + walk 编写的 32 位 Windows XP 兼容 GUI，封装 `aly-client.exe` 的更新命令，在屏幕右下角 topmost 按需显示更新界面。

**架构：** 单向分层 `main → app → flow → bridge → aly-client.exe`；`view` 只依赖 `flow` 暴露的接口。`bridge` 以 `os/exec` 调用 `aly-client.exe` 并逐行解析 stdout JSON；`flow` 实现对齐 C# SDK 的状态机；`view` 用 walk 自绘扁平卡片。所有可测逻辑（定位计算、JSON 解析、状态机、i18n、配置读取）都抽成纯函数，与 walk 窗口解耦。

**技术栈：** Go 1.10.8（GOPATH 模式，`GOOS=windows`/`GOARCH=386`/`CGO_ENABLED=0`）、`github.com/lxn/walk`、`github.com/lxn/win`、`golang.org/x/sys`、`gopkg.in/Knetic/govaluate.v3`、ComCtl32 v6 manifest、`akavel/rsrc`。

**需求来源：** 同目录 [`DEVELOPMENT_PLAN.md`](DEVELOPMENT_PLAN.md)（v2 评审修订版，已评审通过）。

---

## 0. 编码红线（Go 1.10 兼容，**每个任务都必须遵守**）

> 违反任何一条都会在 Go 1.10.8 下编译失败。**这就是 v1 计划踩坑的同一类错误。**

| 禁止 | 替代 |
|------|------|
| `strings.ReplaceAll`（1.12） | `strings.Replace(s, old, new, -1)` |
| `errors.Is` / `errors.As`（1.13） | `os.IsNotExist(err)` 或类型断言 `err.(*os.PathError)` |
| `fmt.Errorf("...: %w", err)`（1.13） | `fmt.Errorf("...: %v", err)` |
| `os.ReadFile` / `os.WriteFile`（1.16） | `ioutil.ReadFile` / `ioutil.WriteFile` |
| `io.ReadAll`（1.16） | `ioutil.ReadAll` |
| `strings.Builder` 的 `WriteString` 返回 `(int, error)` | 直接用 `+=` 或 `bytes.Buffer`（1.10 两者都可用） |
| 泛型、`any`、`min`/`max` 内置 | 具体类型 + 手写辅助函数 |
| `//go:build` 指令（1.17） | `// +build windows` |
| `os.UserHomeDir`（1.12） | 读 `%USERPROFILE%` 环境变量 |
| `filepath.IsLocal`、`os.MkdirTemp`（1.16） | `ioutil.TempDir` |
| `t.Cleanup`（1.14，**测试代码也要遵守**） | `defer` 或显式清理（既有测试已违反此条，故测试只能用现代 Go 跑，见 §2） |
| `exec.ExitError.ExitCode()`（1.12） | `ee.Sys().(syscall.WaitStatus).ExitStatus()` |
| `os.ProcessState.ExitCode()`（1.12） | 同上，从 `WaitStatus` 取 |

**统一约定**：所有文件顶部（除 `_test.go`）加 `// +build windows`；`package` 声明前留一空行。

---

## 1. 文件结构

| 文件 | 职责 |
|------|------|
| `client/aly-client/ui/winui/main.go` | win32gui 入口；解析 `-mode`/`-lang`/`-url`/`-project-name`；装配并启动 |
| `client/aly-client/ui/winui/app/app.go` | 生命周期、单实例互斥体、日志 |
| `client/aly-client/ui/winui/app/config.go` | 只读 `UpdateFolder/client.json` 的 `main_exe_relative_path` 单字段 |
| `client/aly-client/ui/winui/app/config_test.go` | 上述的单元测试 |
| `client/aly-client/ui/winui/bridge/client.go` | 进程执行、逐行 JSON 解析、超时、退出码/EOF 契约 |
| `client/aly-client/ui/winui/bridge/commands.go` | 六个命令的薄封装 |
| `client/aly-client/ui/winui/bridge/parse.go` | 纯函数：一行文本 → `Output` / `Progress`（可测） |
| `client/aly-client/ui/winui/bridge/parse_test.go` | 解析契约的表格测试 |
| `client/aly-client/ui/winui/flow/events.go` | 状态枚举、事件回调接口（`Sink`） |
| `client/aly-client/ui/winui/flow/updater.go` | 状态机：check→download→apply、`force_update` 缺失规则 |
| `client/aly-client/ui/winui/flow/updater_test.go` | 用 fake bridge 测状态迁移 |
| `client/aly-client/ui/winui/view/geometry.go` | 纯函数：工作区 + 尺寸 + 边距 → 右下角坐标（可测） |
| `client/aly-client/ui/winui/view/geometry_test.go` | 定位计算测试 |
| `client/aly-client/ui/winui/view/window.go` | 主窗：创建/定位/topmost/按需显示/5s 自动关闭 |
| `client/aly-client/ui/winui/view/card.go` | 自绘卡片 `CustomWidget` |
| `client/aly-client/ui/winui/view/progress.go` | 自绘进度条 `CustomWidget` |
| `client/aly-client/ui/winui/view/theme/theme.go` | 配色、字体、尺寸常量 |
| `client/aly-client/ui/winui/i18n/i18n.go` | 语言枚举、`T(key)`、键注册表 |
| `client/aly-client/ui/winui/i18n/zh_CN.go` | 中文字串表 |
| `client/aly-client/ui/winui/i18n/en_US.go` | 英文字串表 |
| `client/aly-client/ui/winui/i18n/i18n_test.go` | 中英键集一致性测试 |
| `client/aly-client/ui/winui/aly-client-ui.exe.manifest` | ComCtl32 v6 + `dpiAware=true` |
| `client/aly-client/ui/winui/rsrc.syso` | `rsrc` 生成的嵌入资源（构建产物，**提交**） |
| `client/aly-client/ui/winui/vendor/` | 4 个钉选依赖 |
| `client/aly-client/ui/winui/build-ui.bat` | 本地校验脚本（强制 Go 1.10 断言） |
| `client/aly-client/build.bat` | 修改：加 Go 版本断言（任务 0） |
| `deploy-client.go` | 修改：新增构建/部署 `aly-client-ui.exe` |

---

## 2. 测试策略

> ⚠️ **执行期修正（2026-10-19，实测发现）**
>
> 1. **测试必须用现代 Go 跑，不能用 Go 1.10.8。**
>    既有测试文件使用 `t.Cleanup`（Go **1.14** 才引入），例如 `cmd/apply_update_test.go:26`、`config/version_test.go:19`。
>    用 Go 1.10.8 跑 `go test ./...` 会直接 `[build failed]`：
>    `t.Cleanup undefined (type *testing.T has no field or method Cleanup)`。
>    → **生产代码用 Go 1.10.8 编译；测试用本机 go1.26 跑**（Go 向后兼容，能在 1.10 下编译的代码必然能在 1.26 下编译）。
> 2. **不能用 `GOOS=windows GOARCH=386 go test`。** 交叉编译出的测试二进制在 Linux/异架构下无法执行。
>    测试用**宿主 GOOS/GOARCH（windows/amd64）** 原生运行。
> 3. **既有基线有 2 个失败**（与 UI 改动无关，Go 1.10 与 1.26 下同样失败，均为 CWD 进程探测的环境问题）：
>    - `cmd`：`TestRenameDirWithKillCmdCwdOccupiesFolder`
>    - `util`：`TestFindProcessesWithCWDUnder`
>    → 判定「本次改动是否引入回归」时，以「除这 2 个外无新增失败」为准。
> 4. **新增测试代码避免 `t.Cleanup`**（用 `defer` 或显式清理），使新包在 Go 1.10 下也能编译，便于将来在 XP 侧验证。

- **可单测**（纯函数，无 walk 窗口）：`bridge/parse.go`、`flow/updater.go`、`view/geometry.go`、`i18n`、`app/config.go`。
- **不可单测**（需真实窗口/消息循环）：`view/window.go`、`view/card.go`、`view/progress.go` → 用**手工验收清单**（任务 2、11、12、16）。
- **单元测试运行方式**（GOPATH 内，宿主 windows/amd64 原生跑）：
  ```powershell
  $gp = "$env:TEMP\acgopath"
  New-Item -ItemType Directory -Force -Path "$gp\src\aly\client" | Out-Null
  Copy-Item -Recurse -Force "E:\Project2026\aly\client\aly-client" "$gp\src\aly\client\aly-client"
  $env:GOPATH = $gp; $env:GO111MODULE = "off"; $env:GOOS = "windows"; $env:GOARCH = "amd64"
  cd "$gp\src\aly\client\aly-client"
  go test ./ui/winui/...          # 只跑 UI 包
  go test ./...                   # 全量（基线：cmd/util 各 1 个既有失败）
  ```
- **发布构建**始终 Go 1.10.8 + 386（见任务 0、1、5）。

---

## 任务 0：修复前置问题（Go 1.10.8 重建 `aly-client.exe` + `build.bat` 版本断言）

**背景：** 当前提交的 `client/aly-client/aly-client.exe` 是 Go 1.26 构建（PE `SubsystemVer=6.1`），**XP 无法加载**。

**文件：**
- 修改：`client/aly-client/build.bat`
- 重新生成：`client/aly-client/aly-client.exe`
- 测试：`client/aly-client/cmd/apply_update_test.go`（既有测试，用于回归）

- [ ] **步骤 1：确认当前产物确实不兼容 XP**

运行：
```powershell
go version client/aly-client/aly-client.exe
```
预期：输出 `go1.26.0`（证明是 Go 1.26 构建）。

- [ ] **步骤 2：给 `build.bat` 加 Go 版本断言**

修改 `client/aly-client/build.bat`，在 `set GOOS=windows` **之前**插入：

```bat
REM ---- 强制 Go 1.10.x（XP 兼容；Go 1.11+ 产出的二进制 XP 无法加载）----
for /f "tokens=3" %%v in ('go version') do set GOVER=%%v
echo %GOVER% | findstr /b "go1.10." >nul
if errorlevel 1 (
    echo [ERROR] 必须使用 Go 1.10.x，当前为 %GOVER%
    echo         XP 兼容要求 Go 1.10（Go 1.10 是最后一个支持 XP 的版本）
    echo         本机可用：C:\Users\5001494\sdk\go1.10.8\bin\go.exe
    exit /b 1
)
```

- [ ] **步骤 3：验证断言生效**

运行：
```powershell
cd client\aly-client; .\build.bat
```
预期：**FAIL**，输出 `[ERROR] 必须使用 Go 1.10.x，当前为 go1.26.0`（因为 PATH 上是 go1.26）。

- [ ] **步骤 4：用 WSL Go 1.10.8 重新构建**

运行：
```powershell
go run deploy-client.go
```
（`deploy-client.go` 走 WSL `/usr/local/go` = go1.10.8，会把 `aly-client.exe` 回拷并部署到 `UpdateFolder` 与主程序目录。）

预期：输出 `[2/4] Building aly-client.exe (GOARCH=386) ...` 与 `Build OK`。

- [ ] **步骤 5：校验产物是 XP 可加载的 32 位 GUI**

运行：
```powershell
# 用 PE 头读取（Go 1.10 二进制没有 buildinfo，go version 读不出，属正常）
python -c "import struct,sys; p=r'client\aly-client\aly-client.exe'; f=open(p,'rb'); f.seek(0x3C); o=struct.unpack('<I',f.read(4))[0]; f.seek(o); m=struct.unpack('<H',f.read(2))[0]; f.seek(o+24+40); os_=struct.unpack('<HH',f.read(4)); f.seek(o+24+48); sub=struct.unpack('<HH',f.read(4)); print('Machine=0x%04X MajorSubsystem=%d.%d' % (m, sub[0], sub[1]))"
```
预期：`Machine=0x014C MajorSubsystem=4.0`（≤5.1 即 XP 可加载）。

- [ ] **步骤 6：跑既有单元测试回归**

运行：
```powershell
wsl bash -c "cd /tmp && rm -rf alytest && mkdir -p alytest/src/aly/client && cp -r /mnt/e/Project2026/aly/client/aly-client alytest/src/aly/client/aly-client && cd alytest/src/aly/client/aly-client && GOPATH=/tmp/alytest GOROOT=/usr/local/go GOOS=windows GOARCH=386 /usr/local/go/bin/go test ./... 2>&1 | tail -20"
```
预期：全部 `ok`（或 `no test files`）。

- [ ] **步骤 7：Commit**

```bash
git add client/aly-client/build.bat client/aly-client/aly-client.exe
git commit -m "fix(client): 用 Go 1.10.8 重建 aly-client.exe 并给 build.bat 加版本断言

当前提交的 aly-client.exe 是 Go 1.26 构建（PE SubsystemVer=6.1），
Windows XP 加载器会直接拒绝，违反 AGENTS.md 规则 6。
改用项目既有 WSL Go 1.10.8 路径重建（PE SubsystemVer=4.0），
并在 build.bat 中断言 go1.10.x，避免再次静默产出非 XP 二进制。"
```

---

## 任务 1：搭建 winui 骨架 + vendor 4 依赖（M0 第 1 步）

**文件：**
- 创建：`client/aly-client/ui/winui/main.go`
- 创建：`client/aly-client/ui/winui/vendor/`（4 个包）
- 创建：`client/aly-client/ui/winui/build-ui.bat`
- 测试：编译本身即验证

- [ ] **步骤 1：克隆并钉选 4 个依赖到 vendor**

运行（在仓库根目录，Git Bash 或 WSL）：
```bash
V=client/aly-client/ui/winui/vendor
mkdir -p $V/github.com/lxn $V/golang.org/x $V/gopkg.in/Knetic

git clone -q https://github.com/lxn/walk $V/github.com/lxn/walk
git -C $V/github.com/lxn/walk checkout -q bf6b8a4b207d83866fe5cef6ad09cd4fccc6d65c

git clone -q https://github.com/lxn/win $V/github.com/lxn/win
git -C $V/github.com/lxn/win checkout -q d1d36f0e4f482d3e75b5145232a2d5f50d76bac4

git clone -q https://github.com/golang/sys $V/golang.org/x/sys
git -C $V/golang.org/x/sys checkout -q 19e00faab6ad547203cd66b19e1281807329cf65

git clone -q https://github.com/Knetic/govaluate $V/gopkg.in/Knetic/govaluate.v3
git -C $V/gopkg.in/Knetic/govaluate.v3 checkout -q v3.0.0

# 去掉 .git 以免污染仓库
rm -rf $V/github.com/lxn/walk/.git $V/github.com/lxn/win/.git \
       $V/golang.org/x/sys/.git $V/gopkg.in/Knetic/govaluate.v3/.git
```

预期：4 个目录存在，`walk/messagebox.go` 中**不含** `strings.ReplaceAll`。

验证：
```bash
grep -c "strings.ReplaceAll" $V/github.com/lxn/walk/messagebox.go || echo "0（正确）"
```
预期：`0（正确）`（若为 2，说明 checkout 失败、拿到了新版本）。

- [ ] **步骤 2：写最小 `main.go`（仅编译验证，无窗口）**

创建 `client/aly-client/ui/winui/main.go`：

```go
// +build windows

// aly-client-ui: aly-client 的图形前端（右下角 topmost 常驻/按需显示）。
package main

import (
	"fmt"
	"os"

	"github.com/lxn/walk"
)

func main() {
	// 临时：仅验证 walk 能在 Go 1.10.8 + 386 下编译链接
	if _, err := walk.NewMainWindow(); err != nil {
		fmt.Fprintf(os.Stderr, "walk init failed: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **步骤 3：写 `build-ui.bat`（含 Go 版本断言）**

创建 `client/aly-client/ui/winui/build-ui.bat`：

```bat
@echo off
REM 仅用于本地编译校验；正式构建/部署请用仓库根的 deploy-client.go（WSL + Go 1.10.8）
setlocal

for /f "tokens=3" %%v in ('go version') do set GOVER=%%v
echo %GOVER% | findstr /b "go1.10." >nul
if errorlevel 1 (
    echo [ERROR] 必须使用 Go 1.10.x，当前为 %GOVER%
    echo         XP 兼容要求 Go 1.10。本机可用：C:\Users\5001494\sdk\go1.10.8\bin\go.exe
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
echo 构建成功：aly-client-ui.exe
```

- [ ] **步骤 4：在 GOPATH 内用 Go 1.10.8 编译**

运行（WSL）：
```bash
wsl bash -c "set -e; rm -rf /tmp/uibuild; mkdir -p /tmp/uibuild/src/aly/client; cp -r /mnt/e/Project2026/aly/client/aly-client /tmp/uibuild/src/aly/client/aly-client; cd /tmp/uibuild/src/aly/client/aly-client; GOPATH=/tmp/uibuild GOROOT=/usr/local/go GOOS=windows GOARCH=386 CGO_ENABLED=0 /usr/local/go/bin/go build -ldflags='-s -w -H=windowsgui' -o /tmp/uibuild/aly-client-ui.exe aly/client/aly-client/ui/winui && ls -l /tmp/uibuild/aly-client-ui.exe"
```
预期：编译成功，列出 `aly-client-ui.exe`（约 7–8 MB）。

若报 `cannot find package "gopkg.in/Knetic/govaluate.v3"` → vendor 路径不对，回步骤 1 检查目录名（**必须**是 `govaluate.v3`，不是 `govaluate`）。

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/main.go \
        client/aly-client/ui/winui/build-ui.bat \
        client/aly-client/ui/winui/vendor
git commit -m "feat(ui): winui 骨架与 4 个钉选依赖（Go 1.10.8 + 386 编译通过）

walk@bf6b8a4b（9958a48b 会因 strings.ReplaceAll 在 Go 1.10 下编译失败）
win@d1d36f0、x/sys@19e00faa、govaluate.v3@v3.0.0
build-ui.bat 断言 go1.10.x，避免静默产出非 XP 二进制。"
```

---

## 任务 2：右下角定位纯函数（可单测）

**文件：**
- 创建：`client/aly-client/ui/winui/view/geometry.go`
- 测试：`client/aly-client/ui/winui/view/geometry_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `client/aly-client/ui/winui/view/geometry_test.go`：

```go
// +build windows

package view

import "testing"

func TestCalcBottomRight(t *testing.T) {
	// 工作区 1920x1080（任务栏已扣除），窗口 380x160，边距 16
	wa := Rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	x, y := CalcBottomRight(wa, Size{W: 380, H: 160}, 16)
	if x != 1524 || y != 904 {
		t.Fatalf("want (1524,904), got (%d,%d)", x, y)
	}
}

func TestCalcBottomRightNonZeroOrigin(t *testing.T) {
	// 副屏工作区原点不在 (0,0)
	wa := Rect{Left: 1920, Top: -200, Right: 3840, Bottom: 880}
	x, y := CalcBottomRight(wa, Size{W: 380, H: 160}, 16)
	if x != 3444 || y != 704 {
		t.Fatalf("want (3444,704), got (%d,%d)", x, y)
	}
}

func TestCalcBottomRightClampsSmallWorkArea(t *testing.T) {
	// 工作区比窗口还小：夹到左上角，不能出负坐标
	wa := Rect{Left: 10, Top: 20, Right: 210, Bottom: 120}
	x, y := CalcBottomRight(wa, Size{W: 380, H: 160}, 16)
	if x != 10 || y != 20 {
		t.Fatalf("want clamp to (10,20), got (%d,%d)", x, y)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：
```bash
wsl bash -c "cd /tmp/uibuild/src/aly/client/aly-client 2>/dev/null || (mkdir -p /tmp/uibuild/src/aly/client && cp -r /mnt/e/Project2026/aly/client/aly-client /tmp/uibuild/src/aly/client/aly-client && cd /tmp/uibuild/src/aly/client/aly-client); GOPATH=/tmp/uibuild GOROOT=/usr/local/go GOOS=windows GO111MODULE=off /usr/local/go/bin/go test ./ui/winui/view/ 2>&1 | head"
```
预期：FAIL，`undefined: Rect` / `undefined: CalcBottomRight`。

- [ ] **步骤 3：编写最少实现**

创建 `client/aly-client/ui/winui/view/geometry.go`：

```go
// +build windows

package view

// Rect 是工作区矩形（像素）。
type Rect struct {
	Left, Top, Right, Bottom int
}

// Size 是窗口尺寸（像素）。
type Size struct {
	W, H int
}

// CalcBottomRight 计算窗口贴工作区右下角的左上角坐标。
// 工作区应取 MONITORINFO.RcWork（已扣除任务栏）。
// 若工作区比窗口还小，则夹到工作区左上角，避免出现负坐标。
func CalcBottomRight(wa Rect, win Size, margin int) (int, int) {
	x := wa.Right - win.W - margin
	y := wa.Bottom - win.H - margin
	if x < wa.Left {
		x = wa.Left
	}
	if y < wa.Top {
		y = wa.Top
	}
	return x, y
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：同步骤 2。
预期：`ok  aly/client/aly-client/ui/winui/view`。

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/view/geometry.go client/aly-client/ui/winui/view/geometry_test.go
git commit -m "feat(ui): 右下角定位纯函数与单元测试"
```

---

## 任务 3：最小窗口（右下角 + topmost + 不进任务栏 + 按需显示）

> ⚠️ **执行期修正（2026-10-19，实测发现）：任务 4（manifest）必须先于本任务的运行验证。**
> 没有嵌入 ComCtl32 v6 manifest 时，`walk.NewMainWindow()` 会在创建 ToolTip 时报
> `create window failed: TTM_ADDTOOL failed`，进程立即以 exit 1 退出。
> 因此：**先做任务 4 生成 `rsrc.syso`，再回来验证本任务的窗口**。
> 提交顺序同样建议 manifest 在前（保证每个提交都可运行）。

**文件：**
- 创建：`client/aly-client/ui/winui/view/window.go`
- 修改：`client/aly-client/ui/winui/main.go`

- [ ] **步骤 1：实现窗口**

创建 `client/aly-client/ui/winui/view/window.go`：

```go
// +build windows

package view

import (
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

const (
	defaultW = 380
	defaultH = 160
	margin   = 16
)

// Window 是右下角常驻/按需显示的顶层窗口。
type Window struct {
	mw *walk.MainWindow
}

// New 创建窗口，定位到主显示器工作区右下角，设为 topmost + 工具窗口，
// 但**默认隐藏**（按需显示，见 DEVELOPMENT_PLAN §4.1）。
func New() (*Window, error) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return nil, err
	}
	w := &Window{mw: mw}

	if err := mw.SetTitle("aly-client-ui"); err != nil {
		return nil, err
	}

	// 工具窗口：不进任务栏、不进 Alt-Tab
	ex := win.GetWindowLong(mw.Handle(), win.GWL_EXSTYLE)
	win.SetWindowLong(mw.Handle(), win.GWL_EXSTYLE, ex|win.WS_EX_TOOLWINDOW)

	w.positionBottomRight()
	w.setTopMost()
	return w, nil
}

// positionBottomRight 取窗口所在显示器的工作区并贴右下角。
func (w *Window) positionBottomRight() {
	hmon := win.MonitorFromWindow(w.mw.Handle(), win.MONITOR_DEFAULTTONEAREST)
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !win.GetMonitorInfo(hmon, &mi) {
		return
	}
	wa := Rect{
		Left:   int(mi.RcWork.Left),
		Top:    int(mi.RcWork.Top),
		Right:  int(mi.RcWork.Right),
		Bottom: int(mi.RcWork.Bottom),
	}
	x, y := CalcBottomRight(wa, Size{W: defaultW, H: defaultH}, margin)
	// 先定位再显示，避免闪现到屏幕左上角
	_ = w.mw.SetBounds(walk.Rectangle{X: x, Y: y, Width: defaultW, Height: defaultH})
}

// setTopMost 置顶且不抢焦点（SWP_NOACTIVATE 很关键：不要打断全屏主程序）。
func (w *Window) setTopMost() {
	win.SetWindowPos(w.mw.Handle(), win.HWND_TOPMOST, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
}

// Show 重新定位并显示窗口。
func (w *Window) Show() {
	w.positionBottomRight()
	w.setTopMost()
	w.mw.SetVisible(true)
}

// Hide 隐藏窗口（不退出进程）。
func (w *Window) Hide() {
	w.mw.SetVisible(false)
}

// Run 进入消息循环。
func (w *Window) Run() int {
	return w.mw.Run()
}

// Synchronize 在 UI 线程上执行 fn（供后台 goroutine 调用）。
func (w *Window) Synchronize(fn func()) {
	w.mw.Synchronize(fn)
}
```

- [ ] **步骤 2：改 `main.go` 打开窗口**

替换 `client/aly-client/ui/winui/main.go` 为：

```go
// +build windows

// aly-client-ui: aly-client 的图形前端（右下角 topmost 按需显示）。
package main

import (
	"fmt"
	"os"

	"aly/client/aly-client/ui/winui/view"
)

func main() {
	w, err := view.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create window failed: %v\n", err)
		os.Exit(1)
	}
	// 临时：M0 阶段直接显示，验证右下角 + topmost + 不进任务栏
	w.Show()
	os.Exit(w.Run())
}
```

- [ ] **步骤 3：编译**

运行：同任务 1 步骤 4。
预期：编译成功。

- [ ] **步骤 4：XP 手工验收**

在 **XP SP3 x86 虚拟机**中运行 `aly-client-ui.exe`，逐项确认：

| 检查 | 期望 |
|------|------|
| 窗口位置 | 屏幕**右下角**，距边缘约 16px |
| 是否遮挡任务栏 | **不遮挡**（用了 `RcWork`） |
| 置顶 | 被其他窗口覆盖后重新聚焦其他程序，本窗口仍在最上层 |
| 任务栏 | **无**任务栏按钮 |
| Alt-Tab | **不出现**在 Alt-Tab 列表 |
| 焦点 | 启动时**不抢**当前焦点 |
| 崩溃 | 不崩溃、无错误弹窗 |

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/view/window.go client/aly-client/ui/winui/main.go
git commit -m "feat(ui): 右下角 topmost 工具窗口（默认隐藏，按需显示）"
```

---

## 任务 4：ComCtl32 v6 manifest 嵌入

**文件：**
- 创建：`client/aly-client/ui/winui/aly-client-ui.exe.manifest`
- 生成：`client/aly-client/ui/winui/rsrc.syso`

- [ ] **步骤 1：写 manifest（XP 安全子集）**

创建 `client/aly-client/ui/winui/aly-client-ui.exe.manifest`：

```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity
      version="1.0.0.0"
      processorArchitecture="*"
      name="Aly.Client.UI"
      type="win32"/>
  <description>aly-client-ui</description>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity
          type="win32"
          name="Microsoft.Windows.Common-Controls"
          version="6.0.0.0"
          processorArchitecture="*"
          publicKeyToken="6595b64144ccf1df"
          language="*"/>
    </dependentAssembly>
  </dependency>
  <!-- 只声明 dpiAware。不要写 dpiAwareness/PerMonitorV2：
       那是 Win10 1703+ schema，XP 的 SxS 解析器可能拒绝整个 manifest。 -->
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true</dpiAware>
    </windowsSettings>
  </application>
</assembly>
```

- [ ] **步骤 2：用 Go 1.10 构建 rsrc 工具**

运行（WSL）：
```bash
wsl bash -c "set -e; rm -rf /tmp/rsrcbuild; mkdir -p /tmp/rsrcbuild/src/github.com/akavel; cd /tmp/rsrcbuild/src/github.com/akavel; git clone -q https://github.com/akavel/rsrc; cd rsrc; git log --before=2019-01-01 -1 --format='%H %ad' --date=short; GOPATH=/tmp/rsrcbuild GOROOT=/usr/local/go GO111MODULE=off /usr/local/go/bin/go build -o /tmp/rsrcbuild/rsrc . && echo RSRC_BUILD_OK"
```
预期：打印一个 2018 年的提交，并输出 `RSRC_BUILD_OK`。
若编译失败（用到新语法）→ 用 `git log --before=2018-01-01` 再退一版；仍失败则改用手工构造 `.syso`（见任务 4 备注）。

- [ ] **步骤 3：生成 `rsrc.syso`**

运行（WSL）：
```bash
wsl bash -c "/tmp/rsrcbuild/rsrc -manifest /mnt/e/Project2026/aly/client/aly-client/ui/winui/aly-client-ui.exe.manifest -arch 386 -o /mnt/e/Project2026/aly/client/aly-client/ui/winui/rsrc.syso && ls -l /mnt/e/Project2026/aly/client/aly-client/ui/winui/rsrc.syso"
```
预期：生成 `rsrc.syso`（约 1–2 KB）。

- [ ] **步骤 4：编译并验证 manifest 已嵌入**

运行：任务 1 步骤 4 的编译命令。
预期：编译成功（`rsrc.syso` 会被 Go 自动链接）。

验证嵌入：
```powershell
Select-String -Path client\aly-client\ui\winui\aly-client-ui.exe -Pattern "Common-Controls" -Encoding Byte -ErrorAction SilentlyContinue
# 或用 python 搜字节串
python -c "d=open(r'client\aly-client\ui\winui\aly-client-ui.exe','rb').read(); print('EMBEDDED' if b'Microsoft.Windows.Common-Controls' in d else 'MISSING')"
```
预期：`EMBEDDED`。

- [ ] **步骤 5：XP 手工验收**

在 XP 虚拟机中运行，确认：
| 检查 | 期望 |
|------|------|
| 控件外观 | 有 XP 主题（Luna），**不是** Win95 灰 |
| manifest 是否被拒 | 无「应用程序配置不正确」SxS 错误弹窗 |
| 启动 | 正常启动 |

- [ ] **步骤 6：Commit**

```bash
git add client/aly-client/ui/winui/aly-client-ui.exe.manifest client/aly-client/ui/winui/rsrc.syso
git commit -m "feat(ui): 嵌入 ComCtl32 v6 manifest（XP 安全子集，只声明 dpiAware）"
```

> **备注（rsrc 不可用时的退路）**：手工构造 `.syso`——用 Python 生成一个含 `RT_MANIFEST`(24) 资源的 COFF 对象，或改用 MinGW 的 `windres`：`windres -i manifest.rc -o rsrc.syso --output-format=coff -F pe-i386`。

---

## 任务 5：扩展 `deploy-client.go` 构建/部署 UI

**文件：**
- 修改：`deploy-client.go`

- [ ] **步骤 1：在 Step 2 后新增 UI 构建**

在 `deploy-client.go` 的 `main()` 中，`Build OK` 打印之后、`Step 3` 之前插入：

```go
	// ── Step 2b: Build UI (XP 32-bit) ──
	fmt.Println("  [2b/4] Building aly-client-ui.exe (GOARCH=386) ...")
	buildUICmd := fmt.Sprintf(
		"cd %s && GOROOT=%s GOPATH=%s GOOS=windows GOARCH=386 CGO_ENABLED=0 %s build -ldflags=\"-s -w -H=windowsgui\" -o aly-client-ui.exe aly/client/aly-client/ui/winui",
		wslSrc, wslGoroot, wslGopath, wslGo,
	)
	if err := wsl(buildUICmd); err != nil {
		fmt.Printf("  [ERROR] UI build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Build OK\n")
```

- [ ] **步骤 2：在 Step 3 部署中新增 UI 拷贝**

在部署循环里，`cp -f .../aly-client.exe` 之后追加：

```go
			// UI 只部署到 UpdateFolder；主程序目录放一份作为自更新比对源
			if strings.HasSuffix(strings.ReplaceAll(t.win, "/", "\\"), "UpdateFolder") ||
				strings.HasSuffix(strings.ReplaceAll(t.win, "/", "\\"), "UpdateFolder\\") {
				if err := wsl("cp -f " + wslSrc + "/aly-client-ui.exe " + t.wsl + "/"); err != nil {
					fmt.Printf("         [FAIL] copy ui failed: %v\n", err)
					fail = true
					continue
				}
			}
```

> 注：`deploy-client.go` 是仓库根目录的独立 `package main`，用宿主 Go 编译，**不受** Go 1.10 限制，可用 `strings.ReplaceAll`。

- [ ] **步骤 3：验证部署路径**

运行：
```powershell
go run deploy-client.go
```
预期：输出 `[2b/4] Building aly-client-ui.exe` 与 `Build OK`；结束时列出各 target 的 `aly-client.exe` 与 `aly-client-ui.exe` 路径。

- [ ] **步骤 4：确认文件落位**

运行：
```powershell
Get-ChildItem -Recurse -Filter "aly-client-ui.exe" <你的 ProjectPath 与 UpdateFolder 的父目录> | Select-Object FullName
```
预期：`UpdateFolder\aly-client-ui.exe` 与 `win-x64\aly-client-ui.exe` 各一份。

- [ ] **步骤 5：Commit**

```bash
git add deploy-client.go
git commit -m "feat(deploy): 同步构建并部署 aly-client-ui.exe"
```

---

## 任务 6：i18n 骨架 + 键集一致性测试

**文件：**
- 创建：`client/aly-client/ui/winui/i18n/i18n.go`
- 创建：`client/aly-client/ui/winui/i18n/zh_CN.go`
- 创建：`client/aly-client/ui/winui/i18n/en_US.go`
- 测试：`client/aly-client/ui/winui/i18n/i18n_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `client/aly-client/ui/winui/i18n/i18n_test.go`：

```go
// +build windows

package i18n

import "testing"

func TestLangDefaultIsChinese(t *testing.T) {
	if got := ParseLang(""); got != ZH {
		t.Fatalf("empty lang should default to ZH, got %v", got)
	}
	if got := ParseLang("bogus"); got != ZH {
		t.Fatalf("unknown lang should default to ZH, got %v", got)
	}
	if got := ParseLang("en-US"); got != EN {
		t.Fatalf("en-US should map to EN, got %v", got)
	}
}

func TestKeySetsMatch(t *testing.T) {
	for k := range zhCN {
		if _, ok := enUS[k]; !ok {
			t.Errorf("key %q missing in en-US", k)
		}
	}
	for k := range enUS {
		if _, ok := zhCN[k]; !ok {
			t.Errorf("key %q missing in zh-CN", k)
		}
	}
}

func TestTAndFormat(t *testing.T) {
	SetLang(ZH)
	if got := T(KeyNoUpdate); got == "" {
		t.Fatal("KeyNoUpdate should not be empty")
	}
	SetLang(EN)
	if got := T(KeyNoUpdate); got == "" {
		t.Fatal("KeyNoUpdate should not be empty in EN")
	}
	// 带占位符的键必须能被 Sprintf 正确填充
	if got := Fmt(KeyFoundVersion, "1.2.3"); got == "" || got == T(KeyFoundVersion) {
		t.Fatalf("Fmt should substitute the placeholder, got %q", got)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./ui/winui/i18n/`（在 GOPATH 内，见 §2）。
预期：FAIL，`undefined: ParseLang`。

- [ ] **步骤 3：编写实现**

创建 `client/aly-client/ui/winui/i18n/i18n.go`：

```go
// +build windows

// Package i18n 提供中英文案。语言由主程序经 -lang 传入，UI 不按系统区域自动切换。
package i18n

import (
	"fmt"
	"strings"
)

// Lang 是界面语言。
type Lang int

const (
	ZH Lang = iota
	EN
)

// 字串键（新增键时必须同时加到 zh_CN.go 与 en_US.go）。
const (
	KeyChecking        = "checking"
	KeyNoUpdate        = "no_update"
	KeyFoundVersion    = "found_version"
	KeyDownloading     = "downloading"
	KeyDownloaded      = "downloaded"
	KeyApplying        = "applying"
	KeyApplyDone       = "apply_done"
	KeyCheckNow        = "check_now"
	KeyDownload        = "download"
	KeyCancel          = "cancel"
	KeyApply           = "apply"
	KeyLater           = "later"
	KeyRetry           = "retry"
	KeyClose           = "close"
	KeyCurrentVersion  = "current_version"
	KeyAskDownload     = "ask_download"
	KeyAskApply        = "ask_apply"
	KeyOffline         = "offline"
	KeyErrorGeneric    = "error_generic"
	KeyDetail          = "detail"
	KeyUpdateAvailable = "update_available"
	KeyRollback        = "rollback"
	KeyVersionList     = "version_list"
)

var current = ZH

// ParseLang 解析 -lang 参数；空串或未知值一律回退中文（不按系统区域推断）。
func ParseLang(s string) Lang {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "en-us", "en_us", "en":
		return EN
	default:
		return ZH
	}
}

// SetLang 设置当前语言。
func SetLang(l Lang) {
	current = l
}

// Current 返回当前语言。
func Current() Lang {
	return current
}

// T 取当前语言的文案；键不存在时返回键名本身（便于发现漏配）。
func T(key string) string {
	table := zhCN
	if current == EN {
		table = enUS
	}
	if s, ok := table[key]; ok {
		return s
	}
	return key
}

// Fmt 取文案并用 fmt.Sprintf 填充占位符。
func Fmt(key string, args ...interface{}) string {
	return fmt.Sprintf(T(key), args...)
}
```

创建 `client/aly-client/ui/winui/i18n/zh_CN.go`：

```go
// +build windows

package i18n

var zhCN = map[string]string{
	KeyChecking:        "正在检查更新…",
	KeyNoUpdate:        "已是最新版本",
	KeyFoundVersion:    "发现新版本 v%s",
	KeyDownloading:     "正在下载 %s（%d/%d）",
	KeyDownloaded:      "下载完成",
	KeyApplying:        "正在应用更新…",
	KeyApplyDone:       "更新完成",
	KeyCheckNow:        "检查更新",
	KeyDownload:        "下载",
	KeyCancel:          "取消",
	KeyApply:           "立即应用",
	KeyLater:           "稍后",
	KeyRetry:           "重试",
	KeyClose:           "关闭",
	KeyCurrentVersion:  "当前版本 v%s",
	KeyAskDownload:     "是否下载新版本 v%s？",
	KeyAskApply:        "是否立即重启以应用 v%s？",
	KeyOffline:         "暂时无法检查更新",
	KeyErrorGeneric:    "更新过程中出现问题",
	KeyDetail:          "详细信息",
	KeyUpdateAvailable: "有可用更新",
	KeyRollback:        "回滚",
	KeyVersionList:     "可回滚版本",
}
```

创建 `client/aly-client/ui/winui/i18n/en_US.go`：

```go
// +build windows

package i18n

var enUS = map[string]string{
	KeyChecking:        "Checking for updates…",
	KeyNoUpdate:        "You are up to date",
	KeyFoundVersion:    "New version v%s available",
	KeyDownloading:     "Downloading %s (%d/%d)",
	KeyDownloaded:      "Download complete",
	KeyApplying:        "Applying update…",
	KeyApplyDone:       "Update complete",
	KeyCheckNow:        "Check for updates",
	KeyDownload:        "Download",
	KeyCancel:          "Cancel",
	KeyApply:           "Apply now",
	KeyLater:           "Later",
	KeyRetry:           "Retry",
	KeyClose:           "Close",
	KeyCurrentVersion:  "Current version v%s",
	KeyAskDownload:     "Download new version v%s?",
	KeyAskApply:        "Restart now to apply v%s?",
	KeyOffline:         "Unable to check for updates",
	KeyErrorGeneric:    "Something went wrong during the update",
	KeyDetail:          "Details",
	KeyUpdateAvailable: "Update available",
	KeyRollback:        "Rollback",
	KeyVersionList:     "Available versions",
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./ui/winui/i18n/`。
预期：`ok  aly/client/aly-client/ui/winui/i18n`。

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/i18n
git commit -m "feat(ui): i18n 骨架（中英键集一致，-lang 解析，未知回退中文）"
```

---

## 任务 7：`app` —— 读取 `client.json` 的 `main_exe_relative_path`

**文件：**
- 创建：`client/aly-client/ui/winui/app/config.go`
- 测试：`client/aly-client/ui/winui/app/config_test.go`

> **为什么需要**：UI 自更新（任务 15）要定位主程序目录 `MainFolder`，它由 `client.json` 的 `main_exe_relative_path`（相对 `UpdateFolder`）推导。
> **为什么不共享 `config/` 包**：AGENTS.md 禁止（会把 `shared.json`/崩溃恢复等磁盘副作用拽进 UI）。这里只读一个字段。

- [ ] **步骤 1：编写失败的测试**

创建 `client/aly-client/ui/winui/app/config_test.go`：

```go
// +build windows

package app

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestReadMainExeRelativePath(t *testing.T) {
	dir, err := ioutil.TempDir("", "alyui")
	if err != nil {
		t.Fatal(err)
	}
	content := `{
  "main_exe_relative_path": "../win-x64/YOFC.OTDR3001.exe",
  "must_close_process_name": ["YOFC.OTDR3001"]
}`
	if err := ioutil.WriteFile(filepath.Join(dir, "client.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMainExeRelativePath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "../win-x64/YOFC.OTDR3001.exe" {
		t.Fatalf("got %q", got)
	}
}

func TestReadMainExeRelativePathMissingFile(t *testing.T) {
	dir, _ := ioutil.TempDir("", "alyui")
	if _, err := ReadMainExeRelativePath(dir); err == nil {
		t.Fatal("expected error for missing client.json")
	}
}

func TestMainFolderFromUpdateFolder(t *testing.T) {
	// UpdateFolder 与 win-x64 同级
	update := filepath.Join("C:", "app", "UpdateFolder")
	got, err := MainFolder(update, "../win-x64/YOFC.OTDR3001.exe")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("C:", "app", "win-x64")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./ui/winui/app/`。
预期：FAIL，`undefined: ReadMainExeRelativePath`。

- [ ] **步骤 3：编写实现**

创建 `client/aly-client/ui/winui/app/config.go`：

```go
// +build windows

package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// clientConfig 只声明 UI 需要的字段（不复制 config 包的完整模型）。
type clientConfig struct {
	MainExeRelativePath string `json:"main_exe_relative_path"`
}

// ReadMainExeRelativePath 读取 UpdateFolder/client.json 的 main_exe_relative_path。
// 只读这一个字段；不解析 shared.json，不引入 config 包的磁盘副作用。
func ReadMainExeRelativePath(updateFolder string) (string, error) {
	path := filepath.Join(updateFolder, "client.json")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %v", path, err)
	}
	var c clientConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("parse %s: %v", path, err)
	}
	if c.MainExeRelativePath == "" {
		return "", fmt.Errorf("%s: main_exe_relative_path is empty", path)
	}
	return c.MainExeRelativePath, nil
}

// MainFolder 由 UpdateFolder 与 main_exe_relative_path 推导主程序目录。
// main_exe_relative_path 相对于 UpdateFolder，如 "../win-x64/YOFC.OTDR3001.exe"。
func MainFolder(updateFolder, mainExeRelativePath string) (string, error) {
	if mainExeRelativePath == "" {
		return "", fmt.Errorf("main_exe_relative_path is empty")
	}
	abs := filepath.Join(updateFolder, mainExeRelativePath)
	return filepath.Dir(abs), nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./ui/winui/app/`。
预期：`ok  aly/client/aly-client/ui/winui/app`。

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/app
git commit -m "feat(ui): 只读 client.json 的 main_exe_relative_path（不共享 config 包）"
```

---

## 任务 8：`bridge` —— 逐行 JSON 解析（纯函数 + 契约测试）

> ⚠️ **执行期修正（2026-10-19，实测发现）：结果类型必须用「键存在性」判定，不能用字段零值。**
> 原计划的探测方式 `if chk.HasUpdate || chk.CurrentVersion != ""` 有缺陷：
> `check_update` 在 `has_update=false` 且 `current_version` 为空时，`CheckUpdateData`
> 所有字段都是零值，会被误判成 `CheckSelfUpdateData`。
> 正确做法：先 `json.Unmarshal(data, &map[string]json.RawMessage)` 取键集合，再按
> 各 DTO 互不重叠的判别键分流：`index`→进度、`has_update`→CheckUpdate、
> `need_update`→CheckSelfUpdate、`versions`→RollbackList、`version`→Rollback。
> 已按此实现并补了回归用例 `TestParseLineCheckUpdateNoUpdateEmptyCurrentVersion`。

**文件：**
- 创建：`client/aly-client/ui/winui/bridge/parse.go`
- 测试：`client/aly-client/ui/winui/bridge/parse_test.go`

> 覆盖 `DEVELOPMENT_PLAN.md` §3.3 的 6 条解析规则。

- [ ] **步骤 1：编写失败的测试**

创建 `client/aly-client/ui/winui/bridge/parse_test.go`：

```go
// +build windows

package bridge

import "testing"

func TestParseLineFinalOutput(t *testing.T) {
	line := `{"isSuccess":true,"errorMsg":"","data":{"has_update":true,"need_download_update":true,"current_version":"1.0.0","new_version":"1.0.1","force_update":true}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindResult || !got.Success {
		t.Fatalf("want result success, got %+v", got)
	}
	if got.Check == nil || !got.Check.HasUpdate || got.Check.ForceUpdate == nil || !*got.Check.ForceUpdate {
		t.Fatalf("bad CheckUpdateData: %+v", got.Check)
	}
}

func TestParseLineForceUpdateAbsent(t *testing.T) {
	// downloaded 续跑分支不返回 force_update —— 必须解析成 nil，而不是 false
	line := `{"isSuccess":true,"data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if got.Check.ForceUpdate != nil {
		t.Fatalf("force_update absent must be nil, got %v", *got.Check.ForceUpdate)
	}
}

func TestParseLineProgress(t *testing.T) {
	line := `{"isSuccess":true,"data":{"index":2,"total":5,"file":"a.dll","status":"DONE","file_size":1024}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindProgress {
		t.Fatalf("want progress, got %v", got.Kind)
	}
	if got.Progress.Index != 2 || got.Progress.Total != 5 || got.Progress.Status != "DONE" {
		t.Fatalf("bad progress: %+v", got.Progress)
	}
}

func TestParseLineProgressDone(t *testing.T) {
	// data:null 是下载完成行
	got, err := ParseLine(`{"isSuccess":true,"data":null}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindDone {
		t.Fatalf("want done, got %v", got.Kind)
	}
}

func TestParseLineProgressFail(t *testing.T) {
	line := `{"isSuccess":false,"errorMsg":"a.dll: checksum mismatch","data":{"index":2,"total":5,"file":"a.dll","status":"FAIL","file_size":1024,"error":"checksum mismatch"}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindFail || got.Success {
		t.Fatalf("want fail, got %+v", got)
	}
	if got.ErrMsg != "a.dll: checksum mismatch" {
		t.Fatalf("bad errMsg %q", got.ErrMsg)
	}
}

func TestParseLineNonJSONIsSkipped(t *testing.T) {
	// usage 文本 / panic 输出：不能报错，也不能当成结果
	for _, line := range []string{
		"Usage of apply_update:",
		"  -close-timeout int",
		"panic: runtime error: index out of range",
		"",
		"   ",
	} {
		got, err := ParseLine(line)
		if err != nil {
			t.Fatalf("line %q should not error, got %v", line, err)
		}
		if got.Kind != KindIgnore {
			t.Fatalf("line %q should be ignored, got %v", line, got.Kind)
		}
	}
}

func TestParseLineErrorResult(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":false,"errorMsg":"no server url configured","data":null}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindResult || got.Success {
		t.Fatalf("want failed result, got %+v", got)
	}
	if got.ErrMsg != "no server url configured" {
		t.Fatalf("bad errMsg %q", got.ErrMsg)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./ui/winui/bridge/`。
预期：FAIL，`undefined: ParseLine`。

- [ ] **步骤 3：编写实现**

创建 `client/aly-client/ui/winui/bridge/parse.go`：

```go
// +build windows

package bridge

import (
	"encoding/json"
	"strings"

	"aly/client/aly-client/model"
)

// Kind 表示一行 stdout 的语义。
type Kind int

const (
	KindIgnore   Kind = iota // 非 JSON / 空行 → 跳过
	KindResult               // 最终结果（单行 JSON，data 为具体结构或 null）
	KindProgress             // 下载进度行
	KindDone                 // 下载完成行（data:null）
	KindFail                 // 进度失败行（isSuccess:false 且带进度）
)

// Parsed 是一行的解析结果。
type Parsed struct {
	Kind     Kind
	Success  bool
	ErrMsg   string
	Check    *model.CheckUpdateData
	Self     *model.CheckSelfUpdateData
	Progress *model.DownloadProgress
	Rollback *model.RollbackData
	Versions *model.RollbackListData
}

// envelope 用于先取出 data 的原始字节，再按命令类型二次解析。
type envelope struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrMsg    string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

// ParseLine 解析一行 stdout。非 JSON 行返回 KindIgnore 且 err==nil（调用方跳过即可）。
func ParseLine(line string) (Parsed, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
		return Parsed{Kind: KindIgnore}, nil
	}

	var env envelope
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		// 形似 JSON 但解析失败：按忽略处理，不中断流程
		return Parsed{Kind: KindIgnore}, nil
	}

	p := Parsed{Success: env.IsSuccess, ErrMsg: env.ErrMsg}

	if len(env.Data) == 0 || string(env.Data) == "null" {
		if !env.IsSuccess {
			p.Kind = KindResult // 失败结果（data:null）
		} else {
			p.Kind = KindDone // 成功且 data:null = 下载完成
		}
		return p, nil
	}

	// 进度行：data 里含 index/total/status
	var prog model.DownloadProgress
	if err := json.Unmarshal(env.Data, &prog); err == nil && prog.Total > 0 && prog.Status != "" {
		p.Progress = &prog
		if !env.IsSuccess {
			p.Kind = KindFail
		} else {
			p.Kind = KindProgress
		}
		return p, nil
	}

	// 其余为各命令的结果结构；按字段探测，互不冲突
	var chk model.CheckUpdateData
	if err := json.Unmarshal(env.Data, &chk); err == nil && (chk.HasUpdate || chk.CurrentVersion != "") {
		p.Check = &chk
		p.Kind = KindResult
		return p, nil
	}
	var self model.CheckSelfUpdateData
	if err := json.Unmarshal(env.Data, &self); err == nil {
		p.Self = &self
		p.Kind = KindResult
		return p, nil
	}
	var vers model.RollbackListData
	if err := json.Unmarshal(env.Data, &vers); err == nil {
		p.Versions = &vers
		p.Kind = KindResult
		return p, nil
	}
	var rb model.RollbackData
	if err := json.Unmarshal(env.Data, &rb); err == nil {
		p.Rollback = &rb
		p.Kind = KindResult
		return p, nil
	}

	p.Kind = KindIgnore
	return p, nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./ui/winui/bridge/`。
预期：`ok  aly/client/aly-client/ui/winui/bridge`。

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/bridge/parse.go client/aly-client/ui/winui/bridge/parse_test.go
git commit -m "feat(ui): stdout 逐行 JSON 解析（含非 JSON/force_update 缺失/进度行契约）"
```

---

## 任务 9：`bridge` —— 进程执行（EOF、退出码、超时、活性检测）

**文件：**
- 创建：`client/aly-client/ui/winui/bridge/client.go`
- 创建：`client/aly-client/ui/winui/bridge/commands.go`

> **必须避开 C# SDK 的两个缺陷**：① 未设 `EnableRaisingEvents` 导致 `ReadLine()` 返回 null 后忙等死循环；② `RunAsyncAlone` 不读 stdout、不看退出码、永远返回 OK。

- [ ] **步骤 1：实现进程执行器**

创建 `client/aly-client/ui/winui/bridge/client.go`：

```go
// +build windows

// Package bridge 以子进程方式调用 aly-client.exe 并解析其 JSON 输出。
package bridge

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Options 控制单次调用。
type Options struct {
	// TotalTimeout 为 0 表示不设总超时（下载用）。
	TotalTimeout time.Duration
	// IdleTimeout 为 0 表示不做活性检测；>0 时若超过该时长没有任何 stdout 行则判定卡死。
	IdleTimeout time.Duration
}

// Client 调用 aly-client.exe。
type Client struct {
	exePath string
}

// New 创建 Client。exePath 为 aly-client.exe 的绝对路径。
func New(exePath string) *Client {
	return &Client{exePath: exePath}
}

// ExeDir 返回 aly-client.exe 所在目录（= UpdateFolder）。
func (c *Client) ExeDir() string {
	return filepath.Dir(c.exePath)
}

// Run 执行命令并返回最后一行结果。progress 为 nil 表示不关心进度。
//
// 契约（对应 DEVELOPMENT_PLAN §3.3）：
//   - ReadLine 返回 EOF → 正常结束循环（**绝不忙等**）
//   - 非 JSON 行 → 跳过
//   - stdout 为空且退出码非 0 → 返回错误（含 stderr 诊断）
//   - stdout 有结果但退出码非 0 → 以结果为准（与 C# SDK 一致）
//   - stderr 只记入 error 文本，不单独返回
func (c *Client) Run(ctx context.Context, args []string, opt Options, progress func(progressInfo)) (Parsed, error) {
	if opt.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.TotalTimeout)
		defer cancel()
	}

	cmd := exec.Command(c.exePath, args...)
	cmd.Dir = c.ExeDir() // 对齐 C# SDK，且保证 client.json 相对路径正确
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Parsed{}, fmt.Errorf("stdout pipe: %v", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return Parsed{}, fmt.Errorf("start %s: %v", c.exePath, err)
	}

	// 取消 / 超时 → 杀进程
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()

	var result Parsed
	found := false

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// 活性检测
	var idleTimer *time.Timer
	if opt.IdleTimeout > 0 {
		idleTimer = time.AfterFunc(opt.IdleTimeout, func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})
		defer idleTimer.Stop()
	}

	for scanner.Scan() {
		if idleTimer != nil {
			idleTimer.Reset(opt.IdleTimeout)
		}
		p, perr := ParseLine(scanner.Text())
		if perr != nil {
			continue
		}
		switch p.Kind {
		case KindProgress:
			if progress != nil {
				progress(progressInfo{Index: p.Progress.Index, Total: p.Progress.Total, File: p.Progress.File})
			}
		case KindFail:
			return p, nil
		case KindDone:
			found = true
			result = p
		case KindResult:
			result = p
			found = true
		}
	}
	// scanner 结束 = EOF。这里是**唯一**的正常退出点，不存在忙等。

	waitErr := cmd.Wait()

	if !found {
		msg := stderrBuf.String()
		if msg == "" {
			msg = fmt.Sprintf("aly-client.exe exited with code %d", exitCode(waitErr))
		}
		return Parsed{}, fmt.Errorf("%s %v: %s", c.exePath, args, msg)
	}
	// stdout 有结果 → 以结果为准，退出码只作日志（与 C# SDK 一致）
	return result, nil
}

// progressInfo 是回调用的进度快照（不直接暴露 model 类型，保持 bridge 对外面窄）。
type progressInfo struct {
	Index int
	Total int
	File  string
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}
```

- [ ] **步骤 2：实现命令封装**

创建 `client/aly-client/ui/winui/bridge/commands.go`：

```go
// +build windows

package bridge

import (
	"context"
	"time"
)

// 超时（对应 DEVELOPMENT_PLAN §3.3）。
const (
	quickTimeout = 30 * time.Second
	idleTimeout  = 90 * time.Second
	applyTimeout = 600 * time.Second
)

// CheckUpdate 调用 check_update（不传任何可选参数，靠 client.json）。
func (c *Client) CheckUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"check_update"}, Options{TotalTimeout: quickTimeout}, nil)
}

// CheckSelfUpdate 调用 check_self_update。
func (c *Client) CheckSelfUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"check_self_update"}, Options{TotalTimeout: quickTimeout}, nil)
}

// DownloadUpdate 调用 download_update，逐行回调进度。
func (c *Client) DownloadUpdate(ctx context.Context, onProgress func(progressInfo)) (Parsed, error) {
	return c.Run(ctx, []string{"download_update"},
		Options{IdleTimeout: idleTimeout}, onProgress)
}

// ApplyUpdate 调用 apply_update（--close-timeout 用默认 30s）。
func (c *Client) ApplyUpdate(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"apply_update"}, Options{TotalTimeout: applyTimeout}, nil)
}

// ListRollbackVersions 调用 list_rollback_versions。
func (c *Client) ListRollbackVersions(ctx context.Context) (Parsed, error) {
	return c.Run(ctx, []string{"list_rollback_versions"}, Options{TotalTimeout: quickTimeout}, nil)
}

// Rollback 调用 rollback --version <v>。
func (c *Client) Rollback(ctx context.Context, version string) (Parsed, error) {
	return c.Run(ctx, []string{"rollback", "--version", version},
		Options{TotalTimeout: applyTimeout}, nil)
}
```

> 注：`progressInfo` 未导出，`commands.go` 与 `client.go` 同包，可直接用。

- [ ] **步骤 3：编译**

运行：任务 1 步骤 4 的编译命令（或 `go build ./ui/winui/...`）。
预期：编译成功。
若报 `TotalTimeout` 未使用 → 在 `Run` 中用 `context.WithTimeout` 包装 `ctx`，别留下死字段：

```go
	if opt.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.TotalTimeout)
		defer cancel()
	}
```
（把这段放在 `cmd := exec.Command` 之前。）

- [ ] **步骤 4：Commit**

```bash
git add client/aly-client/ui/winui/bridge/client.go client/aly-client/ui/winui/bridge/commands.go
git commit -m "feat(ui): aly-client 子进程执行器（EOF 正常结束、退出码/超时/活性检测契约）"
```

---

## 任务 10：`flow` —— 状态机与 `force_update` 缺失规则

**文件：**
- 创建：`client/aly-client/ui/winui/flow/events.go`
- 创建：`client/aly-client/ui/winui/flow/updater.go`
- 测试：`client/aly-client/ui/winui/flow/updater_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `client/aly-client/ui/winui/flow/updater_test.go`：

```go
// +build windows

package flow

import (
	"context"
	"testing"
)

// decide 是纯函数：根据 check 结果与现场决定下一步动作。
func TestDecideForceUpdateTrue(t *testing.T) {
	force := true
	got := Decide(CheckView{HasUpdate: true, NeedDownload: true, ForceUpdate: &force, NewVersion: "1.0.1"})
	if got.Action != ActionDownload || got.Ask {
		t.Fatalf("forced update must auto-download without asking, got %+v", got)
	}
}

func TestDecideForceUpdateFalseAsks(t *testing.T) {
	f := false
	got := Decide(CheckView{HasUpdate: true, NeedDownload: true, ForceUpdate: &f, NewVersion: "1.0.1"})
	if got.Action != ActionDownload || !got.Ask {
		t.Fatalf("non-forced must ask before download, got %+v", got)
	}
}

func TestDecideForceUpdateAbsentAsks(t *testing.T) {
	// 字段缺失（nil）必须按"询问"处理，不能当成强制
	got := Decide(CheckView{HasUpdate: true, NeedDownload: true, ForceUpdate: nil, NewVersion: "1.0.1"})
	if got.Action != ActionDownload || !got.Ask {
		t.Fatalf("absent force_update must ask, got %+v", got)
	}
}

func TestDecideApplyingAlwaysAuto(t *testing.T) {
	// applying（崩溃恢复）：主目录可能已丢失，必须无条件自动继续
	got := Decide(CheckView{HasUpdate: true, NeedDownload: false, Applying: true})
	if got.Action != ActionApply || got.Ask {
		t.Fatalf("applying must auto-apply without asking, got %+v", got)
	}
}

func TestDecideNoUpdate(t *testing.T) {
	got := Decide(CheckView{HasUpdate: false})
	if got.Action != ActionNone {
		t.Fatalf("no update → ActionNone, got %+v", got)
	}
}

func TestDecidePendingApplyAsksWhenNotForced(t *testing.T) {
	got := Decide(CheckView{HasUpdate: true, NeedDownload: false, ForceUpdate: nil, NewVersion: "1.0.1"})
	if got.Action != ActionApply || !got.Ask {
		t.Fatalf("pending apply without force must ask, got %+v", got)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./ui/winui/flow/`。
预期：FAIL，`undefined: Decide`。

- [ ] **步骤 3：编写实现**

创建 `client/aly-client/ui/winui/flow/events.go`：

```go
// +build windows

// Package flow 实现与 C# AlyUpdateClient 对齐的更新状态机。
package flow

// Status 对齐 C# AlyClientStatus。
type Status int

const (
	StatusNone Status = iota
	StatusDiscoveredUpdate
	StatusDownloadingUpdate
	StatusDownloadedUpdate
	StatusApplyUpdate
)

// Action 是状态机决定的下一步动作。
type Action int

const (
	ActionNone Action = iota
	ActionDownload
	ActionApply
)

// CheckView 是 check_update 结果中状态机关心的字段。
type CheckView struct {
	HasUpdate   bool
	NeedDownload bool
	// ForceUpdate 为 nil 表示服务端未返回该字段（见 DEVELOPMENT_PLAN §3.5）。
	ForceUpdate *bool
	// Applying 表示 version_status == applying（崩溃恢复现场）。
	Applying   bool
	NewVersion string
	CurrentVersion string
}

// Decision 是状态机的输出。
type Decision struct {
	Action Action
	// Ask 为 true 表示必须先询问用户（非强制更新）。
	Ask bool
}
```

创建 `client/aly-client/ui/winui/flow/updater.go`：

```go
// +build windows

package flow

// Decide 依据 check_update 结果与现场决定下一步。
//
// 规则（DEVELOPMENT_PLAN §3.5）：
//   - applying（崩溃恢复）→ 无条件自动 apply，不询问（主目录可能已丢失，必须自愈）
//   - 有更新且需下载：force_update==true → 自动下载；false **或缺失** → 询问
//   - 有更新且不需下载（续跑 apply）：force_update==true → 自动应用；否则询问
//   - 无更新 → 不做任何事
func Decide(c CheckView) Decision {
	if c.Applying {
		return Decision{Action: ActionApply, Ask: false}
	}
	if !c.HasUpdate {
		return Decision{Action: ActionNone}
	}
	forced := c.ForceUpdate != nil && *c.ForceUpdate
	if c.NeedDownload {
		return Decision{Action: ActionDownload, Ask: !forced}
	}
	return Decision{Action: ActionApply, Ask: !forced}
}

// Updater 驱动完整的检查→下载→应用循环。
type Updater struct {
	// Bridge 为实际执行者；抽成接口便于测试与解耦 view。
	Bridge Bridge
	Sink   Sink
}

// Bridge 是 flow 对 bridge 包的最小依赖。
type Bridge interface {
	CheckUpdate(ctx context.Context) (CheckView, error)
	DownloadUpdate(ctx context.Context, onProgress func(index, total int, file string)) error
	ApplyUpdate(ctx context.Context) error
}

// Sink 由 view 实现，接收状态与询问请求。
type Sink interface {
	OnStatus(s Status, detail string)
	OnProgress(index, total int, file string)
	OnError(msg string, detail string)
	// Ask 阻塞直到用户做出选择；返回 false 表示用户拒绝。
	Ask(kind AskKind, newVersion string) bool
}

// AskKind 区分询问场景。
type AskKind int

const (
	AskDownload AskKind = iota
	AskApply
)
```

> 注意：`updater.go` 需 `import "context"`；`Bridge` 接口的 `CheckUpdate` 返回 `flow.CheckView`（不是 `bridge.Parsed`），由 `app` 层做适配（任务 13 的 `toCheckView`）。这样 `flow` 不 import `bridge`，保持单向依赖。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./ui/winui/flow/`。
预期：`ok  aly/client/aly-client/ui/winui/flow`。

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/flow
git commit -m "feat(ui): 更新状态机 Decide（force_update 缺失=询问，applying=无条件自动续跑）"
```

---

## 任务 11：`view` —— 按需显示与 5 秒自动关闭

**文件：**
- 修改：`client/aly-client/ui/winui/view/window.go`

- [ ] **步骤 1：新增自动关闭定时器**

在 `window.go` 追加：

```go
import "time" // 加到既有 import 块

// autoCloseTimer 由 check 模式下的"无更新"提示使用。
type autoCloseTimer struct {
	t *time.Timer
}

// AutoCloseAfter 在 d 之后关闭窗口。若期间调用 Cancel 则不会关闭。
// 回调通过 walk.Synchronize 回到 UI 线程。
func (w *Window) AutoCloseAfter(d time.Duration) *autoCloseTimer {
	act := &autoCloseTimer{}
	act.t = time.AfterFunc(d, func() {
		w.Synchronize(func() {
			w.Hide()
		})
	})
	return act
}

// Cancel 取消自动关闭（用户已产生交互时调用）。
func (a *autoCloseTimer) Cancel() {
	if a != nil && a.t != nil {
		a.t.Stop()
	}
}
```

- [ ] **步骤 2：在 `main.go` 接入模式分派**

替换 `client/aly-client/ui/winui/main.go` 的 `main()` 为：

```go
func main() {
	// 解析参数（flag 包，Go 1.10 可用）
	mode := flag.String("mode", "auto", "auto|check|update|rollback|silent")
	lang := flag.String("lang", "zh-CN", "zh-CN|en-US")
	url := flag.String("url", "", "server url override")
	project := flag.String("project-name", "", "project name override")
	flag.Parse()

	i18n.SetLang(i18n.ParseLang(*lang))
	_ = url
	_ = project // 透传逻辑见任务 13

	w, err := view.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create window failed: %v\n", err)
		os.Exit(1)
	}

	switch *mode {
	case "auto", "silent":
		// 启动：静默检查，有更新才显示（任务 13 接线；M0 阶段先直接退出）
		os.Exit(0)
	case "check":
		// 手动检查：总是显示；无更新 → 5s 自动关闭（任务 13 接线）
		w.Show()
		os.Exit(w.Run())
	default:
		w.Show()
		os.Exit(w.Run())
	}
}
```

> 记得在 import 加 `"flag"` 与 `"aly/client/aly-client/ui/winui/i18n"`。

- [ ] **步骤 3：编译**

运行：任务 1 步骤 4 的编译命令。
预期：编译成功。

- [ ] **步骤 4：XP 手工验收**

| 场景 | 期望 |
|------|------|
| `aly-client-ui.exe -mode check` | 右下角弹窗；若无更新，显示"已是最新版本"，**5 秒后自动消失** |
| 5 秒内点击窗口 | 定时器被取消，窗口**不**自动消失 |
| `aly-client-ui.exe -mode auto` | **不显示**窗口（M0 阶段无更新逻辑，直接退出） |

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/view/window.go client/aly-client/ui/winui/main.go
git commit -m "feat(ui): 按需显示 + check 模式无更新 5s 自动关闭"
```

---

## 任务 12：`view` —— 自绘卡片与进度条（现代化样式）

**文件：**
- 创建：`client/aly-client/ui/winui/view/theme/theme.go`
- 创建：`client/aly-client/ui/winui/view/card.go`
- 创建：`client/aly-client/ui/winui/view/progress.go`

- [ ] **步骤 1：定义主题常量**

创建 `client/aly-client/ui/winui/view/theme/theme.go`：

```go
// +build windows

// Package theme 集中定义配色、字体与尺寸。单一强调色 + 中性灰阶。
package theme

import "github.com/lxn/walk"

// 配色（直角卡片 + 1px 描边；XP 无 DWM 阴影，用描边代替投影）
var (
	Background   = walk.RGB(0xFF, 0xFF, 0xFF)
	Border       = walk.RGB(0xE5, 0xE7, 0xEB)
	TextPrimary  = walk.RGB(0x11, 0x18, 0x27)
	TextSecondry = walk.RGB(0x6B, 0x72, 0x80)
	Accent       = walk.RGB(0x25, 0x63, 0xEB)
	AccentHover  = walk.RGB(0x1D, 0x4E, 0xD8)
	Track        = walk.RGB(0xE5, 0xE7, 0xEB)
)

// 尺寸
const (
	CardW      = 380
	CardH      = 160
	Pad        = 16
	Radius     = 0 // 本期不做窗口圆角；内容圆角另算
	ProgressH  = 6
	TitlePt    = 11
	BodyPt     = 9
	VersionPt  = 8
)

// Font 返回正文字体：优先 Segoe UI，XP 回退 Tahoma。
func Font() *walk.Font {
	if f, err := walk.NewFont("Segoe UI", BodyPt, 0); err == nil {
		return f
	}
	f, _ := walk.NewFont("Tahoma", BodyPt, 0)
	return f
}

// TitleFont 返回标题字体。
func TitleFont() *walk.Font {
	if f, err := walk.NewFont("Segoe UI", TitlePt, walk.FontBold); err == nil {
		return f
	}
	f, _ := walk.NewFont("Tahoma", TitlePt, walk.FontBold)
	return f
}
```

- [ ] **步骤 2：实现自绘进度条**

创建 `client/aly-client/ui/winui/view/progress.go`：

```go
// +build windows

package view

import (
	"github.com/lxn/walk"

	"aly/client/aly-client/ui/winui/view/theme"
)

// ProgressBar 包装 walk.CustomWidget，提供设置进度的方法。
// walk 的 CustomWidget 需要回调拿到自身，因此用结构体持有引用。
type ProgressBar struct {
	cw       *walk.CustomWidget
	cur, max int
}

// NewProgressBar 创建自绘进度条。
// 注意：walk 官方自绘入口是 NewCustomWidget(parent Container, style uint, paint PaintFunc)，
// PaintFunc 签名为 func(canvas *walk.Canvas, updateBounds walk.Rectangle) error。
func NewProgressBar(parent walk.Container) (*ProgressBar, error) {
	pb := &ProgressBar{cur: 0, max: 1}

	cw, err := walk.NewCustomWidget(parent, 0,
		func(canvas *walk.Canvas, updateBounds walk.Rectangle) error {
			return pb.paint(canvas)
		})
	if err != nil {
		return nil, err
	}
	// PaintBuffered = 双缓冲，避免 XP 上重绘闪烁
	cw.SetPaintMode(walk.PaintBuffered)
	cw.SetInvalidatesOnResize(true)
	pb.cw = cw
	return pb, nil
}

// SetProgress 设置进度（index/total）。
func (pb *ProgressBar) SetProgress(cur, max int) {
	if max <= 0 {
		max = 1
	}
	pb.cur, pb.max = cur, max
	if pb.cw != nil {
		pb.cw.Invalidate()
	}
}

// Widget 返回底层控件，供布局使用。
func (pb *ProgressBar) Widget() walk.Widget {
	return pb.cw
}

// paint 自绘：底槽 + 填充。Canvas 的 API 已按钉选版本核对：
//   FillRectangle(brush Brush, bounds Rectangle) error
func (pb *ProgressBar) paint(canvas *walk.Canvas) error {
	b := pb.cw.ClientBounds()
	h := theme.ProgressH
	y := b.Y + (b.Height-h)/2

	track, err := walk.NewSolidColorBrush(theme.Track)
	if err != nil {
		return err
	}
	defer track.Dispose()
	if err := canvas.FillRectangle(track, walk.Rectangle{X: b.X, Y: y, Width: b.Width, Height: h}); err != nil {
		return err
	}

	w := 0
	if pb.max > 0 {
		w = b.Width * pb.cur / pb.max
	}
	if w > 0 {
		fill, err := walk.NewSolidColorBrush(theme.Accent)
		if err != nil {
			return err
		}
		defer fill.Dispose()
		if err := canvas.FillRectangle(fill, walk.Rectangle{X: b.X, Y: y, Width: w, Height: h}); err != nil {
			return err
		}
	}
	return nil
}
```

> 上述 API 已在钉选版本 `walk@bf6b8a4b` 的源码中核对：
> `func NewCustomWidget(parent Container, style uint, paint PaintFunc) (*CustomWidget, error)`、
> `type PaintFunc func(canvas *Canvas, updateBounds Rectangle) error`、
> `func (c *Canvas) FillRectangle(brush Brush, bounds Rectangle) error`、
> `func (cw *CustomWidget) SetPaintMode(value PaintMode)`、`walk.PaintBuffered`、
> `func (wb *WindowBase) ClientBounds() Rectangle`。

- [ ] **步骤 3：实现自绘卡片（标题 + 状态 + 进度 + 按钮）**

创建 `client/aly-client/ui/winui/view/card.go`：

```go
// +build windows

package view

import (
	"github.com/lxn/walk"

	"aly/client/aly-client/ui/winui/view/theme"
)

// Card 包装 walk.CustomWidget，自绘白底 + 1px 描边 + 标题/版本/状态。
type Card struct {
	cw      *walk.CustomWidget
	title   string
	version string
	status  string
}

// NewCard 创建卡片。
func NewCard(parent walk.Container) (*Card, error) {
	c := &Card{}

	cw, err := walk.NewCustomWidget(parent, 0,
		func(canvas *walk.Canvas, updateBounds walk.Rectangle) error {
			return c.paint(canvas)
		})
	if err != nil {
		return nil, err
	}
	cw.SetPaintMode(walk.PaintBuffered) // 双缓冲，避免 XP 闪烁
	cw.SetInvalidatesOnResize(true)
	c.cw = cw
	return c, nil
}

// SetContent 更新文案。
func (c *Card) SetContent(title, version, status string) {
	c.title, c.version, c.status = title, version, status
	if c.cw != nil {
		c.cw.Invalidate()
	}
}

// Widget 返回底层控件，供布局使用。
func (c *Card) Widget() walk.Widget {
	return c.cw
}

// paint 自绘：直角卡片 + 1px 描边 + 文本。
func (c *Card) paint(canvas *walk.Canvas) error {
	b := c.cw.ClientBounds()

	bg, err := walk.NewSolidColorBrush(theme.Background)
	if err != nil {
		return err
	}
	defer bg.Dispose()
	if err := canvas.FillRectangle(bg, b); err != nil {
		return err
	}

	// 1px 描边（XP 无 DWM 阴影，用描边代替投影）
	pen, err := walk.NewCosmeticPen(walk.PenSolid, theme.Border)
	if err != nil {
		return err
	}
	defer pen.Dispose()
	if err := canvas.DrawRectangle(pen, walk.Rectangle{
		X: b.X, Y: b.Y, Width: b.Width - 1, Height: b.Height - 1,
	}); err != nil {
		return err
	}

	// 标题（左上）
	if err := c.drawText(canvas, c.title, b.X+theme.Pad, b.Y+theme.Pad, theme.TitleFont()); err != nil {
		return err
	}

	// 版本号（右上，右对齐：用 MeasureText 求宽后反推 X）
	if c.version != "" {
		f := theme.Font()
		mr, _, err := canvas.MeasureText(c.version, f,
			walk.Rectangle{X: 0, Y: 0, Width: b.Width, Height: 24},
			walk.TextRight|walk.TextVCenter|walk.TextSingleLine)
		if err == nil {
			_ = c.drawText(canvas, c.version,
				b.X+b.Width-theme.Pad-mr.Width, b.Y+theme.Pad+2, f)
		}
	}

	// 状态行
	if c.status != "" {
		if err := c.drawText(canvas, c.status, b.X+theme.Pad, b.Y+theme.Pad+34, theme.Font()); err != nil {
			return err
		}
	}
	return nil
}

// drawText 在 (x,y) 处绘制单行左对齐文本。
// Canvas.DrawText(text string, font *Font, color Color, bounds Rectangle, format DrawTextFormat) error
func (c *Card) drawText(canvas *walk.Canvas, s string, x, y int, f *walk.Font) error {
	if s == "" || f == nil {
		return nil
	}
	return canvas.DrawText(s, f, theme.TextPrimary,
		walk.Rectangle{X: x, Y: y, Width: 320, Height: 24},
		walk.TextLeft|walk.TextVCenter|walk.TextSingleLine)
}
```

> 上述 API 已在 `walk@bf6b8a4b` 源码中核对：
> `func (c *Canvas) FillRectangle(brush Brush, bounds Rectangle) error`、
> `func (c *Canvas) DrawRectangle(pen Pen, bounds Rectangle) error`、
> `func (c *Canvas) DrawText(text string, font *Font, color Color, bounds Rectangle, format DrawTextFormat) error`、
> `func (c *Canvas) MeasureText(text string, font *Font, bounds Rectangle, format DrawTextFormat) (Rectangle, int, error)`、
> `func NewCosmeticPen(style PenStyle, color Color) (*CosmeticPen, error)`、`walk.PenSolid`、
> `walk.TextLeft` / `walk.TextRight` / `walk.TextVCenter` / `walk.TextSingleLine`。
> 另：`Canvas` 也提供 `FillRoundedRectangle(brush Brush, bounds Rectangle, ellipseSize Size) error`，
> 内容区若要圆角可直接用（窗口圆角本期仍不做，见 DEVELOPMENT_PLAN §4.2）。

- [ ] **步骤 4：编译 + XP 目视验收**

编译：任务 1 步骤 4。
XP 目视确认：
| 检查 | 期望 |
|------|------|
| 卡片外观 | 白底、直角、1px 浅灰描边 |
| 字体 | Segoe UI（若装）或 Tahoma |
| 无闪烁 | 拖动/重绘不闪（`SetInvalidatesOnResize(true)` + 双缓冲由 walk 处理） |

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/view/theme client/aly-client/ui/winui/view/card.go client/aly-client/ui/winui/view/progress.go
git commit -m "feat(ui): 自绘扁平卡片与进度条（直角+描边，Segoe UI/Tahoma 回退）"
```

---

## 任务 13：接线 —— `auto`/`check` 两种场景端到端

**文件：**
- 创建：`client/aly-client/ui/winui/app/app.go`
- 修改：`client/aly-client/ui/winui/main.go`

- [ ] **步骤 1：实现装配层**

创建 `client/aly-client/ui/winui/app/app.go`：

```go
// +build windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"aly/client/aly-client/ui/winui/bridge"
	"aly/client/aly-client/ui/winui/flow"
	"aly/client/aly-client/ui/winui/i18n"
	"aly/client/aly-client/ui/winui/view"
)

// Mode 是启动模式。
type Mode string

const (
	ModeAuto   Mode = "auto"
	ModeSilent Mode = "silent"
	ModeCheck  Mode = "check"
	ModeUpdate Mode = "update"
	ModeRoll   Mode = "rollback"
)

// Run 执行一次完整流程（单次检查，不做后台轮询）。
// 返回进程退出码。
func Run(mode Mode, w *view.Window) int {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return 1
	}
	cli := bridge.New(filepath.Join(exeDir, "aly-client.exe"))
	ctx := context.Background()

	res, err := cli.CheckUpdate(ctx)
	if err != nil {
		w.Show()
		w.SetContent("aly-client-ui", "", i18n.T(i18n.KeyErrorGeneric))
		return w.Run()
	}

	cv := toCheckView(res)
	dec := flow.Decide(cv)

	// auto/silent：无更新 → 不显示，直接退出
	if (mode == ModeAuto || mode == ModeSilent) && dec.Action == flow.ActionNone {
		return 0
	}

	w.Show()

	// check：无更新 → 显示"已是最新版本" → 5s 自动关闭
	if dec.Action == flow.ActionNone {
		w.SetContent("aly-client-ui", "", i18n.T(i18n.KeyNoUpdate))
		if mode == ModeCheck {
			w.AutoCloseAfter(5 * time.Second)
		}
		return w.Run()
	}

	// 有更新：显示，按 Decide 的 Ask 决定是否询问（询问对话框由 view 提供）
	w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.Fmt(i18n.KeyFoundVersion, cv.NewVersion))
	return w.Run()
}

func toCheckView(p bridge.Parsed) flow.CheckView {
	if p.Check == nil {
		return flow.CheckView{}
	}
	return flow.CheckView{
		HasUpdate:      p.Check.HasUpdate,
		NeedDownload:   p.Check.NeedDownloadUpdate,
		ForceUpdate:    p.Check.ForceUpdate,
		NewVersion:     p.Check.NewVersion,
		CurrentVersion: p.Check.CurrentVersion,
	}
}
```

- [ ] **步骤 2：`view` 暴露 `SetContent`**

在 `window.go` 追加：

```go
// SetContent 更新卡片文案（内部转发给 Card）。
func (w *Window) SetContent(title, version, status string) {
	if w.card != nil {
		w.card.SetContent(title, version, status)
	}
}
```

并在 `New()` 里创建卡片与布局（`walk.VBoxLayout`，把 `card` 与 `progress` 加进去），把 `card`/`progress` 存进 `Window` 结构体：

```go
	// 在 New() 中，positionBottomRight() 之前
	if _, err := walk.NewVBoxLayout(mw); err != nil {
		return nil, err
	}
	card, err := NewCard(mw)
	if err != nil {
		return nil, err
	}
	pb, err := NewProgressBar(mw)
	if err != nil {
		return nil, err
	}
	w.card = card
	w.progress = pb
```

- [ ] **步骤 3：`main.go` 调用 `app.Run`**

```go
	m := app.Mode(*mode)
	os.Exit(app.Run(m, w))
```
（替换任务 11 步骤 2 里的 `switch` 分派。）

- [ ] **步骤 4：端到端手工验收**

| 场景 | 命令 | 期望 |
|------|------|------|
| 启动、无更新 | `aly-client-ui.exe -mode auto` | **不显示**任何窗口，进程退出 |
| 启动、有更新 | `aly-client-ui.exe -mode auto`（服务端有新版） | 右下角显示"发现新版本 vX.Y.Z" |
| 手动检查、无更新 | `aly-client-ui.exe -mode check` | 显示"已是最新版本"，**5s 后自动关闭** |
| 手动检查、有更新 | `aly-client-ui.exe -mode check` | 显示更新界面，**不自动关闭** |
| 服务器不可达 | 断网后 `-mode check` | 显示"已是最新版本"（`has_update=false` 是成功） |

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/app/app.go client/aly-client/ui/winui/view/window.go client/aly-client/ui/winui/main.go
git commit -m "feat(ui): 接线 auto/check 两种场景（无更新不显示 / 5s 自动关闭）"
```

---

## 任务 14：下载与应用（含询问对话框）

**文件：**
- 修改：`client/aly-client/ui/winui/view/window.go`（询问对话框）
- 修改：`client/aly-client/ui/winui/app/app.go`（下载/应用编排）

- [ ] **步骤 1：实现询问对话框**

在 `window.go` 追加：

```go
// AskYesNo 在 UI 线程弹出一个模态确认框，阻塞直到用户选择。
func (w *Window) AskYesNo(title, text string) bool {
	var yes bool
	w.Synchronize(func() {
		res := walk.MsgBox(w.mw, title, text, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)
		yes = res == walk.DlgCmdYes
	})
	return yes
}
```

- [ ] **步骤 2：编排下载与应用**

在 `app.go` 的 `Run` 中，把"有更新"分支替换为：

```go
	// 有更新
	w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.Fmt(i18n.KeyFoundVersion, cv.NewVersion))

	if dec.Action == flow.ActionDownload {
		if dec.Ask && !w.AskYesNo("aly-client-ui", i18n.Fmt(i18n.KeyAskDownload, cv.NewVersion)) {
			return 0 // 用户拒绝
		}
		w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.T(i18n.KeyDownloading))
		_, err := cli.DownloadUpdate(ctx, func(index, total int, file string) {
			w.SetProgress(index, total)
			w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.Fmt(i18n.KeyDownloading, file, index, total))
		})
		if err != nil {
			w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.T(i18n.KeyErrorGeneric))
			w.SetDetail(err.Error())
			return w.Run()
		}
		w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.T(i18n.KeyDownloaded))
	}

	// 应用
	if dec.Ask && !w.AskYesNo("aly-client-ui", i18n.Fmt(i18n.KeyAskApply, cv.NewVersion)) {
		return w.Run() // 用户选择稍后：窗口留着
	}
	w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.T(i18n.KeyApplying))
	if _, err := cli.ApplyUpdate(ctx); err != nil {
		w.SetContent("aly-client-ui", "v"+cv.NewVersion, i18n.T(i18n.KeyErrorGeneric))
		w.SetDetail(err.Error())
	}
	return w.Run()
```

> 注意：`apply_update` 成功后 `aly-client.exe` 会 `launchMainExe` 重启主程序，而主程序可能再拉起一个 UI —— 单实例由任务 16 处理。

- [ ] **步骤 3：补 `SetProgress` / `SetDetail`**

在 `window.go` 追加：

```go
// SetProgress 更新进度条（0..total）。
func (w *Window) SetProgress(cur, total int) {
	if w.progress != nil {
		w.progress.SetProgress(cur, total)
	}
}

// SetDetail 记录错误详情（原文，见 DEVELOPMENT_PLAN §3.3 的中文 errorMsg 策略）。
func (w *Window) SetDetail(s string) {
	w.detail = s
	// 以日志留痕；UI 上以可展开形式展示（M3 完善）
	appendLog(w.exeDir, "aly-client-ui.log", s)
}
```

`appendLog` 放在 `app/log.go`（新建），用 `ioutil` + `os.OpenFile` 实现（Go 1.10 兼容）。

- [ ] **步骤 4：手工验收（非强制 / 强制 两条路径）**

| 场景 | 期望 |
|------|------|
| `force_update=false` | 下载前弹确认；应用前弹确认；拒绝则不下载/不应用 |
| `force_update=true` | **不弹**任何确认，直接下载→应用 |
| `force_update` 缺失（downloaded 续跑） | 应用前**弹**确认 |
| `applying`（崩溃恢复现场） | **不弹**确认，自动续跑完成 |

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/app client/aly-client/ui/winui/view/window.go
git commit -m "feat(ui): 下载/应用编排与询问对话框（强制自动、非强制询问、applying 自动续跑）"
```

---

## 任务 15：历史版本列表与回滚（M4）

**文件：**
- 创建：`client/aly-client/ui/winui/view/rollback_dialog.go`
- 修改：`client/aly-client/ui/winui/app/app.go`

- [ ] **步骤 1：实现回滚列表对话框**

创建 `client/aly-client/ui/winui/view/rollback_dialog.go`：

```go
// +build windows

package view

import (
	"github.com/lxn/walk"

	"aly/client/aly-client/ui/winui/i18n"
)

// ChooseVersion 弹出模态列表让用户选择要回滚到的版本。返回空串表示取消。
func (w *Window) ChooseVersion(current string, versions []string) string {
	chosen := ""
	w.Synchronize(func() {
		dlg, err := walk.NewDialog(w.mw)
		if err != nil {
			return
		}
		defer dlg.Dispose()
		dlg.SetTitle(i18n.T(i18n.KeyVersionList))
		_ = dlg.SetLayout(walk.NewVBoxLayout())

		lb, err := walk.NewListBox(dlg)
		if err != nil {
			return
		}
		// ListBox.SetModel(mdl interface{}) error —— 直接传 []string（已在钉选版本核对）
		if err := lb.SetModel(versions); err != nil {
			return
		}
		if len(versions) > 0 {
			_ = lb.SetCurrentIndex(0)
		}

		btn, err := walk.NewPushButton(dlg)
		if err != nil {
			return
		}
		btn.SetText(i18n.T(i18n.KeyRollback))
		btn.Clicked().Attach(func() {
			i := lb.CurrentIndex()
			if i >= 0 && i < len(versions) {
				chosen = versions[i]
			}
			dlg.Accept()
		})

		_ = dlg.SetBounds(walk.Rectangle{X: 300, Y: 200, Width: 280, Height: 260})
		dlg.Run()
	})
	return chosen
}
```

- [ ] **步骤 2：在 `app.Run` 增加 rollback 模式分支**

```go
	if mode == ModeRoll {
		w.Show()
		res, err := cli.ListRollbackVersions(ctx)
		if err != nil || res.Versions == nil {
			w.SetContent("aly-client-ui", "", i18n.T(i18n.KeyErrorGeneric))
			return w.Run()
		}
		v := w.ChooseVersion(res.Versions.CurrentVersion, res.Versions.Versions)
		if v == "" {
			return 0
		}
		w.SetContent("aly-client-ui", "v"+v, i18n.T(i18n.KeyApplying))
		if _, err := cli.Rollback(ctx, v); err != nil {
			w.SetContent("aly-client-ui", "v"+v, i18n.T(i18n.KeyErrorGeneric))
		}
		return w.Run()
	}
```

- [ ] **步骤 3：手工验收**

| 场景 | 期望 |
|------|------|
| `-mode rollback` | 列出 `list_rollback_versions` 返回的版本；当前版本不在列表里 |
| 选择某版本 | 调用 `rollback --version <v>`，成功后主程序被重启 |
| 取消 | 不执行任何操作 |

- [ ] **步骤 4：Commit**

```bash
git add client/aly-client/ui/winui/view/rollback_dialog.go client/aly-client/ui/winui/app/app.go
git commit -m "feat(ui): 历史版本列表与回滚（M4）"
```

---

## 任务 16：UI 自更新 + 单实例（M5）

**文件：**
- 创建：`client/aly-client/ui/winui/app/selfupdate.go`
- 创建：`client/aly-client/ui/winui/app/single.go`
- 修改：`client/aly-client/ui/winui/app/app.go`

- [ ] **步骤 1：实现 `aly-client.exe` 自更新**

创建 `client/aly-client/ui/winui/app/selfupdate.go`：

```go
// +build windows

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// UpdateUpdater 对齐 C# SDK 的 UpdateSelf()：
// 把 主程序目录/aly-client.exe 复制覆盖 UpdateFolder/aly-client.exe。
// 失败时保留旧文件、不清空目标（SDK bug #8 的语义）。
func UpdateUpdater(updateFolder, mainFolder string) error {
	src := filepath.Join(mainFolder, "aly-client.exe")
	dst := filepath.Join(updateFolder, "aly-client.exe")
	return replaceFromSource(src, dst, 3, time.Second)
}

// UpdateSelfUI 把 主程序目录/aly-client-ui.exe 复制到 UpdateFolder/aly-client-ui.exe.new，
// 并派生一个 detached cmd 在自身退出后完成替换与重启（见 DEVELOPMENT_PLAN §6.3B）。
func UpdateSelfUI(updateFolder, mainFolder string, argv []string) (bool, error) {
	src := filepath.Join(mainFolder, "aly-client-ui.exe")
	self := filepath.Join(updateFolder, "aly-client-ui.exe")

	same, err := sameFile(src, self)
	if err != nil {
		return false, err
	}
	if same {
		return false, nil
	}

	newPath := self + ".new"
	if err := copyFile(src, newPath); err != nil {
		return false, err
	}

	// 延迟替换 + 重启
	cmdLine := fmt.Sprintf(
		`cmd /c ping -n 3 127.0.0.1 >nul & move /y "%s" "%s" & start "" "%s"`,
		newPath, self, self)
	return true, spawnDetached(cmdLine)
}

func sameFile(a, b string) (bool, error) {
	ha, err := sha256Of(a)
	if err != nil {
		return false, err
	}
	hb, err := sha256Of(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

func sha256Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0755)
}

// replaceFromSource 复制 src → dst.tmp → 校验 → 原子替换；失败重试 n 次。
func replaceFromSource(src, dst string, retries int, delay time.Duration) error {
	var lastErr error
	tmp := dst + ".new"
	for i := 0; i < retries; i++ {
		if err := copyFile(src, tmp); err != nil {
			lastErr = err
			time.Sleep(delay)
			continue
		}
		same, err := sameFile(src, tmp)
		if err != nil || !same {
			lastErr = fmt.Errorf("checksum mismatch after copy")
			_ = os.Remove(tmp)
			time.Sleep(delay)
			continue
		}
		// Go 1.10 无 os.Rename 覆盖已存在文件的保证（Windows 上会失败），
		// 用 MoveFileEx 语义：先删目标再改名，失败则保留旧文件。
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			lastErr = err
			_ = os.Remove(tmp)
			time.Sleep(delay)
			continue
		}
		if err := os.Rename(tmp, dst); err != nil {
			lastErr = err
			time.Sleep(delay)
			continue
		}
		return nil
	}
	return fmt.Errorf("replace %s: %v", dst, lastErr)
}
```

创建 `client/aly-client/ui/winui/app/spawn_windows.go`：

```go
// +build windows

package app

import (
	"os/exec"
	"syscall"
)

// spawnDetached 启动一个不继承句柄、不显示窗口的 cmd 进程。
func spawnDetached(cmdLine string) error {
	c := exec.Command("cmd.exe", "/c", cmdLine)
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x08000000, // DETACHED_PROCESS | CREATE_NO_WINDOW
	}
	return c.Start()
}
```

- [ ] **步骤 2：实现单实例互斥体**

创建 `client/aly-client/ui/winui/app/single.go`：

```go
// +build windows

package app

import (
	"syscall"
	"unsafe"
)

const (
	errorAlreadyExists = 183
	muName             = `Global\AlyClientUI_SingleInstance`
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	procCreateMu = kernel32.NewProc("CreateMutexW")
	procCloseMu  = kernel32.NewProc("CloseHandle")
)

// AcquireSingleInstance 返回 (是否首个实例, 释放函数)。
func AcquireSingleInstance() (bool, func()) {
	name, _ := syscall.UTF16PtrFromString(muName)
	h, _, err := procCreateMu.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true, func() {} // 拿不到互斥体时不阻断启动
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errorAlreadyExists {
		procCloseMu.Call(h)
		return false, func() {}
	}
	return true, func() { procCloseMu.Call(h) }
}
```

- [ ] **步骤 3：在 `main.go` 接入**

```go
	first, release := app.AcquireSingleInstance()
	if !first {
		// 已有实例：本实例直接退出（后续可改为把请求转给已有实例）
		os.Exit(0)
	}
	defer release()

	// 启动时自更新（在首次 check 之前，对齐 SDK 构造函数顺序）
	updateFolder := filepath.Dir(os.Args[0])
	if mf, err := app.MainFolder(updateFolder, mrp); err == nil {
		_ = app.UpdateUpdater(updateFolder, mf)
		if replaced, err := app.UpdateSelfUI(updateFolder, mf, os.Args); err == nil && replaced {
			os.Exit(0) // 交给延迟替换脚本重启
		}
	}
```

- [ ] **步骤 4：手工验收**

| 场景 | 期望 |
|------|------|
| 双击启动两次 | 只有**一个** UI 实例/窗口 |
| 主程序目录放了新版 `aly-client-ui.exe` | 启动时复制 `.new`，旧实例退出，新实例被拉起，版本一致 |
| 主程序目录放了新版 `aly-client.exe` | `UpdateFolder/aly-client.exe` 被更新（SHA256 一致） |
| 复制失败（目标被占用） | **保留旧文件**，日志留痕，不崩溃 |

- [ ] **步骤 5：Commit**

```bash
git add client/aly-client/ui/winui/app
git commit -m "feat(ui): UI/更新器自更新与单实例互斥（M5）"
```

---

## 任务 17：XP 真机复测与主程序联调（M5 收尾）

**文件：**
- 修改：`client/aly-client/ui/winui/DEVELOPMENT_PLAN.md`（回填验收结果）

- [ ] **步骤 1：XP SP3 x86 虚拟机全量复测**

逐项确认并记录：

| # | 检查 | 期望 |
|---|------|------|
| 1 | 启动（`-mode auto`，无更新） | 无窗口、无错误、进程干净退出 |
| 2 | 启动（`-mode auto`，有更新） | 右下角 topmost 显示，不抢焦点，无任务栏按钮 |
| 3 | 手动检查（`-mode check`，无更新） | 显示"已是最新版本"，5s 自动关闭 |
| 4 | 手动检查（有更新） | 显示更新界面，不自动关闭 |
| 5 | 非强制下载/应用 | 两次确认框；拒绝则不动 |
| 6 | 强制更新 | 全自动，无确认 |
| 7 | 更新中断后重启 | `applying` 自动续跑，不询问 |
| 8 | 回滚 | 列表正确、回滚后主程序被重启 |
| 9 | 386 线程栈 | 长时间运行/大量进度行不崩溃（若崩溃，加 `import _ "runtime/cgo"` 或 `-linkmode internal` 重测） |
| 10 | manifest | 控件有 XP 主题，无 SxS 错误 |

- [ ] **步骤 2：与主程序联调**

主程序零改动，仅把启动 `aly-client.exe` 改为启动 `..\UpdateFolder\aly-client-ui.exe -mode auto -lang zh-CN`。

验收：主程序启动 → 有更新时右下角出现 UI → 下载 → 应用 → 主程序被重启为新版本。

- [ ] **步骤 3：回填验收结果并 Commit**

```bash
git add client/aly-client/ui/winui/DEVELOPMENT_PLAN.md
git commit -m "docs(ui): 回填 XP 真机与主程序联调验收结果"
```

---

## 附录 A：自检

**1. 规格覆盖度**（对照 `DEVELOPMENT_PLAN.md`）

| 规格章节 | 对应任务 |
|----------|----------|
| §1.3 前置任务 | 任务 0 |
| §2 架构/目录 | 任务 1、5 |
| §3.0 部署布局 | 任务 5、16 |
| §3.1 启动参数 | 任务 11、13 |
| §3.2 子进程调用 | 任务 9 |
| §3.3 输出/退出码/超时 | 任务 8、9 |
| §3.4 命令映射 | 任务 9 |
| §3.5 `force_update` 缺失 | 任务 10、14 |
| §4.1 右下角/topmost/按需显示/5s | 任务 2、3、11 |
| §4.2 现代化自绘/字体/配色 | 任务 12 |
| §4.3 状态表 | 任务 13、14 |
| §4.4 线程模型 | 任务 3（`Synchronize`）、11 |
| §4.5 i18n | 任务 6 |
| §4.6 明确不做 | 任务 12（直角、无深色） |
| §5.1 依赖钉选 | 任务 1 |
| §5.2 manifest | 任务 4 |
| §5.3 构建 | 任务 0、1、5 |
| §5.4 产物校验 | 任务 0 步骤 5、任务 4 |
| §6 UI 自更新 | 任务 16 |
| §7 更新行为 | 任务 10、13、14 |
| §8 里程碑 M0–M5 | 任务 1–4（M0）、5–9（M1）、10（M2）、11–14（M3）、15（M4）、16–17（M5） |
| §8 单实例 | 任务 16 |

**2. 占位符扫描**：已检查——无 "TODO"/"待定"/"后续实现"。任务 12、13 中标有 ⚠️ 的 `walk.Canvas` API 名称需按钉选版本核对，已明确要求"编译不过就按 walk 源码调整，不要留占位"。

**3. 类型一致性**：`CheckView.ForceUpdate` 是 `*bool`，任务 8 的解析、任务 10 的 `Decide`、任务 13 的 `toCheckView` 三处一致；`Kind`/`Parsed`/`Options` 在任务 8、9 定义并在 9、13 使用；`Status`/`Action`/`Decision` 在任务 10 定义并在 13、14 使用。

---

## 附录 B：已在钉选版本核对过的 walk API

> 以下签名全部取自 `walk@bf6b8a4b207d83866fe5cef6ad09cd4fccc6d65c` 源码，**可直接使用**。
> 实现时若需其它 API，先 `grep -n "func 名字" <vendor>/github.com/lxn/walk/*.go` 核对，再落代码。

**窗口 / 窗体**
```go
func NewMainWindow() (*MainWindow, error)
func (wb *WindowBase) SetBounds(bounds Rectangle) error      // 注意：不是 SetBoundsPixels 也可用
func (wb *WindowBase) SetBoundsPixels(bounds Rectangle) error
func (wb *WindowBase) SetSize(size Size) error
func (wb *WindowBase) SetVisible(visible bool)
func (fb *FormBase) SetTitle(value string) error
func (fb *FormBase) Run() int
func (wb *WindowBase) Synchronize(f func())                   // 跨线程回 UI 线程
func (wb *WindowBase) ClientBounds() Rectangle
```

**自绘（CustomWidget）**
```go
type PaintFunc func(canvas *Canvas, updateBounds Rectangle) error
func NewCustomWidget(parent Container, style uint, paint PaintFunc) (*CustomWidget, error)
func (cw *CustomWidget) SetPaintMode(value PaintMode)         // PaintNormal|PaintNoErase|PaintBuffered
func (cw *CustomWidget) SetInvalidatesOnResize(value bool)
```

**Canvas 绘制原语**
```go
func (c *Canvas) FillRectangle(brush Brush, bounds Rectangle) error
func (c *Canvas) DrawRectangle(pen Pen, bounds Rectangle) error
func (c *Canvas) FillRoundedRectangle(brush Brush, bounds Rectangle, ellipseSize Size) error
func (c *Canvas) DrawRoundedRectangle(pen Pen, bounds Rectangle, ellipseSize Size) error
func (c *Canvas) FillEllipse(brush Brush, bounds Rectangle) error
func (c *Canvas) DrawEllipse(pen Pen, bounds Rectangle) error
func (c *Canvas) DrawLine(pen Pen, from, to Point) error
func (c *Canvas) DrawText(text string, font *Font, color Color, bounds Rectangle, format DrawTextFormat) error
func (c *Canvas) MeasureText(text string, font *Font, bounds Rectangle, format DrawTextFormat) (Rectangle, int, error)
```

**画刷 / 画笔 / 字体 / 颜色**
```go
func NewSolidColorBrush(color Color) (*SolidColorBrush, error)
func NewCosmeticPen(style PenStyle, color Color) (*CosmeticPen, error)
func NewFont(family string, pointSize int, style FontStyle) (*Font, error)
func RGB(r, g, b byte) Color
const PenSolid PenStyle = ...
const FontBold FontStyle = 0x01
```

**DrawTextFormat 常量**：`TextLeft`、`TextRight`、`TextHCenter`、`TextVCenter`、`TextTop`、`TextBottom`、`TextSingleLine`（另有 `TextWordBreak` 等）。

**对话框 / 控件 / 布局**
```go
func MsgBox(owner Form, title, message string, style MsgBoxStyle) int
const MsgBoxYesNo MsgBoxStyle = ...
const DlgCmdYes = ...
func NewDialog(owner Form) (*Dialog, error)
func (dlg *Dialog) Run() int
func NewVBoxLayout() *BoxLayout
func NewListBox(parent Container) (*ListBox, error)
func (lb *ListBox) SetModel(mdl interface{}) error            // 可直接传 []string
func (lb *ListBox) CurrentIndex() int
func (lb *ListBox) SetCurrentIndex(value int) error
```

**`lxn/win` 中已确认存在**（用于右下角 / topmost / 工具窗口）：
```go
func MonitorFromWindow(hwnd HWND, dwFlags uint32) HMONITOR
func GetMonitorInfo(hMonitor HMONITOR, lpmi *MONITORINFO) bool   // MONITORINFO.RcWork 为工作区
func SetWindowPos(hWnd, hWndInsertAfter HWND, x, y, width, height int32, flags uint32) bool
func GetWindowLong(hWnd HWND, index int32) int32
func SetWindowLong(hWnd HWND, index, value int32) int32
func AnimateWindow(hwnd HWND, dwTime uint32, dwFlags uint32) bool
const HWND_TOPMOST, SWP_NOMOVE, SWP_NOSIZE, SWP_NOACTIVATE, SWP_SHOWWINDOW
const GWL_EXSTYLE, GWL_STYLE, WS_EX_TOOLWINDOW, WS_EX_TOPMOST, WS_EX_LAYERED, WS_POPUP, WS_CAPTION
const MONITOR_DEFAULTTONEAREST = 0x2
```

**`lxn/win` 中确认【不存在】**（不要直接调用；需要时自行 `syscall.NewLazyDLL` 声明）：
`SPI_GETWORKAREA`、`SetWindowRgn`、`CreateRoundRectRgn`、`SetLayeredWindowAttributes`、`MonitorFromPoint`。

---

## 附录 C：执行方式

计划已完成并保存到 `client/aly-client/ui/winui/IMPLEMENTATION_PLAN.md`。两种执行方式：

**1. 子代理驱动（推荐）** —— 每个任务调度一个新的子代理，任务间进行审查，快速迭代。
必需子技能：`subagent-driven-development`。

**2. 内联执行** —— 在当前会话中使用 `executing-plans` 执行任务，批量执行并设有检查点。
必需子技能：`executing-plans`。

> 按 AGENTS.md 规则五：每个任务完成后需先编译，再用技能 `open-code-review`（`--concurrency 16`）审查本次改动，修复后提交。
