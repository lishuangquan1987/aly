---
title: Quick Start
description: Pick your OS and integration path, then run the shortest working route for server, publish and client updates.
---

# Quick Start

::: tip Check the server first
The “Publish a release” and “Integrate the updater” paths assume the server is already running. If it is not, choose “Deploy the server”, or start it in step 1 of the manual flow below.
:::

<QuickStartWizard />

## Full flow (manual)

### 1. Start the server

```bash
./aly-server -p 2000
```

The first run creates `aly.db` and `data/` next to the executable. On Linux use `./aly-server-linux-amd64`, on Windows `.\aly-server.exe`; `-db` changes the database path.

### 2. Create the server project

A new project starts at **V1.0.0**, so the first `push` must use a higher version (for example V1.0.1).

```bash
# In the release directory (e.g. dist), initialize the client config
./aly-publish config init --server http://127.0.0.1:2000 --project myapp

# Register the same project on the server (once)
./aly-publish project create --server http://127.0.0.1:2000 --name myapp --title "My App"
```

### 3. Publish the first release

```bash
./aly-publish status
./aly-publish add --all
./aly-publish push --version V1.0.1 --message "First release"
```

`status` shows new/modified/deleted files; `add --all` stages new and modified files; `push` uploads them and records a changelog. Large files use chunked upload automatically.

### 4. End-user layout

```text
PackageFolder/
├── ApplicationFolder/              # active app
│   ├── YourApp.exe
│   └── .updator/shared.json        # server_url, project_name, ignore rules
├── ApplicationFolder_V1.0.0/       # previous snapshot (rollback)
└── UpdateFolder/
    ├── aly-client(.exe)            # updater
    ├── aly-client-ui(.exe)         # optional GUI
    ├── client.json                 # main exe path + processes to close
    └── version.json                # state machine (automatic)
```

`main_exe_relative_path` is resolved **relative to `UpdateFolder/`**:

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

### 5. Check and apply on the client

```bash
# Step 1 — check
aly-client check_update
# → {"isSuccess":true,"data":{"has_update":true,"need_download_update":true,
#    "new_version":"1.0.2","force_update":false,"release_notes":["..."]}}

# Step 2 — download changed files (MD5 + SHA256 verified)
aly-client download_update

# Step 3 — apply (close processes → atomic replace → restart)
aly-client apply_update
```

Business failures may still exit 0; use `isSuccess` in the JSON. See the [command reference](/en/docs/commands).

### 6. C# host: skip the orchestration

```csharp
using AlyClient.CSharpSDK;

// Requires UpdateFolder/aly-client-ui.exe and aly-client.exe
var ui = new AlyUpdateUIClient();
ui.LaunchAuto();   // silent check at startup; window appears only on update
```

The SDK is not on public NuGet yet; use a project reference or a CI-built nupkg. See [C# SDK integration](/en/docs/sdk).

## Next steps

- Every flag and return field: [aly-client command reference](/en/docs/commands)
- Crash safety and the state machine: [Core concepts](/en/docs/concepts)
- CI or scripted publishing: [aly-publish](/en/docs/publish)
- Troubleshooting: [FAQ](/en/docs/faq)