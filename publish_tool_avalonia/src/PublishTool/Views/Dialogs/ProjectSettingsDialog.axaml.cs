using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using PublishTool.ViewModels;

namespace PublishTool.Views.Dialogs;

public partial class ProjectSettingsDialog : Window
{
    public ProjectSettingsDialog()
    {
        InitializeComponent();
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
        if (topLevel == null) return;

        var files = await topLevel.StorageProvider.OpenFilePickerAsync(
            new FilePickerOpenOptions { Title = "选择 EXE 文件", AllowMultiple = false });

        var path = files.FirstOrDefault()?.Path.LocalPath;
        if (path != null && DataContext is ProjectSettingsDialogViewModel vm)
            vm.ExePath = path;
    }
}
