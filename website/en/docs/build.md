---
title: Build & Cross-Platform
description: Toolchains and commands for building the server, the publishing tools, the XP-compatible updater and the Linux GUI from source.
---

# Build & Cross-Platform

## Server and Publish CLI (modern Go)

```bash
# Server
cd server && go build -ldflags="-s -w" -o aly-server.exe .

# Publish CLI
cd publish/publish-cli && go build -ldflags="-s -w" -o aly-publish.exe .

# Static cross-compile
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aly-server-linux-amd64 .
```

Publish GUI (.NET 8 / Avalonia 12.1):

```bash
cd publish/publish-gui-csharp
dotnet build src/AlyPublish/AlyPublish.csproj
dotnet test tests/AlyPublish.Tests/AlyPublish.Tests.csproj
```

## XP Track: Updater + Windows GUI

Requires **Go 1.10.8** (the XP compatibility ceiling, no Go Modules), with the source under GOPATH:

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

::: warning Do not delete rsrc.syso
`aly-client-winui` is built on [lxn/walk](https://github.com/lxn/walk) and needs the ComCtl32 v6 manifest. The repository already includes `aly-client-winui/rsrc.syso`; deleting it makes the UI fail at startup with `TTM_ADDTOOL failed`.
:::

CI validates XP-track artifacts:

- PE header `machine = 0x014C` and `subsystem ≤ 5.0`
- GUI contains the `Microsoft.Windows.Common-Controls` manifest

## Modern Track: Cross-Platform Updater

Go 1.25+ can build the following targets (prebuilt releases cover only Windows/Linux):

| Target | Command |
|---|---|
| linux amd64 / arm64 | `GOOS=linux GOARCH=amd64 go build -o aly-client-linux-amd64 .` |
| darwin amd64 / arm64 | `GOOS=darwin GOARCH=arm64 go build -o aly-client-darwin-arm64 .` |
| windows amd64 / 386 | `GOOS=windows GOARCH=amd64 go build -o aly-client-windows-amd64.exe .` |
| freebsd amd64 | `GOOS=freebsd GOARCH=amd64 go build -o aly-client-freebsd-amd64 .` |

Platform differences are built in: executable file names (`.exe` on Windows), process enumeration and termination, and how a running exe is replaced.

## Linux GUI

Built on [MyGo](https://github.com/egoist/mygo) 0.8.3 (pure Go, no cgo) + Go 1.27.3, with GTK3 as the only runtime dependency:

```sh
cd client/aly-client-ui
./build-linux.sh              # linux/amd64 + linux/arm64: aly-client + aly-client-ui
./deploy-client-linux.sh <UpdateFolder> <MainFolder> [arch]
```

## One-Step Deployment

```bash
# Windows: build both executables and copy them back according to config.json
go run deploy/deploy-client.go

# Linux
./client/aly-client-ui/deploy-client-linux.sh
```