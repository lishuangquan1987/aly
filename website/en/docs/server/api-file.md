---
title: File API
description: "The 6 file endpoints of aly-server: whole-file upload, chunked upload and merge, file list (MD5/SHA256), release manifest, download by absolute path, and Range resumption."
---

# File API

The file endpoints receive release artifacts and serve file lists and byte streams to the updater. They all live under `/api/file`; there are 6 in total.

All endpoints return the unified envelope (HTTP 200 + `isSuccess` / `errorMsg` / `data`), **with one exception: `download_file`**. It is a streaming download that uses real HTTP status codes and returns no JSON. For the envelope, see [API Overview](/en/docs/api).

## Before you upload

| Item | Value |
| --- | --- |
| Per-file limit | 1 GB (`1 << 30` bytes) |
| Per-chunk limit | 64 MB (`64 << 20` bytes) |
| Chunk numbering | Integer starting at `0`, `0 ≤ chunkIndex < totalChunks` |
| Publisher chunk size | aly-publish uses a fixed 1 MiB; files over 1 MiB automatically use chunks |
| Upload field | `multipart/form-data`; the file field name is always `file` |

The limit is applied to the entire request body (`Content-Length` pre-check + `MaxBytesReader` + the parsed multipart file size as a fallback), so **when the file itself approaches 1 GB, the multipart form overhead hits the limit first**; the effective usable file size is slightly less than 1 GB.

The server's two-phase write to disk:

- Whole-file upload: writes `{target path}.merging` first, then applies `rename` to move it to the final path once the write completes;
- Chunked upload: chunks are stored under `{target path}.chunks/{index}`, and `upload_chunk_complete` merges them into `{target path}.merging`, then applies `rename`.

`.chunks` and `.merging` are filtered consistently by `get_all_files`, the release manifest, and `download_file`, so half-written files are never served to clients while an upload is in progress. Every successful upload invalidates the project's file-list cache, and the next `get_all_files` recomputes the hashes.

If the directory for `projectName` does not exist it is created automatically, but the database project record is **not** created automatically: uploading to a nonexistent project succeeds, and a subsequent `get_all_files` reports `ent: project not found`. The normal flow is to call `create_project` first (see [Project API](/en/docs/server/api-project#create-project)).

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
{ "isSuccess": true, "errorMsg": "", "data": { "chunksComplete": true, "totalSize": 1500 } }
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

### Behavior

- `stat`s each chunk from `0 … totalChunks-1`; if any are missing it returns `missingChunks` without merging or cleaning up, and the client can retry after filling the gaps.
- If the sum of all chunk sizes exceeds 1 GB it returns "file too large" and does not merge.
- Merges into `{target path}.merging`; on success it applies `rename` to move the file to the final path and then deletes the `.chunks` directory. A failure at any step cleans up the temporary files.
- The target is overwritten: uploading a file with the same name again replaces the old content, and the hashes from `get_all_files` change accordingly.

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
- When the project has never published, the project does not exist, or the project directory does not exist, it returns **success + an empty array**, not an error. This behavior exists to remain compatible with old servers/old projects.
- The manifest is updated only on publish. If files are copied directly into `data/` after a publish, `get_all_files` will see the new files, but the manifest will not change, and clients will not proactively delete them.

### Common errors

| `errorMsg` | Trigger condition |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | Invalid name |
| `unexpected end of JSON input` and similar | The manifest file was corrupted externally and cannot be parsed |

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
Content-Disposition: attachment; filename*=UTF-8''small.txt; filename=small.txt
Content-Transfer-Encoding: binary
Accept-Ranges: bytes
Content-Length: 25
```

With `Range` it returns `206 Partial Content` and `Content-Range: bytes 0-9/25`; an out-of-range request returns `416`.

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

- Implemented with `http.ServeContent`, so it naturally supports single-part `Range`, multipart `Range`, and `If-Range`;
- `Content-Disposition` provides both the RFC 6266 `filename*` (UTF-8, spaces as `%20`) and a fallback `filename`;
- The server does not pre-check file existence; it opens the file and then inspects the error, avoiding TOCTOU;
- The server's `WriteTimeout` is 60 seconds (the whole response); very large files on slow links may be cut off midway, and clients should retry with resumption. aly-client enables `.part` + `Range` resumption for files over 100 MB.

## Related pages

- [API Overview](/en/docs/api): envelope, error semantics, client call chain
- [Project API](/en/docs/server/api-project): project creation and version publishing
- [Data Models](/en/docs/server/api-models): `FileInfo` and release manifest fields
- [Deploying aly-server](/en/docs/server/deploy): data directory, proxy upload limits, download timeouts
- [aly-publish publishing tool](/en/docs/publish): the side that calls the upload endpoints