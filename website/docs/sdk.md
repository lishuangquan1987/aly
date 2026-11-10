---
title: C# SDK 集成
description: 在 net40 与 netstandard2.0 宿主中集成 AlyClient.CSharpSDK：两种集成方式、UpdateFolder 部署布局、client.json 相对路径、构建期 targets 属性与常见坑。
---

# C# SDK 集成

> 其它语言（TypeScript / Go / Python / Java / Rust / C++ / Pascal / VB.NET / F# / VB6 / 易语言）见
> [多语言 SDK 总览](/docs/sdk/languages)。本页只讲 .NET 集成。

`AlyClient.CSharpSDK` 把 `aly-client.exe` 的命令行协议封装成 .NET 类型：宿主不必拼接参数、解析 JSON、自己实现超时与进程收尸。它对外只有四个入口：

> **适合谁**：要把更新能力接进 .NET 宿主（WinForms/WPF/控制台）的开发者。读完可选对集成方式、放对 `UpdateFolder`，并用最小宿主跑通第一次更新。

| 类型 | 定位 |
|---|---|
| `AlyUpdateUIClient` | 把「检查 → 下载 → 应用 → 重启主程序」整条链交给独立窗口 `aly-client-ui.exe`，宿主只负责在合适时机拉起（**推荐**） |
| `AlyUpdateClient` | 宿主自建后台轮询与更新 UI，SDK 以事件驱动状态机的方式编排同一条链 |
| `AlyApi` | 静态方法，一对一映射单个 `aly-client` 命令，用于自定义时序 |
| 模型与枚举 | 命令输出的强类型视图，见 [API 参考](/docs/sdk/api) |

前两种方式二选一，不要同时启用：两条链路会各自拉起 `aly-client` 进程，虽然更新器内部有 `.aly.lock` 互斥，但界面与状态会互相干扰。

## 目标框架与依赖

| 项 | 值 |
|---|---|
| 目标框架 | `netstandard2.0`、`net40` |
| NuGet 依赖 | `Newtonsoft.Json` 13.0.4 |
| 程序集 | `AlyClient.CSharpSDK.dll` |
| 运行时组件 | `aly-client.exe`（必需）；`aly-client-ui.exe`（仅 `AlyUpdateUIClient` 需要） |
| 更新器形态 | 32 位单文件 exe，Go 1.10 / `GOARCH=386` 构建，兼容 Windows XP |

包内同时提供 `build\` 与 `buildTransitive\` 两处 targets，因此直接引用与传递引用都会触发部署逻辑（见[构建期自动部署](#构建期自动部署-targets)）。

### net40 与 C# 5 写法约束

SDK 自身用现代编译器构建，宿主用 VS2013 的 C# 5 编译器**引用已编译的 DLL** 没有问题。但如果宿主以项目引用方式引入 SDK 源码，则需要 VS2017 及以上或 `dotnet` CLI——SDK 工程是 SDK 风格项目（`Microsoft.NET.Sdk`），老的 MSBuild 打不开它。

本文所有示例都按 C# 5 可编译的范围写：

| 不要用（C# 6+） | 在 net40 宿主里的替代写法 |
|---|---|
| `using var client = ...`（C# 8） | `AlyUpdateClient client = new AlyUpdateClient();` 再显式 `try/finally { client.Dispose(); }` |
| 字符串插值 `$"{status}"`（C# 6） | `string.Format("[{0}] {1}", status, message)` 或 `"[" + status + "] " + message` |
| `?.` / `??` 空条件运算符（C# 6） | 显式 `if (x != null)` 判断 |
| `nameof(x)`（C# 6） | 直接写字符串字面量 |
| 表达式体成员 `=> ...`（C# 6） | 普通方法体 |
| 元组、模式匹配、`out var`（C# 7） | 输出参数先声明再传入 |
| `await`（需要 .NET 4.5 才提供的 async 支持类型） | 取 `Task<T>.Result`，或引用 `Microsoft.Bcl.Async` 包后再 `await` |

`var` 自 C# 3 起就可用，在 net40 上是合法的；本文仍统一写显式类型，只为让示例在任何老编译器与代码审查工具下表现一致。

## 获取 SDK

**该包当前没有发布到 NuGet.org**：在公开源检索不到 `AlyClient.CSharpSDK`，写 `<PackageReference>` 会导致还原失败。仓库的 Release 工作流只构建 `AlyClient.CSharpSDK.*.nupkg` 并作为 **GitHub Release 附件**上传，不会推送到 nuget.org；从 Release 下载 nupkg，或按下面的方式在本地打包。

可用下面两种方式之一。

### 方式 A：项目引用（源码在同一仓库时最省事）

```xml
<ItemGroup>
  <ProjectReference Include="..\client\aly-client-sdk\AlyClient.CSharpSDK\AlyClient.CSharpSDK\AlyClient.CSharpSDK.csproj" />
</ItemGroup>
```

跨仓库时把 SDK 目录整体拷贝进解决方案，或改用方式 B。

### 方式 B：打包成本地 nupkg

```bash
cd client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK
dotnet pack -c Release -o nupkg -p:Version=1.0.0
```

产物 `AlyClient.CSharpSDK.1.0.0.nupkg` 放进本地源（`NuGet.config` 里 `<add key="local" value="D:\nuget-local" />`）后即可用 `<PackageReference Include="AlyClient.CSharpSDK" Version="1.0.0" />`。

::: warning 本地 pack 会保留版本占位符
`AlyClient.CSharpSDK.targets` 里 `AlyClientVersion` 的默认值是字面量 `ALY_CLIENT_VERSION_PLACEHOLDER`。release 流水线在打包前会把这个字符串替换成真实版本号，**本地 `dotnet pack` 不会**。此时宿主构建出的下载地址是 `.../releases/download/vALY_CLIENT_VERSION_PLACEHOLDER/aly-client.exe`，下载必然失败（只警告，不中断构建），`UpdateFolder` 里不会出现 `aly-client.exe`。

两种补救方式：

- 宿主构建时覆盖属性：`dotnet build -p:AlyClientVersion=1.0.0`，或在宿主 csproj 里写 `<AlyClientVersion>1.0.0</AlyClientVersion>`；
- 或者打包前手动把 `.targets` 中的占位符替换成版本号，与 release 流水线保持一致。
:::

## 部署布局：UpdateFolder

Aly 的客户端目录约定是「包目录 + 应用目录 + 更新目录」三层结构：

```text
PackageFolder/
├── ApplicationFolder/            # 当前生效的应用（apply 时整体替换）
│   ├── YourApp.exe
│   └── .updator/shared.json      # server_url、project_name、忽略规则
├── ApplicationFolder_1.0.0/      # 历史版本快照（回滚用）
└── UpdateFolder/
    ├── aly-client.exe            # 更新器（必需）
    ├── aly-client-ui.exe         # GUI 前端（AlyUpdateUIClient 必需）
    ├── client.json               # 主程序相对路径 + 需关闭进程
    └── version.json              # 版本状态机（自动维护，勿手改）
```

`AlyUpdateClient` 与 `AlyUpdateUIClient` 的默认路径都在 `AppDomain.CurrentDomain.BaseDirectory` 的上一级：`..\UpdateFolder\aly-client.exe` 与 `..\UpdateFolder\aly-client-ui.exe`。所以宿主进程的输出目录要落在能靠 `..\UpdateFolder` 找到更新器的位置：生产环境通常直接发布到 `ApplicationFolder/`，开发环境则是 `bin\Debug\net40\`。

开发目录的实际形态（`OutDir = bin\Debug\net40\`）：

```text
bin\Debug\
├── net40\YourApp.exe             # 宿主输出
└── UpdateFolder\                 # targets 落盘位置（$(OutDir)..\UpdateFolder\）
    ├── aly-client.exe
    ├── aly-client-ui.exe
    └── client.json
```

此时 `client.json` 里的主程序路径应写成 `../net40/YourApp.exe`；如果已经改成 `PackageFolder/ApplicationFolder` 布局，则写 `../ApplicationFolder/YourApp.exe`。

### client.json 的相对路径

`main_exe_relative_path` **相对于 UpdateFolder**（更新器所在目录 `config.ExeDir()`），不是相对进程工作目录，也不是相对包根目录：

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

| 想指向的位置 | 正确写法 |
|---|---|
| 与 UpdateFolder 同级的 `ApplicationFolder` 下的主程序 | `../ApplicationFolder/YourApp.exe` |
| 开发输出目录（`bin\Debug\net40`）里的主程序 | `../net40/YourApp.exe` |
| 包根目录下其他目录 | `../OtherFolder/App.exe` |

- 写成 `ApplicationFolder/YourApp.exe`（少了 `../`）会解析成 `UpdateFolder/ApplicationFolder/YourApp.exe`，`apply_update` 之后重启主程序失败，更新「完成」但应用没起来。
- 反斜杠在 Linux 上会被 `config.normalizeMainExeRelativePath` 换成 `/`，同一份包可以跨平台；统一写 `/` 更稳。
- `must_close_process_name` 是不带 `.exe` 的进程名数组，`apply_update` 会先发 `WM_CLOSE` 再在超时后强杀。
- 这两个字段由 `aly-client` 自己读取；SDK 的 `MustCloseProcessNames` 属性只是额外把值通过 `--must-close-process-name` 传给 `apply_update`/`rollback`。

字段完整语义见[配置参考](/docs/client/config)；更新器命令见[命令参考](/docs/commands)。

## 快速开始（最小宿主）

前置条件三项：宿主工程引用了 `AlyClient.CSharpSDK`（含 `Newtonsoft.Json`）、`UpdateFolder` 里已有 `aly-client.exe` 与 `aly-client-ui.exe`、`client.json` 已按实际目录改过。

完整可编译的最小宿主（`net40` + C# 5）。仓库里的 `client/aly-client-sdk/samples/MinimalHost` 就是同一份可编译工程（`net40` + `net8.0`），可直接作为起点：

```csharp
using System;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

namespace MyApp
{
    internal static class Program
    {
        private static void Main()
        {
            // 默认定位 ..\UpdateFolder\aly-client-ui.exe
            AlyUpdateUIClient ui = new AlyUpdateUIClient();
            ui.Language = "zh-CN";

            AlyResponse result = ui.LaunchAuto();
            if (!result.IsSuccess)
            {
                Console.WriteLine("更新组件不可用：" + result.ErrorMsg);
            }

            // 宿主的初始化与消息循环
            Console.WriteLine("宿主已启动，按回车退出");
            Console.ReadLine();

            ui.Dispose();
        }
    }
}
```

这段代码只用到构造函数、`LaunchAuto()` 与 `Dispose()`；`ui.Dispose()` 不会结束已经打开的更新窗口。需要自己控制更新 UI 与时机时，改用下一节的 `AlyUpdateClient`。

## 集成方式一：AlyUpdateUIClient（推荐）

宿主不画更新界面、不轮询、不处理重启：全部交给 `aly-client-ui.exe`。

```csharp
using System;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

class Program
{
    static void Main()
    {
        AlyUpdateUIClient ui = new AlyUpdateUIClient();   // 默认 ..\UpdateFolder\aly-client-ui.exe
        ui.Language = "zh-CN";                            // zh-CN / en-US
        ui.Exited += OnUiExited;

        AlyResponse launched = ui.LaunchAuto();           // 启动时静默检查，有更新才弹窗
        if (!launched.IsSuccess)
        {
            Console.WriteLine("拉起更新界面失败：" + launched.ErrorMsg);
        }

        // 宿主自己的初始化与消息循环
        Console.ReadLine();

        ui.Dispose();                                     // 只释放句柄，不结束窗口
    }

    static void OnUiExited(int exitCode)
    {
        Console.WriteLine("aly-client-ui 已退出，exit code = " + exitCode);
    }
}
```

按场景选择启动模式：

| 模式 | 快捷方法 | 行为 |
|---|---|---|
| `AlyUpdateUiMode.Auto` | `LaunchAuto()` | 窗口默认隐藏，只有确认有更新才显示；无更新或服务器不可达时静默退出。**应用启动时调用** |
| `AlyUpdateUiMode.Check` | `LaunchCheck()` | 用户主动点「检查更新」：先显示「正在检查」，无更新/出错也会提示并自动关闭，避免「点了没反应」 |
| `AlyUpdateUiMode.Update` | `LaunchUpdate()` | 直接进入下载 + 应用，不再询问 |
| `AlyUpdateUiMode.Rollback` | `LaunchRollback()` | 显示可回滚版本列表，由用户在窗口内选择 |
| `AlyUpdateUiMode.Silent` | `LaunchSilent()` | 与 `Auto` 同口径的静默检查：无更新/出错不打扰，确认有更新才显示 |

也可以直接调 `Launch(AlyUpdateUiMode.Check)`，效果等同 `LaunchCheck()`。

要点：

- `Launch` 是非阻塞的，只负责拉起进程；下载、应用、关闭并重启主程序都在 UI 里完成。
- **单实例**：本对象持有的进程还活着时再次 `Launch`，SDK 直接返回 `IsSuccess=true`，不会再拉起第二个进程。若窗口不是由本对象拉起的（例如用户从开始菜单打开，或另一处代码拉过），`Launch` 仍会启动新进程，该进程通过命名事件把「手动检查请求」转发给既有实例，然后以退出码 0 结束——这是正常路径，宿主不需要区分「新窗口」和「既有实例接管」。
- `aly-client-ui.exe` 必须存在：缺失时 `Launch` 返回 `IsSuccess=false`，`ErrorMsg` 为 `aly-client-ui.exe not found: <路径>`。
- `apply_update` 之后 UI 会按 `client.json` 关闭并重启主程序，宿主不需要自己重启。
- `Dispose` 只释放 `Process` 句柄、退订事件，**不会结束**已经在运行的 UI 窗口。
- 不需要 GUI 时把 `AlyIncludeClientUi` 设为 `false` 跳过部署，改用下面的 `AlyUpdateClient`。

## 集成方式二：AlyUpdateClient 事件循环

构造函数一返回，后台线程就已经开始工作：先做更新器自更新，再进入「检查 → 下载 → 应用」轮询循环。宿主通过事件感知阶段变化。

```csharp
using System;
using System.Diagnostics;
using System.Threading;
using AlyClient.CSharpSDK;

class Program
{
    static AlyUpdateClient _client;
    static ManualResetEvent _userConfirmed = new ManualResetEvent(false);

    static void Main()
    {
        _client = new AlyUpdateClient();                       // 默认 ..\UpdateFolder\aly-client.exe
        _client.MustCloseProcessNames = Process.GetCurrentProcess().ProcessName;
        _client.IdlePollIntervalSeconds = 300;

        _client.StatusChanged += OnStatusChanged;
        _client.ErrorStatusChanged += OnErrorChanged;
        _client.RequestDownloadUpdate += OnRequestDownload;
        _client.RequestApplyUpdate += OnRequestApply;

        Console.ReadLine();

        _client.Cancel();
        _client.Dispose();
    }

    static void OnStatusChanged(AlyClientStatus status, string message)
    {
        // 事件在后台线程触发，跨线程操作 UI 需要 marshal
        Console.WriteLine(string.Format("[{0}] {1}", status, message));
    }

    static void OnErrorChanged(string message)
    {
        if (message == null)
        {
            Console.WriteLine("错误已恢复");
            return;
        }
        Console.WriteLine("[error] " + message);
    }

    static void OnRequestDownload(string newVersion)
    {
        // 回调在后台循环线程上同步执行：返回即代表「用户确认」，SDK 立刻开始下载
        Console.WriteLine("发现新版本 " + newVersion + "，等待用户确认下载");
        _userConfirmed.WaitOne();
        _userConfirmed.Reset();
    }

    static void OnRequestApply(string newVersion)
    {
        Console.WriteLine("新版本 " + newVersion + " 已下载，等待用户确认应用");
        _userConfirmed.WaitOne();
        _userConfirmed.Reset();
    }

    static void Confirm()
    {
        _userConfirmed.Set();   // 真实代码在 UI 线程的「确定」按钮里调用
    }
}
```

事件与状态的对应关系：

```text
CheckUpdate ──has_update=false──▶ 空闲轮询（默认 300s + 抖动）
     │
     └─has_update=true──▶ DiscoveredUpdate ──▶ (非强更) RequestDownloadUpdate
                                   │
                                   ▼
                          DownloadingUpdate（带文件名与百分比）
                                   │
                                   ▼
                          DownloadedUpdate ──▶ (非强更) RequestApplyUpdate
                                   │
                                   ▼
                             ApplyUpdate ──▶ 独立进程 apply_update，主程序被重启
```

关键语义：

- **事件没有「拒绝」语义**：回调返回就视为确认，SDK 继续下一步。要中止整条流程只能 `Cancel()`。想让用户「取消下载」时，正确的做法是让宿主自己决定不弹更新入口，而不是在回调里返回假确认。
- **回调同步阻塞后台线程**：`RequestDownloadUpdate` 里死等会让下载不开始。用 `ManualResetEvent`/信号量等待，并在 UI 线程里 `Set`。
- `force_update = true` 时两个确认事件**不会触发**，SDK 直接下载并应用。
- 同一种错误重复出现不会重复触发 `ErrorStatusChanged`；恢复时以 `message = null` 通知一次。
- 事件处理器抛出的异常会被 SDK 捕获并写入日志，不会中断后台循环——所以别指望用异常中断流程。
- 后台轮询节奏：`check_update` 失败固定 5 秒后重试；无更新时按 `IdlePollIntervalSeconds`（默认 300 秒 + 0–60 秒随机抖动，下限 60 秒）；`apply_update` 拉起后等 30 秒再继续（避免反复拉起进程）。
- 自更新在循环开始前执行，失败信息写入日志；它通过 `self_update` 命令委派替换，目标缺失或命令不可用时回退为直接替换。
- 日志文件在宿主基目录：`aly-client-sdk.log`。看到的 `self-update: ...`、`StatusChanged handler threw: ...` 都来自这里。

`Status`、`IsRunning`、`IsError` 等成员的准确语义见 [API 参考](/docs/sdk/api#alyupdateclient)。

## 直接调用 AlyApi（自定义时序）

`AlyApi` 的每个方法对应一条 `aly-client` 命令，返回值都是 `AlyResponse` / `AlyResponse<T>`：

| 方法 | 对应命令 | 返回模型 |
|---|---|---|
| `CheckSelfUpdateAsync(exePath)` | `check_self_update` | `CheckSelfUpdateData` |
| `SelfUpdateAsync(exePath)` | `self_update` | `SelfUpdateData` |
| `CheckUpdateAsync(exePath)` | `check_update` | `CheckUpdateData` |
| `DownloadUpdateAsync(exePath, progress)` | `download_update` | 无 data |
| `ApplyUpdateAsync(exePath, names, timeout)` | `apply_update` | 无 data |
| `ListRollbackVersionsAsync(exePath)` | `list_rollback_versions` | `ListRollbackData` |
| `RollbackAsync(exePath, version, names, timeout)` | `rollback` | `RollbackData` |

```csharp
using System;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

class Direct
{
    static void Run(string exePath)
    {
        AlyResponse<CheckUpdateData> check = AlyApi.CheckUpdateAsync(exePath).Result;
        if (!check.IsSuccess)
        {
            Console.WriteLine("check 失败：" + check.ErrorMsg);
            return;
        }

        if (check.Data.HasUpdate && check.Data.NeedDownloadUpdate)
        {
            Action<string, double> onProgress = delegate(string file, double ratio)
            {
                Console.WriteLine(file + " " + ((int)(ratio * 100)) + "%");
            };

            AlyResponse download = AlyApi.DownloadUpdateAsync(exePath, onProgress).Result;
            if (!download.IsSuccess)
            {
                Console.WriteLine("download 失败：" + download.ErrorMsg);
                return;
            }
        }

        // 独立进程执行，立即返回；随后当前进程会被关闭
        AlyResponse apply = AlyApi.ApplyUpdateAsync(exePath);
        Console.WriteLine("apply 拉起结果：" + apply.IsSuccess);
    }
}
```

超时与失败形态（默认 30 秒、回滚 600 秒、下载停流 1800 秒）见 [API 参考](/docs/sdk/api#超时与失败形态)。

## 构建期自动部署 targets

SDK 包里的 `AlyClient.CSharpSDK.targets` 在宿主构建时执行 `AlyDeployToUpdateFolder`（`BeforeTargets="Build"`），把更新器与 GUI 下载到 `UpdateFolder`。所有属性都可覆盖：

| MSBuild 属性 | 默认值 | 说明 |
|---|---|---|
| `AlyUpdateFolder` | Windows：`$(OutDir)..\UpdateFolder\`；其他平台：`$(OutDir)../UpdateFolder/` | 落盘目录 |
| `AlyClientVersion` | `ALY_CLIENT_VERSION_PLACEHOLDER` | 拼进下载 URL 的版本号（`v$(AlyClientVersion)`）；本地 pack 时就是占位符 |
| `AlyClientDownloadUrl` | `https://github.com/lishuangquan1987/aly/releases/download/v$(AlyClientVersion)/aly-client.exe` | 更新器下载地址（Linux 附件名为 `aly-client-linux-$(AlyRid)`） |
| `AlyClientUiDownloadUrl` | 同一路径下的 `aly-client-ui.exe`（Linux 为 `aly-client-ui-linux-$(AlyRid)`） | GUI 下载地址 |
| `AlyIncludeClientUi` | `true` | 是否下载 GUI；设为 `false` 可跳过 |
| `AlyRid` | `amd64` | 仅 Linux 生效：`amd64` / `arm64` |

行为：

- 目标文件已存在则跳过下载（幂等），不会覆盖手改过的 `client.json`，也不会覆盖已部署的更新器。
- 下载失败只警告（`ContinueOnError`），**不让宿主构建失败**。也就是说「构建成功」不等于「更新器已就位」，请自己确认 `UpdateFolder/aly-client.exe` 存在。
- `client.json` 缺失时写入默认模板（`main_exe_relative_path` 为 `../ApplicationFolder/YourApp.exe`，`must_close_process_name` 为 `["YourApp"]`），必须按实际目录结构改。
- Windows 用 PowerShell（显式设置 TLS 1.2）下载；Linux 用 `curl`（失败回退 `wget`），落盘改名为 `aly-client` / `aly-client-ui` 并 `chmod 0755`——与 `config.UpdaterFileName()` 的平台命名约定一致。
- 平台判断用 `$(OS)`，兼容 VS2013/2015 在 net40 上的构建。

Linux 交叉部署示例：

```bash
dotnet build -p:AlyRid=arm64 -p:AlyClientVersion=1.0.0
```

## 常见坑

| 现象 | 原因与处理 |
|---|---|
| 构建成功但 `UpdateFolder` 里没有 `aly-client.exe` | 下载失败只警告。多半是 `AlyClientVersion` 还是占位符、或网络不可达 GitHub Releases；用 `-p:AlyClientVersion=<版本>` 覆盖后重试 |
| `check_update` 一直 `has_update=false`，服务器上明明有新版本 | 宿主机离线时客户端会返回 `has_update=false`（无法与服务端核对就不上报），联网后自动继续；先确认 `.updator/shared.json` 的 `server_url` 可达 |
| 更新后主程序没启动 | `client.json` 的 `main_exe_relative_path` 少写了 `../`，或指向的 exe 名与实际不符。按上文「client.json 的相对路径」中的对照表核对 |
| 应用被直接杀掉，没机会保存设置 | 没设置 `MustCloseProcessNames`。更新器在替换阶段探测到目录被占用会强杀；设为 `Process.GetCurrentProcess().ProcessName`（不带 `.exe`） |
| 非强更流程直接自动更新，没等用户确认 | 没有订阅 `RequestDownloadUpdate` / `RequestApplyUpdate`：事件没有处理器时 SDK 视为已确认，直接下载并应用。需要询问用户就必须订阅 |
| 非强更流程卡在「发现新版本」不动 | 确认回调里在等一个永远不会 `Set` 的事件，或回调做了长时间阻塞的工作。回调运行在后台循环线程上，任何等待都必须有对端在别的线程触发 |
| `await` 在 net40 编译不过 | .NET 4.0 没有 async 支持类型。改用 `.Result`，或引用 `Microsoft.Bcl.Async` |
| 在 UI 线程 `.Result` 后界面假死 | `check_update` 默认最多等 30 秒，`download_update` 可能更久。放到后台线程调用 |
| `apply` 之后宿主还在收尾，结果进程突然消失 | `ApplyUpdateAsync` 是拉起独立进程后立即返回，`apply_update` 会关闭当前进程。不要在它之后做清理工作，把清理放到退出钩子里 |
| Linux 上找不到更新器 | 默认路径是 Windows 形式的 `..\UpdateFolder\aly-client.exe`，Linux 上没有 `.exe` 且反斜杠不是分隔符。显式传路径：`new AlyUpdateClient("/opt/app/UpdateFolder/aly-client")` |
| `LaunchAuto` 返回失败说找不到 `aly-client-ui.exe` | GUI 未被部署（`AlyIncludeClientUi=false`，或下载失败）。补部署，或改用 `AlyUpdateClient` |
| 两个更新窗口同时出现 | 一个宿主里同时启用了 `AlyUpdateUIClient` 与 `AlyUpdateClient`。只保留一种 |
| 关闭宿主时卡住几秒 | `Dispose` 会等后台循环退出，最长 5 秒；轮询睡眠本身是可中断的，不会睡满一个空闲周期 |

## 相关文档

- [C# SDK API 参考](/docs/sdk/api)
- [aly-client 命令参考](/docs/commands)
- [客户端配置参考](/docs/client/config)
- [下载与安装](/docs/download)
- [自更新设计](/docs/self-update)
- [核心概念](/docs/concepts)