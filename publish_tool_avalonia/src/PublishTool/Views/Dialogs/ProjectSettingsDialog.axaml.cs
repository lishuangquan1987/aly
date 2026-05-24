using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using Microsoft.Extensions.DependencyInjection;
using PublishTool.Services;
using PublishTool.ViewModels;

namespace PublishTool.Views.Dialogs;

public partial class ProjectSettingsDialog : Window
{

    public ProjectSettingsDialog()
    {
        InitializeComponent();
        DataContextChanged += OnDataContextChanged;
    }

    private void OnDataContextChanged(object? sender, EventArgs e)
    {
        if (DataContext is ProjectSettingsDialogViewModel vm)
        {
            vm.OpenIgnoreConfigRequested += OnOpenIgnoreConfigRequested;
        }
    }

    private async Task OnOpenIgnoreConfigRequested()
    {
        if (DataContext is ProjectSettingsDialogViewModel vm)
        {
            var dialogVm = new ConfigEditorDialogViewModel(vm.Config, App.Services.GetRequiredService<PublishTool.Services.ConfigService>());
            var dialog = new ConfigEditorDialog { DataContext = dialogVm };
            await dialog.ShowDialog(this);
            vm.RefreshFromConfig();
        }
    }

    private async void SelectLocalFolder_Click(object? sender, RoutedEventArgs e)
    {
        var topLevel = TopLevel.GetTopLevel(this);
        if (topLevel == null) return;

        var folders = await topLevel.StorageProvider.OpenFolderPickerAsync(
            new FolderPickerOpenOptions { Title = "选择本地文件夹路径", AllowMultiple = false });

        var path = folders.FirstOrDefault()?.Path.LocalPath;
        if (path != null && DataContext is ProjectSettingsDialogViewModel vm)
            vm.LocalPath = path;
    }

    private async void SelectExeFile_Click(object? sender, RoutedEventArgs e)
    {
        var topLevel = TopLevel.GetTopLevel(this);
        var files = await topLevel.StorageProvider.OpenFilePickerAsync(
            new FilePickerOpenOptions { Title = "选择 EXE 文件", AllowMultiple = false });

        var path = files.FirstOrDefault()?.Path.LocalPath;
        if (path != null && DataContext is ProjectSettingsDialogViewModel vm)
            vm.ExePath = path;
    }
}
