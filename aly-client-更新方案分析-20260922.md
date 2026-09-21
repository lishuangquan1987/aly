# aly 桌面客户端更新方案 —— 代码分析、横向对比与 apply 提速建议（2026-09-22）

分析对象：`github.com/lishuangquan1987/aly`（client / server / publish-cli / C# SDK）
分析版本：`master` @ `7d6c8ce`
核实范围：`client/aly-client/**`、`client/aly-client-sdk/**`、`server/controllers/**`（下载 + 上传）

> **复核说明（第二版）**
> 1. 本版对第一版逐条复核，修正 2 处表述不准确的结论（原 P1-10 版本命名口径、原 P0-1 触发条件），并补充 6 项新发现。
> 2. 文中耗时均为**量级估算**（取决于文件数、进程数、磁盘类型），非实测数据。
> 3. 未覆盖：publish-cli / publish-gui / 服务端业务逻辑（项目、版本、变更日志之外的部分）。

---

## 0. 一句话结论

| 问题 | 结论 |
|---|---|
| 1. 会不会有 bug？ | **会**，且不止一个。有 6 个可复现的确定性 bug（其中 3 个让更新直接失败或静默失败，1 个是供应链 RCE 缺口），另有一批健壮性/一致性缺口。核心设计（目录原子切换 + 状态机 + 崩溃恢复）是对的，问题集中在**参数契约、重复代码导致的行为不一致、异常分支兜底**。 |
| 2. 相对主流方案 | **占用进程处理**与"活动目录名恒定 + 版本快照回滚"是真正的原创亮点，值得被借鉴；**最大差距是包/文件签名校验缺失**（主流方案全都有），其次是二进制差分、服务端 manifest、权限与通道模型。 |
| 3. 乐观重命名（先改名，失败再找占用者） | **方向完全正确，建议采纳**。当前是"每次 apply 无条件全目录扫描 + 可能全进程扫描"，属于先付代价、多半白付。改造后无占用场景可把 apply 从秒级~十秒级压到毫秒级；但必须补三道保险（错误码分类、杀进程白名单、探测结果缓存），否则会引入"无脑重试 + 误杀"。 |

---

## 1. Bug 清单（按严重度分级）

### P0 —— 确定性 bug，可复现

| # | 位置 | 问题 | 后果 |
|---|---|---|---|
| 1 | `AlyApi.cs:95 / :115` ↔ `apply_update.go:26-27`、`rollback.go:17-20` | C# SDK 给 `apply_update` / `rollback` 拼 `--must-close-process-name`，但 Go 端**只定义了 `-main-exe-path` 和 `-close-timeout`** | `flag.ExitOnError` → 打印用法后 `os.Exit(2)`。apply 走 `RunAsyncAlone`（不读输出、不看退出码）→ **宿主拿到 OK，更新其实没执行**；rollback 走 `RunAsync` → 报 "exited with code 2 (may require admin)"，误导排查。<br>**触发条件（复核修正）**：SDK 默认路径 `ApplyUpdateAsync(UpdatorExePath)` 不传该参数，**不会触发**；一旦显式传 `mustCloseProcessNames`（`RollbackAsync` 或业务方调用 apply）**必失败** |
| 2 | `common.go:280-292`（`renameDirWithKill`） | 正式重命名失败后有"杀占用进程"分支，**却没有 explorer 兜底**；而几乎重复的 `renameWithKillRetry`（:327-335）**有**。两份代码行为不一致 | 用户开着资源管理器浏览应用目录时（RM/CWD 常探不到 explorer，作者注释亦已确认该场景），`renameDirWithKill` 每次探测为空 → 不处理 → 5 次重试全失败 → apply 整体失败。**这就是"打开文件夹就更新不了"的直接原因** |
| 3 | `AlyApi.cs:51` | `process.Exited += ...` 用于取消循环，但**没设 `process.EnableRaisingEvents = true`** → 事件永不触发 | 下载进程异常退出（未输出最后一行 `data:null`）时，`ReadLine()` 返回 null 后 while 空转 → **线程 CPU 100% 永久卡死**。正常路径因最后一行 `data:null` 会返回 OK，故只在异常路径暴露 |
| 4 | `explorer_close.go:92`、`shortcut.go:112` | `ioutil.TempFile("", "xxx_*.vbs")`，`*` 占位是 **Go 1.11+** 特性；`BUILD.md` / `build.bat` 要求 **Go 1.10** 构建 | Go 1.10 下随机串拼在**末尾** → 文件名形如 `aly_shortcut_*.vbs87321` → 扩展名不是 .vbs → cscript 拒绝执行 → explorer 精准关闭 + 快捷方式创建**全部静默失效**，只能退化为"杀所有 explorer"。<br>**注**：若实际用 Go ≥1.11 构建则不受影响；需先确认真实构建工具链 |
| 5 | `http_client.go:19` | `http.Client{Timeout: 300s}` 是**覆盖连接 + 读取 body 的整体超时**（续传分支同样受影响） | 单文件下载超过 5 分钟必失败（弱网下的大文件 / 工控机专网同步场景），重试 3 次仍失败 → 整个更新失败 |
| 6 | `file_upload_controller.go:24,112,199` + 全链路 | 服务端上传/分片/合并接口**无任何鉴权**（已核实：无 token、无签名、无中间件）；客户端只校验服务端自报的 MD5/SHA256，**无签名验证** | 任何能访问服务端口的人都可 POST 覆盖任意项目的任意文件（含 `aly-client.exe`、主程序 exe）→ 所有在线客户端自动下载并执行 → **供应链 RCE**。内网/专网不等于安全边界，这是与主流方案最大的差距（Sparkle ed25519、WinSparkle、Velopack、electron-updater 均有签名） |

### P1 —— 健壮性 / 一致性缺口

| # | 位置 | 问题 |
|---|---|---|
| 7 | `common.go:483 applyFailureFallback` | 失败兜底把状态改回 `downloaded`，但崩溃恢复逻辑**只认 `applying`**。若"备份改名成功 + 应用改名失败 + 回滚改名也失败"，主目录已丢失而状态是 downloaded → `check_update` 不再走恢复分支 → **应用目录永久缺失，只能人工重装**。建议：主目录缺失时保持 `applying`，并新增"主目录缺失 + 版本目录存在"的通用恢复分支 |
| 8 | `apply_update.go:130-213` | apply 前**不校验 versionDir 是否存在/完整**。目录被外部删除时，`CopyDirWithExclude(MainFolder → versionDir)` 会用**当前旧版本内容冒充新版本**（`CopyFile` 内 `MkdirAll` 会自动建目录），两次 rename 全部成功 → 标记 `applied`、版本号变成新版本，同时销毁上一版本备份 → **静默假更新**：内容仍是旧的，服务端却认为已升级，之后不再提示。建议 download 结束写 `manifest.json`（清单 + 哈希），apply 前强校验 |
| 9 | `lock.go:24,108-137` | ① `lockTTL=30min` **无条件抢占**：下载超大版本 / 机器睡眠唤醒超过 30 分钟 → 被另一进程抢锁 → 两个更新并发写状态；② `cleanStaleLock` 读到"刚创建尚未写入内容"的空锁文件会判为损坏并删除 → **误删他人新建锁**（TOCTOU 窗口）；③ `IsProcessAlive` 对权限不足一律返回 true（保守），保护进程持锁时永远抢不到 |
| 10 | `config.go:136 AppVersionDir` | `version` 未做字符校验，服务端返回 `..\..\xxx` 可**路径穿越**（版本字符串直接来自服务端响应） |
| 11 | `list_rollback.go:91 isLikelyVersion`（**复核修正**） | 第一版称"与 `stripVPrefix` 口径不一致导致带 V 前缀的版本看不见"——**此说法不准确**：版本目录创建时已 `stripVPrefix`，目录名不含 V。真实问题是 `isLikelyVersion` 只接受**纯数字点分且 ≥2 段**，因此 `1.0.0-beta`、`2026.09.22-rc1` 这类版本号**在回滚列表中不可见**；此外 `rollback --version` 不做 `stripVPrefix`，用户传 `V1.0.0` 会提示"版本不存在" |
| 12 | `download_update.go:48-88` | 先取变更日志拿版本号、再取文件列表，**两次请求非原子**：中途服务端发布新版 → 文件列表与版本号错配 |
| 13 | `file_download_controller.go:50-116` | 每次 `get_all_files` 都**全量重算 MD5 + SHA256**（无 manifest 缓存），大项目秒级~分钟级；同时把服务端**绝对路径**下发给客户端（信息泄露 + 客户端直接拿它拼下载 URL，服务端目录结构一变即失效） |
| 14 | `AlyUpdateClient.cs:180` | 无更新时 `Thread.Sleep(1000)` → **每秒拉起一个 aly-client.exe 进程 + 1~2 次 HTTP**。上位机常年运行，进程创建开销与服务端压力都不小（主流做法：启动时检查 + 15min~数小时间隔，或长轮询/服务端推送） |
| 15 | `apply_update.go:176 launchMainExe` | apply 后由更新器启动新主程序，**旧主程序的退出完全依赖 `must_close_process_name` 配置**。没配 → 新旧两个实例并存；配了但程序有未保存提示 → 一直卡到 `close-timeout`（默认 30s）才强杀 |
| 16 | `download_update.go:94,126`（新增） | 差异判定用**本地文件 MD5 vs 服务端 MD5**，不匹配就下载覆盖 → 用户/现场在机器上手工改过的配置、脚本会被**静默覆盖丢失**（工控机现场改配置的典型场景）。建议对 `un_copy_files` 或新增 `preserve_files` 白名单内的文件跳过覆盖 |

### P2 —— 次要

| # | 位置 | 问题 |
|---|---|---|
| 17 | `apply_update.go:141-155` + `applyReplacement:211` | apply 重试 3 次，**每次都重跑整目录 `CopyDirWithExclude`**。大项目单次复制几十秒~几分钟 → 3 次复制让总耗时成倍放大。建议首次复制完成后记录状态，重试时复用 |
| 18 | `common.go:384 nextAsideName` + `apply_update.go:261` | 旁移目录命名是在 `to` 后再拼 `.old`：对已经是 `X.old` 的路径会生成 `X.old.old` / `X.old.old.1`；而清理只删 `X.old` → **这些残留目录永不回收**（磁盘空间缓慢泄漏） |
| 19 | `check_update.go:166 findLatestLog` | 按 change log 的 **ID 最大**取版本，而非版本号最大。服务端若支持"回滚发布"（重发旧版本号），ID 最大的是回滚记录 → 客户端取到旧版本 → 随后 download 判定 "already at latest version"，行为困惑 |
| 20 | `handle_scan.go:136 queryHandleTable` | 扩容循环 `for { ... continue }` 无重试上限：若 `retLen` 不增长会**无限循环** |
| 21 | `AlyUpdateClient.cs:54-62` | 构造函数内 `Task.Factory.StartNew` + `.Result` 阻塞线程池线程；事件回调在线程池线程触发，**未做 UI 同步上下文封送** → WPF 宿主需自行 `Dispatcher.Invoke`，否则跨线程异常 |
| 22 | `AlyUpdateClient.cs:76 UpdateSelf` | 自更新在构造时立即执行：若上一次 apply 拉起的更新器尚未退出，`File.Replace` 失败 → 3 次重试（共 ~3s）后放弃，留下旧更新器 |
| 23 | `findDeepHolders` 全系统句柄枚举 | 单次可达 10s–60s（handle.exe 同原理），而 apply 重试 3 次 × 3 次 rename → 最坏触发 9 次深扫 |
| 24 | `closeExplorerWindows` 兜底 | 精准关闭未命中时**杀全部 explorer** → 关掉用户所有资源管理器窗口、打断进行中的复制粘贴（审查报告 #13 已提出，代码尚未加白名单） |

---

## 2. 与主流开源更新方案的对比

参照：Squirrel.Windows / **Velopack**（Squirrel 精神续作）、WinSparkle / NetSparkle（Sparkle 的 Windows 移植）、macOS Sparkle、electron-updater、ClickOnce/MSIX、Omaha（Chrome）。

### 2.1 值得被借鉴的三个原创点

| 亮点 | 说明 | 主流方案现状 |
|---|---|---|
| **① 三级占用进程探测 + 主动清理** | Restart Manager 句柄持有者 → PEB/CWD 持有者 → 全系统句柄枚举（`NtQuerySystemInformation` + `DuplicateHandle` + `NtQueryObject`，handle.exe 原理）。主流方案遇到"文件被占用"基本是提示用户关闭或交给安装器；aly 是**无人值守强制解决** | 对上位机/工控机（现场无人、必须自动升级且要重启主程序）是刚需，Squirrel / WinSparkle 没有等价能力 |
| **② 活动目录名恒定 + 版本快照回滚** | `ApplicationFolder`（活动）↔ `ApplicationFolder_V1`（快照）双向 rename。目录**路径不变** → 快捷方式、防火墙规则、文件关联、计划任务、URI 协议全部无需重建；回滚是反向 rename，比"重新解包上一版本包"快一个数量级 | Velopack / Squirrel 保留旧包但回滚需重新解包；Omaha 用版本目录 + 注册表指向，路径会变 |
| **③ 面向断电/崩溃的状态机** | `version.json` 原子写 + `downloaded/applying/applied` 三态 + 旁移代替删除（不提前删 `X.old`）+ 崩溃恢复分支 + 失败兜底启动旧 exe | 与 Velopack 同级，比 WinSparkle（交给安装器）更可控；缺陷见 P1-7（状态降级导致恢复盲区） |

### 2.2 建议向主流借鉴的六件事（按性价比排序）

1. **包/文件签名校验（最高优先级）**：发布侧 ed25519/RSA 签名 manifest，客户端内置公钥验签；强制 HTTPS。同时**给服务端加鉴权**（P0-6）。当前"无鉴权接口 + HTTP + 服务端自报哈希"在 MITM 或内网恶意方前等于零防护。
2. **服务端 manifest 缓存**：发布时生成 `manifest.json`（每文件 size/md5/sha256），`get_all_files` 直接返回，避免每次全量哈希；客户端也省掉每次 `LocalFileMD5Map` 全目录 MD5（`check_diff` / `download_update` 都在算）。
3. **二进制差分（delta）**：现在只做"文件级差异"（整文件重下）。bsdiff / VCDIFF（Velopack delta、electron-updater blockmap）对几百 MB 的主程序 exe 能省 90%+ 带宽。工控机常走专网/4G，收益很大。
4. **更新后校验与自动回滚**：新增 `pending_verify` 态 —— apply 后由主程序启动成功调用 `confirm`，超时/崩溃未确认则自动回滚到快照。Velopack 有 install hook，Chrome 有版本验证；aly 已有快照目录，加这层只需改状态机。
5. **权限与安装位置模型**：装在 `Program Files` 时 apply 需要管理员。需明确 UAC 提权方案或改用 per-user 目录（Squirrel / Velopack 默认 `%LocalAppData%`），否则 rename 权限失败会落进"重试 5 次全失败"。
6. **通道/灰度 + 遥测**：`stable/beta` channel（Squirrel `--channel`、Sparkle `sparkle:channel`）、灰度百分比、更新成功率上报。现在只有 `force_update` 一个开关。

> **架构层面的建议**：客户端为兼容 XP 固定 `GOARCH=386`，由此带来 PEB 偏移硬编码、读不了 64 位进程 CWD、`handle.exe` 级深扫等一整套复杂度。若 XP 已不是硬约束，**改用 64 位构建可直接消除 CWD 探测盲区与深扫的大部分必要性**，整体代码量与不确定性都会显著下降。

---

## 3. 关于"apply-update 先重命名，失败再找占用者"（问题 3）

### 3.1 现状：代价付在前面，而且多半白付

`applyReplacement`（`apply_update.go:184-192`）一开始就**无条件**执行：

```go
closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout) // 必要：按名字关主程序
closeProcessesHoldingFolder(fc.MainFolder, closeTimeout)               // ← 无条件全目录扫描
```

`closeProcessesHoldingFolder → findHoldersOf → FindProcessesHoldingPath`（`process_restartmanager.go:121`）会把**目录下所有文件逐个注册进 Restart Manager**（`filepath.Walk` + 每 400 个一批 `RmRegisterResources`）；RM 没找到还会追加 `FindProcessesWithCWDUnder`（`process.go:222`：枚举全进程表 + 逐进程 `OpenProcess` + `NtQueryInformationProcess` + 多次 `NtReadVirtualMemory`）。

耗时量级（取决于文件数、进程数、磁盘）：

| 探测手段 | 量级 | 触发条件 |
|---|---|---|
| RM 全目录注册 | 数千文件时 ~1–3s（机械盘更久） | **每次 apply 无条件执行** |
| 全进程 CWD/PEB 扫描 | 上百进程时 ~0.5–2s | RM 未发现持有者时**自动追加** |
| 全系统句柄深扫 | **10s–60s**（handle.exe 同级） | rename 失败第 3 轮起，且可重复触发 |
| cscript 关 explorer（explorer 兜底） | 单次 ~0.2–0.5s | 每次重试，最多 5 次 × 2 个目录 |

而绝大多数现场**没有任何占用者**（主程序已按名字优雅关闭）—— 这笔钱 100% 白付。

### 3.2 建议的改造（乐观重命名）

核心原则：**按名字关闭 = 业务必需，留在复制前；按句柄探测 = 补救手段，只在 rename 真正失败时做。**

```go
func applyReplacement(...) error {
    // 保留：must_close_process_name 里的主程序必须先关，
    // 否则下一步 CopyDirWithExclude 读文件就可能失败
    if len(fc.ExeCfg.MustCloseProcessName) > 0 {
        closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout)
    }
    // 删除：closeProcessesHoldingFolder(fc.MainFolder, closeTimeout) —— 不再无条件扫描

    // ...复制、旁移、两次 rename（renameDirWithKill 内部已是"失败才探测"）...
}
```

配套四点：

1. **探测时机后移反而更准确**：复制阶段耗时可能几十秒~几分钟，复制前探到的占用者在 rename 时早已失效（或期间又新开了进程）。把探测推到 rename 前一刻，消除这个 TOCTOU 窗口。
2. **给 `renameDirWithKill` 补 explorer 兜底，或直接复用 `renameWithKillRetry`**：这两个函数 ~90% 重复、行为却不一致（一个带 explorer 兜底一个不带），是 P0-2 的根因。合并成一个即可。
3. **探测结果缓存**：一次 apply 要做 3 次 rename（旁移 / 备份改名 / 应用改名），现在每次都重新全量扫描，最坏 9 次深扫。用 `killed map[uint32]bool` 或"已探测路径集合"在本次 apply 内复用。
4. **只对存在的路径探测**：`findHoldersOf(from, to)` 里的 `to`（如 `X.old`）通常不存在，注册不存在的资源纯属浪费。

### 3.3 必须补的三道保险（否则乐观策略会退化成"无脑重试 + 误杀"）

**① 按 Windows 错误码分类，只对"占用类"错误重试**

| 错误 | 含义 | 处理 |
|---|---|---|
| `ERROR_SHARING_VIOLATION(32)` / `ERROR_LOCK_VIOLATION(33)` / `ERROR_USER_MAPPED_FILE(1224)` / `ERROR_ACCESS_DENIED(5)` | 被占用 | 探测 → 杀 → 重试 |
| `ERROR_NOT_SAME_DEVICE(17)` | 跨卷（挂载点 / 符号链接） | **立即失败**，重试无意义 |
| `ERROR_DIR_NOT_EMPTY(145)` / `ERROR_ALREADY_EXISTS(183)` | 目标已存在 | 走旁移逻辑，不是占用 |
| `ERROR_PATH_NOT_FOUND(3)` | 源不存在 | 立即失败 |

当前代码对**任何**错误都执行"探测 → 杀 → 重试"，跨卷/权限类错误会白白强杀一批进程。**这是乐观模式下必须补的第一道闸。**

**② 杀进程白名单**：只杀 `must_close_process_name` + 主程序 exe + `explorer / cmd / conhost` 这类已知宿主；`devenv.exe`、杀毒软件、正在写日志的服务等宁可失败并回报给用户，别静默强杀（审查报告 #13 已提，未落地）。

**③ 保留"应用可用性"红线**：杀进程前确认主程序已按名字关闭；整个 apply 加总超时（例如 90s）并在超时后走"回滚 + 启动旧版"，绝不让现场停在"目录已改名、新目录没就位"的中间态。

### 3.4 顺带一提：rollback 已经是"乐观模式"

`rollback.go` **没有** `closeProcessesHoldingFolder`，只做按名字关闭，然后直接 `renameDirWithKill`（内部失败才探测）。也就是说：回滚路径已经实践了问题 3 的思路，只是 apply 路径多背了一次前置扫描。**把 apply 对齐到 rollback 的行为，改动很小、收益明确。**

---

## 4. 建议的修复优先级

| 顺序 | 事项 | 说明 |
|---|---|---|
| 1 | P0-1 参数契约 | Go 端补 `--must-close-process-name`（或 SDK 不再传），并让 `apply_update` 失败时输出可被宿主识别的 JSON —— 一天内可闭环 |
| 2 | P0-2 合并两个 rename 函数 + 补 explorer 兜底 | 直接消除"开着文件夹就更新失败" |
| 3 | P0-3 `EnableRaisingEvents = true` | 同时把 `ReadLine()` 返回 null 当作流结束退出循环 |
| 4 | P0-6 服务端鉴权 + 包签名 | 安全主线，工作量最大但优先级最高 |
| 5 | P0-4 / P0-5 | 确认构建工具链（Go 版本）；下载超时改为仅限制响应头超时 |
| 6 | P1-7 / P1-8 状态机与 manifest 校验 | 消灭应用卡死与静默假更新 |
| 7 | 问题 3 乐观重命名 + 错误码分类 + 白名单 | 性能与误杀一起解决 |
| 8 | P1-14 / P1-13 | 轮询间隔可配置（默认 15min）；服务端 manifest 缓存 |
