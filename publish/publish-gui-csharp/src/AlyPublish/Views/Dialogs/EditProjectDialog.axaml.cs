using AlyPublish.ViewModels;
using Avalonia.Controls;
using Avalonia.Interactivity;

namespace AlyPublish.Views.Dialogs;

public partial class EditProjectDialog : Window
{
    public EditProjectDialog()
    {
        InitializeComponent();
        Closing += OnClosing;
    }

    private async void OnBrowseMainExe(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not EditProjectDialogViewModel viewModel)
        {
            return;
        }

        var path = await StoragePicker.PickExecutableAsync(this, "选择主程序", viewModel.ProjectPath);
        if (path is not null)
        {
            viewModel.MainExePath = path;
        }
    }

    private async void OnPickIgnoreFolder(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not EditProjectDialogViewModel viewModel)
        {
            return;
        }

        var paths = await StoragePicker.PickFoldersAsync(this, "选择要忽略的文件夹", viewModel.ProjectPath);
        foreach (var path in paths)
        {
            viewModel.AddIgnoreFolder(path);
        }
    }

    private async void OnPickIgnoreFile(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not EditProjectDialogViewModel viewModel)
        {
            return;
        }

        var paths = await StoragePicker.PickFilesAsync(this, "选择要忽略的文件", viewModel.ProjectPath);
        foreach (var path in paths)
        {
            viewModel.AddIgnoreFile(path);
        }
    }

    private async void OnPickUnCopyFolder(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not EditProjectDialogViewModel viewModel)
        {
            return;
        }

        var paths = await StoragePicker.PickFoldersAsync(this, "选择不复制文件夹", viewModel.ProjectPath);
        foreach (var path in paths)
        {
            viewModel.AddUnCopyFolder(path);
        }
    }

    private async void OnPickUnCopyFile(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not EditProjectDialogViewModel viewModel)
        {
            return;
        }

        var paths = await StoragePicker.PickFilesAsync(this, "选择不复制文件", viewModel.ProjectPath);
        foreach (var path in paths)
        {
            viewModel.AddUnCopyFile(path);
        }
    }

    private void OnClosing(object? sender, WindowClosingEventArgs e)
    {
        if (DataContext is ViewModelBase { IsBusy: true })
        {
            e.Cancel = true;
        }
    }
}
