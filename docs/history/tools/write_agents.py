import sys
sys.stdout.reconfigure(encoding='utf-8')

content = r"""# Aly 项目规则

> 所有新增/修改代码必须遵循以下规则。

---

## 〇、项目概述与职责边界

| 子项目 | 目录 | 职责 | 技术栈 |
| ------ | ---- | ---- | ------ |
| **server** | `server/` | 服务端，提供项目管理和文件管理 REST API | Go 1.25 + Gin + Ent + SQLite |
| **publish-gui** | `publish/publish-gui/` | 版本发布 GUI 工具，管理项目、上传文件、发布版本 | .NET 8 + Avalonia 12 + CommunityToolkit.Mvvm |
| **publish-cli** | `publish/publish-cli/` | 版本发布 CLI 工具，命令行发布管理，类 git 工作流 | Go 1.25 + cobra |
| **client** | `client/` | 终端用户更新程序，检查更新、下载更新、应用更新 | Go 1.10（兼容 Windows XP） |

**关键约束**：
- `publish-gui` / `publish-cli` 面向**发布者**，负责上传文件到服务端、管理版本、触发发布
- `client` 面向**终端用户**，负责从服务端检测新版本、下载更新包、执行文件替换
- `server` 是三端共同依赖的后端，API 路径和数据结构必须与三端保持一致
- 修改 `server` API 时，必须同步确认 GUI/CLI 的 DTO 和 Service 层是否需要调整
- 开发 `client` 时，应复用 `server` 已有 API 端点，避免单独新增接口（除非必要）

---

## 一、项目结构

```
Aly/
├── server/                            # Go 服务端
│   ├── cmd/main.go                    # 入口
│   ├── cmd/gen/main.go                # ent 代码生成入口
│   ├── controllers/                   # HTTP 控制器
│   ├── ent/                           # ent ORM 生成代码
│   ├── internal/                      # 内部包（db, service, utils）
│   ├── models/                        # 数据模型
│   └── routers/routers.go             # 路由定义
│
├── client/                            # Go 客户端（终端用户更新）
│   ├── main.go                        # 入口
│   ├── client.yaml                    # 运行时配置
│   ├── client/http_client.go          # HTTP 客户端
│   ├── cmd/                           # cobra 命令（check_update, download_update, apply_update, rollback 等）
│   ├── config/                        # 配置与版本管理
│   ├── model/dto.go                   # 数据传输对象
│   └── util/                          # 文件操作、进程管理工具
│
├── publish/
│   ├── publish-gui/                   # C# GUI 发布工具
│   │   ├── AlyPublish.slnx
│   │   └── src/AlyPublish/
│   │       ├── App.axaml(.cs)         # 应用入口 + 主题初始化
│   │       ├── Program.cs             # 启动入口
│   │       ├── ViewLocator.cs         # ViewModel 自动解析
│   │       ├── Constants/             # 常量定义
│   │       ├── Converters/            # IValueConverter 实现
│   │       ├── Helpers/               # 静态工具类
│   │       ├── Models/
│   │       │   ├── Cli/               # CLI 输出 DTO（与服务端 JSON 对应）
│   │       │   └── Local/             # 本地业务模型（ObservableObject 子类）
│   │       ├── Services/              # 服务层（CliService, ConfigService, ProcessService）
│   │       ├── ViewModels/            # ViewModel 层
│   │       └── Views/
│   │           ├── MainWindow.axaml   # 主窗口
│   │           └── Dialogs/           # 对话框
│   │
│   └── publish-cli/                   # Go CLI 发布工具
│       ├── cmd/publish-cli/main.go    # 入口
│       ├── internal/
│       │   ├── api/client.go          # HTTP 客户端
│       │   ├── cmd/                   # cobra 命令（add, push, status, log, config, watch 等）
│       │   ├── config/config.go       # 配置管理
│       │   ├── diff/scanner.go        # 文件扫描 + 差异比对
│       │   └── staging/staging.go     # 暂存区管理
│       └── pkg/models/models.go       # DTO 数据模型
│
└── README.md
```

**分层依赖**：View → ViewModel → Service → Model（单向，禁止反向引用）

---

## 二、命名规则（publish-gui）

### 2.1 文件命名

| 类型 | 规则 | 示例 |
| ---- | ---- | ---- |
| DTO（Models/Cli/） | `{EntityName}.cs` | `CliOutput.cs`, `ProjectInfo.cs` |
| 本地模型（Models/Local/） | `{EntityName}.cs` | `ProjectConfig.cs`, `FileItem.cs` |
| Service | `{Domain}Service.cs` | `CliService.cs`, `ConfigService.cs` |
| ViewModel | `{Page}ViewModel.cs` | `MainWindowViewModel.cs` |
| View | `{Name}.axaml + .axaml.cs` | `MainWindow.axaml` |
| Dialog | `{Action}Dialog.axaml + .axaml.cs` | `AddProjectDialog.axaml` |
| Converter | `{Source}To{Target}Converter.cs` | `BytesToSizeConverter.cs` |
| Helper | `{Function}Helper.cs` | `FileSizeHelper.cs` |

### 2.2 代码命名

| 元素 | 规则 | 示例 |
| ---- | ---- | ---- |
| 命名空间 | `AlyPublish.{Layer}` | `AlyPublish.Services` |
| 类 | PascalCase | `MainWindowViewModel` |
| 接口 | `I` 前缀 + PascalCase | `IProjectApi` |
| 属性 | PascalCase | `ServerUrl` |
| 字段（private） | `_camelCase` | `_configService` |
| 方法参数 / 局部变量 | camelCase | `serverUrl`, `filteredProjects` |
| 常量 | PascalCase | `DefaultPort` |

### 2.3 XAML 命名

- `x:Name`：PascalCase（如 `ProjectListBox`）
- 绑定路径：PascalCase（如 `{Binding ServerUrl}`）
- 资源键：PascalCase（如 `{DynamicResource WindowBackgroundBrush}`）

---

## 三、MVVM 架构规则（publish-gui）

### 3.1 ViewModel 规范

继承 `ObservableObject`，使用 `partial class` + 源生成器：

```csharp
public partial class MyViewModel : ObservableObject
{
    [ObservableProperty]
    private string _someProperty = string.Empty;

    partial void OnSomePropertyChanged(string value) { /* 副作用 */ }

    [RelayCommand]
    private async Task DoSomething() { /* 命令实现 */ }
}
```

- `[ObservableProperty]` 字段 `_camelCase`，必须提供初始值
- 集合类型使用 `ObservableCollection<T>`，可空引用标注 `?`
- 异步命令 `async Task`，同步命令 `void`，通过 `{Name}Command.ExecuteAsync(null)` 调用

### 3.2 View 绑定

- code-behind 构造函数中通过 DI 设置 DataContext（或通过 ViewLocator 自动解析）
- XAML 中不写业务逻辑，code-behind 只处理纯 UI 事件

---

## 四、服务层规则（publish-gui）

### 4.1 HTTP 服务

使用 `Flurl.Http`，API 返回强类型 DTO，通过 `CommonResponse<T>` 解包，方法命名 `{Action}{Target}Async`：

```csharp
public async Task<List<ProjectDto>> GetAllProjectsAsync(string serverUrl)
{
    var response = await $"{serverUrl}/api/project/get_all_projects"
        .GetJsonAsync<CommonResponse<List<ProjectDto>>>();
    return response.Data ?? new List<ProjectDto>();
}
```

### 4.2 本地服务

- 文件路径用 `Path.Combine`，配置持久化用 JSON（`{LocalApplicationData}/AlyPublish/`）
- 日志用 Serilog（`{LocalApplicationData}/AlyPublish/logs/log.log`）
- 文件操作在 Service 层封装，ViewModel 不直接使用 `System.IO`

### 4.3 服务注册

- Singleton：全局无状态服务（ConfigService、CliService、ProcessService）
- Transient：ViewModel
- 使用 `Microsoft.Extensions.DependencyInjection`

---

## 五、API 兼容性规则（三端统一）

### 5.1 JSON 字段命名

DTO 属性必须使用 `[JsonProperty("originalName")]` 标注。

**注意**：Go ent 生成的 JSON tag 是 `snake_case`（如 `force_update`），控制器请求体是 `camelCase`（如 `isForceUpdate`），两者不一致，必须逐一核对。

**统一约定**：三端外层包装（`isSuccess`/`errorMsg`）统一使用 **camelCase**。

```csharp
// GET 响应（ent → snake_case）
[JsonProperty("force_update")]
public bool IsForceUpdate { get; set; }

// POST 请求（控制器 → camelCase）
[JsonProperty("isForceUpdate")]
public bool IsForceUpdate { get; set; }
```

### 5.2 接口路径

| 方法 | 路径 | 用途 |
| ---- | ---- | ---- |
| GET | `/api/project/get_all_projects` | 获取所有项目 |
| POST | `/api/project/create_project` | 创建项目 + 初始版本 V1.0.0 |
| POST | `/api/project/update_project` | 更新项目配置 |
| POST | `/api/project/delete_project/{projectId}` | 软删除项目 |
| GET | `/api/project/get_project_change_logs/{projectId}` | 获取变更日志 |
| GET | `/api/project/get_project_os_info/{projectId}` | 获取服务端系统信息 |
| POST | `/api/project/publish_version` | 发布新版本 |
| POST | `/api/file/upload_file` | 上传文件（multipart） |
| GET | `/api/file/get_all_files/{projectId}` | 获取文件列表（含 MD5/SHA256） |
| GET | `/api/file/download_file?path=` | 下载文件 |

### 5.3 上传格式

```csharp
await $"{serverUrl}/api/file/upload_file"
    .PostMultipartAsync(mp => mp
        .AddFile("file", fileStream, fileName)
        .AddString("projectName", projectName)
        .AddString("relativeFileName", relativePath));
```

---

## 六、错误处理（publish-gui）

```csharp
[RelayCommand]
private async Task SomeOperation()
{
    IsOperationEnabled = false;
    StatusMessage = "正在执行...";
    try { /* 核心逻辑 */ StatusMessage = "操作成功"; }
    catch (FlurlHttpException ex) { StatusMessage = $"网络错误: {ex.Message}"; }
    catch (Exception ex) { StatusMessage = $"操作失败: {ex.Message}"; }
    finally { IsOperationEnabled = true; }
}
```

- 每个耗时操作必须有 `IsXxxEnabled` / `IsBusy` 控制 UI 状态，开始时立即禁用按钮
- 状态消息使用中文，异常必须记录到 Serilog，不能静默吞掉

---

## 七、XAML 规则（publish-gui）

### 7.1 布局与绑定

- Grid 布局，明确 RowDefinitions / ColumnDefinitions，Margin 用 4 的倍数
- `{Binding Property}` 不指定 Source，`Converter={StaticResource MyConverter}` 处理类型转换

### 7.2 Semi.Avalonia / Ursa.Avalonia 主题

- 窗口用 `<semi:Window>`（`xmlns:semi="https://irihi.tech/semi"`）
- 主题令牌：`SemiColorPrimary`、`SemiColorSuccess`、`SemiColorDanger`、`SemiColorText0` 等
- 按钮样式：`Classes="Primary"` / `"Tertiary"` / `"Danger"`
- 主题初始化在 `App.axaml`：`<semi:SemiTheme>` + `<semi:UrsaSemiTheme>`

---

## 八、模型设计规则（publish-gui）

### 8.1 DTO（`AlyPublish.Models.Cli`）

- 用于 API 序列化/反序列化，使用 Newtonsoft.Json `[JsonProperty]`
- 属性 `{ get; set; }`，集合默认 `new()`，字符串默认 `string.Empty`

### 8.2 本地模型（`AlyPublish.Models.Local`）

- 用于 UI 绑定，继承 `ObservableObject`，使用 `[ObservableProperty]`
- 枚举类型定义在同文件中

---

## 九、UI 组件设计原则

- **能用系统/库自带控件的，绝不自定义 UserControl**
- 自定义只用于组合多个控件形成有意义的业务单元
- 避免为了一行代码就创建一个 UserControl

---

## 十、禁止事项

1. ❌ 不在 XAML code-behind 中写业务逻辑
2. ❌ 不使用 `System.Windows.Forms`（Avalonia 不支持）
3. ❌ 不在 ViewModel 中引用 View 类型
4. ❌ 不用 `Task.Run` 处理 UI 线程（用异步命令 + `Dispatcher`）
5. ❌ 不硬编码服务器地址
6. ❌ 不写占位符 TODO
7. ❌ 不添加不必要的注释
8. ❌ 不引用未使用的命名空间
9. ❌ 不在生产代码中写 `Console.WriteLine`（用 Serilog）
10. ❌ 不为了简单控件创建自定义 UserControl

---

## 十一、交互与自检

- 写代码前如有不清楚的地方，主动提问，不要猜测或假设
- 将问题写在 `.trae/documents/Interactive.md` 中，等待回答后再继续
- 每次修改完成后，全面检查是否有 bug，确认任务是否完全完成

---

## 十二、PowerShell 编码警告

> PowerShell `Set-Content` 不加 `-Encoding UTF8` 会将 UTF-8 中文按 GBK 编码导致乱码。
>
> **编辑含中文的文件时，始终使用 Python：**
> `python -c "with open(path,'r',encoding='utf-8') as f: c=f.read(); ..."`
>
> **禁止**：`Set-Content`、`Out-File`、`echo >`（不加 `-Encoding UTF8`）
> **已乱码时**：`git checkout -- <file>` 恢复后用 Python 重新编辑

---

## 十三、文档参考

- **Avalonia 12**: https://docs.avaloniaui.net/docs/welcome
- **Semi.Avalonia**: https://github.com/irihi/Semi.Avalonia
- **Ursa.Avalonia**: https://github.com/irihi/Ursa

---

> **版本**: 2.0 | **适用范围**: publish/publish-gui/（.NET 8 + Avalonia 12 + CommunityToolkit.Mvvm + Semi.Avalonia + Ursa.Avalonia）
"""

with open(r'E:\Project2026\aly\AGENTS.md', 'w', encoding='utf-8') as f:
    f.write(content.lstrip())

print(f'Written {len(content)} chars to AGENTS.md')
