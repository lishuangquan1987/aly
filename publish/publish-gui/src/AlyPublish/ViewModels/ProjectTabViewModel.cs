using System;
using System.Collections.ObjectModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Avalonia.Threading;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Ursa.Controls;
using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using Serilog;

namespace AlyPublish.ViewModels;

public partial class ProjectTabViewModel : ObservableObject, IDisposable
{
    private readonly CliService _cli;
    private CancellationTokenSource? _refreshCts;

    public void Dispose()
    {
        _refreshCts?.Cancel();
        _refreshCts?.Dispose();
        _refreshCts = null;
    }

    public ProjectConfig Project { get; }

    [ObservableProperty] private ObservableCollection<FileItem> _unstagedFiles = new();
    [ObservableProperty] private ObservableCollection<FileItem> _stagedFiles = new();
    [ObservableProperty] private ObservableCollection<ChangeLog> _changeLogs = new();
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    [ObservableProperty] private string _newVersion = string.Empty;
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    [ObservableProperty] private string _commitMessage = string.Empty;
    [ObservableProperty] private string _currentVersion = string.Empty;
    [ObservableProperty] private string _statusMessage = "就绪";
    [ObservableProperty] private bool _isBusy;
    [ObservableProperty] private bool _isUploading;
    [ObservableProperty] private string _afterApplyUpdateScript = string.Empty;
    [ObservableProperty] private bool _setForceUpdate;

    [ObservableProperty] private ObservableCollection<UploadProgress> _uploadProgressItems = new();
    [ObservableProperty] private int _uploadDoneCount;
    [ObservableProperty] private int _uploadTotalCount;
    [ObservableProperty] private double _uploadProgressPercent;
    [ObservableProperty] private string _uploadProgressText = string.Empty;
    [ObservableProperty] private bool _isUploadProgressIndeterminate = true;

    [ObservableProperty] private FileItem? _selectedUnStagedFile;
    [ObservableProperty] private FileItem? _selectedStagedFile;

    public IAsyncRelayCommand RefreshCommand { get; }
    public IAsyncRelayCommand AddAllCommand { get; }
    public IAsyncRelayCommand ResetAllCommand { get; }
    public IAsyncRelayCommand AddSelectedCommand { get; }
    public IAsyncRelayCommand ResetSelectedCommand { get; }
    public IAsyncRelayCommand PublishCommand { get; }
    public IAsyncRelayCommand EditProjectCommand { get; }

    /// <summary>由 MainWindowViewModel 注入，用于打开编辑项目对话框</summary>
    public Func<ProjectConfig, Task<ProjectConfig?>>? EditProjectAsync { get; set; }

    public IRelayCommand LaunchProgramCommand { get; }
    public IRelayCommand OpenExplorerCommand { get; }

    public ProjectTabViewModel(ProjectConfig project, CliService cli)
    {
        Project = project;
        _cli = cli;

        var pp = project.ProjectPath;
        RefreshCommand = new AsyncRelayCommand(RefreshAsync, () => !IsBusy && !string.IsNullOrWhiteSpace(pp));
        AddAllCommand = new AsyncRelayCommand(AddAllAsync, () => !IsBusy && UnstagedFiles.Count > 0);
        ResetAllCommand = new AsyncRelayCommand(ResetAllAsync, () => !IsBusy && StagedFiles.Count > 0);
        AddSelectedCommand = new AsyncRelayCommand(AddSelectedAsync, () => !IsBusy && UnstagedFiles.Count > 0);
        ResetSelectedCommand = new AsyncRelayCommand(ResetSelectedAsync, () => !IsBusy && StagedFiles.Count > 0);
        PublishCommand = new AsyncRelayCommand(PublishAsync,
            () => !IsBusy && StagedFiles.Count > 0 && !string.IsNullOrWhiteSpace(NewVersion) && !string.IsNullOrWhiteSpace(CommitMessage));
        EditProjectCommand = new AsyncRelayCommand(EditProjectCmdAsync, () => !IsBusy);

        LaunchProgramCommand = new RelayCommand(LaunchProgram);
        OpenExplorerCommand = new RelayCommand(OpenExplorer);

        UnstagedFiles.CollectionChanged += (_, _) =>
        {
            AddAllCommand.NotifyCanExecuteChanged();
            AddSelectedCommand.NotifyCanExecuteChanged();
        };
        StagedFiles.CollectionChanged += (_, _) =>
        {
            ResetAllCommand.NotifyCanExecuteChanged();
            ResetSelectedCommand.NotifyCanExecuteChanged();
            PublishCommand.NotifyCanExecuteChanged();
        };
    }

    partial void OnIsBusyChanged(bool value)
    {
        RefreshCommand.NotifyCanExecuteChanged();
        AddAllCommand.NotifyCanExecuteChanged();
        ResetAllCommand.NotifyCanExecuteChanged();
        AddSelectedCommand.NotifyCanExecuteChanged();
        ResetSelectedCommand.NotifyCanExecuteChanged();
        PublishCommand.NotifyCanExecuteChanged();
        EditProjectCommand.NotifyCanExecuteChanged();
    }

    // ── Refresh ──────────────────────────────────────────

    // _refreshLock 保护 _refreshCts。IsBusy 仅在 UI 线程上修改。
    private readonly object _refreshLock = new();

    public async Task RefreshAsync()
    {
        CancellationTokenSource newCts;
        lock (_refreshLock)
        {
            if (IsBusy) return;
            IsBusy = true;
            _refreshCts?.Cancel();
            _refreshCts?.Dispose();
            newCts = new CancellationTokenSource();
            _refreshCts = newCts;
        }

        await FetchDataAsync(newCts.Token);
    }

    private async Task FetchDataAsync(CancellationToken cancellationToken)
    {
        var projectPath = Project.ProjectPath;
        StatusMessage = "刷新中...";
        Log.Information("开始刷新: Project={Name}, Path={Path}", Project.DisplayName, projectPath);
        try
        {
            var result = await _cli.GetStatusAsync(projectPath);
            if (cancellationToken.IsCancellationRequested)
                return;

            if (result?.Data == null)
            {
                StatusMessage = $"刷新失败: {result?.ErrorMsg ?? "未知错误"}";
                await MessageBox.ShowAsync(StatusMessage, "错误", MessageBoxIcon.Error);
                return;
            }

            var d = result.Data;
            UnstagedFiles.Clear();
            StagedFiles.Clear();
            foreach (var f in d.Unstaged ?? [])
                if (f.Status != "deleted")
                    UnstagedFiles.Add(FileItem.FromCliItem(f));
            foreach (var f in d.Staged ?? []) StagedFiles.Add(FileItem.FromCliItem(f));

            var logResult = await _cli.GetLogAsync(projectPath);
            if (cancellationToken.IsCancellationRequested)
                return;

            ChangeLogs.Clear();
            if (logResult?.Data != null)
                foreach (var l in logResult.Data)
                    ChangeLogs.Add(l);

            if (ChangeLogs.Count > 0)
                CurrentVersion = ChangeLogs[0].Version ?? string.Empty;

            StatusMessage = $"未暂存 {UnstagedFiles.Count}, 已暂存 {StagedFiles.Count}";
            Log.Information("刷新完成: {Status}", StatusMessage);
        }
        catch (Exception ex)
        {
            if (cancellationToken.IsCancellationRequested)
                return;
            await MessageBox.ShowAsync($"刷新异常: {ex.Message}", "错误", MessageBoxIcon.Error);
            StatusMessage = $"刷新异常: {ex.Message}";
            Log.Error(ex, "刷新异常");
        }
        finally
        {
            IsBusy = false;
        }
    }

    // ── AddAll ───────────────────────────────────────────

    private async Task AddAllAsync()
    {
        if (IsBusy) return;
        IsBusy = true;
        try
        {
            var r = await _cli.AddAllAsync(Project.ProjectPath);
            if (r?.IsSuccess == true)
            {
                StatusMessage = "已添加所有变更";
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg ?? "未知错误"}", "错误", MessageBoxIcon.Error);
                StatusMessage = $"添加失败: {r?.ErrorMsg ?? "未知错误"}";
            }
            await FetchDataAsync(CancellationToken.None);
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"暂存异常: {ex.Message}", "错误", MessageBoxIcon.Error);
            StatusMessage = $"暂存异常: {ex.Message}";
            Log.Error(ex, "暂存异常");
        }
        finally { IsBusy = false; }
    }

    // ── ResetAll ─────────────────────────────────────────

    private async Task ResetAllAsync()
    {
        if (IsBusy) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ResetAllAsync(Project.ProjectPath);
            if (r?.IsSuccess == true)
            {
                StatusMessage = "暂存区已清空";
            }
            else
            {
                await MessageBox.ShowAsync($"重置失败: {r?.ErrorMsg ?? "未知错误"}", "错误", MessageBoxIcon.Error);
                StatusMessage = $"重置失败: {r?.ErrorMsg ?? "未知错误"}";
            }
            await FetchDataAsync(CancellationToken.None);
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"重置异常: {ex.Message}", "错误", MessageBoxIcon.Error);
            StatusMessage = $"重置异常: {ex.Message}";
            Log.Error(ex, "重置异常");
        }
        finally { IsBusy = false; }
    }

    // ── AddSelected ──────────────────────────────────────

    private async Task AddSelectedAsync()
    {
        if (IsBusy) return;
        var files = UnstagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (files.Count == 0) { await MessageBox.ShowAsync("请先勾选文件", "提示", MessageBoxIcon.Warning); return; }
        IsBusy = true;
        try
        {
            var r = await _cli.AddFilesAsync(Project.ProjectPath, files);
            if (r?.IsSuccess == true)
            {
                StatusMessage = $"已暂存 {files.Count} 个文件";
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg ?? "未知错误"}", "错误", MessageBoxIcon.Error);
                StatusMessage = $"添加失败: {r?.ErrorMsg ?? "未知错误"}";
            }
            await FetchDataAsync(CancellationToken.None);
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"暂存异常: {ex.Message}", "错误", MessageBoxIcon.Error);
            StatusMessage = $"暂存异常: {ex.Message}";
            Log.Error(ex, "暂存异常");
        }
        finally { IsBusy = false; }
    }

    // ── ResetSelected ────────────────────────────────────

    private async Task ResetSelectedAsync()
    {
        if (IsBusy) return;
        var toUnstage = StagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (toUnstage.Count == 0) { await MessageBox.ShowAsync("请先勾选文件", "提示", MessageBoxIcon.Warning); return; }
        var keepStaged = StagedFiles.Where(f => !f.IsSelected).Select(f => f.RelativePath).ToList();
        IsBusy = true;
        try
        {
            var resetResult = await _cli.ResetAllAsync(Project.ProjectPath);
            if (resetResult?.IsSuccess != true)
            {
                var errMsg = resetResult?.ErrorMsg ?? "未知错误";
                await MessageBox.ShowAsync($"重置失败: {errMsg}", "错误", MessageBoxIcon.Error);
                StatusMessage = $"取消暂存失败: {errMsg}";
                return;
            }
            if (keepStaged.Count > 0)
            {
                var addResult = await _cli.AddFilesAsync(Project.ProjectPath, keepStaged);
                if (addResult?.IsSuccess == true)
                {
                    StatusMessage = $"已取消暂存 {toUnstage.Count} 个文件";
                }
                else
                {
                    await MessageBox.ShowAsync($"部分文件暂存失败: {addResult?.ErrorMsg ?? "未知错误"}", "错误", MessageBoxIcon.Error);
                    StatusMessage = $"部分文件暂存失败: {addResult?.ErrorMsg ?? "未知错误"}";
                }
            }
            else
            {
                StatusMessage = $"已取消暂存 {toUnstage.Count} 个文件";
            }
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"取消暂存异常: {ex.Message}", "错误", MessageBoxIcon.Error);
            StatusMessage = $"取消暂存异常: {ex.Message}";
            Log.Error(ex, "取消暂存异常");
        }
        finally { IsBusy = false; }
        await FetchDataAsync(CancellationToken.None);
    }

    // ── Publish ──────────────────────────────────────────

    private async Task PublishAsync()
    {
        if (IsBusy) return;
        if (string.IsNullOrWhiteSpace(NewVersion)) { await MessageBox.ShowAsync("请输入版本号", "提示", MessageBoxIcon.Warning); return; }
        if (string.IsNullOrWhiteSpace(CommitMessage)) { await MessageBox.ShowAsync("请输入变更说明", "提示", MessageBoxIcon.Warning); return; }
        if (StagedFiles.Count == 0) { await MessageBox.ShowAsync("暂存区为空，请先暂存文件", "提示", MessageBoxIcon.Warning); return; }

        IsBusy = true;
        IsUploading = true;
        UploadProgressItems.Clear();
        UploadDoneCount = 0;
        UploadTotalCount = StagedFiles.Count;
        UploadProgressPercent = 0;
        IsUploadProgressIndeterminate = true;
        UploadProgressText = $"准备上传 ({StagedFiles.Count} 个文件)...";
        StatusMessage = "发布中...";

        try
        {
            var r = await _cli.PushWithProgressAsync(
                Project.ProjectPath, NewVersion, CommitMessage,
                onProgress: progress =>
                {
                    Dispatcher.UIThread.Post(() => OnUploadProgress(progress));
                },
                afterApplyUpdateScript: AfterApplyUpdateScript,
                setForceUpdate: SetForceUpdate);

            if (r?.IsSuccess == true)
            {
                var stagedCount = StagedFiles.Count;
                var unstagedCount = UnstagedFiles.Count;
                NewVersion = string.Empty;
                CommitMessage = string.Empty;
                AfterApplyUpdateScript = string.Empty;
                StatusMessage = "发布成功，正在刷新...";
                await MessageBox.ShowAsync($"发布成功 ({stagedCount} 暂存, {unstagedCount} 未暂存)", "成功", MessageBoxIcon.Success);
                StatusMessage = $"发布成功 ({stagedCount} 暂存, {unstagedCount} 未暂存)";
                await FetchDataAsync(CancellationToken.None);
            }
            else
            {
                StatusMessage = $"发布失败: {r?.ErrorMsg ?? "未知错误"}";
                await MessageBox.ShowAsync($"发布失败: {r?.ErrorMsg ?? "未知错误"}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            StatusMessage = $"发布异常: {ex.Message}";
            Log.Error(ex, "发布异常");
            await MessageBox.ShowAsync($"发布异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally
        {
            IsUploading = false;
            IsBusy = false;
        }
    }

    private void OnUploadProgress(UploadProgress progress)
    {
        switch (progress.Status)
        {
            case "START":
                IsUploadProgressIndeterminate = false;
                UploadProgressItems.Add(progress);
                UploadProgressPercent = UploadTotalCount > 0
                    ? (double)UploadDoneCount / UploadTotalCount * 100
                    : 0;
                UploadProgressText = $"正在上传 {progress.File} ({UploadDoneCount}/{UploadTotalCount})...";
                break;

            case "DONE":
            case "FAIL":
                // 查找并替换对应索引的项
                for (int i = 0; i < UploadProgressItems.Count; i++)
                {
                    if (UploadProgressItems[i].Index == progress.Index)
                    {
                        UploadProgressItems[i] = progress;
                        break;
                    }
                }
                if (progress.Status == "DONE")
                {
                    UploadDoneCount++;
                }
                UploadProgressPercent = UploadTotalCount > 0
                    ? (double)UploadDoneCount / UploadTotalCount * 100
                    : 0;
                var doneText = progress.Status == "DONE" ? "完成" : "失败";
                UploadProgressText = $"已{doneText} {progress.File} ({UploadDoneCount}/{UploadTotalCount})";
                break;
        }
    }

    // ── EditProject ───────────────────────────────────────

    private async Task EditProjectCmdAsync()
    {
        if (IsBusy) return;
        var editAsync = EditProjectAsync;
        if (editAsync == null) return;
        IsBusy = true;
        try
        {
            var result = await editAsync(Project);
            if (result != null)
            {
                await MessageBox.ShowAsync("项目配置已更新", "成功", MessageBoxIcon.Success);
            }
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"编辑项目异常: {ex.Message}", "错误", MessageBoxIcon.Error);
            StatusMessage = $"编辑项目异常: {ex.Message}";
            Log.Error(ex, "编辑项目异常");
        }
        finally { IsBusy = false; }
    }

    // ── Launch / Explorer ────────────────────────────────

    private void LaunchProgram()
    {
        try
        {
            var exePath = GetMainExeFullPath();
            if (string.IsNullOrWhiteSpace(exePath))
            {
                Dispatcher.UIThread.Post(() =>
                    MessageBox.ShowAsync("请先在项目设置中配置主程序路径", "提示", MessageBoxIcon.Warning));
                return;
            }
            if (!File.Exists(exePath))
            {
                Dispatcher.UIThread.Post(() =>
                    MessageBox.ShowAsync($"主程序不存在: {exePath}", "错误", MessageBoxIcon.Error));
                return;
            }
            Process.Start(new ProcessStartInfo
            {
                FileName = exePath,
                WorkingDirectory = Path.GetDirectoryName(exePath) ?? Project.ProjectPath,
                UseShellExecute = true
            });
            StatusMessage = $"已启动: {exePath}";
            Log.Information("启动程序: {Path}", exePath);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "启动程序失败: {Path}", Project.MainExePath);
            Dispatcher.UIThread.Post(() =>
                MessageBox.ShowAsync($"启动失败: {ex.Message}", "错误", MessageBoxIcon.Error));
        }
    }

    private void OpenExplorer()
    {
        try
        {
            var dir = Project.ProjectPath;
            if (string.IsNullOrWhiteSpace(dir) || !Directory.Exists(dir))
            {
                Dispatcher.UIThread.Post(() =>
                    MessageBox.ShowAsync($"项目目录不存在: {dir}", "错误", MessageBoxIcon.Error));
                return;
            }
            Process.Start(new ProcessStartInfo
            {
                FileName = dir,
                UseShellExecute = true
            });
            StatusMessage = $"已打开: {dir}";
            Log.Information("打开文件管理器: {Path}", dir);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开文件管理器失败: {Path}", Project.ProjectPath);
            Dispatcher.UIThread.Post(() =>
                MessageBox.ShowAsync($"打开失败: {ex.Message}", "错误", MessageBoxIcon.Error));
        }
    }

    private string? GetMainExeFullPath()
    {
        var mainExePath = Project.MainExePath;
        if (string.IsNullOrWhiteSpace(mainExePath))
            return null;
        // 如果是绝对路径，确保在项目目录内
        if (Path.IsPathRooted(mainExePath))
        {
            var projectPath = Project.ProjectPath;
            if (string.IsNullOrWhiteSpace(projectPath))
                return null;
            var normalizedProject = Path.GetFullPath(projectPath).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
            var normalizedExe = Path.GetFullPath(mainExePath);
            if (!normalizedExe.StartsWith(normalizedProject + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
                return null;
            return mainExePath;
        }
        // 否则相对于项目路径
        var projPath = Project.ProjectPath;
        if (string.IsNullOrWhiteSpace(projPath))
            return null;
        var fullPath = Path.GetFullPath(Path.Combine(projPath, mainExePath));
        // 确保解析后的路径仍在项目目录内（防止 ../../ 路径穿越）
        var normProject = Path.GetFullPath(projPath).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        if (!fullPath.StartsWith(normProject + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
            return null;
        return fullPath;
    }
}
