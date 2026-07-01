# Aly 项目代码审查报告

> **审查日期**：2026-06-20
> **审查范围**：`server/`、`publish/publish-cli/`、`publish/publish-gui/`、`client/` 全部手写代码（约 1 万行）
> **审查方法**：静态通读 + 关键文件逐行验证，所有发现均标注 `文件:行号` 便于定位
> **约束前提**：`client/` 必须使用 Go 1.10 + GOARCH=386 + GO111MODULE=off，兼容 Windows XP（AGENTS.md §4.11，**强制约束**）

---

## 一、项目概览

Aly 是一个轻量级桌面应用自动更新系统，采用四模块架构：

| 模块 | 目录 | 技术栈 | 规模（手写） | 职责 |
|------|------|--------|--------|------|
| aly-server | `server/` | Go 1.25 + Gin + Ent + SQLite | ~1141 行 | REST API，存储项目元数据与文件 |
| aly-publish (CLI) | `publish/publish-cli/` | Go 1.25 + cobra | ~2274 行 | 命令行发布引擎（init/status/add/push） |
| aly-publish-gui | `publish/publish-gui/` | .NET 8 + Avalonia 11.3 + MVVM | ~3705 行 | 桌面 GUI，通过子进程调用 CLI |
| aly-update (client) | `client/` | **Go 1.10（兼容 XP）** | ~3043 行 | 终端更新器（检查/下载/原子替换/回滚） |

四模块通过 `.updator/shared.json` 配置目录和统一 JSON 契约 `{isSuccess, errorMsg, data}` 解耦。GUI 完全委托给 CLI，零业务逻辑重复。

---

## 二、总体评价

**架构是优秀的**：职责分明、分层清晰、契约统一。下面这些做得好的地方应当继续保持：

- ✅ 统一 JSON 输出契约 `{isSuccess, errorMsg, data}` 贯穿三端
- ✅ publish-gui 的 MVVM 分层严格（`AlyPublish.{Layer}` 命名空间规范），DI 容器注册规范
- ✅ client 严格兼容 Go 1.10：使用旧式 `// +build`、`ioutil`、GOPATH 模式，未发现 `errors.Is/As`、`%w`、`any`、`context` 等 1.13+ 语法
- ✅ client 的"原子替换 + 版本状态机（applied/downloaded/applying/applied）+ 崩溃自恢复"设计稳健（`apply_update.go`）
- ✅ `ProcessService` 并发读 stdout/stderr 防死锁、超时 Kill 后二次等待
- ✅ 无 `.Result` / `.Wait()` 阻塞、无 `new HttpClient()`、无 `Console.WriteLine`，统一使用 Serilog
- ✅ DTO（`Models/Cli`）与本地模型（`Models/Local`）分层清晰，符合 AGENTS.md §2

以下问题按优先级分类，**P0 应立即修复，P1 近期修复，P2 逐步优化，P3 视情况**。

---

## 三、P0 严重问题（建议立即修复）

### P0-1. CI 构建残留 `go get gopkg.in/yaml.v2`，违反 client "纯标准库"约束

- **位置**：`.github/workflows/release.yml:162`
- **现状**：
  ```yaml
  set GO111MODULE=off
  go get gopkg.in/yaml.v2      # 第 162 行
  go build -ldflags="-s -w" -o aly-update.exe aly/client
  ```
- **问题**：
  - `client/README.MD:170` 明确写"YAML 解析 | 已移除（统一使用 JSON）"，全 client 源码已无 `yaml` import。
  - 此 `go get` 多余，且在 GOPATH 模式下去拉取外网依赖，**污染 GOPATH**。
  - 一旦 `gopkg.in` 不可达或被下架，XP 构建直接失败，构建链脆弱。
- **影响**：构建脆弱，依赖外网，与"client 纯标准库、无第三方依赖"的核心设计相悖。
- **建议**：删除该行。client 现在是纯标准库，不需要任何 `go get`。

### P0-2. `ConfigService.SaveProjects` 静默吞异常，导致内存与磁盘不一致

- **位置**：`publish/publish-gui/src/AlyPublish/Services/ConfigService.cs:40-53`
- **现状**：`SaveProjects` 在 catch 块里只 `Log.Error`，不抛出、不返回错误码：
  ```csharp
  public void SaveProjects(List<ProjectConfig> projects)
  {
      try { /* 写盘 */ }
      catch (Exception ex) { Log.Error(ex, "保存配置失败: {Path}", ConfigFile); }
  }
  ```
- **问题**：调用方 `MainWindowViewModel.AddProjectAsync`（`:133-134`）在 `SaveProjects` 后无条件把项目加到内存 `Projects` 集合并弹"成功"。若写盘失败（磁盘满、权限拒绝、路径被锁），用户看到"添加成功"，但下次启动项目消失——**内存与磁盘数据不一致，且用户无感知**。
- **建议**：
  - 方案 A（推荐）：`SaveProjects` 改为 `bool SaveProjects(...)` 或抛异常，调用方据返回值决定是否回滚内存操作、是否提示用户。
  - 方案 B：在 catch 中弹 MessageBox 提示用户保存失败。

### P0-3. `AddProjectDialogViewModel.OnProjectPathChanged` 是无 try-catch 的 `async partial void`

- **位置**：`publish/publish-gui/src/AlyPublish/ViewModels/AddProjectDialogViewModel.cs:69-91`
- **现状**：
  ```csharp
  async partial void OnProjectPathChanged(string value)
  {
      // 无 try-catch 包裹
      var sharedPath = Path.Combine(value, ".updator", "shared.json");
      if (File.Exists(sharedPath)) { ... }
  }
  ```
- **问题**：
  - **`async void` 抛出的异常会直接终结进程**，无法被外层捕获。
  - 该方法还**标了 `async` 却没有任何 `await`**（编译器会产生 CS1998 警告，且 `async` 标记纯属误导）。
  - 同项目 `AddLocalProjectDialogViewModel` 处理同类问题（检测 `.updator/`）用了 CTS 防竞态 + 异步读文件，两个对话框**实现方式完全不一致**。
- **建议**：
  - 既然方法体内无 await，去掉 `async` 改为普通 `partial void`，即可消除崩溃风险和编译警告。
  - 仍建议包裹 try-catch + `Log.Warning`。
  - 抽取共享的 `.updator/` 检测逻辑到 Helper/Service，两个对话框复用。

### P0-4. `EditProjectDialogViewModel` 五个方法只有 try/finally 无 catch

- **位置**：`publish/publish-gui/src/AlyPublish/ViewModels/EditProjectDialogViewModel.cs:93-198`
- **涉及方法**：`SaveUrlAsync`(93)、`AddFolderAsync`(114)、`RemoveFolderAsync`(136)、`AddFileAsync`(157)、`RemoveFileAsync`(179)
- **现状**：5 个方法都是 `try { ... } finally { IsBusy = false; }`，**完全没有 catch**。
- **问题**：一旦 `ConfigSetArrayAddAsync` 等抛非预期异常（网络层、序列化），异常冒泡到 `AsyncRelayCommand`，CommunityToolkit 默认在 `IAsyncRelayCommand.ExecuteAsync` 里吞掉并记录到 `NullLogger`——**用户看不到任何反馈，`IsBusy` 虽被 finally 还原但 UI 状态可能不一致**。与同文件 `SyncIgnoreToServerAsync`(201-220，有 catch) 风格不一致。
- **建议**：补 catch，至少 `Log.Error(ex, ...)` + 用户提示 MessageBox。

### P0-5. `DialogService` 订阅 `RequestClose` 事件永不取消订阅

- **位置**：`publish/publish-gui/src/AlyPublish/Services/DialogService.cs:57, 85, 107, 127`
- **现状**：4 个对话框方法都订阅了 VM 的 `RequestClose` 事件：
  ```csharp
  vm.RequestClose += result => dialog.Close(result);
  ```
- **问题**：
  - 订阅后**从不 `-=` 取消**。
  - Dialog VM 持有 `CliService` 并可能触发异步回调。若 VM 因挂起的后台任务回调再次触发 `RequestClose`，会对已关闭的 window 调 `Close`。
  - 闭包捕获 `dialog` 实例，阻止其被 GC。
- **建议**：在 `ShowDialog` 返回后取消订阅，或改用 `WeakEventManager`。

### P0-6. `ProjectTabViewModel` 未实现 IDisposable，`_refreshCts` 不释放

- **位置**：`publish/publish-gui/src/AlyPublish/ViewModels/ProjectTabViewModel.cs:19, 92-101`
- **现状**：字段 `_refreshCts` 在 `RefreshAsync` 中每次重建（旧的 Dispose），但 `ProjectTabViewModel` **未实现 `IDisposable`**。
- **问题**：`MainWindowViewModel.CloseTab`(`:180`) 和 `EditProjectAsync`(`:210`) 直接 `Tabs.Remove(tab)` 后丢弃实例。若 Tab 关闭时正好有未完成的刷新，那个 CTS 不会被释放（CTS 内部持有计时器资源）。虽然 `.NET 8` 的 CTS finalizer 会兜底回收，但属资源管理瑕疵。
- **建议**：让 `ProjectTabViewModel` 实现 `IDisposable`，`CloseTab` 时 `_refreshCts?.Cancel(); _refreshCts?.Dispose();`。

---

## 四、P1 重要问题（近期修复）

### P1-1. 全部 ViewModel 绕过 `IDialogService` 直接调 `MessageBox.ShowAsync`

- **位置**（遍布 5 个 ViewModel）：
  - `MainWindowViewModel.cs:140, 157, 177, 212`
  - `ProjectTabViewModel.cs:115, 141, 166, 173, 195, 202, 215, 226, 233, 246, 255, 268, 279, 292-294, 310, 317, 324, 346, 351`（约 20 处）
  - `AddProjectDialogViewModel.cs:111, 128, 136, 155, 170, 177, 187, 208, 214, 220, 227, 237`
  - `CreateProjectDialogViewModel.cs:53, 59, 65, 93, 100`
  - `EditProjectDialogViewModel.cs:70, 89, 104, 108, 130, 151, 173, 194`
- **问题**：项目已有 `IDialogService` 抽象，但所有 VM 都绕过它直接调 `Ursa.Controls.MessageBox.ShowAsync`。这让 ViewModel 与 Ursa UI 库强耦合，**无法单元测试**，职责越界（VM 本应只管状态与流程，弹窗属 UI 关注点）。
- **建议**：在 `IDialogService` 增加 `ShowMessageAsync(string, string, MessageBoxIcon)`，所有 `MessageBox.ShowAsync` 改走接口。这是架构性重构，但收益大。

### P1-2. `EditProjectDialog.axaml.cs` 在 code-behind 写业务逻辑

- **位置**：`publish/publish-gui/src/AlyPublish/Views/Dialogs/EditProjectDialog.axaml.cs:13-24`
- **现状**：
  ```csharp
  private void EditUrlBtn_Click(object? sender, RoutedEventArgs e)
  {
      var vm = DataContext as ViewModels.EditProjectDialogViewModel;
      if (vm == null) return;
      vm.EditUrl = vm.ServerUrl;   // 修改 ViewModel 状态
      vm.IsEditingUrl = true;      // 业务状态切换
  }
  ```
- **问题**：code-behind 修改 ViewModel 状态字段，违反 AGENTS.md §4.1"不在 XAML code-behind 中写业务逻辑"。`EditUrl = ServerUrl; IsEditingUrl = true;` 属于"进入编辑模式"的业务状态切换。同文件 `CancelBtn_Click`(`:13-16`) 只是 `Close(null)` 尚可接受。
- **建议**：在 ViewModel 加 `StartEditUrlCommand`（`[RelayCommand]`），XAML 用 `Command="{Binding StartEditUrlCommand}"`。同项目 `MainWindow.axaml.cs`、`AddProjectDialog.axaml.cs` 等是干净的，应保持一致。

### P1-3. `CliService` 字符串拼接参数 + `AppConstants.*ArgPrefix` 常量完全不被引用

- **位置**：
  - `publish/publish-gui/src/AlyPublish/Services/CliService.cs:108, 116-124, 126-145`（全部字符串插值）
  - `publish/publish-gui/src/AlyPublish/Constants/AppConstants.cs:37-46`
- **现状**：
  - `CliService` 全部用 `$"push --version \"{version}\" --message \"{message}\""` 这种字符串拼接，仅对 `"` 做 `Replace("\"", "\\\"")`，未处理其他 shell 元字符。
  - `AppConstants` 定义了 `JsonOutputArg`、`PathArgPrefix`、`ServerArgPrefix`、`VersionArgPrefix`、`MessageArgPrefix` 等，**`CliService` 一个都没引用**，全是裸字符串 `"--server"`、`"--version"`。
- **问题**：常量定义与使用脱节（典型的"定义了不用"）；参数转义规则散落，职责分散。
- **建议**：
  - 抽取统一的参数构建器（含转义规则），所有命令复用。
  - `CliService` 引用 `AppConstants` 已定义的前缀常量。
  - `AppConstants.CliExecutableName`（`:37` 写死 `"aly-publish.exe"` 不跨平台）与 `CliService.cs:30`（`OperatingSystem.IsWindows() ? "aly-publish.exe" : "aly-publish"` 跨平台）冲突，应统一为后者逻辑。

### P1-4. `AddLocalProjectDialogViewModel` 未通过 DI 注册，手动 `new`

- **位置**：`publish/publish-gui/src/AlyPublish/Services/DialogService.cs:81`
- **现状**：`var vm = new AddLocalProjectDialogViewModel();`
- **问题**：其它三个 Dialog VM 都通过 `_sp.GetRequiredService<T>()` 获取（`:49, 101, 123`），`App.axaml.cs:44-47` 也确实**没有注册** `AddLocalProjectDialogViewModel`。风格不一致，且该 VM 未来若注入依赖会漏注册。
- **建议**：在 `App.axaml.cs` 注册（Transient），`DialogService` 统一用 `GetRequiredService`。

### P1-5. `client/cmd/rollback.go` 静默吞 `os.Rename`/`os.RemoveAll` 错误

- **位置**：`client/cmd/rollback.go:111, 113, 119, 132, 133, 143`
- **现状**：6 处 `os.RemoveAll(oldBackupTemp)` / `os.Rename(...)` 返回的 error 被直接丢弃，无日志。
- **对比**：同场景 `apply_update.go:133-143` 都有 `util.AppendToLog` 记录。`rollback.go:69` 在 `VersionStatusApplying` 崩溃恢复分支甚至硬编码 `"."` 作为日志目录，而项目其它地方用 `logDir()`（`common.go:15`）。
- **影响**：回滚失败时完全无日志可查，无法定位数据丢失原因。
- **建议**：统一改用 `logDir()` + `util.AppendToLog`，与 `apply_update.go` 行为一致。

### P1-6. `deploy-server.py` 硬编码 IP / 用户名 / 含真实姓名的远程目录

- **位置**：`deploy-server.py:4-8`
  ```python
  HOST = "10.96.115.14"
  USER = "quartecs"
  REMOTE_DIR = "/home/quartecs/lishuangquan/aly"   # 含真实姓名
  PORT = 7000
  ```
- **问题**：
  - 内网 IP、用户名、真实姓名路径硬编码，换机器/换人就失效，且**泄露内部信息**。
  - 端口不一致：`README.md:41` 用 `:2000`，`deploy-server.py:8,42,60,66` 用 `:7000`。
- **附**：`deploy-client.py:10-11, 56-57` 硬编码了两组绝对路径（`E:/Yofc/Code/OTDR/...`），同样不可移植。
- **建议**：抽到环境变量或 `.env` 文件；统一端口文档说明（服务端监听 2000，SSH 隧道/代理用 7000？需明确）。

### P1-7. `ProjectTabViewModel.RefreshAsync` 的"取消"是假机制

- **位置**：`publish/publish-gui/src/AlyPublish/ViewModels/ProjectTabViewModel.cs:92-101`
- **现状**：
  ```csharp
  _refreshCts?.Cancel();
  _refreshCts?.Dispose();
  _refreshCts = new CancellationTokenSource();
  await FetchDataAsync();   // FetchDataAsync 内部不带 token
  ```
- **问题**：创建了 `_refreshCts` 却**没把 `.Token` 传给 `FetchDataAsync`**，也没传给 `CliService.RunAsync`（后者签名里根本没有 CancellationToken）。Cancel 之后正在进行的 CLI 进程不会终止。这是**无效代码，给人"支持取消"的错觉**。
- **建议**：要么真打通 token 链路（`CliService.RunAsync` 加重载接收 token 并传给 `ProcessService`/`proc.WaitForExitAsync`），要么删掉 CTS 逻辑减少误导。

---

## 五、P2 工程化问题（逐步优化）

### P2-1. 二进制文件被 git 跟踪 + `.gitignore` 规则名不匹配

- **位置**：
  - `publish/publish-cli/publish-cli.exe`（约 9.6 MB）被 git 跟踪
  - `.gitignore:21` 写的是 `publish/publish-cli/aly-publish.exe`，**实际产物叫 `publish-cli.exe`，规则没匹配上**
- **影响**：仓库膨胀，每次构建改动 exe 都产生大 diff。
- **建议**：
  1. `.gitignore` 改为 `publish/publish-cli/*.exe`（或精确加 `publish-cli.exe`）。
  2. `git rm --cached publish/publish-cli/publish-cli.exe`。

### P2-2. IDE 配置目录与 AI 工具状态目录被跟踪

- **被跟踪的文件**（来自 `git ls-files` 与 status）：
  - `.idea/`（`AlyUpdator.iml`、`dataSources.xml`、`go.imports.xml`、`modules.xml`、`vcs.xml` 等）—— `dataSources.xml` 可能含本地数据库连接信息
  - `.vscode/launch.json`
  - `.codegraph/daemon.pid`（PID 文件，每台机器不同，入库冲突无意义）
  - `.trae/documents/*.md`、`.trae/rules/*`
  - `.reasonix/truncated-results/*.txt`
  - `reasonix.toml`（项目级配置，待确认是否应保留）
- **问题**：`.gitignore` 只忽略了 `server/.idea/`、`publish_tool_avalonia/.idea/`（第 1、6 行），**漏了根目录的 `.idea/` 和 `.vscode/`**。AI 工具目录本就更不该入库。
- **建议**：
  1. `.gitignore` 增加 `.idea/`、`.vscode/`、`.codegraph/`、`.reasonix/`、`.trae/`、`.understand-anything/`。
  2. `git rm --cached -r` 对应目录（注意 `daemon.pid` 在 status 里已显示 deleted，需处理）。

### P2-3. `.gitignore` 缺少通用规则

- **建议补充**：
  ```
  *.exe              # 通用兜底，防止新加产物漏网
  nohup.out          # deploy-server.py:60 会生成
  *.log
  update_*.log       # client 运行时生成（download_update.go:137）
  *.part             # 断点续传临时文件（http_client.go:219）
  *.old              # apply_update 备份后缀（apply_update.go:132）
  ```
  注：`.gitignore:18` 已有 `test_data/`，OK。

### P2-4. `package.json` / `package-lock.json` 是空壳

- **位置**：项目根目录
- **现状**：`package.json` 内容为 `{}`，`package-lock.json` 仅 `lockfileVersion: 3, packages: {}`。
- **问题**：项目主体是 Go + .NET，无 Node 依赖。这两个文件可能是误操作生成，会让 CI/IDE 误判为 Node 项目。
- **建议**：删除这两个文件。

### P2-5. `ProjectTabViewModel` 六个业务方法模板高度重复

- **位置**：`publish/publish-gui/src/AlyPublish/ViewModels/ProjectTabViewModel.cs`
- **涉及方法**：`AddAllAsync`(153)、`ResetAllAsync`(182)、`AddSelectedAsync`(211)、`ResetSelectedAsync`(242)、`PublishAsync`(289)、`EditProjectCmdAsync`(335)
- **问题**：每个方法都是同一模板：
  ```
  if (IsBusy) return; ... IsBusy = true;
  try { 调 CLI; 判断 IsSuccess; 弹 MessageBox; await FetchDataAsync(); }
  catch (Exception ex) { 弹 MessageBox + Log.Error + StatusMessage }
  finally { IsBusy = false; }
  ```
  - 错误文案 `r?.ErrorMsg ?? "未知错误"` 重复 10+ 次。
  - "弹 MessageBox + 设 StatusMessage + Log.Error" 的 catch 块重复 7 次。
- **建议**：抽取 `private async Task ExecuteCliActionAsync(Func<Task<CliOutput<object>?>> action, string successMsg, string actionName, bool refresh = true)` 模板方法，各命令只传委托和文案。

### P2-6. `EditProjectDialogViewModel` 四个 Add/Remove Folder/File 方法重复

- **位置**：`publish/publish-gui/src/AlyPublish/ViewModels/EditProjectDialogViewModel.cs:114-198`
- **问题**：`AddFolderAsync`/`RemoveFolderAsync`/`AddFileAsync`/`RemoveFileAsync` 结构完全一致，只差 `IgnoreFolders`/`IgnoreFiles` 和 `ignore.folders`/`ignore.files` 两个 key。
- **建议**：抽取 `private async Task ModifyIgnoreAsync(ObservableCollection<string> target, string configKey, string item, bool isAdd)`。

### P2-7. `AppConstants` 整类基本是"定义了不用"的死代码

- **位置**：`publish/publish-gui/src/AlyPublish/Constants/AppConstants.cs:29-46`
- **现状**：
  - `DefaultServerUrl`(`:38`)：全代码库**无任何引用**。`AddProjectDialog.axaml:30` 的 Watermark `"http://localhost:2000"` 是重复硬编码字符串。
  - `CliExecutableName`(`:37`)：不被引用，且只含 Windows 名（见 P1-3）。
  - `JsonOutputArg`/`PathArgPrefix`/`ServerArgPrefix`/`ProjectArgPrefix`/`IdArgPrefix`/`VersionArgPrefix`/`MessageArgPrefix`(`:39-46`)：**全部不被 `CliService` 引用**。
- **建议**：要么删除，要么真正在各处引用（推荐后者，配合 P1-3 重构）。

### P2-8. `CliService.CliPath` 每次访问都重做文件系统 IO

- **位置**：`publish/publish-gui/src/AlyPublish/Services/CliService.cs:16`
  ```csharp
  public string CliPath => FindCliDefault();
  ```
- **问题**：`CliPath` 是属性，每次访问都执行 `FindCliDefault()`（含 2 次 `File.Exists` + 1 次 `Path.GetFullPath`）。`Found`(`:25`) 访问 `CliPath`，`MainWindowViewModel` 构造时访问 2 次（`:41-42`），`RunAsync`(`:57`) 每次执行 CLI 都访问 `Found`→`CliPath`。CLI 路径运行期不变，频繁刷新无意义。
- **建议**：构造时算一次缓存到 `private readonly string _cliPath`，`CliPath`/`Found` 读字段。

### P2-9. `CliService.FindCli()` / `PublishAsync` 是死代码

- **位置**：`publish/publish-gui/src/AlyPublish/Services/CliService.cs:51`（`FindCli` 全代码库无调用方）
- **说明**：报告早期提到 `PublishAsync`(`:126`) 无调用方——经核对当前 HEAD，**`PushAsync`(`:116`) 被 `ProjectTabViewModel.PublishAsync:301` 使用，`PublishAsync` 已不存在**，此项仅保留 `FindCli` 死代码。
- **建议**：删除 `FindCli()`。

### P2-10. `FetchDataAsync` 逐条 Add 触发大量 CollectionChanged

- **位置**：`publish/publish-gui/src/AlyPublish/ViewModels/ProjectTabViewModel.cs:120-131`
  ```csharp
  UnstagedFiles.Clear(); StagedFiles.Clear();
  foreach (var f in d.Unstaged ?? []) UnstagedFiles.Add(...);
  foreach (var f in d.Staged ?? []) StagedFiles.Add(...);
  foreach (var l in logResult.Data) ChangeLogs.Add(l);
  ```
- **问题**：`ObservableCollection.Add` 每次触发 `CollectionChanged` → UI 重排。文件列表可能成百上千条，逐条 Add 性能差；且每次 Add 还触发 `:66-77` 订阅的 `NotifyCanExecuteChanged`，导致命令状态刷新 N 次。
- **建议**：构造新列表后一次性赋值 `UnstagedFiles = new ObservableCollection<FileItem>(list)`，或临时挂起通知。

### P2-11. `FileItem.StatusDisplay` 与 `FileStatus.GetDisplayText` 重复

- **位置**：
  - `publish/publish-gui/src/AlyPublish/Models/Local/FileItem.cs:28-34`（`StatusDisplay` 把 `"new"→"新增"` 等写一遍）
  - `publish/publish-gui/src/AlyPublish/Constants/AppConstants.cs:16-23`（`FileStatus.GetDisplayText` 又写一遍完全相同的映射）
- **问题**：状态文案映射存在两份实现，魔法字符串 `"new"/"modified"/"deleted"` 散落在 `ProjectTabViewModel.cs:123`、`StatusToColorConverter.cs:20-22`、`AppConstants.cs:8-11` 等多处。
- **建议**：`FileItem.StatusDisplay` 改为 `=> FileStatus.GetDisplayText(Status)`，统一一处实现；所有魔法字符串改用 `FileStatus.New/Modified/Deleted` 常量。

---

## 六、P3 小问题（视情况修复）

### P3-1. `AGENTS.md` 禁止事项编号断号

- **位置**：`AGENTS.md:60-65`
- **现状**：列表编号为 `1, 5, 8, 9, 10, 11`——中间 2/3/4/6/7 缺失，且第 11 项 `"11.❌写出不兼容的代码"` 与 10 之间无空格。
- **建议**：重新编号为连续 1-6，修正格式。

### P3-2. `client/BUILD.md` 与实际 `build.bat` 不一致

- **位置**：`client/BUILD.md:55-81` vs `client/build.bat`（17 行）
- **问题**：BUILD.md 给出的 build.bat 示例包含 `rmdir GOPATH` + `xcopy` + `copy back`，但实际 `build.bat` 只做 `go build`，没有 GOPATH 拷贝步骤。新人按文档找不到对应文件。
- **建议**：以实际 `build.bat` 为准更新文档，或把 `build.bat` 补全为文档描述的完整流程。

### P3-3. `check_diff.go` / `download_update.go` 重复"找最大 ID"逻辑

- **位置**：`client/cmd/check_diff.go:48-54`、`client/cmd/download_update.go:50-55`
- **现状**：两个文件各自实现了相同的"遍历 logs 找最大 ID"循环；而 `check_update.go:158-166` 已有 `findLatestLog`（同包可见）。
- **建议**：统一调用 `findLatestLog`。

### P3-4. `check_diff.go` SHA256 并发 goroutine 无上限

- **位置**：`client/cmd/check_diff.go:98-110`
- **现状**：每个差异文件起一个 goroutine 计算 SHA256。
- **问题**：若某次 diff 出几千个文件，会同时打开几千个文件句柄，XP 下句柄表压力大。
- **建议**：复用 `util/file.go` 里 `LocalFileMD5Map` 的 worker pool 模式（`numWorkers = runtime.NumCPU()`，Go 1.10 + XP 上可用）。

### P3-5. server/publish CI job 的 Go fallback 版本与 go.mod 不符

- **位置**：`.github/workflows/release.yml:32-43, 84-96`
- **现状**：`setup-go` 找不到指定版本时 fallback 手动下载 `go1.23.0`；而 server/publish 的 `go.mod` 要求 `go 1.25.0`。
- **对比**：client job（`:130-144`）的 fallback 正确下载 `go1.10.8`。
- **建议**：server/publish job 的 fallback 也应读 `go.mod` 版本，或固定与 `go.mod` 一致的版本。

### P3-6. `AfterApplyUpdateScript` 存在路径穿越风险（client 侧加固）

- **位置**：`client/cmd/apply_update.go:72, 196`、`client/cmd/rollback.go:71, 155`
  ```go
  runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript))
  ```
- **问题**：`AfterApplyUpdateScript` 来自服务器，`filepath.Join` 不拒绝 `..\..\xxx`。恶意/被攻破的服务器可让 client 在任意路径执行脚本（RCE / 路径穿越）。
- **建议**：
  - 在 `runScript` 入口做 `filepath.Clean` + 校验结果仍在 `fc.MainFolder` 之下。
  - 限制脚本扩展名（仅 `.bat`/`.cmd`），禁止路径含 `..`。
  - 注意：此约束必须用 Go 1.10 兼容写法（`strings.HasPrefix` 等，不要用 `errors.Is`）。

### P3-7. `client/http_client.go` `DownloadFile` 写入后缺 `f.Sync()`

- **位置**：`client/client/http_client.go:189-201`
- **现状**：`io.Copy` 后没有 `f.Sync()`，`defer f.Close()` 不保证刷盘。
- **问题**：XP + 断电场景可能留下脏数据。注意 `DownloadFileWithResume`（同文件 206+）已走 `.part` + rename 的更安全路径，但小文件（<100MB）走的是这里。
- **缓解**：后续有 MD5+SHA256 双校验（`download_update.go:146-165`）兜底会触发重下，风险有限。
- **建议**：在 `Close` 前 `f.Sync()`。

### P3-8. `App.axaml.cs` 的 `ServiceProvider` 未 Dispose

- **位置**：`publish/publish-gui/src/AlyPublish/App.axaml.cs:48, 53-57`
- **现状**：`Services = svc.BuildServiceProvider();` 返回的 `ServiceProvider` 未在 `ShutdownRequested` 里 Dispose（`:53-57` 只 flush 了日志）。
- **建议**：`ShutdownRequested` 中 `Services?.Dispose();`。

---

## 七、文档与实际偏差

| 文档位置 | 偏差内容 |
|---------|---------|
| `AGENTS.md` §七 | 称"Avalonia 12"，但 `AlyPublish.csproj` 实际引用 **Avalonia 11.3.7**、`Semi.Avalonia 11.3.7` |
| `README.md` | 提到 `publish/aly-publish/` 和 `publish/aly-publish-gui/`，实际目录已改名为 `publish/publish-cli/` 和 `publish/publish-gui/` |
| 端口约定 | `README.md:41` 用 `:2000`，`deploy-server.py:8,42,60,66` 用 `:7000`，**项目内端口不一致** |
| `AGENTS.md` §四 | 编号断号（见 P3-1） |
| 根目录 | 存在 `代码审查-不合理之处.md`（初稿）和 `代码审查-不合理之处-已审查.md`（结论）两份大文档，建议合并或仅保留"已审查"版，避免后续读者分不清权威版本 |

---

## 八、修复优先级汇总表

| 优先级 | 编号 | 问题 | 文件:行 |
|--------|------|------|---------|
| **P0** | P0-1 | CI 残留 `go get yaml.v2`，违反 client 纯标准库 | `release.yml:162` |
| **P0** | P0-2 | `SaveProjects` 吞异常 → 内存磁盘不一致 | `ConfigService.cs:40-53` |
| **P0** | P0-3 | `async partial void` 无 try-catch + 无 await | `AddProjectDialogViewModel.cs:69` |
| **P0** | P0-4 | 5 个方法 try/finally 无 catch | `EditProjectDialogViewModel.cs:93-198` |
| **P0** | P0-5 | `RequestClose` 事件订阅永不取消 | `DialogService.cs:57,85,107,127` |
| **P0** | P0-6 | Tab 关闭不释放 CTS | `ProjectTabViewModel.cs:19` |
| **P1** | P1-1 | VM 绕过 IDialogService 直接调 MessageBox | 5 个 ViewModel 全局 |
| **P1** | P1-2 | code-behind 写业务逻辑 | `EditProjectDialog.axaml.cs:18-24` |
| **P1** | P1-3 | 参数拼接 + 常量不被引用 | `CliService.cs` + `AppConstants.cs:37-46` |
| **P1** | P1-4 | VM 未 DI 注册手动 new | `DialogService.cs:81` |
| **P1** | P1-5 | rollback.go 静默吞 Rename/Remove 错误 | `rollback.go:111,113,119,132,133,143` |
| **P1** | P1-6 | 部署脚本硬编码 IP/用户名/真实姓名路径 | `deploy-server.py:4-8` |
| **P1** | P1-7 | `RefreshAsync` 假取消机制 | `ProjectTabViewModel.cs:92-101` |
| **P2** | P2-1 | 二进制入库 + gitignore 规则不匹配 | `publish-cli.exe` + `.gitignore:21` |
| **P2** | P2-2 | IDE/AI 工具目录入库 | `.idea/`、`.vscode/`、`.codegraph/` 等 |
| **P2** | P2-3 | gitignore 缺通用规则 | `.gitignore` |
| **P2** | P2-4 | 空壳 package.json | 根目录 |
| **P2** | P2-5 | 6 个业务方法模板重复 | `ProjectTabViewModel.cs` |
| **P2** | P2-6 | 4 个 Add/Remove 方法重复 | `EditProjectDialogViewModel.cs:114-198` |
| **P2** | P2-7 | AppConstants 整类死代码 | `AppConstants.cs:29-46` |
| **P2** | P2-8 | CliPath 每次访问重做 IO | `CliService.cs:16` |
| **P2** | P2-9 | `FindCli()` 死代码 | `CliService.cs:51` |
| **P2** | P2-10 | 逐条 Add 触发大量 CollectionChanged | `ProjectTabViewModel.cs:120-131` |
| **P2** | P2-11 | 状态文案映射重复 | `FileItem.cs:28-34` + `AppConstants.cs:16-23` |
| **P3** | P3-1 | AGENTS.md 编号断号 | `AGENTS.md:60-65` |
| **P3** | P3-2 | BUILD.md 与 build.bat 不一致 | `client/BUILD.md:55-81` |
| **P3** | P3-3 | 重复"找最大 ID"逻辑 | `check_diff.go:48-54` 等 |
| **P3** | P3-4 | SHA256 goroutine 无上限 | `check_diff.go:98-110` |
| **P3** | P3-5 | CI fallback Go 版本不符 | `release.yml:32-43,84-96` |
| **P3** | P3-6 | 脚本路径穿越 RCE 风险 | `apply_update.go:72,196` 等 |
| **P3** | P3-7 | DownloadFile 缺 f.Sync() | `http_client.go:189-201` |
| **P3** | P3-8 | ServiceProvider 未 Dispose | `App.axaml.cs:48` |

---

## 九、结语

Aly 项目整体工程质量较高，架构清晰、分层规范、契约统一，client 的 Go 1.10 + XP 兼容性处理到位，原子替换+回滚状态机设计稳健。本次审查发现的问题中：

- **P0（6 项）**集中在构建脆弱、资源/异常处理缺陷，建议立即修复。
- **P1（7 项）**涉及分层架构与代码组织，是中期重构重点。
- **P2（11 项）**和 **P3（8 项）**多为死代码、重复代码、文档偏差，可结合日常迭代逐步消化。

建议优先处理 P0-1（CI 残留依赖）、P0-2（配置保存静默失败）、P0-3/P0-4（异常处理缺陷）这几项影响最直接的问题。修复时注意遵守 AGENTS.md 的约束，特别是 client 侧修改必须保持 Go 1.10 兼容。
