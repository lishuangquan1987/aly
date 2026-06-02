using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
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

public partial class MainWindowViewModel : ObservableObject
{
    private readonly CliService _cliService;
    private readonly ConfigService _configService;

    [ObservableProperty]
    private ObservableCollection<ProjectViewModel> _projects = new();

    [ObservableProperty]
    private ProjectViewModel? _selectedProject;

    [ObservableProperty]
    private string _statusMessage = "就绪";

    [ObservableProperty]
    private bool _isCliFound;

    [ObservableProperty]
    private string _cliPath = string.Empty;

    [ObservableProperty]
    private ServerOsInfo? _serverInfo;

    public MainWindowViewModel(CliService cliService, ConfigService configService)
    {
        _cliService = cliService;
        _configService = configService;

        _ = InitializeAsync();
    }

    private async Task InitializeAsync()
    {
        // Setup logging
        var logDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "PublishGui", "logs");
        Directory.CreateDirectory(logDir);
        Log.Logger = new LoggerConfiguration()
            .WriteTo.File(Path.Combine(logDir, "log.log"), rollingInterval: RollingInterval.Day)
            .CreateLogger();

        // Find publish-cli
        var cliPath = await _cliService.FindCliAsync();
        if (cliPath != null)
        {
            IsCliFound = true;
            CliPath = cliPath;
            StatusMessage = $"已找到 publish-cli: {cliPath}";
        }
        else
        {
            IsCliFound = false;
            StatusMessage = "未找到 publish-cli，请配置路径";
        }

        // Load saved projects
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
    }

    [RelayCommand]
    private async Task AddExistingProjectAsync()
    {
        var dialog = new AddProjectDialog();
        var mainWindow = GetMainWindow();
        if (mainWindow == null) return;

        var result = await dialog.ShowDialog<(string Name, string ServerUrl, string Path, int Id)?>(mainWindow);
        if (result == null) return;

        var name = result.Value.Name;
        var serverUrl = result.Value.ServerUrl;
        var path = result.Value.Path;
        var id = result.Value.Id;

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
        StatusMessage = $"已添加项目: {name}";
    }

    [RelayCommand]
    private async Task AddNewProjectAsync()
    {
        // TODO: Show wizard for creating new project on server
        StatusMessage = "新建项目向导待实现";
    }

    [RelayCommand]
    private async Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return;

        var name = SelectedProject.Config.ProjectName;
        _configService.RemoveProject(name);
        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        StatusMessage = $"已移除项目: {name}";
    }

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
            StatusMessage = $"获取服务器信息失败: {ex.Message}";
            Log.Error(ex, "Failed to get server info");
        }
    }

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

        // Update config
        _configService.UpdateProject(result);
        SelectedProject.Config = result;
        StatusMessage = $"已更新项目设置: {result.ProjectName}";
    }

    private Window? GetMainWindow()
    {
        return (Application.Current?.ApplicationLifetime as IClassicDesktopStyleApplicationLifetime)?.MainWindow;
    }
}
