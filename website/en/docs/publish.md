---
title: Packaging & publishing
description: The day-to-day aly-publish flow — command tree, status → add → push, version rules and exit-code semantics; to get running, read “Publish your first release”.
---

# Packaging & publishing

`aly-publish` works inside a **build output directory**: it compares local artifacts against the server manifest, stages the changes, then uploads files and registers a new version. The server keeps the files and the version log; the terminal [aly-client](/en/docs/commands) performs updates from them.

It does not compile or package, and it does not create projects on the server — that is the job of `project create`. Two common misconceptions follow: `config init` only writes local config, and `push` only uploads files that are already staged.

To get running see [Publish your first release](/en/docs/start/publish); for the desktop app see [Publish GUI](/en/docs/publish/gui).

## CLI and GUI

| Aspect | `aly-publish` (CLI) | `AlyPublish.exe` (GUI) |
|---|---|---|
| Form | Single-file command-line program | Go/MyGo native desktop app (default; C#/Avalonia is historical) |
| Best for | CI, scripts, batch publishing | Day-to-day manual releases |
| Config | Reads and writes `.updator/` in the current directory directly | Calls the CLI; reads and writes the same config |
| Output | human text or `--json` | Parses the CLI's `--json` output and displays progress |
| Dependency | None — the CLI is the `publish-cli/<rid>/` copy inside the package | Must be able to find `publish-cli/<cli_rid>/aly-publish(.exe)` from the same package |
| Distribution | Ships inside the AlyPublish package (`publish-cli/<rid>/`); there is no standalone CLI asset | Install the AlyPublish package |

The GUI is only a caller of the CLI: the `.updator/` config and the staging area are identical, so the two can be mixed.

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
| `config init/set/set-array/get/list/path` | Read and write config under `.updator/` | [Config commands](/en/docs/publish/commands-config) |
| `project list/create/update/delete/info` | Manage server-side projects | [Project commands](/en/docs/publish/commands-project) |
| `status/diff/add/reset/staged/push/log/server info` | Compare, stage, push and inspect the log | [Workflow commands](/en/docs/publish/commands-workflow) |

This version has no `push-all`, `publish` or `watch` subcommands; a complete release is assembled from `status` → `add` → `push`.

## Typical release flow

```bash
# 1. Enter the build output directory (.updator/ is created here)
cd dist

# 2. Initialize the local config once; --project names the server project
aly-publish config init --project myapp --server http://192.168.1.100:2000

# 3. Register the project before the first server publish (once; any directory works)
aly-publish project create --name myapp --title "My App"

# 4. Three steps for day-to-day releases
aly-publish status           # what changed this time
aly-publish add --all        # stage new / modified / deleted files
aly-publish push --version V1.0.1 --message "Fix login timeout"
```

A few constraints:

- `config init` does **not** create a server project; `project create` does **not** write local config. Keep the two sides consistent through `project_name` and `--name`.
- `push` can only upload files in the staging area; when it is empty, `push` fails immediately.
- `add --all` stages `new` / `modified` / `deleted`; files deleted locally are removed on the server by `push` (`file/delete_file`), disappear from the release manifest, and the client deletes the stale local copies.

## Config files

The three files under `.updator/` — `shared.json` (shared with the client, uploaded with each version), `publish.json` and `staging/staged-files.json` — are documented field by field in [Configuration files](/en/docs/client/config).

`shared.json` is uploaded with each version, so the terminal `aly-client` inherits the same server URL, project name and ignore rules; publish one release after changing the rules and they take effect.

## Version number rules

When creating a project, the server writes the initial version **V1.0.0** and inserts a changelog entry whose content is “first creation”. The new version passed to `push` must be **strictly greater** than the latest version published for that project (the server compares the highest version across all non-deleted logs), otherwise it is rejected:

```json
{"isSuccess":false,"errorMsg":"创建版本失败: POST /api/project/publish_version: 版本 V1.0.0 不大于已发布的最新版本 V1.0.1（防倒退保护；如需回退内容，请发布更高的版本号）（文件已上传，可重试 push）","data":null}
```

Two practical rules follow:

1. **The first publish must start at V1.0.1**; V1.0.0 is taken by the changelog entry created with the project.
2. If you really need to roll content back, publish a **higher** version number and re-upload the old files; the version number itself never goes backwards.

The version format is validated by the server: `^[vV]?\d+(\.\d+)*(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`, at most 50 characters. The `V`/`v` prefix is optional; comparisons ignore the prefix and build metadata, and a pre-release sorts below the final version with the same number (`V1.0.1-beta` < `V1.0.1`).

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
- In `--json` mode, both success and failure are a single line of JSON.
- `--quiet` suppresses only human text, not `--json`.
- Exit code 1 only means the command never really ran (rejected at the flag level); it does not mean the server returned a business failure.

The updater (`aly-client`) has its own exit codes and progress-line semantics — see [Command reference · Common conventions](/en/docs/commands#common-conventions).

## Next steps

- Config details: [Config commands](/en/docs/publish/commands-config) · [Configuration files](/en/docs/client/config)
- Server projects: [Project commands](/en/docs/publish/commands-project)
- Day-to-day publishing: [Workflow commands](/en/docs/publish/commands-workflow)
- Desktop workflow: [Publish GUI](/en/docs/publish/gui) · inside the package: [Package layout & distribution](/en/docs/guide/package)
