# C# SDK

`AlyClient.CSharpSDK` is a NuGet package for .NET hosts that wraps the full check → download → apply → self-update orchestration into a few classes; hosts only subscribe to events and await tasks. Targets **net40 + netstandard2.0**.

## Getting It

- NuGet package: build the `AlyClient.CSharpSDK` project to produce the nupkg, which embeds the build targets (see automatic deployment below).
- Source location: `client/aly-client-sdk/AlyClient.CSharpSDK/`.

## Two Styles

### High-Level Orchestration: AlyUpdateClient (recommended)

`AlyUpdateClient` drives the update state machine on a background thread; the host subscribes to events:

```csharp
using AlyClient.CSharpSDK;

using var client = new AlyUpdateClient();  // locates ..\UpdateFolder\aly-client.exe by default

client.StatusChanged += (status, msg) => Console.WriteLine($"{status}: {msg}");
client.RequestDownloadUpdate += _ => Console.WriteLine("New version found, downloading");
client.RequestApplyUpdate   += _ => Console.WriteLine("Download complete, applying");

// MustCloseProcessNames / IdlePollIntervalSeconds etc. are configurable
client.Cancel();   // cancel an in-flight flow
```

Features:

- **Status machine**: the `AlyClientStatus` enum reflects the current stage; `IsError` flags failure states.
- **Self-update built in**: on startup it runs `CheckSelfUpdateAsync` first and delegates to the `self_update` command when needed (the updater itself takes the lock — no concurrent writes).
- **Download guardian**: progress callback `Action<string, double>`, with a stall timeout (default 1800 s) that detects a hung download and can recover.
- **Rollback**: `ListRollbackVersionsAsync` / `RollbackAsync` (default timeout 600 s).

### Low-Level Calls: AlyApi Static Methods

Hosts that want full control can call static methods directly — one per aly-client command:

| Method | Command | Notes |
|---|---|---|
| `CheckUpdateAsync(alyExePath)` | `check_update` | Returns `CheckUpdateData` (has_update / force_update / new_version) |
| `DownloadUpdateAsync(path, progress)` | `download_update` | Progress callback + stall timeout |
| `ApplyUpdateAsync(...)` | `apply_update` | The main app is restarted after apply |
| `ListRollbackVersionsAsync(path)` | `list_rollback_versions` | |
| `RollbackAsync(...)` | `rollback` | |
| `CheckSelfUpdateAsync(path)` | `check_self_update` | Detects an outdated updater copy |
| `SelfUpdateAsync(path)` | `self_update` | Performs the self-update replacement |

## NuGet Targets Auto-Deployment

The package's `build/AlyClient.CSharpSDK.targets` deploys the updater executables into `AlyUpdateFolder` **while building the host project**; all properties are overridable:

| MSBuild property | Default | Description |
|---|---|---|
| `AlyClientVersion` | — | Version label of the updater to download |
| `AlyClientDownloadUrl` | GitHub Release direct link | Download URL for the `aly-client` executable |
| `AlyClientUiDownloadUrl` | GitHub Release direct link | Download URL for the GUI executable |
| `AlyUpdateFolder` | `$(OutDir)..\UpdateFolder\` | Destination directory |
| `AlyIncludeClientUi` | `true` | Whether to deploy the GUI as well |
| `AlyRid` | `amd64` | Linux architecture (override with `arm64`) |

Platform behavior:

- **Windows**: PowerShell downloads `aly-client.exe` / `aly-client-ui.exe`.
- **Linux**: `curl || wget` downloads `aly-client-linux-<rid>` / `aly-client-ui-linux-<rid>`, renames them to `aly-client` / `aly-client-ui` and `chmod 0755` — matching the client's `config.UpdaterFileName()` platform convention.
- Download failures warn without breaking the build; identical existing files are skipped (idempotent).
