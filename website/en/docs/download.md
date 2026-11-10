---
title: Download & Install
description: Release assets, platform matrix and install commands for every Aly component.
---

# Download & Install

All release assets live on GitHub Releases. The links below always point at the latest version; replace `<asset>` with a name from the table.

```text
https://github.com/lishuangquan1987/aly/releases/latest/download/<asset>
```

## Platform matrix

| Component | Windows | Linux | macOS | Notes |
|---|---|---|---|---|
| **aly-server** | `aly-server-windows-amd64.exe` | `aly-server-linux-amd64` · `aly-server-linux-arm64` | `aly-server-darwin-amd64` · `aly-server-darwin-arm64` | single binary, SQLite built in |
| **aly-publish** | `aly-publish-windows-amd64.exe` | `aly-publish-linux-amd64` · `aly-publish-linux-arm64` | `aly-publish-darwin-amd64` · `aly-publish-darwin-arm64` | publish CLI |
| **aly-client** | `aly-client.exe` (32-bit, XP+) | `aly-client-linux-amd64` · `aly-client-linux-arm64` | — | end-user updater |
| **aly-client-ui** | `aly-client-ui.exe` (32-bit, XP+) | `aly-client-ui-linux-amd64` · `aly-client-ui-linux-arm64` | — | optional GUI |
| **AlyPublish GUI** | `AlyPublish-Setup.exe` · `AlyPublish-win-x64.zip` | — | — | Windows x64; self-contained; the installer bundles `aly-publish.exe` |
| **C# SDK** | `AlyClient.CSharpSDK.*.nupkg` · `AlyClient.CSharpSDK-<version>.zip` | same | same | not on public NuGet; get it from the release or source |
| **Language SDKs** | `AlyClient.*SDK-<version>.zip` | same | same | one versioned zip per language; contains wheel / jar / `.crate` / source as applicable |

::: tip Platform boundary
Prebuilt clients exist for Windows and Linux only. macOS/FreeBSD updaters compile from the same source but are not published as binaries; the server and publisher do ship macOS binaries.
:::

## Download commands

Download every language SDK:

```bash
gh release download --pattern 'AlyClient.*SDK-*.zip'
```

::: code-group

```powershell [Windows]
# Server
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-windows-amd64.exe" -OutFile "aly-server.exe"

# Publisher
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-publish-windows-amd64.exe" -OutFile "aly-publish.exe"

# Updater + optional GUI
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client.exe" -OutFile "aly-client.exe"
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client-ui.exe" -OutFile "aly-client-ui.exe"

# Publish GUI installer (self-contained; bundles aly-publish.exe)
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-Setup.exe" -OutFile "AlyPublish-Setup.exe"
Start-Process ".\AlyPublish-Setup.exe"
```

```bash [Linux]
# Server (amd64; replace with arm64 if needed)
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server

# Publisher
curl -fL -o aly-publish "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-publish-linux-amd64"
chmod +x aly-publish

# Updater + optional GUI
curl -fL -o aly-client "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client-linux-amd64"
curl -fL -o aly-client-ui "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client-ui-linux-amd64"
chmod +x aly-client aly-client-ui
```

```bash [macOS]
# Server (Apple Silicon: darwin-arm64)
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-darwin-amd64"
chmod +x aly-server

# Publisher
curl -fL -o aly-publish "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-publish-darwin-amd64"
chmod +x aly-publish
```

:::

macOS binaries are unsigned. Gatekeeper may block the first run; allow it in System Settings → Privacy & Security.

## Next steps

- **Deploy the server**: [Server deployment](/en/docs/server/deploy)
- **Publish the first release**: [aly-publish](/en/docs/publish)
- **End-user layout**: [Updater overview](/en/docs/client/) and [Configuration](/en/docs/client/config)
- **Integrate a .NET host**: [C# SDK](/en/docs/sdk)

## Verification and versions

- Release assets do not include a separate checksum file; compare with `Get-FileHash` / `sha256sum` if the release page provides one.
- The version is the Git tag (e.g. `v1.2.3`). Server, publisher and client versions do not need to match; clients only accept a strictly higher server version.
- The C# SDK is not on public NuGet: reference `AlyClient.CSharpSDK.csproj` from source, or use a CI-built nupkg. A local `dotnet pack` does not inject the updater version; see [SDK integration](/en/docs/sdk).