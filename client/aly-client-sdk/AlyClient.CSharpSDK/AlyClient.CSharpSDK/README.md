# AlyClient.CSharpSDK

Aly auto-update client SDK for .NET. Wraps `aly-client.exe` CLI and provides a managed update lifecycle with event-driven status reporting.

## Installation

The package is **not published to NuGet.org**. Add a project reference, or put the
`AlyClient.CSharpSDK.*.nupkg` release asset into a local feed and reference it:

```xml
<PackageReference Include="AlyClient.CSharpSDK" Version="1.0.0" />
```

Build targets only prepare the folder: `..\UpdateFolder\` is created next to your project's output
directory, and a default `client.json` is written there when missing. The updater executables are
**not** downloaded by the SDK any more — take them from the AlyPublish package
(`client/<rid>/aly-client` + `client/<rid>/aly-client-ui`), which the publish GUI also copies into the
sibling `UpdateFolder/` when it initializes a project:

```
..\UpdateFolder\
├── aly-client(.exe)     # CLI updater (Go 1.10, 32-bit, XP compatible) — from the package
├── aly-client-ui(.exe)  # optional GUI front-end                — from the package
└── client.json          # Main exe path + process names to close — written by the targets
```

Edit `..\UpdateFolder\client.json` to match your application's structure:

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

## Quick Start

```csharp
using System.Threading;
using AlyClient.CSharpSDK;

// The SDK runs a background loop: check → download → apply
var client = new AlyUpdateClient();

// Required: receive status updates
client.StatusChanged += (status, tips) =>
{
    Console.WriteLine($"[{status}] {tips}");
};

// Required for non-force updates: block until user confirms download
client.RequestDownloadUpdate += (newVersion) =>
{
    Console.WriteLine($"Download {newVersion}? Waiting for user...");
    // Your UI shows a dialog; when confirmed, cancel the token
    while (!_cts.IsCancellationRequested)
        Thread.Sleep(500);
};

// Required for non-force updates: block until user confirms apply
client.RequestApplyUpdate += (newVersion) =>
{
    Console.WriteLine($"Apply {newVersion}? Waiting for user...");
    while (!_cts.IsCancellationRequested)
        Thread.Sleep(500);
};
```

## GUI Front-End (aly-client-ui) — Recommended

`AlyUpdateUIClient` hands the whole lifecycle (check → download → apply → restart your app)
to the standalone `aly-client-ui.exe` window, so the host app does **not** need to draw any
update UI, poll in the background, or handle process restart itself:

```csharp
using AlyClient.CSharpSDK;

var ui = new AlyUpdateUIClient();           // defaults to ..\UpdateFolder\aly-client-ui.exe
ui.Exited += code => Console.WriteLine("aly-client-ui exit " + code);

// At startup: hidden unless an update is found; silent when offline / up to date.
ui.LaunchAuto();

// When the user clicks "Check for updates": always shows feedback, auto-closes when no update.
ui.LaunchCheck();

// Other modes
ui.LaunchUpdate();     // download + apply without asking
ui.LaunchRollback();   // version picker for rollback
ui.LaunchSilent();     // same as auto: quiet unless an update is confirmed
```

| Member | Purpose |
|--------|---------|
| `Launch(AlyUpdateUiMode)` | Start the UI with a specific `-mode` |
| `LaunchAuto/Check/Update/Rollback/Silent()` | Convenience wrappers for the modes above |
| `UiExePath` / `Language` | Override exe path (`zh-CN` / `en-US`) |
| `IsRunning` / `LastExitCode` | Last process state |
| `Started` / `Exited` | Process lifecycle events (exit code) |

Notes:

- `aly-client-ui.exe` is a single-instance app. Calling `Launch` while the previous window is
  still alive is fine: the new process forwards a "manual check requested" signal to the existing
  instance (named event) and exits with code 0; the existing instance then shows its window and
  handles the request interactively. `Launch` returns success either way.
- After `apply_update` the UI relaunches your main executable (see `client.json`
  `main_exe_relative_path` / `must_close_process_name`), so the host does not need to restart itself.
- Comes from the AlyPublish package (`client/<rid>/aly-client-ui`), next to `aly-client` in `UpdateFolder/`.

## Update Lifecycle

| Status | Meaning |
|--------|---------|
| `None` | Idle, polling for updates every second |
| `DiscoveredUpdate` | A new version was found on the server |
| `DownloadingUpdate` | Downloading changed files with progress |
| `DownloadedUpdate` | Download complete, waiting to apply |
| `ApplyUpdate` | Performing atomic replace + restart |

If a check reports no pending update while a status is still displayed, `StatusChanged` fires
`None` once — this clears a stale panel after the network drops mid-flow (e.g. "Found new
version" / "Downloading..." never resolves). A downloaded update is only reported as pending
while the server is reachable: **offline machines show no update prompt** and the pending
download stays on disk until the server is reachable again (the network check is the only
thing skipped; nothing is discarded).

### Force Update vs Non-Force

- **`force_update = true`**: The SDK downloads and applies automatically — `RequestDownloadUpdate` / `RequestApplyUpdate` are **not raised**.
- **`force_update = false`**: The SDK waits for user confirmation via `RequestDownloadUpdate` and `RequestApplyUpdate` events before proceeding.

## Events

| Event | Signature | When |
|-------|-----------|------|
| `StatusChanged` | `Action<AlyClientStatus, string>` | Every status transition, with a human-readable message |
| `RequestDownloadUpdate` | `Action<string>` | Non-force only — blocks until your callback returns (user confirmed) |
| `RequestApplyUpdate` | `Action<string>` | Non-force only — blocks until your callback returns (user confirmed) |

## Advanced: Direct CLI Access

If you want to bypass the managed lifecycle, call `AlyApi` static methods directly:

```csharp
using AlyClient.CSharpSDK;

var exePath = @"..\UpdateFolder\aly-client.exe";

// Check for updates
var check = await AlyApi.CheckUpdateAsync(exePath);
if (check.Data.HasUpdate && check.Data.NeedDownloadUpdate)
{
    // Download with progress
    var download = await AlyApi.DownloadUpdateAsync(exePath, (file, progress) =>
    {
        Console.WriteLine($"{file}: {progress:P0}");
    });
}

// Apply update (exits current process, restarts your app)
AlyApi.ApplyUpdateAsync(exePath);

// Rollback
var versions = await AlyApi.ListRollbackVersionsAsync(exePath);
await AlyApi.RollbackAsync(exePath, "1.0.0");
```

## Prerequisites

- `.NET Framework 4.0+` or `.NET Standard 2.0+`
- `..\UpdateFolder\aly-client(.exe)` — taken from the AlyPublish package (the SDK targets only create the folder)
- `..\UpdateFolder\aly-client-ui(.exe)` — optional GUI front-end from the same package
- A Aly server running and configured in `..\UpdateFolder\.updator\shared.json` (or passed via CLI args to `aly-client.exe`)
