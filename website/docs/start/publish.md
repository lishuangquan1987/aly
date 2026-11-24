---
title: 发布第一个版本
description: 装一个 AlyPublish 包、起服务端、在 GUI 里初始化项目并发布 V1.0.1；四步，命令都能直接复制。
---

# 发布第一个版本

目标：**十分钟内让服务端出现你的第一个版本**。装包 → 起服务端 → 初始化项目 → 发布。

::: tip 前置
一台能跑服务端的机器（本机就行），一个要发布的产物目录（例如 `dist/`）。
:::

## 1. 装 AlyPublish

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
./install.sh
```

:::

一个包同时给你发布 GUI、发布 CLI 与终端更新器。Windows 也可以解压 `AlyPublish-win-x64.zip` 直接用；arm64 与 macOS 的资产名见[下载与安装](/docs/download)。

## 2. 起服务端

```bash
./aly-server -p 2000
```

Windows 下载 `aly-server-windows-amd64.exe`，改名 `aly-server.exe` 后运行。首次运行会在程序旁创建 `aly.db` 与 `data/`。

## 3. 在 GUI 里初始化项目

1. 启动 `AlyPublish`，点 **初始化项目**；
2. 填服务端地址（例如 `http://127.0.0.1:2000`）→ **获取列表** → 选中项目；服务端还没有项目就点 **新建服务端项目**；
3. 填本地产物目录（例如 `dist`）、显示名称、**主程序**（填了才会准备终端更新）；
4. 确认。

GUI 会写好 `.updator/`，并在产物目录**同级**建出 `UpdateFolder/`（更新器与 `client.json` 都在里面）。

## 4. 暂存并发布

1. 回主界面点 **刷新**；
2. **全部暂存**；
3. 版本号填 `V1.0.1`，写一句变更说明；
4. **发布**。

::: warning 首个版本号
服务端创建项目时已占用 `V1.0.0`，第一次发布必须 `V1.0.1` 或更高，否则会被防倒退校验拒绝。
:::

不想点界面时，同样的步骤用包内 CLI（`publish-cli/<rid>/aly-publish`）完成：

```bash
aly-publish config init --server http://127.0.0.1:2000 --project myapp
aly-publish project create --server http://127.0.0.1:2000 --name myapp --title "My App"
aly-publish status
aly-publish add --all
aly-publish push --version V1.0.1 --message "首个版本"
```

## ✅ 完成

服务端有了 `V1.0.1`，产物目录同级的 `UpdateFolder/` 也已就位——下一步就能让终端机器自己更新。

## 接下来

- [接入更新器](/docs/start/integrate)：让应用启动时自动检查更新
- [发布 GUI](/docs/publish/gui)：字段、快捷键与日志位置
- [打包与发布](/docs/publish)：日常发版流程与版本号规则
