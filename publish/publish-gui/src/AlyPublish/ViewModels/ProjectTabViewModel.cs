using System.Collections.ObjectModel;
using System.Collections.Specialized;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using AlyPublish.Constants;
using AlyPublish.Helpers;
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
                return "请先在左侧勾选文件并暂存";
            }

            if (string.IsNullOrWhiteSpace(NewVersion))
            {
                return "请填写版本号";
            }

            return string.IsNullOrWhiteSpace(CommitMessage) ? "请填写变更说明" : string.Empty;
        }
    }

    public string PublishButtonText => StagedFiles.Count == 0 ? "发布暂存文件" : $"发布 {StagedFiles.Count} 个文件";

    public string CurrentVersionText => string.IsNullOrWhiteSpace(CurrentVersion) ? "尚无版本" : CurrentVersion;

    public bool HasPublishHint => !string.IsNullOrEmpty(PublishHint);

    public bool HasUnstagedFiles => UnstagedFiles.Count > 0;

    public bool HasStagedFiles => StagedFiles.Count > 0;

    public bool HasChangeLogs => ChangeLogs.Count > 0;

    public string UnstagedSelectionText
        => UnstagedFiles.Count > 0 && UnstagedFiles.All(file => file.IsSelected) ? "取消全选" : "全选";

    public string StagedSelectionText
        => StagedFiles.Count > 0 && StagedFiles.All(file => file.IsSelected) ? "取消全选" : "全选";

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
        StatusMessage = "正在读取项目状态…";
        IsInitialLoading = !_hasLoaded;
        try
        {
            await RunAsync("刷新", token => FetchDataAsync(token), showErrorDialog: false);
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
        var added = await RunAsync("暂存全部文件", async token =>
        {
            await _cli.AddAllAsync(Project.ProjectPath, token);
            await FetchDataAsync(token);
        });

        if (added)
        {
            Notify.Success("已暂存全部变更");
        }
    }

    [RelayCommand(CanExecute = nameof(CanResetAll))]
    private async Task ResetAllAsync()
    {
        var confirmed = await Dialogs.ConfirmAsync(
            "清空暂存区",
            "确定取消暂存所有文件吗？\n\n文件不会被删除，但需要重新勾选并暂存。");
        if (!confirmed)
        {
            return;
        }

        var reset = await RunAsync("清空暂存区", async token =>
        {
            await _cli.ResetAllAsync(Project.ProjectPath, token);
            await FetchDataAsync(token);
        });

        if (reset)
        {
            Notify.Success("暂存区已清空");
        }
    }

    [RelayCommand(CanExecute = nameof(CanAddSelected))]
    private async Task AddSelectedAsync()
    {
        var files = SelectedPaths(UnstagedFiles);
        var added = await RunAsync("暂存所选文件", async token =>
        {
            await _cli.AddFilesAsync(Project.ProjectPath, files, token);
            await FetchDataAsync(token);
        });

        if (added)
        {
            Notify.Success($"已暂存 {files.Count} 个文件");
        }
    }

    [RelayCommand(CanExecute = nameof(CanResetSelected))]
    private async Task ResetSelectedAsync()
    {
        var files = SelectedPaths(StagedFiles);
        var reset = await RunAsync("取消暂存所选文件", async token =>
        {
            await _cli.ResetFilesAsync(Project.ProjectPath, files, token);
            await FetchDataAsync(token);
        });

        if (reset)
        {
            Notify.Success($"已取消暂存 {files.Count} 个文件");
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
                "发布",
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
                UploadProgressText = cancellationToken.IsCancellationRequested ? "发布已取消" : "发布失败";
                return;
            }

            NewVersion = string.Empty;
            CommitMessage = string.Empty;
            AfterApplyUpdateScript = string.Empty;
            SetForceUpdate = false;
            await RunAsync(
                "刷新",
                token => FetchDataAsync(token),
                showErrorDialog: false,
                cancellationToken: CancellationToken.None);
            StatusMessage = $"发布成功（{stagedCount} 个暂存文件）";
            Notify.Success($"发布成功：{version}");
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
            Notify.Warning("请先在项目设置中选择主程序");
            return;
        }

        if (!File.Exists(executablePath))
        {
            await Dialogs.ErrorAsync("启动程序失败", $"主程序不存在：{executablePath}");
            return;
        }

        await RunAsync("启动程序", () =>
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = executablePath,
                WorkingDirectory = Path.GetDirectoryName(executablePath) ?? Project.ProjectPath,
                UseShellExecute = true
            });
            StatusMessage = $"已启动：{executablePath}";
        });
    }

    [RelayCommand]
    private async Task OpenExplorerAsync()
    {
        if (!Directory.Exists(Project.ProjectPath))
        {
            await Dialogs.ErrorAsync("打开项目目录失败", $"目录不存在：{Project.ProjectPath}");
            return;
        }

        await RunAsync("打开项目目录", () =>
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = Project.ProjectPath,
                UseShellExecute = true
            });
            StatusMessage = $"已打开：{Project.ProjectPath}";
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
            await Dialogs.ErrorAsync("打开文件位置失败", $"目录不存在：{directory}");
            return;
        }

        await RunAsync("打开文件位置", () =>
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
        StatusMessage = $"未暂存 {UnstagedFiles.Count}，已暂存 {StagedFiles.Count}";
    }

    private void ApplyStatus(StatusData status, List<ChangeLog> changeLogs)
    {
        ReplaceFiles(
            UnstagedFiles,
            status.Unstaged
                .Where(item => item.Status != FileStatus.Deleted)
                .Select(FileItem.FromCliItem));

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
        UploadProgressText = total == 0 ? "准备上传…" : $"准备上传 {total} 个文件…";
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
                UploadProgressText = $"正在上传 {progress.File}（{UploadDoneCount + UploadFailedCount}/{UploadTotalCount}）";
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
                UploadSummary = $"完成 {UploadDoneCount}，失败 {UploadFailedCount}，共 {UploadTotalCount}";
                UploadProgressText = progress.IsDone
                    ? $"已完成 {progress.File}（{UploadDoneCount + UploadFailedCount}/{UploadTotalCount}）"
                    : $"上传失败 {progress.File}：{progress.Error}";
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
