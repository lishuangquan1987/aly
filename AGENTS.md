# ClientUpdator 项目规则

> 所有新增/修改代码必须遵循以下规则。

***

## 〇、项目概述与职责边界

本项目包含三个子项目，各自职责如下：

| 子项目 | 目录 | 职责 | 开发状态 |
| ------ | ---- | ---- | -------- |
| **server** | `server/` | 服务端（Go + Gin + Ent + SQLite），提供项目管理和文件管理的 REST API | 已开发 |
| **publish_tool_avalonia** | `publish_tool_avalonia/` | **版本发布客户端**（.NET 8 + Avalonia 12），主要功能是推送文件、新版本到服务端，管理发布流程 | 已开发 |
| **client** | `client/` | **更新客户端**（暂未开发），主要功能是发现新版本、下载新版本、替换文件实现自动更新，也支持全量下载整个压缩包 | 未开发 |

**关键约束**：

- `publish_tool_avalonia` 是面向**发布者**的工具，负责将文件上传到服务端、管理版本、触发发布
- `client` 是面向**终端用户**的更新程序，负责从服务端检测新版本、下载更新包、执行文件替换
- `server` 是两者共同依赖的后端服务，API 路径和数据结构必须与两端保持一致
- 修改 `server` 的 API 时，必须同步确认 `publish_tool_avalonia` 的 DTO 和 Service 层是否需要调整
- 开发 `client` 时，应复用 `server` 已有的 API 端点，避免为 client 单独新增接口（除非必要）

***

## 一、项目结构规则

### 1.1 目录组织

```
publish_tool_avalonia/
├── src/
│   ── PublishTool/           # 主项目
│       ├── Models/             # DTO 数据模型（与服务端 JSON 一一对应）
│       │   └── Local/          # 本地业务模型（ObservableObject 子类）
│       ├── Services/           # 服务层（HTTP API + 本地服务）
│       ├── ViewModels/         # ViewModel 层（CommunityToolkit.Mvvm）
│       ├── Views/              # 视图层
│       │   ├── Controls/       # 可复用的 UserControl 控件
│       │   └── Dialogs/        # 对话框窗口
│       ├── Converters/         # IValueConverter 实现
│       └── Helpers/            # 静态工具类
└── PublishTool.Tests/          # 单元测试
```

### 1.2 分层依赖规则

- **View → ViewModel → Service → Model**（单向依赖，禁止反向引用）
- View 只通过 DataContext 绑定 ViewModel，不直接访问 Service
- ViewModel 只引用 Service 和 Model，不引用 View 类型
- Service 之间可以相互调用（通过 DI 注入）

***

## 二、命名规则

### 2.1 文件命名

| 类型                 | 规则                                 | 示例                                     |
| ------------------ | ---------------------------------- | -------------------------------------- |
| DTO 模型             | `{EntityName}Dto.cs`               | `ProjectDto.cs`, `FileInfoDto.cs`      |
| 本地模型               | `{EntityName}.cs`                  | `ProjectConfig.cs`, `LocalFileItem.cs` |
| Service            | `{Domain}Service.cs`               | `ProjectService.cs`, `FileService.cs`  |
| ViewModel          | `{Page}ViewModel.cs`               | `MainWindowViewModel.cs`               |
| View (UserControl) | `{ControlName}.axaml + .axaml.cs`  | `ProjectListPanel.axaml`               |
| Dialog             | `{Action}Dialog.axaml + .axaml.cs` | `AddProjectDialog.axaml`               |
| Converter          | `{Source}To{Target}Converter.cs`   | `BytesToSizeConverter.cs`              |
| Helper             | `{Function}Helper.cs`              | `Md5Helper.cs`                         |

### 2.2 代码命名

| 元素          | 规则                    | 示例                         |
| ----------- | --------------------- | -------------------------- |
| 命名空间        | `PublishTool.{Layer}` | `PublishTool.Services`     |
| 类           | PascalCase            | `ProjectPageViewModel`     |
| 接口          | `I` 前缀 + PascalCase   | `IProjectApi`              |
| 属性          | PascalCase            | `ServerUrl`                |
| 字段（private） | `_camelCase`          | `_configService`           |
| 方法参数        | camelCase             | `serverUrl`                |
| 本地变量        | camelCase             | `filteredProjects`         |
| 常量          | PascalCase            | `DefaultPort`              |
| 命令方法        | `{Action}` 后缀         | `RefreshStatus`, `PushAll` |

### 2.3 XAML 命名

- `x:Name` 使用 PascalCase（如 `ProjectListBox`, `SearchTextBox`）
- 绑定路径使用 PascalCase（如 `{Binding ServerUrl}`）
- 资源键使用 PascalCase（如 `{DynamicResource WindowBackgroundBrush}`）

***

## 三、架构规则（MVVM）

### 3.1 ViewModel 基类

所有 ViewModel 继承自 `ObservableObject`，使用 `partial class` + 源生成器。

```csharp
public partial class MyViewModel : ObservableObject
{
    [ObservableProperty]
    private string _someProperty = string.Empty;

    partial void OnSomePropertyChanged(string value)
    {
        // 属性变更时的副作用逻辑
    }

    [RelayCommand]
    private async Task DoSomething()
    {
        // 命令实现
    }
}
```

### 3.2 属性定义

- 使用 `[ObservableProperty]` 源生成器生成公开属性，字段命名 `_camelCase`
- 必须提供初始值（`string.Empty`、`new()`、`false` 等）
- 所有集合类型使用 `ObservableCollection<T>`
- 所有可空引用类型标注 `?`

### 3.3 命令定义

- 使用 `[RelayCommand]` 源生成器
- 异步方法命名 `async Task`，同步方法命名 `void`
- 通过 `{Name}Command.ExecuteAsync(null)` 调用
- 命令的 `CanExecute` 通过依赖属性自动推断（基于 `[NotifyCanExecuteChangedFor]`）

### 3.4 View 与 ViewModel 绑定

- View 在 code-behind 的构造函数中通过 DI 设置 DataContext
- 或通过 ViewLocator 自动解析
- XAML 中不写任何业务逻辑代码
- code-behind 只处理纯 UI 事件（如窗口关闭、动画控制）

***

## 四、服务层规则

### 4.1 HTTP 服务

- 使用 `Flurl.Http` 作为 HTTP 客户端（支持 URL 模板、multipart 上传、流式下载）
- 所有 API 调用返回强类型 DTO，通过 `CommonResponse<T>` 解包
- 异步方法命名遵循 `{Action}{Target}Async` 模式
- 错误处理：统一 catch `FlurlHttpException`，在 ViewModel 层处理并更新状态

```csharp
public async Task<List<ProjectDto>> GetAllProjectsAsync(string serverUrl)
{
    var response = await $"{serverUrl}/api/project/get_all_projects"
        .GetJsonAsync<CommonResponse<List<ProjectDto>>>();
    return response.Data ?? new List<ProjectDto>();
}
```

### 4.2 本地服务

- 文件路径使用 `Path.Combine` 构建（跨平台兼容）
- 配置持久化使用 JSON 格式，存储在 `{LocalApplicationData}/PublishTool/`
- 日志使用 Serilog，输出到 `{LocalApplicationData}/PublishTool/logs/log.log`
- 文件操作（扫描、MD5）在 Service 层封装，ViewModel 不直接使用 `System.IO`

### 4.3 服务注册

- 全局无状态服务注册为 `Singleton`（ProjectService、FileService、ConfigService、LocalFileService、ProcessService）
- ViewModel 注册为 `Transient`（每个 Tab 的 ProjectPageViewModel 独立实例）
- 使用 Microsoft.Extensions.DependencyInjection

***

## 五、API 兼容性规则

### 5.1 JSON 字段命名

所有 DTO 属性必须使用 `[JsonProperty("originalName")]` 标注原始 JSON 字段名称（与 Go 服务端保持一致）。

**重要**：Go 服务端 `ent` 框架生成的 JSON tag 使用 `snake_case`（如 `force_update`、`created_at`），而控制器请求体的自定义字段使用 `camelCase`（如 `isForceUpdate`）。两者不一致，必须逐一核对。

```csharp
// GET 响应 DTO（ent 生成 → snake_case）
public class ProjectDto
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("force_update")]      // 注意：ent 生成的是 snake_case
    public bool IsForceUpdate { get; set; }
}

// POST 请求 DTO（控制器定义 → camelCase）
public class CreateProjectDto
{
    [JsonProperty("isForceUpdate")]      // 注意：控制器期望的是 camelCase
    public bool IsForceUpdate { get; set; }
}
```

### 5.2 接口路径

| 方法   | 路径                                          | 用途              |
| ---- | ------------------------------------------- | --------------- |
| GET  | `/api/project/get_all_projects`             | 获取所有项目          |
| POST | `/api/project/create_project`               | 创建项目            |
| POST | `/api/project/update_project`               | 更新项目            |
| POST | `/api/project/delete_project/{id}`          | 删除项目            |
| GET  | `/api/project/get_project_change_logs/{id}` | 获取变更日志          |
| GET  | `/api/project/get_project_os_info/{id}`     | 获取服务器信息         |
| POST | `/api/file/upload_file`                     | 上传文件（multipart） |
| GET  | `/api/file/get_all_files/{id}`              | 获取文件列表          |
| GET  | `/api/file/download_file?path=`             | 下载文件            |

### 5.3 上传格式

```csharp
await $"{serverUrl}/api/file/upload_file"
    .PostMultipartAsync(mp => mp
        .AddFile("file", fileStream, fileName)
        .AddString("projectName", projectName)
        .AddString("relativeFileName", relativePath));
```

***

## 六、错误处理规则

### 6.1 ViewModel 层

```csharp
[RelayCommand]
private async Task SomeOperation()
{
    IsOperationEnabled = false;
    StatusMessage = "正在执行...";
    try
    {
        // 核心逻辑
        StatusMessage = "操作成功";
    }
    catch (FlurlHttpException ex)
    {
        StatusMessage = $"网络错误: {ex.Message}";
    }
    catch (Exception ex)
    {
        StatusMessage = $"操作失败: {ex.Message}";
    }
    finally
    {
        IsOperationEnabled = true;
    }
}
```

### 6.2 状态管理

- 每个耗时操作都必须有 `IsXxxEnabled` 或 `IsBusy` 属性控制 UI 状态
- 操作开始时立即禁用按钮，防止重复提交
- 状态消息使用中文，简洁明了
- 异常必须记录到日志（Serilog），不能静默吞掉

***

## 七、XAML 规则

### 7.1 布局规则

- 使用 Grid 布局，明确指定 RowDefinitions / ColumnDefinitions
- 面板尺寸：不使用嵌套过多的 StackPanel
- Margin 间距统一使用 4 的倍数（4, 8, 12, 16）
- 列表项使用 DataTemplate，不写循环控件

### 7.2 绑定规则

- 使用 `{Binding Property}`（不指定 Source，继承 DataContext）
- 使用 `{Binding Path, Converter={StaticResource MyConverter}}` 处理类型转换
- 使用 `{x:Static}` 引用枚举和常量

### 7.3 样式规则 — Semi.Avalonia 与 Ursa.Avalonia 控件与主题

- 窗口使用 `<semi:Window>` 命名空间（`xmlns:semi="https://irihi.tech/semi"`）
- 主题令牌使用 `SemiColor*` 前缀：`SemiColorPrimary`、`SemiColorSuccess`、`SemiColorDanger`、`SemiColorText0` 等
- 按钮样式使用 `Classes="Primary"`、`Classes="Tertiary"`、`Classes="Danger"`
- 主题初始化在 `App.axaml` 中通过 `<semi:SemiTheme>` 和 `<semi:UrsaSemiTheme>` 加载

***

## 八、模型设计规则

### 8.1 DTO（PublishTool.Models 命名空间）

- 用于 API 请求/响应的序列化/反序列化
- 使用 Newtonsoft.Json 的 `[JsonProperty]` 属性
- 所有属性必须有 `{ get; set; }`
- 集合类型默认初始化为 `new()`
- 字符串默认初始化为 `string.Empty`

### 8.2 本地模型（PublishTool.Models.Local 命名空间）

- 用于 UI 绑定和本地业务逻辑
- 使用 CommunityToolkit.Mvvm 的 `[ObservableProperty]` 源生成器
- 必须继承 `ObservableObject`
- 枚举类型定义在模型文件同文件中

***

## 九、交互规则

- 写代码之前，如果有不懂不清楚的地方，主动向我提问
- 将问题写在 `e:\Project2026\ClientUpdator\.trae\documents\Interactive.md` 中
- 我会在 Interactive.md 中作答，确认后再继续执行
- 不要猜测或假设，宁可多问也不做错

## 十、自检规则

- 每次修改完成后，全面检查一遍是否有 bug
- 确认是否完全完成任务，没有遗漏
- 检查是否有跑偏的嫌疑，确保与目标一致
- 执行任务过程中如果有不确定的地方，记录到 `e:\Project2026\ClientUpdator\.trae\documents\Interactive.md` 中，等待回答后再继续

## 十一、UI 组件设计原则

- **能直接用系统或库自带控件的，绝不自定义 UserControl**
- 自定义 UserControl 只用于：组合多个控件形成有意义的业务单元（如项目卡片、文件列表项）
- 单个按钮、单个输入框、单个标签等简单控件，直接在父容器中使用
- 代码要简洁易读，避免过度封装
- 避免为了一行代码就创建一个 UserControl 文件

## 十二、禁止事项

1. ❌ 不在 XAML code-behind 中写业务逻辑
2. ❌ 不使用 `System.Windows.Forms`（WinForms 依赖，Avalonia 不支持）
3. ❌ 不在 ViewModel 中直接引用 View 类型
4. ❌ 不使用 `Task.Run` 处理 UI 线程（使用异步命令 + `Dispatcher`）
5. ❌ 不硬编码服务器地址（通过用户输入配置）
6. ❌ 不在代码中写占位符 TODO（计划阶段已明确）
7. ❌ 不添加不必要的注释（代码本身应自解释）
8. ❌ 不引用未使用的命名空间
9. ❌ 不使用 `var` 以外的隐式类型（保持可读性）
10. ❌ 不在生产代码中写 `Console.WriteLine`（使用 Serilog）
11. ❌ 不为了简单控件而创建自定义 UserControl

***

> <br />
>
> **版本**: 1.2\
> **适用范围**: publish\_tool\_avalonia/ 项目（.NET 8 + Avalonia 12 + CommunityToolkit.Mvvm + Semi.Avalonia + Ursa.Avalonia）\
> **兼容服务端**: server/（Go + Gin + Ent + SQLite）保持完全兼容，无需修改

***

## 十三、文档参考

- **Avalonia 12 官方文档**: https://docs.avaloniaui.net/docs/welcome
- **Semi.Avalonia 使用参考**: https://github.com/irihi/Semi.Avalonia
- **Ursa.Avalonia 使用参考**: https://github.com/irihi/Ursa