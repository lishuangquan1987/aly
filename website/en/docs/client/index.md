---
title: Updater Overview and Integration
description: What aly-client does, the deployment layout, the version lifecycle, host integration (child process JSON and the C# SDK), and the security and rollback model.
---

# Updater Overview and Integration

`aly-client` is an updater that runs on endpoint machines. It does not stay resident and installs no service; the host application launches it when needed. Each run executes one subcommand and writes the result to stdout as JSON. The actual version switch is performed by local directory renames — no network is involved, and the main program does not participate in replacing files.

Optional components:

| Component | Role |
|---|---|
| `aly-client` / `aly-client.exe` | The updater itself: check, download, apply, roll back, self-update |
| `aly-client-ui` / `aly-client-ui.exe` | Optional bottom-right GUI that wraps `aly-client` commands, shows progress, and asks the user |
| `AlyClient.CSharpSDK` | Wrapper for .NET hosts: background state machine, events, static API, GUI launcher |
| `aly-server` | Holds project metadata and file manifests, and serves the REST API |
| `aly-publish` | Publisher-side tool: scan, stage, upload and publish versions |

Prebuilt updaters are no longer published as standalone assets: take them from `client/<rid>/` inside the
AlyPublish package (the GUI copies them into `UpdateFolder/` when initializing a project). Click a file
name to download the package:

<ReleaseDownloads group="package" compact :show-latest-hint="false" :show-version="false" />

The updater is only responsible for turning the local files into the new version published by the server; it does not decide when to prompt the user or whether an update is forced. Prompting policy belongs to the host or the GUI.

## Host Lifecycle: exit together with the host

`aly-client` and `aly-client-ui` are both child processes launched by the host. **They exit when the host exits** and never stay in the process table after the user closes the main program.

The only exception is the **apply stage**: the updater has to close the host process, atomically replace the main directory and restart the main program, so the host is going to (and must) disappear anyway — the updater has to finish that step instead of exiting halfway. Two conditions qualify, either is enough:

- Command level: the current command is `apply_update`;
- State level: `version_status == applying` in `UpdateFolder/version.json` (the crash-recovery window — no command is running that time, but the main directory may already be gone, so it must resume in the same way).

Exit is a graceful shutdown: write the log first, then exit. The update lock needs no explicit release — a lock file is judged stale by "the owning process has exited", and the next run cleans up any leftover lock automatically. The normative text is `spec/host-contract.md` §15.

## Deployment Layout

The endpoint deployment layout from the README looks like this (Windows file names; on Linux there is no `.exe`):

```text
PackageFolder/
├── ApplicationFolder/                 # Currently running directory (MainFolder)
│   ├── YourApp.exe
│   └── .updator/
│       └── shared.json                # server_url, project_name, rules
├── ApplicationFolder_1.0.0/           # Historical version snapshot (rollback-able)
├── ApplicationFolder_1.0.1/           # Downloaded version waiting for apply (download output)
└── UpdateFolder/                      # Updater directory
    ├── aly-client.exe                 # The updater itself
    ├── aly-client-ui.exe              # Optional GUI
    ├── client.json                    # Main program path, processes to close
    ├── version.json                   # Version state machine
    ├── .aly.lock                      # Update mutex (at runtime)
    ├── update.log                     # Update log (rotated to update.log.1 above 5 MB)
├── download.log                   # Download, materialization and deletion-sync log (same 5 MB single-generation rotation)
├── check_diff.log                 # Local MD5 scan warnings (same 5 MB single-generation rotation)
    ├── release_manifest.json          # Release manifest cache (used by delete sync)
    └── aly-client.exe.old             # Self-update backup (at runtime)
```

Key paths:

| Name | Location | Description |
|---|---|---|
| `UpdateFolder` | `PackageFolder/UpdateFolder` | Directory holding the updater; also where `client.json` / `version.json` live |
| `MainFolder` | Derived from `main_exe_relative_path` in `client.json` | Current version directory; with the default layout it is `PackageFolder/ApplicationFolder` |
| `PackageFolder` | Parent directory of `UpdateFolder` | Holds version snapshots and pending directories |
| Version directory | `PackageFolder/ApplicationFolder_{version}` | A download output or a historical snapshot |
| Shared config | `MainFolder/.updator/shared.json` | Shipped inside the version package; the client reads it from the current main directory |

`main_exe_relative_path` is **relative to `UpdateFolder`**, for example:

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

Here `MainFolder` is `../ApplicationFolder`. For the fields and rules see the [configuration reference](/en/docs/client/config).

## Source Modules

| Directory | Description |
|---|---|
| `client/aly-client/` | The updater itself: Go 1.10, 32-bit (`GOARCH=386`), Windows XP compatible, GOPATH mode |
| `client/aly-client-ui/aly-client-winui/` | Windows GUI: Go 1.10.8 + `lxn/walk` |
| `client/aly-client-ui/aly-client-linuxui/` | Linux GUI: Go 1.27 + the MyGo native UI, with a runtime dependency on GTK3 |
| `client/aly-client-sdk/` | C# SDK (`net40` + `netstandard2.0`) |

Build and cross-platform details are in [Build & cross-platform](/en/docs/build).

## Lifecycle

```text
applied/empty ──download_update──> downloaded ──apply_update──> applying ──success──> applied
                                         ▲                          │
                                         └──────── failure/crash ────┘

applied ──rollback --version X──> applying ──success──> applied (version = X)
```

| Stage | Command | What happens |
|---|---|---|
| Check | `check_update` | Compares the highest server version with the local version and returns `has_update` / `need_download_update`; read-only |
| Download | `download_update` | Downloads only the differing files, verifying each with MD5 + SHA256; materializes unchanged files from the main directory into the new version directory; writes `downloaded` |
| Apply | `apply_update` | Validates the download output, closes the business processes, completes the replacement with three renames, writes `applied`, runs the post-update script and starts the main program |
| Rollback | `rollback` | Atomically replaces the current directory with a historical snapshot; no network needed |
| Self-update | `self_update` | Replaces the running copy in `UpdateFolder` with the newer updater distributed with the version in the main directory |
| Cleanup | `clean` | Deletes snapshots beyond the retention count, temporary files, and expired logs |

A few key points:

- **Materialization happens during the download stage.** `download_update` copies the unchanged files from the current main directory into the pending directory (excluding `un_copy_*`), so `apply_update` only has to rename. The application downtime window drops from minutes of "copying the whole package" to seconds.
- **Crashes are recoverable.** `apply_update` writes `applying` before entering the replacement and writes `apply_stage=activating` before the final activation rename. The next `check_update` / `apply_update` uses these to tell whether the rename already happened and to finish the remaining steps.
- **Going offline does not lose state.** In the `downloaded` state, if the server is unreachable, `check_update` does not report a pending update, but the downloaded content and `version.json` stay unchanged. `applying` (crash recovery / rollback resumption) is the exception and is still reported while offline; otherwise the machine could not heal itself.
- **A failed download does not write the pending state.** When a single file still fails after 3 retries, the final result carries `failed_files` and `version.json` remains in its previous state; a retry only downloads the failed files again.

For the full state fields see [version.json](/en/docs/client/config#version-json); for command details see the [command reference](/en/docs/commands).

## Host Integration

### Option 1: Child Process + JSON

The host starts `UpdateFolder/aly-client.exe` directly and reads stdout line by line. Every command prints at least one line of JSON; `download_update` prints several progress lines first and only the last line is the final result.

Recommended sequence:

```text
Startup
 ├─ self_update when needed (bare console deployment)
 ├─ check_update
 │   ├─ has_update=false            → done
 │   ├─ need_download_update=true   → download_update → apply_update
 │   └─ need_download_update=false  → apply_update (crash recovery / downloaded and pending)
 └─ Exit
```

Minimal C# call example:

```csharp
using System.Diagnostics;
using Newtonsoft.Json.Linq;

var psi = new ProcessStartInfo(updaterExePath, "check_update")
{
    UseShellExecute = false,
    RedirectStandardOutput = true,
    CreateNoWindow = true,
};

using (var p = Process.Start(psi))
{
    string line;
    while ((line = p.StandardOutput.ReadLine()) != null)
    {
        var msg = JObject.Parse(line);
        var data = msg["data"] as JObject;

        // Progress line from download_update: keep reading until the final result
        if (data != null && data["status"] != null)
            continue;

        if ((bool)msg["isSuccess"])
        {
            // Read data.has_update / need_download_update / new_version ...
        }
        else
        {
            // Business failure: read msg["errorMsg"], not the exit code
        }
    }
}
```

Integration points:

- **Trust `isSuccess`, never the exit code.** Business failures (a failed download verification, no pending update, a lock held by another process, and so on) also end with exit code `0`. Only an unknown or missing subcommand returns `1`, and a parameter parsing failure returns `2`.
- **Progress lines are not the final result.** The `BEGIN` / `START` / `PROGRESS` / `DONE` / `FAIL` lines of `download_update` all carry `data.status`; the final completion line has `data` set to `null`. Only `data:null` means the download is finished.
- **Do not splice parameters into `cmd.exe`.** Pass arguments to `ProcessStartInfo` directly to avoid quoting, spacing, and `%` expansion problems.
- **The updater reads its own configuration.** Hosts normally do not need to pass `--url` / `--project-name`; `client.json` and `shared.json` must be in place.
- **Closing processes.** Put the main program's process name into `must_close_process_name` in `client.json`, or pass it through the SDK's `MustCloseProcessNames`; otherwise, when a rename fails, the updater probes for and kills the holder, and the host never gets a chance to shut down gracefully.
- If you need a graphical interface, use `aly-client-ui -mode auto` instead of drawing your own UI; see the [GUI front-end](/en/docs/client/ui).

### Option 2: C# SDK

| Type | Purpose |
|---|---|
| `AlyUpdateClient` | Runs `check → download → apply` in a background loop and drives the host UI through `StatusChanged` / `RequestDownloadUpdate` / `RequestApplyUpdate` |
| `AlyApi` | Static methods called directly, one per `aly-client` command; suits hosts that want to control the timing themselves |
| `AlyUpdateUIClient` | Launches `aly-client-ui` and hands the whole lifecycle plus the restart to the GUI |

During the build, the SDK's NuGet targets copy `aly-client.exe` and `client.json` to `..\UpdateFolder\`, optionally along with `aly-client-ui.exe`. At startup `AlyUpdateClient` also delegates `self_update`, so the updater copy never falls behind.

For full usage see [C# SDK integration](/en/docs/sdk); for method-level parameters see the [SDK API reference](/en/docs/sdk/api).

## Command Index

| Command | Purpose | Reference |
|---|---|---|
| `check_update` | Check whether a new version exists | [Command reference](/en/docs/commands#check-update) |
| `check_diff` | List the differing files that need downloading | [Command reference](/en/docs/commands#check-diff) |
| `download_update` | Download differing files and materialize unchanged ones | [Command reference](/en/docs/commands#download-update) |
| `apply_update` | Atomically replace the main directory and restart the main program | [Command reference](/en/docs/commands#apply-update) |
| `list_rollback_versions` | List rollback-able versions | [Command reference](/en/docs/commands#list-rollback-versions) |
| `rollback` | Roll back to a specified version | [Command reference](/en/docs/commands#rollback) |
| `clean` | Clean up snapshots and temporary files | [Command reference](/en/docs/commands#clean) |
| `check_self_update` | Check whether the updater itself needs updating | [Command reference](/en/docs/commands#check-self-update) |
| `self_update` | Replace itself with the newer updater from the main directory | [Command reference](/en/docs/commands#self-update) |

For configuration fields see the [configuration reference](/en/docs/client/config); for GUI modes and parameters see the [GUI front-end](/en/docs/client/ui).

## Security and Rollback

### Security Boundaries

- The server REST API **has no authentication**. Deploy it on a trusted intranet or behind a gateway; never expose it directly to the public internet.
- The updater has write access to `MainFolder` and `PackageFolder`; restrict ordinary users' write access to these directories on the endpoint so version directories or `version.json` cannot be tampered with locally.
- Downloaded files are verified against the MD5 + SHA256 in the server manifest, but the manifest itself comes from the server; the security model is "trust your own server", not end-to-end signing.
- The global lock `.aly.lock` prevents concurrent download/apply/rollback/cleanup on the same endpoint. Do not delete the lock manually unless you are certain no update process is running and the lock is stale (30-minute TTL, 5-minute heartbeat).
- Do not edit `version.json` by hand. A mismatch between the state and the disk content makes the updater refuse to replace; the only repair is rewriting the state through `download_update`.

### Rollback

- Every successful apply renames the old main directory to an `ApplicationFolder_{version_previous}` snapshot; `rollback --version` swaps directories directly and needs no network.
- The `apply_update` / `rollback` entry points opportunistically keep the 3 most recent snapshots; `clean` reclaims them explicitly. The current version, the previous version and the rollback target are never cleaned up.
- When `version_previous` is empty (the first update), an unversioned backup `ApplicationFolder_` is created; it is deleted after success and never appears in the rollback list.
- Rollback is not "version pinning": if the server version is still higher after a successful rollback, the next `check_update` upgrades again. There is no rollback pin policy yet, so operations must unpublish or adjust the server version in parallel.
- A rollback failure after entering the replacement stage restores the pre-rollback state and starts the current version of the main program if the main directory still exists, so a machine is never left in the mismatched state of "state applied, disk still old".

For more design notes see [Self-update design](/en/docs/self-update) and [Core concepts](/en/docs/concepts).