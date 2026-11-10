# 构建与跨平台

Aly 客户端采用**双轨构建**：XP 轨锁定旧工具链保兼容，现代轨用最新 Go 保覆盖。两条轨共享同一份业务代码，差异只在 UI 壳与构建标签。

## aly-server 与 aly-publish（现代轨）

```bash
# 服务端
cd server && go build -ldflags="-s -w" -o aly-server.exe .

# 发布 CLI
cd publish/publish-cli && go build -ldflags="-s -w" -o aly-publish.exe .

# 交叉编译（静态、无 CGO）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aly-server-linux-amd64 .
```

发布 GUI（Avalonia）：

```bash
cd publish/publish-gui && dotnet build
```

## XP 轨：aly-client + Windows GUI

需要 **Go 1.10.8**（XP 兼容的上限版本，不使用 Go Modules），源码必须位于 GOPATH 下：

```bat
:: 拷贝源码到 GOPATH
xcopy /e /i /y client\aly-client %GOPATH%\src\aly\client\aly-client
xcopy /e /i /y client\aly-client-ui\aly-client-winui %GOPATH%\src\aly\client\aly-client-ui\aly-client-winui

:: 构建控制台更新器（32 位 XP 兼容）
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
set CGO_ENABLED=0
go build -ldflags="-s -w" -o aly-client.exe aly/client/aly-client

:: 构建可选 Windows GUI（无控制台窗口）
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client-ui/aly-client-winui
```

::: warning rsrc.syso 不要删
`aly-client-winui` 基于 [lxn/walk](https://github.com/lxn/walk)，需要 ComCtl32 v6 manifest。仓库已包含 `aly-client-winui/rsrc.syso`，`go build` 会自动链接；删除它会导致 UI 启动即报 `TTM_ADDTOOL failed`。
:::

### CI 的 XP 兼容校验

CI 在每次发布前对 XP 轨产物做自动校验：

- PE 头 `machine = 0x014C`（i386）且 `subsystem ≤ 5.0`（XP 可加载）
- GUI 产物包含 `Microsoft.Windows.Common-Controls` manifest

## 现代轨：跨平台构建

控制台更新器与现代 Go（1.25+）构建，以下目标全部实测可编译：

| 目标 | 命令 |
|---|---|
| linux amd64 / arm64 | `GOOS=linux GOARCH=amd64 go build -o aly-client-linux-amd64 .` |
| darwin amd64 / arm64 | `GOOS=darwin GOARCH=arm64 go build -o aly-client-darwin-arm64 .` |
| windows amd64 / 386 | `GOOS=windows GOARCH=amd64 go build -o aly-client-windows-amd64.exe .` |
| freebsd amd64 | `GOOS=freebsd GOARCH=amd64 go build -o aly-client-freebsd-amd64 .` |

平台差异已内置处理：

- 可执行文件命名：Windows `aly-client.exe` / 其余平台 `aly-client`（`config.UpdaterFileName()`）
- 进程枚举与关闭：Windows Toolhelp API / Linux `/proc` + 信号
- 删除运行中的 exe：Windows 改名舞步 / Unix 直接 `os.Rename`

## Linux GUI（aly-client-linuxui）

基于 MyGo 0.8.3（纯 Go、无 cgo 的原生 UI 库）+ Go 1.27.3，运行时仅依赖桌面发行版预装的 GTK3，不需要 WebKitGTK 或 Bun：

```sh
cd client/aly-client-ui
./build-linux.sh              # linux/amd64 + linux/arm64：aly-client + aly-client-ui
./deploy-client-linux.sh <UpdateFolder> <MainFolder> [arch]
```

交叉编译直接生效（`GOARCH=arm64`），产物是单个 ELF 文件。

## 一键部署脚本

```bash
# Windows：构建两件套并按 config.json 清单回拷到各目标
go run deploy/deploy-client.go

# Linux 部署
./client/aly-client-ui/deploy-client-linux.sh
```
