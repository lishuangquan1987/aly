using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Ursa.Controls;
using ZapPublish.Models.Cli;
using ZapPublish.Models.Local;
using ZapPublish.Services;
using Serilog;

namespace ZapPublish.ViewModels;

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
        StatusMessage = "发布中...";
        try
        {
            var r = await _cli.PushAsync(Project.ProjectPath, NewVersion, CommitMessage, AfterApplyUpdateScript, SetForceUpdate);
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
}
