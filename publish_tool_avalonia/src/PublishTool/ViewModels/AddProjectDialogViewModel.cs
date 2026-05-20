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

    public event Action? CloseRequested;

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

    [ObservableProperty]
    private bool _isSaved;

    [ObservableProperty]
    private string _serverUrlError = string.Empty;

    [ObservableProperty]
    private string _localPathError = string.Empty;

    partial void OnServerUrlChanged(string value) => ServerUrlError = string.Empty;
    partial void OnLocalPathChanged(string value) => LocalPathError = string.Empty;

    public AddProjectDialogViewModel(ProjectService projectService, ConfigService configService)
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
        AvailableProjects = new List<ProjectDto>();
        var response = await _projectService.GetAllProjectsAsync(ServerUrl);
        AvailableProjects = response.Data ?? new List<ProjectDto>();
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
}
