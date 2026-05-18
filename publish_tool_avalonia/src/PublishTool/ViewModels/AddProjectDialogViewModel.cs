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

    [ObservableProperty]
    private bool _isSaved;

    public AddProjectDialogViewModel(ProjectService projectService, ConfigService configService)
    {
        _projectService = projectService;
        _configService = configService;
    }

    [RelayCommand]
    private async Task FetchProjects()
    {
        if (string.IsNullOrEmpty(ServerUrl)) return;
        AvailableProjects = await _projectService.GetAllProjectsAsync(ServerUrl);
    }

    [RelayCommand]
    private void Confirm()
    {
        if (SelectedProject == null) return;
        if (string.IsNullOrEmpty(LocalPath)) return;

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
    }
}