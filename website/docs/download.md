---
title: 下载与安装
description: 装一个 AlyPublish 安装包就拿到发布 GUI、发布 CLI 与终端更新器；本文给出 Windows 安装包、Linux 压缩包、包内 payload 结构与服务端下载。
---

# 下载与安装

Aly 的客户端与发布 CLI 都打进 **AlyPublish 安装包**：装一个包，发布 GUI、`aly-publish`、`aly-client`、`aly-client-ui` 全部就绪。服务端单独下载。

所有资产都在 GitHub Releases，下面链接始终指向最新版本：

```text
https://github.com/lishuangquan1987/aly/releases/latest/download/<asset>
```

## 第一步：装 AlyPublish（一个包搞定）

| 平台 | 资产 | 说明 |
|---|---|---|
| Windows x64 | `AlyPublish-Setup.exe` | 安装包：开始菜单入口，自包含 .NET 8，装到 `Program Files\AlyPublish` |
| Windows x64 | `AlyPublish-win-x64.zip` | 便携包：解压即用，绿色无安装 |
| Linux x64 | `AlyPublish-linux-x64.zip` | Linux 安装包（压缩包）：解压后运行 `install.sh` |
| Linux arm64 | `AlyPublish-linux-arm64.zip` | 同上，arm64 |

```powershell [Windows]
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-Setup.exe" -OutFile "AlyPublish-Setup.exe"
Start-Process ".\AlyPublish-Setup.exe"
```

```bash [Linux]
curl -fL -o AlyPublish-linux-x64.zip "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-linux-x64.zip"
unzip AlyPublish-linux-x64.zip
cd AlyPublish-linux-x64
chmod +x install.sh                                 # zip 不保留可执行位
./install.sh                                        # 默认装到 /opt/aly/AlyPublish（需要写权限）
./install.sh --prefix $HOME/.local/lib/AlyPublish    # 无 root 时用这种
```

装好后启动 `AlyPublish`（Linux 装在前缀目录里的 `AlyPublish`，`install.sh` 也会在 `$HOME/.local/bin` 写一个启动器），接着走[快速开始](/docs/getting-started)。

## 第二步：装服务端

| 平台 | 资产 |
|---|---|
| Windows | `aly-server-windows-amd64.exe` |
| Linux | `aly-server-linux-amd64` · `aly-server-linux-arm64` |
| macOS | `aly-server-darwin-amd64` · `aly-server-darwin-arm64` |

```bash [Linux]
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server
./aly-server -p 2000
```

首次运行在程序旁创建 `aly.db` 与 `data/`。macOS 二进制未签名，首次运行可能被 Gatekeeper 拦截，在“系统设置 → 隐私与安全性”里允许即可。

## 包内 payload：GUI、CLI、更新器都在这里

安装后（或解压便携包后）的目录：

```text
AlyPublish(.exe)                        # 发布 GUI 本体
payload.json                            # 平台与 payload 索引：GUI 据此决定用哪个构建
publish-cli/<rid>/aly-publish(.exe)     # 发布 CLI：windows-amd64 | linux-amd64 | linux-arm64
client/<rid>/aly-client(-ui)(.exe)      # 终端更新器与图形前端：windows-x86 | linux-amd64 | linux-arm64
install.sh  uninstall.sh                # 仅 Linux 包
```

- GUI 运行时读 `payload.json` 的 `cli_rid` / `client_rid` 自动取本平台的 CLI 与更新器，**不需要你手工挑**；
- 初始化项目、或在“项目设置”里补上主程序时，GUI 会把 `client/<rid>/` 里那两个更新器复制进项目同级的 `UpdateFolder/`，并写好 `client.json`；
- 要在别的平台交付更新器（例如在 Linux 上准备 Windows 应用的 `UpdateFolder`），直接从包内取 `client/windows-x86/` 的两个文件即可；
- 从 zip 里单独取文件后**必须** `chmod 0755`：zip 不保留可执行位。

::: tip 更新器的平台边界
预构建的更新器只有 Windows（32 位，XP 到 Win11 通用）与 Linux（amd64 / arm64）。macOS / FreeBSD 的更新器可以从同一份源码编译，没有官方预构建；服务端与发布工具提供 macOS 预构建。
::

## 语言 SDK

| 组件 | 资产 | 说明 |
|---|---|---|
| C# SDK | `AlyClient.CSharpSDK.*.nupkg` · `AlyClient.CSharpSDK-<version>.zip` | 未上架公共 NuGet，从 Release 或源码获取 |
| 其它语言 SDK | `AlyClient.*SDK-<version>.zip` | 每个语言一个版本化 zip（wheel / jar / `.crate` / 源码等） |

```bash
gh release download --pattern 'AlyClient.*SDK-*.zip'
```

SDK 不再负责更新器二进制的下载与复制：`UpdateFolder/` 由 AlyPublish GUI 生成，或按上面的 payload 布局手工放置。前置条件与宿主接入见[C# SDK 集成](/docs/sdk)与[多语言 SDK](/docs/sdk/languages)。

## 校验与版本

- 发布资产没有单独的 checksum 文件；下载后可用 `Get-FileHash` / `sha256sum` 与 Release 页面核对。
- 版本号即 Git tag（如 `v1.2.3`）；服务端、发布工具与客户端不要求版本号一致，但客户端只认“严格更高”的服务端版本。

## 下一步

- **三步跑通**：[快速开始](/docs/getting-started)
- **部署服务端**：[服务端部署](/docs/server/deploy)
- **用 GUI 发布**：[发布 GUI](/docs/publish/gui)
- **接入宿主**：[C# SDK 集成](/docs/sdk)
