# AlyClient.CSharpSDK

Aly auto-update client SDK for .NET. Wraps `aly-client.exe` CLI and provides a managed update lifecycle with event-driven status reporting.

## Installation

```xml
<PackageReference Include="AlyClient.CSharpSDK" Version="1.0.0" />
```

After installing the NuGet package, the following files are automatically copied to `..\UpdateFolder\` relative to your project's output directory on build:

```
..\UpdateFolder\
├── aly-client.exe      # CLI updater (Go 1.10, 32-bit, XP compatible)
└── client.json          # Main exe path + process names to close
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

## Update Lifecycle

| Status | Meaning |
|--------|---------|
| `None` | Idle, polling for updates every second |
| `DiscoveredUpdate` | A new version was found on the server |
| `DownloadingUpdate` | Downloading changed files with progress |
| `DownloadedUpdate` | Download complete, waiting to apply |
| `ApplyUpdate` | Performing atomic replace + restart |

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

- .NET Framework 4.0+ or .NET Standard 2.0+
- `..\UpdateFolder\aly-client.exe` (deployed by NuGet build targets)
- A Aly server running and configured in `..\UpdateFolder\.updator\shared.json` (or passed via CLI args to `aly-client.exe`)
