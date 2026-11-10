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

## Linux UI 的收尾进程

替换**正在运行的 GUI 自身**在 Linux 上由 `-finish-self-update` 收尾模式完成：

```bash
aly-client-ui -finish-self-update \
  --target /path/aly-client-ui \
  --pid 12345 --created <starttime> \
  --new /path/aly-client-ui.new \
  -- <重启命令与参数...>
```

流程：脱离当前会话（setsid）→ 校验目标进程身份（`/proc/<pid>/stat` 的 starttime 字段防 PID 复用）→ 等待其退出（最长 10 秒，超时 SIGKILL + 2 秒宽限）→ `.new` 原子落位 → 按参数重启新 UI。全程写日志到 `aly-client-ui.update.log`，且不依赖 GTK——可以在无图形环境的脚本里运行。

## 测试口径

自更新链路的行为契约由三层测试钉住（详见[测试体系](/advanced/testing)）：

- `selfupdate` 包 14 个单元测试（哈希比对、替换编排、改名舞步、回滚、Journal）；
- `cmd` 包 `self_update` 命令 4 个测试（JSON 协议、锁语义）；
- tc 端到端用例 130–135（统一执行路径委派、UI 防循环、收尾进程全流程）。
