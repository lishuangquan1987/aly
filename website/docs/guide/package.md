---
title: 发行包结构与分发
description: AlyPublish 安装包里有什么、publish-cli 与 client 的 rid 映射、把更新器交付到别的平台，以及 zip 不保留可执行位这类坑。
---

# 发行包结构与分发

一个 `AlyPublish` 包同时携带发布 GUI、发布 CLI 与终端更新器。这一页讲包里到底有什么、怎么把它们发出去；想马上跑通看[发布第一个版本](/docs/start/publish)。

## 包里有什么

```text
AlyPublish(.exe)                        # 发布 GUI 本体
payload.json                            # 平台与 payload 索引：GUI 据此决定用哪个构建
publish-cli/<rid>/aly-publish(.exe)     # 发布 CLI
client/<rid>/aly-client(-ui)(.exe)      # 终端更新器与图形前端
install.sh  uninstall.sh                # 仅 Linux 包
```

`<rid>` 是运行标识：

| 目录 | 取值 |
|---|---|
| `publish-cli/<rid>/` | `windows-amd64`、`linux-amd64`、`linux-arm64` |
| `client/<rid>/` | `windows-x86`、`linux-amd64`、`linux-arm64` |

- GUI 运行时读 `payload.json` 的 `cli_rid` / `client_rid` 取本平台的 CLI 与更新器，**不需要手工挑**；
- 初始化项目、或在「项目设置」里补上主程序时，GUI 会把 `client/<rid>/` 里那两个文件复制进项目同级的 `UpdateFolder/`，并写好 `client.json`。

## 把更新器交付到别的平台

要在 Linux 上准备 Windows 应用的 `UpdateFolder`，直接从包里取 `client/windows-x86/` 的两个文件即可。预构建的更新器只有两种平台：

| 平台 | 预构建 | 说明 |
|---|---|---|
| Windows | `client/windows-x86/` | 32 位构建，Windows XP 到 Windows 11 通用（64 位系统直接跑） |
| Linux | `client/linux-amd64`、`client/linux-arm64` | 按架构选；Linux GUI 运行需要发行版预装的 GTK3 |
| macOS / FreeBSD | 无 | 更新器可从同一份源码编译，没有官方预构建 |

::: warning zip 不保留可执行位
从 zip 里单独取出的 `aly-client`、`aly-server`、`install.sh` 都要 `chmod 0755` 才能执行。
:::

## 服务端与 SDK 资产

服务端二进制、发布 GUI 安装包与各语言 SDK 都在 Release 里，完整清单见[下载与安装](/docs/download)。

SDK 不负责下载更新器：`UpdateFolder/` 由上面的包生成，或按本文手工放置；SDK 的构建期 targets 只创建目录并在缺失时写默认 `client.json`。
