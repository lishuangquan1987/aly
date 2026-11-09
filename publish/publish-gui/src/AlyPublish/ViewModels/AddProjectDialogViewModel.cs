using System.Collections.ObjectModel;
using System.IO;
using AlyPublish.Constants;
using AlyPublish.Helpers;
using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Serilog;

namespace AlyPublish.ViewModels;

public enum AddProjectStep
{
    Server,
    Local
}

/// <summary>
/// Two-step initialization wizard: pick a server project, then point at the local build folder.
/// Everything that can be discovered or suggested is filled in automatically.
/// </summary>
public sealed partial class AddProjectDialogViewModel : ViewModelBase
{
    private readonly CliService _cli;
    private readonly ConfigService _config;
    private readonly ProjectSetupService _setup;

    private List<ProjectInfo> _allServerProjects = [];
    private HashSet<string> _existingProjectPaths = new(StringComparer.OrdinalIgnoreCase);

    public AddProjectDialogViewModel(
        CliService cli,
        ConfigService config,
        ProjectSetupService setup,
        IDialogService dialogs,
        INotificationService notifications)
        : base(dialogs, notifications)
    {
        _cli = cli;
        _config = config;
        _setup = setup;
    }

    public ObservableCollection<string> KnownServerUrls { get; } = [];

    public ObservableCollection<ProjectInfo> ServerProjects { get; } = [];

    public ObservableCollection<string> MainExeCandidates { get; } = [];

    public ObservableCollection<string> IgnoreFolders { get; } = [];

    public ObservableCollection<string> IgnoreFiles { get; } = [];

    public ObservableCollection<string> UnCopyFolders { get; } = [];

    public ObservableCollection<string> UnCopyFiles { get; } = [];

    public IReadOnlyList<string> IgnoreFolderSuggestions { get; } = AppConstants.IgnoreFolderSuggestions;

    public IReadOnlyList<string> IgnoreFileSuggestions { get; } = AppConstants.IgnoreFileSuggestions;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsServerStep))]
    [NotifyPropertyChangedFor(nameof(IsLocalStep))]
    [NotifyCanExecuteChangedFor(nameof(NextStepCommand))]
    [NotifyCanExecuteChangedFor(nameof(PreviousStepCommand))]
    [NotifyCanExecuteChangedFor(nameof(ConfirmCommand))]
    private AddProjectStep _step = AddProjectStep.Server;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsServerSelectionValid))]
    [NotifyCanExecuteChangedFor(nameof(FetchProjectsCommand))]
    [NotifyCanExecuteChangedFor(nameof(CreateServerProjectCommand))]
    [NotifyCanExecuteChangedFor(nameof(NextStepCommand))]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(FetchProjectsCommand))]
    [NotifyCanExecuteChangedFor(nameof(CreateServerProjectCommand))]
    [NotifyPropertyChangedFor(nameof(ShowServerEmptyState))]
    private bool _isFetching;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasFetchError))]
    [NotifyPropertyChangedFor(nameof(ShowServerEmptyState))]
    private string _fetchError = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(NextStepCommand))]
    [NotifyPropertyChangedFor(nameof(SelectedProjectVersionText))]
    private ProjectInfo? _selectedServerProject;

    [ObservableProperty]
    private string _filterText = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ConfirmCommand))]
    private string _displayName = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ConfirmCommand))]
    private string _projectPath = string.Empty;

    [ObservableProperty]
    private bool _isPathUsable;

    [ObservableProperty]
    private string _pathState = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ConfirmCommand))]
    [NotifyPropertyChangedFor(nameof(HasValidationMessage))]
    private string _validationMessage = string.Empty;

    [ObservableProperty]
    private string _mainExePath = string.Empty;

    [ObservableProperty]
    private string _closeProcessName = string.Empty;

    [ObservableProperty]
    private string? _selectedMainExeCandidate;

    public bool IsServerStep => Step == AddProjectStep.Server;

    public bool IsLocalStep => Step == AddProjectStep.Local;

    public bool HasServerProjects => ServerProjects.Count > 0;

    public bool HasFetchError => !string.IsNullOrEmpty(FetchError);

    public bool ShowServerEmptyState => !HasServerProjects && !HasFetchError && !IsFetching;

    public bool HasValidationMessage => !string.IsNullOrEmpty(ValidationMessage);

    public bool IsServerSelectionValid => SelectedServerProject is not null && !string.IsNullOrWhiteSpace(ServerUrl);

    /// <summary>Called by the view when a folder is dropped onto the dialog.</summary>
    public void SetDroppedFolder(string folderPath)
    {
        ProjectPath = folderPath;
        if (IsServerSelectionValid)
        {
            Step = AddProjectStep.Local;
        }
    }

    public string SelectedProjectVersionText
        => string.IsNullOrWhiteSpace(SelectedServerProject?.Version) ? "未发布" : SelectedServerProject.Version;

    public void Initialize()
    {
        var projects = _config.LoadProjects();
        _existingProjectPaths = new HashSet<string>(
            projects.Select(project => NormalizeForComparison(project.ProjectPath)),
            StringComparer.OrdinalIgnoreCase);

        foreach (var serverUrl in ProjectConfigService.GetKnownServerUrls(projects.Select(project => project.ProjectPath)))
        {
            KnownServerUrls.Add(serverUrl);
        }

        if (KnownServerUrls.Count > 0)
        {
            ServerUrl = KnownServerUrls[0];
        }
    }

    partial void OnStepChanged(AddProjectStep value)
    {
        if (value == AddProjectStep.Local)
        {
            RefreshPathState();
            UpdateMainExeCandidates();
        }

        UpdateValidation();
    }

    partial void OnServerUrlChanged(string value)
    {
        ClearServerSelection();
        UpdateValidation();
    }

    partial void OnSelectedServerProjectChanged(ProjectInfo? value)
    {
        if (value is not null)
        {
            DisplayName = value.Name;
        }

        UpdateValidation();
    }

    partial void OnFilterTextChanged(string value) => ApplyServerFilter();

    partial void OnDisplayNameChanged(string value) => UpdateValidation();

    partial void OnProjectPathChanged(string value)
    {
        RefreshPathState();
        UpdateMainExeCandidates();

        if (string.IsNullOrWhiteSpace(DisplayName) && !string.IsNullOrWhiteSpace(value))
        {
            DisplayName = Path.GetFileName(Path.TrimEndingDirectorySeparator(value.Trim()));
        }

        UpdateValidation();
    }

    partial void OnSelectedMainExeCandidateChanged(string? value)
    {
        if (!string.IsNullOrWhiteSpace(value))
        {
            MainExePath = value;
        }
    }

    partial void OnMainExePathChanged(string value)
    {
        if (!string.IsNullOrWhiteSpace(value))
        {
            CloseProcessName = Path.GetFileNameWithoutExtension(value.Trim());
        }
    }

    protected override void OnBusyChanged()
    {
        FetchProjectsCommand.NotifyCanExecuteChanged();
        CreateServerProjectCommand.NotifyCanExecuteChanged();
        NextStepCommand.NotifyCanExecuteChanged();
        PreviousStepCommand.NotifyCanExecuteChanged();
        ConfirmCommand.NotifyCanExecuteChanged();
    }

    [RelayCommand(CanExecute = nameof(CanFetchProjects))]
    private async Task FetchProjectsAsync()
    {
        if (IsFetching)
        {
            return;
        }

        IsFetching = true;
        FetchError = string.Empty;
        try
        {
            var projects = await _cli.GetServerProjectsAsync(ServerUrl.Trim());
            _allServerProjects = projects;
            ApplyServerFilter();
            FetchError = projects.Count == 0 ? "服务端暂无项目，可以点击“新建服务端项目”。" : string.Empty;
            Log.Information("获取服务端项目: {Count} 个", projects.Count);
        }
        catch (CliException ex)
        {
            _allServerProjects = [];
            ApplyServerFilter();
            FetchError = $"获取项目列表失败：{ex.Message}";
            Log.Warning(ex, "获取服务端项目失败");
        }
        catch (Exception ex) when (ex is not OperationCanceledException)
        {
            _allServerProjects = [];
            ApplyServerFilter();
            FetchError = $"获取项目列表失败：{ex.Message}";
            Log.Error(ex, "获取服务端项目发生未预期异常");
        }
        finally
        {
            IsFetching = false;
            UpdateValidation();
        }
    }

    [RelayCommand(CanExecute = nameof(CanCreateServerProject))]
    private async Task CreateServerProjectAsync()
    {
        var request = new CreateServerProjectRequest(
            ServerUrl.Trim(),
            _allServerProjects.Select(project => project.Name).ToList(),
            Clean(IgnoreFolders),
            Clean(IgnoreFiles));

        var created = await Dialogs.ShowCreateProjectAsync(request);
        if (created is null)
        {
            return;
        }

        await FetchProjectsAsync();
        SelectedServerProject = ServerProjects.FirstOrDefault(project =>
            string.Equals(project.Name, created.Name, StringComparison.OrdinalIgnoreCase));
        DisplayName = created.Name;
    }

    [RelayCommand(CanExecute = nameof(CanGoNext))]
    private void NextStep() => Step = AddProjectStep.Local;

    [RelayCommand(CanExecute = nameof(CanGoBack))]
    private void PreviousStep() => Step = AddProjectStep.Server;

    [RelayCommand(CanExecute = nameof(CanFinish))]
    private async Task ConfirmAsync()
    {
        var selectedProject = SelectedServerProject;
        if (selectedProject is null)
        {
            return;
        }

        var projectPath = ProjectPath.Trim();
        var request = new ProjectInitializationRequest(
            ServerUrl.Trim(),
            selectedProject.Name,
            projectPath,
            string.IsNullOrWhiteSpace(DisplayName)
                ? Path.GetFileName(Path.TrimEndingDirectorySeparator(projectPath))
                : DisplayName.Trim(),
            MainExePath.Trim(),
            CloseProcessName.Trim(),
            Clean(IgnoreFolders),
            Clean(IgnoreFiles),
            Clean(UnCopyFolders),
            Clean(UnCopyFiles));

        ProjectSetupResult? result = null;
        var completed = await RunAsync(
            "初始化项目",
            async token => result = await _setup.InitializeAsync(request, token));

        if (!completed || result is null)
        {
            return;
        }

        foreach (var warning in result.Warnings)
        {
            Notify.Warning(warning);
        }

        Close(result.Config);
    }

    [RelayCommand]
    private void AddSuggestedIgnoreFolder(string? value) => AddIfMissing(IgnoreFolders, value);

    [RelayCommand]
    private void AddSuggestedIgnoreFile(string? value) => AddIfMissing(IgnoreFiles, value);

    [RelayCommand]
    private void UseServerUrl(string? value)
    {
        if (!string.IsNullOrWhiteSpace(value))
        {
            ServerUrl = value.Trim();
        }
    }

    public void AddIgnoreFolder(string fullPath) => AddProjectEntry(IgnoreFolders, fullPath, "文件夹");

    public void AddIgnoreFile(string fullPath) => AddProjectEntry(IgnoreFiles, fullPath, "文件");

    public void AddUnCopyFolder(string fullPath) => AddProjectEntry(UnCopyFolders, fullPath, "文件夹");

    public void AddUnCopyFile(string fullPath) => AddProjectEntry(UnCopyFiles, fullPath, "文件");

    [RelayCommand]
    private void Cancel() => Close(null);

    private bool CanFetchProjects()
        => !IsBusy && !IsFetching && !string.IsNullOrWhiteSpace(ServerUrl);

    private bool CanCreateServerProject()
        => !IsBusy && !IsFetching && !string.IsNullOrWhiteSpace(ServerUrl);

    private bool CanGoNext() => !IsBusy && IsServerStep && IsServerSelectionValid;

    private bool CanGoBack() => !IsBusy && IsLocalStep;

    private bool CanFinish()
        => !IsBusy
           && IsLocalStep
           && IsPathUsable
           && IsServerSelectionValid
           && !string.IsNullOrWhiteSpace(DisplayName)
           && string.IsNullOrEmpty(ValidationMessage);

    private void ClearServerSelection()
    {
        _allServerProjects = [];
        ServerProjects.Clear();
        SelectedServerProject = null;
        FetchError = string.Empty;
        OnPropertyChanged(nameof(HasServerProjects));
        OnPropertyChanged(nameof(ShowServerEmptyState));
    }

    private void ApplyServerFilter()
    {
        var selectedName = SelectedServerProject?.Name;
        ServerProjects.Clear();

        foreach (var project in _allServerProjects.Where(MatchesFilter))
        {
            ServerProjects.Add(project);
        }

        OnPropertyChanged(nameof(HasServerProjects));
        OnPropertyChanged(nameof(ShowServerEmptyState));

        if (!string.IsNullOrWhiteSpace(selectedName))
        {
            SelectedServerProject = ServerProjects.FirstOrDefault(project =>
                string.Equals(project.Name, selectedName, StringComparison.OrdinalIgnoreCase));
        }
        else if (ServerProjects.Count == 1)
        {
            SelectedServerProject = ServerProjects[0];
        }
    }

    private bool MatchesFilter(ProjectInfo project)
    {
        if (string.IsNullOrWhiteSpace(FilterText))
        {
            return true;
        }

        var filter = FilterText.Trim();
        return project.Name.Contains(filter, StringComparison.OrdinalIgnoreCase)
               || project.Title.Contains(filter, StringComparison.OrdinalIgnoreCase);
    }

    private void RefreshPathState()
    {
        var path = ProjectPath.Trim();
        IsPathUsable = false;

        if (path.Length == 0)
        {
            PathState = "请选择项目目录";
            return;
        }

        if (!Directory.Exists(path))
        {
            PathState = "目录不存在";
            return;
        }

        if (ProjectInputHelper.IsInitialized(path))
        {
            PathState = "该目录已初始化，请使用“添加本地项目”";
            return;
        }

        if (_existingProjectPaths.Contains(NormalizeForComparison(path)))
        {
            PathState = "该项目已在左侧列表中";
            return;
        }

        IsPathUsable = true;
        PathState = "将初始化 .updator/shared.json";
    }

    private void UpdateMainExeCandidates()
    {
        var previous = SelectedMainExeCandidate;
        var candidates = ProjectInputHelper.FindMainExeCandidates(ProjectPath.Trim());

        MainExeCandidates.Clear();
        foreach (var candidate in candidates)
        {
            MainExeCandidates.Add(candidate);
        }

        if (previous is not null && MainExeCandidates.Contains(previous))
        {
            SelectedMainExeCandidate = previous;
        }
        else if (MainExeCandidates.Count == 1)
        {
            SelectedMainExeCandidate = MainExeCandidates[0];
        }
        else
        {
            SelectedMainExeCandidate = null;
        }
    }

    private void UpdateValidation()
    {
        ValidationMessage = BuildValidationMessage();
        NextStepCommand.NotifyCanExecuteChanged();
        PreviousStepCommand.NotifyCanExecuteChanged();
        ConfirmCommand.NotifyCanExecuteChanged();
    }

    private string BuildValidationMessage()
    {
        if (IsServerStep)
        {
            return IsServerSelectionValid ? string.Empty : "请先获取并选择一个服务端项目";
        }

        if (!IsPathUsable)
        {
            return string.IsNullOrWhiteSpace(PathState) ? "请选择本地项目目录" : PathState;
        }

        return string.IsNullOrWhiteSpace(DisplayName) ? "请填写显示名称" : string.Empty;
    }

    private static void AddIfMissing(ObservableCollection<string> target, string? value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return;
        }

        var trimmed = value.Trim();
        if (!target.Contains(trimmed))
        {
            target.Add(trimmed);
        }
    }

    private void AddProjectEntry(ObservableCollection<string> target, string fullPath, string kind)
    {
        if (!ProjectInputHelper.TryGetProjectRelativePath(ProjectPath.Trim(), fullPath, out var relative))
        {
            Notify.Warning($"请选择项目目录内的{kind}");
            return;
        }

        AddIfMissing(target, relative);
    }

    private static List<string> Clean(IEnumerable<string> values)
        => values
            .Select(value => value.Trim())
            .Where(value => value.Length > 0)
            .Distinct(StringComparer.Ordinal)
            .ToList();

    private static string NormalizeForComparison(string path)
    {
        try
        {
            return Path.TrimEndingDirectorySeparator(Path.GetFullPath(path));
        }
        catch (Exception ex) when (ex is ArgumentException or NotSupportedException or PathTooLongException)
        {
            return path.Trim();
        }
    }
}
