---
title: 文件接口
description: aly-server 的 6 个文件接口：整文件上传、分片上传与合并、文件列表（MD5/SHA256）、发布清单、按绝对路径下载与 Range 续传。
---

# 文件接口

文件接口负责接收发布产物、向更新器提供文件清单与字节流，全部挂在 `/api/file` 下，共 6 个。

所有接口返回统一信封（HTTP 200 + `isSuccess` / `errorMsg` / `data`），**唯一例外是 `download_file`**：它是流式下载，用真实 HTTP 状态码，不返回 JSON。信封说明见 [API 总览](/docs/api)。

## 上传前须知

| 项 | 值 |
| --- | --- |
| 单文件上限 | 1 GB（`1 << 30` 字节） |
| 单分片上限 | 64 MB（`64 << 20` 字节） |
| 分片编号 | 从 `0` 开始的整数，`0 ≤ chunkIndex < totalChunks` |
| 发布端分片大小 | aly-publish 固定 1 MiB，超过 1 MiB 的文件自动走分片 |
| 上传字段 | `multipart/form-data`，文件字段名固定为 `file` |

上限施加在整个请求体上（`Content-Length` 预检 + `MaxBytesReader` + multipart 解析后的文件大小兜底），因此**文件本身接近 1 GB 时，multipart 的表单开销会先触发上限**，实际可用文件大小略小于 1 GB。

服务端的两阶段落盘：

- 整文件上传：先写 `{目标路径}.merging`，写完 `rename` 到最终路径；
- 分片上传：分片先存到 `{目标路径}.chunks/{index}`，`upload_chunk_complete` 合并到 `{目标路径}.merging` 后 `rename`。

`.chunks` 与 `.merging` 被 `get_all_files`、发布清单、`download_file` 一致过滤，因此上传进行中不会把半截文件下发给客户端。每次上传成功后都会使该项目的文件列表缓存失效，下一次 `get_all_files` 重新计算哈希。

`projectName` 对应的目录不存在时会自动创建，但**不会**自动创建数据库项目记录：向不存在的项目上传会成功，随后 `get_all_files` 报 `ent: project not found`。正常流程应先 `create_project`（见 [项目接口](/docs/server/api-project#创建项目)）。

`relativeFileName` 用 `/` 或 `\` 都可以，服务端统一转成 `/`。路径会被规范化并校验必须落在项目目录内，含 `..` 的越界路径返回「非法的文件路径」。

## 直传文件

小文件一次性上传。

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/file/upload_file` |
| Content-Type | `multipart/form-data` |

### 请求字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `projectName` | string | 是 | 项目名称 |
| `relativeFileName` | string | 是 | 相对项目目录的路径，如 `sub/app.exe` |
| `file` | file | 是 | 文件内容，≤ 1 GB |

### 请求示例

```bash
curl -X POST http://127.0.0.1:2000/api/file/upload_file \
  -F "projectName=demo" \
  -F "relativeFileName=sub/small.txt" \
  -F "file=@./small.txt"
```

### 响应

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `文件过大: N 字节，上限 1073741824 字节` | `Content-Length` 或文件大小超 1 GB |
| `文件过大: 上限 1073741824 字节` | multipart 解析时被 `MaxBytesReader` 截断 |
| `项目名称不能为空` | `projectName` 缺失 |
| `header中必须包含FileName` | `relativeFileName` 缺失（沿用的历史错误文案） |
| `非法的文件路径` | 路径规范化后逃出项目目录 |
| `create directory error:...` | 目标子目录创建失败（权限/磁盘） |
| `save upload file error:...` | 写临时文件失败 |
| `atomic rename upload file error:...` | 重命名失败（目标被占用/权限） |

## 上传分片

大文件按片上传，单片 ≤ 64 MB。

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/file/upload_chunk` |
| Content-Type | `multipart/form-data` |

### 请求字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `projectName` | string | 是 | 项目名称 |
| `relativeFileName` | string | 是 | 最终文件的相对路径，所有分片必须一致 |
| `chunkIndex` | int | 是 | 当前分片序号，从 0 开始 |
| `totalChunks` | int | 是 | 分片总数，所有分片必须一致 |
| `file` | file | 是 | 分片内容，≤ 64 MB |

### 请求示例

```bash
# 第 0 片，共 2 片
curl -X POST http://127.0.0.1:2000/api/file/upload_chunk \
  -F "projectName=demo" \
  -F "relativeFileName=big/file.bin" \
  -F "chunkIndex=0" \
  -F "totalChunks=2" \
  -F "file=@./chunk0.bin"
```

### 响应

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `分片过大: N 字节，上限 67108864 字节` | 分片超 64 MB |
| `分片过大: 上限 67108864 字节` | multipart 解析时被截断 |
| `项目名称不能为空` | `projectName` 缺失 |
| `文件路径不能为空` | `relativeFileName` 缺失 |
| `分片序号不合法: chunkIndex=X, totalChunks=Y` | `chunkIndex < 0` 或 `≥ totalChunks` |
| `非法的文件路径` | 路径逃出项目目录 |
| `create chunks dir error: ...` | 分片暂存目录创建失败 |
| `save chunk error: ...` | 分片写盘失败 |

分片本身不做服务端重试，重试由客户端负责（aly-publish 对每个分片最多尝试 5 次，仅对断链/超时类错误重试）。

## 完成分片上传

合并已上传的全部分片，校验完整性后原子替换目标文件。

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/file/upload_chunk_complete` |
| Content-Type | `multipart/form-data` |

### 请求字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `projectName` | string | 是 | 项目名称 |
| `relativeFileName` | string | 是 | 与分片上传时一致 |
| `totalChunks` | int | 是 | 分片总数，必须与上传时一致 |

不带 `file` 字段。

### 请求示例

```bash
curl -X POST http://127.0.0.1:2000/api/file/upload_chunk_complete \
  -F "projectName=demo" \
  -F "relativeFileName=big/file.bin" \
  -F "totalChunks=2"
```

### 响应

成功：

```json
{ "isSuccess": true, "errorMsg": "", "data": { "chunksComplete": true, "totalSize": 1500 } }
```

分片缺失（**仍然是成功响应**，调用方必须读 `chunksComplete`）：

```json
{ "isSuccess": true, "errorMsg": "", "data": { "chunksComplete": false, "missingChunks": [2] } }
```

| 字段 | 类型 | 出现条件 | 说明 |
| --- | --- | --- | --- |
| `chunksComplete` | bool | 总是 | 分片是否齐全并已合并 |
| `totalSize` | int64 | `chunksComplete = true` | 合并后文件字节数 |
| `missingChunks` | int[] | `chunksComplete = false` | 缺失的分片序号（升序） |

### 行为说明

- 按 `0 … totalChunks-1` 逐个 `stat` 分片；有缺失就返回 `missingChunks`，不合并、不清理，客户端补齐后重试即可。
- 所有分片大小之和超过 1 GB 时返回「文件过大」，不合并。
- 合并到 `{目标路径}.merging`，成功后 `rename` 到最终路径，再删除 `.chunks` 目录；任何一步失败都会清理临时文件。
- 目标是覆盖写：重复上传同名文件会替换旧内容，`get_all_files` 的哈希随之变化。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称不能为空` | `projectName` 缺失 |
| `文件路径不能为空` | `relativeFileName` 缺失 |
| `非法的文件路径` | 路径逃出项目目录 |
| `未找到分片目录，请先上传分片` | `.chunks` 不存在（未上传，或已完成合并后被再次调用） |
| `文件过大: N 字节，上限 1073741824 字节` | 分片总大小超 1 GB |
| `create dest dir error: ...` | 目标目录创建失败 |
| `create dest file error: ...` | 临时文件创建失败 |
| `open chunk N error: ...` / `copy chunk N error: ...` | 读取或写入分片失败 |
| `close merged file error: ...` / `atomic rename merged file error: ...` | 关闭或重命名失败 |

## 获取文件列表

返回项目下的全部文件及双哈希，是客户端决定"下载哪些文件"的依据。

| 项 | 值 |
| --- | --- |
| 方法 | `GET` |
| 路径 | `/api/file/get_all_files/{projectName}` |
| 路径参数 | `projectName`：项目名称，需 URL 编码 |

### 请求示例

```bash
curl http://127.0.0.1:2000/api/file/get_all_files/demo
```

### 响应

`data` 为 `FileInfo` 数组，按 `fileRelativePath` 升序：

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

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `fileAbsolutePath` | string | 服务端绝对路径；下载时原样作为 `download_file` 的 `path` |
| `fileRelativePath` | string | 相对项目目录的路径，`/` 分隔 |
| `lastUpdateTime` | string | 文件 mtime，RFC 3339 |
| `fileSize` | int64 | 字节数 |
| `md5` | string | MD5，小写十六进制 |
| `sha256` | string | SHA-256，小写十六进制 |

### 过滤与缓存

列表不包含：

- 上传中间态：任何以 `.chunks` 结尾的目录段、以 `.merging` 结尾的文件；
- 发布清单自身 `.updator_manifest.json`；
- 命中项目 `ignoreFolders` / `ignoreFiles` 的文件（忽略文件支持精确匹配、glob 全路径、glob 文件名、`*.ext` 后缀）。

服务端按项目缓存该列表（含哈希）。缓存命中只做一次 `stat` 指纹比对（相对路径 + 大小 + mtime）；指纹变化（如直接往 `data/` 拷文件）或上传接口触发失效都会重建。响应中不含缓存标记。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `ent: project not found` | 项目不存在（软删除后同样报此错） |

项目存在但目录为空时返回 `"data": []`。

## 获取发布清单

返回最近一次成功发布时固化的文件相对路径集合，**只用于删除同步**。

| 项 | 值 |
| --- | --- |
| 方法 | `GET` |
| 路径 | `/api/file/get_release_manifest/{projectName}` |
| 路径参数 | `projectName`：项目名称，需 URL 编码 |

### 请求示例

```bash
curl http://127.0.0.1:2000/api/file/get_release_manifest/demo
```

### 响应

```json
{ "isSuccess": true, "errorMsg": "", "data": { "files": ["big/file.bin", "sub/small.txt"] } }
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `files` | string[] | 发布时的文件相对路径集合，`/` 分隔 |

**只有 `files` 数组**：没有大小、没有哈希、没有版本号。需要哈希或绝对路径时用 `get_all_files`。

### 行为说明

- 清单文件为 `data/{projectName}/.updator_manifest.json`，由 `publish_version` 成功后在事务外写入，采用「临时文件 + rename」原子写。
- 清单内容与 `get_all_files` 同口径过滤（中间态、忽略规则、清单自身都不入清单）。
- 客户端在 `download_update` 末尾读取：清单之外的文件视为"发布端已删除"，在本地清理；服务端从未发布过时 `files` 为空，客户端跳过删除。
- 项目从未发布、项目不存在、项目目录不存在时，都返回**成功 + 空数组**，不报错。这一行为用于兼容老服务器/老项目。
- 清单只在发布时更新。发布后直接往 `data/` 里拷文件，`get_all_files` 能看到新文件，但清单不会变，客户端不会主动删除它们。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |
| `unexpected end of JSON input` 等 | 清单文件被外部损坏，无法解析 |

## 下载文件

按服务端绝对路径下载文件，支持 `Range` 断点续传。

| 项 | 值 |
| --- | --- |
| 方法 | `GET` |
| 路径 | `/api/file/download_file` |
| 查询参数 | `path`：`data/` 下的绝对路径，需 URL 编码 |
| 响应 | 文件字节流，**不是** JSON 信封 |

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `path` | string | 是 | 必须取 `get_all_files` 返回的 `fileAbsolutePath`，绝对路径且位于 `data/` 下 |

`path` 的取值规则很严：

- 相对路径（如 `data/demo/app.exe`）→ 404；
- 不在 `data/` 下的路径（如 `/etc/passwd`、`C:\Windows\win.ini`）→ 404；
- 上传中间态（`.chunks` / `.merging`）→ 404；
- 文件不存在 → 404；
- 其他 I/O 错误 → 500。

### 请求示例

```bash
# 用 --data-urlencode 处理路径中的空格与中文
curl -G -o app.exe \
  --data-urlencode "path=/opt/aly/data/demo/app.exe" \
  http://127.0.0.1:2000/api/file/download_file

# 断点续传：curl 自动带 Range: bytes=<已有字节>-
curl -C - -o app.exe \
  --data-urlencode "path=/opt/aly/data/demo/app.exe" \
  http://127.0.0.1:2000/api/file/download_file
```

### 响应

成功返回 `200 OK`，响应头示例：

```http
Content-Disposition: attachment; filename*=UTF-8''small.txt; filename=small.txt
Content-Transfer-Encoding: binary
Accept-Ranges: bytes
Content-Length: 25
```

带 `Range` 时返回 `206 Partial Content` 与 `Content-Range: bytes 0-9/25`；范围越界返回 `416`。

### 状态码

| 状态码 | 含义 |
| --- | --- |
| `200` | 完整文件 |
| `206` | `Range` 命中的部分内容 |
| `404` | 路径非法、越界、文件不存在（空响应体） |
| `416` | `Range` 无法与文件重叠（带 `Content-Range: bytes */<size>`，响应体为一行错误文本） |
| `500` | 打开或读取文件时的其他错误（空响应体） |

404 / 500 由服务端直接中止请求，没有错误说明；416 由 `http.ServeContent` 生成，带一行 `invalid range` 文本。这是服务端**唯一**使用真实状态码表达失败的接口。

### 行为说明

- 用 `http.ServeContent` 实现，天然支持单段 `Range`、多段 `Range` 与 `If-Range`；
- `Content-Disposition` 同时给出 RFC 6266 的 `filename*`（UTF-8，空格转 `%20`）与回退的 `filename`；
- 服务端不预检文件存在性，直接打开后判断错误，避免 TOCTOU；
- 服务端 `WriteTimeout` 为 60 秒（整次响应），慢链路上的超大文件可能被中途断开，客户端应使用续传重试。aly-client 对超过 100 MB 的文件启用 `.part` + `Range` 续传。

## 相关页面

- [API 总览](/docs/api)：信封、错误语义、客户端调用链
- [项目接口](/docs/server/api-project)：项目创建与版本发布
- [数据模型](/docs/server/api-models)：`FileInfo` 与发布清单字段
- [部署 aly-server](/docs/server/deploy)：数据目录、代理上传上限、下载超时
- [aly-publish 发布工具](/docs/publish)：调用上传接口的一方