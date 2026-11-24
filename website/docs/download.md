---
title: 下载与安装
description: 全部 Release 资产一览：AlyPublish 安装包、服务端二进制与各语言 SDK；以及安装后的第一步。
---

# 下载与安装

发布侧只要装一个 **AlyPublish** 包：发布 GUI、发布 CLI 与终端更新器都在里面。服务端是另一个单文件二进制，SDK 按语言各取一个 zip。

下表由**最新 Release 自动生成**，点击文件名即可下载。

<ReleaseDownloads group="all" />

## 装发布侧

Windows 用 <ReleaseLink asset="AlyPublish-Setup.exe" />（开始菜单入口，装到 `Program Files\AlyPublish`）或 <ReleaseLink asset="AlyPublish-win-x64.zip" />（解压即用）；Linux 用 <ReleaseLink asset="AlyPublish-linux-x64.zip" /> 或 <ReleaseLink asset="AlyPublish-linux-arm64.zip" />。

::: code-group

```powershell [Windows]
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-Setup.exe" -OutFile "AlyPublish-Setup.exe"
Start-Process ".\AlyPublish-Setup.exe"
```

```bash [Linux]
curl -fL -o AlyPublish-linux-x64.zip "https://github.com/lishuangquan1987/aly/releases/latest/download/AlyPublish-linux-x64.zip"
mkdir -p AlyPublish-linux-x64
unzip -q AlyPublish-linux-x64.zip -d AlyPublish-linux-x64   # zip 内没有顶层目录
cd AlyPublish-linux-x64
chmod +x install.sh                                 # zip 不保留可执行位
./install.sh                                        # 默认装到 /opt/aly/AlyPublish
# 无 root 时：./install.sh --prefix $HOME/.local/lib/AlyPublish
```

:::

装完启动 `AlyPublish`，接着走[发布第一个版本](/docs/start/publish)。包里每个目录装的是什么，见[发行包结构与分发](/docs/guide/package)。

::: tip 发布 GUI 的实现
默认发布 Go / MyGo 版：Windows 无额外运行依赖，Linux 需要 GTK3（`libgtk-3.so.0`）。历史 C# / Avalonia 版仍保留在仓库中，在 Release Build 工作流里把 `gui` 选为 `csharp`（或把仓库变量 `PUBLISH_GUI` 设为 `csharp`）可产出**同名**的 4 个资产，包内布局与配置格式完全一致。
:::

## 装服务端

服务端按 `aly-server-{goos}-{goarch}` 命名：Windows 用 <ReleaseLink asset="aly-server-windows-amd64.exe" />，Linux 用 <ReleaseLink asset="aly-server-linux-amd64" /> / <ReleaseLink asset="aly-server-linux-arm64" />，macOS 用 <ReleaseLink asset="aly-server-darwin-amd64" /> / <ReleaseLink asset="aly-server-darwin-arm64" />。

```bash
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server
./aly-server -p 2000
```

首次运行在程序旁创建 `aly.db` 与 `data/`。最快的部署路径见[部署服务端](/docs/start/server)；macOS 二进制未签名，首次运行可能被 Gatekeeper 拦截，在「系统设置 → 隐私与安全性」里允许即可。

## 取 SDK

上表「多语言 SDK」分组里每个 zip 都能直接下载：C# SDK 另有 `.nupkg`（未上架公共 NuGet），其余语言每个版本一个 zip（wheel / jar / `.crate` / 源码等）。

```bash
gh release download --pattern 'AlyClient.*SDK-*.zip'
```

接入方式见[C# SDK 集成](/docs/sdk)与[多语言 SDK](/docs/sdk/languages)。

## 校验与版本

- 发布资产没有单独的 checksum 文件；下载后可用 `Get-FileHash` / `sha256sum` 与 Release 页面核对。
- 版本号即 Git tag（如 `v1.2.3`）；服务端、发布工具与客户端不要求版本号一致，但客户端只认**严格更高**的服务端版本。
- 本站下载链接的生成时机与当前对应版本见[文档版本](/docs/versions)。

## 下一步

- **马上跑通**：[发布第一个版本](/docs/start/publish) · [部署服务端](/docs/start/server)
- **接入宿主**：[接入更新器](/docs/start/integrate)
- **包内结构**：[发行包结构与分发](/docs/guide/package)
