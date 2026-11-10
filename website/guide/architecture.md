# 系统架构

## 全景图

```text
           发布方（开发者/CI/发布员）                       终端用户机器
┌──────────────────────────────────────┐      ┌───────────────────────────────────┐
│  aly-publish-gui (Avalonia)          │      │  宿主应用 your_app.exe            │
│      │ 子进程                        │      │      │ 子进程 + JSON 协议         │
│      ▼                               │      │      ▼                           │
│  aly-publish (cobra CLI) ────────────┼──┐   │  aly-client-ui (可选 GUI)         │
│  config / status / add / push        │  │   │      │ 委派                       │
└──────────────────────────────────────┘  │   │  aly-client (更新器)              │
                                          ▼   │  check → download → apply        │
                              ┌──────────────┐│  self_update（自更新闭环）        │
                              │  aly-server  ││  ┌─────────────────────────────┐ │
                              │ Gin · Ent ·  │◄┼──┤ REST: check_update 等接口   │ │
                              │ SQLite       ││  └─────────────────────────────┘ │
                              │ /api/*       ││                                  │
                              └──────────────┘│  PackageFolder/                  │
                                              │  ├ ApplicationFolder/  (当前)    │
                                              │  ├ ApplicationFolder_V1.0.0/(备) │
                                              │  └ UpdateFolder/       (更新器)  │
                                              └───────────────────────────────────┘
```

## 数据流：一次完整的更新

1. **发布**：publish 工具扫描本地目录与上次发布的差异 → 上传变更文件（大文件分片）→ `publish_version` 登记版本号与变更日志。
2. **检查**：客户端 `check_update` 请求 `get_release_manifest` 对比本地与远端版本，返回 `has_update` / `force_update`。
3. **下载**：`download_update` 只拉取差异文件（MD5+SHA256 校验、断点续传），并物化未变化文件到 `ApplicationFolder_{new_version}`。
4. **应用**：`apply_update` 关闭宿主进程 → 三次目录改名完成替换 → 拉起新版本主程序。
5. **留痕**：`version.json` 状态机贯穿全程，任何一步崩溃都能在下次启动时恢复。

## 双轨工具链：XP 兼容与跨平台如何共存

Aly 的客户端采用**双轨构建**策略，这是"支持 Windows XP"与"现代平台全覆盖"两个目标同时成立的关键：

| 轨道 | 工具链 | 产物 | 目标平台 |
|---|---|---|---|
| **XP 轨** | Go 1.10.8 · GOARCH=386 · GOPATH 模式 | `aly-client.exe` + `aly-client-ui.exe`（walk） | Windows XP → Win11 |
| **现代轨** | Go 1.25+ / 1.27.3 · Go Modules | `aly-client`（控制台）+ `aly-client-ui`（MyGo/GTK3） | linux amd64/arm64 · darwin amd64/arm64 · freebsd amd64 |

两轨共享同一份业务代码（`cmd/`、`selfupdate/`、`util/`），差异只在 UI 壳与构建标签。Go 官方从 1.11 起不再支持 XP，所以 XP 轨必须锁死在 1.10.8——CI 对 XP 轨产物做 PE 头（machine=0x014C、subsystem≤5.0）与公共控件 manifest 校验。

现代轨的 Linux GUI（aly-client-linuxui）基于 [MyGo](https://github.com/lishuangquan1987/aly)（纯 Go 无 cgo 的原生 UI 库），交叉编译 `GOARCH=arm64` 即得 ARM 版本，运行时仅依赖 GTK3。

## GUI 与更新器的桥接

`aly-client-ui` 不实现任何更新逻辑，只是薄薄的表现层：

- 通过子进程调用 `aly-client` 命令，解析 stdout JSON；
- 屏幕右下角置顶窗口按需显示进度/暂停/完成；
- 自更新由 UI 启动时委派 `aly-client self_update`（更新器本体取锁执行替换），仅在更新器缺失或过旧时才走本地复制兜底。

C# SDK 与 GUI 遵循同一协议，见 [C# SDK 集成](/reference/sdk)。

## 仓库结构

```text
aly/
├── server/                 # aly-server（Go 1.25 · Gin · Ent · SQLite）
├── client/
│   ├── aly-client/         # 更新器本体（XP 轨 + 现代轨共享）
│   │   ├── cmd/            #   各子命令实现
│   │   ├── selfupdate/     #   自更新编排包（跨平台共用）
│   │   ├── util/           #   进程管理、文件工具（含平台分支）
│   │   └── ui/winui/       #   Windows GUI（walk · vendored）
│   ├── aly-client-ui/
│   │   ├── aly-client-winui/   # XP 轨 Windows GUI 入口
│   │   └── aly-client-linuxui/ # Linux 轨 GUI（MyGo · Go 1.27.3）
│   └── aly-client-sdk/     # C# SDK（net40 + netstandard2.0 · NuGet targets 多平台）
├── publish/
│   ├── publish-cli/        # 发布 CLI（cobra）
│   └── publish-gui/        # 发布桌面端（Avalonia 11.3 · .NET 8）
├── deploy/                 # 部署脚本（Windows/Linux）
├── test/tc/                # 135 条端到端用例的 Go 测试框架
└── website/                # 本站点（VitePress）
```
