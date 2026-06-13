using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Ursa.Controls;
using ZapPublish.Models.Cli;
using ZapPublish.Services;
using Serilog;

namespace ZapPublish.ViewModels;

public partial class CreateProjectDialogViewModel : ObservableObject
{
    private readonly CliService _cli;

    [ObservableProperty] private string _serverUrl = string.Empty;
    [ObservableProperty] private string _projectName = string.Empty;
    [ObservableProperty] private string _projectTitle = string.Empty;
    [ObservableProperty] private bool _forceUpdate;
    [ObservableProperty] private string _ignoreFolders = string.Empty;
    [ObservableProperty] private string _ignoreFiles = string.Empty;
    [ObservableProperty] private string _statusMessage = string.Empty;
    [ObservableProperty] private bool _isCreating;

    /// <summary>View 订阅此事件以关闭对话框并返回结果</summary>
    public event Action<ProjectInfo?>? RequestClose;

    public IAsyncRelayCommand CreateCommand { get; }
    public IRelayCommand CancelCommand { get; }

    public CreateProjectDialogViewModel(CliService cli)
    {
        _cli = cli;

        CreateCommand = new AsyncRelayCommand(CreateAsync, () => !IsCreating);
        CancelCommand = new RelayCommand(() => RequestClose?.Invoke(null));
    }

    partial void OnIsCreatingChanged(bool value)
    {
        CreateCommand.NotifyCanExecuteChanged();
    }

    private async Task CreateAsync()
    {
        var name = ProjectName?.Trim();
        var title = ProjectTitle?.Trim();

        if (string.IsNullOrEmpty(name))
        {
            await MessageBox.ShowAsync("请填写项目名称", "提示");
            return;
        }
        if (string.IsNullOrEmpty(title))
        {
            await MessageBox.ShowAsync("请填写项目标题", "提示");
            return;
        }

        // 防止 CLI 参数注入
        if (name.Contains('"') || title.Contains('"'))
        {
            await MessageBox.ShowAsync("项目名称和标题不能包含双引号", "提示");
            return;
        }

        var ignoreFolders = ParseCsv(IgnoreFolders);
        var ignoreFiles = ParseCsv(IgnoreFiles);

        IsCreating = true;
        StatusMessage = "正在创建项目...";
        Log.Information("创建项目: Server={Server}, Name={Name}, Title={Title}, Force={Force}, IgnoreFolders={Folders}, IgnoreFiles={Files}",
            ServerUrl, name, title, ForceUpdate,
            string.Join(",", ignoreFolders), string.Join(",", ignoreFiles));

        try
        {
            var result = await _cli.ProjectCreateAsync(
                ServerUrl, name, title, ForceUpdate, ignoreFolders, ignoreFiles);

            Log.Information("创建结果: IsSuccess={Ok}, ErrorMsg={Err}", result?.IsSuccess, result?.ErrorMsg);

            if (result?.IsSuccess == true && result.Data != null)
            {
                Log.Information("项目创建成功: Id={Id}", result.Data.Id);
                RequestClose?.Invoke(result.Data);
            }
            else
            {
                await MessageBox.ShowAsync($"创建失败: {result?.ErrorMsg}", "错误");
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "创建项目异常");
            await MessageBox.ShowAsync("创建项目时发生异常，请重试或联系管理员", "错误");
        }
        finally
        {
            IsCreating = false;
        }
    }

    private static List<string> ParseCsv(string? text)
    {
        if (string.IsNullOrWhiteSpace(text)) return new();
        return text.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Where(s => !string.IsNullOrEmpty(s))
            .ToList();
    }
}
