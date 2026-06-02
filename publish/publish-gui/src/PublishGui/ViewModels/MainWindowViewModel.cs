using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishGui.Models.Cli;
using PublishGui.Models.Local;
using PublishGui.Services;
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
        // TODO: Show dialog to add existing project
        StatusMessage = "添加已有项目功能待实现";
    }

    [RelayCommand]
    private async Task AddNewProjectAsync()
    {
        // TODO: Show dialog to add new project
        StatusMessage = "新建项目功能待实现";
    }

    [RelayCommand]
    private async Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return;

        _configService.RemoveProject(SelectedProject.Config.ProjectName);
        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        StatusMessage = "已移除项目";
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
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "Failed to get server info");
        }
    }

    [RelayCommand]
    private async Task OpenProjectSettingsAsync()
    {
        // TODO: Show project settings dialog
        StatusMessage = "项目设置功能待实现";
    }
}
