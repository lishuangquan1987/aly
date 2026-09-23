# aly 客户端「更新中断 → 恢复」分析报告

> 对象：`lishuangquan1987/aly` @ master（commit `8aed31d`，2026-09-22）
> 范围：`client/aly-client`（Go 更新器）的 download / apply / rollback 全流程
> 方法：逐行核对状态机代码 + 编写状态机模拟器枚举**每一个中断点**（46+48 个场景）并跑多轮恢复
> 模拟器：`tools/interrupt-sim/`（`sim.py` 忠实复刻状态机，`scenarios3.py` 为多轮恢复判定，`results/` 为实测输出）

---

## 结论先行

**会有 bug，但不在你以为的地方。**

| 维度 | 结论 |
|------|------|
| 下载中断 | ✅ 安全。`.part` 原子写 + MD5/SHA256 双校验，`downloaded` 状态只在全部校验通过后落盘 |
| 应用中断（apply 各阶段） | ✅ 安全。9 个中断点全部 1 轮自愈，与仓库 E2E 的 123/123 结论一致 |
| **回滚中断** | ❌ **有 bug**。回滚意图会被静默取消，甚至被**逆转为升级** |
| **中断 + 期间服务器发新版** | ❌ **有 bug**。回滚目录出现"名字与内容错配"，回滚会拿到错版本 |
| **中断 + 宿主无条件 download** | ❌ **死局**。应用目录永久缺失，无任何自愈路径 |
| 断电导致 version.json 损坏 | ❌ **有 bug**。全命令瘫痪且无自愈，需人工介入 |

一句话：**"崩溃安全"这层做得很扎实（原子写、状态机、崩溃恢复分支都到位），但状态机只有 `applying` 一个"进行中"状态，无法区分"更新中"和"回滚中"，也没有"服务器版本漂移"的处理——所有真问题都出在这两处。**

---

## 一、先说做对的地方（避免重复劳动）

核对后确认这些设计是可靠的，不需要改：

1. `WriteVersion` 用 `tmp + rename` 原子替换（`config/version.go:72-81`），不会留下半截 `version.json`。
2. 下载写 `.part` 再 rename（`client/http_client.go:294-341`），且**下载后必做 MD5+SHA256 校验**，校验失败删除重下 —— 所以"半截文件被激活"这条路堵死了。
3. `CopyFile` 也是 `dst.tmp + rename`（`util/file.go:66-99`），复制中断不留半截目标文件。
4. `applyReplacement` 的复制用 `overwrite=false`（`util/file.go:49-53`），**已下载的新版本文件不会被旧版本覆盖**，重做是幂等的。
5. 三次 rename（旁移 / 备份 / 激活）都有统一的错误码分类 + 占用者查杀 + explorer 兜底（`cmd/common.go:356-456`）。
6. `#7` 兜底（主目录丢失时保持 `applying` 而非降级）方向正确。
7. 锁的陈旧回收已考虑到"进程被杀但句柄未回收"（`util/process.go:430-453` 用 `GetExitCodeProcess` 判定）。

我用模拟器验证了 apply 的 **9 个中断点**（复制前 / 复制中 / 旁移后 / 备份改名后 / 应用改名后 / 写 applied 前 …）在 SDK 默认调用链（`check → apply`）下**全部 1 轮自愈**——这部分与仓库 E2E 一致，可以放心。

---

## 二、确认存在的 bug

### Bug #1（P1，最该修）回滚中断后，宿主会把"回滚"变成"升级"

**代码位置**：`cmd/apply_update.go:78-137`（崩溃恢复分支）、`cmd/rollback.go:125-132`

**根因**：`rollback` 和 `apply_update` **共用 `applying` 状态**。`rollback` 额外写了一个 `RollbackPrevious` 字段来标记"这是回滚"，但 `apply_update` 的崩溃恢复分支**从头到尾没读过这个字段**，只用 `versionInfo.Version` 去找版本目录。

**复现链**（模拟器 B2 场景，7/7 中断点全部命中）：

```
当前：App 装 V2，version.json = {Version: V3, VersionPrevious: V2, status: downloaded}
      （V3 已下载待应用；磁盘还有 App_V1 历史快照）
用户：点了「回滚到 V1」
  → rollback 写 status=applying, RollbackPrevious=V2
  → rename App(V2内容) → App_V2 ✅
  → 准备 rename App_V1 → App 时，进程被杀 / 断电   ❌中断

重启后宿主（SDK MainLoop）按常规流程：
  → check_update：status=applying → 返回 NeedDownloadUpdate=false
  → apply_update：走崩溃恢复 → mainFolder 不存在
                 → 取 versionInfo.Version = V3  ← 关键：用的是"待应用的新版本"
                 → rename App_V3 → App
                 → 写 status=applied
```

**结果**：用户点了"回滚到 V1"，程序重启后**自动升级到了 V3**。version.json 与磁盘内容倒是一致的，所以不会报错，但**用户意图被静默逆转**。

同一根因的另一个分支（`B2'`，crash 在 `rb:remove_aside_done`）：回滚其实已经完成，只是没来得及写 `applied`。此时重跑 `rollback --version V1` 会报 `version 1.0.0 not found`（目标目录已被消耗），卡在 `applying`，随后被 apply 接管推向 V3。

**修复方向**（最小改动）：`ApplyUpdate` 的 `applying` 分支开头加一道判断：

```go
if versionInfo.RollbackPrevious != "" {
    // 这是回滚中断，不是更新中断：按回滚语义恢复（目标目录 = RollbackPrevious 对应快照）
    // 或直接委托 Rollback 的崩溃恢复分支，不要碰 versionInfo.Version
}
```

更彻底的做法是引入 `applying_rollback` 独立状态，让两个流程各走各的恢复路径。

---

### Bug #2（P1）中断期间服务器发新版 → 回滚目录"名字与内容错配"

**代码位置**：`cmd/download_update.go:196`（`versionInfo.VersionPrevious = versionInfo.Version`）

`VersionPrevious` 被赋予了两个互相冲突的含义：
- 在 `downloaded` 状态下它表示**"MainFolder 里真实装的版本"**；
- 在 `apply` 时它被当成**"备份目录的命名"**。

一旦"下载了 V2 但没应用，又下载 V3"，`VersionPrevious` 就变成了 V2——而磁盘上从来没装过 V2。

**复现链**（模拟器 G 场景，5/9 中断点命中；SDK 默认链路即可触发，无需特殊宿主）：

```
App 装 V1；下载 V2 → {Version: V2, Prev: V1, downloaded}
apply 中断（status=applying，App 仍是 V1 内容）
期间运维发布了 V3
重启 → check 返回 NeedDownloadUpdate=true → download V3
        → VersionPrevious = Version = V2   ← V2 从没被应用过！
        → {Version: V3, Prev: V2, downloaded}
apply：rename App(V1内容) → App_V2     ← 目录叫 V2，装的却是 V1
       rename App_V3 → App
```

**结果**：`list_rollback_versions` 会列出 `2.0.0`，用户点回滚到 2.0.0 → **拿到的是 1.0.0 的内容**，静默回到两代之前。对上位机这类场景，可能表现为"回滚后配置文件/协议版本不对"。

**修复方向**：把"MainFolder 真实内容版本"独立成字段（如 `active_version`），`download_update` 只在真正 apply 成功时才推进它；备份目录命名也用它，不再复用 `VersionPrevious`。

---

### Bug #3（P1，宿主相关）apply 中断 + 宿主无条件 download → 应用目录永久缺失

**代码位置**：`cmd/download_update.go:71-81`（applying 时跳过守卫）+ `cmd/common.go:704-723`（兜底只"保持"不回写）

`applyFailureFallback` 的"保持 applying"是**不修改**语义，它假设当前状态就是 `applying`。但如果中间跑过一次 `download_update`，状态早已被改写成 `downloaded`：

```
apply 中断在「备份改名后 / 应用改名前」
  → status=applying，App 不存在，App_V2 存在
宿主跑 download_update（applying 时跳过版本守卫）
  → 写 {Version: V2, Prev: V2, downloaded}   ← 状态被降级
apply → copy 阶段失败（App 不存在）
  → 兜底判定"主目录丢失+版本目录存在"→ 保持
  → 但保持的是 downloaded，不是 applying！
此后：check 走 checkUpdatePending（downloaded 分支），永远进不了崩溃恢复分支
```

**结果**：`ApplicationFolder` 永久消失，程序打不开，**无任何自愈路径**，只能人工重装。
模拟显示 4 轮轮询全部原地打转（`disk={'App_1.0.0', 'App_2.0.0'}`，`App` 始终不存在）。

**触发条件**：宿主在 `applying` 期间反复调 `download_update` 且服务器版本未变。

- ✅ C# SDK `AlyUpdateClient.MainLoop`（`AlyUpdateClient.cs:182`）只在 `NeedDownloadUpdate=true` 时下载，**不会触发**；
- ❌ 按 README quick-start 手动跑三步、或自研宿主"每次先 download 再 apply"，**会触发**。

**修复方向**（两个都建议做）：
1. `applyFailureFallback` 显式 `versionInfo.VersionStatus = applying` 而不是"什么都不做"；
2. `download_update` 在 `status=applying` 时只补文件、**不写 version.json**（尤其是不要动 `VersionPrevious`、不要把 `applying` 降级成 `downloaded`）。

顺带一提：同一个"无条件 download"路径下还会触发**静默装回旧版本**——因为 `Version == VersionPrevious` 会让 `prevVersionDir` 与 `versionDir` 变成同一个目录，最终 `App` 里装的是旧版本内容，而 version.json 写的是 `applied` + 新版本号。用户看到"更新成功"，实际跑的是旧程序。

---

### Bug #4（P1）断电写坏 version.json = 全命令瘫痪，且无自愈

`ReadVersion` 解析失败返回 error，`check_update` / `download_update` / `apply_update` / `rollback` **全部直接 return false**。没有任何备份副本、重建或降级逻辑。

仓库 E2E 的 S9a 只断言"安全报错（不崩溃、不卡死）"——**合规，但从恢复角度看是死局**：用户只能手工删掉 `version.json` 才能恢复，而这需要知道这个隐藏文件的存在。

另外 `WriteVersion` 只做了 `tmp + rename`，**没有 fsync**（Windows 下需要 `FlushFileBuffers`）。断电时完全可能出现"rename 已持久化、数据未落盘" → 得到 0 字节或半截 JSON。

**修复方向**：
1. `ReadVersion` 解析失败时：备份为 `version.json.corrupt`，重置为默认状态，并在 `update.log` 留痕（此时 `status=""` → 走 `checkUpdateApplied` → 重新下载应用，可自愈）；
2. 写之前先保留上一版 `version.json.bak`；
3. 写完后对文件句柄做 `FlushFileBuffers`（`syscall.FlushFileBuffers`）。

---

### Bug #5（P2）回滚中断后，SDK 永远不会续跑回滚

`AlyUpdateClient.MainLoop` 的循环是 `check → (download) → apply`，**没有 rollback 分支**。所以只要回滚中断，宿主重启后就不会再继续回滚：

- 若 `App` 还在 → `apply_update` 走"重做替换"或"标记 applied"，**回滚被静默取消**（模拟器 B1，7/7 命中）；
- 若 `App` 已不存在 → 走崩溃恢复 → 回到 Bug #1。

状态是一致的、程序能跑，但用户点回滚没生效，且不报错。

**修复方向**：`check_update` 在 `applying + RollbackPrevious != ""` 时，额外返回一个 `pending_rollback` 标记，SDK 据此调用 `rollback` 续跑；或让 `apply_update` 内部委托给回滚恢复（同 Bug #1 的修复）。

---

### Bug #6（P2）磁盘无上限增长

全仓库生产代码里**唯一的目录删除是 `removeAsideVariants`**（`cmd/common.go:596-608`，只删 `.old` 系列），没有任何旧版本目录的清理逻辑。每成功更新一次就永久留下一个完整副本（`ApplicationFolder_V1`、`_V2`、`_V3`…），中断产生的半成品目录同样不回收。

上位机常年运行 + 每月发版的场景下，这是确定的磁盘耗尽路径。

**修复方向**：保留最近 N 个（如 3 个）版本快照，超出删除最老的；清理动作放在 apply 成功之后。

---

### 其他较小的加固点

| 项 | 位置 | 说明 |
|----|------|------|
| 入口无条件删 `.old` | `apply_update.go:250` | `removeAsideVariants(prevVersionDir)` 在崩溃恢复时会删掉**尚未被覆盖的历史备份**（崩溃点在"旁移后/备份改名前"时），导致可回滚版本永久丢失。建议改为：只有当新备份已就位后才清理 |
| apply 前不校验 hash | `apply_update.go:239` | 下载时校验过，但 apply 可能是几小时后执行。建议 apply 前对 versionDir 内文件做一次 hash 抽验 |
| 后置脚本无幂等保护 | `apply_update.go:195-197` | 脚本在写 `applied` **之后**执行，执行中被杀不会重试；而崩溃恢复分支会**重跑**脚本。做数据库迁移类脚本时两个方向都会出问题。建议加 marker 文件 |
| 锁 TTL 30 分钟 | `cmd/lock.go:24` | PID 被活跃进程复用的极端场景下，最长 30 分钟无法更新（可用性，非数据损坏） |
| 小文件不续传 | `download_update.go:15` | `largeFileThreshold=100MB`，小于 100MB 的文件中断后整份重下（效率，非正确性） |

---

## 三、建议的修复顺序

| 优先级 | 事项 | 改动量 | 说明 |
|--------|------|--------|------|
| 1 | Bug #1：applying 区分更新/回滚 | 小 | 只改 `apply_update` 崩溃恢复分支的判断 |
| 2 | Bug #3：兜底显式回写 applying + download 不降级状态 | 小 | 两个点各几行 |
| 3 | Bug #4：version.json 损坏自愈 | 小 | 改 `ReadVersion` + 加 `.bak` |
| 4 | Bug #2：引入 `active_version` 字段 | 中 | 涉及 version.json 结构与 apply/rollback 的命名逻辑，需回归全部 E2E |
| 5 | Bug #5：check_update 暴露 pending_rollback | 中 | 涉及 Go 端输出契约 + C# SDK（**改 SDK 前务必先在上位机端到端验证**，见仓库 `#27` 的教训） |
| 6 | Bug #6：版本目录配额 | 中 | 独立功能，风险低 |

**E2E 补测建议**：现有 S8 只覆盖 download/apply 中断，**完全没有 rollback 中断场景**。建议新增 `S11`：在 rollback 的 7 个阶段注入强杀，断言"重跑 rollback 能回到目标版本"，以及"宿主按 check→apply 流程时不会被推向新版本"。

---

## 四、复现方式

```bash
cd tools/interrupt-sim
python3 scenarios3.py     # 多轮恢复判定（48 场景，覆盖本报告的 Bug #1/#2/#3/#5）
python3 scenarios.py      # 单轮场景（含 download 中断、目录错配、version.json 损坏）
```

输出同时写入 `results/multi_round_report.txt` 与 `results/single_round_report.txt`（本目录下的 `result*.json` 为机器可读明细）。

判据定义在 `sim.py:check_consistent`：① 主目录必须存在；② `applied` 时磁盘内容版本 == `version.json.Version`；③ `downloaded` 时 == `VersionPrevious`；④ 回滚目录名与其内容版本必须一致。
