---
title: 数据模型
description: aly-server 的库表与 JSON 字段：Project、ProjectChangeLog、FileInfo、ServerOSInfo 与发布清单，以及请求 camelCase 与响应 snake_case 的命名对照。
---

# 数据模型

服务端有两类"模型"，命名风格不同，混用是最常见的集成错误来源：

| 来源 | 用途 | 命名风格 | 示例 |
| --- | --- | --- | --- |
| 控制器内手写 DTO | 请求体（POST JSON / multipart） | camelCase | `isForceUpdate`、`ignoreFolders` |
| ent 生成实体 | 响应（项目、变更日志） | snake_case | `force_update`、`ignore_folders` |
| `server/models` 结构体 | 响应（文件列表、系统信息） | camelCase | `fileAbsolutePath`、`goARCH` |

## 命名约定

### 请求：camelCase

| 接口 | 字段 |
| --- | --- |
| `create_project` / `update_project` | `name`、`title`、`isForceUpdate`、`ignoreFolders`、`ignoreFiles` |
| `set_force_update` | `projectName`、`forceUpdate` |
| `publish_version` | `projectName`、`version`、`logs`、`time`、`afterApplyUpdateScript` |
| `upload_file` / `upload_chunk` | `projectName`、`relativeFileName`（+ `chunkIndex`、`totalChunks`） |
| `upload_chunk_complete` | `projectName`、`relativeFileName`、`totalChunks` |
| `download_file` | 查询参数 `path` |

### 响应：两套风格并存

- `Project`、`ProjectChangeLog`：由 ent 生成，snake_case（`force_update`、`after_apply_update_script`）。
- `FileInfo`、`ServerOSInfo`：由 `server/models/models.go` 定义，camelCase（`fileAbsolutePath`、`goARCH`）。

字段映射：

| 语义 | 请求字段 | 响应字段 |
| --- | --- | --- |
| 强制更新 | `isForceUpdate` / `forceUpdate` | `force_update` |
| 忽略文件夹 | `ignoreFolders` | `ignore_folders` |
| 忽略文件 | `ignoreFiles` | `ignore_files` |
| 更新后脚本 | `afterApplyUpdateScript` | `after_apply_update_script` |
| 项目 ID | —— | `project_id` |
| 删除标志 | —— | `is_deleted` |

### `omitempty`：零值字段不会出现

ent 实体的业务字段 JSON tag 全部带 `omitempty`（只有 `edges` 例外，它总是输出 `{}`），所以响应里**可能缺少某些键**，而不是给出 `false` / `[]`：

| 字段 | 何时不出现 |
| --- | --- |
| `force_update` | 值为 `false` |
| `is_deleted` | 值为 `false` |
| `ignore_folders` | 空数组 |
| `ignore_files` | 空数组 |
| `after_apply_update_script` | 空字符串 |
| `id` / `project_id` | 值为 `0`（正常数据不会发生） |

`created_at` 是 `time.Time` 结构体，`omitempty` 对它无效，所以总是出现。

反序列化方必须把缺失键当作零值处理（aly-publish 的 `models.Project` 正是这么定义的）。不要用"键是否存在"判断业务状态。

## Project

对应 `server/ent/schema/project.go`，表名 `projects`。

| JSON 字段 | 类型 | 数据库列 | 说明 |
| --- | --- | --- | --- |
| `id` | int | `id` | 自增主键 |
| `name` | string | `name` | 项目名称，**唯一**；即 `data/` 下的目录名，创建后不可改 |
| `title` | string | `title` | 项目抬头，展示用 |
| `version` | string | `version` | 当前版本号；创建时为 `V1.0.0`，由 `publish_version` 更新 |
| `force_update` | bool | `force_update` | 是否强制更新；`false` 时不出现 |
| `ignore_folders` | string[] | `ignore_folders` | 忽略的文件夹（JSON 数组），按前缀匹配 |
| `ignore_files` | string[] | `ignore_files` | 忽略的文件（JSON 数组），支持 glob |
| `created_at` | string | `created_at` | 创建时间，默认 `time.Now()` |
| `is_deleted` | bool | `is_deleted` | 软删除标志，默认 `false` |
| `edges` | object | —— | 关联占位，恒为 `{}`；`change_logs` 未预加载 |

关系：`projects` 1—N `project_change_logs`（通过 `project_id`）。

`name` 的唯一约束是数据库级别的，因此软删除的项目仍占用该名称：删除后无法用同名重建。

### 示例

```json
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
```

## ProjectChangeLog

对应 `server/ent/schema/projectchangelog.go`，表名 `project_change_logs`。

| JSON 字段 | 类型 | 数据库列 | 说明 |
| --- | --- | --- | --- |
| `id` | int | `id` | 自增主键；日志接口按它倒序 |
| `project_id` | int | `project_id` | 所属项目 ID；外键指向 `projects.id`，`ON DELETE SET NULL` |
| `version` | string | `version` | 该次发布的版本号 |
| `logs` | string[] | `logs` | 变更说明（JSON 数组） |
| `time` | string | `time` | 发布端传入的时间字符串；服务端只校验格式，不转换时区 |
| `created_at` | string | `created_at` | 服务端写入时间 |
| `is_deleted` | bool | `is_deleted` | 软删除标志，默认 `false`；日志查询会过滤 |
| `after_apply_update_script` | string | `after_apply_update_script` | 更新后执行的脚本，相对应用目录；空字符串时不出现 |
| `edges` | object | —— | 恒为 `{}`；`project` 未预加载 |

创建项目时自动写入一条：`version = V1.0.0`、`logs = ["第一次创建"]`、`time` 为服务端当前时间。

### 示例

```json
{
  "id": 3,
  "project_id": 1,
  "version": "1.0.1",
  "logs": ["修复 A", "修复 B"],
  "time": "2026-11-10 11:00:00",
  "created_at": "2026-11-10T10:59:44.0995513+08:00",
  "after_apply_update_script": "run.bat",
  "edges": {}
}
```

`project_id` 为 `0` / 缺失只可能来自旧库迁移残留（旧列 `project_change_logs` 未搬完）或直接改库；两条正常写入路径（`create_project`、`publish_version`）都会带值。

## FileInfo

对应 `server/models/models.go` 的 `FileInfo`，由 `get_all_files` 返回，不落库。

| JSON 字段 | 类型 | 说明 |
| --- | --- | --- |
| `fileAbsolutePath` | string | 服务端绝对路径；`download_file` 的 `path` 必须用它 |
| `fileRelativePath` | string | 相对项目目录的路径，`/` 分隔，排序依据 |
| `lastUpdateTime` | string | 文件 mtime |
| `fileSize` | int64 | 字节数 |
| `md5` | string | MD5，小写十六进制（32 字符） |
| `sha256` | string | SHA-256，小写十六进制（64 字符） |

列表按 `fileRelativePath` 升序，已过滤上传中间态、发布清单与项目忽略规则。

### 示例

```json
{
  "fileAbsolutePath": "/opt/aly/data/demo/app.exe",
  "fileRelativePath": "app.exe",
  "lastUpdateTime": "2026-11-10T11:00:09.4428015+08:00",
  "fileSize": 1500,
  "md5": "2e3587c7179025e801964d9babf8c70e",
  "sha256": "e83835331f9b903cb728137f30b1bb1d83d3751ed561801e79555cfaca4eeb45"
}
```

路径分隔符随服务端操作系统变化（Windows 为 `\`，Linux/macOS 为 `/`），是给服务端自己解析的绝对路径，客户端不应改写它。

## ServerOSInfo

对应 `server/models/models.go` 的 `ServerOSInfo`，由 `get_project_os_info` 与 `server/info` 返回，固定为**单元素数组**。

| JSON 字段 | 类型 | 说明 |
| --- | --- | --- |
| `os` | string | `runtime.GOOS`：`windows` / `linux` / `darwin` |
| `platform` | string | 发行版/平台描述；取不到时为空字符串 |
| `goARCH` | string | `runtime.GOARCH`：`amd64` / `arm64` / `386` |
| `version` | string | Go 运行时版本，如 `go1.25.0` |
| `numCPU` | int | 逻辑 CPU 数 |
| `cpuName` | string | CPU 型号；取不到时为 `unknown` |
| `cpuMhz` | float64 | CPU 主频（MHz）；取不到时为 `0` |
| `diskUsed` | float64 | 已用空间（GB） |
| `diskFree` | float64 | 可用空间（GB） |
| `diskTotal` | float64 | 总空间（GB） |
| `diskUsedPercent` | float64 | 使用率百分比（`0`–`100`） |

注意字段名的大小写细节：`goARCH`（全大写 ARCH）、`cpuMhz`（小写 `hz`）、`numCPU`。

磁盘统计针对项目目录所在分区；`server/info` 统计可执行文件所在分区。

### 示例

```json
[
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
```

## 发布清单（manifest）

不是数据库实体，没有对应的 ent schema。它是发布时写到磁盘的一个 JSON 文件：

- 路径：`data/{projectName}/.updator_manifest.json`
- 由 `publish_version` 成功后写入（临时文件 + `rename` 原子写）
- 通过 `get_release_manifest` 下发

```json
{
  "files": ["app.exe", "lib/core.dll", "config/app.json"]
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `files` | string[] | 发布时的文件相对路径集合，`/` 分隔，已排序 |

**只有 `files` 一个字段**：不含大小、哈希、版本号或时间。需要这些信息时调用 `get_all_files`。

清单的过滤口径与 `get_all_files` 完全一致：

- 跳过 `.chunks` / `.merging` 上传中间态；
- 应用项目的 `ignore_folders` / `ignore_files`；
- 排除清单自身 `.updator_manifest.json`。

契约行为：

| 场景 | `get_release_manifest` 返回 |
| --- | --- |
| 正常发布过 | 发布时的路径集合 |
| 从未发布 | `{"files": []}`，`isSuccess: true` |
| 项目不存在 / 目录不存在 | `{"files": []}`，`isSuccess: true` |
| 清单文件损坏 | `isSuccess: false`，`errorMsg` 为 JSON 解析错误 |

## 时间字段

| 字段 | 格式 | 示例 |
| --- | --- | --- |
| `created_at`（Project / ProjectChangeLog） | RFC 3339，纳秒精度，本地时区偏移 | `2026-11-10T10:59:35.6148004+08:00` |
| `lastUpdateTime`（FileInfo） | 同上，取自文件 mtime | `2026-11-10T11:00:09.4428015+08:00` |
| `time`（ProjectChangeLog） | `yyyy-MM-dd HH:mm:ss`，发布端传入的字符串 | `2026-11-10 11:00:00` |

- 服务端不做时区转换：`created_at` 用服务端本地时区，`time` 原样保存发布端给的值。
- `time` 允许为空；非空时必须能被解析为 `yyyy-MM-dd HH:mm:ss`，否则发布被拒。
- 客户端比较版本用 `version` 字段，不用时间；版本新旧由 `CompareVersion` 决定（见 [发布版本](/docs/server/api-project#版本号规则)）。

## 实体关系

```text
projects
├── id            (PK)
├── name          (UNIQUE)
└── version
      │ 1
      │
      │ N
project_change_logs
├── id            (PK)
├── project_id    (FK → projects.id, ON DELETE SET NULL)
├── version
└── logs
```

- 两张表都有 `is_deleted`，查询时按需过滤：`get_all_projects` 过滤，`get_project_by_name` 不过滤。
- 服务端实际不物理删除任何行，`ON DELETE SET NULL` 只是外键定义层面的保护。
- 表结构与列变更由 ent 在启动时自动迁移；老库的 `project_id` 列由一次性迁移补齐（见 [部署](/docs/server/deploy)）。

## 相关页面

- [API 总览](/docs/api)：响应信封与错误语义
- [项目接口](/docs/server/api-project)：`Project` / `ProjectChangeLog` 的写入路径
- [文件接口](/docs/server/api-file)：`FileInfo` 与 manifest 的读取路径
- [部署 aly-server](/docs/server/deploy)：数据库、WAL、备份与迁移