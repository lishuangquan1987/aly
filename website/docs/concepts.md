# 核心概念

这一页解释 Aly 可靠性的四个基石：版本状态机、原子替换、崩溃恢复与差异下载。

## 版本状态机（version.json）

`UpdateFolder/version.json` 记录客户端当前处于更新流程的哪个阶段，所有命令都围绕它工作：

```text
applied ──(download_update)──→ downloaded ──(apply_update 启动)──→ applying
                  │                        │
                  │                        ├──(成功)──→ applied
                  │                        └──(崩溃)──→ 下次启动检测 → 自动恢复
                  │
                  └──(check_update)──→ has_update=true（服务端可达时）
```

### 状态守卫

写操作只在状态一致时发生，宁可拒绝也不写坏：

- `apply_update` 在 `version_status` 为空或未知时**拒绝替换**（空状态与 `applied` 语义相同：没有待应用的更新）；同时拒绝 `version_previous == version`（备份目录与版本目录会冲突）。
- `apply_update` 先校验已下载包的完整性：`ApplicationFolder_{version}` 缺失、为空、或文件数少于下载时记录的 `downloaded_file_count`，都拒绝替换并自愈到 `applied`，下一次 `check_update` 会重新下载——绝不会拿当前目录的拷贝或新旧混合的包"成功"交差。
- `rollback` 在应用未完成时拒绝执行，失败时恢复执行前的状态而不是一律写 `applied`。

### 离线语义

`check_update` 只在服务端可达时报告待应用的更新：`downloaded` 状态 + 服务端不可达时返回 `has_update=false`，避免宿主弹出与在线状态矛盾的"可以更新"提示。已下载的内容不丢弃——服务端恢复后 `apply_update` 照常执行。`applying` 是刻意保留的例外：崩溃恢复与回滚续作**始终报告**，哪怕离线——否则一台丢了应用目录的机器将永远无法自救。

## 原子替换（Atomic Replace）

`apply_update` 的核心是**纯目录改名**——不复制文件、不删除文件，GB 级目录也在秒级完成：

1. `download_update` 完成所有文件下载与校验后，把未变化的文件从当前 `ApplicationFolder` **物化**（materialize）到新版本目录——此时应用还在运行，只读拷贝不会锁住随后的目录改名。
2. 改名 `ApplicationFolder` → `ApplicationFolder_{previous_version}`（备份）。
3. 写入 `apply_stage=activating` 标记，然后改名 `ApplicationFolder_{new_version}` → `ApplicationFolder`。
4. 任一步改名失败，回滚 `version_status` 到 `downloaded` 以便重试，并清理临时字段。

得益于下载阶段的物化，应用真正"停机"的窗口只有三次 rename——从分钟级（全量拷贝方案）降到秒级。

## 崩溃恢复

如果启动时发现 `version_status` 为 `applying`，更新器会**重新执行原子改名步骤**——更新永远不会停留在半新半旧的状态。`apply_stage=activating` 标记在最终改名前写入，用于判定改名是否已发生（旧版本无此标记时退回旧的启发式判断并记日志警告）。

## 差异下载与校验

- `download_update` 通过 `check_diff` 的 MD5 + SHA256 双哈希比对，**只下载变更文件**，字节级进度输出。
- 单文件下载失败重试 3 次后不中断整体——失败的文件收集进 `failed_files` 列表，重试时只补下失败文件，已校验通过的文件直接跳过。
- 版本比较是 semver 感知的：`1.0.0-beta` 排在 `1.0.0` 之前（预发布 < 正式发布）；纯数字版本保持原有行为。

## 服务端降级保护

`publish_version` 拒绝发布不严格大于当前最新版本的版本号——防止误发旧版本号导致已更新的客户端"卡"在旧内容上。要回滚内容，发布一个更高的版本号即可。

## 更新锁

更新器通过 `.aly.lock` 保证同一时刻只有一个更新流程在运行：活着的持有进程会被识别（`IsProcessAlive` 采用进程存活探测，而非时间戳猜测），陈旧的残留锁才会被清理。GUI 前端、宿主程序、SDK 多入口并发触发更新时也是安全的。

## 清理（clean）

`clean` 命令修剪旧版本快照、临时文件（`.part`/`.tmp`/`.old*`）与过期日志，`--keep N` 控制保留的快照数，`--all` 全量清理。回滚依赖的最近快照不会被误删。
