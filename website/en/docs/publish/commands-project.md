---
title: Project Commands (project)
description: Flags, JSON output, name and title rules, and soft-delete behavior of aly-publish project list/create/update/delete/info.
---

# Project Commands (project)

`project` manages **server-side** project records: a project is the container for files and version logs, and terminal clients pull updates by project name. It is a separate set of data from the local `.updator/` config.

| Command | Purpose | Needs the server | Needs local `.updator/` |
|---|---|---|---|
| `project list` | List non-deleted projects | Yes | No |
| `project create` | Create a project with initial version V1.0.0 | Yes | No |
| `project update` | Update the title, force update and ignore rules | Yes | No |
| `project delete` | Soft-delete a project | Yes | No |
| `project info` | Show the details of a single project | Yes | No |

The server URL comes from `--server` or from `server_url` in the current directory's `shared.json`. These commands do not use the global `--project` (they take their own `--name`) and do not require `project.name` to be configured.

## Distinguishing `--name`, `--title` and `--project`

| Flag | Owner | Purpose | Writes local config |
|---|---|---|---|
| `--name` | The `project` subcommands' own flag | Unique identifier of the server project; determines the data directory name | No |
| `--title` | Flag of `project create/update` | Title shown to end users | No |
| `--project` | Global flag | Overrides `project_name` in `shared.json` for commands such as `status`/`push` | No |
| `config init --project` | `config init` uses the global flag | Writes the project name into `shared.json` | **Yes** |

The three are not synchronized automatically. The usual practice is to keep `config init --project` and `project create --name` identical, but the CLI does not check this for you; when they differ, `status` looks up a project that does not exist.

## Name and title rules

`--name` is validated by the server:

- Length 1–64;
- Must not contain `< > : " / \ | ? *`;
- Must not be a Windows reserved name (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`, case-insensitive);
- Globally unique. Soft-deleted projects still occupy their name, so recreating a project with the same name returns "项目名称:xxx已存在".

`--title` is validated as non-empty by the server. Both `project create` and `project update` mark `--title` as a required flag, so even when you only want to change the force-update switch you must pass a title.

## project list

```bash
aly-publish project list [--server <server-url>]
```

Human output:

```text
  ID   NAME     TITLE      VERSION  FORCE UPDATE CREATED
   3    myapp    我的应用   V1.0.1   no           2026-11-03
```

- `CREATED` shows only the first 10 characters of the date (`yyyy-MM-dd`).
- Soft-deleted projects do not appear.

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 3,
      "name": "myapp",
      "title": "我的应用",
      "version": "V1.0.1",
      "force_update": false,
      "ignore_folders": ["logs", "temp"],
      "ignore_files": ["*.log"],
      "created_at": "2026-11-03T10:12:33+08:00",
      "is_deleted": false
    }
  ]
}
```

The fields come from the server's ent model and use snake_case; `created_at` is an RFC3339 timestamp with a time zone.

## project create

```bash
aly-publish project create --name <name> --title <title> \
  [--force-update] [--ignore-folders <comma-separated>] [--ignore-files <comma-separated>]
```

| Flag | Type | Required | Description |
|---|---|---|---|
| `--name` | string | **Yes** | Project unique identifier |
| `--title` | string | **Yes** | Project title |
| `--force-update` | bool | No | Project-level force update; defaults to false |
| `--ignore-folders` | string | No | Ignored directories, comma-separated |
| `--ignore-files` | string | No | Ignored files, comma-separated |

Behavior:

1. The server validates the name and the title.
2. It checks for a project with the same name (including soft-deleted ones) and returns a business failure if one exists.
3. It creates the `data/<name>/` directory.
4. It writes the project record with version **V1.0.0** and inserts a changelog entry whose content is "第一次创建" ("first creation").
5. It returns the created project object.

```bash
aly-publish project create --name myapp --title "我的应用" --ignore-folders "logs,temp" --ignore-files "*.log"
```

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 3,
    "name": "myapp",
    "title": "我的应用",
    "version": "V1.0.0",
    "force_update": false,
    "ignore_folders": ["logs", "temp"],
    "ignore_files": ["*.log"],
    "created_at": "2026-11-03T10:12:33+08:00",
    "is_deleted": false
  }
}
```

Human output:

```text
项目创建成功: [3] myapp (V1.0.0)
```

Notes:

- `project create` does not write `.updator/shared.json` and does not copy any local files.
- The initial version V1.0.0 occupies that version number. The first `push` must use a higher version (for example V1.0.1), or it is rejected by the anti-rollback check.
- `--force-update` only affects the client's confirmation behavior when updating; it has nothing to do with the version number.

## project update

```bash
aly-publish project update --name <name> --title <title> \
  [--force-update] [--ignore-folders <comma-separated>] [--ignore-files <comma-separated>]
```

| Flag | Type | Required | Description |
|---|---|---|---|
| `--name` | string | **Yes** | Locates the project; the name itself cannot be changed |
| `--title` | string | **Yes** | New title |
| `--force-update` | bool | No | Defaults to false |
| `--ignore-folders` | string | No | Defaults to empty (cleared) |
| `--ignore-files` | string | No | Defaults to empty (cleared) |

This is a **whole-field overwrite**, not a partial patch: the title, force update and both ignore arrays are written from this invocation's flags. Arrays that are not passed are cleared, and a `--force-update` that is not passed is written as `false`. So even when you only want to change the title, pass the current ignore rules and force-update state as well, or they will be cleared.

```bash
# Keeping the existing ignore rules and force-update state
aly-publish project update --name myapp --title "我的应用 V2" \
  --force-update --ignore-folders "logs,temp" --ignore-files "*.log"
```

Success JSON:

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

Human output:

```text
项目更新成功: myapp
```

Limitations:

- The project cannot be renamed. Renaming requires creating a new project and then pointing the local `config set project.name` at the new name.
- Only non-soft-deleted projects can be updated; a missing project returns a business failure.
- It does not modify the version number and does not affect existing version logs.
- After the ignore rules change, the server clears the project's file-list cache, and the next `get_all_files` collects files again under the new rules.

## project delete

```bash
aly-publish project delete --name <name>
```

| Flag | Type | Required | Description |
|---|---|---|---|
| `--name` | string | **Yes** | Name of the project to delete |

Behavior: a soft delete that only sets `is_deleted` to `true`.

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

Human output:

```text
项目已删除: old_project
```

Consequences of a soft delete:

- `project list` and `project info` no longer return the project;
- The server's files and version logs are **not** physically deleted, and existing configuration on terminals is not modified as a side effect;
- The project name stays occupied. Running `project create` again with the same name yields "项目名称:old_project已存在";
- Because the data still exists, a soft delete is not the same as "make terminals stop updating immediately"; if you need to stop distribution, handle the server data and the terminal configuration separately.

## project info

```bash
aly-publish project info --name <name>
```

| Flag | Type | Required | Description |
|---|---|---|---|
| `--name` | string | **Yes** | Project name |

Although the `--id` flag is still registered in the code, it is marked hidden and does not participate in the lookup; `--name` is the required flag, and `--id` alone does not pass flag validation. Query by name.

Human output:

```text
Project: myapp
Title:   我的应用
Version: V1.0.1
Force Update: no
Created: 2026-11-03

Ignore Folders:
  logs
  temp
Ignore Files:
  *.log
```

JSON:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 3,
    "name": "myapp",
    "title": "我的应用",
    "version": "V1.0.1",
    "force_update": false,
    "ignore_folders": ["logs", "temp"],
    "ignore_files": ["*.log"],
    "created_at": "2026-11-03T10:12:33+08:00",
    "is_deleted": false
  }
}
```

Internally, `project info` first fetches all non-deleted projects and then matches the name exactly on the local side; a missing project returns a business failure (exit code 0):

```json
{"isSuccess":false,"errorMsg":"项目不存在","data":null}
```

## Next steps

- How to write local config: [Config Commands](/en/docs/publish/commands-config)
- Start comparing and pushing: [Workflow Commands](/en/docs/publish/commands-workflow)
- Back to the tool overview: [aly-publish](/en/docs/publish)