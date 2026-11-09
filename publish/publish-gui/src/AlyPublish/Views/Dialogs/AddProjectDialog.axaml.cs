using System.Collections.Generic;
using System.Linq;
using AlyPublish.ViewModels;
using Avalonia.Controls;
using Avalonia.Input;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;

namespace AlyPublish.Views.Dialogs;

public partial class AddProjectDialog : Window
{
    public AddProjectDialog()
    {
        InitializeComponent();
        DragDrop.SetAllowDrop(this, true);
        AddHandler(DragDrop.DragOverEvent, OnDragOver);
        AddHandler(DragDrop.DropEvent, OnDrop);
        Closing += OnClosing;
    }

    private static void OnDragOver(object? sender, DragEventArgs e)
        => e.DragEffects = e.DataTransfer.Formats.Contains(DataFormat.File)
            ? DragDropEffects.Copy
            : DragDropEffects.None;

    private void OnDrop(object? sender, DragEventArgs e)
    {
        var path = GetFirstLocalPath(e);
        if (path is not null)
        {
            (DataContext as AddProjectDialogViewModel)?.SetDroppedFolder(path);
        }
    }

    private async void OnBrowseProjectFolder(object? sender, RoutedEventArgs e)
    {
        var path = await StoragePicker.PickFolderAsync(this, "选择项目文件夹");
        if (path is not null && DataContext is AddProjectDialogViewModel viewModel)
        {
            viewModel.ProjectPath = path;
        }
    }

    private async void OnBrowseMainExe(object? sender, RoutedEventArgs e)
    {
        var path = await StoragePicker.PickExecutableAsync(this, "选择主程序");
        if (path is not null && DataContext is AddProjectDialogViewModel viewModel)
        {
            viewModel.MainExePath = path;
        }
    }

    private void OnClosing(object? sender, WindowClosingEventArgs e)
    {
        if (DataContext is ViewModelBase { IsBusy: true })
        {
            e.Cancel = true;
        }
    }

    internal static string? GetFirstLocalPath(DragEventArgs e)
    {
        var files = e.DataTransfer.TryGetFiles();
        return files?
            .Select(item => item.TryGetLocalPath())
            .FirstOrDefault(path => !string.IsNullOrWhiteSpace(path));
    }
}
