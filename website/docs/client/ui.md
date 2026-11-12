---
title: 图形前端 aly-client-ui
description: aly-client-ui 的 Windows/Linux 形态、启动模式、-lang 与 -project-name 参数、单实例转发以及 -finish-self-update 收尾流程。
---

# 图形前端 aly-client-ui

`aly-client-ui` 是 `aly-client` 的可选图形前端。它不实现更新逻辑，而是启动同目录（或主程序目录）的更新器子进程，解析其 JSON 输出，在屏幕右下角用置顶卡片展示检查结果、下载进度和询问按钮。

<ScreenshotFrame src="/images/client-ui-update-zh.webp" alt="aly-client-ui 发现新版本的中文卡片" caption="发现新版本时的卡片：版本号、本次更新内容，以及「立即更新」「稍后」按钮" />

卡片不抢焦点、不进任务栏，适合在全屏作业软件上方按需出现。更新完成后由 `aly-client` 启动主程序，UI 进程退出。

## 平台与文件命名

| 平台 | UI 可执行文件 | 更新器 | 实现 |
|---|---|---|---|
| Windows | `aly-client-ui.exe` | `aly-client.exe` | Go 1.10.8 / 386，`lxn/walk`，Windows XP 兼容，无控制台窗口 |
| Linux | `aly-client-ui` | `aly-client` | Go 1.27 + MyGo 0.8.3 原生 UI，运行时依赖 GTK3，不依赖 WebKitGTK |

两者都读取 `UpdateFolder/client.json`，并从 `MainFolder/.updator/shared.json` 取项目名作为标题。

- Windows 查找更新器时先看 UI 同目录，找不到再按 `client.json` 推导主程序目录并查找 `aly-client.exe`。
- Linux 只使用 `UpdateFolder/aly-client`；更新器必须与 UI 同目录。

## 启动模式

`-mode` 默认 `auto`。未知值回退 `auto`；大小写与首尾空白会被归一化。

| 模式 | 窗口行为 | 说明 |
|---|---|---|
| `auto` | 有更新才显示 | 主程序启动时调用。无更新、检查失败（断网/未配置服务器）一律静默退出，不打扰用户 |
| `silent` | 同 `auto` | 语义等价，保留以表达“明确不要打扰” |
| `check` | 总是显示 | 用户主动点“检查更新”时调用。先显示“正在检查更新…”；无更新时明确提示（Windows 弹确认框，Linux 5 秒后自动关闭）；出错显示原因与重试/关闭 |
| `update` | 预留 | 接受该值。Windows 当前与 `auto` 共用流程（有更新才显示）；Linux 走 `check` 的可见流程。不要依赖它实现“静默强制更新” |
| `rollback` | 总是显示 | 列出可回滚版本供选择，选定后执行 `rollback --version <v>` |

平台差异：

- Windows 的 `auto` / `silent` 实例在后台检查；若此时用户又启动一个 `-mode check` 或 `-mode update` 实例，后者会转发“手动检查请求”，第一个实例升级为交互式并显示窗口。
- Linux 的第二个实例会把完整参数转发给第一个实例；第一个实例显示并聚焦窗口，按参数切换模式，必要时重新开始检查或回滚。

### 更新交互

非强制更新使用卡片按钮，不弹模态框、不抢焦点：

```text
check_update → 发现新版本 → [立即更新] [稍后]
             → download_update（字节进度 + [暂停/继续]）
             → [重启完成更新] [稍后]
             → apply_update → 主程序被重启
```

- **强制更新**：显示 10 秒倒计时并自动重启；用户可点「重启完成更新」提前结束等待。
- **暂停/继续**：暂停会结束下载子进程，保留 `.part` 断点；继续时用新会话重新发起下载，已完成文件不会重传。
- **失败重试**：错误卡片显示人话化原因（如文件被占用），提供「重试」「关闭」；重试回到失败的那个阶段，不重跑已成功的阶段。
- **下载进度**：优先使用字节级进度（已下载/总量/速度）；旧版更新器没有字节字段时回退为文件计数。
- **成功**：显示“更新完成”约 1.2 秒后退出；主程序由 `aly-client` 启动。
- `auto` / `silent` 的检查失败、无更新均静默退出；转发升级为交互式后才显示错误或无更新提示。

## 启动参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `-mode` | string | 否 | `auto` | `auto` / `check` / `update` / `rollback` / `silent` |
| `-lang` | string | 否 | `zh-CN` | `zh-CN` / `en-US`；未知值回退中文 |
| `-project-name` | string | 否 | 空 | 仅覆盖窗口/卡片标题；不传给更新器 |
| `-url` | string | 否 | 空 | **预留，当前不生效**；更新器自己读取 `shared.json` |

Go 的 `flag` 包同时接受单横线与双横线写法，`-mode` 与 `--mode` 等价。

### -lang

- 默认 `zh-CN`；`en-US` 为英文。
- 接受 `en`、`en_us`、`EN-US` 等大小写/下划线变体。
- 不按系统区域自动切换，界面内也没有语言切换入口；语言是启动期一次性注入。
- 未知值一律回退简体中文。

<ScreenshotFrame src="/images/client-ui-update-en.webp" alt="aly-client-ui update card in English" caption="使用 -lang en-US 启动时的英文卡片" />

### -project-name

只决定窗口与卡片标题，不改服务端项目名，也不传给 `aly-client`。缺省时：

1. 读取 `MainFolder/.updator/shared.json` 的 `project_name`；
2. 读不到时回退 `aly-client-ui`。

### -url

保留参数，当前没有效果。更新器自己从 `shared.json` 读取 `server_url`；需要临时覆盖地址时，应直接调用 `aly-client` 的 `--url`，而不是依赖 UI。

### 示例

Windows：

```bat
:: 主程序启动时调用（推荐）
..\UpdateFolder\aly-client-ui.exe -mode auto -lang zh-CN

:: 用户点“检查更新”
..\UpdateFolder\aly-client-ui.exe -mode check -lang zh-CN

:: 回滚界面
..\UpdateFolder\aly-client-ui.exe -mode rollback -lang en-US
```

Linux：

```sh
./aly-client-ui -mode auto -lang zh-CN
./aly-client-ui -mode check -lang en-US
./aly-client-ui -mode rollback -lang zh-CN
```

## 窗口特征

| 平台 | 尺寸与位置 | 窗口属性 |
|---|---|---|
| Windows | 380×160（96 DPI），位于工作区右下角（已扣除任务栏），随系统 DPI 等比缩放；有更新内容时卡片自动加高并重新定位 | `HWND_TOPMOST`、`WS_EX_TOOLWINDOW`（不进任务栏、不进 Alt-Tab）、`SWP_NOACTIVATE`（不抢焦点） |
| Linux | 420×250（最小 380×210），位于主显示器工作区右下角 | 无边框、不可缩放、`AlwaysOnTop`、`SkipTaskbar` |

## 单实例

同一用户会话只允许一个 UI 实例，第二个实例立即以退出码 `0` 结束。

- **Windows**：使用会话命名互斥体 `AlyClientUI_SingleInstance`。第二个实例若 `-mode check` 或 `-mode update`，会通过命名事件 `AlyClientUI_CheckRequested` 通知第一个实例；第一个实例显示窗口并按交互式流程重新检查。其它模式的第二个实例只退出，不转发。
- **Linux**：使用 MyGo 的 `RequestSingleInstanceLock` 与 `OnSecondInstance`。第二个实例的参数会转发给第一个实例；第一个实例显示/聚焦窗口，按参数切换模式，并在空闲、无更新或错误阶段重新开始检查（`rollback` 模式则打开回滚列表）。

单实例不仅避免两个右下角窗口互相覆盖，也避免两个进程同时替换 `aly-client` 或 UI 自身。

## 更新器与 UI 的自更新

UI 启动时按顺序处理两个自更新，失败都不阻断本次启动：

1. **更新器自更新**：`UpdateUpdater` 委派 `<UpdateFolder>/aly-client self_update`，把 `MainFolder` 中随版本分发的新版更新器同步到 `UpdateFolder`。只有更新器缺失或旧版不认识 `self_update`（进程级失败）时才直接复制兜底；锁被占用（JSON 业务失败）不兜底，避免绕过 `.aly.lock`。
2. **UI 自身自更新**：比较 `MainFolder/aly-client-ui(.exe)` 与 `UpdateFolder/aly-client-ui(.exe)`。有差异时复制为 `.new`，再派生 `.new` 的隐藏收尾模式 `-finish-self-update`；旧 UI 立即退出，把替换与重启交给收尾进程。`self_update` 命令只管更新器，管不了 UI 自身。

启动自更新有防循环 Journal：

| 平台 | Journal 文件 |
|---|---|
| Windows | `UpdateFolder/aly-client-ui.exe.selfupdate.json` |
| Linux | `UpdateFolder/aly-client-ui.selfupdate.json` |

30 分钟观察窗口内连续 3 次未收敛，则本次跳过 UI 自更新（UI 仍正常启动和检查更新）；窗口过后自动恢复。删除该文件可立即重置计数。

## -finish-self-update

`-finish-self-update` 是 UI 的隐藏收尾模式，由旧 UI 在自更新时派生，普通宿主不需要直接调用。它在单实例互斥体与窗口创建之前处理；Linux 上也在 MyGo/GTK 初始化之前处理，因此无图形环境也能完成收尾。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `--target` | string | 是 | 旧 UI 可执行文件的绝对路径 |
| `--pid` | int | 是 | 旧 UI 的进程 ID |
| `--created` | string | 否 | 旧进程身份校验值：Windows 为十进制 `FILETIME` 创建时间，Linux 为 `/proc/<pid>/stat` 的 `starttime`；`0` 或空表示未知 |
| `--host-pid` | int | 否 | 原始宿主进程 PID。收尾进程拉起新 UI 时会把它追加进新 UI 的参数，新 UI 的「宿主跟随」据此继续跟随真正的宿主——否则新 UI 的父进程（收尾进程）随即退出，宿主跟随会在约 1 秒后误判「宿主已退出」而自杀，auto 模式的更新卡片永远弹不出来 |
| `--new` | string | 是 | 已校验的新 UI 文件绝对路径（通常为 `.new`） |
| `--` | — | 否 | 其后所有参数原样透传给重启后的新 UI |

约束：

- `--target` 与 `--new` 必须是绝对路径且不相同；`--new` 必须存在。
- 缺少 `--target`、`--new`、`--pid`，或出现未知参数，收尾进程以退出码 `2` 结束。
- 参数解析不使用 `flag` 包，因为 `--` 之后要原样透传；`--` 之前只接受上述五个选项。

### 流程

1. **等待旧进程退出**：
   - Windows：打开旧进程句柄，`WaitForSingleObject` 最多 10 秒；用进程创建时间校验 PID 是否被复用，复用则视为旧进程已退出，绝不误杀；超时 `TerminateProcess`，再等 2 秒。
   - Linux：轮询 `/proc/<pid>`，用 `starttime` 校验 PID 复用，僵尸态（`state=Z`）视为已退出；最多 10 秒，超时 `SIGKILL`，再等 2 秒。
2. **无法确认旧进程状态时中止**（退出码 `1`）：宁可不换，也不要在旧进程仍持锁时改文件并拉起一个会撞单实例互斥体的新 UI。
3. **先备后换**：目标存在时改名为 `.old`；把 `.new` 改名到目标。rename 以 20 次 × 250 ms 重试，覆盖杀软短暂占用；落位失败则把 `.old` 改回目标。
4. **拉起新 UI**：用 `--` 之后的原参数启动目标（收尾进程会把 `--host-pid <原始宿主>` 追加进去，restart 参数里残留的旧 `--host-pid` 会被剥掉），不经过 `cmd.exe`，没有 PATH、引号、`%` 展开问题。
5. **写日志**：追加到 `<target>.update.log`；Windows 为 `aly-client-ui.exe.update.log`，Linux 为 `aly-client-ui.update.log`。
6. 成功退出码 `0`；等待/替换/启动失败退出码 `1`。

示例（由旧 UI 自动构造，无需手工执行）：

```bat
aly-client-ui.exe -finish-self-update ^
  --target C:\App\UpdateFolder\aly-client-ui.exe ^
  --pid 4321 ^
  --created 133456789012345678 ^
  --host-pid 1234 ^
  --new C:\App\UpdateFolder\aly-client-ui.exe.new ^
  -- -mode check -lang zh-CN
```

## 退出码

| 退出码 | 场景 |
|---|---|
| `0` | 正常结束；第二个实例直接退出；收尾进程替换并重启成功 |
| `1` | 收尾进程无法确认旧进程状态、替换失败或启动新 UI 失败 |
| `2` | 窗口/路径初始化失败；收尾进程参数非法 |

## 相关页面

[命令参考](/docs/commands) · [配置参考](/docs/client/config) · [更新器概览](/docs/client/) · [自更新设计](/docs/self-update)