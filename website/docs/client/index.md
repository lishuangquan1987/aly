---
title: 宿主集成
description: 宿主怎么拉起更新器、怎么读 JSON、宿主退出时子进程怎么跟随，以及更新锁、安全边界与回滚的边界。
---

# 宿主集成

`aly-client` 不常驻、不安装服务，由宿主在需要的时机拉起；每次执行一个子命令，结果以 JSON 写到 stdout。真正的版本切换由本地目录 rename 完成，不依赖网络，也不要求主程序参与文件替换。

最短可用路径见[接入更新器](/docs/start/integrate)；这一页讲跑通之后需要处理的事。更新器只负责“把本机文件换成服务端发布的新版本”，什么时候提示用户、是否强制更新由宿主或 GUI 决定。

## 组件与来源

| 组件 | 作用 |
|---|---|
| `aly-client(.exe)` | 更新器本体：检查、下载、应用、回滚、自更新 |
| `aly-client-ui(.exe)` | 可选更新窗口，包装同样的命令并展示进度 |
| `AlyClient.CSharpSDK` | .NET 宿主的封装：后台状态机、事件、静态 API、GUI 启动器 |
| `aly-server` / `aly-publish` | 保存版本元数据与文件、发布新版本 |

两个更新器二进制都来自 AlyPublish 安装包的 `client/<rid>/`：GUI 初始化项目时复制进 `UpdateFolder/`，也可以手工放置。目录与 `<rid>` 的含义见[发行包结构与分发](/docs/guide/package)。

## 宿主生命周期：宿主退出就跟着退

`aly-client` 与 `aly-client-ui` 都是宿主拉起的子进程，**宿主退出后它们跟随退出**，不会在用户关掉主程序之后还留在进程表里。

唯一的例外是 **apply 阶段**：更新器要关闭宿主进程、原子替换主目录并重启主程序，宿主本来就会（也必须）消失——这时更新器必须继续把这一步做完，不能半路退出。判定两条取或：

- 命令级：当前命令就是 `apply_update`；
- 状态级：`UpdateFolder/version.json` 的 `version_status == applying`（崩溃恢复窗口——那次没有命令在跑，但主目录可能已丢失，同样要续跑）。

退出是优雅收尾：先写日志再退。更新锁不需要显式释放——锁文件按“持有者进程已退出”判陈旧，残留锁由下一次运行自动清理。规范原文见 `spec/host-contract.md` §15。

## 推荐调用顺序

```text
启动
 ├─ 需要时 self_update（裸控制台部署）
 ├─ check_update
 │   ├─ has_update=false            → 结束
 │   ├─ need_download_update=true   → download_update → apply_update
 │   └─ need_download_update=false  → apply_update（崩溃恢复 / 已下载待应用）
 └─ 退出
```

每条命令的参数与返回字段见[命令参考](/docs/commands)。

## 读 JSON 的五条规则

- **只认 `isSuccess`，不要认退出码**。业务失败（下载校验失败、没有待应用更新、锁被占用等）也以退出码 `0` 结束；只有未知子命令/无子命令返回 `1`，参数解析失败返回 `2`。
- **进度行不是最终结果**。`download_update` 的 `BEGIN` / `START` / `PROGRESS` / `DONE` / `FAIL` 行都带 `data.status`；最终完成行的 `data` 是 `null`。读到 `data:null` 才表示下载完成。
- **不要把参数拼进 `cmd.exe`**。直接传参，避免引号、空格和 `%` 展开问题。
- **配置由更新器自己读**。宿主通常不需要传 `--url` / `--project-name`；`client.json` 与 `MainFolder/.updator/shared.json` 必须就位。
- **关闭进程**。把主程序进程名放进 `client.json` 的 `must_close_process_name`，或通过 SDK 的 `MustCloseProcessNames` 传入；否则更新器在 rename 失败时会探测并强杀占用者，宿主没有优雅退出的机会。

需要图形界面时直接用 `aly-client-ui` 替代宿主自绘 UI，见[更新窗口](/docs/client/ui)。

## 进程与锁

- 全局锁 `.aly.lock` 保证同一终端同一时刻只有一个更新流程；持有进程存活时锁有效，陈旧残留锁 30 分钟 TTL、5 分钟心跳后自动清理。宿主、GUI、SDK 多入口并发触发是安全的。
- 不要手工编辑 `version.json`。状态与磁盘内容错配会让更新器拒绝替换，只能靠 `download_update` 重写状态修复。
- 宿主退出跟随退出的唯一例外是 apply 阶段，见上一节。

## 安全边界

- 服务端 REST API **没有鉴权**，应部署在受信内网或前置网关之后，不要直接暴露到公网。
- 更新器对 `MainFolder`、`PackageFolder` 有写权限；终端上应限制普通用户对这些目录的写权限，避免本地篡改版本目录或 `version.json`。
- 下载文件以服务端清单的 MD5 + SHA256 校验，但清单本身来自服务端；安全模型是“信任你自己的服务端”，不是端到端签名。

## 回滚

- 每次成功 apply 会把旧主目录改名为 `ApplicationFolder_{version_previous}` 快照；`rollback --version` 直接交换目录，不需要网络。
- `apply_update` / `rollback` 入口会机会式保留最近 3 个快照；`clean` 可显式回收。当前版本、上一版本与回滚目标永不被清理。
- 回滚不是“锁定版本”：回滚成功后若服务端版本仍更高，下一轮 `check_update` 会再次升级。当前没有 rollback pin 策略，需要运维侧同步下架/调整服务端版本。
- 进入替换阶段后的回滚失败会恢复回滚前状态，并在主目录仍存在时启动当前版本主程序，不会把机器留在“状态已应用、磁盘仍旧版”的错配状态。

## 相关页面

- [接入更新器](/docs/start/integrate)：三步跑通
- [客户端命令参考](/docs/commands) · [配置文件参考](/docs/client/config)
- [更新窗口](/docs/client/ui) · [C# SDK 集成](/docs/sdk)
- [核心概念](/docs/concepts) · [排障](/docs/troubleshooting)
