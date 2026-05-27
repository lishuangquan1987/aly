# 更新客户端设计文档

## 基本信息

- **定位**：命令行工具，方便与其他客户端应用集成
- **语言**：Go 1.10
- **兼容性**：兼容 XP 系统
不能使用go mod
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
main_exe_relative_path: "../Langtian.DCS.UI.exe"
must_close_process_name:
  - Langtian.DCS.UI
  - Langtian.DCS.FlowSetting
  - Langtian.DCS.EChartsDataDisplay
  - Langtian.DCS.Selection
  - langtian.dcs.login
  - langtian.dcs.tcpplugin_go
```

**字段说明**：

| 字段 | 说明 |
| --- | --- |
| `url` | 服务器地址 |
| `main_exe_relative_path` | 主程序相对路径（相对于 `client-updator.exe` 所在目录） |
| `must_close_process_name` | 更新时必须关闭的进程名列表 |

**优先级**：命令行参数 > client.yaml 配置值。`url`、`main_exe_relative_path`、`must_close_process_name` 三个参数均支持命令行覆盖。

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
├── go.mod
├── go.sum
├── main.go                    # 入口，CLI 参数解析
├── config/
│   ├── config.go              # 配置加载（client.yaml + 命令行参数合并）
│   └── version.go             # version.json 读写
├── client/
│   └── http_client.go         # 服务端 HTTP API 调用
├── cmd/
│   ├── check_update.go        # 检查更新
│   ├── check_diff.go          # 文件比对
│   ├── download_update.go     # 下载更新
│   ├── apply_update.go        # 执行更新
│   ├── list_rollback.go       # 列出可回滚版本
│   └── rollback.go            # 版本回滚
├── util/
│   ├── file.go                # 文件操作（复制、MD5、遍历）
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
| MD5 计算 | 标准库 `crypto/md5` | Go 1.10 内置 |
| 进程管理 | `golang.org/x/sys/windows` | Windows 进程操作 |

---

## 与服务端 API 对接

客户端调用服务端已有接口，**不需要新增 API**：

| 命令 | 调用接口 | 方法 |
| --- | --- | --- |
| check_update | `/api/project/get_project_change_logs/{id}` | GET |
| check_diff | `/api/file/get_all_files/{id}` | GET |
| download_update | `/api/file/get_all_files/{id}` + `/api/file/download_file?path=` | GET |

---

## 功能

### 1. 检查更新

```bash
client-updator.exe check_update [--url <url>] [--project-name <name>]
```

**调用接口**：`GET {url}/api/project/get_project_change_logs/{project_id}`

**返回值**：
- 有更新：`true:new_version`
- 无更新：`false:`（版本一致时冒号后为空）
- 错误：`false:错误信息`

**比对方法**：读取本地 version.json 的版本号与服务端最新变更日志的版本号比较，不一致则视为有更新。

---

### 2. 文件比对

```bash
client-updator.exe check_diff [--url <url>] [--project-name <name>]
```

**调用接口**：`GET {url}/api/file/get_all_files/{project_id}`

**功能**：列出客户端文件与服务端文件的 MD5 差异。

**输出格式**（tab 分隔，每行一条）：

```
relative/path/to/file	local_size	server_size	local_md5	server_md5
```

- 如果文件仅存在于本地（服务端无对应文件），`server_size` 和 `server_md5` 显示为 `-`
- 如果文件仅存在于服务端（本地无对应文件），`local_size` 和 `local_md5` 显示为 `-`

---

### 3. 下载更新

```bash
client-updator.exe download_update [--url <url>] [--project-name <name>] [--main-exe-path <path>]
```

**调用接口**：
- `GET {url}/api/file/get_all_files/{project_id}` 获取文件列表
- `GET {url}/api/file/download_file?path={file_path}` 逐文件下载

**执行步骤**：

1. 获取服务端文件详情列表（服务端自动忽略配置中排除的文件夹和文件）
2. 对比服务端文件与本地文件的 MD5 差异
3. 将差异文件下载至 `{main_exe_relative_path_folder}/update/{new_version}/` 文件夹中
4. 下载完成后逐文件校验 MD5，与服务器返回的 MD5 不一致则重试（最多 3 次），仍未通过则中止并报错
5. 将 `main_exe_relative_path_folder` 中的所有文件复制到 `{main_exe_relative_path_folder}/update/{new_version}/`，已存在的文件不覆盖，确保新版本目录是完整可运行的
6. 将 `main_exe_relative_path_folder` 中所有文件备份到 `{main_exe_relative_path_folder}/update/{current_version}/`
7. 更新 version.json：`version` 设为新版本号，`version_status` 设为 `downloaded`
8. 返回 `true/false`

---

### 4. 执行更新

```bash
client-updator.exe apply_update [--main-exe-path <path>] [--must-close-process-name <name1,name2>]
```

**执行步骤**：

1. 读取 version.json，如果 `version_status` 不为 `downloaded`，则返回 `false`
2. 关闭 `must_close_process_name` 中列出的所有进程（等待进程退出，超时后强制终止）
3. 将 `{main_exe_relative_path_folder}/update/{version}/` 中的所有文件复制到 `{main_exe_relative_path_folder}/`，覆盖已存在的文件
4. 更新 version.json：`version_status` 设为 `applied`
5. 返回 `true/false`

---

### 5. 列出可回滚版本

```bash
client-updator.exe list_rollback_versions
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
client-updator.exe rollback --version 1.0.1 [--main-exe-path <path>] [--must-close-process-name <name1,name2>]
```

**执行步骤**：

1. 关闭 `must_close_process_name` 中列出的所有进程（等待进程退出，超时后强制终止）
2. 将 `{main_exe_relative_path_folder}/update/{rollback_version}/` 中的所有文件复制到 `{main_exe_relative_path_folder}/`，覆盖已存在的文件
3. 更新 version.json：`version` 设为回滚目标版本号，`version_status` 设为 `applied`
4. 返回 `true/false`

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
