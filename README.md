# ClientUpdator — 客户端自动更新系统

一个轻量级的客户端自动更新解决方案，包含服务端（Go）和客户端（Go 1.10 / Windows XP 兼容）。

## 项目结构

```
ClientUpdator/
├── server/                    # 服务端（Go + Gin + Ent + SQLite）
├── client/                    # 客户端更新程序（Go 1.10，兼容 Windows XP）
├── publish_tool_avalonia/     # 发布工具（Avalonia 桌面应用，开发中）
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

所有命令输出 JSON 到 stdout，格式为：

```json
{"is_success": true, "err_msg": "", "data": {...}}
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
| POST | `/api/file/upload_file` | 上传文件（multipart） |
| GET | `/api/file/get_all_files/:id` | 获取项目文件列表 |
| GET | `/api/file/download_file?path=` | 下载文件 |

## 发布工具（开发中）

`publish_tool_avalonia` 是面向发布者的桌面工具，提供可视化的项目管理、文件上传和版本发布功能。详细文档将在功能稳定后补充。

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
