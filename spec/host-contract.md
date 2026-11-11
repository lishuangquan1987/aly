# aly-client 宿主行为契约

- **状态**：规范性（normative）
- **协议版本**：`1.0.0`（见 `spec/aly-protocol.schema.json` 的 `x-protocol-version`）
- **适用对象**：所有语言/框架的宿主 SDK 实现者，以及 `aly-client-ui` 等内置调用方

本文件规定 `aly-client` 子进程协议里 **schema 表达不了** 的部分：调用方式、超时、
状态机、确认语义、错误处理与部署约束。字段名、类型、必填性以
`spec/aly-protocol.schema.json` 为准，两者冲突时以 schema 为准并同时修正本文件。

> **关键词**：**必须/不得** 表示强制约束；**应该** 表示推荐做法，偏离需说明理由。

---

## 1. 进程调用约定

宿主通过子进程调用 `aly-client`（Windows 为 `aly-client.exe`，Linux 为 `aly-client`），
一次进程执行一个子命令。

| 设置 | 要求 |
|---|---|
| 工作目录 | 更新器所在目录（`UpdateFolder/`）。`client.json`、`version.json`、`.aly.lock` 都以它为基准解析 |
| shell | **不得**经由 shell 启动（避免引号/转义与安全注入问题） |
| 控制台窗口 | **不得**弹出（Windows `CreateNoWindow` / 无 `console`） |
| stdout | 仅输出 UTF-8 JSON，一行一个对象；**不得**混入日志 |
| stderr | 非结构化诊断信息，可忽略或写入日志 |
| stdin | **不得**写入 |
| 编码 | stdout/stderr 一律按 UTF-8 解码 |

日志由 `aly-client` 自己写到 `UpdateFolder/update.log` 等文件；宿主不要从 stdout 读日志。

---

## 2. 输出与退出码

### 2.1 信封

除 `download_update` 的中间进度行外，每个命令只输出一行：

```json
{"isSuccess":true,"errorMsg":"","data":{}}
```

| 字段 | 判定 |
|---|---|
| `isSuccess` | **唯一**的业务成败依据 |
| `errorMsg` | 失败原因；成功时为空串 |
| `data` | 载荷；无载荷/下载完成时为 `null` |

### 2.2 退出码

| 退出码 | 含义 | 宿主动作 |
|---|---|---|
| `0` | 命令执行完毕，**包括 `isSuccess=false` 的业务失败** | 读 JSON 判定 |
| `1` | 未提供/未知子命令，或 stdout 序列化失败 | 视为实现错误，记录并上报 |
| `2` | 参数解析失败（非法 `--close-timeout`、未知选项等） | 检查宿主拼参，不要重试 |

**禁止只用退出码判断成功**：下载失败、无待应用更新、锁被占用等都以 `0` 结束。
退出码非 0 但 stdout 有合法 JSON 时，信任 JSON（客户端可能用非 0 返回业务结果）。

---

## 3. 超时与重试

`aly-client` 自身不设总时长上限；超时是宿主侧责任。默认值如下：

| 场景 | 超时 | 说明 |
|---|---|---|
| `check_update`、`check_self_update`、`self_update`、`list_rollback_versions` | **30 s** | 短命令，超时杀进程并报错 |
| `download_update` | **无总时长上限**；停流看门狗 **1800 s** | 连续无任何 stdout 输出即判死；大文件下载期间每 3 s 至少输出一行 `PROGRESS` |
| `rollback` | **600 s** | 含目录改名、等待主进程退出、崩溃恢复；30 s 会把回滚留在中间态 |
| `apply_update` | **不等待** | 拉起独立进程后立即返回；30 s 内不要重复拉起（锁在它手里） |

重试节奏：

| 情形 | 间隔 |
|---|---|
| `check_update` 网络/业务失败 | 5 s 后重试 |
| 下载失败（非 `already at latest version`） | 1 s 后重试本轮 |
| 更新循环内未预期异常 | 5 s 后继续 |
| `apply_update` 拉起之后 | 30 s 退避，再继续检查 |

空闲轮询：

- 默认间隔 **300 s**；配置值 `<=0` 按 300 处理，`<60` 按 60 处理。
- 每轮实际睡眠 = 间隔 + **0–60 s 随机抖动**（避免客户端群同时打服务端）。
- 失败重试不受空闲间隔影响（固定 5 s），睡眠必须可被取消（建议每秒检查一次取消信号）。

**看门狗判定的“输出”包括任何一行 stdout**（进度行、最终行）；不是只认 `DONE`。

---

## 4. 更新状态机

```text
None ──check: has_update && need_download──▶ DiscoveredUpdate
                                                 │
                        （非强更：RequestDownloadUpdate）
                                                 ▼
                                          DownloadingUpdate
                                                 │
                                                 ▼
                                          DownloadedUpdate
                                                 │
                         （非强更：RequestApplyUpdate）
                                                 ▼
                                            ApplyUpdate ──▶ 重启主程序

下载完成但服务器不可达：状态复位为 None，已下载产物保留在磁盘
```

规则：

1. 状态值固定为 `None` / `DiscoveredUpdate` / `DownloadingUpdate` / `DownloadedUpdate` / `ApplyUpdate`。
2. 状态属性必须在事件触发**之前**更新；事件丢失不影响状态正确性。
3. `check_update` 返回 `has_update=false` 且当前状态不是 `None` 时，必须补发一次
   `None`（并给出“状态已清除”消息），避免断网后界面停在“发现新版本”。
4. 断网时即使本地已有下载完成的待应用版本，`check_update` 也返回 `has_update=false`；
   宿主不得因此删除下载产物，联网后自动继续。
5. `Cancel`/`Dispose` 只停止轮询，不结束已拉起的 `apply_update` 进程，也不删除文件。
6. 下载失败且 `errorMsg` 含 `already at latest version` 时，按“无更新”处理并复位 `None`，
   不得进入错误重试（强更场景会死循环）。

---

## 5. 确认事件（非强制更新）

| 事件 | 触发条件 | 语义 |
|---|---|---|
| `RequestDownloadUpdate(newVersion)` | `has_update=true`、`need_download_update=true`、`force_update=false` | **同步**调用；返回即代表用户确认，宿主随后必须下载 |
| `RequestApplyUpdate(newVersion)` | 下载完成、`force_update=false` | **同步**调用；返回即代表用户确认，宿主随后必须应用 |

- `force_update=true` 时两个事件**都不得触发**，直接下载并应用。
- 事件**没有“拒绝”语义**：宿主回调返回就视为确认。要中止整条流程只能 `Cancel()`。
- 宿主若想“询问用户后再决定”，必须在回调内阻塞等待用户选择（可取消），不得先返回再反悔。
- 事件处理器抛出的异常必须被 SDK 捕获并记录，不得中断更新循环。

---

## 6. `download_update` 流解析

`download_update` 的输出是 **JSON 行流**，不是单行：

```text
{"isSuccess":true,"errorMsg":"","data":{"index":0,"total":3,"file":"","status":"BEGIN","file_size":0,"total_bytes":123}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":3,"file":"a.dll","status":"START","file_size":50,"downloaded_bytes":0,"total_bytes":123,"speed_bps":0}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":3,"file":"a.dll","status":"PROGRESS","file_size":50,"downloaded_bytes":30,"total_bytes":123,"speed_bps":1024}}
{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":3,"file":"a.dll","status":"DONE","file_size":50,"downloaded_bytes":50,"total_bytes":123,"speed_bps":1024}}
{"isSuccess":true,"errorMsg":"","data":null}
```

解析规则：

1. 逐行解析（按 `\n`；空行跳过），每行都是一个 `Output` 信封。
2. 任一行的 `isSuccess=false` 表示本轮失败；SDK **可以**立即停止读取并返回该行
   `errorMsg`（C# SDK 即如此）。若需要完整的 `failed_files`，应继续读到最终行或进程退出；
   这不是必须行为。
3. `data == null` 且 `isSuccess=true` 是**唯一**的完成信号：返回成功，不要等待进程退出
   （进程随后自行退出）。
4. `status` 只用于展示：进度只统计 `status=DONE` 且 `total>0` 的行；
   `index/total` 是**文件计数**进度。字节级进度优先用 `downloaded_bytes/total_bytes`，
   缺失时回退到 `index/total`。
5. `status` 未知值必须被容忍（忽略该行），不得崩溃或判失败。
6. 进程在完成行之前退出 → 失败（`download interrupted`）；看门狗杀进程 → 失败
   （`download stalled`）。
7. 单文件失败不整体中止：客户端会继续下载其余文件，最后输出带 `failed_files` 的失败结果。
   已成功的文件在下一轮会因校验通过被跳过（断点续传），宿主可直接重试。

---

## 7. `apply_update` 与重启

- `apply_update` 是**独立进程**：宿主拉起后立即返回，不得等待其结束。
- 更新器会关闭 `must_close_process_name` 中的进程，原子替换主目录并重启主程序。
- 宿主在收到“拉起成功”后**不要**再做清理工作（进程随时会被关闭）。
- `apply_update` 自身持全局更新锁；30 s 内重复拉起只会白跑进程并抢占锁。
- 没有待应用更新时返回 `isSuccess=false`（`no pending update to apply`），宿主应停止本轮流程，
  不要重试。
- 主程序重启由更新器根据 `client.json` 的 `main_exe_relative_path` 负责，宿主不需要自己重启。

---

## 8. 自更新（更新器自身）

1. SDK/UI 启动时**应该**先执行 `check_self_update`，再按需委派 `self_update`。
2. `self_update` 是统一执行入口：取 `.aly.lock`，用主程序目录中的新副本替换 `UpdateFolder`
   中的运行副本。
3. 回退规则：仅当主程序目录中的源副本缺失，或 `self_update` 命令失败且 `.aly.lock` **不存在**时，
   SDK 才可以做“临时文件 + SHA256 校验 + 原子替换”的本地兜底。
4. 锁被占用时**不得**走兜底（否则绕过更新锁，与另一更新流程互相破坏）。
5. `self_update` 幂等：已是最新或源缺失都返回 `isSuccess=true`，用 `reason` 区分
   （`already_up_to_date` / `source_missing`）。重复调用只多一次进程创建。

---

## 9. 部署布局与配置

```text
PackageFolder/
├── ApplicationFolder/                # 当前版本（主程序运行目录）
│   ├── YourApp.exe
│   ├── aly-client.exe                # 自更新源副本（随版本分发）
│   └── .updator/shared.json          # server_url、project_name、忽略规则
├── ApplicationFolder_V1.0.0/         # 历史快照（回滚用）
│   └── ...
└── UpdateFolder/                     # 更新器目录（工作目录）
    ├── aly-client.exe                # 正在运行的更新器
    ├── aly-client-ui.exe             # 可选 GUI
    ├── client.json
    ├── version.json                  # 更新器维护，宿主不得写
    └── .aly.lock                     # 运行时锁文件
```

### `UpdateFolder/client.json`

| 字段 | 类型 | 语义 |
|---|---|---|
| `main_exe_relative_path` | string | 主程序 exe 路径，**相对于 `UpdateFolder/`**（如 `../ApplicationFolder/YourApp.exe`） |
| `must_close_process_name` | string[] | 应用更新前需关闭的进程名，**不带 `.exe`**；为空时更新器在替换失败时会探测并强杀占用者 |

### `ApplicationFolder/.updator/shared.json`

| 字段 | 语义 |
|---|---|
| `server_url` | 服务端地址 |
| `project_name` | 项目名 |
| `ignore_folders` / `ignore_files` | 扫描（上传）与应用（复制/替换）时都跳过；支持 `*` 与 `**` |
| `un_copy_folders` / `un_copy_files` | 下载物化时不从主目录复制的路径（运行时数据目录等） |

`shared.json` 随每个版本分发；客户端下载新版本后以新文件为准。

---

## 10. 锁与并发

- 全局更新锁文件为 `UpdateFolder/.aly.lock`。`download_update`、`apply_update`、`rollback`、
  `clean`、`self_update` 会取锁；`check_update`、`check_diff`、`list_rollback_versions` 不取锁。
- 同一时刻只允许一个更新流程。宿主、GUI、SDK 多入口并发触发是安全的：拿不到锁的一方
  得到 `isSuccess=false`，应按“稍后重试”处理（建议 5 s）。
- 持锁进程存活期间锁有效；只有陈旧残留锁会被清理。
- `apply_update` 是独立进程，宿主不得因为它“立刻返回”就认为锁已释放。
- 宿主**不得**自行删除或改写 `.aly.lock`、`version.json`。

---

## 11. 参数约定

| 参数 | 命令 | 约定 |
|---|---|---|
| `--url <server>` | 多数联网命令 | 覆盖 `shared.json` 的 `server_url`；SDK 通常不传 |
| `--project-name <name>` | 多数联网命令 | 覆盖 `project_name` |
| `--main-exe-path <path>` | 多数命令 | 覆盖 `client.json` 的主程序相对路径 |
| `--must-close-process-name "A,B"` | `apply_update`、`rollback` | 逗号分隔、不带 `.exe`；追加到 `client.json` 的同名字段 |
| `--close-timeout <秒>` | `apply_update`、`rollback` | 关闭进程超时；默认 30，等于 30 时宿主不应传该参数 |
| `--version <v>` | `rollback` | 必填；目标版本号（不带 `V` 前缀也可） |

宿主拼接参数时必须做引号处理（路径含空格）；优先使用数组式传参（不经 shell）。

---

## 12. 错误分类

宿主应按下表决定“上报 / 重试 / 复位状态”。匹配基于 `errorMsg` 文本，**不要**依赖退出码。

| `errorMsg` 特征 | 分类 | 建议动作 |
|---|---|---|
| `already at latest version` | 无更新 | 复位 `None`，按空闲间隔继续 |
| `no pending update to apply` | 无待应用 | 终止本轮 apply，复位状态 |
| `下载完成但有 N 个文件失败` | 部分失败 | 上报；下一轮直接重试（断点续传） |
| `download stalled: no output for ...` | 停流 | 上报；按失败重试 |
| `download interrupted (aly-client exited before finish line)` | 进程提前退出 | 上报；重试 |
| 网络错误（连接失败、超时、5xx） | 可重试 | 5 s（检查）/ 1 s（下载）后重试 |
| 锁被占用（`lock` / 占用相关） | 并发 | 5 s 后重试，不要杀进程绕过 |
| `no server url configured` / `no project name configured` | 配置缺失 | 上报一次，**不要**高频重试 |
| `版本状态异常` / `unknown version_status` | 状态损坏 | 上报，停止自动流程，等待人工处理 |
| `version <v> not found` | 回滚目标不存在 | 上报，刷新版本列表 |
| 超时被杀 | 宿主侧失败 | 上报；不要自动无限重试 |

---

## 13. 版本与兼容

- 协议版本在 schema 的 `x-protocol-version`，当前 `1.0.0`。目前**没有**运行时握手命令；
  宿主应通过自更新保证更新器副本与 SDK 兼容。
- 兼容规则：新增可选字段 = MINOR；删除/改名/改类型/改语义 = MAJOR。
- **客户端严格、宿主宽松**：schema 用 `additionalProperties:false` 抓客户端漂移；
  所有 SDK 反序列化**必须忽略未知字段**，不得因为新字段报错。
- 未知的 `status`、未知的 `data` 载荷、`data:null` 都必须被安全处理。

---

## 14. 命令到载荷映射

| 命令 | `data` 定义 | 最终行 | 备注 |
|---|---|---|---|
| `check_update` | `CheckUpdateData` | 单行 | 离线返回 `has_update=false` |
| `check_diff` | `CheckDiffData` | 单行 | 调试/可视化用 |
| `download_update` | `DownloadProgress`（多行）/ `DownloadUpdateData`（失败） | `data:null` | 行流；见 §6 |
| `apply_update` | 无（`null`） | 单行 | 独立进程，不等待 |
| `rollback` | `RollbackData` | 单行 | 崩溃恢复分支可能 `data:null` |
| `list_rollback_versions` | `RollbackListData` | 单行 | |
| `check_self_update` | `CheckSelfUpdateData` | 单行 | |
| `self_update` | `SelfUpdateData` | 单行 | |
| `clean` | `CleanData` | 单行 | 运维/计划任务用 |

---

## 15. 宿主生命周期（宿主退出后跟随退出）

`aly-client` / `aly-client-ui` 都是**宿主拉起的子进程**。宿主退出后，它们不应该变成孤儿：

| 场景 | 要求 |
|---|---|
| 普通命令（`check_update` / `download_update` / `rollback` / `clean` / `*self_update` …）与 UI 常规运行 | 宿主退出后**必须**跟随退出 |
| `apply_update` 阶段（命令级 `apply_update`，或 `version.json` 的 `version_status == applying` 的崩溃恢复窗口） | **不得**跟随退出：更新器要关闭宿主、替换主目录并重启主程序，宿主本来就会（也必须）消失 |

规则：

1. 「宿主」指**拉起了本进程的那个进程**（父进程）。UI 与更新器各自的父进程就是它们的宿主。
2. 两个例外**取或**：命令级（当前命令是 `apply_update`）或状态级（`version_status == applying`）。
   状态级覆盖「崩溃恢复」——那次没有 `apply_update` 命令在跑，但主目录可能已丢失，同样必须续跑。
3. 退出走**优雅收尾**：先写日志（UI/更新器各自写自己的日志或 stderr 诊断），再以 0 退出。
4. 父 PID 拿不到时**不得**跟随退出（宁可不退，也不能因为拿不到父进程就把自己结束掉）。
5. 更新锁不必显式释放：锁文件按「持有者进程已退出」判陈旧（`cmd/lock.go` 的 `cleanStaleLock`），
   本进程退出后残留锁会被下一次运行当陈旧清理掉。
6. 宿主退出**不代表**已下载产物可以丢弃；已下载内容与 `version.json` 一律保持原样（同 §4 规则 4）。
