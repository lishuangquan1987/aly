---
title: Release Workflow Commands
description: Behavior, JSON contract, push progress stream and failure semantics of status, diff, add, reset, staged, push, log and server info.
---

# Release Workflow Commands

This group of commands covers the full release path: compare the differences → stage changes → upload and register a version. Unless stated otherwise, the examples omit the global `--server`, `--project`, `--json` and `--quiet` flags.

| Command | Purpose | Needs the server | Needs `project.name` | Needs the staging area |
|---|---|---|---|---|
| `status` | Local-vs-server diff overview; also syncs the staging area | Yes | Yes | No |
| `diff` | Per-file local/server hashes and sizes | Yes | Yes | No |
| `add` | Stage changes | Yes for `--all`; no for explicit files | Yes | No |
| `reset` | Unstage | No | Yes | Yes |
| `staged` | Show the staging area | No | Yes | No |
| `push` | Upload staged files and register a version | Yes | Yes | Yes (non-empty) |
| `log` | View the version log | Yes | Yes (the server rejects an empty name) | No |
| `server info` | Show server system information | Yes | No | No |

## Staging area model

The staging area is the local file `.updator/staging/staged-files.json` and is never uploaded. Each entry contains a relative path, the status at the time it was staged, and the local MD5 and size:

```json
[
  {
    "relativePath": "DemoApp.dll",
    "status": "modified",
    "localMd5": "96c15c2bb2921193bf290df8cd85e2ba",
    "localSize": 90052
  }
]
```

`add` writes it, `reset` deletes entries, and `push` reads it and clears it on success. `status` merges the staging area into the diff result and automatically prunes entries that "have already been uploaded successfully".

## status

```bash
aly-publish status
```

Flow:

1. Recursively scan the current directory, skipping `.updator/staging/`, `.updator/publish.json` and any paths matched by the ignore rules; `.updator/shared.json` is scanned (it needs to be uploaded with the version).
2. Compute MD5 + SHA256 for every file.
3. Call `GET /api/file/get_all_files/{projectName}` to fetch the server manifest.
4. Classify by relative path and MD5 into `new`, `modified`, `deleted` and `unchanged`.
5. **Automatically sync the staging area**: if a staged file exists on the server with the same MD5, it is considered already uploaded successfully and is removed from the staging area. In human mode it prints:

   ```text
   已同步暂存区：移除了 2 个已上传成功的文件
           DemoApp.dll
           assets/theme.pak
   ```

   This step handles the situation where an upload finished but the version was not registered, or the process was interrupted; re-running `status` restores consistency.
6. Put the remaining staged entries into `staged` and remove them from `unstaged`.

Human output:

```text
Changes staged for commit:
  (use "aly-publish reset <file>..." to unstage)

        modified:    DemoApp.dll
        new:         .updator/shared.json

Changes not staged for commit:
  (use "aly-publish add <file>..." to stage)

        deleted:     old.txt
        modified:    sub/b.bin

Unchanged files:
        a.txt
```

`deleted` only appears under `unstaged`; `add --all` does not stage deletions.

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "staged": [
      {
        "relativePath": "DemoApp.dll",
        "status": "modified",
        "localMd5": "96c15c2bb2921193bf290df8cd85e2ba",
        "localSize": 90052
      }
    ],
    "unstaged": [
      {
        "relativePath": ".updator/shared.json",
        "status": "new",
        "localMd5": "97a209b3de6950d03a6469dea098fd7d",
        "localSize": 234
      },
      {
        "relativePath": "old.txt",
        "status": "deleted",
        "localMd5": "",
        "localSize": 0,
        "serverMd5": "cccccccccccccccccccccccccccccccc",
        "serverSize": 3
      }
    ],
    "unchanged": [
      {
        "relativePath": "a.txt",
        "status": "unchanged",
        "localMd5": "5d41402abc4b2a76b9719d911017c592",
        "localSize": 5,
        "serverMd5": "5d41402abc4b2a76b9719d911017c592",
        "serverSize": 5
      }
    ]
  }
}
```

Notes:

- Empty categories are output as `null` rather than `[]` (Go nil slice).
- Staged entries have no `serverMd5` / `serverSize` fields; they come from the local snapshot taken by `add`.
- A `status` failure (server unreachable, etc.) returns `isSuccess: false`, and the exit code is still 0.
- `--quiet` can suppress human text; `--json` output remains complete.

## diff

```bash
aly-publish diff [--file <relative-path>]
```

| Flag | Description |
|---|---|
| `--file` | Output only the file at the given relative path; matched exactly against the full relative path |

Output example:

```text
--- sub/b.bin ---
- local:  sub/b.bin  modified  11 bytes  md5:96c15c2bb2921193bf290df8cd85e2ba
+ server: sub/b.bin  modified  11 bytes  md5:00000000000000000000000000000000

--- a.txt ---
- local:  a.txt  unchanged  5 bytes  md5:5d41402abc4b2a76b9719d911017c592
+ server: a.txt  unchanged  5 bytes  md5:5d41402abc4b2a76b9719d911017c592
```

Behavior and limitations:

- It outputs **all** files — staged, unstaged and unchanged — with no filtering; `--file` is the only filter.
- **There is no `--json` branch.** Even with `--json` it still prints the text above and produces no JSON; use `status --json` when you need structured data.
- Unlike `status`, `diff` does **not** sync the staging area.
- The `+ server` line of a newly added file has no real data and shows `0 bytes` with an empty MD5; this is expected output.

## add

```bash
aly-publish add --all
aly-publish add <file1> <file2> ...
```

| Mode | Needs the server | Behavior |
|---|---|---|
| `--all` | Yes | Runs a diff scan first and stages only `new` and `modified` |
| Explicit files | No | Computes the local files' MD5 and size directly and then stages them |

Examples:

```bash
aly-publish add --all
aly-publish add DemoApp.dll config/settings.json
```

JSON:

```json
{"isSuccess":true,"errorMsg":"","data":{"added":2}}
```

Behavior and limitations:

- `--all` does not stage `deleted` files, and does not stage files that are already staged.
- Explicit-file mode **does not query the server**, so it cannot tell whether a file is new or modified; the staged entry is always recorded with status `modified`. The real `new`/`modified` decision is still based on the server manifest at `status`/`push` time.
- Explicit files must exist in the project directory; a missing path returns a business failure of the form `hash <path>: ...`.
- Repeating `add` for the same path is idempotent (an existing entry is skipped), but the `added` count in the JSON is the **number of files in this request**, not the number of entries actually added.
- With no arguments and no `--all`, it only prints usage, which is a business failure (exit code 0):

  ```json
  {"isSuccess":false,"errorMsg":"Usage: aly-publish add [--all | <file>...]","data":null}
  ```

## reset

```bash
aly-publish reset --all
aly-publish reset <file1> <file2> ...
```

- `--all` clears the whole staging area, and the JSON `data` is `null`.
- Explicit files are removed by relative path; entries that do not exist are ignored.

```json
{"isSuccess":true,"errorMsg":"","data":{"removed":2}}
```

Human output:

```text
已从暂存区移除2 个文件
```

`reset` only changes the local staging area and does not need the server. With no arguments and no `--all` it prints usage and returns a business failure.

## staged

```bash
aly-publish staged
```

Human output:

```text
Staged files:
  [new] .updator/shared.json  234 bytes
  [modified] sub/b.bin  11 bytes
```

When the staging area is empty it prints `暂存区为空`.

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "relativePath": ".updator/shared.json",
      "status": "new",
      "localMd5": "97a209b3de6950d03a6469dea098fd7d",
      "localSize": 234
    },
    {
      "relativePath": "sub/b.bin",
      "status": "modified",
      "localMd5": "96c15c2bb2921193bf290df8cd85e2ba",
      "localSize": 11
    }
  ]
}
```

Entries are sorted by `relativePath`. When the staging area is empty, `data` is `null`.

## push

```bash
aly-publish push --version <version> --message <message> [--message <message> ...] \
  [--dry-run] [--force] [--after-apply-update-script <script>] [--set-force-update]
```

| Flag | Type | Required | Description |
|---|---|---|---|
| `--version` | string | **Yes** | New version number; must be strictly greater than the server's latest version |
| `--message` | string[] | **Yes** (at least one) | Release note; may be specified repeatedly |
| `--dry-run` | bool | No | Only lists the files and the version that would be pushed; nothing is actually pushed |
| `--force` | bool | No | Skips the MD5 re-check of staged files |
| `--after-apply-update-script` | string | No | Script to run after the update; the path is relative to the application directory |
| `--set-force-update` | bool | No | Marks the project as force update after a successful push |

### Prerequisites and flow

1. The server URL and `project.name` must be resolvable.
2. The staging area must be non-empty; when it is empty it returns "暂存区为空，请先 add 文件".
3. At least one `--message`; when missing it returns "至少需要一条 `--message`".
4. By default it recomputes the MD5 of staged files and compares it with the value recorded by `add`. A mismatch means the file was modified after staging:

   ```json
   {"isSuccess":false,"errorMsg":"以下文件在 add 后被修改，请重新 add: [DemoApp.dll]","data":null}
   ```

   `--force` skips this step and uploads the current files directly.
5. Uploads run concurrently. The concurrency is the CPU count, clamped to 3–16. A file of at most 1 MiB is uploaded directly; a file larger than 1 MiB is uploaded in 1 MiB chunks, and the server is asked to merge them when the chunks are complete. Each file/chunk is attempted at most 5 times (first attempt + 4 retries, with backoff of 400/800/1600/3200 ms).
6. After every upload succeeds it calls `POST /api/project/publish_version` to register the version, with the time taken from the local `yyyy-MM-dd HH:mm:ss`.
7. After a successful registration it clears the staging area and updates the project's force-update flag according to the value of `--set-force-update`.

### JSON progress stream

`push --json` is not a single-line output but line-delimited JSON:

1. The `START` line of every file (in staging order, all emitted up front);
2. A `DONE` or `FAIL` line per file (in the original order);
3. One final result line (on success `data` contains `version` and `files`; on failure `isSuccess: false`);
4. One terminator line: `{"isSuccess":true,"errorMsg":"","data":null}`.

Success example (2 files):

```json
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"START","file_size":234}}
{"isSuccess":true,"errorMsg":"","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"START","file_size":90052}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"DONE","file_size":234}}
{"isSuccess":true,"errorMsg":"","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"DONE","file_size":90052}}
{"isSuccess":true,"errorMsg":"","data":{"files":"2","version":"V1.0.2"}}
{"isSuccess":true,"errorMsg":"","data":null}
```

The progress data fields are `index`, `total`, `file`, `status`, `file_size` and `error` (only present on failure). `data.files` in the final result is a **string**.

Upload failure example:

```json
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"START","file_size":234}}
{"isSuccess":true,"errorMsg":"","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"START","file_size":90052}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":2,"file":".updator/shared.json","status":"DONE","file_size":234}}
{"isSuccess":false,"errorMsg":"DemoApp.dll: 磁盘已满","data":{"index":2,"total":2,"file":"DemoApp.dll","status":"FAIL","file_size":90052,"error":"磁盘已满"}}
{"isSuccess":false,"errorMsg":"上传失败 [DemoApp.dll]: 磁盘已满","data":null}
{"isSuccess":true,"errorMsg":"","data":null}
```

A consumer should stop at "the `isSuccess: true` line whose `data` is `null`" rather than simply reading until EOF.

A failed precondition (missing config, empty staging area, missing `--message`, MD5 conflict) never enters the progress stream: it emits only **one** failure JSON line, with no START/DONE/FAIL and no terminator. Only after uploading begins does the full progress stream appear and end with the terminator.

### Failure semantics

- If any file fails during upload, the remaining files still finish uploading, and then the failure result and the terminator follow; the staging area is **not** cleared, so you can retry directly after fixing the problem.
- If all files upload successfully but version registration fails (for example because the version is not monotonic):
  `errorMsg` has the form "创建版本失败: POST /api/project/publish_version: 版本 ...（文件已上传，可重试 push）". The server files are in place but the version is not registered; retry with a higher version number.
- Every business failure exits with code 0, so you must check `isSuccess`.

### dry-run

`--dry-run` returns after the MD5 re-check and before uploading, printing a human-readable plan:

```text
[DRY RUN] 将上传以下文件：
  .updator/shared.json
  DemoApp.dll
[DRY RUN] 将创建版本 V1.0.3 (1 条日志)
[DRY RUN] 未实际推送任何内容。
```

Limitations:

- dry-run **does not produce** a JSON progress stream. Even with `--json` the output is still the text above; with `--quiet` there is no output at all.
- It does not verify that the version is greater than the server's latest version (that check happens only when the server registers the version).
- It does not clear the staging area.

### Force-update flag

`--set-force-update` means "mark the project as force update after a successful push", so the client updates without asking when it checks for updates.

::: warning Behavior of the current implementation
After a successful push the CLI always calls `set_force_update`, passing the current value of `--set-force-update`. In other words, a successful push that omits the flag sets the project's force-update flag to `false`. If the project was already in force-update mode and needs to stay there, pass `--set-force-update` explicitly.
:::

### Examples

```bash
# Normal release
aly-publish push --version V1.0.2 --message "修复登录超时" --message "优化大文件下载速度"

# Rehearse first
aly-publish push --version V1.0.3 --message "重构上传" --dry-run

# Script + force update
aly-publish push --version V1.0.4 --message "升级数据库" \
  --after-apply-update-script update.bat --set-force-update
```

## log

```bash
aly-publish log [--limit <count>]
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--limit` | int | 20 | Show only the most recent N entries |

Human output:

```text
V1.0.1 (2026-11-10 10:45:42)
  • 修复登录超时
  • 优化大文件下载速度
V1.0.0 (2026-11-03 10:12:33)
  • 第一次创建
```

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 7,
      "project_id": 3,
      "version": "V1.0.1",
      "logs": ["修复登录超时", "优化大文件下载速度"],
      "time": "2026-11-10 10:45:42",
      "created_at": "2026-11-10T10:45:42+08:00",
      "is_deleted": false,
      "after_apply_update_script": ""
    }
  ]
}
```

Notes:

- The server returns logs in descending ID order (newest first), and the CLI takes the first `--limit` entries.
- The project name comes from `project.name` or `--project`. The CLI strictly validates only the server URL; when the project name is empty, the server rejects the request and a business failure is returned.
- `--limit 0` returns an empty array; do not pass a negative number.

## server info

```bash
aly-publish server info
```

Human output:

```text
OS:              windows
Platform:        Microsoft Windows 11 Pro
Architecture:    amd64
Go Version:      go1.25.0
CPU:             AMD Ryzen 7 5800X
Cores:           16 @ 3800 MHz
Disk:            120.5 GiB used / 460.7 GiB total (26.2%)
```

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "os": "windows",
      "platform": "Microsoft Windows 11 Pro",
      "goARCH": "amd64",
      "version": "go1.25.0",
      "numCPU": 16,
      "cpuName": "AMD Ryzen 7 5800X",
      "cpuMhz": 3800,
      "diskUsed": 120.5,
      "diskFree": 340.2,
      "diskTotal": 460.7,
      "diskUsedPercent": 26.2
    }
  ]
}
```

- This command needs only the server URL, not the project config.
- When the server returns no data, the human output is `无服务器信息`, and the JSON `data` is `null` or an empty array (depending on the server response).
- Disk capacity is measured in GiB with one decimal place.

## Next steps

- Meaning of the config items: [Config Commands](/en/docs/publish/commands-config)
- Preparing a server project: [Project Commands](/en/docs/publish/commands-project)
- Graphical workflow: [Publish GUI](/en/docs/publish/gui)
- Back to the tool overview: [aly-publish](/en/docs/publish)