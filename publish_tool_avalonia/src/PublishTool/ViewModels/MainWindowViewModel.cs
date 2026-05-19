using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.DependencyInjection;
using PublishTool.Models.Local;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly ConfigService _configService;
    private readonly ProjectService _projectService;

    public event Func<Task>? AddServerProjectRequested;
    public event Func<Task>? AddClientProjectRequested;

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
        ProjectService projectService)
    {
        _configService = configService;
        _projectService = projectService;

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
            App.Services.GetRequiredService<FileService>(),
            App.Services.GetRequiredService<LocalFileService>(),
            App.Services.GetRequiredService<ProcessService>());
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
    private void RemoveProject(ProjectConfig config)
    {
        _configService.RemoveProject(config);
    }

    [RelayCommand]
    private void MoveUpProject(ProjectConfig project)
    {
        var index = FilteredProjects.IndexOf(project);
        if (index > 0)
            _configService.MoveUp(index);
    }

    [RelayCommand]
    private void MoveDownProject(ProjectConfig project)
    {
        var index = FilteredProjects.IndexOf(project);
        if (index >= 0 && index < FilteredProjects.Count - 1)
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
