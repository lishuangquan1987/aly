# 版本暂存与发布互斥 设计文档（审查 #2 / #9 / #11 绑定方案）

- 日期：2026-11-23
- 来源：`docs/code-review-2026-11-23.md` #2（两阶段非原子发布）、#9（单连接串行化，与 #11 绑定）、#11（多发布者文件级交叉覆盖）
- 状态：设计稿（未实施）
- 审核结论基线：
  - #2 成立：`push.go` 阶段 1 并发上传（服务端逐文件原子 rename 直达现网路径），阶段 2 才 `PublishVersion`；中途失败留下"一半新一半旧"的文件集。
  - #11 成立（修订表述）：除 per-file complete 锁外无项目级发布互斥，同名文件 last-writer-wins；**版本号竞态已被推翻**——`SetMaxOpenConns(1)` 使所有事务在同一连接排队，防倒退校验天然串行、当前完备。
  - #9 成立：`MaxOpenConns(1)` 使 WAL 并发归零，长事务阻塞全部 API。**若拆连接池优化并发，必须同时加发布互斥，不能只做其一**（否则防倒退竞态立即出现）。

---

## 1. 问题回顾

### 1.1 现网混合窗口（#2）

发布分两阶段，且两阶段之间没有任何原子性：

```
push（publish-cli）
 ├─ 阶段 1：并发 UploadFile / UploadChunk+Complete
 │    服务端把每个文件原子 rename 到 data/{project}/{relPath}  ← 立即可被客户端拉到
 └─ 阶段 2：PublishVersion
      事务里防倒退校验 + Project.Version 更新 + changelog 创建
      事务外写 .updator_manifest.json
```

中途失败（网络断、防倒退拒绝、进程被杀）时：

- 客户端 `check_update` 仍认为当前是旧版本 V_n；
- 但 `get_all_files` 返回的文件集已是"部分 V_{n+1}"；
- 老客户端按文件清单做差异比对 → 拉到**混合版本**并 apply。
- 缓解：同版本号重推可补齐（分片续传按 MD5 找 missing），但失败到重推之间现网一直不一致。

### 1.2 多发布者交叉覆盖（#11）

两人同时 push 同一项目（比如都基于 V_n 的工作副本，各自改了不同文件、发布了 V_{n+1} / V_{n+2}）：

- 文件层：同名文件按各自完成顺序互相覆盖，最终现网是两人改动的逐文件混合体；
- 版本层：防倒退保护只保证 changelog 里版本号递增，**不能**保证"V_{n+2} 的文件集 = 发布者意图的完整快照"——后发布者可能只上传了自己的差异文件，覆盖了先发布者的同名文件，且从未感知对方的存在。

### 1.3 并发优化与互斥的绑定（#9）

`db.go: SetMaxOpenConns(1)` 的副作用是所有事务串行，这**意外地**保证了 `PublishVersion` 防倒退校验的原子性。任何把连接池拆成"写 1 + 读 N"的优化都会破坏这个保证。因此：

> **铁律：连接池拆分（#9 优化）与发布互斥（#11 修复）必须同一变更落地。只做其一 = 引入版本号竞态。**

---

## 2. 目标与非目标

### 目标

1. **发布原子性**：任一时刻，`get_all_files` 返回的文件集要么是完整旧版本，要么是完整新版本，不存在混合窗口。
2. **项目级发布互斥**：同一项目同时只允许一个 push 流程；第二个 push 者要么排队、要么被明确拒绝。
3. **乐观版本校验**：publish_version 带 `baseVersion`，服务端校验"当前已发布版本 == baseVersion"，不匹配即拒绝——防止"基于过时认知发布"。
4. **保留 #9 优化通路**：方案落地后，连接池拆分才被允许（作为后续独立变更）。

### 非目标

- 不做版本历史快照/回滚到任意历史版本（现有 rollback 基于客户端本地版本目录，不受影响）。
- 不改变客户端协议（check/download/get_all_files 的客户端侧行为不变；服务端内部布局变化对客户端透明）。
- 不引入外部依赖（不加 Redis/etcd；互斥用 SQLite/文件锁实现）。
- 不处理"上传到一半取消"的断点续传之外的上传状态机改造（staging 目录天然支持重推）。

---

## 3. 方案总览

三件事，一次设计，分两期落地：

| 期 | 内容 | 对应问题 |
|---|---|---|
| 第一期（本设计核心） | A. 服务端版本暂存目录 + 原子切换；B. publish_version 带 baseVersion 乐观校验 | #2、#11 的"基于过时认知"部分 |
| 第二期（可选优化） | C. 项目级发布互斥 + 连接池拆分（绑定变更） | #11 的并发覆盖、#9 |

第一期落地后，#2 消除；#11 的危害从"静默混合"降级为"baseVersion 校验失败、显式报错"。第二期再解决多发布者并发的体验问题与 API 并发性能。

---

## 4. 第一期设计

### 4.1 服务端版本暂存目录

**目录布局**（新增 staging 层，对客户端不可见）：

```
data/{project}/                     ← 现网（get_all_files 只看这里）
data/.staging/{project}/{version}/  ← 发布暂存（上传落位处）
```

关键点：

- staging 放在 `data/.staging/` 而不是 `data/{project}/.staging/`：
  - `get_all_files` 的 walk 根是 `data/{project}`，放外面零侵入——不需要改任何过滤逻辑，staging 天然不被下发；
  - 防止 `.staging` 与项目文件命名冲突，也避免 `ValidateFileNameSegment` 白名单要为一个内部目录开口子。
- `{version}` 目录名需过版本号校验（复用 `validVersion` 字符集白名单），防路径注入。

**上传流程变化**（服务端）：

1. `upload_file` / `upload_chunk` / `upload_chunk_complete` 新增可选参数 `stagingVersion`；
2. 带 `stagingVersion` 时，目标路径从 `data/{project}/{relPath}` 改为 `data/.staging/{project}/{version}/{relPath}`（同样经过 `resolveUploadTarget` 的白名单校验、per-file 锁、唯一临时名 + rename 原子落位）；
3. 不带 `stagingVersion` 时行为完全不变（兼容旧 publish-cli 直发）。

**publish_version 变化**（服务端）：

```
PublishVersion(project, version, logs, ..., staging=bool)
  1. 开事务（BEGIN IMMEDIATE 语义，见 4.3）：
     a. 防倒退校验（现状保留）
     b. 新增：staging 版本目录必须存在且非空 → 不存在则拒绝"没有暂存内容"
     c. Project.Version 更新 + changelog 创建（现状保留）
  2. 事务提交后：
     a. 逐文件把 data/.staging/{project}/{version}/ 的内容
        rename 到 data/{project}/ 对应路径（同盘 rename，原子）
        — rename 前先对现网同名文件做 per-file 锁
     b. 重建 .updator_manifest.json（现状逻辑）
     c. 删除 data/.staging/{project}/{version}/
```

**为什么切换放在事务后而不是事务内**：文件系统操作无法参与 SQLite 事务。窗口分析：

- 切换期间（rename 循环执行中，通常秒级）客户端可能拉到部分新文件——但**每个 rename 都是原子的**，且切换由单一持有者执行（baseVersion 校验保证没有并发发布者），文件内容与版本号语义一致（都是 V_{n+1} 的内容）；
- 与现状的区别：现状窗口内是"混合且语义错误"（旧版本号 + 未知混合内容），切换后窗口内是"新内容逐步可见、版本号同时或稍后翻转"——客户端要么拿到完整旧集，要么拿到新集子集（等价于"新版本已部分发布"，而文件 MD5 校验保证内容正确）。混合窗口从"语义错误"降级为"进度性可见"。
- **彻底消除**该残余窗口需要双目录 + 清单指针切换（见 4.4 备选），成本较高，列为后续演进。

**get_all_files / download 不变**：仍读 `data/{project}`。老客户端零改动。

### 4.2 publish-cli 配合变化

```
push 流程：
  1. 对每个待上传文件调用 upload 时带 --staging-version {version}
     （API: POST /api/file/upload_file?stagingVersion=xxx）
  2. 全部上传成功后调用 publish_version，新增字段：
     { "baseVersion": "<发布者本地认知的当前最新版本，可空>", "staging": true }
  3. publish_version 成功 → 服务端完成切换，push 清理本地暂存区
  4. publish_version 失败（含 baseVersion 不匹配）→ 服务端暂存保留，
     提示用户可用同版本号重推补齐（续传语义天然支持），或主动放弃
     （新增 API delete_staging，或复用 clean 命令语义）
```

`baseVersion` 的取值：publish-cli 在 push 开始时调用 `get_project_by_name` 拿当前 `version`。用户语义：

- `baseVersion == 服务端当前版本` → 正常发布；
- `baseVersion != 服务端当前版本` → 拒绝并报错：
  `"版本已前进（服务端最新 {X}，本地基于 {Y}）。请 pull 最新文件集后重新发布更高版本号。"`
- `baseVersion` 为空 → 兼容模式，跳过校验（旧 CLI 不受影响），仅在服务端日志告警。

注意与防倒退的关系：防倒退（版本号必须严格递增）保留不变；baseVersion 是**更早的失败点**——它在文件覆盖发生前就拒绝"基于过时认知"的发布，而不是发布后才发现版本号冲突。

### 4.3 publish 事务的写互斥（为 #9 铺路）

把 `PublishVersion` 事务改为 `BEGIN IMMEDIATE`（ent 下用 `tx.Driver().Exec("BEGIN IMMEDIATE")` 或在 `db.WithTx` 提供 `WithImmediateTx` 变体）：

- 事务开始即取写锁，"读最新 changelog → 比较 → 写"整个临界区对其他写者互斥；
- 配合 `SetMaxOpenConns(1)` 现状：语义不变（本来就串行）；
- 将来拆池后：即使读连接上有并发读，写事务间的防倒退校验依然互斥，版本号竞态不会出现。

这是 #9 绑定警告要求的"先加互斥"的最小实现，第一期就落地，成本极低。

### 4.4 备选：双目录 + 指针切换（彻底原子，列为演进项）

```
data/{project}/            ← 现网
data/{project}.v{N}/       ← 完整版本快照
data/{project}.current     ← 文本指针，内容 = 当前版本目录名
```

- `get_all_files` / download 按指针解析目标目录；
- 发布 = 上传到新快照目录 → 原子改写指针（一次 rename）→ 完全无混合窗口；
- 代价：磁盘翻倍（或需删除旧快照）、`DownloadFile` 路径解析多一层、历史快照生命周期管理。
- 结论：第一期不做；若第一期切换窗口（秒级）经实测仍不可接受再演进。

---

## 5. 第二期设计（可选）：项目级发布互斥 + 连接池拆分

### 5.1 项目级发布互斥

**方案：SQLite 行锁语义 + 进程内互斥，双保险**

1. 进程内 `sync.Map` 项目锁（复用现有 `lockForFile` 模式，升为 `lockForProject`）——单实例部署下足够；
2. `push` 开始时新增 API `begin_publish(project, baseVersion)`：
   - 校验 baseVersion（提前失败点）；
   - 获取项目发布租约（`project` 表加 `publish_lock_token` / `publish_lock_ts` 两列，`BEGIN IMMEDIATE` 事务内 CAS 写入 token + 时间戳）；
   - 返回 token，后续 upload/publish_version 都带 token，服务端校验 token 归属；
   - 租约 TTL 15 分钟，`finish_publish` / 超时自动释放（复用客户端更新锁的 TTL+心跳设计思路）；
3. `publish_version` 无有效 token 且项目有活跃租约 → 明确报错"另一发布进行中"。

**为什么不只用进程内锁**：为将来多实例部署留路；SQLite 的 CAS 行写就是最简分布式锁。

### 5.2 连接池拆分（#9 优化，与 5.1 同一变更）

```
db.go:
  写连接：SetMaxOpenConns(1)（保留串行写）
  读连接池：SetMaxOpenConns(N)、SetMaxIdleConns(N/2)
  WAL 模式保留
  路由：所有读 API（get_all_files / check 类 / get_project_*）走读池；
        写 API（upload*/publish_version/begin_publish/项目管理）走写连接
```

前提检查清单（缺一不做）：

- [ ] 5.1 的发布租约已上线；
- [ ] `PublishVersion` 用 `BEGIN IMMEDIATE`；
- [ ] 所有"读-改-写"型事务盘点完毕，确认都在写连接上（ent 的 `WithTx` 默认走主连接，需要显式路由）；
- [ ] 压测：并发 check + download + push 场景下防倒退保护仍完备（并发发布同项目必有一方被拒）。

### 5.3 明确不做

- 文件级多版本合并（冲突文件提示手工处理）——超出自动更新系统定位；
- 面向发布者的"pull 服务端最新文件集"完整工作流（第一期只报错引导）。

---

## 6. 兼容性与回滚

| 维度 | 策略 |
|---|---|
| 老客户端 | 零影响：check/download/get_all_files/manifest 协议与语义不变 |
| 老 publish-cli | 零影响：不带 stagingVersion/baseVersion 走旧路径（服务端保留直发现网模式），仅失去 #2/#11 保护并在服务端日志告警 |
| 新 CLI × 老服务端 | CLI 对 publish_version 增加 `staging` 字段——老服务端忽略未知字段（Gin ShouldBindJSON 默认忽略），行为退化为直发；CLI 端无需探测 |
| staging 残留 | 服务端启动时扫描 `data/.staging/` 超过 7 天的版本目录，记日志并清理（防发布者放弃后垃圾累积） |
| 回滚 | 新旧路径并存（带/不带 stagingVersion），回滚只需 CLI 停用新参数；服务端 staging 切换逻辑可独立下线 |

## 7. 测试要点

1. **混合窗口**：并发跑"push 新版本（中途 kill）" + "客户端 check+download 循环"，断言客户端拿到的文件集 MD5 集合 ∈ {完整旧集, 完整新集}；
2. **切换窗口**：publish_version 切换期间并发 download，断言无 404/半文件（rename 原子性 + per-file 锁）；
3. **baseVersion 拒绝**：A 基于 V1 发布 V2，B 在 A 上传期间发布 V2'，A 的 publish_version 应失败且现网 = V2' 完整集；
4. **防倒退回归**：baseVersion 空（兼容模式）时旧防倒退语义不变；
5. **staging 清理**：构造 7 天前的暂存目录，重启后确认被清；
6. **绑定回归（第二期）**：拆池后并发 publish 同项目 × 100，断言 changelog 版本严格递增且无重复。

## 8. 落地顺序建议

1. **P0（第一期核心，1 个 PR）**：staging 上传/切换 + baseVersion 校验 + `BEGIN IMMEDIATE` + publish-cli 配合 + 上述测试 1-5；
2. **P1（第二期，独立 PR，可与 P0 相隔任意时间）**：发布租约 + 连接池拆分 + 测试 6；
3. **P2（演进项，按需）**：双目录指针切换（若 P0 的秒级窗口实测不可接受）。

## 9. 风险与开放问题

- **staging 磁盘占用**：上传期间暂存 + 现网并存，峰值 ≈ 2× 版本大小。大项目（GB 级）需在文档标注磁盘要求；切换完成后立即删除暂存，不长期占用。
- **切换失败的中间态**：rename 循环中途失败（磁盘满等）→ 现网部分新部分旧（与现状同级，但 changelog 已提交、版本号已翻转，无法靠防倒退重推同号）。缓解：切换前先把暂存目录 rename 为 `.staging/.../{version}.switching`，失败保留现场供运维恢复；切换成功才删。该中间态概率远低于现状（切换只剩本地磁盘 rename，无网络环节）。
- **多实例部署**：第一期进程内 per-file 锁与切换流程都假设单实例。多实例需先把 staging 目录放到共享存储或走 5.1 租约的token 路由（同实例亲和）。当前项目无多实例部署形态，标注为约束。
- **open question**：`get_all_files` 的缓存（file_cache.go）在切换后需失效——切换完成时按项目名调缓存失效入口即可，需在实现时确认失效接口的并发安全（已有 singleflight）。
