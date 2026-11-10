# C# SDK 集成

`AlyClient.CSharpSDK` 是给 .NET 宿主用的 NuGet 包，把"检查 → 下载 → 应用 → 自更新"的完整编排封装成几个类，宿主只需订阅事件与等待任务。目标框架 **net40 + netstandard2.0** 双支持。

## 获取

- NuGet 包：构建 `AlyClient.CSharpSDK` 工程产出 nupkg，内含 build targets（见下文自动部署）。
- 源码位置：`client/aly-client-sdk/AlyClient.CSharpSDK/`。

## 两种用法

### 高层编排：AlyUpdateClient（推荐）

`AlyUpdateClient` 在后台线程自动驱动更新状态机，宿主只订阅事件：

```csharp
using AlyClient.CSharpSDK;

using var client = new AlyUpdateClient();  // 默认定位 ..\UpdateFolder\aly-client.exe

client.StatusChanged += (status, msg) => Console.WriteLine($"{status}: {msg}");
client.RequestDownloadUpdate += _ => Console.WriteLine("发现新版本，开始下载");
client.RequestApplyUpdate   += _ => Console.WriteLine("下载完成，准备应用");

// MustCloseProcessNames / IdlePollIntervalSeconds 等可配置
client.Cancel();   // 取消进行中的流程
```

特性：

- **Status 状态机**：`AlyClientStatus` 枚举反映当前阶段，`IsError` 指示异常态。
- **自更新内置**：启动时先 `CheckSelfUpdateAsync`，需要时自动委派 `self_update` 命令（更新器本体取锁执行替换，保证没有并发写）。
- **下载守护**：进度回调 `Action<string, double>`，超过停滞超时（默认 1800 秒）判定为假死并可恢复。
- **回滚**：`ListRollbackVersionsAsync` / `RollbackAsync`（默认超时 600 秒）。

### 低层直调：AlyApi 静态方法

想自己控制时序的宿主可以直接调用静态方法，每个方法对应一个 aly-client 命令：

| 方法 | 对应命令 | 备注 |
|---|---|---|
| `CheckUpdateAsync(alyExePath)` | `check_update` | 返回 `CheckUpdateData`（has_update / force_update / new_version） |
| `DownloadUpdateAsync(path, progress)` | `download_update` | 带进度回调与停滞超时 |
| `ApplyUpdateAsync(...)` | `apply_update` | 应用完成后主程序被重启 |
| `ListRollbackVersionsAsync(path)` | `list_rollback_versions` | |
| `RollbackAsync(...)` | `rollback` | |
| `CheckSelfUpdateAsync(path)` | `check_self_update` | 检测更新器副本是否落后 |
| `SelfUpdateAsync(path)` | `self_update` | 执行自更新替换 |

## NuGet targets 自动部署

包内 `build/AlyClient.CSharpSDK.targets` 在**构建宿主工程时**自动把更新器可执行文件下载到 `AlyUpdateFolder`，属性均可覆盖：

| MSBuild 属性 | 默认值 | 说明 |
|---|---|---|
| `AlyClientVersion` | — | 下载的更新器版本标签 |
| `AlyClientDownloadUrl` | GitHub Release 直链 | `aly-client` 可执行文件下载地址 |
| `AlyClientUiDownloadUrl` | GitHub Release 直链 | GUI 可执行文件下载地址 |
| `AlyUpdateFolder` | `$(OutDir)..\UpdateFolder\` | 落盘目录 |
| `AlyIncludeClientUi` | `true` | 是否同时部署 GUI |
| `AlyRid` | `amd64` | Linux 架构（`arm64` 可覆盖） |

平台行为：

- **Windows**：用 PowerShell 下载 `aly-client.exe` / `aly-client-ui.exe`。
- **Linux**：走 `curl || wget` 下载 `aly-client-linux-<rid>` / `aly-client-ui-linux-<rid>`，落盘改名为 `aly-client` / `aly-client-ui` 并 `chmod 0755`——与客户端 `config.UpdaterFileName()` 的平台命名约定一致。
- 下载失败只警告不阻断构建；已存在且版本一致时跳过（幂等）。
