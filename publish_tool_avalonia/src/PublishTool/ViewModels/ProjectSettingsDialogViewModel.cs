using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models.Local;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class ProjectSettingsDialogViewModel : ObservableObject
{
    private readonly ConfigService _configService;
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
        // 在 View 层调用文件选择器
    }

    [RelayCommand]
    private void SelectLocalFolder()
    {
        // 在 View 层调用文件夹选择器
    }

    [RelayCommand]
    private void Save()
    {
        _configService.UpdateProject(_config);
    }

    [RelayCommand]
    private async Task OpenIgnoreConfig()
    {
        if (OpenIgnoreConfigRequested != null)
            await OpenIgnoreConfigRequested.Invoke();
    }
}
