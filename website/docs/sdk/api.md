---
title: C# SDK API 参考
description: AlyClient.CSharpSDK 的完整 API：AlyUpdateClient、AlyApi、AlyUpdateUIClient 的成员签名、枚举、响应与数据模型、超时与失败形态。
---

# C# SDK API 参考

本页列出 `AlyClient.CSharpSDK` 的全部公开类型与成员，签名与源码一致。集成步骤与部署说明见[集成指南](/docs/sdk)。

> **模型来源**：数据模型字段由仓库 [`spec/aly-protocol.schema.json`](https://github.com/lishuangquan1987/aly/blob/master/spec/aly-protocol.schema.json) 生成（生成器 `tools/alyprotocol`）。增删字段请改 schema 与 `client/aly-client/model/dto.go` 后重新生成，不要手改 `Models/Generated/*.g.cs`；流程见 [`spec/README.md`](https://github.com/lishuangquan1987/aly/blob/master/spec/README.md)。

## 命名空间

| 命名空间 | 类型 |
|---|---|
| `AlyClient.CSharpSDK` | `AlyApi`、`AlyUpdateClient`、`AlyUpdateUIClient`、`AlyClientStatus`、`AlyUpdateUiMode`、`CheckUpdateData`、`CheckSelfUpdateData`、`ListRollbackData`、`RollbackData`、`DownloadProgressData`（`internal`） |
| `AlyClient.CSharpSDK.Models` | `AlyResponse`、`AlyResponse<T>`、`SelfUpdateData` |

::: tip 命名空间容易记错
数据模型并不全在 `.Models` 下：`CheckUpdateData`、`CheckSelfUpdateData`、`ListRollbackData`、`RollbackData` 位于 `AlyClient.CSharpSDK`，只有 `AlyResponse`、`AlyResponse<T>`、`SelfUpdateData` 在 `AlyClient.CSharpSDK.Models`。同时用到响应包装和检查结果时，两个 `using` 都要写。
:::

## AlyUpdateClient

`aly-client.exe` 的托管封装：后台线程依次执行更新器自更新与「检查 → 下载 → 应用」轮询循环。

### 构造函数

```csharp
public AlyUpdateClient(string updatorExePath = null)
```

| 参数 | 说明 |
|---|---|
| `updatorExePath` | `aly-client.exe` 路径。为空时取 `Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\aly-client.exe")` |

构造函数**立即启动后台任务**（`Task.Factory.StartNew`），不是延迟到第一次调用方法时。`IsRunning` 在构造函数里先置为 `true`。

默认路径是 Windows 形式：Linux 上没有 `.exe`、反斜杠也不是分隔符，必须显式传入 `UpdateFolder/aly-client`。

### 属性

| 成员 | 类型 | 访问 | 说明 |
|---|---|---|---|
| `Status` | `AlyClientStatus` | 只读（线程安全） | 当前阶段。getter 与内部 setter 在同一把锁内，可跨线程读 |
| `IsRunning` | `bool` | 只读 | 后台循环是否在运行。构造后为 `true`，循环退出（`Cancel`/`Dispose`/顶层异常）后变 `false` |
| `IsError` | `bool` | 只读（线程安全） | 最近一次 `check_update` 或下载是否失败。下一次 `check_update` 成功会清空 |
| `UpdatorExePath` | `string` | 读写 | 更新器路径。赋值时自动 `Path.GetFullPath`；赋空值保持原样（不转换） |
| `MustCloseProcessNames` | `string` | 读写 | 应用更新前需关闭的进程名，**不带 `.exe`**，多个用逗号分隔。未设置时更新器在替换阶段遇到目录占用会直接强杀宿主 |
| `IdlePollIntervalSeconds` | `int` | 读写 | 空闲（无更新）轮询间隔（秒）。`<= 0` 时按 **300** 处理，小于 **60** 时按 60 处理；实际每轮睡眠为 `interval + 0–60 秒随机抖动`，抖动用于打散客户端群。失败重试不受此值影响（固定 5 秒） |

### 方法

#### `public void Cancel()`

停止后台轮询。通过 `CancellationTokenSource.Cancel()` 实现，幂等，`ObjectDisposedException` 被吞掉。睡眠是可中断的（每 1 秒检查一次 token），所以 `Cancel` 后循环最多 1 秒内退出，不会睡满一个空闲周期。

#### `public void Dispose()`

`Cancel()` + 等待主循环退出（最长 **5 秒**）+ 释放 `CancellationTokenSource`。幂等。`Dispose` 不会结束已经由 `apply_update` 拉起的独立进程，也不会删除 `UpdateFolder` 中的任何文件。

### 事件

| 事件 | 签名 | 触及时机 |
|---|---|---|
| `StatusChanged` | `event Action<AlyClientStatus, string>` | 每次状态迁移，第二参数是可读消息（含下载文件名与百分比） |
| `RequestDownloadUpdate` | `event Action<string>` | 仅非强制更新；参数为新版本号。**同步调用**，返回即代表用户确认 |
| `RequestApplyUpdate` | `event Action<string>` | 仅非强制更新；参数为新版本号。**同步调用**，返回即代表用户确认 |
| `ErrorStatusChanged` | `event Action<string>` | 错误状态变化。参数为错误文本（新错误或错误内容变化）；恢复时参数为 `null`；同一条错误重复出现不再触发 |

事件语义细节：

- 所有事件都在后台循环线程上触发，跨线程更新 UI 需要 marshal。
- 处理器抛出的异常被 SDK 捕获并写入 `aly-client-sdk.log`，不会中断循环。
- `Status` 属性在事件触发前就已更新，即使事件丢失也不影响状态正确性。
- 下载进度通过 `StatusChanged` 上报，文本形如 `Downloading {文件名}... {百分比}%`，百分比是「已完成文件数 / 总文件数」，只统计 `DONE` 状态。
- 无待处理更新且此前状态非 `None` 时，会补发一次 `StatusChanged(None, "Update status cleared")`，避免断网后宿主界面停在「发现新版本」。

### 后台循环行为

| 情形 | 后续动作 |
|---|---|
| `check_update` 返回失败 | 触发 `ErrorStatusChanged`，睡眠 5 秒后重试 |
| `check_update` 成功 | 清空错误状态（此前有错误时触发一次 `ErrorStatusChanged(null)`） |
| `has_update = false` | 状态复位为 `None`（必要时发一次事件），按空闲间隔睡眠 |
| `has_update = true` 且 `need_download_update = true` | 状态 `DiscoveredUpdate`；非强更时先触发 `RequestDownloadUpdate`；然后 `DownloadingUpdate` 并下载 |
| 下载失败且错误含 `already at latest version` | 视为无更新：状态复位 `None`，按空闲间隔睡眠（避免强更场景死循环报错） |
| 下载失败（其他原因） | 触发错误，睡眠 1 秒后重试本轮 |
| 下载成功 | 状态 `DownloadedUpdate`；非强更时触发 `RequestApplyUpdate`；随后 `ApplyUpdate` |
| `apply_update` 拉起失败 | 触发 `ErrorStatusChanged("apply spawn failed: ...")`；成功则宿主进程会被更新器关闭 |
| 任意异常 | `ErrorStatusChanged("update exception: ...")`，睡眠 5 秒后继续 |

`apply_update` 拉起后循环会额外睡眠 **30 秒**再继续检查：该命令是独立进程（SDK 不等待其结束），期间更新锁由它持有，频繁拉起只会白跑进程。

自更新在循环之前执行：优先委派 `self_update` 命令；目标文件缺失、或命令失败且 `UpdateFolder/.aly.lock` 不存在时，回退为「临时文件 + SHA256 校验 + 原子替换」的直接替换（Windows 用 `File.Replace`，非 Windows 用备份改名 + 回滚）。锁被占用时不做兜底，避免绕过更新锁。

## AlyApi

静态方法，一对一映射 `aly-client` 命令。所有 `Task` 返回值都是 `Task.Factory.StartNew` 包装的阻塞调用——`.Result` 会阻塞当前线程直到命令结束，不是真正的异步 I/O。

### 方法总览

| 方法 | 对应命令 | 返回 | 默认超时 |
|---|---|---|---|
| `CheckSelfUpdateAsync(string alyExePath)` | `check_self_update` | `Task<AlyResponse<CheckSelfUpdateData>>` | 30 s |
| `SelfUpdateAsync(string alyExePath)` | `self_update` | `Task<AlyResponse<SelfUpdateData>>` | 30 s |
| `CheckUpdateAsync(string alyExePath)` | `check_update` | `Task<AlyResponse<CheckUpdateData>>` | 30 s |
| `DownloadUpdateAsync(string alyExePath, Action<string, double> progress)` | `download_update` | `Task<AlyResponse>` | 停流看门狗 1800 s |
| `DownloadUpdateAsync(string alyExePath, Action<string, double> progress, int stallTimeoutSeconds)` | `download_update` | `Task<AlyResponse>` | 自定义停流超时 |
| `ListRollbackVersionsAsync(string alyExePath)` | `list_rollback_versions` | `Task<AlyResponse<ListRollbackData>>` | 30 s |
| `RollbackAsync(string alyExePath, string version, string mustCloseProcessNames = null, int closeTimeoutSeconds = 30)` | `rollback --version <v>` | `Task<AlyResponse<RollbackData>>` | 600 s |
| `ApplyUpdateAsync(string alyExePath, string mustCloseProcessNames = null, int closeTimeoutSeconds = 30)` | `apply_update` | `AlyResponse`（同步） | 不等待 |

### 逐方法说明

#### `CheckSelfUpdateAsync`

```csharp
public static Task<AlyResponse<CheckSelfUpdateData>> CheckSelfUpdateAsync(string alyExePath)
```

检查更新器自身副本是否落后于主程序目录里的副本。只检测，不替换。返回 `CheckSelfUpdateData.NeedUpdate`。

#### `SelfUpdateAsync`

```csharp
public static Task<AlyResponse<SelfUpdateData>> SelfUpdateAsync(string alyExePath)
```

执行更新器自更新：按 SHA256 比较后，用主程序目录中随版本分发的 `aly-client.exe` 替换 `UpdateFolder` 中正在运行的副本。幂等；源缺失或已是最新同样返回 `IsSuccess=true`，此时 `SelfUpdateData.Reason` 为 `source_missing` 或 `already_up_to_date`。

#### `CheckUpdateAsync`

```csharp
public static Task<AlyResponse<CheckUpdateData>> CheckUpdateAsync(string alyExePath)
```

检查应用是否有新版本。服务器不可达时返回 `has_update=false`——即使本机已有下载完成的待应用版本也不上报，联网后自动继续。

#### `DownloadUpdateAsync`

```csharp
public const int DefaultDownloadStallTimeoutSeconds = 1800;

public static Task<AlyResponse> DownloadUpdateAsync(string alyExePath, Action<string, double> progress)

public static Task<AlyResponse> DownloadUpdateAsync(string alyExePath, Action<string, double> progress, int stallTimeoutSeconds)
```

下载差异文件。**没有总时长上限**，只有「停流看门狗」：客户端对单文件大下载每 3 秒输出一行 `PROGRESS`，因此「超过 `stallTimeoutSeconds` 无任何输出」即判为真停流（网络挂起、磁盘卡死、进程僵死），SDK 会杀掉进程并返回失败。看门狗定时器每 5 秒检查一次。

| 参数 | 说明 |
|---|---|
| `alyExePath` | 更新器路径 |
| `progress` | `Action<string, double>`，参数为文件名与进度比例 `index / total`（0–1，仅统计 `DONE` 的文件）。回调抛出的异常被吞掉，不会打断下载。可为 `null` |
| `stallTimeoutSeconds` | 无输出超时（秒），默认取 `DefaultDownloadStallTimeoutSeconds`（1800） |

返回 `Task<AlyResponse>`（无 `data`）。进度数据模型 `DownloadProgressData` 是 `internal`，宿主只能通过回调拿到文件名与比例。

#### `ApplyUpdateAsync`

```csharp
public static AlyResponse ApplyUpdateAsync(
    string alyExePath,
    string mustCloseProcessNames = null,
    int closeTimeoutSeconds = 30)
```

应用已下载的更新（原子替换），会关闭主进程并重启。**同步方法、不等待命令结束**：内部以独立进程启动 `aly-client.exe` 后立即返回 `AlyResponse.OK()`，只在进程启动失败时返回 `NG`。

| 参数 | 说明 |
|---|---|
| `mustCloseProcessNames` | 逗号分隔的进程名（不带 `.exe`），非空时追加 `--must-close-process-name "<值>"` |
| `closeTimeoutSeconds` | 关闭进程超时，等于 30 时不传参；否则追加 `--close-timeout <值>` |

调用方不要在它返回后继续做清理工作——更新器会在替换前关闭当前进程。

#### `ListRollbackVersionsAsync`

```csharp
public static Task<AlyResponse<ListRollbackData>> ListRollbackVersionsAsync(string alyExePath)
```

列出可回滚的历史版本快照（`ApplicationFolder_<version>/` 目录），返回 `CurrentVersion` 与 `Versions`。

#### `RollbackAsync`

```csharp
public const int RollbackTimeoutSeconds = 600;

public static Task<AlyResponse<RollbackData>> RollbackAsync(
    string alyExePath,
    string version,
    string mustCloseProcessNames = null,
    int closeTimeoutSeconds = 30)
```

回滚到指定版本。超时 600 秒：回滚要做目录改名、等待主进程退出、崩溃恢复分支（可能多次 rename 重试），30 秒不够——超时杀进程会把回滚留在中间态（由崩溃恢复兜底，但体验差）。

| 参数 | 说明 |
|---|---|
| `version` | 目标版本号，追加为 `rollback --version <version>` |
| `mustCloseProcessNames` / `closeTimeoutSeconds` | 同 `ApplyUpdateAsync` |

### 超时与失败形态

| 常量 | 值 | 作用域 |
|---|---|---|
| `AlyApi.DefaultDownloadStallTimeoutSeconds` | `1800` | `public const`：下载停流看门狗默认值 |
| `AlyApi.RollbackTimeoutSeconds` | `600` | `public const`：`RollbackAsync` 超时 |
| 短命令默认超时 | `30` | `private const`：`check_self_update`、`self_update`、`check_update`、`list_rollback_versions`，不可配置 |

超时到点后的行为：杀掉 `aly-client.exe`，最多再等 2 秒让读流任务收尾，返回 `IsSuccess=false`，`ErrorMsg` 为 `aly-client.exe timed out after <N>s and was killed`。

其他失败形态（均为 `IsSuccess=false`）：

| 情况 | `ErrorMsg` |
|---|---|
| 流已关闭但进程 1 秒内未退出 | `aly-client.exe exited streams but process did not exit`（进程会被杀） |
| 退出码非 0 且 stdout 为空，stderr 也空 | `aly-client.exe exited with code <N> (may require admin)` |
| 退出码非 0 且 stdout 为空，stderr 非空 | stderr 内容 |
| 退出码 0 但 stdout 为空 | `aly-client.exe returned no output (may require admin)` |
| stdout 不是合法 JSON | 反序列化异常的 `Message` |
| 下载中客户端输出失败 JSON | 客户端的 `errorMsg` 原样返回 |
| 下载中进程在完成行之前退出 | `download interrupted (aly-client exited before finish line)` |
| 下载停流被看门狗杀进程 | `download stalled: no output for <N>s, process killed` |
| 启动进程抛异常（找不到文件等） | 异常 `Message` |

退出码非 0 但 stdout 有内容时，SDK 信任 stdout 里的 JSON（客户端可能用非 0 退出码返回业务结果）。

### 进程调用约定

`AlyApi` 启动 `aly-client.exe` 时固定使用：

| 设置 | 值 |
|---|---|
| `UseShellExecute` | `false` |
| `CreateNoWindow` | `true`（不弹控制台窗口） |
| `WorkingDirectory` | `Path.GetDirectoryName(alyExePath)`，即 `UpdateFolder` |
| `RedirectStandardOutput` / `RedirectStandardError` | `true`（`ApplyUpdateAsync` 的独立进程不重定向） |
| `StandardOutputEncoding` / `StandardErrorEncoding` | `Encoding.UTF8` |

因此输出必须是 UTF-8 JSON。同时因为工作目录被设为 `UpdateFolder`，`aly-client` 的 `client.json`、`version.json` 等都以 `UpdateFolder` 为基准解析；主程序配置 `.updator/shared.json` 则是按 `client.json` 里的主程序目录定位。

## AlyUpdateUIClient

`aly-client-ui.exe` 的启动封装：把整条更新链交给 GUI，宿主只负责按场景拉起。

### 构造函数

```csharp
public const string DefaultLanguage = "zh-CN";

public AlyUpdateUIClient(string uiExePath = null, string language = DefaultLanguage)
```

| 参数 | 说明 |
|---|---|
| `uiExePath` | UI 路径。为空时取 `Path.GetFullPath(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\aly-client-ui.exe"))`；非空时同样转成绝对路径 |
| `language` | 界面语言 `zh-CN` / `en-US`；空白时回退 `DefaultLanguage` |

### 属性

| 成员 | 类型 | 访问 | 说明 |
|---|---|---|---|
| `UiExePath` | `string` | 读写 | UI 可执行文件绝对路径，赋值时自动转绝对路径 |
| `Language` | `string` | 读写 | 界面语言，拼进命令行 `-lang` |
| `IsRunning` | `bool` | 只读 | 本对象最近一次拉起的进程是否仍在运行（`HasExited` 异常时按已退出处理） |
| `LastExitCode` | `int?` | 只读 | 上次运行的退出码；本次运行尚未结束时为 `null`，进程一退出就写入 |

### 方法

| 方法 | 返回 | 说明 |
|---|---|---|
| `Launch(AlyUpdateUiMode mode)` | `AlyResponse` | 按模式拉起，非阻塞 |
| `LaunchAuto()` | `AlyResponse` | 等价 `Launch(AlyUpdateUiMode.Auto)` |
| `LaunchCheck()` | `AlyResponse` | 等价 `Launch(AlyUpdateUiMode.Check)` |
| `LaunchUpdate()` | `AlyResponse` | 等价 `Launch(AlyUpdateUiMode.Update)` |
| `LaunchRollback()` | `AlyResponse` | 等价 `Launch(AlyUpdateUiMode.Rollback)` |
| `LaunchSilent()` | `AlyResponse` | 等价 `Launch(AlyUpdateUiMode.Silent)` |
| `static ToArgument(AlyUpdateUiMode mode)` | `string` | 枚举转命令行取值：`auto` / `check` / `update` / `rollback` / `silent`；未知值回退 `auto` |
| `Dispose()` | — | 退订事件、释放 `Process` 句柄；幂等；**不结束**已运行的窗口 |

`Launch` 的执行顺序：

1. 已 `Dispose` → 返回 `NG("AlyUpdateUIClient has been disposed.")`。
2. `UiExePath` 不存在 → 返回 `NG("aly-client-ui.exe not found: <路径>（请确认 UpdateFolder 已部署；构建时设置 AlyIncludeClientUi=true 自动下载）")`（括注是旧版本遗留的提示文案，现状见下节）。
3. 本对象持有的进程仍在运行 → 直接返回 `OK()`，不再拉起。
4. 释放上一次的进程句柄，用 `Arguments = "-mode <mode> -lang <lang>"`、`WorkingDirectory = UI 所在目录`、`CreateNoWindow = true` 启动；成功后重置 `LastExitCode = null` 并触发 `Started(pid)`。
5. `Process.Start()` 返回 `false` → 释放刚创建的句柄并返回 `NG("failed to start aly-client-ui.exe")`；抛异常 → 返回 `NG(ex)`。

### 事件

| 事件 | 签名 | 时机 |
|---|---|---|
| `Started` | `event Action<int>` | 进程成功拉起后，参数为 PID |
| `Exited` | `event Action<int>` | 进程退出后，参数为退出码（读取 `ExitCode` 抛异常时按 0 处理）；触发前已释放句柄 |

### aly-client-ui.exe 的依赖与部署

`AlyUpdateUIClient` **必须**有 `aly-client-ui.exe`。它默认与 `aly-client.exe` 同目录（都在 `UpdateFolder/`）：UI 启动时先在同目录找更新器，找不到再回落到 `client.json` 里主程序目录下的 `aly-client.exe`。UI 是独立单实例程序，负责窗口、下载进度、回滚版本选择，并在 `apply_update` 后按 `client.json` 重启主程序。

更新器与 GUI 由 **AlyPublish 安装包**提供：GUI 初始化项目时会把包内 `client/<rid>/` 的两个文件复制进同级 `UpdateFolder/`，也可以按[下载与安装](/docs/download)的 payload 布局手工放置。SDK 的 targets 现在只建目录、在缺失时写默认 `client.json`，**不再下载任何文件**；缺少 `aly-client-ui.exe` 时按上面的路径补一份即可。

单实例转发：如果窗口已由其他途径启动（SDK 没有该进程句柄），`Launch` 仍会启动新进程；新进程通过命名事件把「手动检查请求」转发给既有实例，然后以退出码 0 立即结束——因此「窗口没变多」不是失败。仅 `check`/`update` 这类手动模式会转发并升级既有实例的交互状态，`auto`/`silent` 实例收到转发请求后按交互式语义处理。

## 枚举

### `AlyClientStatus`

```csharp
public enum AlyClientStatus
{
    None,
    DiscoveredUpdate,
    DownloadingUpdate,
    DownloadedUpdate,
    ApplyUpdate,
}
```

| 值 | 含义 |
|---|---|
| `None` | 空闲/无待处理更新。也用于「状态被清除」——断网导致无法核对时客户端返回 `has_update=false`，SDK 会把残留状态复位成 `None` |
| `DiscoveredUpdate` | 服务端发现新版本 |
| `DownloadingUpdate` | 正在下载变更文件，`StatusChanged` 消息携带文件名与百分比 |
| `DownloadedUpdate` | 下载完成，等待应用 |
| `ApplyUpdate` | 正在执行原子替换并重启主程序 |

### `AlyUpdateUiMode`

```csharp
public enum AlyUpdateUiMode
{
    Auto,
    Check,
    Update,
    Rollback,
    Silent,
}
```

| 值 | 命令行 | 行为 |
|---|---|---|
| `Auto` | `-mode auto` | 启动时静默检查：窗口默认隐藏，只有确认有更新才显示；无更新或服务器不可达时静默退出 |
| `Check` | `-mode check` | 交互式检查：先显示「正在检查」，无更新/出错也提示并自动关闭，避免「点了没反应」 |
| `Update` | `-mode update` | 直接下载 + 应用，不再询问 |
| `Rollback` | `-mode rollback` | 显示可回滚版本列表，由用户在窗口内选择 |
| `Silent` | `-mode silent` | 与 `Auto` 同口径的静默检查：无更新/出错不打扰，确认有更新才显示 |

## 响应与数据模型

### `AlyResponse`（`AlyClient.CSharpSDK.Models`）

对应 `aly-client` 的 stdout JSON 外层（非泛型版本，用于无 `data` 的命令）。

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `IsSuccess` | `bool` | `isSuccess` | 命令是否成功 |
| `ErrorMsg` | `string` | `errorMsg` | 失败原因 |
| `static OK()` | `AlyResponse` | — | 构造成功响应 |
| `static NG(string errMsg)` | `AlyResponse` | — | 构造失败响应 |
| `static NG(Exception e)` | `AlyResponse` | — | 用 `e.Message` 构造失败响应 |

### `AlyResponse<T>`

继承 `AlyResponse`，增加强类型载荷。

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `Data` | `T` | `data` | 命令载荷 |
| `static OK(T data)` | `AlyResponse<T>` | — | 构造成功响应 |
| `static new NG(string errMsg)` | `AlyResponse<T>` | — | 构造失败响应 |
| `static new NG(Exception e)` | `AlyResponse<T>` | — | 用 `e.Message` 构造失败响应 |

### `CheckUpdateData`（`AlyClient.CSharpSDK`）

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `HasUpdate` | `bool` | `has_update` | 是否有新版本。服务器不可达时为 `false` |
| `NeedDownloadUpdate` | `bool` | `need_download_update` | 是否需要下载（已下载待应用时为 `false`） |
| `CurrentVersion` | `string` | `current_version` | 当前版本 |
| `NewVersion` | `string` | `new_version` | 新版本 |
| `ForceUpdate` | `bool` | `force_update` | 强制更新：为 `true` 时不触发确认事件，直接下载并应用 |
| `ReleaseNotes` | `string[]` | `release_notes` | 服务端变更日志，逐条一项；服务端未配置时为 `null`；仅在上报新版本且需要下载时下发 |

### `CheckSelfUpdateData`（`AlyClient.CSharpSDK`）

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `NeedUpdate` | `bool` | `need_update` | 更新器自身是否需要更新 |

### `SelfUpdateData`（`AlyClient.CSharpSDK.Models`）

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `Updated` | `bool` | `updated` | 是否真的替换了 |
| `Reason` | `string` | `reason` | `updated=false` 时的跳过原因：`source_missing` / `already_up_to_date` |
| `OldPath` | `string` | `old_path` | `updated=true` 时被替换下来的旧版备份路径（`.old`） |

### `ListRollbackData`（`AlyClient.CSharpSDK`）

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `CurrentVersion` | `string` | `current_version` | 当前版本 |
| `Versions` | `string[]` | `versions` | 可回滚版本号列表 |

### `RollbackData`（`AlyClient.CSharpSDK`）

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `Version` | `string` | `version` | 实际回滚到的版本 |

### `DownloadProgressData`（`internal`）

下载进度行的原始结构（由 `spec/aly-protocol.schema.json` 生成），宿主无法直接引用，仅通过 `DownloadUpdateAsync` 的回调间接使用。

| 成员 | 类型 | JSON 字段 | 说明 |
|---|---|---|---|
| `Index` | `long` | `index` | 当前文件序号 |
| `Total` | `long` | `total` | 差异文件总数（只计需要下载的文件） |
| `File` | `string` | `file` | 当前文件的相对路径，透传给进度回调 |
| `Status` | `string` | `status` | 行类型：`BEGIN` / `START` / `PROGRESS` / `DONE` / `SKIP` / `FAIL`（SDK 只对 `DONE` 且 `Total > 0` 的行上报进度） |
| `FileSize` | `double` | `file_size` | 当前文件字节数 |
| `Error` | `string` | `error` | 该行失败原因（仅 `FAIL` 行） |
| `DownloadedBytes` | `long` | `downloaded_bytes` | 已下载字节数（累计，字节级进度用） |
| `TotalBytes` | `long` | `total_bytes` | 本次差异文件字节数总和（`BEGIN` 行预告） |
| `SpeedBps` | `long` | `speed_bps` | 最近窗口的平均吞吐（字节/秒） |

进度回调仍按**文件计数**粒度上报（`Index / Total`），且只统计 `DONE` 行，所以单个大文件从开始到完成只有一次进度跳变。字节级字段已进入模型（`internal`），但 `DownloadUpdateAsync` 暂未通过回调暴露；需要字节级进度时可直接解析 `aly-client` 的 stdout。

客户端输出 `data:null` 的完成行时，`DownloadUpdateAsync` 返回成功。

## 线程模型与错误处理

| 关注点 | 行为 |
|---|---|
| `AlyUpdateClient` 线程 | 每个实例一个后台线程（`Task.Factory.StartNew`），循环内所有 `AlyApi` 调用都是阻塞的 `.Result` |
| 事件线程 | `StatusChanged`、`RequestDownloadUpdate`、`RequestApplyUpdate`、`ErrorStatusChanged` 都在该后台线程上同步触发 |
| `AlyUpdateUIClient` 线程 | `Launch` 在调用线程上执行；`Exited` 由 `Process` 的 `Exited` 回调线程触发 |
| `AlyApi` 线程 | `Task` 在线程池上执行，`.Result` 会阻塞调用线程；不要在主 UI 线程等待 30 秒以上的命令 |
| 异常传播 | `AlyApi` 与 `AlyUpdateUIClient` 不向调用方抛业务异常：进程失败、超时、反序列化错误都转成 `IsSuccess=false` + `ErrorMsg`；事件处理器异常写入 `aly-client-sdk.log` |
| 日志 | `AlyUpdateClient` 把自更新结果、事件处理器异常写入宿主基目录的 `aly-client-sdk.log`；`aly-client` 自身的日志在 `UpdateFolder/update.log` |

相关文档：[集成指南](/docs/sdk)、[命令参考](/docs/commands)、[客户端配置](/docs/client/config)、[自更新设计](/docs/self-update)。