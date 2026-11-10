---
layout: home

hero:
  name: Aly
  text: 轻量级客户端自动更新系统
  tagline: 一条命令发布新版本，用户端崩溃安全的原子替换完成升级 —— 从 Windows XP 一路覆盖到 Linux / macOS / FreeBSD
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 项目简介
      link: /guide/overview
    - theme: alt
      text: GitHub
      link: https://github.com/lishuangquan1987/aly

features:
  - icon: 🛡️
    title: 崩溃安全的原子替换
    details: 应用更新 = 纯目录改名，任意时刻断电或崩溃都不会留下半新半旧的目录；重启后按 version.json 状态机自动恢复或回滚。
  - icon: ⚡
    title: 秒级停机窗口
    details: 下载阶段就把未变化文件物化进新版本目录，apply 只需三次 rename —— 应用停机窗口从"分钟级全量拷贝"压缩到"秒级"。
  - icon: 🧩
    title: 一套协议，多种宿主
    details: 更新器全部命令输出统一 JSON 协议，宿主可以是 C# 客户端（NuGet SDK）、可选 GUI，或任何能拉起子进程的程序。
  - icon: 🕰️
    title: Windows XP 兼容
    details: 更新器与 Windows GUI 固定用 Go 1.10.8 + 386 构建，CI 自动做 PE 头与 manifest 校验 —— 老旧工控机、收银机也能被安全地更新。
  - icon: 🌍
    title: 真正的跨平台
    details: 同一套更新逻辑编译到 windows 386/amd64、linux amd64/arm64、darwin amd64/arm64、freebsd amd64；Linux GUI 走纯 Go 原生渲染。
  - icon: 🧪
    title: 135 条端到端用例
    details: Go 驱动的 E2E 测试框架覆盖正常流、崩溃注入、断点续传、回滚、自更新等全部场景，单条用例自带前置条件即可独立复现。
---
