using System;
using System.Collections.Generic;
using System.Linq;
using Avalonia.Controls;
using Avalonia.Interactivity;
using ZapPublish.Models.Cli;
using ZapPublish.Services;
using ZapPublish.Helpers;
using Serilog;

namespace ZapPublish.Views.Dialogs;

public partial class CreateProjectDialog : Window
{
    private readonly CliService _cli;
    private readonly string _serverUrl;

    public CreateProjectDialog(CliService cli, string serverUrl)
    {
        _cli = cli;
        _serverUrl = serverUrl;
        InitializeComponent();
        CancelBtn.Click += (_, _) => Close(null);
        CreateBtn.Click += OnCreate;
    }

    private async void OnCreate(object? sender, RoutedEventArgs e)
    {
        var name = NameBox.Text?.Trim();
        var title = TitleBox.Text?.Trim();

        if (string.IsNullOrEmpty(name))
        {
            ShowStatus("请填写项目名称", StatusKind.Error);
            return;
        }
        if (string.IsNullOrEmpty(title))
        {
            ShowStatus("请填写项目标题", StatusKind.Error);
            return;
        }

        var forceUpdate = ForceUpdateBox.IsChecked == true;
        var ignoreFolders = ParseCsv(IgnoreFoldersBox.Text);
        var ignoreFiles = ParseCsv(IgnoreFilesBox.Text);

        CreateBtn.IsEnabled = false;
        ShowStatus("正在创建项目...", StatusKind.Info);
        Log.Information("创建项目: Server={Server}, Name={Name}, Title={Title}, Force={Force}, IgnoreFolders={Folders}, IgnoreFiles={Files}",
            _serverUrl, name, title, forceUpdate,
            string.Join(",", ignoreFolders), string.Join(",", ignoreFiles));

        try
        {
            var result = await _cli.ProjectCreateAsync(
                _serverUrl, name, title, forceUpdate, ignoreFolders, ignoreFiles);

            Log.Information("创建结果: IsSuccess={Ok}, ErrorMsg={Err}", result?.IsSuccess, result?.ErrorMsg);

            if (result?.IsSuccess == true && result.Data != null)
            {
                Log.Information("项目创建成功: Id={Id}", result.Data.Id);
                Close(result.Data);
            }
            else
            {
                ShowStatus($"创建失败: {result?.ErrorMsg}", StatusKind.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "创建项目异常");
            ShowStatus($"创建异常: {ex.Message}", StatusKind.Error);
        }
        finally
        {
            CreateBtn.IsEnabled = true;
        }
    }

    private static List<string> ParseCsv(string? text)
    {
        if (string.IsNullOrWhiteSpace(text)) return new List<string>();
        return text.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Where(s => !string.IsNullOrEmpty(s))
            .ToList();
    }

    private void ShowStatus(string message, StatusKind kind)
    {
        DialogHelper.SetStatus(StatusText, message, kind, StatusPanel);
    }
}
