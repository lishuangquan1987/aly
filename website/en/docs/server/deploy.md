---
title: Deploying aly-server
description: "Deploying aly-server as a single binary: startup flags, data directory layout, SQLite WAL and automatic migration, systemd unit, reverse proxy and security hardening, backup/restore, and upgrades."
---

# Deploying aly-server

aly-server is the server side of the update system: it stores project and version metadata, receives release artifacts, and serves file lists and downloads to the updater.

**Deployment form**: a single statically compiled binary with the SQLite driver embedded (`modernc.org/sqlite`, a pure-Go implementation). It does not depend on CGO and needs no separate database process or runtime. Apart from the `data/` directory and `aly.db` that the binary writes itself, it has no external dependencies.

## Getting the binary

Release artifacts are named `aly-server-{goos}-{goarch}` and downloaded from GitHub Releases:

| Platform | Asset name |
| --- | --- |
| Windows x64 | `aly-server-windows-amd64.exe` |
| Linux x64 | `aly-server-linux-amd64` |
| Linux arm64 | `aly-server-linux-arm64` |
| macOS x64 | `aly-server-darwin-amd64` |
| macOS arm64 | `aly-server-darwin-arm64` |

You can also build from source (the artifact is a pure static binary, `CGO_ENABLED=0`):

```bash
cd server
CGO_ENABLED=0 go build -ldflags="-s -w" -o aly-server .
```

## Startup and command-line flags

The server has only two flags:

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `-p` | int | `2000` | Listen port |
| `-db` | string | empty | Database file path; when empty, uses `aly.db` in the executable's directory |

```bash
# Default: listen on 2000, database and data directory both next to the executable
./aly-server

# Specify the port and database location
./aly-server -p 8080 -db /var/lib/aly/aly.db
```

After startup it prints `启动中,监听端口:2000` ("starting, listening port: 2000"), followed by Gin's access logs. On `SIGINT` / `SIGTERM` the process shuts down gracefully: it stops accepting new requests, waits up to 10 seconds for in-flight requests, then closes the database.

The listen address is `0.0.0.0:<port>`; there is no flag to bind only to the loopback address. If only local access should be allowed, restrict it with a firewall, or let a reverse proxy own the external port.

## Directory layout

```text
/opt/aly/
├── aly-server                    # binary
├── aly.db                        # SQLite main database
├── aly.db-wal                    # WAL log (present at runtime)
├── aly.db-shm                    # shared-memory index (present at runtime)
└── data/
    ├── demo/                     # project name → directory name
    │   ├── app.exe
    │   ├── config/
    │   │   └── app.json
    │   └── .updator_manifest.json  # manifest frozen by the most recent release
    └── demo2/
```

<ScreenshotFrame src="/images/server-layout-en.svg" alt="aly-server directory layout: binary, SQLite database and data/ project directories at the same level" caption="data/ and aly.db stay next to the executable; one directory per project" wide />

Key points:

- **`data/` is fixed to the executable's directory**, derived from `os.Executable()` and independent of the working directory at startup. systemd's `WorkingDirectory` does not affect it.
- The database also defaults to the executable's directory; only when running via `go run` (the executable lands in a temporary directory) does it fall back to the current working directory.
- `data/{projectName}` is created on demand by the server: creating a project or the first file upload creates the directory.
- `.updator_manifest.json` is an update-protocol file; it never appears in `get_all_files` or the release manifest.
- Upload intermediates `{file}.chunks/` (chunk staging) and `{file}.merging` (merge temp file) are filtered out too; they exist only briefly during a transfer.

## Database: SQLite + WAL + automatic migration

The server initializes the database at startup in the following order (`server/internal/db/db.go`):

1. Resolve the database path: `-db` takes precedence, otherwise `executable directory/aly.db`.
2. `MkdirAll` creates the directory containing the database file (permissions `0700`).
3. The connection string gets `_fk=1`, and `PRAGMA foreign_keys = ON` is executed.
4. `PRAGMA journal_mode=WAL`: concurrent reads and writes do not block each other, and the database is recoverable after a crash.
5. `PRAGMA busy_timeout=5000`: retry for up to 5 seconds on lock contention instead of failing immediately with `database is locked`.
6. `SetMaxOpenConns(1)`: SQLite has a single writer, so the connection pool is capped at 1, eliminating lock upgrades across multiple connections at the root.
7. `Client.Schema.Create`: ent automatic migration, **creating tables/adding columns at startup**; no manual SQL is required.
8. One-time data migration: if the old database's `project_change_logs` table lacks a `project_id` column, first `ADD COLUMN project_id`, then copy the values from the old `project_change_logs` column, and finally `DROP COLUMN` the old column (a failed drop is only logged and does not affect startup).

Therefore:

- Upgrading the binary needs no separate migration step; it is completed automatically on first startup;
- **Do not** open the same database file from two processes at once (for example, starting two copies of the service);
- When backing up, treat `aly.db`, `aly.db-wal`, and `aly.db-shm` as one unit, or stop the service first so the WAL is flushed to disk.

WAL mode produces `aly.db-wal` / `aly.db-shm`; this is normal, do not delete them manually.

## systemd deployment (Linux)

Prepare the directory and account (both the database and the data directory need write access, so they live in the same writable directory):

```bash
sudo useradd --system --home /opt/aly --shell /usr/sbin/nologin aly
sudo mkdir -p /opt/aly
sudo install -m 0755 aly-server-linux-amd64 /opt/aly/aly-server
sudo chown -R aly:aly /opt/aly
```

`/etc/systemd/system/aly-server.service`:

```ini
[Unit]
Description=Aly update server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=aly
Group=aly
WorkingDirectory=/opt/aly
ExecStart=/opt/aly/aly-server -p 2000
Restart=on-failure
RestartSec=3

# Graceful shutdown: SIGTERM starts a 10-second drain, leaving some margin
KillSignal=SIGTERM
TimeoutStopSec=15

# The data directory and aly.db are both under /opt/aly, no temporary directory is used, so this can be enabled safely
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/opt/aly

StandardOutput=journal
StandardError=journal
SyslogIdentifier=aly-server

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now aly-server
sudo systemctl status aly-server
sudo journalctl -u aly-server -f
```

Notes:

- `ProtectSystem=full` mounts only `/usr`, `/boot`, and `/etc` read-only; `/opt/aly` remains writable. `ReadWritePaths=/opt/aly` is an explicit declaration here. If you switch to the stricter `ProtectSystem=strict`, this line becomes mandatory, otherwise both the database and uploads will fail;
- Access logs go to stdout and runtime logs (cache hits, manifest write failures, etc.) go to stderr; both are imported into the journal;
- If you put the binary in `/usr/local/bin`, `data/` and `aly.db` will land in `/usr/local/bin`, which is usually not writable — `-db` can only relocate the database, while `data/` cannot be moved, so keep the binary in its own writable directory.

### Windows

Run `aly-server-windows-amd64.exe -p 2000` directly; `data\` and `aly.db` are created next to the exe. To register it as a Windows service, use a third-party wrapper such as NSSM; the repository does not provide an installer script.

## Reverse proxy

The server only speaks HTTP. When exposing it externally, put Nginx (or an equivalent proxy) in front for TLS termination.

```nginx
server {
    listen 443 ssl;
    server_name update.example.com;

    ssl_certificate     /etc/nginx/ssl/update.example.com.crt;
    ssl_certificate_key /etc/nginx/ssl/update.example.com.key;

    # The server itself limits to 1 GB; keep this slightly higher so over-limit requests get the server's JSON error instead of the proxy's 413
    client_max_body_size 1050m;

    # Uploads/downloads may last several minutes; avoid the proxy timing out first
    proxy_connect_timeout 10s;
    proxy_send_timeout    600s;
    proxy_read_timeout    600s;

    # Large files are no longer fully buffered to disk by Nginx first
    proxy_request_buffering off;

    location / {
        proxy_pass http://127.0.0.1:2000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # Downloads are binary streams; do not compress and do not rewrite the response body
        gzip off;
    }
}
```

Three easy traps:

1. **Do not mount it under a path prefix.** The client's `buildAPIURL` takes only `scheme://host` from `serverURL`; the path part is discarded. Configuring `https://example.com/aly` actually requests `https://example.com/api/...`. Use a dedicated domain or a dedicated port.
2. **Do not add an authentication layer.** Existing aly-client and aly-publish send no `Authorization` header and do not support client certificates; adding Basic Auth / tokens at the proxy makes every official client receive 401. For cross-network access, use a firewall IP allowlist or a VPN.
3. **Preserve Range semantics.** `download_file` uses `http.ServeContent` for resumable downloads; do not cache complete responses or rewrite `206` / `Content-Range`, otherwise resuming a large file degrades into re-downloading the whole file.

TLS itself is supported: simply set `serverURL` to `https://update.example.com`.

## Security hardening

| Measure | Description |
| --- | --- |
| Network isolation | Open the port only on the intranet/office network; public entry goes through the proxy only |
| Firewall | Allow only the proxy host to reach `2000`; deny everything else |
| Runtime account | Use a dedicated low-privilege account that owns `/opt/aly`, with permissions of `0750` or stricter |
| File permissions | `data/` contains no secrets but does contain executables; never expose that directory as a static site from a web server |
| Backup encryption | Backups contain all release artifacts; treat them as internal assets |
| Audit | Keep `journalctl` access logs; the server has no built-in audit or rate limiting |

The server validates no input semantics beyond the project name (for example, the content of publish scripts); `after_apply_update_script` is only stored and served, and execution happens on the client. Publish permission therefore equals code-execution permission on the target machines.

## Backup and restore

Only two things need backing up: `aly.db` (metadata) and `data/` (release artifacts).

### Offline full backup (recommended)

```bash
sudo systemctl stop aly-server
sudo tar -czf /backup/aly-$(date +%F-%H%M).tar.gz -C /opt/aly aly.db data
sudo systemctl start aly-server
```

After shutdown the WAL has been flushed as the database closed, so `aly.db` is complete and consistent; `data/` and the database contents are also from the same point in time.

### Online backup

The database can use SQLite's online backup:

```bash
sqlite3 /opt/aly/aly.db ".backup '/backup/aly-$(date +%F).db'"
```

But `data/` still needs a separate sync (for example `rsync -a`). The database and the file directory are not an atomic snapshot: in extreme cases you can back up a state where "the log has registered a version, but the files are not fully copied" — clients may receive an incomplete version. In production, prefer offline backups.

### Restore

```bash
sudo systemctl stop aly-server
sudo rm -f /opt/aly/aly.db /opt/aly/aly.db-wal /opt/aly/aly.db-shm
sudo tar -xzf /backup/aly-2026-11-10-1200.tar.gz -C /opt/aly
sudo chown -R aly:aly /opt/aly
sudo systemctl start aly-server
```

Verify:

```bash
curl -s http://127.0.0.1:2000/api/project/get_all_projects
curl -s http://127.0.0.1:2000/api/file/get_all_files/demo
```

The first should list projects, and the second should list files with `md5` / `sha256`. If the project exists but the file list is empty, `data/` was usually not restored or has the wrong permissions.

## Upgrade

```bash
sudo systemctl stop aly-server
sudo cp -a /opt/aly/aly.db /backup/aly.db.bak
sudo cp /opt/aly/aly-server /opt/aly/aly-server.bak
sudo install -m 0755 aly-server-linux-amd64 /opt/aly/aly-server
sudo systemctl start aly-server
```

- No manual migration is needed: table creation and column additions are handled by ent at startup, and the old database's `project_id` column migration also runs at startup.
- Back up `aly.db` before upgrading: schema migrations only move forward, and a new version may already have added or changed columns.
- **Rollback**: just swap the old binary back, but if the new version has already applied schema changes, the conservative approach is to also restore the `aly.db` backup taken before the upgrade; swapping only the binary does not guarantee the old version can read the new database.
- `data/` does not change with versions and needs no handling; `.updator_manifest.json` is rewritten on the next publish.

## Troubleshooting

| Symptom | Common cause | Fix |
| --- | --- | --- |
| Exits immediately on startup; the log shows `failed opening connection to sqlite` | The `-db` directory is not writable | Check the directory owner and permissions |
| Exits immediately on startup; the log shows `failed creating schema resources` | The database file is corrupted or held by another process | Stop duplicate instances; restore from a backup |
| Upload returns "file too large" (`文件过大`) | The single file exceeds 1 GB, or the proxy's `client_max_body_size` is too small | Adjust the proxy limit; confirm this is a real business need |
| The proxy returns 413 | Nginx's default 1 MB limit | Set `client_max_body_size 1050m` |
| Download returns 404 | `path` is not an absolute path under `data/`, or the binary has moved | Request the `fileAbsolutePath` from `get_all_files` verbatim |
| Download is cut off midway | The server's `WriteTimeout` is 60 seconds (cap for the whole response) | Large files on slow links get truncated; the client retries with Range resumption |
| `database is locked` | The same database file is opened by multiple processes | Ensure only one aly-server instance is running |
| Changelog time differs from local time | `time` is a string supplied by the publisher and stored verbatim by the server | Have the publisher consistently use `yyyy-MM-dd HH:mm:ss` and the same time zone |

The server has no built-in version-query endpoint; confirm the running version from the `version` returned by `GET /api/server/info` (the Go runtime version) together with the binary filename/release record.

## Related pages

- [API Overview](/en/docs/api): envelope, error semantics, endpoint index
- [Project API](/en/docs/server/api-project) / [File API](/en/docs/server/api-file): endpoint-level details
- [Data Models](/en/docs/server/api-models): tables and JSON fields
- [aly-publish publishing tool](/en/docs/publish): the side that pushes files and versions to the server