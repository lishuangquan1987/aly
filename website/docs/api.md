---
title: 服务端 API 总览
description: aly-server 的请求约定、统一响应信封、HTTP 200 错误语义、无认证的信任边界，以及按项目、文件、服务端分组的接口索引。
---

# 服务端 API 总览

aly-server 基于 Gin 提供 HTTP 接口，全部注册在 `/api` 前缀下（见 `server/routers/routers.go`）。

| 项目 | 值 |
| --- | --- |
| 基础 URL | `http://<host>:2000/api` |
| 监听地址 | `0.0.0.0:2000`，端口由 `-p` 指定，默认 2000 |
| 认证 | 无 |
| 请求体 | JSON 或 `multipart/form-data` |
| 响应体 | JSON；`download_file` 返回文件字节流 |
| 字符集 | UTF-8 |

端口、数据目录与进程管理的细节见 [部署 aly-server](/docs/server/deploy)。

## 统一响应信封

除文件下载外，所有接口都返回 HTTP 200，响应体为固定三字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `isSuccess` | bool | 业务是否成功 |
| `errorMsg` | string | 失败原因；成功时为 `""` |
| `data` | any | 业务数据；失败时为 `null`；无返回值的成功也为 `null` |

成功（`get_project_by_name`）：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 1,
    "name": "demo",
    "title": "演示项目",
    "version": "V1.0.0",
    "created_at": "2026-11-10T10:59:35.6148004+08:00",
    "edges": {}
  }
}
```

失败（重复创建同名项目）：

```json
{ "isSuccess": false, "errorMsg": "项目名称:demo已存在", "data": null }
```

该信封对应 `models.CommonResponse`，客户端 SDK 与 aly-publish 都以 `isSuccess` 判定成败。

## 错误语义：HTTP 200 + `isSuccess:false`

服务端不用 4xx/5xx 表达业务错误。参数校验失败、项目不存在、版本号倒退、磁盘写入失败等一律返回 **HTTP 200**，把原因放在 `errorMsg`。因此：

- 只判断 HTTP 状态码会把所有失败当成成功；
- 必须解析响应体，`isSuccess == false` 时读取 `errorMsg`。

只有两类响应不使用该信封：

| 场景 | 状态码 | 响应体 |
| --- | --- | --- |
| `GET /api/file/download_file` | 200 / 206 / 404 / 416 / 500 | 文件字节流，或空响应体 |
| 路由不存在、方法不匹配 | 404 | Gin 默认纯文本 `404 page not found` |

`download_file` 是流式接口，无法在传输开始后再写错误信封，所以它用真实状态码；调用方需要单独处理。

## 请求格式约定

| 场景 | 编码 | 字段命名 |
| --- | --- | --- |
| 项目管理类 POST | `application/json` | camelCase，如 `isForceUpdate`、`ignoreFolders` |
| 文件上传类 POST | `multipart/form-data` | 表单字段 camelCase，如 `projectName`、`relativeFileName` |
| 项目 / 变更日志响应 | `application/json` | snake_case，直接序列化 ent 实体 |
| 文件列表 / 系统信息响应 | `application/json` | camelCase，序列化 `server/models` 结构体 |

请求与响应字段命名不一致是**有意为之**：请求 DTO 手写在控制器里，响应直接复用 ent 生成的实体。逐字段对照见 [数据模型](/docs/server/api-models)。

## 信任边界：没有认证

服务端没有 Token、签名、IP 白名单或限流。任何能连上端口的客户端都可以创建/删除项目、上传文件、发布版本、下载全部数据。设计前提是**部署在受信内网**：

- 不要将端口直接暴露到公网；
- 上传接口允许写入最大 1 GB 的文件，暴露端口等于开放磁盘写入；
- 现有客户端请求不带认证头，在反向代理上加 Basic Auth 会直接让官方客户端失效；跨网访问应使用防火墙白名单或 VPN。

加固与反向代理配置见 [部署 aly-server](/docs/server/deploy)。

## 接口索引

### 项目 `/api/project`

| 方法 | 路径 | 说明 | 成功 `data` |
| --- | --- | --- | --- |
| POST | [`/api/project/create_project`](/docs/server/api-project#创建项目) | 创建项目并写入首条变更日志 | `Project` |
| POST | [`/api/project/update_project`](/docs/server/api-project#更新项目) | 更新抬头、强制更新标志、忽略规则 | `null` |
| POST | [`/api/project/set_force_update`](/docs/server/api-project#设置强制更新) | 只改强制更新标志 | `null` |
| GET | [`/api/project/get_all_projects`](/docs/server/api-project#获取全部项目) | 列出未删除项目 | `Project[]` |
| GET | [`/api/project/get_project_by_name/:projectName`](/docs/server/api-project#按名称查询项目) | 按名称查单个项目 | `Project` |
| GET | [`/api/project/get_project_change_logs/:projectName`](/docs/server/api-project#获取变更日志) | 变更日志，按 ID 倒序 | `ProjectChangeLog[]` |
| POST | [`/api/project/delete_project/:projectName`](/docs/server/api-project#删除项目) | 软删除项目 | `null` |
| POST | [`/api/project/publish_version`](/docs/server/api-project#发布版本) | 发布新版本并固化发布清单 | `ProjectChangeLog` |
| GET | [`/api/project/get_project_os_info/:projectName`](/docs/server/api-project#获取项目所在服务器信息) | 服务端系统信息（按项目定位磁盘） | `ServerOSInfo[]` |

### 文件 `/api/file`

| 方法 | 路径 | 说明 | 成功 `data` |
| --- | --- | --- | --- |
| POST | [`/api/file/upload_file`](/docs/server/api-file#直传文件) | 整文件上传，单文件上限 1 GB | `null` |
| POST | [`/api/file/upload_chunk`](/docs/server/api-file#上传分片) | 上传单个分片，单分片上限 64 MB | `null` |
| POST | [`/api/file/upload_chunk_complete`](/docs/server/api-file#完成分片上传) | 校验并合并分片 | `{chunksComplete,totalSize}` 或 `{chunksComplete,missingChunks}` |
| GET | [`/api/file/get_all_files/:projectName`](/docs/server/api-file#获取文件列表) | 文件列表，含 MD5 / SHA256 | `FileInfo[]` |
| GET | [`/api/file/get_release_manifest/:projectName`](/docs/server/api-file#获取发布清单) | 发布清单（仅相对路径） | `{files:string[]}` |
| GET | [`/api/file/download_file`](/docs/server/api-file#下载文件) | 按绝对路径下载，支持 Range | 文件字节流 |

### 服务端 `/api/server`

| 方法 | 路径 | 说明 | 成功 `data` |
| --- | --- | --- | --- |
| GET | `/api/server/info` | 服务端系统信息，无需项目名 | `ServerOSInfo[]` |

`ServerOSInfo` 是单元素数组（服务端固定返回 `[{...}]`），磁盘容量单位 GB。

## 客户端实际调用链

更新器 aly-client 只使用上表中的一个子集，两条命令的调用顺序固定：

| 命令 | 调用顺序 |
| --- | --- |
| `check_update` | `get_project_by_name` → `get_project_change_logs` |
| `download_update` | `get_project_change_logs` → `get_all_files` → `download_file` × N → `get_release_manifest` |

各步骤的作用：

1. **`get_project_by_name`**：确认项目存在（客户端判断 `is_deleted`），取得项目当前版本。
2. **`get_project_change_logs`**：取服务端最新版本号与变更日志，与本地版本比较决定 `has_update` / `need_download_update`；同时取 `after_apply_update_script`。
3. **`get_all_files`**：取文件清单与双哈希，决定下载哪些文件、哪些未变化的文件需要在本地物化。
4. **`download_file`**：逐个下载变更文件，参数直接使用上一步返回的 `fileAbsolutePath`。
5. **`get_release_manifest`**：**只用于删除同步**。下载完成后，清单之外的文件被视为"发布端已删除"，在本地被清理；服务端从未发布过时返回空 `files`，客户端跳过删除。

上传侧（aly-publish `push`）使用 `upload_file`（≤ 1 MiB）或 `upload_chunk` + `upload_chunk_complete`（> 1 MiB），随后调用 `publish_version` 登记版本。

`get_all_projects`、`set_force_update`、`get_project_os_info`、`server/info` 由 aly-publish 与发布 GUI 使用，不在客户端更新主链上。命令层面的说明见 [aly-client 命令速查](/docs/commands) 与 [aly-publish 发布工具](/docs/publish)。

### curl 快速验证

```bash
# 列项目（不需要任何请求头）
curl http://127.0.0.1:2000/api/project/get_all_projects

# 建项目
curl -X POST http://127.0.0.1:2000/api/project/create_project \
  -H "Content-Type: application/json" \
  -d '{"name":"demo","title":"演示项目"}'

# 取文件列表（拿 fileAbsolutePath 供下载使用）
curl http://127.0.0.1:2000/api/file/get_all_files/demo
```

## 数据目录

所有项目内容存放在**可执行文件所在目录**下的 `data/`：

```text
/opt/aly/
├── aly-server              # 二进制
├── aly.db                  # SQLite 数据库（可用 -db 改路径）
└── data/
    └── demo/               # 项目名即目录名
        ├── app.exe
        ├── lib/
        │   └── core.dll
        └── .updator_manifest.json
```

这带来两个 API 层面的约束：

- `get_all_files` 返回的 `fileAbsolutePath` 是**服务端文件的绝对路径**；`download_file` 的 `path` 参数必须原样传回该值，且必须落在 `data/` 下。传相对路径或 `data/` 之外的路径一律 404。
- 绝对路径依赖二进制位置。迁移二进制或数据目录后，客户端缓存的旧 `fileAbsolutePath` 会失效，应重新拉取 `get_all_files`。

数据库与备份见 [部署 aly-server](/docs/server/deploy)。

## 相关页面

- [部署 aly-server](/docs/server/deploy)：单二进制、`-p` / `-db`、数据目录、systemd、备份
- [项目接口](/docs/server/api-project)：9 个项目接口的完整参数与错误
- [文件接口](/docs/server/api-file)：上传、分片、清单与下载
- [数据模型](/docs/server/api-models)：JSON 字段名与命名风格对照
- [aly-client 命令速查](/docs/commands)：调用这些接口的命令行入口