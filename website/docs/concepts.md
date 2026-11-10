---
title: 核心概念
description: 版本状态机、原子替换、崩溃恢复、差异下载、更新锁与配置规则的精确语义。
---

# 核心概念

理解这五个机制，就能解释 Aly 在断电、断网、重复触发和回滚时的行为。

## 版本状态机（version.json）

`UpdateFolder/version.json` 记录客户端处于更新流程的哪个阶段：

```text
（空/applied） ──download_update──→ downloaded ──apply_update──→ applying ──成功──→ applied
       │                                  │                         │
       └── check_update 发现新版本 ───────┘                         └── 崩溃 → 下次启动恢复
```

| 字段 | 含义 |
|---|---|
| `version` | 当前/待应用版本 |
| `version_previous` | 主目录当前真实内容版本（回滚与恢复用） |
| `version_status` | 空、`applied`、`downloaded`、`applying` |
| `apply_stage` | `applying` 期间写 `activating`，标记备份改名已完成 |
| `downloaded_file_count` | 下载产物的常规文件数，apply 用它识别残缺目录 |
| `rollback_previous` | 回滚开始时主目录的真实内容版本 |
| `rollback_target` | 本次回滚目标版本（中断续跑用） |
| `apply_fail_count` | 同一待应用版本的连续 apply 失败次数（熔断） |
| `after_apply_update_script` | 更新成功后执行的脚本（相对应用目录） |

### 状态守卫

- `apply_update` 在状态为空/未知时拒绝替换；拒绝 `version_previous == version`。
- apply 前校验待应用目录：缺失、为空或文件数少于 `downloaded_file_count` 时拒绝并自愈为 `applied`，下一轮重新下载。
- `rollback` 在应用未完成时拒绝；失败时恢复执行前状态，而不是一律写 `applied`。
- `apply_fail_count` 达到阈值后 `check_update` 不再上报“继续 apply”，避免反复杀进程/拉起。

### 离线语义

`check_update` 在 `downloaded` 状态且服务端不可达时返回 `has_update=false`——无法核对就不提示，已下载内容保留，联网后继续 apply。`applying`（崩溃恢复）与回滚续跑是例外：断网也必须上报，否则机器无法自愈。

## 原子替换

`apply_update` 的核心是**纯目录改名**，不复制、不删除：

1. `download_update` 把未变化文件从当前 `ApplicationFolder` **物化**到新版本目录（应用仍在运行）。
2. 改名 `ApplicationFolder` → `ApplicationFolder_{previous_version}`。
3. 写 `apply_stage=activating`，改名 `ApplicationFolder_{new_version}` → `ApplicationFolder`。
4. 失败则回滚状态到 `downloaded` 并清理临时字段。

停机窗口只有三次 rename；GB 级目录也是秒级。

## 崩溃恢复

启动时若 `version_status=applying`，更新器重新执行原子改名。`apply_stage=activating` 用于判断“改名是否真的发生过”；旧版本没有该字段时退回启发式判断并记录警告。

## 差异下载与校验

- 发布端只上传与上一版本不同的文件；客户端只下载与本地 MD5/SHA256 不同的文件。
- 单文件失败重试 3 次后不中断整体，失败清单进 `failed_files`；重试只补失败文件。
- 大文件使用 `.part` 断点续传；服务端 `download_file` 支持 Range。
- 版本比较是 semver 感知的：`1.0.0-beta < 1.0.0`，构建元数据忽略。

## 更新锁

`.aly.lock` 保证同一时刻只有一个更新流程。持有进程存活时锁有效；只有陈旧残留锁会被清理。宿主、GUI、SDK 多入口并发触发时是安全的。

## 配置规则：ignore 与 un_copy

两套规则作用在不同阶段，不能混用：

| 配置 | 生效位置 | 语义 |
|---|---|---|
| `ignore_folders` / `ignore_files` | **服务端扫描/发布清单** | 决定哪些文件不下发。文件夹是**纯前缀匹配、区分大小写**（`logs` 生效，`logs/**` 不会按 glob 解释）；文件支持精确、glob、文件名与 `*.ext` 匹配 |
| `un_copy_folders` / `un_copy_files` | **客户端物化** | 默认全部迁移，列出的路径**不复制**进新版本目录；支持 `*`/`**`，精确名不区分大小写 |

::: warning 常见误解
`ignore_*` 不会在客户端 apply 时再过滤一遍；`un_copy_*` 也不会影响发布端上传。写错规则不会报错，只会让更新包变大或让运行时文件不被迁移。
:::

## 清理（clean）

`clean` 修剪旧版本快照、`.part`/`.tmp`/`.old*` 与过期日志。`--keep N` 保留最近 N 个快照（`N < 1` 时按默认 3）；`--all` 只保留当前版本保护集。回滚依赖的最近快照不会被误删。