---
title: aly-client 命令参考
description: aly-client 更新器九个命令的完整参考，包含用途、签名、参数、返回字段、JSON 示例、副作用与错误边界。
---

# aly-client 命令参考

`aly-client` 是部署在终端 `UpdateFolder` 下的更新器。一次进程执行一个子命令，所有结果以 JSON 写到 stdout。本页按命令列出签名、参数、返回字段、示例与副作用；部署布局和宿主接入见[更新器概览](/docs/client/)，配置文件字段见[配置参考](/docs/client/config)。

下文示例统一使用 Windows 文件名 `aly-client.exe`。Linux 下可执行文件名为 `aly-client`，子命令与参数完全一致。

## 统一约定

### 输出信封

```json
{"isSuccess":true,"errorMsg":"","data":{}}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `isSuccess` | bool | 业务是否成功。**宿主必须以此判断结果** |
| `errorMsg` | string | 失败原因；成功时为空串 |
| `data` | any | 命令数据；无数据时为 `null` |

除 `download_update` 外，每个命令只输出一行 JSON。`download_update` 会先输出多行进度对象，最后一行才是最终结果，见[下载进度](#下载进度)。

### 退出码

| 退出码 | 含义 |
|---|---|
| `0` | 命令已执行完毕，**包括 `isSuccess=false` 的业务失败** |
| `1` | 未提供子命令、未知子命令；或 stdout JSON 序列化失败 |
| `2` | 参数解析失败（Go `flag.ExitOnError`），如非法 `--close-timeout`、未知选项 |

::: warning
不要用退出码判断业务成败。下载失败、没有待应用更新、锁被占用等都以退出码 `0` 结束，只在 JSON 的 `isSuccess` 与 `errorMsg` 中体现。宿主若只看退出码，会把失败当成功。
:::

### 配置加载与全局锁

配置加载顺序固定为：

1. `UpdateFolder/client.json`：`main_exe_relative_path` 与 `must_close_process_name`；
2. 由 `main_exe_relative_path` 推导主程序目录 `MainFolder`；
3. `MainFolder/.updator/shared.json`：`server_url`、`project_name` 与忽略/不复制规则；
4. 命令行参数覆盖（`--url`、`--project-name`、`--main-exe-path`）。

需要服务端配置的命令是 `check_update`、`check_diff`、`download_update`。缺少 `server_url` 时返回 `no server url configured`，缺少 `project_name` 时返回 `no project name configured`；`apply_update`、`rollback`、`list_rollback_versions`、`clean`、`check_self_update`、`self_update` 只操作本地文件，不需要联网。

全局更新锁位于 `UpdateFolder/.aly.lock`：

| 取锁命令 | 不取锁命令 |
|---|---|
| `download_update`、`apply_update`、`rollback`、`clean`、`self_update` | `check_update`、`check_diff`、`list_rollback_versions`、`check_self_update` |

锁被另一存活的更新进程持有时，命令输出：

```json
{"isSuccess":false,"errorMsg":"另一更新正在进行中，请稍后重试","data":null}
```

锁带有 30 分钟 TTL 和 5 分钟心跳；持有者崩溃后，下一次取锁会清理陈旧锁。

### version.json 状态机

版本状态只有三个有效值（空状态等价于 `applied`）：

```text
applied/空 ──download_update──> downloaded ──apply_update──> applying ──成功──> applied
                                      ▲                          │
                                      └──────── 失败/崩溃 ────────┘
```

`version_status=applying` 表示更新或回滚在途，下一次 `check_update` / `apply_update` 会走崩溃恢复。字段含义见[version.json](/docs/client/config#version-json)。

## 命令总览

| 命令 | 用途 | 联网 | 取全局锁 | 写 version.json | 典型调用方 |
|---|---|---|---|---|---|
| [`check_update`](#check-update) | 检查服务端是否有新版本 | 是 | 否 | 否 | 宿主、GUI、SDK |
| [`check_diff`](#check-diff) | 列出需要下载的差异文件 | 是 | 否 | 否 | 调试、可视化客户端 |
| [`download_update`](#download-update) | 下载差异文件并物化未变化文件 | 是 | 是 | 是 | 宿主、GUI、SDK |
| [`apply_update`](#apply-update) | 原子替换主目录并重启主程序 | 否 | 是 | 是 | 宿主、GUI、SDK |
| [`list_rollback_versions`](#list-rollback-versions) | 列出可回滚的历史快照 | 否 | 否 | 否 | GUI、SDK、运维 |
| [`rollback`](#rollback) | 回滚到指定历史版本 | 否 | 是 | 是 | GUI、SDK、运维 |
| [`clean`](#clean) | 清理历史快照与临时文件 | 否 | 是 | 否 | 运维、计划任务 |
| [`check_self_update`](#check-self-update) | 检查更新器自身是否需要更新 | 否 | 否 | 否 | SDK、GUI、宿主 |
| [`self_update`](#self-update) | 用主程序目录的新版更新器替换自身 | 否 | 是 | 否 | SDK、GUI、裸控制台宿主 |

## check_update

### 用途

查询服务端最新版本，返回是否需要下载或是否已有下载好的更新待应用。命令只读，不下载、不改写任何文件，宿主可高频调用（例如启动时和定时任务）。

### 签名

```bash
aly-client.exe check_update [--url <server_url>] [--project-name <project_name>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--url` | string | 否 | 空，沿用 `shared.json` 的 `server_url` | 本次调用临时覆盖服务端地址 |
| `--project-name` | string | 否 | 空，沿用 `shared.json` 的 `project_name` | 本次调用临时覆盖项目名称 |

### 行为

1. 读取 `client.json`、`shared.json` 与 `version.json`。
2. 按 `version_status` 分支：
   - `applied` 或空：请求项目信息与变更日志，取最高版本与服务端比较；服务端版本严格更高才上报更新。
   - `downloaded`：若 `rollback_previous` 非空（回滚中断），上报继续回滚；否则请求变更日志；服务端版本更高则要求重新下载；服务端版本相同且待应用产物丢失/残缺时要求重下补齐；相同且产物完好、或服务端版本更低时要求继续应用。
   - `applying`：无条件上报继续应用（崩溃恢复），并跳过联网判断。
   - 其它未知值：向 stderr 打印告警，按 `applied` 处理；只有 `download_update` 能把它重写为 `downloaded`。
3. 最新变更日志按版本号取最大，版本相同时取 `id` 最大的记录。
4. 版本比较是 semver 感知的：`1.0.0-beta < 1.0.0`，构建元数据 `+build` 不参与比较；纯数字旧格式保持原有行为。

### 返回字段

`data` 字段：

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `has_update` | bool | 总是 | 是否有需要处理的新版本或待应用更新 |
| `need_download_update` | bool | 总是 | `true` 表示需要先 `download_update`；`false` 表示只需 `apply_update` |
| `current_version` | string | 总是 | 主目录当前真实内容版本；`downloaded` 时取 `version_previous`；首次部署或未知时为空串；`applying` 且 `version_previous` 为空时回退为 `version` |
| `new_version` | string | 上报新版本时 | 目标版本；回滚续跑时是回滚目标 |
| `force_update` | bool | 需要下载的分支 | 服务端项目的强制更新标记；项目信息读取失败时为 `false` |
| `release_notes` | string[] | 服务端上报更高版本且变更日志非空时 | 服务端 `project_change_logs.logs` 原样透传，不做翻译或裁剪 |

`release_notes` 只在上报“需要下载的更高版本”时出现；离线、`applying` 崩溃恢复、回滚续跑、无更新时都不会出现。宿主和 GUI 用它展示“这次更新了什么”。

### 示例

有更新、需要下载：

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "has_update": true,
    "need_download_update": true,
    "current_version": "1.0.0",
    "new_version": "1.0.1",
    "force_update": false,
    "release_notes": ["修复：测长偶发偏差", "新增：点间距支持 2.5m"]
  }
}
```

已下载、只差应用：

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}
```

无更新：

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":false,"need_download_update":false,"current_version":"1.0.0"}}
```

已有下载产物但断网（不上报待应用更新，产物保留）：

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":false,"need_download_update":false,"current_version":"1.0.0"}}
```

`applying` 崩溃恢复（断网也必须上报）：

```json
{"isSuccess":true,"errorMsg":"","data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}
```

失败：

```json
{"isSuccess":false,"errorMsg":"no server url configured","data":null}
```

### 副作用

- 不写 `version.json`、不建目录、不取全局锁。
- 未知 `version_status` 只写 stderr，不写 `update.log`，避免高频调用刷爆日志。
- 联网失败也写 stderr 诊断信息，但对外仍返回 `isSuccess=true` 且 `has_update=false`。

### 边界与错误

- **离线语义**：`downloaded` 状态且服务端不可达（或没有任何变更日志）时不上报待应用更新，返回 `has_update=false`；已下载内容和 `version.json` 原样保留，联网后自动继续应用。这样避免断网时显示“可更新”、联网后状态又变化。
- **applying 例外**：崩溃恢复与回滚续跑必须在断网时照常上报，否则主目录已改名或回滚中断的机器永远无法自愈。
- **产物丢失**：`downloaded` 但待应用目录缺失/为空/文件数少于 `downloaded_file_count` 时，若服务端版本正好相同，返回 `has_update=true`、`need_download_update=true` 重下补齐；服务端版本更高则正常重下更高版本；服务端版本更低则仍上报继续应用，由 `apply_update` 拒绝并自愈。
- **异常状态**：`version_previous == version` 时不上报待应用更新，避免宿主反复触发必然被拒绝的 apply。
- **熔断**：同一待应用版本连续 apply 失败达到 3 次后，不再上报待应用更新；下载成功、apply 成功，或服务端发布更高版本并重新下载成功后复位。
- **已下载但服务端版本更低**：仍会继续应用本地已下载的更高版本，即服务端“下架”不能阻止已下载的升级；这是当前策略边界。
- 业务失败（未配置地址、`shared.json` 读取失败等）仍以退出码 `0` 结束，宿主必须读 `isSuccess`。

### 相关命令

[`download_update`](#download-update) · [`apply_update`](#apply-update) · [更新器概览](/docs/client/) · [配置参考](/docs/client/config)

## check_diff

### 用途

列出服务端有、本地缺失或 MD5 不同的文件，供 GUI 或排障时查看本次更新会下载哪些文件。命令不下载、不改写文件。

### 签名

```bash
aly-client.exe check_diff [--url <server_url>] [--project-name <project_name>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--url` | string | 否 | 空，沿用 `shared.json` 的 `server_url` | 本次调用临时覆盖服务端地址 |
| `--project-name` | string | 否 | 空，沿用 `shared.json` 的 `project_name` | 本次调用临时覆盖项目名称 |

### 行为

1. 调用 `GET /api/file/get_all_files/{projectName}` 获取服务端文件清单（含 MD5、SHA256）。
2. 递归扫描 `MainFolder`，计算本地文件的 MD5。
3. 逐个服务端文件比对：
   - 本地不存在：列入差异，`local_md5` 为空、`local_size` 为 `0`；
   - 本地存在但 MD5 不同：列入差异，并并行计算本地 SHA256；
   - MD5 一致：跳过。
4. 尽力获取最新变更日志，把最高版本填入 `new_version`。

只返回“服务端有而本地没有或不同”的文件。**仅本地存在、服务端已删除的文件不会返回**。

### 返回字段

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `new_version` | string | 总是 | 服务端最新版本；变更日志不可用时为空串 |
| `files` | array \| null | 总是 | 差异文件列表；无差异时为 `null` |

`files` 元素：

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `path` | string | 总是 | 相对 `MainFolder` 的路径，统一为正斜杠 |
| `local_md5` | string | 总是 | 本地 MD5；本地不存在时为空串 |
| `local_size` | int | 总是 | 本地文件大小；本地不存在时为 `0` |
| `local_sha256` | string | 本地存在且 MD5 不同 | 本地 SHA256；本地不存在时为空串 |
| `server_md5` | string | 总是 | 服务端 MD5 |
| `server_size` | int | 总是 | 服务端文件大小 |
| `server_sha256` | string | 总是 | 服务端 SHA256 |

### 示例

```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": {
    "new_version": "2.0.0",
    "files": [
      {
        "path": "app.exe",
        "local_md5": "abc123",
        "local_size": 43284,
        "local_sha256": "849384",
        "server_md5": "def456",
        "server_size": 5242880,
        "server_sha256": "884374"
      },
      {
        "path": "plugins/new.dll",
        "local_md5": "",
        "local_size": 0,
        "local_sha256": "",
        "server_md5": "xyz789",
        "server_size": 102400,
        "server_sha256": "88437486"
      }
    ]
  }
}
```

无差异：

```json
{"isSuccess":true,"errorMsg":"","data":{"new_version":"2.0.0","files":null}}
```

失败：

```json
{"isSuccess":false,"errorMsg":"get server files: ...","data":null}
```

### 副作用

- 不取全局锁，不写 `version.json`，不下载文件。
- 本地 MD5 扫描出现告警时，向 `UpdateFolder/check_diff.log` 追加日志。
- 只读访问服务端接口。

### 边界与错误

- 缺少 `server_url` / `project_name` 时返回失败。
- 服务端文件清单请求失败时整体失败；变更日志请求失败不阻断，`new_version` 为空串。
- 比较基准是 MD5；SHA256 仅作为附加信息返回。
- 无差异时 `files` 为 `null` 而非空数组，宿主解析时需兼容。
- 业务失败仍以退出码 `0` 结束。

### 相关命令

[`check_update`](#check-update) · [`download_update`](#download-update)

## download_update

### 用途

把差异文件下载到 `PackageFolder/ApplicationFolder_{newVersion}/`，不替换当前运行的程序。下载完成后会把主目录中未变化的文件物化到新版本目录，使后续 `apply_update` 只剩纯 rename。

### 签名

```bash
aly-client.exe download_update [--url <server_url>] [--project-name <project_name>] [--main-exe-path <relative_path>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--url` | string | 否 | 空，沿用 `shared.json` 的 `server_url` | 本次调用临时覆盖服务端地址 |
| `--project-name` | string | 否 | 空，沿用 `shared.json` 的 `project_name` | 本次调用临时覆盖项目名称 |
| `--main-exe-path` | string | 否 | 空，沿用 `client.json` 的 `main_exe_relative_path` | 临时覆盖主程序相对路径（相对 `UpdateFolder`） |

### 行为

1. 取全局锁，读取 `client.json`、`shared.json` 与 `version.json`。
2. 取服务端最高版本；若 `version_status=applying`，直接输出完成行 `data:null` 并返回，不下载、不改状态，由 `apply_update` 走崩溃恢复。
3. 若 `downloaded` 且版本相同、目标目录存在且非空，输出完成行 `data:null` 并返回；目标目录缺失或为空时视为产物丢失，允许同版本重下补齐。
4. 若目标版本不高于当前版本且不需要重下，返回 `already at latest version`。
5. 拉取服务端文件清单，与本地 `MainFolder` 的 MD5 比对；目标目录中已通过 MD5+SHA256 校验的文件也会跳过。只把真正需要下载的文件计入 `total`。
6. 在 `PackageFolder/ApplicationFolder_{newVersion}/` 逐文件下载：
   - 大文件（超过 100 MB）使用 `.part` 临时文件与 `Range` 断点续传；
   - 小文件同样先写 `.part`，完成后再改名；
   - 每个文件下载后校验 MD5 + SHA256，不匹配则删除并重试，最多 3 次。
7. 单文件 3 次失败不会中断整轮下载，记入失败清单继续下一个文件。全部跑完后统一返回失败，`failed_files` 列出失败文件；**不写 `version.json` 的待应用状态**，避免半成品被 `apply_update` 激活。重试时已成功文件会跳过，等效只重失败文件。
8. 所有文件成功后做物化收尾：把 `MainFolder` 中未变化的文件复制进版本目录，排除 `un_copy_folders` / `un_copy_files`。物化失败会让整轮下载失败。
9. 按服务端发布清单做差量删除同步（删除集 = 上次缓存清单 ∖ 本次清单），清理新版本目录中已被服务端删除的文件；清单或缓存不可用时保守跳过。
10. 写 `version.json`：`version_status=downloaded`，记录 `version`、`version_previous`、`downloaded_file_count`、`after_apply_update_script`，清空回滚/激活过渡字段，并把 `apply_fail_count` 复位为 0。

### 下载进度

`download_update` 的 stdout 是多行 JSON。进度行的 `data` 字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `index` | int | 当前差异文件序号；`BEGIN` 行为 0 |
| `total` | int | 差异文件总数（不含跳过文件） |
| `file` | string | 相对路径；`BEGIN` 行为空串 |
| `status` | string | `BEGIN` / `START` / `PROGRESS` / `DONE` / `FAIL` |
| `file_size` | int | 当前文件字节数 |
| `downloaded_bytes` | int | 已下载字节累计；旧版客户端可能没有该字段 |
| `total_bytes` | int | 差异文件总字节数；`BEGIN` 行预告 |
| `speed_bps` | int | 最近窗口平均吞吐（字节/秒） |
| `error` | string | `FAIL` 行的失败原因 |

`PROGRESS` 行按约 3 秒节流输出，保证长文件下载期间 stdout 有活动，宿主不会误判进程卡死。`FAIL` 行的 `isSuccess` 为 `false`。

最终完成行的 `data` 为 `null`。这是 SDK / UI 识别“下载完成”的信号，不要把它当异常。

### 返回字段

最终结果：

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `isSuccess` | bool | 总是 | 全部文件下载并物化成功时为 `true` |
| `errorMsg` | string | 总是 | 失败原因；成功时为空串 |
| `data` | object \| null | 总是 | 成功时为 `null`；部分文件失败时为对象 |
| `data.version` | string | `data` 非空时 | 目标版本 |
| `data.failed_files` | string[] | 有文件失败时 | 失败文件相对路径列表 |

### 示例

成功完成行：

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

部分文件失败：

```json
{
  "isSuccess": false,
  "errorMsg": "下载完成但有 2 个文件失败: a.dll, b.dll",
  "data": {"version": "2.0.0", "failed_files": ["a.dll", "b.dll"]}
}
```

通用失败（未配置地址、无变更日志、已是最新等）：

```json
{"isSuccess":false,"errorMsg":"already at latest version","data":null}
```

进度行示例：

```json
{"isSuccess":true,"errorMsg":"","data":{"index":0,"total":5,"file":"","status":"BEGIN","file_size":0,"downloaded_bytes":0,"total_bytes":734003200,"speed_bps":0}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":5,"file":"app.exe","status":"START","file_size":5242880,"downloaded_bytes":0,"total_bytes":734003200,"speed_bps":0}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":5,"file":"app.exe","status":"PROGRESS","file_size":5242880,"downloaded_bytes":2621440,"total_bytes":734003200,"speed_bps":1835008}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":5,"file":"app.exe","status":"DONE","file_size":5242880,"downloaded_bytes":5242880,"total_bytes":734003200,"speed_bps":2411724}}
{"isSuccess":true,"errorMsg":"","data":null}
```

### 副作用

- 创建 `PackageFolder/ApplicationFolder_{newVersion}/`，下载过程可能留下 `.part` 文件。
- 物化未变化文件、执行差量删除同步；更新（或首次创建）`UpdateFolder/release_manifest.json` 缓存。
- 写 `version.json`（`downloaded`、`version_previous`、`downloaded_file_count`、`after_apply_update_script`，清空 `rollback_previous` / `rollback_target` / `apply_stage`）。
- 持有 `UpdateFolder/.aly.lock`；向 `update.log` / `update_{version}_fail.log` 追加日志。
- 同版本重下（产物丢失）中途失败时，清理目标版本目录，避免留下半成品。

### 边界与错误

- `version_status=applying` 时输出 `data:null` 并直接返回，**不下载**；这是交给 `apply_update` 恢复的信号，宿主不要误以为下载完成。
- 目标目录存在但读不到（ACL、杀软锁、慢盘）时按“不可读”处理，跳过同版本重下，不误判为产物丢失。
- 已是最新版本、未配置服务器、服务端无变更日志、创建目录失败、拉取文件清单失败等返回 `isSuccess=false`、`data=null`。
- 物化失败时最后一条是 `FAIL` 进度行（`file` 为 `materialize`），不输出最终完成行；宿主应把该行视为失败。
- 服务端清单/缓存缺失时删除同步降级为“不删”，不会阻断下载。
- 下载成功以 `data:null` 结束；失败若带 `failed_files`，宿主应重试整条命令，已成功文件会跳过。
- 业务失败仍以退出码 `0` 结束。

### 相关命令

[`check_update`](#check-update) · [`apply_update`](#apply-update) · [`check_diff`](#check-diff) · [`clean`](#clean) · [配置参考](/docs/client/config)

## apply_update

### 用途

把已下载的新版本原子替换到主程序目录，写回 `applied` 状态，执行后置脚本（如有），然后启动主程序。命令不联网。

### 签名

```bash
aly-client.exe apply_update [--main-exe-path <relative_path>] [--close-timeout <seconds>] [--must-close-process-name <names>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--main-exe-path` | string | 否 | 空，沿用 `client.json` 的 `main_exe_relative_path` | 临时覆盖主程序相对路径 |
| `--close-timeout` | int | 否 | `30` | 进程关闭与目录占用重试的超时秒数 |
| `--must-close-process-name` | string | 否 | 空 | 逗号分隔的进程名，追加到 `client.json` 的 `must_close_process_name`；C# SDK 会显式传入 |

### 行为

1. 取全局锁，加载配置，机会式清理历史快照（保留最近 3 个）。
2. 按 `version_status` 分支：
   - `applied` 或空：返回 `no pending update to apply`，不替换。
   - `downloaded`：先校验现场，再进入替换。
   - `applying`：崩溃恢复，按磁盘现场补写 `applied`、重做剩余 rename 或完成回滚续跑。
   - 未知值：拒绝替换并报错，只有 `download_update` 能修复状态。
3. `downloaded` 的校验（任何拒绝都发生在写状态、关进程、改名之前）：
   - `version` 为空：拒绝并自愈为 `applied` + `version_previous`；
   - `version_previous == version`：拒绝替换（备份目录与版本目录同路径）；
   - 待应用目录缺失、为空、或文件数少于 `downloaded_file_count`：拒绝并自愈为 `applied` + 真实内容版本，等待重新下载；
   - 待应用目录存在但读不到：只报错，不做任何改动。
4. 写 `version_status=applying`。
5. 关闭 `must_close_process_name` 列出的进程（Windows 先发 `WM_CLOSE`，等待后强杀）；未列出的占用者由 rename 失败后的探测逻辑处理。
6. 原子替换（最多整体重试 3 次）：
   - 旧备份目录若存在，先旁移为 `.old`；
   - `MainFolder` 改名为 `ApplicationFolder_{version_previous}`；
   - 写入 `apply_stage=activating`，把 `ApplicationFolder_{version}` 改名为 `MainFolder`；
   - 任一步失败则尝试回滚，并把状态退回 `downloaded`（主目录已丢失但版本目录仍在时保持 `applying` 交给崩溃恢复）。
7. 成功：写 `version_status=applied`，清空 `rollback_previous` / `rollback_target` / `apply_stage`，复位 `apply_fail_count`，删除不再引用的无版本号备份目录。
8. 执行 `after_apply_update_script`（相对 `MainFolder`，同步执行，默认 60 秒超时，结果写 `update.log`；调用结束后写 `.updator/after_apply_{version}.done` 标记，不校验脚本退出码，脚本应可幂等执行）。
9. 启动主程序（工作目录设为主程序目录）。

### 返回字段

`data` 在成功和业务失败时都是 `null`：

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `isSuccess` | bool | 总是 | 替换与状态写入是否成功；主程序启动失败只写日志，不改变结果 |
| `errorMsg` | string | 总是 | 失败原因；成功时为空串 |
| `data` | null | 总是 | 固定为 `null` |

### 示例

成功：

```json
{"isSuccess":true,"errorMsg":"","data":null}
```

没有待应用更新：

```json
{"isSuccess":false,"errorMsg":"no pending update to apply","data":null}
```

待应用目录缺失/不完整（已自愈）：

```json
{"isSuccess":false,"errorMsg":"待应用版本目录缺失、为空或不完整: ...\\ApplicationFolder_2.0（已回退为 applied/1.0，等待重新下载）","data":null}
```

替换失败：

```json
{"isSuccess":false,"errorMsg":"backup rename failed: ...","data":null}
```

异常状态：

```json
{"isSuccess":false,"errorMsg":"版本状态异常：Version == VersionPrevious(2.0)，拒绝替换","data":null}
```

### 副作用

- 写 `version.json`（`applying` → `applied`，`apply_stage`，`apply_fail_count`，清空回滚过渡字段）。
- 关闭进程；对 `ApplicationFolder` 与版本目录做 rename；可能产生 `.old` 旁移残留（成功后清理）。
- 运行后置脚本，写 `.updator/after_apply_{version}.done` 标记。
- 启动主程序。
- 持有 `.aly.lock`，机会式 prune 历史快照并写 `update.log`。
- 失败兜底在主目录仍存在时会启动旧版本主程序，保证应用仍可用；主目录已丢失但版本目录还在时保持 `applying`，交给崩溃恢复。

### 边界与错误

- `applied` 或空状态返回“无待应用更新”，不会出现“把主目录改名到自己”的假成功。
- 待应用目录缺失/为空/文件数不足时拒绝替换并自愈，不会把当前目录自拷贝进版本目录后报成功。
- 目录存在但读不到时只报错、不改状态，避免丢弃可能有效的待应用包。
- 替换失败时状态退回 `downloaded` 并递增 `apply_fail_count`；连续失败达到 3 次后 `check_update` 不再上报待应用更新（熔断）。
- 主目录已丢失但版本目录还在时保持 `applying`，下一次 `apply_update` / `check_update` 走崩溃恢复，不会永久失去应用目录。
- 后置脚本同步执行；脚本失败或超时只写 `update.log`，不影响 `isSuccess`。脚本应幂等，因为崩溃恢复可能重跑未完成的脚本。
- 主程序启动失败（路径错误、权限不足）只写日志，命令仍可能返回成功；宿主不要把 `isSuccess=true` 理解为“主程序一定已拉起”。
- 业务失败（包括无待应用更新）仍以退出码 `0` 结束。

### 相关命令

[`download_update`](#download-update) · [`check_update`](#check-update) · [`rollback`](#rollback) · [version.json](/docs/client/config#version-json)

## list_rollback_versions

### 用途

扫描 `PackageFolder` 下的历史版本快照目录，返回可回滚版本列表与当前版本。命令不联网，不取全局锁。

### 签名

```bash
aly-client.exe list_rollback_versions [--main-exe-path <relative_path>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--main-exe-path` | string | 否 | 空，沿用 `client.json` 的 `main_exe_relative_path` | 临时覆盖主程序相对路径 |

### 行为

1. 由 `main_exe_relative_path` 推导主文件夹名（例如 `ApplicationFolder`）。
2. 列出 `PackageFolder` 下以 `{主文件夹名}_` 开头的子目录。
3. 过滤出后缀合法的版本号（纯数字单段或多段、semver 预发布/构建元数据形式），且必须是目录。
4. `downloaded` 状态下，待应用版本目录是下载产物而不是历史快照，从列表中排除；`current_version` 取 `version_previous`（主目录真实内容版本）。
5. 返回 `current_version` 与 `versions`。

### 返回字段

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `current_version` | string | 总是 | 当前主目录内容版本；未知时为空串 |
| `versions` | array \| null | 总是 | 可回滚版本列表；没有时为 `null` |

### 示例

```json
{"isSuccess":true,"errorMsg":"","data":{"current_version":"1.0.1","versions":["1.0.0","0.9.0"]}}
```

没有历史快照：

```json
{"isSuccess":true,"errorMsg":"","data":{"current_version":"1.0.1","versions":null}}
```

读取配置失败：

```json
{"isSuccess":false,"errorMsg":"load config: ...","data":null}
```

### 副作用

- 只读目录与 `version.json`，不取全局锁，不写任何文件。
- 不联网。

### 边界与错误

- 版本目录必须存在且是目录；文件名以 `.old` 等结尾的目录不会进入列表。
- 单段纯数字版本也会被视为合法版本。
- 没有可回滚版本时 `versions` 为 `null`，宿主需兼容。
- 业务失败仍以退出码 `0` 结束。

### 相关命令

[`rollback`](#rollback) · [`clean`](#clean) · [更新器概览](/docs/client/#安全与回滚)

## rollback

### 用途

把当前主目录原子替换为指定的历史版本快照，写回 `applied` 状态并启动主程序。命令不联网。

### 签名

```bash
aly-client.exe rollback --version <target_version> [--main-exe-path <relative_path>] [--close-timeout <seconds>] [--must-close-process-name <names>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--version` | string | **是** | — | 目标回滚版本号 |
| `--main-exe-path` | string | 否 | 空，沿用 `client.json` 的 `main_exe_relative_path` | 临时覆盖主程序相对路径 |
| `--close-timeout` | int | 否 | `30` | 进程关闭与目录占用重试的超时秒数 |
| `--must-close-process-name` | string | 否 | 空 | 逗号分隔的进程名，追加到 `client.json` 的 `must_close_process_name` |

### 行为

1. 取全局锁，校验 `--version` 非空，加载配置与 `version.json`。
2. 机会式 prune 历史快照，并保护本次 CLI 目标版本不被清理。
3. 拒绝未知 `version_status`；`applying` 且 `rollback_previous` 为空表示有一次未完成的 apply，拒绝回滚并提示先执行 `apply_update`。
4. `applying` 且 `rollback_previous` 非空表示回滚中断，委托崩溃恢复续跑。
5. 目标版本目录必须存在；`downloaded` 状态下不允许把“待应用下载版本”当回滚目标。
6. 备份目录名取“替换前 MainFolder 的真实内容版本”：`downloaded` 时用 `version_previous`，回滚续跑时用 `rollback_previous`。
7. 写 `version_status=applying`，记录 `rollback_previous` 与 `rollback_target`，关闭进程。
8. 原子交换：旁移旧备份 → 主目录改名为旧版本备份 → 目标版本目录改名为主目录。进入替换阶段后失败会恢复回滚前状态（不一律写 `applied`），并在主目录仍存在时启动当前版本主程序。
9. 成功：写 `version=目标版本`、`version_previous=回滚前版本`、`version_status=applied`，清空回滚过渡字段，清除发布清单缓存，执行后置脚本（如有），启动主程序。

### 返回字段

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `isSuccess` | bool | 总是 | 回滚是否成功 |
| `errorMsg` | string | 总是 | 失败原因；成功时为空串 |
| `data` | object \| null | 总是 | 成功时为对象，失败时为 `null` |
| `data.version` | string | 成功时 | 回滚到的目标版本 |

### 示例

成功：

```json
{"isSuccess":true,"errorMsg":"","data":{"version":"1.0.0"}}
```

目标版本不存在：

```json
{"isSuccess":false,"errorMsg":"version 0.0.1 not found","data":null}
```

缺少 `--version`：

```json
{"isSuccess":false,"errorMsg":"--version is required","data":null}
```

有一次未完成的 apply：

```json
{"isSuccess":false,"errorMsg":"有一次未完成的更新（applying），请先执行 apply_update 完成恢复后再回滚","data":null}
```

### 副作用

- 写 `version.json`：进入 `applying` 时写 `rollback_previous` / `rollback_target`；成功写 `applied` 并清空过渡字段。
- 目录改名与 `.old` 残留；成功后清理旁移残留。
- 清除 `release_manifest.json` 差量删除缓存。
- 执行后置脚本并启动主程序。
- 持有 `.aly.lock`，机会式 prune 快照并写 `update.log`。
- 替换阶段失败且主目录仍存在时，会启动当前版本主程序，保持应用可用。

### 边界与错误

- 目标版本目录不存在、缺少 `--version`、未知状态、未完成的 apply、目标是待应用下载版本、目标就是当前活动版本，都会拒绝并返回失败。
- 进入替换阶段后的失败会恢复回滚前状态（`preStatus`），避免“状态说已应用、磁盘还是旧内容”的错配；入口守卫的拒绝不改状态。
- 回滚成功后若服务端版本仍更高，下一轮 `check_update` 会再次升级；当前没有 rollback pin 策略，需要运维侧同步处理服务端版本。
- 后置脚本失败、主程序启动失败只写 `update.log`，不改变 `isSuccess`；宿主不要把成功理解为“主程序一定已拉起”。
- 业务失败仍以退出码 `0` 结束。

### 相关命令

[`list_rollback_versions`](#list-rollback-versions) · [`apply_update`](#apply-update) · [`clean`](#clean) · [更新器概览](/docs/client/#安全与回滚)

## clean

### 用途

显式回收磁盘：清理超过保留数量的历史版本快照、旁移残留、孤儿备份目录、下载临时文件与过期日志。`prune` 只在 apply/rollback 入口机会式触发，客户端长期不更新时需要 `clean` 主动清理。

### 签名

```bash
aly-client.exe clean [--keep <N>] [--all]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--keep` | int | 否 | `0` | 保留最近 N 个版本快照；**值为 0（或负数）时按默认值 3 处理** |
| `--all` | bool | 否 | `false` | 激进模式：只保留当前版本保护集；等效 `--keep 1`，但输出 `mode=all` |

### 行为

1. 取全局锁（`clean` 会改动 `PackageFolder`，不能与在途更新并发），加载配置。
2. `--all`：以 `keep=1` 调用 prune，只保留最新一个可回滚快照。
3. 否则 `keep = --keep`；小于 1 时改用默认值 `3`，清理更旧的快照。
4. 保护集永不删除：`MainFolder`、`version`、`version_previous`、`rollback_target`（以及 rollback 入口显式传入的目标）。
5. 两种模式都顺带清理：
   - 版本目录旁的 `.old` / `.old.N` 残留；
   - 孤儿无版本号备份目录 `{MainFolder}_`；
   - 版本快照目录内的 `.part` / `.tmp`，`.updator` 与包根目录顶层的临时文件；
   - 超过 30 天的 `update_*` 日志（含 `update.log`、`update_*_fail.log`）。
6. 不递归清理 `MainFolder` 内的 `.tmp`，避免误删业务程序持有的临时数据。

### 返回字段

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `mode` | string | 总是 | `keep` 或 `all` |
| `keep` | int | `mode=keep` 时 | 实际保留的快照数量 |

### 示例

默认清理（保留 3 个）：

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"keep","keep":3}}
```

`--keep 0` 同样按 3 处理：

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"keep","keep":3}}
```

只保留 1 个：

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"keep","keep":1}}
```

激进清理：

```json
{"isSuccess":true,"errorMsg":"","data":{"mode":"all"}}
```

锁被占用（失败）：

```json
{"isSuccess":false,"errorMsg":"另一更新正在进行中，请稍后重试","data":null}
```

### 副作用

- 删除超出保留数量的版本快照目录。
- 删除 `.old*` 残留、孤儿备份目录、`.part` / `.tmp` 临时文件、30 天前的更新日志。
- 向 `update.log` 记录删除动作。
- 持有 `.aly.lock`；不写 `version.json`。

### 边界与错误

- 锁被占用时返回失败；`clean` 不会绕过锁。
- `--keep 0` 表示默认 3，不是“保留 0 个”；负数同样回退到 3。
- `--all` 优先于 `--keep`，输出不含 `keep`。
- 清理是尽力而为：单个目录 ACL 失败不会导致整体失败，但可能留下残留。
- 业务失败仍以退出码 `0` 结束。

### 相关命令

[`apply_update`](#apply-update) · [`rollback`](#rollback) · [`download_update`](#download-update)

## check_self_update

### 用途

比较“正在运行的更新器副本”与“主程序目录中随版本分发的新版更新器”的 SHA256，判断更新器自身是否需要更新。命令只检测，不替换。

### 签名

```bash
aly-client.exe check_self_update [--main-exe-path <relative_path>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--main-exe-path` | string | 否 | 空，沿用 `client.json` 的 `main_exe_relative_path` | 临时覆盖主程序相对路径 |

### 行为

1. 定位 `MainFolder/aly-client.exe`（Linux 为 `aly-client`）。
2. 若主程序目录没有更新器（首次部署）→ `need_update=false`。
3. 计算当前运行副本与主程序目录副本的 SHA256。
4. 两者不同 → `need_update=true`；相同 → `false`。

新版本来源只有一条：发布方把更新器打进版本包，`download_update` 下载并由 `apply_update` 替换主目录，之后主目录里才有新版更新器。

### 返回字段

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `need_update` | bool | 总是 | 是否需要执行 `self_update` |

### 示例

需要更新：

```json
{"isSuccess":true,"errorMsg":"","data":{"need_update":true}}
```

不需要更新，或主程序目录没有更新器：

```json
{"isSuccess":true,"errorMsg":"","data":{"need_update":false}}
```

读取文件失败：

```json
{"isSuccess":false,"errorMsg":"...","data":null}
```

### 副作用

- 不取全局锁，不写文件，不联网。
- 只读取两个可执行文件并计算哈希。

### 边界与错误

- 主程序目录没有更新器（首次部署）不是错误，返回 `need_update=false`。
- 任一文件哈希失败返回 `isSuccess=false`。
- 检测与执行分离：`check_self_update` 不会自动替换，执行必须调用 `self_update`。
- 业务失败仍以退出码 `0` 结束。

### 相关命令

[`self_update`](#self-update) · [自更新设计](/docs/self-update)

## self_update

### 用途

用主程序目录里随版本分发的新版更新器，替换 `UpdateFolder` 中正在运行的自身副本。与 `check_self_update` 配对，构成自更新闭环。

### 签名

```bash
aly-client.exe self_update [--main-exe-path <relative_path>]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `--main-exe-path` | string | 否 | 空，沿用 `client.json` 的 `main_exe_relative_path` | 临时覆盖主程序相对路径 |

### 行为

1. 取全局锁 `.aly.lock`；另一更新进行中时返回“另一更新正在进行中”。
2. 定位 `MainFolder/aly-client.exe`（源）与当前运行副本。
3. 源缺失（首次部署）→ 跳过，返回 `updated=false`、`reason=source_missing`，不算错误。
4. SHA256 一致 → 跳过，顺带清理 `.new` / `.old` 残留，返回 `updated=false`、`reason=already_up_to_date`。
5. 不一致 → 先备后换：
   - 复制源到 `aly-client.exe.new` 并校验；
   - 把运行中的 exe 改名为 `aly-client.exe.old`（运行中文件允许改名）；
   - `.new` 原子落位；失败则回滚 `.old`；
   - 复制/校验失败时保留原文件并清理 `.new`。
6. 当前进程继续从旧 inode 运行直至退出；宿主下次拉起即为新版。

### 返回字段

| 字段 | 类型 | 出现条件 | 说明 |
|---|---|---|---|
| `updated` | bool | 总是 | 是否完成了替换 |
| `reason` | string | 总是 | `replaced` / `already_up_to_date` / `source_missing` |
| `old_path` | string | `updated=true` 时 | 被替换下来的旧副本备份路径（`.old`） |

### 示例

替换成功：

```json
{"isSuccess":true,"errorMsg":"","data":{"updated":true,"reason":"replaced","old_path":"...\\aly-client.exe.old"}}
```

内容一致：

```json
{"isSuccess":true,"errorMsg":"","data":{"updated":false,"reason":"already_up_to_date"}}
```

主程序目录没有更新器：

```json
{"isSuccess":true,"errorMsg":"","data":{"updated":false,"reason":"source_missing"}}
```

锁被占用：

```json
{"isSuccess":false,"errorMsg":"另一更新正在进行中，请稍后重试","data":null}
```

### 副作用

- 可能产生 `.new` 中转文件与 `.old` 备份；成功替换后旧副本保留到下一次“内容一致”调用时清理。
- 持有 `.aly.lock`，向 `update.log` 记录替换结果。
- 不写 `version.json`，不联网。

### 边界与错误

- 源缺失不是错误；内容一致时幂等跳过。
- 两次 rename 之间存在极短的目标缺失窗口；若进程恰在此被杀，`.old` 仍在，下一次 UI/SDK 启动会从主目录重新落位，或手工把 `.old` 改回 `aly-client.exe`。
- 锁被占用时直接失败；宿主应记录后下次启动再试，不要当启动失败。
- **不要双触发**：UI 部署已由 UI 自己委派 `self_update`；SDK 部署由 SDK 构造函数调用；只有裸控制台宿主才需要显式调用。重复调用虽然幂等，但会多一次进程创建与锁竞争。
- 升级兼容：旧版更新器不认识 `self_update` 时，UI/SDK 会回退为直接复制主目录副本；锁被占用（命令返回 JSON 失败）时不回退。
- 业务失败仍以退出码 `0` 结束。

### 相关命令

[`check_self_update`](#check-self-update) · [自更新设计](/docs/self-update) · [GUI 前端](/docs/client/ui)