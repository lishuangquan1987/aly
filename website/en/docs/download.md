---
title: Download & install
description: Every release asset at a glance — the AlyPublish package, the server binaries and the language SDKs — plus how versions and checks work.
---

# Download & install

This page is the **asset catalogue**: on the publishing side you install one AlyPublish package (GUI, CLI and updater ship together), the server is a separate single binary, and each language SDK is its own zip. The table below is **generated from the latest release**; click a file name to download it.

The install steps live in the getting-started path, one page each:

- Publishing side: [Publish your first release](/en/docs/start/publish) (Windows installer or portable zip, Linux zip + `install.sh`)
- Server: [Deploy the server](/en/docs/start/server) (`./aly-server -p 2000`)
- What each package directory holds: [Package layout & distribution](/en/docs/guide/package)

<ReleaseDownloads group="all" />

## Publishing side (AlyPublish)

On Windows use <ReleaseLink asset="AlyPublish-Setup.exe" /> (start-menu entry, installs to `Program Files\AlyPublish`) or <ReleaseLink asset="AlyPublish-win-x64.zip" /> (portable); on Linux use <ReleaseLink asset="AlyPublish-linux-x64.zip" /> or <ReleaseLink asset="AlyPublish-linux-arm64.zip" />.

Linux packages carry `install.sh` / `uninstall.sh`: `./install.sh` installs to `/opt/aly/AlyPublish` by default and `--prefix` overrides that; binaries taken out of a zip need `chmod 0755` first.

::: tip The publish GUI implementation
Releases ship the Go / MyGo build by default: no extra runtime on Windows, GTK3 (`libgtk-3.so.0`) on Linux. The historical C# / Avalonia build stays in the repository: select `gui=csharp` in the Release Build workflow (or set the repository variable `PUBLISH_GUI` to `csharp`) and it produces the **same** four asset names, with identical package layout and config formats.
:::

## Server (aly-server)

Assets are named `aly-server-{goos}-{goarch}`: Windows uses <ReleaseLink asset="aly-server-windows-amd64.exe" />, Linux uses <ReleaseLink asset="aly-server-linux-amd64" /> / <ReleaseLink asset="aly-server-linux-arm64" />, macOS uses <ReleaseLink asset="aly-server-darwin-amd64" /> / <ReleaseLink asset="aly-server-darwin-arm64" />. macOS binaries are unsigned, so Gatekeeper may block the first run — allow it in System Settings.

## Language SDKs

Every zip in the “Language SDKs” group above is a direct download: C# also ships a `.nupkg` (not on the public NuGet feed), each other language gets one versioned zip (wheel / jar / `.crate` / sources).

```bash
gh release download --pattern 'AlyClient.*SDK-*.zip'
```

Integration is covered in [C# SDK integration](/en/docs/sdk) and [Language SDKs](/en/docs/sdk/languages).

## Verification and versions

- Release assets have no separate checksum file; verify with `Get-FileHash` / `sha256sum` against the Release page.
- The version number is the Git tag (for example `v1.2.3`). Server, publish tool and client do not need matching versions, but a client only accepts a **strictly higher** server version.
- See [Documentation versions](/en/docs/versions) for when these links are regenerated.
