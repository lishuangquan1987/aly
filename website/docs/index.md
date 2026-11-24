---
title: 文档中心
description: Aly 文档目录——开始使用、指南、参考、原理、帮助。
---

# 文档中心

第一次用 Aly？直接看[快速开始](/docs/getting-started)，按你要做的事选一条路径。想知道 Aly 是什么、由哪些组件构成，看[项目简介](/intro)。

## 开始使用

- [快速开始](/docs/getting-started) — 三条路径分流：发布、接入、部署
- [发布第一个版本](/docs/start/publish) — 装包、起服务端、GUI 发布 `V1.0.1`
- [接入更新器](/docs/start/integrate) — 宿主拉起更新器，本机跑通一次完整更新
- [部署服务端](/docs/start/server) — 下载、启动、验证接口
- [下载与安装](/docs/download) — 全部资产、包内结构、校验与版本

## 指南

- [发行包结构与分发](/docs/guide/package) — `AlyPublish` 包里有什么、更新器怎么交付出去
- [打包与发布](/docs/publish) — 命令树、日常发版流程、版本号规则
- [发布 GUI](/docs/publish/gui) — 界面字段、快捷键、日志与排障
- [部署与运维](/docs/server/deploy) — systemd、反向代理、安全加固、备份与升级
- [宿主集成](/docs/client/) — 生命周期、JSON 契约、退出码与回滚
- [更新窗口](/docs/client/ui) — `aly-client-ui` 的启动模式、参数与退出码
- [C# SDK 集成](/docs/sdk) — 两种集成方式与构建期 targets

## 参考

- [aly-client 命令](/docs/commands) — 每条命令的参数、返回字段与失败行为
- [配置命令](/docs/publish/commands-config) · [项目命令](/docs/publish/commands-project) · [发布流程命令](/docs/publish/commands-workflow) — `aly-publish` 全量命令
- [配置文件参考](/docs/client/config) — `client.json`、`shared.json`、`version.json` 与暂存区
- [服务端 API](/docs/api) — 信封、错误语义与接口索引；[项目接口](/docs/server/api-project) · [文件接口](/docs/server/api-file) · [数据模型](/docs/server/api-models)
- [C# SDK API](/docs/sdk/api) — 类、方法、枚举与数据模型
- [多语言 SDK](/docs/sdk/languages) — 各语言资产与能力对照

## 原理

- [核心概念](/docs/concepts) — 版本状态机、原子替换、崩溃恢复、差异下载、更新锁
- [系统架构](/docs/architecture) — 一次完整更新的数据流与双轨工具链
- [自更新设计](/docs/self-update) — 更新器怎么更新自己
- [构建与跨平台](/docs/build) — 服务端、XP 轨更新器与 GUI 的构建方式
- [测试体系](/docs/testing) — E2E 框架、单元测试与 CI

## 帮助

- [排障](/docs/troubleshooting) — 按症状查：没触发、替换失败、锁占用
- [FAQ](/docs/faq) — 平台、宿主语言、停机时间、认证与规则
- [文档版本](/docs/versions) — 本站文档对应哪个代码版本
