using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishGui.Constants;
using PublishGui.Models.Cli;
using PublishGui.Models.Local;
using PublishGui.Services;
using PublishGui.Views.Dialogs;
using Serilog;
using System;
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using System.Threading.Tasks;

namespace PublishGui.ViewModels;

/// <summary>
/// 主窗口 ViewModel
/// 负责管理项目列表、服务器信息和全局操作
/// </summary>
public partial class MainWindowViewModel : ObservableObject
{
    private readonly CliService _cliService;
    private readonly ConfigService _configService;

    /// <summary>项目列表</summary>
    [ObservableProperty]
    private ObservableCollection<ProjectViewModel> _projects = new();

    /// <summary>当前选中的项目</summary>
    [ObservableProperty]
    private ProjectViewModel? _selectedProject;

    /// <summary>状态栏消息</summary>
    [ObservableProperty]
    private string _statusMessage = UiTexts.Ready;

    /// <summary>是否找到 publish-cli</summary>
    [ObservableProperty]
    private bool _isCliFound;

    /// <summary>publish-cli 路径</summary>
    [ObservableProperty]
    private string _cliPath = string.Empty;

    /// <summary>服务器信息</summary>
    [ObservableProperty]
    private ServerOsInfo? _serverInfo;

    public MainWindowViewModel(CliService cliService, ConfigService configService)
    {
        _cliService = cliService;
        _configService = configService;
        
        _ = InitializeAsync();
    }

    /// <summary>
    /// 初始化应用程序
    /// </summary>
    private async Task InitializeAsync()
    {
        SetupLogging();
        await FindCliAsync();
        LoadProjects();
    }

    /// <summary>
    /// 配置日志
    /// </summary>
    private void SetupLogging()
    {
        var logDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            AppConstants.ConfigDirectoryName,
            AppConstants.LogDirectoryName);
        
        Directory.CreateDirectory(logDir);
        
        Log.Logger = new LoggerConfiguration()
            .WriteTo.File(
                Path.Combine(logDir, AppConstants.LogFileName), 
                rollingInterval: RollingInterval.Day)
            .CreateLogger();
    }

    /// <summary>
    /// 查找 publish-cli
    /// </summary>
    private async Task FindCliAsync()
    {
        var cliPath = await _cliService.FindCliAsync();
        
        if (cliPath != null)
        {
            IsCliFound = true;
            CliPath = cliPath;
            StatusMessage = $"{UiTexts.CliFound}: {cliPath}";
        }
        else
        {
            IsCliFound = false;
            StatusMessage = UiTexts.CliNotFound;
        }
    }

    /// <summary>
    /// 加载已保存的项目
    /// </summary>
    private async void LoadProjects()
    {
        var configs = _configService.LoadProjects();
        
        foreach (var config in configs)
        {
            var vm = new ProjectViewModel(config, _cliService, _configService);
            Projects.Add(vm);
        }

        if (Projects.Count > 0)
        {
            SelectedProject = Projects[0];
        }
        else
        {
            // 首次运行，显示向导
            await ShowFirstRunWizardAsync();
        }
    }

    /// <summary>
    /// 显示首次运行向导
    /// </summary>
    private async Task ShowFirstRunWizardAsync()
    {
        var mainWindow = GetMainWindow();
        if (mainWindow == null) return;

        var wizard = new FirstRunWizard();
        var result = await wizard.ShowDialog<WizardResult?>(mainWindow);
        
        if (result == null) return;

        if (result.Mode == WizardResultMode.AddExisting)
        {
            await AddProjectFromWizardAsync(result);
        }
        else if (result.Mode == WizardResultMode.CreateNew)
        {
            await CreateProjectFromWizardAsync(result);
        }
    }

    /// <summary>
    /// 从向导结果添加已有项目
    /// </summary>
    private async Task AddProjectFromWizardAsync(WizardResult result)
    {
        var config = new ProjectConfig
        {
            ProjectName = result.ProjectName,
            ServerUrl = result.ServerUrl,
            ProjectPath = result.LocalPath,
            ProjectId = result.ProjectId
        };

        _configService.AddProject(config);
        
        var vm = new ProjectViewModel(config, _cliService, _configService);
        Projects.Add(vm);
        SelectedProject = vm;
        
        StatusMessage = $"{UiTexts.ProjectAdded}: {result.ProjectName}";
    }

    /// <summary>
    /// 从向导结果创建新项目
    /// </summary>
    private async Task CreateProjectFromWizardAsync(WizardResult result)
    {
        StatusMessage = UiTexts.CreatingProject;

        try
        {
            // 1. 在服务端创建项目
            var createResult = await _cliService.ProjectCreateAsync(
                result.ServerUrl, 
                result.ProjectName, 
                result.ProjectTitle, 
                result.ForceUpdate);
                
            if (createResult == null || !createResult.IsSuccess)
            {
                StatusMessage = $"{UiTexts.CreateProjectFailed}: {createResult?.ErrorMsg ?? "未知错误"}";
                return;
            }

            var projectId = createResult.Data?.Id ?? 0;

            // 2. 初始化本地配置
            var initResult = await _cliService.ConfigInitAsync(
                result.LocalPath, 
                result.ServerUrl, 
                result.ProjectName, 
                projectId);
                
            if (initResult == null || !initResult.IsSuccess)
            {
                StatusMessage = $"{UiTexts.InitConfigFailed}: {initResult?.ErrorMsg ?? "未知错误"}";
                return;
            }

            // 3. 添加到项目列表
            var config = new ProjectConfig
            {
                ProjectName = result.ProjectName,
                ServerUrl = result.ServerUrl,
                ProjectPath = result.LocalPath,
                ProjectId = projectId
            };

            _configService.AddProject(config);
            
            var vm = new ProjectViewModel(config, _cliService, _configService);
            Projects.Add(vm);
            SelectedProject = vm;
            
            StatusMessage = $"{UiTexts.ProjectCreated}: {result.ProjectName} (ID: {projectId})";
        }
        catch (Exception ex)
        {
            StatusMessage = $"{UiTexts.CreateProjectFailed}: {ex.Message}";
            Log.Error(ex, "创建项目失败");
        }
    }

    /// <summary>
    /// 添加已有项目
    /// </summary>
    [RelayCommand]
    private async Task AddExistingProjectAsync()
    {
        var dialog = new AddProjectDialog();
        var mainWindow = GetMainWindow();
        if (mainWindow == null) return;

        var result = await dialog.ShowDialog<(string Name, string ServerUrl, string Path, int Id)?>(mainWindow);
        if (result == null) return;

        var (name, serverUrl, path, id) = result.Value;

        var config = new ProjectConfig
        {
            ProjectName = name,
            ServerUrl = serverUrl,
            ProjectPath = path,
            ProjectId = id
        };

        _configService.AddProject(config);
        
        var vm = new ProjectViewModel(config, _cliService, _configService);
        Projects.Add(vm);
        SelectedProject = vm;
        
        StatusMessage = $"{UiTexts.ProjectAdded}: {name}";
    }

    /// <summary>
    /// 新建项目（在服务端创建）
    /// </summary>
    [RelayCommand]
    private async Task AddNewProjectAsync()
    {
        var dialog = new CreateProjectDialog();
        var mainWindow = GetMainWindow();
        if (mainWindow == null) return;

        var result = await dialog.ShowDialog<(string ServerUrl, string Name, string Title, bool ForceUpdate, string Path)?>(mainWindow);
        if (result == null) return;

        var (serverUrl, name, title, forceUpdate, path) = result.Value;

        StatusMessage = UiTexts.CreatingProject;

        try
        {
            // 1. 在服务端创建项目
            var createResult = await _cliService.ProjectCreateAsync(serverUrl, name, title, forceUpdate);
            if (createResult == null || !createResult.IsSuccess)
            {
                StatusMessage = $"{UiTexts.CreateProjectFailed}: {createResult?.ErrorMsg ?? "未知错误"}";
                return;
            }

            var projectId = createResult.Data?.Id ?? 0;

            // 2. 初始化本地配置
            var initResult = await _cliService.ConfigInitAsync(path, serverUrl, name, projectId);
            if (initResult == null || !initResult.IsSuccess)
            {
                StatusMessage = $"{UiTexts.InitConfigFailed}: {initResult?.ErrorMsg ?? "未知错误"}";
                return;
            }

            // 3. 添加到项目列表
            var config = new ProjectConfig
            {
                ProjectName = name,
                ServerUrl = serverUrl,
                ProjectPath = path,
                ProjectId = projectId
            };

            _configService.AddProject(config);
            
            var vm = new ProjectViewModel(config, _cliService, _configService);
            Projects.Add(vm);
            SelectedProject = vm;
            
            StatusMessage = $"{UiTexts.ProjectCreated}: {name} (ID: {projectId})";
        }
        catch (Exception ex)
        {
            StatusMessage = $"{UiTexts.CreateProjectFailed}: {ex.Message}";
            Log.Error(ex, "创建项目失败");
        }
    }

    /// <summary>
    /// 移除当前项目
    /// </summary>
    [RelayCommand]
    private async Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return;

        var name = SelectedProject.Config.ProjectName;
        
        _configService.RemoveProject(name);
        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        
        StatusMessage = $"{UiTexts.ProjectRemoved}: {name}";
    }

    /// <summary>
    /// 刷新服务器信息
    /// </summary>
    [RelayCommand]
    private async Task RefreshServerInfoAsync()
    {
        if (SelectedProject == null || string.IsNullOrEmpty(SelectedProject.Config.ServerUrl))
            return;

        try
        {
            var result = await _cliService.ServerInfoAsync(
                SelectedProject.Config.ServerUrl,
                SelectedProject.Config.ProjectId);

            if (result?.Data != null && result.Data.Count > 0)
            {
                ServerInfo = result.Data[0];
                StatusMessage = $"服务器: {ServerInfo.OS} {ServerInfo.Platform}, CPU: {ServerInfo.CpuName}";
            }
        }
        catch (Exception ex)
        {
            StatusMessage = $"{UiTexts.GetServerInfoFailed}: {ex.Message}";
            Log.Error(ex, "获取服务器信息失败");
        }
    }

    /// <summary>
    /// 打开项目设置
    /// </summary>
    [RelayCommand]
    private async Task OpenProjectSettingsAsync()
    {
        if (SelectedProject == null) return;

        var dialog = new ProjectSettingsDialog();
        dialog.LoadConfig(SelectedProject.Config);

        var mainWindow = GetMainWindow();
        if (mainWindow == null) return;

        var result = await dialog.ShowDialog<ProjectConfig?>(mainWindow);
        if (result == null) return;

        // 更新配置
        _configService.UpdateProject(result);
        SelectedProject.Config = result;
        
        StatusMessage = $"{UiTexts.SettingsUpdated}: {result.ProjectName}";
    }

    /// <summary>
    /// 获取主窗口
    /// </summary>
    private static Window? GetMainWindow()
    {
        return (Application.Current?.ApplicationLifetime as IClassicDesktopStyleApplicationLifetime)?.MainWindow;
    }
}
