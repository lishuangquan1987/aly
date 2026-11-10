---
title: 快速开始
description: 选择操作系统与接入方式，用最短路径跑通服务端、发布和客户端更新。
---

# 快速开始

::: tip 先确认服务端
“发布版本”和“接入更新器”路径都假设服务端已启动。还没有服务端时，先选“部署服务端”，或在下方手动流程第 1 步启动它。
:::

<QuickStartWizard />

## 完整流程（手动版）

### 1. 启动服务端

```bash
./aly-server -p 2000
```

首次运行会在程序旁创建 `aly.db` 与 `data/`。Linux 上用 `./aly-server-linux-amd64`，Windows 上用 `.\aly-server.exe`；`-db` 可改数据库文件路径。

### 2. 在服务端创建项目

新项目初始版本是 **V1.0.0**，所以第一次 `push` 必须使用更高版本（例如 V1.0.1）。

```bash
# 在产物目录（如 dist）初始化客户端配置
./aly-publish config init --server http://127.0.0.1:2000 --project myapp

# 在服务端注册同名项目（只需一次）
./aly-publish project create --server http://127.0.0.1:2000 --name myapp --title "My App"
```

### 3. 发布第一个版本

```bash
./aly-publish status
./aly-publish add --all
./aly-publish push --version V1.0.1 --message "首个版本"
```

`status` 展示新增/修改/删除；`add --all` 把新增与修改放入暂存区；`push` 上传变更文件并登记版本日志。大文件自动走分片上传。

### 4. 终端机器上的目录布局

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

### 5. 客户端检查并应用

```bash
# 第 1 步：检查
aly-client check_update
# → {"isSuccess":true,"data":{"has_update":true,"need_download_update":true,
#    "new_version":"1.0.2","force_update":false,"release_notes":["..."]}}

# 第 2 步：下载变更文件（MD5 + SHA256 校验）
aly-client download_update

# 第 3 步：应用（关闭进程 → 原子替换 → 重启主程序）
aly-client apply_update
```

业务失败通常仍返回退出码 0，请以 JSON 的 `isSuccess` 为准；完整字段见 [命令参考](/docs/commands)。

### 6. C# 宿主：省掉手工编排

```csharp
using AlyClient.CSharpSDK;

// 需要 UpdateFolder/aly-client-ui.exe 与 aly-client.exe
var ui = new AlyUpdateUIClient();
ui.LaunchAuto();   // 启动时静默检查，有更新才显示窗口
```

当前 SDK 未上架公共 NuGet，使用项目引用或 CI 产出的 nupkg；详见 [C# SDK 集成](/docs/sdk)。

## 下一步

- 想查每条命令的参数与返回字段：[aly-client 命令参考](/docs/commands)
- 想理解崩溃安全与状态机：[核心概念](/docs/concepts)
- 想接 CI 或写发布脚本：[aly-publish 发布工具](/docs/publish)
- 遇到问题：[FAQ](/docs/faq)