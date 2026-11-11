# CI 复查：release v0.1.8 的失败与修复（2026-11-11）

> **更正（同日晚收尾）**：拿到 job 日志后，真实报错是
> `payload source missing: ...\staging\aly-publish-windows-amd64\aly-publish-windows-amd64.exe`
> ——即**下载产物不是脚本假设的「资产名子目录」形态**，而栈里 `Join-Path` 三参问题只是同一次执行中的次要缺陷。
> 最终修法与全绿证据见 [aly-CI-收尾-全绿-20261111.md](aly-CI-收尾-全绿-20261111.md)。

> 触发：发布 v0.1.8 后 Actions 有失败；按要求自行爬取（Actions 页面是 JS 渲染，本地无 `gh`/`curl`，
> 改用 in-world 的 REST 接口）并修到「文档同步」绿、「安装包铺装」在内联 pwsh 下本地复现通过。

## 一、爬到的实况（REST 接口）

接口：`https://api.github.com/repos/lishuangquan1987/aly/actions/runs[?conclusion=failure&per_page=N]`、
`…/actions/runs/<id>/jobs`、`…/check-runs/<jobId>/annotations`。

| run | 提交 | 结论 |
|---|---|---|
| **Release Build v0.1.8**（run 42965884178） | 3650367 | **failure** |
| 其中 `build-publish-gui-windows` / `…-linux`(x64) / `…-linux`(arm64) | 3650367 | 全部 failure，且**同一步**：第 12 步 `Stage the package payload` |
| 同 run 的 `dotnet test`、Windows 单文件发布、Linux 跨平台发布、五个 artifact 下载、客户端/服务端构建 | 3650367 | 全部 success |
| 文档同步（Deploy Website）run #8 | 3650367 | failure（另一件事） |
| 文档同步 run #10 | 07cc405（修好后） | **success** |
| 三个 SDK 工作流 | 3650367 | success |

job 的注解只有一句 `Process completed with exit code 1.`（更细的日志要 admin 权限，403）；
所以根因靠**本地复现**确定。

## 二、根因（本地一一复现）

`stage-payload.ps1` 走的是**脚本文件宿主**，它和 pwsh 内联宿主有三处差异，三处都能在本地复现出同样的栈：

1. **`Join-Path` 只吃两个位置参数**：`Join-Path $stagingRoot $Asset $File` 抛
   `PositionalParameterNotFound` —— 这正对上了三个 GUI job 同一步、1 秒内失败的现象；
2. **「赋值 = if (...) {...} else {...}」的表达式形式得到空值**（`$stagingRoot` 变空，且不报错）；
3. **`Test-Path` 不能包在 `(-not (...))` 里当条件、也不能对空值调用**（报「参数 Path 为空值」）。

复现方法：把下载产物在本地合成**两种形态**（平铺：文件直接落在 staging 根；嵌套：
`staging/<资产名>/…`），再用 CI 同款命令行调用脚本 → 得到上述三种错误；把同一段逻辑**内联**成 pwsh
命令跑 → **10 个文件全部就位、exit 0**（内联宿主没有这些限制）。

## 三、修法（已推送到 master）

- 铺装逻辑改写成 release 工作流里的**内联 pwsh 步骤**（两个 GUI job 各一份 `Stage the package payload`）：
  每个 payload 文件按「直取 → 平铺 → 递归找同名文件（只认 `Length > 0` 的真文件）」三种形态命中，
  缺一即 `throw`，并把结果树打印进 CI 注解；
- **删除** `publish/publish-gui/installer/stage-payload.ps1`；本机开发路径
  `publish/deploy-publish.bat` 用同一套文件名直接 `copy`（不再走 .ps1）；
- 网页 `publish/gui.md` 同步说明，并**去掉指向仓库文件的死链**：本地 `pnpm build` 复现出
  `Found dead link /publish/deploy-publish.bat in docs\publish\gui.md`，这正是「文档同步」run #8 的失败原因；
  修好后本地两次构建全绿（66.5s / 39.6s），CI run #10 `success`；
- 推送到 master：`39e1a9a`（死链）、`07cc405`（内联铺装），随后 Actions run #10 变绿。

## 四、还差一步

`release.yml` 只吃 `release` 事件与手动 dispatch（不吃 push），所以**修好的铺装尚未在 CI 上跑过**。
二选一即可收尾：

1. 发一个新 release（例如 `v0.1.9`）指向 master 当前提交；或
2. 在 Actions 页面手动 dispatch `release.yml`（勾 `upload_to_release=false` 只构建、不发布）。

本机没有 `gh`，也没有 curl 可执行文件，无法代为触发；复查仍可用上面那组 REST 接口。

## 五、顺带：本地全量回归（`test/tc`）的收尾

第 5 次全量跑完（3392s，约 70 条失败），失败面归三类：

1. **服务端状态残留**（绝大多数）：报「版本 1.1.0 不大于已发布的最新版本 1.1.0（防倒退保护）」「清单条目数 0」
   等——用例的服务端 `server/data/` 跨轮复用，前几轮被掐断留下了状态。**修法：清掉 `server/data/`
   （已在 .gitignore 覆盖）再重跑**；
2. **重构后陈旧的源码契约用例**：`TestC104` 断言 `ClyService.cs` 含字面量 `"aly-publish.exe"`，而它已移进
   `Helpers/HostHelper.cs`；需把断言指向新归属（与已修的 125–127 同类）；
3. 基线既有：`TestC37` SKIP、`TestC42` FAIL（UI 无更新提示后不退出）。

第 2、3 类不在本轮 CI 修复范围，已登记，下一轮全量回归前处理。
