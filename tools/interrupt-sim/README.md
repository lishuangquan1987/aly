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

python3 scenarios3.py    # 多轮恢复判定（48 场景）—— 主用例
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
| `sim.py` | 状态机内核：忠实复刻 `cmd/apply_update.go`、`cmd/rollback.go`、`cmd/download_update.go`、`cmd/check_update.go`、`cmd/common.go`、`config/version.go` 的核心语义；提供磁盘模型、中断注入（`Crash`）、一致性判据 |
| `scenarios.py` | 单轮场景族 A–F |
| `scenarios3.py` | 多轮恢复判定（模拟 SDK `MainLoop` 每 5s 轮询，最多 4 轮），场景族 A/B1/B2/B2'/C/G |

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

## 当前结论（详见根目录 `aly-client-中断恢复分析报告-20260923.md`）

| 场景族 | 结果 |
|--------|------|
| A. apply 各阶段中断（服务器版本不变） | ✅ 9/9 一轮自愈 |
| G. apply 中断 + 期间服务器发新版 | ⚠️ 5/9 出现回滚目录"名与内容错配"（Bug #2）；其余可自愈 |
| C. apply 中断 + 宿主无条件 download | ❌ 1 例死局（主目录永久缺失，Bug #3）；3 例静默装回旧版本 |
| B1/B2. rollback 中断 | ⚠️ 回滚意图被取消 / 被逆转为升级（Bug #1、#5） |
| B2'. rollback 中断后重跑 rollback | ⚠️ 1 例卡在 `applying` 无法收敛 |
| D. download 中断 | ✅ 5/5 安全 |
| F. 断电写坏 version.json | ❌ 全命令瘫痪且无自愈（Bug #4） |

---

## 维护约定

- 修改 `client/aly-client` 的状态机后，请同步更新 `sim.py` 的对应分支，并重跑两个脚本；
- 判据 4（回滚目录名与内容一致）容易被忽略，但正是 Bug #2 的发现来源，请勿删除；
- 本目录不参与客户端构建（Go 1.10 / GOPATH 模式），也不属于 publish-gui，
  因此不受 `AGENTS.md` 的 C# 命名与审查流程约束。
