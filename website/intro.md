---
title: 项目简介
description: Aly 是什么、由哪些组件构成、适合什么场景，以及平台覆盖边界。
---

# 项目简介

Aly 是一套**自包含的桌面应用自动更新系统**：发布方用发布工具把新版本推上服务端，终端机器上的更新器以崩溃安全的原子替换完成升级。服务端、发布工具、更新器、GUI 与 SDK 都在同一个仓库里，不依赖第三方更新框架。

## 解决什么问题

| 难题 | Aly 的做法 |
|---|---|
| 老机器更新不了 | 更新器与 Windows GUI 固定用 Go 1.10.8 + 386 构建，**Windows XP 可直接运行**；CI 校验 PE 头与 ComCtl32 manifest |
| 更新过程断电/崩溃变砖 | 纯目录改名 + `version.json` 状态机；启动时检测 `applying` 并恢复 |
| 大文件更新停机太久 | 下载阶段把未变化文件物化进新目录，apply 只做三次 rename，停机窗口秒级 |
| 更新器自身怎么更新 | `check_self_update` 检测、`self_update` 统一执行；GUI/SDK 委派，带失败熔断 |
| 宿主语言被绑死 | 所有命令输出统一 JSON；提供 C#/TypeScript/Go/Python/Java/Rust/C++/Pascal SDK 与 VB6/易语言 COM，任何能拉起子进程的语言都能接入 |

## 五个组件

| 组件 | 角色 | 技术栈 |
|---|---|---|
| **aly-server** | 项目元数据、文件存储、REST API | Go 1.25 · Gin · Ent · SQLite |
| **aly-publish** | 发布命令行：配置、差异、暂存、推送 | Go 1.25 · cobra |
| **aly-publish-gui** | 桌面发布器，包装 CLI | .NET 8 · Avalonia 12.1 · Semi/Ursa |
| **aly-client** | 终端更新器 | Go 1.10.8（XP 轨）/ 现代 Go（跨平台轨） |
| **aly-client-ui** | 可选更新窗口 | Windows：walk（XP）· Linux：MyGo + GTK3 |

宿主程序可以拉子进程解析 JSON，也可以集成 [多语言 SDK](/docs/sdk/languages)；SDK 只是编排层，不改变协议。

## 平台覆盖

| 组件 | 预构建 | 源码可编译 |
|---|---|---|
| 更新器 / 更新窗口 | Windows 32 位（XP+）、Linux amd64/arm64 | 同一份 Go 源码可编译 darwin/freebsd 等 |
| 服务端 | Windows amd64、Linux amd64/arm64、macOS amd64/arm64 | 其他 Go 目标 |
| 发布工具 | Windows amd64、Linux amd64/arm64、macOS amd64/arm64 | 其他 Go 目标 |
| 发布 GUI | Windows（.NET 8） | .NET 8 支持的桌面平台 |

## 下载最新 Release

需要试用时，直接从最新 Release 下载安装包或服务端二进制；链接在每次发版后自动刷新，完整说明见[下载与安装](/docs/download)。

<ReleaseDownloads group="all" compact :show-latest-hint="false" />

## 设计原则

- **单文件、零依赖**：更新器与服务端都是单个可执行文件；Linux GUI 仅依赖发行版预装的 GTK3。
- **协议先于实现**：统一 `{isSuccess, errorMsg, data}`；GUI、SDK 与更新器解耦。
- **配置随版本分发**：`.updator/shared.json` 随版本上传，服务端地址变更会随下一次更新生效。
- **一切可验证**：MD5 + SHA256 双校验；版本单调递增；[135 条端到端用例](/docs/testing)覆盖崩溃、断电、并发与自更新。

## 适用场景

- 企业内网桌面应用（工单、收银、工控上位机）：自托管服务端，无需公网。
- 包含大量 DLL/资源的 .NET / C++ 应用：目录级差异比对，只传变更文件。
- 必须支持 Windows XP / 7 的遗留系统：这是多数现代更新框架做不到的。

::: warning 安全边界
服务端当前没有内置认证与签名，设计为部署在受信内网。公网暴露请在前置网关做认证与访问控制。
:::