using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.DependencyInjection;
using PublishTool.Models.Local;
using PublishTool.Services;
using PublishTool.ViewModels.Pages;
using PublishTool.Views.Pages;
using Serilog;
using Ursa.Controls;

namespace PublishTool.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly ConfigService _configService;
    private readonly ProjectService _projectService;
    private readonly FileService _fileService;
    private readonly LocalFileService _localFileService;
    private readonly ProcessService _processService;

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
        var vm = App.Services.GetRequiredService<DeleteConfirmDialogViewModel>();
        vm.ProjectTitle = config.Title;
        var result = await Dialog.ShowStandardAsync<DeleteConfirmDialogView, DeleteConfirmDialogViewModel>(
            vm, null, new DialogOptions()
            {
                Title = "确认删除",
                Button = DialogButton.YesNo
            });
        if (result == DialogResult.Yes)
        {
            ConfirmRemoveProject(config);
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
        var vm = App.Services.GetRequiredService<AddServerProjectDialogViewModel>();
        var result = await Dialog.ShowStandardAsync<AddServerProjectDialogView, AddServerProjectDialogViewModel>(
            vm, null, new DialogOptions()
            {
                Title = "新建服务端项目",
                Button = DialogButton.OKCancel
            });
        if (result == DialogResult.OK)
        {
            await vm.TryCreateAsync();
        }
    }

    [RelayCommand]
    private async Task AddClientProject()
    {
        var vm = App.Services.GetRequiredService<AddProjectDialogViewModel>();
        var result = await Dialog.ShowStandardAsync<AddProjectDialogView, AddProjectDialogViewModel>(
            vm, null, new DialogOptions()
            {
                Title = "添加新项目",
                Button = DialogButton.OKCancel
            });
        if (result == DialogResult.OK)
        {
            vm.ConfirmCommand.Execute(null);
        }
    }
}
