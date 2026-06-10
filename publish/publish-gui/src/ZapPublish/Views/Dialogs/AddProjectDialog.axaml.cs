using System;
using System.Collections.Generic;
using System.IO;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using Newtonsoft.Json;
using ZapPublish.Models.Cli;
using ZapPublish.Models.Local;
using ZapPublish.Services;
using ZapPublish.Helpers;
using Serilog;

namespace ZapPublish.Views.Dialogs;

internal class SharedConfig
{
    public string server_url { get; set; } = "";
    public string project_name { get; set; } = "";
}

public partial class AddProjectDialog : Window
{
    private readonly CliService _cli;
    private string _serverProjectName = string.Empty; // 服务端项目名（选中的真实 project name）
    private bool _isInit; // 项目的 .updator/shared.json 是否已存在

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
        ProjectPathBox.TextChanged += OnPathChanged;
    }

    private void OnPathChanged(object? sender, TextChangedEventArgs e)
    {
        var path = ProjectPathBox.Text?.Trim();
        if (string.IsNullOrWhiteSpace(path) || !Directory.Exists(path))
            return;

        var sharedPath = Path.Combine(path, ".updator", "shared.json");
        if (File.Exists(sharedPath))
        {
            _isInit = true;
            try
            {
                var json = File.ReadAllText(sharedPath);
                var shared = JsonConvert.DeserializeObject<SharedConfig>(json);
                if (shared != null)
                {
                    _serverProjectName = shared.project_name ?? "";
                    // 如果用户还没填显示名称，自动用 project_name 填充
                    if (string.IsNullOrWhiteSpace(ProjectNameBox.Text))
                        ProjectNameBox.Text = shared.project_name ?? "";
                    if (!string.IsNullOrWhiteSpace(shared.server_url))
                        ServerUrlBox.Text = shared.server_url;
                }
            }
            catch { /* ignore parse errors */ }
            ShowInitStatus("✅ 已检测到 .updator/ 配置（项目已初始化）", StatusKind.Success);
        }
        else
        {
            _isInit = false;
            ShowInitStatus("⚠ 未检测到 .updator/ 配置，需填写服务端信息进行初始化", StatusKind.Warning);
        }
    }

    private async void OnBrowse(object? sender, RoutedEventArgs e)
    {
        var folders = await StorageProvider.OpenFolderPickerAsync(
            new FolderPickerOpenOptions { Title = "选择项目文件夹" });
        if (folders.Count > 0)
        {
            ProjectPathBox.Text = folders[0].Path.LocalPath;
            // TextChanged 已自动触发 OnPathChanged
        }
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
            _serverProjectName = project.Name;
            ProjectNameBox.Text = project.Name;
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
            _serverProjectName = project.Name;
            ProjectNameBox.Text = project.Name;
            ShowFetchStatus($"项目 \"{project.Name}\" 创建成功", false, StatusKind.Success);
            Log.Information("创建项目完成，自动填充: Name={Name}, Id={Id}", project.Name, project.Id);

            await FetchProjects();
        }
    }

    private void ShowFetchStatus(string message, bool showCreateButton, StatusKind kind = StatusKind.Info)
    {
        DialogHelper.SetStatus(FetchStatusText, message, kind, FetchStatusPanel);
        CreateOnServerBtn.IsVisible = showCreateButton;
    }

    private async void OnConfirm(object? sender, RoutedEventArgs e)
    {
        var displayName = ProjectNameBox.Text.Trim();
        var projectPath = ProjectPathBox.Text.Trim();

        if (string.IsNullOrWhiteSpace(displayName) ||
            string.IsNullOrWhiteSpace(projectPath))
        {
            await ShowValidationError("请填写显示名称和本地路径。");
            return;
        }

        if (!Directory.Exists(projectPath))
        {
            await ShowValidationError("指定的路径不存在。");
            return;
        }

        // 情况1：项目未初始化，需运行 config init
        if (!_isInit)
        {
            var serverUrl = ServerUrlBox.Text?.Trim();
            if (string.IsNullOrWhiteSpace(serverUrl) || string.IsNullOrWhiteSpace(_serverProjectName))
            {
                await ShowValidationError("项目未初始化。请填写服务端地址并选择/创建服务端项目。");
                return;
            }

            Log.Information("项目未初始化，执行 config init: Server={Server}, Project={Name}, Path={Path}",
                serverUrl, _serverProjectName, projectPath);
            var initResult = await _cli.ConfigInitAsync(projectPath, serverUrl, _serverProjectName);
            if (initResult?.IsSuccess != true)
            {
                await ShowValidationError($"初始化 .updator/ 失败: {initResult?.ErrorMsg}");
                return;
            }
        }

        var cfg = new ProjectConfig
        {
            DisplayName = displayName,
            ProjectPath = projectPath
        };

        Log.Information("添加项目: DisplayName={Name}, Path={Path}, WasInit={Init}",
            cfg.DisplayName, cfg.ProjectPath, _isInit);

        Close(cfg);
    }

    private void ShowInitStatus(string message, StatusKind kind)
    {
        DialogHelper.SetStatus(FetchStatusText, message, kind, FetchStatusPanel);
        FetchStatusPanel.IsVisible = true;
    }

    private async Task ShowValidationError(string message)
    {
        var msg = new Window
        {
            Title = "验证",
            Width = 360, Height = 120,
            WindowStartupLocation = WindowStartupLocation.CenterOwner,
            Content = new TextBlock
            {
                Text = message,
                Margin = new Avalonia.Thickness(16),
                TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                VerticalAlignment = Avalonia.Layout.VerticalAlignment.Center,
                HorizontalAlignment = Avalonia.Layout.HorizontalAlignment.Center
            }
        };
        await msg.ShowDialog(this);
    }
}
