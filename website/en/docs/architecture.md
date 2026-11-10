---
title: System Architecture
description: Aly's components, the data flow from publishing to application, the dual-track toolchain and how the GUI bridges to the updater.
---

# System Architecture

```text
Publisher (developer / CI / release manager)      End-user machine
┌────────────────────────────────────────┐      ┌────────────────────────────────────────────────────────────┐
│ AlyPublish.exe (Avalonia, optional)    │      │  Host application YourApp.exe                              │
│      │ subprocess + --json             │      │      │ subprocess / C# SDK / GUI                           │
│      ▼                                 │      │      ▼                                                     │
│ aly-publish (cobra CLI)────────────────┼──┐   │  aly-client-ui (optional window)                           │
│ config / status / add / push           │  │   │      │ delegation                                          │
└────────────────────────────────────────┘  │   │  aly-client (the updater)                                  │
                                            ▼   │  check → download → apply                                  │
                            ┌─────────────────┐ │  self_update (self-update loop)                            │
                            │  aly-server     │ │  ┌─────────────────────────────────────┐                   │
                            │ Gin · Ent ·     │◄┼──┤ REST: projects / files / manifests  │                   │
                            │ SQLite          │ │  └─────────────────────────────────────┘                   │
                            │ /api/*          │ │                                                            │
                            │ data/{project}/ │ │  PackageFolder/                                            │
                            └─────────────────┘ │  ├ ApplicationFolder/ (current)                            │
                                                │  ├ ApplicationFolder_V1.0.0/ (snapshot)                    │
                                                │  └ UpdateFolder/ (updater)                                 │
                                                └────────────────────────────────────────────────────────────┘
```

## Data Flow of One Complete Update

1. **Publish**: `aly-publish` scans the output directory and compares it with the server manifest → uploads new and changed files (large files in chunks) → `publish_version` registers the version and changelog, and writes the current file manifest to `data/{project}/.updator_manifest.json`.
2. **Check**: the client's `check_update` calls `get_project_by_name` and `get_project_change_logs`, compares the latest version by semver, and returns `has_update` / `force_update` / `release_notes`.
3. **Download**: `download_update` calls `get_all_files` to obtain the hash manifest, downloads only the differing files (MD5 + SHA256, `.part` resume) and materializes unchanged files into the new version directory; it then uses `get_release_manifest` to synchronize deletions.
4. **Apply**: `apply_update` validates the output → closes `must_close_process_name` → three directory renames → launches the main program.
5. **Record**: the `version.json` state machine spans the whole flow; a crash at any step is recovered on the next startup.

## Dual-Track Toolchain

| Track | Toolchain | Artifacts | Targets |
|---|---|---|---|
| **XP track** | Go 1.10.8 · GOARCH=386 · GOPATH | `aly-client.exe` + `aly-client-ui.exe` (walk) | Windows XP → Win11 |
| **Modern track** | Go 1.25+ / 1.27.3 · Modules | Server, publish CLI, Linux updater | Windows / Linux / macOS servers and tools |

The XP track must stay pinned to Go 1.10.8 (XP support ended with Go 1.11); CI validates the PE header (machine=0x014C, subsystem≤5.0) and the ComCtl32 manifest.

The Linux GUI (`aly-client-linuxui`) is built on [MyGo](https://github.com/egoist/mygo) (a pure-Go, cgo-free native UI library) and depends only on GTK3 at runtime.

## GUI and Updater Bridge

`aly-client-ui` implements no update logic; it is only a presentation layer:

- it calls `aly-client` commands through a subprocess and parses stdout JSON;
- a bottom-right topmost window shows progress, pause and completion as needed;
- at startup it delegates to `aly-client self_update` (the updater takes the lock and performs the replacement), with a local fallback only when the updater is missing or outdated;
- single instance: a second instance forwards the "manual check" request to the existing instance and then exits.

The C# SDK and the GUI follow the same protocol; see [C# SDK](/en/docs/sdk).

## Publish GUI Languages

`AlyPublish` supports Chinese/English through Irihi.Lingua and JSON resources: the language can be switched in the toolbar, and `--lang zh-CN|en-US` selects the startup language. The choice is stored in `%LOCALAPPDATA%\AlyPublish\settings.json`.

## Repository Layout

```text
aly/
├── server/                     # aly-server (Go 1.25 · Gin · Ent · SQLite)
├── client/
│   ├── aly-client/             # the updater itself (shared by the XP and modern tracks)
│   │   ├── cmd/                #   individual subcommands
│   │   ├── selfupdate/         #   self-update orchestration
│   │   ├── util/               #   process and file utilities
│   │   └── test/e2e/           #   fake main / mock server
│   ├── aly-client-ui/
│   │   ├── aly-client-winui/   # XP-track Windows GUI (walk)
│   │   └── aly-client-linuxui/ # Linux GUI (MyGo)
│   └── aly-client-sdk/         # C# SDK (net40 + netstandard2.0)
├── publish/
│   ├── publish-cli/            # publish CLI (cobra)
│   └── publish-gui/            # publish desktop app (Avalonia 12.1 · .NET 8)
├── test/tc/                    # 135 end-to-end cases
├── deploy/                     # deployment scripts
└── website/                    # this site (VitePress)
```