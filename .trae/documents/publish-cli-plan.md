# Publish CLI 设计文档

## 一、概述

### 1.1 定位

命令行发布工具，面向**发布者**，用于将本地构建产物推送到服务端，管理版本发布流程。

### 1.2 与现有工具的关系

| 工具 | 语言 | 面向用户 | 职责 |
|------|------|---------|------|
| **publish-cli** | Go | 发布者 | 管理项目、上传文件、发布新版本 |
| **publish_tool_avalonia** | C# | 发布者 | GUI 发布管理工具 |
| **client** | Go | 终端用户 | 检查更新、下载更新、安装更新 |

### 1.3 核心能力

- 项目管理（创建、更新、删除、查看）
- 本地与服务端文件差异对比（基于 MD5 / SHA256）
- 暂存区管理（类似 git add/reset）
- 文件上传（multipart）与版本发布
- 版本历史查看
- 服务器信息查看

### 1.4 设计理念：类 Git 工作流

publish-cli 借鉴 Git 的 **working tree → staging area → remote** 三层模型，将发布流程映射为开发者熟悉的操作：

| Git 概念 | publish-cli 对应 | 实现方式 |
|----------|-----------------|---------|
| working tree | 本地构建产物目录 (`--path`) | 递归扫描目录，计算文件 MD5/SHA256 |
| staging area | `.publish-cli/staging/staged-files.json` | 本地 JSON 文件，记录待上传文件列表 + MD5 |
| remote (origin) | 服务端 `data/<projectName>/` | `GET /api/file/get_all_files/{id}` 获取远端文件清单 |
| `git status` | `publish-cli status` | 本地 vs 远端文件差异（按相对路径 + MD5 比对） |
| `git add` | `publish-cli add` | 计算文件 MD5，写入 `staged-files.json` |
| `git reset` | `publish-cli reset` | 从 `staged-files.json` 移除条目 |
| `git push` | `publish-cli push` | 逐文件上传 → 创建版本变更日志 → 清空暂存区 |
| `git log` | `publish-cli log` | `GET /api/project/get_project_change_logs/{id}` |

**暂存区文件结构** (`staged-files.json`)：

```json
[
  {
    "relativePath": "app.exe",
    "status": "modified",
    "localMd5": "abc123def456",
    "localSize": 4428592
  },
  {
    "relativePath": "lib.dll",
    "status": "new",
    "localMd5": "789abcdef012",
    "localSize": 102400
  }
]
```

**`push` 两阶段流程**：

```
1. UPLOAD 阶段：逐文件 POST /api/file/upload_file（multipart）
   - 任一文件上传失败 → 停止，报告错误，暂存区保留（可重试）
2. PUBLISH 阶段：POST /api/project/publish_version（创建版本记录 + 更新 Project.Version）
   - 仅在全部文件上传成功后执行
   - 成功 → 清空暂存区
```

> `publish_version` 端点已实现 ✅（见 §6.3），在事务中更新 `Project.Version` 并创建 `ProjectChangeLog`。

---

## 二、职责划分

### Publish CLI (发布者工具)

```
┌─────────────────────────────────────────┐
│           Publish CLI / GUI             │
│                                         │
│  • 管理项目配置                          │
│  • 发现本地与服务端差异                   │
│  • 添加差异文件到暂存区                   │
│  • 推送到服务端（创建新版本）             │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│              Server API                 │
│                                         │
│  /api/project/*                        │
│  /api/file/*                           │
└─────────────────────────────────────────┘
```

### Client (终端用户工具)

```
┌─────────────────────────────────────────┐
│              Client                     │
│                                         │
│  • 检查服务端新版本                      │
│  • 下载更新包                           │
│  • 应用更新/回滚                        │
└─────────────────────────────────────────┘
```

---

## 三、命令设计

### 3.1 项目管理命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `project list` | 列出所有未删除项目 | `--server <url> [--json]` |
| `project create` | 创建新项目（自动创建初始版本 V1.0.0 及首条变更日志） | `--server <url> --name <name> --title <title> [--force-update] [--ignore-folders <folders>] [--ignore-files <files>]` |
| `project update` | 更新项目配置（标题、强制更新、忽略规则） | `--server <url> --id <id> --title <title> [--force-update] [--ignore-folders <folders>] [--ignore-files <files>]` |
| `project delete` | 软删除项目（服务端标记 `is_deleted=true`，不删除已上传文件） | `--server <url> --id <id>` |
| `project info` | 查看项目详情（当前版本号、配置等） | `--server <url> --id <id>` |

> **`project info` 说明**：服务端无「获取单个项目」端点，当前通过 `GET /api/project/get_all_projects` 获取全量列表后按 ID 筛选。若需独立端点，见 §9.1。

#### project list 示例

```bash
$ publish-cli project list --server http://localhost:8080

  ID  NAME     TITLE      VERSION  FORCE UPDATE  CREATED
   1  myapp    我的应用    V1.0.1   yes           2024-01-01
   2  demo     Demo项目    V1.0.0   no            2024-01-05
```

```bash
$ publish-cli project list --json

{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 1,
      "name": "myapp",
      "title": "我的应用",
      "version": "V1.0.1",
      "force_update": true,
      "ignore_folders": [".git", "node_modules"],
      "ignore_files": ["*.log"],
      "created_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": 2,
      "name": "demo",
      "title": "Demo项目",
      "version": "V1.0.0",
      "force_update": false,
      "ignore_folders": [],
      "ignore_files": [],
      "created_at": "2024-01-05T00:00:00Z"
    }
  ]
}
```

#### project info 示例

```bash
$ publish-cli project info --server http://localhost:8080 --id 1

Project: myapp
Title:   我的应用
Version: V1.0.1
Force Update: yes
Created: 2024-01-01

Ignore Folders:
  .git
  node_modules

Ignore Files:
  *.log
```

#### project create 示例

```bash
publish-cli project create \
  --server http://localhost:8080 \
  --name myapp \
  --title "我的应用" \
  --force-update \
  --ignore-folders ".git,node_modules,.vscode" \
  --ignore-files "*.log,*.tmp"
```

### 3.2 文件差异命令

#### diff 算法

`status` 和 `diff` 共用同一套差异计算逻辑：

```
1. 本地扫描：递归遍历 --path 目录，应用 ignore 规则（文件夹 + 文件 glob），
   对每个文件计算 MD5 + SHA256，得到 LocalFileList
2. 远端查询：GET /api/file/get_all_files/{projectId}，得到 ServerFileList
3. 比对：以 fileRelativePath 为 key，对齐后逐项比较 MD5：
   - 仅本地有 → status = "new"
   - 仅远端有 → status = "deleted"
   - 两端都有，MD5 相同 → status = "unchanged"
   - 两端都有，MD5 不同 → status = "modified"
```

#### 忽略规则语法

配置中的 `ignore.folders` 和 `ignore.files` 控制扫描范围：

| 配置项 | 匹配方式 | 示例 | 效果 |
|--------|---------|------|------|
| `ignore.folders` | 路径前缀匹配（`path.IsSubPath`） | `.git` | 跳过 `.git/` 及其所有子文件和子目录 |
| `ignore.folders` | 路径前缀匹配 | `node_modules` | 跳过 `node_modules/`、`src/node_modules/` 等任意层级下的同名目录 |
| `ignore.files` | 精确路径匹配 | `*.log` | 跳过根目录下的 `*.log`（不递归子目录） |
| `ignore.files` | 精确路径匹配 | `config/secrets.json` | 跳过指定相对路径的文件 |

> **注意**：服务端的忽略规则在 `GET /api/file/get_all_files` 时同样生效，因此远端返回的文件列表已排除忽略项。CLI 本地扫描**必须**使用相同规则，否则 diff 结果不准确。

| 命令 | 说明 | 参数 |
|------|------|------|
| `status` | 查看本地与服务端差异（汇总视图） | `--server <url> --project <name> --path <local-path> [--json]` |
| `diff` | 详细对比单个或全部文件差异 | `--server <url> --project <name> --path <local-path> [--file <relative-path>] [--json]` |

#### status 输出示例（人类可读）

```bash
$ publish-cli status --server http://localhost:8080 --project myapp --path ./dist

Changes to be committed:
  (use "publish-cli reset <file>..." to unstage)

        modified:   app.exe
        new file:   lib.dll

Changes not staged for commit:
  (use "publish-cli add <file>..." to stage)

        modified:   config.json
        deleted:    old_plugin.dll

Unchanged files:
        readme.md
        assets/logo.png
```

#### status 输出示例（JSON）

```bash
$ publish-cli status --json

{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "staged": [
      {"relativePath": "app.exe",    "status": "modified",   "localMd5": "abc123", "localSize": 4428592},
      {"relativePath": "lib.dll",    "status": "new",        "localMd5": "def456", "localSize": 102400}
    ],
    "unstaged": [
      {"relativePath": "config.json","status": "modified",   "localMd5": "789xyz", "localSize": 2048},
      {"relativePath": "old_plugin.dll", "status": "deleted","localMd5": "",       "localSize": 0}
    ],
    "unchanged": [
      {"relativePath": "readme.md",  "status": "unchanged",  "localMd5": "aaa111", "localSize": 512}
    ]
  }
}
```

#### diff 输出示例（单文件）

```bash
$ publish-cli diff --server http://localhost:8080 --project myapp --path ./dist --file app.exe

- local:  app.exe   2024-01-01 12:00:00  4.2 MB  md5:abc123  sha256:...
+ server: app.exe   2024-01-02 10:00:00  4.3 MB  md5:def456  sha256:...
```

### 3.3 暂存区命令

暂存区数据存储在 `<project-path>/.publish-cli/staging/staged-files.json`，结构见 §1.4。

**操作语义**：

| 操作 | 行为 |
|------|------|
| `add <file>...` | 对指定文件计算 MD5，写入 `staged-files.json`。文件必须是 `status` 中的 new/modified 类型 |
| `add --all` | 将所有 new + modified 文件加入暂存区 |
| `reset <file>...` | 从 `staged-files.json` 中移除指定条目，不删除本地文件 |
| `reset --all` | 清空整个 `staged-files.json` |
| `staged` | 读取并展示 `staged-files.json` 内容 |

> `add` 时计算的 MD5 会在 `push` 阶段与本地文件重新校验——若文件在 add 后被修改，push 将拒绝上传并提示用户重新 add。

| 命令 | 说明 | 参数 |
|------|------|------|
| `add` | 添加文件到暂存区 | `<file>...` |
| `add --all` | 添加所有变更文件到暂存区 | |
| `reset` | 从暂存区移除文件 | `<file>...` |
| `reset --all` | 清空暂存区 | |
| `staged` | 查看暂存区内容 | `[--json]` |

> 当配置文件中已设置 `server.url`、`project.name`、`project.path` 时，无需在以上命令中重复指定 `--server` / `--project` / `--path`。

#### staged 输出示例（JSON）

```bash
$ publish-cli staged --json

{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {"relativePath": "app.exe", "status": "modified", "localMd5": "abc123", "localSize": 4428592},
    {"relativePath": "lib.dll", "status": "new",      "localMd5": "def456", "localSize": 102400}
  ]
}
```

#### 工作流程示例

```bash
# 查看差异
$ publish-cli status

# 添加特定文件
$ publish-cli add app.exe lib.dll

# 添加所有变更
$ publish-cli add --all

# 查看暂存区
$ publish-cli staged

# 移除某个文件
$ publish-cli reset app.exe

# 清空暂存区
$ publish-cli reset --all
```

### 3.4 发布命令

#### push 执行流程（两阶段）

```
阶段 1 — 上传文件：
  1. 读取 staged-files.json
  2. 重新校验每个文件的本地 MD5（防止 add 后被修改）
  3. 逐文件 POST /api/file/upload_file（multipart）：
     - file: <文件流>
     - projectName: <项目名>
     - relativeFileName: <相对路径>
  4. 任一文件上传失败 → 停止，报告错误，暂存区保留（可重试）
     全部成功 → 进入阶段 2

阶段 2 — 创建版本记录：
  5. POST /api/project/publish_version（创建版本记录 + 更新 Project.Version）
     - projectId, version, logs[], time
  6. 服务端创建 ProjectChangeLog + 更新 Project.Version
  7. 成功 → 清空 staged-files.json
```

| 参数 | 说明 |
|------|------|
| `--version` | **必填**。新版本号（如 `V1.0.1`） |
| `--message` | **必填**。单条变更说明；多次 `--message` 合并为 `logs[]` 数组 |
| `--dry-run` | 仅校验（MD5 复核 + 列出将上传的文件），不实际推送 |
| `--force` | 跳过 MD5 复核检查，强制上传 |

> **`push` vs `push-all`**：`push` 只上传暂存区中的文件；`push-all` 跳过暂存区，直接将所有 new/modified 文件上传并发布。`push-all` 等价于 `add --all && push`。

| 命令 | 说明 | 参数 |
|------|------|------|
| `push` | 推送暂存区文件 → 上传 + 创建版本 | `--version <ver> --message <msg> [--dry-run] [--force]` |
| `push-all` | 一键推送所有变更（无需先 add） | `--version <ver> --message <msg> [--dry-run]` |
| `publish` | 完整发布流程：status → add --all → push | `--version <ver> --message <msg> [--dry-run]` |

#### push 示例

```bash
# 1. 查看变更
$ publish-cli status

# 2. 选择要发布的文件
$ publish-cli add app.exe lib.dll

# 3. 确认暂存内容
$ publish-cli staged

# 4. 推送到服务端
$ publish-cli push --version V1.0.1 --message "修复登录bug" --message "新增搜索功能"

# 或者一键完成
$ publish-cli publish --version V1.0.1 --message "修复登录bug"
```

#### push --dry-run 示例

```bash
$ publish-cli push --version V1.0.1 --message "test" --dry-run

[DRY RUN] 将上传以下文件：
  [modified]  app.exe    4.2 MB
  [new]       lib.dll    100 KB
[DRY RUN] 将创建版本: V1.0.1 (2 条日志)
[DRY RUN] 未实际推送任何内容。
```

### 3.5 版本历史命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `log` | 查看变更日志 | `--server <url> --project <name> [--limit <n>] [--json]` |

#### log 输出示例（人类可读）

```bash
$ publish-cli log --project myapp --limit 5

V1.0.3 (2024-01-15 10:00:00)
  • 优化性能

V1.0.2 (2024-01-10 15:30:00)
  • 修复登录bug

V1.0.1 (2024-01-05 09:00:00)
  • 新增搜索功能
```

#### log 输出示例（JSON）

```bash
$ publish-cli log --project myapp --json

{
  "isSuccess": true,
  "errorMsg": "",
  "data": [
    {
      "id": 3,
      "version": "V1.0.3",
      "logs": ["优化性能"],
      "time": "2024-01-15 10:00:00",
      "created_at": "2024-01-15T10:00:00Z"
    },
    {
      "id": 2,
      "version": "V1.0.2",
      "logs": ["修复登录bug"],
      "time": "2024-01-10 15:30:00",
      "created_at": "2024-01-10T15:30:00Z"
    }
  ]
}
```

### 3.6 服务器信息命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `server info` | 查看服务端系统与硬件信息 | `--server <url> --id <project-id> [--json]` |

> 调用 `GET /api/project/get_project_os_info/{projectId}`，返回服务端系统的操作系统和硬件信息，可用于判断目标部署平台。

#### server info 输出示例（人类可读）

```bash
$ publish-cli server info --server http://localhost:8080 --id 1

OS:              linux
Platform:        ubuntu
Architecture:    amd64
Go Version:      go1.21.0
CPU:             Intel(R) Core(TM) i7-9700K @ 3.60GHz
Cores:           8 @ 3600 MHz
Disk:            120.5 GiB used / 500.0 GiB total (24.1%)
```

#### server info 输出示例（JSON）

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": [{
    "os": "linux",
    "platform": "ubuntu",
    "goARCH": "amd64",
    "version": "go1.21.0",
    "numCPU": 8,
    "cpuName": "Intel(R) Core(TM) i7-9700K",
    "cpuMhz": 3600.0,
    "diskUsed": 120.5,
    "diskFree": 379.5,
    "diskTotal": 500.0,
    "diskUsedPercent": 24.1
  }]
}
```

### 3.7 配置命令

#### 配置加载优先级

每个参数按以下顺序查找，找到即停止：

```
CLI 显式参数 (--server / --project / --path)
  → 项目级 .publish-cli/config.json
    → 全局 ~/.publish-cli/config.json
      → 必填参数缺失时报错
```

| 命令 | 说明 | 参数 |
|------|------|------|
| `config init` | 初始化项目配置 | `--server <url> --project <name> --path <local-path>` |
| `config set` | 设置配置项（标量值） | `<key> <value>` |
| `config set-array` | 设置数组配置项（追加模式） | `<key> --add <item> / --remove <item> / --clear` |
| `config get` | 获取配置项 | `<key>` |
| `config list` | 列出当前生效的全部配置 | `[--json]` |
| `config path` | 显示配置文件路径 | |

#### 配置项说明

| 配置键 | 类型 | 说明 | 示例 |
|--------|------|------|------|
| `server.url` | `string` | 服务器地址 | `http://localhost:8080` |
| `project.name` | `string` | 项目名称 | `myapp` |
| `project.path` | `string` | 本地构建产物路径 | `./dist` |
| `project.id` | `int` | 项目 ID（config init 后自动缓存） | `1` |
| `ignore.folders` | `[]string` | 忽略的文件夹 | `.git,node_modules` |
| `ignore.files` | `[]string` | 忽略的文件 glob | `*.log,*.tmp` |
| `output.format` | `string` | 默认输出格式 | `human` / `json` |

#### config init 示例

```bash
# 一键初始化（创建 ./.publish-cli/config.json）
$ publish-cli config init \
  --server http://localhost:8080 \
  --project myapp \
  --path ./dist

# 后续命令可直接使用，无需重复指定参数
$ publish-cli status
$ publish-cli publish --version V1.0.1 --message "更新说明"
```

#### config set / set-array 示例

```bash
# 标量值
$ publish-cli config set server.url http://deploy.example.com
$ publish-cli config set output.format json

# 数组操作
$ publish-cli config set-array ignore.folders --add ".cache"
$ publish-cli config set-array ignore.folders --add "temp"
$ publish-cli config set-array ignore.folders --remove ".cache"
$ publish-cli config set-array ignore.folders --clear

# 查看当前配置
$ publish-cli config list
server.url      = http://deploy.example.com
project.name    = myapp
project.path    = ./dist
ignore.folders  = [node_modules, temp]
ignore.files    = [*.log, *.tmp]
output.format   = json
```

---

## 四、输出格式约定

### 4.1 统一 JSON 响应格式

server、client、publish-cli 三端 **统一使用 camelCase** 作为 JSON 响应字段命名：

| 项目 | 结构体 | 字段命名 | 说明 |
|------|--------|---------|------|
| server | `models.CommonResponse` | `isSuccess` / `errorMsg` | Go `json:"isSuccess"` |
| publish_tool_avalonia | `CommonResponse` / `CommonResponse<T>` | `isSuccess` / `errorMsg` | C# `[JsonProperty("isSuccess")]` |
| client | `model.CommonResponse` / `model.Output` | `isSuccess` / `errorMsg` | 解析服务端响应 + CLI 输出 |
| **publish-cli** | `CommonResponse` | `isSuccess` / `errorMsg` | 解析服务端响应 + CLI 输出 |

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": { ... }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `isSuccess` | `bool` | 请求是否成功 |
| `errorMsg` | `string` | 失败时的错误信息；成功时为空串 `""` |
| `data` | `any` | 业务数据；无数据时为 `null` |

**输出实现**（参照 `client/cmd/common.go` 的 `printOutput` 模式）：

```go
// printOutput 按 isSuccess/errorMsg/data 格式输出 JSON 到 stdout
func printOutput(success bool, errMsg string, data interface{}) {
    out := CommonResponse{
        IsSuccess: success,
        ErrorMsg:  errMsg,
        Data:      data,
    }
    bytes, _ := json.Marshal(out)
    fmt.Println(string(bytes))
}

// 各命令使用示例
printOutput(true, "", statusData)
printOutput(false, "项目名称不能为空", nil)
```

### 4.2 JSON 输出

使用 `--json` 参数，便于脚本调用和管道处理：

```bash
$ publish-cli status --json

{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "staged": [
      {"relativePath": "app.exe", "status": "modified", "localSize": 4428592, "localMd5": "abc123"},
      {"relativePath": "lib.dll", "status": "new",      "localSize": 102400,  "localMd5": "def456"}
    ],
    "unstaged": [
      {"relativePath": "config.json", "status": "modified", "localSize": 1024, "localMd5": "789xyz"}
    ],
    "unchanged": [
      {"relativePath": "readme.md", "status": "unchanged", "localSize": 512, "localMd5": "aaa111"}
    ]
  }
}
```

### 4.3 人类可读格式（默认）

不带 `--json` 参数时，输出面向人类阅读的彩色文本：

```bash
$ publish-cli status

Changes staged for commit:
  [modified]  app.exe  4.2 MB
  [new]       lib.dll  100 KB

Changes not staged for commit:
  [modified]  config.json  1 KB
  [deleted]   old_plugin.dll
```

### 4.4 静默模式

使用 `--quiet` 参数，仅输出关键结果行：

```bash
$ publish-cli push --version V1.0.1 --message "更新" --quiet
# 仅输出: V1.0.1 published successfully (2 files uploaded)
```

### 4.5 输出格式优先级

```
--quiet > --json > 配置中的 output.format > 默认 human
```

即：`--quiet` 和 `--json` 同时指定时 `--quiet` 生效。

### 4.6 退出码

便于 CI/CD 脚本判断命令执行结果：

| 退出码 | 含义 | 场景 |
|--------|------|------|
| `0` | 成功 | 所有操作正常完成 |
| `1` | 一般错误 | 参数错误、文件不存在、网络不可达 |
| `2` | 部分失败 | `push` 时部分文件上传失败（staging 保留） |
| `3` | 冲突 | MD5 校验失败（文件在 add 后被修改） |
| `4` | 认证失败 | 服务端返回认证错误（预留） |

---

## 五、配置文件

### 5.1 配置文件位置

| 操作系统 | 配置文件路径 |
|---------|-------------|
| Windows | `%USERPROFILE%\.publish-cli\config.json` |
| Linux/Mac | `~/.publish-cli/config.json` |
| 项目级 | `./.publish-cli/config.json` |

### 5.2 配置文件格式

```json
{
  "server": {
    "url": "http://localhost:8080"
  },
  "project": {
    "name": "myapp",
    "path": "./dist"
  },
  "ignore": {
    "folders": [".git", "node_modules", ".vscode"],
    "files": ["*.log", "*.tmp", "*.bak"]
  },
  "output": {
    "format": "human"
  }
}
```

### 5.3 项目目录结构

发布项目（`--path` 指定的本地构建产物目录）的完整结构：

```
dist/                               # --path 指定的本地构建产物根目录
├── app.exe                         # 构建产物文件（会被扫描和上传）
├── lib.dll
├── config.json
├── readme.md
├── assets/
│   └── logo.png
├── .gitignore                      # 不在扫描范围内
└── .publish-cli/                   # publish-cli 元数据目录（不入库，加入 .gitignore）
    ├── config.json                 # 项目级配置（server.url / project.name / ignore 规则）
    └── staging/
        └── staged-files.json       # 已暂存文件清单（结构见 §1.4）
```

> `.publish-cli/` 目录**不应**上传到服务端（它不在扫描范围内，且被自动忽略）。建议在项目 `.gitignore` 中添加 `.publish-cli/`。

---

## 六、与 Server API 的对应关系

### 6.0 服务端完整 API 一览

| 方法 | 路径 | 说明 | CLI 使用者 |
|------|------|------|-----------|
| GET | `/api/project/get_all_projects` | 获取所有未删除项目 | `project list`, `project info` |
| POST | `/api/project/create_project` | 创建项目 + 首条 V1.0.0 变更日志 | `project create` |
| POST | `/api/project/update_project` | 更新项目配置（标题、强制更新、忽略规则） | `project update` |
| POST | `/api/project/delete_project/{projectId}` | 软删除项目 | `project delete` |
| GET | `/api/project/get_project_change_logs/{projectId}` | 获取变更日志列表 | `log` |
| GET | `/api/project/get_project_os_info/{projectId}` | 获取服务端系统信息 | `server info` |
| POST | `/api/project/publish_version` | 发布新版本（更新 Project.Version + 创建 ChangeLog） ✅ | `push`, `push-all`, `publish` |
| POST | `/api/file/upload_file` | 上传单个文件（multipart） | `push`, `push-all`, `publish` |
| GET | `/api/file/get_all_files/{projectId}` | 获取服务端已存储文件列表（含 MD5/SHA256） | `status`, `diff` |
| GET | `/api/file/download_file?path=` | 下载单个文件 | *(保留，供 client 使用)* |

> publish-cli 作为**发布**工具不使用 `download_file`，但该端点与 `client`（终端用户更新程序）共享。

### 6.1 CLI 命令 ↔ API 详细映射

| CLI 命令 | API | 方法 | 请求格式 | 响应格式 |
|---------|-----|------|---------|---------|
| `project list` | `/api/project/get_all_projects` | GET | — | `CommonResponse<[]Project>` |
| `project create` | `/api/project/create_project` | POST | `{name, title, isForceUpdate, ignoreFolders, ignoreFiles}` | `CommonResponse<Project>` |
| `project update` | `/api/project/update_project` | POST | `{id, title, isForceUpdate, ignoreFolders, ignoreFiles}` | `CommonResponse` |
| `project delete` | `/api/project/delete_project/{projectId}` | POST | URI `projectId` | `CommonResponse` |
| `project info` | `/api/project/get_all_projects` | GET | —（从全量列表中按 ID 筛选） | `CommonResponse<[]Project>` |
| `status` | `/api/file/get_all_files/{projectId}` | GET | — | `CommonResponse<[]FileInfo>` |
| `diff` | `/api/file/get_all_files/{projectId}` | GET | — | 同上，本地比对 |
| `push` | `/api/file/upload_file` × N → `POST /api/project/publish_version` | POST | multipart + JSON（见 §6.3） | `CommonResponse` + `CommonResponse<ProjectChangeLog>` |
| `log` | `/api/project/get_project_change_logs/{projectId}` | GET | — | `CommonResponse<[]ProjectChangeLog>` |
| `server info` | `/api/project/get_project_os_info/{projectId}` | GET | — | `CommonResponse<[ServerOSInfo]>`（仅 1 条） |

### 6.2 关键 DTO 字段对照

#### Project（获取/列表响应，ent 生成 → JSON tag 为 **snake_case**）

| JSON 字段 | Go 类型 | 说明 | CLI 用途 |
|-----------|---------|------|---------|
| `id` | `int` | 主键 | `--id` 参数 |
| `name` | `string` | 项目名称（创建后不可改） | `--project` 参数，用于 upload 时指定目标 |
| `title` | `string` | 项目抬头（显示名） | 列表 / info 展示 |
| `version` | `string` | 当前版本号（如 `V1.0.0`） | `project info` 展示 |
| `force_update` | `bool` | 是否强制更新 | `--force-update` |
| `ignore_folders` | `[]string` | 忽略的文件夹 | 本地扫描时排除 |
| `ignore_files` | `[]string` | 忽略的文件 glob | 本地扫描时排除 |
| `created_at` | `time.Time` | 创建时间 | 展示用 |

> **命名注意**：GET 响应的 ent JSON 使用 **snake_case**（`force_update`、`ignore_folders`），而 POST 请求体使用 **camelCase**（`isForceUpdate`、`ignoreFolders`）。CLI 的 DTO 反序列化必须兼容两种风格，详见 §9.3。

#### FileInfo（服务端文件扫描返回）

| JSON 字段 | Go 类型 | 说明 |
|-----------|---------|------|
| `fileAbsolutePath` | `string` | 服务端绝对路径（如 `/data/myapp/app.exe`） |
| `fileRelativePath` | `string` | 相对项目根目录路径（`app.exe`） |
| `lastUpdateTime` | `time.Time` | 最后修改时间 |
| `fileSize` | `int64` | 文件大小（字节） |
| `md5` | `string` | MD5 哈希（32 位 hex） |
| `sha256` | `string` | SHA256 哈希（64 位 hex） |

#### ServerOSInfo（服务端系统信息）

| JSON 字段 | Go 类型 | 说明 |
|-----------|---------|------|
| `os` | `string` | 操作系统（`windows`、`linux`） |
| `platform` | `string` | 平台（`ubuntu`、`debian`） |
| `goARCH` | `string` | CPU 架构（`amd64`） |
| `version` | `string` | Go 运行时版本 |
| `numCPU` | `int` | 逻辑 CPU 数 |
| `cpuName` | `string` | CPU 型号 |
| `cpuMhz` | `float64` | CPU 频率（MHz） |
| `diskUsed` | `float64` | 已用磁盘（GiB） |
| `diskFree` | `float64` | 可用磁盘（GiB） |
| `diskTotal` | `float64` | 总磁盘（GiB） |
| `diskUsedPercent` | `float64` | 磁盘使用率（%） |

#### ProjectChangeLog（版本变更日志）

| JSON 字段 | Go 类型 | 说明 |
|-----------|---------|------|
| `id` | `int` | 主键 |
| `version` | `string` | 版本号（如 `V1.0.1`） |
| `logs` | `[]string` | 变更说明条目列表（每条对应一个 `--message`） |
| `time` | `string` | 变更时间（格式化字符串 `yyyy-MM-dd HH:mm:ss`） |
| `created_at` | `time.Time` | 记录创建时间 |

### 6.3 发布版本 API：`publish_version` ✅ 已实现

发布新版本的专属端点，在 `upload_file` 完成后调用。

#### 接口规格

```
POST /api/project/publish_version
Content-Type: application/json

Request:
{
  "projectId": 1,
  "version": "V1.0.1",
  "logs": ["修复登录bug", "新增搜索功能"],
  "time": "2025-01-15 10:00:00"
}

Response (200):
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "id": 3,
    "version": "V1.0.1",
    "logs": ["修复登录bug", "新增搜索功能"],
    "time": "2025-01-15 10:00:00",
    "created_at": "2025-01-15T10:00:00Z"
  }
}
```

#### 服务端实现

在事务中执行两步操作（[project_service.go](server/internal/service/project_service.go:119-151)）：

1. `tx.Project.Update().SetVersion(version)` — 更新 `Project.Version`
2. `tx.ProjectChangeLog.Create().SetProject(p).SetVersion(version).SetLogs(logs).SetTime(time)` — 创建变更日志并关联项目

路由注册于 [routers.go](server/routers/routers.go:19)：
```go
projectGroup.POST("publish_version", controllers.PublishVersion)
```

---

## 七、使用场景

### 7.1 日常发布

```bash
# 初始化（只需一次）
$ publish-cli config init --server http://deploy.example.com --project myapp --path ./dist

# 日常发布
$ publish-cli status                    # 查看变更
$ publish-cli publish --version V1.2.0 --message "新增用户管理模块"
```

### 7.2 CI/CD 集成

```bash
#!/bin/bash
# build.sh

# 构建
npm run build

# 发布
publish-cli config set server.url $DEPLOY_SERVER
publish-cli config set project.name $PROJECT_NAME
publish-cli config set project.path ./dist

publish-cli publish \
  --version $BUILD_VERSION \
  --message "CI Build #${BUILD_NUMBER}"
```

### 7.3 Git Hooks 集成

```bash
# .git/hooks/post-commit
#!/bin/bash
publish-cli status --quiet
```

---

## 八、项目结构

```
publish-cli/
├── cmd/
│   └── publish-cli/
│       └── main.go              # 入口，注册 cobra 命令
├── internal/
│   ├── config/
│   │   ├── config.go            # 配置结构体 + 全局/项目级加载合并
│   │   └── config_test.go
│   ├── api/
│   │   ├── client.go            # HTTP 客户端（serverUrl 管理 + CommonResponse 解包）
│   │   ├── project.go           # Project 相关 API 调用
│   │   ├── file.go              # File 相关 API 调用（upload / get_all_files）
│   │   └── client_test.go
│   ├── diff/
│   │   ├── scanner.go           # 本地文件扫描 + MD5/SHA256 计算
│   │   ├── differ.go            # 本地 vs 远端差异比对（new/modified/deleted/unchanged）
│   │   └── differ_test.go
│   ├── staging/
│   │   ├── staging.go           # staged-files.json 读写 + MD5 校验
│   │   └── staging_test.go
│   └── cmd/
│       ├── root.go              # root 命令（--server, --quiet, --json 全局 flags）
│       ├── project.go           # project {list,create,update,delete,info} 子命令
│       ├── status.go            # status 命令
│       ├── diff.go              # diff 命令
│       ├── add.go               # add / add --all 命令
│       ├── reset.go             # reset / reset --all 命令
│       ├── staged.go            # staged 命令
│       ├── push.go              # push / push-all 命令
│       ├── publish.go           # publish 命令（组合 status+add+push）
│       ├── log.go               # log 命令
│       ├── server.go            # server info 命令
│       └── config.go            # config {init,set,get,list} 命令
├── pkg/
│   └── models/
│       ├── common.go            # CommonResponse 结构体 (isSuccess/errorMsg)
│       ├── project.go           # Project DTO（GET 响应 snake_case）
│       ├── file_info.go         # FileInfo DTO
│       ├── change_log.go        # ProjectChangeLog DTO
│       └── server_os_info.go    # ServerOSInfo DTO
├── go.mod                       # module github.com/yourorg/publish-cli
├── go.sum
└── README.md
```

### 8.1 包职责

| 包 | 职责 | 依赖 |
|----|------|------|
| `cmd/publish-cli` | 程序入口，装配依赖，启动 cobra | `internal/cmd` |
| `internal/cmd` | cobra 命令定义，参数解析，调用 `api` / `diff` / `staging` | `internal/api`, `internal/diff`, `internal/staging`, `internal/config` |
| `internal/api` | HTTP 客户端：封装 server URL 管理、`CommonResponse` 解包、multipart 上传 | `pkg/models` |
| `internal/diff` | 本地文件扫描 + MD5/SHA256 计算 + 差异比对引擎 | `pkg/models` |
| `internal/staging` | `staged-files.json` 的 CRUD + push 前 MD5 复核 | `pkg/models` |
| `internal/config` | 配置加载（全局 > 项目级优先级合并）、配置写入 | — |
| `pkg/models` | 纯 DTO 结构体（可被外部引用），统一 camelCase 输出 | — |

---

## 九、已知缺口与后续工作

### 9.1 服务端 API 缺口

| # | 缺口 | 优先级 | 说明 |
|---|------|--------|------|
| 1 | `POST /api/project/publish_version` | ✅ 已实现 | [project_service.go](server/internal/service/project_service.go:119-151)，事务内更新版本 + 创建变更日志 |
| 2 | `GET /api/project/get_project_by_id/{id}` | 低 | 当前 `project info` 只能从全量列表中筛选；非阻塞，但独立端点更高效 |
| 3 | `POST /api/file/delete_file` | 低 | 暂无删除服务端文件的能力；`reset` 仅影响本地暂存区 |

### 9.2 CLI 技术选型待定

- [ ] **命令行框架**：`cobra`（推荐，K8s/Helm 同款，支持子命令、自动补全）或 `urfave/cli`
- [ ] **HTTP 客户端**：`resty`（推荐，支持重试、中间件）或标准库 `net/http` + 手动封装
- [ ] **进度条**：上传进度显示 → `progressbar` 或 `cheggaaa/pb`
- [ ] **彩色输出**：终端颜色支持 → `fatih/color` 或 `gookit/color`
- [ ] **配置管理**：JSON 读写 → 标准库 `encoding/json` + `os.UserConfigDir`
- [ ] **大文件上传**：当前单次 multipart，大文件（>100MB）需考虑分片上传 + 断点续传

### 9.3 双命名风格注意事项

服务端 JSON 存在两种命名约定，CLI 的 DTO 必须分别处理：

| 场景 | 命名风格 | 示例 | 来源 |
|------|---------|------|------|
| GET 响应（ent 生成） | **snake_case** | `force_update`, `ignore_folders`, `created_at` | ent/schema → Go struct json tag |
| POST 请求体（控制器） | **camelCase** | `isForceUpdate`, `ignoreFolders`, `projectId` | 控制器 `ShouldBindJSON` 结构体 |

建议在 Go CLI 的 models 包中为同一实体准备两套 struct：

```go
// 反序列化 GET 响应（ent snake_case）
type ProjectResponse struct {
    ID           int      `json:"id"`
    Name         string   `json:"name"`
    Version      string   `json:"version"`
    ForceUpdate  bool     `json:"force_update"`
    IgnoreFolders []string `json:"ignore_folders"`
    IgnoreFiles  []string `json:"ignore_files"`
}

// POST 请求体（camelCase）
type CreateProjectRequest struct {
    Name          string   `json:"name"`
    Title         string   `json:"title"`
    IsForceUpdate bool     `json:"isForceUpdate"`
    IgnoreFolders []string `json:"ignoreFolders"`
    IgnoreFiles   []string `json:"ignoreFiles"`
}
```

### 9.4 测试计划

- [ ] HTTP API mock 测试（`httptest`）
- [ ] diff 算法单元测试（本地文件 → MD5 比对逻辑）
- [ ] staging JSON 读写测试
- [ ] config 解析测试（全局 + 项目级优先级合并）
- [ ] 集成测试（需要运行中的 server）
