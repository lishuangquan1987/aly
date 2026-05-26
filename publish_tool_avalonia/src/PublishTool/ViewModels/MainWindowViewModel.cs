using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models.Local;
using PublishTool.Services;
using Serilog;

namespace PublishTool.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly ConfigService _configService;
    private readonly ProjectService _projectService;
    private readonly FileService _fileService;
    private readonly LocalFileService _localFileService;
    private readonly ProcessService _processService;

    public event Func<Task>? AddServerProjectRequested;
    public event Func<Task>? AddClientProjectRequested;
    public event Func<Task>? DeleteProjectRequested;

    [ObservableProperty]
    private string _searchText = string.Empty;

    [ObservableProperty]
    private ObservableCollection<ProjectConfig> _filteredProjects = new();

    [ObservableProperty]
    private ObservableCollection<ProjectPageViewModel> _openTabs = new();

    [ObservableProperty]
    private ProjectPageViewModel? _selectedTab;

    [ObservableProperty]
    private ProjectConfig? _selectedProject;

    [ObservableProperty]
    private string _currentTime = DateTime.Now.ToString("yyyy/MM/dd HH:mm:ss");

    public MainWindowViewModel(
        ConfigService configService,
        ProjectService projectService,
        FileService fileService,
        LocalFileService localFileService,
        ProcessService processService)
    {
        _configService = configService;
        _projectService = projectService;
        _fileService = fileService;
        _localFileService = localFileService;
        _processService = processService;

        foreach (var project in _configService.Projects)
        {
            FilteredProjects.Add(project);
        }

        _configService.ProjectsChanged += OnProjectsChanged;
    }

    private void OnProjectsChanged()
    {
        try
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
        catch (Exception ex)
        {
            Log.Error(ex, "刷新项目列表失败");
        }
    }

    partial void OnSearchTextChanged(string value)
    {
        OnProjectsChanged();
    }

    partial void OnSelectedProjectChanged(ProjectConfig? value)
    {
        if (value != null)
            OpenProject(value);
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
            _fileService, _localFileService, _processService);
        OpenTabs.Add(tab);
        SelectedTab = tab;
    }

    [RelayCommand]
    private void CloseTab(ProjectPageViewModel tab)
    {
        tab.Dispose();
        OpenTabs.Remove(tab);
    }

    [RelayCommand]
    private async Task RemoveProject(ProjectConfig config)
    {
        if (DeleteProjectRequested != null)
        {
            SelectedProject = config;
            await DeleteProjectRequested.Invoke();
        }
    }

    public void ConfirmRemoveProject(ProjectConfig config)
    {
        _configService.RemoveProject(config);
    }

    [RelayCommand]
    private void MoveUpProject(ProjectConfig project)
    {
        var projects = _configService.Projects;
        var index = -1;
        for (var i = 0; i < projects.Count; i++)
        {
            if (ReferenceEquals(projects[i], project))
            {
                index = i;
                break;
            }
        }
        if (index > 0)
            _configService.MoveUp(index);
    }

    [RelayCommand]
    private void MoveDownProject(ProjectConfig project)
    {
        var projects = _configService.Projects;
        var index = -1;
        for (var i = 0; i < projects.Count; i++)
        {
            if (ReferenceEquals(projects[i], project))
            {
                index = i;
                break;
            }
        }
        if (index >= 0 && index < projects.Count - 1)
            _configService.MoveDown(index);
    }

    [RelayCommand]
    private void CloseAllTabs()
    {
        foreach (var tab in OpenTabs.ToList())
        {
            tab.Dispose();
        }
        OpenTabs.Clear();
    }

    [RelayCommand]
    private async Task AddServerProject()
    {
        if (AddServerProjectRequested != null)
            await AddServerProjectRequested.Invoke();
    }

    [RelayCommand]
    private async Task AddClientProject()
    {
        if (AddClientProjectRequested != null)
            await AddClientProjectRequested.Invoke();
    }
}
