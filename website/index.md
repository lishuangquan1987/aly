---
layout: home
description: 轻量级客户端自动更新系统：崩溃安全的原子替换、Windows XP 到 Linux、统一 JSON 协议、135 条端到端用例。

hero:
  name: Aly
  text: 轻量级客户端自动更新系统
  tagline: 一条命令发布新版本；终端机器上以纯目录改名完成升级——从 Windows XP 到 Linux，断电或崩溃也不会留下半新半旧的目录。
  actions:
    - theme: brand
      text: 5 分钟开始
      link: /docs/getting-started
    - theme: alt
      text: 模块参考
      link: /docs/
    - theme: alt
      text: 下载最新版本
      link: https://github.com/lishuangquan1987/aly/releases/latest
      target: _blank

features:
  - icon: 🛡️
    title: 崩溃安全的原子替换
    details: 应用更新 = 三次目录改名。任意时刻断电或崩溃都不会留下半新半旧的目录，下次启动按 version.json 状态机自动恢复或回滚。
  - icon: ⚡
    title: 秒级停机窗口
    details: 下载阶段就把未变化文件物化进新版本目录，apply 只做三次 rename；停机窗口从分钟级全量拷贝压到秒级。
  - icon: 🧩
    title: 一份 JSON 协议
    details: 更新器所有命令输出统一的 {isSuccess, errorMsg, data}。宿主可以是 C# SDK、可选 GUI，或任何能拉起子进程的程序。
  - icon: 🕰️
    title: Windows XP 原生兼容
    details: 更新器与 Windows GUI 固定用 Go 1.10.8 + 386 构建；CI 校验 PE 头与 ComCtl32 manifest，老工控机也能安全升级。
  - icon: 🌍
    title: 预构建覆盖 Windows / Linux
    details: 客户端提供 Windows 32 位与 Linux amd64/arm64；服务端与发布工具另有 macOS 构建。其他平台可从同一份源码编译。
  - icon: 🧪
    title: 135 条端到端用例
    details: Go 驱动的 E2E 框架覆盖正常流、崩溃注入、断点续传、回滚、锁与自更新；单条用例自带前置条件即可复现。
---