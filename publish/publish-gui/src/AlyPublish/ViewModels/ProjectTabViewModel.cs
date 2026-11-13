using System.Collections.ObjectModel;
using System.Collections.Specialized;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using AlyPublish.Constants;
using AlyPublish.Helpers;
using AlyPublish.Localization;
using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace AlyPublish.ViewModels;

/// <summary>The right-hand project detail: changes, publish panel and version history.</summary>
public sealed partial class ProjectTabViewModel : ViewModelBase
{
    private readonly CliService _cli;
    private readonly Dictionary<int, int> _uploadSlots = [];
    private bool _hasLoaded;

    public ProjectTabViewModel(
        ProjectConfig project,
        CliService cli,
        IDialogService dialogs,
        INotificationService notifications)
        : base(dialogs, notifications)
    {
        Project = project;
        _cli = cli;

        UnstagedFiles.CollectionChanged += OnUnstagedFilesChanged;
        StagedFiles.CollectionChanged += OnStagedFilesChanged;
        ChangeLogs.CollectionChanged += (_, _) => OnPropertyChanged(nameof(HasChangeLogs));
    }

    public ProjectConfig Project { get; }

    public ObservableCollection<FileItem> UnstagedFiles { get; } = [];

    public ObservableCollection<FileItem> StagedFiles { get; } = [];

    public ObservableCollection<ChangeLog> ChangeLogs { get; } = [];

    public ObservableCollection<UploadProgress> UploadProgressItems { get; } = [];

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    [NotifyCanExecuteChangedFor(nameof(SuggestVersionCommand))]
    [NotifyPropertyChangedFor(nameof(PublishHint))]
    [NotifyPropertyChangedFor(nameof(HasPublishHint))]
    [NotifyPropertyChangedFor(nameof(PublishButtonText))]
    private string _newVersion = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    [NotifyPropertyChangedFor(nameof(PublishHint))]
    [NotifyPropertyChangedFor(nameof(HasPublishHint))]
    [NotifyPropertyChangedFor(nameof(PublishButtonText))]
    private string _commitMessage = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(SuggestVersionCommand))]
    [NotifyPropertyChangedFor(nameof(CurrentVersionText))]
    private string _currentVersion = string.Empty;

    [ObservableProperty]
    private string _afterApplyUpdateScript = string.Empty;

    [ObservableProperty]
    private bool _setForceUpdate;

    [ObservableProperty]
    private bool _isInitialLoading;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    private bool _isUploading;

    [ObservableProperty]
    private int _uploadDoneCount;

    [ObservableProperty]
    private int _uploadFailedCount;

    [ObservableProperty]
    private int _uploadTotalCount;

    [ObservableProperty]
    private double _uploadProgressPercent;

    [ObservableProperty]
    private string _uploadProgressText = string.Empty;

    [ObservableProperty]
    private string _uploadSummary = string.Empty;

    [ObservableProperty]
    private bool _isUploadProgressIndeterminate = true;

    public event EventHandler<ProjectConfig>? EditRequested;

    public string PublishHint
    {
        get
        {
            if (StagedFiles.Count == 0)
            {
                return LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_StageFirst);
            }

            if (string.IsNullOrWhiteSpace(NewVersion))
            {
                return LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_NeedVersion);
            }

            return string.IsNullOrWhiteSpace(CommitMessage) ? LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_NeedNotes) : string.Empty;
        }
    }

    public string PublishButtonText => StagedFiles.Count == 0
        ? LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_PublishStaged)
        : LanguageManager.F(LanguageManager.Instance.ProjectTab_Publish_PublishCount, StagedFiles.Count);

    public string CurrentVersionText => string.IsNullOrWhiteSpace(CurrentVersion)
        ? LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_NoVersion)
        : CurrentVersion;

    public bool HasPublishHint => !string.IsNullOrEmpty(PublishHint);

    public bool HasUnstagedFiles => UnstagedFiles.Count > 0;

    public bool HasStagedFiles => StagedFiles.Count > 0;

    public bool HasChangeLogs => ChangeLogs.Count > 0;

    public string UnstagedSelectionText
        => UnstagedFiles.Count > 0 && UnstagedFiles.All(file => file.IsSelected)
            ? LanguageManager.T(LanguageManager.Instance.ProjectTab_Changes_UnselectAll)
            : LanguageManager.T(LanguageManager.Instance.ProjectTab_Changes_SelectAll);

    public string StagedSelectionText
        => StagedFiles.Count > 0 && StagedFiles.All(file => file.IsSelected)
            ? LanguageManager.T(LanguageManager.Instance.ProjectTab_Changes_UnselectAll)
            : LanguageManager.T(LanguageManager.Instance.ProjectTab_Changes_SelectAll);

    /// <summary>Called after a culture switch to refresh computed strings.</summary>
    public void RefreshLocalization()
    {
        OnPropertyChanged(nameof(PublishHint));
        OnPropertyChanged(nameof(HasPublishHint));
        OnPropertyChanged(nameof(PublishButtonText));
        OnPropertyChanged(nameof(CurrentVersionText));
        OnPropertyChanged(nameof(UnstagedSelectionText));
        OnPropertyChanged(nameof(StagedSelectionText));
        foreach (var file in UnstagedFiles.Concat(StagedFiles))
        {
            file.RefreshLocalization();
        }

        StatusMessage = LanguageManager.F(
            LanguageManager.T(LanguageManager.Instance.ProjectTab_Changes_StatusSummary),
            UnstagedFiles.Count,
            StagedFiles.Count);
    }

    protected override void OnBusyChanged()
    {
        RefreshCommand.NotifyCanExecuteChanged();
        AddAllCommand.NotifyCanExecuteChanged();
        ResetAllCommand.NotifyCanExecuteChanged();
        AddSelectedCommand.NotifyCanExecuteChanged();
        ResetSelectedCommand.NotifyCanExecuteChanged();
        PublishCommand.NotifyCanExecuteChanged();
        EditProjectCommand.NotifyCanExecuteChanged();
        ToggleSelectAllUnstagedCommand.NotifyCanExecuteChanged();
        ToggleSelectAllStagedCommand.NotifyCanExecuteChanged();
        SuggestVersionCommand.NotifyCanExecuteChanged();
        OnPropertyChanged(nameof(PublishHint));
        OnPropertyChanged(nameof(HasPublishHint));
    }

    [RelayCommand(CanExecute = nameof(CanRefresh))]
    private async Task RefreshAsync()
    {
        StatusMessage = LanguageManager.T(LanguageManager.Instance.ProjectTab_Changes_ReadingStatus);
        IsInitialLoading = !_hasLoaded;
        try
        {
            await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_Refresh), token => FetchDataAsync(token), showErrorDialog: false);
        }
        finally
        {
            _hasLoaded = true;
            IsInitialLoading = false;
        }
    }

    [RelayCommand(CanExecute = nameof(CanAddAll))]
    private async Task AddAllAsync()
    {
        var added = await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_StageAll), async token =>
        {
            await _cli.AddAllAsync(Project.ProjectPath, token);
            await FetchDataAsync(token);
        });

        if (added)
        {
            Notify.Success(LanguageManager.T(LanguageManager.Instance.ProjectTab_Message_StagedNotify));
        }
    }

    [RelayCommand(CanExecute = nameof(CanResetAll))]
    private async Task ResetAllAsync()
    {
        var confirmed = await Dialogs.ConfirmAsync(
            LanguageManager.T(LanguageManager.Instance.ProjectTab_ClearStagingDialog_Title),
            LanguageManager.T(LanguageManager.Instance.ProjectTab_ClearStagingDialog_Message));
        if (!confirmed)
        {
            return;
        }

        var reset = await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_ClearStaging), async token =>
        {
            await _cli.ResetAllAsync(Project.ProjectPath, token);
            await FetchDataAsync(token);
        });

        if (reset)
        {
            Notify.Success(LanguageManager.T(LanguageManager.Instance.ProjectTab_Message_StagingCleared));
        }
    }

    [RelayCommand(CanExecute = nameof(CanAddSelected))]
    private async Task AddSelectedAsync()
    {
        var files = SelectedPaths(UnstagedFiles);
        var added = await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_StageSelected), async token =>
        {
            await _cli.AddFilesAsync(Project.ProjectPath, files, token);
            await FetchDataAsync(token);
        });

        if (added)
        {
            Notify.Success(LanguageManager.F(LanguageManager.Instance.ProjectTab_Message_StagedSelected, files.Count));
        }
    }

    [RelayCommand(CanExecute = nameof(CanResetSelected))]
    private async Task ResetSelectedAsync()
    {
        var files = SelectedPaths(StagedFiles);
        var reset = await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_UnstageSelected), async token =>
        {
            await _cli.ResetFilesAsync(Project.ProjectPath, files, token);
            await FetchDataAsync(token);
        });

        if (reset)
        {
            Notify.Success(LanguageManager.F(LanguageManager.Instance.ProjectTab_Message_UnstagedSelected, files.Count));
        }
    }

    [RelayCommand(CanExecute = nameof(CanPublish), IncludeCancelCommand = true)]
    private async Task PublishAsync(CancellationToken cancellationToken)
    {
        var version = NewVersion.Trim();
        var message = CommitMessage.Trim();
        var stagedCount = StagedFiles.Count;

        PrepareUpload(stagedCount);
        try
        {
            var succeeded = await RunAsync(
                LanguageManager.T(LanguageManager.Instance.Operation_Publish),
                async token =>
                {
                    await _cli.PushAsync(
                        Project.ProjectPath,
                        version,
                        message,
                        AfterApplyUpdateScript.Trim(),
                        SetForceUpdate,
                        new Progress<UploadProgress>(OnUploadProgress),
                        token);
                },
                showErrorDialog: true,
                cancellationToken: cancellationToken);

            if (!succeeded)
            {
                UploadProgressText = cancellationToken.IsCancellationRequested
                    ? LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_Cancelled)
                    : LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_Failed);
                return;
            }

            NewVersion = string.Empty;
            CommitMessage = string.Empty;
            AfterApplyUpdateScript = string.Empty;
            // 审查 B5：不再把 SetForceUpdate 复位成 false——
            // 旧实现每次发布后硬复位，用户下次发布默认又变成"不强制"，
            // 与服务端现值脱节。改为发布后按服务端实际状态回填。
            await RefreshForceUpdateFromServerAsync(cancellationToken);
            await RunAsync(
                LanguageManager.T(LanguageManager.Instance.Operation_Refresh),
                token => FetchDataAsync(token),
                showErrorDialog: false,
                cancellationToken: CancellationToken.None);
            StatusMessage = LanguageManager.F(LanguageManager.Instance.ProjectTab_Publish_SucceededStatus, stagedCount);
            Notify.Success(LanguageManager.F(LanguageManager.Instance.ProjectTab_Publish_SucceededNotify, version));
        }
        finally
        {
            IsUploading = false;
        }
    }

    [RelayCommand(CanExecute = nameof(CanEdit))]
    private void EditProject() => EditRequested?.Invoke(this, Project);

    [RelayCommand]
    private async Task LaunchProgramAsync()
    {
        var executablePath = GetMainExeFullPath();
        if (executablePath is null)
        {
            Notify.Warning(LanguageManager.T(LanguageManager.Instance.ProjectTab_Message_LaunchNoMain));
            return;
        }

        if (!File.Exists(executablePath))
        {
            await Dialogs.ErrorAsync(
                LanguageManager.T(LanguageManager.Instance.ProjectTab_Message_LaunchFailedTitle),
                LanguageManager.F(LanguageManager.Instance.ProjectTab_Message_MainMissing, executablePath));
            return;
        }

        await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_Launch), () =>
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = executablePath,
                WorkingDirectory = Path.GetDirectoryName(executablePath) ?? Project.ProjectPath,
                UseShellExecute = true
            });
            StatusMessage = LanguageManager.F(LanguageManager.Instance.ProjectTab_Message_Launched, executablePath);
        });
    }

    [RelayCommand]
    private async Task OpenExplorerAsync()
    {
        if (!Directory.Exists(Project.ProjectPath))
        {
            await Dialogs.ErrorAsync(
                LanguageManager.T(LanguageManager.Instance.ProjectTab_Message_OpenFolderFailedTitle),
                LanguageManager.F(LanguageManager.Instance.ProjectTab_Message_DirectoryMissing, Project.ProjectPath));
            return;
        }

        await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_OpenProjectFolder), () =>
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = Project.ProjectPath,
                UseShellExecute = true
            });
            StatusMessage = LanguageManager.F(LanguageManager.Instance.ProjectTab_Message_Opened, Project.ProjectPath);
        });
    }

    [RelayCommand]
    private async Task OpenFileLocationAsync(FileItem? file)
    {
        if (file is null)
        {
            return;
        }

        var fullPath = Path.GetFullPath(Path.Combine(Project.ProjectPath, file.RelativePath));
        var directory = Path.GetDirectoryName(fullPath);
        if (directory is null || !Directory.Exists(directory))
        {
            await Dialogs.ErrorAsync(
                LanguageManager.T(LanguageManager.Instance.ProjectTab_Message_OpenFileLocationFailedTitle),
                LanguageManager.F(LanguageManager.Instance.ProjectTab_Message_DirectoryMissing, directory));
            return;
        }

        await RunAsync(LanguageManager.T(LanguageManager.Instance.Operation_OpenFileLocation), () =>
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = directory,
                UseShellExecute = true
            });
        });
    }

    [RelayCommand(CanExecute = nameof(CanSuggestVersion))]
    private void SuggestVersion()
    {
        var nextVersion = VersionHelper.SuggestNext(CurrentVersion);
        if (nextVersion is not null)
        {
            NewVersion = nextVersion;
        }
    }

    [RelayCommand(CanExecute = nameof(CanToggleUnstagedSelection))]
    private void ToggleSelectAllUnstaged()
    {
        var selectAll = !UnstagedFiles.All(file => file.IsSelected);
        foreach (var file in UnstagedFiles)
        {
            file.IsSelected = selectAll;
        }
    }

    [RelayCommand(CanExecute = nameof(CanToggleStagedSelection))]
    private void ToggleSelectAllStaged()
    {
        var selectAll = !StagedFiles.All(file => file.IsSelected);
        foreach (var file in StagedFiles)
        {
            file.IsSelected = selectAll;
        }
    }

    private bool CanRefresh() => !IsBusy && !string.IsNullOrWhiteSpace(Project.ProjectPath);

    private bool CanAddAll() => !IsBusy && UnstagedFiles.Count > 0;

    private bool CanResetAll() => !IsBusy && StagedFiles.Count > 0;

    private bool CanAddSelected() => !IsBusy && UnstagedFiles.Any(file => file.IsSelected);

    private bool CanResetSelected() => !IsBusy && StagedFiles.Any(file => file.IsSelected);

    private bool CanPublish()
        => !IsBusy
           && !IsUploading
           && StagedFiles.Count > 0
           && !string.IsNullOrWhiteSpace(NewVersion)
           && !string.IsNullOrWhiteSpace(CommitMessage);

    private bool CanEdit() => !IsBusy;

    private bool CanSuggestVersion() => !IsBusy && VersionHelper.SuggestNext(CurrentVersion) is not null;

    private bool CanToggleUnstagedSelection() => !IsBusy && UnstagedFiles.Count > 0;

    private bool CanToggleStagedSelection() => !IsBusy && StagedFiles.Count > 0;

    private async Task FetchDataAsync(CancellationToken cancellationToken)
    {
        var status = await _cli.GetStatusAsync(Project.ProjectPath, cancellationToken);
        var changeLogs = await _cli.GetLogAsync(Project.ProjectPath, cancellationToken: cancellationToken);
        ApplyStatus(status, changeLogs);
        await RefreshForceUpdateFromServerAsync(cancellationToken);
        StatusMessage = LanguageManager.F(
            LanguageManager.T(LanguageManager.Instance.ProjectTab_Changes_StatusSummary),
            UnstagedFiles.Count,
            StagedFiles.Count);
    }

    /// <summary>
    /// 审查 B5：强制更新开关的初值改为继承服务端现值。
    /// 旧实现固定 false 且每次发布后硬复位，用户每次发布都得重新勾选，
    /// 而且默认会把服务端的"强制更新"静默关掉（CLI 侧已改为三态 flag，
    /// 这里保证 GUI 的复选框反映真实状态）。查询失败时保持当前值，不误改。
    /// </summary>
    private async Task RefreshForceUpdateFromServerAsync(CancellationToken cancellationToken)
    {
        try
        {
            // 服务端地址/项目名在 .updator/shared.json 里（ProjectConfig 只是书签）
            var shared = ProjectConfigService.Load(Project.ProjectPath);
            if (string.IsNullOrWhiteSpace(shared.ServerUrl) || string.IsNullOrWhiteSpace(shared.ProjectName))
            {
                return;
            }
            var projects = await _cli.GetServerProjectsAsync(shared.ServerUrl, cancellationToken);
            var match = projects.FirstOrDefault(p => p.Name == shared.ProjectName);
            if (match is not null)
            {
                SetForceUpdate = match.IsForceUpdate;
            }
        }
        catch (CliException)
        {
            // 服务端不可达时保留既有勾选状态：这是展示型同步，不该阻断操作
        }
    }

    private void ApplyStatus(StatusData status, List<ChangeLog> changeLogs)
    {
        // 审查 B6：删除同步链路打通后，deleted 必须显示出来供用户操作
        // （旧实现直接过滤掉，用户根本看不到"本地已删除、服务端仍存在"的文件）
        ReplaceFiles(UnstagedFiles, status.Unstaged.Select(FileItem.FromCliItem));

        ReplaceFiles(StagedFiles, status.Staged.Select(FileItem.FromCliItem));

        ChangeLogs.Clear();
        for (var index = 0; index < changeLogs.Count; index++)
        {
            var entry = changeLogs[index];
            entry.IsLatest = index == 0;
            ChangeLogs.Add(entry);
        }

        CurrentVersion = ChangeLogs.Count > 0 ? ChangeLogs[0].Version : string.Empty;
    }

    private void ReplaceFiles(ObservableCollection<FileItem> target, IEnumerable<FileItem> items)
    {
        foreach (var file in target)
        {
            file.PropertyChanged -= OnFilePropertyChanged;
        }

        target.Clear();
        foreach (var file in items)
        {
            target.Add(file);
        }
    }

    private void PrepareUpload(int total)
    {
        UploadProgressItems.Clear();
        _uploadSlots.Clear();
        UploadDoneCount = 0;
        UploadFailedCount = 0;
        UploadTotalCount = total;
        UploadProgressPercent = 0;
        IsUploadProgressIndeterminate = total == 0;
        UploadProgressText = total == 0
            ? LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_Upload_Preparing)
            : LanguageManager.F(LanguageManager.Instance.ProjectTab_Publish_Upload_PreparingCount, total);
        UploadSummary = string.Empty;
        IsUploading = true;
    }

    private void OnUploadProgress(UploadProgress progress)
    {
        if (progress.Total > 0)
        {
            UploadTotalCount = progress.Total;
        }

        IsUploadProgressIndeterminate = UploadTotalCount <= 0;

        switch (progress.Status)
        {
            case "START":
                UpsertProgress(progress);
                UploadProgressText = LanguageManager.F(
                    LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_Upload_Uploading),
                    progress.File,
                    UploadDoneCount + UploadFailedCount,
                    UploadTotalCount);
                break;

            case "DONE":
            case "FAIL":
                UpsertProgress(progress);
                if (progress.IsDone)
                {
                    UploadDoneCount++;
                }
                else
                {
                    UploadFailedCount++;
                }

                UpdateUploadPercent();
                UploadSummary = LanguageManager.F(
                    LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_Upload_Summary),
                    UploadDoneCount,
                    UploadFailedCount,
                    UploadTotalCount);
                UploadProgressText = progress.IsDone
                    ? LanguageManager.F(
                        LanguageManager.T(LanguageManager.Instance.ProjectTab_Publish_Upload_Done),
                        progress.File,
                        UploadDoneCount + UploadFailedCount,
                        UploadTotalCount)
                    : LanguageManager.F(LanguageManager.Instance.ProjectTab_Publish_Upload_Failed, progress.File, progress.Error);
                break;
        }
    }

    private void UpsertProgress(UploadProgress progress)
    {
        if (_uploadSlots.TryGetValue(progress.Index, out var slot) && slot < UploadProgressItems.Count)
        {
            UploadProgressItems[slot] = progress;
            return;
        }

        _uploadSlots[progress.Index] = UploadProgressItems.Count;
        UploadProgressItems.Add(progress);
    }

    private void UpdateUploadPercent()
    {
        UploadProgressPercent = UploadTotalCount > 0
            ? (double)(UploadDoneCount + UploadFailedCount) / UploadTotalCount * 100
            : 0;
    }

    private void OnUnstagedFilesChanged(object? sender, NotifyCollectionChangedEventArgs e)
    {
        AttachOrDetach(e, UnstagedFiles);
        AddAllCommand.NotifyCanExecuteChanged();
        AddSelectedCommand.NotifyCanExecuteChanged();
        ToggleSelectAllUnstagedCommand.NotifyCanExecuteChanged();
        OnPropertyChanged(nameof(UnstagedSelectionText));
        OnPropertyChanged(nameof(HasUnstagedFiles));
        OnPropertyChanged(nameof(PublishButtonText));
    }

    private void OnStagedFilesChanged(object? sender, NotifyCollectionChangedEventArgs e)
    {
        AttachOrDetach(e, StagedFiles);
        ResetAllCommand.NotifyCanExecuteChanged();
        ResetSelectedCommand.NotifyCanExecuteChanged();
        PublishCommand.NotifyCanExecuteChanged();
        ToggleSelectAllStagedCommand.NotifyCanExecuteChanged();
        OnPropertyChanged(nameof(StagedSelectionText));
        OnPropertyChanged(nameof(HasStagedFiles));
        OnPropertyChanged(nameof(PublishHint));
        OnPropertyChanged(nameof(HasPublishHint));
        OnPropertyChanged(nameof(PublishButtonText));
    }

    private void AttachOrDetach(NotifyCollectionChangedEventArgs args, ObservableCollection<FileItem> files)
    {
        if (args.OldItems is not null)
        {
            foreach (FileItem file in args.OldItems)
            {
                file.PropertyChanged -= OnFilePropertyChanged;
            }
        }

        if (args.NewItems is not null)
        {
            foreach (FileItem file in args.NewItems)
            {
                file.PropertyChanged += OnFilePropertyChanged;
            }
        }
    }

    private void OnFilePropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName != nameof(FileItem.IsSelected))
        {
            return;
        }

        AddSelectedCommand.NotifyCanExecuteChanged();
        ResetSelectedCommand.NotifyCanExecuteChanged();
        OnPropertyChanged(nameof(UnstagedSelectionText));
        OnPropertyChanged(nameof(StagedSelectionText));
    }

    private string? GetMainExeFullPath()
    {
        if (string.IsNullOrWhiteSpace(Project.MainExePath))
        {
            return null;
        }

        return ProjectInputHelper.ResolveMainExePath(Project.ProjectPath, Project.MainExePath);
    }

    private static List<string> SelectedPaths(IEnumerable<FileItem> files)
        => files.Where(file => file.IsSelected).Select(file => file.RelativePath).ToList();
}
