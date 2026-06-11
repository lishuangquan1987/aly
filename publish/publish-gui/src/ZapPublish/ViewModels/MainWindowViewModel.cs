using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
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

    [ObservableProperty] private ObservableCollection<ProjectConfig> _projects = new();
    [ObservableProperty] private ProjectConfig? _selectedProject;
    [ObservableProperty] private ObservableCollection<FileItem> _unstagedFiles = new();
    [ObservableProperty] private ObservableCollection<FileItem> _stagedFiles = new();
    [ObservableProperty] private ObservableCollection<ChangeLog> _changeLogs = new();
    [ObservableProperty] private string _statusMessage = "就绪";
    [ObservableProperty] private string _newVersion = string.Empty;
    [ObservableProperty] private string _commitMessage = string.Empty;
    [ObservableProperty] private string _currentVersion = string.Empty;
    [ObservableProperty] private bool _isBusy;
    [ObservableProperty] private bool _isUploading;
    [ObservableProperty] private int _uploadProgress;
    [ObservableProperty] private int _uploadTotal;
    [ObservableProperty] private bool _isCliFound;
    [ObservableProperty] private string _cliPath = string.Empty;
    [ObservableProperty] private string _afterApplyUpdateScript = string.Empty;

    public IAsyncRelayCommand RefreshCommand { get; }
    public IAsyncRelayCommand AddAllCommand { get; }
    public IAsyncRelayCommand ResetAllCommand { get; }
    public IAsyncRelayCommand AddSelectedCommand { get; }
    public IAsyncRelayCommand ResetSelectedCommand { get; }
    public IAsyncRelayCommand PublishCommand { get; }
    public IAsyncRelayCommand AddProjectCommand { get; }
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

        RefreshCommand       = new AsyncRelayCommand(RefreshAsync,       () => SelectedProject != null && !IsBusy);
        AddAllCommand        = new AsyncRelayCommand(AddAllAsync,        () => SelectedProject != null && !IsBusy);
        ResetAllCommand      = new AsyncRelayCommand(ResetAllAsync,      () => SelectedProject != null && !IsBusy);
        AddSelectedCommand   = new AsyncRelayCommand(AddSelectedAsync,   () => SelectedProject != null && !IsBusy);
        ResetSelectedCommand = new AsyncRelayCommand(ResetSelectedAsync, () => SelectedProject != null && !IsBusy);
        PublishCommand       = new AsyncRelayCommand(PublishAsync,       () => SelectedProject != null && !IsBusy && StagedFiles.Count > 0);
        AddProjectCommand    = new AsyncRelayCommand(AddProjectAsync);
        RemoveProjectCommand = new AsyncRelayCommand(RemoveProjectAsync, () => SelectedProject != null);
        EditProjectCommand   = new AsyncRelayCommand(EditProjectAsync,   () => SelectedProject != null);

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
        if (value != null) _ = RefreshAsync();
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
            var unstaged = new ObservableCollection<FileItem>();
            var staged = new ObservableCollection<FileItem>();
            foreach (var f in d.Unstaged ?? []) unstaged.Add(FileItem.FromCliItem(f));
            foreach (var f in d.Staged ?? []) staged.Add(FileItem.FromCliItem(f));

            UnstagedFiles = unstaged;
            StagedFiles = staged;

            var logResult = await _cli.GetLogAsync(projectPath);
            var logs = new ObservableCollection<ChangeLog>();
            if (logResult?.Data != null)
                foreach (var l in logResult.Data)
                    logs.Add(l);

            ChangeLogs = logs;
            if (logs.Count > 0)
                CurrentVersion = logs[0].Version ?? string.Empty;

            StatusMessage = $"未暂存 {unstaged.Count}, 已暂存 {staged.Count}";
            Log.Information("刷新完成: {Status}", StatusMessage);
        }
        catch (Exception ex)
        {
            StatusMessage = $"刷新异常: {ex.Message}";
            Log.Error(ex, "刷新异常");
        }
    }

    private async Task RefreshAsync()
    {
        if (IsBusy) return;
        IsBusy = true;
        try { await RefetchDataAsync(); }
        finally { IsBusy = false; }
    }

    private async Task AddAllAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { StatusMessage = "项目路径无效"; return; }
        IsBusy = true;
        try
        {
            var r = await _cli.AddAllAsync(projectPath);
            StatusMessage = r?.IsSuccess == true ? "已添加所有变更" : $"添加失败: {r?.ErrorMsg}";
            await RefetchDataAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"暂存异常: {ex.Message}";
            Log.Error(ex, "暂存异常");
        }
        finally { IsBusy = false; }
    }

    private async Task ResetAllAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { StatusMessage = "项目路径无效"; return; }
        IsBusy = true;
        try
        {
            var r = await _cli.ResetAllAsync(projectPath);
            StatusMessage = r?.IsSuccess == true ? "暂存区已清空" : $"重置失败: {r?.ErrorMsg}";
            await RefetchDataAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"重置异常: {ex.Message}";
            Log.Error(ex, "重置异常");
        }
        finally { IsBusy = false; }
    }

    private async Task AddSelectedAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { StatusMessage = "项目路径无效"; return; }
        var files = UnstagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (files.Count == 0) { StatusMessage = "请先勾选文件"; return; }
        IsBusy = true;
        try
        {
            var r = await _cli.AddFilesAsync(projectPath, files);
            StatusMessage = r?.IsSuccess == true ? $"已暂存 {files.Count} 个文件" : $"添加失败: {r?.ErrorMsg}";
            await RefetchDataAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"暂存异常: {ex.Message}";
            Log.Error(ex, "暂存异常");
        }
        finally { IsBusy = false; }
    }

    private async Task ResetSelectedAsync()
    {
        if (IsBusy) return;
        var projectPath = SelectedProject?.ProjectPath;
        if (string.IsNullOrWhiteSpace(projectPath)) { StatusMessage = "项目路径无效"; return; }
        var toUnstage = StagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (toUnstage.Count == 0) { StatusMessage = "请先勾选文件"; return; }
        var keepStaged = StagedFiles.Where(f => !f.IsSelected).Select(f => f.RelativePath).ToList();
        IsBusy = true;
        try
        {
            await _cli.ResetAllAsync(projectPath);
            if (keepStaged.Count > 0)
            {
                var addResult = await _cli.AddFilesAsync(projectPath, keepStaged);
                StatusMessage = addResult?.IsSuccess == true
                    ? $"已取消暂存 {toUnstage.Count} 个文件"
                    : $"部分文件暂存失败: {addResult?.ErrorMsg}";
            }
            else
            {
                StatusMessage = $"已取消暂存 {toUnstage.Count} 个文件";
            }
        }
        catch (Exception ex)
        {
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
        if (string.IsNullOrWhiteSpace(projectPath)) { StatusMessage = "项目路径无效"; return; }
        if (string.IsNullOrWhiteSpace(NewVersion)) { StatusMessage = "请输入版本号"; return; }
        if (string.IsNullOrWhiteSpace(CommitMessage)) { StatusMessage = "请输入变更说明"; return; }
        if (StagedFiles.Count == 0) { StatusMessage = "暂存区为空，请先暂存文件"; return; }

        IsBusy = true;
        IsUploading = true;
        StatusMessage = "发布中...";
        try
        {
            var r = await _cli.PushAsync(projectPath, NewVersion, CommitMessage, AfterApplyUpdateScript);
            if (r?.IsSuccess == true)
            {
                NewVersion = string.Empty;
                CommitMessage = string.Empty;
                AfterApplyUpdateScript = string.Empty;
                StatusMessage = "发布成功，正在刷新...";
                await RefetchDataAsync();
                StatusMessage = $"发布成功";
            }
            else StatusMessage = $"发布失败: {r?.ErrorMsg}";
        }
        catch (Exception ex)
        {
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
        Log.Information("打开添加项目对话框");
        var cfg = await _dialog.ShowAddProjectDialogAsync();
        if (cfg != null)
        {
            _cfg.AddProject(cfg);
            Projects.Add(cfg);
            SelectedProject = cfg;
        }
        else
        {
            StatusMessage = "无法打开对话框";
        }
    }

    private Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return Task.CompletedTask;
        _cfg.RemoveProject(SelectedProject.DisplayName);
        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        return Task.CompletedTask;
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
        }
    }
}
