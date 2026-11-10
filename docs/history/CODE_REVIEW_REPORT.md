

# Aly 项目代码审查报告

**审查日期**: 2026-06-12  
**审查范围**: 全部手写源代码（Go Server / Go Client / Go Publish-CLI / C# GUI）  
**审查文件数**: 68 个手写源文件  
**审核确认日期**: 2026-06-12  

---

## 一、问题统计

### 原始问题数

| 优先级 | Server | Client | Publish-CLI | C# GUI | **合计** |
|--------|--------|--------|-------------|--------|----------|
| [必须修复] | 2 | 5 | 6 | 2 | **15** |
| [建议修改] | 14 | 15 | 12 | 9 | **50** |
| [仅供参考] | 2 | 4 | 0 | 3 | **9** |
| **合计** | 18 | 24 | 18 | 14 | **74** |

### 审核后实际修改数

| 优先级 | 需修改 | 不修改 | 需确认兼容 | **合计** |
|--------|--------|--------|------------|----------|
| [必须修复] | 13 | 2 | 0 | **15** |
| [建议修改] | 32 | 13 | 5 | **50** |
| [仅供参考] | 7 | 2 | 0 | **9** |
| **合计** | **52** | **17** | **5** | **74** |

---

## 二、[必须修复] — 共 15 项

### 🔴 S-1. `project_controller.go:76` — 运算符优先级 Bug

```go
!result.IsSuccess && strings.Contains(result.ErrorMsg, "UNIQUE") || strings.Contains(result.ErrorMsg, "unique")
```

`&&` 优先级高于 `||`，实际求值为：
`(!result.IsSuccess && contains("UNIQUE")) || contains("unique")`

当 `ErrorMsg` 含小写 `"unique"` 时，即使 `result.IsSuccess == true` 也会进入错误分支，导致**成功操作被误判为失败**。

**决定**: ✅ 需要修改  
**修改方案**: 不是简单加括号，而是重构逻辑——插入前先查询 project name 是否存在，存在则直接返回"项目已存在"错误，不再依赖数据库错误信息判断。

---

### 🔴 S-2. `project_controller.go` 全文件 — 所有 API 错误响应都返回 HTTP 200

所有 Controller 函数（Create/Update/Delete/Publish）在业务失败时都用 `ctx.JSON(200, ...)` 返回错误。前端无法通过 HTTP 状态码区分成功/失败。

**决定**: ❌ 不修改（保持现状）

---

### 🔴 C-1. `config/config.go:177-189` — `splitProcessNames` 未被调用（死代码）

该函数在 `config.go` 内定义，全局无任何调用方。

**决定**: ✅ 需要修改  
**修改方案**: 删除 `splitProcessNames` 函数。

---

### 🔴 C-2. `client/http_client.go:62-80` — `GetAllProjects` 未被调用（死代码）

所有命令均使用 `FindProjectByName` / `GetProjectByName`，`GetAllProjects` 无调用方。

**决定**: ✅ 需要修改  
**修改方案**: 删除 `GetAllProjects` 函数。

---

### 🔴 C-3. `util/file.go:88-107` — `CopyDir` 未被调用（死代码）

仅 `CopyDirWithExclude` 被使用，`CopyDir` 无调用方。

**决定**: ✅ 需要修改  
**修改方案**: 删除 `CopyDir` 函数。

---

### 🔴 C-4. `cmd/download_update.go:175-177` — 文件列表为空时静默返回无输出

当 `diffFiles` 为空时，`downloaded` 保持 `false`，函数直接 `return`，不输出任何 JSON。调用方（如 GUI）会**永久阻塞**。

**决定**: ✅ 需要修改  
**修改方案**: 在 `return` 前输出完成信号：
```go
if !downloaded {
    printProgressDone()  // 或 printOutput(true, "无需更新", nil)
    return
}
```

---

### 🔴 C-5. `test/mock_server.go:398-412` — 下载接口路径穿越漏洞

`path` 参数直接传入 `http.ServeFile`，攻击者可构造 `?path=../../../../etc/passwd` 读取任意文件。

**决定**: ✅ 需要修改  
**修改方案**: 增加路径校验，确保文件在 `dataDir` 内。

---

### 🔴 P-1. `cmd/config.go:191` — `runConfigPath` 忽略 `resolveConfig` 错误

```go
cfg, _ := resolveConfig()
```

`resolveConfig` 失败时 `cfg.Path` 为空，输出无意义的相对路径。

**决定**: ✅ 需要修改  
**修改方案**: 检查并处理错误：
```go
cfg, err := resolveConfig()
if err != nil {
    outputResult(false, err.Error(), nil)
    return
}
```

---

### 🔴 P-2. `cmd/project.go:45` — `--id` 标志重复注册

`projectID` 在 `root.go:142` 已注册为 `PersistentFlags()`，`cmdProjectInfo` 第 45 行又注册为局部标志。导致 `project info --help` 中出现两个 `--id`。

**决定**: ✅ 需要修改  
**修改方案**: 废除 project id，全部传递 project name。需要全面检查所有地方还在用 project id 的，都改为使用 project name。

**关联影响**:
- 需要检查 `root.go` 中的 `PersistentFlags` 注册
- 需要检查所有使用 `projectID` 变量的命令
- 需要更新相关帮助文本和文档

---

### 🔴 P-3. `cmd/push.go:115` — 输出文本截断

```go
printHumanLn("[DRY RUN] 将创建版 %s (%d 条日", version, len(messages))
```

**决定**: ✅ 需要修改  
**修改方案**:
```go
printHumanLn("[DRY RUN] 将创建版本 %s (%d 条日志)", version, len(messages))
```

---

### 🔴 P-4. `cmd/push.go:86` — 输出文本截断

```go
fmt.Fprintln(os.Stderr, "Error: 至少要一--message")
```

**决定**: ✅ 需要修改  
**修改方案**:
```go
fmt.Fprintln(os.Stderr, "Error: 至少要一条 --message")
```

---

### 🔴 P-5. `cmd/watch.go:48` — `watchInterval` 无最小值校验

用户可传 `--interval 0` 或 `--interval -1`，导致 **CPU 死循环**。

**决定**: ✅ 需要修改  
**修改方案**: 废除 watch 命令，删除所有跟 watch 命令相关的代码。涉及文件：
- `cmd/watch.go` — 整个文件删除
- `root.go` — 移除 watch 命令注册
- 相关帮助文本和文档更新

---

### 🔴 P-6. `api/client.go:27` — `http.Client` 无超时配置

```go
hc: &http.Client{},
```

默认无超时，网络异常时请求会**无限挂起**。

**决定**: ❌ 不修改（保持现状）

---

### 🔴 G-1. `ProcessService.cs:55` — `WaitForExitAsync` 超时后仍等待 stdoutTask

超时后 `Kill()` 杀死进程，但 `stdoutTask`/`stderrTask` 可能永远不完成，导致**无限阻塞**。

**决定**: ✅ 需要修改  
**修改方案**: 超时后直接返回，或用 `Task.WhenAny` + 超时保护。

---

### 🔴 G-2. `MainWindowViewModel.cs:82` — `ResetAllCommand` 的 CanExecute 条件错误

```csharp
ResetAllCommand = new AsyncRelayCommand(ResetAllAsync, () => SelectedProject != null && this.UnstagedFiles.Count > 0);
```

"重置暂存区" 应在 `StagedFiles.Count > 0` 时可用，而非 `UnstagedFiles.Count > 0`。

**决定**: ✅ 需要修改  
**修改方案**: 改为 `StagedFiles.Count > 0`。

---

## 三、[建议修改] — 共 50 项（按模块分组）

### Server 模块（14 项）

| # | 文件:行号 | 问题 | 决定 | 备注 |
|---|-----------|------|------|------|
| 1 | `main.go:17-18` | 可用 `flag.Int()` 简化，无需 `IntVar`/`StringVar` | ❌ 不改 | |
| 2 | `main.go:33` | `fmt.Printf` 应改用 `log.Printf`，统一日志方式 | ❌ 不改 | |
| 3 | `routers.go:19` | `delete_project` 使用 POST，REST 惯例应使用 DELETE | ❌ 不改 | |
| 4 | `routers.go:14-21` | URL 用 snake_case，REST 社区通常用 kebab-case | ❌ 不改 | |
| 5 | `project_controller.go:206-323` | `GetProjectOSInfo` 和 `ServerInfo` 大量重复代码，应提取公共函数 | ✅ 改 | 提取公共函数减少重复 |
| 6 | `project_controller.go:207` | 变量名 `projectNameUrl` 应为 `projectNameURI` | ❌ 不改 | |
| 7 | `file_upload_controller.go:47` | `stringUtils.Replace` 可替换为标准库 `strings.ReplaceAll` | ❌ 不改 | |
| 8 | `file_upload_controller.go:17-21` | 未限制上传文件大小，建议配置 `r.MaxMultipartMemory` | ❌ 不改 | |
| 9 | `file_download_controller.go:101-125` | 所有错误场景都返回 404，`os.Open` 失败应返回 500 | ✅ 改 | 区分错误类型返回合适状态码 |
| 10 | `file_download_controller.go:51` | `filepath.Rel` 的错误被忽略 | ✅ 改 | 处理错误返回 |
| 11 | `project_service.go:28-156` | 所有函数使用 `context.Background()`，应接收 ctx 参数 | ✅ 改 | 支持超时和取消 |
| 12 | `project_service.go:119` | `PublishVersion` 的 time 参数未校验格式 | ✅ 改 | 添加时间格式校验 |
| 13 | `db.go:79` | `SetMaxOpenConns(1)` 限制并发，SQLite WAL 模式可支持更多 | ❌ 不改 | |
| 14 | `schema/projectchangelog.go:21` | `time` 字段用 `field.String`，应改为 `field.Time` | ❌ 不改 | |

**Server 模块小结**: 5 项修改，9 项不改

---

### Client 模块（15 项）

| # | 文件:行号 | 问题 | 决定 | 备注 |
|---|-----------|------|------|------|
| 1 | `config.go:6,68,84,197` | 使用已废弃的 `io/ioutil`，应迁移到 `io`/`os` | ⚠️ 需确认 | client 使用 go1.10，需确认迁移后是否兼容 |
| 2 | `version.go:6,69` | 同上，`io/ioutil` 已废弃 | ⚠️ 需确认 | 同上 |
| 3 | `http_client.go:7,39` | 同上 | ⚠️ 需确认 | 同上 |
| 4 | `common.go:76` | `strings.Replace` → `strings.ReplaceAll` | ❌ 不改 | |
| 5 | `process.go:1` / `process_unix.go:1` | Build tag 使用旧语法 `// +build`，Go 1.17+ 应用 `//go:build` | ⚠️ 需确认 | 需确认改动后是否兼容 go1.10 |
| 6 | `util/file.go:151-156` | 自定义 `hasPrefix` 应替换为 `strings.HasPrefix` | ❌ 不改 | |
| 7 | `util/process.go:196-203` | `IsProcessRunning` 无调用方 | ✅ 改 | 删除死代码 |
| 8 | `cmd/common.go:94,112,124` | `json.Marshal` 错误被静默丢弃 | ✅ 改 | 处理错误 |
| 9 | `cmd/apply_update.go:124-127` | `os.RemoveAll`/`os.Rename` 错误被丢弃 | ✅ 改 | 处理错误 |
| 10 | `cmd/check_diff.go:84` | 对每个差异文件计算 SHA256，性能浪费 | ✅ 改 | 改为并行计算减少耗时 |
| 11 | `util/file.go:111-148` | `LocalFileMD5Map` 串行计算 MD5，可并发优化 | ✅ 改 | 并发计算提升性能 |
| 12 | `config/version.go:20` | `version_previouse` 拼写错误，建议双字段兼容 | ✅ 改 | 直接修改拼写，不做双字段兼容 |
| 13 | `cmd/common.go:203-236` | `runScript` 执行服务器下发脚本，安全边界需加强 | ❌ 不改 | |
| 14 | `client/http_client.go:63` | URL 拼接未校验 `serverURL` 格式 | ✅ 改 | 使用安全的 URL 拼接方式 |
| 15 | `cmd/common.go:186-201` | `filepathFromSlash` 注释为 Go 1.10 兼容，Go 1.16+ 可直接用 `filepath.FromSlash` | ❌ 不改 | 必须兼容 go1.10 |

**Client 模块小结**: 7 项修改，5 项不改，3 项需确认 go1.10 兼容性

> ⚠️ **go1.10 兼容性注意**: Client 模块（#1-3, #5）的修改需要先确认 go1.10 兼容性。`io/ioutil` 在 go1.10 中存在，在 go1.16+ 中被废弃。如果要迁移，需确保目标环境（go1.10 + XP）仍能编译通过。

---

### Publish-CLI 模块（12 项）

| # | 文件:行号 | 问题 | 决定 | 备注 |
|---|-----------|------|------|------|
| 1 | `cmd/root.go:20` | `printHumanLn` 换行拼接可能产生双换行 | ❌ 不改 | |
| 2 | `cmd/root.go:64-95` | `resolveConfig()` 永不返回非 nil 错误，注释不准确 | ✅ 改 | 修正注释 |
| 3 | `cmd/config.go:210-245` | `applyArrayOp` 中 folders/files 分支逻辑完全重复 | ✅ 改 | 提取公共逻辑 |
| 4 | `cmd/project.go:13-19` | 全局共享 flag 变量跨命令复用，设计脆弱 | ✅ 改 | 改为局部变量 |
| 5 | `cmd/project.go:273` | 错误输出未使用 `outputResult`，JSON 模式不一致 | ✅ 改 | 使用 `outputResult` |
| 6 | `cmd/status.go:120` | `runDiff` 中暂存区文件未过滤 unstaged，与 `runStatus` 不一致 | ✅ 改 | 过滤 unstaged 文件 |
| 7 | `cmd/push.go:186,257` | `staging.Clear()` 返回值被忽略 | ✅ 改 | 处理返回值 |
| 8 | `cmd/push.go:219` | `pushAllForce` 变量永远为 false，应删除或添加 flag | ✅ 改 | 删除该 flag |
| 9 | `cmd/watch.go:96-100` | `scanLocal` 扫描失败后返回空数据，下轮会误判为 deleted | ✅ 改 | 全面去掉 watch 指令（与 P-5 关联） |
| 10 | `api/client.go:184-202` | `UploadFile` 将整个文件读入内存，大文件会 OOM | ✅ 改 | 改为大文件断点传输 |
| 11 | `api/client.go:197,199,201` | `WriteField`/`Close` 错误未检查 | ✅ 改 | 处理错误 |
| 12 | `staging/staging.go:67` | `os.Rename` 错误被忽略，可能导致 staging 文件损坏 | ✅ 改 | 处理错误 |

**Publish-CLI 模块小结**: 11 项修改，1 项不改

> ⚠️ **关联影响**: #9（去掉 watch 指令）与必须修复的 P-5（删除 watch 命令）相关联，需一起处理。

---

### C# GUI 模块（9 项）

| # | 文件:行号 | 问题 | 决定 | 备注 |
|---|-----------|------|------|------|
| 1 | `AppConstants.cs` | 大量未使用的常量（`CliCommands` 全部、`UiTexts` 大部分） | ✅ 改 | 删除未使用的常量 |
| 2 | `FileSizeHelper.cs` | 与 `BytesToSizeConverter` 功能重复，且无引用 | ✅ 改 | 删除整个文件 |
| 3 | `DialogHelper.cs` | 整个文件未被使用 | ✅ 改 | 删除整个文件 |
| 4 | `MainWindowViewModel.cs:124` | `RefreshAsync()` fire-and-forget，异常被静默吞掉 | ✅ 改 | 添加异常处理 |
| 5 | `MainWindowViewModel.cs:336` | `RefetchDataAsync` 在 `finally` 之后，可被其他命令打断 | ✅ 改 | 添加取消令牌保护 |
| 6 | `AddProjectDialogViewModel.cs:86` | `File.ReadAllText` 可能被 CLI 进程锁定 | ✅ 改 | 使用重试机制或共享读取 |
| 7 | `EditProjectDialogViewModel.cs:186-192` | 内部重复定义 `SharedConfig`，与 `Models/Local/SharedConfig.cs` 冲突 | ✅ 改 | 删除内部重复定义 |
| 8 | `SharedConfig.cs` | 蛇形命名不符合 C# 规范，应通过 `[JsonProperty]` 映射 | ✅ 改 | 使用 PascalCase + `[JsonProperty]` |
| 9 | `CliService.cs:108` | `AddFilesAsync` 命令拼接未转义，存在注入风险 | ✅ 改 | 使用安全的命令拼接方式 |

**C# GUI 模块小结**: 9 项全部修改

---

## 四、[仅供参考] — 共 9 项

| # | 模块 | 文件:行号 | 问题 | 决定 | 备注 |
|---|------|-----------|------|------|------|
| 1 | Server | `project_controller.go:53-58` | TOCTOU 竞态，因 SQLite 单连接实际影响极小 | ❌ 不改 | |
| 2 | Server | `models.go:19` | `interface{}` 在 Go 1.18+ 应使用 `any` | ✅ 改 | Server 使用现代 Go，可迁移 |
| 3 | Client | `cmd/common.go:136-158` | `compareVersion` 对非数字段静默为 0 | ✅ 改 | 添加校验 |
| 4 | Client | `http_client.go:18-25` | 全局 `http.Client` 连接池配置合理，但可调大 | ❌ 不改 | |
| 5 | Client | `test/mock_server.go:247-251` | `getChangeLogs` 未按项目过滤 | ✅ 改 | 去掉全部 mock |
| 6 | Client | `test/mock_server.go:505` | 嵌套两层 mux，可简化 | ✅ 改 | 去掉全部 mock |
| 7 | C# | `MainWindowViewModel.cs:30-37` | 重复的 `[NotifyCanExecuteChangedFor]` | ✅ 改 | 去重 |
| 8 | C# | `DialogService.cs:50` | ViewModel 创建在 Service 中，不利于测试 | ✅ 改 | 重构 |
| 9 | C# | `MainWindowViewModel.cs:90-95` | ObservableCollection 替换时不会触发旧集合的 CollectionChanged | ✅ 改 | 改为清空+添加 |

**仅供参考小结**: 7 项修改，2 项不改

---

## 五、架构评价

### 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    Aly 更新系统                           │
├──────────┬──────────────┬──────────────┬────────────────┤
│  Server  │   Client     │ Publish-CLI  │  Publish-GUI   │
│  (Gin)   │   (aly-update)│ (aly-publish)│  (Avalonia)   │
│  Go API  │   Go CLI     │  Go CLI      │  C# Desktop   │
├──────────┼──────────────┼──────────────┼────────────────┤
│  SQLite  │  文件系统     │  HTTP API    │  CLI Bridge    │
│  (Ent)   │  Win32 API   │  HTTP API    │  (ProcessSvc)  │
└──────────┴──────────────┴──────────────┴────────────────┘
```

### 架构优点 ✅

1. **关注点分离清晰**：Server/Client/Publish-CLI/GUI 四个组件职责明确
2. **Server 分层合理**：Controller → Service → DB，层次分明
3. **Client 原子更新设计成熟**：备份→替换→崩溃恢复→脚本执行，流程完整
4. **Publish-CLI Git-like 工作流**：status/add/push/publish，开发者熟悉
5. **C# GUI MVVM 规范**：CommunityToolkit.Mvvm，Service 层抽象，DI 使用得当

### 架构改进建议

| # | 建议 | 优先级 | 决定 | 备注 |
|---|------|--------|------|------|
| 1 | Server 所有 API 错误响应统一使用 HTTP 状态码（非全 200） | 高 | ❌ 不改 | 与 S-2 一致 |
| 2 | Server Service 层传递 `context.Context`，支持超时和取消 | 中 | ✅ 改 | 与建议修改 Server #11 一致 |
| 3 | Client `cmd/common.go` 职责过多，建议拆分为 `output.go`、`version_util.go` 等 | 中 | ✅ 改 | 重构 |
| 4 | C# `MainWindowViewModel` 过于臃肿（426 行），建议拆分为子 ViewModel | 中 | ❌ 不改 | |
| 5 | Client 和 Publish-CLI 的 `ioutil` 废弃 API 统一迁移 | 低 | ❌ 不改 | 必须兼容 go1.10，如改需硬性检查 |

**架构改进小结**: 2 项修改，3 项不改

---

## 六、命名规范检查

### Go 代码

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 导出名 PascalCase | ✅ | 大部分符合 |
| 非导出名 camelCase | ✅ | 符合 |
| 包名小写无下划线 | ✅ | 符合 |
| 接口命名 | ✅ | 无问题 |
| 个别变量名 | ⚠️ | `projectNameUrl` → `projectNameURI`（不改） |
| URL 路径风格 | ⚠️ | snake_case → 建议 kebab-case（不改） |

### C# 代码

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 公有成员 PascalCase | ✅ | 大部分符合 |
| 私有字段 _camelCase | ✅ | 符合 |
| 接口 I 前缀 | ✅ | 符合 |
| DTO 属性 | ✅ | 符合 |
| SharedConfig 蛇形命名 | ❌ | 需应用 PascalCase + `[JsonProperty]`（已确认修改） |

---

## 七、无用代码清理清单

### Go Client（4 处）

- [ ] `config/config.go:177-189` — 删除 `splitProcessNames`（C-1，已确认修改）
- [ ] `client/http_client.go:62-80` — 删除 `GetAllProjects`（C-2，已确认修改）
- [ ] `util/file.go:88-107` — 删除 `CopyDir`（C-3，已确认修改）
- [ ] `util/process.go:196-203` / `process_unix.go:31-34` — 删除 `IsProcessRunning`（Client #7，已确认修改）

### C# GUI（4 处）

- [ ] `Constants/AppConstants.cs:51-141` — 删除未使用的 `CliCommands` 和 `UiTexts` 常量（C# #1，已确认修改）
- [ ] `Helpers/FileSizeHelper.cs` — 整个文件删除（与 Converter 重复且无引用）（C# #2，已确认修改）
- [ ] `Helpers/DialogHelper.cs` — 整个文件删除（无引用）（C# #3，已确认修改）
- [ ] `EditProjectDialogViewModel.cs:186-192` — 删除内部重复定义的 `SharedConfig`（C# #7，已确认修改）

### 额外清理项

- [ ] `cmd/watch.go` — 整个文件删除（P-5 + Publish-CLI #9，废除 watch 命令）
- [ ] `test/mock_server.go` — 去掉全部 mock（仅供参考 #5, #6，已确认修改）

---

## 八、修复优先级建议

### 第一阶段：必须修复（影响正确性和安全性）

| 序号 | 编号 | 问题 | 修改方案 |
|------|------|------|----------|
| 1 | **S-1** | 运算符优先级 Bug | 插入前先查询 project name 是否存在 |
| 2 | **C-4** | download_update 静默返回 | 输出完成信号 |
| 3 | **G-1** | ProcessService 超时无限阻塞 | 超时后直接返回 |
| 4 | **G-2** | ResetAllCommand 条件错误 | 改为 StagedFiles.Count > 0 |
| 5 | **P-5** | watch 命令废除 | 删除所有 watch 相关代码 |
| 6 | **P-2** | project id 废除 | 全面改为使用 project name |
| 7 | **P-1** | runConfigPath 错误处理 | 处理 resolveConfig 错误 |
| 8 | **P-3/P-4** | 输出文本截断 | 修复截断的文本 |
| 9 | **C-1~C-3** | 死代码清理 | 删除未使用的函数 |
| 10 | **C-5** | 路径穿越漏洞 | 增加路径校验 |

### 第二阶段：建议修改（影响可维护性和健壮性）

| 序号 | 编号 | 问题 | 修改方案 |
|------|------|------|----------|
| 11 | **Server #5** | 重复代码提取 | 提取公共函数 |
| 12 | **Server #9-10** | 文件下载错误处理 | 区分错误类型 |
| 13 | **Server #11-12** | Service 层优化 | 添加 ctx 参数和时间校验 |
| 14 | **Client #7-9** | 错误处理完善 | 处理静默丢弃的错误 |
| 15 | **Client #10-11** | 性能优化 | 并行计算 MD5/SHA256 |
| 16 | **Client #12** | 拼写修正 | 修正 version_previouse |
| 17 | **Client #14** | URL 安全 | 安全的 URL 拼接方式 |
| 18 | **Publish-CLI #2-8** | 代码质量 | 注释修正、逻辑简化、错误处理 |
| 19 | **Publish-CLI #10-12** | 大文件上传 | 断点传输、错误处理 |
| 20 | **C# #1-9** | 全部修改 | 清理、安全、命名规范 |

### 第三阶段：架构优化

| 序号 | 编号 | 问题 | 修改方案 |
|------|------|------|----------|
| 21 | **架构 #2** | Server context.Context | 传递 ctx 支持超时取消 |
| 22 | **架构 #3** | Client cmd/common.go 拆分 | 拆分为多个职责文件 |
| 23 | **仅供参考 #2** | interface{} → any | Server 使用现代 Go |
| 24 | **仅供参考 #3** | compareVersion 校验 | 添加非数字段校验 |
| 25 | **仅供参考 #7-9** | C# 细节优化 | 去重、重构、ObservableCollection |

### 需确认兼容性（暂缓）

| 序号 | 编号 | 问题 | 备注 |
|------|------|------|------|
| 26 | **Client #1-3** | io/ioutil 迁移 | 需确认 go1.10 兼容性 |
| 27 | **Client #5** | Build tag 语法 | 需确认 go1.10 兼容性 |
| 28 | **架构 #5** | ioutil 统一迁移 | 不改（兼容性要求） |

---

## 九、特殊注意事项

### 1. go1.10 兼容性要求

Client 模块必须兼容 go1.10 + XP 环境。以下修改需要特别注意：
- `io/ioutil` 包在 go1.10 中存在，go1.16+ 才废弃
- `//go:build` 语法在 go1.17+ 才支持
- `strings.ReplaceAll` 在 go1.12+ 才有
- `filepath.FromSlash` 在 go1.16+ 才直接可用

**建议**: 在修改 Client 模块前，先在 go1.10 环境中验证编译通过。

### 2. watch 命令完全废除

P-5 和 Publish-CLI #9 都涉及 watch 命令，需一起处理：
- 删除 `cmd/watch.go` 整个文件
- 从 `root.go` 移除 watch 命令注册
- 更新相关帮助文本
- 清理所有 watch 相关的变量和函数

### 3. project id 全面废除

P-2 要求全面检查并移除 project id：
- 删除所有 `--id` 标志注册
- 将所有使用 `projectID` 的地方改为 `projectName`
- 更新命令行帮助文本
- 检查是否影响其他模块

---

*报告生成时间：2026-06-12T10:30:00Z*  
*审核确认时间：2026-06-12*
