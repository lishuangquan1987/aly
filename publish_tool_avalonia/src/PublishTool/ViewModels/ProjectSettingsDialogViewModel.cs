using System;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models.Local;
using PublishTool.Services;
using Serilog;

namespace PublishTool.ViewModels;

public partial class ProjectSettingsDialogViewModel : ObservableObject
{
    private readonly ConfigService _configService;
    internal ProjectConfig Config => _config;
    private readonly ProjectConfig _config;

    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    private string _name = string.Empty;

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private string? _exePath;

    [ObservableProperty]
    private string _localPath = string.Empty;

    [ObservableProperty]
    private string _titleError = string.Empty;

    [ObservableProperty]
    private string _localPathError = string.Empty;

    partial void OnTitleChanged(string value) => TitleError = string.Empty;
    partial void OnLocalPathChanged(string value) => LocalPathError = string.Empty;

    public event Func<Task>? OpenIgnoreConfigRequested;

    public ProjectSettingsDialogViewModel(ConfigService configService, ProjectConfig config)
    {
        _configService = configService;
        _config = config;
        ServerUrl = config.ServerUrl;
        Name = config.Name;
        Title = config.Title;
        ExePath = config.ExePath;
        LocalPath = config.LocalPath;
    }

    public ProjectConfig GetUpdatedConfig()
    {
        _config.Title = Title;
        _config.ExePath = ExePath;
        _config.LocalPath = LocalPath;
        return _config;
    }

    [RelayCommand]
    private void SelectExeFile()
    {
        // 由 View 层通过文件选择器交互处理
    }

    [RelayCommand]
    private void SelectLocalFolder()
    {
        // 由 View 层通过文件夹选择器交互处理
    }

    [RelayCommand]
    private void Save()
    {
        var valid = true;
        if (string.IsNullOrWhiteSpace(Title))
        {
            TitleError = "显示标题不能为空";
            valid = false;
        }
        if (string.IsNullOrWhiteSpace(LocalPath))
        {
            LocalPathError = "本地文件夹路径不能为空";
            valid = false;
        }
        if (!valid) return;

        try
        {
            _configService.UpdateProject(_config);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "保存项目设置失败");
        }
    }

    [RelayCommand]
    private async Task OpenIgnoreConfig()
    {
        if (OpenIgnoreConfigRequested != null)
            await OpenIgnoreConfigRequested.Invoke();
    }

    public void RefreshFromConfig()
    {
        Title = _config.Title;
        ExePath = _config.ExePath;
        LocalPath = _config.LocalPath;
    }
}
