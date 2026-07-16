using System;
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using Serilog;
using Ursa.Controls;

namespace AlyPublish.ViewModels;

public partial class AddProjectDialogViewModel : ObservableObject
{
    private readonly CliService _cli;
    private readonly ConfigService _configService;
    internal Func<Task<string?>>? BrowseFolderAsync { get; set; }
    internal Func<Task<string?>>? BrowseMainExeFileAsync { get; set; }

    private bool _isInit;
    [ObservableProperty] private string _serverUrl = string.Empty;
    [ObservableProperty] private ObservableCollection<ProjectInfo> _serverProjects = new();
    [ObservableProperty] private ProjectInfo? _selectedServerProject;
    [ObservableProperty] private string _displayName = string.Empty;
    [ObservableProperty] private string _projectPath = string.Empty;
    [ObservableProperty] private string _statusMessage = string.Empty;
    [ObservableProperty] private bool _isFetching;
    [ObservableProperty] private bool _showProjectList;
    [ObservableProperty] private bool _showCreateButton;
    [ObservableProperty] private string _initStatusMessage = string.Empty;
    [ObservableProperty] private bool _showInitStatus;
    [ObservableProperty] private string _ignoreFolders = string.Empty;
    [ObservableProperty] private string _ignoreFiles = string.Empty;

    // ── 主程序选择 ─────────────────────────────────────
    [ObservableProperty] private string _mainExePath = string.Empty;
    [ObservableProperty] private string _closeProcessName = string.Empty;

    /// <summary>View 订阅此事件以关闭对话框并返回结果</summary>
    public event Action<ProjectConfig?>? RequestClose;

    public IAsyncRelayCommand FetchProjectsCommand { get; }
    public IAsyncRelayCommand BrowseCommand { get; }
    public IAsyncRelayCommand BrowseMainExeCommand { get; }
    public IAsyncRelayCommand ConfirmCommand { get; }
    public IRelayCommand CancelCommand { get; }
    public IAsyncRelayCommand CreateOnServerCommand { get; }

    public AddProjectDialogViewModel(CliService cli, ConfigService configService)
    {
        _cli = cli;
        _configService = configService;

        FetchProjectsCommand = new AsyncRelayCommand(FetchProjectsAsync, () => !IsFetching);
        BrowseCommand = new AsyncRelayCommand(BrowseAsync);
        BrowseMainExeCommand = new AsyncRelayCommand(BrowseMainExeAsync);
        ConfirmCommand = new AsyncRelayCommand(ConfirmAsync);
        CancelCommand = new RelayCommand(() => RequestClose?.Invoke(null));
        CreateOnServerCommand = new AsyncRelayCommand(CreateOnServerAsync);
    }

    partial void OnIsFetchingChanged(bool value)
    {
        FetchProjectsCommand.NotifyCanExecuteChanged();
    }

    partial void OnSelectedServerProjectChanged(ProjectInfo? value)
    {
        if (value == null) return;
        if (string.IsNullOrWhiteSpace(DisplayName))
            DisplayName = value.Name;
        Log.Information("选择了服务端项目: Name={Name}, Id={Id}", value.Name, value.Id);
    }

    async partial void OnProjectPathChanged(string value)
    {
        if (string.IsNullOrWhiteSpace(value) || !Directory.Exists(value))
        {
            _isInit = false;
            ShowInitStatus = false;
            return;
        }

        var sharedPath = Path.Combine(value, ".updator", "shared.json");
        if (File.Exists(sharedPath))
        {
            _isInit = true;
            InitStatusMessage = "❌ 该文件夹已初始化，请使用「添加本地项目」功能";
            ShowInitStatus = true;
        }
        else
        {
            _isInit = false;
            InitStatusMessage = "";
            ShowInitStatus = false;

            // 自动扫描路径下第一个 exe 文件作为主程序
            TryAutoDetectMainExe(value);
        }
    }

    /// <summary>自动扫描项目目录下第一个 .exe 文件</summary>
    private void TryAutoDetectMainExe(string projectPath)
    {
        try
        {
            var firstExe = Directory.EnumerateFiles(projectPath, "*.exe", SearchOption.TopDirectoryOnly)
                .FirstOrDefault();
            if (firstExe != null)
            {
                MainExePath = firstExe;
                CloseProcessName = Path.GetFileNameWithoutExtension(firstExe);
                Log.Information("自动检测主程序: {Path}, 进程名: {Name}", firstExe, CloseProcessName);
            }
        }
        catch (Exception ex)
        {
            Log.Warning(ex, "扫描主程序 exe 失败: {Path}", projectPath);
        }
    }

    /// <summary>浏览选择主程序 exe 文件</summary>
    private async Task BrowseMainExeAsync()
    {
        var browseMainExeFileAsync = BrowseMainExeFileAsync;
        if (browseMainExeFileAsync == null)
        {
            Log.Warning("BrowseMainExeFileAsync delegate is not set; browse action unavailable");
            return;
        }
        var path = await browseMainExeFileAsync();
        if (!string.IsNullOrWhiteSpace(path))
        {
            MainExePath = path;
            CloseProcessName = Path.GetFileNameWithoutExtension(path);
            Log.Information("用户选择主程序: {Path}, 进程名: {Name}", path, CloseProcessName);
        }
    }

    private async Task BrowseAsync()
    {
        var browseFolderAsync = BrowseFolderAsync;
        if (browseFolderAsync == null)
        {
            Log.Warning("BrowseFolderAsync delegate is not set; browse action unavailable");
            return;
        }
        var path = await browseFolderAsync();
        if (!string.IsNullOrWhiteSpace(path))
            ProjectPath = path;
    }

    private async Task FetchProjectsAsync()
    {
        var serverUrl = ServerUrl?.Trim();
        if (string.IsNullOrEmpty(serverUrl))
        {
            await MessageBox.ShowAsync("请先填写服务端地址", "提示", MessageBoxIcon.Warning);
            return;
        }

        IsFetching = true;
        StatusMessage = "正在获取项目列表...";
        Log.Information("获取服务端项目列表: {ServerUrl}", serverUrl);

        try
        {
            var result = await _cli.ProjectListAsync(serverUrl);
            Log.Information("获取项目结果: IsSuccess={IsSuccess}, ErrorMsg={ErrorMsg}, Data={Data}",
                result?.IsSuccess, result?.ErrorMsg,
                result?.Data != null ? $"{result.Data.Count} projects" : "null");

            if (result == null)
            {
                await MessageBox.ShowAsync("未收到服务端响应，请检查地址是否正确", "错误", MessageBoxIcon.Error);
                ShowCreateButton = true;
                ShowProjectList = false;
                return;
            }

            if (!result.IsSuccess)
            {
                await MessageBox.ShowAsync($"获取失败: {result.ErrorMsg}", "错误", MessageBoxIcon.Error);
                ShowCreateButton = true;
                ShowProjectList = false;
                return;
            }

            var projects = result.Data ?? new();
            ServerProjects = new ObservableCollection<ProjectInfo>(projects);
            ShowProjectList = projects.Count > 0;
            ShowCreateButton = true;
            if (projects.Count == 0)
                StatusMessage = "服务端暂无项目，可点击下方按钮创建";
            else
                StatusMessage = string.Empty;
            Log.Information("获取到 {Count} 个项目", projects.Count);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "获取服务端项目异常: {ServerUrl}", serverUrl);
            await MessageBox.ShowAsync($"获取异常: {ex.Message}", "错误", MessageBoxIcon.Error);
            ShowCreateButton = true;
            ShowProjectList = false;
        }
        finally
        {
            IsFetching = false;
        }
    }

    private async Task CreateOnServerAsync()
    {
        var serverUrl = ServerUrl?.Trim();
        if (string.IsNullOrEmpty(serverUrl))
        {
            await MessageBox.ShowAsync("请先填写服务端地址", "提示", MessageBoxIcon.Warning);
            return;
        }

        Log.Information("请求创建服务端项目: ServerUrl={ServerUrl}", serverUrl);
        if (ShowCreateProjectDialogAsync == null)
        {
            await MessageBox.ShowAsync("无法打开创建项目对话框", "错误", MessageBoxIcon.Error);
            return;
        }

        var newProject = await ShowCreateProjectDialogAsync.Invoke(serverUrl,
            IgnoreFolders?.Trim() ?? "", IgnoreFiles?.Trim() ?? "");

        if (newProject != null)
        {
            DisplayName = newProject.Name;
            await MessageBox.ShowAsync($"项目 \"{newProject.Name}\" 创建成功", "成功", MessageBoxIcon.Success);
            Log.Information("创建项目完成，自动填充: Name={Name}, Id={Id}", newProject.Name, newProject.Id);
            await FetchProjectsAsync();
            // 自动选中刚创建的项目
            var created = ServerProjects.FirstOrDefault(p => p.Name == newProject.Name);
            if (created != null)
                SelectedServerProject = created;
        }
    }

    /// <summary>由调用方注入，用于打开创建项目子对话框</summary>
    public Func<string, string, string, Task<ProjectInfo?>>? ShowCreateProjectDialogAsync { get; set; }

    private async Task ConfirmAsync()
    {
        var displayName = DisplayName?.Trim();
        var projectPath = ProjectPath?.Trim();

        if (string.IsNullOrWhiteSpace(displayName) ||
            string.IsNullOrWhiteSpace(projectPath))
        {
            await MessageBox.ShowAsync("请填写显示名称和本地路径", "提示", MessageBoxIcon.Warning);
            return;
        }

        if (!Directory.Exists(projectPath))
        {
            await MessageBox.ShowAsync("指定的路径不存在", "提示", MessageBoxIcon.Warning);
            return;
        }

        if (_isInit)
        {
            await MessageBox.ShowAsync("该文件夹已初始化，请使用「添加本地项目」功能", "提示", MessageBoxIcon.Warning);
            return;
        }

        var serverUrl = ServerUrl?.Trim();
        if (string.IsNullOrWhiteSpace(serverUrl) || SelectedServerProject == null || string.IsNullOrWhiteSpace(SelectedServerProject.Name))
        {
            await MessageBox.ShowAsync("请填写服务端地址并选择/创建服务端项目", "提示", MessageBoxIcon.Warning);
            return;
        }

        Log.Information("执行 config init: Server={Server}, Project={Name}, Path={Path}",
            serverUrl, SelectedServerProject.Name, projectPath);
        var initResult = await _cli.ConfigInitAsync(projectPath, serverUrl, SelectedServerProject.Name,
            IgnoreFolders?.Trim() ?? "", IgnoreFiles?.Trim() ?? "");
        if (initResult?.IsSuccess != true)
        {
            await MessageBox.ShowAsync($"初始化 .updator/ 失败: {initResult?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            return;
        }

        var cfg = new ProjectConfig
        {
            DisplayName = displayName!,
            ProjectPath = projectPath,
            MainExePath = MainExePath
        };

        Log.Information("项目初始化完成: DisplayName={Name}, Path={Path}, Server={Server}, Project={Proj}",
            cfg.DisplayName, cfg.ProjectPath, serverUrl, SelectedServerProject.Name);

        // 创建 UpdateFolder/client.json 并复制 aly-client.exe
        if (!string.IsNullOrWhiteSpace(MainExePath) && File.Exists(MainExePath))
        {
            _configService.CreateUpdateFolder(projectPath, MainExePath, CloseProcessName);
        }
        else
        {
            Log.Warning("未选择主程序，跳过 UpdateFolder 创建");
        }

        RequestClose?.Invoke(cfg);
    }
}
