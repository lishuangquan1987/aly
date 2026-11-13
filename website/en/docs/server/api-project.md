---
title: Project API
description: "The 9 project endpoints of aly-server: create, update, delete, query, changelog, publish version, and server info, with parameters, curl examples, response fields, and error texts."
---

# Project API

The project endpoints handle the project lifecycle and version registration. They all live under `/api/project`; there are 9 in total.

All endpoints return the unified envelope (HTTP 200 + `isSuccess` / `errorMsg` / `data`) and do not use 4xx/5xx for failures. For the envelope and error semantics, see [API Overview](/en/docs/api).

## Common conventions

### Project name rules

The project name is also the directory name under `data/`, so it is validated against filesystem rules (`validateProjectName`):

| Rule | Description |
| --- | --- |
| Length | 1–64 characters |
| Allowed | Any character except those listed below, including Chinese characters and spaces |
| Forbidden characters | `<` `>` `:` `"` `/` `\` <code>&#124;</code> `?` `*` |
| Forbidden reserved names | `CON` `PRN` `AUX` `NUL` `COM1`–`COM9` `LPT1`–`LPT9` (case-insensitive) |
| Immutable | Cannot be renamed after creation; `name` in `update_project` only locates the record |

Response for an invalid name:

```json
{ "isSuccess": false, "errorMsg": "项目名称包含非法字符或长度不在1-64范围内", "data": null }
{ "isSuccess": false, "errorMsg": "项目名称不能使用系统保留名: CON", "data": null }
```

### `Project` fields in responses

`Project` is the ent entity serialized directly; its fields are snake_case, and zero-value fields are omitted (`omitempty`):

| Field | Type | Description |
| --- | --- | --- |
| `id` | int | Auto-increment primary key |
| `name` | string | Project name |
| `title` | string | Project title (display name) |
| `version` | string | Current version number; `V1.0.0` at creation |
| `force_update` | bool | Force-update flag; **omitted** when `false` |
| `ignore_folders` | string[] | Ignored folders; **omitted** when empty |
| `ignore_files` | string[] | Ignored files (glob); **omitted** when empty |
| `created_at` | string | Creation time, RFC 3339, with the local time-zone offset |
| `is_deleted` | bool | Soft-delete flag; **omitted** when `false` |
| `edges` | object | Association placeholder, always `{}` (`change_logs` is not preloaded) |

For full details, see [Data Models](/en/docs/server/api-models).

## Create Project

Creates a project record and also writes the first changelog entry.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/project/create_project` |
| Content-Type | `application/json` |

### Request parameters

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Project name; rules above |
| `title` | string | Yes | Project title; must not be empty |
| `isForceUpdate` | bool | No | Force-update flag; defaults to `false` |
| `ignoreFolders` | string[] | No | Ignored folders; empty by default |
| `ignoreFiles` | string[] | No | Ignored files (glob supported); empty by default |

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/project/create_project \
  -H "Content-Type: application/json" \
  -d '{
        "name": "demo2",
        "title": "Demo2",
        "isForceUpdate": true,
        "ignoreFolders": ["logs"],
        "ignoreFiles": ["*.log"]
      }'
```

### Response

`data` is the created `Project` object:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 2,
    "name": "demo2",
    "title": "Demo2",
    "version": "V1.0.0",
    "force_update": true,
    "ignore_folders": ["logs"],
    "ignore_files": ["*.log"],
    "created_at": "2026-11-10T10:59:35.6878741+08:00",
    "edges": {}
  }
}
```

### Behavior

- The project is created only when the name does not exist; if it already exists, the call fails immediately and does not overwrite it.
- Two records are written in the same transaction: the `projects` row (`version = V1.0.0`) and the first `project_change_logs` entry (`version = V1.0.0`, `logs = ["第一次创建"]`, `time` set to the server's current time as `yyyy-MM-dd HH:mm:ss`).
- The `data/{name}` directory is created automatically (skipped if it already exists).
- Concurrent duplicate creation is caught by the unique constraint on `name`; a conflict likewise returns "project name already exists" (`项目名称:X已存在`).

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称不能为空` | `name` missing or empty |
| `项目名称包含非法字符或长度不在1-64范围内` | Name contains forbidden characters or exceeds the length limit |
| `项目名称不能使用系统保留名: X` | Name is a reserved Windows device name |
| `项目抬头不能为空` | `title` missing or empty |
| `项目名称:X已存在` | A project with the same name exists (including soft-deleted projects) |
| `invalid character ...` | Request body is not valid JSON (the text comes from the JSON parser) |

## Update Project

Updates the title, force-update flag, and ignore rules. **The name cannot be changed, and neither can the version number.**

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/project/update_project` |
| Content-Type | `application/json` |

### Request parameters

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Target project name; must exist and not be soft-deleted |
| `title` | string | Yes | New title; must not be empty |
| `isForceUpdate` | bool | No | Force-update flag; defaults to `false` |
| `ignoreFolders` | string[] | No | New ignored-folder list; **fully overwrites** the old value |
| `ignoreFiles` | string[] | No | New ignored-file list; **fully overwrites** the old value |

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/project/update_project \
  -H "Content-Type: application/json" \
  -d '{"name":"demo","title":"Demo Project (renamed)","isForceUpdate":false}'
```

### Response

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### Behavior

- Ignore rules are **fully replaced**; omitting them clears them. If you only want to change the title, pass the existing `ignoreFolders` / `ignoreFiles` back as well, otherwise the rules are cleared and subsequent `get_all_files` calls will hand out files that were previously ignored.
- On success the server invalidates the project's file-list cache; the next `get_all_files` rescans with the new rules.
- Does not change `version`; the version can only advance through `publish_version`.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `项目抬头不能为空` | `title` missing or empty |
| `项目不存在` | Project does not exist or has been soft-deleted |

## Set Force Update

Writes only the `force_update` field, avoiding a read-modify-write that would overwrite concurrent changes.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/project/set_force_update` |
| Content-Type | `application/json` |

### Request parameters

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `projectName` | string | Yes | Project name |
| `forceUpdate` | bool | Yes | Target value; a missing value is treated as `false` |

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/project/set_force_update \
  -H "Content-Type: application/json" \
  -d '{"projectName":"demo","forceUpdate":true}'
```

### Response

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `项目不存在: X` | Project does not exist or has been soft-deleted (affected row count is 0) |

## Get All Projects

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/api/project/get_all_projects` |

### Request parameters

None.

### Request example

```bash
curl http://127.0.0.1:2000/api/project/get_all_projects
```

### Response

`data` is a `Project` array containing **only projects that are not soft-deleted**:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 2,
      "name": "demo2",
      "title": "Demo2",
      "version": "V1.0.0",
      "force_update": true,
      "ignore_folders": ["logs"],
      "ignore_files": ["*.log"],
      "created_at": "2026-11-10T10:59:35.6878741+08:00",
      "edges": {}
    }
  ]
}
```

Fields match the `Project` table. There is no pagination and no `ORDER BY`; the return order is not guaranteed. With no projects it returns `"data": []`.

### Common errors

Only database-level exceptions (`errorMsg` is the raw ent error); the normal path never fails.

## Get Project by Name

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/api/project/get_project_by_name/{projectName}` |
| Path parameter | `projectName`: project name, URL-encoded |

### Request example

```bash
curl http://127.0.0.1:2000/api/project/get_project_by_name/demo
```

### Response

`data` is a single `Project` object (fields in the table above).

### Behavior

- This query **filters on `is_deleted`**: a soft-deleted project now fails (error text `ent: project not found`), matching the availability rule used by `get_all_files` and `publish_version` (review M2).
- Clients no longer need to inspect `is_deleted` themselves: a failed lookup means the project does not exist.
- If the project genuinely does not exist, the call fails with ent's raw message.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `ent: project not found` | Name does not exist |

## Get Change Logs

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/api/project/get_project_change_logs/{projectName}` |
| Path parameter | `projectName`: project name, URL-encoded |

### Request example

```bash
curl http://127.0.0.1:2000/api/project/get_project_change_logs/demo
```

### Response

`data` is a `ProjectChangeLog` array, ordered by `id` descending (newest version first):

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 3,
      "project_id": 1,
      "version": "1.0.1",
      "logs": ["Fix A", "Fix B"],
      "time": "2026-11-10 11:00:00",
      "created_at": "2026-11-10T10:59:44.0995513+08:00",
      "after_apply_update_script": "run.bat",
      "edges": {}
    },
    {
      "id": 1,
      "project_id": 1,
      "version": "V1.0.0",
      "logs": ["第一次创建"],
      "time": "2026-11-10 10:59:35",
      "created_at": "2026-11-10T10:59:35.6153536+08:00",
      "edges": {}
    }
  ]
}
```

| Field | Type | Description |
| --- | --- | --- |
| `id` | int | Auto-increment primary key; also the sort key |
| `project_id` | int | Owning project ID |
| `version` | string | Version number of this release |
| `logs` | string[] | List of change descriptions |
| `time` | string | Time supplied by the publisher, `yyyy-MM-dd HH:mm:ss`; empty string when not supplied (the key is then omitted from the response) |
| `created_at` | string | Time written by the server, RFC 3339 |
| `after_apply_update_script` | string | Post-update script (relative to the application directory); omitted when empty |
| `edges` | object | Always `{}` |

### Behavior

- **Does not validate that the project exists**: an unknown project name returns success plus an empty array `[]`.
- After a project is soft-deleted, its logs remain queryable (as long as the logs themselves are not soft-deleted); looking the project up by name fails (see `get_project_by_name` above).
- Both client and server pick the latest by **version number** (if versions are equal, the larger `id` wins); the anti-rollback check when publishing also compares versions and ignores insertion order.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |

## Delete Project

Soft delete: sets `is_deleted = true` and deletes no files on disk.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/project/delete_project/{projectName}` |
| Path parameter | `projectName`: project name, URL-encoded |
| Request body | Not used (an empty object is acceptable) |

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/project/delete_project/demo
```

### Response

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### Behavior

- Only the database flag is changed; the files under `data/{projectName}` and `.updator_manifest.json` are all retained.
- The project's file-list cache is released before deletion.
- After deletion: `get_all_projects` no longer returns it; `update_project`, `set_force_update`, `publish_version`, and `get_project_os_info` all report "project does not exist"; `get_project_by_name` no longer returns it; changelogs remain queryable.
- A project with the same name cannot be recreated (`create_project`'s uniqueness check does not filter soft-deleted records); you must clean up the database record manually first.
- Deleting again returns `项目不存在: demo`.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `项目不存在: X` | Project does not exist or has been soft-deleted (affected row count is 0) |

## Publish Version

Registers a new version, writes a changelog entry, and freezes the project's release manifest.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/project/publish_version` |
| Content-Type | `application/json` |

### Request parameters

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `projectName` | string | Yes | Project name; must exist and not be soft-deleted |
| `version` | string | Yes | New version number; format and length below |
| `logs` | string[] | No | List of change descriptions; empty by default |
| `time` | string | No | Publish time, `yyyy-MM-dd HH:mm:ss`; not validated when empty |
| `afterApplyUpdateScript` | string | No | Script to run after the update, relative to the application directory; not written when empty |

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/project/publish_version \
  -H "Content-Type: application/json" \
  -d '{
        "projectName": "demo",
        "version": "1.0.2",
        "logs": ["Fix the login issue", "Speed up startup"],
        "time": "2026-11-10 11:30:00",
        "afterApplyUpdateScript": "run.bat"
      }'
```

### Response

`data` is the newly created `ProjectChangeLog`:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 4,
    "project_id": 1,
    "version": "1.0.2",
    "logs": ["Fix the login issue", "Speed up startup"],
    "time": "2026-11-10 11:30:00",
    "created_at": "2026-11-10T11:30:02.2201145+08:00",
    "after_apply_update_script": "run.bat",
    "edges": {}
  }
}
```

Field descriptions are the same as the changelog table above.

### Version number rules

| Constraint | Description |
| --- | --- |
| Non-empty | Missing or empty string → `版本号不能为空` |
| Length | At most 50 characters |
| Format | Regex `^[vV]?\d+(\.\d+)*(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$` |
| Examples | `1.0.0`, `2.1.0-beta`, `v1.2.3`, `2.1.2+build.4`, `2` (a single segment is valid) |
| Invalid examples | `1.0.0 beta` (contains a space), `2026.10.31 15:00`, `release-1` |
| Strictly increasing | Must be **greater than** the highest version among all non-deleted logs of the project; an equal value is also rejected |

Comparison semantics (`CompareVersion`, consistent with the client):

- Strip the `v` / `V` prefix;
- Compare dot-separated segments numerically, treating missing segments as `0` (`1.0` == `1.0.0`, `2` == `2.0.0`);
- Pre-release segments: `1.0.0-beta` < `1.0.0`; numeric segments sort below alphabetic segments (`1.0.0-1` < `1.0.0-alpha`); numeric segments compare numerically (`beta.2` < `beta.10`); more segments sort higher;
- Build metadata (after `+`) is ignored: `1.0.0+build.2` == `1.0.0`.

**The first publishable version must be greater than `V1.0.0`.** The initial version written when a project is created is `V1.0.0`; publishing `1.0.0` is rejected as "not greater than the latest version". Real publishing starts at `1.0.1` (or higher).

### Behavior

1. Validates the time format (when non-empty); a malformed value is rejected immediately.
2. Within the same transaction: fetch the project (not soft-deleted) → collect all non-deleted logs for the project and compute the highest version for the anti-rollback check → update `projects.version` → insert the new log.
3. After the transaction commits, write `data/{projectName}/.updator_manifest.json`: freeze the project's current files as an array of relative paths, using the same filtering rules as `get_all_files` (skip upload intermediates, apply the project's ignore rules, exclude the manifest itself).
4. A manifest write failure is only logged on the server and **does not affect the publish result**; in that case clients cannot get the manifest and skip deletion sync (degrading to not deleting files).

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `版本号不能为空` | `version` missing or empty |
| `版本号长度不能超过50个字符` | Too long |
| `版本号格式非法: X（应为数字版本号，如 1.0.0 / 2.1.0-beta / v1.2.3）` | Does not match the format regex |
| `时间格式错误，正确格式: yyyy-MM-dd HH:mm:ss` | `time` is non-empty but fails to parse |
| `ent: project not found` | Project does not exist or has been soft-deleted |
| `版本 X 不大于已发布的最新版本 Y（防倒退保护；如需回退内容，请发布更高的版本号）` | Version is not strictly increasing |

There is no "allow downgrade" switch. If content really must be rolled back, publish a higher version number as the server suggests.

## Get Project Server Info

Returns server system information; disk usage is measured for the partition containing the project directory.

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/api/project/get_project_os_info/{projectName}` |
| Path parameter | `projectName`: project name, URL-encoded |

### Request example

```bash
curl http://127.0.0.1:2000/api/project/get_project_os_info/demo
```

### Response

`data` is a `ServerOSInfo` array; the server always returns exactly one element:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "os": "linux",
      "platform": "Ubuntu 24.04.1 LTS",
      "goARCH": "amd64",
      "version": "go1.25.0",
      "numCPU": 8,
      "cpuName": "Intel(R) Xeon(R) Platinum 8269CY CPU @ 2.50GHz",
      "cpuMhz": 2500,
      "diskUsed": 452.12,
      "diskFree": 150.86,
      "diskTotal": 602.98,
      "diskUsedPercent": 74.98
    }
  ]
}
```

| Field | Type | Description |
| --- | --- | --- |
| `os` | string | `runtime.GOOS`, e.g. `linux` / `windows` / `darwin` |
| `platform` | string | Distribution description; empty string when platform information is unavailable |
| `goARCH` | string | `runtime.GOARCH` |
| `version` | string | Go runtime version, e.g. `go1.25.0` |
| `numCPU` | int | Number of logical CPUs |
| `cpuName` | string | CPU model; `unknown` when unavailable |
| `cpuMhz` | float64 | CPU clock speed (MHz); `0` when unavailable |
| `diskUsed` | float64 | Used space (GB) |
| `diskFree` | float64 | Free space (GB) |
| `diskTotal` | float64 | Total space (GB) |
| `diskUsedPercent` | float64 | Usage percentage (`0`–`100`) |

`os` / `goARCH` / `version` / `numCPU` come directly from the Go runtime and are always available; `platform`, `cpuName`, `cpuMhz`, and the four disk values depend on system calls and may be missing in containers or trimmed systems (an empty string, `unknown`, `0`, and `0` respectively).

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `项目不存在` | Project does not exist or has been soft-deleted |

## Related pages

- [API Overview](/en/docs/api): envelope, HTTP 200 error semantics, client call chain
- [File API](/en/docs/server/api-file): upload, file list, release manifest, download
- [Data Models](/en/docs/server/api-models): tables and fields for `Project` / `ProjectChangeLog`
- [Deploying aly-server](/en/docs/server/deploy): data directory, backups, upgrades