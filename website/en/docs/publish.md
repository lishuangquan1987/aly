---
title: Publisher Tool aly-publish
description: What aly-publish does, how the CLI and the GUI divide the work, global flags, the command tree, a typical release flow, config files and exit-code semantics.
---

# Publisher Tool aly-publish

`aly-publish` is the publishing-side command-line tool (Go + cobra). It works inside a **build output directory**: it compares local artifacts against the server manifest, stages the changes, then uploads files and registers a new version. Files and version logs are stored on the server, where the terminal [aly-client](/en/docs/commands) uses them to perform updates.

It does not compile or package, and it does not create projects on the server — that is the job of `project create`. Two common misconceptions follow from this: `config init` only writes local config, and `push` only uploads files that are already staged.

## CLI and GUI

| Aspect | `aly-publish` (CLI) | `AlyPublish.exe` (GUI) |
|---|---|---|
| Form | Single-file command-line program | Avalonia desktop application |
| Best for | CI, scripts, batch publishing | Day-to-day manual releases |
| Config | Reads and writes `.updator/` in the current directory directly | Calls the CLI; reads and writes the same config |
| Output | human text or `--json` | Parses the CLI's `--json` output and displays progress |
| Dependency | None | Must be placed in the same directory as `aly-publish.exe` |
| Release assets | Prebuilt for Windows / Linux / macOS | Not built by the repository's release workflow; must be built from source |

GUI details are in [Publish GUI](/en/docs/publish/gui). The two can be mixed: the GUI is only a caller of the CLI, and the `.updator/` config and staging area are exactly the same.

## Global flags

The following flags apply to all subcommands and may be written before or after the subcommand:

| Flag | Type | Description |
|---|---|---|
| `--server` | string | Overrides `server_url` in `shared.json` for this invocation only |
| `--project` | string | Overrides `project_name` in `shared.json` for this invocation only |
| `--json` | bool | Emits single-line JSON; `push` emits line-delimited progress JSON |
| `--quiet` | bool | Suppresses human text; does not affect structured `--json` output |

::: tip The working directory determines the config
Like git, `aly-publish` always reads `.updator/` from the **current working directory**; there is no `--cwd`-style flag. `cd` into the build output directory (for example `dist/`) before publishing.
:::

## Command tree

```text
aly-publish
├── config                  # local config (.updator/)
│   ├── init                #   initialize shared.json + publish.json
│   ├── set                 #   set server.url / project.name
│   ├── set-array           #   add or remove ignore / un_copy array entries
│   ├── get                 #   read a single scalar setting
│   ├── list                #   list all settings
│   └── path                #   show the .updator/ path
├── project                 # server-side project lifecycle
│   ├── list                #   list non-deleted projects
│   ├── create              #   create a project (initial version V1.0.0)
│   ├── update              #   update title, force update and ignore rules
│   ├── delete              #   soft-delete a project
│   └── info                #   show project details
├── status                  # local-vs-server diff overview (also syncs the staging area)
├── diff                    # per-file comparison (human-readable output only)
├── add                     # stage changes (--all or explicit paths)
├── reset                   # unstage
├── staged                  # show the staging area
├── push                    # upload staged files and register a version
├── log                     # view the version changelog
└── server info             # show server system information
```

| Command | Purpose | Reference |
|---|---|---|
| `config init/set/set-array/get/list/path` | Read and write config under `.updator/` | [Config Commands](/en/docs/publish/commands-config) |
| `project list/create/update/delete/info` | Manage server-side projects | [Project Commands](/en/docs/publish/commands-project) |
| `status/diff/add/reset/staged/push/log/server info` | Compare, stage, push and inspect the log | [Workflow Commands](/en/docs/publish/commands-workflow) |

This version has no `push-all`, `publish` or `watch` subcommands; a complete release is assembled from `status` → `add` → `push`.

## Typical release flow

```bash
# 1. Enter the build output directory (.updator/ is created here)
cd dist

# 2. Initialize the local config once; --project names the server project
aly-publish config init --project myapp --server http://192.168.1.100:2000

# 3. Register the project before the first server publish (once; any directory works)
aly-publish project create --name myapp --title "我的应用"

# 4. Three steps for day-to-day releases
aly-publish status           # what changed this time
aly-publish add --all        # stage new / modified / deleted files
aly-publish push --version V1.0.1 --message "修复登录超时"
```

A few constraints:

- `config init` does **not** create a server project; `project create` does **not** write local config. The two sides are kept consistent through `project_name` and `--name`.
- `push` can only upload files in the staging area. When the staging area is empty, it fails immediately.
- `add --all` stages `new` / `modified` / `deleted`; files deleted locally are removed on the server by `push` (`file/delete_file`), disappear from the release manifest, and the client then deletes the stale local copies.

## Config files

| File | Owner | Uploaded | Description |
|---|---|---|---|
| `.updator/shared.json` | Shared by the CLI and the client | **Yes** | Server URL, project name, ignore rules, no-copy rules |
| `.updator/publish.json` | CLI-only | No | Currently an empty object, reserved for CLI-specific config |
| `.updator/staging/staged-files.json` | CLI-only | No | Staging entries, written by `add` and read by `push` |

`shared.json` is uploaded as an ordinary file with each version, so the `aly-client` on the terminal inherits the same server URL, project name and ignore rules:

```json
{
  "server_url": "http://192.168.1.100:2000",
  "project_name": "myapp",
  "ignore_folders": ["logs", "temp"],
  "ignore_files": ["*.log", ".DS_Store"],
  "un_copy_folders": ["docs"],
  "un_copy_files": ["*.md"]
}
```

`publish.json` is currently always written as `{}`:

```json
{}
```

`staged-files.json` is an array of staging entries, recording the MD5 and size at the time the file was staged:

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

When scanning the publish directory, `.updator/publish.json` and `.updator/staging/` are always skipped, while `.updator/shared.json` is uploaded. `un_copy_folders` / `un_copy_files` only affect how the client copies files when applying an update; they do not participate in the publish scan.

## Exit codes and output contract

| Scenario | Exit code | How to tell |
|---|---|---|
| Command succeeded | 0 | `isSuccess` is `true` |
| Business failure: missing config, network error, server rejection, empty staging area, version conflict | **0** | `isSuccess` is `false` and `errorMsg` gives the reason |
| Usage error: unknown command/flag, missing command-level required flag, wrong flag type | 1 | cobra prints `Error: ...` and the usage |

**Business failures also return 0**, which is the easiest trap when wiring scripts. Parse `isSuccess` from the output instead of looking only at the exit code:

```json
{"isSuccess":false,"errorMsg":"暂存区为空，请先 add 文件","data":null}
```

- In human mode, business errors are written to stderr with an `Error:` prefix.
- In `--json` mode, error JSON is written to stdout; both success and failure are a single line of JSON.
- `--quiet` suppresses only human text, not `--json`.
- Exit code 1 only means the command never really ran (it was rejected at the flag level); it does not mean the server returned a business failure.

## Version number rules

When creating a project, the server writes the initial version **V1.0.0** and inserts a changelog entry whose content is "第一次创建" ("first creation"). The new version passed to `push` must be **strictly greater** than the latest version already published for that project (the server compares the highest version across all non-deleted logs), otherwise it is rejected:

```json
{"isSuccess":false,"errorMsg":"创建版本失败: POST /api/project/publish_version: 版本 V1.0.0 不大于已发布的最新版本 V1.0.1（防倒退保护；如需回退内容，请发布更高的版本号）（文件已上传，可重试 push）","data":null}
```

Two practical rules follow:

1. **The first publish must start at V1.0.1**; V1.0.0 is already taken by the changelog entry created with the project.
2. If you really need to roll content back, publish a **higher** version number and re-upload the old files; the version number itself never goes backwards.

The version format is validated by the server: `^[vV]?\d+(\.\d+)*(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`, with a maximum length of 50. The `V`/`v` prefix is optional; comparisons ignore the prefix and build metadata, and a pre-release sorts below the final version with the same number (`V1.0.1-beta` < `V1.0.1`).

## Next steps

- First time here: get the CLI from [Download & Install](/en/docs/download) and walk through the server and the first version with [Quick Start](/en/docs/getting-started).
- Config details: [Config Commands](/en/docs/publish/commands-config)
- Server projects: [Project Commands](/en/docs/publish/commands-project)
- Day-to-day publishing: [Workflow Commands](/en/docs/publish/commands-workflow)
- Desktop workflow: [Publish GUI](/en/docs/publish/gui)