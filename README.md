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

:: Build
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o aly-client.exe aly/client/aly-client
```

## License

MIT
