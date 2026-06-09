using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishGui.Models.Cli;
using PublishGui.Models.Local;
using PublishGui.Services;
using Serilog;

namespace PublishGui.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly CliService _cli;
    private readonly ConfigService _cfg;

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

    public IAsyncRelayCommand RefreshCommand { get; }
    public IAsyncRelayCommand AddAllCommand { get; }
    public IAsyncRelayCommand ResetAllCommand { get; }
    public IAsyncRelayCommand AddSelectedCommand { get; }
    public IAsyncRelayCommand ResetSelectedCommand { get; }
    public IAsyncRelayCommand PublishCommand { get; }
    public IAsyncRelayCommand AddProjectCommand { get; }
    public IAsyncRelayCommand RemoveProjectCommand { get; }

    public MainWindowViewModel(CliService cli, ConfigService cfg)
    {
        _cli = cli;
        _cfg = cfg;
        IsCliFound = _cli.Found;
        CliPath = _cli.CliPath;
        Log.Information("MainWindowViewModel 初始化: CliFound={Found}, CliPath={Path}", IsCliFound, CliPath);

        RefreshCommand = new AsyncRelayCommand(RefreshAsync);
        AddAllCommand = new AsyncRelayCommand(AddAllAsync);
        ResetAllCommand = new AsyncRelayCommand(ResetAllAsync);
        AddSelectedCommand = new AsyncRelayCommand(AddSelectedAsync);
        ResetSelectedCommand = new AsyncRelayCommand(ResetSelectedAsync);
        PublishCommand = new AsyncRelayCommand(PublishAsync);
        AddProjectCommand = new AsyncRelayCommand(AddProjectAsync);
        RemoveProjectCommand = new AsyncRelayCommand(RemoveProjectAsync);

        LoadProjects();
    }

    private void LoadProjects()
    {
        Log.Information("加载本地项目配置");
        Projects.Clear();
        foreach (var c in _cfg.LoadProjects())
        {
            Log.Debug("加载项目: Name={Name}, Server={Server}, Path={Path}, Id={Id}",
                c.ProjectName, c.ServerUrl, c.ProjectPath, c.ProjectId);
            Projects.Add(c);
        }
        Log.Information("共加载 {Count} 个项目", Projects.Count);
        if (Projects.Count > 0) SelectedProject = Projects[0];
    }

    partial void OnSelectedProjectChanged(ProjectConfig? value)
    {
        Log.Information("切换项目: {Name}", value?.ProjectName ?? "(无)");
        if (value != null) _ = RefreshAsync();
    }

    private async Task RefreshAsync()
    {
        if (SelectedProject == null || IsBusy)
        {
            Log.Debug("刷新跳过: SelectedProject={P}, IsBusy={B}", SelectedProject?.ProjectName, IsBusy);
            return;
        }
        IsBusy = true;
        try
        {
            await RefetchDataAsync();
        }
        finally { IsBusy = false; }
    }

    private async Task RefetchDataAsync()
    {
        if (SelectedProject == null) return;
        StatusMessage = "刷新中...";
        Log.Information("开始刷新: Project={Name}, Path={Path}",
            SelectedProject.ProjectName, SelectedProject.ProjectPath);

        try
        {
            Log.Debug("调用 GetStatusAsync: {Path}", SelectedProject.ProjectPath);
            var result = await _cli.GetStatusAsync(SelectedProject.ProjectPath);
            Log.Information("GetStatus 结果: IsSuccess={Ok}, ErrorMsg={Err}, Unstaged={U}, Staged={S}",
                result?.IsSuccess, result?.ErrorMsg,
                result?.Data?.Unstaged?.Count ?? 0,
                result?.Data?.Staged?.Count ?? 0);

            if (result?.Data == null)
            {
                StatusMessage = $"刷新失败: {result?.ErrorMsg ?? "无数据返回"}";
                Log.Warning("刷新失败: result={Result}", result);
                return;
            }

            UnstagedFiles.Clear();
            StagedFiles.Clear();
            foreach (var f in result.Data.Unstaged)
                UnstagedFiles.Add(FileItem.FromCliItem(f));
            foreach (var f in result.Data.Staged)
                StagedFiles.Add(FileItem.FromCliItem(f));

            Log.Debug("调用 GetLogAsync: {Path}", SelectedProject.ProjectPath);
            var logResult = await _cli.GetLogAsync(SelectedProject.ProjectPath);
            Log.Information("GetLog 结果: IsSuccess={Ok}, Count={Count}",
                logResult?.IsSuccess, logResult?.Data?.Count ?? 0);

            ChangeLogs.Clear();
            if (logResult?.Data != null)
                foreach (var l in logResult.Data)
                    ChangeLogs.Add(l);
            if (ChangeLogs.Count > 0)
                CurrentVersion = ChangeLogs[0].Version;

            StatusMessage = $"未暂存 {UnstagedFiles.Count}, 已暂存 {StagedFiles.Count}";
            Log.Information("刷新完成: {Status}", StatusMessage);
        }
        catch (Exception ex)
        {
            StatusMessage = $"刷新异常: {ex.Message}";
            Log.Error(ex, "刷新异常");
        }
    }

    private async Task AddAllAsync()
    {
        if (SelectedProject == null || IsBusy) return;
        IsBusy = true;
        try
        {
            Log.Information("暂存全部: Project={Name}", SelectedProject.ProjectName);
            var r = await _cli.AddAllAsync(SelectedProject.ProjectPath);
            Log.Information("暂存全部结果: IsSuccess={Ok}, ErrorMsg={Err}", r?.IsSuccess, r?.ErrorMsg);
            StatusMessage = r?.IsSuccess == true ? "已添加所有变更" : $"添加失败: {r?.ErrorMsg}";
            await RefetchDataAsync();
        }
        finally { IsBusy = false; }
    }

    private async Task ResetAllAsync()
    {
        if (SelectedProject == null || IsBusy) return;
        IsBusy = true;
        Log.Information("清空暂存区: Project={Name}", SelectedProject.ProjectName);
        try
        {
            var r = await _cli.ResetAllAsync(SelectedProject.ProjectPath);
            Log.Information("清空暂存区结果: IsSuccess={Ok}, ErrorMsg={Err}", r?.IsSuccess, r?.ErrorMsg);
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
        if (SelectedProject == null || IsBusy) return;
        var files = UnstagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (files.Count == 0) { StatusMessage = "未选中文件"; return; }
        IsBusy = true;
        Log.Information("暂存选中文件: Project={Name}, Count={Count}", SelectedProject.ProjectName, files.Count);
        try
        {
            var r = await _cli.AddFilesAsync(SelectedProject.ProjectPath, files);
            Log.Information("暂存选中结果: IsSuccess={Ok}, ErrorMsg={Err}", r?.IsSuccess, r?.ErrorMsg);
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
        if (SelectedProject == null || IsBusy) return;
        var files = StagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (files.Count == 0) { StatusMessage = "未选中文件"; return; }
        IsBusy = true;
        Log.Information("取消暂存选中: Project={Name}, Count={Count}", SelectedProject.ProjectName, files.Count);
        var keepPaths = UnstagedFiles.Where(f => !f.IsSelected).Select(f => f.RelativePath).ToList();
        try
        {
            await _cli.ResetAllAsync(SelectedProject.ProjectPath);
            if (keepPaths.Count > 0)
            {
                var addResult = await _cli.AddFilesAsync(SelectedProject.ProjectPath, keepPaths);
                if (addResult?.IsSuccess != true)
                {
                    Log.Warning("取消暂存后重新暂存失败: {Error}", addResult?.ErrorMsg);
                    StatusMessage = $"部分文件暂存失败: {addResult?.ErrorMsg}";
                }
                else
                {
                    StatusMessage = $"已取消暂存 {files.Count} 个文件";
                }
            }
            else
            {
                StatusMessage = $"已取消暂存 {files.Count} 个文件";
            }
        }
        catch (Exception ex)
        {
            StatusMessage = $"取消暂存异常: {ex.Message}";
            Log.Error(ex, "取消暂存异常");
        }
        finally
        {
            IsBusy = false;
            await RefetchDataAsync();
        }
    }

    private async Task PublishAsync()
    {
        if (SelectedProject == null || IsBusy) return;
        if (string.IsNullOrWhiteSpace(NewVersion)) { StatusMessage = "请输入版本号"; return; }
        if (string.IsNullOrWhiteSpace(CommitMessage)) { StatusMessage = "请输入变更说明"; return; }
        if (StagedFiles.Count == 0) { StatusMessage = "暂存区为空，请先暂存文件"; return; }

        IsBusy = true;
        StatusMessage = "发布中...";
        Log.Information("开始发布: Project={Name}, Version={Version}, Files={Count}",
            SelectedProject.ProjectName, NewVersion, StagedFiles.Count);

        try
        {
            var r = await _cli.PushAsync(SelectedProject.ProjectPath, NewVersion, CommitMessage);
            Log.Information("发布结果: IsSuccess={Ok}, ErrorMsg={Err}", r?.IsSuccess, r?.ErrorMsg);
            if (r?.IsSuccess == true)
            {
                StatusMessage = $"发布成功: {NewVersion}";
                NewVersion = string.Empty;
                CommitMessage = string.Empty;
                await RefetchDataAsync();
            }
            else StatusMessage = $"发布失败: {r?.ErrorMsg}";
        }
        catch (Exception ex)
        {
            StatusMessage = $"发布异常: {ex.Message}";
            Log.Error(ex, "发布异常");
        }
        finally { IsBusy = false; }
    }

    private async Task AddProjectAsync()
    {
        Log.Information("打开添加项目对话框");
        var dlg = new Views.Dialogs.AddProjectDialog(_cli);
        var cfg = await dlg.ShowDialog<ProjectConfig?>(
            (Avalonia.Application.Current?.ApplicationLifetime as Avalonia.Controls.ApplicationLifetimes.IClassicDesktopStyleApplicationLifetime)?.MainWindow!);
        if (cfg != null)
        {
            Log.Information("添加项目: Name={Name}, Server={Server}, Path={Path}, Id={Id}",
                cfg.ProjectName, cfg.ServerUrl, cfg.ProjectPath, cfg.ProjectId);
            _cfg.AddProject(cfg);
            Projects.Add(cfg);

            StatusMessage = "正在初始化本地配置...";
            Log.Information("初始化 publish-cli 本地配置: Path={Path}, Server={Server}, Name={Name}, Id={Id}",
                cfg.ProjectPath, cfg.ServerUrl, cfg.ProjectName, cfg.ProjectId);
            var initResult = await _cli.ConfigInitAsync(cfg.ProjectPath, cfg.ServerUrl, cfg.ProjectName, cfg.ProjectId);
            Log.Information("初始化本地配置结果: IsSuccess={Ok}, ErrorMsg={Err}", initResult?.IsSuccess, initResult?.ErrorMsg);
            if (initResult?.IsSuccess != true)
            {
                StatusMessage = $"本地配置初始化失败: {initResult?.ErrorMsg}";
                Log.Warning("本地配置初始化失败: {Error}", initResult?.ErrorMsg);
            }

            SelectedProject = cfg;
        }
        else
        {
            Log.Debug("添加项目对话框已取消");
        }
    }

    private Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return Task.CompletedTask;
        Log.Information("移除项目: {Name}", SelectedProject.ProjectName);
        _cfg.RemoveProject(SelectedProject.ProjectName);
        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        return Task.CompletedTask;
    }
}
