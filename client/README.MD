# 更新客户端设计文档

## 基本信息

- **定位**：命令行工具，方便与其他客户端应用集成
- **语言**：Go 1.10（兼容 XP，不使用 go modules）
- **兼容性**：Windows XP 及以上
- **构建**：`set GOARCH=386 && set GO111MODULE=off && go build -ldflags="-s -w"`
- **退出码约定**：0 = 成功，1 = 失败，2 = 参数错误

---

## 配置文件

### version.json

位于 `client-updator.exe` 同级目录下，记录当前版本信息：

```json
{
  "version": "1.0.0",
  "version_status": "applied"
}
```

**version_status 字段说明**：

| 值 | 含义 |
| --- | --- |
| `downloaded` | 已下载，待应用 |
| `applied` | 已应用 |

### client.yaml

默认配置文件，位于 `client-updator.exe` 同级目录下：

```yaml
project_name: "otdr_client"
url: "http://127.0.0.1:2000"
mirror_url: ""
main_exe_relative_path: "../Langtian.DCS.UI.exe"
must_close_process_name:
  - Langtian.DCS.UI
  - Langtian.DCS.FlowSetting
  - Langtian.DCS.EChartsDataDisplay
  - Langtian.DCS.Selection
  - langtian.dcs.login
  - langtian.dcs.tcpplugin_go
post_update_script: ""
```

**字段说明**：

| 字段 | 说明 |
| --- | --- |
| `url` | 服务器地址 |
| `mirror_url` | 备用服务器地址（可选，下载失败时自动切换） |
| `main_exe_relative_path` | 主程序相对路径（相对于 `client-updator.exe` 所在目录） |
| `must_close_process_name` | 更新时必须关闭的进程名列表 |
| `post_update_script` | 更新完成后执行的脚本路径（可选，相对于主程序根目录） |

**优先级**：命令行参数 > client.yaml 配置值。`url`、`mirror_url`、`main_exe_relative_path`、`must_close_process_name` 均支持命令行覆盖。

---

## 约定

- 所有相对路径均相对于 `client-updator.exe` 所在目录解析
- `main_exe_relative_path_folder` = `main_exe_relative_path` 所在的文件夹（即主程序根目录）
- `update/` 目录位于 `main_exe_relative_path_folder` 下，用于存放各版本文件

---

## 项目结构

```
client/
├── README.MD
├── main.go                    # 入口，CLI 参数解析
├── config/
│   ├── config.go              # 配置加载（client.yaml + 命令行参数合并）
│   └── version.go             # version.json 读写
├── http_client/
│   └── http_client.go         # 服务端 HTTP API 调用
├── cmd/
│   ├── check_update.go        # 检查更新
│   ├── check_diff.go          # 文件比对
│   ├── download_update.go     # 下载更新
│   ├── apply_update.go        # 执行更新
│   ├── list_rollback.go       # 列出可回滚版本
│   └── rollback.go            # 版本回滚
├── util/
│   ├── file.go                # 文件操作（复制、SHA-256、MD5、遍历）
│   └── process.go             # 进程管理（查找、关闭、等待）
└── model/
    ├── dto.go                 # 与服务端一致的请求/响应 DTO
    └── diff.go                # 文件差异结构
```

### 依赖库选择

| 用途 | 库 | 说明 |
| --- | --- | --- |
| CLI 参数解析 | 标准库 `flag` | Go 1.10 内置，无需额外依赖 |
| YAML 解析 | `gopkg.in/yaml.v2` | 兼容 Go 1.10 |
| HTTP 请求 | 标准库 `net/http` | Go 1.10 内置 |
| JSON 序列化 | 标准库 `encoding/json` | Go 1.10 内置 |
| SHA-256 计算 | 标准库 `crypto/sha256` | Go 1.10 内置 |
| MD5 计算 | 标准库 `crypto/md5` | Go 1.10 内置 |
| 进程管理 | `golang.org/x/sys/windows` | Windows 进程操作 |

**哈希策略**：下载文件后同时校验 MD5 和 SHA-256。MD5 用于快速比对文件级差异；SHA-256 用于下载完成后的完整性校验，防止篡改。

---

## 安全设计

### 传输安全

- `url` 为 `http://` 时标准错误流输出警告，但允许继续（兼容内网环境）
- 建议生产环境使用 `https://`，服务端证书校验默认开启（可通过 `--insecure` 跳过）

### 文件完整性

- 服务端返回文件列表时附带每个文件的 MD5 和 SHA-256
- 客户端下载后**同时校验 MD5 和 SHA-256**，任一不匹配则重试（最多 3 次）
- 所有校验失败的文件记录到 `update_{version}_fail.log`，便于排查

### 版本防降级

- 不自动安装低于当前版本号的更新（可由 `--allow-downgrade` 覆盖）
- 读取服务端 `force_update` 字段，强制更新时跳过用户确认

---

## 与服务端 API 对接

客户端调用服务端已有接口，**不需要新增 API**：

| 命令 | 调用接口 | 方法 |
| --- | --- | --- |
| check_update | `/api/project/get_project_change_logs/{id}` | GET |
| check_diff | `/api/file/get_all_files/{id}` | GET |
| download_update | `/api/file/get_all_files/{id}` + `/api/file/download_file?path=` | GET |

服务端文件列表返回的每个文件需包含 `sha256` 字段用于完整性校验。

---

## 功能

### 1. 检查更新

```bash
client-updator.exe check_update [--url <url>] [--project-name <name>]
```

**调用接口**：`GET {url}/api/project/get_project_change_logs/{project_id}`

**返回值**：
- 有更新：`true:new_version [force]`（`[force]` 仅强制更新时出现）
- 无更新：`false:`（版本一致时冒号后为空）
- 错误：`false:错误信息`

**比对方法**：
- 读取本地 version.json 的版本号与服务端最新变更日志的版本号比较
- 使用语义化版本比较（`major.minor.patch`），兼容纯数字三段式（`1.0.0`）和带 `v` 前缀（`v1.0.0`）
- 检查服务端 `force_update` 字段，输出中标注是否为强制更新

---

### 2. 文件比对

```bash
client-updator.exe check_diff [--url <url>] [--project-name <name>]
```

**调用接口**：`GET {url}/api/file/get_all_files/{project_id}`

**功能**：列出客户端文件与服务端文件的差异。

**输出格式**（tab 分隔，每行一条）：

```
relative/path/to/file	local_size	server_size	local_md5	server_md5
```

- 如果文件仅存在于本地（服务端无对应文件），`server_size` 和 `server_md5` 显示为 `-`
- 如果文件仅存在于服务端（本地无对应文件），`local_size` 和 `local_md5` 显示为 `-`

---

### 3. 下载更新

```bash
client-updator.exe download_update [--url <url>] [--mirror-url <url>] [--project-name <name>] [--main-exe-path <path>] [--silent]
```

**调用接口**：
- `GET {url}/api/file/get_all_files/{project_id}` 获取文件列表
- `GET {url}/api/file/download_file?path={file_path}` 逐文件下载（支持 `Range` 头断点续传）

**执行步骤**：

1. 获取服务端文件详情列表（服务端自动忽略配置中排除的文件夹和文件）
2. 对比服务端文件与本地文件的 MD5 差异
3. 将差异文件下载至 `{main_exe_relative_path_folder}/update/{new_version}/` 文件夹中
   - 支持断点续传：下载中断后再次执行，已下载完整的文件跳过，未完成的从断点继续
   - 主地址下载失败时自动切换 `mirror_url`（如已配置）
4. 下载完成后逐文件校验 MD5 + SHA-256，与服务器返回值不一致则重试（最多 3 次），仍未通过则中止并报错
5. 将 `main_exe_relative_path_folder` 中的所有文件复制到 `{main_exe_relative_path_folder}/update/{new_version}/`，已存在的文件不覆盖，确保新版本目录是完整可运行的
6. 将 `main_exe_relative_path_folder` 中所有文件备份到 `{main_exe_relative_path_folder}/update/{current_version}/`
7. 更新 version.json：`version` 设为新版本号，`version_status` 设为 `downloaded`
8. 退出码 0 表示成功，1 表示失败

**进度输出**：

非 `--silent` 模式下，标准输出逐行打印进度（便于调用方解析）：
```
progress:12/45 files
progress:downloading app.exe 25%
progress:verifying app.exe sha256 ok
progress:45/45 files complete
```
`--silent` 模式下不输出任何内容，仅靠退出码判断成功/失败。

---

### 4. 执行更新

```bash
client-updator.exe apply_update [--main-exe-path <path>] [--must-close-process-name <name1,name2>] [--close-timeout <seconds>] [--silent]
```

**执行步骤**：

1. 读取 version.json，如果 `version_status` 不为 `downloaded`，退出码 1
2. 关闭 `must_close_process_name` 中列出的所有进程：
   - 先发送 `WM_CLOSE` 消息请求正常退出
   - 等待超时（默认 30 秒，可通过 `--close-timeout` 配置）
   - 超时后调用 `TerminateProcess` 强制终止
3. **原子替换**：
   - 将 `{main_exe_relative_path_folder}/update/{version}/` 中的所有文件复制到临时目录 `{main_exe_relative_path_folder}_swap/`
   - 复制完成后，将 `{main_exe_relative_path_folder}` 重命名为 `{main_exe_relative_path_folder}_old/`
   - 将 `{main_exe_relative_path_folder}_swap/` 重命名为 `{main_exe_relative_path_folder}`
   - 清理 `{main_exe_relative_path_folder}_old/`
   - 如任一步失败，回滚：将 `{main_exe_relative_path_folder}_old/` 重命名回来，保证主程序目录不被损坏
4. 更新 version.json：`version_status` 设为 `applied`
5. 执行 `post_update_script`（如已配置），脚本失败不影响更新结果，仅记录日志到 `update.log`
6. 退出码 0 表示成功，1 表示失败

---

### 5. 列出可回滚版本

```bash
client-updator.exe list_rollback_versions [--main-exe-path <path>]
```

**功能**：扫描本地 `{main_exe_relative_path_folder}/update/` 目录，列出所有可用的回滚版本号。

**输出格式**（每行一个版本号）：

```
1.0.0
1.0.1
```

---

### 6. 版本回滚

```bash
client-updator.exe rollback --version 1.0.1 [--main-exe-path <path>] [--must-close-process-name <name1,name2>] [--close-timeout <seconds>]
```

**执行步骤**：

1. 校验 `{main_exe_relative_path_folder}/update/{rollback_version}/` 目录存在且所有文件 MD5 完整，否则退出码 1
2. 关闭 `must_close_process_name` 中列出的所有进程（同 apply_update 的两阶段关闭策略）
3. 将目录内所有文件以**原子替换**方式复制到 `{main_exe_relative_path_folder}/`（同 apply_update 步骤 3）
4. 更新 version.json：`version` 设为回滚目标版本号，`version_status` 设为 `applied`
5. 退出码 0 表示成功，1 表示失败

---

## 目录结构示例

```
Langtian.DCS/                        # main_exe_relative_path_folder（主程序根目录）
├── Langtian.DCS.UI.exe              # 主程序
├── config/
├── libs/
└── update/
    ├── 1.0.0/                        # v1.0.0 备份（download_update 时备份生成）
    │   ├── Langtian.DCS.UI.exe
    │   └── ...
    └── 1.0.1/                        # v1.0.1 完整版本（download_update 时生成）
        ├── Langtian.DCS.UI.exe
        └── ...

updater/                              # client-updator.exe 所在目录
├── client-updator.exe
├── client.yaml
└── version.json
```
