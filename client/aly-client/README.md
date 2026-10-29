# aly-client-sdk — 更新客户端 SDK

供 `aly-client`（可执行程序）引用的共享库，包含配置管理、HTTP API 客户端、文件工具、进程管理等模块。

## 模块

| 目录 | 说明 |
|------|------|
| `config/` | `client.json` / `shared.json` / `version.json` 加载与状态管理 |
| `client/` | HTTP API 客户端（调用服务端接口） |
| `model/` | JSON DTO 数据结构 |
| `util/` | 文件操作（MD5/SHA256/拷贝/遍历）、进程管理 |
| `test/` | Mock 服务端（用于本地测试） |

## 依赖

| 用途 | 库 | 说明 |
|------|-----|------|
| HTTP 请求 | 标准库 `net/http` | Go 1.10 内置 |
| JSON 序列化 | 标准库 `encoding/json` | Go 1.10 内置 |
| SHA-256 | 标准库 `crypto/sha256` | Go 1.10 内置 |
| MD5 | 标准库 `crypto/md5` | Go 1.10 内置 |
| 进程管理 | `golang.org/x/sys/windows` | Windows API |

## 兼容性

- Go 1.10（兼容 Windows XP，不使用 Go Modules）
- GOARCH=386
- 100% 标准库 + golang.org/x/sys/windows

## stdout JSON 协议（宿主集成说明）

`aly-client.exe` 的所有命令通过 stdout 输出 JSON 行与宿主（UI / C# SDK）通信。每行一个独立 JSON 对象，格式统一：

```json
{"isSuccess": true, "errorMsg": "", "data": { ... }}
```

- `isSuccess`：本行代表操作是否成功。**结果行**（每条命令的最后一行）为 false 时 `errorMsg` 携带失败原因；
- `errorMsg`：失败原因（成功时为空串）；
- `data`：负载，随命令与行类型不同。

### download_update 的三类行

下载过程逐行输出进度，最后一行是结果：

**进度行**（`data` 含 `index`+`status` 键）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` / `total` | int | 当前/差异文件总数（只计需下载的文件） |
| `file` | string | 当前文件相对路径 |
| `status` | string | `START` / `DONE` / `FAIL` / `BEGIN` |
| `file_size` | int64 | 当前文件字节数 |
| `downloaded_bytes` | int64 | **已累计下载字节数**（字节级进度，omitempty） |
| `total_bytes` | int64 | **差异文件总字节数**（BEGIN 行预告全量，omitempty） |
| `speed_bps` | int64 | **3 秒滑动窗口速度（字节/秒）**（omitempty） |
| `error` | string | FAIL 行的失败原因 |

`downloaded_bytes` / `total_bytes` / `speed_bps` 为新增可选字段：旧版宿主解析时忽略即可，无字段时宿主应回退到 `index/total` 文件计数展示。

**完成行**（`data: null` 且 `isSuccess: true`）：下载全部完成、version.json 已写入待应用状态。

**失败结果行**（`isSuccess: false`，`data` 为 `DownloadUpdateData`）：含失败清单，如单文件 3 次重试仍失败时 `failed_files` 数组列出全部失败文件；`errorMsg` 为人话摘要（"下载完成但有 N 个文件失败: …"）。

### 其他命令的结果行

| 命令 | data 负载 |
|------|-----------|
| `check_update` | `CheckUpdateData`（`has_update`/`need_download_update`/`force_update`/版本号/`release_notes`） |
| `apply_update` | 成功 `null`（主程序已被拉起）；失败 `null` + `errorMsg` |
| `list_rollback_versions` | `RollbackListData`（`versions` 数组 + `current_version`） |
| `rollback -version x` | `RollbackData`（`version`） |
| `clean [--keep N] [--all]` | `CleanData`（`mode` + `keep`） |
| `check_self_update` | `CheckSelfUpdateData`（`need_update`） |

### 契约要点（宿主必须遵守）

1. **逐行读 stdout 到 EOF**；非 JSON 行跳过不中断；
2. **结果行为准**：stdout 有结果行时进程退出码不参与判定（可能非 0），成败只看 `isSuccess`；
3. 进度行 `index/total` 只计**差异文件**（本地已匹配或断点续传已存在的文件不输出），宿主进度条不应按全量文件数折算；
4. 单文件失败不中断整体下载（失败清单在最终结果行统一下发，重试时已成功文件全部跳过）。
