# ClientUpdator — 客户端自动更新系统

一个轻量级的客户端自动更新解决方案，包含服务端（Go）和客户端（Go 1.10 / Windows XP 兼容）。

## 项目结构

```
ClientUpdator/
├── server/                    # 服务端（Go 1.25 + Gin + Ent + SQLite）
├── client/                    # 客户端更新程序（Go 1.10，兼容 Windows XP）
├── publish_tool_avalonia/     # 发布工具 GUI（Avalonia 12 桌面应用）
├── publish-cli/               # 发布工具 CLI（Go 1.25 + cobra，命令行列发布）
├── .trae/
│   └── documents/             # 设计文档
│       ├── publish-cli-plan.md
│       └── avalonia-publish-tool-plan.md
└── README.md
```

## 快速开始

### 1. 启动服务端

从 [Releases](https://github.com/lishuangquan1987/ClientUpdator/releases) 下载对应平台的服务端：

| 平台 | 文件 |
|------|------|
| Windows 64-bit | `server-windows-amd64.exe` |
| Linux 64-bit | `server-linux-amd64` |
| Linux ARM64 | `server-linux-arm64` |
| macOS Intel | `server-darwin-amd64` |
| macOS Apple Silicon | `server-darwin-arm64` |

```bash
# Windows
server-windows-amd64.exe -p 2000

# Linux / macOS
./server-linux-amd64 -p 2000
```

首次启动会自动创建 SQLite 数据库（`configs/clientupdator.db`）并建表。

服务端启动后在后台运行，提供 REST API 供发布工具和客户端调用。

### 2. 配置客户端

下载 `client-updator.exe`（仅支持 Windows XP 及以上 32 位），放到目标应用的更新目录下。

在 `client-updator.exe` 同级目录创建 `client.yaml`：

```yaml
project_name: "my_app"
url: "http://your-server:2000"
main_exe_relative_path: "../ApplicationFolder/main_application.exe"
un_copy_files:
  - *.log
un_copy_folders:
  - Logs
  - x86
must_close_process_name:
  - main_application
post_update_script: ""
```

| 字段 | 说明 |
|------|------|
| `project_name` | 项目名称，需与服务端一致 |
| `url` | 服务端地址 |
| `main_exe_relative_path` | 主程序相对路径（相对于 `client-updator.exe`） |
| `must_close_process_name` | 更新前需关闭的进程名（不含 `.exe`） |
| `un_copy_files` | 更新时不复制的文件（支持通配符） |
| `un_copy_folders` | 更新时不复制的文件夹 |
| `post_update_script` | 更新后执行的脚本（可选） |

### 3. 客户端命令

所有命令输出 JSON 到 stdout，格式为（camelCase，三端统一）：

```json
{"isSuccess": true, "errorMsg": "", "data": {...}}
```

#### check_update — 检查更新

```bash
client-updator.exe check_update [--url <url>] [--project-name <name>]
```

有更新时返回 `has_update: true`，`force_update` 表示是否强制更新。

#### check_diff — 文件比对

```bash
client-updator.exe check_diff [--url <url>] [--project-name <name>]
```

列出本地与服务端 MD5 不一致的文件（包括本地不存在的文件）。

#### download_update — 下载更新

```bash
client-updator.exe download_update [--url <url>] [--project-name <name>]
```

下载差异文件到 `{main_exe_folder_name}_{new_version}/`，下载后校验 MD5 + SHA-256（双校验，最多重试 3 次）。文件 > 100MB 时自动启用断点续传。

#### apply_update — 应用更新

```bash
client-updator.exe apply_update [--main-exe-path <path>] [--must-close-process-name <name1,name2>] [--close-timeout <seconds>]
```

执行原子替换：
1. 关闭目标进程（先 WM_CLOSE，超时后强制终止）
2. 备份当前版本 → `{folder}_{old_version}/`
3. 激活新版本 → `{folder}/`
4. 执行 `post_update_script`（如配置）
5. 启动主程序

#### list_rollback_versions — 列出可回滚版本

```bash
client-updator.exe list_rollback_versions
```

#### rollback — 版本回滚

```bash
client-updator.exe rollback --version 1.0.0
```

#### check_self_update — 更新程序自检

```bash
client-updator.exe check_self_update
```

比对自身 SHA-256 与 `ApplicationFolder/check-updator.exe` 是否一致。

## 目录结构

客户端部署后的典型目录结构：

```
PackageFolder/
├── ApplicationFolder/              # 当前主程序
│   ├── main_application.exe
│   └── check-updator.exe           # updator 副本（用于自更新比对）
├── UpdateFolder/                   # 更新程序
│   ├── client-updator.exe
│   ├── client.yaml
│   └── version.json
├── ApplicationFolder_0.0.1/        # 历史版本（可回滚）
├── ApplicationFolder_0.0.2/
└── ...
```

## 服务端 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/project/get_all_projects` | 获取所有项目 |
| POST | `/api/project/create_project` | 创建项目 |
| POST | `/api/project/update_project` | 更新项目 |
| POST | `/api/project/delete_project/:id` | 删除项目 |
| GET | `/api/project/get_project_change_logs/:id` | 获取变更日志 |
| GET | `/api/project/get_project_os_info/:id` | 获取服务器信息 |
| POST | `/api/project/publish_version` | 发布新版本（更新版本号 + 创建变更日志） |
| POST | `/api/file/upload_file` | 上传文件（multipart） |
| GET | `/api/file/get_all_files/:id` | 获取项目文件列表 |
| GET | `/api/file/download_file?path=` | 下载文件 |

## 发布工具

### publish_tool_avalonia（GUI）

面向发布者的桌面工具（Avalonia 12），提供可视化的项目管理、文件上传和版本发布功能。详细文档将在功能稳定后补充。

### publish-cli（命令行）✅ 可用

publish-cli 是 `publish_tool_avalonia` 的命令行版本，使用 Go 1.25 + cobra 构建，面向 CI/CD 集成和终端操作。工作流借鉴 Git（`status` → `add` → `push`），所有 JSON 输出统一使用 `camelCase`。

#### 构建

```bash
cd publish-cli
go build -o publish-cli.exe ./cmd/publish-cli
```

#### 快速上手

```bash
# 1. 初始化项目配置（只需一次）
publish-cli config init \
  --server http://localhost:8080 \
  --project myapp \
  --path ./dist

# 2. 查看差异
publish-cli status

# 3. 一键发布
publish-cli publish --version V1.0.1 --message "修复登录bug"
```

#### 全局参数

所有命令都支持以下全局参数：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--server` | 服务器地址 | 配置文件 `server.url` |
| `--project` | 项目名称 | 配置文件 `project.name` |
| `--path` | 本地构建产物路径 | 配置文件 `project.path` |
| `--id` | 项目 ID（直传，跳过名称查找） | 配置文件 `project.id` |
| `--json` | JSON 格式输出 | `false`（默认人类可读） |
| `--quiet` | 静默模式（仅输出关键行） | `false` |

> 配置参数按优先级查找：**CLI 参数 > 项目级 `.publish-cli/config.json` > 全局 `~/.publish-cli/config.json`**

---

#### project — 项目管理

```bash
# 列出所有项目
publish-cli project list --server http://localhost:8080

# 创建新项目（自动创建初始版本 V1.0.0）
publish-cli project create \
  --server http://localhost:8080 \
  --name myapp \
  --title "我的应用" \
  --force-update \
  --ignore-folders ".git,node_modules,.vscode" \
  --ignore-files "*.log,*.tmp"

# 更新项目配置
publish-cli project update \
  --id 1 \
  --title "我的应用 v2" \
  --force-update \
  --ignore-folders ".git,node_modules,.cache"

# 查看项目详情
publish-cli project info --id 1

# 删除项目（软删除）
publish-cli project delete --id 1
```

| 子命令 | 说明 | 关键参数 |
|--------|------|---------|
| `list` | 列出所有未删除项目 | `--server` |
| `create` | 创建新项目 + 初始版本 V1.0.0 | `--name`(必), `--title`(必), `--force-update`, `--ignore-folders`, `--ignore-files` |
| `update` | 更新项目配置 | `--id`(必), `--title`(必), `--force-update`, `--ignore-folders`, `--ignore-files` |
| `delete` | 软删除项目 | `--id`(必) |
| `info` | 查看项目详情（当前版本、忽略规则等） | `--id`(必) |

---

#### status / diff — 文件差异对比

```bash
# 查看本地与服务端差异（汇总视图）
publish-cli status --project myapp --path ./dist

# JSON 格式输出
publish-cli status --json

# 详细对比所有文件
publish-cli diff --project myapp --path ./dist

# 对比单个文件
publish-cli diff --project myapp --path ./dist --file app.exe
```

**status 输出示例**：

```
Changes staged for commit:
  (use "publish-cli reset <file>..." to unstage)

        modified:   app.exe
        new file:   lib.dll

Changes not staged for commit:
  (use "publish-cli add <file>..." to stage)

        modified:   config.json
        deleted:    old_plugin.dll

Unchanged files:
        readme.md
```

**diff 算法**：本地递归扫描 → 查询服务端文件列表 → 以相对路径为 key，逐项 MD5 比对。分类为 `new` / `modified` / `deleted` / `unchanged`。

---

#### add / reset / staged — 暂存区管理

借鉴 Git 的暂存区概念，文件存储在 `<path>/.publish-cli/staging/staged-files.json`。

```bash
# 添加特定文件到暂存区
publish-cli add app.exe lib.dll

# 添加所有变更文件
publish-cli add --all

# 查看暂存区内容
publish-cli staged

# 从暂存区移除文件
publish-cli reset app.exe

# 清空暂存区
publish-cli reset --all
```

| 命令 | 说明 |
|------|------|
| `add <file>...` | 计算 MD5，加入暂存区 |
| `add --all` | 将所有 `new` + `modified` 文件加入暂存区 |
| `staged` | 查看暂存区文件列表 |
| `reset <file>...` | 移除指定文件 |
| `reset --all` | 清空暂存区 |

> `add` 时记录的 MD5 会在 `push` 时重新校验。若文件在 add 后被修改，push 将拒绝并提示重新 add。使用 `--force` 可跳过此检查。

---

#### push / push-all / publish — 版本发布

```bash
# 标准流程：暂存 → 推送
publish-cli add app.exe lib.dll
publish-cli push --version V1.0.1 --message "修复登录bug" --message "新增搜索功能"

# 跳过暂存区，直接推送所有变更
publish-cli push-all --version V1.0.1 --message "快速修复"

# 一键完成：status → add --all → push
publish-cli publish --version V1.0.1 --message "日常发布"

# 预演模式（不实际推送）
publish-cli push --version V1.0.1 --message "test" --dry-run
```

**push 两阶段流程**：

1. **UPLOAD**：逐文件 `POST /api/file/upload_file`（multipart），任一失败即停止，暂存区保留
2. **PUBLISH**：`POST /api/project/publish_version`（事务更新 `Project.Version` + 创建 `ProjectChangeLog`），成功清空暂存区

| 命令 | 说明 | 参数 |
|------|------|------|
| `push` | 推送暂存区文件 | `--version`(必), `--message`(必, 可多次), `--dry-run`, `--force` |
| `push-all` | 跳过暂存区直接推送 | 同上 |
| `publish` | `add --all` + `push` | 同上 |

---

#### log — 版本历史

```bash
# 查看最近 20 条变更日志
publish-cli log --project myapp

# 限制条数
publish-cli log --project myapp --limit 5

# JSON 输出
publish-cli log --project myapp --json
```

输出示例：

```
V1.0.3 (2024-01-15 10:00:00)
  • 优化性能

V1.0.2 (2024-01-10 15:30:00)
  • 修复登录bug
```

---

#### watch — 实时监控

轮询监控本地目录，文件变更时实时输出。

```bash
# 每 2 秒扫描一次
publish-cli watch --project myapp --path ./dist

# 自定义间隔 + 自动暂存
publish-cli watch --project myapp --path ./dist --interval 5 --auto-add
```

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--interval` | 轮询间隔（秒） | `2` |
| `--auto-add` | 检测到变更时自动加入暂存区 | `false` |

按 `Ctrl+C` 退出。

---

#### server info — 服务器信息

```bash
publish-cli server info --id 1
publish-cli server info --id 1 --json
```

输出服务器操作系统、CPU、内存、磁盘使用情况等信息。

---

#### config — 配置管理

```bash
# 初始化项目配置（创建 ./.publish-cli/config.json）
publish-cli config init \
  --server http://localhost:8080 \
  --project myapp \
  --path ./dist

# 设置配置项
publish-cli config set server.url http://deploy.example.com
publish-cli config set output.format json

# 数组操作
publish-cli config set-array ignore.folders --add ".cache"
publish-cli config set-array ignore.folders --remove ".cache"
publish-cli config set-array ignore.folders --clear

# 查看配置
publish-cli config get server.url
publish-cli config list
publish-cli config path
```

| 子命令 | 说明 |
|--------|------|
| `init` | 初始化项目配置 |
| `set <key> <value>` | 设置标量配置项 |
| `set-array <key> --add/--remove/--clear` | 操作数组配置项 |
| `get <key>` | 获取配置值 |
| `list` | 列出全部生效配置 |
| `path` | 显示配置文件路径 |

**配置键**：

| 键 | 类型 | 说明 |
|----|------|------|
| `server.url` | `string` | 服务器地址 |
| `project.name` | `string` | 项目名称 |
| `project.path` | `string` | 本地构建产物路径 |
| `ignore.folders` | `[]string` | 忽略的文件夹（逗号分隔，路径前缀匹配） |
| `ignore.files` | `[]string` | 忽略的文件（逗号分隔，精确路径匹配） |
| `output.format` | `string` | 默认输出格式（`human` / `json`） |

---

#### 输出格式

**人类可读**（默认）：彩色终端输出。

**JSON**（`--json`）：统一使用 `camelCase` 包装：

```json
{"isSuccess": true, "errorMsg": "", "data": {...}}
```

退出码用于 CI/CD 判断：

| 退出码 | 含义 |
|--------|------|
| `0` | 成功 |
| `1` | 一般错误（参数错误、网络不可达等） |
| `2` | 部分失败（push 上传部分文件失败） |
| `3` | MD5 冲突（文件在 add 后被修改） |

---

#### CI/CD 集成示例

```bash
#!/bin/bash
# 构建 → 发布

npm run build

publish-cli config set server.url $DEPLOY_SERVER
publish-cli config set project.name $PROJECT_NAME
publish-cli config set project.path ./dist

publish-cli publish \
  --version $BUILD_VERSION \
  --message "CI Build #${BUILD_NUMBER}"

if [ $? -ne 0 ]; then
  echo "发布失败"
  exit 1
fi
```

## 从源码构建

### 服务端

```bash
cd server
go build -ldflags="-s -w" -o server.exe ./cmd
```

交叉编译（`CGO_ENABLED=0` 纯静态编译）：

```bash
cd server
# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server-linux-amd64 ./cmd
# Linux arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o server-linux-arm64 ./cmd
# macOS amd64
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o server-darwin-amd64 ./cmd
# macOS arm64
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o server-darwin-arm64 ./cmd
```

### 客户端

需要 Go 1.10（兼容 Windows XP），不使用 Go Modules：

```bat
set GOOS=windows
set GOARCH=386
set GO111MODULE=off
go get gopkg.in/yaml.v2
go build -ldflags="-s -w" -o client-updator.exe clientupdator/client
```

## License

MIT
