# Quick Start

Three roles complete one full update cycle: **start the server** → **publish a version** → **client checks and applies**.

## 1. Start the Server

```bash
./aly-server -p 2000
```

The first run creates a SQLite database (`aly.db`) automatically; uploaded files are stored per project under `data/{project_name}/`. The server binary is a statically built single file with no external dependencies — copy it to your server and run.

## 2. Publish the First Version

Inside your **release directory** (the folder that ships to users, e.g. `./dist`):

```bash
cd ./dist

# Initialize project config (creates .updator/)
aly-publish config init --server http://localhost:2000 --project myapp

# See what changed
aly-publish status

# Stage everything and push
aly-publish add --all
aly-publish push --version V1.0.0 --message "First release"
```

`push` scans the directory, uploads new/changed files (large files automatically use chunked upload) and creates a version changelog entry. Every later release repeats the same three commands: `status → add --all → push`.

::: tip Config ships with versions
The publish config `.updator/shared.json` (server URL, project name, ignore rules) is uploaded with every release. When the client downloads an update, the new config replaces the old — **changes like a new server address roll out automatically with the next update**.
:::

## 3. Client: Check & Apply

On the **end-user machine**, the host application invokes `aly-client` (inside `UpdateFolder/`) in three steps:

```bash
# Step 1 — check for updates
aly-client.exe check_update
# → {"isSuccess":true,"data":{"has_update":true,"need_download_update":true,
#    "new_version":"1.0.1","force_update":false}}

# Step 2 — download (only changed files, SHA-256 verified)
aly-client.exe download_update

# Step 3 — apply (closes the app, atomically replaces, restarts)
aly-client.exe apply_update
```

Drive the UX from `force_update`: when `true`, auto-download and prompt for restart; when `false`, ask the user first.

## 4. Deployment Layout on User Machines

```text
PackageFolder/
├── ApplicationFolder/            # Active app (current version)
│   ├── your_app.exe
│   └── .updator/shared.json     # server_url, project_name, ignore rules
├── ApplicationFolder_V1.0.0/     # Previous version snapshot (for rollback)
├── UpdateFolder/
│   ├── aly-client.exe            # The updater itself
│   ├── aly-client-ui.exe         # Optional GUI front-end
│   ├── client.json               # main_exe_relative_path, must_close_process_name …
│   └── version.json              # Version state machine (maintained automatically)
```

`client.json` is the only client-side config you need to care about:

```json
{
  "main_exe_relative_path": "ApplicationFolder/your_app.exe",
  "must_close_process_name": ["your_app"]
}
```

- `main_exe_relative_path`: the main executable relative to the package root; `apply_update` launches it after the replacement.
- `must_close_process_name`: processes that must exit before an update is applied.

## 5. Skip the Orchestration with the C# SDK

A .NET host doesn't need to hand-write those steps — the `AlyClient.CSharpSDK` NuGet package orchestrates check, download, apply and self-update:

```csharp
var client = new AlyUpdateClient(updateFolder, mainFolder, ...);
if ((await client.CheckUpdateAsync()).Data.HasUpdate)
{
    await client.DownloadUpdateAsync();
    await client.ApplyUpdateAsync();   // app is closed, replaced and restarted
}
```

See [C# SDK](/en/docs/sdk).

## 6. Publish GUI (Optional)

Prefer clicks over commands? `aly-publish-gui` (an Avalonia desktop app) wraps the full publish-cli capability set: pick a directory, review diffs, enter a version, publish.

```bash
cd publish/publish-gui && dotnet build
```
