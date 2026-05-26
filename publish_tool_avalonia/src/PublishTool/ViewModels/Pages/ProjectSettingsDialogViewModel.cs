using System;
using System.Linq;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Platform.Storage;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.DependencyInjection;
using PublishTool.Models.Local;
using PublishTool.Services;
using PublishTool.Views.Pages;
using Serilog;
using Ursa.Controls;

namespace PublishTool.ViewModels.Pages;

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
    private async Task SelectExeFile()
    {
        var topLevel = TopLevel.GetTopLevel(App.MainWindow);
        if (topLevel == null) return;

        var files = await topLevel.StorageProvider.OpenFilePickerAsync(
            new FilePickerOpenOptions
            {
                Title = "选择 EXE 文件",
                AllowMultiple = false,
                FileTypeFilter = new[]
                {
                    new FilePickerFileType("exe文件")
                    {
                        Patterns = ["*.exe"]
                    }
                }
            });

        var path = files.FirstOrDefault()?.Path.LocalPath;
        if (path != null)
            ExePath = path;
    }

    [RelayCommand]
    private async Task SelectLocalFolder()
    {
        var topLevel = TopLevel.GetTopLevel(App.MainWindow);
        if (topLevel == null) return;

        var folders = await topLevel.StorageProvider.OpenFolderPickerAsync(
            new FolderPickerOpenOptions
            {
                Title = "选择本地文件夹路径",
                AllowMultiple = false
            });

        var path = folders.FirstOrDefault()?.Path.LocalPath;
        if (path != null)
            LocalPath = path;
    }

    public void Save()
    {
        GetUpdatedConfig();

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
        var configService = App.Services.GetRequiredService<ConfigService>();
        var vm = new ConfigEditorDialogViewModel(_config, configService);
        var result = await Dialog.ShowStandardAsync<ConfigEditorDialogView, ConfigEditorDialogViewModel>(
            vm, null, new DialogOptions
            {
                Title = "忽略配置",
                Button = DialogButton.OKCancel
            });
        if (result == DialogResult.OK)
        {
            vm.Save();
        }
    }

    public void RefreshFromConfig()
    {
        Title = _config.Title;
        ExePath = _config.ExePath;
        LocalPath = _config.LocalPath;
    }
}
