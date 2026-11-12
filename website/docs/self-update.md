# 自更新设计

"更新器自己怎么更新"是自动更新系统最容易被忽视的死结：替换一个正在运行的 exe，还得保证断电不死、循环不生。Aly 的自更新围绕**检测与执行分离**构建。

## 核心设计：新版本来源只有一条路

任何组件都不会从服务器直接下载"更新器自己"。新版本统一走主更新链路：

1. 发布方把 `aly-client`（更新器）当作普通文件打进版本包；
2. `download_update` 按服务端清单下载（MD5+SHA256 校验）；
3. `apply_update` 把主程序目录整体原子替换成新版本——此时主程序目录里就有了**新版更新器**；
4. 自更新只是把 `UpdateFolder` 里"正在运行的旧副本"同步成主程序目录的新版。

源的完整性由主链路保证，自更新层只做比对与搬运。

## 三个组件的分工

| 组件 | 角色 |
|---|---|
| `check_self_update` 命令 | **只检测**：比较自身副本与主程序目录副本的 SHA256，输出 `need_update` JSON，不执行替换 |
| `self_update` 命令 | **统一执行入口**：更新器本体取 `.aly.lock`，把主程序目录的新版替换进 UpdateFolder（GUI/SDK 都委派给它） |
| GUI / SDK | **编排**：启动时先检测，`need_update=true` 时委派 `self_update`；仅在更新器缺失或过旧时才走本地复制兜底 |

`self_update` 的替换动作由便携包 `selfupdate` 实现（XP 轨与现代轨共用）：

- SHA256 比对决定是否需要更新（无差异直接跳过）；
- `.new` 中转文件 + 复制后校验 + 原子落位；
- Windows 上目标被占用（正在运行的更新器）时执行**改名舞步**：把运行中的 exe 改名为 `.old` 备份 → 落位新版 → 失败则回滚，全程目标路径不会消失；
- 残留的 `.new`/`.old` 文件在启动时自动清理；
- **失败计数 Journal**：30 分钟窗口内连续失败 ≥3 次则跳过自更新——根治"替换失败 → 重启 → 再失败"的无限循环。

## 两件套：`aly-client` 与 `aly-client-ui` 一起收敛

更新器目录里实际上是**两件套**：`aly-client(.exe)`（真正执行下载/替换）与
`aly-client-ui(.exe)`（图形前端）。发布包把两件套都投放到主程序目录（`deploy` 脚本、
publish GUI 的 `UpdateFolderService`、`deploy-client-linux.sh` 都按“同名两件套”布局），
但 `UpdateFolder` 里的副本各有各的收敛路径：

| 副本 | 收敛者 | 语义 |
|---|---|---|
| `aly-client(.exe)` | `self_update`（更新器本体） | 按 SHA256 收敛：无差异跳过、有差异走改名舞步替换、源缺失跳过 |
| `aly-client-ui(.exe)` | **UI 自己的启动自更新**（`UpdateSelfUI` + `-finish-self-update`） | 有差异时复制 `.new` 并派生收尾进程：等旧 UI 退出 → 旧 exe 改名 `.old` → `.new` 落位 → 原参数拉起新 UI |
| `aly-client-ui(.exe)`（**缺失时**） | `self_update --include-ui` | 只在 `UpdateFolder` 里缺失或为 0 字节时从主程序目录落位（补齐），已存在的一律不碰 |

为什么 UI 副本要分两条路：替换**正在运行的 UI 镜像**必须由 UI 自己收尾（它得先退出、
再落位、再把自己拉起来），而“UI 副本根本不在”时没有任何进程能执行这件事——后者就是
现场故障：宿主调 `AlyUpdateUIClient.LaunchAuto()` 只拿到 `aly-client-ui.exe not found`。
`--include-ui` 专治这条路：

- 目标缺失/0 字节 → `ui_reason=replaced`（从主程序目录落位，不产生 `.old`）；
- 目标已存在 → `ui_reason=existing_kepped`（**不替换可能正在运行的镜像**）；
- `<ui>.selfupdate.json` 的 `pending=true`（收尾进程在途）→ `ui_reason=pending_inflight`（让位，不插队）；
- 主程序目录没有该副本 → `ui_reason=source_missing`（不算错误）。

两个平台的副本**同名**（Windows `aly-client-ui.exe`，Linux `aly-client-ui`），源名由目标名
推导，因此这条路径没有平台分支；落位复用 `selfupdate` 包的“临时文件 + SHA256 校验 +
原子落位（失败保留旧目标）”，Linux 上保留可执行位。

## 宿主侧：拉起 UI 之前先保证副本就位

C# SDK 的 `AlyUpdateUIClient` 在 `Launch*` 里先做一次“两件套就位”：

1. UI 副本与主程序目录副本 SHA256 一致 → 什么都不做（不派进程）；
2. 更新器副本缺失 → 直接用主程序目录副本落位（否则命令起不来）；
3. UI 副本缺失/不可读/落后 → 委派 `self_update --include-ui`；
4. 命令不可用或返回后副本仍缺 → 用主程序目录副本直接落位兜底；
5. 同一实例内只尝试一次，失败返回可诊断的 NG（不抛异常）。

**不会造成循环**：`--include-ui` 从不重启任何进程，且只做“缺失补齐”，收敛后必然
`existing_kepped`/一致跳过；UI 自身的替换循环仍由它自己的 Journal（30 分钟窗口内连续
3 次未收敛即跳过）兜住。

## Linux UI 的收尾进程

替换**正在运行的 GUI 自身**在 Linux 上由 `-finish-self-update` 收尾模式完成：

```bash
aly-client-ui -finish-self-update \
  --target /path/aly-client-ui \
  --pid 12345 --created <starttime> \
  --host-pid <原始宿主 PID> \
  --new /path/aly-client-ui.new \
  -- <重启命令与参数...>
```

流程：脱离当前会话（setsid）→ 校验目标进程身份（`/proc/<pid>/stat` 的 starttime 字段防 PID 复用）→ 等待其退出（最长 10 秒，超时 SIGKILL + 2 秒宽限）→ `.new` 原子落位 → 按参数重启新 UI。全程写日志到 `aly-client-ui.update.log`，且不依赖 GTK——可以在无图形环境的脚本里运行。

`--host-pid` 是"宿主跟随"的关键：新 UI 的父进程是收尾进程（拉起新 UI 后立即退出），收尾链把最初拉起旧 UI 的宿主 PID 传下来，新 UI 才不会在约 1 秒后误判"宿主已退出"而自杀（2026-11-12 修复——修复前的表现正是"启动时确认有更新却不弹卡片"）。

## 测试口径

自更新链路的行为契约由三层测试钉住（详见[测试体系](/docs/testing)）：

- `selfupdate` 包单元测试（哈希比对、替换编排、改名舞步、回滚、Journal，以及
  `SyncFile` 两件套补齐：缺失落位 / 已存在让位 / 收尾在途 / 源缺失 / 可执行位）；
- `cmd` 包 `self_update` 命令测试（JSON 协议、锁语义、`--include-ui` 的四种补齐口径）；
- tc 端到端用例 130–137（统一执行路径委派、UI 防循环、收尾进程全流程、
  两件套补齐 `--include-ui`、SDK 拉起 UI 前自动落位 sd17）。
