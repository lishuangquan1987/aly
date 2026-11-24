---
title: Download & install
description: Every release asset at a glance — the AlyPublish package, the server binaries and the language SDKs — plus the first step after installing.
---

# Download & install

On the publishing side you only need one **AlyPublish** package: the publish GUI, the publish CLI and the terminal updater ship together. The server is a separate single binary; each language SDK is its own zip.

The table below is **generated from the latest release**; click a file name to download it.

<ReleaseDownloads group="all" />

## Install the publishing side

On Windows use <ReleaseLink asset="AlyPublish-Setup.exe" /> (start-menu entry, installs to `Program Files\AlyPublish`) or <ReleaseLink asset="AlyPublish-win-x64.zip" /> (portable); on Linux use <ReleaseLink asset="AlyPublish-linux-x64.zip" /> or <ReleaseLink asset="AlyPublish-linux-arm64.zip" />.

::: code-group

```powershell [Windows]
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-Setup.exe" -OutFile "AlyPublish-Setup.exe"
Start-Process ".\AlyPublish-Setup.exe"
```

```bash [Linux]
curl -fL -o AlyPublish-linux-x64.zip "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-linux-x64.zip"
mkdir -p AlyPublish-linux-x64
unzip -q AlyPublish-linux-x64.zip -d AlyPublish-linux-x64   # the zip has no top-level folder
cd AlyPublish-linux-x64
chmod +x install.sh                                 # zip does not keep the exec bit
./install.sh                                        # installs to /opt/aly/AlyPublish
# without root: ./install.sh --prefix $HOME/.local/lib/AlyPublish
```

:::

Start `AlyPublish` afterwards and follow [Publish your first release](/en/docs/start/publish). What each directory of the package holds is in [Package layout & distribution](/en/docs/guide/package).

::: tip The publish GUI implementation
Releases ship the Go / MyGo build by default: no extra runtime on Windows, GTK3 (`libgtk-3.so.0`) on Linux. The historical C# / Avalonia build stays in the repository: select `gui=csharp` in the Release Build workflow (or set the repository variable `PUBLISH_GUI` to `csharp`) and it produces the **same** four asset names, with identical package layout and config formats.
:::

## Install the server

Server assets are named `aly-server-{goos}-{goarch}`: Windows uses <ReleaseLink asset="aly-server-windows-amd64.exe" />, Linux uses <ReleaseLink asset="aly-server-linux-amd64" /> / <ReleaseLink asset="aly-server-linux-arm64" />, macOS uses <ReleaseLink asset="aly-server-darwin-amd64" /> / <ReleaseLink asset="aly-server-darwin-arm64" />.

```bash
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server
./aly-server -p 2000
```

The first run creates `aly.db` and `data/` next to the executable. The shortest deployment path is [Deploy the server](/en/docs/start/server); macOS binaries are unsigned, so Gatekeeper may block the first run — allow it in System Settings.

## Get an SDK

Every zip in the “Language SDKs” group above is a direct download: C# also ships a `.nupkg` (not on the public NuGet feed), each other language gets one versioned zip (wheel / jar / `.crate` / sources).

```bash
gh release download --pattern 'AlyClient.*SDK-*.zip'
```

Integration is covered in [C# SDK integration](/en/docs/sdk) and [Language SDKs](/en/docs/sdk/languages).

## Verification and versions

- Release assets have no separate checksum file; verify with `Get-FileHash` / `sha256sum` against the Release page.
- The version number is the Git tag (for example `v1.2.3`). Server, publish tool and client do not need matching versions, but a client only accepts a **strictly higher** server version.
- See [Documentation versions](/en/docs/versions) for when these links are regenerated.

## Next

- **Get running now**: [Publish your first release](/en/docs/start/publish) · [Deploy the server](/en/docs/start/server)
- **Integrate a host**: [Integrate the updater](/en/docs/start/integrate)
- **Inside the package**: [Package layout & distribution](/en/docs/guide/package)
