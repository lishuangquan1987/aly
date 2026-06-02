using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishGui.Models.Cli;
using PublishGui.Models.Local;
using PublishGui.Services;
using Serilog;
using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;

namespace PublishGui.ViewModels;

public partial class ProjectViewModel : ObservableObject
{
    private readonly CliService _cliService;
    private readonly ConfigService _configService;

    [ObservableProperty]
    private ProjectConfig _config;

    [ObservableProperty]
    private ObservableCollection<FileItem> _stagedFiles = new();

    [ObservableProperty]
    private ObservableCollection<FileItem> _unstagedFiles = new();

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    [ObservableProperty]
    private bool _isBusy;

    [ObservableProperty]
    private string _currentVersion = string.Empty;

    [ObservableProperty]
    private string _newVersion = string.Empty;

    [ObservableProperty]
    private string _commitMessage = string.Empty;

    [ObservableProperty]
    private ObservableCollection<ChangeLog> _changeLogs = new();

    public ProjectViewModel(ProjectConfig config, CliService cliService, ConfigService configService)
    {
        _config = config;
        _cliService = cliService;
        _configService = configService;
    }

    [RelayCommand]
    private async Task RefreshStatusAsync()
    {
        if (IsBusy) return;
        IsBusy = true;
        StatusMessage = "正在刷新状态...";

        try
        {
            var result = await _cliService.GetStatusAsync(Config.ProjectPath);
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"刷新失败: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            StagedFiles.Clear();
            UnstagedFiles.Clear();

            if (result.Data?.Staged != null)
            {
                foreach (var item in result.Data.Staged)
                {
                    StagedFiles.Add(new FileItem
                    {
                        RelativePath = item.RelativePath,
                        Status = item.Status,
                        LocalSize = item.LocalSize,
                        ServerSize = item.ServerSize
                    });
                }
            }

            if (result.Data?.Unstaged != null)
            {
                foreach (var item in result.Data.Unstaged)
                {
                    UnstagedFiles.Add(new FileItem
                    {
                        RelativePath = item.RelativePath,
                        Status = item.Status,
                        LocalSize = item.LocalSize,
                        ServerSize = item.ServerSize
                    });
                }
            }

            StatusMessage = $"已刷新: {StagedFiles.Count} 个暂存文件, {UnstagedFiles.Count} 个未暂存文件";
        }
        catch (Exception ex)
        {
            StatusMessage = $"刷新失败: {ex.Message}";
            Log.Error(ex, "Failed to refresh status");
        }
        finally
        {
            IsBusy = false;
        }
    }

    [RelayCommand]
    private async Task AddAllAsync()
    {
        if (IsBusy) return;
        IsBusy = true;
        StatusMessage = "正在添加所有变更...";

        try
        {
            var result = await _cliService.AddAllAsync(Config.ProjectPath);
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"添加失败: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            StatusMessage = "已添加所有变更到暂存区";
            await RefreshStatusAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"添加失败: {ex.Message}";
            Log.Error(ex, "Failed to add all");
        }
        finally
        {
            IsBusy = false;
        }
    }

    [RelayCommand]
    private async Task ResetAllAsync()
    {
        if (IsBusy) return;
        IsBusy = true;
        StatusMessage = "正在清空暂存区...";

        try
        {
            var result = await _cliService.ResetAllAsync(Config.ProjectPath);
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"清空失败: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            StatusMessage = "暂存区已清空";
            await RefreshStatusAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"清空失败: {ex.Message}";
            Log.Error(ex, "Failed to reset all");
        }
        finally
        {
            IsBusy = false;
        }
    }

    [RelayCommand]
    private async Task PublishAsync()
    {
        if (IsBusy) return;
        if (string.IsNullOrWhiteSpace(NewVersion))
        {
            StatusMessage = "请输入版本号";
            return;
        }
        if (string.IsNullOrWhiteSpace(CommitMessage))
        {
            StatusMessage = "请输入变更说明";
            return;
        }

        IsBusy = true;
        StatusMessage = "正在发布...";

        try
        {
            // First add all changes
            var addResult = await _cliService.AddAllAsync(Config.ProjectPath);
            if (addResult == null || !addResult.IsSuccess)
            {
                StatusMessage = $"添加失败: {addResult?.ErrorMsg ?? "未知错误"}";
                return;
            }

            // Then publish
            var result = await _cliService.PublishAsync(Config.ProjectPath, NewVersion, CommitMessage);
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"发布失败: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            StatusMessage = $"发布成功: {NewVersion}";
            NewVersion = string.Empty;
            CommitMessage = string.Empty;
            await RefreshStatusAsync();
            await RefreshLogAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"发布失败: {ex.Message}";
            Log.Error(ex, "Failed to publish");
        }
        finally
        {
            IsBusy = false;
        }
    }

    [RelayCommand]
    private async Task RefreshLogAsync()
    {
        try
        {
            var result = await _cliService.GetLogAsync(Config.ProjectPath);
            if (result?.Data != null)
            {
                ChangeLogs.Clear();
                foreach (var log in result.Data)
                {
                    ChangeLogs.Add(log);
                }

                if (result.Data.Count > 0)
                {
                    CurrentVersion = result.Data[0].Version;
                }
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "Failed to refresh log");
        }
    }
}
