using System.IO;
using AlyPublish.Helpers;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Serilog;

namespace AlyPublish.ViewModels;

/// <summary>Adds an already initialized folder; everything is derived from .updator/shared.json.</summary>
public sealed partial class AddLocalProjectDialogViewModel : ViewModelBase
{
    private readonly ConfigService _config;
    private HashSet<string> _existingProjectPaths = new(StringComparer.OrdinalIgnoreCase);

    public AddLocalProjectDialogViewModel(
        ConfigService config,
        IDialogService dialogs,
        INotificationService notifications)
        : base(dialogs, notifications)
    {
        _config = config;
    }

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ConfirmCommand))]
    private string _projectPath = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ConfirmCommand))]
    private string _displayName = string.Empty;

    [ObservableProperty]
    private string _initStatusMessage = string.Empty;

    [ObservableProperty]
    private bool _isReady;

    [ObservableProperty]
    private bool _hasPreview;

    [ObservableProperty]
    private string _previewServerUrl = string.Empty;

    [ObservableProperty]
    private string _previewProjectName = string.Empty;

    [ObservableProperty]
    private int _previewIgnoreCount;

    public void Initialize()
    {
        _existingProjectPaths = new HashSet<string>(
            _config.LoadProjects().Select(project => NormalizeForComparison(project.ProjectPath)),
            StringComparer.OrdinalIgnoreCase);

        InitStatusMessage = "请选择已初始化的项目文件夹";
    }

    /// <summary>Called by the view when a folder is dropped onto the dialog.</summary>
    public void SetDroppedFolder(string folderPath) => ProjectPath = folderPath;

    partial void OnProjectPathChanged(string value)
    {
        HasPreview = false;
        IsReady = false;
        PreviewServerUrl = string.Empty;
        PreviewProjectName = string.Empty;
        PreviewIgnoreCount = 0;

        var path = value.Trim();
        if (path.Length == 0)
        {
            InitStatusMessage = "请选择已初始化的项目文件夹";
            return;
        }

        if (!Directory.Exists(path))
        {
            InitStatusMessage = "目录不存在";
            return;
        }

        if (_existingProjectPaths.Contains(NormalizeForComparison(path)))
        {
            InitStatusMessage = "该项目已在左侧列表中";
            return;
        }

        if (!ProjectInputHelper.IsInitialized(path))
        {
            InitStatusMessage = "未找到 .updator/shared.json，请先初始化该项目";
            return;
        }

        SharedConfig? shared;
        try
        {
            shared = ProjectConfigService.Load(path);
            PreviewServerUrl = shared.ServerUrl;
            PreviewProjectName = shared.ProjectName;
            PreviewIgnoreCount = shared.IgnoreFolders.Count + shared.IgnoreFiles.Count;
            HasPreview = true;
            IsReady = true;
            InitStatusMessage = "已检测到项目配置";
        }
        catch (ProjectConfigException ex)
        {
            InitStatusMessage = ex.Message;
            Log.Warning(ex, "读取项目配置失败: {Path}", path);
            return;
        }

        if (string.IsNullOrWhiteSpace(DisplayName))
        {
            DisplayName = ProjectInputHelper.SuggestDisplayName(path, shared);
        }
    }

    [RelayCommand(CanExecute = nameof(CanConfirm))]
    private void Confirm()
    {
        var path = ProjectPath.Trim();
        var name = string.IsNullOrWhiteSpace(DisplayName)
            ? Path.GetFileName(Path.TrimEndingDirectorySeparator(path))
            : DisplayName.Trim();

        Close(new ProjectConfig
        {
            DisplayName = name,
            ProjectPath = path
        });
    }

    [RelayCommand]
    private void Cancel() => Close(null);

    protected override void OnBusyChanged() => ConfirmCommand.NotifyCanExecuteChanged();

    private bool CanConfirm() => !IsBusy && IsReady && !string.IsNullOrWhiteSpace(ProjectPath);

    private static string NormalizeForComparison(string path)
    {
        try
        {
            return Path.TrimEndingDirectorySeparator(Path.GetFullPath(path));
        }
        catch (Exception ex) when (ex is ArgumentException or NotSupportedException or PathTooLongException)
        {
            return path.Trim();
        }
    }
}
