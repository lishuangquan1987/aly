using System;
using System.Collections.Generic;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Media;
using Avalonia.Platform.Storage;
using PublishGui.Models.Cli;
using PublishGui.Models.Local;
using PublishGui.Services;
using Serilog;

namespace PublishGui.Views.Dialogs;

public partial class AddProjectDialog : Window
{
    private readonly CliService _cli;

    private enum StatusKind { Info, Error, Success }

    public AddProjectDialog(CliService cli)
    {
        _cli = cli;
        InitializeComponent();
        ConfirmBtn.Click += OnConfirm;
        CancelBtn.Click += (_, _) => Close(null);
        BrowseBtn.Click += OnBrowse;
        FetchProjectsBtn.Click += OnFetchProjects;
        CreateOnServerBtn.Click += OnCreateOnServer;
        ServerProjectList.SelectionChanged += OnServerProjectSelected;
    }

    private async void OnBrowse(object? sender, RoutedEventArgs e)
    {
        var folders = await StorageProvider.OpenFolderPickerAsync(
            new FolderPickerOpenOptions { Title = "选择项目文件夹" });
        if (folders.Count > 0)
            ProjectPathBox.Text = folders[0].Path.LocalPath;
    }

    private async void OnFetchProjects(object? sender, RoutedEventArgs e)
    {
        await FetchProjects();
    }

    private async Task FetchProjects()
    {
        var serverUrl = ServerUrlBox.Text?.Trim();
        if (string.IsNullOrEmpty(serverUrl))
        {
            ShowFetchStatus("请先填写服务端地址", false, StatusKind.Error);
            return;
        }

        FetchProjectsBtn.IsEnabled = false;
        ShowFetchStatus("正在获取项目列表...", false, StatusKind.Info);
        Log.Information("获取服务端项目列表: {ServerUrl}", serverUrl);

        try
        {
            var result = await _cli.ProjectListAsync(serverUrl);
            Log.Information("获取项目结果: IsSuccess={IsSuccess}, ErrorMsg={ErrorMsg}, Data={Data}",
                result?.IsSuccess, result?.ErrorMsg,
                result?.Data != null ? $"{result.Data.Count} projects" : "null");

            if (result == null)
            {
                ShowFetchStatus("未收到服务端响应，请检查地址是否正确", true, StatusKind.Error);
                return;
            }

            if (!result.IsSuccess)
            {
                ShowFetchStatus($"获取失败: {result.ErrorMsg}", true, StatusKind.Error);
                return;
            }

            var projects = result.Data ?? new List<ProjectInfo>();

            if (projects.Count == 0)
            {
                ProjectListPanel.IsVisible = false;
                ShowFetchStatus("服务端暂无项目，可点击下方按钮创建", true, StatusKind.Info);
                return;
            }

            ServerProjectList.ItemsSource = projects;
            ProjectListPanel.IsVisible = true;
            FetchStatusPanel.IsVisible = false;
            Log.Information("获取到 {Count} 个项目", projects.Count);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "获取服务端项目异常: {ServerUrl}", serverUrl);
            ShowFetchStatus($"获取异常: {ex.Message}", true, StatusKind.Error);
        }
        finally
        {
            FetchProjectsBtn.IsEnabled = true;
        }
    }

    private void OnServerProjectSelected(object? sender, SelectionChangedEventArgs e)
    {
        if (ServerProjectList.SelectedItem is ProjectInfo project)
        {
            ProjectNameBox.Text = project.Name;
            ProjectIdBox.Text = project.Id.ToString();
            Log.Information("选择了服务端项目: Name={Name}, Id={Id}", project.Name, project.Id);
        }
    }

    private async void OnCreateOnServer(object? sender, RoutedEventArgs e)
    {
        var serverUrl = ServerUrlBox.Text?.Trim();
        if (string.IsNullOrEmpty(serverUrl))
        {
            ShowFetchStatus("请先填写服务端地址", false, StatusKind.Error);
            return;
        }

        Log.Information("打开创建项目对话框: ServerUrl={ServerUrl}", serverUrl);
        var dlg = new CreateProjectDialog(_cli, serverUrl);
        var project = await dlg.ShowDialog<ProjectInfo?>(this);

        if (project != null)
        {
            ProjectNameBox.Text = project.Name;
            ProjectIdBox.Text = project.Id.ToString();
            ShowFetchStatus($"项目 \"{project.Name}\" 创建成功 (ID: {project.Id})", false, StatusKind.Success);
            Log.Information("创建项目完成，自动填充: Name={Name}, Id={Id}", project.Name, project.Id);

            // 刷新项目列表
            await FetchProjects();
        }
    }

    private static IBrush GetResourceBrush(string key, IBrush fallback)
    {
        if (Avalonia.Application.Current?.TryFindResource(key, out var value) == true && value is IBrush brush)
            return brush;
        return fallback;
    }

    private void ShowFetchStatus(string message, bool showCreateButton, StatusKind kind = StatusKind.Info)
    {
        FetchStatusText.Text = message;
        FetchStatusText.Foreground = kind switch
        {
            StatusKind.Error => GetResourceBrush("SemiColorDanger", Brushes.Red),
            StatusKind.Success => GetResourceBrush("SemiColorSuccess", Brushes.Green),
            _ => GetResourceBrush("SemiColorText2", Brushes.Gray)
        };
        CreateOnServerBtn.IsVisible = showCreateButton;
        FetchStatusPanel.IsVisible = true;
    }

    private void OnConfirm(object? sender, RoutedEventArgs e)
    {
        if (string.IsNullOrWhiteSpace(ServerUrlBox.Text) ||
            string.IsNullOrWhiteSpace(ProjectNameBox.Text) ||
            string.IsNullOrWhiteSpace(ProjectPathBox.Text))
        {
            var msg = new Window
            {
                Title = "验证",
                Width = 300, Height = 100,
                WindowStartupLocation = WindowStartupLocation.CenterOwner,
                Content = new TextBlock
                {
                    Text = "请填写所有必填项。",
                    Margin = new Avalonia.Thickness(16),
                    VerticalAlignment = Avalonia.Layout.VerticalAlignment.Center,
                    HorizontalAlignment = Avalonia.Layout.HorizontalAlignment.Center
                }
            };
            msg.ShowDialog(this);
            return;
        }

        var cfg = new ProjectConfig
        {
            ServerUrl = ServerUrlBox.Text.Trim(),
            ProjectName = ProjectNameBox.Text.Trim(),
            ProjectPath = ProjectPathBox.Text.Trim(),
            ProjectId = int.TryParse(ProjectIdBox.Text, out var id) ? id : 0
        };

        Log.Information("添加项目: Name={Name}, Server={Server}, Path={Path}, Id={Id}",
            cfg.ProjectName, cfg.ServerUrl, cfg.ProjectPath, cfg.ProjectId);

        Close(cfg);
    }
}
