using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Platform.Storage;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Flurl.Http;
using PublishTool.Models;
using PublishTool.Models.Local;
using PublishTool.Services;

namespace PublishTool.ViewModels.Pages;

public partial class AddProjectDialogViewModel : ObservableObject
{
    private readonly ProjectService _projectService;
    private readonly ConfigService _configService;

    public AddProjectDialogViewModel()
    {
        SelectFolderCmd = new AsyncRelayCommand(SelectFolder);
        SelectExeCmd = new AsyncRelayCommand(SelectExe);
    }

    private async Task SelectExe()
    {
        var topLevel = TopLevel.GetTopLevel(App.MainWindow);
        if (topLevel == null) return;

        var files = await topLevel.StorageProvider.OpenFilePickerAsync(
            new FilePickerOpenOptions
            {
                Title = "选择 EXE 文件",
                AllowMultiple = false,
                FileTypeFilter = new[]
                {
                    new FilePickerFileType("exe文件")
                    {
                        Patterns = ["*.exe"]
                    }
                }
            });

        var path = files.FirstOrDefault()?.Path.LocalPath;
        if (path != null)
            this.ExePath = path;
    }

    private async Task SelectFolder()
    {
        var topLevel = TopLevel.GetTopLevel(App.MainWindow);
        if (topLevel == null) return;

        var folders = await topLevel.StorageProvider.OpenFolderPickerAsync(
            new FolderPickerOpenOptions { Title = "选择本地文件夹路径", AllowMultiple = false });

        if (folders.Count <= 0)
        {
            return;
        }

        var path = folders.FirstOrDefault()?.Path.LocalPath;
        this.LocalPath = path;

        //exe自动识别
        var files = Directory.GetFiles(path, "*.exe");
        if (files?.Length > 0)
        {
            this.ExePath = files[0];
        }
    }

    public event Action? CloseRequested;

    [ObservableProperty] private string _serverUrl = string.Empty;

    [ObservableProperty] private List<ProjectDto> _availableProjects = new();

    [ObservableProperty] private ProjectDto? _selectedProject;

    [ObservableProperty] private string _localPath = string.Empty;

    [ObservableProperty] private string? _exePath;

    [ObservableProperty] private bool _isSaved;

    [ObservableProperty] private string _serverUrlError = string.Empty;

    [ObservableProperty] private string _localPathError = string.Empty;

    [ObservableProperty] private bool _isFetching;

    partial void OnServerUrlChanged(string value) => ServerUrlError = string.Empty;
    partial void OnLocalPathChanged(string value) => LocalPathError = string.Empty;

    public AddProjectDialogViewModel(ProjectService projectService, ConfigService configService)
        : this()
    {
        _projectService = projectService;
        _configService = configService;
    }

    [RelayCommand]
    private async Task FetchProjects()
    {
        if (string.IsNullOrWhiteSpace(ServerUrl))
        {
            ServerUrlError = "请输入服务器地址";
            return;
        }

        if (!ServerUrl.StartsWith("http://") && !ServerUrl.StartsWith("https://"))
        {
            ServerUrlError = "地址需以 http:// 或 https:// 开头";
            return;
        }

        IsFetching = true;
        AvailableProjects = new List<ProjectDto>();
        try
        {
            var response = await _projectService.GetAllProjectsAsync(ServerUrl);
            if (response.IsSuccess)
            {
                AvailableProjects = response.Data ?? new List<ProjectDto>();
            }
            else
            {
                ServerUrlError = response.ErrorMsg ?? "获取项目列表失败";
            }
        }
        catch (FlurlHttpException ex)
        {
            ServerUrlError = $"网络错误: {ex.Message}";
        }
        catch (Exception ex)
        {
            ServerUrlError = $"获取项目列表失败: {ex.Message}";
        }
        finally
        {
            IsFetching = false;
        }
    }

    [RelayCommand]
    private void Confirm()
    {
        if (SelectedProject == null)
        {
            LocalPathError = "请先选择项目";
            return;
        }

        if (string.IsNullOrWhiteSpace(LocalPath))
        {
            LocalPathError = "请选择本地文件夹路径";
            return;
        }

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
        IsSaved = true;
        CloseRequested?.Invoke();
    }

    public AsyncRelayCommand SelectFolderCmd { get; }
    public AsyncRelayCommand SelectExeCmd { get; }
}
