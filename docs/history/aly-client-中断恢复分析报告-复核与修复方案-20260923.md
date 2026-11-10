# aly-client「中断恢复」分析报告 —— 复核结论与详细修复方案

> 复核对象：`aly-client-中断恢复分析报告-20260923.md`（下称"原报告"，报告对象 commit `8aed31d`，当前 HEAD `10b3ab9` 仅追加模拟器与报告，被分析代码未变）
> 复核方法：逐行核对 `client/aly-client` 源码（含行号证据）→ 重跑 `tools/interrupt-sim` 模拟器验证可复现 → 对 4 个 P1 bug 用真实代码手工推演关键场景 → `go test` 建立基线
> 复核日期：2026-09-23

---

## 一、总判定（先说结论）

**原报告 6 个 bug 全部真实存在**，其中 5 个判定与代码完全一致；1 个（#3）结论成立但机制描述有 1 处偏差；1 个（#2）的触发面比原报告描述的**更宽**（不需要任何中断，仅"下载待应用期间服务器发布新版"即可触发）。测试方法**总体正确、可复现**，模拟器对状态机语义的复刻足以支撑全部结论，但有 3 处保真度简化需要在解读结果时注意。

| # | 项 | 原报告判定 | 复核结论 | 关键证据 |
|---|----|-----------|---------|---------|
| — | 下载中断 | ✅ 安全 | ✅ 确认 | `.part`+rename（`http_client.go:293-341`）+ MD5/SHA256 双校验（`download_update.go:160-190`）；模拟 D 族 5/5 PASS；E2E S8a/S9b |
| — | apply 中断 9 点自愈 | ✅ 安全 | ✅ 确认 | 模拟 A 族 9/9 一轮 PASS；E2E S8b/S8c 一致 |
| Bug #1 | 回滚中断 → 被静默逆转为升级 | ❌ 有 bug | ✅ **确认（P1）** | `apply_update.go:78-137` 崩溃恢复分支全程只读 `versionInfo.Version`，从不读 `RollbackPrevious` |
| Bug #2 | 中断期间服务器发新版 → 回滚目录名字与内容错配 | ❌ 有 bug | ✅ **确认（P1），触发面更宽** | `download_update.go:196` 无条件 `VersionPrevious = Version`；无需中断，V2 下载待应用期间发布 V3 即可触发 |
| Bug #3 | apply 中断 + 宿主无条件 download → 应用目录永久缺失 | ❌ 有 bug | ✅ **确认（P1，宿主相关）**，机制描述 1 处偏差 | `download_update.go:71-81` applying 跳过守卫 + `common.go:704-723` 兜底"只保持不回写"；详见 §二.3 |
| Bug #4 | 断电写坏 version.json → 全命令瘫痪无自愈 | ❌ 有 bug | ✅ **确认（P1）** | `config/version.go:53-55` 解析失败即 error；`check/download/apply/rollback` 4 命令全部直接 return false；`WriteVersion`（72-81）无 fsync |
| Bug #5 | 回滚中断后 SDK 永不续跑回滚 | ❌ 有 bug | ✅ **确认（P2）**，但**无需改 SDK 契约**即可修复 | `AlyUpdateClient.cs:163-251` MainLoop 只有 check→download→apply，无 rollback 分支 |
| Bug #6 | 磁盘无上限增长 | ❌ 有 bug | ✅ **确认（P2）** | 生产代码唯一 `os.RemoveAll` 在 `removeAsideVariants`（`common.go:604`），无任何旧版本快照清理 |
| 加固① | 入口无条件删 `.old` | 建议修 | ⚠️ **确认存在但严重度下调** | 多数场景备份会被本次替换重建，仅"旁移内容与当前 MainFolder 内容不同"的历史快照才真正丢失 |
| 加固② | apply 前不校验 hash | 建议修 | ✅ 确认（低危） | 下载时已校验；仅 apply 时从 MainFolder 复制进来的文件未校验 |
| 加固③ | 后置脚本无幂等保护 | 建议修 | ✅ **确认** | `apply_update.go:195-197` 写 `applied` 后执行；崩溃恢复分支（100-102、128-130）会重跑脚本 |
| 加固④ | 锁 TTL 30 分钟 | 建议修 | ✅ 确认（可用性，非正确性） | `lock.go:24` |
| 加固⑤ | 小文件不续传 | 建议修 | ✅ 确认（效率） | `download_update.go:15` `largeFileThreshold=100MB` |

**一句话复核结论**：原报告的结论与代码事实吻合，"崩溃安全"层（原子写、状态机、崩溃恢复分支）确实扎实；真正的缺陷集中在**状态模型**——`applying` 单状态无法区分"更新中/回滚中"，且缺少"服务器版本漂移"与"active 版本"的建模。修复方向正确，本报告在其基础上给出可直接落地的代码级方案。

---

## 二、逐条代码核实

### 2.1 Bug #1：回滚被逆转为升级 —— 确认

**证据链**：

- `rollback.go:125-132`：回滚启动时写 `status=applying` + `RollbackPrevious = oldVersion`（oldVersion 按 78-84 行取：downloaded→`VersionPrevious`，applying→`RollbackPrevious`）。
- `apply_update.go:78-137` 崩溃恢复分支：`versionDir, _ := fc.ExeCfg.AppVersionDir(versionInfo.Version)`（88/110 行）——**用 `Version`（待应用的新版本）找目录**，全程不读 `RollbackPrevious`。
- 复现链（模拟器 B2，7/7 中断点全部命中，`results/multi_round_report.txt` B2 族全 WARN"意图错乱：期望 1.0.0，实际活动 3.0.0"）：
  ```
  App=V2，version.json={V3, V2, downloaded}，磁盘有 App_V1、App_V3
  用户回滚到 V1 → rollback 写 applying+RollbackPrevious=V2 → App→App_V2 备份 ✅ → 崩溃
  重启 → check：applying 分支 → server==V3 → NeedDownloadUpdate=false
       → apply_update 崩溃恢复：App 缺失，AppVersionDir(V3) 存在 → rename App_V3→App
       → 写 applied，Version=V3 → 用户"回滚到 V1"被静默变成"升级到 V3"
  ```
- B2' 分支（崩溃点在 `rb:remove_aside_done`，回滚实际已完成只差写状态）：`rollback.go:53-56` 目标目录已被消耗 → 重跑 `rollback --version V1` 报 "version 1.0.0 not found"，status 卡 `applying` → 被 apply 接管推向 V3（模拟器 B2' `crash@rb:remove_aside_done` FAIL，与报告一致）。

**手工推演（真实代码路径）确认无误**：B2 崩溃后 apply_update 的 fall-through 分支（MainFolder 存在 + versionDir 存在 → 重做替换）会把 `App(V1 内容)` 备份成 `App_2.0.0`（名字与内容再次错配），最终 `App=V3`、`App_2.0.0=V1 内容`，与模拟器终态一致。

### 2.2 Bug #2：回滚目录"名字与内容错配" —— 确认，且触发面比原报告更宽

**证据链**：

- `download_update.go:196`：`versionInfo.VersionPrevious = versionInfo.Version`。`VersionPrevious` 同时承担两个含义：downloaded 状态下的"MainFolder 真实内容版本"（`list_rollback.go:56-60`、`rollback.go:79-81`、`check_update.go:110/128` 都按此语义读它），以及 apply 时备份目录命名（`apply_update.go:244`）。
- 原报告 G 场景（apply 中断 + 服务器发 V3）确认存在：模拟器 G 族 5/9 中断点 WARN（"回滚目录 App_2.0.0 装的是 1.0.0"）。
- **复核发现更宽的触发路径——完全不需要中断**：
  ```
  App=V1，下载 V2 → {V2, V1, downloaded}（V2 待应用）
  运维发布 V3 → check：downloaded 分支，needUpdate(V3, V2)=true → NeedDownloadUpdate=true
  download V3 → VersionPrevious = Version = V2 → {V3, V2, downloaded}  ← V2 从没被应用过！
  apply → App(V1) → App_V2 备份（目录叫 V2，装的 V1）→ App_V3 → App
  list_rollback 列出 2.0.0 → 用户回滚到 2.0.0 → 拿到 1.0.0 内容
  ```
  这是 **SDK 默认链路**即可触发（无需自定义宿主），比原报告"需要 apply 中断"的门槛更低，应升级为最高优先级。

### 2.3 Bug #3：无条件 download → 应用目录永久缺失 —— 确认，机制描述 1 处偏差

**确认成立的部分**：

- `download_update.go:71-81`：`VersionStatus == applying` 时**跳过全部守卫**（允许同名重下）。
- `common.go:704-713` `applyFailureFallback`：主目录缺失 + 版本目录存在 → "保持 applying"（**不写盘**）。
- 复现链（模拟器 C 族 `crash@apply:rename_activate` FAIL，4 轮轮询原地打转）与报告一致：无条件 download 把状态改写为 `downloaded` → apply 以 downloaded 进入普通流程 → `CopyDirWithExclude(MainFolder 缺失)` 失败 → 兜底"保持" → 下次 check 再也进不了崩溃恢复分支 → **App 永久缺失**。
- "静默装回旧版本"变体确认：`Version==VersionPrevious` 时 `prevVersionDir == versionDir`（同一目录），旁移→备份→激活三次 rename 后 `App` 里是旧内容、version.json 写 `applied`+新版本号（模拟器 C 族 `crash@apply:copy/remove_aside_entry/rename_backup` 报"版本错配：磁盘内容是 1.0.0，version.json 声明 2.0.0"）。

**偏差 1 处（不影响结论）**：报告称"兜底保持的是 downloaded，不是 applying"。实际 `apply_update.go:143-148` 在进入 `applyReplacement` **之前**已把 `status=applying` 写盘（第 144 行），所以失败后磁盘状态是 **applying** 而非 downloaded；"永远进不了崩溃恢复分支"的真正原因是：无条件 download 宿主每轮都把状态改回 downloaded，apply 每次读到的都是 downloaded、走普通流程、在 copy 处失败。净效果（App 永久缺失、无自愈）与报告一致。修复时应让兜底**显式写盘 applying**（不依赖 144 行的顺带写），以消除该偏差。

**触发条件核对**：C# SDK `AlyUpdateClient.MainLoop`（`AlyUpdateClient.cs:182`）只在 `NeedDownloadUpdate=true` 时下载，且 `checkUpdatePending`（`check_update.go:126-137`）在服务器版本未高于本地待应用版本时返回 `NeedDownloadUpdate=false` → SDK 不会触发；按 README 手动三步或自研宿主"每次先 download 再 apply"会触发。与原报告一致。

### 2.4 Bug #4：version.json 损坏 = 全命令瘫痪 —— 确认

- `config/version.go:52-55`：`json.Unmarshal` 失败 → `return nil, error`。
- `check_update.go:35-39` / `download_update.go:61-65` / `apply_update.go:66-70` / `rollback.go:58-62`：全部 `printOutput(false, ...)` 直接返回。`list_rollback.go:51` 忽略错误（不会瘫痪但也不自愈）。
- `WriteVersion`（`version.go:72-81`）只有 `tmp + rename`，**无 fsync**（Windows 下无 `FlushFileBuffers`）——断电窗口内可能 rename 已持久化而数据未落盘，得到 0 字节或半截 JSON。
- E2E S9a（`update_e2e.ps1:598-607`）只断言"安全报错（不崩溃、不卡死）"，未要求自愈——与原报告"合规但从恢复角度看是死局"一致。

### 2.5 Bug #5：SDK 永不续跑回滚 —— 确认

- `AlyUpdateClient.cs:163-251`：MainLoop = `check → (NeedDownloadUpdate 时 download) → apply`，**没有任何 rollback 调用**。回滚中断后宿主重启，SDK 只会走 check→apply：
  - App 还在 → apply_update 崩溃恢复走"重做替换"或"标记 applied"→ 回滚被静默取消（模拟器 B1 族 7/7 WARN）；
  - App 已缺失 → 崩溃恢复 → Bug #1（推向新版本）。
- **修复路径重要发现**：该 bug **不需要改 C# SDK 契约**。只要 Go 端（a）check_update 在 `applying + RollbackPrevious != ""` 时强制 `NeedDownloadUpdate=false` 并输出回滚目标，（b）apply_update 的崩溃恢复委托给回滚恢复——SDK 现有的 check→apply 循环就会自动续跑回滚。这正好规避了原报告引用的仓库 `#27` 教训（改 SDK 前需上位机端到端验证）。

### 2.6 Bug #6：磁盘无上限增长 —— 确认

- 全仓库 `os.RemoveAll` 检索结果：生产代码唯一一处是 `common.go:604`（`removeAsideVariants`，只删 `.old` 系列）；`apply_update.go:286-287` 成功路径也只清 `.old` 变体，**从不删旧版本快照**（`ApplicationFolder_V1/_V2/_V3…` 每个更新永久保留一份完整副本）。
- 中断产生的半成品目录（旁移残留以外的版本目录）同样不回收。
- 上位机常年运行 + 每月发版 → 确定的磁盘耗尽路径。确认。

### 2.7 加固点核实

| 项 | 位置 | 复核结论 |
|----|------|---------|
| 入口无条件删 `.old` | `apply_update.go:250`、`rollback.go:158` | ⚠️ 存在但严重度下调：崩溃点"旁移后/备份改名前"时被删的 `.old` 在本次替换完成后会被等价的 `prevVersionDir` 备份重建（内容同版本），仅当该 `.old` 是**不同版本的历史快照**时才真正丢失。仍建议把清理时机后移到新备份就位后 |
| apply 前不校验 hash | `apply_update.go:239` | 确认（低危）：下载时 MD5+SHA256 已校验（且 download 对"目标目录已有正确文件"也做了双 hash 校验，`download_update.go:127-132`），仅 apply 时从 MainFolder 复制进来的文件未校验（这些是本地生成/配置类文件） |
| 后置脚本无幂等保护 | `apply_update.go:195-197` + 崩溃恢复 100-102/128-130 | ✅ 确认：写 `applied` 之后执行脚本，执行中被杀不会重试；而崩溃恢复分支会重跑脚本——做数据库迁移类脚本时两个方向都出问题 |
| 锁 TTL 30 分钟 | `lock.go:24` | 确认（可用性）：PID 被活跃进程复用的极端场景下最长 30 分钟无法更新；`cleanStaleLock`（108-137）用 `GetExitCodeProcess`（`process.go:430-453`）判活，机制本身正确 |
| 小文件不续传 | `download_update.go:15`、`http_client.go:235` | 确认（效率）：`serverFileSize > 100MB` 才走 `.part` 续传，小文件中断后整份重下（有 3 次重试 + 校验，正确性无问题） |

### 2.8 "做对的地方"核实（7 项全部属实）

1. `WriteVersion` tmp+rename（`version.go:72-81`）✅
2. 下载 `.part`+rename + MD5/SHA256 必校验（`http_client.go:293-341`、`download_update.go:160-190`）✅
3. `CopyFile` 也是 `dst.tmp + rename`（`util/file.go:66-99`）✅
4. `applyReplacement` 复制 `overwrite=false`（`file.go:49-53`、`apply_update.go:232/239`）✅
5. 三次 rename 的错误码分类 + 占用者查杀 + explorer 兜底（`common.go:340-456`）✅
6. `#7` 兜底保持 applying 方向正确（`common.go:704-723` + `apply_failure_test.go:79-143` 有测试）✅
7. 锁陈旧回收用 `GetExitCodeProcess`（`process.go:430-453`）✅

---

## 三、测试方法评估（原报告模拟器）

### 3.1 可复现性 —— ✅ 通过

重跑验证：

```
cd tools/interrupt-sim
python scenarios3.py   → 48 场景，输出与提交的 results/multi_round_report.txt 逐条一致
python scenarios.py    → 46 场景（不一致/失败 20），与 results/single_round_report.txt 一致
```

新生成的 `result.json`/`result_final.json` 与提交的 `results/*.json` 经 JSON 归一化后**逐条相等**（差异仅为空白/键序/Unicode 转义格式）。模拟器无随机性，结论可稳定复现。

### 3.2 状态机复刻保真度 —— ✅ 足够支撑结论

逐函数对照确认：`cmd_download_update`↔`download_update.go`、`cmd_apply_update`+`_apply_replacement`↔`apply_update.go:78-289`、`cmd_rollback`↔`rollback.go:47-233`、`cmd_check_update`↔`check_update.go`（pending 分支的 `need_download = server > Version` 与真实 `needUpdate(serverVersion, localVersion=Version)` 等价）、`apply_failure_fallback` 语义↔`common.go:704-723`、`remove_aside_variants`↔`common.go:596-608`。

判据（`sim.py:check_consistent`）合理且比 E2E 更严：① 主目录必须存在；② applied 时磁盘内容==Version；③ downloaded 时==VersionPrevious；④ 回滚目录名==其内容版本。

**3 处保真度简化（解读结果时需注意，均不影响本报告结论）**：

1. **下载模型粗粒度**：`dl:download` 直接把目标目录标记为新版本，未建模"目标目录已有正确文件则跳过/断点续传/校验失败删除重下"分支。影响：E/D 族对"半成品目录是否被重新校验覆盖"的结论需以 E2E S8a/S9b/S9c 为准（E2E 已覆盖且通过）。
2. **rename 目标存在一律视为失败**：真实 `renameDirWithKillState`（`common.go:365-372`）会先把目标旁移为 `.old.N` 再重命名；模拟器 `Disk.rename` 直接报错。等价性由入口 `remove_aside_variants(prev_dir)` 先清空 `.old` 系列保证，常规路径两者行为一致。
3. **copy 失败条件**：主目录缺失 → copy 失败，与真实 `CopyDirWithExclude(filepath.Walk)` 行为一致 ✅；但模拟器对"版本目录缺失时 copy"报错，而真实代码 `CopyFile` 里 `MkdirAll` 会重建目录——该分支在报告场景中不出现（versionDir 必然先于 apply 存在），无影响。

### 3.3 方法缺口（修复落地前建议补齐）

| 缺口 | 说明 |
|------|------|
| version.json 损坏 + applying 中断组合 | Bug #4 修复引入"损坏自愈"，需补测"损坏发生在 MainFolder 已缺失的 applying 现场"（修复 3 的边界分析见 §四.3） |
| 回滚中断 + 服务器同时发新版 | B2 与 G 的组合：回滚中断期间运维发新版，check 必须优先续回滚而非下载（修复 1 的 check 分支），需补模拟场景 |
| 双进程并发 | 模拟器不建模全局锁；`lock_test.go` 已覆盖，修复不涉及锁逻辑 |
| rollback 中断场景 E2E | 原报告建议的 S11 正确且必要：现有 E2E 的 S8 只覆盖 download/apply 中断，**完全没有 rollback 中断场景**（见 §五 回归计划） |

### 3.4 与仓库 E2E 的一致性

- 模拟器 A 族（apply 9 点一轮自愈）与 E2E S8b/S8c（真实进程强杀 + 恢复断言）结论一致；
- D 族（download 中断）与 S8a 一致；
- F 族（version.json 损坏）与 S9a 一致（"安全报错"），且原报告正确指出 S9a 未覆盖自愈。

---

## 四、详细修复方案

> 通用约束：Go 1.10（GOPATH 模式、`GOARCH=386`、兼容 XP）——不得使用 generics/`errors.Is`/`os.ReadFile` 等新 API；`version.json` 只增字段（`omitempty`），老文件兼容；**C# SDK 契约尽量不动**（仓库 #27 教训），需要改时单独列明并标注"必须先上位机端到端验证"。

### 修复 1（P1，对应 Bug #1 + Bug #5）：回滚中断识别与续跑

**根因**：`applying` 单状态 + 崩溃恢复分支只认 `Version`；且回滚目标版本（`V1`）从未持久化，apply_update 无法重建回滚意图。

**改动清单**：

1. **`config/version.go`**：新增字段（只增、向后兼容）：
   ```go
   // RollbackTarget 记录本次回滚的目标版本，供回滚中断后的崩溃恢复使用。
   // 仅在 rollback 写 status=applying 时写入，恢复完成后清空。
   RollbackTarget string `json:"rollback_target,omitempty"`
   ```

2. **`cmd/rollback.go:125-132`**：写状态时一并持久化目标：
   ```go
   versionInfo.VersionStatus = config.VersionStatusApplying
   versionInfo.RollbackPrevious = oldVersion
   versionInfo.RollbackTarget = *versionFlag      // 新增
   ```

3. **`cmd/apply_update.go`**：崩溃恢复分支开头区分"更新中断/回滚中断"：
   ```go
   case config.VersionStatusApplying:
       // 回滚中断：按回滚语义恢复，绝不按 versionInfo.Version 升级
       if versionInfo.RollbackPrevious != "" {
           if err := resumeRollback(fc, versionInfo, closeTimeout); err != nil {
               printOutput(false, fmt.Sprintf("rollback crash recovery failed: %v", err), nil)
               return
           }
           printOutput(true, "", nil)
           return
       }
       // ...原更新中断恢复逻辑（78-137 行）保持不变...
   ```

4. **`cmd/rollback.go`（或新增 `cmd/rollback_recover.go`）**：`resumeRollback` —— 按磁盘现场 4 分支恢复（纯 rename，幂等）：

   | 现场 | 判定 | 动作 |
   |------|------|------|
   | MainFolder 存在 + `AppVersionDir(target)` 不存在 | 激活已完成 | 补写 `applied`：`Version=target, VersionPrevious=RollbackPrevious`，清空 `RollbackPrevious/RollbackTarget`，跑后置脚本、启动主程序 |
   | MainFolder 存在 + target 目录仍存在 | 备份改名前的崩溃 | 重做回滚替换：`MainFolder → AppVersionDir(RollbackPrevious)` 备份（旧备份先旁移），`AppVersionDir(target) → MainFolder` 激活（复用 rollback.go 现有替换段，提取为共享函数 `performVersionSwap`） |
   | MainFolder 缺失 + target 目录存在 | 备份改名后、激活前崩溃 | `AppVersionDir(target) → MainFolder`，补写 `applied` |
   | MainFolder 缺失 + target 目录缺失 + `AppVersionDir(RollbackPrevious)` 存在 | 备份已做、目标被消耗（B2' 现场） | 恢复备份：`AppVersionDir(RollbackPrevious) → MainFolder`（**放弃回滚**回到回滚前版本），补写 `applied`（`Version=RollbackPrevious`） |
   | 其余（目标与备份都不存在） | 无法恢复 | 返回 error，状态保持 applying 由宿主重试 |

   > 兼容老数据：`RollbackPrevious != ""` 但 `RollbackTarget == ""`（本修复发布前的存量数据）时，`target` 取 `RollbackPrevious` 对应的回滚前版本目录不可用，按"安全放弃回滚"处理（上表最后两行），**绝不把 `versionInfo.Version` 当目标**。

5. **`cmd/check_update.go` `checkUpdatePending`**：`applying + RollbackPrevious != ""` 时**强制不下载**并输出回滚目标（这是让 SDK 现有 check→apply 循环自动续跑回滚的关键，**不改 SDK**）：
   ```go
   // 回滚中断：优先完成回滚，绝不因服务器发布新版本而转向升级
   if versionInfo.RollbackPrevious != "" {
       target := versionInfo.RollbackTarget
       if target == "" { target = versionInfo.Version } // 老数据兜底：至少不下载
       prev := versionInfo.RollbackPrevious
       printOutput(true, "", &model.CheckUpdateData{
           HasUpdate:          true,
           NeedDownloadUpdate: false,
           CurrentVersion:     stripVPrefix(prev),
           NewVersion:         stripVPrefix(target),
       })
       return
   }
   ```
   效果：UI 显示的目标即用户回滚目标，SDK 调 apply_update → 修复 1 第 3 步续跑回滚。Bug #5 因此**无需任何 C# 改动**。

6. **`cmd/rollback.go:47-56` 早退修复**：B2' 现场（目标目录已消耗）重跑 `rollback --version` 会先报 "version not found" 卡死。调整顺序：先 `ReadVersion`，若 `applying + RollbackPrevious != ""` 则直接委托 `resumeRollback`（不再要求 CLI 目标存在），CLI 版本仅作为期望值校验。

**测试**：
- 单测：构造 4 种磁盘现场 + 老数据（无 `rollback_target`）各 1 例，断言恢复后的 `version.json` 与目录命名。
- 模拟器：在 `sim.py` 增加 `RollbackTarget` 字段与 `resumeRollback` 分支，B2/B2' 族应全部转 PASS；新增"回滚中断 + 服务器发新版"组合场景。
- E2E：新增 S11（见 §五）。

### 修复 2（P1，对应 Bug #3）：兜底显式写盘 + download 禁止降级状态

**改动清单**：

1. **`cmd/common.go` `applyFailureFallback`**（704-713 行）：主目录缺失分支**显式写盘**，不依赖 `apply_update.go:144` 的顺带写：
   ```go
   if mainFolderLostButVersionDirExists(fc, versionInfo) {
       // 显式落盘：保持 applying，交给崩溃恢复分支修复（消除"取决于 144 行先行写盘"的隐式依赖）
       versionInfo.VersionStatus = config.VersionStatusApplying
       if wErr := config.WriteVersion(versionInfo); wErr != nil {
           util.AppendToLog(logDir(), "update.log",
               fmt.Sprintf("keep applying: write version failed: %v", wErr))
       }
       return failErr.Error()
   }
   ```
   （现有 `TestApplyFailureFallbackKeepsApplyingWhenMainFolderLost` 已断言"保持 applying（内存+磁盘）"，本次改动使该断言从"碰巧通过"变成"显式保证"，测试继续通过。）

2. **`cmd/download_update.go`**：`status=applying` 时**禁止改写 version.json**（只允许补下载文件，不写状态、不动 `VersionPrevious/RollbackPrevious`）：
   ```go
   // applying 期间：正在执行 apply/回滚的崩溃恢复现场，下载只能补文件，
   // 不得把 applying 降级成 downloaded，也不得动 VersionPrevious/RollbackPrevious。
   inFlight := versionInfo.VersionStatus == config.VersionStatusApplying
   // ...（下载/校验逻辑照旧，仅文件写入 targetDir）...
   if !inFlight {
       versionInfo.VersionPrevious = versionInfo.Version
       versionInfo.Version = newVersion
       versionInfo.VersionStatus = config.VersionStatusDownloaded
       versionInfo.RollbackPrevious = ""
       versionInfo.AfterApplyUpdateScript = latestLog.AfterApplyUpdateScript
       if err := config.WriteVersion(versionInfo); err != nil { ... }
   } else {
       util.AppendToLog(logDir(), "download.log",
           "applying 进行中，跳过 version.json 状态写入（仅补充下载文件）")
   }
   ```
   配合修复 3 的 check 策略，即使宿主"无条件先 download 再 apply"，状态也不被降级 → apply 下次以 `applying` 进入崩溃恢复分支 → 自愈。

3. **`cmd/check_update.go` `checkUpdatePending`**：`applying`（非回滚）时**优先完成在途 apply**，只有 apply 自身失败降级为 `downloaded` 后才允许下载新版本（保留"版本目录也丢失"场景的下载逃生口）：
   ```go
   // applying（非回滚）：先完成/重试在途 apply，不要重新下载。
   // 若 apply 失败且主目录/版本目录都不存在，applyFailureFallback 会降级为
   // downloaded，下一轮 check 自然允许下载新版本（逃生口保留）。
   if versionInfo.VersionStatus == config.VersionStatusApplying {
       printOutput(true, "", &model.CheckUpdateData{
           HasUpdate:          true,
           NeedDownloadUpdate: false,
           CurrentVersion:     stripVPrefix(currentActive(versionInfo)),
           NewVersion:         stripVPrefix(versionInfo.Version),
       })
       return
   }
   ```
   （注：现有"服务器版本更高则重下"逻辑在 downloaded 状态保留，仅 applying 提前返回。）

**副作用说明**：修复后"永久占用者导致 rename 持续失败"的场景会卡在 applying 循环（无下载逃生口）。建议同时给 `applyFailureFallback` 增加"连续失败 N 次（如 3 次）后强制降级 downloaded 并报警"的可配置策略（默认关闭或 N 较大），避免极端现场无出路。此项列为可选项，需与运维确认。

### 修复 3（P1，对应 Bug #4）：version.json 损坏自愈 + 落盘加固

**改动清单**：

1. **`config/version.go` `ReadVersion`**：解析失败自愈链：
   ```go
   data, err := ioutilReadFile(path)
   if err != nil {
       if os.IsNotExist(err) { return &VersionInfo{}, nil }
       return nil, err
   }
   var info VersionInfo
   if err := json.Unmarshal(data, &info); err != nil {
       // ① 尝试上次成功写入的备份
       if bak, bErr := ioutilReadFile(path + ".bak"); bErr == nil {
           var bakInfo VersionInfo
           if json.Unmarshal(bak, &bakInfo) == nil {
               util.AppendToLog(exeDirForLog(), "update.log",
                   fmt.Sprintf("version.json 损坏，已从 .bak 恢复: %v", err))
               WriteVersion(&bakInfo) // 用备份重建（WriteVersion 内部不读 ReadVersion，无递归）
               return &bakInfo, nil
           }
       }
       // ② 双份都不可用：隔离损坏文件并重置为首次部署语义（status="" → checkUpdateApplied → 重新下载应用，可自愈）
       os.Rename(path, path+".corrupt")
       util.AppendToLog(exeDirForLog(), "update.log",
           fmt.Sprintf("version.json 损坏且无可用备份，已隔离为 .corrupt 并重置: %v", err))
       return &VersionInfo{}, nil
   }
   return &info, nil
   ```
   （`exeDirForLog` 用现有 `logDir()`/`config.ExeDir()` 表达式；`config` 包内避免依赖 `cmd.logDir`。）

2. **`config/version.go` `WriteVersion`**：先备份上一版 + 落盘强化：
   ```go
   // ① 先保留上一版（last-known-good），供 ReadVersion 自愈回退
   if _, statErr := os.Stat(path); statErr == nil {
       copyFileSimple(path, path+".bak") // 用 ioutil 读+写即可，或 os.Rename 前先复制
   }
   // ② tmp 写完 → FlushFileBuffers（Windows 落盘）→ rename
   tmpPath := path + ".tmp"
   f, err := os.Create(tmpPath)
   if err != nil { ... }
   if _, err := f.Write(data); err != nil { f.Close(); os.Remove(tmpPath); ... }
   if err := f.Sync(); err != nil { f.Close(); os.Remove(tmpPath); ... } // Go 的 Sync 在 Windows 即 FlushFileBuffers
   if err := f.Close(); err != nil { ... }
   if err := os.Rename(tmpPath, path); err != nil { ... }
   ```
   > Go 的 `*os.File.Sync()` 在 Windows 底层就是 `FlushFileBuffers`，Go 1.10 已支持，无需直接调 `syscall.FlushFileBuffers`。**顺序必须是：写 tmp → Sync → rename**（先落盘后发布），比"rename 后补 flush"更正确。

3. **边界分析（损坏 + applying + 主目录缺失组合）**：`.bak` 持有损坏前一刻的状态（多为 downloaded/applied）。若损坏发生在 applying 中途：
   - `.bak = downloaded` → 恢复后 check 走 pending 分支 → apply 普通流程：主目录缺失则 copy 失败 → `applyFailureFallback` 判"主目录丢失+版本目录存在" → 显式写 applying（修复 2）→ 下轮崩溃恢复 `AppVersionDir → MainFolder` 完成 → **自愈**（多一轮）。
   - 主目录与版本目录都缺失 → 降级 downloaded → 下载重装（逃生口）。✅ 无死局。
   - 双份都损坏 → 重置为 `""` → checkUpdateApplied → 全量重下重装。✅ 可自愈（代价是重装一次）。

**测试**：单测 4 例——① 半截 JSON + 有效 `.bak` → 从 bak 恢复；② 0 字节 + 无 `.bak` → 重置为 empty 且生成 `.corrupt`；③ 合法 JSON 不触发自愈；④ 恢复后 `WriteVersion` 落盘成功。

### 修复 4（P1，对应 Bug #2）：`VersionPrevious` 语义收敛（两阶段）

**阶段一（最小改动，立即生效）**：`download_update.go:196` 只在能确定 active 版本时写 `VersionPrevious`：

```go
// VersionPrevious 语义 = "MainFolder 当前真实内容版本"，仅在可确定时更新：
//  - downloaded：MainFolder 仍是 VersionPrevious 的内容 → 保持不变（修复"V2 待应用期间发布 V3"错配）
//  - applying：不碰（崩溃恢复现场，由恢复逻辑决定；且修复 2 已禁止此状态下写版本状态）
//  - applied / 空：MainFolder = Version → 记录为 VersionPrevious
if versionInfo.VersionStatus != config.VersionStatusDownloaded {
    versionInfo.VersionPrevious = versionInfo.Version
}
```
（结合修复 2 第 2 条，applying 分支整体跳过写状态，此判断只对 downloaded 生效。）

验证：模拟器 G 族应全部转 PASS（`App_2.0.0` 不再出现"装 1.0.0"）；E2E 全量回归（S8 各轮依赖的目录命名不能变）。

**阶段二（彻底方案，原报告建议，需全量 E2E 回归）**：引入 `active_version` 字段作为"MainFolder 真实内容版本"的**唯一事实源**：

1. `config/version.go` 新增 `ActiveVersion string json:"active_version,omitempty"`。
2. **只在替换成功时推进**：apply_update 3 个成功点（普通成功 186-192、崩溃恢复 2 分支 95-105/121-133）写 `ActiveVersion = Version`；rollback 成功点（普通 214-222、崩溃恢复 106-118）写 `ActiveVersion = target`。
3. **消费方全部改用 `ActiveVersion`（老文件回退 `VersionPrevious`）**：
   - `apply_update.go:244` 备份目录命名：`prev := versionInfo.ActiveVersion; if prev == "" { prev = versionInfo.VersionPrevious }`；
   - `rollback.go:78-84` oldVersion 推导：优先 `ActiveVersion`；
   - `list_rollback.go:56-60` current_version、`check_update.go:110/128` currentVersion 显示：优先 `ActiveVersion`。
4. `download_update.go` 写 `VersionPrevious = ActiveVersion`（而非 `Version`），彻底消除双含义。

阶段二改动面中等，但把"备份目录名 == 内容"变成不变量（由构造保证而非约定），后续任何新流程都不会再踩 Bug #2。建议在阶段一验证通过、E2E 绿后单独提交。

### 修复 5（P2，对应 Bug #6）：版本快照配额

**改动**：新增 `cmd/prune.go`：
```go
// pruneVersionSnapshots 保留最近 keep 个版本快照，删除更旧的（apply/rollback 成功后调用）。
func pruneVersionSnapshots(fc *FullConfig, keep int) {
    pkgDir, err := config.PackageDir()
    if err != nil { return }
    folderName, err := fc.ExeCfg.MainExeFolderName()
    if err != nil { return }
    // 枚举 pkgDir 下 {folderName}_{version} 目录（isLikelyVersion 过滤，排除 .old 变体）
    // 按 compareVersion 降序排序；排除运行中目录：
    //   MainFolder、AppVersionDir(versionInfo.Version)、AppVersionDir(versionInfo.VersionPrevious)
    // 删除第 keep+1 个及之后的最老目录（os.RemoveAll + update.log 留痕），keep 默认 3
}
```
调用点：apply_update 成功（普通 + 2 个崩溃恢复完成点）、rollback 成功（普通 + 崩溃恢复完成点）。独立功能、风险低。

### 加固项（P2/P3）

| 项 | 方案 |
|----|------|
| 后置脚本幂等 | 脚本完成后写 marker（如 `{MainFolder}/.updator/after_apply_{Version}.done`）；崩溃恢复分支先查 marker，存在则跳过（不重跑）；正常 apply 成功后清理其它版本的旧 marker。**契约：脚本必须可幂等执行**（marker 语义为 at-most-once，被杀后重跑依赖幂等性，写进文档） |
| apply 前 hash 抽验（可选） | apply 开始时对 `versionDir` 内文件按下载记录做抽样 MD5（主 exe / 配置类必验），不匹配即 fail 交给 download 重下。注意：apply 阶段没有服务器文件列表，需在 download 时把 hash 清单落盘（`.updator/download_manifest_{version}.json`） |
| 入口 `removeAsideVariants` 时机 | `apply_update.go:250`、`rollback.go:158` 的入口清理改到"新备份就位后"（与 287/212 行成功路径合并），避免崩溃点"旁移后/备份改名前"时误删尚未被覆盖的历史快照 |
| 锁 TTL / 小文件续传 | 维持现状（可用性/效率项），如需调整另行评估 |

---

## 五、修复顺序与回归计划

| 步 | 事项 | 改动量 | 验证入口 |
|----|------|--------|---------|
| 1 | 修复 2（兜底显式写盘 + download 不降级 + check applying 优先 apply） | 小（3 个文件各几行） | 现有单测（`apply_failure_test.go` 两例）+ 模拟器 C 族应转 PASS + E2E S8 |
| 2 | 修复 1（`rollback_target` 字段 + apply 委托恢复 + check 回滚分支 + rollback 早退修复） | 中（新增恢复函数） | 新增 4 现场单测 + 模拟器 B2/B2' 转 PASS + **E2E 新增 S11** |
| 3 | 修复 3（ReadVersion 自愈 + WriteVersion 落盘） | 小 | 新增 4 例自愈单测 + E2E 新增 S12 |
| 4 | 修复 4 阶段一（download 不覆盖 VersionPrevious） | 小 | 模拟器 G 族转 PASS + E2E 全量（S8 目录命名回归） |
| 5 | 修复 4 阶段二（`active_version`） | 中 | 全量 E2E + 模拟器全族回归；version.json 结构兼容检查 |
| 6 | 修复 5（快照配额）+ 加固项 | 中 | 单测 + 手工压测多版本升级 |

**E2E 补测建议**（原报告 S11 扩展）：

- **S11 rollback 中断**：在 rollback 的 7 个阶段注入强杀 × 3 种宿主行为（a. 重跑 `rollback --version`；b. SDK check→apply；c. check→download→apply），断言：最终活动版本 == 回滚目标（或回滚前版本，B2' 放弃场景），且 `version.json` 与磁盘一致、回滚目录名 == 内容。
- **S12 version.json 损坏自愈**：半截/0 字节/有效 `.bak`/双份损坏 4 例，断言 check/apply 不再瘫痪且能恢复到可运行状态。
- **S13 待应用期间发布新版**：下载 V2 未应用 → 发布 V3 → 升级到 V3 后 `list_rollback_versions` 不含 V2、回滚到 V1 内容正确（验证 Bug #2 修复）。

---

## 六、本次复核执行记录

```
# 源码核对（行号证据见 §二）
git log --oneline -5        → HEAD 10b3ab9（仅追加模拟器+报告，8aed31d 代码未变）

# 模拟器可复现性
cd tools/interrupt-sim
python scenarios3.py        → 48 场景，与 results/multi_round_report.txt 逐条一致
python scenarios.py         → 46 场景（不一致/失败 20），与 results/single_round_report.txt 一致
# 生成的 result*.json 与提交的 results/*.json 归一化后逐条相等

# Go 单测基线（GOPATH 模式）
$env:GOPATH = "E:\Project2026\aly\.gopath"; $env:GO111MODULE = "off"
go test ./cmd/ ./config/ ./util/
# 结果：cmd/config/util 通过；2 个环境相关失败与本报告无关：
#   TestRenameDirWithKillCmdCwdOccupiesFolder / TestFindProcessesWithCWDUnder
#   （沙箱内 cmd.exe CWD 探测不生效，非代码缺陷）
```

---

## 七、风险与遗留问题

1. **修复 1 依赖新字段 `rollback_target`**：发布前的存量数据（有 `RollbackPrevious` 无 `RollbackTarget`）采用"安全放弃回滚"策略（恢复到回滚前版本），不会升级、不会丢应用；上线时需在 release notes 说明。
2. **Bug #3 修复后的极端现场**：永久占用者导致 rename 持续失败 → applying 循环无下载逃生口。建议可配置的"连续失败 N 次强制降级 + 告警"（默认保守），需运维确认默认值。
3. **SDK 契约不变**：Bug #5 通过 Go 端 check/apply 行为修复；`CheckUpdateData` 是否暴露 `pending_rollback` 供 UI 展示属第二阶段，**改前必须按仓库 #27 教训在上位机端到端验证**。
4. **`version.json` 字段演进**：阶段二 `active_version` 为可选字段，老文件回退 `VersionPrevious`，无迁移成本；但 `active_version` 与 `VersionPrevious` 并存期需保证两处写入点（apply/rollback 成功）都更新，避免漂移——建议阶段二收敛后逐步废弃对 `VersionPrevious` 的依赖。
5. **模拟器维护**：修复落地后需同步更新 `tools/interrupt-sim/sim.py`（新增 `RollbackTarget`/`ActiveVersion` 字段与恢复分支），保持"模拟器 = 状态机真相"的可信度，否则后续分析会基于过时模型。
