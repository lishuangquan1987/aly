---
title: 排障
description: 按症状查：发布被拒绝、找不到更新器、更新没触发、下载卡住、替换失败、锁被占用、服务端起不来。每条给出看哪个日志、怎么修。
---

# 排障

先回答两个问题：**问题发生在哪一侧**（发布机 / 终端 / 服务端），**有没有日志**。三侧的日志位置都在本文里。

## 日志在哪

| 位置 | 文件 | 记录什么 |
|---|---|---|
| `UpdateFolder/` | `update.log` | 检查、应用、回滚、自更新的执行过程 |
| `UpdateFolder/` | `download.log` | 下载、未变化文件物化、删除同步 |
| `UpdateFolder/` | `check_diff.log` | 本地 MD5 扫描告警 |
| 发布 GUI | `logs/log.log` | GUI 日志；状态栏「日志」按钮直接打开目录 |

更新器三个日志都按 5 MB 单代轮转（`*.log.1`）。GUI 日志目录按顺序取第一个可写的：`AlyPublish.exe` 同级的 `logs/` → `%LOCALAPPDATA%\AlyPublish\logs\` → 临时目录下的 `AlyPublish\logs\`。

## 发布侧

| 症状 | 怎么查 | 怎么修 |
|---|---|---|
| 顶部提示找不到 `aly-publish` | 包里 `publish-cli/<rid>/` 是否存在、`payload.json` 的 `cli_rid` 是否匹配 | 重新安装 AlyPublish 后点「重新检测」 |
| 初始化后 `UpdateFolder/` 里没有更新器 | 包里 `client/<rid>/` 是否缺失；主程序与项目目录是否跨盘 | 重新安装；跨盘时 `main_exe_relative_path` 算不出来，会直接失败 |
| 发布被拒绝：版本不大于最新版本 | 当前最新版本号 | 用更高版本号；首次发布至少 `V1.0.1`（见[版本号规则](/docs/publish#版本号规则)） |
| 发布失败：文件已修改 | 暂存区记录的 MD5 与磁盘不一致 | 刷新后重新暂存再发 |
| 服务端项目列表为空 | 服务端地址是否正确、服务端是否在跑 | 先在 GUI 里「新建服务端项目」，或检查地址 |
| Linux 启动报找不到 GTK 库 | `libgtk-3.so.0` | `apt install libgtk-3-0`（或对应发行版的包） |
| Linux 上从包里取的 CLI / 更新器不能执行 | zip 不保留可执行位 | `chmod 0755` |
| 目录已初始化，无法继续初始化 | — | 改用「添加本地项目」 |

## 终端更新侧

| 症状 | 怎么查 | 怎么修 |
|---|---|---|
| `check_update` 说没有更新，但服务端有新版本 | 本地 `version.json` 的版本、`shared.json` 的 `project_name` 与 `server_url` | 对齐项目名与服务端地址；客户端只接受**严格更高**的版本 |
| 服务端连不上时也报「没有更新」 | 这是 `downloaded` 状态下的**有意行为** | 联网后再查；已下载的内容不会丢 |
| 下载中断 | `download.log` 的 `failed_files`、`.part` 文件 | 重新 `download_update` 只补失败文件，支持 Range 续传 |
| `apply_update` 反复失败 | `update.log`；`version.json` 的 `apply_fail_count` | 修正占用进程后重试；熔断后 `check_update` 不再催促 apply |
| 替换失败，提示目录被占用 | `must_close_process_name` 是否漏了进程 | 补进 `client.json`；更新器会探测并强杀占用者 |
| 提示锁被占用 | `UpdateFolder/.aly.lock` 的持有者 | 确认没有更新进程在跑；陈旧锁 30 分钟 TTL、5 分钟心跳，会自动清理 |
| 手工改过 `version.json` 后行为异常 | 状态与磁盘内容是否错配 | 别手改；用 `download_update` 重写状态 |

## 服务端

| 症状 | 怎么查 | 怎么修 |
|---|---|---|
| 启动即退出，日志 `failed opening connection to sqlite` | `-db` 目录是否可写 | 检查目录属主与权限 |
| 启动即退出，日志 `failed creating schema resources` | 库文件损坏或被占用 | 停掉重复实例；用备份恢复 |
| 上传返回「文件过大」 | 单文件是否超 1 GB、代理 `client_max_body_size` | 调整代理限制，例如 `client_max_body_size 1050m` |
| 代理返回 413 | Nginx 默认 1 MB | 同上 |
| 下载 404 | `path` 不是 `data/` 下的绝对路径，或二进制搬过位置 | 用 `get_all_files` 返回的 `fileAbsolutePath` 原样请求 |
| 下载到一半中断 | 单次下载按文件大小设写期限（`5 分钟 + 每 MiB 1 秒`，上限 30 分钟），全局 `WriteTimeout` 为 0 | 慢链路超大文件仍可能超期断开；客户端按 Range 续传重试 |
| `database is locked` | 同一库文件被多进程打开 | 保证只有一个 `aly-server` 实例 |
| 变更日志时间与本地不一致 | `time` 由发布端传入并原样存储 | 发布端统一 `yyyy-MM-dd HH:mm:ss` 与同一时区 |

服务端没有内置版本接口，核对运行版本看 `GET /api/server/info` 返回的 `version`（Go 运行时版本）与二进制文件名。

## 相关页面

- [FAQ](/docs/faq)：平台、停机时间、断电恢复、认证
- [宿主集成](/docs/client/)：生命周期、JSON 契约、退出码
- [核心概念](/docs/concepts)：状态机与离线语义
- [部署与运维](/docs/server/deploy)：systemd、反向代理与备份
