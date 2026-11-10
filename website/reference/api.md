# 服务端 API

aly-server 基于 Gin 提供 REST API，统一前缀 `/api`。所有接口无需认证（设计为部署在受信内网）。

## 项目管理 — `/api/project`

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/project/create_project` | 创建项目 |
| POST | `/api/project/update_project` | 更新项目配置 |
| POST | `/api/project/set_force_update` | 设置强制更新标志 |
| GET | `/api/project/get_all_projects` | 获取所有项目 |
| GET | `/api/project/get_project_by_name/:projectName` | 按名称查询项目 |
| GET | `/api/project/get_project_change_logs/:projectName` | 获取版本变更日志 |
| POST | `/api/project/delete_project/:projectName` | 删除项目 |
| POST | `/api/project/publish_version` | 发布版本（版本号必须严格大于当前最新版本） |
| GET | `/api/project/get_project_os_info/:projectName` | 获取项目 OS 信息 |

## 服务端 — `/api/server`

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/server/info` | 服务端系统信息（publish-cli `server info` 使用） |

## 文件 — `/api/file`

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/file/upload_file` | 上传文件（有大小上限保护） |
| POST | `/api/file/upload_chunk` | 分片上传（大文件） |
| POST | `/api/file/upload_chunk_complete` | 完成分片上传（合并校验） |
| GET | `/api/file/get_all_files/:projectName` | 获取项目全部文件清单 |
| GET | `/api/file/get_release_manifest/:projectName` | 获取发布清单（客户端 `check_update` / `download_update` 的数据源） |
| GET | `/api/file/download_file` | 下载文件 |

## 客户端实际使用的接口

一次完整的客户端更新只触碰两个接口：

1. `GET /api/file/get_release_manifest/:projectName` — `check_update` 对比本地版本，`download_update` 拿文件清单与哈希。
2. `GET /api/file/download_file` — 逐文件下载。

## 安全性说明

- 传输完整性：下载内容按 MD5 + SHA256 双哈希校验，防传输损坏。
- 版本单调：`publish_version` 拒绝发布不大于当前最新版本的版本号，防止误发旧内容把客户端"卡死"。
- 上传保护：请求体大小限制 + 内存 multipart 上限双保险。
- **注意**：当前版本没有内置认证与签名机制，请将服务端部署在受信网络内。若需要公网暴露，建议在前置网关做认证，或关注仓库后续的加固计划。
