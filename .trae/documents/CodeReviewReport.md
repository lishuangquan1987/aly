# PublishTool Avalonia 代码审查报告

> 审查日期：2026-06-01
> 审查范围：/workspace/publish_tool_avalonia/src/PublishTool/ 全部文件

---

## 🔴 Critical（严重）

### 1. ConfigService 事件回调跨线程修改 ObservableCollection

**文件**：[ConfigService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/ConfigService.cs#L98) + [MainWindowViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/MainWindowViewModel.cs#L64-L82)

**问题**：`ConfigService.Save()` / `SaveAsync()` 触发 `ProjectsChanged` 事件（L98, L113），`MainWindowViewModel.OnProjectsChanged()` 在事件回调中直接调用 `FilteredProjects.Clear()`、`FilteredProjects.Add()` 修改 `ObservableCollection`。`Save()` 可能被从任意线程调用（例如通过 `AddProject` → `Save()` 在 UI 线程上调用，但 `SaveAsync()` 是异步方法），但 `OnProjectsChanged` 中没有任何 `Dispatcher.UIThread` 保护。如果在非 UI 线程触发，将抛出 `InvalidOperationException`。

**建议**：在 `ConfigService` 中标记 `ProjectsChanged` 触发时的线程上下文，或在 `MainWindowViewModel.OnProjectsChanged` 中使用 `Dispatcher.UIThread.InvokeAsync` 包裹所有 UI 操作。

---

### 2. 版本发布流程不完整 —— 缺失 `publish_version` API 调用

**文件**：[ProjectPageViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/ProjectPageViewModel.cs#L343-L404) + [ProjectService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/ProjectService.cs)

**问题**：`PushAll` 方法仅上传文件到服务端，但服务端存在 `POST /api/project/publish_version` 端点，负责在文件上传后递增版本号并创建变更日志。该端点在整个 GUI 工具中**从未被调用**。`ProjectService` 中也完全没有 `PublishVersionAsync` 方法。

**影响**：用户通过 GUI 上传文件后，服务端项目版本号不会更新，变更日志不会自动生成。终端客户端检测不到新版本，导致"发布"功能形同虚设。

**建议**：
1. 在 `ProjectService` 中添加 `PublishVersionAsync` 方法
2. 在 `PushAll` 完成后（或作为独立步骤）调用此端点
3. 将 `NewVersion` 和 `ChangeLogText` 字段绑定到此调用

---

### 3. MainWindow.axaml 中 `<Run>` 元素配合编译绑定可能导致 StackOverflowException

**文件**：[MainWindow.axaml](file:///workspace/publish_tool_avalonia/src/PublishTool/Views/MainWindow.axaml#L279)

```xml
<Run Text="{Binding SelectedTab.Config.ServerUrl, TargetNullValue='未连接', FallbackValue='未连接'}" />
```

**问题**：Avalonia 12 中 `<Run>` 元素配合编译绑定（项目启用了 `AvaloniaUseCompiledBindingsByDefault`）是一个已知的崩溃场景，会导致 `StackOverflowException`。参见项目规则 `/workspace/.trae/rules/project_rules.md` 中的 `avalonia-run-binding` 技能描述。

**建议**：将 `<Run>` 替换为单独的 `<TextBlock>`，避免在 `<Run>` 中使用编译绑定。

---

## 🟠 High（高）

### 4. ProcessService 中 Process 对象未释放

**文件**：[ProcessService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/ProcessService.cs#L10-L76)

**问题**：`StartProcess()`（L20）、`OpenFolder()`（L34-38）、`OpenInExplorer()`（L68）都调用了 `Process.Start()`，返回的 `Process` 对象均未被 `Dispose()`。这会导致：
- 进程句柄泄漏（每个未释放的 Process 持有内核句柄）
- 如果频繁调用，可能耗尽系统句柄

**建议**：使用 `using` 语句包裹 `Process.Start()` 的返回值，或在启动后调用 `.Dispose()`（注意：Dispose 不会杀死进程，只释放句柄）。

---

### 5. MainWindowViewModel 事件订阅泄漏

**文件**：[MainWindowViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/MainWindowViewModel.cs#L61)

```csharp
_configService.ProjectsChanged += OnProjectsChanged;
```

**问题**：`MainWindowViewModel` 通过 DI 注册为 `Transient`，但在构造函数中订阅了 `ConfigService`（Singleton）的事件，且从未在任何地方取消订阅。`MainWindow` 窗口关闭时 `MainWindowViewModel` 不会被 GC 回收，因为 Singleton 的 `ConfigService` 持有对事件处理方法的强引用。

**建议**：在合适的生命周期点取消订阅（例如窗口关闭事件），或将 `MainWindowViewModel` 也改为 Singleton，或使用弱事件模式。

---

### 6. ProjectListPanel 事件订阅泄漏

**文件**：[ProjectListPanel.axaml.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Views/ProjectListPanel.axaml.cs#L12-L22)

**问题**：
- L12: `DataContextChanged += OnDataContextChanged` — 未取消订阅
- L19: `vm.FilteredProjects.CollectionChanged += (_, _) => UpdateEmptyState();` — 使用匿名 lambda 订阅，无法取消

如果 DataContext 多次变更，会累积多个 `CollectionChanged` 订阅。

**建议**：在订阅新事件前先取消旧的：`DataContextChanged -= OnDataContextChanged`；`CollectionChanged` 订阅使用命名方法并存储引用以便取消。

---

### 7. `!IsRefreshEnabled` 编译绑定取反可能失败

**文件**：[ProjectPage.axaml](file:///workspace/publish_tool_avalonia/src/PublishTool/Views/ProjectPage.axaml#L192)

```xml
IsVisible="{Binding !IsRefreshEnabled}"
```

**问题**：Avalonia 编译绑定（`AvaloniaUseCompiledBindingsByDefault=true`）对取反运算符 `!` 的支持有限，运行时可能不生效或产生异常。这是 Avalonia 12 编译绑定的已知限制。

**建议**：添加一个 `IsNotRefreshEnabled` 计算属性，或在 ViewModel 中通过 `[NotifyCanExecuteChangedFor]` 驱动一个反向属性，或使用 `IValueConverter`。

---

### 8. AddServerProjectDialogViewModel TryCreateAsync 返回值被丢弃

**文件**：[AddServerProjectDialogViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/Pages/AddServerProjectDialogViewModel.cs#L79-L119) + [MainWindowViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/MainWindowViewModel.cs#L197)

```csharp
// MainWindowViewModel.cs L197
await vm.TryCreateAsync(); // 返回值被忽略
```

**问题**：即使 `TryCreateAsync` 返回 `false`（创建失败），调用方也不做任何处理，对话框正常关闭。用户可能误以为项目已成功创建。`StatusMessage` 虽然在 ViewModel 中设置了错误消息，但对话框关闭后用户看不到。

**建议**：根据返回值决定是否关闭对话框，或在失败时保持对话框打开让用户重试。

---

### 9. PushAll 空队列无反馈

**文件**：[ProjectPageViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/ProjectPageViewModel.cs#L344-L345)

```csharp
if (UploadFiles.Count == 0) return;
```

**问题**：当上传队列为空时，方法静默返回，`StatusMessage` 不更新。用户点击"推送更新"按钮后没有任何反馈，不知道操作是被忽略了还是按钮无响应。

**建议**：添加 `StatusMessage = "上传队列为空，请先添加文件";`。

---

## 🟡 Medium（中）

### 10. 日志文件无滚动策略，可能无限增长

**文件**：[Program.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Program.cs#L14-L18)

```csharp
.WriteTo.File(Path.Combine(...))
```

**问题**：Serilog File sink 未配置滚动间隔（rollingInterval）或文件大小限制（fileSizeLimitBytes）。日志文件 `log.log` 会无限增长，长期运行可能耗尽磁盘空间。

**建议**：添加 `rollingInterval: RollingInterval.Day` 和 `retainedFileCountLimit: 7` 等配置。

---

### 11. ConfigService.Save() 同步 I/O 可能阻塞 UI 线程

**文件**：[ConfigService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/ConfigService.cs#L91-L104)

**问题**：`Save()` 方法使用同步 `File.WriteAllText`，并被 `AddProject()`、`RemoveProject()`、`UpdateProject()`、`MoveUp()`、`MoveDown()` 等方法调用。这些方法可能从 UI 线程触发（例如 `AddProjectDialogViewModel.Confirm()` 调用 `_configService.AddProject(config)` → `Save()`），造成 UI 短暂卡顿。

**建议**：统一使用 `SaveAsync()` 替代 `Save()`，或者在调用端确保所有调用都来自后台线程。特别注意 [AddProjectDialogViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/Pages/AddProjectDialogViewModel.cs#L172-L175) 中 `Confirm()` 是 `[RelayCommand]` 非异步方法。

---

### 12. 编译绑定中 `TargetNullValue` 与 `<Run>` 组合不稳

**文件**：[MainWindow.axaml](file:///workspace/publish_tool_avalonia/src/PublishTool/Views/MainWindow.axaml#L260)

```xml
Text="{Binding SelectedTab.StatusMessage, TargetNullValue='就绪'}"
```

**问题**：使用编译绑定时 `TargetNullValue` 的实现路径与反射绑定不同，在某些 Avalonia 12 版本中可能不生效。同一行还有 `{#279}` 处使用 `<Run>` 的同类问题。

**建议**：使用 Converter 或计算属性替代 `TargetNullValue`，确保兼容性。

---

### 13. EventHandler 模式偏离

**文件**：[ConfigService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/ConfigService.cs#L29)

```csharp
public event Action? ProjectsChanged;
```

**问题**：根据 .NET 设计规范，事件应使用 `EventHandler` 或 `EventHandler<T>` 模式。裸 `Action` 委托无法传递 sender 和 EventArgs，不利于调试和扩展。

**建议**：改为 `public event EventHandler? ProjectsChanged;`。

---

### 14. 文件过滤逻辑复杂且易出错

**文件**：[ProjectPageViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/ProjectPageViewModel.cs#L130-L172)

**问题**：`ApplyFileFilter()` 使用手动同步两个列表的方式实现过滤，代码超过 40 行且包含复杂的双重循环和边界判断。这种方式容易在以下场景出 bug：
- 过滤期间 CollectionChanged 事件触发导致 UI 重绘
- 并发调用导致的竞态条件
- 从后向前删除时的索引偏移

**建议**：简化逻辑，使用 `new ObservableCollection<T>(filtered)` 替代手动同步，或在操作前临时解绑 UI。

---

### 15. `UploadFileAsync` 流生命周期依赖调用方

**文件**：[FileService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/FileService.cs#L36-L62) + [ProjectPageViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/ProjectPageViewModel.cs#L359)

**问题**：`UploadFileAsync` 接收 `Stream fileStream` 参数但不对其负责生命周期。调用方使用 `await using` 确保流在 `UploadFileAsync` 调用后立即释放。如果 Flurl 异步读取流时流已被释放，会导致上传失败。

**建议**：在 `UploadFileAsync` 方法内部负责流的管理，或将流所有权转移的语义通过命名或注释明确标注。实际上 Flurl 的 `AddFile` 会在 `PostMultipartAsync` 期间同步读取流，这种模式在当前实现中是安全的——但生产环境中添加大型文件上传时可能出现竞态。

---

### 16. ProjectPageViewModel 构造函数不通过 DI

**文件**：[MainWindowViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/MainWindowViewModel.cs#L106-L107)

```csharp
var tab = new ProjectPageViewModel(config, _projectService,
    _fileService, _localFileService, _processService);
```

**问题**：`ProjectPageViewModel` 在 DI 容器中注册为 `Transient`，但实例化时绕过 DI 使用 `new`。如果将来 `ProjectPageViewModel` 的依赖增加，需要同时修改 `MainWindowViewModel` 和 DI 注册两处。

**建议**：考虑使用工厂模式或通过 `IServiceProvider` 解析带参数的实例。

---

## 🟢 Low（低）

### 17. MD5 用于文件完整性校验

**文件**：[LocalFileService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/LocalFileService.cs#L82-L95)

**问题**：MD5 是密码学上已破解的哈希算法，不适合安全场景。虽然此处仅用于文件差异检测（非安全用途），但服务端已同时提供 SHA256 哈希（`FileInfoDto` 中已建模），建议统一使用 SHA256 以增强安全性并保持前后端一致。

---

### 18. GetModifiedFiles 未处理服务端独有文件（已删除状态）

**文件**：[LocalFileService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/LocalFileService.cs#L114-L141)

**问题**：`GetModifiedFiles` 只检查本地文件是否新于或不同于服务端文件。如果服务端有文件但本地没有（例如上次拉取后被删除），该方法不会生成 `FileCompareStatus.Deleted` 条目。`FileCompareStatus` 枚举定义了 `Deleted` 状态但从未被使用。

---

### 19. AutoGenerateVersion 生成非标准版本号

**文件**：[ProjectPageViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/ProjectPageViewModel.cs#L559-L562)

```csharp
NewVersion = DateTime.Now.ToString("yyyyMMdd-HHmm");
```

**问题**：生成格式如 `20260601-1430`，不符合 SemVer 规范（应为 `MAJOR.MINOR.PATCH`）。如果服务端期望 SemVer 格式进行版本比较，会导致排序错误。

---

### 20. AddProjectDialogViewModel 参数化构造函数设计

**文件**：[AddProjectDialogViewModel.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewModels/Pages/AddProjectDialogViewModel.cs#L22-L26, L99-L103)

**问题**：同时提供无参构造函数（供 XAML 预览）和 DI 构造函数。无参构造函数中 `_projectService` 和 `_configService` 默认为 `null`，如果 View 在不设置 DataContext 的情况下渲染（XAML 设计器），调用 `FetchProjectsCommand` 会抛出 `NullReferenceException`。

**建议**：将无参构造函数标记为设计时专用，或在命令中增加 null 检查。

---

### 21. 缺少 `GetAllProjectsAsync` 超时设置

**文件**：[ProjectService.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/Services/ProjectService.cs#L16-L18)

**问题**：`ProjectService` 中 `GetAllProjectsAsync`、`CreateProjectAsync`、`UpdateProjectAsync` 等方法均未设置 `WithTimeout()`，而 `FileService` 中的方法都配置了超时（60s/300s）。不一致的超时策略可能导致项目列表接口在网络故障时无限挂起。

---

### 22. ViewLocator 字符串替换解析不健壮

**文件**：[ViewLocator.cs](file:///workspace/publish_tool_avalonia/src/PublishTool/ViewLocator.cs#L13-L16)

**问题**：使用简单的字符串替换从 ViewModel 类型名推断 View 类型名，且未使用缓存。每次创建 View 都进行 `Type.GetType()` 反射调用。对于频繁创建销毁的 ViewModel（如 Tab 页），这是性能瓶颈。

---

## ✅ 已验证通过（无问题）

| 项目 | 状态 |
|------|------|
| API 路径与服务端 Router 一致性 | ✅ 全部匹配 |
| CommonResponse JSON 字段命名 | ✅ `isSuccess`/`errorMsg`/`data` 双端一致 |
| ProjectDto 反序列化字段 (snake_case) | ✅ 与服务端 ent 生成 JSON tag 一致 |
| CreateProjectDto / UpdateProjectDto 序列化字段 (camelCase) | ✅ 与服务端控制器结构体一致 |
| FileInfoDto 字段命名 | ✅ 全部 camelCase，与服务端一致 |
| ProjectChangeLogDto 混合字段 | ✅ 与服务端 ent 生成一致 |
| ServerOsInfoDto 字段 | ✅ 与服务端 models.ServerOSInfo 完全一致 |
| View code-behind 无业务逻辑 | ✅ 全部仅 `InitializeComponent()` |
| IDisposable 实现 | ✅ ProjectPageViewModel 正确释放 _cts |
| 取消令牌传播 | ✅ RefreshStatus/PushAll/DownloadAll 正确使用 _cts |

---

## 📊 统计总览

| 严重度 | 数量 | 涉及文件 |
|--------|------|----------|
| 🔴 Critical | 3 | MainWindow.axaml, ProjectPageViewModel.cs, MainWindowViewModel.cs, ConfigService.cs |
| 🟠 High | 6 | ProcessService.cs, MainWindowViewModel.cs, ProjectListPanel.axaml.cs, MainWindow.axaml, AddServerProjectDialogViewModel.cs, ProjectPageViewModel.cs |
| 🟡 Medium | 7 | Program.cs, ConfigService.cs, FileService.cs, ProjectPageViewModel.cs, MainWindow.axaml, MainWindowViewModel.cs |
| 🟢 Low | 5 | LocalFileService.cs, ProjectPageViewModel.cs, AddProjectDialogViewModel.cs, ProjectService.cs, ViewLocator.cs |
| **合计** | **21** | |