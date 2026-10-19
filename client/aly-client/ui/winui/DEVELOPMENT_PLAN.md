# aly-client-ui 开发计划（v2 评审修订版）

> 状态：**已评审通过（2026-10-19）**，尚未开始编码；详细任务拆分见同目录 `IMPLEMENTATION_PLAN.md`
> 目标产物：`aly-client-ui.exe`（**Go 1.10.8** + walk，32 位，**兼容 Windows XP（硬性要求）**）
> 位置：`client/aly-client/ui/winui/`（GOPATH 下即 `aly/client/aly-client/ui/winui`）
> 调用链：`主程序(YOFC.OTDR3001.exe) → aly-client-ui.exe → aly-client.exe`
>
> **v2 相对 v1 的变更**（依据见 §0.9，均为实测/源码证据）：
> 1. **修正 walk 钉选版本** —— v1 钉的 `9958a48b` 用 Go 1.10.8 **编译不过**
> 2. **补全依赖清单** —— 4 个；v1 只列 2 个，漏 `golang.org/x/sys` 与 `govaluate`
> 3. **改用 WSL Go 1.10.8 构建** —— v1 的 `build-ui.bat`（参照 `build.bat`）会用 go1.26 产出 XP 无法加载的二进制
> 4. **修正命令映射** —— `check_update` 不接受 `--main-exe-path`
> 5. **修正 `force_update` 语义** —— 它是 `*bool`+`omitempty`，**可能整个缺失**
> 6. **删除已修复的 P0 风险**（`--must-close-process-name` 已补齐）
> 7. **新增**：§3.0 部署布局、§3.3 错误/退出码/超时契约、§6 UI 自更新、§4.6 明确不做清单
> 8. **新增前置任务**：当前提交的 `aly-client.exe` 是 Go 1.26 构建，**XP 无法加载，必须先修**

---

## 0. 关键决策

### 0.1 已确认决策

| 项 | 结论 | 确认时间 |
|----|------|----------|
| **0.1 UI 实现方式** | **方案 A**：Go + walk **复刻**现有 C# SDK 的更新流程与行为（状态机、事件、JSON 字段对齐），UI 用 Go 写。与"Go+walk / 32 位 / XP"约束一致 | 2026-10-19 |
| **0.2 多语言** | 中英双语，**由主程序（调用者）通过启动参数传入**；**未传参数默认中文**（zh-CN）。UI 不自行按系统区域切换 | 2026-10-19 |
| **0.3 强制更新语义** | 由服务器 `force_update` 决定，本地不做额外强制；**但必须处理字段缺失**（见 §3.5） | 2026-10-19 |
| **0.4 UI 目录** | `client/aly-client/ui/winui/`（GOPATH 下 `aly/client/aly-client/ui/winui`） | 2026-10-19 |
| **0.5 XP 兼容** | **硬性要求**。Go 1.10.8 + GOPATH + `GOARCH=386` | 2026-10-19（复核确认） |
| **0.6 UI 部署位置** | **`UpdateFolder/`**（与 `aly-client.exe` 同目录）。**不得**放入主程序目录 | 2026-10-19（复核确认） |
| **0.7 窗口形态** | **右下角按需显示**：窗口在右下角创建但默认隐藏，topmost，默认不抢焦点。`auto`（启动）无更新不显示；`check`（手动）总是显示（见 §4.1） | 2026-10-19（复核确认） |
| **0.8 视觉风格** | 扁平现代化（XP 约束内）：自绘进度/图标、ComCtl32 v6、单一强调色 | 2026-10-19（复核确认） |
| **0.9 DTO 共享** | 直接共用 `aly/client/aly-client/model`（同仓库编译，非镜像副本）；仅共享 `model`，禁共享 `config/cmd/util` | 2026-10-19 |

### 0.9 v2 修正依据（评审发现）

| # | v1 的问题 | 证据 | v2 处理 |
|---|-----------|------|---------|
| 1 | §5.1 钉 walk `9958a48b`，理由是其 README 写 "Go 1.8+" | **实测**：该 rev 用 Go 1.10.8 编译报 `messagebox.go:62: undefined: strings.ReplaceAll`（Go 1.12 API，由 `0121872` 于 2019-05-22 引入）。**README 已过时且错误** | 改钉 `bf6b8a4b`（`0121872` 父提交），已实测编译通过（§5.1） |
| 2 | §2.1 vendor 只列 walk + win | walk 的 `expression.go` 强制 import `gopkg.in/Knetic/govaluate.v3`；walk 另 import `golang.org/x/sys/windows`（`icon.go`）。缺任一直接编译失败 | 补全 4 个依赖并逐个钉 rev（§5.1） |
| 3 | §5.3 新增 `build-ui.bat`「参照 `build.bat`」 | **实测**：本机 `build.bat` 用 PATH 上的 go1.26，产出 PE `SubsystemVer=6.1`（Win7）→ **XP 加载器拒绝**；WSL Go 1.10.8 产出 `4.0` → XP 可加载。项目真正的 XP 构建路径是 `deploy-client.go`（WSL + `/usr/local/go` = go1.10.8） | 改用 WSL Go 1.10.8，扩展 `deploy-client.go`；`build-ui.bat` 降级为本地校验且**强制校验 go 版本**（§5.3） |
| 4 | §3.3 表给 `check_update` 列了 `--main-exe-path` | `check_update.go:15-18` 只注册 `-url`/`-project-name`；C# SDK 也从不传任何 flag（`AlyApi.cs:27-28`） | 修正映射表（§3.4） |
| 5 | §6 称「对齐 SDK 的 `RequestDownloadUpdate`/`RequestApplyUpdate` 仅在非强制时**阻塞等待用户**」 | **错误**。二者是普通 event（`AlyUpdateClient.cs:325/328`），`?.Invoke` 同步返回，**不阻塞、不等待决定**，下载/应用照常继续；SDK 内无任何 `ManualResetEvent`/`TaskCompletionSource` | 明确：UI 的"询问"是**新增行为**，非复刻（§3.5、§7） |
| 6 | §8 风险 4 称 `--must-close-process-name` 触发 exit 2 | 已在 `903baab`(2026-09-22) 修复：`apply_update.go:44`、`rollback.go:20` 注册该 flag，`mergeMustCloseFlag` 合并 | 删除该风险项 |
| 7 | §9 引用 `client/aly-client/COMMANDS.md` | 该文件实际在**仓库根目录** | 修正路径（§10） |
| 8 | §3.3 把 `force_update` 当恒存在的 bool | `model/dto.go:64` 是 `*bool` + `omitempty`，在 `downloaded`/`applying` 续跑分支**不返回**；`DownloadProgress` 另含 `error` 字段 | 修正 DTO 说明并定义缺失语义（§3.5） |
| 9 | §9 引用 `aly-client-更新方案分析-20260922.md` 作为风险依据 | 该文档写于修复之前，P0 结论已失效 | 降级为历史参考 |
| 10 | §4.1 用 `SystemParametersInfo(SPI_GETWORKAREA)` | **实测 `SPI_GETWORKAREA` 不在 `lxn/win` 中**（`SystemParametersInfo` 本身在，常量不在） | 改用 `MonitorFromWindow` + `GetMonitorInfo`（两者均已在 win 包中，见 §4.1） |
| 11 | §4.2 用 `SetWindowRgn` 做圆角 | **实测 `SetWindowRgn` / `CreateRoundRectRgn` 不在 `lxn/win` 中** | 需自行 `syscall.NewLazyDLL("gdi32.dll")` 声明，或先不做圆角（§4.2） |
| 12 | 全文未定义 UI exe 的部署位置与自更新 | C# SDK 的 `UpdateSelf()`（`AlyUpdateClient.cs:76-133`）负责拷贝 `aly-client.exe`；UI 取代 SDK 后无人接手 | 新增 §3.0 部署布局、§6 UI 自更新 |

---

## 1. 目标与边界

### 1.1 目标
- 提供一个**独立可执行**的 GUI `aly-client-ui.exe`，封装 `aly-client.exe` 的全部更新相关命令。
- 主程序**只需启动 `aly-client-ui.exe`**；以后更新只替换 `aly-client-ui.exe` / `aly-client.exe`，**主程序零集成、零改动**。
- 更新行为与现有 C# SDK（`AlyUpdateClient`）**流程一致**：检查 → 发现 → 下载 → 应用 → 历史回滚。是否强制由服务器决定。
- 进程启动即在**屏幕右下角 topmost 显示**，外观在 XP 约束内尽量现代化。

### 1.2 边界（不做）
- 不重写 `aly-client.exe` 的下载/校验/应用逻辑；UI 只做**编排 + 展示**。
- 不引入任何需要 cgo、Go 1.11+ 或 Vista+ 专属 API 的依赖。
- 不修改主程序代码。
- 视觉上**不做**（XP + walk 硬限制，见 §4.6）：深色模式、自定义标题栏、DWM 圆角/阴影/毛玻璃、Per-Monitor V2 DPI。

### 1.3 前置任务（阻塞本计划，**已确认纳入本计划执行并提交**）
> **当前仓库已提交的 `client/aly-client/aly-client.exe` 是 Go 1.26 构建**（`go version` 读出 `go1.26.0`，PE `SubsystemVer=6.1`）→ **Windows XP 无法加载**，与 AGENTS.md 规则 6 冲突。
> 必须先修：用 WSL Go 1.10.8 重新构建并提交（产出 PE `SubsystemVer=4.0`），否则 UI 做出来也无法在 XP 上工作。
> 同时修 `client/aly-client/build.bat`：它直接用 PATH 上的 `go`，会静默产出非 XP 二进制。应加 go 版本校验。
>
> 交付：`aly-client.exe`（Go 1.10.8 构建）+ `build.bat`（含 go 版本断言）→ 提交。
> 详细任务拆分见 `IMPLEMENTATION_PLAN.md` 的**任务 0**。

---

## 2. 架构与职责

```
主程序 YOFC.OTDR3001.exe
        │  仅启动（可带 -mode / -lang 参数）
        ▼
aly-client-ui.exe            ← 本计划产物（win32 GUI, 无控制台, 常驻右下角 topmost）
        │  os/exec 调用，CREATE_NO_WINDOW，读 stdout JSON
        ▼
aly-client.exe               ← 既有 Go CLI（不修改）
```

### 2.1 目录与包

```
client/aly-client/ui/winui/
├── main.go               // win32gui 入口，解析 -mode/-lang，装配依赖
├── app/                  // 应用装配与生命周期（单实例、日志、client.json 单字段读取）
├── bridge/               // 调用 aly-client.exe 的薄封装
│   └── client.go         //   exec + 逐行 JSON 解析（对齐 C# AlyApi，但修正其缺陷）；
│                         //   DTO 直接共用 aly/client/aly-client/model，见 §3.4
├── flow/                 // 更新状态机（对齐 C# AlyUpdateClient）
│   └── updater.go
├── view/                 // walk 窗口/对话框/进度 + 自绘控件（现代化样式）
│   └── theme/            //   配色、字体、尺寸常量
├── i18n/                 // 中英文案
├── vendor/               // 三方依赖（4 个，见 §5.1）
│   ├── github.com/lxn/walk
│   ├── github.com/lxn/win
│   ├── golang.org/x/sys
│   └── gopkg.in/Knetic/govaluate.v3
└── DEVELOPMENT_PLAN.md   // 本文档
```

### 2.2 分层依赖方向（单向，禁止回指）
`main → app → flow → bridge → aly-client.exe`；`view` 仅依赖 `flow` 暴露的接口，`flow` 不依赖 `view`（通过回调/事件通知 UI）。

---

## 3. 接口契约

### 3.0 部署布局与文件位置（v2 新增）

实测布局（`client.json` 的 `main_exe_relative_path = "../win-x64/YOFC.OTDR3001.exe"`）：

```
PackageFolder/                    ← ExeDir 的父目录
├── UpdateFolder/                 ← aly-client.exe 所在目录 = 所有相对路径的基准
│   ├── aly-client.exe            ← 更新器本体
│   ├── aly-client-ui.exe         ← 【本计划产物，必须放这里】
│   ├── client.json               ← main_exe_relative_path + must_close_process_name
│   ├── version.json              ← 状态机
│   ├── .aly.lock                 ← 全局更新锁
│   └── update.log
├── win-x64/                      ← 主程序目录（= MainFolder，会被原子改名）
│   ├── YOFC.OTDR3001.exe
│   ├── aly-client.exe            ← check_self_update 的比对源（发布包携带）
│   ├── aly-client-ui.exe         ← UI 自更新的比对源（发布包携带，见 §6）
│   └── .updator/shared.json
├── win-x64_{version}/            ← 版本快照（下载产物 + 历史备份）
└── win-x64.old / win-x64.old.N   ← 旁移残留
```

**为什么 UI 必须在 `UpdateFolder`：**
- `apply_update` 会把 `MainFolder`（`win-x64/`）整体重命名，UI 若住在里面会被搬走；
- `win-x64/` 的内容会被更新包整包替换；
- 运行中的 exe 镜像被系统锁定，会干扰目录改名。

**推论**：`UpdateFolder` 是"更新器目录"、不属于发布物，因此 UI 的**分发与自更新需单独机制**（§6）。

### 3.1 启动参数（主程序 → UI）

```
aly-client-ui.exe [-url <服务器>] [-project-name <项目>] [-lang zh-CN|en-US] [-mode auto|check|update|rollback|silent]
```

| 参数 | 说明 | 默认 |
|------|------|------|
| `-url` | 覆盖服务器地址（透传给 aly-client） | **不传**（由 aly-client 读 `client.json`/`shared.json`） |
| `-project-name` | 覆盖项目名 | **不传**（同上） |
| `-lang` | 界面语言：`zh-CN` / `en-US`，**由主程序传入** | `zh-CN` |
| `-mode` | `auto`=**启动模式**（默认）：静默检查，**有更新才在右下角显示，无更新不显示**；`check`=**手动检查模式**：右下角弹窗，**无更新显示"已是最新版本"并 5s 后自动关闭**，有更新则显示更新界面（不自动关闭）；`update`=检查并更新；`rollback`=历史版本回滚界面；`silent`=同 `auto` 但不轮询 | `auto` |

> UI 自身不硬编码服务器地址（遵守 AGENTS.md 规则 2）。
> **设计决策**：`-url`/`-project-name` 默认不传，由 `aly-client.exe` 自行读配置 —— UI 无需解析配置，也无需共享 `config/` 包。
> 唯一例外见 §6.2（UI 为定位主程序目录需读 `client.json` 的单个字段）。

### 3.2 子进程调用（UI → aly-client）

- 一律 `os/exec`，`SysProcAttr.CreateNoWindow=true`、`HideWindow=true`，`WorkingDirectory = aly-client.exe 所在目录`（对齐 C# SDK 且修复其相对路径隐患）。
- stdout 为**单行 JSON**（`printOutput`）：`{"isSuccess":bool,"errorMsg":string,"data":...}`；下载类命令另在 stdout 逐行输出进度（`printProgress`）。
- UI 只消费 JSON，不解析人类可读文本。

### 3.3 输出、退出码与超时契约（v2 新增，v1 只写了"只消费 JSON"）

**stdout 解析规则**（必须逐条实现）：

1. 逐行读取；每行尝试解析为 `{"isSuccess":bool,"errorMsg":string,"data":...}`。
2. **非 JSON 行**（Go panic 输出、usage 文本等）→ 记日志、跳过，**不崩溃**。
3. **下载类命令**：进度行 `data` 非 null（`index`/`total`/`file`/`status`/`file_size`/`error`）；结束行 `data: null`；失败行 `isSuccess:false`。
4. **stdout 为空 + 非 0 退出码** → 判定为 `flag.ExitOnError`（未知参数会 `os.Exit(2)`）或进程崩溃；把 stderr 作为诊断信息展示。
5. **stdout 有 JSON 但退出码非 0** → 以 JSON 为准，退出码仅记日志（与 C# SDK 一致）。
6. **stderr 一律只记日志**，不直接展示（内容是英文诊断，如 `check_update: GetProjectChangeLogs failed: ...`）。

**超时**（v1 未定义）：

| 命令 | 超时 | 说明 |
|------|------|------|
| `check_update` / `check_self_update` / `list_rollback_versions` | 30s | 含 HTTP 请求 |
| `download_update` | 无总超时 + **活性检测** | 进度行间隔 >90s 视为卡死，可取消 |
| `apply_update` / `rollback` | 600s | 内含 3 次重试 ×2s + 深扫（10–60s/路径）+ `close-timeout` |

> ⚠️ **绝不能复刻 C# SDK 的忙等死循环**：`AlyApi.cs:54` 注册了 `process.Exited` 但**未设 `EnableRaisingEvents`**，`cts` 永不取消；`ReadLine()` 返回 null 后循环 100% CPU 死转。UI 必须把 `ReadLine()==nil`（EOF）当作流结束。
> ⚠️ 同理**不能复刻 `RunAsyncAlone`**（`AlyApi.cs:194-214`）：`apply_update` 走它时**不读 stdout、不看退出码，永远返回 OK**。UI 必须读取真实结果。

**`errorMsg` 的本地化问题**（v1 遗漏）：
`aly-client.exe` 的 `errorMsg` 是**硬编码中文**（如 `另一更新正在进行中，请稍后重试`、`待应用版本目录缺失、为空或不完整: ...`）。
→ `en-US` 下 UI 策略：外层显示本地化的通用文案，**原文作为"详细信息"原样展示**（可折叠/小字）。
→ 对少数高频文案做精确匹配映射（至少：`already at latest version` → 无需更新；`另一更新正在进行中` → 已有更新在进行；`no pending update to apply` → 无待应用更新）。其余保持原文。

### 3.4 命令映射（v2 修正，与 `COMMANDS.md` 及源码对齐）

| 场景 | 调用 | UI 实际传入的参数 | 返回 data |
|------|------|-------------------|-----------|
| 自更新检查 | `check_self_update` | **不传**（靠 `client.json`） | `{need_update:bool}` |
| 检查更新 | `check_update` | **不传**（⚠️ 该命令**不接受** `--main-exe-path`） | `CheckUpdateData` |
| 下载 | `download_update` | **不传** | 进度行 + 结束 |
| 应用 | `apply_update` | **不传**（`--close-timeout` 用默认 30s） | 结果 |
| 历史版本 | `list_rollback_versions` | **不传** | `ListRollbackData` |
| 回滚 | `rollback` | 仅 `--version <目标版本>`（必填） | `RollbackData` |

> **设计决策**：所有可选参数一律不传，由 `aly-client.exe` 自己读 `UpdateFolder/client.json` 与 `.updator/shared.json`。
> 这样 UI **无需解析配置、也无需共享 `config/` 包**（符合 AGENTS.md「禁止共享 config/cmd/util」的意图），且与 C# SDK 的实际行为一致（SDK 也从不传这些 flag）。

DTO 结构体**直接共用 `aly/client/aly-client/model`**（同仓库同 GOPATH 编译，非镜像副本，字段名不可能漂移）。仅共享 `model` 包（纯 JSON 结构体，只依赖 `encoding/json`），**禁止共享 `config/`、`cmd/`、`util/`**（避免把磁盘/网络副作用拽进 UI）。C# SDK 为独立 NuGet 交付物，其 Models 镜像副本维持不变。model 包对应定义：

- `CheckUpdateData`: `has_update` / `need_download_update` / `current_version` / `new_version` / **`force_update`（`*bool`，可缺失，见 §3.5）**
- `CheckSelfUpdateData`: `need_update`
- `DownloadProgress`（C# `DownloadProgressData`）: `index` / `total` / `file` / `status` / `file_size` / **`error`（C# 侧没有，v1 漏列）**
- `RollbackListData`（C# `ListRollbackData`）: `current_version` / `versions[]`
- `RollbackData`: `version`
- 外层 `Output`: `isSuccess` / `errorMsg` / `data`

> 已知类型差异（不影响 JSON 解析，仅记录）：Go `RollbackListData` ↔ C# `ListRollbackData`（类名不同）；Go `FileSize int64` ↔ C# `double`。

### 3.5 `force_update` 语义与缺失处理（v2 修正）

**事实**：`model/dto.go:64` 定义为 `ForceUpdate *bool \`json:"force_update,omitempty"\``，即：

- `force_update: true` → 强制；
- `force_update: false` → 非强制；
- **字段整个缺失**（nil）→ 出现在 `downloaded` 续跑、`applying` 崩溃恢复、回滚续跑等分支。

**UI 决策规则**（必须显式实现；不能像 C# SDK 那样把缺失当 false 就算了）：

| 现场 | `force_update` | UI 行为 |
|------|----------------|---------|
| 发现新版本、需下载 | `true` | **自动**下载 → 自动应用，不询问 |
| 发现新版本、需下载 | `false` 或缺失 | 询问（下载前、应用前各一次） |
| `applying`（更新中断/崩溃恢复） | 任意（通常缺失） | **无条件自动继续**，不询问 —— 此时主目录可能已丢失、应用不可用，必须自愈 |
| `downloaded` 续跑 apply | `false` 或缺失 | 询问（应用前） |
| 回滚续跑 | 任意（通常缺失） | **无条件自动继续** |

> **与 C# SDK 的行为差异（需知悉）**：SDK 把缺失读成 `false` 且**不阻塞**，所以"强制更新中断后"在 SDK 下会静默变成"询问"。UI 在 `applying` 分支改为自动续跑，是**有意的改进**，需在验收用例中覆盖。

### 3.6 版本状态机（`version.json`，只读感知）
`applied → downloaded → applying → applied`。UI **不直接改** `version.json`，全部经 aly-client。

---

## 4. UI 设计（walk）

### 4.1 窗口位置与层级

- **启动定位**：窗口出现在屏幕**右下角**。
  - ⚠️ **v2 修正**：v1 拟用 `SystemParametersInfo(SPI_GETWORKAREA)`，但**实测该常量不在 `lxn/win` 中**（`SystemParametersInfo` 函数在，`SPI_GETWORKAREA` 常量不在）。
  - **改用已确认可用的 API**（两者均在 `lxn/win` 中）：
    ```go
    hmon := win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTONEAREST) // 或 MONITOR_DEFAULTTOPRIMARY
    var mi win.MONITORINFO
    mi.CbSize = uint32(unsafe.Sizeof(mi))
    win.GetMonitorInfo(hmon, &mi)
    wa := mi.RcWork        // 工作区：已自动扣除任务栏
    x := int(wa.Right) - width - margin
    y := int(wa.Bottom) - height - margin
    ```
  - `RcWork` 是工作区（已扣除任务栏），优于 `RcMonitor`。
  - 多显示器：`MONITOR_DEFAULTTONEAREST`（已实测存在于 win 包）= 窗口所在屏；如需强制主屏用 `MONITOR_DEFAULTTOPRIMARY`（**常量存在性列入 M0 验证**）。
  - `margin ≈ 16px`；先 `SetBounds` 再 `SetVisible(true)`，避免闪现到左上角。
- **置顶**：`win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0,0,0,0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)` 保持 topmost。
  - `SWP_NOACTIVATE` **不抢焦点**（重要：不要打断全屏主程序 OTDR 的操作）。
  - 不进任务栏 / 不进 Alt-Tab：`GWL_EXSTYLE |= WS_EX_TOOLWINDOW`（`GetWindowLong`/`SetWindowLong`/`GWL_EXSTYLE`/`WS_EX_TOOLWINDOW` 均已实测存在于 win 包）。
- **尺寸**：宽约 360–420px，高度自适应内容。
- **显示策略（2026-10-19 确认）**：
  - 窗口在右下角**创建但默认隐藏**（`SetVisible(false)`），需要时才显示并置顶。
  - **启动场景（`-mode auto`，主程序刚启动时）**：静默检查 →
    - **有更新** → 右下角显示更新界面（**不自动关闭**）；
    - **无更新** → **不显示**（不打扰用户）。
  - **手动检查场景（`-mode check`，运行中有人点"检查更新"）**：右下角弹窗 →
    - **无更新** → 显示"已是最新版本" → **5 秒后自动关闭**；
    - **有更新** → 显示更新界面（**不自动关闭**，等用户操作）。
  - `update` / `rollback` 模式一律显示窗口。
  - ⚠️ 5 秒自动关闭实现：`time.AfterFunc(5*time.Second, ...)`，回调内用 `walk.Synchronize` 关闭窗口；**若期间用户已产生交互（点按、下载已开始）则取消该定时器**，避免误关。
  - ⚠️ 主程序运行中触发"手动检查"的方式：再启动一次 `aly-client-ui.exe -mode check`（配合 §8 单实例：已有实例则把检查请求转给它，见 M5）。

### 4.2 视觉风格：现代化（受 XP 约束）

- **约束**：walk = 原生 Win32 + ComCtl32 v6，XP **无 D2D/DComp/模糊/阴影**；"现代化" = 扁平配色 + 留白 + 统一字体 + 自绘控件，**不含**透明/毛玻璃/自定义阴影标题栏。
- **字体**：9pt 正文、12–14pt 半粗标题；`Segoe UI` → XP 回退 `Tahoma`（XP 不一定装 Segoe UI）。
- **配色**：扁平中性灰阶 + 单一强调色（如 `#0A84FF`）；白底卡片 + 1px 分隔线。
- **圆角（2026-10-19 确认：本期不做）**：窗口用**直角 + 1px 描边**。
  - 原因：`SetWindowRgn` / `CreateRoundRectRgn` **实测不在 `lxn/win` 中**，需自行 `syscall.NewLazyDLL("gdi32.dll")` 声明，属额外 XP 验证成本，本期不做。
  - 内容区（按钮、进度条）的圆角仍可用 `walk.Canvas` 直接绘制（`roundedRectangle` 是 Canvas 绘图原语，与窗口镶边无关），**不受此限制**。
  - 后续如需窗口圆角，作为独立增量任务（gdi32 syscall + XP 验证）。
- **原生控件**：ComCtl32 v6 manifest 使原生控件呈现现代外观；**进度条自绘**（walk `CustomWidget` + `Canvas`/GDI+）。
- **间距**：`padding 12–16px` 分区。
- **图标**：成功/失败用自绘图标。
- **动画**：v1 决定"不做动画（避免 XP 闪烁）"。补充：`AnimateWindow` + `AW_SLIDE` + `AW_VER_NEGATIVE` **实测均存在于 win 包且 XP 支持**，如后续想要滑入效果可直接启用（列为可选）。

### 4.3 窗口与状态（对齐 C# `AlyClientStatus`）

| 状态 | 界面表现 |
|------|----------|
| `None` | 主窗（当前版本、检查按钮） |
| `DiscoveredUpdate` | 提示"发现新版本 vX.Y.Z"；非强制→问"是否下载"；强制→自动进入下载 |
| `DownloadingUpdate` | 进度条（`index/total`、当前文件、`file_size`） |
| `DownloadedUpdate` | "下载完成"；非强制→"是否立即重启以应用"；强制→直接应用 |
| `ApplyUpdate` | "正在应用更新…"，随后提示结果；强制→直接重启应用 |
| 错误 | 本地化通用文案 + 可展开的 `errorMsg` 原文（见 §3.3） |

### 4.4 线程模型

- 所有子进程/更新循环运行在**后台 goroutine**；跨线程更新界面统一用 `walk.Synchronize`（禁止在 goroutine 内直接改控件）。
- 取消：用户点"取消/关闭" → `context.Cancel` + `cmd.Process.Kill()`；关闭 = 退出 UI 进程（对齐 SDK `Cancel()`，但 SDK 的 `Cancel()` 实际不杀子进程，UI 需补上）。

### 4.5 文案（i18n）

`i18n/` 提供 zh-CN / en-US 两套字符串，键名统一；**语言由主程序经 `-lang` 启动参数传入**（见 §3.1），未传参数默认 `zh-CN`。UI **不按系统区域自动切换**，也不在界面内提供语言切换入口。

### 4.6 明确不做（XP + walk 硬限制，v2 新增）

| 项 | 原因（实测） |
|----|--------------|
| 深色模式 | walk 无 `DWMWA_USE_IMMERSIVE_DARK_MODE`/`SetPreferredAppMode`；原生控件无法暗色化 |
| 自定义标题栏 | walk 无 `WM_NCCALCSIZE` / `DwmExtendFrameIntoClientArea` |
| DWM 圆角 / 阴影 / 毛玻璃 | XP 无 DWM |
| Per-Monitor V2 DPI | XP 不支持；manifest schema 为 Win10 1703+ |
| 更换 GUI 库获得"更现代"外观 | 实测：所有 Go 1.10 + XP + 386 可用的 Go GUI 库（含 `gonutz/wui`）都是原生控件包装，**没有更现代的选择** |

**DPI**：不做 Per-Monitor V2。仅按 **96 DPI** 布局；启动时用 `GetDeviceCaps(hdc, LOGPIXELSY)`（walk 的降级路径）取系统 DPI，按比例整体缩放尺寸与字号。manifest 只声明 `dpiAware=true`。

---

## 5. XP + 386 工程化（关键约束）

AGENTS.md 规则 6：Go 1.10、GOPATH 模式、`GOARCH=386`、兼容 XP。

### 5.1 依赖钉选（v2 修正 —— v1 最严重的错误）

**必须钉 4 个依赖**（GOPATH 模式无版本管理，`go get` 会拉 master）：

| 依赖 | 钉选 rev | 不钉的后果（实测） |
|------|----------|-------------------|
| `github.com/lxn/walk` | `bf6b8a4b207d83866fe5cef6ad09cd4fccc6d65c`（2019-05-22） | v1 钉的 `9958a48b` **编译失败**：`messagebox.go:62: undefined: strings.ReplaceAll`（Go 1.12 API） |
| `github.com/lxn/win` | `d1d36f0e4f482d3e75b5145232a2d5f50d76bac4`（2019-07-16） | master 使用 `windows.UTF16PtrToString`（x/sys 2020-05-18 才有）→ 编译失败 |
| `golang.org/x/sys` | `19e00faab6ad547203cd66b19e1281807329cf65`（2019-08-29） | master 声明 `go 1.26.0` 且使用 `//go:build` → 编译失败 |
| `gopkg.in/Knetic/govaluate.v3` | `v3.0.0` | walk 的 `expression.go` 强制依赖，**v1 完全没列**，缺失直接编译不过 |

**已实测通过的组合**（Go 1.10.8 / `GOOS=windows` / `GOARCH=386` / `CGO_ENABLED=0` / `-H=windowsgui`，exit 0）：

```
walk@bf6b8a4b207d83866fe5cef6ad09cd4fccc6d65c
win@d1d36f0e4f482d3e75b5145232a2d5f50d76bac4
golang.org/x/sys@19e00faab6ad547203cd66b19e1281807329cf65
gopkg.in/Knetic/govaluate.v3@v3.0.0
```

**关于 walk 版本的结论（修正 v1 的推理方法）**：
- v1 依据 README 文本选版，**方法错误** —— `9958a48b` 的 README 写 "Go 1.8.x or later"，但代码已要求 ≥1.12。**README 已过时**。
- 顺带：`810455cc` 把 README 改成 "Go 1.11.x or later" 本身也低估了（实际需 1.12）。
- walk master 无 `go.mod`，唯一 tag `release`(2011-08-14) 不可用 —— 这部分 v1 描述正确。
- `lxn/win` **有** `go.mod`（`go 1.12`）且**零 tag**；walk 不钉 win 的 rev，必须手工 `git checkout`。

**vendor 方式**：以 `vendor/` 纳入 4 个包（Go 1.10 GOPATH 模式支持），避免污染全局 GOPATH。
> ⚠️ 新版 Go 的 GOPATH 模式**已禁用 `go get`**（实测报 `go: modules disabled by GO111MODULE=off`），依赖必须手工 `git clone` + `git checkout` 后复制进 `vendor/`。

### 5.2 清单文件（ComCtl32 v6）—— 硬性要求

walk 依赖 Common Controls v6，且 `InitCommonControlsEx` 的返回值被 walk **忽略**，失败是**静默的**。

- XP 自带 comctl32 **v6.0**，因此**有正确 manifest 时** XP 也能得到主题化控件；无 manifest 则退回 5.82（Win95 灰）且 `ICC_LINK_CLASS`(SysLink) 不存在 → `LinkLabel` 创建失败。
- 用 `akavel/rsrc` 生成 `.syso` 嵌入，声明 `Microsoft.Windows.Common-Controls version="6.0.0.0"`。
- ⚠️ **XP 兼容注意**：manifest 里**只写 `dpiAware=true`**，**不要**写 walk 示例里的 `dpiAwareness PerMonitorV2, PerMonitor`（Win10 1703+ schema，XP 的 SxS 解析器可能拒绝）。列入 M0 验证。
- README 警告：exe 一旦在 manifest 就位前启动过，之后补放 manifest 也不生效 —— 必须嵌入为资源后重建。
- ⚠️ 风险：`rsrc` 自身需用 Go 1.10 能编译的版本（M0 验证；退路：`windres` 或手工构造 `.syso`）。

### 5.3 构建（v2 修正）

`-ldflags="-s -w -H=windowsgui"` 生成**无控制台**窗口程序（实测 Go 1.10.8 接受该写法，PE `Subsystem=2 WINDOWS_GUI`）。

**正式构建路径 = 扩展 `deploy-client.go`（WSL + Go 1.10.8）**：

项目真正的 XP 构建路径是 `deploy-client.go`（WSL，`/usr/local/go`，实测 `go1.10.8 linux/amd64`）。
本机另有 `C:\Users\5001494\sdk\go1.10.8`（实测 `go1.10.8 windows/amd64`）。

1. 复制 `client/aly-client` 到 WSL GOPATH（含 `ui/` 与 `vendor/`）；
2. 构建 `aly-client.exe`（既有逻辑不变）；
3. **新增**构建 `aly-client-ui.exe`：
   ```
   cd <wslSrc> && GOROOT=/usr/local/go GOPATH=<wslGopath> \
     GOOS=windows GOARCH=386 CGO_ENABLED=0 \
     /usr/local/go/bin/go build -ldflags="-s -w -H=windowsgui" \
     -o aly-client-ui.exe aly/client/aly-client/ui/winui
   ```
4. 部署：
   - `aly-client.exe` → `ProjectPath`（主程序目录）**和** `UpdateFolder`（既有行为，保持不变）
   - `aly-client-ui.exe` → **`UpdateFolder`**（UI 不放主程序目录，见 §3.0）
   - 同时把 `aly-client-ui.exe` 复制到 `ProjectPath`，作为 §6 自更新的比对源（发布包携带）

> 必须在 GOPATH 下用 import path 编译（见 `BUILD.md` 常见问题）。

**本地校验脚本 `build-ui.bat`（仅作本地快速校验，非发布路径）** 必须**强制校验 Go 版本**，否则会静默产出 XP 无法加载的二进制：

```bat
@echo off
REM 仅用于本地编译校验；正式构建/部署请用 deploy-client.go（WSL + Go 1.10.8）
for /f "tokens=3" %%v in ('go version') do set GOVER=%%v
echo %GOVER% | findstr /b "go1.10." >nul
if errorlevel 1 (
    echo [ERROR] 必须使用 Go 1.10.x（当前 %GOVER%）。XP 兼容要求 Go 1.10。
    echo         本机可用：C:\Users\5001494\sdk\go1.10.8\bin\go.exe
    exit /b 1
)
REM 清理 GOPATH → xcopy 源码（含 ui/ 与 vendor/）→ go build → 回拷
```

> 同理，**既有 `client/aly-client/build.bat` 也需加此校验** —— 它目前会用 PATH 上的 go1.26 产出 PE `SubsystemVer=6.1` 的二进制，XP 直接拒绝加载（这是 §1.3 前置任务的根因）。

### 5.4 构建产物校验（M0 验收项）

| 检查 | 期望 | 方法 |
|------|------|------|
| 位数 | 32 位 | PE `Machine = 0x014C` |
| **XP 可加载** | PE `MajorSubsystemVersion ≤ 5.1`（Go 1.10 实测为 `4.0`） | 读 PE 可选头 |
| 无控制台 | PE `Subsystem = 2 (WINDOWS_GUI)` | 读 PE 可选头 |
| 工具链 | Go 1.10.x | 构建时校验 `go version` |

### 5.5 已核实的 walk/win 事实（用于设计判断）

- ✅ 纯 Go，**无需 cgo**（`CGO_ENABLED=0` 编译通过）；cgo 是可选优化（build tag `walk_use_cgo`）。
- ✅ 仅惰性加载 `user32/comctl32/gdi32/msimg32/gdiplus/kernel32/ole32/oleaut32/shell32/uxtheme/advapi32/comdlg32/...`；**从不引用 `dwmapi`/`shcore`/`uiautomation`**（v1 的相关描述不准确）。
- ✅ 有真实降级路径（`Find() != nil`）：`GetDpiForWindow`→`GetDeviceCaps`、`GetSystemMetricsForDpi`→`GetSystemMetrics` 等。
- ✅ `x/sys` 的 `NewLazySystemDLL` 显式处理 XP（回退 `GetSystemDirectory` + `LOAD_WITH_ALTERED_SEARCH_PATH`）。
- ⚠️ **以上均为编译期证据，XP 运行时行为未验证**（无 XP 主机）。M0/M5 必须真机/虚拟机复测。
- ⚠️ walk 在 386 上默认线程栈偏小（golang/go#20975）；`810455cc` 之前 README 记载的规避手段是 `import _ "runtime/cgo"` 或 `-linkmode internal`。列入 M0 验证。

---

## 6. UI 自更新（v2 新增 —— v1 完全缺失）

### 6.1 背景

`check_self_update` 只比对 `主程序目录/aly-client.exe` ↔ `UpdateFolder/aly-client.exe` 的 SHA256。
C# SDK 里真正执行拷贝的是 `UpdateSelf()`（`AlyUpdateClient.cs:76-133`），源 = `AppDomain.BaseDirectory/aly-client.exe`。
**UI 取代 SDK 后必须自己承担**，且 UI 住在 `UpdateFolder`，`BaseDirectory` 不再是主程序目录 → **源路径算法必须重写**。

### 6.2 主程序目录的定位

`client.json` 在 `UpdateFolder/`，其 `main_exe_relative_path`（如 `../win-x64/YOFC.OTDR3001.exe`）**相对于 `UpdateFolder`** 解析，取父目录即得 `MainFolder`。

> 这需要 UI 读 `client.json`。为遵守 AGENTS.md「禁止共享 `config/`」（该包会拽入 `shared.json`/崩溃恢复等磁盘副作用），UI 在 `app/` 内**自行实现约 20 行的单字段 JSON 读取**，只取 `main_exe_relative_path`。

### 6.3 更新流程

**A. 更新 `aly-client.exe`**（对齐 SDK 语义）：
1. 调用 `check_self_update`；`need_update=false` → 跳过。
2. 源 = `MainFolder/aly-client.exe`，目标 = `UpdateFolder/aly-client.exe`。
3. 复制到 `aly-client.exe.new` → SHA256 校验副本与源一致 → 原子替换（`MoveFileEx(MOVEFILE_REPLACE_EXISTING)`）。
4. 目标被占用时重试 3 次（间隔 1s）；**失败保留旧文件、不清空目标**，写 `aly-client-ui.log`（对齐 SDK 的 bug `#8` 语义）。

**B. 更新 `aly-client-ui.exe` 自身**（v1 未定义；Windows 下不能替换运行中的自身）：
1. 发布包把新版 UI 放在 `MainFolder/aly-client-ui.exe`（比对源）。
2. UI 启动时比对自身与源的 SHA256；一致 → 跳过。
3. 不一致 → 复制为 `UpdateFolder/aly-client-ui.exe.new` 并校验。
4. 派生一个 detached 的 `cmd.exe`，在 UI 退出后执行替换并重启：
   ```
   cmd /c ping -n 3 127.0.0.1 >nul & move /y "<UpdateFolder>\aly-client-ui.exe.new" "<UpdateFolder>\aly-client-ui.exe" & start "" "<UpdateFolder>\aly-client-ui.exe" <原参数>
   ```
5. UI 自身退出；由第 4 步拉起的新实例继续工作。

> ⚠️ 此方案（延迟替换 + 自我重启）需在 M5 用真实发布包验证；退路是"下次由主程序启动时完成替换"（UI 检测到差异时先替换再提示用户重启）。

### 6.4 时机

自更新检查在**启动时、首次 `check_update` 之前**完成（对齐 SDK 构造函数的执行顺序），失败不阻断主流程，仅记日志。

---

## 7. 更新行为与发布边界

- **强制与否**：以 `check_update` 返回的 `force_update` 为准，**并处理字段缺失**（完整规则见 §3.5）。
- `force_update=false`：在下载前、应用前**询问用户**。
  > ⚠️ **这是 UI 的新增行为，不是复刻 SDK**：C# SDK 的 `RequestDownloadUpdate`/`RequestApplyUpdate` 只是同步 event，**不阻塞、不等待用户决定**（见 §0.9 #5）。UI 必须自己实现"阻塞等待用户点按"。
- `force_update=true`：自动下载 → 自动应用，不询问。
- `applying`（崩溃恢复）与回滚续跑：**无条件自动继续**，不询问。
- 服务器不可达：`check_update` 返回 `has_update=false`（**是成功、不是错误**），UI 提示"暂时无法检查更新"，并**复位**残留的"发现新版本/下载中"状态（对齐 SDK 的 `TryClearStatus`）。
- 发布细节（包内容、是否需要静默）随 `force_update` 与服务端下发而定，不在 UI 内固化。

---

## 8. 里程碑

| 阶段 | 内容 | 验收 |
|------|------|------|
| **前置** | 用 Go 1.10.8 重建并提交 `aly-client.exe`；给 `build.bat` 加 go 版本校验 | PE `SubsystemVer ≤ 5.1`，XP 可加载 |
| **M0** | 钉选 4 个依赖 + vendor；`main.go` 最小窗口（右下角 + topmost + `WS_EX_TOOLWINDOW`）；manifest 嵌入；扩展 `deploy-client.go` | Go 1.10.8 + 386 编译通过；PE 校验（§5.4）；**XP 弹出一个右下角 topmost 空窗口不崩溃**；manifest 生效（控件有主题）；386 线程栈问题验证；`rsrc` 可用性验证 |
| **M1** | `bridge/` 调用 aly-client + JSON DTO；错误/退出码/超时契约（§3.3） | 能跑通 `check_update` 并解析；空 stdout、非 JSON、exit 2 均被正确处理 |
| **M2** | `flow/` 状态机（对齐 SDK 循环）+ `force_update` 缺失规则（§3.5） | 按强制 / 非强制 / `applying` 三条路径走通 |
| **M3** | `view/` 主窗 + 进度 + 对话框（右下角/置顶 + 自绘现代化控件 + 按需显示）；i18n 中英 | 手动走完 检查→下载→应用；位置/置顶/样式符合 §4.1–4.2；**`auto` 无更新不显示 / `check` 无更新显示"已是最新版本"并 5s 自动关闭**；en-US 下中文 `errorMsg` 展示策略生效 |
| **M4** | 历史版本列表 + 回滚 | 可列出并回滚到指定版本 |
| **M5** | UI 自更新（§6）；单实例；打包、XP 真机复测、与主程序联调 | 主程序零改动拉起 UI 完成更新；UI 自更新验证通过 |

> **单实例**（v1 未定机制，M5 必做）：用命名互斥体（`CreateMutexW` + `ERROR_ALREADY_EXISTS`）或 `UpdateFolder/.aly-ui.lock`。
> 必要性：`apply_update` 成功后会 `launchMainExe` 重启主程序，主程序又会拉起 UI → 极易产生双实例/双窗口。

---

## 9. 主要风险

| # | 风险 | 缓解 |
|---|------|------|
| 1 | **walk/win/x-sys 与 Go 1.10 兼容**（v1 已踩坑：按 README 选版失败） | 已实测可用组合（§5.1）；M0 首日复验，并在构建脚本中断言 rev |
| 2 | **XP 无真机** | M0/M5 以虚拟机（XP SP3 x86）验证；编译期结论不能替代实机 |
| 3 | **manifest 在 XP 上被 SxS 拒绝** | manifest 只写 `dpiAware=true`；M0 优先验证 |
| 4 | **`rsrc` 与 Go 1.10 兼容** | M0 验证；退路：`windres` 或手工构造 `.syso` |
| 5 | **386 线程栈过小导致崩溃** | 按 walk 旧 README 的规避手段（`import _ "runtime/cgo"` 或 `-linkmode internal`）验证 |
| 6 | **UI 自更新的"延迟替换 + 自我重启"不可靠** | M5 真实发布包验证；退路见 §6.3 |
| 7 | **`errorMsg` 中文硬编码**，en-US 体验不完整 | 高频文案映射 + 原文折叠展示（§3.3）；后续可推动 `aly-client` 输出错误码 |
| 8 | **aly-client 输出契约变更**：DTO 与 `COMMANDS.md` 强耦合 | 变更需同步；UI 共用 `model` 包可自动发现字段漂移 |
| 9 | **DTO 共享耦合**：UI 编译期依赖 `aly/client/aly-client/model` | 约束 `model` 保持纯 JSON 结构体（只依赖 `encoding/json`）；`build-ui.bat` 与 `build.bat` 同树拷贝、同步编译验证 |
| 10 | **强制更新中途关机/断网** | 依赖 `version.json` 状态机与 crash-recovery（`loadSharedFromVersionDirs`）；UI 只做展示，`applying` 分支自动续跑（§3.5） |
| 11 | **"现代化"与 XP 能力落差** | XP 无 D2D/DComp/模糊/阴影，视觉上限受限（§4.2、§4.6）。若追求更强效果需牺牲 XP 兼容，与本项目约束冲突，故不做 |
| 12 | **旧版 Go 工具链获取** | 已确认 WSL `/usr/local/go` = go1.10.8，本机 `C:\Users\5001494\sdk\go1.10.8`；go.dev/dl 归档仍提供 1.10.8 |

---

## 10. 参考文件

- `COMMANDS.md`（**仓库根目录**）— 命令与 JSON 契约
- `client/aly-client/BUILD.md` / `build.bat` — 编译流程
- `client/aly-client/main.go` / `cmd/` / `model/dto.go` / `config/` — CLI 与配置
- `deploy-client.go`（**仓库根目录**）— 真正的 XP 构建/部署路径（WSL + Go 1.10.8）
- `client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/` — `AlyUpdateClient.cs` / `AlyApi.cs` / `AlyClientStatus.cs` / `Models/`
- `aly-client-更新方案分析-20260922.md` — **历史参考**（其 P0 结论已在 `903baab` 修复，不再作为风险依据）
- `AGENTS.md` 规则 6 — XP/Go1.10/386 约束
- walk 相关提交：`bf6b8a4b`（v2 钉选）、`9958a48b`（v1 误钉）、`0121872`（引入 `strings.ReplaceAll`，Go 1.12）、`810455cc`（README 改 "Go 1.11+"）

---

## 11. 确认记录

- [x] §0.1 采用**方案 A**（Go+walk 复刻 C# 流程）— 2026-10-19 确认
- [x] §0.4 UI 目录 `client/aly-client/ui/winui/` — 2026-10-19 确认
- [x] §0.2 语言由主程序经 `-lang` 传入，**未传默认 zh-CN**，UI 不自动切换 — 2026-10-19 确认
- [x] §0.3 强制更新**完全由服务器 `force_update` 决定** — 2026-10-19 确认
- [x] §0.9 DTO 直接共用 `aly/client/aly-client/model`（同仓库编译，非镜像副本）；仅共享 `model`，禁共享 `config/cmd/util` — 2026-10-19 确认
- [x] §4.1 启动定位**右下角**、**topmost**、默认**不抢焦点**；`auto`/`silent` 无更新不弹窗 — 2026-10-19 确认
- [x] §4.2 采用扁平现代化风格（XP 约束内）：自绘进度/图标、ComCtl32 v6、单一强调色 — 2026-10-19 确认
- [x] **§0.5 XP 兼容为硬性要求** — 2026-10-19 复核确认
- [x] **§0.6 `aly-client-ui.exe` 部署到 `UpdateFolder/`** — 2026-10-19 复核确认
- [x] **§0.7 进程启动即在屏幕右下角 topmost 显示** — 2026-10-19 复核确认
- [x] **§0.8 视觉：XP 约束内自绘扁平卡片** — 2026-10-19 复核确认

> **复核结论（2026-10-19，全部已确认）**：
> 1. **§4.1 显示策略** —— 已确认并细化：主程序可传参；启动（`auto`）静默检查、**有更新才显示**；手动（`check`）总是弹窗、**无更新显示"已是最新版本"并 5s 自动关闭**、有更新则显示更新界面。
> 2. **§4.2 圆角** —— **本期不做**，窗口用直角 + 1px 描边（内容区圆角仍可自绘）。
> 3. **§6.3 B UI 自更新方案**（延迟替换 + 自我重启）—— **接受**。
> 4. **§1.3 前置任务**（用 Go 1.10.8 重建 `aly-client.exe`、给 `build.bat` 加 go 版本校验）—— **纳入本计划执行并提交**。

---

## 12. 验收结果（执行期回填）

> 分支 `feat/aly-client-ui`。以下均为**实测**结果，命令与输出可在提交历史中复核。
> 标注「待 XP」的项目需要 Windows XP SP3 x86 真机/虚拟机，本机无法覆盖。

### 12.1 已完成（有实测证据）

| # | 项目 | 结果 | 证据 |
|---|------|------|------|
| 1 | 生产代码用 Go 1.10.8 编译 | 通过 | `GOROOT=/usr/local/go`（go1.10.8）+ `GOOS=windows GOARCH=386 CGO_ENABLED=0`，`BUILD_EXIT=0` |
| 2 | 32 位 + XP 可加载的 PE 头 | 通过 | `Machine=0x014C`、`SubsystemVer=4.0`、`Subsystem=2(GUI)`；Go 1.26 构建的旧产物是 `6.1`，XP 加载器会直接拒绝 |
| 3 | 静态导入表无 XP 之后的 API | 通过 | 仅导入 `kernel32.dll`(29)、`winmm.dll`(2)、`ws2_32.dll`(1)；29 个 kernel32 函数全为 XP 时代接口，无 `GetTickCount64` / `InitializeCriticalSectionEx` 等 Vista+ API |
| 4 | 动态加载的 DLL 均为 XP 自带 | 通过 | comctl32 / advapi32 / gdi32 / msimg32 / comdlg32 / gdiplus / ole32 / oleaut32 / opengl32 / pdh / kernel32 / shell32 / user32 / uxtheme / winspool.drv，无 `dwmapi` / `shcore` / `api-ms-win-*` |
| 5 | 新 API 均有降级路径 | 通过 | `win.GetSystemMetricsForDpi` 用 `LazyProc.Find() != nil` 回退 `GetSystemMetrics`；`win.GetDpiForWindow` 内部走 `GetDC(0)+GetDeviceCaps(LOGPIXELSY)`，不调用 Win10 的 `GetDpiForWindow` |
| 6 | ComCtl32 v6 manifest 已嵌入 | 通过 | exe 内含 `Microsoft.Windows.Common-Controls` 与 `dpiAware` 字符串；未嵌入时 `walk.NewMainWindow()` 报 `TTM_ADDTOOL failed` 且进程 exit 1 |
| 7 | 右下角定位 + topmost + 不进任务栏 | 通过 | Win32 实测：窗口 `(1524,856)-(1904,1016)`、工作区 `(0,0)-(1920,1032)`、右侧/底部间隙均 16px、`exStyle=0x10188` → `TOPMOST=True TOOLWINDOW=True`、无任务栏按钮 |
| 8 | 高 DPI 缩放 | 通过 | 144 DPI 下窗口 570x240（380x160 等比）、间隙 24px（16x1.5）；144 DPI 由 `GetDpiForWindow` 取得 |
| 9 | `-mode auto` 无更新/失败时静默 | 通过 | 2.5s 内退出、`exitCode=0`、**从未出现窗口** |
| 10 | `-mode check` 无更新/失败时 5s 自动关闭 | 通过 | 2.0s 窗口可见 → 7.5s 进程已退出 |
| 11 | `-mode rollback` 列表失败时表现 | 通过 | 2.5s 窗口可见（错误卡片）→ 7.5s 进程退出 |
| 12 | 单实例互斥 | 通过 | 同时启动两个 `-mode check`：第二个立刻退出（`exitCode=0`），第一个仍在运行，**可见 UI 窗口数恰好为 1** |
| 13 | 自绘卡片渲染 | 通过 | 像素级采样：白底 133992px、描边 `#E5E7EB` 四边完整、标题 `#111827`、状态 `#6B7280`；进度条 2/5 时强调色 `#2563EB` 恰好 1872px = 522x9x2/5 |
| 14 | 无标题栏外观 | 通过 | 清 `WS_CAPTION|WS_THICKFRAME|WS_MINIMIZEBOX|WS_MAXIMIZEBOX|WS_SYSMENU` + `SWP_FRAMECHANGED` |
| 15 | 单元测试 | 通过 | `go test ./ui/winui/...` 5 个包全 ok（view / app / bridge / flow / i18n），含 60+ 用例 |
| 16 | 部署链路 | 通过 | `go run deploy-client.go` 构建并部署两个 exe 到 8 个目标；仓库内 `aly-client.exe` 与部署物 SHA256 一致 |
| 17 | 自更新替换语义 | 通过 | 单测：源缺失时**目标文件原样保留**；`MoveFileEx(REPLACE_EXISTING)` 原子替换 |
| 18 | 延迟替换脚本的命令行传递 | 通过 | 真实 spawn 写入「带空格的目录」成功，证明 `SysProcAttr.CmdLine` 原样传递（用 `exec.Command("cmd.exe","/c",...)` 会因 Go 转义引号而失败） |

### 12.2 待 XP 真机复测

| # | 检查 | 说明 |
|---|------|------|
| 1 | SxS 是否接受本 manifest | 只声明 `dpiAware`，刻意未写 `dpiAwareness`/`PerMonitorV2`（Win10 1703+ schema，XP 的 SxS 解析器可能拒绝整个 manifest） |
| 2 | 控件是否有 XP 主题 | 需目视确认 ComCtl32 v6 生效 |
| 3 | 386 线程栈 | 长时间运行 / 大量进度行是否崩溃；若崩溃，按计划尝试 `import _ "runtime/cgo"` 或 `-linkmode internal` 重测 |
| 4 | 界面在 XP 上的观感 | 字体回退 Tahoma、直角描边、扁平配色是否协调 |
| 5 | `uxtheme` / `gdiplus` 的实际加载 | 两者均为 XP 自带（SP1+），但需实机确认无加载失败 |

### 12.3 待可用更新服务器

| # | 检查 | 说明 |
|---|------|------|
| 1 | 真实下载 | `download_update` 的进度行解析、进度条推进、90s 活性检测 |
| 2 | 真实应用 | `apply_update` 的目录改名、主程序重启、`applying` 崩溃恢复自动续跑 |
| 3 | 真实回滚 | `list_rollback_versions` 的版本列表、`rollback --version` 执行 |
| 4 | 强制更新全自动 | `force_update=true` 时跳过两次确认 |
| 5 | 与主程序联调 | 主程序把启动 `aly-client.exe` 改为 `..\UpdateFolder\aly-client-ui.exe -mode auto -lang zh-CN` |

### 12.4 与计划的偏差（执行期修正）

执行中发现并已修正的计划问题，详见 `IMPLEMENTATION_PLAN.md` 各任务处的「执行期修正」：

1. **任务 3 依赖任务 4** —— 没有 manifest 时 `walk.NewMainWindow()` 直接失败，故 manifest 必须先做。
2. **任务 8 的结果类型判定** —— 不能按字段零值判断，须按**键存在性**（`has_update` 缺失且 `current_version` 为空时会误判）。
3. **任务 9 的 `exec.ExitError.ExitCode()`** —— Go 1.12 才有，Go 1.10 必须走 `syscall.WaitStatus.ExitStatus()`。
4. **任务 12 的 DPI 坐标** —— walk 的 canvas 是 96-DPI 逻辑坐标，不能自己再缩放（双重缩放会让底部元素不可见）；`SetBounds` 收逻辑值而 `GetMonitorInfo` 给物理值。
5. **任务 14/15 的 `Synchronize`** —— 是**异步**的，计划里「闭包内赋值、闭包外读取」的写法会永远拿到零值（询问框永远返回「否」、版本选择永远返回空）。已改为 channel 阻塞等待。
6. **任务 14 的编排位置** —— 计划的编排代码在后台分支里调用 `w.Run()`，但 walk 消息循环必须在主线程且只能跑一次。
7. **任务 16 的 `spawnDetached`** —— `exec.Command("cmd.exe","/c",cmdLine)` 的引号会被 Go 转义成 `\"`，cmd.exe 不认；须用 `SysProcAttr.CmdLine` 原样传递。
8. **任务 16 的 `replaceFromSource`** —— 「先删后改名」存在空窗期，改为 `MoveFileEx(REPLACE_EXISTING)`。
9. **任务 16 的互斥体命名** —— `Global\` 在 XP 多用户下会因 ACL 失败，改用会话命名空间。
10. **测试策略** —— 既有测试使用 `t.Cleanup`（Go 1.14+），Go 1.10.8 无法编译测试；且交叉编译的测试二进制无法执行。故：**生产代码用 Go 1.10.8 编译，测试用现代 Go 原生跑**。
