---
title: 快速开始
description: 三步跑通 Aly：装一个 AlyPublish 安装包 → 用 GUI 初始化项目并发布首个版本 → 终端机器自己更新。
---

# 快速开始

目标是**最短路径跑通**：装一个安装包，用 GUI 点几下发出第一个版本，再让终端机器自己更新。每节都附上等价的命令行做法。

::: tip 前置：服务端要先起来
“发布版本”与“接入更新器”都假设服务端已启动。还没有服务端时，按[服务端部署](/docs/server/deploy)起一个；也可以先看下面第 0 步。
::

<QuickStartWizard />

## 0. 起服务端（只需一次）

```bash
./aly-server -p 2000
```

首次运行在程序旁创建 `aly.db` 与 `data/`；上传的文件固定放在 `data/{项目}/`。Linux 用 `./aly-server-linux-amd64`，Windows 用 `.\aly-server.exe`；`-db` 可改数据库文件路径。

## 1. 装 AlyPublish：一个包拿到 GUI + CLI + 更新器

Windows 下运行 `AlyPublish-Setup.exe`，或解压 `AlyPublish-win-x64.zip` 直接用；Linux 下解压 `AlyPublish-linux-x64.zip` 后运行 `./install.sh`。完整下载表与包内结构见[下载与安装](/docs/download)。

装好后就绪的东西：

```text
AlyPublish(.exe)                        # 发布 GUI
publish-cli/<rid>/aly-publish(.exe)     # 发布 CLI（GUI 用它干活）
client/<rid>/aly-client(-ui)            # 终端更新器（GUI 会复制进 UpdateFolder/）
payload.json                            # GUI 据此自动选本平台的构建
```

## 2. 用 GUI 初始化项目并发布（推荐路径）

1. 启动 `AlyPublish`，点**初始化项目**；
2. 第一步选服务端项目：填服务端地址（例如 `http://127.0.0.1:2000`）→ 获取列表 → 选中；服务端还没有项目时点**新建服务端项目**；
3. 第二步填本地目录（要分发给用户的产物目录，例如 `dist`）、显示名称、**主程序**（可选，但填了才会准备终端更新）；
4. 确认后 GUI 调用 `aly-publish config init ...` 写好 `.updator/`，并在项目**同级**创建 `UpdateFolder/`：从安装包 `client/<rid>/` 复制 `aly-client` 与 `aly-client-ui`，再写好 `client.json`；
5. 回主界面点**刷新**看变更 → **全部暂存** → 填版本号与变更说明 → **发布**。

::: warning 首个版本号
服务端创建项目时已占用 `V1.0.0`，所以第一次发布必须填 `V1.0.1` 或更高，否则会被防倒退校验拒绝。
::

等价的命令行做法（在产物目录里执行）：

```bash
aly-publish config init --server http://127.0.0.1:2000 --project myapp
aly-publish project create --server http://127.0.0.1:2000 --name myapp --title "My App"
aly-publish status
aly-publish add --all
aly-publish push --version V1.0.1 --message "首个版本"
```

命令行的 `aly-publish` 就在安装包的 `publish-cli/<rid>/` 里（Linux 从包里取出来记得 `chmod 0755`）。

## 3. 终端机器上的目录布局

```text
PackageFolder/
├── ApplicationFolder/              # 当前生效的应用
│   ├── YourApp.exe
│   └── .updator/shared.json        # server_url、project_name、忽略规则
├── ApplicationFolder_V1.0.0/       # 上一版本快照（回滚用）
└── UpdateFolder/
    ├── aly-client(.exe)            # 更新器
    ├── aly-client-ui(.exe)         # 可选 GUI
    ├── client.json                 # 主程序路径与待关闭进程
    └── version.json                # 状态机（自动维护）
```

`client.json` 的 `main_exe_relative_path` **相对于 `UpdateFolder/`**：

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

## 4. 客户端检查并应用

```bash
aly-client check_update      # 有无新版本：has_update / new_version / force_update
aly-client download_update   # 只下变更文件，MD5 + SHA256 双校验
aly-client apply_update      # 关进程 → 原子替换 → 重启主程序
```

业务失败通常仍返回退出码 0，请以 JSON 的 `isSuccess` 为准；完整字段见[命令参考](/docs/commands)。

## 5. 宿主里别再写编排（可选）

C# 宿主一行就能把整条更新链交给打包好的窗口：

```csharp
using AlyClient.CSharpSDK;

var ui = new AlyUpdateUIClient();   // 默认 ..\UpdateFolder\aly-client-ui.exe
ui.LaunchAuto();                    // 启动时静默检查，有更新才显示窗口
```

更新器由安装包与 GUI 提供，SDK 不再负责下载；其它语言见[多语言 SDK 总览](/docs/sdk/languages)。

## 下一步

- 每条客户端命令的参数与返回字段：[aly-client 命令参考](/docs/commands)
- 发布 GUI 的字段、快捷键与排障：[发布 GUI](/docs/publish/gui)
- 崩溃安全与状态机：[核心概念](/docs/concepts)
- 遇到问题：[FAQ](/docs/faq)
