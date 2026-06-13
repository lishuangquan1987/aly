using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using Ursa.Controls;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using ZapPublish.Models.Cli;
using ZapPublish.Models.Local;
using ZapPublish.Services;
using Serilog;

namespace ZapPublish.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly CliService _cli;
    private readonly ConfigService _cfg;
    private readonly IDialogService _dialog;
    private CancellationTokenSource? _refreshCts;

    [ObservableProperty] private ObservableCollection<ProjectConfig> _projects = new();
    [NotifyCanExecuteChangedFor(nameof(AddSelectedCommand))]
    [NotifyCanExecuteChangedFor(nameof(EditProjectCommand))]
    [NotifyCanExecuteChangedFor(nameof(RemoveProjectCommand))]
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    [NotifyCanExecuteChangedFor(nameof(ResetSelectedCommand))]
    [NotifyCanExecuteChangedFor(nameof(ResetAllCommand))]
    [NotifyCanExecuteChangedFor(nameof(AddAllCommand))]
    [ObservableProperty] private ProjectConfig? _selectedProject;
    [ObservableProperty] private ObservableCollection<FileItem> _unstagedFiles = new();
    [ObservableProperty] private ObservableCollection<FileItem> _stagedFiles = new();
    [ObservableProperty] private ObservableCollection<ChangeLog> _changeLogs = new();
    [ObservableProperty] private string _statusMessage = "就绪";
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    [ObservableProperty] private string _newVersion = string.Empty;
    [NotifyCanExecuteChangedFor(nameof(PublishCommand))]
    [ObservableProperty] private string _commitMessage = string.Empty;
    [ObservableProperty] private string _currentVersion = string.Empty;
    [ObservableProperty] private bool _isBusy;
    [ObservableProperty] private bool _isUploading;
    [ObservableProperty] private int _uploadProgress;
    [ObservableProperty] private int _uploadTotal;
    [ObservableProperty] private bool _isCliFound;
    [ObservableProperty] private string _cliPath = string.Empty;
    [ObservableProperty] private string _afterApplyUpdateScript = string.Empty;
    [ObservableProperty] private bool _setForceUpdate;

    [NotifyCanExecuteChangedFor(nameof(AddSelectedCommand))]
    [ObservableProperty] private FileItem? _selectedUnStagedFile;
    [NotifyCanExecuteChangedFor(nameof(ResetSelectedCommand))]
    [ObservableProperty] private FileItem? _selectedStagedFile;

    public IAsyncRelayCommand RefreshCommand { get; }
    public IAsyncRelayCommand AddAllCommand { get; }
    public IAsyncRelayCommand ResetAllCommand { get; }
    public IAsyncRelayCommand AddSelectedCommand { get; }
    public IAsyncRelayCommand ResetSelectedCommand { get; }
    public IAsyncRelayCommand PublishCommand { get; }
    public IAsyncRelayCommand AddProjectCommand { get; }
    public IAsyncRelayCommand AddLocalProjectCommand { get; }
    public IAsyncRelayCommand RemoveProjectCommand { get; }
    public IAsyncRelayCommand EditProjectCommand { get; }

    public MainWindowViewModel(CliService cli, ConfigService cfg, IDialogService dialog)
    {
        _cli = cli;
        _cfg = cfg;
        _dialog = dialog;
        IsCliFound = _cli.Found;
        CliPath = _cli.CliPath;
        Log.Information("MainWindowViewModel 初始化: CliFound={Found}, CliPath={Path}", IsCliFound, CliPath);

        RefreshCommand = new AsyncRelayCommand(RefreshAsync, () => SelectedProject != null);
        AddAllCommand = new AsyncRelayCommand(AddAllAsync, () => this.UnstagedFiles.Count > 0);
        ResetAllCommand = new AsyncRelayCommand(ResetAllAsync, () => SelectedProject != null && this.StagedFiles.Count > 0);
        AddSelectedCommand = new AsyncRelayCommand(AddSelectedAsync, () => SelectedProject != null && this.SelectedUnStagedFile != null);
        ResetSelectedCommand = new AsyncRelayCommand(ResetSelectedAsync, () => SelectedProject != null && this.SelectedStagedFile != null);
        PublishCommand = new AsyncRelayCommand(PublishAsync, () => SelectedProject != null && StagedFiles.Count > 0 && !string.IsNullOrEmpty(this.NewVersion) && !string.IsNullOrEmpty(CommitMessage));
        AddProjectCommand = new AsyncRelayCommand(AddProjectAsync);
        AddLocalProjectCommand = new AsyncRelayCommand(AddLocalProjectAsync);
        RemoveProjectCommand = new AsyncRelayCommand(RemoveProjectAsync, () => SelectedProject != null);
        EditProjectCommand = new AsyncRelayCommand(EditProjectAsync, () => SelectedProject != null);

        this.UnstagedFiles.CollectionChanged += (_, _) => this.AddAllCommand.NotifyCanExecuteChanged();
        this.StagedFiles.CollectionChanged += (_, _) =>
        {
            this.ResetAllCommand.NotifyCanExecuteChanged();
            this.PublishCommand?.NotifyCanExecuteChanged();
        };

        LoadProjects();


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

    partial void OnSelectedProjectChanged(ProjectConfig? value)
    {
        Log.Information("切换项目: {Name}", value?.DisplayName ?? "(无)");
        RefreshCommand.NotifyCanExecuteChanged();
        AddAllCommand.NotifyCanExecuteChanged();
        ResetAllCommand.NotifyCanExecuteChanged();
        AddSelectedCommand.NotifyCanExecuteChanged();
        ResetSelectedCommand.NotifyCanExecuteChanged();
        PublishCommand.NotifyCanExecuteChanged();
        RemoveProjectCommand.NotifyCanExecuteChanged();
        EditProjectCommand.NotifyCanExecuteChanged();
        if (value != null) _ = RefreshAsync().ContinueWith(t =>
        {
            if (t.IsFaulted && t.Exception != null)
                Log.Error(t.Exception, "RefreshAsync 未捕获异常");
        }, TaskContinuationOptions.OnlyOnFaulted);
    }

    partial void OnStagedFilesChanged(ObservableCollection<FileItem> value)
    {
        PublishCommand.NotifyCanExecuteChanged();
    }

    private void LoadProjects()
    {
        Log.Information("加载本地项目配置");
        Projects.Clear();
        foreach (var c in _cfg.LoadProjects())
        {
            Log.Debug("加载项目: DisplayName={Name}, Path={Path}",
                c.DisplayName, c.ProjectPath);
            Projects.Add(c);
        }
        Log.Information("共加载 {Count} 个项目", Projects.Count);
        if (Projects.Count > 0) SelectedProject = Projects[0];
    }

    private async Task RefetchDataAsync()
    {
        var project = SelectedProject;
        var projectPath = project?.ProjectPath;
        if (project == null || string.IsNullOrWhiteSpace(projectPath))
        {
            StatusMessage = "请先选择一个项目";
            return;
        }
        StatusMessage = "刷新中...";
        Log.Information("开始刷新: Project={Name}, Path={Path}", project.DisplayName, projectPath);

        try
        {
            var result = await _cli.GetStatusAsync(projectPath);
            if (result?.Data == null)
            {
                StatusMessage = $"刷新失败: {result?.ErrorMsg ?? "无数据返回"}";
                return;
            }

            var d = result.Data;
            UnstagedFiles.Clear();
            StagedFiles.Clear();
            foreach (var f in d.Unstaged ?? []) UnstagedFiles.Add(FileItem.FromCliItem(f));
            foreach (var f in d.Staged ?? []) StagedFiles.Add(FileItem.FromCliItem(f));

            var logResult = await _cli.GetLogAsync(projectPath);
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
            await MessageBox.ShowAsync($"刷新异常: {ex.Message}", "错误");
            StatusMessage = $"刷新异常: {ex.Message}";
            Log.Error(ex, "刷新异常");
        }
    }

    private async Task RefreshAsync()
    {
        if (IsBusy) return;
        _refreshCts?.Cancel();
        _refreshCts?.Dispose();
        _refreshCts = new CancellationTokenSource();
        var ct = _refreshCts.Token;

        IsBusy = true;
        try
        {
            await RefetchDataAsync();
            if (ct.IsCancellationRequested) return;
        }
        finally { IsBusy = false; }
    }

    private async Task AddAllAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { await MessageBox.ShowAsync("请先选择一个项目", "提示"); StatusMessage = "请先选择一个项目"; return; }
        IsBusy = true;
        try
        {
            var r = await _cli.AddAllAsync(projectPath);
            if (r?.IsSuccess == true)
            {
                await MessageBox.ShowAsync("已添加所有变更", "成功");
                StatusMessage = "已添加所有变更";
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg}", "错误");
                StatusMessage = $"添加失败: {r?.ErrorMsg}";
            }
            await RefetchDataAsync();
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"暂存异常: {ex.Message}", "错误");
            StatusMessage = $"暂存异常: {ex.Message}";
            Log.Error(ex, "暂存异常");
        }
        finally { IsBusy = false; }
    }

    private async Task ResetAllAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { await MessageBox.ShowAsync("请先选择一个项目", "提示"); StatusMessage = "请先选择一个项目"; return; }
        IsBusy = true;
        try
        {
            var r = await _cli.ResetAllAsync(projectPath);
            if (r?.IsSuccess == true)
            {
                await MessageBox.ShowAsync("暂存区已清空", "成功");
                StatusMessage = "暂存区已清空";
            }
            else
            {
                await MessageBox.ShowAsync($"重置失败: {r?.ErrorMsg}", "错误");
                StatusMessage = $"重置失败: {r?.ErrorMsg}";
            }
            await RefetchDataAsync();
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"重置异常: {ex.Message}", "错误");
            StatusMessage = $"重置异常: {ex.Message}";
            Log.Error(ex, "重置异常");
        }
        finally { IsBusy = false; }
    }

    private async Task AddSelectedAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { await MessageBox.ShowAsync("请先选择一个项目", "提示"); StatusMessage = "请先选择一个项目"; return; }
        var files = UnstagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (files.Count == 0) { await MessageBox.ShowAsync("请先勾选文件", "提示"); StatusMessage = "请先勾选文件"; return; }
        IsBusy = true;
        try
        {
            var r = await _cli.AddFilesAsync(projectPath, files);
            if (r?.IsSuccess == true)
            {
                await MessageBox.ShowAsync($"已暂存 {files.Count} 个文件", "成功");
                StatusMessage = $"已暂存 {files.Count} 个文件";
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg}", "错误");
                StatusMessage = $"添加失败: {r?.ErrorMsg}";
            }
            await RefetchDataAsync();
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"暂存异常: {ex.Message}", "错误");
            StatusMessage = $"暂存异常: {ex.Message}";
            Log.Error(ex, "暂存异常");
        }
        finally { IsBusy = false; }
    }

    private async Task ResetSelectedAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { await MessageBox.ShowAsync("请先选择一个项目", "提示"); StatusMessage = "请先选择一个项目"; return; }
        var toUnstage = StagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (toUnstage.Count == 0) { await MessageBox.ShowAsync("请先勾选文件", "提示"); StatusMessage = "请先勾选文件"; return; }
        var keepStaged = StagedFiles.Where(f => !f.IsSelected).Select(f => f.RelativePath).ToList();
        IsBusy = true;
        try
        {
            await _cli.ResetAllAsync(projectPath);
            if (keepStaged.Count > 0)
            {
                var addResult = await _cli.AddFilesAsync(projectPath, keepStaged);
                if (addResult?.IsSuccess == true)
                {
                    await MessageBox.ShowAsync($"已取消暂存 {toUnstage.Count} 个文件", "成功");
                    StatusMessage = $"已取消暂存 {toUnstage.Count} 个文件";
                }
                else
                {
                    await MessageBox.ShowAsync($"部分文件暂存失败: {addResult?.ErrorMsg}", "错误");
                    StatusMessage = $"部分文件暂存失败: {addResult?.ErrorMsg}";
                }
            }
            else
            {
                await MessageBox.ShowAsync($"已取消暂存 {toUnstage.Count} 个文件", "成功");
                StatusMessage = $"已取消暂存 {toUnstage.Count} 个文件";
            }
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"取消暂存异常: {ex.Message}", "错误");
            StatusMessage = $"取消暂存异常: {ex.Message}";
            Log.Error(ex, "取消暂存异常");
        }
        finally { IsBusy = false; }
        await RefetchDataAsync();
    }

    private async Task PublishAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { await MessageBox.ShowAsync("请先选择一个项目", "提示"); StatusMessage = "请先选择一个项目"; return; }
        if (string.IsNullOrWhiteSpace(NewVersion)) { await MessageBox.ShowAsync("请输入版本号", "提示"); StatusMessage = "请输入版本号"; return; }
        if (string.IsNullOrWhiteSpace(CommitMessage)) { await MessageBox.ShowAsync("请输入变更说明", "提示"); StatusMessage = "请输入变更说明"; return; }
        if (StagedFiles.Count == 0) { await MessageBox.ShowAsync("暂存区为空，请先暂存文件", "提示"); StatusMessage = "暂存区为空，请先暂存文件"; return; }

        IsBusy = true;
        IsUploading = true;
        StatusMessage = "发布中...";
        try
        {
            var r = await _cli.PushAsync(projectPath, NewVersion, CommitMessage, AfterApplyUpdateScript, SetForceUpdate);
            if (r?.IsSuccess == true)
            {
                var stagedCount = StagedFiles.Count;
                var unstagedCount = UnstagedFiles.Count;
                NewVersion = string.Empty;
                CommitMessage = string.Empty;
                AfterApplyUpdateScript = string.Empty;
                StatusMessage = "发布成功，正在刷新...";
                await RefetchDataAsync();
                await MessageBox.ShowAsync($"发布成功 ({stagedCount} 暂存, {unstagedCount} 未暂存)", "成功");
                StatusMessage = $"发布成功 ({stagedCount} 暂存, {unstagedCount} 未暂存)";
            }
            else
            {
                await MessageBox.ShowAsync($"发布失败: {r?.ErrorMsg}", "错误");
                StatusMessage = $"发布失败: {r?.ErrorMsg}";
            }
        }
        catch (Exception ex)
        {
            await MessageBox.ShowAsync($"发布异常: {ex.Message}", "错误");
            StatusMessage = $"发布异常: {ex.Message}";
            Log.Error(ex, "发布异常");
        }
        finally
        {
            IsUploading = false;
            IsBusy = false;
        }
    }

    private async Task AddProjectAsync()
    {
        Log.Information("打开初始化项目对话框");
        var cfg = await _dialog.ShowAddProjectDialogAsync();
        if (cfg != null)
        {
            _cfg.AddProject(cfg);
            Projects.Add(cfg);
            SelectedProject = cfg;
            await MessageBox.ShowAsync($"项目 \"{cfg.DisplayName}\" 已添加", "成功");
        }
    }

    private async Task AddLocalProjectAsync()
    {
        Log.Information("打开添加本地项目对话框");
        var cfg = await _dialog.ShowAddLocalProjectDialogAsync();
        if (cfg != null)
        {
            _cfg.AddProject(cfg);
            Projects.Add(cfg);
            SelectedProject = cfg;
            await MessageBox.ShowAsync($"项目 \"{cfg.DisplayName}\" 已添加", "成功");
        }
    }

    private async Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return;
        var name = SelectedProject.DisplayName;
        _cfg.RemoveProject(name);
        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        await MessageBox.ShowAsync($"项目 \"{name}\" 已移除", "成功");
    }

    private async Task EditProjectAsync()
    {
        if (SelectedProject == null) return;
        var result = await _dialog.ShowEditProjectDialogAsync(SelectedProject);
        if (result != null)
        {
            _cfg.UpdateProject(result);
            var idx = Projects.IndexOf(SelectedProject);
            if (idx >= 0) Projects[idx] = result;
            SelectedProject = result;
            await MessageBox.ShowAsync("项目配置已更新", "成功");
        }
    }
}
