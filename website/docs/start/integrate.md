---
title: 接入更新器
description: 三步让已有应用自动更新：确认 UpdateFolder、宿主启动时拉起更新器、本机跑一次完整更新。
---

# 接入更新器

目标：**让应用启动时检查并应用更新**。前提是已经发布过至少一个版本，没有就先看[发布第一个版本](/docs/start/publish)。

## 1. 确认 UpdateFolder/ 就位

用 GUI 初始化项目时已经建好，布局是：

```text
PackageFolder/
├── ApplicationFolder/                 # 应用本体（当前版本）
│   └── .updator/shared.json           # 服务端地址、项目名
└── UpdateFolder/
    ├── aly-client(.exe)               # 更新器
    ├── aly-client-ui(.exe)            # 可选更新窗口
    └── client.json                    # 主程序路径与要关闭的进程
```

没有 `UpdateFolder/` 时，从安装包 `client/<rid>/` 把两个文件复制进去，再写 `client.json`：

```json
{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}
```

`main_exe_relative_path` 是**相对 `UpdateFolder/`** 的路径。字段含义见[配置文件参考](/docs/client/config)。

## 2. 宿主启动时拉起更新器

::: code-group

```csharp [C# SDK]
using AlyClient.CSharpSDK;

var ui = new AlyUpdateUIClient();   // 默认 ..\UpdateFolder\aly-client-ui.exe
ui.LaunchAuto();                    // 静默检查，有更新才显示窗口
```

```bash [命令行 / 任意语言]
cd UpdateFolder
./aly-client check_update      # 逐行 JSON；只认 isSuccess
./aly-client download_update   # 只下变更文件
./aly-client apply_update      # 关进程 → 替换 → 重启主程序
```

:::

其它语言的做法就是拉起 `aly-client` 子进程并逐行读 stdout：跳过带 `data.status` 的进度行，最终结果看 `isSuccess`。业务失败也可能返回退出码 0，别用退出码判断。完整契约见[命令参考](/docs/commands#统一约定)。

## 3. 本机验证一次

上面三条命令跑完，应用应该被关闭、目录被替换、然后重新拉起为新版本。

## ✅ 完成

应用启动时会先检查更新；有新版本时，`apply_update` 关掉主程序、换上目录、再拉起。

## 接下来

- [宿主集成](/docs/client/)：生命周期、进度行与失败处理
- [客户端命令参考](/docs/commands)：每条命令的参数与返回字段
- [排障](/docs/troubleshooting)：更新没触发、替换失败怎么查
