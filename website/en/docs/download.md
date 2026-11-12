---
title: Download & Install
description: One AlyPublish package gives you the publish GUI, the publish CLI and the terminal updater; this page covers the Windows installer, the Linux packages, the in-package payload layout and the server download.
---

# Download & Install

Both the updater and the publish CLI ship **inside the AlyPublish package**: install one package and you
have the publish GUI, `aly-publish`, `aly-client` and `aly-client-ui` ready. The server is a separate
download.

All assets live on GitHub Releases. The table below is **generated from the latest release** — click any
file name to download. Links and the version badge refresh automatically whenever a release is published
or the docs are re-deployed manually.

<ReleaseDownloads group="all" />

## Step 1: install AlyPublish (one package)

On Windows download <ReleaseLink asset="AlyPublish-Setup.exe" /> (installer: start-menu entry,
self-contained .NET 8, installs to `Program Files\AlyPublish`) or
<ReleaseLink asset="AlyPublish-win-x64.zip" /> (portable: unzip and run). On Linux download
<ReleaseLink asset="AlyPublish-linux-x64.zip" /> (x64) or
<ReleaseLink asset="AlyPublish-linux-arm64.zip" /> (arm64). The full matrix with file sizes is above.

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
./install.sh                                        # installs to /opt/aly/AlyPublish (needs write access)
# without root use: ./install.sh --prefix $HOME/.local/lib/AlyPublish
```

Start `AlyPublish` after install (on Linux it is the `AlyPublish` file inside the prefix; `install.sh`
also writes a launcher into `$HOME/.local/bin`), then follow [Quick Start](/en/docs/getting-started).

## Step 2: install the server

Server assets are named `aly-server-{goos}-{goarch}`: Windows uses
<ReleaseLink asset="aly-server-windows-amd64.exe" />, Linux uses
<ReleaseLink asset="aly-server-linux-amd64" /> / <ReleaseLink asset="aly-server-linux-arm64" />,
macOS uses <ReleaseLink asset="aly-server-darwin-amd64" /> /
<ReleaseLink asset="aly-server-darwin-arm64" />. All clickable links are in the table above.

```bash [Linux]
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server
./aly-server -p 2000
```

The first run creates `aly.db` and `data/` next to the executable. macOS binaries are unsigned;
Gatekeeper may block the first run — allow it in System Settings.

## The in-package payload: GUI, CLI, updater

After install (or after unzipping the portable package):

```text
AlyPublish(.exe)                        # publish GUI
payload.json                            # platform and payload index: the GUI picks its builds from it
publish-cli/<rid>/aly-publish(.exe)     # publish CLI: windows-amd64 | linux-amd64 | linux-arm64
client/<rid>/aly-client(-ui)(.exe)      # terminal updater and its front end: windows-x86 | linux-amd64 | linux-arm64
install.sh  uninstall.sh                # Linux packages only
```

- The GUI reads `cli_rid` / `client_rid` from `payload.json` and uses the builds of its own platform —
  you never pick by hand;
- When you initialize a project, or add the main executable in “Project settings”, the GUI copies the two
  updaters from `client/<rid>/` into the sibling `UpdateFolder/` and writes `client.json`;
- To deliver updaters for another platform (for example preparing a Windows app's `UpdateFolder` on
  Linux), take `client/windows-x86/` straight out of the package;
- Files taken out of a zip **must** be `chmod 0755`: zip does not carry the exec bit.

::: tip Platform boundaries of the updater
Prebuilt updaters exist for Windows (32-bit, XP through Win11) and Linux (amd64 / arm64). macOS and
FreeBSD updaters can be compiled from the same sources, but there is no official prebuild; the server and
the publish tool do have macOS prebuilds.
::

## Language SDKs

Every zip in the “Language SDKs” group above is a direct download link. C# also has a `.nupkg` (not on
the public NuGet feed); each other language ships one versioned zip (wheel / jar / `.crate` / sources).

```bash
gh release download --pattern 'AlyClient.*SDK-*.zip'
```

SDKs no longer download or copy the updater binaries: `UpdateFolder/` is produced by the AlyPublish GUI,
or you place the files by hand following the payload layout above. See [C# SDK integration](/en/docs/sdk)
and [language SDKs](/en/docs/sdk/languages).

## Verification and versions

- Release assets have no separate checksum file; verify with `Get-FileHash` / `sha256sum` against the
  Release page afterwards.
- The version number is the Git tag (for example `v1.2.3`). The server, the publish tool and the client
  do not need matching versions, but a client only accepts a strictly higher server version.
- See [Documentation Versions](/en/docs/versions) for when these links are regenerated.

## Next

- **Run it in three steps**: [Quick Start](/en/docs/getting-started)
- **Deploy the server**: [Server deployment](/en/docs/server/deploy)
- **Publish with the GUI**: [Publish GUI](/en/docs/publish/gui)
- **Integrate a host**: [C# SDK integration](/en/docs/sdk)
