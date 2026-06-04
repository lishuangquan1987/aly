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

    public MainWindowViewModel(CliService cli, ConfigService cfg)
    {
        _cli = cli;
        _cfg = cfg;
        IsCliFound = _cli.Found;
        CliPath = _cli.CliPath;
        LoadProjects();
    }

    private void LoadProjects()
    {
        foreach (var c in _cfg.LoadProjects()) Projects.Add(c);
        if (Projects.Count > 0) SelectedProject = Projects[0];
    }

    partial void OnSelectedProjectChanged(ProjectConfig? value)
    {
        if (value != null) _ = RefreshAsync();
    }

    [RelayCommand]
    private async Task RefreshAsync()
    {
        if (SelectedProject == null || IsBusy) return;
        IsBusy = true;
        StatusMessage = "刷新中...";

        try
        {
            var result = await _cli.GetStatusAsync(SelectedProject.ProjectPath);
            if (result?.Data == null) { StatusMessage = "刷新失败"; return; }

            UnstagedFiles.Clear();
            StagedFiles.Clear();
            foreach (var f in result.Data.Unstaged)
                UnstagedFiles.Add(FileItem.FromCliItem(f));
            foreach (var f in result.Data.Staged)
                StagedFiles.Add(FileItem.FromCliItem(f));

            // Change logs
            var logResult = await _cli.GetLogAsync(SelectedProject.ProjectPath);
            ChangeLogs.Clear();
            if (logResult?.Data != null)
                foreach (var l in logResult.Data.OrderByDescending(l2 => l2.Id))
                    ChangeLogs.Add(l);
            if (ChangeLogs.Count > 0)
                CurrentVersion = ChangeLogs[0].Version;

            StatusMessage = $"{UnstagedFiles.Count} unstaged, {StagedFiles.Count} staged";
        }
        catch (Exception ex) { StatusMessage = $"Error: {ex.Message}"; Log.Error(ex, "Refresh"); }
        finally { IsBusy = false; }
    }

    [RelayCommand]
    private async Task AddAllAsync()
    {
        if (SelectedProject == null) return;
        var r = await _cli.AddAllAsync(SelectedProject.ProjectPath);
        StatusMessage = r?.IsSuccess == true ? "已添加所有变更" : $"Add failed: {r?.ErrorMsg}";
        await RefreshAsync();
    }

    [RelayCommand]
    private async Task ResetAllAsync()
    {
        if (SelectedProject == null) return;
        var r = await _cli.ResetAllAsync(SelectedProject.ProjectPath);
        StatusMessage = r?.IsSuccess == true ? "暂存区已清空" : $"Reset failed: {r?.ErrorMsg}";
        await RefreshAsync();
    }

    [RelayCommand]
    private async Task AddSelectedAsync()
    {
        if (SelectedProject == null) return;
        var files = UnstagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (files.Count == 0) { StatusMessage = "未选中文件"; return; }
        var r = await _cli.AddFilesAsync(SelectedProject.ProjectPath, files);
        StatusMessage = r?.IsSuccess == true ? $"{files.Count} files staged" : $"Add failed: {r?.ErrorMsg}";
        await RefreshAsync();
    }

    [RelayCommand]
    private async Task ResetSelectedAsync()
    {
        if (SelectedProject == null) return;
        var files = StagedFiles.Where(f => f.IsSelected).Select(f => f.RelativePath).ToList();
        if (files.Count == 0) { StatusMessage = "未选中文件"; return; }
        // reset by re-adding the files from unstaged... actually publish-cli doesn't have reset per-file easily.
        // We'll reset all and re-add the unstaged files that should stay.
        var keepPaths = UnstagedFiles.Where(f => !f.IsSelected).Select(f => f.RelativePath).ToList();
        await _cli.ResetAllAsync(SelectedProject.ProjectPath);
        if (keepPaths.Count > 0)
            await _cli.AddFilesAsync(SelectedProject.ProjectPath, keepPaths);
        await RefreshAsync();
    }

    [RelayCommand]
    private async Task PublishAsync()
    {
        if (SelectedProject == null || IsBusy) return;
        if (string.IsNullOrWhiteSpace(NewVersion)) { StatusMessage = "请输入版本号"; return; }
        if (string.IsNullOrWhiteSpace(CommitMessage)) { StatusMessage = "请输入变更说明"; return; }
        if (StagedFiles.Count == 0) { StatusMessage = "暂存区为空，请先 add 文件"; return; }

        IsBusy = true;
        StatusMessage = "发布中...";

        try
        {
            var r = await _cli.PushAsync(SelectedProject.ProjectPath, NewVersion, CommitMessage);
            if (r?.IsSuccess == true)
            {
                StatusMessage = $"发布成功: {NewVersion}";
                NewVersion = string.Empty;
                CommitMessage = string.Empty;
                await RefreshAsync();
            }
            else StatusMessage = $"发布失败: {r?.ErrorMsg}";
        }
        catch (Exception ex) { StatusMessage = $"Error: {ex.Message}"; Log.Error(ex, "Publish"); }
        finally { IsBusy = false; }
    }

    [RelayCommand]
    private async Task AddProjectAsync()
    {
        var dlg = new Views.Dialogs.AddProjectDialog();
        var cfg = await dlg.ShowDialog<ProjectConfig?>(
            (Avalonia.Application.Current?.ApplicationLifetime as Avalonia.Controls.ApplicationLifetimes.IClassicDesktopStyleApplicationLifetime)?.MainWindow!);
        if (cfg != null)
        {
            _cfg.AddProject(cfg);
            Projects.Add(cfg);
            SelectedProject = cfg;
        }
    }

    [RelayCommand]
    private async Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return;
        _cfg.RemoveProject(SelectedProject.ProjectName);
        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
    }
}