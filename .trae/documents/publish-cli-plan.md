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

- 项目管理（创建、更新、删除）
- 本地与服务端文件差异对比
- 暂存区管理（类似 git add/commit）
- 版本发布与变更日志

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
| `project list` | 列出所有项目 | `--server <url>` |
| `project create` | 创建新项目 | `--server <url> --name <name> --title <title> [--force-update] [--ignore-folders <folders>] [--ignore-files <files>]` |
| `project update` | 更新项目 | `--server <url> --id <id> --title <title> [--force-update] [--ignore-folders <folders>] [--ignore-files <files>]` |
| `project delete` | 删除项目 | `--server <url> --id <id>` |
| `project info` | 查看项目详情 | `--server <url> --id <id>` |

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

| 命令 | 说明 | 参数 |
|------|------|------|
| `status` | 查看本地与服务端差异 | `--server <url> --project <name> --path <local-path>` |
| `diff` | 详细对比文件差异 | `--server <url> --project <name> --path <local-path> [--file <relative-path>]` |

#### status 输出示例

```bash
$ publish-cli status --server http://localhost:8080 --project myapp --path ./dist

On branch main
Changes to be committed:
  (use "publish-cli reset <file>..." to unstage)

        modified:   app.exe
        new file:   lib.dll

Changes not staged for commit:
  (use "publish-cli add <file>..." to stage)

        modified:   config.json
```

#### diff 输出示例

```bash
$ publish-cli diff --server http://localhost:8080 --project myapp --path ./dist --file app.exe

- local:  app.exe   1.0.0 (2024-01-01 12:00:00)  4.2 MB  md5:abc123
+ server: app.exe   1.0.0 (2024-01-02 10:00:00)  4.3 MB  md5:def456
```

### 3.3 暂存区命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `add` | 添加文件到暂存区 | `--server <url> --project <name> --path <local-path> <file>...` |
| `add-all` | 添加所有变更文件 | `--server <url> --project <name> --path <local-path>` |
| `reset` | 从暂存区移除文件 | `--server <url> --project <name> --path <local-path> <file>...` |
| `reset-all` | 清空暂存区 | `--server <url> --project <name> --path <local-path>` |
| `stash list` | 查看暂存区内容 | `--server <url> --project <name> --path <local-path>` |

#### 工作流程示例

```bash
# 查看差异
$ publish-cli status

# 添加特定文件
$ publish-cli add app.exe lib.dll

# 添加所有变更
$ publish-cli add-all

# 查看暂存区
$ publish-cli stash list

# 移除某个文件
$ publish-cli reset app.exe

# 清空暂存区
$ publish-cli reset-all
```

### 3.4 发布命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `push` | 推送暂存区文件到服务端 | `--server <url> --project <name> --path <local-path> --version <version> --message <message> [--force]` |
| `push-all` | 一键推送所有变更 | `--server <url> --project <name> --path <local-path> --version <version> --message <message>` |
| `publish` | 完整发布流程 (diff + add-all + push) | `--server <url> --project <name> --path <local-path> --version <version> --message <message>` |

#### push 示例

```bash
# 先添加文件
$ publish-cli add-all

# 推送暂存区文件
$ publish-cli push --version 1.0.1 --message "修复登录bug"
```

#### push-all 示例

```bash
# 直接推送所有变更，无需先 add
$ publish-cli push-all --version 1.0.1 --message "修复登录bug"
```

#### publish 示例

```bash
# 一键完成：diff + add-all + push
$ publish-cli publish --version 1.0.1 --message "修复登录bug"
```

### 3.5 版本历史命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `log` | 查看变更日志 | `--server <url> --project <name> [--limit <n>]` |

#### log 输出示例

```bash
$ publish-cli log --project myapp --limit 5

v1.0.3 (2024-01-15 10:00:00)
  • 优化性能

v1.0.2 (2024-01-10 15:30:00)
  • 修复登录bug

v1.0.1 (2024-01-05 09:00:00)
  • 新增搜索功能
```

### 3.6 服务器信息命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `server info` | 查看服务器信息 | `--server <url> --project <name>` |

#### server info 输出示例

```bash
$ publish-cli server info --server http://localhost:8080 --project myapp

OS:       linux
Platform: ubuntu
Arch:     amd64
Go:       1.21.0
CPU:      Intel(R) Core(TM) i7-9700K @ 3.60GHz (8 cores)
Disk:     120.5 GB / 500.0 GB (24.1% used)
```

### 3.7 配置命令

| 命令 | 说明 | 参数 |
|------|------|------|
| `config set` | 设置配置项 | `<key> <value>` |
| `config get` | 获取配置项 | `<key>` |
| `config list` | 列出所有配置 | |
| `config init` | 初始化项目配置 | `--server <url> --project <name> --path <local-path>` |

#### 配置项说明

| 配置键 | 说明 | 示例 |
|--------|------|------|
| `server.url` | 服务器地址 | `http://localhost:8080` |
| `project.name` | 项目名称 | `myapp` |
| `project.path` | 本地路径 | `./dist` |
| `ignore.folders` | 忽略文件夹 | `.git,node_modules` |
| `ignore.files` | 忽略文件 | `*.log,*.tmp` |

#### config init 示例

```bash
# 一键初始化
$ publish-cli config init \
  --server http://localhost:8080 \
  --project myapp \
  --path ./dist

# 后续命令可直接使用，无需重复指定参数
$ publish-cli status
$ publish-cli publish --version 1.0.1 --message "更新说明"
```

---

## 四、输出格式约定

### 4.1 JSON 输出（默认）

便于其他工具调用和解析：

```bash
$ publish-cli status --format json

{
  "is_success": true,
  "err_msg": "",
  "data": {
    "staged": [
      {"path": "app.exe", "status": "modified", "size": 4428592, "md5": "abc123"},
      {"path": "lib.dll", "status": "new", "size": 102400, "md5": "def456"}
    ],
    "unstaged": [
      {"path": "config.json", "status": "modified", "size": 1024, "md5": "789xyz"}
    ],
    "unchanged": [
      {"path": "readme.md", "status": "unchanged", "size": 2048, "md5": "aaaaaa"}
    ]
  }
}
```

### 4.2 人类可读格式

使用 `--human` 或 `--format human` 参数：

```bash
$ publish-cli status --format human

On branch main
Changes staged for commit:
  [modified]  app.exe  4.2 MB
  [new]       lib.dll  100 KB

Changes not staged for commit:
  [modified]  config.json  1 KB
```

### 4.3 静默模式

使用 `--quiet` 参数，减少输出：

```bash
$ publish-cli push --version 1.0.1 --message "更新" --quiet
# 仅输出: v1.0.1 published successfully
```

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

### 5.3 暂存区文件位置

```
.local/                         # 项目级暂存区（与 .git 同级）
├── .publish-cli/
│   ├── config.json             # 项目配置
│   └── staging/                # 暂存文件清单
│       └── staged-files.json   # 已暂存文件列表
```

---

## 六、与 Server API 的对应关系

| CLI 命令 | 调用 API | 方法 |
|---------|---------|------|
| `project list` | `/api/project/get_all_projects` | GET |
| `project create` | `/api/project/create_project` | POST |
| `project update` | `/api/project/update_project` | POST |
| `project delete` | `/api/project/delete_project/{id}` | POST |
| `project info` | `/api/project/get_project_os_info/{id}` | GET |
| `status` / `diff` | `/api/file/get_all_files/{id}` | GET |
| `push` | `/api/file/upload_file` | POST (multipart) |
| `log` | `/api/project/get_project_change_logs/{id}` | GET |
| `server info` | `/api/project/get_project_os_info/{id}` | GET |

---

## 七、使用场景

### 7.1 日常发布

```bash
# 初始化（只需一次）
$ publish-cli config init --server http://deploy.example.com --project myapp --path ./dist

# 日常发布
$ publish-cli status                    # 查看变更
$ publish-cli publish --version 1.2.0 --message "新增用户管理模块"
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

## 八、项目结构（TODO）

```
publish-cli/
├── cmd/
│   └── main.go
├── internal/
│   ├── config/          # 配置管理
│   ├── api/             # HTTP 客户端
│   ├── staging/         # 暂存区管理
│   └── diff/            # 文件差异计算
├── pkg/
│   └── models/          # 数据模型
├── go.mod
├── go.sum
└── README.md
```

---

## 九、待补充

- [ ] 命令行参数解析库选择（cobra / cli / urfave/cli）
- [ ] 进度条实现方案
- [ ] 大文件上传分片方案
- [ ] 错误重试机制
- [ ] 日志输出方案
- [ ] 单元测试覆盖
