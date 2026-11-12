---
title: 更新器概览与集成
description: aly-client 的职责、部署布局、版本生命周期、宿主集成方式（子进程 JSON 与 C# SDK）以及安全与回滚要点。
---

# 更新器概览与集成

`aly-client` 是运行在终端机器上的更新器。它不常驻、不安装服务，由宿主应用在需要的时机启动；每次执行一个子命令，结果以 JSON 写到 stdout。真正的版本切换由本地目录 rename 完成，不依赖网络，也不要求主程序参与文件替换。

可选组件：

| 组件 | 作用 |
|---|---|
| `aly-client` / `aly-client.exe` | 更新器本体：检查、下载、应用、回滚、自更新 |
| `aly-client-ui` / `aly-client-ui.exe` | 可选的右下角 GUI，包装 `aly-client` 的命令并展示进度/询问 |
| `AlyClient.CSharpSDK` | .NET 宿主的封装：后台状态机、事件、静态 API、GUI 启动器 |
| `aly-server` | 保存项目元数据与文件清单，提供 REST API |
| `aly-publish` | 发布侧工具：扫描、暂存、上传并发布版本 |

预构建的更新器不再单独发布：从 AlyPublish 安装包的 `client/<rid>/` 目录取（GUI 初始化项目时会自动复制进 `UpdateFolder/`）。点文件名下载安装包：

<ReleaseDownloads group="package" compact :show-latest-hint="false" :show-version="false" />

更新器只负责“把本机文件换成服务端发布的新版本”，不决定什么时候提示用户、是否强制更新。提示策略由宿主或 GUI 决定。

## 宿主生命周期：宿主退出就跟着退

`aly-client` 与 `aly-client-ui` 都是宿主拉起的子进程，**宿主退出后它们跟随退出**，不会在用户关掉主程序之后还留在进程表里。

唯一的例外是 **apply 阶段**：更新器要关闭宿主进程、原子替换主目录并重启主程序，宿主本来就会（也必须）消失——这时更新器必须继续把这一步做完，不能半路退出。判定两条取或：

- 命令级：当前命令就是 `apply_update`；
- 状态级：`UpdateFolder/version.json` 的 `version_status == applying`（崩溃恢复窗口——那次没有命令在跑，但主目录可能已丢失，同样要续跑）。

退出是优雅收尾：先写日志再退。更新锁不需要显式释放——锁文件按“持有者进程已退出”判陈旧，残留锁由下一次运行自动清理。规范原文见 `spec/host-contract.md` §15。

## 部署布局

README 的终端部署布局如下（Windows 文件名；Linux 无 `.exe`）：

```text
PackageFolder/
├── ApplicationFolder/                 # 当前运行目录（MainFolder）
│   ├── YourApp.exe
│   └── .updator/
│       └── shared.json                # server_url、project_name、规则
├── ApplicationFolder_1.0.0/           # 历史版本快照（可回滚）
├── ApplicationFolder_1.0.1/           # 已下载的待应用版本（download 产物）
└── UpdateFolder/                      # 更新器目录
    ├── aly-client.exe                 # 更新器本体
    ├── aly-client-ui.exe              # 可选 GUI
    ├── client.json                    # 主程序路径、需关闭的进程
    ├── version.json                   # 版本状态机
    ├── .aly.lock                      # 更新互斥锁（运行时）
    ├── update.log                     # 更新日志
    ├── release_manifest.json          # 发布清单缓存（删除同步用）
    └── aly-client.exe.old             # 自更新备份（运行时）
```

关键路径：

| 名称 | 位置 | 说明 |
|---|---|---|
| `UpdateFolder` | `PackageFolder/UpdateFolder` | 更新器所在目录，也是 `client.json` / `version.json` 的位置 |
| `MainFolder` | 由 `client.json` 的 `main_exe_relative_path` 推导 | 当前版本目录，默认布局下是 `PackageFolder/ApplicationFolder` |
| `PackageFolder` | `UpdateFolder` 的父目录 | 版本快照与待应用目录都放在这里 |
| 版本目录 | `PackageFolder/ApplicationFolder_{version}` | 下载产物或历史快照 |
| 共享配置 | `MainFolder/.updator/shared.json` | 随版本包分发，客户端从当前主目录读取 |

`main_exe_relative_path` 是**相对 `UpdateFolder`** 的路径，例如：

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

这里 `MainFolder` 就是 `../ApplicationFolder`。字段与规则见[配置参考](/docs/client/config)。

## 源码模块

| 目录 | 说明 |
|---|---|
| `client/aly-client/` | 更新器本体，Go 1.10、32 位（`GOARCH=386`）、Windows XP 兼容，GOPATH 模式 |
| `client/aly-client-ui/aly-client-winui/` | Windows GUI，Go 1.10.8 + `lxn/walk` |
| `client/aly-client-ui/aly-client-linuxui/` | Linux GUI，Go 1.27 + MyGo 原生 UI，运行时依赖 GTK3 |
| `client/aly-client-sdk/` | C# SDK（`net40` + `netstandard2.0`） |

构建与跨平台细节见[构建与跨平台](/docs/build)。

## 生命周期

```text
applied/空 ──download_update──> downloaded ──apply_update──> applying ──成功──> applied
                                      ▲                          │
                                      └──────── 失败/崩溃 ────────┘

applied ──rollback --version X──> applying ──成功──> applied（version = X）
```

| 阶段 | 命令 | 发生了什么 |
|---|---|---|
| 检查 | `check_update` | 比较服务端最高版本与本地版本，返回 `has_update` / `need_download_update`；只读 |
| 下载 | `download_update` | 只下载差异文件，逐文件校验 MD5 + SHA256；把主目录未变化文件物化进新版本目录；写 `downloaded` |
| 应用 | `apply_update` | 校验下载产物，关闭业务进程，三段 rename 完成替换，写 `applied`，执行后置脚本并启动主程序 |
| 回滚 | `rollback` | 用历史快照原子替换当前目录；不需要网络 |
| 自更新 | `self_update` | 用主目录中随版本分发的新版更新器替换 `UpdateFolder` 中的运行副本 |
| 清理 | `clean` | 删除超出保留数量的快照、临时文件与过期日志 |

几个关键点：

- **物化在下载阶段完成**。`download_update` 会把当前主目录中未变化的文件复制进待应用目录（排除 `un_copy_*`），因此 `apply_update` 只需要 rename，应用停机窗口从“复制整包”的分钟级降到秒级。
- **崩溃可恢复**。`apply_update` 进入替换前写 `applying`，最终激活 rename 前写 `apply_stage=activating`。下次 `check_update` / `apply_update` 会据此判断改名是否已经发生，并补完剩余步骤。
- **离线不丢状态**。`downloaded` 状态下若服务端不可达，`check_update` 不上报“待应用更新”，但已下载内容与 `version.json` 保持不变；`applying`（崩溃恢复 / 回滚续跑）是例外，断网也照常上报，否则机器无法自愈。
- **下载失败不写待应用状态**。单文件重试 3 次仍失败时，最终结果带 `failed_files`，`version.json` 仍是上一状态；重试只会重下失败文件。

完整状态字段见[version.json](/docs/client/config#version-json)，命令细节见[命令参考](/docs/commands)。

## 宿主集成

### 方式一：子进程 + JSON

宿主直接启动 `UpdateFolder/aly-client.exe`，逐行读取 stdout。每个命令至少输出一行 JSON；`download_update` 会先输出多行进度，最后一行才是最终结果。

推荐顺序：

```text
启动
 ├─ 需要时 self_update（裸控制台部署）
 ├─ check_update
 │   ├─ has_update=false            → 结束
 │   ├─ need_download_update=true   → download_update → apply_update
 │   └─ need_download_update=false  → apply_update（崩溃恢复 / 已下载待应用）
 └─ 退出
```

最小 C# 调用示例：

```csharp
using System.Diagnostics;
using Newtonsoft.Json.Linq;

var psi = new ProcessStartInfo(updaterExePath, "check_update")
{
    UseShellExecute = false,
    RedirectStandardOutput = true,
    CreateNoWindow = true,
};

using (var p = Process.Start(psi))
{
    string line;
    while ((line = p.StandardOutput.ReadLine()) != null)
    {
        var msg = JObject.Parse(line);
        var data = msg["data"] as JObject;

        // download_update 的进度行：继续读，直到最终结果
        if (data != null && data["status"] != null)
            continue;

        if ((bool)msg["isSuccess"])
        {
            // 读取 data.has_update / need_download_update / new_version ...
        }
        else
        {
            // 业务失败：读取 msg["errorMsg"]，不要看退出码
        }
    }
}
```

集成要点：

- **只认 `isSuccess`，不要认退出码**。业务失败（下载校验失败、没有待应用更新、锁被占用等）也以退出码 `0` 结束。只有未知子命令/无子命令返回 `1`，参数解析失败返回 `2`。
- **进度行不是最终结果**。`download_update` 的 `BEGIN` / `START` / `PROGRESS` / `DONE` / `FAIL` 行都带 `data.status`；最终完成行的 `data` 是 `null`。读到 `data:null` 才表示下载完成。
- **不要把参数拼进 `cmd.exe`**。直接传 `ProcessStartInfo` 参数，避免引号、空格和 `%` 展开问题。
- **配置由更新器自己读**。宿主通常不需要传 `--url` / `--project-name`；`client.json` 与 `shared.json` 必须就位。
- **关闭进程**。把主程序进程名放进 `client.json` 的 `must_close_process_name`，或通过 SDK 的 `MustCloseProcessNames` 传入；否则更新器在 rename 失败时会探测并强杀占用者，宿主没有优雅退出的机会。
- 需要图形界面时，直接用 `aly-client-ui -mode auto` 替代宿主自绘 UI，见[GUI 前端](/docs/client/ui)。

### 方式二：C# SDK

| 类型 | 用途 |
|---|---|
| `AlyUpdateClient` | 后台循环 `check → download → apply`，通过 `StatusChanged` / `RequestDownloadUpdate` / `RequestApplyUpdate` 驱动宿主 UI |
| `AlyApi` | 静态方法直调，每个方法对应一个 `aly-client` 命令；适合自己控制时序的宿主 |
| `AlyUpdateUIClient` | 启动 `aly-client-ui`，把整个生命周期和重启交给 GUI |

更新器二进制来自 **AlyPublish 安装包**的 `client/<rid>/`：GUI 初始化项目时会把它们复制进同级 `UpdateFolder/`；SDK 的 targets 只建目录并在缺失时写默认 `client.json`（不再下载二进质）。`AlyUpdateClient` 启动时还会委派 `self_update`，保证更新器副本不落后。

完整用法见 [C# SDK 集成](/docs/sdk)，方法级参数见 [SDK API 参考](/docs/sdk/api)。

## 命令索引

| 命令 | 用途 | 参考 |
|---|---|---|
| `check_update` | 检查是否有新版本 | [命令参考](/docs/commands#check-update) |
| `check_diff` | 列出需要下载的差异文件 | [命令参考](/docs/commands#check-diff) |
| `download_update` | 下载差异文件并物化未变化文件 | [命令参考](/docs/commands#download-update) |
| `apply_update` | 原子替换主目录并重启主程序 | [命令参考](/docs/commands#apply-update) |
| `list_rollback_versions` | 列出可回滚版本 | [命令参考](/docs/commands#list-rollback-versions) |
| `rollback` | 回滚到指定版本 | [命令参考](/docs/commands#rollback) |
| `clean` | 清理快照与临时文件 | [命令参考](/docs/commands#clean) |
| `check_self_update` | 检查更新器自身是否需要更新 | [命令参考](/docs/commands#check-self-update) |
| `self_update` | 用主目录新版更新器替换自身 | [命令参考](/docs/commands#self-update) |

配置字段见[配置参考](/docs/client/config)，GUI 模式与参数见[GUI 前端](/docs/client/ui)。

## 安全与回滚

### 安全边界

- 服务端 REST API **没有鉴权**，应部署在受信内网或前置网关之后，不要直接暴露到公网。
- 更新器对 `MainFolder`、`PackageFolder` 有写权限；终端上应限制普通用户对这些目录的写权限，避免本地篡改版本目录或 `version.json`。
- 下载文件以服务端清单的 MD5 + SHA256 校验，但清单本身来自服务端；安全模型是“信任你自己的服务端”，不是端到端签名。
- 全局锁 `.aly.lock` 防止同一终端并发执行下载/应用/回滚/清理。不要手工删除锁，除非确认没有更新进程且锁已陈旧（30 分钟 TTL，5 分钟心跳）。
- 不要手工编辑 `version.json`。状态与磁盘内容错配会让更新器拒绝替换，只能靠 `download_update` 重写状态修复。

### 回滚

- 每次成功 apply 会把旧主目录改名为 `ApplicationFolder_{version_previous}` 快照；`rollback --version` 直接交换目录，不需要网络。
- `apply_update` / `rollback` 入口会机会式保留最近 3 个快照；`clean` 可显式回收。当前版本、上一版本与回滚目标永不被清理。
- `version_previous` 为空（首次更新）时会产生无版本号备份 `ApplicationFolder_`，成功后会被删除，它不在回滚列表里。
- 回滚不是“锁定版本”：回滚成功后若服务端版本仍更高，下一轮 `check_update` 会再次升级。当前没有 rollback pin 策略，需要运维侧同步下架/调整服务端版本。
- 进入替换阶段后的回滚失败会恢复回滚前状态，并在主目录仍存在时启动当前版本主程序，不会把机器留在“状态已应用、磁盘仍旧版”的错配状态。

更多设计说明见[自更新设计](/docs/self-update)与[核心概念](/docs/concepts)。