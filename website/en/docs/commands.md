---
title: aly-client Command Reference
description: Complete reference for the nine aly-client updater commands, with purpose, signature, parameters, return fields, JSON examples, side effects and error boundaries.
---

# aly-client Command Reference

`aly-client` is the updater deployed under `UpdateFolder` on the endpoint machine. Each process run executes one subcommand, and every result is written to stdout as JSON. This page documents the signature, parameters, return fields, examples and side effects of each command. For the deployment layout and host integration, see the [updater overview](/en/docs/client/); for configuration fields, see the [configuration reference](/en/docs/client/config).

The examples below use the Windows file name `aly-client.exe` throughout. On Linux the executable is named `aly-client`; subcommands and parameters are identical.

> **Who this is for**: integration developers who have completed Quick Start and need parameters, return fields and failure boundaries. After this page you can orchestrate all nine commands without trial and error.

::: tip Output language
The CLI currently emits `errorMsg` and release-note strings in Chinese. The JSON examples preserve that real output; field names and the schema are the stable contract.
:::

## Command Overview

Use this table to locate a command, then jump to its section. Protocol details, exit codes and locking follow under Common Conventions.

| Command | Purpose | Network | Takes global lock | Writes version.json | Typical caller |
|---|---|---|---|---|---|
| [`check_update`](#check-update) | Check whether the server has a new version | Yes | No | No | Host, GUI, SDK |
| [`check_diff`](#check-diff) | List the differing files that need downloading | Yes | No | No | Debugging, visual client |
| [`download_update`](#download-update) | Download differing files and materialize unchanged ones | Yes | Yes | Yes | Host, GUI, SDK |
| [`apply_update`](#apply-update) | Atomically replace the main directory and restart the main program | No | Yes | Yes | Host, GUI, SDK |
| [`list_rollback_versions`](#list-rollback-versions) | List historical snapshots available for rollback | No | No | No | GUI, SDK, operations |
| [`rollback`](#rollback) | Roll back to a specified historical version | No | Yes | Yes | GUI, SDK, operations |
| [`clean`](#clean) | Clean up historical snapshots and temporary files | No | Yes | No | Operations, scheduled tasks |
| [`check_self_update`](#check-self-update) | Check whether the updater itself needs updating | No | No | No | SDK, GUI, host |
| [`self_update`](#self-update) | Replace itself with the newer updater from the main program directory | No | Yes | No | SDK, GUI, bare console host |

## Common Conventions

### Output Envelope

```json
{"isSuccess":true,"errorMsg":"","data":{}}
```

| Field | Type | Description |
|---|---|---|
| `isSuccess` | bool | Whether the operation succeeded. **Hosts must use this field to decide the outcome** |
| `errorMsg` | string | Failure reason; an empty string on success |
| `data` | any | Command payload; `null` when there is no data |

Apart from `download_update`, every command prints a single line of JSON. `download_update` prints several progress objects first and only the last line is the final result; see [Download Progress](#download-progress).

### Exit Codes

| Exit code | Meaning |
|---|---|
| `0` | The command finished running, **including business failures where `isSuccess=false`** |
| `1` | No subcommand or an unknown subcommand; or stdout JSON serialization failed |
| `2` | Parameter parsing failed (Go `flag.ExitOnError`), for example an invalid `--close-timeout` or an unknown option |

::: warning
Do not use the exit code to judge whether an operation succeeded. A failed download, no pending update, or a lock held by another process all end with exit code `0` and are only reflected in `isSuccess` and `errorMsg` in the JSON. A host that only looks at the exit code will treat failures as successes.
:::

### Configuration Loading and the Global Lock

Configuration is loaded in a fixed order:

1. `UpdateFolder/client.json`: `main_exe_relative_path` and `must_close_process_name`;
2. the main program directory `MainFolder`, derived from `main_exe_relative_path`;
3. `MainFolder/.updator/shared.json`: `server_url`, `project_name`, and the ignore / no-copy rules;
4. command-line overrides (`--url`, `--project-name`, `--main-exe-path`).

The commands that need server configuration are `check_update`, `check_diff` and `download_update`. A missing `server_url` returns `no server url configured`, and a missing `project_name` returns `no project name configured`. `apply_update`, `rollback`, `list_rollback_versions`, `clean`, `check_self_update` and `self_update` only touch local files and need no network access.

The global update lock lives at `UpdateFolder/.aly.lock`:

| Commands that take the lock | Commands that do not |
|---|---|
| `download_update`, `apply_update`, `rollback`, `clean`, `self_update` | `check_update`, `check_diff`, `list_rollback_versions`, `check_self_update` |

When another live updater process holds the lock, the command prints:

```json
{"isSuccess":false,"errorMsg":"另一更新正在进行中，请稍后重试","data":null}
```

The lock has a 30-minute TTL and a 5-minute heartbeat; if the holder crashes, the next acquisition cleans up the stale lock.

### The version.json State Machine

There are only three valid version states (an empty state is equivalent to `applied`):

```text
applied/empty ──download_update──> downloaded ──apply_update──> applying ──success──> applied
                                         ▲                          │
                                         └──────── failure/crash ────┘
```

`version_status=applying` means an update or a rollback is in flight; the next `check_update` / `apply_update` goes through crash recovery. For field meanings see [version.json](/en/docs/client/config#version-json).

## check_update

### Purpose

Query the server for the latest version and report whether a download is needed or whether a downloaded update is already waiting to be applied. The command is read-only: it downloads nothing and rewrites no files, so hosts may call it frequently (for example at startup and from scheduled tasks).

### Signature

```bash
aly-client.exe check_update [--url <server_url>] [--project-name <project_name>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--url` | string | No | Empty; falls back to `server_url` in `shared.json` | Temporarily override the server address for this call |
| `--project-name` | string | No | Empty; falls back to `project_name` in `shared.json` | Temporarily override the project name for this call |

### Behavior

1. Read `client.json`, `shared.json` and `version.json`.
2. Branch on `version_status`:
   - `applied` or empty: request project info and the changelog, take the highest version and compare it with the server version; only report an update when the server version is strictly higher.
   - `downloaded`: if `rollback_previous` is non-empty (an interrupted rollback), report that the rollback should continue; otherwise request the changelog; if the server version is higher, require a fresh download; if the server version is the same and the pending artifacts are missing or incomplete, require a re-download to fill them in; if the versions match and the artifacts are intact, or the server version is lower, require continuing with the apply.
   - `applying`: unconditionally report that the apply should continue (crash recovery) and skip the network comparison.
   - Any other unknown value: print a warning to stderr and treat it as `applied`; only `download_update` can rewrite it to `downloaded`.
3. The latest changelog entry is the record with the highest version number; when versions are equal, the record with the largest `id` wins.
4. Version comparison is semver-aware: `1.0.0-beta < 1.0.0`, and build metadata such as `+build` does not participate in the comparison; the legacy plain-numeric format keeps its previous behavior.

### Return Fields

`data` fields:

| Field | Type | Present when | Description |
|---|---|---|---|
| `has_update` | bool | Always | Whether there is a new version to handle or a pending update to apply |
| `need_download_update` | bool | Always | `true` means `download_update` must run first; `false` means only `apply_update` is needed |
| `current_version` | string | Always | Version of the actual content in the main directory; `version_previous` while in the `downloaded` state; empty on first deployment or when unknown; falls back to `version` when `applying` and `version_previous` is empty |
| `new_version` | string | When a new version is reported | Target version; for a resumed rollback this is the rollback target |
| `force_update` | bool | On branches that require a download | The server project's forced-update flag; `false` when project info could not be read |
| `release_notes` | string[] | When the server reports a higher version and the changelog is non-empty | Passed through verbatim from the server's `project_change_logs.logs`, neither translated nor trimmed |

`release_notes` only appears when a "higher version requiring download" is reported; it is absent when offline, during `applying` crash recovery, during a resumed rollback, and when there is no update. Hosts and the GUI use it to show what changed in this update.

### Examples

An update is available and must be downloaded:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "has_update": true,
    "need_download_update": true,
    "current_version": "1.0.0",
    "new_version": "1.0.1",
    "force_update": false,
    "release_notes": ["修复：测长偶发偏差", "新增：点间距支持 2.5m"]
  }
}
```

Already downloaded, only the apply remains:

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}
```

No update:

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":false,"need_download_update":false,"current_version":"1.0.0"}}
```

Download artifacts exist but the network is down (the pending update is not reported and the artifacts are kept):

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":false,"need_download_update":false,"current_version":"1.0.0"}}
```

`applying` crash recovery (must be reported even when offline):

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}
```

Failure:

```json
{"isSuccess":false,"errorMsg":"no server url configured","data":null}
```

### Side Effects

- Writes no `version.json`, creates no directories, and takes no global lock.
- An unknown `version_status` is written to stderr only, never to `update.log`, so frequent calls cannot flood the log.
- Network failures also produce diagnostics on stderr, but the command still returns `isSuccess=true` with `has_update=false` to the caller.

### Boundaries and Errors

- **Offline semantics**: in the `downloaded` state, when the server is unreachable (or there is no changelog at all) the pending update is not reported and the command returns `has_update=false`; the downloaded content and `version.json` are left untouched, and applying continues automatically once the network returns. This avoids showing "update available" while offline and then changing state after reconnecting.
- **The `applying` exception**: crash recovery and rollback resumption must still be reported while offline, otherwise a machine whose main directory has already been renamed, or whose rollback was interrupted, can never heal itself.
- **Missing artifacts**: when the state is `downloaded` but the pending directory is missing, empty, or holds fewer files than `downloaded_file_count`, the command returns `has_update=true` and `need_download_update=true` if the server version is exactly the same, so the artifacts are re-downloaded and filled in; if the server version is higher, the higher version is downloaded as usual; if the server version is lower, continuing the apply is still reported, and `apply_update` rejects it and self-heals.
- **Anomalous state**: when `version_previous == version`, the pending update is not reported, so the host does not repeatedly trigger an apply that is bound to be rejected.
- **Circuit breaker**: after 3 consecutive apply failures for the same pending version, the pending update is no longer reported; the counter resets on a successful download, a successful apply, or when the server publishes a higher version that is downloaded successfully.
- **Downloaded but the server version is lower**: the local, already-downloaded higher version is still applied, meaning a server-side "unpublish" cannot stop an update that has already been downloaded; this is a known limit of the current policy.
- Business failures (no address configured, `shared.json` read failure, and so on) still end with exit code `0`; hosts must read `isSuccess`.

### Related Commands

[`download_update`](#download-update) · [`apply_update`](#apply-update) · [Updater overview](/en/docs/client/) · [Configuration reference](/en/docs/client/config)

## check_diff

### Purpose

List the files that exist on the server but are missing locally or differ by MD5, so the GUI or a troubleshooting session can see which files this update would download. The command downloads nothing and rewrites no files.

### Signature

```bash
aly-client.exe check_diff [--url <server_url>] [--project-name <project_name>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--url` | string | No | Empty; falls back to `server_url` in `shared.json` | Temporarily override the server address for this call |
| `--project-name` | string | No | Empty; falls back to `project_name` in `shared.json` | Temporarily override the project name for this call |

### Behavior

1. Call `GET /api/file/get_all_files/{projectName}` to fetch the server-side file manifest (including MD5 and SHA256).
2. Recursively scan `MainFolder` and compute the MD5 of each local file.
3. Compare each server file one by one:
   - missing locally: add to the diff list with an empty `local_md5` and `local_size` of `0`;
   - present locally but with a different MD5: add to the diff list and compute the local SHA256 in parallel;
   - identical MD5: skip.
4. Make a best-effort attempt to fetch the latest changelog and fill `new_version` with the highest version.

Only files that exist on the server and are missing or different locally are returned. **Files that exist only locally, already deleted on the server, are never returned**.

### Return Fields

| Field | Type | Present when | Description |
|---|---|---|---|
| `new_version` | string | Always | Latest server version; an empty string when the changelog is unavailable |
| `files` | array \| null | Always | List of differing files; `null` when there is no difference |

Elements of `files`:

| Field | Type | Present when | Description |
|---|---|---|---|
| `path` | string | Always | Path relative to `MainFolder`, always with forward slashes |
| `local_md5` | string | Always | Local MD5; empty string when the file is missing locally |
| `local_size` | int | Always | Local file size; `0` when the file is missing locally |
| `local_sha256` | string | File exists locally with a different MD5 | Local SHA256; empty string when the file is missing locally |
| `server_md5` | string | Always | Server-side MD5 |
| `server_size` | int | Always | Server-side file size |
| `server_sha256` | string | Always | Server-side SHA256 |

### Examples

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "new_version": "2.0.0",
    "files": [
      {
        "path": "app.exe",
        "local_md5": "abc123",
        "local_size": 43284,
        "local_sha256": "849384",
        "server_md5": "def456",
        "server_size": 5242880,
        "server_sha256": "884374"
      },
      {
        "path": "plugins/new.dll",
        "local_md5": "",
        "local_size": 0,
        "local_sha256": "",
        "server_md5": "xyz789",
        "server_size": 102400,
        "server_sha256": "88437486"
      }
    ]
  }
}
```

No differences:

```json
{"isSuccess":true,"errorMsg":"","data":{"new_version":"2.0.0","files":null}}
```

Failure:

```json
{"isSuccess":false,"errorMsg":"get server files: ...","data":null}
```

### Side Effects

- Takes no global lock, writes no `version.json`, and downloads no files.
- Warnings raised while scanning local MD5s are appended to `UpdateFolder/check_diff.log`.
- Server endpoints are accessed read-only.

### Boundaries and Errors

- Returns a failure when `server_url` / `project_name` is missing.
- A failed request for the server file manifest fails the whole command; a failed changelog request does not block it, and `new_version` is left as an empty string.
- Comparison is based on MD5; SHA256 is returned as supplementary information only.
- With no differences, `files` is `null` rather than an empty array, so hosts must tolerate both.
- Business failures still end with exit code `0`.

### Related Commands

[`check_update`](#check-update) · [`download_update`](#download-update)

## download_update

### Purpose

Download the differing files into `PackageFolder/ApplicationFolder_{newVersion}/` without replacing the currently running program. Once the download completes, unchanged files from the main directory are materialized into the new version directory, leaving `apply_update` with nothing but renames to do.

### Signature

```bash
aly-client.exe download_update [--url <server_url>] [--project-name <project_name>] [--main-exe-path <relative_path>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--url` | string | No | Empty; falls back to `server_url` in `shared.json` | Temporarily override the server address for this call |
| `--project-name` | string | No | Empty; falls back to `project_name` in `shared.json` | Temporarily override the project name for this call |
| `--main-exe-path` | string | No | Empty; falls back to `main_exe_relative_path` in `client.json` | Temporarily override the main program path (relative to `UpdateFolder`) |

### Behavior

1. Take the global lock and read `client.json`, `shared.json` and `version.json`.
2. Fetch the highest server version; if `version_status=applying`, print a completion line with `data:null` and return without downloading or changing the state, leaving crash recovery to `apply_update`.
3. If the state is `downloaded` with the same version and the target directory exists and is non-empty, print a completion line with `data:null` and return; a missing or empty target directory counts as lost artifacts, so the same version may be downloaded again to fill it in.
4. If the target version is not higher than the current one and no re-download is needed, return `already at latest version`.
5. Fetch the server file manifest and compare it with the MD5s in the local `MainFolder`; files already in the target directory that pass MD5 + SHA256 verification are also skipped. Only files that really need downloading are counted in `total`.
6. Download file by file into `PackageFolder/ApplicationFolder_{newVersion}/`:
   - large files (over 100 MB) use a `.part` temporary file plus `Range` resumption;
   - small files are also written to `.part` first and renamed when complete;
   - every downloaded file is verified with MD5 + SHA256; a mismatch deletes the file and retries, up to 3 times.
7. Three failures on a single file do not abort the whole run: the file is recorded in the failure list and the next file is processed. When everything has run, the command returns one failure with the failed files listed in `failed_files`, and **does not write the pending-apply state to `version.json`**, so a half-finished package can never be activated by `apply_update`. On retry, already-successful files are skipped, which is equivalent to retrying only the failures.
8. After every file succeeds, finish with materialization: copy unchanged files from `MainFolder` into the version directory, excluding `un_copy_folders` / `un_copy_files`. A materialization failure fails the whole run.
9. Perform differential delete synchronization against the server release manifest (deletion set = previous cached manifest ∖ current manifest), removing files in the new version directory that the server has deleted; skip conservatively when the manifest or the cache is unavailable.
10. Write `version.json`: `version_status=downloaded`, recording `version`, `version_previous`, `downloaded_file_count` and `after_apply_update_script`, clearing the rollback / activation transition fields, and resetting `apply_fail_count` to 0.

### Download Progress

`download_update` writes multiple lines of JSON to stdout. The `data` field of a progress line:

| Field | Type | Description |
|---|---|---|
| `index` | int | Index of the current differing file; `0` on the `BEGIN` line |
| `total` | int | Total number of differing files (skipped files excluded) |
| `file` | string | Relative path; an empty string on the `BEGIN` line |
| `status` | string | `BEGIN` / `START` / `PROGRESS` / `DONE` / `FAIL` |
| `file_size` | int | Size of the current file in bytes |
| `downloaded_bytes` | int | Cumulative bytes downloaded; older clients may omit this field |
| `total_bytes` | int | Total bytes of all differing files; announced on the `BEGIN` line |
| `speed_bps` | int | Average throughput over the recent window (bytes/second) |
| `error` | string | Failure reason on a `FAIL` line |

`PROGRESS` lines are throttled to roughly every 3 seconds so that stdout stays active while a long file downloads and the host does not mistake the process for hung. A `FAIL` line carries `isSuccess=false`.

The final completion line has `data:null`. This is the signal the SDK and UI use to recognize "download finished"; do not treat it as an anomaly.

### Return Fields

Final result:

| Field | Type | Present when | Description |
|---|---|---|---|
| `isSuccess` | bool | Always | `true` when every file was downloaded and materialized successfully |
| `errorMsg` | string | Always | Failure reason; an empty string on success |
| `data` | object \| null | Always | `null` on success; an object when some files failed |
| `data.version` | string | When `data` is non-null | Target version |
| `data.failed_files` | string[] | When files failed | Relative paths of the failed files |

### Examples

Successful completion line:

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

Some files failed:

```json
{
  "isSuccess": false,
  "errorMsg": "下载完成但有 2 个文件失败: a.dll, b.dll",
  "data": {"version": "2.0.0", "failed_files": ["a.dll", "b.dll"]}
}
```

Generic failure (no address configured, no changelog, already up to date, and so on):

```json
{"isSuccess":false,"errorMsg":"already at latest version","data":null}
```

Sample progress lines:

```json
{"isSuccess":true,"errorMsg":"","data":{"index":0,"total":5,"file":"","status":"BEGIN","file_size":0,"downloaded_bytes":0,"total_bytes":734003200,"speed_bps":0}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":5,"file":"app.exe","status":"START","file_size":5242880,"downloaded_bytes":0,"total_bytes":734003200,"speed_bps":0}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":5,"file":"app.exe","status":"PROGRESS","file_size":5242880,"downloaded_bytes":2621440,"total_bytes":734003200,"speed_bps":1835008}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":5,"file":"app.exe","status":"DONE","file_size":5242880,"downloaded_bytes":5242880,"total_bytes":734003200,"speed_bps":2411724}}
{"isSuccess":true,"errorMsg":"","data":null}
```

### Side Effects

- Creates `PackageFolder/ApplicationFolder_{newVersion}/`; the download may leave `.part` files behind.
- Materializes unchanged files and runs differential delete synchronization; updates (or creates for the first time) the `UpdateFolder/release_manifest.json` cache.
- Writes `version.json` (`downloaded`, `version_previous`, `downloaded_file_count`, `after_apply_update_script`, clearing `rollback_previous` / `rollback_target` / `apply_stage`).
- Holds `UpdateFolder/.aly.lock`; appends to `update.log` / `update_{version}_fail.log`.
- When re-downloading the same version (because artifacts were lost) and the run fails midway, the target version directory is cleaned up so no half-finished package is left behind.

### Boundaries and Errors

- With `version_status=applying`, the command prints `data:null` and returns immediately **without downloading**; this signals that `apply_update` should perform recovery, so the host must not mistake it for a completed download.
- When the target directory exists but cannot be read (ACLs, antivirus locks, a slow disk), it is treated as unreadable: the same-version re-download is skipped and the artifacts are not misjudged as lost.
- When the client is already at the latest version, no server is configured, the server has no changelog, the directory cannot be created, or the file manifest cannot be fetched, the command returns `isSuccess=false` with `data=null`.
- On materialization failure the last line is a `FAIL` progress line (with `file` set to `materialize`), and no final completion line is printed; the host should treat that line as a failure.
- When the server manifest or the cache is missing, delete synchronization degrades to "delete nothing" and does not block the download.
- A successful download ends with `data:null`; if a failure carries `failed_files`, the host should retry the whole command, and files that already succeeded are skipped.
- Business failures still end with exit code `0`.

### Related Commands

[`check_update`](#check-update) · [`apply_update`](#apply-update) · [`check_diff`](#check-diff) · [`clean`](#clean) · [Configuration reference](/en/docs/client/config)

## apply_update

### Purpose

Atomically replace the main program directory with the downloaded new version, write the `applied` state back, run the post-update script (if any), and then start the main program. The command does not use the network.

### Signature

```bash
aly-client.exe apply_update [--main-exe-path <relative_path>] [--close-timeout <seconds>] [--must-close-process-name <names>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--main-exe-path` | string | No | Empty; falls back to `main_exe_relative_path` in `client.json` | Temporarily override the main program path |
| `--close-timeout` | int | No | `30` | Timeout in seconds for process shutdown and directory-occupancy retries |
| `--must-close-process-name` | string | No | Empty | Comma-separated process names appended to `must_close_process_name` in `client.json`; the C# SDK passes this explicitly |

### Behavior

1. Take the global lock, load the configuration, and opportunistically prune historical snapshots (keeping the 3 most recent).
2. Branch on `version_status`:
   - `applied` or empty: return `no pending update to apply` without replacing anything.
   - `downloaded`: validate the on-disk situation first, then replace.
   - `applying`: crash recovery, patching `applied` according to what is actually on disk, redoing the remaining renames, or completing a resumed rollback.
   - Unknown value: refuse to replace and report an error; only `download_update` can repair the state.
3. Validation for `downloaded` (every rejection happens before the state is written, processes are closed, or directories are renamed):
   - `version` is empty: reject and self-heal to `applied` + `version_previous`;
   - `version_previous == version`: reject the replacement (the backup directory and the version directory would be the same path);
   - the pending directory is missing, empty, or holds fewer files than `downloaded_file_count`: reject and self-heal to `applied` with the real content version, waiting for a new download;
   - the pending directory exists but cannot be read: report the error only, changing nothing.
4. Write `version_status=applying`.
5. Close the processes listed in `must_close_process_name` (on Windows, send `WM_CLOSE` first, wait, then kill); any unlisted holder is handled by the probe logic after a rename failure.
6. Atomic replacement (the whole sequence is retried up to 3 times):
   - if an old backup directory exists, move it aside as `.old` first;
   - rename `MainFolder` to `ApplicationFolder_{version_previous}`;
   - write `apply_stage=activating` and rename `ApplicationFolder_{version}` to `MainFolder`;
   - if any step fails, attempt a rollback and return the state to `downloaded` (if the main directory is already gone but the version directory still exists, stay in `applying` and let crash recovery take over).
7. On success: write `version_status=applied`, clear `rollback_previous` / `rollback_target` / `apply_stage`, reset `apply_fail_count`, and delete unversioned backup directories that are no longer referenced.
8. Run `after_apply_update_script` (relative to `MainFolder`, run synchronously with a default 60-second timeout; the result is written to `update.log`). After the call finishes, a `.updator/after_apply_{version}.done` marker is written; the script's exit code is not checked, so scripts must be idempotent.
9. Start the main program (with the main program directory as the working directory).

### Return Fields

`data` is `null` for both success and business failure:

| Field | Type | Present when | Description |
|---|---|---|---|
| `isSuccess` | bool | Always | Whether the replacement and state write succeeded; a failure to start the main program is only logged and does not change the result |
| `errorMsg` | string | Always | Failure reason; an empty string on success |
| `data` | null | Always | Always `null` |

### Examples

Success:

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

No pending update:

```json
{"isSuccess":false,"errorMsg":"no pending update to apply","data":null}
```

Pending directory missing or incomplete (already self-healed):

```json
{"isSuccess":false,"errorMsg":"待应用版本目录缺失、为空或不完整: ...\\ApplicationFolder_2.0（已回退为 applied/1.0，等待重新下载）","data":null}
```

Replacement failed:

```json
{"isSuccess":false,"errorMsg":"backup rename failed: ...","data":null}
```

Anomalous state:

```json
{"isSuccess":false,"errorMsg":"版本状态异常：Version == VersionPrevious(2.0)，拒绝替换","data":null}
```

### Side Effects

- Writes `version.json` (`applying` → `applied`, `apply_stage`, `apply_fail_count`, clearing the rollback transition fields).
- Closes processes; renames `ApplicationFolder` and the version directory; may leave `.old` move-aside leftovers (cleaned up on success).
- Runs the post-update script and writes the `.updator/after_apply_{version}.done` marker.
- Starts the main program.
- Holds `.aly.lock`, opportunistically prunes historical snapshots, and writes `update.log`.
- As a failure fallback, if the main directory still exists the previous version of the main program is started so the application remains usable; if the main directory is already gone but the version directory remains, the state stays `applying` for crash recovery.

### Boundaries and Errors

- In the `applied` or empty state, the command returns "no pending update to apply"; a fake success from "renaming the main directory onto itself" cannot happen.
- When the pending directory is missing, empty, or has too few files, the replacement is refused and the state self-heals; the current directory is never copied into the version directory and then reported as a success.
- When the directory exists but cannot be read, the command only reports an error and changes no state, so a possibly valid pending package is not discarded.
- On replacement failure the state returns to `downloaded` and `apply_fail_count` is incremented; after 3 consecutive failures, `check_update` stops reporting the pending update (circuit breaker).
- When the main directory is already gone but the version directory still exists, the state stays `applying` and the next `apply_update` / `check_update` performs crash recovery, so the application directory is never lost permanently.
- The post-update script runs synchronously; a failed or timed-out script only writes `update.log` and does not affect `isSuccess`. Scripts should be idempotent because crash recovery may run an unfinished script again.
- A failure to start the main program (wrong path, insufficient permissions) is only logged and the command may still return success; hosts must not read `isSuccess=true` as "the main program is definitely running".
- Business failures (including "no pending update to apply") still end with exit code `0`.

### Related Commands

[`download_update`](#download-update) · [`check_update`](#check-update) · [`rollback`](#rollback) · [version.json](/en/docs/client/config#version-json)

## list_rollback_versions

### Purpose

Scan the historical version snapshot directories under `PackageFolder` and return the rollback-able versions along with the current version. The command does not use the network and does not take the global lock.

### Signature

```bash
aly-client.exe list_rollback_versions [--main-exe-path <relative_path>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--main-exe-path` | string | No | Empty; falls back to `main_exe_relative_path` in `client.json` | Temporarily override the main program path |

### Behavior

1. Derive the main folder name from `main_exe_relative_path` (for example `ApplicationFolder`).
2. List the subdirectories under `PackageFolder` whose names start with `{main folder name}_`.
3. Keep only those whose suffix is a valid version number (a single numeric segment or multiple segments, the semver pre-release / build-metadata form) and that are directories.
4. In the `downloaded` state, the pending version directory is a download artifact rather than a historical snapshot, so it is excluded from the list; `current_version` is taken from `version_previous` (the real content version of the main directory).
5. Return `current_version` and `versions`.

### Return Fields

| Field | Type | Present when | Description |
|---|---|---|---|
| `current_version` | string | Always | Version of the content currently in the main directory; an empty string when unknown |
| `versions` | array \| null | Always | List of rollback-able versions; `null` when there are none |

### Examples

```json
{"isSuccess":true,"errorMsg":"","data":{"current_version":"1.0.1","versions":["1.0.0","0.9.0"]}}
```

No historical snapshots:

```json
{"isSuccess":true,"errorMsg":"","data":{"current_version":"1.0.1","versions":null}}
```

Configuration read failure:

```json
{"isSuccess":false,"errorMsg":"load config: ...","data":null}
```

### Side Effects

- Reads directories and `version.json` only; takes no global lock and writes no files.
- Does not use the network.

### Boundaries and Errors

- A version directory must exist and actually be a directory; directories whose names end in something like `.old` never enter the list.
- A single-segment numeric version is also considered a valid version.
- When there is no rollback-able version, `versions` is `null`, and hosts must tolerate that.
- Business failures still end with exit code `0`.

### Related Commands

[`rollback`](#rollback) · [`clean`](#clean) · [Updater overview](/en/docs/client/#security-and-rollback)

## rollback

### Purpose

Atomically replace the current main directory with a specified historical version snapshot, write the `applied` state back, and start the main program. The command does not use the network.

### Signature

```bash
aly-client.exe rollback --version <target_version> [--main-exe-path <relative_path>] [--close-timeout <seconds>] [--must-close-process-name <names>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--version` | string | **Yes** | — | Target rollback version |
| `--main-exe-path` | string | No | Empty; falls back to `main_exe_relative_path` in `client.json` | Temporarily override the main program path |
| `--close-timeout` | int | No | `30` | Timeout in seconds for process shutdown and directory-occupancy retries |
| `--must-close-process-name` | string | No | Empty | Comma-separated process names appended to `must_close_process_name` in `client.json` |

### Behavior

1. Take the global lock, verify that `--version` is non-empty, and load the configuration and `version.json`.
2. Opportunistically prune historical snapshots, protecting this CLI invocation's target version from cleanup.
3. Refuse an unknown `version_status`; `applying` with an empty `rollback_previous` means an apply is unfinished, so rollback is refused and the caller is told to run `apply_update` first.
4. `applying` with a non-empty `rollback_previous` means a rollback was interrupted; delegate to crash recovery and resume it.
5. The target version directory must exist; in the `downloaded` state, the "version waiting to be applied" cannot be used as a rollback target.
6. The backup directory name comes from the real content version of `MainFolder` before replacement: `version_previous` when `downloaded`, `rollback_previous` when resuming a rollback.
7. Write `version_status=applying`, record `rollback_previous` and `rollback_target`, and close the processes.
8. Atomic swap: move the old backup aside → rename the main directory to the old-version backup → rename the target version directory to the main directory. Once the replacement stage has begun, a failure restores the pre-rollback state (it does not unconditionally write `applied`), and starts the current version of the main program if the main directory still exists.
9. On success: write `version=<target version>`, `version_previous=<pre-rollback version>`, `version_status=applied`, clear the rollback transition fields, drop the release manifest cache, run the post-update script (if any), and start the main program.

### Return Fields

| Field | Type | Present when | Description |
|---|---|---|---|
| `isSuccess` | bool | Always | Whether the rollback succeeded |
| `errorMsg` | string | Always | Failure reason; an empty string on success |
| `data` | object \| null | Always | An object on success, `null` on failure |
| `data.version` | string | On success | The target version rolled back to |

### Examples

Success:

```json
{"isSuccess":true,"errorMsg":"","data":{"version":"1.0.0"}}
```

Target version does not exist:

```json
{"isSuccess":false,"errorMsg":"version 0.0.1 not found","data":null}
```

Missing `--version`:

```json
{"isSuccess":false,"errorMsg":"--version is required","data":null}
```

An unfinished apply:

```json
{"isSuccess":false,"errorMsg":"有一次未完成的更新（applying），请先执行 apply_update 完成恢复后再回滚","data":null}
```

### Side Effects

- Writes `version.json`: `rollback_previous` / `rollback_target` on entering `applying`; `applied` with the transition fields cleared on success.
- Directory renames and `.old` leftovers; move-aside leftovers are cleaned up after success.
- Drops the `release_manifest.json` differential delete cache.
- Runs the post-update script and starts the main program.
- Holds `.aly.lock`, opportunistically prunes snapshots, and writes `update.log`.
- If the replacement stage fails while the main directory still exists, starts the current version of the main program so the application stays usable.

### Boundaries and Errors

- A missing target version directory, a missing `--version`, an unknown state, an unfinished apply, a target that is the pending download version, or a target that is already the active version are all refused with a failure.
- A failure after entering the replacement stage restores the pre-rollback state (`preStatus`), avoiding the mismatch where "the state says applied while the disk still holds the old content"; rejections from the entry guards do not change the state.
- If the server version is still higher after a successful rollback, the next `check_update` upgrades again; there is no rollback pin policy yet, so operations must adjust or unpublish the server version in parallel.
- A failed post-update script or a failed main-program start only writes `update.log` and does not change `isSuccess`; hosts must not read a success as "the main program is definitely running".
- Business failures still end with exit code `0`.

### Related Commands

[`list_rollback_versions`](#list-rollback-versions) · [`apply_update`](#apply-update) · [`clean`](#clean) · [Updater overview](/en/docs/client/#security-and-rollback)

## clean

### Purpose

Reclaim disk space explicitly: remove historical version snapshots beyond the retention count, move-aside leftovers, orphaned backup directories, download temporary files, and expired logs. `prune` only runs opportunistically at the apply/rollback entry points, so `clean` is needed to actively reclaim space on clients that have not updated in a long time.

### Signature

```bash
aly-client.exe clean [--keep <N>] [--all]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--keep` | int | No | `0` | Keep the N most recent version snapshots; **a value of 0 (or a negative number) is treated as the default, 3** |
| `--all` | bool | No | `false` | Aggressive mode: keep only the current-version protection set; equivalent to `--keep 1` but reports `mode=all` |

### Behavior

1. Take the global lock (`clean` modifies `PackageFolder` and must not run concurrently with an update in flight) and load the configuration.
2. With `--all`: call prune with `keep=1`, keeping only the single most recent rollback-able snapshot.
3. Otherwise `keep = --keep`; when it is less than 1, the default of `3` is used instead and older snapshots are removed.
4. The protection set is never deleted: `MainFolder`, `version`, `version_previous`, `rollback_target` (plus any target passed explicitly by the rollback entry point).
5. Both modes also clean up:
   - `.old` / `.old.N` leftovers next to version directories;
   - orphaned, version-less backup directories `{MainFolder}_`;
   - `.part` / `.tmp` files inside version snapshot directories, plus temporary files at the top level of `.updator` and the package root;
   - `update_*` logs older than 30 days (including `update.log` and `update_*_fail.log`).
6. `.tmp` files inside `MainFolder` are not cleaned recursively, so temporary data held by the business application is not deleted by mistake.

### Return Fields

| Field | Type | Present when | Description |
|---|---|---|---|
| `mode` | string | Always | `keep` or `all` |
| `keep` | int | When `mode=keep` | The number of snapshots actually retained |

### Examples

Default cleanup (keep 3):

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"keep","keep":3}}
```

`--keep 0` is also treated as 3:

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"keep","keep":3}}
```

Keep only 1:

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"keep","keep":1}}
```

Aggressive cleanup:

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"all"}}
```

Lock held (failure):

```json
{"isSuccess":false,"errorMsg":"另一更新正在进行中，请稍后重试","data":null}
```

### Side Effects

- Deletes version snapshot directories beyond the retention count.
- Deletes `.old*` leftovers, orphaned backup directories, `.part` / `.tmp` temporary files, and update logs older than 30 days.
- Records the deletions in `update.log`.
- Holds `.aly.lock`; does not write `version.json`.

### Boundaries and Errors

- Returns a failure when the lock is held; `clean` never bypasses the lock.
- `--keep 0` means the default of 3, not "keep 0"; negative numbers also fall back to 3.
- `--all` takes precedence over `--keep`, and its output contains no `keep`.
- Cleanup is best-effort: an ACL failure on a single directory does not fail the whole command, but may leave leftovers behind.
- Business failures still end with exit code `0`.

### Related Commands

[`apply_update`](#apply-update) · [`rollback`](#rollback) · [`download_update`](#download-update)

## check_self_update

### Purpose

Compare the SHA256 of "the updater copy that is currently running" with "the newer updater distributed with the version inside the main program directory" to decide whether the updater itself needs updating. The command only detects; it does not replace anything.

### Signature

```bash
aly-client.exe check_self_update [--main-exe-path <relative_path>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--main-exe-path` | string | No | Empty; falls back to `main_exe_relative_path` in `client.json` | Temporarily override the main program path |

### Behavior

1. Locate `MainFolder/aly-client.exe` (`aly-client` on Linux).
2. If the main program directory has no updater (first deployment) → `need_update=false`.
3. Compute the SHA256 of the running copy and of the copy in the main program directory.
4. If they differ → `need_update=true`; if they match → `false`.

There is only one source for a new version: the publisher bundles the updater into the version package, `download_update` downloads it and `apply_update` replaces the main directory, and only then does a newer updater exist in the main directory.

### Return Fields

| Field | Type | Present when | Description |
|---|---|---|---|
| `need_update` | bool | Always | Whether `self_update` should be run |

### Examples

Update needed:

```json
{"isSuccess":true,"errorMsg":"","data":{"need_update":true}}
```

No update needed, or the main program directory has no updater:

```json
{"isSuccess":true,"errorMsg":"","data":{"need_update":false}}
```

File read failure:

```json
{"isSuccess":false,"errorMsg":"...","data":null}
```

### Side Effects

- Takes no global lock, writes no files, and does not use the network.
- Only reads the two executables and computes their hashes.

### Boundaries and Errors

- The main program directory having no updater (first deployment) is not an error; it returns `need_update=false`.
- If hashing either file fails, the command returns `isSuccess=false`.
- Detection and execution are separate: `check_self_update` never replaces anything; the replacement must be performed by calling `self_update`.
- Business failures still end with exit code `0`.

### Related Commands

[`self_update`](#self-update) · [Self-update design](/en/docs/self-update)

## self_update

### Purpose

Replace the running copy of the updater in `UpdateFolder` with the newer updater distributed with the version inside the main program directory. Paired with `check_self_update`, it closes the self-update loop.

### Signature

```bash
aly-client.exe self_update [--main-exe-path <relative_path>]
```

### Parameters

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--main-exe-path` | string | No | Empty; falls back to `main_exe_relative_path` in `client.json` | Temporarily override the main program path |

### Behavior

1. Take the global lock `.aly.lock`; if another update is in progress, return "another update is in progress".
2. Locate `MainFolder/aly-client.exe` (the source) and the running copy.
3. Source missing (first deployment) → skip and return `updated=false`, `reason=source_missing`; this is not an error.
4. SHA256 matches → skip, cleaning up `.new` / `.old` leftovers along the way, and return `updated=false`, `reason=already_up_to_date`.
5. Mismatch → back up first, then replace:
   - copy the source to `aly-client.exe.new` and verify it;
   - rename the running exe to `aly-client.exe.old` (a running file may be renamed);
   - atomically place `.new`; on failure, roll back `.old`;
   - if copying or verification fails, keep the original file and clean up `.new`.
6. The current process keeps running from the old inode until it exits; the next launch by the host runs the new version.

### Return Fields

| Field | Type | Present when | Description |
|---|---|---|---|
| `updated` | bool | Always | Whether the replacement was completed |
| `reason` | string | Always | `replaced` / `already_up_to_date` / `source_missing` |
| `old_path` | string | When `updated=true` | Backup path of the replaced old copy (`.old`) |

### Examples

Replacement succeeded:

```json
{"isSuccess":true,"errorMsg":"","data":{"updated":true,"reason":"replaced","old_path":"...\\aly-client.exe.old"}}
```

Content already identical:

```json
{"isSuccess":true,"errorMsg":"","data":{"updated":false,"reason":"already_up_to_date"}}
```

No updater in the main program directory:

```json
{"isSuccess":true,"errorMsg":"","data":{"updated":false,"reason":"source_missing"}}
```

Lock held:

```json
{"isSuccess":false,"errorMsg":"另一更新正在进行中，请稍后重试","data":null}
```

### Side Effects

- May create a `.new` staging file and an `.old` backup; after a successful replacement the old copy is kept until the next "content identical" call cleans it up.
- Holds `.aly.lock` and records the replacement result in `update.log`.
- Does not write `version.json` and does not use the network.

### Boundaries and Errors

- A missing source is not an error; identical content is skipped idempotently.
- There is a very short window between the two renames during which the target does not exist; if the process is killed exactly then, `.old` still exists, and the next UI/SDK start places it again from the main directory, or it can be renamed back to `aly-client.exe` by hand.
- When the lock is held the command fails directly; the host should log it and retry at the next start rather than treating it as a startup failure.
- **Do not trigger it twice**: in a UI deployment the UI already delegates `self_update` itself; in an SDK deployment the SDK constructor calls it; only a bare console host needs to call it explicitly. Repeated calls are idempotent, but cost an extra process creation and lock contention.
- Upgrade compatibility: when an older updater does not recognize `self_update`, the UI/SDK falls back to copying the copy from the main directory directly; when the lock is held (the command returns a JSON failure), there is no fallback.
- Business failures still end with exit code `0`.

### Related Commands

[`check_self_update`](#check-self-update) · [Self-update design](/en/docs/self-update) · [GUI front-end](/en/docs/client/ui)