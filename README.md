# Aly — Lightweight Client Auto-Update System

A minimal, self-contained auto-update solution for desktop applications. Ship new versions with a single command, and let your users update seamlessly with crash-safe atomic replacement.

## Project Structure

```
aly/
├── server/         # API server (Go, Gin + Ent + SQLite)
├── client/
│   └── aly-client/      # Client updater (Go 1.10, Windows XP compatible)
│   └── aly-client-sdk/  # (placeholder) Future Go/C# integration SDK
├── publish/
│   ├── publish-cli/      # CLI publish engine (Go, cobra)
│   └── publish-gui/  # Desktop GUI (Avalonia 11.3, .NET 8)
└── README.md
```

**Four modules:**

| Module | Role | For whom |
|--------|------|----------|
| **aly-server** | Stores project metadata & files, serves REST API | Deploy on your infrastructure |
| **aly-publish** | CLI tool to init configs, scan files, and publish versions | Developers / CI |
| **aly-publish-gui** | Desktop GUI wrapping aly-publish via subprocess | Non-technical publishers |
| **aly-client** | The updater executable — runs on end-user machines | End users (via your app) |
| **aly-client-ui** | Optional GUI front-end for aly-client (bottom-right, topmost, XP-compatible) | End users (via your app) |

## Quick Start

### 1. Start the Server

```bash
./aly-server -p 2000
```

First run auto-creates a SQLite database (`aly.db`). The server stores project info and uploaded files under `data/{project_name}/`.

### 2. Publish a New Version

```bash
# Initialize project config (creates .updator/ directory)
cd ./dist
aly-publish config init --server http://localhost:2000 --project myapp

# Check what files changed
aly-publish status

# Stage all changes and push
aly-publish add --all
aly-publish push --version V1.0.1 --message "Fixed login bug"
```

This scans the directory, uploads new/changed files to the server, and creates a version changelog entry.

### 3. Client: Check & Apply Updates

On the end-user's machine, your application invokes `aly-client.exe` (located in `UpdateFolder/`):

```bash
# Step 1 — Check for updates
aly-client.exe check_update
# → {"isSuccess":true,"data":{"has_update":true,"need_download_update":true,"new_version":"1.0.1","force_update":false}}

# Step 2 — Download (only downloads changed files, with SHA-256 verification)
aly-client.exe download_update

# Step 3 — Apply (closes your app, atomically replaces files, restarts)
aly-client.exe apply_update
```

If `force_update` is `true`, your app should auto-download and prompt a restart; otherwise, ask the user first.

### Deployment Layout (End-User Machine)

```
PackageFolder/
├── ApplicationFolder/            # Active app (current version)
│   ├── your_app.exe
│   └── .updator/shared.json     # server_url, project_name, ignore rules
├── ApplicationFolder_V1.0.0/     # Previous version snapshot (for rollback)
├── UpdateFolder/
│   ├── aly-client.exe            # The updater itself
│   ├── aly-client-ui.exe         # Optional GUI front-end (bottom-right, topmost)
│   ├── client.json               # main_exe_relative_path, must_close_process_name
│   └── version.json              # Version state machine
```

## Key Concepts

### `.updator/` — Shared Config Directory

Both `aly-publish` and `aly-client` read from `.updator/shared.json`:

```json
{
  "server_url": "http://10.0.0.1:2000",
  "project_name": "myapp",
  "ignore_folders": ["logs", "temp"],
  "ignore_files": ["*.log"]
}
```

- `ignore_folders` / `ignore_files`: applied during **scanning** (what gets uploaded) and **applying** (what gets skipped when copying files during version replacement).
- `shared.json` is uploaded with each release. When the client downloads an update, the new `shared.json` replaces the old one — so config changes roll out automatically.

### Version State Machine (`version.json`)

```
applied ──(download_update)──→ downloaded ──(apply_update starts)──→ applying
                  │                        │
                  │                        ├──(success)──→ applied
                  │                        └──(crash)──→ detected on next startup → auto-recover
                  │
                  └──(check_update)──→ has_update=true (server reachable)
```

`check_update` only reports a pending apply while the server is reachable: with
`version_status=downloaded` and an unreachable server (or no usable change logs) it returns
`has_update=false`, so the host does not show a "ready to update" prompt that would
disagree with the online state. Nothing is discarded — the downloaded version directory and
`version.json` stay untouched, and `apply_update` runs as soon as the server is reachable again.
`applying` is the deliberate exception: crash recovery and rollback resumption are always
reported, even offline, otherwise a machine left without its application folder could never
recover by itself.

Crash recovery: if `version_status` is `applying` on startup, the updater re-executes the atomic rename step — the update is never left in a broken half-state. The updater writes `apply_stage=activating` right before the final activate rename, and that marker is the evidence used to decide whether the rename already happened (legacy states without it fall back to the old heuristic and log a warning).

State guards (a write never happens on an inconsistent state):

- `apply_update` refuses to replace when `version_status` is empty or unknown (an empty state means
  "no pending update", exactly like `applied`), and refuses `version_previous == version` (the backup
  and version directories would collide). Unknown values are only repaired by `download_update`,
  which rewrites the state as `downloaded`.
- `apply_update` validates the downloaded package first: if `ApplicationFolder_{version}` is missing,
  empty or has fewer files than the `downloaded_file_count` recorded at download time (files deleted,
  or a same-version re-download killed halfway), it refuses the replace and self-heals to `applied` +
  the real content version, so the next `check_update` re-downloads instead of "succeeding" with a
  copy of the current folder or a mixed old/new package. An empty `version` in the `downloaded` state
  is rejected the same way, and a directory that exists but cannot be read is reported without any
  state change.
- `rollback` refuses to run while an apply is unfinished (`applying` with no `rollback_previous`), and
  on failure restores the pre-rollback status instead of always writing `applied`.
- First-ever updates (`version_previous` empty) used to leave a nameless `ApplicationFolder_` backup
  that no rollback could reach and `prune` never cleaned; it is now removed on success (and pruned when
  leftover from older builds).

### Atomic Replace

`apply_update` performs:
1. Copy all files from current `ApplicationFolder` into `ApplicationFolder_{new_version}` (excluding ignored paths) so the new version directory is complete.
2. Rename `ApplicationFolder` → `ApplicationFolder_{previous_version}` (backup; the nameless
   `ApplicationFolder_` when there is no previous version).
3. Rename `ApplicationFolder_{new_version}` → `ApplicationFolder` (after writing `apply_stage=activating`).
4. If any rename fails, revert `version_status` to `downloaded` so it can be retried (and clear the
   transient `rollback_previous`/`rollback_target`/`apply_stage` fields).

### Client API Overview

| Command | Description |
|---------|-------------|
| `check_update` | Compare local version vs. server latest |
| `check_diff` | List files that differ (MD5+SHA256 comparison) |
| `download_update` | Download changed files, verify SHA-256 |
| `apply_update` | Atomic replace + restart your app |
| `list_rollback_versions` | List available previous versions |
| `rollback --version X` | Rollback to a previous version snapshot |
| `check_self_update` | Check if the updater exe itself needs updating |

## Building from Source

### Server & Publish CLI

```bash
# Server
cd server && go build -ldflags="-s -w" -o aly-server.exe .

# Publish CLI
cd publish/publish-cli && go build -ldflags="-s -w" -o aly-publish.exe .

# Cross-compile (static, no CGO)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aly-server-linux-amd64 .
```

### Publish GUI

```bash
cd publish/publish-gui && dotnet build
```

### Client Updater

Requires Go 1.10 (Windows XP compatibility, no Go modules). Source must be under GOPATH:

```bat
:: Copy source to GOPATH
xcopy /e /i /y client\aly-client %GOPATH%\src\aly\client\aly-client

:: Build the updater (console)
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
set CGO_ENABLED=0
go build -ldflags="-s -w" -o aly-client.exe aly/client/aly-client

:: Build the optional GUI front-end (no console window)
go build -ldflags="-s -w -H=windowsgui" -o aly-client-ui.exe aly/client/aly-client/ui/winui
```

> `aly-client-ui` is written with [lxn/walk] and needs the ComCtl32 v6 manifest.
> The repository already contains `client/aly-client/ui/winui/rsrc.syso`, which
> `go build` links automatically. Do **not** delete it, otherwise the UI fails at
> startup with `TTM_ADDTOOL failed`.
>
> Locally you can run `go run deploy-client.go` (WSL + Go 1.10.8) to build both
> executables and deploy them to every target listed in `config.json`.

[lxn/walk]: https://github.com/lxn/walk

## License

MIT
