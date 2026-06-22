# Zap — Lightweight Client Auto-Update System

A minimal, self-contained auto-update solution for desktop applications. Ship new versions with a single command, and let your users update seamlessly with crash-safe atomic replacement.

## Project Structure

```
zap/
├── server/         # API server (Go, Gin + Ent + SQLite)
├── client/
│   └── zap-client/      # Client updater (Go 1.10, Windows XP compatible)
│   └── zap-client-sdk/  # (placeholder) Future Go/C# integration SDK
├── publish/
│   ├── publish-cli/      # CLI publish engine (Go, cobra)
│   └── publish-gui/  # Desktop GUI (Avalonia 11.3, .NET 8)
└── README.md
```

**Four modules:**

| Module | Role | For whom |
|--------|------|----------|
| **zap-server** | Stores project metadata & files, serves REST API | Deploy on your infrastructure |
| **zap-publish** | CLI tool to init configs, scan files, and publish versions | Developers / CI |
| **zap-publish-gui** | Desktop GUI wrapping zap-publish via subprocess | Non-technical publishers |
| **zap-client** | The updater executable — runs on end-user machines | End users (via your app) |

## Quick Start

### 1. Start the Server

```bash
./zap-server -p 2000
```

First run auto-creates a SQLite database (`zap.db`). The server stores project info and uploaded files under `data/{project_name}/`.

### 2. Publish a New Version

```bash
# Initialize project config (creates .updator/ directory)
cd ./dist
zap-publish config init --server http://localhost:2000 --project myapp

# Check what files changed
zap-publish status

# Stage all changes and push
zap-publish add --all
zap-publish push --version V1.0.1 --message "Fixed login bug"
```

This scans the directory, uploads new/changed files to the server, and creates a version changelog entry.

### 3. Client: Check & Apply Updates

On the end-user's machine, your application invokes `zap-update.exe` (located in `UpdateFolder/`):

```bash
# Step 1 — Check for updates
zap-update.exe check_update
# → {"isSuccess":true,"data":{"has_update":true,"need_download_update":true,"new_version":"1.0.1","force_update":false}}

# Step 2 — Download (only downloads changed files, with SHA-256 verification)
zap-update.exe download_update

# Step 3 — Apply (closes your app, atomically replaces files, restarts)
zap-update.exe apply_update
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
│   ├── zap-update.exe            # The updater itself
│   ├── client.json               # main_exe_relative_path, must_close_process_name
│   └── version.json              # Version state machine
```

## Key Concepts

### `.updator/` — Shared Config Directory

Both `zap-publish` and `zap-update` read from `.updator/shared.json`:

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
                  └──(check_update)──→ directly returns has_update=true
```

Crash recovery: if `version_status` is `applying` on startup, the updater re-executes the atomic rename step — the update is never left in a broken half-state.

### Atomic Replace

`apply_update` performs:
1. Copy all files from current `ApplicationFolder` into `ApplicationFolder_{new_version}` (excluding ignored paths) so the new version directory is complete.
2. Rename `ApplicationFolder` → `ApplicationFolder_{previous_version}` (backup).
3. Rename `ApplicationFolder_{new_version}` → `ApplicationFolder`.
4. If any rename fails, revert `version_status` to `downloaded` so it can be retried.

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
cd server && go build -ldflags="-s -w" -o zap-server.exe .

# Publish CLI
cd publish/publish-cli && go build -ldflags="-s -w" -o zap-publish.exe .

# Cross-compile (static, no CGO)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o zap-server-linux-amd64 .
```

### Publish GUI

```bash
cd publish/publish-gui && dotnet build
```

### Client Updater

Requires Go 1.10 (Windows XP compatibility, no Go modules). Source must be under GOPATH:

```bat
:: Copy source to GOPATH
xcopy /e /i /y client\zap-client %GOPATH%\src\zap\client\zap-client

:: Build
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o zap-update.exe zap/client/zap-client
```

## License

MIT
