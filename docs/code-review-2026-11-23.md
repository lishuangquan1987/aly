# Aly 项目全面代码审查报告（第二版，逐条人工复核）

> 审查范围：server（Gin+Ent+SQLite）、client/aly-client（Go 1.10/XP 更新器）、SDK、publish-cli、publish-gui-go、spec、CI。
> 第一版由分模块并行分析产出；第二版对每条结论做了源码逐行复核，**推翻 1 条、修正细节 2 条、降级 2 条**。
> 日期：2026-11-23（v2）

## 相对第一版的修正

| 结论 | 变化 | 原因 |
|------|------|------|
| 并发发布版本号竞态回退 | **推翻** | `SetMaxOpenConns(1)` 使所有事务串行，"先 V3 后 V2 打回 V2"不可能发生（详见下文） |
| SDK 在 FAIL 行杀进程 | 细节修正 | 仅 GoSDK 杀进程；CSharpSDK 是提前返回，客户端会因管道写满**挂死**（更糟） |
| version.json 自愈写竞态 | 降级为轻微 | WriteVersion 原子写，读方看不到半截内容，触发窗口极窄 |
| manifest.tmp 幻影循环 | 降级为轻微 | 不是循环，单轮自愈（删除同步会清掉） |
| UpdateFolder 半成品永久跳过 | 降级改述 | 仅 ensureUpdatersFor 路径静默跳过；显式重跑初始化会补写 client.json |

---

## 总体评价

这是一个**被多轮审查打磨过的、工程质量明显高于平均水准**的项目：

- 路径安全多层防线（项目名校验 → 落点断言 → 前缀断言）且有回归测试；
- 原子写（tmp + rename）在服务端、staging、GUI 配置、version.json 全链路贯彻；
- aly-client 的崩溃恢复矩阵（ApplyStage 状态机、熔断防活锁、resumeRollback 分支恢复）达到工业级；
- 协议单一来源（spec/schema + alyprotocol 生成 9 种语言 SDK + fixtures + 148 个 E2E 用例）名副其实；
- 注释即文档：大量"审查 Mxx/Bxx"留痕，bug 成因与修复理由写在现场。

**剩余风险集中在一处：单请求内的原子性做得极好，跨请求/跨进程的状态一致性（发布两阶段、多发布者并发、客户端与 SDK 的行为级协议）缺少整体设计。**

---

## 一、复核后成立的最高优先级问题

### 1.【server】上传并发竞态会产出损坏的正式文件 ✅成立
- `file_upload_controller.go:212-216`：`UploadChunk` 写分片不持 `lockForFile`、非原子；`:274-276` 的 `UploadChunkComplete` 持锁合并。同一文件"分片重传 vs 合并中读取"并发时，`SaveUploadedFile` 的 O_TRUNC 截断正在被读取的分片 → 损坏数据经原子 rename 成为正式文件。
- `file_upload_controller.go:106-115`：`UploadFile` 不持锁，且与 Complete 共用同一 `.merging` 临时名，同样交错损坏。
- 触发前提：两个发布者同时操作同一项目（或 unusual 重试时序）。单人使用不触发。
- **修法**：两处也取 `lockForFile(absFileName)`；临时名改 `os.CreateTemp` 唯一名。

> **🔍 审核（2026-11-23 逐行对源码）：存在**
> 三处引用全部属实：UploadChunk 写分片（:212-216）确无 `lockForFile`，Complete 持锁（:274-276）
> 后逐片 `os.Open` 读取（:336）；UploadFile（:106-115）无锁且与 Complete 共用固定
> `absFileName+".merging"` 临时名（:106 vs :323）。**一处精度修正**：分片路径的"损坏直达现网"
> 会被 Complete 的整文件 MD5 比对（:361；publish-cli 始终上报期望 MD5，client.go:394-416）拦下——
> 不符即拒并保留分片；真正可穿透的是 **UploadFile 直传路径**：无锁、无 MD5 校验、与 Complete
> 共享同一 `.merging`，交错后经 rename 直达现网。结论：问题成立，修法正确且必要。

### 2.【全局】发布是"两阶段非原子"：文件先覆盖现网，版本最后才建 ✅成立
- `push.go:95-187`：阶段 1 并发上传（服务端 rename 直接落最终路径），阶段 2 `PublishVersion`。中途失败（网络断、防倒退拒绝）时服务端留下"一半新一半旧"的文件集；已开始更新流程的老客户端会按文件拉到混合版本。重推同版本号可补齐，但窗口内现网不一致。
- **修法**：服务端按版本暂存（`data/{project}/.staging/{version}/`）+ PublishVersion 时切换清单；短期至少文档声明该窗口。

> **🔍 审核：存在**
> push.go 结构属实：阶段 1 并发上传（服务端逐文件原子 rename 直接落最终路径），阶段 2 才
> `PublishVersion`；中途失败即留下混合文件集，无版本级暂存或回滚。"同版本重推可补齐"的缓解
> 也属实（missing-chunks 探测支持续传）。架构级缺口成立，版本暂存方案方向正确。

### 3.【SDK × client】SDK 击穿"单文件失败不整体失败"协议 ✅成立（细节已修正）
- 客户端设计（`download_update.go:349-376`）：单文件 3 次重试失败 → FAIL 进度行（`isSuccess:false`，`common.go:409-422`）→ 继续下载 → 最后聚合输出 `FailedFiles` 信封。
- GoSDK `api.go:312-314`：见到任意 `isSuccess:false` 行 → **Kill 进程** → 聚合清单永远到不了宿主。
- CSharpSDK `AlyApi.cs:135-137`：见到即 **return NG**（不 Kill）→ 客户端进程继续跑，但 stdout 无消费者、管道写满后 aly-client **永久阻塞挂死**。
- **修法**：FAIL 进度行改 `isSuccess:true + status:"FAIL"`（进度行只承载信息，终态信封表达成败），走 spec → dto.go → alyprotocol 重新生成流程，并更新 host-contract.md。

> **🔍 审核：存在**
> 三个引用逐一确认：客户端 `printProgressFail`（common.go:408-424）输出 `isSuccess:false` 的
> 进度行、末尾聚合 `failedFiles`；GoSDK api.go:312-318 任一行 `!IsSuccess` 即 `Process.Kill()`
> + return NG；CSharpSDK AlyApi.cs:135-138 同条件 `return NG` 但不 Kill——客户端继续写 stdout，
> 管道写满后必然阻塞。修法方向正确，需按 spec 单一来源流程三语言同步。

### 4.【gui-go】批量 add/reset 单命令行传全部路径 ✅成立
- `cli.go:251-263`：`append([]string{"add", "--"}, files...)` 一次 exec。Windows CreateProcess 命令行上限 32767 字符，全选几千个文件直接失败且报错晦涩。
- **修法**：分批（每批 200 个）或 CLI 支持 `add --stdin-paths`。

> **🔍 审核：存在**
> cli.go `addFiles`/`resetFiles` 确为 `append([]string{"add","--"}, files...)` 单次 exec；
> Windows CreateProcess 32k 字符上限是系统事实，全选大目录必然触发，报错出自 CLI 参数解析
> 确实晦涩。GUI 侧分批循环实现最简单，建议采纳。

---

## 二、复核后成立的中等问题

| # | 模块 | 问题 | 位置 | 建议 | 🔍 复核 |
|---|------|------|------|------|------|
| 5 | server | JSON 接口无请求体上限，超大 logs 数组全量读入并无界入库 | version_controller.go:31、project_controller.go | JSON 路由组挂 1MB MaxBytesReader；logs 限条数/长度 | **存在**（:31 ShouldBindJSON 无 MaxBytesReader——分片路由装了、JSON 组没装；logs 无条数/长度限制直入库） |
| 6 | server | manifest 在事务外写，失败仅记日志，DB 与清单可不一致且调用方无感知 | project_service.go:300-311 | 响应带回 `manifestWritten:false`；或清单生成前置到发布校验阶段 | **存在**（逐行吻合：事务提交后 :307-311 才写清单，失败仅 log，响应不含写清单结果） |
| 7 | server | get_all_files 缓存重建每文件读两遍（MD5、SHA256 各一次全量 IO）且在请求路径同步执行 | file_cache.go:185-192 | `io.MultiWriter` 单遍双哈希；singleflight 合并并发重建 | **存在**（:185-189 两次独立全量读；指纹不符即在请求路径内同步重建，无 singleflight） |
| 8 | server | 软删除项目永久占用项目名：create 查重**不过滤** is_deleted（与 get_project_by_name 口径相反），data 目录永不回收 | project_controller.go:58、project_service.go:479-490 | 统一口径：改名归档允许复用，或提供恢复接口 | **存在**（:58 仅 NameEQ 无 is_deleted 过滤，与 get_project_by_name 口径相反属实） |
| 9 | server | `SetMaxOpenConns(1)` 使 WAL 并发归零，长事务阻塞全部 API（含每分片一次的 checkUploadProject 查库）⚠️ | db.go:73 | **与 #11 绑定处理**（见下文警告） | **存在**（db.go:73 原样；checkUploadProject 每请求查库属实） |
| 10 | publish-cli | staging Add/Verify 仍用 HashFile 双哈希（scanner 注释明示 SHA256 无消费者），add --all 全流程数据读三遍 | staging/staging.go:119,179 | 改 HashFileMD5 或复用 status 已算的 MD5 | **存在**（staging.go 两处均 diff.HashFile；读三遍=status 扫描+add 哈希+verify 复核，成立） |
| 11 | publish | 多发布者文件级交叉覆盖：两人同时 push 同一项目，同名文件按完成顺序互相覆盖，现网成逐文件混合体 ✅（文件层成立；**版本号竞态部分已被推翻**，见下文） | push.go:95-143 | 项目维度互斥或 publish_version 带 baseVersion 乐观校验 | **存在（按修订后表述）**（除 per-file complete 锁外无项目级发布互斥，last-writer-wins 属实；版本号竞态因单连接串行化确认不存在） |
| 12 | client | `isRenameRetryableErr` 硬编码 Windows errno（32/33/1224/5），common.go 无构建标签；Linux 下 5=EIO、32=EPIPE 被误判"占用"→ 探测+杀进程+重试 | cmd/common.go:670-685 | 构建标签拆分 errno 表；影响 linux-amd64 部署链 | **存在**（common.go:680 字面量 switch，cmd 目录无任何 //go:build，Linux 更新链在发布范围内） |
| 13 | SDK/协议 | 依赖 `"already at latest version"` 人类文案做机器判定（GoSDK client.go:177、C#/TS 同） | 各 SDK | Output 增加机器码字段进 schema | **存在**（GoSDK client.go:177、C# AlyUpdateClient.cs:303、TS client.ts:158 三处文本匹配实锤） |
| 14 | publish | shared.json 及 `.updator/shared.json.tmp`、`.corrupted` 备份（含明文 server_url）会随版本包下发（scanner 只排除 staging/ 与 publish.json） | diff/scanner.go:36-59 | 排除规则改".updator/ 下仅放行 shared.json" | **部分存在**（shared.json 随包下发是**有意设计**——注释与文档均明示客户端靠它拿 server_url，放行规则反转会破坏协议；`.tmp`/`.corrupted` 残留确不在排除列表、会被打包，该半条成立。修法应只针对残留文件） |
| 15 | gui-go | UpdateFolder 半成品（有目录缺 client.json）被 ensureUpdatersFor 静默跳过（显式重跑初始化可修复，非"永久"） | updatefolder.go:145-167 | ensureUpdatersFor 同时检查 client.json | **存在**（:155-157 仅 Stat 目录存在即 return，不查 client.json） |
| 16 | publish-cli | 扫描无缓存，GUI 每次刷新全量 walk+MD5；哈希中 ctx 取消不生效（只在 goroutine 开始前检查） | diff/scanner.go | size+mtime→md5 缓存；按读取块检查 ctx | **存在**（goroutine 内仅开始前一次 select ctx，HashFileMD5 全程不查；无任何缓存层） |
| 17 | 模型漂移 | GUI 的 ChangeLog 缺 `AfterApplyUpdateScript` 字段（服务端/CLI 已支持）；模型四处手工重复 | gui-go/cli.go:41-47 | 抽公共 Go package | **存在**（GUI ChangeLog 仅 ID/Project/Version/Logs/Time，服务端返回的 AfterApplyUpdateScript 被静默丢弃） |

### ⚠️ 关于 #9 和 #11 的绑定警告（第一版结论的重要修正）

第一版同时建议"拆分读写连接池提升并发"（M5）和"防倒退校验有并发竞态"（S2）。**这两条不能同时成立，也不能分开采纳：**

- 现状 `MaxOpenConns(1)` 意味着所有事务在同一连接上排队，`PublishVersion` 的"读全部 changelog → 比较 → SetVersion"天然串行——**当前不存在版本号竞态**，防倒退保护是完备的。
- 一旦按 M5 拆池（写 1 读 N），读池上的事务不再与写互斥，"两人并发 push，版本回退"的竞态**立即出现**。
- 结论：若做并发优化，必须先（或同时）给发布加项目级互斥（`BEGIN IMMEDIATE` 写事务、或 Project 行带锁读、或 publish_version 带 baseVersion 校验）。**不要只做其一。**

---

## 三、轻微问题（复核成立）

- server：`lockForFile` map 只增不减；`checkDiskSpace` 失败静默放行无日志；walk 单文件不可读导致整个 get_all_files 失败；`NGWithError` 把内部 DB/OS 错误原样回给客户端；版本号数字段 Atoi 溢出按 0（只会让版本"变小"被拒，不破坏防倒退）；上传文件路径未校验 Windows 保留名/冒号/尾点（项目名校验了，文件名没校验）；`relativeFileName` 尾 `/` 造出与目录同名的文件形成自锁；`ReadTimeout=60s` 对 64MB 分片偏紧；防倒退校验全量读 changelog（事务串行下不致竞态，但长期项目越查越慢）；`.updator_manifest.json.tmp` 残留会被 get_all_files 下发一次（单轮自愈，非循环）；`ioutil.WriteFile` 弃用。
- client：差异检测只比 MD5（已有 FileSize 前置，实际构成 size+MD5 双因子，建议注释说明）；`check_self_update` 未 EvalSymlinks；`cleanStaleLock` 读失败即删锁；`runScript` Kill 不杀孙进程（建议 `taskkill /T /F`）；version.json 自愈写竞态（极窄窗口，见修正说明）；CSharpSDK SelfUpdateAsync 30s 超时对大文件偏短；`GetAllFiles` 全量清单在大项目下体量大。
- publish：dry-run 在 --json 下无输出；`chunckSizeEnviron` 拼写；push 只报第一个失败文件；GUI 120s/30min 硬超时对大版本不够且切 tab 静默取消；GUI 配置目录兜底 TempDir。

### 三·逐条复核（轻微问题）

**server**

| 条目 | 结论 | 依据 |
|------|------|------|
| lockForFile 只增不减 | 存在 | upload_common.go:42-52 懒创建、无删除路径 |
| checkDiskSpace 失败静默放行无日志 | 存在 | :80-83 注释写"仅记录"但无任何 log 调用 |
| walk 单文件不可读整体失败 | 存在 | file_cache.go:120 Walk 错误向上传播（:75/:96/:170） |
| NGWithError 泄露内部错误 | 存在 | models.go 直接 `ErrorMsg: err.Error()` |
| 版本号 Atoi 溢出按 0 | 存在 | project_service.go:383/386、440/441 忽略错误（后果确如所述：只变小被拒） |
| 上传文件名未校验保留名/冒号/尾点 | 存在 | resolveUploadTarget 仅做前缀断言 |
| 尾 `/` 造出同名文件自锁 | **部分存在** | 尾斜杠经 Clean 落到目录路径，后续写/删报错；Windows 不可能创建与目录同名的文件，"自锁"不成立，实际后果是错误提示混乱 |
| ReadTimeout=60s 对 64MB 分片偏紧 | 存在 | main.go `ReadTimeout: 60 * time.Second`（64MB/60s≈11Mbps，慢链路可触发） |
| 防倒退校验全量读 changelog | 存在 | 事务内 `All(ctx)` 无上限（project_service.go:265） |
| `.updator_manifest.json.tmp` 残留下发一次 | 存在 | file_cache.go:141 仅精确匹配清单名，tmp 兄弟文件不排除（单轮自愈描述准确） |
| ioutil.WriteFile 弃用 | **不存在** | server 全目录 grep 无 `ioutil.WriteFile`（client 侧存在，但该条归属 server 组） |

**client**

| 条目 | 结论 | 依据 |
|------|------|------|
| 差异检测只比 MD5 | **部分存在**（按其自注成立） | download_update.go:207-208 比对阶段仅比 MD5；"FileSize 前置"自注与下载后 MD5+SHA256 双校验的事实相符 |
| check_self_update 未 EvalSymlinks | 存在 | selfupdate 目录 grep 无 EvalSymlinks |
| cleanStaleLock 读失败即删锁 | 存在 | 读 err 分支直接 `os.Remove + return true`（JSON 损坏分支有 1 分钟在途保护，读失败分支没有） |
| runScript Kill 不杀孙进程 | 存在 | common.go:1201 仅 `cmd.Process.Kill()`，无 taskkill /T 或进程组 |
| version.json 自愈写竞态 | **部分存在** | 写路径确为 tmp+Sync+rename 原子；"竞态"仅是极窄理论窗口，与降级描述一致 |
| CSharpSDK SelfUpdateAsync 30s 超时偏短 | 存在 | AlyApi.cs:289 `DefaultRunTimeoutSeconds = 30`，self_update 含全量 SHA256 比对+复制 |
| GetAllFiles 全量清单体量大 | 存在（设计取舍） | 路由无分页/增量协商 |

**publish**

| 条目 | 结论 | 依据 |
|------|------|------|
| dry-run 在 --json 下无输出 | 存在 | push.go:85-93 仅 printHumanLn 后 return，jsonOutput 分支无输出 |
| chunckSizeEnviron 拼写 | 存在 | client.go:268-269（chunck → chunk） |
| push 只报第一个失败文件 | 存在 | 仅 firstErr/failedPath 记首个 |
| GUI 120s/30min 硬超时且切 tab 静默取消 | 存在 | app_detail.go:100（120s）/:306（30min）；openSelected 对 in-flight 上传直接 cancel |
| GUI 配置目录兜底 TempDir | 存在 | config.go:73 `os.TempDir()/AlyPublish` |

---

## 四、信任模型声明（建议写进文档，不一定要改代码）

- MD5/SHA256 与文件同信道获取，**防损坏不防主动中间人**；`after_apply_update_script` = 服务器完全受信模型。与"无鉴权是有意设计"一致，但应在 README/spec 显式声明。
- 若将来要防主动攻击，最低成本方案：publish-cli 发布时对清单做 Ed25519 签名（Go 1.10 支持），公钥烧进 client.json。

---

## 五、值得保持的优点（复核确认）

1. version.json 的 tmp+Sync+rename 与 ApplyStage 持久化状态机；
2. 熔断（MaxApplyFailCount=3）防"杀主程序死循环"；
3. 分片上传 missing-chunks 探测 + complete 端边合并边验 MD5 + perFileLock（覆盖了 Complete 路径，缺口仅在 #1）；
4. DisableKeepAlives 应对中间设备断链、transient 错误重试矩阵；
5. ignore 规则跨平台语义（任意层级、大小写不敏感、不跨目录）+ 回归测试；
6. spec/fixtures/-check 的协议防漂移机制与 CI 红灯；
7. `-print-routes` 文档防漂移、`go:debug httpservecontentmaxranges` 这类深挖到运行时行为的修复；
8. 单连接 SQLite 意外提供了发布事务的串行化保护（见 #9/#11 警告）——这是"缺陷即特性"的例子，优化时务必保留等价语义。

## 六、建议的行动顺序

1. **一周内**：#1（上传锁）、#4（命令行分批）——改动小、收益直接；
2. **短期**：#3 协议 FAIL 行语义（走 spec 流程，三个 SDK 同步改）、#5 JSON 限制、#12 Linux errno、#10/#7 哈希单遍化；
3. **规划级**：#2 按版本暂存 + #11/#9 发布互斥（绑定设计，先写设计文档）；
4. **持续**：#17 模型收敛为公共 package，防止 GUI/CLI/服务端继续漂移。
