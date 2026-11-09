using System.Collections.ObjectModel;
using System.Diagnostics;
using System.IO;
using AlyPublish.Constants;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Serilog;

namespace AlyPublish.ViewModels;

public sealed partial class MainWindowViewModel : ViewModelBase
{
    private readonly CliService _cli;
    private readonly ConfigService _config;

    public MainWindowViewModel(
        CliService cli,
        ConfigService config,
        IDialogService dialogs,
        INotificationService notifications)
        : base(dialogs, notifications)
    {
        _cli = cli;
        _config = config;

        IsCliAvailable = _cli.IsAvailable;
        CliPath = _cli.ExecutablePath;

        Projects.CollectionChanged += (_, _) => OnPropertyChanged(nameof(HasProjects));
        Tabs.CollectionChanged += (_, _) => OnPropertyChanged(nameof(HasTabs));

        LoadProjects();
        Log.Information("主窗口初始化: 项目 {Count} 个, CLI={Cli}", Projects.Count, CliPath);
    }

    public ObservableCollection<ProjectConfig> Projects { get; } = [];

    public ObservableCollection<ProjectTabViewModel> Tabs { get; } = [];

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(RemoveProjectCommand))]
    [NotifyCanExecuteChangedFor(nameof(EditProjectCommand))]
    private ProjectConfig? _selectedProject;

    [ObservableProperty]
    private ProjectTabViewModel? _selectedTab;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(CliHint))]
    private bool _isCliAvailable;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(CliHint))]
    private string _cliPath = string.Empty;

    public bool HasProjects => Projects.Count > 0;

    public bool HasTabs => Tabs.Count > 0;

    public string CliHint => IsCliAvailable
        ? $"aly-publish：{CliPath}"
        : "未找到 aly-publish.exe。请将其放在本程序目录，或点击“重新检测”。";

    partial void OnSelectedProjectChanged(ProjectConfig? value)
    {
        if (value is null)
        {
            SelectedTab = null;
            return;
        }

        var tab = Tabs.FirstOrDefault(candidate => string.Equals(
            candidate.Project.ProjectPath,
            value.ProjectPath,
            StringComparison.OrdinalIgnoreCase));

        if (tab is null)
        {
            tab = CreateTab(value);
            Tabs.Add(tab);
            tab.RefreshCommand.Execute(null);
        }

        SelectedTab = tab;
    }

    partial void OnSelectedTabChanged(ProjectTabViewModel? value)
    {
        if (value is not null && !ReferenceEquals(SelectedProject, value.Project))
        {
            SelectedProject = value.Project;
        }
    }

    protected override void OnBusyChanged()
    {
        AddProjectCommand.NotifyCanExecuteChanged();
        AddLocalProjectCommand.NotifyCanExecuteChanged();
        RemoveProjectCommand.NotifyCanExecuteChanged();
        EditProjectCommand.NotifyCanExecuteChanged();
    }

    [RelayCommand(CanExecute = nameof(CanAddProject))]
    private async Task AddProjectAsync()
    {
        var config = await Dialogs.ShowAddProjectAsync();
        if (config is not null)
        {
            await AddProjectCoreAsync(config);
        }
    }

    [RelayCommand(CanExecute = nameof(CanAddProject))]
    private async Task AddLocalProjectAsync()
    {
        var config = await Dialogs.ShowAddLocalProjectAsync();
        if (config is not null)
        {
            await AddProjectCoreAsync(config);
        }
    }

    [RelayCommand(CanExecute = nameof(CanRemoveProject))]
    private async Task RemoveProjectAsync()
    {
        var project = SelectedProject;
        if (project is null)
        {
            return;
        }

        var confirmed = await Dialogs.ConfirmAsync(
            "移除项目",
            $"确定从列表中移除 “{project.DisplayName}” 吗？\n\n项目文件和 .updator 配置不会被删除。");
        if (!confirmed)
        {
            return;
        }

        var removed = await RunAsync("移除项目", () =>
        {
            var remaining = Projects.Where(candidate => !ReferenceEquals(candidate, project)).ToList();
            _config.SaveProjects(remaining);

            var wasSelected = ReferenceEquals(SelectedProject, project);
            if (wasSelected)
            {
                SelectedProject = null;
            }

            var tab = FindTab(project);
            if (tab is not null)
            {
                RemoveTab(tab, selectNeighbor: false);
            }

            Projects.Remove(project);
            if (wasSelected)
            {
                SelectedProject = Tabs.FirstOrDefault()?.Project;
            }
        });

        if (removed)
        {
            Notify.Success($"项目 “{project.DisplayName}” 已移除");
        }
    }

    [RelayCommand(CanExecute = nameof(CanEditProject))]
    private async Task EditProjectAsync()
    {
        var project = SelectedProject;
        if (project is not null)
        {
            await EditProjectCoreAsync(project);
        }
    }

    [RelayCommand]
    private void CloseTab(ProjectTabViewModel? tab)
    {
        if (tab is not null)
        {
            RemoveTab(tab, selectNeighbor: true);
        }
    }

    [RelayCommand]
    private void RetryCli()
    {
        _cli.RefreshAvailability();
        IsCliAvailable = _cli.IsAvailable;
        CliPath = _cli.ExecutablePath;

        if (IsCliAvailable)
        {
            Notify.Success("已找到 aly-publish");
        }
        else
        {
            Notify.Warning("仍未找到 aly-publish.exe，请检查程序目录");
        }
    }

    [RelayCommand]
    private void OpenCliFolder() => OpenDirectory(AppDomain.CurrentDomain.BaseDirectory);

    [RelayCommand]
    private void OpenLogFolder()
    {
        var logDirectory = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, AppConstants.LogDirectoryName);
        if (!Directory.Exists(logDirectory))
        {
            logDirectory = Path.Combine(Path.GetTempPath(), AppConstants.AppName, AppConstants.LogDirectoryName);
        }

        OpenDirectory(logDirectory);
    }

    [RelayCommand]
    private void RefreshCurrent() => SelectedTab?.RefreshCommand.Execute(null);

    [RelayCommand]
    private void PublishCurrent() => SelectedTab?.PublishCommand.Execute(null);

    private bool CanAddProject() => !IsBusy;

    private bool CanRemoveProject() => !IsBusy && SelectedProject is not null;

    private bool CanEditProject() => !IsBusy && SelectedProject is not null;

    private async Task AddProjectCoreAsync(ProjectConfig project)
    {
        var added = await RunAsync("添加项目", () =>
        {
            _config.SaveProjects(Projects.Append(project));
            Projects.Add(project);
            SelectedProject = project;
        });

        if (added)
        {
            Notify.Success($"项目 “{project.DisplayName}” 已添加");
        }
    }

    private async Task EditProjectCoreAsync(ProjectConfig project)
    {
        var saved = false;
        var completed = await RunAsync("保存项目设置", async _ =>
        {
            var updated = await Dialogs.ShowEditProjectAsync(project);
            if (updated is null)
            {
                return;
            }

            project.DisplayName = updated.DisplayName;
            project.MainExePath = updated.MainExePath;
            _config.SaveProjects(Projects);
            saved = true;
        });

        if (completed && saved)
        {
            Notify.Success("项目设置已保存");
        }
    }

    private async void OnTabEditRequested(object? sender, ProjectConfig project)
    {
        await EditProjectCoreAsync(project);
    }

    private ProjectTabViewModel CreateTab(ProjectConfig project)
    {
        var tab = new ProjectTabViewModel(project, _cli, Dialogs, Notify);
        tab.EditRequested += OnTabEditRequested;
        return tab;
    }

    private ProjectTabViewModel? FindTab(ProjectConfig project)
        => Tabs.FirstOrDefault(tab => ReferenceEquals(tab.Project, project));

    private void RemoveTab(ProjectTabViewModel tab, bool selectNeighbor)
    {
        var index = Tabs.IndexOf(tab);
        if (index < 0)
        {
            return;
        }

        var wasSelected = ReferenceEquals(SelectedTab, tab);
        Tabs.RemoveAt(index);

        if (!wasSelected || !selectNeighbor)
        {
            return;
        }

        SelectedTab = Tabs.Count == 0 ? null : Tabs[Math.Min(index, Tabs.Count - 1)];
        SelectedProject = SelectedTab?.Project;
    }

    private void LoadProjects()
    {
        foreach (var project in _config.LoadProjects())
        {
            Projects.Add(project);
        }

        if (Projects.Count > 0)
        {
            SelectedProject = Projects[0];
        }
    }

    private void OpenDirectory(string path)
    {
        if (!Directory.Exists(path))
        {
            Notify.Warning($"目录不存在：{path}");
            return;
        }

        try
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = path,
                UseShellExecute = true
            });
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开目录失败: {Path}", path);
            Notify.ShowError($"打开目录失败：{ex.Message}");
        }
    }
}
