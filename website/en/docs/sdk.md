---
title: C# SDK Integration
description: Integrating AlyClient.CSharpSDK into net40 and netstandard2.0 hosts, covering the two integration styles, UpdateFolder deployment layout, client.json relative paths, build-time targets properties, and common pitfalls.
---

# C# SDK Integration

> For other languages (TypeScript / Go / Python / Java / Rust / C++ / Pascal / VB.NET / F# / VB6 / EPL) see the
> [Language SDK overview](/en/docs/sdk/languages). This page covers .NET only.

`AlyClient.CSharpSDK` wraps the command-line protocol of `aly-client.exe` into .NET types: hosts no longer have to concatenate arguments, parse JSON, or implement timeouts and process reaping themselves. It exposes only four entry points:

> **Who this is for**: developers adding updates to a .NET host (WinForms/WPF/console). After this page you can choose an integration style, lay out `UpdateFolder` correctly, and run the first update from a minimal host.

| Type | Role |
|---|---|
| `AlyUpdateUIClient` | Hands the entire "check → download → apply → restart the host" chain to a standalone window, `aly-client-ui.exe`; the host only launches it at the right moment (**recommended**) |
| `AlyUpdateClient` | The host provides its own background polling and update UI, and the SDK orchestrates the same chain as an event-driven state machine |
| `AlyApi` | Static methods mapping one `aly-client` command each, for custom timing |
| Models and enums | Strongly typed views of command output; see [API Reference](/en/docs/sdk/api) |

The first two styles are mutually exclusive; do not enable both. Each path spawns its own `aly-client` processes, and although the updater enforces mutual exclusion with `.aly.lock` internally, the windows and states will interfere with each other.

## Target Frameworks and Dependencies

| Item | Value |
|---|---|
| Target frameworks | `netstandard2.0`, `net40` |
| NuGet dependency | `Newtonsoft.Json` 13.0.4 |
| Assembly | `AlyClient.CSharpSDK.dll` |
| Runtime components | `aly-client.exe` (required); `aly-client-ui.exe` (required only by `AlyUpdateUIClient`) |
| Updater form | 32-bit single-file exe, built with Go 1.10 / `GOARCH=386`, compatible with Windows XP |

The package ships both `build\` and `buildTransitive\` targets, so both direct and transitive references trigger the deployment logic (see [build-time automatic deployment](#build-time-auto-deployment-targets)).

### net40 and C# 5 Syntax Constraints

The SDK itself is built with a modern compiler, and a host using the VS2013 C# 5 compiler has no problem **referencing the compiled DLL**. If the host pulls in the SDK source as a project reference, however, it needs VS2017 or later, or the `dotnet` CLI — the SDK projects are SDK-style (`Microsoft.NET.Sdk`), and old MSBuild cannot open them.

All examples in this article stay within what C# 5 can compile:

| Do not use (C# 6+) | Replacement in a net40 host |
|---|---|
| `using var client = ...` (C# 8) | `AlyUpdateClient client = new AlyUpdateClient();` followed by an explicit `try/finally { client.Dispose(); }` |
| String interpolation `$"{status}"` (C# 6) | `string.Format("[{0}] {1}", status, message)` or `"[" + status + "] " + message` |
| `?.` / `??` null-conditional operators (C# 6) | explicit `if (x != null)` checks |
| `nameof(x)` (C# 6) | write the string literal directly |
| Expression-bodied members `=> ...` (C# 6) | a normal method body |
| Tuples, pattern matching, `out var` (C# 7) | declare the output parameter first, then pass it in |
| `await` (the async support types only ship with .NET 4.5) | take `Task<T>.Result`, or reference the `Microsoft.Bcl.Async` package and then `await` |

`var` has been available since C# 3 and is legal on net40; this article still writes explicit types uniformly, only so that the examples behave consistently under any old compiler and code-review tool.

## Getting the SDK

**The package is not published to NuGet.org**: `AlyClient.CSharpSDK` cannot be found in public feeds, and writing a `<PackageReference>` will fail to restore. The Release workflow only builds `AlyClient.CSharpSDK.*.nupkg` and uploads it as a **GitHub Release asset**; it does not push to nuget.org. Download the nupkg from the release, or pack it locally as described below.

Use one of the following two approaches.

### Option A: Project Reference (Least Effort When the Source Is in the Same Repository)

```xml
<ItemGroup>
  <ProjectReference Include="..\client\aly-client-sdk\AlyClient.CSharpSDK\AlyClient.CSharpSDK\AlyClient.CSharpSDK.csproj" />
</ItemGroup>
```

Across repositories, copy the entire SDK directory into the solution, or switch to Option B.

### Option B: Pack into a Local nupkg

```bash
cd client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK
dotnet pack -c Release -o nupkg -p:Version=1.0.0
```

Put the resulting `AlyClient.CSharpSDK.1.0.0.nupkg` into a local feed (for example `<add key="local" value="D:\nuget-local" />` in `NuGet.config`), after which you can use `<PackageReference Include="AlyClient.CSharpSDK" Version="1.0.0" />`.

::: tip Updater binaries come from the AlyPublish package
`AlyClient.CSharpSDK.targets` now does only two things: creates `..\UpdateFolder\` and writes a default
`client.json` when one is missing. `aly-client` / `aly-client-ui` come from `client/<rid>/` inside the
**AlyPublish package**: the GUI copies them into the sibling `UpdateFolder/` when initializing a project,
or you can take them out of the package by hand (`chmod 0755` on Linux). See
[Download & Install](/en/docs/download).
:::

## Deployment Layout: UpdateFolder

Aly's client directory convention is a three-layer structure of "package directory + application directory + update directory":

```text
PackageFolder/
├── ApplicationFolder/            # the active application (replaced wholesale on apply)
│   ├── YourApp.exe
│   └── .updator/shared.json      # server_url, project_name, ignore rules
├── ApplicationFolder_1.0.0/      # historical version snapshot (for rollback)
└── UpdateFolder/
    ├── aly-client.exe            # updater (required)
    ├── aly-client-ui.exe         # GUI front-end (required by AlyUpdateUIClient)
    ├── client.json               # host relative path + processes to close
    └── version.json              # version state machine (maintained automatically, do not edit)
```

The default paths of `AlyUpdateClient` and `AlyUpdateUIClient` are both one level above `AppDomain.CurrentDomain.BaseDirectory`: `..\UpdateFolder\aly-client.exe` and `..\UpdateFolder\aly-client-ui.exe`. The host process output directory must therefore live where `..\UpdateFolder` can find the updater: production usually publishes straight into `ApplicationFolder/`, while development uses `bin\Debug\net40\`.

The actual shape of the development directory (`OutDir = bin\Debug\net40\`):

```text
bin\Debug\
├── net40\YourApp.exe             # host output
└── UpdateFolder\                 # where the targets write ($(OutDir)..\UpdateFolder\)
    ├── aly-client.exe
    ├── aly-client-ui.exe
    └── client.json
```

In this case the host path in `client.json` should be written as `../net40/YourApp.exe`; if you have already switched to the `PackageFolder/ApplicationFolder` layout, write `../ApplicationFolder/YourApp.exe`.

### Relative Paths in client.json

`main_exe_relative_path` is **relative to UpdateFolder** (the updater's directory, `config.ExeDir()`), not to the process working directory, and not to the package root:

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

| Desired target | Correct value |
|---|---|
| Host under `ApplicationFolder`, a sibling of `UpdateFolder` | `../ApplicationFolder/YourApp.exe` |
| Host inside the development output directory (`bin\Debug\net40`) | `../net40/YourApp.exe` |
| Another directory under the package root | `../OtherFolder/App.exe` |

- Writing `ApplicationFolder/YourApp.exe` (missing the `../`) resolves to `UpdateFolder/ApplicationFolder/YourApp.exe`; after `apply_update` the host fails to restart, so the update "completes" but the application never comes up.
- On Linux, backslashes are rewritten to `/` by `config.normalizeMainExeRelativePath`, so the same package works across platforms; writing `/` uniformly is safer.
- `must_close_process_name` is an array of process names without `.exe`; `apply_update` sends `WM_CLOSE` first and force-kills after the timeout.
- These two fields are read by `aly-client` itself; the SDK's `MustCloseProcessNames` property only additionally passes the value to `apply_update`/`rollback` through `--must-close-process-name`.

For the full semantics of these fields see [Configuration Reference](/en/docs/client/config); for updater commands see [Command Reference](/en/docs/commands).

## Quick Start (Minimal Host)

Three prerequisites: the host project references `AlyClient.CSharpSDK` (including `Newtonsoft.Json`), `UpdateFolder` already contains `aly-client.exe` and `aly-client-ui.exe`, and `client.json` has been adjusted for the actual directory layout.

A complete, compilable minimal host (`net40` + C# 5). The repository's `client/aly-client-sdk/samples/MinimalHost` is the same project (`net40` + `net8.0`) and can be used as a starting point:

```csharp
using System;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

namespace MyApp
{
    internal static class Program
    {
        private static void Main()
        {
            // Locates ..\UpdateFolder\aly-client-ui.exe by default
            AlyUpdateUIClient ui = new AlyUpdateUIClient();
            ui.Language = "zh-CN";

            AlyResponse result = ui.LaunchAuto();
            if (!result.IsSuccess)
            {
                Console.WriteLine("Update component unavailable: " + result.ErrorMsg);
            }

            // Host initialization and message loop
            Console.WriteLine("Host started; press Enter to exit");
            Console.ReadLine();

            ui.Dispose();
        }
    }
}
```

This code only uses the constructor, `LaunchAuto()` and `Dispose()`; `ui.Dispose()` does not close an update window that is already open. To control the update UI and its timing yourself, use `AlyUpdateClient` in the next section.

## Integration Style 1: AlyUpdateUIClient (Recommended)

The host draws no update UI, does no polling and handles no restart: everything is delegated to `aly-client-ui.exe`.

```csharp
using System;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

class Program
{
    static void Main()
    {
        AlyUpdateUIClient ui = new AlyUpdateUIClient();   // defaults to ..\UpdateFolder\aly-client-ui.exe
        ui.Language = "zh-CN";                            // zh-CN / en-US
        ui.Exited += OnUiExited;

        AlyResponse launched = ui.LaunchAuto();           // silent check at startup; prompts only when an update exists
        if (!launched.IsSuccess)
        {
            Console.WriteLine("Failed to launch the update UI: " + launched.ErrorMsg);
        }

        // The host's own initialization and message loop
        Console.ReadLine();

        ui.Dispose();                                     // releases handles only; does not close the window
    }

    static void OnUiExited(int exitCode)
    {
        Console.WriteLine("aly-client-ui exited, exit code = " + exitCode);
    }
}
```

Choose the launch mode by scenario:

| Mode | Convenience method | Behavior |
|---|---|---|
| `AlyUpdateUiMode.Auto` | `LaunchAuto()` | Window hidden by default, shown only when an update is confirmed; exits silently when there is no update or the server is unreachable. **Call at application startup** |
| `AlyUpdateUiMode.Check` | `LaunchCheck()` | The user explicitly clicks "Check for updates": shows "Checking" first; on no update or error it still notifies and closes automatically, so that clicking it never appears to do nothing |
| `AlyUpdateUiMode.Update` | `LaunchUpdate()` | Goes straight to download + apply without asking again |
| `AlyUpdateUiMode.Rollback` | `LaunchRollback()` | Shows the list of rollback-able versions for the user to choose in the window |
| `AlyUpdateUiMode.Silent` | `LaunchSilent()` | Silent check with the same semantics as `Auto`: no update/error stays unobtrusive, and the window appears only once an update is confirmed |

You can also call `Launch(AlyUpdateUiMode.Check)` directly, which is equivalent to `LaunchCheck()`.

Key points:

- `Launch` is non-blocking and only spawns the process; downloading, applying, closing and restarting the host all happen inside the UI.
- **Single instance**: if the process held by this object is still alive, a further `Launch` returns `IsSuccess=true` directly without spawning a second process. If the window was not launched by this object (for example the user opened it from the Start menu, or another code path launched it), `Launch` still starts a new process; that process forwards the "manual check request" to the existing instance through a named event and then exits with code 0 — this is the normal path, and the host does not need to distinguish "new window" from "existing instance taking over".
- `aly-client-ui.exe` must exist: when missing, `Launch` returns `IsSuccess=false` and `ErrorMsg` is `aly-client-ui.exe not found: <path>`.
- After `apply_update` the UI closes and restarts the host according to `client.json`; the host does not restart itself.
- `Dispose` only releases the `Process` handle and unsubscribes events; it **does not close** an already running UI window.
- When no GUI window is needed, switch to `AlyUpdateClient` below; both binaries come from the AlyPublish package and the build-time targets no longer download anything.

## Integration Style 2: AlyUpdateClient Event Loop

Background work starts as soon as the constructor returns: it first self-updates the updater, then enters the "check → download → apply" polling loop. The host observes stage changes through events.

```csharp
using System;
using System.Diagnostics;
using System.Threading;
using AlyClient.CSharpSDK;

class Program
{
    static AlyUpdateClient _client;
    static ManualResetEvent _userConfirmed = new ManualResetEvent(false);

    static void Main()
    {
        _client = new AlyUpdateClient();                       // defaults to ..\UpdateFolder\aly-client.exe
        _client.MustCloseProcessNames = Process.GetCurrentProcess().ProcessName;
        _client.IdlePollIntervalSeconds = 300;

        _client.StatusChanged += OnStatusChanged;
        _client.ErrorStatusChanged += OnErrorChanged;
        _client.RequestDownloadUpdate += OnRequestDownload;
        _client.RequestApplyUpdate += OnRequestApply;

        Console.ReadLine();

        _client.Cancel();
        _client.Dispose();
    }

    static void OnStatusChanged(AlyClientStatus status, string message)
    {
        // Events fire on a background thread; cross-thread UI updates need marshaling
        Console.WriteLine(string.Format("[{0}] {1}", status, message));
    }

    static void OnErrorChanged(string message)
    {
        if (message == null)
        {
            Console.WriteLine("Error cleared");
            return;
        }
        Console.WriteLine("[error] " + message);
    }

    static void OnRequestDownload(string newVersion)
    {
        // The callback runs synchronously on the background loop thread: returning means "user confirmed", and the SDK starts downloading immediately
        Console.WriteLine("New version " + newVersion + " found; waiting for the user to confirm the download");
        _userConfirmed.WaitOne();
        _userConfirmed.Reset();
    }

    static void OnRequestApply(string newVersion)
    {
        Console.WriteLine("New version " + newVersion + " downloaded; waiting for the user to confirm apply");
        _userConfirmed.WaitOne();
        _userConfirmed.Reset();
    }

    static void Confirm()
    {
        _userConfirmed.Set();   // real code calls this from the "OK" button on the UI thread
    }
}
```

Event and state correspondence:

```text
CheckUpdate ──has_update=false──▶ idle polling (300s default + jitter)
     │
     └─has_update=true──▶ DiscoveredUpdate ──▶ (non-force) RequestDownloadUpdate
                                   │
                                   ▼
                          DownloadingUpdate (with file name and percentage)
                                   │
                                   ▼
                          DownloadedUpdate ──▶ (non-force) RequestApplyUpdate
                                   │
                                   ▼
                              ApplyUpdate ──▶ separate apply_update process; host is restarted
```

Key semantics:

- **Events have no "reject" semantics**: returning from the callback counts as confirmation, and the SDK proceeds. The only way to abort the whole flow is `Cancel()`. If you want the user to be able to "cancel the download", the correct approach is for the host to decide not to show the update entry point, not to return a fake confirmation from the callback.
- **Callbacks block the background thread synchronously**: waiting forever in `RequestDownloadUpdate` prevents the download from starting. Wait on a `ManualResetEvent`/semaphore and call `Set` from the UI thread.
- With `force_update = true` the two confirmation events **do not fire**; the SDK downloads and applies directly.
- The same error repeating does not raise `ErrorStatusChanged` again; on recovery it fires once with `message = null`.
- Exceptions thrown by event handlers are caught by the SDK and written to the log; they do not interrupt the background loop — so do not expect exceptions to abort the flow.
- Background polling cadence: a failed `check_update` retries after a fixed 5 seconds; with no update it follows `IdlePollIntervalSeconds` (default 300 seconds + 0–60 seconds random jitter, floor 60 seconds); after launching `apply_update` it waits 30 seconds before continuing (to avoid spawning the process repeatedly).
- Self-update runs before the loop starts, and failure information goes to the log; it delegates the replacement to the `self_update` command and falls back to a direct replacement when the target is missing or the command is unavailable.
- The log file lives in the host base directory: `aly-client-sdk.log`. The `self-update: ...` and `StatusChanged handler threw: ...` lines you see come from there.

For the exact semantics of `Status`, `IsRunning`, `IsError` and other members see [API Reference](/en/docs/sdk/api#alyupdateclient).

## Calling AlyApi Directly (Custom Timing)

Each method of `AlyApi` maps to one `aly-client` command, and every return value is an `AlyResponse` / `AlyResponse<T>`:

| Method | Command | Return model |
|---|---|---|
| `CheckSelfUpdateAsync(exePath)` | `check_self_update` | `CheckSelfUpdateData` |
| `SelfUpdateAsync(exePath)` | `self_update` | `SelfUpdateData` |
| `CheckUpdateAsync(exePath)` | `check_update` | `CheckUpdateData` |
| `DownloadUpdateAsync(exePath, progress)` | `download_update` | no data |
| `ApplyUpdateAsync(exePath, names, timeout)` | `apply_update` | no data |
| `ListRollbackVersionsAsync(exePath)` | `list_rollback_versions` | `ListRollbackData` |
| `RollbackAsync(exePath, version, names, timeout)` | `rollback` | `RollbackData` |

```csharp
using System;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

class Direct
{
    static void Run(string exePath)
    {
        AlyResponse<CheckUpdateData> check = AlyApi.CheckUpdateAsync(exePath).Result;
        if (!check.IsSuccess)
        {
            Console.WriteLine("check failed: " + check.ErrorMsg);
            return;
        }

        if (check.Data.HasUpdate && check.Data.NeedDownloadUpdate)
        {
            Action<string, double> onProgress = delegate(string file, double ratio)
            {
                Console.WriteLine(file + " " + ((int)(ratio * 100)) + "%");
            };

            AlyResponse download = AlyApi.DownloadUpdateAsync(exePath, onProgress).Result;
            if (!download.IsSuccess)
            {
                Console.WriteLine("download failed: " + download.ErrorMsg);
                return;
            }
        }

        // Runs in a separate process and returns immediately; the current process is closed afterwards
        AlyResponse apply = AlyApi.ApplyUpdateAsync(exePath);
        Console.WriteLine("apply spawn result: " + apply.IsSuccess);
    }
}
```

For timeouts and failure modes (30 seconds by default, 600 seconds for rollback, 1800 seconds for a stalled download) see [API Reference](/en/docs/sdk/api#timeouts-and-failure-modes).

## Build-Time Targets: Only Prepare UpdateFolder

`AlyClient.CSharpSDK.targets` in the SDK package runs `AlyDeployToUpdateFolder` (`BeforeTargets="Build"`)
during the host build. It does **only two things**: creates the destination directory and writes a default
`client.json` when one is missing. The updater binaries come from the AlyPublish package (the GUI copies
them into the sibling `UpdateFolder/`); the SDK no longer downloads anything.

| MSBuild property | Default | Description |
|---|---|---|
| `AlyUpdateFolder` | Windows: `$(OutDir)..\UpdateFolder\`; other platforms: `$(OutDir)../UpdateFolder/` | Destination directory |

Behavior:

- When `client.json` is missing, a default template is written (`main_exe_relative_path` is
  `../ApplicationFolder/YourApp.exe`, `must_close_process_name` is `["YourApp"]`); adjust it to the real
  layout. An existing file is never overwritten, so hand-edited config is safe.
- The updater and GUI are **not** placed here: they ship in the AlyPublish package and the GUI copies them
  in (see [Download & Install](/en/docs/download)). Manual placement still works: take
  `client/<rid>/aly-client(.exe)` and `aly-client-ui(.exe)` out of the package into `UpdateFolder/`
  (`chmod 0755` on Linux).
- Platform detection uses `$(OS)`, compatible with VS2013/2015 builds on net40.

To deliver updaters for another platform, take the matching `client/<rid>/` directory from the package
(Windows: `client/windows-x86/`; Linux: `client/linux-amd64/` or `client/linux-arm64/`).

## Common Pitfalls

| Symptom | Cause and fix |
|---|---|
| The build succeeds but there is no `aly-client.exe` in `UpdateFolder` | The SDK no longer downloads the updater: use the AlyPublish package (the GUI copies it when initializing a project), or take `client/<rid>/` out of the package by hand (`chmod 0755` on Linux) |
| `check_update` keeps returning `has_update=false` although the server clearly has a new version | When the host machine is offline the client returns `has_update=false` (it reports nothing it cannot verify with the server) and resumes automatically once online; first confirm that `server_url` in `.updator/shared.json` is reachable |
| The host does not start after an update | `main_exe_relative_path` in `client.json` is missing the leading `../`, or the exe name does not match the actual file. Check against the table under "Relative Paths in client.json" above |
| The application is killed outright with no chance to save settings | `MustCloseProcessNames` is not set. The updater force-kills when it detects the directory is in use during the replacement phase; set it to `Process.GetCurrentProcess().ProcessName` (without `.exe`) |
| A non-force update proceeds automatically without waiting for user confirmation | `RequestDownloadUpdate` / `RequestApplyUpdate` are not subscribed: with no handler the SDK treats the update as confirmed and downloads and applies directly. If you need to ask the user, you must subscribe |
| A non-force flow hangs at "new version found" | The confirmation callback is waiting on an event that is never `Set`, or the callback performs long blocking work. Callbacks run on the background loop thread, so any wait needs the other side to signal from another thread |
| `await` does not compile on net40 | .NET 4.0 has no async support types. Use `.Result`, or reference `Microsoft.Bcl.Async` |
| The UI freezes after calling `.Result` on the UI thread | `check_update` waits up to 30 seconds by default and `download_update` can take longer. Call them from a background thread |
| The host is still shutting down after `apply`, then the process suddenly disappears | `ApplyUpdateAsync` returns immediately after spawning a separate process, and `apply_update` closes the current process. Do not do cleanup after it; move cleanup into an exit hook |
| The updater cannot be found on Linux | The default path is the Windows-style `..\UpdateFolder\aly-client.exe`; on Linux there is no `.exe` and backslash is not a separator. Pass the path explicitly: `new AlyUpdateClient("/opt/app/UpdateFolder/aly-client")` |
| `LaunchAuto` fails saying `aly-client-ui.exe` was not found | There is no GUI front-end in `UpdateFolder`: take `client/<rid>/aly-client-ui(.exe)` from the package and place it there, or switch to `AlyUpdateClient` |
| Two update windows appear at once | `AlyUpdateUIClient` and `AlyUpdateClient` are both enabled in one host. Keep only one |
| Closing the host hangs for a few seconds | `Dispose` waits for the background loop to exit, up to 5 seconds; the polling sleep itself is interruptible and never sleeps a full idle period |

## Related Documentation

- [C# SDK API Reference](/en/docs/sdk/api)
- [aly-client Command Reference](/en/docs/commands)
- [Client Configuration Reference](/en/docs/client/config)
- [Download and Installation](/en/docs/download)
- [Self-Update Design](/en/docs/self-update)
- [Core Concepts](/en/docs/concepts)