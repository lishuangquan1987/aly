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

Build commands for the publish GUI (default Go / MyGo build) and the historical C# build, the payload staging and the release assets live in [Publishing the GUI](#publishing-the-gui).

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

## Publishing the GUI

The Release workflow builds and uploads **four GUI assets** (see `.github/workflows/release.yml`):

- `AlyPublish-Setup.exe`: a Windows x64 installer bundling the GUI, `publish-cli/`, `client/` and `payload.json`;
- `AlyPublish-win-x64.zip`: a portable bundle that runs after extraction;
- `AlyPublish-linux-x64.zip`, `AlyPublish-linux-arm64.zip`: Linux packages (with `install.sh` / `uninstall.sh` inside); after extraction run `chmod +x install.sh` (the zip does not preserve the executable bit), then `./install.sh` to install into `/opt/aly/AlyPublish` (`--prefix` overrides; `install.sh` also restores the 0755 bits on the payload executables).

The publish CLI and the terminal updaters are **not** uploaded as standalone assets: they ship only inside the GUI package. The payload is staged by the inline pwsh step (`Stage the package payload`): it lays out the per-platform artifacts into the layout above, copies the platform's `payload.json`, and asserts all 10 items (9 binaries + the manifest) are present; for local development `publish/deploy-publish.bat` copies the same file names into the GUI output directory.

Building `publish-gui-go` from source:

```bash
cd publish/publish-gui-go
go test ./... -count=1
go tool mygo vet

# Windows single file (-H windowsgui removes the console window)
go build -trimpath -ldflags "-H windowsgui" -o build/windows-amd64/AlyPublish.exe .

# Linux cross-compile (CI adds install.sh and zips on top)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o build/linux-amd64/AlyPublish .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o build/linux-arm64/AlyPublish .
```

(Or run `build.ps1` / `build.sh`: one run of fmt/vet/test plus all three platform binaries.) When distributing manually, keep the GUI next to `payload.json`, `publish-cli/` and `client/` in one directory.

The historical C# / Avalonia build (.NET 8; kept in the repository, no longer built by default — select `gui=csharp` in Release Build to produce it):

```bash
cd publish/publish-gui-csharp
dotnet build src/AlyPublish/AlyPublish.csproj
dotnet test tests/AlyPublish.Tests/AlyPublish.Tests.csproj
```

## One-Step Deployment

```bash
# Windows: build both executables and copy them back according to config.json
# (requires WSL + the Docker image golang:1.10.8-stretch)
go run deploy/deploy-client.go

# The binaries are built by Go 1.10.8 inside the image (windows/386); the PE header
# is verified (Machine=0x014C, SubsystemVer=4.0) before anything is copied or deployed

# Linux
./client/aly-client-ui/deploy-client-linux.sh
```