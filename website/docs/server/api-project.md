---
title: 项目接口
description: aly-server 的 9 个项目接口：创建、更新、删除、查询、变更日志、发布版本与系统信息，含参数、curl 示例、响应字段与错误文本。
---

# 项目接口

项目接口负责项目生命周期与版本登记，全部挂在 `/api/project` 下，共 9 个。

所有接口返回统一信封（HTTP 200 + `isSuccess` / `errorMsg` / `data`），失败时不使用 4xx/5xx。信封与错误语义见 [API 总览](/docs/api)。

## 通用约定

### 项目名称规则

项目名称同时是 `data/` 下的目录名，因此按文件系统规则校验（`validateProjectName`）：

| 规则 | 说明 |
| --- | --- |
| 长度 | 1–64 个字符 |
| 允许 | 除下列字符外的任意字符，含中文、空格 |
| 禁止字符 | `<` `>` `:` `"` `/` `\` `|` `?` `*` |
| 禁止保留名 | `CON` `PRN` `AUX` `NUL` `COM1`–`COM9` `LPT1`–`LPT9`（大小写不敏感） |
| 不可变 | 创建后不能改名；`update_project` 的 `name` 只用于定位记录 |

非法名称的返回：

```json
{ "isSuccess": false, "errorMsg": "项目名称包含非法字符或长度不在1-64范围内", "data": null }
{ "isSuccess": false, "errorMsg": "项目名称不能使用系统保留名: CON", "data": null }
```

### 响应中的 `Project` 字段

`Project` 直接序列化 ent 实体，字段为 snake_case，且零值字段会被省略（`omitempty`）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int | 自增主键 |
| `name` | string | 项目名称 |
| `title` | string | 项目抬头（展示名） |
| `version` | string | 当前版本号，创建时为 `V1.0.0` |
| `force_update` | bool | 强制更新标志；为 `false` 时**不出现** |
| `ignore_folders` | string[] | 忽略的文件夹；为空时**不出现** |
| `ignore_files` | string[] | 忽略的文件（glob）；为空时**不出现** |
| `created_at` | string | 创建时间，RFC 3339，带本地时区偏移 |
| `is_deleted` | bool | 软删除标志；为 `false` 时**不出现** |
| `edges` | object | 关联占位，恒为 `{}`（不预加载 `change_logs`） |

完整说明见 [数据模型](/docs/server/api-models)。

## 创建项目

创建项目记录，同时写入首条变更日志。

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/project/create_project` |
| Content-Type | `application/json` |

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 项目名称，规则见上 |
| `title` | string | 是 | 项目抬头，不能为空 |
| `isForceUpdate` | bool | 否 | 强制更新标志，缺省 `false` |
| `ignoreFolders` | string[] | 否 | 忽略的文件夹，缺省空 |
| `ignoreFiles` | string[] | 否 | 忽略的文件（支持 glob），缺省空 |

### 请求示例

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

### 响应

`data` 为创建出的 `Project` 对象：

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

### 行为说明

- 项目名不存在时才创建；已存在直接返回失败，不覆盖。
- 在同一事务里写入两条记录：`projects` 行（`version = V1.0.0`）与 `project_change_logs` 首条日志（`version = V1.0.0`、`logs = ["第一次创建"]`、`time` 为服务端当前时间 `yyyy-MM-dd HH:mm:ss`）。
- 自动创建 `data/{name}` 目录（已存在则跳过）。
- 并发重复创建由 `name` 唯一约束兜底，冲突时同样返回「项目名称已存在」。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称不能为空` | `name` 缺失或为空 |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称含禁用字符或超长 |
| `项目名称不能使用系统保留名: X` | 名称为 Windows 保留设备名 |
| `项目抬头不能为空` | `title` 缺失或为空 |
| `项目名称:X已存在` | 同名项目已存在（含已软删除的项目） |
| `invalid character ...` | 请求体不是合法 JSON（原文来自 JSON 解析器） |

## 更新项目

更新抬头、强制更新标志与忽略规则。**不能改名，也不能改版本号。**

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/project/update_project` |
| Content-Type | `application/json` |

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 目标项目名称，需存在且未软删除 |
| `title` | string | 是 | 新抬头，不能为空 |
| `isForceUpdate` | bool | 否 | 强制更新标志，缺省 `false` |
| `ignoreFolders` | string[] | 否 | 新的忽略文件夹列表，**整体覆盖**旧值 |
| `ignoreFiles` | string[] | 否 | 新的忽略文件列表，**整体覆盖**旧值 |

### 请求示例

```bash
curl -X POST http://127.0.0.1:2000/api/project/update_project \
  -H "Content-Type: application/json" \
  -d '{"name":"demo","title":"演示项目（改名）","isForceUpdate":false}'
```

### 响应

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### 行为说明

- 忽略规则是**全量替换**，不传即清空。只想改抬头时，应把原有 `ignoreFolders` / `ignoreFiles` 一起回传，否则规则被清空，之后 `get_all_files` 会把原本忽略的文件也下发。
- 成功后服务端使该项目的文件列表缓存失效，下次 `get_all_files` 按新规则重新扫描。
- 不改变 `version`，版本只能通过 `publish_version` 前进。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |
| `项目抬头不能为空` | `title` 缺失或为空 |
| `项目不存在` | 项目不存在或已被软删除 |

## 设置强制更新

只写 `force_update` 一个字段，避免「读-改-写」覆盖并发修改。

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/project/set_force_update` |
| Content-Type | `application/json` |

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `projectName` | string | 是 | 项目名称 |
| `forceUpdate` | bool | 是 | 目标值；缺省按 `false` 处理 |

### 请求示例

```bash
curl -X POST http://127.0.0.1:2000/api/project/set_force_update \
  -H "Content-Type: application/json" \
  -d '{"projectName":"demo","forceUpdate":true}'
```

### 响应

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |
| `项目不存在: X` | 项目不存在或已软删除（影响行数为 0） |

## 获取全部项目

| 项 | 值 |
| --- | --- |
| 方法 | `GET` |
| 路径 | `/api/project/get_all_projects` |

### 请求参数

无。

### 请求示例

```bash
curl http://127.0.0.1:2000/api/project/get_all_projects
```

### 响应

`data` 为 `Project` 数组，**只包含未软删除的项目**：

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

字段与 `Project` 表一致。没有分页，也没有 `ORDER BY`，返回顺序不作保证；无项目时返回 `"data": []`。

### 常见错误

只有数据库层面的异常（`errorMsg` 为 ent 原始错误），正常路径不会失败。

## 按名称查询项目

| 项 | 值 |
| --- | --- |
| 方法 | `GET` |
| 路径 | `/api/project/get_project_by_name/{projectName}` |
| 路径参数 | `projectName`：项目名称，需 URL 编码 |

### 请求示例

```bash
curl http://127.0.0.1:2000/api/project/get_project_by_name/demo
```

### 响应

`data` 为单个 `Project` 对象（字段见上表）。

### 行为说明

- 该查询**过滤 `is_deleted`**：项目被软删除后返回失败（错误文本 `ent: project not found`），与 `get_all_files`、`publish_version` 的可用性口径一致（审查 M2）。
- 客户端不再需要自己判断 `is_deleted`：查不到即视为不存在。
- 项目确实不存在时返回失败，错误文本是 ent 的原始信息。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |
| `ent: project not found` | 名称不存在 |

## 获取变更日志

| 项 | 值 |
| --- | --- |
| 方法 | `GET` |
| 路径 | `/api/project/get_project_change_logs/{projectName}` |
| 路径参数 | `projectName`：项目名称，需 URL 编码 |

### 请求示例

```bash
curl http://127.0.0.1:2000/api/project/get_project_change_logs/demo
```

### 响应

`data` 为 `ProjectChangeLog` 数组，按 `id` 倒序（最新版本在前）：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 3,
      "project_id": 1,
      "version": "1.0.1",
      "logs": ["修复 A", "修复 B"],
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

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int | 自增主键，同时也是排序依据 |
| `project_id` | int | 所属项目 ID |
| `version` | string | 该次发布的版本号 |
| `logs` | string[] | 变更说明列表 |
| `time` | string | 发布端传入的时间，`yyyy-MM-dd HH:mm:ss`；未传时为空字符串（响应中该键被省略） |
| `created_at` | string | 服务端写入时间，RFC 3339 |
| `after_apply_update_script` | string | 更新后脚本（相对应用目录）；为空时不出现 |
| `edges` | object | 恒为 `{}` |

### 行为说明

- **不校验项目是否存在**：项目名不存在时返回成功 + 空数组 `[]`。
- 项目被软删除后，日志仍可查到（只要日志自身未被软删除）；按名查询项目则会失败（见上文 `get_project_by_name` 的过滤口径）。
- 客户端与服务端都按**版本号**取最新（版本号相同再比 `id`，取更大者）；服务端发布时的防倒退校验同样按版本比较，不看插入顺序。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |

## 删除项目

软删除：置 `is_deleted = true`，不删除任何磁盘文件。

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/project/delete_project/{projectName}` |
| 路径参数 | `projectName`：项目名称，需 URL 编码 |
| 请求体 | 不使用（可发空对象） |

### 请求示例

```bash
curl -X POST http://127.0.0.1:2000/api/project/delete_project/demo
```

### 响应

```json
{ "isSuccess": true, "errorMsg": "", "data": null }
```

### 行为说明

- 只改数据库标志位；`data/{projectName}` 下的文件、`.updator_manifest.json` 全部保留。
- 删除前先释放该项目的文件列表缓存。
- 删除后：`get_all_projects` 不再返回；`update_project`、`set_force_update`、`publish_version`、`get_project_os_info` 均报「项目不存在」；`get_project_by_name` 也不再返回；变更日志仍可查。
- 同名项目无法重建（`create_project` 的唯一性校验不过滤软删除记录），需要先手工清理数据库记录。
- 重复删除返回 `项目不存在: demo`。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |
| `项目不存在: X` | 项目不存在或已软删除（影响行数为 0） |

## 发布版本

登记新版本、写入变更日志，并固化该项目的发布清单。

| 项 | 值 |
| --- | --- |
| 方法 | `POST` |
| 路径 | `/api/project/publish_version` |
| Content-Type | `application/json` |

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `projectName` | string | 是 | 项目名称，需存在且未软删除 |
| `version` | string | 是 | 新版本号，格式与长度见下 |
| `logs` | string[] | 否 | 变更说明列表，缺省空 |
| `time` | string | 否 | 发布时刻，`yyyy-MM-dd HH:mm:ss`；为空时不校验 |
| `afterApplyUpdateScript` | string | 否 | 更新后执行的脚本，相对应用目录；为空时不写 |

### 请求示例

```bash
curl -X POST http://127.0.0.1:2000/api/project/publish_version \
  -H "Content-Type: application/json" \
  -d '{
        "projectName": "demo",
        "version": "1.0.2",
        "logs": ["修复登录问题", "优化启动速度"],
        "time": "2026-11-10 11:30:00",
        "afterApplyUpdateScript": "run.bat"
      }'
```

### 响应

`data` 为新建的 `ProjectChangeLog`：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 4,
    "project_id": 1,
    "version": "1.0.2",
    "logs": ["修复登录问题", "优化启动速度"],
    "time": "2026-11-10 11:30:00",
    "created_at": "2026-11-10T11:30:02.2201145+08:00",
    "after_apply_update_script": "run.bat",
    "edges": {}
  }
}
```

字段说明同上文变更日志表。

### 版本号规则

| 约束 | 说明 |
| --- | --- |
| 非空 | 缺失或空串 → `版本号不能为空` |
| 长度 | 不超过 50 个字符 |
| 格式 | 正则 `^[vV]?\d+(\.\d+)*(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$` |
| 示例 | `1.0.0`、`2.1.0-beta`、`v1.2.3`、`2.1.2+build.4`、`2`（单段合法） |
| 非法示例 | `1.0.0 beta`（含空格）、`2026.10.31 15:00`、`release-1` |
| 严格递增 | 必须**大于**该项目所有未删除日志中的最高版本，相等也拒绝 |

比较口径（`CompareVersion`，与客户端一致）：

- 剥离 `v` / `V` 前缀；
- 按 `.` 分段做数值比较，缺失段按 `0`（`1.0` == `1.0.0`，`2` == `2.0.0`）；
- 预发布段：`1.0.0-beta` < `1.0.0`；数字段小于字母段（`1.0.0-1` < `1.0.0-alpha`）；数字段按数值（`beta.2` < `beta.10`）；段数多者大；
- 构建元数据（`+` 之后）忽略：`1.0.0+build.2` == `1.0.0`。

**第一个可发布的版本必须大于 `V1.0.0`**。创建项目时写入的初始版本是 `V1.0.0`，发布 `1.0.0` 会被判为"不大于最新版本"而拒绝；实际发布从 `1.0.1`（或更高）开始。

### 行为说明

1. 校验时间格式（非空时），格式错误直接拒绝。
2. 在同一事务里：取项目（未软删除）→ 收集该项目全部未删除日志、求最高版本做防倒退校验 → 更新 `projects.version` → 插入新日志。
3. 事务提交后写 `data/{projectName}/.updator_manifest.json`：把当前项目文件按 `get_all_files` 相同的过滤口径（跳过上传中间态、应用忽略规则、排除清单自身）固化为相对路径数组。
4. 清单写入失败只记服务端日志，**不影响发布结果**；此时客户端拿不到清单，会跳过删除同步（退化为不删文件）。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |
| `版本号不能为空` | `version` 缺失或为空 |
| `版本号长度不能超过50个字符` | 超长 |
| `版本号格式非法: X（应为数字版本号，如 1.0.0 / 2.1.0-beta / v1.2.3）` | 不匹配格式正则 |
| `时间格式错误，正确格式: yyyy-MM-dd HH:mm:ss` | `time` 非空但解析失败 |
| `ent: project not found` | 项目不存在或已软删除 |
| `版本 X 不大于已发布的最新版本 Y（防倒退保护；如需回退内容，请发布更高的版本号）` | 版本未严格递增 |

没有"允许降级"开关。内容确需回退时，按服务端建议发布一个更高的版本号。

## 获取项目所在服务器信息

返回服务端系统信息；磁盘用量按该项目目录所在分区统计。

| 项 | 值 |
| --- | --- |
| 方法 | `GET` |
| 路径 | `/api/project/get_project_os_info/{projectName}` |
| 路径参数 | `projectName`：项目名称，需 URL 编码 |

### 请求示例

```bash
curl http://127.0.0.1:2000/api/project/get_project_os_info/demo
```

### 响应

`data` 为 `ServerOSInfo` 数组，服务端固定返回一个元素：

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

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `os` | string | `runtime.GOOS`，如 `linux` / `windows` / `darwin` |
| `platform` | string | 发行版描述；平台信息取不到时为空字符串 |
| `goARCH` | string | `runtime.GOARCH` |
| `version` | string | Go 运行时版本，如 `go1.25.0` |
| `numCPU` | int | 逻辑 CPU 数 |
| `cpuName` | string | CPU 型号，取不到时为 `unknown` |
| `cpuMhz` | float64 | CPU 主频（MHz）；取不到时为 `0` |
| `diskUsed` | float64 | 已用空间（GB） |
| `diskFree` | float64 | 可用空间（GB） |
| `diskTotal` | float64 | 总空间（GB） |
| `diskUsedPercent` | float64 | 使用率（百分比，`0`–`100`） |

`os` / `goARCH` / `version` / `numCPU` 由 Go 运行时直接给出，稳定可得；`platform`、`cpuName`、`cpuMhz` 与磁盘四项依赖系统调用，在容器或裁剪系统中可能缺失（分别为空字符串、`unknown`、`0`、`0`）。

### 常见错误

| `errorMsg` | 触发条件 |
| --- | --- |
| `项目名称包含非法字符或长度不在1-64范围内` | 名称非法 |
| `项目不存在` | 项目不存在或已软删除 |

## 相关页面

- [API 总览](/docs/api)：信封、HTTP 200 错误语义、客户端调用链
- [文件接口](/docs/server/api-file)：上传、文件列表、发布清单、下载
- [数据模型](/docs/server/api-models)：`Project` / `ProjectChangeLog` 的库表与字段
- [部署 aly-server](/docs/server/deploy)：数据目录、备份、升级