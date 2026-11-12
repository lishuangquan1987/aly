---
title: C# SDK API Reference
description: The complete AlyClient.CSharpSDK API, covering member signatures, enums, response and data models, timeouts and failure modes for AlyUpdateClient, AlyApi and AlyUpdateUIClient.
---

# C# SDK API Reference

This page lists every public type and member of `AlyClient.CSharpSDK`; the signatures match the source. For integration steps and deployment notes see the [Integration Guide](/en/docs/sdk).

> **Model source:** data-model fields are generated from [`spec/aly-protocol.schema.json`](https://github.com/lishuangquan1987/aly/blob/master/spec/aly-protocol.schema.json) by `tools/alyprotocol`. To add or remove a field, edit the schema and `client/aly-client/model/dto.go`, then regenerate; do not hand-edit `Models/Generated/*.g.cs`. See [`spec/README.md`](https://github.com/lishuangquan1987/aly/blob/master/spec/README.md).

## Namespaces

| Namespace | Types |
|---|---|
| `AlyClient.CSharpSDK` | `AlyApi`, `AlyUpdateClient`, `AlyUpdateUIClient`, `AlyClientStatus`, `AlyUpdateUiMode`, `CheckUpdateData`, `CheckSelfUpdateData`, `ListRollbackData`, `RollbackData`, `DownloadProgressData` (`internal`) |
| `AlyClient.CSharpSDK.Models` | `AlyResponse`, `AlyResponse<T>`, `SelfUpdateData` |

::: tip Namespaces are easy to misremember
Not all data models live under `.Models`: `CheckUpdateData`, `CheckSelfUpdateData`, `ListRollbackData` and `RollbackData` are in `AlyClient.CSharpSDK`, while only `AlyResponse`, `AlyResponse<T>` and `SelfUpdateData` are in `AlyClient.CSharpSDK.Models`. When you use both response wrappers and check results, you need both `using` directives.
:::

## AlyUpdateClient

Managed wrapper around `aly-client.exe`: a background thread first runs the updater self-update and then the "check → download → apply" polling loop.

### Constructors

```csharp
public AlyUpdateClient(string updatorExePath = null)
```

| Parameter | Description |
|---|---|
| `updatorExePath` | Path to `aly-client.exe`. When empty, uses `Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\aly-client.exe")` |

The constructor **starts the background task immediately** (`Task.Factory.StartNew`), rather than deferring it to the first method call. `IsRunning` is set to `true` inside the constructor.

The default path is Windows-style: on Linux there is no `.exe` and backslash is not a separator, so `UpdateFolder/aly-client` must be passed explicitly.

### Properties

| Member | Type | Access | Description |
|---|---|---|---|
| `Status` | `AlyClientStatus` | read-only (thread-safe) | Current phase. The getter and the internal setter share the same lock, so it can be read across threads |
| `IsRunning` | `bool` | read-only | Whether the background loop is running. `true` after construction, `false` once the loop exits (`Cancel`/`Dispose`/top-level exception) |
| `IsError` | `bool` | read-only (thread-safe) | Whether the most recent `check_update` or download failed. Cleared by the next successful `check_update` |
| `UpdatorExePath` | `string` | read/write | Updater path. Assignment applies `Path.GetFullPath` automatically; assigning an empty value leaves it as-is (no conversion) |
| `MustCloseProcessNames` | `string` | read/write | Names of processes to close before applying an update, **without `.exe`**, comma-separated. When unset, the updater force-kills the host if it finds the directory in use during the replacement phase |
| `IdlePollIntervalSeconds` | `int` | read/write | Idle (no update) polling interval in seconds. `<= 0` is treated as **300**, and values below **60** as 60; each actual sleep is `interval + 0–60 seconds random jitter`, the jitter spreading out client fleets. Failure retries are unaffected (fixed 5 seconds) |

### Methods

#### `public void Cancel()`

Stops background polling. Implemented via `CancellationTokenSource.Cancel()`; idempotent, and `ObjectDisposedException` is swallowed. Sleeps are interruptible (the token is checked every second), so the loop exits within at most 1 second after `Cancel`, never sleeping a full idle period.

#### `public void Dispose()`

`Cancel()` + waits for the main loop to exit (at most **5 seconds**) + releases the `CancellationTokenSource`. Idempotent. `Dispose` does not terminate a separate process already launched by `apply_update`, nor does it delete any file in `UpdateFolder`.

### Events

| Event | Signature | When it fires |
|---|---|---|
| `StatusChanged` | `event Action<AlyClientStatus, string>` | On every state transition; the second argument is a human-readable message (including the download file name and percentage) |
| `RequestDownloadUpdate` | `event Action<string>` | Non-force updates only; the argument is the new version number. **Called synchronously**; returning means the user confirmed |
| `RequestApplyUpdate` | `event Action<string>` | Non-force updates only; the argument is the new version number. **Called synchronously**; returning means the user confirmed |
| `ErrorStatusChanged` | `event Action<string>` | Error state changes. The argument is the error text (a new error or changed error content); `null` on recovery; the same error repeating does not fire again |

Event semantics in detail:

- All events fire on the background loop thread; cross-thread UI updates need marshaling.
- Exceptions thrown by handlers are caught by the SDK and written to `aly-client-sdk.log`; they do not interrupt the loop.
- The `Status` property is updated before the event fires, so a lost event cannot make the state stale.
- Download progress is reported through `StatusChanged`; the text looks like `Downloading {file name}... {percentage}%`, where the percentage is "completed files / total files" and counts only `DONE` files.
- When there is no pending update and the previous status was not `None`, one extra `StatusChanged(None, "Update status cleared")` is emitted, so the host UI does not stay stuck at "new version found" after the network drops.

### Background Loop Behavior

| Situation | Follow-up |
|---|---|
| `check_update` returns failure | Fires `ErrorStatusChanged`, sleeps 5 seconds, retries |
| `check_update` succeeds | Clears the error state (fires `ErrorStatusChanged(null)` once if there was an error) |
| `has_update = false` | Resets the status to `None` (emitting an event if necessary) and sleeps for the idle interval |
| `has_update = true` and `need_download_update = true` | Status `DiscoveredUpdate`; for non-force updates fires `RequestDownloadUpdate` first; then `DownloadingUpdate` and the download |
| Download fails and the error contains `already at latest version` | Treated as no update: status resets to `None` and sleeps for the idle interval (avoiding an endless error loop in force-update scenarios) |
| Download fails (other reason) | Fires the error, sleeps 1 second, retries this round |
| Download succeeds | Status `DownloadedUpdate`; for non-force updates fires `RequestApplyUpdate`; then `ApplyUpdate` |
| `apply_update` fails to spawn | Fires `ErrorStatusChanged("apply spawn failed: ...")`; on success the host process is closed by the updater |
| Any exception | `ErrorStatusChanged("update exception: ...")`, sleeps 5 seconds and continues |

After `apply_update` is spawned the loop sleeps an extra **30 seconds** before checking again: the command is a separate process (the SDK does not wait for it to finish) and holds the update lock meanwhile, so spawning it frequently only wastes processes.

Self-update runs before the loop: it prefers to delegate to the `self_update` command; when the target file is missing, or the command fails and `UpdateFolder/.aly.lock` does not exist, it falls back to a direct replacement of "temp file + SHA256 verification + atomic replace" (using `File.Replace` on Windows, and backup rename + rollback on non-Windows). When the lock is held it does not fall back, to avoid bypassing the update lock.

## AlyApi

Static methods mapping one `aly-client` command each. All `Task` return values are blocking calls wrapped in `Task.Factory.StartNew` — `.Result` blocks the current thread until the command finishes; this is not true asynchronous I/O.

### Method Overview

| Method | Command | Returns | Default timeout |
|---|---|---|---|
| `CheckSelfUpdateAsync(string alyExePath)` | `check_self_update` | `Task<AlyResponse<CheckSelfUpdateData>>` | 30 s |
| `SelfUpdateAsync(string alyExePath)` | `self_update` | `Task<AlyResponse<SelfUpdateData>>` | 30 s |
| `CheckUpdateAsync(string alyExePath)` | `check_update` | `Task<AlyResponse<CheckUpdateData>>` | 30 s |
| `DownloadUpdateAsync(string alyExePath, Action<string, double> progress)` | `download_update` | `Task<AlyResponse>` | stall watchdog 1800 s |
| `DownloadUpdateAsync(string alyExePath, Action<string, double> progress, int stallTimeoutSeconds)` | `download_update` | `Task<AlyResponse>` | custom stall timeout |
| `ListRollbackVersionsAsync(string alyExePath)` | `list_rollback_versions` | `Task<AlyResponse<ListRollbackData>>` | 30 s |
| `RollbackAsync(string alyExePath, string version, string mustCloseProcessNames = null, int closeTimeoutSeconds = 30)` | `rollback --version <v>` | `Task<AlyResponse<RollbackData>>` | 600 s |
| `ApplyUpdateAsync(string alyExePath, string mustCloseProcessNames = null, int closeTimeoutSeconds = 30)` | `apply_update` | `AlyResponse` (synchronous) | does not wait |

### Method Details

#### `CheckSelfUpdateAsync`

```csharp
public static Task<AlyResponse<CheckSelfUpdateData>> CheckSelfUpdateAsync(string alyExePath)
```

Checks whether the updater's own copy is behind the copy in the main application directory. Detection only, no replacement. Returns `CheckSelfUpdateData.NeedUpdate`.

#### `SelfUpdateAsync`

```csharp
public static Task<AlyResponse<SelfUpdateData>> SelfUpdateAsync(string alyExePath)
```

Performs the updater self-update: after comparing SHA256, replaces the running copy in `UpdateFolder` with the `aly-client.exe` distributed with the version in the main application directory. Idempotent; a missing source or an already up-to-date copy also returns `IsSuccess=true`, with `SelfUpdateData.Reason` being `source_missing` or `already_up_to_date`.

#### `CheckUpdateAsync`

```csharp
public static Task<AlyResponse<CheckUpdateData>> CheckUpdateAsync(string alyExePath)
```

Checks whether the application has a new version. Returns `has_update=false` when the server is unreachable — even if a downloaded version is already pending apply locally, it is not reported, and the check resumes automatically once online.

#### `DownloadUpdateAsync`

```csharp
public const int DefaultDownloadStallTimeoutSeconds = 1800;

public static Task<AlyResponse> DownloadUpdateAsync(string alyExePath, Action<string, double> progress)

public static Task<AlyResponse> DownloadUpdateAsync(string alyExePath, Action<string, double> progress, int stallTimeoutSeconds)
```

Downloads the diff files. **There is no upper bound on total duration**, only a "stall watchdog": for large single-file downloads the client prints one `PROGRESS` line every 3 seconds, so "no output for longer than `stallTimeoutSeconds`" means a real stall (network hang, disk deadlock, hung process) and the SDK kills the process and returns failure. The watchdog timer checks every 5 seconds.

| Parameter | Description |
|---|---|
| `alyExePath` | Updater path |
| `progress` | `Action<string, double>`: file name and progress ratio `index / total` (0–1, counting only `DONE` files). Exceptions thrown by the callback are swallowed and do not interrupt the download. May be `null` |
| `stallTimeoutSeconds` | No-output timeout in seconds; defaults to `DefaultDownloadStallTimeoutSeconds` (1800) |

Returns `Task<AlyResponse>` (no `data`). The progress data model `DownloadProgressData` is `internal`, so hosts can only obtain the file name and ratio through the callback.

#### `ApplyUpdateAsync`

```csharp
public static AlyResponse ApplyUpdateAsync(
    string alyExePath,
    string mustCloseProcessNames = null,
    int closeTimeoutSeconds = 30)
```

Applies an already downloaded update (atomic replace), which closes and restarts the main process. **A synchronous method that does not wait for the command to finish**: it internally launches `aly-client.exe` as a separate process and immediately returns `AlyResponse.OK()`, returning `NG` only when the process fails to start.

| Parameter | Description |
|---|---|
| `mustCloseProcessNames` | Comma-separated process names (without `.exe`); when non-empty, appends `--must-close-process-name "<value>"` |
| `closeTimeoutSeconds` | Close-process timeout; when equal to 30 no argument is passed, otherwise appends `--close-timeout <value>` |

Callers must not do cleanup work after it returns — the updater closes the current process before replacement.

#### `ListRollbackVersionsAsync`

```csharp
public static Task<AlyResponse<ListRollbackData>> ListRollbackVersionsAsync(string alyExePath)
```

Lists the rollback-able historical version snapshots (`ApplicationFolder_<version>/` directories) and returns `CurrentVersion` and `Versions`.

#### `RollbackAsync`

```csharp
public const int RollbackTimeoutSeconds = 600;

public static Task<AlyResponse<RollbackData>> RollbackAsync(
    string alyExePath,
    string version,
    string mustCloseProcessNames = null,
    int closeTimeoutSeconds = 30)
```

Rolls back to the specified version. The timeout is 600 seconds: rollback involves directory renames, waiting for the main process to exit, and crash-recovery branches (possibly several rename retries); 30 seconds is not enough — killing the process on timeout would leave the rollback in an intermediate state (crash recovery covers it, but the experience is poor).

| Parameter | Description |
|---|---|
| `version` | Target version number, appended as `rollback --version <version>` |
| `mustCloseProcessNames` / `closeTimeoutSeconds` | Same as `ApplyUpdateAsync` |

### Timeouts and Failure Modes

| Constant | Value | Scope |
|---|---|---|
| `AlyApi.DefaultDownloadStallTimeoutSeconds` | `1800` | `public const`: default download stall watchdog |
| `AlyApi.RollbackTimeoutSeconds` | `600` | `public const`: `RollbackAsync` timeout |
| Short-command default timeout | `30` | `private const`: `check_self_update`, `self_update`, `check_update`, `list_rollback_versions`; not configurable |

On timeout: kills `aly-client.exe`, waits at most 2 more seconds for the read tasks to finish, returns `IsSuccess=false` with `ErrorMsg` set to `aly-client.exe timed out after <N>s and was killed`.

Other failure modes (all `IsSuccess=false`):

| Situation | `ErrorMsg` |
|---|---|
| Streams closed but the process does not exit within 1 second | `aly-client.exe exited streams but process did not exit` (the process is killed) |
| Non-zero exit code with empty stdout and empty stderr | `aly-client.exe exited with code <N> (may require admin)` |
| Non-zero exit code with empty stdout and non-empty stderr | the stderr content |
| Exit code 0 but empty stdout | `aly-client.exe returned no output (may require admin)` |
| stdout is not valid JSON | the deserialization exception's `Message` |
| Client outputs a failure JSON during download | the client's `errorMsg` returned verbatim |
| Process exits during download before the finish line | `download interrupted (aly-client exited before finish line)` |
| Download stalled and the watchdog killed the process | `download stalled: no output for <N>s, process killed` |
| Launching the process throws (file not found, etc.) | the exception's `Message` |

When the exit code is non-zero but stdout has content, the SDK trusts the JSON in stdout (the client may return a business result with a non-zero exit code).

### Process Invocation Conventions

`AlyApi` always uses the following settings when starting `aly-client.exe`:

| Setting | Value |
|---|---|
| `UseShellExecute` | `false` |
| `CreateNoWindow` | `true` (no console window) |
| `WorkingDirectory` | `Path.GetDirectoryName(alyExePath)`, i.e. `UpdateFolder` |
| `RedirectStandardOutput` / `RedirectStandardError` | `true` (the separate process from `ApplyUpdateAsync` is not redirected) |
| `StandardOutputEncoding` / `StandardErrorEncoding` | `Encoding.UTF8` |

Output must therefore be UTF-8 JSON. Also, because the working directory is set to `UpdateFolder`, `aly-client` resolves `client.json`, `version.json` and the like relative to `UpdateFolder`, while the main-app configuration `.updator/shared.json` is located from the main application directory recorded in `client.json`.

## AlyUpdateUIClient

Launch wrapper for `aly-client-ui.exe`: the entire update chain is handed to the GUI, and the host only launches it for each scenario.

### Constructors

```csharp
public const string DefaultLanguage = "zh-CN";

public AlyUpdateUIClient(string uiExePath = null, string language = DefaultLanguage)
```

| Parameter | Description |
|---|---|
| `uiExePath` | UI path. When empty, uses `Path.GetFullPath(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\aly-client-ui.exe"))`; a non-empty value is also converted to an absolute path |
| `language` | UI language `zh-CN` / `en-US`; falls back to `DefaultLanguage` when blank |

### Properties

| Member | Type | Access | Description |
|---|---|---|---|
| `UiExePath` | `string` | read/write | Absolute path of the UI executable; assignment converts to an absolute path automatically |
| `Language` | `string` | read/write | UI language, spliced into the command line as `-lang` |
| `IsRunning` | `bool` | read-only | Whether the process most recently launched by this object is still running (a `HasExited` exception is treated as exited) |
| `LastExitCode` | `int?` | read-only | Exit code of the last run; `null` while the current run has not finished, written as soon as the process exits |

### Methods

| Method | Returns | Description |
|---|---|---|
| `Launch(AlyUpdateUiMode mode)` | `AlyResponse` | Launches by mode; non-blocking |
| `LaunchAuto()` | `AlyResponse` | Equivalent to `Launch(AlyUpdateUiMode.Auto)` |
| `LaunchCheck()` | `AlyResponse` | Equivalent to `Launch(AlyUpdateUiMode.Check)` |
| `LaunchUpdate()` | `AlyResponse` | Equivalent to `Launch(AlyUpdateUiMode.Update)` |
| `LaunchRollback()` | `AlyResponse` | Equivalent to `Launch(AlyUpdateUiMode.Rollback)` |
| `LaunchSilent()` | `AlyResponse` | Equivalent to `Launch(AlyUpdateUiMode.Silent)` |
| `static ToArgument(AlyUpdateUiMode mode)` | `string` | Enum to command-line value: `auto` / `check` / `update` / `rollback` / `silent`; unknown values fall back to `auto` |
| `Dispose()` | — | Unsubscribes events and releases the `Process` handle; idempotent; **does not close** a running window |

`Launch` execution order:

1. Already disposed → returns `NG("AlyUpdateUIClient has been disposed.")`.
2. `UiExePath` does not exist → returns `NG("aly-client-ui.exe not found: <path> (confirm that UpdateFolder is deployed; set AlyIncludeClientUi=true at build time to download it automatically)")` (the parenthetical is a legacy message from an older release; see the deployment note below).
3. The process held by this object is still running → returns `OK()` directly without launching again.
4. Releases the previous process handle and starts with `Arguments = "-mode <mode> -lang <lang>"`, `WorkingDirectory = directory containing the UI`, `CreateNoWindow = true`; on success resets `LastExitCode = null` and fires `Started(pid)`.
5. `Process.Start()` returns `false` → disposes the newly created handle and returns `NG("failed to start aly-client-ui.exe")`; an exception is thrown → returns `NG(ex)`.

### Events

| Event | Signature | When it fires |
|---|---|---|
| `Started` | `event Action<int>` | After the process launches successfully; the argument is the PID |
| `Exited` | `event Action<int>` | After the process exits; the argument is the exit code (treated as 0 if reading `ExitCode` throws); the handle is already released before it fires |

### aly-client-ui.exe Dependencies and Deployment

`AlyUpdateUIClient` **requires** `aly-client-ui.exe`. By default it lives in the same directory as `aly-client.exe` (both in `UpdateFolder/`): at startup the UI first looks for the updater in its own directory and, failing that, falls back to `aly-client.exe` in the main application directory recorded in `client.json`. The UI is a standalone single-instance program responsible for the window, download progress and rollback version selection, and it restarts the main program according to `client.json` after `apply_update`.

The binaries come from the **AlyPublish package**: when initializing a project the GUI copies both files
from `client/<rid>/` into the sibling `UpdateFolder/`, or you can place them by hand following the payload
layout in [Download & Install](/en/docs/download). The SDK targets now only create the directory and write a
default `client.json` when missing — they **no longer download anything**. If `aly-client-ui.exe` is
missing, add it from the package as described above.

Single-instance forwarding: if the window was already started by another route (the SDK holds no handle to that process), `Launch` still starts a new process; the new process forwards the "manual check request" to the existing instance through a named event and then exits immediately with code 0 — so "the window count did not increase" is not a failure. Only manual modes such as `check`/`update` forward the request and upgrade the existing instance's interaction state; `auto`/`silent` instances handle a forwarded request with interactive semantics.

## Enums

### `AlyClientStatus`

```csharp
public enum AlyClientStatus
{
    None,
    DiscoveredUpdate,
    DownloadingUpdate,
    DownloadedUpdate,
    ApplyUpdate,
}
```

| Value | Meaning |
|---|---|
| `None` | Idle / no pending update. Also used for "status cleared" — when the network is down and the client cannot verify, it returns `has_update=false`, and the SDK resets any leftover status to `None` |
| `DiscoveredUpdate` | A new version was discovered on the server |
| `DownloadingUpdate` | Changed files are downloading; `StatusChanged` messages carry the file name and percentage |
| `DownloadedUpdate` | Download complete, waiting to be applied |
| `ApplyUpdate` | Atomic replacement and host restart in progress |

### `AlyUpdateUiMode`

```csharp
public enum AlyUpdateUiMode
{
    Auto,
    Check,
    Update,
    Rollback,
    Silent,
}
```

| Value | Command line | Behavior |
|---|---|---|
| `Auto` | `-mode auto` | Silent check at startup: the window is hidden by default and shown only when an update is confirmed; exits silently when there is no update or the server is unreachable |
| `Check` | `-mode check` | Interactive check: shows "Checking" first; on no update or error it still notifies and closes automatically, so that clicking it never appears to do nothing |
| `Update` | `-mode update` | Downloads + applies directly, without asking again |
| `Rollback` | `-mode rollback` | Shows the list of rollback-able versions for the user to choose in the window |
| `Silent` | `-mode silent` | Silent check with the same semantics as `Auto`: no update/error stays unobtrusive, and the window appears only once an update is confirmed |

## Responses and Data Models

### `AlyResponse` (`AlyClient.CSharpSDK.Models`)

Corresponds to the outer stdout JSON from `aly-client` (non-generic version, used by commands without `data`).

| Member | Type | JSON field | Description |
|---|---|---|---|
| `IsSuccess` | `bool` | `isSuccess` | Whether the command succeeded |
| `ErrorMsg` | `string` | `errorMsg` | Failure reason |
| `static OK()` | `AlyResponse` | — | Constructs a success response |
| `static NG(string errMsg)` | `AlyResponse` | — | Constructs a failure response |
| `static NG(Exception e)` | `AlyResponse` | — | Constructs a failure response from `e.Message` |

### `AlyResponse<T>`

Inherits `AlyResponse` and adds a strongly typed payload.

| Member | Type | JSON field | Description |
|---|---|---|---|
| `Data` | `T` | `data` | Command payload |
| `static OK(T data)` | `AlyResponse<T>` | — | Constructs a success response |
| `static new NG(string errMsg)` | `AlyResponse<T>` | — | Constructs a failure response |
| `static new NG(Exception e)` | `AlyResponse<T>` | — | Constructs a failure response from `e.Message` |

### `CheckUpdateData` (`AlyClient.CSharpSDK`)

| Member | Type | JSON field | Description |
|---|---|---|---|
| `HasUpdate` | `bool` | `has_update` | Whether a new version exists. `false` when the server is unreachable |
| `NeedDownloadUpdate` | `bool` | `need_download_update` | Whether a download is needed (`false` when already downloaded and waiting to apply) |
| `CurrentVersion` | `string` | `current_version` | Current version |
| `NewVersion` | `string` | `new_version` | New version |
| `ForceUpdate` | `bool` | `force_update` | Force update: when `true`, confirmation events are not fired and the update is downloaded and applied directly |
| `ReleaseNotes` | `string[]` | `release_notes` | Server changelog, one item per entry; `null` when the server has none configured; delivered only when a new version is reported and a download is needed |

### `CheckSelfUpdateData` (`AlyClient.CSharpSDK`)

| Member | Type | JSON field | Description |
|---|---|---|---|
| `NeedUpdate` | `bool` | `need_update` | Whether the updater itself needs an update |

### `SelfUpdateData` (`AlyClient.CSharpSDK.Models`)

| Member | Type | JSON field | Description |
|---|---|---|---|
| `Updated` | `bool` | `updated` | Whether a replacement actually happened |
| `Reason` | `string` | `reason` | Skip reason when `updated=false`: `source_missing` / `already_up_to_date` |
| `OldPath` | `string` | `old_path` | Path of the old-version backup (`.old`) that was replaced when `updated=true` |

### `ListRollbackData` (`AlyClient.CSharpSDK`)

| Member | Type | JSON field | Description |
|---|---|---|---|
| `CurrentVersion` | `string` | `current_version` | Current version |
| `Versions` | `string[]` | `versions` | List of rollback-able version numbers |

### `RollbackData` (`AlyClient.CSharpSDK`)

| Member | Type | JSON field | Description |
|---|---|---|---|
| `Version` | `string` | `version` | The version actually rolled back to |

### `DownloadProgressData` (`internal`)

Raw structure of a download progress line (generated from `spec/aly-protocol.schema.json`); hosts cannot reference it directly and only use it indirectly through the `DownloadUpdateAsync` callback.

| Member | Type | JSON field | Description |
|---|---|---|---|
| `Index` | `long` | `index` | Current file index |
| `Total` | `long` | `total` | Total diff files (only files that need downloading) |
| `File` | `string` | `file` | Relative path of the current file, passed through to the progress callback |
| `Status` | `string` | `status` | Line type: `BEGIN` / `START` / `PROGRESS` / `DONE` / `SKIP` / `FAIL` (the SDK only reports progress for `DONE` lines with `Total > 0`) |
| `FileSize` | `double` | `file_size` | Size of the current file in bytes |
| `Error` | `string` | `error` | Failure reason for this line (only on `FAIL` lines) |
| `DownloadedBytes` | `long` | `downloaded_bytes` | Bytes downloaded so far (cumulative, for byte-level progress) |
| `TotalBytes` | `long` | `total_bytes` | Total bytes of the diff files (announced on the `BEGIN` line) |
| `SpeedBps` | `long` | `speed_bps` | Average throughput of the recent window (bytes/s) |

The progress callback still reports at **file-count** granularity (`Index / Total`) and only counts `DONE` lines, so a single large file produces only one progress jump from start to finish. The byte-level fields are now part of the model (`internal`) but `DownloadUpdateAsync` does not expose them through the callback yet; parse `aly-client`'s stdout directly if you need byte-level progress.

When the client emits a completion line with `data:null`, `DownloadUpdateAsync` returns success.

## Threading Model and Error Handling

| Concern | Behavior |
|---|---|
| `AlyUpdateClient` thread | One background thread per instance (`Task.Factory.StartNew`); every `AlyApi` call inside the loop is a blocking `.Result` |
| Event thread | `StatusChanged`, `RequestDownloadUpdate`, `RequestApplyUpdate` and `ErrorStatusChanged` all fire synchronously on that background thread |
| `AlyUpdateUIClient` thread | `Launch` runs on the calling thread; `Exited` is fired by the `Process.Exited` callback thread |
| `AlyApi` thread | `Task` runs on the thread pool, and `.Result` blocks the calling thread; do not wait on the main UI thread for commands that take 30 seconds or more |
| Exception propagation | `AlyApi` and `AlyUpdateUIClient` do not throw business exceptions to callers: process failures, timeouts and deserialization errors all become `IsSuccess=false` + `ErrorMsg`; event-handler exceptions are written to `aly-client-sdk.log` |
| Logging | `AlyUpdateClient` writes self-update results and event-handler exceptions to `aly-client-sdk.log` in the host base directory; `aly-client`'s own log is `UpdateFolder/update.log` |

Related documentation: [Integration Guide](/en/docs/sdk), [Command Reference](/en/docs/commands), [Client Configuration](/en/docs/client/config), [Self-Update Design](/en/docs/self-update).