using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Newtonsoft.Json;
using ZapPublish.Models.Local;
using Serilog;

namespace ZapPublish.ViewModels;

public partial class AddLocalProjectDialogViewModel : ObservableObject
{
    /// <summary>由调用方注入，用于打开系统文件夹选择器</summary>
    internal Func<Task<string?>>? BrowseFolderAsync { get; set; }

    [ObservableProperty] private string _displayName = string.Empty;
    [ObservableProperty] private string _projectPath = string.Empty;
    [ObservableProperty] private string _initStatusMessage = string.Empty;
    [ObservableProperty] private bool _showInitStatus;

    /// <summary>View 订阅此事件以关闭对话框并返回结果</summary>
    public event Action<ProjectConfig?>? RequestClose;

    public IAsyncRelayCommand BrowseCommand { get; }
    public IAsyncRelayCommand ConfirmCommand { get; }
    public IRelayCommand CancelCommand { get; }

    private CancellationTokenSource? _pathChangedCts;

    public AddLocalProjectDialogViewModel()
    {
        BrowseCommand = new AsyncRelayCommand(BrowseAsync);
        ConfirmCommand = new AsyncRelayCommand(ConfirmAsync);
        CancelCommand = new RelayCommand(() => RequestClose?.Invoke(null));
    }

    async partial void OnProjectPathChanged(string value)
    {
        // 取消上一次未完成的异步操作，防止竞态
        _pathChangedCts?.Cancel();
        _pathChangedCts = new CancellationTokenSource();
        var ct = _pathChangedCts.Token;

        if (string.IsNullOrWhiteSpace(value) || !Directory.Exists(value))
        {
            ShowInitStatus = false;
            return;
        }

        var sharedPath = Path.Combine(value, ".updator", "shared.json");
        if (File.Exists(sharedPath))
        {
            try
            {
                var json = await ReadFileWithRetryAsync(sharedPath, ct: ct);
                ct.ThrowIfCancellationRequested();
                var shared = JsonConvert.DeserializeObject<SharedConfig>(json);
                if (shared != null)
                {
                    if (string.IsNullOrWhiteSpace(DisplayName))
                        DisplayName = shared.ProjectName ?? "";
                }
            }
            catch (OperationCanceledException)
            {
                return; // 被新路径替换，静默退出
            }
            catch (Exception ex)
            {
                Log.Warning(ex, "读取 .updator/shared.json 失败: {Path}", sharedPath);
            }
            InitStatusMessage = "✅ 已检测到 .updator/ 配置";
            ShowInitStatus = true;
        }
        else
        {
            InitStatusMessage = "⚠ 未检测到 .updator/ 配置，无法添加";
            ShowInitStatus = true;
        }
    }

    private async Task BrowseAsync()
    {
        if (BrowseFolderAsync == null)
        {
            Log.Warning("BrowseFolderAsync delegate 未设置");
            return;
        }
        var path = await BrowseFolderAsync();
        if (!string.IsNullOrWhiteSpace(path))
            ProjectPath = path;
    }

    private async Task ConfirmAsync()
    {
        var projectPath = ProjectPath?.Trim();
        var displayName = DisplayName?.Trim();

        if (string.IsNullOrWhiteSpace(projectPath) || !Directory.Exists(projectPath))
        {
            InitStatusMessage = "请选择有效的项目文件夹";
            ShowInitStatus = true;
            return;
        }

        var sharedPath = Path.Combine(projectPath, ".updator", "shared.json");
        if (!File.Exists(sharedPath))
        {
            InitStatusMessage = "❌ 该文件夹未初始化，无法添加（缺少 .updator/shared.json）";
            ShowInitStatus = true;
            return;
        }

        var cfg = new ProjectConfig
        {
            DisplayName = string.IsNullOrWhiteSpace(displayName) ? Path.GetFileName(projectPath) : displayName!,
            ProjectPath = projectPath!
        };

        Log.Information("添加本地项目: DisplayName={Name}, Path={Path}", cfg.DisplayName, cfg.ProjectPath);
        RequestClose?.Invoke(cfg);
    }

    private static async Task<string> ReadFileWithRetryAsync(string path, int maxRetries = 3, int delayMs = 50, CancellationToken ct = default)
    {
        for (int i = 0; i < maxRetries; i++)
        {
            ct.ThrowIfCancellationRequested();
            try
            {
                return await File.ReadAllTextAsync(path, ct);
            }
            catch (IOException) when (i < maxRetries - 1)
            {
                await Task.Delay(delayMs, ct);
            }
        }
        ct.ThrowIfCancellationRequested();
        return await File.ReadAllTextAsync(path, ct);
    }
}
