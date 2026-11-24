---
title: 构建与跨平台
description: 从源码构建服务端、发布工具、XP 兼容更新器与 Linux GUI 的工具链与命令。
---

# 构建与跨平台

## 服务端与发布 CLI（现代 Go）

```bash
# 服务端
cd server && go build -ldflags="-s -w" -o aly-server.exe .

# 发布 CLI
cd publish/publish-cli && go build -ldflags="-s -w" -o aly-publish.exe .

# 静态交叉编译
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aly-server-linux-amd64 .
```

发布 GUI（默认 Go / MyGo 版：纯 Go、无 cgo，可交叉编译；Windows 无额外运行依赖，Linux 需要 GTK3）：

```bash
cd publish/publish-gui-go
go test ./... -count=1      # 无窗口视图测试
go tool mygo vet            # MyGo 静态检查
./build.sh                  # windows-amd64、linux-amd64、linux-arm64 三平台产物
# Windows 用 build.ps1
```

历史 C# / Avalonia 版（.NET 8；保留在仓库中，不再默认构建，Release Build 选 `gui=csharp` 可产出）：

```bash
cd publish/publish-gui-csharp
dotnet build src/AlyPublish/AlyPublish.csproj
dotnet test tests/AlyPublish.Tests/AlyPublish.Tests.csproj
```

## XP 轨：更新器 + Windows GUI

需要 **Go 1.10.8**（XP 兼容上限，不使用 Go Modules），源码位于 GOPATH 下：

```bat
xcopy /e /i /y client\aly-client %GOPATH%\src\aly\client\aly-client
xcopy /e /i /y client\aly-client-ui\aly-client-winui %GOPATH%\src\aly\client\aly-client-ui\aly-client-winui

set GOOS=windows
set GOARCH=386
set GO111MODULE=off
set CGO_ENABLED=0
go build -ldflags="-s -w" -o aly-client.exe aly/client/aly-client
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client-ui/aly-client-winui
```

::: warning rsrc.syso 不要删
`aly-client-winui` 基于 [lxn/walk](https://github.com/lxn/walk)，需要 ComCtl32 v6 manifest。仓库已包含 `aly-client-winui/rsrc.syso`，删除后 UI 启动会报 `TTM_ADDTOOL failed`。
:::

CI 对 XP 轨产物校验：

- PE 头 `machine = 0x014C` 且 `subsystem ≤ 5.0`
- GUI 包含 `Microsoft.Windows.Common-Controls` manifest

## 现代轨：跨平台更新器

Go 1.25+ 可编译以下目标（预构建只发布 Windows/Linux）：

| 目标 | 命令 |
|---|---|
| linux amd64 / arm64 | `GOOS=linux GOARCH=amd64 go build -o aly-client-linux-amd64 .` |
| darwin amd64 / arm64 | `GOOS=darwin GOARCH=arm64 go build -o aly-client-darwin-arm64 .` |
| windows amd64 / 386 | `GOOS=windows GOARCH=amd64 go build -o aly-client-windows-amd64.exe .` |
| freebsd amd64 | `GOOS=freebsd GOARCH=amd64 go build -o aly-client-freebsd-amd64 .` |

平台差异已内置：可执行文件名（Windows 带 `.exe`）、进程枚举/关闭、运行中 exe 的替换方式。

## Linux GUI

基于 [MyGo](https://github.com/egoist/mygo) 0.8.3（纯 Go、无 cgo）+ Go 1.27.3，运行时仅依赖 GTK3：

```sh
cd client/aly-client-ui
./build-linux.sh              # linux/amd64 + linux/arm64：aly-client + aly-client-ui
./deploy-client-linux.sh <UpdateFolder> <MainFolder> [arch]
```

## 发布 GUI 的打包与分发

Release 工作流会构建并上传 **4 个 GUI 资产**（见 `.github/workflows/release.yml`）：

- `AlyPublish-Setup.exe`：Windows x64 安装包，内含 GUI 本体、`publish-cli/`、`client/`、`payload.json`；
- `AlyPublish-win-x64.zip`：Windows 便携包，解压即用；
- `AlyPublish-linux-x64.zip`、`AlyPublish-linux-arm64.zip`：Linux 安装包压缩包，包内另有 `install.sh` / `uninstall.sh`；解压后先 `chmod +x install.sh`（zip 不保留可执行位）再 `./install.sh` 装到 `/opt/aly/AlyPublish`（`--prefix` 可改；`install.sh` 会替 payload 里的可执行文件补齐 0755）。

发布 CLI 与终端更新器**不单独上传资产**：它们只随 GUI 包分发。payload 铺装由 release 工作流的内联 pwsh 步骤（`Stage the package payload`）完成——按各平台产物铺成上面的布局、拷入对应 `payload.json`、断言 10 项（9 个二进制 + 清单）全部到位，缺任一即构建失败；本地开发走 `publish/deploy-publish.bat`。

从源码构建 `publish-gui-go`：

```bash
cd publish/publish-gui-go
go test ./... -count=1
go tool mygo vet

# Windows 单文件（-H windowsgui 去掉控制台窗口）
go build -trimpath -ldflags "-H windowsgui" -o build/windows-amd64/AlyPublish.exe .

# Linux 交叉编译（CI 在此之上再加 install.sh 并压 zip）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o build/linux-amd64/AlyPublish .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o build/linux-arm64/AlyPublish .
```

（或直接跑 `build.ps1` / `build.sh`：一次跑 fmt/vet/test 并产出三平台二进制。）手工分发时把 GUI 本体与 `payload.json`、`publish-cli/`、`client/` 放同一目录即可。历史 C# 版：`cd publish/publish-gui-csharp && dotnet build`。

## 一键部署

```bash
# Windows：构建两件套并按 config.json 回拷（需 WSL + Docker 镜像 golang:1.10.8-stretch）
go run deploy/deploy-client.go

# 产物由镜像内的 Go 1.10.8 编译（windows/386），构建后校验 PE 头
# （Machine=0x014C、SubsystemVer=4.0）不通过即中止，不会回拷/部署

# Linux
./client/aly-client-ui/deploy-client-linux.sh
```