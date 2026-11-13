---
title: FAQ
description: 平台、宿主语言、停机时间、断电恢复、回滚、认证与更新规则的常见问题。
---

# FAQ

## 为什么还要支持 Windows XP？

工控机、收银终端、医院设备仍在运行 XP，而这些机器最需要安全更新。主流更新框架（Squirrel、Velopack、Electron updater）通常要求 Windows 7+；Aly 通过锁定 Go 1.10.8 + 386 保留这条通道，并由 CI 校验 PE 头与 manifest。

和主流框架相比，Aly 的取舍是：以单个二进制 + 纯目录改名完成内网部署，服务端与客户端都不需要额外运行时；代价是信任模型更朴素——客户端只校验服务端清单提供的 MD5（下载阶段另有 SHA-256），没有签名链，所以服务端必须部署在可信内网。也正因为如此，「无鉴权」是有意的设计前提而不是缺陷，详见 [安全说明](https://github.com/lishuangquan1987/aly/blob/master/SECURITY.md)。

## 客户端宿主必须用 C# 吗？

不需要。`aly-client` 是独立可执行文件，所有命令输出统一 JSON。任何能拉起子进程并解析 JSON 的语言都能做宿主；仓库提供 C#/TypeScript/Go/Python/Java/Rust/C++/Pascal SDK 与 VB6/易语言 COM，SDK 只是省掉编排。见[多语言 SDK](/docs/sdk/languages)。

## 更新时应用会停机多久？

`apply_update` 只做三次目录改名，秒级。因为 `download_update` 已在应用运行时把未变化文件物化进新版本目录。

## 更新到一半断电会怎样？

下次启动检测到 `version_status=applying` 会重新执行改名；待应用目录会校验文件数/内容版本，不完整则拒绝并自愈，下一轮重新下载。见[核心概念](/docs/concepts)。

## 更新器自己怎么更新？

新版本更新器随版本包进入主程序目录；`check_self_update` 只检测，`self_update` 统一执行替换（取锁、SHA256、`.new` 中转、失败回滚、30 分钟窗口 3 次失败熔断）。见[自更新设计](/docs/self-update)。

## 回滚需要联网吗？

不需要。`rollback` 使用本地快照目录（`ApplicationFolder_V1.0.0/`），纯本地改名。快照保留数量由 `clean --keep N` 控制。

## 服务端有认证吗？

当前版本没有内置认证与签名，设计为受信内网部署。公网暴露请在前置网关做认证。见[服务端 API · 信任边界](/docs/api#信任边界-没有认证)。

## 支持增量更新吗？

支持。发布端只上传差异文件，客户端只下载与本地哈希不同的文件；大文件分片且断点续传；单文件失败不中断整体，重试只补失败文件。

## 为什么我第一次 push 被拒绝？

服务端创建项目时初始版本是 **V1.0.0**，`publish_version` 要求严格大于当前最新版本。首次推送请用 `V1.0.1` 或更高。之后每次发版同理，不能用旧版本号覆盖。

## `ignore_folders` 写 `logs/**` 为什么没生效？

`ignore_folders` 是**纯目录前缀匹配、区分大小写**，不是 glob；写 `logs` 才会忽略整个目录。`ignore_files` 才支持 glob。客户端的 `un_copy_*` 是另一套规则，只影响物化。详见[核心概念 · 配置规则](/docs/concepts#配置规则-ignore-与-un-copy)。

## 发布 GUI 支持英文吗？

支持。`AlyPublish` 基于 Irihi.Lingua + JSON 资源，工具栏可切换中文/English，也可用 `--lang zh-CN|en-US` 启动；选择保存在 `%LOCALAPPDATA%\AlyPublish\settings.json`。

## Linux GUI 需要装什么依赖？

只依赖桌面发行版预装的 GTK3。MyGo 是纯 Go 实现（无 cgo），产物是单个 ELF；无图形环境仍可运行 `-finish-self-update` 收尾模式。

## 如何更换服务端地址？

改 `.updator/shared.json` 的 `server_url` 后重新发布；配置随版本分发，客户端下一次更新会自动替换。也可在客户端用 `check_update --url` 临时覆盖。