using System.Collections.ObjectModel;
using System.Diagnostics;
using System.Diagnostics.CodeAnalysis;
using System.IO;
using AlyPublish.Constants;
using AlyPublish.Helpers;
using AlyPublish.Localization;
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
    private readonly Dictionary<string, ProjectTabViewModel> _projectViews =
        new(StringComparer.OrdinalIgnoreCase);

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

        Projects.CollectionChanged += (_, _) =>
        {
            OnPropertyChanged(nameof(HasProjects));
            OnPropertyChanged(nameof(HasNoFilterMatches));
            ApplyProjectFilter();
        };

        LanguageManager.LanguageChanged += OnLanguageChanged;
        LoadProjects();
        Log.Information("主窗口初始化: 项目 {Count} 个, CLI={Cli}", Projects.Count, CliPath);
    }

    public ObservableCollection<ProjectConfig> Projects { get; } = [];

    public ObservableCollection<ProjectConfig> FilteredProjects { get; } = [];

    [ObservableProperty]
    private string _projectFilter = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(RemoveProjectCommand))]
    [NotifyCanExecuteChangedFor(nameof(EditProjectCommand))]
    private ProjectConfig? _selectedProject;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasSelectedTab))]
    private ProjectTabViewModel? _selectedTab;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(CliHint))]
    private bool _isCliAvailable;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(CliHint))]
    private string _cliPath = string.Empty;

    public bool HasProjects => Projects.Count > 0;

    public bool HasFilteredProjects => FilteredProjects.Count > 0;

    public bool HasNoFilterMatches => HasProjects && !HasFilteredProjects;

    public bool HasSelectedTab => SelectedTab is not null;

    public string CliHint => IsCliAvailable
        ? LanguageManager.F(LanguageManager.Instance.MainWindow_Cli_Hint, CliPath)
        : LanguageManager.T(LanguageManager.Instance.MainWindow_Cli_MissingHint);

    [SuppressMessage(
        "Performance",
        "CA1822:Mark members as static",
        Justification = "Bound from XAML; bound members must remain instance members.")]
    public string LanguageLabel => LanguageManager.LanguageLabel;

    partial void OnProjectFilterChanged(string value) => ApplyProjectFilter();

    partial void OnSelectedProjectChanged(ProjectConfig? value)
    {
        if (value is null)
        {
            SelectedTab = null;
            return;
        }

        var tab = GetOrCreateProjectView(value, out var isNew);
        if (isNew)
        {
            tab.RefreshCommand.Execute(null);
        }

        SelectedTab = tab;
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
            LanguageManager.T(LanguageManager.Instance.MainWindow_RemoveProjectDialog_Title),
            LanguageManager.F(LanguageManager.Instance.MainWindow_RemoveProjectDialog_Message, project.DisplayName));
        if (!confirmed)
        {
            return;
        }

        var removed = await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_RemoveProject), () =>
        {
            var remaining = Projects.Where(candidate => !ReferenceEquals(candidate, project)).ToList();
            _config.SaveProjects(remaining);

            var wasSelected = ReferenceEquals(SelectedProject, project);
            if (wasSelected)
            {
                SelectedProject = null;
            }

            Projects.Remove(project);
            _projectViews.Remove(project.ProjectPath);
            if (wasSelected)
            {
                SelectedProject = FilteredProjects.FirstOrDefault();
            }
        });

        if (removed)
        {
            Notify.Success(LanguageManager.F(LanguageManager.Instance.MainWindow_Message_Removed, project.DisplayName));
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
    private void ClearProjectFilter() => ProjectFilter = string.Empty;

    [SuppressMessage(
        "Performance",
        "CA1822:Mark members as static",
        Justification = "Generated command is bound from XAML.")]
    [RelayCommand]
    private void ToggleLanguage() => LanguageManager.Toggle();

    private void OnLanguageChanged(object? sender, EventArgs e)
    {
        OnPropertyChanged(nameof(LanguageLabel));
        OnPropertyChanged(nameof(CliHint));
        SelectedTab?.RefreshLocalization();
    }

    [RelayCommand]
    private void RetryCli()
    {
        _cli.RefreshAvailability();
        IsCliAvailable = _cli.IsAvailable;
        CliPath = _cli.ExecutablePath;

        if (IsCliAvailable)
        {
            Notify.Success(LanguageManager.T(LanguageManager.Instance.MainWindow_Message_FoundCli));
        }
        else
        {
            Notify.Warning(LanguageManager.T(LanguageManager.Instance.MainWindow_Message_StillNoCli));
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
        ProjectFilter = string.Empty;
        var added = await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_AddProject), () =>
        {
            _config.SaveProjects(Projects.Append(project));
            Projects.Add(project);
            SelectedProject = project;
        });

        if (added)
        {
            Notify.Success(LanguageManager.F(LanguageManager.Instance.MainWindow_Message_Added, project.DisplayName));
            RefreshUpdaters(project);
        }
    }

    private async Task EditProjectCoreAsync(ProjectConfig project)
    {
        var saved = false;
        var completed = await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_SaveProjectSettings), async _ =>
        {
            var updated = await Dialogs.ShowEditProjectAsync(project);
            if (updated is null)
            {
                return;
            }

            project.DisplayName = updated.DisplayName;
            project.MainExePath = updated.MainExePath;
            _config.SaveProjects(Projects);
            ApplyProjectFilter();
            saved = true;
        });

        if (completed && saved)
        {
            Notify.Success(LanguageManager.T(LanguageManager.Instance.MainWindow_Message_SettingsSaved));
            EnsureUpdaters(project);
        }
    }

    /// <summary>
    /// When the project settings supply a main executable and the sibling <c>UpdateFolder</c> does not exist
    /// yet, build the updater package right now — the same code path the initialization wizard uses. A
    /// failure is reported as a warning: the bookmarked settings are already saved either way.
    /// </summary>
    private void EnsureUpdaters(ProjectConfig project)
    {
        if (string.IsNullOrWhiteSpace(project.MainExePath))
        {
            return;
        }

        var projectDirectory = Path.TrimEndingDirectorySeparator(Path.GetFullPath(project.ProjectPath));
        var parentDirectory = Path.GetDirectoryName(projectDirectory);
        if (string.IsNullOrWhiteSpace(parentDirectory)
            || Directory.Exists(Path.Combine(parentDirectory, "UpdateFolder")))
        {
            return;
        }

        var mainExeFullPath = ProjectInputHelper.ResolveMainExePath(project.ProjectPath, project.MainExePath);
        if (!File.Exists(mainExeFullPath))
        {
            Log.Warning("主程序不存在，跳过 UpdateFolder 创建: {Path}", mainExeFullPath);
            return;
        }

        try
        {
            var warning = UpdateFolderService.Create(
                project.ProjectPath,
                mainExeFullPath,
                Path.GetFileNameWithoutExtension(mainExeFullPath),
                AppDomain.CurrentDomain.BaseDirectory);
            if (warning is not null)
            {
                Notify.Warning(warning);
            }
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or InvalidOperationException)
        {
            Log.Warning(ex, "创建 UpdateFolder 失败: {Path}", project.ProjectPath);
            Notify.Warning(ex.Message);
        }
    }

    /// <summary>
    /// An added project gets its updater executables refressed from the packaged payload when a sibling
    /// <c>UpdateFolder</c> already exists. A missing payload is reported, never fatal, and
    /// <c>client.json</c> is left alone.
    /// </summary>
    private void RefreshUpdaters(ProjectConfig project)
    {
        try
        {
            var warning = UpdateFolderService.RefreshUpdaters(
                project.ProjectPath,
                AppDomain.CurrentDomain.BaseDirectory);
            if (warning is not null)
            {
                Notify.Warning(warning);
            }
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            Log.Warning(ex, "刷新更新器失败: {Path}", project.ProjectPath);
        }
    }

    private async void OnProjectEditRequested(object? sender, ProjectConfig project)
    {
        await EditProjectCoreAsync(project);
    }

    private ProjectTabViewModel GetOrCreateProjectView(ProjectConfig project, out bool isNew)
    {
        if (_projectViews.TryGetValue(project.ProjectPath, out var existing))
        {
            isNew = false;
            return existing;
        }

        var view = new ProjectTabViewModel(project, _cli, Dialogs, Notify);
        view.EditRequested += OnProjectEditRequested;
        _projectViews[project.ProjectPath] = view;
        isNew = true;
        return view;
    }

    private void LoadProjects()
    {
        foreach (var project in _config.LoadProjects())
        {
            Projects.Add(project);
        }

        if (Projects.Count > 0)
        {
            SelectedProject = FilteredProjects.FirstOrDefault();
        }
    }

    private void ApplyProjectFilter()
    {
        var previousSelection = SelectedProject;
        var filter = ProjectFilter.Trim();
        FilteredProjects.Clear();

        foreach (var project in Projects.Where(project => MatchesFilter(project, filter)))
        {
            FilteredProjects.Add(project);
        }

        OnPropertyChanged(nameof(HasFilteredProjects));
        OnPropertyChanged(nameof(HasNoFilterMatches));

        if (previousSelection is not null && FilteredProjects.Contains(previousSelection))
        {
            SelectedProject = previousSelection;
        }
        else if (previousSelection is not null)
        {
            SelectedProject = FilteredProjects.FirstOrDefault();
        }
        else if (filter.Length == 0 && FilteredProjects.Count > 0)
        {
            SelectedProject = FilteredProjects[0];
        }
    }

    private static bool MatchesFilter(ProjectConfig project, string filter)
        => filter.Length == 0
           || project.DisplayName.Contains(filter, StringComparison.OrdinalIgnoreCase)
           || project.ProjectPath.Contains(filter, StringComparison.OrdinalIgnoreCase);

    private void OpenDirectory(string path)
    {
        if (!Directory.Exists(path))
        {
            Notify.Warning(LanguageManager.F(LanguageManager.Instance.MainWindow_Message_DirectoryMissing, path));
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
            Notify.ShowError(LanguageManager.F(LanguageManager.Instance.MainWindow_Message_OpenFolderFailed, ex.Message));
        }
    }
}
