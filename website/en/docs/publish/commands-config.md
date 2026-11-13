---
title: Config Commands (config)
description: Usage, flags, JSON output, behavioral boundaries and limitations of aly-publish config init/set/set-array/get/list/path.
---

# Config Commands (config)

`config` manages `.updator/` in the current working directory:

| File | Contents | Uploaded |
|---|---|---|
| `.updator/shared.json` | `server_url`, `project_name`, `ignore_folders/files`, `un_copy_folders/files` | Yes (distributed with each version) |
| `.updator/publish.json` | CLI-specific config, currently the empty object `{}` | No |

All commands read config from the **current working directory**; there is no flag for pointing them at another directory, so `cd` first. The global `--server` and `--project` flags override the values read by this invocation but are not written back to the file.

::: tip JSON example formatting
The CLI actually emits single-line JSON. The longer examples below are indented for readability; fields and values are unchanged.
:::

## Readable and writable keys

| Key | Type | Commands | Description |
|---|---|---|---|
| `server.url` | string | `set`, `get`, `list` | Server URL |
| `project.name` | string | `set`, `get`, `list` | Server project name |
| `ignore.folders` | string[] | `set-array`, `list` | Directories skipped during scanning (relative to the project root) |
| `ignore.files` | string[] | `set-array`, `list` | Files skipped during scanning (globs supported) |
| `un_copy.folders` | string[] | `set-array`, `list` | Directories not copied when the client applies an update |
| `un_copy.files` | string[] | `set-array`, `list` | Files not copied when the client applies an update |

`set` / `get` recognize only the two scalar keys. Arrays must go through `set-array`; `get` cannot read an array, and `set` cannot replace an array as a whole.

## config init

Creates `.updator/` and writes `shared.json` and `publish.json`.

```bash
aly-publish config init --project <project> [--server <server-url>] \
  [--ignore-folders <comma-separated>] [--ignore-files <comma-separated>] \
  [--un-copy-folders <comma-separated>] [--un-copy-files <comma-separated>]
```

| Flag | Type | Required | Description |
|---|---|---|---|
| `--project` | string | **Yes** | Written to `project_name` |
| `--server` | string | No | Written to `server_url`; can also be set later with `config set server.url` |
| `--ignore-folders` | string | No | Ignored directories, comma-separated |
| `--ignore-files` | string | No | Ignored files, comma-separated |
| `--un-copy-folders` | string | No | Directories not to copy, comma-separated |
| `--un-copy-files` | string | No | Files not to copy, comma-separated |

Behavior:

1. Load any existing `shared.json` in the current directory first, then override the corresponding fields with the command-line flags.
2. Write `shared.json`; fields that were not provided keep their previous values, and a first-time initialization uses empty strings / empty arrays.
3. Write `publish.json` with the fixed content `{}`.
4. Both files use an atomic "temp file + rename" write, so a crash cannot leave half-written JSON behind.

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "publish_path": "C:\\build\\dist\\.updator\\publish.json",
    "shared_path": "C:\\build\\dist\\.updator\\shared.json"
  }
}
```

Human output:

```text
配置已保存到: C:\build\dist\.updator
```

Limitations and notes:

- `--project` is required (unless the existing `shared.json` in the workspace already has a `project_name`): when neither is present the command **fails immediately** with `config init 必须通过 --project 指定项目名称` and writes nothing (fixed 2026-11-12, M14).
- `config init` only writes local files; it does not validate whether the project name is legal and does not create a server project. The naming rules are checked by the server in `project create` / `push`.
- Re-running it in an already initialized directory does not clear fields you did not specify; to clear an array use `config set-array <key> --clear`, and to clear a scalar use `config set <key> ""`. The CLI has no `unset` command.
- `publish.json` currently has no writable fields, so writing `{}` is expected.

## config set

Sets a single scalar config item.

```bash
aly-publish config set <key> <value>
```

| Argument | Description |
|---|---|
| `<key>` | Only `server.url` and `project.name` are supported |
| `<value>` | The string written verbatim; wrap it in quotes if it contains spaces |

```bash
aly-publish config set server.url http://192.168.1.100:2000
aly-publish config set project.name myapp
```

Success output (human):

```text
配置已更新 server.url = http://192.168.1.100:2000
```

JSON:

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

Behavior and limitations:

- Reads the existing config and writes it back to `shared.json` and `publish.json`; if the directory does not exist, `.updator/` is created automatically, but running `config init` first is preferable.
- Unknown keys return a business failure (the exit code is still 0):

  ```json
  {"isSuccess":false,"errorMsg":"unknown config key 'output.format'","data":null}
  ```

- There is no `output.format` config item; the output format is decided per invocation by `--json`.
- `server.url` must contain only `scheme://host:port`, with no path prefix. The API client appends `/api/...`, so a path would make requests land on the wrong address; a trailing `/` is stripped automatically.
- `set` does not check whether the server is reachable, nor whether the project exists.

## config set-array

Appends to, removes from or clears an array config item.

```bash
aly-publish config set-array <key> --add <item> | --remove <item> | --clear
```

| Flag | Type | Description |
|---|---|---|
| `<key>` | string | `ignore.folders`, `ignore.files`, `un_copy.folders`, `un_copy.files` |
| `--add` | string | Append one item |
| `--remove` | string | Remove every exactly matching item |
| `--clear` | bool | Clear the whole array |

```bash
aly-publish config set-array ignore.folders --add logs
aly-publish config set-array ignore.folders --add temp
aly-publish config set-array ignore.folders --remove temp
aly-publish config set-array ignore.files --clear
```

The execution order is fixed: `--clear` → `--add` → `--remove`. All three flags may appear together, though normally you use only one.

Success output (human):

```text
配置已更新 ignore.folders
```

JSON:

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

Behavior and limitations:

- **Only one item can be added or removed per call.** `--add` is a plain string flag, not a repeatable one; writing `--add a --add b` keeps only the last value, `b`. To add several items, call the command multiple times.
- Appending does **not** deduplicate; running `--add` twice with the same item produces two identical entries. `--remove` removes every exactly matching entry.
- Matching is a string comparison and does not support globs; `*.log` is only stored as a pattern string, and the actual wildcard matching happens during scanning or when the client uses it.
- To inspect an array's current value use `config list`; `config get ignore.folders` returns an empty string and cannot be used to read arrays.
- Unknown keys return the business failure `unknown config key '...'`.

## config get

Reads a single scalar config item.

```bash
aly-publish config get <key>
```

```bash
aly-publish config get server.url
# http://192.168.1.100:2000
```

JSON:

```json
{"isSuccess":true,"errorMsg":"","data":{"server.url":"http://192.168.1.100:2000"}}
```

Behavior and limitations:

- Only `server.url` and `project.name` are supported.
- Unknown keys and array keys **do not error**: they return an empty string with `isSuccess` set to `true`. This is the easiest place to misjudge things — an empty value may mean "not configured" or "the key does not exist".

  ```json
  {"isSuccess":true,"errorMsg":"","data":{"ignore.folders":""}}
  ```

- The command output applies this invocation's `--server` / `--project` overrides; for example, `config get server.url --server http://10.0.0.1:2000` returns the temporary value and does not write it back.

## config list

Lists all config.

```bash
aly-publish config list
```

Human output (note that the spacing is not perfectly aligned across lines):

```text
server.url      = http://192.168.1.100:2000
project.name    = myapp
ignore.folders   = [logs temp]
ignore.files     = [*.log]
un_copy.folders  = [docs]
un_copy.files    = [*.md]
```

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "publish": {},
    "shared": {
      "server_url": "http://192.168.1.100:2000",
      "project_name": "myapp",
      "ignore_folders": ["logs", "temp"],
      "ignore_files": ["*.log"],
      "un_copy_folders": ["docs"],
      "un_copy_files": ["*.md"]
    }
  }
}
```

Note that the JSON uses the raw `shared` field names (`server_url`, `project_name`, etc.), unlike the dotted key names in the human output.

Limitations:

- It outputs all fields at once; there are no filter flags.
- `publish` is always `{}` for now, because `publish.json` has no fields yet.
- It does not include the staging area; use `aly-publish staged` to inspect it.
- As with `config get`, the output reflects this invocation's `--server` / `--project` overrides.

## config path

Shows the `.updator/` directory path (not a particular file).

```bash
aly-publish config path
# C:\build\dist\.updator
```

JSON:

```json
{"isSuccess":true,"errorMsg":"","data":{"path":"C:\\build\\dist\\.updator"}}
```

The path is assembled from the current working directory, so the result depends on the directory the command is run in.

## Limitations at a glance

| Limitation | Description |
|---|---|
| `set` / `get` support only two scalar keys | `server.url`, `project.name` |
| Arrays can only be modified through `set-array` | `get` returns an empty string for arrays |
| `set-array` handles one item per call | Repeated `--add` keeps only the last item |
| `init` requires a project name | Without `--project` and without an existing `project_name` it fails instead of silently writing an empty config |
| Config always belongs to the current directory | There is no `--path` / `--cwd` |
| There is no `output.format` | The output format is controlled only by `--json` |
| There is no `unset` / delete-key command | To clear something, use `set-array --clear` or edit the JSON by hand |

## Next steps

- Create and manage server projects: [Project Commands](/en/docs/publish/commands-project)
- Once configured, start comparing and pushing: [Workflow Commands](/en/docs/publish/commands-workflow)
- Back to the tool overview: [aly-publish](/en/docs/publish)