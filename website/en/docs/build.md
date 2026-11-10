# Build & Cross-Platform

Aly's client builds on **two tracks**: the XP track pins an old toolchain for compatibility, the modern track uses the latest Go for coverage. Both share one business codebase — only UI shells and build tags differ.

## aly-server & aly-publish (modern track)

```bash
# Server
cd server && go build -ldflags="-s -w" -o aly-server.exe .

# Publish CLI
cd publish/publish-cli && go build -ldflags="-s -w" -o aly-publish.exe .

# Cross-compile (static, no CGO)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aly-server-linux-amd64 .
```

Publish GUI (Avalonia):

```bash
cd publish/publish-gui && dotnet build
```

## XP Track: aly-client + Windows GUI

Requires **Go 1.10.8** (the last XP-capable release, no Go modules); source must live under GOPATH:

```bat
:: Copy sources into GOPATH
xcopy /e /i /y client\aly-client %GOPATH%\src\aly\client\aly-client
xcopy /e /i /y client\aly-client-ui\aly-client-winui %GOPATH%\src\aly\client\aly-client-ui\aly-client-winui

:: Build the console updater (32-bit, XP compatible)
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
set CGO_ENABLED=0
go build -ldflags="-s -w" -o aly-client.exe aly/client/aly-client

:: Build the optional Windows GUI (no console window)
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client-ui/aly-client-winui
```

::: warning Do not delete rsrc.syso
`aly-client-winui` is built on [lxn/walk](https://github.com/lxn/walk) and needs the ComCtl32 v6 manifest. The repository ships `aly-client-winui/rsrc.syso`, linked automatically by `go build`; deleting it makes the UI fail at startup with `TTM_ADDTOOL failed`.
:::

### CI Validation for XP Compatibility

Before every release, CI validates XP-track artifacts:

- PE header `machine = 0x014C` (i386) and `subsystem ≤ 5.0` (loadable on XP)
- GUI artifacts contain the `Microsoft.Windows.Common-Controls` manifest

## Modern Track: Cross-Platform Builds

The console updater builds with modern Go (1.25+); all of these targets are verified:

| Target | Command |
|---|---|
| linux amd64 / arm64 | `GOOS=linux GOARCH=amd64 go build -o aly-client-linux-amd64 .` |
| darwin amd64 / arm64 | `GOOS=darwin GOARCH=arm64 go build -o aly-client-darwin-arm64 .` |
| windows amd64 / 386 | `GOOS=windows GOARCH=amd64 go build -o aly-client-windows-amd64.exe .` |
| freebsd amd64 | `GOOS=freebsd GOARCH=amd64 go build -o aly-client-freebsd-amd64 .` |

Platform differences are handled inside the codebase:

- Executable naming: Windows `aly-client.exe` / other platforms `aly-client` (`config.UpdaterFileName()`)
- Process enumeration & termination: Windows Toolhelp API / Linux `/proc` + signals
- Deleting a running executable: Windows rename dance / Unix plain `os.Rename`

## Linux GUI (aly-client-linuxui)

Built on MyGo 0.8.3 (a pure-Go, cgo-free native UI library) + Go 1.27.3. The only runtime dependency is the GTK3 that desktop distros preinstall — no WebKitGTK, no Bun:

```sh
cd client/aly-client-ui
./build-linux.sh              # linux/amd64 + linux/arm64: aly-client + aly-client-ui
./deploy-client-linux.sh <UpdateFolder> <MainFolder> [arch]
```

Cross-compiling works out of the box (`GOARCH=arm64`); artifacts are single ELF files.

## One-Shot Deployment Scripts

```bash
# Windows: build both executables and copy them to every target listed in config.json
go run deploy/deploy-client.go

# Linux deployment
./client/aly-client-ui/deploy-client-linux.sh
```
