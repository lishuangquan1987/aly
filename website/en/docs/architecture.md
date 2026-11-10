# Architecture

## Big Picture

```text
        Publisher (developer / CI / release manager)              End-user machine
┌──────────────────────────────────────┐      ┌───────────────────────────────────┐
│  aly-publish-gui (Avalonia)          │      │  Host app your_app.exe            │
│      │ subprocess                    │      │      │ subprocess + JSON protocol │
│      ▼                               │      │      ▼                           │
│  aly-publish (cobra CLI) ────────────┼──┐   │  aly-client-ui (optional GUI)     │
│  config / status / add / push        │  │   │      │ delegates                  │
└──────────────────────────────────────┘  │   │  aly-client (the updater)         │
                                          ▼   │  check → download → apply        │
                              ┌──────────────┐│  self_update (closed loop)       │
                              │  aly-server  ││  ┌─────────────────────────────┐ │
                              │ Gin · Ent ·  │◄┼──┤ REST: check_update etc.     │ │
                              │ SQLite       ││  └─────────────────────────────┘ │
                              │ /api/*       ││                                  │
                              └──────────────┘│  PackageFolder/                  │
                                              │  ├ ApplicationFolder/  (current) │
                                              │  ├ ApplicationFolder_V1.0.0/(bak)│
                                              │  └ UpdateFolder/       (updater) │
                                              └───────────────────────────────────┘
```

## Data Flow: One Complete Update

1. **Publish**: the publish tool scans local diffs against the last release → uploads changed files (chunked for large ones) → `publish_version` registers the version and changelog.
2. **Check**: the client's `check_update` fetches `get_release_manifest`, compares against the local version, returns `has_update` / `force_update`.
3. **Download**: `download_update` pulls only differing files (MD5+SHA256 verified, resumable) and materializes unchanged files into `ApplicationFolder_{new_version}`.
4. **Apply**: `apply_update` closes host processes → three directory renames → launches the new main executable.
5. **Bookkeeping**: the `version.json` state machine spans the whole flow; a crash at any step is recovered on next startup.

## Dual-Track Toolchain: XP Compatibility Meets Cross-Platform

The client uses a **dual-track build** — the key to supporting Windows XP while covering modern platforms:

| Track | Toolchain | Artifacts | Targets |
|---|---|---|---|
| **XP track** | Go 1.10.8 · GOARCH=386 · GOPATH mode | `aly-client.exe` + `aly-client-ui.exe` (walk) | Windows XP → Win11 |
| **Modern track** | Go 1.25+ / 1.27.3 · Go Modules | `aly-client` (console) + `aly-client-ui` (MyGo/GTK3) | linux amd64/arm64 · darwin amd64/arm64 · freebsd amd64 |

Both tracks share one business codebase (`cmd/`, `selfupdate/`, `util/`); only the UI shells and build tags differ. Go officially dropped XP in 1.11, so the XP track must stay on 1.10.8 — CI validates XP artifacts (PE machine=0x014C, subsystem≤5.0) and the ComCtl32 manifest.

The modern-track Linux GUI (`aly-client-linuxui`) is built on MyGo — a pure-Go, cgo-free native UI library. Cross-compiling `GOARCH=arm64` just works; the only runtime dependency is GTK3.

## GUI–Updater Bridge

`aly-client-ui` implements no update logic at all — it is a thin presentation shell:

- Spawns `aly-client` subprocesses and parses stdout JSON;
- Shows a bottom-right topmost window with progress/pause/done on demand;
- Self-update at startup delegates to `aly-client self_update` (the updater itself takes the lock and performs the replacement); local copying remains only as a fallback when the updater is missing or outdated.

The C# SDK follows the same protocol — see [C# SDK](/en/docs/sdk).

## Repository Layout

```text
aly/
├── server/                 # aly-server (Go 1.25 · Gin · Ent · SQLite)
├── client/
│   ├── aly-client/         # The updater (shared by XP & modern tracks)
│   │   ├── cmd/            #   subcommand implementations
│   │   ├── selfupdate/     #   self-update orchestration package
│   │   ├── util/           #   process & file helpers (per-platform)
│   │   └── ui/winui/       #   Windows GUI (walk · vendored)
│   ├── aly-client-ui/
│   │   ├── aly-client-winui/   # XP-track Windows GUI entry
│   │   └── aly-client-linuxui/ # Linux-track GUI (MyGo · Go 1.27.3)
│   └── aly-client-sdk/     # C# SDK (net40 + netstandard2.0 · multi-platform NuGet targets)
├── publish/
│   ├── publish-cli/        # Publish CLI (cobra)
│   └── publish-gui/        # Publish desktop (Avalonia 11.3 · .NET 8)
├── deploy/                 # Deployment scripts (Windows/Linux)
├── test/tc/                # Go harness for the 135 E2E cases
└── website/                # This site (VitePress)
```
