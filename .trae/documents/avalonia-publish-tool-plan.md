# Avalonia Publish Tool 实现计划

> **面向 AI 代理的工作者：** 此计划用于实现一个新的桌面发布工具客户端，替换现有的 Flutter 客户端，保留 Go 服务端不变。

**目标：** 使用 .NET 8 + Avalonia UI + CommunityToolkit.Mvvm + Atom UI 重新实现现有的 Flutter 发布工具客户端，与已有的 Go 服务端完全兼容。

**架构：** 经典的 MVVM 三层架构（View/ViewModel/Service），通过 HTTP REST API 与现有 Go 服务端通信。保持与现有 Flutter 客户端完全相同的 API 接口和业务逻辑。

**技术栈：**
- .NET 8 / .NET 8
- Avalonia 11.x（跨平台桌面 UI 框架）
- CommunityToolkit.Mvvm 8.x（MVVM 框架，源生成器）
- Atom UI（Avalonia 主题）
- FluentAvalonia（Avalonia Windows 11 Fluent 风格控件）
- Refit / Flurl（HTTP 客户端）
- Newtonsoft.Json / System.Text.Json（JSON 序列化）
- Serilog（日志框架）

---

## 文件结构

```
publish_tool_avalonia/
├── PublishTool.sln
├── src/
│   ├── PublishTool/                      # 主项目
│   │   ├── PublishTool.csproj
│   │   ├── App.axaml                     # 应用入口 + Atom UI 主题/样式配置
│   │   ├── App.axaml.cs
│   │   ├── Program.cs                    # 启动入口
│   │   ├── ViewLocator.cs               # 视图定位器（用于数据模板绑定 ViewModel→View）
│   │   │
│   │   ├── Models/                       # 数据模型（DTO）
│   │   │   ├── CommonResponse.cs
│   │   │   ├── ProjectDto.cs
│   │   │   ├── CreateProjectDto.cs
│   │   │   ├── UpdateProjectDto.cs
│   │   │   ├── FileInfoDto.cs
│   │   │   ├── ProjectChangeLog.cs
│   │   │   └── ServerOsInfoDto.cs
│   │   │
│   │   ├── Models/Local/                 # 本地业务模型
│   │   │   ├── ProjectConfig.cs          # 本地项目配置（JSON 持久化）
│   │   │   ├── LocalFileItem.cs          # 本地文件列表项
│   │   │   └── UploadFileItem.cs         # 上传队列项
│   │   │
│   │   ├── Services/                     # 服务层
│   │   │   ├── IProjectApi.cs            # 项目相关 API 接口定义
│   │   │   ├── IFileApi.cs               # 文件接口定义
│   │   │   ├── ProjectService.cs         # 项目 API 实现
│   │   │   ├── FileService.cs            # 文件 API 实现（上传/下载/扫描/MD5）
│   │   │   ├── LocalFileService.cs       # 本地文件操作（扫描目录、MD5 计算）
│   │   │   ├── ConfigService.cs          # 本地配置持久化服务
│   │   │   └── ProcessService.cs         # 进程管理服务（启动 exe、打开文件夹）
│   │   │
│   │   ├── ViewModels/                   # ViewModel 层
│   │   │   ├── MainWindowViewModel.cs    # 主窗口 ViewModel（项目列表、Tab 管理）
│   │   │   ├── ProjectPageViewModel.cs   # 单项目页面 ViewModel（核心业务逻辑）
│   │   │   ├── AddProjectDialogViewModel.cs
│   │   │   ├── AddServerProjectDialogViewModel.cs
│   │   │   ├── ProjectSettingsDialogViewModel.cs
│   │   │   │   ├── ConfigEditorDialogViewModel.cs
│   │   │
│   │   ├── Views/                        # 视图层（Avalonia Window/UserControl）
│   │   │   ├── MainWindow.axaml
│   │   │   ├── MainWindow.axaml.cs
│   │   │   ├── Controls/                 # 自定义控件
│   │   │   │   ├── ProjectListPanel.axaml
│   │   │   │   ├── ProjectCard.axaml
│   │   │   │   ├── ProjectTabBar.axaml
│   │   │   │   ├── StatusBar.axaml  ├── StatusBar.axaml
│   │   │   │   ├── ServerInfoBar.axaml
│   │   │   │   ├── ToolbarBar.axaml
│   │   │   │   ├── LocalFilesPanel.axaml
│   │   │   │   ├── OperationButtons.axaml
│   │   │   │   ├── UploadQueuePanel.axaml
│   │   │   │   └── BottomActionBar.axaml
│   │   │   │
│   │   │   ├── Dialogs/                  # 对话框
│   │   │   │   ├── AddProjectDialog.axaml
│   │   │   │   ├── AddServerProjectDialog.axaml
│   │   │   │   ├── ProjectSettingsDialog.axaml
│   │   │   │   └── ConfigEditorDialog.axaml
│   │   │
│   │   ├── Converters/                   # 值转换器
│   │   │   ├── BoolToColorConverter.cs
│   │   │   ├── FileStatusToColorConverter.cs
│   │   │   └── BytesToSizeConverter.cs
│   │   │
│   │   ├── Helpers/                      # 工具类
│   │   │   ├── Md5Helper.cs
│   │   │   └── LogHelper.cs
│   │   │
│   │   └── Assets/                       # 资源文件
│   │       ├── Icons/                    # 图标资源
│   │       └── Styles/                   # 自定义样式
│   │
│   └── PublishTool.Tests/                # 单元测试项目
│       ├── PublishTool.Tests.csproj
│       └── ViewModels/
│           └── ProjectPageViewModelTests.cs
```

---

## 任务分解

### 任务 1：项目骨架搭建

**文件：**
- 创建：`publish_tool_avalonia/PublishTool.sln`
- 创建：`publish_tool_avalonia/src/PublishTool/PublishTool.csproj`
- 创建：`publish_tool_avalonia/src/PublishTool/Program.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/App.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/App.axaml.cs`

- [ ] **步骤 1：创建解决方案和项目文件**

创建 `PublishTool.csproj`，配置：
- TargetFramework: net8.0
- Avalonia 11.x 包引用（Avalonia.Desktop, Avalonia.Themes.Fluent, Avalonia.Fonts.Inter）
- CommunityToolkit.Mvvm 8.x
- FluentAvalonia NuGet 包（提供 Win11 风格控件）
- Atom UI 主题包（AtomUI.Avalonia）
- Refit/Flurl.Http 用于 HTTP 请求
- Serilog 用于日志

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>WinExe</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Avalonia" Version="11.2.+" />
    <PackageReference Include="Avalonia.Desktop" Version="11.2.+" />
    <PackageReference Include="Avalonia.Themes.Fluent" Version="11.2.+" />
    <PackageReference Include="Avalonia.Fonts.Inter" Version="11.2.+" />
    <PackageReference Include="CommunityToolkit.Mvvm" Version="8.4.+" />
    <PackageReference Include="AtomUI" Version="11.2.+" />
    <PackageReference Include="Flurl.Http" Version="4.0.+" />
    <PackageReference Include="Newtonsoft.Json" Version="13.0.+" />
    <PackageReference Include="Serilog.Sinks.File" Version="6.0.+" />
    <PackageReference Include="Serilog.Sinks.Console" Version="6.0.+" />
  </ItemGroup>
</Project>
```

- [ ] **步骤 2：创建 Program.cs 入口**

```csharp
using Avalonia;
using Serilog;

namespace PublishTool;

public static class Program
{
    public static void Main(string[] args)
    {
        Log.Logger = new LoggerConfiguration()
            .WriteTo.Console()
            .WriteTo.File(Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                "PublishTool", "logs", "log.log"))
            .MinimumLevel.Debug()
            .CreateLogger();

        try
        {
            BuildAvaloniaApp().StartWithClassicDesktopLifetime(args);
        }
        catch (Exception ex)
        {
            Log.Fatal(ex, "Application terminated unexpectedly");
        }
        finally
        {
            Log.CloseAndFlush();
        }
    }

    public static AppBuilder BuildAvaloniaApp()
        => AppBuilder.Configure<App>()
            .UsePlatformDetect()
            .WithInterFont()
            .LogToTrace();
}
```

- [ ] **步骤 3：创建 App.axaml 和 App.axaml.cs**

Atom UI 主题已在 App.axaml 中配置全局样式。注册全局服务到 DI 容器。

```xml
<!-- App.axaml -->
<Application xmlns="https://github.com/avaloniaui"
             xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
             x:Class="PublishTool.App">
  x:Class="PublishTool.App">
  <Application.Styles>
    <FluentTheme />
    <AtomTheme />
  </Application.Styles>
</Application>
```

```csharp
// App.axaml.cs
using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Data.Core.Plugins;
using Avalonia.Markup.Xaml;
using Microsoft.Extensions.DependencyInjection;
using PublishTool.Services;
using PublishTool.ViewModels;
using PublishTool.Views;

namespace PublishTool;

public partial class App : Application
{
    public override void Initialize()
    {
        AvaloniaXamlLoader.Load(this);
    }

    public override void OnFrameworkInitializationCompleted()
    {
        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            // Avoid duplicate validations from both Avalonia and CommunityToolkit
            BindingPlugins.DataValidators.RemoveAt(0);

            var services = new ServiceCollection();
            ConfigureServices(services);
            var provider = services.BuildServiceProvider();

            desktop.MainWindow = new MainWindow
            {
                DataContext = provider.GetRequiredService<MainWindowViewModel>()
            };
        }

        base.OnFrameworkInitializationCompleted();
    }

    private void ConfigureServices(IServiceCollection services)
    {
        services.AddSingleton<ConfigService>();
        services.AddSingleton<LocalFileService>();
        services.AddSingleton<ProcessService>();
        services.AddSingleton<ProjectService>();
        services.AddSingleton<FileService>();

        services.AddTransient<MainWindowViewModel>();
        services.AddTransient<ProjectPageViewModel>();
    }
}
```

使用 Microsoft.Extensions.DependencyInjection 而非 CommunityToolkit 的 DI，以获得更完整的 DI 容器功能。需要额外引入 `Microsoft.Extensions.DependencyInjection` NuGet 包。

---

### 任务 2：DTO 数据模型定义

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Models/CommonResponse.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/ProjectDto.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/FileInfoDto.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/ProjectChangeLog.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/ServerOsInfoDto.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/CreateProjectDto.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/UpdateProjectDto.cs`

- [ ] **步骤 1：创建 CommonResponse**

```csharp
using Newtonsoft.Json;

namespace PublishTool.Models;

public class CommonResponse
{
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }

    [JsonProperty("errorMsg")]
    public string? ErrorMsg { get; set; }

    [JsonProperty("data")]
    public object? Data { get; set; }
}

public class CommonResponse<T> where T : class
{
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }

    [JsonProperty("errorMsg")]
    public string? ErrorMsg { get; set; }

    [JsonProperty("data")]
    public T? Data { get; set; }
}
```

- [ ] **步骤 2：创建 ProjectDto**

```csharp
using Newtonsoft.Json;

namespace PublishTool.Models;

public class ProjectDto
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("name")]
    public string Name { get; set; } = string.Empty;

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("isForceUpdate")]
    public bool IsForceUpdate { get; set; }

    [JsonProperty("ignoreFolders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignoreFiles")]
    public List<string> IgnoreFiles { get; set; } = new();

    [JsonProperty("createdAt")]
    public string? CreatedAt { get; set; }

    [JsonProperty("isDeleted")]
    public bool IsDeleted { get; set; }
}
```

- [ ] **步骤 3：创建 FileInfoDto**

```csharp
using Newtonsoft.Json;

namespace PublishTool.Models;

public class FileInfoDto
{
    [JsonProperty("fileAbsolutePath")]
    public string FileAbsolutePath { get; set; } = string.Empty;

    [JsonProperty("fileRelativePath")]
    public string FileRelativePath { get; set; } = string.Empty;

    [JsonProperty("lastUpdateTime")]
    public string? LastUpdateTime { get; set; }

    [JsonProperty("fileSize")]
    public long FileSize { get; set; }

    [JsonProperty("md5")]
    public string Md5 { get; set; } = string.Empty;
}
```

- [ ] **步骤 4：创建 ProjectChangeLog**

```csharp
using Newtonsoft.Json;

namespace PublishTool.Models;

public class ProjectChangeLog
{
    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("logs")]
    public List<string> Logs { get; set; } = new();

    [JsonProperty("time")]
    public string? Time { get; set; }

    [JsonProperty("createdAt")]
    public string? CreatedAt { get; set; }

    [JsonProperty("isDeleted")]
    public bool IsDeleted { get; set; }
}
```

- [ ] **步骤 5：创建 ServerOsInfoDto**

```csharp
using Newtonsoft.Json;

namespace PublishTool.Models;

public class ServerOsInfoDto
{
    [JsonProperty("OS")]
    public string? Os { get; set; }

    [JsonProperty("platform")]
    public string? Platform { get; set; }

    [JsonProperty("goARCH")]
    public string? GoArch { get; set; }

    [JsonProperty("version")]
    public string? Version { get; set; }

    [JsonProperty("numCPU")]
    public int NumCpu { get; set; }

    [JsonProperty("cpuName")]
    public string? CpuName { get; set; }

    [JsonProperty("cpuMhz")]
    public double CpuMhz { get; set; }

    [JsonProperty("diskUsed")]
    public ulong DiskUsed { get; set; }

    [JsonProperty("diskFree")]
    public ulong DiskFree { get; set; }

    [JsonProperty("diskTotal")]
    public ulong DiskTotal { get; set; }

    [JsonProperty("diskUsedPercent")]
    public double DiskUsedPercent { get; set; }
}
```

- [ ] **步骤 6：创建 CreateProjectDto 和 UpdateProjectDto**

```csharp
// CreateProjectDto.cs
using Newtonsoft.Json;

namespace PublishTool.Models;

public class CreateProjectDto
{
    [JsonProperty("name")]
    public string Name { get; set; } = string.Empty;

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("isForceUpdate")]
    public bool IsForceUpdate { get; set; }

    [JsonProperty("ignoreFolders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignoreFiles")]
    public List<string> IgnoreFiles { get; set; } = new();
}

// UpdateProjectDto.cs
public class UpdateProjectDto
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("isForceUpdate")]
    public bool IsForceUpdate { get; set; }

    [JsonProperty("ignoreFolders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignoreFiles")]
    public List<string> IgnoreFiles { get; set; } = new();
}
```

---

### 任务 3：本地业务模型

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Models/Local/ProjectConfig.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/Local/LocalFileItem.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Models/Local/UploadFileItem.cs`

- [ ] **步骤 1：创建 ProjectConfig**

```csharp
using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishTool.Models.Local;

public partial class ProjectConfig : ObservableObject
{
    [ObservableProperty]
    private int _serverId;

    [ObservableProperty]
    private string _name = string.Empty;

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    private string? _exePath;

    [ObservableProperty]
    private string _localPath = string.Empty;

    [ObservableProperty]
    private int _sortOrder;
}
```

- [ ] **步骤 2：创建LocalFileItem**

```csharp
using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishTool.Models.Local;

public partial class LocalFileItem : ObservableObject
{
    [ObservableProperty]
    private string _fileName = string.Empty;

    [ObservableProperty]
    private string _absolutePath = string.Empty;

    [ObservableProperty]
    private string _relativePath = string.Empty;

    [ObservableProperty]
    private DateTime _lastModified;

    [ObservableProperty]
    private bool _isChecked;

    [ObservableProperty]
    private bool _isModified;
}
```

- [ ] **步骤 3：创建 UploadFileItem**

```csharp
using CommunityToolkit.Mvvm.ComponentModel;

namespace PublishTool.Models.Local;

public enum UploadStatus
{
    Pending,
    Uploading,
    Done,
    Failed
}

public partial class UploadFileItem : ObservableObject
{
    [ObservableProperty]
    private string _fileName = string.Empty;

    [ObservableProperty]
    private string _localPath = string.Empty;

    [ObservableProperty]
    private string _relativePath = string.Empty;

    [ObservableProperty]
    private DateTime _lastModified;

    [ObservableProperty]
    private UploadStatus _status = UploadStatus.Pending;
}
```

---

### 任务 4：服务层 - HTTP API 封装

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Services/ProjectService.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Services/FileService.cs`

使用 `Flurl.Http` 作为 HTTP 客户端（比 Refit 更灵活，适合有文件上传/下载的场景）。

- [ ] **步骤 1：创建 ProjectService**

```csharp
using Flurl.Http;
using Newtonsoft.Json;
using PublishTool.Models;

namespace PublishTool.Services;

public class ProjectService
{
    public async Task<List<ProjectDto>> GetAllProjectsAsync(string serverUrl)
    {
        var response = await $"{serverUrl}/api/project/get_all_projects"
            .GetJsonAsync<CommonResponse<List<ProjectDto>>>();
        return response.Data ?? new List<ProjectDto>();
    }

    public async Task<ProjectDto?> CreateProjectAsync(string serverUrl, CreateProjectDto dto)
    {
        var response = await $"{serverUrl}/api/project/create_project"
            .PostJsonAsync(dto)
            .ReceiveJson<CommonResponse<ProjectDto>>();
        return response.Data;
    }

    public async Task<ProjectDto?> UpdateProjectAsync(string serverUrl, UpdateProjectDto dto)
    {
        var response = await $"{serverUrl}/api/project/update_project"
            .PostJsonAsync(dto)
            .ReceiveJson<CommonResponse<ProjectDto>>();
        return response.Data;
    }

    public async Task<bool> DeleteProjectAsync(string serverUrl, int projectId)
    {
        var response = await $"{serverUrl}/api/project/delete_project/{projectId}"
            .PostAsync()
            .ReceiveJson<CommonResponse>();
        return response.IsSuccess;
    }

    public async Task<List<ProjectChangeLog>> GetChangeLogsAsync(string serverUrl, int projectId)
    {
        var response = await $"{serverUrl}/api/project/get_project_change_logs/{projectId}"
            .GetJsonAsync<CommonResponse<List<ProjectChangeLog>>>();
        return response.Data ?? new List<ProjectChangeLog>();
    }

    public async Task<ServerOsInfoDto?> GetOsInfoAsync(string serverUrl, int projectId)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/get_project_os_info/{projectId}"
                .GetJsonAsync<CommonResponse<ServerOsInfoDto>>();
            return response.Data;
        }
        catch
        {
            return null;
        }
    }
}
```

- [ ] **步骤 2：创建 FileService**

```csharp
using Flurl.Http;
using PublishTool.Models;

namespace PublishTool.Services;

public class FileService
{
    public async Task<List<FileInfoDto>> GetAllFilesAsync(string serverUrl, int projectId)
    {
        var response = await $"{serverUrl}/api/file/get_all_files/{projectId}"
            .GetJsonAsync<CommonResponse<List<FileInfoDto>>>();
        return response.Data ?? new List<FileInfoDto>();
    }

    public async Task UploadFileAsync(string serverUrl, string projectName,
        string relativePath, Stream fileStream)
    {
        await $"{serverUrl}/api/file/upload_file"
            .PostMultipartAsync(mp => mp
                .AddFile("file", fileStream, Path.GetFileName(relativePath))
                .AddString("projectName", projectName)
                .AddString("relativeFileName", relativePath));
    }

    public async Task<Stream> DownloadFileAsync(string serverUrl, string filePath)
    {
        return await $"{serverUrl}/api/file/download_file"
            .SetQueryParam("path", filePath)
            .GetStreamAsync();
    }
}
```

---

### 任务 5：服务层 - 本地服务

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Services/ConfigService.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Services/LocalFileService.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Services/ProcessService.cs`

- [ ] **步骤 1：创建 ConfigService（本地配置持久化）**

```csharp
using Newtonsoft.Json;
using PublishTool.Models.Local;

namespace PublishTool.Services;

public class ConfigService
{
    private readonly string _configPath;
    private List<ProjectConfig> _projects = new();

    public ConfigService()
    {
        var appData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        var dir = Path.Combine(appData, "PublishTool");
        Directory.CreateDirectory(dir);
        _configPath = Path.Combine(dir, "publish_tool_config.json");
        Load();
    }

    public IReadOnlyList<ProjectConfig> Projects => _projects.AsReadOnly();

    public event Action? ProjectsChanged;

    public void Load()
    {
        if (File.Exists(_configPath))
        {
            var json = File.ReadAllText(_configPath);
            _projects = JsonConvert.DeserializeObject<List<ProjectConfig>>(json) ?? new();
        }
    }

    public void Save()
    {
        var json = JsonConvert.SerializeObject(_projects, Formatting.Indented);
        File.WriteAllText(_configPath, json);
        ProjectsChanged?.Invoke();
    }

    public void AddProject(ProjectConfig config)
    {
        config.SortOrder = _projects.Count;
        _projects.Add(config);
        Save();
    }

    public void RemoveProject(ProjectConfig config)
    {
        _projects.Remove(config);
        Save();
    }

    public void UpdateProject(ProjectConfig config)
    {
        var index = _projects.FindIndex(p => p.ServerId == config.ServerId
            && p.ServerUrl == config.ServerUrl);
        if (index >= 0)
        {
            _projects[index] = config;
            Save();
        }
    }

    public void MoveUp(int index)
    {
        if (index > 0)
        {
            (_projects[index], _projects[index - 1]) = (_projects[index - 1], _projects[index]);
            Save();
        }
    }

    public void MoveDown(int index)
    {
        if (index < _projects.Count - 1)
        {
            (_projects[index], _projects[index + 1]) = (_projects[index + 1], _projects[index]);
            Save();
        }
    }
}
```

- [ ] **步骤 2：创建 LocalFileService（文件扫描与 MD5 对比）**

```csharp
using System.Security.Cryptography;
using PublishTool.Models;
using PublishTool.Models.Local;

namespace PublishTool.Services;

public class LocalFileService
{
    public List<LocalFileItem> ScanDirectory(string directoryPath)
    {
        var items = new List<LocalFileItem>();
        if (!Directory.Exists(directoryPath)) return items;

        foreach (var file in Directory.GetFiles(directoryPath, "*", SearchOption.AllDirectories))
        {
            var fullPath = Path.GetFullPath(file);
            var relativePath = Path.GetRelativePath(directoryPath, fullPath);
            items.Add(new LocalFileItem
            {
                FileName = Path.GetFileName(file),
                AbsolutePath = fullPath,
                RelativePath = relativePath,
                LastModified = File.GetLastWriteTime(file),
                IsChecked = false,
                IsModified = false
            });
        }
        return items;
    }

    public string CalculateMd5(string filePath)
    {
        using var md5 = MD5.Create();
        using var stream = File.OpenRead(filePath);
        var hash = md5.ComputeHash(stream);
        return BitConverter.ToString(hash).Replace("-", "").ToLowerInvariant();
    }

    public List<LocalFileItem> GetModifiedFiles(
        string localPath, List<FileInfoDto> serverFiles)
    {
        var localFiles = ScanDirectory(localPath);
        var serverFileDict = serverFiles.ToDictionary(f => f.FileRelativePath);

        foreach (var local in localFiles)
        {
            if (serverFileDict.TryGetValue(local.RelativePath, out var serverFile))
            {
                var localMd5 = CalculateMd5(local.AbsolutePath);
                local.IsModified = !string.Equals(localMd5, serverFile.Md5,
                    StringComparison.OrdinalIgnoreCase);
            }
            else
            {
                local.IsModified = true; // new file not on server
            }
        }
        return localFiles;
    }
}
```

- [ ] **步骤 3：创建 ProcessService**

```csharp
using System.Diagnostics;

namespace PublishTool.Services;

public class ProcessService
{
    public void StartProcess(string exePath, string? arguments = null)
    {
        var psi = new ProcessStartInfo
        {
            FileName = exePath,
            Arguments = arguments ?? string.Empty,
            UseShellExecute = true
        };
        Process.Start(psi);
    }

    public void OpenFolder(string folderPath)
    {
        Process.Start(new ProcessStartInfo
        {
            FileName = folderPath,
            UseShellExecute = true
        });
    }

    public void OpenInExplorer(string filePath)
    {
        Process.Start("explorer.exe", $"/select,\"{filePath}\"");
    }
}
```

---

### 任务 6：ViewModel 核心层（CommunityToolkit.Mvvm）

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/ViewModels/MainWindowViewModel.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/ViewModels/ProjectPageViewModel.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/ViewModels/AddProjectDialogViewModel.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/ViewModels/AddServerProjectDialogViewModel.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/ViewModels/ProjectSettingsDialogViewModel.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/ViewModels/ConfigEditorDialogViewModel.cs`

- [ ] **步骤 1：创建 MainWindowViewModel**

核心职责：管理左侧项目列表示、Tab 打开/关闭/激活、搜索过滤、项目排序、新建项目。

```csharp
using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models.Local;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly ConfigService _configService;
    private readonly ProjectService _projectService;

    [ObservableProperty]
    private string _searchText = string.Empty;

    [ObservableProperty]
    private ObservableCollection<ProjectConfig> _filteredProjects = new();

    [ObservableProperty]
    private ObservableCollection<ProjectPageViewModel> _openTabs = new();

    [ObservableProperty]
    private ProjectPageViewModel? _selectedTab;

    public MainWindowViewModel(
        ConfigService configService,
        ProjectService projectService)
    {
        _configService = configService;
        _projectService = projectService;

        // 初始化时加载项目列表
        foreach (var project in _configService.Projects)
        {
            FilteredProjects.Add(project);
        }

        _configService.ProjectsChanged += OnProjectsChanged;
    }

    private void OnProjectsChanged()
    {
        FilteredProjects.Clear();
        foreach (var project in _configService.Projects)
        {
            if (string.IsNullOrEmpty(SearchText) ||
                project.Title.Contains(SearchText, StringComparison.OrdinalIgnoreCase))
            {
                FilteredProjects.Add(project);
            }
        }
    }

    partial void OnSearchTextChanged(string value)
    {
        OnProjectsChanged();
    }

    [RelayCommand]
    private void OpenProject(ProjectConfig config)
    {
        var existing = OpenTabs.FirstOrDefault(t => t.Config.ServerId == config.ServerId
            && t.Config.ServerUrl == config.ServerUrl);
        if (existing != null)
        {
            SelectedTab = existing;
            return;
        }

        var tab = new ProjectPageViewModel(config, _projectService,
            App.Current.Services.GetRequiredService<FileService>(),
            App.Current.Services.GetRequiredService<LocalFileService>());
        OpenTabs.Add(tab);
        SelectedTab = tab;
    }

    [RelayCommand]
    private void CloseTab(ProjectPageViewModel tab)
    {
        tab.Dispose();
        OpenTabs.Remove(tab);
    }
}
```

- [ ] **步骤 2：创建 ProjectPageViewModel（核心业务逻辑）**

这是最复杂的 ViewModel，对应原 Flutter 的 `project_controller.dart`。

```csharp
using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models;
using PublishTool.Models.Local;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class ProjectPageViewModel : ObservableObject, IDisposable
{
    private readonly ProjectService _projectService;
    private readonly FileService _fileService;
    private readonly LocalFileService _localFileService;

    // 从 ProjectConfig 同步的状态
    [ObservableProperty]
    private ProjectConfig _config;

    // 服务端数据
    [ObservableProperty]
    private ServerOsInfoDto? _serverOsInfo;

    [ObservableProperty]
    private string _serverVersion = string.Empty;

    [ObservableProperty]
    private string _latestChangeLog = string.Empty;

    // 本地文件
    [ObservableProperty]
    private ObservableCollection<LocalFileItem> _localFiles = new();

    // 上传队列
    [ObservableProperty]
    private ObservableCollection<UploadFileItem> _uploadFiles = new();

    // 版本信息
    [ObservableProperty]
    private string _newVersion = string.Empty;

    [ObservableProperty]
    private string _changeLogText = string.Empty;

    // 状态
    [ObservableProperty]
    private bool _isUploading;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    [ObservableProperty]
    private bool _isRefreshEnabled = true;

    public ProjectPageViewModel(
        ProjectConfig config,
        ProjectService projectService,
        FileService fileService,
        LocalFileService localFileService)
    {
        _config = config;
        _projectService = projectService;
        _fileService = fileService;
        _localFileService = localFileService;
    }

    [RelayCommand]
    private async Task RefreshStatus()
    {
        IsRefreshEnabled = false;
        StatusMessage = "正在刷新服务端状态...";
        try
        {
            var osInfo = await _projectService.GetOsInfoAsync(Config.ServerUrl, Config.ServerId);
            if (osInfo != null)
            {
                ServerOsInfo = osInfo;
            }

            var logs = await _projectService.GetChangeLogsAsync(Config.ServerUrl, Config.ServerId);
            if (logs.Count > 0)
            {
                var latest = logs[0];
                ServerVersion = latest.Version;
                LatestChangeLog = string.Join("\n", latest.Logs);
            }

            var files = await _fileService.GetAllFilesAsync(Config.ServerUrl, Config.ServerId);
            // 获取服务端文件信息后可以更新本地对比
            var modifiedFiles = _localFileService.GetModifiedFiles(Config.LocalPath, files);
            LocalFiles = new ObservableCollection<LocalFileItem>(modifiedFiles);

            StatusMessage = "刷新完成";
        }
        catch (Exception ex)
        {
            StatusMessage = $"刷新失败: {ex.Message}";
        }
        finally
        {
            IsRefreshEnabled = true;
        }
    }

    [RelayCommand]
    private void ScanLocalFiles()
    {
        var files = _localFileService.ScanDirectory(Config.LocalPath);
        LocalFiles = new ObservableCollection<LocalFileItem>(files);
        StatusMessage = $"扫描到 {files.Count} 个本地文件";
    }

    [RelayCommand]
    private void AddToUploadQueue()
    {
        var checkedFiles = LocalFiles.Where(f => f.IsChecked).ToList();
        var uploadItems = new ObservableCollection<UploadFileItem>();
        foreach (var file in checkedFiles)
        {
            uploadItems.Add(new UploadFileItem
            {
                FileName = file.FileName,
                LocalPath = file.AbsolutePath,
                RelativePath = file.RelativePath,
                LastModified = file.LastModified,
                Status = UploadStatus.Pending
            });
        }
        UploadFiles = uploadItems;
        StatusMessage = $"已添加 {uploadItems.Count} 个文件到上传队列";
    }

    [RelayCommand]
    private async Task PushAll()
    {
        if (UploadFiles.Count == 0) return;
        IsUploading = true;
        try
        {
            foreach (var item in UploadFiles)
            {
                item.Status = UploadStatus.Uploading;
                StatusMessage = $"正在上传: {item.FileName}";

                await using var stream = File.OpenRead(item.LocalPath);
                await _fileService.UploadFileAsync(
                    Config.ServerUrl, Config.Name, item.RelativePath, stream);

                item.Status = UploadStatus.Done;
            }
            StatusMessage = "所有文件上传完成";
        }
        catch (Exception ex)
        {
            StatusMessage = $"上传失败: {ex.Message}";
            foreach (var item in UploadFiles.Where(f => f.Status == UploadStatus.Uploading))
            {
                item.Status = UploadStatus.Failed;
            }
        }
        finally
        {
            IsUploading = false;
        }
    }

    [RelayCommand]
    private async Task DownloadAll()
    {
        StatusMessage = "正在获取服务端文件列表...";
        try
        {
            var files = await _fileService.GetAllFilesAsync(Config.ServerUrl, Config.ServerId);
            StatusMessage = $"正在下载 {files.Count} 个文件...";
            foreach (var file in files)
            {
                var localPath = Path.Combine(Config.LocalPath, file.FileRelativePath);
                var dir = Path.GetDirectoryName(localPath);
                if (dir != null) Directory.CreateDirectory(dir);

                await using var stream = await _fileService.DownloadFileAsync(
                    Config.ServerUrl, file.FileAbsolutePath);
                await using var fs = File.Create(localPath);
                await stream.CopyToAsync(fs);
            }
            StatusMessage = "全部下载完成";
        }
        catch (Exception ex)
        {
            StatusMessage = $"下载失败: {ex.Message}";
        }
    }

    [RelayCommand]
    private async Task PullUpdate()
    {
        // 增量拉取：仅下载差异文件
        StatusMessage = "正在对比文件差异...";
        try
        {
            var serverFiles = await _fileService.GetAllFilesAsync(Config.ServerUrl, Config.ServerId);
            var modifiedFiles = _localFileService.GetModifiedFiles(Config.LocalPath, serverFiles);
            var toDownload = modifiedFiles.Where(f => f.IsModified).ToList();

            StatusMessage = $"发现 {toDownload.Count} 个差异文件，开始下载...";
            foreach (var item in toDownload)
            {
                var serverFile = serverFiles.First(f =>
                    f.FileRelativePath == item.RelativePath);
                var localPath = Path.Combine(Config.LocalPath, item.RelativePath);
                var dir = Path.GetDirectoryName(localPath);
                if (dir != null) Directory.CreateDirectory(dir);

                await using var stream = await _fileService.DownloadFileAsync(
                    Config.ServerUrl, serverFile.FileAbsolutePath);
                await using var fs = File.Create(localPath);
                await stream.CopyToAsync(fs);
            }
            StatusMessage = "增量拉取完成";
        }
        catch (Exception ex)
        {
            StatusMessage = $"拉取失败: {ex.Message}";
        }
    }

    [RelayCommand]
    private void Stop()
    {
        // 实际停止逻辑需要通过 CancellationToken 实现
        StatusMessage = "操作已停止";
    }

    [RelayCommand]
    private void AutoGenerateVersion()
    {
        NewVersion = DateTime.Now.ToString("yyyyMMdd-HHmm");
    }

    public void Dispose()
    {
        // 清理资源
    }
}
```

- [ ] **步骤 3：创建对话框 ViewModel**

```csharp
// AddProjectDialogViewModel.cs
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models;
using PublishTool.Models.Local;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class AddProjectDialogViewModel : ObservableObject
{
    private readonly ProjectService _projectService;
    private readonly ConfigService _configService;

    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    private List<ProjectDto> _availableProjects = new();

    [ObservableProperty]
    private ProjectDto? _selectedProject;

    [ObservableProperty]
    private string _localPath = string.Empty;

    [ObservableProperty]
    private string? _exePath;

    public AddProjectDialogViewModel(ProjectService projectService, ConfigService configService)
    {
        _projectService = projectService;
        _configService = configService;
    }

    [RelayCommand]
    private async Task FetchProjects()
    {
        AvailableProjects = await _projectService.GetAllProjectsAsync(ServerUrl);
    }

    [RelayCommand]
    private void SelectLocalFolder() { /* 调用文件选择器 */ }

    [RelayCommand]
    private void SelectExeFile() { /* 调用文件选择器 */ }

    [RelayCommand]
    private void Confirm()
    {
        if (SelectedProject == null) return;
        var config = new ProjectConfig
        {
            ServerId = SelectedProject.Id,
            Name = SelectedProject.Name,
            Title = SelectedProject.Title,
            ServerUrl = ServerUrl,
            LocalPath = LocalPath,
            ExePath = ExePath
        };
        _configService.AddProject(config);
    }
}
```

---

### 任务 7：主窗口视图

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Views/MainWindow.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/MainWindow.axaml.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/ViewLocator.cs`

- [ ] **步骤 1：创建 ViewLocator**

```csharp
using Avalonia.Controls;
using Avalonia.DataTemplate;

namespace PublishTool;

public class ViewLocator : IDataTemplate
{
    public Control? Build(object? param)
    {
        if (param is null) return null;

        var viewModelType = param.GetType();
        var viewTypeName = viewModelType.FullName!.Replace("ViewModels", "Views")
            .Replace("ViewModel", "View");
        var viewType = Type.GetType(viewTypeName);

        if (viewType != null)
        {
            return (Control)Activator.CreateInstance(viewType)!;
        }

        return new TextBlock { Text = $"View not found: {viewTypeName}" };
    }

    public bool Match(object? data) => data is ViewModelBase;
}
```

- [ ] **步骤 2：创建 MainWindow.axaml（主窗口布局）**

左侧项目列表面板（260px）+ 右侧内容区（TabBar + ContentControl 多页面）+ 底部状态栏。

```xml
<!-- MainWindow.axaml -->
<Window xmlns="https://github.com/avaloniaui"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        xmlns:vm="using:PublishTool.ViewModels"
        xmlns:views="using:PublishTool.Views"
        xmlns:controls="using:PublishTool.Views.Controls"
        x:Class="PublishTool.Views.MainWindow"
        Title="长飞客户端软件版本发布工具"
        Width="1200" Height="800"
        Background="{DynamicResource WindowBackgroundBrush}">
  <Grid RowDefinitions="Auto,*,Auto">
    <!-- 顶部标题栏 -->
    <Border Grid.Row="0" Padding="8" Background="{DynamicResource RegionBrush}">
      <TextBlock Text="长飞客户端软件版本发布工具"
                 FontSize="16" FontWeight="Bold"/>
    </Border>

    <!-- 主体区域：左侧列表 + 右侧内容 -->
    <Grid Grid.Row="1" ColumnDefinitions="260,*">
      <!-- 左侧项目列表面板 -->
      <controls:ProjectListPanel Grid.Column="0" />

      <!-- 右侧内容区 -->
      <Grid Grid.Column="1" RowDefinitions="Auto,*">
        <controls:ProjectTabBar Grid.Row="0" />
        <ContentControl Grid.Row="1"
                        Content="{Binding SelectedTab}"
                        DataTemplate="{x:Static UserControl.DataTemplate}">
        </ContentControl>
      </Grid>
    </Grid>

    <!-- 底部状态栏 -->
    <controls:StatusBar Grid.Row="2" />
  </Grid>
</Window>
```

```csharp
// MainWindow.axaml.cs
using Avalonia.Controls;
using PublishTool.ViewModels;

namespace PublishTool.Views;

public partial class MainWindow : Window
{
    public MainWindow()
    {
        InitializeComponent();
    }
}
```

- [ ] **步骤 3：创建 ProjectListPanel**

项目列表控件包含：两个新建按钮（服务端/客户端）、搜索框、项目卡片列表。

```xml
<!-- ProjectListPanel.axaml -->
<UserControl xmlns="https://github.com/avaloniaui"
             xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
             x:Class="PublishTool.Views.Controls.ProjectListPanel">
  <Border BorderBrush="{DynamicResource ControlStrokeColorSecondary}"
          BorderThickness="0,0,1,0">
    <Grid RowDefinitions="Auto,Auto,*">
      <!-- 标题 + 新建按钮 -->
      <StackPanel Grid.Row="0" Orientation="Horizontal" Spacing="4" Margin="8">
        <Button Content="+服务端" />
        <Button Content="+客户端" />
      </StackPanel>

      <!-- 搜索框 -->
      <TextBox Grid.Row="1" Margin="8,4"
               Watermark="搜索项目..."
               Text="{Binding SearchText}" />

      <!-- 项目列表 -->
      <ListBox Grid.Row="2" Margin="4"
               ItemsSource="{Binding FilteredProjects}"
               SelectedItem="{Binding SelectedProject}">
        <ListBox.ItemTemplate>
          <DataTemplate>
            <controls:ProjectCard />
          </DataTemplate>
        </ListBox.ItemTemplate>
      </ListBox>
    </Grid>
  </Border>
</UserControl>
```

---

### 任务 8：项目页面视图

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Controls/ServerInfoBar.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Controls/ToolbarBar.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Controls/LocalFilesPanel.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Controls/OperationButtons.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Controls/UploadQueuePanel.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Views/Controls/BottomActionBar.axaml`

- [ ] **步骤 1：创建单个 ProjectPage**

组合所有子控件：

```xml
<!-- ProjectPage.axaml -->
<UserControl xmlns="https://github.com/avaloniaui"
             xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
             xmlns:controls="using:PublishTool.Views.Controls"
             x:Class="PublishTool.Views.Controls.ProjectPage">
  <Grid Row Definitions="Auto,Auto,*,Auto,Auto,Auto">
    <!-- 服务器信息栏 -->
    <controls:ServerInfoBar Grid.Row="0" />

    <!-- 工具栏 -->
    <controls:ToolbarBar Grid.Row="1" />

    <!-- 主要内容区 -->
    <Grid Grid.Row="2" ColumnDefinitions="*,Auto">
      <!-- 本地文件面板 -->
      <controls:LocalFilesPanel Grid.Column="0" />
      <!-- 操作按钮 -->
      <controls:OperationButtons Grid.Column="1" />
    </Grid>

    <!-- 上传队列面板 -->
    <controls:UploadQueuePanel Grid.Row="3" />

    <!-- 底部操作栏 -->
    <controls:BottomActionBar Grid.Row="4" />
  </Grid>
</UserControl>
```

---

### 任务 9：对话框视图

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Dialogs/AddProjectDialog.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Dialogs/AddServerProjectDialog.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Dialogs/ProjectSettingsDialog.axaml`
- 创建：`publish_tool_avalonia/src/PublishTool/Views/Dialogs/ConfigEditorDialog.axaml`

- [ ] **步骤 1：创建 AddProjectDialog**

输入服务器地址 → 获取项目列表 → 选择项目 → 选择本地文件夹 → 确认。

```xml
<!-- AddProjectDialog.axaml -->
<Window xmlns="https://github.com/avaloniaui"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        x:Class="PublishTool.Views.Dialogs.AddProjectDialog"
        Title="添加项目" Width="500" SizeToContent="Height"
        Background="{DynamicResource WindowBackgroundBrush}">
  <StackPanel Spacing="12" Margin="16">
    <TextBox Watermark="服务器地址"
             Text="{Binding ServerUrl}" />
    <Button Content="获取项目列表"
            Command="{Binding FetchProjectsCommand}" />

    <ListBox ItemsSource="{Binding AvailableProjects}"
             SelectedItem="{Binding SelectedProject}"
             Height="200">
      <ListBox.ItemTemplate>
        <DataTemplate>
          <StackPanel>
            <TextBlock Text="{Binding Title}" FontWeight="Bold" />
            <TextBlock Text="{Binding Name}" Foreground="Gray" />
          </StackPanel>
        </DataTemplate>
      </ListBox.ItemTemplate>
    </ListBox>

    <StackPanel Orientation="Horizontal" Spacing="8">
      <TextBox Text="{Binding LocalPath}"
               Watermark="本地文件夹路径" Width="*" />
      <Button Content="浏览..."
              Command="{Binding SelectLocalFolderCommand}" />
    </StackPanel>

    <StackPanel Orientation="Horizontal" Spacing="8">
      <TextBox Text="{Binding ExePath}"
               Watermark="EXE 路径（可选）" Width="*" />
      <Button Content="浏览..."
              Command="{Binding SelectExeFileCommand}" />
    </StackPanel>

    <Button Content="确认添加"
            Command="{Binding ConfirmCommand}"
            HorizontalAlignment="Right" />
  </StackPanel>
</Window>
```

---

### 任务 10：值转换器和工具类

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool/Converters/BoolToColorConverter.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Converters/FileStatusToColorConverter.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Converters/BytesToSizeConverter.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Helpers/Md5Helper.cs`
- 创建：`publish_tool_avalonia/src/PublishTool/Helpers/LogHelper.cs`

- [ ] **步骤 1：创建 BytesToSizeConverter**

```csharp
using Avalonia.Data.Converters;
using System.Globalization;

namespace PublishTool.Converters;

public class BytesToSizeConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is long bytes)
        {
            string[] suffixes = { "B", "KB", "MB", "GB", "TB" };
            int order = 0;
            double size = bytes;
            while (size >= 1024 && order < suffixes.Length - 1)
            {
                order++;
                size /= 1024;
            }
            return $"{size:0.##} {suffixes[order]}";
        }
        return "0 B";
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
```

---

### 任务 11：项目测试

**文件：**
- 创建：`publish_tool_avalonia/src/PublishTool.Tests/PublishTool.Tests.csproj`
- 创建：`publish_tool_avalonia/src/PublishTool.Tests/ViewModels/ProjectPageViewModelTests.cs`

- [ ] **步骤 1：创建测试项目 csproj**

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="xunit" Version="2.9.+" />
    <PackageReference Include="Moq" Version="4.20.+" />
    <PackageReference Include="Shouldly" Version="4.2.+" />
    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.12.+" />
  </ItemGroup>
  <ItemGroup>
    <ProjectReference Include="..\PublishTool\PublishTool.csproj" />
  </ItemGroup>
</Project>
```

- [ ] **步骤 2：创建 ViewModel 测试**

```csharp
using Moq;
using PublishTool.Models.Local;
using PublishTool.Services;
using PublishTool.ViewModels;
using Shouldly;

namespace PublishTool.Tests.ViewModels;

public class ProjectPageViewModelTests
{
    [Fact]
    public async Task RefreshStatus_ShouldUpdateServerVersion()
    {
        var projectService = new Mock<ProjectService>();
        var config = new ProjectConfig
        {
            ServerId = 1Name = "test-project",
            ServerUrl = "http://localhost:2000",
            LocalPath = @"C:\test"
        };

        var vm = new ProjectPageViewModel(
            config,
            projectService.Object,
            Mock.Of<FileService>(),
            Mock.Of<LocalFileService>());

        // 执行
        await vm.RefreshStatusCommand.ExecuteAsync(null);

        // 验证
        vm.IsRefreshEnabled.ShouldBeTrue();
    }
}
```

---

## 执行顺序

1. **任务 1** → 项目骨架（sln + csproj + App + Program）
2. **任务 2** → DTO 数据模型（7 个模型类）
3. **任务 3** → 本地业务模型（3 个 ObservableObject）
4. **任务 4** → HTTP API 服务层（ProjectService + FileService）
5. **任务 5** → 本地服务层（ConfigService + LocalFileService + LocalFileService + ProcessService）
6. **任务 6** → ViewModel 层（MainWindow + ProjectPage + 对话框 ViewModel）
7. **任务 7** → 主窗口视图（MainWindow + ProjectListPanel + StatusBar）
8. **任务 8** → 项目页面视图（6 个子控件）
9. **任务 9** → 对话框视图（4 个对话框）
10. **任务 10** → 值转换器和工具类
11. **任务 11** → iewModel 单元测试

---

## 兼容性说明

- 所有 HTTP API 接口与现有 Go 服务端完全兼容，无需修改服务端代码
- JSON 序列化字段名使用小驼峰（与 Go 服务端保持一致）
- 上传/下载接口使用 multipart/form-data 格式（与现有 Flutter 客户端一致）
- 本地配置存储路径为 `{LocalApplicationData}/PublishTool/publish_tool_config.json`
- 日志路径为 `{ApplicationData}/PublishTool/logs/log.log`