---
title: Package layout & distribution
description: What is inside an AlyPublish package, the publish-cli and client rid mapping, how to deliver updaters for another platform, and the zip-does-not-keep-the-exec-bit trap.
---

# Package layout & distribution

One `AlyPublish` package carries the publish GUI, the publish CLI and the terminal updater. This page covers what is inside and how to ship those pieces; to get running instead, read [Publish your first release](/en/docs/start/publish).

## What the package contains

```text
AlyPublish(.exe)                        # the publish GUI
payload.json                            # platform and payload index: the GUI picks its builds from it
publish-cli/<rid>/aly-publish(.exe)     # the publish CLI
client/<rid>/aly-client(-ui)(.exe)      # the terminal updater and its front end
install.sh  uninstall.sh                # Linux packages only
```

`<rid>` is the runtime identifier:

| Directory | Values |
|---|---|
| `publish-cli/<rid>/` | `windows-amd64`, `linux-amd64`, `linux-arm64` |
| `client/<rid>/` | `windows-x86`, `linux-amd64`, `linux-arm64` |

- The GUI reads `cli_rid` / `client_rid` from `payload.json` and uses its own platform's builds — **never pick by hand**;
- When you initialize a project, or add the main executable in “Project settings”, the GUI copies the two files from `client/<rid>/` into the sibling `UpdateFolder/` and writes `client.json`.

## Delivering updaters for another platform

To prepare a Windows app's `UpdateFolder` on Linux, take the two files from `client/windows-x86/` in the package. Prebuilt updaters exist for two platforms only:

| Platform | Prebuild | Notes |
|---|---|---|
| Windows | `client/windows-x86/` | 32-bit build, Windows XP through Windows 11 (runs on 64-bit systems) |
| Linux | `client/linux-amd64`, `client/linux-arm64` | pick by architecture; the Linux GUI needs the distro's GTK3 |
| macOS / FreeBSD | none | the updater compiles from the same sources, no official prebuild |

::: warning zip does not keep the exec bit
Files taken out of a zip (`aly-client`, `aly-server`, `install.sh`) need `chmod 0755` before they run.
:::

## Server and SDK assets

Server binaries, the publish GUI installers and every language SDK live in Releases; the full list is in [Download & install](/en/docs/download).

SDKs do not download the updater: `UpdateFolder/` comes from the package above, or is placed by hand as described here. The SDK build-time targets only create the directory and write a default `client.json` when one is missing.
