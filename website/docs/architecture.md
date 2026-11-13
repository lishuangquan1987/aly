---
title: 系统架构
description: Aly 的组件、发布到应用的数据流、双轨工具链与 GUI 桥接方式。
---

# 系统架构

```text
发布方（开发者 / CI / 发布员）                    终端用户机器
┌──────────────────────────────────────┐      ┌────────────────────────────────────┐
│  AlyPublish.exe（Avalonia，可选）     │      │  宿主应用 YourApp.exe              │
│      │ 子进程 + --json                │      │      │ 子进程 / C# SDK / GUI     │
│      ▼                               │      │      ▼                             │
│  aly-publish（cobra CLI）────────────┼──┐   │  aly-client-ui（可选窗口）         │
│  config / status / add / push        │  │   │      │ 委派                       │
└──────────────────────────────────────┘  │   │  aly-client（更新器）              │
                                          ▼   │  check → download → apply          │
                              ┌──────────────┐│  self_update（自更新闭环）         │
                              │  aly-server  ││  ┌───────────────────────────────┐ │
                              │ Gin · Ent ·  │◄┼──┤ REST：项目/文件/清单          │ │
                              │ SQLite       ││  └───────────────────────────────┘ │
                              │ /api/*       ││                                    │
                              │ data/{项目}/ ││  PackageFolder/                    │
                              └──────────────┘│  ├ ApplicationFolder/（当前）       │
                                              │  ├ ApplicationFolder_V1.0.0/（快照）│
                                              │  └ UpdateFolder/（更新器）          │
                                              └────────────────────────────────────┘
```

## 一次完整更新的数据流

1. **发布**：`aly-publish` 扫描产物目录，与服务端清单比对 → 上传新增/变更文件（大文件分片）→ `publish_version` 登记版本与日志，并把当前文件清单写入 `data/{项目}/.updator_manifest.json`。
2. **检查**：客户端 `check_update` 请求 `get_project_by_name` 与 `get_project_change_logs`，按 semver 比较最新版本，返回 `has_update` / `force_update` / `release_notes`。
3. **下载**：`download_update` 请求 `get_all_files` 取得哈希清单，只下载差异文件（MD5 + SHA256、`.part` 续传），并把未变化文件物化到新版本目录；随后用 `get_release_manifest` 做删除同步。
4. **应用**：`apply_update` 校验产物 → 关闭 `must_close_process_name` → 三次目录 rename → 拉起主程序。
5. **留痕**：`version.json` 状态机贯穿全程；任意一步崩溃都能在下次启动恢复。

## 双轨工具链

| 轨道 | 工具链 | 产物 | 目标 |
|---|---|---|---|
| **XP 轨** | Go 1.10.8 · GOARCH=386 · GOPATH | `aly-client.exe` + `aly-client-ui.exe`（walk） | Windows XP → Win11 |
| **现代轨** | Go 1.27.3（服务端、Linux UI）/ 1.25+（发布 CLI）· Modules | 服务端、发布 CLI、Linux 更新器 | Windows / Linux / macOS 服务端与工具 |

XP 轨必须锁在 Go 1.10.8（Go 1.11 起不再支持 XP），CI 对 PE 头（machine=0x014C、subsystem≤5.0）与 ComCtl32 manifest 做校验。

Linux GUI（`aly-client-linuxui`）基于 [MyGo](https://github.com/egoist/mygo)（纯 Go、无 cgo 的原生 UI 库），运行时仅依赖 GTK3。

## GUI 与更新器的桥接

`aly-client-ui` 不实现更新逻辑，只做表现层：

- 通过子进程调用 `aly-client` 命令并解析 stdout JSON；
- 右下角置顶窗口按需显示进度/暂停/完成；
- 启动时委派 `aly-client self_update`（更新器取锁执行替换），仅在缺失或旧版时本地兜底；
- 单实例：第二个实例把“手动检查”请求转发给已有实例后退出。

C# 与其它语言 SDK、GUI 遵循同一协议，见 [多语言 SDK](/docs/sdk/languages)。

## 发布 GUI 的语言

`AlyPublish` 通过 Irihi.Lingua + JSON 资源支持中文/English：工具栏可切换，`--lang zh-CN|en-US` 可指定启动语言。语言选择保存在 `%LOCALAPPDATA%\AlyPublish\settings.json`。

## 仓库结构

```text
aly/
├── server/                     # aly-server（Go 1.27.3 · Gin · Ent · SQLite）
├── client/
│   ├── aly-client/             # 更新器本体（XP 轨 + 现代轨共享）
│   │   ├── cmd/                #   各子命令
│   │   ├── selfupdate/         #   自更新编排
│   │   ├── util/               #   进程/文件工具
│   │   └── test/e2e/           #   fake main / mock server
│   ├── aly-client-ui/
│   │   ├── aly-client-winui/   # XP 轨 Windows GUI（walk）
│   │   └── aly-client-linuxui/ # Linux GUI（MyGo）
│   └── aly-client-sdk/         # 多语言 SDK（C#/TS/Go/Python/Java/Rust/C++/Pascal/COM）
├── publish/
│   ├── publish-cli/            # 发布 CLI（cobra）
│   └── publish-gui-csharp/     # 发布桌面端（Avalonia 12.1 · .NET 8）
├── test/tc/                    # 148 条端到端用例
├── deploy/                     # 部署脚本
└── website/                    # 本站点（VitePress）
```