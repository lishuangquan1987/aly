# Zap — 客户端自动更新系统

一个轻量级的客户端自动更新解决方案，包含四个模块：zap-server（服务端）、zap-publish（发布引擎，原 zap-publish）、ZapPublish（发布 GUI）、zap-update（终端更新器）。

## 项目结构

```
Zap/
├── server/                    # 服务端（Go 1.25 + Gin + Ent + SQLite）
├── client/                    # 客户端更新程序（Go 1.10，兼容 Windows XP）
├── publish/
│   ├── zap-publish-gui/       # 发布工具 GUI（Avalonia 12 桌面应用）
│   └── zap-publish/           # 发布工具 CLI（Go 1.25 + cobra，命令行发布）
└── README.md
```

## 整体架构

```
┌─────────────────────────────┐          ┌─────────────────────────────┐
│       发布者的工作站          │          │       终端用户的机器          │
│                             │          │                             │
│  ┌───────────┐              │          │                             │
│  │zap-publish-gui│ 只存项目列表  │          │  ┌─────────┐               │
│  │ (GUI壳子) │ 不调用API    │          │  │ client  │──→ 应用程序    │
│  └─────┬─────┘              │          │  │ (更新器) │   (被更新目标) │
│        │ 子进程调用          │          │  └────┬────┘               │
│        ▼                    │          │       │                   │
│  ┌───────────┐              │          │       │                   │
│  │zap-publish│──────────────┼──┐       └───────┼───────────────────┘
│  │ (发布引擎) │              │  │               │
│  └─────┬─────┘              │  │               │
│        │ 扫描               │  │               │
│        ▼                    │  │               │
│  ┌───────────┐              │  │               │
│  │ 本地构建   │              │  │               │
│  │ 产物目录   │              │  │               │
│  │  .updator/ │              │  │               │
│  └───────────┘              │  │               │
│                             │  │               │
└─────────────────────────────┘  │               │
                                 ▼               ▼
                          ┌───────────────────────────┐
                          │        server             │
                          │   项目管理 / 文件存储       │
                          │   版本记录 / 文件清单       │
                          └───────────────────────────┘
```

### 模块间通信关系

| 调用方 | 被调用方 | 通信方式 | 数据格式 |
|--------|---------|---------|---------|
| zap-publish-gui | zap-publish | 子进程（`ProcessService`） | stdout JSON（`--json` 参数） |
| zap-publish | server | HTTP REST API | JSON（camelCase 统一包装） |
| client | server | HTTP REST API | JSON（camelCase 统一包装） |

zap-publish-gui 本身 **不直接调用 server API**，所有发布操作都通过子进程调用 zap-publish 完成。zap-publish-gui 仅在本地维护一个项目列表配置（`%LOCALAPPDATA%/ZapPublish/config.json`）。

**唯一例外**：添加项目时，zap-publish-gui 调用 `zap-publish project list` / `zap-publish project create` 操作服务端项目；通过 `zap-publish config init` 在选中的本地路径初始化 `.updator/` 配置（类比 SourceTree 的"克隆/添加仓库"时初始化 .git）。

---

## `.updator/` 共享配置体系

zap-publish 和 client 共用一套配置，存放在构建产物目录的 `.updator/` 文件夹中（类似 `.git` 的概念）。

### 目录结构

```
构建产物目录/                    ← zap-publish 的 project_path
├── .updator/
│   ├── shared.json             # zap-publish + client 共用配置
│   ├── publish.json            # zap-publish 专有配置（本地，不上传）
│   └── staging/
│       └── staged-files.json   # 暂存区（本地，不上传）
├── app.exe
├── config.json
├── plugins/
│   └── core.dll
└── ...其他构建产物
```

### shared.json（两端共用）

```json
{
  "server_url": "http://10.0.0.1:2000",
  "project_name": "myapp",
  "ignore_folders": ["logs", "temp", ".git", ".updator"],
  "ignore_files": ["*.log", ".DS_Store"]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `server_url` | `string` | 服务端地址 |
| `project_name` | `string` | 项目名称，需与 server 一致（服务端保证唯一） |
| `ignore_folders` | `[]string` | 忽略的文件夹（路径前缀匹配） |
| `ignore_files` | `[]string` | 忽略的文件（支持 glob 通配符） |

### client.json（client 专有，部署时手动放置于 UpdateFolder）

```json
{
  "main_exe_relative_path": "../ApplicationFolder/main_application.exe",
  "must_close_process_name": ["main_application", "plugin_host"]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `main_exe_relative_path` | `string` | 主程序可执行文件的相对路径（相对于 UpdateFolder） |
| `must_close_process_name` | `[]string` | 更新前需关闭的进程名（不含 `.exe`） |

### publish.json（zap-publish 专有，本地不上传）

```json
{
  "output_format": "human"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `output_format` | `string` | 默认输出格式（`human` / `json`） |

> `project_id` 不需要记录，因为 `shared.json` 中已有 `project_name`，zap-publish 通过项目名称查找即可定位 server 上的项目。

### 哪些文件会上传到 server

zap-publish 在扫描文件时自动忽略 `.updator/` 目录。此外，以下文件不会出现在 server 的文件清单中：

- `.updator/` 整个目录（scanner 自动跳过）
- `.zap-publish/` 整个目录（向后兼容，scanner 自动跳过）

`shared.json` 会随构建产物一起上传到 server。当 client 下载更新时，新版本的 `shared.json` 也会被下载到 `ApplicationFolder/.updator/` 中，使客户端配置自动跟随版本更新。

> **注意**：`shared.json` 中的 `ignore_folders` / `ignore_files` 同时用于两个场景：
> 1. **zap-publish 扫描** — 决定哪些文件被扫描并上传到 server
> 2. **client apply_update** — `CopyDirWithExclude` 时决定哪些文件/文件夹不复制到版本目录

### client.json（精简版）

位于 `zap-update.exe` 同级目录（UpdateFolder），包含定位 ApplicationFolder 和关闭进程所需的字段：

```json
{
  "main_exe_relative_path": "../ApplicationFolder/main_application.exe",
  "must_close_process_name": ["main_application", "plugin_host"]
}
```

client 启动时的配置加载流程：
1. 读取 `client.json`（UpdateFolder）→ 获取 `main_exe_relative_path`、`must_close_process_name`
2. 通过 `main_exe_relative_path` 推导出 `ApplicationFolder` 路径
3. 读取 `ApplicationFolder/.updator/shared.json` → 获取 `server_url`、`project_name`、ignore 规则
4. 命令行参数覆盖以上所有配置

### 配置优先级

**zap-publish**：CLI 参数 > `.updator/shared.json` + `.updator/publish.json`

**client**：CLI 参数 > `ApplicationFolder/.updator/shared.json` > `client.json`

---

## 快速开始

### 1. 启动服务端

```bash
# Windows
zap-server-windows-amd64.exe -p 2000

# Linux / macOS
./zap-server-linux-amd64 -p 2000
```

首次启动会自动创建 SQLite 数据库（`configs/zap.db`）并建表。

### 2. 初始化项目并发布

```bash
# 1. 进入项目目录，初始化项目配置（创建 .updator/shared.json + .updator/publish.json）
cd ./dist
zap-publish config init \
  --server http://localhost:2000 \
  --project myapp

# 2. 查看差异
zap-publish status

# 3. 一键发布
zap-publish publish --version V1.0.1 --message "修复登录bug"
```

### 3. 配置客户端

在 `zap-update.exe` 同级目录创建 `client.json`（精简版）：

```yaml
main_exe_relative_path: "../ApplicationFolder/main_application.exe"
```

在 ApplicationFolder 中创建 `.updator/shared.json` 和 `.updator/client.json`（或通过 zap-publish 发布时自动带入）。

---

## 部署后的目录结构

```
PackageFolder/
├── ApplicationFolder/                  # 当前活跃的应用目录
│   ├── main_application.exe
│   ├── .updator/
│   │   ├── shared.json                 # 共用配置（来自 server）
│   │   └── client.json                 # client 专有配置（来自 server）
│   └── zap-update.exe               # updator 副本（用于自更新比对）
├── ApplicationFolder_V1.0.0/           # 历史版本备份（可用于回滚）
├── ApplicationFolder_V1.0.1/
└── UpdateFolder/                       # 更新程序目录
    ├── zap-update.exe
    ├── client.json                     # 精简配置（main_exe_relative_path + must_close_process_name）
    ├── version.json                    # 版本状态机
    └── logs/
```

| 目录/文件 | 归属 | 说明 |
|-----------|------|------|
| `ApplicationFolder/` | 构建产物 | 当前运行的应用程序文件 |
| `ApplicationFolder_{ver}/` | 运行时生成 | 版本备份，apply_update 时自动创建 |
| `UpdateFolder/` | 部署时手动放置 | zap-update.exe 及其配置 |
| `.updator/shared.json` | 构建产物 | 随版本更新自动同步 |
| `.updator/client.json` | 构建产物 | 随版本更新自动同步 |
| `version.json` | 运行时生成 | 版本状态机，记录当前版本和状态 |

---

## 服务端（server）

### 定位

纯后端 API 服务，不包含任何业务逻辑，只负责数据存储和文件管理。

### 数据模型

**Project**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `int` | 自增主键 |
| `name` | `string` | 项目名称（唯一，不可更改，服务端创建时校验重复） |
| `title` | `string` | 项目抬头 |
| `version` | `string` | 当前最新版本号 |
| `force_update` | `bool` | 是否强制更新 |
| `ignore_folders` | `[]string` | 获取文件清单时忽略的文件夹 |
| `ignore_files` | `[]string` | 获取文件清单时忽略的文件 |
| `created_at` | `time` | 创建时间 |
| `is_deleted` | `bool` | 软删除标记 |

**ProjectChangeLog**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `int` | 自增主键 |
| `version` | `string` | 版本号 |
| `logs` | `[]string` | 变更说明数组 |
| `time` | `string` | 发布时间（格式 `2006-01-02 15:04:05`） |
| `created_at` | `time` | 记录创建时间 |
| `is_deleted` | `bool` | 软删除标记 |

**FileInfo**（运行时计算，不持久化）

| 字段 | 类型 | 说明 |
|------|------|------|
| `fileAbsolutePath` | `string` | 服务端文件绝对路径 |
| `fileRelativePath` | `string` | 相对路径（正斜杠统一） |
| `lastUpdateTime` | `time` | 最后修改时间 |
| `fileSize` | `int64` | 文件大小（字节） |
| `md5` | `string` | MD5 校验值 |
| `sha256` | `string` | SHA256 校验值 |

### 文件存储

文件按 `{exe_dir}/data/{project_name}/` 目录组织，保持原始的相对路径结构。

### API 端点

| 方法 | 路径 | 说明 | 实现逻辑 |
|------|------|------|---------|
| `POST` | `/api/project/create_project` | 创建项目 | 验证 name/title 非空 → 检查 name 唯一 → 创建 `data/{name}/` 目录 → 事务插入 Project（version=V1.0.0）+ ProjectChangeLog（"第一次创建"） |
| `POST` | `/api/project/update_project` | 更新项目配置 | 按 ID 查找 → 更新 title/force_update/ignore_folders/ignore_files |
| `GET` | `/api/project/get_all_projects` | 获取所有项目 | 查询 `is_deleted=false` 的所有 Project 记录 |
| `POST` | `/api/project/delete_project/:projectName` | 删除项目 | 软删除（设置 `is_deleted=true`） |
| `GET` | `/api/project/get_project_change_logs/:projectName` | 获取变更日志 | 按 Project 名称关联查询，按 ID 倒序 |
| `POST` | `/api/project/publish_version` | 发布新版本 | 事务：更新 `Project.Version` + 创建 `ProjectChangeLog` |
| `GET` | `/api/project/get_project_os_info/:projectName` | 获取服务器信息 | 采集 OS/CPU/磁盘信息（`gopsutil`） |
| `POST` | `/api/file/upload_file` | 上传文件 | multipart 接收 → 路径穿越防护（`filepath.Clean` + 前缀检查）→ 存储到 `data/{project}/{relativePath}` |
| `GET` | `/api/file/get_all_files/:projectName` | 获取文件清单 | 递归扫描 `data/{project}/` → 应用 ignore 规则 → 计算 MD5+SHA256 → 返回 FileInfo 数组 |
| `GET` | `/api/file/download_file?path=` | 下载文件 | 路径穿越防护 → `http.ServeContent`（自动支持 Range 请求/断点续传） |

### 统一响应格式

所有 API 响应使用统一的 JSON 包装（camelCase）：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": { ... }
}
```

---

## 发布引擎（zap-publish）

### 定位

类 Git 工作流的命令行工具，是整个发布流程的核心引擎。zap-publish-gui 通过子进程调用它完成所有发布操作。

### 构建

```bash
cd publish/zap-publish
go build -ldflags="-s -w" -o zap-publish.exe .
```

### 全局参数

所有命令都支持以下全局参数：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--server` | 服务器地址 | 配置文件 `server_url` |
| `--project` | 项目名称 | 配置文件 `project_name` |

| `--id` | 项目 ID（直传，跳过名称查找） | 无（纯 CLI 参数） |
| `--json` | JSON 格式输出 | `false`（默认人类可读） |
| `--quiet` | 静默模式 | `false` |

### 配置加载流程

`resolveConfig()` 函数按以下优先级合并配置：

1. **CLI 参数**（`--server`、`--project`、`--id`）
2. **项目级配置**（`.updator/shared.json` + `.updator/publish.json`，从当前工作目录读取）

合并规则：非零/非空值覆盖（CLI 参数 > `.updator/`）。

### JSON 输出格式

所有命令在 `--json` 模式下输出统一格式：

```json
{"isSuccess": true, "errorMsg": "", "data": { ... }}
```

---

### config — 配置管理

#### `config init`

初始化项目配置，创建 `.updator/shared.json` 和 `.updator/publish.json`。

```bash
cd ./dist
zap-publish config init \
  --server http://localhost:2000 \
  --project myapp
```

**实现**：
1. 调用 `resolveConfig()` 合并参数
2. 调用 `config.SaveProject()` 写入 `.updator/shared.json`（server_url、project_name、ignore 规则）和 `.updator/publish.json`（output_format）

#### `config set <key> <value>`

设置标量配置项。

```bash
zap-publish config set server.url http://deploy.example.com
zap-publish config set output.format json
```

**实现**：解析 key-value → `resolveConfig()` → `applyConfigSet()` 修改对应字段 → 保存到 `.updator/` 下的 JSON 文件。

支持的 key：`server.url`、`project.name`、`output.format`

#### `config set-array <key> --add/--remove/--clear`

操作数组配置项。

```bash
zap-publish config set-array ignore.folders --add ".cache"
zap-publish config set-array ignore.folders --remove ".cache"
zap-publish config set-array ignore.folders --clear
```

**实现**：解析操作参数 → `resolveConfig()` → `applyArrayOp()` 执行添加/移除/清空 → 保存。

支持的 key：`ignore.folders`、`ignore.files`

#### `config get <key>`

获取配置值。

```bash
zap-publish config get server.url
```

**实现**：`resolveConfig()` → `getConfigValue()` 按 key 读取 → 输出。

#### `config list`

列出全部生效配置。

```bash
zap-publish config list
```

**实现**：`resolveConfig()` → 按行输出所有配置项。

#### `config path`

显示配置文件路径。

```bash
zap-publish config path
```

**实现**：输出当前工作目录下的 `.updator/` 路径。

---

### project — 项目管理

#### `project list`

列出所有未删除的项目。

```bash
zap-publish project list --server http://localhost:2000
```

**实现**：`api.GetAllProjects()` → `GET /api/project/get_all_projects` → 格式化输出（ID、Name、Title、Version、ForceUpdate、CreatedAt）。

#### `project create`

创建新项目（自动创建初始版本 V1.0.0）。

```bash
zap-publish project create \
  --server http://localhost:2000 \
  --name myapp \
  --title "我的应用" \
  --force-update \
  --ignore-folders ".git,node_modules" \
  --ignore-files "*.log,*.tmp"
```

**实现**：解析参数 → `api.CreateProject()` → `POST /api/project/create_project`（服务端事务创建 Project + ProjectChangeLog）→ 输出项目信息。

#### `project update`

更新项目配置。

```bash
zap-publish project update --name myapp --title "新标题" --force-update
```

**实现**：`api.UpdateProject()` → `POST /api/project/update_project`。

#### `project delete`

软删除项目。

```bash
zap-publish project delete --name myapp
```

**实现**：`api.DeleteProject()` → `POST /api/project/delete_project/{id}`。

#### `project info`

查看项目详情。

```bash
zap-publish project info --name myapp
```

**实现**：`api.GetAllProjects()` → 按 ID 过滤 → 输出详细信息（Name、Title、Version、ForceUpdate、IgnoreFolders、IgnoreFiles）。

---

### status / diff — 文件差异对比

#### `status`

查看本地与服务端的文件差异，分为 staged（已暂存）、unstaged（未暂存）、unchanged（无变化）三组。

```bash
zap-publish status --project myapp
```

**实现**：
1. `resolveConfig()` 获取配置
2. `diff.RunStatus()` 执行核心比对：
   - `diff.ScanDirectory()` 递归扫描本地目录（自动跳过 `.updator/` 和 `.zap-publish/`，应用 ignore 规则），对每个文件同时计算 MD5 和 SHA256
   - `api.GetAllFiles()` 从 server 拉取文件清单（`GET /api/file/get_all_files/{projectName}`，server 端也应用 ignore 规则）
   - `diff.Diff()` 按相对路径做 diff：本地有服务端没有 → `new`，MD5 不同 → `modified`，服务端有本地没有 → `deleted`，MD5 相同 → `unchanged`
3. 加载暂存区 `.updator/staging/staged-files.json`
4. 合并：已暂存的文件从 unstaged 移到 staged
5. 格式化输出（类 Git 风格）

#### `diff`

详细对比文件差异。

```bash
zap-publish diff --file app.exe
```

**实现**：与 status 相同的核心比对流程 → 逐文件输出 local vs server 的 MD5/Size 信息。

---

### add / reset / staged — 暂存区管理

暂存区借鉴 Git 概念，文件存储在 `.updator/staging/staged-files.json`。

#### `add [--all | <file>...]`

将文件加入暂存区。

```bash
zap-publish add app.exe lib.dll    # 添加指定文件
zap-publish add --all              # 添加所有 new + modified 文件
```

**实现**：
- `add --all`：先调 `diff.RunStatus()` 获取 unstaged 中 status 为 `new` 或 `modified` 的文件 → `staging.AddWithStatus()` 将每个文件的相对路径、MD5、Size、Status 写入暂存区
- `add <file>...`：直接 `staging.Add()` 计算每个文件的 MD5 并写入暂存区

暂存区的 `staging.AddWithStatus()` 逻辑：
1. 读取现有暂存区
2. 对新文件计算 MD5（`diff.HashFile()`）
3. 记录 `relativePath`、`status`（new/modified）、`localMd5`、`localSize`
4. 已存在的文件不会重复添加
5. 写回 `.updator/staging/staged-files.json`

#### `reset [--all | <file>...]`

从暂存区移除文件。

```bash
zap-publish reset app.exe     # 移除指定文件
zap-publish reset --all       # 清空暂存区
```

**实现**：读取暂存区 → 过滤掉指定文件 → 写回。`--all` 直接写入空数组。

#### `staged`

查看暂存区内容。

```bash
zap-publish staged
```

**实现**：读取 `.updator/staging/staged-files.json` → 格式化输出每个文件的 Status、RelativePath、LocalSize。

---

### push / push-all / publish — 版本发布

#### `push`

推送暂存区文件到 server。

```bash
zap-publish push --version V1.0.1 --message "修复登录bug" --message "新增搜索"
```

**两阶段实现**：

**阶段 1 — 校验与上传**：
1. 加载暂存区文件列表
2. `staging.Verify()` 重新计算每个暂存文件的当前 MD5，与 add 时记录的 MD5 比对。不一致的文件列为冲突，拒绝推送（使用 `--force` 可跳过此检查）
3. 逐文件上传：`POST /api/file/upload_file`（multipart，字段包含 `file`、`projectName`、`relativeFileName`），任一失败即停止，暂存区保留

**阶段 2 — 创建版本记录**：
1. `POST /api/project/publish_version`（参数：`projectName`、`version`、`logs`、`time`）
2. 服务端事务更新 `Project.Version` + 创建 `ProjectChangeLog`
3. 成功后清空暂存区

> ⚠️ **非原子操作**：文件上传和版本创建是两个独立 HTTP 请求。如果阶段 1 成功但阶段 2 失败，文件已在 server 但无版本记录。恢复方式：重新执行 `publish`（已上传文件秒传）。

| 参数 | 说明 | 必填 |
|------|------|------|
| `--version` | 新版本号 | ✅ |
| `--message` | 变更说明（可多次指定） | ✅（至少一条） |
| `--dry-run` | 仅校验不实际推送 | ❌ |
| `--force` | 跳过 MD5 复核 | ❌ |

#### `push-all`

跳过暂存区，直接推送所有变更文件。

```bash
zap-publish push-all --version V1.0.1 --message "快速修复"
```

**实现**：`diff.RunStatus()` 获取 unstaged 中 `new` + `modified` 的文件 → 直接调 `pushFiles()`（跳过暂存区）。

#### `publish`

一键完成完整发布流程。

```bash
zap-publish publish --version V1.0.1 --message "日常发布"
```

**实现**：等价于 `status` → `add --all` → `push`：
1. `diff.RunStatus()` 获取所有变更文件
2. `staging.Add()` 暂存所有 `new` + `modified` 文件
3. `pushFiles()` 上传 + 创建版本记录
4. 成功后清空暂存区

---

### log — 版本历史

```bash
zap-publish log --project myapp --limit 5
```

**实现**：
1. `resolveProjectID()` 获取项目 ID
2. `api.GetProjectChangeLogs()` → `GET /api/project/get_project_change_logs/{projectName}`
3. 按 ID 倒序排列（最新在前）
4. 截取 `--limit` 条（默认 20）
5. 格式化输出：`V1.0.3 (2024-01-15 10:00:00)  • 优化性能`

---

### watch — 实时监控

轮询监控本地目录文件变更。

```bash
zap-publish watch --interval 5 --auto-add
```

**实现**：
1. 首次扫描 `diff.ScanDirectory()` 建立文件快照（path + MD5）
2. 每隔 `--interval` 秒（默认 2）重新扫描
3. `detectChanges()` 比对前后快照：新增 → `new`，MD5 变化 → `modified`，消失 → `deleted`
4. `--auto-add` 时，自动将 `new` + `modified` 的文件加入暂存区
5. 按 `Ctrl+C` 退出

---

### server info — 服务器信息

```bash
zap-publish server info --project myapp
```

**实现**：`api.GetProjectOSInfo()` → `GET /api/project/get_project_os_info/{projectName}` → 输出 OS、Platform、Architecture、CPU、磁盘等信息。

---

### 输出格式

**人类可读**（默认）：格式化终端输出。

**JSON**（`--json`）：统一 camelCase 包装：

```json
{"isSuccess": true, "errorMsg": "", "data": { ... }}
```

| 退出码 | 含义 |
|--------|------|
| `0` | 成功 |
| `1` | 一般错误（参数错误、网络不可达等） |

---

## 客户端更新器（client）

### 定位

集成到最终用户应用程序中，负责检测更新、下载更新、原子替换应用文件。使用 Go 1.10 编译以兼容 Windows XP。

### 构建

需要 Go 1.10（兼容 Windows XP），不使用 Go Modules：

```bat
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o zap-update.exe zap/client
```

### 配置加载流程

所有命令启动时的配置加载顺序：

1. 读取 `client.json`（与 `zap-update.exe` 同级目录）→ 获取 `main_exe_relative_path`、`must_close_process_name`
2. 通过 `main_exe_relative_path` 推导出 `ApplicationFolder` 路径（`filepath.Join(exeDir, main_exe_relative_path)` 的父目录）
3. 读取 `ApplicationFolder/.updator/shared.json` → 获取 `server_url`、`project_name`、ignore 规则
4. 命令行参数覆盖以上所有配置（`--url`、`--project-name`、`--main-exe-path`）

### JSON 输出格式

所有命令输出 JSON 到 stdout（与 zap-publish 格式统一）：

```json
{"isSuccess": true, "errorMsg": "", "data": { ... }}
```

---

### check_update — 检查更新

```bash
zap-update.exe check_update [--url <url>] [--project-name <name>]
```

**实现**：
1. 加载配置 + 读取 `version.json`（UpdateFolder 下）
2. **如果 `version_status` 为 `downloaded` 或 `applying`**：说明有未完成的更新 → 直接返回 `has_update=true`（current_version = 上一版本，new_version = 已下载的版本）
3. 调 `FindProjectByName()` → `GET /api/project/get_project_by_name/{projectName}` → 获取 `project.force_update`
4. 调 `GetProjectChangeLogs()` → `GET /api/project/get_project_change_logs/{projectName}` → 取 ID 最大的日志 → 获取 `latestVersion`
5. `compareVersion(serverVersion, localVersion)` 逐段数值比较（按 `.` 分割，每段转 int 比较）
6. 服务端版本 > 本地版本 → 返回 `has_update=true` + `force_update`；否则返回 `has_update=false`

**返回 data 结构**：
```json
{
  "has_update": true,
  "current_version": "1.0.0",
  "new_version": "1.0.1",
  "force_update": true
}
```

---

### check_diff — 文件比对

```bash
zap-update.exe check_diff [--url <url>] [--project-name <name>]
```

**实现**：
1. `FindProjectByName()` 获取项目 ID
2. `GetAllFiles()` → `GET /api/file/get_all_files/{id}` 获取服务端文件清单（每个文件有 relativePath、MD5、SHA256、fileSize）
3. `util.LocalFileMD5Map(mainFolder)` 递归扫描 ApplicationFolder，对每个文件计算 MD5，返回 `{relativePath: md5}` 映射
4. 逐项比对：
   - 本地不存在该文件 → 差异（local_md5 为空）
   - MD5 不同 → 差异（同时输出 local 和 server 的 MD5、Size、SHA256）
   - MD5 相同 → 跳过
5. 输出差异文件列表

**返回 data 结构**：
```json
{
  "new_version": "1.0.1",
  "files": [
    {
      "path": "app.exe",
      "local_md5": "abc...",
      "local_size": 1024000,
      "local_sha256": "def...",
      "server_md5": "xyz...",
      "server_size": 1025000,
      "server_sha256": "uvw..."
    }
  ]
}
```

---

### download_update — 下载更新

```bash
zap-update.exe download_update [--url <url>] [--project-name <name>]
```

**实现**：
1. 通过 `GetProjectChangeLogs()` 获取最新版本号（`download_update` 不需调用 `FindProjectByName`，直接用项目名称查日志）
2. 与 `version.json` 中的当前版本比较，如果已是最新 → 返回错误
3. 创建目标目录 `{main_exe_folder_name}_{new_version}/`（如 `ApplicationFolder_1.0.1/`）
4. `GetAllFiles()` 获取服务端文件清单
5. `util.LocalFileMD5Map(mainFolder)` 扫描本地 ApplicationFolder 的 MD5
6. 比对差异文件（本地不存在或 MD5 不同的文件）
7. 逐文件下载：
   - **已有且校验通过**：跳过（支持断点续传场景下已完成的文件）
   - **大文件（>100MB）**：先检查 `.part` 文件 → 如果存在且大小 < 服务端大小 → 用 `Range: bytes={offset}-` 请求续传 → 下载完成后 rename 为正式文件
   - **普通文件**：直接下载到 `.part` 文件 → rename 为正式文件
   - **校验**：下载后计算 MD5 + SHA256，与 server 记录比对。不一致 → 删除文件重试（最多 3 次）
8. 全部文件下载完成 → 更新 `version.json`：
   ```json
   {
     "version_previouse": "1.0.0",
     "version": "1.0.1",
     "version_status": "downloaded",
     "after_apply_update_script": "post_update.bat"
   }
   ```
   其中 `after_apply_update_script` 从服务端变更日志同步（发布时指定）
9. 如果 `version.json` 已经是 `downloaded` 且版本相同 → 跳过版本更新（幂等）

**返回 data 结构**：
```json
null
```

---

### apply_update — 应用更新（原子替换）

```bash
zap-update.exe apply_update [--main-exe-path <path>] [--close-timeout <seconds>]
```

**状态机实现**：

**1. 检查 `version_status`**：

| 状态 | 处理 |
|------|------|
| `applied` | 无待更新 → 返回"no pending update" |
| `downloaded` | 正常流程 ↓ |
| `applying` | 崩溃恢复：检查 mainFolder 是否存在 → 存在则继续正常流程 → 不存在则从 versionDir rename 恢复 |

**2. 设置 `version_status = "applying"`**（写入 `version.json`，标记原子操作开始）

**3. 关闭目标进程**：
- 从 `client.json` 读取 `must_close_process_name`，对列表中的每个进程名：
  - 先发送 `WM_CLOSE` 消息（优雅关闭）
  - 超时后 `TerminateProcess`（强制终止）
- 超时时间由 `--close-timeout` 控制（默认 30 秒）

**4. 原子文件夹替换**：
- **Step A**：`CopyDirWithExclude(mainFolder, versionDir)` — 将当前 ApplicationFolder 的内容复制到 versionDir（排除 `shared.json` 中 `ignore_folders` 和 `ignore_files` 匹配的文件/文件夹）。这确保 versionDir 是一个完整的可运行应用（因为 download_update 只下载了变更文件）
- **Step B**：将旧备份目录（`ApplicationFolder_{oldVersion}`）临时移走为 `.old` 后缀（防止断电丢失）
- **Step C**：`os.Rename(mainFolder, prevVersionDir)` — 将当前应用目录备份为 `ApplicationFolder_{oldVersion}/`
- **Step D**：`os.Rename(versionDir, mainFolder)` — 将新版目录激活为 ApplicationFolder
- **Step E**：删除临时的 `.old` 备份
- 任何步骤失败都有回滚逻辑

**5. 设置 `version_status = "applied"`**

**6. 执行后更新脚本**：从 `version.json` 的 `after_apply_update_script` 字段读取脚本路径（该字段在 `download_update` 时从服务端变更日志同步），异步执行，失败仅记日志到 `update.log`

**7. 启动主程序**（`filepath.Join(exeDir, main_exe_relative_path)`）

**返回**：`{"isSuccess": true, "data": null}`

---

### list_rollback_versions — 列出可回滚版本

```bash
zap-update.exe list_rollback_versions [--main-exe-path <path>]
```

**实现**：
1. 获取 `PackageDir/`（UpdateFolder 的父目录）
2. 获取 `main_exe_folder_name`（如 `ApplicationFolder`）
3. 扫描 `PackageDir/` 下所有匹配 `{folder_name}_*` 的目录
4. 验证后缀是合法的版本号格式（如 `1.0.0`）
5. 输出当前版本 + 可回滚版本列表

**返回 data 结构**：
```json
{
  "current_version": "1.0.1",
  "versions": ["1.0.0"]
}
```

---

### rollback — 版本回滚

```bash
zap-update.exe rollback --version 1.0.0 [--main-exe-path <path>] [--close-timeout <seconds>]
```

**实现**：
1. 验证目标版本目录 `ApplicationFolder_{version}` 存在
2. 检查 `version_status`：`applying` → 崩溃恢复逻辑
3. 设置 `version_status = "applying"`
4. 关闭目标进程（同 apply_update）
5. **与 apply_update 的区别**：rollback 的目标版本目录是之前 apply 时保存的完整快照，不需要 `CopyDirWithExclude` 补全文件
6. 旧备份临时移走 → `mainFolder → prevVersionDir`（备份当前版本）→ `versionDir → mainFolder`（激活目标版本）
7. 更新 `version.json`（`version` = 目标版本，`version_previouse` = 当前版本，`version_status` = `applied`）
8. 执行 `version.json` 中的 `after_apply_update_script`（异步）→ 启动主程序

**返回 data 结构**：
```json
{ "version": "1.0.0" }
```

---

### check_self_update — 更新程序自检

```bash
zap-update.exe check_self_update [--main-exe-path <path>]
```

**实现**：
1. 计算自身文件（`zap-update.exe`）的 SHA256
2. 检查 `ApplicationFolder/zap-update.exe` 是否存在（不存在 → 首次部署，无需自更新）
3. 计算 `zap-update.exe` 的 SHA256
4. 比较两个 SHA256 → 不一致则 `need_update=true`

**返回 data 结构**：
```json
{ "need_update": true }
```

---

### version.json 状态机

version.json 位于 UpdateFolder（`zap-update.exe` 同级目录）：

```json
{
  "version_previouse": "1.0.0",
  "version": "1.0.1",
  "version_status": "applied",
  "after_apply_update_script": "post_update.bat"
}
```

**状态流转**：

```
applied ──→ (check_update 发现新版本) ──→ [无变化]
    │
    ├──→ (download_update 成功) ──→ downloaded
    │                                    │
    │                                    ├──→ (apply_update 开始) ──→ applying
    │                                    │                                │
    │                                    │     ┌──────────────────────────┘
    │                                    │     │
    │                                    │     ├──→ (apply_update 成功) ──→ applied
    │                                    │     │
    │                                    │     └──→ (崩溃/断电) ──→ 下次启动时
    │                                    │         检测到 applying 状态
    │                                    │         → 自动恢复
    │                                    │
    │                                    └──→ (下载后不 apply，再次 check_update)
    │                                         → 检测到 downloaded 状态
    │                                         → 直接返回 has_update=true
    │
    └──→ (rollback 成功) ──→ applied（version 回退到旧版）
```

**崩溃恢复逻辑**：
- 当检测到 `version_status = "applying"` 时：
  - 如果 ApplicationFolder 存在 → 说明 rename 未完成，继续执行后续 rename 步骤
  - 如果 ApplicationFolder 不存在 → 检查 versionDir → 存在则 rename 为 ApplicationFolder → 设为 `applied`
  - 两种恢复路径都会执行 `after_apply_update_script` 并启动主程序

---

## 发布 GUI（zap-publish-gui）

### 定位

面向发布者的桌面工具（Avalonia 12），纯前端壳子，所有发布操作通过子进程调用 zap-publish 完成。

### 构建

```bash
cd publish/zap-publish-gui
dotnet build
```

### 三个 Service 的职责

**ConfigService** — 本地项目列表管理

- 存储位置：`%LOCALAPPDATA%/ZapPublish/config.json`
- 数据结构：`List<ProjectConfig>`（每个 ProjectConfig 包含 `DisplayName`、`ProjectPath`——类比 SourceTree 的书签，只记别名+路径，真实配置来自项目的 `.updator/`）
- 方法：`LoadProjects()`、`SaveProjects()`、`AddProject()`、`RemoveProject()`、`UpdateProject()`

**ProcessService** — 子进程执行引擎

- `RunAsync(fileName, arguments, workingDir, timeoutMs)` → 启动子进程 → 捕获 stdout/stderr → 超时控制（默认 30s）→ 返回 `ProcessResult`（Success、StandardOutput、StandardError、ExitCode）

**CliService** — zap-publish 命令封装

- 自动查找 `zap-publish.exe`（同级目录 → 相对路径回退）
- `RunAsync<T>(args, projectPath, timeoutMs)` → 自动追加 `--json` → 设 `WorkingDirectory` 为项目路径 → 调 ProcessService → 解析 stdout JSON 为 `CliOutput<T>`
- 每个公开方法对应一个 CLI 命令：

### UI 操作 → CLI 命令映射

| UI 操作 | ViewModel 方法 | 调用的 CLI 命令 |
|---------|---------------|----------------|
| **刷新** | `RefreshAsync()` | `zap-publish status --json` + `zap-publish log --limit 20 --json` |
| **全部暂存** | `AddAllAsync()` | `zap-publish add --all --json` |
| **清空暂存** | `ResetAllAsync()` | `zap-publish reset --all --json` |
| **暂存选中文件** | `AddSelectedAsync()` | `zap-publish add "file1" "file2" ... --json` |
| **取消暂存选中** | `ResetSelectedAsync()` | 先 `zap-publish reset --all --json`，再 `zap-publish add "保留文件1" ... --json` |
| **发布** | `PublishAsync()` | `zap-publish push --version "{ver}" --message "{msg}" --json`（timeout 120s） |
| **添加项目** | `AddProjectAsync()` | 打开 AddProjectDialog → 填写服务端地址、选择/创建项目、选择本地路径 → 返回 ProjectConfig（仅 DisplayName + ProjectPath） |
| **移除项目** | `RemoveProjectAsync()` | 仅操作本地 ConfigService，不调用 CLI |

### 发布流程（GUI 用户视角）

1. 用户启动 zap-publish-gui → 加载 `%LOCALAPPDATA%/ZapPublish/config.json` 中的项目列表
2. 选择项目 → 自动调 `status` + `log` 刷新数据
3. 左侧显示 unstaged 文件列表（new/modified/deleted）
4. 右侧显示 staged 文件列表
5. 用户勾选文件 → 点击"暂存选中" 或 "全部暂存"
6. 输入版本号和变更说明 → 点击"发布"
7. GUI 调 `push` → zap-publish 上传文件 + 创建版本 → 清空暂存区 → 自动刷新

---

## 应用程序集成 client 的完整流程

### 端到端更新时序

```
应用程序启动
│
├─ Step 1: check_self_update
│  └─ need_update = true?
│     ├─ 是 → 用新版本替换自身 → 重启更新器
│     └─ 否 → 继续
│
├─ Step 2: check_update
│  └─ has_update = true?
│     ├─ 否 → 正常启动主程序
│     └─ 是 →
│        ├─ force_update = true  → 自动进入 Step 3
│        └─ force_update = false → 弹窗提示用户"发现新版本 X.X.X，是否更新？"
│                                  ├─ 用户确认 → 进入 Step 3
│                                  └─ 用户取消 → 跳过更新，正常启动
│
├─ Step 3: download_update
│  └─ 下载差异文件（显示进度）
│     ├─ 成功 → 进入 Step 4
│     └─ 失败 → 提示用户，可重试或跳过
│
├─ Step 4: apply_update
│  └─ 关闭当前进程 → 原子替换 → 重启应用程序
│
└─ 异常恢复: rollback --version X.X.X
   └─ 回退到指定版本
```

### C# 集成示例

```csharp
using System.Diagnostics;
using System.Text.Json;

public class UpdateManager
{
    private readonly string _updatorPath; // zap-update.exe 的路径

    public UpdateManager(string updatorPath)
    {
        _updatorPath = updatorPath;
    }

    /// <summary>
    /// 检查是否有更新
    /// </summary>
    public async Task<(bool hasUpdate, string? newVersion, bool forceUpdate)> CheckUpdateAsync()
    {
        var result = await RunUpdatorAsync("check_update");
        if (!result.IsSuccess) return (false, null, false);

        var data = result.Data;
        bool hasUpdate = data.GetProperty("has_update").GetBoolean();
        string? newVersion = hasUpdate ? data.GetProperty("new_version").GetString() : null;
        bool forceUpdate = data.TryGetProperty("force_update", out var fu) && fu.GetBoolean();

        return (hasUpdate, newVersion, forceUpdate);
    }

    /// <summary>
    /// 下载更新
    /// </summary>
    public async Task<bool> DownloadUpdateAsync()
    {
        var result = await RunUpdatorAsync("download_update");
        return result.IsSuccess;
    }

    /// <summary>
    /// 应用更新（会关闭当前进程并重启）
    /// </summary>
    public async Task<bool> ApplyUpdateAsync()
    {
        var result = await RunUpdatorAsync("apply_update");
        return result.IsSuccess;
    }

    /// <summary>
    /// 回滚到指定版本
    /// </summary>
    public async Task<bool> RollbackAsync(string version)
    {
        var result = await RunUpdatorAsync($"rollback --version {version}");
        return result.IsSuccess;
    }

    private async Task<UpdatorResult> RunUpdatorAsync(string arguments)
    {
        var psi = new ProcessStartInfo
        {
            FileName = _updatorPath,
            Arguments = arguments,
            UseShellExecute = false,
            RedirectStandardOutput = true,
            CreateNoWindow = true
        };

        using var process = Process.Start(psi);
        var output = await process.StandardOutput.ReadToEndAsync();
        await process.WaitForExitAsync();

        using var doc = JsonDocument.Parse(output);
        var root = doc.RootElement;

        return new UpdatorResult
        {
            IsSuccess = root.GetProperty("isSuccess").GetBoolean(),
            ErrorMsg = root.GetProperty("errorMsg").GetString(),
            Data = root.TryGetProperty("data", out var d) ? d : default
        };
    }
}

/// <summary>
/// 应用启动时的更新检查逻辑
/// </summary>
public static class AppStartup
{
    public static async Task CheckAndApplyUpdates()
    {
        var updater = new UpdateManager("UpdateFolder/zap-update.exe");

        // Step 1: 检查自更新
        // (通常不需要，除非更新器自身需要升级)

        // Step 2: 检查更新
        var (hasUpdate, newVersion, forceUpdate) = await updater.CheckUpdateAsync();
        if (!hasUpdate) return;

        // Step 3: 用户确认（非强制更新时）
        if (!forceUpdate)
        {
            bool confirmed = ShowUpdateDialog(newVersion!);
            if (!confirmed) return;
        }

        // Step 4: 下载
        bool downloaded = await updater.DownloadUpdateAsync();
        if (!downloaded)
        {
            ShowError("更新下载失败");
            return;
        }

        // Step 5: 应用（会关闭当前进程）
        await updater.ApplyUpdateAsync();
        // apply_update 成功后会自动重启主程序，执行到此处说明失败
    }
}
```

### 完整调用链示例

```bash
# 1. 检查自更新
zap-update.exe check_self_update
# → {"isSuccess":true,"errorMsg":"","data":{"need_update":false}}

# 2. 检查更新
zap-update.exe check_update
# → {"isSuccess":true,"errorMsg":"","data":{"has_update":true,"current_version":"1.0.0","new_version":"1.0.1","force_update":false}}

# 3. 下载更新
zap-update.exe download_update
# → {"isSuccess":true,"errorMsg":"","data":null}

# 4. 应用更新（会重启应用）
zap-update.exe apply_update --close-timeout 30
# → {"isSuccess":true,"errorMsg":"","data":null}

# 异常时回滚
zap-update.exe rollback --version 1.0.0
# → {"isSuccess":true,"errorMsg":"","data":{"version":"1.0.0"}}
```

---

## CI/CD 集成示例

```bash
#!/bin/bash
# 构建 → 发布

npm run build

zap-publish config set server.url $DEPLOY_SERVER
zap-publish config set project.name $PROJECT_NAME

cd ./dist && zap-publish publish \
  --version $BUILD_VERSION \
  --message "CI Build #${BUILD_NUMBER}"

if [ $? -ne 0 ]; then
  echo "发布失败"
  exit 1
fi
```

---

## 从源码构建

### 服务端

```bash
cd server
go build -ldflags="-s -w" -o zap-server.exe .
```

交叉编译（`CGO_ENABLED=0` 纯静态编译）：

```bash
cd server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o zap-server-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o zap-server-linux-arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o zap-server-darwin-amd64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o zap-server-darwin-arm64 .
```

### zap-publish

```bash
cd publish/zap-publish
go build -ldflags="-s -w" -o zap-publish.exe .
```

交叉编译（`CGO_ENABLED=0`）：

```bash
cd publish/zap-publish
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o zap-publish-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o zap-publish-linux-arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o zap-publish-darwin-amd64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o zap-publish-darwin-arm64 .
```

### zap-publish-gui

```bash
cd publish/zap-publish-gui
dotnet build
```

### 客户端

需要 Go 1.10（兼容 Windows XP），不使用 Go Modules：

```bat
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go build -ldflags="-s -w" -o zap-update.exe zap/client
```

---

## License

MIT
