# 更新客户端设计文档
## 基本信息

- **定位**：命令行工具，方便与其他客户端应用集成
- **语言**：Go 1.10（兼容 XP，不使用 go modules）
- **兼容性**：Windows XP 及以上
- **构建**：`set GOARCH=386 && set GO111MODULE=off && go build -ldflags="-s -w"`

- **输出约定**：标准json，禁止流式输出，执行完之后，一次性输出
  ```json
  {
    "isSuccess":true,
    "errorMsg":"",
    "data":...
  }
  ```

## 部署后的目录结构

```
PackageFolder/
├── ApplicationFolder/              # 主程序文件夹
│   ├── main_application.exe
│   ├── .updator/
│   │   └── shared.json             # 共用配置（server_url、project_name、ignore规则）
│   ├── zap-update.exe           # updator 副本（用于自更新比对）
│   └── ...
├── UpdateFolder/                   # 更新程序文件夹
│   ├── zap-update.exe
│   ├── client.json                 # 精简配置（main_exe_relative_path + must_close_process_name）
│   ├── version.json                # 版本状态机
│   └── logs/
├── ApplicationFolder_1.0.0/        # 主程序文件夹 1.0.0 版本（回滚用）
│   ├── main_application.exe
│   ├── .updator/
│   │   └── shared.json
│   └── ...
├── ApplicationFolder_1.0.1/        # 主程序文件夹 1.0.1 版本
│   └── ...
└── ApplicationFolder_1.0.2/        # 主程序文件夹 1.0.2 版本
    └── ...
```

---

## 配置文件

### client.json（精简版）

位于 `zap-update.exe` 同级目录（UpdateFolder），包含定位 ApplicationFolder 和关闭进程所需的字段：

```json
{
  "main_exe_relative_path": "../ApplicationFolder/main_application.exe",
  "must_close_process_name": ["main_application", "plugin_host"]
}
```

### shared.json

位于 `ApplicationFolder/.updator/shared.json`，zap-publish 和 client 共用。随构建产物上传到 server，client 更新时自动同步：

```json
{
  "server_url": "http://127.0.0.1:2000",
  "project_name": "myapp",
  "ignore_folders": ["logs", "temp", ".git"],
  "ignore_files": ["*.log", ".DS_Store"]
}
```

| 字段 | 说明 |
| --- | --- |
| `server_url` | 服务器地址 |
| `project_name` | 项目名称（服务端保证唯一） |
| `ignore_folders` | apply_update 时不复制的文件夹（路径前缀匹配） |
| `ignore_files` | apply_update 时不复制的文件（支持 glob 通配符） |

### version.json

位于 `zap-update.exe` 同级目录（UpdateFolder），记录当前版本信息：

```json
{
  "version_previouse": "0.0.9",
  "version": "1.0.0",
  "version_status": "applied",
  "after_apply_update_script": "post_update.bat"
}
```

**version_status 状态流转**：

```
applied --> (download_update 成功) --> downloaded
                                     |
                                     ├--> (apply_update 开始) --> applying
                                     |                              |
                                     |     ┌────────────────────────┘
                                     |     |
                                     |     ├--> (apply_update 成功) --> applied
                                     |     |
                                     |     └--> (崩溃/断电) --> 下次启动时
                                     |         检测到 applying 状态
                                     |         --> 自动恢复
                                     |
                                     └--> (下载后不 apply，再次 check_update)
                                          --> 检测到 downloaded 状态
                                          --> 直接返回 has_update=true
```

| 值 | 含义 |
| --- | --- |
| `downloaded` | 已下载，待应用 |
| `applying` | 正在应用更新（崩溃后用于恢复） |
| `applied` | 已应用 |

### 配置加载流程

每个命令启动时的配置加载顺序：

1. 读取 `client.json`（UpdateFolder）--> 获取 `main_exe_relative_path`
2. 通过 `main_exe_relative_path` 推导出 `ApplicationFolder` 路径
3. 读取 `ApplicationFolder/.updator/shared.json` --> 获取 `server_url`、`project_name`、`ignore_folders`、`ignore_files`
4. 命令行参数覆盖以上所有配置

**优先级**：CLI 参数 > `.updator/shared.json` > `client.json`

---

## 约定

- `mainFolder` = `main_exe_relative_path` 所在的文件夹（即 `ApplicationFolder/`）
- `main_exe_folder_name` = mainFolder 的文件夹名称（如 `ApplicationFolder`）

---

## 项目结构

```
client/
├── README.MD
├── main.go                    # 入口，CLI 参数解析
├── config/
│   ├── config.go              # 配置加载（client.json + .updator/shared.json）
│   └── version.go             # version.json 读写
├── client/
│   └── http_client.go         # 服务端 HTTP API 调用
├── cmd/
│   ├── common.go              # 公共工具函数（loadFullConfig、输出、进程管理）
│   ├── check_update.go        # 检查更新
│   ├── check_diff.go          # 文件比对
│   ├── download_update.go     # 下载更新
│   ├── apply_update.go        # 应用更新
│   ├── list_rollback.go       # 列出可回滚版本
│   ├── rollback.go            # 版本回滚
│   └── check_self_update.go   # 自更新检查
├── util/
│   ├── file.go                # 文件操作（复制、SHA-256、MD5、遍历）
│   └── process.go             # 进程管理（查找、关闭、等待）
└── model/
    └── dto.go                 # 请求/响应 DTO
```

### 依赖库

| 用途 | 库 | 说明 |
| --- | --- | --- |
| CLI 参数解析 | 标准库 `flag` | Go 1.10 内置 |
| YAML 解析 | 已移除（统一使用 JSON） | 之前使用 `gopkg.in/yaml.v2` |
| HTTP 请求 | 标准库 `net/http` | Go 1.10 内置 |
| JSON 序列化 | 标准库 `encoding/json` | Go 1.10 内置 |
| SHA-256 计算 | 标准库 `crypto/sha256` | Go 1.10 内置 |
| MD5 计算 | 标准库 `crypto/md5` | Go 1.10 内置 |
| 进程管理 | `golang.org/x/sys/windows` | Windows 进程操作 |

### 文件完整性

- 服务端返回文件列表时附带每个文件的 MD5 和 SHA-256
- 下载后**同时校验 MD5 和 SHA-256**，任一不匹配则重试（最大 3 次）
- 所有校验失败的文件记录到 `update_{version}_fail.log`

---

## 与服务端 API 对接

| 命令 | 调用接口 | 方法 |
| --- | --- | --- |
| check_update | `/api/project/get_project_by_name/{projectName}` + `/api/project/get_project_change_logs/{projectName}` | GET |
| check_diff | `/api/project/get_project_by_name/{projectName}` + `/api/file/get_all_files/{projectName}` | GET |
| download_update | `/api/project/get_project_by_name/{projectName}` + `/api/file/get_all_files/{projectName}` + `/api/file/download_file?path=` | GET |
| check_self_update | 无（纯本地 SHA-256 比对） | -- |

> **注意**：所有接口已从 `{projectId}` 迁移为 `{projectName}`。服务端保证 projectName 唯一。在重构之前，check_update / check_diff / download_update 先调用 `GetAllProjects` 拉全量列表再在本地过滤，现已改用独立的 `/api/project/get_project_by_name/{projectName}` 接口按名称精确查询。

---

## 功能

### 1. 检查更新

注意：此步骤需检查 version.json 中的 status 状态。如果为 `downloaded` 或 `applying`，则直接返回有更新，版本为当前 version.json 中记录的版本号。

```bash
zap-update.exe check_update [--url <url>] [--project-name <name>]
```

**调用接口**：
- `GET {url}/api/project/get_project_by_name/{projectName}`（获取项目信息）
- `GET {url}/api/project/get_project_change_logs/{projectName}`（获取变更日志）

**实现**：
1. 加载配置 + 读取 version.json
2. 如果 `version_status` 为 `downloaded` 或 `applying`：有未完成的更新 --> 直接返回 `has_update=true`（current_version = 上一版本，new_version = 已下载的版本）
3. `GetProjectByName()` --> 获取项目信息和 `project.force_update`（独立接口，不再拉全量列表后本地过滤）
4. `GetProjectChangeLogs()` --> 获取最新日志版本号
5. `compareVersion()` 逐段数值比较：服务端版本 > 本地版本 --> `has_update=true`

**对比方法**：读取本地 version.json 的版本号与服务端最新变更日志的版本号比较，不一致则视为有更新。**若本地不存在 version.json**，能获取到服务端新版本信息则认为有更新（`current_version` 返回空字符串）。

**返回 data**：

有更新：
```json
{
  "isSuccess": true, "errorMsg": "",
  "data": { "has_update": true, "current_version": "1.0.0", "new_version": "1.0.1", "force_update": false }
}
```

无更新：
```json
{
  "isSuccess": true, "errorMsg": "",
  "data": { "has_update": false, "current_version": "1.0.0" }
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `has_update` | bool | 是否有新版本 |
| `current_version` | string | 本地当前版本号 |
| `new_version` | string（可选） | 新版本号，仅 `has_update=true` 时出现 |
| `force_update` | bool（可选） | 是否强制更新，仅 `has_update=true` 时出现。`true` 时调用方应自动下载更新并提示重启；`false` 时由用户确认是否更新 |

---

### 2. 文件比对

注意：此步骤调用者可跳过，当调用者检测到有新版本时，可以直接调用第 3 步进行更新。此步骤只是为了方便可视化更新客户端查看差异文件。

```bash
zap-update.exe check_diff [--url <url>] [--project-name <name>]
```

**调用接口**：`GET {url}/api/file/get_all_files/{projectName}`

**功能**：列出需要从服务端下载的文件列表。只返回服务端存在且与本地 MD5 不同的文件（包括本地不存在的文件）。**不列出**仅本地存在、服务端不存在的文件。

**返回 data**：
```json
{
  "isSuccess": true, "errorMsg": "",
  "data": {
    "new_version": "1.0.1",
    "files": [
      { "path": "app.exe", "local_md5": "abc123", "local_size": 43284, "local_sha256": "849384", "server_md5": "def456", "server_size": 5242880, "server_sha256": "884374" },
      { "path": "new.dll", "local_md5": "", "local_size": 0, "local_sha256": "", "server_md5": "xyz789", "server_size": 102400, "server_sha256": "88437486" }
    ]
  }
}
```

| 字段 | 说明 |
| --- | --- |
| `new_version` | 服务端最新版本号 |
| `files` | 差异文件列表 |
| `files[].path` | 文件相对路径 |
| `files[].local_md5` | 本地文件 MD5（`new` 时为空） |
| `files[].server_md5` | 服务端文件 MD5 |
| `files[].server_size` | 服务端文件大小（字节） |

---

### 3. 下载更新

注意：此步骤只下载更新，不复制文件。当本地文件夹 `./{main_exe_folder_name}_{new_version}` 中存在要下载的文件且 MD5/SHA256 校验通过（可能是以前下载了部分文件），则跳过此文件下载。

```bash
zap-update.exe download_update [--url <url>] [--project-name <name>] [--main-exe-path <path>]
```

**调用接口**：
- `GET {url}/api/file/get_all_files/{projectName}` 获取文件列表
- `GET {url}/api/file/download_file?path={file_path}` 逐文件下载

**执行步骤**：

1. 获取服务端文件详情列表（服务端自动忽略配置中排除的文件夹和文件）
2. 对比服务端文件与**本地 ApplicationFolder** 中文件的 MD5 差异，得出需要下载的文件列表（diff 基准为当前运行的 ApplicationFolder，而非 `ApplicationFolder_{version}` 半成品目录）
3. 遍历差异文件列表，逐文件下载至 `./{main_exe_folder_name}_{new_version}` 文件夹中：
   - **下载前先检查目标目录**：若 `ApplicationFolder_{version}` 中已存在同名文件且 MD5/SHA-256 与服务端一致，则跳过此文件
   - **大文件断点续传**：文件 > 100MB 时，下载前检查目标目录是否存在同名文件或 `.part` 临时文件
     - 若存在且大小 < 服务端大小，发送 `Range: bytes={size}-` 从断点继续下载
     - 下载中写入 `.part` 临时文件（在目标目录中），完成后重命名为正式文件名
   - 文件 ≤100MB 直接全量下载
4. 下载完成后逐文件校验 MD5 + SHA-256，与服务器返回值不一致则重试（最大 3 次），仍未通过则中止并返回错误
5. 更新 version.json（**仅在 status 非 `downloaded` 时执行，防止重复调用覆盖 `version_previouse`**）：
   - `version_previouse` 设为当前 `version` 的值（记录本次更新前的版本号）
   - `version` 设为新版本号（从接口中获取）
   - `version_status` 设为 `downloaded`
   - `after_apply_update_script` 从服务端变更日志同步

**返回 data**：

成功：`{"isSuccess": true, "errorMsg": "", "data": null}`

失败：`{"isSuccess": false, "errorMsg": "app.exe 校验失败，已重试 3 次", "data": null}`

---

### 4. 应用更新（原子替换）

```bash
zap-update.exe apply_update [--main-exe-path <path>] [--close-timeout <seconds>]
```

**执行步骤**：

1. 读取 version.json，检查 `version_status`：
   - 若为 `applied`：返回错误（没有待应用的更新）
   - 若为 `applying`：说明上次应用中途崩溃，尝试恢复：
     - 若 `ApplicationFolder`（主程序目录）存在且完整：说明步骤 4 尚未执行到重命名，重新执行步骤 4
     - 若 `ApplicationFolder` 不存在但 `ApplicationFolder_{version}` 存在：说明旧目录已被重命名走、新目录尚未改过来，直接将 `ApplicationFolder_{version}` 重命名为 `ApplicationFolder`，然后执行步骤 6
   - 若为 `downloaded`：正常执行后续步骤
2. 将 `version_status` 设为 `applying`（标记开始应用）
3. 关闭 `client.json` 中 `must_close_process_name` 列出的所有进程：
   - 先发送 `WM_CLOSE` 消息请求正常退出
   - 等待超时（默认 30 秒，可通过 `--close-timeout` 配置）
   - 超时后调用 `TerminateProcess` 强制终止
4. **原子替换**：
   - 将 `{mainFolder}` 的内容全部（除去 `ignore_folders`/`ignore_files` 匹配的文件和文件夹）复制到 `./{main_exe_folder_name}_{version}` 中，确保 `./{main_exe_folder_name}_{version}` 中是一个完整可以运行的程序（`ignore_folders`/`ignore_files` 来自 `.updator/shared.json`）
   - 将 `./{main_exe_folder_name}` 重命名为 `./{main_exe_folder_name}_{version_previouse}`
   - 将 `./{main_exe_folder_name}_{version}` 重命名为 `./{main_exe_folder_name}`
   - **若任一步失败**：将 `version_status` 回设为 `downloaded`（允许重试），返回错误
5. 更新 version.json：`version` 设为新版本号，`version_previouse` 设为旧版本号，`version_status` 设为 `applied`
6. 执行 `version.json` 中 `after_apply_update_script`（如已配置），脚本失败不影响更新结果，仅记录日志到 `update.log`
7. **启动主程序**：以 `main_exe_relative_path` 为路径启动主程序 exe

**返回 data**：

成功：`{"isSuccess": true, "errorMsg": "", "data": null}`

失败：`{"isSuccess": false, "errorMsg": "原子替换失败，已回滚", "data": null}`

---

### 5. 列出可回滚版本

```bash
zap-update.exe list_rollback_versions [--main-exe-path <path>]
```

**功能**：扫描 `PackageDir/` 下所有匹配 `{folder_name}_*` 的目录，验证后缀是合法的版本号格式（如 `1.0.0`）。

**返回 data**：
```json
{ "isSuccess": true, "errorMsg": "", "data": { "current_version": "1.0.1", "versions": ["1.0.0"] } }
```

---

### 6. 版本回滚

```bash
zap-update.exe rollback --version 1.0.0 [--main-exe-path <path>] [--close-timeout <seconds>]
```

**执行步骤**：与第 4 步（应用更新）相同的原子替换操作。

> 与 apply_update 的区别：rollback 的目标版本目录是之前 apply 时保存的完整快照，不需要 CopyDirWithExclude 补全文件。

**返回 data**：

成功：`{"isSuccess": true, "errorMsg": "", "data": {"version": "1.0.0"}}`

失败：`{"isSuccess": false, "errorMsg": "回滚目标版本目录不存在", "data": null}`

---

### 7. 自更新检查

```bash
zap-update.exe check_self_update [--main-exe-path <path>]
```

**功能**：比对自身与 `ApplicationFolder/zap-update.exe` 的 SHA-256，判断 updator 是否需要更新。

**原理**：发布者每次打包时，将当前版本的 `zap-update.exe` 复制一份放入主程序目录，命名为 `zap-update.exe`。当主程序更新后，`zap-update.exe` 变成了新版，而 `UpdateFolder/zap-update.exe` 仍是旧版，比对不一致即为需要更新。

**行为**：
- 若 `ApplicationFolder/zap-update.exe` 不存在（首次部署），视为不需要更新
- 若存在且 SHA-256 一致，不需要更新
- 若存在且 SHA-256 不一致，需要更新

**返回 data**：

需要更新：`{ "isSuccess": true, "errorMsg": "", "data": { "need_update": true } }`
不需要更新：`{ "isSuccess": true, "errorMsg": "", "data": { "need_update": false } }`

**调用方（main app）后续动作**：
收到 `need_update: true` 后，将 `ApplicationFolder/zap-update.exe` 复制到 `UpdateFolder/zap-update.exe` 覆盖旧版（此时 updator 已退出，文件锁释放，可安全覆盖）。

---

## 待优化项 (Future Improvements)

以下为设计方案评审中识别的问题，当前暂不处理，留待后续迭代优化。

### 文件与目录管理

| # | 问题 | 说明 |
|---|------|------|
| 1 | 已删除文件无法被更新掉 | check_diff / download_update 只下载服务端有而本地 MD5 不同的文件，不处理服务端已删除、本地仍残留的文件。这会导致废弃 DLL / 配置文件一直残留，可能引发版本冲突或安全漏洞 |
| 2 | 无旧版本清理机制 | list_rollback_versions 只能列出，没有对应的删除/清理命令。长期使用后 ApplicationFolder_0.0.x 会无限积累，浪费磁盘空间 |
| 3 | 回滚无文件完整性校验 | rollback 只扫描目录名，不校验目标目录内文件的 MD5/SHA-256。如果用户手动删除了某个版本目录中的文件，回滚后会得到一个残缺的应用 |
| 4 | 非版本目录可能被误识别 | list_rollback_versions 扫描 ApplicationFolder_* 时，如果用户创建了 ApplicationFolder_backup 等非版本目录，也可能被当成版本号列出。目前已加版本号格式校验（`X.Y.Z` 格式） |

### 安全与签名

| # | 问题 | 说明 |
|---|------|------|
| 5 | 纯 HTTP + 无签名校验 | 只校验 MD5/SHA-256，无数字签名或代码签名。如果攻击者控制了服务器或实施 MITM，可以同时替换文件和对应的哈希值，校验形同虚设。建议至少加 RSA/ECDSA 签名校验（服务端私钥签名文件哈希，客户端内置公钥验签） |

### 进程与并发

| # | 问题 | 说明 |
|---|------|------|
| 6 | WM_CLOSE 对控制台程序无效 | 只发 WM_CLOSE 消息，对无消息循环的控制台程序 / Windows 服务无效，只能等超时后强杀。建议对控制台进程使用 AttachConsole + CTRL_C_EVENT |
| 7 | 无并发锁/互斥机制 | 没有文件锁、Mutex 等机制防止同时运行两个 zap-update.exe 实例，可能破坏 version.json 和文件系统状态 |
| 8 | 关闭进程后可能被看门狗重启 | 关闭进程后、文件操作前的时间窗口内，如果有守护进程或服务自动重启了目标进程，后续重命名会因文件被占用而失败 |

### 下载安全

| # | 问题 | 说明 |
|---|------|------|
| 9 | 无文件大小/总下载量上限 | 没有对单个文件大小和总下载量做限制。如果服务端被入侵返回伪造的巨大文件列表，客户端磁盘会被写满 |
