---
title: Server API Overview
description: aly-server request conventions, the unified response envelope, HTTP 200 error semantics, the unauthenticated trust boundary, and the endpoint index grouped by project, file, and server.
---

# Server API Overview

aly-server exposes HTTP endpoints through Gin, all registered under the `/api` prefix (see `server/routers/routers.go`).

If you just need an endpoint, jump to the [Endpoint Index](#endpoint-index); the shared conventions follow below.

> **Who this is for**: developers implementing a custom publisher/updater or debugging the HTTP layer. After this page you can call all 17 endpoints and handle HTTP 200 + `isSuccess:false` correctly.

::: tip Output language
The server currently returns `errorMsg`, changelogs and some sample values in Chinese. JSON examples preserve that real output; endpoint paths, field names and the envelope are the stable contract.
:::

| Item | Value |
| --- | --- |
| Base URL | `http://<host>:2000/api` |
| Listen address | `0.0.0.0:2000`; the port is set by `-p`, default 2000 |
| Authentication | None |
| Request body | JSON or `multipart/form-data` |
| Response body | JSON; `download_file` returns a raw file byte stream |
| Charset | UTF-8 |

For details on the port, data directory, and process management, see [Deploying aly-server](/en/docs/server/deploy).

## Unified response envelope

Except for file downloads, every endpoint returns HTTP 200 with a fixed three-field body:

| Field | Type | Description |
| --- | --- | --- |
| `isSuccess` | bool | Whether the business operation succeeded |
| `errorMsg` | string | Failure reason; `""` on success |
| `data` | any | Business payload; `null` on failure; also `null` for a success that returns no value |

Success (`get_project_by_name`):

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 1,
    "name": "demo",
    "title": "Demo Project",
    "version": "V1.0.0",
    "created_at": "2026-11-10T10:59:35.6148004+08:00",
    "edges": {}
  }
}
```

Failure (creating a project with a duplicate name):

```json
{ "isSuccess": false, "errorMsg": "项目名称:demo已存在", "data": null }
```

This envelope maps to `models.CommonResponse`; both the client SDK and aly-publish use `isSuccess` to decide success or failure.

## Error semantics: HTTP 200 + `isSuccess:false`

The server does not express business errors with 4xx/5xx. Parameter validation failures, missing projects, version rollbacks, disk write failures, and so on all return **HTTP 200**, with the reason in `errorMsg`. Therefore:

- Checking only the HTTP status code treats every failure as success;
- You must parse the response body and read `errorMsg` when `isSuccess == false`.

Only two kinds of responses do not use this envelope:

| Scenario | Status code | Response body |
| --- | --- | --- |
| `GET /api/file/download_file` | 200 / 206 / 404 / 416 / 500 | Raw file byte stream, or an empty body |
| Route not found, method mismatch | 404 | Gin's default plain-text `404 page not found` |

`download_file` is a streaming endpoint and cannot write an error envelope after the transfer has begun, so it uses real status codes; callers must handle it separately.

## Request format conventions

| Scenario | Encoding | Field naming |
| --- | --- | --- |
| Project-management POSTs | `application/json` | camelCase, e.g. `isForceUpdate`, `ignoreFolders` |
| File-upload POSTs | `multipart/form-data` | Form fields in camelCase, e.g. `projectName`, `relativeFileName` |
| Project / changelog responses | `application/json` | snake_case; ent entities are serialized directly |
| File-list / system-info responses | `application/json` | camelCase; `server/models` structs are serialized |

The mismatch between request and response field naming is **intentional**: request DTOs are hand-written in the controllers, while responses reuse ent-generated entities. For a field-by-field comparison, see [Data Models](/en/docs/server/api-models).

## Trust boundary: no authentication

The server has no token, signature, IP allowlist, or rate limiting. Any client that can reach the port can create/delete projects, upload files, publish versions, and download all data. The design assumes **deployment on a trusted intranet**:

- Do not expose the port directly to the public internet;
- The upload endpoints accept files up to 1 GB, so exposing the port is equivalent to opening disk writes;
- Existing clients send no authentication headers; adding Basic Auth at a reverse proxy would make every official client receive 401. For cross-network access, use a firewall allowlist or a VPN.

For hardening and reverse-proxy configuration, see [Deploying aly-server](/en/docs/server/deploy).

## Endpoint index

### Project `/api/project`

| Method | Path | Description | Successful `data` |
| --- | --- | --- | --- |
| POST | [`/api/project/create_project`](/en/docs/server/api-project#create-project) | Creates a project and writes the first changelog entry | `Project` |
| POST | [`/api/project/update_project`](/en/docs/server/api-project#update-project) | Updates the title, force-update flag, and ignore rules | `null` |
| POST | [`/api/project/set_force_update`](/en/docs/server/api-project#set-force-update) | Changes only the force-update flag | `null` |
| GET | [`/api/project/get_all_projects`](/en/docs/server/api-project#get-all-projects) | Lists non-deleted projects | `Project[]` |
| GET | [`/api/project/get_project_by_name/:projectName`](/en/docs/server/api-project#get-project-by-name) | Looks up a single project by name | `Project` |
| GET | [`/api/project/get_project_change_logs/:projectName`](/en/docs/server/api-project#get-change-logs) | Changelog entries, in descending ID order | `ProjectChangeLog[]` |
| POST | [`/api/project/delete_project/:projectName`](/en/docs/server/api-project#delete-project) | Soft-deletes a project | `null` |
| POST | [`/api/project/publish_version`](/en/docs/server/api-project#publish-version) | Publishes a new version and freezes the release manifest | `ProjectChangeLog` |
| GET | [`/api/project/get_project_os_info/:projectName`](/en/docs/server/api-project#get-project-server-info) | Server system info (locates the disk by project) | `ServerOSInfo[]` |

### Files `/api/file`

| Method | Path | Description | Successful `data` |
| --- | --- | --- | --- |
| POST | [`/api/file/upload_file`](/en/docs/server/api-file#upload-file) | Whole-file upload; 1 GB per-file limit | `null` |
| POST | [`/api/file/upload_chunk`](/en/docs/server/api-file#upload-chunk) | Uploads a single chunk; 64 MB per-chunk limit | `null` |
| POST | [`/api/file/upload_chunk_complete`](/en/docs/server/api-file#complete-chunked-upload) | Verifies and merges chunks | `{chunksComplete,totalSize}` or `{chunksComplete,missingChunks}` |
| GET | [`/api/file/get_all_files/:projectName`](/en/docs/server/api-file#get-file-list) | File list including MD5 / SHA256 | `FileInfo[]` |
| GET | [`/api/file/get_release_manifest/:projectName`](/en/docs/server/api-file#get-release-manifest) | Release manifest (relative paths only) | `{files:string[]}` |
| POST | [`/api/file/delete_file`](/en/docs/server/api-file#delete-file) | Deletes a file inside a project (idempotent, used for deletion sync) | `null` |
| GET | [`/api/file/download_file`](/en/docs/server/api-file#download-file) | Downloads by absolute path; supports Range | File byte stream |

### Server `/api/server`

| Method | Path | Description | Successful `data` |
| --- | --- | --- | --- |
| GET | `/api/server/info` | Server system info, no project name required | `ServerOSInfo[]` |

`ServerOSInfo` is a single-element array (the server always returns `[{...}]`), with disk capacities in GB.

## Client call chain in practice

The updater aly-client uses only a subset of the table above, with a fixed call order for its two commands:

| Command | Call order |
| --- | --- |
| `check_update` | `get_project_by_name` → `get_project_change_logs` |
| `download_update` | `get_project_change_logs` → `get_all_files` → `download_file` × N → `get_release_manifest` |

What each step does:

1. **`get_project_by_name`**: confirms the project exists (the client checks `is_deleted`) and obtains the project's current version.
2. **`get_project_change_logs`**: fetches the latest server version and changelog, compares them with the local version to decide `has_update` / `need_download_update`, and also retrieves `after_apply_update_script`.
3. **`get_all_files`**: fetches the file list and both hashes, deciding which files to download and which unchanged files must be materialized locally.
4. **`download_file`**: downloads changed files one by one, passing the `fileAbsolutePath` returned by the previous step directly.
5. **`get_release_manifest`**: **used only for deletion sync**. After downloading, files outside the manifest are considered "deleted on the publishing side" and are cleaned up locally; if the server has never published, it returns an empty `files` and the client skips deletion; if the project does not exist or is soft-deleted, the endpoint fails.

The publishing side (aly-publish `push`) uses `upload_file` (≤ 128 KiB, adjustable via `ALY_CHUNK_SIZE`) or `upload_chunk` + `upload_chunk_complete` (> 128 KiB), then calls `publish_version` to register the version; staged `deleted` entries make `push` call `file/delete_file` first.

`get_all_projects`, `set_force_update`, `get_project_os_info`, and `server/info` are used by aly-publish and the publish GUI; they are not on the client's main update path. For command-level details, see [aly-client command reference](/en/docs/commands) and [aly-publish](/en/docs/publish).

### curl quick verification

```bash
# List projects (no request headers needed)
curl http://127.0.0.1:2000/api/project/get_all_projects

# Create a project
curl -X POST http://127.0.0.1:2000/api/project/create_project \
  -H "Content-Type: application/json" \
  -d '{"name":"demo","title":"Demo Project"}'

# Fetch the file list (grab fileAbsolutePath for downloads)
curl http://127.0.0.1:2000/api/file/get_all_files/demo
```

## Data directory

All project content lives in `data/` under **the directory containing the executable**:

```text
/opt/aly/
├── aly-server              # binary
├── aly.db                  # SQLite database (path can be changed with -db)
└── data/
    └── demo/               # project name is the directory name
        ├── app.exe
        ├── lib/
        │   └── core.dll
        └── .updator_manifest.json
```

This imposes two API-level constraints:

- The `fileAbsolutePath` returned by `get_all_files` is an **absolute path on the server**; the `path` parameter of `download_file` must be passed back verbatim, and it must fall under `data/`. Relative paths or paths outside `data/` always return 404.
- Absolute paths depend on the binary's location. After moving the binary or the data directory, stale `fileAbsolutePath` values cached by clients become invalid; clients should re-fetch `get_all_files`.

For the database and backups, see [Deploying aly-server](/en/docs/server/deploy).

## Related pages

- [Deploying aly-server](/en/docs/server/deploy): single binary, `-p` / `-db`, data directory, systemd, backups
- [Project API](/en/docs/server/api-project): complete parameters and errors for the 9 project endpoints
- [File API](/en/docs/server/api-file): upload, chunking, manifest, and download
- [Data Models](/en/docs/server/api-models): JSON field names and naming-style comparison
- [aly-client command reference](/en/docs/commands): CLI entry points that call these endpoints