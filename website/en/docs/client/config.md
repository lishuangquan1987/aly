---
title: Client Configuration Reference
description: Fields, locations, effective timing and precedence for client.json, .updator/shared.json, version.json, publish.json and the staging file.
---

# Client Configuration Reference

The client involves three core configuration files, plus two publisher-side local files that are easy to confuse with them:

| File | Location | Read/written by | Shipped with versions |
|---|---|---|---|
| `client.json` | `UpdateFolder/client.json` | Read by the updater; written by hand at deployment | No |
| `shared.json` | `MainFolder/.updator/shared.json` | Shared by `aly-publish` and `aly-client` | Yes; uploaded to the server and downloaded with updates |
| `version.json` | `UpdateFolder/version.json` | Written and read by the updater | No; local machine state |
| `publish.json` | Publish directory `.updator/publish.json` | Exclusive to `aly-publish` | No; local, not uploaded |
| `staged-files.json` | Publish directory `.updator/staging/staged-files.json` | Exclusive to `aly-publish` | No; local staging |

`UpdateFolder` is the directory holding the updater and `MainFolder` is the currently running directory (derived from `client.json`). For the deployment layout see the [updater overview](/en/docs/client/).

## client.json

### Location and Role

Located at `UpdateFolder/client.json`, next to the `aly-client` executable. Every updater command reads it first, using it to locate the main program directory and the processes that must be closed.

### Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `main_exe_relative_path` | string | Yes | — | Path of the main program executable relative to `UpdateFolder` |
| `must_close_process_name` | string[] | No | `[]` | Process names to close before an update/rollback, usually without `.exe` |

Rules for `main_exe_relative_path`:

- The base directory is **`UpdateFolder`**, not `PackageFolder`, and not the current working directory.
- The updater takes the parent directory of this path as `MainFolder`; the file name itself is only used to launch the main program.
- The recommended value is `../ApplicationFolder/YourApp.exe`, corresponding to `MainFolder = PackageFolder/ApplicationFolder`.
- On Linux, backslashes are normalized to forward slashes, so a `..\ApplicationFolder\YourApp.exe` produced by Windows packaging tools can be reused as-is.
- The field must point to a real main program file; leaving it empty makes the `MainFolder` derivation wrong, and the updater will not guess or correct it for you.

Rules for `must_close_process_name`:

- Fill in process names, not full paths; on Windows both forms with and without `.exe` match (case-insensitive).
- `apply_update` first sends `WM_CLOSE` to request a normal exit, waits briefly, then kills any surviving process; `rollback` also closes gracefully first.
- Holders that are not listed are handled by the probe logic after a rename failure and may then be killed outright. The host should list its own process names here so it gets a chance to save data and stop listening.

### Example

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp", "YourApp.PluginHost"]
}
```

### Boundaries and Errors

- If `client.json` is missing or its JSON is malformed, every updater command fails during configuration loading and returns `load config: ...`.
- Unknown fields are ignored, so deployment tools may attach comment-like fields (their future meaning is not guaranteed).
- The GUI reads only `main_exe_relative_path` to derive the main program directory, then reads `project_name` from `shared.json` as its title.
- `--main-exe-path` can temporarily override the path; `--must-close-process-name` **appends** to the list rather than replacing it.

## .updator/shared.json

### Location and Role

Located at `MainFolder/.updator/shared.json`, maintained by `aly-publish` in the publish directory and uploaded with each version. After the client downloads an update, the new version's `shared.json` replaces the old file, so rule changes can be delivered automatically with the version.

During the download/check phase the client reads the `shared.json` of the **current main directory**; only after an update is applied does the `shared.json` in the new main directory take effect. During crash recovery, if the main directory has already been renamed away, the client tries to read a usable `shared.json` from the version directory.

### Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `server_url` | string | Required for network commands | Empty | Server address, e.g. `http://10.0.0.1:2000` |
| `project_name` | string | Required for network commands | Empty | Server project name; must match the name used when publishing |
| `ignore_folders` | string[] | No | `[]` | Folders ignored during the publish scan |
| `ignore_files` | string[] | No | `[]` | Files ignored during the publish scan |
| `un_copy_folders` | string[] | No | `[]` | Folders "not migrated" during client materialization |
| `un_copy_files` | string[] | No | `[]` | Files "not migrated" during client materialization |

### How ignore_* and un_copy_* Differ

These are two sets of rules that apply at completely different points:

| Rule | Applies to | When it takes effect | Affects the server file manifest | Affects local client files |
|---|---|---|---|---|
| `ignore_folders` / `ignore_files` | `aly-publish` | Publish scan | Yes; matching files are never uploaded | Indirectly: they are absent from the manifest, so the client never downloads them |
| `un_copy_folders` / `un_copy_files` | `aly-client` | Materialization at the end of `download_update` | No | Yes; matching files in the current main directory are not copied into the pending version directory |

`ignore_*` **does not apply during client materialization**. If a file has already been picked up by the server, the client still downloads it, even when it also appears in `ignore_*`. Conversely, `ignore_*` is not a client-side delete or keep rule either: whether a locally existing file with the same name ends up in the new version directory depends on `un_copy_*` at materialization time and on whether it is part of the new version package.

`un_copy_*` only affects the step that copies unchanged files from the current main directory into the new version directory; files that genuinely exist in the new version package are still downloaded normally. The typical use is to leave runtime data, logs and caches in the old directory instead of carrying them into the new version snapshot:

```json
{
  "un_copy_folders": ["logs", "cache"],
  "un_copy_files": ["*.log", "config/local.json"]
}
```

Matching semantics:

- `ignore_folders` is plain relative-path prefix matching: `logs` skips `logs` and its subdirectories; `**` is not expanded.
- `ignore_files` supports exact relative-path matching, `filepath.Match` globs (`*` does not cross `/`), matching by file name, and suffix matching with a leading `*`.
- `un_copy_folders` supports exact relative-path matching, globs, and `**` at any depth (for example `**/node_modules`).
- `un_copy_files` supports matching by file name or relative path, with globs such as `*` and `?`.

::: warning
Do not add `.updator` to `ignore_folders`: `shared.json` must be uploaded with the version, otherwise new clients cannot obtain the server address and project name, and configuration updates can no longer be delivered. `.updator/publish.json` and `.updator/staging/` are excluded automatically by the publishing tool, so there is no need to ignore them by hand.
:::

### Example

```json
{
  "server_url": "http://10.0.0.1:2000",
  "project_name": "myapp",
  "ignore_folders": ["logs", "temp"],
  "ignore_files": ["*.log", "*.tmp"],
  "un_copy_folders": ["logs", "cache"],
  "un_copy_files": ["*.log"]
}
```

### Boundaries and Errors

- When `server_url` or `project_name` is empty, `check_update`, `check_diff` and `download_update` return `no server url configured` / `no project name configured` respectively.
- If `shared.json` is missing or corrupt, the updater fails during configuration loading; only when `MainFolder` genuinely does not exist (a crash in progress) does it fall back to looking for `shared.json` in the version directory.
- `--url` / `--project-name` only affect `check_update`, `check_diff` and `download_update`, and only for the current call; they are never written back to the file.
- `apply_update`, `rollback`, `list_rollback_versions`, `clean`, `check_self_update` and `self_update` do not need server configuration.

## version.json

### Location and Role

Located at `UpdateFolder/version.json`, recording the local version state machine. It is not part of the release package and should not be distributed with the application; the updater writes it and hosts only read it.

Do not edit it by hand. When the state and the disk content disagree, `apply_update` / `rollback` refuse to replace; only `download_update` can rewrite an unknown state to `downloaded` and thereby repair it.

### Fields

| Field | Type | Present/Default | Description |
|---|---|---|---|
| `version` | string | Usually present | Current applied version when `applied`; the version waiting to be applied or currently being applied when `downloaded` / `applying` |
| `version_previous` | string | May be empty | Real content version of `MainFolder` before the replacement, and the directory suffix of the rollback-able snapshot; empty on the first update |
| `version_status` | string | May be empty | `applied` / `downloaded` / `applying`; an empty string or a missing file is equivalent to `applied` |
| `apply_stage` | string | May be empty | Only written as `activating` while an apply is in flight, as the crash-recovery evidence that "the activation rename has started"; cleared in the terminal state |
| `downloaded_file_count` | int | `0` | Number of regular files in the download output (excluding `.part` / `.tmp`), used by apply to detect an incomplete on-disk state; `0` in an old file means unknown and degrades to the weaker "directory is non-empty" check |
| `rollback_previous` | string | May be empty | Real content version of `MainFolder` when the rollback started; present only while a rollback is in flight (`applying`) |
| `rollback_target` | string | May be empty | Target version of the current rollback; present only while the rollback is in flight, so crash recovery can resume it |
| `apply_fail_count` | int | `0` | Number of consecutive apply failures for the same pending version; once it reaches 3, `check_update` stops reporting the pending update |
| `after_apply_update_script` | string | May be empty | Path of the post-update script relative to `MainFolder`; synchronized from the server changelog by `download_update`, then run after a successful apply/rollback |

### version_status Values

| Value | Meaning | Main directory content | Next step |
|---|---|---|---|
| `""` (missing/empty) | No version record; treated as `applied` | Unknown / first deployment | `check_update` checks again; `apply_update` returns "no pending update to apply" |
| `applied` | No pending update | Content matching `version` | Check for a new version normally |
| `downloaded` | New version downloaded, waiting to be applied | Content matching `version_previous` | `apply_update`; if artifacts are lost, `download_update` fills them in |
| `applying` | An apply or rollback is in flight | Depends on the crash point | `check_update` / `apply_update` perform crash recovery |

While a rollback is in flight, `version` is still the pre-rollback version and the target version is recorded in `rollback_target`; `version` is written with the target version only after the rollback succeeds.

### Examples

Download complete, waiting to be applied:

```json
{
  "version": "1.0.1",
  "version_previous": "1.0.0",
  "version_status": "downloaded",
  "downloaded_file_count": 128,
  "after_apply_update_script": "scripts/after_update.bat"
}
```

Rollback in flight:

```json
{
  "version": "1.0.1",
  "version_previous": "1.0.0",
  "version_status": "applying",
  "rollback_previous": "1.0.1",
  "rollback_target": "0.9.0"
}
```

Apply activation-rename stage (crash recovery evidence):

```json
{
  "version": "1.0.1",
  "version_previous": "1.0.0",
  "version_status": "applying",
  "apply_stage": "activating",
  "downloaded_file_count": 128
}
```

### Writes and Recovery

- Writes are atomic: `version.json.tmp` is written and `fsync`ed first, then renamed over the target.
- Before every write, if the current file parses, it is saved as `version.json.bak` (last-known-good).
- When the JSON is corrupt on read, the updater first tries to recover from `.bak`; if that still fails, it renames the corrupt file to `.corrupt` and resets to the empty state (equivalent to a first deployment), so the updater is never permanently wedged.
- After the post-update script returns, a `MainFolder/.updator/after_apply_{version}.done` marker is written (the implementation does not check the script's exit code); crash recovery uses it to skip a script that has already been invoked. Scripts should be written to be idempotent, and the presence of the marker must not be read as "the script succeeded".

## .updator/publish.json

A configuration exclusive to `aly-publish` in the publish directory; it is currently a placeholder empty object:

```json
{}
```

| Attribute | Description |
|---|---|
| Location | Publish directory `.updator/publish.json` |
| Read/written by | `aly-publish` (including publish-gui, which calls it through a child process) |
| Uploaded | No; excluded automatically by the publishing tool |
| Fields | No official fields yet; reserved for CLI-specific settings |

Do not put it in `UpdateFolder`; the client never reads it.

## .updator/staging/staged-files.json

The local staging area in the publish directory, written by `aly-publish add` and cleared after a successful `push`:

```json
[
  {
    "relativePath": "app.exe",
    "status": "modified",
    "localMd5": "abc123",
    "localSize": 5242880
  }
]
```

| Attribute | Description |
|---|---|
| Location | Publish directory `.updator/staging/staged-files.json` |
| Read/written by | Exclusive to `aly-publish` |
| Uploaded | No; the whole `staging/` directory is skipped during scanning |
| Purpose | Records the files waiting to be published and their status, for `push` to consume |

The client never reads this file. Do not edit it by hand; use `aly-publish add` / `reset` when the staged content needs adjusting.

## Configuration Precedence

There is no complex merging of "several configuration files overwriting the same field"; the client loads a fixed order and applies overrides as needed:

| Setting | Source order (later overrides earlier) |
|---|---|
| Main program path | `main_exe_relative_path` in `client.json` → `--main-exe-path` |
| Processes to close | `must_close_process_name` in `client.json` → `--must-close-process-name` (**appends**) |
| Server address | `server_url` in `shared.json` → `--url` (check/diff/download only) |
| Project name | `project_name` in `shared.json` → `--project-name` (check/diff/download only) |
| Ignore / no-copy rules | `shared.json` only; no command-line override |
| Version state | `version.json` only |

Loading order:

1. Read `client.json` from `UpdateFolder`;
2. merge `--main-exe-path` and derive `MainFolder`;
3. read `MainFolder/.updator/shared.json`;
4. override this call's server configuration with `--url` / `--project-name`;
5. append `--must-close-process-name`.

The publisher-side configuration precedence (CLI parameters take precedence over `.updator/shared.json` / `publish.json`) is decided by `aly-publish`; see the [publishing tool](/en/docs/publish).