using System;
using System.Collections.ObjectModel;
using System.Linq;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models.Local;
using PublishTool.Services;
using Serilog;

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
        _config.IgnoreFolders = new ObservableCollection<string>(
            IgnoreFoldersText.Split('\n').Where(s => !string.IsNullOrWhiteSpace(s)));
        _config.IgnoreFiles = new ObservableCollection<string>(
            IgnoreFilesText.Split('\n').Where(s => !string.IsNullOrWhiteSpace(s)));
        return _config;
    }

    [RelayCommand]
    private void Save()
    {
        try
        {
            GetUpdatedConfig();
            _configService.UpdateProject(_config);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "保存忽略配置失败");
        }
    }
}
