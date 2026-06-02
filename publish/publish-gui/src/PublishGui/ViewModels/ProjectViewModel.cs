using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishGui.Constants;
using PublishGui.Models.Cli;
using PublishGui.Models.Local;
using PublishGui.Services;
using Serilog;
using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;

namespace PublishGui.ViewModels;

/// <summary>
/// 项目页面 ViewModel
/// 负责管理单个项目的文件状态、暂存区、发布和版本日志
/// </summary>
public partial class ProjectViewModel : ObservableObject
{
    private readonly CliService _cliService;
    private readonly ConfigService _configService;

    /// <summary>项目配置</summary>
    [ObservableProperty]
    private ProjectConfig _config;

    /// <summary>已暂存文件列表</summary>
    [ObservableProperty]
    private ObservableCollection<FileItem> _stagedFiles = new();

    /// <summary>未暂存文件列表</summary>
    [ObservableProperty]
    private ObservableCollection<FileItem> _unstagedFiles = new();

    /// <summary>状态消息</summary>
    [ObservableProperty]
    private string _statusMessage = string.Empty;

    /// <summary>是否正在执行操作</summary>
    [ObservableProperty]
    private bool _isBusy;

    /// <summary>当前版本号</summary>
    [ObservableProperty]
    private string _currentVersion = string.Empty;

    /// <summary>新版本号（用户输入）</summary>
    [ObservableProperty]
    private string _newVersion = string.Empty;

    /// <summary>变更说明（用户输入）</summary>
    [ObservableProperty]
    private string _commitMessage = string.Empty;

    /// <summary>版本变更日志列表</summary>
    [ObservableProperty]
    private ObservableCollection<ChangeLog> _changeLogs = new();

    public ProjectViewModel(ProjectConfig config, CliService cliService, ConfigService configService)
    {
        _config = config;
        _cliService = cliService;
        _configService = configService;
    }

    /// <summary>
    /// 刷新文件状态
    /// </summary>
    [RelayCommand]
    private async Task RefreshStatusAsync()
    {
        if (IsBusy) return;
        
        IsBusy = true;
        StatusMessage = UiTexts.RefreshingStatus;

        try
        {
            var result = await _cliService.GetStatusAsync(Config.ProjectPath);
            
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"{UiTexts.RefreshFailed}: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            UpdateFileLists(result.Data);
            StatusMessage = $"已刷新: {StagedFiles.Count} 个暂存文件, {UnstagedFiles.Count} 个未暂存文件";
        }
        catch (Exception ex)
        {
            StatusMessage = $"{UiTexts.RefreshFailed}: {ex.Message}";
            Log.Error(ex, "刷新状态失败");
        }
        finally
        {
            IsBusy = false;
        }
    }

    /// <summary>
    /// 添加所有变更到暂存区
    /// </summary>
    [RelayCommand]
    private async Task AddAllAsync()
    {
        if (IsBusy) return;
        
        IsBusy = true;
        StatusMessage = UiTexts.AddingAllChanges;

        try
        {
            var result = await _cliService.AddAllAsync(Config.ProjectPath);
            
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"{UiTexts.AddFailed}: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            StatusMessage = UiTexts.AddedAllSuccess;
            await RefreshStatusAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"{UiTexts.AddFailed}: {ex.Message}";
            Log.Error(ex, "添加变更失败");
        }
        finally
        {
            IsBusy = false;
        }
    }

    /// <summary>
    /// 清空暂存区
    /// </summary>
    [RelayCommand]
    private async Task ResetAllAsync()
    {
        if (IsBusy) return;
        
        IsBusy = true;
        StatusMessage = UiTexts.ClearingStaging;

        try
        {
            var result = await _cliService.ResetAllAsync(Config.ProjectPath);
            
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"{UiTexts.ClearFailed}: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            StatusMessage = UiTexts.StagingCleared;
            await RefreshStatusAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"{UiTexts.ClearFailed}: {ex.Message}";
            Log.Error(ex, "清空暂存区失败");
        }
        finally
        {
            IsBusy = false;
        }
    }

    /// <summary>
    /// 发布新版本
    /// </summary>
    [RelayCommand]
    private async Task PublishAsync()
    {
        if (IsBusy) return;
        
        // 验证输入
        if (string.IsNullOrWhiteSpace(NewVersion))
        {
            StatusMessage = UiTexts.EnterVersion;
            return;
        }
        
        if (string.IsNullOrWhiteSpace(CommitMessage))
        {
            StatusMessage = UiTexts.EnterCommitMessage;
            return;
        }

        IsBusy = true;
        StatusMessage = UiTexts.Publishing;

        try
        {
            // 1. 先添加所有变更
            var addResult = await _cliService.AddAllAsync(Config.ProjectPath);
            if (addResult == null || !addResult.IsSuccess)
            {
                StatusMessage = $"{UiTexts.AddFailed}: {addResult?.ErrorMsg ?? "未知错误"}";
                return;
            }

            // 2. 发布
            var result = await _cliService.PublishAsync(Config.ProjectPath, NewVersion, CommitMessage);
            if (result == null || !result.IsSuccess)
            {
                StatusMessage = $"{UiTexts.PublishFailed}: {result?.ErrorMsg ?? "未知错误"}";
                return;
            }

            // 3. 成功后清理输入并刷新
            StatusMessage = $"{UiTexts.PublishSuccess}: {NewVersion}";
            NewVersion = string.Empty;
            CommitMessage = string.Empty;
            
            await RefreshStatusAsync();
            await RefreshLogAsync();
        }
        catch (Exception ex)
        {
            StatusMessage = $"{UiTexts.PublishFailed}: {ex.Message}";
            Log.Error(ex, "发布失败");
        }
        finally
        {
            IsBusy = false;
        }
    }

    /// <summary>
    /// 刷新版本日志
    /// </summary>
    [RelayCommand]
    private async Task RefreshLogAsync()
    {
        try
        {
            var result = await _cliService.GetLogAsync(Config.ProjectPath);
            
            if (result?.Data != null)
            {
                ChangeLogs.Clear();
                
                // 按版本号倒序排列
                var sortedLogs = result.Data.OrderByDescending(l => l.Id);
                foreach (var log in sortedLogs)
                {
                    ChangeLogs.Add(log);
                }

                // 更新当前版本号
                if (result.Data.Count > 0)
                {
                    CurrentVersion = result.Data.OrderByDescending(l => l.Id).First().Version;
                }
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "刷新版本日志失败");
        }
    }

    /// <summary>
    /// 更新文件列表
    /// </summary>
    private void UpdateFileLists(StatusData? data)
    {
        StagedFiles.Clear();
        UnstagedFiles.Clear();

        if (data?.Staged != null)
        {
            foreach (var item in data.Staged)
            {
                StagedFiles.Add(CreateFileItem(item));
            }
        }

        if (data?.Unstaged != null)
        {
            foreach (var item in data.Unstaged)
            {
                UnstagedFiles.Add(CreateFileItem(item));
            }
        }
    }

    /// <summary>
    /// 创建文件列表项
    /// </summary>
    private static FileItem CreateFileItem(FileStatusItem item) => new()
    {
        RelativePath = item.RelativePath,
        Status = item.Status,
        LocalSize = item.LocalSize,
        ServerSize = item.ServerSize
    };
}
