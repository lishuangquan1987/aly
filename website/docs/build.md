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

发布 GUI（.NET 8 / Avalonia 12.1）：

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

## 一键部署

```bash
# Windows：构建两件套并按 config.json 回拷（需 WSL + Docker 镜像 golang:1.10.8-stretch）
go run deploy/deploy-client.go

# 产物由镜像内的 Go 1.10.8 编译（windows/386），构建后校验 PE 头
# （Machine=0x014C、SubsystemVer=4.0）不通过即中止，不会回拷/部署

# Linux
./client/aly-client-ui/deploy-client-linux.sh
```