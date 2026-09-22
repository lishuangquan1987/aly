# aly 客户端 bug 清单核实与修复计划（2026-09-22）

> 本文针对《aly-client-更新方案分析-20260922.md》中的 bug 清单逐条核实：
> **1、2、3、4、7、18、19、20、21、22、23** 是否真实存在，并给出可执行的修复计划。
> 核实基于当前工作区代码（master），每条都给出**代码位置、触发条件、后果**与**修复步骤**。
>
> **结论先行**：11 项全部核实**存在**。其中 #2、#4、#23 已随"乐观重命名"与"服务端缓存"改造一并修复；
> 其余 8 项（#1/#3/#7/#18/#19/#20/#21/#22）已按本文修复计划全部实施。

---

## 总览

| # | 严重度 | 核实结果 | 状态 | 一句话 |
|---|--------|----------|------|--------|
| 1 | P0 | ✅ 存在 | ✅ 已修 | SDK 传 `--must-close-process-name`，Go 端未定义该 flag → `os.Exit(2)` |
| 2 | P0 | ✅ 存在 | ✅ 已修 | `renameDirWithKill` 缺 explorer 兜底，与 `renameWithKillRetry` 行为不一致 |
| 3 | P0 | ✅ 存在 | ✅ 已修 | C# `EnableRaisingEvents` 未设置 → 异常退出时线程 100% 卡死 |
| 4 | P0 | ✅ 存在 | ✅ 已修 | Go 1.10 `TempFile` 不支持 `*` 占位符 → `.vbs` 扩展名失效 |
| 7 | P1 | ✅ 存在 | ✅ 已修 | 失败兜底状态降级 `downloaded`，崩溃恢复只认 `applying` |
| 18 | P2 | ✅ 存在 | ✅ 已修 | 旁移命名 `X.old.old` / `X.old.old.1` 永不回收 |
| 19 | P2 | ✅ 存在 | ✅ 已修 | `findLatestLog` 按 ID 最大而非版本号最大取最新 |
| 20 | P2 | ✅ 存在 | ✅ 已修 | `queryHandleTable` 扩容循环无重试上限 |
| 21 | P2 | ✅ 存在 | ✅ 已修 | SDK 事件回调无线程封送，宿主需自行 Dispatcher |
| 22 | P2 | ✅ 存在 | ✅ 已修 | 自更新在构造时立即执行，更新器未退出则 3 次重试后放弃 |
| 23 | P2 | ✅ 存在 | ✅ 已修 | 深扫 10s-60s，apply 最坏触发 9 次深扫 |

---

## #1（P0）SDK 传 `--must-close-process-name`，Go 端未定义该 flag

### 核实

- **位置**：`AlyApi.cs:95`（`ApplyUpdateAsync`）、`AlyApi.cs:115`（`RollbackAsync`）。
- **行为**：SDK 拼接命令行参数时，只要 `mustCloseProcessNames` 非空就追加
  `--must-close-process-name "..."`。
- **对照 Go 端**：
  - `apply_update.go:26-27` 只注册 `-main-exe-path`、`-close-timeout`；
  - `rollback.go:18-20` 只注册 `-version`、`-main-exe-path`、`-close-timeout`；
  - 均使用 `flag.ExitOnError` → 遇到未定义 flag 打印用法后 `os.Exit(2)`。
- **触发条件**：SDK 默认 `ApplyUpdateAsync(UpdatorExePath)` 不传该参数 → 不触发；
  一旦显式传 `mustCloseProcessNames`（`RollbackAsync` 默认也走同一拼接）→ **必失败**。
- **后果**：
  - `apply` 走 `RunAsyncAlone`（`AlyApi.cs:191-211`，`Process.Start` 后即返回 OK，
    不读 stdout、不看退出码）→ 宿主拿到 OK，但更新**实际未执行**（静默失败）；
  - `rollback` 走 `RunAsync`（读取退出码 2）→ 报 "exited with code 2 (may require admin)"，
    误导排查方向。

### 修复计划（推荐方案 A）

**方案 A（推荐，改动在 Go 端，向后兼容）**：

1. `apply_update.go` 与 `rollback.go` 各增加 flag：
   ```go
   mustCloseFlag := fs.String("must-close-process-name", "", "comma separated process names to close")
   ```
2. 解析后合并进 `fc.ExeCfg.MustCloseProcessName`：
   ```go
   if *mustCloseFlag != "" {
       for _, n := range strings.Split(*mustCloseFlag, ",") {
           n = strings.TrimSpace(n)
           if n != "" { fc.ExeCfg.MustCloseProcessName = append(fc.ExeCfg.MustCloseProcessName, n) }
       }
   }
   ```
3. `applyReplacement` / `Rollback` 里已有 `closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, ...)`，
   合并后无需改动调用点。
4. **附带加固**：`RunAsyncAlone` 无法感知失败，建议 `apply_update` 失败时把错误也
   `AppendToLog` 到 `UpdateFolder/update.log`（当前已做），并在 README 注明
   "apply 为异步执行，成功与否以 update.log 为准"。

**方案 B（C# 端不传）**：C# SDK 不再传 `--must-close-process-name`，靠 `client.json`
配置。缺点：已暴露的 `ApplyUpdateAsync(alyExePath, mustCloseProcessNames)` API 语义改变，
不推荐。

> 建议实施顺序：方案 A 第 1、2 步为必改（消除静默失败）；第 3、4 步为加固。

---

## #2（P0）`renameDirWithKill` 缺 explorer 兜底 —— **已修**

### 核实（改造前代码）

- **位置**：`common.go`（改造前）`renameDirWithKill`（251-296 行）与
  `renameWithKillRetry`（300-339 行）。
- **差异**：
  - `renameWithKillRetry` 失败分支在 `len(pids) == 0` 时调用 `closeExplorerWindows(timeout, from, to)`
    （explorer 兜底）；
  - `renameDirWithKill` 失败分支**没有**该兜底——只有 `findHoldersOf`/`findDeepHolders` + `ForceKillPIDs`。
- **后果**：用户开着资源管理器浏览应用目录时（Explorer 持目录句柄，RM/CWD 探不到），
  `renameDirWithKill` 每次探测为空 → 5 次重试全失败 → apply 整体失败。
  **"打开文件夹就更新不了"的直接原因**。

### 修复（已实施）

将两个函数合并为统一的 `renameDirWithKillState` / `renameWithKillRetryState`，
共享 `renameProbeState`，失败分支统一为：
`浅扫（RM+CWD）→ 第 3 轮起深扫（句柄枚举）→ 无持有者时 closeExplorerWindows 兜底`。
`apply_update` 与 `rollback` 的三次 rename 均走该统一实现，行为完全一致。
同时删除已无引用的 `closeProcessesHoldingFolder`（死代码）。

---

## #3（P0）C# `EnableRaisingEvents` 未设置

### 核实

- **位置**：`AlyApi.cs:47-51`：
  ```csharp
  using (var process = new Process { StartInfo = psi })
  {
      process.Start();
      process.Exited += (s, e) => cts.Cancel();   // ← 从未设置 EnableRaisingEvents
      while (!cts.IsCancellationRequested) {
          var stdoutTask = process.StandardOutput.ReadLine();
          if (!string.IsNullOrEmpty(stdoutTask)) { ... }
      }
  }
  ```
- **行为**：`Process.Exited` 事件**默认不触发**，必须 `EnableRaisingEvents = true`。
- **触发条件**：下载进程异常退出且未输出最后一行 `data:null`（`result.Data == null`）时，
  `ReadLine()` 返回 null → `IsNullOrEmpty(null)` 为 true → `if` 块跳过、循环不退出 →
  `cts` 永不 Cancel → **while 空转、线程 CPU 100% 永久卡死**。
  正常路径因最后一行 `data:null` 会 `return OK`，故只在异常路径暴露。

### 修复计划

1. `process.EnableRaisingEvents = true;`（放在 `process.Start()` 之后、注册事件之前均可）。
2. **双保险**：`while` 循环内把 `ReadLine() == null` 当作流结束：
   ```csharp
   var line = process.StandardOutput.ReadLine();
   if (line == null) break;      // 进程退出、流结束
   ```
   这样即使事件机制失效也不会空转。
3. `while` 循环后保留 `return AlyResponse.NG("Download interrupted")` 作为兜底返回。

> 注意：C# SDK 目标是 `net40;netstandard2.0`，`EnableRaisingEvents` 两目标都支持。

---

## #4（P0）Go 1.10 的 `TempFile` 不支持 `*` 占位符 —— **已修**

### 核实

- **位置**：`explorer_close.go:92`、`shortcut.go:112`：
  ```go
  f, err := ioutil.TempFile("", "aly_closeexplorer_*.vbs")
  ```
- **根因（源码级确认）**：查 Go 1.10 源码 `src/io/ioutil/tempfile.go`（`go1.10` tag）：
  `TempFile(dir, prefix)` 直接 `prefix + nextSuffix()`，**没有** `*` 替换逻辑
  （该特性 Go 1.11 才引入）。Go 1.10 下生成文件名形如 `aly_closeexplorer_*.vbs87321` →
  **扩展名是 `.vbs87321` 而非 `.vbs`** → cscript 拒绝执行 → explorer 精准关闭与
  快捷方式处理**全部静默失效**，只能退化为"杀全部 explorer"。
- **前提**：实际构建工具链确为 Go 1.10（`BUILD.md` 明确要求，GOPATH 模式、GOARCH=386）。
  若实际用 ≥1.11 构建则不受影响。

### 修复（已实施）

改为 `ioutil.TempFile("", "aly_closeexplorer_")`（无 `*`）生成唯一临时名，
再手动补 `.vbs` 后缀并移除占位文件：
```go
f, err := ioutil.TempFile("", "aly_closeexplorer_")
if err != nil { ... }
vbsPath := f.Name() + ".vbs"
f.Close()
os.Remove(f.Name()) // 移除占位文件，使用带 .vbs 后缀的路径
```
`explorer_close.go` 与 `shortcut.go` 两处同步修改。Go 1.10 与更高版本均兼容。

---

## #7（P1）失败兜底状态降级导致恢复盲区

### 核实

- **位置**：`common.go` `applyFailureFallback`（483-494 行，改造前编号）：
  ```go
  func applyFailureFallback(fc *FullConfig, versionInfo *config.VersionInfo, failErr error) string {
      if versionInfo != nil {
          versionInfo.VersionStatus = config.VersionStatusDownloaded   // ← 降级
          config.WriteVersion(versionInfo)
      }
      launchMainExeFn(fc.ExeCfg, fc.MainFolder)
      return failErr.Error()
  }
  ```
- **对照**：`apply_update.go:58-117`、`rollback.go:91-120` 的崩溃恢复分支
  **只在 `VersionStatusApplying` 下执行**。
- **场景**（最坏）：
  1. 备份改名（MainFolder → prevVersionDir）成功；
  2. 应用改名（versionDir → MainFolder）失败；
  3. 回滚改名（prevVersionDir → MainFolder）也失败；
  → MainFolder 已丢失、状态被 `applyFailureFallback` 写成 `downloaded`。
  4. 下次 `check_update` 走 `checkUpdatePending`（downloaded 分支），**不进入**
     崩溃恢复分支 → 应用目录永久缺失，只能人工重装。

### 修复计划

1. `applyFailureFallback` 增加判断：**若 MainFolder 缺失且对应版本目录存在**，
   **保持 `applying` 状态不降级**（仅记日志），让下次 apply/check 走崩溃恢复分支完成
   `versionDir → MainFolder` 重命名：
   ```go
   func applyFailureFallback(fc *FullConfig, versionInfo *config.VersionInfo, failErr error) string {
       if versionInfo != nil {
           mainGone := false
           if _, err := os.Stat(fc.MainFolder); os.IsNotExist(err) {
               mainGone = true
           }
           if mainGone {
               if vd, vErr := fc.ExeCfg.AppVersionDir(versionInfo.Version); vErr == nil {
                   if _, err2 := os.Stat(vd); err2 == nil {
                       // 主目录缺失 + 版本目录存在：保持 applying，让崩溃恢复分支修复
                       util.AppendToLog(logDir(), "update.log",
                           fmt.Sprintf("apply failed with main folder missing, keep applying for crash recovery: %v", failErr))
                       launchMainExeFn(fc.ExeCfg, fc.MainFolder) // 尽力启动旧 exe（可能失败）
                       return failErr.Error()
                   }
               }
           }
           versionInfo.VersionStatus = config.VersionStatusDownloaded
           config.WriteVersion(versionInfo)
       }
       launchMainExeFn(fc.ExeCfg, fc.MainFolder)
       return failErr.Error()
   }
   ```
2. （可选加固）`check_update` 的 `checkUpdatePending` 中补充提示：若 MainFolder 缺失
   且版本目录存在，输出 "need apply" 让宿主继续调用 apply（崩溃恢复）而非报错。
3. （可选加固）`ApplyUpdate` 崩溃恢复分支增加"MainFolder 缺失 + 版本目录存在"的
   通用恢复（当前已覆盖 applying 场景，保持 applying 即可命中）。

> 风险提示：改动会改变失败后的状态语义（失败不再一律回 downloaded）。需回归
> `apply_failure_test.go` 的 `TestApplyFailureFallback`（该测试是 MainFolder 存在场景，
> 应保持 downloaded 降级不变）。

---

## #18（P2）旁移目录 `X.old.old` 永不回收

### 核实

- **位置**：`common.go` `nextAsideName`（384-392 行）+ `apply_update.go:261`、`rollback.go:208`。
- **根因**：
  - `nextAsideName(to)` 对 `to` 直接拼 `.old` / `.old.1`；
  - `apply_update.go:221` `oldBackupTemp := prevVersionDir + ".old"`；
  - 当 `to` 本身已是 `X.old`（`RemoveAll(oldBackupTemp)` 失败或被占用导致残留时），
    旁移出 `X.old.old`；
  - 清理只 `RemoveAll(oldBackupTemp)`（即 `X.old`），**`X.old.old` / `X.old.old.1`
    永不回收** → 磁盘空间缓慢泄漏。
- **触发场景**：上一次更新崩溃留下 `X.old`，且本次 `RemoveAll` 失败（文件被占用），
  或 `oldBackupTemp` 与旁移目标恰好冲突。

### 修复计划（组合方案）

1. **命名收敛**（治本）：`nextAsideName` 剥离已有 `.old` 后缀再命名，杜绝 `.old.old` 链：
   ```go
   // 只保留一个 .old 层：X.old / X.old.1 / X.old.2 ...
   func nextAsideName(to string) string {
       base := strings.TrimSuffix(to, ".old")
       aside := base + ".old"
       for i := 1; ; i++ {
           if _, err := os.Stat(aside); os.IsNotExist(err) {
               return aside
           }
           aside = fmt.Sprintf("%s.old.%d", base, i)
       }
   }
   ```
2. **清理变体**（治标）：`applyReplacement` / `Rollback` 成功后，除 `oldBackupTemp`
   外一并清理其旁移变体：
   ```go
   // 清理 X.old、X.old.1、X.old.2 ...（按命名规则枚举删除）
   func removeAsideVariants(base string) {
       for _, p := range []string{base + ".old"} {
           os.RemoveAll(p)
       }
       for i := 1; i < 100; i++ {
           p := fmt.Sprintf("%s.old.%d", base, i)
           if _, err := os.Stat(p); os.IsNotExist(err) {
               break
           }
           os.RemoveAll(p)
       }
   }
   ```
3. 在 apply/rollback 入口与成功路径调用 `removeAsideVariants(prevVersionDir)`。

> 1+2 缺一不可：只收敛命名会漏掉历史残留；只清理变体仍会短暂生成链式名。

---

## #19（P2）`findLatestLog` 按 ID 最大取版本

### 核实

- **位置**：`check_update.go:166-174` `findLatestLog`：
  ```go
  func findLatestLog(logs []model.ProjectChangeLog) model.ProjectChangeLog {
      latest := logs[0]
      for i := 1; i < len(logs); i++ {
          if logs[i].ID > latest.ID { latest = logs[i] }   // ← 按 ID
      }
      return latest
  }
  ```
- **同款内联**：`download_update.go:58-63`、`check_diff.go:48-53` 也是 ID 比较。
- **后果**：服务端"回滚发布"（重发旧版本号生成新 changelog 记录）时，ID 最大的记录
  是回滚记录 → 客户端取到**旧版本** → `download_update` 判定 "already at latest version"，
  行为困惑，且与 `needUpdate`（按版本号比较）口径不一致。

### 修复计划

1. 提取统一 helper（三处共用）：
   ```go
   // findLatestLog 取版本号最大者；版本相同则取 ID 最大（最新记录优先）。
   func findLatestLog(logs []model.ProjectChangeLog) model.ProjectChangeLog {
       latest := logs[0]
       for i := 1; i < len(logs); i++ {
           c := compareVersion(stripVPrefix(logs[i].Version), stripVPrefix(latest.Version))
           if c > 0 || (c == 0 && logs[i].ID > latest.ID) {
               latest = logs[i]
           }
       }
       return latest
   }
   ```
2. `download_update.go`、`check_diff.go` 的内联循环改为调用 `findLatestLog(logs)`。
3. 注意 `compareVersion` 对带后缀版本（`1.0.0-beta`）按非数字段视为 0，行为与
   `isLikelyVersion`/`stripVPrefix` 口径一致；如需严格语义比较可另行增强，
   但至少保证"版本号优先于 ID"。

---

## #20（P2）`queryHandleTable` 扩容循环无上限

### 核实

- **位置**：`handle_scan.go:136-155`：
  ```go
  for {
      r, _, _ := procNtQuerySysInfo.Call(...)
      if r == scanStatusInfoLengthMismatch {
          capLen = int(retLen) + 65536
          buf = make([]byte, capLen)
          continue        // ← 无重试上限
      }
      ...
  }
  ```
- **行为**：正常时 `retLen` 会增长并成功；但若系统句柄表异常导致 `retLen` 不增长
  （或持续返回 mismatch），将**无限循环**并不断分配更大缓冲区 → 挂死/内存膨胀。
- **触发条件**：极端/异常系统状态（句柄表持续增长、驱动异常）。非日常路径，但属
  确定性健壮性缺口。

### 修复计划

1. 增加最大尝试次数与缓冲区上限：
   ```go
   const maxRetries = 16
   const maxCap = 1 << 30 // 1GB 上限
   for attempt := 0; attempt < maxRetries; attempt++ {
       r, _, _ := procNtQuerySysInfo.Call(...)
       if r == scanStatusInfoLengthMismatch {
           if int(retLen) > maxCap { return nil, 0, fmt.Errorf("handle table too large") }
           capLen = int(retLen) + 65536
           if capLen > maxCap { capLen = maxCap }
           buf = make([]byte, capLen)
           continue
       }
       ...
   }
   return nil, 0, fmt.Errorf("NtQuerySystemInformation buffer growth did not converge")
   ```
2. 调用方（`FindProcessesWithHandlesUnder`）对错误按"探测失败"降级处理（返回 nil，
   走其它兜底），避免因深扫失败阻塞整个 apply。

---

## #21（P2）SDK 事件回调无线程封送

### 核实

- **位置**：`AlyUpdateClient.cs:54-62`（构造函数）、`:253-279`（`OnStatusChanged`/`OnError`）。
- **行为**：
  - 构造函数 `Task.Factory.StartNew` 启动后台循环；`MainLoop` 内多次 `.Result`
    （`CheckUpdateAsync(...).Result`、`DownloadUpdateAsync(...).Result`）阻塞线程池线程；
  - `StatusChanged` / `ErrorStatusChanged` / `RequestDownloadUpdate` / `RequestApplyUpdate`
    均在后台线程触发，**未做 UI 同步上下文（SynchronizationContext）封送**。
- **后果**：WPF 宿主必须自行 `Dispatcher.Invoke`，否则跨线程更新 UI 抛异常；
  未封送时 UI 线程安全性完全依赖宿主自觉。属设计缺口（非确定性崩溃）。

### 修复计划

1. 构造函数捕获 `SynchronizationContext.Current`（宿主若在 UI 线程创建实例则拿到 WPF 上下文）：
   ```csharp
   private readonly SynchronizationContext _syncContext;
   public AlyUpdateClient(string updatorExePath = null) {
       _syncContext = SynchronizationContext.Current;
       ...
   }
   ```
2. 事件触发点统一封送：
   ```csharp
   private void Raise(Action a) {
       if (_syncContext != null) { _syncContext.Post(_ => a(), null); }
       else { a(); }
   }
   // OnStatusChanged/OnError 内部改用 Raise(() => handler(s, msg))
   ```
3. 文档明确：非 UI 线程创建实例时 SDK 不做封送（保持行为一致，宿主自行处理）。
4. （可选）`MainLoop` 内 `.Result` 改为 `await ... .ConfigureAwait(false)`，避免阻塞
   线程池线程（netstandard2.0 可用；net40 需注意 async/await 支持，可用
   `Task.ContinueWith` 等价改造）。

---

## #22（P2）自更新在构造时立即执行，更新器未退出则放弃

### 核实

- **位置**：`AlyUpdateClient.cs:54-62`（构造函数）、`:76-133`（`UpdateSelf`）。
- **行为**：构造函数后台任务先 `CheckSelfUpdateAsync(...).Result`，需要更新就立即
  `UpdateSelf()`；`UpdateSelf` 用 `File.Replace` 原子替换更新器自身，**目标被占用时
  重试 3 次（间隔 1s，共约 3s）后放弃**，留下旧更新器。
- **触发条件**：上一次 apply 拉起的更新器进程尚未完全退出（还在运行/句柄未释放），
  或杀毒软件短时锁定 exe。
- **后果**：自更新静默失败（留痕日志但不致命），旧更新器延续；若旧更新器版本过旧
  可能在下一次更新时带来旧行为。

### 修复计划

1. `UpdateSelf` 前先检查更新器进程是否仍在运行：
   ```csharp
   // 等待上一次 aly-client.exe 进程退出（最多 10s），再替换自身
   for (int i = 0; i < 10; i++) {
       if (!IsProcessRunning("aly-client")) break;
       Thread.Sleep(1000);
   }
   ```
2. 重试策略改为**指数退避**（1s、2s、4s、8s、16s），总时长放宽到约 30s；
   或把自更新延迟到 `MainLoop` 第一轮 `check_update` 之后（此时更新器已启动稳定）。
3. 失败时保留留痕（不清空目标），下轮轮询（`check_self_update`）再试。
4. （可选）与全局更新锁（`AcquireUpdateLock`）串行化，避免与 download/apply/rollback
   并发替换更新器。

---

## #23（P2）深扫 10s-60s，apply 最坏触发 9 次深扫 —— **已修**

### 核实（改造前代码）

- **位置**：`renameDirWithKill` / `renameWithKillRetry` 失败分支从第 3 轮起调用
  `findDeepHolders`（全系统句柄枚举，handle.exe 原理，10s-60s）。
- **最坏路径**：apply 重试 3 次 × 每次 3 次 rename（旁移/备份/应用）→ 最坏 9 次深扫；
  加上每轮最多 3 次深扫尝试，实际更糟。

### 修复（已实施）

1. **乐观重命名**：`applyReplacement` 不再无条件 `closeProcessesHoldingFolder`
   全目录预扫描；先直接 `os.Rename`，只有失败才探测——无占用场景**零探测**，
   耗时从秒~十秒级压到毫秒级。
2. **深扫路径级去重**：`renameProbeState.deepPaths` 保证同一路径在本次 apply/rollback
   内最多深扫一次（3 次 rename 共享状态）。
3. **击杀去重**：`killedPids` 避免重复击杀同一 PID。
4. **只探测存在的路径**：`probeHoldersState` 跳过尚不存在的目标路径（如 `X.old`），
   避免注册不存在的资源。
5. 浅扫（RM + CWD）**不缓存**——占用者随时间变化，每次失败重试都重新探测，
   满足"再失败，再查杀"。

---

## 已实施的修复（本会话代码改动清单）

1. **服务端按项目缓存 get_all_files（任务 2）**
   - 新增 `server/internal/service/file_cache.go`：
     `GetProjectFileList` / `InvalidateProjectFileList` / `IsUploadTempPath` / `MatchIgnoreFile`。
   - 缓存策略：按项目名缓存 `[]models.FileInfo`（含 md5/sha256）；
     **upload 成功即失效**（`UploadFile` / `UploadChunk` / `UploadChunkComplete`），
     也覆盖 `UpdateProject`（忽略规则变化影响列表）与 `DeleteProject`；
     下次 `get_all_files` 重新扫描缓存。附带"仅 stat 指纹"比对（相对路径+大小+mtime），
     磁盘文件被外部改动时自动重建，确保缓存始终最新。
   - `file_download_controller.go` 的 `GetAllFilesByProjectName` / `DownloadFile`
     改用 service 方法；删除 controller 内重复的 `matchIgnoreFile` / `isUploadTempPath`。
   - 测试：`server/internal/service/file_cache_test.go`（缓存命中/失效重建/忽略规则/目录缺失）。

2. **apply_update 乐观重命名（任务 3）**
   - `common.go`：新增 `renameProbeState` / `isRenameRetryableErr` / `probeHoldersState` /
     `deepScanCandidates`；`renameDirWithKill` + `renameWithKillRetry` 合并为
     `renameDirWithKillState` / `renameWithKillRetryState`（统一 explorer 兜底、错误码分类）。
   - `apply_update.go`：删除无条件预扫描；三次 rename 共享 `st`；apply 重试 3 次也共享。
   - `rollback.go`：三次 rename 共享 `st`。
   - 测试：`cmd/rename_optimistic_test.go`（错误码分类、探测状态、深扫去重、乐观路径）。

3. **修复 #4**：`explorer_close.go` / `shortcut.go` 的 `TempFile` `*` 占位符问题。

## #24 追加修复（2026-09-22 实测触发，用户桌面/任务栏黑屏事故）

### 事故回放
在真实桌面环境运行"explorer 打开子文件夹"占用测试时，触发 `closeExplorerWindows`
的兜底逻辑**强杀全部 explorer**（`ForceKillPIDs`），导致用户桌面（Progman/WorkerW）
与任务栏（Shell_TrayWnd）黑屏；且 Windows 对被杀掉的 shell 不会自动重启，
黑屏持续到手动重启 explorer。

### 根因（两层）
1. **VBS 精准关闭只做完全相等匹配**（`LCase(path) = target`）：用户打开的是目标目录的
   子文件夹（浏览 `C:\app\config` 而目标是 `C:\app`）时匹配不上，`closed=0`，
   调用方误以为"没有 explorer 窗口占用"，进而走兜底杀全部 explorer。
2. **兜底杀全部 explorer**（`closeExplorerWindows` 中 `ForceKillPIDs`）：explorer 是
   shell 进程（桌面/任务栏/开始菜单都靠它），强杀必然黑屏。

### 修复（已实施）
1. `util/explorer_close.go`：VBS 匹配改为**前缀匹配**——目标目录本身或目标目录的
   任意子目录（`LCase(path) = target Or Left(LCase(path) & "\", Len(target)+1) = target & "\"`），
   子文件夹窗口也能被精准关闭，避免触发兜底。
2. `common.go closeExplorerWindows`：兜底从 `ForceKillPIDs` 强杀改为 **WM_CLOSE 优雅
   关闭**——explorer 的 shell 窗口会忽略 WM_CLOSE，只有文件窗口被关闭，**绝不杀 shell**。

### 测试（已补充）
- `TestCloseExplorerWindowsSubfolderPrefixMatch`：验证前缀匹配（closed>=1）且 explorer 进程存活。
- `TestRenameDirWithKillExplorerHoldsSubfolder`：场景 1 增强断言——rename 成功后 explorer
  仍存活（防杀光 shell 回归）。
- 注意：`explorer.exe <dir>` 会启动独立文件浏览实例（shell 保持不动），窗口关闭后该实例
  延迟退出属正常；测试只保证不会把所有 explorer 杀光。

## 修复实施记录（2026-09-22 全部完成）

| 顺序 | 事项 | 工作量 | 风险 |
|------|------|--------|------|
| 1 | #1 参数契约（Go 端补 flag） | 小 | 低，消除静默失败 |
| 2 | #3 C# `EnableRaisingEvents` + null 判流 | 极小 | 低 |
| 3 | #7 失败兜底保持 applying | 小 | 中（需回归 fallback 测试） |
| 4 | #19 统一按版本号取最新 | 小 | 中（影响 check/download/check_diff 三处口径） |
| 5 | #20 句柄表查询上限 | 小 | 低 |
| 6 | #18 旁移命名收敛 + 变体清理 | 中 | 低 |
| 7 | #21 SDK 线程封送 | 中 | 低（仅 SDK，不影响 Go 端） |
| 8 | #22 自更新串行化/退避 | 中 | 低（仅 SDK） |

## 附：与报告不一致/需注意的补充说明

- 报告提到 #23 "最坏 9 次深扫"；实际改造前最坏路径为 **3 次 apply 重试 × 3 次
  rename × 每轮最多 3 次深扫尝试 ≈ 27 次**，比报告更严重。改造后深扫按路径去重，
  同一路径一次 apply 最多 1 次。
- #4 的成立依赖"Go 1.10 构建"前提；若项目实际用 Go ≥1.11 构建，该 bug 不触发，
  但修复向后兼容无害，建议保留。
- #21、#22 属 C# SDK 侧设计/健壮性缺口，不影响 Go 更新器本体；修复不改变
  `AlyApi` 公共 API 签名，宿主代码可无感升级。

## #25 追加修复：apply_update 误触发 Windows 关机（2026-09-22 实测报告）

### 事故现象

用户在真实环境执行 `apply_update` 时，Windows 弹出“**系统将在 60 秒内关机**”的
关机选项（可取消的关机倒计时）。

### 根因（两条独立链路，均为“替换目录前清理占用者”引入）

1. **强杀系统关键进程**
   `probeHoldersState` 的深扫（`NtQuerySystemInformation` + 全系统句柄枚举）会
   把持有目标目录/子目录句柄的 `csrss` / `winlogon` / `services` / `lsass` 等
   系统关键进程也列为“占用者”（这些进程确实会持有卷/目录句柄，例如
   `C:\Windows\Prefetch`、工作目录、DLL 所在目录等）。
   `ForceKillPIDs` 随后直接 `TerminateProcess`：Windows 判定关键进程被终止，
   立即启动关机倒计时（部分进程被杀直接蓝屏 0x000000F4）。
   风险面更大的一点：`--must-close-process-name` 由配置/命令行传入，一旦误配成
   `csrss.exe` 之类，`closeProcessesGracefully` 同样会走到强杀。

2. **向 explorer 的 shell 窗口发送 WM_CLOSE**
   `closeExplorerWindows` 的兜底逻辑调用 `SendCloseMessageToProcess(explorer)`，
   该函数枚举 explorer 的**全部可见顶层窗口**——其中包含 shell 桌面/任务栏窗口
   （`Shell_TrayWnd` / `Progman` / `WorkerW`）。
   `#24` 当时的判断是“shell 窗口会忽略 WM_CLOSE”，实测并不成立：向 shell 窗口
   发送 WM_CLOSE 可能被解释为“退出 shell / 结束会话”，从而弹出手机关机或注销提示。

### 修复（2026-09-22，commit 见仓库）

**多重闸门：任何 PID 在真正 TerminateProcess / SendMessage 之前都必须通过白名单过滤。**

1. `util/process.go` 新增 `criticalProcessNames`（小写去 `.exe`）：
   `system` / `registry` / `idle` / `secure system` / `smss` / `csrss` / `wininit` /
   `winlogon` / `services` / `lsass` / `lsaiso` / `svchost` / `fontdrvhost` / `dwm` /
   `sihost` / `ctfmon` / `taskhostw` / `runtimebroker` / `shellexperiencehost` /
   `startmenuexperiencehost` / `searchhost` / `textinputhost` / `searchindexer` /
   `spoolsv` / `audiodg` / `msmpeng` / `securityhealthservice` / `windefend` / `nissrv`。
2. 新增 `shellProcessNames`（`explorer`）：**允许** WM_CLOSE 关闭其文件窗口，
   **绝不**允许强杀（强杀 shell 黑屏，`#24` 回归）。
3. 新增 `FilterKillablePIDs(pids) (killable, blocked)`，保护：
   PID 0 / PID 4 / 当前进程自身 / 关键进程 / shell / **进程名无法识别（保守保护）**。
   接入点：
   - `probeHoldersState`：探测结果先过滤，被保护的记录
     `probe skipped protected holders: [...]`；
   - `ForceKillPIDs`、`KillPIDsAndWait`：入口过滤，被保护的写 stderr；
   - `KillProcess`：**最后一道兜底**，受保护对象直接返回
     `拒绝结束受保护进程 %d`。
4. `SendCloseMessageToProcess` 发送前用 `isCriticalProcess` 过滤（给 winlogon 发
   WM_CLOSE 会注销/关机）。
5. 新增 `SendCloseMessageToExplorer`：**只**对 `explorerFileWindowClasses`
   （`CabinetWClass` / `ExploreWClass`）发 WM_CLOSE，刻意排除 shell 窗口类；
   `closeExplorerWindows` 兜底改用它。
6. `util/process_unix.go` 补齐 `SendCloseMessageToExplorer` / `FilterKillablePIDs` 桩。

### 测试

- `util/process_protect_test.go`（新增 6 例）：
  - `TestFilterKillablePIDsProtectsCritical`：`csrss`/`winlogon`/`services`/`lsass` 一律被保护；
  - `TestFilterKillablePIDsProtectsSelfAndSystemPids`：PID 0/4/自身被保护；
  - `TestFilterKillablePIDsProtectsExplorer`：explorer 被保护（不可强杀）；
  - `TestFilterKillablePIDsAllowsNormalProcess`：普通进程可杀；
  - `TestIsCriticalProcess`：关键进程判定（explorer 不算“关键”，允许发 WM_CLOSE）；
  - `TestExplorerFileWindowClassesExcludesShellWindows`：
    白名单只有 `CabinetWClass`/`ExploreWClass`，不含 `Shell_TrayWnd`/`Progman`/`WorkerW`。
- client 全量测试通过：`ok aly/client/aly-client/cmd`、`ok aly/client/aly-client/util`。
- E2E `client/aly-client/test/e2e/update_e2e.ps1`：115/115 PASS（含 S2 explorer 占用、
  S8 各更新阶段中断、S9 断电半写文件恢复）。

### 复查结论（是否还有别的关机路径）

对 client 全量 grep 了 `InitiateSystemShutdown` / `ExitWindowsEx` /
`NtShutdownSystem` / `shutdown.exe` / `taskkill` / `RmShutdown` / `RmRestart`：
**均无使用**。客户端能影响系统的只有两类调用：

| 调用 | 位置 | 现状 |
|------|------|------|
| `TerminateProcess` | `util/process.go` | 三重白名单过滤（探测 / 批量杀 / 单杀兜底） |
| `SendMessage(WM_CLOSE)` | `util/process.go` | 关键进程过滤 + explorer 仅文件窗口 |
| Restart Manager | `util/process_restartmanager.go` | 只用 `RmStartSession`/`RmRegisterResources`/`RmGetList`/`RmEndSession`（**只查不关**） |

因此 `apply_update` 已无触发 Windows 关机的路径。
