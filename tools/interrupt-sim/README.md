# aly-client 更新状态机「中断 → 恢复」模拟器

用于验证：**在更新/回滚的任意一个阶段被中断（断电、任务管理器结束进程）后，客户端能否自愈恢复到一致状态。**

纯 Python 3，无第三方依赖，不依赖 Windows API，可在任意平台运行。

---

## 为什么要有这个

仓库现有的 E2E（`client/aly-client/test/e2e/update_e2e.ps1`，123 例）覆盖了
download / apply 各阶段的中断，但**没有覆盖 rollback 中断**，也难以穷举
"中断 + 期间服务器发新版"这类组合。本模拟器用状态机模型补齐这块：
把每一步都作为注入点，逐个打断后跑多轮恢复，自动判定结果是否一致。

---

## 运行

```bash
cd tools/interrupt-sim

python3 scenarios3.py    # 多轮恢复判定（55 场景）—— 主用例
python3 scenarios.py     # 单轮场景（46 场景，含 download 中断、目录错配、version.json 损坏）
```

无需参数，输出直接打印到 stdout，机器可读明细写入 `result*.json`。

`results/` 下是本仓库提交时的一次实测快照：

| 文件 | 内容 |
|------|------|
| `results/multi_round_report.txt` | `scenarios3.py` 的输出（多轮恢复最终判定） |
| `results/single_round_report.txt` | `scenarios.py` 的输出 |
| `results/result_final.json` | 多轮判定明细（场景/中断点/轮数/磁盘/version.json 状态） |

---

## 文件说明

| 文件 | 作用 |
|------|------|
| `sim.py` | 状态机内核：忠实复刻 `cmd/apply_update.go`、`cmd/rollback.go`、`cmd/download_update.go`、`cmd/check_update.go`、`cmd/common.go`、`config/version.go` 的核心语义；提供磁盘模型、中断注入（`Crash`）、一致性判据。含 `cmd_resume_rollback`（对应 Go 端 `resumeRollback` 的四分支回滚中断恢复） |
| `scenarios.py` | 单轮场景族 A–F |
| `scenarios3.py` | 多轮恢复判定（模拟 SDK `MainLoop` 每 5s 轮询，最多 4 轮），场景族 A/B1/B2/B2'/C/G/H |

### 磁盘模型

`PackageFolder` 下每个目录用一个字符串表示该目录**当前装的是哪个版本的内容**：

- `App` —— 活动目录（`ApplicationFolder`）
- `App_{version}` —— 版本目录 / 历史快照
- `App_{version}.old[.N]` —— 旁移残留

`os.Rename` 按 Windows 语义建模：源必须存在；目标已存在（非空目录）时失败。

### 一致性判据（`sim.py: check_consistent`）

1. 主目录必须存在（否则应用打不开）；
2. `status=applied` 时：主目录内容版本 == `version.json.Version`；
3. `status=downloaded` 时：主目录内容版本 == `version.json.VersionPrevious`；
4. 回滚目录 `App_{VersionPrevious}` 若存在，其内容版本必须 == `VersionPrevious`（否则回滚会拿到错的内容）。

命中 1–3 判为 `FAIL`（应用不可用或版本错配）；只命中 4 判为 `WARN`（应用可用但回滚语义错乱）。

---

## 当前结论（详见根目录 `aly-client-中断恢复分析报告-复核与修复方案-20260923.md`）

> 下表为**修复落地后**（2026-09-23）的实测结果。对比修复前：Bug #3 死局、Bug #1/#2 的
> 回滚目录错配、Bug #2' 的 `version not found` 卡死均已消除。

| 场景族 | 结果（修复后） | 说明 |
|--------|--------------|------|
| A. apply 各阶段中断（服务器版本不变） | ✅ 9/9 PASS | 一轮自愈，与修复前一致 |
| G. apply 中断 + 期间服务器发新版 | ✅ 自愈且**目录错配消失** | 中断的 apply 先完成 V2（目录命名正确，不再出现 `App_2.0.0 装 1.0.0`），随后轮询下载应用 V3；sim 因"applied 即停"可能停在 V2（见下方 WARN 分类②） |
| C. apply 中断 + 宿主无条件 download | ✅ 9/9 PASS | **死局消除**：download 在 applying 期间不再改写状态（Bug #3） |
| B1/B2. rollback 中断 | ✅ 真正中断点（rename_backup 之后）3/3 PASS | 回滚被续跑完成到目标版本，不再被逆转成升级；剩余 FAIL 见下方分类①③ |
| B2'. rollback 中断后重跑 rollback | ✅ 7/7 PASS | **`version not found` 卡死消除**（委托 resumeRollback 续跑） |
| H. 回滚中断 + 期间服务器发新版 | ✅ 真正中断点 PASS | check 在回滚中断期强制不下载，先完成回滚（Bug #5，无需改 SDK） |
| D. download 中断 | ✅ 5/5 PASS | 与修复前一致 |
| E. 多次下载导致目录错配 | ✅ PASS | `VersionPrevious` 不再被覆盖，备份目录命名正确（Bug #2） |
| F. 断电写坏 version.json | ✅ PASS | ReadVersion 自愈：重置为首次部署语义后重下重装，应用可用（Bug #4） |

### 剩余 FAIL/WARN 分类（均非本批次修复范围内的缺陷）

模拟器忠实建模宿主"check→apply 无限轮询"，因此以下两类行为会如实显示为不一致：

1. **回滚意图从未持久化**（`rb:start` / `rb:write_applying` 中断点）：崩溃发生在
   `rollback_previous` 写入**之前**，version.json 无任何回滚记录，宿主机无从感知用户意图，
   停留在回滚前版本（无升级、无数据损坏）。这是固有行为，任何实现都无法恢复未持久化的意图。
2. **G 族"applied 即停"的人工产物**：sim 在 `applied` 状态立即停轮，G 族中断的 apply 先完成 V2
   后即停，未跑到下一轮下载 V3；真实宿主会继续下载并应用 V3。目录命名已正确（判据 4 通过），
   仅 `expect` 版本号不同。
3. **回滚成功后被下一轮轮询撤销**（`rb:remove_aside_entry` / `rb:aside_prev` 且磁盘无对应备份时
    tick 不触发 → 回滚实际完成 → 宿主检测到服务器版本更高后重新更新）：这是**回滚"钉住"
    （rollback pin）策略缺失**（产品/运维决策），不在原报告 6 个 bug 与本批次修复范围内；
   运维侧需下架/重发服务器版本才能让回滚保持。

---

## 维护约定

- 修改 `client/aly-client` 的状态机后，请同步更新 `sim.py` 的对应分支，并重跑两个脚本；
- 判据 4（回滚目录名与内容一致）容易被忽略，但正是 Bug #2 的发现来源，请勿删除；
- 本目录不参与客户端构建（Go 1.10 / GOPATH 模式），也不属于 publish-gui，
  因此不受 `AGENTS.md` 的 C# 命名与审查流程约束。
