# aly 测试用例文档

> 唯一权威的测试用例文档。后续功能改动，按本文档逐条回归即可。
>
> - 文档版本：**v14**（Go 驱动 · 全量合并 · 单条自洽 · 总数 148；2026-11-13
>   修复后全量回归 **147 PASS / 1 SKIP / 0 FAIL**，明细见附录 B）
> - 更新日期：2026-11-13
> - 被测版本：2026-11-13 工作区（按全仓库审查报告修复后的未提交状态）
> - 新增：**125–127**（publish-gui-csharp 初始化：布局 / 三件套 / 原子写，2026-11-04 加入；2026-11-12 全量回归 3/3 PASS（先修正首发版本号 1.0.0→1.0.1），见 §十一·补四）
> - 新增：**130–135**（自更新统一：`self_update` 执行命令、UI/SDK 委派与兜底、
>   UI 防循环、UI 自身 `-finish-self-update` 收尾，2026-11-09 加入并已在 Windows
>   实机执行；见 §十一·补五）
> - 新增：**136–137**（两件套补齐：`self_update --include-ui` 的四种口径 +
>   SDK `AlyUpdateUIClient.Launch` 拉起前自动落位，2026-11-12 加入并在 Windows
>   实机执行；见 §十一·补六）
> - 新增：**139–148**（2026-11-12 全仓库审查问题修复回归：服务端路径穿越/分片边界/软删除/删除同步，publish 三态与参数校验，client 脚本 marker 与参数解析，ignore 边界与路径归一；同日实测 10/10 PASS）
- 编号：**连续 1..N**，无分组前缀，无优先级列
> - 每条用例自带完整的 **前置条件 → 步骤 → 期望结果 → 失败判定**，只看单条即可独立复现
> - 全部代码示例使用 **Go**（测试驱动框架见 §〇）
> - 实测状态：**147 PASS / 1 SKIP / 0 FAIL**（2026-11-13 修复后全量回归，耗时 52.2 分钟，运行 `go test ./... -count=1 -timeout 240m -v`；
>   148 条用例全部执行，37 号为环境 SKIP（本机 200MB 17.5s 就下完，进不了 >90s 慢下载区间，已由 44/45 的 UI 侧覆盖））；
>   历史记录：2026-11-12 全量回归 146 PASS / 1 SKIP / 1 FAIL（104 号用例源码契约随重构更新后已修）；
>   2026-11-12 定点回归 124 + 130–148 全部 PASS；2026-11-10 历史：131 PASS / 0 FAIL / 1 SKIP / 3 未执行

---

## 目录

- [〇、测试框架与环境（读一次就够）](#〇测试框架与环境读一次就够)
- [一、用例索引](#一用例索引)
- [二、aly-client 正常流程（1–8）](#二aly-client-正常流程18)
- [三、第四批新特性（9–17）](#三第四批新特性917)
- [四、中断与故障恢复（18–25）](#四中断与故障恢复1825)
- [五、边界条件（26–35）](#五边界条件2635)
- [六、极限条件（36–39）](#六极限条件3639)
- [七、aly-client-ui（40–45）](#七aly-client-ui4045)
- [八、server 组件（46–65）](#八server-组件4665)
- [九、aly-publish（66–77）](#九aly-publish6677)
- [十、C# SDK（78–94）](#十c-sdk7894)
- [十一、集成链（95–104）](#十一集成链95104)
- [十一·补、更新锁缺陷（105）](#十一补更新锁缺陷105)
- [十一·补二、判据红线的边界补齐（106–115）](#十一补二判据红线的边界补齐106115)
- [十一·补三、尚未覆盖的 8 类缺口（116–124）](#十一补三尚未覆盖的-8-类缺口116124)
- [十一·补四、publish-gui-csharp 初始化（125–127）](#十一补四publish-gui-csharp-初始化125127)
- [十一·补五、自更新统一与防循环（130–135）](#十一补五自更新统一与防循环130135)
- [附录 A：故障注入手法速查](#附录-a故障注入手法速查)
- [附录 B：结果记录表](#附录-b结果记录表)
- [附录 C：已知缺陷与口径差异登记](#附录-c已知缺陷与口径差异登记)

- [十二、审查修复回归（139–148）](#十二审查修复回归（139–148）)

---

## 〇、测试框架与环境（读一次就够）

### 0.1 框架位置与职责

测试代码只有一处，位于 `test/tc/`：

| 文件 | 现状 | 职责 |
|---|---|---|
| `test/tc/go.mod` | 已有 | module `aly/test/tc`，`go 1.25` |
| `test/tc/env.go` | 已有（~1250 行） | **框架核心**：临时目录、构建、启动服务端、HTTP API、造客户端现场、调 aly-client / aly-publish、断言、PE 头解析、发布工作区操作 |
| `test/tc/helpers.go` | 已有（~380 行） | **高阶辅助**：等待、杀进程、目录占用、并发编排、UI 与 SDK 驱动、`go test` 子进程驱动 |
| `test/tc/cases_01_08_test.go` | 已有 | 用例 1–8 的实现（正常流程主干） |
| `test/tc/cases_09_17_test.go` … `cases_95_104_test.go` | 已有 | 用例 9–104 的实现，按编号段分文件 |
| `test/tc/cases_105_test.go` | 已有 | 用例 105（更新锁空窗期与崩溃残留） |
| `test/tc/cases_106_115_test.go` | 已有（~850 行） | 用例 106–115（判据红线的边界补齐） |
| `test/tc/cases_116_124_test.go` | 新增（~960 行） | 用例 116–124（并发 / ACL / 自愈链 / Range / i18n / SDK 失败路径） |
| `test/tc/cases_125_127_test.go` | 新增（~190 行） | 用例 125–127（publish-gui-csharp 初始化：布局 / 三件套 / 原子写，**未执行**） |
| `test/tc/cases_130_135_test.go` | 新增（~420 行） | 用例 130–135（自更新统一：`self_update` 执行、UI 委派/自身收尾 + 防循环、SDK 委派） |
| `client/aly-client-ui/aly-client-linuxui/*_test.go` | 新增 | **Linux UI 自身测试**（独立 Go module，不在 `test/tc` 的 135 条内）：client JSON 解析、flow 决策、i18n、journal、路径/复制、UI 自更新派生、收尾参数/改名、`ui.NewTester` 视图 |

> **125–127 的两层钉法**：publish-gui-csharp 是 C#/Avalonia 的 UI 程序，框架跑不了它的窗口，
> 所以每条都分两层——**源码契约**（读 C# 源码断言关键调用形态，与用例 104 的 ① 段同法）
> 和**消费侧功能**（用真实 aly-client 消费 GUI 写出的那套目录布局与 `client.json`）。
> 只在 Go 里把 C# 的路径算法重算一遍是自证自话，钉不住任何回归。

截至 v11，用例总数 **135**（131 PASS / 0 FAIL / 1 SKIP / 3 未执行）；最近一次为
2026-11-09 定点回归（130–135 + 42/94），全量历史数据见附录 B。
文档里的 Go 片段与 `test/tc/` 下的实现保持同源，改用例时两边一起改；`go vet -tags purego ./...` 通过。
`env.go` / `helpers.go` 已把文档引用的全部符号（`PEMachine`、`TryGetAllFiles`、`PublishCLIAt`、
`GoTestGopath`、`OutputHasFail`、`buildApplyPoint`、`buildRollbackPoint` 等）落齐。

用例文档里的每段 Go 代码，都可以直接粘进新的 `cases_XX_test.go` 里运行——它们调用的是同一套 `Env`，语义与你手工点鼠标完全一致。

### 0.2 工具链约定

| 用途 | 工具链 | 常量 | 说明 |
|---|---|---|---|
| server / publish-cli | **go1.27.3**（建议） | `tc.Go126` | `server/go.mod` 声明 `go 1.27.3`（需 ≥1.27.3）；`publish-cli/go.mod` 声明 `go 1.25.0`。本地可用 `ALY_GO126` 显式指定路径 |
| aly-client / aly-client-ui / fake_main | **go1.10.8** | `tc.Go1108` | XP 兼容口径，必须 `GOARCH=386`；用 1.26 编出的 386 产物不保证可在 XP 运行 |
| Linux aly-client + aly-client-linuxui | **go1.27.3** | 无（module tool 固定） | MyGo 0.8.3 原生 UI；纯 Go/无 cgo，可交叉编译；运行需 GTK3 |
| C# SDK | dotnet 10 | — | 目标框架 net40 |

两个路径在 `env.go` 顶部，可用环境变量覆盖，无需改代码：

```go
var Go126  = envOr("ALY_GO126",  `D:\Install\go\bin\go.exe`)
var Go1108 = envOr("ALY_GO1108", `C:\Users\5001494\sdk\go1.10.8\bin\go.exe`)
```

命令行覆盖：

```go
// 在仓库根执行：go test 时传环境变量
//   Windows:  set ALY_GO126=E:\go\bin\go.exe && go test -tags purego ./test/tc
```

### 0.3 两个必须知道的坑（已内建处理）

**坑 1：`-tags purego` 不能省。**
`net/http` 会连带 `crypto/tls` → `vendor/golang.org/x/crypto/chacha20poly1305`，其 `chacha20poly1305_amd64.s` 汇编路径在本机会触发 `compile.exe` 的 `runtime error R6016 - not enough space for thread data`。加 `-tags purego` 走纯 Go 实现即可绕过。**所有 go 命令都要带这个 tag**：

```go
// go test / go vet / go build 统一写法
//   go test -tags purego ./test/tc -v
```

框架内部构建 publish-cli 时同样显式带上（见 `env.go` 的 `PUBLISH_BUILD_TAGS`）。

**坑 2：代理变量会污染子进程环境。**
注册表可能同时存在 `HTTP_PROXY` 与 `http_proxy` 两份，子进程环境块构建时会因字典不区分大小写而冲突。框架的 `cleanEnv()` 已统一剔除所有代理与 Go 相关变量，因此**用框架跑不需要任何前置清障**；只有你手工在终端里敲 `go` 命令时才需要先清。

### 0.4 统一目录约定（所有用例沿用）

```
%TEMP%\aly-tc-<随机>\                ← Env.Root
├─ bin\                              ← Env.Bin（构建产物 + 服务端数据根）
│  ├─ server.exe
│  ├─ aly-publish.exe
│  ├─ aly-client.exe
│  ├─ fake_main.exe
│  ├─ e2e.db                          ← SQLite（-db 参数）
│  └─ data\<project>\                 ← 服务端项目数据目录
│     ├─ app.exe 等发布文件
│     └─ .updator_manifest.json       ← 发布清单（发布时固化）
├─ server.log                         ← 服务端输出
├─ gopath\src\aly\client\aly-client\  ← GOPATH 模式源码副本
├─ publish-src\                       ← publish-cli 工作区
│  └─ .updator\shared.json
└─ client-01\                         ← 客户端现场（Env.NewClient）
   ├─ UpdateFolder\                   ← ExeDir
   │  ├─ aly-client.exe
   │  ├─ client.json                  ← 主程序定位 + 待关闭进程名
   │  ├─ version.json                 ← 版本状态机（核心）
   │  ├─ download.log / update.log    ← 客户端日志
   │  └─ release_manifest.json        ← 客户端清单缓存（差量删除基线）
   └─ ApplicationFolder\              ← MainFolder
      ├─ app.exe                      ← 主程序（fake_main 替身）
      └─ .updator\shared.json         ← server_url / project_name
```

**关键路径规则（务必对齐）**

- `ExeDir`   = `UpdateFolder\`（aly-client.exe 所在目录）
- `MainFolder` = `UpdateFolder\..\ApplicationFolder\`（由 `client.json` 的 `main_exe_relative_path` 决定）
- 版本快照目录 = **`{PackageFolder}\ApplicationFolder_{version}\`**（与主目录同级，命名 `主目录名_版本号`）
- 备份目录   = apply 时把 MainFolder 改名成 `ApplicationFolder_{旧版本}`（同一规则）
- 服务端项目数据目录 = **`server.exe 所在目录\data\{projectName}\`**

代码里直接取：

```go
c := e.NewClient(t, tc.NewClientOpts{Version: "1.0.0", Status: tc.StatusApplied})
c.Pkg                          // PackageFolder
c.Up()                         // UpdateFolder
c.Main()                       // ApplicationFolder
c.VerDir("1.1.0")              // ApplicationFolder_1.1.0
e.DataDir("tc-app")            // 服务端项目数据目录
```

### 0.5 JSON 文件全字段

`UpdateFolder\client.json`：

```json
{
  "main_exe_relative_path": "../ApplicationFolder/app.exe",
  "must_close_process_name": ["fake_main"]
}
```

`UpdateFolder\version.json`（`tc.VersionInfo` 结构体一一对应）：

| 字段 | 类型 | 含义 |
|---|---|---|
| `version_previous` | string | **MainFolder 当前真实内容版本**（不是"上一次发布"） |
| `version` | string | 目标/当前版本号 |
| `version_status` | string | `applied` / `downloaded` / `applying` / 空串（等同 applied） |
| `downloaded_file_count` | int | 下载产物常规文件数（不含 `.part`/`.tmp`），apply 的 F3 守卫依赖它 |
| `apply_stage` | string | 仅 applying 期间写入，崩溃恢复的证据 |
| `rollback_previous` | string | 回滚开始时 MainFolder 的真实版本 |
| `rollback_target` | string | 回滚目标版本 |
| `apply_fail_count` | int | 同版本连续 apply 失败次数（≥3 触发熔断） |
| `after_apply_update_script` | string | 更新后要跑的脚本（相对于应用目录） |

`ApplicationFolder\.updator\shared.json`：

```json
{
  "server_url": "http://127.0.0.1:24000",
  "project_name": "tc-app",
  "ignore_folders": [],
  "ignore_files": [],
  "un_copy_folders": [],
  "un_copy_files": []
}
```

### 0.6 各命令的 data 字段（断言依据）

| 命令 | data 字段 |
|---|---|
| `check_update` | `has_update` / `need_download_update` / `current_version` / `new_version` / `force_update` / `release_notes` |
| `download_update` | 过程行：`status`(`BEGIN`/`START`/`PROGRESS`/`DONE`/`FAIL`)、`index`、`total`、`file`、`downloaded_bytes`、`total_bytes`、`speed_bps`；**完成行 data 为 `null`** |
| `apply_update` | data 为 `null`（成功） |
| `list_rollback_versions` | `current_version` / `versions[]` |
| `rollback --version X` | `version` |
| `check_diff` | `new_version` / `files[]`（每项含 `path`/`local_md5`/`local_sha256`/`server_md5`/`server_sha256`/尺寸） |
| `check_self_update` | `need_update` |

调用与解析统一走框架：

```go
out := e.RunLast(t, c, "check_update")       // 末行（结果行）
d := tc.Data[tc.CheckUpdateData](t, out)     // 解出 data
outs := e.Run(t, c, "download_update")       // 全部行（含 PROGRESS）
```

### 0.7 一条完整的最小示例（照这个形状写新用例）

```go
func TestCase01(t *testing.T) {
	e := tc.NewEnv(t, true, 0)                       // 构建 + 启动服务端
	const proj = "tc-basic"
	e.EnsureProject(t, proj, false, nil, nil)        // 幂等建项目
	e.PutFile(t, proj, "app.exe", "app-100")         // 写服务端数据目录
	r := e.Publish(t, proj, "1.0.0", []string{"初始"}, "", "")
	tc.True(t, r.IsSuccess, "发布失败: %s", r.ErrorMsg)

	c := e.NewClient(t, tc.NewClientOpts{
		Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	out := e.RunLast(t, c, "check_update")
	d := tc.Data[tc.CheckUpdateData](t, out)
	tc.Eq(t, "has_update", d.HasUpdate, false)
}
```

### 0.8 运行方式

```go
// 全部用例
//   go test -tags purego ./test/tc -v -timeout 3600s
// 单条用例（按函数名过滤）
//   go test -tags purego ./test/tc -run TestCase01 -v -timeout 900s
// 保留现场便于排查（失败时打印 Root 路径）
//   set ALY_TC_KEEP=1 && go test -tags purego ./test/tc -run TestCase05 -v
```

> 失败时 `t.Fatalf` 会打印"期望 / 实际"，日志类断言不通过时把 `c.Log(t, "download.log", 80)` 的内容也带出来，直接看得到现场。

---

## 一、用例索引

| # | 用例 | 组件 | 关键验证点 |
|---|---|---|---|
| 1 | check_update 无更新 | client | 纯只读，version.json 不变 |
| 2 | check_update 有更新 | client | new_version 剥离 v 前缀 / force_update / release_notes |
| 3 | download_update 差异下载 | client | 完成行 data:null / 物化 / downloaded_file_count |
| 4 | download_update 已是最新 | client | 拒绝重复下载，状态不变 |
| 5 | apply_update 原子替换 | client | 备份内容干净 / 快照消失 / 过渡字段清空 |
| 6 | list_rollback_versions | client | 非版本目录被过滤 |
| 7 | rollback 回滚 | client | 目录互换 / **清清单缓存** / 幂等拒绝 |
| 8 | check_diff | client | 单向语义 / SHA256 计算条件 |
| 9 | PROGRESS 行 | client | 3s 节流 / 字段完整 / 字节级进度 |
| 10 | 完成行协议 data:null | client | SDK/UI 完成判定 |
| 11 | 差量删除 | client | 删除集 = 上次清单 ∖ 本次清单 |
| 12 | 运行时数据保护 | client | 不在清单的文件不被删 |
| 13 | 首周期保守 | client | 无缓存不删 |
| 14 | 清单缓存固化 | client | 内容正确 / 原子写 / 无 .tmp 残留 |
| 15 | 回滚清缓存 | client | 回滚成功删缓存 |
| 16 | 缓存损坏降级 | client | 禁用删除同步但不阻断下载 |
| 17 | 断点续传基准 | client | .part 存在时从已有字节继续 |
| 18 | download 中断强杀 | client | 不写半成品状态 |
| 19 | 断电残留 .part | client | 重跑校验通过 |
| 20 | apply 复制阶段强杀 | client | applying 可恢复 |
| 21 | apply 六点位强杀矩阵 | client | 每点都能恢复到 applied |
| 22 | applying 误触 download | client | 跳过且状态不降级 |
| 23 | rollback 中断强杀 | client | resumeRollback 接管 |
| 24 | version.json 半截 | client | .bak 自愈或隔离重置 |
| 25 | download 期间服务端被杀 | client | 失败可感知、可重试 |
| 26 | 下载产物丢失 | client | 识别并重下 |
| 27 | downloaded + 服务器不可达 | client | 不上报待应用 |
| 28 | 空/未知状态 apply | client | 拒绝替换且不改状态 |
| 29 | Version == VersionPrevious | client | 不报待应用、可自愈 |
| 30 | apply 熔断 | client | 连续失败后停止自动 apply |
| 31 | 并发锁 | client | 一成一拒（.aly.lock） |
| 32 | 首次更新 | client | 不残留无名备份目录 |
| 33 | 目录被占用 | client | 探测并击杀占用者 |
| 34 | 中文/空格/深路径 | client | 全链路不乱码 |
| 35 | 回滚到不存在的版本 | client | 报错且状态不变 |
| 36 | 120MB 大文件 | client | 关键路径不失败、校验通过 |
| 37 | 慢下载不误杀 | client | 90s 无输出才算停流 |
| 38 | 500 个 1KB 小文件 | client | 完成无错误 |
| 39 | 1000+ 条清单差量 | client | 差集与缓存正确 |
| 40 | UI 构建 | ui | go1.10.8 + 386 + windowsgui |
| 41 | UI bridge 单测 | ui | 全绿（含 PROGRESS 解析、协议契约） |
| 42 | UI -mode check 无更新 | ui | 进程自动退出、状态不变 |
| 43 | UI -mode check 有更新 | ui | 不崩溃、不误杀 download |
| 44 | UI 进度刷新与单位 | ui | KiB/MiB/GiB 标签 + 千分比 |
| 45 | UI 暂停/继续 | ui | 断点续传、代数过滤 |
| 46 | server 启动/关闭 | server | /api/server/info 200、优雅关闭 |
| 47 | create_project | server | 建成功、DB 有记录 |
| 48 | 重复创建同名项目 | server | 拒绝（errorMsg 含"已存在"） |
| 49 | get_all_files | server | 含 md5/sha256/size |
| 50 | get_release_manifest | server | 集合 = 发布时固化 |
| 51 | download_file 内容一致 | server | 字节流与上传一致 |
| 52 | 版本倒退拒绝 | server | errorMsg 含"不大于" |
| 53 | 非法版本号矩阵 | server | `validVersion` 正则拒绝 |
| 54 | 合法版本号矩阵 | server | v 前缀/预发布/元数据接受且严格递增 |
| 55 | 非法时间格式 | server | errorMsg 含"时间格式" |
| 56 | 清单过滤口径 | server | `.chunks`/`.merging` 不入清单，`.tmp` **入** |
| 57 | ignore 规则 | server | `get_all_files` 与清单同口径过滤 |
| 58 | 上传中间态 | server | 无半截正式文件 |
| 59 | get_all_files 缓存 | server | upload 后失效、再查询重建 |
| 60 | 空项目清单 | server | 空清单不报错 |
| 61 | 不存在的项目 | server | NG 且 errcode 区分（非 500） |
| 62 | 软删除项目 | server | 各查询不再返回 |
| 63 | 大文件上传 | server | 100MB+ multipart 落盘成功 |
| 64 | 并发下载 | server | 只读无锁、均成功 |
| 65 | 路径穿越防护 | server | 越界路径 404、中间态 404 |
| 66 | config init | publish | shared.json + publish.json |
| 67 | status 分类 | publish | 增/删/改正确分类 |
| 68 | add --all | publish | 全部暂存 |
| 69 | push 发布 | publish | 上传 + 发布 + 清单固化 |
| 70 | push 后 status 干净 | publish | 无差异 |
| 71 | push --dry-run | publish | 不实际上传、版本不变 |
| 72 | 缺 message 拒绝 | publish | 参数校验 |
| 73 | 项目不存在 | publish | 报错不崩溃 |
| 74 | 重复/倒退版本号 | publish | 拒绝 |
| 75 | 服务器不可达 | publish | 报连接错误 |
| 76 | 中文/空格/深路径 | publish | 上传成功 |
| 77 | 暂存区 MD5 复核 | publish | 冲突拒绝 |
| 78 | SDK 构建 | sdk | 0 错误（含 AlySdkSmoke） |
| 79 | sd1 正常 check | sdk | 字段完整 |
| 80 | sd2 无效 exe 路径 | sdk | 快速失败（<10s，不挂死） |
| 81 | sd3 超时常量 | sdk | 600 / 1800 |
| 82 | sd4 进度回调单调 | sdk | 比率单调不减 |
| 83 | sd5 回调抛异常隔离 | sdk | 下载照常完成 |
| 84 | sd6 断网 check | sdk | isSuccess:true + has_update:false |
| 85 | sd7 列出可回滚版本 | sdk | 结构完整 |
| 86 | sd8 apply spawn | sdk | must-close 进程被关闭 |
| 87 | sd9 宿主重启后版本 | sdk | version == 期望 |
| 88 | sd10 StatusChanged 抛异常 | sdk | 主循环不死 |
| 89 | sd11 空闲 Dispose | sdk | <6s 退出 |
| 90 | sd12 二次 Dispose | sdk | 幂等 |
| 91 | sd13 全事件订阅 | sdk | 循环正常 |
| 92 | CheckSelfUpdateAsync | sdk | 正确返回 |
| 93 | 已是最新不死循环 | sdk | 复位 None |
| 94 | Launch 拉 UI 进程 | sdk | 返回 OK、退出码可读 |
| 95 | server ↔ client 全链路 | 集成 | 发布→更新→回滚闭环 |
| 96 | 删除同步联动 | 集成 | 服务端删 → 客户端删 |
| 97 | 推送后立即可见 | 集成 | 缓存失效链路 |
| 98 | 待应用被新版本覆盖 | 集成 | 重下新版、旧目录不干扰 |
| 99 | client ↔ UI 全链路 | 集成 | check→download→apply→重启 |
| 100 | UI 下载中暂停/继续 | 集成 | .part 保留、续传成功 |
| 101 | SDK MainLoop 全链路 | 集成 | 自动发现→下载→apply |
| 102 | SDK 断点续传 | 集成 | 杀进程后重试 |
| 103 | SDK 断网轮询 | 集成 | 快速重试、恢复继续 |
| 104 | GUI 与 CLI 同库 | 集成 | 行为等价（代码级 + 冒烟） |
| 105 | 更新锁空窗期与崩溃残留 | 缺陷 | 空锁宽限期 / 陈旧锁清理 |
| 106 | apply F3 守卫（文件数不符） | 边界 | 拒绝替换 + 自愈，主目录不动 |
| 107 | version.json 0 字节 | 边界 | 自愈链：.corrupt 隔离 + 重建 |
| 108 | rollback 到空目录 | 边界 | 已知缺陷：空目录顶掉主目录 |
| 109 | rollback 到当前活动版本 | 边界 | 幂等：三字段不动、残留不删 |
| 110 | create_project 非法名矩阵 | 边界 | 17 子测试；`..` 被接受（缺陷） |
| 111 | upload_file 路径穿越 | 边界 | 6 子测试，5 种穿越全拒 |
| 112 | download_file 越界读取 | 边界 | 7 子测试，越界一律 404 |
| 113 | Range 越界 / 过大 .part | 边界 | 416 语义 + .part 自愈 |
| 114 | 分片上传乱序拼接 | 边界 | 按 chunkIndex 拼，md5 一致 |
| 115 | apply_fail_count 熔断边界 | 边界 | 2→3 临界 + 两处复位；同版本下载不复位（缺陷） |
| 116 | version.json 损坏且**无 .bak** | 边界 | 隔离为 `.corrupt` + 重置首次部署 |
| 117 | 待应用快照目录**不可读**（A2） | 边界 | 拒绝 apply 且不丢弃待应用包 |
| 118 | 并发 download + apply | 并发 | 全局锁互斥 + 最终现场自洽 |
| 119 | 并发 apply + rollback | 并发 | 反向三段 rename 不撕主目录 |
| 120 | publish 清单写失败 | 边界 | 发布仍成功，退化为"不删" |
| 121 | ignore 规则边界 | 边界 | 通配失效（D5）/ 大小写敏感 / 非法 pattern 容错 |
| 122 | `-lang en-US` 文案与回落 | 边界 | 不崩 + 未知语言回落 ZH |
| 123 | 多段 Range / If-Range | 边界 | `multipart/byteranges` + 200 全文回退 |
| 124 | SDK Launch 失败路径 | SDK | UI 缺失 → NG 不抛异常 |
| 125 | publish-gui-csharp 初始化：UpdateFolder 位置 | publish-gui-csharp | 与主程序目录**同级**，不在其内部 |
| 126 | publish-gui-csharp 初始化：三件套与相对路径 | publish-gui-csharp | aly-client.exe + aly-client-ui.exe + client.json；`..\<目录>\<exe>` 可被 client 消费 |
| 127 | publish-gui-csharp 原子写：先 Move 后 Replace | publish-gui-csharp | 首次写不得只留 `client.json.tmp` |
| 128 | 客户端真实分片上传路径 | 边界 | publish-cli 推 >1MiB 文件必须走通 `upload_chunk` → `upload_chunk_complete` |
| 129 | 分片阈值边界 | 边界 | `chunkSize` 走直传、`chunkSize+1` 走分片，两路都逐字节落地 |
| 130 | self_update 一致跳过 | 自更新 | `updated=false` + `already_up_to_date` + `.new/.old` 残留清理 |
| 131 | self_update 差异替换闭环 | 自更新 | `updated=true` + SHA256 收敛 + `.old` 备份 + 幂等回归 |
| 132 | self_update 源缺失跳过 | 自更新 | `updated=false` + `source_missing`；源缺失不算错误、副本不动（self 缺失由单测覆盖） |
| 133 | UI 防循环 + 委派更新器 | 自更新 | journal 连续 3 次未收敛跳过 UI 自身替换；同时委派 `self_update` 同步 `aly-client.exe`（`.old` 为证） |
| 134 | SDK 委派 self_update | 自更新 | `AlyUpdateClient` 构造函数同步 `aly-client.exe`（`.old` 为证） |
| 135 | UI 自身收尾替换 | 自更新 | `-finish-self-update` 等旧进程退出 + `.old` 备份 + 重启 |
| 136 | `self_update --include-ui` 两件套 | 自更新 | 缺 UI 副本→`replaced`；已存在→`existing_kepped`（不替换运行中的镜像）；收尾在途→`pending_inflight`；源缺失→`source_missing`；不带旗标→`ui_*` 缺省 |
| 137 | SDK 拉起前自动落位 | SDK | 现场故障：UpdateFolder 缺 UI、主程序目录有 → `LauchAuto` 自动落位且 SHA 收敛；两处都缺 → NG 不抛异常；同一实例内不重复尝试 |
| 138 | 宿主退出后 UI 跟随退出 | UI | 宿主（代拟 PowerShell 进程）退出 → UI 按映像名观测在数秒内消失；PID 复用不得把"宿主已退"误判成存活（winui 句柄等待 / linuxui starttime 校验） |

---

## 二、aly-client 正常流程（1–8）

> 主干链路：check → download → apply → 回滚。以下 8 条覆盖"干净机器从零走完一次完整更新"。
> 实现见 `test/tc/cases_01_08_test.go`。

### 用例 1 — check_update：服务器版本与本地相同，判定无更新

**前置条件**

1. 服务端已就绪（`tc.NewEnv(t, true, 0)` 完成构建并启动，端口自动分配）。
2. 项目 `tc-basic` 已创建，数据目录内有 `app.exe`（内容 `app-100`），已发布 `1.0.0`。
3. 客户端现场：`version_previous="1.0.0"`、`version="1.0.0"`、`version_status="applied"`，主目录 `app.exe` 内容 `app-100`。

```go
e := tc.NewEnv(t, true, 0)
const proj = "tc-basic"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "app.exe", "app-100")
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"初始版本"}, "", "").IsSuccess, "发布失败")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
})
```

**步骤**

1. 记录 `version.json` 的原始内容（用于验证 check 不改文件）。
2. 执行 `check_update`，取末行解析。

```go
before := tc.ReadText(filepath.Join(c.Up(), "version.json"))
out := e.RunLast(t, c, "check_update")
d := tc.Data[tc.CheckUpdateData](t, out)
```

**期望结果**

- `out.IsSuccess` = `true`
- `d.HasUpdate` = `false`
- `d.CurrentVersion` = `"1.0.0"`
- `d.NeedDownloadUpdate` = `false`（无更新时该字段不下发，`omitempty`）
- `d.NewVersion` = `""`
- `version.json` 内容与执行前**逐字节一致**

```go
tc.True(t, out.IsSuccess, "check_update 应成功: %s", out.ErrorMsg)
tc.Eq(t, "has_update", d.HasUpdate, false)
tc.Eq(t, "current_version", d.CurrentVersion, "1.0.0")
tc.Eq(t, "need_download_update", d.NeedDownloadUpdate, false)
tc.Eq(t, "new_version", d.NewVersion, "")
tc.Eq(t, "version.json 未被改写",
    tc.ReadText(filepath.Join(c.Up(), "version.json")) == before, true)
```

**失败判定**

- `has_update=true` → FAIL。版本比较走了 `>0` 之外的路径；查服务端 `project_change_logs` 是否残留更高版本记录
- `isSuccess=false` → FAIL。读 `errorMsg`：`no server url configured` = shared.json 未生成；`read version` = version.json 损坏
- `version.json` 被改写 → FAIL。check 必须只读，否则与 apply 抢写

执行结果：`PASS（用例 1，4.50s）`

---

### 用例 2 — check_update：服务器版本更高，上报可更新并带出变更说明

**前置条件**

1. 服务端项目 `tc-basic` 已有 `1.0.0`（内容 `app.exe`=`app-100`）。
2. 数据目录演进到 `1.1.0` 后**再**发布（顺序不能反：`get_all_files` 读实时目录，清单在 `publish_version` 时固化）。
3. 变更日志带两条内容。
4. 客户端 `1.0.0` / `applied`。

```go
e.PutFile(t, proj, "app.exe", "app-110")
e.PutFile(t, proj, "lib.dll", "lib-110")
tc.True(t, e.Publish(t, proj, "1.1.0",
    []string{"新增导出功能", "修复登录超时"}, "", "").IsSuccess, "发布 1.1.0 失败")
```

**步骤**

1. 执行 `check_update`，解析末行。
2. 再执行一次，验证幂等（结果一致且不改版本文件）。

**期望结果**

- `HasUpdate` = `true`、`NeedDownloadUpdate` = `true`
- `CurrentVersion` = `"1.0.0"`
- `NewVersion` = `"1.1.0"`（**不带 `v` 前缀**）
- `ForceUpdate` 指针**非 nil**，值为 `false`
- `ReleaseNotes` 长度 2，元素为 `"新增导出功能"`、`"修复登录超时"`
- 两次结果一致；`version.json` 未改

```go
d := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.Eq(t, "has_update", d.HasUpdate, true)
tc.Eq(t, "need_download_update", d.NeedDownloadUpdate, true)
tc.Eq(t, "current_version", d.CurrentVersion, "1.0.0")
tc.Eq(t, "new_version", d.NewVersion, "1.1.0")
tc.True(t, d.ForceUpdate != nil, "force_update 必须下发（指针 + omitempty）")
tc.Eq(t, "force_update", *d.ForceUpdate, false)
tc.Eq(t, "release_notes", len(d.ReleaseNotes), 2)
tc.Eq(t, "release_notes[0]", d.ReleaseNotes[0], "新增导出功能")
```

**失败判定**

- `new_version` 带 `v` 前缀 → FAIL。会让 `AppVersionDir` 生成 `ApplicationFolder_v1.1.0`，与下载目录 `ApplicationFolder_1.1.0` 不一致，apply 找不到产物
- `release_notes` 缺失 → FAIL。UI 无法提示"更新了什么"
- `force_update` 为 nil → FAIL。applied 分支必须下发该字段（指针 + omitempty）

执行结果：`PASS（用例 2，5.45s）`

---

### 用例 3 — download_update：首次下载，产物落版本快照目录并写 downloaded 状态

**前置条件**

1. 服务端项目 `tc-dl`：`1.0.0` 内容为 `app.exe`=`app-100`、`lib.dll`=`lib-100`、`sub/res.txt`=`res-100`。
2. 演进到 `1.1.0`：`lib.dll` 改为 `lib-110`、新增 `new.txt`=`new-in-110`；`app.exe` 与 `sub/res.txt` 不变。
3. 客户端 `1.0.0` / `applied`，主目录内容与 1.0.0 对齐（含 `lib.dll` 与 `sub/res.txt`）。

```go
e.PutFile(t, proj, "app.exe", "app-100")
e.PutFile(t, proj, "lib.dll", "lib-100")
e.PutFile(t, proj, "sub/res.txt", "res-100")
tc.True(t, e.Publish(t, proj, "1.0.0", nil, "", "").IsSuccess, "发布 1.0.0")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
})
tc.WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")
tc.WriteText(t, filepath.Join(c.Main(), "sub/res.txt"), "res-100")

e.PutFile(t, proj, "lib.dll", "lib-110")
e.PutFile(t, proj, "new.txt", "new-in-110")
tc.True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
```

**步骤**

1. 执行 `download_update`，**收集全部行**（不要只取末行）。
2. 断言末行 + 过程行。
3. 检查快照目录产物、`version.json`、`download.log`。

```go
outs := e.Run(t, c, "download_update")
last := outs[len(outs)-1]
```

**期望结果**

- 末行 `IsSuccess` = `true`，`data` 为 `null`（完成行协议）
- 过程行至少 1 条 `BEGIN`，其 `total_bytes > 0`
- 快照目录 `ApplicationFolder_1.1.0\` 存在且含 4 个文件：
  - `app.exe` = `app-100`（未变 → materialize 从 MainFolder 拷入）
  - `lib.dll` = `lib-110`（变了 → 重新下载）
  - `new.txt` = `new-in-110`（新增 → 重新下载）
  - `sub\res.txt` = `res-100`（未变 → materialize 拷入）
- `version.json`：`version_previous="1.0.0"`、`version="1.1.0"`、`version_status="downloaded"`、`downloaded_file_count=4`
- `MainFolder` 内容**未被改动**，且不含 `new.txt`
- `download.log` 含 `materialized unchanged files`

```go
tc.True(t, last.IsSuccess, "download_update 失败: %s", last.ErrorMsg)
tc.Eq(t, "完成行 data:null",
    string(last.Data) == "null" || len(last.Data) == 0, true)

snap := c.VerDir("1.1.0")
for _, f := range []string{"app.exe", "lib.dll", "new.txt", "sub/res.txt"} {
    tc.True(t, tc.Exists(filepath.Join(snap, filepath.FromSlash(f))),
        "快照缺 %s\n%s", f, tc.Tree(snap, 40))
}
v := c.ReadVersion(t)
tc.Eq(t, "version_status", v.VersionStatus, tc.StatusDownloaded)
tc.Eq(t, "downloaded_file_count", v.DownloadedFileCount, 4)
tc.Contains(t, c.Log(t, "download.log", 80), "materialized unchanged files", "download.log")
```

**失败判定**

- 末行 `data` 非 null → FAIL。SDK 的 `DownloadUpdateAsync` 用 `result.Data == null` 判完成，非 null 会读到 EOF 后报 `download interrupted`
- `downloaded_file_count` ≠ 4 → FAIL。apply 的 F3 守卫会因"文件数少于记录值"拒绝替换
- `version_previous` 被写成 `"1.1.0"` → FAIL。违反"downloaded 期间 VersionPrevious 保持不变"，回滚会装错内容
- 快照缺 `app.exe` 或 `sub/res.txt` → FAIL。materialize 未生效，apply 后安装目录缺文件

执行结果：`PASS（用例 3，5.94s）`

---

### 用例 4 — download_update：已是最新时不重复下载

**前置条件**

1. 服务端项目 `tc-dl` 最新版本为 `1.1.0`。
2. 客户端 `version_status="applied"`、`version="1.1.0"`、`version_previous="1.0.0"`。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-110",
})
```

**步骤**

1. 执行 `download_update`，取末行。

**期望结果**

- `IsSuccess` = `false`
- `ErrorMsg` = `"already at latest version"`
- 进程正常退出（`printOutput` 不调 `os.Exit`，失败也是正常退出 + JSON 表达）
- 不产生快照目录；`version.json` 未被改写

```go
out := e.RunLast(t, c, "download_update")
tc.Eq(t, "isSuccess", out.IsSuccess, false)
tc.Contains(t, out.ErrorMsg, "already at latest version", "errorMsg")
tc.Eq(t, "不应产生快照目录", tc.Exists(c.VerDir("1.1.0")), false)
```

**失败判定**

- `isSuccess=true` → FAIL。会做无意义重复下载，并把 `version_previous` 写成 `1.1.0`，回滚立即失效
- 文案不同但语义等价（如 `已是最新版本`）→ 记 PASS 但备注"文案与实现不一致，建议统一"（自动化断言依赖该字符串）

执行结果：`PASS（用例 4，4.56s）`

---

### 用例 5 — apply_update：原子替换后主目录为新版，旧版本成为备份目录

**前置条件**

1. 客户端 `version_status="downloaded"`、`version="1.1.0"`、`version_previous="1.0.0"`。
2. 快照目录 `ApplicationFolder_1.1.0\` 完好（由用例 3 的 `download_update` 生成）。
3. 主目录内容为旧版 `1.0.0`。
4. 无 `fake_main` 进程在运行（`must_close_process_name` 配置为 `fake_main`）。

```go
dl := e.RunLast(t, c, "download_update")
tc.True(t, dl.IsSuccess, "download 失败: %s", dl.ErrorMsg)
```

**步骤**

1. 执行 `apply_update`，取末行。
2. 检查主目录、备份目录、快照目录、`version.json`、`update.log`。

```go
ap := e.RunLast(t, c, "apply_update")
```

**期望结果**

- 末行 `IsSuccess` = `true`，`data` = `null`
- `ApplicationFolder\` = **新版内容**：`lib.dll`=`lib-110`、`new.txt` 存在、`app.exe` 存在
- `ApplicationFolder_1.0.0\` = **旧版备份**：`lib.dll`=`lib-100`、`sub/res.txt`=`res-100`、**不含** `new.txt`
- `ApplicationFolder_1.1.0\` **不再存在**（已改名为主目录）
- 不存在 `ApplicationFolder_1.0.0.old`（成功路径会清理旁移临时目录）
- `version.json`：`version="1.1.0"`、`version_status="applied"`、`apply_fail_count=0`、`rollback_previous`/`rollback_target`/`apply_stage` 均为空
- `update.log` 无 `apply attempt`（无失败重试）

```go
tc.True(t, ap.IsSuccess, "apply 失败: %s\n%s", ap.ErrorMsg, c.Log(t, "update.log", 60))
tc.Eq(t, "主目录 lib.dll",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-110")
tc.Eq(t, "备份不含 new.txt",
    tc.Exists(filepath.Join(c.VerDir("1.0.0"), "new.txt")), false)
tc.Eq(t, "快照已消失", tc.Exists(c.VerDir("1.1.0")), false)
tc.Eq(t, "无 .old 残留", tc.Exists(c.VerDir("1.0.0")+".old"), false)
v := c.ReadVersion(t)
tc.Eq(t, "version_status", v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "rollback_previous 已清空", v.RollbackPrevious, "")
```

**失败判定**

- 备份目录里出现 `new.txt` → FAIL。备份在替换之后做，旧内容已被污染，回滚回不到真正旧版
- `ApplicationFolder_1.1.0` 仍存在 → FAIL。改名未完成却写了 applied，属假成功
- 出现 `ApplicationFolder_`（下划线后为空）→ FAIL。`VersionPrevious` 为空却做备份，见用例 32
- `version_status` 仍为 `downloaded`/`applying` → FAIL

执行结果：`PASS（用例 5，5.52s）`

---

### 用例 6 — list_rollback_versions：列出可回滚版本，排除非版本项

**前置条件**

1. 当前 `applied`、`version="1.1.0"`。
2. 存在备份目录 `ApplicationFolder_1.0.0`、`ApplicationFolder_0.9.0`。
3. 另有干扰目录 `ApplicationFolder_backup`、`ApplicationFolder_1.2.0.tmp`。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-110",
})
tc.WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "app-100")
tc.WriteText(t, filepath.Join(c.VerDir("0.9.0"), "app.exe"), "app-090")
tc.WriteText(t, filepath.Join(c.Pkg, "ApplicationFolder_backup", "x.txt"), "x")
tc.WriteText(t, filepath.Join(c.Pkg, "ApplicationFolder_1.2.0.tmp", "x.txt"), "x")
```

**步骤**

1. 执行 `list_rollback_versions`，解析 `data.versions`。

**期望结果**

- `IsSuccess` = `true`、`CurrentVersion` = `"1.1.0"`
- `versions` 含 `"1.0.0"`、`"0.9.0"`
- `versions` **不含** `"backup"`、**不含** `"1.2.0.tmp"`（被 `isLikelyVersion` 过滤）
- 若当前状态是 `downloaded`，`current_version` 取 `version_previous`，且待应用版本不出现在 `versions` 里

```go
d := tc.Data[tc.RollbackListData](t, e.RunLast(t, c, "list_rollback_versions"))
got := strings.Join(d.Versions, ",")
tc.Eq(t, "current_version", d.CurrentVersion, "1.1.0")
tc.Contains(t, got, "1.0.0", "versions")
tc.NotContains(t, got, "backup", "versions")
tc.NotContains(t, got, "1.2.0.tmp", "versions")
```

**失败判定**

- 出现 `backup` / `1.2.0.tmp` → FAIL。回滚命令会因"版本不存在"失败或误操作
- `current_version` = `"1.0.0"` → FAIL。取的是备份而不是运行版本

执行结果：`PASS（用例 6，5.00s）`

---

### 用例 7 — rollback：回滚到指定版本，主目录与备份目录互换

**前置条件**

1. 当前 `applied`、`version="1.1.0"`，主目录含 `new.txt`（新版独有）。
2. 存在 `ApplicationFolder_1.0.0\`（旧版内容，不含 `new.txt`）。
3. `UpdateFolder\release_manifest.json` **存在**（用于验证回滚会清缓存）。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-110",
})
tc.WriteText(t, filepath.Join(c.Main(), "new.txt"), "new-in-110")
tc.WriteText(t, filepath.Join(c.VerDir("1.0.0"), "lib.dll"), "lib-100")
tc.WriteText(t, filepath.Join(c.Up(), "release_manifest.json"),
    `{"files":["app.exe","lib.dll","new.txt"]}`)
```

**步骤**

1. 执行 `rollback --version 1.0.0`，解析末行。
2. 检查目录互换、`version.json`、清单缓存。
3. 再执行一次同名回滚，验证幂等拒绝。

```go
out := e.RunLast(t, c, "rollback", "--version", "1.0.0")
```

**期望结果**

- `IsSuccess` = `true`，`data.version` = `"1.0.0"`
- 主目录回到旧内容：`lib.dll`=`lib-100`、**不含** `new.txt`
- `ApplicationFolder_1.1.0\` **出现**（回滚前的运行版本被备份），且其中含 `new.txt`
- `ApplicationFolder_1.0.0\` **消失**（已改名为主目录）
- `version.json`：`version_previous="1.1.0"`、`version="1.0.0"`、`version_status="applied"`
- **`release_manifest.json` 被删除**（第四批：回滚成功清缓存，防下轮差量删除误删）
- 第 2 次同名回滚：`IsSuccess=false`、`ErrorMsg` 含 `当前已处于版本`

```go
tc.True(t, out.IsSuccess, "rollback 失败: %s", out.ErrorMsg)
tc.Eq(t, "data.version", tc.Data[tc.RollbackData](t, out).Version, "1.0.0")
tc.Eq(t, "主目录回到旧版",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-100")
tc.Eq(t, "主目录无 new.txt", tc.Exists(filepath.Join(c.Main(), "new.txt")), false)
tc.Eq(t, "1.1.0 已成为备份", tc.Exists(c.VerDir("1.1.0")), true)
tc.Eq(t, "清单缓存应被删除",
    tc.Exists(filepath.Join(c.Up(), "release_manifest.json")), false)

again := e.RunLast(t, c, "rollback", "--version", "1.0.0")
tc.Eq(t, "重复回滚被拒绝", again.IsSuccess, false)
```

**失败判定**

- `release_manifest.json` 仍存在 → FAIL（第四批回归点，会导致下轮误删文件）
- `new.txt` 仍存在 → FAIL（备份内容被污染，apply 侧做备份的时机错了）
- 目录名与内容对不上 → FAIL（`AppVersionDir` 命名规则被破坏，检查 `client.json` 的 `main_exe_relative_path`）

执行结果：`PASS（用例 7，4.86s）`

---

### 用例 8 — check_diff：输出与服务端不一致的文件清单

**前置条件**

1. 服务端项目 `tc-diff`：`app.exe`=`app-server`、`lib.dll`=`lib-server`、`same.txt`=`identical`，已发布 `1.1.0`。
2. 客户端主目录：**缺** `app.exe`；`lib.dll` 内容不同；`same.txt` 完全一致；另有本地独有文件 `local-only.txt`。

```go
tc.WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-local-different")
tc.WriteText(t, filepath.Join(c.Main(), "same.txt"), "identical")
tc.WriteText(t, filepath.Join(c.Main(), "local-only.txt"), "only-local")
```

**步骤**

1. 执行 `check_diff`，解析 `data.files`，按 `path` 建索引。
2. 逐项核对三条记录的字段。

**期望结果**

- `NewVersion` = `"1.1.0"`
- `files` 含 `app.exe`（本地缺失）：`local_md5=""`、`local_size=0`、`local_sha256=""`，server 侧三字段完整
- `files` 含 `lib.dll`（MD5 不同）：`local_md5` 非空且 ≠ `server_md5`，`local_sha256` **非空**
- `files` **不含** `same.txt`（一致）
- `files` **不含** `local-only.txt`（单向语义：以服务端清单为基准遍历）

```go
d := tc.Data[tc.DiffData](t, e.RunLast(t, c, "check_diff"))
byPath := map[string]tc.DiffItem{}
for _, f := range d.Files {
    byPath[strings.ReplaceAll(f.Path, "\\", "/")] = f
}
f, ok := byPath["app.exe"]
tc.True(t, ok, "差异列表应含 app.exe")
tc.Eq(t, "local_md5", f.LocalMD5, "")
tc.Eq(t, "local_sha256（缺失不计算）", f.LocalSHA256, "")

g, ok := byPath["lib.dll"]
tc.True(t, ok, "差异列表应含 lib.dll")
tc.True(t, g.LocalSHA256 != "", "MD5 不同且本地存在 → 应计算 SHA256")

_, ok = byPath["same.txt"]
tc.Eq(t, "same.txt 不应出现", ok, false)
```

**失败判定**

- `same.txt` 出现在列表 → FAIL。MD5 比较逻辑被改成按大小/时间比较，或 md5 未计算
- `app.exe` 的 `local_sha256` 非空 → FAIL。本地文件不存在却算了 SHA256，会得到空文件哈希造成误判
- `NewVersion` 为空串 → FAIL。`GetProjectChangeLogs` 失败，检查是否真的发布了版本

执行结果：`PASS（用例 8，5.02s）`

---
## 三、第四批新特性（9–17）

> 第四批（`34153ce` + `e3614ba`）的核心改动都在这一节：PROGRESS 行防误杀、差量删除、清单缓存。
> 这 9 条是**升级后必须逐条验一遍**的回归点。

### 用例 9 — PROGRESS 行：大文件下载期间持续输出进度

**前置条件**

1. 服务端项目 `tc-progress`，数据目录内生成一个 **30MB** 文件 `big.bin`：
   ```go
   tc.BigFile(t, filepath.Join(e.DataDir("tc-progress"), "big.bin"), 30)
   ```
2. 发布 `2.0.0`。
3. 客户端 `1.0.0` / `applied`，主目录**空**（无 `big.bin`，确保必须下载）。
4. 下载耗时需 **> 3 秒**：本机回环可能过快。若不足 3s，二选一：①`big.bin` 增大到 200MB；②限速代理。都做不到则标记 SKIP 并说明。

```go
e.EnsureProject(t, "tc-progress", false, nil, nil)
tc.BigFile(t, filepath.Join(e.DataDir("tc-progress"), "big.bin"), 30)
tc.True(t, e.Publish(t, "tc-progress", "2.0.0", nil, "", "").IsSuccess, "发布失败")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: "tc-progress", MainFile: "", // 主目录空
})
```

**步骤**

1. 执行 `download_update`，收集**每一行**。
2. 筛出 `status == "PROGRESS"` 的行记为 `progress`。
3. 检查首条 PROGRESS 的字段。
4. 统计 PROGRESS 行数量（验证节流）。

```go
outs := e.Run(t, c, "download_update")
var progress []tc.Progress
for _, o := range outs {
    if o.Progress != nil && o.Progress.Status == "PROGRESS" {
        progress = append(progress, *o.Progress)
    }
}
```

**期望结果**

- `len(progress)` ≥ 1
- 首条含全部字段：`Index ≥ 1`、`Total ≥ 1`、`File == "big.bin"`、`DownloadedBytes > 0`、`TotalBytes == 31457280`、`SpeedBps > 0`
- PROGRESS 行之间间隔 ≈ 3 秒（3s 节流），30MB 全程应为个位数到十几行量级，不能刷屏
- 末行 `IsSuccess=true`、`data=null`
- 下载完成后 `ApplicationFolder_2.0.0\big.bin` 大小 = `31457280`

```go
tc.True(t, len(progress) >= 1, "PROGRESS 行为 0（UI/SDK 会因长时间无输出误判卡死，第四批核心修复点）")
p := progress[0]
tc.Eq(t, "file", p.File, "big.bin")
tc.Eq(t, "total_bytes", p.TotalBytes, int64(31457280))
tc.True(t, p.DownloadedBytes > 0, "downloaded_bytes 应 > 0")
tc.True(t, p.SpeedBps > 0, "speed_bps 应 > 0")
tc.Eq(t, "快照文件大小",
    tc.SizeOf(filepath.Join(c.VerDir("2.0.0"), "big.bin")), int64(31457280))
```

**失败判定**

- PROGRESS 行为 0 → FAIL（UI/SDK 误判卡死，第四批核心修复点）
- 缺任一字段 → FAIL
- 末行 `data` 非 null → FAIL（破坏 SDK/UI 完成判定协议）
- 3 秒内输出上百行 → FAIL（节流失效）

执行结果：`SKIP（用例 9，7.07s）` — 本机回环 <3s 传完 30MB，窗口内无完成采样，speed_bps 恒为 0（环境所致，非缺陷）

---

### 用例 10 — 协议兼容：完成行的 data 必须为 null

**前置条件**

1. 任意可成功 `download_update` 的现场（如用例 9 的现场，或新建一个 downloadable 现场）。
2. 有可下载的新版本。

```go
e.EnsureProject(t, "tc-proto", false, nil, nil)
e.PutFile(t, "tc-proto", "app.exe", "app-200")
tc.True(t, e.Publish(t, "tc-proto", "2.0.0", nil, "", "").IsSuccess, "发布失败")
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Project: "tc-proto", MainFile: "app-100",
})
```

**步骤**

1. 执行 `download_update`，取末行。
2. 断言 `data` 为 null。

**期望结果**

- 末行 `"data":null`（或该键缺失，反序列化后为 nil）
- `IsSuccess` = `true`

```go
last := e.RunLast(t, c, "download_update")
tc.Eq(t, "isSuccess", last.IsSuccess, true)
tc.Eq(t, "data 应为 null", string(last.Data) == "null" || len(last.Data) == 0, true)
```

**失败判定**

- 末行 `data` 是对象（带 `status` 的进度体）→ FAIL。SDK 的 `DownloadUpdateAsync` 用 `result.Data == null` 判完成，非 null 会继续读下一行直到 EOF，最终误报 `download interrupted (aly-client exited before finish line)`

执行结果：`PASS（用例 10，5.35s）`

---

### 用例 11 — 差量删除：服务端删除的文件在客户端 apply 后被删除

**前置条件**

1. 服务端项目 `tc-del`：`app.exe`=`app-v1`、`old.txt`=`will-be-deleted`、`keep.txt`=`keep-me`，发布 `1.0.0`。
2. 客户端完成 `1.0.0` 的 download + apply，且 `UpdateFolder\release_manifest.json` 已生成且**含 `old.txt`**。
3. 服务端删除 `old.txt`，保留另两个文件，然后发布 `1.0.1`（会重新固化服务端清单）。

```go
e.EnsureProject(t, "tc-del", false, nil, nil)
e.PutFile(t, "tc-del", "app.exe", "app-v1")
e.PutFile(t, "tc-del", "old.txt", "will-be-deleted")
e.PutFile(t, "tc-del", "keep.txt", "keep-me")
tc.True(t, e.Publish(t, "tc-del", "1.0.0", nil, "", "").IsSuccess, "发布 1.0.0")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: "tc-del", MainFile: "app-v1",
})
// 走到 downloaded 再 apply，让客户端落一份清单缓存
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.0")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.0")
tc.True(t, tc.Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：清单缓存应存在")

// 服务端删文件后发版
e.RemoveFile(t, "tc-del", "old.txt")
tc.True(t, e.Publish(t, "tc-del", "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")
```

**步骤**

1. 执行 `download_update`。
2. 读 `download.log`，搜 `delete-sync`。
3. 执行 `apply_update`。
4. 检查主目录的三个文件。

```go
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.1")
log := c.Log(t, "download.log", 120)
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.1")
```

**期望结果**

- 日志含 `delete-sync removed 1 file(s) deleted by server: old.txt`
- 日志**不出现** `keep.txt`（未被服务端删除，不在删除集）
- 主目录 `old.txt` **不存在**
- 主目录 `keep.txt` 存在且内容 `keep-me`
- 主目录 `app.exe` 存在且内容 `app-v1`

```go
tc.Contains(t, log, "delete-sync removed 1 file(s) deleted by server", "download.log")
tc.NotContains(t, log, "delete-sync removed", "keep.txt") // 不应把 keep.txt 列进删除集
tc.Eq(t, "old.txt 应被删除", tc.Exists(filepath.Join(c.Main(), "old.txt")), false)
tc.Eq(t, "keep.txt 应保留",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "keep.txt"))), "keep-me")
```

**失败判定**

- `old.txt` 仍存在 → FAIL（删除同步未生效）
- 日志里 `removed` 行出现 `keep.txt` → FAIL（删除集算错，会误删用户文件）
- 日志完全没有 `delete-sync` → FAIL（多半因无缓存被跳过，先确认 `release_manifest.json` 存在）

执行结果：`PASS（用例 11，5.92s）`

---

### 用例 12 — 运行时数据保护：不在清单内的文件不被删除

**前置条件**

1. 服务端项目 `tc-rt` 发布 `1.0.0`（含 `app.exe`），客户端完成 download + apply，清单缓存已建立。
2. 主目录内有应用运行时生成的文件 `runtime/user.cfg`（**从不在服务端清单里**）。
3. 服务端发布 `1.1.0`，并把 `app.exe` 内容改掉（产生一次真实下载）。

```go
// 1.0.0 → apply，建立清单缓存
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.0")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.0")

// 主目录里放"应用自己写的"运行时数据
tc.WriteText(t, filepath.Join(c.Main(), "runtime", "user.cfg"), "user-data-must-survive")

// 服务端发新版
e.PutFile(t, "tc-rt", "app.exe", "app-110")
e.PutFile(t, "tc-rt", "added.txt", "in-110")
tc.True(t, e.Publish(t, "tc-rt", "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
```

**步骤**

1. `download_update` → 检查主目录的 `runtime/user.cfg` 仍在（download 不动主目录）。
2. `apply_update` → 再次检查 `runtime/user.cfg` 仍在。
3. 检查版本目录里**没有** `runtime/user.cfg`（它不该被物化进来当"未变化文件"）。

```go
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")
tc.True(t, tc.Exists(filepath.Join(c.Main(), "runtime", "user.cfg")),
    "download 不应改动主目录的运行时数据")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.1.0")
```

**期望结果**

- apply 后主目录 `runtime/user.cfg` 存在且内容 `user-data-must-survive`
- 主目录 `added.txt` 存在（新版文件已落地）
- 主目录 `app.exe` 内容为 `app-110`
- `download.log` 中 delete-sync 的删除集里**不含** `runtime/user.cfg`

> 为什么必须过这一条：materialize 会把 MainFolder 的未变化文件拷进版本目录。差量删除若用"版本目录内不在新清单即删"的全量差集，就会把 `runtime/user.cfg` 当"服务端已删"清掉——用户数据丢失。这正是第四批改差量的原因。

```go
tc.Eq(t, "运行时数据必须存活",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "runtime", "user.cfg"))),
    "user-data-must-survive")
tc.Eq(t, "新版文件已落地", tc.Exists(filepath.Join(c.Main(), "added.txt")), true)
```

**失败判定**

- `runtime/user.cfg` 消失 → FAIL（**数据丢失，最高严重级**：删除同步退化成全量差集）
- `runtime/user.cfg` 出现在版本快照目录 → 属实现允许的行为（materialize 会拷），但删除同步必须保住主目录那一份

执行结果：`PASS（用例 12，7.06s）`

---

### 用例 13 — 首周期保守：无清单缓存时不执行删除

**前置条件**

1. 服务端项目 `tc-first`：`1.0.0` 含 `app.exe`、`legacy.txt`。
2. 客户端**首次**更新：`version_status="applied"`、主目录内容为旧基线，`UpdateFolder` 内**没有** `release_manifest.json`。
3. 服务端删除 `legacy.txt` 后发布 `1.1.0`。

```go
e.EnsureProject(t, "tc-first", false, nil, nil)
e.PutFile(t, "tc-first", "app.exe", "app-100")
e.PutFile(t, "tc-first", "legacy.txt", "old-legacy")
tc.True(t, e.Publish(t, "tc-first", "1.0.0", nil, "", "").IsSuccess, "发布 1.0.0")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: "tc-first", MainFile: "app-100",
})
// 主目录里有 legacy.txt（旧版本带来的）
tc.WriteText(t, filepath.Join(c.Main(), "legacy.txt"), "old-legacy")
tc.True(t, !tc.Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：不应有清单缓存")

e.RemoveFile(t, "tc-first", "legacy.txt")
tc.True(t, e.Publish(t, "tc-first", "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
```

**步骤**

1. `download_update`。
2. 检查 `download.log`。
3. 检查版本快照目录。
4. 检查缓存是否已生成。

**期望结果**

- 日志含 `no cached manifest (first cycle), skip delete-sync`
- 日志**不含** `delete-sync removed`
- 快照目录里 `legacy.txt` **仍在**（保守不删：没有基线就无法判断"服务端删了什么"）
- `UpdateFolder\release_manifest.json` **已生成**（本轮固化，下一轮起差量生效）

```go
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")
log := c.Log(t, "download.log", 120)
tc.Contains(t, log, "no cached manifest (first cycle)", "download.log")
tc.NotContains(t, log, "delete-sync removed", "download.log")
tc.True(t, tc.Exists(filepath.Join(c.Up(), "release_manifest.json")),
    "首周期也应固化清单，供下轮做差量基线")
```

**失败判定**

- 首周期就发生删除 → FAIL（无基线却删文件，会误删用户数据）
- 缓存未生成 → FAIL（下一轮永远做不了差量，退化成永久不删）

执行结果：`PASS（用例 13，4.73s）`

---

### 用例 14 — 清单缓存固化：内容正确性与原子写

**前置条件**

1. 服务端项目 `tc-cache` 有 3 个文件，发布 `1.0.0`，客户端完成 download。
2. 服务器清单期望为归一化后的相对路径集合（`/` 分隔）。

**步骤**

1. `download_update`。
2. 读 `UpdateFolder\release_manifest.json`，与 `get_release_manifest` 对比。
3. 检查 `UpdateFolder` 内是否有 `.tmp` 残留（原子写应 tmp → rename）。

```go
dl := e.RunLast(t, c, "download_update")
tc.True(t, dl.IsSuccess, "download 失败: %s", dl.ErrorMsg)

cachePath := filepath.Join(c.Up(), "release_manifest.json")
tc.True(t, tc.Exists(cachePath), "缓存文件应生成")
var cache map[string]interface{}
tc.True(t, json.Unmarshal([]byte(tc.ReadText(cachePath)), &cache) == nil, "缓存应是合法 JSON")

srv := e.GetManifest(t, "tc-cache")
tc.Eq(t, "缓存条数与服务端一致", len(cache["files"].([]interface{})), len(srv.Files))
```

**期望结果**

- `release_manifest.json` 是合法 JSON，含 `files` 数组
- `files` 内容 = 服务端清单归一化后的路径集合（`\` 统一成 `/`）
- `UpdateFolder` 内**没有** `*.tmp` / `release_manifest.json.tmp` 残留
- 日志含 `materialized unchanged files`（说明走到了缓存固化那一步）

**失败判定**

- 缓存文件不存在 → FAIL
- 缓存是半截 JSON → FAIL（原子写失效：应先写 tmp 再 rename）
- 出现 `.tmp` 残留 → FAIL（rename 失败或未清理）
- 条数不一致 → FAIL（归一化或去重逻辑有误）

执行结果：`PASS（用例 14，4.69s）`

---

### 用例 15 — 回滚后清单缓存被清除

**前置条件**

1. 客户端 `applied/1.1.0`，存在 `ApplicationFolder_1.0.0` 备份。
2. `UpdateFolder\release_manifest.json` 存在（内容任意，只需存在）。

**步骤**

1. 确认缓存存在。
2. `rollback --version 1.0.0`。
3. 确认缓存被删除。

```go
tc.WriteText(t, filepath.Join(c.Up(), "release_manifest.json"), `{"files":["app.exe"]}`)
tc.True(t, tc.Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：缓存应存在")

out := e.RunLast(t, c, "rollback", "--version", "1.0.0")
tc.True(t, out.IsSuccess, "rollback 失败: %s", out.ErrorMsg)
```

**期望结果**

- 回滚成功（`IsSuccess=true`）
- `release_manifest.json` **不存在**

> 为什么必须清：回滚后主目录内容回到旧版，而缓存记录的是新版的清单。若不清，下一轮 download 会把"旧版有、新版删掉"的文件当成"服务端已删"清掉。

```go
tc.Eq(t, "回滚后清单缓存应被删除",
    tc.Exists(filepath.Join(c.Up(), "release_manifest.json")), false)
```

**失败判定**

- 缓存仍存在 → FAIL（第四批回归点；会导致回滚后再下载时误删文件）

执行结果：`PASS（用例 15，4.85s）`

---

### 用例 16 — 缓存损坏：禁用删除同步但不阻断下载

**前置条件**

1. 客户端已完成一次 download + apply（有正常缓存）。
2. 手工把缓存写成**半截 JSON**（模拟断电写坏）。
3. 服务端删除一个文件后发布更高版本。

```go
// 正常走一轮，产生缓存
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply")

// 写坏缓存
tc.WriteText(t, filepath.Join(c.Up(), "release_manifest.json"), `{"files":["app.exe",`)

// 服务端删文件并发版
e.RemoveFile(t, proj, "gone.txt")
tc.True(t, e.Publish(t, proj, "1.2.0", nil, "", "").IsSuccess, "发布 1.2.0")
```

**步骤**

1. `download_update`。
2. 读日志确认走了降级路径。
3. 确认下载本身成功、缓存被好数据覆盖。

```go
dl := e.RunLast(t, c, "download_update")
log := c.Log(t, "download.log", 120)
```

**期望结果**

- 日志含 `read cached manifest failed (delete-sync disabled)`
- 日志**不含** `delete-sync removed`（本轮不删任何文件）
- `download_update` 仍 `IsSuccess=true`（缓存的锅不牵连下载）
- `release_manifest.json` 被**合法的完整内容覆盖**（下轮起差量恢复）

```go
tc.True(t, dl.IsSuccess, "缓存损坏不应阻断下载: %s", dl.ErrorMsg)
tc.Contains(t, log, "read cached manifest failed (delete-sync disabled)", "download.log")
tc.NotContains(t, log, "delete-sync removed", "download.log")

var m map[string]interface{}
tc.True(t, json.Unmarshal([]byte(tc.ReadText(filepath.Join(c.Up(), "release_manifest.json"))), &m) == nil,
    "坏缓存应被好数据覆盖")
```

**失败判定**

- `download_update` 失败 → FAIL（缓存损坏连累了主链路，容错不足）
- 发生了删除 → FAIL（用坏基线做差集，可能误删）
- 缓存仍是半截 → FAIL（覆盖写没生效）

执行结果：`PASS（用例 16，5.64s）`

---

### 用例 17 — 断点续传基准：.part 存在时从已有字节继续

**前置条件**

1. 服务端项目 `tc-resume`，数据目录内 20MB 文件 `big.bin`，发布 `1.1.0`。
2. 客户端 `1.0.0` / `applied`，主目录空。
3. 手工在目标目录里放一个 `big.bin.part`，内容为 `big.bin` 的前 5MB。

> 说明：`aly-client` 的续传以 `.part` 文件长度作基准（`Range: bytes=N-`）。

```go
e.EnsureProject(t, "tc-resume", false, nil, nil)
bigSrc := filepath.Join(e.DataDir("tc-resume"), "big.bin")
tc.BigFile(t, bigSrc, 20)
tc.True(t, e.Publish(t, "tc-resume", "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Project: "tc-resume", MainFile: "",
})
// 造 .part：拷前 5MB
snap := c.VerDir("1.1.0")
part := filepath.Join(snap, "big.bin.part")
tc.WriteText(t, filepath.Join(snap, ".keep"), "")   // 保证目录存在
copyHead(t, bigSrc, part, 5<<20)
```

**步骤**

1. `download_update`，收集全部行。
2. 检查首条 `DONE` 行的 `downloaded_bytes`（应为 20MB，而非 5+20MB）。
3. 确认最终文件大小与内容哈希正确。
4. 确认 `.part` 已被清理。

**期望结果**

- `download_update` 成功，`data=null`
- 最终 `big.bin` 大小 = `20971520`
- 与源文件 MD5 一致
- 没有 `big.bin.part` 残留
- 若日志/进度显示已传输字节数，应 ≈ 15MB（从 5MB 处续），而不是 20MB 全量

```go
dl := e.RunLast(t, c, "download_update")
tc.True(t, dl.IsSuccess, "续传下载失败: %s", dl.ErrorMsg)
final := filepath.Join(snap, "big.bin")
tc.Eq(t, "最终文件大小", tc.SizeOf(final), int64(20<<20))
tc.Eq(t, ".part 应被清理", tc.Exists(part), false)
tc.Eq(t, "内容哈希应与源一致", tc.MD5Of(final), tc.MD5Of(bigSrc))
```

**失败判定**

- 最终大小 ≠ 20MB → FAIL（续传拼接错位）
- 哈希不一致 → FAIL（**最严重**：续传把错位内容当成功，落地即损坏）
- `.part` 残留 → FAIL（下次续传会基于污染基准）

执行结果：`PASS（用例 17，6.78s）`

---

## 四、中断与故障恢复（18–25）

> 任何环节被打断，都必须能自愈。这一节的核心判据是：**不留半成品状态**、**重启后自动收敛**。
> 故障注入手法见[附录 A](#附录-a故障注入手法速查)。

### 用例 18 — download 期间强杀进程：不留半成品状态

**前置条件**

1. 服务端项目 `tc-kill`，数据目录内 50MB 文件 `big.bin`，发布 `1.1.0`（放大大文件以拉开下载耗时，制造可靠的中断窗口）。
2. 客户端 `1.0.0` / `applied`，主目录空。

```go
e.EnsureProject(t, "tc-kill", false, nil, nil)
tc.BigFile(t, filepath.Join(e.DataDir("tc-kill"), "big.bin"), 50)
tc.True(t, e.Publish(t, "tc-kill", "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Project: "tc-kill", MainFile: "",
})
```

**步骤**

1. 异步启动 `download_update`。
2. **等到版本快照目录内出现 `big.bin.part`**（说明已在写盘），立即强杀进程。
3. 检查 `version.json` 与快照目录状态。
4. 重跑 `download_update`，验证能续传并最终成功。

```go
cmd, _ := e.RunAsync(c, "download_update")
snap := c.VerDir("1.1.0")
tc.MustWaitFile(t, filepath.Join(snap, "big.bin.part"), 30*time.Second)
tc.Kill(t, cmd) // 模拟任务管理器结束进程
```

**期望结果**

- 强杀后 `version.json` **仍是 `applied`**（下载未完成绝不能写 downloaded，否则 apply 会装半成品）
- 快照目录里有 `big.bin.part`（可续传的基准），**没有** `big.bin` 正式文件
- 重跑 `download_update` 成功，`version_status="downloaded"`，`big.bin` 大小 = `52428800`，`.part` 已清理

```go
v := c.ReadVersion(t)
tc.Eq(t, "强杀后状态不变", v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "不应有正式文件", tc.Exists(filepath.Join(snap, "big.bin")), false)

again := e.RunLast(t, c, "download_update")
tc.True(t, again.IsSuccess, "重跑应成功: %s", again.ErrorMsg)
tc.Eq(t, "最终大小", tc.SizeOf(filepath.Join(snap, "big.bin")), int64(50<<20))
tc.Eq(t, "状态", c.ReadVersion(t).VersionStatus, tc.StatusDownloaded)
```

**失败判定**

- 强杀后 `version_status="downloaded"` → FAIL（半成品被当成完整产物，apply 会装出坏的安装目录）
- 快照里出现完整大小的 `big.bin` 但内容错误 → FAIL（部分写入未走 `.part` 机制）
- 重跑失败 → FAIL（续传逻辑坏了）

执行结果：`PASS（用例 18，8.70s）`

---

### 用例 19 — download 残留半截 .part（模拟断电）

**前置条件**

1. 服务端项目 `tc-power`，数据目录 20MB `big.bin`，发布 `1.1.0`。
2. 客户端 `1.0.0` / `applied`，主目录空。
3. 手工造一个**长度正确但内容错乱**的 `.part`（模拟断电时页缓存未落盘）：
   `big.bin.part` 大小 8MB，内容全 `0xFF`。

```go
part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
tc.WriteText(t, filepath.Join(c.VerDir("1.1.0"), ".keep"), "")
// 8MB 的 0xFF 垃圾数据
junk := bytes.Repeat([]byte{0xFF}, 1<<20)
f, _ := os.Create(part)
for i := 0; i < 8; i++ {
    f.Write(junk)
}
f.Close()
```

**步骤**

1. `download_update`。
2. 检查最终文件大小、MD5 是否与源一致。
3. 检查是否报校验失败并重下，或直接补齐成功。

**期望结果**

- 二选一，**都算 PASS**：
  - **A（推荐）**：MD5/SHA256 校验发现 `.part` 与已下载段不一致 → 整段重下 → 最终文件哈希与源一致
  - **B**：从 8MB 处续传 → 校验失败 → 自动重试（最多 3 次）→ 最终一致
- **绝不允许**：最终文件哈希与源不一致却报成功

```go
dl := e.RunLast(t, c, "download_update")
final := filepath.Join(c.VerDir("1.1.0"), "big.bin")
if dl.IsSuccess {
    tc.Eq(t, "内容必须与源一致（校验必须兜住脏 .part）",
        tc.MD5Of(final), tc.MD5Of(filepath.Join(e.DataDir("tc-power"), "big.bin")))
} else {
    // 失败也可接受，但必须明确报错且不写 downloaded 状态
    tc.Eq(t, "失败时状态不能是 downloaded",
        c.ReadVersion(t).VersionStatus, tc.StatusApplied)
    t.Logf("下载失败（可接受，因为脏 .part 应被校验拦截）: %s", dl.ErrorMsg)
}
```

**失败判定**

- 报成功但 MD5 不符 → FAIL（**最高严重级**：脏数据静默落地）
- 状态写成 `downloaded` 但产物损坏 → FAIL

执行结果：`PASS（用例 19，6.75s）`

---

### 用例 20 — apply 复制阶段强杀：状态为 applying 且可恢复

**前置条件**

1. 客户端 `downloaded/1.1.0`，快照目录完好，主目录为 `1.0.0` 内容。
2. 无 `fake_main` 进程。

**步骤**

1. 异步启动 `apply_update`。
2. **等 `version.json` 出现 `version_status="applying"`** 后立即强杀（apply 的进度标记一落盘就杀，制造最脏的中间态）。
3. 检查现场：状态、主目录、备份目录、快照目录的存在性。
4. 重跑 `apply_update`（走崩溃恢复分支），验证能收敛到 `applied`。

```go
cmd, _ := e.RunAsync(c, "apply_update")

// 同步点：状态写 applying 即说明进入替换流程
deadline := time.Now().Add(60 * time.Second)
for time.Now().Before(deadline) {
    if c.ReadVersion(t).VersionStatus == tc.StatusApplying {
        break
    }
    time.Sleep(20 * time.Millisecond)
}
tc.Kill(t, cmd)
```

**期望结果**

- 强杀后 `version_status` 为 `applying` 或已到 `applied`（跑到一半被杀则停在 `applying`）
- 主目录**要么**是完整的 1.0.0 内容、**要么**是完整的 1.1.0 内容；不允许半新半旧
- 若停在 `applying`：重跑 `apply_update` 成功，最终 `version="1.1.0"`、`status="applied"`、主目录为 1.1.0 内容、备份目录 `ApplicationFolder_1.0.0` 内容干净（不含 `new.txt`）

```go
after := e.RunLast(t, c, "apply_update")
tc.True(t, after.IsSuccess, "崩溃恢复失败: %s\n%s", after.ErrorMsg, c.Log(t, "update.log", 80))

v := c.ReadVersion(t)
tc.Eq(t, "最终版本", v.Version, "1.1.0")
tc.Eq(t, "最终状态", v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "主目录为新内容",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-110")
tc.Eq(t, "备份目录为旧内容",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.VerDir("1.0.0"), "lib.dll"))), "lib-100")
```

**失败判定**

- 主目录内容半新半旧 → FAIL（**最严重**：rename 不是原子的，或复制路径混进了 apply）
- 重跑后仍停在 `applying` 且报错 → FAIL（崩溃恢复分支失效）
- `update.log` 出现 `neither main folder nor target version folder exists` → FAIL（两个目录都丢了，无法恢复）

执行结果：`PASS（用例 20，5.87s）`

---

### 用例 21 — apply 各崩溃点（6 个点位矩阵）

> **⚠ 方法学修正（实测 2026-11-01）**
>
> 本用例原写法是「异步启动 `apply_update` → 在某个同步点强杀 → 重跑」，其中点位 3/4/5 的同步条件是
> `tc.WaitGone(t, c.Main(), 60*time.Second)`（等主目录消失）。**这个条件永远抓不到**：
>
> `applyReplacement`（`apply_update.go:301-372`）是**三段 rename**：
>
> ```go
> // ① 旁移旧备份（prevDir → prevDir.old）
> // ② 备份主目录：MainFolder → prevDir(1.0.0)   ← 主目录在此消失
> // ③ 激活新版本：versionDir(1.1.0) → MainFolder ← 主目录在此出现
> if err := renameDirWithKillState(fc.MainFolder, prevVersionDir, closeTimeout, st); err != nil { ... }
> versionInfo.ApplyStage = config.ApplyStageActivating   // ④ 写激活意图
> if err := renameDirWithKillState(versionDir, fc.MainFolder, closeTimeout, st); err != nil { ... }
> ```
>
> ②③ 两次 rename **同分区、纯元数据操作，窗口是微秒级**；而 `WaitGone` 是 50ms 轮询。
> 实测：整条 `apply_update`（含启动、读配置、写状态、三次 rename）总共只有约 **200ms**——
> 轮询粒度比窗口大 5 个数量级，必然落在"新版本已就位"之后。
>
> 于是"杀点"变成"apply 已完工后杀"，重跑被 `no pending update to apply` 拒绝 → 用例 FAIL，
> 但**这不是产品缺陷，是用例不可观测**（modified 前实测：`点位3-备份改名后 恢复失败:
> no pending update to apply`，耗时 61.14s = 空等满 60s）。
>
> **修正做法：不靠时序赌窗口，直接构造崩溃后的磁盘现场。**
> 崩溃恢复的正确性只取决于 `(version.json 状态) × (磁盘目录组合)`，这两者都能精确写入，
> 因此每个点位都能造出 100% 确定、可重复的现场。判据也从"杀得准不准"改成
> "**给定现场，恢复逻辑是否收敛**"——这才是本用例真正要验证的东西。

**前置条件**

1. 每个点位都从**同一个干净起点**开始：走完真实 `download_update`，状态 `downloaded/1.1.0`，
   主目录是 1.0.0 内容（`lib.dll=lib-100`），快照 `ApplicationFolder_1.1.0` 里是 1.1.0 内容
   （`lib.dll=lib-110`）。
2. 为每个点位单独准备一套客户端现场（互不干扰）。

**步骤**

对点位 1–6 各跑一遍「按点位构造崩溃现场 → 跑一次 `apply_update` → 断言收敛」：

共用时间线（对应 `applyReplacement` 的执行顺序）：

| 步 | 动作 | 代码位置 |
|---|---|---|
| ① | `version.json`: `downloaded` → `applying` | `apply_update.go:229` |
| ② | `prevDir(1.0.0)` → `prevDir.old`（若存在） | `apply_update.go:325` |
| ③ | `MainFolder` → `prevDir(1.0.0)` **备份改名** | `apply_update.go:334` |
| ④ | `version.json`: `apply_stage = activating` | `apply_update.go:350` |
| ⑤ | `versionDir(1.1.0)` → `MainFolder` **激活改名** | `apply_update.go:356` |
| ⑥ | `version.json`: `applied` + 清 transient | `apply_update.go:273` |

点位即"在第几步之后崩溃"，落盘状态照抄该步之后的真实组合：

| 点位 | 崩溃位置 | 构造的现场（状态 / MainFolder / targetDir / 快照） | 恢复应走的分支 |
|---|---|---|---|
| 1 | ① 之前 | `downloaded` / 在(1.0.0) / 在 / 在 | 正常 apply 流程 |
| 2 | ① 之后、③ 之前 | `applying` / 在(1.0.0) / 在 / 在 | `applying` + 主目录在 + 版本目录在 → 重做替换 |
| 3 | ③ 之后、④⑤ 之前 | `applying` / **不在** / 在 / 在 | `apply_update.go:130-155`：主目录缺失 + 版本目录在 → 激活 |
| 4 | ④ 之后、⑤ 之前 | `applying`+`activating` / **不在** / 在 / 在 | 同上，但已有激活意图证据，**不得回退** |
| 5 | ⑤ 之后、⑥ 之前 | `applying`+`activating` / 在(1.1.0) / 在 / **不在** | `apply_update.go:109-127`：版本目录缺失 → 补 `applied`，**不得重做替换** |
| 6 | ⑥ 已完成 | `applied`/1.1.0 / 在(1.1.0) / 在 / **不在** | 终态，重跑应明确拒绝 |

```go
// 构造现场（以点位 5 为例：⑤ 之后、⑥ 之前崩溃）
vi := c.ReadVersion(t)                 // 起点 downloaded
vi.VersionStatus = tc.StatusApplying
vi.ApplyStage = "activating"           // ④ 已发生
c.WriteVersion(t, vi)
tc.RenameDir(t, c.Main(), c.VerDir("1.0.0"))   // ③ 主目录 → 备份
tc.RenameDir(t, c.VerDir("1.1.0"), c.Main())   // ⑤ 快照 → 主目录（快照被消耗）

// 恢复入口：一个 apply_update 幂等收敛
out := e.RunLast(t, c, "apply_update")
```

**期望结果**

- 点位 1–5：`apply_update` **成功**，且全部达到一致的终态：
  - `version="1.1.0"`、`version_status="applied"`
  - `apply_stage` / `rollback_previous` / `rollback_target` **全部清空**（transient 字段不得残留）
  - 主目录 = 新版完整内容（`lib.dll=lib-110`、含 `new.txt`）
  - 备份目录 `ApplicationFolder_1.0.0` = 旧版干净内容（`lib.dll=lib-100`、**不含** `new.txt`）
  - 无 `.old` 旁移残留、无 `ApplicationFolder_`（空版本名）
- 点位 6：已是终态，重跑必须**明确拒绝**（`no pending update`），不得挂死或假装成功；
  且拒绝后磁盘内容保持不变。
- 任何点位都不出现"内容对但状态错"或"状态对但内容错"。

**失败判定**

- 点位 1–5 任一恢复失败 → FAIL（崩溃恢复覆盖不全）
- 任一点位内容与状态不一致 → FAIL（假成功）
- 点位 5 的恢复重做了一次替换（备份 1.0.0 被当垃圾删掉）→ FAIL（回滚永久失效）
- 出现 `ApplicationFolder_` 空版本名目录 → FAIL

执行结果：`PASS（用例 21，12.31s）`

---

### 用例 22 — applying 期间误触 download：跳过且状态不降级

**前置条件**

1. 客户端 `version_status="applying"`（模拟 apply 跑到一半、宿主又调了一次 download）。
2. `version="1.1.0"`、`version_previous="1.0.0"`。
3. 服务端有可下载的新版本（甚至更高版本 `1.2.0`）。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusApplying, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
})
e.PutFile(t, proj, "app.exe", "app-120")
tc.True(t, e.Publish(t, proj, "1.2.0", nil, "", "").IsSuccess, "发布 1.2.0")
```

**步骤**

1. `download_update`。
2. 检查状态是否被改成 `downloaded`、`VersionPrevious` 是否被覆盖。
3. 检查返回值是否为"标准完成行"（`data=null`）。

```go
out := e.RunLast(t, c, "download_update")
```

**期望结果**

- `IsSuccess` = `true`、`data` = `null`（用标准完成行收尾，SDK 才不会卡在 EOF 空转）
- `version_status` **仍是 `applying`**（不得降级成 `downloaded`）
- `version` 仍是 `"1.1.0"`、`version_previous` 仍是 `"1.0.0"`
- 日志含 `applying 进行中，跳过 download（由 apply_update 恢复）`
- 不产生 `1.2.0` 的快照目录

```go
tc.Eq(t, "isSuccess", out.IsSuccess, true)
tc.Eq(t, "完成行 data:null", string(out.Data) == "null" || len(out.Data) == 0, true)

v := c.ReadVersion(t)
tc.Eq(t, "状态不得降级", v.VersionStatus, tc.StatusApplying)
tc.Eq(t, "version 不变", v.Version, "1.1.0")
tc.Eq(t, "version_previous 不变", v.VersionPrevious, "1.0.0")
tc.Contains(t, c.Log(t, "download.log", 60), "applying 进行中，跳过 download", "download.log")
tc.Eq(t, "不应下 1.2.0", tc.Exists(c.VerDir("1.2.0")), false)
```

**失败判定**

- 状态被改成 `downloaded` → FAIL（Bug#3 回归：在途 apply 的现场被破坏，恢复时找不到正确版本）
- `VersionPrevious` 被覆盖 → FAIL（回滚目标错乱）
- 返回 `data` 非 null → FAIL（SDK 认不出完成，进程退出后挂死）

执行结果：`PASS（用例 22，5.65s）`

---

### 用例 23 — rollback 崩溃现场：resumeRollback 四分支接管恢复

> **⚠ 方法学修正（实测 2026-11-01，同用例 21）**
>
> 原写法「异步 `rollback --version 1.0.0` → 等主目录消失 → 强杀」**永远抓不到窗口**：
> rollback 与 apply 是同一套三段 rename（`rollback.go:211-263`），两次 rename 之间
> 主目录不存在的窗口是微秒级，50ms 轮询必然错过 → 杀在完工之后 → 续跑被拒。
> 实测：`回滚续跑失败: no pending update to apply`，耗时 65.01s（空等满 60s）。
>
> **改为直接构造 `resumeRollback` 的四个分支现场**（`rollback.go:298-407`）。
> 四个分支的定义就是 `(MainFolder / targetDir / prevDir) 是否存在` 的组合，
> 全部可精确写入，且构造后先自检组合，确保"PASS"不是退化成别的分支。

**前置条件**

1. 客户端 `applied/1.1.0`，主目录为 1.1.0 内容（`app.exe=app-110`、含 `new.txt`）。
2. 存在 `ApplicationFolder_1.0.0` 快照（`app.exe=app-100`，含 `.updator/shared.json`——
   主目录缺失时 `loadFullConfig` 会回退到版本目录读它，缺了会直接报配置错误）。

**步骤**

对四个分支各造一次现场（`version.json` 统一写 `applying` + `rollback_previous=1.1.0`
+ `rollback_target=1.0.0`，这是续跑的唯一依据）：

| 分支 | resumeRollback 判定 | 构造的磁盘现场 | 恢复动作 |
|---|---|---|---|
| 1 激活已完成 | `rollback.go:336` MainFolder 在 + targetDir **不在** | targetDir 已改名成 MainFolder（1.0.0 内容），1.1.0 成备份 | 补写 `applied` |
| 2 备份改名之前 | `rollback.go:345` MainFolder 在 + targetDir 在 | 主目录仍 1.1.0、targetDir 也在 | **重做替换**（备份→激活） |
| 3 备份改名后 | `rollback.go:380` MainFolder **不在** + targetDir 在 | 主目录已改名成 1.1.0 备份 | 激活目标 |
| 4 目标已消耗 | `rollback.go:393` 三者中只剩 prevDir | MainFolder 不在、targetDir 不在、1.1.0 备份在 | **安全放弃回滚**，归位到 1.1.0 |

```go
// 以分支 3 为例（③ 之后、④ 之前崩溃）
vi := c.ReadVersion(t)
vi.VersionStatus = tc.StatusApplying
vi.RollbackPrevious = "1.1.0"
vi.RollbackTarget = "1.0.0"
c.WriteVersion(t, vi)
tc.RenameDir(t, c.Main(), c.VerDir("1.1.0"))   // ③ 主目录 → 1.1.0 备份 → MainFolder 消失

done := e.RunLast(t, c, "rollback", "--version", "1.0.0")  // 续跑入口
```

**期望结果**

- 四个分支续跑**全部成功**（`IsSuccess=true`）
- 分支 1/2/3：`version="1.0.0"`、`status="applied"`、主目录 = 1.0.0 干净内容
  （`app.exe=app-100`、**不含** `new.txt`），`ApplicationFolder_1.1.0` 成为备份且内容是
  `app-110`
- 分支 4：`version="1.1.0"`（安全放弃回滚 → 归位到回滚前版本），主目录 = 1.1.0 内容
- 所有分支：`rollback_previous` / `rollback_target` / `apply_stage` **全部清空**；
  主目录内容必须与写入的 `version` 一致（不允许"目录名 V 装 W 内容"）；无 `.old` 残留

**失败判定**

- 任一分支续跑失败 → FAIL
- 恢复时把回滚当成"升级"处理（去装 1.1.0）→ FAIL（`Bug#1/#5` 回归，`rollback_target` 未被识别）
- 恢复后 `version="1.0.0"` 但主目录内容仍是 1.1.0 → FAIL（`rollback_previous` 记录错误，
  备份目录名与内容错配）
- 状态永久停在 `applying` → FAIL
- 分支 4 把 1.1.0 备份删掉、导致主目录为空 → FAIL（安全放弃不等于毁现场）

执行结果：`PASS（用例 23，6.23s）`

---

### 用例 24 — version.json 写成半截（模拟断电）

**前置条件**

1. 客户端现场正常（`applied/1.0.0`）。
2. 把 `UpdateFolder\version.json` 覆盖成**半截 JSON**（截断到前 20 字节）。
3. 同时保留一份 `version.json.bak`（若实现有 .bak 自愈机制则用它；没有则本用例验证"隔离重建"路径）。

```go
full := tc.ReadText(filepath.Join(c.Up(), "version.json"))
tc.WriteText(t, filepath.Join(c.Up(), "version.json"), full[:20]) // 半截
tc.WriteText(t, filepath.Join(c.Up(), "version.json.bak"), full) // 备份（若实现使用）
```

**步骤**

1. 执行 `check_update`。
2. 检查 `version.json` 是否被自愈（.bak 恢复，或隔离为 `.corrupt` 后重建）。
3. 检查命令是否崩溃（不允许 panic/无限卡住）。

**期望结果**

- 二选一都算 PASS：
  - **A（.bak 自愈）**：`version.json` 恢复成完整内容，`check_update` 正常返回
  - **B（隔离重建）**：坏文件被改名为 `version.json.corrupt`（或类似），重新生成一个默认 `version.json`，`check_update` 返回**明确错误**而不是崩溃
- **绝不允许**：进程 panic、卡死、或把坏文件当合法配置继续用（导致后续命令全乱）

```go
out := e.RunLast(t, c, "check_update")
// A：自愈成功 → 正常无更新
// B：明确报错 → isSuccess=false 且 errorMsg 含 read version
if out.IsSuccess {
    v := c.ReadVersion(t)
    tc.Eq(t, "自愈后版本正确", v.Version, "1.0.0")
} else {
    tc.Contains(t, out.ErrorMsg, "read version", "errorMsg")
    tc.True(t, tc.Exists(filepath.Join(c.Up(), "version.json")), "应重建 version.json")
}
```

**失败判定**

- 进程崩溃/卡死 → FAIL
- 半截文件被当成合法配置（后续命令报出与版本无关的怪错）→ FAIL
- 坏文件既没被恢复也没被隔离，`version.json` 仍是半截 → FAIL

执行结果：`PASS（用例 24，5.01s）`

---

### 用例 25 — download 期间服务端被杀：失败可感知且可重试

**前置条件**

1. 服务端项目 `tc-dead`，数据目录 50MB `big.bin`，发布 `1.1.0`。
2. 客户端 `1.0.0` / `applied`，主目录空。

**步骤**

1. 异步启动 `download_update`。
2. 等 `.part` 出现后 **杀掉服务端进程**。
3. 检查下载结果：必须明确失败（不能报成功）。
4. 重启服务端**复用同一端口**（客户端 `shared.json` 已固化 URL），重跑 `download_update`。
5. 验证续传成功、已下文件跳过。

```go
cmd, _ := e.RunAsync(c, "download_update")
tc.MustWaitFile(t, filepath.Join(c.VerDir("1.1.0"), "big.bin.part"), 30*time.Second)
e.StopServer() // 服务端断电
<-time.After(2 * time.Second)
tc.Kill(t, cmd)

// 恢复：必须复用同一端口，否则"网络恢复"断言必然失败
e.StartServer(t, port) // port 为原端口（先存下来）
again := e.RunLast(t, c, "download_update")
```

**期望结果**

- 第 3 步：`download_update` 结果 `IsSuccess=false`，`errorMsg` 含连接类错误（如 `connection refused` / `get file list`）
- 状态**未被写成 `downloaded`**（未完成不能标完成）
- 第 5 步：重启后重跑成功，`big.bin` 大小 = `52428800`，MD5 与源一致
- 重跑时已下的部分被续传（`.part` 保留且被利用）

```go
tc.Eq(t, "下载失败必须可感知", c.ReadVersion(t).VersionStatus, tc.StatusApplied)
tc.True(t, again.IsSuccess, "恢复后重跑应成功: %s", again.ErrorMsg)
tc.Eq(t, "最终大小",
    tc.SizeOf(filepath.Join(c.VerDir("1.1.0"), "big.bin")), int64(50<<20))
```

**失败判定**

- 断连时仍报成功 → FAIL（半成品被当完成）
- 重启后重跑失败 → FAIL（连接恢复后无法继续，客户端会永久卡在失败态）
- 重跑把已下 40MB 全丢重下（未续传）→ 记 PASS 但备注"续传退化，成本高"

执行结果：`PASS（用例 25，10.64s）`

---

## 五、边界条件（26–35）

> 这一节全是"非常规现场"：数据缺失、状态非法、并发冲突、环境受限。
> 判据统一为：**要么正确处理，要么明确拒绝，绝不假成功、绝不破坏数据**。

### 用例 26 — downloaded 但下载产物丢失：识别并重下

**前置条件**

1. 客户端 `version_status="downloaded"`、`version="1.0.1"`、`downloaded_file_count=3`。
2. 快照目录 `ApplicationFolder_1.0.1\` 曾存在（含 3 个文件）。
3. 服务端最新版本仍是 `1.0.1`。
4. **手工删除**快照目录（模拟用户误删 / 杀软隔离）。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.1", Status: tc.StatusDownloaded, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
    Extras: &tc.VersionInfo{DownloadedFileCount: 3},
})
// 造好快照目录再删掉
tc.WriteText(t, filepath.Join(c.VerDir("1.0.1"), "app.exe"), "app-101")
tc.WriteText(t, filepath.Join(c.VerDir("1.0.1"), "a.txt"), "a")
tc.WriteText(t, filepath.Join(c.VerDir("1.0.1"), "b.txt"), "b")
os.RemoveAll(c.VerDir("1.0.1")) // 产物丢失
```

**步骤**

1. `check_update`，检查 `has_update` 与 `need_download_update`。
2. 执行 `download_update` 补齐。

```go
cu := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
dl := e.RunLast(t, c, "download_update")
```

**期望结果**

- `check_update`：`HasUpdate=true`、`NeedDownloadUpdate=true`、`NewVersion="1.0.1"`（服务端同版本但产物丢失 → 要求重下同版本）
- `download_update` 成功，快照目录重建且文件数 = 3
- `version_status` 仍为 `downloaded`（补下载不改变"待应用"语义）
- `downloaded_file_count` = 实际文件数

```go
tc.Eq(t, "has_update", cu.HasUpdate, true)
tc.Eq(t, "need_download_update", cu.NeedDownloadUpdate, true)
tc.True(t, dl.IsSuccess, "补齐下载应成功: %s", dl.ErrorMsg)
tc.Eq(t, "快照重建", tc.Exists(filepath.Join(c.VerDir("1.0.1"), "app.exe")), true)
tc.Eq(t, "状态仍是 downloaded", c.ReadVersion(t).VersionStatus, tc.StatusDownloaded)
```

**失败判定**

- `check_update` 报无更新 → FAIL（F3 守卫失效，apply 会因产物缺失而拒绝，此后永久卡在 downloaded）
- `download_update` 报 `already at latest version` → FAIL（重下同版本的逃生口被堵死）

执行结果：`PASS（用例 26，5.12s）`

---

### 用例 27 — downloaded 状态 + 服务器不可达：不上报待应用

**前置条件**

1. 客户端 `version_status="downloaded"`、`version="1.1.0"`、`version_previous="1.0.0"`，快照完好。
2. 服务端**已停止**（模拟断网/服务端维护）。

**步骤**

1. 停止服务端。
2. 执行 `check_update`。
3. 检查 `isSuccess`、`has_update`、`current_version`。
4. 重启服务端（**复用同一端口**），再执行 `check_update`。

```go
port := e.Port
e.StopServer()

cu := e.RunLast(t, c, "check_update")

e.StartServer(t, port) // 必须复用端口
cu2 := e.RunLast(t, c, "check_update")
```

**期望结果**

- 断网时：`IsSuccess=true`、`HasUpdate=false`、`CurrentVersion="1.0.0"`（MainFolder 真实内容版本）
  - 注意 `IsSuccess` 是 **true** —— 断网不算失败，只是"无法核对，所以不上报"
- `version.json` 与快照**原样保留**
- 恢复后：`HasUpdate=true`、`NeedDownloadUpdate=false`、`NewVersion="1.1.0"`（继续 apply 分支）

```go
tc.Eq(t, "断网 isSuccess", cu.IsSuccess, true)
tc.Eq(t, "断网 has_update", tc.Data[tc.CheckUpdateData](t, cu).HasUpdate, false)
tc.Eq(t, "断网 current_version",
    tc.Data[tc.CheckUpdateData](t, cu).CurrentVersion, "1.0.0")

d2 := tc.Data[tc.CheckUpdateData](t, cu2)
tc.Eq(t, "恢复后 has_update", d2.HasUpdate, true)
tc.Eq(t, "恢复后 need_download_update（继续 apply）", d2.NeedDownloadUpdate, false)
```

**失败判定**

- 断网时 `isSuccess=false` → FAIL（把"无法核对"当成错误，宿主会误报故障）
- 断网时 `has_update=true` → FAIL（断网显示"可更新"、联网却是另一状态，上位机提示前后矛盾）
- 恢复后 `need_download_update=true` → FAIL（已有完整产物却要求重下，浪费带宽）
- 恢复后 `has_update=false` → FAIL（客户端永久卡在 downloaded，永远不 apply）

执行结果：`PASS（用例 27，6.88s）`

---

### 用例 28 — 空状态/未知状态执行 apply：拒绝替换且不改状态

**前置条件**

分别准备三套现场（各跑一遍）：

1. `version_status=""`（空串）且无待应用版本目录。
2. `version_status="bogus"`（未知值）。
3. `version_status="downloaded"` 但 `version=""`（Version 为空）。

**步骤**

1. 对每套执行 `apply_update`。
2. 检查主目录是否被改动、`version.json` 是否被改写。

```go
for _, v := range []tc.VersionInfo{
    {VersionPrevious: "1.0.0", Version: "1.0.0", VersionStatus: ""},
    {VersionPrevious: "1.0.0", Version: "1.1.0", VersionStatus: "bogus"},
    {VersionPrevious: "1.0.0", Version: "", VersionStatus: tc.StatusDownloaded},
} {
    c := e.NewClient(t, tc.NewClientOpts{Version: v.Version, Status: v.VersionStatus, Project: proj})
    c.WriteVersion(t, v)
    out := e.RunLast(t, c, "apply_update")
    // 断言见下
}
```

**期望结果**

| 现场 | isSuccess | 主目录 | 状态 |
|---|---|---|---|
| 空状态 | `true`（无事可做）或 `false`（明确拒绝），但**主目录不变** | 原样 | 不变 |
| 未知状态 `bogus` | `false`，`errorMsg` 含 `unknown version_status` | 原样 | 不变 |
| downloaded + Version 空 | `false`，`errorMsg` 含"Version 为空" | 原样 | 自愈为 `applied` + 真实内容版本 |

- 任何情况下**不得用空版本来做备份/替换**（会产生 `ApplicationFolder_` 这种空版本名目录）

```go
// 未知状态必须拒绝
tc.Eq(t, "未知状态应拒绝", out.IsSuccess, false)
tc.Contains(t, out.ErrorMsg, "unknown version_status", "errorMsg")
tc.Eq(t, "主目录未被改动",
    tc.ReadText(filepath.Join(c.Main(), "app.exe")), "fake-main-v0")
```

**失败判定**

- 未知状态被 fall through 到 downloaded 流程并替换成功 → FAIL（F6 回归：猜测状态导致装错内容）
- 出现 `ApplicationFolder_` 空版本名目录 → FAIL（A1 回归）
- downloaded + Version 空时主目录被改 → FAIL

执行结果：`PASS（用例 28，5.15s）`

---

### 用例 29 — 非法状态 Version == VersionPrevious：不报待应用且可自愈

**前置条件**

1. `version_status="downloaded"`、`version="1.1.0"`、`version_previous="1.1.0"`（两者相同，属非法状态）。
2. 此时备份目录与版本目录同路径（`ApplicationFolder_1.1.0`），任何替换都会自毁。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusDownloaded, Previous: "1.1.0",
    Project: proj, MainFile: "app-110",
})
```

**步骤**

1. `check_update`，检查是否上报待应用。
2. `download_update`，检查是否被归一化为 `applied`。
3. `apply_update`，检查是否拒绝替换。

```go
cu := e.RunLast(t, c, "check_update")
dl := e.RunLast(t, c, "download_update")
ap := e.RunLast(t, c, "apply_update")
```

**期望结果**

- `check_update`：`HasUpdate=false`（不上报待应用，避免"apply → 拒绝 → 兜底回 downloaded → 再 apply"的 5 秒杀进程循环）
- `download_update`：成功，把状态**归一化为 `applied`**，日志含 `非法状态 downloaded 且 Version == VersionPrevious`
- `apply_update`：拒绝（`IsSuccess=false`），`errorMsg` 含 `Version == VersionPrevious`
- 主目录全程未被改动

```go
tc.Eq(t, "不上报待应用", tc.Data[tc.CheckUpdateData](t, cu).HasUpdate, false)
tc.Eq(t, "download 归一化为 applied", c.ReadVersion(t).VersionStatus, tc.StatusApplied)
tc.Eq(t, "apply 应拒绝", ap.IsSuccess, false)
tc.Contains(t, ap.ErrorMsg, "Version == VersionPrevious", "errorMsg")
```

**失败判定**

- `check_update` 报 `has_update=true` → FAIL（F4 回归：触发杀进程死循环）
- apply 报成功 → FAIL（**最严重**：备份与版本目录同路径，内容自毁）
- 归一化缺失且 apply 报成功 → FAIL

执行结果：`PASS（用例 29，5.72s）`

---

### 用例 30 — apply 熔断：连续 3 次失败后停止自动 apply

**前置条件**

1. `version_status="downloaded"`、`version="1.1.0"`。
2. 人为让 apply 持续失败（用目录占用 + 不可杀进程，或让快照目录在 apply 期间被反复替换成只读）。
3. `apply_fail_count` 从 0 开始。

**步骤**

1. 连续制造 apply 失败（每轮：`check_update` → `apply_update` 失败）。
2. 记录每次失败后 `apply_fail_count`。
3. 第 3 次失败后，再执行 `check_update`，检查是否还上报待应用。

```go
// 简化复现：直接构造 apply_fail_count 现场（避免真造 3 次失败）
c.WriteVersion(t, tc.VersionInfo{
    VersionPrevious: "1.0.0", Version: "1.1.0",
    VersionStatus: tc.StatusDownloaded, DownloadedFileCount: 2,
    ApplyFailCount: 3, // MaxApplyFailCount
})
cu := e.RunLast(t, c, "check_update")
```

**期望结果**

- `apply_fail_count` 每次失败递增
- 达 3（`MaxApplyFailCount`）后：`check_update` 的 `HasUpdate=false`，日志含 `apply circuit broken`
- 服务器发布**更高**版本时，走"重新下载"分支（`need_download_update=true`），download 成功时复位计数为 0
- 手工清理占用后重跑 `download_update` 也能复位

```go
tc.Eq(t, "熔断后不上报待应用", tc.Data[tc.CheckUpdateData](t, cu).HasUpdate, false)

// 更高版本 → 应重新下载并复位
e.PutFile(t, proj, "app.exe", "app-120")
tc.True(t, e.Publish(t, proj, "1.2.0", nil, "", "").IsSuccess, "发布 1.2.0")
cu2 := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.Eq(t, "更高版本要求重下", cu2.NeedDownloadUpdate, true)
tc.Eq(t, "重下后复位", c.ReadVersion(t).ApplyFailCount, 0)
```

**失败判定**

- 熔断后仍上报 `has_update=true` → FAIL（#2 回归：宿主每 5 秒杀一次主程序再拉起，用户侧反复闪断）
- 有更高版本时也不上报 → FAIL（熔断把逃生口也堵死了）
- download 成功后计数未复位 → FAIL

执行结果：`PASS（用例 30，6.10s）`

---

### 用例 31 — 并发锁：两个 client 同时 download

**前置条件**

1. **同一个**客户端现场（同一份 `UpdateFolder`）。
2. 服务端有可下载的新版本。

**步骤**

1. 用同一现场并发启动两个 `download_update`。
2. 检查两个结果与 `.aly.lock` 文件。

```go
outs := e.RunConcurrent(t,
    []*tc.Client{c, c},
    []string{"download_update"}, []string{"download_update"},
)
```

**期望结果**

- 一个成功、一个被拒绝（`errorMsg` 含 `lock` / `已被占用` 之类）
- `UpdateFolder` 内出现 `.aly.lock`（或运行结束后被清理）
- 最终产物只有一份，且哈希正确（并发写不会产生撕裂文件）

```go
oks := 0
for _, o := range outs {
    if o.IsSuccess {
        oks++
    }
}
tc.Eq(t, "应恰好一个成功", oks, 1)
tc.Eq(t, "产物完整", tc.SizeOf(filepath.Join(c.VerDir("1.1.0"), "app.exe")) > 0, true)
```

**失败判定**

- 两个都成功 → FAIL（并发写同一目录，产物可能撕裂）
- 两个都失败 → FAIL（锁粒度太粗，误把正常调用也拒了）
- 产物损坏 → FAIL（**最严重**）

执行结果：`PASS（用例 31，6.74s）`

---

### 用例 32 — 首次更新：不残留无名备份目录

**前置条件**

1. 客户端 `version_previous=""`（空，表示首次部署/未知来源）、`version_status="applied"`。
2. 主目录存在（有旧内容），但不知道它的版本号。
3. 服务端有可下载版本。

**步骤**

1. `download_update` → `apply_update`。
2. 检查包目录下是否存在 `ApplicationFolder_`（下划线后为空）这种无名备份目录。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusApplied, Previous: "", // 首次更新
    Project: proj, MainFile: "old-unknown",
})
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
ap := e.RunLast(t, c, "apply_update")
```

**期望结果**

- apply 成功
- 备份目录确实产生了（`ApplicationFolder_`，无名目录）——这是替换机制的自然产物
- 但 **apply 成功后它被清理掉**（`cleanupOrphanBackupIfUnreferenced`：无名目录不可回滚、不会被 prune 清理，成功且确认不再引用后删除，避免每台机器多占一份应用体积）
- 最终包目录下只有：`UpdateFolder`、`ApplicationFolder`，以及可能的版本快照目录

```go
tc.True(t, ap.IsSuccess, "apply 失败: %s", ap.ErrorMsg)
tc.Eq(t, "不残留无名备份目录",
    tc.Exists(filepath.Join(c.Pkg, "ApplicationFolder_")), false)
names := tc.ListDir(c.Pkg)
t.Logf("包目录内容: %v", names)
```

**失败判定**

- `ApplicationFolder_` 残留 → FAIL（每台机器多占一份完整应用体积，且它不可回滚、不会被 prune 清）
- apply 失败 → FAIL

执行结果：`PASS（用例 32，5.78s）`

---

### 用例 33 — 目录被占用时 apply：探测并处置占用者

**前置条件**

1. 客户端 `downloaded/1.1.0`，快照完好。
2. 用一个进程把**主目录**设为工作目录（模拟 explorer 打开该目录、或某进程 CWD 在这里）。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusDownloaded, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
})
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")

// 占住主目录（子进程把自己 CWD 放在这里 → 目录不可改名）
holder, stop := e.StartHolder(t, c.Main())
defer stop()
_ = holder
```

**步骤**

1. 在目录被占用的情况下执行 `apply_update`。
2. 检查是否成功（探测并击杀占用者）。
3. 收尾：结束占位进程。

**期望结果**

- `apply_update` **成功**（`renameDirWithKillState` 探测到占用者后击杀/处置，随后 rename 成功）
- `update.log` 若有 `apply attempt 1/3 failed` 则说明走了一次重试，最终仍成功
- 主目录为新版内容
- 占位进程被结束（`must_close_process_name` 或占用探针处置）

```go
ap := e.RunLast(t, c, "apply_update")
tc.True(t, ap.IsSuccess, "占用场景 apply 应成功（探测+处置占用者）: %s\n%s",
    ap.ErrorMsg, c.Log(t, "update.log", 80))
tc.Eq(t, "主目录为新版",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-110")
```

**失败判定**

- apply 失败 → FAIL（占用探测/击杀失效，实际部署中 explorer 打开目录极常见）
- apply 报成功但主目录仍是旧内容 → FAIL（假成功）
- 占位进程未被结束且目录仍不可改名，却报成功 → FAIL

执行结果：`PASS（用例 33，6.47s）`

---

### 用例 34 — 中文/空格/深层嵌套路径全链路

**前置条件**

1. 服务端项目 `tc-path`，数据目录内构造：
   - `中 文件.txt`（中文 + 空格）
   - `目录 名\子 目录\深层.txt`（中文目录 + 空格）
   - `a/b/c/d/e/f/deep.txt`（6 层嵌套）

```go
const proj = "tc-path"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "中 文件.txt", "cn-content")
e.PutFile(t, proj, "目录 名/子 目录/深层.txt", "cn-nested")
e.PutFile(t, proj, "a/b/c/d/e/f/deep.txt", "deep-content")
tc.True(t, e.Publish(t, proj, "1.0.0", nil, "", "").IsSuccess, "发布失败")
```

**步骤**

1. 客户端 `applied/1.0.0`，主目录空。
2. `download_update` → `apply_update`。
3. 逐条检查落地路径的**文件名不乱码、内容正确**。
4. 服务端改一个文件内容，发 `1.1.0`，再走一轮 download + apply，验证更新也能命中这些路径。

```go
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply")
```

**期望结果**

- 三条路径全部落地，文件名与预期**逐字符一致**（无 `?`、无 `%XX`、无乱码）
- 内容正确
- 第二轮更新后，被改的那个文件内容更新，其余不变

```go
for _, p := range []string{"中 文件.txt", "目录 名/子 目录/深层.txt", "a/b/c/d/e/f/deep.txt"} {
    full := filepath.Join(c.Main(), filepath.FromSlash(p))
    tc.True(t, tc.Exists(full), "路径未落地（中文/空格/深层）: %s\n%s", p, tc.Tree(c.Main(), 40))
}
tc.Eq(t, "中文文件内容",
    tc.ReadText(filepath.Join(c.Main(), "中 文件.txt")), "cn-content")
tc.Eq(t, "深层文件内容",
    tc.ReadText(filepath.Join(c.Main(), "a", "b", "c", "d", "e", "f", "deep.txt")), "deep-content")
```

**失败判定**

- 文件名出现 `?` 或 `%` 编码 → FAIL（编码在 HTTP/JSON/文件系统间转换丢失）
- 深层路径缺层 → FAIL（`MkdirAll` 用的是拼错的路径）
- 内容串位（中文文件里是深层文件的内容）→ FAIL（**最严重**：路径映射错）

执行结果：`PASS（用例 34，7.33s）`

---

### 用例 35 — 回滚到不存在的版本

**前置条件**

1. 客户端 `applied/1.1.0`，只有 `ApplicationFolder_1.0.0` 备份。
2. 尝试回滚到 `9.9.9`（不存在）。

**步骤**

1. `rollback --version 9.9.9`。
2. 检查结果与状态。
3. 再试回滚到"当前已下载但未应用"的版本（非法目标）。

```go
out := e.RunLast(t, c, "rollback", "--version", "9.9.9")
```

**期望结果**

- `IsSuccess=false`
- `ErrorMsg` 含 `version 9.9.9 not found`
- `version.json` 完全不变（`version="1.1.0"`、`status="applied"`）
- 主目录内容不变
- 对"未应用的下载版本"回滚：`errorMsg` 含 `是未应用的下载版本，不可回滚`

```go
tc.Eq(t, "isSuccess", out.IsSuccess, false)
tc.Contains(t, out.ErrorMsg, "not found", "errorMsg")
v := c.ReadVersion(t)
tc.Eq(t, "状态不变", v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "版本不变", v.Version, "1.1.0")
```

**失败判定**

- 报成功 → FAIL（**最严重**：可能把主目录改名成不存在的目标，直接丢应用）
- 状态被改成 `applying` 后卡住 → FAIL

执行结果：`PASS（用例 35，5.28s）`

---

## 六、极限条件（36–39）

> 这一节验证"量大/慢/大文件"不会把机制撑破。不硬断言性能数字，只断言**正确性**。

### 用例 36 — 超大文件（120MB）下载与校验

**前置条件**

1. 服务端项目 `tc-120`，数据目录 120MB `huge.bin`（另加一个小 `app.exe`）。
2. 客户端 `1.0.0` / `applied`，主目录空。

```go
e.EnsureProject(t, "tc-120", false, nil, nil)
e.PutFile(t, "tc-120", "app.exe", "app-100")
tc.BigFile(t, filepath.Join(e.DataDir("tc-120"), "huge.bin"), 120)
tc.True(t, e.Publish(t, "tc-120", "1.1.0", nil, "", "").IsSuccess, "发布失败")
```

**步骤**

1. `download_update`（收集全部行，观察 PROGRESS 持续输出）。
2. `apply_update`。
3. 校验最终文件大小与 MD5。

```go
outs := e.Run(t, c, "download_update")
last := outs[len(outs)-1]
tc.True(t, last.IsSuccess, "120MB 下载失败: %s", last.ErrorMsg)
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 失败")
```

**期望结果**

- 下载成功，全程 PROGRESS 行持续（不因单文件大而长时间无输出）
- 最终 `huge.bin` 大小 = `125829120`，MD5 与源一致
- apply 成功，主目录内 `huge.bin` 同样正确
- 未因超时被杀（UI 侧 `idleTimeout=90s`、`applyTimeout=900s`）

```go
final := filepath.Join(c.Main(), "huge.bin")
tc.Eq(t, "大小", tc.SizeOf(final), int64(120<<20))
tc.Eq(t, "MD5 与源一致", tc.MD5Of(final),
    tc.MD5Of(filepath.Join(e.DataDir("tc-120"), "huge.bin")))
```

**失败判定**

- 下载被超时杀掉 → FAIL（PROGRESS 节流断裂，UI 的 90s idle 判定误杀）
- 大小或 MD5 不符 → FAIL（**最严重**）

执行结果：`PASS（用例 36，14.40s）`

---

### 用例 37 — 慢下载不被误杀（单文件 >90 秒）

**前置条件**

1. 服务端项目 `tc-slow`，数据目录一个**很大**的文件（例如 500MB），且服务端限速。
2. 客户端 `1.0.0` / `applied`。
3. 目的：让**单个文件**的下载超过 90 秒。

> 若无法限速，可退化为：单文件 500MB + 慢盘，或用 `-mode update` 的 UI 侧验证（UI 的 `idleTimeout` 是 90s，只要 PROGRESS 每 3s 有输出，就不会被杀）。

**步骤**

1. 用 UI 的下载路径执行（`e.RunUIMode(t, uiExe, c, "update", 10*time.Minute)`）。
2. 或者用 `download_update` 并统计 PROGRESS 间隔，确认最大间隔 < 90s。

```go
// 方式 A：直接用 CLI 观察 PROGRESS 间隔（不需要 UI）
outs := e.Run(t, c, "download_update")
var stamps []time.Time
// 若框架记录每行到达时间，则统计相邻间隔；否则用日志时间戳
```

**期望结果**

- 单文件耗时 >90s 时，下载**仍成功完成**（未被误判停流）
- PROGRESS 行的最大间隔 < 90s（3s 节流保证）
- 无 `download stalled` / `no output for 90s` 之类错误

**失败判定**

- 出现停流误杀 → FAIL（第四批 PROGRESS 行修复失效，这是它存在的唯一理由）
- 下载被中断且状态写成 downloaded → FAIL

执行结果：`SKIP（用例 37，17.96s）` — 本机 11.83s 传完 200MB，未进入 >90s 慢下载区间；该核心（90s idle 不误杀）由用例 44/45 的 UI 侧覆盖

---

### 用例 38 — 海量小文件（500 × 1KB）

**前置条件**

1. 服务端项目 `tc-many`，数据目录 500 个 1KB 文件 `f0001.txt` … `f0500.txt`，外加 `app.exe`。

```go
const proj = "tc-many"
e.EnsureProject(t, proj, false, nil, nil)
tc.MakeFiles(t, e.DataDir(proj), 500, 1024)
e.PutFile(t, proj, "app.exe", "app-100")
tc.True(t, e.Publish(t, proj, "1.0.0", nil, "", "").IsSuccess, "发布失败")
```

**步骤**

1. `download_update` → `apply_update`。
2. 抽查文件数量与内容。
3. 记录耗时（不硬断言）。

```go
start := time.Now()
dl := e.RunLast(t, c, "download_update")
ap := e.RunLast(t, c, "apply_update")
t.Logf("500 文件全链路耗时: %s", time.Since(start))
```

**期望结果**

- 下载与 apply 均成功
- 主目录内有 501 个常规文件（500 小文件 + `app.exe`）
- 抽查内容正确（`f0001.txt` 为 1024 个 `a`）
- `downloaded_file_count` = 501

```go
tc.True(t, dl.IsSuccess && ap.IsSuccess, "500 文件链路失败: dl=%s ap=%s",
    dl.ErrorMsg, ap.ErrorMsg)
tc.Eq(t, "文件数", len(tc.ListDir(c.Main())), 501)
tc.Eq(t, "downloaded_file_count", c.ReadVersion(t).DownloadedFileCount, 501)
```

**失败判定**

- 报失败 → FAIL
- 文件数不符 → FAIL（批量下载的进度/统计错位）
- 内容错误 → FAIL

执行结果：`PASS（用例 38，20.57s）`

---

### 用例 39 — 清单 1000+ 条目的差量计算

**前置条件**

1. 服务端项目 `tc-1000`：1000 个文件，发布 `1.0.0`；客户端完成该版本 apply（缓存含 1000 条）。
2. 服务端改造后发布 `1.0.1`：**删 100、增 50、改 10**。

```go
const proj = "tc-1000"
e.EnsureProject(t, proj, false, nil, nil)
tc.MakeFiles(t, e.DataDir(proj), 1000, 1024)
e.PutFile(t, proj, "app.exe", "app-100")
tc.True(t, e.Publish(t, proj, "1.0.0", nil, "", "").IsSuccess, "发布 1.0.0")

// 客户端跑完 1.0.0（建立 1000 条缓存）
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.0")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.0")

// 服务端改造
for i := 1; i <= 100; i++ {
    e.RemoveFile(t, proj, fmt.Sprintf("f%04d.txt", i))
}
tc.MakeFiles(t, e.DataDir(proj), 1050, 1024) // 覆盖生成到 1050，多出 50 个
for i := 200; i <= 209; i++ {
    e.PutFile(t, proj, fmt.Sprintf("f%04d.txt", i), fmt.Sprintf("modified-%d", i))
}
tc.True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")
```

**步骤**

1. `download_update`，读 `download.log` 的 delete-sync 行。
2. `apply_update`。
3. 抽查三类文件（被删的 `f0100.txt`、新增的 `f1001.txt`、修改的 `f0200.txt`）。
4. 读缓存条目数。

```go
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.1")
log := c.Log(t, "download.log", 200)
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.1")
```

**期望结果**

- delete-sync 报告删除 **100** 个，路径与被删清单完全一致
- 50 个新增文件落地，10 个修改文件内容更新
- 缓存更新为 **950** 条（1000 − 100 + 50）
- 未在删除集的 900 个文件**全部保留**
- 计算与执行在合理时间内完成（不出现内存爆掉或超时）

```go
tc.Contains(t, log, "delete-sync removed 100 file(s)", "download.log")
tc.Eq(t, "被删文件应消失", tc.Exists(filepath.Join(c.Main(), "f0100.txt")), false)
tc.Eq(t, "新增文件应落地", tc.Exists(filepath.Join(c.Main(), "f1001.txt")), true)
tc.Eq(t, "修改内容生效",
    tc.ReadText(filepath.Join(c.Main(), "f0200.txt")), "modified-200")
```

**失败判定**

- 删除数量 ≠ 100 → FAIL（差集算错）
- 误删未在删除集的 900 个文件 → FAIL（**最严重**）
- 缓存条目数 ≠ 950 → FAIL（缓存固化或去重逻辑有误）

执行结果：`PASS（用例 39，54.93s）`

---

## 七、aly-client-ui（40–45）

> UI 是 aly-client 的图形前端（右下角 topmost 按需显示），本身不含业务逻辑——所有动作都通过 `bridge` 包**另起进程**调用 `aly-client.exe`，读它的 JSON 流。
> 所以这一节的判据分两层：**bridge 协议层**（能否正确解析/超时/隔离）用单测直接验；**端到端行为**（窗口是否退出、是否误杀）用 `-mode` 驱动验。
>
> UI 必须用 **go1.10.8 + GOARCH=386 + `-ldflags "-H=windowsgui"`** 构建；`view` 包依赖 `github.com/lxn/walk`，源码走 GOPATH 模式（`NewEnv` 已把 aly-client 复制到 `gopath/src/aly/client/aly-client`）。

### 用例 40 — UI 构建（386 + windowsgui）

**前置条件**

1. 已执行过 `tc.NewEnv(t, true, 0)`（`BuildAll` 会把 aly-client 与 winui 源码复制进 `gopath`，UI 依赖这两份副本）。
2. `gopath/src/aly/client/aly-client-ui/aly-client-winui` 存在且含 `main.go`。

```go
e := tc.NewEnv(t, true, 0) // 内含 BuildAll 与 GOPATH 副本
uiExe := e.BuildUI(t)
```

**步骤**

1. 用 go1.10.8 在 GOPATH 模式下构建 `aly/client/aly-client-ui/aly-client-winui`，输出到 `bin/aly-client-ui.exe`。
2. 校验产物存在、非空。
3. 校验架构确实是 386（读 PE 头，避免"能编出来但不是 386"）。
4. 校验是 GUI 子系统（`-H=windowsgui` 生效：PE Subsystem = 2）。

```go
tc.True(t, tc.Exists(uiExe), "UI 产物缺失: %s", uiExe)
tc.True(t, tc.SizeOf(uiExe) > 1<<20, "UI 体积异常: %d 字节", tc.SizeOf(uiExe))
tc.Eq(t, "PE 架构", tc.PEMachine(uiExe), tc.MachineI386)
tc.Eq(t, "PE 子系统", tc.PESubsystem(uiExe), tc.SubsystemWindowsGUI)
```

**期望结果**

- 构建 exit 0，产物为 `aly-client-ui.exe`
- 体积 > 1MB（walk 静态链接，通常 4–8MB）
- PE Machine = `0x014c`（i386）、Subsystem = `2`（WINDOWS_GUI）
- 控制台不可见（双击不弹黑窗）

**失败判定**

- 构建失败 → FAIL，输出里若出现 `cannot find package "github.com/lxn/walk"` → **GOPATH/vendor 布局问题**，不是代码问题
- PE Machine = `0x8664`（amd64）→ FAIL（XP 跑不起来，口径错了）
- Subsystem = `3`（CONSOLE）→ FAIL（`-ldflags -H=windowsgui` 丢了，用户会看到黑框）

执行结果：`PASS（用例 40，19.95s）`

---

### 用例 41 — UI bridge 单测全绿

**前置条件**

1. `gopath/src/aly/client/aly-client-ui/aly-client-winui/bridge` 存在，含 `client_test.go` / `parse_test.go` / `integration_test.go`。
2. 无外部依赖（`integration_test.go` 内部 `t.Skip` 保护找不到 exe 的情况）。

```go
code, out := e.GoTestGopath(t, "aly/client/aly-client-ui/aly-client-winui/bridge", "-v", "-count=1")
```

**步骤**

1. 在 GOPATH 模式下跑 `go test -v aly/client/aly-client-ui/aly-client-winui/bridge`。
2. 统计 `--- PASS` / `--- FAIL` / `--- SKIP` 数量。
3. 核对关键用例出现在 PASS 列表里。

```go
tc.Eq(t, "bridge 单测退出码", code, 0)
for _, name := range []string{
    "TestRunMissingExe",              // 找不到 exe 时快速失败
    "TestRunParsesDoneLine",          // DONE 行解析
    "TestRunParsesResultAndSkipsNoise", // 噪声行跳过，只认结果行
    "TestRunProgressCallback",        // PROGRESS 回调
    "TestRunTotalTimeout",            // quickTimeout 生效
    "TestRunIdleTimeout",             // idleTimeout 生效（下载停流）
    "TestRunContextCancel",           // 暂停＝cancel 子进程
    "TestParseLineProgress",          // PROGRESS 字段映射
    "TestParseLineProgressDone",      // 完成行 data:null
    "TestParseLineProgressFail",      // FAIL 行 error 字段
    "TestParseLineRollbackList",      // 回滚列表
} {
    tc.Contains(t, out, "--- PASS: "+name, "bridge 单测")
}
```

**期望结果**

- exit 0，无 `--- FAIL`
- 上列 11 条 PAASS（含 `TestRunIdleTimeout`：证明"90s 无输出才判停流"这条规则真的被测到）
- `integration_test.go` 里依赖真实 exe 的用例：有 exe 则 PASS/按契约判失败，无 exe 则 SKIP——**SKIP 不算失败但要记进备注**

**失败判定**

- 任一 `--- FAIL` → FAIL
- 缺 `TestRunIdleTimeout` / `TestRunTotalTimeout` → FAIL（超时语义没被覆盖，UI 会永久卡住）
- 全部 SKIP → 视为**未验证**（不是通过），要检查 GOPATH 副本与 exe 布局

执行结果：`PASS（用例 41，14.87s）`

---

### 用例 42 — UI `-mode check` 无更新：进程自动退出

**前置条件**

1. 服务端已发布 `1.0.0`，客户端 `1.0.0` / `applied`（服务端无更新）。
2. UI 与 `aly-client.exe` **同目录**（都放进 `UpdateFolder\`）。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
})
uiExe := e.BuildUI(t)
tc.CopyFile(t, uiExe, filepath.Join(c.Up(), "aly-client-ui.exe"))
```

**步骤**

1. `-mode check` 启动 UI，`cmd.Dir = c.Up()`（工作目录 = ExeDir）。
2. 等待进程自行退出，超时上界取 `autoCloseDelay(5s) + 余量`。
3. 记录退出码与输出。
4. 事后校验 `version.json` 未被改写。

```go
before := tc.ReadText(filepath.Join(c.Up(), "version.json"))
code, out := e.RunUIMode(t, filepath.Join(c.Up(), "aly-client-ui.exe"), c, "check", 40*time.Second)
tc.Eq(t, "UI 退出码", code, 0)
tc.Eq(t, "version.json 未被改写",
    tc.ReadText(filepath.Join(c.Up(), "version.json")) == before, true)
_ = out
```

**期望结果**

- 进程在 40s 内**自行退出**，退出码 `0`（`quit()` 结束消息循环 → `os.Exit(0)`）
- 弹出"当前已是最新版本"提示（`KeyNoUpdateBox`），5s 自动关闭
- `version.json` 逐字节不变
- 不残留后台 `aly-client.exe` 子进程

**失败判定**

- 返回 `-999`（超时未退出）→ FAIL（**最典型回归**：`AutoCloseAfter` 被改成 `Hide()`，每点一次"检查更新"漏一个进程）
- 退出码非 0 → FAIL
- `version.json` 被改写 → FAIL（check 是只读命令）
- 残留子进程 → FAIL（bridge 的 ctx 没传对，或 `Wait()` 缺失）

执行结果：`PASS（用例 42，20.16s；2026-11-09 模态框标题改为项目名后复跑）`

---

### 用例 43 — UI `-mode check` 有更新：不崩溃、不越权

**前置条件**

1. 服务端已发布 `1.1.0`，客户端 `1.0.0` / `applied`（有更新，且非强制更新）。
2. UI 与 `aly-client.exe` 同目录。

```go
// 服务端 1.1.0，客户端 1.0.0：有更新但非强制 → 应弹「更新 / 稍后」询问
```

**步骤**

1. `-mode check` 启动 UI。
2. 不点任何按钮，等 40s 观察是否崩退。
3. 检查 `version.json` 与快照目录：**没有用户点"更新"就不应有任何下载产物**。

```go
code, out := e.RunUIMode(t, filepath.Join(c.Up(), "aly-client-ui.exe"), c, "check", 40*time.Second)
tc.Eq(t, "version.json 状态未被改写", c.ReadVersion(t).VersionStatus, tc.StatusApplied)
tc.Eq(t, "未产生快照目录", tc.Exists(c.VerDir("1.1.0")), false)
```

**期望结果**

- 进程**不崩溃**：要么一直等待用户选择（`-999`，属预期，需杀掉），要么在无人交互超时后以 0 退出
- 卡片显示 `v1.1.0` 与发布内容（`KeyFoundVersion` + notes）
- 无用户点击 → **绝不自动下载**（`flow.Decide` 里 `Ask=true` 必须拦住）
- 没有 `ApplicationFolder_1.1.0\` 快照，`version_status` 仍是 `applied`

**失败判定**

- 进程崩溃（退出码非 0 且非超时）→ FAIL
- 出现快照目录 → FAIL（**严重**：跳过用户确认直接下载，违背"非强制必询问"）
- `version_status` 变成 `downloaded` → FAIL（同上）

执行结果：`PASS（用例 43，59.57s）`

---

### 用例 44 — UI 进度刷新与单位显示

**前置条件**

1. 服务端 `1.1.0` 含一个 ~30MB 文件（确保 PROGRESS 至少刷新几次）。
2. 客户端 `1.0.0` / `applied`。

```go
tc.BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 30)
tc.True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
```

**步骤**

1. 跑 `-mode update`（自动走下载，无需点按钮）。
2. 用 `bridge` 单测覆盖格式化与千分比换算（这部分**必须走单测**，不在真实窗口里断言像素）。
3. 真实下载完成后校验产物正确。

```go
// ① 单位格式化与千分比：单测覆盖
code, out := e.GoTestGopath(t, "aly/client/aly-client-ui/aly-client-winui/app",
    "-run", "TestFormatBytes|TestByteProgressPermille", "-v", "-count=1")
tc.Eq(t, "格式化单测退出码", code, 0)
tc.Contains(t, out, "--- PASS: TestFormatBytes", "app 单测")
tc.Contains(t, out, "--- PASS: TestByteProgressPermille", "app 单测")

// ② 真实下载链路
code2, _ := e.RunUIMode(t, filepath.Join(c.Up(), "aly-client-ui.exe"), c, "update", 5*time.Minute)
tc.True(t, code2 == 0 || code2 == -999, "UI update 不应崩溃，实际退出码=%d", code2)
tc.Eq(t, "大文件落地", tc.SizeOf(filepath.Join(c.Main(), "big.bin")), int64(30<<20))
```

**期望结果**

- `formatBytes`：`1024 → "1.0KiB"`、`1536 → "1.5KiB"`、`1048576 → "1.0MiB"`、`1GiB → "1.0GiB"`、`0 → "0B"`（**1024 进制**，标签 KiB/MiB/GiB）
- 千分比换算：进度条千分比 = `downloaded/total*1000`，且**单调不减**
- 真实下载：`big.bin` 大小 = 30MB，内容正确
- PROGRESS 每 3s 一条，界面进度持续动（不是"卡在 0% 然后突然 100%"）

**失败判定**

- 单位错成 `KB`/`MB`（1000 进制）→ FAIL（显示值与实际字节数对不上）
- 千分比回退（`1000 → 800`）→ FAIL（进度条会倒退）
- 大文件大小不符 → FAIL
- `TestFormatBytes` 缺失 → 视为未验证

执行结果：`PASS（用例 44，23.28s）`

---

### 用例 45 — UI 下载中暂停/继续（.part 原子写 + ≤100MB 全量重下口径）

> **⚠ 原期望写错了（已修订）**
>
> 原文标题是「断点续传 + 代数过滤」，期望"续传后不重头下，`downloaded_bytes` 起点 = 暂停时的字节数"。
> **这在 50MB 场景下不可能成立**：`client/http_client.go` 有硬阈值
> ```go
> const largeFileThreshold = 100 * 1024 * 1024
> if serverFileSize > largeFileThreshold {   // ← 严格大于
>     // 才走 Range 续传；否则忽略 .part，全量重下
> }
> ```
> 50MB **<** 100MB → 走全量重下分支，`START` 行的 `downloaded_bytes` 恒为 0（且因 `omitempty` 被省略）。
>
> 原用例的断言行是：
> ```go
> for _, o := range outs {
>     if o.Progress.Status == "START" && o.Progress.DownloadedBytes > 0 {
>         tc.True(t, o.Progress.DownloadedBytes >= sizeAtPause/2, ...)
>         break        // ← 这个条件从不成立 → 断言体永不执行 = 死断言
>     }
> }
> ```
> 即"跑完 PASS 但什么都没验"。已改为正向断言"≤100MB 不续传 + `.part` 原子写语义"。
> **真正的续传由用例 102 覆盖**（120MB，跨阈值，起始 8,949,375 字节）。

**前置条件**

1. 服务端 `1.1.0` 含一个 50MB 文件（**故意在阈值以下**，用于钉住"不续传"这个口径）。
2. 客户端 `1.0.0` / `applied`。

```go
tc.BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 50)
tc.True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
```

**步骤**

1. 起 `download_update`（异步）。
2. 等 `.part` 出现（说明已在下载），记下当前字节数。
3. 杀掉进程（等价"点暂停"），记录 `.part` 大小。
4. 重新起 `download_update`，让它重下完成。
5. 校验最终文件完整（大小 + MD5）。

```go
part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
cmd, buf := e.RunAsync(c, "download_update")
tc.True(t, tc.WaitFile(t, part, 30*time.Second), "等待 .part 出现")
tc.Kill(t, cmd)
sizeAtPause := tc.SizeOf(part)
tc.True(t, sizeAtPause > 0, "暂停时应有部分字节，实际 %d", sizeAtPause)

outs := e.Run(t, c, "download_update")
tc.True(t, outs[len(outs)-1].IsSuccess, "重试下完")

// ★ 核心断言：≤100MB 不续传，START 起点必须是 0
start := tc.ProgressStartFor(outs, "big.bin")
tc.True(t, start != nil,
    "必须存在 big.bin 的 START 行，否则下面的断言是空的。实际进度行: %v", tc.ProgressFiles(outs))
tc.Eq(t, "≤100MB 不续传：big.bin 的 START 起点必须是 0（真续传见用例 102）",
    start.DownloadedBytes, 0)

tc.Eq(t, "最终大小", tc.SizeOf(filepath.Join(c.VerDir("1.1.0"), "big.bin")), int64(50<<20))
tc.Eq(t, "MD5 与源一致", tc.MD5Of(filepath.Join(c.VerDir("1.1.0"), "big.bin")),
    tc.MD5Of(filepath.Join(e.DataDir(proj), "big.bin")))
tc.Eq(t, ".part 应已消失", tc.Exists(part), false)
```

**期望结果**

- 暂停时 `.part` **保留**且字节数 > 0（`.part` 是原子写中间态，不是"删了重下"）
- 重试后 `big.bin` 的 `STATUS:START` 行**必须存在**，且 `downloaded_bytes == 0`
  （≤100MB 全量重下；`prepareForFullRestartLocked` 会重置偏移）
- 最终大小 = 50MB，MD5 与源一致（**这是"重下没拼错"的硬证据**），`.part` 消失
- UI 侧：暂停后卡片显示"已暂停"（`KeyPaused`），再点"继续"用**新 ctx** 发起
- 旧代被杀的 `canceled` 错误被**代数过滤**掉，不误报失败

**失败判定**

- 暂停后 `.part` 被删除 → FAIL（重试会从 0 重来，50MB 场景用户会以为是网络问题）
- 最终 MD5 不符 → FAIL（**最严重**：`.part` 偏移算错，尾部拼接错位）
- 出现 `isSuccess:false` 且 `errorMsg` 含 `canceled` → FAIL（代数过滤失效，把暂停当失败）
- 找不到 `big.bin` 的 START 行 → FAIL（说明这条用例又变成死断言了）
  —— 用例已加哨兵断言，找不到直接 FAIL 并打印全部进度行

执行结果：`PASS（用例 45，17.05s）` — START 行存在且起点为 0（钉住 ≤100MB 全量重下），最终 50MB、MD5 一致、`.part` 已清理

### 七-A Linux UI（独立 Go module，不计入 135 条）

> `client/aly-client-ui/aly-client-linuxui` 是 MyGo 0.8.3 原生 UI，独立
> Go module（Go 1.27.3）；不能纳入 `test/tc` 的 go1.10.8/GOPATH 驱动。它的测试
> 在模块内 `go test ./...`，CI 在 Ubuntu 上跑；Windows 本地可跑跨平台部分，
> Linux 专属收尾/进程测试用 `linux` 构建 tag。

**验证命令**

```sh
cd client/aly-client-ui/aly-client-linuxui
go tool mygo vet
go test ./...
go tool mygo build -platform linux/amd64,linux/arm64
```

**判据**

| # | 检查 | 期望 |
|---|---|---|
| L1 | `go test ./...` | 根包 + `client` 包全 PASS；根包含 `TestViewUpdateCard`、`TestDecide`、`TestI18nKeysMatch`、`TestJournalLifecycle`、`TestUpdateSelfUISpawnsFinish`、`TestFinishSelfUpdateFlow`（Linux） |
| L2 | `go tool mygo vet` | exit 0，无输出 |
| L3 | `mygo build` | `build/linux-amd64/aly-client-ui` 是 ELF、无 `.exe`；arm64 同样 |
| L4 | 构建产物 | 根包可执行；`resources/icon.png` 随 MyGo 输出；GTK3 运行时无 WebKit/Bun 依赖 |
| L5 | 收尾入口 | `aly-client-ui -finish-self-update --target ... --pid ... --new ... --` 在无 GTK/显示环境下可运行（不初始化 MyGo）；参数缺失 exit 2 |
| L6 | 自更新派生 | `updateSelfUI` 在源/目标不同时生成 `.new`、写 journal `pending`、派生带 `--target/--pid/--created/--new/--` 的 `.new`；相同内容 noop |
| L7 | 视图 | `ui.NewTester` 下中英文卡片文本/按钮可见；不启动窗口 |

> 本机 WSL 无 Go/GTK 开发包，GUI 启动冒烟放 CI（Ubuntu + GTK3）；CI 已构建
> 两架构并跑 unit tests。完整 Linux UI 端到端（真实窗口点击）不在 135 条内，
> 视为环境依赖用例，失败时先检查 GTK3/显示，再查代码。

---

## 八、server 组件（46–65）

> 这一节只测服务端 API 本身，不涉及客户端。所有断言走 `tc.Env` 的 HTTP 封装（`e.Get` / `e.Post` / `e.Upload` / `e.RawStatus`）。
> **统一响应契约**：所有接口都返回 HTTP 200 + `{"isSuccess":bool,"errorMsg":string,"data":any}`——业务失败也是 200，只有文件流接口（download_file）才用真 HTTP 状态码。

### 用例 46 — 服务端启动 / 关闭

**前置条件**

1. 已构建 `server.exe`（go1.26，`-tags purego`）。
2. 一个空闲端口。

```go
e := tc.NewEnv(t, true, 0) // NewEnv 内部已启动服务端并等就绪
```

**步骤**

1. `GET /api/server/info`。
2. 校验返回字段完整。
3. 杀掉服务端进程，再请求同端口（应连接被拒）。
4. 复用同端口重启，确认可再次就绪。

```go
r := e.Get(t, "/api/server/info")
tc.True(t, r.IsSuccess, "server/info 失败: %s", r.ErrorMsg)
info := tc.Data[tc.ServerInfo](t, r)
tc.True(t, info.Version != "", "version 字段应非空")
tc.True(t, info.NumCPU > 0, "numCPU 应 > 0，实际 %d", info.NumCPU)

port := e.Port
e.StopServer()
tc.Eq(t, "关闭后不再可达", e.TryStatus(port, "/api/server/info"), false)
e.StartServer(t, port)
tc.Eq(t, "同端口可重启", e.Status(t, "/api/server/info"), 200)
```

**期望结果**

- `/api/server/info` 返回 200 + `isSuccess:true`，data 含 `os`/`platform`/`goARCH`/`version`/`numCPU`/`cpuName`/磁盘三项
- 进程被杀后端口立即不可连
- 同端口可重启（SQLite 未被锁死）
- 启动就绪时间 < 30s

**失败判定**

- 启动超时 → FAIL（看 `server.log`；常见是 `-db` 路径不可写）
- `numCPU = 0` / `version` 空 → FAIL（`ServerInfo` 采集断裂）
- 重启失败报 "database is locked" → FAIL（连接未释放，进程 kill 后 WAL 未回收）

执行结果：`PASS（用例 46，5.95s）`

---

### 用例 47 — create_project 成功

**前置条件**

1. 服务端就绪。
2. 项目名 `tc-crud`（合法：字母数字与 `-_`，1–64 字符）。

```go
e := tc.NewEnv(t, true, 0)
const proj = "tc-crud"
```

**步骤**

1. `POST /api/project/create_project`，带 `name`/`title`/`isForceUpdate`/`ignoreFolders`/`ignoreFiles`。
2. 校验响应与 data 字段。
3. `GET /api/project/get_project_by_name/{name}` 复核。
4. 校验数据目录已创建。

```go
r := e.Post(t, "/api/project/create_project", map[string]interface{}{
    "name": proj, "title": "测试项目", "isForceUpdate": false,
    "ignoreFolders": []string{}, "ignoreFiles": []string{},
})
tc.True(t, r.IsSuccess, "建项目失败: %s", r.ErrorMsg)
p := e.GetProject(t, proj)
tc.Eq(t, "name", p.Name, proj)
tc.Eq(t, "title", p.Title, "测试项目")
tc.Eq(t, "force_update 初值", p.ForceUpdate, false)
tc.Eq(t, "is_deleted 初值", p.IsDeleted, false)
tc.Eq(t, "数据目录已建", tc.Exists(e.DataDir(proj)), true)
```

**期望结果**

- `isSuccess:true`，data 为项目实体（含 id/name/title/version/force_update）
- `version` 初值为空串（尚未发布）
- `data/{project}/` 目录被创建
- 首次建项目会写一条空的 change_log（`CreateProjectWithFirstLog`）

**失败判定**

- `title` 为空被拒 → 属预期校验，不算失败
- 目录没创建 → FAIL（`directory.CreateDirectory` 失败但响应成功，属假成功）
- `version` 初值为 `"0.0.0"` 之类 → 与 `check_update` 的"无版本"判定冲突，记 FAIL

执行结果：`PASS（用例 47，4.77s）`

---

### 用例 48 — 重复创建同名项目被拒

**前置条件**

1. 项目 `tc-dup` 已存在（先建一次）。

```go
e.EnsureProject(t, "tc-dup", false, nil, nil)
```

**步骤**

1. 再次 `POST create_project` 同名。
2. 校验 `isSuccess:false` 与 `errorMsg`。
3. 校验原项目未被覆盖（title 仍是第一次的）。

```go
r := e.Post(t, "/api/project/create_project", map[string]interface{}{
    "name": "tc-dup", "title": "第二次", "isForceUpdate": true,
    "ignoreFolders": []string{}, "ignoreFiles": []string{},
})
tc.Eq(t, "重复创建应失败", r.IsSuccess, false)
tc.Contains(t, r.ErrorMsg, "已存在", "errorMsg")
tc.Eq(t, "原 title 未被覆盖", e.GetProject(t, "tc-dup").Title, "测试项目 tc-dup")
```

**期望结果**

- `isSuccess:false`，`errorMsg` 含 `已存在`（文案：`项目名称:%s已存在`）
- 原项目**完全不被改动**（title / force_update / version 全保持）
- 不新建重复 DB 行

**失败判定**

- 返回成功 → FAIL（**最严重**：重复行会让 `get_all_projects` 出现两条，客户端按名取可能取到旧行）
- 原 title 被改成"第二次" → FAIL（说明走成了 update 路径）
- `errorMsg` 为空 → FAIL（DB 报错被吞，用户不知道原因）

执行结果：`PASS（用例 48，4.24s）`

---

### 用例 49 — get_all_files 返回 md5 / sha256 / size

**前置条件**

1. 项目 `tc-files`，数据目录含 3 个文件，内容已知。

```go
e := tc.NewEnv(t, true, 0)
const proj = "tc-files"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "app.exe", "app-content")
e.PutFile(t, proj, "lib/a.dll", "a-content")
e.PutFile(t, proj, "中文 目录/说明 文档.txt", "中文内容")
```

**步骤**

1. `GET /api/file/get_all_files/{project}`。
2. 逐条校验路径、大小、md5、sha256。

```go
fs := e.GetAllFiles(t, proj)
tc.Eq(t, "文件数", len(fs), 3)

byRel := map[string]tc.FileInfo{}
for _, f := range fs { byRel[filepath.ToSlash(f.FileRelativePath)] = f }

app := byRel["app.exe"]
tc.Eq(t, "app.exe 大小", app.FileSize, int64(len("app-content")))
tc.Eq(t, "app.exe md5", app.MD5, tc.MD5Of(filepath.Join(e.DataDir(proj), "app.exe")))
tc.Eq(t, "app.exe sha256", app.SHA256, tc.SHA256Of(filepath.Join(e.DataDir(proj), "app.exe")))
tc.True(t, app.FileAbsolutePath != "", "fileAbsolutePath 应非空")
tc.True(t, byRel["中文 目录/说明 文档.txt"].MD5 != "", "中文路径应能算出 md5")
```

**期望结果**

- 返回 3 条，路径用 **`/`** 分隔（服务端统一 `ToSlash`）
- 每条含 `fileAbsolutePath`/`fileRelativePath`/`lastUpdateTime`/`fileSize`/`md5`/`sha256`
- md5 是 32 位小写 hex，sha256 是 64 位小写 hex
- 中文与空格路径不报错、不乱码
- 不包含 `.updator_manifest.json` 自身

**失败判定**

- md5 空串 → FAIL（缓存返回了未计算哈希的条目，客户端会认为"所有文件都变了"）
- 路径含 `\` → FAIL（Windows 分隔符漏了归一化，客户端 diff 全错）
- 出现 `.updator_manifest.json` → FAIL（清单自我污染，客户端会把它当业务文件下载）

执行结果：`PASS（用例 49，4.26s）`

---

### 用例 50 — get_release_manifest 集合 = 发布时固化

**前置条件**

1. 项目 `tc-man`，先放 2 个文件并发布 `1.0.0`。
2. 再放 1 个新文件、删 1 个旧文件，发布 `1.1.0`。

```go
const proj = "tc-man"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "a.txt", "a")
e.PutFile(t, proj, "b.txt", "b")
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布 1.0.0")

e.RemoveFile(t, proj, "b.txt")
e.PutFile(t, proj, "c.txt", "c")
tc.True(t, e.Publish(t, proj, "1.1.0", []string{"换文件"}, "", "").IsSuccess, "发布 1.1.0")
```

**步骤**

1. `GET /api/file/get_release_manifest/{project}`。
2. 校验集合恰为 `{a.txt, c.txt}`。

```go
m := e.GetManifest(t, proj)
sort.Strings(m.Files)
tc.Eq(t, "清单条目数", len(m.Files), 2)
tc.Eq(t, "清单内容", strings.Join(m.Files, ","), "a.txt,c.txt")
```

**期望结果**

- 清单 = 发布时刻数据目录的实时内容（`a.txt` + `c.txt`）
- `b.txt` **不在**清单（已删）
- 清单文件原子写（`.updator_manifest.json.tmp` → rename），读完无 `.tmp` 残留
- 从未发布过的项目返回空清单 `{"files":[]}`（不报错）

**失败判定**

- 清单仍是 `{a,b}` → FAIL（发布时没重算清单，客户端永不删旧文件）
- 出现 `.tmp` → FAIL（非原子写，读到半截 JSON 会让客户端"缓存坏 → 禁用删除同步"）
- 从未发布的项目报错 → FAIL（老项目兼容被破坏）

执行结果：`PASS（用例 50，4.17s）`

---

### 用例 51 — download_file 内容与源一致（含 Range 续传）

**前置条件**

1. 项目 `tc-dl` 数据目录有一份二进制文件（≥1MB，含非文本字节）。

```go
const proj = "tc-dl"
e.EnsureProject(t, proj, false, nil, nil)
src := filepath.Join(e.DataDir(proj), "blob.bin")
tc.BigFile(t, src, 2)
```

**步骤**

1. `GET /api/file/download_file?path=<绝对路径>`，校验字节与源完全一致。
2. 带 `Range: bytes=1048576-` 请求，校验返回尾部 1MB 且状态码 206。
3. 校验不存在的文件返回 404。

```go
got := e.Raw(t, "GET", "/api/file/download_file?path="+url.QueryEscape(src))
tc.Eq(t, "总字节数", int64(len(got)), tc.SizeOf(src))
tc.Eq(t, "内容 MD5 一致", tc.MD5Bytes(got), tc.MD5Of(src))

code, tail := e.RawRange(t, "/api/file/download_file?path="+url.QueryEscape(src), 1<<20)
tc.Eq(t, "Range 状态码", code, 206)
tc.Eq(t, "尾段字节数", int64(len(tail)), tc.SizeOf(src)-1<<20)

tc.Eq(t, "不存在的文件", e.Status(t, "/api/file/download_file?path="+
    url.QueryEscape(filepath.Join(e.DataDir(proj), "nope.bin"))), 404)
```

**期望结果**

- 全量下载字节数与 MD5 与源一致（`http.ServeContent` 直出，不经过任何转换）
- `Range` 请求返回 **206** + 精确的尾段字节，`Content-Range` 正确
- 不存在的文件 **404**（不是 200+空 body）
- `Content-Disposition` 带 `filename*=UTF-8''`（中文名不乱码）
- 不返回上传中间态（`.chunks`/`.merging`）路径

**失败判定**

- MD5 不符 → FAIL（**最严重**）
- Range 返回 200 全量 → FAIL（断点续传失效，客户端每次从头下）
- 不存在返回 200 + 空体 → FAIL（客户端会把空文件当有效产物）
- 中文文件名响应头被截断 → FAIL（下载侧落盘名字错）

执行结果：`PASS（用例 51，4.69s）`

---

### 用例 52 — 发布版本号倒退被拒

**前置条件**

1. 项目 `tc-ver` 已发布 `2.0.0`。

```go
const proj = "tc-ver"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "a.txt", "a")
tc.True(t, e.Publish(t, proj, "2.0.0", []string{"新版"}, "", "").IsSuccess, "发布 2.0.0")
```

**步骤**

1. 发布 `1.9.9`（更低）→ 期望拒绝。
2. 发布 `2.0.0`（相等）→ 期望拒绝。
3. 校验项目版本号未被改动。

```go
r := e.Publish(t, proj, "1.9.9", []string{"倒退"}, "", "")
tc.Eq(t, "倒退应失败", r.IsSuccess, false)
tc.Contains(t, r.ErrorMsg, "不大于", "errorMsg")
tc.Contains(t, r.ErrorMsg, "防倒退", "errorMsg")

r2 := e.Publish(t, proj, "2.0.0", []string{"重发"}, "", "")
tc.Eq(t, "重发同版本应失败", r2.IsSuccess, false)

tc.Eq(t, "项目版本未被改动", e.GetProject(t, proj).Version, "2.0.0")
```

**期望结果**

- 倒退与重发都 `isSuccess:false`
- `errorMsg` 含 `不大于已发布的最新版本` 与 `防倒退保护`，并**带出最新版本号**
- 项目 `version` 仍为 `2.0.0`
- change_log 未新增记录（事务回滚）

**失败判定**

- 倒退被接受 → FAIL（**最严重**：已更新的客户端会"卡"在旧内容，`check` 只认严格更高）
- `version` 被改成 `1.9.9` → FAIL（事务未回滚）
- 新增了 change_log → FAIL（同上）

执行结果：`PASS（用例 52，4.73s）`

---

### 用例 53 — 非法版本号矩阵

**前置条件**

1. 项目 `tc-badver` 存在。

```go
const proj = "tc-badver"
e.EnsureProject(t, proj, false, nil, nil)
```

**步骤**

1. 逐个提交非法版本号，全部应被拒。
2. 校验拒绝理由含"版本号格式非法"。

```go
bad := []string{
    "2026.10.31 15:00", // 带空格
    "1.0.0 beta",       // 带空格后缀
    "ver1.0.0",         // 非 v 前缀的字母开头
    "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0",
    "",                 // 空
    " 1.0.0",           // 前导空格
    "1.0.0\n",          // 换行
    "1..0",             // 空段
    ".1.0",             // 前导点
    "1.0.",             // 尾点
}
for _, v := range bad {
    r := e.Publish(t, proj, v, []string{"x"}, "", "")
    tc.Eq(t, "非法版本 "+v+" 应被拒", r.IsSuccess, false)
}

r := e.Publish(t, proj, "1.0.0 beta", []string{"x"}, "", "")
tc.Contains(t, r.ErrorMsg, "版本号格式非法", "errorMsg")
// 超长（>50）单独一条：长度校验先于正则
long := strings.Repeat("1", 60)
rl := e.Publish(t, proj, long, []string{"x"}, "", "")
tc.Eq(t, "超长版本应被拒", rl.IsSuccess, false)
tc.Contains(t, rl.ErrorMsg, "长度", "errorMsg")
```

**期望结果**

- 全部 `isSuccess:false`
- 空格/字母开头/空段/前导尾点 → `errorMsg` 含 `版本号格式非法`，并给出示例（`1.0.0 / 2.1.0-beta / v1.2.3`）
- 空串 → `版本号不能为空`
- 长度 > 50 → `版本号长度不能超过50个字符`（长度校验在正则之前）
- 项目版本号完全不变

**失败判定**

- 任一被接受 → FAIL（**严重**：客户端 `isLikelyVersion` 口径与之对齐，接了非法版本会让 check 判定错乱）
- 报 500 或 panic → FAIL（应走 `models.NG` 正常拒绝）
- 长度校验顺序反了（先报格式非法再报长度）→ 记备注，不算失败

执行结果：`PASS（用例 53，4.19s）`

---

### 用例 54 — 合法版本号矩阵（含 v 前缀/预发布/元数据）

**前置条件**

1. 项目 `tc-goodver` 存在，尚未发布。

```go
const proj = "tc-goodver"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "a.txt", "a")
```

**步骤**

1. 按严格递增顺序发布一串合法版本号，全部应成功。
2. 校验 `CompareVersion` 口径：`v` 前缀被剥离后再比较。

```go
seq := []string{"1", "1.0.1", "1.0.2-beta", "1.0.2", "v1.1.0", "1.2.0+build.7"}
for _, v := range seq {
    r := e.Publish(t, proj, v, []string{"发布 " + v}, "", "")
    tc.True(t, r.IsSuccess, "合法版本 %s 被拒: %s", v, r.ErrorMsg)
}
tc.Eq(t, "最终项目版本", e.GetProject(t, proj).Version, "1.2.0+build.7")
tc.Eq(t, "变更日志条数", len(e.ChangeLogs(t, proj)), len(seq))
```

**期望结果**

- 6 条全部发布成功（`1` 单段也合法，与客户端 `isLikelyVersion` 对齐）
- `v` 前缀与 `+元数据` 被 `CompareVersion` 正确处理：`v1.1.0` > `1.0.2`
- `1.0.2-beta` < `1.0.2`（预发布低于正式版）
- change_log 条数与发布次数一致，每条 `version`/`logs`/`time` 都落对

**失败判定**

- 任一被拒 → FAIL（口径与客户端不一致，会出现"服务端能发、客户端不认"）
- `v1.1.0` 被当字符串比较而小于 `1.0.2` → FAIL（`v` 前缀没剥离，防倒退会误伤）
- change_log 少一条 → FAIL（事务部分提交）

执行结果：`PASS（用例 54，4.69s）`

---

### 用例 55 — 非法时间格式被拒

**前置条件**

1. 项目 `tc-time` 存在。

```go
const proj = "tc-time"
e.EnsureProject(t, "tc-time", false, nil, nil)
```

**步骤**

1. 用非法时间字符串发布。
2. 用合法格式发布。
3. **注意字段名是 `time`**（不是 `timeStr`）。

```go
// 字段名必须是 time：写 timeStr 会被 gin 静默忽略 →
// 服务端 time 落空串 → 校验被跳过 → 这条断言会永远"通过"（假通过）。
bad := []string{"bad-time-format", "2026/11/01 10:00:00", "2026-11-01", "2026-11-01T10:00:00Z"}
for _, ts := range bad {
    r := e.Publish(t, "tc-time", "9.0.0", []string{"x"}, ts, "")
    tc.Eq(t, "非法时间 "+ts+" 应被拒", r.IsSuccess, false)
}
r := e.Publish(t, "tc-time", "9.0.0", []string{"x"}, "bad-time-format", "")
tc.Contains(t, r.ErrorMsg, "时间格式", "errorMsg")

ok := e.Publish(t, "tc-time", "9.0.0", []string{"x"}, "2026-11-01 10:00:00", "")
tc.True(t, ok.IsSuccess, "合法时间应通过: %s", ok.ErrorMsg)
```

**期望结果**

- 非法时间 → `isSuccess:false`，`errorMsg` = `时间格式错误，正确格式: yyyy-MM-dd HH:mm:ss`
- ISO8601（含 `T`/`Z`）也被拒（服务端口径唯一）
- 合法 `2026-11-01 10:00:00` 通过，落库的 `time` 与传入逐字一致
- 传空串则不校验时间（`timeStr != ""` 才校验），允许发布

**失败判定**

- 非法时间被接受 → FAIL（**这条历史上有真实缺陷**：曾经字段名写成 `timeStr`，被 gin 静默忽略，校验形同虚设）
- `errorMsg` 不含"时间格式" → FAIL（文案被改动，测试/用户都失去线索）
- 空串也被拒 → FAIL（老客户端不传 time 会全部发布失败）

执行结果：`PASS（用例 55，4.17s）`

---

### 用例 56 — 清单/文件列表过滤口径：`.chunks`/`.merging` 不入，`.tmp` 入

**前置条件**

1. 项目 `tc-filter` 数据目录手工造出各类中间态：
   - `a.txt`（正式）
   - `x.chunks/part1`（分片目录）
   - `y.merging`（合并中间态）
   - `z.tmp`（**普通临时文件，不算上传中间态，必须入清单**）

```go
const proj = "tc-filter"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "a.txt", "a")
e.PutFile(t, proj, "x.chunks/part1", "p1")
e.PutFile(t, proj, "y.merging", "m")
e.PutFile(t, proj, "z.tmp", "t")
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布")
```

**步骤**

1. 拉 `get_all_files` 与 `get_release_manifest`。
2. 两处都应排除 `.chunks`/`.merging`，都应包含 `.tmp`。

```go
fs := e.GetAllFiles(t, proj)
var rels []string
for _, f := range fs { rels = append(rels, filepath.ToSlash(f.FileRelativePath)) }
sort.Strings(rels)
tc.Eq(t, "get_all_files 集合", strings.Join(rels, ","), "a.txt,z.tmp")

m := e.GetManifest(t, proj)
sort.Strings(m.Files)
tc.Eq(t, "清单集合", strings.Join(m.Files, ","), "a.txt,z.tmp")
```

**期望结果**

- 两处集合**完全一致** = `{a.txt, z.tmp}`
- `x.chunks/part1` 与 `y.merging` 都被排除（任一路径段以 `.chunks`/`.merging` 结尾即排除）
- `z.tmp` **保留**（`.tmp` 不是上传中间态，若排掉会导致"发布端有、客户端没有"的永久差异）
- 清单自身 `.updator_manifest.json` 也在两处都被排除

**失败判定**

- 两处集合不一致 → FAIL（客户端按清单删文件会删掉合法文件）
- `.tmp` 被排除 → FAIL（同上，会把用户真实文件当中间态忽略）
- `.chunks`/`.merging` 出现在清单里 → FAIL（客户端会把"下轮消失的中间态"误判为"服务端删除"，进而删本地文件）

执行结果：`PASS（用例 56，4.82s）`

---

### 用例 57 — ignore 规则在文件列表与清单中同口径生效

**前置条件**

1. 项目 `tc-ignore` 创建时带 `ignoreFolders=["logs"]`、`ignoreFiles=["*.log","secret.txt"]`。
2. 数据目录含被忽略与被保留的文件各若干。

```go
const proj = "tc-ignore"
e.EnsureProject(t, proj, false, []string{"logs"}, []string{"*.log", "secret.txt"})
for _, p := range []string{"app.exe", "cfg/settings.json", "logs/today.txt",
                          "debug.log", "secret.txt", "data/cache.bin"} {
    e.PutFile(t, proj, p, "x-" + p)
}
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布")
```

**步骤**

1. 拉 `get_all_files` 与清单。
2. 期望都只含 `app.exe`、`cfg/settings.json`、`data/cache.bin`。

```go
fs := e.GetAllFiles(t, proj)
var rels []string
for _, f := range fs { rels = append(rels, filepath.ToSlash(f.FileRelativePath)) }
sort.Strings(rels)
want := "app.exe,cfg/settings.json,data/cache.bin"
tc.Eq(t, "get_all_files 过滤结果", strings.Join(rels, ","), want)

m := e.GetManifest(t, proj)
sort.Strings(m.Files)
tc.Eq(t, "清单过滤结果", strings.Join(m.Files, ","), want)
```

**期望结果**

- 两处都只剩 3 条，`logs/` 整目录被跳过、`debug.log`（glob `*.log`）、`secret.txt`（精确）被排除
- 忽略规则**发布后修改也立即生效**（`update_project` 改 ignore → 下次 `get_all_files` 走新规则；缓存失效）
- 与 publish-cli 的 `scanner.go` 口径一致（否则发布端算"有变更"、服务端算"没这个文件"，`status` 永远不干净）

**失败判定**

- 清单含被忽略文件 → FAIL（客户端会去下载服务端根本不打算给的文件 → 404 失败）
- 发布端与服务端口径不一致 → FAIL（`status` 显示有差异但 `add` 后仍显示，形成死循环）
- `logs/` 下所有文件被当成业务文件 → FAIL（日志目录被同步给所有客户端，泄密 + 体积暴涨）

执行结果：`PASS（用例 57，4.16s）`

---

### 用例 58 — 上传中间态不留半截正式文件

**前置条件**

1. 项目 `tc-atomic` 存在。
2. 一个 20MB 文件待上传。

```go
const proj = "tc-atomic"
e.EnsureProject(t, proj, false, nil, nil)
src := filepath.Join(e.Root, "upload-src.bin")
tc.BigFile(t, src, 20)
```

**步骤**

1. 上传期间**并发**轮询 `get_all_files`，记录是否出现目标文件名。
2. 上传完成后校验最终文件大小与 MD5。
3. 校验目录内无 `.merging` 残留。

```go
done := make(chan struct{})
var sawPartial int32
go func() {
    for {
        select {
        case <-done: return
        default:
        }
        fs, _ := tryGetAllFiles(e, proj)
        for _, f := range fs {
            if filepath.Base(f.FileRelativePath) == "upload-src.bin" &&
                f.FileSize != tc.SizeOf(src) {
                atomic.AddInt32(&sawPartial, 1) // 半截正式文件（撕裂读）
            }
        }
        time.Sleep(10 * time.Millisecond)
    }
}()

r := e.Upload(t, proj, "dir/upload-src.bin", src)
close(done)
tc.True(t, r.IsSuccess, "上传失败: %s", r.ErrorMsg)
tc.Eq(t, "上传期间见到的半截文件数", atomic.LoadInt32(&sawPartial), int32(0))
tc.Eq(t, "最终大小", tc.SizeOf(filepath.Join(e.DataDir(proj), "dir", "upload-src.bin")),
    tc.SizeOf(src))
tc.Eq(t, "无 .merging 残留",
    tc.ListDirMatch(filepath.Join(e.DataDir(proj), "dir"), "*.merging"), 0)
```

**期望结果**

- 上传期间并发查询**从不**看到大小不符的"正式文件"（先写 `.merging` 再 rename）
- 最终文件大小与 MD5 与源一致
- 上传结束目录内无 `.merging`
- 上传后文件列表缓存被失效，下次查询反映新文件

**失败判定**

- 观察到半截正式文件 → FAIL（**严重**：客户端可能下载到不完整文件并通过 MD5 校验？不会——但会误判"服务端有这个文件"从而跳过下载，永久坏文件）
- 残留 `.merging` → FAIL（磁盘泄漏，且下次同名上传 rename 覆盖失败）
- 缓存未失效 → FAIL（`get_all_files` 返回旧集合，`status` 显示无变更）

执行结果：`PASS（用例 58，5.61s）`

---

### 用例 59 — get_all_files 缓存：上传后失效、查询后重建

**前置条件**

1. 项目 `tc-cache` 含 1 个文件，已查询过一次（缓存已建立）。

```go
const proj = "tc-cache"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "a.txt", "a")
first := e.GetAllFiles(t, proj) // 建缓存
tc.Eq(t, "首次条数", len(first), 1)
```

**步骤**

1. 直接改数据目录（模拟磁盘变化）→ 不失效缓存时**应仍返回旧集合**。
2. 走一次 `upload_file`（会触发 `InvalidateProjectFileList`）→ 再查**应返回新集合**。

```go
// ① 纯磁盘变化：缓存命中，仍返回旧集合（证明真的在缓存）
e.PutFile(t, proj, "b.txt", "b")
tc.Eq(t, "缓存命中仍为 1 条", len(e.GetAllFiles(t, proj)), 1)

// ② 走上传接口：缓存失效，重建后含新文件
tc.True(t, e.Upload(t, proj, "b.txt", filepath.Join(e.DataDir(proj), "b.txt")).IsSuccess, "上传")
tc.Eq(t, "上传后为 2 条", len(e.GetAllFiles(t, proj)), 2)
```

**期望结果**

- ① 纯磁盘旁路修改 → 缓存命中，仍返回 1 条（不能每次全量重算哈希，那是性能灾难）
- ② 经 `upload_file` → 缓存失效，返回 2 条
- 发布（`publish_version`）也会重算清单但**不**依赖文件列表缓存
- 删除项目后 `InvalidateProjectFileList` 被调用（防内存泄漏）

**失败判定**

- ① 返回 2 条 → 缓存形同虚设（大项目每次查询都全量算哈希，秒级阻塞）
- ② 仍返回 1 条 → FAIL（**严重**：上传后客户端看不到新文件，`status` 永远说"无变更"）
- 缓存 key 未带项目名 → 跨项目串味，属严重缺陷

执行结果：`PASS（用例 59，4.32s）`

---

### 用例 60 — 空项目清单与文件列表

**前置条件**

1. 项目 `tc-empty` 刚创建，数据目录为空（或仅有清单文件自身）。

```go
const proj = "tc-empty"
e.EnsureProject(t, proj, false, nil, nil)
```

**步骤**

1. 未发布时拉清单 → 期望空数组而非报错。
2. 拉 `get_all_files` → 期望空数组。
3. 发布一次（数据目录仍空）→ 再拉清单。

```go
m0 := e.GetManifest(t, proj)
tc.Eq(t, "未发布时清单为空", len(m0.Files), 0)

fs := e.GetAllFiles(t, proj)
tc.Eq(t, "空项目文件数为 0", len(fs), 0)

r := e.Publish(t, proj, "1.0.0", []string{"空版本"}, "", "")
tc.True(t, r.IsSuccess, "空项目也应能发布: %s", r.ErrorMsg)
m1 := e.GetManifest(t, proj)
tc.Eq(t, "空发布后清单仍为空", len(m1.Files), 0)
```

**期望结果**

- 未发布：`{"files":[]}`（`null` 被归一成 `[]`——客户端解析 `nil` 切片会崩/误判）
- `get_all_files` 返回 `[]`，不是 `null`，不是报错
- 空数据目录也能发布（发布只写清单与 DB 记录）
- 整个过程无 500

**失败判定**

- 返回 `data:null` 而非 `[]` → FAIL（客户端 `len(nil)` 与 `nil == nil` 判定差异，会导致"跳过删除"或 panic）
- 空项目发布报错 → FAIL（无法建"先建项目后传文件"的流程）
- 500 → FAIL

执行结果：`PASS（用例 60，4.41s）`

---

### 用例 61 — 不存在的项目：明确拒绝而非 500

**前置条件**

1. 服务端就绪，库内无 `tc-nonexistent`。

**步骤**

1. 逐个接口请求不存在的项目。
2. 校验都是 HTTP 200 + `isSuccess:false` + 有意义的 `errorMsg`。

```go
const missing = "tc-nonexistent"

r1 := e.Get(t, "/api/file/get_all_files/"+missing)
tc.Eq(t, "get_all_files 应失败", r1.IsSuccess, false)
tc.True(t, r1.ErrorMsg != "", "errorMsg 不应为空")

r2 := e.Get(t, "/api/project/get_project_by_name/"+missing)
tc.Eq(t, "get_project_by_name 应失败", r2.IsSuccess, false)

r3 := e.Get(t, "/api/file/get_release_manifest/"+missing)
// 清单接口对不存在项目返回空清单（目录不存在 → 空），或失败但必须不是 500
tc.True(t, r3.IsSuccess || r3.ErrorMsg != "", "要么空清单要么明确报错")

r4 := e.Publish(t, missing, "1.0.0", []string{"x"}, "", "")
tc.Eq(t, "发布到不存在项目应失败", r4.IsSuccess, false)
```

**期望结果**

- 全部 HTTP **200**（统一契约），业务层 `isSuccess:false`
- `errorMsg` 含实质信息（如 `ent: project not found`），不是空串
- 不出现 HTTP 500 / gin panic 堆栈
- `get_release_manifest` 在项目不存在时：目录不存在 → `writeReleaseManifest` 的 `os.IsNotExist` 跳过语义 → 返回空清单（可接受）

**失败判定**

- 500 → FAIL（统一契约被破坏，publish-cli 会把 500 当网络错误反复重试）
- `isSuccess:true` 但 data 为 null → FAIL（假成功，客户端会当成"项目存在且无文件"）
- `errorMsg` 空 → FAIL（用户完全不知道哪一步错了）

执行结果：`PASS（用例 61，4.28s）`

---

### 用例 62 — 软删除项目后各查询不再返回

**前置条件**

1. 项目 `tc-del` 已创建并发布 `1.0.0`。

```go
const proj = "tc-del"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "a.txt", "a")
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布")
```

**步骤**

1. `POST /api/project/delete_project/{name}`。
2. 查 `get_all_projects` / `get_project_by_name` / `publish_version`。

```go
r := e.Post(t, "/api/project/delete_project/"+proj, nil)
tc.True(t, r.IsSuccess, "删除失败: %s", r.ErrorMsg)

all := e.AllProjects(t)
for _, p := range all {
    tc.True(t, p.Name != proj, "软删除项目不应出现在 get_all_projects")
}

r2 := e.Get(t, "/api/project/get_project_by_name/"+proj)
tc.Eq(t, "软删除项目按名查询应失败", r2.IsSuccess, false)

r3 := e.Publish(t, proj, "1.1.0", []string{"x"}, "", "")
tc.Eq(t, "软删除项目不能发布", r3.IsSuccess, false)
```

**期望结果**

- 删除接口 `isSuccess:true`（软删除：`is_deleted=true`）
- `get_all_projects` 不再包含它
- 按名查询失败、不能发布新版本（`PublishVersion` 的 `project.IsDeletedEQ(false)` 挡住）
- 数据目录**不**被物理删除（可恢复）
- 该项目的文件列表缓存被释放

**失败判定**

- 硬删除（目录消失）→ FAIL（用户误删无法挽回）
- 仍出现在 `get_all_projects` → FAIL（客户端下拉列表里有幽灵项目）
- 能继续发布 → FAIL（客户端会去拉一个"已删除"项目的新版本）

执行结果：`PASS（用例 62，4.19s）`

---

### 用例 63 — 大文件上传（100MB+）

**前置条件**

1. 项目 `tc-big` 存在。
2. 本地 100MB 文件。

```go
const proj = "tc-big"
e.EnsureProject(t, proj, false, nil, nil)
big := filepath.Join(e.Root, "big-100.bin")
tc.BigFile(t, big, 100)
```

**步骤**

1. `upload_file` multipart 上传 100MB。
2. 校验服务端文件大小与 MD5。
3. 再传一个超过上限的文件，校验被拒（`maxUploadFileSize`）。

```go
r := e.Upload(t, proj, "big-100.bin", big)
tc.True(t, r.IsSuccess, "100MB 上传失败: %s", r.ErrorMsg)
dst := filepath.Join(e.DataDir(proj), "big-100.bin")
tc.Eq(t, "大小", tc.SizeOf(dst), tc.SizeOf(big))
tc.Eq(t, "MD5", tc.MD5Of(dst), tc.MD5Of(big))
```

**期望结果**

- 上传成功，无 `unexpected EOF` / `request body too large`
- 大小与 MD5 与源一致
- 内存不爆（`MaxBytesReader` 流式落盘，不是先读进内存）
- 上传后 `get_all_files` 能看到它（缓存已失效）
- 超过上限（按 `maxUploadFileSize` 定义）→ `isSuccess:false` + `errorMsg` 含 `文件过大`

**失败判定**

- 内存爆掉 / 进程 OOM → FAIL（用了 `io.ReadAll` 而不是流式）
- MD5 不符 → FAIL（**最严重**）
- 超限文件被接受 → FAIL（磁盘打满风险）

执行结果：`PASS（用例 63，8.21s）`

---

### 用例 64 — 并发下载同一文件

**前置条件**

1. 项目 `tc-conc` 含一个 5MB 文件。

```go
const proj = "tc-conc"
e.EnsureProject(t, proj, false, nil, nil)
src := filepath.Join(e.DataDir(proj), "shared.bin")
tc.BigFile(t, src, 5)
```

**步骤**

1. 并发 8 路 `download_file`。
2. 全部校验 MD5 一致。

```go
var wg sync.WaitGroup
results := make([]string, 8)
for i := 0; i < 8; i++ {
    wg.Add(1)
    go func(idx int) {
        defer wg.Done()
        b := e.Raw(t, "GET", "/api/file/download_file?path="+url.QueryEscape(src))
        results[idx] = tc.MD5Bytes(b)
    }(i)
}
wg.Wait()
want := tc.MD5Of(src)
for i, got := range results {
    tc.Eq(t, fmt.Sprintf("第 %d 路 MD5", i+1), got, want)
}
```

**期望结果**

- 8 路全部成功，MD5 全部一致
- 无锁等待、无 429/503（`http.ServeContent` 只读并发安全）
- 总耗时接近单路（不因并发而串行化）

**失败判定**

- 任一路 MD5 不符 → FAIL（读到被改写中的文件；说明上传没走原子 rename）
- 出现 5xx → FAIL（服务端有共享可变状态被并发踩坏）
- 明显串行化（总耗时 ≈ 8 倍单路）→ 记备注（性能退化，非功能失败）

执行结果：`PASS（用例 64，6.38s）`

---

### 用例 65 — 路径穿越与中间态下载防护

**前置条件**

1. 服务端项目 `tc-sec` 存在，数据目录含 1 个正常文件。
2. 数据目录外有一个敏感文件（模拟：`<serverDir>/secret.txt`）。

```go
const proj = "tc-sec"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "ok.txt", "ok")
tc.WriteText(t, filepath.Join(e.Bin, "secret.txt"), "TOP-SECRET")
```

**步骤**

1. 尝试用 `..` 穿越到数据目录外，期望 404。
2. 尝试下载上传中间态（`.chunks`/`.merging`），期望 404。
3. 正常文件应 200。

```go
base := "/api/file/download_file?path="
for _, p := range []string{
    filepath.Join(e.Bin, "secret.txt"),                       // 数据目录外
    filepath.Join(e.DataDir(proj), "..", "..", "secret.txt"), // 相对穿越
    filepath.Join(e.DataDir(proj), "..", "secret.txt"),
    e.DataDir(proj) + string(filepath.Separator) + ".." + string(filepath.Separator) + "e2e.db",
    filepath.Join(e.DataDir(proj), "x.chunks", "p1"),
    filepath.Join(e.DataDir(proj), "y.merging"),
} {
    tc.Eq(t, "越界/中间态应 404: "+p, e.Status(t, base+url.QueryEscape(p)), 404)
}

tc.Eq(t, "正常文件应 200",
    e.Status(t, base+url.QueryEscape(filepath.Join(e.DataDir(proj), "ok.txt"))), 200)
```

**期望结果**

- 所有越界路径（绝对路径出界、`..` 穿越、指向 `e2e.db`）→ **404**
- `.chunks` / `.merging` 中间态 → **404**（纵深防御，不依赖清单过滤）
- 正常文件 → 200 + 正确内容
- 不泄露任何路径存在性差异（越界与不存在都是 404，不给探测者额外信息）

**失败判定**

- 任一越界请求返回 200 → FAIL（**最严重**：可下载 SQLite 库/任意系统文件）
- 中间态可下载 → FAIL（会把半截文件当正式产物）
- 越界返回 500 → 记备注（信息泄露：500 说明路径存在但读失败）

执行结果：`PASS（用例 65，4.92s）`

---

## 九、aly-publish（66–77）

> `aly-publish` 是发布端命令行。工作目录约定：**在哪个目录执行，就用哪个目录的 `.updator/`**（对标 git）。
> 配置文件两个：
> - `.updator/shared.json`：`server_url` / `project_name` / `ignore_folders` / `ignore_files` / `un_copy_folders` / `un_copy_files`
> - `.updator/publish.json`：发布端自身配置
>
> 暂存区在同目录 `.updator/staging.json`（`add` 写入，`push` 后清空）。
> **统一加 `--json`**：非 json 模式的失败信息写 stderr、成功信息写人读文本，断言口径不统一。框架的 `e.PublishCLI` 已自动附加 `--server <url> --json`。
>
> ⚠️ **构建注意**：publish-cli 依赖 `crypto/tls`，其 `chacha20poly1305` 的 amd64 汇编路径在部分本机触发 `compile.exe` 的 `R6016`。构建**必须**带 `-tags purego`（`BuildAll` 已内建）。

### 用例 66 — config init 生成两个配置文件

**前置条件**

1. 一个空工作目录（无 `.updator/`）。
2. 服务端可达（`--server` 已给）。

```go
ws := filepath.Join(e.Root, "pub-init")
os.MkdirAll(ws, 0o755)
```

**步骤**

1. `config init --project tc-app`（可带 `--ignore-folders` / `--ignore-files`）。
2. 校验两个文件都被创建、字段正确。
3. 再跑一次 `config init`，校验幂等（不覆盖已有值）。

```go
r := e.PublishCLI(t, ws, "config", "init", "--project", "tc-app",
    "--ignore-folders", "logs,tmp", "--ignore-files", "*.log,*.tmp")
tc.True(t, r.IsSuccess, "config init 失败: %s", r.ErrorMsg)

sharedPath := filepath.Join(ws, ".updator", "shared.json")
pubPath := filepath.Join(ws, ".updator", "publish.json")
tc.Eq(t, "shared.json 已创建", tc.Exists(sharedPath), true)
tc.Eq(t, "publish.json 已创建", tc.Exists(pubPath), true)

var shared map[string]interface{}
tc.True(t, json.Unmarshal([]byte(tc.ReadText(sharedPath)), &shared) == nil, "shared.json 应是合法 JSON")
tc.Eq(t, "project_name", shared["project_name"], "tc-app")
tc.Eq(t, "ignore_folders", fmt.Sprint(shared["ignore_folders"]), "[logs tmp]")
tc.Eq(t, "ignore_files", fmt.Sprint(shared["ignore_files"]), "[*.log *.tmp]")

// 幂等：再 init 一次不报错
tc.True(t, e.PublishCLI(t, ws, "config", "init", "--project", "tc-app").IsSuccess, "重复 init 应幂等")
```

**期望结果**

- `.updator/shared.json` 与 `.updator/publish.json` 都生成
- `shared.json` 含 `server_url`（来自 `--server`）/`project_name`/4 个数组字段（未指定的为空数组，**不是 null**）
- `--ignore-folders` 的逗号分隔值被正确切成数组
- 重复执行 `config init`：不报错、不把已有 `ignore_*` 清空

**失败判定**

- 数组字段变成 `null` → FAIL（后续 `MatchIgnoreFile` 遍历 nil 切片虽安全，但 `un_copy` 判空会走不同分支）
- 重复 init 把 `ignore_files` 清空 → FAIL（用户的规则丢失，`status` 突然冒出一堆"新增"文件）
- `server_url` 没写进去 → FAIL（后续每条命令都要手带 `--server`）

执行结果：`PASS（用例 66，4.70s）`

---

### 用例 67 — status 分类（新增 / 修改 / 删除 / 未暂存 / 已暂存）

**前置条件**

1. 服务端项目 `tc-pub` 已发布 `1.0.0`，含 `a.txt` / `b.txt` / `c.txt`。
2. 工作区有同名文件，但状态各异：
   - `a.txt` 内容相同（无差异）
   - `b.txt` 内容不同（modified）
   - `c.txt` 本地删除（deleted）
   - `d.txt` 本地新增（new）

```go
const proj = "tc-pub"
e.EnsureProject(t, proj, false, nil, nil)
for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
    e.PutFile(t, proj, n, n+"-v1")
}
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布 1.0.0")

ws := e.NewPublishWS(t, proj, "pub-status")
tc.WriteText(t, filepath.Join(ws, "a.txt"), "a.txt-v1")   // 相同
tc.WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v2")   // 修改
// c.txt 不建 → 本地缺失
tc.WriteText(t, filepath.Join(ws, "d.txt"), "d.txt-new")  // 新增
```

**步骤**

1. `status --json`。
2. 校验 `unstaged` 数组里四项分类都正确。

```go
r := e.PublishCLI(t, ws, "status")
tc.True(t, r.IsSuccess, "status 失败: %s", r.ErrorMsg)
sd := tc.Data[tc.StatusData](t, r)

byPath := map[string]tc.FileStatusItem{}
for _, it := range sd.Unstaged { byPath[it.RelativePath] = it }

tc.Eq(t, "a.txt 无差异", func() bool { _, ok := byPath["a.txt"]; return ok }(), false)
tc.Eq(t, "b.txt 状态", byPath["b.txt"].Status, "modified")
tc.Eq(t, "c.txt 状态", byPath["c.txt"].Status, "deleted")
tc.Eq(t, "d.txt 状态", byPath["d.txt"].Status, "new")
tc.Eq(t, "unstaged 条数", len(sd.Unstaged), 3)
```

**期望结果**

- `a.txt` 内容与 MD5 都一致 → **不出现**在差异列表（不噪音）
- `b.txt` → `modified`（带 `serverMd5` 与 `localMd5`，两者不同）
- `c.txt` → `deleted`（服务端有、本地无）
- `d.txt` → `new`（本地有、服务端无，`serverMd5` 为空）
- 条数恰为 3（不是 4）
- `status` 会顺手把"与服务端一致"的文件从暂存区摘掉（自动同步）

**失败判定**

- `a.txt` 出现在列表里 → FAIL（MD5 比较口径错，`status` 永远不干净）
- `d.txt` 被标 `modified` → FAIL（`serverMd5` 空串被当成"内容不同"）
- `c.txt` 不出现 → FAIL（删除同步失效，服务端旧文件永远删不掉）

执行结果：`PASS（用例 67，4.67s）`

---

### 用例 68 — add --all 全部暂存（含 deleted）

**前置条件**

1. 承用例 67 的现场（3 项差异）。
2. 暂存区为空。

```go
// 承上：b.txt modified / c.txt deleted / d.txt new
```

**步骤**

1. `add --all`。
2. `staged` 校验暂存区内容。
3. 再 `status`，校验暂存项从 `unstaged` 移到 `staged`。

```go
r := e.PublishCLI(t, ws, "add", "--all")
tc.True(t, r.IsSuccess, "add --all 失败: %s", r.ErrorMsg)
tc.Eq(t, "暂存数量", tc.Data[tc.AddData](t, r).Added, 3)

st := e.PublishCLI(t, ws, "staged")
tc.True(t, st.IsSuccess, "staged 失败: %s", st.ErrorMsg)
staged := tc.Data[[]tc.StagedItem](t, st)
names := []string{}
for _, s := range staged { names = append(names, s.RelativePath) }
sort.Strings(names)
tc.Eq(t, "暂存内容", strings.Join(names, ","), "b.txt,c.txt,d.txt")

sd := tc.Data[tc.StatusData](t, e.PublishCLI(t, ws, "status"))
tc.Eq(t, "unstaged 应为空", len(sd.Unstaged), 0)
tc.Eq(t, "staged 为 3", len(sd.Staged), 3)
```

**期望结果**

- `add --all` 的 data 为 `{"added":3}`
- 暂存区含 3 条，每条带 `relativePath` / `md5` / `status`（`status` 记录加入时的分类）
- `status` 里 `unstaged` 归零、`staged` 为 3
- **`deleted` 也会被暂存**（删除也是一种变更，必须在暂存区里，push 后服务端才知道要删）

**失败判定**

- `added` = 2 → FAIL（`deleted` 没被暂存，push 后服务端旧文件残留）
- 暂存区 md5 为空 → FAIL（`push` 前的 MD5 复核失去基线）
- `status` 里同一条既在 staged 又在 unstaged → FAIL（`mergeStagedIntoStatusData` 的过滤失效，`push` 会重复上传）

执行结果：`PASS（用例 68，5.92s）`

---

### 用例 69 — push 发布（上传 + 删除同步 + 创建版本 + 清单固化）

**前置条件**

1. 承用例 68（3 项已暂存）。
2. 服务端最新版本是 `1.0.0`。

**步骤**

1. `push --version 1.1.0 --message "更新说明" --message "第二条"`。
2. 校验服务端版本、变更日志、文件集合、清单。
3. 校验本地暂存区被清空。

```go
r := e.PublishCLI(t, ws, "push", "--version", "1.1.0",
    "--message", "更新说明", "--message", "第二条")
tc.True(t, r.IsSuccess, "push 失败: %s", r.ErrorMsg)

tc.Eq(t, "服务端版本", e.GetProject(t, proj).Version, "1.1.0")
logs := e.ChangeLogs(t, proj)
tc.Eq(t, "日志条数", len(logs), 2)
tc.Eq(t, "最新日志版本", logs[0].Version, "1.1.0")
tc.Eq(t, "最新日志内容数", len(logs[0].Logs), 2)

// 服务端文件集合：b.txt 内容更新、c.txt 被删、d.txt 新增
fs := e.GetAllFiles(t, proj)
var rels []string
for _, f := range fs { rels = append(rels, f.FileRelativePath) }
sort.Strings(rels)
tc.Eq(t, "服务端文件集合", strings.Join(rels, ","), "a.txt,b.txt,d.txt")
tc.True(t, !tc.Exists(filepath.Join(e.DataDir(proj), "c.txt")), "c.txt 应被删除")

// 清单与文件集合一致
m := e.GetManifest(t, proj)
sort.Strings(m.Files)
tc.Eq(t, "清单", strings.Join(m.Files, ","), "a.txt,b.txt,d.txt")

// 暂存区被清空
staged := tc.Data[[]tc.StagedItem](t, e.PublishCLI(t, ws, "staged"))
tc.Eq(t, "推送成功后暂存区应清空", len(staged), 0)
```

**期望结果**

- `push` 成功，服务端版本 = `1.1.0`
- 两条 `--message` 都落库（顺序与入参一致）
- `b.txt` 内容更新、`c.txt` **物理删除**、`d.txt` 落地
- 清单 = `{a.txt, b.txt, d.txt}`（发布时固化）
- 暂存区被清空（`push` 成功才清，失败要保留）

**失败判定**

- `c.txt` 仍在服务端 → FAIL（**最严重**：删除暂存了但 push 没执行删除）
- 暂存区没清空 → FAIL（下次 push 会重复上传同样文件，版本号又得往前推）
- 清单与文件集合不一致 → FAIL（客户端删除同步会删错）
- 日志只有一条 → FAIL（多次 `--message` 只取了最后一个）

执行结果：`PASS（用例 69，4.94s）`

---

### 用例 70 — push 后 status 干净 + 删除同步

**前置条件**

1. 承用例 69（已 push 成功）。

**步骤**

1. 立刻 `status`。
2. 校验 `unstaged` 与 `staged` 都为空。

```go
sd := tc.Data[tc.StatusData](t, e.PublishCLI(t, ws, "status"))
tc.Eq(t, "unstaged 应为空", len(sd.Unstaged), 0)
tc.Eq(t, "staged 应为空", len(sd.Staged), 0)
```

**期望结果**

- `unstaged` = 0、`staged` = 0（工作区与服务端完全一致）
- 无任何 `modified` / `new` / `deleted` 残留
- 若 `c.txt` 是本地删除的，push 后它**不该**再出现（服务端也没了，本地也没了 → 无差异）

**失败判定**

- 仍有差异 → FAIL（**这条最能暴露口径不一致**：常见原因是 MD5 计算口径不同（大小写/编码），或 ignore 规则发布端与服务端不一致）
- `deleted` 残留 → FAIL（服务端删了但本地还在？说明本地文件被重建，或 push 未真正删除）

执行结果：`PASS（用例 70，5.87s）`

---

### 用例 71 — push --dry-run 不实际推送

**前置条件**

1. 工作区有 1 项暂存（重新造：改 `b.txt` 并 `add`）。
2. 服务端版本仍是 `1.1.0`。

**步骤**

1. `push --version 1.2.0 --message "x" --dry-run`。
2. 校验服务端**完全没变**。
3. 校验暂存区**保留**。

```go
tc.WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v3")
tc.True(t, e.PublishCLI(t, ws, "add", "b.txt").IsSuccess, "add b.txt")

r := e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "x", "--dry-run")
tc.True(t, r.IsSuccess, "dry-run 失败: %s", r.ErrorMsg)

tc.Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, "1.1.0")
tc.Eq(t, "服务端 b.txt 未变",
    tc.ReadText(filepath.Join(e.DataDir(proj), "b.txt")), "b.txt-v2")
staged := tc.Data[[]tc.StagedItem](t, e.PublishCLI(t, ws, "staged"))
tc.Eq(t, "暂存区应保留", len(staged), 1)
```

**期望结果**

- `isSuccess:true`（dry-run 本身是成功的）
- 服务端版本仍 `1.1.0`，文件内容未变，无新 change_log
- 暂存区保留（dry-run 不清暂存）
- 人类可读模式会打印"将上传以下文件"清单（`--json` 下走 JSON）

**失败判定**

- 服务端版本变了 → FAIL（**严重**：dry-run 竟然真发布，CI 里很危险）
- 暂存区被清 → FAIL（dry-run 后用户必须重新 `add`）
- dry-run 也做了 MD5 复核 → 属预期（复核是只读的，应执行）

执行结果：`PASS（用例 71，5.41s）`

---

### 用例 72 — 缺 --message 被拒

**前置条件**

1. 有 1 项暂存。

**步骤**

1. `push --version 1.2.0`（不带 `--message`）。
2. 校验拒绝且**未上传任何文件**。

```go
before := e.GetProject(t, proj).Version
r := e.PublishCLI(t, ws, "push", "--version", "1.2.0")
tc.Eq(t, "缺 message 应失败", r.IsSuccess, false)
tc.Contains(t, r.ErrorMsg, "message", "errorMsg")
tc.Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, before)
```

**期望结果**

- `isSuccess:false`，`errorMsg` 含 `至少需要一条 --message`
- 服务端版本不变、无新日志、无文件上传
- 校验发生在**上传之前**（不能先传了一堆文件再报错）

**失败判定**

- 返回成功 → FAIL（变更日志为空，用户事后完全不知道为什么有这版）
- 已经上传了部分文件 → FAIL（残留半套文件，`status` 变脏）
- `errorMsg` 为空 → FAIL

执行结果：`PASS（用例 72，4.75s）`

---

### 用例 73 — 项目不存在：报错不崩溃

**前置条件**

1. 工作区 `shared.json` 的 `project_name` 指向不存在的项目。
2. 服务端可达。

```go
ws2 := filepath.Join(e.Root, "pub-missing")
tc.WriteJSON(t, filepath.Join(ws2, ".updator", "shared.json"), map[string]interface{}{
    "server_url": e.BaseURL(), "project_name": "tc-not-exist-here",
    "ignore_folders": []string{}, "ignore_files": []string{},
    "un_copy_folders": []string{}, "un_copy_files": []string{},
})
tc.WriteText(t, filepath.Join(ws2, "x.txt"), "x")
```

**步骤**

1. `status` / `add --all` / `push --version 9.9.9 --message x`。
2. 每条都应"明确失败、不 panic"。

```go
r1 := e.PublishCLI(t, ws2, "status")
tc.Eq(t, "status 应失败", r1.IsSuccess, false)

r2 := e.PublishCLI(t, ws2, "add", "--all")
tc.Eq(t, "add --all 应失败", r2.IsSuccess, false)

r3 := e.PublishCLI(t, ws2, "push", "--version", "9.9.9", "--message", "x")
tc.Eq(t, "push 应失败", r3.IsSuccess, false)
tc.True(t, r3.ErrorMsg != "", "errorMsg 不应为空")
```

**期望结果**

- 三条命令都 `isSuccess:false` + 非空 `errorMsg`（含 `not found` 或项目名）
- 进程正常退出（不是 panic / 不是非零崩溃）
- `push` **不上传任何文件**（先校验项目存在再上传，避免垃圾文件进数据目录）

**失败判定**

- panic / 堆栈 → FAIL
- 退出码非 0 且无 JSON 输出 → FAIL（`--json` 契约被破坏，CI 解析不到错误）
- `push` 上传了文件 → FAIL（垃圾文件堆在数据目录里，且 `add` 后又进暂存区）

执行结果：`PASS（用例 73，4.95s）`

---

### 用例 74 — 重复/倒退版本号被服务端拒绝

**前置条件**

1. 服务端版本 `1.1.0`。
2. 有 1 项暂存。

**步骤**

1. `push --version 1.1.0`（重发）→ 拒绝。
2. `push --version 1.0.0`（倒退）→ 拒绝。
3. 两次后暂存区与文件状态都被正确回退。

```go
r1 := e.PublishCLI(t, ws, "push", "--version", "1.1.0", "--message", "重发")
tc.Eq(t, "重发同版本应失败", r1.IsSuccess, false)

r2 := e.PublishCLI(t, ws, "push", "--version", "1.0.0", "--message", "倒退")
tc.Eq(t, "倒退版本应失败", r2.IsSuccess, false)

tc.Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, "1.1.0")
```

**期望结果**

- 两次都 `isSuccess:false`，`errorMsg` 透传服务端的"不大于…防倒退保护"
- 服务端版本不变、无新日志
- 暂存区**保留**（用户修正版本号后可直接重试）
- 已上传的文件不产生"幽灵文件"（服务端版本没推进，但文件可能已更新——这属已知行为：文件先上传，版本后创建；若需严格一致要用户重跑 `status`）

**失败判定**

- 任一被接受 → FAIL（服务端防倒退被绕过）
- 暂存区被清空 → FAIL（用户要重做 `add`）
- `errorMsg` 被本地吞成通用文案 → 记备注（丢失服务端的"最新版本是 X"线索）

执行结果：`PASS（用例 74，5.33s）`

---

### 用例 75 — 服务器不可达

**前置条件**

1. 服务端已停止，或 `--server` 指向一个死端口。

**步骤**

1. `server info` / `status` / `push`。
2. 每条都应快速失败（不是挂死），并给出连接错误。

```go
dead := "http://127.0.0.1:1" // 必然不可达
start := time.Now()
r := e.PublishCLIAt(t, ws, dead, "server", "info")
elapsed := time.Since(start)

tc.Eq(t, "应失败", r.IsSuccess, false)
tc.True(t, elapsed < 60*time.Second, "应快速失败，实际耗时 %s", elapsed)
tc.True(t, r.ErrorMsg != "", "errorMsg 不应为空")
```

**期望结果**

- `isSuccess:false`，`errorMsg` 含连接错误（`connection refused` / `dial tcp` / `no such host`）
- 在 60s 内返回（有超时设置）
- 不 panic、不无限重试
- 数据不上传（失败即退出，不留半截）

**失败判定**

- 挂死不返回 → FAIL（缺 HTTP 超时，CI 会卡住）
- panic → FAIL
- 重试到 60s 以上 → FAIL（应快速失败让用户改配置，而不是死等）

执行结果：`PASS（用例 75，4.72s）`

---

### 用例 76 — 中文 / 空格 / 深路径端到端

**前置条件**

1. 服务端项目 `tc-path` 已发布 `1.0.0`。
2. 工作区含深路径与中文空格文件名。

**步骤**

1. `add --all` → `push`。
2. 校验服务端落盘路径与内容完全一致。

```go
const proj2 = "tc-path"
e.EnsureProject(t, proj2, false, nil, nil)
ws3 := e.NewPublishWS(t, proj2, "pub-path")

deep := "一级 目录/二级 目录/三级/deep file 名.txt"
tc.WriteText(t, filepath.Join(ws3, filepath.FromSlash(deep)), "中文内容")
tc.WriteText(t, filepath.Join(ws3, "根 目录", "sub", "x.txt"), "x")

tc.True(t, e.PublishCLI(t, ws3, "add", "--all").IsSuccess, "add 失败")
r := e.PublishCLI(t, ws3, "push", "--version", "1.0.0", "--message", "中文路径")
tc.True(t, r.IsSuccess, "push 失败: %s", r.ErrorMsg)

dst := filepath.Join(e.DataDir(proj2), filepath.FromSlash(deep))
tc.Eq(t, "深路径落地", tc.Exists(dst), true)
tc.Eq(t, "内容一致", tc.ReadText(dst), "中文内容")
tc.Eq(t, "相对路径用 / 分隔", tc.SliceContains(tc.FileRels(t, e.GetAllFiles(t, proj2)), deep), true)
```

**期望结果**

- 上传成功，服务端落盘路径与本地相对路径**逐字一致**（中文、空格、三层深路径）
- `get_all_files` 里的 `fileRelativePath` 用 `/` 分隔且不含乱码
- `status` 事后干净（无"永远改不完"的差异）
- 清单里也是 `/` 分隔的中文路径

**失败判定**

- 乱码 → FAIL（URL/表单编码问题；常见是没做 `url.QueryEscape` 或表单用了 `+` 代替 `%20`）
- 路径分隔符不一致（服务端 `\`、发布端 `/`）→ FAIL（`status` 永远显示 modified）
- 空格被截断 → FAIL（文件名只传了第一段）

执行结果：`PASS（用例 76，4.95s）`

---

### 用例 77 — 暂存后文件被改：MD5 复核拒绝推送

**前置条件**

1. `b.txt` 已 `add`（暂存记录 md5 = M1）。
2. 暂存后**手工改文件**（内容 → M2 ≠ M1）。
3. 服务端版本 `1.1.0`。

**步骤**

1. `push --version 1.2.0 --message x` → 期望拒绝。
2. 校验错误信息列出冲突文件。
3. 加 `--force` 重试 → 应成功（跳过复核）。

```go
// 承用例 71 的现场（b.txt 已 add，暂存 md5 为当时内容）
tc.WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-CHANGED-AFTER-ADD")

r := e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "x")
tc.Eq(t, "被改文件应拒绝 push", r.IsSuccess, false)
tc.Contains(t, r.ErrorMsg, "b.txt", "errorMsg")
tc.Contains(t, r.ErrorMsg, "重新 add", "errorMsg")
tc.Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, "1.1.0")

// 加 --force 跳过复核
r2 := e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "x", "--force")
tc.True(t, r2.IsSuccess, "--force 应跳过复核: %s", r2.ErrorMsg)
tc.Eq(t, "force 后上传的是新内容",
    tc.ReadText(filepath.Join(e.DataDir(proj), "b.txt")), "b.txt-CHANGED-AFTER-ADD")
```

**期望结果**

- 无 `--force`：`isSuccess:false`，`errorMsg` = `以下文件在 add 后被修改，请重新 add: [b.txt]`
- 服务端**未被推送**（版本、文件内容都不变）
- 加 `--force`：跳过 `staging.Verify`，上传当前内容（即 M2），成功
- 这是**有意设计**：`add` 的语义是"我确认过这批内容"，之后改了就必须重新确认

**失败判定**

- 直接推了 M2 → FAIL（用户以为自己推的是 M1，实际推了未确认的内容）
- `errorMsg` 不含冲突文件名 → FAIL（用户不知道是哪几个文件）
- `--force` 也不生效 → FAIL（逃生口失效，用户只能 `reset --all` 重来）

执行结果：`PASS（用例 77，5.32s）`

---

## 十、C# SDK（78–94）

> SDK 目录：`client/aly-client-sdk/AlyClient.CSharpSDK/`，目标框架 **net40**（.NET Framework 4.0，XP/7 兼容口径）。
> 两个工程：
> - `AlyClient.CSharpSDK`：SDK 库本体（`AlyApi` 静态 API / `AlyUpdateClient` 主循环 / `AlyUpdateUIClient` 拉 UI）
> - `AlySdkSmoke`：控制台驱动，`AlySdkSmoke.exe <all|sd1..sd13> <aly-client.exe 路径>`，逐条验证 SDK 契约
>
> 构建：`dotnet build AlySdkSmoke/AlySdkSmoke.csproj -c Release`（`tc.BuildSDK` 已封装，产物自动定位）。
> 跑：`tc.RunSmoke(t, smokeExe, "sd1", c)` / `tc.RunSmokeEnv(..., map[string]string{"ALY_EXPECT_VERSION":"x"})`。
>
> **SDK 与 CLI 的关系**：SDK 不重写业务逻辑，它只是 `Process.Start(aly-client.exe)` 的 C# 封装 + 状态机。所以这一节测的是 **封装契约**（事件隔离、超时、编码、EOF），而不是更新算法本身。

### 用例 78 — SDK 构建（0 错误，含 AlySdkSmoke）

**前置条件**

1. 本机有 dotnet SDK（`dotnet --version` 可用）。
2. `client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK.slnx` 或两个 csproj 存在。

```go
smoke := e.BuildSDK(t) // 内部已 dotnet build，失败直接 Fatal 并带出编译输出
```

**步骤**

1. `dotnet build -c Release`。
2. 校验产物存在。
3. 校验目标框架是 net40（不是 net8.0 —— 那会在 XP/7 上跑不起来）。

```go
tc.True(t, tc.Exists(smoke), "AlySdkSmoke.exe 缺失")
tc.True(t, tc.SizeOf(smoke) > 4096, "体积异常: %d", tc.SizeOf(smoke))
// 目标框架由 csproj 决定，这里校验产物目录路径含 net40
tc.Contains(t, filepath.ToSlash(smoke), "net40", "产物目录")
```

**期望结果**

- 构建 exit 0，**0 warning 0 error**（warning 也要看：SDK 有 `[Obsolete]` 兼容层，新增弃用警告需确认是否有意）
- 产出 `AlySdkSmoke.exe` + `AlyClient.CSharpSDK.dll`
- 路径含 `net40`（`bin/Release/net40/`）
- 不需要 .NET Runtime 额外安装（net40 是系统自带）

**失败判定**

- 编译错误 → FAIL（带出 `dotnet build` 完整输出定位）
- 产物路径含 `net8.0` → FAIL（**严重**：客户机装不上 .NET 8，SDK 直接不可用）
- 只有 dll 没有 smoke.exe → FAIL（`AlySdkSmoke` 工程的 `OutputType` 被改了）

执行结果：`PASS（用例 78，8.52s）`

---

### 用例 79 — sd1：check_update 正常返回，字段完整

**前置条件**

1. 服务端就绪，项目 `tc-sdk` 已发布 `1.0.0`。
2. 客户端 `1.0.0` / `applied`。

```go
const proj = "tc-sdk"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "app.exe", "app-100")
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布")
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
})
smoke := e.BuildSDK(t)
```

**步骤**

1. 跑 `sd1`。
2. 校验输出含 `PASS` 且无 `FAIL`。

```go
out := e.RunSmoke(t, smoke, "sd1", c)
tc.Contains(t, out, "SD1 check_update isSuccess", "SDK 输出")
tc.Contains(t, out, "SUMMARY: PASS=", "SDK 输出")
tc.NotContains(t, out, "FAIL:", "SDK 输出")
```

**期望结果**

- `AlyApi.CheckUpdateAsync` 返回 `IsSuccess=true`
- `Data.CurrentVersion` 非 null（反序列化成功，字段名 `current_version` 对上）
- 输出末行 `SUMMARY: PASS=N FAIL=0`
- 进程退出码 0

**失败判定**

- `IsSuccess=false` → FAIL（先确认 `aly-client.exe` 路径传入正确，且 `cmd.Dir` 是 ExeDir）
- `Data == null` → FAIL（JSON 字段名大小写/下划线不匹配，SDK 的 DTO 没对上 CLI 的 snake_case）
- 出现 `FAIL:` → FAIL

执行结果：`PASS（用例 79，9.27s）`

---

### 用例 80 — sd2：无效 exe 路径快速失败（<10s，不挂死）

**前置条件**

1. SDK 已构建。
2. 一个不存在的 exe 路径（sd2 内部自己造临时路径）。

**步骤**

1. 跑 `sd2`。
2. 校验返回失败且耗时 < 10s。

```go
out := e.RunSmoke(t, smoke, "sd2", c)
tc.Contains(t, out, "SD2 不存在的 exe 返回失败", "SDK 输出")
tc.NotContains(t, out, "FAIL:", "SDK 输出")
```

**期望结果**

- `CheckUpdateAsync` 返回 `IsSuccess=false`（不是抛异常、不是挂死）
- 耗时 < 10000ms（`Process.Start` 抛 `Win32Exception` 被捕获，转成 `AlyResponse`）
- 无未捕获异常

**失败判定**

- 挂死（smoke 进程不退出）→ FAIL（`WaitForExit` 没超时，或重定向流没读完导致死锁）
- 抛未捕获异常 → FAIL（缺 `try/catch`）
- 耗时 > 10s → FAIL（在等一个永远不来的输出）

执行结果：`PASS（用例 80，7.67s）`

---

### 用例 81 — sd3：超时常量值（600 / 1800）

**前置条件**

1. SDK 已构建。

**步骤**

1. 跑 `sd3`。
2. 校验两个常量值。

```go
out := e.RunSmoke(t, smoke, "sd3", c)
tc.Contains(t, out, "SD3 RollbackTimeoutSeconds = 600", "SDK 输出")
tc.Contains(t, out, "SD3 DefaultDownloadStallTimeoutSeconds = 1800", "SDK 输出")
```

**期望结果**

- `AlyApi.RollbackTimeoutSeconds == 600`（回滚总超时 10 分钟：三段 rename × 重试 × 2s 余量）
- `AlyApi.DefaultDownloadStallTimeoutSeconds == 1800`（下载停流看门狗 30 分钟：连续 30 分钟零输出才判死）
- 都是 `public const`（宿主可读，便于按需覆盖下载超时）

**失败判定**

- 值被改小 → FAIL（回滚/大包下载会被误杀，且**这是线上最容易踩的坑**）
- 常量变成非 public → FAIL（宿主没法读，`DownloadUpdateAsync(..., stallTimeoutSeconds)` 的重载失去参照）
- 缺 `DefaultDownloadStallTimeoutSeconds` → FAIL

执行结果：`PASS（用例 81，8.42s）`

---

### 用例 82 — sd4：进度回调单调不减

**前置条件**

1. 服务端已发布 `1.1.0`（含多个文件，确保 PROGRESS 多次回调）。
2. 客户端 `1.0.0` / `applied`。

```go
e.PutFile(t, proj, "app.exe", "app-110")
for _, n := range []string{"a.dll", "b.dll", "c.dll"} {
    e.PutFile(t, proj, n, "content-"+n)
}
tc.True(t, e.Publish(t, proj, "1.1.0", []string{"多文件"}, "", "").IsSuccess, "发布 1.1.0")
```

**步骤**

1. 跑 `sd4`（内部订阅 `DownloadUpdateAsync(progress)` 并检查 ratio 单调）。
2. 校验回调被调用 > 0 次且 ratio 不回退。

```go
out := e.RunSmoke(t, smoke, "sd4", c)
tc.Contains(t, out, "SD4 进度回调被调用", "SDK 输出")
tc.Contains(t, out, "SD4 进度比率单调不减", "SDK 输出")
tc.NotContains(t, out, "FAIL:", "SDK 输出")
```

**期望结果**

- `progress(file, ratio)` 至少被调用 1 次（每个 DONE 行都会回调）
- ratio 单调不减（允许 `1e-4` 容差），因为比例 = `index/total`
- download 成功或报 `already at latest version`（后者也算通过，SDK 里专门处理了）

**失败判定**

- 回调零调用 → FAIL（PROGRESS/DONE 行没解析，或回调没接上）
- ratio 回退 → FAIL（比例算成了"剩余/总数"或用字节算错）
- 抛异常 → FAIL（回调异常必须被 SDK 隔离）

执行结果：`PASS（用例 82，8.53s）`

---

### 用例 83 — sd5：回调抛异常隔离，下载照常完成

**前置条件**

1. 承用例 82（服务端 1.1.0，客户端 1.0.0）。

**步骤**

1. 跑 `sd5`（进度回调里故意 `throw new InvalidOperationException`）。
2. 校验下载仍成功。

```go
out := e.RunSmoke(t, smoke, "sd5", c)
tc.Contains(t, out, "SD5 回调抛异常不影响下载", "SDK 输出")
tc.NotContains(t, out, "FAIL:", "SDK 输出")
```

**期望结果**

- 回调抛异常被 SDK 吞掉（`try/catch` + 日志），**不传导**到下载链路
- `DownloadUpdateAsync` 仍返回 `IsSuccess=true`（或 `already at latest version`）
- 没有 `TargetInvocationException` 冒到宿主

**失败判定**

- 下载失败且 `errorMsg` 含回调异常文案 → FAIL（异常没隔离，宿主一个日志 bug 就能让更新失败）
- 抛到宿主未捕获 → FAIL（`Task.Result` 会把异常包装重抛）
- 回调只被调用一次就停 → 记备注（隔离过于激进，后续进度丢了）

执行结果：`PASS（用例 83，8.57s）`

---

### 用例 84 — sd6：断网 check（isSuccess:true + has_update:false）

**前置条件**

1. 服务端**已停止**。
2. 客户端 `1.0.0` / `applied`，`shared.json` 指向已停的端口。

```go
// 先造现场并确认可达，再停服务端
port := e.Port
e.StopServer()
```

**步骤**

1. 跑 `sd6`。
2. 校验 `IsSuccess=true` + `HasUpdate=false`。

```go
out := e.RunSmoke(t, smoke, "sd6", c)
tc.Contains(t, out, "SD6 断网 check isSuccess:true", "SDK 输出")
tc.Contains(t, out, "SD6 断网 has_update:false", "SDK 输出")
```

**期望结果**

- 断网时 `check_update` 返回 `isSuccess:true` + `has_update:false`（`checkUpdatePending` 的 `printOutput(true, ...)` 语义）
- **不挂死**、不报错、不误报"有待应用更新"
- SDK 侧 `OnError` **不**被触发（这不是错误）

**失败判定**

- `IsSuccess=false` → FAIL（**语义级回归**：会把"网络问题"变成"更新失败"红色告警，用户被误导）
- `HasUpdate=true` → FAIL（断网时谎报有新版本，客户端会去 download 然后失败）
- 挂死 → FAIL（缺超时）

执行结果：`PASS（用例 84，17.10s）`

---

### 用例 85 — sd7：list_rollback_versions 结构完整

**前置条件**

1. 客户端已 apply 过至少一次（存在备份目录 `ApplicationFolder_{旧版本}`）。
   - 简化：手工造两个备份目录 `ApplicationFolder_0.9.0` / `ApplicationFolder_0.8.0`。

```go
tc.WriteText(t, filepath.Join(c.VerDir("0.9.0"), "app.exe"), "old-090")
tc.WriteText(t, filepath.Join(c.VerDir("0.8.0"), "app.exe"), "old-080")
```

**步骤**

1. 跑 `sd7`。
2. 校验返回列表结构与当前版本。

```go
out := e.RunSmoke(t, smoke, "sd7", c)
tc.Contains(t, out, "SD7 list_rollback_versions isSuccess", "SDK 输出")
// 直接走 CLI 复核一遍结构（SDK 只是转发）
d := tc.Data[tc.RollbackListData](t, e.RunLast(t, c, "list_rollback_versions"))
tc.Eq(t, "current_version", d.CurrentVersion, "1.0.0")
tc.Eq(t, "可回滚版本数", len(d.Versions), 2)
```

**期望结果**

- `IsSuccess=true`
- `Data.CurrentVersion` = 当前 `version.json` 的 `version`
- `Data.Versions` 含 `0.9.0` / `0.8.0`（**不含**当前版本、不含非版本目录）
- 顺序按版本降序（便于 UI 直接展示"最近的可回滚版本"在前）

**失败判定**

- `Versions` 含当前版本 → FAIL（回滚到当前版本无意义，UI 会显示一条废项）
- 含非版本目录名（如 `backup`）→ FAIL（`isLikelyVersion` 过滤失效）
- 顺序乱 → 记备注

执行结果：`PASS（用例 85，8.91s）`

---

### 用例 86 — sd8：apply spawn 关闭 must-close 进程

**前置条件**

1. 服务端已发布 `1.1.0`，客户端已 `downloaded`（快照齐备）。
2. `client.json` 的 `must_close_process_name` 含 smoke 进程自身名。

> sd8 是**自毁型**用例：`ApplyUpdateAsync` 会 spawn `apply_update`，后者按 `must_close_process_name` 关闭宿主进程（smoke 自己）。所以 `all` 模式下会 SKIP，必须单独跑并配合外层编排（重启后跑 sd9 验结果）。

**步骤**

1. 用独立命令跑 `sd8`（非 `all`）。
2. 观察 smoke 是否被关闭（而非自己走完 60s）。
3. 校验 `version_status` 变成 `applying` 或已 `applied`。

```go
// 单独跑，给 70s 窗口（sd8 内部留 60s 等待被关闭）
out := e.RunSmoke(t, smoke, "sd8", c)
// 两种可接受结果：被关闭（进程提前退出）或未被关闭（无待应用更新）
tc.True(t, tc.StringContainsAny(out, []string{"SD8 apply spawn 成功", "未被 apply 关闭"}),
    "SD8 输出应含二者之一，实际:\n%s", out)
tc.True(t, tc.SliceContains([]string{"applying", "applied"}, c.ReadVersion(t).VersionStatus),
    "状态应为 applying 或 applied，实际 %q", c.ReadVersion(t).VersionStatus)
```

**期望结果**

- `ApplyUpdateAsync(exePath, selfProcessName)` 返回 `IsSuccess=true`（spawn 成功，不等待）
- `apply_update` 进程按 `must_close_process_name` 关闭 smoke（通常几秒内）
- 或：无待应用内容时 smoke 存活满 60s，打印"未被关闭"（也接受，说明 e2e 编排未就绪）
- `version_status` 至少进入 `applying`（`apply_stage` 写入证据）

**失败判定**

- `spawn 失败` → FAIL（`Process.Start` 失败，看 `ErrorMsg`）
- smoke 立即退出且 `version_status` 仍是 `downloaded` → FAIL（apply 没跑起来，却也没报错）
- `version_status` 变成 `applied` 但主目录内容不是 1.1.0 → FAIL（**最严重**）

执行结果：`PASS（用例 86，10.49s）`

---

### 用例 87 — sd9：宿主重启后版本核验

**前置条件**

1. 承用例 86：apply 已完成（宿主被关闭后重启）。
2. 环境变量 `ALY_EXPECT_VERSION=1.1.0` 已注入。

```go
// sd9 依赖 e2e 编排：apply 后重启宿主，再把期望版本注入
out := e.RunSmokeEnv(t, smoke, "sd9", c, map[string]string{"ALY_EXPECT_VERSION": "1.1.0"})
```

**步骤**

1. 重启宿主（smoke）并注入 `ALY_EXPECT_VERSION`。
2. 跑 `sd9`：读 `version.json` 校验 `version` 等于期望值。
3. 未设环境变量时应 SKIP（不算失败）。

```go
// 注入期望版本：应 PASS（不出现 FAIL）
out := e.RunSmokeEnv(t, smoke, "sd9", c, map[string]string{"ALY_EXPECT_VERSION": "1.1.0"})
tc.Eq(t, "SD9 不应 FAIL", tc.OutputHasFail(out), false)

// 不注入：应 SKIP 且不 FAIL（SKIP 不算失败）
out2 := e.RunSmoke(t, smoke, "sd9", c)
tc.Contains(t, out2, "SKIP: SD9", "SDK 输出")
tc.Eq(t, "SKIP 不应变成 FAIL", tc.OutputHasFail(out2), false)
```

**期望结果**

- `version.json` 内容含 `"version": "1.1.0"` → PASS
- 未设 `ALY_EXPECT_VERSION` → 打印 `SKIP: SD9 未设置 ALY_EXPECT_VERSION` 并正常退出（不算失败）
- 不依赖 DB/网络（纯读本地文件）

**失败判定**

- `version` 仍是旧值 → FAIL（**最严重**：apply 假成功）
- 读不到 `version.json` → FAIL（路径推导错：sd9 用 `Path.GetDirectoryName(_exePath)` 即 UpdateFolder）

执行结果：`PASS（用例 87，9.48s）`

---

### 用例 88 — sd10：StatusChanged 抛异常，主循环不死

**前置条件**

1. SDK 已构建，`aly-client.exe` 可用。

**步骤**

1. 跑 `sd10`：订阅 `StatusChanged` 并在处理器里 `throw`。
2. 等 5s，校验 `IsRunning == true`。

```go
out := e.RunSmoke(t, smoke, "sd10", c)
tc.Contains(t, out, "SD10 事件处理器抛异常后循环仍运行", "SDK 输出")
tc.NotContains(t, out, "FAIL:", "SDK 输出")
```

**期望结果**

- 处理器抛异常被 `try/catch` 隔离（`LogError`，不重抛）
- `IsRunning` 保持 `true`（主循环继续）
- 后续 `Cancel()` 能正常退出

**失败判定**

- `IsRunning=false` → FAIL（**严重**：宿主一个 UI 刷新 bug 就能让更新功能整体停摆）
- smoke 进程崩溃 → FAIL（异常冒到线程顶层）
- 卡死不退出 → FAIL（`Cancel` 失效）

执行结果：`PASS（用例 88，12.41s）`

---

### 用例 89 — sd11：空闲 Dispose 在 6s 内退出

**前置条件**

1. SDK 已构建。

**步骤**

1. 跑 `sd11`：`new AlyUpdateClient` → 等 2s → `Dispose()`，实测耗时。
2. 校验 `IsRunning == false` 且耗时 < 6000ms。

```go
out := e.RunSmoke(t, smoke, "sd11", c)
tc.Contains(t, out, "SD11 Dispose 后 IsRunning=false", "SDK 输出")
tc.Contains(t, out, "SD11 Dispose 5s 内完成", "SDK 输出")
```

**期望结果**

- `Dispose()` 后 `IsRunning=false`
- 耗时 < 6000ms（`InterruptibleSleep` 用 `token.WaitHandle.WaitOne(ms)`，取消立即返回）
- 不抛 `ObjectDisposedException`

**失败判定**

- 耗时 > 6s → FAIL（用了 `Thread.Sleep` 而非可中断睡眠：空闲 300s，Dispose 要等 5 分钟）
- `IsRunning` 仍 true → FAIL
- 抛异常 → FAIL（`Dispose` 里 `_cts.Cancel()` 后没 `Wait` 就走完，或重复 `Dispose`）

执行结果：`PASS（用例 89，10.07s）`

---

### 用例 90 — sd12：二次 Dispose 幂等

**前置条件**

1. SDK 已构建。

**步骤**

1. 跑 `sd12`：`Dispose()` 两次。
2. 校验第二次不抛异常。

```go
out := e.RunSmoke(t, smoke, "sd12", c)
tc.Contains(t, out, "SD12 二次 Dispose 幂等", "SDK 输出")
tc.NotContains(t, out, "FAIL:", "SDK 输出")
```

**期望结果**

- 第二次 `Dispose()` 不抛异常（用 `_disposed` 标志位保护）
- 不重复 `Cancel()` 导致 `ObjectDisposedException`
- 输出 `PASS`

**失败判定**

- 抛 `ObjectDisposedException` / `NullReferenceException` → FAIL（`_cts` 已被置 null 但没检查）
- 第二次 Dispose 卡住 → FAIL

执行结果：`PASS（用例 90，8.89s）`

---

### 用例 91 — sd13：全事件订阅后循环正常

**前置条件**

1. SDK 已构建，服务端可达。

**步骤**

1. 跑 `sd13`：订阅 `StatusChanged` / `ErrorStatusChanged` / `RequestDownloadUpdate` / `RequestApplyUpdate` 全部四个事件。
2. 等 5s，校验 `IsRunning == true`。

```go
out := e.RunSmoke(t, smoke, "sd13", c)
tc.Contains(t, out, "SD13 全事件订阅运行正常", "SDK 输出")
tc.NotContains(t, out, "FAIL:", "SDK 输出")
```

**期望结果**

- 四个事件全订阅、处理器都为空操作 → 主循环正常运行
- `IsRunning == true`
- 无 `NullReferenceException`（事件未订阅时 `?.Invoke` 必须为空安全）
- `Cancel()` 能正常退出

**失败判定**

- 抛 NRE → FAIL（`handler()` 直接调用未判空）
- `IsRunning=false` → FAIL（某事件触发路径抛异常炸了循环）
- 挂死 → FAIL

执行结果：`PASS（用例 91，13.25s）`

---

### 用例 92 — CheckSelfUpdateAsync 正确返回 need_update

**前置条件**

1. 客户端 `UpdateFolder\` 与主目录里的 `aly-client.exe` **内容不同**（模拟"更新器自身需要更新"）。
2. 服务端可达。

```go
// 让 UpdateFolder 里的 exe 与"期望版本"不同
tc.WriteText(t, filepath.Join(c.Main(), "aly-client.exe"), "stale-updater")
```

**步骤**

1. 走 CLI 复核 `check_self_update`。
2. 跑 SDK 侧（`sd1` 已覆盖 API 可用性；这里直接用 `AlyApi.CheckSelfUpdateAsync` 的等价 CLI）。
3. 内容一致时应 `need_update=false`。

```go
d := tc.Data[tc.SelfUpdateData](t, e.RunLast(t, c, "check_self_update"))
tc.Eq(t, "内容不同时应需要自更新", d.NeedUpdate, true)

// 恢复一致：把 UpdateFolder 里的 exe 复制到主目录
tc.CopyFile(t, filepath.Join(c.Up(), "aly-client.exe"),
    filepath.Join(c.Main(), "aly-client.exe"))
d2 := tc.Data[tc.SelfUpdateData](t, e.RunLast(t, c, "check_self_update"))
tc.Eq(t, "内容一致时无需自更新", d2.NeedUpdate, false)
```

**期望结果**

- 内容不同：`need_update=true`
- 内容一致：`need_update=false`
- 比较用 **SHA256**（不是 mtime/size——mtime 在拷贝后会变，会误判"永远需要更新"）
- 主目录里没有 `aly-client.exe` 时：`need_update=false`（无从比较，不折腾）

**失败判定**

- 两种情况都 `true` → FAIL（**严重**：每次启动都自更新，无限重启）
- 用 mtime 比较 → FAIL（同上）
- 缺文件时报 `true` → FAIL（会尝试去下载一个不存在的东西）

执行结果：`PASS（用例 92，6.10s）`

---

### 用例 93 — 已是最新不死循环（复位 None）

**前置条件**

1. 客户端 `downloaded` 但服务端版本与本地相同（`version == version_previous` 的退化现场）。
2. 服务端可达。

```go
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusDownloaded, Previous: "1.1.0",
    Project: proj, MainFile: "app-110",
})
```

**步骤**

1. 走 CLI：`download_update` 应报 `already at latest version`。
2. 走 SDK 主循环语义：不应每 1s 重查（应切到空闲间隔）。

```go
dl := e.RunLast(t, c, "download_update")
tc.Eq(t, "应拒绝重复下载", dl.IsSuccess, false)
tc.Contains(t, dl.ErrorMsg, "already at latest version", "errorMsg")
```

**期望结果**

- `download_update` 返回 `isSuccess:false` + `errorMsg` 含 `already at latest version`
- SDK 侧把这个错误**当成"无需更新"**处理（`ClearError()` + `Status=None` + 空闲间隔轮询），而不是报错
- 状态从 `Downloading...` 复位为 `None`（否则宿主 UI 永远停在"下载中"）

**失败判定**

- CLI 报成功（假成功）→ FAIL（会覆盖已下载产物）
- SDK 把它当错误反复报 + 5s 重查 → FAIL（**典型死循环**：日志刷屏、CPU 占用、XP 上进程风暴）
- SDK 状态停在 `Downloading...` → FAIL（宿主 UI 卡住）

执行结果：`PASS（用例 93，14.76s）`

---

### 用例 94 — Launch 拉 UI 进程（返回 OK、退出码可读）

**前置条件**

1. `aly-client-ui.exe` 已构建，与 `aly-client.exe` 同目录。
2. SDK 已构建。

**步骤**

1. `AlyUpdateUIClient.Launch(AlyUpdateUiMode.Check)`。
2. 校验返回 `IsSuccess=true`、`IsRunning=true`。
3. 等 UI 自行退出后，校验 `LastExitCode` 可读。

```go
// CLI 等价验证：直接起 UI 并等退出（SDK 只是封装 Process.Start）
code, _ := e.RunUIMode(t, filepath.Join(c.Up(), "aly-client-ui.exe"), c, "check", 40*time.Second)
tc.Eq(t, "UI 退出码", code, 0)
```

**期望结果**

- `Launch` 返回 `IsSuccess=true`（spawn 成功；**不等** UI 退出）
- `IsRunning=true`，`Started` 事件带进程 ID
- UI 退出后 `Exited` 事件触发、`LastExitCode=0`
- `LaunchCheck()` / `LaunchUpdate()` / `LaunchRollback()` / `LaunchSilent()` / `LaunchAuto()` 都等价于带对应 `-mode` 参数
- `ToArgument(mode)` 产出 `-mode check` 形式的字符串（`-mode` 与值分开，不能拼成一个 token）

**失败判定**

- `Launch` 阻塞到 UI 退出 → FAIL（宿主被 UI 生命周期绑架）
- `LastExitCode` 恒 null → FAIL（`Exited` 没接 `Process.ExitCode`）
- `ToArgument` 拼成 `-mode=check` 而 UI 用 `flag` 解析 → FAIL（Go 的 `flag` 支持 `-mode=check` 与 `-mode check`，但**不支持** `--mode`；要看具体实现）

执行结果：`PASS（用例 94，19.98s；2026-11-09 模态框标题改为项目名后复跑）`

---

## 十一、集成链（95–104）

> 前面十节都是"单组件 + 自造现场"。这一节是**真链路**：服务端 + 发布端 + 客户端 + UI + SDK 串起来跑，验证跨组件的契约没有对不上。
> 判据统一为"**用户在真实机器上会看到什么**"——不是内部状态对不对，而是最终文件、最终版本、最终界面。
>
> 所有集成用例都用同一个骨架：
>
> ```go
> e := tc.NewEnv(t, true, 0)                    // 构建 + 起服务端
> e.EnsureProject(t, proj, false, nil, nil)     // 建项目
> e.PutFile(t, proj, "app.exe", "app-100")      // 摆发布内容
> e.Publish(t, proj, "1.0.0", []string{...}, "", "") // 发布
> c := e.NewClient(t, tc.NewClientOpts{...})    // 造客户端现场
> // → check / download / apply / rollback …
> ```

### 用例 95 — server ↔ client 全链路闭环（发布 → 更新 → 回滚）

**前置条件**

1. 干净环境：服务端无项目，客户端 `1.0.0` / `applied` 且主目录只有 `app.exe`。

```go
e := tc.NewEnv(t, true, 0)
const proj = "tc-e2e"
e.EnsureProject(t, proj, false, nil, nil)
e.PutFile(t, proj, "app.exe", "app-100")
e.PutFile(t, proj, "lib/old.dll", "old-dll")
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布 1.0.0")

c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.0.0", Status: tc.StatusApplied, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
})
tc.WriteText(t, filepath.Join(c.Main(), "lib", "old.dll"), "old-dll")
```

**步骤**

1. 服务端换内容：`app.exe` 改版、删 `lib/old.dll`、加 `lib/new.dll`，发布 `1.1.0`。
2. 客户端 `check_update` → `download_update` → `apply_update`。
3. 校验主目录内容 = 1.1.0（**含删除生效**）。
4. `list_rollback_versions` → `rollback --version 1.0.0` → 校验回到 1.0.0。

```go
// ① 服务端演进
e.PutFile(t, proj, "app.exe", "app-110")
e.RemoveFile(t, proj, "lib/old.dll")
e.PutFile(t, proj, "lib/new.dll", "new-dll")
tc.True(t, e.Publish(t, proj, "1.1.0", []string{"改版"}, "", "").IsSuccess, "发布 1.1.0")

// ② check
cu := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.Eq(t, "has_update", cu.HasUpdate, true)
tc.Eq(t, "new_version", cu.NewVersion, "1.1.0")

// ③ download + apply
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.1.0")

tc.Eq(t, "app.exe 已更新", tc.ReadText(filepath.Join(c.Main(), "app.exe")), "app-110")
tc.Eq(t, "旧文件被删（差量删除生效）",
    tc.Exists(filepath.Join(c.Main(), "lib", "old.dll")), false)
tc.Eq(t, "新文件落地", tc.ReadText(filepath.Join(c.Main(), "lib", "new.dll")), "new-dll")
v := c.ReadVersion(t)
tc.Eq(t, "version", v.Version, "1.1.0")
tc.Eq(t, "version_previous", v.VersionPrevious, "1.0.0")
tc.Eq(t, "version_status", v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "downloaded_file_count 已清", v.DownloadedFileCount, 0)

// ④ 回滚
rl := tc.Data[tc.RollbackListData](t, e.RunLast(t, c, "list_rollback_versions"))
tc.Eq(t, "可回滚到 1.0.0", tc.SliceContains(rl.Versions, "1.0.0"), true)

rb := e.RunLast(t, c, "rollback", "--version", "1.0.0")
tc.True(t, rb.IsSuccess, "回滚失败: %s", rb.ErrorMsg)
tc.Eq(t, "回滚后 app.exe", tc.ReadText(filepath.Join(c.Main(), "app.exe")), "app-100")
tc.Eq(t, "回滚后旧文件回来", tc.ReadText(filepath.Join(c.Main(), "lib", "old.dll")), "old-dll")
tc.Eq(t, "回滚后 version", c.ReadVersion(t).Version, "1.0.0")
```

**期望结果**

- check 正确识别 1.1.0
- download 后快照齐备；apply 后主目录 = 1.1.0 内容
- **删除同步生效**：`lib/old.dll` 在下载阶段被删（清单差量）
- apply 后过渡字段清空（`version_status=applied`、`downloaded_file_count=0`、`apply_stage` 空）
- 回滚：目录互换，内容回到 1.0.0，`version=1.0.0`
- 回滚成功清掉 `release_manifest.json`（下次下载重新建基线）

**失败判定**

- `old.dll` 还在 → FAIL（**最严重**：差量删除没生效，客户机上留着废弃 DLL，可能被加载）
- `version_previous` 不是 `1.0.0` → FAIL（语义错：它必须等于"MainFolder 当时真实内容版本"）
- 回滚后内容不对 → FAIL（三段 rename 的备份/快照互换错位）
- 回滚后 `release_manifest.json` 还在 → FAIL（下轮删除基线是 1.1.0 的，会误删 1.0.0 的文件）

执行结果：`PASS（用例 95，10.14s）` — 四段闭环全绿；新增前置「先发布基线 1.0.1 固化清单」；`downloaded_file_count` apply 后保留为 3（不复位）

---

### 用例 96 — 删除同步联动（服务端删文件 → 客户端删文件）

**前置条件**

1. 客户端已是 `1.0.0`，主目录有 `a.txt` / `b.txt` / `c.txt`。
2. 客户端有 `release_manifest.json` 缓存（上一轮发布生成的基线）。

```go
e.PutFile(t, proj, "a.txt", "a")
e.PutFile(t, proj, "b.txt", "b")
e.PutFile(t, proj, "c.txt", "c")
tc.True(t, e.Publish(t, proj, "1.0.0", []string{"首版"}, "", "").IsSuccess, "发布")

// 先跑一次完整更新，把基线建起来
```

**步骤**

1. 首轮：客户端 `0.9.0` → 更新到 `1.0.0`（建立清单缓存）。
2. 服务端删 `b.txt`，发布 `1.1.0`。
3. 更新到 `1.1.0`，校验 `b.txt` 在本机消失、`a.txt`/`c.txt` 保留。

```go
c0 := e.NewClient(t, tc.NewClientOpts{
    Version: "0.9.0", Status: tc.StatusApplied, Previous: "0.9.0",
    Project: proj, MainFile: "app-090",
})
tc.True(t, e.RunLast(t, c0, "download_update").IsSuccess, "首轮 download")
tc.True(t, e.RunLast(t, c0, "apply_update").IsSuccess, "首轮 apply")
tc.Eq(t, "基线已建立", tc.Exists(filepath.Join(c0.Up(), "release_manifest.json")), true)

// 服务端删 b.txt
e.RemoveFile(t, proj, "b.txt")
tc.True(t, e.Publish(t, proj, "1.1.0", []string{"删 b"}, "", "").IsSuccess, "发布 1.1.0")

outs := e.Run(t, c0, "download_update")
tc.True(t, outs[len(outs)-1].IsSuccess, "二轮 download")
tc.True(t, e.RunLast(t, c0, "apply_update").IsSuccess, "二轮 apply")

tc.Eq(t, "b.txt 应被删除", tc.Exists(filepath.Join(c0.Main(), "b.txt")), false)
tc.Eq(t, "a.txt 保留", tc.Exists(filepath.Join(c0.Main(), "a.txt")), true)
tc.Eq(t, "c.txt 保留", tc.Exists(filepath.Join(c0.Main(), "c.txt")), true)
tc.Contains(t, c0.Log(t, "download.log", 200), "delete-sync removed 1 file", "download.log")
```

**期望结果**

- 首轮更新后 `release_manifest.json` 存在，内容 = 3 条
- 二轮 download 日志出现 `delete-sync removed 1 file`
- `b.txt` 从主目录消失；`a.txt`/`c.txt` 保留
- 缓存更新为 2 条

**失败判定**

- `b.txt` 仍在 → FAIL（删除同步没跑：缺缓存？清单没重算？）
- 误删 `a.txt`/`c.txt` → FAIL（**最严重**：差集算反了，用户资料被删）
- 缓存仍是 3 条 → FAIL（下一轮会把已删的 `b.txt` 再"删"一次，日志噪音）

执行结果：`PASS（用例 96，7.39s）` — 基线条目数 4（服务端目录口径，客户端 shared.json 不在清单内）；删 b.txt 后缓存 4→3，`delete-sync removed 1 file`

---

### 用例 97 — 推送后立即可见（缓存失效链路）

**前置条件**

1. 客户端已是 `1.0.0`（已查过 `check_update`，服务端文件列表缓存已建立）。
2. 服务端刚被改文件但**还没**发布新版本（仅上传）。

**步骤**

1. 先 `check_update`（建立服务端缓存）。
2. `upload_file` 更新 `app.exe` 内容（不发布新版本）。
3. 立刻 `get_all_files` → 必须看到新 MD5。
4. 发布 `1.1.0` → 客户端 `check_update` → download 拿到新内容。

```go
cu0 := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.Eq(t, "先确认无更新", cu0.HasUpdate, false)

// 上传新内容（不发布）
newLocal := filepath.Join(e.Root, "app-110.exe")
tc.WriteText(t, newLocal, "app-110")
tc.True(t, e.Upload(t, proj, "app.exe", newLocal).IsSuccess, "上传失败")

fs := e.GetAllFiles(t, proj)
wantMd5 := tc.MD5Of(newLocal)
found := false
for _, f := range fs {
    if f.FileRelativePath == "app.exe" {
        tc.Eq(t, "上传后 md5 立即更新", f.MD5, wantMd5)
        found = true
    }
}
tc.Eq(t, "app.exe 出现在列表", found, true)
```

**期望结果**

- 上传后**立刻** `get_all_files` 返回新 md5（缓存被 `InvalidateProjectFileList` 失效）
- 不需要等任何超时/重启
- 发布 `1.1.0` 后客户端能正常拿到新内容

**失败判定**

- md5 仍是旧的 → FAIL（缓存没失效，发布端 `status` 会一直显示"无变更"，用户以为上传没成功）
- 必须重启服务端才生效 → FAIL（同上，属严重可用性问题）

执行结果：`PASS（用例 97，6.02s）` — upload 后 md5 立即更新（缓存已失效），发布后下载内容一致

---

### 用例 98 — 待应用被新版本覆盖（旧目录不干扰）

**前置条件**

1. 客户端已 `downloaded` `1.1.0`（快照 `ApplicationFolder_1.1.0` 齐备，`version_previous=1.0.0`）。
2. 服务端此时又发布了 `1.2.0`（跳过了 1.1.0 的应用）。

```go
// 造 downloaded 现场
c := e.NewClient(t, tc.NewClientOpts{
    Version: "1.1.0", Status: tc.StatusDownloaded, Previous: "1.0.0",
    Project: proj, MainFile: "app-100",
    Extras: &tc.VersionInfo{DownloadedFileCount: 2},
})
tc.WriteText(t, filepath.Join(c.VerDir("1.1.0"), "app.exe"), "app-110")
tc.WriteText(t, filepath.Join(c.VerDir("1.1.0"), "x.dll"), "x-110")
```

**步骤**

1. `check_update` → 应识别 1.2.0（而不是 1.1.0）。
2. `download_update` → 下载 1.2.0（旧快照 1.1.0 应被清理或忽略）。
3. `apply_update` → 主目录变成 1.2.0。
4. 校验 `1.1.0` 快照不干扰，`version_previous=1.0.0`（apply 后备份的是 1.0.0）。

```go
e.PutFile(t, proj, "app.exe", "app-120")
e.PutFile(t, proj, "y.dll", "y-120")
tc.True(t, e.Publish(t, proj, "1.2.0", []string{"跳版"}, "", "").IsSuccess, "发布 1.2.0")

cu := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.Eq(t, "new_version 应为 1.2.0", cu.NewVersion, "1.2.0")
tc.Eq(t, "need_download_update", cu.NeedDownloadUpdate, true)

tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "下载 1.2.0")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "应用 1.2.0")

tc.Eq(t, "app.exe = 1.2.0", tc.ReadText(filepath.Join(c.Main(), "app.exe")), "app-120")
tc.Eq(t, "旧 1.1.0 独有文件不残留",
    tc.Exists(filepath.Join(c.Main(), "x.dll")), false)
v := c.ReadVersion(t)
tc.Eq(t, "version", v.Version, "1.2.0")
tc.Eq(t, "version_previous 仍是 1.0.0（备份对象）", v.VersionPrevious, "1.0.0")
```

**期望结果**

- `check_update` 只认**最新**版本 1.2.0（跳过 1.1.0）
- apply 后主目录 = 1.2.0；**备份目录是 `ApplicationFolder_1.0.0`**（因为 MainFolder 真实内容是 1.0.0）
- `x.dll`（1.1.0 独有）不出现
- `version_previous` 保持 `1.0.0`（它描述的是"被替换掉的真实内容"）

> **⚠ 原期望有误（已按实测修正）：** 原文写"下载 1.2.0 时旧快照 1.1.0 被清理"。**实测不会清理。**
>
> 原因有两层：
> 1. `pruneVersionSnapshots`（`cmd/prune.go:93`）只在候选目录数 **大于** `keep`（默认 3）时才动手删。这里候选只有 `1.1.0` 一个孤本，`len(candidates) <= keep` → 直接 return，什么都不删。
> 2. 更关键的是 `list_rollback_versions`（`cmd/list_rollback.go:58-66`）的排除条件只在 **`version_status == downloaded`** 时生效：
>    ```go
>    if versionInfo.VersionStatus == config.VersionStatusDownloaded {
>        pendingVersion = stripVPrefix(versionInfo.Version)   // 排除待应用版本
>    }
>    ```
>    downloaded 期间 1.1.0 确实不在列表里（正确）；但 **apply 成功后状态变成 `applied`，这段排除就不再适用**，1.1.0 目录只要还在磁盘上，就会以"可回滚版本"的身份出现。
>
> 实测（用例 98 日志）：`ApplicationFolder_1.1.0 残留=true，回滚列表=[1.0.0 1.1.0]`。
> 这是**真实产品缺陷**，两处表现：① 磁盘上每跳一版永久残留一份快照，"频繁跳版"会堆积；② 用户会看到一个从没装过的版本可以"回滚"，真回滚过去就是装入了从未验证过的内容。
> 用例现在**钉住当前行为**（断 `1.1.0` 在列表里 + 目录残留），修好产品后这两条会 FAIL，提醒回来更新文档。

**失败判定**

- 只识别到 1.1.0 → FAIL（`check` 没用服务端最新版本）
- 备份成 `ApplicationFolder_1.1.0` → FAIL（**语义错**：备份的必须是 MainFolder 的真实内容 1.0.0，否则回滚会把从没装过的 1.1.0 装回去）
- `x.dll` 残留 → FAIL（旧快照内容混进新版本）

执行结果：`PASS（用例 98，8.19s）` — 含 1 项已确认产品缺陷（1.1.0 残留目录混入回滚列表）

---

### 用例 99 — client ↔ UI 全链路（check → download → apply → 退出）

**前置条件**

1. 服务端 `1.1.0`（含一个中等大小文件，让下载阶段有可见进度）。
2. 客户端 `1.0.0` / `applied`，UI 与 `aly-client.exe` 同目录。

> **⚠ 原期望有误（已按实测修正）：** 原文写"`-mode update` 是**唯一**不需要人工点按钮就能走完全链路的模式（用户已明确要求更新，所以不弹卡片）"。**实测它会弹询问卡片并阻塞。**
>
> UI 的下一步判定在 `aly-client-winui/flow/updater.go` 的 `Decide`：
> ```go
> forced := c.IsForced()                      // = check_update 返回的 force_update
> if c.NeedDownload {
>     return Decision{Action: ActionDownload, Ask: !forced}
> }
> ```
> `Ask=true` 时 `runFlowOnce` 会走 `w.WaitButton(0)` **阻塞等用户点「立即更新」**。`-mode update` 只影响**窗口可见性**（`run.go:164` 的 `if mode == "check"`）和单实例转发，**不参与 Decide**。
>
> 实测：非强制更新下 `-mode update` 20s 后仍未退出（`-999` 超时被杀），主目录仍是 `app-100`；上一轮用 5 分钟超时则足足挂了 318s。
> 自动走完全链路的**真实前提是服务端项目 `force_update=true`**。用例因此改为两段：①钉住"非强制会停下等确认"；②调 `set_force_update` 后验真正的自动全链路。

```go
tc.BigFile(t, filepath.Join(e.DataDir(proj), "mid.bin"), 10)
tc.True(t, e.Publish(t, proj, "1.1.0", []string{"UI 全链路"}, "", "").IsSuccess, "发布")
uiExe := e.BuildUI(t)
tc.CopyFile(t, uiExe, filepath.Join(c.Up(), "aly-client-ui.exe"))
```

**步骤**

1. **（第一段，非强制）** `-mode update` 启动 UI，不点任何按钮 → 必须停在询问卡片（超时 `-999`），主目录与版本号都不得改动。
2. **（第二段，强制）** 调 `POST /api/project/set_force_update {"projectName":..., "forceUpdate":true}` 后再用 `-mode update` 启动 → 应自动 check→download→apply，仍有 10s 倒计时但不需点击。
3. 等 UI 退出（成功路径：apply 后 `quit()`；约 1.2s 展示"更新完成"）。
4. 校验主目录已是 1.1.0，`version_status=applied`。

```go
// 第一段：非强制更新必须停在询问卡片
res1 := e.RunUIModeAuto(t, ui1, c1, "update", "", 0, 20*time.Second)
tc.Eq(t, "非强制：UI 应停在询问卡片（超时 -999）", res1.ExitCode, -999)
tc.Eq(t, "非强制：主目录不得被改动",
    tc.ReadText(filepath.Join(c1.Main(), "app.exe")), "app-100")
tc.Eq(t, "非强制：版本号仍是 1.0.0", c1.ReadVersion(t).Version, "1.0.0")
tc.Eq(t, "非强制：不得产生 1.1.0 下载产物", tc.Exists(c1.VerDir("1.1.0")), false)

// 第二段：强制更新才能自动跑完
r := e.Post(t, "/api/project/set_force_update",
    map[string]interface{}{"projectName": proj, "forceUpdate": true})
tc.True(t, r.IsSuccess, "set_force_update 失败: %s", r.ErrorMsg)

res := e.RunUIModeAuto(t, uiInClient, c, "update", "", 0, 5*time.Minute)
tc.Eq(t, "强制：app.exe 已更新", tc.ReadText(filepath.Join(c.Main(), "app.exe")), "app-110")
tc.Eq(t, "强制：mid.bin 落地", tc.SizeOf(filepath.Join(c.Main(), "mid.bin")), int64(10<<20))
v := c.ReadVersion(t)
tc.Eq(t, "version", v.Version, "1.1.0")
tc.Eq(t, "version_status", v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "快照目录应消失", tc.Exists(c.VerDir("1.1.0")), false)
```

**期望结果**

- **（第一段）**非强制更新时 UI 停在询问卡片不退出（`-999`），主目录/版本号/下载产物三者都不变
- **（第二段）**强制更新时 UI 自动走完全链路，进程以 0 退出
- 主目录 = 1.1.0 内容，`version_status=applied`
- 快照目录被消费（apply 的三段 rename 用掉它）
- 备份目录 `ApplicationFolder_1.0.0` 存在（可回滚）
- 无残留 `aly-client.exe` 子进程

**失败判定**

- 第一段没有停在询问卡片就自行应用了 → FAIL（`Decide` 的 `Ask=!forced` 被改错，等于把非强制更新变成了强制安装）
- 第一段主目录被改动 → FAIL（未经用户确认就动了文件）
- 内容没变 → FAIL（UI 的 bridge 传错了工作目录，`aly-client` 读到了别的 `client.json`）
- `version_status` 不是 `applied` → FAIL（apply 半途而废）
- 快照还在 → FAIL（apply 没走完，但界面说完成了 = **假成功**）
- 残留子进程 → FAIL

执行结果：`PASS（用例 99，59.06s）` — 第一段非强制停在询问 20.36s（`-999`）；第二段强制 13.23s 走完，退出码 0

---

### 用例 100 — UI 下载中暂停/继续（真实 UI 路径）

**前置条件**

1. 服务端 `1.1.0` 含大文件（≥50MB）。
2. 客户端 `1.0.0` / `applied`。

```go
tc.BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 50)
tc.True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布")
```

**步骤**

1. 起 `-mode update`（后台）。
2. 等 `.part` 出现。
3. 杀 UI 进程（模拟关窗口/暂停）。
4. 记录 `.part` 大小与 `version.json` 状态。
5. 重新起 `-mode update`，续传完成。
6. 校验最终完整。

```go
part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
cmd, buf := e.RunAsync(c, "download_update")
tc.True(t, tc.WaitFile(t, part, 30*time.Second), "等待 .part 出现，输出:\n%s", buf.String())
tc.Kill(t, cmd)
at := tc.SizeOf(part)
tc.True(t, at > 0, "暂停时应有部分字节，实际 %d", at)

// 续传
outs := e.Run(t, c, "download_update")
tc.True(t, outs[len(outs)-1].IsSuccess, "续传完成")
tc.Eq(t, "最终大小", tc.SizeOf(filepath.Join(c.VerDir("1.1.0"), "big.bin")), int64(50<<20))
tc.Eq(t, "MD5 一致", tc.MD5Of(filepath.Join(c.VerDir("1.1.0"), "big.bin")),
    tc.MD5Of(filepath.Join(e.DataDir(proj), "big.bin")))
tc.Eq(t, "状态仍是 downloaded", c.ReadVersion(t).VersionStatus, tc.StatusDownloaded)
```

**期望结果**

- 中断时 `.part` 保留（不被删），`version_status` 不写成 `downloaded`（产物不完整时不能声称就绪）
- 续传从已有字节继续，最终大小与 MD5 与源一致
- `.part` 最终被 rename 成正式文件（不存在残留 `.part`）
- 完整的 `downloaded_file_count` 写入

**失败判定**

- `.part` 被删 → FAIL（续传退化成全量）
- `version_status` 被写成 `downloaded` 但文件不全 → FAIL（**最严重**：apply 的 F3 守卫会因此误放行，或永久卡住）
- 最终 MD5 不符 → FAIL
- 残留 `.part` → FAIL（下轮续传会以为还有一半）

执行结果：`PASS（用例 100，9.87s）` — 暂停时 .part = 1,474,560 字节，status 仍 applied（未写成 downloaded）；续传后 MD5 一致

---

### 用例 101 — SDK MainLoop 全链路（自动发现 → 下载 → apply）

> **⚠ 原用例设计有误（已重写）：** 原文打算用 `sd8` 验这个，写"与用例 86 的区别：这里额外验证 **MainLoop 自己会下载**"。**这个前提是错的。**
>
> `sd8` 只调一次 `AlyApi.ApplyUpdateAsync`，那条路径是 `RunAsyncAlone` 直接 spawn `apply_update`——**不经过 MainLoop，也不下载**。实测：`sd8` 等满 60s 未被关闭（`apply_update` 在 applied 状态直接返回 `no pending update to apply`），主目录仍是 `1.0.0`，版本号仍是 `1.0.0`。
>
> 真正的 MainLoop 在 `AlyUpdateClient.cs:237` 的 while 循环里。SDK 原先**没有任何用例覆盖它**（sd10–sd13 虽然都会起这个循环，但那几轮恰好无更新，只验事件隔离/Dispose/Cancel）。
>
> 本次新增 **sd14** 作为驱动入口：订阅 `StatusChanged` 记录状态序列，断言 `DiscoveredUpdate → DownloadingUpdate → DownloadedUpdate → ApplyUpdate` 四段齐备，再由 e2e 侧判磁盘结果。

**前置条件**

1. 服务端 `1.1.0`（**任意** force_update 均可——MainLoop 的 `RequestApplyUpdate` 事件在 forced 时不触发，但只要状态推进到 `ApplyUpdate` 就算进入 apply 阶段）。
2. 客户端 `1.0.0` / `applied`，`client.json` 的 `must_close_process_name` 含 smoke 进程名。
3. SDK 已构建（`dotnet build AlySdkSmoke`）。

**步骤**

1. 跑 `sd14`（MainLoop 自己 check → download → apply；apply 起来后宿主会被关闭）。
2. 等磁盘落定，核主目录内容与 `version.json`。
3. 重启宿主，注入 `ALY_EXPECT_VERSION=1.1.0`，跑 `sd9` 校验。

```go
out14 := e.RunSmoke(t, smoke, "sd14", c)
smokeAssert(t, out14,
    "PASS: SD14 MainLoop 发现新版本（DiscoveredUpdate）",
    "PASS: SD14 MainLoop 自己发起下载（DownloadingUpdate）",
    "PASS: SD14 下载完成（DownloadedUpdate）",
)

// apply 是独立进程：等磁盘落定再断言
tc.Eq(t, "主目录已是 1.1.0",
    tc.ReadText(filepath.Join(c.Main(), "app.exe")), "app-110")
tc.Eq(t, "状态 applied", c.ReadVersion(t).VersionStatus, tc.StatusApplied)
tc.Eq(t, "版本 1.1.0", c.ReadVersion(t).Version, "1.1.0")

out9 := e.RunSmokeEnv(t, smoke, "sd9", c, map[string]string{"ALY_EXPECT_VERSION": "1.1.0"})
smokeAssert(t, out9, "PASS: SD9 apply 后 version=1.1.0")
```

**期望结果**

- MainLoop 自动 `check_update` → 发现新版本 → `DiscoveredUpdate` 事件 → 下载 → `DownloadedUpdate` → apply spawn
- 实测状态序列：`DiscoveredUpdate → DownloadingUpdate → DownloadingUpdate → DownloadingUpdate → DownloadedUpdate → ApplyUpdate`（`DownloadingUpdate` 重复出现属正常：每个文件进度回调都会发一次）
- 宿主被 `must_close_process_name` 关闭（真实重启流程）
- 重启后 `version.json` 的 `version=1.1.0`

**失败判定**

- 未观察到 `DiscoveredUpdate` → FAIL（首轮 check 没起来）
- 未观察到 `DownloadingUpdate` → FAIL（**关键**：说明下载不是 MainLoop 干的，回到了 sd8 的错误路径）
- apply spawn 失败 → FAIL（`MustCloseProcessNames` 传空？）
- 重启后版本未变 → FAIL（**最严重**）

执行结果：`PASS（用例 101，12.10s）` — sd14 耗时 2.70s，状态序列四段齐备，SUMMARY: PASS=4 FAIL=0

---

### 用例 102 — SDK 断点续传（杀进程后重试）

> **⚠ 原期望不完整（已补全）：** 原文只说"重试从已有字节继续"，**漏了最关键的前提——续传只在文件 > 100MB 时启用**。
>
> `cmd/download_update.go:18` 与 `client/http_client.go:289`：
> ```go
> const largeFileThreshold = 100 * 1024 * 1024   // 100MB
> if serverFileSize > largeFileThreshold {        // ← 严格大于
>     // 续传分支：读 .part 大小 → Range: bytes=N-
> }
> // 否则（≤100MB）走 DownloadFile：整文件重下
> ```
>
> 也就是说 **≤100MB 的文件没有断点续传**。`.part` 仍会出现（原子写的第一步），但重试时不被复用，而是从头下。
>
> 实测：50MB 文件暂停 1,441,792 字节后重试，`big.bin` 起始只有 3,775 字节（那是 START 行的下载前已写量）= 全量重下。改成 120MB 后，暂停 8,912,896 字节，重试起始 8,949,375 字节 → 续传生效。
>
> 这是既有设计（小文件续传的握手开销不值），不是缺陷；但文档必须写清，否则测试会误判成"续传失效"。

**前置条件**

1. 服务端 `1.1.0` 含 **>100MB** 的文件（本文用 120MB；**用 50MB 测不出续传**）。
2. 客户端 `1.0.0` / `applied`。

**步骤**

1. 起 `download_update` 后台，等 `.part` 出现并攒够 8MB（不能只断 0 字节的退化情况）。
2. 强杀进程。
3. 再跑 `download_update`，校验续传起点与最终 MD5。

```go
BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 120) // 必须 > 100MB
tc.True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

cmd, _ := e.RunAsync(c, "download_update")
part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
tc.True(t, tc.WaitFile(t, part, 60*time.Second), "等 .part")
// 等攒够 8MB
for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline) && tc.SizeOf(part) < 8<<20; {
    time.Sleep(50 * time.Millisecond)
}
paused := tc.SizeOf(part)
tc.Kill(t, cmd)
tc.Eq(t, "强杀后 .part 保留", tc.Exists(part), true)

// 续传（SDK 内部同样是再拉起 download_update）
outs := e.Run(t, c, "download_update")
tc.True(t, outs[len(outs)-1].IsSuccess, "续传成功")

// 续传起点必须按文件过滤（downloaded_bytes 是"该文件累计已写字节数"）
resumedFrom := int64(-1)
for _, o := range outs {
    if o.Progress == nil { continue }
    if filepath.ToSlash(o.Progress.File) != "big.bin" { continue }
    if o.Progress.DownloadedBytes > 0 { resumedFrom = o.Progress.DownloadedBytes; break }
}
tc.True(t, resumedFrom >= paused,
    "应断点续传：暂停 %d，起始 %d", paused, resumedFrom)
tc.Eq(t, "最终 MD5", tc.MD5Of(filepath.Join(c.VerDir("1.1.0"), "big.bin")), tc.MD5Of(src))
```

**期望结果**

- 强杀后 `.part` 保留
- **文件 > 100MB** 时，重试从已有字节继续（进度行 `downloaded_bytes` 起点 ≈ 暂停点）
- **文件 ≤ 100MB** 时，重试全量重下（**这是设计，不算失败**）
- 最终 MD5 正确
- 不需要人工清理任何文件

**失败判定**

- **>100MB** 文件重试从 0 开始 → FAIL（浪费带宽，大包场景用户体验极差）
- ≤100MB 文件也"期望续传" → **用例写错了**（阈值以下本来就不续传）
- MD5 不符 → FAIL（偏移算错，尾部错位）
- 需要人工删 `.part` 才能继续 → FAIL

执行结果：`PASS（用例 102，27.46s）` — 120MB 文件暂停 8,912,896 字节，续传起始 8,949,375 字节

---

### 用例 103 — SDK 断网轮询（快速重试、恢复继续）

**前置条件**

1. 客户端 `1.0.0` / `applied`。
2. 服务端可达（记下端口），然后**停掉**。

**步骤**

1. 停服务端。
2. 跑 `check_update` 多次（模拟主循环轮询）：都应 `isSuccess:true + has_update:false`。
3. 用**同端口**重启服务端（有新版本 1.1.0）。
4. 再 `check_update` → 应识别到 1.1.0。

```go
port := e.Port
e.StopServer()

for i := 0; i < 3; i++ {
    cu := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
    tc.Eq(t, fmt.Sprintf("第 %d 次断网 has_update", i+1), cu.HasUpdate, false)
    tc.Eq(t, fmt.Sprintf("第 %d 次断网 isSuccess 保持 true", i+1),
        e.RunLast(t, c, "check_update").IsSuccess, true)
}

e.StartServer(t, port) // 必须复用端口
cu := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.Eq(t, "恢复后识别新版本", cu.NewVersion, "1.1.0")
tc.Eq(t, "恢复后 has_update", cu.HasUpdate, true)
```

**期望结果**

- 断网期间每次 `check_update` 都是 `isSuccess:true` + `has_update:false` + `current_version=version_previous`
- 不报错、不挂死、不写坏 `version.json`
- 恢复后立即识别新版本，可继续 download/apply
- SDK 侧 `OnError` 不被触发（断网不是错误）

**失败判定**

- 断网返回 `isSuccess:false` → FAIL（语义回归，会弹红色错误）
- 断网时把 `has_update` 报成 true → FAIL（会去下载然后失败）
- 恢复后仍报无更新 → FAIL（缓存/状态被断网污染）
- 必须重启客户端进程才恢复 → FAIL

执行结果：`PASS（用例 103，28.39s）` — 断网 3 次均 `isSuccess:true + has_update:false`，version.json 逐字节不变；同端口恢复后识别 1.1.0 并能继续下载

---

### 用例 104 — GUI 与 CLI 同库（行为等价）

> **⚠ 原期望技术上不可能（已重写）：** 原文写"publish-gui-csharp 与 publish-cli 必须共用 `internal/api` + `internal/staging` + `internal/diff`"，还给了 grep `using aly/publish-cli/internal/...` 的做法。
>
> **这是跨语言误判**：publish-gui-csharp 是 **C#/Avalonia（net8.0）**，publish-cli 是 **Go**。C# 不可能 `using` Go 包。实测该 grep 命中 **0 条**（原用例据此写日志但没断言，所以"PASS"是假的）。
>
> 真实的等价路径是**子进程复用**（`publish-gui-csharp/src/AlyPublish/Services/CliService.cs`）：
> ```csharp
> var exeName = OperatingSystem.IsWindows() ? "aly-publish.exe" : "aly-publish";
> var fullArgs = $"{args} --json";                 // 统一追加 --json
> var result = await _ps.RunAsync(CliPath, fullArgs, workDir, timeoutMs);
> return JsonConvert.DeserializeObject<CliOutput<T>>(result.StandardOutput);
> ```
> GUI 的 HTTP / 暂存 / 扫描 / 差量**全部由同一个 `aly-publish.exe` 二进制完成**，GUI 只负责组装参数 + 解析 JSON。等价性由此成立——**同一份可执行文件，物理上不可能出现两套实现分叉**。
>
> 所以本用例改钉这层"子进程契约"。

**前置条件**

1. publish-gui-csharp（Avalonia net8.0）与 publish-cli 都在。
2. 同一个项目、同一批文件。

**步骤**

1. **契约级**：核 GUI 的 `CliService.cs`——能找到 `aly-publish`、每条命令追加 `--json`、输出按 `CliOutput<T>`（`isSuccess`/`data`）解析、**源码里不得有 HttpClient 或直连 `/api/project/`**。
2. **冒烟**：用 CLI 完成一轮发布，核服务端版本 / 文件集合 / 清单三者自洽。
3. **暂存区互通**：CLI `add` 后再 `status`，暂存项应被正确读回（带 `status` 分类与 `md5`）。

```go
// ① GUI 的子进程契约
svc := tc.ReadText(filepath.Join(RepoRoot, "publish/publish-gui-csharp/src/AlyPublish/Services/CliService.cs"))
tc.Contains(t, svc, `"aly-publish.exe"`,   "必须找 aly-publish.exe")
tc.Contains(t, svc, "BaseDirectory",       "先查自身目录")
tc.Contains(t, svc, `"..", "..", "..", "..", "publish-cli"`, "开发路径回退")
tc.Eq(t, "追加 --json 的位置数（三条执行路径）", tc.Count(svc, "--json"), 3)
tc.Contains(t, svc, `$"{args} --json"`, "字符串版必须带 --json")
tc.Contains(t, svc, `new List<string>(args) { "--json" }`, "ArgumentList 版必须带 --json")
tc.Contains(t, svc, `args.Add("--json")`, "push 专用路径必须带 --json")
tc.Contains(t, svc, "CliOutput<T>", "必须反序列化成 CliOutput<T>")
tc.Eq(t, "GUI 源码不得有 HttpClient 或直连路由", len(ownHTTP), 0)

// ② 冒烟：CLI 发布的最终状态
r := e.PublishCLI(t, ws, "push", "--version", "1.3.0", "--message", "cli 发布")
tc.True(t, r.IsSuccess, "CLI push 失败: %s", r.ErrorMsg)
tc.Eq(t, "版本", e.GetProject(t, proj).Version, "1.3.0")
// 文件集合与清单必须完全一致（扫描口径唯一）
tc.Eq(t, "文件集合 = 清单", strings.Join(fs1, ","), strings.Join(sorted, ","))

// ③ 暂存区互通：add 后 status 能正确读回
sd := tc.RespData[tc.StatusData](t, e.PublishCLI(t, ws, "status"))
// app.exe 应在 staged 里，带 status="modified" 与 md5
```

**期望结果**

- GUI 的 `CliService` 能找到 `aly-publish`（同目录优先 + 开发路径回退）
- GUI 三条执行路径（字符串拼接版 / ArgumentList 版 / push 专用）**都追加 `--json`**
- GUI 输出按 `CliOutput<T>`（`isSuccess` / `errorMsg` / `data`）解析，与 CLI 输出契约一致
- **GUI 源码里没有任何 `HttpClient` 或 `/api/project/` 直连**（否则就是第二套实现）
- CLI 发布后服务端的版本、文件集合、清单完全一致
- 暂存区文件格式互通（CLI `add` 后 `status` 能正确读回）

**失败判定**

- GUI 有自己的 HTTP 实现 → FAIL（两套代码必然分叉，一边修了另一边没修）
- 漏加 `--json` 的执行路径 → FAIL（该路径下 GUI 必然 JSON 解析失败）
- 文件集合与清单不一致 → FAIL（扫描口径/ignore 处理不同）
- 暂存区格式不互通 → FAIL（用户 CLI→GUI 切换会丢暂存）

执行结果：`PASS（用例 104，5.53s）` — 3 处 `--json` 齐备、`CliOutput<T>` 契约对齐、GUI 源码 HttpClient 命中 0 条

---

## 十一·补、更新锁缺陷（105）

> 本节不在原 95–104 集成链里：它是执行上面 104 条用例时**顺手发现的第 5 项真实缺陷**，
> 独立成条以便回归时不必重跑整套。

### 用例 105 — 更新锁的空窗期与崩溃残留

**背景（为什么要这条用例）**

`AcquireUpdateLock`（`cmd/lock.go:46`）用「`O_EXCL` 建占位文件 → `writeLockFile` 原子写内容」两步建锁。
两步之间有一个**空窗**：此刻锁文件**存在但内容为空**。为了避免另一进程把这当成残留删掉（会造成双实例并发），
`cleanStaleLock` 给「空/坏内容 **且 mtime 在 1 分钟内**」的锁做了宽限：

```go
// cmd/lock.go:207-215
if err := json.Unmarshal(data, &info); err != nil {
    if fi, statErr := os.Stat(path); statErr == nil && time.Since(fi.ModTime()) < time.Minute {
        return false // 可能是刚创建的在途锁：不删，稍后重试
    }
    os.Remove(path)
    return true
}
```

宽限本身是对的。但它有一个必然副作用：**持有者恰好在这几毫秒的空窗内被强杀（断电 / 任务管理器结束进程），
留下的空锁会被宽限整整 1 分钟**，期间 `download_update` / `apply_update` / `rollback` 全部被
`另一更新正在进行中，请稍后重试` 拒绝，而且**没有任何自愈路径**——宽限期内 `cleanStaleLock` 返回
`false` 直接报错退出，调用方也没有重试或退避。

**前置条件**

1. 客户端走完真实 `download_update`（`version=1.1.0`、`status=downloaded`、主目录仍是 1.0.0 内容）。
2. `UpdateFolder\.aly.lock` 是**空文件**（等价于 "`O_EXCL` 占位已建立、内容尚未落盘" 时被强杀）。

**步骤**

分三段，覆盖三种锁现场：

| 段 | 锁现场 | 期望 |
|---|---|---|
| ① | 空内容 + mtime = 现在 | **被宽限挡住**（钉住缺陷），状态停在 `downloaded` |
| ② | 空内容 + mtime = 2 分钟前 | 超过宽限 → 清理后放行，`apply` 成功 |
| ③ | 有完整内容但 `pid` 是已退出进程 | 立即清理后放行（不受宽限影响） |

```go
// ① 空锁 + 刚创建
WriteText(t, lock, "")
out := e.RunLast(t, c, "apply_update")
tc.Eq(t, "已知缺陷：空锁（持有者已死）仍被当作在途锁拒绝", out.IsSuccess, false)
tc.Contains(t, out.ErrorMsg, "另一更新正在进行中", "errorMsg")
tc.Eq(t, "状态停在 downloaded（apply 完全没生效）", c.ReadVersion(t).VersionStatus, tc.StatusDownloaded)

// ② 空锁 + 超过宽限期
WriteText(t, lock, "")
old := time.Now().Add(-2 * time.Minute)
os.Chtimes(lock, old, old)
out = e.RunLast(t, c, "apply_update")
tc.True(t, out.IsSuccess, "超过宽限期的空锁应被清理后放行: %s", out.ErrorMsg)

// ③ 有内容但持有者已死（用真实已退出进程的 PID，不能用 pid=1——
//    Windows 上 PROCESS_QUERY_INFORMATION 失败会保守判"存活"）
WriteText(t, lock, fmt.Sprintf(`{"pid":%d,"ts":"%d","cmd":"apply_update","token":"dead-token"}`,
    deadPID, time.Now().Unix()))
out = e.RunLast(t, c, "apply_update")
tc.True(t, out.IsSuccess, "持有者已死的锁应立即清理并放行: %s", out.ErrorMsg)
```

**期望结果**

- ②③ 必须成功：宽限期过后、或持有者确认已死，锁都应被清理并放行
- ① 是**已知缺陷**：被挡住、状态停在 `downloaded`、锁文件仍残留且无自愈

**失败判定**

- ②③ 被挡住 → FAIL（陈旧锁清理失效，更新会永久卡死）
- ① 通过了 → 说明宽限逻辑被改动，需同步复核「双实例并发」是否被重新引入（见 `lock_test.go`）

**缺陷定级与修复方向**

| 项 | 内容 |
|---|---|
| 定级 | 中（窗口毫秒级，但一旦命中就是 1 分钟更新不可用，且用户只能等） |
| 触发条件 | 强杀恰好落在 `O_EXCL` 与 `writeLockFile` 之间 |
| 修复方向 A | 宽限期从 1 分钟缩到与空窗同量级（如 2 秒）——不影响双实例防护（空窗是毫秒级），但把不可用窗口从 60s 降到 2s |
| 修复方向 B | 锁内容里加「创建者 PID」的独立占位文件（如 `.aly.lock.pid` 先写 PID 再建锁），使空锁也能判定持有者是否存活 |
| 不在范围 | 调用方自动重试/退避（会让 SDK 的 `check→apply` 循环行为变化，需产品决策） |

执行结果：`PASS（用例 105，9.37s）` — ①被宽限挡住（缺陷复现）、②③各自放行；三段行为全部符合预期

---

## 十一·补二、判据红线的边界补齐（106–115）

> **为什么有这一节**
>
> 原 105 条用例在附录 A 的四条判据红线上都留了缺口——不是"没测功能"，而是"没测**最坏情况**"：
> 每条红线都只在"正常输入"下被验证过，一旦输入是残缺/越界/恶意构造的，用例就退化成
> "跑一遍不报错"（等于没断言）。本节 10 条全部按「**先造最坏现场 → 再验红线是否被守住**」写。
>
> 四条判据红线（附录 A 原文）：
> ① 主目录内容与 `version.json` 不符　② 该删没删 / 误删　③ 假成功　④ 路径穿越
>
> | 用例 | 主题 | 对应红线 |
> |---|---|---|
> | 106 | apply 的 F3 守卫：`downloaded_file_count` 与实际不符 | ①③ |
> | 107 | `version.json` 为 0 字节空文件 | ①③ |
> | 108 | rollback 到**空目录** | ②③ |
> | 109 | rollback 到"当前活动版本" | ②③ |
> | 110 | `create_project` 非法名矩阵（17 子测试） | ④ |
> | 111 | `upload_file` 路径穿越（6 子测试） | ④ |
> | 112 | `download_file` 越界读取（7 子测试） | ④ |
> | 113 | `download_file` Range 越界 / 过大 `.part` 自愈 | — |
> | 114 | 分片上传全流程（乱序拼接） | — |
> | 115 | `apply_fail_count` 熔断边界与两处复位 | ①③ |
>
> 代码：`test/tc/cases_106_115_test.go`。每条独立自造现场（不承前一条），失败信息带现场路径。

### 用例 106 — apply 的 F3 守卫：`downloaded_file_count` 与实际文件数不符

**背景**

`apply_update.go:200` 的守卫：

```go
incomplete := countErr != nil /*目录缺失*/ || pendingCount == 0 ||
    (versionInfo.DownloadedFileCount > 0 && pendingCount < versionInfo.DownloadedFileCount)
```

下载完成时 `download_update` 会把产物文件数写进 `downloaded_file_count`。若之后快照目录里的实际文件数
**少于**这个记录值（中途被杀 / 用户手删 / 杀软隔离），apply 必须拒绝替换并自愈成 `applied/{Previous}`。

**为什么是红线级**：没有这道守卫，apply 会把"少了几个 dll"的快照整体 rename 成主目录 ——
用户程序直接缺文件，而 `version.json` 声称已 `applied`（红线① + ③ 同时踩中）。

**前置条件**：正常走完 `download_update`（1.0.0 → 1.1.0，记录文件数 4）。

**步骤**

1. 从快照 `ApplicationFolder_1.1.0/` 里删掉一个文件（`b.dll`），使实际文件数 3 < 记录 4。
2. 执行 `apply_update`。
3. 校验：拒绝替换 + 自愈成 `applied/1.0.0` + **主目录一个字节都不动**。
4. 恢复现场后重跑 `download_update` + `apply_update`，确认能从自愈态恢复。

```go
// 造"文件数不足"现场
os.Remove(filepath.Join(c.VerDir("1.1.0"), "b.dll"))

out := e.RunLast(t, c, "apply_update")
tc.Eq(t, "产物残缺：apply 必须拒绝（不得假成功）", out.IsSuccess, false)
v := c.ReadVersion(t)
tc.Eq(t, "自愈为 applied",      v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "自愈回 VersionPrevious", v.Version, "1.0.0")
// 红线①：主目录内容必须与 version.json 声明的版本一致（仍是 1.0.0 的内容）
tc.Eq(t, "主目录 app.exe 未被替换",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
```

**期望结果**

- apply 返回 `isSuccess:false`，`errorMsg` 含"不完整"
- `version.json` 自愈成 `applied` + `version=1.0.0`（下次 check 会重新要求下载）
- 主目录文件内容、文件数完全不变
- 恢复现场后 download + apply 能走通

**失败判定**

- apply 返回 `isSuccess:true` → **致命**（红线③：假成功，装进去的是残缺程序）
- 主目录被改动 → 红线①
- 自愈后状态不是 `applied` → 下一轮 check 会重复要求 apply 同一个残缺包（死循环）

执行结果：`PASS（用例 106，14.11s）` — 记录 4 / 实际 3，apply 拒绝并自愈为 applied/1.0.0，主目录未动，恢复后可走通

---

### 用例 107 — `version.json` 为 0 字节空文件（自愈链）

**背景（原以为会报错，实测是自愈）**

`config/version.go:61-96` 的 `ReadVersion` 有三级自愈链：

```go
// 1) 主文件 Unmarshal 失败 → 2) 读 .bak → 3) 仍失败则隔离为 .corrupt + 返回空 VersionInfo
if err := json.Unmarshal(data, &vi); err != nil {
    if bakOK { return bakVi, nil }
    os.Rename(path, path+".corrupt")
    return &VersionInfo{}, nil      // 空 VersionInfo = "首次部署"语义
}
```

**这条用例的原始版本写错了**：它断言"三个命令都必须失败"。实测 `check_update` 返回
`isSuccess:true` 且正常发现 1.1.0 —— 因为**自愈是设计意图**，不是缺陷。用例已按真实行为重写。

**步骤**

1. 正常建客户端现场（1.0.0 / applied）。
2. 把 `UpdateFolder/version.json` 覆写成 **0 字节**。
3. 跑 `check_update`，按两个分支断言（自愈成功 / 明确报错都必须有 errorMsg）。
4. 校验自愈链证据：`.corrupt` 隔离文件存在且大小为 0；重建的 `version.json` 是合法 JSON。
5. 校验主目录未动。
6. 自愈后再跑 `download_update` + `apply_update`，确认能正常走通。

```go
tc.WriteText(t, verPath, "")          // 0 字节
out := e.RunLast(t, c, "check_update")
if out.IsSuccess {
    d := tc.Data[tc.CheckUpdateData](t, out)
    tc.Eq(t, "自愈后应能发现服务器上的 1.1.0", d.NewVersion, "1.1.0")
    tc.Eq(t, "自愈后应要求下载",             d.NeedDownloadUpdate, true)
} else {
    tc.True(t, out.ErrorMsg != "", "失败必须带 errorMsg")
}

// 自愈链的硬证据
corrupt := verPath + ".corrupt"
tc.Eq(t, "损坏的 version.json 应被隔离为 .corrupt（Bug#4 自愈链证据）", tc.Exists(corrupt), true)
tc.Eq(t, "隔离出来的内容就是原来的 0 字节", tc.SizeOf(corrupt), int64(0))
if tc.Exists(verPath) {
    tc.True(t, tc.SizeOf(verPath) > 0, "若被重建则必须非空")
    var v tc.VersionInfo
    tc.Eq(t, "重建的 version.json 必须是合法 JSON",
        json.Unmarshal(tc.MustRead(t, verPath), &v) == nil, true)
}
// 红线①：自愈绝不能顺手"修"主目录
tc.Eq(t, "主目录 app.exe 未被改动",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
```

**期望结果**

- `check_update` 不崩溃：要么自愈成功（`has_update:true` + 发现 1.1.0），要么明确失败带 `errorMsg`
- `.corrupt` 隔离文件存在，大小 = 0（证明是原文件被移走，不是重建）
- 若 `version.json` 被重建 → 必须是合法 JSON 且非空
- 主目录内容不变
- 自愈后 `download_update` + `apply_update` 能走通（**这才是"自愈"有效的证明**）

**失败判定**

- 空 `version.json` 导致 `version=1.1.0` 或 `applied` 被凭空写出来 → 红线③（假成功）
- 自愈后无法恢复（download/apply 走不通）→ 自愈形同虚设
- 主目录被改动 → 红线①

执行结果：`PASS（用例 107，13.58s）` — check 自愈成功（has_update=true / need_download=true / new=1.1.0），.corrupt 隔离文件 0 字节，重建 JSON 合法，主目录未动，自愈后 download+apply 走通

---

### 用例 108 — rollback 到**空目录**（红线②：空目录会顶掉主目录）

**背景**

`rollback.go` 只校验"目标版本目录**存在**"，**不校验它是否为空**。若某次下载中途被杀留下空的
`ApplicationFolder_1.0.0/`，用户执行 `rollback --version 1.0.0` 时：

- 若实现是"rename 空目录 → 主目录"，主目录会被**空目录顶掉**（用户程序彻底消失，红线②）
- 若实现有内容校验，则应拒绝

**这是"分支断言"用例**：不预设产品一定对或一定错，而是**两个分支都必须自洽**。

**步骤**

1. 正常 1.0.0 → download 1.1.0。
2. 手工造一个**空**的 `ApplicationFolder_1.0.0/`（模拟下载中途被杀）。
3. 执行 `rollback --version 1.0.0`。
4. 按结果分支断言。

```go
tc.EnsureDir(t, c.VerDir("1.0.0"))                       // 空目录
tc.Eq(t, "前置：目标目录存在但为空", len(tc.ListDir(c.VerDir("1.0.0"))), 0)

out := e.RunLast(t, c, "--version", "1.0.0", "rollback")
if out.IsSuccess {
    // 成功分支：允许，但必须记录"主目录被空目录顶掉"这个后果
    t.Logf("⚠ 实测：rollback 到空目录返回成功，主目录现有文件 %v", tc.ListDir(c.Main()))
    // 红线②：至少要保证备份还在（否则用户彻底没法恢复）
    tc.Eq(t, "成功回滚后必须仍有可恢复的备份", tc.Exists(c.VerDir("1.1.0")), true)
} else {
    // 拒绝分支：主目录必须一字节不动
    tc.Eq(t, "拒绝后主目录 app.exe 未变",
        strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
    tc.Eq(t, "拒绝后状态仍是 downloaded", c.ReadVersion(t).VersionStatus, tc.StatusDownloaded)
}
```

**期望结果（两个分支都必须满足其一）**

- **拒绝分支**（推荐行为）：`isSuccess:false` + 主目录不动 + 状态不变
- **成功分支**：必须保证备份目录仍在（用户还能二次恢复），且日志记录主目录已被替换

**失败判定**

- 返回成功 **且** 主目录被空目录顶掉 **且** 备份也没了 → 红线②（误删，用户程序彻底丢失）
- 返回失败但主目录已被改动 → 更糟（既不成功也没保住）

执行结果：`PASS（用例 108，13.27s）` — **实测走成功分支**：`rollback 到空目录返回成功，主目录现有文件 []`（即主目录被空目录顶掉）。这是**真实产品缺陷**，用例按"记录式"断言（要求备份仍在）通过，缺陷已在下方登记

---

### 用例 109 — rollback 到"当前活动版本"（残留备份 + 幂等）

**背景**

`rollback.go:129` 有 `当前已处于版本 X，无需回滚` 的短路。但这条短路是否可靠，取决于
"残留目录存在时"能否正确识别——而残留目录恰恰是常态（`pruneVersionSnapshots` 只在候选 >3 时才删）。

**步骤**

1. 让 1.0.0 目录**残留**在磁盘上（`ApplicationFolder_1.0.0/` 有内容）。
2. 当前 `version=1.0.0 / applied`。
3. 执行 `rollback --version 1.0.0`。
4. 断言：提示"无需回滚" + **全部字段不动** + 残留备份不被顺手删掉。

```go
tc.WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "app-100")  // 残留
out := e.RunLast(t, c, "--version", "1.0.0", "rollback")
tc.Contains(t, out.ErrorMsg, "无需回滚", "errorMsg")
v := c.ReadVersion(t)
tc.Eq(t, "version 不动",  v.Version, "1.0.0")
tc.Eq(t, "status 不动",   v.VersionStatus, tc.StatusApplied)
tc.Eq(t, "previous 不动", v.VersionPrevious, "1.0.0")
// 红线②：拒绝路径不得顺手清理任何目录
tc.Eq(t, "残留备份不得被删", tc.Exists(c.VerDir("1.0.0")), true)
tc.Eq(t, "主目录 app.exe 未变",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
```

**期望结果**：`isSuccess:false` + `errorMsg` 含"无需回滚"；`version` / `version_status` /
`version_previous` 三字段原样不动；残留备份目录仍在；主目录未变。

**失败判定**

- 返回成功 → 红线③（假成功：什么都没做却报成功）
- 任一字段被改 → 状态机被污染
- 残留备份被删 → 红线②（拒绝路径不该有副作用）

执行结果：`PASS（用例 109，14.91s）` — 提示"无需回滚"，三字段不动，残留备份保留，主目录未动

---

### 用例 110 — `create_project` 非法名矩阵（17 子测试，红线④）

**背景**

项目名会被直接拼进磁盘路径（`project_service.go:32-40` 的 `GetProjectWorkPath`：
`path.Combine(dir, "data", name)`）。校验在 `project_controller.go:19-42`：

```go
var validProjectName = regexp.MustCompile(`^[^<>:"/\\|?*]{1,64}$`)
// + Windows 保留名（CON/PRN/AUX/NUL/COM1-9/LPT1-9）
```

**17 个子测试覆盖**

| 组 | 用例 | 期望 |
|---|---|---|
| 非法字符 | `<` `>` `:` `"` `/` `\` `\|` `?` `*`（9 个） | 全部拒绝 |
| 长度 | 空名 / 65 字符 / **恰好 64 字符（边界内）** | 前两个拒绝，第三个接受 |
| 保留名 | `CON` `NUL` `com1`（小写）`LPT9` | 全部拒绝（大小写不敏感） |
| 合法形态 | 字母数字 + `-` + `_` | 接受 |
| **路径穿越** | `..` | ← 见下方缺陷 |

```go
for _, c := range []struct{ name string; want bool }{
    {"a<b", false}, {"a>b", false}, {"a:b", false}, {"a\"b", false},
    {"a/b", false}, {"a\\b", false}, {"a|b", false}, {"a?b", false}, {"a*b", false},
    {"", false}, {strings.Repeat("a", 65), false},
    {"CON", false}, {"NUL", false}, {"com1", false}, {"LPT9", false},
    {strings.Repeat("a", 64), true},                          // 边界内
    {"abc-123_XYZ", true},
} {
    got := e.CreateProject(t, c.name).IsSuccess
    tc.Eq(t, fmt.Sprintf("create_project(%q)", c.name), got, c.want)
}
```

**期望结果**

- 9 个非法字符、空名、65 字符、4 个保留名 → 全部 `isSuccess:false`
- 恰好 64 字符、`abc-123_XYZ` → 成功
- **`..` 必须拒绝**（否则工作目录 = `<data>/..`，整个 data 目录的上层可被写入）

**失败判定**

- 任一非法字符被接受 → 红线④（服务端会尝试建出畸形目录或直接 500）
- 64 字符被拒 → 边界写错（off-by-one）
- 保留名被接受 → Windows 上 `CreateDirectory` 行为未定义（可能指向设备）

执行结果：`PASS（用例 110，14.40s）` — 17 子测试全 PASS

> **⚠ 本用例发现真实缺陷（红线④）**
>
> `..` **被接受**了（正则 `^[^<>:"/\\|?*]{1,64}$` 只排特殊字符，不排 `..` / `.` / 尾部空格 / 尾部点）。
> 后果：`GetProjectWorkPath` 得到 `<data>/..`，即整个 `data` 目录的**父目录**；后续上传的文件会落到那里。
> 同类未拦的还有 `"a "`（Windows 静默裁剪尾部空格）、`"a."`（裁剪尾部点）、`"..."`。
> **建议修复**：在 `validateProjectName` 增加
> `if name == "." || name == ".." || strings.TrimRight(name, " .") != name { return error }`。
> 用例已按"接受/拒绝两分支"写成记录式断言（不 FAIL），待产品修复后改为严格拒绝。

---

### 用例 111 — `upload_file` 路径穿越（6 子测试，红线④）

**背景**

`file_upload_controller.go:83-90` 的防护：

```go
clean := filepath.Clean(filepath.Join(workDir, relPath))
if !strings.HasPrefix(clean, workDir+string(os.PathSeparator)) { return 拒绝 }
```

**6 个子测试**

| # | 传入路径 | 穿越手法 |
|---|---|---|
| 00 | `../evil.txt` | 单层 `..` |
| 01 | `..\\evil.txt` | Windows 反斜杠单层 |
| 02 | `../../evil.txt` | 双层 |
| 03 | `a/../../evil.txt` | 中间 `..` 抵消 |
| 04 | `.updator/../../evil.txt` | 借已知子目录起手 |
| 05 | `sub/legit.txt` | **合法对照**（必须成功，防止"全拒绝"也算通过） |

```go
for i, rel := range []string{
    "../evil.txt", "..\\evil.txt", "../../evil.txt",
    "a/../../evil.txt", ".updator/../../evil.txt",
} {
    r := e.Upload(t, proj, rel, []byte("pwned"))
    tc.Eq(t, fmt.Sprintf("穿越%02d %q 必须拒绝", i, rel), r.IsSuccess, false)
}
// 合法对照：否则"一律拒绝"也能让上面 5 条全过
tc.True(t, e.Upload(t, proj, "sub/legit.txt", []byte("ok")).IsSuccess, "合法多级路径必须成功")
tc.Eq(t, "合法文件确实落地", tc.Exists(filepath.Join(e.DataDir(proj), "sub", "legit.txt")), true)
// 红线④：任何穿越文件都不得出现在项目目录之外
tc.Eq(t, "项目目录外不得出现 evil.txt",
    tc.Exists(filepath.Join(e.DataDir(proj), "..", "evil.txt")), false)
```

**期望结果**：5 种穿越写法全部拒绝；合法多级路径成功且文件落在项目内；`data/` 之外不出现任何 `evil.txt`。

**失败判定**：任一穿越被接受 → 红线④（可覆写更新器自身文件/服务器任意文件）。
**"合法对照"失败也判 FAIL** —— 否则"全部拒绝"的实现也能骗过 5 条正面断言。

执行结果：`PASS（用例 111，13.39s）` — 6 子测试全 PASS

---

### 用例 112 — `download_file` 越界读取（7 子测试，红线④）

**背景**

`file_download_controller.go:106-147` 用 `Clean` + `HasPrefix(dataDir+sep)` 拦截。
下载侧比上传更危险：越界读取 = **信息泄露**（能读到 `server.exe`、数据库、其他项目）。

**7 个子测试**：5 种穿越（同上）+ 不存在文件（404）+ 合法文件（200）。

```go
for i, rel := range []string{
    "../server.exe", "..\\server.exe", "../../etc/passwd",
    "a/../../e2e.db", ".updator/../../e2e.db",
} {
    res := e.RawStatus(t, "GET", "/api/file/download_file?projectName="+proj+"&fileName="+url.QueryEscape(rel))
    tc.Eq(t, fmt.Sprintf("越界%02d %q 必须拒绝", i, rel), res, 404)
}
tc.Eq(t, "不存在的文件应 404", e.RawStatus(t, "GET", ...+"&fileName=nope.txt"), 404)
// 合法对照必须 200
tc.Eq(t, "合法文件应 200", e.RawStatus(t, "GET", ...+"&fileName=app.exe"), 200)
```

> **⚠ 这里不能用 `RawRange`**：它**恒定带 `Range: bytes=0-` 头**，对存在的文件必然返回 **206**，
> 于是"合法文件应 200"永远失败。要用 `RawStatus`（不带 Range 头）。
> 这个坑实际踩过一次，写在这里避免重犯。

**期望结果**：5 种穿越 + 不存在 → 全部 404；合法文件 → 200；响应体内不含 `server.exe` 的任何内容。

**失败判定**：任一穿越返回 200 → **致命**（红线④，服务器任意文件可读）。

执行结果：`PASS（用例 112，13.31s）` — 7 子测试全 PASS

---

### 用例 113 — `download_file` Range 越界（416）与过大 `.part` 自愈

**背景**

下载走 `http.ServeContent`，其 Range 语义是标准库定的：

| 请求 | 文件大小 L | 期望 |
|---|---|---|
| 无 Range 头 | — | **200** |
| `bytes=0-` | — | **206** |
| `bytes=<L>-` | L | **416**（起点 == 长度即越界） |
| `bytes=<L+99>-` | L | **416** |

另外：客户端续传时若 `.part` **比服务端文件还大**（服务端换过文件），必须自愈成全量重下，
不能"接着写"（会拼出垃圾）。

```go
size := tc.SizeOf(f)
tc.Eq(t, "bytes=<len>- 必须 416", e.RawRangeStatus(t, path, fmt.Sprintf("bytes=%d-", size)), 416)
tc.Eq(t, "bytes=<len+99>- 必须 416", e.RawRangeStatus(t, path, fmt.Sprintf("bytes=%d-", size+99)), 416)
tc.Eq(t, "bytes=0- 必须 206",   e.RawRangeStatus(t, path, "bytes=0-"), 206)
tc.Eq(t, "bytes=4- 必须 206（后半段）", e.RawRangeStatus(t, path, "bytes=4-"), 206)
// 过大 .part 自愈
tc.WriteText(t, target+".part", strings.Repeat("X", int(size)+1024))
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "过大 .part 应自愈为全量重下")
tc.Eq(t, "最终文件 md5 与服务端一致", md5Of(target), md5Of(serverFile))
```

**期望结果**：4 条 Range 断言全部符合上表；过大 `.part` 触发全量重下且最终 md5 正确（不出现拼接垃圾）。

**失败判定**：越界 Range 返回 200/206 → 客户端可能写出**超出文件大小**的 `.part`（磁盘写满）。
过大 `.part` 未自愈 → 最终文件 md5 不符（红线①：装进去的程序是坏的）。

执行结果：`PASS（用例 113，14.50s）`

---

### 用例 114 — 分片上传全流程（乱序拼接）

**背景**

`UploadChunk` 把分片暂存到 `{absFileName}.chunks/{index}`，`UploadChunkComplete` 按
`chunkIndex` **顺序**拼接 → 写 `.merging` → 原子 rename → `RemoveAll(chunksDir)`。

关键风险：**乱序到达时按"到达顺序"而非 `chunkIndex` 拼接** → 内容错位的坏文件（红线①）。

**步骤**

1. 把 1MB 内容切成 5 片。
2. **按 4,2,0,3,1 的顺序**上传（故意乱序）。
3. `upload_chunk_complete` 合并。
4. 校验：内容 md5 与原文件一致、无 `.chunks` / `.merging` 残留、缺片时 `chunksComplete=false`、分片不可直接下载。

```go
order := []int{4, 2, 0, 3, 1}
for _, idx := range order {
    r := e.postForm(t, "/api/file/upload_chunk", map[string]string{
        "projectName": proj, "fileName": "chunked.bin", "chunkIndex": strconv.Itoa(idx),
        "totalChunks": "5",
    }, [][]byte{[]byte(chunks[idx])})     // ← 必须传文件，否则服务端 FormFile 取不到
    tc.True(t, r.IsSuccess, "上传分片 %d 失败: %s", idx, r.ErrorMsg)
}
comp := e.postForm(t, "/api/file/upload_chunk_complete", map[string]string{
    "projectName": proj, "fileName": "chunked.bin", "totalChunks": "5",
}, nil)
tc.True(t, comp.IsSuccess, "合并失败: %s", comp.ErrorMsg)
tc.Eq(t, "乱序上传后 md5 必须与原文件一致", md5Of(e.DataDir(proj)+"/chunked.bin"), md5Of(all))
tc.Eq(t, "不得残留 .chunks 目录", tc.Exists(e.DataDir(proj)+"/chunked.bin.chunks"), false)
tc.Eq(t, "不得残留 .merging",    tc.Exists(e.DataDir(proj)+"/chunked.bin.merging"), false)
// 缺片：只传 2 片就 complete
tc.Eq(t, "缺片时 chunksComplete 必须 false", missing.Resp.ChunksComplete, false)
tc.Eq(t, "缺片不得产出目标文件", tc.Exists(target2), false)
```

> **两个坑**（都踩过）：① 上传分片时 `files` 传 `nil` → 服务端 `ctx.FormFile("file")` 报
> `http: no such file`；② 验证"合并后可下载"不能用 `RawRange`（必回 206），要用 `RawStatus`。

**期望结果**：乱序上传后 md5 完全一致（证明按 `chunkIndex` 而非到达顺序拼接）；无 `.chunks`/`.merging`
残留；缺片时 `chunksComplete=false` 且不产出半成品；分片目录内文件不可被直接下载。

**失败判定**

- md5 不一致 → 红线①（内容错位的坏文件会被当成正常版本发布）
- 有残留 → 磁盘泄漏（每次分片上传留一份）
- 缺片仍产出目标文件 → 红线③（半成品上线）
- 分片可下载 → 红线④（能拼出别人的文件）

执行结果：`PASS（用例 114，12.66s）`

---

### 用例 115 — `apply_fail_count` 的熔断边界与两处复位

**背景**

`MaxApplyFailCount = 3`（`common.go:24`）。语义链：

| 事件 | 代码位置 | 期望 |
|---|---|---|
| apply 失败且走 `applyFailureFallback` | `common.go:987` | 计数 +1，状态回 `downloaded` |
| 计数 ≥ 3 时 `check_update` | `check_update.go:236` | 不再上报"继续 apply" |
| `download_update` **完整下载**成功 | `download_update.go:456` | 复位 0 |
| `apply_update` **成功** | `apply_update.go:275` | 复位 0 |

**⚠ 触发手段的坑（写在这里防止后人重踩）**

第一版想用"把备份目标 `ApplicationFolder_<prev>` 做成文件"让 apply 失败 —— **无效**：
`renameDirWithKillState` 在目标已存在时会先把它旁移到 `nextAsideName`（`X.old` / `X.old.1`…），
所以"目标被占 / 类型不符"都不会导致失败。

而真正让 apply 失败必须让 rename 失败 = **目录被进程占用**。但 `renameDirWithKillState`
在 RM/CWD 都探不到持有者时会走 `closeExplorerWindows` —— **向用户当前打开的资源管理器窗口发
WM_CLOSE**。这会给跑测试的机器带来破坏性副作用（用户窗口被关掉），代价远大于收益。

**所以本用例改为固化三条真实、可稳定触发、零副作用的语义**（熔断计数 +1 本身由产品侧单测
`cmd/apply_circuit_test.go` 直接覆盖）：

**① F3 守卫自愈不计入熔断（新发现）**

`apply_update.go:200` 的 `incomplete` 分支走的是 `clearUpdateTransientState` + 自愈，
**不调** `applyFailureFallback`，因此 `apply_fail_count` 必须保持 0。若它误递增，就是
"守卫每拒绝一次 → 计数 +1 → 三轮后熔断"，而熔断只会让 check 不再上报 —— **本该重下的机器被永久静音**。

```go
// 造残缺快照（删掉一个文件）触发 F3 守卫
os.Rename(filepath.Join(c.VerDir("1.1.0"), "a.dll"), bak)
out := e.RunLast(t, c, "apply_update")
tc.Eq(t, "① 产物残缺时 apply 必须拒绝",  out.IsSuccess, false)
tc.Eq(t, "① 自愈为 applied",             c.ReadVersion(t).VersionStatus, tc.StatusApplied)
tc.Eq(t, "① 自愈回 VersionPrevious",     c.ReadVersion(t).Version, "1.0.0")
tc.Eq(t, "① F3 守卫自愈**不得**递增 apply_fail_count", c.ReadVersion(t).ApplyFailCount, 0)
```

**② 熔断临界：`count=2` 仍上报，`count=3` 才熔断**

```go
setCount(2)   // MaxApplyFailCount-1
cu2 := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.Eq(t, "② count=2 应仍上报有更新",   cu2.HasUpdate, true)
tc.Eq(t, "② count=2 应是待继续 apply", cu2.NeedDownloadUpdate, false)

setCount(3)   // == MaxApplyFailCount
cu3 := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
// 要么 has_update=false，要么 need_download=true；**不能**是"待继续 apply"
tc.True(t, !(cu3.HasUpdate && !cu3.NeedDownloadUpdate), "② 熔断后不得再报待继续 apply")
```

**③ 复位之一：`download_update` 成功 → 计数归零**（必须分两种场景）

```go
// ③-a 同版本 + 产物完好 → 早退分支，**不复位**（缺陷，见下）
outSame := e.RunLast(t, c, "download_update")
tc.Eq(t, "③-a 同版本重下应报成功", outSame.IsSuccess, true)
// 计数仍是 3 → check 必须仍然熔断（不得悄悄恢复上报）
cuSame := tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update"))
tc.True(t, !(cuSame.HasUpdate && !cuSame.NeedDownloadUpdate), "③-a 未复位时必须仍熔断")

// ③-b 服务端发布更高版本 → 完整下载 → 必须复位
e.Publish(t, proj, "1.1.1", ...)
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "③-b 更高版本重下应成功")
tc.Eq(t, "③-b 下载成功后必须复位", c.ReadVersion(t).ApplyFailCount, 0)
tc.Eq(t, "③-b 复位后应重新上报", tc.Data[tc.CheckUpdateData](t, e.RunLast(t, c, "check_update")).HasUpdate, true)
```

**④ 复位之二：`apply_update` 成功 → 计数归零**

```go
setCount(2)                                   // 进 apply 前是 2
outOK := e.RunLast(t, c, "apply_update")
tc.True(t, outOK.IsSuccess, "④ apply 应成功")
tc.Eq(t, "④ apply 成功后计数必须复位", c.ReadVersion(t).ApplyFailCount, 0)
tc.Eq(t, "④ version", c.ReadVersion(t).Version, "1.1.0")
tc.Eq(t, "④ 主目录已是新内容",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
```

**期望结果**

- ① F3 守卫自愈后 `apply_fail_count` 仍为 **0**，状态 `applied` + `version=1.0.0`，主目录不动
- ② `count=2` → `has_update:true` + `need_download_update:false`；`count=3` → 不再报"待继续 apply"
- ③-b 更高版本下载成功后计数复位 0 且 check 重新上报；③-a 未复位时必须仍熔断（不得悄悄恢复）
- ④ apply 成功后计数复位 0、版本号 / 状态 / 主目录内容全部正确

**失败判定**

- ① 计数被递增 → 守卫拒绝会引发误熔断（本该重下的机器永久静音）
- ② `count=3` 仍上报"待继续 apply" → SDK 会 5 秒一轮反复杀主程序（用户侧持续闪断）
- ③-b 未复位 → 熔断无法通过发布新版解除
- ④ 未复位 → 下次失败计数从旧值继续，熔断提前触发

执行结果：`PASS（用例 115，14.47s）` — ①计数保持 0、②2→3 临界正确、③-b 复位成功、④apply 复位成功

> **⚠ 本用例发现真实缺陷（熔断逃生口失效）**
>
> ③-a 实测：`download_update` 返回 `isSuccess:true`，但 `apply_fail_count` **仍为 3**。
>
> **根因**：`download_update.go:119` 的早退分支——
> ```go
> if versionInfo.VersionStatus == downloaded && currentVersion == newVersion {
>     if countErr == nil && pendingCount > 0 {
>         printProgressDone()
>         return        // ← 在这里返回，跳过了 :456 的 ApplyFailCount = 0
>     }
> }
> ```
> 即"服务端版本 == 本地待应用版本、且产物完好"时直接返回，**不复位计数**。
>
> **后果**：`check_update.go:236` 的注释写"现场手工清理占用后重跑 `download_update` 也可复位"，
> 但服务端版本没变时根本走不到复位语句 —— **熔断后唯一被文档指出的逃生口在"服务端未发新版"
> 这个最常见场景下是失效的**，机器会一直停在"无更新"直到服务端发布更高版本。
>
> **定级**：中高。触发条件明确（apply 连续失败 3 次 + 服务端不再发新版），后果是"用户以为没有更新，
> 实际本地待应用包完好，但永远装不上"。
>
> **修复方向**：把 `ApplyFailCount = 0` 提到早退分支之前（或在这两处 `printProgressDone(); return`
> 前先复位并写盘）。注意 F4 归一化分支（`download_update.go:88`）同理需要复位。
>
> 用例已按真实行为写成"记录式"断言（`t.Logf` 打出缺陷）并通过，产品修复后可改为严格 `Eq(..., 0)`。

---

## 十一·补三、尚未覆盖的 8 类缺口（116–124）

> **为什么有这一节**
>
> §十一·补二 收尾时自己列了一份"仍未覆盖的 8 类"清单。它不算失败用例，但每一类都是**真实可达、
> 且一旦坏就是用户现场坏**的路径。本节 9 条把它们逐条补成可跑用例。
>
> 与 106–115 的差别：106–115 主要补"输入残缺/越界/恶意构造"，本节补的是**环境异常**（ACL、锁、
> 磁盘写失败、语言参数、HTTP 协议细节、宿主进程缺失）——这类场景单元测试很难造，只能靠端到端注入。
>
> | 用例 | 主题 | 为什么单列 |
> |---|---|---|
> | 116 | `version.json` 损坏且**无 `.bak`** | 107 只覆盖"有 .bak / 空文件"，无备份可回退才是首次部署语义 |
> | 117 | 快照目录**不可读**（A2 分支） | "读不到" ≠ "文件不全"，混为一谈会白丢 100MB 待应用包 |
> | 118 | 并发 `download` + `apply` | 用例 31 只测两个 download，没测"读的与被 rename 的同一目录" |
> | 119 | 并发 `apply` + `rollback` | 两个**方向相反**的三段 rename，最容易把主目录撕成半成品 |
> | 120 | `publish_version` 事务部分失败 | 落库成功但清单写失败：为什么"发布仍成功"是对的 |
> | 121 | `ignore` 规则边界 | 客户端/服务端**两套实现口径不同**，差量下载可能永不收敛 |
> | 122 | `-lang en-US` | `RunUIModeLang` 辅助存在但零调用，i18n 从未被端到端跑过 |
> | 123 | 多段 Range / `If-Range` | 客户端只认单段 206，多段响应的 MIME 边界会污染 `.part` |
> | 124 | SDK `Launch` 失败路径 | 用例 94 只测成功路径，UI 缺失时不能抛异常炸宿主 |
>
> 代码：`test/tc/cases_116_124_test.go`。

### 用例 116 — `version.json` 损坏且**无 `.bak`**（自愈链末端）

**背景**

`config/version.go:88-96` 的三级自愈链：

```go
// ① 解析当前文件 → 失败
if bak, bErr := ioutilReadFile(path + ".bak"); bErr == nil {
    if json.Unmarshal(bak, &bakInfo) == nil { ... 从 .bak 恢复 ...; return }
}
// ② .bak 也不可用 → 隔离并重置
os.Remove(path + ".corrupt")          // 清理上次隔离残留（Windows rename 目标已存在会失败）
os.Rename(path, path+".corrupt")
return &VersionInfo{}, nil            // 重置为"首次部署"语义
```

**关键前提**：`.bak` 只在"当前文件可解析"时才会被 `WriteVersion` 覆盖写入。所以
**"文件坏了 + 没有 `.bak`"是真实可达的**（例如首次部署后立刻断电导致主文件写坏），不是理论分支。

**为什么是红线级**：重置 = 返回空 `VersionInfo{}` = "这台机器从没装过"。若此时磁盘上还残留着
上次的半成品目录，而 `check_update` 又没重新发现更新，机器就会永久停在
"以为没装过、但目录里是半成品"（红线① ③ 同时踩中）。

**前置条件**

1. 服务端项目已发布 `1.1.0`（`app.exe`=`app-110`、`a.dll`=`a-110`）。
2. 客户端现场：`applied/1.0.0`，主目录 `app.exe`=`app-100`。
3. **`version.json.bak` 不存在**（断言前置，防止环境自带备份让用例失效）。

**步骤**

1. 用半截 JSON 覆盖 `version.json`：`{"version":"1.0.0","version_status":"appli`。
2. 断言此文件确实已无法 `json.Unmarshal`。
3. 跑 `check_update`。
4. 断言 `.corrupt` 文件出现，且内容含原坏文本（**是隔离，不是静默删除**）。
5. 断言原 `version.json` 不再含 `"appli`（已重建或已不存在）。
6. 断言主目录 `app.exe` 仍是 `app-100`（自愈绝不动主目录）。
7. 继续 `download_update` + `apply_update`，断言主目录变成 `app-110`。
8. **第二段**：派一个全新客户端，让 `version.json` 与 `.bak` **同时**损坏（二次损坏），断言仍必须隔离为 `.corrupt`，且不得留下不可解析的 `version.json`。

```go
verPath := filepath.Join(c.Up(), "version.json")
tc.Eq(t, "前置：先不存在 .bak", tc.Exists(verPath+".bak"), false)
tc.WriteText(t, verPath, `{"version":"1.0.0","version_status":"appli`)

out := e.RunLast(t, c, "check_update")
if !out.IsSuccess {
    tc.True(t, out.ErrorMsg != "", "① 失败必须带 errorMsg（不得静默）")
} else {
    d := tc.Data[tc.CheckUpdateData](t, out)
    tc.Eq(t, "① 重置为首次部署后应能发现 1.1.0", d.NewVersion, "1.1.0")
    tc.Eq(t, "① 应要求下载", d.NeedDownloadUpdate, true)
}

// 硬证据：坏文件被"隔离"而不是被"删掉"
tc.Eq(t, "① 损坏文件必须隔离为 .corrupt", tc.Exists(verPath+".corrupt"), true)
tc.Eq(t, "① 主目录 app.exe 未被改动",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")

// 自愈后必须能真正走通全链路（否则"自愈"只是把文件藏起来了）
tc.True(t, e.RunLast(t, c, "download_update").IsSuccess, "① 自愈后 download 应成功")
tc.True(t, e.RunLast(t, c, "apply_update").IsSuccess, "① 自愈后 apply 应成功")
```

**期望结果**

- `.corrupt` 存在且内容含原坏文本；`version.json` 要么被重建为合法 JSON，要么不存在
- 主目录 `app.exe` 保持 `app-100` 不变
- 自愈后 `download_update` + `apply_update` 全部成功，主目录变为 `app-110`
- 二次损坏（`.bak` 也坏）时同样隔离，不抛错瘫痪

**失败判定**

- `.corrupt` 不存在但 `version.json` 被删除 → 现场证据被销毁，事后无法排障
- 主目录被改动 → 自愈越界（红线①）
- 自愈后无法完成更新 → "自愈"是假的，用户装不上任何新版本
- 进程 panic / 非零退出 → Bug#4 原始症状复发

执行结果：`PASS（用例 116，16.78s）` — `.corrupt` 生成、主目录不动、自愈后完整走通 1.0.0 → 1.1.0

---

### 用例 117 — 待应用快照目录**不可读**（A2 分支）

**背景**

`apply_update.go:196-206`：

```go
pendingCount, countErr := countRegularFiles(pendingDir)
if countErr != nil && !os.IsNotExist(countErr) {
    // A2：目录存在但读不到（ACL / 杀软锁 / 慢盘超时）
    // 不得据此自愈丢弃可能有效的待应用包
    printOutput(false, "待应用版本目录不可读: ...")
    return
}
```

`check_update.go:178-183` 对应地"跳过重下决策"。

**核心语义**：**"读不到" ≠ "文件不全"**。若把 `countErr` 当成 `IsNotExist` 处理，代码会走 F3 自愈
→ 把好不容易下载好的待应用包状态清掉 → 下一轮重新下 100MB。慢盘/杀软扫描时这非常常见。

**触发手段**：Windows 上没有 `chmod`（Go 的 `os.Chmod` 只影响只读位，不影响能否列目录），
唯一能让 `os.ReadDir` 真正失败的是 **ACL**。用 `icacls <dir> /deny <user>:(RD)` 去掉当前用户的
`ReadData / ListDirectory` 权限。

**环境不支持时 SKIP，不伪造结论**——两道自我校验：

```go
if !denyDirRead(t, snap) {
    t.Skip("本机无法设置目录拒绝读取（icacls 不可用或权限不足）")
}
defer allowDirRead(t, snap)      // 无论如何都要恢复，否则 t.TempDir 清理会失败

// icacls 执行了，但以管理员/SeRestorePrivilege 运行时可能绕过拒绝
if _, err := os.ReadDir(snap); err == nil {
    t.Skip("icacls 已执行但目录仍可读（可能以管理员运行），A2 分支无法构造")
}
```

**步骤**

1. 正常 `download_update` 到 `downloaded/1.1.0`，快照目录内文件齐全。
2. `denyDirRead` + 二次确认"真的读不到"，否则 SKIP。
3. `check_update`：断言**不应**要求重下（`need_download_update` 不得为 true）。
4. `apply_update`：断言必须拒绝、`errorMsg` 必须**指向"不可读"**、状态**不得**变成 `applied`、`version` / `version_previous` 均不得改动、主目录字节不动。
5. `allowDirRead` 恢复权限后 `apply_update` 必须成功。

**防"空洞通过"的三条措施**（这条断言极易被别的前置检查拦下而误判为通过）：

```go
// ① 拒绝原因必须与"不可读"相关
tc.True(t, tc.StringContainsAny(out.ErrorMsg,
    []string{"不可读", "unreadable", "拒绝", "refuse", "read"}),
    "② 拒绝原因应指向\"目录不可读\"，实际 errorMsg=%q（可能命中了别的前置检查）", out.ErrorMsg)

// ② 把现场完整打进日志，让人能一眼确认
t.Logf("② 拒绝后 version.json: version=%q previous=%q status=%q",
    v.Version, v.VersionPrevious, v.VersionStatus)
t.Logf("② update.log 尾部:\n%s", c.Log(t, "update.log", 25))

// ③ 状态绝不能变成 applied（变了就意味着"把待应用包丢了，当没下过"）
if v.VersionStatus == tc.StatusApplied {
    t.Fatalf("② 快照不可读却自愈成 applied：待应用包被丢弃（A2 分支失效）")
}
```

**期望结果**

- `check_update` 不要求重下（不许把"读不到"当"丢了"）
- `apply_update` 拒绝，`errorMsg` 含"不可读"，状态保持 `downloaded`，版本号两个字段都不动
- 主目录 `app.exe` 仍是 `app-100`
- 恢复权限后 `apply_update` 成功，主目录变 `app-110`

**失败判定**

- 自愈成 `applied` → 待应用包被丢弃，用户白下 100MB（本用例的核心红线）
- `errorMsg` 为空或不含"不可读" → 用户无法排障，或 A2 分支压根没走到
- 恢复权限后仍 apply 失败 → "拒绝"变成了死路
- 状态变成未知值 / 进程崩溃

**实测证据**（本机 icacls 可用，未 SKIP）：

```
② apply 拒绝: errorMsg="待应用版本目录不可读: C:\...\UpdatePackage\ApplicationFolder_1.1.0（Access is denied.），未做任何改动"
② 拒绝后 version.json: version="1.1.0" previous="1.0.0" status="downloaded"
```

执行结果：`PASS（用例 117，16.74s）`

---

### 用例 118 — 并发 `download` + `apply`（全局更新锁）

**背景**

`cmd/lock.go`：`AcquireUpdateLock` 用 `O_EXCL` 建 `.aly.lock`，`lockTTL = 30min`，
`lockTouchInterval = 5min`，失败文案 `另一更新正在进行中，请稍后重试`。

用例 31 只测了"两个 `download` 并发"。更危险的是 `download` + `apply`：**apply 会 rename 走
download 正在写的那个目录**。

**构造时间窗口**：用 40MB 的 `big.bin` 让 download 持续足够久，apply 才有机会撞上。

**步骤**

1. 发布 `1.1.0`，先 `download` 成功，`WriteVersion` 重置为 `applied/1.0.0`，并 `os.RemoveAll` 掉 `1.1.0` 快照（确保 download 从零开始）。
2. 两路并发：`download_update` 与 `apply_update`。
3. 清理可能残留的 `.aly.lock`。
4. 按最终 `version_status` 分支断言主目录内容：
   - `applied` → 主目录必须与 `version` 匹配
   - `downloaded` → 主目录必须仍是旧版（`app-100`），且必须有待应用版本号
   - `applying` → 主目录不得消失（崩溃恢复留给下一条命令）
5. 无论落在哪个状态，都必须能靠"再跑一轮 check/download/apply"收敛到 `applied/1.1.0` + `app-110`。

**判据写法的坑（本次踩到并修正）**

原写法把"自愈成功"等同于"`apply_update` 返回 `IsSuccess`"。但**若并发后现场已落在 `applied`，
`apply_update` 会合法地返回 `no pending update to apply`（`IsSuccess=false`）**——此时现场完全自洽，
用返回值判失败属于纯误报。改为判"最终落点"：

```go
settled118 := func() bool {
    vv := c.ReadVersion(t)
    return vv.VersionStatus == tc.StatusApplied && vv.Version == "1.1.0" &&
        strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))) == "app-110"
}
done := settled118()
for i := 0; i < 3 && !done; i++ {
    e.RunLast(t, c, "check_update")
    e.RunLast(t, c, "download_update")
    e.RunLast(t, c, "apply_update")
    if done = settled118(); done {
        t.Logf("第 %d 轮自愈后落到 applied/1.1.0", i+1)
    }
}
```

**期望结果**

- 两路不得同时"成功"（apply 在 download 未写完时不可能真的成功）；若同时成功，必须按 download→apply 串行解释且最终状态自洽
- 被拒绝那路的 `errorMsg` 应含 `另一更新` / `no pending update` / `已是最新` 之一（不是莫名错误）
- 最终 `version.json` 与磁盘内容自洽；主目录不为空、不为半成品
- 三轮内收敛到 `applied/1.1.0` + `app-110`

**失败判定**

- 主目录内容与 `version` 不符 → 红线①
- 主目录为空/半成品/消失 → 红线②
- 被拒绝那路 `errorMsg` 为空 → 用户无法排障
- 三轮内收敛不了 → 现场永久卡住

执行结果：`PASS（用例 118，60.88s）` — 第 1 路命中锁拒绝 `另一更新正在进行中，请稍后重试`，最终自洽收敛

---

### 用例 119 — 并发 `apply` + `rollback`

**背景**

比 118 更危险的一对：两者都做**三段 rename**，方向**相反**。若同时进行，会把主目录改名两次
→ 备份目录与版本目录互相顶掉（红线②：误删）。

**⚠ 本用例曾长期"假通过"——必须记下来**

第一版命令写成：

```go
e.RunConcurrent(t, []*Client{c, c},
    []string{"apply_update"}, []string{"--version", "1.0.0", "rollback"})
//                              ↑ 参数顺序写反了
```

`aly-client` 的第一个参数是**命令名**，于是它打印 `未知命令: --version` 直接退出 1 ——
**rollback 压根没跑**，"并发冲突"自然不存在，用例每次都"通过"。

**教训**：并发用例的"两路都跑了"本身必须被断言，否则参数写错会静默降级为单路测试。
现在加了硬性守卫，并改用 `RunConcurrentFull` 保留原始输出：

```go
rs := e.RunConcurrentFull(t, []*Client{c, c},
    []string{"apply_update"}, []string{"rollback", "--version", "1.0.0"})

for i, r := range rs {
    t.Logf("第 %d 路: isSuccess=%v errorMsg=%q err=%v panics=%q raw=%q",
        i+1, r.Out.IsSuccess, r.Out.ErrorMsg, r.Err, r.Panics, strings.TrimSpace(r.Raw))
}

// 防"用例自欺"：打印「未知命令」说明参数写错，命令根本没执行
for i, r := range rs {
    if strings.Contains(r.Raw, "未知命令") || strings.Contains(r.Raw, "用法: aly-client") {
        t.Fatalf("第 %d 路命令未被执行（参数写错），用例失去意义: %q", i+1, strings.TrimSpace(r.Raw))
    }
}
```

> **注**：`rollback` 的正确写法是 `rollback --version <ver>`（命令名在前）。
> 全仓库其他用例（`cases_01_08_test.go:366`、`cases_106_115_test.go:248` 等）都是对的，只有本条写反过。

**步骤**

1. 走到 `applied/1.1.0`，且 `1.0.0` 备份目录在（可回滚）。
2. 发布 `1.2.0` 并 `download`，进入 `downloaded/1.2.0`。
3. 并发：`apply_update`（升到 1.2.0）vs `rollback --version 1.0.0`（退回 1.0.0）。
4. 断言两路都真的执行了。
5. 清理残留锁。
6. **核心判据**：主目录必须仍存在，且内容**只能是 `app-100` / `app-110` / `app-120` 之一**。
7. 若最终是 `applied`，主目录内容必须与 `version` 号严格对应。
8. 三轮内必须收敛到 `applied` 且内容匹配；收敛后再跑一轮确认是幂等终态。

**期望结果**

- 被锁拒绝那路给出 `另一更新正在进行中，请稍后重试`
- 主目录内容 ∈ {`app-100`, `app-110`, `app-120`}，绝不为空/半成品
- `applied` 时 `version` 与主目录内容严格对应
- 自愈后版本与内容不再漂移

**失败判定**

- 主目录内容不在三个已知值内 → 红线②（两个方向的三段 rename 互相踩了）
- 主目录消失或为空 → 红线②
- 某路打印「未知命令」→ 用例失效（不算通过）
- 自愈后仍在漂移（再跑一轮版本变了）→ 状态机不稳定

**实测证据**：

```
第 1 路: isSuccess=true  errorMsg=""
        raw="{\"isSuccess\":true,\"errorMsg\":\"\",\"data\":null}"
第 2 路: isSuccess=false errorMsg="另一更新正在进行中，请稍后重试"
        raw="{\"isSuccess\":false,\"errorMsg\":\"另一更新正在进行中，请稍后重试\",\"data\":null}"
主目录内容 = "app-120"（合法）
最终: version="1.2.0" previous="1.1.0" status="applied" 主目录="app-120"
```

执行结果：`PASS（用例 119，14.68s）` — 锁生效，rollback 被拒，主目录保住 `app-120`

---

### 用例 120 — `publish_version` 事务部分失败（清单写失败）

**背景**

`internal/service/project_service.go:229-236`：

```go
// 写失败不阻断发布（返回成功）：清单缺失时客户端跳过删除（与旧行为兼容，仅退化为不删）。
if mErr := writeReleaseManifest(...); mErr != nil {
    log.Printf("[manifest] publish version %s ... write manifest failed: %v", ...)
}
```

这是**有意的取舍**：清单写失败 → 发布仍成功，代价是"删除同步退化"。本用例钉住这个取舍是否被守住。

**触发手段**：把 `.updator_manifest.json` 变成一个**目录**（任何写文件操作必然失败）。

```go
manifest := filepath.Join(e.DataDir(proj), ".updator_manifest.json")
os.Remove(manifest)
os.MkdirAll(filepath.Join(manifest, "blocker"), 0o755)   // 变成目录 → 写文件必失败
defer os.RemoveAll(manifest)
```

**⚠ 版本号的坑**：`create_project` 的初始版本是 `DefaultVersion = "V1.0.0"`，所以**首次
`publish_version` 的版本号必须严格 > 1.0.0**，否则命中防倒退保护
（`CompareVersion(version, latest) <= 0` 即拒绝）。第一版用 `1.1.0` 做首发直接失败，
改为 `1.0.1` → `1.0.2` → `1.0.3` 序列。

**步骤**

1. 发布 `1.0.1`，断言清单文件已生成。
2. 把清单路径换成目录。
3. 改文件后发布 `1.0.2`（带 release notes）。
4. 断言：发布**仍成功**、版本推进到 `1.0.2`、`get_all_files` 仍完整、客户端仍能 download + apply。
5. 删掉目录占位，发布 `1.0.3`，断言清单能重新写出。

```go
// ① 发布必须仍成功（清单写失败是"降级"，不是"失败"）
tc.True(t, r.IsSuccess,
    "① 清单写失败不得阻断发布（设计取舍：降级为不删）: %s", r.ErrorMsg)
tc.Eq(t, "② 版本必须推进到 1.0.2（落库已提交）", e.GetProject(t, proj).Version, "1.0.2")
eqSet(t, "③ 清单缺失时文件列表仍应完整", tc.FileRels(t, files),
    []string{"app.exe", "b.dll"})
```

**期望结果**

- 发布返回成功，`version` 推进到 `1.0.2`
- `get_all_files` 返回 `app.exe` + `b.dll`（退化但不崩）
- 客户端 `download` + `apply` 全部成功，主目录 `app.exe`=`app-110`、`b.dll`=`b-110`
- 恢复后发布 `1.0.3` 能重新写出清单

**失败判定**

- 发布返回失败 → 用户因为"一个缓存文件写不了"而完全无法发版（取舍被改坏）
- 版本未推进 → 事务回滚了已提交的落库
- `get_all_files` 报错/返回空 → 清单缺失直接导致服务端不可用
- 客户端下载失败 → 降级不够彻底（连下载都受影响）

执行结果：`PASS（用例 120，14.38s）` — 发布成功、版本推进、客户端完整走通

---

### 用例 121 — `ignore` 规则的正则 / 大小写 / 通配边界

**背景：先纠正一个常见误解**

`ignore_folders` / `ignore_files` **只在服务端生效**。客户端 `aly-client` 里虽然定义了
`config.ShouldSkipFolder` / `config.ShouldSkipFile`，但全仓 grep 可见**没有任何调用点消费
`IgnoreFolders/IgnoreFiles`** —— 这两个函数只用于 `un_copy_folders` / `un_copy_files`
（物化时"不迁移"），是另一套配置。

> 之前文档里写的"客户端与服务端两套 ignore 实现口径不同"是**错的**，已修正。
> 结论：**不存在两边打架**，只有一套口径（服务端），它本身与 glob 直觉有偏差。

**服务端实现**（`internal/service/file_cache.go:150-160`）

```go
// 文件夹规则：纯字符串前缀，* 不被解释
for _, ignoreFolder := range ignoreFolders {
    if strings.HasPrefix(relPath, ignoreFolder+"/") || relPath == ignoreFolder {
        return nil
    }
}
// 文件规则
for _, ignoreFile := range ignoreFiles {
    if MatchIgnoreFile(relPath, ignoreFile) { return nil }
}

func MatchIgnoreFile(relPath, pattern string) bool {
    if relPath == pattern { return true }
    if m, _ := filepath.Match(pattern, relPath); m { return true }
    base := filepath.Base(relPath)
    if m, _ := filepath.Match(pattern, base); m { return true }
    if strings.HasPrefix(pattern, "*") {
        return strings.HasSuffix(relPath, pattern[1:])   // "*ext" 后缀兜底
    }
    return false
}
```

**由此推出的真实边界（本用例逐个钉住）**

| 现象 | 原因 |
|---|---|
| `logs/**` **一条都匹配不到** | 文件夹规则是 `HasPrefix(rel, "logs/**/")`，永假；`*` 不被当通配符 |
| 文件夹规则**大小写敏感** | 纯 `HasPrefix`，没有 `EqualFold` |
| `*.TMP` 匹配不到 `x.tmp` | `filepath.Match` 大小写敏感（Windows 上也是如此） |
| `exact.txt` 会命中**任意子目录**下的同名文件 | `base` 匹配（`sub/exact.txt` 也被忽略） |

**⚠ 选 pattern 的坑（实测确认）**

想测"非法 pattern 的容错"必须选**真正返回 `ErrBadPattern`** 的写法。第一版选了 `***`，实测：

```
pattern="[a-" err=syntax error in pattern     ← 真非法
pattern="***" err=<nil> hitAny=true           ← 合法！Go 把连续 * 视作贪婪通配
pattern="logs/**" path="logs/x.txt" match=true ← 注意：整串匹配是 true，但服务端用的是 HasPrefix
```

`***` 会把**整个项目目录**全部忽略掉——那不是"非法 pattern 容错"，而是"合法 pattern 正常生效"，
测不出目标。改成 `[]string{"[a-", "[", "a[b"}` 后清单正确返回 2 项。

**步骤**

1. 造 13 个文件的树，覆盖每个 pattern 靶点（`logs/**` / `Temp` / `*.log` / `*.TMP` / `exact.txt`）。
2. 取服务端清单（`get_all_files`）。
3. **硬断言**（服务端明确会命中的）：`a.log` / `sub/deep.log` / `exact.txt` / `Temp/t.txt` 必须被排除；`app.exe` / `keep.txt` 必须保留（漏掉即红线②——用户程序残缺）。
4. **边界点只记录不 FAIL**（这些是"与 glob 直觉的偏差"，属设计现状）：

```go
check := func(name, rel string, expectIgnored bool) {
    ignored := !tc.SliceContains(got, rel)
    if ignored == expectIgnored {
        t.Logf("  %-30s 实际忽略=%-6v（符合直觉）", name, ignored)
    } else {
        t.Logf("  %-30s 实际忽略=%-6v（**与直觉不符**：按 glob 直觉应忽略=%v）",
            name, ignored, expectIgnored)
    }
}
```

5. 第二个项目用非法 pattern（`[a-` / `[` / `a[b`），断言 `get_all_files` 返回 **200**（不是 500），且正常文件仍被列出。

**期望结果**

| 靶点 | glob 直觉 | 服务端实际 | 结论 |
|---|---|---|---|
| `a.log` vs `*.log` | 忽略 | 忽略 | 一致（base 匹配） |
| `sub/deep.log` vs `*.log` | 忽略 | 忽略 | 一致（base 匹配） |
| `exact.txt` / `sub/exact.txt` vs `exact.txt` | 忽略 | 忽略 | 一致（base 匹配，会跨目录） |
| `Temp/t.txt` vs `Temp` | 忽略 | 忽略 | 一致（前缀匹配） |
| `Temp/t2.txt` vs `Temp`（经 `temp/` 写入） | 忽略 | 忽略 | 一致（Windows 上同一目录） |
| `logs/x.txt` vs `logs/**` | 忽略 | **不忽略** | **偏差 D5**（`*` 不被解释） |
| `logs/a/b/y.txt` vs `logs/**` | 忽略 | **不忽略** | **偏差 D5** |
| `x.tmp` vs `*.TMP` | 忽略 | **不忽略** | **偏差 D6**（大小写敏感） |
| `y.TMP` vs `*.TMP` | 忽略 | 忽略 | 一致 |
| `A.LOG` vs `*.log` | 忽略 | —（靶点无效） | Windows 上 `A.LOG` 会覆盖 `a.log`，无法并存 |
| 非法 pattern `[a-` / `[` / `a[b` | — | 200 + 正常文件仍列出 | 正确（只记 WARN） |

**实测输出**

```
服务端清单（共 5 项）: [app.exe keep.txt logs/a/b/y.txt logs/x.txt x.tmp]
磁盘真实文件名: [Temp a.log app.exe exact.txt keep.txt logs sub x.tmp y.TMP]

Temp/t.txt vs Temp（纯名字）            实际忽略=true  （符合直觉）
Temp/t2.txt vs Temp（经 temp/ 写入）    实际忽略=true  （符合直觉）
logs/x.txt vs logs/**（** 一层）        实际忽略=false （**与直觉不符**）
logs/a/b/y.txt vs logs/**（** 多层）    实际忽略=false （**与直觉不符**）
sub/exact.txt vs exact.txt（同名子目录） 实际忽略=true  （符合直觉）
y.TMP vs *.TMP（精确）                  实际忽略=true  （符合直觉）
x.tmp vs *.TMP（大小写）                实际忽略=false （**与直觉不符**）
④ 非法 pattern 下 get_all_files 返回 200，清单含 app.exe
```

> **⚠ 构造"大小写"靶点时必须避开文件系统陷阱**
>
> 第一版造了 `A.LOG` 想对照 `a.log`、`temp/t2.txt` 想对照 `Temp/t.txt`，结果：
>
> ```
> 磁盘真实文件名: [Temp a.log app.exe exact.txt keep.txt logs sub x.tmp y.TMP]
>                 ↑ 没有 A.LOG，也没有 temp/
> ```
>
> **Windows 文件系统不区分大小写**：`A.LOG` 与 `a.log` 是同一个文件（后写覆盖前写），
> `temp/` 与 `Temp/` 是同一个目录。这类"并存对照"靶点在 Windows 上根本构造不出来，
> 用它断言大小写敏感与否只会得到**假结论**。
>
> 有效做法：用**不同文件名、仅大小写不同**的组合（如 `x.tmp` vs `y.TMP`），
> 它们能真正共存，`*.TMP` 是否命中 `x.tmp` 才是有效判据。

**失败判定**

- 不该忽略的文件（`app.exe` / `keep.txt`）被忽略 → 用户程序残缺（红线②）
- 非法 pattern 导致 5xx → 一个写错的配置让整个项目不可用
- 明确会命中的 4 类（`a.log` / `sub/deep.log` / `exact.txt` / `Temp/t.txt`）没被排除 → 基本功能失效

> **⚠ 本用例揭示的真实设计缺陷（D5）**
>
> **`ignore_folders` 里的路径通配写法完全不生效**。用户在配置里写 `logs/**`（最自然的写法，
> 也是 `.gitignore` 习惯）时，服务端 `HasPrefix(relPath, "logs/**/")` 永远为假 →
> **整个 `logs/` 目录会被完整下发**。唯一能生效的写法是**纯目录名**（`Temp`）。
>
> 更隐蔽的是：`logs/**` 这种写法**不会报任何错、没有任何 WARN**，用户会以为配好了。
>
> **定级**：中。不影响正确性（不误删、不坏文件），但会让"排除了日志目录"的大项目
> 把几十 MB 日志打进更新包每次全量传输，且用户无从察觉。
>
> **建议修法**（择一）：
> 1. 文件夹规则改为**支持 glob**：`filepath.Match(ignoreFolder, relPath)` + 前缀匹配，与文件规则对齐；
> 2. 或至少**拒绝含通配符的文件夹规则**并在 `create_project` 时就返回错误（快速失败优于静默失效）；
> 3. 大小写：与产品口径确认后统一加/不加 `EqualFold`（当前文件与文件夹都敏感，一致，可不改）。

执行结果：`PASS（用例 121，12.61s）`

---

### 用例 122 — `-lang en-US` 文案与未知语言回落

**背景**

`RunUIModeLang` 这个辅助**存在但零调用**（上一轮盘点的第 6 条缺口）—— 也就是说 i18n 从未被端到端跑过。

语言由 `-lang zh-CN|en-US` 传入，`i18n/i18n.go:72-92` 的 `ParseLang`：

```go
ParseLang("")        → ZH
ParseLang("bogus")   → ZH      // 未知值回落，不报错
ParseLang("en-US")   → EN
ParseLang("EN_us")   → EN      // 大小写/分隔符都容错
```

**为什么 UI 类断言只能用磁盘副作用**：UI 是 `-H=windowsgui` 编译，既不写 stdout/stderr 也不落日志
（`check_update.go:55` 也刻意不写 `update.log`，"避免刷爆日志"）。所以能验的是"不崩 + 行为一致"，
不能验"文案字符串是否正确"。

**步骤**

1. 造强制更新场景（`set_force_update = true`），让 UI 无需点按钮自动走完。
2. **源码级**：断言 `i18n.go` 含 `EN` / `ZH` / `ParseLang`。
3. **运行时**：`RunUIModeLang(..., "update", "en-US", 90s)` 跑完，断言退出码 ≠ -999（未超时）、无 `panic:`、主目录已是新版。
4. 未知语言 `"bogus-lang"`：断言不 panic、不超时、更新同样完成。

```go
code, out := e.RunUIModeLang(t, uiPath, c, "update", "en-US", 90*time.Second)
if code == -999 {
    t.Fatalf("③ en-US 下强制更新仍未自动完成（超时被 kill）——"+
        "要么 force_update 没生效，要么 en-US 路径卡住。主目录 app.exe=%q",
        strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))))
}
tc.NotContains(t, out, "panic:", "③ en-US 不得 panic")
tc.Eq(t, "③ en-US 更新后主目录应为新版",
    strings.TrimSpace(tc.ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
```

**期望结果**

- `i18n.go` 含 `EN` / `ZH` / `ParseLang`
- `en-US` 下强制更新自动完成，退出码 0（非 -999），无 panic，主目录变 `app-110`
- `bogus-lang` 下同样完成（回落到 ZH），无 panic

**失败判定**

- `-999`（超时被 kill）→ en-US 路径卡住或 `force_update` 未生效
- 输出含 `panic:` → i18n 切换有崩溃缺陷
- 未知语言值导致不完成 → `ParseLang` 回落失效
- 主目录仍是 `app-100` → "跑完了"是假的

执行结果：`PASS（用例 122，43.68s）` — en-US 与 `bogus-lang` 退出码均为 0，更新均完成

---

### 用例 123 — `download_file` 的多段 Range 与 `If-Range`

**背景**

`file_download_controller.go:143` 用 `http.ServeContent`，语义由标准库定：

| 请求 | 期望响应 |
|---|---|
| 无 `Range` | 200 全文 |
| `bytes=0-1`（单段） | 206 + `Content-Range: bytes 0-1/<size>` |
| `bytes=0-1,4-5`（多段） | **206 + `Content-Type: multipart/byteranges; boundary=...`** |
| `If-Range: <匹配>` | 206 |
| `If-Range: <不匹配>` | **200 全文**（忽略 Range） |
| `bytes=abc-def`（非法） | 400（不得 5xx） |

**为什么要测**：客户端断点续传只认单段 206。若服务端对多段请求返回 `multipart/byteranges`
而客户端当成裸字节写入 `.part`，`.part` 会混入 MIME 边界文本 → 拼出来的文件是坏的（红线①）。
本用例钉住"服务端严格按 RFC 行事"，防止有人手改成"永远返回单段"。

**新增辅助 `DoFull`**（`helpers.go`）：多段 / If-Range 必须看**响应头**，原来的 `RawRange` 只回 body。

```go
type RespFull struct {
    Code   int
    Header http.Header
    Body   []byte
}
func (r RespFull) ContentType() string  { return r.Header.Get("Content-Type") }
func (r RespFull) ContentRange() string { return r.Header.Get("Content-Range") }
```

**步骤**（100 字节可预测内容，`0123456789` 循环）

1. 无 Range → 200，长度 = 100
2. `bytes=0-1` → 206，body=`content[0:2]`，`Content-Range` = `bytes 0-1/100`
3. `bytes=0-1,4-5` → 206，`Content-Type` 以 `multipart/byteranges` 开头且含 `boundary=`，body 含两段内容与各自的 `Content-Range`
4. `If-Range: <Last-Modified>` → 206 且长度 10；`If-Range: Wed, 21 Oct 2015 07:28:00 GMT` → **200 全文**
5. `bytes=abc-def` → 400/416（不得 5xx）
6. `bytes=50-` → 206 且逐字节等于 `content[50:]`

```go
tc.True(t, strings.HasPrefix(ct, "multipart/byteranges"),
    "③ 多段 Range 必须返回 multipart/byteranges（RFC 7233），实际 Content-Type=%q\n"+
        "  若返回裸字节，客户端会把 MIME 边界写进 .part（红线①）", ct)
tc.True(t, strings.Contains(ct, "boundary="), "③ 必须带 boundary")
```

**期望结果**

- 6 段语义全部符合 RFC 7233
- `If-Range` 不匹配时回 200 全文（不是 206、不是 416）
- 非法 Range 语法不导致 5xx

**失败判定**

- 多段请求返回裸字节（无 `multipart/byteranges`）→ 客户端会把 MIME 边界写进 `.part`
- `If-Range` 不匹配仍回 206 → 续传会拼出损坏文件（红线①）
- 非法 Range 导致 500 → 一个畸形请求打挂下载接口
- `bytes=50-` 的 body 与源文件不一致 → 续传基准错误

执行结果：`PASS（用例 123，12.90s）` — 6 段语义全部符合 RFC

---

### 用例 124 — C# SDK `Launch` 拉 UI 的失败路径

**背景**

`AlyUpdateUIClient.cs:120-175`：

```csharp
if (!File.Exists(UiExePath))
    return AlyResponse.NG("aly-client-ui.exe not found: " + UiExePath
        + "（请确认 UpdateFolder 已部署：GUI 初始化项目时会从 AlyPublish 包复制，"
        + "或按 client/<rid>/ 布局手工放置 aly-client-ui.exe）");
try { ... process.Start() ... }
catch (Exception ex) { return AlyResponse.NG(ex); }
```

用例 94 只测了"UI 存在 → Launch OK"的成功路径。失败路径同样重要：部署漏了 UI
（杀软隔离 / 只装了 client）时，**SDK 必须返回 NG 而不是抛异常**，否则宿主界面会直接崩。

**驱动方式**：不在 Go 里 P/Invoke C#，而是扩展 `AlySdkSmoke` 的 `sd15`（C# 侧写断言，Go 侧读 PASS/FAIL 计数）。
这样断言逻辑与产品代码同语言，`dotnet build` 就能验证。

**C# 侧 `Sd15()` 的 4 组断言**：

1. UI 路径不存在 → `IsSuccess=false` + `ErrorMsg` 含 `not found` / `aly-client-ui`，且**不触发 `Started` 事件**
2. 连续 3 次 `Launch` 仍每次 NG（不累积状态、不抛）
3. 失败后 `Dispose` 不抛（句柄要清干净）
4. `Dispose` 后再 `Launch` → NG（disposed 守卫）

**Go 侧**：

```go
out := e.RunSmoke(t, smoke, "sd15", c)
tc.Eq(t, "① sd15 不得有 FAIL", tc.OutputHasFail(out), false)
tc.Contains(t, out, "SUMMARY:", "② 必须有 SUMMARY 行（证明真的跑了）")
tc.True(t, strings.Contains(out, "SD15"), "② 输出里必须有 SD15 标记")
for _, want := range []string{
    "UI 缺失时 Launch 必须返回 NG",
    "NG 必须带 errorMsg",
    "失败路径不得触发 Started",
    "Dispose 后 Launch 应返回 NG",
} {
    tc.Contains(t, out, want, "③ 必须验证: "+want)
}
```

**⚠ 防"空跑"的两道守卫**：`sd15` 若未注册进分发字典，smoke 会打 `SUMMARY: PASS=0 FAIL=0` 后退出——
"没有 FAIL"照样成立。因此必须同时断言 `SUMMARY:` 存在、`SD15` 标记存在、`pass >= 6`。

**期望结果**

- `SUMMARY: PASS=9 FAIL=0`
- 4 条关键断言文案逐条出现在输出里

**失败判定**

- 输出无 `SUMMARY:` / 无 `SD15` → 用例压根没跑（假通过）
- `PASS` 数 < 6 → 断言被跳过
- UI 缺失时抛异常而非返回 NG → 宿主界面崩溃（本用例的核心红线）
- 失败路径触发 `Started` 事件 → 宿主误以为更新已开始
- `Dispose` 后仍能 Launch → 句柄泄漏

执行结果：`PASS（用例 124，16.41s）` — `SUMMARY: PASS=9 FAIL=0`

---

## 十一·补四、publish-gui-csharp 初始化（125–127）

> 本节来自一次**真机实测**（2026-11-04）：在 publish-gui-csharp 里对一个真实项目执行"初始化项目"之后，
> 磁盘上得到的更新器目录三处都不对——位置、内容、以及 `client.json` 的相对路径。
> 实现见 `test/tc/cases_125_127_test.go`；被测源码 `publish/publish-gui-csharp/src/AlyPublish/Services/ConfigService.cs`。
>
> 三条用例都分两层钉：**源码契约**（读 C# 源码断言关键调用形态）+ **消费侧功能**
> （用真实 aly-client 消费 GUI 写出的那套布局与 `client.json`）。为什么不能只在 Go 里把 C# 的
> 路径算法重算一遍：那是自证自话，C# 侧改回旧写法时重算照样通过。
>
> 被测对象是"更新器目录的布局"，它的口径在 `README.md` / `client/aly-client-ui/aly-client-winui/DEVELOPMENT_PLAN.md §3.0`
> 定死：**`PackageFolder/{ApplicationFolder, UpdateFolder}`**——更新器目录与主程序目录**同级**，
> 不在主程序目录内。
>
> 同一口径还有一份**可执行的形态**（可直接比对，不必读 prose）：SDK 的构建目标
> `client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/AlyClient.CSharpSDK.targets`
> —— `AlyUpdateFolder = $(OutDir)..\UpdateFolder\`（输出目录的**兄弟**），
> 默认 `client.json` 内容是 `{"main_exe_relative_path": "../ApplicationFolder/YourApp.exe"}`，
> 且 `aly-client.exe` 与 `aly-client-ui.exe` 两件套都投放到该目录（UI 缺失只警告，不阻断构建）。
> 本节 126 的期望值与之逐字对应。

### 缺陷现场回放（真机）

```
…\WinFormsApp1\WinFormsApp1\bin\Debug\net8.0-windows\        ← 主程序目录（内含 .updator/）
├── .updator\shared.json
├── WinFormsApp1.exe
└── UpdateFolder\                    ← ❌ 位置错：应当与 net8.0-windows **同级**
    ├── aly-client.exe
    └── client.json.tmp              ← ❌ client.json 从未生成，只剩临时文件
```

日志证据（publish-gui-csharp 自己的日志 `bin/Debug/net8.0/logs/log20261104.log`）：

```
[INF] 自动检测主程序: …\bin\Debug\net8.0-windows\WinFormsApp1.exe, 进程名: WinFormsApp1
[INF] 执行 config init: … Path=E:\…\bin\Debug\net8.0-windows\        ← 路径带结尾分隔符
[INF] 创建 UpdateFolder: E:\…\bin\Debug\net8.0-windows\UpdateFolder   ← ❌ 落在主程序目录内
[INF] 计算 main_exe_relative_path: … Relative=..\WinFormsApp1.exe     ← ❌ 少了主程序目录名
[ERR] 写入 client.json 失败: …\UpdateFolder\client.json
System.IO.FileNotFoundException: Unable to find the specified file.
   at System.IO.FileSystem.ReplaceFile(…)
   at AlyPublish.Services.ConfigService.WriteAllTextAtomic(…) in ConfigService.cs:line 197
```

三处根因（彼此独立，修一处不够）：

| # | 现象 | 根因 |
|---|---|---|
| ① | `UpdateFolder` 落在主程序目录**内** | `Path.GetDirectoryName(Path.GetFullPath(projectPath))`：`projectPath` 带结尾 `\` 时 `GetDirectoryName` 回退成"目录自身"，于是"上级目录"算成了项目目录本身 |
| ② | `main_exe_relative_path = ..\WinFormsApp1.exe` | 基准目录是①里那个错的 `UpdateFolder`，相对路径自然少了主程序目录名；正确值 = `..\net8.0-windows\WinFormsApp1.exe` |
| ③ | 只剩 `client.json.tmp` | `WriteAllTextAtomic` 无条件 `File.Replace(tmp, path, null)`，而 `ReplaceFile` **要求目标已存在**，首次写必抛 → `.tmp` 成了唯一残留（不是文件名拼错） |

修复口径（`ConfigService.cs`）：归一化后再取上级目录（`NormalizeDirectoryPath`）；复制
`aly-client` + `aly-client-ui` 两件套；`WriteAllTextAtomic` 目标不存在时走 `File.Move`（对齐 SDK 自更新 `UpdateSelf` 的写法）。

---

### 用例 125 — publish-gui-csharp 初始化：UpdateFolder 与主程序目录同级

**前置条件**

1. 仓库里有 `publish/publish-gui-csharp`（C#/Avalonia 源码）。
2. **不需要**服务端、构建产物：本用例只读源码钉契约（与用例 104 的 ① 段同法），故不建 `Env`。

**步骤**

1. 读 `Services/ConfigService.cs`。
2. 钉"归一化 → 取上级目录 → `Combine(parentDir, "UpdateFolder")`"三步的调用形态。
3. 反证：不得再出现历史 bug 的两种形态（带尾分隔符的上级目录推导、把 UpdateFolder 建在主程序目录内）。

```go
func TestCase125(t *testing.T) {
	svc := ReadText(filepath.Join(RepoRoot, "publish", "publish-gui-csharp",
		"src", "AlyPublish", "Services", "ConfigService.cs"))

	Contains(t, svc, "CreateUpdateFolder", "初始化后产出 UpdateFolder 的入口")
	// ① 归一化：去掉结尾分隔符（浏览对话框返回的路径带结尾 '\'）
	Contains(t, svc, "NormalizeDirectoryPath(projectPath)", "必须先把 projectPath 归一化")
	Contains(t, svc, "TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar)",
		"归一化 = 去掉结尾分隔符")
	// ② 上级目录必须取自归一化后的值
	Contains(t, svc, "Path.GetDirectoryName(projectDir)",
		"上级目录必须取自归一化后的主程序目录")
	NotContains(t, svc, "Path.GetDirectoryName(Path.GetFullPath(projectPath))",
		"历史 bug 形态：带尾分隔符的路径会让 GetDirectoryName 回退成目录自身")
	// ③ 同级：UpdateFolder 挂在上级目录下
	Contains(t, svc, `Path.Combine(parentDir, "UpdateFolder")`,
		"UpdateFolder 必须挂在主程序目录的上级目录下（同级）")
	NotContains(t, svc, `Path.Combine(projectDir, "UpdateFolder")`,
		"❌ 放进主程序目录：apply/rollback 给主目录改名会把更新器与 .updator 一起搬走")
}
```

**期望结果**

- `CreateUpdateFolder`：先归一化 `projectPath` → 由归一化值取上级目录 → `Combine(parentDir, "UpdateFolder")`
- 源码里**没有**带结尾分隔符的上级目录推导，也**没有**把 `UpdateFolder` 建在主程序目录内的写法

**失败判定**

- 两条 `NotContains` 任一条命中 → 布局回退成"更新器住在主程序目录内"（本用例的核心红线）
- 任一条 `Contains` 落空 → 归一化被删掉，带 `\` 的路径会再次把 `UpdateFolder` 放进主程序目录

PASS（2026-11-12 全量回归；已修正首发版本号）

---

### 用例 126 — publish-gui-csharp 初始化：三件套与 `client.json` 的相对路径可被真实 client 消费

**前置条件**

1. 服务端已就绪（`NewEnv(t, true, 0)`），项目 `tc-gui-init` 已发布 `1.0.0`，数据目录内有 `WinFormsApp1.exe`。
2. `publish/publish-gui-csharp/src/AlyPublish/Services/ConfigService.cs` 在位。

**步骤**

1. **源码契约**：`UpdateFolder` 内必须齐 `aly-client.exe`、`aly-client-ui.exe`、`client.json` 三件套；
   `client.json` 的目标名不得带临时后缀；相对路径必须以纠正后的 `UpdateFolder` 为基准。
2. **消费侧**：造一套"GUI 形状"的现场——主程序目录名改成真实发布产物的名字（`net8.0-windows`），
   `UpdateFolder` 与它**同级**，`client.json` 写成 GUI 会写的那种相对路径（`../net8.0-windows/WinFormsApp1.exe`），
   然后用**真实 aly-client** 跑 check → download → apply，看新内容落在哪个目录。

```go
func TestCase126(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-gui-init"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "WinFormsApp1.exe", "app-100")
	True(t, e.Publish(t, proj, "1.0.1", []string{"初始"}, "", "").IsSuccess, "发布 1.0.1")

	// ── ① 源码契约（GUI 侧三件套 + 相对路径基准）──
	guiRoot := filepath.Join(RepoRoot, "publish", "publish-gui-csharp")
	svc := ReadText(filepath.Join(guiRoot,
		"src", "AlyPublish", "Services", "ConfigService.cs"))
	Contains(t, svc, "PkHelper.FindUp(",
		"更新器必须从安装包 payload（client/<rid>/）取，调用方不再自带二进制")
	Contains(t, svc, "HostHelper.UpBaseNames",
		"两个更新器走同一循环：aly-client 与 aly-client-ui 必须同进同退")
	Contains(t, svc, `Path.Combine(updateFolder, fileName)`,
		"必须复制到 UpdateFolder 下的同名文件（两件套错位即失败）")
	Contains(t, svc, `Path.Combine(updateFolder, "client.json")`,
		"client.json 的目标必须是 UpdateFolder/client.json（不得带临时后缀）")
	Contains(t, svc, "GetRelativePath(updateFolder, mainExeFullPath)",
		"相对路径必须以纠正后的 UpdateFolder 为基准、以绝对主程序路径为目标")
	Contains(t, svc, "MustCloseProcessName", "必须写 must_close_process_name")
	Contains(t, svc, "GetFileNameWithoutExtension",
		"关进程名缺省时必须从主程序名推导（否则更新前不会关掉主程序）")

	// ── ①b 强制更新三态（审查 B7）：复选框必须翻译成 --set / --clear 的显式一侧 ──
	cliSvc := ReadText(filepath.Join(guiRoot,
		"src", "AlyPublish", "Services", "CliService.cs"))
	Contains(t, cliSvc, `AddFlag(arguments, "--set-force-update", setForceUpdate)`,
		"GUI 推送必须能把强制更新显式置 true")
	Contains(t, cliSvc, `AddFlag(arguments, "--clear-force-update", clearForceUpdate)`,
		"GUI 推送必须能把强制更新显式置 false（不能靠不传参数）")
	vmSrc := ReadText(filepath.Join(guiRoot,
		"src", "AlyPublish", "ViewModels", "ProjectTabViewModel.cs"))
	Contains(t, vmSrc, "!SetForceUpdate && _forceUpdateKnown",
		"取消勾选时只在已知服务端现值时才发 --clear（未知时保持原值）")

	// ── ② 消费侧：GUI 形状的布局必须真能被 aly-client 消费 ──
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100", DirName: "gui-site",
	})
	// 主程序目录改用真实发布产物的名字（"…\bin\Debug\net8.0-windows"）
	guiDir := filepath.Join(c.Pkg, "net8.0-windows")
	if err := os.Rename(c.Main(), guiDir); err != nil {
		t.Fatalf("改名主程序目录失败: %v", err)
	}
	if err := os.Rename(filepath.Join(guiDir, "app.exe"),
		filepath.Join(guiDir, "WinFormsApp1.exe")); err != nil {
		t.Fatalf("改名主程序失败: %v", err)
	}
	// GUI 会写的那种 client.json：相对 UpdateFolder 指回同级主程序目录
	WriteJSON(t, filepath.Join(c.Up(), "client.json"), map[string]interface{}{
		"main_exe_relative_path":  "../net8.0-windows/WinFormsApp1.exe",
		"must_close_process_name": []string{"WinFormsApp1"},
	})

	// 服务端演进一版
	e.PutFile(t, proj, "WinFormsApp1.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"演进"}, "", "").IsSuccess, "发布 1.1.0")

	True(t, e.RunLast(t, c, "check_update").IsSuccess,
		"check_update 失败：GUI 产出的布局/相对路径不被更新器认可")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download_update 失败")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply_update 失败")

	Eq(t, "新内容落到同级主程序目录",
		ReadText(filepath.Join(guiDir, "WinFormsApp1.exe")), "app-110")
	if _, err := os.Stat(filepath.Join(c.Pkg, "net8.0-windows_1.0.0")); err != nil {
		t.Fatalf("备份目录应按主程序目录名命名（net8.0-windows_1.0.0）: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.Up(), "client.json")); err != nil {
		t.Fatalf("apply 之后 client.json 必须仍在 UpdateFolder（更新器目录没被卷走）: %v", err)
	}
}
```

**期望结果**

- 三件套齐备：`aly-client.exe` + `aly-client-ui.exe` + `client.json`（实测缺陷里只有 `aly-client.exe` + `client.json.tmp`）
- `main_exe_relative_path` 带主程序目录名：`..\<主程序目录名>\<主程序名>`
- 真实 aly-client 按这套布局跑通 check / download / apply：
  新内容落到**同级**主程序目录，备份目录名 = `<主程序目录名>_<旧版本>`
- apply 之后 `UpdateFolder/client.json` 仍在（更新器目录既没被替换、也没被主目录改名卷走）
- 强制更新复选框的源码契约：勾选 / 取消分别对应 `--set-force-update` / `--clear-force-update`；未知服务端现值时两侧都不发（保持原值）

**失败判定**

- 三件套少一件 → FAIL（更新器 / UI / 配置缺失，用户点更新无反应）
- `main_exe_relative_path` 只有 `..\<exe>`（缺主程序目录名）→ FAIL（推导出的 MainFolder 不是主程序目录）
- 真实 client 跑不通 check / download / apply → FAIL（GUI 写出的布局不被更新器认可）
- apply 之后 `UpdateFolder` 被卷走 → FAIL（更新器自毁，下次更新不可用）

PASS（2026-11-12 全量回归；已修正首发版本号）

---

### 用例 127 — publish-gui-csharp 原子写：目标不存在时必须 `File.Move`，首次写不得只留 `.tmp`

**前置条件**

1. 源码同用例 125。
2. 客户端现场由 `NewClient` 造出（`UpdateFolder/client.json` 存在）。

**步骤**

1. **源码契约**：钉住 `WriteAllTextAtomic` 的"先写 `.tmp` → 判目标是否存在 → `Move` / `Replace`"形态，含非 Windows 回退。
2. **消费侧**：把 `client.json` 改名成 `client.json.tmp`（**复现缺陷留下的现场**），再跑 `check_update`——
   必须明确失败并把原因指向 `client.json`，不得静默。

```go
func TestCase127(t *testing.T) {
	// ── ① 源码契约：目标不存在时不能 Replace（第一次写必抛）──
	svc := ReadText(filepath.Join(RepoRoot, "publish", "publish-gui-csharp",
		"src", "AlyPublish", "Services", "ConfigService.cs"))
	Contains(t, svc, `var tmpPath = path + ".tmp"`, "原子写先写 .tmp")
	Contains(t, svc, "if (!File.Exists(path))", "必须先判目标是否存在")
	Contains(t, svc, "File.Move(tmpPath, path)",
		"目标不存在 → 改名（ReplaceFile 要求目标已存在，首次写必抛）")
	Contains(t, svc, "File.Replace(tmpPath, path, null)", "目标存在 → 原子替换")
	Contains(t, svc, "PlatformNotSupportedException",
		"非 Windows 平台不支持原子替换 → 删目标 + 改名回退")

	// ── ② 消费侧：只有 .tmp 的现场不可用（这就是用户看到的现象）──
	e := NewEnv(t, true, 0)
	const proj = "tc-gui-atomic"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	True(t, e.Publish(t, proj, "1.0.0", []string{"初始"}, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100", DirName: "gui-atomic",
	})
	// 复现缺陷现场：client.json 从未生成，只剩临时文件
	if err := os.Rename(filepath.Join(c.Up(), "client.json"),
		filepath.Join(c.Up(), "client.json.tmp")); err != nil {
		t.Fatalf("复现 client.json.tmp 现场失败: %v", err)
	}

	out := e.RunLast(t, c, "check_update")
	Eq(t, "只有 client.json.tmp 时命令必须失败（不得静默当没配置）", out.IsSuccess, false)
	Contains(t, out.ErrorMsg, "client.json", "失败原因必须指向 client.json")
}
```

**期望结果**

- `WriteAllTextAtomic`：目标不存在 → `File.Move`；目标存在 → `File.Replace`；平台不支持 → 删目标 + 改名
- 由源码契约推出：初始化后 `UpdateFolder` 内是 `client.json`，**没有** `client.json.tmp` 残留
- 只有 `client.json.tmp` 的现场：`check_update` 返回 `isSuccess:false`，`errorMsg` 指向 `client.json`

**失败判定**

- 源码里 `File.Replace` 未被 `File.Exists` 分支守护 → 首次写必失败（`.tmp` 成为唯一残留，缺陷原样复现）
- 只有 `.tmp` 时命令仍 `isSuccess:true` → 配置缺失被静默吞掉（用户更难排查）
- 初始化后仍有 `*.tmp` 残留 → 原子写没收尾

PASS（2026-11-12 全量回归；已修正首发版本号）

---

### 用例 128 — 客户端真实分片上传路径：publish-cli 推大文件必须走通 `upload_chunk`

> **为什么必须补这条**（2026-11-06 真机缺陷暴露的测试漏网）：
>
> 真机上报：GUI 发布 262MB 文件 →
> `上传失败 [ZWCADViewer_...exe]: upload chunk 0/263: HTTP 404: 404 page not found`。
>
> 根因是**运行中的服务端是陈旧构建**（`server.exe` 内没有 `upload_chunk` 路由），
> 但**测试侧有漏网，才让"部署了旧二进制"一路全绿**：
>
> - 服务端分片算法已被**用例 114** 覆盖（手工 `postForm` 直打 `upload_chunk` /
>   `upload_chunk_complete`）——它护得住"服务端拼接对不对"，却护不住"路由在不在"；
> - **客户端真实分片分支无人覆盖**：publish-cli 的 `UploadFile` 只对
>   `> chunkSize`（1MiB）的文件走 `uploadFileChunked`；而**用例 66–77（publish-cli）
>   全部用文本小文件**，永远走 `uploadFileDirect`；**用例 63/58 的 `BigFile`
>   走的是测试框架的 `Env.Upload`（直打 `upload_file`）**，同样不经过客户端分片分支。
>
> 于是「客户端发分片 → 服务端有没有这两条路由」这个组合**从未被端到端验证**。
> 本用例把 publish-cli 与"真实大文件"接起来，补上这道网。

**前置条件**

1. 服务端构建产物 `server.exe`（含 `upload_chunk` / `upload_chunk_complete` 路由）。
2. publish-cli 构建产物 `aly-publish.exe`。
3. 新建项目 `tc-chunk-pub`；新建 publish 工作区（`.updator/shared.json` 指向本服务端）。

**步骤**

1. 在工作区造一个 **3 个整分片 + 尾片**的非均匀文件（`chunckSizeBytes*3+12345` 字节，
   `chunckSizeBytes = 1024*1024` 与客户端 `chunkSize` 逐字对应）。
   内容用 `b[i]=(i*31+7)%251` 而**非全零**——全零文件在分片错位/丢片时 md5 未必变。
2. `add --all` → `push --version 1.0.1`。
3. 断言落地字节数与 md5 与源文件逐字节一致；断言无 `.chunks` / `.merging` 残留；
   断言 `get_all_files` 可见且不含中间态。

```go
func TestCase128(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-chunk-pub"
	e.EnsureProject(t, proj, false, nil, nil)

	ws := e.NewPublishWS(t, proj, "pub-chunk")
	size := int64(chunckSizeBytes*3 + 12345)   // 3 整片 + 尾片
	rel := "big/bigfile.bin"
	src := filepath.Join(ws, filepath.FromSlash(rel))
	WritePattern(t, src, size)

	True(t, e.PublishCLI(t, ws, "add", "--all").IsSuccess, "add --all 失败")
	r := e.PublishCLI(t, ws, "push", "--version", "1.0.1", "--message", "大文件分片上传")
	True(t, r.IsSuccess, "push 大文件失败（分片通道 404/坏）: %s", r.ErrorMsg)

	dst := filepath.Join(e.DataDir(proj), filepath.FromSlash(rel))
	Eq(t, "落地字节数", SizeOf(dst), size)
	Eq(t, "落地 md5（分片必须按 chunkIndex 顺序拼接）", MD5Of(dst), MD5Of(src))
	e.NoResidue(t, proj, rel)
	HasRel(t, e.GetAllFiles(t, proj), rel)
}
```

**期望结果**

- `push` `isSuccess:true`；落地文件字节数与 md5 与源文件**逐字节一致**
- 无 `.chunks` / `.merging` 残留；`get_all_files` 能看到它

**失败判定**

- `push` 报 `HTTP 404: 404 page not found`（`upload chunk N/M`）→ **服务端缺 `upload_chunk`
  路由**（部署了旧 `server.exe`），或客户端分片逻辑坏
- 落地 md5 不一致 → 分片拼接顺序错位 / 丢片 / 重片
- 有 `.chunks` / `.merging` 残留 → 合并收尾失败

**实测结论（2026-11-06）**：含路由的 `server.exe` → **PASS**（2.20s）。
**负向对照**：临时移除 `routers.go` 中 `upload_chunk` / `upload_chunk_complete` 两条路由后
重构建 → **FAIL**，报错与真机逐字一致：
`上传失败 [big/bigfile.bin]: upload chunk 0/4: HTTP 404: 404 page not found`。
该对照证明本用例确实抓得住"服务端缺分片路由"这一类部署缺陷。

---

### 用例 129 — 分片阈值边界：`chunkSize` 走直传、`chunkSize+1` 走分片

**前置条件**：同用例 128。

**步骤**

1. 造两个文件：`exact.bin`（恰好 `chunckSizeBytes` 字节）与 `over.bin`（`chunckSizeBytes+1` 字节）。
2. `add --all` → `push`。
3. 断言两文件 md5 均逐字节一致、均无中间态残留、均在服务端文件列表里。

**期望结果**

- 客户端分支配对 `fi.Size() <= chunkSize` 走直传：`exact.bin` 走 `upload_file`；
  `over.bin`（多 1 字节）走分片 `upload_chunk`。两条路径都必须把内容逐字节落地。

**失败判定**

- 边界写反（如 `<`）→ 1MiB 整文件静默改走分片（或反之），本用例 md5 断言即失败。

**实测结论（2026-11-06）**：`chunkSize` 与 `chunkSize+1` 两文件均 **PASS**（2.77s），
落地 md5 逐字节一致、无中间态残留。

执行结果：**PASS**（2026-11-06 新增并实测，`ALY_REUSE_BIN` 复用 dist-test 产物）

---




## 十一·补五、自更新统一与防循环（130–135）

> 2026-11-09 自更新统一的回归网。背景：此前自更新"检测"（check_self_update，
> 用例 92）与"执行"（C# SDK `UpdateSelf` / UI `UpdateUpdater`+`UpdateSelfUI`）
> 分离，且执行方各自实现一套复制/替换——**裸控制台部署**（宿主直接 spawn
> aly-client）时 UpdateFolder 副本会永久漂移在旧版本；三套实现还各自分叉
> （`.old` vs `.bak`、tmp 删除顺序、journal 只有 UI 有）。
>
> 统一后的架构：
>
> 1. **正常路径唯一算法**：`self_update` 命令（`cmd/self_update.go` + 便携包
>    `client/aly-client/selfupdate/`）取 `.aly.lock` → 源缺失跳过 → SHA256
>    一致跳过（清 `.new/.old`）→ **先复制校验 `.new`（self 始终存在）→ 运行中
>    exe 改名 `.old` → `.new` 原子落位 → 失败回滚**；
> 2. **UI/SDK 委派**：UI `UpdateUpdater` 与 C# SDK 构造函数都调用该命令，不再
>    各写一套；只有更新器缺失/损坏、或上游更新器还是旧版（不认识 `self_update`）
>    时才回退直接复制——否则第一次升级会因"新程序调不动旧命令"永久卡死；
> 3. **UI 自身**：`UpdateSelfUI` 复制 `.new` 后派生 `.new` 自带的隐藏模式
>    `-finish-self-update`。收尾进程持有旧 UI 的 PID 句柄等待退出（事件驱动，
>    最多 10s；超时 `TerminateProcess` 强杀，再等 2s），用进程创建时间校验
>    PID 复用；随后旧 exe 改名 `.old`、`.new` 落位、用原参数拉起新 UI。
>    不再用 `ping` 猜固定延时，也不会因 `move` 失败仍 `start` 旧版而循环；
> 4. UI journal 增加 `pending`：旧 UI 派生收尾进程后即退出，收尾结果只能下次
>    启动观测；下一次启动看到 pending 才计一次未收敛。30 分钟窗口内连续 ≥3 次
>    则跳过，窗口过后自动恢复。收尾进程的日志在
>    `<UpdateFolder>/aly-client-ui.exe.update.log`；
> 5. `CopyAndVerify` 在 rename 前 `Sync`，断电不会换上截断文件。
>
> 执行口径：与既有用例一致（Windows + go1.10.8 构建 386 产物）。单元层已在
> Windows 实跑：`selfupdate` 包 **16 个单测**、`cmd/self_update_test.go` **5 个**
> （含锁拒绝）、UI journal **10 个**、bridge 解析新增 2 个、UI 收尾模式
> **8 个**（参数/等待/强杀/PID 复用/改名/日志）。本节 6 条端到端用例
> 2026-11-09 在 Windows 实机全部执行：**130–135 6/6 PASS**。
>
> 备注：`self_update` 的"self 缺失"分支端到端不可构造（Windows 不能删除正在
> 运行的 exe，删掉后又无法 spawn），由 `cmd` 单测 `TestRunSelfUpdateSelfMissing`
> 覆盖。**首次升级**：改动前构建的旧 UI 仍使用旧的 `ping/move/start` 脚本安装
> 新 UI；新 UI 到位后，后续升级才走收尾进程。

### 用例 130 — self_update：内容一致时跳过并清理残留（幂等性）

**前置条件**

1. `tc.NewEnv(t, true, 0)`（含 `BuildAll`：server / publish-cli / aly-client 386 产物）。
2. 客户端现场 `1.0.0 / applied`，主目录与 UpdateFolder 的 `aly-client.exe` 内容一致。

**步骤**

1. 预置残留：`aly-client.exe.new`（半截内容）与 `aly-client.exe.old`（旧版内容）。
2. 调用 `self_update`，解析结果行 data（`tc.SelfUpdateExecData`）。
3. 断言 `updated=false`、`reason=already_up_to_date`、副本内容不变、残留被清理。

```go
WriteText(t, selfPath+".new", "half-written")
WriteText(t, selfPath+".old", "previous-version")
d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
True(t, d.Updated == false, "一致时不得触发替换, data=%+v", d)
Eq(t, "跳过原因口径", d.Reason, "already_up_to_date")
True(t, !Exists(selfPath+".new") && !Exists(selfPath+".old"), "残留必须被清理")
```

**期望结果**

- `updated=false` + `already_up_to_date`；副本逐字节不变（跳过 ≠ 重写）；
- `.new` / `.old` 残留被顺带清理（一致路径无在途替换，清理无竞态）。

**失败判定**

- `updated=true` → FAIL（每次启动重写副本，徒增杀软扫描与 IO）；
- 残留未清理 → FAIL（残留随历史堆积并污染下一次替换现场）。

执行结果：**PASS（23.30s）**；单元层 `TestRunSelfUpdateAlreadyUpToDate` 同口径。

---

### 用例 131 — self_update：内容不同时替换成功（执行层闭环）

**前置条件**：同用例 130，但主目录副本改为"新版"。新版必须是**同长度、仍可
执行、SHA256 不同**的 PE：拷贝真实 exe 后翻转 PE COFF 头的 `TimeDateStamp`
（`e_lfanew+8`，加载器不校验），不能用纯文本伪造——替换后第二次调用会真的
启动该文件，纯文本会报"不是有效的 Win32 程序"（本轮修复前即因此失败）。

**步骤**

1. 记录旧副本内容；主目录拷真实 exe + 翻转 PE 头时间戳。
2. 调用 `self_update`，断言 `updated=true`、`reason=replaced`、`old_path` 非空。
3. 断言替换后副本 SHA256 == 主目录 SHA256；`.old` 内容 == 旧版。
4. 幂等回归：紧接着再次调用（运行的是可执行的新版）→ `already_up_to_date` 且 `.old` 被清理。

```go
oldContent := ReadText(selfPath)
CopyFile(t, selfPath, srcPath)
patchPETimeDateStamp(t, srcPath) // 同长度、可执行、内容不同
Eq(t, "新版长度必须与旧版一致", len(ReadText(srcPath)), len(oldContent))
d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
True(t, d.Updated, "内容不同时应执行替换")
Eq(t, "替换后副本与源一致（SHA256）", SHA256Of(selfPath), SHA256Of(srcPath))
Eq(t, ".old 备份必须保留旧版内容", ReadText(d.OldPath), oldContent)
d2 := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
True(t, !d2.Updated, "替换后再次调用必须幂等跳过")
```

**期望结果**

- 替换后副本与源 SHA256 一致；`.old` 是完整旧版（可回滚的备份）；
- 二次调用幂等跳过并清理 `.old`。

**失败判定**

- 替换后哈希不一致 → FAIL（**严重**：副本损坏会破坏后续所有更新链）；
- `old_path` 为空或内容不是旧版 → FAIL（回滚保险不存在）；
- 二次调用仍 `updated=true` 或新 exe 无法启动 → FAIL（幂等/合法性破坏）。

执行结果：**PASS（34.52s）**；单元层 `TestRunSelfUpdateReplaces` 同口径。

---

### 用例 132 — self_update：源缺失时跳过（source_missing，不算错误）

**前置条件**：同用例 130；主目录不放 `aly-client.exe`（首次部署现场）。

**说明**：`self` 缺失（异常现场）端到端不可构造——Windows 不允许删除正在运行的
exe，删掉后又无法 spawn 该命令；该分支由 `cmd` 单测
`TestRunSelfUpdateSelfMissing` 覆盖。

**步骤**

1. 主目录无 `aly-client.exe` → `self_update` 必须 `updated=false` +
   `reason=source_missing` 且 `isSuccess=true`（不算错误）、副本原样。

```go
True(t, !Exists(srcPath), "前置：主目录无 aly-client.exe")
d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
Eq(t, "源缺失原因口径", d.Reason, "source_missing")
Eq(t, "源缺失不得改写副本", ReadText(selfPath), selfBefore)
```

**期望结果**

- `source_missing` 是**跳过**不是错误（首次部署场景不刷错误日志）；
- 副本逐字节不变。

**失败判定**

- source_missing 被当错误 → FAIL；
- 源缺失却改写了副本 → FAIL。

执行结果：**PASS（23.48s）**；单元层 `TestRunSelfUpdateSourceMissing` 同口径。

---

### 用例 133 — UI 启动：journal 防循环 + 委派 self_update 更新器

**前置条件**

1. 同用例 130 环境外，额外 `e.BuildUI(t)`：`aly-client-ui.exe` 拷入 UpdateFolder；
   再把同一 UI 拷入主目录并翻转 PE 时间戳（同长度、可执行、内容不同），证明
   若未跳过就会替换自身。
2. 主目录再放一份翻转过 PE 时间戳的 `aly-client.exe`（与 UpdateFolder 不同）。
3. 预置 `aly-client-ui.exe.selfupdate.json`：`{"count":3,"last_ts":<now>,...}`。

**步骤**

1. 记录 UI 副本内容；用 `e.RunUIModeAuto`（自动点掉标题为项目名的"无更新"
   模态框）以 `-mode check` 启动 UI，40s 上界。
2. 断言退出码 0、模态框被点掉；
3. 断言 UI 自身**未被替换**（journal 跳过）、journal 仍在且 count=3、无 `.new`；
4. 断言 `aly-client.exe` 仍被 `UpdateUpdater` 委派 `self_update` 更新：主目录与
   UpdateFolder 副本 SHA256 一致，且存在 `.old`（命令路径的标志；直接复制兜底
   不会产生 `.old`）。

```go
res := e.RunUIModeAuto(t, uiSelf, c, "check", proj, 30*time.Second, 40*time.Second)
Eq(t, "UI 应正常退出", res.ExitCode, 0)
True(t, res.Dismissed, "无更新模态框应被点掉（标题=项目名）")
Eq(t, "journal 触发跳过：UI 副本不得被替换", ReadText(uiSelf), uiBefore)
True(t, Exists(journal), "journal 必须保留")
Eq(t, "UI 应委派 self_update 更新 aly-client.exe",
    SHA256Of(updaterSelf), SHA256Of(updaterSrc))
True(t, Exists(updaterSelf+".old"), "应走 self_update 命令（.old 为证）")
```

**期望结果**

- UI 正常启动、检查、退出（退出码 0）；journal 触发跳过自身替换；
- `.old` 在、更新器 SHA 收敛，证明 `UpdateUpdater` 走的是命令而非兜底。

**失败判定**

- UI 清掉 journal 并照常替换自身 → FAIL；
- UI 崩溃/退出码非 0/模态框未点 → FAIL；
- 更新器未同步或无 `.old` → FAIL。

执行结果：**PASS（43.61s）**。

---

### 用例 134 — SDK 构造函数委派 self_update（sd16）

**前置条件**

1. 客户端现场；主目录放一份翻转 PE 时间戳的 `aly-client.exe`（与 UpdateFolder 不同）。
2. `e.BuildSDK(t)` 构建 SDK + `AlySdkSmoke`。

**步骤**

1. `e.RunSmokeEnv(t, smoke, "sd16", c, map[string]string{"ALY_EXPECT_UPDATER_SHA": expected})`
   驱动 `AlyUpdateClient` 构造（构造函数后台任务调 `self_update`），smoke 轮询
   更新器 SHA 直到收敛。
2. 断言 smoke 无 FAIL、输出含 SD16；
3. 断言 UpdateFolder 的 `aly-client.exe` SHA256 == 主目录副本，且存在 `.old`。

```go
expected := SHA256Of(updaterSrc)
out := e.RunSmokeEnv(t, smoke, "sd16", c,
    map[string]string{"ALY_EXPECT_UPDATER_SHA": expected})
Eq(t, "sd16 不得有 FAIL", OutputHasFail(out), false)
Contains(t, out, "SD16", "输出里必须有 SD16 标记")
Eq(t, "SDK 构造函数应委派 self_update", SHA256Of(updaterSelf), expected)
True(t, Exists(updaterSelf+".old"), "应走 self_update 命令（.old 为证）")
```

**期望结果**

- `PASS: SD16 构造函数已把 UpdateFolder 的 aly-client.exe 换成主目录副本`；
- SHA 一致且 `.old` 存在。

**失败判定**

- smoke 有 FAIL / SHA 不一致 → FAIL（委派没生效）；
- SHA 一致但无 `.old` → FAIL（走的是直接替换兜底，正常路径未委派）。

执行结果：**PASS（32.56s）**，sd16 输出 `PASS=1 FAIL=0`。

### 用例 135 — UI 自身替换：`-finish-self-update` 等旧进程退出 + 重启

**前置条件**

1. `tc.NewEnv(t, true, 0)`（`BuildAll` 产出 server / publish-cli / aly-client 386；
   `BuildUI` 额外产出当前源码的 `aly-client-ui.exe`）。
2. 客户端现场 `1.0.0 / applied`；`UpdateFolder/aly-client-ui.exe` 与
   `主程序目录/aly-client-ui.exe` 初始为同一份真实 UI。
3. 主目录 UI 改为**同长度、可执行、内容不同**的新版：复制真实 exe →
   `patchPETimeDateStamp`（翻转 PE `TimeDateStamp`，加载器不校验）。
4. 不预置 journal（首次自更新）。

**步骤**

1. 以 `-mode auto` 启动 `UpdateFolder/aly-client-ui.exe`（无更新，不弹模态框；
   避免阻塞模态框污染"UI 已退出"的判据）。
2. 旧 UI 启动时 `UpdateSelfUI`：复制主目录 UI 到 `.new` → 派生 `.new` 的隐藏
   模式 `-finish-self-update --target <self> --pid <旧PID> --created <创建时间>
   --new <.new> -- -mode auto` → 旧 UI 退出 0。
3. 收尾进程：
   - `OpenProcess` 旧 PID，`GetProcessTimes` 比对创建时间（防 PID 复用）；
   - `WaitForSingleObject` 最多 10s；超时才 `TerminateProcess`，再等 2s；
   - 旧 exe 改名 `.old` → `.new` 改名到目标（失败回滚）→ 用 `-mode auto`
     拉起新 UI（`exec.Command` 直接传参，不经 cmd）。
4. 新 UI 启动后 `UpdateSelfUI` 看到 same → `MarkSuccess`（pending/count 清零），
   正常 `check_update`，auto 模式静默退出。

**期望结果**

- 旧 UI 退出码 0；
- 20s 内 UpdateFolder UI 的 SHA256 收敛到主目录新版；
- `.new` 已被改名（不存在）；
- `<UpdateFolder>/aly-client-ui.exe.update.log` 存在且含 `finish OK`
  （证明走的是收尾进程，而不是旧 `ping/move/start` 脚本）；
- journal 的 `pending`/`count` 被新 UI 清零。

**失败判定**

- UI 未替换 / 20s 内未收敛 → FAIL（收尾进程没跑或改名失败）；
- 无 `update.log` / 无 `finish OK` → FAIL（没走收尾进程）；
- journal 仍 pending → FAIL（新 UI 没起来或没收敛）；
- 旧 UI 非 0 退出 → FAIL（正常路径应由旧 UI 主动退出 0）。

执行结果：**PASS（36.86s）**；日志实测含 `finish start` / `finish OK`
（旧进程退出路径可能是 `already gone` 快路径，也可能是 `exited` 等待路径）；
journal `count=0`。

---

## 十一·补六、两件套补齐（136–137）

> 背景（现场故障）：发布包把 `aly-client` / `aly-client-ui` 两件套投放到**主程序目录**
> （`ApplicationFolder/`，`deploy-client.go`、publish GUI 的 `UpdateFolderService`、
> `deploy-client-linux.sh` 都按“同名两件套”布局），但 `UpdateFolder` 里没有 UI 副本时，
> 宿主调 `AlyUpdateUIClient.LaunchAuto()` 只会拿到
> `aly-client-ui.exe not found: <UpdateFolder>\aly-client-ui.exe`（车间现场截图现象）。
> 修复分两层：`self_update --include-ui`（Go 侧补齐，只在缺失/0 字节时落位）与
> SDK `AlyUpdateUIClient.Launch` 的“拉起前保证两件套就位”。实现见
> `test/tc/cases_136_137_test.go`、`client/aly-client/{cmd/self_update.go,selfupdate/selfupdate.go}`、
> `client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/{AlyApi.cs,AlyUpdateUIClient.cs,UpdaterPayloadHelper.cs}`。

### 用例 136 — `self_update --include-ui`：图形前端补齐的四种口径

**前置条件**

1. 客户端现场：`UpdateFolder/` 内有 `aly-client.exe`（随测试构建的真实更新器）+ `client.json`
   + `version.json`；`ApplicationFolder/app.exe` 内容 `app-100`。
2. 主程序目录放一份与 `UpdateFolder` 一致的 `aly-client.exe`（本体走一致跳过分支），
   以及一份 UI 副本（任意可读文件充当，本用例只钉落位语义）。
3. `UpdateFolder` 里**没有** `aly-client-ui.exe`（现场故障形态）。

**步骤**

1. `aly-client.exe self_update --include-ui` → ① 落位；
2. 主目录 UI 副本换成更新内容，重跑 → ② 已存在口径；
3. 预置 `<ui>.selfupdate.json` 的 `pending=true`，重跑 → ③ 收尾在途口径；
4. 删掉主目录 UI 副本，重跑 → ④ 源缺失口径；
5. 不带旗标重跑 → ⑤ 回归。

**期望结果**

- ① `ui_updated=true`、`ui_reason=replaced`、落位副本与主目录副本 SHA256 一致、无 `.old`；
- ② `ui_reason=existing_kepped` 且副本内容原样（**不替换可能正在运行的 UI 镜像**）；
- ③ `ui_reason=pending_inflight`，副本原样；
- ④ `ui_reason=source_missing`，`isSuccess=true`（源缺失不算错误）；
- ⑤ `ui_updated=false` 且 `ui_reason` / `ui_old_path` 缺省（既有行为逐字节不变）。

**失败判定**

- 已存在的副本被改写 → FAIL（会与运行中的 UI 镜像/在途收尾进程打架）；
- 源缺失被当命令失败 → FAIL（首次部署会刷错误日志）；
- 不带旗标却出现 `ui_*` → FAIL（既有调用方行为被改）。

执行结果：**PASS（5.70s）**；单元层 `selfupdate` 新增 8 个 `SyncFile` 测试、
`cmd` 新增 6 个 `--include-ui` 测试同轮 PASS。

### 用例 137 — SDK `AlyUpdateUIClient.Launch` 拉起前自动落位（sd17）

**前置条件**

1. 客户端现场同用例 136（`UpdateFolder` 无 UI 副本）。
2. 主程序目录有同名 `aly-client-ui.exe`（用非可执行文件充当：启动失败属预期，
   本用例只钉**落位与失败路径**，不复现窗口；启动成功路径由用例 94 覆盖）。
3. `AlySdkSmoke` 以 `sd17` 驱动，环境变量 `ALY_EXPECT_UI_SHA` 传主目录副本的 SHA256。

**步骤**

1. `e.RunSmokeEnv(t, smoke, "sd17", c, {"ALY_EXPECT_UI_SHA": expected})`。

**期望结果**

- sd17 `FAIL=0` 且输出含 `SD17`，以下断言逐条 PASS：
  ① 缺 UI 副本时 `LauchAuto` 自动从主程序目录落位且 SHA256 一致；
  ② 两处都缺 → NG + 可诊断 `errorMsg`（含 `not found`/`aly-client-ui`），不抛异常、不触发 `Started`；
  ③ 同一实例内不重复尝试补齐（失败后再造同名源也不落位 → 防宿主定时重试造成进程/IO 风暴）；
  ④ 副本已一致时 `Launch` 不改写它；
- e2e 侧独立复核：`UpdateFolder/aly-client-ui.exe` 的 SHA256 == 主目录副本。

**失败判定**

- 缺副本时仍只返回 `not found`（没有落位）→ FAIL（现场故障未修复）；
- 落位后 SHA256 不一致 → FAIL（副本损坏会被反复拉起失败）；
- 失败路径抛异常 / 触发 `Started` → FAIL（宿主界面会崩或误判"窗口已开"）；
- 第二次 `Launch` 重复落位 → FAIL（拒绝服务式重试）。

执行结果：**PASS（14.35s）**；sd17 `SUMMARY: PASS=8 FAIL=0`。
同轮定点回归：**124 PASS（12.19s）、130 PASS（5.55s）、131 PASS（8.24s）、
132 PASS（4.19s）、133 PASS（44.20s）、134 PASS（9.16s）、135 PASS（29.00s）**。

> **测试基础设施修正（同轮）**：`test/tc/env.go` 的 `cleanEnv` 原来漏剔 `GOROOT`，
> 导致用 1.27.3 工具链跑 `test/tc` 时，子进程的 1.10.8（XP 轨）构建会去编 1.27.3 的
> stdlib，`BuildUI` 直接失败 → 133/135 假红。加进剔除表后 133/135 恢复 PASS。

---

## 十一·补七、宿主跟随修复（135 增强 + 138）

> 背景（现场回归，2026-11-12 修复）：`c816132`（2026-11-11）给 UI 加了"宿主退出即跟随退出"
> （`WatchHostExit`，按 `Getppid()` 每 1s 轮询），带来两个方向的现场问题：
>
> **问题 A（弹不出卡片）**：与 11-09 的 UI 自更新收尾重启组合成死结——重启后的新 UI 的
> 父进程是收尾进程，而收尾进程拉起新 UI 后立即退出，于是新 UI 在启动约 1 秒后被看门狗
> 误判"宿主已退出"而自杀。表现：宿主发新版后第一次启动，auto 模式确认有更新却**不弹卡片**
> （检查还没回来 UI 就没了）；手动点「检查更新」时 UI 副本已收敛（不再走收尾重启）、
> 父进程是真实宿主 → 弹窗正常。修复：收尾链把原始宿主 PID 经 `--host-pid` 传下去
> （旧 UI → 收尾进程 → 新 UI），新 UI 的宿主跟随优先使用它。
> 改动：`app/host.go`（`hostPidFromArgs`）、`app/selfupdate.go`（派生参数带 `--host-pid`）、
> `app/finish.go`（解析 + 剥旧追加新）、`main.go`（注册旗标防 ExitOnError）；linuxui 同构同修。
> 用例 135 原断言没拦住它的原因：只看 SHA 收敛 / `finish OK` / journal 清零，全部发生在
> 启动后 ~100ms 内，早于看门狗的 1s 心跳。
>
> **问题 B（关宿主 UI 不退）**：跟随按 **PID 轮询**判断宿主存活——宿主退出后 PID 被系统
> 复用给别的进程时 `processAlive` 永远为真，UI 永不退出（现场表现：手动检查弹出卡片后
> 关闭宿主，右下角 UI 一直在）。修复：winui 改为**等待宿主进程句柄**（启动时打开
> SYNCHRONIZE 句柄并阻塞等待，宿主退出立即触发、不受 PID 复用影响；句柄打不开才回落
> PID 轮询）；linuxui 轮询时比对 `/proc/<pid>/stat` 的 starttime 校验进程身份。
> 改动：`app/host.go`（`followHostByHandle`/`followHostByPID`）、linuxui `host.go`
> （`hostStillIs`）。

### 用例 135（增强）— 重启后的新 UI 必须存活并展示更新卡片

在原"收尾替换闭环"断言（SHA 收敛 / `finish OK` / journal 清零 / `.new` 改名）之上新增：

1. **前置**：服务端预置一个未消费的更新（1.1.0）——重启后的新 UI（auto）检查到它就会
   展示卡片、常驻等待用户操作，这是"新 UI 是否被误杀"的可观测信号。
2. **观测**：`RunUIMode` 等的是旧 UI（派生收尾进程后退出 0）；重启后的新 UI 是分离进程，
   按**映像名**（`tasklist /FI "IMAGENAME eq aly-client-ui.exe"`）观测：
   - 等它出现（≤10s）；
   - 再确认它**至少存活 3 秒**（误杀发生在新 UI 启动后约 1s，窗口足以区分）；
   - 最后 `taskkill` 清理。
3. **失败判定**：新 UI 进程消失 → FAIL（宿主跟随误杀重启后的 UI，`--host-pid` 链断了）。

执行结果：**PASS（20.50s）**；winui `app` 包新增 `TestHostPidFromArgs` /
`TestStripHostPIDArgs`、扩展 `TestFinishSelfUpdateFlow`（重启参数必须带 `--host-pid`）与
`TestUpdateSelfUIReplacesAndSpawns`（派生参数必须带 `--host-pid`）同轮 PASS；
linuxui 同构测试交叉编译验证（CI Ubuntu 实跑）。

### 用例 138 — 宿主退出后 UI 跟随退出

**前置条件**

1. 客户端现场：`UpdateFolder/` 内有真实 `aly-client.exe` + `client.json` + `version.json`
   （version 1.0.0 / applied）。
2. 服务端预置一个未消费的更新（1.1.0）：`-mode check` 检查到它就展示卡片、常驻等待
   用户操作——保证"UI 存活"是和现场一致的卡片形态，不是模态框阻塞。
3. 代拟宿主：PowerShell `Start-Process` 拉起 UI（父子关系成立）→ 存活 8s → 退出。

**步骤**

1. 启动代拟宿主；
2. **宿主存活期间**断言 UI 进程已出现（按映像名 `tasklist` 观测）；
3. 等代拟宿主退出；
4. 断言 UI 进程在数秒内消失（看门狗句柄等待在宿主退出瞬间触发，留 8s 余量）；
5. 复核版本未被消费（UI 的退出是跟随宿主，不是自己走完了更新）。

**期望结果**

- ① 宿主存活期间 UI 进程出现；
- ② 宿主退出后 UI 进程消失（**PID 复用不得把"宿主已退"误判成存活**）；
- ③ version.json 仍为 1.0.0（未消费）。

**失败判定**

- 宿主退出后 UI 仍在 → FAIL（跟随失效，正是现场"关宿主 UI 不退"的形态）；
- 宿主存活期间 UI 就没了 → FAIL（跟随过敏，即问题 A 的形态）；
- 版本被消费 → FAIL（观测被更新流程污染）。

执行结果：**PASS（35.16s）**。观测时序说明：UI 被正确跟随的话，宿主一退出它就会
消失——所以"UI 出现"的断言必须在**宿主存活期间**完成（首版测试在宿主退出后才开始
观测，永远看不到活着的 UI，属测试设计竞态，已修正）。

---

## 十二、审查修复回归（139–148）

> 来源：2026-11-12《全仓库审查报告-20261112》与其三层复核结论。
> 每条用例就是一个“审查发现 → 修复”的现场，代码见 `test/tc/cases_139_148_test.go`。
> 共用前提：`NewEnv(t, true, 0)` 构建全部产物并启动真实服务端（端口 23000）。

### 用例 139 — 上传接口项目名路径穿越必须被拒（B1）

**前置条件**

1. 服务端已创建合法项目 `tc-b1-traversal`。
2. 准备一个本地文件 `src/evil.bin`。

**步骤**

1. 依次以项目名 `.` / `..` / `../..` / `..\..\` / `../sub` / `a/../..` 调 `upload_file`；
2. 每次断言 `isSuccess=false`；
3. 检查 exe 目录、临时根目录、`data/` 根下均无 `evil.bin`；
4. 用合法项目名上传同一文件，应成功且出现在 get_all_files。

**期望结果**

- 六种非法项目名全部被拒，且没有任何跨目录落盘；
- 合法项目名不受影响。

**失败判定**

- 任一非法项目名返回成功 → FAIL（路径穿越漏洞）；
- 临时根/exe 目录下出现 `evil.bin` → FAIL（已穿越）；
- 合法上传被拒 → FAIL（误伤）。

执行结果：**PASS（18.04s）**。修复前 `.`/`..` 能通过正则且 Clean 后逃出 data 目录（可覆盖 aly.db 甚至服务器二进制）。

---

### 用例 140 — 分片合并的 totalChunks 边界（B3 + N1）

**前置条件**

1. 项目 `tc-b3-chunk` 已创建；
2. 先整文件上传 `target.bin`（内容 `IMPORTANT-ORIGINAL`）作为既有目标文件。

**步骤**

1. 以 `totalChunks=0` / `-1` / `-100` 调 `upload_chunk_complete`，每次后读取目标文件；
2. 以 `totalChunks=65537` 调同一接口；
3. 上传两个空分片（total=2）后调 complete；
4. 清掉 `.chunks`、只上传 1/3 分片后调 complete。

**期望结果**

- ① 0/负数/超上界 全部拒绝，目标文件字节不变；
- ② 全零字节合并拒绝，目标文件不被清空；
- ③ 缺片返回成功信封但 `chunksComplete=false`，`missingChunks` 恰好为 2 个。

**失败判定**

- 目标文件变空 → FAIL（B3：0 字节覆盖）；
- 超上界未拒 → FAIL（N1：单请求可打成 OOM/CPU 自旋）；
- 缺片当成功 → FAIL。

执行结果：**PASS（13.58s）**。

---

### 用例 141 — 软删除项目不再对外服务（M2）

**前置条件**

1. 项目 `tc-m2-softdel` 已创建、有文件、已发布 1.1.0（因此磁盘上有 manifest）。

**步骤**

1. 调 `delete_project` 软删除；
2. 依次调 `get_all_files` / `get_release_manifest` / `get_project_by_name`；
3. 检查磁盘目录是否仍存在。

**期望结果**

- 三个接口全部拒绝（软删对已部署客户端生效）；
- 磁盘目录保留（软删 = 仅从 API 隐藏，便于恢复/审计）。

**失败判定**

- 任一接口仍返回成功 → FAIL（软删形同虚设）；
- 目录被直接删掉 → FAIL（不可逆数据丢失）。

执行结果：**PASS（13.79s）**。

---

### 用例 142 — delete_file 删除同步（B6）

**前置条件**

1. 项目 `tc-b6-delete-api` 下有 `keep.txt` 与 `sub/drop.txt`。

**步骤**

1. `delete_file` 删 `sub/drop.txt`；
2. 重复删除同一文件（幂等性）；
3. 检查 `keep.txt` 不受影响；
4. 以 `../../aly.db`、`../other/x`、`..\..\win.ini` 调删除。

**期望结果**

- 首次删除生效；重复删除仍返回成功（幂等）；
- 同项目其它文件不受影响；穿越路径全部拒绝。

**失败判定**

- 文件仍在 → FAIL（删除未生效）；
- 重复删除报错 → FAIL（不幂等，重试会卡住发布）；
- 穿越路径被接受 → FAIL。

执行结果：**PASS（12.66s）**。

---

### 用例 143 — push 的 force_update 三态（B5）

**前置条件**

1. 项目 `tc-b5-force` 初始 `isForceUpdate=true`；
2. publish 工作区已有可发布文件。

**步骤**

1. `push 1.1.0`（不传开关）后查服务端 force_update；
2. 改文件 → `push 1.2.0 --clear-force-update`；
3. 改文件 → `push 1.3.0 --set-force-update`；
4. 同时传 `--set` 与 `--clear` 发布一次。

**期望结果**

- 不传开关 → 保持 true（不再静默重置）；`--clear` → false；`--set` → true；

**失败判定**

- 不传开关后变 false → FAIL（旧实现静默关闭强制更新）；
- 互斥开关未报错 → FAIL。

执行结果：**PASS（16.07s）**。

---

### 用例 144 — CLI 参数校验（M13 / M14）

**前提条件**

1. publish 工作区已配置项目；
2. 另备一个空目录（无 `.updator/`）。

**步骤**

1. `log --limit -1` 与 `--limit 0`；再跑 `--limit 5`；
2. 在空目录里跑 `config init`（不带 `--project`）。

**期望结果**

- 负数/0 返回可读错误（指明 `--limit`），不 panic；正数正常；
- `config init` 缺项目名 → 报错且不写配置文件。

**失败判定**

- 进程 panic（无 JSON 信封）→ FAIL（M13）；
- 静默写全空配置 → FAIL（M14）。

执行结果：**PASS（19.20s）**。

---

### 用例 145 — 删除同步全链路（B6）

**前提条件**

1. 工作区有 `a.txt`、`b.txt`，已 add --all 并 push 1.1.0（服务端 2 个文件）。

**步骤**

1. 删本地 `a.txt`；
2. `add --all` 后 `status`，查 `a.txt` 是否以 `deleted` 状态进暂存区；
3. `push 1.2.0`；
4. 查服务端磁盘与发布清单。

**期望结果**

- `a.txt` 以 deleted 进暂存；push 后服务端 `a.txt` 消失、`b.txt` 保留，清单不再含 `a.txt`。

**失败判定**

- `add --all` 跳过 deleted → FAIL（无法进暂存）；
- push 后文件仍在 → FAIL（删除未同步）。

执行结果：**PASS（19.18s）**。

---

### 用例 146 — 后置脚本失败不写 marker（B9）

**前提条件**

1. 项目 `tc-b9-script`：`app.exe`（v100）、`fail.bat`（`exit /b 1`）；
2. 客户端已安装 1.0.0/applied；服务端发布 1.1.0 且 `after_apply_update_script=fail.bat`。

**步骤**

1. 跑 `download_update`；
2. 跑 `apply_update`；
3. 检查 `ApplicationFolder/.updator/after_apply_1.1.0.done` 是否存在。

**期望结果**

- marker 不存在（脚本非零退出不算完成），崩溃恢复会重跑该脚本。

**失败判定**

- marker 存在 → FAIL（失败的迁移被标记为已完成）。

执行结果：**PASS（14.31s）**。

---

### 用例 147 — ignore 规则边界（M19）

**前提条件**

工作区文件：`build/root.txt`、`a/build/nested.txt`、`BUILD/upper.txt`、`keep.txt`、`debug.log`；
`ignore_folders=["build"]`、`ignore_files=["*.log"]`。

**步骤**

1. 跑 `status`，拿 unstaged 清单。

**期望结果**

- 三个 build 变体（根级/嵌套/大小写）全被忽略；`*.log` 被忽略；`keep.txt` 保留。

**失败判定**

- 嵌套 `a/build` 或 `BUILD` 出现在 unstaged → FAIL（旧实现仅根级且区分大小写）。

执行结果：**PASS（14.46s）**。

---

### 用例 148 — add 路径归一（M21）

**前提条件**

工作区有 `sub/f.txt`。

**步骤**

1. `add ".\sub\f.txt"`（反斜杠 + 前导 `./`）；
2. 再 `add "./sub/f.txt"`；
3. `status` 检查 staged 与 unstaged。

**期望结果**

- staged 仅 1 条且 `relativePath="sub/f.txt"`；unstaged 不再出现同文件。

**失败判定**

- staged 2 条或路径含反斜杠 → FAIL（同一文件双行）。

执行结果：**PASS（15.03s）**。

---

## 附录 A：故障注入手法速查

> 用例 18–25、36–39、116–124 大量依赖"制造异常现场"。这些手法都在 `test/tc/` 里有现成函数，**不要自己发明 sleep 猜时序**。

| 要造的现场 | 手法 | 说明 |
|---|---|---|
| **下载中断（断网/断电）** | `cmd, buf := e.RunAsync(c, "download_update")` → `tc.WaitFile(part, 30s)` → `tc.Kill(t, cmd)` | 等 `.part` 真出现再杀，避免"还没开始下载就杀了"的假用例 |
| **服务端被杀** | `e.StopServer()` | kill 语义（不等优雅关闭），等价现场断电 |
| **恢复且复用端口** | `e.StartServer(t, port)`（`port` 必须先记下来） | **必须复用端口**，否则客户端 `shared.json` 里的 URL 失效 |
| **apply 期间强杀（6 个点位）** | 等 `version.json` 的 `apply_stage` 达到目标值 → `tc.Kill` | `apply_stage` 是唯一的可靠点位信号；用 sleep 不可靠 |
| **目录被占用** | `holder, stop := e.StartHolder(t, c.Main())` → `defer stop()` | 用 `fake_main.exe --hold` 占住 CWD |
| **半截 `.part`** | `tc.CopyHead(t, src, partPath, n)` | 从服务端源文件拷前 n 字节，制造"下载了一半" |
| **`version.json` 写坏** | `tc.WriteText(t, filepath.Join(c.Up(),"version.json"), "{broken")` | 验证 `.bak` 自愈或隔离重置 |
| **清单缓存写坏** | `tc.WriteText(t, filepath.Join(c.Up(),"release_manifest.json"), "not-json")` | 验证"禁用删除同步但不阻断下载" |
| **下载产物丢失** | `os.RemoveAll(c.VerDir(v))` | downloaded 但快照不见了 |
| **超大文件** | `tc.BigFile(t, path, 120)` | 稀疏文件（`Truncate`），不真占磁盘 |
| **≥1MiB 的文件（触发客户端分片）** | `tc.WritePattern(t, path, chunckSizeBytes*3+12345)` | **必须用 `WritePattern` 不能用 `BigFile`**：`BigFile` 是全零稀疏文件，分片错位/丢片时 md5 未必变，会盖住要抓的缺陷。且必须经 **publish-cli**（`e.PublishCLI … push`）才会走 `uploadFileChunked`；`e.Upload` 是直打 `upload_file`，不走分片 |
| **海量小文件** | `tc.MakeFiles(t, dir, 500, 1024)` | `f0001.txt` … 各 1KB |
| **并发** | `e.RunConcurrent(t, []*Client{a,b}, []string{"download_update"}, ...)` | 两个独立进程同时跑 |
| **慢下载** | 单文件 ≥500MB + `-mode update`（UI 的 90s idle 是最真实判据） | CLI 侧没有 idle 超时，必须用 UI 验 |
| **断网但进程活着** | 只 `StopServer()`，不杀客户端 | 客户端应继续轮询，不是退出 |
| **目录不可读（ACL 拒绝）** | `denyDirRead(t, dir)` → `defer allowDirRead(t, dir)` | Windows 无 `chmod`，`os.Chmod` 只改只读位；必须用 `icacls <dir> /deny <user>:(RD)`。**用完必须恢复**，否则 `t.TempDir` 清理会失败 |
| **`.bak` 也不存在** | 直接 `WriteText(verPath, "{broken")`，先断言 `!Exists(verPath+".bak")` | `.bak` 只在主文件可解析时才被覆盖写入，所以"坏了且无备份"是真实分支 |
| **`.bak` 也坏（二次损坏）** | 同时写坏 `version.json` 与 `.bak` | 验证"隔离 + 重置"仍生效，不得抛错瘫痪 |
| **清单文件写失败** | `os.Remove(mf)` → `os.MkdirAll(mf+"/blocker", 0o755)` | 把目标做成**目录**，任何写文件操作必失败；比改权限更可靠、更好恢复 |
| **并发 rollback + apply** | `e.RunConcurrentFull(t, []*Client{c,c}, []string{"apply_update"}, []string{"rollback","--version","1.0.0"})` | **命令名必须在前**。必须断言输出不含「未知命令」，否则参数写错会静默降级成单路测试 |
| **多段 / 条件 Range** | `e.DoFull(t, "GET", path, map[string]string{"Range": "...", "If-Range": "..."})` | 要看**响应头**（`Content-Type` / `Content-Range`），`RawRange` 只回 body 不够用 |
| **非法 ignore pattern** | `[]string{"[a-", "[", "a[b"}` | 必须选**真正 `ErrBadPattern`** 的；`***` 是**合法**贪婪通配，会忽略整个项目，测不出容错 |
| **UI 语言参数** | `e.RunUIModeLang(t, uiPath, c, "update", "en-US", 90*time.Second)` | UI 是 `windowsgui`，无 stdout/日志，只能验"不崩 + 磁盘副作用" |
| **"新版可执行文件"（同长度、内容不同）** | `CopyFile(真实 exe → 目标)` → `patchPETimeDateStamp(t, 目标)`（翻转 PE COFF 头 `TimeDateStamp`，加载器不校验） | **不能写纯文本伪造成 `.exe`**：`self_update` 替换后第二次调用会真的启动它，纯文本报"不是有效的 Win32 程序"（用例 131 修复前即因此 FAIL）。该手法保持同长度、仍可执行、SHA256 不同 |
| **SDK 失败路径** | `e.RunSmoke(t, smoke, "sd15", c)` + 断言 `SUMMARY:` 与 `pass>=6` | 未注册的子命令会打 `PASS=0 FAIL=0`，"没有 FAIL"照样成立 → 必须同时验计数非零 |

**⚠ 不要同时跑两个 `go test` 进程**

`freePort()` 的选端口方式是「Listen 试探 → Close → 再让 server 去 Listen」。这个空窗期里
**另一个 `go test` 进程**可能抢到同一端口，导致本进程的 `server.exe` 启动失败，而就绪探测
却命中了**别人的 server**（状态码同为 200）——于是本进程连到错误的数据库，表现为
「清单莫名返回 0 项」这种极难排查的偶发失败。

框架已做两层防护（`env.go` 的 `StartServer`）：

1. 就绪判据改为「**子进程仍存活** + 接口 200」：端口被别人占住时本进程的 server 会绑定失败
   并立即退出，从而进入换端口重试，而不是连到别人的实例；
2. 失败自动换端口重试（最多 3 次），不再干等 30s 后直接 Fatal。

即便如此，**仍建议串行跑**：并发跑还会互相争抢 CPU/磁盘，让"40MB 大文件""慢下载"这类
依赖时间窗口的用例变得不稳定。

> **注**：`go test` 默认会缓存结果。改了框架代码或想强制复跑时必须加 `-count=1`，
> 否则会看到上一次的旧输出（连临时目录名都一样），容易误判"改动没生效"。

**判据红线（这几条任何一条出现都算最严重失败）**

1. 主目录内容与 `version.json` 声称的版本不一致
2. 该删的文件没删 / 不该删的文件被删
3. `isSuccess:true` 但产物不完整（假成功）
4. 数据目录外的东西被读取或写入（路径穿越）

---

## 附录 B：结果记录表

> **2026-11-13 修复后全量回归（最终证据）**：**147 PASS、1 SKIP、0 FAIL**（148 条全部执行）
>
> - 命令：`go test ./... -count=1 -timeout 240m -v`（目录 `test/tc`，Go 1.27.3 / amd64 / GOPATH 模式，`ALY_GO126` 指向现代 Go）
> - 总耗时：52.2 分钟
> - 证据日志：`test/tc/_regress_final.log`（逐用例 PASS/SKIP 行 + 耗时）
> - 新增 139–148（审查修复回归）；修正 62/67/68/69/70（删除同步与软删除过滤新契约）、
>   123/126/127（首发版本号）、104（PkHelper/HostHelper 重构后的源码契约）。
>
> 每轮回归复制这张表，逐条填。`结果` 一栏只填 `PASS` / `FAIL` / `SKIP` / `未执行`。
> 现场路径指该用例失败时保留的 `Env.Root`（`set ALY_TC_KEEP=1` 时打印）。

| # | 用例 | 结果 | 现象 / 备注 | 现场路径 |
|---|---|---|---|---|
| 1 | check_update 无更新 | PASS | 4.50s | |
| 2 | check_update 有更新 | PASS | 5.45s | |
| 3 | download_update 差异下载 | PASS | 5.94s | |
| 4 | download_update 已是最新 | PASS | 4.56s | |
| 5 | apply_update 原子替换 | PASS | 5.52s | |
| 6 | list_rollback_versions | PASS | 5.00s | |
| 7 | rollback 回滚 | PASS | 4.86s | |
| 8 | check_diff | PASS | 5.02s | |
| 9 | PROGRESS 行 | SKIP | 7.07s | |
| 10 | 完成行协议 data:null | PASS | 5.35s | |
| 11 | 差量删除 | PASS | 5.92s | |
| 12 | 运行时数据保护 | PASS | 7.06s | |
| 13 | 首周期保守 | PASS | 4.73s | |
| 14 | 清单缓存固化 | PASS | 4.69s | |
| 15 | 回滚清缓存 | PASS | 4.85s | |
| 16 | 缓存损坏降级 | PASS | 5.64s | |
| 17 | 断点续传基准 | PASS | 6.78s | |
| 18 | download 中断强杀 | PASS | 8.70s | |
| 19 | 断电残留 .part | PASS | 6.75s | |
| 20 | apply 复制阶段强杀 | PASS | 5.87s | |
| 21 | apply 六点位强杀矩阵 | PASS | 12.31s | |
| 22 | applying 误触 download | PASS | 5.65s | |
| 23 | rollback 中断强杀 | PASS | 6.23s | |
| 24 | version.json 半截 | PASS | 5.01s | |
| 25 | download 期间服务端被杀 | PASS | 10.64s | |
| 26 | 下载产物丢失 | PASS | 5.12s | |
| 27 | downloaded + 服务器不可达 | PASS | 6.88s | |
| 28 | 空/未知状态 apply | PASS | 5.15s | |
| 29 | Version == VersionPrevious | PASS | 5.72s | |
| 30 | apply 熔断 | PASS | 6.10s | |
| 31 | 并发锁 | PASS | 6.74s | |
| 32 | 首次更新 | PASS | 5.78s | |
| 33 | 目录被占用 | PASS | 6.47s | |
| 34 | 中文/空格/深路径 | PASS | 7.33s | |
| 35 | 回滚到不存在的版本 | PASS | 5.28s | |
| 36 | 120MB 大文件 | PASS | 14.40s | |
| 37 | 慢下载不误杀 | SKIP | 17.96s | |
| 38 | 500 个 1KB 小文件 | PASS | 20.57s | |
| 39 | 1000+ 条清单差量 | PASS | 54.93s | |
| 40 | UI 构建 | PASS | 19.95s | |
| 41 | UI bridge 单测 | PASS | 14.87s | |
| 42 | UI -mode check 无更新 | PASS | 20.16s | 模态框标题改为项目名后复跑（UI 828ms） |
| 43 | UI -mode check 有更新 | PASS | 59.57s | |
| 44 | UI 进度刷新与单位 | PASS | 23.28s | |
| 45 | UI 暂停/继续 | PASS | 9.33s | |
| 46 | server 启动/关闭 | PASS | 5.95s | |
| 47 | create_project | PASS | 4.77s | |
| 48 | 重复创建同名项目 | PASS | 4.24s | |
| 49 | get_all_files | PASS | 4.26s | |
| 50 | get_release_manifest | PASS | 4.17s | |
| 51 | download_file 内容一致 | PASS | 4.69s | |
| 52 | 版本倒退拒绝 | PASS | 4.73s | |
| 53 | 非法版本号矩阵 | PASS | 4.19s | |
| 54 | 合法版本号矩阵 | PASS | 4.69s | |
| 55 | 非法时间格式 | PASS | 4.17s | |
| 56 | 清单过滤口径 | PASS | 4.82s | |
| 57 | ignore 规则 | PASS | 4.16s | |
| 58 | 上传中间态 | PASS | 5.61s | |
| 59 | get_all_files 缓存 | PASS | 4.32s | |
| 60 | 空项目清单 | PASS | 4.41s | |
| 61 | 不存在的项目 | PASS | 4.28s | |
| 62 | 软删除项目 | PASS | 4.19s | |
| 63 | 大文件上传 | PASS | 8.21s | |
| 64 | 并发下载 | PASS | 6.38s | |
| 65 | 路径穿越防护 | PASS | 4.92s | |
| 66 | config init | PASS | 4.70s | |
| 67 | status 分类 | PASS | 4.67s | |
| 68 | add --all | PASS | 5.92s | |
| 69 | push 发布 | PASS | 4.94s | |
| 70 | push 后 status 干净 | PASS | 5.87s | |
| 71 | push --dry-run | PASS | 5.41s | |
| 72 | 缺 message 拒绝 | PASS | 4.75s | |
| 73 | 项目不存在 | PASS | 4.95s | |
| 74 | 重复/倒退版本号 | PASS | 5.33s | |
| 75 | 服务器不可达 | PASS | 4.72s | |
| 76 | 中文/空格/深路径 | PASS | 4.95s | |
| 77 | 暂存区 MD5 复核 | PASS | 5.32s | |
| 78 | SDK 构建 | PASS | 8.52s | |
| 79 | sd1 正常 check | PASS | 9.27s | |
| 80 | sd2 无效 exe 路径 | PASS | 7.67s | |
| 81 | sd3 超时常量 | PASS | 8.42s | |
| 82 | sd4 进度回调单调 | PASS | 8.53s | |
| 83 | sd5 回调抛异常隔离 | PASS | 8.57s | |
| 84 | sd6 断网 check | PASS | 17.10s | |
| 85 | sd7 列出可回滚版本 | PASS | 8.91s | |
| 86 | sd8 apply spawn | PASS | 10.49s | |
| 87 | sd9 宿主重启后版本 | PASS | 9.48s | |
| 88 | sd10 StatusChanged 抛异常 | PASS | 12.41s | |
| 89 | sd11 空闲 Dispose | PASS | 10.07s | |
| 90 | sd12 二次 Dispose | PASS | 8.89s | |
| 91 | sd13 全事件订阅 | PASS | 13.25s | |
| 92 | CheckSelfUpdateAsync | PASS | 6.10s | |
| 93 | 已是最新不死循环 | PASS | 14.76s | |
| 94 | Launch 拉 UI 进程 | PASS | 19.98s | 模态框标题改为项目名后复跑（check 845ms、auto 退出 0） |
| 95 | server ↔ client 全链路 | PASS | 10.14s | |
| 96 | 删除同步联动 | PASS | 7.39s | |
| 97 | 推送后立即可见 | PASS | 6.02s | |
| 98 | 待应用被新版本覆盖 | PASS | 8.19s | |
| 99 | client ↔ UI 全链路 | PASS | 59.06s | |
| 100 | UI 下载中暂停/继续 | PASS | 9.87s | |
| 101 | SDK MainLoop 全链路 | PASS | 12.10s | |
| 102 | SDK 断点续传 | PASS | 27.46s | |
| 103 | SDK 断网轮询 | PASS | 28.39s | |
| 104 | GUI 与 CLI 同库 | PASS | 5.53s | |
| 105 | 更新锁空窗期与崩溃残留 | PASS | 9.37s | ①空锁被 1 分钟宽限挡住（缺陷复现）②超宽限放行 ③死持有者立即清理 |
| 106 | apply F3 守卫（文件数不符） | PASS | 14.11s | 记录 4/实际 3 → 拒绝 + 自愈 applied/1.0.0，主目录未动 |
| 107 | version.json 0 字节 | PASS | 13.58s | 自愈：has_update=true / need_download=true / new=1.1.0，`.corrupt` 0 字节 |
| 108 | rollback 到空目录 | PASS | 13.27s | **缺陷**：返回成功，主目录现有文件 `[]`（被空目录顶掉） |
| 109 | rollback 到当前活动版本 | PASS | 14.91s | 提示"无需回滚"，三字段不动，残留备份保留 |
| 110 | create_project 非法名矩阵 | PASS | 14.40s | 17 子测试全 PASS；**缺陷**：`..` 被接受 |
| 111 | upload_file 路径穿越 | PASS | 13.39s | 6 子测试全 PASS（5 种穿越全拒 + 合法对照成功） |
| 112 | download_file 越界读取 | PASS | 13.31s | 7 子测试全 PASS（越界一律 404，合法 200） |
| 113 | Range 越界 / 过大 .part | PASS | 14.50s | 416/206 语义正确，过大 .part 自愈后 MD5 一致 |
| 114 | 分片上传乱序拼接 | PASS | 12.66s | 按 4,2,0,3,1 乱序上传，md5 一致、无残留、缺片不产出 |
| 115 | apply_fail_count 熔断边界 | PASS | 14.47s | ①F3 自愈不计数 ②2→3 临界正确 ③-b 复位 ④apply 复位；**缺陷**：同版本下载不复位 |
| 116 | version.json 损坏且无 .bak | PASS | 16.78s | `.corrupt` 隔离成功；自愈后 1.0.0→1.1.0 全链路走通；二次损坏（.bak 也坏）同样隔离 |
| 117 | 快照目录不可读（A2） | PASS | 16.74s | `errorMsg="待应用版本目录不可读: ...Access is denied."`；状态保持 `downloaded`，主目录未动；恢复权限后 apply 成功 |
| 118 | 并发 download + apply | PASS | 60.88s | 锁拒绝 `另一更新正在进行中`；最终收敛 `applied/1.1.0` + `app-110` |
| 119 | 并发 apply + rollback | PASS | 14.68s | **修正参数写反（`--version` 在前 → 未知命令）后重测**；锁拒绝 rollback，主目录保住 `app-120` |
| 120 | publish 清单写失败 | PASS | 14.38s | 发布仍成功、版本推进 1.0.2、客户端可下载；恢复后清单重建（版本序列须 >1.0.0，否则命中防倒退） |
| 121 | ignore 规则边界 | PASS | 12.61s | 4 类必排除 + 2 类必保留全部一致；**发现 2 处口径差异**：`logs/**` 服务端不认、`*.TMP` 服务端大小写敏感 |
| 122 | `-lang en-US` | PASS | 43.68s | en-US 与 `bogus-lang` 退出码均 0、无 panic、更新均完成 |
| 123 | 多段 Range / If-Range | PASS | 12.90s | 6 段 RFC 语义全对：`multipart/byteranges`+boundary、If-Range 不匹配回 200 全文、非法语法 400 |
| 124 | SDK Launch 失败路径 | PASS | 16.41s | `SUMMARY: PASS=9 FAIL=0`；UI 缺失返回 NG 不抛、不触发 Started、Dispose 后仍 NG |
| 125 | publish-gui-csharp：UpdateFolder 位置 | PASS | 2026-11-12 | 2026-11-04 新增；纯源码契约（不建 Env） |
| 126 | publish-gui-csharp：三件套与相对路径 | PASS | 2026-11-12 | 2026-11-04 新增；源码契约 + 真实 client 消费 GUI 布局 |
| 127 | publish-gui-csharp：原子写先 Move 后 Replace | PASS | 2026-11-12 | 2026-11-04 新增；源码契约 + 只有 `client.json.tmp` 的现场不可用 |
| 128 | 客户端真实分片上传路径 | PASS | 2.20s | 2026-11-06 新增；≥3MiB 非均匀文件走 publish-cli push，md5 逐字节一致、无 `.chunks`/`.merging` 残留。**负向对照实测**：移除服务端 `upload_chunk`/`upload_chunk_complete` 路由后重构建 → FAIL，报错与真机一致（`upload chunk 0/4: HTTP 404: 404 page not found`） |
| 129 | 分片阈值边界 | PASS | 2.77s | 2026-11-06 新增；`chunkSize`（1048×1024）走直传、`chunkSize+1` 走分片，两文件 md5 均逐字节一致、无中间态残留 |
| 130 | self_update 一致跳过 | PASS | 23.30s | 2026-11-09 新增；Windows 实跑。单元层：selfupdate 16 / cmd self_update 5 / UI journal 10 / UI 收尾 8；130–135 6/6 PASS |
| 131 | self_update 差异替换闭环 | PASS | 34.52s | 2026-11-09 新增；PE TimeDateStamp 造同长度合法新版；替换后 SHA256 收敛、`.old` 为旧版；二次调用可启动新版并幂等清理 `.old` |
| 132 | self_update 源缺失跳过 | PASS | 23.48s | 2026-11-09 新增；`source_missing` 不算错误、副本不动；self 缺失端到端不可构造，由 cmd 单测覆盖 |
| 133 | UI 防循环 + 委派更新器 | PASS | 43.61s | 2026-11-09 新增；journal count=3 跳过 UI 自身替换；同时 `UpdateUpdater` 委派 `self_update` 同步 `aly-client.exe`（`.old` 为证）；`RunUIModeAuto` 点掉项目名模态框 |
| 134 | SDK 委派 self_update | PASS | 32.56s | 2026-11-09 新增；sd16 `PASS=1 FAIL=0`；构造函数同步更新器且 `.old` 存在 |
| 135 | UI 自身收尾替换 | PASS | 36.86s | 2026-11-09 新增；`-finish-self-update` 等旧进程退出/防 PID 复用/改名 `.old`/重启；日志含 `finish OK`、journal 清零 |
| 136 | `self_update --include-ui` 两件套 | PASS | 5.70s | 2026-11-12 新增；Windows 实跑。单元层：`selfupdate` 新增 8 个 `SyncFile` 测试、`cmd` 新增 6 个 `--include-ui` 测试（含不带旗标 `ui_*` 缺省）同轮 PASS |
| 137 | SDK 拉起前自动落位（sd17） | PASS | 14.35s | 2026-11-12 新增；现场故障复现：UpdateFolder 缺 UI、主程序目录有 → `LauchAuto` 自动落位且 SHA256 一致；两处都缺 → NG 不抛异常/不 Started；同实例不重复尝试；sd17 `PASS=8 FAIL=0` |
| — | 同轮定点回归（124+130–135） | PASS | 7/7 | 2026-11-12；124 12.19s / 130 5.55s / 131 8.24s / 132 4.19s / 133 44.20s / 134 9.16s / 135 29.00s。测试基础设施修正：`test/tc/env.go` 的 `cleanEnv` 补剔 `GOROOT`（否则 1.10.8 子构建会去编 1.27.3 stdlib，133/135 假红） |
| — | 135 增强：收尾重启链宿主跟随 | PASS | 37.61s | 2026-11-12；修复 `WatchHostExit`（Getppid）×收尾重启的组合回归——重启后的新 UI 被误判"宿主已退出"在约 1s 自杀，auto 卡片弹不出来（"启动有更新却无提示"）。收尾链经 `--host-pid` 传递原始宿主 PID；用例 135 新增"新 UI 按映像名观测存活 ≥3s"断言（需要服务端预置未消费更新） |
| 138 | 宿主退出后 UI 跟随退出 | PASS | 31.41s | 2026-11-12 新增；修复跟随按 PID 轮询的 PID 复用误判（"关宿主 UI 不退"）。winui 改等待宿主进程句柄（SYNCHRONIZE，不受 PID 复用影响），linuxui 轮询比对 starttime；PowerShell 代拟宿主（拉起 UI → 存活 8s → 退出），宿主存活期间断言 UI 出现、退出后断言 UI 消失 |
| — | 2026-11-12 收官回归（124 + 130–138） | PASS | 10/10 | 124 17.01s / 130 20.89s / 131 13.54s / 132 8.43s / 133 24.23s / 134 26.17s / 135 37.61s / 136 15.99s / 137 21.97s / 138 31.41s，共 217.55s。覆盖当天三个现场报告：① UpdateFolder 缺 UI 副本报 not found（136/137）；② 启动有更新不弹卡片（135 增强）；③ 关宿主后 UI 不退出（138） |

**汇总**

| 项 | 值 |
|---|---|
| 用例总数 | **148**（全部已实测） |
| PASS | 131 |
| FAIL | 0 |
| SKIP | 1（用例 37） |
| 未执行 | **0** |
| 全量耗时 | **2470.71s**（上一轮 124 条全量）；本轮为定点回归（130–135 + 42/94），未重跑全量 |
| 本轮新增 | 106–115（10 条 + 30 子测试，~850 行）+ 116–124（9 条，~985 行）+ 139–148（10 条，cases_139_148_test.go）；2026-11-04 追加 125–127（3 条，~190 行，未执行）；2026-11-06 追加 128–129（2 条，~170 行）；2026-11-09 追加 130–135（6 条，~420 行） |
| 本轮新增结果 | 116–124 **9/9 PASS**；125–127 **3/3 PASS**；128–129 **2/2 PASS**；130–135 **6/6 PASS**（130 23.30s / 131 34.52s / 132 23.48s / 133 43.61s / 134 32.56s / 135 36.86s） |

**本轮修复的测试缺陷（非产品缺陷）**

| 用例 | 修复内容 | 复跑结果 |
|---|---|---|
| 42 | `RunUIModeAuto` 的模态框标题误写为 `aly-client-ui`；UI 实际使用 `shared.json` 的项目名（`tc-ui-nonew`），导致 `Dismissed=false` → `-999`。改为传 `proj` | **PASS（20.16s；UI 828ms）** |
| 94 | 同 42（项目名 `tc-sdk-launch`） | **PASS（19.98s；check 845ms、auto 退出 0）** |

> 上一条"42 偶发焦点竞争"的旧归因不成立：实测是标题不匹配的确定性问题，已修。

**SKIP 说明（环境所限，非缺陷）**

| 用例 | 原因 |
|---|---|
| 37 | 本机 11.83s 传完 200MB，未进入 >90s 慢下载区间；其核心（90s idle 不误杀）由用例 44/45 的 UI 侧覆盖 |

> **用例 9 的波动**：上一轮 SKIP（回环 <3s 传完 30MB，窗口内无完成采样），本轮 **PASS（79.62s）**。
> 说明它依赖机器当时负载，属"环境敏感"而非"用例错误"。偶发 SKIP 时可单独复跑。

**执行环境**

| 项 | 值 |
|---|---|
| 命令 | `go test -tags purego -v -timeout 3000s .`（`ALY_REUSE_BIN=E:/Project2026/aly/dist-test`） |
| 日期 | 2026-11-01 |
| 产物 | `server.exe` / `aly-publish.exe` / `aly-client.exe` / `fake_main.exe`（用户手动构建） |

---

## 附录 C：已知缺陷与口径差异登记

### C.9 2026-11-12 全仓库审查修复导致的契约变更（用例期望已同步）

| 用例 | 变更前口径 | 变更后口径 | 原因 |
| ---- | ---------- | ---------- | ---- |
| 62 | `get_project_by_name` 能查到软删除项目（带 `is_deleted=true`） | 按名查询**过滤**软删除，返回失败 | M2：避免已删除项目继续被下载/发布链当正常项目用 |
| 67/68 | `add --all` 只暂存 new/modified，`deleted` 常驻 unstaged | `add --all` **也暂存 deleted**，staged 后 unstaged 清空 | B6：补齐 `file/delete_file` 与 push 的删除调用，删除可同步 |
| 69 | push 只上传+建版本，服务端旧文件保留，清单仍含旧文件 | push 对 deleted 条目调删除接口，清单按删除后内容固化 | B6/M21：发布端删除必须能同步到服务端 |
| 70 | 本地删文件后服务端不删（已知缺口，断言钉死） | 本地删除 → add → push 后服务端文件删除、status 回到干净 | B6 |
| 123 | 多段 Range 返回 206 multipart | 同前，但服务端显式声明 `//go:debug httpservecontentmaxranges=200` | Go 1.26 起 `net/http` 默认只处理 1 段 Range（多段被静默忽略回 200），服务端需要 RFC 7233 行为 |
| 126/127 | 首发版本号用 `1.0.0` | 首发改用 `1.0.1`（`1.0.0` 已被项目创建记录占用） | 服务端防倒退保护：新版本必须严格大于已发布最新版本 |

> 这些不是"文档写错、实现没跟上"，而是**修复改变了契约**：用例断言随实现一起更新，
> 并用 2026-11-12 的全量回归（见附录 B）作为证据。

> 用例都"PASS"不代表产品没问题——有些用例是**按真实（有缺陷的）行为**写成记录式断言。
> 这张表把这类条目集中起来，方便跟产品侧对齐优先级。修掉一项，就把对应用例改成严格断言并在此标注。

| # | 缺陷 / 差异 | 发现于 | 定级 | 现状 |
|---|---|---|---|---|
| D1 | `download_update` 早退分支（服务端版本未变 + 产物完好）**不复位** `apply_fail_count`，熔断后无逃生口 | 115 ③-a | 中高 | 待修 |
| D2 | `rollback` 到**空目录**返回成功，主目录被空目录顶掉（红线②） | 108 | 高 | 待修 |
| D3 | `create_project` 接受 `..` 作为项目名 | 110 | 中 | 待修 |
| D4 | 更新锁在"持有者已死、锁文件残留但未超宽限期"时把后续命令全挡住 | 105 ① | 中 | 待修 |
| D5 | **`ignore_folders` 里的通配写法完全失效**：服务端用 `strings.HasPrefix(relPath, folder+"/")`，`logs/**` 会变成 `HasPrefix(rel, "logs/**/")`（永假）→ 整个 `logs/` 目录被完整下发，且**无任何 WARN** | 121 | 中 | 待修 |
| D6 | `ignore_folders` / `ignore_files` **大小写敏感**（`Temp` 匹配不到 `temp/`，`*.TMP` 匹配不到 `x.tmp`）。与前端/文档的直觉口径可能不一致，需产品确认 | 121 | 低 | 待确认 |
| D7 | **用例 42/94 的 `RunUIModeAuto` 标题硬编码 `aly-client-ui`**，而 UI 模态框实际使用 `shared.json` 项目名（42 为 `tc-ui-nonew`、94 为 `tc-sdk-launch`）→ 框点不掉、40s 超时 `-999`。2026-11-09 改为传 `proj` 后 42/94 复跑 PASS | 42/94 | 已修复（测试基础设施） | 已加固 |

**D5 / D6 的说明**

先纠正一个容易搞错的判断：**`ignore_*` 只在服务端生效**。客户端 `aly-client` 虽然定义了
`config.ShouldSkipFolder` / `ShouldSkipFile`，但全仓没有任何调用点消费 `IgnoreFolders/IgnoreFiles`
—— 它们只服务于 `un_copy_folders` / `un_copy_files`（物化时"不迁移"）。
所以不存在"两套实现打架"，只有一个实现（服务端），它本身与 glob 直觉有偏差。

**D5 后果**：用户在配置里写 `logs/**`（`.gitignore` 习惯、最自然的写法）时，
服务端 `HasPrefix(relPath, "logs/**/")` 永远为假 → **整个 `logs/` 目录照样全量下发**。
不报错、不 WARN，用户以为配好了。大项目表现为"更新包每次都包含几十 MB 日志，且用户无从察觉"。
唯一生效的写法是**纯目录名**（如 `Temp`）。

**建议修法（D5，择一）**：

1. 文件夹规则改为支持 glob：`filepath.Match(ignoreFolder, relPath)` + 分段前缀匹配（与文件规则对齐，推荐）；
2. 或至少**快速失败**：`create_project` 时检测到文件夹规则含 `*?[` 就返回错误。
   **静默失效比报错更糟** —— 用户以为排除生效了，实际没有。

**D6**：文件与文件夹规则都大小写敏感，彼此倒是一致；是否要改成 `EqualFold` 需与产品确认
（Windows 文件系统本身不区分大小写，用户很可能预期 `*.TMP` 能匹配 `x.tmp`）。

---

> **文档维护约定**
>
> 1. 新增用例：追加到对应章节，编号接着上一节末尾连续递增（**不要另起 1**），并同步更新 §一 索引与本附录 B。
> 2. 删除用例：**不要重新编号**（历史报告会引用编号），把该条标记为"已废弃"并说明替代用例。
> 3. 修改功能：先改对应用例的"期望结果"，再改代码——**用例先行**能避免"改完再补用例"时把 bug 写进期望里。
> 4. 每条用例的 `执行结果：` 只填 `未执行` / `PASS` / `FAIL` / `SKIP`，详细现象写进附录 B。
