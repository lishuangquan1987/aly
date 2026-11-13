---
title: File API
description: "The 7 file endpoints of aly-server: whole-file upload, chunked upload and merge, file list (MD5/SHA256), release manifest, file deletion, download by absolute path, and Range resumption."
---

# File API

The file endpoints receive release artifacts and serve file lists and byte streams to the updater. They all live under `/api/file`; there are 7 in total.

All endpoints return the unified envelope (HTTP 200 + `isSuccess` / `errorMsg` / `data`), **with one exception: `download_file`**. It is a streaming download that uses real HTTP status codes and returns no JSON. For the envelope, see [API Overview](/en/docs/api).

## Before you upload

| Item | Value |
| --- | --- |
| Per-file limit | 1 GB (`1 << 30` bytes) |
| Per-chunk limit | 64 MB (`64 << 20` bytes) |
| Chunk numbering | Integer starting at `0`, `0 ≤ chunkIndex < totalChunks` |
| Publisher chunk size | aly-publish defaults to 128 KiB (override with the `ALY_CHUNK_SIZE` environment variable); files larger than the chunk size use chunks |
| Max chunk count | `totalChunks` must be within `1..65536` (inclusive) |
| Free disk before upload | The target disk must have at least "upload bytes + 512 MB" free, otherwise the upload is rejected |
| Project precondition | `projectName` must already exist (via `create_project`) and not be soft-deleted; otherwise the server returns "project does not exist" |
| Upload field | `multipart/form-data`; the file field name is always `file` |

The limit is applied to the entire request body (`Content-Length` pre-check + `MaxBytesReader` + the parsed multipart file size as a fallback), so **when the file itself approaches 1 GB, the multipart form overhead hits the limit first**; the effective usable file size is slightly less than 1 GB.

The server's two-phase write to disk:

- Whole-file upload: writes `{target path}.merging` first, then applies `rename` to move it to the final path once the write completes;
- Chunked upload: chunks are stored under `{target path}.chunks/{index}`, and `upload_chunk_complete` merges them into `{target path}.merging`, then applies `rename`.

`.chunks` and `.merging` are filtered consistently by `get_all_files`, the release manifest, and `download_file`, so half-written files are never served to clients while an upload is in progress. Every successful upload invalidates the project's file-list cache, and the next `get_all_files` recomputes the hashes.

Before accepting an upload the server checks that the database project for `projectName` exists and is not soft-deleted (the same rule as `get_all_files`); otherwise it returns "project does not exist: X (create the project first)". The project data directory is no longer created implicitly by an upload. The normal flow is to call `create_project` first (see [Project API](/en/docs/server/api-project#create-project)).

`relativeFileName` may use `/` or `\`; the server normalizes it to `/`. The path is normalized and must stay inside the project directory; out-of-bounds paths containing `..` return "invalid file path" (`非法的文件路径`).

## Upload File

Uploads a small file in one request.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/file/upload_file` |
| Content-Type | `multipart/form-data` |

### Request fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `projectName` | string | Yes | Project name |
| `relativeFileName` | string | Yes | Path relative to the project directory, e.g. `sub/app.exe` |
| `file` | file | Yes | File content, ≤ 1 GB |

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/file/upload_file \
  -F "projectName=demo" \
  -F "relativeFileName=sub/small.txt" \
  -F "file=@./small.txt"
```

### Response

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `文件过大: N 字节，上限 1073741824 字节` | `Content-Length` or the file size exceeds 1 GB |
| `文件过大: 上限 1073741824 字节` | Truncated by `MaxBytesReader` during multipart parsing |
| `项目名称不能为空` | `projectName` missing |
| `header中必须包含FileName` | `relativeFileName` missing (legacy error text) |
| `非法的文件路径` | Path escapes the project directory after normalization |
| `create directory error:...` | Target subdirectory creation failed (permissions/disk) |
| `save upload file error:...` | Writing the temp file failed |
| `atomic rename upload file error:...` | Rename failed (target in use/permissions) |

## Upload Chunk

Uploads a large file in chunks, each ≤ 64 MB.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/file/upload_chunk` |
| Content-Type | `multipart/form-data` |

### Request fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `projectName` | string | Yes | Project name |
| `relativeFileName` | string | Yes | Relative path of the final file; must be identical for all chunks |
| `chunkIndex` | int | Yes | Current chunk index, starting at 0 |
| `totalChunks` | int | Yes | Total number of chunks; must be identical for all chunks |
| `file` | file | Yes | Chunk content, ≤ 64 MB |

### Request example

```bash
# Chunk 0 of 2
curl -X POST http://127.0.0.1:2000/api/file/upload_chunk \
  -F "projectName=demo" \
  -F "relativeFileName=big/file.bin" \
  -F "chunkIndex=0" \
  -F "totalChunks=2" \
  -F "file=@./chunk0.bin"
```

### Response

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `分片过大: N 字节，上限 67108864 字节` | Chunk exceeds 64 MB |
| `分片过大: 上限 67108864 字节` | Truncated during multipart parsing |
| `项目名称不能为空` | `projectName` missing |
| `文件路径不能为空` | `relativeFileName` missing |
| `分片序号不合法: chunkIndex=X, totalChunks=Y` | `chunkIndex < 0` or `≥ totalChunks` |
| `非法的文件路径` | Path escapes the project directory |
| `create chunks dir error: ...` | Chunk staging directory creation failed |
| `save chunk error: ...` | Writing the chunk to disk failed |

Chunks themselves are not retried server-side; retries are the client's responsibility (aly-publish tries each chunk up to 5 times, and only retries connection-drop/timeout-class errors).

## Complete Chunked Upload

Merges all uploaded chunks, verifies integrity, and atomically replaces the target file.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/file/upload_chunk_complete` |
| Content-Type | `multipart/form-data` |

### Request fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `projectName` | string | Yes | Project name |
| `relativeFileName` | string | Yes | Must match the value used for the chunk uploads |
| `totalChunks` | int | Yes | Total number of chunks; must match the value used during upload |
| `md5` | string | No | MD5 of the whole file (lowercase hex, 32 chars). When provided, the server verifies the merged result: on mismatch it keeps the chunks, deletes the merge temp file and reports an error |

No `file` field is sent.

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/file/upload_chunk_complete \
  -F "projectName=demo" \
  -F "relativeFileName=big/file.bin" \
  -F "totalChunks=2"
```

### Response

Success:

```json
{ "isSuccess": true, "errorMsg": "", "data": { "chunksComplete": true, "totalSize": 1500, "md5": "2e3587c7179025e801964d9babf8c70e" } }
```

Chunks missing (**still a success response**; the caller must read `chunksComplete`):

```json
{ "isSuccess": true, "errorMsg": "", "data": { "chunksComplete": false, "missingChunks": [2] } }
```

| Field | Type | Present when | Description |
| --- | --- | --- | --- |
| `chunksComplete` | bool | Always | Whether all chunks are present and have been merged |
| `totalSize` | int64 | `chunksComplete = true` | Size of the merged file in bytes |
| `missingChunks` | int[] | `chunksComplete = false` | Missing chunk indices (ascending) |
| `md5` | string | `chunksComplete = true` | MD5 of the merged result (lowercase hex; returned whether or not the request supplied `md5`) |

### Behavior

- `totalChunks` must be within `1..65536` (inclusive); a zero-byte target is rejected (no empty file is ever produced).
- Before merging, the sum of chunk sizes must be ≤ 1 GB and the target disk must have at least that sum + 512 MB free.
- The server `stat`s chunks `0 … totalChunks-1`; when some are missing it returns `missingChunks` without merging or cleaning up, so the client can upload them and retry.
- When `md5` is provided the server hashes while merging and compares before the atomic rename; on mismatch it returns the server message `合并结果 MD5 不符: 期望 X，实际 Y（分片已保留，可重试）`, deletes the `.merging` temp file and **keeps the chunks**.
- `complete` calls for the same target file are serialized by a per-file lock, so two publishers merging concurrently cannot wipe each other's chunks.
- The server merges into `{target}.merging`, renames it to the final path on success and then deletes the `.chunks` directory; any failure cleans up the temporaries.
- The write is an overwrite: uploading the same name again replaces the old content and changes the hashes reported by `get_all_files`.
- On startup the server sweeps orphan `.chunks` / `.merging` entries older than 24 hours.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称不能为空` | `projectName` missing |
| `文件路径不能为空` | `relativeFileName` missing |
| `非法的文件路径` | Path escapes the project directory |
| `未找到分片目录，请先上传分片` | `.chunks` does not exist (nothing uploaded, or called again after a completed merge) |
| `文件过大: N 字节，上限 1073741824 字节` | Total chunk size exceeds 1 GB |
| `create dest dir error: ...` | Target directory creation failed |
| `create dest file error: ...` | Temporary file creation failed |
| `open chunk N error: ...` / `copy chunk N error: ...` | Reading or writing a chunk failed |
| `close merged file error: ...` / `atomic rename merged file error: ...` | Closing or renaming failed |
| `totalChunks 不合法: N（允许范围 1-65536）` | `totalChunks` out of range or ≤ 0 |
| `分片内容为空，拒绝合并（避免把目标文件清空）` | All chunks add up to 0 bytes |
| `合并结果 MD5 不符: 期望 X，实际 Y（分片已保留，可重试）` | The provided `md5` does not match the merged result |
| `磁盘空间不足: ...` | Not enough free space on the target disk (including the 512 MB reserve) |
| `项目不存在: X（请先创建项目）` | The project was never created or is soft-deleted |

## Get File List

Returns all files under the project plus both hashes; this is what the client uses to decide "which files to download".

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/api/file/get_all_files/{projectName}` |
| Path parameter | `projectName`: project name, URL-encoded |

### Request example

```bash
curl http://127.0.0.1:2000/api/file/get_all_files/demo
```

### Response

`data` is a `FileInfo` array, sorted ascending by `fileRelativePath`:

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "fileAbsolutePath": "C:\\aly\\data\\demo\\big\\file.bin",
      "fileRelativePath": "big/file.bin",
      "lastUpdateTime": "2026-11-10T11:00:09.4428015+08:00",
      "fileSize": 1500,
      "md5": "2e3587c7179025e801964d9babf8c70e",
      "sha256": "e83835331f9b903cb728137f30b1bb1d83d3751ed561801e79555cfaca4eeb45"
    }
  ]
}
```

| Field | Type | Description |
| --- | --- | --- |
| `fileAbsolutePath` | string | Absolute path on the server; pass it verbatim as `path` when downloading |
| `fileRelativePath` | string | Path relative to the project directory, `/`-separated |
| `lastUpdateTime` | string | File mtime, RFC 3339 |
| `fileSize` | int64 | Size in bytes |
| `md5` | string | MD5, lowercase hexadecimal |
| `sha256` | string | SHA-256, lowercase hexadecimal |

### Filtering and caching

The list excludes:

- Upload intermediates: any directory segment ending in `.chunks`, and any file ending in `.merging`;
- The release manifest itself, `.updator_manifest.json`;
- Files matching the project's `ignoreFolders` / `ignoreFiles` (ignore files support exact match, glob on the full path, glob on the file name, and `*.ext` suffix).

The server caches this list per project (hashes included). On a cache hit it performs only one `stat` fingerprint comparison (relative path + size + mtime); a fingerprint change (for example, copying files directly into `data/`) or an invalidation triggered by an upload endpoint rebuilds it. The response contains no cache marker.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `ent: project not found` | Project does not exist (the same error applies after soft deletion) |

If the project exists but its directory is empty, it returns `"data": []`.

## Get Release Manifest

Returns the set of relative file paths frozen at the most recent successful publish, **used only for deletion sync**.

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/api/file/get_release_manifest/{projectName}` |
| Path parameter | `projectName`: project name, URL-encoded |

### Request example

```bash
curl http://127.0.0.1:2000/api/file/get_release_manifest/demo
```

### Response

```json
{ "isSuccess": true, "errorMsg": "", "data": { "files": ["big/file.bin", "sub/small.txt"] } }
```

| Field | Type | Description |
| --- | --- | --- |
| `files` | string[] | Set of file paths relative to the project at publish time, `/`-separated |

**Only the `files` array**: no sizes, no hashes, no version number. When you need hashes or absolute paths, use `get_all_files`.

### Behavior

- The manifest file is `data/{projectName}/.updator_manifest.json`, written outside the transaction after `publish_version` succeeds, using an atomic "temp file + rename" write.
- The manifest contents are filtered by the same rules as `get_all_files` (intermediates, ignore rules, and the manifest itself are all excluded from the manifest).
- The client reads it at the end of `download_update`: files outside the manifest are treated as "deleted on the publishing side" and cleaned up locally; when the server has never published, `files` is empty and the client skips deletion.
- A project that has never published returns **success + an empty array** (the client skips deletion). When the project **does not exist or is soft-deleted**, the endpoint now returns an error ("project does not exist") instead of an empty manifest (review M2/N2).
- The manifest is updated only on publish. If files are copied directly into `data/` after a publish, `get_all_files` will see the new files, but the manifest will not change, and clients will not proactively delete them.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `unexpected end of JSON input` and similar | The manifest file was corrupted externally and cannot be parsed |

## Delete File

Deletes a single file inside a project so the publisher can mirror "deleted locally → deleted on the server". The endpoint is **idempotent**: deleting a file that does not exist still succeeds.

| Item | Value |
| --- | --- |
| Method | `POST` |
| Path | `/api/file/delete_file` |
| Content-Type | `application/json` or `multipart/form-data` |

### Request fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `projectName` | string | Yes | Project name; must exist and not be soft-deleted |
| `relativeFileName` | string | Yes | Path relative to the project directory; same rules as upload (`/` or `\`, traversal rejected) |

### Request example

```bash
curl -X POST http://127.0.0.1:2000/api/file/delete_file \
  -H "Content-Type: application/json" \
  -d '{"projectName":"demo","relativeFileName":"old/legacy.dll"}'
```

### Response

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### Behavior

- The same landing-point assertion as upload runs first: the path must be strictly inside the project directory and must not be the project root itself.
- A leftover `{target}.chunks` directory for that file is removed, and the project's file-list cache is invalidated.
- The server does **not** write the release manifest here; the manifest is still frozen by `publish_version` from what is actually on disk. The normal flow is `delete_file` → `publish_version`, after which clients remove their stale local copies on the next `download_update`.
- Missing files or directories count as success; only a missing project, an illegal path or an I/O failure returns an error.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `project name must not be empty` / `file path must not be empty` | Missing field |
| `illegal file path` | Path escapes the project directory, or points at the project root |
| `project does not exist: X (create the project first)` | Project not created or soft-deleted |
| `remove ...` and other I/O errors | Insufficient permissions, file locked, and so on |

## Download File

Downloads a file by its absolute server path; supports `Range` resumable downloads.

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/api/file/download_file` |
| Query parameter | `path`: absolute path under `data/`, URL-encoded |
| Response | File byte stream, **not** a JSON envelope |

### Request parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `path` | string | Yes | Must be the `fileAbsolutePath` returned by `get_all_files`; an absolute path under `data/` |

`path` is strictly validated:

- A relative path (e.g. `data/demo/app.exe`) → 404;
- A path outside `data/` (e.g. `/etc/passwd`, `C:\Windows\win.ini`) → 404;
- An upload intermediate (`.chunks` / `.merging`) → 404;
- File does not exist → 404;
- Other I/O errors → 500.

### Request example

```bash
# Use --data-urlencode to handle spaces and non-ASCII characters in the path
curl -G -o app.exe \
  --data-urlencode "path=/opt/aly/data/demo/app.exe" \
  http://127.0.0.1:2000/api/file/download_file

# Resumption: curl automatically sends Range: bytes=<bytes already downloaded>-
curl -C - -o app.exe \
  --data-urlencode "path=/opt/aly/data/demo/app.exe" \
  http://127.0.0.1:2000/api/file/download_file
```

### Response

Success returns `200 OK`. Example response headers:

```http
Content-Disposition: attachment; filename=small.txt
Accept-Ranges: bytes
Content-Length: 25
```

With `Range` it returns `206 Partial Content` and `Content-Range: bytes 0-9/25`; a multipart `Range` (for example `bytes=0-1,4-5`) returns `206` plus `multipart/byteranges` (the server explicitly restores multi-range support that Go 1.26 disabled by default, capped at 200 ranges); an out-of-range request returns `416`.

### Status codes

| Status code | Meaning |
| --- | --- |
| `200` | Complete file |
| `206` | Partial content matched by `Range` |
| `404` | Invalid path, out-of-bounds path, or missing file (empty response body) |
| `416` | `Range` cannot overlap the file (with `Content-Range: bytes */<size>`; the body is a one-line error text) |
| `500` | Other errors while opening or reading the file (empty response body) |

404 / 500 abort the request directly on the server with no error description; 416 is generated by `http.ServeContent` and carries a one-line `invalid range` text. This is the server's **only** endpoint that expresses failure with real status codes.

### Behavior

- Implemented with `http.ServeContent`, so it supports single-part `Range`, multipart `Range` (`multipart/byteranges`, capped at 200 ranges) and `If-Range`;
- `Content-Disposition` is produced by `mime.FormatMediaType` and follows RFC 6266/5987 (non-ASCII names go through the `filename*` percent encoding; quotes and newlines are escaped);
- The server does not pre-check file existence; it opens the file and then inspects the error, avoiding TOCTOU;
- The global `WriteTimeout` is 0 (large downloads must not have a fixed total limit); each download gets a write deadline based on file size — `5 minutes + 1 second per MiB`, capped at 30 minutes — so slow readers cannot hold a connection forever. aly-client enables `.part` + `Range` resumption for files over 100 MB.

## Related pages

- [API Overview](/en/docs/api): envelope, error semantics, client call chain
- [Project API](/en/docs/server/api-project): project creation and version publishing
- [Data Models](/en/docs/server/api-models): `FileInfo` and release manifest fields
- [Deploying aly-server](/en/docs/server/deploy): data directory, proxy upload limits, download timeouts
- [aly-publish publishing tool](/en/docs/publish): the side that calls the upload endpoints