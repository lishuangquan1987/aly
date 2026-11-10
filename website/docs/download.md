---
title: 下载与安装
description: Aly 各组件的最新发布资产、平台矩阵与安装命令。
---

# 下载与安装

所有发布资产都在 GitHub Releases。下面的链接始终指向最新版本；把 `<asset>` 换成表格中的文件名即可。

```text
https://github.com/lishuangquan1987/aly/releases/latest/download/<asset>
```

## 平台矩阵

| 组件 | Windows | Linux | macOS | 说明 |
|---|---|---|---|---|
| **aly-server** | `aly-server-windows-amd64.exe` | `aly-server-linux-amd64` · `aly-server-linux-arm64` | `aly-server-darwin-amd64` · `aly-server-darwin-arm64` | 单文件，SQLite 内置 |
| **aly-publish** | `aly-publish-windows-amd64.exe` | `aly-publish-linux-amd64` · `aly-publish-linux-arm64` | `aly-publish-darwin-amd64` · `aly-publish-darwin-arm64` | 发布 CLI |
| **aly-client** | `aly-client.exe`（32 位，XP+） | `aly-client-linux-amd64` · `aly-client-linux-arm64` | — | 终端更新器 |
| **aly-client-ui** | `aly-client-ui.exe`（32 位，XP+） | `aly-client-ui-linux-amd64` · `aly-client-ui-linux-arm64` | — | 可选 GUI |
| **C# SDK** | 源码/本地 nupkg | 源码/本地 nupkg | 源码/本地 nupkg | 未上架公共 NuGet |

::: tip 平台边界
客户端预构建只有 Windows 与 Linux。macOS / FreeBSD 的更新器可以从同一份源码编译，但没有官方预构建；服务端与发布工具提供 macOS 预构建。
:::

## 下载命令

::: code-group

```powershell [Windows]
# 服务端
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-windows-amd64.exe" -OutFile "aly-server.exe"

# 发布工具
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-publish-windows-amd64.exe" -OutFile "aly-publish.exe"

# 更新器 + 可选 GUI
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client.exe" -OutFile "aly-client.exe"
Invoke-WebRequest -Uri "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client-ui.exe" -OutFile "aly-client-ui.exe"
```

```bash [Linux]
# 服务端（amd64；arm64 改文件名即可）
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-linux-amd64"
chmod +x aly-server

# 发布工具
curl -fL -o aly-publish "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-publish-linux-amd64"
chmod +x aly-publish

# 更新器 + 可选 GUI
curl -fL -o aly-client "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client-linux-amd64"
curl -fL -o aly-client-ui "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-client-ui-linux-amd64"
chmod +x aly-client aly-client-ui
```

```bash [macOS]
# 服务端（Apple Silicon 用 darwin-arm64）
curl -fL -o aly-server "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-server-darwin-amd64"
chmod +x aly-server

# 发布工具
curl -fL -o aly-publish "https://github.com/lishuangquan1987/aly/releases/latest/download/aly-publish-darwin-amd64"
chmod +x aly-publish
```

:::

macOS 二进制未签名，首次运行可能被 Gatekeeper 拦截；在“系统设置 → 隐私与安全性”中允许即可。

## 下一步

- **部署服务端**：[服务端部署](/docs/server/deploy)
- **发布第一个版本**：[aly-publish 发布工具](/docs/publish)
- **终端机器布局**：[更新器概览](/docs/client/) 与 [配置参考](/docs/client/config)
- **接入 C# 宿主**：[C# SDK 集成](/docs/sdk)

## 校验与版本

- 发布资产没有单独的 checksum 文件；下载后可用 `Get-FileHash` / `sha256sum` 与 Release 页面核对（若已提供）。
- 版本号即 Git tag（如 `v1.2.3`）；服务端、发布工具与客户端不要求版本号一致，但客户端只认“严格更高”的服务端版本。
- 当前 C# SDK 未上架公共 NuGet：从源码引用 `AlyClient.CSharpSDK.csproj`，或使用 CI 产出的 nupkg。本地 `dotnet pack` 不会自动注入更新器版本，详见 [SDK 集成](/docs/sdk)。