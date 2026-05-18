using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models.Local;
using PublishTool.Services;

namespace PublishTool.ViewModels;

public partial class ConfigEditorDialogViewModel : ObservableObject
{
    private readonly ProjectConfig _config;
    private readonly ConfigService _configService;

    [ObservableProperty]
    private string _ignoreFoldersText = string.Empty;

    [ObservableProperty]
    private string _ignoreFilesText = string.Empty;

    public ConfigEditorDialogViewModel(ProjectConfig config, ConfigService configService)
    {
        _config = config;
        _configService = configService;
        IgnoreFoldersText = string.Join("\n", _config.IgnoreFolders);
        IgnoreFilesText = string.Join("\n", _config.IgnoreFiles);
    }

    public ProjectConfig GetUpdatedConfig()
    {
        _config.IgnoreFolders = IgnoreFoldersText.Split('\n')
            .Where(s => !string.IsNullOrWhiteSpace(s)).ToList();
        _config.IgnoreFiles = IgnoreFilesText.Split('\n')
            .Where(s => !string.IsNullOrWhiteSpace(s)).ToList();
        return _config;
    }

    [RelayCommand]
    private void Save()
    {
        GetUpdatedConfig();
        _configService.UpdateProject(_config);
    }
}