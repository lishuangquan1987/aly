# Zap 项目命令行参考文档

本文档涵盖两个命令行工具的所有命令：

1. **`zap-update.exe`**（位于 `zap-client/`）— 自动更新客户端，负责检查、下载、应用更新及回滚
2. **`zap-publish.exe`**（位于 `publish/publish-cli/`）— 发布命令行工具，负责项目管理、文件比对、暂存和推送发布

---

## 一、更新客户端 — zap-update.exe

### 设计概要

| 项目 | 说明 |
|------|------|
| 语言 | Go 1.10（兼容 Windows XP，不使用 Go Modules） |
| 架构 | 32 位（`GOARCH=386`） |
| 输出格式 | 所有命令输出统一 JSON：`{"isSuccess": bool, "errorMsg": string, "data": ...}` |
| 配置加载 | `client.json` → `.updator/shared.json` → CLI 参数覆盖 |
| version.json 状态机 | `applied` → `downloaded` → `applying` → `applied`（支持崩溃恢复） |

### 全局用法

```bash
zap-update.exe <命令> [选项]
```

| 命令 | 说明 |
|------|------|
| `check_update` | 检查是否有新版本 |
| `check_diff` | 比对本地的服务端文件差异 |
| `download_update` | 下载变更文件到版本目录 |
| `apply_update` | 应用已下载的更新（原子替换） |
| `list_rollback_versions` | 列出可回滚的历史版本 |
| `rollback` | 回滚到指定版本 |
| `check_self_update` | 检查更新程序自身是否需要更新 |

---

### 1.1 check_update — 检查更新

**用途**：查询服务端是否有新版本，供上层应用（如 GUI 客户端）决定是否提示用户更新。

**使用场景**：应用启动时、定时检查任务、用户手动点击"检查更新"时调用。

**用法**：

```bash
zap-update.exe check_update [--url <服务器地址>] [--project-name <项目名称>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--url` | string | 否 | 覆盖配置文件中的服务器地址 |
| `--project-name` | string | 否 | 覆盖配置文件中的项目名称 |

**实现逻辑**：

1. 加载完整配置（`client.json` → `.updator/shared.json` + CLI 参数覆盖）
2. 读取 `version.json`，根据 `version_status` 分三路处理：
   - `applied` 或空：联系服务端比对版本，一致或断网 → 无更新；不一致 → `need_download_update=true`
   - `downloaded`：联系服务端比对版本，一致或断网 → 继续 apply（`need_download_update=false`）；不一致 → 重新下载
   - `applying`：同 `downloaded`，支持崩溃恢复
3. 调用 `GET /api/project/get_project_by_name/{projectName}` 获取项目信息（含 `force_update`）
4. 调用 `GET /api/project/get_project_change_logs/{projectName}` 获取变更日志
5. 取最新一条日志的版本号，与本地 `version.json` 的版本号做比较

**返回示例**：

```json
// 有更新（需要下载）
{"isSuccess":true,"data":{"has_update":true,"need_download_update":true,"current_version":"1.0.0","new_version":"1.0.1","force_update":false}}

// 有更新（已下载，只需 apply）
{"isSuccess":true,"data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}

// 无更新
{"isSuccess":true,"data":{"has_update":false,"need_download_update":false,"current_version":"1.0.0"}}

// 已有未完成的更新（downloaded/applying + 断网）
{"isSuccess":true,"data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}

// 失败
{"isSuccess":false,"errorMsg":"no server url configured","data":null}
```

**示例 1：检查默认配置项目的更新**

```bash
zap-update.exe check_update
```

**示例 2：用临时参数检查其他服务端项目的更新**

```bash
zap-update.exe check_update --url http://192.168.1.100:2000 --project-name myapp
```

---

### 1.2 check_diff — 文件比对

**用途**：列出需要从服务端下载的差异文件列表（服务端有且与本地 MD5 不同的文件）。供可视化客户端展示更新详情。

**使用场景**：检测到有更新后，在下载前查看具体哪些文件会变更，方便用户确认。

**用法**：

```bash
zap-update.exe check_diff [--url <服务器地址>] [--project-name <项目名称>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--url` | string | 否 | 覆盖服务器地址 |
| `--project-name` | string | 否 | 覆盖项目名称 |

**实现逻辑**：

1. 加载完整配置
2. 调用 `GET /api/file/get_all_files/{projectName}` 获取服务端所有文件列表（含 MD5、SHA256）
3. 递归扫描本地 `ApplicationFolder`，计算每个文件的 MD5
4. 对比服务端文件与本地文件：
   - **本地不存在** → 加入差异列表（`local_md5` 为空）
   - **本地存在但 MD5 不同** → 加入差异列表
   - 本地存在且 MD5 一致 → 跳过
5. 同时获取最新版本号

> **注意**：只返回"服务端有而本地没有或不同"的文件，**不返回**仅本地存在、服务端已删除的文件。

**返回示例**：

```json
{
  "isSuccess": true,
  "data": {
    "new_version": "2.0.0",
    "files": [
      {"path": "app.exe", "local_md5": "abc123", "local_size": 43284, "local_sha256": "849384", "server_md5": "def456", "server_size": 5242880, "server_sha256": "884374"},
      {"path": "plugins/new.dll", "local_md5": "", "local_size": 0, "local_sha256": "", "server_md5": "xyz789", "server_size": 102400, "server_sha256": "88437486"}
    ]
  }
}
```

**示例 1：查看当前项目差异文件**

```bash
zap-update.exe check_diff
```

**示例 2：指定远程项目查看差异**

```bash
zap-update.exe check_diff --url http://update.example.com:2000 --project-name game_client
```

---

### 1.3 download_update — 下载更新

**用途**：将差异文件从服务端下载到本地版本目录（`ApplicationFolder_{newVersion}/`），不替换当前运行的程序。

**使用场景**：确认有新版本后，在实际应用更新前先下载所有文件。

**用法**：

```bash
zap-update.exe download_update [--url <服务器地址>] [--project-name <项目名称>] [--main-exe-path <相对路径>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--url` | string | 否 | 覆盖服务器地址 |
| `--project-name` | string | 否 | 覆盖项目名称 |
| `--main-exe-path` | string | 否 | 覆盖主程序相对路径 |

**实现逻辑**：

1. 获取服务端变更日志，取最新版本号
2. 读取 `version.json` 获取当前版本号
3. 若 `newVersion <= currentVersion`，返回"已是最新版本"
4. 获取服务端所有文件列表，与本地的 `ApplicationFolder` 对比 MD5，得出需下载的文件列表
5. 为目标版本创建目录 `PackageFolder/ApplicationFolder_{newVersion}/`
6. 逐文件下载：
   - 若目标目录中已存在同名文件且 MD5+SHA256 校验通过 → `SKIP`
   - **大文件断点续传**（>100MB）：写入 `.part` 临时文件，支持 `Range` 请求头断点续传，完成后重命名
   - **小文件直接下载**：写入 `.part` 临时文件，完成后重命名
7. 每个文件下载后校验 MD5+SHA256，不匹配则重试最多 3 次
8. 全部成功后更新 `version.json`：`version_status → "downloaded"`，记录新旧版本号，并从服务端变更日志同步 `after_apply_update_script` 字段

**进度输出**（每行一个 JSON，最后一行 `data: null` 表示全部完成。所有服务端文件都有进度行，`total` = 服务端文件总数，`index` 1..total 无重复）：

```
{"isSuccess":true,"data":{"index":1,"total":5,"file":"app.exe","status":"START","file_size":5242880}}
{"isSuccess":true,"data":{"index":1,"total":5,"file":"app.exe","status":"DONE","file_size":5242880}}
{"isSuccess":true,"data":{"index":2,"total":5,"file":"new.dll","status":"SKIP","file_size":102400}}
...
{"isSuccess":true,"data":null}   ← 完成行
```

**返回示例**：

```json
// 成功（所有文件下载完成后输出 data:null）
{"isSuccess":true,"data":null}

// 失败（文件中途校验失败）
{"isSuccess":false,"errorMsg":"app.exe: checksum mismatch","data":null}
```

**示例 1：下载默认项目的最新更新**

```bash
zap-update.exe download_update
```

**示例 2：指定服务端下载特定项目的更新**

```bash
zap-update.exe download_update --url http://192.168.1.100:2000 --project-name myapp --main-exe-path ../MyApp/MyApp.exe
```

---

### 1.4 apply_update — 应用更新

**用途**：将已下载的新版本通过原子替换方式应用到当前运行目录，随后启动主程序。

**使用场景**：`download_update` 完成后调用，执行实际的版本切换。

**用法**：

```bash
zap-update.exe apply_update [--main-exe-path <路径>] [--close-timeout <秒数>]
```

**参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `--main-exe-path` | string | 否 | — | 覆盖主程序相对路径 |
| `--close-timeout` | int | 否 | 30 | 进程关闭超时秒数 |

**实现逻辑**：

1. 读取 `version.json`，检查 `version_status`：
   - `applied` → 返回"没有待应用的更新"
   - `applying` → 崩溃恢复流程：
     - 若 `ApplicationFolder` 存在 → 重新走步骤 4 的替换
     - 若 `ApplicationFolder` 不存在但 `ApplicationFolder_{version}` 存在 → 直接重命名并跳到步骤 6
     - 两者都不存在 → 返回错误
   - `downloaded` → 正常执行

2. 将 `version_status` 设为 `applying`（标记开始）

3. **关闭进程**：从 `client.json` 读取 `must_close_process_name` 列表，对列表中的每个进程：
   - 先发送 `WM_CLOSE` 消息请求正常退出（Windows）
   - 等待超时（默认 30s）
   - 超时后 `TerminateProcess` 强杀

4. **原子替换**（三步确保崩溃可恢复）：
   - ① 将 `ApplicationFolder` 内容（排除忽略规则）复制到 `ApplicationFolder_{version}`
   - ② 将 `ApplicationFolder` 重命名为 `ApplicationFolder_{previousVersion}`
   - ③ 将 `ApplicationFolder_{version}` 重命名为 `ApplicationFolder`
   - 任一步失败则回滚并重置状态为 `downloaded`

5. 更新 `version.json`：`version_status → "applied"`

6. **执行后更新脚本**：从 `version.json` 的 `after_apply_update_script` 字段读取脚本路径（该字段在 download_update 时从服务端变更日志同步），异步执行，失败仅记日志到 `update.log`

7. **启动主程序**（`main_exe_relative_path`）

**返回示例**：

```json
// 成功
{"isSuccess":true,"data":null}

// 无待应用更新
{"isSuccess":false,"errorMsg":"no pending update to apply","data":null}

// 替换失败已回滚
{"isSuccess":false,"errorMsg":"backup rename failed: ...","data":null}
```

**示例 1：正常应用更新**

```bash
zap-update.exe apply_update
```

**示例 2：应用更新前延长关闭超时**

```bash
zap-update.exe apply_update --close-timeout 60
```

---

### 1.5 list_rollback_versions — 列出可回滚版本

**用途**：扫描包目录下所有已备份的历史版本目录，返回可回滚的版本列表。

**使用场景**：更新后发现新版本有问题，需要查看可回滚到哪些历史版本。

**用法**：

```bash
zap-update.exe list_rollback_versions [--main-exe-path <路径>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--main-exe-path` | string | 否 | 覆盖主程序相对路径 |

**实现逻辑**：

1. 获取 `PackageFolder/` 目录下所有子目录
2. 过滤出以 `{main_exe_folder_name}_` 为前缀的目录名
3. 对后缀进行版本号格式验证（`X.Y.Z` 格式，至少两段，每段为纯数字）
4. 返回合法版本号列表 + 当前版本号

**返回示例**：

```json
{"isSuccess":true,"data":{"current_version":"1.0.1","versions":["1.0.0","0.9.0"]}}
```

**示例 1：列出可回滚版本**

```bash
zap-update.exe list_rollback_versions
```

**示例 2：指定主程序路径查看历史版本**

```bash
zap-update.exe list_rollback_versions --main-exe-path ../MyApp/MyApp.exe
```

---

### 1.6 rollback — 版本回滚

**用途**：将当前运行的版本回滚到指定的历史版本（原子替换）。

**使用场景**：新版本有问题，需要紧急回滚到上一个稳定版本。

**用法**：

```bash
zap-update.exe rollback --version <目标版本号> [--main-exe-path <路径>] [--close-timeout <秒数>]
```

**参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `--version` | string | **是** | — | 目标回滚版本号 |
| `--main-exe-path` | string | 否 | — | 覆盖主程序相对路径 |
| `--close-timeout` | int | 否 | 30 | 进程关闭超时秒数 |

**实现逻辑**：

流程与 `apply_update` 基本一致，核心区别：

- **不执行 `CopyDirWithExclude`**：回滚的目标版本目录是之前 apply 时保存的完整快照，无需补全文件
- **直接做原子重命名**：`ApplicationFolder` ↔ `ApplicationFolder_{previousVersion}` 交换

同样支持 `applying` 状态下的崩溃恢复。

**返回示例**：

```json
// 成功
{"isSuccess":true,"data":{"version":"1.0.0"}}

// 目标版本目录不存在
{"isSuccess":false,"errorMsg":"version 0.0.1 not found","data":null}

// --version 参数缺失
{"isSuccess":false,"errorMsg":"--version is required","data":null}
```

**示例 1：回滚到上一个版本**

```bash
zap-update.exe rollback --version 1.0.0
```

**示例 2：回滚到指定版本并延长超时**

```bash
zap-update.exe rollback --version 0.9.0 --close-timeout 45
```

---

### 1.7 check_self_update — 自更新检查

**用途**：比对更新客户端自身（`zap-update.exe`）是否需要更新。

**使用场景**：主程序启动后检查，若需要更新则将新版本复制覆盖旧版。

**用法**：

```bash
zap-update.exe check_self_update [--main-exe-path <路径>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--main-exe-path` | string | 否 | 覆盖主程序相对路径 |

**实现逻辑**：

1. 找到 `ApplicationFolder/zap-update.exe`（随每次发布打包的副本）
2. 计算当前运行中的 `zap-update.exe`（`UpdateFolder/zap-update.exe`）的 SHA256
3. 计算 `ApplicationFolder/zap-update.exe` 的 SHA256
4. 若两者不一致 → 需要更新
5. 若 `ApplicationFolder/zap-update.exe` 不存在（首次部署）→ 不需要更新

> **原理**：发布者每次打包时将当前版本的 `zap-update.exe` 复制一份放入主程序目录。更新后主程序目录的版本已更新，但 UpdateFolder 的仍是旧版。

**返回示例**：

```json
// 需要更新
{"isSuccess":true,"data":{"need_update":true}}

// 不需要更新
{"isSuccess":true,"data":{"need_update":false}}
```

**示例 1：检查自身是否需要更新**

```bash
zap-update.exe check_self_update
```

**示例 2：指定主程序路径进行自检**

```bash
zap-update.exe check_self_update --main-exe-path ../MyApp/MyApp.exe
```

---

## 二、发布命令行工具 — zap-publish.exe

### 设计概要

| 项目 | 说明 |
|------|------|
| 语言 | Go（使用 Go Modules） |
| CLI 框架 | `github.com/spf13/cobra` |
| 配置存储 | 项目目录下的 `.updator/shared.json` + `.updator/publish.json` |
| 工作流 | `config init` → `status`/`diff` → `add` → `push`（类似 Git） |
| 输出格式 | 支持 human 可读格式（默认）和 `--json` JSON 格式 |
| 全局参数 | `--server`, `--project`, `--id`, `--json`, `--quiet` |

### 全局参数

以下参数适用于所有子命令：

| 参数 | 类型 | 说明 |
|------|------|------|
| `--server` | string | 服务器地址（覆盖配置文件） |
| `--project` | string | 项目名称（覆盖配置文件） |
| `--id` | int | 项目 ID（直传，跳过名称查找） |
| `--json` | bool | JSON 格式输出 |
| `--quiet` | bool | 静默模式（抑制 human 输出） |

### 用法

```bash
zap-publish [全局参数] <命令> [子命令] [选项]
```

### 命令树

```
zap-publish
├── config              # 配置管理
│   ├── init            # 初始化项目配置
│   ├── set             # 设置配置项
│   ├── set-array       # 设置数组配置项
│   ├── get             # 获取配置项
│   ├── list            # 列出所有配置
│   └── path            # 显示配置文件路径
├── project             # 项目管理
│   ├── list            # 列出所有项目
│   ├── create          # 创建新项目
│   ├── update          # 更新项目配置
│   ├── delete          # 删除项目
│   └── info            # 查看项目详情
├── status              # 查看本地与服务端文件差异
├── diff                # 详细对比文件差异
├── add                 # 添加文件到暂存区
├── reset               # 从暂存区移除文件
├── staged              # 查看暂存区内容
├── push                # 推送暂存区文件到服务端
├── push-all            # 一键推送所有变更
├── publish             # 完整发布流程
├── log                 # 查看版本变更日志
├── watch               # 实时监控文件变更
└── server
    └── info            # 查看服务端系统信息
```

---

### 2.1 config init — 初始化项目配置

**用途**：在当前目录创建 `.updator/` 目录及配置文件（`shared.json` + `publish.json`）。

**使用场景**：首次使用 `zap-publish` 发布项目时的初始化步骤。

**用法**：

```bash
zap-publish config init --project <项目名称> [--server <服务器地址>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--project` | string | **是** | 项目名称 |
| `--server` | string | 否 | 服务器地址（也可后续通过 `config set` 设置） |

**实现逻辑**：

1. 在当前工作目录创建 `.updator/` 目录
2. 写入 `shared.json`：包含 `server_url`、`project_name` 和空的 `ignore_folders`/`ignore_files`
3. 写入 `publish.json`：包含 `output_format`（默认 `human`）

**返回示例**：

```text
# human 格式
配置已保存到: /path/to/project/.updator

# JSON 格式
{"isSuccess":true,"data":{"shared_path":"/path/.updator/shared.json","publish_path":"/path/.updator/publish.json"}}
```

**示例 1：初始化项目配置**

```bash
cd /path/to/myapp
zap-publish config init --project myapp --server http://192.168.1.100:2000
```

**示例 2：先初始化再设置服务器地址**

```bash
cd /path/to/myapp
zap-publish config init --project myapp
zap-publish config set server.url http://192.168.1.100:2000
```

---

### 2.2 config set — 设置配置项

**用途**：设置单个配置项的值。

**用法**：

```bash
zap-publish config set <key> <value>
```

**支持的键**：

| 键 | 类型 | 说明 |
|------|------|------|
| `server.url` | string | 服务器地址 |
| `project.name` | string | 项目名称 |
| `output.format` | string | 输出格式（`human` / `json`） |

**示例 1：设置服务器地址**

```bash
zap-publish config set server.url http://192.168.1.100:2000
```

**示例 2：设置输出格式**

```bash
zap-publish config set output.format json
```

---

### 2.3 config set-array — 设置数组配置项

**用途**：管理忽略规则等数组类型的配置项。

**用法**：

```bash
zap-publish config set-array <key> --add <条目> | --remove <条目> | --clear
```

**支持的键**：

| 键 | 说明 |
|------|------|
| `ignore.folders` | 忽略的文件夹列表 |
| `ignore.files` | 忽略的文件列表 |

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `--add` | string | 添加一个条目 |
| `--remove` | string | 移除一个条目 |
| `--clear` | bool | 清空整个列表 |

**示例 1：添加忽略文件夹**

```bash
zap-publish config set-array ignore.folders --add logs
zap-publish config set-array ignore.folders --add temp
```

**示例 2：移除忽略文件夹并查看结果**

```bash
zap-publish config set-array ignore.folders --remove temp
zap-publish config list
```

**示例 3：清空忽略文件列表**

```bash
zap-publish config set-array ignore.files --clear
```

---

### 2.4 config get — 获取配置项

**用途**：查看指定配置项的当前值。

**用法**：

```bash
zap-publish config get <key>
```

**示例 1：获取服务器地址**

```bash
zap-publish config get server.url
# 输出: http://192.168.1.100:2000
```

**示例 2：获取项目名称**

```bash
zap-publish config get project.name
```

---

### 2.5 config list — 列出所有配置

**用途**：查看当前项目的所有配置项。

**用法**：

```bash
zap-publish config list [--json]
```

**示例 1：human 格式查看所有配置**

```bash
zap-publish config list
# server.url      = http://192.168.1.100:2000
# project.name    = myapp
# ignore.folders  = [logs temp]
# ignore.files    = [*.log .DS_Store]
# output.format   = human
```

**示例 2：JSON 格式输出**

```bash
zap-publish config list --json
```

---

### 2.6 config path — 显示配置文件路径

**用途**：显示当前项目的 `.updator/` 目录路径。

**用法**：

```bash
zap-publish config path
```

**示例**：

```bash
cd /path/to/myapp
zap-publish config path
# /path/to/myapp/.updator
```

---

### 2.7 project list — 列出所有项目

**用途**：获取服务端上所有项目的列表。

**使用场景**：查看服务端已有项目，或在多个项目中做选择。

**用法**：

```bash
zap-publish project list [--server <地址>]
```

**实现逻辑**：

1. 调用 `GET /api/project/get_all_projects` 获取所有项目
2. 以表格形式显示 ID、名称、标题、版本号、是否强制更新、创建日期

**示例 1：查看所有项目**

```bash
zap-publish project list --server http://192.168.1.100:2000
```

**示例 2：JSON 格式输出**

```bash
zap-publish project list --server http://192.168.1.100:2000 --json
```

---

### 2.8 project create — 创建新项目

**用途**：在服务端创建一个新项目。

**使用场景**：首次发布某个应用时，先在服务端创建项目记录。

**用法**：

```bash
zap-publish project create --name <名称> --title <标题> [--force-update] [--ignore-folders <逗号分隔>] [--ignore-files <逗号分隔>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--name` | string | **是** | 项目名称（唯一标识） |
| `--title` | string | **是** | 项目抬头（显示用） |
| `--force-update` | bool | 否 | 是否强制更新（默认 false） |
| `--ignore-folders` | string | 否 | 忽略的文件夹（逗号分隔） |
| `--ignore-files` | string | 否 | 忽略的文件（逗号分隔） |

**实现逻辑**：

1. 调用 `POST /api/project/create_project` 将项目信息发送到服务端
2. 服务端返回创建后的项目信息（含自动生成的初始版本号）

**示例 1：创建一个简单项目**

```bash
zap-publish project create --name myapp --title "我的应用"
```

**示例 2：创建带忽略规则的项目**

```bash
zap-publish project create --name game_client --title "游戏客户端" --force-update --ignore-folders "logs,cache" --ignore-files "*.log,.DS_Store"
```

---

### 2.9 project update — 更新项目配置

**用途**：更新已存在项目的配置（如标题、强制更新标志、忽略规则）。

**使用场景**：需要修改项目属性时调用。

**用法**：

```bash
zap-publish project update --name <名称> [--title <标题>] [--force-update] [--ignore-folders <逗号分隔>] [--ignore-files <逗号分隔>]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--name` | string | **是** | 项目名称（用于查找） |
| `--title` | string | 否 | 项目抬头 |
| `--force-update` | bool | 否 | 是否强制更新 |
| `--ignore-folders` | string | 否 | 忽略的文件夹 |
| `--ignore-files` | string | 否 | 忽略的文件 |

**示例 1：修改项目标题**

```bash
zap-publish project update --name myapp --title "我的应用 V2"
```

**示例 2：开启强制更新**

```bash
zap-publish project update --name myapp --force-update
```

---

### 2.10 project delete — 删除项目

**用途**：软删除服务端上的项目（标记 `is_deleted = true`，不物理删除数据）。

**使用场景**：项目下线或不再维护时调用。

**用法**：

```bash
zap-publish project delete --name <名称>
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--name` | string | **是** | 项目名称 |

**示例**：

```bash
zap-publish project delete --name old_project
```

---

### 2.11 project info — 查看项目详情

**用途**：查看指定项目的详细信息。

**用法**：

```bash
zap-publish project info [--name <名称> | --id <ID>]
```

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `--name` | string | 按名称查找项目 |
| `--id` | int | 按 ID 查找项目 |

**示例 1：按名称查看项目详情**

```bash
zap-publish project info --name myapp
```

**示例 2：按 ID 查看并 JSON 输出**

```bash
zap-publish project info --id 1 --json
```

---

### 2.12 status — 查看文件差异

**用途**：对比本地文件与服务端文件，显示新增、修改、删除、未变化的文件列表。

**使用场景**：在发布前查看与服务器之间的文件差异，确认哪些文件需要推送。

**用法**：

```bash
zap-publish status [--server <地址>] [--project <名称>]
```

**实现逻辑**：

1. 递归扫描本地项目目录（跳过 `.updator`、`.publish-cli` 及忽略规则匹配的路径），计算每个文件的 MD5+SHA256
2. 调用 `GET /api/file/get_all_files/{projectName}` 获取服务端文件列表
3. 比对两端文件（按相对路径+MD5），分类为：
   - **new**：本地有，服务端没有
   - **modified**：两边都有但 MD5 不同
   - **deleted**：本地已删除但服务端还有
   - **unchanged**：两边完全一致
4. 加载本地暂存区（`.updator/staging/staged-files.json`），将已暂存的文件从 unstaged 移到 staged 分类

**输出说明**：

```text
Changes staged for commit:
  (use "zap-publish reset <file>..." to unstage)

        modified:    src/main.go
        new:         config/new_setting.json

Changes not staged for commit:
  (use "zap-publish add <file>..." to stage)

        modified:    src/utils.go
        new:         assets/icon.png

Unchanged files:
        config/default.json
```

**示例 1：查看文件差异**

```bash
cd /path/to/myapp
zap-publish status --server http://192.168.1.100:2000 --project myapp
```

**示例 2：JSON 格式输出差异**

```bash
zap-publish status --server http://192.168.1.100:2000 --project myapp --json
```

---

### 2.13 diff — 详细对比文件差异

**用途**：查看具体文件的 MD5、大小等详细差异信息。

**使用场景**：需要深入查看某个文件的变更细节。

**用法**：

```bash
zap-publish diff [--file <相对路径>]
```

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `--file` | string | 指定要查看详情的文件路径 |

**示例 1：查看所有文件的详细差异**

```bash
zap-publish diff
```

**示例 2：查看指定文件的差异**

```bash
zap-publish diff --file src/main.go
```

---

### 2.14 add — 添加文件到暂存区

**用途**：将文件添加到暂存区（staging area），标记为待推送状态。

**使用场景**：类似 `git add`，选择要发布到服务端的文件。

**用法**：

```bash
zap-publish add [--all | <file1> <file2> ...]
```

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `--all` | bool | 添加所有变更文件（new + modified） |
| 位置参数 | string[] | 指定要添加的文件路径 |

**实现逻辑**：

- **`--all`** 模式：先执行 `status` 获取所有变更文件，然后将所有 `new` 和 `modified` 文件加入暂存区
- **指定文件** 模式：直接计算指定文件的 MD5 和大小，写入暂存区文件 `.updator/staging/staged-files.json`

**示例 1：添加所有变更文件**

```bash
zap-publish add --all
```

**示例 2：添加指定文件**

```bash
zap-publish add src/main.go config/settings.json
```

---

### 2.15 reset — 从暂存区移除文件

**用途**：将文件从暂存区中移除，取消推送标记。

**使用场景**：误添加了文件或决定本次不发布某些文件。

**用法**：

```bash
zap-publish reset [--all | <file1> <file2> ...]
```

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `--all` | bool | 清空整个暂存区 |
| 位置参数 | string[] | 指定要移除的文件路径 |

**示例 1：清空暂存区**

```bash
zap-publish reset --all
```

**示例 2：移除指定文件**

```bash
zap-publish reset config/settings.json
```

---

### 2.16 staged — 查看暂存区内容

**用途**：列出当前暂存区中的所有文件及其状态。

**使用场景**：在 push 之前确认暂存区中的文件是否正确。

**用法**：

```bash
zap-publish staged
```

**示例**：

```bash
zap-publish staged
# Staged files:
#   [modified] src/main.go  43284 bytes
#   [new]      config/new_setting.json  1024 bytes
```

---

### 2.17 push — 推送暂存区文件到服务端

**用途**：将暂存区中的文件上传到服务端，并创建版本记录。

**使用场景**：确认暂存区内容无误后，执行发布。

**用法**：

```bash
zap-publish push --version <版本号> --message <变更说明> [--dry-run] [--force]
```

**参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--version` | string | **是** | 新版本号（如 `1.0.1`） |
| `--message` | string[] | **是** | 变更说明（可多次指定） |
| `--set-force-update` | bool | 否 | 推送后设置强制更新 |
| `--after-apply-update-script` | string | 否 | 更新后执行的脚本路径 |
| `--dry-run` | bool | 否 | 仅校验不实际推送 |
| `--force` | bool | 否 | 跳过 MD5 复核强制上传 |

**实现逻辑**：

1. 加载暂存区文件列表
2. （非 `--force` 模式）调用 `staging.Verify()`：重新计算暂存区文件的 MD5，若与 add 时记录的不一致 → 提示"文件已修改，请重新 add"
3. 逐文件通过 `POST /api/file/upload_file`（multipart 上传）上传到服务端
4. 调用 `POST /api/project/publish_version` 创建版本变更日志记录
5. 成功后清空暂存区

**示例 1：正常推送**

```bash
zap-publish push --version 1.0.1 --message "修复登录闪退" --message "优化首页加载速度"
```

**示例 2：使用 dry-run 先行验证**

```bash
zap-publish push --version 1.0.1 --message "修复 bug" --dry-run
```

---

### 2.18 log — 查看版本变更日志

**用途**：查看项目的版本发布历史及每次发布的变更说明。

**使用场景**：查看项目版本演进历史、定位某次发布的具体变更。

**用法**：

```bash
zap-publish log [--limit <条数>]
```

**参数**：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--limit` | int | 20 | 显示最近 N 条记录 |

**实现逻辑**：

1. 调用 `GET /api/project/get_project_change_logs/{projectName}`
2. 按 ID 倒序排列，取最近 `--limit` 条
3. 逐条显示版本号、发布时间和变更说明列表

**示例 1：查看最近 5 条日志**

```bash
zap-publish log --limit 5
```

**示例 2：JSON 格式输出全部日志**

```bash
zap-publish log --limit 100 --json
```

---

### 2.19 server info — 查看服务端系统信息

**用途**：获取服务端服务器的操作系统、硬件、磁盘等信息。

**使用场景**：排查部署环境、确认服务端运行状态。

**用法**：

```bash
zap-publish server info [--server <地址>]
```

**实现逻辑**：

1. 调用 `GET /api/server/info` 获取服务端系统信息
2. 返回操作系统、平台、架构、Go 版本、CPU 信息、磁盘使用情况

**示例 1：查看服务端信息**

```bash
zap-publish server info --server http://192.168.1.100:2000
```

**示例 2：JSON 格式输出**

```bash
zap-publish server info --server http://192.168.1.100:2000 --json
```

---

## 附录

### A. 典型工作流

#### 更新客户端工作流

```bash
# 步骤 1：检查更新
zap-update.exe check_update

# 步骤 2：查看差异文件（可选）
zap-update.exe check_diff

# 步骤 3：下载更新
zap-update.exe download_update

# 步骤 4：应用更新（自动关闭进程 → 原子替换 → 启动主程序）
zap-update.exe apply_update

# 步骤 5：如果需要回滚
zap-update.exe list_rollback_versions
zap-update.exe rollback --version 1.0.0
```

#### 发布工作流

```bash
# 步骤 1：初始化配置（首次使用）
cd /path/to/project
zap-publish config init --project myapp --server http://192.168.1.100:2000

# 步骤 2：创建项目（服务端首次发布）
zap-publish project create --name myapp --title "我的应用"

# 步骤 3：查看文件差异
zap-publish status

# 步骤 4：添加文件到暂存区
zap-publish add --all

# 步骤 5：推送到服务端
zap-publish push --version 1.0.0 --message "首次发布"

# 或者使用一键发布
zap-publish publish --version 1.0.0 --message "首次发布"
```

### B. 状态码速查

| 状态 | 说明 |
|------|------|
| `new` | 本地新增文件，服务端不存在 |
| `modified` | 本地文件已修改 |
| `deleted` | 本地已删除，但服务端仍存在 |
| `unchanged` | 本地与服务端一致 |
| `downloaded` | 已下载待应用 |
| `applying` | 正在应用更新 |
| `applied` | 更新已应用 |

### C. 配置文件的存储位置

| 文件 | 所属工具 | 路径（相对于项目根目录） |
|------|----------|-------------------------|
| `shared.json` | 两者共用 | `.updator/shared.json` |
| `publish.json` | publish-cli 专有 | `.updator/publish.json` |
| `client.json` | client 专有 | `UpdateFolder/client.json` |
| `version.json` | client 专有 | `UpdateFolder/version.json` |
| `staged-files.json` | publish-cli 专有 | `.updator/staging/staged-files.json` |
